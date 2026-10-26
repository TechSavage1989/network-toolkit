(function(root){
 'use strict';
 const models=new Map(),minimum=.3,maximum=.75,defaultRatio=.6;
 let initialized=false,drag=null,scheduled=false,resizeObserver=null;
 const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 function normalize(value){const ratio=value&&typeof value==='object'&&!Array.isArray(value)&&typeof value.ratio==='number'?value.ratio:NaN;return {ratio:Number.isFinite(ratio)?Math.max(minimum,Math.min(maximum,ratio)):defaultRatio};}
 function appState(){return typeof state==='object'&&state?state:root.state&&typeof root.state==='object'?root.state:null;}
 function preferences(){return normalize(appState()?.graphWorkspacePreferences);}
 function mergePreferences(saved){const target=appState();if(target)target.graphWorkspacePreferences=normalize(saved?.graphWorkspacePreferences);}
 function model(id){const key=String(id??'').slice(0,512);let item=models.get(key);if(!item)item={graphTop:0,graphLeft:0,resultTop:0,resultLeft:0,node:null};models.delete(key);models.set(key,item);while(models.size>64){const first=models.keys().next().value,removed=models.get(first);if(removed?.node){resizeObserver?.unobserve(removed.node);const results=removed.node.querySelector('[data-graph-results]');if(results)resizeObserver?.unobserve(results);}models.delete(first);}return item;}
 function find(id){return [...(root.document?.querySelectorAll?.('[data-graph-workspace]')||[])].find(node=>node.dataset.graphWorkspace===id);}
 function capture(node,item){if(!node||!item)return;const graph=node.querySelector('[data-graph-pane]'),results=node.querySelector('[data-graph-results]');if(graph){item.graphTop=graph.scrollTop||0;item.graphLeft=graph.scrollLeft||0;}if(results){item.resultTop=results.scrollTop||0;item.resultLeft=results.scrollLeft||0;}}
 function html(id,graph,results){
  const key=String(id??'').slice(0,512),item=model(key);capture(find(key),item);
  const percent=Math.round(preferences().ratio*100);
  return '<div class="graph-workspace" data-graph-workspace="'+escape(key)+'" style="--graph-pane-share:'+percent+'fr;--graph-result-share:'+(100-percent)+'fr"><section class="graph-workspace-graph" data-graph-pane tabindex="0" aria-label="Graph, selected point details and run statistics">'+graph+'</section><div class="graph-workspace-divider" data-graph-divider tabindex="0" role="separator" aria-orientation="horizontal" aria-label="Graph and result list divider" aria-valuemin="30" aria-valuemax="75" aria-valuenow="'+percent+'" aria-valuetext="'+percent+' percent graph, '+(100-percent)+' percent results" title="Drag to adjust graph and result list space. Use Up and Down arrows; double-click to reset."></div><section class="graph-workspace-results" data-graph-results tabindex="0" role="region" aria-label="Individual scan results">'+results+'</section></div>';
 }
 function property(node,name,value){if(node?.style?.setProperty)node.style.setProperty(name,value);else if(node?.style)node.style[name]=value;}
 function setRatio(node,ratio,persist=false){
  const value=normalize({ratio}).ratio,percent=Math.round(value*100),target=appState();
  if(target)target.graphWorkspacePreferences={ratio:value};
  for(const workspace of root.document?.querySelectorAll?.('[data-graph-workspace]')||[node]){property(workspace,'--graph-pane-share',percent+'fr');property(workspace,'--graph-result-share',(100-percent)+'fr');const divider=workspace.querySelector('[data-graph-divider]');divider?.setAttribute('aria-valuenow',String(percent));divider?.setAttribute('aria-valuetext',percent+' percent graph, '+(100-percent)+' percent results');}
  if(persist&&typeof saveState==='function')saveState();
  return value;
 }
 function measure(node){const results=node.querySelector('[data-graph-results]'),toolbar=results?.querySelector('.data-list-tools');if(results&&toolbar)property(results,'--graph-list-toolbar-height',Math.ceil(toolbar.getBoundingClientRect().height||0)+'px');}
 function revealSample(id,key){
  const node=find(String(id)),results=node?.querySelector('[data-graph-results]');
  if(!results||typeof dataListViews==='undefined'||typeof dataListRows!=='function'||typeof refreshDataList!=='function')return false;
  const config=[...dataListViews.values()].find(value=>value.graphId===String(id)&&typeof value.graphKey==='function');if(!config)return false;
  const rows=dataListRows(config),index=rows.findIndex(row=>config.graphKey(row)===key);if(index<0)return false;
  const page=Math.floor(index/(config.pageSize||100));
  if(typeof dataListPages!=='undefined'&&dataListPages.get(config.id)!==page){dataListPages.set(config.id,page);refreshDataList(config.id);measure(node);}
  const row=[...results.querySelectorAll('[data-result-sample-key]')].find(value=>value.dataset.resultSampleKey===key);if(!row)return false;
  const bounds=results.getBoundingClientRect(),rect=row.getBoundingClientRect(),toolbar=results.querySelector('.data-list-tools'),heading=results.querySelector('thead'),inset=(toolbar?.getBoundingClientRect().height||0)+(heading?.getBoundingClientRect().height||0);
  if(rect.top<bounds.top+inset)results.scrollTop+=rect.top-bounds.top-inset;
  else if(rect.bottom>bounds.bottom)results.scrollTop+=rect.bottom-bounds.bottom;
  capture(node,model(String(id)));return true;
 }
 function restore(){
  scheduled=false;
  for(const node of root.document?.querySelectorAll?.('[data-graph-workspace]')||[]){
   const item=model(node.dataset.graphWorkspace),graph=node.querySelector('[data-graph-pane]'),results=node.querySelector('[data-graph-results]');
   if(item.node!==node){if(item.node){resizeObserver?.unobserve(item.node);const oldResults=item.node.querySelector('[data-graph-results]');if(oldResults)resizeObserver?.unobserve(oldResults);}item.node=node;if(graph){graph.scrollTop=item.graphTop;graph.scrollLeft=item.graphLeft;}if(results){results.scrollTop=item.resultTop;results.scrollLeft=item.resultLeft;}resizeObserver?.observe(node);if(results)resizeObserver?.observe(results);}
   const percent=Math.round(preferences().ratio*100);property(node,'--graph-pane-share',percent+'fr');property(node,'--graph-result-share',(100-percent)+'fr');measure(node);
  }
 }
 function schedule(){if(scheduled)return;scheduled=true;if(typeof root.requestAnimationFrame==='function')root.requestAnimationFrame(restore);else restore();}
 function finish(event){if(!drag||event?.pointerId!=null&&event.pointerId!==drag.pointer)return;drag.node.classList.remove('graph-workspace-dragging');drag=null;if(typeof saveState==='function')saveState();}
 function initialize(){
  const document=root.document;if(initialized||!document?.addEventListener)return;initialized=true;
  if(typeof root.ResizeObserver==='function')resizeObserver=new root.ResizeObserver(entries=>{for(const entry of entries){const node=entry.target.closest?.('[data-graph-workspace]');if(node)measure(node);}});
  document.addEventListener('scroll',event=>{const node=event.target.closest?.('[data-graph-workspace]');if(node&&(event.target.hasAttribute('data-graph-pane')||event.target.hasAttribute('data-graph-results')))capture(node,model(node.dataset.graphWorkspace));},true);
  document.addEventListener('pointerdown',event=>{const divider=event.target.closest?.('[data-graph-divider]'),node=divider?.closest('[data-graph-workspace]');if(!node||event.button!==0)return;const bounds=node.getBoundingClientRect();if(!(bounds.height>10))return;event.preventDefault();drag={node,pointer:event.pointerId,top:bounds.top,height:bounds.height-10};node.classList.add('graph-workspace-dragging');try{divider.setPointerCapture?.(event.pointerId);}catch{}});
  document.addEventListener('pointermove',event=>{if(!drag||drag.pointer!==event.pointerId)return;event.preventDefault();setRatio(drag.node,(event.clientY-drag.top)/drag.height);});
  document.addEventListener('pointerup',finish);document.addEventListener('pointercancel',finish);document.addEventListener('lostpointercapture',finish);
  document.addEventListener('keydown',event=>{const divider=event.target.closest?.('[data-graph-divider]'),node=divider?.closest('[data-graph-workspace]');if(!node||!['ArrowUp','ArrowDown','Home','End','Enter'].includes(event.key))return;event.preventDefault();event.stopPropagation();const step=event.shiftKey ? .1 : .05,current=preferences().ratio;setRatio(node,event.key==='Home'?minimum:event.key==='End'?maximum:event.key==='Enter'?defaultRatio:current+(event.key==='ArrowUp'?-step:step),true);});
  document.addEventListener('dblclick',event=>{const divider=event.target.closest?.('[data-graph-divider]'),node=divider?.closest('[data-graph-workspace]');if(node){event.preventDefault();setRatio(node,defaultRatio,true);}});
  if(typeof root.MutationObserver==='function')new root.MutationObserver(schedule).observe(document.documentElement||document,{subtree:true,childList:true});
  root.addEventListener?.('resize',schedule);schedule();
 }
 const api=Object.freeze({html,normalize,preferences,mergePreferences,restore,revealSample,initialize});
 if(typeof module==='object'&&module.exports)module.exports=api;
 root.NetworkToolkitGraphWorkspace=api;initialize();
})(typeof window!=='undefined'?window:globalThis);
