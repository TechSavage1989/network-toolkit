'use strict';
const test=require('node:test'),assert=require('node:assert/strict');
const {guidance,recovery,guides}=require('../web/assets/ux-phase5.js');
test('all navigation tools provide a useful first step with an example or input',()=>{
 const tools=['quick','ping','trace','dns','tcp','smtp','http','tls','identify','discover','monitor','capture','wol','sftp','ftp','tftp','local','hosts','subnet','editor','terminal','profiles','reports','overview'];
 for(const tool of tools){const result=guidance(tool);assert(result.title.length>4,tool);assert(result.description.length>35,tool);assert(/enter|choose|start|select|refresh|create|open|run|add|use/i.test(result.description),tool);}
 assert.equal(Object.keys(guides).length>=24,true);
});
test('offline guidance never implies that a live scan is available',()=>{
 assert.match(guidance('ping',false).description,/Open Network_Toolkit.exe/);
 assert.doesNotMatch(guidance('ping',false).description,/Start ping/);
});
test('retry instructions retain security checks for remote connections',()=>{
 for(const tool of ['sftp','ftp','tftp'])assert.match(recovery(tool),/Do not bypass/);
 assert.match(recovery('tls'),/certificate verification enabled/);
 assert.match(recovery('hosts'),/Windows permissions/);
});
