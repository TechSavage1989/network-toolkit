'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(require.resolve('../web/assets/editor-saved-connections.js'),'utf8');
function harness(child=false){
 const rows=[{id:'ssh:p1',name:'Fixture',protocol:'ssh',host:'ssh.invalid',port:22,password:'never-expose',credentialId:'never-expose'},{id:'ftp:p2',name:'Files',protocol:'ftps',host:'files.invalid',port:990}],calls=[],opened=[],events={};
 const node=()=>({dataset:{},append(){},remove(){},className:'',textContent:''});
 const root={console,document:{addEventListener(n,f){events[n]=f},querySelector(){return null},createElement:node,body:node()},esc:String,toast(){},openModal(){},closeModal(){},NetworkToolkitEditorFiles:{detached:()=>child},NetworkToolkitRemoteBrowser:{useSavedEditorProfile:async(...args)=>{opened.push(args);return {source:'client:ssh',pending:false}}},NetworkToolkitFileEditor:{preferConnection(){}},api:async(path,method,body)=>{calls.push({path,method,body});if(method==='POST')return {requestId:'a'.repeat(32),pending:true};return {profiles:rows,pendingRequest:''}}};
 root.globalThis=root;vm.runInNewContext(source,root);return {root,module:root.NetworkToolkitEditorSavedConnections,calls,opened,rows};
}
test('listing is an allowlisted display-only read and never connects',async()=>{
 const h=harness();const result=await h.module.list();assert.equal(result.profiles.length,2);assert(!JSON.stringify(result).includes('never-expose'));assert.equal(h.opened.length,0);assert.equal(h.calls.length,1);assert.equal(h.calls[0].method,undefined);
});
test('main explicit action uses existing profile wrapper with checked display destination',async()=>{
 const h=harness();await h.module.list();const result=await h.module.connect('ssh:p1');assert.equal(result.source,'client:ssh');assert.equal(h.opened.length,1);assert.deepEqual(h.opened[0].slice(0,2),['ssh','p1']);assert.equal(h.opened[0][2].host,'ssh.invalid');
});
test('child only sees SSH and requests main; never invokes connection or credential APIs',async()=>{
 const h=harness(true);const result=await h.module.list();assert.equal(result.profiles.length,1);await assert.rejects(h.module.connect('ftp:p2'),/Refresh/);const answer=await h.module.connect('ssh:p1');assert(answer.requested);assert.equal(h.opened.length,0);assert(h.calls.every(c=>c.path.startsWith('/editor-views/saved-connections')));assert.equal(JSON.stringify(h.calls[1].body),JSON.stringify({id:'ssh:p1'}));
 await h.module.cancelRequest();assert.equal(h.calls.at(-1).path,'/editor-views/saved-connections/cancel');assert.equal(h.calls.at(-1).body.requestId,'a'.repeat(32));
});
test('child revoked listing fails visibly with no fallback catalog or connection',async()=>{
 const h=harness(true);h.root.api=async()=>{throw Error('Request file explorer access first')};await assert.rejects(h.module.list(),/access/);assert.equal(h.opened.length,0);
});
test('repeated explicit action is coalesced while original authentication is pending',async()=>{
 const h=harness();await h.module.list();let resolve;h.root.NetworkToolkitRemoteBrowser.useSavedEditorProfile=()=>new Promise(r=>resolve=r);const pending=h.module.connect('ssh:p1');await assert.rejects(h.module.connect('ssh:p1'),/pending/);resolve({pending:true,source:''});assert((await pending).pending);
});
