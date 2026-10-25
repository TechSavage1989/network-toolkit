'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const source=fs.readFileSync(require.resolve('../web/assets/app-updater.js'),'utf8');
const pageSource=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const initSource=pageSource.slice(pageSource.indexOf('function startupStateLoader(){'),pageSource.indexOf('\nfunction toolForSavedTarget'));
const available={currentVersion:'1.17.61',supported:true,installing:false,busy:[],hasUpdate:true,staged:null,latest:{version:'1.17.62'}};
const deferred=()=>{let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b});return {promise,resolve,reject};};
const settle=async()=>{for(let i=0;i<12;i++)await Promise.resolve();};
function ui(options={}){
  const document=makeDocument('<button id="mobileMenu"></button><button id="settingsButton"></button><div id="appUpdateNotice"></div><div id="modalRoot"><div id="existingDialog">Existing work</div></div>');
  const calls=[],timers=new Map(),saved=[],toasts=[],listeners={};let timerId=0,now=1792872000000,data=structuredClone(available);
  class TestDate extends Date{constructor(value=now){super(value)}static now(){return now}}
  const c={document,AbortController,Date:TestDate,online:true,OFFLINE:false,shutDown:false,TERMINAL_ONLY:false,DETACHED_TERMINAL:false,status:{version:'1.17.61'},VERSION:'1.17.61',state:{appUpdatePreferences:{enabled:true,...options.preferences}},
    addEventListener:(name,fn)=>(listeners[name]??=[]).push(fn),
    setTimeout:(fn,delay)=>{timers.set(++timerId,{fn,delay});return timerId},clearTimeout:id=>timers.delete(id),
    saveState:()=>saved.push(structuredClone(c.state.appUpdatePreferences)),flushState:async()=>true,toast:message=>toasts.push(message),
    api:async(path,method,body)=>{calls.push([path,method,body]);if(options.api)return options.api(path,method,body);return structuredClone(data)},
    openModal:(title,body,footer)=>{document.getElementById('modalRoot').innerHTML='<h2>'+title+'</h2>'+body+footer},
    ...options.context};
  if(options.brandTimers){
    const schedule=c.setTimeout,cancel=c.clearTimeout;
    c.setTimeout=function(...args){assert.equal(this?.window,this,'setTimeout requires the Window receiver');return schedule(...args)};
    c.clearTimeout=function(...args){assert.equal(this?.window,this,'clearTimeout requires the Window receiver');return cancel(...args)};
  }
  c.window=c;vm.createContext(c);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/app-update-notes.js'),'utf8'),c);vm.runInContext(source,c);
  return {c,document,calls,timers,saved,toasts,listeners,advance:ms=>now+=ms,setData:value=>data=value,
    async tick(){const entry=[...timers.entries()].sort((a,b)=>a[1].delay-b[1].delay)[0];assert.ok(entry,'a timer is pending');const [id,timer]=entry;timers.delete(id);now+=timer.delay;await timer.fn();await settle();},
    nextDelay:()=>timers.values().next().value?.delay,
    async click(action){const button=document.querySelector('[data-app-update="'+action+'"]');assert.ok(button,action+' exists');for(const listener of document.listeners.click||[])await listener({target:button});},
    settings(){document.getElementById('modalRoot').innerHTML='<section id="appUpdateSettings">'+c.NetworkToolkitUpdater.html()+'</section>';return c.NetworkToolkitUpdater.load()},
    scheduleText:()=>document.querySelector('[data-app-update-schedule]')?.textContent||'',
    noticeText:()=>document.getElementById('appUpdateNotice').textContent};
}
function init(h){
  const c=h.c,nothing=()=>{};
  for(const name of ['NetworkToolkitSource','NetworkToolkitWorkspace','NetworkToolkitResultDetails','NetworkToolkitScanRoutes','NetworkToolkitConfiguration','NetworkToolkitNavigation','NetworkToolkitTerminalInteraction','NetworkToolkitCredentials'])c[name]={init:nothing};
  Object.assign(c,{$:selector=>c.document.querySelector(selector),$$:selector=>c.document.querySelectorAll(selector),icon:()=>'',initToolWorkspace:nothing,configureWindowMode:nothing,defaults:nothing,renderPage:nothing,setTheme:nothing,updateEngine:nothing,startTerminalFeed:nothing,startTerminalWindowRequests:nothing,startTerminalViews:nothing,pollSoon:nothing,system:{},jobs:[],fullVersions:new Map(),jobVersions:new Map(),fullVersion:()=>'',mergeState:s=>{const {appUpdatePreferences,...other}=s;Object.assign(c.state,other);c.NetworkToolkitUpdater.mergePreferences(appUpdatePreferences)}});
  vm.runInContext(initSource,c);
  return c.init();
}

