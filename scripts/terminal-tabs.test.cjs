'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test'),{document:makeDocument,Element}=require('./terminal-dom-fixture.cjs');
function harness(){
 const calls=[],revisions=new Map([['one',0],['two',0]]),entries=new Map();for(const id of ['one','two'])entries.set(id,{info:{id,label:id,state:'connected'},xterm:{hasSelection:()=>true,selectAll:()=>calls.push(['select',id])}});
 const document=makeDocument('<div id="terminalTabs"><div class="terminal-tab"><button data-terminal-tab="one">one</button></div><div class="terminal-tab"><button data-terminal-tab="two">two</button></div></div>');document.body=document;
 const window={addEventListener(){},NetworkToolkitTerminalCopy:{withInputSuppressed:(_e,fn)=>fn()}},extras={current:()=>({}),output:()=>({text:'retained output'}),copy:async e=>calls.push(['copy-output',e.info.id]),select:e=>calls.push(['select-output',e.info.id]),scrollBottom:e=>calls.push(['bottom',e.info.id]),scrollTop(){},clear(){}};
 const context={window,document,terminalSessions:entries,terminalDraftTabs:new Map(),terminalSessionRevision:id=>revisions.get(id),terminalOwnerCanMutate:e=>!e.unowned,terminalReconnectAllowed:e=>!!e&&['connected','closed','error'].includes(e.info.state)&&!e.unowned&&!e.transferBusy&&!e.reconnectBusy&&context.online&&!context.terminalWorkspaceTransferBusy&&context.terminalEntryShown(e),terminalEntryShown:()=>true,terminalLive:e=>e?.info.state==='connected',terminalConnectBusy:false,terminalWorkspaceTransferBusy:false,terminalViewRegistry:{available:true},terminalWorkspaceView:()=>null,terminalViewForSession:()=>null,requestTerminalReattach:async e=>calls.push(['reattach',e.info.id]),terminalSingleDetachReason:()=>'',DETACHED_TERMINAL:false,DETACHED_WORKSPACE:false,TERMINAL_ONLY:false,online:true,CSS:{escape:String},innerHeight:900,innerWidth:1400,esc:String,icon:()=>'',selectTerminal:id=>calls.push(['activate',id]),enqueueTerminalInput:(e,data)=>calls.push(['input',e.info.id,data]),reconnectTerminal:async e=>calls.push(['reconnect',e.info.id]),disconnectTerminal:async e=>calls.push(['disconnect',e.info.id]),NetworkToolkitTerminalExtras:extras,NetworkToolkitTerminalInteraction:{copy:async e=>calls.push(['copy',e.info.id]),paste:async e=>calls.push(['paste',e.info.id])},detachTerminal:async e=>calls.push(['detach',e.info.id]),renderTerminalChrome(){},toast(){},openModal(title,body,foot){calls.push(['modal',title]);const modal=new Element('section',{id:'fixtureModal'});modal.innerHTML=body+foot;document.append(modal);},closeModal(){document.getElementById('fixtureModal')?.remove();},saveState(){calls.push(['save']);},deleteTerminalTab:async(entry,disconnect)=>{calls.push(['delete',entry.info.id,disconnect]);entries.delete(entry.info.id);},closeTerminalDraft:id=>{calls.push(['draft-close',id]);context.terminalDraftTabs.delete(id);},state:{}};
 vm.runInNewContext(fs.readFileSync(require.resolve('../web/assets/terminal-tabs.js'),'utf8'),context);return {api:window.NetworkToolkitTerminalTabs,document,entries,revisions,calls,context};
}
test('right clicking an inactive terminal tab opens all tab actions without changing the active terminal',async()=>{
 const h=harness(),tab=h.document.querySelector('[data-terminal-tab="two"]');h.document.emit('contextmenu',tab,{clientX:80,clientY:90});const menu=h.document.querySelector('#terminalTabContextMenu');assert(menu);for(const action of ['rename','duplicate','move','workspace','reconnect','reconnect-all','disconnect','close','close-others','close-disconnected','clear','copy-output'])assert(menu.querySelector('[data-terminal-tab-action="'+action+'"]'),action);assert.deepEqual(h.calls,[]);await h.api.run('control-c',h.api.token('two'));assert.deepEqual(h.calls,[['activate','two'],['input','two','\x03']]);
});
test('tab actions retain their target and reject stale ownership or disconnected input',async()=>{
 const h=harness(),t=h.api.token('two');h.revisions.set('two',1);await h.api.run('disconnect',t);assert.deepEqual(h.calls,[]);const e=h.entries.get('two');e.info.state='closed';const current=h.api.token('two');assert.equal(h.api.enabled('paste',current),false);assert.equal(h.api.enabled('reconnect',current),true);await h.api.run('reconnect',current);assert.deepEqual(h.calls,[['reconnect','two']]);e.unowned=true;assert.equal(h.api.enabled('close',h.api.token('two')),false);assert.equal(h.api.enabled('disconnect',h.api.token('two')),false);
});
test('session context action detaches one pane even while another workspace is linked',async()=>{
 const h=harness();h.context.terminalWorkspaceView=()=>({workspace:true,id:'other-workspace'});
 await h.api.run('move',h.api.token('two'));assert.deepEqual(h.calls,[['detach','two']]);
 h.context.DETACHED_TERMINAL=true;h.context.DETACHED_WORKSPACE=true;await h.api.run('move',h.api.token('one'));assert.deepEqual(h.calls,[['detach','two'],['detach','one']]);
});

