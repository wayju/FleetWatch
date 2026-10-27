package forge

// Shared request plumbing: a short timeout, no redirects (the token must go
// only to the configured server), a body cap, and one place that clips and
// redacts a title before it can reach the board.

import (
	"encoding/json"
	"github.com/wayju/fleetwatch/internal/textsafe"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxBody = 4 << 20

func newHTTP() *http.Client {
	return &http.Client{Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// splitRepo validates "owner/name" and escapes each part for a URL path.
func splitRepo(repo string) (owner, name string, ok bool) {
	o, n, found := strings.Cut(repo, "/")
	if !found || o == "" || n == "" || strings.Contains(n, "/") {
		return "", "", false
	}
	return url.PathEscape(o), url.PathEscape(n), true
}

func cleanTitle(s string) string { return textsafe.Clip(textsafe.Redact(s), 140) }

// doJSON runs req and decodes a 200 body into out. The response is returned
// with its body closed, so a caller can read the status and headers.
func doJSON(c *http.Client, req *http.Request, out any) (*http.Response, error) {
	res, err := c.Do(req)
	if err != nil {
		return nil, ErrUnknown
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		return res, ErrUnknown
	}
	if json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(out) != nil {
		return res, ErrUnknown
	}
	return res, nil
}
