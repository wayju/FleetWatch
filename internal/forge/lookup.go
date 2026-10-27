package forge

// Lookup caches forge answers and fetches misses in the background. A call
// returns what is cached at once and queues a fetch when the entry is missing
// or stale, so a slow forge never delays a snapshot; an unknown answer is
// reported as unknown and retried after a short error TTL.

import (
	"fmt"
	"sync"
	"time"
)

const (
	issueTTL = 5 * time.Minute
	prTTL    = 10 * time.Minute
	errTTL   = time.Minute
)

type cached[T any] struct {
	at  time.Time
	ok  bool
	val T
}

type cache[T any] struct {
	mu       sync.Mutex
	entries  map[string]cached[T]
	inflight map[string]bool
}

func newCache[T any]() *cache[T] {
	return &cache[T]{entries: map[string]cached[T]{}, inflight: map[string]bool{}}
}

// get returns the held answer and starts fetch in the background when the
// entry is missing or past its TTL (the error TTL when the last try failed).
func (c *cache[T]) get(key string, now time.Time, ttl time.Duration, fetch func() (T, error)) (T, bool) {
	c.mu.Lock()
	e, have := c.entries[key]
	life := ttl
	if have && !e.ok {
		life = errTTL
	}
	if (!have || now.Sub(e.at) >= life) && !c.inflight[key] {
		c.inflight[key] = true
		go func() {
			v, err := fetch()
			c.mu.Lock()
			c.entries[key] = cached[T]{at: time.Now(), ok: err == nil, val: v}
			delete(c.inflight, key)
			c.mu.Unlock()
		}()
	}
	c.mu.Unlock()
	return e.val, have && e.ok
}

// Lookup is the cached front of a Set.
type Lookup struct {
	Set    *Set
	issues *cache[Issue]
	prs    *cache[*PR]
}

// NewLookup wraps a Set. A nil or empty Set yields a Lookup that answers nothing.
func NewLookup(set *Set) *Lookup {
	return &Lookup{Set: set, issues: newCache[Issue](), prs: newCache[*PR]()}
}

// Match is the forge for a repo, or nil. Safe on a nil Lookup.
func (l *Lookup) Match(host, repo string) *Entry {
	if l == nil {
		return nil
	}
	return l.Set.Match(host, repo)
}

// Issue is the cached issue, if any answer has landed yet.
func (l *Lookup) Issue(e *Entry, repo string, n int, now time.Time) (Issue, bool) {
	if l == nil || e == nil {
		return Issue{}, false
	}
	key := fmt.Sprintf("%s|%s#%d", e.Host, repo, n)
	return l.issues.get(key, now, issueTTL, func() (Issue, error) { return e.Forge.Issue(repo, n) })
}

// Merged is true or false only when the forge has answered for this branch;
// nil otherwise, never a guess. Only forges serving the remote host are asked.
func (l *Lookup) Merged(host, repo, branch string, now time.Time) *bool {
	if l == nil || host == "" {
		return nil
	}
	e := l.Set.Match(host, repo)
	if e == nil {
		return nil
	}
	pr, ok := l.prs.get(e.Host+"|"+repo+"|"+branch, now, prTTL, func() (*PR, error) { return e.Forge.PRForBranch(repo, branch) })
	if !ok {
		return nil
	}
	res := pr != nil && pr.Merged
	return &res
}
