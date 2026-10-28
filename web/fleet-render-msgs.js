// Inline "messages" list for one session card. On narrow screens the
// dashed connectors between cards cannot be drawn legibly, so each card lists
// the inter-session messages it sent or received instead: direction, the other
// side, age and preview. Hidden by CSS above 700px, where the arrows remain.
//
// PURE, same injection rules as fleet-render.js (toString() into the page).

const { fleetEsc, fleetAgo, fleetAge } = require('./fleet-render-base');

// Friendly name for a session id: "slot N", else the short id, else the id.
function fleetMsgName(board, id) {
  var list = (board && board.sessions) || [];
  for (var i = 0; i < list.length; i++) {
    if (list[i].id === id) return list[i].slot ? 'slot ' + list[i].slot : String(list[i].short || id);
  }
  return String(id || '');
}

// Links touching session s from the last 30 minutes (longer than the 30-second
// arrows, so a finished exchange stays readable), newest first, at most five. External links have no card to jump to.
function fleetMsgList(s, board, now) {
  var rows = ((board && board.links) || []).filter(function (l) {
    return (l.from === s.id || l.to === s.id) && now - l.at <= 1800;
  }).sort(function (a, b) { return b.at - a.at; }).slice(0, 5);
  if (!rows.length) return '';
  return '<div class="fl-xmsgs"><div class="fl-xmsgs-h">messages</div>' + rows.map(function (l) {
    var out = l.from === s.id;
    var other = out ? l.to : l.from;
    var label = l.external ? (l.to_label || other) : fleetMsgName(board, other);
    var who = l.external ? fleetEsc(label) : '<a href="#fl-' + fleetEsc(other) + '">' + fleetEsc(label) + '</a>';
    return '<div class="fl-xmsg"><span class="fl-xmsg-dir">' + (out ? '→ ' : '← ') + who + '</span>'
      + fleetAgo(l.at, now, ' ago') + '<span class="fl-xmsg-text">' + fleetEsc(l.preview || '') + '</span></div>';
  }).join('') + '</div>';
}

// Pill text for a desktop arrow: "slot 1 → slot 2 · "preview…" · 2m". `text` is
// clipped to one line; `full` is the tooltip. Both are plain text, escaped by
// whoever puts them in the page.
function fleetLinkLabel(board, l, now) {
  var from = fleetMsgName(board, l.from), to = fleetMsgName(board, l.to);
  var full = String(l.preview || '').replace(/\s+/g, ' ').trim();
  var clip = full.length > 36 ? full.slice(0, 35).replace(/\s+$/, '') + '\u2026' : full;
  var age = fleetAge(Math.max(0, now - l.at));
  var q = function (t) { return t ? ' \u00b7 \u201c' + t + '\u201d' : ''; };
  return { text: from + ' \u2192 ' + to + q(clip) + ' \u00b7 ' + age, full: from + ' \u2192 ' + to + q(full) + ' \u00b7 ' + age };
}

module.exports = { fleetMsgName, fleetMsgList, fleetLinkLabel };
