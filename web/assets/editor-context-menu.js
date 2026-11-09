/* Text Editor right-click menu (owner request, v1.20.14).
 * Right-clicking the document text (or pressing the Menu key / Shift+F10 in
 * it) opens the app's own menu instead of the browser's, styled like the
 * Terminal menu: Undo/Redo, Cut/Copy/Paste/Select all, Send to terminal,
 * Find/Replace/Go to line, and the line and text tools. Commands reuse the
 * editor's existing toolbar actions (NetworkToolkitFileEditor.textMenu.run),
 * so their rules for frozen documents, loading, and comparison still apply.
 * Clipboard access uses the same browser clipboard calls as the Terminal;
 * edits go through the editor's replaceSelection, so Undo works. */
(function(root){
'use strict';
const document=root.document;if(!document)return;
let menu=null,restore=null;
const api=()=>root.NetworkToolkitFileEditor?.textMenu;
const ico=name=>typeof root.icon==='function'?root.icon(name):(typeof icon==='function'?icon(name):'');
function toastMessage(message,kind){try{if(typeof toast==='function')toast(message,kind);}catch{}}
function items(s){const sel=s.selection,compares=api()?.compareActions?.()||[];return [
 ['undo','Undo','undo','Ctrl+Z',s.editable&&s.canUndo],['redo','Redo','redo','Ctrl+Y',s.editable&&s.canRedo],null,
 ['cut','Cut','cut','Ctrl+X',s.editable&&sel],['copy','Copy','copy','Ctrl+C',sel],['paste','Paste','clipboard','Ctrl+V',s.editable&&!!navigator.clipboard?.readText],['selectAll','Select all','selectAll','Ctrl+A',!s.empty],null,
 ['sendTerminal',sel?'Send selection to terminal…':'Send document to terminal…','sendTerminal','Ctrl+Shift+Enter',!s.empty&&!!root.NetworkToolkitEditorSendTerminal?.available?.()],null,
 ['find','Find','search','Ctrl+F',s.open],['replace','Replace','search','Ctrl+H',s.editable],['goto','Go to line','goto','Ctrl+G',s.open],null,
 ['comment','Toggle comment','comment','Ctrl+/',s.editable],['duplicate','Duplicate line','duplicate','Ctrl+D',s.editable],['upper','UPPERCASE','textCase','',s.editable&&sel],['lower','lowercase','textCase','',s.editable&&sel],['format','Format document','format','Ctrl+Shift+F',s.editable&&!s.empty],null,
 ...(api()?.conversions||[]).map(([action,label])=>[action,label,'textCase','',s.convertible&&sel]),
 ...(compares.length?[null,...compares.map(([action,label,enabled,checked])=>[action,label,'compare','',enabled,checked])]:[])];}
function close(focus=true){if(!menu)return;menu.remove();menu=null;if(focus)restore?.();restore=null;}
function open(x,y){const t=api();if(!t)return false;close(false);const s=t.state();if(!s.open)return false;
 menu=document.createElement('div');menu.id='editorContextMenu';menu.className='terminal-context-menu nt-editor-context-menu';menu.setAttribute('role','menu');menu.setAttribute('aria-label','Text Editor actions');
 menu.innerHTML=items(s).map(item=>item?'<button type="button" role="'+(item[5]===undefined?'menuitem':'menuitemcheckbox')+'" tabindex="-1" data-editor-menu-action="'+item[0]+'"'+(item[5]===undefined?'':' aria-checked="'+item[5]+'"')+(item[4]?'':' disabled')+'>'+(item[5]===undefined?ico(item[2]):'<span class="nt-editor-menu-check" aria-hidden="true">'+(item[5]?'✓':'')+'</span>')+'<span>'+item[1]+'</span>'+(item[3]?'<kbd>'+item[3]+'</kbd>':'')+'</button>':'<div role="separator" class="terminal-menu-separator"></div>').join('');
 document.body.append(menu);const r=menu.getBoundingClientRect();menu.style.left=Math.max(6,Math.min(x,innerWidth-r.width-6))+'px';menu.style.top=Math.max(6,Math.min(y,innerHeight-r.height-6))+'px';
 restore=()=>api()?.focus();menu.querySelector('button:not(:disabled)')?.focus({preventScroll:true});return true;}
async function run(action){const t=api();if(!t)return;close(false);t.focus();
 const editor=root.NetworkToolkitFileEditor,owner=editor?.active;
 const sameDocument=()=>editor?.active===owner;
 try{switch(action){
  case 'copy':case 'cut':{
   const cut=action==='cut',copy=t.copy?.(cut);
   if(copy!==undefined){await copy;break;}
   const text=t.selection();if(!text)break;
   await navigator.clipboard.writeText(text);
   if(cut&&sameDocument())await t.replace('');
   break;
  }
  case 'paste':{const text=await navigator.clipboard.readText();if(text&&sameDocument())await t.replace(text);break;}
  case 'selectAll':t.selectAll();break;
  default:t.run(action);
 }}catch{toastMessage(action==='paste'?'The clipboard could not be read. Use Ctrl+V instead.':'The clipboard is not available. Use the keyboard shortcut instead.','error');}
 if(sameDocument()&&!['find','replace','goto','sendTerminal','compareOptions','compareOnly'].includes(action))t.focus();}
document.addEventListener('contextmenu',event=>{const t=api();if(!t?.target(event.target))return;event.preventDefault();event.stopImmediatePropagation();let x=event.clientX,y=event.clientY;if(!Number.isFinite(x)||!Number.isFinite(y)||x<=0&&y<=0){const r=event.target.getBoundingClientRect();x=r.left+24;y=r.top+24;}t.focusTarget?.(event.target);open(x,y);},true);
document.addEventListener('keydown',event=>{
 if(menu){const list=[...menu.querySelectorAll('button:not(:disabled)')],at=list.indexOf(document.activeElement);
  if(event.key==='Escape'||event.key==='Tab'){event.preventDefault();event.stopImmediatePropagation();close(true);return;}
  if(['ArrowDown','ArrowUp','Home','End'].includes(event.key)){event.preventDefault();event.stopImmediatePropagation();const next=event.key==='Home'?0:event.key==='End'?list.length-1:(at+(event.key==='ArrowUp'?-1:1)+list.length)%list.length;list[next]?.focus({preventScroll:true});return;}
  return;}
 const t=api();if((event.key==='ContextMenu'||event.shiftKey&&event.key==='F10')&&t?.target(event.target)){event.preventDefault();event.stopImmediatePropagation();const r=event.target.getBoundingClientRect();t.focusTarget?.(event.target);open(r.left+24,r.top+24);}
},true);
document.addEventListener('click',event=>{const button=event.target.closest?.('[data-editor-menu-action]');if(!button||!menu?.contains(button))return;event.preventDefault();event.stopImmediatePropagation();if(!button.disabled)void run(button.dataset.editorMenuAction);},true);
document.addEventListener('pointerdown',event=>{if(menu&&!menu.contains(event.target))close(false);},true);
root.addEventListener('resize',()=>close(false));root.addEventListener('blur',()=>close(false));
document.addEventListener('scroll',event=>{if(menu&&!menu.contains(event.target))close(false);},true);
root.NetworkToolkitEditorContextMenu=Object.freeze({open,close,isOpen:()=>!!menu});
})(window);
