'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test'),{document:makeDocument,Element}=require('./terminal-dom-fixture.cjs');
function harness(){
 const calls=[],revisions=new Map([['one',0],['two',0]]),entries=new Map();for(const id of ['one','two'])entries.set(id,{info:{id,label:id,state:'connected'},xterm:{hasSelection:()=>true,selectAll:()=>calls.push(['select',id])}});
 const document=makeDocument('<div id="terminalTabs"><div class="terminal-tab"><button data-terminal-tab="one">one</button></div><div class="terminal-tab"><button data-terminal-tab="two">two</button></div></div>');document.body=document;
 const window={addEventListener(){},NetworkToolkitTerminalCopy:{withInputSuppressed:(_e,fn)=>fn()}},extras={current:()=>({}),output:()=>({text:'retained output'}),copy:async e=>calls.push(['copy-output',e.info.id]),select:e=>calls.push(['select-output',e.info.id]),scrollBottom:e=>calls.push(['bottom',e.info.id]),scrollTop(){},clear(){}};
 const context={window,document,terminalSessions:entries,terminalDraftTabs:new Map(),terminalSessionRevision:id=>revisions.get(id),terminalOwnerCanMutate:e=>!e.unowned,terminalReconnectAllowed:e=>e.info.state==='closed',terminalLive:e=>e?.info.state==='connected',terminalConnectBusy:false,terminalWorkspaceTransferBusy:false,terminalViewRegistry:{available:true},terminalWorkspaceView:()=>null,terminalViewForSession:()=>null,requestTerminalReattach:async e=>calls.push(['reattach',e.info.id]),terminalSingleDetachReason:()=>'',DETACHED_TERMINAL:false,DETACHED_WORKSPACE:false,TERMINAL_ONLY:false,online:true,CSS:{escape:String},innerHeight:900,innerWidth:1400,esc:String,icon:()=>'',selectTerminal:id=>calls.push(['activate',id]),enqueueTerminalInput:(e,data)=>calls.push(['input',e.info.id,data]),reconnectTerminal:async e=>calls.push(['reconnect',e.info.id]),disconnectTerminal:async e=>calls.push(['disconnect',e.info.id]),NetworkToolkitTerminalExtras:extras,NetworkToolkitTerminalInteraction:{copy:async e=>calls.push(['copy',e.info.id]),paste:async e=>calls.push(['paste',e.info.id])},detachTerminal:async e=>calls.push(['detach',e.info.id]),renderTerminalChrome(){},toast(){},openModal(title,body,foot){calls.push(['modal',title]);const modal=new Element('section',{id:'fixtureModal'});modal.innerHTML=body+foot;document.append(modal);},closeModal(){document.getElementById('fixtureModal')?.remove();},saveState(){calls.push(['save']);},deleteTerminalTab:async(entry,disconnect)=>{calls.push(['delete',entry.info.id,disconnect]);entries.delete(entry.info.id);},closeTerminalDraft:id=>{calls.push(['draft-close',id]);context.terminalDraftTabs.delete(id);},state:{}};
 vm.runInNewContext(fs.readFileSync(require.resolve('../web/assets/terminal-tabs.js'),'utf8'),context);return {api:window.NetworkToolkitTerminalTabs,document,entries,revisions,calls,context};
}
test('right clicking an inactive terminal tab opens all tab actions without changing the active terminal',async()=>{
 const h=harness(),tab=h.document.querySelector('[data-terminal-tab="two"]');h.document.emit('contextmenu',tab,{clientX:80,clientY:90});const menu=h.document.querySelector('#terminalTabContextMenu');assert(menu);for(const action of ['rename','duplicate','move','workspace','reconnect','disconnect','close','close-others','close-disconnected','clear','copy-output'])assert(menu.querySelector('[data-terminal-tab-action="'+action+'"]'),action);assert.deepEqual(h.calls,[]);await h.api.run('control-c',h.api.token('two'));assert.deepEqual(h.calls,[['activate','two'],['input','two','\x03']]);
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
