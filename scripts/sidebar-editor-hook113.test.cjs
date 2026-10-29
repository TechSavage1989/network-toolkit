'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const source=html.match(/NetworkToolkitSidebarActivity\.init\(\{request:(signal=>\{[\s\S]*?\}),available:/)?.[1];
test('sidebar metadata report never blocks the existing counts-only activity request',()=>{
 assert(source);const signal={},result={},calls=[];
 const request=vm.runInNewContext('('+source+')',{window:{NetworkToolkitEditorWindows:{reportActivity(){calls.push('metadata');return new Promise(()=>{});}}},api(...args){calls.push(args);return result;}});
 assert.equal(request(signal),result);assert.equal(calls[0],'metadata');assert.deepEqual(calls[1],['/activity','GET',undefined,false,signal]);
});
test('absent or failed editor metadata cannot suppress terminal and server indicators',()=>{
 for(const editor of [undefined,{reportActivity(){throw new Error('synthetic metadata failure');}}]){let reads=0;const request=vm.runInNewContext('('+source+')',{window:{NetworkToolkitEditorWindows:editor},api(path,method){assert.equal(path,'/activity');assert.equal(method,'GET');reads++;return 'counts';}});assert.equal(request({}),'counts');assert.equal(reads,1);}
});
test('main metadata availability requires the live engine, not native window support',()=>{
 const gate=html.match(/activityAvailable:(\(\)=>online&&!OFFLINE&&!shutDown),self:/)?.[1];assert(gate);
 for(const [online,OFFLINE,shutDown,expected]of [[true,false,false,true],[false,false,false,false],[true,true,false,false],[true,false,true,false]]){
  for(const editorWindowAvailable of [true,false])assert.equal(vm.runInNewContext('('+gate+')()',{online,OFFLINE,shutDown,status:{editorWindowAvailable}}),expected);
 }
});
