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
 * before Send, the dialog asks the user to reopen it.
 *
 * Standalone Text Editor: the same dialog lists the sessions of the receiver
 * its engine picked and sends through peer-send.js: the standalone Terminal
 * app (Network Toolkit Terminal) if it is running, otherwise the main Toolkit
 * window. That app's page then calls deliver() below, so the text takes
 * exactly the same paste path there. If neither is running, the Terminal app
 * is opened automatically (no question). A session still has to be connected
 * by the user: nothing here connects one, as inside one window. */
(function(root){
'use strict';
const document=root.document;if(!document)return;
const MAX_BYTES=1024*1024;
let pending=null,sequence=0,lastTarget='';
const g=name=>{try{return root[name]??(typeof globalThis!=='undefined'?globalThis[name]:undefined);}catch{return undefined;}};
function sessions(){try{return typeof terminalSessions!=='undefined'?terminalSessions:null;}catch{return null;}}
// The standalone Text Editor sends to the standalone Terminal app instead.
function peer(){const p=root.NetworkToolkitPeerSend;return p?.sender?.()==='editor'?p:null;}
function windowAllows(){try{if(typeof EDITOR_STANDALONE!=='undefined'&&EDITOR_STANDALONE)return peer()?'':'Open the editor in Network Toolkit to send text to a terminal.';if(typeof EDITOR_WINDOW!=='undefined'&&EDITOR_WINDOW)return 'Return this editor to the main window to send text to a terminal.';if(typeof DETACHED_TERMINAL!=='undefined'&&DETACHED_TERMINAL)return 'Open the main window to send text to a terminal.';}catch{}return sessions()?'':'The Terminal is not available in this window.';}
function shown(entry){try{return typeof terminalEntryShown!=='function'||terminalEntryShown(entry);}catch{return false;}}
function owned(entry){try{return typeof terminalOwnerCanMutate!=='function'||terminalOwnerCanMutate(entry);}catch{return false;}}
// Connected sessions displayed in this window, in tab order.
function targets(){const map=sessions();if(!map)return [];const order=root.NetworkToolkitTerminalTabDrag?.ids?.()||[];const list=[...map.values()].filter(e=>e?.info&&e.info.state==='connected'&&shown(e)&&owned(e)&&!e.transferBusy);
 return list.sort((a,b)=>{const x=order.indexOf(a.info.id),y=order.indexOf(b.info.id);return (x<0?1e9:x)-(y<0?1e9:y);});}
function label(entry){const i=entry.info;return i.label||i.host||i.serialPort||'Terminal';}
function kindOf(t){return t==='ssh'?'SSH':t==='telnet'?'Telnet':t==='serial'?'Serial':t==='powershell'?'PowerShell':t==='cmd'?'Command Prompt':'';}
function kind(entry){return kindOf(entry.info.type);}
function bytes(text){return new TextEncoder().encode(text).length;}
function lines(text){return text?(text.match(/\r\n|\r|\n/g)||[]).length+(/[\r\n]$/.test(text)?0:1):0;}
function describe(text){const n=lines(text),b=bytes(text);return n.toLocaleString()+(n===1?' line':' lines')+' · '+b.toLocaleString()+' bytes';}
function escapeHTML(value){return String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]);}
function toastMessage(message,kind){const t=g('toast');if(typeof t==='function')t(message,kind);}
// options: the <option> list; help: the line under the session picker.
function formHTML(id,content,selection,options,help){
 const useSelection=!!selection;
 return '<form id="editorSendTerminalForm" class="editor-send-terminal" data-send-id="'+id+'">'+
  '<fieldset class="editor-send-what"><legend>Send</legend>'+
  '<label class="toggle-row"><input type="radio" name="editorSendWhat" value="selection" '+(useSelection?'checked':'disabled')+'><span>Selection'+(useSelection?' <span class="help-line">('+describe(selection)+')</span>':' <span class="help-line">(nothing selected)</span>')+'</span></label>'+
  '<label class="toggle-row"><input type="radio" name="editorSendWhat" value="document" '+(useSelection?'':'checked')+(content?'':' disabled')+'><span>Whole document <span class="help-line">('+(content?describe(content):'empty')+')</span></span></label></fieldset>'+
  '<div class="field"><label for="editorSendTarget">Terminal session</label><select id="editorSendTarget">'+options+'</select></div>'+
  '<p class="help-line">'+help+'</p>'+
  '<p class="help-line editor-send-error" id="editorSendError" role="alert"></p></form>';
}
const FORM_FOOTER='<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button class="btn primary" type="submit" form="editorSendTerminalForm">Send to terminal</button>';
/* snapshot: {name, content, selection, stamp(): value compared at Send, hideFloating(): optional} */
function open(snapshot){
 const blocked=windowAllows();if(blocked){toastMessage(blocked);return false;}
 const openModal=g('openModal');if(typeof openModal!=='function'||!snapshot)return false;
 const other=peer();if(other){void openPeer(other,snapshot).catch(e=>toastMessage(e.message,'error'));return true;}
 const list=targets(),content=String(snapshot.content||''),selection=String(snapshot.selection||'');
 if(!list.length){openModal('Send to terminal','<p>No terminal session is connected in this window.</p><p class="help-line">Connect one in Terminal, then choose Send to terminal again.</p>','<button class="btn primary" data-action="close-modal">OK</button>');return false;}
 const id=++sequence,active=(()=>{try{return typeof terminalActive!=='undefined'?terminalActive:'';}catch{return '';}})();
 const chosen=list.find(e=>e.info.id===lastTarget)||list.find(e=>e.info.id===active)||list[0];
 pending={id,snapshot,stamp:snapshot.stamp?.(),content,selection};
 openModal('Send to terminal',formHTML(id,content,selection,list.map(e=>'<option value="'+escapeHTML(e.info.id)+'"'+(e===chosen?' selected':'')+'>'+escapeHTML(label(e)+(kind(e)?' · '+kind(e):''))+'</option>').join(''),
  'The Terminal opens on this session and the text is entered as a paste, using your Terminal paste settings. Text with line breaks is shown for review first when Review multi-line paste is on. Line breaks can run commands.'),FORM_FOOTER,true);
 return true;
}
// Standalone Text Editor: the sessions are those of the receiver this app's
// engine picked: the standalone Terminal app, or the main Toolkit window.
async function openPeer(other,snapshot){
 const openModal=g('openModal'),id=++sequence,stamp=snapshot.stamp?.(),content=String(snapshot.content||''),selection=String(snapshot.selection||'');
 pending=null;
 let info=await other.targets();
 if(id!==sequence)return false;
 if(!info.running){
  // Nothing is running: open the Terminal app, with no question. The document stays as it is.
  toastMessage('Opening '+other.peerName()+'…');
  const opened=await other.open({quiet:true,failure:' Nothing was sent; your document is unchanged.'});
  if(!opened||id!==sequence)return false;
  // That wait can take many seconds. Whatever dialog the user opened meanwhile is never replaced.
  if(other.dialogOpen?.()){toastMessage(other.peerName()+' is open. A dialog is open here, so Send to terminal was not shown and nothing was sent. Close the dialog, then choose Send to terminal again.');return false;}
  info=await other.targets();
  if(id!==sequence)return false;
  if(other.dialogOpen?.()){toastMessage(other.peerName()+' is open. A dialog is open here, so Send to terminal was not shown and nothing was sent. Close the dialog, then choose Send to terminal again.');return false;}
 }
 const name=info.name||other.peerName();
 if(info.note)toastMessage(info.note);
 const list=info.targets.filter(item=>item.connected);
 if(!info.running||!list.length){
  // As inside one window: a session has to be connected first; nothing connects one for the user.
  const what=!info.running?name+' is not open.':info.ready?'No terminal session is connected in '+name+'.':name+' is not ready yet.';
  const next=info.running&&info.ready?'Connect one there, then choose Send to terminal again.':'Choose Send to terminal again in a moment.';
  // The Terminal app can be brought to the front from here; the main window has no such request.
  const show=info.running&&info.receiver==='standalone'?'<button type="button" class="btn ghost" data-editor-send-open-peer>Show Terminal</button>':'';
  openModal('Send to terminal','<div data-editor-send-peer-missing><p>'+escapeHTML(what)+'</p><p class="help-line">'+escapeHTML(next)+'</p></div>',show+'<button type="button" class="btn primary" data-action="close-modal">OK</button>');
  return false;
 }
 const chosen=list.find(item=>item.id===lastTarget)||list.find(item=>item.active)||list[0];
 pending={id,snapshot,stamp,content,selection,peer:other,names:new Map(list.map(item=>[item.id,item.name]))};
 openModal('Send to terminal',formHTML(id,content,selection,'<optgroup label="'+escapeHTML(name)+'">'+list.map(item=>'<option value="'+escapeHTML(item.id)+'"'+(item===chosen?' selected':'')+'>'+escapeHTML(item.name+(kindOf(item.type)?' · '+kindOf(item.type):''))+'</option>').join('')+'</optgroup>',
  escapeHTML(name)+' opens on this session and the text is entered as a paste, using its paste settings. Text with line breaks is shown there for review first when Review multi-line paste is on. Line breaks can run commands.'),FORM_FOOTER,true);
 return true;
}
function wait(bounded){return new Promise(resolve=>{const raf=root.requestAnimationFrame||(fn=>setTimeout(fn,16));let done=false;const finish=()=>{if(!done){done=true;resolve();}};raf(()=>raf(finish));
 // A window in the background may not paint; text from the other app must not wait for it.
 if(bounded)setTimeout(finish,150);});}
