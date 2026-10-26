(function (root) {
  'use strict';

  const limit = 180, width = 820, height = 110, padding = 9;
  const models = new Map();
  let initialized = false, lastFocus = null;
  const escape = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const numeric = value => (typeof value === 'number' || typeof value === 'string' && value.trim()) && Number.isFinite(Number(value)) && Number(value) >= 0 ? Number(value) : null;
  const number = value => value == null ? 'Not measured' : new Intl.NumberFormat(undefined, {maximumFractionDigits:3}).format(value) + ' ms';

  function localTime(value) {
    if (root.NetworkToolkitResultTime) return root.NetworkToolkitResultTime.format(value);
    if (value == null || String(value).trim() === '' || /^0001-01-01/.test(String(value))) return 'Not recorded';
    const date = new Date(value);
    if (!Number.isFinite(date.getTime())) return 'Not recorded';
    return new Intl.DateTimeFormat(undefined, {year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',fractionalSecondDigits:3,timeZoneName:'short'}).format(date);
  }

  function normalize(samples) {
    if (!Array.isArray(samples)) return [];
    const offset = Math.max(0, samples.length - limit);
    let previous = null;
    return samples.slice(offset).map((sample, index) => {
      const source = sample && typeof sample === 'object' ? sample : {};
      const time = [source.time,source.timestamp,source.observedAt,source.recordedAt].find(value=>value!=null && String(value).trim()!=='') ?? '';
      const sequence = source.sequence ?? offset + index + 1;
      const address = String(source.address ?? '').slice(0, 512);
      const success = source.success === true, rtt = success ? numeric(source.rttMs) : null;
      const status = String(source.status || (success ? 'Reply' : 'No response')).slice(0, 1024);
      const jitter = rtt != null && previous != null ? Math.abs(rtt - previous) : null;
      previous = rtt;
      const key = source.sequence != null || time !== '' ? JSON.stringify([source.sequence ?? '', time, address]) : 'legacy:' + JSON.stringify([offset + index, rtt, status, address]);
      return {key, index, sequence, time, address, status, success, rtt, jitter, error:String(source.error || '').slice(0, 2048)};
    });
  }

  function summary(points) {
    const successful = points.filter(point => point.success), values = points.map(point => point.rtt).filter(value => value != null), changes = points.map(point => point.jitter).filter(value => value != null);
    return {count:points.length, received:successful.length, lost:points.length-successful.length,
      lossPercent:points.length ? (points.length-successful.length)/points.length*100 : 0,
      min:values.length ? Math.min(...values) : null, max:values.length ? Math.max(...values) : null,
      average:values.length ? values.reduce((a,b)=>a+b,0)/values.length : null,
      jitter:changes.length ? changes.reduce((a,b)=>a+b,0)/changes.length : null, jitterPairs:changes.length};
  }

  function pointDetails(point) {
    if (!point) return '<span class="rt-instruction">Hover over a point for details. Click to keep it selected. Focus a chart and use the arrow keys to move between samples; Escape clears the selection.</span>';
    const fields = [['Local time',localTime(point.time)],['Sample',String(point.sequence)],['Result',point.status],['Response time',number(point.rtt)],['RTT change',point.jitter == null ? 'No consecutive measured pair' : number(point.jitter)]];
    if (point.address) fields.push(['Address',point.address]);
    if (point.error) fields.push(['Error',point.error]);
    return '<dl class="rt-detail-values">' + fields.map(([label,value])=>'<div><dt>'+escape(label)+'</dt><dd>'+escape(value)+'</dd></div>').join('') + '</dl>';
  }

  function coordinate(index, count) { return padding + index * (width - 2 * padding) / Math.max(1, count - 1); }
  function ordinate(value, maximum) { return height - padding - value / maximum * (height - 2 * padding); }

  function plot(points, metric, maximum, average, selected) {
    const segments = [], marks = [];
    let segment = [];
    for (const point of points) {
      const x = coordinate(point.index,points.length), value = point[metric], selectedClass = point.key === selected ? ' rt-point-selected' : '';
      if (value == null) {
        if (segment.length) segments.push(segment);
        segment = [];
      } else segment.push(x.toFixed(2) + ',' + ordinate(value,maximum).toFixed(2));
      if (metric === 'rtt' && !point.success) {
        marks.push('<g class="rt-point rt-missed'+selectedClass+'" data-rt-point="'+point.index+'"><rect x="'+(x-3).toFixed(2)+'" y="3" width="6" height="104"/><path d="M'+(x-3).toFixed(2)+' 5l6 6m-6 0l6-6"/></g>');
      } else if (value != null) {
        marks.push('<circle class="rt-point'+selectedClass+'" data-rt-point="'+point.index+'" cx="'+x.toFixed(2)+'" cy="'+ordinate(value,maximum).toFixed(2)+'" r="3"/>');
      }
    }
    if (segment.length) segments.push(segment);
    const lines = segments.filter(segment=>segment.length>1).map(segment=>'<polyline points="'+segment.join(' ')+'"/>').join('');
    const mean = average == null ? '' : '<line class="rt-average" x1="'+padding+'" x2="'+(width-padding)+'" y1="'+ordinate(average,maximum).toFixed(2)+'" y2="'+ordinate(average,maximum).toFixed(2)+'"/>';
    const name = metric === 'rtt' ? 'latency' : 'jitter';
    return '<div class="rt-plot rt-'+name+'"><div class="rt-axis-label">'+escape(number(maximum))+'</div><svg data-rt-surface="'+name+'" tabindex="0" role="group" aria-label="'+(metric==='rtt'?'Response time':'Absolute RTT change between consecutive measured replies')+'. Use Left and Right arrows to inspect samples, Home and End to jump, Enter to select, Escape to clear." viewBox="0 0 '+width+' '+height+'" preserveAspectRatio="none"><path class="rt-grid" d="M0 27H820 M0 55H820 M0 82H820"/>'+mean+'<g class="rt-series">'+lines+'</g><g class="rt-points" aria-hidden="true">'+marks.join('')+'</g></svg></div>';
  }

  function latency(samples, context = {}) {
    const points = normalize(samples);
    if (!points.length) return '';
    const id = String(context.id ?? 'latency').slice(0,512), fingerprint = points.map(point=>point.key).join('|');
    let model = models.get(id);
    if (!model) model = {pinned:null,hovered:null,fingerprint:''};
    const dropped = model.pinned && (!points.some(point=>point.key===model.pinned) || model.pinned.startsWith('legacy:') && model.fingerprint!==fingerprint);
    if (dropped) model.pinned = null;
    if (model.hovered && !points.some(point=>point.key===model.hovered)) model.hovered=null;
    Object.assign(model,{points,fingerprint});
    models.delete(id);models.set(id,model);
    while (models.size > 64) models.delete(models.keys().next().value);
    const stats = summary(points), selected = points.find(point=>point.key===(model.hovered||model.pinned));
    const maximum = Math.max(1,stats.max || 0)*1.12, maxJitter = Math.max(1,...points.map(point=>point.jitter || 0))*1.12;
    const windowLabel = samples.length>limit ? 'Latest '+limit+' retained samples' : 'Displayed samples';
    const loss = new Intl.NumberFormat(undefined,{maximumFractionDigits:1}).format(stats.lossPercent);
    const label = String(context.title || 'Response time');
    const detail = selected ? pointDetails(selected) : dropped ? '<span class="rt-instruction">The selected sample is outside the displayed window. Select another point to inspect it.</span>' : pointDetails(null);
    return '<section class="rt-timeline" data-rt-id="'+escape(id)+'" aria-label="'+escape(label)+' timeline"><div class="rt-heading"><h3>'+escape(label)+'</h3><span>'+escape(windowLabel)+' ('+points.length+')</span></div>'+plot(points,'rtt',maximum,stats.average,selected?.key)+'<div class="rt-time-axis"><time>'+escape(localTime(points[0].time))+'</time><time>'+escape(localTime(points.at(-1).time))+'</time></div><div class="rt-window-stats" aria-label="Statistics for displayed samples"><span><strong>'+stats.received+'/'+stats.count+'</strong> successful</span><span><strong>'+loss+'%</strong> unsuccessful</span><span>Minimum <strong>'+escape(number(stats.min))+'</strong></span><span>Average <strong>'+escape(number(stats.average))+'</strong></span><span>Maximum <strong>'+escape(number(stats.max))+'</strong></span></div><div class="rt-loss-bar" role="img" aria-label="Displayed samples: '+stats.received+' successful, '+stats.lost+' unsuccessful"><span class="rt-success-share" style="width:'+((stats.count-stats.lost)/stats.count*100).toFixed(4)+'%"></span><span class="rt-loss-share" style="width:'+stats.lossPercent.toFixed(4)+'%"></span></div><div class="rt-loss-legend"><span>Successful: '+stats.received+'</span><span>Unsuccessful: '+stats.lost+'</span><span>Dashed line: displayed average RTT</span></div><details class="rt-variability"'+(model.variabilityOpen?' open':'')+'><summary>RTT variability <span>'+escape(number(stats.jitter))+' mean absolute change</span></summary><p>Absolute RTT change between consecutive successful measured replies in this displayed window. A missing or unsuccessful probe breaks the pair.</p>'+(stats.jitterPairs ? plot(points,'jitter',maxJitter,stats.jitter,selected?.key) : '<p>No consecutive measured reply pairs are available.</p>')+'</details><div class="rt-detail-controls"><span>Point details</span><button type="button" class="btn small ghost" data-rt-clear '+(model.pinned?'':'disabled')+'>Clear selection</button></div><div class="rt-detail" role="status" aria-live="polite" aria-atomic="true">'+detail+'</div></section>';
  }

  function contextFor(target) {
    const container = target?.closest?.('.rt-timeline');
    return container ? {container,model:models.get(container.dataset.rtId)} : null;
  }

  function renderSelection(context, point) {
    if (!context?.model) return;
    const {container,model} = context;
    const detail = container.querySelector('.rt-detail');
    if (detail) detail.innerHTML = pointDetails(point);
    for (const mark of container.querySelectorAll('[data-rt-point]')) mark.classList.toggle('rt-point-selected',!!point && Number(mark.dataset.rtPoint)===point.index);
    const clear = container.querySelector('[data-rt-clear]');
    if (clear) clear.disabled = !model.pinned;
  }

  function selectedPoint(model) { return model.points.find(point=>point.key===model.pinned) || null; }
  function pointerPoint(event, context) {
    const surface = event.target?.closest?.('[data-rt-surface]');
    if (!surface || !context?.model) return null;
    const mark = event.target.closest?.('[data-rt-point]');
    if (mark) return context.model.points[Number(mark.dataset.rtPoint)] || null;
    const bounds = surface.getBoundingClientRect();
    if (!bounds.width || !Number.isFinite(event.clientX)) return null;
    const index = Math.round(((event.clientX-bounds.left)/bounds.width*width-padding)/(width-2*padding)*(context.model.points.length-1));
    return context.model.points[Math.max(0,Math.min(context.model.points.length-1,index))];
  }

  function initialize() {
    const document = root.document;
    if (initialized || !document?.addEventListener) return;
    initialized = true;
    document.addEventListener('pointermove',event=>{
      const context=contextFor(event.target),point=pointerPoint(event,context);
      if (!point || context.model.hovered===point.key) return;
      context.model.hovered=point.key;renderSelection(context,point);
    });
    document.addEventListener('pointerout',event=>{
      const context=contextFor(event.target);
      if (!context || event.relatedTarget && context.container.contains(event.relatedTarget)) return;
      context.model.hovered=null;renderSelection(context,selectedPoint(context.model));
    });
    document.addEventListener('click',event=>{
      const context=contextFor(event.target);
      if (!context?.model) return;
      if (event.target.closest?.('[data-rt-clear]')) { context.model.pinned=null;context.model.hovered=null;renderSelection(context,null);return; }
      const point=pointerPoint(event,context);
      if (!point) return;
      context.model.pinned=point.key;renderSelection(context,point);
    });
    document.addEventListener('keydown',event=>{
      const context=contextFor(event.target),surface=event.target.closest?.('[data-rt-surface]');
      if (!context?.model || !surface || !['ArrowLeft','ArrowRight','Home','End','Enter',' ','Escape'].includes(event.key)) return;
      event.preventDefault();event.stopPropagation();
      if (event.key==='Escape') {context.model.pinned=null;context.model.hovered=null;renderSelection(context,null);return;}
      const points=context.model.points,current=points.findIndex(point=>point.key===(context.model.pinned||context.model.hovered));
      const index=event.key==='Home'?0:event.key==='End'?points.length-1:event.key==='ArrowLeft'?Math.max(0,(current<0?points.length:current)-1):event.key==='ArrowRight'?Math.min(points.length-1,current+1):Math.max(0,current);
      context.model.pinned=points[index].key;renderSelection(context,points[index]);
    });
    document.addEventListener('focusin',event=>{
      const surface=event.target.closest?.('[data-rt-surface]'),context=contextFor(event.target);
      if (surface && context) lastFocus={node:surface,id:context.container.dataset.rtId,surface:surface.dataset.rtSurface};
      else lastFocus=null;
    });
    document.addEventListener('focusout',event=>{
      if (lastFocus && event.target===lastFocus.node && event.relatedTarget) lastFocus=null;
    });
    document.addEventListener('toggle',event=>{
      if (!event.target.matches?.('.rt-variability')) return;
      const context=contextFor(event.target);
      if (context?.model) context.model.variabilityOpen=event.target.open===true;
    },true);
    if (typeof root.MutationObserver==='function') new root.MutationObserver(()=>{
      if (!lastFocus || lastFocus.node.isConnected || document.activeElement && document.activeElement!==document.body) return;
      const saved=lastFocus;
      const container=[...document.querySelectorAll('.rt-timeline')].find(item=>item.dataset.rtId===saved.id);
      const replacement=container?.querySelector('[data-rt-surface="'+saved.surface+'"]');
      lastFocus=null;
      replacement?.focus({preventScroll:true});
    }).observe(document.documentElement || document,{subtree:true,childList:true});
  }

  const api=Object.freeze({latency,normalize,summary,pointDetails,initialize});
  if (typeof module==='object' && module.exports) module.exports=api;
  root.NetworkToolkitResultTimeline=api;
  initialize();
})(typeof window!=='undefined'?window:globalThis);
