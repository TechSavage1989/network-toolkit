'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
const {createController,html}=require('../web/assets/app-updater.js');
const staged={currentVersion:'1.17.58',supported:true,staged:{version:'1.17.59',sha256:'abc',size:123},busy:[],installing:false};
function harness(overrides={}){const calls=[],renders=[];let saved=true;const env={api:async(...args)=>{calls.push(args);return args[0]==='/updates/install'?{restarting:true}:structuredClone(staged);},save:async()=>saved,upload:async()=>structuredClone(staged),render:s=>renders.push(s),restarting:()=>calls.push(['restart']),...overrides};return {c:createController(env),calls,renders,setSaved:v=>saved=v};}
test('direct update UI is explicit about public binary feed and integrity limits',()=>{const text=html({data:staged},true,'1.17.58');assert.match(text,/official public binary-release feed/);assert.match(text,/Check for updates/);assert.match(text,/public main repository provides a binary-only update package/);assert.match(text,/corruption, not publisher identity/);assert.match(text,/Official binary-only Update ZIP/);assert.match(text,/Install and restart/);assert.doesNotMatch(text,/Review install and restart/);assert.match(text,/data-app-update="confirm" data-version="1.17.59"/);});
test('unavailable preview and unsupported OS cannot import or install',()=>{assert.doesNotMatch(html({data:staged},false,'1.17.58'),/id="appUpdatePackage"/);assert.doesNotMatch(html({data:{...staged,supported:false}},true,'1.17.58'),/id="appUpdatePackage"/);});
test('untrusted response labels are escaped',()=>{const text=html({data:{...staged,currentVersion:'<img>',staged:{version:'<svg>',sha256:'"<script>'},busy:['<img>']},error:'<iframe>'},true,'1');assert.doesNotMatch(text,/<(?:img|svg|script|iframe)>/);assert.match(text,/&lt;iframe&gt;/);});
test('import requires both files and enforces client size limits',async()=>{let uploads=0;const h=harness({upload:async()=>{uploads++;return staged;}});await h.c.importPackage();assert.equal(uploads,0);await h.c.importPackage({name:'x.zip',size:300*1024*1024},{size:1});assert.equal(uploads,0);await h.c.importPackage({name:'x.zip',size:100},{size:1});assert.equal(uploads,1);assert.equal(h.c.snapshot().data.staged.version,'1.17.59');});
test('repeat imports are ignored while request is pending',async()=>{let resolve,count=0;const h=harness({upload:()=>{count++;return new Promise(r=>resolve=r);}});const first=h.c.importPackage({name:'x.zip',size:1},{size:1});await h.c.importPackage({name:'x.zip',size:1},{size:1});assert.equal(count,1);resolve(staged);await first;assert.equal(h.c.snapshot().busy,false);});
test('installation requires reviewed staged version and saves settings first',async()=>{const h=harness();await h.c.load();await h.c.install('1.17.60');assert.equal(h.calls.filter(c=>c[0]==='/updates/install').length,0);h.setSaved(false);await h.c.install('1.17.59');assert.equal(h.calls.filter(c=>c[0]==='/updates/install').length,0);assert.match(h.c.snapshot().error,/Settings could not be saved/);h.setSaved(true);await h.c.install('1.17.59');assert.deepEqual(h.calls.find(c=>c[0]==='/updates/install'),['/updates/install','POST',{confirm:true,version:'1.17.59'}]);assert.equal(h.c.snapshot().data.installing,true);await h.c.install('1.17.59');assert.equal(h.calls.filter(c=>c[0]==='/updates/install').length,1);});
test('operations that started after review block installation',async()=>{let n=0;const h=harness({api:async()=>({...staged,busy:++n>1?['SSH session']:[]})});await h.c.load();await h.c.install('1.17.59');assert.match(h.c.snapshot().error,/SSH session/);assert.equal(n,2);});
test('changed staged package after review requires fresh consent',async()=>{let n=0;const h=harness({api:async()=>({...staged,staged:{...staged.staged,version:++n>1?'1.17.60':'1.17.59'}})});await h.c.load();await h.c.install('1.17.59');assert.match(h.c.snapshot().error,/staged update changed/);});
test('failed requests recover UI and do not claim successful import',async()=>{const h=harness({upload:async()=>{throw new Error('Checksum mismatch');}});await h.c.importPackage({name:'x.zip',size:100},{size:1});assert.equal(h.c.snapshot().busy,false);assert.match(h.c.snapshot().error,/Checksum mismatch/);assert.equal(h.c.snapshot().data,null);});
test('network install error is not reported as success',async()=>{const h=harness({api:async path=>{if(path==='/updates/install')throw new Error('Disconnected; refresh status before retrying');return structuredClone(staged);}});await h.c.load();await h.c.install('1.17.59');assert.equal(h.c.snapshot().data.installing,false);assert.match(h.c.snapshot().error,/Disconnected/);assert.equal(h.calls.length,0);});

