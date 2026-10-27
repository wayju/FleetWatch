# ADR-007: File-read hardening

Status: accepted

## Context

The poll loop reads `/proc` and the user's files every few seconds on a shared host. Under the default container AppArmor profile, reads of `/proc/<pid>/environ`, the `cwd` link, `fd` and `exe` are mediated by ptrace checks and flood the audit queue. Files that look like state can be symlinks that lead elsewhere or FIFOs that block the loop.

## Decision

Runtime adapters read only `/proc/<pid>/{stat,cmdline,cgroup}`; a test over `internal/host` refuses the other four. All state, transcript and generated-token files are opened through `internal/fsafe`, which requires a regular file, refuses symlinks with `O_NOFOLLOW`, does not block on a FIFO and caps size. Transcript reads are bounded per line and per poll. HTTP clients that carry a bearer token refuse redirects.

## Consequences

- The service is quiet under confinement and cannot be pointed at an arbitrary file by a planted link.
- Environment and working directory of a process are unavailable, so a process is identified by its command line and by what the agent itself records.
- New code that reads files has one helper to use and a test that catches a shortcut.
