// Tests for the worktrees-in-flight strip.
const w = require('./fleet-render-worktrees');
const r = require('./fleet-render');
const { fleetClientScript } = require('./page');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

const NOW = 1792576900;
function wt(over) {
  return Object.assign({
    repo: 'org/repo', path_rel: '.claude/worktrees/fix/3167-origin', branch: 'fix/3167-origin', issue: 3167,
    writers: [{ session: 'sess-1', slot: '3', agent_id: 'ag-9', agent_desc: 'Implement the fix', last_at: NOW - 120 }],
    active: true, last_git_at: NOW - 300, stale: false, merged: null, two_writers: false,
  }, over || {});
}

console.log('active chips');
const chip = w.fleetWtChip(wt(), NOW);
assert(chip.includes('href="#fl-sess-1"') && chip.includes('data-agent="ag-9"'), 'anchors to the writer card and agent box');
assert(chip.includes('repo') && chip.includes('fix/3167-origin') && chip.includes('#3167'), 'shows repo, branch and issue');
assert(chip.includes('slot 3') && chip.includes('› Implement the fix'), 'shows slot and agent description');
assert(chip.includes('<span class="fl-ago" data-t="' + (NOW - 120) + '" data-sfx=" ago">2m ago</span>'), 'age is a ticking span');
assert(!chip.includes('two writers'), 'no warning for one writer');
assert(w.fleetWtChip(wt({ writers: [{ session: 's', slot: '1', last_at: NOW - 5 }] }), NOW).indexOf('data-agent') < 0, 'no agent anchor without agent_id');
assert(w.fleetWtChip(wt({ writers: [] }), NOW).startsWith('<span'), 'no writer renders an unlinked chip');

console.log('two writers');
const two = w.fleetWtChip(wt({ two_writers: true, writers: [
  { session: 'old', slot: '1', last_at: NOW - 400 }, { session: 'new', slot: '2', agent_desc: 'Review', last_at: NOW - 10 }] }), NOW);
assert(two.includes('⚠ two writers') && two.includes('fl-warn') && two.includes('fl-shared'), 'amber two-writers mark');
assert(two.includes('href="#fl-new"') && two.includes('slot 2 › Review, slot 1'), 'newest writer leads and the title lists both');

console.log('collapsed tail');
const board = [wt(),
  wt({ path_rel: '.claude/worktrees/feat/1-a', branch: 'feat/1-a', active: false, stale: true, last_git_at: NOW - 90000, writers: [] }),
  wt({ path_rel: '.claude/worktrees/feat/2-b', branch: 'feat/2-b', active: false, merged: true, last_git_at: NOW - 7200, writers: [] }),
  wt({ path_rel: '.claude/worktrees/feat/3-c', branch: 'feat/3-c', active: false, merged: null, last_git_at: NOW - 600, writers: [] })];