test('actual initializer starts the updater before unrelated system and jobs requests settle',async()=>{
  const system=deferred(),jobs=deferred();
  const h=ui({context:{online:false},api:async path=>path==='/status'?{version:'1.17.61'}:path==='/state'?{appUpdatePreferences:{enabled:true}}:path==='/system'?system.promise:path==='/jobs'?jobs.promise:available});
  let finished=false;const initializing=init(h).then(()=>finished=true);await settle();
  assert.equal(finished,false);assert.equal(h.nextDelay(),0);
  await h.tick();assert.match(h.noticeText(),/1.17.62 is available/);
  assert.deepEqual(h.calls.filter(c=>c[0].startsWith('/updates/')).map(c=>c[0]),['/updates/status','/updates/check']);
  assert.ok(h.document.getElementById('existingDialog'));
  system.resolve({});jobs.resolve([]);await initializing;assert.equal(h.timers.size,1);
});

test('delayed saved preferences prevent checks and preserve disabled setting',async()=>{
  const prefs=deferred();const h=ui({api:async path=>path==='/status'?{}:path==='/state'?prefs.promise:path==='/jobs'?[]:{}});
  const initializing=init(h);await settle();assert.equal([...h.timers.values()].some(t=>t.delay===0),false);
  await h.settings();assert.match(h.scheduleText(),/waiting for your saved update preferences/);
  assert.equal(h.document.querySelector('[data-app-update-auto]').hasAttribute('disabled'),true);
  prefs.resolve({appUpdatePreferences:{enabled:false}});await initializing;await settle();await h.tick();
  assert.equal(h.calls.some(c=>c[0]==='/updates/check'),false);assert.match(h.scheduleText(),/turned off/);
});

test('a failed preferences read retries locally and never assumes permission to check the feed',async()=>{
  let reads=0;const h=ui({api:async path=>{if(path==='/state'){if(++reads===1)throw Error('Settings unavailable');return {appUpdatePreferences:{enabled:false}};}return available;}});
  h.c.NetworkToolkitUpdater.start(Promise.reject(Error('Initial settings unavailable')),async()=>{const saved=await h.c.api('/state');h.c.NetworkToolkitUpdater.mergePreferences(saved.appUpdatePreferences)});await settle();
  await h.tick();assert.equal(h.nextDelay(),30000);assert.deepEqual(h.calls.map(c=>c[0]),['/state']);
  await h.settings();assert.match(h.scheduleText(),/Next automatic retry/);assert.match(h.scheduleText(),/Settings unavailable/);
  await h.tick();assert.equal(h.calls.some(c=>c[0]==='/updates/check'),false);assert.match(h.scheduleText(),/turned off/);
});

test('settings status loading overlapping the timer is deferred, then automatically recovered',async()=>{
  const pending=deferred();let count=0;const h=ui({api:async path=>path==='/updates/status'&&++count===1?pending.promise:available});
  h.c.NetworkToolkitUpdater.start();const settings=h.settings();await h.tick();
  assert.ok([...h.timers.values()].some(t=>t.delay===15000));assert.equal(h.calls.some(c=>c[0]==='/updates/check'),false);
  assert.match(h.scheduleText(),/Deferred while another update action was busy/);
  pending.resolve({...available,hasUpdate:false});await settings;await h.tick();
  assert.match(h.noticeText(),/1.17.62 is available/);assert.match(h.scheduleText(),/Last automatic attempt/);assert.match(h.scheduleText(),/Update available: 1.17.62/);assert.match(h.scheduleText(),/Next automatic check/);
});

