# FleetWatch

FleetWatch is a live board of the AI coding-agent sessions on a machine (Claude Code today, through the `claudecode` adapter) and the subagents under each one. It shows which are working, which are waiting for you, which are stuck on an approval prompt, what each costs at API list prices, and which two are about to step on the same branch. It is a single static binary with no dependencies beyond the Go standard library, and it only reads: it never sends keys, text or prompts to a session.

It polls every 2 seconds and serves the board as JSON and as Server-Sent Events, pushing a new board only when something changed, with a `tick` in between. It also serves a page at `/` that renders the board live (see [docs/ui.md](docs/ui.md)); any other front end can render the same feed.

> **About this code.** FleetWatch was written almost entirely by AI coding agents under human direction and review. It is released as is, under the Apache License 2.0, with no warranty. **Users beware:** it reads every agent transcript on the machine and serves prompts, replies and file paths over HTTP, so read [docs/security.md](docs/security.md) and the code before you run it on a machine that holds anything you care about. Copying, forking and reproducing the approach are welcome; keep the notices in `LICENSE` and `NOTICE`.

## Quick start

You need Go 1.24 or later and a Linux machine (or a mounted `/proc`). There is no macOS support in 0.1.

```sh
go build -o fleetwatch ./cmd/fleetwatch
./fleetwatch
```

With no config file it reads `~/.claude`, finds Claude Code processes through `/proc` (plus tmux panes when a `tmux` binary is on `PATH`) and listens on `127.0.0.1:8909`. A token is generated on first run and stored at `$XDG_STATE_HOME/fleet/token` (else `~/.local/state/fleet/token`, mode 0600). The log names the path and never the value.

```sh
curl -H "Authorization: Bearer $(cat ~/.local/state/fleet/token)" http://127.0.0.1:8909/v1/fleet
```

Open `http://127.0.0.1:8909/` in a browser and paste the same token once to see the board. The token trades for a session cookie; it is never kept in the page or the URL.

State from the transcript and the process list is good enough to tell working from idle. Two optional steps make it exact: reading the tmux screen for approval prompts, and installing the hook plugin, which reports approval, question and stop events as they happen. Both are described in [docs/adapters.md](docs/adapters.md) and [docs/session-state.md](docs/session-state.md), with ready-made configs in `examples/config/`.

## Adapters

The board is assembled from two kinds of adapter, each a package under `internal/`. A **runtime** finds agent processes and says what its host knows about them. An **agent** owns the transcripts and everything folded from them. `GET /health` reports the effective `profile`.

| Runtime | Finds sessions through | Adds | Needs |
|---|---|---|---|
| `proc` | `/proc/<pid>/{stat,cmdline,cgroup}` | live sessions | Linux, or a mounted `/proc` |
| `tmux` | `tmux list-panes -a`, walked to the agent pid | pane id as `address`, `location`; optionally state from the screen | the `tmux` binary and the server socket |
| `herdr` | herdr's local socket | pane placement and herdr's own state, with event-driven wake | herdr running |
| `hookfile` | `<dir>/<session_id>.json` | exact approval, question and stop state (rank 50) | the hook plugin, or any writer of the [state file](docs/session-state.md) |

| Agent | Source |
|---|---|
| `claudecode` | Claude Code's config directory: transcripts, `.claude.json`, `sessions/<pid>.json` |

[docs/adapters.md](docs/adapters.md) describes each one and how to write another.

## API

| Route | What |
|---|---|
| `GET /events` | SSE: `event: board` (full JSON) on change, `event: tick` otherwise |
| `GET /fleet` | the latest board as JSON |
| `GET /health` | 200 while passes are completing, else 503; includes the effective `profile` |
| `GET /liveness` | the process is up (use it as a container healthcheck) |
| `GET /version` | `{"name": "FleetWatch", "version": "0.1.4", "commit": "<build commit>"}` |

Every route is also served under `/v1` (`/v1/fleet`, `/v1/events`, `/v1/health`, `/v1/liveness`, `/v1/version`) with the same guards, so a client can pin the contract. `/fleet` and `/events` need `Authorization: Bearer <token>`; the three probe routes do not. The version is the `cmd/fleetwatch/VERSION` file embedded in the binary, unless the build sets `APP_VERSION` (other than `dev`) or a link-time stamp. The commit is `APP_COMMIT` or the stamp, else `unknown`.

## Documentation

| Document | Covers |
|---|---|
| [docs/architecture.md](docs/architecture.md) | the poll loop, the collector pass, adapter kinds, the ranked state merge |
| [docs/adapters.md](docs/adapters.md) | the built-in adapters, and how to write and register one |
| [docs/configuration.md](docs/configuration.md) | every config key and environment variable |
| [docs/security.md](docs/security.md) | threat model, what is read and what leaves, deployment rules |
| [docs/session-state.md](docs/session-state.md) | the hook state-file format, for agents that want to report exact state |
| [docs/hub.md](docs/hub.md) | merging several machines into one board |
| [docs/release.md](docs/release.md) | how a release is cut and verified |
| [docs/decisions/](docs/decisions/README.md) | architecture decision records |
| [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md), [AGENTS.md](AGENTS.md) | contributing, reporting a vulnerability, notes for AI agents |

## Install and run

From a release, download the binary for your platform and `SHA256SUMS`, check it with `sha256sum -c`, and run it. Releases are cut from `v<version>` tags and carry static binaries for Linux on amd64 and arm64, and a container image at `ghcr.io/wayju/fleetwatch`. See [docs/release.md](docs/release.md).

From source:

```sh
go vet ./... && go test ./...
go run ./cmd/fleetwatch
```

In a container the process runs as uid 1000 and needs the user's transcripts. Mount the config directory read-only, share the host's pid namespace for `proc`, and bind to all interfaces, because a published port cannot reach a loopback bind:

```sh
docker run --rm --pid=host --user 1000:1000 -p 127.0.0.1:8909:8909 \
  -e FLEET_BIND=0.0.0.0 -e FLEET_ALLOWED_HOSTS=127.0.0.1:8909,localhost:8909 \
  -v ~/.claude:/home/fleet/.claude:ro -e FLEET_HOME=/home/fleet \
  -v ~/.config/fleet:/etc/fleet:ro -e FLEET_CONFIG=/etc/fleet/config.json \
  ghcr.io/wayju/fleetwatch:0.1.4
```

Publish the port on `127.0.0.1` as above unless you have read the exposure rules in [docs/security.md](docs/security.md). `/health` answers 200 once a poll has completed, which makes it a usable container healthcheck alongside `/liveness`.

## Naming

The product is called FleetWatch. The binary and the container image are named `fleetwatch`; the `FLEET_*` environment variables, the config keys and the `fleet` state directories keep the shorter `fleet` name.

## Licence

Apache-2.0. See `LICENSE` and `NOTICE`. Changes are in `CHANGELOG.md`.
