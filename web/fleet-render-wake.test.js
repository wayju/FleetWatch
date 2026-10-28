// Tests for the fleet wake countdown helpers.
const w = require('./fleet-render-wake');
const r = require('./fleet-render');
const { fleetClientScript } = require('./page');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

const NOW = 1792576900;
const timer = (secs, reason) => ({ kind: 'timer', at: NOW + secs, reason: reason === undefined ? 'poll CI' : reason });
const cond = { kind: 'condition', reason: '2 background tasks' };
const sess = (state, next_wake) => ({ id: 's1', short: 'abc', state, live: true, slot: '3', next_wake });

console.log('minutes');
assert(w.fleetWakeMins(NOW + 2580, NOW) === 43, '43 minutes exactly');
assert(w.fleetWakeMins(NOW + 2581, NOW) === 44, 'rounds up');
assert(w.fleetWakeMins(NOW + 1, NOW) === 1, 'minimum 1 while pending');
assert(w.fleetWakeMins(NOW, NOW) === 0 && w.fleetWakeMins(NOW - 30, NOW) === 0, 'at or after the time is 0');
assert(w.fleetWakeText(NOW + 2580, NOW) === '43m' && w.fleetWakeText(NOW, NOW) === 'due', 'text is Nm or due');
assert(w.fleetWakeMins('x', NOW) === 0, 'bad input is 0');

console.log('badge');
const b = w.fleetWakeBadge(sess('waiting', timer(2580)), NOW);
assert(b.includes('⏰') && b.includes('>43m<') && b.includes('title="poll CI"') && b.includes('data-wake-at="' + (NOW + 2580) + '"'), 'timer badge on a waiting session');
assert(w.fleetWakeBadge(sess('idle', timer(2580)), NOW).includes('43m'), 'idle shows too');
assert(w.fleetWakeBadge(sess('waiting', timer(-5)), NOW).includes('>due<'), 'past timer reads due');
const cb = w.fleetWakeBadge(sess('waiting', cond), NOW);
assert(cb.includes('⏳ on condition') && cb.includes('title="2 background tasks"') && !cb.includes('data-wake-at'), 'condition badge, not ticked');
assert(w.fleetWakeBadge(sess('waiting', null), NOW) === '' && w.fleetWakeBadge(sess('waiting'), NOW) === '', 'null or absent shows nothing');
assert(w.fleetWakeBadge(sess('working', cond), NOW).includes('⏳ on condition') && w.fleetWakeSuffix(sess('working', timer(2580)), NOW).includes('(43)'), 'working shows the wake the fleet promoted it for');
assert(w.fleetWakeBadge(sess('working', null), NOW) === '', 'plain working shows nothing');
for (const st of ['delegating', 'approval', 'question', 'starting', 'ended']) {
  assert(w.fleetWakeBadge(sess(st, timer(600)), NOW) === '' && w.fleetWakeSuffix(sess(st, cond), NOW) === '', st + ' shows nothing');
}
assert(w.fleetWakeBadge(sess('waiting', { kind: 'timer', reason: 'no at' }), NOW) === '', 'timer without at shows nothing');
assert(w.fleetWakeBadge(sess('waiting', { kind: 'bogus', at: NOW + 60 }), NOW) === '', 'unknown kind shows nothing');
const evil = w.fleetWakeBadge(sess('waiting', timer(120, '"><script>alert(1)</script>')), NOW);
assert(!evil.includes('<script>') && evil.includes('&quot;&gt;&lt;script&gt;'), 'reason is escaped');
assert(!/\son[a-z]+=/i.test(b + cb), 'no inline handlers');

console.log('slot suffix');
assert(w.fleetWakeSuffix(sess('waiting', timer(2580)), NOW).includes('(43)'), 'timer chip suffix (43)');
assert(w.fleetWakeSuffix(sess('waiting', timer(-5)), NOW).includes('(due)') && !w.fleetWakeSuffix(sess('waiting', timer(-5)), NOW).includes('(0)'), 'due chip reads (due)');
assert(w.fleetWakeSuffix(sess('idle', cond), NOW) === ' (C)', 'condition chip suffix (C)');
assert(w.fleetWakeSuffix(sess('waiting', null), NOW) === '', 'no suffix without a wake');

console.log('page integration');
const board = { sessions: [sess('waiting', timer(2580)), Object.assign(sess('idle', cond), { id: 's2', slot: '4' }), Object.assign(sess('delegating', timer(600)), { id: 's3', slot: '5' })] };
const slots = r.fleetSlots(board, NOW);
assert(slots.includes('3 <span data-wake-at') && slots.includes('4 (C)') && !/>5 [(<]/.test(slots), 'slot chips carry the suffix for waiting and idle, never delegating');
const card = r.fleetSession(board.sessions[0], NOW);
assert(card.includes('fl-wake-timer') && card.indexOf('fl-pill') < card.indexOf('fl-wake-timer'), 'card badge follows the pill');
assert(!r.fleetSession(board.sessions[2], NOW).includes('fl-wake'), 'delegating card has no badge');
const old = Object.assign({}, board.sessions[0]); delete old.next_wake;
assert(!r.fleetSession(old, NOW).includes('fl-wake'), 'card without next_wake renders as before');
const script = fleetClientScript();
assert(script.includes('function fleetWakeBadge') && script.includes('[data-wake-at]'), 'helpers injected and ticked');
try { new Function(script.replace('flConnect();', '')); assert(true, 'client script parses'); } catch (e) { assert(false, 'client script parses: ' + e.message); }

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed ? 1 : 0);
