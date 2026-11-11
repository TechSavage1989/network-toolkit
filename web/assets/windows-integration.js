/* Windows integration of the main window (App settings), docs/guide/editor.md
 * and docs/guide/settings-and-updates.md:
 * - Editor: "Use this copy's Text Editor in Windows". Register writes Setup's
 *   per-user registrations for this copy, Remove takes back what it wrote
 *   (cmd/network-toolkit/text_editor_registration.go). Replacing another
 *   copy's registration needs a second click. The installed program's
 *   registration is never changed from here.
 * - Updates: "Install the full version" (portable copy only). Find Setup of
 *   this version, download it after the user agreed, verified against
 *   SHA256SUMS, then start it after a second confirmation (full_install.go).
 * Shown only in the Windows main window: the standalone Text Editor and
 * Terminal have no App settings, and the Linux edition has no such section.
 * Nothing here writes anything without a click; the startup check only reads,
 * and offers to clean up a registration whose program was moved or deleted. */
(function(root){
'use strict';
const esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const bytes=n=>{n=Math.max(0,Number(n)||0);return n<1048576?Math.max(1,Math.round(n/1024))+' KiB':(n/1048576).toFixed(1)+' MiB';};

// The Text Editor section, from the status the engine reports.
function editorHTML(view){
 const data=view.data,s=data?.textEditor,busy=view.busy,dis=busy?' disabled':'';
 let body='<h4>Use this copy\'s Text Editor in Windows</h4><p class="help-line">Lets Windows offer this copy\'s Text Editor the way the installed program does: under <strong>Open with</strong> for every file, in <strong>Settings &gt; Apps &gt; Default apps</strong> for text, configuration, script and source files, and, if you want, as <strong>Edit with Network Toolkit Text Editor</strong> when you right-click a file. Only your Windows account is changed and no administrator approval is needed. Windows never makes it the default by itself: you choose in Windows.</p>';
 if(!data)return body+'<p class="help-line" role="status">'+esc(view.error||'Checking…')+'</p>';
 if(!s||data.error&&!s.state)return body+'<div class="notice warn" role="alert">'+esc(data.error||'The registration could not be read.')+'</div>';
 body+='<p class="help-line" id="windowsEditorStatus" role="status" data-state="'+esc(s.state)+'">'+esc(s.message)+'</p>';
 if(s.state==='this'&&s.openWithElsewhere)body+='<p class="help-line">Open with for files of other types still starts '+esc(s.openWithElsewhere)+', which another program registered; the listed types, Default apps and the right-click entry use this copy.</p>';
 if(s.canRegister&&!s.needsTakeover||s.state==='this')body+='<label class="toggle-row"><input type="checkbox" data-windows-editor-menu'+(view.menu??s.contextMenu?' checked':'')+dis+'><span><strong>Right-click menu: Edit with Network Toolkit Text Editor</strong><p>Adds the entry for every file. On Windows 11 it is under Show more options.</p></span></label>';
 const buttons=[];
 if(s.canRegister&&!s.needsTakeover)buttons.push('<button type="button" class="btn small" data-windows-editor="register"'+dis+'>'+(s.state==='this'?'Register again':'Register')+'</button>');
 if(s.canRegister&&s.needsTakeover&&!view.confirmTakeover)buttons.push('<button type="button" class="btn small" data-windows-editor="takeover"'+dis+'>Use this copy instead…</button>');
 if(s.canRemove)buttons.push('<button type="button" class="btn small ghost" data-windows-editor="remove"'+dis+'>'+(s.state==='this'?'Remove':'Clean up')+'</button>');
 if(buttons.length)body+='<div class="button-row">'+buttons.join('')+'</div>';
 if(view.confirmTakeover)body+='<div class="mt" data-windows-editor-confirm><div class="notice info"><span>Replace the registration of '+esc(s.owner||'the other copy')+' with this copy? Windows will open files with this copy from now on, and the other copy shows that it is no longer registered.</span></div><label class="toggle-row"><input type="checkbox" data-windows-editor-menu'+(view.menu?' checked':'')+dis+'><span><strong>Right-click menu: Edit with Network Toolkit Text Editor</strong></span></label><div class="button-row"><button type="button" class="btn small primary" data-windows-editor="takeover-confirm"'+dis+'>Replace it</button><button type="button" class="btn small ghost" data-windows-editor="takeover-cancel"'+dis+'>Cancel</button></div></div>';
 if(s.state==='this')body+='<p class="help-line">Before you move or delete this folder, choose Remove. Otherwise Windows keeps pointing at the missing program and says it cannot find the app; another copy can then clean up the registration. Running Setup takes the registration over for the installed program.</p>';
 if(view.message)body+='<p class="help-line" role="status">'+esc(view.message)+'</p>';
 if(view.error)body+='<div class="notice warn mt" role="alert">'+esc(view.error)+'</div>';
 return body;
}

// The install section, for a portable copy.
function installHTML(view){
 const data=view.data,i=data?.install,busy=view.busy||i?.busy,dis=busy?' disabled':'';
 if(!data||!i||!i.portable)return '';
 let body='<h4>Install the full version</h4><p class="help-line">The installed version adds Start menu entries for Network Toolkit, its Text Editor and its Terminal, lets Windows offer the Text Editor for files and the Terminal for SSH, Telnet and serial links, and comes with an uninstaller. It uses the same data folder, so your settings and saved data stay as they are. This portable copy is left where it is; you can close or delete it after installing.</p>';
 if(i.data?.note)body+='<p class="help-line" data-windows-install-data="'+(i.data.same?'same':'different')+'">'+esc(i.data.note)+'</p>';
 if(!i.supported)return body+'<p class="help-line">Setup runs on Windows x64.</p>';
 if(i.installed&&!view.reinstall){
  body+='<div class="notice info" data-windows-install-installed><span>Network Toolkit is already installed'+(i.installLocation?' at '+esc(i.installLocation):'')+'.'+(i.installedExists?'':' Its program file is missing; run Setup to repair it.')+'</span></div><div class="button-row">'+(i.installedExists?'<button type="button" class="btn small primary" data-windows-install="open-installed"'+dis+'>Open the installed program</button>':'')+'<button type="button" class="btn small ghost" data-windows-install="reinstall"'+dis+'>Reinstall or repair anyway</button></div>';
  return body+statusLines(view);
 }
 const found=i.found,download=i.download,p=i.progress;
 if(!download){
  if(!found)body+='<div class="button-row"><button type="button" class="btn small" data-windows-install="find"'+dis+'>Find Setup for version '+esc(i.version)+'</button></div>';
  else{
   body+=found.fallback?'<div class="notice warn" data-windows-install-fallback><span>Setup for version '+esc(i.version)+' is not on the release page. The latest '+(found.beta?'beta':'stable')+' release, '+esc(found.version)+', is offered instead. Installing it also updates the program.</span></div>':'<p class="help-line">Found '+esc(found.name)+' ('+bytes(found.size)+') on the official release page.</p>';
   if(view.confirmDownload)body+='<div data-windows-install-confirm="download"><div class="notice info"><span>Download '+esc(found.name)+' ('+bytes(found.size)+') from the official release page and check it against the release\'s SHA256SUMS? Nothing is installed yet; Setup starts only when you choose to start it.</span></div><div class="button-row"><button type="button" class="btn small primary" data-windows-install="download-confirm" data-version="'+esc(found.version)+'"'+dis+'>Download and verify</button><button type="button" class="btn small ghost" data-windows-install="cancel"'+dis+'>Cancel</button></div></div>';
   else body+='<div class="button-row"><button type="button" class="btn small" data-windows-install="download"'+dis+'>Download Setup ('+bytes(found.size)+')…</button><button type="button" class="btn small ghost" data-windows-install="find"'+dis+'>Look again</button></div>';
  }
 }else{
  body+='<p class="help-line">'+esc(download.name)+' ('+bytes(download.size)+') was downloaded and matches the release\'s SHA256SUMS.</p><p class="help-line" style="overflow-wrap:anywhere">SHA-256: '+esc(download.sha256)+'</p>';
  if(i.started)body+='<div class="notice info" data-windows-install-started><span>Setup is running in its own window. When it has finished, open Network Toolkit from the Start menu. You can then close this portable copy and delete its folder; your settings and saved data stay in the data folder.</span></div>';
  else if(view.confirmStart)body+='<div data-windows-install-confirm="start"><div class="notice info"><span>Start Network Toolkit Setup '+esc(download.version)+' now? It opens in its own window and asks before it installs anything. This copy keeps running.</span></div><div class="button-row"><button type="button" class="btn small primary" data-windows-install="start-confirm" data-version="'+esc(download.version)+'"'+dis+'>Start Setup</button><button type="button" class="btn small ghost" data-windows-install="cancel"'+dis+'>Cancel</button></div></div>';
  else body+='<div class="button-row"><button type="button" class="btn small primary" data-windows-install="start"'+dis+'>Start Setup…</button><button type="button" class="btn small ghost" data-windows-install="discard"'+dis+'>Delete the download</button></div>';
 }
 if(p&&(p.phase==='checksums'||p.phase==='download'||p.phase==='verifying')){
  const total=Number(p.total)||0,received=Number(p.received)||0,known=p.phase==='download'&&total>0&&received<=total;
  const label={checksums:'Preparing download',download:'Downloading Setup',verifying:'Verifying Setup'}[p.phase];
  body+='<div class="app-update-progress" data-windows-install-progress><div class="row between gap6"><span>'+label+'</span>'+(p.phase==='download'?'<span>'+(known?bytes(received)+' of '+bytes(total)+' ('+Math.floor(received/total*100)+'%)':bytes(received)+' received')+'</span>':'')+'</div><progress aria-label="'+label+'"'+(known?' value="'+received+'" max="'+total+'"':'')+'></progress></div>';
 }
 return body+statusLines(view);
}
function statusLines(view){return (view.message?'<p class="help-line" role="status">'+esc(view.message)+'</p>':'')+(view.error?'<div class="notice warn mt" role="alert">'+esc(view.error)+'</div>':'');}

if(typeof module==='object'&&module.exports){module.exports={editorHTML,installHTML};return;}
const document=root.document;if(!document)return;
// The page's globals (index.html): status, api, state, toast.
const mainWindow=()=>{try{return typeof status!=='undefined'&&!!status&&status.os==='windows'&&status.windowMode==='main';}catch{return false;}};
const call=(path,method,body)=>typeof api==='function'?api(path,method,body):Promise.reject(new Error('The app is not ready.'));
const channel=()=>{try{return typeof state!=='undefined'&&state?.appUpdatePreferences?.beta===true?'beta':'stable';}catch{return 'stable';}};
const editorView={data:null,busy:false,error:'',message:'',menu:null,confirmTakeover:false};
const installView={busy:false,error:'',message:'',reinstall:false,confirmDownload:false,confirmStart:false};
let shared=null;
// Both sections stay hidden until the engine says it supports them (the
// Windows main window); a read that fails is shown in the Editor section.
function render(){
 installView.data=editorView.data=shared;
 const supported=shared?.supported===true;
 const e=document.getElementById('windowsTextEditorSettings');if(e){e.innerHTML=supported||editorView.error?editorHTML(editorView):'';e.hidden=!e.innerHTML;}
 const i=document.getElementById('windowsFullInstall');if(i){i.innerHTML=supported?installHTML(installView):'';i.hidden=!i.innerHTML;}
 document.querySelector('[data-windows-editor-confirm],[data-windows-install-confirm]')?.scrollIntoView?.({block:'nearest'});
}
async function refresh(){
 try{shared=await call('/windows-integration');}
 catch(error){if(shared?.supported===true)editorView.error='The Windows registration could not be read: '+(error.message||error);}
 render();return shared;
}
function inject(){
 if(!mainWindow())return;
 const editor=document.querySelector('[data-settings-panel="editor"]'),updates=document.querySelector('[data-settings-panel="updates"]');
 let added=false;
 if(editor&&!editor.querySelector('#windowsTextEditorSettings')){const block=document.createElement('section');block.id='windowsTextEditorSettings';block.className='data-section';const heading=editor.querySelector(':scope>h3');if(heading)heading.after(block);else editor.prepend(block);added=true;}
 if(updates&&!updates.querySelector('#windowsFullInstall')){const block=document.createElement('section');block.id='windowsFullInstall';block.className='data-section';updates.append(block);added=true;}
 if(added){editorView.message='';editorView.error='';installView.message='';installView.error='';render();void refresh();}
}
async function editorAction(action){
 const menu=document.querySelector('[data-windows-editor-menu]');if(menu)editorView.menu=menu.checked;
 if(action==='takeover'){editorView.confirmTakeover=true;editorView.menu=!!shared?.textEditor?.contextMenu;render();return;}
 if(action==='takeover-cancel'){editorView.confirmTakeover=false;render();return;}
 editorView.busy=true;editorView.error='';editorView.message='';render();
 try{
  if(action==='remove'){const result=await call('/windows-integration/text-editor/remove','POST',{});shared={...shared,textEditor:result.textEditor};editorView.message=result.textEditor?.state==='none'?'Removed. Windows no longer offers this copy\'s Text Editor.':'Removed what this registration had written.';}
  else{const result=await call('/windows-integration/text-editor/register','POST',{contextMenu:editorView.menu===true,takeover:action==='takeover-confirm'});shared={...shared,textEditor:result.textEditor};editorView.message='Registered. Choose Network Toolkit Text Editor in Open with or in Settings > Apps > Default apps to use it.';}
  editorView.confirmTakeover=false;
 }catch(error){editorView.error=error.message||String(error);}
 finally{editorView.busy=false;editorView.menu=null;await refresh();}
}
let progressTimer=null;
function watchProgress(on){
 if(progressTimer){root.clearTimeout(progressTimer);progressTimer=null;}
 if(!on)return;
 const tick=async()=>{try{const data=await call('/windows-integration');if(installView.busy){shared=data;render();}}catch{}if(installView.busy)progressTimer=root.setTimeout(tick,400);};
 progressTimer=root.setTimeout(tick,400);
}
async function installAction(action,version){
 if(action==='reinstall'){installView.reinstall=true;render();return;}
 if(action==='download'){installView.confirmDownload=true;render();return;}
 if(action==='start'){installView.confirmStart=true;render();return;}
 if(action==='cancel'){installView.confirmDownload=installView.confirmStart=false;render();return;}
 installView.busy=true;installView.error='';installView.message='';render();
 try{
  let result;
  if(action==='find'){installView.message='Looking for Setup on the official release page…';render();result=await call('/windows-integration/full-install/find','POST',{channel:channel()});installView.message='';}
  else if(action==='download-confirm'){installView.confirmDownload=false;installView.message='Downloading and verifying Setup. Nothing is installed.';watchProgress(true);result=await call('/windows-integration/full-install/download','POST',{version,confirm:true});installView.message='Setup was downloaded and verified.';}
  else if(action==='start-confirm'){installView.confirmStart=false;result=await call('/windows-integration/full-install/start','POST',{version,confirm:true});installView.message='';}
  else if(action==='discard'){result=await call('/windows-integration/full-install/discard','POST',{});installView.message='The downloaded Setup was deleted.';}
  else if(action==='open-installed'){await call('/windows-integration/open-installed','POST',{confirm:true});installView.message='The installed program is starting. You can close this portable copy.';}
  if(result?.install)shared={...shared,install:result.install};
 }catch(error){installView.error=(action==='download-confirm'?'Setup was not downloaded: ':action==='find'?'Setup could not be found: ':'')+(error.message||String(error));installView.message='';}
 finally{installView.busy=false;watchProgress(false);await refresh();}
}
document.addEventListener('click',event=>{
 const editor=event.target.closest?.('[data-windows-editor]');
 if(editor&&!editor.disabled){event.preventDefault();void editorAction(editor.dataset.windowsEditor);return;}
 const install=event.target.closest?.('[data-windows-install]');
 if(install&&!install.disabled){event.preventDefault();void installAction(install.dataset.windowsInstall,install.dataset.version);return;}
 const clean=event.target.closest?.('[data-windows-cleanup]');
 if(clean){event.preventDefault();const toast=clean.closest('.toast');if(clean.dataset.windowsCleanup==='later'){toast?.remove();return;}
  clean.disabled=true;call('/windows-integration/text-editor/remove','POST',{}).then(result=>{shared={...shared,textEditor:result.textEditor};toast?.remove();notify('The registration of the missing program was removed.');render();},error=>{toast?.remove();notify('The registration could not be cleaned up: '+(error.message||error),'error');});}
},true);
document.addEventListener('change',event=>{if(event.target.matches?.('[data-windows-editor-menu]'))editorView.menu=event.target.checked;});
function notify(message,kind){try{if(typeof toast==='function')toast(message,kind);}catch{}}
// Startup: read only. A registration that names a program that no longer
// exists is offered for clean up once per run, as a notice that never covers
// a dialog.
let checked=false;
async function startupCheck(attempt=0){
 if(checked)return;
 if(!mainWindow()){if(attempt<60)root.setTimeout(()=>startupCheck(attempt+1),1000);return;}
 checked=true;
 let data;try{data=await call('/windows-integration');}catch{return;}
 shared=data;const s=data?.textEditor;
 if(!s||!(s.state==='other'||s.state==='unrecorded')||!s.ownerMissing)return;
 const host=document.getElementById('toastStack');if(!host)return;
 const box=document.createElement('div');box.className='toast info';box.id='windowsEditorCleanup';box.setAttribute('role','status');
 box.innerHTML='<span>Windows still offers a Network Toolkit Text Editor at '+esc(s.owner||'a missing location')+', which no longer exists. Clean up that registration?</span><button type="button" class="btn small" data-windows-cleanup="now">Clean up</button><button type="button" class="btn small ghost" data-windows-cleanup="later">Not now</button>';
 host.append(box);
}
const watch=()=>{const target=document.getElementById('modalRoot');if(!target){root.setTimeout(watch,500);return;}new MutationObserver(()=>inject()).observe(target,{childList:true,subtree:true});};watch();
root.addEventListener('load',()=>root.setTimeout(()=>startupCheck(),1500));
root.NetworkToolkitWindowsIntegration=Object.freeze({inject,refresh,startupCheck,editorHTML,installHTML});
})(typeof window==='object'?window:globalThis);
