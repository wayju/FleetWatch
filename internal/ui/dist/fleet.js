function fleetEsc(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}
function fleetUrl(u) {
  return /^https?:\/\//i.test(String(u || '')) ? fleetEsc(u) : '';
}
function fleetAge(sec) {
  if (sec == null || !isFinite(sec) || sec < 0) return '';
  if (sec < 5) return 'now';
  if (sec < 60) return Math.floor(sec) + 's';
  if (sec < 3600) return Math.floor(sec / 60) + 'm';
  if (sec < 86400) return Math.floor(sec / 3600) + 'h';
  return Math.floor(sec / 86400) + 'd';
}
function fleetAgeSfx(sec, sfx) {
  var a = fleetAge(sec);
  if (!sfx || !a) return a;
  return a === 'now' ? 'just now' : a + sfx;
}
function fleetAgo(t, now, sfx) {
  if (!t) return '';
  return '<span class="fl-ago" data-t="' + fleetEsc(Number(t) || 0) + '"' + (sfx ? ' data-sfx="' + fleetEsc(sfx) + '"' : '') + '>' + fleetAgeSfx(now - t, sfx) + '</span>';
}
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
function fleetClock(at) {
  var d = new Date((Number(at) || 0) * 1000);
  function p(n) { return ('0' + n).slice(-2); }
  return p(d.getHours()) + ':' + p(d.getMinutes()) + ':' + p(d.getSeconds());
}
function fleetActivity(cur, now) {
  if (!cur || typeof cur !== 'object') return '';
  var text = cur.description || cur.name;
  if (!text) return '';
  return '<div class="fl-act"><div class="fl-act-main"><span class="fl-spin">◐</span><span class="fl-act-text">' + fleetEsc(text) + '</span>'
    + (cur.at ? '<span class="fl-act-sep">·</span>' + fleetAgo(cur.at, now) : '') + '</div>'
    + (cur.target ? '<div class="fl-act-cmd fl-dim">' + fleetEsc(cur.target) + '</div>' : '')
    + '</div>';
}
function fleetAgentCurrent(cur, glyph) {
  if (!cur) return '';
  if (cur.description) {
    return '<div class="fl-ag-cur fl-ag-cur-d">' + glyph + ' ' + fleetEsc(cur.description) + '</div>'
      + (cur.target ? '<div class="fl-ag-cmd fl-dim">' + fleetEsc(cur.target) + '</div>' : '');
  }
  return '<div class="fl-ag-cur">' + glyph + ' ' + fleetEsc(cur.name) + (cur.target ? ' <span class="fl-dim">' + fleetEsc(cur.target) + '</span>' : '') + '</div>';
}
function fleetLifeline(events) {
  if (!Array.isArray(events) || !events.length) return '';
  var g = { launch: '▶', to: '→', from: '←', handback: '↩', done: '✓', error: '✗' };
  return '<div class="fl-life">' + events.map(function (e) {
    if (!e) return '';
    var k = String(e.kind || '');
    return '<span class="fl-lf fl-lf-' + fleetEsc(g[k] ? k : 'other') + '" title="' + fleetEsc(k + ' ' + fleetClock(e.at)) + '">' + (g[k] || '·') + '</span>';
  }).join('') + '</div>';
}
function fleetRecent(a, now) {
  var best = null;
  function take(kind, at) {
    at = Number(at) || 0;
    if (!at || now - at > 120 || now - at < -5) return;
    if (!best || at > best.at || (at === best.at && kind === 'handback')) best = { kind: kind, at: at };
  }
  take('handback', a && a.handed_back_at);
  ((a && a.events) || []).forEach(function (e) {
    if (e && e.kind === 'from') take('from', e.at);
    if (e && e.kind === 'handback') take('handback', e.at);
  });
  return best;
}
function fleetRecentBadge(recent, now) {
  if (!recent) return '';
  return '<span class="fl-ag-badge">' + (recent.kind === 'handback' ? '↩ handed back ' : '← message ') + fleetAgo(recent.at, now, ' ago') + '</span>';
}
function fleetHandbackChip(agents, now) {
  var n = (agents || []).filter(function (a) {
    var t = Number(a && a.handed_back_at) || 0;
    return t && now - t <= 300 && now - t >= -5;
  }).length;
  if (!n) return '';
  return '<span class="fl-hb-chip" title="subagents that returned control in the last 5 minutes">↩ ' + fleetEsc(n) + (n === 1 ? ' hand-back' : ' hand-backs') + ' (5m)</span>';
}
function fleetUsd(n) {
  var v = Number(n);
  if (!isFinite(v) || v < 0) v = 0;
  if (v < 10) return '$' + v.toFixed(2);
  return '$' + String(Math.round(v)).replace(/\B(?=(\d{3})+(?!\d))/g, ',');
}
function fleetTok(n) {
  var v = Number(n);
  if (!isFinite(v) || v < 0) v = 0;
  function f(x, sfx) { return (x >= 100 ? String(Math.round(x)) : String(Math.round(x * 10) / 10)) + sfx; }
  if (v >= 1e9) return f(v / 1e9, 'B');
  if (v >= 1e6) return f(v / 1e6, 'M');
  if (v >= 1e3) return f(v / 1e3, 'k');
  return String(Math.round(v));
}
function fleetModelClass(entry) {
  var mc = entry && typeof entry === 'object' ? String(entry.model_class || '').toLowerCase() : '';
  return /^[a-z0-9][a-z0-9_-]{0,31}$/.test(mc) ? 'fl-mc-' + mc : 'fl-mc-other';
}
function fleetCostBar(byModel) {
  var rows = (Array.isArray(byModel) ? byModel : []).filter(function (m) { return Number(m && m.usd) > 0; });
  var total = rows.reduce(function (t, m) { return t + Number(m.usd); }, 0);
  if (!total) return '';
  return '<span class="fl-cost-bar">' + rows.map(function (m) {
    var w = Math.round(Number(m.usd) / total * 1000) / 10;
    return '<i class="' + fleetModelClass(m) + '" style="width:' + fleetEsc(w) + '%"></i>';
  }).join('') + '</span>';
}
function fleetCostTokens(t) {
  t = t || {};
  return 'in ' + fleetTok(t.input) + ' · write ' + fleetTok((Number(t.cache_write_5m) || 0) + (Number(t.cache_write_1h) || 0))
    + ' · read ' + fleetTok(t.cache_read) + ' · out ' + fleetTok(t.output);
}
function fleetCostTip(cost) {
  var rows = (cost && Array.isArray(cost.by_model)) ? cost.by_model : [];
  var lines = rows.map(function (m) {
    return String(m.model || 'unknown') + ' · ' + (Number(m.messages) || 0) + ' msgs · ' + (m.unpriced ? 'unpriced' : fleetUsd(m.usd)) + ' · ' + fleetCostTokens(m.tokens);
  });
  lines.push('API list-price equivalent, not billed (plan)');
  return lines.join('\n');
}
function fleetCostChip(cost) {
  if (!cost || typeof cost !== 'object') return '';
  var label = cost.partial ? '≈ … counting' : '≈ ' + fleetUsd(cost.usd) + ' API';
  return '<span class="fl-cost" title="' + fleetEsc(fleetCostTip(cost)) + '"><span class="fl-cost-chip' + (cost.partial ? ' fl-cost-partial' : '') + '">'
    + fleetEsc(label) + '</span>' + fleetCostBar(cost.by_model) + '</span>';
}
function fleetCostTotal(cost, open) {
  if (!cost || typeof cost !== 'object') return '';
  return '<button type="button" class="fl-count fl-cost-total' + (open ? ' active' : '') + '" id="fl-cost-toggle" aria-expanded="' + (open ? 'true' : 'false')
    + '" title="API list-price equivalent by model">≈ <b>' + fleetEsc(fleetUsd(cost.usd)) + '</b> API</button>';
}
function fleetCostPanel(cost, open) {
  if (!open || !cost || typeof cost !== 'object') return '';
  var rows = Array.isArray(cost.by_model) ? cost.by_model : [];
  return '<div class="fl-cost-panel" role="region" aria-label="cost by model">'
    + rows.map(function (m) {
      return '<div class="fl-cost-row"><i class="fl-cost-sw ' + fleetModelClass(m) + '"></i><span class="fl-cost-model">' + fleetEsc(m.model || 'unknown')
        + '</span><b>' + (m.unpriced ? 'unpriced' : fleetEsc(fleetUsd(m.usd))) + '</b><span class="fl-dim">' + fleetEsc(fleetCostTokens(m.tokens)) + '</span></div>';
    }).join('')
    + '<div class="fl-cost-foot">API list-price equivalent · prices as of ' + fleetEsc(cost.priced_as_of || 'unknown') + ' · not billed (plan)</div></div>';
}
function fleetSetKeepScroll(host, sel, html) {
  var old = host.querySelector(sel);
  var top = old ? old.scrollTop : 0;
  host.innerHTML = html;
  var fresh = host.querySelector(sel);
  if (fresh && top) fresh.scrollTop = top;
}
function fleetWakeOf(s) {
  var w = s && (s.state === 'working' || s.state === 'waiting' || s.state === 'idle') ? s.next_wake : null;
  if (!w) return null;
  if (w.kind === 'timer') return isFinite(Number(w.at)) && w.at != null ? w : null;
  return w.kind === 'condition' ? w : null;
}
function fleetWakeMins(at, now) {
  var left = Number(at) - Number(now);
  if (!isFinite(left) || left <= 0) return 0;
  return Math.max(1, Math.ceil(left / 60));
}
function fleetWakeText(at, now) {
  var m = fleetWakeMins(at, now);
  return m ? m + 'm' : 'due';
}
function fleetWakeChipText(at, now) {
  var m = fleetWakeMins(at, now);
  return '(' + (m ? m : 'due') + ')';
}
function fleetWakeBadge(s, now) {
  var w = fleetWakeOf(s);
  if (!w) return '';
  var tip = ' title="' + fleetEsc(w.reason || (w.kind === 'timer' ? 'scheduled wake-up' : 'waiting on a background task')) + '"';
  if (w.kind === 'timer') {
    return '<span class="fl-wake fl-wake-timer"' + tip + '>⏰ <span data-wake-at="' + fleetEsc(Number(w.at)) + '">' + fleetEsc(fleetWakeText(w.at, now)) + '</span></span>';
  }
  return '<span class="fl-wake fl-wake-cond"' + tip + '>⏳ on condition</span>';
}
function fleetWakeSuffix(s, now) {
  var w = fleetWakeOf(s);
  if (!w) return '';
  if (w.kind === 'condition') return ' (C)';
  return ' <span data-wake-at="' + fleetEsc(Number(w.at)) + '" data-wake-chip="1">' + fleetEsc(fleetWakeChipText(w.at, now)) + '</span>';
}
function fleetServersDur(sec) {
  sec = Number(sec);
  if (!isFinite(sec) || sec < 0) return '';
  var m = Math.floor(sec / 60), h = Math.floor(m / 60), d = Math.floor(h / 24);
  if (d) return d + 'd ' + (h % 24) + 'h';
  if (h) return h + 'h ' + (m % 60) + 'm';
  return m ? m + 'm' : '<1m';
}
function fleetServersOf(s) {
  var list = s && Array.isArray(s.servers) ? s.servers : [];
  return list.filter(function (v) {
    return v && Number.isInteger(Number(v.pid)) && Number(v.pid) > 0 && Number.isInteger(Number(v.port)) && Number(v.port) > 0;
  }).sort(function (a, b) { return (Number(a.started_at) || 0) - (Number(b.started_at) || 0); });
}
function fleetServersWhy(reason) {
  var why = { age: 'older than 12 h', merged: 'branch merged', dir_gone: 'worktree removed' };
  return why[reason] || 'stale';
}
function fleetServerRow(v, now) {
  var cls = 'fl-srv-row' + (v.stale ? ' fl-srv-stale' : '');
  var age = fleetServersDur(now - Number(v.started_at));
  return '<div class="' + cls + '"><div><b>:' + fleetEsc(Number(v.port)) + '</b> pid ' + fleetEsc(Number(v.pid))
    + (age ? ' · up ' + fleetEsc(age) : '') + (v.stale ? ' · <span class="fl-srv-why">' + fleetEsc(fleetServersWhy(v.stale_reason)) + '</span>' : '') + '</div>'
    + (v.cmd ? '<div class="fl-srv-cmd">' + fleetEsc(v.cmd) + '</div>' : '')
    + (v.dir ? '<div class="fl-srv-dir">' + fleetEsc(v.dir) + '</div>' : '')
    + '<code class="fl-srv-kill">kill ' + fleetEsc(Number(v.pid)) + '</code></div>';
}
function fleetServersLine(s, now) {
  var list = fleetServersOf(s);
  if (!list.length) return '';
  var stale = list.filter(function (v) { return v.stale; }).length;
  var ports = list.map(function (v) {
    return '<span class="fl-srv-port' + (v.stale ? ' fl-srv-stale' : '') + '">:' + fleetEsc(Number(v.port)) + '</span>';
  }).join(' ');
  var oldest = fleetServersDur(now - (Number(list[0].started_at) || now));
  return '<details class="fl-servers" data-key="' + fleetEsc(s.id) + ':servers"><summary title="agent-started servers still listening; open for the command and a kill line">'
    + '<span class="fl-srv-n">' + fleetEsc(list.length) + (list.length === 1 ? ' server' : ' servers') + '</span> · ' + ports
    + (oldest ? ' · oldest ' + fleetEsc(oldest) : '')
    + (stale ? ' · <span class="fl-srv-stale">' + fleetEsc(stale) + ' stale</span>' : '')
    + '</summary>' + list.map(function (v) { return fleetServerRow(v, now); }).join('') + '</details>';
}
function fleetServersCount(c) {
  var n = Number(c && c.servers) || 0, st = Number(c && c.servers_stale) || 0;
  if (n <= 0) return '';
  return '<span class="fl-count fl-srv-count"><b>' + fleetEsc(n) + '</b> ' + (n === 1 ? 'server' : 'servers') + '</span>'
    + (st > 0 ? '<span class="fl-count fl-srv-count fl-srv-stale"><b>' + fleetEsc(st) + '</b> stale</span>' : '');
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
function fleetSortSessions(list) {
  return (list || []).slice().sort(function (a, b) {
    var ta = Number(a.started_at) || 0, tb = Number(b.started_at) || 0;
    if (ta && tb && ta !== tb) return ta - tb;
    if (ta && !tb) return -1;
    if (!ta && tb) return 1;
    var ia = String(a.id || ''), ib = String(b.id || '');
    return ia < ib ? -1 : ia > ib ? 1 : 0;
  });
}
function fleetOrder(v) {
  return v === 'started' || v === 'attention' || v === 'active' || v === 'cost' ? v : 'slot';
}
function fleetSortBy(list, order) {
  var o = fleetOrder(order);
  var base = fleetSortSessions(list);
  if (o === 'started') return base;
  var rank = {};
  base.forEach(function (s, i) { rank[s.id] = i; });
  function slotNum(s) { var n = parseInt(s.slot, 10); return isFinite(n) ? n : 1e9; }
  function attn(s) {
    var st = s.state;
    if (!s.live) return 5;
    if (st === 'approval' || st === 'question') return 0;
    if (st === 'waiting') return 1;
    if (st === 'delegating' || st === 'working' || st === 'starting') return 2;
    if (st === 'idle') return 3;
    return 4;
  }
  function key(s) {
    if (o === 'cost') return -(Number(s.cost && s.cost.usd) || 0);
    if (o === 'active') return -(Number(s.last_activity) || 0);
    if (o === 'attention') return attn(s);
    return slotNum(s);
  }
  return base.slice().sort(function (a, b) {
    var d = key(a) - key(b);
    if (!d && (o === 'attention' || o === 'cost')) d = slotNum(a) - slotNum(b);
    return d || rank[a.id] - rank[b.id];
  });
}
function fleetSortAccounts(list) {
  return (list || []).slice().sort(function (a, b) {
    var ka = String(a.key || ''), kb = String(b.key || '');
    return ka < kb ? -1 : ka > kb ? 1 : 0;
  });
}
function fleetClean(text, max) {
  var t = String(text == null ? '' : text).slice(0, 20000)
    .replace(/<pasted_content\b[^>]*>[\s\S]*?<\/pasted_content>/gi, ' ')
    .replace(/<pasted_content\b[\s\S]*$/i, ' ')
    .replace(/^\s*\/[\w:.-]+/, ' ')
    .replace(/\s+/g, ' ').trim();
  var lim = max || 120;
  return t.length > lim ? t.slice(0, lim - 1).trimEnd() + '…' : t;
}
function fleetHeadline(s) {
  var r = s.repo || {};
  var h = fleetClean(s.title) || fleetClean(s.recap && s.recap.first_prompt);
  if (!h) h = [r.name, s.branch].filter(Boolean).join(' · ');
  return h || String(s.short || s.id || '');
}
function fleetProse(text) {
  var t = String(text == null ? '' : text);
  var m = /\|\s*:?-{3,}/.exec(t);
  if (!m) return t;
  var cut = t.slice(0, m.index).indexOf('|');
  var head = (cut < 0 ? t.slice(0, m.index) : t.slice(0, cut)).trim();
  return head ? head + '…' : t;
}
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
function fleetMsgName(board, id) {
  var list = (board && board.sessions) || [];
  for (var i = 0; i < list.length; i++) {
    if (list[i].id === id) return list[i].slot ? 'slot ' + list[i].slot : String(list[i].short || id);
  }
  return String(id || '');
}
function fleetMsgList(s, board, now) {
  var rows = ((board && board.links) || []).filter(function (l) {
    return (l.from === s.id || l.to === s.id) && now - l.at <= 1800;
  }).sort(function (a, b) { return b.at - a.at; }).slice(0, 5);
  if (!rows.length) return '';
  return '<div class="fl-xmsgs"><div class="fl-xmsgs-h">messages</div>' + rows.map(function (l) {
    var out = l.from === s.id;
    var other = out ? l.to : l.from;
    var label = l.external ? (l.to_label || other) : fleetMsgName(board, other);
    var who = l.external ? fleetEsc(label) : '<a href="#fl-' + fleetEsc(other) + '">' + fleetEsc(label) + '</a>';
    return '<div class="fl-xmsg"><span class="fl-xmsg-dir">' + (out ? '→ ' : '← ') + who + '</span>'
      + fleetAgo(l.at, now, ' ago') + '<span class="fl-xmsg-text">' + fleetEsc(l.preview || '') + '</span></div>';
  }).join('') + '</div>';
}
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
function fleetHasBox(el) {
  if (!el || el.getClientRects().length === 0) return false;
  if (el.checkVisibility) return el.checkVisibility();
  return !(el.closest && el.closest('details:not([open]) > :not(summary), details:not([open]) > :not(summary) *'));
}
function fleetLinkClass(l) {
  if (l.kind === 'handback') return 'fl-link-handback';
  return l.from_agent || l.to_agent ? 'fl-link-agent' : '';
}
function fleetLinkFresh(l, now) {
  return !l.external && now - l.at <= 30;
}
function fleetLinkSig(links, now) {
  var s = '';
  for (var i = 0; i < (links || []).length; i++) if (fleetLinkFresh(links[i], now)) s += i + ',';
  return s;
}
function fleetLinkCards(l, now, byId) {
  if (!fleetLinkFresh(l, now)) return null;
  var a = byId('fl-' + l.from), c = byId('fl-' + l.to);
  if (!a || !c || a === c || !fleetHasBox(a) || !fleetHasBox(c)) return null;
  return { src: a, dst: c, cls: fleetLinkClass(l) };
}
function fleetLinkLabel(board, l, now) {
  var from = fleetMsgName(board, l.from), to = fleetMsgName(board, l.to);
  var full = String(l.preview || '').replace(/\s+/g, ' ').trim();
  var clip = full.length > 36 ? full.slice(0, 35).replace(/\s+$/, '') + '\u2026' : full;
  var age = fleetAge(Math.max(0, now - l.at));
  var q = function (t) { return t ? ' \u00b7 \u201c' + t + '\u201d' : ''; };
  return { text: from + ' \u2192 ' + to + q(clip) + ' \u00b7 ' + age, full: from + ' \u2192 ' + to + q(full) + ' \u00b7 ' + age };
}
function fleetRouteGaps(edges, lo, hi, minGap) {
  var e = edges.concat([lo, hi]).filter(function (v) { return v >= lo && v <= hi; }).sort(function (a, b) { return a - b; });
  var out = [];
  for (var i = 0; i + 1 < e.length; i++) {
    if (e[i + 1] - e[i] >= minGap) out.push({ lo: e[i], hi: e[i + 1], mid: (e[i] + e[i + 1]) / 2, key: String(e[i]) });
  }
  return out;
}
function fleetRouteHit(p, q, cards) {
  var x1 = Math.min(p.x, q.x), x2 = Math.max(p.x, q.x), y1 = Math.min(p.y, q.y), y2 = Math.max(p.y, q.y);
  if (x2 - x1 < 0.01 && y2 - y1 < 0.01) return null;
  for (var i = 0; i < cards.length; i++) {
    var c = cards[i];
    if (x2 > c.x + 0.5 && x1 < c.x + c.w - 0.5 && y2 > c.y + 0.5 && y1 < c.y + c.h - 0.5) return c;
  }
  return null;
}
function fleetRouteShares(p, q, r, s, tol) {
  var ph = Math.abs(p.y - q.y) < 0.01, rh = Math.abs(r.y - s.y) < 0.01;
  if (ph && rh) return Math.abs(p.y - r.y) < tol && Math.min(Math.max(p.x, q.x), Math.max(r.x, s.x)) - Math.max(Math.min(p.x, q.x), Math.min(r.x, s.x)) > 1;
  var pv = Math.abs(p.x - q.x) < 0.01, rv = Math.abs(r.x - s.x) < 0.01;
  if (pv && rv) return Math.abs(p.x - r.x) < tol && Math.min(Math.max(p.y, q.y), Math.max(r.y, s.y)) - Math.max(Math.min(p.y, q.y), Math.min(r.y, s.y)) > 1;
  return false;
}
function fleetRouteLane(gap, n, step, half) {
  var lo = gap.lo + half, hi = gap.hi - half;
  var off = n === 0 ? 0 : (n % 2 === 1 ? 1 : -1) * Math.ceil(n / 2) * step;
  var pos = gap.mid + off;
  if (hi < lo) return n === 0 ? gap.mid : null;
  return pos < lo || pos > hi ? null : pos;
}
function fleetRoundPath(pts, r) {
  var d = 'M' + pts[0].x + ' ' + pts[0].y;
  for (var i = 1; i < pts.length - 1; i++) {
    var a = pts[i - 1], b = pts[i], c = pts[i + 1];
    var l1 = Math.abs(b.x - a.x) + Math.abs(b.y - a.y), l2 = Math.abs(c.x - b.x) + Math.abs(c.y - b.y);
    var k = Math.min(r, l1 / 2, l2 / 2);
    var ux = Math.sign(b.x - a.x), uy = Math.sign(b.y - a.y), vx = Math.sign(c.x - b.x), vy = Math.sign(c.y - b.y);
    d += ' L' + (b.x - ux * k) + ' ' + (b.y - uy * k) + ' Q' + b.x + ' ' + b.y + ' ' + (b.x + vx * k) + ' ' + (b.y + vy * k);
  }
  var e = pts[pts.length - 1];
  return d + ' L' + e.x + ' ' + e.y;
}
function fleetRouteRows(cards, b, o) {
  // Bands are non-card blocks (account headers, the log) that keep horizontal
  // lanes out without stopping a vertical one from passing.
  var blocks = cards.concat(o.bands || []);
  var gaps = fleetRouteGaps([].concat.apply([], blocks.map(function (c) { return [c.y, c.y + c.h]; })), b.y0, b.y1, o.minGap);
  // A usable gap is empty at every x and borders a card, so a pill never floats
  // over a header or the page margin.
  return gaps.filter(function (g) {
    var touches = cards.some(function (c) { return Math.abs(c.y + c.h - g.lo) < 0.5 || Math.abs(c.y - g.hi) < 0.5; });
    return touches && !blocks.some(function (c) { return c.y < g.hi - 0.01 && c.y + c.h > g.lo + 0.01; });
  }).map(function (g) {
    var lo = g.lo, hi = g.hi;
    if (hi - lo > o.rowBand) { if (lo <= b.y0 + 0.01) lo = hi - o.rowBand; else hi = lo + o.rowBand; }
    return { lo: lo, hi: hi, mid: (lo + hi) / 2, key: 'y' + g.key };
  });
}
function fleetRoutePorts(todo, byId, o) {
  function stack(side, other) {
    var groups = {};
    todo.forEach(function (t) { (groups[t.l[side]] = groups[t.l[side]] || []).push(t); });
    Object.keys(groups).forEach(function (id) {
      var c = byId[id];
      groups[id].sort(function (p, q) { return byId[p.l[other]].y - byId[q.l[other]].y || p.index - q.index; });
      groups[id].forEach(function (t, n) { t[side + 'Y'] = Math.min(c.y + o.portTop + n * o.portStep, c.y + c.h - 6); });
    });
  }
  stack('from', 'to');
  stack('to', 'from');
}
function fleetRouteSearch(ctx, S, E) {
  var o = ctx.o, used = ctx.used, best = null;
  var xa = ctx.xGaps.filter(function (g) { return g.lo >= S.x - 0.5; }).sort(function (p, q) { return Math.abs(p.mid - S.x) - Math.abs(q.mid - S.x); }).slice(0, 3);
  var xb = ctx.xGaps.filter(function (g) { return g.hi <= E.x + 0.5; }).sort(function (p, q) { return Math.abs(p.mid - E.x) - Math.abs(q.mid - E.x); }).slice(0, 3);
  function laneOf(gap, extra, step, half) { return fleetRouteLane(gap, (used[gap.key] || 0) + extra, step, half); }
  function clear(pts) {
    for (var k = 0; k + 1 < pts.length; k++) {
      if (fleetRouteHit(pts[k], pts[k + 1], ctx.cards)) return false;
      for (var m = 0; m < ctx.segs.length; m++) {
        if (fleetRouteShares(pts[k], pts[k + 1], ctx.segs[m][0], ctx.segs[m][1], 4)) return false;
      }
    }
    return true;
  }
  function length(pts) {
    var len = 0;
    for (var k = 0; k + 1 < pts.length; k++) len += Math.abs(pts[k + 1].x - pts[k].x) + Math.abs(pts[k + 1].y - pts[k].y);
    return len;
  }
  xa.forEach(function (ga) {
    var pa = laneOf(ga, 0, o.lane, o.clear);
    if (pa == null) return;
    ctx.yGaps.forEach(function (gy) {
      var py = laneOf(gy, 0, o.hlane, o.pillH / 2 + 1), roomy = gy.hi - gy.lo >= o.pillH + 2;
      if (py == null) { py = fleetRouteLane(gy, 0, o.hlane, 0); roomy = false; }
      if (py == null) return;
      xb.forEach(function (gb) {
        var pb = laneOf(gb, gb === ga ? 1 : 0, o.lane, o.clear);
        if (pb == null) return;
        var pts = [S, { x: pa, y: S.y }, { x: pa, y: py }, { x: pb, y: py }, { x: pb, y: E.y }, E];
        if (!clear(pts)) return;
        var cost = length(pts) + (roomy ? 0 : 300) + 6 * ((used[ga.key] || 0) + (used[gy.key] || 0) + (used[gb.key] || 0));
        if (!best || cost < best.cost) best = { cost: cost, pts: pts, gaps: [ga, gy, gb] };
      });
    });
  });
  if (best) return best;
  // No clear track (a very cramped board): the plain Z, flagged, so the link still shows.
  var xm = (S.x + E.x) / 2;
  return { pts: [S, { x: xm, y: S.y }, { x: xm, y: E.y }, E], gaps: [], fallback: true };
}
function fleetRoutePill(ctx, pts, S, E, text) {
  var o = ctx.o, b = ctx.b, run = null, k;
  for (k = 0; k + 1 < pts.length; k++) {
    var p = pts[k], q = pts[k + 1];
    if (Math.abs(p.y - q.y) < 0.01 && p.y !== S.y && p.y !== E.y && (!run || Math.abs(q.x - p.x) > run.len)) run = { y: p.y, cx: (p.x + q.x) / 2, len: Math.abs(q.x - p.x) };
  }
  if (!run) run = { y: (S.y + E.y) / 2, cx: (S.x + E.x) / 2, len: 0 };
  var w = Math.min(o.pillMax, Math.round((text && text.w ? text.w : String(text || '').length * o.charW) + o.pillPad)), h = o.pillH;
  function at(shift) { return { x: Math.max(b.x0 + 2, Math.min(b.x1 - w - 2, run.cx + shift - w / 2)), y: run.y - h / 2, w: w, h: h }; }
  for (var j = 0; j < 9; j++) {
    var cand = at(j === 0 ? 0 : (j % 2 ? 1 : -1) * Math.ceil(j / 2) * (w + 6));
    var clash = ctx.placed.some(function (r) { return cand.x < r.x + r.w && cand.x + cand.w > r.x && cand.y < r.y + r.h && cand.y + cand.h > r.y; });
    if (!clash) return cand;
  }
  return at(0);
}
function fleetRoute(cards, links, opts) {
  var o = Object.assign({ lane: 6, hlane: 10, radius: 9, portTop: 22, portStep: 10, pillH: 18, charW: 6.6, pillPad: 18, pillMax: 480, minGap: 14, clear: 3, rowBand: 60 }, opts || {});
  var byId = {};
  cards.forEach(function (c) { byId[c.id] = c; });
  var b = o.bounds || { x0: 0, y0: 0, x1: Math.max.apply(null, cards.map(function (c) { return c.x + c.w; })) + 24, y1: Math.max.apply(null, cards.map(function (c) { return c.y + c.h; })) + 24 };
  var ctx = {
    o: o, b: b, cards: cards, used: {}, segs: [], placed: [],
    xGaps: fleetRouteGaps([].concat.apply([], cards.map(function (c) { return [c.x, c.x + c.w]; })), b.x0, b.x1, o.minGap).map(function (g) { g.key = 'x' + g.key; return g; }),
    yGaps: fleetRouteRows(cards, b, o),
  };
  var todo = [];
  links.forEach(function (l, index) {
    if (l.from !== l.to && byId[l.from] && byId[l.to]) todo.push({ l: l, index: index });
  });
  fleetRoutePorts(todo, byId, o);
  return todo.map(function (t) {
    var src = byId[t.l.from], dst = byId[t.l.to];
    var S = { x: src.x + src.w, y: t.fromY }, E = { x: dst.x, y: t.toY };
    var best = fleetRouteSearch(ctx, S, E);
    best.gaps.forEach(function (g) { ctx.used[g.key] = (ctx.used[g.key] || 0) + 1; });
    var pts = best.pts.filter(function (p, k, a) { return k === 0 || Math.abs(p.x - a[k - 1].x) > 0.01 || Math.abs(p.y - a[k - 1].y) > 0.01; });
    for (var k = 0; k + 1 < pts.length; k++) ctx.segs.push([pts[k], pts[k + 1]]);
    var pill = fleetRoutePill(ctx, pts, S, E, t.l.labelW ? { w: t.l.labelW } : t.l.label);
    ctx.placed.push(pill);
    return { index: t.index, from: t.l.from, to: t.l.to, kind: t.l.kind, points: pts, d: fleetRoundPath(pts, o.radius), start: S, end: E, label: pill, fallback: !!best.fallback };
  });
}
function fleetWtWriters(wt) {
  var ws = Array.isArray(wt.writers) ? wt.writers.filter(Boolean) : [];
  return ws.slice().sort(function (a, b) { return (Number(b.last_at) || 0) - (Number(a.last_at) || 0); });
}
function fleetWtName(wt) {
  var p = String(wt.path_rel || '');
  var pre = '.claude/worktrees/';
  return p.indexOf(pre) === 0 ? p.slice(pre.length).replace(/\/+$/, '') : '';
}
function fleetWtHint(wt) {
  var name = fleetWtName(wt);
  // The hint is copied to a shell, so a name with anything unusual in it gets none.
  if (!name || !/^[A-Za-z0-9._\/-]+$/.test(name) || name.indexOf('..') >= 0) return '';
  var cmd = 'git worktree remove .claude/worktrees/' + name;
  return '<code class="fl-wt-cmd" data-copy="' + fleetEsc(cmd) + '" title="click to copy">' + fleetEsc(cmd) + '</code>';
}
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
function fleetInFlight(board, now) {
  var b = board || {};
  return Array.isArray(b.worktrees) ? fleetWorktrees(b.worktrees, now) : fleetBranches(b.branches);
}
var FL_URLS={stream:"/v1/events",snapshot:"/v1/fleet",session:"/v1/session"};
var FL={open:{},board:null,tickAt:0,filter:'all',showEnded:true,order:'slot',costOpen:false,es:null,down:null};
try{var saved=JSON.parse(localStorage.getItem('fleet-view')||'{}');if(saved.filter)FL.filter=saved.filter;if(saved.showEnded===false)FL.showEnded=false;FL.order=fleetOrder(saved.order)}catch(e){}
function flNow(){return Math.floor(Date.now()/1000)}
// The server's clock, from the last board: message ages are compared on it, so a
// viewer whose clock is a few seconds out still sees a 30-second line.
function flSrvNow(){return flNow()+(FL.skew||0)}
function flSave(){try{localStorage.setItem('fleet-view',JSON.stringify({filter:FL.filter,showEnded:FL.showEnded,order:FL.order}))}catch(e){}}
function flLive(){
  var el=document.getElementById('fl-live');if(!el)return;
  var age=FL.tickAt?flNow()-FL.tickAt:null;
  if(FL.down){el.className='fl-live down';el.innerHTML='<i></i>offline · '+fleetEsc(FL.down);return}
  if(age==null){el.className='fl-live';el.innerHTML='<i></i>connecting';return}
  if(age>15){el.className='fl-live stale';el.innerHTML='<i></i>stale '+fleetAge(age);return}
  el.className='fl-live on';el.innerHTML='<i></i>live';
}
// Keep a scrolled-to card clear of the sticky header, whatever height it wraps to.
function flTop(){var t=document.querySelector('.fl-top');if(t)document.documentElement.style.setProperty('--fl-top',(t.offsetHeight+8)+'px')}
function flRender(){
  var b=FL.board;if(!b)return;var now=flNow();
  fleetSetKeepScroll(document.getElementById('fl-counts'),'.fl-cost-panel',fleetCounts(b.counts)+(b.cost?'<span class="fl-cost-wrap">'+fleetCostTotal(b.cost,FL.costOpen)+fleetCostPanel(b.cost,FL.costOpen)+'</span>':''));
  document.getElementById('fl-slots').innerHTML=fleetSlots(b,now);
  flTop();
  document.getElementById('fl-branches').innerHTML=fleetInFlight(b,now);
  document.getElementById('fl-accounts').innerHTML=fleetAccounts(b,now,FL.filter,FL.showEnded,FL.order);
  document.getElementById('fl-log-body').innerHTML=fleetLog(b.log,now);
  document.querySelectorAll('[data-filter]').forEach(function(x){x.classList.toggle('active',x.dataset.filter===FL.filter)});
  document.getElementById('fl-order').value=FL.order;
  document.getElementById('fl-ended').classList.toggle('active',FL.showEnded);
  // innerHTML replaced every <details>, so put back the ones the viewer opened.
  document.querySelectorAll('details[data-key]').forEach(function(d){if(FL.open[d.dataset.key])d.open=true});
  flLinks();
}
// Arrows for messages sent in the last 30 seconds: output port on the sender's
// right edge, input port on the receiver's left, routed through the gutters by
// fleetRoute (pure, unit tested) and drawn here.
function flRects(main,base){
  var out=[],els=main.querySelectorAll('.fl-card');
  for(var i=0;i<els.length;i++){
    if(!fleetHasBox(els[i]))continue;
    var r=els[i].getBoundingClientRect();
    out.push({id:els[i].id,x:r.left-base.left,y:r.top-base.top,w:r.width,h:r.height});
  }
  return out;
}
function flRects2(main,base,sel){
  var out=[],els=main.querySelectorAll(sel);
  for(var i=0;i<els.length;i++){var r=els[i].getBoundingClientRect();out.push({x:r.left-base.left,y:r.top-base.top,w:r.width,h:r.height})}
  return out;
}
// Pill text width as the browser will draw it (monospace stack, 11px).
function flTextW(text){
  try{
    var c=FL.ctx||(FL.ctx=document.createElement('canvas').getContext('2d'));
    c.font='400 11px '+getComputedStyle(document.documentElement).getPropertyValue('--fl-mono');
    return c.measureText(text).width;
  }catch(e){return 0}
}
function flLinkSvg(r,l,lab,i){
  var mk=r.cls==='fl-link-handback'?'fl-arrow-handback':r.cls==='fl-link-agent'?'fl-arrow-agent':'fl-arrow';
  var at=' data-i="'+i+'" data-from="'+fleetEsc(r.from)+'" data-to="'+fleetEsc(r.to)+'" data-at="'+fleetEsc(l.at)+'"';
  var pill=r.label,cx=pill.x+pill.w/2,cy=pill.y+pill.h/2;
  return {
    line:'<g class="fl-link '+r.cls+'"'+at+'><path class="fl-hit" d="'+r.d+'"/><path class="fl-line" d="'+r.d+'" marker-end="url(#'+mk+')"/>'
      +'<circle class="fl-port" cx="'+r.start.x+'" cy="'+r.start.y+'" r="3.5"/><circle class="fl-port" cx="'+r.end.x+'" cy="'+r.end.y+'" r="3.5"/></g>',
    pill:'<g class="fl-link fl-pillg '+r.cls+'"'+at+'><rect x="'+pill.x+'" y="'+pill.y+'" width="'+pill.w+'" height="'+pill.h+'" rx="9"/>'
      +'<text x="'+cx+'" y="'+(cy+3.5)+'" text-anchor="middle">'+fleetEsc(lab.text)+'</text><title>'+fleetEsc(lab.full)+'</title></g>'
  };
}
function flLinks(){
  var svg=document.getElementById('fl-links'),main=document.querySelector('.fl-main');if(!svg||!FL.board)return;
  var now=flSrvNow();
  FL.linkSig=fleetLinkSig(FL.board.links,now);
  // Narrow screens show each card's own messages list instead (CSS hides the svg).
  if(window.matchMedia&&window.matchMedia('(max-width:700px)').matches){svg.innerHTML='';return}
  // A redraw removes the hovered node without a mouseout, so drop any highlight it left.
  document.querySelectorAll('.fl-card.fl-hl').forEach(function(c){c.classList.remove('fl-hl')});
  var base=main.getBoundingClientRect(),ls=[],raw=[],byId=function(id){return document.getElementById(id)};
  (FL.board.links||[]).forEach(function(l){
    var e=fleetLinkCards(l,now,byId);if(!e)return;
    var lab=fleetLinkLabel(FL.board,l,now);
    ls.push({from:e.src.id,to:e.dst.id,kind:l.kind,label:lab.text,labelW:flTextW(lab.text)});raw.push({l:l,lab:lab,cls:e.cls});
  });
  var w=main.scrollWidth,h=main.scrollHeight,lines='',pills='';
  fleetRoute(flRects(main,base),ls,{bounds:{x0:0,y0:0,x1:w,y1:h},bands:flRects2(main,base,'.fl-account-head,.fl-strip,.fl-log,.fl-wt-tail,.fl-banner')}).forEach(function(r,i){
    var m=raw[r.index];r.cls=m.cls;
    var s=flLinkSvg(r,m.l,m.lab,i);lines+=s.line;pills+=s.pill;
  });
  svg.setAttribute('width',w);svg.setAttribute('height',h);
  // Chevron heads in user-space pixels, tip on the input port, filled by kind.
  var mkr=function(id,cls){return '<marker id="'+id+'" viewBox="0 0 10 10" refX="10" refY="5" markerUnits="userSpaceOnUse" markerWidth="12" markerHeight="12" orient="auto"><path class="'+cls+'" d="M0 0L10 5L0 10L3.5 5z"/></marker>'};
  svg.innerHTML='<defs>'+mkr('fl-arrow','fl-mk-msg')+mkr('fl-arrow-agent','fl-mk-agent')+mkr('fl-arrow-handback','fl-mk-handback')+'</defs>'+lines+pills;
  flLinkFade();
}
// The last eight seconds of a line's 30 fade it out; it is removed at 30 by the redraw in flTick.
function flLinkFade(){
  var now=flSrvNow();
  document.querySelectorAll('#fl-links [data-at]').forEach(function(e){var age=now-Number(e.dataset.at);e.style.opacity=age>22?String(Math.max(0.2,(30-age)/8)):''});
}
// Hovering a line or its pill highlights both cards.
function flLinkHover(e,on){
  var g=e.target.closest&&e.target.closest('#fl-links .fl-link');if(!g)return;
  [g.dataset.from,g.dataset.to].forEach(function(id){var c=document.getElementById(id);if(c)c.classList.toggle('fl-hl',on)});
  document.querySelectorAll('#fl-links [data-i="'+g.dataset.i+'"]').forEach(function(x){x.classList.toggle('fl-on',on)});
}
function flTick(){
  var now=flNow();
  document.querySelectorAll('.fl-ago[data-t]').forEach(function(e){e.textContent=fleetAgeSfx(now-Number(e.dataset.t),e.dataset.sfx||'')});
  document.querySelectorAll('[data-wake-at]').forEach(function(e){var at=Number(e.dataset.wakeAt);e.textContent=e.dataset.wakeChip?fleetWakeChipText(at,now):fleetWakeText(at,now)});
  // Redraw only when a line expires: rewriting the SVG every second would restart the dash animation.
  if(FL.board&&fleetLinkSig(FL.board.links,flSrvNow())!==FL.linkSig)flLinks();else flLinkFade();
  flLive();
}
// The snapshot endpoint answers a bare board or the {available, board, error}
// envelope; this returns the board, or null with FL.down set to the reason.
function flBoardOf(d){
  if(d&&typeof d.available==='boolean'){if(d.available&&d.board)return d.board;FL.down=d.error||'unavailable';return null}
  if(d&&d.error){FL.down=d.error;return null}
  return d&&typeof d==='object'?d:null;
}
function flApply(d){
  var b=flBoardOf(d);if(!b)return false;
  FL.board=b;FL.tickAt=b.generated_at||flNow();FL.skew=FL.tickAt-flNow();FL.down=null;flRender();return true;
}
// A 401 means the cookie is missing or expired: stop the stream and ask for the token.
function flLoginShow(msg){
  var box=document.getElementById('fl-login');if(!box)return;
  if(FL.es){FL.es.close();FL.es=null}
  box.hidden=false;document.getElementById('fl-signout').hidden=true;
  document.getElementById('fl-login-err').textContent=msg||'';
  FL.down='sign-in needed';flLive();
  var t=document.getElementById('fl-token');if(t)t.focus();
}
// EventSource reports no status, so a failed stream is probed with a plain fetch.
function flProbe(){
  fetch(FL_URLS.snapshot,{credentials:'same-origin',cache:'no-store'}).then(function(r){
    if(r.status===401){flLoginShow();return}
    setTimeout(flConnect,5000);
  }).catch(function(){setTimeout(flConnect,5000)});
}
function flConnect(){
  if(FL.es){FL.es.close();FL.es=null}
  if(!window.EventSource){flPoll();return}
  var es=new EventSource(FL_URLS.stream);FL.es=es;
  es.addEventListener('board',function(e){try{flApply(JSON.parse(e.data))}catch(err){}});
  es.addEventListener('tick',function(e){try{FL.tickAt=JSON.parse(e.data).generated_at||flNow();FL.down=null}catch(err){}flLive()});
  es.addEventListener('down',function(e){try{FL.down=JSON.parse(e.data).error}catch(err){FL.down='unreachable'}flLive()});
  es.onopen=function(){if(FL.down==='reconnecting')FL.down=null;flLive()};
  es.onerror=function(){if(!FL.down)FL.down='reconnecting';flLive();if(es.readyState===2){FL.es=null;flProbe()}};
}
// Fallback for a browser without EventSource.
function flPoll(){
  fetch(FL_URLS.snapshot,{credentials:'same-origin',cache:'no-store'}).then(function(r){
    if(r.status===401){flLoginShow();return null}
    return r.json();
  }).then(function(d){
    if(d===null)return;
    if(!flApply(d)&&!FL.down)FL.down='no board yet';
    flLive();setTimeout(flPoll,5000);
  }).catch(function(){FL.down='unreachable';flLive();setTimeout(flPoll,5000)});
}
function flStart(){
  fetch(FL_URLS.snapshot,{credentials:'same-origin',cache:'no-store'}).then(function(r){
    if(r.status===401){flLoginShow();return}
    flConnect();
  }).catch(flConnect);
}
document.addEventListener('submit',function(e){
  if(!e.target||e.target.id!=='fl-login-form')return;
  e.preventDefault();
  var t=document.getElementById('fl-token'),err=document.getElementById('fl-login-err');
  fetch(FL_URLS.session,{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},body:JSON.stringify({token:t.value})}).then(function(r){
    if(r.ok){t.value='';err.textContent='';document.getElementById('fl-login').hidden=true;document.getElementById('fl-signout').hidden=false;FL.down=null;flConnect();return}
    err.textContent=r.status===401?'That token was not accepted.':r.status===404?'Sign-in is switched off on this server.':'Sign-in failed ('+r.status+').';
  }).catch(function(){err.textContent='The server could not be reached.'});
});
document.addEventListener('click',function(e){
  if(!e.target.closest('#fl-signout'))return;
  fetch(FL_URLS.session,{method:'DELETE',credentials:'same-origin'}).finally(function(){FL.board=null;flLoginShow()});
});
document.addEventListener('change',function(e){
  if(e.target&&e.target.id==='fl-order'){FL.order=fleetOrder(e.target.value);flSave();flRender()}
});
document.addEventListener('click',function(e){
  var f=e.target.closest('[data-filter]');if(f){FL.filter=f.dataset.filter;FL.costOpen=false;flSave();flRender();return}
  if(e.target.closest('#fl-cost-toggle')){FL.costOpen=!FL.costOpen;flRender();return}
  if(FL.costOpen&&!e.target.closest('.fl-cost-panel')){FL.costOpen=false;flRender()}
  if(e.target.closest('#fl-ended')){FL.showEnded=!FL.showEnded;flSave();flRender();return}
  var cp=e.target.closest('[data-copy]');
  if(cp){try{navigator.clipboard.writeText(cp.dataset.copy);cp.classList.add('fl-copied');setTimeout(function(){cp.classList.remove('fl-copied')},1200)}catch(err){}return}
  var t=e.target.closest('a[href^="#fl-"]');
  if(t){
    var ids=t.dataset.sessions?t.dataset.sessions.split(' ').filter(Boolean).map(function(i){return 'fl-'+i}):[t.getAttribute('href').slice(1)];
    var cards=ids.map(function(i){return document.getElementById(i)}).filter(Boolean);
    if(t.dataset.agent){var ab=document.getElementById('fl-ag-'+t.dataset.agent);if(ab)cards.push(ab)}
    if(cards.length){e.preventDefault();cards[0].scrollIntoView({behavior:'smooth',block:'center'});cards.forEach(function(c){c.classList.add('fl-hl')});setTimeout(function(){cards.forEach(function(c){c.classList.remove('fl-hl')})},1600)}
  }
});
document.addEventListener('keydown',function(e){if(e.key==='Escape'&&FL.costOpen){FL.costOpen=false;flRender()}});
// toggle does not bubble, so listen in the capture phase.
document.addEventListener('toggle',function(e){
  var d=e.target;if(!d||d.tagName!=='DETAILS'||!d.dataset.key)return;
  if(d.open)FL.open[d.dataset.key]=true;else delete FL.open[d.dataset.key];
},true);
document.addEventListener('mouseover',function(e){flLinkHover(e,true)});
document.addEventListener('mouseout',function(e){flLinkHover(e,false)});
window.addEventListener('resize',function(){flTop();flLinks()});
setInterval(flTick,1000);
flStart();
