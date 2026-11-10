'use strict';
// The page side of shared app data (web/assets/shared-data.js): which
// revision a save names, when merged values are applied, and when a change
// from another app waits because the user is in the middle of something.
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(require.resolve('../web/assets/shared-data.js'),'utf8');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const engine=fs.readFileSync(require.resolve('../cmd/network-toolkit/shared_data.go'),'utf8');
const plain=value=>JSON.parse(JSON.stringify(value));
const profile=(id,extra={})=>({id,name:'Device '+id,type:'ssh',host:'192.0.2.10',port:22,username:'operator',...extra});
function harness(){
 const calls=[],events=[],selectors=new Set();
 const document={visibilityState:'visible',activeElement:null,body:{classList:{contains:()=>false}},addEventListener(){},querySelector:selector=>selector.split(',').some(part=>selectors.has(part.trim()))?{}:null,getElementById:()=>null,dispatchEvent:event=>{events.push(event);return true;}};
 const c={document,console,JSON,Set,Map,Number,Math,Object,Array,String,Promise,encodeURIComponent,calls,
  state:{theme:'dark',terminalProfiles:[],terminalRecent:[],terminalSavedManualOrder:false,terminalPromptNames:[],ftpProfiles:[],ftpRecent:[],sftpRecent:[],sftpVerifiedHostKeys:{},editorPreferences:{wordWrap:false}},
  stateRevision:0,savedStateRevision:-1,saveTimer:null,stateLoaded:true,online:true,OFFLINE:false,shutDown:false,EDITOR_WINDOW:false,DETACHED_TERMINAL:false,
  CustomEvent:class{constructor(type,options){this.type=type;this.detail=options.detail;}},
  NetworkToolkitTerminalPromptLabels:{revision:item=>Number.isSafeInteger(item?.nameRevision)?item.nameRevision:0,normalize:value=>Array.isArray(value)?value:[]},
  NetworkToolkitSessionFolders:{normalize:value=>typeof value==='string'?value.trim():''},
  NetworkToolkitSessionSidebar:{render:()=>calls.push('sidebar')},NetworkToolkitSavedSessions:{sync:()=>calls.push('indicators')},
  renderTerminalProfiles:()=>calls.push('profiles'),answers:[],requests:[]};
 c.api=async path=>{c.requests.push(path);return c.answers.shift();};
 c.window=c;vm.createContext(c);
 vm.runInContext(html.split('\n').find(line=>line.startsWith('const TERMINAL_SAFE_FIELDS=')),c);
 vm.runInContext(html.split('\n').find(line=>line.startsWith('function safeTerminalConfig(')),c);
 vm.runInContext(source.replace("if(typeof document!=='undefined')start();",''),c);
 return {c,shared:c.NetworkToolkitSharedData,calls,events,selectors};
}
test('the page refreshes exactly the live keys of the engine table',()=>{
 const {shared}=harness();
 const live=[...engine.matchAll(/\{Name: "([A-Za-z]+)"[^}]*Live: true[^}]*\}/g)].map(match=>match[1]);
 assert.deepEqual([...shared.LIVE].sort(),live.sort());
 assert(!shared.LIVE.includes('editorPreferences')&&engine.includes('{Name: "editorPreferences", Kind: sharedMap}'),'editor preferences are read when an editor starts');
 assert(html.includes('<script src="assets/shared-data.js"></script>'));
 assert(html.includes("NetworkToolkitSharedData.outgoing(state)")&&html.includes('NetworkToolkitSharedData?.saved(response,revision)')&&html.includes('NetworkToolkitSharedData?.failed()'));
});
test('a save names the revision the page holds and never changes the settings object',()=>{
 const {c,shared}=harness();
 assert.equal(shared.outgoing(c.state),c.state,'nothing is added before a revision is known');shared.failed();
 shared.saved({sharedData:{revision:'r1'}},0);
 const body=shared.outgoing(c.state);
 assert.equal(body._sharedBase,'r1');assert.equal(body.theme,'dark');assert(!('_sharedBase' in c.state));
 shared.saved({ok:true,sharedData:{revision:'r2'}},0);assert.equal(shared.revision,'r2');
 shared.saved({ok:true},0);assert.equal(shared.revision,'r2','an answer without shared data changes nothing');
});
test('merged values from a save are applied only when nothing changed meanwhile',()=>{
 const {c,shared,calls,events}=harness();
 c.stateRevision=4;shared.outgoing(c.state);
 shared.saved({sharedData:{revision:'r1',values:{terminalProfiles:[profile('other')],terminalSavedManualOrder:true}}},4);
 assert.deepEqual(c.state.terminalProfiles.map(p=>p.id),['other']);assert.equal(c.state.terminalSavedManualOrder,true);assert.equal(shared.revision,'r1');
 assert.deepEqual(calls,['indicators','profiles']);assert.deepEqual([...events[0].detail.keys],['terminalProfiles','terminalSavedManualOrder']);
 assert.equal(c.state.terminalProfiles[0].sshCompatibility,'modern','values pass through the page clean-up');
 assert.deepEqual({...c.state.editorPreferences},{wordWrap:false},'editor preferences are not replaced in a running page');
 // The user added a profile while the save was on its way: keep the page's
 // list and the old revision, so the next save is merged from there.
 c.state.terminalProfiles=[profile('mine'),...c.state.terminalProfiles];c.stateRevision=6;shared.outgoing(c.state);
 shared.saved({sharedData:{revision:'r2',values:{terminalProfiles:[profile('other'),profile('third')]}}},5);
 assert.deepEqual(c.state.terminalProfiles.map(p=>p.id),['mine','other']);assert.equal(shared.revision,'r1');
});
test('a change from another app waits for unsaved changes, dialogs, menus and drags',async()=>{
 const {c,shared,selectors,calls}=harness();
 const offer=()=>c.answers.push({revision:'r9',values:{terminalProfiles:[profile('new')]}});
 for(const [name,block,release] of [
  ['an unsaved change',()=>{c.stateRevision=3;c.savedStateRevision=2;},()=>{c.savedStateRevision=3;}],
  ['a pending save',()=>{c.saveTimer=7;},()=>{c.saveTimer=null;}],
  ['an open dialog',()=>selectors.add('#modalRoot .modal'),()=>selectors.delete('#modalRoot .modal')],
  ['an open session menu',()=>selectors.add('#sessionLibraryContextMenu'),()=>selectors.delete('#sessionLibraryContextMenu')],
  ['a drag in the Saved list',()=>selectors.add('.session-saved-dragging'),()=>selectors.delete('.session-saved-dragging')],
  ['typing in the sidebar',()=>{c.document.activeElement={tagName:'INPUT',closest:()=>({})};},()=>{c.document.activeElement=null;}]]){
  block();offer();
  assert.equal(await shared.poll(),false,name+' defers the refresh');assert.equal(shared.busy(),true);
  assert.deepEqual(c.state.terminalProfiles,[],name+': the list is untouched');assert.equal(shared.revision,'',name+': the page keeps its revision');
  release();
 }
 assert.equal(shared.busy(),false);offer();
 assert.equal(await shared.poll(),true);assert.deepEqual(c.state.terminalProfiles.map(p=>p.id),['new']);assert.equal(shared.revision,'r9');
 assert(c.requests.every(path=>path.startsWith('/shared-data?revision=')));assert.equal(c.requests.at(-1),'/shared-data?revision=');
 // Unchanged values: no redraw, the revision is simply recorded.
 calls.length=0;c.answers.push({revision:'r9'});assert.equal(await shared.poll(),false);assert.deepEqual(calls,[]);
 assert.equal(c.requests.at(-1),'/shared-data?revision=r9');
 // Same values under a new revision: nothing to redraw either.
 c.answers.push({revision:'r10',values:{terminalProfiles:[profile('new')]}});assert.equal(await shared.poll(),true);
 assert.deepEqual(calls,[]);assert.equal(shared.revision,'r10');
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
test('refreshed lists are cleaned like loaded ones and carry no unknown fields',()=>{
 const {shared}=harness();
 const clean=shared.normalize({terminalProfiles:[profile('a',{password:'secret-marker',folder:' MDF ',nameRevision:4,nameExplicit:true}),{id:7,name:'bad'},null],terminalRecent:[profile('r',{lastUsed:5}),profile('r',{lastUsed:9})],ftpProfiles:[{id:'f',name:'Files',host:'files.test',port:'990',security:'weird',password:'secret-marker'}],ftpRecent:[{id:'g',name:'Files',host:'files.test',port:21,username:'u',lastUsed:3,secret:'secret-marker'}],sftpVerifiedHostKeys:[],terminalSavedManualOrder:'yes'});
 assert.equal(JSON.stringify(clean).includes('secret-marker'),false);
 assert.deepEqual(clean.terminalProfiles.map(p=>[p.id,p.folder,p.nameRevision,p.nameExplicit]),[['a','MDF',4,true]]);
 assert.deepEqual(clean.terminalRecent.map(p=>[p.id,p.lastUsed]),[['r',5]]);
 assert.deepEqual(plain(clean.ftpProfiles),[{id:'f',name:'Files',folder:'',host:'files.test',port:990,username:'',security:'explicit',fingerprint:'',credentialId:''}]);
 assert.deepEqual(plain(clean.ftpRecent),[{id:'g',name:'Files',host:'files.test',port:21,username:'u',security:'explicit',fingerprint:'',credentialId:'',lastUsed:3}]);
 assert.deepEqual(plain(clean.sftpVerifiedHostKeys),{});assert.equal(clean.terminalSavedManualOrder,false);
 assert.deepEqual(Object.keys(clean).sort(),[...shared.LIVE].sort());
 assert.deepEqual(plain(shared.normalize(null).terminalProfiles),[]);
});
