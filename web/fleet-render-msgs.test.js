// Tests for the per-card messages list, the model_class colour and the library entry.
const m = require('./fleet-render-msgs');
const c = require('./fleet-render-cost');
const lib = require('./index');
const { fleetClientScript, fleetPageHTML } = require('./page');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

const NOW = 1792576900;
const board = {
  sessions: [{ id: 'S1', slot: '2', short: 's1' }, { id: 'S2', slot: '', short: 'bbbb' }],
  links: [
    { from: 'S1', to: 'S2', at: NOW - 60, preview: 'rebase <please>' },
    { from: 'S2', to: 'S1', at: NOW - 600, preview: 'done' },
    { from: 'S1', to: 'S3', at: NOW - 5000, preview: 'too old' },
    { from: 'S1', to: 'X', at: NOW - 30, external: true, to_label: 'telegram', preview: 'ping' },
  ],
};
console.log('messages list');
const out = m.fleetMsgList(board.sessions[0], board, NOW);
assert(out.includes('→ <a href="#fl-S2">bbbb</a>') && out.includes('← <a href="#fl-S2">bbbb</a>'), 'sent and received rows name the other side');
assert(out.includes('rebase &lt;please&gt;'), 'preview is escaped');
assert(!out.includes('too old'), 'links older than 30 minutes are dropped');
assert(out.includes('→ telegram') && !out.includes('#fl-X'), 'external recipients are plain text');
assert(out.indexOf('ping') < out.indexOf('rebase'), 'newest first');
assert(m.fleetMsgList({ id: 'S9' }, board, NOW) === '' && m.fleetMsgList(board.sessions[0], null, NOW) === '', 'nothing to show gives no markup');
assert(m.fleetMsgName(board, 'S1') === 'slot 2' && m.fleetMsgName(board, 'S2') === 'bbbb' && m.fleetMsgName(board, 'zz') === 'zz', 'names prefer slot, then short id');

const evil = { sessions: [{ id: 'S1', slot: '1' }, { id: 'a"><img src=x onerror=1>', short: '<b>x</b>' }],
  links: [{ from: 'S1', to: 'a"><img src=x onerror=1>', at: NOW - 5, preview: '<script>alert(1)</script>' },
    { from: 'S1', to: 'E', at: NOW - 4, external: true, to_label: '<i>lbl</i>', preview: '"onmouseover="x' }] };
const evilOut = m.fleetMsgList(evil.sessions[0], evil, NOW);
assert(!/<(script|img|b|i)[ >]/.test(evilOut)&& !evilOut.includes('onerror=1>'), 'ids, names, labels and previews are escaped');

console.log('model_class');
assert(c.fleetModelClass({ model: 'm-a', model_class: 'small' }) === 'fl-mc-small', 'model_class names the class');
assert(c.fleetModelClass({ model: 'm-a' }) === 'fl-mc-other', 'no model_class gives the neutral class');
assert(c.fleetModelClass({ model: 'x', model_class: 'bogus"><' }) === 'fl-mc-other', 'a model_class that is not a plain token is never emitted as a class');
assert(c.fleetModelClass('m-a') === 'fl-mc-other', 'bare model string gives the neutral class');
assert(c.fleetCostBar([{ model: 'm-a', model_class: 'medium', usd: 1 }]).includes('fl-mc-medium'), 'cost bar uses model_class');

console.log('library entry');
const html = lib.fleetBoardHTML({ accounts: [{ key: 'a', live: 1, slots: ['1'] }], sessions: [{ id: 'S1', account_key: 'a', live: true, state: 'working', short: 's1' }], links: [], branches: [] }, NOW);
assert(html.includes('id="fl-S1"'), 'fleetBoardHTML renders cards from a snapshot');
assert(fleetPageHTML('N', { streamUrl: '/x/stream', snapshotUrl: '/x' }).includes('"/x/stream"'), 'page takes the stream URL');
assert(fleetClientScript().includes('"/v1/events"') && fleetClientScript().includes('"/v1/fleet"') && fleetClientScript().includes('"/v1/session"'), 'default URLs are the FleetWatch routes');
assert(/@media \(max-width:700px\)/.test(lib.fleetStyles()) && /\.fl-xmsgs\{display:none\}/.test(lib.fleetStyles()), 'narrow media query and hidden-by-default messages list');
const fs = require('fs'), path = require('path');
const srcFiles = fs.readdirSync(__dirname).filter((f) => f.endsWith('.js') && !f.endsWith('.test.js') && !['assets.js', 'fleet-chrome.js', 'build.js'].includes(f));
const coupled = srcFiles.filter((f) => /require\((['"])(?!\.\/)/.test(fs.readFileSync(path.join(__dirname, f), 'utf8').replace(/\/\/.*$/gm, '')));
assert(coupled.length === 0, 'no module in the directory requires anything outside it' + (coupled.length ? ': ' + coupled.join(', ') : ''));

// Arrow pill text
{
  const bd = { sessions: [{ id: 'S1', slot: '1' }, { id: 'S2', slot: '2' }] };
  const lab = m.fleetLinkLabel(bd, { from: 'S1', to: 'S2', at: 1000 - 130, preview: 'please review\nthe narrow layout of the whole fleet page' }, 1000);
  assert(lab.text === 'slot 1 \u2192 slot 2 \u00b7 \u201cplease review the narrow layout of\u2026\u201d \u00b7 2m', 'pill reads from \u2192 to \u00b7 clipped preview \u00b7 age on one line');
  assert(lab.full.includes('the whole fleet page') && !lab.full.includes('\n'), 'tooltip keeps the whole preview on one line');
  assert(m.fleetLinkLabel(bd, { from: 'S1', to: 'S2', at: 999, preview: '' }, 1000).text === 'slot 1 \u2192 slot 2 \u00b7 now', 'no preview, no quotes');
  assert(m.fleetLinkLabel(bd, { from: 'X<b>', to: 'S2', at: 999, preview: '<i>' }, 1000).text.includes('<b>'), 'label is raw text; the page escapes it');
}

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed ? 1 : 0);
