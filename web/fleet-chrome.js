// Minimal headless-Chrome driver for the fleet layout check: launches Chrome
// with a DevTools port, opens one page and exposes viewport sizing, evaluate
// and screenshot over the DevTools protocol (Node's built-in WebSocket, no
// dependencies). Test support only; nothing in the app imports it.
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

function findChrome() {
  return process.env.CHROME_BIN || ['/usr/bin/google-chrome', '/usr/bin/chromium', '/usr/bin/chromium-browser'].find((p) => fs.existsSync(p)) || null;
}

function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Starts Chrome with a DevTools port; resolves to { proc, port }.
function startChrome(chrome, profile) {
  const proc = spawn(chrome, ['--headless=new', '--no-sandbox', '--disable-gpu', '--hide-scrollbars', '--remote-debugging-port=0',
    '--user-data-dir=' + profile, 'about:blank'], { stdio: ['ignore', 'ignore', 'pipe'] });
  return new Promise((resolve, reject) => {
    let buf = '';
    const t = setTimeout(() => reject(new Error('Chrome did not announce a DevTools port')), 20000);
    proc.stderr.on('data', (d) => {
      buf += d;
      const m = /DevTools listening on ws:\/\/127\.0\.0\.1:(\d+)\//.exec(buf);
      if (m) { clearTimeout(t); resolve({ proc, port: Number(m[1]) }); }
    });
    proc.on('exit', () => reject(new Error('Chrome exited before it was ready')));
  });
}

// Opens the first page target; resolves to { ws, send } where send(method, params) is a DevTools call.
async function connect(port) {
  const targets = await (await fetch('http://127.0.0.1:' + port + '/json/list')).json();
  const ws = new WebSocket(targets.find((x) => x.type === 'page').webSocketDebuggerUrl);
  await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = () => reject(new Error('DevTools socket failed')); });
  let id = 0;
  const waiting = new Map();
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.id && waiting.has(msg.id)) {
      const { resolve, reject } = waiting.get(msg.id);
      waiting.delete(msg.id);
      if (msg.error) reject(new Error(msg.error.message)); else resolve(msg.result);
    }
  };
  const send = (method, params) => new Promise((resolve, reject) => {
    id += 1;
    waiting.set(id, { resolve, reject });
    ws.send(JSON.stringify({ id, method, params: params || {} }));
  });
  return { ws, send };
}

// Starts Chrome and returns { page, close }.
async function launch(chrome) {
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'fleet-chrome-'));
  const { proc, port } = await startChrome(chrome, profile);
  const { ws, send } = await connect(port);
  const page = {
    send,
    // Sets the viewport (a real width, so the media queries see it) and loads the url.
    async open(url, width, height) {
      await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false });
      await send('Page.navigate', { url });
    },
    async evaluate(expression) {
      const r = await send('Runtime.evaluate', { expression, returnByValue: true });
      if (r.exceptionDetails) throw new Error('page script failed: ' + (r.exceptionDetails.exception && r.exceptionDetails.exception.description));
      return r.result.value;
    },
    // Polls until the expression is truthy, or throws on timeout.
    async waitFor(expression, ms) {
      const end = Date.now() + ms;
      while (Date.now() < end) {
        if (await page.evaluate(expression)) return;
        await sleep(100);
      }
      throw new Error('timed out waiting for: ' + expression);
    },
    // Full-page PNG to `file`.
    async screenshot(file) {
      const dims = await page.evaluate('({w:document.documentElement.scrollWidth,h:document.documentElement.scrollHeight})');
      const shot = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true, clip: { x: 0, y: 0, width: dims.w, height: dims.h, scale: 1 } });
      fs.writeFileSync(file, Buffer.from(shot.data, 'base64'));
    },
  };
  async function close() {
    try { ws.close(); } catch (e) { /* already closed */ }
    proc.kill('SIGKILL');
    await sleep(100);
    fs.rmSync(profile, { recursive: true, force: true });
  }
  return { page, close };
}

module.exports = { findChrome, launch, sleep };
