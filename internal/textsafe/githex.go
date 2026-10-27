package textsafe

import (
	"regexp"
	"strings"
)

// Long hex runs are blanked: SHA-256 digests, API keys and the like. A bare
// 40-hex run is also blanked, because that is the shape of a Gitea access token
// as well as of a git SHA-1. The one exemption is a 40-hex run that sits in a
// git context on the same line (see gitContextRe), where it is a commit id the
// board has reason to show. Anything longer than 40 is never exempt.
var hexRunRe = regexp.MustCompile(`(?i)\b[0-9a-f]{40,}\b`)

// gitContextRe matches the text just before a commit id: a word such as
// commit, sha, rev, merge, cherry-pick, revert or HEAD (with an optional
// separator), a /commit/ or /commits/ URL path, a git subcommand with its
// flags, or the ".." of a range.
var gitContextRe = regexp.MustCompile(`(?i)(?:\b(?:commits?|sha1?|rev|merge|cherry-pick|revert|head)[\s:=@#]*|/commits?/|\bgit\s+[\w-]+\s+(?:-\S+\s+)*|\.\.\.?)$`)

// isGitSHA reports whether s is exactly 40 hex digits.
func isGitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// inGitContext reports whether the run starting at s[at] follows a git cue on
// its own line.
func inGitContext(s string, at int) bool {
	line := s[strings.LastIndexByte(s[:at], '\n')+1 : at]
	return gitContextRe.MatchString(line)
}

// keepAsCommit reports whether the run at s[at:end] is a 40-hex commit id.
func keepAsCommit(s string, at, end int) bool {
	return end-at == 40 && isGitSHA(s[at:end]) && inGitContext(s, at)
}

// replaceRuns rewrites every match of re in s that keep does not exempt.
func replaceRuns(s string, re *regexp.Regexp, keep func(at, end int) bool) string {
	var b strings.Builder
	last := 0
	for _, m := range re.FindAllStringIndex(s, -1) {
		if keep(m[0], m[1]) {
			continue
		}
		b.WriteString(s[last:m[0]])
		b.WriteString("[redacted]")
		last = m[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

func redactHex(s string) string {
	return replaceRuns(s, hexRunRe, func(at, end int) bool { return keepAsCommit(s, at, end) })
}