test('check and download stage an update without installing it',async()=>{const calls=[];const h=harness({api:async(path,method,body)=>{calls.push([path,method,body]);return {...staged,latest:{version:'1.17.59',notes:'Changes'}};}});await h.c.check();await h.c.download('1.17.59');assert.deepEqual(calls,[['/updates/check','POST',{}],['/updates/download','POST',{version:'1.17.59'}]]);assert.match(h.c.snapshot().message,/verified and staged/);});
test('download rejects stale version and does not invoke install',async()=>{const h=harness();await h.c.load();await h.c.download('1.17.60');assert.equal(h.calls.length,1);assert.match(h.c.snapshot().error,/available release changed/);});
test('release notes and release version are escaped',()=>{const text=html({data:{...staged,latest:{version:'<img>',notes:'<script>alert(1)</script>'}}},true,'1.17.58');assert.doesNotMatch(text,/<script>/);assert.match(text,/&lt;script&gt;/);assert.match(text,/network-toolkit/);});
test('server no-update state does not offer a downgrade or reinstall',()=>{const text=html({data:{...staged,staged:null,latest:{version:'1.17.57'},hasUpdate:false}},true,'1.17.58');assert.doesNotMatch(text,/data-app-update="download"/);assert.match(text,/last successful check/);});
test('native releases action uses app endpoint button instead of blocked WebView popup',()=>{const text=html({data:staged},true,'1.17.58');assert.match(text,/data-app-update="releases"/);assert.doesNotMatch(text,/target="_blank"/);});
test('an invalid successful download response is never reported as staged',async()=>{let call=0;const h=harness({api:async()=>++call===1?{...staged,latest:{version:'1.17.59'},hasUpdate:true}:{...staged,staged:null}});await h.c.check();await h.c.download('1.17.59');assert.match(h.c.snapshot().error,/No staged package/);assert.equal(h.c.snapshot().message,'');});

