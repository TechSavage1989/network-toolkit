'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const source=fs.readFileSync(require.resolve('../web/assets/session-sidebar.js'),'utf8');
function harness(options={}){
 const document=makeDocument('<div id="terminalWorkspace"><div id="terminalWindowRequests"></div><section class="terminal-panel"><div id="terminalCommandBar"></div><div id="existingConsole">Retained transcript</div></section></div><div id="testModal"></div>');document.body=document;
 const calls=[],rootListeners=new Map(),state={terminalProfiles:[],terminalRecent:[],terminalSessionConfirmDelete:true};let serial=0;
 const c={document,state,console,Date,Math,Set,Map,JSON,Number,String,innerWidth:1400,innerHeight:900,CSS:{escape:String},addEventListener(){},esc:value=>String(value).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),icon:()=>'<svg></svg>',terminalProtocolLabel:item=>item.type.toUpperCase(),terminalProfileID:'',terminalConnectBusy:false,terminalWorkspaceTransferBusy:false,terminalDetachedReady:true,terminalDetachedReturning:false,DETACHED_TERMINAL:false,DETACHED_WORKSPACE:false,online:true,OFFLINE:false,terminalAssetsReady:()=>true,terminalSessions:new Map(),terminalDestination:()=>({pane:0,panes:['','','','']}),terminalOwnerCanMutate:entry=>!entry.unowned,terminalReconnectAllowed:entry=>['connected','closed','error'].includes(entry.info.state),saveState:()=>calls.push(['save-state']),renderTerminalProfiles(){},renderTerminalChrome(){},renderTerminalViewControls(){},scheduleTerminalFit(){},startTerminalFeed(){},receiveTerminalViewState(){},placeConnectedTerminal:entry=>{calls.push(['place',entry.info.id]);return 0;},navigate:page=>calls.push(['navigate',page]),selectTerminal:id=>calls.push(['select',id]),reconnectTerminal:async entry=>{calls.push(['reconnect',entry.info.id]);entry.info.state='connected';},toast:(...args)=>calls.push(['toast',...args]),openTerminalConnect:(...args)=>calls.push(['edit',...args]),NetworkToolkitTerminalInteraction:{copyValue:async value=>calls.push(['copy',value])},NetworkToolkitCredentials:{resolve:async config=>{calls.push(['credential',config.credentialId]);return {username:config.username,authMethod:'password',password:'vault-secret'};}},openModal:(title,body,footer)=>{calls.push(['modal',title]);document.getElementById('testModal').innerHTML=body+footer;},closeModal:()=>{document.getElementById('testModal').innerHTML='';},api:async (path,method,body)=>{calls.push(['request',path,method,JSON.parse(JSON.stringify(body))]);return {session:{id:'created-'+(++serial),state:'connecting',type:body.type,host:body.host,port:body.port,username:body.username}};},upsertTerminalSession:info=>{const entry={info,config:{}};c.terminalSessions.set(info.id,entry);return entry;}};
 c.addEventListener=(type,fn)=>{const listeners=rootListeners.get(type)||new Set();listeners.add(fn);rootListeners.set(type,listeners);};c.removeEventListener=(type,fn)=>rootListeners.get(type)?.delete(fn);
 document.dispatchEvent=event=>{calls.push(['sidebar-change',event.detail]);document.emit(event.type,document,{detail:event.detail});return true;};
 c.CustomEvent=class{constructor(type,options){this.type=type;this.detail=options.detail;}};
 Object.assign(c,options.flags||{});c.window=c;vm.createContext(c);
 vm.runInContext(html.split('\n').find(line=>line.startsWith('const TERMINAL_SAFE_FIELDS=')),c);
 for(const name of ['safeTerminalConfig','validateTerminalConfig','terminalConnectPayload'])vm.runInContext(html.split('\n').find(line=>line.startsWith('function '+name+'(')),c);
 vm.runInContext(fs.readFileSync(require.resolve('../web/assets/panel-controls.js'),'utf8'),c);
 vm.runInContext(source,c);
 const api=c.NetworkToolkitSessionSidebar;api.preferences(options.preferences||state);api.mount(document.getElementById('terminalWorkspace'));
 for(const button of document.querySelectorAll('[data-session-tab]'))button.focus=()=>{document.activeElement=button;};
 return {c,api,document,calls,state,emitRoot:(type,event={})=>{for(const listener of [...rootListeners.get(type)||[]])listener(event);}};
}
const stored=(id='saved-one',extra={})=>({id,name:'Core switch',type:'ssh',host:'192.0.2.10',port:2222,username:'operator',credentialId:'vault-reference',...extra});
const settle=()=>new Promise(resolve=>setImmediate(resolve));
test('session sidebar preserves the existing console and persists collapse and deletion preferences',()=>{
 const h=harness(),consoleNode=h.document.getElementById('existingConsole');
 assert.equal(consoleNode.textContent,'Retained transcript');assert.equal(consoleNode.closest('#terminalWorkspaceColumns').id,'terminalWorkspaceColumns');
 const clear=h.document.querySelector('[data-session-action="clear-recent"]'),create=h.document.querySelector('[data-session-action="new"]');
 assert.equal(clear.parentElement.className,'session-library-section-heading');assert.equal(create.parentElement.className,'session-library-section-heading');assert.equal(clear.closest('#sessionRecentSection').id,'sessionRecentSection');assert.equal(create.closest('#sessionSavedSection').id,'sessionSavedSection');assert.equal(clear.closest('details'),null);assert.equal(create.closest('details'),null);assert(clear.disabled);assert.equal(create.getAttribute('aria-label'),'New saved session');
 const recentToggle=h.document.getElementById('sessionRecentToggle'),savedToggle=h.document.getElementById('sessionSavedToggle'),recentList=h.document.getElementById('sessionRecentList'),savedList=h.document.getElementById('sessionSavedList');
 h.document.emit('click',recentToggle);assert.equal(h.state.terminalRecentCollapsed,true);assert.equal(recentToggle.getAttribute('aria-expanded'),'false');assert(recentList.hidden);assert(!savedList.hidden);assert.equal(savedToggle.getAttribute('aria-expanded'),'true');
 h.document.emit('click',savedToggle);assert.equal(h.state.terminalSavedCollapsed,true);assert(savedList.hidden);assert(!create.disabled);h.document.emit('click',create);assert(h.calls.some(call=>call[0]==='edit'));assert.equal(h.state.terminalSavedCollapsed,true);h.document.emit('click',recentToggle);assert.equal(h.state.terminalRecentCollapsed,false);assert(!recentList.hidden);assert(savedList.hidden);
 h.document.emit('click',h.document.getElementById('terminalSidebarTabSaved'));
 assert.equal(h.state.terminalSidebarCollapsed,true);assert(h.document.getElementById('terminalSessionSidebar').classList.contains('is-collapsed'));assert(h.document.getElementById('terminalSidebarContent').hidden);assert(!h.document.getElementById('terminalSidebarTabSaved').hidden);assert.equal(h.document.getElementById('existingConsole'),consoleNode);
 const checkbox=h.document.getElementById('sessionConfirmDelete');checkbox.checked=false;h.document.emit('change',checkbox);assert.equal(h.state.terminalSessionConfirmDelete,false);
 h.document.emit('click',h.document.getElementById('terminalSidebarTabSaved'));assert.equal(h.state.terminalSidebarCollapsed,false);assert(!h.document.getElementById('terminalSidebarContent').hidden);
});
test('selected tabs toggle sidebar content without separate visibility buttons',()=>{
 const h=harness(),saved=h.document.getElementById('terminalSidebarTabSaved');
 assert.equal(h.document.getElementById('terminalSidebarVisibilityToggle'),null);
 assert.equal(h.document.querySelectorAll('[data-session-action="collapse"]').length,0);
 assert.equal(h.document.querySelector('[data-session-tab]').dataset.sessionTab,'settings');
 h.document.emit('click',saved);assert(h.state.terminalSidebarCollapsed);assert.equal(saved.getAttribute('aria-expanded'),'false');
 h.document.emit('click',saved);assert(!h.state.terminalSidebarCollapsed);assert.equal(saved.getAttribute('aria-expanded'),'true');
 h.document.emit('click',h.document.getElementById('terminalSidebarTabConnection'));assert(!h.state.terminalSidebarCollapsed);assert.equal(h.api.activeTab(),'settings');assert(!h.document.getElementById('terminalSidebarConnection').hidden);
 h.document.emit('keydown',h.document.getElementById('terminalSidebarTabConnection'),{key:' '});assert(h.state.terminalSidebarCollapsed);
 h.document.emit('keydown',h.document.getElementById('terminalSidebarTabConnection'),{key:'Enter'});assert(!h.state.terminalSidebarCollapsed);
 assert.equal(h.document.getElementById('existingConsole').textContent,'Retained transcript');
});
test('terminal sidebar drag resizing is bounded and double-click restores its previous width',()=>{
 const h=harness(),divider=h.document.getElementById('terminalSidebarDivider'),sidebar=h.document.getElementById('terminalSessionSidebar'),consoleNode=h.document.getElementById('existingConsole');
 assert.equal(divider.getAttribute('role'),'separator');assert.equal(divider.getAttribute('aria-controls'),'terminalSidebarContent');assert.equal(sidebar.style.flexBasis,'300px');
 h.document.emit('pointerdown',divider,{button:0,clientX:300,pointerId:1});h.emitRoot('pointermove',{clientX:100,pointerId:1});h.emitRoot('pointerup');
 assert.equal(h.state.terminalSidebarWidth,480);assert.equal(sidebar.style.flexBasis,'480px');assert(!divider.classList.contains('dragging'));
 h.document.emit('dblclick',divider);assert(h.state.terminalSidebarCollapsed);assert.equal(sidebar.style.flexBasis,'44px');assert.equal(divider.getAttribute('aria-valuenow'),'0');assert.equal(h.state.terminalSidebarWidth,480);
 h.document.emit('pointerdown',divider,{button:0,clientX:50,pointerId:2});h.emitRoot('pointermove',{clientX:400,pointerId:2});h.emitRoot('pointerup');assert.equal(h.state.terminalSidebarWidth,480);
 h.document.emit('dblclick',divider);assert(!h.state.terminalSidebarCollapsed);assert.equal(sidebar.style.flexBasis,'480px');assert.equal(h.document.getElementById('existingConsole'),consoleNode);
 const saved=h.api.displayPreferences();assert.equal(saved.terminalSidebarWidth,480);const restored=harness({preferences:saved});assert.equal(restored.state.terminalSidebarWidth,480);assert.equal(restored.document.getElementById('terminalSessionSidebar').style.flexBasis,'480px');
});
test('terminal sidebar divider supports keyboard resize, collapse, restore and reset',()=>{
 const h=harness(),divider=h.document.getElementById('terminalSidebarDivider');
 h.document.emit('keydown',divider,{key:'ArrowLeft'});assert.equal(h.state.terminalSidebarWidth,310);
 h.document.emit('keydown',divider,{key:'ArrowRight',shiftKey:true});assert.equal(h.state.terminalSidebarWidth,270);
 h.document.emit('keydown',divider,{key:'ArrowRight',shiftKey:true});assert.equal(h.state.terminalSidebarWidth,260);
 h.document.emit('keydown',divider,{key:'End'});assert.equal(h.state.terminalSidebarWidth,480);
 h.document.emit('keydown',divider,{key:'Enter'});assert(h.state.terminalSidebarCollapsed);assert.equal(h.state.terminalSidebarWidth,480);
 h.document.emit('keydown',divider,{key:' '});assert(!h.state.terminalSidebarCollapsed);assert.equal(divider.getAttribute('aria-valuenow'),'480');
 h.api.setCollapsed(true);h.document.emit('keydown',divider,{key:'ArrowRight'});assert(!h.state.terminalSidebarCollapsed);assert.equal(h.state.terminalSidebarWidth,470);
 h.document.emit('keydown',divider,{key:'Home'});assert.equal(h.state.terminalSidebarWidth,300);
});
test('responsive sidebar limits preserve the expanded width preference and validate saved widths strictly',()=>{
 const h=harness({preferences:{terminalSidebarWidth:440}}),columns=h.document.getElementById('terminalWorkspaceColumns'),sidebar=h.document.getElementById('terminalSessionSidebar'),divider=h.document.getElementById('terminalSidebarDivider');
 columns.clientWidth=600;h.emitRoot('resize');assert.equal(sidebar.style.flexBasis,'288px');assert.equal(divider.getAttribute('aria-valuemax'),'288');assert.equal(h.state.terminalSidebarWidth,440);
 columns.clientWidth=1200;h.emitRoot('resize');assert.equal(sidebar.style.flexBasis,'440px');assert.equal(divider.getAttribute('aria-valuemax'),'480');assert.equal(h.state.terminalSidebarWidth,440);
 columns.clientWidth=400;h.emitRoot('resize');assert.equal(sidebar.style.flexBasis,'188px');assert.equal(h.api.displayPreferences().terminalSidebarWidth,440);
 for(const width of [259,481,300.5,'400',null,NaN,Infinity]){h.api.preferences({terminalSidebarWidth:width});assert.equal(h.state.terminalSidebarWidth,300);}
});
test('vertical sidebar tabs show one pane and retain macro content, search values and the console',()=>{
 const h=harness(),consoleNode=h.document.getElementById('existingConsole'),host=h.document.getElementById('terminalSidebarMacros'),search=h.document.getElementById('sessionLibrarySearch');
 assert.equal(h.api.activeTab(),'saved');assert.equal(h.document.querySelector('.terminal-sidebar-tabs').getAttribute('aria-orientation'),'vertical');
 assert(!h.document.getElementById('terminalSidebarSaved').hidden);assert(h.document.getElementById('terminalSidebarRecent').hidden);assert(host.hidden);
 const input=h.document.createElement('input');input.id='persistentMacroSearch';input.value='interface audit';host.append(input);host.scrollTop=120;documentActive(input);
 function documentActive(node){h.document.activeElement=node;}
 search.value='core';h.document.emit('input',search);
 h.document.emit('click',h.document.getElementById('terminalSidebarTabMacros'));
 assert.equal(h.api.activeTab(),'macros');assert(!host.hidden);assert(h.document.getElementById('terminalSidebarSaved').hidden);assert(h.document.getElementById('terminalSidebarRecent').hidden);assert.equal(h.document.getElementById('terminalSidebarHeading').textContent,'Macros');
 assert(h.document.getElementById('sessionLibrarySearchGroup').hidden);assert(h.document.getElementById('sessionLibrarySettings').hidden);
 for(let i=0;i<5;i++)h.api.render();assert.equal(h.document.activeElement,input);assert.equal(h.document.getElementById('persistentMacroSearch'),input);assert.equal(input.value,'interface audit');assert.equal(host.scrollTop,120);
 h.api.activateTab('recent');assert(!h.document.getElementById('terminalSidebarRecent').hidden);assert(host.hidden);assert(!h.document.getElementById('sessionLibrarySearchGroup').hidden);assert(!h.document.getElementById('sessionLibrarySettings').hidden);assert.equal(search.value,'core');
 h.api.activateTab('saved');assert.equal(h.document.getElementById('existingConsole'),consoleNode);assert.equal(h.api.ensure(h.document.getElementById('terminalWorkspace')),h.document.getElementById('terminalSessionSidebar'));assert.equal(h.document.querySelectorAll('#terminalSidebarMacros').length,1);
});
test('collapsing retains the vertical rail and clicking any tab reopens the chosen pane',()=>{
 const h=harness(),rail=h.document.querySelector('.terminal-sidebar-rail'),host=h.document.getElementById('terminalSidebarMacros');
 h.api.setCollapsed(true);assert(h.document.getElementById('terminalSidebarContent').hidden);assert.equal(rail.parentElement.id,'terminalSessionSidebar');assert(!rail.hidden);
 h.document.emit('click',h.document.getElementById('terminalSidebarTabRecent'));assert(!h.state.terminalSidebarCollapsed);assert.equal(h.api.activeTab(),'recent');assert(!h.document.getElementById('terminalSidebarRecent').hidden);
 h.api.activateTab('macros');h.api.setCollapsed(true);assert(host.hidden);h.document.emit('click',h.document.getElementById('terminalSidebarTabMacros'));assert(!host.hidden);assert(!h.state.terminalSidebarCollapsed);
 h.api.setCollapsed(true);h.api.activateTab('saved',{expand:false});assert(h.state.terminalSidebarCollapsed);assert.equal(h.api.activeTab(),'saved');assert.equal(h.document.getElementById('terminalSidebarTabSaved').getAttribute('aria-selected'),'true');
});
test('programmatic tab activation expands without toggling an open tab',()=>{
 const h=harness();h.api.activateTab('settings');h.api.activateTab('settings');assert(!h.state.terminalSidebarCollapsed);
 h.api.setCollapsed(true);h.api.activateTab('settings');assert(!h.state.terminalSidebarCollapsed);
 assert.equal(h.api.activeTab(),'settings');assert.equal(h.api.displayPreferences().terminalSidebarTab,'settings');
 h.api.preferences(h.api.displayPreferences());assert.equal(h.api.activeTab(),'settings');
});
test('vertical tabs support roving keyboard focus and persist the selected tab',()=>{
 const h=harness(),saved=h.document.getElementById('terminalSidebarTabSaved'),recent=h.document.getElementById('terminalSidebarTabRecent'),macros=h.document.getElementById('terminalSidebarTabMacros');
 h.document.emit('keydown',saved,{key:'ArrowDown'});assert.equal(h.api.activeTab(),'recent');assert.equal(h.document.activeElement,recent);assert.equal(recent.tabIndex,0);assert.equal(saved.tabIndex,-1);
 h.document.emit('keydown',recent,{key:'End'});assert.equal(h.api.activeTab(),'macros');assert.equal(h.document.activeElement,macros);
 h.document.emit('keydown',macros,{key:'ArrowUp'});assert.equal(h.api.activeTab(),'recent');
 h.document.emit('keydown',recent,{key:'Home'});assert.equal(h.api.activeTab(),'settings');
 h.api.setCollapsed(true);h.document.emit('keydown',saved,{key:'Enter'});assert(!h.state.terminalSidebarCollapsed);
 h.api.activateTab('macros');h.api.setCollapsed(true);const persisted=JSON.parse(JSON.stringify(h.api.displayPreferences())),restored=harness({preferences:persisted});assert.equal(restored.api.activeTab(),'macros');assert(restored.state.terminalSidebarCollapsed);assert(restored.document.getElementById('terminalSidebarContent').hidden);
 restored.api.preferences({terminalSidebarTab:'invalid'});assert.equal(restored.api.activeTab(),'saved');assert(!restored.state.terminalSidebarCollapsed);
});
test('sidebar change events describe only actual changes and tolerate macro UI synchronization',()=>{
 const h=harness();h.document.addEventListener('terminal-sidebar-tab-change',event=>{h.api.render();h.api.activateTab(event.detail.tab,{expand:false});});
 assert.equal(h.calls.filter(call=>call[0]==='sidebar-change').length,0);
 h.api.activateTab('macros');h.api.activateTab('macros');for(let i=0;i<4;i++)h.api.render();
 h.api.setCollapsed(true);h.api.setCollapsed(true);h.api.activateTab('saved');
 assert.deepEqual(h.calls.filter(call=>call[0]==='sidebar-change').map(call=>JSON.parse(JSON.stringify(call[1]))),[{tab:'macros',collapsed:false},{tab:'macros',collapsed:true},{tab:'saved',collapsed:false}]);
});
test('a restricted detached pane keeps Macros available while session tabs stay unavailable',async()=>{
 const h=harness({flags:{DETACHED_TERMINAL:true,DETACHED_WORKSPACE:false},preferences:{terminalSidebarTab:'saved'}}),sidebar=h.document.getElementById('terminalSessionSidebar');
 assert(sidebar);assert.equal(h.api.activeTab(),'macros');assert(!h.document.getElementById('terminalSidebarMacros').hidden);
 for(const name of ['Connection','Saved','Recent']){const tab=h.document.getElementById('terminalSidebarTab'+name);assert(tab.hidden);assert(tab.disabled);assert(h.document.getElementById('terminalSidebar'+name).hidden);}
 assert.equal(h.api.activateTab('saved'),false);assert.equal(h.api.activeTab(),'macros');
 h.api.setCollapsed(true);h.document.emit('keydown',h.document.getElementById('terminalSidebarTabMacros'),{key:'Home'});assert(!h.state.terminalSidebarCollapsed);assert.equal(h.api.activeTab(),'macros');
 h.state.terminalProfiles=[stored()];await h.api.open(h.api.token('saved','saved-one'));assert(!h.calls.some(call=>call[0]==='request'||call[0]==='edit'));
 assert(h.document.getElementById('sessionLibrarySearchGroup').hidden);assert(h.document.getElementById('sessionLibrarySettings').hidden);
});
test('double click opens a saved SSH session with its protected credential without a connection form',async()=>{
 const h=harness();h.state.terminalProfiles=[stored()];h.api.render();
 const button=h.document.querySelector('[data-session-entry]');h.document.emit('dblclick',button);await settle();
 const request=h.calls.find(call=>call[0]==='request');assert(request);assert.equal(request[1],'/terminals');assert.equal(request[3].host,'192.0.2.10');assert.equal(request[3].port,2222);assert.equal(request[3].password,'vault-secret');assert.equal(request[3].profileId,'saved-one');assert.equal([...h.c.terminalSessions.values()][0].macroStartupPending.reconnected,false);
 assert(!h.calls.some(call=>call[0]==='edit'||call[0]==='modal'));assert.equal(h.state.terminalRecent.length,1);assert.equal(h.state.terminalRecent[0].credentialId,'vault-reference');assert(!JSON.stringify(h.state).includes('vault-secret'));assert.equal(h.c.terminalConnectBusy,false);
});
test('opening recent sessions selects a live tab or reconnects the same disconnected tab',async()=>{
 const h=harness(),entry={info:{id:'original-session',label:'Router',state:'connected'},config:stored()};h.c.terminalSessions.set(entry.info.id,entry);h.api.record(entry);
 let t=h.api.token('recent',h.state.terminalRecent[0].id);await h.api.open(t);assert(h.calls.some(call=>call[0]==='select'&&call[1]==='original-session'));assert(!h.calls.some(call=>call[0]==='request'||call[0]==='reconnect'));
 entry.info.state='closed';t=h.api.token('recent',h.state.terminalRecent[0].id);await h.api.open(t);assert(h.calls.some(call=>call[0]==='reconnect'&&call[1]==='original-session'));assert(!h.calls.some(call=>call[0]==='request'));
 t=h.api.token('recent',h.state.terminalRecent[0].id);await h.api.open(t,true);assert(h.calls.some(call=>call[0]==='request'));assert.equal(h.c.terminalSessions.size,2);
});
test('recent connection settings survive reload while secrets and output are excluded',async()=>{
 const h=harness(),entry={info:{id:'old-session',label:'Lab',state:'closed'},config:{...stored(),password:'secret-canary',privateKey:'secret-canary',output:'secret-canary'}};h.api.record(entry);h.api.record(entry);
 assert.equal(h.state.terminalRecent.length,1);assert(!JSON.stringify(h.state).includes('secret-canary'));
 const next=harness();next.api.preferences(JSON.parse(JSON.stringify(h.state)));await next.api.open(next.api.token('recent',next.state.terminalRecent[0].id));assert(next.calls.some(call=>call[0]==='request'&&call[3].host==='192.0.2.10'));
 const restored=harness();restored.api.preferences(JSON.parse(JSON.stringify(h.state)));const retained={info:{id:'restored-tab',state:'connected'},config:{...stored(),credentialId:''}};restored.c.terminalSessions.set('restored-tab',retained);await restored.api.open(restored.api.token('recent',restored.state.terminalRecent[0].id));assert(restored.calls.some(call=>call[0]==='select'&&call[1]==='restored-tab'));assert(!restored.calls.some(call=>call[0]==='request'||call[0]==='reconnect'));
 restored.state.terminalProfiles=[stored()];await restored.api.open(restored.api.token('saved','saved-one'));assert(!restored.calls.some(call=>call[0]==='request'));await restored.api.open(restored.api.token('saved','saved-one'),true);assert(restored.calls.some(call=>call[0]==='request'));
});
test('removal confirmation is configurable and removing a list entry leaves its live console intact',()=>{
 const h=harness(),entry={info:{id:'live',state:'connected'},config:stored()};h.c.terminalSessions.set('live',entry);h.state.terminalProfiles=[stored()];h.api.record(entry);
 assert.equal(h.document.querySelector('[data-session-more]'),null);const remove=h.document.querySelector('[data-session-kind="saved"][data-session-id="saved-one"] [data-session-remove]');assert(remove);h.document.emit('click',remove);assert.equal(h.state.terminalProfiles.length,1);assert(h.document.getElementById('sessionDeleteDontAsk'));assert(!h.calls.some(call=>call[0]==='request'||call[0]==='edit'));
 h.document.getElementById('sessionDeleteDontAsk').checked=true;h.document.emit('click',h.document.querySelector('[data-session-action="confirm-delete"]'));
 assert.equal(h.state.terminalProfiles.length,0);assert.equal(h.state.terminalSessionConfirmDelete,false);assert.equal(h.c.terminalSessions.get('live'),entry);assert.equal(h.state.terminalRecent.length,1);
 h.api.remove('recent',h.state.terminalRecent[0].id);assert.equal(h.state.terminalRecent.length,0);assert.equal(h.c.terminalSessions.get('live'),entry);
});
test('Clear Recent preserves entries added while confirmation is open and preserves saved sessions',()=>{
 const h=harness();h.state.terminalProfiles=[stored()];h.api.record({info:{id:'old',label:'Old'},config:stored()});h.api.clearRecent();
 h.api.record({info:{id:'new',label:'New'},config:stored('another',{host:'192.0.2.99'})});
 h.document.emit('click',h.document.querySelector('[data-session-action="confirm-delete"]'));
 assert.equal(h.state.terminalRecent.length,1);assert.equal(h.state.terminalRecent[0].host,'192.0.2.99');assert.equal(h.state.terminalProfiles.length,1);
});
test('context menu actions target their row and reject stale rows changed during an action',async()=>{
 const h=harness();h.state.terminalProfiles=[stored(),stored('second',{name:'Other',host:'192.0.2.20'})];h.api.render();
 const row=h.document.querySelector('[data-session-id="second"]');h.document.emit('contextmenu',row,{clientX:50,clientY:60});
 const menu=h.document.getElementById('sessionLibraryContextMenu');for(const action of ['open','new-tab','edit','rename','duplicate','copy','remove'])assert(menu.querySelector('[data-session-menu="'+action+'"]'));
 await h.api.run('copy',h.api.token('saved','second'));assert(h.calls.some(call=>call[0]==='copy'&&call[1].includes('192.0.2.20')));
 const stale=h.api.token('saved','second');h.state.terminalProfiles[1].host='192.0.2.30';await h.api.open(stale);assert(!h.calls.some(call=>call[0]==='request'));
 h.api.remove('saved','second');h.state.terminalProfiles[1].name='Changed after confirmation';h.document.emit('click',h.document.querySelector('[data-session-action="confirm-delete"]'));assert.equal(h.state.terminalProfiles.length,2);
});
test('unlinked SSH sessions request only authentication and clear entered secrets after submission',async()=>{
 const h=harness();h.state.terminalProfiles=[stored('no-credential',{credentialId:''})];await h.api.open(h.api.token('saved','no-credential'));
 assert(!h.calls.some(call=>call[0]==='request'));const password=h.document.getElementById('sessionLibraryPassword');assert(password);assert(!h.calls.some(call=>call[0]==='edit'));
 password.value='typed-secret';h.document.emit('submit',h.document.getElementById('sessionLibraryAuthForm'));await settle();
 assert.equal(password.value,'');assert(h.calls.some(call=>call[0]==='request'&&call[3].password==='typed-secret'));assert(!JSON.stringify(h.state).includes('typed-secret'));
});
test('serial saved sessions open directly and duplicate saves a separate safe profile',async()=>{
 const h=harness();h.state.terminalProfiles=[stored('serial',{name:'Console',type:'serial',serialPort:'COM8',baudRate:115200,credentialId:''})];
 await h.api.open(h.api.token('saved','serial'));const request=h.calls.find(call=>call[0]==='request');assert.equal(request[3].serialPort,'COM8');assert.equal(request[3].baudRate,115200);assert(!h.calls.some(call=>call[0]==='modal'));
 await h.api.run('duplicate',h.api.token('saved','serial'));h.document.getElementById('sessionLibraryName').value='Bench console';h.document.emit('submit',h.document.getElementById('sessionLibraryNameForm'));assert.equal(h.state.terminalProfiles.length,2);assert.notEqual(h.state.terminalProfiles[0].id,'serial');assert.equal(h.state.terminalProfiles[0].name,'Bench console');assert.equal(h.state.terminalProfiles[0].serialPort,'COM8');
});
test('startup completion flag waits for async workspace initialization and reports rejected initialization',async()=>{
 const source=html.slice(html.indexOf('window.__networkToolkitStartupState=0;'),html.lastIndexOf('</script>'));
 for(const fail of [false,true]){
  let finish;const c={init:()=>new Promise((resolve,reject)=>{finish=fail?reject:resolve;}),console:{error(){}}};c.window=c;vm.runInNewContext(source,c);assert.equal(c.__networkToolkitStartupState,0);finish(fail?new Error('fixture'):undefined);await settle();assert.equal(c.__networkToolkitStartupState,fail?-1:1);
 }
});

