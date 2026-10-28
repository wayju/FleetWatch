package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Built, not written out, so no scanner takes it for a credential.
var testToken = strings.Repeat("tk", 12)

type uiRig struct {
	srv      *httptest.Server
	sessions *sessionStore
	clock    *time.Time
}

// newUIRig serves the production wiring (mux, UI routes, auth guard) over
// plain HTTP, or TLS when tls is set, with a clock the test can move.
func newUIRig(t *testing.T, opt uiOptions, token string, tls bool) *uiRig {
	t.Helper()
	hub := NewHub()
	hub.Publish([]byte(`{"counts":{}}`), 1, true)
	mux := newMux(hub, &pollHealth{}, time.Second, nil)
	now := time.Unix(1_700_000_000, 0)
	sessions := newSessionStore(sessionTTL, sessionCap)
	sessions.now = func() time.Time { return now }
	registerUI(mux, opt, token, sessions)
	h := authGuard(token, sessions, mux)
	var srv *httptest.Server
	if tls {
		srv = httptest.NewTLSServer(h)
	} else {
		srv = httptest.NewServer(h)
	}
	t.Cleanup(srv.Close)
	return &uiRig{srv: srv, sessions: sessions, clock: &now}
}

func (g *uiRig) do(t *testing.T, method, path, origin, body string, hdr map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, g.srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := g.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res
}

func (g *uiRig) login(t *testing.T, origin string, hdr map[string]string) *http.Response {
	return g.do(t, "POST", "/v1/session", origin, `{"token":"`+testToken+`"}`, hdr)
}

func sessionCookieOf(res *http.Response) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	return nil
}

func withCookie(c *http.Cookie) map[string]string {
	return map[string]string{"Cookie": sessionCookie + "=" + c.Value}
}

func TestSessionLoginGrantsCookieAccess(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	res := g.login(t, g.srv.URL, nil)
	if res.StatusCode != 204 {
		t.Fatalf("login: %d, want 204", res.StatusCode)
	}
	c := sessionCookieOf(res)
	if c == nil {
		t.Fatal("no session cookie")
	}
	if strings.Contains(c.Value, testToken) || len(c.Value) != 64 {
		t.Errorf("cookie must hold a random id, never the token: %q", c.Value)
	}
	if got := g.do(t, "GET", "/v1/fleet", "", "", withCookie(c)).StatusCode; got != 200 {
		t.Errorf("/v1/fleet with the cookie: %d, want 200", got)
	}
	if got := g.do(t, "GET", "/v1/fleet", "", "", nil).StatusCode; got != 401 {
		t.Errorf("/v1/fleet with neither: %d, want 401", got)
	}
	if got := g.do(t, "GET", "/fleet", "", "", withCookie(c)).StatusCode; got != 401 {
		t.Errorf("/fleet outside the cookie's path needs the bearer: %d, want 401", got)
	}
	if got := g.do(t, "GET", "/v1/fleet", "", "", map[string]string{"Authorization": "Bearer " + testToken}).StatusCode; got != 200 {
		t.Errorf("the bearer still works: %d", got)
	}
	if got := g.do(t, "GET", "/v1/fleet", "", "", map[string]string{"Cookie": sessionCookie + "=" + strings.Repeat("0", 64)}).StatusCode; got != 401 {
		t.Errorf("an unknown session id: %d, want 401", got)
	}
}

func TestSessionWrongOrMissingTokenIsRefused(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	for name, body := range map[string]string{"wrong": `{"token":"nope"}`, "empty": `{"token":""}`, "absent": `{}`} {
		res := g.do(t, "POST", "/v1/session", g.srv.URL, body, nil)
		if res.StatusCode != 401 || sessionCookieOf(res) != nil {
			t.Errorf("%s token: %d with cookie %v, want 401 and none", name, res.StatusCode, sessionCookieOf(res))
		}
	}
	if got := g.do(t, "POST", "/v1/session", g.srv.URL, "not json", nil).StatusCode; got != 400 {
		t.Errorf("malformed body: %d, want 400", got)
	}
	if got := g.do(t, "POST", "/v1/session", g.srv.URL, `{"token":"`+strings.Repeat("a", 8192)+`"}`, nil).StatusCode; got != 400 {
		t.Errorf("oversized body: %d, want 400", got)
	}
}

func TestSessionRefusesMissingOrForeignOrigin(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	for name, origin := range map[string]string{
		"missing": "", "foreign": "http://evil.example", "null": "null",
		"same host other port": "http://127.0.0.1:1", "bad scheme": "ftp://" + strings.TrimPrefix(g.srv.URL, "http://"),
	} {
		res := g.login(t, origin, nil)
		if res.StatusCode != 403 || sessionCookieOf(res) != nil {
			t.Errorf("%s origin: %d, want 403 and no cookie", name, res.StatusCode)
		}
	}
	if got := g.do(t, "DELETE", "/v1/session", "http://evil.example", "", nil).StatusCode; got != 403 {
		t.Errorf("foreign-origin logout: %d, want 403", got)
	}
}

