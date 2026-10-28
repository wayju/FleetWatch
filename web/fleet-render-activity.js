// Live-activity render helpers for the fleet page: the session
// activity line, the agent lifeline and the hand-back highlights.
//
// PURE, same rules as fleet-render.js: each function is injected into the page
// by toString(), so nothing below may reference a module-level binding other
// than functions injected alongside it (see template-fleet.js). Every board
// string goes through fleetEsc, every number through Number().
//
// All inputs are optional: an older board has none
// of current / description / events / handed_back_at, and renders as before.

const { fleetEsc, fleetAgo } = require('./fleet-render-base');

// Unix seconds -> local HH:MM:SS.
function fleetClock(at) {
  var d = new Date((Number(at) || 0) * 1000);
  function p(n) { return ('0' + n).slice(-2); }
  return p(d.getHours()) + ':' + p(d.getMinutes()) + ':' + p(d.getSeconds());
}

// The running tool of a session: its description (else the tool name) with a
// ticking elapsed time, and the command dimmed on a second line.
function fleetActivity(cur, now) {
  if (!cur || typeof cur !== 'object') return '';
  var text = cur.description || cur.name;
  if (!text) return '';
  return '<div class="fl-act"><div class="fl-act-main"><span class="fl-spin">◐</span><span class="fl-act-text">' + fleetEsc(text) + '</span>'
    + (cur.at ? '<span class="fl-act-sep">·</span>' + fleetAgo(cur.at, now) : '') + '</div>'
    + (cur.target ? '<div class="fl-act-cmd fl-dim">' + fleetEsc(cur.target) + '</div>' : '')
    + '</div>';
}

// An agent's current tool. With a description the description leads and the
// command sits dimmed beneath; without one the old name + target line stays.
function fleetAgentCurrent(cur, glyph) {
  if (!cur) return '';
  if (cur.description) {
    return '<div class="fl-ag-cur fl-ag-cur-d">' + glyph + ' ' + fleetEsc(cur.description) + '</div>'
      + (cur.target ? '<div class="fl-ag-cmd fl-dim">' + fleetEsc(cur.target) + '</div>' : '');
  }
  return '<div class="fl-ag-cur">' + glyph + ' ' + fleetEsc(cur.name) + (cur.target ? ' <span class="fl-dim">' + fleetEsc(cur.target) + '</span>' : '') + '</div>';
}

// A compact row of event marks; the time is in each tooltip.
function fleetLifeline(events) {
  if (!Array.isArray(events) || !events.length) return '';
  var g = { launch: '▶', to: '→', from: '←', handback: '↩', done: '✓', error: '✗' };
  return '<div class="fl-life">' + events.map(function (e) {
    if (!e) return '';
    var k = String(e.kind || '');
    return '<span class="fl-lf fl-lf-' + fleetEsc(g[k] ? k : 'other') + '" title="' + fleetEsc(k + ' ' + fleetClock(e.at)) + '">' + (g[k] || '·') + '</span>';
  }).join('') + '</div>';
}

// The newest control hand-over inside the 120s window: {kind, at} or null.
// handed_back_at counts as a hand-back; ties go to the hand-back.
function fleetRecent(a, now) {
  var best = null;
  function take(kind, at) {
    at = Number(at) || 0;
    if (!at || now - at > 120 || now - at < -5) return;
    if (!best || at > best.at || (at === best.at && kind === 'handback')) best = { kind: kind, at: at };
  }
  take('handback', a && a.handed_back_at);
  ((a && a.events) || []).forEach(function (e) {
    if (e && e.kind === 'from') take('from', e.at);
    if (e && e.kind === 'handback') take('handback', e.at);
  });
  return best;
}

function fleetRecentBadge(recent, now) {
  if (!recent) return '';
  return '<span class="fl-ag-badge">' + (recent.kind === 'handback' ? '↩ handed back ' : '← message ') + fleetAgo(recent.at, now, ' ago') + '</span>';
}

// Hand-backs in the last 5 minutes across every agent, history ones included.
function fleetHandbackChip(agents, now) {
  var n = (agents || []).filter(function (a) {
    var t = Number(a && a.handed_back_at) || 0;
    return t && now - t <= 300 && now - t >= -5;
  }).length;
  if (!n) return '';
  return '<span class="fl-hb-chip" title="subagents that returned control in the last 5 minutes">↩ ' + fleetEsc(n) + (n === 1 ? ' hand-back' : ' hand-backs') + ' (5m)</span>';
}

module.exports = {
  fleetClock, fleetActivity, fleetAgentCurrent, fleetLifeline, fleetRecent, fleetRecentBadge, fleetHandbackChip,
};
