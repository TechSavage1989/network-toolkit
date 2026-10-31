'use strict';
// Synthetic browser acceptance for the split Disconnect action in real Toolkit UI.
// No device authentication, remote network transport or actual shell is used.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT;
const origin='https://disconnect133.toolkit.test',report={passed:false,cases:[],errors:[]};
let browser,page;
async function setup(width){
 const context=await browser.newContext({viewport:{width,height:900}});
 const p=await context.newPage();page=p;p.setDefaultTimeout(15000);
 let inflight=0,maxInflight=0;
 const calls=[],disconnectCalls=[],deleteCalls=[],sessions=[
  {id:'synthetic-one',type:'ssh',state:'connected',host:'192.0.2.1',port:22,label:'Synthetic one',cols:80,rows:24},
  {id:'synthetic-two',type:'telnet',state:'connected',host:'192.0.2.2',port:23,label:'Synthetic two',cols:80,rows:24}
 ];
 p.on('pageerror',error=>report.errors.push(error.message));
 await p.route(origin+'/**',async route=>{
  const req=route.request(),pathname=new URL(req.url()).pathname,method=req.method();
  try{
   if(pathname.startsWith('/api/')){
    let data={};
    if(pathname==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64'};
    else if(pathname==='/api/state'&&method==='GET')data={activePage:'terminal',appUpdatePreferences:{enabled:false},terminalSidebarCollapsed:true};
    else if(pathname==='/api/jobs')data=[];
    else if(pathname==='/api/terminals')data={sessions};
    else if(pathname==='/api/terminal-views')data={available:false,views:[],owners:{}};
    else if(pathname==='/api/terminals/events'){await new Promise(resolve=>setTimeout(resolve,80));data={sessions,events:[],after:0};}
    else if(pathname==='/api/remote-browser'&&method==='POST')data={supported:false};
    else if(pathname.match(/^\/api\/terminals\/synthetic-(one|two)\/reconnect$/)&&method==='POST'){
     const id=pathname.split('/')[3];calls.push(id);inflight++;maxInflight=Math.max(maxInflight,inflight);
     await new Promise(resolve=>setTimeout(resolve,25));
     const i=sessions.findIndex(s=>s.id===id);assert(i>=0);
     sessions[i]={...sessions[i],state:'connected',updatedAt:new Date().toISOString()};
     data={session:sessions[i]};inflight--;
    }else if(pathname.match(/^\/api\/terminals\/synthetic-(one|two)\/disconnect$/)&&method==='POST'){
     const id=pathname.split('/')[3];disconnectCalls.push(id);inflight++;maxInflight=Math.max(maxInflight,inflight);
     await new Promise(resolve=>setTimeout(resolve,25));const i=sessions.findIndex(s=>s.id===id);
     if(i<0)throw Error('Session missing: '+id);
     sessions[i]={...sessions[i],state:'closed',updatedAt:new Date().toISOString()};data={session:sessions[i]};inflight--;
    }else if(pathname.match(/^\/api\/terminals\/synthetic-(one|two)$/)&&method==='DELETE'){
     const id=pathname.split('/')[3];deleteCalls.push(id);const i=sessions.findIndex(s=>s.id===id);
     if(i<0)throw Error('Session missing on close: '+id);
     sessions.splice(i,1);data={};
    }else if(pathname.match(/^\/api\/terminals\/synthetic-(one|two)\/resize$/)&&method==='POST')data={};
    else if(pathname==='/api/state'&&method==='PUT')data={};
    else if(pathname==='/api/editor-views/current/activity'&&method==='POST')data={};
    else if(method!=='GET')throw Error('Unexpected mutation '+method+' '+pathname);
    return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
   }
   let file=path.join(root,'web',pathname==='/'?'index.html':pathname);
   if(!fs.existsSync(file))file=path.join(root,pathname);
   let bytes=fs.readFileSync(file);
   if(pathname==='/')bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','test132')
    .replaceAll('__WINDOW_MODE__','main').replaceAll('__TERMINAL_VIEW__','null').replaceAll('__EDITOR_VIEW__','null');
   return route.fulfill({body:bytes,contentType:pathname==='/'?'text/html':pathname.endsWith('.js')?'application/javascript':pathname.endsWith('.css')?'text/css':pathname.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
  }catch(error){report.errors.push(error.message);return route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:'Synthetic mock operation rejected'})});}
 });
 await p.goto(origin+'/');await p.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
 await p.evaluate(()=>navigate('terminal'));await p.waitForFunction(()=>terminalSessions.size===2&&!!document.querySelector('#terminalReconnectMore'));
 return {context,p,calls,disconnectCalls,deleteCalls,resetConnected(){for(let i=0;i<sessions.length;i++)sessions[i]={...sessions[i],state:'connected',updatedAt:new Date().toISOString()};},get maxInflight(){return maxInflight;}};
}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 for(const width of [1280,390]){
  const f=await setup(width),p=f.p,arrow=p.locator('#terminalDisconnectMore');
  const disconnect=p.locator('#terminalActiveActions .terminal-disconnect-split');
  const reconnect=p.locator('#terminalActiveActions .terminal-reconnect-split');
  const primary=disconnect.locator('[data-terminal-action="disconnect"]');
  assert.equal(await primary.textContent(),'Disconnect');
  assert.equal(await disconnect.locator('[data-terminal-action="disconnect-all"]').textContent(),'Disconnect all');
  assert.equal(await arrow.getAttribute('aria-expanded'),'false');
  assert(await arrow.isEnabled());
  assert(await p.evaluate(()=>{
   const actions=document.querySelector('.terminal-active-actions .terminal-toolgroup'),children=[...actions.children];
   return children.indexOf(actions.querySelector('.terminal-reconnect-split'))===children.indexOf(actions.querySelector('.terminal-disconnect-split'))+1;
  }),'Reconnect must be directly beside Disconnect');
  await arrow.click();assert.equal(await arrow.getAttribute('aria-expanded'),'true');
  const option=disconnect.locator('[data-terminal-action="disconnect-all"]');
  assert(await option.isVisible());
  const position=await disconnect.evaluate(group=>{
   const parent=group.getBoundingClientRect(),menu=group.querySelector('#terminalDisconnectOptions').getBoundingClientRect();
   return {parentBottom:parent.bottom,menuTop:menu.top,overflow:document.documentElement.scrollWidth-innerWidth};
  });
  assert(position.menuTop>=position.parentBottom-1);
  assert(position.overflow<=1,'Disconnect dropdown must not introduce horizontal overflow');
  await arrow.focus();await p.keyboard.press('Escape');
  assert.equal(await arrow.getAttribute('aria-expanded'),'false');
  assert.equal(await option.isVisible(),false);
  await arrow.click();await option.click();
  await p.locator('[data-terminal-window-bulk-confirm]').waitFor({state:'visible'});
  assert.equal(f.disconnectCalls.length,0,'Opening a bulk action does not disconnect anything');
  await p.locator('#modalRoot [data-action="close-modal"]').first().click();
  assert.equal(f.disconnectCalls.length,0,'Cancellation must leave both sessions connected');
  await p.locator('[data-terminal-tab="synthetic-two"]').click({button:'right'});
  const menu=p.locator('#terminalTabContextMenu [data-terminal-tab-action="disconnect-all"]');
  assert(await menu.isVisible());assert(await menu.isEnabled());
  await menu.click();await p.locator('[data-terminal-window-bulk-confirm]').waitFor({state:'visible'});
  await p.locator('#modalRoot #terminalWindowBulkDontAsk').check();
  await p.locator('#modalRoot [data-terminal-window-bulk-confirm]').click();
  await p.waitForFunction(()=>!window.NetworkToolkitTerminalTabs.windowBulkBusy());
  assert.deepEqual(f.disconnectCalls,['synthetic-one','synthetic-two']);
  assert.equal(f.maxInflight,1,'Disconnect all must be serialized');
  assert.equal(await p.evaluate(()=>state.terminalConfirmDisconnectAll),false);
  assert.equal(await p.evaluate(()=>state.terminalConfirmReconnectAll),true);
  assert.equal(await p.evaluate(()=>state.terminalConfirmCloseAll),true);
  await p.evaluate(()=>{const el=document.querySelector('#terminalConfirmDisconnectAll');if(!el)throw Error('Missing settings checkbox');el.checked=true;el.dispatchEvent(new Event('change',{bubbles:true}));});
  assert.equal(await p.evaluate(()=>state.terminalConfirmDisconnectAll),true,'Settings should restore the bulk confirmation');
  await p.evaluate(()=>{const el=document.querySelector('#terminalConfirmReconnectAll');el.checked=false;el.dispatchEvent(new Event('change',{bubbles:true}));});
  await p.locator('#terminalReconnectMore').click();
  await p.locator('#terminalReconnectOptions [data-terminal-action="reconnect-all"]').click();
  await p.waitForFunction(()=>!window.NetworkToolkitTerminalTabs.reconnectAllBusy());
  assert.deepEqual(f.calls,['synthetic-one','synthetic-two'],'Optional reconnect confirmation must not block both eligible sessions');
  assert.equal(await p.locator('[data-terminal-reconnect-all-confirm]').count(),0);
  await p.evaluate(()=>{const el=document.querySelector('#terminalConfirmDisconnectAll');el.checked=false;el.dispatchEvent(new Event('change',{bubbles:true}));});
  await p.locator('#terminalDisconnectMore').click();
  await p.locator('#terminalDisconnectOptions [data-terminal-action="disconnect-all"]').click();
  await p.waitForFunction(()=>!window.NetworkToolkitTerminalTabs.windowBulkBusy());
  assert.deepEqual(f.disconnectCalls,['synthetic-one','synthetic-two','synthetic-one','synthetic-two']);
  assert.equal(await p.locator('[data-terminal-window-bulk-confirm]').count(),0);
  await p.evaluate(()=>{const el=document.querySelector('#terminalConfirmCloseAll');el.checked=false;el.dispatchEvent(new Event('change',{bubbles:true}));});
  await p.locator('[data-terminal-tab="synthetic-one"]').click({button:'right'});
  const closeAll=p.locator('#terminalTabContextMenu [data-terminal-tab-action="close-all"]');
  assert(await closeAll.isEnabled());await closeAll.click();
  await p.waitForFunction(()=>terminalSessions.size===0);
  assert.deepEqual(f.deleteCalls,['synthetic-one','synthetic-two']);
  assert.equal(await p.locator('[data-terminal-window-bulk-confirm]').count(),0);
  if(out)await p.locator('#terminalActiveActions').screenshot({path:path.join(out,'terminal-disconnect133-'+width+'.png')});
  report.cases.push({width,adjacent:true,dropBelow:true,keyboardEscape:true,cancelSafe:true,modalOptOut:true,settingsRestore:true,disconnectOrder:f.disconnectCalls.slice(),reconnectOrder:f.calls.slice(),closed:f.deleteCalls.slice(),maxInflight:f.maxInflight});
  await f.context.close();
 }
 assert.deepEqual(report.errors,[]);report.passed=true;console.log(JSON.stringify(report));
})().catch(async error=>{report.failure=error.stack||String(error);console.error(report.failure);process.exitCode=1;
 if(out&&page&&!page.isClosed())await page.screenshot({path:path.join(out,'terminal-disconnect133-failure.png')}).catch(()=>{});
}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'terminal-disconnect133-results.json'),JSON.stringify(report,null,2));await browser?.close();});
