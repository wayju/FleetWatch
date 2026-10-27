# Security

FleetWatch reads every agent transcript on a machine and serves prompts, replies and file paths over HTTP. This page says what it reads, what leaves the process, what the token protects and what it does not, and how to deploy it. To report a vulnerability, see `SECURITY.md`.

## What it reads

| Source | What is taken |
|---|---|
| `/proc/<pid>/{stat,cmdline,cgroup}` and `/proc/stat` | process list, start time, command line, boot time. Never `environ`, the `cwd` link, `fd` or `exe` (ADR-007). |
| Agent transcripts | prompts, replies, tool calls and targets, token usage, branch, model, subagents |
| The agent's `.claude.json` | the account email, name and organisation. Other fields, which include credentials, are not kept. |
| tmux panes (opt in) | the last lines of each agent's screen, matched against your rules |
| herdr socket | pane state, over a read-only method allowlist |
| Session state files | the state, reason and pane |
| Git checkouts | remote, branch, commit and version file of repos under worktree roots you name |

State, transcript and generated-token files are read through `fsafe`, which refuses symlinks and FIFOs and caps size. Files the operator names in the config (the config file, price file, tmux rules and token files) are read as ordinary files. Transcripts are read as lines of at most 2 MiB, 4 MiB per poll, and a first open of a larger file starts at its last 4 MiB.

## What leaves the process

| Destination | What | Control |
|---|---|---|
| Clients of `/v1/fleet` and `/v1/events` | the board: prompts, replies, paths, branches, costs | the bearer token, the bind address, `show_text` |
| A recap endpoint, if configured | a redacted digest of a session | off unless `recap.url`, `recap.model` and `FLEET_RECAP_KEY` are all set |
| A forge, if configured | repository owner, name and issue or PR number | only to owners you list; the token goes only to `api_url` |
| A hub's upstreams | the bearer token, in the `Authorization` header | https unless the entry sets `allow_insecure` |

Account emails never leave the process; the board carries a short hash and, with `show_text` on, the display name and organisation. Setting `show_text: false` (`FLEET_SHOW_TEXT=0`) removes every free-text field from the board and the event stream: recap, prompts, replies, session titles, tool targets, message previews, agent descriptions, issue titles and log detail. State, tool names, branch, issue numbers and counts remain.

## Threat model

FleetWatch assumes a single trusted user on the machine and a network it does not trust.

| Threat | Mitigation |
|---|---|
| A remote or LAN client reads the board | loopback bind by default; a bearer token compared in constant time; the data routes fail closed with 503 if no token is available |
| A web page in your browser reaches the board through DNS rebinding | the `Host` header must be in `allowed_hosts`, else 421 |
| A client exhausts the service | a cap on `/events` subscribers (503 beyond), a 10 second write deadline, header and idle timeouts |
| A credential pasted into a prompt appears on the board | redaction and clipping of every free-text field |
| A planted symlink or FIFO redirects a read or hangs the poll loop | `fsafe` opens with `O_NOFOLLOW` and `O_NONBLOCK` and requires a regular file; this includes the token file |
| A hostile upstream or forge response | event size cap, no redirects, timeouts, the token sent only to its configured host |
| A forged state file | files must be regular, small, schema-valid and named for their session; a file older than its process is ignored; but see below |
| An adapter does something to a session | adapters are read-only by design, and tests refuse the writes and `/proc` reads that would break that |

### What it does not protect against

- **Another process under the same user account.** Anyone who can read your files can already read the transcripts, the generated token and the state directory. The token protects the network path, not the account.
- **A writer to the state directory.** A process that can write `$XDG_STATE_HOME/fleet/sessions/` can set any session's displayed state. That affects what the board says, not what the agents do.
- **Cleartext transport.** FleetWatch serves plain HTTP. The token travels in a header, so anything beyond a trusted loopback link needs TLS from a reverse proxy.
- **Redaction misses.** The patterns below catch common credential shapes. They are not a guarantee, which is why `show_text: false` exists.

## The token

`/fleet` and `/events` (and their `/v1` forms) need `Authorization: Bearer <token>`. `/health`, `/liveness` and `/version` do not. With `FLEET_TOKEN` unset the service generates a random 32-byte token on first run and stores it at `$XDG_STATE_HOME/fleet/token` (else `~/.local/state/fleet/token`) through a private temporary file and a rename, so the file is never wider than 0600 and a reader never sees it half written. Later runs reuse it. The log names the path, never the value. If no token can be obtained the data routes answer 503; they never serve open. The environment variable always wins.

`/health` is unauthenticated and includes the effective profile (config path and adapter names), the last poll error and the recap error. Do not publish it beyond the network that may see that.

## Deployment rules

1. **Loopback by default.** Do not change `server.bind` unless you need to. A container must bind `0.0.0.0` because a published port cannot reach a loopback bind; publish it on the host's loopback (`-p 127.0.0.1:8909:8909`) unless a proxy fronts it.
2. **TLS only through a reverse proxy.** FleetWatch does not terminate TLS. Put it behind a proxy that does, and pass the `Authorization` header through.
3. **Never on a public interface.** The board is your working notes. If it must be reachable over the internet, put authentication in front of it as well as the token.
4. **Name the proxy in `allowed_hosts`.** Requests are admitted by their `Host` header. The proxy's public name and port must be listed, or every request gets 421.

A worked Caddy example:

```
fleet.example.com {
    reverse_proxy 127.0.0.1:8909 {
        flush_interval -1
    }
}
```

and the matching config:

```json
{ "version": 1, "server": { "allowed_hosts": ["fleet.example.com"] } }
```

The same with nginx. Buffering must be off or the event stream stalls:

```
server {
    listen 443 ssl;
    server_name fleet.example.com;
    location / {
        proxy_pass http://127.0.0.1:8909;
        proxy_set_header Host $host;
        proxy_set_header Authorization $http_authorization;
        proxy_buffering off;
        proxy_read_timeout 1h;
    }
}
```

Caddy sends the original `Host` and passes `Authorization` by default. When the proxy listens on a non-default port, the `Host` header includes it and `allowed_hosts` must too.

## Redaction

Free text is redacted and clipped before it reaches any board field. The patterns, in order:

- whole PEM private key blocks, or from the header to the end of a truncated string
- `NAME=value` and `NAME: value` where the name ends in `api_key`, `token`, `secret`, `password`, `passwd` or `pwd`, or in `_key`, `_pass`, `_auth` or `_credentials`
- space-separated CLI flags such as `--password x` and `--token x`
- `mysql -pSECRET`, `sshpass -p`, and `curl -u user:pass`
- `Authorization`, `Bearer` and `token` followed by a credential-length value
- provider token shapes: `sk-`, `sk_live_`, `xox*-`, `AKIA`/`ASIA` keys, `glpat-`, `AIza`, `ghp_`-style and `github_pat_` tokens, JWTs, and signed URL parameters
- the password in `scheme://user:pass@host`
- a catch-all for long random-looking runs, and for hex runs of 40 or more digits

**False-positive policy.** The catch-alls err towards hiding. A 40-digit hex run is also the shape of a Gitea access token, so a bare one is redacted. The one exemption is a 40-hex git commit id that sits in a git context on the same line (`commit`, `sha`, `git show`, a `/commit/` path or a `..` range), where the board has reason to show it. A hex run longer than 40 is never exempt.

## Plugin

The Claude Code plugin writes state files and nothing else. Its script reads the hook payload on stdin, never fails a hook, and creates files at mode 0600 in a 0700 directory. It needs `jq`.
