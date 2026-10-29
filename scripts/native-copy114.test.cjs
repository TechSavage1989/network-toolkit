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
