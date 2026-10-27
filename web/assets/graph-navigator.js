(function(root){
 'use strict';
 const history=()=>root.NetworkToolkitGraphHistory, configs=new Map();
 const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 let drag=null,frame=null,pending=null,initialized=false;
 const bound=(n,a,b)=>n<a?a:n>b?b:n;
 function scope(start,end){return {start,end,from:history().epoch(start),to:history().epoch(end)};}
 function extent(items,getTime=item=>item.time){let a=null,b=null;for(const item of items){const t=history().instant(getTime(item));if(t===null)continue;if(a===null||t<a)a=t;if(b===null||t>b)b=t;}return scope(a===null?'':history().timestamp(a),b===null?'':history().timestamp(b));}
 function coverage(full,view){
  const h=history(),a=h.instant(full?.start),b=h.instant(full?.end),s=h.instant(view?.start),e=h.instant(view?.end);
  if(a===null||b===null||s===null||e===null)return {valid:false,left:0,width:1,before:false,after:false};
  if(b<=a)return {valid:true,single:true,left:s<=a&&e>=a?0:1,width:s<=a&&e>=a?1:0,before:s<a,after:e>b};
  const span=Number(b-a),left=Number(bound(s,a,b)-a)/span,right=Number(bound(e,a,b)-a)/span;
  return {valid:true,left,width:Math.max(0,right-left),before:s<a,after:e>b};
 }
 // The overview is always linear, even when the main plot compresses old time.
 // Arithmetic retains nanoseconds; only the pointer's pixel fraction is rounded.
 function move(full,view,part,delta){
  const h=history(),a=h.instant(full.start),b=h.instant(full.end),s=h.instant(view.start),e=h.instant(view.end);
  if(a===null||b===null||s===null||e===null||b<=a||e<s)return null;
  const shift=typeof delta==='bigint'?delta:BigInt(Math.round(Number(b-a)*delta)),minimum=1n;
  if(part==='start')return scope(h.timestamp(bound(s+shift,s<a?s:a,e-minimum<b?e-minimum:b)),view.end);
  if(part==='end')return scope(view.start,h.timestamp(bound(e+shift,s+minimum>a?s+minimum:a,e>b?e:b)));
  const length=e-s;let next=s+shift;
  // Oversized requested intervals retain their duration, including their
  // explicitly labeled unrecorded portion, rather than silently truncating it.
  next=length>b-a?bound(next,b-length,a):bound(next,a,b-length);
  return scope(h.timestamp(next),h.timestamp(next+length));
 }
 function fit(){return history().setPreferences({durationMs:0,anchor:'latest',start:'',end:'',allHistory:true},{fit:true,fitAll:true});}
 function description(view){const format=root.NetworkToolkitResultTime?.format||String;return format(view?.start)+' ['+(view?.start||'unrecorded')+'] to '+format(view?.end)+' ['+(view?.end||'unrecorded')+']';}
 function geometry(full,items,{bars=false}={}){
  const h=history(),valid=items.filter(p=>h.instant(p.time)!==null).slice().sort((a,b)=>h.compare(a.time,b.time)),values=valid.map(p=>p.value),scale=h.valueScale(values),out=[];
  if(h.instant(full.start)===null)return {html:'',scale};
  const x=t=>{const n=h.fraction(t,full);return Number.isFinite(n)?Math.max(0,Math.min(1000,n*1000)):500;},y=v=>30-(v-scale.lower)/(scale.upper-scale.lower)*25;
  // At most 320 timestamp bins. Each retains its own extrema and status lane;
  // no average-only downsampling can hide a spike, trough or missing value.
  const bins=new Map();for(const p of valid){const key=Math.min(319,Math.floor(x(p.time)*.32));let bin=bins.get(key);if(!bin){bin={first:p,last:p,min:null,max:null,levels:new Set(),missing:false,match:false};bins.set(key,bin);}bin.last=p;bin.match ||= p.match!==false;bin.levels.add(p.level||'info');if(typeof p.value==='number'&&Number.isFinite(p.value)&&p.value>=0){if(!bin.min||p.value<bin.min.value)bin.min=p;if(!bin.max||p.value>bin.max.value)bin.max=p;}else bin.missing=true;}
  const gaps=[];for(let i=Math.max(1,valid.length-256);i<valid.length;i++){const gap=h.instant(valid[i].time)-h.instant(valid[i-1].time);if(gap>0n)gaps.push(Number(gap));}gaps.sort((a,b)=>a-b);const gapLimit=(gaps[Math.floor(gaps.length/2)]||0)*4;let previous=null;
  for(const bin of bins.values()){
   const level=['bad','error','warn','info','note','good','normal'].find(l=>bin.levels.has(l))||'info',cls='gn-series gn-'+level+(bin.match?'':' gn-dim'),left=x(bin.first.time),right=x(bin.last.endTime||bin.last.time);
   if(bin.min){if(bars){if(bin.max.value>0)out.push('<path class="'+cls+'" d="M'+left+' 32V'+y(bin.max.value)+'H'+Math.max(left+.25,right)+'V32"/>');}else{
    if(previous&&previous.value!=null&&bin.first.value!=null&&!bin.missing&&Number(h.instant(bin.first.time)-h.instant(previous.time))<=gapLimit)out.push('<path class="'+cls+'" d="M'+x(previous.time)+' '+y(previous.value)+'L'+x(bin.first.time)+' '+y(bin.first.value)+'"/>');
    const samples=[bin.first,bin.min,bin.max,bin.last].filter(p=>p.value!=null).sort((a,b)=>h.compare(a.time,b.time));out.push('<polyline class="'+cls+'" points="'+samples.map(p=>x(p.time)+','+y(p.value)).join(' ')+'"/>');
   }}previous=bin.missing?null:bin.last;
   if(bin.missing)out.push('<path class="'+cls+' gn-unmeasured" d="M'+left+' 2H'+Math.max(left+.5,right)+'"/>');
  }
  return {html:out.join(''),scale};
 }
 function html(id,full,view,preview,{label='Full retained history',note=''}={}){
  const c=coverage(full,view),dated=c.valid&&Number(history().instant(full.end)-history().instant(full.start))>=86400000000000,disabled=!c.valid||c.single,desc=description(view),outside=c.before||c.after?'Requested interval extends '+(c.before&&c.after?'before and after':c.before?'before':'after')+' retained history. ':'';
  return '<div class="graph-navigator" data-graph-navigator="'+escape(id)+'" data-gn-tiny="'+(c.width<.02)+'" data-full-start="'+escape(full.start)+'" data-full-end="'+escape(full.end)+'" data-view-start="'+escape(view.start)+'" data-view-end="'+escape(view.end)+'" aria-label="'+escape(label)+'"><div class="gn-track" title="'+escape(label+'. '+note)+'"><svg class="gn-overview" viewBox="0 0 1000 34" preserveAspectRatio="none" role="img" aria-label="'+escape(label+'; display summary, original records unchanged')+'">'+(preview?.html||'')+'</svg><div class="gn-shade gn-shade-before" style="width:'+c.left*100+'%"></div><div class="gn-shade gn-shade-after" style="left:'+(c.left+c.width)*100+'%"></div><div class="gn-window" style="left:'+c.left*100+'%;width:'+c.width*100+'%" aria-hidden="true"></div>'+['window','start','end'].map(part=>'<div class="gn-'+part+'-hit" data-gn-part="'+part+'" role="slider" tabindex="'+(disabled?'-1':'0')+'" aria-disabled="'+disabled+'" aria-label="'+(part==='window'?'Move displayed time interval':part==='start'?'Displayed interval start':'Displayed interval end')+'" aria-valuemin="0" aria-valuemax="100" aria-valuenow="'+((part==='end'?c.left+c.width:c.left)*100)+'" aria-valuetext="'+escape(desc)+'" title="'+escape((part==='window'?'Drag to move time interval. Left/Right move; Home/End align with history.':'Drag to adjust '+part+'. Left/Right adjust; Home/End extend to history boundary.')+' '+desc)+'" style="left:'+(part==='end'?c.left+c.width:c.left)*100+'%;'+(part==='window'?'width:'+c.width*100+'%':'')+'">'+(part==='window'?'':'<span></span>')+'</div>').join('')+'</div><div class="gn-labels" title="'+escape(outside+desc+' | '+view.start+' to '+view.end+' | '+note)+'"><span data-gn-start>'+escape(c.valid?root.NetworkToolkitResultTime?.format?.(full.start,{date:dated,milliseconds:false,timezone:false})||full.start:'No recorded timeline')+'</span><span class="gn-context">'+escape(outside?'Outside history':label)+'</span><span data-gn-end>'+escape(c.valid?root.NetworkToolkitResultTime?.format?.(full.end,{date:dated,milliseconds:false,timezone:false})||full.end:'')+'</span></div><span class="sr-only" data-gn-announcement role="status">'+escape(outside+desc)+'</span></div>';
 }
 function attach(node,config){if(!node)return;configs.set(node.dataset.graphNavigator,{...config,node});while(configs.size>70)configs.delete(configs.keys().next().value);}
 // Retain actual handle nodes during re-rendering, keeping pointer capture and
 // keyboard focus reliable while the main graph and exact time labels update.
 function preserve(oldRoot,newRoot){
  const active=[...(oldRoot?.querySelectorAll?.('[data-graph-navigator]')||[])].some(n=>drag?.id===n.dataset.graphNavigator||n.contains(root.document?.activeElement));if(!active)return false;
  function patch(old,next){
   if(old.nodeType!==1||next.nodeType!==1){if(old.textContent!==next.textContent)old.textContent=next.textContent;return;}
   const navigator=old.hasAttribute('data-graph-navigator');
   for(const attr of [...old.attributes])if(!next.hasAttribute(attr.name)&&attr.name!=='class')old.removeAttribute(attr.name);
   for(const attr of [...next.attributes])if(!(navigator&&attr.name==='class'))old.setAttribute(attr.name,attr.value);
   if(navigator){
    for(const selector of ['.gn-window','.gn-shade-before','.gn-shade-after','[data-gn-part="window"]','[data-gn-part="start"]','[data-gn-part="end"]']){const a=old.querySelector(selector),b=next.querySelector(selector);for(const attr of [...b.attributes])a.setAttribute(attr.name,attr.value);}
    const labels=old.querySelector('.gn-labels');labels.replaceWith(next.querySelector('.gn-labels'));old.querySelector('[data-gn-announcement]').textContent=next.querySelector('[data-gn-announcement]').textContent;
    const svg=old.querySelector('svg'),newSVG=next.querySelector('svg');if(svg.innerHTML!==newSVG.innerHTML)svg.innerHTML=newSVG.innerHTML;return;
   }
   const children=[...old.childNodes],incoming=[...next.childNodes];
   for(let i=0;i<Math.max(children.length,incoming.length);i++){const a=children[i],b=incoming[i];if(!a){old.append(b);continue;}if(!b){a.remove();continue;}if(a.nodeType===1&&b.nodeType===1&&(a.hasAttribute('data-graph-navigator')||a.querySelector('[data-graph-navigator]'))&&a.tagName===b.tagName)patch(a,b);else a.replaceWith(b);}
  }
  patch(oldRoot,newRoot);return true;
 }
 function cancelFrame(){if(frame!==null)root.cancelAnimationFrame?.(frame);frame=null;}
 function flush(){cancelFrame();const value=pending;pending=null;if(value){const config=configs.get(value.id);config?.change?.(value.scope,{part:value.part});}}
 function schedule(value){pending=value;if(frame===null){if(root.requestAnimationFrame)frame=root.requestAnimationFrame(flush);else flush();}}
 function initialize(){
  const d=root.document;if(initialized||!d?.addEventListener)return;initialized=true;
  d.addEventListener('pointerdown',event=>{const handle=event.target.closest?.('[data-gn-part]'),node=handle?.closest?.('[data-graph-navigator]'),config=node&&configs.get(node.dataset.graphNavigator);if(!handle||!config||event.button!==0||handle.getAttribute('aria-disabled')==='true')return;event.preventDefault();event.stopPropagation();const track=node.querySelector('.gn-track').getBoundingClientRect();drag={id:node.dataset.graphNavigator,pointer:event.pointerId,handle,part:handle.dataset.gnPart,x:event.clientX,width:track.width,full:{...config.full},view:{...config.view}};handle.focus({preventScroll:true});handle.setPointerCapture?.(event.pointerId);node.classList.add('gn-dragging');config.begin?.();},true);
  d.addEventListener('pointermove',event=>{if(!drag||drag.pointer!==event.pointerId)return;event.preventDefault();event.stopPropagation();const scope=move(drag.full,drag.view,drag.part,(event.clientX-drag.x)/Math.max(1,drag.width));if(scope)schedule({id:drag.id,scope,part:drag.part});},true);
  const finish=event=>{if(!drag||event.pointerId!==undefined&&drag.pointer!==event.pointerId)return;flush();const previous=drag;drag=null;previous.handle.closest('[data-graph-navigator]')?.classList.remove('gn-dragging');if(previous.handle.hasPointerCapture?.(previous.pointer))previous.handle.releasePointerCapture(previous.pointer);configs.get(previous.id)?.end?.();};
  d.addEventListener('pointerup',finish,true);d.addEventListener('pointercancel',finish,true);root.addEventListener?.('blur',finish);
  d.addEventListener('click',event=>{if(event.target.closest?.('[data-graph-navigator]'))event.stopPropagation();},true);
  d.addEventListener('keydown',event=>{const handle=event.target.closest?.('[data-gn-part]'),node=handle?.closest?.('[data-graph-navigator]'),config=node&&configs.get(node.dataset.graphNavigator);if(!config||handle.getAttribute('aria-disabled')==='true'||!['ArrowLeft','ArrowRight','Home','End','PageUp','PageDown'].includes(event.key))return;event.preventDefault();event.stopPropagation();const h=history(),a=h.instant(config.full.start),b=h.instant(config.full.end),s=h.instant(config.view.start),e=h.instant(config.view.end),part=handle.dataset.gnPart;
   let delta;if(event.key==='Home')delta=part==='end'?s+1n-e:a-s;else if(event.key==='End')delta=part==='start'?e-1n-s:b-e;else{const direction=['ArrowLeft','PageUp'].includes(event.key)?-1n:1n,step=(e-s)/BigInt(event.shiftKey||event.key.startsWith('Page')?5:100);delta=direction*(step>0n?step:1n);}const next=move(config.full,config.view,part,delta);if(next){config.begin?.();config.change(next,{part});config.end?.();}
  },true);
 }
 const api=Object.freeze({scope,extent,coverage,move,fit,geometry,html,attach,preserve,initialize});if(typeof module==='object'&&module.exports)module.exports=api;root.NetworkToolkitGraphNavigator=api;initialize();
})(typeof window!=='undefined'?window:globalThis);
