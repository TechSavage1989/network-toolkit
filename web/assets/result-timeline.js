(function (root) {
  'use strict';

  const limit=180, retainedLimit=10000, width=820, height=110, padding=9, spacing=8;
  const models=new Map();
  let initialized=false,lastFocus=null,activeHover=null,tooltip=null,restoring=false,resizeObserver=null;
  const observedScrollers=new Set();
  let fallbackPreferences={mode:'window'};
  const escape=value=>String(value==null?'':value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const numeric=value=>(typeof value==='number'||typeof value==='string'&&value.trim())&&Number.isFinite(Number(value))&&Number(value)>=0?Number(value):null;
  const numberFormatter=new Intl.NumberFormat(undefined,{maximumFractionDigits:3}),lossFormatter=new Intl.NumberFormat(undefined,{maximumFractionDigits:1});
  const number=value=>value==null?'Not measured':numberFormatter.format(value)+' ms';
  function normalizePreferences(value){return {mode:value?.mode==='progressive'?'progressive':'window'};}
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
    const count=options.mode==='progressive'?retainedLimit:limit,offset=Math.max(0,samples.length-count);let previous=null;
    return samples.slice(offset).map((sample,index)=>{
      const source=sample&&typeof sample==='object'?sample:{},time=sampleTime(source),success=source.success===true,rtt=success?numeric(source.rttMs):null;
      const jitter=rtt!=null&&previous!=null?Math.abs(rtt-previous):null;previous=rtt;
      return {key:sampleKey(source,offset+index),index,sequence:source.sequence??offset+index+1,time,address:String(source.address??'').slice(0,512),status:String(source.status||(success?'Reply':'No response')).slice(0,1024),success,rtt,jitter,error:String(source.error||'').slice(0,2048)};
    });
  }
  function summary(points){
    const successful=points.filter(point=>point.success),values=points.map(point=>point.rtt).filter(value=>value!=null),changes=points.map(point=>point.jitter).filter(value=>value!=null);
    return {count:points.length,received:successful.length,lost:points.length-successful.length,lossPercent:points.length?(points.length-successful.length)/points.length*100:0,min:values.length?Math.min(...values):null,max:values.length?Math.max(...values):null,average:values.length?values.reduce((a,b)=>a+b,0)/values.length:null,jitter:changes.length?changes.reduce((a,b)=>a+b,0)/changes.length:null,jitterPairs:changes.length};
  }
  function pointDetails(point,{compact=false}={}){
    if(!point)return '<span class="rt-instruction">Hover over a point for details. Click to keep it selected. Focus a chart and use the arrow keys to move between samples; Escape clears the selection.</span>';
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
    return '<p class="rt-selected-context">Sample '+escape(selectedPoint(model)?.sequence)+' stays selected until you select another point or clear the selection.</p>';
  }
  function plotGeometry(model){
    const viewport=Math.max(1,model.viewportWidth||width),track=model.mode==='progressive'?Math.max(viewport,padding*2+(model.points.length-1)*spacing):viewport;
    const left=model.mode==='progressive'?Math.max(0,Math.min(model.scrollLeft||0,track-viewport)):0;
    const first=model.mode==='progressive'?Math.max(0,Math.floor((left-padding)/spacing)-1):0;
    const last=model.mode==='progressive'?Math.min(model.points.length,Math.ceil((left+viewport-padding)/spacing)+2):model.points.length;
    return {viewport,track,left,first,last};
  }
  function axisPoints(model){if(model.mode!=='progressive')return [model.points[0],model.points.at(-1)];const geometry=plotGeometry(model),first=Math.max(0,Math.min(model.points.length-1,Math.ceil((geometry.left-padding)/spacing))),last=Math.max(first,Math.min(model.points.length-1,Math.floor((geometry.left+geometry.viewport-padding)/spacing)));return [model.points[first],model.points[last]];}
  function coordinate(index,model,geometry){return model.mode==='progressive'?padding+index*spacing-geometry.left:padding+index*(width-2*padding)/Math.max(1,model.points.length-1);}
  function ordinate(value,maximum){return height-padding-value/maximum*(height-2*padding);}
  function plotContent(model,metric,maximum,average){
    const geometry=plotGeometry(model),viewWidth=model.mode==='progressive'?geometry.viewport:width,segments=[],marks=[];let segment=[];
    for(const point of model.points.slice(geometry.first,geometry.last)){
      const x=coordinate(point.index,model,geometry),value=point[metric],classes=(point.key===model.pinned?' rt-point-selected':'')+(point.key===model.hovered?' rt-point-hovered':'');
      if(value==null){if(segment.length)segments.push(segment);segment=[];}else segment.push(x.toFixed(2)+','+ordinate(value,maximum).toFixed(2));
      const attrs=' data-rt-point="'+point.index+'" data-rt-key="'+escape(point.key)+'" data-rt-sequence="'+escape(point.sequence)+'"';
      if(metric==='rtt'&&!point.success)marks.push('<g class="rt-point rt-missed'+classes+'"'+attrs+'><rect x="'+(x-3).toFixed(2)+'" y="3" width="6" height="104"/><path d="M'+(x-3).toFixed(2)+' 5l6 6m-6 0l6-6"/></g>');
      else if(value!=null)marks.push('<circle class="rt-point'+classes+'"'+attrs+' cx="'+x.toFixed(2)+'" cy="'+ordinate(value,maximum).toFixed(2)+'" r="3"/>');
    }
    if(segment.length)segments.push(segment);
    const lines=segments.filter(segment=>segment.length>1).map(segment=>'<polyline points="'+segment.join(' ')+'"/>').join('');
    const mean=average==null?'':'<line class="rt-average" x1="0" x2="'+viewWidth+'" y1="'+ordinate(average,maximum).toFixed(2)+'" y2="'+ordinate(average,maximum).toFixed(2)+'"/>';
    return '<path class="rt-grid" d="M0 27H'+viewWidth+' M0 55H'+viewWidth+' M0 82H'+viewWidth+'"/>'+mean+'<g class="rt-series">'+lines+'</g><g class="rt-points" aria-hidden="true">'+marks.join('')+'</g>';
  }
  function plot(model,metric,maximum,average){
    const name=metric==='rtt'?'latency':'jitter',geometry=plotGeometry(model),progressive=model.mode==='progressive';
    const svg='<svg data-rt-surface="'+name+'" tabindex="0" role="group" aria-label="'+(metric==='rtt'?'Response time':'Absolute RTT change between consecutive measured replies')+'. Use Left and Right arrows to inspect samples, Home and End to jump, Enter to select, Escape to clear." viewBox="0 0 '+(progressive?geometry.viewport:width)+' '+height+'" preserveAspectRatio="none">'+plotContent(model,metric,maximum,average)+'</svg>';
    return '<div class="rt-plot rt-'+name+'"><div class="rt-axis-label">'+escape(number(maximum))+'</div>'+(progressive?'<div class="rt-scroll" data-rt-scroll="'+name+'" tabindex="0" aria-label="Scroll '+name+' retained history"><div class="rt-track" style="width:'+geometry.track+'px"><div class="rt-visible-plot" style="width:'+geometry.viewport+'px">'+svg+'</div></div></div>':svg)+'</div>';
  }
  function captureScroll(model){
    const container=findContainer(model.id),scroll=container?.querySelector('[data-rt-scroll="latency"]');
    if(!scroll||model.mode!=='progressive'||!Number.isFinite(scroll.scrollLeft))return;
    model.scrollLeft=scroll.scrollLeft;
  }
  function latency(samples,context={}){
    if(!Array.isArray(samples)||!samples.length)return '';
    const id=String(context.id??'latency').slice(0,512),mode=preferences().mode;
    let model=models.get(id);
    if(!model)model={id,pinned:null,pinnedSnapshot:null,hovered:null,follow:true,scrollLeft:0,viewportWidth:width};
    else captureScroll(model);
    const oldMode=model.mode;model.allPoints=normalize(samples,{mode:'progressive'});model.points=mode==='progressive'?model.allPoints:normalize(samples);model.samples=samples;model.title=String(context.title||'Response time');model.mode=mode;
    if(oldMode!==mode){model.scrollLeft=0;model.follow=true;}
    if(model.hovered&&!model.allPoints.some(point=>point.key===model.hovered))model.hovered=null;
    if(model.pinned){const refreshed=model.allPoints.find(point=>point.key===model.pinned);if(refreshed)model.pinnedSnapshot={...refreshed};}
    if(mode==='progressive'&&model.follow)model.scrollLeft=Math.max(0,plotGeometry(model).track-model.viewportWidth);
    models.delete(id);models.set(id,model);while(models.size>64)models.delete(models.keys().next().value);
    const stats=summary(model.points);model.maximum=Math.max(1,stats.max||0)*1.12;model.maxJitter=Math.max(1,...model.points.map(point=>point.jitter||0))*1.12;model.stats=stats;
    const windowLabel=mode==='progressive'?'Progressive retained history':samples.length>limit?'Latest '+limit+' retained samples':'Displayed samples',loss=lossFormatter.format(stats.lossPercent);
    return '<section class="rt-timeline" data-rt-id="'+escape(id)+'" data-rt-mode-active="'+mode+'" aria-label="'+escape(model.title)+' timeline"><div class="rt-heading"><h3>'+escape(model.title)+'</h3><span>'+escape(windowLabel)+' ('+model.points.length+')</span><div class="rt-view-controls"><label>Graph view <select data-rt-mode aria-label="Graph view"><option value="window"'+(mode==='window'?' selected':'')+'>Moving window</option><option value="progressive"'+(mode==='progressive'?' selected':'')+'>Progressive history</option></select></label>'+(mode==='progressive'?'<button type="button" class="btn small ghost" data-rt-follow aria-pressed="'+model.follow+'">Follow latest</button>':'')+'</div></div>'+plot(model,'rtt',model.maximum,stats.average)+'<div class="rt-time-axis"><time>'+escape(localTime(axisPoints(model)[0].time))+'</time><time>'+escape(localTime(axisPoints(model)[1].time))+'</time></div><div class="rt-window-stats" aria-label="Statistics for displayed samples"><span><strong>'+stats.received+'/'+stats.count+'</strong> successful</span><span><strong>'+loss+'%</strong> unsuccessful</span><span>Minimum <strong>'+escape(number(stats.min))+'</strong></span><span>Average <strong>'+escape(number(stats.average))+'</strong></span><span>Maximum <strong>'+escape(number(stats.max))+'</strong></span></div><div class="rt-loss-bar" role="img" aria-label="Displayed samples: '+stats.received+' successful, '+stats.lost+' unsuccessful"><span class="rt-success-share" style="width:'+((stats.count-stats.lost)/stats.count*100).toFixed(4)+'%"></span><span class="rt-loss-share" style="width:'+stats.lossPercent.toFixed(4)+'%"></span></div><div class="rt-loss-legend"><span>Successful: '+stats.received+'</span><span>Unsuccessful: '+stats.lost+'</span><span>Dashed line: displayed average RTT</span></div><details class="rt-variability"'+(model.variabilityOpen?' open':'')+'><summary>RTT variability <span>'+escape(number(stats.jitter))+' mean absolute change</span></summary><p>Absolute RTT change between consecutive successful measured replies in this displayed window. A missing or unsuccessful probe breaks the pair.</p>'+(stats.jitterPairs?plot(model,'jitter',model.maxJitter,stats.jitter):'<p>No consecutive measured reply pairs are available.</p>')+'</details><div class="rt-detail-controls"><span>'+(model.pinned?'Selected point details':'Point details')+'</span><button type="button" class="btn small ghost" data-rt-clear '+(model.pinned?'':'disabled')+'>Clear selection</button></div><div class="rt-selection-context">'+selectionContext(model)+'</div><div class="rt-hover-context"></div><div class="rt-detail" role="status" aria-live="polite" aria-atomic="true">'+pointDetails(detailsPoint(model))+'</div></section>';
  }
  function findContainer(id){return [...(root.document?.querySelectorAll?.('.rt-timeline')||[])].find(item=>item.dataset.rtId===id)||null;}
  function contextFor(target){const container=target?.closest?.('.rt-timeline');return container?{container,model:models.get(container.dataset.rtId)}:null;}
  function rowContext(target){
    const row=target?.closest?.('[data-result-graph-id][data-result-sample-key]');if(!row)return null;
    const model=models.get(row.dataset.resultGraphId),container=model?findContainer(model.id):null;
    return model&&container?{row,container,model,point:model.allPoints.find(point=>point.key===row.dataset.resultSampleKey)||null}:null;
  }
  function updateHighlights(context){
    const {container,model}=context;
    for(const mark of container.querySelectorAll('[data-rt-point]')){const point=model.points[Number(mark.dataset.rtPoint)];mark.classList.toggle('rt-point-selected',!!point&&point.key===model.pinned);mark.classList.toggle('rt-point-hovered',!!point&&point.key===model.hovered);}
    for(const row of root.document?.querySelectorAll?.('[data-result-graph-id][data-result-sample-key]')||[]){if(row.dataset.resultGraphId!==model.id)continue;row.classList.toggle('result-graph-selected',row.dataset.resultSampleKey===model.pinned);row.classList.toggle('result-graph-hovered',row.dataset.resultSampleKey===model.hovered);}
  }
  function renderSelection(context){
    if(!context?.model)return;
    const {container,model}=context,detail=container.querySelector('.rt-detail'),caption=container.querySelector('.rt-detail-controls')?.querySelector('span'),note=container.querySelector('.rt-selection-context');
    if(detail)detail.innerHTML=pointDetails(detailsPoint(model));if(caption)caption.textContent=model.pinned?'Selected point details':'Point details';if(note)note.innerHTML=selectionContext(model);
    const hover=model.allPoints.find(point=>point.key===model.hovered),hoverNote=container.querySelector('.rt-hover-context');
    if(hoverNote)hoverNote.textContent=hover&&!model.points.some(point=>point.key===hover.key)?'Sample '+hover.sequence+' is outside the moving window. Choose Progressive history to inspect this retained sample.':hover&&model.rowUnavailable===hover.key?'Sample '+hover.sequence+' is not included in the current result filter.':'';
    const follow=container.querySelector('[data-rt-follow]');if(follow)follow.setAttribute('aria-pressed',String(model.follow));
    const clear=container.querySelector('[data-rt-clear]');if(clear)clear.disabled=!model.pinned;updateHighlights(context);
  }
  function pointerPoint(event,context){
    const surface=event.target?.closest?.('[data-rt-surface]');if(!surface||!context?.model)return null;
    const mark=event.target.closest?.('[data-rt-point]');if(mark)return context.model.points[Number(mark.dataset.rtPoint)]||null;
    const bounds=surface.getBoundingClientRect(),actualWidth=bounds.width||bounds.right-bounds.left;if(!actualWidth||!Number.isFinite(event.clientX))return null;
    const geometry=plotGeometry(context.model),x=(event.clientX-bounds.left)/actualWidth*(context.model.mode==='progressive'?geometry.viewport:width);
    const index=context.model.mode==='progressive'?Math.round((x+geometry.left-padding)/spacing):Math.round((x-padding)/(width-2*padding)*(context.model.points.length-1));
    return context.model.points[Math.max(0,Math.min(context.model.points.length-1,index))]||null;
  }
  function hideTooltip(){if(tooltip){tooltip.hidden=true;tooltip.setAttribute('aria-hidden','true');}}
  function showTooltip(point,event){
    const document=root.document;if(!document?.createElement||!document.body)return;
    if(!tooltip||!tooltip.isConnected){tooltip=document.createElement('div');tooltip.className='rt-tooltip';tooltip.id='networkToolkitGraphTooltip';tooltip.dataset.rtTooltip='';tooltip.setAttribute('role','tooltip');document.body.append(tooltip);}
    const content='<strong>Sample '+escape(point.sequence)+'</strong>'+pointDetails(point,{compact:true});
    if(tooltip._rtContent!==content){tooltip.innerHTML=content;tooltip._rtContent=content;}tooltip.hidden=false;tooltip.setAttribute('aria-hidden','false');
    const anchor=event.target?.getBoundingClientRect?.()||{left:0,right:0,top:0,bottom:0},x=Number.isFinite(event.clientX)?event.clientX:(anchor.left+anchor.right)/2,y=Number.isFinite(event.clientY)?event.clientY:anchor.top;
    const bounds=tooltip.getBoundingClientRect(),w=bounds.width||Math.max(0,bounds.right-bounds.left)||300,h=bounds.height||Math.max(0,bounds.bottom-bounds.top)||180,vw=root.innerWidth||1024,vh=root.innerHeight||768;
    tooltip.style.left=Math.max(8,Math.min(vw-w-8,x+14))+'px';tooltip.style.top=Math.max(8,Math.min(vh-h-8,y+h+22<vh?y+16:y-h-12))+'px';
  }
  function clearHover(){
    if(activeHover){const model=models.get(activeHover),container=findContainer(activeHover);if(model){model.hovered=null;model.rowUnavailable=null;if(container)renderSelection({container,model});}}activeHover=null;hideTooltip();
  }
  function revealPoint(context,point){
    if(context.model.mode!=='progressive')return;
    const {model}=context,geometry=plotGeometry(model),x=padding+point.index*spacing;
    if(x<geometry.left+padding||x>geometry.left+geometry.viewport-padding){model.follow=false;model.scrollLeft=Math.max(0,x-geometry.viewport/2);restoreContainer(context.container,model);}
  }
  function pinPoint(context,point){context.model.pinned=point.key;context.model.pinnedSnapshot={...point};if(context.model.mode==='progressive')context.model.follow=false;root.NetworkToolkitGraphWorkspace?.revealSample?.(context.model.id,point.key);renderSelection(context);}
  function rebuild(container,model){
    const holder=root.document.createElement('div');holder.innerHTML=latency(model.samples,{id:model.id,title:model.title});const replacement=holder.firstElementChild;if(!replacement)return;
    container.before(replacement);container.remove();restore();
  }
  function restoreContainer(container,model){
    if(model.mode==='progressive'){
      const main=container.querySelector('[data-rt-scroll="latency"]'),measured=main?.clientWidth||main?.getBoundingClientRect?.().width;
      if(main&&resizeObserver&&!observedScrollers.has(main)){resizeObserver.observe(main);observedScrollers.add(main);}
      if(measured>0)model.viewportWidth=measured;
      const geometry=plotGeometry(model);model.scrollLeft=model.follow?Math.max(0,geometry.track-geometry.viewport):Math.max(0,Math.min(model.scrollLeft,geometry.track-geometry.viewport));
      for(const scroll of container.querySelectorAll('[data-rt-scroll]')){
        const track=scroll.querySelector('.rt-track'),visible=scroll.querySelector('.rt-visible-plot'),svg=scroll.querySelector('[data-rt-surface]');if(!track||!visible||!svg)continue;
        track.style.width=geometry.track+'px';visible.style.width=geometry.viewport+'px';
        if(Math.abs((scroll.scrollLeft||0)-model.scrollLeft)>.5)scroll.scrollLeft=model.scrollLeft;
        svg.setAttribute('viewBox','0 0 '+geometry.viewport+' '+height);
        const metric=scroll.dataset.rtScroll==='jitter'?'jitter':'rtt',stamp=[model.scrollLeft,geometry.viewport,model.points.length,model.points.at(-1)?.key,model.pinned,model.hovered,metric].join('|');
        if(svg.dataset.rtWindow!==stamp){svg.innerHTML=plotContent(model,metric,metric==='rtt'?model.maximum:model.maxJitter,metric==='rtt'?model.stats.average:model.stats.jitter);svg.dataset.rtWindow=stamp;}
      }
      const follow=container.querySelector('[data-rt-follow]');if(follow)follow.setAttribute('aria-pressed',String(model.follow));
      const axis=container.querySelector('.rt-time-axis')?.querySelectorAll('time')||[],times=axisPoints(model).map(point=>localTime(point.time));
      for(let index=0;index<axis.length;index++)if(axis[index].textContent!==times[index])axis[index].textContent=times[index];
    }
    updateHighlights({container,model});
  }
  function restore(){
    if(restoring)return;restoring=true;
    try{for(const item of observedScrollers)if(!item.isConnected){resizeObserver?.unobserve(item);observedScrollers.delete(item);}
      for(const container of root.document?.querySelectorAll?.('.rt-timeline')||[]){const model=models.get(container.dataset.rtId);if(model)restoreContainer(container,model);}
      if(lastFocus&&!lastFocus.node.isConnected&&(!root.document.activeElement||root.document.activeElement===root.document.body)){const saved=lastFocus;lastFocus=null;const replacement=findContainer(saved.id)?.querySelector('[data-rt-surface="'+saved.surface+'"]');replacement?.focus({preventScroll:true});}
    }finally{restoring=false;}
  }
  function initialize(){
    const document=root.document;if(initialized||!document?.addEventListener)return;initialized=true;
    document.addEventListener('pointermove',event=>{
      const row=rowContext(event.target),context=row||contextFor(event.target),point=row?.point||pointerPoint(event,context);
      if(!point||!context?.model){clearHover();return;}
      if(activeHover&&activeHover!==context.model.id)clearHover();activeHover=context.model.id;
      const changed=context.model.hovered!==point.key;context.model.hovered=point.key;
      if(row){context.model.rowUnavailable=null;revealPoint(context,point);}else if(changed)context.model.rowUnavailable=root.NetworkToolkitGraphWorkspace?.revealSample?.(context.model.id,point.key)===false?point.key:null;
      if(changed)renderSelection(context);showTooltip(point,event);
    });
    document.addEventListener('pointerout',event=>{
      const context=rowContext(event.target)||contextFor(event.target);if(!context)return;
      const related=event.relatedTarget;
      if(related&&(related.closest?.('[data-rt-surface]')===event.target.closest?.('[data-rt-surface]')&&event.target.closest?.('[data-rt-surface]')||rowContext(related)?.row===rowContext(event.target)?.row&&rowContext(event.target)?.row))return;
      clearHover();
    });
    document.addEventListener('click',event=>{
      const context=contextFor(event.target);if(!context?.model)return;
      if(event.target.closest?.('[data-rt-clear]')){context.model.pinned=null;context.model.pinnedSnapshot=null;context.model.hovered=null;activeHover=null;hideTooltip();renderSelection(context);return;}
      if(event.target.closest?.('[data-rt-follow]')){context.model.follow=true;restoreContainer(context.container,context.model);return;}
      const point=pointerPoint(event,context);if(point)pinPoint(context,point);
    });
    document.addEventListener('change',event=>{if(!event.target.matches?.('[data-rt-mode]'))return;setPreferences({mode:event.target.value});for(const container of [...document.querySelectorAll('.rt-timeline')]){const model=models.get(container.dataset.rtId);if(model)rebuild(container,model);}hideTooltip();});
    document.addEventListener('keydown',event=>{
      const context=contextFor(event.target),surface=event.target.closest?.('[data-rt-surface]');if(!context?.model||!surface||!['ArrowLeft','ArrowRight','Home','End','Enter',' ','Escape'].includes(event.key))return;
      event.preventDefault();event.stopPropagation();
      if(event.key==='Escape'){context.model.pinned=null;context.model.pinnedSnapshot=null;context.model.hovered=null;clearHover();renderSelection(context);return;}
      const points=context.model.points,current=points.findIndex(point=>point.key===(context.model.pinned||context.model.hovered)),index=event.key==='Home'?0:event.key==='End'?points.length-1:event.key==='ArrowLeft'?Math.max(0,(current<0?points.length:current)-1):event.key==='ArrowRight'?Math.min(points.length-1,current+1):Math.max(0,current);
      pinPoint(context,points[index]);revealPoint(context,points[index]);
    });
    document.addEventListener('scroll',event=>{
      if(!event.target.matches?.('[data-rt-scroll]')||restoring)return;const context=contextFor(event.target);if(!context?.model||context.model.mode!=='progressive')return;
      const scroll=event.target,maximum=Math.max(0,(scroll.scrollWidth||0)-(scroll.clientWidth||context.model.viewportWidth));context.model.scrollLeft=scroll.scrollLeft||0;context.model.follow=maximum-context.model.scrollLeft<=12;restoreContainer(context.container,context.model);hideTooltip();
    },true);
    document.addEventListener('focusin',event=>{const surface=event.target.closest?.('[data-rt-surface]'),context=contextFor(event.target);if(surface&&context)lastFocus={node:surface,id:context.model.id,surface:surface.dataset.rtSurface};else lastFocus=null;});
    document.addEventListener('focusout',event=>{if(lastFocus&&event.target===lastFocus.node&&event.relatedTarget)lastFocus=null;});
    document.addEventListener('toggle',event=>{if(!event.target.matches?.('.rt-variability'))return;const context=contextFor(event.target);if(context?.model)context.model.variabilityOpen=event.target.open===true;},true);
    root.addEventListener?.('resize',()=>{hideTooltip();restore();});
    if(typeof root.ResizeObserver==='function')resizeObserver=new root.ResizeObserver(restore);
    if(typeof root.MutationObserver==='function')new root.MutationObserver(restore).observe(document.documentElement||document,{subtree:true,childList:true});
  }
  const api=Object.freeze({latency,normalize,summary,pointDetails,sampleKey,preferences,normalizePreferences,setPreferences,mergePreferences,restore,initialize});
  if(typeof module==='object'&&module.exports)module.exports=api;root.NetworkToolkitResultTimeline=api;initialize();
})(typeof window!=='undefined'?window:globalThis);
