# Architecture decision records

Each record states the context, the decision and its consequences. Code comments and documents cite a decision as `ADR-00N` rather than by an issue number. To add one, copy the shape of an existing record, take the next number, and add it to the table.

| ADR | Title | Summary | Status |
|---|---|---|---|
| [001](ADR-001-read-only-observer.md) | Read-only observer | Adapters never send keys, text or prompts to a session; the tool only observes. | accepted |
| [002](ADR-002-runtime-and-agent-adapter-split.md) | Runtime and agent adapter split | Where a session runs (runtime) and what is running in it (agent) are separate adapter kinds. | accepted |
| [003](ADR-003-ranked-state-merge.md) | Ranked state merge | Competing signals about one session are merged by rank, so the strongest evidence wins. | accepted |
| [004](ADR-004-strict-json-configuration.md) | Strict JSON config | Configuration is strict JSON with no secrets in the file; unknown keys are errors. | accepted |
| [005](ADR-005-nothing-baked-in.md) | No baked constants | Models, prices, context windows, hosts and forges are configuration, never compiled in. | accepted |
| [006](ADR-006-standard-library-only.md) | Standard library only | The binary depends on the Go standard library and nothing else. | accepted |
| [007](ADR-007-file-read-hardening.md) | File-read hardening | Reads use a /proc allowlist and safe file helpers, and never touch environ, cwd, fd or exe, so AppArmor stays quiet. | accepted |
| [008](ADR-008-hub-pull-model.md) | Hub pull model | A hub pulls and merges other fleets' boards, authenticating with tokens read from files. | accepted |
| [009](ADR-009-recap-outside-the-session.md) | Recap outside the session | Summaries are produced by the observer, never written into the observed session. | accepted |
| [010](ADR-010-hook-state-file-contract.md) | Hook state-file contract | Agent hooks publish session state as one JSON file per session under a versioned schema. | accepted |
| [011](ADR-011-private-adapters.md) | Private adapters | Site-specific adapters plug in through a registration file and a public export. | accepted |

## Template

```
# ADR-0NN: Title

Status: proposed | accepted | superseded by ADR-0MM

## Context
## Decision
## Consequences
```
