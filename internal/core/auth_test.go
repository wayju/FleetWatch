package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func authServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	hub := NewHub()
	hub.Publish([]byte(`{"counts":{}}`), 1, true)
	srv := httptest.NewServer(tokenGuard(token, newMux(hub, &pollHealth{}, time.Second, nil)))
	t.Cleanup(srv.Close)
	return srv
}

func status(t *testing.T, url, auth string) int {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func TestTokenGuard(t *testing.T) {
	srv := authServer(t, "s3cr3t-token-value")
	for _, path := range []string{"/fleet", "/events"} {
		if got := status(t, srv.URL+path, ""); got != 401 {
			t.Errorf("%s without token: %d, want 401", path, got)
		}
		if got := status(t, srv.URL+path, "Bearer wrong"); got != 401 {
			t.Errorf("%s wrong token: %d, want 401", path, got)
		}
		if got := status(t, srv.URL+path, "s3cr3t-token-value"); got != 401 {
			t.Errorf("%s token without Bearer scheme: %d, want 401", path, got)
		}
	}
	if got := status(t, srv.URL+"/fleet", "Bearer s3cr3t-token-value"); got != 200 {
		t.Errorf("/fleet right token: %d, want 200", got)
	}
	if got := status(t, srv.URL+"/liveness", ""); got != 200 {
		t.Errorf("/liveness needs no token: %d", got)
	}
	if got := status(t, srv.URL+"/version", ""); got != 200 {
		t.Errorf("/version needs no token: %d", got)
	}
}

func TestTokenGuardFailsClosedWithoutToken(t *testing.T) {
	srv := authServer(t, "")
	for _, auth := range []string{"", "Bearer ", "Bearer anything"} {
		for _, path := range []string{"/fleet", "/events"} {
			if got := status(t, srv.URL+path, auth); got != 503 {
				t.Errorf("%s with empty FLEET_TOKEN and %q: %d, want 503", path, auth, got)
			}
		}
	}
	if got := status(t, srv.URL+"/liveness", ""); got != 200 {
		t.Errorf("/liveness with empty FLEET_TOKEN: %d, want 200", got)
	}
}

func TestAnonymiseDropsNameAndOrgWithTextOff(t *testing.T) {
	in := Account{Email: "a@work.example", Name: "Alice", Org: "Work"}
	off := anonymise(in, false)
	if off.Name != "" || off.Org != "" || strings.Contains(off.Email, "@") {
		t.Errorf("text off: %+v", off)
	}
	if on := anonymise(in, true); on.Name != "Alice" || on.Org != "Work" {
		t.Errorf("text on: %+v", on)
	}
}
