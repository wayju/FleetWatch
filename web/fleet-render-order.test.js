// Tests for the fleet order selector.
const o = require('./fleet-render-order');
const r = require('./fleet-render');
const { fleetPageHTML, fleetClientScript } = require('./page');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

const NOW = 1792576900;
function s(over) {
  return Object.assign({ id: 'x', short: 'x', live: true, slot: '', state: 'idle', started_at: 100, last_activity: 100, account_key: 'a@x' }, over || {});
}
const ids = (list) => list.map((x) => x.id).join(',');

console.log('slot (default)');
const slots = [
  s({ id: 'def', slot: '', started_at: 10 }), s({ id: 'n10', slot: '10', started_at: 20 }), s({ id: 'n2', slot: '2', started_at: 30 }),
  s({ id: 'n2b', slot: '2', started_at: 25 }), s({ id: 'n2c', slot: '2', started_at: 25 }), s({ id: 'name', slot: 'work', started_at: 5 }),
];
assert(ids(o.fleetSortBy(slots)) === 'n2b,n2c,n2,n10,name,def', 'numeric slots first; ties by start then id; non-numeric and default last');
assert(ids(o.fleetSortBy(slots, 'slot')) === ids(o.fleetSortBy(slots)), 'no order given is slot');
assert(ids(o.fleetSortBy(slots, 'bogus')) === ids(o.fleetSortBy(slots)), 'unknown order falls back to slot');
assert(o.fleetOrder(undefined) === 'slot' && o.fleetOrder('nope') === 'slot' && o.fleetOrder('') === 'slot', 'fleetOrder defaults to slot');
assert(['slot', 'started', 'attention', 'active'].every((v) => o.fleetOrder(v) === v), 'fleetOrder keeps the four known values');

console.log('started');
assert(ids(o.fleetSortBy(slots, 'started')) === 'name,def,n10,n2b,n2c,n2', 'oldest first, id breaks ties');
assert(ids(o.fleetSortBy([s({ id: 'b' }), s({ id: 'a' }), s({ id: 'z', started_at: 0 })], 'started')) === 'a,b,z', 'unstarted last');

console.log('attention');
const att = [
  s({ id: 'idle', state: 'idle', slot: '1' }), s({ id: 'work', state: 'working', slot: '2' }), s({ id: 'wait', state: 'waiting', slot: '3' }),
  s({ id: 'appr', state: 'approval', slot: '5' }), s({ id: 'q', state: 'question', slot: '4' }), s({ id: 'del', state: 'delegating', slot: '1' }),
  s({ id: 'end', state: 'ended', live: false, slot: '0' }), s({ id: 'start', state: 'starting', slot: '6' }),
];
assert(ids(o.fleetSortBy(att, 'attention')) === 'q,appr,wait,del,work,start,idle,end', 'approval/question, waiting, active, idle, ended; ties by slot');

console.log('active');
const act = [s({ id: 'a', last_activity: 50 }), s({ id: 'b', last_activity: 300 }), s({ id: 'c', last_activity: 300, started_at: 50 }), s({ id: 'd', last_activity: 0 })];
assert(ids(o.fleetSortBy(act, 'active')) === 'c,b,a,d', 'most recent first, ties by start, no activity last');

console.log('stability');
const before = JSON.stringify(att);
['slot', 'started', 'attention', 'active'].forEach((v) => {
  const once = ids(o.fleetSortBy(att, v));
  assert(once === ids(o.fleetSortBy(att.slice().reverse(), v)), `${v}: input order does not matter`);
  assert(once === ids(o.fleetSortBy(o.fleetSortBy(att, v), v)), `${v}: idempotent`);
});
assert(JSON.stringify(att) === before, 'sorting does not mutate the input');
assert(ids(o.fleetSortBy(null)) === '' && ids(o.fleetSortBy(undefined, 'active')) === '', 'null list is empty');
assert(r.fleetSortSessions === o.fleetSortSessions, 'fleet-render re-exports fleetSortSessions');

console.log('render and page');
const board = { host: 'h', accounts: [{ key: 'a@x', email: 'a@x' }], sessions: [
  s({ id: 'one', slot: '9', state: 'idle', last_activity: 10 }), s({ id: 'two', slot: '3', state: 'approval', last_activity: 20, started_at: 200 }),
] };
const at = (html, id) => html.indexOf('id="fl-' + id + '"');
const bySlot = r.fleetAccounts(board, NOW, 'all', true);
assert(at(bySlot, 'two') < at(bySlot, 'one'), 'fleetAccounts defaults to slot order');
const byStart = r.fleetAccounts(board, NOW, 'all', true, 'started');
assert(at(byStart, 'one') < at(byStart, 'two'), 'fleetAccounts takes the order');
const byBogus = r.fleetAccounts(board, NOW, 'all', true, 'bogus');
assert(byBogus === bySlot, 'fleetAccounts with an unknown order equals slot');
const page = fleetPageHTML();
assert(/<select[^>]*id="fl-order"/.test(page) && page.includes('<span>order</span>'), 'page has the labelled order select');
['slot', 'started', 'attention', 'active'].forEach((v) => assert(page.includes(`<option value="${v}"`), `select offers ${v}`));
const script = fleetClientScript();
assert(script.includes('function fleetSortBy') && script.includes('function fleetOrder'), 'order helpers are injected into the page');
assert(script.indexOf('function fleetSortSessions') < script.indexOf('function fleetSortBy'), 'injected in dependency order');
assert(script.includes("FL.order=fleetOrder(saved.order)") && script.includes('order:FL.order'), 'order is loaded from and saved to fleet-view');
assert(script.includes("addEventListener('change'"), 'order is wired by a delegated change listener');
assert(!/\sonchange=|\sonclick=/.test(page), 'no inline handlers (CSP)');
try { new (require('vm').Script)(script); assert(true, 'client script parses'); } catch (e) { assert(false, 'client script parses: ' + e.message); }

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed ? 1 : 0);
