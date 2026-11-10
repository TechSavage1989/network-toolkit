/* Peer send: text between the standalone Text Editor and the standalone
 * Terminal (owner request). In the installed edition these are two separate
 * app windows of the same executable. Each one's existing command, "Send to
 * terminal" in the editor and "To editor" in the Terminal, reaches the other
 * app when both use the same data folder.
 *
 * Sending: this page asks its own engine (/api/peer-send/). The engine finds
 * the other app through that app's protected instance record and calls it on
 * this computer only. This page never sees the other app's token.
 * Receiving: this page asks its engine once a second (a GET) whether a request
 * is waiting, and tells it what can take text (connected sessions, or open
 * document names) when asked and whenever that changes. It collects at most
 * one request at a time and runs the same code as a send inside one window:
 *   Terminal: NetworkToolkitEditorSendTerminal.deliver → the console's paste
 *     path (multi-line review, paste warnings, 1 MiB limit, bracketed paste).
 *   Text Editor: NetworkToolkitTerminalSendEditor.deliver → a new document, an
 *     insert, or the "choose the position" bar; an ordinary undoable edit.
 * Only text travels. Never credentials, host keys, file paths or contents the
 * user did not choose to send. The main Toolkit window and detached child
 * windows do not use this module. */
(function(root){
'use strict';
const document=root.document;if(!document)return;
const NAMES={terminal:'Network Toolkit Terminal',editor:'Network Toolkit Text Editor'};
const ID=/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/,TYPE=/^[a-z0-9-]{0,24}$/;
let mode='',started=false,polling=false,receiving=false;
const g=name=>{try{return root[name];}catch{return undefined;}};
function toastMessage(message,kind){const t=g('toast');if(typeof t==='function')t(message,kind);}
function call(path,method='GET',body){const api=g('api');if(typeof api!=='function')return Promise.reject(new Error('The app is not ready.'));return api('/peer-send/'+path,method,body);}
function live(){try{return typeof online!=='undefined'&&online===true&&!(typeof shutDown!=='undefined'&&shutDown)&&!(typeof OFFLINE!=='undefined'&&OFFLINE);}catch{return false;}}
function sender(){return mode;}
function peerKind(){return mode==='editor'?'terminal':mode==='terminal'?'editor':'';}
function peerName(){return NAMES[peerKind()]||'';}
function sleep(ms){return new Promise(resolve=>setTimeout(resolve,ms));}
function clip(value){return String(value??'').replace(/[\u0000-\u001f\u007f-\u009f]/g,' ').trim().slice(0,160);}
// What the other app may list as a destination: names as the tabs show them.
function summary(){
 if(mode==='terminal'){
  const send=root.NetworkToolkitEditorSendTerminal;let active='';try{active=typeof terminalActive!=='undefined'?terminalActive:'';}catch{}
  return (send?.targets?.()||[]).map(entry=>({id:String(entry.info.id),name:clip(send.label(entry)),type:TYPE.test(entry.info.type||'')?entry.info.type||'':'',connected:true,active:entry.info.id===active})).filter(item=>ID.test(item.id)).slice(0,64);
 }
 if(mode==='editor')return (root.NetworkToolkitFileEditor?.listDocuments?.()||[]).filter(doc=>ID.test(doc.uid||'')).slice(0,200).map(doc=>({id:doc.uid,name:clip(doc.name),unsaved:!!doc.dirty,busy:!!doc.blocked,active:!!doc.active}));
 return [];
}
async function targets(){
 if(!mode)return {running:false,ready:false,targets:[]};
 try{const reply=await call('targets');return {running:reply.running===true,ready:reply.ready===true,targets:Array.isArray(reply.targets)?reply.targets.filter(item=>item&&ID.test(item.id||'')).map(item=>({id:item.id,name:clip(item.name)||'Untitled',type:typeof item.type==='string'?item.type:'',connected:item.connected===true,unsaved:item.unsaved===true,busy:item.busy===true,active:item.active===true})):[]};}
 catch(error){return {running:false,ready:false,targets:[],error:error.message};}
}
// Raises the other app, or starts it the way its Start menu shortcut does.
async function open(options={}){
 const name=peerName();if(!name)return false;
 try{const reply=await call('open','POST');if(!options.quiet)toastMessage(name+' is open.','success');return reply;}
 catch(error){toastMessage(error.message||name+' could not be opened.','error');return false;}
}
function doneMessage(outcome,target){
 const name=peerName(),where=target||(mode==='editor'?'the terminal session':'the document');
 if(outcome==='review')return name+' is showing the text for review before it goes to '+where+'.';
 if(outcome==='pick')return name+' is asking where the text goes in '+where+'.';
 return 'Sent to '+where+' in '+name+'.';
}
// payload: {session,text} from the editor, {target,where,line,text,label} from
// the Terminal. Resolves when the other app has taken the text or refused it.
async function send(payload,options={}){
 const name=peerName();if(!name)return {ok:false,error:'Sending between apps is not available in this window.'};
 let queued;
 try{queued=await call('send','POST',payload);}
 catch(error){const message=error.message||name+' did not accept the text.';toastMessage(message,'error');return {ok:false,error:message};}
 const began=Date.now();let noted=false;
 for(;;){
  await sleep(options.interval||300);
  let reply;try{reply=await call('result?requestId='+encodeURIComponent(queued.requestId));}catch(error){reply={state:'lost',message:error.message};}
  if(reply.state==='done'){const message=doneMessage(reply.outcome,options.target);toastMessage(message,'success');return {ok:true,outcome:reply.outcome,message};}
  if(reply.state!=='queued'&&reply.state!=='taken'){const message=name+': '+(reply.message||'The text was not accepted.');toastMessage(message,'error');return {ok:false,error:message};}
  if(!noted&&Date.now()-began>2500){noted=true;toastMessage('Waiting for '+name+' to take the text…');}
  if(Date.now()-began>75000){const message=name+' did not confirm the text. Check that window.';toastMessage(message,'error');return {ok:false,error:message};}
 }
}
// ---- Receiving ----
function dialogOpen(){return !!document.querySelector('#modalRoot .modal,#modalRoot [role="dialog"],.nt-editor-dialog,[data-ed-dialog],#uxCommandOverlay');}
const refuse=error=>({ok:false,error});
async function receiveTerminal(request){
 const send=root.NetworkToolkitEditorSendTerminal;
 if(!send?.deliver||typeof request.text!=='string'||!request.text)return refuse(NAMES.terminal+' cannot take the text right now.');
 // Never replace a connection form, a host key or sign-in prompt, or a paste review.
 if(dialogOpen())return refuse('A dialog is open in '+NAMES.terminal+'. Finish or close it there, then send again.');
 const entry=send.targets().find(item=>item.info.id===request.session);
 if(!entry)return refuse('That terminal session is no longer connected. Nothing was sent.');
 return send.deliver(entry,request.text,{report:true});
}
async function receiveEditor(request){
 const send=root.NetworkToolkitTerminalSendEditor,editor=root.NetworkToolkitFileEditor;
 if(!send?.deliver||!editor?.listDocuments||typeof request.text!=='string'||!request.text)return refuse(NAMES.editor+' cannot take the text right now.');
 if(dialogOpen())return refuse('A dialog is open in '+NAMES.editor+'. Finish or close it there, then send again.');
 let docId='';
 if(request.target!=='new'){const doc=editor.listDocuments().find(item=>item.uid===request.target);if(!doc)return refuse('That document is no longer open. Nothing was added.');if(doc.blocked)return refuse(doc.blocked);docId=doc.id;}
 return send.deliver({docId,where:request.where==='line'?{line:Number(request.line)}:request.where||'end',text:request.text,label:clip(request.label)||'Terminal output',report:true});
}
async function receive(request){
 if(receiving||!request||typeof request.requestId!=='string')return;receiving=true;
 let result;
 try{result=mode==='terminal'?await receiveTerminal(request):await receiveEditor(request);}catch(error){result=refuse(error?.message||'The text could not be delivered.');}
 try{await call('ack','POST',result?.ok?{requestId:request.requestId,ok:true,outcome:result.outcome}:{requestId:request.requestId,ok:false,error:String(result?.error||'The text was not accepted.').slice(0,280)});}catch{}
 finally{receiving=false;}
}
// The summary goes to this app's engine when the engine asks for it, on this
// page's first poll, and afterwards only when it changes.
let reported=null;
async function report(force){const targets=summary(),json=JSON.stringify(targets);if(!force&&json===reported)return;await call('report','POST',{targets});reported=json;}
async function poll(){
 if(polling||receiving||!mode||!live())return;polling=true;
 try{const reply=await call('inbox');
  // An engine with peer send always answers with a "report" flag.
  if(typeof reply?.report==='boolean')await report(reply.report||reported===null);
  polling=false;if(reply?.request)await receive(reply.request);}
 catch{}finally{polling=false;}
}
function start(app){if(app!=='editor'&&app!=='terminal')return false;mode=app;if(started)return true;started=true;setInterval(()=>void poll(),1000);return true;}
root.NetworkToolkitPeerSend=Object.freeze({start,sender,peerKind,peerName,targets,open,send,poll,summary});
})(window);
