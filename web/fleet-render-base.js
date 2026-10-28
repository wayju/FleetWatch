// Leaf helpers shared by fleet-render.js and fleet-render-activity.js, split out
// so those two need not require each other. Same injection rules as
// fleet-render.js: functions only, no module-level bindings.

function fleetEsc(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

// Seconds -> "now" / "42s" / "12m" / "3h" / "2d".
function fleetAge(sec) {
  if (sec == null || !isFinite(sec) || sec < 0) return '';
  if (sec < 5) return 'now';
  if (sec < 60) return Math.floor(sec) + 's';
  if (sec < 3600) return Math.floor(sec / 60) + 'm';
  if (sec < 86400) return Math.floor(sec / 3600) + 'h';
  return Math.floor(sec / 86400) + 'd';
}

// fleetAge with a suffix that reads as a phrase: "28s ago", but "just now"
// rather than "now ago". No suffix returns the plain age.
function fleetAgeSfx(sec, sfx) {
  var a = fleetAge(sec);
  if (!sfx || !a) return a;
  return a === 'now' ? 'just now' : a + sfx;
}

// A board timestamp as a ticking age. The page re-renders every [data-t]
// once a second, so "waiting 12m" keeps counting between boards. An optional
// sfx (" ago") rides in data-sfx so the tick keeps the phrase grammatical.
function fleetAgo(t, now, sfx) {
  if (!t) return '';
  return '<span class="fl-ago" data-t="' + fleetEsc(Number(t) || 0) + '"' + (sfx ? ' data-sfx="' + fleetEsc(sfx) + '"' : '') + '>' + fleetAgeSfx(now - t, sfx) + '</span>';
}

module.exports = { fleetEsc, fleetAge, fleetAgeSfx, fleetAgo };