/* Select the session and paste. A send inside one window shows its own
 * messages and returns what pasteText returned. With options.report (text
 * from the standalone Text Editor) nothing is shown here beyond what the
 * paste path itself shows, and the result is returned for the other app:
 * {ok:true,outcome:'sent'|'review'} or {ok:false,error}. options.guard (text
 * from another app) returns a reason when a dialog is open in this window; it
 * is asked before the page changes and again immediately before the paste. */
async function deliver(entry,text,options={}){
 const report=options.report===true,fail=message=>{if(report)return {ok:false,error:message};toastMessage(message,'error');return false;};
 const guard=()=>{try{return report&&typeof options.guard==='function'?String(options.guard()||''):'';}catch{return 'The terminal cannot take text right now.';}};
 const navigate=g('navigate'),select=g('selectTerminal'),interaction=root.NetworkToolkitTerminalInteraction;
 if(typeof navigate!=='function'||typeof select!=='function'||!interaction?.pasteText)return report?{ok:false,error:'The terminal cannot take text right now.'}:false;
 {const busy=guard();if(busy)return fail(busy);}
 navigate('terminal');
 for(let attempt=0;attempt<30;attempt++){
  await wait(report);
  if(sessions()?.get(entry.info.id)!==entry||entry.info.state!=='connected')return fail('That terminal session is no longer connected. Nothing was sent.');
  let current=null;try{current=typeof currentTerminal==='function'?currentTerminal():null;}catch{}
  if(current!==entry){select(entry.info.id,true);continue;}
  let ready=true;try{ready=typeof terminalEntryAcceptsInput!=='function'||terminalEntryAcceptsInput(entry);}catch{ready=false;}
  if(!ready)continue;
  {const busy=guard();if(busy)return fail(busy);}
  const sent=interaction.pasteText(entry,text);
  if(!report)return sent;
  if(sent)return {ok:true,outcome:'sent'};
  // pasteText returns false both when it opened the review and when it refused.
  return document.querySelector('#terminalPasteForm')?{ok:true,outcome:'review'}:{ok:false,error:'The terminal did not accept the text. Check that window for the reason.'};
 }
 return fail('The terminal session was not ready for input. Nothing was sent.');
}
async function submit(form){
 const p=pending,error=form.querySelector('#editorSendError');if(!p||String(p.id)!==form.dataset.sendId)return;
 const what=form.querySelector('[name="editorSendWhat"]:checked')?.value,text=what==='selection'?p.selection:p.content;
 const fail=message=>{if(error)error.textContent=message;};
 if(!text){fail('There is no text to send.');return;}
 if(bytes(text)>MAX_BYTES){fail('Send no more than 1 MiB at a time. Select a smaller part of the document.');return;}
 if(p.snapshot.stamp&&p.snapshot.stamp()!==p.stamp){fail('The document changed after this dialog opened. Cancel and choose Send to terminal again.');return;}
 const closeModal=g('closeModal');
 if(p.peer){
  const id=form.querySelector('#editorSendTarget')?.value||'';if(!p.names.has(id)){fail('Choose a terminal session.');return;}
  pending=null;lastTarget=id;if(typeof closeModal==='function')closeModal();
  // The document stays open here; the other app shows the paste or its review.
  await p.peer.send({session:id,text},{target:p.names.get(id)});return;
 }
 const entry=sessions()?.get(form.querySelector('#editorSendTarget')?.value);
 if(!entry||entry.info.state!=='connected'||!shown(entry)){fail('That terminal session is no longer connected. Choose another session.');return;}
 pending=null;lastTarget=entry.info.id;
 if(typeof closeModal==='function')closeModal();
 try{p.snapshot.hideFloating?.();}catch{}
 await deliver(entry,text);
}
document.addEventListener('submit',event=>{const form=event.target;if(form?.id!=='editorSendTerminalForm')return;event.preventDefault();event.stopImmediatePropagation();void submit(form).catch(e=>toastMessage(e.message,'error'));},true);
document.addEventListener('click',event=>{const button=event.target.closest?.('[data-editor-send-open-peer]');if(!button||button.disabled)return;const other=peer();if(!other)return;button.disabled=true;const closeModal=g('closeModal');
 void other.open().then(opened=>{if(!opened){button.disabled=false;return;}if(typeof closeModal==='function'&&document.querySelector('[data-editor-send-peer-missing]'))closeModal();},()=>{button.disabled=false;});});
root.NetworkToolkitEditorSendTerminal=Object.freeze({open,targets,label,kind,deliver,available:()=>!windowAllows(),describe});
})(window);
