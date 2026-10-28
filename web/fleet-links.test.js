// Tests for fleet agent links: endpoint selection, message-line markup and page CSS.
const links = require('./fleet-links');
const r = require('./fleet-render');
const { fleetPageHTML, fleetClientScript } = require('./page');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

const NOW = 1792576900;
function el(name, box, lines) {
  return { name, getClientRects: () => (box === false ? [] : [{}]), querySelectorAll: () => lines || [] };
}
function lookup(map) { return (id) => map[id] || null; }
function link(over) {
  return Object.assign({ from: 'S1', to: 'S2', from_agent: '', to_agent: '', kind: 'message', at: NOW - 10, external: false }, over);
}

// Card selection: arrows run card to card, whatever agents sit inside them.
const s1 = el('s1'), s2 = el('s2');
let map = { 'fl-S1': s1, 'fl-S2': s2 };
let e = links.fleetLinkCards(link(), NOW, lookup(map));
assert(e && e.src === s1 && e.dst === s2 && e.cls === '', 'session to session keeps the default class');
e = links.fleetLinkCards(link({ from_agent: 'A', to_agent: 'B' }), NOW, lookup(map));
assert(e && e.src === s1 && e.dst === s2 && e.cls === 'fl-link-agent', 'agent traffic still ends on the session cards, with the agent class');
e = links.fleetLinkCards(link({ from_agent: 'A', kind: 'handback' }), NOW, lookup(map));
assert(e && e.cls === 'fl-link-handback', 'handback gets the handback class');
assert(links.fleetLinkCards(link({ to: 'S1' }), NOW, lookup(map)) === null, 'a message inside one card draws no arrow');
assert(links.fleetLinkCards(link({ to: 'GONE' }), NOW, lookup(map)) === null, 'missing card skipped');
assert(links.fleetLinkCards(link({ external: true }), NOW, lookup(map)) === null, 'external link skipped');
assert(links.fleetLinkCards(link({ at: NOW - 31 }), NOW, lookup(map)) === null, 'a link older than 30 seconds is not drawn');
assert(links.fleetLinkCards(link({ at: NOW - 30 }), NOW, lookup(map)) !== null, 'a link exactly 30 seconds old is still drawn');
assert(links.fleetLinkCards(link(), NOW, lookup({ 'fl-S1': s1, 'fl-S2': el('hidden', false) })) === null, 'a card with no box is skipped');
// A closed <details> still reports rects for its content, at a phantom spot below the card.
const folded = Object.assign(el('folded'), { checkVisibility: () => false });
assert(links.fleetHasBox(folded) === false, 'checkVisibility false means no box despite client rects');
assert(links.fleetHasBox(Object.assign(el('shown'), { checkVisibility: () => true })) === true, 'checkVisibility true keeps the box');
assert(links.fleetHasBox(Object.assign(el('old'), { closest: () => ({}) })) === false, 'without checkVisibility, content of a closed details has no box');
assert(links.fleetHasBox(Object.assign(el('old2'), { closest: () => null })) === true, 'without checkVisibility, an element outside closed details keeps its box');
assert(links.fleetLinkSig([link({ at: NOW - 5 }), link({ at: NOW - 40 }), link({ at: NOW - 5, external: true })], NOW) === '0,', 'signature lists only fresh, internal links');
assert(links.fleetLinkSig([link({ at: NOW - 5 })], NOW) !== links.fleetLinkSig([link({ at: NOW - 5 })], NOW + 26), 'signature changes when a line expires, so the page redraws');
assert(links.fleetLinkSig(undefined, NOW) === '', 'a board without links has an empty signature');

// Message line markup
const sess = {
  id: 'S1', short: 'S1', live: true, state: 'waiting', state_since: NOW - 10, repo: {}, agents: [], tools: [],
  messages: [
    { to: 'S1', to_agent: 'a39a913ba1', to_label: 'code-reviewer <a39a913>', preview: 'hi', at: NOW - 5, external: false, kind: 'message' },
    { to: 'S2', to_agent: '', to_label: 'other', preview: 'yo', at: NOW - 5, external: false, kind: 'message' },
  ],
};
const html = r.fleetHistory(sess, NOW);
assert(html.includes('href="#fl-ag-a39a913ba1"') && html.includes('data-agent="a39a913ba1"') && html.includes('data-target="S1"'), 'agent message line links to the agent card and carries data-agent');
assert(html.includes('code-reviewer &lt;a39a913&gt;') && !html.includes('<a39a913>'), 'to_label is escaped');
assert(html.includes('href="#fl-S2"') && !/data-target="S2" data-agent/.test(html), 'session message line is unchanged');

// Page CSS and script
const page = fleetPageHTML('N');
assert(page.includes('#fl-links .fl-link-agent') && page.includes('#fl-links .fl-link-handback'), 'page styles agent and handback links');
const rm = page.slice(page.indexOf('prefers-reduced-motion'));
assert(rm.includes('#fl-links .fl-line') && rm.includes('animation:none'), 'reduced motion stops the dash animation');
const script = fleetClientScript();
assert(script.includes('fl-arrow-agent') && script.includes('fl-arrow-handback') && script.includes('function fleetLinkCards') && script.includes('function fleetRoute('), 'client script carries the markers, the helper and the router');
assert(script.includes('markerUnits="userSpaceOnUse"'), 'arrowheads are sized in pixels, not stroke widths');
// Markers are children of <defs>, not of the link groups, so the dashed no-fill rule cannot hollow them.
assert(page.includes('#fl-links .fl-line{fill:none') && !page.includes('#fl-links path{'), 'dashed no-fill style applies to the line paths only, so arrowheads stay solid');
assert(/#fl-links\{[^}]*z-index:4/.test(page), 'link layer stacks above the cards');
assert(page.includes('@media (min-width:701px){.fl-main{padding:1.5rem 1.75rem}.fl-grid{gap:3rem 3.75rem}.fl-account{margin-bottom:2.5rem}}'), 'desktop gutters are wide enough for lanes');
assert(/fleetEsc\(lab\.text\)/.test(script) && /fleetEsc\(lab\.full\)/.test(script), 'pill text and tooltip are escaped');

console.log(`\n${passed} passed, ${failed} failed`);
if (failed) process.exit(1);
