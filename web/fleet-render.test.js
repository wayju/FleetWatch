// Tests for the fleet page's render helpers and page shell.
const r = require('./fleet-render');
const { fleetPageHTML, fleetClientScript } = require('./page');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

const NOW = 1792576900;

function session(over) {
  return Object.assign({
    id: 'aaaaaaaa-1111', short: 'aaaaaaaa', live: true, link: 'projection', pid: 100, slot: 'work', pane: '%17',
    account: { email: 'a@work.example' }, account_key: 'a@work.example',
    state: 'waiting', state_source: 'hook', state_since: NOW - 720,
    repo: { owner: 'org', name: 'app', branch: 'feature/42-fleet', commit: '012345678', version: '3.4.1', version_source: 'VERSION' },
    branch: 'feature/42-fleet', claude_version: '2.1.306', model: 'claude-sonnet-5-5',
    context: { tokens: 50000, limit: 200000, pct: 25, source: 'transcript' },
    issues: [{ repo: 'org/app', number: 42, source: 'branch', title: 'Fleet view', state: 'open', url: 'https://git.example.com/org/app/issues/42' }],
    recap: { first_prompt: 'work on #42', last_prompt: 'ship it', last_reply: 'Done.' },
    tools: [{ name: 'Bash', target: 'npm test', at: NOW - 30, status: 'ok' }],
    agents: [{ id: 'sub1', type: 'Explore', model: 'haiku', status: 'running', started_at: NOW - 60, last_active: NOW - 5, tool_count: 3, current: { name: 'Grep', target: 'TODO', status: 'running', at: NOW - 5 }, context: { source: 'none' } }],
    messages: [{ from: 'aaaaaaaa-1111', to: 'bbbbbbbb-2222', to_label: 'bbbbbbbb', at: NOW - 100, preview: 'rebase please', external: false }],
    started_at: NOW - 3600, last_activity: NOW - 5,
  }, over || {});
}

console.log('escaping');
assert(r.fleetEsc('<img src=x onerror="a">') === '&lt;img src=x onerror=&quot;a&quot;&gt;', 'escapes markup and quotes');
assert(r.fleetUrl('javascript:alert(1)') === '', 'drops non-http hrefs');
assert(r.fleetUrl('https://git.example.com/x?a=1&b=2') === 'https://git.example.com/x?a=1&amp;b=2', 'keeps and escapes http(s) hrefs');
const evil = r.fleetSession(session({ title: '<script>x</script>', branch: 'x"><svg onload=1>', recap: { last_prompt: '<i>hi</i>' } }), NOW);
assert(!evil.includes('<script>') && !evil.includes('<i>hi') && !evil.includes('<svg'), 'session card escapes title, branch and recap');
assert(evil.includes('x&quot;&gt;&lt;svg onload=1&gt;'), 'hostile branch name renders as text');

console.log('ages');
assert(r.fleetAge(3) === 'now' && r.fleetAge(42) === '42s' && r.fleetAge(720) === '12m' && r.fleetAge(7200) === '2h' && r.fleetAge(172800) === '2d', 'age buckets');
assert(r.fleetAge(-1) === '' && r.fleetAge(null) === '', 'no age for bad input');
assert(r.fleetAgo(NOW - 720, NOW).includes('data-t="' + (NOW - 720) + '"'), 'ago carries its timestamp for the ticker');

console.log('state');
assert(r.fleetState('approval').cls === 'fl-st-approval' && r.fleetState('question').cls === 'fl-st-approval', 'approval and question both read as blocked');
assert(r.fleetState('waiting').label === 'WAITING ON YOU', 'waiting names whose move it is');
assert(r.fleetState('weird').label === 'WEIRD', 'unknown states still render');

