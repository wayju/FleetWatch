// Route for a message between two boxes of one session card (the card header
// and its subagent boxes). PURE: rectangles in, path and label out, no DOM.
//
// Card-to-card arrows leave a card's right edge and cannot describe traffic
// inside a card, so these stay inside it. Boxes in one column are joined by a
// bracket down the strip to their left (the card's inner left gutter for the
// first column): out of the sender's left edge, along the strip, into the
// receiver's left edge. Boxes in different columns are joined through the
// strip beside the receiver (or the sender, going left), which is the gap
// between grid columns and so clear of every box.
//
// Serialised into the page script by toString(); it calls fleetRoundPath.

const { fleetRoundPath } = require('./fleet-route');

// The middle of the edge a line meets: near the top for a tall box, so the
// port stays beside the box heading rather than far down its body.
function fleetRouteAnchor(r) { return r.y + Math.min(r.h / 2, 16); }

// Corner points for a line from src to dst inside `card`, lane 0 first.
// Returns { pts, vx } where vx is the x of the vertical run.
function fleetRouteInCardPoints(card, src, dst, lane) {
  var sy = fleetRouteAnchor(src), ey = fleetRouteAnchor(dst), nudge = lane * 3;
  if (Math.abs(src.x - dst.x) < 2) {
    // One column: the strip to its left, kept clear of the boxes.
    var room = src.x - card.x;
    var gx = card.x + (room < 4 ? room / 2 : Math.max(2, Math.min(3 + lane * 4, room - 2)));
    return { pts: [{ x: src.x, y: sy }, { x: gx, y: sy }, { x: gx, y: ey }, { x: dst.x, y: ey }], vx: gx };
  }
  if (dst.x > src.x) {
    var vx = Math.max(src.x + 1, dst.x - 4 - nudge), E = { x: dst.x, y: ey };
    if (src.x + src.w <= vx) return { pts: [{ x: src.x + src.w, y: sy }, { x: vx, y: sy }, { x: vx, y: ey }, E], vx: vx };
    // The sender spans over the receiver's column (the header): drop from its foot.
    return { pts: [{ x: vx, y: src.y + src.h }, { x: vx, y: ey }, E], vx: vx };
  }
  var wx = Math.max(dst.x + 1, src.x - 4 - nudge), S = { x: src.x, y: sy };
  if (dst.x + dst.w <= wx) return { pts: [S, { x: wx, y: sy }, { x: wx, y: ey }, { x: dst.x + dst.w, y: ey }], vx: wx };
  return { pts: [S, { x: wx, y: sy }, { x: wx, y: dst.y + dst.h }], vx: wx };
}

// card, src, dst: {x, y, w, h} in the board's coordinate space.
// o: { lane (0 for the first line in this card, then 1, 2...), label (text),
//      labelW (measured px, else estimated), bounds: {x1} }.
// Returns { points, d, start, end, label: {x, y, w, h} }, or null when the two
// ends are the same place and there is nothing to draw.
function fleetRouteInCard(card, src, dst, o) {
  o = o || {};
  var lane = o.lane || 0, pillH = 18, charW = 6.6, pad = 18, pillMax = 480;
  if (Math.abs(fleetRouteAnchor(src) - fleetRouteAnchor(dst)) < 1 && Math.abs(src.x - dst.x) < 1) return null;
  var r = fleetRouteInCardPoints(card, src, dst, lane);
  var pts = r.pts.filter(function (p, k, a) {
    return k === 0 || Math.abs(p.x - a[k - 1].x) > 0.01 || Math.abs(p.y - a[k - 1].y) > 0.01;
  });
  var S = pts[0], E = pts[pts.length - 1];
  var w = Math.min(pillMax, Math.round((o.labelW || String(o.label || '').length * charW) + pad));
  var right = o.bounds && o.bounds.x1 != null ? Math.min(o.bounds.x1, card.x + card.w) : card.x + card.w;
  var px = Math.max(card.x + 2, Math.min(r.vx + 10, right - w - 2));
  var py = (S.y + E.y) / 2 - pillH / 2 + lane * (pillH + 2);
  return { points: pts, d: fleetRoundPath(pts, 4), start: S, end: E, label: { x: px, y: py, w: w, h: pillH } };
}

module.exports = { fleetRouteAnchor, fleetRouteInCardPoints, fleetRouteInCard };
