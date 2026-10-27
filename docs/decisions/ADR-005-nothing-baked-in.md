# ADR-005: Nothing baked in

Status: accepted

## Context

Model names, prices, context windows, forge hosts and owners change faster than releases, and favour one provider or one site if compiled in. A tool that hard-codes them is stale the day a model ships and is not reusable elsewhere.

## Decision

No model name, price, context window, host, owner, recap model or URL is compiled into the binary. They are configuration: a `models` table of globs giving context window and display class, a prices file, a `forges` list, a recap endpoint. With an empty config FleetWatch still works and says what it does not know: tokens shown as unpriced, no context percentage, no issue titles. A policy test fails the build if a model-family literal appears in shipped Go code, comments included.

## Consequences

- Releases do not go stale when a model is released, and no provider is favoured.
- Out of the box the board is less rich; the example configs show how to enrich it.
- The sample prices file is dated and unmaintained, and says so.
