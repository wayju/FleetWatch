# ADR-011: Private adapters

Status: accepted

## Context

Some deployments need an adapter tied to a particular site or launcher that has no place in a general-purpose project. Keeping it in a fork means diverging from the core, and keeping it in the tree means shipping site details.

## Decision

An optional adapter lives in its own package and registers from a single file in `cmd/fleetwatch`, which is the only place that imports it. The core knows adapters only through the registry. A build without the file has no such adapter, and a config naming its type fails with an unknown-type error that lists the known ones. The private source repository carries such adapters; the public tree is exported from it with the paths named in `.public-exclude` removed, and an export check builds and tests the result and scans it for site-specific strings and secrets before it is published.

## Consequences

- The public tree builds and passes its tests without any private adapter.
- Public history holds one commit per release rather than the private history.
- Tests for a private adapter carry a build tag and live in files that are excluded together with it.
- The export check is the control that keeps site details out; it must keep passing.
