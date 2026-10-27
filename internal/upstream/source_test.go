package upstream

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/config"
)

func boardEvent(pad int) string {
	return fmt.Sprintf("event: board\ndata: {\"host\":%q}\n\n", strings.Repeat("h", pad))
}

func streamOnce(t *testing.T, s *Source) (bool, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Client = &http.Client{}
	return s.stream(ctx, func() {})
}

func TestRedirectIsRefusedAndTokenNotForwarded(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
		fmt.Fprint(w, boardEvent(1))
	}))
	defer target.Close()
	// The same-host redirect is the case Go would follow with the header intact.
	var hits atomic.Int32
	var self *httptest.Server
	self = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/v1/events" {
			http.Redirect(w, r, self.URL+"/elsewhere", http.StatusFound)
			return
		}
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
	}))
	defer self.Close()

	for _, base := range []string{self.URL, target.URL} {
		redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, base+"/v1/events", http.StatusTemporaryRedirect)
		}))
		s := NewSource(config.Upstream{Name: "u", URL: redirector.URL, Token: "secret-token"})
		got, err := streamOnce(t, s)
		redirector.Close()
		if got || err == nil || !strings.Contains(err.Error(), "HTTP 307") {
			t.Fatalf("redirect to %s: got %v, err %v; want an HTTP 307 error", base, got, err)
		}
	}
	if leaked.Load() {
		t.Fatal("the Authorization header reached a redirect target")
	}
	if hits.Load() != 0 {
		t.Fatalf("a redirect was followed (%d requests)", hits.Load())
	}
}

func eventServer(t *testing.T, pad int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, boardEvent(pad))
	}))
}

func TestOversizedEventRejectedAndLargerLimitAccepts(t *testing.T) {
	srv := eventServer(t, 200<<10)
	defer srv.Close()

	small := NewSource(config.Upstream{Name: "u", URL: srv.URL, Token: "t"})
	small.MaxEventBytes = 64 << 10
	if got, err := streamOnce(t, small); got || err == nil {
		t.Fatalf("event over the limit: got %v, err %v; want a refusal", got, err)
	}

	big := NewSource(config.Upstream{Name: "u", URL: srv.URL, Token: "t"})
	big.MaxEventBytes = 1 << 20
	if got, err := streamOnce(t, big); !got || err != nil {
		t.Fatalf("event under the limit: got %v, err %v", got, err)
	}
}

func TestDefaultEventLimitIs8MiB(t *testing.T) {
	if got := (&Source{}).eventLimit(); got != 8<<20 {
		t.Fatalf("default limit %d", got)
	}
}
