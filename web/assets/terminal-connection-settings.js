/* The sidebar and connection dialog share one form and the established
   terminal connection/profile/credential handlers. Drafts stay in memory. */
(function(root){'use strict';
let parked=null,pending=null,building=false;
const target=()=>document.getElementById('terminalSidebarConnection');
const allowed=()=>!(DETACHED_TERMINAL&&!DETACHED_WORKSPACE);
function park(){
 const form=target()?.querySelector('#terminalConnectForm');if(!form)return;
 captureTerminalConnection();
 const container=form.parentElement;
 parked={nodes:[...container.childNodes],draft:safeTerminalConfig(terminalDraft),profile:terminalProfileID,destination:terminalConnectDestination};
 for(const node of parked.nodes)node.remove();
}
function mount(host=target()){
 if(!host||!allowed()||building||state.activePage!=='terminal')return;
 if(host.querySelector('#terminalConnectForm'))return;
 if(document.getElementById('terminalConnectForm'))return;
 building=true;
 try{
  if(pending){const value=pending;pending=null;parked=null;openTerminalConnect(value.config,value.profile,{...value.options,sidebarHost:host});}
  else if(parked){const value=parked;parked=null;terminalDraft=value.draft;terminalProfileID=value.profile;terminalConnectDestination=value.destination;host.replaceChildren(...value.nodes);updateTerminalConnectButton();}
  else openTerminalConnect({},'',{sidebarHost:host});
 }finally{building=false;}
}
function open(config={},profile='',options={}){
 if(!allowed()||state.activePage!=='terminal'||!target())return false;
 pending={config:{...config},profile,options};
 park();root.NetworkToolkitSessionSidebar.activateTab('settings');
 const host=target();if(pending&&host){host.replaceChildren();mount(host);}return true;
}
function sync(){if(state.activePage==='terminal'&&root.NetworkToolkitSessionSidebar?.activeTab?.()==='settings')mount();}
document.addEventListener('click',event=>{
 const button=event.target.closest?.('[data-terminal-settings-current]');if(!button)return;
 event.preventDefault();const entry=currentTerminal();if(!entry){toast('Choose a terminal session first.','info');return;}
 const config=safeTerminalConfig(entry.config||entry.info),profile=(state.terminalProfiles||[]).find(row=>row.id===entry.info.profileId)||null;
 open(config,profile?.id||'');
});
root.NetworkToolkitTerminalConnectionSettings=Object.freeze({mount,park,open,sync});
})(globalThis);
