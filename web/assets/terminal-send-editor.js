/* Send Terminal output to the Text Editor (owner request, v1.20.13).
 * "To editor" in the console bar (and "Send output to Text Editor…" in a tab's
 * menu) opens a dialog:
 * - Send: the chosen output block (Last command or Last batch, the same text
 *   Select output / Copy output use, honouring Include commands), or the text
 *   currently selected in the terminal.
 * - To: a new document, or an open document.
 * - Where (open document): choose the position in the editor (a bar asks the
 *   user to place the cursor or select text to replace, then Insert here), the
 *   last cursor position, the start, the end, or a line number.
 * The text is captured when the dialog opens; nothing reaches the editor until
 * Send. Inserts are ordinary edits: undoable, and the document becomes unsaved.
 * Only the main window's editor is used; a detached Terminal window has none.
 *
 * Standalone Terminal: the same dialog lists the documents of the receiver its
 * engine picked and sends through peer-send.js: the standalone Text Editor app
 * (Network Toolkit Text Editor) if it is running, otherwise the main Toolkit
 * window. That app's page then calls deliver() below, so the text is added
 * there in exactly the same way. If neither is running, Send starts the Text
 * Editor app: this app's engine holds the text until that app is ready and
 * delivers it to a new document, so the user sends once. */
