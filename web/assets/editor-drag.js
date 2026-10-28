(function(root){
'use strict';
// HTML drag data contains one short-lived opaque broker ticket only. Document
// bytes, paths, leases and native/session capabilities never cross this boundary.
// Windowed WebView2 routes OS drag input to its own HWND; no native hook or
// composition controller is installed. A bare-desktop/foreign drop is a cancel,
// never an inferred detach: dropEffect=none also represents Escape/rejection.
const MIME='application/x-network-toolkit-editor-tab',TICKET=/^[0-9a-f]{32}$/;
function create(options={}){
 const document=options.document||root.document,editor=()=>options.editor?.()||root.NetworkToolkitFileEditor,windows=()=>options.windows?.()||root.NetworkToolkitEditorWindows;
 const tell=(message)=>{(options.message||root.toast)?.(message,'info');};
 let gesture=null,zone=null,installed=false,suppressClick=false;
 const eligible=(g=gesture)=>!editor()?.canReorderDocuments?.({ignoreDragPreparation:!!(g?.registering&&!g.ready)});
 const native=()=>windows()?.state?.enabled===true;
 function token(){const bytes=new Uint8Array(16);(options.crypto||root.crypto).getRandomValues(bytes);return [...bytes].map(n=>n.toString(16).padStart(2,'0')).join('');}
 function types(data){return Array.from(data?.types||[]).includes(MIME);}
 function read(data){if(!types(data))return '';const value=data.getData(MIME);return TICKET.test(value)?value:'';}
 function clearVisual(){document?.querySelectorAll?.('[data-editor-drop-before]').forEach(el=>el.removeAttribute('data-editor-drop-before'));document?.querySelectorAll?.('.nt-editor-drag-over').forEach(el=>el.classList.remove('nt-editor-drag-over'));if(zone)zone.hidden=true;}
 function mount(){
  const host=document?.querySelector?.('.nt-editor-window');if(!host)return;
  if(!zone||!host.contains(zone)){zone=document.createElement('div');zone.className='nt-editor-drag-detach';zone.dataset.editorDragDetach='';zone.hidden=true;zone.textContent='Drop tab here to open a new editor window';zone.setAttribute('aria-label',zone.textContent);host.append(zone);}
  // Per-view tabs can sit higher in compact/detached windows. Keep this
  // separate-window target inside the text area, below every visible tab strip.
  const tabs=[...(host.querySelectorAll?.('[data-ed-tabs]')||[])].map(el=>el.getBoundingClientRect()).filter(r=>r.width>0&&r.height>0);
  if(tabs.length&&zone.style){const box=host.getBoundingClientRect();zone.style.top=(Math.max(...tabs.map(r=>r.bottom))-box.top+12)+'px';}
 }
 function cancel(g){if(!g||g.cancelled)return;g.cancelled=true;if(g.native&&g.started)void windows().cancelDrag(g.ticket).catch(()=>{});}
 function retainScroll(g=gesture){for(const view of g?.scroll||[]){if(view.el.isConnected===false)continue;view.el.scrollTop=view.top;view.el.scrollLeft=view.left;}}
 function scroll(event){if(gesture?.started&&gesture.scroll?.some(view=>view.el===event.target))retainScroll();}
 function begin(event){
  if(event.isTrusted===false||event.button!==0||event.target.closest?.('[data-ed-close-tab],[data-ed="new"]'))return;
  const tab=event.target.closest?.('[data-ed-uid]');if(!tab||!eligible())return;
  if(gesture)cancel(gesture);mount();
  const g={uid:tab.dataset.edUid,ticket:token(),native:native(),ready:!native(),registering:false,started:false,dropSeen:false,cancelled:false,error:''};gesture=g;
 }
 function start(event){
  const tab=event.target.closest?.('[data-ed-uid]'),g=gesture;
  if(event.isTrusted===false||!g||g.cancelled||!tab||tab.dataset.edUid!==g.uid||!eligible()||!event.dataTransfer){event.preventDefault();return;}
  // A tab move must not inherit Chromium's text-area edge autoscroll while the
  // pointer crosses a pane on the way to a window/drop target.
  const host=tab.closest?.('.nt-editor-window');g.scroll=[...(host?.querySelectorAll?.('[data-ed-text],[data-compare-text]')||[])].map(el=>({el,top:el.scrollTop,left:el.scrollLeft}));
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
   else{const before=beforeUID(event),view=event.target.closest?.('[data-ed-view]')?.dataset.edView;await g.promise;if(!g.cancelled){if(['left','right'].includes(view))editor().moveDocumentToView?.(g.uid,view);editor().reorderDocument(g.uid,before);}cancel(g);if(gesture===g)gesture=null;}
   return;
  }
  clearVisual();if(!native()){event.dataTransfer.dropEffect='none';return;}
  event.dataTransfer.dropEffect='move';
  try{await windows().claimDrag(ticket);}catch(error){tell('The tab was not moved. '+(error.message||String(error)));}
 }
 function leave(event){if(types(event.dataTransfer)&&!event.relatedTarget?.closest?.('.nt-editor-window'))document?.querySelectorAll?.('.nt-editor-drag-over').forEach(el=>el.classList.remove('nt-editor-drag-over'));}
 function end(event){const g=gesture;clearVisual();if(!g)return;retainScroll(g);gesture=null;if(g.started){suppressClick=true;(options.setTimeout||root.setTimeout)(()=>{suppressClick=false;},0);}if(!g.dropSeen&&event.dataTransfer?.dropEffect!=='move'){cancel(g);if(g.started)tell('Original tab kept. Drop on another editor window or the in-app detach target to move it.');}}
 function up(){if(gesture&&!gesture.started){cancel(gesture);gesture=null;}}
 function escape(event){if(event.key==='Escape'&&gesture){retainScroll();cancel(gesture);clearVisual();}}
 function install(){if(installed||!document)return;installed=true;document.addEventListener('pointerdown',begin,true);document.addEventListener('pointerup',up,true);document.addEventListener('pointercancel',up,true);document.addEventListener('dragstart',start,true);document.addEventListener('dragover',over,true);document.addEventListener('dragleave',leave,true);document.addEventListener('drop',event=>{void drop(event);},true);document.addEventListener('dragend',end,true);document.addEventListener('keydown',escape,true);document.addEventListener('click',event=>{if(suppressClick&&event.target.closest?.('[data-ed-uid]')){event.preventDefault();event.stopImmediatePropagation();}},true);}
 document?.addEventListener('scroll',scroll,true);
 install();return Object.freeze({mount,begin,start,over,drop,end,leave,up,escape,read,get state(){return {armed:!!gesture,started:!!gesture?.started,ready:!!gesture?.ready,uid:gesture?.uid||''};}});
}
root.NetworkToolkitEditorDrag=create();if(typeof module==='object'&&module.exports)module.exports={create,MIME};
})(globalThis);
