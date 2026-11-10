/* Settings and saved data are shared by every Network Toolkit app that uses
   the same data folder: the main window (portable or installed), the Terminal
   and the Text Editor (docs/SHARED_APP_DATA.md).

   Every page keeps all its settings in memory and saves all of them. So that
   a save never undoes what another app saved meanwhile, this module remembers
   the shared values the page and the engine last agreed on. With each save it
   sends, for every shared key the page changed, the value the page started
   from; the engine applies that change to what is on disk now and leaves the
   other shared keys as they are on disk.

   Every few seconds, while the window is visible, the page asks whether
   another app changed something and applies it: lists are redrawn, the
   theme, terminal settings and editor preferences take effect at once. A
   change waits while the user has unsaved changes, an open dialog, a menu, a
   drag or text entry it would disturb. */
(function(root){'use strict';
const POLL_MS=2500;
// The engine's table of shared keys (sharedDataKeys in shared_data.go). LIVE
// keys are refreshed in a running page; the others are read when it starts.
const LIVE=['terminalProfiles','terminalRecent','terminalSavedManualOrder','terminalPromptNames','ftpProfiles','ftpRecent','sftpRecent','sftpVerifiedHostKeys','theme','accentColor','colorPickerPreferences','diagnosticColors','terminalAppearance','terminalSyntax','terminalHighlight','terminalBehavior','terminalInteraction','terminalFontSize','terminalMacroConfirmation','terminalActiveActionsPosition','terminalSidebarSide','terminalSessionConfirmDelete','terminalConfirmClose','terminalConfirmReconnectAll','terminalConfirmDisconnectAll','terminalConfirmCloseAll','editorPreferences'];
const AT_START=['resultTimePreferences','editorFilesPreferences','transferExplorerSettings','profiles','savedResultFilters','tftpProfiles','tftpRecent','transferServerSettings','tableColumns','testTabOrder','overviewLayout','graphPlotPreferences','packetMapSettings'];
const SHARED=[...LIVE,...AT_START];
// Applied through the page's own path for display preferences
// (applyTerminalWorkspaceDisplay in index.html).
const DISPLAY=new Set(['theme','colorPickerPreferences','terminalAppearance','terminalSyntax','terminalHighlight','terminalBehavior','terminalInteraction','terminalFontSize','terminalMacroConfirmation','terminalActiveActionsPosition','terminalSidebarSide','terminalSessionConfirmDelete','terminalConfirmClose','terminalConfirmReconnectAll','terminalConfirmDisconnectAll','terminalConfirmCloseAll']);
const LISTS=new Set(['terminalProfiles','terminalRecent','terminalSavedManualOrder','terminalPromptNames','ftpProfiles','ftpRecent','sftpRecent','sftpVerifiedHostKeys']);
let revision='',timer=null,polling=false,stopped=false,listening=false,noticeHandler=null,credentialMark;
// The JSON text of each shared value as last agreed with the engine. Null
// until the page has loaded its settings.
let baseline=null;
const pending=[];

const textOf=value=>JSON.stringify(value);
const short=(value,limit)=>typeof value==='string'?value.slice(0,limit):'';
const list=value=>Array.isArray(value)?value:[];
const plain=value=>value&&typeof value==='object'&&!Array.isArray(value);

// The same clean-up the page applies to these values when it loads its
// settings (mergeState and mergeTerminalPreferences in index.html, and the
// session sidebar), so a refreshed value looks exactly like a loaded one.
function normalize(values,deps={}){
 const source=plain(values)?values:{};
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
 out.ftpRecent=list(source.ftpRecent).filter(x=>x&&typeof x.id==='string'&&typeof x.name==='string'&&typeof x.host==='string'&&typeof x.username==='string'&&Number.isInteger(Number(x.port))&&Number(x.port)>=1&&Number(x.port)<=65535).slice(0,50).map(x=>({id:x.id.slice(0,80),name:x.name.slice(0,100),host:x.host.slice(0,253),port:Number(x.port),username:x.username.slice(0,256),security:['explicit','implicit','plain'].includes(x.security)?x.security:'explicit',fingerprint:short(x.fingerprint,256),credentialId:short(x.credentialId,128),lastUsed:Number.isSafeInteger(x.lastUsed)?x.lastUsed:0}));
 out.sftpRecent=list(source.sftpRecent);
 out.sftpVerifiedHostKeys=plain(source.sftpVerifiedHostKeys)?source.sftpVerifiedHostKeys:{};
 // Settings.
 out.theme=['dark','light'].includes(source.theme)?source.theme:'dark';
 out.accentColor=typeof source.accentColor==='string'&&/^(#[0-9a-f]{6})?$/i.test(source.accentColor)?source.accentColor:'';
 const picker=deps.colorPicker||root.NetworkToolkitColorPicker,appearance=deps.appearance||root.NetworkToolkitTerminalAppearance,syntax=deps.syntax||root.NTKTerminalSyntaxRules,behavior=deps.behavior||root.NetworkToolkitTerminalBehavior,interaction=deps.interaction||root.NetworkToolkitTerminalInteraction,colors=deps.diagnosticColors||root.cleanDiagnosticColors;
 out.colorPickerPreferences=picker?.normalizePreferences?picker.normalizePreferences(source.colorPickerPreferences):source.colorPickerPreferences;
 out.diagnosticColors=typeof colors==='function'?colors(source.diagnosticColors):source.diagnosticColors;
 out.terminalAppearance=appearance?.normalize?appearance.normalize(source.terminalAppearance):source.terminalAppearance;
 if(plain(source.terminalSyntax))out.terminalSyntax=syntax?.preferences?syntax.preferences(source.terminalSyntax):source.terminalSyntax;
 if(plain(source.terminalBehavior))out.terminalBehavior=behavior?.normalize?behavior.normalize(source.terminalBehavior):source.terminalBehavior;
 out.terminalInteraction=interaction?.safePreferences?interaction.safePreferences(source.terminalInteraction):source.terminalInteraction;
 out.terminalHighlight=source.terminalHighlight!==false;
 out.terminalFontSize=Number.isInteger(source.terminalFontSize)&&source.terminalFontSize>=8&&source.terminalFontSize<=36?source.terminalFontSize:13;
 out.terminalMacroConfirmation=['always','multiple','never'].includes(source.terminalMacroConfirmation)?source.terminalMacroConfirmation:'always';
 out.terminalActiveActionsPosition=source.terminalActiveActionsPosition==='bottom'?'bottom':'top';
 out.terminalSidebarSide=source.terminalSidebarSide==='left'?'left':'right';
 for(const key of ['terminalSessionConfirmDelete','terminalConfirmClose','terminalConfirmReconnectAll','terminalConfirmDisconnectAll','terminalConfirmCloseAll'])out[key]=source[key]!==false;
 out.editorPreferences=plain(source.editorPreferences)?source.editorPreferences:{};
 return out;
}

// The page has loaded its settings: this is what it starts from.
function loaded(){
 baseline=new Map();pending.length=0;
 for(const key of SHARED)baseline.set(key,textOf(state[key]));
}

// What a save sends: the page's settings plus, for each shared key the page
// changed, the value it started from.
let untouched=null;
function outgoing(current){
 if(!baseline){pending.push(null);return current;}
 const started={},sent=new Map();
 for(const key of SHARED){
  const now=textOf(current[key]),before=baseline.get(key);
  sent.set(key,now);
  if(now===before)continue;
  // A setting nobody has saved yet, which this page merely filled in with
  // its default, is not a change the user made.
  if(before===undefined){untouched=untouched||normalize({});if(now===textOf(untouched[key]))continue;}
  started[key]=before===undefined?null:JSON.parse(before);
 }
 pending.push(sent);
 return {...current,_sharedBase:started};
}
function failed(){pending.shift();}

const interrupting='#modalRoot .modal,#sessionLibraryContextMenu,.session-saved-dragging';
// Something of the user's is in progress: do not change values under it.
function busy(keys=[]){
 try{
  if(saveTimer!==null||pending.length>0)return true;
  if(stateRevision!==(savedStateRevision<0?0:savedStateRevision))return true;
 }catch{return true;}
 if(document.querySelector(interrupting))return true;
 if(document.body.classList.contains('resizing-columns'))return true;
 const active=typing();
 if(!active)return false;
 if(active.closest('#terminalSessionSidebar,[data-rb-root]'))return true;
 // Applying editor preferences redraws the document being typed in.
 return keys.includes('editorPreferences')&&!!active.closest('#networkToolkitFileEditor,.nt-editor');
}
// The text field the user is typing in, if this window has the keyboard. A
// window in the background is not being typed in, whatever was focused last.
function typing(){
 const active=document.activeElement;
 if(!active||!/^(INPUT|TEXTAREA|SELECT)$/.test(active.tagName))return null;
 if(typeof document.hasFocus==='function'&&!document.hasFocus())return null;
 return active;
}

const attempt=task=>{try{task();}catch(error){console.error('Shared data refresh',error);}};
function redrawLists(){
 attempt(()=>root.NetworkToolkitSavedSessions?.sync?.());
 attempt(()=>{if(document.getElementById('terminalSessionSidebar'))root.NetworkToolkitSessionSidebar?.render?.();});
 attempt(()=>{if(typeof renderTerminalProfiles==='function')renderTerminalProfiles();});
 for(const kind of ['ssh','ftp'])attempt(()=>{if(document.querySelector('[data-rb-root="'+kind+'"]'))root.NetworkToolkitRemoteBrowser?.render?.(kind);});
}

// Put values another app saved into this page and make them take effect.
// Only the keys that differ are touched.
function applyChanged(next,changed){
 const display={};
 for(const key of changed){
  if(DISPLAY.has(key))display[key]=next[key];
  else state[key]=next[key];
 }
 if(Object.keys(display).length)attempt(()=>{
  if(typeof applyTerminalWorkspaceDisplay==='function')applyTerminalWorkspaceDisplay(display);
  else Object.assign(state,display);
  root.NetworkToolkitTerminalAppearance?.sync?.();
 });
 if(changed.includes('theme')||changed.includes('accentColor'))attempt(()=>{if(typeof applyAccentColor==='function')applyAccentColor();if(typeof syncAppAccentControls==='function')syncAppAccentControls();});
 if(changed.includes('diagnosticColors'))attempt(()=>{if(typeof applyDiagnosticColors==='function')applyDiagnosticColors();});
 if(changed.includes('editorPreferences'))attempt(()=>root.NetworkToolkitFileEditor?.windowAdapter?.applyPreferences?.(state.editorPreferences));
 if(changed.some(key=>LISTS.has(key)))redrawLists();
 document.dispatchEvent(new CustomEvent('shared-data-changed',{detail:{keys:changed}}));
}

function differing(next){return LIVE.filter(key=>textOf(state[key])!==textOf(next[key]));}

function apply(values){
 const next=normalize(values),changed=differing(next);
 if(changed.length)applyChanged(next,changed);
 if(baseline)for(const key of LIVE)baseline.set(key,textOf(state[key]));
 return changed;
}

// After a save. sent is the page's change counter when the save was made.
function saved(response,sent){
 const values=pending.shift();
 // The engine has merged what was sent: that is the new starting point.
 if(values&&baseline)for(const [key,text] of values)baseline.set(key,text);
 const shared=response?.sharedData;
 if(!shared||typeof shared.revision!=='string')return;
 if(!shared.values){revision=shared.revision;return;}
 // The engine merged in something this page did not have. If the user has
 // changed anything since, or is in a dialog, the next poll offers it again.
 let current=sent;try{current=stateRevision;}catch{}
 if(current!==sent||saveTimer!==null||pending.length>0||document.querySelector(interrupting))return;
 const next=normalize(shared.values),changed=differing(next);
 if(changed.length&&busyTyping(changed))return;
 if(changed.length)applyChanged(next,changed);
 if(baseline)for(const key of LIVE)baseline.set(key,textOf(state[key]));
 revision=shared.revision;
}
function busyTyping(keys){
 const active=typing();
 return !!(active&&keys.includes('editorPreferences')&&active.closest('#networkToolkitFileEditor,.nt-editor'));
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
  if(Array.isArray(result.notices)&&result.notices.length)attempt(()=>noticeHandler?.(result.notices));
  if(result.credentials!==undefined)credentialsChanged(result.credentials);
  if(!result.values){revision=result.revision;return false;}
  const next=normalize(result.values),changed=differing(next);
  if(busy(changed))return false;
  if(changed.length)applyChanged(next,changed);
  if(baseline)for(const key of LIVE)baseline.set(key,textOf(state[key]));
  revision=result.revision;return true;
 }catch{return false;}
 finally{polling=false;}
}

// Things the one-time move to shared data wants the user to know, such as
// SSH hosts for which two apps had remembered different keys. Shown once, in
// plain words, when nothing else is open; acknowledged notices do not return.
let noticeShown='';
function showNotices(notices){
 const list=notices.filter(n=>n&&typeof n.id==='string'&&Array.isArray(n.items)&&n.items.length).slice(0,8),key=list.map(n=>n.id).join(',');
 if(!list.length||key===noticeShown||busy()||typeof openModal!=='function')return;
 const names={terminal:'Terminal',editor:'Text Editor'},escape=typeof esc==='function'?esc:value=>String(value).replace(/[&<>"']/g,'');
 const parts=list.map(n=>{
  const app=names[n.source]||'other',items='<ul>'+n.items.slice(0,40).map(item=>'<li style="overflow-wrap:anywhere">'+escape(item)+'</li>').join('')+'</ul>';
  if(n.kind==='host-key-conflict')return '<p>For these SSH hosts the '+app+' window had remembered a different host key than the main window. The main window\'s key was kept and nothing else was changed. The next time you connect to one of them, compare the fingerprint that is shown with one you trust before you accept a changed key.</p>'+items;
  return '<p>Not everything the '+app+' window had saved could be added because these lists are full. Nothing was removed; remove entries you no longer need and the rest is added the next time the app starts.</p>'+items;
 });
 noticeShown=key;
 openModal('Saved data is now shared','<p>Network Toolkit, its Terminal and its Text Editor now share saved connections, settings, trusted SSH host keys and macros.</p>'+parts.join(''),'<button class="btn primary" data-shared-notice-ok>OK</button>',true);
 const button=document.querySelector('[data-shared-notice-ok]');
 if(button)button.addEventListener('click',()=>{void api('/shared-data/notices','POST',{acknowledge:list.map(n=>n.id)}).catch(()=>{noticeShown='';});if(typeof closeModal==='function')closeModal();});
}
noticeHandler=showNotices;

// The password manager keeps a list of saved credentials; another app may
// have added one for a connection that is now shown here.
function credentialsChanged(mark){
 if(credentialMark===undefined){credentialMark=mark;return;}
 if(credentialMark===mark)return;
 credentialMark=mark;
 attempt(()=>{void root.NetworkToolkitCredentials?.refresh?.();});
}

function start(){
 stopped=false;
 if(timer!==null||typeof root.setInterval!=='function')return;
 timer=root.setInterval(()=>{void poll();},POLL_MS);timer?.unref?.();
 if(!listening){listening=true;document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='visible')void poll();});}
}
function stop(){stopped=true;if(timer!==null)root.clearInterval(timer);timer=null;}

const api_=Object.freeze({normalize,loaded,outgoing,saved,failed,apply,poll,start,stop,busy,LIVE,AT_START,SHARED,POLL_MS,onNotices(handler){noticeHandler=handler;},get revision(){return revision;}});
root.NetworkToolkitSharedData=api_;
if(typeof module==='object'&&module.exports)module.exports=api_;
if(typeof document!=='undefined')start();
})(typeof window!=='undefined'?window:globalThis);
