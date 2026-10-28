// Render helpers for the fleet page: sessions, cards, agents, branches and the counts strip.
//
// PURE and dependency-free, in the template-builds-render.js idiom: each
// function is unit-tested here and injected into the page by toString() in
// page.js, so nothing below may reference a module-level binding
// other than the functions injected alongside it. Every string from the board
// goes through fleetEsc; every href through fleetUrl.

const { fleetEsc, fleetAge, fleetAgo } = require('./fleet-render-base');
const { fleetSortSessions, fleetOrder, fleetSortBy } = require('./fleet-render-order');
const { fleetCostChip } = require('./fleet-render-cost');
const { fleetWakeBadge, fleetWakeSuffix } = require('./fleet-render-wake');
const { fleetServersLine, fleetServersCount } = require('./fleet-render-servers');
const { fleetMsgList } = require('./fleet-render-msgs');
// Injected into the page first, in this order, by template-fleet.js.
const { fleetActivity, fleetAgentCurrent, fleetLifeline, fleetRecent, fleetRecentBadge, fleetHandbackChip } = require('./fleet-render-activity');

// Only http(s) links leave the page; anything else renders as plain text.
function fleetUrl(u) {
  return /^https?:\/\//i.test(String(u || '')) ? fleetEsc(u) : '';
}

// State -> label and class. approval/question are red because the session is
// blocked on a person; waiting is amber because it is the person's move but
// nothing is stuck.
function fleetState(state) {
  var m = {
    working: { label: 'WORKING', cls: 'fl-st-working', glyph: '◐' },
    delegating: { label: 'DELEGATING', cls: 'fl-st-working', glyph: '◐' },
    waiting: { label: 'WAITING ON YOU', cls: 'fl-st-waiting', glyph: '●' },
    approval: { label: 'NEEDS APPROVAL', cls: 'fl-st-approval', glyph: '▲' },
    question: { label: 'ASKING', cls: 'fl-st-approval', glyph: '?' },
    idle: { label: 'IDLE', cls: 'fl-st-idle', glyph: '○' },
    starting: { label: 'STARTING', cls: 'fl-st-starting', glyph: '◌' },
    ended: { label: 'ENDED', cls: 'fl-st-ended', glyph: '■' },
  };
  return m[state] || { label: fleetEsc(String(state || 'unknown').toUpperCase()), cls: 'fl-st-idle', glyph: '·' };
}

// A stable colour per branch name, so the same branch reads the same in every
// card and in the branches strip. Trunk branches stay neutral.
function fleetBranchHue(branch) {
  var b = String(branch || '');
  if (b === '' || b === 'main' || b === 'master' || b === 'dev' || b === 'uat') return -1;
  var h = 0;
  for (var i = 0; i < b.length; i++) h = (h * 31 + b.charCodeAt(i)) % 360;
  return h;
}

function fleetBranchChip(branch) {
  if (!branch) return '';
  var hue = fleetBranchHue(branch);
  var style = hue < 0 ? '' : ' style="--br-h:' + fleetEsc(Number(hue) || 0) + '"';
  return '<span class="fl-branch' + (hue < 0 ? ' fl-branch-trunk' : '') + '"' + style + ' title="branch">⎇ ' + fleetEsc(branch) + '</span>';
}

function fleetCtxBar(ctx, small) {
  if (!ctx || ctx.source === 'none' || ctx.pct == null) {
    return '<div class="fl-ctx' + (small ? ' fl-ctx-sm' : '') + '"><span class="fl-ctx-label">ctx —</span></div>';
  }
  var pct = Math.max(0, Math.min(100, Number(ctx.pct) || 0));
  var cls = pct >= 85 ? 'fl-ctx-hot' : pct >= 60 ? 'fl-ctx-warm' : 'fl-ctx-ok';
  var tip = Number(ctx.tokens) ? (Math.round(Number(ctx.tokens) / 1000) || 0) + 'k / ' + (Math.round(Number(ctx.limit) / 1000) || 0) + 'k tokens' : 'from statusline';
  return '<div class="fl-ctx ' + cls + (small ? ' fl-ctx-sm' : '') + '" title="' + fleetEsc(tip) + '">'
    + '<span class="fl-ctx-label">ctx ' + fleetEsc(Math.round(pct)) + '%</span>'
    + '<span class="fl-ctx-track"><span class="fl-ctx-fill" style="width:' + fleetEsc(pct.toFixed(1)) + '%"></span></span></div>';
}

