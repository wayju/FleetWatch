# Configuration

Settings resolve in three layers: built-in defaults, then a config file, then environment variables, which win. The file is **strict JSON**: an unknown key is an error, and there are no comments, so a misspelt key cannot silently do nothing (ADR-004). It holds no secrets. `FLEET_TOKEN`, `GITEA_TOKEN` and `FLEET_RECAP_KEY` stay in the environment, and a file that names one fails to load.

`FLEET_CONFIG` names the file and it must load, else the service exits 1. With it unset, `$XDG_CONFIG_HOME/fleet/config.json` (`~/.config/fleet/config.json`) is read if present, and a present-but-broken file is an error. With no file the profile is the zero-config default: runtime `proc` (plus `tmux` when a `tmux` binary is on `PATH`) with agent `claudecode`. A leading `~` in a path expands against the resolved home (`FLEET_HOME`, else the file's `home`, else the process's).

The startup log prints every effective setting and whether it came from the default, the file or the environment.

Ready-made files are in `examples/config/`: `zero-config.json`, `tmux.json`, `herdr.json`, `hookfile.json`, `hub.json` and `full.json` (models, recap, prices and a forge). A Go test loads every file in that directory through the strict loader, so they cannot rot.

## Annotated reference

The real file is plain JSON with no comments; the comments below are for reading.

```jsonc
{
  "version": 1,                       // required, and must be 1
  "host": "my-laptop",                // default: the hostname           (FLEET_HOST)
  "home": "/home/me",                 // default: $HOME                  (FLEET_HOME)
  "poll": "2s",                       // (FLEET_POLL)
  "window": "2h",                     // transcripts touched within this are tailed (FLEET_WINDOW)
  "recent": "30m",                    // an ended session stays this long (FLEET_RECENT)
  "rescan": "10s",                    // transcript directory walk cadence (FLEET_RESCAN)
  "show_text": true,                  // false removes every free-text field (FLEET_SHOW_TEXT)
  "server": {
    "port": 8909,                     // 1-65535 (FLEET_PORT)
    "bind": "127.0.0.1",              // listen address (FLEET_BIND)
    "allowed_hosts": ["fleet.example.com:8909"],  // Host headers admitted; default localhost:<port>, 127.0.0.1:<port> (FLEET_ALLOWED_HOSTS)
    "max_subscribers": 32,            // concurrent /events streams, at least 1 (FLEET_MAX_SUBSCRIBERS)
    "memory_limit_mib": 0             // Go soft memory limit in MiB; 0 (default) sets none; GOMEMLIMIT wins
  },
  "cost": { "prices": "/etc/fleet/prices.json" },   // default: none, nothing priced (FLEET_PRICES_FILE)
  "models": [                         // first match wins; default: none
    { "match": "example-large-*",     // path.Match glob over the transcript's model id; write a literal bracket as \\[
      "context_window": 200000,       // tokens, positive
      "extended_window": 1000000,     // optional: the window once usage passes context_window; must exceed it
      "class": "large" }              // optional: display class, board model_class
  ],
  "recap": {                          // default: off
    "url": "https://gateway.example.com:4000",   // (FLEET_RECAP_URL)
    "model": "my-summary-model",      // no default (FLEET_RECAP_MODEL)
    "every": "5m",                    // positive duration (FLEET_RECAP_EVERY)
    "allow_insecure": false },        // a plain-http url is accepted only to loopback unless this is true
  "runtimes": [                       // first to report a pid owns it; at least one when the file lists adapters
    { "type": "proc", "proc_root": "/proc" },           // (FLEET_PROC_ROOT)
    { "type": "tmux",                                   // panes, plus state from the screen
      "proc_root": "/proc",                             // (FLEET_PROC_ROOT)
      "binary": "tmux",                                 // default "tmux" on PATH
      "sockets": ["/tmp/tmux-1000/default"],            // default: tmux's own default socket
      "timeout": "2s",                                  // per tmux call
      "scrape": true,                                   // default false; needs "rules"
      "scrape_lines": 20,                               // lines of the pane read, 1-1000
      "rules": "~/.config/fleet/tmux-rules.json" },
    { "type": "herdr",                                  // read-only
      "socket": "~/.config/herdr/herdr.sock",           // (HERDR_SOCKET_PATH, herdr's own variable)
      "rank": 25 },                                     // screen-detected state, above 10 and below 30
    { "type": "hookfile",                               // the fleet-state plugin's state files
      "dir": "~/.local/state/fleet/sessions",           // default: $XDG_STATE_HOME/fleet/sessions (FLEET_HOOKFILE_DIR)
      "fresh": "1h" }                                   // how long a write stays authoritative
  ],
  "agents": [                         // at least one when the file lists adapters
    { "type": "claudecode", "config_dirs": ["~/.claude"] }   // the first is the default
  ],
  "forges": [                         // which server answers for a repo's remote host
    { "kind": "gitea",                // gitea | github
      "host": "git.example.com",      // the remote host that selects this forge (github: github.com)
      "aliases": ["git.internal.example.com"],   // more remote hosts served by the same forge
      "api_url": "https://git.example.com",      // required for gitea; github: https://api.github.com
      "web_url": "https://git.example.com",      // default: api_url (github: https://github.com)
      "token_env": "GITEA_TOKEN",     // or "token_file": "~/.config/fleet/gitea.token", not both; neither: no token
      "owners": ["my-org"] },         // empty or absent: every owner
    { "kind": "github" }              // public repos work without a token
  ],
  "worktrees": {
    "roots": [{ "path": "~/src", "depth": 1 }]   // at least one root; depth 1-4
  },
  "issues": {
    "branch_pattern": "^(?:feature|fix|chore)/(\\d+)-"   // must compile and have exactly one capture group
  },
  "hub": {                            // merge other hosts' boards into this one; see hub.md
    "local": true,                    // also collect this host; default false
    "max_event_bytes": 8388608,       // largest single event from an upstream; 65536 to 268435456, default 8 MiB
    "upstreams": [
      { "name": "build-box",          // required, unique; labels the upstream in "upstreams"
        "url": "https://build-box.example.com:8909",
        "token_file": "~/.config/fleet/build-box.token",   // required; the upstream's token
        "allow_insecure": false }     // http:// is accepted only to loopback unless this is true
    ]
  }
}
```

