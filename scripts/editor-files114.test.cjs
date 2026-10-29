'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const sidebarSource=fs.readFileSync(require.resolve('../web/assets/editor-files-sidebar.js'),'utf8');
function harness(child=false){
 const document=makeDocument('<section id="networkToolkitFileEditor"><button data-ed="files"></button><div id="ntEditorRemotePanel"></div><aside data-ed-files-sidebar></aside></section>');document.body=document;const editor=document.querySelector('#networkToolkitFileEditor'),host=editor.querySelector('[data-ed-files-sidebar]');editor.hidden=false;editor.clientWidth=900;host.hidden=true;const models={ssh:{connected:false},ftp:{connected:false}},calls=[],refresh=[];
 const root={document,EDITOR_WINDOW:child,clearTimeout,setTimeout,esc:String,api:async()=>({allowed:true}),NetworkToolkitFileEditor:{refreshConnectedSources:async()=>refresh.push('list')},NetworkToolkitRemoteBrowser:{mountSidebar:(...args)=>calls.push(['mount',...args]),unmountSidebar:()=>calls.push(['unmount']),get:k=>models[k],connectTerminal:async(k,id)=>{calls.push(['connect',k,id]);Object.assign(models[k],{connected:true,sessionId:id});}},console,listeners:{},addEventListener:(name,fn)=>root.listeners[name]=fn};root.globalThis=root;vm.runInNewContext(sidebarSource,root);return {root,editor,host,models,calls,refresh,api:root.NetworkToolkitEditorFiles};
}
test('Files moves the existing connection chooser rather than cloning controls or documents',()=>{
 const h=harness(),panel=h.editor.querySelector('#ntEditorRemotePanel');h.api.toggle(h.editor);assert.equal(h.host.hidden,false);assert.equal(h.host.querySelector('#ntEditorRemotePanel'),panel);assert.equal(panel.hidden,true);assert.equal(h.editor.querySelectorAll('#ntEditorRemotePanel').length,1);assert(!h.calls.some(x=>x[0]==='connect'));h.api.toggle(h.editor);assert.equal(h.host.hidden,true);
});
test('listing/mounting is not a connection; explicit source choice reuses only requested active SSH',async()=>{
 const h=harness();h.api.toggle(h.editor);await h.api.useSource('terminal:active-fixture');assert.deepEqual(h.calls.filter(x=>x[0]==='connect'),[['connect','ssh','active-fixture']]);assert.equal(h.host.querySelector('#ntEditorRemotePanel').hidden,true);assert.equal(h.host.dataset.editorSidebarPanel,'files');await h.api.useSource('terminal:active-fixture');assert.equal(h.calls.filter(x=>x[0]==='connect').length,1);
});
test('unrelated live or busy clients are not replaced by a Files selection',async()=>{
 for(const value of [{connected:true,sessionId:'original'},{busy:true},{connecting:true},{pendingClose:1}]){const h=harness();h.api.toggle(h.editor);Object.assign(h.models.ssh,value);await assert.rejects(h.api.useSource('terminal:new'),/Disconnect|operation/);assert(!h.calls.some(x=>x[0]==='connect'));}
});
test('child FTP and closed/missing sources fail without fallback or transport action',async()=>{
 const h=harness(true);h.host.hidden=false;await assert.rejects(h.api.useSource('client:ftp'),/approved SFTP/);await assert.rejects(h.api.useSource('saved:ssh:fixture'),/active file/);await assert.rejects(h.api.useSource('client:ssh'),/closed/);assert(!h.calls.some(x=>x[0]==='connect'));
});
test('existing main FTP stays a client operation and does not navigate or reconnect',async()=>{
 const h=harness();h.models.ftp.connected=true;h.api.toggle(h.editor);await h.api.useSource('client:ftp');assert.equal(h.calls.at(-1)[0],'mount');assert.equal(h.calls.at(-1)[3],'ftp');assert(!h.calls.some(x=>x[0]==='connect'));
});
test('drive inventory preserves mounted paths, normalizes duplicates and rejects control characters',()=>{
 const source=fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8'),start=source.indexOf('function localDriveChoices('),end=source.indexOf('function paintDrives(',start),root={Set};vm.runInNewContext(source.slice(start,end),root);
 const rows=root.localDriveChoices({drives:[{path:'C:/',label:'Local disk',kind:'local'},{path:'c:\\',label:'Duplicate'},{path:'D:/',label:'USB',kind:'removable'},{path:'C:/mount/data/',label:'Mounted data',kind:'local'},{path:'Z:/',label:'Network',kind:'network'},{path:'bad\u0000'}]});assert.deepEqual(Array.from(rows,x=>x.path),['C:/','D:/','C:/mount/data/','Z:/']);assert.equal(rows[3].kind,'network');assert.equal(root.localDriveChoices({roots:['/']})[0].path,'/');assert.equal(root.localDriveChoices({roots:Array.from({length:200},(_,i)=>'/'+i)}).length,128);
});

test('closing Files during a pending explicit Use never reopens it, and repeated Use is refused',async()=>{
 const h=harness();h.api.toggle(h.editor);let release;h.root.NetworkToolkitRemoteBrowser.connectTerminal=(kind,id)=>new Promise(resolve=>{release=()=>{Object.assign(h.models[kind],{connected:true,sessionId:id});resolve();};});
 const pending=h.api.useSource('terminal:fixture');await assert.rejects(h.api.useSource('terminal:fixture'),/current file connection action/);h.api.toggle(h.editor);const mounts=h.calls.filter(x=>x[0]==='mount').length;release();await pending;assert.equal(h.host.hidden,true);assert.equal(h.calls.filter(x=>x[0]==='mount').length,mounts);
});

// Rendering an approved child explorer must never probe the main credential API.
test('credential refresh is main-only for both sidebar protocols',async()=>{
 const source=fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8'),start=source.indexOf("async function ftpCredentialsRefresh("),end=source.indexOf('function ftpProfilePicker(',start);
 for(const child of [true,false])for(const protocol of ['ssh','ftp']){
  const calls=[],root={detachedEditor:()=>child,root:{NetworkToolkitCredentials:{refresh:async()=>calls.push('refresh')}},syncCredentials:k=>calls.push('sync:'+k),note:()=>calls.push('note')};
  vm.runInNewContext(source.slice(start,end),root);await root.ftpCredentialsRefresh(protocol);assert.deepEqual(calls,child?[]:['refresh','sync:'+protocol]);
 }
});


test('main sidebar preferences use the app lexical state, retain tabs and collapse without unmounting',()=>{
 const h=harness();vm.runInNewContext("let state={editorFilesPreferences:{width:360,panel:'connections',side:'remote',collapsed:false}};globalThis.savedCount=0;globalThis.saveState=()=>savedCount++;",h.root);
 h.api.initialize(h.editor);assert.equal(h.host.hidden,false);assert.equal(h.host.style.width,'360px');assert.equal(h.host.dataset.editorSidebarPanel,'connections');const browser=h.host.querySelector('[data-editor-file-browser]'),sessions=h.host.querySelector('#ntEditorRemotePanel');
 h.api.selectPanel('files');assert.equal(h.host.querySelector('[data-editor-file-browser]'),browser);assert.equal(h.host.querySelector('#ntEditorRemotePanel'),sessions);assert.equal(sessions.hidden,true);h.api.toggle(h.editor);assert.equal(h.host.hidden,true);assert(!h.calls.some(c=>c[0]==='unmount'));assert.equal(vm.runInNewContext('state.editorFilesPreferences.collapsed',h.root),true);assert(h.root.savedCount>0);
});
test('drive buttons always identify roots and preserve real labels safely',()=>{
 const source=fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8'),start=source.indexOf('function driveButtonLabel('),end=source.indexOf('function paint(k,',start),root={};vm.runInNewContext(source.slice(start,end),root);
 assert.equal(root.driveButtonLabel({path:'C:/',label:'Windows'}),'C: Windows');assert.equal(root.driveButtonLabel({path:'D:/',label:'Local drive'}),'D: · Local drive');assert.equal(root.driveButtonLabel({path:'Z:/',label:'Z: Offline mapping'}),'Z: Offline mapping');assert.equal(root.driveButtonLabel({path:'/',label:'Filesystem'}),'/ Filesystem');
 const doc=makeDocument('<nav data-rb-drives></nav>');root.escape=v=>String(v).replaceAll('&','&amp;').replaceAll('"','&quot;').replaceAll('<','&lt;');root.paintDrives(doc,{path:'D:/folder',drives:[{path:'C:/',label:'Windows'},{path:'D:/',label:'Data <unsafe>'}]});assert.equal(doc.querySelectorAll('[data-rb-drive]').length,2);assert.equal(doc.querySelector('[aria-pressed="true"]').dataset.rbDrive,'D:/');assert(!doc.querySelector('unsafe'));
});

test('Files and Connections visibility changes reuse the mounted form and never dispatch a transport',()=>{
 const source=fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8'),start=source.indexOf('function mountSidebar('),end=source.indexOf('function unmountSidebar(',start),document=makeDocument('<div id="host"></div>'),host=document.querySelector('#host'),models={ssh:{}},calls=[];
 const root={TYPES:new Set(['ssh','ftp']),detachedEditor:()=>false,sidebarMounts:new Map(),get:k=>models[k],render:k=>{calls.push('render');host.innerHTML='<div data-rb-root="ssh"><input data-rb-setting="password"></div>';},restoreConnectionControls:()=>calls.push('controls'),updateSidebarActions:()=>calls.push('actions')};vm.runInNewContext(source.slice(start,end),root);
 root.mountSidebar(host,'local','ssh',{panel:'files'});const input=host.querySelector('input');input.value='synthetic';root.mountSidebar(host,'remote','ssh',{panel:'connections'});assert.equal(host.querySelector('input'),input);assert.equal(input.value,'synthetic');assert.equal(host.querySelector('[data-rb-root]').dataset.sidebarPanel,'connections');assert.equal(calls.filter(v=>v==='render').length,1);
 root.sidebarMounts.get('ssh').dirty=true;root.mountSidebar(host,'remote','ssh',{panel:'files'});assert.equal(calls.filter(v=>v==='render').length,2,'a model update while collapsed is repainted on reopen');
});

test('older detached access rejection cannot erase a newer approved Connections panel',async()=>{
 const h=harness(true),pending=[];h.root.api=()=>new Promise((resolve,reject)=>pending.push({resolve,reject}));h.root.NetworkToolkitRemoteBrowser.mountSidebar=host=>{host.innerHTML='<p>New authorized browser</p>';};
 h.api.toggle(h.editor);h.api.selectPanel('connections');assert.equal(pending.length,2);pending[1].resolve({allowed:true});await new Promise(resolve=>setImmediate(resolve));pending[0].reject(Error('old request failed'));await new Promise(resolve=>setImmediate(resolve));assert.equal(h.host.querySelector('[data-editor-file-browser]').textContent,'New authorized browser');
});
test('older access-request rejection cannot overwrite a panel changed after the click',async()=>{
 const h=harness(true);let rejectPost;h.root.api=(_path,method)=>method==='POST'?new Promise((_resolve,reject)=>{rejectPost=reject;}):Promise.resolve({allowed:false,pending:false});
 h.api.toggle(h.editor);await new Promise(resolve=>setImmediate(resolve));const body=h.host.querySelector('[data-editor-file-browser]'),button=body.querySelector('button'),pending=button.onclick();h.root.api=async()=>({allowed:true});h.root.NetworkToolkitRemoteBrowser.mountSidebar=host=>{host.innerHTML='<p>New authorized browser</p>';};h.api.selectPanel('connections');await new Promise(resolve=>setImmediate(resolve));rejectPost(Error('old request failed'));await pending;assert.equal(body.textContent,'New authorized browser');
});
test('standalone client rendering marks a retained hidden editor form stale before returning',()=>{
 const source=fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8'),start=source.indexOf('function render(k)'),end=source.indexOf('function connectionIdentity(',start),stored={host:{}},root={TYPES:new Set(['ssh']),loadSaved:()=>{},sidebarMount:()=>null,sidebarMounts:new Map([['ssh',stored]]),pageActive:()=>true,document:{getElementById:()=>null}};vm.runInNewContext(source.slice(start,end),root);root.render('ssh');assert.equal(stored.dirty,true);
});


test('narrow entry collapses without overwriting wide preference and restores the retained form on widening',()=>{
 const h=harness();h.root.innerWidth=1440;vm.runInNewContext("let state={editorFilesPreferences:{width:420,panel:'connections',side:'remote',collapsed:false}};globalThis.saveState=()=>{};",h.root);h.api.initialize(h.editor);
 const body=h.host.querySelector('[data-editor-file-browser]'),panel=h.host.querySelector('#ntEditorRemotePanel');assert.equal(h.host.hidden,false);const mounts=h.calls.filter(c=>c[0]==='mount').length;
 h.root.innerWidth=390;h.root.listeners.resize();assert.equal(h.host.hidden,true);assert.equal(vm.runInNewContext('state.editorFilesPreferences.collapsed',h.root),false);assert.equal(h.host.querySelector('[data-editor-file-browser]'),body);assert.equal(h.host.querySelector('#ntEditorRemotePanel'),panel);assert.equal(h.host.style.width,'420px');
 h.root.listeners.resize();assert.equal(h.calls.filter(c=>c[0]==='mount').length,mounts);h.api.toggle(h.editor);assert.equal(h.host.hidden,false);h.root.innerWidth=1440;h.root.listeners.resize();assert.equal(h.host.hidden,false);assert.equal(h.host.style.width,'420px');assert(!h.calls.some(c=>c[0]==='unmount'||c[0]==='connect'));
 h.api.toggle(h.editor);assert.equal(vm.runInNewContext('state.editorFilesPreferences.collapsed',h.root),true);h.root.innerWidth=390;h.root.listeners.resize();h.api.toggle(h.editor);assert.equal(h.host.hidden,false);h.root.innerWidth=1440;h.root.listeners.resize();assert.equal(h.host.hidden,true,'compact open does not replace the collapsed desktop preference');
});
test('initial compact workspace ignores saved expanded desktop state until explicit opening',()=>{
 for(const child of [false,true]){const h=harness(child);h.root.innerWidth=390;h.root.sessionStorage={getItem:()=>JSON.stringify({width:380,collapsed:false,panel:'files'})};if(!child)vm.runInNewContext("let state={editorFilesPreferences:{width:380,collapsed:false,panel:'files'}};globalThis.saveState=()=>{};",h.root);
 h.api.initialize(h.editor);assert.equal(h.host.hidden,true);assert.equal(h.calls.length,0);h.api.toggle(h.editor);assert.equal(h.host.hidden,false);h.api.initialize(h.editor);assert.equal(h.host.hidden,false,'normal rerender preserves explicit compact open');h.root.innerWidth=1440;h.root.listeners.resize();assert.equal(h.host.hidden,false);assert.equal(h.host.style.width,'380px');}
});
