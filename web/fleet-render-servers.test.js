// Tests for the agent-started dev servers line and fleet count.
const sv = require('./fleet-render-servers');
const r = require('./fleet-render');
const { fleetClientScript } = require('./page');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

const NOW = 1792576900;
const H = 3600;
const srv = (pid, port, ageSec, extra) => Object.assign({ pid, port, cmd: 'python3 -m http.server ' + port, cwd: '/home/u/src/app', dir: '/home/u/src/app/.claude/worktrees/x', started_at: NOW - ageSec, stale: false }, extra);
const sess = (servers) => ({ id: 's1', short: 'abc', state: 'working', live: true, slot: '3', servers });

console.log('duration');
assert(sv.fleetServersDur(40 * H + 5) === '1d 16h', '40 h reads 1d 16h');
assert(sv.fleetServersDur(5 * H + 20 * 60) === '5h 20m', 'hours and minutes');
assert(sv.fleetServersDur(90) === '1m' && sv.fleetServersDur(10) === '<1m', 'minutes and under');
assert(sv.fleetServersDur(-5) === '' && sv.fleetServersDur('x') === '', 'bad input is empty');

console.log('line');
assert(sv.fleetServersLine(sess([]), NOW) === '' && sv.fleetServersLine(sess(), NOW) === '' && sv.fleetServersLine({}, NOW) === '', 'none renders nothing');
const line = sv.fleetServersLine(sess([srv(11, 8791, 40 * H), srv(12, 5173, H), srv(13, 3000, 2 * H)]), NOW);
assert(line.includes('3 servers') && line.indexOf(':8791') < line.indexOf(':3000') && line.indexOf(':3000') < line.indexOf(':5173') && line.includes('oldest 1d 16h'), 'count, ports oldest first, oldest age');
assert(sv.fleetServersLine(sess([srv(11, 80, H)]), NOW).includes('1 server<') || sv.fleetServersLine(sess([srv(11, 80, H)]), NOW).includes('1 server '), 'singular');
const st = sv.fleetServersLine(sess([srv(11, 8791, 40 * H, { stale: true, stale_reason: 'age' }), srv(12, 5173, H)]), NOW);
assert(st.includes('fl-srv-stale') && st.includes('1 stale') && /fl-srv-port fl-srv-stale[^>]*>:8791/.test(st) && !/fl-srv-stale[^>]*>:5173/.test(st), 'stale port and count marked distinctly');
assert(!sv.fleetServersLine(sess([srv(12, 5173, H)]), NOW).includes('stale'), 'no stale mark when none stale');

console.log('detail');
assert(line.includes('<details') && line.includes('data-key="s1:servers"'), 'details keeps its open state across boards');
assert(line.includes('<code class="fl-srv-kill">kill 11</code>') && line.includes('kill 12') && line.includes('kill 13'), 'copyable kill text per server');
assert(line.includes('python3 -m http.server 8791') && line.includes('/home/u/src/app/.claude/worktrees/x'), 'command and directory shown');
assert(sv.fleetServersLine(sess([srv(11, 1, H, { stale: true, stale_reason: 'merged' })]), NOW).includes('branch merged'), 'stale reason in words');
assert(sv.fleetServersLine(sess([srv(11, 1, H, { cmd: '', dir: '', cwd: '' })]), NOW).includes('kill 11'), 'works with text hidden');
assert(!/<(button|a)\b/.test(line) && !/\son[a-z]+=/i.test(line), 'no buttons, links or inline handlers');

console.log('escaping');
const evil = sv.fleetServersLine(sess([srv(11, 80, H, { cmd: '"><script>alert(1)</script>', dir: '<img src=x onerror=1>', stale: true, stale_reason: '"><b>' })]), NOW);
assert(!evil.includes('<script>') && !evil.includes('<img') && !evil.includes('&quot;><b>') && !evil.includes('"><b>') && evil.includes('&lt;script&gt;'), 'cmd, dir and reason escaped');
const badPid = sv.fleetServersLine(sess([srv('1; rm -rf /', 80, H), srv(14, 'x', H)]), NOW);
assert(badPid === '', 'non-numeric pid or port dropped');

console.log('fleet count');
assert(sv.fleetServersCount({}) === '' && sv.fleetServersCount({ servers: 0 }) === '' && sv.fleetServersCount() === '', 'zero renders nothing');
const c = sv.fleetServersCount({ servers: 5, servers_stale: 2 });
assert(c.includes('<b>5</b> servers') && c.includes('<b>2</b> stale') && c.includes('fl-srv-stale'), 'total and stale chips');
assert(!sv.fleetServersCount({ servers: 5, servers_stale: 0 }).includes('stale'), 'no stale chip at zero');
assert(sv.fleetServersCount({ servers: 1 }).includes('<b>1</b> server<'), 'singular count');

console.log('page integration');
const card = r.fleetSession(sess([srv(11, 8791, H)]), NOW);
assert(card.includes('fl-servers') && card.includes(':8791'), 'card carries the servers line');
assert(!r.fleetSession(sess([]), NOW).includes('fl-servers'), 'card without servers renders as before');
assert(r.fleetCounts({ live: 2, servers: 3, servers_stale: 1 }).includes('<b>3</b> servers'), 'summary strip carries the count');
assert(!r.fleetCounts({ live: 2 }).includes('servers'), 'summary strip unchanged without servers');
const script = fleetClientScript();
assert(script.includes('function fleetServersLine') && script.includes('function fleetServersDur') && script.includes('function fleetServersCount'), 'helpers injected');
try { new Function(script.replace('flConnect();', '')); assert(true, 'client script parses'); } catch (e) { assert(false, 'client script parses: ' + e.message); }

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed ? 1 : 0);
