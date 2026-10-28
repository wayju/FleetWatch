// session.go: browser sessions (ADR-012). EventSource cannot send an
// Authorization header and a token in the URL leaks into logs and history, so a
// browser trades the token once, at POST /v1/session, for a cookie holding a
// random session id. The id is the only thing the cookie carries; the token
// never leaves the request that presented it.
package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie = "fleet_session"
	sessionPath   = "/v1/session"
	sessionTTL    = 12 * time.Hour
	sessionCap    = 64
	// A token is at most a few hundred bytes; the cap stops a client streaming
	// an unbounded body at an unauthenticated route.
	sessionBodyMax = 4 << 10

	// Failed logins allowed per remote address in one window before the route
	// answers 429, and how many addresses are tracked at once.
	loginFailMax    = 5
	loginFailWindow = time.Minute
	loginTrackMax   = 1024
)

// authEqual compares two secrets through their SHA-256 digests, so neither the
// content nor the length of the expected value shows in the timing.
func authEqual(a, b string) bool {
	x, y := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(x[:], y[:]) == 1
}

// weakTokenWarning is the startup warning for a token too short to resist
// guessing, or "" when the token is long enough or absent.
func weakTokenWarning(token string) string {
	if token == "" || len(token) >= 32 {
		return ""
	}
	return "WARNING: the token is under 32 bytes; use a longer random value (FLEET_TOKEN), since anyone who can reach the port may guess it"
}

type failRec struct {
	n     int
	start time.Time
}

// sessionRef is what a cookie-admitted request carries in its context, so a
// long-lived stream can notice its session ending.
type sessionRef struct {
	store *sessionStore
	id    string
	done  <-chan struct{} // closed when the session is dropped or expires
	exp   time.Time
}

type sessionCtxKey struct{}

func sessionFrom(ctx context.Context) (sessionRef, bool) {
	r, ok := ctx.Value(sessionCtxKey{}).(sessionRef)
	return r, ok
}

// alive reports whether the session is still valid.
func (r sessionRef) alive() bool { return r.store.valid(r.id) }

// remaining is the time left before the session expires.
func (r sessionRef) remaining() time.Duration { return r.exp.Sub(r.store.now()) }

// sessionStore keeps session ids in memory with an absolute expiry. A restart
// signs everyone out, which is the intended cost of holding no state on disk.
type sessionStore struct {
	mu  sync.Mutex
	ttl time.Duration
	max int
	now func() time.Time
	m   map[string]time.Time // id -> expiry
	// done holds a channel per session, closed when it ends, which is how
	// logout and expiry reach the streams it admitted.
	done  map[string]chan struct{}
	fails map[string]failRec // remote address -> recent failed logins
}

func newSessionStore(ttl time.Duration, max int) *sessionStore {
	return &sessionStore{ttl: ttl, max: max, now: time.Now, m: map[string]time.Time{}, done: map[string]chan struct{}{}, fails: map[string]failRec{}}
}

// create mints a session and returns its id and expiry. At the cap it drops
// expired entries first, then the one closest to expiry, so a flood of logins
// can push out older sessions but never grow the map.
func (s *sessionStore) create() (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	id := hex.EncodeToString(buf)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if len(s.m) >= s.max {
		s.sweepLocked(now)
	}
	for len(s.m) >= s.max {
		var oldest string
		for k, exp := range s.m {
			if oldest == "" || exp.Before(s.m[oldest]) {
				oldest = k
			}
		}
		s.endLocked(oldest)
	}
	exp := now.Add(s.ttl)
	s.m[id] = exp
	s.done[id] = make(chan struct{})
	return id, exp, nil
}

// endLocked forgets a session and wakes anything waiting on it.
func (s *sessionStore) endLocked(id string) {
	delete(s.m, id)
	if c, ok := s.done[id]; ok {
		close(c)
		delete(s.done, id)
	}
}

func (s *sessionStore) sweepLocked(now time.Time) {
	for k, exp := range s.m {
		if !now.Before(exp) {
			s.endLocked(k)
		}
	}
}

// valid reports whether id names a live session, forgetting it if it expired.
func (s *sessionStore) valid(id string) bool {
	if s == nil || id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.m[id]
	if ok && !s.now().Before(exp) {
		s.endLocked(id)
		return false
	}
	return ok
}

func (s *sessionStore) drop(id string) {
	s.mu.Lock()
	s.endLocked(id)
	s.mu.Unlock()
}

// fromCookie returns a reference to the live session the request's cookie
// names, if any.
func (s *sessionStore) fromCookie(r *http.Request) (sessionRef, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || !s.valid(c.Value) {
		return sessionRef{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.m[c.Value]
	if !ok {
		return sessionRef{}, false
	}
	return sessionRef{store: s, id: c.Value, done: s.done[c.Value], exp: exp}, true
}

// throttled reports whether addr has used up its failed logins for this
// window. The table is bounded: expired records go first, then an arbitrary
// one, so a spread of addresses cannot grow it.
func (s *sessionStore) throttled(addr string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.fails[addr]
	if ok && s.now().Sub(rec.start) >= loginFailWindow {
		delete(s.fails, addr)
		return false
	}
	return ok && rec.n >= loginFailMax
}

func (s *sessionStore) fail(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	rec, ok := s.fails[addr]
	if !ok || now.Sub(rec.start) >= loginFailWindow {
		if len(s.fails) >= loginTrackMax {
			for k, v := range s.fails {
				if now.Sub(v.start) >= loginFailWindow {
					delete(s.fails, k)
				}
			}
			for k := range s.fails {
				if len(s.fails) < loginTrackMax {
					break
				}
				delete(s.fails, k)
			}
		}
		rec = failRec{start: now}
	}
	rec.n++
	s.fails[addr] = rec
}

func (s *sessionStore) forgive(addr string) {
	s.mu.Lock()
	delete(s.fails, addr)
	s.mu.Unlock()
}

func remoteAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// sameOrigin is true when the request's Origin names this server's own Host.
// A missing or "null" Origin fails: a browser always sends Origin on a POST
// that matters, and a client without one can use the bearer header instead.
func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return false
	}
	u, err := url.Parse(o)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// overTLS is true when the request reached us over TLS, directly or through a
// proxy that says so. A spoofed header can only add the Secure flag.
func overTLS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, id string, exp time.Time, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: id, Path: "/v1", Expires: exp, MaxAge: maxAge,
		HttpOnly: true, Secure: overTLS(r), SameSite: http.SameSiteStrictMode,
	})
}

// sessionHandler serves POST (log in) and DELETE (log out) on /v1/session.
func sessionHandler(token string, s *sessionStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !sameOrigin(r) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodDelete {
			if c, err := r.Cookie(sessionCookie); err == nil {
				s.drop(c.Value)
			}
			setSessionCookie(w, r, "", time.Unix(0, 0), -1)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if token == "" {
			http.Error(w, "FLEET_TOKEN not configured", http.StatusServiceUnavailable)
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, sessionBodyMax)).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		addr := remoteAddr(r)
		if s.throttled(addr) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "too many attempts", http.StatusTooManyRequests)
			return
		}
		if !authEqual(body.Token, token) {
			s.fail(addr)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		s.forgive(addr)
		id, exp, err := s.create()
		if err != nil {
			http.Error(w, "session unavailable", http.StatusInternalServerError)
			return
		}
		setSessionCookie(w, r, id, exp, int(s.ttl/time.Second))
		w.WriteHeader(http.StatusNoContent)
	})
}
