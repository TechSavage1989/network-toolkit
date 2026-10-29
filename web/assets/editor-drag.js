(function(root){
'use strict';
// Ordinary tab drags use captured pointers, as Terminal does. Alt-drag retains
// the existing native cross-window path: HTML data carries one opaque ticket,
// never document bytes, paths, leases or capabilities. Its failed OS drop is a
// cancellation, never an inferred detach. Both paths keep the existing ACK gate.
const MIME='application/x-network-toolkit-editor-tab',TICKET=/^[0-9a-f]{32}$/;
function create(options={}){
 const document=options.document||root.document,editor=()=>options.editor?.()||root.NetworkToolkitFileEditor,windows=()=>options.windows?.()||root.NetworkToolkitEditorWindows;
 const tell=(message)=>{(options.message||root.toast)?.(message,'info');};
 let gesture=null,zone=null,indicator=null,installed=false,suppressClick=false;
 const eligible=(g=gesture)=>!editor()?.canReorderDocuments?.({ignoreDragPreparation:!!(g?.registering&&!g.ready)});
 const native=()=>windows()?.state?.enabled===true;
 function token(){const bytes=new Uint8Array(16);(options.crypto||root.crypto).getRandomValues(bytes);return [...bytes].map(n=>n.toString(16).padStart(2,'0')).join('');}
 function types(data){return Array.from(data?.types||[]).includes(MIME);}
 function read(data){if(!types(data))return '';const value=data.getData(MIME);return TICKET.test(value)?value:'';}
 function clearVisual(){document?.querySelectorAll?.('[data-editor-drop-before],[data-editor-drop-after]').forEach(el=>{el.removeAttribute('data-editor-drop-before');el.removeAttribute('data-editor-drop-after');});document?.querySelectorAll?.('.nt-editor-drag-over').forEach(el=>el.classList.remove('nt-editor-drag-over'));if(zone)zone.hidden=true;indicator?.remove?.();indicator=null;document?.querySelectorAll?.('.nt-editor-tabs-dragging').forEach(el=>el.classList.remove('nt-editor-tabs-dragging'));}
 function mount(){
  const host=document?.querySelector?.('.nt-editor-window');if(!host)return;
  for(const tabs of host.querySelectorAll?.('[data-ed-tabs]')||[])tabs.setAttribute('aria-description','Drag tabs to reorder. Drag away to detach; Escape cancels. Hold Alt before dragging to another editor window, or use Move / merge.');
  if(!zone||!host.contains(zone)){zone=document.createElement('div');zone.className='nt-editor-drag-detach';zone.dataset.editorDragDetach='';zone.hidden=true;zone.textContent='Drop tab here to open a new editor window';zone.setAttribute('aria-label',zone.textContent);host.append(zone);}
  // Per-view tabs can sit higher in compact/detached windows. Keep this
  // separate-window target inside the text area, below every visible tab strip.
  const tabs=[...(host.querySelectorAll?.('[data-ed-tabs]')||[])].map(el=>el.getBoundingClientRect()).filter(r=>r.width>0&&r.height>0);
  if(tabs.length&&zone.style){const box=host.getBoundingClientRect();zone.style.top=(Math.max(...tabs.map(r=>r.bottom))-box.top+12)+'px';}
 }
 function cancel(g){if(!g||g.cancelled)return;g.cancelled=true;if(g.mode==='native'&&g.native&&g.started)void windows().cancelDrag(g.ticket).catch(()=>{});}
 function retainScroll(g=gesture){for(const view of g?.scroll||[]){if(view.el.isConnected===false)continue;view.el.scrollTop=view.top;view.el.scrollLeft=view.left;}}
 function scroll(event){if(gesture?.started&&gesture.scroll?.some(view=>view.el===event.target))retainScroll();}
 function documentFor(uid){return [...(editor()?.documents?.values?.()||[])].find(d=>d.uid===uid);}
 function pointerValid(g){return !!g&&!g.cancelled&&g.tab?.isConnected!==false&&g.strip?.isConnected!==false&&eligible(g)&&documentFor(g.uid)===g.document&&windows()?.state?.owned?.[g.uid]===g.lease;}
 function releaseCapture(g){if(g?.tab)g.tab.draggable=true;try{if(g?.strip?.hasPointerCapture?.(g.pointer))g.strip.releasePointerCapture(g.pointer);}catch{}}
 function keepClickSuppressed(){suppressClick=true;(options.setTimeout||root.setTimeout)(()=>{suppressClick=false;},500);}
 function begin(event){
  if(event.isTrusted===false||event.button!==0||event.isPrimary===false||event.pointerType==='touch'||event.ctrlKey||event.metaKey||event.target.closest?.('[data-ed-close-tab],[data-ed="new"]'))return;
  const tab=event.target.closest?.('[data-ed-uid]');if(!tab||!eligible())return;
  if(gesture){cancel(gesture);releaseCapture(gesture);}mount();
  const mode=event.altKey?'native':'pointer',uid=tab.dataset.edUid;tab.draggable=mode==='native';
  const g={uid,ticket:mode==='native'?token():'',mode,native:native(),ready:!native(),registering:false,started:false,dropSeen:false,cancelled:false,error:'',tab,strip:tab.closest?.('[data-ed-tabs]'),sourceView:tab.closest?.('[data-ed-view]')?.dataset.edView,pointer:event.pointerId,x:event.clientX,y:event.clientY,document:documentFor(uid),lease:windows()?.state?.owned?.[uid]};gesture=g;
  if(mode==='pointer'){
   tab.draggable=false;
   if(!g.document||!g.strip){gesture=null;releaseCapture(g);return;}
   try{g.strip.setPointerCapture?.(g.pointer);}catch{cancel(g);gesture=null;releaseCapture(g);}
  }
 }
 function pointerDestination(event,g){
  const found=document.elementFromPoint?.(event.clientX,event.clientY)?.closest?.('[data-ed-tabs]'),host=g.tab.closest?.('.nt-editor-window');
  const target=found&&found.closest?.('.nt-editor-window')===host?found:g.strip,rect=target.getBoundingClientRect();
  if(event.clientY<rect.top-24||event.clientY>rect.bottom+24||event.clientX<rect.left-40||event.clientX>rect.right+40)return {kind:'detach'};
  const rows=[...(target.querySelectorAll?.('[data-ed-uid]')||[])];if(!rows.length)return {kind:'reorder',before:null,view:target.closest?.('[data-ed-view]')?.dataset.edView};
  const row=rows.find(el=>event.clientX<el.getBoundingClientRect().right)||rows.at(-1),r=row.getBoundingClientRect(),after=event.clientX>=r.left+r.width/2;
  let next=row.nextElementSibling;while(next&&!next.dataset?.edUid)next=next.nextElementSibling;
  return {kind:'reorder',before:after?(next?.dataset.edUid||null):row.dataset.edUid,view:target.closest?.('[data-ed-view]')?.dataset.edView,row,after};
 }
 function pointerMove(event){
  const g=gesture;if(g?.mode!=='pointer'||event.pointerId!==g.pointer)return;
  if(!pointerValid(g)){abortPointer();return;}
  if(!g.started&&Math.hypot(event.clientX-g.x,event.clientY-g.y)<6)return;
  if(!g.started){const host=g.tab.closest?.('.nt-editor-window');g.scroll=[...(host?.querySelectorAll?.('[data-ed-text],[data-compare-text],.cm-scroller')||[])].map(el=>({el,top:el.scrollTop,left:el.scrollLeft}));g.started=true;}
  event.preventDefault();g.destination=pointerDestination(event,g);clearVisual();g.strip.classList.add('nt-editor-tabs-dragging');
  const d=g.destination;if(d.kind==='reorder'&&d.row)d.row.setAttribute(d.after?'data-editor-drop-after':'data-editor-drop-before','');
  indicator=document.createElement('div');indicator.className='terminal-tab-drag-indicator nt-editor-tab-drag-indicator';indicator.setAttribute('role','status');indicator.textContent=d.kind==='detach'?(g.native?'Release to detach this document · Esc cancels':'Native editor windows are unavailable'):'Reorder document tab · Esc cancels';
  indicator.style.left=Math.max(6,Math.min((root.innerWidth||800)-280,event.clientX+12))+'px';indicator.style.top=Math.max(6,Math.min((root.innerHeight||600)-42,event.clientY+16))+'px';document.body?.append(indicator);
 }
 function abortPointer(){const g=gesture;if(g?.mode!=='pointer')return;gesture=null;cancel(g);retainScroll(g);clearVisual();releaseCapture(g);if(g.started)keepClickSuppressed();}
 function pointerUp(event){
  const g=gesture;if(g?.mode!=='pointer'||event.pointerId!==g.pointer)return;
  const valid=pointerValid(g),moved=g.started,d=moved?pointerDestination(event,g):null;gesture=null;clearVisual();releaseCapture(g);retainScroll(g);
  if(!moved)return;keepClickSuppressed();event.preventDefault();event.stopPropagation?.();if(!valid)return;
  if(d?.kind==='detach'){if(!g.native){tell('Use a desktop editor window to detach documents.');return;}void Promise.resolve().then(()=>{if(!pointerValid(g))throw Error('The document changed before the move could begin.');return windows().move([g.uid],'new');}).catch(error=>tell('The document could not detach. '+(error.message||String(error))));}
  else if(d?.kind==='reorder'){if(['left','right'].includes(d.view)&&d.view!==g.sourceView)editor().moveDocumentToView?.(g.uid,d.view);editor().reorderDocument(g.uid,d.before);}
 }
 function start(event){
  const tab=event.target.closest?.('[data-ed-uid]'),g=gesture;
  if(event.isTrusted===false||!g||g.mode!=='native'||g.cancelled||!tab||tab.dataset.edUid!==g.uid||!eligible()||!event.dataTransfer){event.preventDefault();return;}
  // A tab move must not inherit Chromium's text-area edge autoscroll while the
  // pointer crosses a pane on the way to a window/drop target.
  const host=tab.closest?.('.nt-editor-window');g.scroll=[...(host?.querySelectorAll?.('[data-ed-text],[data-compare-text],.cm-scroller')||[])].map(el=>({el,top:el.scrollTop,left:el.scrollLeft}));
  g.started=true;tab.addEventListener?.('dragend',end,{once:true});event.dataTransfer.clearData();event.dataTransfer.setData(MIME,g.ticket);event.dataTransfer.effectAllowed='move';
  // Registration starts only for a real drag, never an ordinary tab click.
  // The receiver coordinator retries only a not-yet-registered ticket briefly;
  // no document can move before an authenticated broker claim and existing ACK.
  if(g.native){g.registering=true;g.promise=Promise.resolve().then(()=>windows().beginDrag(g.uid,g.ticket)).then(()=>{g.ready=true;if(g.cancelled)void windows().cancelDrag(g.ticket).catch(()=>{});}).catch(error=>{g.error=error.message||String(error);if(!g.dropSeen)tell(g.error);}).finally(()=>{g.registering=false;});}
  if(zone&&g.native)zone.hidden=false;
 }
 function beforeUID(event){
  const tab=event.target.closest?.('[data-ed-uid]');if(!tab)return null;
  const rect=tab.getBoundingClientRect();if(event.clientX<rect.left+rect.width/2)return tab.dataset.edUid;
  let next=tab.nextElementSibling;while(next&&!next.dataset?.edUid)next=next.nextElementSibling;return next?.dataset.edUid||null;
 }
 function over(event){
  if(!types(event.dataTransfer))return;
  const host=event.target.closest?.('.nt-editor-window');event.preventDefault();event.dataTransfer.dropEffect='none';
  if(!host||!eligible())return;
  if(gesture?.started){
   if(event.target.closest?.('[data-editor-drag-detach]')&&gesture.native){event.dataTransfer.dropEffect='move';return;}
   if(!event.target.closest?.('[data-ed-tabs]'))return;
   event.dataTransfer.dropEffect='move';document?.querySelectorAll?.('[data-editor-drop-before]').forEach(el=>el.removeAttribute('data-editor-drop-before'));
   const uid=beforeUID(event);if(uid)host.querySelector('[data-ed-uid="'+uid+'"]')?.setAttribute('data-editor-drop-before','');
  }else if(native()){event.dataTransfer.dropEffect='move';host.classList.add('nt-editor-drag-over');}
 }
 async function drop(event){
  if(!types(event.dataTransfer))return;event.preventDefault();event.stopPropagation();
  const ticket=read(event.dataTransfer),host=event.target.closest?.('.nt-editor-window'),g=gesture;
  if(!ticket||!host||!eligible()){event.dataTransfer.dropEffect='none';clearVisual();return;}
  if(g?.started&&g.ticket===ticket){
   const detach=!!event.target.closest?.('[data-editor-drag-detach]');
   if(!detach&&!event.target.closest?.('[data-ed-tabs]')){event.dataTransfer.dropEffect='none';clearVisual();return;}
   g.dropSeen=true;event.dataTransfer.dropEffect='move';retainScroll(g);clearVisual();
   if(detach){if(!g.native||g.cancelled)return;try{await windows().detachDrag(ticket);}catch(error){tell(error.message||String(error));cancel(g);}}
   else{const before=beforeUID(event),view=event.target.closest?.('[data-ed-view]')?.dataset.edView;await g.promise;if(!g.cancelled){if(['left','right'].includes(view)&&view!==g.sourceView)editor().moveDocumentToView?.(g.uid,view);editor().reorderDocument(g.uid,before);}cancel(g);if(gesture===g)gesture=null;}
   return;
  }
  clearVisual();if(!native()){event.dataTransfer.dropEffect='none';return;}
  event.dataTransfer.dropEffect='move';
  try{await windows().claimDrag(ticket);}catch(error){tell('The tab was not moved. '+(error.message||String(error)));}
 }
 function leave(event){if(types(event.dataTransfer)&&!event.relatedTarget?.closest?.('.nt-editor-window'))document?.querySelectorAll?.('.nt-editor-drag-over').forEach(el=>el.classList.remove('nt-editor-drag-over'));}
 function end(event){const g=gesture;clearVisual();if(!g)return;retainScroll(g);gesture=null;if(g.started){suppressClick=true;(options.setTimeout||root.setTimeout)(()=>{suppressClick=false;},0);}if(!g.dropSeen&&event.dataTransfer?.dropEffect!=='move'){cancel(g);if(g.started)tell('Original tab kept. Drop on another editor window or the in-app detach target to move it.');}}
 function up(event){if(gesture?.mode==='pointer'){if(event)pointerUp(event);else abortPointer();return;}if(gesture&&!gesture.started){cancel(gesture);gesture=null;}}
 function escape(event){if(event.key==='Escape'&&gesture){if(gesture.mode==='pointer'){event.preventDefault?.();event.stopImmediatePropagation?.();abortPointer();}else{retainScroll();cancel(gesture);clearVisual();}}}
 function install(){if(installed||!document)return;installed=true;document.addEventListener('pointerdown',begin,true);document.addEventListener('pointermove',pointerMove,true);document.addEventListener('pointerup',up,true);document.addEventListener('pointercancel',event=>{if(gesture?.mode==='pointer')abortPointer();else up(event);},true);document.addEventListener('lostpointercapture',event=>{if(gesture?.mode==='pointer'&&event.pointerId===gesture.pointer)abortPointer();},true);root.addEventListener?.('blur',abortPointer);document.addEventListener('dragstart',start,true);document.addEventListener('dragover',over,true);document.addEventListener('dragleave',leave,true);document.addEventListener('drop',event=>{void drop(event);},true);document.addEventListener('dragend',end,true);document.addEventListener('keydown',escape,true);document.addEventListener('click',event=>{if(suppressClick&&event.target.closest?.('[data-ed-uid]')){event.preventDefault();event.stopImmediatePropagation();}},true);}
 document?.addEventListener('scroll',scroll,true);
 install();return Object.freeze({mount,begin,start,over,drop,end,leave,up,pointerMove,abortPointer,escape,read,get state(){return {armed:!!gesture,started:!!gesture?.started,ready:!!gesture?.ready,mode:gesture?.mode||'',uid:gesture?.uid||''};}});
}
root.NetworkToolkitEditorDrag=create();if(typeof module==='object'&&module.exports)module.exports={create,MIME};
})(globalThis);
