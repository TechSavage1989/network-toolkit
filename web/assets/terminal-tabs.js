(function(root){
'use strict';
let menu=null,target=null,pending=[],pendingBulk=null,pendingReconnect=null,reconnectAllBusy=false,windowBulkBusy=false,bulkSequence=0,pendingRename=null,renameBusy=false;
const items=[['reconnect','Reconnect','refresh'],['reconnect-all','Reconnect all…','refresh'],['disconnect','Disconnect','stop'],['browse','Browse SFTP files','folder'],['editor','Open Text Editor','fileEdit'],['control-c','Send Ctrl+C','stop'],['copy','Copy selection','copy'],['paste','Paste into console','clipboard'],['select-all','Select all output','terminal'],['select-output','Select last command output','terminal'],['copy-output','Copy last command output','copy'],['scroll-top','Scroll to top','arrowUp'],['scroll-bottom','Scroll to bottom','arrowDown'],['search','Find in output','search'],['clear','Clear scrollback','trash'],['export','Export transcript','download'],['save-connection','Save connection...','save'],['rename','Rename tab','edit'],['duplicate','Duplicate connection settings','plus'],['move','Detach / reattach session','external'],['workspace','Detach / reattach workspace','maximize'],['copy-connection','Copy connection details','copy'],['close','Close tab','close'],['close-others','Close other tabs','close'],['disconnect-all','Disconnect all sessions in this window…','stop'],['close-all','Close all tabs in this window…','close'],['close-disconnected','Close disconnected tabs','trash']];
function token(id){const entry=terminalSessions.get(id),draft=terminalDraftTabs.get(id);return entry||draft?{id,entry,draft,revision:entry?terminalSessionRevision(id):0}:null;}
function valid(t){return !!t&&(t.entry?terminalSessions.get(t.id)===t.entry&&terminalSessionRevision(t.id)===t.revision:terminalDraftTabs.get(t.id)===t.draft);}
function enabled(action,t){if(!valid(t))return false;const entry=t.entry,owned=entry&&terminalOwnerCanMutate(entry)&&!entry.transferBusy;
 if(action==='reconnect-all')return reconnectAllAvailable();
 if(action==='disconnect-all')return disconnectAllAvailable();
 if(action==='close-all')return !windowBulkBusy&&windowTargets().length>0;
 if(action==='close')return !!t.draft||!!owned;
 if(action==='close-others')return [...terminalSessions.values()].some(e=>e!==entry&&terminalOwnerCanMutate(e)&&!e.transferBusy)||[...terminalDraftTabs.keys()].some(id=>id!==t.id);
 if(action==='close-disconnected')return [...terminalSessions.values()].some(e=>['closed','error'].includes(e.info.state)&&terminalOwnerCanMutate(e)&&!e.transferBusy);
 if(action==='rename')return !!t.draft||!!owned;
 if(action==='duplicate')return !(DETACHED_TERMINAL&&!DETACHED_WORKSPACE)&&!terminalConnectBusy&&!terminalWorkspaceTransferBusy;
 if(!entry)return action==='workspace'&&terminalViewRegistry.available;
 if(action==='reconnect')return !reconnectAllBusy&&terminalReconnectAllowed(entry);
 if(action==='disconnect')return !windowBulkBusy&&owned&&terminalLive(entry)&&entry.info.state!=='disconnecting'&&online;
 if(['browse','editor'].includes(action))return !TERMINAL_ONLY&&entry.info.type==='ssh'&&entry.info.state==='connected'&&online;
 if(['paste','control-c'].includes(action))return owned&&entry.info.state==='connected'&&online;
 if(action==='copy')return !!entry.xterm?.hasSelection();
 if(['select-output','copy-output'].includes(action))return !!NetworkToolkitTerminalExtras.output(entry,NetworkToolkitTerminalExtras.current(entry),true)?.text;
 if(action==='move')return !terminalWorkspaceTransferBusy&&!terminalConnectBusy&&(DETACHED_WORKSPACE?!terminalSingleDetachReason(entry)&&terminalViewRegistry.available:!!terminalViewForSession(entry.info.id)||!terminalSingleDetachReason(entry)&&terminalViewRegistry.available);
 if(action==='workspace')return terminalViewRegistry.available&&!terminalWorkspaceTransferBusy&&!terminalConnectBusy;
 return !!entry.xterm;
}
function close(focus=false){setReconnectOptions(false);setDisconnectOptions(false);const old=target;menu?.remove();menu=null;target=null;if(focus&&valid(old))document.querySelector('[data-terminal-tab="'+CSS.escape(old.id)+'"]')?.focus({preventScroll:true});}
function open(id,x,y){close();const t=token(id);if(!t)return;target=t;menu=document.createElement('div');menu.id='terminalTabContextMenu';menu.className='terminal-context-menu terminal-tab-context-menu';menu.setAttribute('role','menu');menu.setAttribute('aria-label','Terminal tab actions');const title=t.entry?.info.label||t.draft?.label||'Terminal';menu.innerHTML='<div class="terminal-context-label">'+esc(title)+'</div>'+items.map(([action,label,ico])=>'<button role="menuitem" type="button" data-terminal-tab-action="'+action+'" '+(enabled(action,t)?'':'disabled')+'>'+icon(ico)+'<span>'+label+'</span></button>').join('');document.body.append(menu);const rect=menu.getBoundingClientRect();menu.style.left=Math.max(6,Math.min(x,innerWidth-rect.width-6))+'px';menu.style.top=Math.max(6,Math.min(y,innerHeight-rect.height-6))+'px';menu.querySelector('button:not(:disabled)')?.focus({preventScroll:true});}
function activate(entry){if(entry)selectTerminal(entry.info.id,true);}
function reconnectTargets(){
 if(reconnectAllBusy||terminalWorkspaceTransferBusy||typeof terminalDetachedReturning!=='undefined'&&terminalDetachedReturning)return [];
 return [...terminalSessions.values()].filter(entry=>terminalReconnectAllowed(entry)).map(entry=>token(entry.info.id)).filter(Boolean);
}
function reconnectAllAvailable(){return reconnectTargets().length>0;}
function setReconnectOptions(open=false){
 const toggle=document.getElementById('terminalReconnectMore'),panel=document.getElementById('terminalReconnectOptions');
 if(!toggle||!panel)return false;
 const visible=!!open&&!toggle.disabled;
 if(visible)setDisconnectOptions(false);
 panel.classList.toggle('hidden',!visible);
 toggle.setAttribute('aria-expanded',String(visible));
 return visible;
}
function toggleReconnectOptions(){
 const panel=document.getElementById('terminalReconnectOptions');
 return setReconnectOptions(!!panel&&panel.classList.contains('hidden'));
}
function setDisconnectOptions(open=false){
 const toggle=document.getElementById('terminalDisconnectMore'),panel=document.getElementById('terminalDisconnectOptions');
 if(!toggle||!panel)return false;
 const visible=!!open&&!toggle.disabled;
 if(visible)setReconnectOptions(false);
 panel.classList.toggle('hidden',!visible);
 toggle.setAttribute('aria-expanded',String(visible));
 return visible;
}
function toggleDisconnectOptions(){
 const panel=document.getElementById('terminalDisconnectOptions');
 return setDisconnectOptions(!!panel&&panel.classList.contains('hidden'));
}
function requestReconnectAll(){
 setReconnectOptions(false);
 const targets=reconnectTargets();if(!targets.length)return false;
 const batch={targets,sequence:++bulkSequence};
 pendingReconnect=null;
 if(state.terminalConfirmReconnectAll===false){
  void executeReconnectAll(batch).catch(error=>toast('Reconnect all stopped: '+error.message,'error'));
  return true;
 }
 pendingReconnect=batch;
 const connected=targets.filter(t=>t.entry.info.state==='connected').length;
 const count=targets.length,confirm='data-terminal-reconnect-all-confirm="'+batch.sequence+'"';
 openModal('Reconnect all terminal sessions',
  '<p>Reconnect '+count+' eligible session'+(count===1?'':'s')+' owned by this window?</p>'+
  (connected?'<div class="notice warn">'+connected+' currently connected session'+(connected===1?' will':'s will')+' restart. Unsent commands and active work in those sessions may be interrupted.</div>':'')+
  '<p>Tabs and retained output stay open. Other detached windows, sessions being transferred and sessions requiring an active connection prompt are excluded.</p>'+
  '<label class="toggle-row mt"><input type="checkbox" id="terminalReconnectAllDontAsk">Do not ask again for Reconnect all</label>',
  '<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button type="button" class="btn primary" '+confirm+'>Reconnect all</button>',true);
 return true;
}
async function executeReconnectAll(batch){
 if(!batch||reconnectAllBusy)return false;
 reconnectAllBusy=true;renderTerminalChrome();
 let attempted=0,halted=false;
 try{
  for(const t of batch.targets){
   if(!valid(t)||!terminalReconnectAllowed(t.entry))continue;
   attempted++;
   await reconnectTerminal(t.entry);
   if(document.getElementById('terminalConnectForm')||t.entry.info.state==='awaiting_input'){halted=true;break;}
  }
 }finally{reconnectAllBusy=false;renderTerminalChrome();}
 if(halted)toast('Reconnect all paused for authentication. Finish signing in before reconnecting other sessions.','info');
 else if(attempted)toast('Reconnect requested for '+attempted+' session'+(attempted===1?'':'s')+'.','info');
 return attempted>0;
}
async function confirmReconnectAll(){
 const batch=pendingReconnect;pendingReconnect=null;
 if(!batch||reconnectAllBusy||!document.querySelector('[data-terminal-reconnect-all-confirm="'+batch.sequence+'"]'))return false;
 if(document.getElementById('terminalReconnectAllDontAsk')?.checked){
  state.terminalConfirmReconnectAll=false;saveState();root.NetworkToolkitTerminalAppearance?.sync?.();
 }
 closeModal();
 return executeReconnectAll(batch);
}
function windowTargets(){
 if(typeof terminalWorkspaceTransferBusy!=='undefined'&&terminalWorkspaceTransferBusy||typeof terminalDetachedReturning!=='undefined'&&terminalDetachedReturning)return [];
 const shown=e=>typeof terminalEntryShown!=='function'||terminalEntryShown(e);
 return [...terminalSessions.values()].filter(e=>shown(e)&&terminalOwnerCanMutate(e)&&!e.transferBusy&&!e.reconnectBusy).map(e=>token(e.info.id)).concat([...terminalDraftTabs.keys()].map(token)).filter(Boolean);
}
function disconnectTargets(){
 if(windowBulkBusy||!online||terminalConnectBusy||terminalWorkspaceTransferBusy||typeof terminalDetachedReturning!=='undefined'&&terminalDetachedReturning)return [];
 return windowTargets().filter(t=>t.entry&&terminalLive(t.entry)&&t.entry.info.state!=='disconnecting');
}
function disconnectAllAvailable(){return disconnectTargets().length>0;}
function bulkConfirmationKey(action){return action==='disconnect-all'?'terminalConfirmDisconnectAll':'terminalConfirmCloseAll';}
function bulkWindow(action){
 setDisconnectOptions(false);
 if(!['disconnect-all','close-all'].includes(action)||windowBulkBusy)return false;
 const targets=action==='disconnect-all'?disconnectTargets():windowTargets();
 if(!targets.length)return false;
 const batch={action,targets,sequence:++bulkSequence};
 pendingBulk=null;
 if(state[bulkConfirmationKey(action)]===false){
  void executeWindowBulk(batch).catch(error=>toast('Bulk terminal action stopped: '+error.message,'error'));
  return true;
 }
 pendingBulk=batch;
 const connected=targets.filter(t=>t.entry&&terminalLive(t.entry)).length,closing=action==='close-all';
 openModal(closing?'Close all tabs in this window':'Disconnect all sessions in this window',
  '<p>'+targets.length+' '+(closing?'tab'+(targets.length===1?'':'s')+' will close.':'live session'+(targets.length===1?' will':'s will')+' disconnect; tabs and output will remain.')+'</p>'+
  '<p>Only eligible sessions owned by this window are included. Other detached windows, saved profiles and busy transfers are not changed.</p>'+
  (connected?'<div class="notice warn">'+connected+' active connection'+(connected===1?' will':'s will')+' be interrupted, including commands or macros using them.</div>':'')+
  '<label class="toggle-row mt"><input type="checkbox" id="terminalWindowBulkDontAsk">Do not ask again for '+(closing?'Close all tabs':'Disconnect all')+'</label>',
  '<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button type="button" class="btn danger" data-terminal-window-bulk-confirm="'+batch.sequence+'">'+(closing?'Close tabs':'Disconnect sessions')+'</button>',true);
 return true;
}
async function executeWindowBulk(batch){
 if(!batch||windowBulkBusy)return false;
 windowBulkBusy=true;renderTerminalChrome();
 let attempted=0;
 try{
  for(const t of batch.targets){
   if(!valid(t)||!windowTargets().some(value=>value.id===t.id&&value.entry===t.entry&&value.draft===t.draft))continue;
   if(t.draft){if(batch.action==='close-all'){closeTerminalDraft(t.id);attempted++;}continue;}
   if(batch.action==='close-all'){await deleteTerminalTab(t.entry,terminalLive(t.entry));attempted++;}
   else if(online&&terminalLive(t.entry)&&t.entry.info.state!=='disconnecting'){await disconnectTerminal(t.entry);attempted++;}
  }
 }finally{windowBulkBusy=false;renderTerminalChrome();}
 return attempted>0;
}
async function confirmWindowBulk(){
 const batch=pendingBulk;pendingBulk=null;
 if(!batch||windowBulkBusy||!document.querySelector('[data-terminal-window-bulk-confirm="'+batch.sequence+'"]'))return false;
 if(document.getElementById('terminalWindowBulkDontAsk')?.checked){
  state[bulkConfirmationKey(batch.action)]=false;saveState();root.NetworkToolkitTerminalAppearance?.sync?.();
 }
 closeModal();
 return executeWindowBulk(batch);
}
async function closeTargets(targets){for(const t of targets){if(!valid(t))continue;if(t.draft)closeTerminalDraft(t.id);else if(terminalOwnerCanMutate(t.entry)&&!t.entry.transferBusy)await deleteTerminalTab(t.entry,terminalLive(t.entry));}}
async function bulk(ids){const targets=ids.map(token).filter(Boolean);if(state.terminalConfirmClose===false){await closeTargets(targets);return;}pending=targets;const live=targets.filter(t=>terminalLive(t.entry)).length;openModal('Close terminal tabs','<p>Close '+targets.length+' selected tab'+(targets.length===1?'':'s')+'?</p>'+(live?'<p>'+live+' active connection'+(live===1?' will':'s will')+' be disconnected. Export any output you need first.</p>':'')+'<label class="toggle-row mt"><input type="checkbox" id="terminalBulkCloseDontAsk">Do not ask again when closing terminal sessions</label>','<button class="btn ghost" data-action="close-modal">Cancel</button><button class="btn danger" data-terminal-tab-confirm>Close tabs</button>',true);}
async function run(action,t){if(!enabled(action,t))return;const entry=t.entry;close();switch(action){
 case'reconnect':await reconnectTerminal(entry);break;case'reconnect-all':requestReconnectAll();break;case'disconnect':await disconnectTerminal(entry);break;case'browse':if(root.NetworkToolkitTerminalSFTP?.available?.(entry)){activate(entry);root.NetworkToolkitSessionSidebar?.activateTab?.('files');}else{root.NetworkToolkitRemoteBrowser?.useTerminalSession(entry.info.id);state.transferModes=state.transferModes||{};state.transferModes.sftp='client';navigate('sftp');}break;
 case'editor':root.NetworkToolkitFileEditor?.preferConnection?.('terminal:'+entry.info.id);navigate('editor');break;
 case'control-c':activate(entry);enqueueTerminalInput(entry,'\x03');break;
 case'copy':await NetworkToolkitTerminalInteraction.copy(entry);break;
 case'paste':activate(entry);await NetworkToolkitTerminalInteraction.paste(entry);break;
 case'select-all':activate(entry);root.NetworkToolkitTerminalCopy?.withInputSuppressed?root.NetworkToolkitTerminalCopy.withInputSuppressed(entry,()=>entry.xterm.selectAll()):entry.xterm.selectAll();break;
 case'select-output':activate(entry);NetworkToolkitTerminalExtras.select(entry);break;
 case'copy-output':await NetworkToolkitTerminalExtras.copy(entry);break;
 case'scroll-top':activate(entry);NetworkToolkitTerminalExtras.scrollTop(entry);break;case'scroll-bottom':activate(entry);NetworkToolkitTerminalExtras.scrollBottom(entry);break;
 case'search':activate(entry);showTerminalSearch();break;
 case'clear':NetworkToolkitTerminalExtras.clear(entry);entry.xterm.clear();entry.outputRevision=(entry.outputRevision||0)+1;break;
 case'export':exportTerminalTranscript(entry);break;
 case'save-connection':root.NetworkToolkitSessionSidebar?.save(entry);break;
 case'rename':pendingRename=t;openModal('Rename terminal tab','<form id="terminalTabRenameForm" data-terminal-id="'+esc(t.id)+'"><div class="field"><label for="terminalTabName">Tab name</label><input id="terminalTabName" maxlength="100" value="'+esc(entry?.info.label||t.draft?.label||'Terminal')+'" required autocomplete="off"></div></form>','<button class="btn ghost" data-action="close-modal">Cancel</button><button class="btn primary" type="submit" form="terminalTabRenameForm">Rename</button>'+(['ssh','telnet'].includes(entry?.info.type)?'<button class="btn ghost" type="button" data-terminal-clear-name>Use device name / IP</button>':''),true);break;
 case'duplicate':{const config=entry?safeTerminalConfig(entry.config):safeTerminalConfig(state.terminalConnection);newTerminalTab();openTerminalConnect(config);break;}
 case'move':if(DETACHED_WORKSPACE)await detachTerminal(entry);else if(terminalViewForSession(entry.info.id)||DETACHED_TERMINAL)await requestTerminalReattach(entry);else await detachTerminal(entry);break;
 case'workspace':if(DETACHED_TERMINAL||terminalWorkspaceView(entry))await requestTerminalReattach(entry);else await detachTerminalWorkspace();break;
 case'copy-connection':{const i=entry.info;await NetworkToolkitTerminalInteraction.copyValue([terminalProtocolLabel(i),i.host?i.host+':'+i.port:i.serialPort,i.username?'Username: '+i.username:''].filter(Boolean).join('\n'),{entry});break;}
 case'disconnect-all':case'close-all':bulkWindow(action);break;
 case'close':if(t.draft)closeTerminalDraft(t.id);else await removeTerminal(entry);break;
 case'close-others':await bulk([...terminalSessions.values()].filter(e=>e!==entry&&terminalOwnerCanMutate(e)&&!e.transferBusy).map(e=>e.info.id).concat([...terminalDraftTabs.keys()].filter(id=>id!==t.id)));break;
 case'close-disconnected':for(const e of [...terminalSessions.values()].filter(e=>['closed','error'].includes(e.info.state)&&terminalOwnerCanMutate(e)&&!e.transferBusy))await deleteTerminalTab(e,true);break;
 }renderTerminalChrome();}
document.addEventListener('contextmenu',event=>{const tab=event.target.closest('#terminalTabs .terminal-tab');if(!tab)return;const button=tab.querySelector('[data-terminal-tab]');if(!button)return;event.preventDefault();event.stopImmediatePropagation();open(button.dataset.terminalTab,event.clientX,event.clientY);},true);
document.addEventListener('keydown',event=>{
 if(event.key==='Escape'){
  for(const [panelId,toggleId,hide] of [['terminalReconnectOptions','terminalReconnectMore',setReconnectOptions],['terminalDisconnectOptions','terminalDisconnectMore',setDisconnectOptions]]){
   const panel=document.getElementById(panelId);
   if(panel&&!panel.classList.contains('hidden')){event.preventDefault();event.stopImmediatePropagation();hide(false);document.getElementById(toggleId)?.focus?.({preventScroll:true});return;}
  }
 }
 const tab=event.target.closest?.('#terminalTabs [data-terminal-tab]');if(tab&&(event.key==='ContextMenu'||event.shiftKey&&event.key==='F10')){event.preventDefault();event.stopImmediatePropagation();const r=tab.getBoundingClientRect();open(tab.dataset.terminalTab,r.left,r.bottom);return;}if(!menu)return;if(event.key==='Escape'){event.preventDefault();close(true);return;}if(event.key==='Tab'){close();return;}if(['ArrowUp','ArrowDown','Home','End'].includes(event.key)){event.preventDefault();const list=[...menu.querySelectorAll('button:not(:disabled)')],at=list.indexOf(document.activeElement),index=event.key==='Home'?0:event.key==='End'?list.length-1:(at+(event.key==='ArrowUp'?-1:1)+list.length)%list.length;list[index]?.focus({preventScroll:true});}},true);
document.addEventListener('pointerdown',event=>{if(menu&&!menu.contains(event.target))close();if(!event.target.closest?.('.terminal-reconnect-split'))setReconnectOptions(false);if(!event.target.closest?.('.terminal-disconnect-split'))setDisconnectOptions(false);},true);
document.addEventListener('click',event=>{if(event.target.closest('[data-terminal-reconnect-all-confirm]')){event.preventDefault();event.stopImmediatePropagation();void confirmReconnectAll().catch(e=>toast(e.message,'error'));return;}if(event.target.closest('[data-terminal-window-bulk-confirm]')){event.preventDefault();event.stopImmediatePropagation();void confirmWindowBulk().catch(e=>toast(e.message,'error'));return;}const action=event.target.closest('[data-terminal-tab-action]');if(action){event.preventDefault();event.stopImmediatePropagation();if(!action.disabled)void run(action.dataset.terminalTabAction,target).catch(e=>toast(e.message,'error'));return;}const confirm=event.target.closest('[data-terminal-tab-confirm]');if(confirm){event.preventDefault();const targets=pending;pending=[];if(document.getElementById('terminalBulkCloseDontAsk')?.checked){state.terminalConfirmClose=false;saveState();root.NetworkToolkitTerminalAppearance?.sync?.();}closeModal();void closeTargets(targets).catch(e=>toast(e.message,'error'));}},true);
async function submitName(clear=false){
 const form=document.getElementById('terminalTabRenameForm'),t=pendingRename;
 if(renameBusy||!form||!t||form.dataset.terminalId!==t.id||!enabled('rename',t)){if(form)toast('This session changed. Open Rename again.');return;}
 const label=clear?'':document.getElementById('terminalTabName').value.trim();if(!clear&&(!label||label.length>100))return;
 if(t.draft){if(clear)return;t.draft.label=label;terminalTabsSignature='';pendingRename=null;closeModal();renderTerminalChrome();return;}
 if(clear&&!['ssh','telnet'].includes(t.entry.info.type))return;
 renameBusy=true;for(const button of form.querySelectorAll('button'))button.disabled=true;
 try{const response=await api('/terminals/'+encodeURIComponent(t.id)+'/rename','POST',clear?{label:'',clear:true}:{label});if(!response.session||response.session.id!==t.id)throw new Error('The name response was incomplete. Check the session before trying again.');
  root.NetworkToolkitTerminalPromptLabels?.accept(response.session);upsertTerminalSession(response.session);renderTerminalChrome();
  if(document.getElementById('terminalTabRenameForm')===form){pendingRename=null;closeModal();}startTerminalFeed();
 }catch(e){toast(e.message,'error');}finally{renameBusy=false;for(const button of form.querySelectorAll('button'))button.disabled=false;}
}
document.addEventListener('click',event=>{if(!event.target.closest('[data-terminal-clear-name]'))return;event.preventDefault();event.stopImmediatePropagation();void submitName(true);},true);
document.addEventListener('submit',event=>{if(event.target.id!=='terminalTabRenameForm')return;event.preventDefault();event.stopImmediatePropagation();void submitName(false);},true);
root.addEventListener('resize',()=>close());
root.NetworkToolkitTerminalTabs=Object.freeze({open,close,run,enabled,token,items,windowTargets,reconnectTargets,reconnectAllAvailable,requestReconnectAll,confirmReconnectAll,setReconnectOptions,toggleReconnectOptions,reconnectAllBusy:()=>reconnectAllBusy,disconnectAllAvailable,bulkWindow,windowBulkBusy:()=>windowBulkBusy,setDisconnectOptions,toggleDisconnectOptions,confirmWindowBulk});
})(window);
