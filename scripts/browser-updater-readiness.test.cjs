'use strict';
const test=require('node:test'),assert=require('node:assert/strict');
const {completedAutomaticCheck,waitForAutomaticCompletion}=require('./browser-updater-readiness.cjs');
const ready={checks:2,maxActiveChecks:1,pendingUpdateRequests:0,checkDisabled:false,schedule:'Last automatic attempt: synthetic. Update available. Next automatic check: synthetic.'};
test('POST arrival does not mean completed controller and rearmed scheduler',()=>{
  assert.equal(completedAutomaticCheck({...ready,pendingUpdateRequests:1},2),false);
  assert.equal(completedAutomaticCheck({...ready,checkDisabled:true},2),false);
  assert.equal(completedAutomaticCheck({...ready,schedule:'Last automatic attempt: synthetic. Checking…'},2),false);
  assert.equal(completedAutomaticCheck({...ready,schedule:'No automatic update check has run in this window yet. Next automatic check: synthetic.'},2),false);
  assert.equal(completedAutomaticCheck(ready,2),true);
});
test('delayed status before POST waits for exact completion without advancing virtual time',async()=>{
  let now=0,reads=0;const states=[{...ready,checks:1,pendingUpdateRequests:1,checkDisabled:true}, {...ready,pendingUpdateRequests:1}, {...ready,schedule:'Last automatic attempt: synthetic. Checking…'},ready];
  assert.deepEqual(await waitForAutomaticCompletion(async()=>states[Math.min(reads++,3)],2,{now:()=>now,sleep:async ms=>{now+=ms;}}),ready);
  assert.equal(reads,4);assert.equal(now,75);
});
test('extra and overlapping requests fail rather than polling until the failure disappears',async()=>{
  await assert.rejects(waitForAutomaticCompletion(async()=>({...ready,checks:3}),2),/Unexpected extra release check/);
  await assert.rejects(waitForAutomaticCompletion(async()=>({...ready,maxActiveChecks:2}),2),/Overlapping release checks/);
});
test('missing check or unexpected retry remains a bounded failure',async()=>{
  for(const state of [{...ready,checks:1},{...ready,schedule:'Last automatic attempt: synthetic. Failed. Next automatic retry: synthetic.'}]){
    let now=0;await assert.rejects(waitForAutomaticCompletion(async()=>state,2,{timeoutMs:50,now:()=>now,sleep:async ms=>{now+=ms;}}),/completion timed out/);assert.equal(now,50);
  }
});
