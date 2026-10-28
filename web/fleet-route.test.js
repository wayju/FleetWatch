// Tests for the pure arrow router: lane separation, no segment under a
// card, ports on the card edges, rounded corners and label placement.
const route = require('./fleet-route');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

// Two columns by three rows, 60px column gutter, 48px row gutter, 24px margins.
const W = 480, H = 300, CG = 60, RG = 48, M = 24;
const cards = [];
['A', 'B', 'C', 'D', 'E', 'F'].forEach((id, n) => {
  cards.push({ id, x: M + (n % 2) * (W + CG), y: M + Math.floor(n / 2) * (H + RG), w: W, h: H });
});
const bounds = { x0: 0, y0: 0, x1: 2 * W + CG + 2 * M, y1: 3 * H + 2 * RG + 2 * M };
const byId = {};
cards.forEach((c) => { byId[c.id] = c; });
const lk = (from, to, kind) => ({ from, to, kind: kind || 'message', label: 'slot 1 → slot 2 · "please review it" · 2m' });

const links = [
  lk('A', 'B'),            // same row, left to right
  lk('B', 'A'),            // same row, right to left
  lk('A', 'D'),            // cross row
  lk('C', 'D', 'handback'), // second link in the C/D gutter
  lk('E', 'B'),            // up two rows
  lk('F', 'E'),            // right to left on the bottom row
  lk('A', 'A'),            // self: not drawn
  lk('A', 'missing'),      // unknown card: not drawn
];
const out = route.fleetRoute(cards, links, { bounds });

console.log('routing');
assert(out.length === 6, 'self and unknown links are left out');
assert(out.every((r) => !r.fallback), 'every link finds a clear track on a roomy board');

function segs(r) { return r.points.slice(1).map((p, i) => [r.points[i], p]); }
function overlap(a1, a2, b1, b2) { return Math.min(Math.max(a1, a2), Math.max(b1, b2)) - Math.max(Math.min(a1, a2), Math.min(b1, b2)); }

console.log('ports and ends');
assert(out.every((r) => r.start.x === byId[r.from].x + byId[r.from].w), 'each line leaves its sender on the right edge');
assert(out.every((r) => r.end.x === byId[r.to].x), 'each line ends on its receiver left edge');
assert(out.every((r) => r.start.y > byId[r.from].y && r.start.y < byId[r.from].y + byId[r.from].h && r.end.y > byId[r.to].y && r.end.y < byId[r.to].y + byId[r.to].h), 'ports sit inside the card edge span');
assert(out.every((r) => segs(r)[0][0].y === segs(r)[0][1].y) && out.every((r) => { const s = segs(r); return s[s.length - 1][0].y === s[s.length - 1][1].y; }), 'lines leave and enter horizontally');
assert(out.every((r) => segs(r).every(([p, q]) => p.x === q.x || p.y === q.y)), 'every segment is horizontal or vertical');
const aOut = out.filter((r) => r.from === 'A').map((r) => r.start.y);
assert(aOut.length === 2 && aOut[0] !== aOut[1], 'two lines leaving one card get different ports');
const bIn = out.filter((r) => r.to === 'B').map((r) => r.end.y);
assert(bIn.length === 2 && bIn[0] !== bIn[1], 'two lines entering one card get different ports');

console.log('no line under a card');
let hits = 0;
out.forEach((r) => segs(r).forEach(([p, q]) => { if (route.fleetRouteHit(p, q, cards)) hits += 1; }));
assert(hits === 0, 'no segment passes through a card rect');
// Independent check of the same property, not via the router helper.
let bad = 0;
out.forEach((r) => segs(r).forEach(([p, q]) => cards.forEach((c) => {
  const ox = overlap(p.x, q.x, c.x + 1, c.x + c.w - 1), oy = overlap(p.y, q.y, c.y + 1, c.y + c.h - 1);
  if (ox > 0 && oy >= 0 && (Math.abs(p.y - q.y) < 0.01 ? p.y > c.y + 1 && p.y < c.y + c.h - 1 : ox >= 0 && p.x > c.x + 1 && p.x < c.x + c.w - 1 && oy > 0)) bad += 1;
})));
assert(bad === 0, 'no segment enters a card interior (independent check)');

