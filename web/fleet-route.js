// Orthogonal routing for the message arrows on /fleet. PURE: card
// rectangles and links in, paths and label points out, no DOM. The page script
// feeds it measured rectangles and draws the result.
//
// Every route leaves the sender's output port (right edge) horizontally, runs
// down a vertical gutter, along a horizontal gutter (the row gap, where the
// label pill sits), down a second vertical gutter and enters the receiver's
// input port (left edge) horizontally. A route is only accepted when none of
// its segments crosses a card, and each use of a gutter takes its own lane, so
// two lines never share a track.
//
// Functions are serialised into the page script by toString(), so each is
// self-contained apart from calls to the other fleetRoute* helpers.

// Free intervals of an axis: the stretches between consecutive card edges that
// are at least minGap wide, bounded by lo/hi (the board edges).
function fleetRouteGaps(edges, lo, hi, minGap) {
  var e = edges.concat([lo, hi]).filter(function (v) { return v >= lo && v <= hi; }).sort(function (a, b) { return a - b; });
  var out = [];
  for (var i = 0; i + 1 < e.length; i++) {
    if (e[i + 1] - e[i] >= minGap) out.push({ lo: e[i], hi: e[i + 1], mid: (e[i] + e[i + 1]) / 2, key: String(e[i]) });
  }
  return out;
}

// First card the segment p-q passes through, or null. Touching an edge is fine:
// ports sit exactly on the edge.
function fleetRouteHit(p, q, cards) {
  var x1 = Math.min(p.x, q.x), x2 = Math.max(p.x, q.x), y1 = Math.min(p.y, q.y), y2 = Math.max(p.y, q.y);
  if (x2 - x1 < 0.01 && y2 - y1 < 0.01) return null;
  for (var i = 0; i < cards.length; i++) {
    var c = cards[i];
    if (x2 > c.x + 0.5 && x1 < c.x + c.w - 0.5 && y2 > c.y + 0.5 && y1 < c.y + c.h - 0.5) return c;
  }
  return null;
}

// True when two axis-aligned segments run along the same track and overlap.
function fleetRouteShares(p, q, r, s, tol) {
  var ph = Math.abs(p.y - q.y) < 0.01, rh = Math.abs(r.y - s.y) < 0.01;
  if (ph && rh) return Math.abs(p.y - r.y) < tol && Math.min(Math.max(p.x, q.x), Math.max(r.x, s.x)) - Math.max(Math.min(p.x, q.x), Math.min(r.x, s.x)) > 1;
  var pv = Math.abs(p.x - q.x) < 0.01, rv = Math.abs(r.x - s.x) < 0.01;
  if (pv && rv) return Math.abs(p.x - r.x) < tol && Math.min(Math.max(p.y, q.y), Math.max(r.y, s.y)) - Math.max(Math.min(p.y, q.y), Math.min(r.y, s.y)) > 1;
  return false;
}

// Position of the nth lane in a gap: centre, then alternately either side.
// `half` keeps the line (or pill) clear of the gap edges; null when it won't fit.
function fleetRouteLane(gap, n, step, half) {
  var lo = gap.lo + half, hi = gap.hi - half;
  var off = n === 0 ? 0 : (n % 2 === 1 ? 1 : -1) * Math.ceil(n / 2) * step;
  var pos = gap.mid + off;
  if (hi < lo) return n === 0 ? gap.mid : null;
  return pos < lo || pos > hi ? null : pos;
}

// SVG path through points with rounded corners (quadratic, radius r, clipped
// to half of each neighbouring segment). The last point is hit exactly.
function fleetRoundPath(pts, r) {
  var d = 'M' + pts[0].x + ' ' + pts[0].y;
  for (var i = 1; i < pts.length - 1; i++) {
    var a = pts[i - 1], b = pts[i], c = pts[i + 1];
    var l1 = Math.abs(b.x - a.x) + Math.abs(b.y - a.y), l2 = Math.abs(c.x - b.x) + Math.abs(c.y - b.y);
    var k = Math.min(r, l1 / 2, l2 / 2);
    var ux = Math.sign(b.x - a.x), uy = Math.sign(b.y - a.y), vx = Math.sign(c.x - b.x), vy = Math.sign(c.y - b.y);
    d += ' L' + (b.x - ux * k) + ' ' + (b.y - uy * k) + ' Q' + b.x + ' ' + b.y + ' ' + (b.x + vx * k) + ' ' + (b.y + vy * k);
  }
  var e = pts[pts.length - 1];
  return d + ' L' + e.x + ' ' + e.y;
}