function fleetToolGlyph(status) {
  return status === 'running' ? '<span class="fl-spin">◐</span>' : status === 'error' ? '<span class="fl-err">✗</span>' : '<span class="fl-ok">✓</span>';
}

function fleetIssues(issues) {
  if (!issues || !issues.length) return '';
  return '<div class="fl-issues">' + issues.map(function (i) {
    var label = (i.is_pr ? 'PR ' : '') + '#' + i.number;
    var repo = i.repo ? '<span class="fl-issue-repo">' + fleetEsc(i.repo.split('/').pop()) + '</span>' : '';
    var title = i.title ? '<span class="fl-issue-title">' + fleetEsc(i.title) + '</span>' : '';
    var state = i.state ? '<span class="fl-issue-state fl-issue-' + fleetEsc(i.state) + '">' + fleetEsc(i.state) + '</span>' : '';
    var inner = repo + '<b>' + fleetEsc(label) + '</b>' + title + state;
    var href = fleetUrl(i.url);
    return href
      ? '<a class="fl-issue" href="' + href + '" target="_blank" rel="noopener" title="from ' + fleetEsc(i.source) + '">' + inner + '</a>'
      : '<span class="fl-issue" title="from ' + fleetEsc(i.source) + '">' + inner + '</span>';
  }).join('') + '</div>';
}

function fleetRepoLine(s) {
  var r = s.repo || {};
  var name = r.owner && r.name ? r.owner + '/' + r.name : (r.name || '');
  var parts = [];
  if (name) parts.push('<span class="fl-repo" title="' + fleetEsc(r.root || s.cwd || '') + '">' + fleetEsc(name) + '</span>');
  if (s.branch) parts.push(fleetBranchChip(s.branch));
  if (r.version) parts.push('<span class="fl-ver" title="project version (' + fleetEsc(r.version_source || '') + ')">v' + fleetEsc(String(r.version).replace(/^v/, '')) + '</span>');
  if (r.commit) parts.push('<span class="fl-commit" title="checked-out commit">@' + fleetEsc(r.commit) + '</span>');
  if (r.worktree) parts.push('<span class="fl-tag">worktree</span>');
  if (!parts.length && s.cwd) parts.push('<span class="fl-repo">' + fleetEsc(s.cwd) + '</span>');
  return parts.length ? '<div class="fl-repo-line">' + parts.join('') + '</div>' : '';
}

function fleetAgent(a, now) {
  var st = { running: ['◐', 'fl-ag-running', 'running'], done: ['✓', 'fl-ag-done', 'done'], error: ['✗', 'fl-ag-error', 'error'], stalled: ['⚠', 'fl-ag-stalled', 'stalled'], parked: ['⏸', 'fl-ag-parked', 'waiting on its own background work'] }[a.status] || ['·', 'fl-ag-done', a.status];
  var cur = fleetAgentCurrent(a.current, a.current ? fleetToolGlyph(a.current.status) : '');
  var recent = fleetRecent(a, now);
  var when = a.status === 'running' || a.status === 'stalled' ? fleetAgo(a.last_active || a.started_at, now) : fleetAgo(a.ended_at || a.last_active, now);
  return '<div class="fl-agent ' + st[1] + (recent ? ' fl-ag-recent' : '') + '"' + (a.id ? ' id="fl-ag-' + fleetEsc(a.id) + '"' : '') + '>'
    + '<div class="fl-ag-head"><b>' + fleetEsc(a.type) + '</b>' + (a.background ? '<span class="fl-tag">bg</span>' : '') + '<span class="fl-ag-st">' + st[0] + ' ' + fleetEsc(st[2]) + ' ' + when + '</span></div>'
    + (a.model ? '<div class="fl-ag-model">' + fleetEsc(a.model) + '</div>' : '')
    + (a.description ? '<div class="fl-ag-desc">' + fleetEsc(a.description) + '</div>' : '')
    + fleetRecentBadge(recent, now)
    + cur
    + fleetLifeline(a.events)
    + '<div class="fl-ag-foot"><span class="fl-dim">' + fleetEsc(Number(a.tool_count) || 0) + ' tools</span>' + fleetCtxBar(a.context, true) + '</div>'
    + '</div>';
}

