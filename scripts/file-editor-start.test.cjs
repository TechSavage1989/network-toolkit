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
 vm.createContext(c);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/editor-tab-groups.js'),'utf8'),c);vm.runInContext(source,c);const api=c.NetworkToolkitFileEditor;
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
 const h=harness();await h.first();let contents='';h.c.api=async(_path,_method,body)=>{if(body.action==='write-text'){contents=body.content;return {hash:'written'};}assert.equal(body.action,'read-text');return {content:contents+'\nRead from disk',hash:'reloaded'};};h.click('new');await settle();const root=h.document.getElementById('networkToolkitFileEditor'),ta=h.document.querySelector('[data-ed-text]');ta.value='new file contents';root.emit('input',ta);h.click('save');root.querySelector('[data-ed-dialog-input]').value='C:\\Review\\new.txt';root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));await settle();assert.equal(h.api.active.source,'Local computer');assert.equal(h.api.active.id,'local:C:\\Review\\new.txt');h.click('reload');await settle();assert.equal(h.api.active.content,'new file contents\nRead from disk');assert.equal(h.api.active.dirty,false);
});


test('first paste after the final tab closes creates an unsaved document, binds tools and survives navigation',async()=>{
 const h=harness();h.c.OFFLINE=true;h.document.body.appendChild=h.document.body.append;await h.first();await h.api.closeAll();const mount=h.document.createElement('div');h.document.append(mount);h.api.renderPage(mount);const root=h.document.getElementById('networkToolkitFileEditor'),ta=root.querySelector('[data-ed-text]');
 assert.equal(h.api.documents.size,0);ta.value='pasted first line\nsecond line';ta.setSelectionRange(5,12);ta.scrollTop=12;root.emit('input',ta);await settle();
 assert.equal(h.api.documents.size,1,'first input must enter the document model');assert.equal(h.api.active.content,'pasted first line\nsecond line');assert.equal(h.api.active.dirty,true);assert.deepEqual(Array.from(h.api.active.undo),['']);assert.equal(ta.selectionStart,5);assert.equal(ta.selectionEnd,12);
 h.click('undo');assert.equal(h.api.active.content,'');h.click('redo');assert.equal(h.api.active.content,'pasted first line\nsecond line');
 h.api.detachPage();h.api.renderPage(mount);assert.equal(root.querySelector('[data-ed-text]').value,'pasted first line\nsecond line');assert.equal(h.api.active.dirty,true);
});

test('tab keyboard switching retains per-document views and pending saves block destructive closes',async()=>{
 const h=harness(),ta=await h.first();ta.setSelectionRange(40,49);ta.scrollTop=230;ta.scrollLeft=11;await h.api.open({id:'second',source:'Local computer',path:'second.txt',create:true,initialContent:'second',read:async()=>({content:''}),write:async()=>({})});
 const root=h.document.getElementById('networkToolkitFileEditor');root.emit('keydown',root.querySelector('[data-ed-open="second"]'),{key:'ArrowLeft'});assert.equal(h.api.active.id,'first');assert.equal(ta.selectionStart,40);assert.equal(ta.scrollTop,230);assert.equal(ta.scrollLeft,11);
 const pending=deferred();h.api.active.write=()=>pending.promise;h.click('save');assert(h.api.active.saving);assert.equal(await h.api.closeAll(),false);assert.equal(h.api.documents.size,2);pending.resolve({hash:'saved'});await settle();assert.equal(h.api.active.saving,false);
});


test('window snapshots freeze atomically, preserve undo and views, and rollback without losing the draft',async()=>{
 const h=harness(),ta=await h.first();h.api.active.binding={kind:'local'};ta.setSelectionRange(3,7);ta.scrollTop=150;ta.scrollLeft=10;h.click('upper');const doc=h.api.active,uid=doc.uid,contents=doc.content,undo=Array.from(doc.undo),adapter=h.api.windowAdapter;
 assert.equal(adapter.canTransfer([uid]),'');const snapshot=adapter.snapshot([uid]);assert(doc.frozen);assert.equal(snapshot.documents[0].content,contents);assert.deepEqual(Array.from(snapshot.documents[0].undo),undo);assert.equal(snapshot.documents[0].view.top,150);ta.value='should not become owner';h.document.getElementById('networkToolkitFileEditor').emit('input',ta);assert.equal(doc.content,contents);assert.equal(ta.value,contents);assert.equal(await h.api.closeAll(),false);
 adapter.rollback([uid]);assert.equal(doc.frozen,false);assert.equal(adapter.canTransfer([uid]),'');
 const dest=harness();await dest.first();await dest.api.closeAll();const destination=dest.api.windowAdapter;await destination.stage(snapshot,()=>({read:async()=>({content:contents}),write:async()=>({hash:'new'})}));assert.equal(dest.api.documents.size,1);assert(dest.api.active.frozen);destination.activate([uid]);assert.equal(dest.api.active.frozen,false);assert.equal(dest.api.active.uid,uid);assert.deepEqual(Array.from(dest.api.active.undo),undo);assert.equal(dest.api.active.view.top,150);
 adapter.snapshot([uid]);adapter.release([uid]);assert.equal(h.api.documents.size,0);assert.equal(dest.api.active.content,contents);
});
test('unsupported file access and oversized/duplicate destination batches never discard source documents',async()=>{
 const h=harness();await h.first();const d=h.api.active,a=h.api.windowAdapter;d.binding={kind:'desktopHandle'};assert.match(a.canTransfer([d.uid]),/cannot move/);assert.throws(()=>a.snapshot([d.uid]),/cannot move/);assert.equal(d.frozen,undefined);d.binding={kind:'local'};const snap=a.snapshot([d.uid]);a.rollback([d.uid]);await assert.rejects(()=>a.stage(snap,()=>({read(){},write(){}})),/already owns/);assert.equal(h.api.documents.size,1);assert.equal(d.frozen,false);
});

