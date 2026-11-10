'use strict';
// The page side of shared app data (web/assets/shared-data.js): what a save
// tells the engine, when merged values are applied, and when a change from
// another app waits because the user is in the middle of something.
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(require.resolve('../web/assets/shared-data.js'),'utf8');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const engine=fs.readFileSync(require.resolve('../cmd/network-toolkit/shared_data.go'),'utf8');
const plain=value=>JSON.parse(JSON.stringify(value));
const profile=(id,extra={})=>({id,name:'Device '+id,type:'ssh',host:'192.0.2.10',port:22,username:'operator',...extra});
function harness(){
 const calls=[],events=[],selectors=new Set();
 const document={visibilityState:'visible',activeElement:null,documentElement:{dataset:{}},body:{classList:{contains:()=>false}},addEventListener(){},querySelector:selector=>selector.split(',').some(part=>selectors.has(part.trim()))?{}:null,getElementById:()=>null,dispatchEvent:event=>{events.push(event);return true;}};
 const c={document,console,JSON,Set,Map,Number,Math,Object,Array,String,Promise,encodeURIComponent,calls,
  state:{theme:'dark',accentColor:'',activePage:'terminal',terminalSidebarWidth:300,terminalProfiles:[],terminalRecent:[],terminalSavedManualOrder:false,terminalPromptNames:[],ftpProfiles:[],ftpRecent:[],sftpRecent:[],sftpVerifiedHostKeys:{},editorPreferences:{wordWrap:false},terminalAppearance:{cursor:'block'},terminalInteraction:{},terminalFontSize:13,terminalHighlight:true,terminalMacroConfirmation:'always',terminalActiveActionsPosition:'top',terminalSidebarSide:'right',terminalSessionConfirmDelete:true,terminalConfirmClose:true,terminalConfirmReconnectAll:true,terminalConfirmDisconnectAll:true,terminalConfirmCloseAll:true,profiles:[{id:'t',name:'Gateway'}],resultTimePreferences:{hourFormat:'12'}},
  stateRevision:0,savedStateRevision:-1,saveTimer:null,stateLoaded:true,online:true,OFFLINE:false,shutDown:false,EDITOR_WINDOW:false,DETACHED_TERMINAL:false,
  CustomEvent:class{constructor(type,options){this.type=type;this.detail=options.detail;}},
  NetworkToolkitTerminalPromptLabels:{revision:item=>Number.isSafeInteger(item?.nameRevision)?item.nameRevision:0,normalize:value=>Array.isArray(value)?value:[]},
  NetworkToolkitSessionFolders:{normalize:value=>typeof value==='string'?value.trim():''},
  NetworkToolkitTerminalAppearance:{normalize:value=>({cursor:'block',...(value||{})}),sync:()=>calls.push('appearance-sync')},
  NetworkToolkitTerminalInteraction:{safePreferences:value=>({...(value||{})})},
  NetworkToolkitSessionSidebar:{render:()=>calls.push('sidebar')},NetworkToolkitSavedSessions:{sync:()=>calls.push('indicators')},
  NetworkToolkitFileEditor:{windowAdapter:{applyPreferences:value=>calls.push('editor:'+JSON.stringify(value))}},
  NetworkToolkitCredentials:{refresh:()=>{calls.push('credentials');}},
  renderTerminalProfiles:()=>calls.push('profiles'),applyAccentColor:()=>calls.push('accent'),answers:[],requests:[]};
 // The page's own path for display preferences.
 c.applyTerminalWorkspaceDisplay=value=>{Object.assign(c.state,value);calls.push('display:'+Object.keys(value).sort().join(','));};
 c.api=async path=>{c.requests.push(path);return c.answers.shift();};
 c.window=c;vm.createContext(c);
 vm.runInContext(html.split('\n').find(line=>line.startsWith('const TERMINAL_SAFE_FIELDS=')),c);
 vm.runInContext(html.split('\n').find(line=>line.startsWith('function safeTerminalConfig(')),c);
 vm.runInContext(source.replace("if(typeof document!=='undefined')start();",''),c);
 return {c,shared:c.NetworkToolkitSharedData,calls,events,selectors};
}
// Save the way flushState does and answer like the engine.
function save(h,answer){const sent=h.c.stateRevision,body=h.shared.outgoing(h.c.state);h.c.savedStateRevision=sent;h.shared.saved(answer,sent);return body;}

