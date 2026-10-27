# ADR-009: Recap outside the session

Status: accepted

## Context

A short summary of what a session is doing is useful on a board. The obvious way to get one is to ask the agent, but typing into a working session queues the text as its next prompt (ADR-001).

## Decision

The summary is produced by FleetWatch. It sends a redacted digest of a session (requests, recent activity, subagents, the latest reply) to an OpenAI-compatible chat endpoint the operator names, and keeps the answer in memory. One call runs at a time, only for sessions that moved, at most every `recap.every`. It is off until the URL, model and key are all configured.

## Consequences

- The observed session is never touched.
- The digest leaves the machine, so the operator must trust the endpoint; the documentation says so, and `show_text: false` turns the feature off with the rest of the free text.
- Summaries can lag a session by up to the interval.