test('Save As reserves its destination while pending so another tab cannot be overwritten',async()=>{
 const h=harness();await h.first();const d=h.api.active,pending=deferred();d.write=()=>pending.promise;h.click('saveAs');const root=h.document.getElementById('networkToolkitFileEditor');root.querySelector('[data-ed-dialog-input]').value='C:\\Review\\reserved.txt';root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));await settle();
 assert(d.saving);await assert.rejects(()=>h.api.open({id:'other',path:'C:\\Review\\reserved.txt',source:'Local computer',create:true,initialContent:'must not replace',read:async()=>({}),write:async()=>({})}),/pending Save As/);assert.equal(h.api.documents.size,1);
 pending.resolve({hash:'written'});await settle();assert.equal(d.id,'local:C:\\Review\\reserved.txt');assert.equal(h.api.documents.get(d.id),d);assert.equal(d.saving,false);
});
test('window checkpoints deep-copy without freezing and native preparation wraps new and Save-As documents',async()=>{
 const h=harness();let prepared=[];h.c.NetworkToolkitEditorWindows={mount(){},prepareDocument(d){prepared.push(d.uid);}};await h.first();const d=h.api.active,a=h.api.windowAdapter;d.binding={kind:'local'};const checkpoint=a.checkpoint();assert.equal(d.frozen,undefined);assert.equal(checkpoint.documents[0].uid,d.uid);checkpoint.documents[0].undo.push('external');assert.equal(d.undo.length,0);assert.equal(checkpoint.preferences.wordWrap,false);
 d.saving=true;assert.throws(()=>a.checkpoint(),/saving/);const single=a.snapshotDocument(d);assert.equal(single.uid,d.uid);d.saving=false;assert(prepared.includes(d.uid));
 const pending=deferred();d.write=()=>pending.promise;h.click('saveAs');const root=h.document.getElementById('networkToolkitFileEditor');root.querySelector('[data-ed-dialog-input]').value='C:\\Review\\prepared.txt';root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));await settle();pending.resolve({});await settle();assert.equal(prepared.filter(uid=>uid===d.uid).length,2);
});

test('window snapshots preflight combined history size and strip nested transient fields',async()=>{
 const h=harness();await h.first();const d=h.api.active,a=h.api.windowAdapter;d.binding={kind:'terminal',sessionId:'synthetic-session',revision:2,secret:'must-not-transfer'};d.view={start:1,end:2,top:3,left:4,direction:'forward',extra:'must-not-transfer'};const snap=a.checkpoint();assert.equal(Object.hasOwn(snap.documents[0].binding,'secret'),false);assert.equal(Object.hasOwn(snap.documents[0].view,'extra'),false);
 d.undo=Array(65).fill('x'.repeat(4*1024*1024));assert.throws(()=>a.snapshot([d.uid]),/256 MiB/);assert.equal(d.frozen,undefined);assert.equal(d.undo.length,65);
 d.undo=Array(44).fill('\u0001'.repeat(1024*1024));assert.throws(()=>a.snapshot([d.uid]),/256 MiB/);assert.equal(d.frozen,undefined,'Escaped controls count their actual serialized size before freezing');
});

test('closing the last loading tab re-enables the empty input and ignores its late read',async()=>{
 const h=harness();h.c.OFFLINE=true;h.document.body.appendChild=h.document.body.append;const pending=deferred(),loading=h.api.open({id:'loading',source:'Local computer',path:'loading.txt',read:()=>pending.promise,write:async()=>({})}),root=h.document.getElementById('networkToolkitFileEditor'),ta=root.querySelector('[data-ed-text]');assert.equal(ta.disabled,true);
 root.emit('click',root.querySelector('[data-ed-close-tab="loading"]'));await settle();assert.equal(h.api.documents.size,0);assert.equal(ta.disabled,false);ta.value='new unsaved draft';root.emit('input',ta);await settle();const draft=h.api.active;pending.resolve({content:'old late read'});await loading;assert.equal(h.api.active,draft);assert.equal(draft.content,'new unsaved draft');assert.equal(ta.value,'new unsaved draft');
});

test('native close consent never discards before shutdown confirmation and rollback retains edits',async()=>{
 const h=harness(),ta=await h.first(),d=h.api.active,a=h.api.windowAdapter,root=h.document.getElementById('networkToolkitFileEditor');ta.value='unsaved native close';root.emit('input',ta);const contents=d.content;
 let pending=a.prepareWindowClose();root.emit('click',root.querySelector('[data-ed-dialog-action="cancel"]'));assert.equal(await pending,false);assert.equal(h.api.active,d);assert.equal(d.content,contents);assert.equal(d.frozen,undefined);
 pending=a.prepareWindowClose();root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));assert.equal(await pending,true);assert.equal(h.api.documents.size,1);assert.equal(d.frozen,true);assert.equal(d.content,contents);await assert.rejects(()=>h.api.newDocument(),/close confirmation/);
 a.cancelWindowClose();assert.equal(d.frozen,false);assert.equal(d.content,contents);assert(d.dirty);assert.equal(h.api.documents.size,1);
 pending=a.prepareWindowClose();root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));assert.equal(await pending,true);assert.equal(h.api.documents.size,1);a.commitWindowClose();assert.equal(h.api.documents.size,0);assert.equal(ta.value,'');
});
test('native close refuses changed documents or pending operations instead of freezing stale consent',async()=>{
 const h=harness(),ta=await h.first(),d=h.api.active,a=h.api.windowAdapter,root=h.document.getElementById('networkToolkitFileEditor');ta.value='first edit';root.emit('input',ta);let pending=a.prepareWindowClose();ta.value='newer edit';root.emit('input',ta);root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));assert.equal(await pending,false);assert.equal(d.content,'newer edit');assert.equal(d.frozen,undefined);
 for(const key of ['loading','reloading','saving','formatting','frozen']){d[key]=true;assert.equal(await a.prepareWindowClose(),false,key);d[key]=false;}assert.equal(h.api.documents.size,1);
});