test('SSH terminal tab can launch shared SFTP file browser without reconnecting',async()=>{
 const h=harness(),entry=h.entries.get('one');entry.info.type='ssh';
 h.context.window.NetworkToolkitRemoteBrowser={useTerminalSession:id=>h.calls.push(['browse-files',id])};
 h.context.navigate=page=>h.calls.push(['navigate',page]);
 const token=h.api.token('one');assert.equal(h.api.enabled('browse',token),true);
 await h.api.run('browse',token);
 assert.deepEqual(h.calls,[['browse-files','one'],['navigate','sftp']]);assert.equal(h.context.state.transferModes.sftp,'client');
 entry.info.state='closed';assert.equal(h.api.enabled('browse',h.api.token('one')),false);
});

test('close other tabs confirms by default and skips sessions whose ownership changed during review',async()=>{
 const h=harness();await h.api.run('close-others',h.api.token('one'));
 assert.deepEqual(h.calls,[['modal','Close terminal tabs']]);assert(h.document.getElementById('terminalBulkCloseDontAsk'));
 h.revisions.set('two',1);h.document.emit('click',h.document.querySelector('[data-terminal-tab-confirm]'));await new Promise(resolve=>setImmediate(resolve));
 assert(h.entries.has('two'));assert.equal(h.context.state.terminalConfirmClose,undefined);
});
test('close other tabs remembers opt-out only after confirmation and disconnects before closing',async()=>{
 const h=harness();await h.api.run('close-others',h.api.token('one'));h.document.getElementById('terminalBulkCloseDontAsk').checked=true;
 h.document.emit('click',h.document.querySelector('[data-terminal-tab-confirm]'));await new Promise(resolve=>setImmediate(resolve));
 assert.equal(h.context.state.terminalConfirmClose,false);assert.deepEqual(h.calls,[['modal','Close terminal tabs'],['save'],['delete','two',true]]);assert(h.entries.has('one'));
});
test('disabled close confirmation closes eligible active and disconnected sessions and drafts without a modal',async()=>{
 const h=harness();h.context.state.terminalConfirmClose=false;
 const unowned={info:{id:'unowned',state:'connected'},unowned:true},busy={info:{id:'busy',state:'connected'},transferBusy:true},closed={info:{id:'closed',state:'closed'}};
 h.entries.set('unowned',unowned);h.entries.set('busy',busy);h.entries.set('closed',closed);h.context.terminalDraftTabs.set('draft',{label:'New'});
 await h.api.run('close-others',h.api.token('one'));
 assert.deepEqual(h.calls,[['delete','two',true],['delete','closed',false],['draft-close','draft']]);assert(h.entries.has('unowned'));assert(h.entries.has('busy'));assert(h.entries.has('one'));
});

