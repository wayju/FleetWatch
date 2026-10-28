// Tests for the fleet live-activity helpers.
const a = require('./fleet-render-activity');
const r = require('./fleet-render');
const { fleetClientScript } = require('./page');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

const NOW = 1792576900;
function hhmmss(at) {
  const d = new Date(at * 1000);
  return [d.getHours(), d.getMinutes(), d.getSeconds()].map((n) => ('0' + n).slice(-2)).join(':');
}
function agent(over) {
  return Object.assign({ id: 'sub1', type: 'Explore', status: 'running', started_at: NOW - 60, last_active: NOW - 5, tool_count: 3, context: { source: 'none' } }, over || {});
}
function session(over) {
  return Object.assign({ id: 'aaaaaaaa-1111', short: 'aaaaaaaa', live: true, state: 'working', account_key: 'k', tools: [], agents: [], messages: [] }, over || {});
}

console.log('session activity line');
const act = r.fleetSession(session({ current: { name: 'Bash', target: 'curl -sI https://x.lan', description: 'Probing the origin check', status: 'running', at: NOW - 42 } }), NOW);
assert(act.includes('class="fl-act"') && act.includes('Probing the origin check'), 'shows the description');
assert(act.includes('fl-act-cmd fl-dim">curl -sI https://x.lan'), 'command dimmed on its own line');
assert(act.includes('<span class="fl-ago" data-t="' + (NOW - 42) + '">42s</span>'), 'elapsed is a ticking span');
assert(a.fleetActivity({ name: 'Bash', at: NOW - 3 }, NOW).includes('fl-act-text">Bash<'), 'falls back to the tool name without a description');
assert(!a.fleetActivity({ name: 'Bash', at: NOW }, NOW).includes('fl-act-cmd'), 'no command line without a target');
assert(r.fleetSession(session(), NOW).indexOf('fl-act"') < 0 && a.fleetActivity(null, NOW) === '', 'absent when current is null or missing');
const evil = r.fleetSession(session({ current: { name: '<b>n</b>', target: '"><img src=x>', description: '<script>1</script>', at: NOW } }), NOW);
assert(!evil.includes('<script>') && !evil.includes('<img') && !evil.includes('<b>n'), 'activity line escapes everything');

console.log('agent current');
const withDesc = r.fleetAgent(agent({ current: { name: 'Grep', target: 'TODO', description: 'Searching for TODOs', status: 'running', at: NOW } }), NOW);
assert(withDesc.includes('fl-ag-cur fl-ag-cur-d">') && withDesc.includes('Searching for TODOs') && withDesc.includes('fl-ag-cmd fl-dim">TODO'), 'description leads, target dimmed beneath');
assert(withDesc.indexOf('Grep') < 0, 'tool name replaced when a description exists');
const noDesc = r.fleetAgent(agent({ current: { name: 'Grep', target: 'TODO', status: 'running', at: NOW } }), NOW);
assert(noDesc.includes('Grep <span class="fl-dim">TODO</span>') && noDesc.indexOf('fl-ag-cmd') < 0, 'without description the old name + target line is kept');
assert(r.fleetAgent(agent(), NOW).indexOf('fl-ag-cur') < 0, 'no current, no line');

console.log('lifeline');
const ev = [{ kind: 'launch', at: NOW - 300 }, { kind: 'to', at: NOW - 200 }, { kind: 'from', at: NOW - 150 }, { kind: 'handback', at: NOW - 100 }, { kind: 'done', at: NOW - 90 }, { kind: 'error', at: NOW - 80 }];
const life = a.fleetLifeline(ev);
assert(['▶', '→', '←', '↩', '✓', '✗'].every((g) => life.includes('>' + g + '<')), 'all six glyphs render');
assert(life.includes('title="launch ' + hhmmss(NOW - 300) + '"') && life.includes('title="handback ' + hhmmss(NOW - 100) + '"'), 'tooltips carry kind and local HH:MM:SS');
assert(a.fleetLifeline(null) === '' && a.fleetLifeline([]) === '', 'empty without events');
const hostile = a.fleetLifeline([{ kind: '"><img src=x>', at: NOW }]);
assert(!hostile.includes('<img') && hostile.includes('title="&quot;&gt;&lt;img'), 'unknown hostile kind is escaped');
assert(r.fleetAgent(agent({ events: ev }), NOW).includes('class="fl-life"'), 'agent box includes the lifeline');
assert(r.fleetAgent(agent(), NOW).indexOf('fl-life') < 0, 'agent without events has no lifeline');

console.log('recent badge');
const hb = r.fleetAgent(agent({ status: 'done', handed_back_at: NOW - 30 }), NOW);
assert(hb.includes('fl-ag-recent') && hb.includes('↩ handed back <span class="fl-ago" data-t="' + (NOW - 30) + '" data-sfx=" ago">30s ago</span>'), 'hand-back within 120s highlights and badges');
const msg = r.fleetAgent(agent({ events: [{ kind: 'from', at: NOW - 20 }] }), NOW);
assert(msg.includes('fl-ag-recent') && msg.includes('← message '), 'a from-event within 120s badges as a message');
assert(r.fleetAgent(agent({ handed_back_at: NOW - 121 }), NOW).indexOf('fl-ag-recent') < 0, 'older than 120s: no highlight');
assert(r.fleetAgent(agent({ handed_back_at: NOW - 120 }), NOW).includes('fl-ag-recent'), 'exactly 120s still counts');
assert(r.fleetAgent(agent({ events: [{ kind: 'to', at: NOW - 5 }, { kind: 'launch', at: NOW - 6 }] }), NOW).indexOf('fl-ag-recent') < 0, 'to and launch events are not hand-overs');
assert(a.fleetRecent(agent({ handed_back_at: NOW - 50, events: [{ kind: 'from', at: NOW - 10 }] }), NOW).kind === 'from', 'newest hand-over wins');
assert(a.fleetRecent(agent({ handed_back_at: NOW - 10, events: [{ kind: 'from', at: NOW - 10 }] }), NOW).kind === 'handback', 'tie goes to the hand-back');
assert(a.fleetRecent(agent({ handed_back_at: 0 }), NOW) === null && a.fleetRecentBadge(null, NOW) === '', 'zero timestamp is no hand-back');

