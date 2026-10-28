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

// The subagent box (id fl-ag-<id>) inside `card` that a link end names, or null
// when the link names none, the box is not on the page, or it is folded away.
function fleetAgentBox(id, card, byId) {
  var el = id ? byId('fl-ag-' + id) : null;
  if (!el || !fleetHasBox(el)) return null;
  return card.contains && !card.contains(el) ? null : el;
}

// How a link is drawn, or null when it should not be. `byId` maps a DOM id to
// an element. Across two cards the arrow runs card to card (output port on the
// sender's right edge, input port on the receiver's left) and srcBox/dstBox are
// the visible subagent boxes it should line up with, or null. Inside one card
// (inCard) the arrow runs from the sender's box to the receiver's: a subagent
// box, else the card header. When both ends land on the same element, because a
// box is folded away or the narrow layout hides it, nothing is drawn.
function fleetLinkCards(l, now, byId) {
  if (!fleetLinkFresh(l, now)) return null;
  var a = byId('fl-' + l.from), c = byId('fl-' + l.to);
  if (!a || !c || !fleetHasBox(a) || !fleetHasBox(c)) return null;
  var cls = fleetLinkClass(l);
  var sb = fleetAgentBox(l.from_agent, a, byId), db = fleetAgentBox(l.to_agent, c, byId);
  if (a !== c) return { src: a, dst: c, srcBox: sb, dstBox: db, cls: cls, inCard: false };
  var head = a.querySelector ? a.querySelector('.fl-card-head') : null;
  if (head && !fleetHasBox(head)) head = null;
  var s = sb || head, d = db || head;
  if (!s || !d || s === d) return null;
  return { src: a, dst: c, srcBox: s, dstBox: d, cls: cls, inCard: true };
}

module.exports = { fleetHasBox, fleetLinkClass, fleetLinkFresh, fleetLinkSig, fleetAgentBox, fleetLinkCards };