test('subset checkpoints use a selected active UID and omit an unsupported active document',async()=>{
 const h=harness();await h.first();const eligible=h.api.active;eligible.binding={kind:'local'};const desktop=await h.api.open({id:'desktop-active',source:'Desktop',path:'desktop.txt',binding:{kind:'desktopHandle'},create:true,initialContent:'desktop remains here',read:async()=>({}),write:async()=>({})});const snapshot=h.api.windowAdapter.checkpoint([eligible.uid]);assert.equal(snapshot.active,eligible.uid);assert.equal(snapshot.documents.length,1);assert.equal(snapshot.documents[0].uid,eligible.uid);assert.equal(h.api.active,desktop);assert.equal(desktop.frozen,undefined);assert.equal(eligible.frozen,undefined);
 assert.match(h.api.windowAdapter.canTransfer([eligible.uid,desktop.uid]),/cannot move/,'Explicit Move-all still refuses unsupported bindings');
});

test('new-window appearance snapshots carry an allowlisted app theme for Follow app colors',async()=>{
 const h=harness();await h.first();h.api.active.binding={kind:'local'};h.c.state.theme='light';const adapter=h.api.windowAdapter,batch=adapter.checkpoint();assert.equal(batch.preferences.palette,'auto');assert.equal(batch.preferences.appTheme,'light');const applied=[];h.c.setTheme=value=>applied.push(value);adapter.applyPreferences(batch.preferences);assert.deepEqual(applied,['light']);adapter.applyPreferences({...batch.preferences,appTheme:'unrecognized'});assert.deepEqual(applied,['light']);
 h.c.state.theme='unrecognized';assert.equal(adapter.checkpoint().preferences.appTheme,'dark');
});

test('navigation captures view before the mounted editor is hidden and restores it after DOM scroll reset',async()=>{
 const h=harness();h.c.OFFLINE=true;h.document.body.appendChild=h.document.body.append;const ta=await h.first(),mount=h.document.createElement('div');h.document.append(mount);h.api.renderPage(mount);ta.setSelectionRange(600,609);ta.scrollTop=620;ta.scrollLeft=35;const doc=h.api.active;
 h.api.detachPage();assert.equal(doc.view.top,620);assert.equal(doc.view.left,35);ta.scrollTop=0;ta.scrollLeft=0;h.api.renderPage(mount);assert.equal(h.api.active,doc);assert.equal(ta.scrollTop,620);assert.equal(ta.scrollLeft,35);assert.equal(ta.selectionStart,600);assert.equal(ta.selectionEnd,609);
});