assert(r.fleetAgent(agent({ status: 'done', handed_back_at: NOW - 2 }), NOW).includes('>just now</span>'), 'under 5s reads "just now", not "now ago"');
assert(require('./fleet-render-base').fleetAgeSfx(28, ' ago') === '28s ago' && require('./fleet-render-base').fleetAgeSfx(2, ' ago') === 'just now' && require('./fleet-render-base').fleetAgeSfx(2, '') === 'now', 'fleetAgeSfx phrases');
assert(fleetClientScript().includes('fleetAgeSfx(now-Number(e.dataset.t),e.dataset.sfx'), 'the tick keeps the suffix');

console.log('running list drops the current tool');
const dup = r.fleetSession(session({ current: { name: 'Bash', target: 'npm test', status: 'running', at: NOW - 9 }, tools: [{ name: 'Bash', target: 'npm test', status: 'running', at: NOW - 9 }, { name: 'Read', target: 'a.js', status: 'running', at: NOW - 2 }] }), NOW);
assert((dup.match(/npm test/g) || []).length === 1 && dup.includes('a.js'), 'current appears once; other running tools stay');

console.log('card head chip');
const chip = r.fleetSession(session({ agents: [agent({ status: 'done', handed_back_at: NOW - 60 }), agent({ id: 's2', status: 'done', handed_back_at: NOW - 299 }), agent({ id: 's3', handed_back_at: NOW - 400 }), agent({ id: 's4' })] }), NOW);
assert(chip.includes('↩ 2 hand-backs (5m)'), 'counts hand-backs in the last 5 minutes, history agents included');
assert(a.fleetHandbackChip([agent({ handed_back_at: NOW - 10 })], NOW).includes('↩ 1 hand-back (5m)'), 'singular for one');
assert(a.fleetHandbackChip([agent({ handed_back_at: NOW - 301 })], NOW) === '' && a.fleetHandbackChip(undefined, NOW) === '', 'no chip when none are recent');

console.log('older boards');
const old = r.fleetSession(session({ agents: [agent({ current: { name: 'Grep', target: 'x', status: 'running', at: NOW } })] }), NOW);
assert(old.indexOf('fl-act"') < 0 && old.indexOf('fl-life') < 0 && old.indexOf('fl-ag-recent') < 0 && old.indexOf('fl-hb-chip') < 0, 'a board without the new fields renders as before');

console.log('slot picker');
const slots = r.fleetSlots({ sessions: [
  session({ id: 'c', slot: '10', state: 'waiting', title: 'Ten' }), session({ id: 'b', slot: '2', title: '<i>Two</i>"' }),
  session({ id: 'd', slot: '', title: 'Default' }), session({ id: 'e', slot: '1', live: false }), session({ id: 'a', slot: '1', state: 'approval' })] });
assert((slots.match(/class="fl-slot/g) || []).length === 4, 'one chip per live session only');
assert(slots.indexOf('>1<') < slots.indexOf('>2<') && slots.indexOf('>2<') < slots.indexOf('>10<') && slots.indexOf('>10<') < slots.indexOf('>def<'), 'sorted numerically, default last');
assert(slots.includes('fl-slot fl-st-approval" href="#fl-a"') && slots.includes('fl-slot fl-st-waiting" href="#fl-c"'), 'coloured by state, anchored to the card');
assert(!slots.includes('<i>') && slots.includes('&lt;i&gt;Two&lt;/i&gt;&quot;'), 'title escapes the headline');
assert(r.fleetSlots({}) === '' && r.fleetSlots(null) === '', 'empty board gives no chips');
assert(require('./page').fleetPageHTML().includes('id="fl-slots"') && /\.fl-card\{scroll-margin-top/.test(require('./page').fleetPageHTML()), 'page has the picker slot and card scroll margin');

console.log('delegating state');
assert(r.fleetState('delegating').label === 'DELEGATING' && r.fleetState('delegating').cls === 'fl-st-working', 'delegating renders as a working-class state');
assert(r.fleetVisible(session({ state: 'delegating' }), 'working', true) && !r.fleetVisible(session({ state: 'delegating' }), 'attention', true), 'working filter includes delegating, attention does not');
assert(r.fleetSession(session({ state: 'delegating' }), NOW).includes('DELEGATING'), 'card shows the DELEGATING pill');

console.log('page');
let compiles = true;
const script = fleetClientScript();
try { new Function(script); } catch (e) { compiles = false; console.error(e); }
assert(compiles && ['fleetActivity', 'fleetLifeline', 'fleetHandbackChip', 'fleetRecentBadge', 'fleetAgentCurrent', 'fleetClock', 'fleetRecent'].every((n) => script.includes('function ' + n + '(')), 'activity helpers are injected into the page');

console.log(`\n${passed} passed, ${failed} failed`);
if (failed) process.exit(1);
