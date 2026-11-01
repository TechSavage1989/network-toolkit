(function(root){
'use strict';
// Deliberately memory-only. No credentials, console output or document text
// enter this history. Active SSH/Telnet/Serial sessions are never resurrected.
const recent=[],MAX=10;
let restoreDraft=null;
function configure(options={}){if(typeof options.restoreDraft==='function')restoreDraft=options.restoreDraft;}
function recordEditor(path){
 const value=String(path||'').trim();
 if(!value||value.length>4096)return false;
 return add({kind:'editor-local',path:value});
}
function recordDraft(label){
 const value=String(label||'New connection').slice(0,100);
 return add({kind:'terminal-draft',label:value});
}
function add(item){
 if(!['editor-local','terminal-draft'].includes(item.kind))return false;
 if(item.kind==='editor-local'){const prior=recent.findIndex(x=>x.kind===item.kind&&x.path===item.path);if(prior>=0)recent.splice(prior,1);}
 recent.unshift(item);if(recent.length>MAX)recent.length=MAX;return true;
}
function inConsole(event){
 const target=event?.target;
 return !!target?.closest?.('.xterm,.xterm-helper-textarea,.xterm-screen,.terminal-xterm,[data-terminal-input-surface],.terminal-pane .xterm,textarea[data-terminal-console]');
}
function blocked(event){
 return event?.isComposing||inConsole(event)||
 !!root.document.querySelector('#modalRoot .modal, #modalRoot [role="dialog"], .nt-editor-dialog, [data-ed-dialog], #uxCommandOverlay');
}
async function reopen(){
 const item=recent[0];if(!item)return false;
 try{
  if(item.kind==='editor-local'){
   if(!root.NetworkToolkitFileEditor?.openLocalPath||typeof root.navigate!=='function')return false;
   root.navigate('editor');
   await root.NetworkToolkitFileEditor.openLocalPath(item.path);
  }else if(item.kind==='terminal-draft'){
   if(!restoreDraft||!restoreDraft(item.label))return false;
  }else return false;
  recent.shift();return true;
 }catch(error){
  if(typeof root.toast==='function')root.toast('Could not reopen the tab: '+error.message,'error');
  return false;
 }
}
root.document.addEventListener('keydown',event=>{
 if(!event.ctrlKey||!event.shiftKey||event.altKey||event.metaKey||String(event.key).toLowerCase()!=='t'||blocked(event))return;
 event.preventDefault();event.stopImmediatePropagation();void reopen();
});
root.NetworkToolkitClosedTabs=Object.freeze({configure,recordEditor,recordDraft,reopen,count:()=>recent.length});
})(globalThis);
