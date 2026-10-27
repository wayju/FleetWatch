# Adapters

An adapter is a package that tells FleetWatch something about agent sessions. There are two kinds, and the board is assembled from the ones named in the config (see [architecture.md](architecture.md)).

- A **runtime** (`internal/host/<name>`) finds agent processes and reports what its host knows about them: where a session sits, what state it is in, how full its context is.
- An **agent** (`internal/agent/<name>`) owns an agent's transcripts and folds them into session views.

Adapters are read-only (ADR-001). They never send keys, text or prompts to a session, and the guard tests fail the build if one tries.

## Built-in runtimes

| Type | Finds sessions through | Notes |
|---|---|---|
| `proc` | `/proc/<pid>/{stat,cmdline,cgroup}`, and `/proc/stat` for boot time | The base runtime. It never reads `environ`, the `cwd` link, `fd` or `exe` (ADR-007). |
| `tmux` | `tmux list-panes -a`, walked to the agent pid | Reports the pane id (`%N`) as `address`. Optionally scrapes the screen. |
| `herdr` | herdr's local socket | Reads pane placement and herdr's own state. Event-driven wake. |
| `hookfile` | `<dir>/<session_id>.json` | An observer only: it discovers nothing and annotates sessions found elsewhere. See [session-state.md](session-state.md). |

Sessions are keyed by pid. The first runtime in the list owns a pid, and later runtimes fill only the fields it left empty, so `[proc, tmux]` gives the `proc` session a pane address. When two runtimes report one pid, a runtime that places the session (tmux, herdr) owns `runtime` over the generic `proc`, in either order.

### tmux

The runtime lists panes with `tmux [-S socket] list-panes -a -F`, walks from each pane's pid down to the agent process through `/proc/<pid>/stat`, and reports the pane id as `address`. A server that cannot be reached is skipped, not an error.

With `"scrape": true` it also reads the last `scrape_lines` lines of each agent's pane (`capture-pane -p -J`) and matches them, per agent type, against a rules file. Rules are tried in order and the first match wins. The wording of an agent's prompts belongs to its releases, so the rules are data, not code: `examples/tmux-rules.claude-code.json` is a starting point, and you copy it to `~/.config/fleet/tmux-rules.json`. Anchor patterns to the bottom of the screen (`$`) so scrollback cannot match. A scrape is a pane-rank signal (20): it beats the transcript, loses to hooks, and expires two polls after it was read. Scraping is off by default because it reads every agent's screen on every poll.

In a container the runtime needs the `tmux` binary in the image, the host's server socket directory mounted (the directory rather than the socket file, so a restarted server is picked up), the same uid as the server, and the host's `/proc` (`pid: host`).

### herdr

The runtime reads herdr's local socket (newline-delimited JSON: `agent.list`, `pane.process_info`, `agent.explain` and an `events.subscribe` stream) and never sends a write method; the client refuses any method off a short allowlist. A pane binds to the pid of the foreground process an agent claims, so list `herdr` after `proc`.

| herdr state | Board state |
|---|---|
| `working` | `working` |
| `idle`, `done` | `waiting` |
| `blocked` | `approval` or `question`, decided from `agent.explain`; `approval` when unclear |
| `unknown` | no claim |

A screen-detected state ranks 25, below hooks. When herdr reports `screen_detection_skipped` the agent's own hooks feed it and the state ranks as native (50). The event stream wakes the poll loop early. A missing socket means herdr is not running, and the watcher retries quietly with backoff. In a container, bind-mount the socket's directory and set `socket`.

### hookfile

Reads the state files described in [session-state.md](session-state.md) and answers with a native-rank (50) signal that expires after `fresh`. It joins a file to a session by session id first and by pid second, and ignores a file written more than a second before the process started.

## Built-in agent

`claudecode` reads Claude Code's config directory: transcripts under `projects/`, `.claude.json` for the account (only the email, which is hashed before it leaves the process, plus name and organisation), and `sessions/<pid>.json` for exact binding. Everything else on the session view comes from the transcript, tailed incrementally: working directory, branch, version, model, tools, subagents, messages between sessions, cost and recap input.

## Signal ranks

A runtime reports state as `StateSignal` values on its `Observation`. The merge in `core` takes the highest unexpired rank, and the newer `ObservedAt` on a tie. Pick the rank that matches how much the source can be trusted.