// Ports stack down each card's edge, ordered by where the other end is, so
// lines to a card above leave above those to a card below. A link may pin its
// port to a height (fromY/toY, in board space: where a subagent box sits); a
// pinned port is clamped to the edge and nudged off its neighbours, and the
// stack steps round it. Sets t.fromY/t.toY.
function fleetRoutePorts(todo, byId, o) {
  function stack(side, other) {
    var groups = {}, taken = {}, pins = [];
    function near(id, y) { return (taken[id] || []).some(function (v) { return Math.abs(v - y) < o.portStep - 1; }); }
    function put(t, id, y) { (taken[id] = taken[id] || []).push(y); t[side + 'Y'] = y; }
    todo.forEach(function (t) {
      var id = t.l[side];
      if (t.l[side + 'Y'] == null) (groups[id] = groups[id] || []).push(t);
      else pins.push(t);
    });
    pins.forEach(function (t) {
      var id = t.l[side], c = byId[id], hi = c.y + c.h - 6;
      var y = Math.max(c.y + o.portTop, Math.min(hi, t.l[side + 'Y']));
      while (near(id, y) && y + o.portStep <= hi) y += o.portStep;
      put(t, id, y);
    });
    Object.keys(groups).forEach(function (id) {
      var c = byId[id], k = 0, hi = c.y + c.h - 6;
      groups[id].sort(function (p, q) { return byId[p.l[other]].y - byId[q.l[other]].y || p.index - q.index; });
      groups[id].forEach(function (t) {
        var y;
        do { y = Math.min(c.y + o.portTop + k * o.portStep, hi); k++; } while (near(id, y) && y < hi);
        put(t, id, y);
      });
    });
  }
  stack('from', 'to');
  stack('to', 'from');
}

// The cheapest clear route from S to E: leave right into a gutter (xa), cross
// to a row gap (py), run along it, drop into a gutter beside the receiver (xb).
// ctx carries the geometry and the lanes and segments already taken.
function fleetRouteSearch(ctx, S, E) {
  var o = ctx.o, used = ctx.used, best = null;
  var xa = ctx.xGaps.filter(function (g) { return g.lo >= S.x - 0.5; }).sort(function (p, q) { return Math.abs(p.mid - S.x) - Math.abs(q.mid - S.x); }).slice(0, 3);
  var xb = ctx.xGaps.filter(function (g) { return g.hi <= E.x + 0.5; }).sort(function (p, q) { return Math.abs(p.mid - E.x) - Math.abs(q.mid - E.x); }).slice(0, 3);
  function laneOf(gap, extra, step, half) { return fleetRouteLane(gap, (used[gap.key] || 0) + extra, step, half); }
  function clear(pts) {
    for (var k = 0; k + 1 < pts.length; k++) {
      if (fleetRouteHit(pts[k], pts[k + 1], ctx.cards)) return false;
      for (var m = 0; m < ctx.segs.length; m++) {
        if (fleetRouteShares(pts[k], pts[k + 1], ctx.segs[m][0], ctx.segs[m][1], 4)) return false;
      }
    }
    return true;
  }
  function length(pts) {
    var len = 0;
    for (var k = 0; k + 1 < pts.length; k++) len += Math.abs(pts[k + 1].x - pts[k].x) + Math.abs(pts[k + 1].y - pts[k].y);
    return len;
  }
  xa.forEach(function (ga) {
    var pa = laneOf(ga, 0, o.lane, o.clear);
    if (pa == null) return;
    ctx.yGaps.forEach(function (gy) {
      var py = laneOf(gy, 0, o.hlane, o.pillH / 2 + 1), roomy = gy.hi - gy.lo >= o.pillH + 2;
      if (py == null) { py = fleetRouteLane(gy, 0, o.hlane, 0); roomy = false; }
      if (py == null) return;
      xb.forEach(function (gb) {
        var pb = laneOf(gb, gb === ga ? 1 : 0, o.lane, o.clear);
        if (pb == null) return;
        var pts = [S, { x: pa, y: S.y }, { x: pa, y: py }, { x: pb, y: py }, { x: pb, y: E.y }, E];
        if (!clear(pts)) return;
        var cost = length(pts) + (roomy ? 0 : 300) + 6 * ((used[ga.key] || 0) + (used[gy.key] || 0) + (used[gb.key] || 0));
        if (!best || cost < best.cost) best = { cost: cost, pts: pts, gaps: [ga, gy, gb] };
      });
    });
  });
  if (best) return best;
  // No clear track (a very cramped board): the plain Z, flagged, so the link still shows.
  var xm = (S.x + E.x) / 2;
  return { pts: [S, { x: xm, y: S.y }, { x: xm, y: E.y }, E], gaps: [], fallback: true };
}

// The pill sits on the longest horizontal run that is clear of the port rows
// (the row-gap leg), slid sideways past pills already placed.
function fleetRoutePill(ctx, pts, S, E, text) {
  var o = ctx.o, b = ctx.b, run = null, k;
  for (k = 0; k + 1 < pts.length; k++) {
    var p = pts[k], q = pts[k + 1];
    if (Math.abs(p.y - q.y) < 0.01 && p.y !== S.y && p.y !== E.y && (!run || Math.abs(q.x - p.x) > run.len)) run = { y: p.y, cx: (p.x + q.x) / 2, len: Math.abs(q.x - p.x) };
  }
  if (!run) run = { y: (S.y + E.y) / 2, cx: (S.x + E.x) / 2, len: 0 };
  var w = Math.min(o.pillMax, Math.round((text && text.w ? text.w : String(text || '').length * o.charW) + o.pillPad)), h = o.pillH;
  function at(shift) { return { x: Math.max(b.x0 + 2, Math.min(b.x1 - w - 2, run.cx + shift - w / 2)), y: run.y - h / 2, w: w, h: h }; }
  for (var j = 0; j < 9; j++) {
    var cand = at(j === 0 ? 0 : (j % 2 ? 1 : -1) * Math.ceil(j / 2) * (w + 6));
    var clash = ctx.placed.some(function (r) { return cand.x < r.x + r.w && cand.x + cand.w > r.x && cand.y < r.y + r.h && cand.y + cand.h > r.y; });
    if (!clash) return cand;
  }
  return at(0);
}

