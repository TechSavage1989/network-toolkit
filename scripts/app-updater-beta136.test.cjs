'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
const {createController,html,notice}=require('../web/assets/app-updater.js');

test('Beta136 defaults to stable and requires explicit beta check and download requests',async()=>{
 const calls=[];let current='stable';
 const c=createController({channel:()=>current,api:async(path,method,body)=>{
  calls.push({path,body});if(path==='/updates/check')return {channel:current,checked:true,hasUpdate:true,latest:{version:current==='beta'?'1.17.137':'1.17.136',beta:current==='beta'},supported:true};
  if(path==='/updates/download')return {channel:current,staged:{version:'1.17.137',beta:true},supported:true};
  throw Error('unexpected '+path);
 },render(){},changed(){}});
 assert.equal(await c.check(),'ok');
 assert.deepEqual(calls.at(-1),{path:'/updates/check',body:{}});
 current='beta';
 assert.equal(c.snapshot().data.latest,null,'previous channel feed was not hidden');
 assert.equal(await c.check(),'ok');
 assert.deepEqual(calls.at(-1),{path:'/updates/check',body:{channel:'beta'}});
 await c.download('1.17.137');
 assert.deepEqual(calls.at(-1),{path:'/updates/download',body:{version:'1.17.137',channel:'beta'}});
 current='stable';
 assert.equal(c.snapshot().data.latest,null);
 await c.install('1.17.137');
 assert.match(c.snapshot().error,/Enable beta updates/);
 assert.equal(calls.filter(x=>x.path==='/updates/install').length,0);
});
test('Beta136 settings show correct channel and do not offer beta install after opt out',()=>{
 const beta={currentVersion:'1.17.136',supported:true,channel:'beta',latest:{version:'1.17.137',notes:'beta work',beta:true},hasUpdate:true};
 assert.match(html({data:beta},true,'1.17.136',{beta:true}),/Update channel: Beta/);
 assert.match(html({data:beta},true,'1.17.136',{beta:true}),/Latest release: 1.17.137 \(Beta\)/);
 assert.match(html({data:beta},true,'1.17.136',{beta:false}),/Update channel: Stable/);
 const stage={...beta,staged:{version:'1.17.137',beta:true,sha256:'abc'}};
 assert.doesNotMatch(html({data:stage},true,'1.17.136',{beta:false}),/data-app-update="confirm"/);
 assert.match(html({data:stage},true,'1.17.136',{beta:false}),/Enable beta updates/);
 assert.equal(notice({data:beta},{beta:false}),null);
 assert.equal(notice({data:stage},{beta:false}),null);
 assert.equal(notice({data:beta},{beta:true}).kind,'download');
});
