package forge

// GitHub's rate limits, honoured rather than fought: the limiter reads the
// response headers and, once the budget is spent or the server says to wait,
// refuses further calls until the stated time. An unauthenticated client has
// 60 requests an hour, so this is what keeps a public-repo board polite.

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	maxBackoff     = 15 * time.Minute
	defaultBackoff = time.Minute // a secondary limit that states no time
)

type limiter struct {
	mu    sync.Mutex
	until time.Time
	now   func() time.Time
}

// wait returns ErrUnknown while a backoff is in force.
func (l *limiter) wait() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.now().Before(l.until) {
		return ErrUnknown
	}
	return nil
}

// observe records a response. Retry-After wins; otherwise an exhausted
// X-RateLimit-Remaining waits for X-RateLimit-Reset (epoch seconds); a bare
// 403 or 429 waits a default minute.
func (l *limiter) observe(res *http.Response) {
	if res == nil {
		return
	}
	now := l.now()
	var until time.Time
	if s, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && s > 0 {
		until = now.Add(time.Duration(s) * time.Second)
	} else if res.Header.Get("X-RateLimit-Remaining") == "0" {
		if reset, err := strconv.ParseInt(res.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			until = time.Unix(reset, 0)
		}
	}
	if until.IsZero() && (res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusTooManyRequests) &&
		res.Header.Get("X-RateLimit-Remaining") != "" {
		until = now.Add(defaultBackoff)
	}
	if until.IsZero() {
		return
	}
	if until.After(now.Add(maxBackoff)) {
		until = now.Add(maxBackoff)
	}
	l.mu.Lock()
	if until.After(l.until) {
		l.until = until
	}
	l.mu.Unlock()
}