func TestSessionExpires(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	c := sessionCookieOf(g.login(t, g.srv.URL, nil))
	*g.clock = g.clock.Add(sessionTTL - time.Minute)
	if got := g.do(t, "GET", "/v1/fleet", "", "", withCookie(c)).StatusCode; got != 200 {
		t.Errorf("just before expiry: %d, want 200", got)
	}
	*g.clock = g.clock.Add(2 * time.Minute)
	if got := g.do(t, "GET", "/v1/fleet", "", "", withCookie(c)).StatusCode; got != 401 {
		t.Errorf("after expiry: %d, want 401", got)
	}
	if len(g.sessions.m) != 0 {
		t.Errorf("an expired session is forgotten on use, %d left", len(g.sessions.m))
	}
}

func TestSessionLogout(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	c := sessionCookieOf(g.login(t, g.srv.URL, nil))
	res := g.do(t, "DELETE", "/v1/session", g.srv.URL, "", withCookie(c))
	if res.StatusCode != 204 {
		t.Fatalf("logout: %d, want 204", res.StatusCode)
	}
	if gone := sessionCookieOf(res); gone == nil || gone.MaxAge >= 0 || gone.Value != "" {
		t.Errorf("logout should clear the cookie, got %+v", gone)
	}
	if got := g.do(t, "GET", "/v1/fleet", "", "", withCookie(c)).StatusCode; got != 401 {
		t.Errorf("after logout the old id: %d, want 401", got)
	}
}

func TestSessionCookieFlags(t *testing.T) {
	plain := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	c := sessionCookieOf(plain.login(t, plain.srv.URL, nil))
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/v1" || c.Secure {
		t.Errorf("plain http cookie: %+v, want HttpOnly, Strict, Path /v1, not Secure", c)
	}
	if c.MaxAge != int(sessionTTL.Seconds()) {
		t.Errorf("max-age %d, want the session lifetime", c.MaxAge)
	}
	fwd := sessionCookieOf(plain.login(t, plain.srv.URL, map[string]string{"X-Forwarded-Proto": "https"}))
	if !fwd.Secure {
		t.Error("a proxy that terminated TLS should get a Secure cookie")
	}
	tlsRig := newUIRig(t, uiOptions{Enabled: true}, testToken, true)
	tc := sessionCookieOf(tlsRig.login(t, tlsRig.srv.URL, nil))
	if tc == nil || !tc.Secure || !tc.HttpOnly {
		t.Errorf("TLS cookie: %+v, want Secure and HttpOnly", tc)
	}
}

func TestSessionStoreCapDropsExpiredThenOldest(t *testing.T) {
	now := time.Unix(1000, 0)
	s := newSessionStore(time.Hour, 2)
	s.now = func() time.Time { return now }
	a, _, _ := s.create()
	now = now.Add(time.Minute)
	b, _, _ := s.create()
	now = now.Add(time.Minute)
	c, _, _ := s.create()
	if len(s.m) != 2 || s.valid(a) || !s.valid(b) || !s.valid(c) {
		t.Fatalf("at the cap the oldest goes: size %d, a=%v b=%v c=%v", len(s.m), s.valid(a), s.valid(b), s.valid(c))
	}
	now = now.Add(61 * time.Minute)
	d, _, _ := s.create()
	if len(s.m) != 1 || !s.valid(d) {
		t.Errorf("expired entries are swept first: size %d", len(s.m))
	}
	var none *sessionStore
	if none.valid(d) {
		t.Error("a nil store has no sessions")
	}
}

func TestSessionRouteFailsClosedWithoutToken(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, "", false)
	if got := g.login(t, g.srv.URL, nil).StatusCode; got != 503 {
		t.Errorf("no token configured: %d, want 503", got)
	}
}

func TestUIDisabledRemovesPageAndSessionRoute(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: false}, testToken, false)
	for _, p := range []string{"/", "/ui/fleet.js", "/ui/index.html"} {
		if got := g.do(t, "GET", p, "", "", nil).StatusCode; got != 404 {
			t.Errorf("GET %s with the UI off: %d, want 404", p, got)
		}
	}
	if got := g.login(t, g.srv.URL, nil).StatusCode; got != 404 {
		t.Errorf("POST /v1/session with the UI off: %d, want 404", got)
	}
	if got := g.do(t, "GET", "/v1/fleet", "", "", nil).StatusCode; got != 401 {
		t.Errorf("the data routes stay guarded: %d, want 401", got)
	}
}

