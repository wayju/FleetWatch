// Package httpsafe holds HTTP client policy shared by the clients that send a
// bearer token.
package httpsafe

import "net/http"

// RefuseRedirect is an http.Client CheckRedirect that never follows a redirect.
// The 3xx answer is returned as it is, so the caller sees a non-200 status.
// Go would otherwise resend Authorization on a same-host redirect, including a
// downgrade from https to http.
func RefuseRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