console.log('branches and versions');
assert(r.fleetBranchHue('main') === -1 && r.fleetBranchHue('master') === -1, 'trunk branches are neutral');
assert(r.fleetBranchHue('feature/42-x') === r.fleetBranchHue('feature/42-x') && r.fleetBranchHue('feature/42-x') >= 0, 'branch hue is stable');
const card = r.fleetSession(session(), NOW);
assert(card.includes('⎇ feature/42-fleet') && card.includes('v3.4.1') && card.includes('@012345678'), 'card shows branch, project version and commit');
assert(card.includes('cc 2.1.306') && card.includes('sonnet-5-5'), 'card shows Claude Code version and model');
const strip = r.fleetBranches([{ repo: 'org/app', branch: 'feature/42-fleet', version: 'v3.4.1', sessions: ['a', 'b'], shared: true }]);
assert(strip.includes('⚠ shared') && strip.includes('×2') && strip.includes('v3.4.1') && !strip.includes('vv'), 'strip flags a shared branch and normalises the v prefix');

console.log('session card');
assert(card.includes('WAITING ON YOU') && card.includes('data-t="' + (NOW - 720) + '"'), 'pill shows state and how long');
assert(card.includes('href="https://git.example.com/org/app/issues/42"') && card.includes('Fleet view'), 'issue chip links with its title');
assert(card.includes('ctx 25%') && card.includes('fl-ctx-ok'), 'context bar');
assert(card.includes('delegate to subagents') && card.includes('Explore') && card.includes('Grep'), 'subagents fan out with their current tool');
assert(card.includes('href="#fl-bbbbbbbb-2222"') && card.includes('rebase please'), 'message links to the target card');
assert(r.fleetCtxBar({ pct: 91, tokens: 1, limit: 2 }).includes('fl-ctx-hot'), 'high context is hot');
assert(r.fleetSession(session({ link: 'guess' }), NOW).includes('guessed link'), 'a guessed link is labelled');
assert(r.fleetSession(session({ recap: undefined }), NOW).indexOf('fl-recap') < 0, 'no recap block when text is off');
const summed = r.fleetSession(session({ recap: { summary: 'Rotating the cert; <b>blocked</b> on approval.', summary_at: NOW - 240 } }), NOW);
assert(summed.includes('fl-summary') && summed.includes('&lt;b&gt;blocked') && summed.includes('data-t="' + (NOW - 240) + '"'), 'out-of-session summary shows, escaped, with its age');

