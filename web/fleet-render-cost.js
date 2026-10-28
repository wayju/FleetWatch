// API-list-price-equivalent cost helpers for the fleet page. Same
// injection rules as fleet-render.js: PURE functions, no module-level bindings
// beyond the injected fleetEsc, shipped to the page by toString() in
// client.js (dependency order: fleetUsd, fleetTok, fleetModelClass,
// fleetCostBar, fleetCostTokens, fleetCostTip, fleetCostChip, fleetCostTotal,
// fleetCostPanel). Every board field is optional: absent cost renders nothing.

const { fleetEsc } = require('./fleet-render-base');

// <$10 keeps cents, anything larger is whole dollars with thousands separators.
function fleetUsd(n) {
  var v = Number(n);
  if (!isFinite(v) || v < 0) v = 0;
  if (v < 10) return '$' + v.toFixed(2);
  return '$' + String(Math.round(v)).replace(/\B(?=(\d{3})+(?!\d))/g, ',');
}

// 11.9B, 68M, 920k, 42.
function fleetTok(n) {
  var v = Number(n);
  if (!isFinite(v) || v < 0) v = 0;
  function f(x, sfx) { return (x >= 100 ? String(Math.round(x)) : String(Math.round(x * 10) / 10)) + sfx; }
  if (v >= 1e9) return f(v / 1e9, 'B');
  if (v >= 1e6) return f(v / 1e6, 'M');
  if (v >= 1e3) return f(v / 1e3, 'k');
  return String(Math.round(v));
}

// Colour class for a model: the board's model_class (a by_model row or a session
// carries one, from the models config). A name that is not a plain token, or no
// class at all, gives the neutral class; theme.css maps each class to a colour.
function fleetModelClass(entry) {
  var mc = entry && typeof entry === 'object' ? String(entry.model_class || '').toLowerCase() : '';
  return /^[a-z0-9][a-z0-9_-]{0,31}$/.test(mc) ? 'fl-mc-' + mc : 'fl-mc-other';
}

// Thin stacked bar of each model's share of the cost. Widths are percentages
// of the summed cost, so the bar always fills.
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

// Plain text for a title attribute: one line per model.
function fleetCostTip(cost) {
  var rows = (cost && Array.isArray(cost.by_model)) ? cost.by_model : [];
  var lines = rows.map(function (m) {
    return String(m.model || 'unknown') + ' · ' + (Number(m.messages) || 0) + ' msgs · ' + (m.unpriced ? 'unpriced' : fleetUsd(m.usd)) + ' · ' + fleetCostTokens(m.tokens);
  });
  lines.push('API list-price equivalent, not billed (plan)');
  return lines.join('\n');
}

// The card chip: "≈ $1,234 API" (or "≈ … counting" while the first pass runs).
function fleetCostChip(cost) {
  if (!cost || typeof cost !== 'object') return '';
  var label = cost.partial ? '≈ … counting' : '≈ ' + fleetUsd(cost.usd) + ' API';
  return '<span class="fl-cost" title="' + fleetEsc(fleetCostTip(cost)) + '"><span class="fl-cost-chip' + (cost.partial ? ' fl-cost-partial' : '') + '">'
    + fleetEsc(label) + '</span>' + fleetCostBar(cost.by_model) + '</span>';
}

// Header fleet total; a button so the delegated click handler can toggle the panel.
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

// Replace host's markup with html, then put the scrollTop of the element
// matching sel back. The board re-renders every ~2s and a fresh panel element
// starts at 0, which threw a scrolled cost dropdown back to the top.
function fleetSetKeepScroll(host, sel, html) {
  var old = host.querySelector(sel);
  var top = old ? old.scrollTop : 0;
  host.innerHTML = html;
  var fresh = host.querySelector(sel);
  if (fresh && top) fresh.scrollTop = top;
}

module.exports = { fleetSetKeepScroll, fleetUsd, fleetTok, fleetModelClass, fleetCostBar, fleetCostTokens, fleetCostTip, fleetCostChip, fleetCostTotal, fleetCostPanel };
