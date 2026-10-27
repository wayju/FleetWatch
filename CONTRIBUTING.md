# Contributing

Thank you for looking at FleetWatch. Contributions are welcome, including ones written with AI assistance. FleetWatch itself was written mostly by AI coding agents, and a contribution is judged on the same gates whoever or whatever wrote it.

## Set up

You need Go 1.24 or later, and Linux or a mounted `/proc`. There are no third-party dependencies and none may be added: the binary is standard library only (ADR-006).

```sh
git clone https://github.com/wayju/FleetWatch.git
cd FleetWatch
go vet ./...
go test ./...
go run ./cmd/fleetwatch
```

`go vet ./... && go test ./...` is the whole gate. There is no Makefile.

## What the tests enforce

Besides the unit tests, `internal/policy` holds repository-wide guards. They fail the build, so read them before you are surprised by one.

| Guard | Forbids |
|---|---|
| `forbidden_test.go` | reading `/proc/<pid>/{environ,cwd,fd,exe}` anywhere under `internal/host` (ADR-007) |
| `literals_test.go` | a model name in shipped Go code, comments included (ADR-005) |
| `citations_test.go` | tracker numbers in comments and docs; cite `ADR-00N` or state the reason |

Config examples under `examples/config/` are loaded through the strict loader by a test. The board's wire types are frozen by golden snapshots in `internal/core/testdata/golden`: if you change what the board says, update the goldens in the same commit and explain why in the pull request.

## Making a change

1. Open an issue first for anything beyond a small fix, so the approach can be agreed before you write it.
2. Branch from the default branch, with a name that describes the change, such as `fix/short-description`.
3. Keep to the conventions in [AGENTS.md](AGENTS.md): small files and functions, an intent comment at the top of each file, strict config, read-only adapters, free text through `textsafe`.
4. Add tests with the change. A bug fix gets a regression test. Tests sit beside the code they test, which is the Go convention, and use fixtures recorded from the real thing.
5. Run `gofmt -l .`, `go vet ./...` and `go test ./...`.
6. Open a pull request against the default branch. Describe what changed and why, and how you checked it. A decision that others will need to understand later goes in a short ADR under `docs/decisions/`.
7. Commit messages are short and in the imperative, with a conventional prefix (`fix:`, `feat:`, `docs:`, `refactor:`, `test:`).

Pull requests are squash-merged once checks are green and a maintainer has reviewed them.

## Sign-off

There is no contributor agreement and no DCO requirement. By submitting a contribution you agree it is licensed under the Apache License 2.0, as the rest of the project is.

## AI-assisted contributions

They are welcome and held to the same bar. Say in the pull request that an agent wrote the change, read the diff yourself before submitting it, and be ready to explain it. Do not paste agent output you have not run. A change that makes the tests pass by loosening them, or that adds a dependency, will be declined.

## Reporting a vulnerability

Do not open a public issue. See [SECURITY.md](SECURITY.md).

## Conduct

This project follows the [Code of Conduct](CODE_OF_CONDUCT.md).
