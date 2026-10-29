'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
test('prepared copy only arms real movement; release tombstones exact delayed gesture',async()=>{
 const listeners={},globalListeners={},calls=[],handle={style:{},isConnected:true,addEventListener(name,fn){listeners[name]=fn;}};
 let releaseArm;const c={console,Map,setTimeout(){return 1},clearTimeout(){},document:{addEventListener(){},querySelector(selector){return selector.includes('handle')?handle:{textContent:''}}},__networkToolkitNativeSession:'fixture',openModal(){},closeModal(){},api:async(path,method,body)=>{calls.push({path,method,body});if(body?.action==='arm')await new Promise(resolve=>{releaseArm=resolve});return {id:'fixture',state:'ready',message:'Ready',entries:1,bytes:1};},addEventListener(name,fn){globalListeners[name]=fn},removeEventListener(){}};c.globalThis=c;c.window=c;vm.createContext(c);vm.runInContext(fs.readFileSync('web/assets/remote-drag.js','utf8'),c);
 await c.NetworkToolkitNativeDrag.prepareCopy({binding:{kind:'local'},paths:['synthetic']});await Promise.resolve();
 for(let i=0;i<20;i++){listeners.pointerdown({button:0,pointerId:1,clientX:0,clientY:0});globalListeners.pointerup();}
 assert.equal(calls.filter(x=>['arm','disarm'].includes(x.body?.action)).length,0);
 listeners.pointerdown({button:0,pointerId:1,clientX:0,clientY:0});listeners.pointermove({pointerId:1,buttons:1,clientX:10,clientY:0});globalListeners.pointerup();
 const arm=calls.find(x=>x.body?.action==='arm'),end=calls.find(x=>x.body?.action==='disarm');assert.ok(arm&&end);assert.equal(arm.body.gesture,end.body.gesture);releaseArm();await Promise.resolve();
 assert.equal(calls.filter(x=>x.body?.action==='arm').length,1);
});
function nativeFixture(api){
 const events={},windowEvents={};let handle,status={textContent:''},closed=0;
 const c={console,Map,setTimeout(){return 1},clearTimeout(){},document:{addEventListener(name,fn){events[name]=fn},querySelector(selector){return selector.includes('handle')?handle:status}},__networkToolkitNativeSession:'fixture',openModal(){handle={style:{},isConnected:true,addEventListener(name,fn){this[name]=fn}};status={textContent:'new-modal'}},closeModal(){closed++;handle.isConnected=false},api,addEventListener(name,fn){windowEvents[name]=fn},removeEventListener(){},toast(){}};c.globalThis=c;c.window=c;vm.createContext(c);vm.runInContext(fs.readFileSync('web/assets/remote-drag.js','utf8'),c);
 return {c,events,windowEvents,get handle(){return handle},get status(){return status},get closed(){return closed},close(){events.click({target:{closest(selector){return selector.includes('data-native-copy-cancel')?{}:null}},preventDefault(){},stopImmediatePropagation(){}})}};
}
const turn=()=>new Promise(resolve=>setImmediate(resolve));
test('rejected or uncertain cancellation retains modal; active OLE Escape sends no page cancel',async()=>{
 const calls=[];const f=nativeFixture(async(path,method,body)=>{calls.push(body?.action||'status');if(body?.action==='cancel')throw Error('Cancellation not confirmed');return{id:'one',state:'ready',message:'Ready',entries:1,bytes:1}});
 await f.c.NetworkToolkitNativeDrag.prepareCopy({});await turn();f.close();await turn();assert.equal(f.closed,0);
 f.handle.pointerdown({button:0,pointerId:1,clientX:0,clientY:0});f.handle.pointermove({pointerId:1,buttons:1,clientX:10,clientY:0});let stopped=false;const before=calls.filter(x=>x==='cancel').length;
 f.events.keydown({key:'Escape',stopImmediatePropagation(){stopped=true}});await turn();assert.equal(stopped,true);assert.equal(calls.filter(x=>x==='cancel').length,before);assert.equal(f.closed,0);
});
test('late closed-job status cannot overwrite a new copy modal',async()=>{
 let firstResolve,gets=0,prepares=0;const f=nativeFixture(async(path,method,body)=>{if(body?.action==='prepare')return{id:String(++prepares),state:'preparing'};if(body?.action==='cancel')return{state:'cancelled'};if(++gets===1)return await new Promise(resolve=>{firstResolve=resolve});return{state:'ready',message:'NEW_READY',entries:1,bytes:1}});
 await f.c.NetworkToolkitNativeDrag.prepareCopy({});f.close();await turn();assert.equal(f.closed,1);await f.c.NetworkToolkitNativeDrag.prepareCopy({});await turn();const before=f.status.textContent;firstResolve({state:'finished',message:'OLD_FINISHED',entries:9,bytes:9});await turn();assert.equal(f.status.textContent,before);assert.equal(f.status.textContent.includes('OLD_FINISHED'),false);
});
