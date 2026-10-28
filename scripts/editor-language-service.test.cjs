'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
function harness(){const workers=[],timers=new Map();let next=0;class Worker{constructor(url){this.url=url;this.messages=[];workers.push(this);}postMessage(value){this.messages.push(value);}terminate(){this.terminated=true;}reply(value){this.onmessage({data:{id:this.messages.at(-1).id,...value}});}}
 const c={Worker,setTimeout:(callback,delay)=>{timers.set(++next,{callback,delay});return next;},clearTimeout:id=>timers.delete(id)};vm.runInNewContext(fs.readFileSync(require.resolve('../web/assets/editor-language-service.js'),'utf8'),c);return {c,api:c.NTKEditorLanguageService,workers,timers};}
test('superseded syntax terminates its worker and a late load error cannot stop the replacement',async()=>{
 const h=harness(),old=h.api.highlight('old','python'),oldRejected=assert.rejects(old,/Superseded/);const current=h.api.highlight('current','powershell');await oldRejected;assert(h.workers[0].terminated);h.workers[0].onerror();assert(!h.workers[1].terminated);h.workers[1].reply({html:'current'});assert.equal((await current).html,'current');assert.equal(h.timers.size,0);h.api.dispose();
});
test('syntax and formatting use independent workers and timeout terminates only the blocked operation',async()=>{
 const h=harness(),syntax=h.api.highlight('code','java'),format=h.api.format('code','java',{indent:4});assert.equal(h.workers.length,2);const rejected=assert.rejects(syntax,/timed out/);const timer=[...h.timers.values()].find(t=>t.delay===2000);timer.callback();await rejected;assert(h.workers[0].terminated);assert(!h.workers[1].terminated);h.workers[1].reply({text:'formatted',mode:'indent'});assert.equal((await format).text,'formatted');h.api.dispose();
});
test('worker failure rejects pending work and a later request can recover',async()=>{
 const h=harness(),first=h.api.format('{}','json',{}),rejected=assert.rejects(first,/could not be loaded/);h.workers[0].onerror();await rejected;assert.equal(h.timers.size,0);const second=h.api.format('{}','json',{});h.workers[1].reply({text:'{}\n',mode:'format'});assert.equal((await second).text,'{}\n');h.api.dispose();
});
test('a blocked worker constructor rejects cleanly instead of escaping the asynchronous paint callback',async()=>{
 const h=harness();h.c.Worker=class{constructor(){throw Error('Worker blocked');}};await assert.rejects(h.api.highlight('text','c'),/Worker blocked/);assert.equal(h.timers.size,0);
});
test('fixed compare channels paint independently and only same-channel requests supersede',async()=>{
 const h=harness(),left=h.api.highlight('left1','network',{channel:'compare-left'}),right=h.api.highlight('right1','xml',{channel:'compare-right'}),main=h.api.highlight('main','text');assert.equal(h.workers.length,3);
 const rejected=assert.rejects(left,/Superseded/),newLeft=h.api.highlight('left2','network',{channel:'compare-left'});await rejected;assert(h.workers[0].terminated);assert(!h.workers[1].terminated);assert(!h.workers[2].terminated);
 h.workers[0].reply({html:'stale-left'});h.workers[1].reply({html:'right'});h.workers[3].reply({html:'left2'});h.workers[2].reply({html:'main'});assert.equal((await right).html,'right');assert.equal((await newLeft).html,'left2');assert.equal((await main).html,'main');assert.equal(h.timers.size,0);h.api.dispose();
});
test('cancel one comparison leaves other streams alive; dispose rejects all pending streams',async()=>{
 const h=harness(),left=h.api.highlight('left','text',{channel:'compare-left'}),right=h.api.highlight('right','text',{channel:'compare-right'}),main=h.api.highlight('main','text'),format=h.api.format('{}','json');const rejected=assert.rejects(left,/cancelled/);h.api.cancelHighlight('compare-left');await rejected;assert(!h.workers[1].terminated);assert(!h.workers[2].terminated);
 const rest=[assert.rejects(right,/closed/),assert.rejects(main,/closed/),assert.rejects(format,/closed/)];h.api.dispose();await Promise.all(rest);assert.equal(h.timers.size,0);assert(h.workers.every(w=>w.terminated));
});
test('unknown channels use default; worker errors affect only their fixed stream',async()=>{
 const h=harness(),defaultPaint=h.api.highlight('default','text'),rejected=assert.rejects(defaultPaint,/Superseded/),unknown=h.api.highlight('unknown','text',{channel:'untrusted filename'});await rejected;assert.equal(h.workers[1].messages[0].options.channel,'default');
 const left=h.api.highlight('left','text',{channel:'compare-left'}),failed=assert.rejects(left,/could not be loaded/);h.workers[2].onerror();await failed;assert(!h.workers[1].terminated);h.workers[1].reply({html:'okay'});assert.equal((await unknown).html,'okay');assert.equal(h.timers.size,0);h.api.dispose();
});
