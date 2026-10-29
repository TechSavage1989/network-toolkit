'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const sidebarSource=fs.readFileSync(require.resolve('../web/assets/editor-files-sidebar.js'),'utf8');
function harness(child=false){
 const document=makeDocument('<section id="networkToolkitFileEditor"><button data-ed="files"></button><div id="ntEditorRemotePanel"></div><aside data-ed-files-sidebar></aside></section>');document.body=document;const editor=document.querySelector('#networkToolkitFileEditor'),host=editor.querySelector('[data-ed-files-sidebar]');editor.hidden=false;editor.clientWidth=900;host.hidden=true;const models={ssh:{connected:false},ftp:{connected:false}},calls=[],refresh=[];
 const root={document,EDITOR_WINDOW:child,clearTimeout,setTimeout,esc:String,api:async()=>({allowed:true}),NetworkToolkitFileEditor:{refreshConnectedSources:async()=>refresh.push('list')},NetworkToolkitRemoteBrowser:{mountSidebar:(...args)=>calls.push(['mount',...args]),unmountSidebar:()=>calls.push(['unmount']),get:k=>models[k],connectTerminal:async(k,id)=>{calls.push(['connect',k,id]);Object.assign(models[k],{connected:true,sessionId:id});}},console};root.globalThis=root;vm.runInNewContext(sidebarSource,root);return {root,editor,host,models,calls,refresh,api:root.NetworkToolkitEditorFiles};
}
test('Files moves the existing connection chooser rather than cloning controls or documents',()=>{
 const h=harness(),panel=h.editor.querySelector('#ntEditorRemotePanel');h.api.toggle(h.editor);assert.equal(h.host.hidden,false);assert.equal(h.host.querySelector('#ntEditorRemotePanel'),panel);assert.equal(panel.hidden,true);assert.equal(h.editor.querySelectorAll('#ntEditorRemotePanel').length,1);assert(!h.calls.some(x=>x[0]==='connect'));h.api.toggle(h.editor);assert.equal(h.host.hidden,true);
});
test('listing/mounting is not a connection; explicit source choice reuses only requested active SSH',async()=>{
 const h=harness();h.api.toggle(h.editor);await h.api.useSource('terminal:active-fixture');assert.deepEqual(h.calls.filter(x=>x[0]==='connect'),[['connect','ssh','active-fixture']]);assert.equal(h.host.querySelector('#ntEditorRemotePanel').hidden,false);await h.api.useSource('terminal:active-fixture');assert.equal(h.calls.filter(x=>x[0]==='connect').length,1);
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
