/* Peer send: text between the standalone Text Editor and the standalone
 * Terminal (owner request). In the installed edition these are two separate
 * app windows of the same executable. Each one's existing command, "Send to
 * terminal" in the editor and "To editor" in the Terminal, reaches the other
 * app when both use the same data folder.
 *
 * Who receives (owner request): this app's engine picks, without asking the
 * user to open anything: the other standalone app if it is running, otherwise
 * the main Toolkit window of the same data folder (its Terminal page or its
 * Text Editor), otherwise it starts the other standalone app and delivers the
 * text when that app is ready. The reply names the receiver.
 *
 * Sending: this page asks its own engine (/api/peer-send/). The engine finds
 * the receiver through that app's protected instance record and calls it on
 * this computer only. This page never sees another app's token.
 * Receiving: this page asks its engine once a second (a GET) whether a request
 * is waiting, and tells it what can take text (connected sessions, or open
 * document names) when asked and whenever that changes. It collects at most
 * one request at a time and runs the same code as a send inside one window:
 *   Terminal: NetworkToolkitEditorSendTerminal.deliver → the console's paste
 *     path (multi-line review, paste warnings, 1 MiB limit, bracketed paste).
 *   Text Editor: NetworkToolkitTerminalSendEditor.deliver → a new document, an
 *     insert, or the "choose the position" bar; an ordinary undoable edit.
 * Only text travels. Never credentials, host keys, file paths or contents the
 * user did not choose to send (the hand-over of documents below carries the
 * local paths and unsaved text of the tabs the user popped out).
 * The main Toolkit window only receives (startMain(), when its engine says it
 * can be found); its own Send to commands stay inside the window. Detached
 * child windows never use peer send; between the windows of one engine
 * window-send.js uses this module's summaries and receive code.
 * Documents: the standalone Text Editor also receives tabs that the main
 * window hands over when its editor is popped out (kind "documents"). They
 * open through editor-handover.js behind the same blocked()/guard checks. */
