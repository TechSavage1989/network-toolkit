/* Session-scoped SFTP sidebar. Availability is checked on a separate SFTP
   subsystem channel over the already authenticated SSH terminal connection.
   No saved credentials or terminal shell data are accessed by this module. */
(function(root){
'use strict';
const sessions=new Map();
let contextMenu=null,clipboard=null;
const byId=id=>document.getElementById(id);
const safeName=name=>typeof name==='string'&&name&&name!=='.'&&name!=='..'&&name.length<=255&&!/[\\/\0\r\n]/.test(name);
const parent=path=>{const p=String(path||'.').replace(/\/$/,'');if(p==='/'||p==='.')return p;const at=p.lastIndexOf('/');return at<0?'.':at===0?'/':p.slice(0,at)};
const join=(path,name)=>String(path||'.').replace(/\/$/,'')+'/'+name;
const escaped=x=>esc(String(x??''));
const shownBytes=n=>{if(!Number.isFinite(Number(n)))return '';const v=Number(n);return v>=1048576?(v/1048576).toFixed(1)+' MB':v>=1024?Math.round(v/1024)+' KB':v+' B'};
const active=()=>typeof currentTerminal==='function'?currentTerminal():null;
const eligible=entry=>!!(entry?.info?.id&&entry.info.type==='ssh'&&entry.info.state==='connected'&&typeof terminalOwnerCanMutate==='function'&&terminalOwnerCanMutate(entry));
function identity(entry){return entry.info.id+'|'+entry.info.host+'|'+entry.info.port+'|'+terminalSessionRevision(entry.info.id)}
function current(entry){if(!eligible(entry))return null;const existing=sessions.get(entry.info.id);return existing&&existing.identity===identity(entry)?existing:null}
function notify(){root.NetworkToolkitSessionSidebar?.render?.()}
function sync(entry){
 if(!eligible(entry))return false;
 const id=entry.info.id,key=identity(entry);
 let m=sessions.get(id);
 if(m&&m.identity===key)return m.status==='supported';
 m={id,identity:key,status:'checking',folder:'.',home:'',entries:[],selected:'',filter:'',loading:false,busy:false,error:'',loaded:false};
 sessions.set(id,m);
 // A timeout or failed SFTP handshake simply leaves the Files tab absent.
 // The terminal remains usable regardless of this result.
 void api('/remote-browser','POST',{protocol:'ssh',action:'probe',sessionId:id}).then(result=>{
  if(sessions.get(id)!==m)return;
  m.status=result.supported===true?'supported':'unsupported';
  // Load the directory only after the user opens the SFTP sidebar tab.
 }).catch(()=>{if(sessions.get(id)===m)m.status='unsupported';}).finally(()=>{
  if(sessions.get(id)===m)notify();
 });
 return false;
}
function available(entry){return current(entry)?.status==='supported';}
async function request(id,action,extra={}){
 const m=sessions.get(id);if(!m||m.status!=='supported')throw Error('This SSH session does not support SFTP browsing.');
 return api('/remote-browser','POST',{protocol:'ssh',sessionId:id,action,...extra});
}
function host(){return byId('terminalSidebarSFTP')}
function message(text,fail=false){const out=host()?.querySelector('[data-tsftp-note]');if(!out)return;out.textContent=text||'';out.hidden=!text;out.classList.toggle('error',fail);}
function paintSelection(m){
 const area=host();if(!area||!m)return;
 for(const row of area.querySelectorAll('[data-tsftp-item]')){
  const selected=m.selected===row.dataset.tsftpItem;
  row.classList.toggle('selected',selected);
  row.setAttribute('aria-pressed',String(selected));
 }
 for(const button of area.querySelectorAll('[data-tsftp-needs-file]'))button.disabled=!m.selected||m.busy;
}
function paint(){
 const area=host(),entry=active(),m=current(entry);
 if(!area||!m||area.dataset.tsftpSession!==m.id)return;
 const label=area.querySelector('[data-tsftp-address]');if(label&&document.activeElement!==label)label.value=m.folder;
 const list=area.querySelector('[data-tsftp-list]');if(!list)return;
 const values=m.entries.filter(item=>!m.filter||item.name.toLowerCase().includes(m.filter.toLowerCase()));
 const html=m.loading?'<p class="tsftp-empty">Loading folder...</p>':values.length?values.map(item=>'<button type="button" class="tsftp-item'+(m.selected===item.path?' selected':'')+'" data-tsftp-item="'+escaped(item.path)+'" draggable="true" aria-pressed="'+(m.selected===item.path)+'" title="'+escaped(item.path)+'"><span class="tsftp-file-icon">'+icon(item.directory?'folder':'file')+'</span><span class="tsftp-name">'+escaped(item.name)+'</span><small>'+escaped(item.directory?'':shownBytes(item.size))+'</small></button>').join(''):'<p class="tsftp-empty">'+(m.filter?'No matching files.':'This folder is empty.')+'</p>';
 if(list._tsftpHTML!==html){list.innerHTML=html;list._tsftpHTML=html;}
 const status=area.querySelector('[data-tsftp-status]');if(status)status.textContent=m.busy?'Working...':m.entries.length+' item'+(m.entries.length===1?'':'s');
 for(const btn of area.querySelectorAll('[data-tsftp-needs-file]'))btn.disabled=!m.selected||m.busy;
 for(const btn of area.querySelectorAll('[data-tsftp-needs-free]'))btn.disabled=m.busy;
 message(m.error||'',!!m.error);
}
async function load(id,location){
 const m=sessions.get(id);if(!m||m.status!=='supported'||m.loading)return;
 m.loading=true;m.error='';paint();
 try{const data=await request(id,'list',{path:location||'.'});if(sessions.get(id)!==m)return;
  m.folder=data.path||location||'.';if(!m.home)m.home=m.folder;m.entries=(data.entries||[]).filter(x=>safeName(x.name)).map(x=>({name:x.name,path:x.path,directory:!!x.directory,size:x.size}));
  m.entries.sort((a,b)=>Number(b.directory)-Number(a.directory)||a.name.localeCompare(b.name,undefined,{numeric:true,sensitivity:'base'}));
  m.selected='';m.loaded=true;
 }catch(error){if(sessions.get(id)===m){m.error=error.message;m.loaded=true;}}
 finally{if(sessions.get(id)===m){m.loading=false;paint();}}
}
function render(panel,entry){
 if(!panel)return;
 const m=current(entry);
 if(!m||m.status!=='supported'){panel.innerHTML='';delete panel.dataset.tsftpSession;return;}
 if(panel.dataset.tsftpSession!==m.id){
  panel.dataset.tsftpSession=m.id;
  panel.innerHTML='<div class="tsftp-browser" aria-label="SFTP browser for '+escaped(entry.info.label||entry.info.host)+'">'+
  '<div class="tsftp-connection" title="'+escaped(entry.info.host)+'"><strong>'+escaped(entry.info.label||entry.info.host)+'</strong><small>'+escaped((entry.info.username||'')+'@'+(entry.info.host||''))+'</small></div>'+
  '<div class="tsftp-toolbar"><button class="btn small ghost" data-tsftp="textEditor" title="Open this connection in the Text Editor">'+icon('fileEdit')+' Editor</button><button class="btn small ghost" data-tsftp="up" title="Parent folder">'+icon('arrowUp')+'</button><button class="btn small ghost" data-tsftp="home" title="Home directory" aria-label="Home directory">'+icon('home')+'</button><button class="btn small ghost" data-tsftp="root" title="Root directory" aria-label="Root directory">/</button><button class="btn small ghost" data-tsftp="refresh" title="Refresh files">'+icon('refresh')+'</button><button class="btn small ghost" data-tsftp="new" data-tsftp-needs-free>+ Folder</button><button class="btn small ghost" data-tsftp="full" title="Open full two-pane SFTP browser">Expand</button><button class="btn small ghost" data-tsftp="uploadWindows" title="Upload files dropped from Windows">From Windows</button><input type="file" data-tsftp-os-files multiple hidden></div>'+
  '<form class="tsftp-path-form" data-tsftp-path-form><input data-tsftp-address type="text" spellcheck="false" aria-label="Remote SFTP folder" placeholder="Remote folder"><button type="submit" class="btn small ghost">Go</button></form>'+
  '<input type="search" class="tsftp-filter" data-tsftp-filter placeholder="Filter remote files" aria-label="Filter SFTP files">'+
  '<div data-tsftp-note class="tsftp-notice" hidden></div><div class="tsftp-file-list" data-tsftp-list role="group" aria-label="Remote files"></div>'+
  '<div class="tsftp-file-tools"><button class="btn small ghost" data-tsftp="upload" data-tsftp-needs-free>Upload</button><button class="btn small ghost" data-tsftp="download" data-tsftp-needs-file>Download</button><button class="btn small ghost" data-tsftp="rename" data-tsftp-needs-file>Rename</button><button class="btn small ghost danger" data-tsftp="delete" data-tsftp-needs-file>Delete</button><button class="btn small ghost" data-tsftp="edit" data-tsftp-needs-file>Edit</button><button class="btn small ghost" data-tsftp="newFile" data-tsftp-needs-free>+ File</button><button class="btn small ghost" data-tsftp="preview" data-tsftp-needs-file>Preview</button></div><div class="tsftp-footer"><span data-tsftp-status>Ready</span><span>Drag files to Windows Explorer</span></div></div>';
  const filter=panel.querySelector('[data-tsftp-filter]');if(filter)filter.value=m.filter;
 }
 paint();
 if(!m.loaded&&!m.loading)void load(m.id,m.folder);
}
async function editor(create=false){const m=current(active()),entry=m?.entries.find(x=>x.path===m.selected);if(!m)return;if(!create&&(!entry||entry.directory)){message('Select one text file.');return;}
 const filename=create?prompt('New UTF-8 text file name:','new-file.txt'):entry.name;if(filename===null)return;if(!safeName(filename)){message('Enter a valid file name.',true);return;}
 const path=create?join(m.folder,filename):entry.path;
 try{await root.NetworkToolkitFileEditor.open({source:'SSH terminal',path,id:'terminal:'+m.id+':'+path,create,
  read:async path=>request(m.id,'read-text',{path}),write:async data=>request(m.id,'write-text',data),afterSave:()=>load(m.id,m.folder)});
 }catch(e){message('Cannot edit: '+e.message,true);}}
async function osUpload(m,files){if(!files.length||m.busy)return;m.busy=true;paint();let done=0,failed=0;
 try{for(const file of files){if(!safeName(file.name)){failed++;continue;}
  const form=new FormData();form.append('file',file,file.name);form.append('connection',JSON.stringify({protocol:'ssh',sessionId:m.id,path:m.folder,overwrite:false}));
  try{const resp=await fetch('/api/remote-browser/drop-upload',{method:'POST',headers:{'X-Session-Token':SESSION,'X-Terminal-Revision':String(terminalSessionRevision(m.id))},body:form});const answer=await resp.json();if(!resp.ok)throw Error(answer.error||'Upload failed');done++;}catch(e){failed++;m.error=e.message;}}
 }finally{m.busy=false;message(done+' uploaded'+(failed?', '+failed+' failed':''),!!failed);void load(m.id,m.folder);}}
async function recursiveDelete(m,file,tracker){if(++tracker.count>1500)throw Error('Maximum 1,500 items per delete operation');if(!file.directory){await request(m.id,'remove',{path:file.path,confirm:true});return;}
 if(file.path==='/'||file.path==='.')throw Error('Deleting remote root is not allowed');
 const children=await request(m.id,'list',{path:file.path});for(const child of children.entries||[]){if(!safeName(child.name))throw Error('Unsafe directory entry');await recursiveDelete(m,child,tracker);}
 await request(m.id,'rmdir',{path:file.path,confirm:true});}
async function recursiveRemoteCopy(m,source,target,tracker){
 if(++tracker.count>1500)throw Error('Maximum 1,500 items per folder copy');
 if(!source.directory){await request(m.id,'copy',{path:source.path,newPath:target});return;}
 if(target===source.path||target.startsWith(source.path.replace(/\/$/,'')+'/'))throw Error('Cannot copy a folder inside itself');
 await request(m.id,'mkdir',{path:target});
 const result=await request(m.id,'list',{path:source.path});
 for(const child of result.entries||[]){if(!safeName(child.name))throw Error('Unsafe directory entry');await recursiveRemoteCopy(m,child,join(target,child.name),tracker);}
}
const localJoin=(base,name)=>String(base).replace(/[\\/]$/,'')+(String(base).includes('\\')?'\\':'/')+name;
async function recursiveDownload(m,source,localDestination,tracker){
 if(++tracker.count>1500)throw Error('Maximum 1,500 items per folder download');
 if(!source.directory){await request(m.id,'download',{path:source.path,localPath:localDestination,overwrite:false});return;}
 await api('/transfer/local-files','POST',{action:'mkdir',path:localDestination});
 const children=await request(m.id,'list',{path:source.path});
 for(const child of children.entries||[]){if(!safeName(child.name))throw Error('Unsafe directory entry');await recursiveDownload(m,child,localJoin(localDestination,child.name),tracker);}
}
async function paste(m){if(!clipboard||!clipboard.path)return;
 if(clipboard.sessionId&&clipboard.sessionId!==m.id){message('Copying across SSH sessions is not supported. Use the full file browser.',true);return;}
 const name=clipboard.path.split('/').pop();if(!safeName(name))return;const target=join(m.folder,name);
 if(target===clipboard.path){message('Choose a different destination folder.',true);return;}
 m.busy=true;paint();
 try{if(clipboard.move)await request(m.id,'rename',{path:clipboard.path,newPath:target});
  else await recursiveRemoteCopy(m,{path:clipboard.path,directory:!!clipboard.directory},target,{count:0});
  if(clipboard.move)clipboard=null;void load(m.id,m.folder);
 }catch(e){message(e.message,true);}finally{m.busy=false;paint();}
}
function contextMenuAt(m,file,event){if(contextMenu)contextMenu.remove();contextMenu=document.createElement('div');contextMenu.className='tsftp-context-menu';contextMenu.innerHTML=(file?(file.directory?'<button data-tsftp-menu="open">Open folder</button>':'<button data-tsftp-menu="edit">Edit text file</button><button data-tsftp-menu="preview">Preview</button>')+'<button data-tsftp-menu="download">Download</button><button data-tsftp-menu="rename">Rename</button><button data-tsftp-menu="copy">Copy</button><button data-tsftp-menu="cut">Cut / Move</button><button data-tsftp-menu="permissions">Permissions</button><button data-tsftp-menu="delete">Delete</button><button data-tsftp-menu="copyPath">Copy path</button>':'')+'<button data-tsftp-menu="paste">Paste here</button><button data-tsftp-menu="new">New folder</button><button data-tsftp-menu="newFile">New text file</button><button data-tsftp-menu="uploadWindows">Upload from Windows</button><button data-tsftp-menu="refresh">Refresh</button>';
 document.body.append(contextMenu);contextMenu.style.left=Math.max(4,Math.min(innerWidth-190,event.clientX))+'px';contextMenu.style.top=Math.max(4,Math.min(innerHeight-contextMenu.offsetHeight-8,event.clientY))+'px';}
function operationDialog(action){
 const entry=active(),m=current(entry);if(!m||m.busy)return;
 const selected=m.entries.find(x=>x.path===m.selected),name=selected?.name||'';
 let title='',body='',label='Apply';
 if(action==='new'){title='Create remote folder';label='Create folder';body='<label>Folder name<input id="tsftpDialogValue" required maxlength="255" autocomplete="off" placeholder="New folder"></label>';}
 if(action==='rename'&&selected){title='Rename remote item';label='Rename';body='<label>New name<input id="tsftpDialogValue" required maxlength="255" value="'+escaped(name)+'"></label>';}
 if(action==='delete'&&selected){title='Delete remote item';label='Delete';body='<p>Delete <strong>'+escaped(name)+'</strong> permanently?'+(selected.directory?' This includes all files and subfolders.':'')+'</p><label class="tsftp-check"><input type="checkbox" id="tsftpDeleteConfirm"> I confirm permanent deletion</label>';}
 if(action==='upload'){title='Upload to '+m.folder;label='Upload';body='<label>Path to local file<input id="tsftpDialogValue" required placeholder="C:\\Users\\...\\file.txt" autocomplete="off"></label><label class="tsftp-check"><input type="checkbox" id="tsftpOverwrite"> Overwrite existing remote file</label>';}
 if(action==='download'&&selected){title='Download '+name;label='Download';body=selected.directory?'<label>Destination parent folder on this computer<input id="tsftpDialogValue" required placeholder="C:\\Users\\Public\\Downloads" autocomplete="off"></label><p>Creates a folder named '+escaped(name)+' and downloads its contents. Existing files are not overwritten.</p>':'<label>Save to this local file path<input id="tsftpDialogValue" required placeholder="C:\\Users\\...\\'+escaped(name)+'" autocomplete="off"></label><label class="tsftp-check"><input type="checkbox" id="tsftpOverwrite"> Overwrite existing local file</label>'; }
 if(!body){toast('Select a file for this action.','info');return;}
 openModal(title,'<form id="tsftpOperationForm">'+body+'</form>','<button class="btn ghost" data-action="close-modal">Cancel</button><button type="submit" form="tsftpOperationForm" class="btn '+(action==='delete'?'danger':'primary')+'">'+label+'</button>',true);
 const form=byId('tsftpOperationForm');if(form){form.dataset.tsftpOperation=action;form.dataset.tsftpSession=m.id;form.dataset.tsftpSelected=m.selected;}
}
async function apply(form){
 const id=form.dataset.tsftpSession,m=sessions.get(id),action=form.dataset.tsftpOperation,selected=m?.entries.find(x=>x.path===form.dataset.tsftpSelected);
 if(!m||m.status!=='supported'||m.busy||current(active())!==m)return;
 const value=String(byId('tsftpDialogValue')?.value||'').trim(),overwrite=!!byId('tsftpOverwrite')?.checked;
 let cmd='',extra={};
 if(action==='new'){if(!safeName(value)){toast('Enter a valid folder name.','error');return;}cmd='mkdir';extra.path=join(m.folder,value);}
 if(action==='rename'){if(!selected||!safeName(value)){toast('Enter a valid new name.','error');return;}cmd='rename';extra={path:selected.path,newPath:join(parent(selected.path),value)};}
 if(action==='delete'){if(!selected||!byId('tsftpDeleteConfirm')?.checked){toast('Confirm the permanent deletion.','error');return;}cmd=selected.directory?'recursive-delete':'remove';extra={path:selected.path,confirm:true};}
 if(action==='upload'){if(!value||value.length>4096){toast('Enter a valid local path.','error');return;}const name=value.split(/[\\/]/).pop();if(!safeName(name)){toast('Invalid local file name.','error');return;}cmd='upload';extra={path:join(m.folder,name),localPath:value,overwrite};}
 if(action==='download'){if(!selected||!value||value.length>4096){toast('Select a file/folder and local destination.','error');return;}
  if(selected.directory){cmd='download-folder';extra={destination:localJoin(value,selected.name)};}
  else {cmd='download';extra={path:selected.path,localPath:value,overwrite};}}

 if(!cmd)return;
 closeModal();m.busy=true;m.error='';paint();
 try{if(cmd==='recursive-delete')await recursiveDelete(m,selected,{count:0});else if(cmd==='download-folder')await recursiveDownload(m,selected,extra.destination,{count:0});else await request(id,cmd,extra);toast('SFTP '+cmd+' completed.','success');await load(id,m.folder);}
 catch(error){m.error=error.message;toast('SFTP '+cmd+': '+error.message,'error');}
 finally{m.busy=false;paint();}
}
async function preview(){
 const entry=active(),m=current(entry),selected=m?.entries.find(x=>x.path===m.selected);
 if(!selected||selected.directory){toast('Choose a file to preview.','info');return;}
 try{const response=await request(m.id,'preview',{path:selected.path});if(response.binary){toast('Binary files cannot be previewed.','info');return;}
  openModal('Preview '+selected.name,'<pre class="tsftp-preview">'+escaped(response.content||'')+'</pre>','<button class="btn primary" data-action="close-modal">Close</button>',true);
 }catch(error){message(error.message,true);}
}
function command(name){
 const entry=active(),m=current(entry);if(!m||m.status!=='supported')return;
 if(name==='up')void load(m.id,parent(m.folder));
 if(name==='home')void load(m.id,m.home||'.');
 if(name==='root')void load(m.id,'/');
 if(name==='refresh')void load(m.id,m.folder);
 if(name==='full'){
  root.NetworkToolkitRemoteBrowser?.useTerminalSession(m.id);
  state.transferModes=state.transferModes||{};state.transferModes.sftp='client';navigate('sftp');saveState();
 }
 if(name==='preview')void preview();
 if(name==='textEditor'){root.NetworkToolkitFileEditor?.preferConnection?.('terminal:'+m.id,m.selected&&!(m.entries.find(row=>row.path===m.selected)?.directory)?m.selected:'');if(typeof navigate==='function')navigate('editor');return;}
 if(name==='edit')void editor();
 if(name==='newFile')void editor(true);
 if(name==='uploadWindows')host()?.querySelector('[data-tsftp-os-files]')?.click();
 if(name==='copy'||name==='cut'){const file=m.entries.find(x=>x.path===m.selected);if(file){clipboard={path:file.path,directory:!!file.directory,sessionId:m.id,move:name==='cut'};toast('Ready to paste '+file.name,'info');}}
 if(name==='paste')void paste(m);
 if(name==='copyPath'){const file=m.entries.find(x=>x.path===m.selected);void copyText(file?.path||m.folder);}
 if(name==='open'){const file=m.entries.find(x=>x.path===m.selected);if(file?.directory)void load(m.id,file.path);}
 if(name==='permissions'){const file=m.entries.find(x=>x.path===m.selected);if(!file)return;const value=prompt('SFTP permissions (octal, e.g. 0644 or 0755):',file.directory?'0755':'0644');if(value===null)return;if(!/^[0-7]{3,4}$/.test(value)){message('Invalid octal permissions',true);return;}void request(m.id,'chmod',{path:file.path,permissions:value}).then(()=>load(m.id,m.folder)).catch(e=>message(e.message,true));}

 if(['upload','download','new','rename','delete'].includes(name))operationDialog(name);
}
document.addEventListener('click',event=>{
 const cm=event.target.closest?.('[data-tsftp-menu]');if(cm&&contextMenu){event.preventDefault();event.stopImmediatePropagation();const cmd=cm.dataset.tsftpMenu;contextMenu.remove();contextMenu=null;command(cmd);return;}
 if(contextMenu&&!event.target.closest?.('.tsftp-context-menu')){contextMenu.remove();contextMenu=null;}
 const item=event.target.closest?.('[data-tsftp-item]');if(item){
  const m=current(active());if(!m||m.busy)return;event.preventDefault();event.stopImmediatePropagation();m.selected=item.dataset.tsftpItem;paintSelection(m);return;
 }
 const button=event.target.closest?.('[data-tsftp]');if(!button)return;
 if(!button.closest('#terminalSidebarSFTP'))return;
 event.preventDefault();event.stopImmediatePropagation();if(!button.disabled)command(button.dataset.tsftp);
},true);
document.addEventListener('dblclick',event=>{
 const row=event.target.closest?.('[data-tsftp-item]');if(!row)return;
 const m=current(active()),file=m?.entries.find(x=>x.path===row.dataset.tsftpItem);if(!file)return;
 event.preventDefault();event.stopImmediatePropagation();m.selected=file.path;
 if(file.directory)void load(m.id,file.path);else void editor();
},true);
document.addEventListener('input',event=>{
 if(!event.target.matches?.('[data-tsftp-filter]'))return;
 const m=current(active());if(m){m.filter=event.target.value;paint();}
},true);
document.addEventListener('submit',event=>{
 if(event.target.matches?.('[data-tsftp-path-form]')){event.preventDefault();event.stopImmediatePropagation();const m=current(active());const path=event.target.querySelector('[data-tsftp-address]')?.value.trim();if(m)void load(m.id,path||'.');return;}
 if(event.target.id==='tsftpOperationForm'){event.preventDefault();event.stopImmediatePropagation();void apply(event.target);}
},true);
document.addEventListener('contextmenu',event=>{const row=event.target.closest?.('[data-tsftp-item]'),list=event.target.closest?.('[data-tsftp-list]');if(!list)return;const m=current(active());if(!m)return;event.preventDefault();event.stopImmediatePropagation();if(row)m.selected=row.dataset.tsftpItem;const file=m.entries.find(x=>x.path===m.selected);paint();contextMenuAt(m,file,event);},true);
document.addEventListener('change',event=>{const input=event.target.closest?.('[data-tsftp-os-files]');if(!input)return;const m=current(active());if(m&&input.files?.length)void osUpload(m,[...input.files]);input.value='';},true);
document.addEventListener('pointerover',event=>{const row=event.target.closest?.('#terminalSidebarSFTP [data-tsftp-item]');if(!row)return;const m=current(active()),item=m?.entries.find(x=>x.path===row.dataset.tsftpItem);if(!m||!item||item.directory)return;
 const key='terminal:'+m.identity+':'+item.path;
 root.NetworkToolkitNativeDrag?.hover?.(row,key,()=>({protocol:'ssh',sessionId:m.id,action:'stat',path:item.path}));
},true);
document.addEventListener('dragstart',event=>{const row=event.target.closest?.('[data-tsftp-item]');if(!row)return;const m=current(active()),file=m?.entries.find(x=>x.path===row.dataset.tsftpItem);if(!file)return;
 event.dataTransfer?.setData('application/x-networktoolkit-sidebar-sftp',JSON.stringify({sessionId:m.id,files:[file]}));event.dataTransfer.effectAllowed='copy';if(!file.directory)root.NetworkToolkitNativeDrag?.apply(event,'terminal:'+m.identity+':'+file.path);},true);
document.addEventListener('dragover',event=>{const area=event.target.closest?.('#terminalSidebarSFTP .tsftp-file-list');if(!area)return;event.preventDefault();area.classList.add('tsftp-drop-target');},true);
document.addEventListener('dragleave',event=>{event.target.closest?.('#terminalSidebarSFTP .tsftp-file-list')?.classList.remove('tsftp-drop-target');},true);
document.addEventListener('drop',event=>{const area=event.target.closest?.('#terminalSidebarSFTP .tsftp-file-list');if(!area)return;event.preventDefault();area.classList.remove('tsftp-drop-target');const m=current(active());if(!m)return;const dt=event.dataTransfer;
 if(dt.files?.length){void osUpload(m,[...dt.files]);return;}
 const payload=dt.getData('application/x-networktoolkit-remote-browser');if(payload){try{const x=JSON.parse(payload);if(x.side==='local'){void Promise.all((x.files||[]).map(file=>{if(file.directory)throw Error('Drag individual files from the local pane');return request(m.id,'upload',{path:join(m.folder,file.name),localPath:file.path,overwrite:false});})).then(()=>load(m.id,m.folder)).catch(e=>message(e.message,true));}}catch(e){message(e.message,true);}return;}
 const source=dt.getData('application/x-networktoolkit-sidebar-sftp');if(source){try{const data=JSON.parse(source);const file=data.files?.[0];if(file&&data.sessionId===m.id){clipboard={path:file.path,directory:!!file.directory,sessionId:m.id,move:false};void paste(m);}}catch(e){message(e.message,true);}}
},true);
root.NetworkToolkitTerminalSFTP=Object.freeze({sync,available,render,load,request,editor,osUpload});
})(globalThis);
