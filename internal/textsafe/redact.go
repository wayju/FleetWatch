// Package textsafe holds the text guards every free-text field passes through:
// redaction of credential shapes and clipping.
package textsafe

import (
	"regexp"
	"strings"
)

// Prompts and commands can carry a pasted credential, and this service's
// output leaves the host. These patterns blank the common shapes before any
// text leaves the process; FLEET_SHOW_TEXT=0 drops free text altogether.
//
// Order matters: the multi-line and flag-shaped patterns run first so a later,
// broader pattern never sees half of a secret.
var redactions = []struct {
	re   *regexp.Regexp
	with string
}{
	// A whole PEM private key, or from its header to the end of the string when
	// the text was truncated before the footer.
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----.*?(?:-----END [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----|\z)`), "[redacted]"},
	// NAME=value / NAME: value where the name ends in a credential word. The
	// first group has no separator requirement (apikey, mytoken); the second
	// needs one (AWS_SECRET_ACCESS_KEY, DB_PASS, X_AUTH, SVC_CREDENTIALS), so
	// "bypass" and "keyboard" are left alone.
	{regexp.MustCompile(`(?i)\b([a-z0-9_]*(?:api[_-]?key|token|secret|password|passwd|pwd))(["']?\s*[:=]\s*)["']?[^\s"'&]{6,}`), "$1$2[redacted]"},
	{regexp.MustCompile(`(?i)\b([a-z0-9_]*[_-](?:key|pass|auth|credentials?))(["']?\s*[:=]\s*)["']?[^\s"'&]{6,}`), "$1$2[redacted]"},
	// Space-separated CLI flags: --password hunter2, --token abc.
	{regexp.MustCompile(`(?i)(--(?:password|passwd|pwd|pass|token|secret|api-?key|access-?key|secret-?key|client-?secret|auth-?token|bearer|private-?key)\s+)["']?[^\s"']+`), "${1}[redacted]"},
	// mysql-style -pSECRET (attached), only on a mysql/mariadb command line:
	// -p means a port, a path or a pattern in plenty of other tools.
	{regexp.MustCompile(`(?i)(\b(?:mysql|mariadb)\w*\b[^\n]*?\s)-p\S+`), "${1}-p[redacted]"},
	// sshpass -p secret.
	{regexp.MustCompile(`(?i)(\bsshpass\b[^\n]*?\s-p\s*)["']?[^\s"']+`), "${1}[redacted]"},
	// curl -u user:pass, --user, --proxy-user.
	{regexp.MustCompile(`(?i)(\bcurl\b[^\n]*?\s(?:-u|--user|--proxy-user)[\s=]*)["']?[^\s"']+`), "${1}[redacted]"},
	// "Authorization: token X", "Bearer X": only a credential-length value, so
	// prose like "token usage is high" survives.
	{regexp.MustCompile(`(?i)\b(authorization:?\s*(?:bearer|token|basic)?|bearer|token)\s+[A-Za-z0-9._~+/=-]{16,}`), "$1 [redacted]"},
	// Provider token shapes.
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`), "[redacted]"},
	{regexp.MustCompile(`\b[sr]k_(?:live|test)_[A-Za-z0-9]{8,}`), "[redacted]"},
	{regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{8,}`), "[redacted]"},
	{regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), "[redacted]"},
	{regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{12,}`), "[redacted]"},
	{regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{30,}`), "[redacted]"},
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`), "[redacted]"},
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`), "[redacted]"},
	{regexp.MustCompile(`(?i)(X-Amz-(?:Signature|Credential|Security-Token)=)[^&\s"']+`), "${1}[redacted]"},
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]+`), "[redacted]"},
	{regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`), "://[redacted]@"},
}

// longRunRe finds the candidates for the entropy catch-all.
var longRunRe = regexp.MustCompile(`[A-Za-z0-9_-]{32,}`)

// looksRandom decides whether a long [A-Za-z0-9_-] run is a credential rather
// than a slug. Slugs, branch names and UUIDs are made of short words split by
// - or _; random base64/hex tokens carry a long stretch with no separator that
// mixes letters and digits.
func looksRandom(run string) bool {
	for _, seg := range strings.FieldsFunc(run, func(r rune) bool { return r == '-' || r == '_' }) {
		if len(seg) < 16 {
			continue
		}
		letters, digits := 0, 0
		for _, c := range seg {
			switch {
			case c >= '0' && c <= '9':
				digits++
			default:
				letters++
			}
		}
		if letters > 0 && digits >= 3 {
			return true
		}
	}
	return false
}

func Redact(s string) string {
	for _, r := range redactions {
		s = r.re.ReplaceAllString(s, r.with)
	}
	s = redactHex(s)
	// The catch-all must not undo the commit-id exemption in redactHex.
	return replaceRuns(s, longRunRe, func(at, end int) bool {
		return !looksRandom(s[at:end]) || keepAsCommit(s, at, end)
	})
}

// Clip collapses whitespace and cuts s to n runes, ending in an ellipsis.
func Clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
