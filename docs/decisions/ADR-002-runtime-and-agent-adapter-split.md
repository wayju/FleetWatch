# ADR-002: Runtime and agent adapter split

Status: accepted

## Context

Two questions get tangled when a tool watches coding agents. Where does the process live (a bare process, a tmux pane, a terminal multiplexer, a launcher's pool), and what is running in it (which agent, with which transcripts)? Combining them in one adapter means every new agent needs every environment re-implemented, and the reverse.

## Decision

There are two adapter kinds. A **runtime** (`internal/host`) finds agent processes and reports what its place knows about them; it never reads transcripts. An **agent** (`internal/agent`) claims processes, binds them to sessions and folds its own transcripts into views. The core consumes only these interfaces. Optional capabilities (observing, waking the poll loop, adding config directories) are separate small interfaces found by type assertion. Adapters register by name in `internal/registry`. The board's JSON contract is frozen by golden snapshots, so the split could be made without changing what clients see.

## Consequences

- A new agent reuses every runtime, and a new runtime serves every agent.
- The core stays free of any one agent's or terminal's details.
- A session is described by two adapters, so binding has rules: the first runtime to report a pid owns it, and an agent binds on proof before it guesses.
- The registry is the one place to look to learn which adapters exist.