test('Disconnect all confirms current-window scope and retains tabs/output',async()=>{
 const h=harness(),one=h.entries.get('one'),two=h.entries.get('two');h.context.terminalEntryShown=e=>e!==two;
 await h.api.run('disconnect-all',h.api.token('one'));assert.deepEqual(h.calls,[['modal','Disconnect all sessions in this window']]);assert.equal(h.entries.size,2);
 await h.api.confirmWindowBulk();assert.deepEqual(h.calls,[['modal','Disconnect all sessions in this window'],['disconnect','one']]);assert.equal(h.entries.get('one'),one);assert.equal(h.entries.get('two'),two);
});
test('Close all tabs confirms even when single-tab close confirmation is disabled, skips stale/busy and other windows',async()=>{
 const h=harness();h.context.state.terminalConfirmClose=false;const busy={info:{id:'busy',state:'connected'},transferBusy:true},remote={info:{id:'remote',state:'connected'}};h.entries.set('busy',busy);h.entries.set('remote',remote);h.context.terminalEntryShown=e=>e!==remote;h.context.terminalDraftTabs.set('draft',{id:'draft',label:'Untitled'});
 await h.api.run('close-all',h.api.token('one'));assert.deepEqual(h.calls,[['modal','Close all tabs in this window']]);h.revisions.set('two',1);await h.api.confirmWindowBulk();
 assert.deepEqual(h.calls,[['modal','Close all tabs in this window'],['delete','one',true],['draft-close','draft']]);assert(h.entries.has('two'));assert(h.entries.has('remote'));assert(h.entries.has('busy'));
});
test('bulk cancellation has no side effect and a pending ownership transfer disables all-window actions',async()=>{
 const h=harness();await h.api.run('close-all',h.api.token('one'));h.context.closeModal();assert.equal(h.entries.size,2);assert.equal(h.calls.length,1);h.context.terminalWorkspaceTransferBusy=true;assert.equal(h.api.enabled('close-all',h.api.token('one')),false);assert.equal(h.api.enabled('disconnect-all',h.api.token('one')),false);
});

test('cancelled or replaced bulk confirmation cannot execute retained targets',async()=>{const h=harness();await h.api.run('close-all',h.api.token('one'));h.context.closeModal();await h.api.confirmWindowBulk();assert.equal(h.entries.size,2);assert.deepEqual(h.calls,[['modal','Close all tabs in this window']]);});
test('a transfer becoming busy during bulk disconnect protects later draft tabs',async()=>{const h=harness();h.context.terminalDraftTabs.set('draft',{id:'draft'});h.context.deleteTerminalTab=async e=>{h.calls.push(['delete',e.info.id]);h.context.terminalWorkspaceTransferBusy=true;};await h.api.run('close-all',h.api.token('one'));await h.api.confirmWindowBulk();assert(h.context.terminalDraftTabs.has('draft'));assert.deepEqual(h.calls,[['modal','Close all tabs in this window'],['delete','one']]);});

