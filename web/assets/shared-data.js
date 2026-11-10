/* Saved connections are shared by every Network Toolkit app that uses the
   same data folder: the main window (portable or installed), the Terminal and
   the Text Editor (docs/SHARED_APP_DATA.md).

   The engine merges every save with what the other apps saved. This module
   tells it which shared values this page holds (their revision), takes the
   merged values back, and asks every few seconds, while the window is
   visible, whether another app changed something. A change is applied only
   when nothing here is unsaved and no dialog, menu or drag is in progress;
   until then the page keeps naming its old revision, so its next save is
   still merged correctly. */
(function(root){'use strict';
const POLL_MS=2500;
// The shared values a running page refreshes. Editor preferences are shared
// too, but an editor reads them when it starts.
const LIVE=['terminalProfiles','terminalRecent','terminalSavedManualOrder','terminalPromptNames','ftpProfiles','ftpRecent','sftpRecent','sftpVerifiedHostKeys'];
let revision='',timer=null,polling=false,writes=0,stopped=false,noticeHandler=null;

const text=(value,limit)=>typeof value==='string'?value.slice(0,limit):'';
const list=value=>Array.isArray(value)?value:[];

// The same clean-up the page applies to these values when it loads its
// settings (mergeState and mergeTerminalPreferences in index.html, and the
// session sidebar), so a refreshed list looks exactly like a loaded one.
function normalize(values,deps={}){
 const source=values&&typeof values==='object'?values:{};
 const config=deps.config||root.safeTerminalConfig,labels=deps.labels||root.NetworkToolkitTerminalPromptLabels,folders=deps.folders||root.NetworkToolkitSessionFolders;
 const revisionOf=item=>labels?.revision?.(item)||0,folder=value=>folders?.normalize?folders.normalize(value):'';
 const named=item=>item&&typeof item==='object'&&typeof item.id==='string'&&typeof item.name==='string';
 const out={};
 out.terminalProfiles=list(source.terminalProfiles).filter(named).slice(0,200).map(p=>({id:p.id.slice(0,80),name:p.name.slice(0,100),...(typeof p.nameExplicit==='boolean'?{nameExplicit:p.nameExplicit}:{}),nameRevision:revisionOf(p),folder:folder(p.folder),...config(p)}));
 const seen=new Set();
 out.terminalRecent=list(source.terminalRecent).filter(named).slice(0,50).map(item=>({id:item.id.slice(0,80),name:item.name.slice(0,100),...(typeof item.nameExplicit==='boolean'?{nameExplicit:item.nameExplicit}:{}),nameRevision:revisionOf(item),lastUsed:Number.isSafeInteger(item.lastUsed)&&item.lastUsed>=0?item.lastUsed:0,...config(item)})).filter(item=>{if(!item.id||seen.has(item.id))return false;seen.add(item.id);return true;});
 out.terminalSavedManualOrder=source.terminalSavedManualOrder===true;
 out.terminalPromptNames=labels?.normalize?labels.normalize(source.terminalPromptNames):list(source.terminalPromptNames);
 out.ftpProfiles=list(source.ftpProfiles).filter(x=>x&&typeof x.id==='string'&&typeof x.name==='string'&&typeof x.host==='string').slice(0,100).map(x=>({id:x.id,name:x.name,folder:folder(x.folder),host:x.host,port:Number(x.port)||21,username:typeof x.username==='string'?x.username:'',security:['explicit','implicit','plain'].includes(x.security)?x.security:'explicit',fingerprint:typeof x.fingerprint==='string'?x.fingerprint:'',credentialId:typeof x.credentialId==='string'?x.credentialId:''}));
 out.ftpRecent=list(source.ftpRecent).filter(x=>x&&typeof x.id==='string'&&typeof x.name==='string'&&typeof x.host==='string'&&typeof x.username==='string'&&Number.isInteger(Number(x.port))&&Number(x.port)>=1&&Number(x.port)<=65535).slice(0,50).map(x=>({id:x.id.slice(0,80),name:x.name.slice(0,100),host:x.host.slice(0,253),port:Number(x.port),username:x.username.slice(0,256),security:['explicit','implicit','plain'].includes(x.security)?x.security:'explicit',fingerprint:text(x.fingerprint,256),credentialId:text(x.credentialId,128),lastUsed:Number.isSafeInteger(x.lastUsed)?x.lastUsed:0}));
 out.sftpRecent=list(source.sftpRecent);
 out.sftpVerifiedHostKeys=source.sftpVerifiedHostKeys&&typeof source.sftpVerifiedHostKeys==='object'&&!Array.isArray(source.sftpVerifiedHostKeys)?source.sftpVerifiedHostKeys:{};
 return out;
}

// What a save sends: the page's settings plus the revision it holds.
function outgoing(current){writes++;return revision?{...current,_sharedBase:revision}:current;}
function failed(){writes=Math.max(0,writes-1);}

// Something of the user's is in progress: do not change lists under it.
function busy(){
 try{
  if(saveTimer!==null||writes>0)return true;
  if(stateRevision!==(savedStateRevision<0?0:savedStateRevision))return true;
 }catch{return true;}
 if(document.querySelector('#modalRoot .modal,#sessionLibraryContextMenu,.session-saved-dragging'))return true;
 if(document.body.classList.contains('resizing-columns'))return true;
 const active=document.activeElement;
 return !!(active&&/^(INPUT|TEXTAREA|SELECT)$/.test(active.tagName)&&active.closest('#terminalSessionSidebar,[data-rb-root]'));
}

function redraw(keys){
 const attempt=task=>{try{task();}catch(error){console.error('Shared data refresh',error);}};
 attempt(()=>root.NetworkToolkitSavedSessions?.sync?.());
 attempt(()=>{if(document.getElementById('terminalSessionSidebar'))root.NetworkToolkitSessionSidebar?.render?.();});
 attempt(()=>{if(typeof renderTerminalProfiles==='function')renderTerminalProfiles();});
 for(const kind of ['ssh','ftp'])attempt(()=>{if(document.querySelector('[data-rb-root="'+kind+'"]'))root.NetworkToolkitRemoteBrowser?.render?.(kind);});
 document.dispatchEvent(new CustomEvent('shared-data-changed',{detail:{keys}}));
}

// Put shared values into the page's settings and redraw what shows them.
function apply(values){
 const next=normalize(values),changed=[];
 for(const key of LIVE){
  if(JSON.stringify(state[key])===JSON.stringify(next[key]))continue;
  state[key]=next[key];changed.push(key);
 }
 if(changed.length)redraw(changed);
 return changed;
}

// After a save. sent is the page's change counter when the save was made.
function saved(response,sent){
 writes=Math.max(0,writes-1);
 const shared=response?.sharedData;
 if(!shared||typeof shared.revision!=='string')return;
 if(!shared.values){revision=shared.revision;return;}
 // The engine merged in something this page did not have. If the user has
 // changed anything since, keep the old revision: the next save carries it.
 let current=sent;try{current=stateRevision;}catch{}
 if(current!==sent||saveTimer!==null||document.querySelector('#modalRoot .modal,#sessionLibraryContextMenu,.session-saved-dragging'))return;
 apply(shared.values);revision=shared.revision;
}

function ready(){
 try{return stateLoaded&&online&&!OFFLINE&&!shutDown&&!EDITOR_WINDOW&&!DETACHED_TERMINAL;}catch{return false;}
}

async function poll(){
 if(polling||stopped||!ready()||document.visibilityState==='hidden')return false;
 polling=true;
 try{
  const result=await api('/shared-data?revision='+encodeURIComponent(revision));
  if(!result||typeof result.revision!=='string')return false;
  if(Array.isArray(result.notices)&&result.notices.length)noticeHandler?.(result.notices);
  if(result.credentials!==undefined)credentialsChanged(result.credentials);
  if(!result.values){revision=result.revision;return false;}
  if(busy())return false;
  apply(result.values);revision=result.revision;return true;
 }catch{return false;}
 finally{polling=false;}
}

// The password manager keeps a list of saved credentials; another app may
// have added one for a connection that is now shown here.
let credentialMark;
function credentialsChanged(mark){
 if(credentialMark===undefined){credentialMark=mark;return;}
 if(credentialMark===mark)return;
 credentialMark=mark;
 try{root.NetworkToolkitCredentials?.refresh?.();}catch{}
}

let listening=false;
function start(){
 stopped=false;
 if(timer!==null||typeof root.setInterval!=='function')return;
 timer=root.setInterval(()=>{void poll();},POLL_MS);timer?.unref?.();
 if(!listening){listening=true;document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='visible')void poll();});}
}
function stop(){stopped=true;if(timer!==null)root.clearInterval(timer);timer=null;}

const api_=Object.freeze({normalize,outgoing,saved,failed,apply,poll,start,stop,busy,LIVE,POLL_MS,onNotices(handler){noticeHandler=handler;},get revision(){return revision;}});
root.NetworkToolkitSharedData=api_;
if(typeof module==='object'&&module.exports)module.exports=api_;
if(typeof document!=='undefined')start();
})(typeof window!=='undefined'?window:globalThis);
