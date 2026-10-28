// Client script for the fleet page: the pure render helpers shipped to the
// browser by toString() (so it runs exactly the code the tests cover) plus the
// small event/SSE glue and the sign-in form. It imports no server framework;
// the host passes the URLs in.

const render = require('./fleet-render');
const activity = require('./fleet-render-activity');
const base = require('./fleet-render-base');
const wt = require('./fleet-render-worktrees');
const cost = require('./fleet-render-cost');
const wake = require('./fleet-render-wake');
const servers = require('./fleet-render-servers');
const msgs = require('./fleet-render-msgs');
const links = require('./fleet-links');
const route = require('./fleet-route');
const routeCard = require('./fleet-route-card');

// opts.streamUrl: SSE endpoint; opts.snapshotUrl: JSON snapshot, either a bare
// board or an {available, board, error} envelope; opts.sessionUrl: where the
// sign-in form posts the token.
function fleetClientScript(opts) {
  const o = Object.assign({ streamUrl: '/v1/events', snapshotUrl: '/v1/fleet', sessionUrl: '/v1/session' }, opts || {});
  const fns = [
    base.fleetEsc, render.fleetUrl, base.fleetAge, base.fleetAgeSfx, base.fleetAgo, render.fleetState, render.fleetBranchHue,
    render.fleetBranchChip, render.fleetCtxBar, render.fleetToolGlyph, render.fleetIssues, render.fleetRepoLine,
    activity.fleetClock, activity.fleetActivity, activity.fleetAgentCurrent, activity.fleetLifeline, activity.fleetRecent,
    activity.fleetRecentBadge, activity.fleetHandbackChip,
    cost.fleetUsd, cost.fleetTok, cost.fleetModelClass, cost.fleetCostBar, cost.fleetCostTokens, cost.fleetCostTip, cost.fleetCostChip,
    cost.fleetCostTotal, cost.fleetCostPanel, cost.fleetSetKeepScroll,
    wake.fleetWakeOf, wake.fleetWakeMins, wake.fleetWakeText, wake.fleetWakeChipText, wake.fleetWakeBadge, wake.fleetWakeSuffix,
    servers.fleetServersDur, servers.fleetServersOf, servers.fleetServersWhy, servers.fleetServerRow, servers.fleetServersLine, servers.fleetServersCount,
    render.fleetAgent, render.fleetSortSessions, render.fleetOrder, render.fleetSortBy, render.fleetSortAccounts, render.fleetClean,
    render.fleetHeadline, render.fleetProse, render.fleetDescription, render.fleetPlural, render.fleetToolRow, render.fleetHistory, msgs.fleetMsgName, msgs.fleetMsgList, render.fleetSession, render.fleetBranches, render.fleetCounts, render.fleetVisible,
    render.fleetAccounts, render.fleetLog, render.fleetSlots,
    links.fleetHasBox, links.fleetLinkClass, links.fleetLinkFresh, links.fleetLinkSig, links.fleetAgentBox, links.fleetLinkCards, msgs.fleetEndName, msgs.fleetLinkLabel,
    route.fleetRouteGaps, route.fleetRouteHit, route.fleetRouteShares, route.fleetRouteLane, route.fleetRoundPath, route.fleetRouteRows, route.fleetRoutePorts, route.fleetRouteSearch, route.fleetRoutePill, route.fleetRoute, routeCard.fleetRouteAnchor, routeCard.fleetRouteInCardPoints, routeCard.fleetRouteInCard,
    wt.fleetWtWriters, wt.fleetWtName, wt.fleetWtHint, wt.fleetWtChip, wt.fleetWtRow, wt.fleetWorktrees, wt.fleetInFlight,
  ];
  return `
${fns.map((f) => f.toString()).join('\n')}
var FL_URLS={stream:${JSON.stringify(o.streamUrl)},snapshot:${JSON.stringify(o.snapshotUrl)},session:${JSON.stringify(o.sessionUrl)}};
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
// fleetRoute (pure, unit tested) and drawn here. A message inside one card is
// routed by fleetRouteInCard instead.
function flRects(main,base){
  var out=[],els=main.querySelectorAll('.fl-card');
  for(var i=0;i<els.length;i++){
    if(!fleetHasBox(els[i]))continue;
    var r=els[i].getBoundingClientRect();
    out.push({id:els[i].id,x:r.left-base.left,y:r.top-base.top,w:r.width,h:r.height});
  }
  return out;
}
function flRect(el,base){var r=el.getBoundingClientRect();return {x:r.left-base.left,y:r.top-base.top,w:r.width,h:r.height}}
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
  var at=' data-i="'+i+'" data-from="'+fleetEsc(r.from)+'" data-to="'+fleetEsc(r.to)+'" data-at="'+fleetEsc(l.at)+'" data-fa="'+fleetEsc(l.from_agent||'')+'" data-ta="'+fleetEsc(l.to_agent||'')+'"';
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
  var base=main.getBoundingClientRect(),ls=[],raw=[],inner=[],byId=function(id){return document.getElementById(id)};
  // A subagent box pins the port to its own height, so the line lines up with it.
  var portY=function(b){if(!b)return null;var r=b.getBoundingClientRect();return r.top-base.top+Math.min(r.height/2,16)};
  (FL.board.links||[]).forEach(function(l){
    var e=fleetLinkCards(l,now,byId);if(!e)return;
    var lab=fleetLinkLabel(FL.board,l,now);
    if(e.inCard){inner.push({e:e,l:l,lab:lab});return}
    ls.push({from:e.src.id,to:e.dst.id,kind:l.kind,label:lab.text,labelW:flTextW(lab.text),fromY:portY(e.srcBox),toY:portY(e.dstBox)});raw.push({l:l,lab:lab,cls:e.cls});
  });
  var w=main.scrollWidth,h=main.scrollHeight,lines='',pills='',n=0;
  fleetRoute(flRects(main,base),ls,{bounds:{x0:0,y0:0,x1:w,y1:h},bands:flRects2(main,base,'.fl-account-head,.fl-strip,.fl-log,.fl-wt-tail,.fl-banner')}).forEach(function(r){
    var m=raw[r.index];r.cls=m.cls;
    var s=flLinkSvg(r,m.l,m.lab,n++);lines+=s.line;pills+=s.pill;
  });
  // Messages between boxes of one card run down that card's left gutter.
  var lane={};
  inner.forEach(function(m){
    var card=m.e.src,k=lane[card.id]||0;lane[card.id]=k+1;
    var r=fleetRouteInCard(flRect(card,base),flRect(m.e.srcBox,base),flRect(m.e.dstBox,base),{lane:k,label:m.lab.text,labelW:flTextW(m.lab.text),bounds:{x1:w}});
    if(!r)return;
    r.from=card.id;r.to=card.id;r.cls=m.e.cls;
    var s=flLinkSvg(r,m.l,m.lab,n++);lines+=s.line;pills+=s.pill;
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
`;
}

module.exports = { fleetClientScript };
