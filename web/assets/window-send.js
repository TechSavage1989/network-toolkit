/* Window send: "Send to terminal" and "Send to Text Editor" between the
 * windows of ONE engine (owner request: popped-out windows keep every feature
 * of the single Toolkit window). The main Toolkit window, its detached
 * Terminal windows and its detached Text Editor windows each show their own
 * sessions and documents; a send from one of them can reach a session or a
 * document in another (cmd/network-toolkit/window_send.go).
 *
 * Every such window polls its engine's /api/window-send/inbox (once a second;
 * the main window every few seconds while no other window is open) and
 * reports what it can receive when asked and when that changes: the connected
 * sessions it shows and owns, and its open documents (names only). A sender
 * lists the other windows' targets and sends text; the engine checks that the
 * window still holds the target and queues it for that window only.
 *
 * Receiving uses peer-send.js's receive code, which is the code of a send
 * inside one window: the Terminal's pasteText (paste review, paste warnings,
 * bracketed paste, 1 MiB limit) and the editor's newDocument / insertText /
 * pickInsert. It never replaces an open dialog: blocked() is asked before the
 * window changes page and again immediately before the text goes in. No
 * Enter is added and only the text the user chose travels.
 *
 * Edit in the SFTP browser of a detached Terminal workspace (owner decision)
 * asks the main window to open that remote file (kind "remote"): the main
 * window opens it with the same code as its own SFTP browser's Edit, on the
 * same SSH connection; the engine admits only that file (window_send.go).
 *
 * This is not peer send: nothing leaves the engine, and a standalone app's
 * windows do not use it (its engine does not offer it). */
