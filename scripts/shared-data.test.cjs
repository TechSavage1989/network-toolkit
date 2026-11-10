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
