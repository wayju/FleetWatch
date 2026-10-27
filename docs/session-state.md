# Session state files

Hooks report an agent's state exactly: an approval prompt, a question, a finished turn. A transcript or a screen scrape can only infer these. The `hookfile` runtime reads one small JSON file per session, and any agent or wrapper that writes the file in the format below can feed the board. The format is a versioned contract (ADR-010) described by the JSON Schema `schema/session-state.v1.json`.

## Location and lifecycle

- One file per session at `$XDG_STATE_HOME/fleet/sessions/<session_id>.json`, defaulting to `~/.local/state/fleet/sessions/`. The reader's directory is the `dir` key of the `hookfile` runtime, or `FLEET_HOOKFILE_DIR`.
- Write atomically: create a temporary file in the same directory, then rename it over the target. Create the directory with mode 0700 and the file with 0600.
- Delete the file when the session ends.
- Keep writing on activity. The reader trusts a file only while it is fresh, one hour after `observed_at` by default (`fresh`). A crashed session's last write therefore stops winning on its own.

## Format

```json
{
  "schema": 1,
  "session_id": "3f2a9c10-aaaa-bbbb-cccc-0123456789ab",
  "pid": 41873,
  "state": "approval",
  "reason": "permission: Bash",
  "hook_event": "PermissionRequest",
  "pane": "%3",
  "cwd": "/home/me/src/app",
  "observed_at": "2026-10-27T09:14:02.481516Z"
}
```

| Field | Type | Rules |
|---|---|---|
| `schema` | integer | Must be `1`. Any other value is ignored. |
| `session_id` | string | Matches `^[A-Za-z0-9_-]+$` and equals the file name without `.json`. |
| `pid` | integer | The agent process id, 0 when unknown. Used to join a session the reader could not bind by id. |
| `state` | string | One of `working`, `waiting`, `approval`, `question`, `idle`. |
| `reason` | string | Short human-readable cause, shown on the board. |
| `hook_event` | string | The event that produced the write, for example `PermissionRequest`. |
| `pane` | string | Terminal pane id such as a tmux `%3`; empty when none. |
| `cwd` | string | Optional working directory. |
| `observed_at` | string | RFC 3339 in UTC. Sub-second precision is read and orders two writes to one pid. |

The schema allows no other keys. The reader refuses a file that is not a regular file, is over 16 KiB, fails to parse, has an unknown state, or whose name is not its session id. It reads at most 512 files per pass.

## How the reader uses a file

A file joins a session by session id first and by pid second. A file written more than a second before the process started belongs to a previous owner of the pid and is ignored. When two files share a pid, the newest `observed_at` wins. The result is a state signal of rank 50, the highest, with an expiry of `observed_at` plus `fresh`, so it beats a screen scrape or a transcript reading while it lasts. The pane becomes the session's `address`.

## State meanings

| State | Meaning |
|---|---|
| `working` | the agent is acting |
| `waiting` | the agent finished and waits for the user; becomes `idle` on the board after 30 minutes |
| `approval` | blocked on a permission prompt |
| `question` | blocked on a question to the user |
| `idle` | nothing is happening and nothing is expected |

## The Claude Code plugin

`plugin/claude-code/` is a Claude Code plugin that writes these files. It needs `jq`. Each hook event runs `scripts/fleet-hook.sh`, which writes the file and deletes it on `SessionEnd`. The script exits 0 on every path and prints nothing, so it can never block or fail a hook. It runs a shell and `jq` on every tool call, which is cheap but not free.

```sh
claude plugin marketplace add /path/to/fleetwatch/plugin
claude plugin install fleet-state@fleetwatch
```

Then enable the `hookfile` runtime (`examples/config/hookfile.json`).

| Event | State |
|---|---|
| `PermissionRequest`, `Notification` `permission_prompt` | `approval` |
| `Notification` `elicitation_dialog` | `question` |
| `Notification` `idle_prompt`, `Stop`, `SessionStart` | `waiting` |
| `UserPromptSubmit`, `PreToolUse`, `PostToolUse` | `working` |
| `SubagentStop`, other notifications | unchanged |

The script finds the agent's pid by climbing past the shells the hook runs through, validates that it is an integer (else it records 0), and takes `pane` from `$TMUX_PANE`. Both are best effort, since the board binds by session id first. To remove the plugin, uninstall it through Claude Code and delete the `sessions` directory.

## Writing the file from another agent

Anything that can run a command on its lifecycle events can write the file. A minimal writer needs the session id, the state and a UTC timestamp. Validate your output against `schema/session-state.v1.json` and check that the board reports the session's state source as `hook`.
