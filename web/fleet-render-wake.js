// Wake countdown helpers for the fleet page. A waiting or idle session
// may carry next_wake = {kind: "timer"|"condition", at (unix, timer only), reason}
// from FleetWatch; absent or null renders nothing.
//
// PURE and dependency-free: injected into the page by toString() in
// template-fleet.js, so nothing here may reference a module-level binding other
// than the functions injected alongside it. Everything from the board goes
// through fleetEsc.

const { fleetEsc } = require('./fleet-render-base');

// The wake the session will act on, or null. FleetWatch reports a session
// with a pending wake as working, not waiting,
// so working shows one too; delegating, approval and question never do.
function fleetWakeOf(s) {
  var w = s && (s.state === 'working' || s.state === 'waiting' || s.state === 'idle') ? s.next_wake : null;
  if (!w) return null;
  if (w.kind === 'timer') return isFinite(Number(w.at)) && w.at != null ? w : null;
  return w.kind === 'condition' ? w : null;
}

// Minutes left, rounded up and never below 1; 0 once the time has arrived.
function fleetWakeMins(at, now) {
  var left = Number(at) - Number(now);
  if (!isFinite(left) || left <= 0) return 0;
  return Math.max(1, Math.ceil(left / 60));
}

// "43m" or "due" for a timer. The badge renders it and the tick refreshes it.
function fleetWakeText(at, now) {
  var m = fleetWakeMins(at, now);
  return m ? m + 'm' : 'due';
}

// "(43)" or "(due)" for a slot chip.
function fleetWakeChipText(at, now) {
  var m = fleetWakeMins(at, now);
  return '(' + (m ? m : 'due') + ')';
}

// Badge beside the state pill. The timer text sits in a span carrying
// data-wake-at so the page's one-second tick can recount it between boards.
function fleetWakeBadge(s, now) {
  var w = fleetWakeOf(s);
  if (!w) return '';
  var tip = ' title="' + fleetEsc(w.reason || (w.kind === 'timer' ? 'scheduled wake-up' : 'waiting on a background task')) + '"';
  if (w.kind === 'timer') {
    return '<span class="fl-wake fl-wake-timer"' + tip + '>⏰ <span data-wake-at="' + fleetEsc(Number(w.at)) + '">' + fleetEsc(fleetWakeText(w.at, now)) + '</span></span>';
  }
  return '<span class="fl-wake fl-wake-cond"' + tip + '>⏳ on condition</span>';
}

// Slot chip suffix: " (43)" for a timer, " (C)" for a condition, "" otherwise.
// A timer suffix is a span with data-wake-chip so the tick can recount it.
function fleetWakeSuffix(s, now) {
  var w = fleetWakeOf(s);
  if (!w) return '';
  if (w.kind === 'condition') return ' (C)';
  return ' <span data-wake-at="' + fleetEsc(Number(w.at)) + '" data-wake-chip="1">' + fleetEsc(fleetWakeChipText(w.at, now)) + '</span>';
}

module.exports = { fleetWakeOf, fleetWakeMins, fleetWakeText, fleetWakeChipText, fleetWakeBadge, fleetWakeSuffix };
