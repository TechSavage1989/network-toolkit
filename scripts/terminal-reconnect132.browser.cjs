'use strict';
// Synthetic browser acceptance for the split Reconnect action in real Toolkit UI.
// No device authentication, remote network transport or actual shell is used.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT;
const origin='https://reconnect132.toolkit.test',report={passed:false,cases:[],errors:[]};
let browser,page;
async function setup(width){
 const context=await browser.newContext({viewport:{width,height:900}});
 const p=await context.newPage();page=p;p.setDefaultTimeout(15000);
 let inflight=0,maxInflight=0;
 const calls=[],sessions=[
  {id:'synthetic-one',type:'ssh',state:'connected',host:'192.0.2.1',port:22,label:'Synthetic one',cols:80,rows:24},
  {id:'synthetic-two',type:'telnet',state:'closed',host:'192.0.2.2',port:23,label:'Synthetic two',cols:80,rows:24}
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
    else if(pathname==='/api/terminals/events')data={sessions,events:[],after:0};
    else if(pathname==='/api/remote-browser'&&method==='POST')data={supported:false};
    else if(pathname.match(/^\/api\/terminals\/synthetic-(one|two)\/reconnect$/)&&method==='POST'){
     const id=pathname.split('/')[3];calls.push(id);inflight++;maxInflight=Math.max(maxInflight,inflight);
     await new Promise(resolve=>setTimeout(resolve,25));
     const i=sessions.findIndex(s=>s.id===id);assert(i>=0);
     sessions[i]={...sessions[i],state:'connected',updatedAt:new Date().toISOString()};
     data={session:sessions[i]};inflight--;
    }else if(pathname.match(/^\/api\/terminals\/synthetic-(one|two)\/resize$/)&&method==='POST')data={};
    else if(pathname==='/api/state'&&method==='PUT')data={};
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
 return {context,p,calls,get maxInflight(){return maxInflight;}};
}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 for(const width of [1280,390]){
  const f=await setup(width),p=f.p,arrow=p.locator('#terminalReconnectMore');
  const primary=p.locator('#terminalActiveActions .terminal-reconnect-main');
  assert.equal(await primary.textContent(),'Reconnect');
  assert.equal(await arrow.getAttribute('aria-expanded'),'false');
  assert.equal(await arrow.isEnabled(),true);
  await arrow.click();
  assert.equal(await arrow.getAttribute('aria-expanded'),'true');
  const option=p.locator('#terminalReconnectOptions [data-terminal-action="reconnect-all"]');
  assert(await option.isVisible());
  const position=await p.locator('.terminal-reconnect-split').evaluate(group=>{
   const parent=group.getBoundingClientRect(),menu=group.querySelector('#terminalReconnectOptions').getBoundingClientRect();
   return {parentBottom:parent.bottom,menuTop:menu.top,overflow:document.documentElement.scrollWidth-innerWidth};
  });
  assert(position.menuTop>=position.parentBottom-1,'Secondary action is below the primary button');
  assert(position.overflow<=1,'Split reconnect actions overflow the application');
  await arrow.focus();await p.keyboard.press('Escape');
  assert.equal(await arrow.getAttribute('aria-expanded'),'false');
  assert.equal(await option.isVisible(),false);
  await arrow.click();await option.click();
  await p.locator('[data-terminal-reconnect-all-confirm]').waitFor({state:'visible'});
  assert.equal(f.calls.length,0,'Opening secondary action must not reconnect before confirmation');
  await p.locator('#modalRoot [data-action="close-modal"]').first().click();
  assert.equal(f.calls.length,0,'Cancel must not send reconnect requests');
  await p.locator('[data-terminal-tab="synthetic-two"]').click({button:'right'});
  const menuAction=p.locator('#terminalTabContextMenu [data-terminal-tab-action="reconnect-all"]');
  assert(await menuAction.isVisible());assert(await menuAction.isEnabled());
  await menuAction.click();
  assert.equal(f.calls.length,0,'Context menu requires confirmation as well');
  await p.locator('#modalRoot [data-terminal-reconnect-all-confirm]').click();
  await p.waitForFunction(()=>!window.NetworkToolkitTerminalTabs.reconnectAllBusy());
  assert.deepEqual(f.calls,['synthetic-one','synthetic-two'],'Both eligible sessions must reconnect in tab order');
  assert.equal(f.maxInflight,1,'Reconnect all must not send concurrent transport mutations');
  assert.equal(await p.evaluate(()=>terminalSessions.size),2,'Session tabs must be retained');
  if(out)await p.locator('#terminalActiveActions').screenshot({path:path.join(out,'terminal-reconnect132-'+width+'.png')});
  report.cases.push({width,secondaryBelow:true,keyboardEscape:true,cancelSafe:true,contextMenu:true,bulkOrder:f.calls.slice(),maxInflight:f.maxInflight});
  await f.context.close();
 }
 assert.deepEqual(report.errors,[]);report.passed=true;console.log(JSON.stringify(report));
})().catch(async error=>{report.failure=error.stack||String(error);console.error(report.failure);process.exitCode=1;
 if(out&&page&&!page.isClosed())await page.screenshot({path:path.join(out,'terminal-reconnect132-failure.png')}).catch(()=>{});
}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'terminal-reconnect132-results.json'),JSON.stringify(report,null,2));await browser?.close();});
