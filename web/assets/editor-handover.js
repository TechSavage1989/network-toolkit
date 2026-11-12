/* Document hand-over (owner decision): popping the Text Editor out of the main
 * Toolkit window adds tabs to the standalone Text Editor when that app is
 * already running with the same data folder, instead of opening a new
 * connected window for them.
 *
 * Sending (main window, editor-windows.js move() to "new" with handover:
 * Detach editor and Move all only; a single tab moved to a separate window
 * always gets its own connected window, owner decision):
 *   - Handed over: untitled documents and local files (saved, or with unsaved
 *     text held by this page). Saved files that this window holds disk-backed
 *     are reopened from disk there.
 *   - Not handed over, they still open in a connected window as before: files
 *     opened over SFTP or SSH, binary files, unsaved changes in large
 *     (disk-backed) documents, formats that cannot be carried exactly, files
 *     not on this computer or not saved yet, and files that changed on disk.
 *   - The handed-over tabs stay here, read-only, until the Text Editor confirms
 *     each one; only confirmed tabs leave this window. A refusal, a timeout or
 *     an editor that closes leaves them here unchanged, and the user is told.
 *   - This page talks to its own engine only (/api/peer-handover/); the engine
 *     finds the editor through its protected record (peer_handover.go).
 * Receiving (standalone Text Editor, peer-send.js receive() of "documents"):
 *   each tab opens through the editor's ordinary local open or as a new
 *   document (NetworkToolkitFileEditor.adoptHandover), behind the same guard
 *   as text from another app, and unsaved text is confirmed only once this
 *   editor keeps it for next time (when "Reopen files next time" is on).
 * Undo history does not travel: the first Undo there returns to the file's
 * text. Never credentials, tokens, host keys, connections or remote paths. */
