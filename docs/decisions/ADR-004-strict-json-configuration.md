# ADR-004: Strict JSON configuration

Status: accepted

## Context

A misspelt configuration key that silently does nothing is a common and expensive failure: the file looks right and the behaviour is wrong. Secrets in a config file end up in backups, screenshots and version control.

## Decision

Configuration is JSON, decoded strictly. Unknown keys, trailing data and a wrong `version` are errors, in the top level, in every section and in every adapter entry. Precedence is built-in default, then file, then environment variable, and the startup log shows which layer each setting came from. A broken named file stops the service rather than falling back to defaults. No config struct has a field for a secret: the token, the recap key and forge tokens come from the environment, or from a file named by reference (`token_file`), read once and never logged.

## Consequences

- Typos fail at startup with the key named.
- There are no comments in the file; the documentation carries the annotations.
- Adding a setting means adding it to a strict decoder, the documentation and an example, and a test loads every example.
- Secrets can live in whatever store the operator already protects.
