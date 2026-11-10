/* Notepad-style session restore for the primary Text Editor workspaces: the
   main window and the standalone Text Editor. The local engine keeps a small
   metadata index plus one kept copy per document with unsaved text in the
   workspace's app data folder (editor_session.go). Disk-backed documents are
   copied by the engine from a stable buffer snapshot, so their text never
   passes through this renderer; only small memory documents send their text.
   Records never contain bindings, connection identities or credentials, and
   detached editor windows never save or restore a session. */
(function(root){'use strict';
const ENDPOINT='/editor-session',TICK_MS=3000,RETRY_MS=15000,TAB_LIMIT=64;
const ENCODINGS=['UTF-8','UTF-16LE','UTF-16BE'],EOLS=['LF','CRLF','CR'];
const absolutePath=path=>/^(?:[A-Za-z]:[\\/]|\\\\[^\\/]|\/)/.test(String(path||''));
const baseName=path=>String(path||'').split(/[\\/]/).pop()||'';
const label=(text,limit)=>String(text||'').replace(/[\u0000-\u001f\u007f-\u009f]+/g,' ').trim().slice(0,limit);
const format=(encoding,bom,eol)=>({encoding:ENCODINGS.includes(encoding)?encoding:'UTF-8',bom:bom===true,eol:EOLS.includes(eol)?eol:'LF'});
const currentFormat=d=>format(d.encoding,d.bom,d.eol);
const savedFormat=d=>format(d.savedEncoding||d.encoding,d.savedBOM===undefined?d.bom:d.savedBOM,d.savedEol||d.eol);
const languageOf=d=>/^[A-Za-z0-9._+-]{1,64}$/.test(d.language||'')?d.language:'';

// Where a tab reopens from. A remote, browser-handle or imported document can
// only come back as a copy of its unsaved text.
function kindOf(d){
 if(d.sessionOrigin)return 'copy';
 const binding=d.binding?.kind;
 if(!d.untitled&&absolutePath(d.path)&&(binding==='local'||binding==='localDraft'&&d.source==='Local computer'))return 'local';
 return d.untitled?'untitled':'copy';
}
const blank=d=>d.disk?!(d.diskMeta?.units>0):!d.content;
const originOf=d=>label(d.sessionOrigin||[d.source,d.path].filter(Boolean).join(' · '),1024)||'another location';
const nameOf=d=>label(baseName(d.path),255)||'Untitled.txt';

// The records for the open tabs, in tab order. Clean remote documents and
// blank, untouched drafts have nothing to keep and are left out.
function plan(docs){
 const rows=[];
 for(const d of docs){
  if(d.staged)continue;
  const kind=kindOf(d),language=languageOf(d),extra=language?{language}:{};
  if(kind==='local'){
   if(!d.dirty){rows.push({d,record:{kind,path:d.path,...extra,dirty:false}});continue;}
   rows.push({d,keep:true,record:{kind,path:d.path,...extra,dirty:true,hash:/^[0-9a-f]{64}$/.test(d.hash||'')?d.hash:'',format:currentFormat(d),savedFormat:savedFormat(d)}});
  }else if(kind==='untitled'){
   if(!d.dirty&&blank(d))continue;
   rows.push({d,keep:true,record:{kind,name:nameOf(d),...extra,dirty:true,format:currentFormat(d)}});
  }else if(d.dirty){
   rows.push({d,keep:true,record:{kind,name:nameOf(d),origin:originOf(d),...extra,dirty:true,format:currentFormat(d)}});
  }
 }
 return rows;
}

function create(host){
 let phase='idle',carried=[],lastSignature='',lastIndex='',retryAt=0,chain=Promise.resolve(),running=0,timer=null,waiting=null,waitStarted=0;
 const kept=new WeakMap();
 const signature=()=>{const docs=host.documents();return JSON.stringify([host.activeId(),carried.length,docs.map(d=>[d.id,d.uid,d.path,!!d.dirty,!!d.staged,!!d.disk,d.language||'',host.stamp(d)])]);};

 // Keep one document's unsaved text. A copy whose revision is still current
 // is reused; background saves of large disk documents are spaced out so
 // writing copies stays a small share of the time spent editing.
 async function keepCopy(d,final){
  const key=(d.disk?'disk:':'memory:')+d.uid,previous=d.sessionBackup?.key===key?d.sessionBackup:null;
  if(d.disk)await host.flush(d);
  const revision=d.editRevision,stamp=host.stamp(d),settled=!d.disk||!host.pending(d);
  if(previous&&settled&&previous.revision===revision)return {ref:previous.ref,stamp};
  if(previous&&!final&&Date.now()<previous.next)return {ref:previous.ref,stamp:null};
  const started=Date.now();
  const result=d.disk?await host.request(ENDPOINT+'/backup','POST',{source:'buffer',uid:d.uid,lease:host.lease(d)}):await host.request(ENDPOINT+'/backup','POST',{source:'text',text:d.content});
  if(!result||!/^[0-9a-f]{32}$/.test(result.id||''))throw Error('The engine did not confirm the kept copy.');
  const ref={id:result.id,size:result.size,sha256:result.sha256},copied=d.disk?result.revision:revision,elapsed=Number.isFinite(result.elapsedMs)?result.elapsedMs:Date.now()-started;
  d.sessionBackup={key,revision:copied,ref,next:Date.now()+elapsed*9};
  return {ref,stamp:settled&&copied===revision?stamp:null};
 }

 async function write(final){
  if(phase!=='ready')return new Set();
  const startSignature=signature(),docs=host.documents().filter(d=>!d.staged),activeId=host.activeId(),rows=plan(docs),stamps=new Map(rows.map(row=>[row.d,host.stamp(row.d)]));
  let stale=false;
  for(const row of rows){
   if(!row.keep)continue;
   // A document that cannot be copied right now keeps its previous copy in
   // the index, so a failed or postponed copy never removes older text.
   const previous=row.d.sessionBackup?.ref?{ref:row.d.sessionBackup.ref,stamp:null}:null;
   if(row.d.loading||row.d.promoting||row.d.frozen&&!final){stale=true;row.copy=previous;continue;}
   try{row.copy=await keepCopy(row.d,final);if(!row.copy.stamp)stale=true;}
   catch(error){row.error=error;stale=true;row.copy=previous;}
  }
  const tabs=[],now=new Map();let active=-1;
  for(const row of rows){
   if(row.keep){if(!row.copy)continue;row.record.backup=row.copy.ref;if(row.copy.stamp)now.set(row.d,row.copy.stamp);}
   else now.set(row.d,stamps.get(row.d));
   if(row.d.id===activeId)active=tabs.length;
   tabs.push(row.record);
  }
  for(const record of carried)if(tabs.length<TAB_LIMIT)tabs.push(record);
  const index=JSON.stringify({version:1,active,tabs});
  if(index!==lastIndex){
   // A refused index (for example a kept copy that was removed) makes every
   // document write a fresh copy on the next attempt.
   try{await host.request(ENDPOINT,'PUT',{version:1,active,tabs});}
   catch(error){for(const row of rows)delete row.d.sessionBackup;throw error;}
   lastIndex=index;
  }
  for(const [d,stamp] of now)kept.set(d,stamp);
  lastSignature=stale?'':startSignature;
  const failed=rows.filter(row=>row.error);
  if(failed.length&&final)host.message('Could not keep '+failed.map(row=>nameOf(row.d)).join(', ')+' for next time: '+failed[0].error.message,true);
  return new Set(docs.filter(d=>isKept(d)));
 }

 // Saves never overlap. A failed background save is retried after a pause.
 function save({final=false}={}){
  running++;
  const next=chain.then(()=>write(final));
  chain=next.catch(()=>{}).finally(()=>{running--;});
  return next.catch(error=>{lastSignature='';retryAt=Date.now()+RETRY_MS;if(final)host.message('Open files could not be kept for next time: '+error.message,true);throw error;});
 }

 function tick(){
  if(phase!=='ready'||running||!host.enabled()||Date.now()<retryAt||!host.idle())return false;
  if(signature()===lastSignature)return false;
  void save().catch(()=>{});
  return true;
 }

 function isKept(d){const stamp=kept.get(d);return stamp!==undefined&&stamp===host.stamp(d);}

 // Reopen the stored tabs in order. Every record that cannot be reopened now
 // but still names a kept copy is carried into later saves, so unsaved text
 // is never dropped because of the tab limit or a temporary failure.
 async function restore(){
  if(phase!=='idle')return false;
  phase='restoring';
  let session;
  try{session=await host.request(ENDPOINT,'GET');}
  catch(error){phase='failed';host.message('Files from the last session could not be reopened: '+error.message,true);return false;}
  const records=Array.isArray(session?.tabs)?session.tabs:[],notes=session?.warning?[session.warning]:[],earlier=new Set(host.documents());
  const tasks=records.map(record=>{try{return Promise.resolve(host.restoreRecord(record));}catch(error){return Promise.reject(error);}});
  const results=await Promise.allSettled(tasks),restored=[],carry=[];let activeDoc=null,limited=0;
  results.forEach((result,index)=>{
   const record=records[index],value=result.status==='fulfilled'?result.value||{}:{error:result.reason,carry:true};
   if(value.note)notes.push(value.note);
   if(value.doc){restored.push(value.doc);if(index===session.active)activeDoc=value.doc;return;}
   if(value.carry&&record.backup){carry.push(record);if(value.limit)limited++;}
   if(value.error&&!value.limit)notes.push('Could not reopen '+(record.kind==='local'?baseName(record.path):record.name)+': '+(value.error.message||value.error));
  });
  carried=carry;
  if(limited)notes.push(limited+' unsaved tab(s) from the last session did not fit in 12 tabs. Their text is kept and they reopen the next time the editor starts with fewer tabs open.');
  const userOpened=host.documents().some(d=>!restored.includes(d))||earlier.size>0;
  host.finishRestore({docs:restored,active:userOpened?null:activeDoc});
  phase='ready';lastSignature='';lastIndex='';startTimer();
  if(notes.length)host.message(notes.join(' '),false);
  return true;
 }

 // Wait until saved settings and the engine are available, then restore once.
 function start(){
  if(phase!=='idle'||waiting)return;
  waitStarted=Date.now();
  const attempt=()=>{
   waiting=null;
   if(phase!=='idle')return;
   const state=host.readiness();
   if(state==='never'){phase='unavailable';return;}
   if(state==='off'){phase='off';return;}
   if(state==='ready'){void restore();return;}
   if(Date.now()-waitStarted<120000)waiting=root.setTimeout(attempt,250);
  };
  attempt();
 }

 // Turning the preference back on restores nothing (the tabs are already
 // open) and starts keeping the current tabs.
 function enable(){if(phase==='off'){root.clearTimeout(waiting);waiting=null;phase='ready';lastSignature='';lastIndex='';carried=[];startTimer();}}
 async function disable(){root.clearTimeout(waiting);waiting=null;phase='off';carried=[];lastIndex='';lastSignature='';await chain;for(const d of host.documents())delete d.sessionBackup;await host.request(ENDPOINT,'DELETE');}
 function startTimer(){if(timer===null&&typeof root.setInterval==='function'){timer=root.setInterval(tick,TICK_MS);timer?.unref?.();}}
 // After a window close is committed nothing may overwrite the final session.
 function stop(){if(timer!==null)root.clearInterval(timer);timer=null;root.clearTimeout(waiting);waiting=null;phase='closed';}
 return Object.freeze({start,restore,save,tick,isKept,enable,disable,stop,get phase(){return phase;},get carried(){return carried.slice();},get busy(){return running>0;}});
}

// The page's engine status (index.html) says whether this workspace keeps a
// session; file-editor.js has its own status() function, so it asks here.
const engineAvailable=()=>typeof status!=='undefined'&&status?.editorSessionAvailable===true;
const api=Object.freeze({create,plan,kindOf,engineAvailable,ENDPOINT,TICK_MS});
root.NTKEditorSession=api;
if(typeof module==='object'&&module.exports)module.exports=api;
})(globalThis);
