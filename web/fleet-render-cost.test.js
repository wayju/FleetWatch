// Tests for the fleet API-equivalent cost helpers.
const c = require('./fleet-render-cost');
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
const tk = (n) => ({ input: n, cache_write_5m: 0, cache_write_1h: 0, cache_read: n * 10, output: n });
const cost = {
  usd: 1234.5, partial: false, priced_as_of: '2026-10-22', tokens: tk(1),
  by_model: [
    { model: 'claude-opus-5-5', model_class: 'large', usd: 900, messages: 40596, tokens: { input: 1000, cache_write_5m: 1e6, cache_write_1h: 5e5, cache_read: 11.9e9, output: 920000 } },
    { model: 'claude-sonnet-5-5', model_class: 'small', usd: 300, messages: 12, tokens: tk(5) },
    { model: '<synthetic>', usd: 0, messages: 3, unpriced: true, tokens: tk(1) },
  ],
};

console.log('formatting');
assert(c.fleetUsd(0) === '$0.00' && c.fleetUsd(9.994) === '$9.99' && c.fleetUsd(9.996) === '$10.00', 'under $10 keeps two decimals');
assert(c.fleetUsd(10) === '$10' && c.fleetUsd(1234.5) === '$1,235' && c.fleetUsd(1234567) === '$1,234,567', 'from $10 whole dollars with separators');
assert(c.fleetUsd('x') === '$0.00' && c.fleetUsd(-3) === '$0.00' && c.fleetUsd(null) === '$0.00', 'bad input is $0.00');
assert(c.fleetTok(11.9e9) === '11.9B' && c.fleetTok(68e6) === '68M' && c.fleetTok(920000) === '920k' && c.fleetTok(42) === '42', 'short token forms');
assert(c.fleetTok(250e6) === '250M' && c.fleetTok(undefined) === '0', 'large and missing token values');

console.log('model colours');
assert(c.fleetModelClass({ model_class: 'large' }) === 'fl-mc-large' && c.fleetModelClass({ model_class: 'SMALL' }) === 'fl-mc-small', 'the class comes from the board, lower-cased');
assert(c.fleetModelClass('claude-opus-5-5') === 'fl-mc-other' && c.fleetModelClass({ model: 'claude-opus-5-5' }) === 'fl-mc-other', 'a model name alone never picks a class');
assert(c.fleetModelClass('<synthetic>') === 'fl-mc-other' && c.fleetModelClass(null) === 'fl-mc-other', 'others are grey');

console.log('card chip');
const chip = c.fleetCostChip(cost);
assert(chip.includes('≈ $1,235 API'), 'chip shows the rounded API figure');
assert(chip.includes('fl-cost-bar') && chip.includes('fl-mc-large" style="width:75%"') && chip.includes('fl-mc-small" style="width:25%"'), 'bar shares by cost');
assert(!chip.includes('fl-mc-other'), 'zero-cost models are not in the bar');
assert(chip.includes('claude-opus-5-5 · 40596 msgs · $900 · in 1k · write 1.5M · read 11.9B · out 920k'), 'tooltip lists model, messages, cost, tokens');
assert(chip.includes('&lt;synthetic&gt; · 3 msgs · unpriced'), 'unpriced model is labelled and escaped');
const part = c.fleetCostChip(Object.assign({}, cost, { partial: true }));
assert(part.includes('≈ … counting') && !part.includes('$1,235 API') && part.includes('fl-cost-partial'), 'partial reads counting');
assert(c.fleetCostChip(undefined) === '' && c.fleetCostChip(null) === '' && c.fleetCostChip('x') === '', 'absent cost renders nothing');
assert(c.fleetCostBar([]) === '' && c.fleetCostBar(undefined) === '', 'empty bar renders nothing');
const hostile = c.fleetCostChip({ usd: 1, by_model: [{ model: '"><script>x</script>', usd: 1, messages: 1, tokens: {} }] });
assert(!hostile.includes('<script') && hostile.includes('&lt;script&gt;'), 'hostile model name is escaped');

console.log('header total and panel');
const total = c.fleetCostTotal({ usd: 4321, by_model: [] }, false);
assert(total.includes('≈ <b>$4,321</b> API') && total.includes('id="fl-cost-toggle"') && total.includes('aria-expanded="false"'), 'total chip is a toggle button');
assert(c.fleetCostTotal({ usd: 1 }, true).includes('aria-expanded="true"'), 'open state is reflected');
assert(c.fleetCostTotal(undefined) === '' && c.fleetCostPanel(undefined, true) === '' && c.fleetCostPanel(cost, false) === '', 'absent cost or closed panel renders nothing');
const panel = c.fleetCostPanel(cost, true);
assert(panel.includes('claude-opus-5-5') && panel.includes('$900') && panel.includes('read 11.9B'), 'panel lists per-model dollars and tokens');
assert(panel.includes('API list-price equivalent · prices as of 2026-10-22 · not billed (plan)'), 'panel carries the footnote');
assert(c.fleetCostPanel({ usd: 1, priced_as_of: '<b>', by_model: [] }, true).includes('&lt;b&gt;'), 'priced_as_of is escaped');

