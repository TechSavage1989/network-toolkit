/* Text Editor right-click menu (owner request, v1.20.14) and Tools menu
 * (owner request, v1.21.53).
 * Right-clicking the document text (or pressing the Menu key / Shift+F10 in
 * it) opens the app's own menu instead of the browser's, styled like the
 * Terminal menu: Undo/Redo, Cut/Copy/Paste/Select all, Send to terminal,
 * Find/Replace/Go to line, the text tool groups (Lines, Encode / decode,
 * Format, Device configs) and Compare tools as submenus. The toolbar's Tools
 * button opens the same tool groups on their own. Every tool item shows a
 * one-line description, also its tooltip, and its shortcut; the tools come
 * from editor-text-tools.js through NetworkToolkitFileEditor.textMenu, so
 * their rules for frozen documents, loading, and comparison still apply.
 * Clipboard access uses the same browser clipboard calls as the Terminal;
 * edits go through the editor's replaceSelection, so Undo works. */
(function(root){
'use strict';
const document=root.document;if(!document)return;
let menu=null,primary=null,submenu=null,trigger=null,hoverTimer=0,anchor=null,suppressAnchor=null;
const api=()=>root.NetworkToolkitFileEditor?.textMenu;
const ico=name=>typeof root.icon==='function'?root.icon(name):(typeof icon==='function'?icon(name):'');
const html=value=>String(value??'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
function toastMessage(message,kind){try{if(typeof toast==='function')toast(message,kind);}catch{}}
// The text tool groups, as submenus.
function toolGroups(){
 return (api()?.toolGroups?.()||[]).map(group=>({group:'tools-'+group.id,label:group.label,icon:group.icon,description:group.description,
  actions:group.tools.map(tool=>[tool.id,tool.label,group.icon,tool.shortcut||'',tool.enabled,undefined,tool.description])}));
}
function items(s){const sel=s.selection,compares=api()?.compareActions?.()||[];
 return [
 ['undo','Undo','undo','Ctrl+Z',s.editable&&s.canUndo],['redo','Redo','redo','Ctrl+Y',s.editable&&s.canRedo],null,
 ['cut','Cut','cut','Ctrl+X',s.editable&&sel],['copy','Copy','copy','Ctrl+C',sel],['paste','Paste','clipboard','Ctrl+V',s.editable&&!!navigator.clipboard?.readText],['selectAll','Select all','selectAll','Ctrl+A',!s.empty],null,
 ['sendTerminal',sel?'Send selection to terminal…':'Send document to terminal…','sendTerminal','Ctrl+Shift+Enter',!s.empty&&!!root.NetworkToolkitEditorSendTerminal?.available?.()],null,
 ['find','Find','search','Ctrl+F',s.open],['replace','Replace','search','Ctrl+H',s.editable],['goto','Go to line','goto','Ctrl+G',s.open],null,
 ...toolGroups(),
 ...(compares.length?[{group:'compare',label:'Compare',icon:'compare',actions:compares.flatMap(([action,label,enabled,checked])=>[
  ...(['compareFirst','compareOnly','compareExact'].includes(action)?[null]:[]),[action,label,'compare','',enabled,checked]
 ])}]:[])];}
function text(label,description){
 return description?'<span class="nt-editor-menu-text"><span class="nt-editor-menu-label">'+html(label)+'</span><small>'+html(description)+'</small></span>':'<span>'+html(label)+'</span>';
}
function itemMarkup(item){
 if(!item)return '<div role="separator" class="terminal-menu-separator"></div>';
 if(item.group){const id='editorSubmenu-'+item.group;
  return '<button type="button" role="menuitem" tabindex="-1" id="'+id+'-trigger" aria-haspopup="menu" aria-expanded="false" aria-controls="'+id+'" data-editor-submenu="'+item.group+'"'+(item.description?' title="'+html(item.description)+'"':'')+'>'+ico(item.icon)+text(item.label,anchor?item.description:'')+'<span class="nt-editor-submenu-arrow" aria-hidden="true">›</span></button>';}
 const [action,label,icon,shortcut,enabled,checked,description]=item;
 return '<button type="button" role="'+(checked===undefined?'menuitem':'menuitemcheckbox')+'" tabindex="-1" data-editor-menu-action="'+action+'"'+(checked===undefined?'':' aria-checked="'+checked+'"')+(description?' title="'+html(description)+'"':'')+(shortcut?' aria-keyshortcuts="'+html(shortcut.replace(/Ctrl/g,'Control'))+'"':'')+(enabled?'':' disabled')+'>'+(checked===undefined?ico(icon):'<span class="nt-editor-menu-check" aria-hidden="true">'+(checked?'✓':'')+'</span>')+text(label,description)+(shortcut?'<kbd>'+shortcut+'</kbd>':'')+'</button>';
}
function place(panel,x,y){const rect=panel.getBoundingClientRect();
 panel.style.left=Math.max(6,Math.min(x,root.innerWidth-rect.width-6))+'px';
 panel.style.top=Math.max(6,Math.min(y,root.innerHeight-rect.height-6))+'px';}
function placeSubmenu(){
 const parent=primary.getBoundingClientRect(),from=trigger.getBoundingClientRect(),width=submenu.getBoundingClientRect().width;
 const rightRoom=root.innerWidth-parent.right,leftRoom=parent.left;
 const left=rightRoom<width+5&&leftRoom>rightRoom;
 trigger.classList.toggle('opens-left',left);
 place(submenu,left?parent.left-width+1:parent.right-1,from.top-6);
}
function cancelHover(){root.clearTimeout(hoverTimer);hoverTimer=0;}
function hideSubmenu(focus=false){if(!submenu)return;
 cancelHover();
 const returnFocus=focus||submenu.contains(document.activeElement);
 submenu.hidden=true;trigger.setAttribute('aria-expanded','false');trigger.classList.remove('opens-left');
 if(returnFocus)trigger.focus({preventScroll:true});
 submenu=trigger=null;
}
function showSubmenu(button,focus=false){
 cancelHover();
 if(button!==trigger){hideSubmenu();trigger=button;submenu=document.getElementById(button.getAttribute('aria-controls'));
  submenu.hidden=false;submenu.scrollTop=0;trigger.setAttribute('aria-expanded','true');placeSubmenu();}
 if(focus)(submenu.querySelector('button:not(:disabled)')||submenu).focus();
}
function close(focus=true){if(!menu)return;cancelHover();menu.remove();menu=primary=submenu=trigger=null;
 const from=anchor;anchor=null;
 if(from){from.setAttribute('aria-expanded','false');if(focus)from.focus({preventScroll:true});return;}
 if(focus)api()?.focus();}
function build(actions,label,x,y){
 menu=document.createElement('div');menu.id='editorContextMenu';menu.className='nt-editor-context-menu'+(anchor?' nt-editor-tools-menu':'');
 menu.innerHTML='<div class="terminal-context-menu nt-editor-menu-root" role="menu" aria-label="'+html(label)+'">'+actions.map(itemMarkup).join('')+'</div>'+
  actions.filter(item=>item?.group).map(({group,actions})=>'<div class="terminal-context-menu nt-editor-submenu" role="menu" tabindex="-1" id="editorSubmenu-'+group+'" data-editor-submenu-panel="'+group+'" aria-labelledby="editorSubmenu-'+group+'-trigger" hidden>'+actions.map(itemMarkup).join('')+'</div>').join('');
 primary=menu.firstElementChild;
 primary.addEventListener('pointerover',event=>{const button=event.target.closest('button');if(!button)return;
  cancelHover();if(button===trigger)return;
  const update=()=>{if(button.hasAttribute('data-editor-submenu'))showSubmenu(button);else hideSubmenu();};
  // Give the pointer time to cross into a flyout whose top had to move upward.
  if(submenu)hoverTimer=root.setTimeout(update,160);else update();});
 menu.addEventListener('pointerover',event=>{if(submenu?.contains(event.target))cancelHover();});
 primary.addEventListener('scroll',()=>{if(!submenu)return;const from=trigger.getBoundingClientRect(),bounds=primary.getBoundingClientRect();
  if(from.bottom<=bounds.top||from.top>=bounds.bottom)hideSubmenu();else placeSubmenu();});
 document.body.append(menu);place(menu,x,y);
 primary.querySelector('button:not(:disabled)')?.focus({preventScroll:true});return true;}
function open(x,y){const t=api();if(!t)return false;close(false);const s=t.state();if(!s.open)return false;
 return build(items(s),'Text Editor actions',x,y);}
// The toolbar's Tools button: the tool groups, under the button. A second
// click on the button closes the menu.
function openTools(button){
 const t=api();if(!t||!button)return false;
 if(suppressAnchor===button){suppressAnchor=null;return false;}
 close(false);
 const groups=toolGroups();if(!groups.length)return false;
 anchor=button;button.setAttribute('aria-expanded','true');
 const r=button.getBoundingClientRect();
 return build(groups,'Text tools',r.left,r.bottom+2);
}
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
 // Tools that ask for a text (Split, Join) keep the focus in their dialog.
 if(sameDocument()&&!['find','replace','goto','sendTerminal','compareOptions','compareOnly','split','join'].includes(action))t.focus();}
document.addEventListener('contextmenu',event=>{const t=api();if(!t?.target(event.target))return;event.preventDefault();event.stopImmediatePropagation();let x=event.clientX,y=event.clientY;if(!Number.isFinite(x)||!Number.isFinite(y)||x<=0&&y<=0){const r=event.target.getBoundingClientRect();x=r.left+24;y=r.top+24;}t.focusTarget?.(event.target);open(x,y);},true);
document.addEventListener('keydown',event=>{
 if(menu){const current=document.activeElement,inSubmenu=submenu?.contains(current);
  cancelHover();
  if(event.key==='Escape'||event.key==='Tab'){event.preventDefault();event.stopImmediatePropagation();
   if(event.key==='Escape'&&submenu)hideSubmenu(true);else close(true);return;}
  if(event.key==='ArrowLeft'&&inSubmenu){event.preventDefault();event.stopImmediatePropagation();hideSubmenu(true);return;}
  if(event.key==='ArrowRight'&&current?.hasAttribute('data-editor-submenu')){event.preventDefault();event.stopImmediatePropagation();showSubmenu(current,true);return;}
  if(['ArrowDown','ArrowUp','Home','End'].includes(event.key)){event.preventDefault();event.stopImmediatePropagation();
   const list=[...(inSubmenu?submenu:primary).querySelectorAll('button:not(:disabled)')],at=list.indexOf(current);
   if(!list.length)return;
   const step=event.key==='ArrowUp'?-1:1;
   let next=(at+step+list.length)%list.length;
   if(event.key==='Home')next=0;
   if(event.key==='End'||at<0&&step<0)next=list.length-1;
   if(!inSubmenu&&list[next]!==trigger)hideSubmenu();
   list[next].focus();return;}
  return;}
 const t=api();if((event.key==='ContextMenu'||event.shiftKey&&event.key==='F10')&&t?.target(event.target)){event.preventDefault();event.stopImmediatePropagation();const r=event.target.getBoundingClientRect();t.focusTarget?.(event.target);open(r.left+24,r.top+24);}
 // The Tools button opens its menu with the arrow keys too.
 if(event.key==='ArrowDown'&&event.target?.closest?.('[data-ed="tools"]')){event.preventDefault();event.stopImmediatePropagation();openTools(event.target.closest('[data-ed="tools"]'));}
},true);
document.addEventListener('click',event=>{const button=event.target.closest?.('[data-editor-menu-action],[data-editor-submenu]');if(!button||!menu?.contains(button))return;
 event.preventDefault();event.stopImmediatePropagation();if(button.disabled)return;
 if(button.hasAttribute('data-editor-submenu'))showSubmenu(button,true);else void run(button.dataset.editorMenuAction);},true);
document.addEventListener('pointerdown',event=>{if(!menu||menu.contains(event.target))return;
 // A press on the Tools button while its menu is open closes it; the click
 // that follows must not open it again.
 if(anchor&&anchor.contains(event.target)){const pressed=anchor;suppressAnchor=pressed;root.setTimeout(()=>{if(suppressAnchor===pressed)suppressAnchor=null;},600);}
 close(false);},true);
root.addEventListener('resize',()=>close(false));root.addEventListener('blur',()=>close(false));
document.addEventListener('scroll',event=>{if(menu&&!menu.contains(event.target))close(false);},true);
root.NetworkToolkitEditorContextMenu=Object.freeze({open,openTools,close,isOpen:()=>!!menu});
})(window);