const {createScheduler,notice,noticeHTML}=require('../web/assets/app-updater.js');
const available={...staged,staged:null,hasUpdate:true,latest:{version:'1.17.60',notes:'New'}};
function clockHarness(overrides={}){
 let id=0;const pending=new Map(),delays=[],calls=[];
 const env={setTimeout:(fn,ms)=>{const n=++id;pending.set(n,fn);delays.push(ms);return n;},clearTimeout:n=>pending.delete(n),enabled:()=>true,ready:()=>true,check:async()=>{calls.push('check');return 'ok';},...overrides};
 return {s:createScheduler(env),pending,delays,calls,async tick(){const [id,fn]=pending.entries().next().value;pending.delete(id);await fn();}};
}
test('automatic schedule starts once immediately, then checks every five minutes',async()=>{const h=clockHarness();h.s.start();h.s.start();assert.equal(h.pending.size,1);assert.equal(h.delays[0],0);await h.tick();assert.deepEqual(h.calls,['check']);assert.equal(h.delays.at(-1),300000);h.s.stop();assert.equal(h.pending.size,0);});
test('automatic errors back off and successful check resets failures',async()=>{let outcome='retry';const h=clockHarness({check:async()=>outcome});h.s.start();await h.tick();assert.equal(h.delays.at(-1),30000);await h.tick();assert.equal(h.delays.at(-1),120000);await h.tick();assert.equal(h.delays.at(-1),900000);await h.tick();assert.equal(h.delays.at(-1),1800000);for(let i=0;i<8;i++)await h.tick();assert.equal(h.delays.at(-1),21600000);outcome='ok';await h.tick();outcome='retry';await h.tick();assert.equal(h.delays.at(-1),30000);});
test('disabled checks, disconnected engine, and busy requests never trigger a download',async()=>{let enabled=false,ready=true;const h=clockHarness({enabled:()=>enabled,ready:()=>ready});h.s.start();await h.tick();assert.equal(h.calls.length,0);enabled=true;ready=false;await h.tick();assert.equal(h.calls.length,0);assert.equal(h.delays.at(-1),15000);ready=true;await h.tick();assert.equal(h.calls.length,1);});
test('stopping or resetting an in-flight scheduler never creates duplicate timers',async()=>{let resolve;const h=clockHarness({check:()=>new Promise(r=>resolve=r)});h.s.start();const pending=h.tick();h.s.reset();resolve('ok');await pending;assert.equal(h.pending.size,1);assert.equal(h.delays.at(-1),0);h.s.stop();assert.equal(h.pending.size,0);});
test('unexpected check rejection is quiet and scheduled for retry',async()=>{const h=clockHarness({check:async()=>{throw new Error('offline')}});h.s.start();await h.tick();assert.equal(h.delays.at(-1),30000);});
test('background controller only checks and quietly records network errors for settings',async()=>{const calls=[];const h=harness({api:async(...args)=>{calls.push(args);throw new Error('Offline')}});assert.equal(await h.c.check(true),'retry');assert.equal(h.c.snapshot().error,'');assert.equal(h.c.snapshot().message,'');assert.match(h.c.snapshot().backgroundError,/Offline/);assert.match(html(h.c.snapshot(),true,'1.17.59'),/Use Check for updates to retry/);assert.deepEqual(calls,[['/updates/check','POST',{}]]);});
test('background availability never downloads or installs; manual check reveals snoozed notice',async()=>{let reveals=0;const calls=[];const h=harness({api:async(...args)=>{calls.push(args);return available;},reveal:()=>reveals++});assert.equal(await h.c.check(true),'ok');assert.equal(reveals,0);assert.equal(notice(h.c.snapshot()).kind,'download');await h.c.check();assert.equal(reveals,1);assert.deepEqual(calls.map(c=>c[0]),['/updates/check','/updates/check']);});
test('same-version same-stage snooze persists but expires, and newer versions appear',()=>{const prefs={dismissed:'download:1.17.60',until:100};assert.equal(notice({data:available},prefs,99),null);assert.equal(notice({data:available},prefs,100).kind,'download');assert.equal(notice({data:{...available,latest:{version:'1.17.61'}}},prefs,99).version,'1.17.61');assert.equal(notice({data:{...available,staged:{version:'1.17.60'}}},prefs,99).kind,'install');});
test('staged update wins over newer feed release and final install reminder respects Later',()=>{const data={...available,staged:{version:'1.17.59'}};assert.equal(notice({data}).version,'1.17.59');assert.equal(notice({data},{dismissed:'install:1.17.59',until:100},50),null);const text=noticeHTML({data},{},50);assert.match(text,/Install and restart/);assert.match(text,/data-app-update="confirm" data-version="1.17.59"/);assert.match(text,/Save your work before restarting/);assert.doesNotMatch(text,/Review install and restart|data-app-update="review"/);});
test('no update, unsupported platform, and install in progress have no notice',()=>{for(const data of [null,{...available,hasUpdate:false},{...available,supported:false},{...available,installing:true}])assert.equal(notice({data}),null);});
test('notice content is escaped and busy actions are disabled without hiding Later',()=>{const content=noticeHTML({data:{...available,latest:{version:'<svg>'}},busy:true,error:'<script>',message:'Downloading'},{},0);assert.match(content,/&lt;svg&gt;/);assert.match(content,/&lt;script&gt;/);assert.match(content,/disabled/);assert.match(content,/data-app-update="later"/);assert.doesNotMatch(content,/<svg>|<script>/);});
test('repeated download clicks and background checks do not overlap the download',async()=>{let resolve,count=0;const h=harness({api:async path=>{if(path==='/updates/check')return available;count++;return new Promise(r=>resolve=r);}});await h.c.check();const pending=h.c.download('1.17.60');await h.c.download('1.17.60');assert.equal(await h.c.check(true),'skip');assert.equal(count,1);resolve({...available,staged:{version:'1.17.60'}});await pending;assert.equal(notice(h.c.snapshot()).kind,'install');});
test('download failure retains retry action and does not offer installation',async()=>{const h=harness({api:async path=>{if(path==='/updates/check')return available;throw new Error('Connection interrupted');}});await h.c.check();await h.c.download('1.17.60');assert.equal(h.c.snapshot().busy,false);assert.equal(notice(h.c.snapshot()).kind,'download');assert.match(noticeHTML(h.c.snapshot(),{}),/Connection interrupted/);});

