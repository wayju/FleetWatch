# ADR-003: Ranked state merge

Status: accepted

## Context

A session's state can be inferred from its transcript, read from a terminal screen, reported by a multiplexer, or stated exactly by an agent's hook. These sources disagree, go stale and arrive at different times. Averaging them or letting the latest write win produces flapping.

## Decision

Each source reports a `StateSignal` with a rank: native state file 50, hook 40, projection 30, multiplexer screen 25, pane scrape 20, transcript 10, process 0. The merge drops signals past their expiry, takes the highest rank, and breaks a tie with the newer observation time. The transcript reading is the floor every session has. After the merge, `waiting` for over 30 minutes becomes `idle`, and a session with a scheduled wake or pending background work is `working`, so `waiting` always means the human's move.

## Consequences

- The strongest evidence wins and the rule is one function that is easy to test.
- Any source can be turned off without changing the others.
- An expiring signal retires itself, so a crashed session's last write stops winning.
- Ranks are a shared scale: a new adapter must choose a rank that fits it, and the herdr rank is configurable only inside the band between the transcript and the projection.