test('Files replaces redundant connection toolbar without recreating documents or storing content',async()=>{
 const h=harness();h.c.OFFLINE=true;h.document.body.appendChild=h.document.body.append;await h.first();const mount=h.document.createElement('div');h.document.append(mount);h.api.renderPage(mount);
 assert.equal(h.document.querySelector('[data-ed-panel-toggle="remote"]'),null);assert.equal(h.document.querySelector('[data-ed-panel-toggle="local"]'),null);assert(h.document.querySelector('[data-ed="files"]'));
 const doc=h.api.active,ta=h.document.querySelector('[data-ed-text]');ta.setSelectionRange(34,55);ta.scrollTop=123;h.click('upper');const before={text:doc.content,undo:[...doc.undo],revision:doc.editRevision};
 h.api.detachPage();h.api.renderPage(mount);assert.equal(doc.content,before.text);assert.deepEqual([...doc.undo],before.undo);assert.equal(doc.editRevision,before.revision);assert.equal(ta.selectionStart,34);assert.equal(ta.scrollTop,123);assert(!JSON.stringify(h.c.state).includes('Line 299'));
});
test('legacy disclosure preferences cannot restore a redundant toolbar or expose an orphaned form',async()=>{
 for(const value of [{localPanelOpen:true,remotePanelOpen:false},{localPanelOpen:false,remotePanelOpen:true},{localPanelOpen:'true',remotePanelOpen:1}]){
  const h=harness();h.c.OFFLINE=true;h.c.state.editorPreferences=value;await h.first();const mount=h.document.createElement('div');h.document.append(mount);h.api.renderPage(mount);
  assert.equal(h.document.querySelector('[data-ed-panel-toggle="local"]'),null);assert.equal(h.document.querySelector('[data-ed-panel-toggle="remote"]'),null);assert(h.document.querySelector('#ntEditorRemotePanel').hasAttribute('hidden'));assert.equal(h.api.active.loading,false);
 }
});
test('compact Compare action retains its full accessible name and explanatory title',async()=>{
 const h=harness();await h.first();const button=h.document.querySelector('[data-ed="compare"]');assert.equal(button.getAttribute('aria-label'),'Compare documents');assert.equal(button.getAttribute('title'),'Compare documents');assert.equal(button.querySelector('.nt-editor-label-compact').textContent,'Compare');
});
test('More stays in the toolbar while its separate action row toggles and restores keyboard focus',async()=>{
 const h=harness(),ta=await h.first(),root=h.document.getElementById('networkToolkitFileEditor'),more=root.querySelector('.nt-editor-more'),actions=root.querySelector('#ntEditorMoreActions'),summary=more.querySelector('summary');
 assert(more.closest('.nt-editor-toolbar'));assert(!actions.closest('.nt-editor-toolbar'));assert(actions.hasAttribute('hidden'));const doc=h.api.active;ta.setSelectionRange(8,16);ta.scrollTop=40;
 more.open=true;more.emit('toggle',more);assert.equal(actions.hidden,false);assert.equal(summary.getAttribute('aria-expanded'),'true');assert.equal(actions.querySelector('[data-ed="openLocalPath"]'),null);assert.equal(actions.querySelector('[data-ed="newLocalPath"]'),null);assert.equal(root.querySelector('[data-ed-panel-toggle="remote"]'),null);
 h.document.activeElement=actions.querySelector('[data-ed="trim"]');summary.focus=()=>{h.document.activeElement=summary;};more.open=false;more.emit('toggle',more);assert.equal(actions.hidden,true);assert(h.document.activeElement===summary);assert.equal(h.api.active,doc);assert.equal(ta.selectionStart,8);assert.equal(ta.scrollTop,40);
});
test('local file APIs remain available to Files while More contains no duplicate path commands',async()=>{
 const h=harness();await h.first();const requests=[];h.c.api=async(path,method,body)=>{requests.push({path,method,body});return {content:'synthetic opened text',hash:'read-hash'};};
 await h.api.openLocalPath('C:\\synthetic\\opened.txt');assert.equal(h.api.active.content,'synthetic opened text');assert.equal(requests.length,1);assert.equal(requests[0].body.action,'read-text');
 await h.api.openLocalPath('C:\\synthetic\\new.txt',true);assert.equal(h.api.active.path,'C:\\synthetic\\new.txt');assert.equal(requests.length,1);assert.equal(h.api.active.content,'');assert.equal(h.api.active.binding.kind,'local');
});
test('saved connection listing and selection never connect; explicit action is single-flight and ignores stale completion',async()=>{
 const h=harness();h.c.OFFLINE=true;await h.first();const pending=deferred(),calls=[];h.c.NetworkToolkitEditorSavedConnections={list:async()=>({profiles:[{id:'ssh:one',name:'Synthetic one',protocol:'ssh',host:'fixture.test',port:22},{id:'ssh:two',name:'Synthetic two',protocol:'ssh',host:'other.test',port:2222}],detached:false}),connect:id=>{calls.push(id);return pending.promise;}};
 await h.api.refreshConnectedSources();const picker=h.document.getElementById('ntEditorRemoteSource'),connect=h.document.getElementById('ntEditorConnectSaved');picker.value='saved:ssh:one';h.document.emit('change',picker);assert.equal(calls.length,0);assert.equal(connect.hidden,false);assert.equal(connect.textContent,'Connect');assert.equal(h.document.getElementById('ntEditorOpenRemote').disabled,true);
 h.document.emit('click',connect);h.document.emit('click',connect);assert.deepEqual(calls,['ssh:one']);assert.equal(connect.disabled,true);picker.value='saved:ssh:two';h.document.emit('change',picker);pending.resolve({pending:true,source:'client:ssh',message:'Old completion'});await settle();assert.equal(picker.value,'saved:ssh:two');assert(!h.document.getElementById('ntEditorRemoteHint').textContent.includes('Old completion'));assert.equal(connect.disabled,false);
});
test('detached saved selection labels explicit request and preserves approval guidance without a file operation',async()=>{
 const h=harness();h.c.OFFLINE=true;await h.first();let calls=0;h.c.NetworkToolkitEditorSavedConnections={list:async()=>({profiles:[{id:'ssh:one',name:'Synthetic',protocol:'ssh',host:'fixture.test',port:22}],detached:true}),connect:async()=>{calls++;return {requested:true,message:'Await main approval; nothing connected.'};}};await h.api.refreshConnectedSources();const picker=h.document.getElementById('ntEditorRemoteSource'),connect=h.document.getElementById('ntEditorConnectSaved');picker.value='saved:ssh:one';h.document.emit('change',picker);assert.equal(connect.textContent,'Request connection');assert.equal(calls,0);h.document.emit('click',connect);await settle();assert.equal(calls,1);assert.equal(h.document.getElementById('ntEditorRemoteHint').textContent,'Await main approval; nothing connected.');assert.equal(h.document.getElementById('ntEditorOpenRemote').disabled,true);
});