test('executable paths and naming warnings are visible and safely escaped',()=>{const text=html({data:{...staged,executable:{currentPath:'C:/<old>.exe',canonicalPath:'C:/<new>.exe',warning:'<unsafe>'}}},true,'1.17.60');assert.match(text,/Running file/);assert.match(text,/Stable executable/);assert.match(text,/Pinned shortcuts, shared shortcuts/);assert.match(text,/&lt;unsafe&gt;/);assert.doesNotMatch(text,/<old>|<new>|<unsafe>/);});

test('scheduler exposes automatic attempt, result and next retry without recording disabled checks',async()=>{
  let now=1000,enabled=true,result='retry';const reports=[];
  const h=clockHarness({now:()=>now,enabled:()=>enabled,check:async()=>result,describeResult:r=>r==='retry'?'Network unavailable':'Up to date',changed:s=>reports.push(s)});
  h.s.start();assert.equal(h.s.snapshot().nextAt,1000);now=6000;await h.tick();
  assert.equal(h.s.snapshot().lastAttempt,6000);assert.equal(h.s.snapshot().lastSummary,'Network unavailable');assert.equal(h.s.snapshot().nextAt,36000);assert.equal(h.s.snapshot().nextKind,'retry');
  assert.ok(reports.some(s=>s.running));enabled=false;now=36000;await h.tick();assert.equal(h.s.snapshot().lastAttempt,6000);
  enabled=true;result='ok';now=40000;await h.tick();assert.equal(h.s.snapshot().lastResult,'ok');assert.equal(h.s.snapshot().nextAt,40000+300000);
});

test('manual completion replaces a pending automatic timer and ignores a late in-flight result',async()=>{
  let resolve;const h=clockHarness({check:()=>new Promise(r=>resolve=r)});h.s.start();const running=h.tick();
  h.s.checked('ok');resolve('retry');await running;
  assert.equal(h.pending.size,1);assert.equal(h.delays.at(-1),300000);assert.equal(h.s.snapshot().failures,0);
});

test('update copy describes public source and binary-only payload',()=>{const text=html({data:staged},true,'1.17.58');assert.doesNotMatch(text,/source repository stays private/);assert.match(text,/public main repository/);});

test('focus freshness gate replaces only one timer and manual success refreshes it',async()=>{
  let now=1000;const h=clockHarness({now:()=>now});h.s.start();await h.tick();
  now+=119999;h.s.wake();assert.equal(h.delays.at(-1),300000);
  now++;h.s.wake();h.s.wake();assert.equal(h.pending.size,1);assert.equal(h.delays.at(-1),0);
  await h.tick();assert.equal(h.calls.length,2);
  h.s.checked('ok');now+=119999;h.s.wake();assert.equal(h.delays.at(-1),300000);
});
test('focus and timer races never overlap an in-flight check',async()=>{
  let resolve,now=1000,checks=0;const h=clockHarness({now:()=>now,check:()=>{checks++;return new Promise(r=>resolve=r)}});
  h.s.start();const pending=h.tick();now+=180000;h.s.wake();h.s.wake();assert.equal(checks,1);assert.equal(h.pending.size,0);
  resolve('ok');await pending;assert.equal(h.pending.size,1);
});
test('hidden or offline checks pause without a short readiness loop and resume when stale',async()=>{
  let active=false,now=1000;const h=clockHarness({now:()=>now,active:()=>active});h.s.start();await h.tick();
  assert.equal(h.calls.length,0);assert.equal(h.delays.at(-1),300000);h.s.wake();assert.equal(h.delays.at(-1),300000);
  active=true;h.s.wake();assert.equal(h.delays.at(-1),0);await h.tick();assert.equal(h.calls.length,1);
});
test('focus respects opt-out, transient retry and server deadline',async()=>{
  let now=1000,enabled=true,after=0;const h=clockHarness({now:()=>now,enabled:()=>enabled,notBefore:()=>after,check:async()=> 'retry'});
  h.s.start();await h.tick();now+=120000;h.s.wake();assert.equal(h.delays.at(-1),30000);
  h.s.checked('ok');after=now+600000;h.s.wake();assert.equal(h.delays.at(-1),300000);
  h.s.checked('retry');assert.equal(h.delays.at(-1),600000);assert.equal(h.pending.size,1);
  enabled=false;after=0;h.s.checked('ok');now+=120000;h.s.wake();assert.equal(h.delays.at(-1),21600000);
});
test('controller retains known release on rate limit and manual checks cannot bypass cooldown',async()=>{
  let now=1000,calls=0;const h=harness({now:()=>now,api:async()=>{calls++;if(calls===1)return available;const e=new Error('Rate limited');e.checkAfter=61000;throw e;}});
  await h.c.check();await h.c.check(true);assert.equal(h.c.snapshot().data.latest.version,'1.17.60');
  assert.equal(h.c.snapshot().checkAfter,61000);await h.c.check();assert.equal(calls,2);assert.match(h.c.snapshot().error,/paused until/);
  now=61000;await h.c.check();assert.equal(calls,3);
});
test('status reload restores server cooldown and keeps cached release review available',async()=>{
  const h=harness({now:()=>1000,api:async()=>({...available,checkAfter:61000})});await h.c.load();
  assert.equal(h.c.snapshot().checkAfter,61000);assert.equal(await h.c.check(),'retry');assert.equal(h.c.snapshot().data.latest.version,'1.17.60');
});