test('128 manual rename waits for authoritative response, blocks duplicate submission and respects changed ownership',async()=>{
 const h=harness(),entry=h.entries.get('one');entry.info.type='ssh';let finish;const requests=[];h.context.api=async(path,method,body)=>{requests.push({path,method,body});return new Promise(resolve=>finish=resolve);};h.context.upsertTerminalSession=info=>{entry.info=info;};h.context.startTerminalFeed=()=>h.calls.push(['feed']);
 await h.api.run('rename',h.api.token('one'));let form=h.document.getElementById('terminalTabRenameForm');h.document.getElementById('terminalTabName').value='Manual east';h.revisions.set('one',1);h.document.emit('submit',form);assert.equal(requests.length,0);h.context.closeModal();
 await h.api.run('rename',h.api.token('one'));form=h.document.getElementById('terminalTabRenameForm');h.document.getElementById('terminalTabName').value='Manual east';h.document.emit('submit',form);h.document.emit('submit',form);assert.equal(requests.length,1);assert.equal(entry.info.label,'one');
 h.context.closeModal();h.context.openModal('Other task','<p>Keep me</p>','');finish({session:{...entry.info,label:'Manual east',manualName:'Manual east',nameExplicit:true,nameRevision:7,labelSource:'manual'}});await new Promise(resolve=>setImmediate(resolve));assert.equal(entry.info.nameRevision,7);assert(h.document.getElementById('fixtureModal'));assert(h.calls.some(c=>c[0]==='feed'));
});
test('128 clear override is an explicit action, not an empty ordinary rename',async()=>{
 const h=harness(),entry=h.entries.get('one');entry.info.type='ssh';const requests=[];h.context.api=async(path,method,body)=>{requests.push({path,method,body});return {session:{...entry.info,label:'SSH 192.0.2.23:22',manualName:'',nameExplicit:false,nameRevision:8,labelSource:'default'}};};h.context.upsertTerminalSession=info=>{entry.info=info;};h.context.startTerminalFeed=()=>{};
 await h.api.run('rename',h.api.token('one'));const form=h.document.getElementById('terminalTabRenameForm');h.document.getElementById('terminalTabName').value='';h.document.emit('submit',form);assert.equal(requests.length,0);h.document.emit('click',h.document.querySelector('[data-terminal-clear-name]'));await new Promise(resolve=>setImmediate(resolve));assert.equal(requests.length,1);assert.equal(requests[0].body.label,'');assert.equal(requests[0].body.clear,true);assert.equal(entry.info.nameExplicit,false);assert.equal(entry.info.nameRevision,8);
});


test('tab right-click Reconnect all asks before reconnecting every eligible window session',async()=>{
 const h=harness();
 h.entries.get('one').info.state='closed';
 h.entries.get('two').info.state='error';
 const menuTab=h.document.querySelector('[data-terminal-tab="one"]');
 h.document.emit('contextmenu',menuTab,{clientX:90,clientY:90});
 const button=h.document.querySelector('[data-terminal-tab-action="reconnect-all"]');
 assert(button&&!button.disabled);
 await h.api.run('reconnect-all',h.api.token('one'));
 assert.deepEqual(h.calls,[['modal','Reconnect all terminal sessions']]);
 assert.equal(h.document.querySelector('[data-terminal-reconnect-all-confirm]').disabled,false);
 await h.api.confirmReconnectAll();
 assert.deepEqual(h.calls,[['modal','Reconnect all terminal sessions'],['reconnect','one'],['reconnect','two']]);
 assert.equal(h.entries.get('one').info.state,'closed','bulk reconnect does not remove retained tabs');
});

test('Reconnect all includes connected sessions but excludes other windows and busy or unowned sessions',async()=>{
 const h=harness();
 const connected=h.entries.get('one'),closed=h.entries.get('two');
 closed.info.state='closed';
 const detached={info:{id:'detached',state:'closed'}};
 const unowned={info:{id:'unowned',state:'connected'},unowned:true};
 const transfer={info:{id:'transfer',state:'closed'},transferBusy:true};
 const pending={info:{id:'pending',state:'awaiting_input'}};
 h.entries.set('detached',detached);h.entries.set('unowned',unowned);h.entries.set('transfer',transfer);h.entries.set('pending',pending);
 h.context.terminalEntryShown=entry=>entry!==detached;
 assert.deepEqual(Array.from(h.api.reconnectTargets(),t=>t.id),['one','two']);
 assert.equal(h.api.reconnectAllAvailable(),true);
 assert.equal(h.api.requestReconnectAll(),true);
 const modal=h.document.querySelector('#fixtureModal');
 assert.match(modal.textContent,/1 currently connected session/);
 h.revisions.set('two',1);
 await h.api.confirmReconnectAll();
 assert.deepEqual(h.calls,[['modal','Reconnect all terminal sessions'],['reconnect','one']]);
 assert.equal(connected.info.state,'connected');
 assert(h.entries.has('detached'));
});

