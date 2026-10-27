package herdr

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// errResubscribe ends a connection because the set of panes changed, so the
// per-pane status subscriptions must be rebuilt. It reconnects without backoff.
var errResubscribe = errors.New("herdr: pane set changed")

// Watch implements runtime.Watcher. It holds an events.subscribe stream open
// and calls poke whenever an agent's status changes or a pane appears or
// goes, reconnecting with exponential backoff. herdr not running is the quiet
// normal case and logs nothing.
func (r *Runtime) Watch(ctx context.Context, poke func()) error {
	back := r.cfg.BackoffMin
	var lastErr string
	for ctx.Err() == nil {
		err := r.watchOnce(ctx, poke, func() { back = r.cfg.BackoffMin })
		wait := back
		switch {
		case errors.Is(err, errResubscribe):
			wait = 50 * time.Millisecond
		default:
			if msg := errText(err); msg != "" && msg != lastErr {
				r.cfg.Logf("herdr: watch: %s", msg)
				lastErr = msg
			}
			if back *= 2; back > r.cfg.BackoffMax {
				back = r.cfg.BackoffMax
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}
	return ctx.Err()
}

// errText is the error worth logging: a socket that is absent or refusing is
// herdr not running.
func errText(err error) string {
	if err == nil || isNotRunning(err) {
		return ""
	}
	return err.Error()
}

func (r *Runtime) watchOnce(ctx context.Context, poke func(), subscribed func()) error {
	var list struct {
		Agents []agentInfo `json:"agents"`
	}
	if err := r.once("agent.list", struct{}{}, &list); err != nil {
		return err
	}
	c, err := dial(r.cfg.Socket, r.cfg.Timeout)
	if err != nil {
		return err
	}
	defer c.close()
	stop := context.AfterFunc(ctx, c.close)
	defer stop()
	subs := []map[string]string{{"type": "pane.created"}, {"type": "pane.closed"}, {"type": "pane.exited"}, {"type": "pane.agent_detected"}}
	for _, a := range list.Agents {
		if a.PaneID != "" {
			subs = append(subs, map[string]string{"type": "pane.agent_status_changed", "pane_id": a.PaneID})
		}
	}
	if err := c.call("events.subscribe", map[string]any{"subscriptions": subs}, nil); err != nil {
		return err
	}
	subscribed()
	poke() // anything that changed while disconnected
	for {
		b, err := c.line(time.Time{})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		var f frame
		if json.Unmarshal(b, &f) != nil {
			continue
		}
		if f.Error != nil {
			return &rpcError{f.Error.Code, f.Error.Message} // events_lost reconnects and re-polls
		}
		switch normalise(f.Event) {
		case "pane_agent_status_changed":
			poke()
		case "pane_created", "pane_closed", "pane_exited", "pane_agent_detected":
			poke()
			return errResubscribe
		}
	}
}

// normalise reads an event name in either notation: the subscription kinds
// use dots (pane.created) and the lifecycle payloads use underscores.
func normalise(name string) string { return strings.ReplaceAll(name, ".", "_") }