(function(root){
'use strict';
const document=root.document;if(!document)return;
const MAX_CHARS=8*1024*1024,PEER_MAX_BYTES=8*1024*1024;
let pending=null,sequence=0;
const g=name=>{try{return root[name];}catch{return undefined;}};
function editor(){return root.NetworkToolkitFileEditor;}
// The standalone Terminal sends to the standalone Text Editor app instead.
function peer(){try{if(typeof TERMINAL_WINDOW==='undefined'||!TERMINAL_WINDOW)return null;}catch{return null;}const p=root.NetworkToolkitPeerSend;return p?.sender?.()==='terminal'?p:null;}
function unavailable(){try{if(typeof DETACHED_TERMINAL!=='undefined'&&DETACHED_TERMINAL)return 'Use the main window to send output to the Text Editor.';if(typeof TERMINAL_ONLY!=='undefined'&&TERMINAL_ONLY)return peer()?'':'The Text Editor is not available in this window.';}catch{}return editor()?.insertText?'':'The Text Editor is not available.';}
function escapeHTML(value){return String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]);}
function lines(text){return text?(text.match(/\r\n|\r|\n/g)||[]).length+(/[\r\n]$/.test(text)?0:1):0;}
function describe(text){const n=lines(text);return n.toLocaleString()+(n===1?' line':' lines')+' · '+text.length.toLocaleString()+' characters';}
function toastMessage(message,kind){const t=g('toast');if(typeof t==='function')t(message,kind);}
function normalize(text){return String(text||'').replace(/\r\n?/g,'\n');}
function sessionLabel(entry){const i=entry?.info||{};return i.label||i.host||i.serialPort||'Terminal';}
async function open(entry){
 const blocked=unavailable();if(blocked){toastMessage(blocked);return false;}
 const openModal=g('openModal');if(!entry||typeof openModal!=='function')return false;
 const id=++sequence,selection=normalize(entry.xterm?.hasSelection?.()?entry.xterm.getSelection():'');
 let block=null;try{block=await root.NetworkToolkitTerminalExtras?.capture?.(entry);}catch{block=null;}
 if(id!==sequence)return false;
 const output=normalize(block?.text||''),kind=(entry._ntOutputChoice||'last-command')==='last-paste'?'Last batch output':'Last command output';
 if(!output&&!selection){toastMessage('No retained output or selected text to send yet.');return false;}
 // Standalone Terminal: the documents are those of the standalone Text Editor app.
 const other=peer();let remote=null;
 if(other){remote=await other.targets();if(id!==sequence)return false;}
 const docs=other?remote.targets.map(item=>({id:item.id,name:item.name,dirty:item.unsaved,active:item.active,blocked:item.busy?'unavailable':''})):(editor().listDocuments?.()||[]),usable=docs.filter(d=>!d.blocked),activeDoc=usable.find(d=>d.active)||usable[0];
 // The receiver: the Text Editor app, or the main Toolkit window when that app is not running.
 const name=other?remote.name||other.peerName():'';
 pending={id,entry,output,selection,kind,label:sessionLabel(entry),peer:other,names:new Map(docs.map(d=>[d.id,d.name]))};
 const sourceRow=(value,title,text,checked)=>'<label class="toggle-row"><input type="radio" name="terminalSendSource" value="'+value+'" '+(text?(checked?'checked':''):'disabled')+'><span>'+title+' <span class="help-line">('+(text?describe(text):value==='selection'?'nothing selected':'nothing retained yet')+')</span></span></label>';
 openModal('Send to Text Editor',
  '<form id="terminalSendEditorForm" class="terminal-send-editor" data-send-id="'+id+'">'+
  '<fieldset><legend>Send</legend>'+sourceRow('output',kind,output,false)+sourceRow('selection','Selected terminal text',selection,!!selection)+(block?.truncated?'<p class="help-line">Only the retained part of a long output is available.</p>':'')+'</fieldset>'+
  '<fieldset><legend>'+(other?'To '+escapeHTML(name):'To')+'</legend>'+(other&&!remote.running?'<p class="help-line" data-terminal-send-peer-missing>'+escapeHTML(name)+' is not open. It opens when you choose Send to editor.</p>':'')+(other&&remote.note?'<p class="help-line" data-terminal-send-peer-note>'+escapeHTML(remote.note)+'</p>':'')+'<label class="toggle-row"><input type="radio" name="terminalSendTarget" value="new" '+(activeDoc?'':'checked')+'><span>New document</span></label>'+
  '<label class="toggle-row"><input type="radio" name="terminalSendTarget" value="existing" '+(activeDoc?'checked':'disabled')+'><span>Open document'+(docs.length?'':' <span class="help-line">(none open)</span>')+'</span></label>'+
  '<div class="field terminal-send-existing"><label for="terminalSendDocument">Document</label><select id="terminalSendDocument" '+(activeDoc?'':'disabled')+'>'+docs.map((d,i)=>'<option value="'+escapeHTML(d.id)+'"'+(d.blocked?' disabled':'')+(d===activeDoc?' selected':'')+'>'+escapeHTML((i+1)+'. '+d.name+(d.dirty?' (unsaved)':'')+(d.source?' · '+d.source:'')+(d.blocked?' · unavailable':''))+'</option>').join('')+'</select></div>'+
  '<div class="field terminal-send-existing"><label for="terminalSendWhere">Insert at</label><select id="terminalSendWhere" '+(activeDoc?'':'disabled')+'><option value="pick" selected>A position I choose in the editor</option><option value="cursor">The document’s current cursor</option><option value="end">The end of the document</option><option value="start">The start of the document</option><option value="line">The start of line…</option></select></div>'+
  '<div class="field terminal-send-line" hidden><label for="terminalSendLine">Line number</label><input id="terminalSendLine" type="number" min="1" step="1" value="1" inputmode="numeric"></div></fieldset>'+
  '<p class="help-line">'+(other?escapeHTML(name)+' opens with the text added as an ordinary edit: the document is marked unsaved and Undo removes it.':'The Text Editor opens with the text added as an ordinary edit: the document is marked unsaved and Undo removes it.')+'</p>'+
  '<p class="help-line terminal-send-error" id="terminalSendError" role="alert"></p></form>',
  '<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button class="btn primary" type="submit" form="terminalSendEditorForm">Send to editor</button>',true);
 // Output is the default source unless only a selection exists.
 const form=document.querySelector('#terminalSendEditorForm');if(form){const out=form.querySelector('[name="terminalSendSource"][value="output"]'),sel=form.querySelector('[name="terminalSendSource"][value="selection"]');if(output&&out)out.checked=true;else if(sel&&!sel.disabled)sel.checked=true;sync(form);}
 return true;
}
function sync(form){const existing=form.querySelector('[name="terminalSendTarget"]:checked')?.value==='existing';for(const el of form.querySelectorAll('.terminal-send-existing select'))el.disabled=!existing;const line=form.querySelector('.terminal-send-line');if(line)line.hidden=!existing||form.querySelector('#terminalSendWhere')?.value!=='line';}
function wait(bounded){return new Promise(resolve=>{const raf=root.requestAnimationFrame||(fn=>setTimeout(fn,16));let done=false;const finish=()=>{if(!done){done=true;resolve();}};raf(()=>raf(finish));
 // A window in the background may not paint; text from the other app must not wait for it.
 if(bounded)setTimeout(finish,150);});}
