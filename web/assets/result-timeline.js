(function (root) {
  'use strict';

  const retainedLimit=10000, width=820, height=110, padding=12, spacing=8, minZoom=.25, maxZoom=16;
  const models=new Map(),wheelFrames=new Map();
  let initialized=false,lastFocus=null,activeHover=null,hoverPointer=null,tooltip=null,restoring=false,resizeObserver=null;
  const observedScrollers=new Set();
  let fallbackPreferences={mode:'linear'},fallbackPlots={};
  const plotName=value=>value==='jitter'?'jitter':'rtt';
  function plotPreferences(){try{return typeof state!=='undefined'&&state.graphPlotPreferences||fallbackPlots;}catch{return fallbackPlots;}}
  function setPlot(id,value){const model=models.get(String(id));if(!model)return false;model.plot=plotName(value);fallbackPlots={...plotPreferences(),[model.tool]:model.plot};try{if(typeof state!=='undefined')state.graphPlotPreferences=fallbackPlots;if(typeof saveState==='function')saveState();}catch{}const container=findContainer(id);if(container)rebuild(container,model);return true;}
  function tabsHTML(id){const model=models.get(String(id));if(!model)return '';return '<div class="rt-plot-tabs" role="tablist" aria-label="Measurement plot">'+[['rtt','Response time (RTT)'],['jitter','RTT variability']].map(([key,label])=>'<button type="button" class="btn small ghost" role="tab" data-rt-tab="'+key+'" aria-selected="'+(model.plot===key)+'" tabindex="'+(model.plot===key?'0':'-1')+'" aria-controls="rt-active-'+encodeURIComponent(model.id)+'">'+label+'</button>').join('')+'</div>';}
  function tracking(id){const m=models.get(String(id));return {following:!!m?.follow&&!m?.pinned,held:!!m?.pinned};}
  const escape=value=>String(value==null?'':value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const numeric=value=>(typeof value==='number'||typeof value==='string'&&value.trim())&&Number.isFinite(Number(value))&&Number(value)>=0?Number(value):null;
  const numberFormatter=new Intl.NumberFormat(undefined,{maximumFractionDigits:3}),lossFormatter=new Intl.NumberFormat(undefined,{maximumFractionDigits:1});
  const number=value=>value==null?'Not measured':numberFormatter.format(value)+' ms';
  function normalizePreferences(value){return {mode:value?.mode==='condensed'?'condensed':'linear'};}
  function preferences(){try{return normalizePreferences(typeof state!=='undefined'?state.graphPreferences:fallbackPreferences);}catch{return normalizePreferences(fallbackPreferences);}}
  function mergePreferences(value){fallbackPlots={};for(const [key,v] of Object.entries(value?.graphPlotPreferences||{}))if(['ping','monitor','latency'].includes(key)&&['rtt','jitter'].includes(v))fallbackPlots[key]=v;try{if(typeof state!=='undefined')state.graphPlotPreferences={...fallbackPlots};}catch{}if(!value||!Object.prototype.hasOwnProperty.call(value,'graphPreferences'))return preferences();const next=normalizePreferences(value.graphPreferences);fallbackPreferences=next;try{if(typeof state!=='undefined')state.graphPreferences=next;}catch{}return next;}
  function setPreferences(value,{persist=true}={}){
    const next=normalizePreferences(value);fallbackPreferences=next;
    try{if(typeof state!=='undefined')state.graphPreferences=next;if(persist&&typeof saveState==='function')saveState();}catch{}
    if(root.document?.dispatchEvent&&typeof root.CustomEvent==='function')root.document.dispatchEvent(new root.CustomEvent('network-toolkit-graph-preferences',{detail:next}));
    return next;
  }
  function localTime(value){
    if(root.NetworkToolkitResultTime)return root.NetworkToolkitResultTime.format(value);
    if(value==null||String(value).trim()===''||/^0001-01-01/.test(String(value)))return 'Not recorded';
    const date=new Date(value);if(!Number.isFinite(date.getTime()))return 'Not recorded';
    return new Intl.DateTimeFormat(undefined,{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',fractionalSecondDigits:3,timeZoneName:'short'}).format(date);
  }
  const timeLabel=()=>root.NetworkToolkitResultTime?.timeLabel?.()||'Local time';
  function sampleTime(source){return [source.time,source.timestamp,source.observedAt,source.recordedAt].find(value=>value!=null&&String(value).trim()!=='')??'';}
  function sampleKey(sample,index=0){
    const source=sample&&typeof sample==='object'?sample:{},time=sampleTime(source),address=String(source.address??'').slice(0,512);
    return source.sequence!=null||time!==''?JSON.stringify([source.sequence??'',time,address]):'legacy:'+JSON.stringify([index,source.success===true?numeric(source.rttMs):null,String(source.status||(source.success===true?'Reply':'No response')).slice(0,1024),address]);
  }
  function normalize(samples,options={}){
    if(!Array.isArray(samples))return [];
    const offset=Math.max(0,samples.length-retainedLimit);let previous=null,previousSequence=null;
    return samples.slice(offset).map((sample,index)=>{
      const source=sample&&typeof sample==='object'?sample:{},time=sampleTime(source),success=source.success===true,rtt=success?numeric(source.rttMs):null;
      const sequence=numeric(source.sequence),consecutive=previousSequence==null||sequence==null||sequence===previousSequence+1;
      const jitter=rtt!=null&&previous!=null&&consecutive?Math.abs(rtt-previous):null;previous=rtt;previousSequence=sequence;
      return {key:sampleKey(source,offset+index),index,sequence:source.sequence??offset+index+1,time,address:String(source.address??'').slice(0,512),status:String(source.status||(success?'Reply':'No response')).slice(0,1024),success,rtt,jitter,error:String(source.error||'').slice(0,2048),source};
    });
  }
  function summary(points){
    const successful=points.filter(point=>point.success),values=points.map(point=>point.rtt).filter(value=>value!=null),changes=points.map(point=>point.jitter).filter(value=>value!=null);
    return {count:points.length,received:successful.length,lost:points.length-successful.length,lossPercent:points.length?(points.length-successful.length)/points.length*100:0,min:values.length?Math.min(...values):null,max:values.length?Math.max(...values):null,average:values.length?values.reduce((a,b)=>a+b,0)/values.length:null,jitter:changes.length?changes.reduce((a,b)=>a+b,0)/changes.length:null,jitterPairs:changes.length};
  }
  function pointDetails(point,{compact=false}={}){
    if(!point)return '<span class="rt-instruction">Hover over a point for details. Click to keep it selected. Focus a chart and use the arrow keys to move between samples; Escape clears the selection.</span>';
    if(point.members)return '<dl class="rt-detail-values">'+[['Grouped samples',point.stats.count],['Interval start',localTime(point.time)],['Interval end',localTime(point.endTime)],['Unsuccessful',point.stats.lost],['Minimum RTT',number(point.stats.min)],['Average RTT (aggregate)',number(point.stats.average)],['Maximum RTT',number(point.stats.max)]].map(([name,value])=>'<div><dt>'+escape(name)+'</dt><dd>'+escape(value)+'</dd></div>').join('')+'</dl>';
    const clipped=(text,max)=>compact&&text.length>max?text.slice(0,max)+'…':text;
    const fields=[[timeLabel(),localTime(point.time)],['Sample',String(point.sequence)],['Result',clipped(point.status,160)],['Response time',number(point.rtt)],['RTT change',point.jitter==null?'No consecutive measured pair':number(point.jitter)]];
    if(point.address)fields.push(['Address',clipped(point.address,128)]);if(point.error)fields.push(['Error',clipped(point.error,240)]);
    return '<dl class="rt-detail-values">'+fields.map(([label,value])=>'<div><dt>'+escape(label)+'</dt><dd>'+escape(value)+'</dd></div>').join('')+'</dl>';
  }
  function selectedPoint(model){return model.pinned?(model.allPoints.find(point=>point.key===model.pinned)||model.pinnedSnapshot):null;}
  function detailsPoint(model){return selectedPoint(model)||model.allPoints.find(point=>point.key===model.hovered)||null;}
  function selectionContext(model){
    if(!model.pinned)return '';
    const retained=model.allPoints.some(point=>point.key===model.pinned),displayed=model.points.some(point=>point.key===model.pinned);
    if(!retained)return '<p class="rt-selected-context">Selected sample '+escape(model.pinnedSnapshot?.sequence)+' is no longer in retained history. Its recorded details remain selected.</p>';
    if(!displayed)return '<p class="rt-selected-context">Selected sample '+escape(selectedPoint(model)?.sequence)+' is outside the displayed time range. Its recorded details remain selected; use Fit or navigation to locate it.</p>';
    return '<p class="rt-selected-context">Sample '+escape(selectedPoint(model)?.sequence)+' stays selected until you select another point, click outside the graph or clear the selection.</p>';
  }
  const zoomValue=value=>Math.max(1e-18,Math.min(1e24,Number.isFinite(Number(value))?Number(value):1));
  const stepFor=model=>spacing*zoomValue(model.zoom);
  function filterState(model){try{return root.NetworkToolkitResultFilters?.current?.(model.id)||{active:false};}catch{return {active:false};}}
  function pointMatches(model,point){if(point.members)return point.members.some(item=>pointMatches(model,item));try{return root.NetworkToolkitResultFilters?.graphMatches?.(model.id,point.source||point)!==false;}catch{return true;}}
  function severity(point,model){
    if(point.members){const levels=point.members.map(item=>severity(item,model));return ['bad','warn','info','good'].find(level=>levels.includes(level))||'info';}
    try{const value=root.NetworkToolkitResultFilters?.severity?.(point.source||point,model.tool);if(['good','bad','warn','info'].includes(value))return value;}catch{}
    const text=String(point.status||'').toLowerCase();return /warn|partial/.test(text)?'warn':point.success?'good':/pending|queued|running|info/.test(text)?'info':'bad';
  }
  function latestPoint(model){const filtered=filterState(model).active,points=filtered?model.allPoints.filter(point=>pointMatches(model,point)):model.allPoints;return points.at(-1)||null;}
  function aggregate(group,index){
    if(!group.aggregate)return {...group.items[0],index};
    const stats=summary(group.items),first=group.items[0],last=group.items.at(-1);
    return {key:'interval:'+JSON.stringify([first.key,last.key]),index,sequence:String(first.sequence)+' to '+last.sequence,time:first.time,endTime:last.time,members:group.items,aggregate:true,success:stats.lost===0,status:'Condensed interval: '+stats.count+' samples, '+stats.lost+' unsuccessful',rtt:stats.average,jitter:stats.jitter,error:'',address:first.address,stats,source:{kind:'Condensed interval',time:first.time,recordedAt:last.time,samples:stats.count,received:stats.received,lost:stats.lost,minimumRttMs:stats.min,averageRttMs:stats.average,maximumRttMs:stats.max,meanRttChangeMs:stats.jitter}};
  }
  function selectPoints(model){
    const history=root.NetworkToolkitGraphHistory,pref=history?.preferences?.();
    let candidates=model.allPoints;
    if(history){
      let scope=history.range(candidates,pref,model.heldEnd||(filterState(model).active&&!(pref.allHistory&&model.zoom===1&&model.follow)?latestPoint(model)?.time||'':''));
      if(model.panSpanNs&&history.instant(scope.end)!==null)scope={...scope,start:history.timestamp(history.instant(scope.end)-BigInt(model.panSpanNs))};model.baseScope=scope;scope=model.manualScope?{...scope,...root.NetworkToolkitGraphNavigator.scope(model.manualScope.start,model.manualScope.end)}:history.viewport(scope,model.zoom,model.viewEnd||'');model.timeScope=scope;
      const timed=Number.isFinite(scope.from)&&Number.isFinite(scope.to),visible=timed?history.select(model.allPoints,scope):candidates,indices=new Set(visible.map(point=>point.index));
      model.rawVisiblePoints=visible.map(point=>({...point,jitter:indices.has(point.index-1)?point.jitter:null}));
      const groups=model.mode==='condensed'?history.condense(model.rawVisiblePoints,{end:scope.end,recentMs:pref.recentMs,preserveRaw:true}):model.rawVisiblePoints.map(point=>({items:[point],aggregate:false}));
      return groups.map(aggregate);
    }
    model.timeScope=null;model.rawVisiblePoints=candidates;return candidates.map((p,index)=>({...p,index}));
  }
  function refreshModel(model){
    model.points=selectPoints(model);model.stats=summary(model.rawVisiblePoints||model.points);const filter=filterState(model),reference=Number.isFinite(filter.reference)?filter.reference:null;
    model.rttScale=root.NetworkToolkitGraphHistory?.valueScale?.((model.rawVisiblePoints||[]).map(point=>point.rtt))||{lower:0,upper:Math.max(1,model.stats.max||0)*1.12};model.jitterScale=root.NetworkToolkitGraphHistory?.valueScale?.((model.rawVisiblePoints||[]).map(point=>point.jitter))||{lower:0,upper:Math.max(1,...model.points.map(point=>point.jitter||0))*1.12};model.maximum=model.rttScale.upper;model.maxJitter=model.jitterScale.upper;
  }
  function plotGeometry(model){const viewport=model.viewportWidth||width,scale=model.plot==='jitter'?model.jitterScale:model.rttScale,labels=root.NetworkToolkitGraphHistory?.scaleLabels?.(scale)||[],axisWidth=Math.max(48,...labels.map(label=>label.length*5.5+9));return {viewport,track:viewport,left:Math.min(axisWidth,Math.max(24,viewport*.4)),right:Math.max(Math.min(axisWidth,Math.max(24,viewport*.4))+1,viewport-padding),first:0,last:model.points.length,step:0};}
  function coordinate(index,model,geometry){
    const point=model.points[index],history=root.NetworkToolkitGraphHistory,share=history?.fraction?.(point?.time,model.timeScope,model.mode);
    return geometry.left+(Number.isFinite(share)?Math.max(0,Math.min(1,share)):index/Math.max(1,model.points.length-1))*(geometry.right-geometry.left);
  }
  function ordinate(value,scale,model){const h=model?.viewportHeight||height,top=Math.min(18,h*.42),bottom=Math.max(top+1,h-padding);return bottom-(value-scale.lower)/(scale.upper-scale.lower)*(bottom-top);}
  function zoomText(model){return root.NetworkToolkitGraphHistory?.scaleLabel?.(model.zoom)||String(model.zoom);}
  function visibleInterval(model){const history=root.NetworkToolkitGraphHistory,a=history?.instant?.(model.timeScope?.start),b=history?.instant?.(model.timeScope?.end);if(a==null||b==null)return 'Time not recorded';const ms=Number(b-a)/1000000;for(const [unit,size] of [['d',86400000],['h',3600000],['min',60000],['s',1000],['ms',1],['µs',.001],['ns',.000001]])if(ms>=size)return history.scaleLabel(ms/size)+' '+unit+' visible';return 'Single instant';}
  function referenceState(model){const filter=filterState(model),scale=model.plot==='jitter'?model.jitterScale:model.rttScale,relevant=model.plot==='jitter'?filter.statistic==='jitter':filter.statistic==='average',value=filter.reference;if(!relevant||!Number.isFinite(value)||value>=scale.lower&&value<=scale.upper)return {label:'',description:''};const direction=value>scale.upper?'above':'below';return {label:' · Ref '+(direction==='above'?'↑':'↓'),description:'Reference '+number(value)+' is '+direction+' the visible scale'};}
  function zoomControls(model){return '<div class="rt-zoom-controls graph-inline-zoom" role="group" aria-label="Graph time zoom"><button class="btn small ghost" type="button" data-rt-zoom="out" aria-label="Zoom out" title="Zoom out" '+(model.zoom<=minZoom?'disabled':'')+'>−</button><span data-rt-zoom-level title="'+escape(visibleInterval(model))+'">'+zoomText(model)+'×</span><button class="btn small ghost" type="button" data-rt-zoom="in" aria-label="Zoom in" title="Zoom in" '+(model.zoom>=maxZoom?'disabled':'')+'>+</button><button class="btn small ghost" type="button" data-rt-zoom="reset" aria-label="Fit all retained history" title="Fit all retained history">Fit</button></div>';}

  function plotContent(model,metric,maximum,average){
    const geometry=plotGeometry(model),viewWidth=geometry.viewport,scale=metric==='rtt'?model.rttScale:model.jitterScale,segments=[],marks=[];let segment=[],previousPoint=null;
    for(const point of model.points.slice(geometry.first,geometry.last)){
      const x=coordinate(point.index,model,geometry),value=point[metric],filter=filterState(model),classes=' rt-severity-'+severity(point,model)+(filter.active?(pointMatches(model,point)?' rt-point-filtered':' rt-point-filter-dimmed'):'')+(point.key===model.pinned?' rt-point-selected':'')+(point.key===model.hovered?' rt-point-hovered':'');
      if(previousPoint&&numeric(previousPoint.sequence)!=null&&numeric(point.sequence)!=null&&Number(point.sequence)!==Number(previousPoint.sequence)+1){if(segment.length)segments.push(segment);segment=[];}previousPoint=point;
      if(value==null){if(segment.length)segments.push(segment);segment=[];}else segment.push(x.toFixed(2)+','+ordinate(value,scale,model).toFixed(2));
      const attrs=' data-rt-point="'+point.index+'" data-rt-key="'+escape(point.key)+'" data-rt-sequence="'+escape(point.sequence)+'"';
      if(point.aggregate){const values=point.members.map(member=>member[metric]).filter(value=>value!=null);if(values.length)marks.push('<line class="rt-aggregate-range rt-severity-'+severity(point,model)+'" x1="'+x+'" x2="'+x+'" y1="'+ordinate(Math.min(...values),scale,model)+'" y2="'+ordinate(Math.max(...values),scale,model)+'"/>');}
      const y=value==null?8:ordinate(value,scale,model);point.plotX=x;point.plotY=y;const radius=point.aggregate?5:3.5;
      // Missing measurements occupy a separate status lane; they are not zero RTT.
      marks.push('<g class="rt-point'+classes+(value==null?' rt-unmeasured':'')+'"'+attrs+(point.aggregate?' data-rt-aggregate="'+point.members.length+'"':'')+'><circle class="rt-point-hit" cx="'+x.toFixed(2)+'" cy="'+y.toFixed(2)+'" r="9"/><circle data-rt-marker cx="'+x.toFixed(2)+'" cy="'+y.toFixed(2)+'" r="'+radius+'"/>'+(value==null?'<title>'+escape(point.status)+': '+(metric==='rtt'?'response time not measured':'no consecutive measured pair')+'</title>':'')+'</g>');
    }
    if(segment.length)segments.push(segment);
    const lines=segments.filter(segment=>segment.length>1).map(segment=>'<polyline points="'+segment.join(' ')+'"/>').join('');
    const mean=average==null?'':'<line class="rt-average" x1="'+geometry.left+'" x2="'+geometry.right+'" y1="'+ordinate(average,scale,model).toFixed(2)+'" y2="'+ordinate(average,scale,model).toFixed(2)+'"/>';
    const filter=filterState(model),reference=Number.isFinite(filter.reference)?filter.reference:null,referenceMetric=metric==='rtt'&&filter.statistic==='average'||metric==='jitter'&&filter.statistic==='jitter',runReference=referenceMetric&&reference!=null?'<line class="rt-stat-reference" data-rt-stat-reference data-rt-reference-outside="'+(reference<scale.lower?'below':reference>scale.upper?'above':'')+'" x1="'+geometry.left+'" x2="'+geometry.right+'" y1="'+ordinate(Math.max(scale.lower,Math.min(scale.upper,reference)),scale,model).toFixed(2)+'" y2="'+ordinate(Math.max(scale.lower,Math.min(scale.upper,reference)),scale,model).toFixed(2)+'"><title>'+escape((filter.referenceKind==='run'?'Run':'Retained')+' '+(metric==='rtt'?'average RTT':'RTT variability')+': '+number(reference)+(reference<scale.lower?' (below the visible scale)':reference>scale.upper?' (above the visible scale)':''))+'</title></line>':'';
    const labels=root.NetworkToolkitGraphHistory?.scaleLabels?.(scale)||[String(scale.upper),String(scale.lower)],top=ordinate(scale.upper,scale,model),bottom=ordinate(scale.lower,scale,model),axis='<g class="rt-value-axis" aria-hidden="true"><text data-rt-y-label="upper" data-rt-y-value="'+scale.upper+'" x="'+(geometry.left-5)+'" y="'+(top+3)+'" text-anchor="end">'+escape(labels[0])+'</text><text data-rt-y-label="lower" data-rt-y-value="'+scale.lower+'" x="'+(geometry.left-5)+'" y="'+bottom+'" text-anchor="end">'+escape(labels[1])+'</text></g>';
    return '<path class="rt-grid" d="'+[0,.5,1].map(v=>'M'+geometry.left+' '+(top+v*(bottom-top))+'H'+geometry.right).join(' ')+'"/>'+axis+mean+runReference+'<g class="rt-series">'+lines+'</g><g class="rt-points" aria-hidden="true">'+marks.join('')+'</g>';
  }
  function filterDescription(model){
    const filter=filterState(model);if(!filter.active)return '';
    const name=filter.statistic==='average'?'average RTT':filter.statistic==='jitter'?'RTT variability':'';
    const reference=name&&Number.isFinite(filter.reference)?' '+(filter.referenceKind==='run'?'Run':'Retained')+' '+name+' reference: '+number(filter.reference)+'.':'';
    const explanation=typeof filter.message==='string'&&filter.message.trim()?filter.message.trim()+' ':'';
    return '<p class="rt-filter-note">'+escape(explanation)+'Matching samples are emphasized; other recorded samples remain visible for context.'+escape(reference)+'</p>';
  }
  function axisHTML(model){return root.NetworkToolkitGraphHistory?.axisHTML?.(model.timeScope,Math.max(1,plotGeometry(model).right-plotGeometry(model).left),model.mode)||'';}
  function plot(model,metric,maximum,average){
    const name=metric==='rtt'?'latency':'jitter';
    return '<div class="rt-plot rt-'+name+'" id="rt-active-'+encodeURIComponent(model.id)+'" role="tabpanel" aria-label="'+(metric==='rtt'?'Response time (RTT)':'RTT variability')+'"><div class="rt-axis-label" title="'+escape(visibleInterval(model)+' · '+(model.timeScope?.start||'')+' to '+(model.timeScope?.end||'')+' · '+referenceState(model).description)+'">'+(metric==='rtt'?'Response time':'Absolute change')+' · ms · '+escape(visibleInterval(model))+' · '+zoomText(model)+'×'+referenceState(model).label+((metric==='rtt'?model.rttScale:model.jitterScale).empty?' · Not measured':'')+'</div><svg data-rt-surface="'+name+'" data-rt-lower="'+(metric==='rtt'?model.rttScale:model.jitterScale).lower+'" data-rt-upper="'+(metric==='rtt'?model.rttScale:model.jitterScale).upper+'" data-rt-minimum="'+((metric==='rtt'?model.rttScale:model.jitterScale).minimum??'')+'" data-rt-maximum="'+((metric==='rtt'?model.rttScale:model.jitterScale).maximum??'')+'" data-rt-left="'+plotGeometry(model).left+'" tabindex="0" role="group" aria-label="'+(metric==='rtt'?'Response time':'Absolute RTT change between consecutive measured replies')+'. Drag to pan time. Use arrows to inspect, Enter to select, Escape to clear." viewBox="0 0 '+(model.viewportWidth||width)+' '+(model.viewportHeight||height)+'" preserveAspectRatio="xMidYMid meet">'+plotContent(model,metric,maximum,average)+'</svg></div>';
  }
  function navigatorHTML(model){
    const nav=root.NetworkToolkitGraphNavigator;if(!nav)return '';model.fullScope=nav.extent(model.allPoints);
    // Cache the bounded preview independently of viewport changes. Its Y range
    // uses the full recording, while each main plot fits its visible records.
    const stamp=[model.samples,model.plot,JSON.stringify(filterState(model))];
    if(!model.navigatorCache||stamp.some((v,i)=>v!==model.navigatorCache.stamp[i])){const preview=nav.geometry(model.fullScope,model.allPoints.map(p=>({time:p.time,value:p[model.plot],level:severity(p,model),match:pointMatches(model,p)})));model.navigatorCache={stamp,preview};}
    return nav.html('result:'+model.id,model.fullScope,model.timeScope,model.navigatorCache.preview,{label:'Full history',note:model.plot==='jitter'?'RTT variability; original consecutive sample pairs':'Response time; original retained measurements'});
  }
  function navigateWindow(model,scope){
    const h=root.NetworkToolkitGraphHistory,a=h.instant(scope.start),b=h.instant(scope.end),base=model.panSpanNs?BigInt(model.panSpanNs):h.instant(model.baseScope.end)-h.instant(model.baseScope.start);if(a===null||b===null||b<=a)return false;
    model.panSpanNs=String(base>0n?base:1000000000n);model.zoom=Number(BigInt(model.panSpanNs))/Number(b-a);model.manualScope={start:scope.start,end:scope.end};model.viewEnd=scope.end;model.follow=false;clearHover();const container=findContainer(model.id);if(container)rebuild(container,model);else refreshModel(model);return true;
  }
  function captureScroll(){} // Retained API call sites; history navigation now pans data.

  function latency(samples,context={}){
    if(!Array.isArray(samples)||!samples.length)return '';
    const id=String(context.id??'latency').slice(0,512),mode=preferences().mode;
    let model=models.get(id);
    if(!model)model={id,pinned:null,pinnedSnapshot:null,hovered:null,follow:true,scrollLeft:0,viewportWidth:width,viewportHeight:height,zoom:1};
    else captureScroll(model);
    const oldMode=model.mode;if(!model.renderingViewport)model.allPoints=normalize(samples);model.samples=samples;model.title=String(context.title||'Response time');model.tool=context.tool||context.job?.tool||model.tool||'monitor';model.context={...context,id,title:model.title};model.mode=mode;model.plot=plotName(plotPreferences()[model.tool]||model.plot);model.zoom=zoomValue(model.zoom);refreshModel(model);
    if(oldMode!==mode)model.scrollLeft=0;
    if(model.hovered&&!model.allPoints.some(point=>point.key===model.hovered)&&!model.points.some(point=>point.key===model.hovered))model.hovered=null;
    if(model.pinned){const refreshed=model.allPoints.find(point=>point.key===model.pinned);if(refreshed)model.pinnedSnapshot={...refreshed};}
    models.delete(id);models.set(id,model);while(models.size>64)models.delete(models.keys().next().value);
    const stats=model.stats;
    const scope=mode==='condensed'?'Condensed · nonlinear time axis; older intervals compressed':root.NetworkToolkitGraphHistory?.label?.()||'Retained measurements',metric=model.plot;
    return '<section class="rt-timeline" data-rt-id="'+escape(id)+'" data-rt-scope-start="'+escape(model.timeScope?.start||'')+'" data-rt-scope-end="'+escape(model.timeScope?.end||'')+'" data-rt-mode-active="'+mode+'" data-rt-plot-active="'+metric+'" aria-label="'+escape(model.title)+' timeline"><div class="rt-heading"><div class="rt-view-controls"><label class="rt-mode-control"><span class="sr-only">Graph view</span><select data-rt-mode aria-label="Graph view" title="Linear uses proportional recorded time; Condensed compresses older time. Timeframe, zoom and navigation control the displayed range."><option value="linear"'+(mode==='linear'?' selected':'')+'>Linear</option><option value="condensed"'+(mode==='condensed'?' selected':'')+'>Condensed</option></select></label>'+(root.NetworkToolkitGraphHistory?.controlHTML?.()||'')+zoomControls(model)+'<span class="sr-only" data-rt-selection-status></span></div></div>'+plot(model,metric,metric==='rtt'?model.maximum:model.maxJitter,metric==='rtt'?stats.average:stats.jitter)+'<div class="rt-time-axis" style="margin-left:'+plotGeometry(model).left+'px;margin-right:'+(plotGeometry(model).viewport-plotGeometry(model).right)+'px" aria-label="'+escape(timeLabel())+' horizontal time axis">'+axisHTML(model)+'</div>'+navigatorHTML(model)+'<div class="rt-plot-caption" title="'+escape(scope+' · '+(filterState(model).active?'Matching samples emphasized. ':'')+stats.count+' samples; dashed line: displayed average. Unmeasured values use the top status lane. Drag plot to pan time.')+'">'+escape(scope)+' · '+escape(root.NetworkToolkitResultTime?.zoneLabel?.()||'Local time')+'</div><div class="sr-only">'+filterDescription(model)+'</div><span class="sr-only rt-selection-announcement" role="status"></span></section>';
  }

  function findContainer(id){return [...(root.document?.querySelectorAll?.('.rt-timeline')||[])].find(item=>item.dataset.rtId===id)||null;}
  function contextFor(target){const container=target?.closest?.('.rt-timeline')||findContainer(target?.closest?.('[data-graph-workspace]')?.dataset.graphWorkspace);return container?{container,model:models.get(container.dataset.rtId)}:null;}
  function rowContext(target){
    const row=target?.closest?.('[data-result-graph-id][data-result-sample-key]');if(!row)return null;
    const model=models.get(row.dataset.resultGraphId),container=model?findContainer(model.id):null;
    return model&&container?{row,container,model,point:model.allPoints.find(point=>point.key===row.dataset.resultSampleKey)||null}:null;
  }
  function updateHighlights(context){
    const {container,model}=context,filter=filterState(model),heldMembers=new Set(selectedPoint(model)?.members?.map(p=>p.key)||[]);
    for(const mark of container.querySelectorAll('[data-rt-point]')){const point=model.points[Number(mark.dataset.rtPoint)],selected=model.pinned||model.liveDetails&&latestPoint(model)?.key;mark.classList.toggle('rt-point-selected',!!point&&(point.key===selected||heldMembers.has(point.key)||!!point.members?.some(p=>p.key===selected||heldMembers.has(p.key))));mark.classList.toggle('rt-point-hovered',!!point&&point.key===model.hovered);mark.classList.toggle('rt-point-filtered',!!point&&filter.active===true&&pointMatches(model,point));mark.classList.toggle('rt-point-filter-dimmed',!!point&&filter.active===true&&!pointMatches(model,point));}
    for(const row of root.document?.querySelectorAll?.('[data-result-graph-id][data-result-sample-key]')||[]){if(row.dataset.resultGraphId!==model.id)continue;row.classList.toggle('result-graph-selected',row.dataset.resultSampleKey===(model.pinned||model.liveDetails&&latestPoint(model)?.key)||!!selectedPoint(model)?.members?.some(p=>p.key===row.dataset.resultSampleKey));row.classList.toggle('result-graph-hovered',row.dataset.resultSampleKey===model.hovered);}
  }
  function presentSelection(model,point,following=false,force=false){
    if(!point)return;
    const retained=point.members?point.members.every(p=>model.allPoints.some(q=>q.key===p.key)):model.allPoints.some(p=>p.key===point.key),displayed=point.members?point.members.some(p=>root.NetworkToolkitGraphHistory.contains(p.time,model.timeScope)):model.points.some(p=>p.key===point.key||p.members?.some(q=>q.key===point.key)),matches=pointMatches(model,point);
    const message=(!retained?'This selection is retained as a snapshot. ':'')+(!displayed?'This result is outside the displayed graph range. ':'')+(!matches?'This result is outside the active result filters. ':'')+(following?'Following the latest matching result.':'Selection held. New results continue recording. Choose Follow latest to follow again.');
    root.NetworkToolkitResultDetails?.showGraphSelection?.({id:model.id,point,job:model.context?.job,message,following,force});
  }
  function renderSelection(context){
    if(!context?.model)return;const {container,model}=context;
    const follow=container.querySelector('[data-rt-follow]');if(follow)follow.setAttribute('aria-pressed',String(model.follow&&!model.pinned));
    const clear=container.querySelector('[data-rt-clear]');if(clear)clear.disabled=!model.pinned&&!model.liveDetails;
    const note=container.querySelector('[data-rt-selection-status]'),message=model.pinned?'Selection held. Follow latest resumes following.':model.liveDetails?'Following latest matching result.':'';if(note&&note.textContent!==message)note.textContent=message;
    root.NetworkToolkitGraphWorkspace?.refreshFollowState?.(model.id);
    updateHighlights(context);
    if(model.pinned)presentSelection(model,selectedPoint(model));else if(model.liveDetails&&model.follow)presentSelection(model,latestPoint(model),true);
  }
  function pointerPoint(event,context){
    const surface=event.target?.closest?.('[data-rt-surface]');if(!surface||!context?.model)return null;
    const mark=event.target.closest?.('[data-rt-point]'),bounds=surface.getBoundingClientRect(),actualWidth=bounds.width||bounds.right-bounds.left;
    if(!actualWidth||!Number.isFinite(event.clientX)||mark&&(event.clientX<bounds.left||event.clientX>bounds.left+actualWidth))return mark?context.model.points[Number(mark.dataset.rtPoint)]||null:null;
    const geometry=plotGeometry(context.model),x=(event.clientX-bounds.left)/actualWidth*geometry.viewport,h=bounds.height||bounds.bottom-bounds.top,y=Number.isFinite(event.clientY)&&h>0?(event.clientY-bounds.top)/h*(context.model.viewportHeight||height):null;
    let closest=null,distance=Infinity;for(const point of context.model.points){const dx=(point.plotX??coordinate(point.index,context.model,geometry))-x,dy=y==null?0:(point.plotY??0)-y,delta=dx*dx+dy*dy;if(delta<distance){distance=delta;closest=point;}}return closest;
  }

  function hideTooltip(){if(tooltip){tooltip.hidden=true;tooltip.setAttribute('aria-hidden','true');}}
  function showTooltip(point,event,model){
    const document=root.document;if(!document?.createElement||!document.body)return;
    if(!tooltip||!tooltip.isConnected){tooltip=document.createElement('div');tooltip.className='rt-tooltip';tooltip.id='networkToolkitGraphTooltip';tooltip.dataset.rtTooltip='';tooltip.setAttribute('role','tooltip');document.body.append(tooltip);}
    const content='<strong>'+(point.members?'Condensed interval':'Sample '+escape(point.sequence))+'</strong>'+pointDetails(point,{compact:true})+(model&&!model.points.some(p=>p.key===point.key||p.members?.some(m=>m.key===point.key))?'<p>Outside the displayed graph range. Recorded details are shown.</p>':'');
    if(tooltip._rtContent!==content){tooltip.innerHTML=content;tooltip._rtContent=content;}tooltip.hidden=false;tooltip.setAttribute('aria-hidden','false');
    const anchor=event.target?.getBoundingClientRect?.()||{left:0,right:0,top:0,bottom:0},x=Number.isFinite(event.clientX)?event.clientX:(anchor.left+anchor.right)/2,y=Number.isFinite(event.clientY)?event.clientY:anchor.top;
    const bounds=tooltip.getBoundingClientRect(),w=bounds.width||Math.max(0,bounds.right-bounds.left)||300,h=bounds.height||Math.max(0,bounds.bottom-bounds.top)||180,vw=root.innerWidth||1024,vh=root.innerHeight||768;
    tooltip.style.left=Math.max(8,Math.min(vw-w-8,x+14))+'px';tooltip.style.top=Math.max(8,Math.min(vh-h-8,y+h+22<vh?y+16:y-h-12))+'px';
  }
  function inspection(model,on){
    const next=on===true||model.keyboardInspect===true;if(model.inspecting===next)return;
    model.inspecting=next;root.NetworkToolkitGraphWorkspace?.inspection?.(model.id,next);
  }
  function clearHover(){
    if(activeHover){const model=models.get(activeHover),container=findContainer(activeHover);if(model){model.hovered=null;model.hoverOrigin=null;model.rowUnavailable=null;inspection(model,false);if(container)renderSelection({container,model});}}activeHover=null;hoverPointer=null;hideTooltip();
  }
  function clearPin(model,container,{close=true}={}){model.pinned=null;model.pinnedSnapshot=null;model.keyboardInspect=false;model.heldEnd='';model.liveDetails=false;model.follow=(root.NetworkToolkitGraphHistory?.preferences?.().anchor||'latest')==='latest'&&!model.viewEnd;root.NetworkToolkitGraphWorkspace?.hold?.(model.id,false);if(close)root.NetworkToolkitResultDetails?.clearGraphSelection?.(model.id);inspection(model,model.hovered!=null);refreshModel(model);if(container)renderSelection({container,model});}
  function jumpLatest(id){
    const model=models.get(String(id)),container=model&&findContainer(model.id);if(!model)return false;
    if(root.NetworkToolkitGraphHistory?.preferences?.().allHistory&&model.zoom===1)model.panSpanNs='';clearHover();clearPin(model,container,{close:false});model.liveDetails=true;model.follow=true;model.viewEnd='';model.manualScope=null;model.windowEndKey=null;
    if(root.NetworkToolkitGraphHistory?.preferences?.().anchor!=='latest'&&root.NetworkToolkitGraphHistory)root.NetworkToolkitGraphHistory.setPreferences({anchor:'latest'},{persist:true});refreshModel(model);
    if(container?.isConnected)rebuild(container,model);root.NetworkToolkitGraphWorkspace?.latest?.(model.id,{linked:false});presentSelection(model,latestPoint(model),true,true);return !!latestPoint(model);
  }
  function zoom(id,action,{anchorShare,scope:anchorScope}={}){
    const model=models.get(String(id)),container=model&&findContainer(model.id),history=root.NetworkToolkitGraphHistory;if(!model)return null;
    if(action==='reset'){root.NetworkToolkitGraphNavigator.fit();return 1;}
    const next=Math.max(Math.min(minZoom,model.zoom),Math.min(Math.max(maxZoom,model.zoom),zoomValue(action==='in'?Math.min(Math.max(maxZoom,model.zoom),model.zoom*2):action==='out'?Math.max(minZoom,model.zoom/2):action)));if(next===model.zoom)return next;
    if(model.manualScope||Number.isFinite(anchorShare)){const scope=anchorScope||model.timeScope,end=Number.isFinite(anchorShare)?history.anchoredEnd(scope,model.zoom,next,anchorShare,model.mode):scope.end,span=history.instant(scope.end)-history.instant(scope.start),length=BigInt(Math.max(1,Math.round(Number(span)*model.zoom/next)));navigateWindow(model,{start:history.timestamp(history.instant(end)-length),end});}
    else{model.zoom=next;refreshModel(model);if(container)rebuild(container,model);}return model.zoom;
  }
  function pan(id,amount){
    const model=models.get(String(id)),h=root.NetworkToolkitGraphHistory;if(!model||!h||!model.timeScope)return false;
    const a=h.instant(model.timeScope.start),b=h.instant(model.timeScope.end);if(a===null||b===null||b<=a)return false;const shift=BigInt(Math.round(Number(b-a)*amount));return navigateWindow(model,{start:h.timestamp(a+shift),end:h.timestamp(b+shift)});
  }
  function revealPoint(context,point){
    const {model}=context,history=root.NetworkToolkitGraphHistory;if(!history||!Number.isFinite(history.epoch(point.time))||history.contains(point.time,model.timeScope))return;
    const span=history.instant(model.timeScope.end)-history.instant(model.timeScope.start),end=history.instant(point.time)+span/2n;navigateWindow(model,{start:history.timestamp(end-span),end:history.timestamp(end)});
  }

  function pinPoint(context,point){const model=context.model;model.pinned=point.key;model.pinnedSnapshot={...point};model.liveDetails=false;model.follow=false;model.windowEndKey=(model.rawVisiblePoints||model.points).at(-1)?.key||model.allPoints.at(-1)?.key;model.heldEnd=model.timeScope?.end||root.NetworkToolkitGraphHistory?.range?.(model.allPoints).end||'';root.NetworkToolkitGraphWorkspace?.hold?.(model.id,true,point.members?null:point.key);if(!point.members)root.NetworkToolkitGraphWorkspace?.revealSample?.(model.id,point.key);presentSelection(model,point,false,true);renderSelection(context);}
  function selectResult(id,key){const model=models.get(String(id)),container=model&&findContainer(model.id),selected=model&&selectedPoint(model),point=model?.allPoints.find(p=>p.key===key)||selected?.members?.find(p=>p.key===key)||(selected?.key===key?selected:null);if(!model||!point||!container)return false;pinPoint({container,model},point);revealPoint({container,model},point);return true;}
  function clearSelection(id){const model=models.get(String(id)),container=model&&findContainer(model.id);if(!model)return false;clearHover();clearPin(model,container);model.skipCaptureOnce=true;if(container)rebuild(container,model);return true;}
  function isHeld(id){return !!models.get(String(id))?.pinned;}
  function rebuild(container,model){
    const holder=root.document.createElement('div');model.renderingViewport=true;try{holder.innerHTML=latency(model.samples,model.context||{id:model.id,title:model.title});}finally{model.renderingViewport=false;}const replacement=holder.firstElementChild;if(!replacement)return;
    if(!root.NetworkToolkitGraphNavigator?.preserve?.(container,replacement)){container.before(replacement);container.remove();}restore();
  }
  function restoreContainer(container,model){
    const svg=container.querySelector('[data-rt-surface]'),measured=svg?.getBoundingClientRect?.();
    if(svg&&resizeObserver&&!observedScrollers.has(svg)){resizeObserver.observe(svg);observedScrollers.add(svg);}
    if(svg&&measured?.width>0&&measured?.height>0){
      model.viewportWidth=measured.width;model.viewportHeight=measured.height;
      const metric=model.plot,stamp=[model.viewportWidth,model.viewportHeight,model.points.length,model.points.at(-1)?.key,model.timeScope?.start,model.timeScope?.end,model.zoom,model.pinned,model.hovered,metric,JSON.stringify(model.rttScale),JSON.stringify(model.jitterScale),JSON.stringify(filterState(model))].join('|');
      if(svg.dataset.rtWindow!==stamp){const scale=metric==='rtt'?model.rttScale:model.jitterScale;for(const [name,value]of Object.entries({lower:scale.lower,upper:scale.upper,minimum:scale.minimum??'',maximum:scale.maximum??'',left:plotGeometry(model).left}))svg.setAttribute('data-rt-'+name,String(value));svg.setAttribute('viewBox','0 0 '+model.viewportWidth+' '+model.viewportHeight);svg.innerHTML=plotContent(model,metric,metric==='rtt'?model.maximum:model.maxJitter,metric==='rtt'?model.stats.average:model.stats.jitter);svg.dataset.rtWindow=stamp;}
    }
    const axis=container.querySelector('.rt-time-axis'),ticks=axisHTML(model);if(axis){axis.style.marginLeft=plotGeometry(model).left+'px';axis.style.marginRight=(plotGeometry(model).viewport-plotGeometry(model).right)+'px';if(axis.innerHTML!==ticks)axis.innerHTML=ticks;}
    const slot=container.closest('[data-graph-workspace]')?.querySelector('[data-graph-tabs-slot]'),tabs=tabsHTML(model.id);if(slot&&slot.innerHTML!==tabs)slot.innerHTML=tabs;
    root.NetworkToolkitGraphNavigator?.attach?.(container.querySelector('[data-graph-navigator]'),{full:model.fullScope,view:model.timeScope,change:scope=>navigateWindow(model,scope),begin:()=>{model.navigatorDragging=true;clearHover();},end:()=>{model.navigatorDragging=false;}});
    renderSelection({container,model});if(activeHover===model.id&&model.hovered&&hoverPointer){const point=model.allPoints.find(p=>p.key===model.hovered)||model.points.find(p=>p.key===model.hovered);if(point)showTooltip(point,hoverPointer,model);}
  }

  function restore(){
    if(restoring)return;restoring=true;
    try{if(activeHover){const model=models.get(activeHover),container=findContainer(activeHover),hidden=container?.closest?.('[hidden]')||container?.closest?.('[data-graph-pane]')?.hidden===true;
        const rowPresent=model?.hoverOrigin==='row'&&[...(root.document?.querySelectorAll?.('[data-result-graph-id][data-result-sample-key]')||[])].some(row=>row.dataset.resultGraphId===activeHover&&row.dataset.resultSampleKey===model.hovered);
        if(!model||!container||hidden||model.hoverOrigin==='row'&&!rowPresent||model.hoverOrigin==='graph'&&!model.points.some(point=>point.key===model.hovered))clearHover();}
      for(const model of models.values())if(model.keyboardInspect){const container=findContainer(model.id);if(!container||container.closest?.('[hidden]')||container.closest?.('[data-graph-pane]')?.hidden===true){model.keyboardInspect=false;inspection(model,false);}}
      for(const item of observedScrollers)if(!item.isConnected){resizeObserver?.unobserve(item);observedScrollers.delete(item);}
      for(const container of root.document?.querySelectorAll?.('.rt-timeline')||[]){const model=models.get(container.dataset.rtId);if(model)restoreContainer(container,model);}
      if(lastFocus&&!lastFocus.node.isConnected&&(!root.document.activeElement||root.document.activeElement===root.document.body)){const saved=lastFocus;lastFocus=null;const replacement=findContainer(saved.id)?.querySelector('[data-rt-surface="'+saved.surface+'"]');replacement?.focus({preventScroll:true});}
    }finally{restoring=false;}
  }
  function initialize(){
    const document=root.document;if(initialized||!document?.addEventListener)return;initialized=true;
    document.addEventListener('pointermove',event=>{
      const row=rowContext(event.target),context=row||contextFor(event.target),point=row?.point||pointerPoint(event,context);
      if(!point||!context?.model||context.model.navigatorDragging){clearHover();return;}
      // Revealing an offscreen row pans/rebuilds the plot and clears transient
      // hover. Apply this row's hover afterwards to the current plot container.
      if(row){context.model.rowUnavailable=null;if(!context.model.pinned)revealPoint(context,point);context.container=findContainer(context.model.id)||context.container;}
      if(activeHover&&activeHover!==context.model.id)clearHover();activeHover=context.model.id;hoverPointer={target:event.target,clientX:event.clientX,clientY:event.clientY};
      const changed=context.model.hovered!==point.key;context.model.hovered=point.key;context.model.hoverOrigin=row?'row':'graph';inspection(context.model,true);
      if(!row&&changed&&!context.model.pinned)context.model.rowUnavailable=root.NetworkToolkitGraphWorkspace?.revealSample?.(context.model.id,point.key)===false?point.key:null;
      if(changed||row)renderSelection(context);showTooltip(point,event,context.model);
    });
    document.addEventListener('pointerout',event=>{
      const context=rowContext(event.target)||contextFor(event.target);if(!context)return;
      const related=event.relatedTarget;
      if(related&&(related.closest?.('[data-rt-surface]')===event.target.closest?.('[data-rt-surface]')&&event.target.closest?.('[data-rt-surface]')||rowContext(related)?.row===rowContext(event.target)?.row&&rowContext(event.target)?.row))return;
      clearHover();
    });
    document.addEventListener('click',event=>{
      const context=contextFor(event.target);
      if(activeHover&&activeHover!==context?.model?.id)clearHover();

      if(!context?.model)return;
      const tab=event.target.closest?.('[data-rt-tab]');if(tab){setPlot(context.model.id,tab.dataset.rtTab);findContainer(context.model.id)?.closest('[data-graph-workspace]')?.querySelector('[data-rt-tab="'+tab.dataset.rtTab+'"]')?.focus({preventScroll:true});return;}
      const panControl=event.target.closest?.('[data-rt-pan]');if(panControl){pan(context.model.id,Number(panControl.dataset.rtPan));return;}
      if(event.target.closest?.('[data-rt-clear]')){clearHover();clearPin(context.model,context.container);return;}
      const zoomControl=event.target.closest?.('[data-rt-zoom]');if(zoomControl){const action=zoomControl.dataset.rtZoom;zoom(context.model.id,action);findContainer(context.model.id)?.querySelector('[data-rt-zoom="'+action+'"]')?.focus?.({preventScroll:true});return;}
      if(event.target.closest?.('[data-rt-latest]')||event.target.closest?.('[data-rt-follow]')){jumpLatest(context.model.id);return;}
      if(context.model.dragged){context.model.dragged=false;return;}const point=event.target.closest?.('[data-rt-point]')?pointerPoint(event,context):null;if(point)pinPoint(context,point);else if(event.target.matches?.('[data-rt-surface]'))clearSelection(context.model.id);
    });
    document.addEventListener('change',event=>{if(!event.target.matches?.('[data-rt-mode]'))return;setPreferences({mode:event.target.value});for(const container of [...document.querySelectorAll('.rt-timeline')]){const model=models.get(container.dataset.rtId);if(model)rebuild(container,model);}hideTooltip();});
    document.addEventListener('keydown',event=>{
      const context=contextFor(event.target),surface=event.target.closest?.('[data-rt-surface]');if(!context?.model||!surface||!['ArrowLeft','ArrowRight','Home','End','Enter',' ','Escape'].includes(event.key))return;
      event.preventDefault();event.stopPropagation();
      if(event.key==='Escape'){context.model.hovered=null;clearHover();clearPin(context.model,context.container);return;}
      const points=context.model.points,current=points.findIndex(point=>point.key===(context.model.pinned||context.model.hovered)),index=event.key==='Home'?0:event.key==='End'?points.length-1:event.key==='ArrowLeft'?Math.max(0,(current<0?points.length:current)-1):event.key==='ArrowRight'?Math.min(points.length-1,current+1):Math.max(0,current);
      if(!points[index])return;context.model.keyboardInspect=true;inspection(context.model,true);pinPoint(context,points[index]);revealPoint(context,points[index]);
    });
    let panning=null;
    document.addEventListener('pointerdown',event=>{const surface=event.target.closest?.('[data-rt-surface]'),context=contextFor(surface);if(!surface||!context?.model||event.button!==0)return;const geometry=plotGeometry(context.model),bounds=surface.getBoundingClientRect(),left=bounds.left+geometry.left,width=geometry.right-geometry.left;panning={context,surface,id:event.pointerId,x:event.clientX,left,width,share:(event.clientX-left)/width,scope:{...context.model.timeScope},mode:context.model.mode};});
    document.addEventListener('pointerup',event=>{if(!panning||panning.id!==event.pointerId)return;const p=panning;panning=null;const distance=event.clientX-p.x;if(Math.abs(distance)>8&&p.width>0){p.context.model.dragged=true;clearHover();navigateWindow(p.context.model,root.NetworkToolkitGraphHistory.panScope(p.scope,p.share,(event.clientX-p.left)/p.width,p.mode));root.setTimeout?.(()=>{p.context.model.dragged=false;},0);}});
    document.addEventListener('pointercancel',()=>{panning=null;});
    document.addEventListener('keydown',event=>{const tab=event.target.closest?.('[data-rt-tab]'),context=contextFor(tab);if(!context?.model||!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;event.preventDefault();const next=event.key==='Home'?'rtt':event.key==='End'?'jitter':context.model.plot==='rtt'?'jitter':'rtt';setPlot(context.model.id,next);findContainer(context.model.id)?.closest('[data-graph-workspace]')?.querySelector('[data-rt-tab="'+next+'"]')?.focus({preventScroll:true});});
    document.addEventListener('wheel',event=>{
      const surface=event.target.closest?.('[data-rt-surface]'),context=contextFor(surface);if(!surface||!context?.model)return;const model=context.model,history=root.NetworkToolkitGraphHistory;if(!history)return;const pending=wheelFrames.get(model.id),next=history.wheelLevel(event,pending?.next??model.zoom,Math.min(minZoom,model.zoom),Math.max(maxZoom,model.zoom));if(next===(pending?.next??model.zoom))return;event.preventDefault();const bounds=surface.getBoundingClientRect(),geometry=plotGeometry(model),share=Math.max(0,Math.min(1,(event.clientX-bounds.left-geometry.left)/(geometry.right-geometry.left)));
      if(pending){pending.next=next;return;}const frame={next,share,scope:{...model.timeScope}};wheelFrames.set(model.id,frame);const apply=()=>{wheelFrames.delete(model.id);if(findContainer(model.id))zoom(model.id,frame.next,{anchorShare:frame.share,scope:frame.scope});};if(root.requestAnimationFrame)root.requestAnimationFrame(apply);else apply();
    },{passive:false});
    document.addEventListener('focusin',event=>{const surface=event.target.closest?.('[data-rt-surface]'),context=contextFor(event.target);if(surface&&context)lastFocus={node:surface,id:context.model.id,surface:surface.dataset.rtSurface};else lastFocus=null;});
    document.addEventListener('focusout',event=>{const context=contextFor(event.target);if(context&&event.target.closest?.('[data-rt-surface]')&&event.relatedTarget){context.model.keyboardInspect=false;inspection(context.model,context.model.hovered!=null);}if(lastFocus&&event.target===lastFocus.node&&event.relatedTarget)lastFocus=null;});
    document.addEventListener('toggle',event=>{if(!event.target.matches?.('.rt-variability'))return;const context=contextFor(event.target);if(context?.model)context.model.variabilityOpen=event.target.open===true;},true);
    document.addEventListener('networkToolkitGraphTimeChange',event=>{for(const [key,model] of [...models]){
      const wasFollowing=model.follow&&!model.pinned;model.viewEnd='';model.panSpanNs='';model.manualScope=null;if(event.detail?.fit)model.zoom=1;
      if(event.detail?.fitAll){model.heldEnd='';model.follow=wasFollowing;if(!wasFollowing){const full=root.NetworkToolkitGraphNavigator.extent(model.allPoints);if(root.NetworkToolkitGraphHistory.instant(full.start)!==null)model.manualScope=full;}}
      else if(!model.pinned){model.follow=root.NetworkToolkitGraphHistory.preferences().anchor==='latest';if(!model.follow){model.liveDetails=false;root.NetworkToolkitResultDetails?.clearGraphSelection?.(key);}}
      const container=findContainer(key);if(container){model.skipCaptureOnce=true;rebuild(container,model);}
    }});
    document.addEventListener('networkToolkitResultFilterChange',event=>{const id=event.detail?.jobId;for(const [key,model] of [...models]){if(id!=null&&String(id)!==key)continue;const container=findContainer(key);if(container){refreshModel(model);model.skipCaptureOnce=true;rebuild(container,model);}}});
    root.addEventListener?.('resize',()=>{hideTooltip();restore();});
    if(typeof root.ResizeObserver==='function')resizeObserver=new root.ResizeObserver(restore);
    if(typeof root.MutationObserver==='function')new root.MutationObserver(restore).observe(document.documentElement||document,{subtree:true,childList:true});
  }
  const api=Object.freeze({latency,normalize,summary,pointDetails,sampleKey,preferences,normalizePreferences,setPreferences,mergePreferences,tabsHTML,setPlot,tracking,pan,zoom,jumpLatest,navigate:(id,scope)=>{const model=models.get(String(id));return model?navigateWindow(model,scope):false;},selectResult,clearSelection,isHeld,restore,initialize});
  if(typeof module==='object'&&module.exports)module.exports=api;root.NetworkToolkitResultTimeline=api;initialize();
})(typeof window!=='undefined'?window:globalThis);