func TestTokenGuardIgnoresCookies(t *testing.T) {
	srv := authServer(t, testToken)
	req, _ := http.NewRequest("GET", srv.URL+"/v1/fleet", nil)
	req.Header.Set("Cookie", sessionCookie+"="+strings.Repeat("a", 64))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Errorf("a guard built without sessions accepts only the bearer: %d", res.StatusCode)
	}
}

func TestCookieIsAcceptedOnTheDataRoutesOnly(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	c := sessionCookieOf(g.login(t, g.srv.URL, nil))
	for _, p := range []string{"/v1/version-x", "/v1/health/../fleet", "/v1/fleet/extra", "/v1/other"} {
		if got := g.do(t, "GET", p, "", "", withCookie(c)).StatusCode; got == 200 {
			t.Errorf("%s accepted the cookie", p)
		}
	}
}

// readUntilEnd reports whether the stream ends within d.
func streamEnds(t *testing.T, g *uiRig, c *http.Cookie, trigger func(), d time.Duration) bool {
	t.Helper()
	req, _ := http.NewRequest("GET", g.srv.URL+"/v1/events", nil)
	if c != nil {
		req.Header.Set("Cookie", sessionCookie+"="+c.Value)
	} else {
		req.Header.Set("Authorization", "Bearer "+testToken)
	}
	res, err := g.srv.Client().Do(req)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("stream: %v %v", err, res)
	}
	defer res.Body.Close()
	buf := make([]byte, 4096)
	res.Body.Read(buf) // the first event
	done := make(chan struct{})
	go func() {
		for {
			if _, err := res.Body.Read(buf); err != nil {
				close(done)
				return
			}
		}
	}()
	trigger()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

func TestLogoutEndsOpenStreams(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	c := sessionCookieOf(g.login(t, g.srv.URL, nil))
	if !streamEnds(t, g, c, func() { g.do(t, "DELETE", "/v1/session", g.srv.URL, "", withCookie(c)) }, 2*time.Second) {
		t.Error("the stream outlived logout")
	}
}

func TestExpiryEndsOpenStreams(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	c := sessionCookieOf(g.login(t, g.srv.URL, nil))
	if !streamEnds(t, g, c, func() {
		*g.clock = g.clock.Add(sessionTTL + time.Minute)
		g.sessions.mu.Lock()
		g.sessions.sweepLocked(g.sessions.now())
		g.sessions.mu.Unlock()
	}, 2*time.Second) {
		t.Error("the stream outlived the session's expiry")
	}
}

func TestBearerStreamsAreNotTiedToASession(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	if streamEnds(t, g, nil, func() {
		*g.clock = g.clock.Add(2 * sessionTTL)
		g.sessions.drop("anything")
	}, 300*time.Millisecond) {
		t.Error("a bearer stream ended with no session involved")
	}
}

func TestLoginBackoffPerAddress(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	bad := `{"token":"nope"}`
	for i := 0; i < loginFailMax; i++ {
		if got := g.do(t, "POST", "/v1/session", g.srv.URL, bad, nil).StatusCode; got != 401 {
			t.Fatalf("attempt %d: %d, want 401", i+1, got)
		}
	}
	if got := g.do(t, "POST", "/v1/session", g.srv.URL, bad, nil).StatusCode; got != 429 {
		t.Errorf("after %d failures: %d, want 429", loginFailMax, got)
	}
	if got := g.login(t, g.srv.URL, nil).StatusCode; got != 429 {
		t.Errorf("even the right token waits out the window: %d, want 429", got)
	}
	*g.clock = g.clock.Add(loginFailWindow + time.Second)
	if got := g.login(t, g.srv.URL, nil).StatusCode; got != 204 {
		t.Errorf("after the window: %d, want 204", got)
	}
}

func TestLoginBackoffTableIsBounded(t *testing.T) {
	s := newSessionStore(time.Hour, 4)
	for i := 0; i < loginTrackMax*3; i++ {
		s.fail(strings.Repeat("a", 1) + string(rune('A'+i%50)) + time.Duration(i).String())
	}
	if len(s.fails) > loginTrackMax {
		t.Errorf("tracked %d addresses, cap %d", len(s.fails), loginTrackMax)
	}
}

func TestAuthEqualIgnoresLength(t *testing.T) {
	if !authEqual("abc", "abc") || authEqual("abc", "abcd") || authEqual("", "x") || authEqual("abc", "") {
		t.Error("authEqual")
	}
}

func TestWeakTokenWarning(t *testing.T) {
	if weakTokenWarning(strings.Repeat("x", 31)) == "" {
		t.Error("31 bytes should warn")
	}
	if weakTokenWarning(strings.Repeat("x", 32)) != "" || weakTokenWarning("") != "" {
		t.Error("32 bytes or no token should not warn")
	}
}
