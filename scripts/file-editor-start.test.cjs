'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const source=fs.readFileSync(process.env.TOOLKIT_EDITOR_SOURCE||require.resolve('../web/assets/file-editor.js'),'utf8');
const text=Array.from({length:300},(_,i)=>'Line '+(i+1)+' '+ 'x'.repeat(100)).join('\n');
const deferred=()=>{let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b;});return {promise,resolve,reject};};
const settle=()=>new Promise(resolve=>setImmediate(resolve));
function harness(){
 const document=makeDocument('');document.body=document;
 const create=document.createElement;document.createElement=tag=>{const el=create(tag);el.style.setProperty=(k,v)=>el.style[k]=v;el.style.removeProperty=k=>delete el.style[k];return el;};
 const c={document,console,Map,Set,Date,Math,TextDecoder,TextEncoder,Uint8Array,DataView,Blob,URL,setTimeout,clearTimeout,state:{},saveState(){},confirm:()=>true,esc:x=>String(x).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),addEventListener(){}};
 vm.createContext(c);vm.runInContext(source,c);const api=c.NetworkToolkitFileEditor;
 async function first(){
  const pending=deferred(),running=api.open({id:'first',path:'first.txt',source:'Local computer',read:()=>pending.promise,write:async()=>({})}),ta=document.querySelector('[data-ed-text]');
  let value=ta.value||'';
  // The native textarea setter moves the caret to the end of a new value.
  Object.defineProperty(ta,'value',{get:()=>value,set:next=>{if(value!==next){value=next;ta.selectionStart=ta.selectionEnd=value.length;}}});
  ta.focus=()=>{document.activeElement=ta;if(ta.selectionStart===value.length&&value.length){ta.scrollTop=900;ta.scrollLeft=200;}};
  pending.resolve({content:text});await running;return ta;
 }
 return {api,document,first,click:action=>{const root=document.getElementById('networkToolkitFileEditor');root.emit('click',root.querySelector('[data-ed="'+action+'"]'));}};
}
function atStart(h,ta){assert.equal(ta.selectionStart,0);assert.equal(ta.selectionEnd,0);assert.equal(ta.scrollTop,0);assert.equal(ta.scrollLeft,0);assert.equal(h.document.querySelector('[data-ed-position]').textContent,'Ln 1, Col 1');assert.equal(h.document.querySelector('[data-ed-highlight]').style.transform,'translate(0px,0px)');assert.equal(h.document.querySelector('[data-ed-gutter]').style.transform,'translateY(0px)');}
test('loaded files, imported content and explicit reopens show the document start without changing text',async()=>{
 const h=harness(),ta=await h.first();atStart(h,ta);assert.equal(ta.value,text);assert.equal(h.api.active.dirty,false);
 ta.scrollTop=900;ta.scrollLeft=200;
 await h.api.open({id:'imported',path:'imported.txt',source:'Desktop import',create:true,initialContent:text+'\nImported',read:async()=>({content:''}),write:async()=>({})});atStart(h,ta);assert.equal(ta.value,text+'\nImported');
 await h.api.open({id:'first',path:'first.txt',read:async()=>{throw Error('Must reuse the open file');},write:async()=>({})});atStart(h,ta);assert.equal(ta.value,text);
});
test('reload resets the active file but a background completion leaves the other document view untouched',async()=>{
 const h=harness(),ta=await h.first();let pending=deferred();h.api.active.read=()=>pending.promise;
 ta.selectionStart=ta.selectionEnd=text.length;ta.scrollTop=900;h.click('reload');pending.resolve({content:text+'\nReloaded'});await settle();atStart(h,ta);assert(ta.value.endsWith('Reloaded'));
 pending=deferred();h.api.active.read=()=>pending.promise;h.click('reload');
 await h.api.open({id:'other',path:'other.txt',create:true,initialContent:text+'\nOther',read:async()=>({content:''}),write:async()=>({})});
 ta.selectionStart=ta.selectionEnd=120;ta.scrollTop=444;ta.scrollLeft=33;
 pending.resolve({content:text+'\nBackground result'});await settle();
 assert.equal(h.api.active.id,'other');assert(ta.value.endsWith('Other'));assert.equal(ta.selectionStart,120);assert.equal(ta.scrollTop,444);assert.equal(ta.scrollLeft,33);
 const tab=h.document.querySelector('[data-ed-open="first"]');h.document.getElementById('networkToolkitFileEditor').emit('click',tab);atStart(h,ta);assert(ta.value.endsWith('Background result'));
});
