/* Network Toolkit Commander-style local/server file explorer.
   Both panes use authenticated local APIs. The 'server' pane is the exposed
   server root folder on this computer; it is not a remote TFTP directory. */
(function(root){'use strict';
const PROTOCOLS=new Set(['sftp','tftp']);
const model={};
const sep=(p,base)=>/[\\]/.test(base)?'\\':'/';
const joinLocal=(base,name)=>String(base||'').replace(/[\\/]$/, '')+sep('',base)+name;
const joinServer=(base,name)=>[base,name].filter(Boolean).join('/');
const parentServer=value=>{const path=String(value||'').replace(/\\/g,'/');return path.includes('/')?path.slice(0,path.lastIndexOf('/')):''};
const basename=value=>String(value||'').split(/[\\/]/).pop();
const bytes=value=>{if(value===0)return '0 B';if(!Number.isFinite(value)||value<0)return '-';let num=value;for(const unit of ['B','KB','MB','GB','TB']){if(num<1024||unit==='TB')return num.toLocaleString(undefined,{maximumFractionDigits:unit==='B'?0:1})+' '+unit;num/=1024}return '-'};
const humanDate=value=>{if(!value)return '-';const d=new Date(value);return Number.isNaN(d.getTime())?'-':(globalThis.NetworkToolkitResultTime?.format(d,{milliseconds:false})||d.toLocaleString(undefined,{hour12:true}))};
const safe=value=>esc(value);
function freshPane(){return {path:'',parent:'',entries:[],roots:[],separator:'/',history:[],at:-1,selected:new Set(),anchor:-1,filter:'',sort:'name',reverse:false,hidden:false,loading:false,seq:0};}
function get(p){if(!model[p])model[p]={root:'',local:freshPane(),server:freshPane(),busy:false,cancelPending:false,queue:[],single:false,split:49,statusKey:'',active:'server'};return model[p]}
function remember(p){const m=get(p);state.transferExplorerSettings=state.transferExplorerSettings||{};state.transferExplorerSettings[p]={localPath:m.local.path,serverPath:m.server.path,split:m.split,single:m.single,sort:{local:m.local.sort,server:m.server.sort},reverse:{local:m.local.reverse,server:m.server.reverse},hidden:{local:m.local.hidden,server:m.server.hidden}};saveState()}
function restore(p){const m=get(p);if(m._restored)return;m._restored=true;const old=state.transferExplorerSettings?.[p]||{};if(typeof old.localPath==='string')m.local.path=old.localPath;if(typeof old.serverPath==='string')m.server.path=old.serverPath;if(Number.isFinite(old.split))m.split=Math.max(25,Math.min(75,old.split));m.single=old.single===true;for(const side of ['local','server']){if(['name','size','modified','type'].includes(old.sort?.[side]))m[side].sort=old.sort[side];m[side].reverse=old.reverse?.[side]===true;m[side].hidden=old.hidden?.[side]===true}}
function currentPage(p){return state.activePage===p&&root.NetworkToolkitTransfers?.mode(p)==='server'}
function toastError(e){toast(String(e?.message||e),'error')}
function paneLabel(side){return side==='local'?'This computer':'Server files'}
function paneHelp(p,side){return side==='local'?'Local files on this PC':'Files shared by the '+p.toUpperCase()+' server'}
function paneHTML(p,side){return '<section class="fm-pane" data-fm-pane="'+side+'" aria-label="'+paneLabel(side)+' file browser"><header class="fm-pane-head"><div><strong>'+paneLabel(side)+'</strong><small>'+paneHelp(p,side)+'</small></div><span class="fm-pane-count" data-fm-count="'+side+'">0 items</span></header>'+ 
 '<div class="fm-pane-quick"><button type="button" class="btn small ghost" data-fm-action="mkdir" data-fm-side="'+side+'" title="Create a folder here">'+icon('plus')+' New folder</button><button type="button" class="btn small ghost" data-fm-action="refresh" data-fm-side="'+side+'">'+icon('refresh')+' Refresh</button></div>'+
 '<div class="fm-navigation">'+
 '<button class="fm-icon" title="Back (Alt+Left)" aria-label="Back" data-fm-action="back" data-fm-side="'+side+'">'+icon('arrowLeft')+'</button>'+
 '<button class="fm-icon" title="Forward (Alt+Right)" aria-label="Forward" data-fm-action="forward" data-fm-side="'+side+'">'+icon('arrowRight')+'</button>'+
 '<button class="fm-icon" title="Parent folder (Backspace)" aria-label="Parent folder" data-fm-action="up" data-fm-side="'+side+'">'+icon('arrowUp')+'</button>'+
 '<button class="fm-icon" title="Root folder" aria-label="Root" data-fm-action="home" data-fm-side="'+side+'">'+icon('folder')+'</button>'+
 '<input class="fm-address mono" data-fm-address="'+side+'" aria-label="'+paneLabel(side)+' folder path" placeholder="Folder path - press Enter to open" spellcheck="false" />'+
 '<button class="fm-icon" title="Go to folder" aria-label="Go to folder" data-fm-action="go" data-fm-side="'+side+'">'+icon('arrowRight')+'</button>'+
 '<button class="fm-icon" title="Refresh (F5)" aria-label="Refresh folder" data-fm-action="refresh" data-fm-side="'+side+'">'+icon('refresh')+'</button></div>'+
 '<div class="fm-filter-bar"><input class="fm-filter" data-fm-filter="'+side+'" placeholder="Filter this folder..." aria-label="Filter '+side+' files" spellcheck="false" /><label title="Show hidden dotfiles"><input type="checkbox" data-fm-hidden="'+side+'">Hidden</label></div>'+
 '<div class="fm-list-scroll" data-fm-scroll="'+side+'" tabindex="0"><table class="fm-table"><thead><tr>'+['name','size','modified','type'].map(field=>'<th data-fm-sort="'+field+'" data-fm-side="'+side+'" title="Sort by '+field+'" scope="col">'+({name:'Name',size:'Size',modified:'Modified',type:'Type'}[field])+'<span data-fm-arrow="'+field+'"></span></th>').join('')+'</tr></thead><tbody data-fm-rows="'+side+'"><tr><td colspan="4" class="fm-empty">Choose a folder.</td></tr></tbody></table></div>'+
 '<footer class="fm-pane-foot" data-fm-foot="'+side+'">Double-click to open · Ctrl/Shift to select · F2 to rename</footer></section>'}
function shell(p){return '<div class="fm-explorer" data-fm-explorer="'+p+'"><div class="fm-toolbar"><div class="fm-tool-actions">'+
 '<button class="btn small primary" data-fm-action="upload" title="Copy selected computer files into the server folder">'+icon('arrowRight')+' Upload</button>'+
 '<button class="btn small ghost" data-fm-action="download" title="Copy selected server files into the computer folder">'+icon('arrowLeft')+' Download</button>'+
 '<span class="fm-tool-separator"></span><button class="btn small ghost" data-fm-action="mkdir" title="Create a directory in the active pane">'+icon('plus')+' Folder</button>'+
 '<button class="btn small ghost" data-fm-action="rename" title="Rename or move selected server file (F2)">'+icon('edit')+' Rename</button>'+
 '<button class="btn small ghost" data-fm-action="duplicate" title="Copy selected server file or folder to a different server folder">'+icon('copy')+' Copy to...</button>'+
 '<button class="btn small ghost" data-fm-action="delete" title="Delete selected server items (Delete)">'+icon('trash')+' Delete</button>'+
 '<button class="btn small ghost" data-fm-action="preview" title="Preview selected text file">'+icon('file')+' Preview</button>'+
 '<button class="btn small ghost" data-fm-action="path" title="Copy selected file path or current folder path">'+icon('copy')+' Path</button>'+
 '</div><div class="fm-toolbar-end"><button class="btn small ghost" data-fm-action="single" title="Show or hide computer pane">'+icon('grid')+' <span data-fm-single-label>Two panes</span></button><span class="fm-toolbar-help">Ctrl/Shift select · Double-click to open</span></div></div>'+ 
 '<div class="fm-split" data-fm-split>'+paneHTML(p,'local')+'<div class="fm-gutter" data-fm-gutter role="separator" tabindex="0" aria-label="Resize file browser panels" aria-orientation="vertical" aria-valuemin="25" aria-valuemax="75" aria-valuenow="49" title="Drag to resize. Left and Right arrows adjust, Shift makes larger steps, Home or End selects a limit. Double-click resets."></div>'+paneHTML(p,'server')+'</div>'+ 
 '<div class="fm-operation" data-fm-operation hidden></div><p class="fm-footnote">The server pane is the local folder exposed to incoming '+p.toUpperCase()+' connections. Moving files between panes copies them on this computer, even when the server is stopped. '+(p==='tftp'?'TFTP clients cannot browse remote directories.':'Remote SFTP clients can browse the server root after authenticating.')+'</p></div>'}
function mount(p,rootPath,status){if(!PROTOCOLS.has(p))return;const host=document.querySelector('.transfer-file-manager');if(!host)return;
 const m=get(p);restore(p);if(rootPath!==m.root){m.root=rootPath;m.server=freshPane();m.server.path='';m.statusKey=''}
 host.innerHTML=shell(p);host.classList.add('fm-file-manager');
 const card=document.querySelector('.transfer-server-card');if(card){const form=card.querySelector('#transferServerSettings');if(form&&!card.querySelector('.fm-config-toggle')){const helper=form.nextElementSibling;const d=document.createElement('details');d.className='fm-config-toggle';d.open=root.NetworkToolkitTransfers?.settingsVisible?.(p)!==false;d.innerHTML='<summary>Server settings <span>Network interface, permissions and shared folder</span></summary>';form.before(d);d.append(form);d.querySelector('summary')?.addEventListener('click',()=>{root.NetworkToolkitTransfers?.setSettingsVisible?.(p,!d.open)});if(helper?.classList.contains('help-line'))d.append(helper);}const bind=card.querySelector('[data-transfer-setting="bind"]');if(bind){const line=document.createElement('div');line.className='fm-bind-refresh';line.innerHTML='<button type="button" class="btn small ghost" data-fm-action="interfaces">'+icon('refresh')+' Refresh network adapters</button>';bind.closest('.field')?.append(line);}}
 applyLayout(p);
 for(const side of ['local','server']){const a=host.querySelector('[data-fm-address="'+side+'"]'),f=host.querySelector('[data-fm-filter="'+side+'"]'),hidden=host.querySelector('[data-fm-hidden="'+side+'"]');if(a)a.value=m[side].path;if(f)f.value=m[side].filter;if(hidden)hidden.checked=m[side].hidden;}
 if(currentPage(p)){
  void load(p,'local',m.local.path,{skipHistory:true});
  if(rootPath&&status?.root===rootPath)void load(p,'server',m.server.path,{skipHistory:true});
  else setMessage(p,'server','Select a server root folder to browse shared files.');
 }
}
function applyLayout(p){const m=get(p),el=document.querySelector('[data-fm-explorer="'+p+'"]');if(!el)return;el.style.setProperty('--fm-left',m.split+'%');el.classList.toggle('fm-single',m.single);const tag=el.querySelector('[data-fm-single-label]');if(tag)tag.textContent=m.single?'Server pane only':'Two panes';const gutter=el.querySelector('[data-fm-gutter]');if(gutter){gutter.setAttribute('aria-valuenow',String(Math.round(m.split)));gutter.setAttribute('aria-valuetext',Math.round(m.split)+'% computer pane, '+Math.round(100-m.split)+'% server pane');gutter.setAttribute('aria-disabled',String(!!m.single));gutter.tabIndex=m.single?-1:0;}}
function setMessage(p,side,text){const node=document.querySelector('[data-fm-rows="'+side+'"]');if(node&&currentPage(p))node.innerHTML='<tr><td colspan="4" class="fm-empty">'+safe(text)+'</td></tr>'}
function rememberHistory(p,side,path,skip=false){const v=get(p)[side];if(skip){if(v.at<0){v.history=[path];v.at=0}return;}if(v.history[v.at]!==path){v.history=v.history.slice(0,v.at+1).concat([path]).slice(-80);v.at=v.history.length-1}}
async function load(p,side,dir='',opts={}){if(!currentPage(p))return;const m=get(p),v=m[side];if(side==='server'&&!m.root){setMessage(p,side,'Choose the root folder first.');return}
 const seq=++v.seq;v.loading=true;setMessage(p,side,'Loading folder...');try{
  const url=side==='local'?'/transfer/local'+(dir?'?path='+encodeURIComponent(dir):''):'/transfer/files?protocol='+p+'&path='+encodeURIComponent(dir);
  const data=await api(url);
  if(seq!==v.seq||!currentPage(p))return;
  v.path=data.path||'';v.parent=data.parent||'';v.separator=data.separator||v.separator;v.roots=data.roots||[];v.entries=data.entries||[];v.selected=new Set();v.anchor=-1;v.loading=false;
  rememberHistory(p,side,v.path,opts.skipHistory);
  draw(p,side);remember(p);
 }catch(e){v.loading=false;if(seq!==v.seq)return;setMessage(p,side,e.message);toastError(e)}}
function extension(row){if(row.directory)return 'Folder';const name=row.name||'',suffix=name.lastIndexOf('.');return suffix>0?name.slice(suffix+1).toUpperCase()+' file':'File'}
// Sorting a directory can be expensive. Selection and visibility changes
// reuse this view until the directory array or relevant preferences change.
const transferSortCache=new WeakMap();
const transferNameSort=new Intl.Collator(undefined,{numeric:true,sensitivity:'base'});
function ordered(v){
 const entries=v.entries,key=v.sort,reverse=!!v.reverse,cached=transferSortCache.get(v);
 if(cached&&cached.entries===entries&&cached.sort===key&&cached.reverse===reverse)return cached.rows;
 const rows=entries.slice(),types=key==='type'?new Map(rows.map(row=>[row,extension(row)])):null,dates=key==='modified'?new Map(rows.map(row=>[row,Date.parse(row.modified)||0])):null;
 rows.sort((a,b)=>{if(a.directory!==b.directory)return a.directory?-1:1;let cmp=0;if(key==='size')cmp=(a.size||0)-(b.size||0);else if(key==='modified')cmp=dates.get(a)-dates.get(b);else if(key==='type')cmp=transferNameSort.compare(types.get(a),types.get(b));else cmp=transferNameSort.compare(a.name,b.name);return (reverse?-1:1)*cmp||transferNameSort.compare(a.name,b.name)});
 transferSortCache.set(v,{entries,sort:key,reverse,rows});
 return rows;
}
function visible(v){
 const rows=ordered(v),filter=String(v.filter||'').trim().toLowerCase(),hidden=!!v.hidden,cached=transferSortCache.get(v);
 if(cached?.visible&&cached.visibleSource===rows&&cached.filter===filter&&cached.hidden===hidden)return cached.visible;
 const shown=rows.filter(row=>(hidden||!row.name.startsWith('.'))&&(!filter||row.name.toLowerCase().includes(filter)));
 cached.visibleSource=rows;cached.filter=filter;cached.hidden=hidden;cached.visible=shown;
 return shown;
}
function paint(p,side){const m=get(p),v=m[side];if(!currentPage(p))return;const pane=document.querySelector('[data-fm-pane="'+side+'"]');if(!pane)return;pane.classList.toggle('fm-focused',m.active===side);for(const tr of pane.querySelectorAll('[data-fm-row]')){const marked=v.selected.has(tr.dataset.fmPath);tr.classList.toggle('fm-selected',marked);tr.setAttribute('aria-selected',String(marked))}
 const count=pane.querySelector('[data-fm-count]');if(count)count.textContent=v.entries.length+' items';const foot=pane.querySelector('[data-fm-foot]');if(foot){const selected=v.entries.filter(e=>v.selected.has(e.path));const total=selected.reduce((n,e)=>n+(e.directory?0:e.size||0),0);foot.textContent=selected.length?selected.length+' selected'+(total?' · '+bytes(total):''):v.entries.length+' items in folder';}
}
function draw(p,side){const m=get(p),v=m[side],pane=document.querySelector('[data-fm-pane="'+side+'"]');if(!pane||!currentPage(p))return;const address=pane.querySelector('[data-fm-address]');if(address&&document.activeElement!==address)address.value=v.path;
 pane.querySelector('[data-fm-filter]').value=v.filter;pane.querySelector('[data-fm-hidden]').checked=v.hidden;
 const rows=visible(v),body=pane.querySelector('[data-fm-rows]');body.innerHTML=rows.length?rows.map(row=>'<tr tabindex="0" draggable="true" role="row" data-fm-row="'+side+'" data-fm-path="'+safe(row.path)+'" title="'+safe(row.path)+'" aria-selected="false"><td><div class="fm-entry-name">'+icon(row.directory?'folder':'file')+'<span>'+safe(row.name)+'</span></div></td><td class="fm-entry-size">'+(row.directory?'':bytes(row.size))+'</td><td>'+humanDate(row.modified)+'</td><td>'+extension(row)+'</td></tr>').join(''):'<tr><td colspan="4" class="fm-empty">'+(v.entries.length?'No items match the filter.':'This folder is empty.')+'</td></tr>';
 for(const key of ['name','size','modified','type']){const mark=pane.querySelector('[data-fm-arrow="'+key+'"]');if(mark)mark.textContent=v.sort===key?(v.reverse?' ▼':' ▲'):'';}
 paint(p,side);
}
function selectRow(p,side,path,evt){const m=get(p),v=m[side],all=visible(v).map(item=>item.path),idx=all.indexOf(path);if(idx<0)return;m.active=side;
 if(evt.shiftKey&&v.anchor>=0){if(!evt.ctrlKey&&!evt.metaKey)v.selected.clear();for(let i=Math.min(idx,v.anchor);i<=Math.max(idx,v.anchor);i++)v.selected.add(all[i]);}
 else if(evt.ctrlKey||evt.metaKey){if(v.selected.has(path))v.selected.delete(path);else v.selected.add(path);v.anchor=idx;}
 else{v.selected=new Set([path]);v.anchor=idx}
 paint(p,'local');paint(p,'server');}
function selected(p,side){const m=get(p),v=m[side];return v.entries.filter(e=>v.selected.has(e.path))}
function destination(p,side,row){const m=get(p);return side==='server'?joinServer(m.server.path,row.name):joinLocal(m.local.path,row.name)}
function dialog(title,content,action,label='Apply',danger=false){openModal(title,content,'<button class="btn ghost" data-action="close-modal">Cancel</button><button class="btn '+(danger?'danger':'primary')+'" type="button" data-fm-apply="'+action+'">'+safe(label)+'</button>',true)}
function ask(p,action){const m=get(p),s=action==='upload'?'local':'server',entries=selected(p,s);
 if(['upload','download'].includes(action)){
  if(!entries.length){toast('Select one or more files or folders in the '+paneLabel(s)+' pane first.','info');return}
  if(!m.root){toast('Select the server root first.','error');return}
  const target=action==='upload'?'Server files':'This computer';
  dialog((action==='upload'?'Upload':'Download')+' '+entries.length+' item'+(entries.length===1?'':'s'),'<p>Copy the selected '+entries.length+' item'+(entries.length===1?'':'s')+' to the current '+target+' folder?</p><p class="mono fm-dialog-path">'+safe(m[action==='upload'?'server':'local'].path||m.root)+'</p><label class="toggle-row"><input type="checkbox" id="fmOverwrite"><span>Merge folders and overwrite existing files</span></label><p class="help-line">Original files remain in place. Folder transfers include subfolders and skip unsafe links.</p>',action,'Copy '+entries.length+' item'+(entries.length===1?'':'s'));return;
 }
 if(action==='mkdir'){
  dialog('Create folder in '+paneLabel(m.active),'<label class="field"><span>New folder name</span><input id="fmNewName" placeholder="New folder" maxlength="250"></label>',action,'Create folder');return;
 }
 if(action==='rename'||action==='duplicate'){
  const side=m.active,entries=selected(p,side);
  if(entries.length!==1){toast('Select one file or folder in the active pane.','info');return}
  const old=entries[0].path,defaultPath=action==='duplicate'?(side==='server'?joinServer(parentServer(old),'Copy of '+basename(old)):joinLocal(m.local.path,'Copy of '+basename(old))):old;
  dialog(action==='duplicate'?'Copy file or folder':'Rename or move','<p class="help-line">Source: <span class="mono">'+safe(old)+'</span></p><label class="field"><span>Destination '+(side==='server'?'relative to server root':'absolute local path')+'</span><input id="fmDestination" value="'+safe(defaultPath)+'" autocomplete="off"></label>'+(action==='duplicate'?'<label class="toggle-row"><input type="checkbox" id="fmOverwrite"><span>Merge folders / overwrite existing files</span></label>':''),action,action==='duplicate'?'Copy':'Rename / move');return;
 }
 if(action==='delete'){
  if(!entries.length){toast('Select files or empty folders to delete.','info');return}
  dialog('Delete '+entries.length+' item'+(entries.length===1?'':'s'),'<div class="notice warn">Delete is permanent. Only empty directories can be deleted. '+(m.active==='server'?'Selected server files may be used by connected network devices.':'Selected local files will be permanently removed from this computer.')+'</div><p class="fm-delete-list">'+entries.slice(0,8).map(v=>safe(v.name)).join(', ')+(entries.length>8?' and '+(entries.length-8)+' more':'')+'</p><label class="toggle-row"><input type="checkbox" id="fmDeleteConfirm"><span>I confirm permanent deletion</span></label>',action,'Delete',true);return;
 }
 if(action==='preview'){
  const v=selected(p,m.active);if(v.length!==1||v[0].directory){toast('Select one text file to preview.','info');return}void preview(p,m.active,v[0]);return;
 }
 if(action==='path'){
  const v=selected(p,m.active),path=v.length===1?v[0].path:m[m.active].path;
  if(path){void copyText(path);toast('Path copied.','success')}return;
 }
}
async function preview(p,side,item){const url=side==='server'?'/transfer/files?protocol='+p+'&preview='+encodeURIComponent(item.path):'/transfer/local?preview='+encodeURIComponent(item.path);
 try{const content=await api(url);if(content.binary){toast('Binary file: use a suitable editor to inspect it.','info');return}openModal('Preview: '+item.name,'<pre class="transfer-file-preview">'+safe(content.content||'')+'</pre><p class="help-line">Read-only preview, maximum 128 KiB.</p>','<button class="btn primary" data-action="close-modal">Close</button>',true)}catch(e){toastError(e)}}
async function apply(p,action){const m=get(p),loc=m.local,server=m.server;let actions=[];const overwrite=!!document.querySelector('#fmOverwrite')?.checked;
 if(action==='mkdir'){
  const name=String(document.querySelector('#fmNewName')?.value||'').trim();if(!name||name==='.'||name==='..'||/[\\/]/.test(name)){toast('Enter one folder name without slashes.','error');return}
  actions=[{protocol:p,action:'mkdir',path:m.active==='server'?joinServer(server.path,name):joinLocal(loc.path,name),_side:m.active}];
 } else if(['upload','download'].includes(action)){
  const side=action==='upload'?'local':'server',entries=selected(p,side);if(!entries.length)return;
  actions=entries.map(item=>({protocol:p,action,path:destination(p,action==='upload'?'server':'local',item),localPath:action==='upload'?item.path:destination(p,'local',item),overwrite}));
  if(action==='download')for(let i=0;i<actions.length;i++)actions[i].path=entries[i].path;
 } else if(['rename','duplicate'].includes(action)){
  const entry=selected(p,m.active)[0];const newPath=document.querySelector('#fmDestination')?.value.trim();if(!entry||!newPath||newPath===entry.path){toast('Choose a different destination.','error');return}
  actions=[{protocol:p,action:action==='duplicate'?'copy':'rename',path:entry.path,newPath,overwrite,_side:m.active}];
 } else if(action==='delete'){
  if(!document.querySelector('#fmDeleteConfirm')?.checked){toast('Confirm deletion explicitly.','error');return}
  actions=selected(p,m.active).map(entry=>({protocol:p,action:'delete',path:entry.path,confirm:true,_side:m.active}));
 }
 if(!actions.length)return;
 closeModal();m.busy=true;m.cancelPending=false;let completed=0;const errors=[];
 const progress=document.querySelector('[data-fm-operation]');
 const display=()=>{if(!progress)return;const done=completed+errors.length;progress.hidden=false;progress.innerHTML='<span>'+safe(action)+' '+done+' / '+actions.length+(errors.length?' · '+errors.length+' failed':'')+'</span><progress max="'+actions.length+'" value="'+done+'" aria-label="File operations complete"></progress>'+(m.busy?'<button class="btn small ghost" data-fm-action="stopQueue" title="Stop after current item">Stop pending</button>':'')};display();
 for(const request of actions){if(m.cancelPending)break;try{await api(request._side==='local'?'/transfer/local-files':'/transfer/files','POST',request);completed++}catch(e){errors.push((request.path||'item')+': '+e.message)}display();}
 const stopped=m.cancelPending;m.busy=false;display();
 await Promise.all([load(p,'local',loc.path,{skipHistory:true}),load(p,'server',server.path,{skipHistory:true})]);
 if(progress){progress.hidden=false;progress.textContent=completed+' completed'+(errors.length?' · '+errors.length+' failed':'');}
 toast(completed+' item'+(completed===1?'':'s')+' completed'+(errors.length?' · '+errors.length+' failed':'')+(stopped?' · pending items cancelled':''),errors.length?'error':'success');
 if(errors.length)openModal('File operation results','<pre class="transfer-file-preview">'+safe(errors.join('\n'))+'</pre>','<button class="btn primary" data-action="close-modal">Close</button>',true);
}
function nav(p,side,op,value){const m=get(p),v=m[side];m.active=side;
 if(op==='go'){const field=document.querySelector('[data-fm-address="'+side+'"]');void load(p,side,field?.value||'');return}
 if(op==='refresh'){void load(p,side,v.path,{skipHistory:true});return}
 if(op==='up'){void load(p,side,side==='server'?v.parent:v.parent||v.path);return}
 if(op==='home'){void load(p,side,side==='server'?'':v.roots[0]||v.path);return}
 if(op==='back'&&v.at>0){v.at--;void load(p,side,v.history[v.at],{skipHistory:true});return}
 if(op==='forward'&&v.at<v.history.length-1){v.at++;void load(p,side,v.history[v.at],{skipHistory:true});return}
}
function action(p,op,side){const m=get(p);if(op==='stopQueue'){m.cancelPending=true;toast('The queue will stop after the current file.','info');return}if(m.busy){toast('Complete the current file operation first.','info');return}if(op==='single'){m.single=!m.single;applyLayout(p);remember(p);return}
 if(op==='interfaces'){void(async()=>{try{system=await api('/system');root.NetworkToolkitTransfers.render(p);toast('Network adapters refreshed.','success')}catch(e){toastError(e)}})();return}
 if(['back','forward','up','home','go','refresh'].includes(op)){nav(p,side||m.active,op);return}
 if(['upload','download','mkdir','rename','duplicate','delete','preview','path'].includes(op)){if(side)m.active=side;ask(p,op)}
}
function resetLayout(p){const m=get(p);m.split=49;m.single=false;applyLayout(p);remember(p);}
function notice(p,status){const m=get(p);if(!status?.running)return;const entries=status.transfers||[];const key=entries.length+'|'+(entries[0]?.time||'')+'|'+(entries[0]?.file||'');if(key!==m.statusKey){const changed=!!m.statusKey;m.statusKey=key;if(changed&&currentPage(p))void load(p,'server',m.server.path,{skipHistory:true})}}
let paneResize=null;
function setPaneRatio(p,value,persist=false){
 const m=get(p);if(m.single)return false;
 const next=Math.min(75,Math.max(25,Number(value)||49));
 if(next===m.split)return false;
 m.split=next;applyLayout(p);if(persist)remember(p);return true;
}
function applyPanePointer(d,clientX){
 if(!d.node?.isConnected)return false;
 const bounds=d.node.getBoundingClientRect();
 return bounds.width>0?setPaneRatio(d.protocol,((clientX-bounds.left)/bounds.width)*100):false;
}
function finishPaneResize(event,commit){
 const d=paneResize;if(!d||event?.pointerId!=null&&event.pointerId!==d.pointerId)return;
 paneResize=null;
 if(d.frame!==null)root.cancelAnimationFrame?.(d.frame);
 if(commit&&d.node.isConnected)applyPanePointer(d,event.clientX??d.lastX);
 else{get(d.protocol).split=d.initial;applyLayout(d.protocol);}
 try{if(d.gutter.hasPointerCapture?.(d.pointerId))d.gutter.releasePointerCapture(d.pointerId);}catch{}
 if(commit&&get(d.protocol).split!==d.initial)remember(d.protocol);
}
function handlers(){
 document.addEventListener('click',event=>{const el=event.target.closest('[data-fm-action],[data-fm-sort],[data-fm-row]');if(!el)return;const wrap=el.closest('[data-fm-explorer]'),p=wrap?.dataset.fmExplorer||state.activePage;if(!PROTOCOLS.has(p)||!currentPage(p))return;const side=el.dataset.fmSide||el.dataset.fmRow;
 if(el.dataset.fmRow){selectRow(p,side,el.dataset.fmPath,event);return}
 event.preventDefault();event.stopPropagation();
 if(el.dataset.fmSort){const m=get(p),v=m[side],key=el.dataset.fmSort;if(v.sort===key)v.reverse=!v.reverse;else{v.sort=key;v.reverse=false}draw(p,side);remember(p);return}
 if(el.dataset.fmAction)action(p,el.dataset.fmAction,side);
 },true);
 document.addEventListener('dblclick',event=>{const row=event.target.closest('[data-fm-row]');if(!row)return;const wrap=row.closest('[data-fm-explorer]');if(!wrap)return;event.preventDefault();const p=wrap.dataset.fmExplorer,side=row.dataset.fmRow,item=get(p)[side].entries.find(x=>x.path===row.dataset.fmPath);if(!item)return;if(item.directory)void load(p,side,item.path);else void preview(p,side,item)},true);
 document.addEventListener('change',event=>{const hidden=event.target.closest('[data-fm-hidden]');if(!hidden)return;const p=state.activePage,side=hidden.dataset.fmHidden;if(!PROTOCOLS.has(p))return;const v=get(p)[side];v.hidden=hidden.checked;draw(p,side);remember(p)},true);
 document.addEventListener('input',event=>{const filter=event.target.closest('[data-fm-filter]');if(!filter)return;const p=state.activePage,side=filter.dataset.fmFilter;if(!PROTOCOLS.has(p))return;const v=get(p)[side];v.filter=filter.value;const body=filter.closest('.fm-pane')?.querySelector('[data-fm-rows]');if(body){const rows=visible(v),matches=new Set(rows.map(r=>r.path));for(const tr of body.querySelectorAll('[data-fm-row]'))tr.hidden=!matches.has(tr.dataset.fmPath)}},true);
 document.addEventListener('focusin',event=>{const node=event.target.closest('[data-fm-pane]');if(node&&PROTOCOLS.has(state.activePage)){get(state.activePage).active=node.dataset.fmPane;paint(state.activePage,'server');paint(state.activePage,'local')}},true);
 document.addEventListener('keydown',event=>{const p=state.activePage;if(!PROTOCOLS.has(p)||!currentPage(p)||document.querySelector('.modal'))return;
  const gutter=event.target.closest('[data-fm-gutter]');if(gutter){
   if(get(p).single||!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;
   event.preventDefault();event.stopPropagation();
   const m=get(p),step=event.shiftKey?10:2,ratio=event.key==='Home'?25:event.key==='End'?75:m.split+(event.key==='ArrowLeft'?-step:step);
   setPaneRatio(p,ratio,true);return;
  }
  const address=event.target.closest('[data-fm-address]');if(address){if(event.key==='Enter'){event.preventDefault();nav(p,address.dataset.fmAddress,'go')}return}
  if(event.target.closest('input,textarea,select')&&!event.target.closest('[data-fm-scroll]'))return;
  const pane=event.target.closest('[data-fm-pane]'),m=get(p),side=pane?.dataset.fmPane||m.active;
  const key=event.key.toLowerCase();let command='';
  if(event.key==='Backspace'&&pane)command='up';else if(event.key==='F5'&&pane)command='refresh';else if(event.key==='F2'&&pane)command='rename';else if(event.key==='Delete'&&pane)command='delete';else if(event.altKey&&key==='arrowleft')command='back';else if(event.altKey&&key==='arrowright')command='forward';else if((event.ctrlKey||event.metaKey)&&key==='a'&&pane){event.preventDefault();get(p)[side].selected=new Set(visible(get(p)[side]).map(x=>x.path));paint(p,side);return}
  else if((event.ctrlKey||event.metaKey)&&key==='l'&&pane){event.preventDefault();pane.querySelector('[data-fm-address]')?.focus();pane.querySelector('[data-fm-address]')?.select();return}
  else if((event.ctrlKey||event.metaKey)&&key==='c'&&pane)command='path';
  if(command){event.preventDefault();event.stopPropagation();action(p,command,side)}
 },true);
 document.addEventListener('click',event=>{const control=event.target.closest('[data-fm-apply]');if(!control)return;event.preventDefault();event.stopPropagation();const p=state.activePage;if(PROTOCOLS.has(p))void apply(p,control.dataset.fmApply)},true);
 document.addEventListener('dragstart',event=>{const tr=event.target.closest('[data-fm-row]');if(!tr)return;const p=state.activePage,side=tr.dataset.fmRow;if(!PROTOCOLS.has(p))return;const m=get(p);if(!m[side].selected.has(tr.dataset.fmPath))selectRow(p,side,tr.dataset.fmPath,{});event.dataTransfer?.setData('application/x-networktoolkit-file-pane',side);if(event.dataTransfer)event.dataTransfer.effectAllowed='copy'},true);
 document.addEventListener('dragover',event=>{const pane=event.target.closest('[data-fm-pane]');if(!pane)return;const p=state.activePage,side=pane.dataset.fmPane;if(!PROTOCOLS.has(p))return;if(side==='local'||side==='server'){event.preventDefault();event.dataTransfer.dropEffect='copy'}},true);
 document.addEventListener('drop',event=>{const pane=event.target.closest('[data-fm-pane]');if(!pane)return;const p=state.activePage;if(!PROTOCOLS.has(p))return;const from=event.dataTransfer?.getData('application/x-networktoolkit-file-pane'),to=pane.dataset.fmPane;if(from&&from!==to){event.preventDefault();get(p).active=from;ask(p,from==='local'?'upload':'download')}},true);
 document.addEventListener('pointerdown',event=>{const gutter=event.target.closest?.('[data-fm-gutter]'),p=gutter?.closest('[data-fm-explorer]')?.dataset.fmExplorer||state.activePage;if(!gutter||event.button!==0||!PROTOCOLS.has(p)||!currentPage(p)||get(p).single)return;finishPaneResize(null,false);paneResize={protocol:p,pointerId:event.pointerId,gutter,node:gutter.parentElement,initial:get(p).split,lastX:event.clientX,frame:null};event.preventDefault();try{gutter.setPointerCapture?.(event.pointerId);}catch{}},true);
 document.addEventListener('pointermove',event=>{const d=paneResize;if(!d||event.pointerId!==d.pointerId)return;d.lastX=event.clientX;if(d.frame!==null)return;if(typeof root.requestAnimationFrame!=='function'){applyPanePointer(d,d.lastX);return;}d.frame=root.requestAnimationFrame(()=>{d.frame=null;if(paneResize===d)applyPanePointer(d,d.lastX);});},true);
 document.addEventListener('pointerup',event=>finishPaneResize(event,true),true);
 document.addEventListener('pointercancel',event=>finishPaneResize(event,false),true);
 document.addEventListener('lostpointercapture',event=>finishPaneResize(event,false),true);
 root.addEventListener?.('blur',()=>finishPaneResize(null,false));
 document.addEventListener('dblclick',event=>{const gutter=event.target.closest?.('[data-fm-gutter]'),p=gutter?.closest('[data-fm-explorer]')?.dataset.fmExplorer||state.activePage;if(!gutter||!PROTOCOLS.has(p)||!currentPage(p))return;event.preventDefault();setPaneRatio(p,49,true);},true);
}
handlers();root.NetworkToolkitTransferExplorer=Object.freeze({mount,notice,load,resetLayout,model:get});
})(globalThis);