const html = w.fleetWorktrees(board, NOW);
assert(html.includes('worktrees in flight') && !html.includes('branches in flight'), 'strip is labelled worktrees in flight');
assert(html.includes('<details class="fl-wt-tail" data-key="worktrees"><summary>3 idle · 1 stale · 1 merged</summary>'), 'collapsed summary counts');
assert(!/<details[^>]* open/.test(html), 'collapsed by default');
assert(html.includes('◌ stale') && html.includes('✓ merged'), 'stale and merged marks');
assert(html.includes('git worktree remove .claude/worktrees/feat/1-a') && html.includes('worktree remove .claude/worktrees/feat/2-b'), 'removal hints for stale and merged');
assert(!html.includes('worktrees/feat/3-c</code>'), 'no removal hint for an unknown idle worktree');
assert(html.includes('<code class="fl-wt-cmd" data-copy="git worktree remove'), 'hint is a copyable code element');
const odd = w.fleetWtRow(wt({ path_rel: '.claude/worktrees/other-name', branch: 'feat/9-z', active: false, stale: true, writers: [] }), NOW);
assert(odd.includes('worktrees/other-name') && !odd.includes('worktrees/feat/9-z'), 'hint uses the path name, not the branch');
for (const bad of ['x; rm -rf ~', 'a b', 'x$(id)', '../../etc', 'a/../b', 'x`id`', "x'y"]) {
  assert(!w.fleetWtRow(wt({ path_rel: '.claude/worktrees/' + bad, active: false, stale: true, writers: [] }), NOW).includes('<code'), 'no hint for the name ' + JSON.stringify(bad));
}
const outside = w.fleetWtRow(wt({ path_rel: '/srv/elsewhere/x', active: false, stale: true, writers: [] }), NOW);
assert(!outside.includes('<code') && outside.includes('◌ stale'), 'no hint when path_rel is outside .claude/worktrees');
const row = w.fleetWtRow(wt({ active: false, merged: null, stale: false }), NOW);
assert(!row.includes('merged') && !row.includes('stale') && !row.includes('<code'), 'merged null and not stale shows no mark');
assert(w.fleetWtRow(wt({ active: false, merged: false, stale: false }), NOW).indexOf('✓') < 0, 'merged false shows no mark');
assert(w.fleetWorktrees([wt()], NOW).indexOf('<details') < 0, 'no tail without idle worktrees');
assert(w.fleetWorktrees([wt({ active: false, writers: [] })], NOW).includes('none being written to'), 'placeholder when none active');
assert(w.fleetWorktrees([], NOW) === '', 'empty list renders nothing');

console.log('escaping');
const H = '<img src=x onerror=alert(1)>"\'';
const evil = w.fleetWorktrees([
  wt({ repo: H, path_rel: H, branch: H, issue: H, writers: [{ session: H, slot: H, agent_id: H, agent_desc: H, last_at: NOW }] }),
  wt({ repo: H, path_rel: '.claude/worktrees/' + H, branch: H, active: false, stale: true, writers: [] })], NOW);
assert(!evil.includes('<img') && evil.includes('&lt;img') && evil.includes('&quot;'), 'markup and quotes are escaped');
assert(!evil.includes('data-copy="git worktree remove'), 'a hostile path gets no copyable hint');
assert(!/(href|data-agent|data-copy|title)="[^"]*<[^"]*"/.test(evil), 'attribute values hold no raw angle brackets');

console.log('fallback');
const old = [{ repo: 'org/app', branch: 'feature/42-x', sessions: ['a'], shared: false }];
assert(w.fleetInFlight({ branches: old }, NOW) === r.fleetBranches(old), 'falls back to the branch strip without worktrees');
assert(w.fleetInFlight({ branches: old, worktrees: [wt()] }, NOW).includes('worktrees in flight'), 'worktrees win when present');
assert(w.fleetInFlight({ worktrees: [] }, NOW) === '', 'empty worktrees array hides the strip');
assert(w.fleetInFlight(null, NOW) === '', 'no board renders nothing');

console.log('agent box id');
const box = r.fleetSession({ id: 's', short: 's', live: true, state: 'working', account_key: 'k', tools: [], messages: [],
  agents: [{ id: 'ag-9"x', type: 'Explore', status: 'running', started_at: NOW - 60, last_active: NOW - 5, context: { source: 'none' } }] }, NOW);
assert(box.includes('id="fl-ag-ag-9&quot;x"'), 'agent box carries an escaped id');

console.log('page');
let compiles = true;
const script = fleetClientScript();
try { new Function(script); } catch (e) { compiles = false; console.error(e); }
assert(compiles && ['fleetWorktrees', 'fleetWtChip', 'fleetWtRow', 'fleetWtHint', 'fleetInFlight'].every((n) => script.includes('function ' + n + '(')), 'helpers injected into the page');
assert(script.includes('fleetInFlight(b,now)') && script.includes('data-copy') && script.includes("'fl-ag-'+t.dataset.agent"), 'render, copy and agent highlight are wired');
assert(!/onclick=/i.test(w.fleetWorktrees(board, NOW)), 'no inline handlers');

console.log(`\n${passed} passed, ${failed} failed`);
if (failed) process.exit(1);