test('Reconnect all cancellation and ownership transfer do not send any reconnect requests',async()=>{
 const h=harness();
 h.api.requestReconnectAll();
 h.context.closeModal();
 assert.equal(await h.api.confirmReconnectAll(),false);
 assert.deepEqual(h.calls,[['modal','Reconnect all terminal sessions']]);
 h.context.terminalWorkspaceTransferBusy=true;
 assert.equal(h.api.reconnectAllAvailable(),false);
 assert.equal(h.api.requestReconnectAll(),false);
 h.context.terminalWorkspaceTransferBusy=false;
 h.api.requestReconnectAll();
 h.entries.get('one').unowned=true;h.entries.get('two').transferBusy=true;
 assert.equal(await h.api.confirmReconnectAll(),false);
 assert.equal(h.calls.filter(c=>c[0]==='reconnect').length,0);
});

test('bulk reconnection is serialized, guarded from duplicate starts and pauses for authentication',async()=>{
 const h=harness();let releaseFirst;const calls=h.calls;
 h.context.reconnectTerminal=async entry=>{
  calls.push(['reconnect',entry.info.id]);
  if(entry.info.id==='one')await new Promise(resolve=>releaseFirst=resolve);
 };
 h.api.requestReconnectAll();
 const task=h.api.confirmReconnectAll();
 assert.equal(h.api.reconnectAllBusy(),true);
 assert.equal(h.api.reconnectAllAvailable(),false);
 assert.equal(h.api.requestReconnectAll(),false);
 releaseFirst();
 await task;
 assert.deepEqual(calls.filter(c=>c[0]==='reconnect'),[['reconnect','one'],['reconnect','two']]);
 h.context.reconnectTerminal=async entry=>{
  calls.push(['reconnect-auth',entry.info.id]);
  if(entry.info.id==='one'){
   const form=new Element('form',{id:'terminalConnectForm'});h.document.append(form);
  }
 };
 h.api.requestReconnectAll();
 await h.api.confirmReconnectAll();
 assert.deepEqual(calls.filter(c=>c[0]==='reconnect-auth'),[['reconnect-auth','one']]);
 assert.equal(h.api.reconnectAllBusy(),false);
});

test('Reconnect all can remember a scoped confirmation opt-out and restore the prompt independently',async()=>{
 const h=harness();
 assert.equal(h.api.requestReconnectAll(),true);
 assert.deepEqual(h.calls,[['modal','Reconnect all terminal sessions']]);
 assert.equal(h.context.state.terminalConfirmReconnectAll,undefined);
 const skip=h.document.getElementById('terminalReconnectAllDontAsk');assert(skip);skip.checked=true;
 await h.api.confirmReconnectAll();
 assert.equal(h.context.state.terminalConfirmReconnectAll,false);
 assert.equal(h.calls.filter(x=>x[0]==='save').length,1);
 assert.deepEqual(h.calls.filter(x=>x[0]==='reconnect').map(x=>x[1]),['one','two']);
 assert.equal(h.api.requestReconnectAll(),true);
 await new Promise(resolve=>setImmediate(resolve));
 assert.equal(h.calls.filter(x=>x[0]==='modal').length,1);
 assert.deepEqual(h.calls.filter(x=>x[0]==='reconnect').map(x=>x[1]),['one','two','one','two']);
 h.context.state.terminalConfirmReconnectAll=true;
 assert.equal(h.api.requestReconnectAll(),true);
 assert.equal(h.calls.filter(x=>x[0]==='modal').length,2);
 assert.equal(h.context.state.terminalConfirmDisconnectAll,undefined);
});