// Accounts by key; session order lives in fleet-render-order.js.
function fleetSortAccounts(list) {
  return (list || []).slice().sort(function (a, b) {
    var ka = String(a.key || ''), kb = String(b.key || '');
    return ka < kb ? -1 : ka > kb ? 1 : 0;
  });
}

// A prompt as a one-line label: pasted_content blocks (closed or cut off),
// a leading /command token and runs of whitespace go; the rest is clipped.
function fleetClean(text, max) {
  var t = String(text == null ? '' : text).slice(0, 20000)
    .replace(/<pasted_content\b[^>]*>[\s\S]*?<\/pasted_content>/gi, ' ')
    .replace(/<pasted_content\b[\s\S]*$/i, ' ')
    .replace(/^\s*\/[\w:.-]+/, ' ')
    .replace(/\s+/g, ' ').trim();
  var lim = max || 120;
  return t.length > lim ? t.slice(0, lim - 1).trimEnd() + '…' : t;
}

// Title, else the cleaned first prompt, else repo and branch, else short id.
function fleetHeadline(s) {
  var r = s.repo || {};
  var h = fleetClean(s.title) || fleetClean(s.recap && s.recap.first_prompt);
  if (!h) h = [r.name, s.branch].filter(Boolean).join(' · ');
  return h || String(s.short || s.id || '');
}

// Recap text can arrive flattened with a markdown table inline. The table is
// detail; the prose before it is the point. Cut at the first "|" ahead of the
// separator row, unless nothing would be left.
function fleetProse(text) {
  var t = String(text == null ? '' : text);
  var m = /\|\s*:?-{3,}/.exec(t);
  if (!m) return t;
  var cut = t.slice(0, m.index).indexOf('|');
  var head = (cut < 0 ? t.slice(0, m.index) : t.slice(0, cut)).trim();
  return head ? head + '…' : t;
}

// What the session is doing: the out-of-session recap, else Claude's last
// reply. A session waiting on a person also gets the reply under a recap.
function fleetDescription(s, now) {
  var rc = s.recap || {};
  var attn = s.state === 'waiting' || s.state === 'approval' || s.state === 'question';
  var out = '';
  if (rc.summary) {
    out += '<div class="fl-desc fl-summary"><span class="fl-who fl-who-sum" title="written outside the session by the fleet\'s recapper">recap</span>' + fleetEsc(fleetProse(rc.summary)) + ' ' + fleetAgo(rc.summary_at, now) + '</div>';
    if (attn && rc.last_reply) out += '<div class="fl-desc fl-desc-attn"><span class="fl-who fl-who-cc">last reply</span>' + fleetEsc(fleetProse(rc.last_reply)) + '</div>';
  } else if (rc.last_reply) {
    out += '<div class="fl-desc' + (attn ? ' fl-desc-attn' : '') + '"><span class="fl-who fl-who-cc">last reply</span>' + fleetEsc(fleetProse(rc.last_reply)) + '</div>';
  }
  return out;
}

function fleetPlural(n, one, many) {
  return n + ' ' + (n === 1 ? one : many);
}

function fleetToolRow(t, now) {
  return '<div class="fl-tool">' + fleetToolGlyph(t.status) + '<b>' + fleetEsc(t.name) + '</b><span class="fl-dim">' + fleetEsc(t.target || '') + '</span>' + fleetAgo(t.at, now) + '</div>';
}

