(function(root){
'use strict';
let side='local',protocol='ssh',width=300,accessTimer=null,requests=[];
const detached=()=>typeof EDITOR_WINDOW!=='undefined'&&EDITOR_WINDOW;
function toggle(editor){
 const host=editor?.querySelector('[data-ed-files-sidebar]'),button=editor?.querySelector('[data-ed="files"]');if(!host)return;
 const show=host.hidden;host.hidden=!show;button?.setAttribute('aria-expanded',String(show));
 if(!show){clearTimeout(accessTimer);root.NetworkToolkitRemoteBrowser?.unmountSidebar(host.querySelector('[data-editor-file-browser]'));return;}
 if(!host.querySelector('[data-editor-file-browser]')){
  host.innerHTML='<div class="nt-editor-files-heading"><div role="tablist" aria-label="File locations"><button type="button" class="btn small ghost" role="tab" data-editor-files-side="local">Windows files</button><button type="button" class="btn small ghost" role="tab" data-editor-files-side="remote">SFTP</button></div><button type="button" class="btn small ghost" data-editor-files-close aria-label="Close file browser">×</button></div><div class="nt-editor-files-body" data-editor-file-browser></div><div class="nt-editor-files-resize" role="separator" tabindex="0" aria-label="Resize file browser" aria-orientation="vertical"></div>';
  host.addEventListener('click',event=>{if(event.target.closest('[data-editor-files-close]')){toggle(editor);return;}const tab=event.target.closest('[data-editor-files-side]');if(tab){side=tab.dataset.editorFilesSide;if(side==='local')protocol='ssh';mount(host);}});
  const handle=host.querySelector('.nt-editor-files-resize');let drag=null;
  const resize=value=>{width=Math.max(220,Math.min(600,editor.clientWidth*.65,value));host.style.flexBasis=width+'px';host.style.width=width+'px';handle.setAttribute('aria-valuenow',String(Math.round(width)));};
  handle.addEventListener('pointerdown',event=>{if(event.button!==0)return;event.preventDefault();drag={id:event.pointerId,left:host.getBoundingClientRect().left};handle.setPointerCapture(event.pointerId);});
  handle.addEventListener('pointermove',event=>{if(drag?.id===event.pointerId)resize(event.clientX-drag.left);});
  const stop=event=>{if(drag?.id!==event.pointerId)return;drag=null;if(handle.hasPointerCapture(event.pointerId))handle.releasePointerCapture(event.pointerId);};handle.addEventListener('pointerup',stop);handle.addEventListener('pointercancel',stop);handle.addEventListener('lostpointercapture',()=>{drag=null;});
  handle.addEventListener('keydown',event=>{if(['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();resize(event.key==='Home'?220:event.key==='End'?600:width+(event.key==='ArrowLeft'?-20:20));}});
  resize(width);
 }
 mount(host);
}
function mount(host){
 const remoteTab=host.querySelector('[data-editor-files-side="remote"]');if(remoteTab)remoteTab.textContent=protocol==='ftp'?'FTP / FTPS':'SFTP';
 for(const tab of host.querySelectorAll('[data-editor-files-side]')){const selected=tab.dataset.editorFilesSide===side;tab.setAttribute('aria-selected',String(selected));tab.classList.toggle('primary',selected);tab.classList.toggle('ghost',!selected);}
 if(detached()){void checkAccess(host);return;}
 root.NetworkToolkitRemoteBrowser.mountSidebar(host.querySelector('[data-editor-file-browser]'),side,protocol);syncAccessButton();
}
async function checkAccess(host){
 clearTimeout(accessTimer);if(host.hidden)return;
 const body=host.querySelector('[data-editor-file-browser]');
 try{const access=await api('/editor-views/current/explorer');if(host.hidden)return;
  if(access.allowed){root.NetworkToolkitRemoteBrowser.mountSidebar(body,side);return;}
  body.innerHTML='<div class="nt-editor-files-access"><p>'+ (access.pending?'Waiting for approval in the main Network Toolkit window.':'Enable local file management and access to the main window’s existing SFTP connections.')+'</p><button type="button" class="btn small ghost" data-editor-files-request'+(access.pending?' disabled':'')+'>Request access</button><p class="help-line">Your open documents remain available. Connection credentials and terminal control stay in the main window.</p></div>';
  body.querySelector('button').onclick=async()=>{try{await api('/editor-views/current/explorer','POST',{});void checkAccess(host);}catch(error){body.querySelector('p').textContent=error.message;}};
  if(access.pending)accessTimer=setTimeout(()=>void checkAccess(host),1500);
 }catch(error){body.textContent=error.message;}
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
function showConnection(kind){if(detached()||!['ssh','ftp'].includes(kind))throw Error('Open saved connections in the main window.');const editor=document.querySelector('#networkToolkitFileEditor'),host=editor?.querySelector('[data-ed-files-sidebar]');if(!editor||editor.hidden||!host)throw Error('Open the editor before connecting a saved profile.');protocol=kind;side='remote';if(host.hidden)toggle(editor);else mount(host);host.querySelector('.nt-editor-files-connection')?.setAttribute('open','');}
root.NetworkToolkitEditorFiles=Object.freeze({showConnection,toggle,syncRequests,manageAccess,detached});
})(globalThis);
