// Tests for the page shell: generic chrome, the linked shape that runs under
// script-src 'self', the structure/theme split and the snapshot envelope.
const fs = require('fs');
const path = require('path');
const { fleetPageHTML, fleetCsp, FLEET_CSP_SELF, fleetClientScript } = require('./page');
const lib = require('./index');

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

console.log('generic chrome');
const def = fleetPageHTML('', { assets: '/ui/' });
assert(def.includes('<title>FleetWatch</title>') && def.includes('<h1 class="fl-h1"><span>FleetWatch</span></h1>'), 'title defaults to FleetWatch');
assert(!def.includes('fl-back'), 'no back link by default');
assert(!/dashboard|claude|local dev/i.test(def), 'no estate wording in the default page');
const custom = fleetPageHTML('', { assets: '/ui/', title: 'My "Fleet"', backHref: '/home?a=1&b=2', backLabel: 'Home' });
assert(custom.includes('<title>My &quot;Fleet&quot;</title>') && custom.includes('href="/home?a=1&amp;b=2">← Home</a>'), 'title, back link and label come from options and are escaped');
assert(fleetPageHTML('', { backHref: '/x' }).includes('← Back</a>'), 'back label defaults to Back');
const headed = fleetPageHTML('', { assets: '/ui/', title: 'Fleet — Site', heading: { accent: 'My<', text: 'Fleet&' }, backHref: '/', backTitle: 'Back "home"', emptyText: 'Nothing <yet>' });
assert(headed.includes('<title>Fleet — Site</title>') && headed.includes('<h1 class="fl-h1"><span>My&lt;</span> Fleet&amp;</h1>'), 'heading is separate from the title and escaped');
assert(headed.includes('href="/" title="Back &quot;home&quot;">'), 'back link tooltip comes from backTitle, escaped');
assert(headed.includes('<div class="fl-empty">Nothing &lt;yet&gt;</div>'), 'empty-state text comes from emptyText, escaped');

console.log('linked shape');
assert(!/<script(?![^>]*\bsrc=)/.test(def) && !/<style/.test(def), 'no inline script or style element');
assert(def.includes('<script src="/ui/fleet.js"></script>') && def.includes('href="/ui/fleet.css"') && def.includes('href="/ui/theme.css"'), 'script and both stylesheets are linked');
assert(!/\son[a-z]+=/.test(def), 'no inline event handlers');
assert(/script-src 'self'/.test(FLEET_CSP_SELF) && /style-src 'self'/.test(FLEET_CSP_SELF) && !/script-src[^;]*unsafe/.test(FLEET_CSP_SELF), 'the linked policy admits only same-origin scripts');
assert(/base-uri 'none'/.test(FLEET_CSP_SELF) && /form-action 'none'/.test(FLEET_CSP_SELF), 'the policy blocks base and form redirection');
assert(/frame-ancestors 'none'/.test(FLEET_CSP_SELF) && /connect-src 'self'/.test(FLEET_CSP_SELF), 'the policy forbids framing and cross-origin connections');

console.log('inline shape');
const inline = fleetPageHTML('N1', { themeCss: ':root{--fl-night:#fff}' });
assert(inline.includes('<style nonce="N1">') && inline.includes('<script nonce="N1">') && inline.includes('--fl-night:#fff'), 'inline page carries the nonce and a host theme');
assert(!inline.includes('--fl-sky-panel:#1e3a5f'), 'a host theme replaces the default one');
assert(/script-src 'nonce-N1'/.test(fleetCsp('N1')), 'the inline policy pins scripts to the nonce');

console.log('structure and theme');
const css = fs.readFileSync(path.join(__dirname, 'fleet.css'), 'utf8');
const theme = fs.readFileSync(path.join(__dirname, 'theme.css'), 'utf8');
assert(!/[:\s,(]#[0-9a-f]{3,8}\b/i.test(css) && !/rgba?\(|hsl\(\s*\d/.test(css.replace(/hsl\(var\(--br-h\)[^)]*\)/g, '')), 'fleet.css holds no colour literal');
const used = new Set((css.match(/var\(--fl-[a-z-]+/g) || []).map((s) => s.slice(4)));
const defined = new Set((theme.match(/--fl-[a-z-]+(?=:)/g) || []));
const missing = [...used].filter((t) => !defined.has(t) && t !== '--fl-top');
assert(missing.length === 0, 'theme.css defines every token fleet.css uses' + (missing.length ? ': ' + missing.join(', ') : ''));
assert(!/--(night|panel|mono|sans|mc-)/.test(css.replace(/--fl-/g, '')) , 'every custom property carries the fl prefix');
assert(lib.fleetStyles() === css + theme, 'fleetStyles is structure plus theme');
assert(/\.fl-mc-other|--fl-mc-other/.test(css), 'unmapped model classes fall back to the neutral token');

console.log('no model names in code');
const code = fs.readdirSync(__dirname).filter((f) => f.endsWith('.js') && !f.endsWith('.test.js'));
const named = code.filter((f) => /opus|sonnet|haiku|fable|gpt-|gemini/i.test(fs.readFileSync(path.join(__dirname, f), 'utf8')));
assert(named.length === 0, 'no model family appears in the library code' + (named.length ? ': ' + named.join(', ') : ''));

console.log('snapshot shapes');
const script = fleetClientScript();
const m = script.match(/function flBoardOf[\s\S]*?\nfunction flApply/);
const flBoardOf = new Function('FL', m[0].replace(/\nfunction flApply$/, '') + '\nreturn flBoardOf;');
const FL = {};
const bare = { counts: {}, sessions: [] };
assert(flBoardOf(FL)(bare) === bare, 'a bare snapshot is the board');
assert(flBoardOf(FL)({ available: true, board: bare }) === bare, 'the envelope is unwrapped');
assert(flBoardOf(FL)({ available: false, error: 'down' }) === null && FL.down === 'down', 'an unavailable envelope reports its error');
assert(flBoardOf(FL)({ error: 'first pass not finished' }) === null && FL.down === 'first pass not finished', 'an error body is not a board');
assert(script.includes('"/v1/events"') && script.includes('"/v1/fleet"') && script.includes('"/v1/session"'), 'default URLs are the FleetWatch routes');
assert(script.includes("r.status===401") && script.includes("method:'POST'") && script.includes("credentials:'same-origin'"), 'client handles 401 with a sign-in that posts the token');
assert(!script.includes('localStorage.setItem(\'token') && !/Authorization/.test(script), 'the client never stores the token or sends it as a header');

console.log(`\n${passed} passed, ${failed} failed`);
if (failed) process.exit(1);
