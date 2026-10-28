'use strict';
const assert=require('node:assert/strict');
const {performance}=require('node:perf_hooks');
// Read-only barrier for the accelerated browser fixture. Request arrival alone
// is not completion: production awaits status, then POST, then arms its timer.
function completedAutomaticCheck(state,expected){
  assert.ok(state.checks<=expected,'Unexpected extra release check: '+JSON.stringify(state));
  assert.ok(state.maxActiveChecks<=1,'Overlapping release checks: '+JSON.stringify(state));
  return state.checks===expected&&state.pendingUpdateRequests===0&&
    state.checkDisabled===false&&/Last automatic attempt:/.test(state.schedule)&&
    !/Checking…/.test(state.schedule)&&/Next automatic check:/.test(state.schedule)&&
    !/Next automatic retry:|Next readiness check:/.test(state.schedule);
}
async function waitForAutomaticCompletion(read,expected,{timeoutMs=10000,now=()=>performance.now(),sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms))}={}){
  const deadline=now()+timeoutMs;let state;
  do{
    state=await read();if(completedAutomaticCheck(state,expected))return state;
    assert.ok(now()<deadline,'Automatic update completion timed out: '+JSON.stringify({expected,state}));
    await sleep(25);
  }while(true);
}
module.exports={completedAutomaticCheck,waitForAutomaticCompletion};
