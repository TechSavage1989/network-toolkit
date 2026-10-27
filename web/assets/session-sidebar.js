(function(root){
'use strict';
const bindings=new Map();
let menu=null,menuTarget=null,pendingDelete=null,pendingName=null,pendingOpen=null,pendingFolder=null,savedFolder='',sidebarSizeObserver=null,sidebarRenderedWidth=null;
const folders=root.NetworkToolkitSessionFolders;
const byId=id=>document.getElementById(id);
const profiles=()=>state.terminalProfiles||[];
const recent=()=>state.terminalRecent||[];
const list=kind=>kind==='saved'?profiles():kind==='recent'?recent():[];
const key=config=>JSON.stringify(safeTerminalConfig(config));
const connectionKey=config=>{const clean=safeTerminalConfig(config);delete clean.credentialId;return JSON.stringify(clean);};
const signature=item=>JSON.stringify(item);
const restricted=()=>DETACHED_TERMINAL&&!DETACHED_WORKSPACE;
const sidebarTabs=['saved','recent','macros','files'];
const tabTitle=tab=>tab==='recent'?'Recent':tab==='macros'?'Macros':tab==='files'?'SFTP':'Saved';
function activeTab(){if(restricted())return 'macros';const wanted=sidebarTabs.includes(state.terminalSidebarTab)?state.terminalSidebarTab:'saved';return wanted==='files'&&!root.NetworkToolkitTerminalSFTP?.available?.(typeof currentTerminal==='function'?currentTerminal():null)?'saved':wanted;}
function sidebarSnapshot(){return {tab:activeTab(),collapsed:state.terminalSidebarCollapsed===true};}
function notifySidebarChange(before){const after=sidebarSnapshot();if(before.tab===after.tab&&before.collapsed===after.collapsed)return false;document.dispatchEvent(new CustomEvent('terminal-sidebar-tab-change',{detail:after}));return true;}
function activateTab(tab,options={}){
 if(!sidebarTabs.includes(tab)||restricted()&&tab!=='macros'||tab==='files'&&!root.NetworkToolkitTerminalSFTP?.available?.(typeof currentTerminal==='function'?currentTerminal():null))return false;
 const before=sidebarSnapshot();state.terminalSidebarTab=tab;if(options.toggle===true)state.terminalSidebarCollapsed=before.tab===tab&&!before.collapsed;else if(options.expand!==false)state.terminalSidebarCollapsed=false;
 const after=sidebarSnapshot();if(before.tab===after.tab&&before.collapsed===after.collapsed)return false;
 closeMenu();render();saveState();scheduleTerminalFit();notifySidebarChange(before);return true;
}
function setCollapsed(value){
 const before=sidebarSnapshot();state.terminalSidebarCollapsed=value===true;
 if(before.collapsed===state.terminalSidebarCollapsed)return false;
 closeMenu();render();saveState();scheduleTerminalFit();notifySidebarChange(before);return true;
}
const sidebarSide=()=>state.terminalSidebarSide==='left'?'left':'right';
const safeSidebarWidth=value=>Number.isInteger(value)&&value>=260&&value<=480?value:300;
function sidebarWidthLimits(){
 const columns=byId('terminalWorkspaceColumns'),available=columns?.clientWidth||columns?.getBoundingClientRect().width||innerWidth;
 const maximum=Math.max(44,Math.min(480,Math.floor(available/2)-12));
 return {minimum:Math.min(260,maximum),maximum};
}
function layoutSidebarSize(){
 const sidebar=byId('terminalSessionSidebar'),divider=byId('terminalSidebarDivider');if(!sidebar||!divider)return;
 const columns=byId('terminalWorkspaceColumns');if(columns)columns.classList.toggle('sidebar-left',sidebarSide()==='left');
 const {minimum,maximum}=sidebarWidthLimits(),collapsed=state.terminalSidebarCollapsed===true,expanded=Math.max(minimum,Math.min(maximum,safeSidebarWidth(state.terminalSidebarWidth))),width=collapsed?44:expanded;
 sidebar.style.flexBasis=width+'px';
 divider.classList.toggle('sidebar-collapsed',collapsed);
 divider.setAttribute('aria-label',collapsed?'Show terminal sidebar':'Resize or hide terminal sidebar');
 divider.setAttribute('aria-valuemin',collapsed?'0':String(minimum));divider.setAttribute('aria-valuemax',String(maximum));divider.setAttribute('aria-valuenow',collapsed?'0':String(expanded));divider.setAttribute('aria-valuetext',collapsed?'Collapsed':expanded+' pixels');
 divider.title=collapsed?'Double-click or press Enter or Space to show the sidebar.':'Drag to resize. Double-click or press Enter or Space to hide the sidebar. Arrow keys resize; Home resets.';
 if(sidebarRenderedWidth!==width){sidebarRenderedWidth=width;scheduleTerminalFit();}
}
function resizeSidebar(event){
 if(event.button!==0)return;
 event.preventDefault();event.stopImmediatePropagation();const divider=event.target;divider.focus({preventScroll:true});
 if(state.terminalSidebarCollapsed)return;
 const initial=Number(divider.getAttribute('aria-valuenow')),start=event.clientX,original=safeSidebarWidth(state.terminalSidebarWidth),pointer=event.pointerId;
 divider.classList.add('dragging');document.body.classList.add('resizing-columns');
 const move=next=>{if(pointer!==undefined&&next.pointerId!==undefined&&pointer!==next.pointerId)return;const {minimum,maximum}=sidebarWidthLimits(),width=Math.max(minimum,Math.min(maximum,Math.round(initial+(sidebarSide()==='left'?1:-1)*(next.clientX-start))));state.terminalSidebarWidth=Math.max(260,Math.min(480,width));layoutSidebarSize();};
 const finish=()=>{root.removeEventListener('pointermove',move);root.removeEventListener('pointerup',finish);root.removeEventListener('pointercancel',finish);root.removeEventListener('blur',finish);divider.classList.remove('dragging');document.body.classList.remove('resizing-columns');if(safeSidebarWidth(state.terminalSidebarWidth)!==original)saveState();};
 root.addEventListener('pointermove',move);root.addEventListener('pointerup',finish);root.addEventListener('pointercancel',finish);root.addEventListener('blur',finish);
}
const canOpen=()=>!restricted()&&!terminalConnectBusy&&!terminalWorkspaceTransferBusy&&(!DETACHED_TERMINAL||terminalDetachedReady&&!terminalDetachedReturning)&&online&&!OFFLINE&&terminalAssetsReady();
function token(kind,id){const item=list(kind).find(item=>item.id===id);return item?{kind,id,signature:signature(item)}:null;}
function itemFor(target){const item=target&&list(target.kind).find(item=>item.id===target.id);return item&&signature(item)===target.signature?item:null;}
function label(config){return ['cmd','powershell'].includes(config.type)?terminalProtocolLabel(config):config.type==='serial'?config.serialPort:config.host;}
function description(config){if(['cmd','powershell'].includes(config.type))return terminalProtocolLabel(config)+' · This computer';return terminalProtocolLabel(config)+' · '+(config.type==='serial'?config.serialPort+' · '+config.baudRate+' baud':(config.username?config.username+'@':'')+config.host+':'+config.port);}
function preferences(value={}){
 const before=sidebarSnapshot();
 state.terminalSidebarCollapsed=value.terminalSidebarCollapsed===true;
 state.terminalSidebarWidth=safeSidebarWidth(value.terminalSidebarWidth);
 state.terminalSidebarSide=value.terminalSidebarSide==='left'?'left':'right';
 state.terminalSidebarTab=restricted()?'macros':sidebarTabs.includes(value.terminalSidebarTab)?value.terminalSidebarTab:'saved';
 state.terminalRecentCollapsed=value.terminalRecentCollapsed===true;
 state.terminalSavedCollapsed=value.terminalSavedCollapsed===true;
 state.terminalSessionConfirmDelete=value.terminalSessionConfirmDelete!==false;
 const seen=new Set();
 state.terminalRecent=(Array.isArray(value.terminalRecent)?value.terminalRecent:[]).filter(item=>item&&typeof item==='object'&&typeof item.id==='string'&&typeof item.name==='string').slice(0,50).map(item=>({id:item.id.slice(0,80),name:item.name.slice(0,100),lastUsed:Number.isSafeInteger(item.lastUsed)&&item.lastUsed>=0?item.lastUsed:0,...safeTerminalConfig(item)})).filter(item=>{if(!item.id||seen.has(item.id))return false;seen.add(item.id);return true;});
 if(byId('terminalSessionSidebar')){render();notifySidebarChange(before);}
}
function displayPreferences(){return {terminalRecent:recent().map(item=>({id:item.id,name:item.name,lastUsed:item.lastUsed,...safeTerminalConfig(item)})),terminalSidebarTab:activeTab(),terminalSidebarSide:sidebarSide(),terminalSidebarWidth:safeSidebarWidth(state.terminalSidebarWidth),terminalSidebarCollapsed:state.terminalSidebarCollapsed===true,terminalRecentCollapsed:state.terminalRecentCollapsed===true,terminalSavedCollapsed:state.terminalSavedCollapsed===true,terminalSessionConfirmDelete:state.terminalSessionConfirmDelete!==false};}
function record(entry,name=''){
 if(!entry?.info?.id)return;
 const config=safeTerminalConfig(entry.config||entry.info);
 if(!label(config))return;
 const old=recent().find(item=>key(item)===key(config));
 const item={id:old?.id||'recent-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,8),name:String(name||old?.name||entry.info.label||label(config)).slice(0,100),lastUsed:Date.now(),...config};
 state.terminalRecent=[item,...recent().filter(other=>other.id!==item.id)].slice(0,50);
 bindings.set(item.id,entry.info.id);
 const retained=new Set(recent().map(item=>item.id));for(const id of bindings.keys())if(!retained.has(id))bindings.delete(id);
 saveState();render();
}
function rowHTML(kind,item){
 const detail=description(item),used=kind==='recent'&&item.lastUsed?'Last opened: '+new Date(item.lastUsed).toLocaleString():'';
 return '<div class="session-library-row" data-session-kind="'+kind+'" data-session-id="'+esc(item.id)+'"><button type="button" class="session-library-entry" data-session-entry title="'+esc([detail,used,'Double-click or press Enter to open'].filter(Boolean).join('\n'))+'"><span class="session-library-icon">'+icon(item.type==='serial'?'plug':'terminal')+'</span><span class="session-library-text"><strong>'+esc(item.name||label(item))+'</strong><small>'+esc(detail)+'</small>'+(kind==='saved'&&item.folder?'<small class="session-library-folder" title="'+esc(item.folder)+'">'+esc(item.folder)+'</small>':'')+'</span></button>'+(kind==='saved'?folderButton('terminal',item):'')+'<button type="button" class="iconbtn session-library-delete" data-session-remove aria-label="Remove '+esc(item.name||label(item))+' from '+(kind==='saved'?'Saved':'Recent')+'" title="Remove from '+(kind==='saved'?'Saved':'Recent')+'">'+icon('close')+'</button></div>';
}
function folderButton(type,item){return '<button type="button" class="iconbtn session-library-folder-action" data-session-folder="'+type+'" data-session-folder-id="'+esc(item.id)+'" aria-label="Move '+esc(item.name||item.host)+' to folder" title="Move to folder">'+icon('folder')+'</button>';}
function transferItems(kind){const key=kind==='saved'?'ftpProfiles':'ftpRecent';return (Array.isArray(state[key])?state[key]:[]).filter(x=>x&&x.id&&x.host);}
function transferRowsHTML(kind,query,filter=''){const items=transferItems(kind).filter(x=>(kind!=='saved'||folders.matches(x,filter))&&([x.name,x.folder,x.host,x.username,x.security].join(' ').toLocaleLowerCase()).includes(query));
 if(kind==='saved'&&filter&&!items.length)return '';
 if(!items.length&&!transferItems(kind).length)return '';
 return '<div class="session-library-section transfer-session-section"><div class="session-library-section-heading"><strong class="session-library-section-toggle">FTP / FTPS</strong><span class="session-library-section-count">'+transferItems(kind).length+'</span></div><div class="session-library-section-body">'+
 (items.map(x=>'<div class="session-library-row" data-transfer-session-kind="'+kind+'" data-transfer-session-id="'+esc(x.id)+'"><button type="button" class="session-library-entry" data-transfer-session-open title="Open in FTP/FTPS client"><span class="session-library-icon">'+icon('ftp')+'</span><span class="session-library-text"><strong>'+esc(x.name||x.host)+'</strong><small>'+esc((x.username||'')+'@'+x.host+':'+x.port+' · '+(x.security||'ftp'))+'</small>'+(kind==='saved'&&x.folder?'<small class="session-library-folder" title="'+esc(x.folder)+'">'+esc(x.folder)+'</small>':'')+'</span></button>'+(kind==='saved'?folderButton('ftp',x):'')+'<button type="button" class="iconbtn session-library-delete" data-transfer-session-remove title="Remove FTP/FTPS session" aria-label="Remove FTP/FTPS session">'+icon('close')+'</button></div>').join('')||'<p class="session-library-empty">No matching FTP/FTPS connections.</p>')+'</div></div>';
}
function sectionHTML(kind){
 const saved=kind==='saved',title=saved?'Saved':'Recent',action=saved?'new':'clear-recent',caption=saved?'New':'Clear',description=saved?'New saved session':'Clear recent sessions';
 return '<section id="session'+title+'Section" class="session-library-section" aria-label="'+title+' sessions"><div class="session-library-section-heading"><button type="button" id="session'+title+'Toggle" class="session-library-section-toggle" data-session-action="toggle-'+kind+'" aria-expanded="true" aria-controls="session'+title+'List"><svg class="session-library-chevron" viewBox="0 0 16 16" aria-hidden="true"><path d="m5 3 5 5-5 5"/></svg><span>'+title+'</span><span class="session-library-section-count" id="session'+title+'Count"></span></button><button type="button" class="btn small ghost session-library-section-action" data-session-action="'+action+'" title="'+description+'" aria-label="'+description+'">'+icon(saved?'plus':'trash')+caption+'</button></div><div id="session'+title+'List" class="session-library-section-body"></div></section>';
}
function mount(workspace){
 if(!workspace)return;
 if(!byId('terminalSessionSidebar')){
  const panel=workspace.querySelector('.terminal-panel');if(!panel)return;
  const columns=document.createElement('div');columns.id='terminalWorkspaceColumns';columns.className='terminal-workspace-columns';
  columns.innerHTML='<aside id="terminalSessionSidebar" class="terminal-session-sidebar" aria-label="Terminal sidebar"><div class="terminal-sidebar-rail"><div class="terminal-sidebar-tabs" role="tablist" aria-label="Terminal sidebar tools" aria-orientation="vertical">'+sidebarTabs.map(tab=>'<button type="button" id="terminalSidebarTab'+tabTitle(tab)+'" class="terminal-sidebar-tab" role="tab" data-session-tab="'+tab+'" aria-controls="terminalSidebar'+tabTitle(tab)+'" aria-selected="false" tabindex="-1" title="Open '+tabTitle(tab)+'">'+icon(tab==='saved'?'bookmark':tab==='recent'?'clock':tab==='files'?'folder':'play')+'<span>'+tabTitle(tab)+'</span></button>').join('')+'</div></div><div id="terminalSidebarContent" class="terminal-sidebar-content"><div class="session-library-heading"><strong id="terminalSidebarHeading">Saved</strong><button type="button" class="iconbtn terminal-sidebar-side-button" data-session-action="swap-side" title="Move sidebar to other side" aria-label="Move terminal sidebar to other side">'+icon('arrowSwap')+'</button></div><div id="sessionLibrarySearchGroup"><label class="sr-only" for="sessionLibrarySearch">Search recent and saved sessions</label><input id="sessionLibrarySearch" type="search" placeholder="Search sessions" autocomplete="off"><div id="sessionFolderFilterGroup" class="session-library-folder-filter"><label class="sr-only" for="sessionLibraryFolder">Saved session folder</label><select id="sessionLibraryFolder"><option value="">All folders</option><option value="unfiled">Unfiled</option></select></div></div><div id="terminalSidebarPanels" class="session-library-lists"><div id="terminalSidebarSaved" class="session-library-tabpanel" role="tabpanel" aria-labelledby="terminalSidebarTabSaved">'+sectionHTML('saved')+'</div><div id="terminalSidebarRecent" class="session-library-tabpanel" role="tabpanel" aria-labelledby="terminalSidebarTabRecent">'+sectionHTML('recent')+'</div><div id="terminalSidebarSFTP" class="session-library-tabpanel terminal-sidebar-sftp" role="tabpanel" aria-labelledby="terminalSidebarTabSFTP"></div><div id="terminalSidebarMacros" class="session-library-tabpanel terminal-sidebar-macros" role="tabpanel" aria-labelledby="terminalSidebarTabMacros"></div></div><div id="sessionLibrarySettings" class="session-library-settings"><label><input type="checkbox" id="sessionConfirmDelete" checked><span>Confirm before removing Saved or Recent entries</span></label><p>Double-click or press Enter to open. Right-click for more actions.</p></div></div></aside>';
  columns.insertAdjacentHTML('beforeend','<div id="terminalSidebarDivider" role="separator" tabindex="0" aria-label="Resize or hide terminal sidebar" aria-controls="terminalSidebarContent" aria-orientation="vertical" aria-valuemin="260" aria-valuemax="480" aria-valuenow="300"></div>');
  workspace.append(columns);columns.append(panel);columns.append(byId('terminalSidebarDivider'),byId('terminalSessionSidebar'));
  if(typeof ResizeObserver==='function'){sidebarSizeObserver=new ResizeObserver(layoutSidebarSize);sidebarSizeObserver.observe(columns);}
 }
 render();
 return byId('terminalSessionSidebar');
}
function ensure(workspace=byId('terminalWorkspace')){return mount(workspace);}
function render(){
 const sidebar=byId('terminalSessionSidebar');if(!sidebar)return;
 const collapsed=state.terminalSidebarCollapsed===true;
 const sideButton=sidebar.querySelector('[data-session-action="swap-side"]');if(sideButton)sideButton.title='Move terminal sidebar to '+(sidebarSide()==='left'?'right':'left');
 const current=typeof currentTerminal==='function'?currentTerminal():null;
 const filesSupported=!restricted()&&root.NetworkToolkitTerminalSFTP?.sync?.(current)===true;
 const tab=activeTab(),limited=restricted();state.terminalSidebarTab=tab;
 sidebar.classList.toggle('is-collapsed',collapsed);sidebar.classList.toggle('sessions-restricted',limited);
 layoutSidebarSize();
 const content=byId('terminalSidebarContent');if(content){content.hidden=collapsed;content.inert=collapsed;}
 const heading=byId('terminalSidebarHeading');if(heading&&heading.textContent!==tabTitle(tab))heading.textContent=tabTitle(tab);
 for(const name of sidebarTabs){
  const button=byId('terminalSidebarTab'+tabTitle(name)),panel=byId('terminalSidebar'+tabTitle(name)),selected=name===tab,unavailable=limited&&name!=='macros'||name==='files'&&!filesSupported;
  if(button){button.hidden=unavailable;button.disabled=unavailable;button.tabIndex=selected?0:-1;button.setAttribute('aria-selected',String(selected));button.setAttribute('aria-disabled',String(unavailable));button.title=(collapsed||!selected?'Open ':'Close ')+tabTitle(name);button.setAttribute('aria-expanded',String(selected&&!collapsed));}
  if(panel)panel.hidden=collapsed||!selected||unavailable;
 }
 const searchGroup=byId('sessionLibrarySearchGroup'),settings=byId('sessionLibrarySettings');if(searchGroup)searchGroup.hidden=!['saved','recent'].includes(tab);if(settings)settings.hidden=!['saved','recent'].includes(tab);
 if(filesSupported&&tab==='files')root.NetworkToolkitTerminalSFTP?.render?.(byId('terminalSidebarSFTP'),current);
 const confirm=byId('sessionConfirmDelete');if(confirm)confirm.checked=state.terminalSessionConfirmDelete!==false;
 const query=(byId('sessionLibrarySearch')?.value||'').trim().toLocaleLowerCase();
 const filterGroup=byId('sessionFolderFilterGroup'),picker=byId('sessionLibraryFolder'),folderNames=folders.names(profiles(),transferItems('saved'));
 if(filterGroup)filterGroup.hidden=tab!=='saved';
 if(savedFolder.startsWith('folder:')&&!folderNames.includes(savedFolder.slice(7)))savedFolder='';
 if(picker){const options='<option value="">All folders</option><option value="unfiled">Unfiled</option>'+folderNames.map(name=>'<option value="'+esc('folder:'+name)+'">'+esc(name)+'</option>').join('');if(picker.innerHTML!==options)picker.innerHTML=options;picker.value=savedFolder;picker.title=savedFolder.startsWith('folder:')?savedFolder.slice(7):savedFolder==='unfiled'?'Unfiled':'All folders';}
 for(const kind of ['recent','saved']){
  const title=kind==='recent'?'Recent':'Saved',items=list(kind),area=byId('session'+title+'List'),collapsed=state[kind==='recent'?'terminalRecentCollapsed':'terminalSavedCollapsed']===true;
  byId('session'+title+'Toggle')?.setAttribute('aria-expanded',String(!collapsed));
  if(area)area.hidden=collapsed;
  const count=byId('session'+title+'Count');if(count)count.textContent=String(items.length+(kind==='saved'?transferItems(kind).length:0));
  if(!area)continue;
  const rows=kind==='saved'?items.slice().sort((a,b)=>a.name.localeCompare(b.name)):items;
  const filtered=rows.filter(item=>(kind!=='saved'||folders.matches(item,savedFolder))&&(item.name+' '+(kind==='saved'?item.folder||'':'')+' '+description(item)).toLocaleLowerCase().includes(query));
  const transferHTML=transferRowsHTML(kind,query,kind==='saved'?savedFolder:'');
  const html=(filtered.map(item=>rowHTML(kind,item)).join('')||(!transferHTML?'<p class="session-library-empty">'+(query||kind==='saved'&&savedFolder?'No matching sessions.':kind==='recent'?'SSH connections you open appear here.':'Save a connection to open it here.')+'</p>':''))+transferHTML;
  if(area._sessionHTML!==html){area.innerHTML=html;area._sessionHTML=html;}
 }
 const clear=sidebar.querySelector('[data-session-action="clear-recent"]');if(clear)clear.disabled=!recent().length;
}
function changed(){saveState();render();renderTerminalProfiles();if(typeof terminalProfileListConfig==='function'&&typeof dataListViews!=='undefined'){const config=terminalProfileListConfig();dataListViews.set(config.id,config);refreshDataList(config.id);}}
function closeMenu(focus=false){const old=menuTarget;menu?.remove();menu=null;menuTarget=null;if(focus&&itemFor(old))document.querySelector('[data-session-kind="'+old.kind+'"][data-session-id="'+CSS.escape(old.id)+'"] [data-session-entry]')?.focus({preventScroll:true});}
function menuItems(kind){return kind==='saved'?[['open','Open','play'],['new-tab','Open in new tab','plus'],['edit','Edit connection...','settings'],['rename','Rename...','edit'],['folder','Move to folder...','folder'],['duplicate','Duplicate...','copy'],['copy','Copy connection details','copy'],['remove','Remove from Saved...','trash']]:[['open','Open / reconnect','play'],['new-tab','Open in new tab','plus'],['save','Save connection...','save'],['edit','Edit and open...','settings'],['copy','Copy connection details','copy'],['remove','Remove from Recent...','trash']];}
function openMenu(target,x,y){
 closeMenu();const item=itemFor(target);if(!item)return;
 menuTarget=target;menu=document.createElement('div');menu.id='sessionLibraryContextMenu';menu.className='terminal-context-menu';menu.setAttribute('role','menu');menu.setAttribute('aria-label','Session actions');
 menu.innerHTML='<div class="terminal-context-label">'+esc(item.name||label(item))+'</div>'+menuItems(target.kind).map(([action,name,ico])=>'<button type="button" role="menuitem" data-session-menu="'+action+'" '+(['open','new-tab'].includes(action)&&!canOpen()?'disabled':'')+'>'+icon(ico)+'<span>'+name+'</span></button>').join('');
 document.body.append(menu);const rect=menu.getBoundingClientRect();menu.style.left=Math.max(6,Math.min(x,innerWidth-rect.width-6))+'px';menu.style.top=Math.max(6,Math.min(y,innerHeight-rect.height-6))+'px';
 menu.querySelector('button:not(:disabled)')?.focus({preventScroll:true});
}
function remove(kind,id){const target=token(kind,id);if(target)requestDelete([target]);}
function clearRecent(){requestDelete(recent().map(item=>token('recent',item.id)).filter(Boolean),true);}
function requestDelete(targets,all=false){
 if(!targets.length)return;
 closeMenu();
 if(state.terminalSessionConfirmDelete===false){commitDelete(targets);return;}
 pendingDelete=targets;
 const item=itemFor(targets[0]);
 openModal(all?'Clear recent sessions':'Remove session','<p>'+ (all?'Remove all '+targets.length+' entries from Recent?':'Remove <strong>'+esc(item?.name||'this session')+'</strong> from '+(targets[0].kind==='saved'?'Saved':'Recent')+'?')+'</p><p class="muted">Open terminal connections and their output remain available.</p><label class="toggle-row mt"><input type="checkbox" id="sessionDeleteDontAsk"><span>Do not ask again for session list removals</span></label>','<button class="btn ghost" data-action="close-modal">Cancel</button><button class="btn danger" data-session-action="confirm-delete">'+(all?'Clear Recent':'Remove session')+'</button>',true);
}
function commitDelete(targets){
 const matching=targets.filter(target=>itemFor(target));
 for(const kind of ['saved','recent']){
  const ids=new Set(matching.filter(target=>target.kind===kind).map(target=>target.id));
  if(kind==='saved'){state.terminalProfiles=profiles().filter(item=>!ids.has(item.id));if(ids.has(terminalProfileID))terminalProfileID='';}
  else{state.terminalRecent=recent().filter(item=>!ids.has(item.id));for(const id of ids)bindings.delete(id);}
 }
 changed();
}
function nameDialog(mode,target,config,name){
 pendingName={mode,target,config:safeTerminalConfig(config)};
 openModal(mode==='rename'?'Rename saved session':mode==='duplicate'?'Duplicate saved session':'Save session','<form id="sessionLibraryNameForm"><div class="field"><label for="sessionLibraryName">Session name</label><input id="sessionLibraryName" maxlength="100" value="'+esc(name||label(config))+'" required autocomplete="off"></div>'+folders.inputHTML('sessionLibrarySaveFolder',config.folder,folders.names(profiles(),transferItems('saved')))+'<p class="help-line">'+esc(description(config))+'</p></form>','<button class="btn ghost" data-action="close-modal">Cancel</button><button type="submit" form="sessionLibraryNameForm" class="btn primary">'+(mode==='rename'?'Rename':'Save session')+'</button>',true);
}
function folderDialog(type,id){
 const item=(type==='ftp'?transferItems('saved'):profiles()).find(row=>row.id===id);if(!item)return;
 closeMenu();pendingFolder={type,id,signature:signature(item)};
 openModal('Move saved session','<form id="sessionLibraryFolderForm"><p>'+esc(item.name||item.host)+'</p>'+folders.inputHTML('sessionLibraryMoveFolder',item.folder,folders.names(profiles(),transferItems('saved')))+'</form>','<button class="btn ghost" data-action="close-modal">Cancel</button><button type="submit" form="sessionLibraryFolderForm" class="btn primary">Move session</button>',true);
}
function save(entry){if(entry?.config)nameDialog('save',null,entry.config,entry.info.label);}
async function open(target,newTab=false){
 const item=itemFor(target);if(!item)return;
 if(!canOpen()){toast('Wait for the terminal workspace and local engine to be ready.');return;}
 closeMenu();
 if(!newTab){
  const bound=target.kind==='recent'?terminalSessions.get(bindings.get(target.id)):null;
  const entry=bound||[...terminalSessions.values()].reverse().find(entry=>terminalOwnerCanMutate(entry)&&!entry.transferBusy&&connectionKey(entry.config)===connectionKey(item));
  if(entry&&terminalOwnerCanMutate(entry)&&!entry.transferBusy){navigate('terminal');selectTerminal(entry.info.id);if(['closed','error'].includes(entry.info.state)&&terminalReconnectAllowed(entry))await reconnectTerminal(entry);return;}
 }
 const destination=terminalDestination();
 await connect({...item,profileId:target.kind==='saved'?item.id:''},item.name,destination);
}
function authDialog(config,name,destination){
 pendingOpen={config:{...safeTerminalConfig(config),profileId:config.profileId||''},name,destination};
 const key=config.authMethod==='key';
 openModal('Sign in: '+name,'<form id="sessionLibraryAuthForm" autocomplete="off"><p>'+esc(description(config))+'</p>'+ (key?'<div class="field"><label for="sessionLibraryKey">Private key</label><textarea id="sessionLibraryKey" required spellcheck="false" autocomplete="off" data-terminal-secret></textarea></div><div class="field mt"><label for="sessionLibraryPassphrase">Key passphrase (optional)</label><input type="password" id="sessionLibraryPassphrase" autocomplete="new-password" data-terminal-secret></div>':'<div class="field"><label for="sessionLibraryPassword">Password</label><input type="password" id="sessionLibraryPassword" autocomplete="new-password" maxlength="4096" data-terminal-secret></div>')+'<p class="help-line">Link a saved credential in Edit connection to open without this prompt.</p></form>','<button class="btn ghost" data-action="close-modal">Cancel</button><button type="submit" class="btn primary" form="sessionLibraryAuthForm">Connect</button>',true);
}
async function connect(config,name,destination,secrets=null){
 if(!canOpen())return;
 terminalConnectBusy=true;renderTerminalViewControls();
 let request;
 try{
  let c=safeTerminalConfig(config);
  if(secrets===null&&c.type==='ssh'&&c.credentialId){const credential=await root.NetworkToolkitCredentials.prepare(c);if(!credential)throw new Error('The saved credential is unavailable. Edit this connection to choose another credential.');c={...c,...credential};}else if(secrets!==null)delete c.credentialId;
  c=validateTerminalConfig(c);
  if(secrets===null&&!c.credentialId&&c.type==='ssh'&&['password','key'].includes(c.authMethod)){authDialog({...c,profileId:config.profileId||''},name,destination);return;}
  request=terminalConnectPayload(c,secrets||{});
  if(config.profileId)request.profileId=config.profileId;
  if(c.type==='ssh'&&!c.credentialId&&c.authMethod==='key'&&!request.privateKey.trim())throw new Error('Enter a private key to open this session.');
  const response=await api('/terminals','POST',request);
  if(!response.session?.id)throw new Error('The app did not return a terminal session.');
  if(response.viewState)receiveTerminalViewState(response.viewState);
  const entry=terminalSessions.get(response.session.id)||upsertTerminalSession(response.session);
  if(!entry)throw new Error('The new session is no longer available.');
  entry.config=safeTerminalConfig(c);
  entry.macroStartupPending={reconnected:false};
  if(typeof notifyTerminalMacroConnected==='function')notifyTerminalMacroConnected(entry);
  if(c.type==='serial')entry.serialLines={dtr:request.dtr??true,rts:c.flowControl==='rtscts'?null:request.rts??true,flowControl:c.flowControl};
  record(entry,name);
  const pane=placeConnectedTerminal(entry,destination,true);
  navigate('terminal');if(pane>=0)selectTerminal(entry.info.id);else toast('The connection is available in its terminal tab.');
  startTerminalFeed();scheduleTerminalFit();
 }catch(error){toast(error.message,'error');}
 finally{if(request){delete request.password;delete request.privateKey;delete request.passphrase;}if(secrets){delete secrets.password;delete secrets.privateKey;delete secrets.passphrase;}terminalConnectBusy=false;renderTerminalChrome();}
}
async function run(action,target){
 const item=itemFor(target);if(!item)return;
 closeMenu();
 switch(action){
  case'open':await open(target);break;case'new-tab':await open(target,true);break;
  case'edit':openTerminalConnect(item,target.kind==='saved'?item.id:'');break;
  case'rename':nameDialog('rename',target,item,item.name);break;
  case'duplicate':nameDialog('duplicate',target,item,item.name+' copy');break;
  case'folder':if(target.kind==='saved')folderDialog('terminal',item.id);break;
  case'save':nameDialog('save',target,item,item.name);break;
  case'copy':await NetworkToolkitTerminalInteraction.copyValue(description(item));break;
  case'remove':remove(target.kind,target.id);break;
 }
}
function eventTarget(node){const row=node.closest?.('[data-session-kind][data-session-id]');return row?token(row.dataset.sessionKind,row.dataset.sessionId):null;}
document.addEventListener('dblclick',event=>{const ftp=event.target.closest?.('[data-transfer-session-open]');const entry=ftp?.closest('[data-transfer-session-id]');if(entry){event.preventDefault();event.stopImmediatePropagation();void root.NetworkToolkitRemoteBrowser?.openLibraryEntry?.('ftp',entry.dataset.transferSessionKind,entry.dataset.transferSessionId);return;}},true);
document.addEventListener('dblclick',event=>{if(event.target.id==='terminalSidebarDivider'){event.preventDefault();event.stopImmediatePropagation();setCollapsed(!state.terminalSidebarCollapsed);return;}if(!event.target.closest?.('[data-session-entry]'))return;const target=eventTarget(event.target);if(target){event.preventDefault();event.stopImmediatePropagation();void open(target).catch(error=>toast(error.message,'error'));}},true);
document.addEventListener('contextmenu',event=>{const target=eventTarget(event.target);if(!target)return;event.preventDefault();event.stopImmediatePropagation();openMenu(target,event.clientX,event.clientY);},true);
document.addEventListener('click',event=>{
 const folder=event.target.closest?.('[data-session-folder]');if(folder){event.preventDefault();event.stopImmediatePropagation();folderDialog(folder.dataset.sessionFolder,folder.dataset.sessionFolderId);return;}
 const ftpOpen=event.target.closest?.('[data-transfer-session-open]');
 const ftpRemove=event.target.closest?.('[data-transfer-session-remove]');
 if(ftpOpen||ftpRemove){const row=(ftpOpen||ftpRemove).closest('[data-transfer-session-id]');if(!row)return;event.preventDefault();event.stopImmediatePropagation();
  if(ftpOpen){void root.NetworkToolkitRemoteBrowser?.openLibraryEntry?.('ftp',row.dataset.transferSessionKind,row.dataset.transferSessionId);return;}
  const group=row.dataset.transferSessionKind==='saved'?'ftpProfiles':'ftpRecent';if(!confirm('Remove this FTP/FTPS connection from '+row.dataset.transferSessionKind+'?'))return;
  state[group]=(state[group]||[]).filter(item=>item.id!==row.dataset.transferSessionId);saveState();render();return;
 }
 const tab=event.target.closest?.('[data-session-tab]');
 if(tab){event.preventDefault();event.stopImmediatePropagation();if(!tab.disabled)activateTab(tab.dataset.sessionTab,{toggle:true});return;}
 const action=event.target.closest?.('[data-session-action]'),removeButton=event.target.closest?.('[data-session-remove]'),menuButton=event.target.closest?.('[data-session-menu]');
 if(!action&&!removeButton&&!menuButton)return;
 event.preventDefault();event.stopImmediatePropagation();
 if(menuButton){if(!menuButton.disabled)void run(menuButton.dataset.sessionMenu,menuTarget).catch(error=>toast(error.message,'error'));return;}
 if(removeButton){const target=eventTarget(removeButton);if(target&&!removeButton.disabled)remove(target.kind,target.id);return;}
 if(action.disabled)return;
 switch(action.dataset.sessionAction){
  case'swap-side':state.terminalSidebarSide=sidebarSide()==='left'?'right':'left';layoutSidebarSize();saveState();scheduleTerminalFit();break;
  case'collapse':{const wasCollapsed=state.terminalSidebarCollapsed===true;setCollapsed(!wasCollapsed);if(wasCollapsed)action.focus({preventScroll:true});else byId('terminalSidebarTab'+tabTitle(activeTab()))?.focus({preventScroll:true});break;}
  case'toggle-recent':case'toggle-saved':{const property=action.dataset.sessionAction==='toggle-recent'?'terminalRecentCollapsed':'terminalSavedCollapsed';state[property]=!state[property];saveState();render();break;}
  case'clear-recent':clearRecent();break;
  case'new':openTerminalConnect();break;
  case'confirm-delete':{const targets=pendingDelete;pendingDelete=null;if(!targets)return;if(byId('sessionDeleteDontAsk')?.checked)state.terminalSessionConfirmDelete=false;closeModal();commitDelete(targets);break;}
 }
},true);
document.addEventListener('submit',event=>{
 if(event.target.id==='sessionLibraryNameForm'){
  event.preventDefault();event.stopImmediatePropagation();
  const pending=pendingName,name=byId('sessionLibraryName')?.value.trim();if(!pending||!name||name.length>100)return;
  if(pending.target&&!itemFor(pending.target)){toast('This session changed. Open its actions again.');closeModal();return;}
  const folder=folders.normalize(byId('sessionLibrarySaveFolder')?.value);
  if(pending.mode==='rename'){const item=itemFor(pending.target);item.name=name;item.folder=folder;}
  else{if(profiles().length>=100){toast('Remove a saved session before adding another. The limit is 100.');return;}state.terminalProfiles=[{id:'terminal-profile-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,8),name,folder,...pending.config},...profiles()];}
  pendingName=null;closeModal();changed();
 }else if(event.target.id==='sessionLibraryFolderForm'){
  event.preventDefault();event.stopImmediatePropagation();const pending=pendingFolder;if(!pending)return;
  const item=(pending.type==='ftp'?transferItems('saved'):profiles()).find(row=>row.id===pending.id);
  if(!item||signature(item)!==pending.signature){pendingFolder=null;closeModal();toast('This session changed. Open its actions again.');return;}
  item.folder=folders.normalize(byId('sessionLibraryMoveFolder')?.value);pendingFolder=null;closeModal();changed();
 }else if(event.target.id==='sessionLibraryAuthForm'){
  event.preventDefault();event.stopImmediatePropagation();
  const pending=pendingOpen;if(!pending||!canOpen())return;
  const secrets={password:byId('sessionLibraryPassword')?.value||'',privateKey:byId('sessionLibraryKey')?.value||'',passphrase:byId('sessionLibraryPassphrase')?.value||''};
  for(const input of event.target.querySelectorAll('[data-terminal-secret]'))input.value='';
  pendingOpen=null;closeModal();void connect(pending.config,pending.name,pending.destination,secrets);
 }
},true);
document.addEventListener('input',event=>{if(event.target.id==='sessionLibrarySearch')render();});
document.addEventListener('change',event=>{if(event.target.id==='sessionLibraryFolder'){savedFolder=event.target.value;closeMenu();render();return;}if(event.target.id==='sessionConfirmDelete'){state.terminalSessionConfirmDelete=event.target.checked;saveState();}});
document.addEventListener('pointerdown',event=>{if(menu&&!menu.contains(event.target))closeMenu();if(event.target.id==='terminalSidebarDivider')resizeSidebar(event);},true);
document.addEventListener('keydown',event=>{
 if(event.target.id==='terminalSidebarDivider'&&['Enter',' ','ArrowLeft','ArrowRight','Home','End'].includes(event.key)){
  event.preventDefault();event.stopImmediatePropagation();
  if(event.key==='Enter'||event.key===' '){setCollapsed(!state.terminalSidebarCollapsed);return;}
  if(state.terminalSidebarCollapsed)setCollapsed(false);
  const {minimum,maximum}=sidebarWidthLimits(),current=Number(event.target.getAttribute('aria-valuenow')),width=event.key==='Home'?300:event.key==='End'?maximum:current+(event.key==='ArrowLeft'?(sidebarSide()==='left'?-1:1):(sidebarSide()==='left'?1:-1))*(event.shiftKey?40:10);
  state.terminalSidebarWidth=Math.max(260,Math.min(480,Math.round(Math.max(minimum,Math.min(maximum,width)))));layoutSidebarSize();saveState();return;
 }
 const tab=event.target.closest?.('[data-session-tab]');
 if(tab&&['ArrowUp','ArrowDown','Home','End','Enter',' '].includes(event.key)){
  event.preventDefault();event.stopImmediatePropagation();
  const available=sidebarTabs.filter(name=>(!restricted()||name==='macros')&&(name!=='files'||root.NetworkToolkitTerminalSFTP?.available?.(typeof currentTerminal==='function'?currentTerminal():null))),at=available.indexOf(tab.dataset.sessionTab),next=event.key==='Home'?available[0]:event.key==='End'?available[available.length-1]:event.key==='ArrowUp'?available[(at+available.length-1)%available.length]:event.key==='ArrowDown'?available[(at+1)%available.length]:tab.dataset.sessionTab;
  activateTab(next,{toggle:event.key==='Enter'||event.key===' '});byId('terminalSidebarTab'+tabTitle(next))?.focus({preventScroll:true});return;
 }
 const entry=event.target.closest?.('[data-session-entry]');
 if(entry&&(event.key==='ContextMenu'||event.shiftKey&&event.key==='F10')){event.preventDefault();event.stopImmediatePropagation();const rect=entry.getBoundingClientRect();openMenu(eventTarget(entry),rect.left,rect.bottom);return;}
 if(entry&&event.key==='Enter'){event.preventDefault();event.stopImmediatePropagation();void open(eventTarget(entry)).catch(error=>toast(error.message,'error'));return;}
 if(!menu)return;
 if(event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();closeMenu(true);return;}
 if(event.key==='Tab'){closeMenu();return;}
 if(['ArrowUp','ArrowDown','Home','End'].includes(event.key)){event.preventDefault();event.stopImmediatePropagation();const buttons=[...menu.querySelectorAll('button:not(:disabled)')],at=buttons.indexOf(document.activeElement),index=event.key==='Home'?0:event.key==='End'?buttons.length-1:(at+(event.key==='ArrowUp'?-1:1)+buttons.length)%buttons.length;buttons[index]?.focus({preventScroll:true});}
},true);
root.addEventListener('resize',()=>{closeMenu();layoutSidebarSize();});
root.NetworkToolkitSessionSidebar=Object.freeze({mount,ensure,render,activateTab,activeTab,setCollapsed,record,preferences,displayPreferences,remove,clearRecent,save,open,connect,run,token,menuItems});
})(window);
