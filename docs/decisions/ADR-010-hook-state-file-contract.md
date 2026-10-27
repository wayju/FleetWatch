# ADR-010: Hook state-file contract

Status: accepted

## Context

Hooks in an agent can state its condition exactly (waiting on an approval, asking a question, stopped), where transcripts and screens can only suggest it. Every agent has its own hook mechanism, and the board should not need a plugin per agent in Go.

## Decision

Hooks publish state as one small JSON file per session in a state directory, under a versioned JSON Schema (`schema/session-state.v1.json`). Files are written atomically at mode 0600, deleted at session end, and trusted only while fresh. The `hookfile` runtime reads them and answers with the highest rank, native (50). The Claude Code plugin is one writer; any agent can be another by writing the same file.

## Consequences

- The contract is a file format, so supporting a new agent needs no change to FleetWatch.
- Readers refuse malformed, oversized, misnamed and stale files, and a writer cannot crash the service.
- The format is versioned; a reader accepts only schema 1.
- A process that can write to the directory can change the displayed state, which the threat model records.
