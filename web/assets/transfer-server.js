(function(){'use strict';
const protocols=['sftp','tftp','ftp'];
const current={sftp:null,tftp:null,ftp:null};
const folders={sftp:'',tftp:'',ftp:''};
const cache={sftp:null,tftp:null,ftp:null};
// The native Windows host suppresses browser alert/confirm/prompt dialogs. All
// server confirmations and validation therefore use the app's own UI.
const pending={sftp:false,tftp:false,ftp:false};
const notes={sftp:null,tftp:null,ftp:null};
const expanded={sftp:true,tftp:true,ftp:true};
let activityObserver=null;
// Never persist the server password in state or job history. Keep a transient
// copy only while editing settings so an unrelated redraw cannot erase it.
const sessionPasswords={sftp:'',ftp:''};
const config={sftp:{bind:'127.0.0.1',port:2222,root:'',access:'readwrite',overwrite:false,username:'',authorizedKeys:''},tftp:{bind:'127.0.0.1',port:69,root:'',access:'readwrite',overwrite:false},ftp:{bind:'127.0.0.1',port:2121,root:'',access:'readwrite',overwrite:false,username:'',security:'explicit'}};
const ui={mode:function(p){return state.transferModes?.[p]==='client'?'client':'server'},
bindItems:function(p,selected){
 const seen=new Set(['127.0.0.1','0.0.0.0']);
 const items=[['127.0.0.1','This computer only - Loopback (127.0.0.1)'],['0.0.0.0','All network adapters (0.0.0.0)']];
 for(const adapter of (typeof interfaces==='function'?interfaces():[])){
  if(!adapter.up)continue;
  for(const raw of (typeof addresses==='function'?addresses(adapter):[])){
   const ip=String(raw).split('/')[0];
   if(!/^\d{1,3}(?:\.\d{1,3}){3}$/.test(ip)||seen.has(ip))continue;
   seen.add(ip);
   const name=typeof adapterLabel==='function'?adapterLabel(adapter):adapter.name||adapter.description||'Network adapter';
   items.push([ip,name+' | '+ip]);
  }
 }
 if(selected&&!seen.has(selected))items.push([selected,'Unavailable adapter address: '+selected]);
 return items;
},
bindPicker:function(p,selected,locked){
 const items=this.bindItems(p,selected),label=items.find(x=>x[0]===selected)?.[1]||selected;
 // Use the same native control as client connection sources. Its popup and
 // keyboard selection do not expand or replace the server settings layout.
 return '<select id="transferBind-'+p+'" class="connection-source-select transfer-bind-picker" data-transfer-setting="bind" data-transfer-proto="'+p+'" aria-describedby="transferBindHint-'+p+'" title="'+esc(label)+'"'+locked+'>'+items.map(([ip,description])=>'<option value="'+esc(ip)+'"'+(ip===selected?' selected':'')+'>'+esc(description)+'</option>').join('')+'</select>';
},tabs:function(p){
 return '<div class="transfer-mode-tabs row gap6" role="group" aria-label="'+p.toUpperCase()+' operation mode">'+
 '<button type="button" class="btn small '+(this.mode(p)==='server'?'primary':'ghost')+'" data-transfer-mode="server" data-transfer-proto="'+p+'">Server</button>'+
 '<button type="button" class="btn small '+(this.mode(p)==='client'?'primary':'ghost')+'" data-transfer-mode="client" data-transfer-proto="'+p+'">Client</button></div>';
},
render:function(p){
 if(!protocols.includes(p))return;
 if(!config[p]._restored){
  const saved=state.transferServerSettings?.[p];
  if(saved&&typeof saved==='object')for(const k of ['bind','port','root','access','overwrite','username','authorizedKeys','security'])if(Object.hasOwn(config[p],k)&&Object.hasOwn(saved,k))config[p][k]=saved[k];
  config[p]._restored=true;
 }
 if(this.mode(p)==='client')return;
 const profile=config[p],status=current[p],running=!!status?.running;
 const root=running?(status.root||profile.root||''):(profile.root||status?.root||'');
 const bind=running?(status.bind||profile.bind||'127.0.0.1'):(profile.bind||'127.0.0.1');
 const port=running?(status.port||profile.port):(profile.port||(p==='tftp'?69:p==='ftp'?2121:2222));
 const address=running?(status.address||bind+':'+port):(bind+':'+port+' (not listening)');
 const access=running?(status.access||profile.access):profile.access;
 const readAccess=access==='read'?'Read only':access==='write'?'Write only':'Read and write';
 const locked=running?' disabled':'';
 const notice=notes[p];
 const statusMarkup=`<div class="transfer-server-summary" aria-label="Server status and shared folder">
  <div class="transfer-root-summary"><small>Server root folder</small><div class="transfer-root-display"><strong class="mono" data-transfer-current-root title="${esc(root)}">${esc(root||'Not selected')}</strong>${root?`<button type="button" class="btn small ghost" data-copy="${esc(root)}" title="Copy server root path" aria-label="Copy server root path">${icon('copy')}</button>`:''}</div></div>
  <div><small>Listening address</small><strong class="mono" data-transfer-current-bind title="${esc(address)}">${esc(address)}</strong></div>
  <div><small>Remote permissions</small><strong data-transfer-current-access>${esc(readAccess)}</strong></div>
  <div><small>Active sessions / transfers</small><strong data-transfer-current-active>${Number(status?.active||0)}</strong></div>
 </div>
 <div class="transfer-server-feedback ${notice?.severity==='success'?'good':notice?.severity==='error'?'bad':''}" id="transferServerFeedback" role="status" aria-live="polite" ${notice?'':'hidden'}>${notice?esc(notice.message):''}</div>
 ${bind==='127.0.0.1'?'<p class="transfer-server-hint">Loopback selected. Devices elsewhere on the network cannot connect. Choose a LAN adapter in Server settings.</p>':''}
 ${p==='tftp'?'<p class="transfer-server-hint">TFTP uses UDP without encryption or authentication. The server opens additional transfer ports. Use trusted networks only.</p>':''}${p==='ftp'?'<p class="transfer-server-hint">Plain FTP is unencrypted. Explicit FTPS protects the control and data channels. Firewall rules must also permit passive data ports.</p>':''}
 ${running&&status.fingerprint?`<div class="transfer-server-fingerprint">${p==='ftp'?'FTPS certificate fingerprint:':'SSH server fingerprint:'} <span class="mono">${esc(status.fingerprint)}</span> <button class="btn small ghost" data-copy="${esc(status.fingerprint)}">Copy</button></div>`:''}`;
 const settings=`<div class="transfer-server-settings" id="transferServerSettings">
  <div class="field transfer-interface-field"><label for="transferBind-${p}">Listen on adapter</label>${this.bindPicker(p,bind,locked)}</div>
  <div class="field transfer-port-field"><label>Listening port</label><input type="number" min="1" max="65535" data-transfer-setting="port" data-transfer-proto="${p}" value="${esc(port)}"${locked}></div>
  <p id="transferBindHint-${p}" class="transfer-bind-hint help-line">Choose the LAN IP reachable by your network equipment. Loopback is local-only. All adapters expose the service on all available interfaces.</p>
  <div class="field wide" data-transfer-root-field><label>Server root folder</label><input data-transfer-setting="root" data-transfer-proto="${p}" placeholder="Absolute local folder path" value="${esc(root)}"${locked}>
   <div class="transfer-root-actions"><button type="button" class="btn small ghost" data-transfer-root-browse="${p}"${locked}>${icon('file')}Browse folders</button><button type="button" class="btn small ghost" data-transfer-open-files="${p}">${icon('file')}Browse shared files</button></div></div>
  <div class="field"><label>Remote access permissions</label><select data-transfer-setting="access" data-transfer-proto="${p}"${locked}><option value="read" ${access==='read'?'selected':''}>Read only (devices download)</option><option value="write" ${access==='write'?'selected':''}>Write only (devices upload)</option><option value="readwrite" ${access==='readwrite'?'selected':''}>Read and write</option></select></div>
  <label class="toggle-row"><input type="checkbox" data-transfer-setting="overwrite" data-transfer-proto="${p}" ${profile.overwrite?'checked':''}${locked}><span><strong>Allow overwriting files</strong><p>Off by default for backup protection.</p></span></label>
  ${p==='ftp'?`<div class="field"><label>Security</label><select data-transfer-setting="security" data-transfer-proto="ftp"${locked}><option value="explicit" ${profile.security==='explicit'?'selected':''}>Explicit FTPS (TLS, recommended)</option><option value="plain" ${profile.security==='plain'?'selected':''}>Plain FTP (unencrypted)</option></select></div>
  <div class="field"><label>FTP username</label><input data-transfer-setting="username" data-transfer-proto="ftp" autocomplete="off" value="${esc(profile.username)}"${locked}></div><div class="field"><label>Session password</label><input type="password" id="ftpServerPassword" autocomplete="new-password"${locked}></div>`:''}
  ${p==='sftp'?`<div class="field"><label>SSH username</label><input data-transfer-setting="username" data-transfer-proto="sftp" autocomplete="off" value="${esc(profile.username)}"${locked}></div>
  <div class="field"><label>Session password</label><input type="password" id="sftpServerPassword" placeholder="Required unless authorized keys are configured" autocomplete="new-password"${locked}></div>
  <div class="field wide"><label>Authorized SSH public keys file (optional)</label><input data-transfer-setting="authorizedKeys" data-transfer-proto="sftp" value="${esc(profile.authorizedKeys)}" placeholder="Absolute path to authorized_keys file"${locked}></div>`:''}
 </div><p class="help-line">${running?'The server is listening. Stop it before changing network settings, credentials, permissions, or the root folder.':'The server starts only when requested and stops when Network Toolkit closes. Windows Firewall may require an inbound rule.'}</p>`;
 $('#pageContent').innerHTML=`<div class="transfer-server-app">
  <section class="panel transfer-server-card"><div class="row between wrap gap6"><div><h2>${p.toUpperCase()} server</h2><p class="help-line">${p==='sftp'?'Authenticated SFTP server. Remote shell access is disabled.':p==='ftp'?'FTP / explicit FTPS file server with passive data connections.':'TFTP configuration backups and firmware distribution.'}</p></div>
  <div class="row gap6"><span class="pill ${running?'good':''}" data-transfer-state>${running?'Listening':'Stopped'}</span>
  <button type="button" class="btn small ghost" data-transfer-refresh="${p}" data-transfer-proto="${p}">${icon('refresh')}Refresh</button>
  ${running?`<button type="button" class="btn small danger" data-transfer-stop="${p}" data-transfer-proto="${p}">${icon('stop')}Stop server</button>`:`<button type="button" class="btn small primary" data-transfer-start="${p}" data-transfer-proto="${p}" ${pending[p]?'disabled':''}>${icon('play')}${pending[p]?'Starting...':'Start server'}</button>`}
  </div></div>${statusMarkup}<details class="fm-config-toggle" ${expanded[p]?'open':''} data-transfer-settings="${p}"><summary>Server settings <span>Interface, credentials, permissions and shared folder</span></summary>${settings}</details></section>
  <section class="panel transfer-file-manager transfer-compact-manager"><div class="row between wrap gap6"><div><h2>Shared files</h2><p class="help-line">Quick access to the folder exposed by this server. Use Windows File Explorer for advanced local file management.</p></div><div class="row gap6">
  <button type="button" class="btn small ghost" data-transfer-open-root="${p}" data-transfer-proto="${p}">${icon('folder')}Open in Explorer</button>
  <button type="button" class="btn small ghost" data-transfer-proto="${p}" data-transfer-file="mkdir">${icon('plus')}New folder</button>
  <button type="button" class="btn small ghost" data-transfer-proto="${p}" data-transfer-file="upload">${icon('upload')}Add file</button>
  <button type="button" class="btn small ghost" data-transfer-proto="${p}" data-transfer-file="refresh">${icon('refresh')}Refresh</button></div></div>
  <div class="transfer-manager-path"><button type="button" class="btn small ghost" data-transfer-proto="${p}" data-transfer-file="parent" title="Parent folder">${icon('arrowUp')}Up</button><strong class="mono" id="transferBrowserPath">${esc(root)}</strong><input id="transferBrowserFilter" placeholder="Filter filenames" aria-label="Filter shared files"></div>
  <div class="transfer-file-scroller" id="transferBrowserItems"><p class="muted">Loading shared files...</p></div></section>
  <section class="panel transfer-log-panel"><div class="row between gap6 wrap"><h2>Live server activity <span id="transferLogCount">${status?.transfers?.length||0}</span></h2><div class="row gap6"><button type="button" class="btn small ghost" data-transfer-copy-log="${p}">${icon('copy')}Copy output</button><button type="button" class="btn small ghost" data-transfer-clear-log="${p}">${icon('trash')}Clear</button><button type="button" class="btn small ghost" data-transfer-refresh="${p}" data-transfer-proto="${p}">${icon('refresh')}Refresh</button></div></div><div class="transfer-history transfer-console" id="transferLogItems" role="log" aria-label="${p.toUpperCase()} server activity">${this.logs(status)}</div></section>
 </div>`;
 this.mountSidebar(p);
 this.mountActivityResize(p);
 if(p==='sftp'){const pass=$('#sftpServerPassword');if(pass&&!running)pass.value=sessionPasswords.sftp;}if(p==='ftp'){const pass=$('#ftpServerPassword');if(pass&&!running)pass.value=sessionPasswords.ftp;}
 if(status?.root===root&&root)void this.files(p,folders[p]);
 if(online)void this.refresh(p,false);
},
feedback:function(p,message,severity='error'){
 notes[p]=message?{message,severity}:null;
 if(state.activePage!==p||this.mode(p)!=='server')return;
 const area=$('#transferServerFeedback');if(area){area.hidden=!message;area.textContent=message||'';area.classList.toggle('bad',severity==='error');area.classList.toggle('good',severity==='success');}
},
syncSummary:function(p){
 if(state.activePage!==p||this.mode(p)!=='server')return;
 const profile=config[p],status=current[p],running=!!status?.running,root=running?(status.root||profile.root):(profile.root||status?.root||'');
 const label=$('[data-transfer-current-root]');if(label){label.textContent=root||'Not selected';label.title=root||'';}
 const rootCopy=$('.transfer-root-display [data-copy]');if(rootCopy){rootCopy.dataset.copy=root||'';rootCopy.hidden=!root;}
 const bind=running?(status.address||profile.bind+':'+profile.port):((profile.bind||'127.0.0.1')+':'+profile.port+' (not listening)');
 const addr=$('[data-transfer-current-bind]');if(addr){addr.textContent=bind;addr.title=bind;}
 const permissions=$('[data-transfer-current-access]');if(permissions){const a=running?status.access:profile.access;permissions.textContent=a==='read'?'Read only':a==='write'?'Write only':'Read and write';}
 const active=$('[data-transfer-current-active]');if(active)active.textContent=String(status?.active||0);
},
sidebarState:function(p){
 state.transferServerSidebar=state.transferServerSidebar||{};
 const s=state.transferServerSidebar[p];
 if(!s||typeof s!=='object')state.transferServerSidebar[p]={collapsed:(document.documentElement?.clientWidth||1024)<760,width:320};
 const settings=state.transferServerSidebar[p];
 settings.width=Math.max(270,Math.min(460,Number(settings.width)||320));
 settings.collapsed=settings.collapsed===true;
 const height=Number(settings.activityHeight);
 // Zero selects a balanced default; explicit user sizes survive redraws and
 // smaller windows without losing their original preferred height.
 settings.activityHeight=Number.isFinite(height)&&height>=180?Math.min(1200,height):0;
 return settings;
},
mountSidebar:function(p){
 const app=document.querySelector('.transfer-server-app'),card=app?.querySelector('.transfer-server-card'),settings=card?.querySelector('[data-transfer-settings]');
 if(!app||!card||!settings)return;
 const saved=this.sidebarState(p),browser=app.querySelector('.transfer-file-manager'),logs=app.querySelector('.transfer-log-panel');
 const sidebar=document.createElement('aside');
 sidebar.className='transfer-server-config-panel panel';sidebar.dataset.transferSidebar=p;
 sidebar.setAttribute('aria-label',p.toUpperCase()+' server configuration');
 sidebar.innerHTML='<div class="transfer-server-sidebar-rail"><button type="button" class="panel-visibility-toggle transfer-server-sidebar-button" data-transfer-sidebar-toggle="'+p+'" data-transfer-proto="'+p+'" aria-controls="transferServerSidebarBody" title="Show or hide server settings">'+icon('settings')+'<span>Settings</span></button></div><div class="transfer-server-sidebar-body" id="transferServerSidebarBody"><div class="transfer-server-sidebar-header"><strong>'+p.toUpperCase()+' settings</strong><button type="button" class="btn small ghost" data-transfer-sidebar-toggle="'+p+'" data-transfer-proto="'+p+'" title="Collapse settings">'+icon('close')+'</button></div><div class="transfer-server-sidebar-fields"></div></div>';
 const fields=sidebar.querySelector('.transfer-server-sidebar-fields');
 settings.open=true;fields.appendChild(settings);
 const adapter=fields.querySelector('.transfer-interface-field');
 if(adapter&&!adapter.querySelector('[data-fm-action="interfaces"]')){
  adapter.insertAdjacentHTML('beforeend','<button type="button" class="btn small ghost transfer-sidebar-refresh" data-fm-action="interfaces">'+icon('refresh')+' Refresh adapters</button>');
 }
 const splitter=document.createElement('div');splitter.className='transfer-server-splitter';splitter.tabIndex=0;
 splitter.setAttribute('role','separator');splitter.setAttribute('aria-label','Resize server settings sidebar');splitter.setAttribute('aria-orientation','vertical');splitter.title='Drag to resize settings. Double-click to collapse or expand. Arrow keys resize.';
 const primary=document.createElement('main');primary.className='transfer-server-primary';primary.append(card,browser,logs);
 app.classList.add('transfer-server-layout');app.append(sidebar,splitter,primary);
 this.applySidebar(p);
 let startX,startWidth,activePointer;
 splitter.addEventListener('pointerdown',event=>{
  if(event.button!==0||this.sidebarState(p).collapsed)return;
  event.preventDefault();startX=event.clientX;startWidth=this.sidebarState(p).width;activePointer=event.pointerId;
  splitter.setPointerCapture(event.pointerId);
 });
 splitter.addEventListener('pointermove',event=>{
  if(activePointer!==event.pointerId)return;
  const max=Math.min(460,Math.max(270,app.clientWidth-320));
  this.sidebarState(p).width=Math.min(max,Math.max(270,Math.round(startWidth+event.clientX-startX)));
  this.applySidebar(p);
 });
 const finish=event=>{if(activePointer!==event.pointerId)return;activePointer=undefined;saveState();};
 splitter.addEventListener('pointerup',finish);splitter.addEventListener('pointercancel',finish);
 splitter.addEventListener('dblclick',()=>this.toggleSidebar(p));
 splitter.addEventListener('keydown',event=>{
  if(event.key==='Enter'||event.key===' '){event.preventDefault();this.toggleSidebar(p);return;}
  if(!['ArrowLeft','ArrowRight','Home'].includes(event.key))return;
  event.preventDefault();const state=this.sidebarState(p);state.collapsed=false;
  state.width=event.key==='Home'?320:Math.max(270,Math.min(460,state.width+(event.key==='ArrowRight'?1:-1)*(event.shiftKey?40:12)));
  this.applySidebar(p);saveState();
 });
},
applySidebar:function(p){
 const app=document.querySelector('.transfer-server-layout');if(!app)return;
 const saved=this.sidebarState(p),aside=app.querySelector('.transfer-server-config-panel'),divider=app.querySelector('.transfer-server-splitter');
 if(!aside)return;
 app.style.setProperty('--transfer-sidebar-width',saved.width+'px');
 app.classList.toggle('transfer-settings-collapsed',saved.collapsed);
 aside.classList.toggle('is-collapsed',saved.collapsed);
 const body=aside.querySelector('.transfer-server-sidebar-body');if(body)body.inert=saved.collapsed;
 for(const toggle of aside.querySelectorAll('[data-transfer-sidebar-toggle]')){
  toggle.setAttribute('aria-expanded',String(!saved.collapsed));
  toggle.title=(saved.collapsed?'Show':'Hide')+' server settings';
 }
 if(divider){divider.setAttribute('aria-valuemin','270');divider.setAttribute('aria-valuemax','460');divider.setAttribute('aria-valuenow',String(saved.width));divider.setAttribute('aria-expanded',String(!saved.collapsed));}
},
toggleSidebar:function(p){const saved=this.sidebarState(p);saved.collapsed=!saved.collapsed;this.applySidebar(p);saveState();},
activityBounds:function(primary){
 const card=primary.querySelector('.transfer-server-card');
 const style=getComputedStyle(primary),gap=parseFloat(style.rowGap)||0;
 const space=primary.clientHeight-(card?.offsetHeight||0)-190-8-gap*3-(parseFloat(style.paddingTop)||0)-(parseFloat(style.paddingBottom)||0);
 return {min:180,max:Math.max(180,Math.min(1200,Math.floor(space)))};
},
applyActivitySize:function(p){
 const primary=document.querySelector('.transfer-server-primary'),divider=primary?.querySelector('.transfer-activity-splitter');
 if(!primary||!divider)return;
 const bounds=this.activityBounds(primary),preferred=this.sidebarState(p).activityHeight||Math.min(280,bounds.max-120),height=Math.max(bounds.min,Math.min(bounds.max,preferred));
 primary.style.setProperty('--transfer-activity-height',height+'px');
 divider.setAttribute('aria-valuemin',String(bounds.min));divider.setAttribute('aria-valuemax',String(bounds.max));divider.setAttribute('aria-valuenow',String(height));
 divider.setAttribute('aria-valuetext',height+' pixels for server activity');
},
mountActivityResize:function(p){
 activityObserver?.disconnect();activityObserver=null;
 const primary=document.querySelector('.transfer-server-primary'),logs=primary?.querySelector('.transfer-log-panel');
 if(!primary||!logs)return;
 logs.id='transferServerActivityPanel';
 const divider=document.createElement('div');divider.className='transfer-activity-splitter';divider.tabIndex=0;
 divider.setAttribute('role','separator');divider.setAttribute('aria-label','Resize shared files and server activity');divider.setAttribute('aria-orientation','horizontal');divider.setAttribute('aria-controls',logs.id);
 divider.title='Drag up to enlarge server activity. Arrow keys resize. Double-click to reset.';
 logs.before(divider);this.applyActivitySize(p);
 let pointer,startY,startHeight;
 divider.addEventListener('pointerdown',event=>{
  if(event.button!==0)return;
  event.preventDefault();pointer=event.pointerId;startY=event.clientY;startHeight=logs.getBoundingClientRect().height;
  divider.setPointerCapture(pointer);divider.classList.add('is-resizing');
 });
 divider.addEventListener('pointermove',event=>{
  if(pointer!==event.pointerId)return;
  const bounds=this.activityBounds(primary);
  this.sidebarState(p).activityHeight=Math.max(bounds.min,Math.min(bounds.max,Math.round(startHeight+startY-event.clientY)));
  this.applyActivitySize(p);
 });
 const finish=event=>{
  if(pointer!==event.pointerId)return;pointer=undefined;divider.classList.remove('is-resizing');saveState();
 };
 divider.addEventListener('pointerup',finish);divider.addEventListener('pointercancel',finish);divider.addEventListener('lostpointercapture',finish);
 divider.addEventListener('dblclick',()=>{this.sidebarState(p).activityHeight=0;this.applyActivitySize(p);saveState();});
 divider.addEventListener('keydown',event=>{
  if(!['ArrowUp','ArrowDown','Home','End'].includes(event.key))return;
  event.preventDefault();const bounds=this.activityBounds(primary),current=Number(divider.getAttribute('aria-valuenow'));
  this.sidebarState(p).activityHeight=event.key==='Home'?0:event.key==='End'?bounds.max:Math.max(bounds.min,Math.min(bounds.max,current+(event.key==='ArrowUp'?1:-1)*(event.shiftKey?40:12)));
  this.applyActivitySize(p);saveState();
 });
 if(typeof ResizeObserver==='function'){
  activityObserver=new ResizeObserver(()=>{
   if(!primary.isConnected){activityObserver?.disconnect();activityObserver=null;return;}
   this.applyActivitySize(p);
  });
  activityObserver.observe(primary);const card=primary.querySelector('.transfer-server-card');if(card)activityObserver.observe(card);
 }
},
settingsVisible:function(p){return expanded[p]!==false},
setSettingsVisible:function(p,open){if(protocols.includes(p))expanded[p]=!!open},
logs:function(status){const logs=status?.transfers||[];return logs.length?'<div class="transfer-log-scroll" data-transfer-log-scroll>'+logs.map(r=>'<div class="transfer-log-line '+(r.status==='Failed'?'bad':'')+'"><time>'+esc(date(r.time))+'</time><span class="transfer-log-level">'+esc(r.status==='Failed'?'ERROR':'INFO')+'</span><strong>'+esc(r.operation)+'</strong><span class="mono">'+esc(r.peer||'local')+'</span><span class="mono">'+esc(r.file||'')+'</span><span>'+((r.bytes||0)>0?fmt(r.bytes,0)+' B ':'')+esc(r.detail||'')+'</span></div>').join('')+'</div>':'<p class="muted">No activity recorded. Connection attempts and transfers appear here after the server starts.</p>'},
refresh:async function(p,rerender=true){if(!online)return;try{
 const s=await api('/transfer/server?protocol='+p);const old=current[p],hadRoot=!!config[p].root;
 current[p]=s;if(!config[p].root)config[p].root=s.root||'';
 if(state.activePage!==p||this.mode(p)!=='server')return;
 // Avoid replacing settings while a person is typing or navigating a folder.
 if(rerender||old?.running!==s.running||(!hadRoot&&!!config[p].root))this.render(p);
 else {const log=$('#transferLogItems');if(log)log.innerHTML=this.logs(s);
  const count=$('#transferLogCount');if(count)count.textContent=(s.transfers||[]).length;
  this.syncSummary(p);window.NetworkToolkitTransferExplorer?.notice?.(p,s);}
 }catch(e){this.feedback(p,'Server status could not be refreshed: '+e.message,'error');toast(p.toUpperCase()+' server: '+e.message,'error')}},
start:async function(p,confirmed=false){
 if(!protocols.includes(p)||pending[p])return;
 const c={...config[p],protocol:p,action:'start'};
 if(p==='sftp')c.password=$('#sftpServerPassword')?.value||sessionPasswords.sftp||'';
 if(p==='ftp')c.password=$('#ftpServerPassword')?.value||sessionPasswords.ftp||'';
 const fail=(message,field)=>{this.feedback(p,message,'error');toast(message,'error');if(field){const el=document.querySelector(field);const panel=el?.closest('.fm-config-toggle');if(panel&&!panel.open){panel.open=true;expanded[p]=true;}el?.focus();}};
 if(!String(c.root||'').trim()){fail('Choose a server root folder before starting.','[data-transfer-setting="root"]');return;}
 if(!Number.isInteger(Number(c.port))||Number(c.port)<1||Number(c.port)>65535){fail('The listening port must be between 1 and 65535.','[data-transfer-setting="port"]');return;}
 if(p==='ftp'&&!String(c.username||'').trim()){fail('Enter an FTP username before starting.','[data-transfer-setting="username"]');return;}
 if(p==='ftp'&&!c.password){fail('Enter an FTP server password.','#ftpServerPassword');return;}
 if(p==='sftp'&&!String(c.username||'').trim()){fail('Enter an SSH username before starting the SFTP server.','[data-transfer-setting="username"]');return;}
 if(p==='sftp'&&!c.password&&!String(c.authorizedKeys||'').trim()){fail('Enter a session password or select an authorized SSH keys file.','#sftpServerPassword');return;}
 if((p==='tftp'||(p==='ftp'&&c.security==='plain'))&&c.bind!=='127.0.0.1'&&!confirmed){
  openModal('Start '+p.toUpperCase()+' server',`<p>${p==='tftp'?'TFTP transfers':'Plain FTP transfers'} have <strong>no authentication protection or encryption</strong>. Devices that can reach the selected address may access files according to your configured permissions.</p><div class="data-grid"><div><small>Listening on</small><strong class="mono">${esc(c.bind+':'+c.port)}</strong></div><div><small>Shared folder</small><strong class="mono">${esc(c.root)}</strong></div><div><small>Access</small><strong>${esc(c.access)}</strong></div></div><p class="help-line">Confirm that you are on a trusted network and Windows Firewall is configured appropriately.</p>`,
   `<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button type="button" class="btn primary" data-transfer-confirm-start="${p}" data-transfer-proto="${p}">Start ${p.toUpperCase()} server</button>`,true);
  return;
 }
 pending[p]=true;this.feedback(p,'Starting '+p.toUpperCase()+' server on '+c.bind+':'+c.port+'...','info');
 const button=$('[data-transfer-start="'+p+'"]');if(button){button.disabled=true;button.textContent='Starting...';}
 try{
  const s=await api('/transfer/server','POST',c);current[p]=s;config[p].root=s.root;
  if(p==='sftp')sessionPasswords.sftp='';if(p==='ftp')sessionPasswords.ftp='';
  this.feedback(p,p.toUpperCase()+' server listening on '+(s.address||c.bind+':'+c.port)+'. Remote clients can access '+s.root+'.','success');
  this.render(p);toast(p.toUpperCase()+' server started','success');
 }catch(e){
  const message='Cannot start '+p.toUpperCase()+' server: '+e.message;
  this.feedback(p,message+' Check the listening IP, port availability, root folder and access credentials.','error');toast(message,'error');
 }finally{pending[p]=false;const btn=$('[data-transfer-start="'+p+'"],[data-transfer-stop="'+p+'"]');if(btn){btn.disabled=false;if(btn.hasAttribute('data-transfer-start'))btn.innerHTML=icon('play')+'Start server';}}
},
stop:async function(p){if(pending[p])return;pending[p]=true;try{
 current[p]=await api('/transfer/server','POST',{action:'stop',protocol:p});
 this.feedback(p,p.toUpperCase()+' server stopped. The shared folder is still available in the file manager.','info');
 this.render(p);toast(p.toUpperCase()+' server stopped.','success');
 }catch(e){this.feedback(p,'Cannot stop server: '+e.message,'error');toast(e.message,'error')}finally{pending[p]=false;const btn=$('[data-transfer-start="'+p+'"],[data-transfer-stop="'+p+'"]');if(btn){btn.disabled=false;if(btn.hasAttribute('data-transfer-start'))btn.innerHTML=icon('play')+'Start server';}}},
setRoot:async function(p,requested){const dir=(requested||config[p].root||'').trim();if(!dir){this.feedback(p,'Choose a server root folder first.','error');toast('Choose a server root folder first.','error');return}
 try{const s=await api('/transfer/server','POST',{protocol:p,action:'setroot',root:dir});config[p].root=s.root;current[p]=s;this.feedback(p,'Selected server root: '+s.root,'info');this.render(p);toast('File manager opened in '+s.root,'success')}
 catch(e){const message='Cannot open folder: '+e.message;this.feedback(p,message,'error');toast(message,'error')}},
files:async function(p,sub=''){if(!current[p]?.root)return;try{const data=await api('/transfer/files?protocol='+p+'&path='+encodeURIComponent(sub));cache[p]=data;folders[p]=data.path||'';if(state.activePage!==p||this.mode(p)!=='server')return;const body=$('#transferBrowserItems');if(!body)return;$('#transferBrowserPath').textContent=data.root+(data.path?'/'+data.path:'');const items=data.entries||[];body.innerHTML='<table class="data-table transfer-file-table"><thead><tr><th>Name</th><th>Size</th><th>Modified</th><th>Actions</th></tr></thead><tbody>'+items.map(item=>'<tr data-transfer-entry-name="'+esc(item.name.toLowerCase())+'"><td><button type="button" class="transfer-file-name" data-transfer-proto="'+p+'" data-transfer-file="'+(item.directory?'open':'preview')+'" data-transfer-path="'+esc(item.path)+'">'+icon(item.directory?'file':'download')+'<span>'+esc(item.name)+'</span></button></td><td>'+(item.directory?'Folder':fmt(item.size,0)+' B')+'</td><td>'+esc(date(item.modified))+'</td><td><div class="row gap6"><button class="btn small ghost" data-transfer-proto="'+p+'" data-transfer-file="rename" data-transfer-path="'+esc(item.path)+'">Rename / move</button>'+(item.directory?'':'<button class="btn small ghost" data-transfer-proto="'+p+'" data-transfer-file="download" data-transfer-path="'+esc(item.path)+'">Download</button>')+'<button class="btn small ghost" data-transfer-proto="'+p+'" data-transfer-file="delete" data-transfer-path="'+esc(item.path)+'">Delete</button></div></td></tr>').join('')+'</tbody></table>'+(items.length?'':'<p class="help-line">This folder is empty. Use Add file or New folder.</p>');}
 catch(e){const area=$('#transferBrowserItems');if(area)area.innerHTML='<div class="notice warn">'+esc(e.message)+'</div>'}},
fileAction:function(p,action,sub){if(action==='refresh'){void this.files(p,folders[p]);return}if(action==='parent'){void this.files(p,cache[p]?.parent||'');return}if(action==='open'){void this.files(p,sub);return}
 const defaultName=sub||folders[p]||'';if(action==='preview'){void this.preview(p,sub);return}
 const isDelete=action==='delete',isRename=action==='rename',isMkdir=action==='mkdir',isUpload=action==='upload',isDownload=action==='download';
 let content='<div class="field wide"><label>File or folder relative to server root</label><input id="transferActionPath" value="'+esc(isMkdir||isUpload?(folders[p]?folders[p]+'/':''):defaultName)+'"></div>';
 if(isRename)content+='<div class="field wide"><label>New name or destination (relative to root)</label><input id="transferActionTarget" value="'+esc(sub)+'" autocomplete="off"></div>';
 if(isUpload||isDownload)content+='<div class="field wide"><label>Local '+(isUpload?'source':'destination')+' path on this computer</label><input id="transferActionLocal" placeholder="Enter an absolute Windows file path" autocomplete="off"><button class="btn small ghost mt" data-transfer-local-picker="'+p+'" data-transfer-picker-mode="'+(isUpload?'upload':'download')+'">'+icon('file')+'Browse local '+(isUpload?'files':'folder')+'</button></div>';
 if(isUpload||isDownload)content+='<label class="toggle-row"><input type="checkbox" id="transferActionOverwrite"><span>Allow overwrite if a file already exists</span></label>';
 content+='<div id="transferLocalPicker" class="transfer-picker-inline" hidden></div>';
 if(isDelete)content+='<div class="notice warn">Delete is permanent. Folders must be empty. Confirm the selected path.</div><label class="toggle-row"><input type="checkbox" id="transferActionConfirm"><span>I confirm deletion of this file or empty folder</span></label>';
 openModal(({mkdir:'Create folder',upload:'Add local file to server root',download:'Copy server file to local folder',rename:'Rename or move',delete:'Delete file or folder'})[action],content,'<button class="btn ghost" data-action="close-modal">Cancel</button><button type="button" class="btn '+(isDelete?'danger':'primary')+'" data-transfer-apply="'+action+'" data-transfer-proto="'+p+'">'+(isDelete?'Delete':'Apply')+'</button>',true);
 },
apply:async function(p,action){const payload={protocol:p,action,path:$('#transferActionPath')?.value||'',newPath:$('#transferActionTarget')?.value||'',localPath:$('#transferActionLocal')?.value||'',overwrite:!!$('#transferActionOverwrite')?.checked,confirm:!!$('#transferActionConfirm')?.checked};try{const r=await api('/transfer/files','POST',payload);closeModal();toast(r.message||'File action completed','success');void this.files(p,folders[p])}catch(e){toast(e.message,'error')}},
preview:async function(p,sub){try{const data=await api('/transfer/files?protocol='+p+'&preview='+encodeURIComponent(sub));if(data.binary){toast('This file contains binary data. Use Download to inspect it.');return}openModal('File preview: '+sub,'<pre class="transfer-file-preview">'+esc(data.content||'')+'</pre><div class="help-line">Read-only preview limited to 128 KiB.</div>','<button class="btn primary" data-action="close-modal">Close</button>',true);}catch(e){toast(e.message,'error')}},
browseRoot:async function(p,requested=''){
 try {
  const folder=await api('/transfer/local'+(requested?'?path='+encodeURIComponent(requested):''));
  const dirs=(folder.entries||[]).filter(item=>item.directory).sort((a,b)=>a.name.localeCompare(b.name,undefined,{sensitivity:'base'}));
  const roots=(folder.roots||[]).map(location=>'<button type="button" class="btn small ghost" data-transfer-root-path="'+esc(location)+'" data-transfer-proto="'+p+'">'+esc(location)+'</button>').join('');
  const rows=dirs.map(item=>'<button type="button" class="transfer-browser-row" data-transfer-root-path="'+esc(item.path)+'" data-transfer-proto="'+p+'" data-transfer-root-entry="'+esc(item.name.toLowerCase())+'" title="Open '+esc(item.name)+'">'+icon('folder')+'<span>'+esc(item.name)+'</span>'+icon('chevron')+'</button>').join('');
  const contents='<div class="transfer-root-picker" data-transfer-root-current="'+esc(folder.path)+'" data-transfer-root-protocol="'+p+'">'+
   '<p class="help-line">Choose the directory shared with devices. Double-click is not required to enter a folder. You may create a new folder here.</p>'+
   '<div class="row wrap gap6 transfer-root-drives">'+roots+'</div>'+
   '<div class="transfer-root-address"><button type="button" class="btn small ghost" data-transfer-root-path="'+esc(folder.parent)+'" data-transfer-proto="'+p+'" aria-label="Parent directory">'+icon('arrowUp')+' Up</button><input id="transferRootAddress" class="mono" value="'+esc(folder.path)+'" aria-label="Folder path" spellcheck="false"><button type="button" class="btn small ghost" data-transfer-root-go="'+p+'">Go</button></div>'+
   '<div class="transfer-root-address"><input id="transferRootFilter" placeholder="Find a folder..." aria-label="Filter folders"><button type="button" class="btn small ghost" data-transfer-root-refresh="'+p+'">'+icon('refresh')+' Refresh</button></div>'+
   '<div class="transfer-browser-list" id="transferRootResults">'+(rows||'<p class="muted" style="padding:12px">No subfolders. Create a new one or use this folder.</p>')+'</div>'+
   '<div class="transfer-root-create"><label for="transferRootNewName">New folder in this location</label><div class="row gap6"><input id="transferRootNewName" placeholder="Folder name" autocomplete="off" maxlength="250"><button type="button" class="btn small ghost" data-transfer-root-create="'+p+'">'+icon('plus')+' Create folder</button></div></div></div>';
  openModal('Select '+p.toUpperCase()+' server root',contents,
   '<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button type="button" class="btn primary" data-transfer-root-select="'+esc(folder.path)+'" data-transfer-proto="'+p+'">Use this folder</button>',true);
  $('#transferRootAddress')?.focus({preventScroll:true});
 } catch(e){toast('Browse folders: '+e.message,'error')}
},
createRootFolder:async function(p){
 const where=document.querySelector('[data-transfer-root-current]')?.dataset.transferRootCurrent;
 const name=String($('#transferRootNewName')?.value||'').trim();
 if(!where||!name||name==='.'||name==='..'||/[\\/]/.test(name)||/[<>:"|?*]/.test(name)||name.endsWith('.')||name.endsWith(' ')){
  toast('Enter a valid folder name without path separators or reserved characters.','error');return;
 }
 const sep=where.includes('\\')?'\\':'/';
 const destination=where.replace(/[\\/]$/,'')+sep+name;
 try{await api('/transfer/local-files','POST',{action:'mkdir',path:destination});await this.browseRoot(p,destination);toast('Folder created. Choose Use this folder to select it.','success')}
 catch(e){toast('Create folder: '+e.message,'error')}
},
chooseLocal:async function(p,mode,location=''){
 try{
  const folder=await api('/transfer/local'+(location?'?path='+encodeURIComponent(location):''));
  const wrap=$('#transferLocalPicker');if(!wrap)return;
  const roots=(folder.roots||[]).map(root=>'<button class="btn small ghost" data-transfer-local-path="'+esc(root)+'" data-transfer-local-mode="'+mode+'" data-transfer-proto="'+p+'">'+esc(root)+'</button>').join('');
  const rows=(folder.entries||[]).map(item=>'<button class="transfer-browser-row" data-transfer-local-'+(item.directory?'path':'file')+'="'+esc(item.path)+'" data-transfer-local-mode="'+mode+'" data-transfer-proto="'+p+'">'+icon('file')+'<span>'+esc(item.name)+'</span><small>'+esc(item.directory?'Folder':fmt(item.size,0)+' B')+'</small></button>').join('');
  wrap.hidden=false;wrap.innerHTML='<div class="row wrap gap6">'+roots+'</div><div class="transfer-browser-path">'+esc(folder.path)+'</div><div class="row wrap gap6 mt"><button class="btn small ghost" data-transfer-local-path="'+esc(folder.parent)+'" data-transfer-local-mode="'+mode+'" data-transfer-proto="'+p+'">Parent</button>'+(mode==='download'?'<button class="btn small primary" data-transfer-local-directory="'+esc(folder.path)+'" data-transfer-local-mode="'+mode+'" data-transfer-proto="'+p+'">Use this folder</button>':'')+'</div><div class="transfer-browser-list">'+rows+'</div>';
 }catch(e){toast('Local file browser: '+e.message,'error')}
},
localSelected:function(p,file,mode){const target=$('#transferActionLocal');if(!target)return;if(mode==='upload'){target.value=file}else{const name=($('#transferActionPath')?.value||'file.bin').split('/').pop();target.value=file.replace(/[\\/]$/,'')+'\\'+name}const picker=$('#transferLocalPicker');if(picker)picker.hidden=true},

};
window.NetworkToolkitTransfers=ui;
document.addEventListener('click',event=>{const sidebar=event.target.closest('[data-transfer-sidebar-toggle]');if(sidebar){event.preventDefault();event.stopPropagation();ui.toggleSidebar(sidebar.dataset.transferSidebarToggle);return;}const b=event.target.closest('[data-transfer-open-root],[data-transfer-copy-log],[data-transfer-clear-log],[data-transfer-mode],[data-transfer-start],[data-transfer-stop],[data-transfer-confirm-start],[data-transfer-refresh],[data-transfer-file],[data-transfer-apply],[data-transfer-local-picker],[data-transfer-local-path],[data-transfer-local-file],[data-transfer-local-directory],[data-transfer-root-browse],[data-transfer-root-path],[data-transfer-root-select],[data-transfer-root-go],[data-transfer-root-refresh],[data-transfer-root-create],[data-transfer-open-files]');if(!b)return;const p=b.dataset.transferProto||state.activePage;if(!protocols.includes(p))return;event.preventDefault();event.stopPropagation();if(b.dataset.transferOpenRoot){void api('/transfer/open-root','POST',{protocol:p}).then(()=>toast('Opened shared folder in Windows Explorer','success')).catch(e=>toast(e.message,'error'));return}if(b.dataset.transferCopyLog){const records=current[p]?.transfers||[];void copyText(records.map(r=>[r.time,r.peer,r.operation,r.file,r.status,r.bytes,r.detail].join(' | ')).join('\n'));return}if(b.dataset.transferClearLog){void api('/transfer/server','POST',{protocol:p,action:'clearlog'}).then(s=>{current[p]=s;const log=$('#transferLogItems');if(log)log.innerHTML=ui.logs(s);$('#transferLogCount').textContent='0'}).catch(e=>toast(e.message,'error'));return}if(b.dataset.transferOpenFiles){if(current[p]?.running){document.querySelector('.transfer-file-manager')?.scrollIntoView({block:'nearest',behavior:'smooth'});return}void ui.setRoot(p);return}if(b.dataset.transferRootBrowse){void ui.browseRoot(p,config[p].root);return}if(b.dataset.transferRootPath){void ui.browseRoot(p,b.dataset.transferRootPath);return}if(b.dataset.transferRootGo||b.dataset.transferRootRefresh){void ui.browseRoot(p,b.dataset.transferRootRefresh?document.querySelector('[data-transfer-root-current]')?.dataset.transferRootCurrent:$('#transferRootAddress')?.value);return}if(b.dataset.transferRootCreate){void ui.createRootFolder(p);return}if(b.dataset.transferRootSelect){config[p].root=b.dataset.transferRootSelect;state.transferServerSettings=state.transferServerSettings||{};const { _restored,...safe}=config[p];state.transferServerSettings[p]={...safe};closeModal();saveState();void ui.setRoot(p);return}if(b.dataset.transferMode){state.transferModes=state.transferModes||{};state.transferModes[p]=b.dataset.transferMode;renderPage();saveState();return}if(b.dataset.transferConfirmStart){closeModal();void ui.start(p,true);return}if(b.dataset.transferStart){void ui.start(p);return}if(b.dataset.transferStop){void ui.stop(p);return}if(b.dataset.transferRefresh){void ui.refresh(p);return}if(b.dataset.transferFile){ui.fileAction(p,b.dataset.transferFile,b.dataset.transferPath||'');return}if(b.dataset.transferApply){void ui.apply(p,b.dataset.transferApply);return}if(b.dataset.transferLocalPicker){void ui.chooseLocal(p,b.dataset.transferPickerMode);return}if(b.dataset.transferLocalPath){void ui.chooseLocal(p,b.dataset.transferLocalMode,b.dataset.transferLocalPath);return}if(b.dataset.transferLocalFile){ui.localSelected(p,b.dataset.transferLocalFile,b.dataset.transferLocalMode);return}if(b.dataset.transferLocalDirectory){ui.localSelected(p,b.dataset.transferLocalDirectory,b.dataset.transferLocalMode);return}},true);
document.addEventListener('toggle',event=>{const n=event.target;if(n.matches?.('[data-transfer-settings]'))expanded[n.dataset.transferSettings]=n.open},true);
document.addEventListener('input',event=>{const b=event.target;if(b.id==='sftpServerPassword'){sessionPasswords.sftp=b.value;ui.feedback('sftp','');}if(b.id==='ftpServerPassword'){sessionPasswords.ftp=b.value;ui.feedback('ftp','');}if(b.dataset.transferSetting){const p=b.dataset.transferProto;if(!protocols.includes(p))return;config[p][b.dataset.transferSetting]=b.type==='checkbox'?b.checked:b.type==='number'?Number(b.value):b.value;state.transferServerSettings=state.transferServerSettings||{};const { _restored,...safe}=config[p];state.transferServerSettings[p]={...safe};saveState();ui.syncSummary(p);if(b.dataset.transferSetting==='bind'){b.title=b.selectedOptions?.[0]?.textContent||b.value;}}if(b.id==='transferRootFilter'){const q=b.value.trim().toLowerCase();for(const row of document.querySelectorAll('[data-transfer-root-entry]'))row.hidden=!row.dataset.transferRootEntry.includes(q);}if(b.id==='transferBrowserFilter'){const text=b.value.toLowerCase();for(const row of document.querySelectorAll('[data-transfer-entry-name]'))row.hidden=!row.dataset.transferEntryName.includes(text);}},true);
document.addEventListener('keydown',event=>{if(event.key==='Enter'&&event.target.id==='transferRootAddress'){event.preventDefault();void ui.browseRoot(state.activePage,event.target.value);}else if(event.key==='Enter'&&event.target.id==='transferRootNewName'){event.preventDefault();void ui.createRootFolder(state.activePage);}},true);
setInterval(()=>{const p=state.activePage;if(protocols.includes(p)&&ui.mode(p)==='server'&&current[p]?.running)void ui.refresh(p,false)},2500);
})();