(function(root){
'use strict';
const document=root.document;if(!document)return;
const NAMES={terminal:'Network Toolkit Terminal',editor:'Network Toolkit Text Editor',main:'Network Toolkit'};
const ID=/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/,TYPE=/^[a-z0-9-]{0,24}$/;
let mode='',started=false,polling=false,receiving=false;
const g=name=>{try{return root[name];}catch{return undefined;}};
function toastMessage(message,kind){const t=g('toast');if(typeof t==='function')t(message,kind);}
function call(path,method='GET',body){const api=g('api');if(typeof api!=='function')return Promise.reject(new Error('The app is not ready.'));return api('/peer-send/'+path,method,body);}
function live(){try{return typeof online!=='undefined'&&online===true&&!(typeof shutDown!=='undefined'&&shutDown)&&!(typeof OFFLINE!=='undefined'&&OFFLINE);}catch{return false;}}
// The main window receives only: it is never a sender.
function sender(){return mode==='main'?'':mode;}
function peerKind(){return mode==='editor'?'terminal':mode==='terminal'?'editor':'';}
function peerName(){return NAMES[peerKind()]||'';}
function sleep(ms){return new Promise(resolve=>setTimeout(resolve,ms));}
function clip(value,limit=160){return String(value??'').replace(/[\u0000-\u001f\u007f-\u009f]/g,' ').trim().slice(0,limit);}
// What another app may list as a destination: names as the tabs show them.
function sessionSummary(){
 const send=root.NetworkToolkitEditorSendTerminal;let active='';try{active=typeof terminalActive!=='undefined'?terminalActive:'';}catch{}
 return (send?.targets?.()||[]).map(entry=>({id:String(entry.info.id),name:clip(send.label(entry)),type:TYPE.test(entry.info.type||'')?entry.info.type||'':'',connected:true,active:entry.info.id===active})).filter(item=>ID.test(item.id)).slice(0,64);
}
function documentSummary(){return (root.NetworkToolkitFileEditor?.listDocuments?.()||[]).filter(doc=>ID.test(doc.uid||'')).slice(0,200).map(doc=>({id:doc.uid,name:clip(doc.name),unsaved:!!doc.dirty,busy:!!doc.blocked,active:!!doc.active}));}
function summary(){return mode==='terminal'?sessionSummary():mode==='editor'?documentSummary():[];}
// A standalone app reports its own kind; the main window its sessions and its documents.
function reportBody(){return mode==='main'?{targets:sessionSummary(),documents:documentSummary()}:{targets:summary()};}
async function targets(){
 if(!sender())return {running:false,ready:false,targets:[]};
 try{const reply=await call('targets');
  const out={running:reply.running===true,ready:reply.ready===true,targets:Array.isArray(reply.targets)?reply.targets.filter(item=>item&&ID.test(item.id||'')).map(item=>({id:item.id,name:clip(item.name)||'Untitled',type:typeof item.type==='string'?item.type:'',connected:item.connected===true,unsaved:item.unsaved===true,busy:item.busy===true,active:item.active===true})):[]};
  // Which app answers (the other standalone app or the main window), and any app that was passed over.
  if(typeof reply.name==='string'&&clip(reply.name))out.name=clip(reply.name);
  if(reply.receiver==='standalone'||reply.receiver==='main')out.receiver=reply.receiver;
  if(typeof reply.note==='string'&&clip(reply.note,600))out.note=clip(reply.note,600);
  return out;}
 catch(error){return {running:false,ready:false,targets:[],error:error.message};}
}
// Raises the other standalone app, or starts it the way its Start menu shortcut
// does, and waits until it is ready. options.failure is added to the reason
// when it does not come up.
async function open(options={}){
 const name=peerName();if(!name)return false;
 try{const reply=await call('open','POST');if(!options.quiet)toastMessage(name+' is open.','success');return reply;}
 catch(error){toastMessage((error.message||name+' could not be opened.')+(options.failure||''),'error');return false;}
}
function doneMessage(outcome,target,name){
 const where=target||(mode==='editor'?'the terminal session':'the document');
 if(outcome==='review')return name+' is showing the text for review before it goes to '+where+'.';
 if(outcome==='pick')return name+' is asking where the text goes in '+where+'.';
 return 'Sent to '+where+' in '+name+'.';
}
// payload: {session,text} from the editor, {target,where,line,text,label} from
// the Terminal. Resolves when the receiver has taken the text or refused it.
// When nothing was running the engine answers "starting": it is opening the
// other app and delivers this same text itself; the user does not send again.
async function send(payload,options={}){
 let name=peerName();if(!name)return {ok:false,error:'Sending between apps is not available in this window.'};
 let queued;
 try{queued=await call('send','POST',payload);}
 catch(error){const message=error.message||name+' did not accept the text.';toastMessage(message,'error');return {ok:false,error:message};}
 name=clip(queued.name)||name;
 if(typeof queued.note==='string'&&queued.note)toastMessage(clip(queued.note,600));
 if(queued.state==='starting')toastMessage('Opening '+name+'… The text is sent when it is ready.');
 let began=Date.now(),noted=queued.state==='starting';
 for(;;){
  await sleep(options.interval||300);
  let reply;try{reply=await call('result?requestId='+encodeURIComponent(queued.requestId));}catch(error){reply={state:'lost',message:error.message};}
  // Still opening the other app: the engine bounds that wait itself.
  if(reply.state==='starting'){began=Date.now();continue;}
  if(reply.state==='done'){const message=doneMessage(reply.outcome,options.target,name);toastMessage(message,'success');return {ok:true,outcome:reply.outcome,message};}
  if(reply.state!=='queued'&&reply.state!=='taken'){const message=reply.local===true&&reply.message?clip(reply.message,600):name+': '+(reply.message||'The text was not accepted.');toastMessage(message,'error');return {ok:false,error:message};}
  if(!noted&&Date.now()-began>2500){noted=true;toastMessage('Waiting for '+name+' to take the text…');}
  if(Date.now()-began>75000){const message=name+' did not confirm the text. Check that window.';toastMessage(message,'error');return {ok:false,error:message};}
 }
}
// ---- Receiving ----
function ownName(){return NAMES[mode]||'';}
// Anything the user is in the middle of: an app dialog, a native <dialog> (the
// colour picker), the editor's Move / merge box, command search.
function dialogOpen(){return !!document.querySelector('#modalRoot .modal,#modalRoot [role="dialog"],.nt-editor-dialog,[data-ed-dialog],#uxCommandOverlay,dialog[open],[data-editor-window-controls]:not([hidden])');}
// The main window clears a typed secret (the Wi-Fi password) when it changes page.
function entryPending(main=mode==='main'){if(!main)return false;try{return [...document.querySelectorAll('[data-transient-secret]')].some(input=>!!input.value);}catch{return false;}}
// Why arriving text cannot be shown right now, or ''. It is asked before the
// window changes page and again immediately before the text is pasted or
// inserted (deliver() can wait a few seconds for a session to be ready).
// window-send.js asks the same question for a window of this engine: name is
// that window's name and main says whether it is the main window.
function blocked(name=ownName(),main=mode==='main'){
 if(dialogOpen())return 'A dialog is open in '+name+'. Finish or close it there, then send again.';
 if(entryPending(main))return 'A password is being entered in '+name+'. Finish that there, then send again.';
 return '';
}
const refuse=error=>({ok:false,error});
// guard: the blocked() question for the receiving window (window-send.js
// passes its own; peer send uses this app's).
async function receiveTerminal(request,guard=blocked,name=ownName()){
 const send=root.NetworkToolkitEditorSendTerminal;
 if(!send?.deliver||typeof request.text!=='string'||!request.text)return refuse(name+' cannot take the text right now.');
 // Never replace a connection form, a host key or sign-in prompt, or a paste review.
 // Checked before anything changes here (deliver() changes the page), and again by deliver().
 const busy=guard();if(busy)return refuse(busy);
 const entry=send.targets().find(item=>item.info.id===request.session);
 if(!entry)return refuse('That terminal session is no longer connected. Nothing was sent.');
 return send.deliver(entry,request.text,{report:true,guard});
}
async function receiveEditor(request,guard=blocked,name=ownName()){
 const send=root.NetworkToolkitTerminalSendEditor,editor=root.NetworkToolkitFileEditor;
 if(!send?.deliver||!editor?.listDocuments||typeof request.text!=='string'||!request.text)return refuse(name+' cannot take the text right now.');
 const busy=guard();if(busy)return refuse(busy);
 let docId='';
 if(request.target!=='new'){const doc=editor.listDocuments().find(item=>item.uid===request.target);if(!doc)return refuse('That document is no longer open. Nothing was added.');if(doc.blocked)return refuse(doc.blocked);docId=doc.id;}
 return send.deliver({docId,where:request.where==='line'?{line:Number(request.line)}:request.where||'end',text:request.text,label:clip(request.label)||'Terminal output',report:true,guard});
}
// Tabs from the main window (editor-handover.js), standalone Text Editor only.
async function receiveDocuments(request){
 const handover=root.NetworkToolkitEditorHandover;
 if(!handover?.receive||!Array.isArray(request.documents))return refuse(ownName()+' cannot take tabs right now.');
 const busy=blocked();if(busy)return refuse(busy);
 return handover.receive(request,{guard:blocked});
}
// kind: what the text is for. A standalone app receives its own kind only; the
// main window is told by its engine ("terminal" or "editor").
async function receive(request,kind){
 if(receiving||!request||typeof request.requestId!=='string')return;receiving=true;
 const own=mode==='main'?(kind==='terminal'||kind==='editor'?kind:''):mode;
 let result;
 try{result=kind==='documents'?(mode==='editor'?await receiveDocuments(request):refuse(ownName()+' cannot take tabs.')):own==='terminal'?await receiveTerminal(request,()=>blocked()):own==='editor'?await receiveEditor(request,()=>blocked()):refuse(ownName()+' cannot take the text right now.');}catch(error){result=refuse(error?.message||'The text could not be delivered.');}
 try{await call('ack','POST',result?.ok?{requestId:request.requestId,ok:true,outcome:result.outcome,...(Array.isArray(result.items)?{items:result.items}:{})}:{requestId:request.requestId,ok:false,error:String(result?.error||'The text was not accepted.').slice(0,280)});}catch{}
 finally{receiving=false;}
}
// The summary goes to this app's engine when the engine asks for it, on this
// page's first poll, and afterwards only when it changes.
let reported=null;
async function report(force){const body=reportBody(),json=JSON.stringify(body);if(!force&&json===reported)return;await call('report','POST',body);reported=json;}
async function poll(){
 if(polling||receiving||!mode||!live())return;polling=true;
 try{const reply=await call('inbox');
  // An engine with peer send always answers with a "report" flag.
  if(typeof reply?.report==='boolean')await report(reply.report||reported===null);
  polling=false;if(reply?.request)await receive(reply.request,reply.kind);}
 catch{}finally{polling=false;}
}
function begin(app){mode=app;if(started)return true;started=true;setInterval(()=>void poll(),1000);return true;}
function start(app){if(app!=='editor'&&app!=='terminal')return false;return begin(app);}
// The main Toolkit window: receive only, and only when its engine reports that
// a standalone app can find it (status.peerReceive).
function startMain(){if(mode&&mode!=='main')return false;return begin('main');}
// window-send.js (windows of one engine) reports and receives with the same code.
root.NetworkToolkitPeerSend=Object.freeze({start,startMain,sender,peerKind,peerName,targets,open,send,poll,summary,dialogOpen,blocked,sessionSummary,documentSummary,receiveTerminal,receiveEditor});
})(window);
