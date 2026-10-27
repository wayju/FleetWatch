# Hub mode

A hub is a FleetWatch instance that merges the boards of other instances into one, so a single dashboard can watch several machines. It serves the merged board in the usual shape on `/v1/fleet` and `/v1/events`. Hub mode is configured by the `hub` section (ADR-008), and `examples/config/hub.json` is a starting point. With no `hub` section nothing changes. A `hub` section with no upstreams and `local` false is a load error.

## Model

The hub pulls. Each upstream is an ordinary FleetWatch instance, and the hub follows its `/v1/events` stream with a bearer token. Upstreams need no knowledge of the hub and can sit behind any reverse proxy that passes the `Authorization` header. The hub reads both `board` and `tick` events.

| Key | Meaning |
|---|---|
| `hub.local` | also collect this host. Without it no runtimes or agents are started. Default false. |
| `hub.upstreams[].name` | unique label for the upstream. Sessions are keyed by this name, not by the hostname the upstream reports, since inside a container the hostname is often an id. |
| `hub.upstreams[].url` | base URL; `/v1/events` is appended. |
| `hub.upstreams[].token_file` | required. The upstream's token, read once at startup, never logged and never in an error message. |
| `hub.upstreams[].allow_insecure` | admit plain `http://` to a non-loopback host. Default false. |
| `hub.max_event_bytes` | largest single event accepted from an upstream, 64 KiB to 256 MiB, default 8 MiB. A larger event means a broken or hostile upstream. |

Tokens are never inline in the config. Redirects are never followed, because Go would resend the `Authorization` header to a same-host redirect, including a downgrade from https to http.

## Merging

Every session carries its `host`: the upstream's configured `name`, or this host's name for local sessions. A session id gains a `<host>:` prefix only where two hosts hold the same id. Counts, accounts, branches, worktrees (each with a `host`), the log, links and cost totals are merged. A link the sending host could not place (`external`) resolves to a session on another host when the id matches exactly, or as a unique prefix of 8 or more characters.

## Stale and down

The board gains `upstreams: [{name, state, last_seen, error}]`, and `/v1/health` carries the same list.

| State | Meaning |
|---|---|
| `live` | the stream is open |
| `stale` | the stream dropped. The upstream's sessions stay, each marked `stale: true`, and are left out of the counts, for the `recent` window. |
| `down` | the window passed. The upstream's sessions are removed. |

A host that restarts therefore blinks rather than vanishes. The hub reconnects with backoff from 1 to 30 seconds, and drops and redials a stream that has been silent for 60 seconds. Because `last_seen` moves on every tick, a hub publishes a `board` event on every poll, which a single host does not.

## Chaining and exposure

The hub's own `FLEET_TOKEN` guards its routes as on any instance, so a hub can feed another hub. The same exposure rules apply as for any instance; see [security.md](security.md). A hub holds one token per upstream, so keep the token files at mode 0600.
