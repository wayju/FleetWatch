# Changelog: FleetWatch

All notable changes to FleetWatch are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[semantic versioning](https://semver.org/).

## [Unreleased]

## [0.1.5] - 2026-10-28

### Fixed

- A background subagent woken by `SendMessage` after it handed back now shows
  as working while it works. It used to stay "done", so its box and its message
  arrows disappeared and the header counted no subagents.

## [0.1.4] - 2026-10-28

### Fixed

- Messages between a session and its own subagents, between sibling subagents
  and hand-backs from a subagent are now drawn on `/`. They used to be dropped
  because both ends sit in one card. The line runs inside the card, down the
  inner left gutter (or through the gap between subagent columns), from the
  sender's box to the receiver's, with the usual pill and class colour. A
  cross-card message whose end is a visible subagent now lines its port up with
  that subagent's box. A folded or hidden box falls back to the card header, and
  the narrow-screen message list names the agents at each end.

## [0.1.3] - 2026-10-28

### Added

- A browser page at `/` renders the board live, so a standalone install has
  something to look at. It is served from files embedded in the binary under a
  policy that allows no inline script, and is built from `web/`
  (`@fleetwatch/ui`, plain JavaScript, no dependencies).
- Browser sign-in: `POST /v1/session` exchanges the token for an `HttpOnly`,
  `SameSite=Strict` session cookie holding a random id, and `DELETE
  /v1/session` signs out. `/v1/fleet` and `/v1/events` accept the cookie or the
  bearer. Login requires an `Origin` that matches the request host (ADR-012).
- Config `ui.enabled` (default true) switches the page and the session route
  off, and `ui.theme_file` serves an operator's stylesheet in place of the
  built-in theme.
- Each release attaches `fleetwatch-ui-<version>.tgz`, the page library as an
  npm tarball, so another host can render the same board.
- The page is split into `fleet.css` (structure, `--fl-*` tokens only) and
  `theme.css` (colours, fonts and model-class fills).

### Changed

- A model's colour on the page comes from the board's `model_class` and the
  theme. The page no longer guesses a class from a model name.
- The worktree removal hint on the page is `git worktree remove`.
- Page options `heading`, `backTitle` and `emptyText` let a host head the page
  apart from its `<title>`, give the back link a tooltip and word the empty state.

## [0.1.2] - 2026-10-27

### Changed

- A release cut from a private repository is published as a prerelease without
  build provenance, since GitHub stores attestations only for public
  repositories on a personal account.

## [0.1.1] - 2026-10-27

Tagged but not published: its release run failed at the provenance step on the
private preview repository. The changes below ship in 0.1.2.

### Changed

- Sessions carry `agent_version`, the coding agent's own version. `claude_version`
  is still emitted with the same value, but it is deprecated and removed in 0.2;
  use `agent_version`.
- The default worktree roots are now none, so the worktree view is off until
  `worktrees.roots` names a root (formerly `~/src` at depth 1).
- The recap prompt calls the session a coding-agent session rather than a
  Claude Code session.
- Darwin binaries are dropped from the release matrix. FleetWatch is Linux only
  (or a mounted `/proc`); macOS is not supported in 0.1 because no `ps`-based
  runtime exists yet.

- Hub upstreams and the recap gateway must use `https://`. A plain `http://`
  URL is accepted only for a loopback host (127.0.0.0/8, `::1`, `localhost`)
  or when the entry sets `allow_insecure: true`, which logs a warning at start.
  Existing configs that point at a remote `http://` host need one of the two.
- The hub and the recap client no longer follow HTTP redirects, so the bearer
  token cannot be replayed to a redirect target. A redirect shows as an error.
- The largest event a hub accepts from an upstream drops from 64 MiB to 8 MiB.
  `hub.max_event_bytes` (64 KiB to 256 MiB) tunes it.
- The built-in Go memory limit (192 MiB) is removed. `server.memory_limit_mib`
  sets one; the default is none, and `GOMEMLIMIT` still wins.
- The redactor keeps a 40-hex git commit id when it follows a git cue on the
  same line (`commit`, `sha`, `git show`, `/commit/`, a `..` range). A bare
  40-hex run, which is also the shape of a Gitea token, and any longer hex run
  are still redacted.
- The generated token file is read without following symlinks, and a
  non-regular file at that path is an error.
- The Claude Code hook script validates the process id it records and writes
  timestamps with sub-second precision where `date` supports it.

## [0.1.0] - 2026-10-27

Tagged but never published; 0.1.1 is the first public release.

### Added

- A live board of every Claude Code session on a host, with the subagents under
  each one, served as JSON (`/v1/fleet`) and Server-Sent Events (`/v1/events`).
- Runtime adapters: `proc` (processes through `/proc`), `tmux` (panes, with
  optional screen scraping driven by a rules file), `herdr` (read-only, over
  herdr's local socket) and `hookfile` (state files written by the bundled Claude
  Code hook plugin or any tool following `schema/session-state.v1.json`).
- Agent adapter `claudecode`: transcripts, account, model, context use,
  subagents, messages between sessions, recap input and API-equivalent cost.
- Ranked state merge: native 50, hook 40, herdr screen 25, projection 30,
  pane 20, transcript 10, process 0. The highest-ranked unexpired signal wins.
- The `fleet-state` Claude Code plugin, which reports approval prompts,
  questions and stops exactly.
- Hub mode: merge several hosts' boards into one, with per-upstream health.
- Forges: Gitea and GitHub issue and merge state, selected by a repo's remote
  host. Token read once from the environment or a file.
- Optional LLM recap of each live session, off until a URL, model and key are
  all set.
- Strict-JSON configuration with environment overrides. Nothing site-specific
  (model names, prices, hosts, owners) is compiled in.
- `location` on each session: tmux's `session:window.pane` label.
- `name`, `version` and `commit` on `/version` and `/v1/version`. The version comes from
  the `cmd/fleetwatch/VERSION` file embedded in the binary.
- Release packaging: static binaries for linux on amd64 and arm64,
  a `SHA256SUMS` file and a linux/amd64 container image (arm64 image: later).

### Security

- Binds `127.0.0.1` by default. Exposing the board is an explicit `server.bind`.
- `/fleet` and `/events` need a bearer token, compared in constant time. With
  `FLEET_TOKEN` unset a random token is generated into
  `$XDG_STATE_HOME/fleet/token` (mode 0600) and its path, never its value, is
  logged.
- `Host` header allowlist (421), subscriber cap (503), write deadlines, and
  file reads that refuse symlinks and FIFOs.
- Free text is redacted and clipped, and `show_text: false` removes it entirely.
  Account emails never leave the process.
- Adapters are read-only: no keys, text or prompts are ever sent to a session.
  `/proc/<pid>/environ`, `cwd`, `fd` and `exe` are never read.