// Finished work, collapsed: the last five finished tools, subagents that are
// done or failed, and the last three messages. Closed by default; the page
// keeps it open across re-renders by data-key.
function fleetHistory(s, now) {
  var tools = (s.tools || []).filter(function (t) { return t.status !== 'running'; }).slice(-5).reverse();
  var agents = (s.agents || []).filter(function (a) { return a.status === 'done' || a.status === 'error'; });
  var msgs = (s.messages || []).slice(-3);
  var totals = s.agent_totals || {};
  var total = Number(totals.total) > 0 ? Number(totals.total) : 0;
  if (!tools.length && !agents.length && !msgs.length && !total) return '';
  var done = agents.filter(function (a) { return a.status === 'done'; }).length;
  var failed = agents.length - done;
  // Each part is HTML: text parts are escaped, the ago part is the ticking span.
  var label = [fleetEsc('history')];
  if (tools.length) label.push(fleetEsc(fleetPlural(tools.length, 'tool', 'tools')));
  // The boxes below are the agents the server still holds (finished within 15 minutes); the
  // recency hint only appears when none are held, so the label never carries both.
  if (total) label.push(fleetEsc(total + ' run in total'));
  if (total && agents.length) label.push(fleetEsc(agents.length + ' finished in the last 15m'));
  else if (total && totals.last_finished_at) label.push('last finished ' + fleetAgo(totals.last_finished_at, now, ' ago'));
  // The lifetime total already counts finished agents, so "N agents done" only shows without it.
  if (done && !total) label.push(fleetEsc(fleetPlural(done, 'agent', 'agents') + ' done'));
  if (failed) label.push(fleetEsc(failed + ' failed'));
  if (msgs.length) label.push(fleetEsc(fleetPlural(msgs.length, 'message', 'messages')));
  var msgRows = msgs.map(function (m) {
    var to = m.external ? fleetEsc(m.to_label) : '<a href="#fl-' + (m.to_agent ? 'ag-' + fleetEsc(m.to_agent) : fleetEsc(m.to)) + '" class="fl-msg-to" data-target="' + fleetEsc(m.to) + '"'
      + (m.to_agent ? ' data-agent="' + fleetEsc(m.to_agent) + '"' : '') + '>' + fleetEsc(m.to_label) + '</a>';
    return '<div class="fl-msg">→ ' + to + ' <span class="fl-dim">' + fleetEsc(m.preview || '') + '</span> ' + fleetAgo(m.at, now) + '</div>';
  }).join('');
  return '<details class="fl-history" data-key="' + fleetEsc(s.id) + ':history"><summary>' + label.join(' · ') + '</summary>'
    + (tools.length ? '<div class="fl-tools">' + tools.map(function (t) { return fleetToolRow(t, now); }).join('') + '</div>' : '')
    + (msgRows ? '<div class="fl-msgs">' + msgRows + '</div>' : '')
    + (agents.length ? '<div class="fl-agents">' + agents.map(function (a) { return fleetAgent(a, now); }).join('') + '</div>' : '')
    + '</details>';
}