test('transient failure retries promptly and quietly, then returns to the five-minute cadence',async()=>{
  let checks=0;const h=ui({api:async path=>{if(path==='/updates/check'&&++checks===1)throw Error('Network briefly unavailable');return {...available,hasUpdate:path==='/updates/check'};}});
  h.c.NetworkToolkitUpdater.start();await h.tick();assert.equal(h.nextDelay(),30000);assert.equal(h.noticeText(),'');assert.equal(h.toasts.length,0);
  await h.settings();assert.match(h.scheduleText(),/Network briefly unavailable/);assert.match(h.scheduleText(),/Next automatic retry/);
  await h.tick();assert.equal(checks,2);assert.equal(h.nextDelay(),300000);assert.match(h.noticeText(),/1.17.62 is available/);
  assert.equal(h.calls.some(c=>['/updates/download','/updates/install'].includes(c[0])),false);
});

test('manual check before startup timer reveals a snoozed release without a duplicate check',async()=>{
  const h=ui({preferences:{dismissed:'download:1.17.62',until:Date.now()+86400000}});
  h.c.NetworkToolkitUpdater.start();await h.settings();await h.click('check');
  assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,1);assert.equal(h.nextDelay(),300000);assert.equal(h.timers.size,1);assert.match(h.noticeText(),/available/);
});

test('manual check while initial preferences load preserves the explicit reveal and does not overwrite saved opt-out',async()=>{
  const prefs=deferred(),h=ui();
  const merged=prefs.promise.then(value=>h.c.NetworkToolkitUpdater.mergePreferences(value));
  h.c.NetworkToolkitUpdater.start(merged);await h.settings();await h.click('check');
  assert.equal(h.saved.length,0);assert.equal(h.timers.size,0);
  prefs.resolve({enabled:false,dismissed:'download:1.17.62',until:Date.now()+86400000});await settle();
  assert.equal(h.c.state.appUpdatePreferences.enabled,false);assert.equal(h.c.state.appUpdatePreferences.dismissed,'');assert.equal(h.saved.length,1);assert.equal(h.nextDelay(),21600000);
  await h.tick();assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,1);
});

test('snoozed versions stay hidden while newer release or stage changes are surfaced',async()=>{
  const h=ui({preferences:{dismissed:'download:1.17.62',until:Date.now()+86400000}});h.c.NetworkToolkitUpdater.start();await h.tick();assert.equal(h.noticeText(),'');
  h.setData({...available,latest:{version:'1.17.63'}});await h.tick();assert.match(h.noticeText(),/1.17.63/);
  h.setData({...available,staged:{version:'1.17.62'}});await h.tick();assert.match(h.noticeText(),/downloaded and verified/);
});

test('disabling automatic checks during status load prevents the feed request',async()=>{
  const pending=deferred(),h=ui({api:async()=>pending.promise});h.c.NetworkToolkitUpdater.start();
  const checking=h.tick();await settle();h.c.state.appUpdatePreferences.enabled=false;
  pending.resolve(available);await checking;assert.deepEqual(h.calls.map(c=>c[0]),['/updates/status']);
});

test('automatic failure details are escaped in settings without replacing an unrelated dialog',async()=>{
  const h=ui({api:async()=>{throw Error('<script>bad</script>')}});h.c.NetworkToolkitUpdater.start();await h.tick();assert.ok(h.document.getElementById('existingDialog'));
  await h.settings();const content=h.document.getElementById('appUpdateSettings').innerHTML;
  assert.match(content,/&lt;script&gt;bad/);assert.doesNotMatch(content,/<script>/);
});