// Row gaps: horizontal bands with no card in them at any x. Only the stretch
// next to a card is kept (a gap holding an account header is tall, and a line
// mid-way down it would cross the header text).
function fleetRouteRows(cards, b, o) {
  // Bands are non-card blocks (account headers, the log) that keep horizontal
  // lanes out without stopping a vertical one from passing.
  var blocks = cards.concat(o.bands || []);
  var gaps = fleetRouteGaps([].concat.apply([], blocks.map(function (c) { return [c.y, c.y + c.h]; })), b.y0, b.y1, o.minGap);
  // A usable gap is empty at every x and borders a card, so a pill never floats
  // over a header or the page margin.
  return gaps.filter(function (g) {
    var touches = cards.some(function (c) { return Math.abs(c.y + c.h - g.lo) < 0.5 || Math.abs(c.y - g.hi) < 0.5; });
    return touches && !blocks.some(function (c) { return c.y < g.hi - 0.01 && c.y + c.h > g.lo + 0.01; });
  }).map(function (g) {
    var lo = g.lo, hi = g.hi;
    if (hi - lo > o.rowBand) { if (lo <= b.y0 + 0.01) lo = hi - o.rowBand; else hi = lo + o.rowBand; }
    return { lo: lo, hi: hi, mid: (lo + hi) / 2, key: 'y' + g.key };
  });
}

// cards: [{id, x, y, w, h}] in one coordinate space (the board's).
// links: [{from, to, kind, label, labelW, fromY, toY}] where from/to are card ids,
//   label is the pill text and labelW its measured width in px (else estimated
//   from the length of label); fromY/toY optionally pin a port's height.
// opts: { bounds: {x0, y0, x1, y1}, bands: [rects] } plus optional tuning (lane, hlane, ...).
// Returns one entry per routable link:
//   { index, from, to, kind, points, d, start, end, label: {x, y, w, h} }
// Links to a missing card, or from a card to itself, are left out.
function fleetRoute(cards, links, opts) {
  var o = Object.assign({ lane: 6, hlane: 10, radius: 9, portTop: 22, portStep: 10, pillH: 18, charW: 6.6, pillPad: 18, pillMax: 480, minGap: 14, clear: 3, rowBand: 60 }, opts || {});
  var byId = {};
  cards.forEach(function (c) { byId[c.id] = c; });
  var b = o.bounds || { x0: 0, y0: 0, x1: Math.max.apply(null, cards.map(function (c) { return c.x + c.w; })) + 24, y1: Math.max.apply(null, cards.map(function (c) { return c.y + c.h; })) + 24 };
  var ctx = {
    o: o, b: b, cards: cards, used: {}, segs: [], placed: [],
    xGaps: fleetRouteGaps([].concat.apply([], cards.map(function (c) { return [c.x, c.x + c.w]; })), b.x0, b.x1, o.minGap).map(function (g) { g.key = 'x' + g.key; return g; }),
    yGaps: fleetRouteRows(cards, b, o),
  };
  var todo = [];
  links.forEach(function (l, index) {
    if (l.from !== l.to && byId[l.from] && byId[l.to]) todo.push({ l: l, index: index });
  });
  fleetRoutePorts(todo, byId, o);
  return todo.map(function (t) {
    var src = byId[t.l.from], dst = byId[t.l.to];
    var S = { x: src.x + src.w, y: t.fromY }, E = { x: dst.x, y: t.toY };
    var best = fleetRouteSearch(ctx, S, E);
    best.gaps.forEach(function (g) { ctx.used[g.key] = (ctx.used[g.key] || 0) + 1; });
    var pts = best.pts.filter(function (p, k, a) { return k === 0 || Math.abs(p.x - a[k - 1].x) > 0.01 || Math.abs(p.y - a[k - 1].y) > 0.01; });
    for (var k = 0; k + 1 < pts.length; k++) ctx.segs.push([pts[k], pts[k + 1]]);
    var pill = fleetRoutePill(ctx, pts, S, E, t.l.labelW ? { w: t.l.labelW } : t.l.label);
    ctx.placed.push(pill);
    return { index: t.index, from: t.l.from, to: t.l.to, kind: t.l.kind, points: pts, d: fleetRoundPath(pts, o.radius), start: S, end: E, label: pill, fallback: !!best.fallback };
  });
}

module.exports = { fleetRoute, fleetRouteGaps, fleetRouteHit, fleetRouteShares, fleetRouteLane, fleetRoundPath, fleetRouteRows, fleetRoutePorts, fleetRouteSearch, fleetRoutePill };