test('stable UID tab reordering preserves active document objects, edit history, selection and scroll',async()=>{
 const h=harness(),ta=await h.first(),first=h.api.active;await h.api.open({id:'second',path:'second.txt',source:'Local computer',create:true,initialContent:'second content',read:async()=>({content:''}),write:async()=>({})});const second=h.api.active;await h.api.open({id:'third',path:'third.txt',source:'Local computer',create:true,initialContent:'third content',read:async()=>({content:''}),write:async()=>({})});const third=h.api.active;ta.setSelectionRange(1,5);ta.scrollTop=61;ta.scrollLeft=7;h.click('upper');const saved={text:third.content,undo:[...third.undo],revision:third.editRevision,start:ta.selectionStart,end:ta.selectionEnd,top:ta.scrollTop,left:ta.scrollLeft};assert(h.api.reorderDocument(third.uid,first.uid));assert.deepEqual([...h.api.documents.values()],[third,first,second]);assert.equal(h.api.active,third);assert.deepEqual({text:third.content,undo:[...third.undo],revision:third.editRevision,start:ta.selectionStart,end:ta.selectionEnd,top:ta.scrollTop,left:ta.scrollLeft},saved);assert.equal(h.document.querySelector('[data-ed-open="third"]').getAttribute('aria-posinset'),'1');assert.equal(h.document.querySelector('[data-ed-open="third"]').id,'nt-editor-tab-'+third.uid);assert(h.api.reorderDocument(first.uid,null));assert.deepEqual([...h.api.documents.values()],[third,second,first]);assert.equal(h.api.active,third);
});
test('keyboard reorder is distinct from tab switching and pending/stale requests leave order intact',async()=>{
 const h=harness();await h.first();const first=h.api.active;await h.api.open({id:'second',path:'second.txt',source:'Local computer',create:true,initialContent:'second',read:async()=>({content:''}),write:async()=>({})});const second=h.api.active,root=h.document.getElementById('networkToolkitFileEditor');root.emit('keydown',root.querySelector('[data-ed-open="second"]'),{key:'ArrowLeft',altKey:true,shiftKey:true});assert.deepEqual([...h.api.documents.values()],[second,first]);assert.equal(h.api.active,second);root.emit('keydown',root.querySelector('[data-ed-open="second"]'),{key:'ArrowRight',altKey:true,shiftKey:true});assert.deepEqual([...h.api.documents.values()],[first,second]);for(const field of ['saving','loading','reloading','formatting','frozen']){first[field]=true;assert.equal(h.api.reorderDocument(second.uid,first.uid),false,field);assert.deepEqual([...h.api.documents.values()],[first,second]);first[field]=false;}assert.equal(h.api.reorderDocument('stale',first.uid),false);assert.equal(h.api.reorderDocument(second.uid,'stale'),false);assert.deepEqual([...h.api.documents.values()],[first,second]);h.c.NetworkToolkitEditorWindows={state:{outgoing:1}};assert.equal(h.api.reorderDocument(second.uid,first.uid),false);assert.equal(h.api.active,second);
});

test('native drag preparation may be inspected without weakening actual reorder or other guards',async()=>{const h=harness();await h.first();const d=h.api.active;h.c.NetworkToolkitEditorWindows={state:{dragPreparing:true}};assert.match(h.api.canReorderDocuments(),/pending/);assert.equal(h.api.canReorderDocuments({ignoreDragPreparation:true}),'');assert.equal(h.api.reorderDocument(d.uid,null),false);for(const field of ['closing','outgoing','incoming','detaching']){h.c.NetworkToolkitEditorWindows.state[field]=true;assert.match(h.api.canReorderDocuments({ignoreDragPreparation:true}),/pending/,field);h.c.NetworkToolkitEditorWindows.state[field]=false;}for(const field of ['saving','loading','reloading','formatting','frozen']){d[field]=true;assert.match(h.api.canReorderDocuments({ignoreDragPreparation:true}),/pending/,field);d[field]=false;}h.c.NetworkToolkitEditorWindows.state.dragPreparing=false;assert.equal(h.api.reorderDocument(d.uid,null),true);});


test('an empty split cannot reuse or edit the current document when creation reaches the tab limit',async()=>{
 const h=harness(),ta=await h.first();while(h.api.documents.size<12)await h.api.newDocument();
 let options;h.c.NTKEditorCompare={create:value=>{options=value;return {textarea:ta,refresh(){},showDocument(){},snapshot(){return {};},dispose(){}};}};
 h.api.openSplit();const current=h.api.active,before=[...h.api.documents.values()].map(d=>({uid:d.uid,content:d.content,dirty:d.dirty,undo:[...d.undo]})),groups=JSON.stringify(h.api.viewGroups);
 assert.equal(await options.createInView('right'),null);assert.equal(h.api.active,current);assert.equal(h.api.documents.size,12);assert.equal(JSON.stringify(h.api.viewGroups),groups);
 assert.deepEqual([...h.api.documents.values()].map(d=>({uid:d.uid,content:d.content,dirty:d.dirty,undo:[...d.undo]})),before);
});

