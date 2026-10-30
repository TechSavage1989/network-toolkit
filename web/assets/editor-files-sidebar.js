(function(root){
'use strict';
let side='local',panel='files',protocol='ssh',width=300,accessTimer=null,requests=[],sourceBusy=false,navigationRevision=0,preferencesLoaded=false,wideCollapsed=false,compactOpen=false,compactLayout=null;
const detached=()=>typeof EDITOR_WINDOW!=='undefined'&&EDITOR_WINDOW;
function readPreferences(){if(preferencesLoaded)return;preferencesLoaded=true;let v={};if(detached()){try{v=JSON.parse(root.sessionStorage?.getItem('ntk.editor.files')||'{}');}catch{}}else v=typeof state==='undefined'?{}:state.editorFilesPreferences||{};side=v.side==='remote'?'remote':'local';panel=v.panel==='connections'?'connections':'files';width=Number.isFinite(v.width)?Math.max(220,Math.min(600,v.width)):300;wideCollapsed=v.collapsed===true;return v;}
function remember(host){const value={side,panel,width,collapsed:wideCollapsed};if(detached()){try{root.sessionStorage?.setItem('ntk.editor.files',JSON.stringify(value));}catch{}}else if(typeof state!=='undefined'){state.editorFilesPreferences=value;if(typeof saveState==='function')saveState();}}
function setup(editor){
 const host=editor?.querySelector('[data-ed-files-sidebar]');if(!host)return null;
 if(host.querySelector('[data-editor-file-browser]'))return host;
 host.id='ntEditorFilesContent';host.setAttribute('role','tabpanel');
 const shell=document.createElement('div');shell.className='nt-editor-files-shell';host.before(shell);
 shell.innerHTML='<div class="terminal-sidebar-rail"><div class="terminal-sidebar-tabs" role="tablist" aria-label="Editor sidebar" aria-orientation="vertical">'+[['connections','Connect','connect'],['files','Files','folder']].map(([name,label,symbol])=>'<button type="button" class="terminal-sidebar-tab" role="tab" data-editor-files-panel="'+name+'" id="'+(name==='files'?'ntEditorFilesTab':'ntEditorConnectionsTab')+'" aria-label="'+(name==='files'?'Files':'Connections')+'" aria-controls="ntEditorFilesContent">'+icon(symbol)+'<span>'+label+'</span></button>').join('')+'</div></div>';shell.append(host);
 host.innerHTML='<div id="ntEditorFilesBrowse" class="nt-editor-files-locations" role="tablist" aria-label="File locations"><button type="button" class="btn small ghost" role="tab" data-editor-files-side="local">Windows files</button><button type="button" class="btn small ghost" role="tab" data-editor-files-side="remote">SFTP</button></div><div id="ntEditorFilesConnections" data-editor-file-sessions></div><div class="nt-editor-files-body" id="ntEditorFilesBody" data-editor-file-browser></div><div data-editor-file-paths></div><div class="nt-editor-files-resize" role="separator" tabindex="0" aria-label="Resize or hide editor sidebar" aria-controls="ntEditorFilesContent" aria-orientation="vertical" aria-valuemin="220" aria-valuemax="600" title="Drag to resize. Double-click or press Enter or Space to hide. Arrow keys resize; Home or End selects a size limit."></div>';
 const sessions=editor.querySelector('#ntEditorRemotePanel');if(sessions)host.querySelector('[data-editor-file-sessions]').append(sessions);
 const paths=sessions?.querySelector('.nt-editor-file-path-actions');if(paths)host.querySelector('[data-editor-file-paths]').append(paths);
 shell.addEventListener('click',event=>{const primary=event.target.closest('[data-editor-files-panel]'),tab=event.target.closest('[data-editor-files-side]');if(primary){if(primary.dataset.editorFilesPanel===panel&&!host.hidden)toggle(editor);else selectPanel(primary.dataset.editorFilesPanel);return;}if(tab){navigationRevision++;side=tab.dataset.editorFilesSide;if(side==='local'&&protocol!=='ftp')protocol='ssh';mount(host);remember(host);}});
 shell.addEventListener('keydown',event=>{if(event.key==='Escape'&&isCompact()&&!host.hidden){event.preventDefault();event.stopPropagation();toggle(editor);return;}const tab=event.target.closest('[role="tab"]');if(!tab)return;const vertical=tab.parentElement.getAttribute('aria-orientation')==='vertical',back=vertical?'ArrowUp':'ArrowLeft',forward=vertical?'ArrowDown':'ArrowRight';if(![back,forward,'Home','End'].includes(event.key))return;const tabs=[...tab.parentElement.querySelectorAll('[role="tab"]')],at=tabs.indexOf(tab),next=event.key==='Home'?0:event.key==='End'?tabs.length-1:(at+(event.key===back?-1:1)+tabs.length)%tabs.length;event.preventDefault();event.stopPropagation();if(vertical)selectPanel(tabs[next].dataset.editorFilesPanel);else tabs[next].click();tabs[next].focus();});
 const handle=host.querySelector('.nt-editor-files-resize');let drag=null;
 const resize=value=>{width=Math.max(220,Math.min(600,value));shell.style.flexBasis=width+'px';shell.style.width=width+'px';handle.setAttribute('aria-valuenow',String(Math.round(width)));};
 handle.addEventListener('pointerdown',event=>{if(event.button!==0)return;event.preventDefault();drag={id:event.pointerId,left:shell.getBoundingClientRect().left};handle.setPointerCapture(event.pointerId);});
 handle.addEventListener('pointermove',event=>{if(drag?.id===event.pointerId)resize(event.clientX-drag.left);});
 const stop=event=>{if(drag?.id!==event.pointerId)return;drag=null;if(handle.hasPointerCapture(event.pointerId))handle.releasePointerCapture(event.pointerId);remember(host);};handle.addEventListener('pointerup',stop);handle.addEventListener('pointercancel',stop);handle.addEventListener('lostpointercapture',()=>{drag=null;});
 handle.addEventListener('dblclick',()=>toggle(editor));
 handle.addEventListener('keydown',event=>{if(['Enter',' '].includes(event.key)){event.preventDefault();toggle(editor);return;}if(['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();resize(event.key==='Home'?220:event.key==='End'?600:width+(event.key==='ArrowLeft'?-20:20));remember(host);}});resize(width);return host;
}
function syncRail(host){
 const shell=host.closest('.nt-editor-files-shell');shell.classList.toggle('is-collapsed',host.hidden);host.inert=host.hidden;host.setAttribute('aria-labelledby',panel==='connections'?'ntEditorConnectionsTab':'ntEditorFilesTab');
 for(const tab of shell.querySelectorAll('[data-editor-files-panel]')){const selected=tab.dataset.editorFilesPanel===panel;tab.setAttribute('aria-selected',String(selected));tab.setAttribute('aria-expanded',String(selected&&!host.hidden));tab.tabIndex=selected?0:-1;tab.title=(selected&&!host.hidden?'Close ':'Open ')+(tab.dataset.editorFilesPanel==='files'?'Files':'Connections');tab.removeAttribute('data-editor-files-close');tab.removeAttribute('data-editor-files-reopen');if(selected)tab.setAttribute(host.hidden?'data-editor-files-reopen':'data-editor-files-close','');}
}
function visibility(editor,host,show){host.hidden=!show;editor.querySelector('[data-ed="files"]')?.setAttribute('aria-expanded',String(show));syncRail(host);if(!show)clearTimeout(accessTimer);else mount(host);}
function isCompact(){return (root.innerWidth||900)<=680;}
function expandedIntent(show){compactLayout=isCompact();if(compactLayout)compactOpen=show;else wideCollapsed=!show;}
function responsive(editor,host){const compact=isCompact();if(compactLayout!==compact){compactOpen=false;compactLayout=compact;}const show=compact?compactOpen:!wideCollapsed,focused=!show&&host.contains(document.activeElement);if(host.hidden===show)navigationRevision++;visibility(editor,host,show);if(focused)editor.querySelector('[data-editor-files-reopen]')?.focus?.({preventScroll:true});}
function initialize(editor){readPreferences();const host=setup(editor);if(host)responsive(editor,host);}
function toggle(editor){readPreferences();const host=setup(editor);if(!host)return;navigationRevision++;expandedIntent(host.hidden);visibility(editor,host,host.hidden);remember(host);(host.hidden?editor.querySelector('[data-editor-files-reopen]'):editor.querySelector('[data-editor-files-panel][aria-selected="true"]'))?.focus?.({preventScroll:true});}
function selectPanel(value){const host=document.querySelector('[data-ed-files-sidebar]');if(!host)return;navigationRevision++;panel=value==='connections'?'connections':'files';if(host.hidden){expandedIntent(true);visibility(host.closest('#networkToolkitFileEditor'),host,true);}else mount(host);remember(host);}
function mount(host){
 syncRail(host);host.dataset.editorSidebarPanel=panel;host.dataset.editorSidebarSide=side;host.querySelector('[data-editor-file-browser]')?.setAttribute('aria-labelledby',panel==='connections'?'ntEditorConnectionsTab':'ntEditorFilesTab');
 const sessions=host.querySelector('#ntEditorRemotePanel');if(sessions){sessions.hidden=panel!=='connections';if(panel==='connections')void root.NetworkToolkitFileEditor?.refreshConnectedSources?.();}
 const locations=host.querySelector('.nt-editor-files-locations');if(locations)locations.hidden=panel!=='files';
 const remoteTab=host.querySelector('[data-editor-files-side="remote"]');if(remoteTab)remoteTab.textContent=protocol==='ftp'?'FTP / FTPS':'SFTP';
 for(const tab of host.querySelectorAll('[role="tab"]')){const selected=tab.dataset.editorFilesPanel?tab.dataset.editorFilesPanel===panel:tab.dataset.editorFilesSide===side;tab.setAttribute('aria-selected',String(selected));tab.tabIndex=selected?0:-1;tab.classList.toggle('primary',selected);tab.classList.toggle('ghost',!selected);}
 if(detached()){void checkAccess(host);return;}
 root.NetworkToolkitRemoteBrowser.mountSidebar(host.querySelector('[data-editor-file-browser]'),side,protocol,{panel});syncAccessButton();
}
async function checkAccess(host){
 clearTimeout(accessTimer);if(host.hidden)return;
 const body=host.querySelector('[data-editor-file-browser]'),revision=navigationRevision,current=()=>!host.hidden&&revision===navigationRevision&&host.isConnected;
 try{const access=await api('/editor-views/current/explorer');if(!current())return;
  if(access.allowed){root.NetworkToolkitRemoteBrowser.mountSidebar(body,side,protocol,{panel});return;}
  root.NetworkToolkitRemoteBrowser?.unmountSidebar?.(body);
  body.innerHTML='<div class="nt-editor-files-access"><p>'+ (access.pending?'Waiting for approval in the main Network Toolkit window.':'Enable local file management and access to the main window’s existing SFTP connections.')+'</p><button type="button" class="btn small ghost" data-editor-files-request'+(access.pending?' disabled':'')+'>Request access</button><p class="help-line">Your open documents remain available. Connection credentials and terminal control stay in the main window.</p></div>';
  const button=body.querySelector('button');button.onclick=async()=>{try{await api('/editor-views/current/explorer','POST',{});if(current()&&body.contains(button))void checkAccess(host);}catch(error){if(current()&&body.contains(button))body.querySelector('p').textContent=error.message;}};
  if(access.pending)accessTimer=setTimeout(()=>void checkAccess(host),1500);
 }catch(error){if(current())body.textContent=error.message;}
}
function syncAccessButton(){
 const host=document.querySelector('[data-ed-files-sidebar]');if(!host||host.hidden)return;const body=host.querySelector('[data-editor-file-browser]');let button=body?.querySelector('[data-editor-files-manage]');
 if(!requests.length){button?.remove();return;}if(!button&&body){button=document.createElement('button');button.className='btn small ghost';button.dataset.editorFilesManage='';button.textContent='Editor window access';button.onclick=manageAccess;body.append(button);}
}
function syncRequests(rows){
 if(detached())return;requests=Array.isArray(rows)?rows:[];syncAccessButton();
 let banner=document.querySelector('[data-editor-files-access-banner]');
 if(!requests.some(row=>row.pending)){banner?.remove();return;}
 if(!banner){banner=document.createElement('div');banner.className='nt-editor-files-access-banner';banner.dataset.editorFilesAccessBanner='';banner.innerHTML='<span>An editor window requests file explorer access.</span><button type="button" class="btn small primary">Review</button>';banner.querySelector('button').onclick=manageAccess;document.body.append(banner);}
}
function manageAccess(){
 openModal('Editor file explorer access','<p>Allow a separate editor window to browse and manage local files, and use the SFTP connections currently open in the main window. This does not share saved passwords or terminal control. Open editor files remain protected from explorer changes.</p>'+requests.map(row=>'<div class="setting-row"><span>'+esc(row.name||'Editor window')+'</span><div class="row gap6">'+(row.pending?'<button type="button" class="btn small primary" data-editor-files-allow="'+esc(row.viewId)+'">Allow</button>':'')+'<button type="button" class="btn small ghost" data-editor-files-deny="'+esc(row.viewId)+'">'+(row.pending?'Decline':'Revoke')+'</button></div></div>').join(''),'<button type="button" class="btn ghost" data-action="close-modal">Close</button>');
 const modal=document.querySelector('#modalRoot');modal.onclick=async function answer(event){const button=event.target.closest('[data-editor-files-allow],[data-editor-files-deny]');if(!button)return;const id=button.dataset.editorFilesAllow||button.dataset.editorFilesDeny,row=requests.find(row=>row.viewId===id);if(!row)return;button.disabled=true;try{await api('/editor-views/explorer/'+id,'POST',{requestId:row.requestId,allow:!!button.dataset.editorFilesAllow});modal.onclick=null;closeModal();syncRequests(requests.filter(item=>item!==row));}catch(error){button.disabled=false;toast(error.message,'error');}};
}
function showConnection(kind){if(detached()||!['ssh','ftp'].includes(kind))throw Error('Open saved connections in the main window.');const editor=document.querySelector('#networkToolkitFileEditor'),host=editor?.querySelector('[data-ed-files-sidebar]');if(!editor||editor.hidden||!host)throw Error('Open the editor before connecting a saved profile.');navigationRevision++;protocol=kind;side='remote';panel='connections';if(host.hidden)toggle(editor);else mount(host);remember(host);}
async function useSource(source){
 if(sourceBusy)throw Error('Finish the current file connection action first.');
 const browser=root.NetworkToolkitRemoteBrowser;if(!browser)throw Error('The file browser is unavailable.');
 if(!/^(client:(ssh|ftp)|terminal:.+)$/.test(source||''))throw Error('Choose an active file connection first.');
 const kind=source==='client:ftp'?'ftp':'ssh';if(detached()&&kind!=='ssh')throw Error('This editor may use approved SFTP connections only.');
 const editor=document.querySelector('#networkToolkitFileEditor'),host=editor?.querySelector('[data-ed-files-sidebar]');if(!host||host.hidden||editor.hidden)throw Error('Open the editor Files sidebar first.');
 const revision=navigationRevision;sourceBusy=true;try{
 const model=browser.get(kind);if(source.startsWith('client:')){if(!model.connected)throw Error('This file connection has closed. Refresh the list.');}
 else{const id=source.slice(9);if(model.connecting||model.busy||model.pendingClose)throw Error('Finish the current file operation first.');if(model.connected&&model.sessionId!==id)throw Error('Disconnect the current file browser before choosing another connection. Open documents remain unchanged.');if(!model.connected)await browser.connectTerminal(kind,id);if(!model.connected||model.sessionId!==id)throw Error('The SSH connection is no longer available. Refresh the list.');}
 if(revision!==navigationRevision||host.hidden||editor.hidden||!host.isConnected)return;
 const selected=document.querySelector('#ntEditorRemoteSource');if(selected&&selected.value!==source)return;
 protocol=kind;side='remote';panel='files';mount(host);remember(host);
 }finally{sourceBusy=false;}
}
root.addEventListener?.('resize',()=>{if(!preferencesLoaded)return;const editor=document.querySelector('#networkToolkitFileEditor'),host=editor?.querySelector('[data-ed-files-sidebar]');if(editor&&!editor.hidden&&host&&isCompact()!==compactLayout)responsive(editor,host);});
root.NetworkToolkitEditorFiles=Object.freeze({initialize,selectPanel,useSource,showConnection,toggle,syncRequests,manageAccess,detached});
})(globalThis);