test('failed state load then manual reveal restores complete saved profiles before persisting',async()=>{
  let reads=0;const stored={theme:'light',profiles:[{id:'saved-profile',name:'Important host',target:'example.test'}],appUpdatePreferences:{enabled:true,dismissed:'download:1.17.62',until:Date.now()+86400000}};
  const h=ui({api:async path=>{if(path==='/state'){if(++reads===1)throw Error('Temporary settings failure');return structuredClone(stored)}return path==='/jobs'?[]:available}});
  const writes=[];h.c.saveState=()=>writes.push(structuredClone(h.c.state));
  await init(h);await settle();await h.settings();await h.click('check');assert.equal(writes.length,0);
  await h.tick();assert.equal(writes.length,1);assert.deepEqual(writes[0].profiles,stored.profiles);assert.equal(writes[0].theme,'light');assert.equal(writes[0].appUpdatePreferences.dismissed,'');
});

test('unrelated edits after failed initial state read are retained without overwriting saved settings',async()=>{
  let reads=0;const stored={profiles:[{id:'saved-profile',name:'Important host'}],appUpdatePreferences:{enabled:true}};
  const h=ui({api:async path=>{if(path==='/state'){if(++reads===1)throw Error('Temporary settings failure');return structuredClone(stored)}return path==='/jobs'?[]:available}});
  await init(h);await settle();await h.settings();await h.click('check');
  h.c.state.profiles=[{id:'new-profile',name:'Unsaved new host'}];
  await h.tick();assert.equal(h.saved.length,0);assert.equal(h.c.state.profiles[0].id,'new-profile');assert.equal(stored.profiles[0].id,'saved-profile');
  assert.match(h.scheduleText(),/Preserve your current work.*reopen the app.*Existing saved settings are unchanged/s);
});

test('full-state writer refuses startup defaults until saved settings are loaded',async()=>{
  const calls=[],c={stateLoaded:false,state:{profiles:[]},stateRevision:0,savedStateRevision:-1,stateWrites:Promise.resolve(true),shutDown:false,OFFLINE:false,online:true,clearTimeout:()=>{},saveTimer:null,api:async(...args)=>calls.push(args),storageError:()=>{},lastStorageError:''};
  vm.createContext(c);vm.runInContext(pageSource.slice(pageSource.indexOf('function flushState('),pageSource.indexOf('\nfunction closeApp(')),c);
  assert.equal(await c.flushState(),false);assert.equal(calls.length,0);c.stateLoaded=true;assert.equal(await c.flushState(),true);assert.equal(calls.length,1);
});


test('actual startup checks with unrelated platform status and diagnostics still pending',async()=>{
  const pending=deferred();const h=ui({context:{online:false},api:async path=>path==='/state'?{appUpdatePreferences:{enabled:true}}:['/status','/system','/jobs'].includes(path)?pending.promise:available});
  const initializing=init(h);await settle();await h.tick();
  assert.equal(h.c.online,false);assert.match(h.noticeText(),/1.17.62 is available/);
  assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,1);
  pending.resolve({});await initializing;
});

test('hung startup preferences time out locally and recover without assuming opt-in',async()=>{
  const pending=deferred();let reads=0;
  const h=ui({api:async path=>path==='/state'?(++reads===1?pending.promise:{appUpdatePreferences:{enabled:false}}):path==='/jobs'?[]:available});
  const initializing=init(h);await settle();assert.equal(h.calls.some(c=>c[0]==='/updates/check'),false);
  await h.tick();await initializing;await settle();await h.tick();
  assert.equal(reads,2);assert.equal(h.c.state.appUpdatePreferences.enabled,false);
  assert.equal(h.calls.some(c=>c[0]==='/updates/check'),false);
  pending.resolve({appUpdatePreferences:{enabled:true}});await settle();assert.equal(h.c.state.appUpdatePreferences.enabled,false);
});

test('hung release request aborts and late response cannot replace the next result',async()=>{
  const pending=deferred();let checks=0;
  const h=ui({api:async path=>path==='/updates/check'&&++checks===1?pending.promise:{...available,hasUpdate:path==='/updates/check'}});
  h.c.NetworkToolkitUpdater.start();const running=h.tick();await settle();
  await h.tick();await running;assert.equal(h.noticeText(),'');assert.equal(h.nextDelay(),30000);
  await h.tick();assert.match(h.noticeText(),/1.17.62 is available/);
  pending.resolve({...available,latest:{version:'9.9.9'}});await settle();assert.doesNotMatch(h.noticeText(),/9.9.9/);
});

