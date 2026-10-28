// A subagent parked on its own background work renders as that, not as done.
const r = require('./fleet-render');
const { fleetClientScript, fleetPageHTML } = require('./page');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

const NOW = 1792576900;
function agent(over) {
  return Object.assign({ id: 'a0ee46c362f79143d', type: 'code-reviewer', status: 'parked', background: true, started_at: NOW - 600, last_active: NOW - 90, tool_count: 14, context: { source: 'none' } }, over || {});
}
function session(over) {
  return Object.assign({ id: 'aaaaaaaa-1111', short: 'aaaaaaaa', live: true, state: 'waiting', account_key: 'k', tools: [], agents: [], messages: [] }, over || {});
}

console.log('parked agent box');
const box = r.fleetAgent(agent(), NOW);
assert(box.includes('fl-ag-parked'), 'has its own style class');
assert(box.includes('waiting on its own background work'), 'says what it is waiting on');
assert(box.indexOf('fl-ag-done') < 0 && box.indexOf('fl-ag-running') < 0, 'not styled as done or running');
assert(box.indexOf('✓ ') < 0 && box.indexOf('◐') < 0, 'no done tick or running spinner');
assert(box.includes('data-t="' + (NOW - 90) + '"'), 'age counts from its last activity');
assert(r.fleetAgent(agent({ status: 'done', ended_at: NOW - 5 }), NOW).indexOf('parked') < 0, 'a done agent is unaffected');
assert(r.fleetAgent(agent({ status: 'running' }), NOW).indexOf('waiting on its own') < 0, 'a running agent is unaffected');

console.log('parked agent on the card');
const card = r.fleetSession(session({ agents: [agent()] }), NOW);
assert(card.includes('fl-ag-parked') && card.includes('waiting on its own background work'), 'stays in the active agent list');

console.log('page assets');
const css = fleetPageHTML('n0nce');
assert(css.includes('.fl-ag-parked'), 'stylesheet defines the parked style');
assert(!/\sonclick=|\sonerror=/.test(box), 'no inline handlers');
let compiles = true;
try { new Function(fleetClientScript()); } catch (e) { compiles = false; }
assert(compiles, 'client script still compiles');

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed ? 1 : 0);
