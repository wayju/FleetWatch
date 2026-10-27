package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// routeStatus fetches path from a server guarded as production guards it.
func routeStatus(t *testing.T, token, auth, path string) int {
	t.Helper()
	hub := NewHub()
	hub.Publish([]byte(`{"generated_at":1}`), 1, true)
	health := &pollHealth{}
	health.ok(time.Now(), 0)
	mux := newMux(hub, health, time.Second, nil)
	srv := httptest.NewUnstartedServer(nil)
	srv.Config.Handler = hostGuard([]string{srv.Listener.Addr().String()}, tokenGuard(token, mux))
	srv.Start()
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel() // ends an /events stream once its status is known
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	defer res.Body.Close()
	return res.StatusCode
}

// Each route answers the same under /v1 as without it, with the same guards:
// no data route is ever open, and the probes need no token.
func TestRoutesAndV1AliasesBehaveIdentically(t *testing.T) {
	const token = "s3cret-token"
	cases := []struct {
		name        string
		token, auth string
		data, probe int
	}{
		{"no token configured", "", "", 503, 200},
		{"no token configured, bearer sent", "", "Bearer anything", 503, 200},
		{"wrong token", token, "Bearer nope", 401, 200},
		{"no authorization header", token, "", 401, 200},
		{"right token", token, "Bearer " + token, 200, 200},
	}
	for _, c := range cases {
		for _, p := range []struct {
			path string
			want int
		}{{"/fleet", c.data}, {"/events", c.data}, {"/health", c.probe}, {"/liveness", c.probe}, {"/version", c.probe}} {
			old := routeStatus(t, c.token, c.auth, p.path)
			v1 := routeStatus(t, c.token, c.auth, "/v1"+p.path)
			if old != p.want || v1 != p.want {
				t.Errorf("%s %s: old %d, /v1 %d, want %d", c.name, p.path, old, v1, p.want)
			}
		}
	}
}

func TestUnknownPathsAreNotAliased(t *testing.T) {
	for _, p := range []string{"/v1", "/v1/", "/v2/fleet", "/v1/v1/fleet", "/v1/nope"} {
		if got := routeStatus(t, "tok", "Bearer tok", p); got != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", p, got)
		}
	}
	// A probe-looking path under /v1/v1 must not skip the token guard.
	if got := routeStatus(t, "tok", "", "/v1/v1/health"); got != http.StatusUnauthorized {
		t.Errorf("/v1/v1/health without a token: %d, want 401", got)
	}
}
