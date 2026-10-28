'use strict';
const fs=require('node:fs'),vm=require('node:vm'),{test}=require('node:test'),assert=require('node:assert/strict');
const {document:makeDocument,Element}=require('./terminal-dom-fixture.cjs');
Element.prototype.replaceChildren=function(...children){this.children=[];this.append(...children);};
Object.defineProperty(Element.prototype,'options',{get(){return this.children;}});
Element.prototype.focus=function(){let top=this;while(top.parentElement)top=top.parentElement;top.activeElement=this;};
const source=fs.readFileSync(require.resolve('../web/assets/editor-windows.js'),'utf8');
function fixture(){
 const document=makeDocument('<div class="nt-editor-window"><div class="nt-editor-toolbar"></div></div>');
 const calls=[],docs=[{uid:'11111111-1111-4111-8111-111111111111',content:'synthetic retained draft',dirty:true}],events=[];let transfer='',viewName='Other window';
 const ctx={document,console,Uint8Array,AbortController,crypto:require('node:crypto').webcrypto,setTimeout:()=>1,clearTimeout(){}};vm.createContext(ctx);vm.runInContext(source,ctx);
 const adapter={documents:()=>docs,active:()=>docs[0]?.uid,canTransfer:()=>'',snapshot(ids){events.push('freeze');docs[0].frozen=true;return {documents:docs.map(d=>({...d})),active:docs[0].uid};},rollback(){docs[0].frozen=false;events.push('rollback');},release(){events.push('release');docs.splice(0);}};
 const controller=ctx.NetworkToolkitEditorWindows;controller.configure({adapter,available:()=>true,request:async(path,method,body)=>{calls.push({path,method,body});if(path.endsWith('/transfers')){transfer=body.id;return {id:transfer,state:'offered'};}if(path.endsWith('/cancel'))return {id:transfer,state:'cancelled'};return {available:true,ownerInstanceId:'engine',self:'',owned:{},incoming:[],outgoing:transfer?[{id:transfer,state:'offered'}]:[],views:[{id:'other',name:viewName,state:'ready'}]};}});
 return {document,controller,calls,docs,events,adapter,setName:v=>viewName=v,button:()=>document.querySelector('[data-editor-window-toggle]'),box:()=>document.querySelector('[data-editor-window-controls]')};
}
test('compact window action adds no row and opens a keyboard dialog without transfer side effects',async()=>{
 const f=fixture();await f.controller.poll();const button=f.button(),box=f.box();assert.equal(button.parentElement.className,'nt-editor-toolbar');assert.equal(box.hidden,true);assert.equal(button.getAttribute('aria-expanded'),'false');assert.equal(box.getAttribute('role'),'dialog');assert.equal(box.getAttribute('aria-modal'),'true');assert.equal(f.calls.filter(x=>x.method==='POST').length,0);
 button.emit('click',button);assert.equal(box.hidden,false);assert.equal(button.getAttribute('aria-expanded'),'true');assert.equal(f.document.activeElement,box.querySelector('select'));assert.equal(f.calls.filter(x=>x.method==='POST').length,0);
 const firstOption=box.querySelector('select').children[0];await f.controller.poll();assert.equal(box.querySelector('select').children[0],firstOption,'unchanged polling must not rebuild the live destination chooser');
 box.emit('keydown',box.querySelector('select'),{key:'Escape'});assert.equal(box.hidden,true);assert.equal(f.document.activeElement,button);assert.equal(f.docs[0].content,'synthetic retained draft');f.controller.stop();
});
test('move and cancellation stay reachable through the compact dialog and retain original ownership rules',async()=>{
 const f=fixture();await f.controller.poll();const box=f.box();f.button().emit('click',f.button());box.querySelector('select').value='other';box.emit('click',box.querySelector('[data-editor-window-move="tab"]'));await new Promise(resolve=>setImmediate(resolve));assert.equal(box.hidden,true);assert.equal(f.docs[0].frozen,true);assert.equal(f.controller.state.outgoing,1);assert.equal(f.calls.find(x=>x.path.endsWith('/transfers')).body.destination,'other');assert.match(f.button().getAttribute('aria-label'),/protected/);assert.equal(f.button().querySelector('[data-editor-window-indicator]').textContent,'…');assert.equal(f.button().querySelector('[data-editor-window-label]').textContent,'Move pending · details');
 f.button().emit('click',f.button());assert.equal(box.hidden,false);const cancel=box.querySelector('[data-editor-window-cancel]');assert.equal(cancel.hidden,false);box.emit('click',cancel);await new Promise(resolve=>setImmediate(resolve));assert.equal(box.hidden,true);assert.equal(f.controller.state.outgoing,0);assert.equal(f.docs[0].frozen,false);assert.equal(f.docs[0].content,'synthetic retained draft');assert.deepEqual(f.events,['freeze','rollback']);f.controller.stop();
});
test('menu Escape and close restore focus; Tab stays inside enabled visible controls',async()=>{
 const f=fixture();await f.controller.poll();const box=f.box();f.button().emit('click',f.button());const select=box.querySelector('select');f.document.activeElement=select;let prevented=false;box.emit('keydown',select,{key:'Tab',preventDefault(){prevented=true;}});assert(prevented);assert.equal(f.document.activeElement,box.querySelector('[data-editor-window-move="tab"]'));
 box.emit('click',box.querySelector('[data-editor-window-dismiss]'));assert.equal(box.hidden,true);assert.equal(f.document.activeElement,f.button());assert.equal(f.calls.filter(c=>c.method==='POST').length,0);f.controller.stop();
});

test('an actionable move rejection stays visibly discoverable without relying on a tooltip',async()=>{
 const f=fixture();await f.controller.poll();f.adapter.canTransfer=()=> 'This document cannot move without losing its existing file access.';f.button().emit('click',f.button());f.box().emit('click',f.box().querySelector('[data-editor-window-move="tab"]'));await new Promise(resolve=>setImmediate(resolve));assert.equal(f.box().hidden,true);assert.equal(f.button().querySelector('[data-editor-window-label]').textContent,'Window needs attention');f.button().emit('click',f.button());assert.match(f.box().querySelector('[data-editor-window-message]').textContent,/cannot move/);assert.equal(f.events.length,0);assert.equal(f.docs[0].content,'synthetic retained draft');f.controller.stop();
});