test('Disconnect all exposes a confirmation choice independent of Close all and Reconnect all',async()=>{
 const h=harness();
 assert.equal(h.api.disconnectAllAvailable(),true);
 assert.equal(h.api.bulkWindow('disconnect-all'),true);
 assert.deepEqual(h.calls,[['modal','Disconnect all sessions in this window']]);
 assert.equal(h.context.state.terminalConfirmDisconnectAll,undefined);
 const skip=h.document.getElementById('terminalWindowBulkDontAsk');assert(skip);skip.checked=true;
 await h.api.confirmWindowBulk();
 assert.equal(h.context.state.terminalConfirmDisconnectAll,false);
 assert.equal(h.calls.filter(x=>x[0]==='save').length,1);
 assert.deepEqual(h.calls.filter(x=>x[0]==='disconnect').map(x=>x[1]),['one','two']);
 assert.equal(h.api.bulkWindow('disconnect-all'),true);
 await new Promise(resolve=>setImmediate(resolve));
 assert.equal(h.calls.filter(x=>x[0]==='modal').length,1);
 assert.deepEqual(h.calls.filter(x=>x[0]==='disconnect').map(x=>x[1]),['one','two','one','two']);
 assert.equal(h.context.state.terminalConfirmCloseAll,undefined);
 h.context.state.terminalConfirmDisconnectAll=true;
 assert.equal(h.api.bulkWindow('disconnect-all'),true);
 assert.equal(h.calls.filter(x=>x[0]==='modal').length,2);
});

test('Close all optional confirmation does not disable single active tab close confirmation',async()=>{
 const h=harness();
 assert.equal(h.api.bulkWindow('close-all'),true);
 assert.deepEqual(h.calls,[['modal','Close all tabs in this window']]);
 const opt=h.document.getElementById('terminalWindowBulkDontAsk');assert(opt);opt.checked=true;
 await h.api.confirmWindowBulk();
 assert.equal(h.context.state.terminalConfirmCloseAll,false);
 assert.notEqual(h.context.state.terminalConfirmClose,false);
 assert.equal(h.entries.size,0);
 h.entries.set('three',{info:{id:'three',state:'connected',label:'three'}});
 assert.equal(h.api.bulkWindow('close-all'),true);
 await new Promise(resolve=>setImmediate(resolve));
 assert.equal(h.calls.filter(x=>x[0]==='modal').length,1);
 assert.equal(h.entries.has('three'),false);
 h.entries.set('four',{info:{id:'four',state:'connected',label:'four'}});
 h.context.state.terminalConfirmCloseAll=true;
 assert.equal(h.api.bulkWindow('close-all'),true);
 assert.equal(h.calls.filter(x=>x[0]==='modal').length,2);
});

test('skipped Disconnect all confirmation still respects ownership changes and blocks duplicate batches',async()=>{
 const h=harness();
 h.context.state.terminalConfirmDisconnectAll=false;
 let release;
 h.context.disconnectTerminal=async entry=>{h.calls.push(['disconnect',entry.info.id]);if(entry.info.id==='one')await new Promise(resolve=>release=resolve);};
 assert.equal(h.api.bulkWindow('disconnect-all'),true);
 assert.equal(h.api.windowBulkBusy(),true);
 assert.equal(h.api.disconnectAllAvailable(),false);
 assert.equal(h.api.bulkWindow('disconnect-all'),false);
 h.revisions.set('two',1);
 release();
 await new Promise(resolve=>setImmediate(resolve));
 assert.equal(h.api.windowBulkBusy(),false);
 assert.deepEqual(h.calls,[['disconnect','one']]);
 assert.equal(h.calls.filter(x=>x[0]==='modal').length,0);
});

test('Disconnect all remains unavailable when offline or while a workspace moves',()=>{
 const h=harness();h.context.online=false;
 assert.equal(h.api.disconnectAllAvailable(),false);
 assert.equal(h.api.bulkWindow('disconnect-all'),false);
 h.context.online=true;h.context.terminalWorkspaceTransferBusy=true;
 assert.equal(h.api.disconnectAllAvailable(),false);
 assert.equal(h.api.bulkWindow('disconnect-all'),false);
 h.context.terminalWorkspaceTransferBusy=false;
 h.entries.get('one').info.state='closed';h.entries.get('two').info.state='error';
 assert.equal(h.api.disconnectAllAvailable(),false);
});
