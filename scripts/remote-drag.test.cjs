'use strict';
const {test}=require('node:test');const assert=require('node:assert/strict');const fs=require('node:fs');const vm=require('node:vm');const path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'../web/assets/remote-drag.js'),'utf8');
function initialize(){const calls=[];const global={location:{origin:'http://127.0.0.1:52007'},api:async(url,verb,payload)=>{calls.push({url,verb,payload});return {ticket:'ab'.repeat(24),filename:'remote.cfg'};}};global.globalThis=global;vm.runInNewContext(source,global);return {global,calls,tool:global.NetworkToolkitNativeDrag};}
test('native drag uses a short-lived ticket, not embedded SSH credentials or remote paths',async()=>{
 const {tool,calls}=initialize();await tool.prepare('session:1:file',()=>({protocol:'sftp',action:'stat',credentialId:'vault-reference',host:'router.example',username:'admin',path:'/network/remote.cfg'}));
 assert.equal(calls.length,1);assert.equal(calls[0].url,'/remote-browser/drag-ticket');
 const data=new Map();const event={dataTransfer:{setData:(key,value)=>data.set(key,value),effectAllowed:''}};
 assert.equal(tool.apply(event,'session:1:file'),true);
 const actual=data.get('DownloadURL');assert.ok(actual.startsWith('application/octet-stream:remote.cfg:http://127.0.0.1:52007/drag-download/'));
 for(const secret of ['do-not-expose','router.example','/network/remote.cfg','admin'])assert.equal(actual.includes(secret),false);
 assert.equal(tool.apply(event,'session:1:file'),false,'one ticket is attached at most once');
});
test('drag ticket payload is made lazily and cache reduces duplicate requests',async()=>{
 const {tool,calls}=initialize();await Promise.all([tool.prepare('key',()=>({protocol:'local',action:'stat',path:'C:/test.cfg'})),tool.prepare('key',()=>({protocol:'local',action:'stat',path:'C:/test.cfg'}))]);
 assert.equal(calls.length,1);assert.equal(tool.has('key'),true);
});

test('quick clicks and pointer movement cancel drag preparation before remote I/O',async()=>{
 const timers=new Map(),listeners={},calls=[];
 let serial=0;
 const context={location:{origin:'http://127.0.0.1:52007'},document:{addEventListener:(name,fn)=>{const prior=listeners[name];listeners[name]=event=>{prior?.(event);fn(event);};}},
  setTimeout:fn=>{const id=++serial;timers.set(id,fn);return id;},clearTimeout:id=>timers.delete(id),
  api:async(url,verb,payload)=>{calls.push({url,verb,payload});return {ticket:'ab'.repeat(24),filename:'config.txt'};}};
 context.globalThis=context;vm.runInNewContext(source,context);
 const tool=context.NetworkToolkitNativeDrag,row={isConnected:true,contains:target=>target==='inside'};
 tool.hover(row,'one',()=>({protocol:'ssh',action:'stat',path:'/config.txt'}));
 tool.hover(row,'one',()=>({protocol:'ssh',action:'stat',path:'/config.txt'}));
 assert.equal(timers.size,1);assert.equal(calls.length,0);
 listeners.pointerdown();assert.equal(timers.size,0);assert.equal(calls.length,0);
 tool.hover(row,'one',()=>({protocol:'ssh',action:'stat',path:'/config.txt'}));
 listeners.pointerout({target:'inside',relatedTarget:'outside'});assert.equal(timers.size,0);
 tool.hover(row,'one',()=>({protocol:'ssh',action:'stat',path:'/config.txt'}));
 const run=[...timers.values()][0];run();await new Promise(resolve=>setImmediate(resolve));
 assert.equal(calls.length,1);assert.equal(tool.has('one'),true);
});

test('native drag never submits manual authentication to a cached ticket',async()=>{for(const field of ['password','privateKey','passphrase','keyPath']){const {tool,calls}=initialize();assert.equal(await tool.prepare('manual',()=>({protocol:'ftp',action:'stat',[field]:'fixture-secret',path:'/config.txt'})),false);assert.equal(calls.length,0);assert.equal(tool.has('manual'),false);}});
