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
    const offset=Math.max(0,samples.length-retainedLimit);let previous=options.history?.lastSuccess?numeric(options.history.lastRttMs):null,previousSequence=numeric(options.history?.lastSequence);
    return samples.slice(offset).map((sample,index)=>{
      const source=sample&&typeof sample==='object'?sample:{},time=sampleTime(source),success=source.success===true,rtt=success?numeric(source.rttMs):null;
      const sequence=numeric(source.sequence),consecutive=previousSequence==null||sequence==null||sequence===previousSequence+1;
      const jitter=rtt!=null&&previous!=null&&consecutive?Math.abs(rtt-previous):null;previous=rtt;previousSequence=sequence;
      return {key:sampleKey(source,offset+index),index,sequence:source.sequence??offset+index+1,time,address:String(source.address??'').slice(0,512),status:String(source.status||(success?'Reply':'No response')).slice(0,1024),success,rtt,jitter,error:String(source.error||'').slice(0,2048),source};
    });
  }
  function summary(points,prefix=null){
    const successful=points.filter(point=>point.success),values=points.map(point=>point.rtt).filter(value=>value!=null),changes=points.map(point=>point.jitter).filter(value=>value!=null),p=prefix||{},count=points.length+(p.count||0),received=successful.length+(p.received||0),lost=count-received,measured=values.length+(p.measured||0),sum=values.reduce((a,b)=>a+b,0)+(p.sumMs||0),jitterPairs=changes.length+(p.jitterPairs||0),extrema=[...values,...(p.measured>0?[p.minMs,p.maxMs]:[])].filter(Number.isFinite),jitterExtrema=[...changes,...(p.jitterPairs>0?[p.jitterMinMs,p.jitterMaxMs]:[])].filter(Number.isFinite);
    return {count,received,lost,measured,zeroCount:values.filter(value=>value===0).length+(p.zeroCount||0),untimedCount:p.untimedCount||0,lossPercent:count?lost/count*100:0,min:extrema.length?Math.min(...extrema):null,max:extrema.length?Math.max(...extrema):null,average:measured?sum/measured:null,jitter:jitterPairs?(changes.reduce((a,b)=>a+b,0)+(p.jitterSumMs||0))/jitterPairs:null,jitterPairs,jitterMin:jitterExtrema.length?Math.min(...jitterExtrema):null,jitterMax:jitterExtrema.length?Math.max(...jitterExtrema):null,archived:p.count||0};
  }
  function pointDetails(point,{compact=false}={}){
    if(!point)return '<span class="rt-instruction">Hover in line with a sample time for details. Click anywhere vertically in the plot to keep that sample selected. Focus a chart and use the arrow keys to move between samples; Escape clears the selection.</span>';
    if(point.members)return '<dl class="rt-detail-values">'+[['Grouped samples',point.stats.count],...(point.historical?[['History cluster',point.archiveInsideHorizon?'Raw retention limit: earlier results summarized before the display horizon':'Older results; one fixed boundary marker'],['Raw availability',point.members.length+' retained; '+point.stats.archived+' summarized only'],...(point.stats.untimedCount?[['Untimed archived samples',point.stats.untimedCount]]:[])]:[]),['Interval start',localTime(point.time)],['Interval end',localTime(point.endTime)],['Unsuccessful',point.stats.lost],['Minimum RTT',number(point.stats.min)],['Average RTT (aggregate)',number(point.stats.average)],['Maximum RTT',number(point.stats.max)]].map(([name,value])=>'<div><dt>'+escape(name)+'</dt><dd>'+escape(value)+'</dd></div>').join('')+'</dl>';
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
  function pointMatches(model,point){if(point.members)return !filterState(model).active||point.members.some(item=>pointMatches(model,item));try{return root.NetworkToolkitResultFilters?.graphMatches?.(model.id,point.source||point)!==false;}catch{return true;}}
  function severity(point,model){
    if(point.members){if(point.stats?.lost>0)return 'bad';const levels=point.members.map(item=>severity(item,model));return ['bad','warn','info','good'].find(level=>levels.includes(level))||'info';}
    try{const value=root.NetworkToolkitResultFilters?.severity?.(point.source||point,model.tool);if(['good','bad','warn','info'].includes(value))return value;}catch{}
    const text=String(point.status||'').toLowerCase();return /warn|partial/.test(text)?'warn':point.success?'good':/pending|queued|running|info/.test(text)?'info':'bad';
  }
  function latestPoint(model){const filtered=filterState(model).active,points=filtered?model.allPoints.filter(point=>pointMatches(model,point)):model.allPoints;return points.at(-1)||null;}
  function aggregate(group,index){
    if(!group.aggregate)return {...group.items[0],index};
    const stats=summary(group.items,group.prefix),first=group.items[0],last=group.items.at(-1),time=group.prefix?.firstTime||first?.time,endTime=last?.time||group.prefix?.lastTime;
    return {key:group.historical?'history:'+JSON.stringify([group.id,time,endTime,stats.count]):'interval:'+JSON.stringify([first.key,last.key]),index,sequence:String(group.prefix?.firstSequence??first?.sequence)+' to '+(last?.sequence??group.prefix?.lastSequence),time,endTime,members:group.items,aggregate:true,historical:group.historical,archiveInsideHorizon:group.archiveInsideHorizon,success:stats.lost===0,status:'Historical cluster: '+stats.count+' samples, '+stats.lost+' unsuccessful',rtt:stats.average,jitter:stats.jitter,error:'',address:first?.address||'',stats,source:{kind:'Historical cluster',time,recordedAt:endTime,samples:stats.count,received:stats.received,lost:stats.lost,minimumRttMs:stats.min,averageRttMs:stats.average,maximumRttMs:stats.max,meanRttChangeMs:stats.jitter,retainedRawSamples:group.items.length,summarizedOnlySamples:stats.archived,zeroRttSamples:stats.zeroCount,untimedArchivedSamples:stats.untimedCount}};
  }
  function selectPoints(model){
    const history=root.NetworkToolkitGraphHistory,pref=history?.preferences?.();let candidates=model.allPoints;
    if(history){
      const condensed=model.mode==='condensed',source=condensed&&model.prefix?[{time:model.prefix.firstTime},...candidates]:candidates;
      // In Condensed, timeframe is the recent spacing reference. It must not
      // discard the older records needed for the compressed tail/history cluster.
      const rangePref=condensed&&pref.anchor!=='range'?{...pref,durationMs:0}:pref;
      let scope=history.range(source,rangePref,model.heldEnd||(filterState(model).active&&!(pref.allHistory&&model.zoom===1&&model.follow)?latestPoint(model)?.time||'':''));
      if(model.panSpanNs&&history.instant(scope.end)!==null)scope={...scope,start:history.timestamp(history.instant(scope.end)-BigInt(model.panSpanNs))};model.baseScope=scope;
      scope=model.manualScope?{...scope,...root.NetworkToolkitGraphNavigator.scope(model.manualScope.start,model.manualScope.end)}:condensed?scope:history.viewport(scope,model.zoom,model.viewEnd||'');
      const timed=Number.isFinite(scope.from)&&Number.isFinite(scope.to),visible=timed?history.select(model.allPoints,scope):candidates,indices=new Set(visible.map(point=>point.index));
      model.rawVisiblePoints=visible.map(point=>({...point,jitter:indices.has(point.index-1)?point.jitter:null}));model.visiblePrefix=null;model.prefixPartial=false;
      const p=model.prefix,manual=!!model.manualScope||pref.anchor==='range';
      if(condensed&&p&&(!p.untimedCount||!scope.enabled&&!model.manualScope&&model.zoom===1)&&history.contains(p.firstTime,scope)&&history.contains(p.lastTime,scope)){model.visiblePrefix=p;if(model.rawVisiblePoints[0]?.index===0)model.rawVisiblePoints[0].jitter=model.allPoints[0].jitter;}
      else if(condensed&&p&&history.compare(p.lastTime,scope.start)>=0&&history.compare(p.firstTime,scope.end)<=0)model.prefixPartial=true;
      const requestedReferenceMs=(pref.durationMs||pref.recentMs/(condensed?6:2)||(condensed?10000:30000))/model.zoom,fullStart=history.instant(source[0]?.time),scopeStart=history.instant(scope.start),first=visible[0]?.time,last=visible.at(-1)?.time,span=first&&last?Number(history.instant(last)-history.instant(first)):0;
      const scaleKey=JSON.stringify([model.zoom,pref.durationMs,pref.recentMs,model.mode,model.manualScope||null,pref.anchor,pref.start,pref.end]);if(model.mature&&model.effectiveScale?.key!==scaleKey)model.effectiveScale={key:scaleKey,ms:span>0?Math.min(requestedReferenceMs,span/1000000):requestedReferenceMs};const referenceMs=model.mature&&model.effectiveScale?.key===scaleKey?model.effectiveScale.ms:requestedReferenceMs;const filled=model.mature||manual||!!(scope.enabled&&fullStart!==null&&scopeStart!==null&&fullStart<=scopeStart)||span>=referenceMs*1000000;
      let projection=history.sampleProjection(visible,scope,{mode:model.mode,width:plotGeometry(model).right-plotGeometry(model).left,referenceMs,compression:pref.compression,filled});
      if(projection?.filled)model.mature=true;if(model.mature&&model.effectiveScale?.key!==scaleKey)model.effectiveScale={key:scaleKey,ms:referenceMs};if(condensed&&model.mature&&!manual&&projection&&!projection.filled&&span>0)projection=history.fitProjection(projection,span);if(condensed&&manual&&projection&&span>0){const recent=Math.min(span,projection.recent),tail=span-recent,slope=1/(recent+tail*projection.curve.integral);projection={...projection,recent,tail,horizon:span,slope,recentShare:recent*slope,start:first,filled:true};}
      model.timeScope=history.projectScope(scope,projection);if(projection?.horizon)model.timeScope.condensedMs=projection.horizon/1000000;model.compressionLimit=projection?.widthLimited?projection.limit:null;
      let groups=model.rawVisiblePoints.map(point=>({items:[point],aggregate:false}));
      if(condensed&&timed&&projection){const cutoff=history.instant(projection.start),old=model.rawVisiblePoints.filter(point=>history.instant(point.time)<=cutoff),recent=model.rawVisiblePoints.filter(point=>history.instant(point.time)>cutoff);
        groups=recent.map(point=>({items:[point],aggregate:false}));if(old.length===1&&!model.visiblePrefix&&projection.fitted)groups.unshift({items:old,aggregate:false});else if(old.length||model.visiblePrefix)groups.unshift({items:old,prefix:model.visiblePrefix,aggregate:true,historical:true,archiveInsideHorizon:!!model.visiblePrefix&&history.instant(model.visiblePrefix.lastTime)>cutoff,id:model.id});model.timeScope.historyCluster=!!(old.length||model.visiblePrefix)&&!(old.length===1&&!model.visiblePrefix&&projection.fitted);
      }else if(condensed&&timed&&model.visiblePrefix){
        // A fully selected archive remains one honest summary even when the
        // interval contains no retained raw sample from which to build a curve.
        const p=model.visiblePrefix;model.timeScope=history.projectScope(scope,{kind:'linear',start:p.firstTime,end:p.lastTime,span:Number(history.instant(p.lastTime)-history.instant(p.firstTime)),filled:true});model.timeScope.historyCluster=true;groups=[{items:[],prefix:p,aggregate:true,historical:true,id:model.id}];
      }
      return groups.map(aggregate);
    }
    model.timeScope=null;model.rawVisiblePoints=candidates;return candidates.map((p,index)=>({...p,index}));
  }
  function refreshModel(model){
    model.points=selectPoints(model);model.stats=summary(model.rawVisiblePoints||model.points,model.visiblePrefix);const filter=filterState(model),reference=Number.isFinite(filter.reference)?filter.reference:null;
    model.rttScale=root.NetworkToolkitGraphHistory?.valueScale?.([...(model.rawVisiblePoints||[]).map(point=>point.rtt),...(model.visiblePrefix?.measured>0?[model.visiblePrefix.minMs,model.visiblePrefix.maxMs]:[])])||{lower:0,upper:Math.max(1,model.stats.max||0)*1.12};model.jitterScale=root.NetworkToolkitGraphHistory?.valueScale?.([...(model.rawVisiblePoints||[]).map(point=>point.jitter),...(model.visiblePrefix?.jitterPairs>0?[model.visiblePrefix.jitterMinMs,model.visiblePrefix.jitterMaxMs]:[])])||{lower:0,upper:Math.max(1,...model.points.map(point=>point.jitter||0))*1.12};model.maximum=model.rttScale.upper;model.maxJitter=model.jitterScale.upper;
  }
  function plotGeometry(model){const viewport=model.viewportWidth||width,left=Math.min(64,Math.max(24,viewport*.25));return {viewport,track:viewport,left,right:Math.max(left+1,viewport-padding),first:0,last:model.points?.length||0,step:0};}
  function coordinate(index,model,geometry){
    const point=model.points[index],history=root.NetworkToolkitGraphHistory,share=point?.historical?0:history?.fraction?.(point?.time,model.timeScope,model.mode);
    return geometry.left+(Number.isFinite(share)?Math.max(0,Math.min(1,share)):index/Math.max(1,model.points.length-1))*(geometry.right-geometry.left);
  }
  function ordinate(value,scale,model){const h=model?.viewportHeight||height,top=Math.min(18,h*.42),bottom=Math.max(top+1,h-padding);return bottom-(value-scale.lower)/(scale.upper-scale.lower)*(bottom-top);}
  function visibleScope(model){const scope=model.timeScope||{},first=model.points[0];return first?.historical?{...scope,start:first.time}:scope;}
  function visibleInterval(model){return (root.NetworkToolkitGraphHistory?.spanLabel?.(visibleScope(model))||'—')+' visible'+(model.mode==='condensed'?' · nonlinear'+(model.points[0]?.historical?' · includes history cluster':''):'');}
  function zoomDescription(model){const scope=visibleScope(model);return visibleInterval(model)+' · '+(scope.start||'Time not recorded')+' to '+(scope.end||'Time not recorded');}
  function referenceState(model){const filter=filterState(model),scale=model.plot==='jitter'?model.jitterScale:model.rttScale,relevant=model.plot==='jitter'?filter.statistic==='jitter':filter.statistic==='average',value=filter.reference;if(!relevant||!Number.isFinite(value)||value>=scale.lower&&value<=scale.upper)return {label:'',description:''};const direction=value>scale.upper?'above':'below';return {label:' · Ref '+(direction==='above'?'↑':'↓'),description:'Reference '+number(value)+' is '+direction+' the visible scale'};}
  function zoomControls(model){return '<div class="rt-zoom-controls graph-inline-zoom" role="group" aria-label="Graph time zoom"><button class="btn small ghost" type="button" data-rt-zoom="out" aria-label="Zoom out" title="Zoom out" '+(model.zoom<=minZoom?'disabled':'')+'>−</button><span data-rt-zoom-level data-rt-zoom-factor="'+model.zoom+'" title="'+escape(zoomDescription(model))+'" aria-label="'+escape(visibleInterval(model))+'">'+escape(root.NetworkToolkitGraphHistory?.spanLabel?.(visibleScope(model))||'—')+'</span><button class="btn small ghost" type="button" data-rt-zoom="in" aria-label="Zoom in" title="Zoom in" '+(model.zoom>=maxZoom?'disabled':'')+'>+</button><button class="btn small ghost" type="button" data-rt-zoom="reset" aria-label="Fit all retained history" title="Fit all retained history">Fit</button></div>';}

  function plotContent(model,metric,maximum,average){
    const instants=(model.rawVisiblePoints||[]).map(p=>root.NetworkToolkitGraphHistory?.instant(p.time)),gaps=[];for(let i=Math.max(1,instants.length-256);i<instants.length;i++)if(instants[i]!==null&&instants[i-1]!==null&&instants[i]>instants[i-1])gaps.push(Number(instants[i]-instants[i-1]));gaps.sort((a,b)=>a-b);const gapLimit=(gaps[Math.floor(gaps.length/2)]||Infinity)*4;
    const geometry=plotGeometry(model),viewWidth=geometry.viewport,scale=metric==='rtt'?model.rttScale:model.jitterScale,segments=[],marks=[];let segment=[],previousPoint=null;
    for(const point of model.points.slice(geometry.first,geometry.last)){
      const x=coordinate(point.index,model,geometry),value=point[metric],filter=filterState(model),classes=' rt-severity-'+severity(point,model)+(filter.active?(pointMatches(model,point)?' rt-point-filtered':' rt-point-filter-dimmed'):'')+(point.key===model.pinned?' rt-point-selected':'')+(point.key===model.hovered?' rt-point-hovered':'');
      if(previousPoint&&(previousPoint.aggregate||point.aggregate||numeric(previousPoint.sequence)!=null&&numeric(point.sequence)!=null&&Number(point.sequence)!==Number(previousPoint.sequence)+1||Number((root.NetworkToolkitGraphHistory?.instant?.(point.time)??0n)-(root.NetworkToolkitGraphHistory?.instant?.(previousPoint.time)??0n))>gapLimit)){if(segment.length)segments.push(segment);segment=[];}previousPoint=point;
      if(value==null){if(segment.length)segments.push(segment);segment=[];}else segment.push(x.toFixed(2)+','+ordinate(value,scale,model).toFixed(2));
      const attrs=' data-rt-point="'+point.index+'" data-rt-key="'+escape(point.key)+'" data-rt-sequence="'+escape(point.sequence)+'"';
      if(point.aggregate){const values=metric==='rtt'?[point.stats.min,point.stats.max].filter(value=>value!=null):[point.stats.jitterMin,point.stats.jitterMax].filter(value=>value!=null);if(values.length)marks.push('<line class="rt-aggregate-range rt-severity-'+severity(point,model)+'" x1="'+x+'" x2="'+x+'" y1="'+ordinate(Math.min(...values),scale,model)+'" y2="'+ordinate(Math.max(...values),scale,model)+'"/>');}
      const y=value==null?Math.min(18,(model.viewportHeight||height)*.42)+5:ordinate(value,scale,model);point.plotX=x;point.plotY=y;const radius=point.aggregate?5:3.5;
      // Missing measurements occupy a separate status lane; they are not zero RTT.
      marks.push('<g class="rt-point'+classes+(value==null?' rt-unmeasured':'')+'"'+attrs+(point.aggregate?' data-rt-aggregate="'+point.stats.count+'"':'')+'><circle class="rt-point-hit" cx="'+x.toFixed(2)+'" cy="'+y.toFixed(2)+'" r="9"/><circle data-rt-marker cx="'+x.toFixed(2)+'" cy="'+y.toFixed(2)+'" r="'+radius+'"/>'+(value==null?'<title>'+escape(point.status)+': '+(metric==='rtt'?'response time not measured':'no consecutive measured pair')+'</title>':'')+'</g>');
    }
    if(segment.length)segments.push(segment);
    const lines=segments.filter(segment=>segment.length>1).map(segment=>'<polyline points="'+segment.join(' ')+'"/>').join('');
    const mean=average==null?'':'<line class="rt-average" x1="'+geometry.left+'" x2="'+geometry.right+'" y1="'+ordinate(average,scale,model).toFixed(2)+'" y2="'+ordinate(average,scale,model).toFixed(2)+'"/>';
    const filter=filterState(model),reference=Number.isFinite(filter.reference)?filter.reference:null,referenceMetric=metric==='rtt'&&filter.statistic==='average'||metric==='jitter'&&filter.statistic==='jitter',runReference=referenceMetric&&reference!=null?'<line class="rt-stat-reference" data-rt-stat-reference data-rt-reference-outside="'+(reference<scale.lower?'below':reference>scale.upper?'above':'')+'" x1="'+geometry.left+'" x2="'+geometry.right+'" y1="'+ordinate(Math.max(scale.lower,Math.min(scale.upper,reference)),scale,model).toFixed(2)+'" y2="'+ordinate(Math.max(scale.lower,Math.min(scale.upper,reference)),scale,model).toFixed(2)+'"><title>'+escape((filter.referenceKind==='run'?'Run':'Retained')+' '+(metric==='rtt'?'average RTT':'RTT variability')+': '+number(reference)+(reference<scale.lower?' (below the visible scale)':reference>scale.upper?' (above the visible scale)':''))+'</title></line>':'';
    const labels=root.NetworkToolkitGraphHistory?.scaleLabels?.(scale)||[String(scale.upper),String(scale.lower)],top=ordinate(scale.upper,scale,model),bottom=ordinate(scale.lower,scale,model),axis='<g class="rt-value-axis" aria-hidden="true"><text data-rt-y-label="upper" data-rt-y-value="'+scale.upper+'" x="'+(geometry.left-5)+'" y="'+(top+3)+'" text-anchor="end" textLength="'+Math.min(56,labels[0].length*5.4)+'" lengthAdjust="spacingAndGlyphs">'+escape(labels[0])+'</text><text data-rt-y-label="lower" data-rt-y-value="'+scale.lower+'" x="'+(geometry.left-5)+'" y="'+bottom+'" text-anchor="end" textLength="'+Math.min(56,labels[1].length*5.4)+'" lengthAdjust="spacingAndGlyphs">'+escape(labels[1])+'</text></g>';
    const clip='rt-clip-'+encodeURIComponent(model.id);return '<defs><clipPath id="'+clip+'"><rect data-rt-plot-bounds x="'+geometry.left+'" y="'+top+'" width="'+(geometry.right-geometry.left)+'" height="'+(bottom-top)+'"/></clipPath></defs><path class="rt-grid" d="'+[0,.5,1].map(v=>'M'+geometry.left+' '+(top+v*(bottom-top))+'H'+geometry.right).join(' ')+' M'+geometry.left+' '+top+'V'+bottom+' M'+geometry.right+' '+top+'V'+bottom+'"/>'+axis+mean+runReference+'<g class="rt-series" clip-path="url(#'+clip+')">'+lines+'</g><g class="rt-points" clip-path="url(#'+clip+')" aria-hidden="true">'+marks.join('')+'</g>';
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
    return '<div class="rt-plot rt-'+name+'" id="rt-active-'+encodeURIComponent(model.id)+'" role="tabpanel" aria-label="'+(metric==='rtt'?'Response time (RTT)':'RTT variability')+'"><div class="rt-axis-label" title="'+escape(visibleInterval(model)+' · '+(model.timeScope?.start||'')+' to '+(model.timeScope?.end||'')+' · '+referenceState(model).description)+'">'+(metric==='rtt'?'Response time':'Absolute change')+' · ms · '+escape(visibleInterval(model))+referenceState(model).label+((metric==='rtt'?model.rttScale:model.jitterScale).empty?' · Not measured':'')+'</div><svg data-rt-surface="'+name+'" data-rt-lower="'+(metric==='rtt'?model.rttScale:model.jitterScale).lower+'" data-rt-upper="'+(metric==='rtt'?model.rttScale:model.jitterScale).upper+'" data-rt-minimum="'+((metric==='rtt'?model.rttScale:model.jitterScale).minimum??'')+'" data-rt-maximum="'+((metric==='rtt'?model.rttScale:model.jitterScale).maximum??'')+'" data-rt-left="'+plotGeometry(model).left+'" data-rt-right="'+plotGeometry(model).right+'" data-rt-horizon-ms="'+(model.timeScope?.condensedMs||'')+'" tabindex="0" role="group" aria-label="'+(metric==='rtt'?'Response time':'Absolute RTT change between consecutive measured replies')+'. Hover or click by sample time anywhere vertically in the plot. Drag to pan time. Use arrows to inspect, Enter to select, Escape to clear." viewBox="0 0 '+(model.viewportWidth||width)+' '+(model.viewportHeight||height)+'" preserveAspectRatio="xMidYMid meet">'+plotContent(model,metric,maximum,average)+'</svg></div>';
  }
  function navigatorHTML(model){
    const nav=root.NetworkToolkitGraphNavigator;if(!nav)return '';model.fullScope=nav.extent(model.mode==='condensed'&&model.prefix?[{time:model.prefix.firstTime},...model.allPoints]:model.allPoints);
    // Cache the bounded preview independently of viewport changes. Its Y range
    // uses the full recording, while each main plot fits its visible records.
    const stamp=[model.samples,model.prefix,model.plot,JSON.stringify(filterState(model))];
    if(!model.navigatorCache||stamp.some((v,i)=>v!==model.navigatorCache.stamp[i])){const preview=nav.geometry(model.fullScope,model.allPoints.map(p=>({time:p.time,value:p[model.plot],level:severity(p,model),match:pointMatches(model,p)})));model.navigatorCache={stamp,preview};}
    model.navigatorScope=nav.extent(model.visiblePrefix?[{time:model.visiblePrefix.firstTime},{time:model.visiblePrefix.lastTime},...model.rawVisiblePoints]:model.rawVisiblePoints);return nav.html('result:'+model.id,model.fullScope,model.navigatorScope,model.navigatorCache.preview,{label:'Full history',note:(model.plot==='jitter'?'RTT variability; original consecutive sample pairs':'Response time; original retained measurements')+(model.prefix?'; older history retained as summary, exact discarded records unavailable':'')});
  }
  function navigateWindow(model,scope){
    const h=root.NetworkToolkitGraphHistory,a=h.instant(scope.start),b=h.instant(scope.end),base=model.panSpanNs?BigInt(model.panSpanNs):BigInt(Math.max(1,Math.round(Number(h.instant(model.timeScope.end)-h.instant(model.timeScope.start))*model.zoom)));if(a===null||b===null||b<=a)return false;
    model.panSpanNs=String(base>0n?base:1000000000n);model.zoom=Number(BigInt(model.panSpanNs))/Number(b-a);model.manualScope={start:scope.start,end:scope.end};model.viewEnd=scope.end;model.follow=false;clearHover();const container=findContainer(model.id);if(container)rebuild(container,model);else refreshModel(model);return true;
  }
  function captureScroll(){} // Retained API call sites; history navigation now pans data.

  function latency(samples,context={}){
    if(!Array.isArray(samples)||!samples.length){models.delete(String(context.id??'latency'));return '';}
    const id=String(context.id??'latency').slice(0,512),mode=preferences().mode;
    let model=models.get(id);
    if(!model)model={id,pinned:null,pinnedSnapshot:null,hovered:null,follow:true,scrollLeft:0,viewportWidth:width,viewportHeight:height,zoom:1};
    else captureScroll(model);
    const oldMode=model.mode,prefix=context.history??(context.job?.result?.data?.samples?context.job.result.data.history:context.job?.sampleHistory);model.prefix=prefix?.count>0&&root.NetworkToolkitGraphHistory?.instant(prefix.firstTime)!==null&&root.NetworkToolkitGraphHistory?.instant(prefix.lastTime)!==null?prefix:null;if(model.prefix&&samples.length){const first=samples[Math.max(0,samples.length-retainedLimit)],lastSequence=numeric(model.prefix.lastSequence),firstSequence=numeric(first.sequence);if(lastSequence!=null&&firstSequence!=null?lastSequence>=firstSequence:root.NetworkToolkitGraphHistory.compare(model.prefix.lastTime,sampleTime(first))>0)model.prefix=null;}if(!model.renderingViewport)model.allPoints=normalize(samples,{history:model.prefix});model.missingHistory=Math.max(0,(Number(context.job?.result?.data?.statistics?.sent)||0)-model.allPoints.length-(model.prefix?.count||0));model.samples=samples;model.title=String(context.title||'Response time');model.tool=context.tool||context.job?.tool||model.tool||'monitor';model.context={...context,id,title:model.title};model.mode=mode;model.plot=plotName(plotPreferences()[model.tool]||model.plot);model.zoom=zoomValue(model.zoom);refreshModel(model);
    if(oldMode!==mode)model.scrollLeft=0;
    if(model.hovered&&!model.allPoints.some(point=>point.key===model.hovered)&&!model.points.some(point=>point.key===model.hovered))model.hovered=null;
    if(model.pinned){const refreshed=model.allPoints.find(point=>point.key===model.pinned);if(refreshed)model.pinnedSnapshot={...refreshed};}
    models.delete(id);models.set(id,model);while(models.size>64)models.delete(models.keys().next().value);
    const stats=model.stats;
    const scope=mode==='condensed'?'Condensed · nonlinear time; up to '+(model.timeScope?.projection?.count||12)+' recent points at normal spacing'+(model.compressionLimit?' (width limit; saved '+root.NetworkToolkitGraphHistory.preferences().compression.count+')':'')+'; older history in left cluster':root.NetworkToolkitGraphHistory?.label?.()||'Retained measurements',metric=model.plot;
    return '<section class="rt-timeline" data-rt-id="'+escape(id)+'" data-rt-scope-start="'+escape(model.timeScope?.start||'')+'" data-rt-scope-end="'+escape(model.timeScope?.end||'')+'" data-rt-mode-active="'+mode+'" data-rt-plot-active="'+metric+'" aria-label="'+escape(model.title)+' timeline"><div class="rt-heading"><div class="rt-view-controls"><label class="rt-mode-control"><span class="sr-only">Graph view</span><select data-rt-mode aria-label="Graph view" title="Linear uses proportional recorded time; Condensed compresses older time. Timeframe, zoom and navigation control the displayed range."><option value="linear"'+(mode==='linear'?' selected':'')+'>Linear</option><option value="condensed"'+(mode==='condensed'?' selected':'')+'>Condensed</option></select></label>'+(root.NetworkToolkitGraphHistory?.controlHTML?.(mode)||'')+zoomControls(model)+(mode==='condensed'?root.NetworkToolkitGraphHistory.compressionControlHTML(model.compressionLimit):'')+'<span class="sr-only" data-rt-selection-status></span></div></div>'+plot(model,metric,metric==='rtt'?model.maximum:model.maxJitter,metric==='rtt'?stats.average:stats.jitter)+'<div class="rt-time-axis" style="margin-left:'+plotGeometry(model).left+'px;margin-right:'+(plotGeometry(model).viewport-plotGeometry(model).right)+'px" aria-label="'+escape(timeLabel())+' horizontal time axis">'+axisHTML(model)+'</div>'+navigatorHTML(model)+'<div class="rt-plot-caption" title="'+escape(scope+' · '+(filterState(model).active?'Matching samples emphasized. ':'')+stats.count+' samples; dashed line: displayed average. Unmeasured values use the top status lane. Drag plot to pan time.')+'">'+escape(scope)+(model.prefixPartial?' · Partial archived interval unavailable':'')+(model.missingHistory?' · '+model.missingHistory+' earlier results have no recoverable summary':'')+' · '+escape(root.NetworkToolkitResultTime?.zoneLabel?.()||'Local time')+'</div><div class="sr-only">'+filterDescription(model)+'</div><span class="sr-only rt-selection-announcement" role="status"></span></section>';
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
    for(const mark of container.querySelectorAll('[data-rt-point]')){const point=model.points[Number(mark.dataset.rtPoint)],selected=model.pinned||model.liveLatest&&latestPoint(model)?.key;mark.classList.toggle('rt-point-selected',!!point&&(point.key===selected||heldMembers.has(point.key)||!!point.members?.some(p=>p.key===selected||heldMembers.has(p.key))));mark.classList.toggle('rt-point-hovered',!!point&&(point.key===model.hovered||!!point.members?.some(p=>p.key===model.hovered)));mark.classList.toggle('rt-point-filtered',!!point&&filter.active===true&&pointMatches(model,point));mark.classList.toggle('rt-point-filter-dimmed',!!point&&filter.active===true&&!pointMatches(model,point));}
    for(const row of root.document?.querySelectorAll?.('[data-result-graph-id][data-result-sample-key]')||[]){if(row.dataset.resultGraphId!==model.id)continue;row.classList.toggle('result-graph-selected',row.dataset.resultSampleKey===(model.pinned||model.liveLatest&&latestPoint(model)?.key)||!!selectedPoint(model)?.members?.some(p=>p.key===row.dataset.resultSampleKey));row.classList.toggle('result-graph-hovered',row.dataset.resultSampleKey===model.hovered);}
  }
  function presentSelection(model,point,following=false,force=false){
    if(!point)return;
    const retained=point.members?point.members.every(p=>model.allPoints.some(q=>q.key===p.key)):model.allPoints.some(p=>p.key===point.key),displayed=point.historical?model.points.some(p=>p.key===point.key):point.members?point.members.some(p=>root.NetworkToolkitGraphHistory.contains(p.time,model.timeScope)):model.points.some(p=>p.key===point.key||p.members?.some(q=>q.key===point.key)),matches=pointMatches(model,point);
    const message=(point.historical&&point.stats.archived&&filterState(model).active?'Historical summary is unfiltered; exact matches among discarded records are unavailable. ':'')+(!retained?'This selection is retained as a snapshot. ':'')+(!displayed?'This result is outside the displayed graph range. ':'')+(!matches?'This result is outside the active result filters. ':'')+(following?'Following the latest matching result.':'Selection held. New results continue recording. Choose Follow latest to follow again.');
    root.NetworkToolkitResultDetails?.showGraphSelection?.({id:model.id,point,job:model.context?.job,message,following,force});
  }
  function renderSelection(context){
    if(!context?.model)return;const {container,model}=context;
    const follow=container.querySelector('[data-rt-follow]');if(follow)follow.setAttribute('aria-pressed',String(model.follow&&!model.pinned));
    const clear=container.querySelector('[data-rt-clear]');if(clear)clear.disabled=!model.pinned&&!model.liveLatest;
    const note=container.querySelector('[data-rt-selection-status]'),message=model.pinned?'Selection held. Follow latest resumes following.':model.liveLatest?'Following latest matching result.':'';if(note&&note.textContent!==message)note.textContent=message;
    root.NetworkToolkitGraphWorkspace?.refreshFollowState?.(model.id);
    updateHighlights(context);
    if(model.pinned)presentSelection(model,selectedPoint(model));
  }
  function pointerPoint(event,context){
    const surface=event.target?.closest?.('[data-rt-surface]');if(!surface||!context?.model)return null;
    const mark=event.target.closest?.('[data-rt-point]'),bounds=surface.getBoundingClientRect(),actualWidth=bounds.width||bounds.right-bounds.left;
    if(!actualWidth||!Number.isFinite(event.clientX)||mark&&(event.clientX<bounds.left||event.clientX>bounds.left+actualWidth))return mark?context.model.points[Number(mark.dataset.rtPoint)]||null:null;
    const geometry=plotGeometry(context.model),x=(event.clientX-bounds.left)/actualWidth*geometry.viewport,h=bounds.height||bounds.bottom-bounds.top,y=Number.isFinite(event.clientY)&&h>0?(event.clientY-bounds.top)/h*(context.model.viewportHeight||height):null;
    // Time is the hit dimension. A low-Y neighboring sample must not win over
    // the visibly highlighted timestamp just because its measurement is closer.
    // Keep the value-axis gutter and space outside the SVG non-interactive.
    if(!mark&&(x<geometry.left||x>geometry.right||y!==null&&(y<0||y>(context.model.viewportHeight||height))))return null;
    // Transparent hit circles overlap in dense plots. DOM stacking is not a
    // timestamp: use the nearest X even when a later sample owns the hit area.
    // An explicit marker breaks only a same-X tie; otherwise keep an existing
    // highlighted duplicate timestamp stable while moving vertically.
    const hit=mark&&context.model.points[Number(mark.dataset.rtPoint)];
    let closest=null,distance=Infinity;for(const point of context.model.points){const delta=Math.abs((point.plotX??coordinate(point.index,context.model,geometry))-x);if(delta<distance){distance=delta;closest=point;}else if(delta===distance&&(point.key===hit?.key||closest?.key!==hit?.key&&point.key===context.model.hovered))closest=point;}return closest;
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
  function clearPin(model,container,{close=true}={}){model.pinned=null;model.pinnedSnapshot=null;model.keyboardInspect=false;model.heldEnd='';model.liveLatest=false;model.follow=(root.NetworkToolkitGraphHistory?.preferences?.().anchor||'latest')==='latest'&&!model.viewEnd;root.NetworkToolkitGraphWorkspace?.hold?.(model.id,false);if(close)root.NetworkToolkitResultDetails?.clearGraphSelection?.(model.id);inspection(model,model.hovered!=null);refreshModel(model);if(container)renderSelection({container,model});}
  function jumpLatest(id){
    const model=models.get(String(id)),container=model&&findContainer(model.id);if(!model)return false;
    if(root.NetworkToolkitGraphHistory?.preferences?.().allHistory&&model.zoom===1)model.panSpanNs='';clearPin(model,container);clearHover();model.liveLatest=true;model.follow=true;model.viewEnd='';model.manualScope=null;model.windowEndKey=null;
    if(root.NetworkToolkitGraphHistory?.preferences?.().anchor!=='latest'&&root.NetworkToolkitGraphHistory)root.NetworkToolkitGraphHistory.setPreferences({anchor:'latest'},{persist:true});refreshModel(model);
    if(container?.isConnected)rebuild(container,model);root.NetworkToolkitGraphWorkspace?.latest?.(model.id,{linked:false});return !!latestPoint(model);
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
    const {model}=context,history=root.NetworkToolkitGraphHistory;if(model.mode==='condensed'&&model.points.some(group=>group.historical&&group.members?.some(item=>item.key===point.key)))return;if(!history||!Number.isFinite(history.epoch(point.time))||history.contains(point.time,model.timeScope))return;
    const span=history.instant(model.timeScope.end)-history.instant(model.timeScope.start),end=history.instant(point.time)+span/2n;navigateWindow(model,{start:history.timestamp(end-span),end:history.timestamp(end)});
  }

  function pinPoint(context,point){const model=context.model;model.pinned=point.key;model.pinnedSnapshot={...point};model.liveLatest=false;model.follow=false;model.windowEndKey=(model.rawVisiblePoints||model.points).at(-1)?.key||model.allPoints.at(-1)?.key;model.heldEnd=model.timeScope?.end||root.NetworkToolkitGraphHistory?.range?.(model.allPoints).end||'';root.NetworkToolkitGraphWorkspace?.hold?.(model.id,true,point.members?null:point.key);if(!point.members)root.NetworkToolkitGraphWorkspace?.revealSample?.(model.id,point.key);presentSelection(model,point,false,true);renderSelection(context);}
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
      const widthChanged=model.viewportWidth!==measured.width;model.viewportWidth=measured.width;model.viewportHeight=measured.height;if(widthChanged)refreshModel(model);
      const metric=model.plot,stamp=[model.viewportWidth,model.viewportHeight,model.points.length,model.points.at(-1)?.key,model.timeScope?.start,model.timeScope?.end,model.zoom,metric,JSON.stringify(model.rttScale),JSON.stringify(model.jitterScale),JSON.stringify(filterState(model))].join('|');
      if(svg.dataset.rtWindow!==stamp){const scale=metric==='rtt'?model.rttScale:model.jitterScale;for(const [name,value]of Object.entries({lower:scale.lower,upper:scale.upper,minimum:scale.minimum??'',maximum:scale.maximum??'',left:plotGeometry(model).left,right:plotGeometry(model).right,'horizon-ms':model.timeScope?.condensedMs||''}))svg.setAttribute('data-rt-'+name,String(value));svg.setAttribute('viewBox','0 0 '+model.viewportWidth+' '+model.viewportHeight);svg.innerHTML=plotContent(model,metric,metric==='rtt'?model.maximum:model.maxJitter,metric==='rtt'?model.stats.average:model.stats.jitter);svg.dataset.rtWindow=stamp;}
    }
    const axis=container.querySelector('.rt-time-axis'),ticks=axisHTML(model);if(axis){axis.style.marginLeft=plotGeometry(model).left+'px';axis.style.marginRight=(plotGeometry(model).viewport-plotGeometry(model).right)+'px';if(axis.innerHTML!==ticks)axis.innerHTML=ticks;}
    const spanLabel=container.querySelector('[data-rt-zoom-level]');if(spanLabel){const text=root.NetworkToolkitGraphHistory?.spanLabel?.(visibleScope(model))||'—',title=zoomDescription(model),label=visibleInterval(model);if(spanLabel.textContent!==text)spanLabel.textContent=text;if(spanLabel.title!==title)spanLabel.title=title;if(spanLabel.getAttribute('aria-label')!==label)spanLabel.setAttribute('aria-label',label);}
    const curveButton=container.querySelector('[data-graph-compression-open]');if(curveButton){const limit=model.compressionLimit,requested=root.NetworkToolkitGraphHistory.preferences().compression.count;const label=limit?'Curve '+limit+'/'+requested:'Curve…',title=limit?'This width/time scale fits '+limit+' uncompressed newest points; '+requested+' remain saved. Adjust the curve or recent scale.':'Adjust recent-point count and older-history compression';if(curveButton.textContent!==label)curveButton.textContent=label;if(curveButton.title!==title)curveButton.title=title;}const slot=container.closest('[data-graph-workspace]')?.querySelector('[data-graph-tabs-slot]'),tabs=tabsHTML(model.id);if(slot&&slot.innerHTML!==tabs)slot.innerHTML=tabs;
    root.NetworkToolkitGraphNavigator?.attach?.(container.querySelector('[data-graph-navigator]'),{full:model.fullScope,view:model.navigatorScope||model.timeScope,change:scope=>navigateWindow(model,scope),begin:()=>{model.navigatorDragging=true;clearHover();},end:()=>{model.navigatorDragging=false;}});
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
    let panning=null,completedPress=null;
    document.addEventListener('pointermove',event=>{
      if(panning&&Math.hypot(event.clientX-panning.x,(event.clientY??panning.y)-panning.y)>8){clearHover();return;}
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
      // Replacement nodes can emit pointerout even though the pointer has not
      // left the plot. restore() separately clears hover when a view is gone.
      if(event.target?.isConnected===false)return;
      const context=rowContext(event.target)||contextFor(event.target);if(!context)return;
      const related=event.relatedTarget;
      if(related&&(related.closest?.('[data-rt-surface]')===event.target.closest?.('[data-rt-surface]')&&event.target.closest?.('[data-rt-surface]')||rowContext(related)?.row===rowContext(event.target)?.row&&rowContext(event.target)?.row))return;
      clearHover();
    });
    document.addEventListener('click',event=>{
      if(completedPress){completedPress=null;return;}
      if(event.button!==undefined&&event.button!==0||event.ctrlKey||event.metaKey||event.altKey||event.shiftKey)return;
      const context=contextFor(event.target);
      if(activeHover&&activeHover!==context?.model?.id)clearHover();

      if(!context?.model)return;
      const tab=event.target.closest?.('[data-rt-tab]');if(tab){setPlot(context.model.id,tab.dataset.rtTab);findContainer(context.model.id)?.closest('[data-graph-workspace]')?.querySelector('[data-rt-tab="'+tab.dataset.rtTab+'"]')?.focus({preventScroll:true});return;}
      const panControl=event.target.closest?.('[data-rt-pan]');if(panControl){pan(context.model.id,Number(panControl.dataset.rtPan));return;}
      if(event.target.closest?.('[data-rt-clear]')){clearHover();clearPin(context.model,context.container);return;}
      const zoomControl=event.target.closest?.('[data-rt-zoom]');if(zoomControl){const action=zoomControl.dataset.rtZoom;zoom(context.model.id,action);findContainer(context.model.id)?.querySelector('[data-rt-zoom="'+action+'"]')?.focus?.({preventScroll:true});return;}
      if(event.target.closest?.('[data-rt-latest]')||event.target.closest?.('[data-rt-follow]')){jumpLatest(context.model.id);return;}
      if(context.model.dragged){context.model.dragged=false;return;}const point=pointerPoint(event,context);if(point)pinPoint(context,point);
    });
    document.addEventListener('change',event=>{if(!event.target.matches?.('[data-rt-mode]'))return;setPreferences({mode:event.target.value});for(const container of [...document.querySelectorAll('.rt-timeline')]){const model=models.get(container.dataset.rtId);if(model)rebuild(container,model);}hideTooltip();});
    document.addEventListener('keydown',event=>{
      const context=contextFor(event.target),surface=event.target.closest?.('[data-rt-surface]');if(!context?.model||!surface||!['ArrowLeft','ArrowRight','Home','End','Enter',' ','Escape'].includes(event.key))return;
      event.preventDefault();event.stopPropagation();
      if(event.key==='Escape'){context.model.hovered=null;clearHover();clearPin(context.model,context.container);return;}
      const points=context.model.points,current=points.findIndex(point=>point.key===(context.model.pinned||context.model.hovered)),index=event.key==='Home'?0:event.key==='End'?points.length-1:event.key==='ArrowLeft'?Math.max(0,(current<0?points.length:current)-1):event.key==='ArrowRight'?Math.min(points.length-1,current+1):Math.max(0,current);
      if(!points[index])return;context.model.keyboardInspect=true;inspection(context.model,true);pinPoint(context,points[index]);revealPoint(context,points[index]);
    });
    document.addEventListener('pointerdown',event=>{
      completedPress=null;const surface=event.target.closest?.('[data-rt-surface]'),context=contextFor(surface);
      if(!surface||!context?.model||event.button!==0||event.ctrlKey||event.metaKey||event.altKey||event.shiftKey)return;
      const candidate=pointerPoint(event,context);if(!candidate)return;
      const geometry=plotGeometry(context.model),bounds=surface.getBoundingClientRect(),left=bounds.left+geometry.left,width=geometry.right-geometry.left;
      const stationary=activeHover===context.model.id&&context.model.hoverOrigin==='graph'&&hoverPointer&&Math.abs(event.clientX-hoverPointer.clientX)<1&&Math.abs((event.clientY??0)-(hoverPointer.clientY??0))<1;
      const highlighted=stationary&&(context.model.points.find(point=>point.key===context.model.hovered)||context.model.allPoints.find(point=>point.key===context.model.hovered));
      // Record identity, not an array index or an x coordinate. Polling can move
      // the plot (or replace its DOM) between press and release.
      panning={context,surface,id:event.pointerId,x:event.clientX,y:event.clientY??0,left,width,share:(event.clientX-left)/width,scope:{...context.model.timeScope},mode:context.model.mode,point:{...(highlighted||candidate)}};
    });
    document.addEventListener('pointerup',event=>{
      if(!panning||panning.id!==event.pointerId)return;const p=panning;panning=null;const distance=event.clientX-p.x,moved=Math.hypot(distance,(event.clientY??p.y)-p.y)>8;
      completedPress={id:p.context.model.id};const token=completedPress;root.setTimeout?.(()=>{if(completedPress===token)completedPress=null;},0);
      if(moved){p.context.model.dragged=true;clearHover();if(Math.abs(distance)>8&&p.width>0)navigateWindow(p.context.model,root.NetworkToolkitGraphHistory.panScope(p.scope,p.share,(event.clientX-p.left)/p.width,p.mode));root.setTimeout?.(()=>{p.context.model.dragged=false;},0);return;}
      const current=contextFor(event.target);if(current?.model!==p.context.model||!pointerPoint(event,current)||event.ctrlKey||event.metaKey||event.altKey||event.shiftKey)return;
      // Select on release so a replaced press target cannot retarget the click
      // to a different sample. Consume only its immediately following click.
      pinPoint(current,p.point);
    });
    document.addEventListener('pointercancel',()=>{panning=null;completedPress=null;});
    root.addEventListener?.('blur',()=>{panning=null;completedPress=null;clearHover();});
    document.addEventListener('keydown',event=>{const tab=event.target.closest?.('[data-rt-tab]'),context=contextFor(tab);if(!context?.model||!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;event.preventDefault();const next=event.key==='Home'?'rtt':event.key==='End'?'jitter':context.model.plot==='rtt'?'jitter':'rtt';setPlot(context.model.id,next);findContainer(context.model.id)?.closest('[data-graph-workspace]')?.querySelector('[data-rt-tab="'+next+'"]')?.focus({preventScroll:true});});
    document.addEventListener('wheel',event=>{
      const surface=event.target.closest?.('[data-rt-surface]'),context=contextFor(surface);if(!surface||!context?.model)return;const model=context.model,history=root.NetworkToolkitGraphHistory;if(!history)return;const pending=wheelFrames.get(model.id),next=history.wheelLevel(event,pending?.next??model.zoom,Math.min(minZoom,model.zoom),Math.max(maxZoom,model.zoom));if(next===(pending?.next??model.zoom))return;event.preventDefault();const bounds=surface.getBoundingClientRect(),geometry=plotGeometry(model),share=Math.max(0,Math.min(1,(event.clientX-bounds.left-geometry.left)/(geometry.right-geometry.left)));
      if(pending){pending.next=next;return;}const frame={next,share,scope:{...model.timeScope}};wheelFrames.set(model.id,frame);const apply=()=>{wheelFrames.delete(model.id);if(findContainer(model.id))zoom(model.id,frame.next,{anchorShare:frame.share,scope:frame.scope});};if(root.requestAnimationFrame)root.requestAnimationFrame(apply);else apply();
    },{passive:false});
    document.addEventListener('focusin',event=>{const surface=event.target.closest?.('[data-rt-surface]'),context=contextFor(event.target);if(surface&&context)lastFocus={node:surface,id:context.model.id,surface:surface.dataset.rtSurface};else lastFocus=null;});
    document.addEventListener('focusout',event=>{const context=contextFor(event.target);if(context&&event.target.closest?.('[data-rt-surface]')&&event.relatedTarget){context.model.keyboardInspect=false;inspection(context.model,context.model.hovered!=null);}if(lastFocus&&event.target===lastFocus.node&&event.relatedTarget)lastFocus=null;});
    document.addEventListener('toggle',event=>{if(!event.target.matches?.('.rt-variability'))return;const context=contextFor(event.target);if(context?.model)context.model.variabilityOpen=event.target.open===true;},true);
    document.addEventListener('networkToolkitGraphTimeChange',event=>{for(const [key,model] of [...models]){
      if(event.detail?.compressionOnly){const container=findContainer(key);if(container){model.skipCaptureOnce=true;rebuild(container,model);}continue;}const wasFollowing=model.follow&&!model.pinned;model.viewEnd='';model.panSpanNs='';model.manualScope=null;if(event.detail?.fit)model.zoom=1;
      if(event.detail?.fitAll){model.heldEnd='';model.follow=wasFollowing;if(!wasFollowing&&model.mode!=='condensed'){const full=root.NetworkToolkitGraphNavigator.extent(model.mode==='condensed'&&model.prefix?[{time:model.prefix.firstTime},...model.allPoints]:model.allPoints);if(root.NetworkToolkitGraphHistory.instant(full.start)!==null)model.manualScope=full;}}
      else if(!model.pinned){model.follow=root.NetworkToolkitGraphHistory.preferences().anchor==='latest';if(!model.follow){model.liveLatest=false;root.NetworkToolkitResultDetails?.clearGraphSelection?.(key);}}
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