## Rules the loader enforces

| Rule | Detail |
|---|---|
| Strict keys | Unknown keys, trailing data and a `version` other than 1 are errors, in the top level and in every section and adapter entry. |
| Adapter lists | `runtimes` and `agents` are given together: a file that names one list must name the other, and neither may be empty. A `type` that no registered adapter claims fails with the list of known types. |
| Durations | `poll`, `window`, `recent`, `rescan`, `recap.every`, `timeout` and `fresh` use Go duration syntax (`2s`, `30m`) and must be positive. A malformed environment override is ignored; a malformed file value is an error. |
| Cleartext | A hub upstream or recap `url` that is plain `http://` to a non-loopback host is an error unless `allow_insecure` is true, which also logs a warning at start. |
| Tokens | A forge names its token by `token_env` or `token_file`. The value is read once at startup and never logged. Hub tokens come only from `token_file`. |
| Hub | A `hub` section needs at least one upstream, or `local` true. Upstream names are required and unique. |
| Forge hosts | No remote host may be claimed by two forges. |
| tmux scrape | `scrape: true` without `rules` is an error. The rules file is strict JSON: `{ "<agent type>": [ { "state", "reason", "regex" } ] }`, tried in order, first match wins. A bad regex or state fails the load. |

## Environment variables

Variables named in the reference above override the file. These are environment-only or have a special role.