console.log('order');
const s = (id, slot, usd) => ({ id, slot, live: true, started_at: 100, cost: usd == null ? undefined : { usd } });
const list = [s('a', '1', 5), s('b', '2', 500), s('c', '3'), s('d', '4', 500), s('e', '0', 5)];
assert(o.fleetOrder('cost') === 'cost', 'fleetOrder keeps cost');
assert(o.fleetSortBy(list, 'cost').map((x) => x.id).join(',') === 'b,d,e,a,c', 'cost desc, ties by slot, no cost last');
assert(o.fleetSortBy(list.map((x) => Object.assign({}, x, { cost: { usd: 'nope' } })), 'cost').map((x) => x.id).join(',') === 'e,a,b,c,d', 'non-numeric cost counts as zero and falls back to slot');

console.log('render and page');
const board = { host: 'h', accounts: [{ key: 'k', email: 'k' }], sessions: [
  { id: 'x1', short: 'x1', live: true, state: 'idle', account_key: 'k', slot: '1', started_at: 1, tools: [], agents: [], messages: [], cost },
  { id: 'x2', short: 'x2', live: true, state: 'idle', account_key: 'k', slot: '2', started_at: 2, tools: [], agents: [], messages: [] },
] };
const html = r.fleetAccounts(board, NOW, 'all', true);
assert(html.includes('fl-cost-chip') && html.split('fl-cost-chip').length === 2, 'only the card with cost gets a chip');
const byCost = r.fleetAccounts({ sessions: board.sessions.slice().reverse(), accounts: board.accounts }, NOW, 'all', true, 'cost');
assert(byCost.indexOf('id="fl-x1"') < byCost.indexOf('id="fl-x2"'), 'cost order puts the dearer card first');
const page = fleetPageHTML();
assert(page.includes('<option value="cost">highest cost</option>'), 'select offers highest cost');
assert(page.includes('fl-cost-wrap') && page.includes("key==='Escape'"), 'panel is an anchored dropdown closed by Escape');
assert(!/\sonclick=|\sonchange=/.test(page), 'no inline handlers (CSP)');
const script = fleetClientScript();
assert(script.includes("closest('#fl-cost-toggle')") && script.includes('FL.costOpen=!FL.costOpen'), 'toggle goes through the delegated click handler');
const idx = (n) => script.indexOf('function ' + n);
assert(['fleetUsd', 'fleetTok', 'fleetModelClass', 'fleetCostBar', 'fleetCostTokens', 'fleetCostTip', 'fleetCostChip', 'fleetCostTotal', 'fleetCostPanel'].every((n, i, a) => idx(n) > 0 && (i === 0 || idx(a[i - 1]) < idx(n))), 'cost helpers injected in dependency order');
assert(idx('fleetCostChip') < idx('fleetSession'), 'chip helper precedes fleetSession');
try { new (require('vm').Script)(script); assert(true, 'client script parses'); } catch (e) { assert(false, 'client script parses: ' + e.message); }

console.log('dropdown survives the board tick');
// A host whose innerHTML setter swaps in a fresh panel, as the browser does:
// the new element starts at scrollTop 0.
function fakeHost(panelTop) {
  const host = { panel: panelTop === null ? null : { scrollTop: panelTop }, html: '' };
  host.querySelector = (sel) => (sel === '.fl-cost-panel' ? host.panel : null);
  Object.defineProperty(host, 'innerHTML', {
    get() { return host.html; },
    set(v) { host.html = v; host.panel = v.includes('fl-cost-panel') ? { scrollTop: 0 } : null; },
  });
  return host;
}
const open = fakeHost(240);
c.fleetSetKeepScroll(open, '.fl-cost-panel', '<div class="fl-cost-panel">x</div>');
assert(open.innerHTML.includes('fl-cost-panel') && open.panel.scrollTop === 240, 're-render restores the panel scrollTop');
const shut = fakeHost(null);
c.fleetSetKeepScroll(shut, '.fl-cost-panel', '<div class="fl-cost-panel">x</div>');
assert(shut.panel.scrollTop === 0, 'a panel that was closed opens at the top');
const closing = fakeHost(240);
c.fleetSetKeepScroll(closing, '.fl-cost-panel', '<span>no panel</span>');
assert(closing.panel === null, 'closing the panel leaves nothing to restore');
const sc = fleetClientScript();
assert(sc.includes("fleetSetKeepScroll(document.getElementById('fl-counts'),'.fl-cost-panel'"), 'board render swaps the counts through the scroll-keeping helper');
assert(/data-filter\]'\);if\(f\)\{FL\.filter=f\.dataset\.filter;FL\.costOpen=false;/.test(sc), 'a filter click closes the dropdown');
assert(idx('fleetSetKeepScroll') > idx('fleetCostPanel') && idx('fleetSetKeepScroll') < idx('fleetSession'), 'scroll helper injected after the cost helpers');

console.log('390px layout');
assert(/\.fl-card-head\{[^}]*flex-wrap:wrap/.test(page), 'card header wraps so pill, wake and handback chips never push past the card');
assert(/\.fl-headline\{flex:1 1 8rem;/.test(page), 'headline keeps a basis so it wraps onto its own line instead of collapsing');

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed ? 1 : 0);
