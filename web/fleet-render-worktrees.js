// "Worktrees in flight" strip for the fleet page. Replaces the branch
// strip when the board carries `worktrees`. Same injection rules as
// fleet-render.js: functions only, no module-level bindings besides requires,
// every board string through fleetEsc. The page never runs anything; the
// removal hint is text a click copies to the clipboard.

const { fleetEsc, fleetAgo } = require('./fleet-render-base');
const { fleetBranchChip, fleetBranches } = require('./fleet-render');

// Newest writer first; a board without last_at keeps its order.
function fleetWtWriters(wt) {
  var ws = Array.isArray(wt.writers) ? wt.writers.filter(Boolean) : [];
  return ws.slice().sort(function (a, b) { return (Number(b.last_at) || 0) - (Number(a.last_at) || 0); });
}

// The worktree's name: the path under .claude/worktrees/,
// which usually but not always equals the branch. '' when path_rel is elsewhere.
function fleetWtName(wt) {
  var p = String(wt.path_rel || '');
  var pre = '.claude/worktrees/';
  return p.indexOf(pre) === 0 ? p.slice(pre.length).replace(/\/+$/, '') : '';
}

// Copyable removal hint for stale and merged worktrees; '' without a name.
function fleetWtHint(wt) {
  var name = fleetWtName(wt);
  // The hint is copied to a shell, so a name with anything unusual in it gets none.
  if (!name || !/^[A-Za-z0-9._\/-]+$/.test(name) || name.indexOf('..') >= 0) return '';
  var cmd = 'git worktree remove .claude/worktrees/' + name;
  return '<code class="fl-wt-cmd" data-copy="' + fleetEsc(cmd) + '" title="click to copy">' + fleetEsc(cmd) + '</code>';
}

// Active worktree: repo, branch, issue, newest writer (anchored to its card
// and agent box) and a ticking age.
function fleetWtChip(wt, now) {
  var ws = fleetWtWriters(wt);
  var w = ws[0] || {};
  var inner = '<span class="fl-repo">' + fleetEsc(String(wt.repo || '').split('/').pop()) + '</span>'
    + fleetBranchChip(wt.branch)
    + (wt.issue ? '<span class="fl-wt-issue">#' + fleetEsc(wt.issue) + '</span>' : '')
    + (w.slot !== undefined && w.slot !== '' && w.slot !== null ? '<span class="fl-wt-slot">slot ' + fleetEsc(w.slot) + '</span>' : '')
    + (w.agent_desc ? '<span class="fl-wt-agent">› ' + fleetEsc(w.agent_desc) + '</span>' : '')
    + (w.last_at ? fleetAgo(w.last_at, now, ' ago') : '')
    + (wt.two_writers ? '<span class="fl-warn">⚠ two writers</span>' : '');
  var cls = 'fl-strip-item fl-wt' + (wt.two_writers ? ' fl-shared' : '');
  var who = ws.map(function (x) { return 'slot ' + (x.slot || '?') + (x.agent_desc ? ' › ' + x.agent_desc : ''); }).join(', ');
  var title = fleetEsc((wt.path_rel || '') + (who ? ' · ' + who : ''));
  if (!w.session) return '<span class="' + cls + '" title="' + title + '">' + inner + '</span>';
  return '<a class="' + cls + '" href="#fl-' + fleetEsc(w.session) + '"'
    + (w.agent_id ? ' data-agent="' + fleetEsc(w.agent_id) + '"' : '') + ' title="' + title + '">' + inner + '</a>';
}

// Collapsed-list row: path, branch, last git activity, mark, removal hint.
// merged === null (unknown) shows no mark.
function fleetWtRow(wt, now) {
  var mark = wt.merged === true ? '<span class="fl-wt-mark fl-ok" title="PR merged">✓ merged</span>'
    : wt.stale ? '<span class="fl-wt-mark fl-dim" title="no writer or git activity for over 24h">◌ stale</span>' : '';
  return '<div class="fl-wt-row">'
    + '<span class="fl-repo">' + fleetEsc(String(wt.repo || '').split('/').pop()) + '</span>'
    + '<span class="fl-wt-path" title="' + fleetEsc(wt.path_rel || '') + '">' + fleetEsc(wt.path_rel || '') + '</span>'
    + fleetBranchChip(wt.branch)
    + '<span class="fl-dim">git ' + (wt.last_git_at ? fleetAgo(wt.last_git_at, now, ' ago') : '—') + '</span>'
    + mark
    + (wt.stale || wt.merged === true ? fleetWtHint(wt) : '')
    + '</div>';
}

function fleetWorktrees(worktrees, now) {
  if (!Array.isArray(worktrees)) return '';
  var list = worktrees.filter(Boolean);
  if (!list.length) return '';
  var active = list.filter(function (w) { return w.active; });
  var rest = list.filter(function (w) { return !w.active; });
  var stale = rest.filter(function (w) { return w.stale; }).length;
  var merged = rest.filter(function (w) { return w.merged === true; }).length;
  var strip = '<div class="fl-strip"><span class="fl-strip-label">worktrees in flight</span>'
    + (active.length ? active.map(function (w) { return fleetWtChip(w, now); }).join('') : '<span class="fl-dim">none being written to</span>')
    + '</div>';
  if (!rest.length) return strip;
  var rows = rest.slice().sort(function (a, b) { return (Number(b.last_git_at) || 0) - (Number(a.last_git_at) || 0); })
    .map(function (w) { return fleetWtRow(w, now); }).join('');
  return strip + '<details class="fl-wt-tail" data-key="worktrees"><summary>'
    + fleetEsc(rest.length + ' idle · ' + stale + ' stale · ' + merged + ' merged') + '</summary>' + rows + '</details>';
}

// worktrees when the board has them, else the older branch strip.
function fleetInFlight(board, now) {
  var b = board || {};
  return Array.isArray(b.worktrees) ? fleetWorktrees(b.worktrees, now) : fleetBranches(b.branches);
}

module.exports = { fleetWtWriters, fleetWtName, fleetWtHint, fleetWtChip, fleetWtRow, fleetWorktrees, fleetInFlight };
