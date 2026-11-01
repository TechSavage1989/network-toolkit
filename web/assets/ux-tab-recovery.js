/* Recover individual closed tabs in memory only. Never resume a live connection. */
(function(root){
'use strict';
const doc=root.document;
if(!doc)return;
const history=[],limit=8;
function notice(message,success=false){
 const host=doc.getElementById('toastStack');if(!host)return;
 const item=doc.createElement('div');item.className='toast'+(success?' success':'');
 item.textContent=message;host.append(item);
 root.setTimeout?.(()=>item.remove(),4800);
}
function remember(kind,label,restore){
 if(!['editor','terminal'].includes(kind)||typeof restore!=='function')return false;
 history.unshift({kind,label:String(label||'tab').slice(0,100),restore});
 if(history.length>limit)history.length=limit;
 return true;
}
function canReopen(){return history.length>0;}
function reopen(){
 const item=history[0];
 if(!item){notice('No recently closed tabs are available in this window.');return false;}
 try{
  if(item.restore()!==true){notice('This tab cannot be restored right now. Its entry is kept for another try.');return false;}
  history.shift();notice('Restored '+item.label+'.',true);return true;
 }catch(error){notice('Could not restore this tab. Close a tab or review the current workspace first.');return false;}
}
function terminalFocus(event){
 const target=event.target;
 return root.NetworkToolkitCommands?.inTerminalInput?.(event)===true||
  !!target?.closest?.('.xterm,.xterm-screen,.xterm-helper-textarea,[data-terminal-input-surface],.terminal-pane .xterm');
}
function blocked(){
 return !!doc.querySelector?.('#modalRoot .modal,.nt-editor-dialog,[data-ed-dialog],#uxCommandOverlay');
}
doc.addEventListener('keydown',event=>{
 if(event.defaultPrevented||event.isComposing||terminalFocus(event)||blocked())return;
 if((event.ctrlKey||event.metaKey)&&event.shiftKey&&!event.altKey&&event.key?.toLowerCase()==='t'){
  event.preventDefault();event.stopImmediatePropagation?.();reopen();
 }
},true);
doc.addEventListener('click',event=>{
 const button=event.target.closest?.('[data-ux-reopen-tab]');
 if(!button)return;
 event.preventDefault();
 if(!blocked())reopen();
},true);
root.NetworkToolkitTabRecovery=Object.freeze({remember,reopen,canReopen,historyCount:()=>history.length});
})(window);
