/* Document List (owner request, like Notepad++'s Document List): the open
   documents of this Text Editor as a list on the editor's left rail, beside
   Connect and Files (editor-files-sidebar.js), so it shares that sidebar's
   width, side and open/closed setting. A row shows an unsaved marker, the
   file name and its extension in a column of its own; the active document is
   highlighted. Rows follow the tab order, grouped by view while the editor
   shows two views. Clicking a row (or Enter on it) activates that tab, Up and
   Down move through the rows, and the right-click menu (or Shift+F10) is the
   tab strip's own menu. The list reads the editor's documents through the
   `source` it is given and keeps nothing of its own. */
(function(root){
'use strict';
const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

// "router.cfg" -> {base:"router", ext:"cfg"}; a leading dot is part of the name.
function splitName(name){
 const value=String(name||'Untitled'),at=value.lastIndexOf('.');
 return at>0&&at<value.length-1?{base:value.slice(0,at),ext:value.slice(at+1)}:{base:value,ext:''};
}

// The rows of a model {views:[{side,label,documents:[...]}]}: a heading per
// view when there are two, then the documents of that view in tab order.
function rows(model){
 const views=Array.isArray(model?.views)?model.views:[],out=[];
 for(const view of views){
  if(views.length>1)out.push({group:true,side:view.side,label:view.label,count:view.documents.length});
  for(const d of view.documents)out.push({...d,side:view.side,...splitName(d.name)});
 }
 return out;
}

function create(source){
 let host=null,list=null,signature='',focusUID='';

 function mount(element){
  if(!element||host===element)return;
  host=element;
  host.innerHTML='<div class="nt-editor-documents-head" aria-hidden="true"><span></span><span>Name</span><span>Ext.</span></div><div class="nt-editor-documents-list" role="listbox" aria-label="Open documents" data-editor-documents-list></div><p class="nt-editor-documents-empty" data-editor-documents-empty hidden>No documents are open.</p>';
  list=host.querySelector('[data-editor-documents-list]');
  list.addEventListener('click',event=>{const row=event.target.closest('[data-ed-doc-uid]');if(row)choose(row,false);});
  list.addEventListener('keydown',keydown);
  list.addEventListener('focusin',event=>{const row=event.target.closest('[data-ed-doc-uid]');if(row)focusUID=row.dataset.edDocUid;});
  list.addEventListener('contextmenu',event=>{
   const row=event.target.closest('[data-ed-doc-uid]');if(!row)return;
   event.preventDefault();event.stopPropagation();
   menu(row,event.clientX,event.clientY);
  });
  signature='';render();
 }

 function options(){return list?[...list.querySelectorAll('[data-ed-doc-uid]')]:[];}

 function render(){
  if(!host)return;
  const model=source.model(),items=rows(model);
  const next=JSON.stringify(items.map(row=>row.group?['g',row.side,row.count]:[row.uid,row.id,row.name,row.title,row.dirty,row.saving,row.active,row.shown]));
  if(next===signature)return;
  signature=next;
  const current=root.document.activeElement,focused=list.contains(current)?current.closest?.('[data-ed-doc-uid]')?.dataset.edDocUid||focusUID:'';
  const documents=items.filter(row=>!row.group);
  if(!documents.some(row=>row.uid===focusUID))focusUID=documents.find(row=>row.active)?.uid||documents[0]?.uid||'';
  let html='',open=false;
  for(const row of items){
   if(row.group){
    if(open)html+='</div>';
    html+='<div role="group" class="nt-editor-documents-view" aria-labelledby="ntEditorDocumentsView-'+escape(row.side)+'"><div class="nt-editor-documents-group" id="ntEditorDocumentsView-'+escape(row.side)+'" role="presentation">'+escape(row.label)+'</div>';
    open=true;continue;
   }
   const state=(row.saving?', saving':row.dirty?', unsaved changes':'')+(row.side==='left'||row.side==='right'?', '+row.side+' view':'');
   html+='<div role="option" class="nt-editor-documents-row'+(row.active?' is-active':'')+(row.shown?' is-shown':'')+(row.dirty?' is-dirty':'')+'" id="ntEditorDocumentsRow-'+escape(row.uid)+'" data-ed-doc-uid="'+escape(row.uid)+'" data-ed-doc-id="'+escape(row.id)+'" aria-selected="'+(row.active?'true':'false')+'" tabindex="'+(row.uid===focusUID?'0':'-1')+'" title="'+escape(row.title||row.name)+'" aria-label="'+escape(row.name+state)+'">'+
    '<span class="nt-editor-documents-mark" aria-hidden="true">'+(row.saving?'◌':row.dirty?'●':'')+'</span><span class="nt-editor-documents-name">'+escape(row.base)+'</span><span class="nt-editor-documents-ext">'+escape(row.ext)+'</span></div>';
  }
  if(open)html+='</div>';
  list.innerHTML=html;
  host.querySelector('[data-editor-documents-empty]').hidden=documents.length>0;
  if(focused)(options().find(row=>row.dataset.edDocUid===focused)||options().find(row=>row.tabIndex===0))?.focus({preventScroll:true});
  reveal(options().find(row=>row.classList.contains('is-active')));
 }

 // Scroll the list (never the page) so a row is in view.
 function reveal(row){
  if(!row||!list.clientHeight)return;
  const box=row.getBoundingClientRect(),area=list.getBoundingClientRect();
  if(box.top<area.top)list.scrollTop-=area.top-box.top;
  else if(box.bottom>area.bottom)list.scrollTop+=box.bottom-area.bottom;
 }

 function rove(row){
  if(!row)return;
  for(const option of options())option.tabIndex=option===row?0:-1;
  focusUID=row.dataset.edDocUid;row.focus({preventScroll:true});reveal(row);
 }

 function choose(row,keyboard){
  focusUID=row.dataset.edDocUid;
  if(source.activate(row.dataset.edDocId)===false)return;
  render();
  source.chosen?.({keyboard});
 }

 function menu(row,x,y){
  focusUID=row.dataset.edDocUid;
  source.menu(row.dataset.edDocUid,x,y,()=>options().find(option=>option.dataset.edDocUid===focusUID)||null);
 }

 function keydown(event){
  const row=event.target.closest?.('[data-ed-doc-uid]');if(!row)return;
  const all=options(),at=all.indexOf(row);
  if(event.key==='ContextMenu'||(event.shiftKey&&event.key==='F10')){
   event.preventDefault();event.stopPropagation();
   const box=row.getBoundingClientRect();menu(row,box.left+12,box.bottom);return;
  }
  const target={ArrowDown:all[at+1],ArrowUp:all[at-1],Home:all[0],End:all.at(-1)}[event.key];
  if(['ArrowDown','ArrowUp','Home','End'].includes(event.key)){event.preventDefault();event.stopPropagation();rove(target||row);return;}
  if(event.key==='Enter'||event.key===' '){event.preventDefault();event.stopPropagation();choose(row,true);return;}
  if(event.key==='Escape'){event.preventDefault();event.stopPropagation();(source.escape||source.focusText)?.();}
 }

 function focus(){
  render();
  const row=options().find(option=>option.dataset.edDocUid===focusUID)||options().find(option=>option.classList.contains('is-active'))||options()[0];
  if(row)rove(row);else list?.focus?.({preventScroll:true});
  return !!row;
 }

 return Object.freeze({mount,render,focus,get host(){return host;}});
}

const api={create,rows,splitName};
root.NTKEditorDocumentList=Object.freeze(api);
if(typeof module==='object'&&module.exports)module.exports=api;
})(globalThis);