test('the page lists exactly the shared keys of the engine table',()=>{
 const {shared}=harness();
 const rows=[...engine.matchAll(/\{Name: "([A-Za-z]+)", Kind: shared(List|Map|Value|Counter)([^}]*)\}/g)].map(match=>({name:match[1],kind:match[2],live:/Live: true/.test(match[3])}));
 assert(rows.length>30,'the engine table was read');
 assert.deepEqual([...shared.LIVE].sort(),rows.filter(row=>row.live).map(row=>row.name).sort());
 assert.deepEqual([...shared.AT_START].sort(),rows.filter(row=>!row.live&&row.kind!=='Counter').map(row=>row.name).sort());
 assert.deepEqual([...shared.SHARED].sort(),[...shared.LIVE,...shared.AT_START].sort());
 for(const key of ['theme','accentColor','terminalAppearance','terminalSyntax','terminalBehavior','terminalInteraction','terminalFontSize','editorPreferences','terminalProfiles'])assert(shared.LIVE.includes(key),key+' is applied in a running page');
 assert(html.includes('<script src="assets/shared-data.js"></script>'));
 assert(html.includes('NetworkToolkitSharedData.outgoing(state)')&&html.includes('NetworkToolkitSharedData?.saved(response,revision)')&&html.includes('NetworkToolkitSharedData?.failed()')&&html.includes('mergeState(saved);globalThis.NetworkToolkitSharedData?.loaded();'));
});
test('a save says what the page started from, only for the shared keys it changed',()=>{
 const h=harness(),{c,shared}=h;
 assert.equal(shared.outgoing(c.state),c.state,'before settings are loaded nothing is added');shared.failed();
 shared.loaded();
 let body=save(h,{sharedData:{revision:'r1'}});
 assert.deepEqual(plain(body._sharedBase),{},'nothing changed: no shared key is claimed');assert(!('_sharedBase' in c.state));assert.equal(shared.revision,'r1');
 // Window state is not shared: it is sent but never listed.
 c.state.terminalSidebarWidth=420;c.state.activePage='editor';c.stateRevision++;
 assert.deepEqual(plain(save(h,{sharedData:{revision:'r1'}})._sharedBase),{});
 // The user adds a connection and changes two settings.
 c.state.terminalProfiles=[profile('mine')];c.state.theme='light';c.state.terminalAppearance={...c.state.terminalAppearance,cursor:'bar'};c.stateRevision++;
 body=save(h,{sharedData:{revision:'r2'}});
 assert.deepEqual(plain(body._sharedBase),{terminalProfiles:[],theme:'dark',terminalAppearance:{cursor:'block'}});
 assert.equal(body.theme,'light');assert.equal(shared.revision,'r2');
 // Once acknowledged, the sent values are the new starting point.
 c.stateRevision++;assert.deepEqual(plain(save(h,{sharedData:{revision:'r2'}})._sharedBase),{});
 c.state.theme='dark';c.stateRevision++;
 assert.deepEqual(plain(save(h,{sharedData:{revision:'r3'}})._sharedBase),{theme:'light'});
 // A failed save keeps the old starting point, so the change is sent again.
 c.state.terminalFontSize=15;c.stateRevision++;
 assert.deepEqual(plain(shared.outgoing(c.state)._sharedBase),{terminalFontSize:13});shared.failed();
 assert.deepEqual(plain(save(h,{sharedData:{revision:'r4'}})._sharedBase),{terminalFontSize:13});
 // Data only the main window uses and settings read at start are tracked too.
 c.state.profiles=[];c.state.resultTimePreferences={hourFormat:'24'};c.stateRevision++;
 assert.deepEqual(plain(save(h,{sharedData:{revision:'r5'}})._sharedBase),{profiles:[{id:'t',name:'Gateway'}],resultTimePreferences:{hourFormat:'12'}});
});
test('a default the page fills in for a setting nobody saved is not sent as a change',()=>{
 const h=harness(),{c,shared}=h;
 delete c.state.terminalInteraction;delete c.state.terminalHighlight;
 shared.loaded();
 c.state.terminalInteraction={};c.state.terminalHighlight=true;c.stateRevision++;
 assert.deepEqual(plain(save(h,{sharedData:{revision:'r1'}})._sharedBase),{});
 // A real choice for such a setting is a change, started from nothing.
 c.state.terminalHighlight=false;c.stateRevision++;
 assert.deepEqual(plain(save(h,{sharedData:{revision:'r2'}})._sharedBase),{terminalHighlight:true});
});
test('merged values from a save are applied only when nothing changed meanwhile',()=>{
 const h=harness(),{c,shared,calls,events}=h;shared.loaded();
 c.stateRevision=4;shared.outgoing(c.state);c.savedStateRevision=4;
 shared.saved({sharedData:{revision:'r1',values:{...c.state,terminalProfiles:[profile('other')],terminalSavedManualOrder:true,theme:'light',editorPreferences:{wordWrap:true}}}},4);
 assert.deepEqual(c.state.terminalProfiles.map(p=>p.id),['other']);assert.equal(c.state.terminalSavedManualOrder,true);assert.equal(c.state.theme,'light');assert.equal(shared.revision,'r1');
 assert.deepEqual(calls,['display:theme','appearance-sync','accent','editor:{"wordWrap":true}','indicators','profiles']);
 assert.deepEqual([...events[0].detail.keys].sort(),['editorPreferences','terminalProfiles','terminalSavedManualOrder','theme']);
 assert.equal(c.state.terminalProfiles[0].sshCompatibility,'modern','values pass through the page clean-up');
 // What was applied is the new starting point: the next save claims nothing.
 c.stateRevision++;assert.deepEqual(plain(save(h,{sharedData:{revision:'r1'}})._sharedBase),{});
 // The user added a profile while the save was on its way: keep the page's
 // list; the next save starts from what was sent, not from the merged list.
 c.stateRevision=7;shared.outgoing(c.state);
 c.state.terminalProfiles=[profile('mine'),...c.state.terminalProfiles];c.stateRevision=8;
 shared.saved({sharedData:{revision:'r2',values:{...c.state,terminalProfiles:[profile('other'),profile('third')]}}},7);
 assert.deepEqual(c.state.terminalProfiles.map(p=>p.id),['mine','other']);assert.equal(shared.revision,'r1');
 const body=shared.outgoing(c.state);
 assert.deepEqual(body._sharedBase.terminalProfiles.map(p=>p.id),['other']);
});
test('a change from another app waits for unsaved changes, dialogs, menus, drags and text entry',async()=>{
 const {c,shared,selectors,calls}=harness();shared.loaded();
 const offer=()=>c.answers.push({revision:'r9',values:{...c.state,terminalProfiles:[profile('new')],editorPreferences:{wordWrap:true}}});
 for(const [name,block,release] of [
  ['an unsaved change',()=>{c.stateRevision=3;c.savedStateRevision=2;},()=>{c.savedStateRevision=3;}],
  ['a pending save',()=>{c.saveTimer=7;},()=>{c.saveTimer=null;}],
  ['an open dialog',()=>selectors.add('#modalRoot .modal'),()=>selectors.delete('#modalRoot .modal')],
  ['an open session menu',()=>selectors.add('#sessionLibraryContextMenu'),()=>selectors.delete('#sessionLibraryContextMenu')],
  ['a drag in the Saved list',()=>selectors.add('.session-saved-dragging'),()=>selectors.delete('.session-saved-dragging')],
  ['typing in the sidebar',()=>{c.document.activeElement={tagName:'INPUT',closest:selector=>selector.includes('#terminalSessionSidebar')?{}:null};},()=>{c.document.activeElement=null;}],
  ['typing in the editor while editor preferences changed',()=>{c.document.activeElement={tagName:'TEXTAREA',closest:selector=>selector.includes('#networkToolkitFileEditor')?{}:null};},()=>{c.document.activeElement=null;}]]){
  block();offer();
  assert.equal(await shared.poll(),false,name+' defers the refresh');
  assert.deepEqual(c.state.terminalProfiles,[],name+': the list is untouched');assert.equal(shared.revision,'',name+': the page keeps its revision');
  release();
 }
 assert.equal(shared.busy(),false);offer();
 assert.equal(await shared.poll(),true);assert.deepEqual(c.state.terminalProfiles.map(p=>p.id),['new']);assert.equal(shared.revision,'r9');
 assert(c.requests.every(path=>path.startsWith('/shared-data?revision=')));assert.equal(c.requests.at(-1),'/shared-data?revision=');
 // Typing in the terminal or another form does not hold back a theme change.
 c.document.activeElement={tagName:'TEXTAREA',closest:()=>null};
 c.answers.push({revision:'r10',values:{...c.state,theme:'light'}});
 assert.equal(await shared.poll(),true);assert.equal(c.state.theme,'light');c.document.activeElement=null;
 // Unchanged values: no redraw, the revision is simply recorded.
 calls.length=0;c.answers.push({revision:'r10'});assert.equal(await shared.poll(),false);assert.deepEqual(calls,[]);
 assert.equal(c.requests.at(-1),'/shared-data?revision=r10');
 // Same values under a new revision: nothing to redraw either.
 c.answers.push({revision:'r11',values:{...c.state}});assert.equal(await shared.poll(),true);
 assert.deepEqual(calls,[]);assert.equal(shared.revision,'r11');
 // The password manager's list is read again when the vault changed.
 c.answers.push({revision:'r11',credentials:'a'});await shared.poll();c.answers.push({revision:'r11',credentials:'a'});await shared.poll();assert.deepEqual(calls,[]);
 c.answers.push({revision:'r11',credentials:'b'});await shared.poll();assert.deepEqual(calls,['credentials']);
});
test('an entry that did not fit in a full list leaves the page and the user is told',()=>{
 const h=harness(),{c,shared}=h;shared.loaded();
 const toasts=[];c.toast=(text,kind)=>toasts.push([kind,text]);
 c.state.terminalProfiles=[profile('mine'),profile('kept')];c.stateRevision++;
 const sent=c.stateRevision,body=shared.outgoing(c.state);
 assert.deepEqual(plain(body._sharedBase.terminalProfiles),[]);
 // The answer arrives while the user is already changing something else.
 c.state.theme='light';c.stateRevision++;c.savedStateRevision=sent;
 shared.saved({sharedData:{revision:'r2',refused:{terminalProfiles:[{id:'mine',name:'Device mine'}]},values:{...c.state,terminalProfiles:[profile('kept'),profile('theirs')]}}},sent);
 assert.deepEqual(c.state.terminalProfiles.map(p=>p.id),['kept'],'the entry that was not saved is not shown as saved');
 assert.equal(toasts.length,1);assert.equal(toasts[0][0],'error');
 assert.match(toasts[0][1],/"Device mine" was not saved: the list of saved sessions is full \(200\) because another Network Toolkit app added entries at the same time/);
 assert(h.calls.includes('profiles')&&h.calls.includes('indicators'),'the lists are redrawn');
 // The next save does not tell the engine that this page removed anything.
 const next=shared.outgoing(c.state);
 assert.deepEqual(Object.keys(plain(next._sharedBase)),['theme']);
 // A list this page does not refresh while it runs (saved targets).
 shared.failed();c.state.profiles=[{id:'t',name:'Gateway'},{id:'extra',name:'Extra'}];c.stateRevision++;
 save(h,{sharedData:{revision:'r3',refused:{profiles:[{id:'extra',name:'Extra'}]}}});
 assert.deepEqual(c.state.profiles.map(p=>p.id),['t']);assert.match(toasts.at(-1)[1],/saved targets is full \(200\)/);
});
test('only the refused entry leaves when another entry shares its id, and the main-window lists are redrawn',()=>{
 const h=harness(),{c,shared}=h;shared.loaded();
 c.toast=()=>{};c.renderProfiles=()=>h.calls.push('targets');c.state.activePage='profiles';
 c.NetworkToolkitTFTPConnections={mount:()=>h.calls.push('tftp')};
 c.state.profiles=[{id:'t',name:'Gateway'},{id:'same',name:'Kept'},{id:'same',name:'Refused'}];c.state.tftpProfiles=[{id:'x',name:'One'},{id:'y',name:'Two'}];c.stateRevision++;
 save(h,{sharedData:{revision:'r2',refused:{profiles:[{id:'same',name:'Refused'}],tftpProfiles:[{id:'y',name:'Two'}]}}});
 assert.deepEqual(c.state.profiles.map(p=>p.name),['Gateway','Kept']);assert.deepEqual(c.state.tftpProfiles.map(p=>p.id),['x']);
 assert(h.calls.includes('targets')&&h.calls.includes('tftp'),'saved targets and TFTP profiles are redrawn');
});
test('an answer to a poll that was asked before a save is ignored',async()=>{
 const h=harness(),{c,shared}=h;shared.loaded();
 let release;c.api=()=>new Promise(resolve=>{release=resolve;});
 const polled=shared.poll();
 // While the poll is on its way the user adds a connection and it is saved.
 c.state.terminalProfiles=[profile('mine')];c.stateRevision=1;save(h,{sharedData:{revision:'r2'}});
 release({revision:'r1',values:{...c.state,terminalProfiles:[]}});
 assert.equal(await polled,false);
 assert.deepEqual(c.state.terminalProfiles.map(p=>p.id),['mine'],'the older answer did not take the new connection away');assert.equal(shared.revision,'r2');
});
test('a host-key difference found by the one-time move is shown once, in plain words, and acknowledged',async()=>{
 const h=harness(),{c,shared,selectors}=h;shared.loaded();
 const modals=[],posts=[];let click=null;
 c.openModal=(title,body,footer)=>{modals.push({title,body,footer});};c.closeModal=()=>modals.push('closed');
 c.esc=value=>String(value).replace(/</g,'&lt;');
 const query=c.document.querySelector;c.document.querySelector=selector=>selector==='[data-shared-notice-ok]'?{addEventListener:(type,fn)=>{click=fn;}}:query(selector);
 c.api=async(path,method,body)=>{if(method==='POST'){posts.push([path,JSON.parse(JSON.stringify(body))]);return {ok:true};}return {revision:'r1',notices:[{id:'n1',kind:'host-key-conflict',source:'terminal',items:['router.test:22','<b>edge</b>:22']}]};};
 // Not while a dialog is open.
 selectors.add('#modalRoot .modal');await shared.poll();assert.equal(modals.length,0);selectors.delete('#modalRoot .modal');
 await shared.poll();
 assert.equal(modals.length,1);assert.equal(modals[0].title,'Saved data is now shared');
 assert(modals[0].body.includes('router.test:22')&&modals[0].body.includes('&lt;b>edge&lt;/b>:22')&&!modals[0].body.includes('<b>edge'),'the hosts are listed as text');
 assert(/the Terminal window had remembered a different host key than the main window/.test(modals[0].body)&&/main window's key was kept/.test(modals[0].body)&&/compare the fingerprint/.test(modals[0].body));
 await shared.poll();assert.equal(modals.length,1,'the same notice is not shown again while it is open');
 click();await new Promise(resolve=>setImmediate(resolve));
 assert.deepEqual(posts,[['/shared-data/notices',{acknowledge:['n1']}]]);assert.equal(modals.at(-1),'closed');
});
test('hosts that only one window trusted are named, with where to remove them',async()=>{
 const h=harness(),{c,shared}=h;shared.loaded();
 const modals=[];c.openModal=(title,body)=>{modals.push({title,body});};c.closeModal=()=>{};c.esc=value=>String(value).replace(/</g,'&lt;');
 c.api=async()=>({revision:'r1',notices:[{id:'n4',kind:'hosts-added',source:'terminal',items:['lab.test:2222','<i>x</i>.test:22']},{id:'n5',kind:'host-key-conflict',source:'terminal',items:['router.test:22']}]});
 await shared.poll();
 assert.equal(modals.length,1);assert.equal(modals[0].title,'Saved data is now shared');
 assert(modals[0].body.includes('lab.test:2222')&&modals[0].body.includes('&lt;i>x&lt;/i>.test:22')&&modals[0].body.includes('router.test:22'));
 assert.match(modals[0].body,/These SSH hosts were trusted only by the Terminal window\. They are trusted by every app now/);
 assert.match(modals[0].body,/To stop trusting one, open the Saved sidebar of the Terminal, choose SSH host keys and remove it/);
 assert.match(modals[0].body,/had remembered a different host key/,'both notices are shown together');
});
test('an older version saving to the data folder is reported once, and what was put back is named',async()=>{
 const h=harness(),{c,shared}=h;shared.loaded();
 const toasts=[];c.toast=(text,kind)=>toasts.push([kind,text]);
 c.answers.push({revision:'r1',olderVersion:true},{revision:'r1',olderVersion:true},{revision:'r1'});
 await shared.poll();await shared.poll();await shared.poll();
 assert.equal(toasts.length,1,'the warning is shown once, not with every refresh');assert.equal(toasts[0][0],'error');
 assert.match(toasts[0][1],/An older version of Network Toolkit is saving to this data folder/);
 assert.match(toasts[0][1],/changes you make here may be replaced/);assert.match(toasts[0][1],/Close that window and update it/);
 // The saved entries that were written again after the older version dropped
 // them are listed, with what to do about one that was removed on purpose.
 const modals=[];c.openModal=(title,body)=>{modals.push({title,body});};c.closeModal=()=>{};c.esc=value=>String(value).replace(/</g,'&lt;');
 c.api=async()=>({revision:'r1',notices:[{id:'n2',kind:'older-version',source:'',items:['Core switch','<i>Edge</i>']}]});
 await shared.poll();
 assert.equal(modals.length,1);assert.equal(modals[0].title,'Saved entries were put back');
 assert(modals[0].body.includes('Core switch')&&modals[0].body.includes('&lt;i>Edge&lt;/i>')&&!modals[0].body.includes('<i>Edge'));
 assert.match(modals[0].body,/An older version of Network Toolkit saved its settings over the shared ones/);
 assert.match(modals[0].body,/removed one of them in the older version on purpose, remove it again here/);
 assert(!/now share saved connections/.test(modals[0].body),'this is not the notice of the one-time move');
});
test('saved entries that came back after the settings file was lost are named',async()=>{
 const h=harness(),{c,shared}=h;shared.loaded();
 const modals=[];c.openModal=(title,body)=>{modals.push({title,body});};c.closeModal=()=>{};c.esc=value=>String(value).replace(/</g,'&lt;');
 c.api=async()=>({revision:'r1',notices:[{id:'n7',kind:'recovered',source:'',items:['Core switch','<i>Edge</i>']},{id:'n8',kind:'limit',source:'terminal',items:['Saved sessions']}]});
 await shared.poll();
 assert.equal(modals.length,1);assert.equal(modals[0].title,'Saved entries were recovered');
 assert(modals[0].body.includes('Core switch')&&modals[0].body.includes('&lt;i>Edge&lt;/i>')&&!modals[0].body.includes('<i>Edge'));
 assert.match(modals[0].body,/The settings file of this data folder was missing/);
 assert.match(modals[0].body,/these came back from it/);
 assert.match(modals[0].body,/deleted the settings file on purpose to start over, remove what you do not want here/);
 assert(!/now share saved connections/.test(modals[0].body)&&!/lists are full/.test(modals[0].body),'it is shown on its own, not as part of the one-time move');
 // Together with entries an older version dropped, both are explained.
 const both=harness();both.shared.loaded();
 const seen=[];both.c.openModal=(title,body)=>{seen.push({title,body});};both.c.closeModal=()=>{};
 both.c.api=async()=>({revision:'r1',notices:[{id:'n7',kind:'recovered',source:'',items:['Core switch']},{id:'n2',kind:'older-version',source:'',items:['Lab']}]});
 await both.shared.poll();
 assert.equal(seen[0].title,'Saved entries were put back');
 assert.match(seen[0].body,/settings file of this data folder was missing/);assert.match(seen[0].body,/An older version of Network Toolkit saved its settings/);
});
test('nothing is asked while the page is hidden, offline, closing or a detached window',async()=>{
 const {c,shared}=harness();
 for(const [set,reset] of [[()=>{c.document.visibilityState='hidden';},()=>{c.document.visibilityState='visible';}],[()=>{c.online=false;},()=>{c.online=true;}],[()=>{c.stateLoaded=false;},()=>{c.stateLoaded=true;}],[()=>{c.shutDown=true;},()=>{c.shutDown=false;}],[()=>{c.EDITOR_WINDOW=true;},()=>{c.EDITOR_WINDOW=false;}],[()=>{c.DETACHED_TERMINAL=true;},()=>{c.DETACHED_TERMINAL=false;}]]){
  set();assert.equal(await shared.poll(),false);reset();
 }
 assert.deepEqual(c.requests,[]);
 // A failed request is not an error for the user and changes nothing.
 c.api=async()=>{throw new Error('engine restarting');};assert.equal(await shared.poll(),false);assert.equal(shared.revision,'');
});
test('refreshed values are cleaned like loaded ones and carry no unknown fields',()=>{
 const {shared}=harness();
 const clean=shared.normalize({terminalProfiles:[profile('a',{password:'secret-marker',folder:' MDF ',nameRevision:4,nameExplicit:true}),{id:7,name:'bad'},null],terminalRecent:[profile('r',{lastUsed:5}),profile('r',{lastUsed:9})],ftpProfiles:[{id:'f',name:'Files',host:'files.test',port:'990',security:'weird',password:'secret-marker'}],ftpRecent:[{id:'g',name:'Files',host:'files.test',port:21,username:'u',lastUsed:3,secret:'secret-marker'}],sftpVerifiedHostKeys:[],terminalSavedManualOrder:'yes',theme:'neon',accentColor:'red',terminalFontSize:99,terminalMacroConfirmation:'sometimes',terminalSidebarSide:'top',terminalConfirmClose:'no',editorPreferences:[1]});
 assert.equal(JSON.stringify(clean.terminalProfiles).includes('secret-marker')||JSON.stringify(clean.ftpProfiles).includes('secret-marker')||JSON.stringify(clean.ftpRecent).includes('secret-marker'),false);
 assert.deepEqual(clean.terminalProfiles.map(p=>[p.id,p.folder,p.nameRevision,p.nameExplicit]),[['a','MDF',4,true]]);
 assert.deepEqual(clean.terminalRecent.map(p=>[p.id,p.lastUsed]),[['r',5]]);
 assert.deepEqual(plain(clean.ftpProfiles),[{id:'f',name:'Files',folder:'',host:'files.test',port:990,username:'',security:'explicit',fingerprint:'',credentialId:''}]);
 assert.deepEqual(plain(clean.ftpRecent),[{id:'g',name:'Files',host:'files.test',port:21,username:'u',security:'explicit',fingerprint:'',credentialId:'',lastUsed:3}]);
 assert.deepEqual(plain(clean.sftpVerifiedHostKeys),{});assert.equal(clean.terminalSavedManualOrder,false);
 assert.deepEqual([clean.theme,clean.accentColor,clean.terminalFontSize,clean.terminalMacroConfirmation,clean.terminalSidebarSide,clean.terminalConfirmClose],['dark','',13,'always','right',true]);
 assert.deepEqual(plain(clean.editorPreferences),{});
 assert.deepEqual(plain(shared.normalize(null).terminalProfiles),[]);
 assert.deepEqual(shared.normalize({accentColor:'#286DBB',theme:'light',terminalFontSize:16}).accentColor,'#286DBB');
});
