// Tests for the in-card router: the path runs down the inner left gutter, from
// the sender box left edge to the receiver box left edge, with a pill beside it.
const rc = require('./fleet-route-card');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

const card = { x: 100, y: 50, w: 400, h: 500 };
const head = { x: 114, y: 62, w: 372, h: 40 };
const agA = { x: 114, y: 200, w: 180, h: 120 };
const agB = { x: 114, y: 340, w: 180, h: 120 };
const r = rc.fleetRouteInCard(card, head, agA, { label: 'main \u2192 general-purpose a39a913', labelW: 200, bounds: { x1: 600 } });

console.log('shape');
assert(r && r.start.x === head.x && r.end.x === agA.x, 'starts on the sender left edge and ends on the receiver left edge');
assert(r.points.length === 4 && r.points[1].x === r.points[2].x, 'out, down the gutter, in');
assert(r.points[1].x > card.x && r.points[1].x < head.x, 'the gutter lies between the card edge and the boxes');
assert(r.start.y >= head.y && r.start.y <= head.y + head.h && r.end.y >= agA.y && r.end.y <= agA.y + agA.h, 'ends sit within their boxes');
assert(r.points.slice(1).every((p, i) => p.x === r.points[i].x || p.y === r.points[i].y), 'every segment is horizontal or vertical');
assert(typeof r.d === 'string' && r.d.indexOf('M') === 0 && r.d.endsWith(r.end.x + ' ' + r.end.y), 'path data finishes exactly on the receiver');

console.log('direction');
const up = rc.fleetRouteInCard(card, agB, agA, { label: 'x', bounds: { x1: 600 } });
assert(up.start.y > up.end.y && up.start.x === agB.x && up.end.x === agA.x, 'a sibling above the sender is reached by running up the gutter');

console.log('label');
assert(r.label.w === 218 && r.label.h === 18, 'pill width is the measured text plus padding');
assert(r.label.x >= card.x && r.label.x + r.label.w <= card.x + card.w, 'pill stays inside the card');
const mid = (r.start.y + r.end.y) / 2;
assert(Math.abs(r.label.y + r.label.h / 2 - mid) < 1, 'pill sits half-way down the gutter');
const wide = rc.fleetRouteInCard(card, head, agA, { label: 'x', labelW: 900, bounds: { x1: 600 } });
assert(wide.label.w <= 480, 'pill width is capped');

console.log('lanes');
const l1 = rc.fleetRouteInCard(card, head, agB, { lane: 1, label: 'x', labelW: 40, bounds: { x1: 600 } });
assert(l1.points[1].x > r.points[1].x && l1.points[1].x < head.x, 'a second line in one card takes a lane further in, clear of the boxes');
assert(l1.label.y !== rc.fleetRouteInCard(card, head, agB, { lane: 0, label: 'x', labelW: 40, bounds: { x1: 600 } }).label.y, 'a second pill is offset so the two do not overlap');
const tight = rc.fleetRouteInCard({ x: 100, y: 0, w: 400, h: 500 }, { x: 102, y: 10, w: 100, h: 40 }, { x: 102, y: 100, w: 100, h: 40 }, { lane: 5, label: 'x', bounds: { x1: 600 } });
assert(tight.points[1].x > 100 && tight.points[1].x < 102, 'a card with almost no padding still gets a gutter inside it');

console.log('columns');
const agC = { x: 300, y: 200, w: 180, h: 120 }, agD = { x: 300, y: 340, w: 180, h: 120 };
const side = rc.fleetRouteInCard(card, agA, agC, { label: 'x', bounds: { x1: 600 } });
assert(side.start.x === agA.x + agA.w && side.end.x === agC.x, 'beside each other: leaves the sender facing edge, enters the receiver left edge');
assert(side.points[1].x >= agA.x + agA.w && side.points[1].x < agC.x, 'the run sits in the gap between the columns');
const diag = rc.fleetRouteInCard(card, agA, agD, { label: 'x', bounds: { x1: 600 } });
assert(diag.points.every((p) => !(p.x > agA.x && p.x < agA.x + agA.w && p.y > agA.y + agA.h)), 'a line to another column never runs under the sender column');
assert(diag.start.x === agA.x + agA.w && diag.end.x === agD.x && diag.end.y > agD.y, 'down and across ends on the receiver');
const back = rc.fleetRouteInCard(card, agC, agA, { label: 'x', bounds: { x1: 600 } });
assert(back.start.x === agC.x && back.end.x === agA.x + agA.w && back.points[1].x > agA.x + agA.w, 'going left leaves the sender left edge and enters the receiver right edge');
const fromHead = rc.fleetRouteInCard(card, head, agC, { label: 'x', bounds: { x1: 600 } });
assert(fromHead.start.y === head.y + head.h && fromHead.end.x === agC.x && fromHead.points[0].x < agC.x, 'the header drops from its foot, in the gap beside the receiver column');
const toHead = rc.fleetRouteInCard(card, agC, head, { label: 'x', bounds: { x1: 600 } });
assert(toHead.start.x === agC.x && toHead.end.y === head.y + head.h, 'a hand-back from another column rises into the foot of the header');

console.log('degenerate');
assert(rc.fleetRouteInCard(card, agA, agA, { label: 'x' }) === null, 'the same box at both ends draws nothing');

console.log(`${passed} passed, ${failed} failed`);
if (failed) process.exit(1);
