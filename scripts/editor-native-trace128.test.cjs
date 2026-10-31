'use strict';
const test=require('node:test'),assert=require('node:assert/strict');
const {create}=require('./editor-native-trace128.js');
test('passive native trace bounds metadata and excludes text, paths, identities and payloads',()=>{
 const listeners={};let now=0,reads=0;
 const el={closest:s=>s==='[data-ed-open]'?el:null,textContent:'PRIVATE-TEXT',dataset:{edUid:'PRIVATE-UID'}};
 const doc={addEventListener:(n,fn)=>listeners[n]=fn,elementFromPoint:()=>el,hasFocus:()=>true};
 const snapshot=create(doc,()=>++now,()=>({running:true,error:'PRIVATE-PATH',owned:{'PRIVATE-UID':42}}),()=>({armed:true,mode:'native',uid:'PRIVATE-UID'}));
 for(let i=0;i<300;i++)listeners.dragover({type:'dragover',isTrusted:true,altKey:true,key:'PRIVATE-TEXT',target:el,clientX:12,clientY:34,dataTransfer:{types:['application/x-network-toolkit-editor-tab','PRIVATE-TYPE'],dropEffect:'move',effectAllowed:'move',getData(){reads++;return 'PRIVATE-TICKET';}}});
 const result=snapshot();assert.equal(result.total,300);assert.equal(result.events.length,256);assert.equal(result.events[0].at,45);assert.equal(reads,0);assert.doesNotMatch(JSON.stringify(result),/PRIVATE/);assert.equal(result.events[0].target,'tab-open');assert.equal(result.events[0].alt,true);assert.equal(result.events[0].trusted,true);assert.equal(result.events[0].effect,'move');assert.equal(result.events[0].hasError,true);
 result.events[0].target='changed';assert.equal(snapshot().events[0].target,'tab-open');
});
test('untrusted and outside events remain distinguishable; keypress contents omitted',()=>{
 const listeners={};const snapshot=create({addEventListener:(n,fn)=>listeners[n]=fn},()=>1,()=>null,()=>null);
 listeners.keydown({type:'keydown',key:'Alt',isTrusted:false,target:null});listeners.keyup({type:'keyup',key:'sensitive'});
 assert.equal(snapshot().events[0].key,'Alt');assert.equal(snapshot().events[0].trusted,false);assert.equal(snapshot().events[0].target,'outside');assert.equal(snapshot().events[1].key,'');
});
