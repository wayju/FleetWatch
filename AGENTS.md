# AGENTS.md

Notes for AI agents extending FleetWatch. Humans may find them useful too. For what the project is, read [README.md](README.md) and [docs/architecture.md](docs/architecture.md) first.

## Commands

| Task | Command |
|---|---|
| Vet | `go vet ./...` |
| Test everything | `go test ./...` |
| Format check | `gofmt -l .` (must print nothing) |
| Run | `go run ./cmd/fleetwatch` |
| Build | `CGO_ENABLED=0 go build -trimpath -o fleetwatch ./cmd/fleetwatch` |

Vet and test must both pass before a change is done. Go 1.24 or later. The binary depends on the standard library only; do not add a dependency (ADR-006).

## Layout

| Path | Contents |
|---|---|
| `cmd/fleetwatch/` | entry point, embedded `VERSION`, and one file per optional adapter registration |
| `internal/core/` | collector, poll loop, HTTP routes, token, recap, hub source |
| `internal/config/` | strict config loader and the built-in adapter decoders |
| `internal/registry/` | adapter registration by name |
| `internal/host/` | the runtime interface and the runtimes `proc`, `tmux`, `herdr`, `hookfile` |
| `internal/agent/` | the agent interface and `claudecode` |
| `internal/board/` | wire types served to clients |
| `internal/upstream/` | hub follower and merge |
| `internal/forge/`, `internal/models/` | issue lookups, and the config-driven model table |
| `internal/fsafe/`, `internal/httpsafe/`, `internal/textsafe/` | safe reads, HTTP client policy, redaction |
| `internal/policy/` | repository-wide guard tests |
| `internal/fleettest/` | test support that fabricates transcripts |
| `plugin/` | the Claude Code hook plugin |
| `schema/` | the session state file schema |
| `docs/` | documentation; `docs/decisions/` holds the ADRs |
| `examples/` | example configs, loaded by a test |

## Invariants

Each of these is enforced by a test or is a decision in `docs/decisions/`. Do not weaken one to make a change fit.

1. **Adapters are read-only** (ADR-001). Never send keys, text or prompts to a session, and never write to a transcript or an agent's files.
2. **No `/proc/<pid>/{environ,cwd,fd,exe}`** under `internal/host/` (ADR-007). Those reads flood the audit queue under docker-default AppArmor. `internal/policy/forbidden_test.go` enforces it.
3. **Nothing is compiled in** (ADR-005). No model name, price, context window, host, owner or URL in shipped code. They are config. `internal/policy/literals_test.go` enforces it, comments included.
4. **Config is strict JSON** (ADR-004). Decode with `registry.Strict`, so an unknown key is an error. No struct in config has a field for a secret; tokens are read from the environment or a file named by reference.
5. **Free text goes through `textsafe.Redact` and `textsafe.Clip`** before it reaches a board field, and every free-text field must also respect `show_text: false`.
6. **Read state, transcript and token files with `internal/fsafe`.** (Operator-named config files are the exception.) Never follow a symlink or block on a FIFO.
7. **HTTP clients that carry a token refuse redirects** (`httpsafe.RefuseRedirect`) and refuse cleartext to a non-loopback host unless the entry opts in.
8. **The wire contract is frozen by goldens** (`internal/core/testdata/golden`). A change to a board field updates the goldens in the same commit and says why. Routes are served under `/v1`.
9. **No tracker numbers in comments or docs.** Cite `ADR-00N` or state the reason (`internal/policy/citations_test.go`).
10. **Standard library only** (ADR-006).

## Style

- Every file opens with a comment that says what it is for. Files stay under about 300 lines and functions under about 50; split before they grow.
- Comments explain intent and the reason for a decision, not what the next line does.
- NZ English in prose. No marketing words.
- Tests sit beside the code (`*_test.go`) and use recorded fixtures under `testdata/`, never a live system.
- Commit messages are short and imperative with a prefix: `fix:`, `feat:`, `docs:`, `refactor:`, `test:`.

## Adding an adapter

The full guide is [docs/adapters.md](docs/adapters.md). In short:

1. Create `internal/host/<name>` (a runtime) or `internal/agent/<name>` (an agent) and implement the interface, plus any optional interfaces.
2. Write a strict decoder with a `json` tag for every key, resolving paths through `registry.Resolver`.
3. Register it: built-in adapters from the `init` in `internal/config/adapters.go`; optional ones from their own file in `cmd/fleetwatch` so deleting the file removes the adapter.
4. Add `examples/config/<name>.json`.
5. Add tests with recorded fixtures, including the decoder's valid, unknown-key and default cases.
6. Run vet and test; the policy tests apply to the new package.

## Decisions and documentation

Design decisions go in a short ADR under `docs/decisions/` (context, decision, consequences), numbered in sequence and added to the index in `docs/decisions/README.md`. When behaviour changes, update `docs/` and `CHANGELOG.md` in the same change. When you add a config key, add it to the strict decoder, `docs/configuration.md` and an example.

## Do not

- Add a dependency, a model name, or an estate-specific host, account or path.
- Commit a credential, even a fake one, without allowing it by exact value in `.gitleaks.toml`.
- Make a test pass by loosening it, skipping it or raising a retry count.
- Rewrite published history or tags.
