(function (root) {
  'use strict';

  const limit=180, retainedLimit=10000, width=820, height=110, padding=9, spacing=8, minZoom=.25, maxZoom=16;
  const models=new Map();
  let initialized=false,lastFocus=null,activeHover=null,hoverPointer=null,tooltip=null,restoring=false,resizeObserver=null;
  const observedScrollers=new Set();
  let fallbackPreferences={mode:'window'};
  const escape=value=>String(value==null?'':value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const numeric=value=>(typeof value==='number'||typeof value==='string'&&value.trim())&&Number.isFinite(Number(value))&&Number(value)>=0?Number(value):null;
  const numberFormatter=new Intl.NumberFormat(undefined,{maximumFractionDigits:3}),lossFormatter=new Intl.NumberFormat(undefined,{maximumFractionDigits:1});
  const number=value=>value==null?'Not measured':numberFormatter.format(value)+' ms';
  function normalizePreferences(value){return {mode:['window','progressive','condensed'].includes(value?.mode)?value.mode:'window'};}
  function preferences(){try{return normalizePreferences(typeof state!=='undefined'?state.graphPreferences:fallbackPreferences);}catch{return normalizePreferences(fallbackPreferences);}}
  function mergePreferences(value){if(!value||!Object.prototype.hasOwnProperty.call(value,'graphPreferences'))return preferences();const next=normalizePreferences(value.graphPreferences);fallbackPreferences=next;try{if(typeof state!=='undefined')state.graphPreferences=next;}catch{}return next;}
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
    const count=options.mode==='progressive'?retainedLimit:limit,offset=Math.max(0,samples.length-count);let previous=null,previousSequence=null;
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
    if(!displayed)return '<p class="rt-selected-context">Selected sample '+escape(selectedPoint(model)?.sequence)+' is outside the moving window. Its recorded details remain selected; switch to progressive history to locate it.</p>';
    return '<p class="rt-selected-context">Sample '+escape(selectedPoint(model)?.sequence)+' stays selected until you select another point, click outside the graph or clear the selection.</p>';
  }
  const zoomValue=value=>Math.max(minZoom,Math.min(maxZoom,Number.isFinite(Number(value))?Number(value):1));
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
    if(history&&(model.mode==='condensed'||pref.durationMs>0||pref.anchor!=='latest')){
      let scope=history.range(model.allPoints,pref,model.heldEnd||(filterState(model).active?latestPoint(model)?.time||'':''));
      if(model.zoom!==1&&Number.isFinite(scope.from)&&Number.isFinite(scope.to)&&scope.to>scope.from){scope={...scope,start:history.shift(scope.end,-Math.round((scope.to-scope.from)/model.zoom))};scope.from=history.epoch(scope.start);}
      model.timeScope=scope;const visible=history.select(model.allPoints,scope),indices=new Set(visible.map(point=>point.index));model.rawVisiblePoints=visible.map(point=>({...point,jitter:indices.has(point.index-1)?point.jitter:null}));
      const groups=model.mode==='condensed'?history.condense(model.rawVisiblePoints,{end:scope.end,recentMs:pref.recentMs}):model.rawVisiblePoints.map(point=>({items:[point],aggregate:false}));
      return groups.map(aggregate);
    }
    model.timeScope=null;
    if(model.mode==='progressive'){model.rawVisiblePoints=model.allPoints;return model.allPoints;}
    const count=Math.min(retainedLimit,Math.max(5,Math.round(limit/zoomValue(model.zoom))));
    let end=model.allPoints.length-1;
    const target=model.follow?latestPoint(model):model.allPoints.find(point=>point.key===model.windowEndKey);
    if(target)end=target.index;
    const start=Math.max(0,end-count+1);model.rawVisiblePoints=model.allPoints.slice(start,end+1).map((point,index)=>({...point,index,jitter:index?point.jitter:null}));return model.rawVisiblePoints;
  }
  function refreshModel(model){
    model.points=selectPoints(model);model.stats=summary(model.rawVisiblePoints||model.points);const filter=filterState(model),reference=Number.isFinite(filter.reference)?filter.reference:null;
    model.maximum=Math.max(1,model.stats.max||0,filter.statistic==='average'?reference||0:0)*1.12;
    model.maxJitter=Math.max(1,...model.points.map(point=>point.jitter||0),filter.statistic==='jitter'?reference||0:0)*1.12;
  }
  function plotGeometry(model){
    const viewport=Math.max(1,model.viewportWidth||width),step=stepFor(model),track=model.mode==='progressive'?Math.max(viewport,padding*2+(model.points.length-1)*step):viewport;
    const left=model.mode==='progressive'?Math.max(0,Math.min(model.scrollLeft||0,track-viewport)):0;
    const first=model.mode==='progressive'?Math.max(0,Math.floor((left-padding)/step)-1):0;
    const last=model.mode==='progressive'?Math.min(model.points.length,Math.ceil((left+viewport-padding)/step)+2):model.points.length;
    return {viewport,track,left,first,last,step};
  }
  function axisPoints(model){if(model.timeScope&&model.mode!=='progressive')return [{time:model.timeScope.start},{time:model.timeScope.end}];if(model.mode!=='progressive')return [model.points[0],model.points.at(-1)];const geometry=plotGeometry(model),first=Math.max(0,Math.min(model.points.length-1,Math.ceil((geometry.left-padding)/geometry.step))),last=Math.max(first,Math.min(model.points.length-1,Math.floor((geometry.left+geometry.viewport-padding)/geometry.step)));return [model.points[first],model.points[last]];}
  function coordinate(index,model,geometry){
    if(model.mode==='progressive')return padding+index*geometry.step-geometry.left;
    const scope=model.timeScope,point=model.points[index],history=root.NetworkToolkitGraphHistory;
    if(scope&&point&&scope.to>scope.from){const time=history.epoch(point.time),age=Math.max(0,scope.to-time),span=scope.to-scope.from,recent=history.preferences().recentMs,share=model.mode==='condensed'?1-Math.log1p(age/recent)/Math.log1p(span/recent):(time-scope.from)/span;return padding+Math.max(0,Math.min(1,share))*(width-2*padding);}
    return padding+index*(width-2*padding)/Math.max(1,model.points.length-1);
  }
  function ordinate(value,maximum){return height-padding-value/maximum*(height-2*padding);}
  function plotContent(model,metric,maximum,average){
    const geometry=plotGeometry(model),viewWidth=model.mode==='progressive'?geometry.viewport:width,segments=[],marks=[];let segment=[],previousPoint=null;
    for(const point of model.points.slice(geometry.first,geometry.last)){
      const x=coordinate(point.index,model,geometry),value=point[metric],filter=filterState(model),classes=' rt-severity-'+severity(point,model)+(filter.active?(pointMatches(model,point)?' rt-point-filtered':' rt-point-filter-dimmed'):'')+(point.key===model.pinned?' rt-point-selected':'')+(point.key===model.hovered?' rt-point-hovered':'');
      if(previousPoint&&numeric(previousPoint.sequence)!=null&&numeric(point.sequence)!=null&&Number(point.sequence)!==Number(previousPoint.sequence)+1){if(segment.length)segments.push(segment);segment=[];}previousPoint=point;
      if(value==null){if(segment.length)segments.push(segment);segment=[];}else segment.push(x.toFixed(2)+','+ordinate(value,maximum).toFixed(2));
      const attrs=' data-rt-point="'+point.index+'" data-rt-key="'+escape(point.key)+'" data-rt-sequence="'+escape(point.sequence)+'"';
      if(point.aggregate&&metric==='rtt'&&point.stats.min!=null)marks.push('<line class="rt-aggregate-range rt-severity-'+severity(point,model)+'" x1="'+x+'" x2="'+x+'" y1="'+ordinate(point.stats.min,maximum)+'" y2="'+ordinate(point.stats.max,maximum)+'"/>');
      if(metric==='rtt'&&!point.success&&!point.aggregate)marks.push('<g class="rt-point rt-missed'+classes+'"'+attrs+'><rect x="'+(x-3).toFixed(2)+'" y="3" width="6" height="104"/><path d="M'+(x-3).toFixed(2)+' 5l6 6m-6 0l6-6"/></g>');
      else if(value!=null)marks.push('<circle class="rt-point'+classes+'"'+attrs+' cx="'+x.toFixed(2)+'" cy="'+ordinate(value,maximum).toFixed(2)+'" r="'+(point.aggregate?5:3)+'"'+(point.aggregate?' data-rt-aggregate="'+point.members.length+'"':'')+'/>');
      if(point.aggregate&&value==null)marks.push('<rect class="rt-point rt-aggregate'+classes+'"'+attrs+' data-rt-aggregate="'+point.members.length+'" x="'+(x-4)+'" y="4" width="8" height="8"/>');
    }
    if(segment.length)segments.push(segment);
    const lines=segments.filter(segment=>segment.length>1).map(segment=>'<polyline points="'+segment.join(' ')+'"/>').join('');
    const mean=average==null?'':'<line class="rt-average" x1="0" x2="'+viewWidth+'" y1="'+ordinate(average,maximum).toFixed(2)+'" y2="'+ordinate(average,maximum).toFixed(2)+'"/>';
    const filter=filterState(model),reference=Number.isFinite(filter.reference)?filter.reference:null,referenceMetric=metric==='rtt'&&filter.statistic==='average'||metric==='jitter'&&filter.statistic==='jitter',runReference=referenceMetric&&reference!=null?'<line class="rt-stat-reference" data-rt-stat-reference x1="0" x2="'+viewWidth+'" y1="'+ordinate(reference,maximum).toFixed(2)+'" y2="'+ordinate(reference,maximum).toFixed(2)+'"><title>'+escape((filter.referenceKind==='run'?'Run':'Retained')+' '+(metric==='rtt'?'average RTT':'RTT variability')+': '+number(reference))+'</title></line>':'';
    return '<path class="rt-grid" d="M0 27H'+viewWidth+' M0 55H'+viewWidth+' M0 82H'+viewWidth+'"/>'+mean+runReference+'<g class="rt-series">'+lines+'</g><g class="rt-points" aria-hidden="true">'+marks.join('')+'</g>';
  }
  function filterDescription(model){
    const filter=filterState(model);if(!filter.active)return '';
    const name=filter.statistic==='average'?'average RTT':filter.statistic==='jitter'?'RTT variability':'';
    const reference=name&&Number.isFinite(filter.reference)?' '+(filter.referenceKind==='run'?'Run':'Retained')+' '+name+' reference: '+number(filter.reference)+'.':'';
    const explanation=typeof filter.message==='string'&&filter.message.trim()?filter.message.trim()+' ':'';
    return '<p class="rt-filter-note">'+escape(explanation)+'Matching samples are emphasized; other recorded samples remain visible for context.'+escape(reference)+'</p>';
  }
  function plot(model,metric,maximum,average){
    const name=metric==='rtt'?'latency':'jitter',geometry=plotGeometry(model),progressive=model.mode==='progressive';
    const svg='<svg data-rt-surface="'+name+'" tabindex="0" role="group" aria-label="'+(metric==='rtt'?'Response time':'Absolute RTT change between consecutive measured replies')+'. Use Left and Right arrows to inspect samples, Home and End to jump, Enter to select, Escape to clear." viewBox="0 0 '+(progressive?geometry.viewport:width)+' '+height+'" preserveAspectRatio="none">'+plotContent(model,metric,maximum,average)+'</svg>';
    return '<div class="rt-plot rt-'+name+'"><div class="rt-axis-label">'+escape(number(maximum))+'</div>'+(progressive?'<div class="rt-scroll" data-rt-scroll="'+name+'" tabindex="0" aria-label="Scroll '+name+' retained history"><div class="rt-track" style="width:'+geometry.track+'px"><div class="rt-visible-plot" style="width:'+geometry.viewport+'px">'+svg+'</div></div></div>':svg)+'</div>';
  }
  function capturePendingScroll(model,scroll){
    if(!(scroll?.clientWidth>0)||!Number.isFinite(scroll.scrollLeft)||!Number.isFinite(scroll._rtAppliedLeft)||Math.abs(scroll.scrollLeft-scroll._rtAppliedLeft)<=.5)return false;
    model.scrollLeft=scroll.scrollLeft;model.follow=!model.pinned&&Math.max(0,(scroll.scrollWidth||0)-scroll.clientWidth)-model.scrollLeft<=12;scroll._rtAppliedLeft=model.scrollLeft;return true;
  }
  function captureScroll(model){
    if(model.skipCaptureOnce){model.skipCaptureOnce=false;return;}
    const container=findContainer(model.id),scroll=container?.querySelector('[data-rt-scroll="latency"]');
    if(!scroll||model.mode!=='progressive'||!(scroll.clientWidth>0)||!Number.isFinite(scroll.scrollLeft))return;
    capturePendingScroll(model,scroll);model.scrollLeft=scroll.scrollLeft;
  }
  function latency(samples,context={}){
    if(!Array.isArray(samples)||!samples.length)return '';
    const id=String(context.id??'latency').slice(0,512),mode=preferences().mode;
    let model=models.get(id);
    if(!model)model={id,pinned:null,pinnedSnapshot:null,hovered:null,follow:true,scrollLeft:0,viewportWidth:width,zoom:1};
    else captureScroll(model);
    const oldMode=model.mode;model.allPoints=normalize(samples,{mode:'progressive'});model.samples=samples;model.title=String(context.title||'Response time');model.tool=context.tool||context.job?.tool||model.tool||'monitor';model.context={...context,id,title:model.title};model.mode=mode;model.zoom=zoomValue(model.zoom);refreshModel(model);
    if(oldMode!==mode){model.scrollLeft=0;if(!model.pinned)model.follow=true;}
    if(model.hovered&&!model.allPoints.some(point=>point.key===model.hovered)&&!model.points.some(point=>point.key===model.hovered))model.hovered=null;
    if(model.pinned){const refreshed=model.allPoints.find(point=>point.key===model.pinned);if(refreshed)model.pinnedSnapshot={...refreshed};}
    if(mode==='progressive'&&model.follow)model.scrollLeft=followPosition(model);
    models.delete(id);models.set(id,model);while(models.size>64)models.delete(models.keys().next().value);
    refreshModel(model);const stats=model.stats;
    const windowLabel=mode==='condensed'?'Condensed history (groups are aggregate measurements)':model.timeScope?root.NetworkToolkitGraphHistory.label():mode==='progressive'?'Progressive retained history':samples.length>model.points.length?(model.follow?'Latest ':'Displayed ')+model.points.length+' retained samples':'Displayed samples',loss=lossFormatter.format(stats.lossPercent);
    return '<section class="rt-timeline" data-rt-id="'+escape(id)+'" data-rt-mode-active="'+mode+'" aria-label="'+escape(model.title)+' timeline"><div class="rt-heading"><h3>'+escape(model.title)+'</h3><span>'+escape(windowLabel)+' ('+model.points.length+')</span><div class="rt-view-controls"><label>Graph view <select data-rt-mode aria-label="Graph view"><option value="window"'+(mode==='window'?' selected':'')+'>Moving window</option><option value="progressive"'+(mode==='progressive'?' selected':'')+'>Progressive history</option><option value="condensed"'+(mode==='condensed'?' selected':'')+'>Condensed history</option></select></label>'+ (root.NetworkToolkitGraphHistory?.controlHTML?.()||'') +'<button type="button" class="btn small ghost" data-rt-clear '+(model.pinned||model.liveDetails?'':'disabled')+'>Clear selection</button><span class="sr-only" data-rt-selection-status></span><div class="rt-zoom-controls" role="group" aria-label="Graph time zoom"><button type="button" class="btn small ghost" data-rt-zoom="out" aria-label="Zoom graph out" '+(model.zoom<=minZoom?'disabled':'')+'>-</button><span data-rt-zoom-level aria-live="polite">'+model.zoom+'x</span><button type="button" class="btn small ghost" data-rt-zoom="in" aria-label="Zoom graph in" '+(model.zoom>=maxZoom?'disabled':'')+'>+</button><button type="button" class="btn small ghost" data-rt-zoom="reset">Reset zoom</button></div><button type="button" class="btn small ghost" data-rt-latest>Latest</button>'+(mode==='progressive'?'<button type="button" class="btn small ghost" data-rt-follow aria-pressed="'+model.follow+'">Follow latest</button>':'')+'</div></div>'+(model.points.length?plot(model,'rtt',model.maximum,stats.average):'<p class="help-line">No recorded measurements in the displayed time range.</p>')+'<div class="rt-time-axis"><time>'+escape(localTime(axisPoints(model)[0]?.time))+'</time><time>'+escape(localTime(axisPoints(model)[1]?.time))+'</time></div>'+filterDescription(model)+'<div class="rt-window-stats" aria-label="Statistics for displayed samples"><span><strong>'+stats.received+'/'+stats.count+'</strong> successful</span><span><strong>'+loss+'%</strong> unsuccessful</span><span>Minimum <strong>'+escape(number(stats.min))+'</strong></span><span>Average <strong>'+escape(number(stats.average))+'</strong></span><span>Maximum <strong>'+escape(number(stats.max))+'</strong></span></div><div class="rt-loss-bar" role="img" aria-label="Displayed samples: '+stats.received+' successful, '+stats.lost+' unsuccessful"><span class="rt-success-share" style="width:'+(stats.count?(stats.count-stats.lost)/stats.count*100:0).toFixed(4)+'%"></span><span class="rt-loss-share" style="width:'+stats.lossPercent.toFixed(4)+'%"></span></div><div class="rt-loss-legend"><span>Successful: '+stats.received+'</span><span>Unsuccessful: '+stats.lost+'</span><span>Dashed line: displayed average RTT</span></div><details class="rt-variability"'+(model.variabilityOpen?' open':'')+'><summary>RTT variability <span>'+escape(number(stats.jitter))+' mean absolute change</span></summary><p>Absolute RTT change between consecutive successful measured replies in this displayed window. A missing or unsuccessful probe breaks the pair.</p>'+(stats.jitterPairs?plot(model,'jitter',model.maxJitter,stats.jitter):'<p>No consecutive measured reply pairs are available.</p>')+'</details><span class="sr-only rt-selection-announcement" role="status"></span></section>';
  }
  function findContainer(id){return [...(root.document?.querySelectorAll?.('.rt-timeline')||[])].find(item=>item.dataset.rtId===id)||null;}
  function contextFor(target){const container=target?.closest?.('.rt-timeline');return container?{container,model:models.get(container.dataset.rtId)}:null;}
  function rowContext(target){
    const row=target?.closest?.('[data-result-graph-id][data-result-sample-key]');if(!row)return null;
    const model=models.get(row.dataset.resultGraphId),container=model?findContainer(model.id):null;
    return model&&container?{row,container,model,point:model.allPoints.find(point=>point.key===row.dataset.resultSampleKey)||null}:null;
  }
  function updateHighlights(context){
    const {container,model}=context,filter=filterState(model);
    for(const mark of container.querySelectorAll('[data-rt-point]')){const point=model.points[Number(mark.dataset.rtPoint)],selected=model.pinned||model.liveDetails&&latestPoint(model)?.key;mark.classList.toggle('rt-point-selected',!!point&&(point.key===selected||!!point.members?.some(p=>p.key===selected)));mark.classList.toggle('rt-point-hovered',!!point&&point.key===model.hovered);mark.classList.toggle('rt-point-filtered',!!point&&filter.active===true&&pointMatches(model,point));mark.classList.toggle('rt-point-filter-dimmed',!!point&&filter.active===true&&!pointMatches(model,point));}
    for(const row of root.document?.querySelectorAll?.('[data-result-graph-id][data-result-sample-key]')||[]){if(row.dataset.resultGraphId!==model.id)continue;row.classList.toggle('result-graph-selected',row.dataset.resultSampleKey===(model.pinned||model.liveDetails&&latestPoint(model)?.key)||!!selectedPoint(model)?.members?.some(p=>p.key===row.dataset.resultSampleKey));row.classList.toggle('result-graph-hovered',row.dataset.resultSampleKey===model.hovered);}
  }
  function presentSelection(model,point,following=false,force=false){
    if(!point)return;
    const retained=point.members?point.members.every(p=>model.allPoints.some(q=>q.key===p.key)):model.allPoints.some(p=>p.key===point.key),displayed=model.points.some(p=>p.key===point.key||p.members?.some(q=>q.key===point.key)),matches=pointMatches(model,point);
    const message=(!retained?'This selection is retained as a snapshot. ':'')+(!displayed?'This result is outside the displayed graph range. ':'')+(!matches?'This result is outside the active result filters. ':'')+(following?'Following the latest matching result.':'Selection held. New results continue recording. Choose Latest to follow again.');
    root.NetworkToolkitResultDetails?.showGraphSelection?.({id:model.id,point,job:model.context?.job,message,following,force});
  }
  function renderSelection(context){
    if(!context?.model)return;const {container,model}=context;
    const follow=container.querySelector('[data-rt-follow]');if(follow)follow.setAttribute('aria-pressed',String(model.follow&&!model.pinned));
    const clear=container.querySelector('[data-rt-clear]');if(clear)clear.disabled=!model.pinned&&!model.liveDetails;
    const note=container.querySelector('[data-rt-selection-status]'),message=model.pinned?'Selection held. Latest resumes following.':model.liveDetails?'Following latest matching result.':'';if(note&&note.textContent!==message)note.textContent=message;
    updateHighlights(context);
    if(model.pinned)presentSelection(model,selectedPoint(model));else if(model.liveDetails&&model.follow)presentSelection(model,latestPoint(model),true);
  }
  function pointerPoint(event,context){
    const surface=event.target?.closest?.('[data-rt-surface]');if(!surface||!context?.model)return null;
    const mark=event.target.closest?.('[data-rt-point]');if(mark)return context.model.points[Number(mark.dataset.rtPoint)]||null;
    const bounds=surface.getBoundingClientRect(),actualWidth=bounds.width||bounds.right-bounds.left;if(!actualWidth||!Number.isFinite(event.clientX))return null;
    const geometry=plotGeometry(context.model),x=(event.clientX-bounds.left)/actualWidth*(context.model.mode==='progressive'?geometry.viewport:width);
    if(context.model.timeScope&&context.model.mode!=='progressive'){let closest=null,distance=Infinity;for(const point of context.model.points){const delta=Math.abs(coordinate(point.index,context.model,geometry)-x);if(delta<distance){distance=delta;closest=point;}}return closest;}
    const index=context.model.mode==='progressive'?Math.round((x+geometry.left-padding)/geometry.step):Math.round((x-padding)/(width-2*padding)*(context.model.points.length-1));
    return context.model.points[Math.max(0,Math.min(context.model.points.length-1,index))]||null;
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
  function clearPin(model,container,{close=true}={}){model.pinned=null;model.pinnedSnapshot=null;model.keyboardInspect=false;model.heldEnd='';model.liveDetails=false;model.follow=(root.NetworkToolkitGraphHistory?.preferences?.().anchor||'latest')==='latest';root.NetworkToolkitGraphWorkspace?.hold?.(model.id,false);if(close)root.NetworkToolkitResultDetails?.clearGraphSelection?.(model.id);inspection(model,model.hovered!=null);refreshModel(model);if(container)renderSelection({container,model});}
  function followPosition(model){const geometry=plotGeometry(model),point=latestPoint(model);return Math.max(0,Math.min(geometry.track-geometry.viewport,point?padding+Math.max(0,model.points.findIndex(p=>p.key===point.key))*geometry.step+padding-geometry.viewport:0));}
  function jumpLatest(id){
    const model=models.get(String(id)),container=model&&findContainer(model.id);if(!model)return false;
    clearHover();clearPin(model,container,{close:false});model.liveDetails=true;model.follow=true;model.skipCaptureOnce=true;model.windowEndKey=null;if(root.NetworkToolkitGraphHistory?.preferences?.().anchor!=='latest'&&root.NetworkToolkitGraphHistory)root.NetworkToolkitGraphHistory.setPreferences({anchor:'latest'},{persist:true});refreshModel(model);if(model.mode==='progressive')model.scrollLeft=followPosition(model);
    if(container){if(model.mode==='progressive')restoreContainer(container,model);else{model.skipCaptureOnce=true;rebuild(container,model);}}root.NetworkToolkitGraphWorkspace?.latest?.(model.id,{linked:false});presentSelection(model,latestPoint(model),true,true);return !!latestPoint(model);
  }
  function zoom(id,action){
    const model=models.get(String(id)),container=model&&findContainer(model.id);if(!model)return null;
    captureScroll(model);const oldZoom=zoomValue(model.zoom),next=zoomValue(action==='in'?oldZoom*2:action==='out'?oldZoom/2:action==='reset'?1:action);if(next===oldZoom)return next;
    const geometry=plotGeometry(model),selected=selectedPoint(model),hovered=model.allPoints.find(point=>point.key===model.hovered),visible=model.mode==='progressive'?model.points.slice(geometry.first,geometry.last):model.points;
    const anchor=[hovered,selected].find(point=>point&&visible.some(item=>item.key===point.key))||visible[Math.floor(visible.length/2)],index=anchor?model.allPoints.findIndex(point=>point.key===anchor.key):model.allPoints.length-1;
    model.zoom=next;
    if(model.mode==='progressive')model.scrollLeft=model.follow?followPosition(model):Math.max(0,padding+index*stepFor(model)-geometry.viewport/2);
    else if(!model.follow||((hovered||selected)&&index<model.allPoints.length-1)){model.follow=false;const count=Math.min(retainedLimit,Math.max(5,Math.round(limit/next))),end=Math.min(model.allPoints.length-1,index+Math.floor(count/2));model.windowEndKey=model.allPoints[end]?.key;}
    refreshModel(model);model.skipCaptureOnce=true;if(container)rebuild(container,model);return next;
  }
  function revealPoint(context,point){
    if(context.model.mode!=='progressive')return;
    const {model}=context,geometry=plotGeometry(model),index=model.points.findIndex(p=>p.key===point.key);if(index<0)return;const x=padding+index*geometry.step;
    if(x<geometry.left+padding||x>geometry.left+geometry.viewport-padding){model.follow=false;model.scrollLeft=Math.max(0,x-geometry.viewport/2);restoreContainer(context.container,model);}
  }
  function pinPoint(context,point){const model=context.model;model.pinned=point.key;model.pinnedSnapshot={...point};model.liveDetails=false;model.follow=false;model.windowEndKey=(model.rawVisiblePoints||model.points).at(-1)?.key||model.allPoints.at(-1)?.key;model.heldEnd=model.timeScope?.end||root.NetworkToolkitGraphHistory?.range?.(model.allPoints).end||'';root.NetworkToolkitGraphWorkspace?.hold?.(model.id,true,point.members?null:point.key);if(!point.members)root.NetworkToolkitGraphWorkspace?.revealSample?.(model.id,point.key);presentSelection(model,point,false,true);renderSelection(context);}
  function selectResult(id,key){const model=models.get(String(id)),container=model&&findContainer(model.id),selected=model&&selectedPoint(model),point=model?.allPoints.find(p=>p.key===key)||selected?.members?.find(p=>p.key===key)||(selected?.key===key?selected:null);if(!model||!point||!container)return false;pinPoint({container,model},point);revealPoint({container,model},point);return true;}
  function clearSelection(id){const model=models.get(String(id)),container=model&&findContainer(model.id);if(!model)return false;clearHover();clearPin(model,container);model.skipCaptureOnce=true;if(container)rebuild(container,model);return true;}
  function isHeld(id){return !!models.get(String(id))?.pinned;}
  function rebuild(container,model){
    const holder=root.document.createElement('div');holder.innerHTML=latency(model.samples,model.context||{id:model.id,title:model.title});const replacement=holder.firstElementChild;if(!replacement)return;
    container.before(replacement);container.remove();restore();
  }
  function restoreContainer(container,model){
    if(model.mode==='progressive'){
      const main=container.querySelector('[data-rt-scroll="latency"]'),measured=main?.clientWidth||main?.getBoundingClientRect?.().width;
      if(main&&resizeObserver&&!observedScrollers.has(main)){resizeObserver.observe(main);observedScrollers.add(main);}
      if(measured>0)model.viewportWidth=measured;capturePendingScroll(model,main);
      const geometry=plotGeometry(model);model.scrollLeft=model.follow?followPosition(model):Math.max(0,Math.min(model.scrollLeft,geometry.track-geometry.viewport));
      for(const scroll of container.querySelectorAll('[data-rt-scroll]')){
        const track=scroll.querySelector('.rt-track'),visible=scroll.querySelector('.rt-visible-plot'),svg=scroll.querySelector('[data-rt-surface]');if(!track||!visible||!svg)continue;
        track.style.width=geometry.track+'px';visible.style.width=geometry.viewport+'px';
        if(Math.abs((scroll.scrollLeft||0)-model.scrollLeft)>.5)scroll.scrollLeft=model.scrollLeft;
        if(scroll.clientWidth>0)scroll._rtAppliedLeft=scroll.scrollLeft;
        svg.setAttribute('viewBox','0 0 '+geometry.viewport+' '+height);
        const metric=scroll.dataset.rtScroll==='jitter'?'jitter':'rtt',filter=filterState(model),stamp=[model.scrollLeft,geometry.viewport,model.points.length,model.points.at(-1)?.key,model.zoom,model.pinned,model.hovered,metric,JSON.stringify(filter)].join('|');
        if(svg.dataset.rtWindow!==stamp){svg.innerHTML=plotContent(model,metric,metric==='rtt'?model.maximum:model.maxJitter,metric==='rtt'?model.stats.average:model.stats.jitter);svg.dataset.rtWindow=stamp;}
      }
    }
    const follow=container.querySelector('[data-rt-follow]');if(follow)follow.setAttribute('aria-pressed',String(model.follow));
    const axis=container.querySelector('.rt-time-axis')?.querySelectorAll('time')||[],times=axisPoints(model).map(point=>localTime(point?.time));
    for(let index=0;index<axis.length;index++)if(axis[index].textContent!==times[index])axis[index].textContent=times[index];
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
      if(!point||!context?.model){clearHover();return;}
      if(activeHover&&activeHover!==context.model.id)clearHover();activeHover=context.model.id;hoverPointer={target:event.target,clientX:event.clientX,clientY:event.clientY};
      const changed=context.model.hovered!==point.key;context.model.hovered=point.key;context.model.hoverOrigin=row?'row':'graph';inspection(context.model,true);
      if(row){context.model.rowUnavailable=null;if(!context.model.pinned)revealPoint(context,point);}else if(changed&&!context.model.pinned)context.model.rowUnavailable=root.NetworkToolkitGraphWorkspace?.revealSample?.(context.model.id,point.key)===false?point.key:null;
      if(changed)renderSelection(context);showTooltip(point,event,context.model);
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
      if(event.target.closest?.('[data-rt-clear]')){clearHover();clearPin(context.model,context.container);return;}
      const zoomControl=event.target.closest?.('[data-rt-zoom]');if(zoomControl){const action=zoomControl.dataset.rtZoom;zoom(context.model.id,action);findContainer(context.model.id)?.querySelector('[data-rt-zoom="'+action+'"]')?.focus?.({preventScroll:true});return;}
      if(event.target.closest?.('[data-rt-latest]')||event.target.closest?.('[data-rt-follow]')){jumpLatest(context.model.id);return;}
      const point=event.target.closest?.('[data-rt-point]')?pointerPoint(event,context):null;if(point)pinPoint(context,point);else if(event.target.matches?.('[data-rt-surface]'))clearSelection(context.model.id);
    });
    document.addEventListener('change',event=>{if(!event.target.matches?.('[data-rt-mode]'))return;setPreferences({mode:event.target.value});for(const container of [...document.querySelectorAll('.rt-timeline')]){const model=models.get(container.dataset.rtId);if(model)rebuild(container,model);}hideTooltip();});
    document.addEventListener('keydown',event=>{
      const context=contextFor(event.target),surface=event.target.closest?.('[data-rt-surface]');if(!context?.model||!surface||!['ArrowLeft','ArrowRight','Home','End','Enter',' ','Escape'].includes(event.key))return;
      event.preventDefault();event.stopPropagation();
      if(event.key==='Escape'){context.model.hovered=null;clearHover();clearPin(context.model,context.container);return;}
      const points=context.model.points,current=points.findIndex(point=>point.key===(context.model.pinned||context.model.hovered)),index=event.key==='Home'?0:event.key==='End'?points.length-1:event.key==='ArrowLeft'?Math.max(0,(current<0?points.length:current)-1):event.key==='ArrowRight'?Math.min(points.length-1,current+1):Math.max(0,current);
      if(!points[index])return;context.model.keyboardInspect=true;inspection(context.model,true);pinPoint(context,points[index]);revealPoint(context,points[index]);
    });
    document.addEventListener('scroll',event=>{
      if(!event.target.matches?.('[data-rt-scroll]')||restoring||!(event.target.clientWidth>0))return;const context=contextFor(event.target);if(!context?.model||context.model.mode!=='progressive')return;
      const scroll=event.target,maximum=Math.max(0,(scroll.scrollWidth||0)-(scroll.clientWidth||context.model.viewportWidth));context.model.scrollLeft=scroll.scrollLeft||0;context.model.follow=!context.model.pinned&&maximum-context.model.scrollLeft<=12;if(!context.model.follow&&context.model.timeScope)context.model.heldEnd=context.model.timeScope.end;restoreContainer(context.container,context.model);hideTooltip();
    },true);
    document.addEventListener('focusin',event=>{const surface=event.target.closest?.('[data-rt-surface]'),context=contextFor(event.target);if(surface&&context)lastFocus={node:surface,id:context.model.id,surface:surface.dataset.rtSurface};else lastFocus=null;});
    document.addEventListener('focusout',event=>{const context=contextFor(event.target);if(context&&event.target.closest?.('[data-rt-surface]')&&event.relatedTarget){context.model.keyboardInspect=false;inspection(context.model,context.model.hovered!=null);}if(lastFocus&&event.target===lastFocus.node&&event.relatedTarget)lastFocus=null;});
    document.addEventListener('toggle',event=>{if(!event.target.matches?.('.rt-variability'))return;const context=contextFor(event.target);if(context?.model)context.model.variabilityOpen=event.target.open===true;},true);
    document.addEventListener('networkToolkitGraphTimeChange',()=>{for(const [key,model] of [...models]){if(!model.pinned){model.follow=root.NetworkToolkitGraphHistory.preferences().anchor==='latest';if(!model.follow){model.liveDetails=false;root.NetworkToolkitResultDetails?.clearGraphSelection?.(key);}}const container=findContainer(key);if(container){model.skipCaptureOnce=true;rebuild(container,model);}}});
    document.addEventListener('networkToolkitResultFilterChange',event=>{const id=event.detail?.jobId;for(const [key,model] of [...models]){if(id!=null&&String(id)!==key)continue;const container=findContainer(key);if(container){refreshModel(model);model.skipCaptureOnce=true;rebuild(container,model);}}});
    root.addEventListener?.('resize',()=>{hideTooltip();restore();});
    if(typeof root.ResizeObserver==='function')resizeObserver=new root.ResizeObserver(restore);
    if(typeof root.MutationObserver==='function')new root.MutationObserver(restore).observe(document.documentElement||document,{subtree:true,childList:true});
  }
  const api=Object.freeze({latency,normalize,summary,pointDetails,sampleKey,preferences,normalizePreferences,setPreferences,mergePreferences,zoom,jumpLatest,selectResult,clearSelection,isHeld,restore,initialize});
  if(typeof module==='object'&&module.exports)module.exports=api;root.NetworkToolkitResultTimeline=api;initialize();
})(typeof window!=='undefined'?window:globalThis);
