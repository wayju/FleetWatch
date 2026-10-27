// Package forge looks issues and pull requests up on a code forge, so the
// board can show titles and merged state without knowing which forge a repo
// lives on (see ADR-005). A Forge answers for one server; a Set picks the server for
// a repo from its remote host, and a Lookup caches the answers and fetches
// misses in the background so a slow forge never delays a snapshot.
package forge

import (
	"errors"
	"fmt"
	"strings"
)

// Issue is what a forge says about an issue or pull request.
type Issue struct {
	Title string
	State string // open, closed or merged
	URL   string
	IsPR  bool
}

// PR is a pull request whose head is a given branch.
type PR struct{ Merged bool }

var (
	// ErrUnknown means the forge could not give a certain answer (a failed
	// request, an unreadable body, or a listing cut short).
	ErrUnknown = errors.New("forge: no certain answer")
	// ErrUnsupported means this forge cannot answer the question as configured,
	// for instance a Gitea branch lookup without a token.
	ErrUnsupported = errors.New("forge: lookup not supported as configured")
)

// Forge answers for a single server. Repo is "owner/name".
type Forge interface {
	Issue(repo string, n int) (Issue, error)
	// PRForBranch returns the pull request headed by branch, preferring a
	// merged one. (nil, nil) is a certain "no pull request"; any error means
	// the answer is unknown.
	PRForBranch(repo, branch string) (*PR, error)
}

// Spec is one configured forge.
type Spec struct {
	Kind    string   // gitea or github
	Host    string   // the remote host that selects this forge
	Aliases []string // further remote hosts served by the same forge
	APIURL  string
	WebURL  string
	Token   string   // optional for GitHub, needed by Gitea for branch lookups
	Owners  []string // lower-case; empty means every owner is allowed
}

// New builds the Forge a Spec describes.
func New(s Spec) (Forge, error) {
	switch s.Kind {
	case "gitea":
		return newGitea(s), nil
	case "github":
		return newGitHub(s), nil
	}
	return nil, fmt.Errorf("unknown forge kind %q (known: gitea, github)", s.Kind)
}

// Entry pairs a Spec with its Forge.
type Entry struct {
	Spec
	Forge Forge
}

// Hosts lists every remote host the entry answers for.
func (e *Entry) Hosts() []string { return append([]string{e.Host}, e.Aliases...) }

func (e *Entry) hasHost(host string) bool {
	for _, h := range e.Hosts() {
		if strings.EqualFold(h, host) {
			return true
		}
	}
	return false
}

// OwnerAllowed keeps a prompt that names someone else's repo from making this
// service query (and authenticate to) a forge for it. No owners means all.
func (e *Entry) OwnerAllowed(repo string) bool {
	if len(e.Owners) == 0 {
		return true
	}
	owner, _, _ := strings.Cut(repo, "/")
	for _, o := range e.Owners {
		if strings.EqualFold(o, owner) {
			return true
		}
	}
	return false
}

// WebIssueURL is the browser link for an issue on this forge.
func (e *Entry) WebIssueURL(repo string, n int) string {
	return fmt.Sprintf("%s/%s/issues/%d", e.WebURL, repo, n)
}

// Set is the configured forges, in config order.
type Set struct{ entries []*Entry }

// NewSet builds every forge in specs.
func NewSet(specs []Spec) (*Set, error) {
	s := &Set{}
	for _, sp := range specs {
		f, err := New(sp)
		if err != nil {
			return nil, err
		}
		s.entries = append(s.entries, &Entry{Spec: sp, Forge: f})
	}
	return s, nil
}

// Match picks the forge for a repo. A known remote host must be one of the
// forge's hosts; an unknown host (an issue named in a prompt, with no remote
// to go by) takes the first forge whose owners admit the repo. Nil when none.
func (s *Set) Match(host, repo string) *Entry {
	if s == nil {
		return nil
	}
	for _, e := range s.entries {
		if (host == "" || e.hasHost(host)) && e.OwnerAllowed(repo) {
			return e
		}
	}
	return nil
}

// HostKnown reports whether any forge serves the remote host.
func (s *Set) HostKnown(host string) bool {
	if s == nil {
		return false
	}
	for _, e := range s.entries {
		if e.hasHost(host) {
			return true
		}
	}
	return false
}

// Empty reports whether no forge is configured.
func (s *Set) Empty() bool { return s == nil || len(s.entries) == 0 }