function compareHarness(h){const instances=[];h.c.innerWidth=1440;h.c.innerHeight=1000;h.c.NTKEditorCompare={create:options=>{options.container.innerHTML='<div data-ed-tabs data-ed-view="left"></div><div data-ed-tabs data-ed-view="right"></div>';const instance={options,textarea:h.document.querySelector('[data-ed-text]'),refresh(){this.selection=options.selection();},snapshot(){return {...options.selection(),scale:0,sync:true};},restore(value){options.choose('left',value.left);options.choose('right',value.right);this.refresh();},showDocument(){this.refresh();},dispose(){}};instances.push(instance);instance.refresh();return instance;}};return instances;}
function contextTab(h,d){const root=h.document.getElementById('networkToolkitFileEditor');root.emit('contextmenu',root.querySelector('[data-ed-uid="'+d.uid+'"]'),{clientX:120,clientY:160});return root;}
test('Compare to captures clicked UID as left, excludes self, disambiguates names and restores split membership',async()=>{
 const h=harness(),ta=await h.first(),left=h.api.active,instances=compareHarness(h);left.path='/left/same.txt';left.dirty=true;const other=await h.api.open({id:'other',path:'/right/same.txt',source:'Synthetic remote',create:true,initialContent:'other unsaved',read:async()=>({}),write:async()=>({})});other.dirty=true;other.undo=['previous'];
 const third=await h.api.newDocument();h.api.openSplit();h.api.moveDocumentToView(other.uid,'right');const groups=JSON.stringify(h.api.viewGroups.groups),state=[...h.api.documents.values()].map(d=>({uid:d.uid,text:d.content,dirty:d.dirty,undo:[...d.undo],redo:[...d.redo]}));
 const root=contextTab(h,other);root.emit('click',root.querySelector('[data-ed-context-action="compareTo"]'));const menu=root.querySelector('.nt-editor-compare-menu');assert(menu);assert.equal(menu.querySelector('[data-ed-compare-target="'+other.uid+'"]'),null);assert(menu.textContent.includes('/left/same.txt'));assert(menu.textContent.includes('Untitled'));assert(menu.textContent.includes('Unsaved'));assert(menu.textContent.includes('Tab 1'));
 root.emit('click',menu.querySelector('[data-ed-compare-target="'+left.uid+'"]'));assert.equal(h.api.comparison.selection.left,other.id);assert.equal(h.api.comparison.selection.right,left.id);assert.equal(h.api.active,other);assert.equal(h.api.documents.size,3);assert.deepEqual([...h.api.documents.values()].map(d=>({uid:d.uid,text:d.content,dirty:d.dirty,undo:[...d.undo],redo:[...d.redo]})),state);
 h.api.closeComparison();assert.equal(JSON.stringify(h.api.viewGroups.groups),groups);assert.equal(h.api.viewGroups.split,true);assert.equal(h.api.documents.get(third.id),third);assert(instances.length>=3);
});
test('Compare to is disabled without another document and cancellation or stale/pending selection does not change state',async()=>{
 const h=harness();await h.first();const first=h.api.active,instances=compareHarness(h);let root=contextTab(h,first);assert(root.querySelector('[data-ed-context-action="compareTo"]').hasAttribute('disabled'));root.emit('keydown',root.querySelector('.nt-editor-tab-menu'),{key:'Escape'});assert.equal(root.querySelector('.nt-editor-tab-menu'),null);
 const other=await h.api.newDocument(),groups=JSON.stringify(h.api.viewGroups.groups);root=contextTab(h,first);root.emit('click',root.querySelector('[data-ed-context-action="compareTo"]'));root.emit('click',root.querySelector('[data-ed-compare-cancel]'));assert.equal(root.querySelector('.nt-editor-tab-menu'),null);assert.equal(instances.length,0);assert.equal(JSON.stringify(h.api.viewGroups.groups),groups);
 root=contextTab(h,first);root.emit('click',root.querySelector('[data-ed-context-action="compareTo"]'));first.saving=true;root.emit('click',root.querySelector('[data-ed-compare-target="'+other.uid+'"]'));assert.equal(instances.length,0);first.saving=false;
 root=contextTab(h,first);root.emit('click',root.querySelector('[data-ed-context-action="compareTo"]'));const target=root.querySelector('[data-ed-compare-target="'+other.uid+'"]');h.api.documents.delete(other.id);root.emit('click',target);assert.equal(instances.length,0);assert.equal(h.api.active,first);assert.equal(h.api.documents.size,1);
});

test('unfinished comparison input blocks rearrangement, transfer and close without discarding source state',async()=>{
 const h=harness(),ta=await h.first(),doc=h.api.active,instances=compareHarness(h);doc.binding={kind:'local'};h.api.openComparison();const view=instances.at(-1),adapter=h.api.windowAdapter,original=doc.content;
 assert.equal(typeof view.options.selectionChanged,'function');ta.setSelectionRange(8,8);view.options.selectionChanged();assert.match(h.document.querySelector('[data-ed-position]').textContent,/Col 9/);
 view.composing=true;assert.match(h.api.canReorderDocuments(),/pending/);assert.match(adapter.canTransfer([doc.uid]),/composition/);assert.equal(await adapter.prepareWindowClose(),false);assert.equal(doc.frozen,undefined);assert.equal(doc.content,original);
 view.composing=false;assert.equal(h.api.canReorderDocuments(),'');assert.equal(adapter.canTransfer([doc.uid]),'');doc.dirty=true;
 const pending=adapter.prepareWindowClose();view.composing=true;const root=h.document.getElementById('networkToolkitFileEditor');root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));assert.equal(await pending,false);assert.equal(doc.frozen,undefined);assert.equal(doc.content,original);assert.equal(adapter.hasPendingWindowClose(),false);
 view.composing=false;assert.equal(adapter.canTransfer([doc.uid]),'');
});

