// The fleet page: a document that renders a Snapshot (the board JSON) and
// follows a Server-Sent Events stream. It has no server imports; the host
// supplies the URLs and serves the result.
//
// Two shapes, one markup. With opts.assets set (a path prefix such as "/ui/")
// the page links fleet.css, theme.css and fleet.js, carries no inline script
// or style element, and so runs under script-src 'self'. Without it the page
// inlines all three under a per-request nonce, for a host that keeps a
// nonce-based policy.

const { fleetClientScript } = require('./client');
const { fleetCss, themeCss } = require('./assets');

// Policy for the inline shape. The style="" attributes the renderer emits
// (branch hue, context bar width) are allowed by style-src-attr only.
function fleetCsp(nonce) {
  return "default-src 'none'; script-src 'nonce-" + nonce + "'; style-src-elem 'nonce-" + nonce
    + "'; style-src-attr 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'";
}

// Policy for the linked shape: everything from this origin, nothing inline
// except style attributes.
const FLEET_CSP_SELF = "default-src 'none'; script-src 'self'; style-src 'self'; style-src-attr 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'";

function attr(s) {
  return String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function head(title, nonce, o) {
  const n = nonce ? ' nonce="' + nonce + '"' : '';
  const meta = '<meta charset="UTF-8">\n<meta name="viewport" content="width=device-width, initial-scale=1.0">\n<title>' + attr(title) + '</title>\n';
  if (o.assets != null) {
    const base = attr(o.assets);
    return meta + '<link rel="stylesheet" href="' + base + 'fleet.css">\n<link rel="stylesheet" href="' + base + 'theme.css">\n';
  }
  return meta + '<style' + n + '>\n' + fleetCss() + (o.themeCss != null ? o.themeCss : themeCss()) + '</style>\n';
}

function login() {
  return `<div class="fl-login" id="fl-login" hidden>
  <form id="fl-login-form" class="fl-login-box" autocomplete="off">
    <label for="fl-token">Access token</label>
    <input id="fl-token" type="password" autocomplete="off" spellcheck="false" required>
    <button type="submit" class="fl-btn">sign in</button>
    <div class="fl-login-err" id="fl-login-err" role="alert"></div>
  </form>
</div>`;
}

// opts: { streamUrl, snapshotUrl, sessionUrl, title, heading, backHref, backLabel,
// backTitle, emptyText, assets, themeCss }. All are optional; the title defaults
// to "FleetWatch" and there is no back link unless backHref is given. heading is
// { accent, text }: the accent is drawn in the highlight colour and the text
// after it plain, so a host can head the page differently from its <title>.
function fleetPageHTML(nonce, opts) {
  const o = opts || {};
  const title = o.title || 'FleetWatch';
  const hd = o.heading || { accent: title };
  const h1 = '<span>' + attr(hd.accent || '') + '</span>' + (hd.text ? ' ' + attr(hd.text) : '');
  const tip = o.backTitle ? ' title="' + attr(o.backTitle) + '"' : '';
  const back = o.backHref
    ? '<a class="fl-back" href="' + attr(o.backHref) + '"' + tip + '>← ' + attr(o.backLabel || 'Back') + '</a>\n  '
    : '';
  const empty = attr(o.emptyText || 'Waiting for the first board…');
  const n = nonce ? ' nonce="' + nonce + '"' : '';
  const script = o.assets != null
    ? '<script src="' + attr(o.assets) + 'fleet.js"></script>'
    : '<script' + n + '>\n' + fleetClientScript(o) + '\n</script>';
  return `<!DOCTYPE html>
<html lang="en">
<head>
${head(title, nonce, o)}</head>
<body>
<div class="fl-top">
  ${back}<h1 class="fl-h1">${h1}</h1>
  <span class="fl-live" id="fl-live"><i></i>connecting</span>
  <button type="button" class="fl-btn" id="fl-signout" hidden>sign out</button>
  <div class="fl-topright"><div class="fl-counts" id="fl-counts"></div><div class="fl-slots" id="fl-slots" aria-label="jump to a session"></div></div>
  <div class="fl-controls">
    <button type="button" class="fl-btn" data-filter="all">all</button>
    <button type="button" class="fl-btn" data-filter="attention">needs me</button>
    <button type="button" class="fl-btn" data-filter="working">working</button>
    <button type="button" class="fl-btn" id="fl-ended">show ended</button>
    <label class="fl-order"><span>order</span><select class="fl-btn" id="fl-order">
      <option value="slot">slot</option>
      <option value="started">started</option>
      <option value="attention">needs me first</option>
      <option value="active">last active</option>
      <option value="cost">highest cost</option>
    </select></label>
  </div>
</div>
${login()}
<main class="fl-main">
  <svg id="fl-links" aria-hidden="true"></svg>
  <div id="fl-branches"></div>
  <div id="fl-accounts"><div class="fl-empty">${empty}</div></div>
  <details class="fl-log" data-key="log"><summary>session log</summary><div id="fl-log-body"></div></details>
</main>
${script}
</body></html>`;
}

module.exports = { fleetCsp, fleetPageHTML, fleetClientScript, FLEET_CSP_SELF };
