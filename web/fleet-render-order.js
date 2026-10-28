// Session ordering for the fleet page. Same injection rules as
// fleet-render.js: PURE functions, no module-level bindings, shipped to the
// page by toString() in template-fleet.js (dependency order: fleetSortSessions
// first, then fleetOrder, then fleetSortBy).

// Oldest session first (id breaks ties and covers a missing start). The
// stable base every other order falls back to.
function fleetSortSessions(list) {
  return (list || []).slice().sort(function (a, b) {
    var ta = Number(a.started_at) || 0, tb = Number(b.started_at) || 0;
    if (ta && tb && ta !== tb) return ta - tb;
    if (ta && !tb) return -1;
    if (!ta && tb) return 1;
    var ia = String(a.id || ''), ib = String(b.id || '');
    return ia < ib ? -1 : ia > ib ? 1 : 0;
  });
}

// Orders: slot, started, attention, active, cost (highest API-equivalent first).
// Saved or unknown values fall back to the default, slot.
function fleetOrder(v) {
  return v === 'started' || v === 'attention' || v === 'active' || v === 'cost' ? v : 'slot';
}

// Sort a copy of the list by the chosen order. Ties always resolve through
// fleetSortSessions' ranking, so a card never moves for no reason.
function fleetSortBy(list, order) {
  var o = fleetOrder(order);
  var base = fleetSortSessions(list);
  if (o === 'started') return base;
  var rank = {};
  base.forEach(function (s, i) { rank[s.id] = i; });
  function slotNum(s) { var n = parseInt(s.slot, 10); return isFinite(n) ? n : 1e9; }
  function attn(s) {
    var st = s.state;
    if (!s.live) return 5;
    if (st === 'approval' || st === 'question') return 0;
    if (st === 'waiting') return 1;
    if (st === 'delegating' || st === 'working' || st === 'starting') return 2;
    if (st === 'idle') return 3;
    return 4;
  }
  function key(s) {
    if (o === 'cost') return -(Number(s.cost && s.cost.usd) || 0);
    if (o === 'active') return -(Number(s.last_activity) || 0);
    if (o === 'attention') return attn(s);
    return slotNum(s);
  }
  return base.slice().sort(function (a, b) {
    var d = key(a) - key(b);
    if (!d && (o === 'attention' || o === 'cost')) d = slotNum(a) - slotNum(b);
    return d || rank[a.id] - rank[b.id];
  });
}

module.exports = { fleetSortSessions, fleetOrder, fleetSortBy };