console.log('lane separation');
let shared = 0;
for (let i = 0; i < out.length; i++) {
  for (let j = i + 1; j < out.length; j++) {
    segs(out[i]).forEach(([p, q]) => segs(out[j]).forEach(([r, s]) => {
      if (route.fleetRouteShares(p, q, r, s, 4)) shared += 1;
    }));
  }
}
assert(shared === 0, 'no two lines run along the same track (within 4px)');
// The A to B and B to A lines both use the A/B column gutter: distinct x lanes.
const verticalsIn = (r, lo, hi) => segs(r).filter(([p, q]) => p.x === q.x && p.x > lo && p.x < hi).map(([p]) => p.x);
const gutterX = verticalsIn(out[0], M + W, M + W + CG).concat(verticalsIn(out[1], M + W, M + W + CG), verticalsIn(out[2], M + W, M + W + CG));
assert(gutterX.length >= 4 && new Set(gutterX).size === gutterX.length, 'verticals sharing the column gutter each take their own lane');
assert(gutterX.every((x) => x > M + W + 2 && x < M + W + CG - 2), 'lanes stay inside the gutter');

console.log('labels');
const L = out.map((r) => r.label);
assert(L.every((r) => cards.every((c) => !(r.x < c.x + c.w && r.x + r.w > c.x && r.y < c.y + c.h && r.y + r.h > c.y))), 'no pill overlaps a card');
let pillClash = 0;
for (let i = 0; i < L.length; i++) for (let j = i + 1; j < L.length; j++) if (L[i].x < L[j].x + L[j].w && L[i].x + L[i].w > L[j].x && L[i].y < L[j].y + L[j].h && L[i].y + L[i].h > L[j].y) pillClash += 1;
assert(pillClash === 0, 'pills do not overlap each other');
assert(L.every((r) => r.x >= 0 && r.x + r.w <= bounds.x1), 'pills stay inside the board');
assert(L.every((r) => r.h === 18 && r.w > 100), 'pill size follows the text');

const long = route.fleetRoute(cards, [Object.assign(lk('A', 'B'), { label: 'x'.repeat(400) })], { bounds });
assert(long[0].label.w === 480, 'a long label is capped at the pill maximum');

console.log('path string');
const d = out[0].d;
assert(d.startsWith('M' + out[0].start.x + ' ' + out[0].start.y) && d.endsWith('L' + out[0].end.x + ' ' + out[0].end.y), 'path starts at the output port and ends exactly on the input port');
assert((d.match(/Q/g) || []).length === out[0].points.length - 2, 'one rounded corner per bend');
assert(route.fleetRoundPath([{ x: 0, y: 0 }, { x: 4, y: 0 }, { x: 4, y: 100 }], 9).includes('L2 0 Q4 0 4 2'), 'corner radius is clipped to half the short segment');

console.log('cramped board');
const tight = [{ id: 'A', x: 0, y: 0, w: 100, h: 60 }, { id: 'B', x: 104, y: 0, w: 100, h: 60 }];
const t = route.fleetRoute(tight, [lk('A', 'B')], { bounds: { x0: 0, y0: 0, x1: 204, y1: 60 } });
assert(t.length === 1 && t[0].fallback === true && t[0].end.x === 104, 'with no gutter wide enough the link still draws, flagged as a fallback');

console.log('pinned ports');
const pinned = route.fleetRoute(cards, [
  { from: 'A', to: 'B', kind: 'message', label: 'x', fromY: byId.A.y + 150, toY: byId.B.y + 200 },
  { from: 'A', to: 'B', kind: 'message', label: 'y' },
  { from: 'A', to: 'C', kind: 'message', label: 'z', fromY: byId.A.y + 150 },
], { bounds });
assert(pinned[0].start.y === byId.A.y + 150 && pinned[0].end.y === byId.B.y + 200, 'a link pinned to a height leaves and enters there');
assert(pinned[1].start.y !== pinned[0].start.y && pinned[1].start.y < byId.A.y + 150, 'an unpinned link steps round the pinned port');
assert(pinned[2].start.y !== pinned[0].start.y && Math.abs(pinned[2].start.y - pinned[0].start.y) >= 9, 'two links pinned to the same height are nudged apart');
const clamped = route.fleetRoute(cards, [{ from: 'A', to: 'B', kind: 'message', label: 'x', fromY: -500, toY: 99999 }], { bounds });
assert(clamped[0].start.y > byId.A.y && clamped[0].end.y < byId.B.y + byId.B.h, 'a pinned height outside the card is clamped onto its edge');

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed ? 1 : 0);