test('cached no-update result after failed or limited check does not claim current freshness',async()=>{
  let calls=0;const h=harness({api:async()=>{if(++calls===1)return {...available,hasUpdate:false};const error=new Error('Rate limited');error.checkAfter=Date.now()+60000;throw error;}});
  await h.c.check();await h.c.check(true);const text=html(h.c.snapshot(),true,'1.17.58');
  assert.doesNotMatch(text,/You are up to date|No newer update is available/);assert.match(text,/last successful check/);assert.match(text,/Rate limited/);
});

test('cached no-update status during server cooldown never claims a fresh successful check',async()=>{
  const h=harness({now:()=>1000,api:async()=>({...available,hasUpdate:false,checkAfter:61000})});await h.c.load();await h.c.check(true);
  const text=html(h.c.snapshot(),true,'1.17.58');assert.doesNotMatch(text,/You are up to date|No newer update is available/);assert.match(text,/last successful check/);
});

test('download progress polls only while pending and ignores a late status after completion',async()=>{
 const pending=new Map(),calls=[];let id=0,finish,late;
 const h=harness({setTimeout:fn=>{pending.set(++id,fn);return id;},clearTimeout:id=>pending.delete(id),api:async path=>{
  calls.push(path);if(path==='/updates/check')return available;if(path==='/updates/download')return new Promise(resolve=>finish=resolve);return new Promise(resolve=>late=resolve);
 }});
 await h.c.check();const download=h.c.download('1.17.60');assert.equal(pending.size,1);
 const [timer,poll]=pending.entries().next().value;pending.delete(timer);const status=poll();
 finish({...available,staged:{version:'1.17.60'}});await download;late({...available,progress:{phase:'download',received:5,total:10}});await status;
 assert.equal(h.c.snapshot().progress.phase,'ready');assert.equal(h.c.snapshot().busy,false);assert.equal(pending.size,0);assert(!calls.includes('/updates/install'));
});
test('measured and unknown download sizes render accessible honest progress',()=>{
 const {progressHTML}=require('../web/assets/app-updater.js');
 const measured=progressHTML({busy:true,progress:{phase:'download',received:1048576,total:2097152}});
 assert.match(measured,/value="1048576" max="2097152"/);assert.match(measured,/1.0 MiB of 2.0 MiB \(50%\)/);
 const unknown=progressHTML({busy:true,progress:{phase:'download',received:1024,total:0}});assert.match(unknown,/1 KiB received/);assert.doesNotMatch(unknown,/<progress[^>]*value=/);
 const installing=progressHTML({progress:{phase:'restarting'}});assert.match(installing,/Installing and restarting/);assert.doesNotMatch(installing,/<progress[^>]*value=/);
});