/* Add the text to the editor in this window: a new document (no docId), the
 * "choose the position" bar, or an insert at the cursor, start, end or a line.
 * A send inside one window shows its own messages. With request.report (text
 * from the standalone Terminal) the result is returned for the other app:
 * {ok:true,outcome:'inserted'|'pick'} or {ok:false,error}. request.guard (text
 * from another app) returns a reason when a dialog is open in this window; it
 * is asked before the page changes and again immediately before each step
 * that changes the editor. */
async function deliver(request){
 const {docId,where,text,label}=request,report=request.report===true,ed=editor();
 const fail=message=>{if(report)return {ok:false,error:message};toastMessage(message,'error');return false;},done=outcome=>report?{ok:true,outcome}:true;
 const guard=()=>{try{return report&&typeof request.guard==='function'?String(request.guard()||''):'';}catch{return 'The Text Editor cannot take text right now.';}};
 if(!ed?.insertText)return fail('The Text Editor is not available.');
 let busy=guard();if(busy)return fail(busy);
 const navigate=g('navigate');if(typeof navigate==='function')navigate('editor');await wait(report);
 if((busy=guard()))return fail(busy);
 if(!docId){const doc=await ed.newDocument?.();if(!doc?.id)return fail('Could not create a new document.');if((busy=guard()))return fail(busy);const r=await ed.insertText(doc.id,text,'start');return r?.ok?done('inserted'):fail(r?.error||'The text could not be inserted.');}
 if(where==='pick')return ed.pickInsert(docId,text,label)?done('pick'):fail('That document is not available. Nothing was added.');
 const r=await ed.insertText(docId,text,where);return r?.ok?done('inserted'):fail(r?.error||'The text could not be inserted.');
}
async function submit(form){
 const p=pending,error=form.querySelector('#terminalSendError');if(!p||String(p.id)!==form.dataset.sendId)return;const fail=m=>{if(error)error.textContent=m;};
 const source=form.querySelector('[name="terminalSendSource"]:checked')?.value,text=source==='selection'?p.selection:p.output;
 if(!text){fail('There is no text to send.');return;}if(text.length>MAX_CHARS){fail('That is more text than the editor accepts at once. Select a smaller part.');return;}
 const target=form.querySelector('[name="terminalSendTarget"]:checked')?.value,ed=editor();let docId='',where='end';
 if(target==='existing'){docId=form.querySelector('#terminalSendDocument')?.value||'';
  if(p.peer){if(!p.names.has(docId)){fail('That document is no longer open. Choose another.');return;}}
  else{const doc=(ed.listDocuments?.()||[]).find(d=>d.id===docId);if(!doc){fail('That document is no longer open. Choose another.');return;}if(doc.blocked){fail(doc.blocked);return;}}
  where=form.querySelector('#terminalSendWhere')?.value||'pick';if(where==='line'){const n=Number(form.querySelector('#terminalSendLine')?.value);if(!Number.isSafeInteger(n)||n<1){fail('Enter a line number of 1 or more.');return;}where={line:n};}}
 const label=(source==='selection'?'Selected terminal text':p.kind)+' from '+p.label;
 const closeModal=g('closeModal');
 if(p.peer){
  if(text.length*3>PEER_MAX_BYTES&&new TextEncoder().encode(text).length>PEER_MAX_BYTES){fail('Send no more than 8 MiB of text to the Text Editor at a time. Select a smaller part.');return;}
  pending=null;if(typeof closeModal==='function')closeModal();
  // If no receiver is running, this app's engine starts the Text Editor app and delivers this text itself.
  const payload=target==='existing'?{target:docId,where:typeof where==='object'?'line':where,text,label}:{target:'new',text,label};if(typeof where==='object')payload.line=where.line;
  await p.peer.send(payload,{target:target==='existing'?p.names.get(docId):'a new document'});return;
 }
 pending=null;if(typeof closeModal==='function')closeModal();
 await deliver({docId:target==='existing'?docId:'',where,text,label});
}
document.addEventListener('change',event=>{const form=event.target.closest?.('#terminalSendEditorForm');if(form)sync(form);});
document.addEventListener('submit',event=>{const form=event.target;if(form?.id!=='terminalSendEditorForm')return;event.preventDefault();event.stopImmediatePropagation();void submit(form).catch(e=>toastMessage(e.message,'error'));},true);
root.NetworkToolkitTerminalSendEditor=Object.freeze({open,deliver,available:()=>!unavailable()});
})(window);