| Variable | Default and effect |
|---|---|
| `FLEET_CONFIG` | unset: look in `~/.config/fleet/`, else the zero-config default |
| `FLEET_TOKEN` | unset: a token is generated into `$XDG_STATE_HOME/fleet/token` (0600) and reused |
| `FLEET_RECAP_KEY` | unset: no summaries |
| `GITEA_URL`, `GITEA_WEB_URL`, `GITEA_TOKEN` | override the first `gitea` forge, or build one when the file has none (`GITEA_URL` is required then). Unset: nothing. The token needs the `read:issue` scope |
| `FLEET_GITEA_OWNERS` | comma list replacing that forge's `owners` |
| `FLEET_GITEA_HOSTS` | comma list replacing that forge's `aliases` |
| `GOMEMLIMIT` | the standard Go variable; it wins over `server.memory_limit_mib` |

## Behaviour behind the keys

**Nothing is compiled in** (ADR-005). No model name, price, context window, host, owner or recap model is built into the binary, and a Go test fails the build on a model-family literal in shipped code. With an empty config the fleet shows tokens with `unpriced: true`, no context percentage, no class and no issue titles. Board sessions carry `model_class`, and each cost `by_model` row carries it too, both taken from the `models` entry that matches the model id (omitted when none does).

**Cost.** Each session carries `cost` (`usd`, `partial`, `priced_as_of`, `tokens`, `by_model`) and the board carries `cost` (`usd`, `by_model`, `priced_as_of`): what the token usage would cost at API list prices. Sessions often run on a plan, so this compares sessions and models and is not a bill. Prices are config, not a live pull: `cost.prices` names the file, and `examples/prices.json` is a dated sample (`as_of`) that will go stale and is not maintained. A model can carry a `long_context` tier applied per message above its `threshold` prompt tokens. With no file, or a model missing from it, tokens are counted with `usd: 0` and `unpriced: true`. A price file that is missing or invalid is logged and ignored. The accumulator reads every transcript of a session, main and subagents, from byte 0 in the background (two files at a time), then only appended bytes; `partial` is true until that first read finishes.

**Recap.** `recap.summary` on each live session is written outside the session (ADR-009). FleetWatch sends a redacted digest (requests, recent activity, subagents, latest reply) to an OpenAI-compatible chat endpoint you name and keeps the answer. One call runs at a time, only for sessions that moved, at most every `recap.every`. There is no default model: recap stays off until `recap.url`, `recap.model` and `FLEET_RECAP_KEY` are all set. The digest leaves the machine, so point it at an endpoint you trust.

**Forges.** A repo's remote host selects its forge, so one board can mix Gitea and GitHub. `owners` limits which owners' repos are looked up and authenticated to. A reference named in a prompt, with no remote host to go by, goes to the first forge whose owners admit it; a remote host no forge serves is linked to its own web UI without a lookup. The token is sent only to `api_url`, and redirects are refused. GitHub uses REST v3 and honours `Retry-After` and `X-RateLimit-*`: once the budget is spent it makes no calls until the reset (capped at 15 minutes), which matters at the 60 requests an hour an unauthenticated client gets. Merged state for a worktree's branch needs a token on Gitea; GitHub needs none for public repos. With no `forges` the board shows issue numbers and links but no titles or merged state.

**Worktree roots.** Without a `worktrees` section there are no roots and the worktree view is off. Name where your repos live to turn it on. Depth 1 means repos sit directly under the path, 2 means one grouping directory between. A directory holding a `.git` directory ends the descent, and symlinks are never followed.

**Branch pattern.** Without an `issues` section the pattern is `(?:^|/)(\d+)[-_]`, which reads `123-fix`, `feature/123-fix` and `me/123_fix`. Numbers above 999999 are ignored.

**Server.** `allowed_hosts` must name every `Host` header clients send, including a reverse proxy's public name. In a container `bind` must be `0.0.0.0`; see [security.md](security.md) before doing that anywhere but a published loopback port.
