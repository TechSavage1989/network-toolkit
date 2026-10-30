'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(require.resolve('../web/assets/file-editor.js'),'utf8'),code=source.slice(source.indexOf("const sidebarFileMIME="),source.indexOf('function skin(){'));
const MIME='application/x-networktoolkit-remote-browser';
function harness({side='local',k='ssh',files=[{path:'C:/Synthetic/a.txt',directory:false}]}={}){
 const listeners={},calls=[],messages=[],host={dataset:{rbRoot:k},isConnected:true},pane={entries:files,selected:new Set(files.map(f=>f.path))},model={local:pane,remote:pane,connected:true,busy:false,connecting:false};let identity='fixture-identity';
 const row={dataset:{rbItem:files[0].path,rbSide:side},closest:s=>s==='[data-rb-root]'?host:null};
 const root={NetworkToolkitRemoteBrowser:{get:()=>model,dragIdentity:()=>identity,edit:async(...args)=>calls.push(args)}};
 const context={root,windowCloseState:null,canReorderDocuments:()=>'',document:{addEventListener:(name,fn)=>listeners[name]=fn},setMessage:(...v)=>messages.push(v),toast:()=>{}};
 vm.runInNewContext(code,context);let payload={k,side,files};
 const event=(trusted=true)=>({isTrusted:trusted,target:{closest:s=>s==='[data-rb-item]'?row:s==='.nt-editor-document-area'?{}:null},dataTransfer:{types:[MIME],getData:()=>JSON.stringify(payload),dropEffect:'none'},preventDefault(){this.prevented=true;},stopPropagation(){this.stopped=true;}});
 return {context,root,model,pane,host,row,calls,messages,listeners,event,capture:trusted=>context.captureSidebarFileDrag(event(trusted)),drop:trusted=>context.openSidebarFileDrop(event(trusted)),setPayload:value=>payload=value,changeIdentity:()=>identity='other-identity'};
}
test('trusted local and SFTP drops reuse canonical edit pipeline, not importing path text',async()=>{
 for(const side of ['local','remote']){const h=harness({side});h.capture();const ev=h.event();h.context.editorFileDragOver(ev);assert.equal(ev.dataTransfer.dropEffect,'copy');assert(ev.prevented&&ev.stopped);await h.drop();assert.deepEqual(h.calls,[['ssh',false,side,'C:/Synthetic/a.txt']]);assert.equal(await h.drop(),true);assert.equal(h.calls.length,1,'consumed drop cannot replay');}
});
test('foreign MIME, untrusted source/drop and malformed or substituted paths never open',async()=>{
 for(const mode of ['missing','untrustedStart','untrustedDrop','malformed','substitute']){const h=harness();if(mode!=='missing')h.capture(mode==='untrustedStart'?false:true);if(mode==='substitute')h.setPayload({k:'ssh',side:'local',files:[{path:'C:/different.txt',directory:false}]});if(mode==='malformed')h.setPayload(null);await h.drop(mode!=='untrustedDrop');assert.equal(h.calls.length,0,mode);assert(h.messages.at(-1)[1]);}
 const h=harness();h.capture();const ev=h.event();ev.dataTransfer.types=['text/plain'];assert.equal(await h.context.openSidebarFileDrop(ev),false);assert(!ev.prevented);assert.equal(h.calls.length,0);
});
test('folders, expired host/listing, busy clients and replaced SFTP transports fail closed',async()=>{
 for(const mode of ['folder','host','listing','busy','connecting','disconnected','transport','ftp']){const h=harness({side:'remote',k:mode==='ftp'?'ftp':'ssh',files:[{path:'/synthetic/a',directory:mode==='folder'}]});h.capture();if(mode==='host')h.host.isConnected=false;if(mode==='listing')h.pane.entries=[...h.pane.entries];if(mode==='busy')h.model.busy=true;if(mode==='connecting')h.model.connecting=true;if(mode==='disconnected')h.model.connected=false;if(mode==='transport')h.changeIdentity();await h.drop();assert.equal(h.calls.length,0,mode);assert(h.messages.at(-1)[1]);}
});
test('selection is fixed at dragstart while every subsequent file rechecks source identity',async()=>{
 const files=[{path:'/synthetic/a.txt',directory:false},{path:'/synthetic/b.txt',directory:false}],h=harness({side:'remote',files});h.capture();h.pane.selected=new Set();h.root.NetworkToolkitRemoteBrowser.edit=async(...args)=>{h.calls.push(args);h.changeIdentity();};await h.drop();assert.deepEqual(h.calls,[['ssh',false,'remote','/synthetic/a.txt']]);assert.match(h.messages.at(-1)[0],/Remaining files/);
});
test('cancelled gesture and closing editor cannot read or modify files',async()=>{
 for(const mode of ['dragend','Escape','close']){const h=harness();h.capture();if(mode==='dragend')h.listeners.dragend();else if(mode==='Escape')h.listeners.keydown({key:'Escape'});else h.context.windowCloseState={};await h.drop();assert.equal(h.calls.length,0,mode);}
});
test('read errors retain normal feedback and a second rejected drop cannot unlock the first read',async()=>{
 const h=harness();let release;h.root.NetworkToolkitRemoteBrowser.edit=()=>new Promise(resolve=>release=resolve);h.capture();const first=h.drop();h.capture();await h.drop();h.capture();const over=h.event();h.context.editorFileDragOver(over);assert.equal(over.dataTransfer.dropEffect,'none');assert.match(h.messages.at(-1)[0],/current editor operation/);release();await first;h.root.NetworkToolkitRemoteBrowser.edit=async()=>{throw Error('Binary files cannot be opened');};h.capture();await h.drop();assert.match(h.messages.at(-1)[0],/Binary/);
});
test('handler is capture-phase before text editing defaults; ordinary OS imports remain',()=>{
 assert(source.includes("overlay.addEventListener('dragover',editorFileDragOver,true)"));assert(source.includes("void openSidebarFileDrop(event);},true)"));assert(source.includes('void importFiles(files)'));assert(!code.includes("api("));assert(!code.includes('connect('));
});
test('native pointer bridge accepts only matching real event objects and derives its own target',async()=>{
 const h=harness();class Pointer{constructor(type,x=0){Object.assign(this,h.event(),{type,button:0,pointerId:7,clientX:x,clientY:1});}}h.root.PointerEvent=Pointer;h.context.document.elementFromPoint=()=>h.event().target;
 assert.equal(h.context.beginSidebarPointerDrag({...new Pointer('pointerdown')}),false,'Plain object cannot impersonate native event');
 assert.equal(h.context.beginSidebarPointerDrag(new Pointer('pointerdown')),true);assert.equal(h.context.finishSidebarPointerDrop(new Pointer('pointerup',20)),true);await new Promise(resolve=>setImmediate(resolve));assert.equal(h.calls.length,1);
 for(const mode of ['wrongPointer','short','cancel','untrusted','outside']){h.context.beginSidebarPointerDrag(new Pointer('pointerdown'));const up=new Pointer('pointerup',20);if(mode==='wrongPointer')up.pointerId=8;if(mode==='short')up.clientX=2;if(mode==='cancel')h.context.cancelSidebarPointerDrag();if(mode==='untrusted')up.isTrusted=false;if(mode==='outside')h.context.document.elementFromPoint=()=>({closest:()=>null});assert.equal(h.context.finishSidebarPointerDrop(up),false,mode);}
 assert.equal(h.calls.length,1);
});
test('pending read completion does not discard a newer captured drag',async()=>{
 const h=harness();let release;h.root.NetworkToolkitRemoteBrowser.edit=()=>new Promise(resolve=>release=resolve);h.capture();const first=h.drop();h.capture();release();await first;h.root.NetworkToolkitRemoteBrowser.edit=async(...args)=>h.calls.push(args);await h.drop();assert.equal(h.calls.length,1);
});
test('existing pending dialog, composition, save and ownership guards apply before any dropped read',async()=>{
 const h=harness();h.context.canReorderDocuments=()=> 'Finish pending editor operations before rearranging tabs.';h.capture();await h.drop();assert.equal(h.calls.length,0);assert.match(h.messages.at(-1)[0],/pending editor operations/);
});