console.log('filters and grouping');
const board = {
  host: 'host1',
  accounts: [{ key: 'a@work.example', email: 'a@work.example', slots: ['work'], live: 1 }, { key: 'b@home', email: 'b@home', slots: ['home'], live: 1 }],
  sessions: [session(), session({ id: 'w2', short: 'w2', state: 'working', account_key: 'b@home', agents: [] }), session({ id: 'e3', short: 'e3', live: false, state: 'ended' })],
};
assert(r.fleetVisible(board.sessions[0], 'attention', true) && !r.fleetVisible(board.sessions[1], 'attention', true), 'needs-me keeps waiting, drops working');
assert(r.fleetVisible(board.sessions[1], 'working', true) && r.fleetVisible(board.sessions[0], 'working', true), 'working keeps sessions with running subagents');
assert(!r.fleetVisible(board.sessions[2], 'all', false), 'ended hidden when show-ended is off');
const grouped = r.fleetAccounts(board, NOW, 'all', true);
assert(grouped.indexOf('a@work.example') < grouped.indexOf('b@home') && (grouped.match(/class="fl-card /g) || []).length === 3, 'sessions grouped under their account');
assert(r.fleetAccounts({ host: 'host1', accounts: [], sessions: [] }, NOW, 'all', true).includes('No Claude sessions on host1'), 'empty board says so');

console.log('log');
const log = r.fleetLog([{ at: NOW - 3, session: 's1', short: 's1', agent: 'sub1', kind: 'tool', text: 'Grep · TODO' }, { at: NOW, session: 's1', short: 's1', kind: 'prompt', text: 'go' }], NOW);
assert(log.indexOf('go') < log.indexOf('Grep'), 'newest first');
assert(log.includes('↳ sub1') && log.includes('fl-lk-prompt'), 'agent rows marked, kinds classed');

console.log('hostile payloads');
const H = '<img src=x onerror=1>';
const Q = '"onmouseover=alert(1)';
// A payload is contained when no raw <img survives, no tag carries an event
// handler once quoted attribute values are removed, and no quote broke out.
function contained(html, label) {
  const stripped = html.replace(/="[^"]*"/g, '="_"');
  const tags = stripped.match(/<[^>]*>/g) || [];
  const handler = tags.some((t) => /\son[a-z]+=/i.test(t));
  assert(!/<img/i.test(html) && !handler && !/"onmouseover/i.test(html), label);
}
const hostile = session({
  id: H, short: H, pane: H, pid: H, slot: H, model: H, claude_version: H, title: H, state: H, link: 'guess',
  branch: H + Q,
  repo: { owner: H, name: Q, root: Q, version: H, version_source: Q, commit: H, worktree: true },
  context: { pct: H, tokens: H, limit: H, source: 'transcript' },
  issues: [{ repo: H, number: H, title: H, state: H, url: 'https://x/' + Q, source: Q, is_pr: true }],
  recap: { summary: H, summary_at: H, first_prompt: H, last_prompt: H, last_reply: H },
  tools: [{ name: H, target: Q + H, at: H, status: H }],
  messages: [{ to: H + Q, to_label: H, preview: H + Q, at: H, external: false }, { to: H, to_label: Q + H, preview: H, at: H, external: true }],
  agents: [{ id: H, type: H, model: H, description: H, status: H, tool_count: H, last_active: H, background: true, current: { name: H, target: H + Q, status: H }, context: { pct: H, source: 'transcript' } }],
});
const hcard = r.fleetSession(hostile, NOW);
contained(hcard, 'session hcard contains every field in a hostile session');
assert(hcard.includes('&lt;img src=x onerror=1&gt;'), 'hostile text renders as text');
assert(!hcard.includes('<script'), 'no script element');
contained(r.fleetIssues(hostile.issues), 'issue url/title/state/repo/source contained');
assert(r.fleetIssues(hostile.issues).includes('href="https://x/&quot;onmouseover=alert(1)"'), 'fleetUrl quote breakout is escaped inside href');
contained(r.fleetAgent(hostile.agents[0], NOW), 'agent type/description/current.target/tool_count contained');
contained(r.fleetCtxBar({ pct: H, tokens: H, limit: H, source: 'transcript' }), 'ctx bar with string fields contained');
assert(/width:0\.0%/.test(r.fleetCtxBar({ pct: NaN, source: 'transcript' })) && !/NaN/.test(r.fleetCtxBar({ pct: NaN, tokens: NaN, limit: NaN, source: 'transcript' })), 'ctx bar NaN renders as 0, never NaN');
contained(r.fleetCounts({ live: H, approval: H, waiting: H, working: H, agents_running: H }), 'count chips with string values contained');
assert(!/NaN/.test(r.fleetCounts({ live: H })) && r.fleetCounts({ live: H }).includes('<b>0</b>'), 'non-numeric counts become 0');
contained(r.fleetBranches([{ repo: H, branch: H + Q, version: H, commit: H, sessions: H, shared: true }]), 'branches strip contained');
const hboard = { host: H, sessions: [hostile], accounts: [{ key: 'a@work.example', email: H, org: H, slots: [H, Q], live: H }] };
contained(r.fleetAccounts(hboard, NOW, 'all', true), 'account email/org/slots/live contained');
contained(r.fleetAccounts({ host: H, sessions: [] }, NOW, 'all', true), 'empty-hboard host contained');
contained(r.fleetLog([{ at: H, session: H + Q, short: H, agent: H, kind: H + Q, text: H }], NOW), 'log session/short/agent/kind/text contained');
contained(r.fleetAgo(H, NOW), 'ago with hostile timestamp contained');
contained(r.fleetState(H).label, 'unknown state label contained');

console.log('stable order');
const ordered = r.fleetSortSessions([
  session({ id: 'c', started_at: 300 }), session({ id: 'a', started_at: 100 }), session({ id: 'z' }), session({ id: 'b', started_at: 200 }), session({ id: 'y' }),
]).map((x) => x.id).join(',');
assert(ordered === 'a,b,c,y,z', 'sessions sort by start time, unstarted last by id');
assert(r.fleetSortAccounts([{ key: 'b' }, { key: 'a' }]).map((x) => x.key).join('') === 'ab', 'accounts sort by key');
const flip = { host: 'h', accounts: [{ key: 'b@x', email: 'b@x' }, { key: 'a@x', email: 'a@x' }], sessions: [
  session({ id: 'new', short: 'newsess', started_at: 200, account_key: 'a@x', state: 'working' }),
  session({ id: 'old', short: 'oldsess', started_at: 100, account_key: 'a@x', state: 'idle' }),
  session({ id: 'oth', short: 'othsess', started_at: 150, account_key: 'b@x' }),
] };
const flipped = r.fleetAccounts(flip, NOW, 'all', true);
assert(flipped.indexOf('id="fl-old"') < flipped.indexOf('id="fl-new"') && flipped.indexOf('a@x') < flipped.indexOf('b@x'), 'server state order and account order are ignored');
const before = JSON.stringify(flip.sessions);
r.fleetAccounts(flip, NOW, 'all', true);
assert(JSON.stringify(flip.sessions) === before, 'sorting does not mutate the board');

console.log('headline');
const head = (over) => r.fleetHeadline(session(over));
assert(head({ title: 'autodev', recap: { first_prompt: 'x' } }) === 'autodev', 'title wins');
assert(head({ title: '', recap: { first_prompt: 'fix the thing' } }) === 'fix the thing', 'first prompt when no title');
assert(head({ title: '', recap: { first_prompt: '<pasted_content id=1>big\nblob</pasted_content>  review   this ' } }) === 'review this', 'closed pasted_content stripped, whitespace collapsed');
assert(head({ title: '', recap: { first_prompt: 'start <pasted_content id=1>never closed\nmore' } }) === 'start', 'unclosed pasted_content stripped');
assert(head({ title: '', recap: { first_prompt: '/review-pr 42 please' } }) === '42 please', 'leading slash command stripped');
assert(head({ title: '', recap: { first_prompt: 'a'.repeat(300) } }).length === 120, 'long prompt clipped to 120');
assert(head({ title: '', recap: { first_prompt: '<pasted_content>only</pasted_content>' } }) === 'app · feature/42-fleet', 'nothing left falls back to repo and branch');
assert(head({ title: '', recap: undefined, repo: {}, branch: '' , short: 'abcd1234' }) === 'abcd1234', 'short id is the last resort');
const hcardTitle = r.fleetSession(session({ title: '<b>x</b>' }), NOW);
assert(hcardTitle.includes('class="fl-headline">&lt;b&gt;x&lt;/b&gt;'), 'headline is escaped');
assert(hcardTitle.includes('class="fl-foot">aaaaaaaa · slot work · %17 · pid 100 · sonnet-5-5 · cc 2.1.306'), 'id, slot, pane, pid, model and version sit in the footer');

console.log('description');
const sum = r.fleetSession(session({ state: 'working', recap: { summary: 'Doing X', last_reply: 'Reply Y' } }), NOW);
assert(sum.includes('>recap</span>Doing X') && !sum.includes('Reply Y'), 'summary labelled recap, hides last reply while working');
const rep = r.fleetSession(session({ state: 'working', recap: { last_reply: 'Reply Y' } }), NOW);
assert(rep.includes('>last reply</span>Reply Y') && !rep.includes('fl-desc-attn'), 'last reply is the fallback');
assert(r.fleetSession(session({ recap: { last_reply: 'Need a call' } }), NOW).includes('fl-desc fl-desc-attn'), 'waiting session shows the reply prominently');
assert(r.fleetSession(session({ state: 'waiting', recap: { summary: 'S', last_reply: 'R' } }), NOW).includes('>last reply</span>R'), 'waiting session keeps the reply beside a summary');
assert(r.fleetDescription(session({ recap: { first_prompt: 'p' } }), NOW) === '', 'no description when there is nothing to say');

console.log('history');
const busy = session({
  tools: [1, 2, 3, 4, 5, 6].map((i) => ({ name: 'T' + i, at: NOW - i, status: 'ok' })).concat([{ name: 'Running1', at: NOW, status: 'running' }]),
  agents: [{ id: 'd1', type: 'DoneAgent', status: 'done' }, { id: 'd2', type: 'DoneAgent2', status: 'done' }, { id: 'e1', type: 'ErrAgent', status: 'error' }, { id: 'r1', type: 'RunAgent', status: 'running' }, { id: 's1', type: 'StallAgent', status: 'stalled' }],
  messages: [{ to: 'bbbbbbbb-2222', to_label: 'bbbbbbbb', at: NOW, preview: 'hi' }],
});
const bc = r.fleetSession(busy, NOW);
const hist = bc.slice(bc.indexOf('<details'), bc.indexOf('</details>'));
assert((bc.match(/<details/g) || []).length === 1 && /<details class="fl-history" data-key="aaaaaaaa-1111:history">/.test(bc), 'one closed history details keyed by session');
assert(hist.includes('<summary>history · 5 tools · 2 agents done · 1 failed · 1 message</summary>'), 'summary counts finished work');
assert(!hist.includes('Running1') && !hist.includes('RunAgent') && !hist.includes('StallAgent') && hist.includes('DoneAgent') && hist.includes('ErrAgent'), 'history holds only finished work');
assert(!hist.includes('T1<') && hist.includes('T6'), 'history keeps the last five finished tools');
const live = bc.replace(hist, '');
assert(live.includes('Running1') && live.includes('RunAgent') && live.includes('StallAgent') && !live.includes('DoneAgent'), 'running tool and running or stalled subagents stay visible');
assert(live.includes('issues') && live.includes('ctx 25%') && live.includes('fl-repo-line'), 'issues, ctx and repo line stay visible');
assert(r.fleetSession(session({ tools: [], agents: [], messages: [] }), NOW).indexOf('<details') < 0, 'no history element when there is nothing finished');
assert(!/<details[^>]* open/.test(bc), 'history starts closed');
assert(/<details class="fl-log"/.test(fleetPageHTML()) && !/<details class="fl-log"[^>]* open/.test(fleetPageHTML()), 'session log is a closed details');
assert(r.fleetHistory(session({ id: '"><x' , tools: [{ name: 'a', status: 'ok' }] }), NOW).includes('data-key="&quot;&gt;&lt;x:history"'), 'history key is escaped');

console.log('agent totals in history');
const totalsHist = r.fleetHistory(session({ agent_totals: { total: 3, running: 1, last_finished_at: NOW - 120 } }), NOW);
assert(totalsHist.includes('3 run in total') && /last finished <span class="fl-ago" data-t="[^"]+" data-sfx=" ago">2m ago<\/span>/.test(totalsHist), 'totals give a count and a ticking last-finished age');
const oneTotal = r.fleetHistory(session({ tools: [], agents: [], messages: [], agent_totals: { total: 1, running: 0, last_finished_at: 0 } }), NOW);
assert(oneTotal.includes('1 run in total<') && !oneTotal.includes('last finished'), 'singular, and no age when nothing finished');
assert(!r.fleetHistory(session({ agent_totals: { total: 2, running: 0, last_finished_at: NOW - 5 }, agents: [{ id: 'a', status: 'done' }] }), NOW).includes('agents done'), 'done count dropped when totals cover it');
assert(r.fleetHistory(session({ agent_totals: { total: 2, running: 0, last_finished_at: 1 }, agents: [{ id: 'a', status: 'error' }] }), NOW).includes('1 failed'), 'failed count kept');
const withBoxes = r.fleetHistory(session({ agent_totals: { total: 56, running: 0, last_finished_at: NOW - 60 }, agents: [{ id: 'a', status: 'done' }, { id: 'b', status: 'error' }, { id: 'c', status: 'running' }] }), NOW);
assert(withBoxes.includes('56 run in total · 2 finished in the last 15m') && !withBoxes.includes('last finished') && withBoxes.includes('1 failed'), 'finished count replaces the recency hint');
assert(!totalsHist.includes('finished in the last 15m'), 'no finished count when none are held');
const noTotals = r.fleetHistory(session({ tools: [{ name: 'a', status: 'ok' }] }), NOW);
assert(noTotals.includes('1 tool') && !noTotals.includes('subagent'), 'absent totals leave the label as before');
assert(r.fleetHistory(session({ tools: [], agents: [], messages: [], agent_totals: { total: 0, running: 0, last_finished_at: 0 } }), NOW) === '', 'zero totals and no activity renders no history');
assert(r.fleetHistory(session({ tools: [], agents: [], messages: [], agent_totals: { total: 2, running: 0, last_finished_at: 0 } }), NOW).includes('<details'), 'history renders on totals alone');
assert(r.fleetHistory(session({ agent_totals: { total: 2, last_finished_at: '"><img src=1>' } }), NOW).indexOf('<img') === -1, 'last_finished_at is escaped');
assert(r.fleetHistory(session({ tools: [{ name: 'a', status: 'ok' }], agent_totals: { total: 2 } }), NOW).includes('history · 1 tool · 2 run in total'), 'label parts stay joined');

console.log('branch strip links');
const links = r.fleetBranches([{ repo: 'org/app', branch: 'feature/x', sessions: ['s-1', 's-2'], shared: true }, { repo: 'o/lone', branch: 'main', sessions: [] }]);
assert(links.includes('<a class="fl-strip-item fl-shared" href="#fl-s-1" data-sessions="s-1 s-2"'), 'strip item is an anchor to the first session listing all');
assert(links.includes('<span class="fl-strip-item"'), 'a branch with no session stays plain');
const evilLink = r.fleetBranches([{ repo: 'r', branch: 'b', sessions: ['x"><img src=1 onerror=1>', 'y'] }]);
assert(!evilLink.includes('<img') && evilLink.includes('href="#fl-x&quot;&gt;&lt;img'), 'session ids are escaped in href and data-sessions');
const script = fleetClientScript();
assert(script.includes("addEventListener('toggle'") && script.includes('FL.open') && script.includes('checkVisibility()'), 'client script tracks open details and guards hidden link sources');

console.log('inline tables');
assert(r.fleetProse('Waiting for the 5 | Item | State | |---|---| | #1504 S5 | Closed |') === 'Waiting for the 5…', 'table cut, prose kept with an ellipsis');
assert(r.fleetProse('Plain reply, a | b is fine.') === 'Plain reply, a | b is fine.', 'text without a table is untouched');
assert(r.fleetProse('| Item | State | |---|---| | a | b |') === '| Item | State | |---|---| | a | b |', 'table-only text is kept');
assert(r.fleetSession(session({ recap: { last_reply: 'Done. | A | B | |:---|---:| | 1 | 2 |' } }), NOW).includes('Done.…') && !r.fleetSession(session({ recap: { summary: 'S | A | |---| | 1 |' } }), NOW).includes('|---|'), 'descriptions drop inline tables');

console.log('page');
const html = fleetPageHTML();
const nonced = fleetPageHTML('N0NCE');
assert(nonced.includes('<script nonce="N0NCE">') && nonced.includes('<style nonce="N0NCE">'), 'page carries the nonce on its inline script and style');
assert(/script-src 'nonce-N0NCE'/.test(require('./page').fleetCsp('N0NCE')) && !/unsafe-inline'; /.test(require('./page').fleetCsp('N0NCE').split('style-src-attr')[0]), 'CSP pins scripts to the nonce');
assert(html.includes('<title>FleetWatch</title>') && html.includes("new EventSource(FL_URLS.stream)") && html.includes('"/v1/events"'), 'page streams from the FleetWatch route');
let compiles = true;
try { new Function(fleetClientScript()); } catch (e) { compiles = false; console.error(e); }
assert(compiles, 'injected client script parses');

console.log(`\n${passed} passed, ${failed} failed`);
if (failed) process.exit(1);