test('ordinary tab, editor and comparison disposal refuse unfinished source composition',async()=>{
 const h=harness();await h.first();const doc=h.api.active,instances=compareHarness(h);h.api.openComparison();const view=instances.at(-1),root=h.document.getElementById('networkToolkitFileEditor'),count=h.api.documents.size,original=doc.content;
 const other=await h.api.newDocument();h.api.activateDocument(doc.id);view.composing=true;assert.equal(h.api.activateDocument(other.id),false);assert.equal(h.api.active,doc);h.api.documents.delete(other.id);view.composing=true;root.emit('click',root.querySelector('[data-ed-close-tab="'+doc.id+'"]'));await settle();assert.equal(h.api.documents.size,count);assert.equal(await h.api.closeAll(),false);assert.equal(h.api.closeComparison(),false);assert.equal(h.api.openSplit(),false);assert.equal(h.api.comparison,view);assert.equal(doc.content,original);
 view.composing=false;doc.dirty=true;root.emit('click',root.querySelector('[data-ed-close-tab="'+doc.id+'"]'));view.composing=true;root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));await settle();assert.equal(h.api.documents.get(doc.id),doc);
 view.composing=false;const closing=h.api.closeAll();view.composing=true;root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));assert.equal(await closing,false);assert.equal(h.api.documents.get(doc.id),doc);assert.equal(h.api.comparison,view);assert.equal(doc.content,original);
});

test('close preference defaults on, requires explicit warning to disable and preserves explicit false',async()=>{
 const h=harness();await h.first();const root=h.document.getElementById('networkToolkitFileEditor'),box=root.querySelector('[data-ed-ask-save-close]');assert.equal(box.checked,true);
 box.checked=false;root.emit('change',box);assert.match(root.querySelector('.nt-editor-dialog').textContent,/discard|Discard/);root.emit('click',root.querySelector('[data-ed-dialog-action="cancel"]'));await settle();assert.equal(box.checked,true);assert.notEqual(h.c.state.editorPreferences?.askSaveBeforeClose,false);
 box.checked=false;root.emit('change',box);root.emit('click',root.querySelector('[data-ed-dialog-action="accept"]'));await settle();assert.equal(h.c.state.editorPreferences.askSaveBeforeClose,false);assert.equal(box.checked,false);
 const wrap=root.querySelector('[data-ed-wrap]');wrap.checked=false;root.emit('change',wrap);assert.equal(h.c.state.editorPreferences.askSaveBeforeClose,false);
});
test('disabled close prompt skips only dirty confirmation and keeps save failure and active operation guards',async()=>{
 const h=harness();await h.first();h.c.state.editorPreferences={askSaveBeforeClose:false};const root=h.document.getElementById('networkToolkitFileEditor'),d=h.api.active;d.content='unsaved';d.dirty=true;
 for(const flag of ['saving','frozen']){d[flag]=true;assert.equal(await h.api.closeAll(),false,flag);assert.equal(await h.api.windowAdapter.prepareWindowClose(),false,flag);assert.equal(h.api.documents.size,1);d[flag]=false;}for(const flag of ['loading','formatting','reloading']){d[flag]=true;assert.equal(await h.api.windowAdapter.prepareWindowClose(),false,flag);d[flag]=false;}
 d.write=async()=>{throw Error('Synthetic save failure')};h.click('save');await settle();assert.equal(d.saveFailed,true);assert.match(root.querySelector('[data-ed-message]').textContent,/Save failed/);
 const pending=h.api.closeAll();assert.match(root.querySelector('.nt-editor-dialog').textContent,/previous save failed/);root.emit('click',root.querySelector('[data-ed-dialog-action="cancel"]'));assert.equal(await pending,false);assert.equal(h.api.documents.size,1);
 d.write=async()=>({hash:'good'});h.click('save');await settle();assert.equal(d.saveFailed,false);d.dirty=true;assert.equal(await h.api.closeAll(),true);assert.equal(h.api.documents.size,0);
});
test('disabled native prepare stays nondestructive until commit and child policy is readonly, unknown safe',async()=>{
 const h=harness();await h.first();h.c.state.editorPreferences={askSaveBeforeClose:false};const d=h.api.active,a=h.api.windowAdapter;d.dirty=true;assert.equal(await a.prepareWindowClose(),true);assert.equal(d.frozen,true);assert.equal(h.api.documents.size,1);a.cancelWindowClose();assert.equal(d.frozen,false);assert.equal(h.api.documents.size,1);
 h.c.EDITOR_VIEW={id:'child'};a.setClosePolicy(false);const root=h.document.getElementById('networkToolkitFileEditor'),box=root.querySelector('[data-ed-ask-save-close]');assert.equal(box.checked,false);assert.equal(box.disabled,true);assert.equal(await a.prepareWindowClose(),true);a.cancelWindowClose();a.setClosePolicy(undefined);assert.equal(box.checked,true);const pending=a.prepareWindowClose();root.emit('click',root.querySelector('[data-ed-dialog-action="cancel"]'));assert.equal(await pending,false);
});
test('activity metadata includes saved empty files and touched drafts, never untouched blank or text',async()=>{
 const h=harness();await h.api.newDocument();const d=h.api.active,a=h.api.windowAdapter;assert.deepEqual(JSON.parse(JSON.stringify(a.activityDocuments())),[{uid:d.uid,loaded:false}]);d.content='typed';d.dirty=true;d.editRevision++;assert.equal(a.activityDocuments()[0].loaded,true);d.content='';d.dirty=false;d.editRevision++;assert.equal(a.activityDocuments()[0].loaded,true);d.staged=true;assert.equal(a.activityDocuments().length,0);d.staged=false;h.c.state.editorPreferences={askSaveBeforeClose:false};assert(await h.api.closeAll());assert.equal(a.activityDocuments().length,0);
 await h.api.open({id:'saved-empty',path:'empty.txt',source:'Local computer',read:async()=>({content:''}),write:async()=>({})});const row=a.activityDocuments()[0];assert.equal(row.loaded,true);assert.deepEqual(Object.keys(row).sort(),['loaded','uid']);
});

