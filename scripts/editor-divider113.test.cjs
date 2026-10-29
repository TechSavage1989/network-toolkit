'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const compare=require('../web/assets/editor-compare.js'),{Element,document:makeDocument}=require('./terminal-dom-fixture.cjs');
Object.defineProperty(Element.prototype,'style',{get(){return this._style;},set(value){this._style=value;value.setProperty=(key,v)=>value[key]=v;value.getPropertyValue=key=>value[key]||'';}});
Object.assign(Element.prototype,{clientWidth:1000,clientHeight:300,scrollHeight:6000,scrollTop:0,scrollLeft:0,offsetHeight:500});
Element.prototype.replaceChildren=function(){this.children=[];};
Element.prototype.getBoundingClientRect=function(){let width=this.classList.contains('nt-editor-pane-resize')?6:this.classList.contains('nt-compare-map-panel')?72:this.clientWidth;return {left:100,top:0,right:100+width,bottom:500,width,height:500};};
function harness(comparing=true){
 const document=makeDocument(''),editor=document.createElement('section');let host=document.createElement('section');editor.className='nt-editor';document.append(editor);editor.append(host);const timers=new Map(),observers=[];let next=0;
 const context={document,console,setTimeout:fn=>{timers.set(++next,fn);return next},clearTimeout:id=>timers.delete(id),getComputedStyle:()=>({lineHeight:'20px',font:'13px monospace',paddingTop:'10px',paddingLeft:'10px',paddingRight:'10px'}),ResizeObserver:class{constructor(fn){this.fn=fn;observers.push(this)}observe(){}disconnect(){this.disconnected=true}}};
 vm.createContext(context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/editor-compare.js'),'utf8'),context);
 const docs=['a','b'].map(id=>({id,uid:id,content:'line one\nline two',editRevision:4,path:id,source:'Synthetic',dirty:true,undo:['before'],redo:[],view:{start:2,end:6,top:0,left:0}}));
 const options={container:host,comparing,documents:()=>docs,active:()=>docs[0],wordWrap:()=>false,syntax:()=>false,activate(){},edit(){throw Error('resizing must not edit')},undo(){throw Error('resizing must not undo')},save(){throw Error('resizing must not save')}};
 let api=context.NTKEditorCompare.create(options);
 const h={context,get host(){return host},docs,observers,options,timers,get api(){return api},get handle(){return host.querySelector('[data-compare-divider]')},get grid(){return host.querySelector('.nt-compare-grid')},value(){return Number(h.handle.getAttribute('aria-valuenow'))},event(type,values={}){h.handle.emit(type,h.handle,values)},async compute(){const fn=[...timers.values()][0];timers.clear();if(fn)await fn()},remount(){api.dispose();host.remove();host=document.createElement('section');editor.append(host);options.container=host;api=context.NTKEditorCompare.create(options)}};return h;
}
test('split bounds keep desktop minimums, compact panes and invalid widths safe',()=>{
 assert.deepEqual(compare.splitBounds(1000),{min:.16,max:.84});assert.deepEqual(compare.splitBounds(200),{min:.4,max:.6});assert.deepEqual(compare.splitBounds(100),{min:.5,max:.5});assert.deepEqual(compare.splitBounds(0),{min:.5,max:.5});
 assert.equal(compare.splitRatio(-9,1000),.16);assert.equal(compare.splitRatio(20,1000),.84);assert.equal(compare.splitRatio(NaN,1000),.5);
});
test('both editing modes expose keyboard separator with reset and stable document state',async()=>{
 for(const comparing of [true,false]){const h=harness(comparing);await h.compute();const before=h.docs.map(d=>JSON.stringify({content:d.content,dirty:d.dirty,undo:d.undo,redo:d.redo,revision:d.editRevision}));
  assert.equal(h.handle.getAttribute('role'),'separator');assert.equal(h.handle.getAttribute('aria-orientation'),'vertical');assert.equal(h.value(),50);
  h.event('keydown',{key:'ArrowRight'});assert(h.value()>50);h.event('keydown',{key:'Home'});assert.equal(h.value(),Number(h.handle.getAttribute('aria-valuemin')));h.event('keydown',{key:'End'});assert.equal(h.value(),Number(h.handle.getAttribute('aria-valuemax')));h.event('keydown',{key:'Enter'});assert.equal(h.value(),50);
  await h.compute();assert.deepEqual(h.docs.map(d=>JSON.stringify({content:d.content,dirty:d.dirty,undo:d.undo,redo:d.redo,revision:d.editRevision})),before);assert.deepEqual(Object.keys(h.api.snapshot()).sort(),['left','right','scale','sync']);
 }
});
test('pointer identity, cancel and Escape never leave an active or unwanted resize',()=>{
 const h=harness();h.event('pointerdown',{pointerId:1,button:2,clientX:103});h.event('pointermove',{pointerId:1,clientX:800});assert.equal(h.value(),50);
 h.event('pointerdown',{pointerId:2,button:0,clientX:103});h.event('pointermove',{pointerId:3,clientX:800});assert.equal(h.value(),50);h.event('pointermove',{pointerId:2,clientX:800});assert(h.value()>70);h.event('pointercancel',{pointerId:2});assert.equal(h.value(),50);assert(!h.handle.classList.contains('is-resizing'));
 h.event('pointerdown',{pointerId:4,button:0,clientX:103});h.event('pointermove',{pointerId:4,clientX:800});h.event('keydown',{key:'Escape'});assert.equal(h.value(),50);h.event('pointermove',{pointerId:4,clientX:900});assert.equal(h.value(),50);
 h.event('pointerdown',{pointerId:5,button:0,clientX:103});h.event('pointermove',{pointerId:5,clientX:700});h.event('pointerup',{pointerId:5});assert(h.value()>60);h.event('dblclick');assert.equal(h.value(),50);
});
test('responsive clamping and new comparison host inside the same editor retains only window-local ratio',()=>{
 const h=harness();h.event('keydown',{key:'End'});const wide=h.value();h.grid.clientWidth=240;h.observers[0].fn();assert(h.value()<=51);h.grid.clientWidth=1000;h.observers[0].fn();assert.equal(h.value(),wide);
 h.event('keydown',{key:'ArrowLeft'});const saved=h.value();h.remount();assert.equal(h.value(),saved);assert(h.observers[0].disconnected);assert.equal(harness().value(),50,'unrelated editor gets its own balanced default');
});
test('hidden width and disposal release a captured drag without touching textarea selection',()=>{
 const h=harness(),handle=h.handle,left=h.host.querySelector('[data-compare-text="left"]'),right=h.host.querySelector('[data-compare-text="right"]');
 left.setSelectionRange(2,7);right.setSelectionRange(3,8);h.event('keydown',{key:'ArrowRight'});const preferred=h.value();h.grid.clientWidth=0;h.observers[0].fn();assert.equal(h.value(),50);h.grid.clientWidth=1000;h.observers[0].fn();assert.equal(h.value(),preferred);assert.deepEqual([left.selectionStart,left.selectionEnd,right.selectionStart,right.selectionEnd],[2,7,3,8]);
 let captured=null;handle.setPointerCapture=id=>{captured=id};handle.hasPointerCapture=id=>captured===id;handle.releasePointerCapture=id=>{assert.equal(captured,id);captured=null};h.event('pointerdown',{pointerId:8,button:0,clientX:103});assert.equal(captured,8);h.api.dispose();assert.equal(captured,null);assert(!handle.classList.contains('is-resizing'));handle.emit('pointermove',handle,{pointerId:8,clientX:999});assert.deepEqual([left.selectionStart,left.selectionEnd,right.selectionStart,right.selectionEnd],[2,7,3,8]);
});
