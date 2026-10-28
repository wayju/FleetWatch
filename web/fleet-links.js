// Pure endpoint and class selection for the message arrows on /fleet.
// Functions are serialised into the page script, so each is self-contained.

// Client rects alone are not enough: Chrome lays out the content of a closed
// <details> where it would sit if open, a phantom spot below the card.
function fleetHasBox(el) {
  if (!el || el.getClientRects().length === 0) return false;
  if (el.checkVisibility) return el.checkVisibility();
  return !(el.closest && el.closest('details:not([open]) > :not(summary), details:not([open]) > :not(summary) *'));
}

// Class for the path: handbacks amber, other agent traffic sky, session to session default.
function fleetLinkClass(l) {
  if (l.kind === 'handback') return 'fl-link-handback';
  return l.from_agent || l.to_agent ? 'fl-link-agent' : '';
}

// A message line shows for 30 seconds after it was sent, so a line on the
// board means a conversation happening now, not one that has finished.
function fleetLinkFresh(l, now) {
  return !l.external && now - l.at <= 30;
}

// Which links are fresh, as a string: the page redraws when it changes, so a
// line leaves on time even when no new board arrives.
function fleetLinkSig(links, now) {
  var s = '';
  for (var i = 0; i < (links || []).length; i++) if (fleetLinkFresh(links[i], now)) s += i + ',';
  return s;
}

// The two session cards a link joins, or null when it should not be drawn.
// `byId` maps a DOM id to an element. Arrows run card to card (output port on
// the sender's right edge, input port on the receiver's left), so subagent
// boxes inside a card no longer pick the endpoint.
function fleetLinkCards(l, now, byId) {
  if (!fleetLinkFresh(l, now)) return null;
  var a = byId('fl-' + l.from), c = byId('fl-' + l.to);
  if (!a || !c || a === c || !fleetHasBox(a) || !fleetHasBox(c)) return null;
  return { src: a, dst: c, cls: fleetLinkClass(l) };
}

module.exports = { fleetHasBox, fleetLinkClass, fleetLinkFresh, fleetLinkSig, fleetLinkCards };
