#!/bin/sh
# fleet-hook.sh: writes the FleetWatch session state file for one Claude Code
# hook event (see ADR-010). Schema: schema/session-state.v1.json.
#
# Reads the hook payload on stdin and writes
#   ${XDG_STATE_HOME:-$HOME/.local/state}/fleet/sessions/<session_id>.json
# atomically (temp file, then mv) at 0600 in a 0700 directory. Needs jq.
# It never blocks or fails the hook: every path exits 0, and it prints nothing.
#
# FLEET_NOW overrides the timestamp and FLEET_PID the process id, for tests.
# FLEET_HOOK_LIB=1 defines the helper functions and returns before any work
# (a test sources the script that way, without needing jq).

# fleet_pid prints its argument when it is all digits, else 0: the value goes
# into jq --argjson, and a stray non-integer would fail the write every time.
fleet_pid() {
	case $1 in '' | *[!0-9]*) echo 0 ;; *) echo "$1" ;; esac
}

# fleet_now prints a UTC timestamp. With GNU date it carries nanoseconds, so two
# events inside one second still order; elsewhere (busybox and BSD date do not
# expand %N) it falls back to whole seconds.
fleet_now() {
	t=$(date -u +%Y-%m-%dT%H:%M:%S.%NZ 2>/dev/null)
	case $t in
	*[0-9].[0-9]*Z) ;; # nanoseconds present
	*) t=$(date -u +%Y-%m-%dT%H:%M:%SZ) ;;
	esac
	echo "$t"
}

if [ -n "${FLEET_HOOK_LIB:-}" ]; then
	return 0 2>/dev/null || exit 0
fi

umask 077
command -v jq >/dev/null 2>&1 || exit 0
payload=$(cat) || exit 0

field() { printf '%s' "$payload" | jq -r "$1 // empty" 2>/dev/null; }

sid=$(field .session_id)
event=$(field .hook_event_name)
case $sid in '' | *[!A-Za-z0-9_-]*) exit 0 ;; esac

base=${XDG_STATE_HOME:-$HOME/.local/state}
dir=$base/fleet/sessions
file=$dir/$sid.json

if [ "$event" = SessionEnd ]; then
	rm -f "$file"
	exit 0
fi

# Event to state. An event that says nothing about the state leaves the file as
# it is (SubagentStop, and notifications that are neither a prompt nor idle).
case $event in
PermissionRequest) state=approval reason="permission: $(field .tool_name)" ;;
Notification)
	case $(field .notification_type) in
	permission_prompt) state=approval reason="permission prompt" ;;
	elicitation_dialog) state=question reason="asking a question" ;;
	idle_prompt) state=waiting reason="idle at the prompt" ;;
	*) exit 0 ;;
	esac
	;;
Stop) state=waiting reason="turn finished" ;;
SessionStart) state=waiting reason="session started" ;;
UserPromptSubmit) state=working reason="prompt submitted" ;;
PreToolUse | PostToolUse) state=working reason="tool: $(field .tool_name)" ;;
*) exit 0 ;;
esac

# Hooks run through a short-lived shell: climb past shells to the agent itself.
pid=${FLEET_PID:-$PPID}
if [ -z "$FLEET_PID" ]; then
	n=0
	while [ $n -lt 4 ]; do
		case $(ps -o comm= -p "$pid" 2>/dev/null) in
		sh | bash | dash | zsh | ash) ;;
		*) break ;;
		esac
		up=$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ')
		[ -n "$up" ] && [ "$up" -gt 1 ] 2>/dev/null || break
		pid=$up
		n=$((n + 1))
	done
fi

pid=$(fleet_pid "$pid")
now=${FLEET_NOW:-$(fleet_now)}
mkdir -p "$dir" 2>/dev/null && chmod 700 "$dir" 2>/dev/null || exit 0
tmp=$dir/.$sid.$$.tmp
jq -n --arg sid "$sid" --argjson pid "$pid" --arg state "$state" --arg reason "$reason" \
	--arg ev "$event" --arg pane "${TMUX_PANE:-}" --arg cwd "$(field .cwd)" --arg at "$now" \
	'{schema: 1, session_id: $sid, pid: $pid, state: $state, reason: $reason, hook_event: $ev, pane: $pane}
	 + (if $cwd != "" then {cwd: $cwd} else {} end) + {observed_at: $at}' >"$tmp" 2>/dev/null &&
	chmod 600 "$tmp" && mv -f "$tmp" "$file" 2>/dev/null
rm -f "$tmp" 2>/dev/null
exit 0
