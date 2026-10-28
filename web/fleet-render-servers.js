// Agent-started dev servers on the fleet page. Each live session may
// carry servers = [{pid, port, cmd, cwd, dir, started_at, stale, stale_reason}]
// from FleetWatch, and counts.servers / counts.servers_stale for the board.
// The page stays read-only: the kill line is plain text to copy, never a button.
//
// PURE and dependency-free: injected into the page by toString() in
// template-fleet.js, so nothing here may reference a module-level binding other
// than the functions injected alongside it. Everything from the board goes
// through fleetEsc.

const { fleetEsc } = require('./fleet-render-base');

// 144000 s -> "1d 16h", 19200 -> "5h 20m", 90 -> "1m", 10 -> "<1m".
function fleetServersDur(sec) {
  sec = Number(sec);
  if (!isFinite(sec) || sec < 0) return '';
  var m = Math.floor(sec / 60), h = Math.floor(m / 60), d = Math.floor(h / 24);
  if (d) return d + 'd ' + (h % 24) + 'h';
  if (h) return h + 'h ' + (m % 60) + 'm';
  return m ? m + 'm' : '<1m';
}

// Servers with a numeric pid and port, oldest first.
function fleetServersOf(s) {
  var list = s && Array.isArray(s.servers) ? s.servers : [];
  return list.filter(function (v) {
    return v && Number.isInteger(Number(v.pid)) && Number(v.pid) > 0 && Number.isInteger(Number(v.port)) && Number(v.port) > 0;
  }).sort(function (a, b) { return (Number(a.started_at) || 0) - (Number(b.started_at) || 0); });
}

function fleetServersWhy(reason) {
  var why = { age: 'older than 12 h', merged: 'branch merged', dir_gone: 'worktree removed' };
  return why[reason] || 'stale';
}

// One block per server inside the card's details: command, directory and a
// selectable "kill <pid>" line.
function fleetServerRow(v, now) {
  var cls = 'fl-srv-row' + (v.stale ? ' fl-srv-stale' : '');
  var age = fleetServersDur(now - Number(v.started_at));
  return '<div class="' + cls + '"><div><b>:' + fleetEsc(Number(v.port)) + '</b> pid ' + fleetEsc(Number(v.pid))
    + (age ? ' · up ' + fleetEsc(age) : '') + (v.stale ? ' · <span class="fl-srv-why">' + fleetEsc(fleetServersWhy(v.stale_reason)) + '</span>' : '') + '</div>'
    + (v.cmd ? '<div class="fl-srv-cmd">' + fleetEsc(v.cmd) + '</div>' : '')
    + (v.dir ? '<div class="fl-srv-dir">' + fleetEsc(v.dir) + '</div>' : '')
    + '<code class="fl-srv-kill">kill ' + fleetEsc(Number(v.pid)) + '</code></div>';
}

// "3 servers · :8791 :5173 · oldest 1d 16h" as the summary of a details block
// holding the per-server rows. Stale ports and the stale count are marked.
function fleetServersLine(s, now) {
  var list = fleetServersOf(s);
  if (!list.length) return '';
  var stale = list.filter(function (v) { return v.stale; }).length;
  var ports = list.map(function (v) {
    return '<span class="fl-srv-port' + (v.stale ? ' fl-srv-stale' : '') + '">:' + fleetEsc(Number(v.port)) + '</span>';
  }).join(' ');
  var oldest = fleetServersDur(now - (Number(list[0].started_at) || now));
  return '<details class="fl-servers" data-key="' + fleetEsc(s.id) + ':servers"><summary title="agent-started servers still listening; open for the command and a kill line">'
    + '<span class="fl-srv-n">' + fleetEsc(list.length) + (list.length === 1 ? ' server' : ' servers') + '</span> · ' + ports
    + (oldest ? ' · oldest ' + fleetEsc(oldest) : '')
    + (stale ? ' · <span class="fl-srv-stale">' + fleetEsc(stale) + ' stale</span>' : '')
    + '</summary>' + list.map(function (v) { return fleetServerRow(v, now); }).join('') + '</details>';
}

// Summary-strip chips: total servers, and the stale ones when there are any.
function fleetServersCount(c) {
  var n = Number(c && c.servers) || 0, st = Number(c && c.servers_stale) || 0;
  if (n <= 0) return '';
  return '<span class="fl-count fl-srv-count"><b>' + fleetEsc(n) + '</b> ' + (n === 1 ? 'server' : 'servers') + '</span>'
    + (st > 0 ? '<span class="fl-count fl-srv-count fl-srv-stale"><b>' + fleetEsc(st) + '</b> stale</span>' : '');
}

module.exports = { fleetServersDur, fleetServersOf, fleetServersWhy, fleetServerRow, fleetServersLine, fleetServersCount };
