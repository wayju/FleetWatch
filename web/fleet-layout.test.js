// Layout check for /fleet: loads the real page, fed a synthetic board over
// a local SSE endpoint, in headless Chrome. At 390px it asserts there is no
// horizontal scroll; at 1280px it asserts every message arrow ends on its
// receiver's left edge. FLEET_SHOT_DIR=<dir> also writes screenshots of both.
//
// Needs Chrome (CHROME_BIN or /usr/bin/google-chrome). Without it the check is
// skipped with a note, unless FLEET_LAYOUT_REQUIRED=1, which makes that a failure.
const http = require('http');
const fs = require('fs');
const path = require('path');
const { fleetPageHTML } = require('./page');
const { findChrome, launch } = require('./fleet-chrome');

const chrome = findChrome();
if (!chrome) {
  if (process.env.FLEET_LAYOUT_REQUIRED === '1') { console.error('  FAIL: no Chrome found for the layout check'); process.exit(1); }
  console.log('SKIP: no Chrome found, layout check not run');
  process.exit(0);
}

let passed = 0;
let failed = 0;
function assert(condition, message) {
  if (condition) { console.log(`  PASS: ${message}`); passed += 1; }
  else { console.error(`  FAIL: ${message}`); failed += 1; }
}

function session(n, over) {
  return Object.assign({
    id: 'sess-' + n, short: 'sess' + n, live: true, link: 'projection', pid: 100 + n, slot: String(n), pane: '%' + n,
    account: { email: 'wayne@example.test' }, account_key: 'wayne@example.test',
    state: n % 2 ? 'working' : 'waiting', state_since: 0,
    repo: { owner: 'org', name: 'app' + n, branch: 'feature/' + (40 + n) + '-fleet', commit: '0123456' + n, version: '1.' + n + '.0' },
    branch: 'feature/' + (40 + n) + '-fleet', claude_version: '2.1.306', model: 'claude-sonnet-5-5',
    context: { tokens: 50000, limit: 200000, pct: 25, source: 'transcript' },
    issues: [], recap: { last_prompt: 'work on item ' + n, last_reply: 'Done.' }, tools: [], agents: [], messages: [],
    started_at: 0, last_activity: 0,
  }, over || {});
}

function board() {
  const now = Math.floor(Date.now() / 1000);
  const sessions = [1, 2, 3, 4, 5, 6].map((n) => session(n, { state_since: now - 60 * n, started_at: now - 3600, last_activity: now - 5 }));
  const link = (from, to, ago, preview, kind) => ({ from: 'sess-' + from, to: 'sess-' + to, from_agent: '', to_agent: '', kind: kind || 'message', at: now - ago, preview, external: false });
  return {
    host: 'fixture', generated_at: now, counts: { live: 6, waiting: 3, working: 3 },
    accounts: [{ key: 'wayne@example.test', email: 'wayne@example.test', slots: ['1', '2', '3', '4', '5', '6'], live: 6 }],
    sessions, branches: [], log: [],
    links: [
      link(1, 2, 4, 'please review the narrow layout', 'message'),   // same row, left to right
      link(2, 1, 6, 'reviewed, two nits on the gutters', 'handback'),  // same row, right to left
      link(1, 4, 8, 'rebase onto main before you start', 'message'), // cross row
      link(3, 4, 10, 'tests are green on my branch', 'message'),     // shares the column gutter
      link(5, 2, 12, 'the deploy finished, check the dev page', 'message'), // up two rows
      link(6, 5, 14, 'taking the bottom-row ticket now', 'message'), // right to left, bottom row
    ],
  };
}

// What the page actually laid out, read through DevTools once the arrows are drawn.
const MEASURE = `(function(){
  var main=document.querySelector('.fl-main').getBoundingClientRect(),arrows=[];
  document.querySelectorAll('#fl-links .fl-line').forEach(function(p){
    var n=p.getAttribute('d').match(/-?[\\d.]+/g).map(Number),to=document.getElementById(p.parentNode.dataset.to).getBoundingClientRect();
    arrows.push({endX:n[n.length-2]+main.left,endY:n[n.length-1]+main.top,left:to.left,top:to.top,bottom:to.bottom});
  });
  return {innerWidth:innerWidth,scrollWidth:document.documentElement.scrollWidth,cards:document.querySelectorAll('.fl-card').length,
    arrows:arrows,pills:document.querySelectorAll('#fl-links .fl-pillg').length,xmsgs:document.querySelectorAll('.fl-xmsgs').length,
    svgShown:getComputedStyle(document.getElementById('fl-links')).display!=='none'};
})()`;

const server = http.createServer((req, res) => {
  if (req.url.startsWith('/stream')) {
    res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' });
    res.write('event: board\ndata: ' + JSON.stringify(board()) + '\n\n');
    return;
  }
  res.writeHead(200, { 'Content-Type': 'text/html' });
  res.end(fleetPageHTML('', { streamUrl: '/stream', snapshotUrl: '/snapshot' }));
});

// Async on purpose: the fixture server lives in this process, so a blocking
// spawn would starve it and Chrome would never get its page.
async function measure(page, url, width) {
  await page.open(url, width, 900);
  await page.waitFor("document.querySelectorAll('.fl-card').length === 6", 15000);
  // Arrows are drawn after the cards, on the same board event.
  if (width > 700) await page.waitFor("document.querySelectorAll('#fl-links .fl-line').length > 0", 15000);
  return page.evaluate(MEASURE);
}

server.listen(0, '127.0.0.1', async () => {
  const url = 'http://127.0.0.1:' + server.address().port + '/';
  let browser;
  try {
    browser = await launch(chrome);
    const shots = process.env.FLEET_SHOT_DIR;
    if (shots) fs.mkdirSync(shots, { recursive: true });
    const shoot = async (w) => {
      if (!shots) return;
      const out = path.join(shots, 'fleet-' + w + '.png');
      await browser.page.screenshot(out);
      console.log('  screenshot ' + out);
    };

    console.log('narrow (390px)');
    const narrow = await measure(browser.page, url, 390);
    assert(narrow.cards === 6, 'all six session cards render');
    assert(narrow.scrollWidth <= narrow.innerWidth, 'no horizontal scroll (' + narrow.scrollWidth + ' <= ' + narrow.innerWidth + ')');
    assert(narrow.svgShown === false && narrow.arrows.length === 0, 'connector layer is off and the inline messages list takes over');
    assert(narrow.xmsgs > 0, 'cards carry an inline messages list');
    await shoot(390);

    console.log('desktop (1280px)');
    const wide = await measure(browser.page, url, 1280);
    assert(wide.cards === 6, 'all six session cards render');
    assert(wide.scrollWidth <= wide.innerWidth, 'no horizontal scroll (' + wide.scrollWidth + ' <= ' + wide.innerWidth + ')');
    assert(wide.arrows.length === 6 && wide.pills === 6, 'six arrows, each with a label pill');
    wide.arrows.forEach((a, i) => {
      assert(Math.abs(a.endX - a.left) <= 0.5, 'arrow ' + (i + 1) + ' ends on its receiver left edge (' + a.endX.toFixed(1) + ' vs ' + a.left.toFixed(1) + ')');
      assert(a.endY > a.top && a.endY < a.bottom, 'arrow ' + (i + 1) + ' ends level with its receiver');
    });
    await shoot(1280);
  } catch (e) {
    assert(false, String(e.message || e));
  }
  if (browser) await browser.close();
  server.close();
  console.log(`\n${passed} passed, ${failed} failed`);
  process.exit(failed ? 1 : 0);
});