test('SFTP file browser sidebar tab appears dynamically for supported active SSH sessions only',()=>{
 const entry={info:{id:'ssh-one',type:'ssh',state:'connected',host:'192.0.2.10',port:22}};
 const events=[];
 const supported={sync:x=>x?.info?.type==='ssh'&&x.info.state==='connected',available:x=>x?.info?.type==='ssh'&&x.info.state==='connected',render:(panel,x)=>{events.push([panel.id,x.info.id]);panel.textContent='Remote files for '+x.info.id;}};
 const h=harness({flags:{currentTerminal:()=>entry,NetworkToolkitTerminalSFTP:supported}}),tab=h.document.getElementById('terminalSidebarTabSFTP'),panel=h.document.getElementById('terminalSidebarSFTP');
 assert(tab&&!tab.hidden&&!tab.disabled);assert(panel.hidden);
 assert.equal(h.api.activateTab('files'),true);
 assert.equal(h.api.activeTab(),'files');assert(!panel.hidden);
 assert.equal(panel.textContent,'Remote files for ssh-one');assert.deepEqual(events,[['terminalSidebarSFTP','ssh-one']]);
 entry.info.state='closed';h.api.render();
 assert.equal(tab.hidden,true);assert(panel.hidden);assert.equal(h.api.activeTab(),'saved');
 entry.info.state='connected';entry.info.type='telnet';h.api.render();assert(tab.hidden);
});
