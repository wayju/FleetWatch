# ADR-001: Read-only observer

Status: accepted

## Context

FleetWatch sits beside agent sessions that hold the user's working state. A tool that can type into a pane can corrupt a prompt in progress: keystrokes sent to a working session queue up as its next prompt. A tool that can write to transcripts or agent files can corrupt history.

## Decision

Adapters only observe. They never send keys, text or prompts to a session and never write to an agent's files. The herdr client refuses any method off a short read-only allowlist. The recap summary is produced by FleetWatch and kept in its own memory, never typed into the session it describes (ADR-009).

## Consequences

- A user can run FleetWatch next to live sessions without risk to them.
- Features that need to act on a session, such as answering an approval prompt from the board, are out of scope. Clients that want them must use the agent's own interfaces.
- Reviewers have one rule to check on every adapter change.
