'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {Element,document:makeDocument}=require('./terminal-dom-fixture.cjs');
Object.defineProperty(Element.prototype,'style',{get(){return this._style;},set(v){this._style=v;v.setProperty=(k,x)=>v[k]=x;v.getPropertyValue=k=>v[k]||'';}});
Object.assign(Element.prototype,{clientWidth:500,clientHeight:300,scrollHeight:30000,scrollTop:0,scrollLeft:0,offsetHeight:500});
Element.prototype.getBoundingClientRect=function(){return {top:0,left:0,right:500,bottom:30,height:20,width:500};};
const source=fs.readFileSync(require.resolve('../web/assets/editor-compare.js'),'utf8').replace('get result(){return result;}','get debug(){return {pixelReady,layoutGeneration,offsets};},get result(){return result;}');
function harness(){
 const document=makeDocument(''),host=document.createElement('section');document.append(host);const timers=new Map(),rafs=[];let seq=0;
 const context={document,console,setTimeout:fn=>{timers.set(++seq,fn);return seq;},clearTimeout:id=>timers.delete(id),requestAnimationFrame:fn=>rafs.push(fn),getComputedStyle:()=>({lineHeight:'20px',font:'13px monospace',paddingTop:'10px',paddingLeft:'10px',paddingRight:'10px'})};vm.createContext(context);vm.runInContext(source,context);
 const docs=['a','b'].map(id=>({id,uid:id+'-original',content:'old\n'.repeat(600),editRevision:1,path:id,source:'Synthetic'}));
 const api=context.NTKEditorCompare.create({container:host,documents:()=>docs,active:()=>docs[0],wordWrap:()=>true,syntax:()=>false,activate(){},edit(){},undo(){},save(){}});
 return {api,docs,rafs,timers,start(){const next=[...timers.values()][0];timers.clear();return next();},async frames(){for(let i=0;i<12&&rafs.length;i++){for(const fn of rafs.splice(0))fn();await Promise.resolve();}}};
}
test('same-path reopened documents cannot reuse the prior UID wrapped geometry',async()=>{
 const h=harness(),first=h.start();await h.frames();await first;assert.equal(h.api.debug.pixelReady,true);assert.equal(h.api.debug.offsets.left.length-1,601);
 h.docs[0]={...h.docs[0],uid:'a-reopened',content:'new',editRevision:1};h.api.refresh(true);await h.start();assert.equal(h.api.debug.pixelReady,true);assert.equal(h.api.result.lineCounts.left,1);assert.equal(h.api.debug.offsets.left.length-1,1);
});
test('an edit invalidates old asynchronous wrap measurements before their next frame',async()=>{
 const h=harness(),first=h.start();assert.equal(h.rafs.length,2);h.docs[0].content='new';h.docs[0].editRevision++;h.api.refresh();assert.equal(h.api.debug.pixelReady,false);await h.frames();await first;
 assert.equal(h.api.debug.pixelReady,false,'old geometry must remain unusable until the replacement completes');assert.equal(h.timers.size,1);const replacement=h.start();await h.frames();await replacement;assert.equal(h.api.debug.pixelReady,true);assert.equal(h.api.result.lineCounts.left,1);assert.equal(h.api.debug.offsets.left.length-1,1);
});