(function(root){
'use strict';
const document=root.document;if(!document)return;
const ID=/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/,WINDOW=/^(main|terminal:[0-9a-f]{32}|editor:[0-9a-f]{32})$/;
let polling=false,receiving=false,others=0,reported=null,ticks=0;
const g=name=>{try{return root[name];}catch{return undefined;}};
function toastMessage(message,kind){const t=g('toast');if(typeof t==='function')t(message,kind);}
function clip(value,limit=160){return String(value??'').replace(/[\u0000-\u001f\u007f-\u009f]/g,' ').trim().slice(0,limit);}
function sleep(ms){return new Promise(resolve=>setTimeout(resolve,ms));}
// Which window this page is: 'main', a detached 'terminal' or a detached 'editor'.
function kind(){try{if(typeof EDITOR_WINDOW!=='undefined'&&EDITOR_WINDOW)return 'editor';if(typeof DETACHED_TERMINAL!=='undefined'&&DETACHED_TERMINAL)return 'terminal';if(typeof WINDOW_MODE!=='undefined'&&WINDOW_MODE==='main')return 'main';}catch{}return '';}
function ownName(){const k=kind();return k==='terminal'?'the Terminal window':k==='editor'?'the Text Editor window':'the main window';}
// The engine says whether it offers window send (only the main Toolkit engine does).
function enabled(){try{return !!kind()&&typeof status!=='undefined'&&status?.windowSend===true&&typeof online!=='undefined'&&online===true&&!(typeof shutDown!=='undefined'&&shutDown)&&!(typeof OFFLINE!=='undefined'&&OFFLINE);}catch{return false;}}
function call(path,method='GET',body){const api=g('api');if(typeof api!=='function')return Promise.reject(new Error('The app is not ready.'));return api('/window-send/'+path,method,body);}
function peer(){return root.NetworkToolkitPeerSend;}
function reportBody(){const p=peer(),k=kind();return {targets:k==='editor'?[]:p?.sessionSummary?.()||[],documents:k==='terminal'?[]:p?.documentSummary?.()||[]};}
async function report(force){const body=reportBody(),json=JSON.stringify(body);if(!force&&json===reported)return;await call('report','POST',body);reported=json;}
// What the other windows of this engine can receive: {windows:[{window,name,targets}], newDocument?, newName?}.
async function targets(want){
 if(!enabled()||(want!=='terminal'&&want!=='editor'))return {windows:[],others:0};
 try{const reply=await call('targets?kind='+want);
  others=Number(reply.others)||0;
  const windows=(Array.isArray(reply.windows)?reply.windows:[]).filter(w=>w&&WINDOW.test(w.window||'')).map(w=>({window:w.window,name:clip(w.name)||'Window',
   targets:(Array.isArray(w.targets)?w.targets:[]).filter(t=>t&&ID.test(t.id||'')).map(t=>({id:t.id,name:clip(t.name)||'Untitled',type:typeof t.type==='string'?t.type:'',unsaved:t.unsaved===true,busy:t.busy===true,active:t.active===true}))}));
  const out={windows,others};
  if(WINDOW.test(reply.newDocument||'')){out.newDocument=reply.newDocument;out.newName=clip(reply.newName)||'the main window';}
  return out;}
 catch(error){return {windows:[],others:0,error:error.message};}
}
function doneMessage(outcome,target,name){
 if(outcome==='opened')return 'Opened '+target+' in the Text Editor of '+name+'.';
 if(outcome==='review')return name+' is showing the text for review before it goes to '+target+'.';
 if(outcome==='pick')return name+' is asking where the text goes in '+target+'.';
 return 'Sent to '+target+' in '+name+'.';
}
// payload: {kind:'terminal',window,session,text} or {kind:'editor',window,target,where,line,label,text}.
// Resolves when the receiving window has taken the text or refused it.
async function send(payload,options={}){
 let queued;
 try{queued=await call('send','POST',payload);}
 catch(error){const message=error.message||'The other window did not accept the text.';toastMessage(message,'error');return {ok:false,error:message};}
 const name=clip(queued.name)||'The other window',target=options.target||(payload.kind==='terminal'?'the terminal session':'the document'),began=Date.now();let noted=false;
 for(;;){
  await sleep(options.interval||300);
  let reply;try{reply=await call('result?requestId='+encodeURIComponent(queued.requestId));}catch(error){reply={state:'lost',message:error.message};}
  if(reply.state==='done'){const message=doneMessage(reply.outcome,target,name);toastMessage(message,'success');return {ok:true,outcome:reply.outcome,message};}
  if(reply.state!=='queued'&&reply.state!=='taken'){const message=name+': '+(reply.message||'The text was not accepted.');toastMessage(message,'error');return {ok:false,error:message};}
  if(!noted&&Date.now()-began>2500){noted=true;toastMessage('Waiting for '+name+' to take the text…');}
  if(Date.now()-began>75000){const message=name+' did not confirm the text. Check that window.';toastMessage(message,'error');return {ok:false,error:message};}
 }
}
// The main window opens a remote file that a detached Terminal workspace
// asked for, with its SFTP browser's own Edit code (terminal-sftp-sidebar.js).
async function receiveRemote(request,guard){
 const editor=root.NetworkToolkitFileEditor,api=g('api'),session=String(request.session||''),path=String(request.path||'');
 if(!editor?.open||typeof api!=='function'||!ID.test(session)||!path)return {ok:false,error:'The Text Editor cannot open the file right now.'};
 let busy=guard();if(busy)return {ok:false,error:busy};
 const navigate=g('navigate');if(typeof navigate==='function')navigate('editor');
 await new Promise(resolve=>setTimeout(resolve,50));
 if((busy=guard()))return {ok:false,error:busy};
 const revision=(()=>{try{return typeof terminalSessionRevision==='function'?terminalSessionRevision(session):0;}catch{return 0;}})();
 const call=(action,extra)=>api('/remote-browser','POST',{protocol:'ssh',sessionId:session,action,...extra});
 const d=await editor.open({source:'SSH terminal',path,id:'terminal:'+session+':'+path,binding:{kind:'terminal',sessionId:session,revision},read:async p=>call('read-text',{path:p}),write:async data=>call('write-text',data)});
 // The engine's grant follows this tab: closing it, or moving it away, ends the grant.
 return ID.test(d?.uid||'')?{ok:true,outcome:'opened',document:d.uid}:{ok:false,error:'The Text Editor did not open the file.'};
}
// A detached Terminal workspace: open one remote file in the main window's Text Editor.
function openRemote({session,path,name}={}){return send({kind:'remote',window:'main',session,path},{target:clip(name)||'the file'});}
async function receive(request,want){
 if(receiving||!request||typeof request.requestId!=='string')return;receiving=true;
 const p=peer(),k=kind(),name=ownName(),guard=()=>p?.blocked?p.blocked(name,k==='main'):'';
 let result;
 try{result=want==='remote'&&k==='main'?await receiveRemote(request,guard):want==='terminal'&&k!=='editor'&&p?.receiveTerminal?await p.receiveTerminal(request,guard,name):want==='editor'&&k!=='terminal'&&p?.receiveEditor?await p.receiveEditor(request,guard,name):{ok:false,error:name+' cannot take this text.'};}
 catch(error){result={ok:false,error:error?.message||'The text could not be delivered.'};}
 try{await call('ack','POST',result?.ok?{requestId:request.requestId,ok:true,outcome:result.outcome,...(result.document?{document:result.document}:{})}:{requestId:request.requestId,ok:false,error:String(result?.error||'The text was not accepted.').slice(0,280)});}catch{}
 finally{receiving=false;}
}
async function poll(){
 if(polling||receiving||!enabled())return;polling=true;
 try{const reply=await call('inbox');others=Number(reply?.others)||0;
  if(typeof reply?.report==='boolean')await report(reply.report||reported===null);
  polling=false;if(reply?.request)await receive(reply.request,reply.kind);}
 catch{}finally{polling=false;}
}
// The main window polls every few seconds while it is the only window; any
// detached window, and the main window beside one, once a second.
setInterval(()=>{ticks++;if(kind()==='main'&&others===0&&ticks%3)return;void poll();},1000);
root.NetworkToolkitWindowSend=Object.freeze({enabled,kind,others:()=>others,targets,send,poll,openRemote});
})(window);
