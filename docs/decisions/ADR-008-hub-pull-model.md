# ADR-008: Hub pull model

Status: accepted

## Context

One person often runs agents on several machines, and wants one board. The options are pushing from each machine to a collector, or having a collector pull from each machine.

## Decision

A hub is an ordinary FleetWatch instance with a `hub` section. It follows each upstream's `/v1/events` stream with a bearer token read from a file, merges the boards, and serves the result in the same shape. An upstream that drops goes `stale` for the recent window, with its sessions marked, and then `down`. Redirects are never followed, plain http is accepted only to loopback unless the entry opts in, and one event is capped at 8 MiB by default.

## Consequences

- Upstreams need no knowledge of the hub and no outbound access; they only need to be reachable and to issue a token.
- The hub is itself a source, so hubs chain.
- The hub holds a token per upstream and publishes a board event every poll, because last-seen times move on every tick.
- A host that restarts blinks rather than vanishes.