// board is optional; with it the card lists its inter-session messages (narrow screens).
function fleetSession(s, now, board) {
  var st = fleetState(s.state);
  var since = s.state_since ? ' ' + fleetAgo(s.state_since, now) : '';
  var foot = [fleetEsc(s.short), s.slot ? 'slot ' + fleetEsc(s.slot) : 'default', fleetEsc(s.pane || ''), s.pid ? 'pid ' + fleetEsc(s.pid) : '',
    s.model ? fleetEsc(String(s.model).replace(/^claude-/, '')) : '', s.claude_version ? 'cc ' + fleetEsc(s.claude_version) : ''].filter(Boolean).join(' · ');
  // The activity block already shows the session's current tool; drop its twin from the list.
  var cur = s.current;
  function isCurrent(t) { return !!cur && t.name === cur.name && (t.at === cur.at || (t.target || '') === (cur.target || '')); }
  var running = (s.tools || []).filter(function (t) { return t.status === 'running' && !isCurrent(t); }).map(function (t) { return fleetToolRow(t, now); }).join('');
  var agents = (s.agents || []);
  var active = agents.filter(function (a) { return a.status !== 'done' && a.status !== 'error'; });
  var nRunning = agents.filter(function (a) { return a.status === 'running'; }).length;
  var agentBlock = active.length
    ? '<div class="fl-delegate"><span>delegate to subagents</span><span class="fl-dim">' + fleetEsc(nRunning) + ' running · ' + fleetEsc(active.length) + ' active</span></div><div class="fl-agents">' + active.map(function (a) { return fleetAgent(a, now); }).join('') + '</div>'
    : '';
  var link = s.link === 'guess' ? '<span class="fl-tag" title="matched by recency, not by pid">guessed link</span>' : '';
  return '<section class="fl-card ' + st.cls + (s.live ? '' : ' fl-card-ended') + '" id="fl-' + fleetEsc(s.id) + '" data-id="' + fleetEsc(s.id) + '">'
    + '<header class="fl-card-head"><div class="fl-headline">' + fleetEsc(fleetHeadline(s)) + '</div>'
    + '<span class="fl-pill ' + st.cls + '">' + st.glyph + ' ' + st.label + since + '</span>' + fleetWakeBadge(s, now) + fleetHandbackChip(s.agents, now) + link + '</header>'
    + fleetDescription(s, now)
    + fleetActivity(s.current, now)
    + fleetRepoLine(s)
    + fleetServersLine(s, now)
    + fleetIssues(s.issues)
    + (running ? '<div class="fl-tools">' + running + '</div>' : '')
    + '<div class="fl-meta-row">' + fleetCtxBar(s.context) + fleetCostChip(s.cost) + '</div>'
    + (board ? fleetMsgList(s, board, now) : '')
    + agentBlock
    + fleetHistory(s, now)
    + '<footer class="fl-foot">' + foot + '</footer>'
    + '</section>';
}

// Each item links to the first session on the branch; data-sessions lists
// every session so the click handler can highlight them all.
function fleetBranches(branches) {
  if (!branches || !branches.length) return '';
  return '<div class="fl-strip"><span class="fl-strip-label">branches in flight</span>' + branches.map(function (b) {
    var ids = Array.isArray(b.sessions) ? b.sessions.filter(Boolean) : [];
    var cls = 'fl-strip-item' + (b.shared ? ' fl-shared' : '');
    var title = b.shared ? 'more than one live session on this branch' : 'jump to the session';
    var inner = '<span class="fl-repo">' + fleetEsc(String(b.repo || '').split('/').pop()) + '</span>'
      + fleetBranchChip(b.branch)
      + (b.version ? '<span class="fl-ver">v' + fleetEsc(String(b.version).replace(/^v/, '')) + '</span>' : '')
      + (b.commit ? '<span class="fl-commit">@' + fleetEsc(b.commit) + '</span>' : '')
      + '<span class="fl-dim">×' + fleetEsc(ids.length) + '</span>'
      + (b.shared ? '<span class="fl-warn">⚠ shared</span>' : '');
    return ids.length
      ? '<a class="' + cls + '" href="#fl-' + fleetEsc(ids[0]) + '" data-sessions="' + fleetEsc(ids.join(' ')) + '" title="' + title + '">' + inner + '</a>'
      : '<span class="' + cls + '" title="' + title + '">' + inner + '</span>';
  }).join('') + '</div>';
}

function fleetCounts(c) {
  c = c || {};
  function chip(n, label, cls) {
    return '<span class="fl-count ' + cls + '"><b>' + fleetEsc(Number(n) || 0) + '</b> ' + label + '</span>';
  }
  return chip(c.live, 'live', '') + chip(c.approval, 'blocked', 'fl-st-approval') + chip(c.waiting, 'waiting on you', 'fl-st-waiting')
    + chip(c.working, 'working', 'fl-st-working') + chip(c.agents_running, 'subagents', 'fl-st-starting') + fleetServersCount(c);
}

