/* Send Text Editor text to a Terminal session (owner request, v1.20.13).
 * The editor's "Send to terminal" button opens a small dialog: choose the
 * selection or the whole document, and a connected Terminal session in this
 * window. Sending switches to the Terminal page, selects that session and
 * hands the text to the console's normal paste path
 * (NetworkToolkitTerminalInteraction.pasteText), so every configured input
 * rule applies unchanged: multi-line review when "Review multi-line paste" is
 * on, the 1 MiB limit, bracketed paste, selection replacement, queued-input
 * checks and the destination-changed guard. Nothing is sent from the dialog
 * itself, and editor documents (including unsaved changes) stay open.
 * The dialog snapshots the text when it opens; if the document changes
 * before Send, the dialog asks the user to reopen it. */
(function(root){
'use strict';
const document=root.document;if(!document)return;
const MAX_BYTES=1024*1024;
let pending=null,sequence=0,lastTarget='';
const g=name=>{try{return root[name]??(typeof globalThis!=='undefined'?globalThis[name]:undefined);}catch{return undefined;}};
function sessions(){try{return typeof terminalSessions!=='undefined'?terminalSessions:null;}catch{return null;}}
function windowAllows(){try{if(typeof EDITOR_WINDOW!=='undefined'&&EDITOR_WINDOW)return 'Return this editor to the main window to send text to a terminal.';if(typeof DETACHED_TERMINAL!=='undefined'&&DETACHED_TERMINAL)return 'Open the main window to send text to a terminal.';}catch{}return sessions()?'':'The Terminal is not available in this window.';}
function shown(entry){try{return typeof terminalEntryShown!=='function'||terminalEntryShown(entry);}catch{return false;}}
function owned(entry){try{return typeof terminalOwnerCanMutate!=='function'||terminalOwnerCanMutate(entry);}catch{return false;}}
// Connected sessions displayed in this window, in tab order.
function targets(){const map=sessions();if(!map)return [];const order=root.NetworkToolkitTerminalTabDrag?.ids?.()||[];const list=[...map.values()].filter(e=>e?.info&&e.info.state==='connected'&&shown(e)&&owned(e)&&!e.transferBusy);
 return list.sort((a,b)=>{const x=order.indexOf(a.info.id),y=order.indexOf(b.info.id);return (x<0?1e9:x)-(y<0?1e9:y);});}
function label(entry){const i=entry.info;return i.label||i.host||i.serialPort||'Terminal';}
function kind(entry){const t=entry.info.type;return t==='ssh'?'SSH':t==='telnet'?'Telnet':t==='serial'?'Serial':t==='powershell'?'PowerShell':t==='cmd'?'Command Prompt':'';}
function bytes(text){return new TextEncoder().encode(text).length;}
function lines(text){return text?(text.match(/\r\n|\r|\n/g)||[]).length+(/[\r\n]$/.test(text)?0:1):0;}
function describe(text){const n=lines(text),b=bytes(text);return n.toLocaleString()+(n===1?' line':' lines')+' · '+b.toLocaleString()+' bytes';}
function escapeHTML(value){return String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]);}
function toastMessage(message,kind){const t=g('toast');if(typeof t==='function')t(message,kind);}
/* snapshot: {name, content, selection, stamp(): value compared at Send, hideFloating(): optional} */
function open(snapshot){
 const blocked=windowAllows();if(blocked){toastMessage(blocked);return false;}
 const openModal=g('openModal');if(typeof openModal!=='function'||!snapshot)return false;
 const list=targets(),content=String(snapshot.content||''),selection=String(snapshot.selection||'');
 if(!list.length){openModal('Send to terminal','<p>No terminal session is connected in this window.</p><p class="help-line">Connect one in Terminal, then choose Send to terminal again.</p>','<button class="btn primary" data-action="close-modal">OK</button>');return false;}
 const id=++sequence,active=(()=>{try{return typeof terminalActive!=='undefined'?terminalActive:'';}catch{return '';}})();
 const chosen=list.find(e=>e.info.id===lastTarget)||list.find(e=>e.info.id===active)||list[0];
 const useSelection=!!selection;
 pending={id,snapshot,stamp:snapshot.stamp?.(),content,selection};
 openModal('Send to terminal',
  '<form id="editorSendTerminalForm" class="editor-send-terminal" data-send-id="'+id+'">'+
  '<fieldset class="editor-send-what"><legend>Send</legend>'+
  '<label class="toggle-row"><input type="radio" name="editorSendWhat" value="selection" '+(useSelection?'checked':'disabled')+'><span>Selection'+(useSelection?' <span class="help-line">('+describe(selection)+')</span>':' <span class="help-line">(nothing selected)</span>')+'</span></label>'+
  '<label class="toggle-row"><input type="radio" name="editorSendWhat" value="document" '+(useSelection?'':'checked')+(content?'':' disabled')+'><span>Whole document <span class="help-line">('+(content?describe(content):'empty')+')</span></span></label></fieldset>'+
  '<div class="field"><label for="editorSendTarget">Terminal session</label><select id="editorSendTarget">'+list.map(e=>'<option value="'+escapeHTML(e.info.id)+'"'+(e===chosen?' selected':'')+'>'+escapeHTML(label(e)+(kind(e)?' · '+kind(e):''))+'</option>').join('')+'</select></div>'+
  '<p class="help-line">The Terminal opens on this session and the text is entered as a paste, using your Terminal paste settings. Text with line breaks is shown for review first when Review multi-line paste is on. Line breaks can run commands.</p>'+
  '<p class="help-line editor-send-error" id="editorSendError" role="alert"></p></form>',
  '<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button class="btn primary" type="submit" form="editorSendTerminalForm">Send to terminal</button>',true);
 return true;
}
function wait(){return new Promise(resolve=>{const raf=root.requestAnimationFrame||(fn=>setTimeout(fn,16));raf(()=>raf(resolve));});}
async function deliver(entry,text){
 const navigate=g('navigate'),select=g('selectTerminal'),interaction=root.NetworkToolkitTerminalInteraction;
 if(typeof navigate!=='function'||typeof select!=='function'||!interaction?.pasteText)return false;
 navigate('terminal');
 for(let attempt=0;attempt<30;attempt++){
  await wait();
  if(sessions()?.get(entry.info.id)!==entry||entry.info.state!=='connected'){toastMessage('That terminal session is no longer connected. Nothing was sent.','error');return false;}
  let current=null;try{current=typeof currentTerminal==='function'?currentTerminal():null;}catch{}
  if(current!==entry){select(entry.info.id,true);continue;}
  let ready=true;try{ready=typeof terminalEntryAcceptsInput!=='function'||terminalEntryAcceptsInput(entry);}catch{ready=false;}
  if(!ready)continue;
  return interaction.pasteText(entry,text);
 }
 toastMessage('The terminal session was not ready for input. Nothing was sent.','error');return false;
}
async function submit(form){
 const p=pending,error=form.querySelector('#editorSendError');if(!p||String(p.id)!==form.dataset.sendId)return;
 const what=form.querySelector('[name="editorSendWhat"]:checked')?.value,text=what==='selection'?p.selection:p.content;
 const fail=message=>{if(error)error.textContent=message;};
 if(!text){fail('There is no text to send.');return;}
 if(bytes(text)>MAX_BYTES){fail('Send no more than 1 MiB at a time. Select a smaller part of the document.');return;}
 if(p.snapshot.stamp&&p.snapshot.stamp()!==p.stamp){fail('The document changed after this dialog opened. Cancel and choose Send to terminal again.');return;}
 const entry=sessions()?.get(form.querySelector('#editorSendTarget')?.value);
 if(!entry||entry.info.state!=='connected'||!shown(entry)){fail('That terminal session is no longer connected. Choose another session.');return;}
 pending=null;lastTarget=entry.info.id;
 const closeModal=g('closeModal');if(typeof closeModal==='function')closeModal();
 try{p.snapshot.hideFloating?.();}catch{}
 await deliver(entry,text);
}
document.addEventListener('submit',event=>{const form=event.target;if(form?.id!=='editorSendTerminalForm')return;event.preventDefault();event.stopImmediatePropagation();void submit(form).catch(e=>toastMessage(e.message,'error'));},true);
root.NetworkToolkitEditorSendTerminal=Object.freeze({open,targets,available:()=>!windowAllows(),describe});
})(window);