test('changed diagnostic work cannot permanently prevent saved updater preferences loading',async()=>{
  let reads=0;const h=ui({api:async path=>{if(path==='/state'){if(++reads===1)throw Error('Temporary failure');return {profiles:[],appUpdatePreferences:{enabled:true}};}return path==='/jobs'?[]:available;}});
  await init(h);await settle();h.c.state.profiles=[{id:'unsaved'}];await h.tick();
  assert.match(h.noticeText(),/1.17.62 is available/);assert.equal(h.c.state.profiles[0].id,'unsaved');assert.equal(h.saved.length,0);
});


test('actual initializer preserves Window receiver for startup, manual replacement and toggle cancellation',async()=>{
  const h=ui({brandTimers:true,api:async path=>path==='/state'?{appUpdatePreferences:{enabled:true}}:path==='/jobs'?[]:available});
  await init(h);await settle();assert.equal(h.timers.size,1);assert.equal(h.nextDelay(),0);
  await h.tick();assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,1);
  assert.equal(h.timers.size,1);assert.equal(h.nextDelay(),300000);
  await h.settings();assert.match(h.scheduleText(),/Last automatic attempt:.*Update available/);
  await h.click('check');assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,2);
  assert.equal(h.timers.size,1);assert.equal(h.nextDelay(),300000);
  for(const enabled of [false,true]){
    const box=h.document.querySelector('[data-app-update-auto]');box.checked=enabled;
    for(const handler of h.document.listeners.change||[])await handler({target:box});
    assert.equal(h.timers.size,1);assert.equal(h.nextDelay(),0);
    await h.tick();assert.equal(h.timers.size,1);assert.equal(h.nextDelay(),enabled?300000:21600000);
  }
  assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,3);
  assert.equal(h.calls.some(c=>/updates\/(download|install)/.test(c[0])),false);
});

test('Window-bound timer cancellation retains bounded retry and no duplicate timer',async()=>{
  let attempts=0;
  const h=ui({brandTimers:true,api:async path=>{
    if(path==='/state')return {appUpdatePreferences:{enabled:true}};
    if(path==='/jobs')return [];
    if(path==='/updates/check'&&++attempts===1)throw new Error('Temporary connection failure');
    return available;
  }});
  await init(h);await settle();await h.tick();
  assert.equal(attempts,1);assert.equal(h.timers.size,1);assert.equal(h.nextDelay(),30000);
  await h.settings();assert.match(h.scheduleText(),/Next automatic retry/);
  await h.click('check');assert.equal(attempts,2);assert.equal(h.timers.size,1);assert.equal(h.nextDelay(),300000);
  await h.tick();assert.equal(attempts,3);assert.equal(h.timers.size,1);
});

test('native-bound focus, visibility and online handlers share freshness and one automatic timer',async()=>{
  const h=ui({brandTimers:true,context:{navigator:{onLine:true}}});h.c.NetworkToolkitUpdater.start();await h.tick();
  assert.equal(h.listeners.focus.length,1);assert.equal(h.listeners.online.length,1);
  h.advance(120000);for(const fn of h.listeners.focus)fn();for(const fn of h.document.listeners.visibilitychange)fn();for(const fn of h.listeners.online)fn();
  assert.equal(h.timers.size,1);assert.equal(h.nextDelay(),0);await h.tick();assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,2);
  h.c.document.hidden=true;await h.tick();assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,2);assert.equal(h.nextDelay(),300000);
  h.c.document.hidden=false;for(const fn of h.document.listeners.visibilitychange)fn();await h.tick();assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,3);
  h.c.navigator.onLine=false;await h.tick();assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,3);
  h.c.navigator.onLine=true;for(const fn of h.listeners.online)fn();await h.tick();assert.equal(h.calls.filter(c=>c[0]==='/updates/check').length,4);
  assert.equal(h.calls.some(c=>/updates\/(download|install)/.test(c[0])),false);
});
