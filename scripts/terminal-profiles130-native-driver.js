/* Isolated synthetic profile130 native acceptance; never embedded. */
(function(){
'use strict';
const profile=()=>state.terminalProfiles.find(p=>p.id==='native-profile114');
const baseline=new Map(),bootAt=Date.now();let preferencesReads=0;
const identity=p=>JSON.stringify(Object.fromEntries(Object.entries(safeTerminalConfig(p)).sort(([a],[b])=>a.localeCompare(b))));
function badge(node){const b=node?.querySelector('.terminal-protocol-badge'),r=b?.getBoundingClientRect();return {text:b?.textContent||'',visible:!!(r&&r.width>0&&r.height>0),noninteractive:!!b&&b.tagName==='SPAN'&&!b.hasAttribute('tabindex')};}
window.__nativeProfiles130Snapshot=function(e){const p=profile(),tab=document.querySelector('#terminalTabs [data-terminal-tab="'+e?.info.id+'"]'),saved=document.querySelector('[data-session-kind="saved"][data-session-id="native-profile114"]'),recent=[...document.querySelectorAll('[data-session-kind="recent"]')].find(n=>n.querySelector('.terminal-protocol-badge')?.textContent==='Telnet');return {bootAt,preferencesReads,storedName:p?.name||'',explicit:p?.nameExplicit===true,pending:!!p?._namePending,profileId:p?.id||'',identityUnchanged:!baseline.has('profile')||identity(p)===baseline.get('profile'),tabBadge:badge(tab),savedBadge:badge(saved),recentBadge:badge(recent)};};
window.__nativeProfiles130=async function(c,snapshot,entry,wait){
 const action=c.action.slice('profiles130:'.length);
 if(action==='baseline'){baseline.set('profile',identity(profile()));NetworkToolkitSessionSidebar.activateTab('saved',{expand:true});}
 else if(action==='save-empty'||action==='save-manual'){
  await NetworkToolkitSessionSidebar.run('edit',NetworkToolkitSessionSidebar.token('saved','native-profile114'));
  const input=await wait(()=>document.getElementById('terminalProfileName'),'real connection profile form');input.value=action==='save-manual'?'Native manual130':'';
  document.querySelector('[data-terminal-action="save-profile"]').click();
  await wait(()=>profile()&&profile().nameExplicit===(action==='save-manual')&&!profile()._namePending,'persisted profile name acknowledgement');
  document.querySelector('.modal [data-action="close-modal"]').click();
 }
 else if(action==='saved')NetworkToolkitSessionSidebar.activateTab('saved',{expand:true});
 else if(action==='recent')NetworkToolkitSessionSidebar.activateTab('recent',{expand:true});
 else if(action==='reload-preferences'){
  // Native host intentionally blocks document reloads; use authenticated persisted preferences.
  const saved=await api('/state');mergeTerminalPreferences(saved);renderTerminalProfiles();renderTerminalChrome();preferencesReads++;
 }
 else if(action==='reconnect'){
  const e=entry(),disconnect=[...document.querySelectorAll('[data-terminal-action="disconnect"]')].find(n=>!n.disabled&&n.getBoundingClientRect().width>0);if(!disconnect)throw Error('Visible enabled disconnect absent');disconnect.click();await wait(()=>['closed','error'].includes(e.info.state),'explicit disconnect');
  const b=[...document.querySelectorAll('[data-terminal-action="reconnect"]')].find(n=>!n.disabled&&n.getBoundingClientRect().width>0);if(!b)throw Error('Visible enabled reconnect absent');b.click();await wait(()=>e.info.state==='connected'&&!e.reconnectBusy,'same-session reconnect');
 }
 else throw Error('Unknown profile130 fixture action');
 return snapshot();
};
})();
