package textsafe

import (
	"strings"
	"testing"
)

func TestRedactShapes(t *testing.T) {
	for _, s := range []string{
		"export GITEA_TOKEN=abcdef123456",
		"curl -H 'Authorization: token 0123456789abcdef'",
		"key sk-ant-api03-abcdefghijklmnopqrstuv",
		"ghp_abcdefghijklmnopqrstuvwxyz0123",
		"https://bob:hunter22@git.example.test/x",
	} {
		out := Redact(s)
		if !strings.Contains(out, "[redacted]") {
			t.Errorf("not redacted: %q -> %q", s, out)
		}
	}
	for _, s := range []string{"run npm test in src/", "token usage is high", "reset the password flow"} {
		if out := Redact(s); out != s {
			t.Errorf("over-redacted: %q -> %q", s, out)
		}
	}
}

func TestRedactHardening(t *testing.T) {
	// Each sample carries a secret that must not survive.
	secrets := map[string]string{
		"mysql -u root -pS3cretPass123 mydb":                             "S3cretPass123",
		"mysqldump --host db -uroot -phunter2hunter2 all":                "hunter2hunter2",
		"sshpass -p letmein123 ssh root@host":                            "letmein123",
		"curl -u admin:s3cr3tvalue https://x.lan/api":                    "s3cr3tvalue",
		"curl -sS --user bob:pa55word https://x.lan":                     "pa55word",
		"login --password correcthorse now":                              "correcthorse",
		"tool --token abc123def456":                                      "abc123def456",
		"AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY": "wJalrXUtnFEMI",
		"export DB_PASS=sup3rsecret":                                     "sup3rsecret",
		"X_AUTH: abcdefghij12":                                           "abcdefghij12",
		"SVC_CREDENTIALS=eyAiYSI6ICJiIn0":                                "eyAiYSI6ICJiIn0",
		"-----" + "BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\nabcd\n-----" + "END RSA PRIVATE KEY-----": "MIIEowIBAAKCAQEA",
		"key: -----" + "BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA\ntruncated here":          "b3BlbnNzaC1rZXktdjEAAAAA",
		"stripe sk_" + "live_4eC39HqLyjWDarjtT1zdp7dc":                                                     "4eC39HqLyjWDarjtT1zdp7dc",
		"slack xox" + "b-123456789012-abcdefghijkl":                                                        "abcdefghijkl",
		"slack xox" + "p-123456789012-abcdefghijkl":                                                        "abcdefghijkl",
		"aws AKIAIOSFODNN7EXAMPLE":                                                                         "AKIAIOSFODNN7EXAMPLE",
		"gitlab glpat-xxxxxxxxxxxxxxxxxxxx":                                                                "glpat-xxxxxxxxxxxxxxxxxxxx",
		"google AIzaSyA-1234567890abcdefghijklmnopqrstu":                                                   "AIzaSyA-1234567890abcdefghijklmnopqrstu",
		"https://b.s3.amazonaws.com/o?X-Amz-Signature=0a1b2c3d4e5f&X-Amz-Date=1":                           "0a1b2c3d4e5f",
		"random Zk3nQ8vT1pLx9RmB2wYc7HdF4sJa6GuE0":                                                         "Zk3nQ8vT1pLx9RmB2wYc7HdF4sJa6GuE0",
		"b64 a1B2c3D4e5F6g7H8i9J0k1L2m3N4o5P6q7R8":                                                         "a1B2c3D4e5F6g7H8i9J0k1L2m3N4o5P6q7R8",
	}
	for in, secret := range secrets {
		out := Redact(in)
		if strings.Contains(out, secret) || !strings.Contains(out, "[redacted]") {
			t.Errorf("not redacted: %q -> %q", in, out)
		}
	}
	// Ordinary text, paths and identifiers must come through untouched.
	for _, s := range []string{
		"feature/3206-claude-fleet",
		"feature/3206-claude-fleet-redaction-hardening-and-host-allowlist",
		"/home/user/src/example-stack/.claude/worktrees/feature/claude-fleet/claude-fleet/redact.go",
		"session f311f2d8-5d04-4a3d-81c8-a4a8292e5102 resumed",
		"docker run -p 8080:80 nginx",
		"find . -perm 644 -path ./x -print",
		"ssh -p 2222 host",
		"bypass the keyboard shortcut",
		"reset the password flow",
		"token usage is high",
		"curl -fsS http://localhost:8909/health",
		"mysql --version and --password-file /etc/x",
		"fix: resolve combat crash #42 (refs #3206)",
		"claude-fleet-build-deploy.yml",
	} {
		if out := Redact(s); out != s {
			t.Errorf("over-redacted: %q -> %q", s, out)
		}
	}
}

func TestRedactGitSHAPolicy(t *testing.T) {
	const sha = "9fceb02d0ae598e95dc970b74767f19372d61af8"
	kept := []string{
		"commit " + sha,
		"merged commit " + sha + " into main",
		"git show " + sha,
		"git cherry-pick -x " + sha,
		"https://git.example/o/r/commit/" + sha,
		"https://git.example/o/r/commits/" + sha,
		"HEAD " + sha,
		"sha: " + strings.ToUpper(sha),
		"git log 1111111..." + sha,
		"line one\ncommit " + sha,
	}
	for _, s := range kept {
		if out := Redact(s); out != s {
			t.Errorf("commit id redacted: %q -> %q", s, out)
		}
	}
	redacted := []string{
		sha,                        // bare: also the shape of a Gitea access token
		"token is " + sha,          // no git cue
		"commit\n" + sha,           // the cue is on another line
		"commit " + sha + "0",      // 41 digits
		"commit " + sha + sha[:24], // 64 digits
		"digest " + sha + sha[:24],
		"token=" + sha,
	}
	for _, s := range redacted {
		if out := Redact(s); !strings.Contains(out, "[redacted]") || strings.Contains(out, sha[:20]) {
			t.Errorf("not redacted: %q -> %q", s, out)
		}
	}
}
