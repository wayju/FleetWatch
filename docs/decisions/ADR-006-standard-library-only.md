# ADR-006: Standard library only

Status: accepted

## Context

Every dependency is code that runs with access to the user's transcripts, and a supply-chain path into a tool that reads sensitive material. Dependencies also cost updates and audits.

## Decision

The binary depends on the Go standard library and nothing else. The module has no `require` lines, and a change that adds one is declined. The HTTP server, JSON, regular expressions, server-sent events and the clients are all standard library.

## Consequences

- The attack surface from third-party code is nil, and builds are reproducible with only a Go toolchain.
- Some conveniences are written by hand, such as the strict-decode helper and the redaction patterns.
- The NOTICE file can truthfully say the product contains no third-party code.