| Rank | Constant | Use for |
|---|---|---|
| 50 | `host.RankNative` | a documented state file written by the agent's own plugin |
| 40 | `host.RankHook` | a hook relayed through another component |
| 30 | `host.RankProjection` | a launcher's view of the session |
| 20 | `host.RankPane` | a scrape of the terminal |
| 10 | `host.RankTranscript` | the transcript floor; agents produce this, runtimes should not |
| 0 | `host.RankProcess` | "starting", before any transcript exists |

Set `FreshUntil` on any signal that can go stale, such as a file a crashed session leaves behind. Valid `State` values are `working`, `waiting`, `approval`, `question`, `idle`, `ended`, `unknown` and `starting`.

## Writing a runtime

1. **Package.** Create `internal/host/<name>`. Implement `host.Runtime` (`Name` and `Sessions`). If the runtime only annotates sessions found elsewhere, return nil from `Sessions` as `hookfile` does. Add `host.Observer` to return observations, `host.Watcher` to wake the poll loop early, and `host.ConfigDirSource` to add agent config directories. The core finds the optional interfaces by type assertion.
2. **Config decoder.** Declare a struct with a `json` tag for every key, and a decoder with the signature `func(registry.Resolver, json.RawMessage, int) (registry.RuntimeSettings, error)`. Decode with `registry.Strict` so unknown keys are errors. Resolve paths through the `Resolver`: `Pick(key, envName, fileVal, default)` applies environment over file over default, expands `~` and records the source for the startup log. Never put a secret in the struct.
3. **Settings.** Return a value with `Build(registry.BuildCtx) host.Runtime` and `Value(key) (string, bool)`, the latter feeding the startup report.
4. **Register.** A built-in adapter registers from the `init` in `internal/config/adapters.go`: `registry.RegisterRuntime("<name>", ...)`. An optional adapter that some builds omit registers from its own file in `cmd/fleetwatch`, which is the only place that imports its package. Deleting that file and the package leaves a tree that still builds, and a config naming the type then fails with an unknown-type error that lists the known ones (ADR-011).
5. **Example.** Add a file under `examples/config/`. A test loads every example through the strict loader.
6. **Tests.** Test against recorded output, not a live system: keep fixtures under the package's `testdata/`, as `tmux` does with captured `list-panes` output and `herdr` with recorded socket responses. Cover the strict decoder (valid input, an unknown key, defaults).
7. **Rules.** Run `go vet ./... && go test ./...`. The `internal/policy` tests apply to your package:
   - no read of `/proc/<pid>/{environ,cwd,fd,exe}` under `internal/host/` (ADR-007);
   - no model name in shipped Go code (ADR-005);
   - no tracker citations in comments or docs; cite `ADR-00N` or state the reason.
8. **Free text.** Anything that can carry a prompt, a path or a command goes through `textsafe.Redact` and `textsafe.Clip` before it reaches a board field.
9. **Files.** Read files with `internal/fsafe` helpers, which refuse symlinks and FIFOs and cap the size.

### Worked example: hookfile

`internal/host/hookfile/hookfile.go` is about 130 lines and shows the whole pattern.

- `Config{Dir, Fresh}` holds the resolved settings; `internal/config/adapters_hookfile.go` decodes `{type, dir, fresh}` strictly and picks the directory from `FLEET_HOOKFILE_DIR`, then the file, then `$XDG_STATE_HOME/fleet/sessions`.
- `Sessions` returns nil. `Observe` reads the directory through `fsafe.ReadSmall` (at most 512 files of 16 KiB), validates each file (schema 1, a known state, the file name equals the session id, a parseable timestamp), then joins files to the bound sessions.
- Each match becomes one `StateSignal` with `Rank: host.RankNative` and `FreshUntil: observed_at + fresh`.
- `internal/config/adapters.go` registers it with `registry.RegisterRuntime("hookfile", asRuntime(decodeHookfile))`, and `examples/config/hookfile.json` exercises it.

## Writing an agent

Implement `agent.Agent` in `internal/agent/<name>`: `Claims` (is this process mine, from its `comm` and, lazily, its command line), `Bind` (exact proof first, a guess only when the core asks), `Tail`, `Session`, `Recent`, `Account` and `EndPass`. Each session implements `agent.Session`: a detail view with every free-text field passed through the supplied redaction function, a transcript-floor state at `RankTranscript`, a next-wake estimate, a recap digest, a log, and the paths it touched. Register the decoder with `registry.RegisterAgent`. `internal/agent/claudecode` is the reference; its package comment lists what it reads.

Any wire field you need to add belongs in `internal/board`, and a change there updates the golden snapshots in the same commit.
