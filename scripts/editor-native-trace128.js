/* Test-only, passive and bounded. Never reads text, payloads, paths or tokens. */
(function(root){
'use strict';
function create(document, now, coordinator, drag){
 const events=[];let total=0;
 const target=el=>{for(const [selector,name] of [['[data-ed-open]','tab-open'],['[data-ed-close-tab]','tab-close'],['[data-ed-tabs]','tabs'],['[data-ed-text],.cm-content,.cm-scroller','text'],['[data-ed-files-sidebar]','files'],['.nt-editor-window','editor']])if(el?.closest?.(selector))return name;return 'outside';};
 const effects=new Set(['none','copy','move','link','copyLink','copyMove','linkMove','all','uninitialized']);
 function record(e){
  const s=coordinator()||{},g=drag()||{};
  const p=Number.isFinite(e.clientX)&&Number.isFinite(e.clientY)?document.elementFromPoint?.(e.clientX,e.clientY):null;
  const row={at:Math.round(now()),type:e.type,key:['Alt','Escape','Control','Shift','Meta'].includes(e.key)?e.key:'',trusted:e.isTrusted===true,alt:!!e.altKey,ctrl:!!e.ctrlKey,shift:!!e.shiftKey,meta:!!e.metaKey,target:target(e.target),hit:target(p),x:Number.isFinite(e.clientX)?e.clientX:null,y:Number.isFinite(e.clientY)?e.clientY:null,focused:document.hasFocus?.()===true,prevented:!!e.defaultPrevented,editorMIME:Array.from(e.dataTransfer?.types||[]).includes('application/x-network-toolkit-editor-tab'),effect:effects.has(e.dataTransfer?.dropEffect)?e.dataTransfer.dropEffect:'',allowed:effects.has(e.dataTransfer?.effectAllowed)?e.dataTransfer.effectAllowed:'',armed:!!g.armed,started:!!g.started,ready:!!g.ready,mode:['native','pointer'].includes(g.mode)?g.mode:'',running:!!s.running,preparing:!!s.dragPreparing,outgoing:Number(s.outgoing)||0,incoming:Number(s.incoming)||0,hasError:!!s.error};
  total++;events.push(row);if(events.length>256)events.shift();
 }
 for(const type of ['pointerdown','pointerup','pointercancel','dragstart','dragenter','dragover','drop','dragend','keydown','keyup'])document.addEventListener(type,record,false);
 return ()=>({total,events:events.map(e=>({...e}))});
}
if(typeof module==='object'&&module.exports)module.exports={create};
else root.__nativeEditorTrace128=create(root.document,()=>Date.now(),()=>root.NetworkToolkitEditorWindows?.state,()=>root.NetworkToolkitEditorDrag?.state);
})(globalThis);
