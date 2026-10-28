// Fleet UI library entry. Input: a Snapshot (the board JSON) and, for the live
// page, an SSE URL. Output: HTML, CSS and a client script as strings. Nothing
// here imports a server framework; the host serves what this returns and owns
// authentication.

const render = require('./fleet-render');
const { fleetPageHTML, fleetCsp, fleetClientScript, FLEET_CSP_SELF } = require('./page');
const { fleetCss, themeCss } = require('./assets');

// Structure plus the default theme, as one stylesheet.
function fleetStyles() { return fleetCss() + themeCss(); }

// The board body as an HTML string, for hosts that render server-side or embed
// the cards in their own page. `now` is epoch seconds.
function fleetBoardHTML(snapshot, now, view) {
  const v = Object.assign({ filter: 'all', showEnded: true, order: 'slot' }, view || {});
  return render.fleetBranches(snapshot.branches) + render.fleetAccounts(snapshot, now, v.filter, v.showEnded, v.order);
}

module.exports = { fleetPageHTML, fleetCsp, FLEET_CSP_SELF, fleetClientScript, fleetStyles, fleetCss, themeCss, fleetBoardHTML, render };