(function(root){
'use strict';
const TEXT_LIMIT=8*1024*1024,WAIT_MS=80000,ENCODINGS=['UTF-8','UTF-16LE','UTF-16BE'],EOLS=['LF','CRLF','CR'];
const REASONS={
 sftp:['uses an SFTP connection','use an SFTP connection'],
 binary:['is a binary file','are binary files'],
 large:['has unsaved changes in a large document','have unsaved changes in large documents'],
 format:['uses an encoding or line endings that cannot be carried exactly','use encodings or line endings that cannot be carried exactly'],
 notLocal:['is not a file on this computer','are not files on this computer'],
 unsaved:['is not saved to disk yet','are not saved to disk yet'],
 disk:['changed on disk since it was opened','changed on disk since they were opened'],
 size:['is too large to move at once','are too large to move at once'],
 mixed:['needs this window','need this window']
};
const g=name=>{try{return root[name];}catch{return undefined;}};
const clip=(value,limit=280)=>String(value??'').replace(/[\u0000-\u001f\u007f-\u009f]+/g,' ').trim().slice(0,limit);
const tabs=n=>n===1?'1 tab':n+' tabs';
const sleep=ms=>new Promise(resolve=>root.setTimeout(resolve,ms));
const absolute=path=>/^(?:[A-Za-z]:[\\/]|\/(?!\/))/.test(String(path||''));
const baseName=path=>String(path||'').split(/[\\/]/).pop()||'';
const carried=(encoding,eol)=>ENCODINGS.includes(encoding)&&EOLS.includes(eol);
function bytes(text){try{return new TextEncoder().encode(text).length;}catch{return text.length*3;}}
function call(path,method='GET',body){const api=g('api');if(typeof api!=='function')return Promise.reject(Error('The app is not ready.'));return api('/peer-handover/'+path,method,body);}
// Only the main Toolkit window hands tabs over.
function enabled(){try{return typeof status!=='undefined'&&status?.windowMode==='main';}catch{return false;}}

// Why a tab opens in a connected window instead, or '' when it can be handed over.
function reasonFor(d){
 const kind=d?.binding?.kind;
 if(kind==='terminal'||kind==='sshConnection')return 'sftp';
 if(d.encoding==='Binary')return 'binary';
 const saved=carried(d.savedEncoding||d.encoding,d.savedEol||d.eol),current=carried(d.encoding,d.eol);
 if(d.untitled&&kind==='localDraft'){if(d.disk)return 'large';return current&&saved?'':'format';}
 if(d.untitled||!absolute(d.path)||!(kind==='local'||kind==='localDraft'&&d.source==='Local computer'))return 'notLocal';
 if(!/^[0-9a-f]{64}$/.test(d.hash||''))return 'unsaved';
 if(d.disk)return d.dirty?'large':'';
 if(!d.dirty)return '';
 return current&&saved?'':'format';
}

function itemFor(v){
 const format={encoding:v.encoding,bom:v.bom===true,eol:v.eol},savedFormat={encoding:v.savedEncoding||v.encoding,bom:v.savedBOM===undefined?v.bom===true:v.savedBOM===true,eol:v.savedEol||v.eol};
 const extra=/^[A-Za-z0-9._+-]{1,64}$/.test(v.language||'')?{language:v.language}:{};
 if(v.untitled)return {id:v.uid,kind:'untitled',name:clip(baseName(v.path),255).replace(/[\\/]/g,' ')||'Untitled.txt',dirty:!!v.dirty,text:v.content||'',format,savedFormat,...extra};
 if(v.disk||!v.dirty)return {id:v.uid,kind:'file',path:v.path,hash:v.hash,...extra};
 return {id:v.uid,kind:'file',path:v.path,hash:v.hash,dirty:true,text:v.content,format,savedFormat,...extra};
}

async function receiver(){if(!enabled())return null;try{const reply=await call('receiver');return reply?.available===true?reply:null;}catch{return null;}}

// Waits for the Text Editor's answer. Its engine ends a request after a minute.
async function outcome(requestId){
 const deadline=Date.now()+WAIT_MS;
 for(;;){
  await sleep(300);
  let reply;
  try{reply=await call('result?requestId='+encodeURIComponent(requestId));}
  catch(error){if(error?.status===400||error?.status===404||Date.now()>deadline)return {state:'lost',items:[],message:clip(error?.message)||'The Text Editor did not confirm the tabs.'};continue;}
  if(reply?.state==='queued'||reply?.state==='taken'){if(Date.now()>deadline)return {state:'lost',items:[],message:'The Text Editor did not confirm the tabs in time. Check that window for copies.'};continue;}
  return {state:String(reply?.state||'lost'),items:Array.isArray(reply?.items)?reply.items:[],message:clip(reply?.message)};
 }
}

function summary(moved,rest,reasons,stayed,stayedWhy,name){
 const parts=[];
 if(moved)parts.push(tabs(moved)+' moved to the open '+name);
 if(rest.length){const kinds=[...new Set(rest.map(uid=>reasons.get(uid)||'mixed'))],[one,many]=REASONS[kinds.length===1?kinds[0]:'mixed'];parts.push(tabs(rest.length)+' that '+(rest.length===1?one:many)+(rest.length===1?' opens':' open')+' in a connected window');}
 if(stayed)parts.push(tabs(stayed)+' stayed here'+(stayedWhy?': '+stayedWhy.replace(/\.$/,''):''));
 return parts.length?parts.join('; ')+'.':'';
}

// ids: the tabs the user moved to a new window. Hands over what it can and
// returns the tabs that still go to a connected window (rest). Tabs that the
// Text Editor did not confirm stay in this window, unchanged.
async function handOver(ids,{adapter}={}){
 const none={rest:[...ids],message:''};
 if(!enabled()||!adapter||!Array.isArray(ids)||!ids.length)return none;
 const live=adapter.documents().filter(d=>ids.includes(d.uid)),reasons=new Map();
 for(const d of live){const why=reasonFor(d);if(why)reasons.set(d.uid,why);}
 const eligible=live.filter(d=>!reasons.has(d.uid)).map(d=>d.uid),others=ids.filter(uid=>!eligible.includes(uid));
 if(!eligible.length)return none;
 // A tab that cannot move to any window keeps today's answer for the whole move.
 if(others.length&&adapter.canTransfer(others))return none;
 const found=await receiver();if(!found)return none;
 const name=clip(found.name,80)||'Text Editor';
 await adapter.flushDocuments?.(eligible);
 if(adapter.canTransfer(eligible))return none;
 let batch;try{batch=adapter.snapshot(eligible);}catch{return none;}
 // From here the eligible tabs are read-only until confirmed or given back.
 const items=[],back=[];let budget=TEXT_LIMIT;
 for(const v of batch.documents){const why=reasonFor(v);if(why){reasons.set(v.uid,why);back.push(v.uid);continue;}const item=itemFor(v),size=bytes(item.text||'');if(size>budget){reasons.set(v.uid,'size');back.push(v.uid);continue;}budget-=size;items.push(item);}
 if(back.length)adapter.rollback(back);
 let moved=[],stayed=[],stayedWhy='';
 const frozen=new Set(items.map(item=>item.id));
 const giveBack=list=>{const keep=list.filter(uid=>frozen.has(uid));if(keep.length)adapter.rollback(keep);for(const uid of keep)frozen.delete(uid);};
 try{
  if(items.length){
   let started;
   try{started=await call('start','POST',{items});}
   catch(error){stayed=[...frozen];stayedWhy=clip(error?.message)||name+' did not take the tabs.';giveBack(stayed);}
   if(started){
    for(const entry of Array.isArray(started.ineligible)?started.ineligible:[]){if(!frozen.has(entry?.id))continue;reasons.set(entry.id,'disk');giveBack([entry.id]);}
    const sent=(Array.isArray(started.sent)?started.sent:[]).filter(uid=>frozen.has(uid));
    if(started.requestId&&sent.length){
     const result=await outcome(started.requestId);
     const confirmed=new Set(result.state==='done'?result.items.filter(item=>item&&item.ok===true&&sent.includes(item.id)).map(item=>item.id):[]);
     moved=sent.filter(uid=>confirmed.has(uid));
     stayed=sent.filter(uid=>!confirmed.has(uid));
     stayedWhy=result.state==='done'?clip(result.items.find(item=>item&&item.ok!==true)?.error):result.message||name+' did not take the tabs.';
     if(moved.length){adapter.release(moved);for(const uid of moved)frozen.delete(uid);}
    }
   }
  }
 }finally{giveBack([...frozen]);}
 const rest=ids.filter(uid=>reasons.has(uid)&&!moved.includes(uid)&&!stayed.includes(uid));
 const message=summary(moved.length,rest,reasons,stayed.length,stayedWhy,name);
 const toast=g('toast');if(message&&typeof toast==='function')toast(message,stayed.length?'error':moved.length?'success':'info');
 return {rest,message,moved,stayed};
}

// ---- Receiving: the standalone Text Editor ----
async function receive(request,{guard}={}){
 const editor=root.NetworkToolkitFileEditor,items=Array.isArray(request?.documents)?request.documents:null;
 if(!editor?.adoptHandover||!items||!items.length)return {ok:false,error:'This Text Editor cannot take tabs right now.'};
 const busy=typeof guard==='function'?guard():'';if(busy)return {ok:false,error:busy};
 const results=[],adopted=[];
 for(const item of items){
  const id=String(item?.id||'');
  // Never replace a dialog that opened while earlier tabs were opening.
  const why=typeof guard==='function'?guard():'';
  if(why){results.push({id,ok:false,error:clip(why)});continue;}
  // created: this request opened the tab. A file already open here returns
  // the existing tab, which this request must never discard or count as its own.
  try{const {document:d,created}=await editor.adoptHandover(item);adopted.push({id,d,created:created===true});results.push({id,ok:true});}
  catch(error){results.push({id,ok:false,error:clip(error?.message)||'The tab could not be opened.'});}
 }
 // Unsaved text is confirmed once this editor keeps it for next time. With
 // "Reopen files next time" off, it is held in this window like any tab.
 const unsaved=adopted.filter(x=>x.created&&x.d.dirty&&x.d.handoverAdopted);
 if(unsaved.length){
  const session=editor.session;let failure='';
  if(session.phase==='ready'){try{await session.save();}catch(error){failure='Its unsaved text could not be kept for next time: '+clip(error?.message);}}
  else if(!['off','unavailable','closed'].includes(session.phase))failure='This Text Editor is still reopening its last session. Try again in a moment.';
  for(const x of unsaved){
   if(!failure&&(session.phase!=='ready'||session.isKept(x.d)))continue;
   const entry=results.find(r=>r.id===x.id);entry.ok=false;entry.error=clip(failure||'Its unsaved text could not be kept for next time.');
   editor.discardAdopted(x.d);
  }
 }
 const first=adopted.find(x=>results.find(r=>r.id===x.id)?.ok);
 if(first)editor.activateDocument?.(first.d.id);
 return {ok:true,outcome:'opened',items:results};
}

const api=Object.freeze({enabled,receiver,reasonFor,itemFor,handOver,receive});
root.NetworkToolkitEditorHandover=api;
if(typeof module==='object'&&module.exports)module.exports=api;
})(globalThis);
