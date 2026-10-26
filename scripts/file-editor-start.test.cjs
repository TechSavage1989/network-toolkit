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
 return {api,document,first,c,click:action=>{const root=document.getElementById('networkToolkitFileEditor');root.emit('click',root.querySelector('[data-ed="'+action+'"]'));}};
}
function atStart(h,ta){assert.equal(ta.selectionStart,0);assert.equal(ta.selectionEnd,0);assert.equal(ta.scrollTop,0);assert.equal(ta.scrollLeft,0);assert.equal(h.document.querySelector('[data-ed-position]').textContent,'Ln 1, Col 1');assert.equal(h.document.querySelector('[data-ed-highlight]').style.transform,'translate(0px,0px)');assert.equal(h.document.querySelector('[data-ed-gutter-lines]').style.transform,'translateY(0px)');}
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

test('case, line transforms, sort, undo and redo retain the current viewport and selected contents',async()=>{
 const h=harness(),ta=await h.first();h.c.getComputedStyle=()=>({lineHeight:'20px'});
 for(const action of ['upper','lower','duplicate','trim','sortLines','undo','redo']){
  ta.setSelectionRange(230,290);ta.scrollTop=360;ta.scrollLeft=42;h.click(action);
  assert.equal(ta.scrollTop,360,action+' scrolled vertically');assert.equal(ta.scrollLeft,42,action+' scrolled horizontally');assert.equal(h.document.querySelector('[data-ed-highlight]').style.transform,'translate(-42px,-360px)');assert(ta.selectionStart<ta.value.length,action+' left the caret at EOF');
 }
});
test('dirty tab and editor closes use in-app dialogs; cancelling and stale documents never lose edits',async()=>{
 const h=harness(),ta=await h.first();h.c.confirm=h.c.prompt=()=>{throw Error('Native browser dialogs are disabled');};
 ta.setSelectionRange(0,4);h.click('upper');const root=h.document.getElementById('networkToolkitFileEditor');
 root.emit('click',root.querySelector('[data-ed-close-tab="first"]'));assert(root.querySelector('.nt-editor-dialog'));root.emit('click',root.querySelector('[data-ed-dialog-action="cancel"]'));await settle();assert.equal(h.api.documents.size,1);
 const closing=h.api.closeAll();root.emit('click',root.querySelector('[data-ed-dialog-action="cancel"]'));assert.equal(await closing,false);assert.equal(h.api.documents.size,1);assert(!root.hidden);
 const stale=h.api.closeAll();ta.value+='new edit';root.emit('input',ta);root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));assert.equal(await stale,false);assert.equal(h.api.documents.size,1);
 root.emit('click',root.querySelector('[data-ed-close-tab="first"]'));root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));await settle();assert.equal(h.api.documents.size,0);assert(root.hidden);
});
test('Go to line validates input inside the editor and goes to the requested logical line',async()=>{
 const h=harness(),ta=await h.first();h.c.getComputedStyle=()=>({lineHeight:'20px'});h.c.prompt=()=>{throw Error('Disabled prompt');};h.click('goto');const root=h.document.getElementById('networkToolkitFileEditor'),field=root.querySelector('[data-ed-dialog-input]');field.value='999999';root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));assert(root.querySelector('.nt-editor-dialog'));assert(root.querySelector('[data-ed-dialog-error]').textContent);
 field.value='30';root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));await settle();assert.equal(ta.value.slice(0,ta.selectionStart).split('\n').length,30);assert.equal(ta.scrollTop,560);assert.equal(h.api.active.dirty,false);
});
test('late save completion keeps newer edits dirty and cannot switch the active document',async()=>{
 const h=harness(),ta=await h.first(),pending=deferred(),doc=h.api.active;let request;doc.write=data=>{request=data;return pending.promise;};ta.setSelectionRange(0,4);h.click('upper');h.click('save');const sent=request.content;
 ta.setSelectionRange(0,4);h.click('lower');assert.notEqual(doc.content,sent);pending.resolve({hash:'new'});await settle();assert.equal(doc.saved,sent);assert(doc.dirty);
});

test('confirmation Enter on Cancel preserves edits, Escape dismisses, and accepting closes once',async()=>{
 const h=harness(),ta=await h.first();ta.setSelectionRange(0,4);h.click('upper');const root=h.document.getElementById('networkToolkitFileEditor');
 const closing=h.api.closeAll();root.emit('keydown',root.querySelector('[data-ed-dialog-action="cancel"]'),{key:'Enter'});assert.equal(await closing,false);assert.equal(h.api.documents.size,1);
 const escape=h.api.closeAll();root.emit('keydown',root.querySelector('[data-ed-dialog-action="accept"]'),{key:'Escape'});assert.equal(await escape,false);assert.equal(h.api.documents.size,1);
 const accept=h.api.closeAll();root.emit('keydown',root.querySelector('[data-ed-dialog-action="accept"]'),{key:'Enter'});assert.equal(await accept,true);assert.equal(h.api.documents.size,0);
});

test('a new local document acquires a real reader after Save As and reopens under its canonical file ID',async()=>{
 const h=harness();let contents='';h.c.api=async(_path,_method,body)=>{if(body.action==='write-text'){contents=body.content;return {hash:'written'};}assert.equal(body.action,'read-text');return {content:contents+'\nRead from disk',hash:'reloaded'};};h.click('new');await settle();const root=h.document.getElementById('networkToolkitFileEditor'),ta=h.document.querySelector('[data-ed-text]');ta.value='new file contents';root.emit('input',ta);h.click('save');root.querySelector('[data-ed-dialog-input]').value='C:\\Review\\new.txt';root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));await settle();assert.equal(h.api.active.source,'Local computer');assert.equal(h.api.active.id,'local:C:\\Review\\new.txt');h.click('reload');await settle();assert.equal(h.api.active.content,'new file contents\nRead from disk');assert.equal(h.api.active.dirty,false);
});