// One compact chip per live session, in slot order, so a card can be reached
// from the sticky header. Same #fl-<id> anchor the click handler already uses.
function fleetSlots(board, now) {
  var live = ((board && board.sessions) || []).filter(function (s) { return s.live; });
  function num(s) { var n = parseInt(s.slot, 10); return isFinite(n) ? n : 1e9; }
  live.sort(function (a, b) {
    var d = num(a) - num(b);
    if (d) return d;
    var ia = String(a.id || ''), ib = String(b.id || '');
    return ia < ib ? -1 : ia > ib ? 1 : 0;
  });
  return live.map(function (s) {
    var st = fleetState(s.state);
    var label = s.slot ? String(s.slot) : 'def';
    return '<a class="fl-slot ' + st.cls + '" href="#fl-' + fleetEsc(s.id) + '" title="' + fleetEsc(fleetHeadline(s) + ' · ' + st.label.toLowerCase()) + '">' + fleetEsc(label) + fleetWakeSuffix(s, now) + '</a>';
  }).join('');
}

// filter: all | attention (blocked or waiting) | working; showEnded toggles
// sessions whose process has gone.
function fleetVisible(s, filter, showEnded) {
  if (!s.live && !showEnded) return false;
  if (filter === 'attention') return s.state === 'approval' || s.state === 'question' || s.state === 'waiting';
  if (filter === 'working') return s.state === 'working' || s.state === 'delegating' || (s.agents || []).some(function (a) { return a.status === 'running'; });
  return true;
}

function fleetAccounts(board, now, filter, showEnded, order) {
  var sessions = fleetSortBy(board.sessions, order).filter(function (s) { return fleetVisible(s, filter, showEnded); });
  if (!sessions.length) {
    return '<div class="fl-empty">' + ((board.sessions || []).length ? 'No sessions match this filter.' : 'No Claude sessions on ' + fleetEsc(board.host || 'the host') + ' right now.') + '</div>';
  }
  return fleetSortAccounts(board.accounts).map(function (a) {
    var mine = sessions.filter(function (s) { return s.account_key === a.key; });
    if (!mine.length) return '';
    var who = a.email || 'unknown account';
    return '<div class="fl-account">'
      + '<div class="fl-account-head"><span class="fl-account-name">' + fleetEsc(who) + '</span>'
      + (a.org ? '<span class="fl-dim">' + fleetEsc(a.org) + '</span>' : '')
      + '<span class="fl-dim">slots ' + fleetEsc((a.slots || []).join(', ')) + '</span>'
      + '<span class="fl-dim">' + fleetEsc(Number(a.live) || 0) + ' live</span></div>'
      + '<div class="fl-grid">' + mine.map(function (s) { return fleetSession(s, now, board); }).join('') + '</div>'
      + '</div>';
  }).join('');
}

function fleetLog(log, now) {
  if (!log || !log.length) return '<div class="fl-dim">No activity yet.</div>';
  return log.slice(-40).reverse().map(function (l) {
    var d = new Date((Number(l.at) || 0) * 1000);
    var hh = ('0' + d.getHours()).slice(-2) + ':' + ('0' + d.getMinutes()).slice(-2) + ':' + ('0' + d.getSeconds()).slice(-2);
    return '<div class="fl-log-row fl-lk-' + fleetEsc(l.kind) + '"><span class="fl-dim">' + hh + '</span>'
      + '<a href="#fl-' + fleetEsc(l.session) + '" class="fl-log-sess">' + fleetEsc(l.short) + '</a>'
      + '<span class="fl-log-agent">' + (l.agent ? '↳ ' + fleetEsc(l.agent) : '') + '</span>'
      + '<span class="fl-log-text">' + fleetEsc(l.text) + '</span></div>';
  }).join('');
}

module.exports = {
  fleetEsc, fleetUrl, fleetAge, fleetAgo, fleetState, fleetBranchHue, fleetBranchChip, fleetCtxBar,
  fleetToolGlyph, fleetIssues, fleetRepoLine, fleetAgent, fleetSortSessions, fleetOrder, fleetSortBy, fleetSortAccounts, fleetClean,
  fleetHeadline, fleetProse, fleetDescription, fleetPlural, fleetToolRow, fleetHistory, fleetSession, fleetBranches, fleetCounts,
  fleetVisible, fleetAccounts, fleetLog, fleetSlots,
};