test('capacity snapshots clone mutable containers without duplicating immutable source text and revision tokens observe views',async()=>{
 const h=harness();await h.first();const d=h.api.active,a=h.api.windowAdapter;d.binding={kind:'local'};d.undo=['before'];d.view={start:1,end:2,direction:'none',top:5,left:0};const one=a.checkpointVersion([d.uid]);assert.equal(a.checkpointVersion([d.uid]),one);d.saveFailed=true;assert.notEqual(a.checkpointVersion([d.uid]),one);const snap=a.checkpoint([d.uid]);snap.documents[0].undo[0]='changed';snap.documents[0].binding.kind='bad';snap.documents[0].view.top=123;assert.equal(d.undo[0],'before');assert.equal(d.binding.kind,'local');assert.notEqual(d.view.top,123);assert.equal(snap.documents[0].content,d.content);
});

test('large-file mode preserves wrap preference, bounds future history and never prunes on a move',async()=>{
 const h=harness();h.c.getComputedStyle=()=>({lineHeight:'20px',paddingTop:'10px'});await h.first();const root=h.document.getElementById('networkToolkitFileEditor');
 const wrap=root.querySelector('[data-ed-wrap]');wrap.checked=true;root.emit('change',wrap);
 const large='x'.repeat(2*1024*1024+1);await h.api.open({id:'large',path:'large.txt',source:'Local computer',binding:{kind:'local'},create:true,initialContent:large,read:async()=>({content:large}),write:async()=>({})});
 const ta=root.querySelector('[data-ed-text]');assert.equal(ta.wrap,'off');assert.equal(wrap.checked,true);assert.equal(wrap.disabled,true);assert.match(root.querySelector('[data-ed-size]').textContent,/Large-file mode/);
 h.api.active.undo=Array(40).fill(large);ta.value=large+'x';root.emit('input',ta);assert(h.api.active.undo.reduce((sum,value)=>sum+value.length*2,0)<=128*1024*1024);
 const count=h.api.active.undo.length,snapshot=h.api.windowAdapter.snapshot([h.api.active.uid]);assert.equal(snapshot.documents[0].undo.length,count);h.api.windowAdapter.rollback([h.api.active.uid]);assert.equal(h.api.active.undo.length,count);
 await h.api.open({id:'small-again',path:'small.txt',source:'Local computer',create:true,initialContent:'small',read:async()=>({content:'small'}),write:async()=>({})});assert.equal(ta.wrap,'soft');assert.equal(wrap.checked,true);assert.equal(wrap.disabled,false);
});


test('desktop encoded file bounds include BOM and UTF16 before opening a writable stream',async()=>{
 const h=harness();h.c.crypto=require('node:crypto').webcrypto;await h.first();let writableCalls=0;const bytes=new TextEncoder().encode('original synthetic file'),file={name:'synthetic.txt',size:bytes.length,arrayBuffer:async()=>bytes.buffer},handle={getFile:async()=>file,createWritable:async()=>{writableCalls++;throw Error('oversize must not open stream');}};
 await h.api.importFiles([file],[handle]);const d=h.api.active;assert.equal(d.binding.kind,'desktopHandle');
 const limit=16*1024*1024;
 for(const data of [{encoding:'UTF-8',content:'x'.repeat(limit+1),bom:false},{encoding:'UTF-8',content:'x'.repeat(limit-2),bom:true},{encoding:'UTF-16LE',content:'x'.repeat(limit/2+1),bom:false},{encoding:'UTF-16BE',content:'x'.repeat(limit/2),bom:true}]){
  await assert.rejects(d.write({...data,expectedHash:d.hash}),/16 MiB.*unchanged/);assert.equal(writableCalls,0);assert.equal(d.content,'original synthetic file');
 }
});
test('desktop codec accepts exact encoded boundary including BOM and preserves supplementary Unicode',()=>{
 const start=source.indexOf('function encodeDesktopText(data)'),end=source.indexOf('async function importFiles',start),context={TextEncoder,Uint8Array,DataView,EDITOR_FILE_BYTES:16*1024*1024};vm.createContext(context);vm.runInContext(source.slice(start,end)+';globalThis.encode=encodeDesktopText;',context);const limit=context.EDITOR_FILE_BYTES;
 for(const data of [{encoding:'UTF-8',content:'x'.repeat(limit),bom:false},{encoding:'UTF-8',content:'x'.repeat(limit-3),bom:true},{encoding:'UTF-16LE',content:'x'.repeat(limit/2),bom:false},{encoding:'UTF-16BE',content:'x'.repeat(limit/2-1),bom:true}])assert.equal(context.encode(data).length,limit);
 for(const encoding of ['UTF-8','UTF-16LE','UTF-16BE'])for(const bom of [false,true]){const content='Synthetic 日本語 😀\r\n',encoded=context.encode({content,encoding,bom}),decoded=new TextDecoder(encoding.toLowerCase()).decode(encoded);assert.equal(decoded,content);}
});

test('oversized Save copy surfaces the encoded limit without creating a download or changing the draft',async()=>{
 const h=harness();await h.first();const d=h.api.active;d.content='x'.repeat(16*1024*1024+1);d.dirty=true;h.c.URL={createObjectURL(){throw Error('oversize must not create download');}};
 h.click('download');assert.equal(d.content.length,16*1024*1024+1);assert.equal(d.dirty,true);assert.match(h.document.querySelector('[data-ed-message]').textContent,/16 MiB/);
});
