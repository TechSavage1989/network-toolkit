'use strict';
// Actual application assets in main, separate-terminal and detached modes.
// Every HTTP request is intercepted: sessions, input delivery, macro execution
// and locked-vault status are synthetic fixtures. No engine/device is contacted.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT;
const report={passed:false,checks:[],errors:[],screenshots:[],scope:'Full application browser UI with mocked APIs in main, separate Terminal and detached-session modes; no native windows or live devices.'};
let browser,page;
const pauseSelector='[data-macro-action="record-pause"]',stopSelector='[data-macro-action="record-stop"]';
const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));
async function until(predicate){for(let i=0;i<200;i++){if(predicate())return;await delay(20);}throw Error('Synthetic request did not reach its route');}
async function shot(name){if(!out)return;const file='terminal-macro-'+name+'.png';await page.screenshot({path:path.join(out,file),fullPage:true});report.screenshots.push(file);}
async function pushInput(value){assert.equal(await page.evaluate(value=>enqueueTerminalInput(currentTerminal(),value),value),true);await page.waitForFunction(()=>currentTerminal().inputBytes===0&&!currentTerminal().inputSending);}
async function holdClick(selector,update){const box=await page.locator(selector).boundingBox();assert(box);await page.mouse.move(box.x+box.width/2,box.y+box.height/2);await page.mouse.down();await update();await page.mouse.up();}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
 for(const mode of ['main','terminal','detached']){
  page=await browser.newPage({viewport:{width:1280,height:900}});page.setDefaultTimeout(15000);page.on('pageerror',error=>report.errors.push(mode+': '+error.message));
  const session={id:'recording-fixture',label:'Synthetic recording terminal',type:'ssh',host:'recording.test',port:22,state:'connected',cols:80,rows:24},view={id:'view-fixture',sessionId:session.id,state:'detached',revision:0};
  const calls=[],runs=[],pending=[];let saved={appUpdatePreferences:{enabled:false},activePage:'terminal',terminalSidebarTab:'macros',terminalSidebarSide:'right',terminalSidebarWidth:300,terminalMacroConfirmation:'never'},failNext=false;
  await page.route('**/*',async route=>{
   const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method(),body=request.postData()?request.postDataJSON():null;
   if(url.origin!=='http://toolkit.local'){report.errors.push('Unexpected external request: '+url.origin);return route.abort();}
   if(p.startsWith('/api/')){
    calls.push({p,method,body});let data={};
    if(p==='/api/status')data={version,os:'windows',arch:'amd64',terminalWindowAvailable:true};
    if(p==='/api/state'){if(method==='GET')data=saved;else saved=body;}
    if(p==='/api/jobs')data=[];
    if(p==='/api/terminals')data={sessions:[session]};
    if(p==='/api/terminal-views')data={available:true,views:[],owners:{}};
    if(p==='/api/terminal-views/current')data={view,session,snapshot:{fresh:true,cols:80,rows:24,after:0,config:{type:'ssh',host:'recording.test',port:22}}};
    if(p==='/api/terminal-views/current/ready')data={view};
    if(p==='/api/credentials')data={vault:{available:true,state:'locked'},entries:[]};
    if(p==='/api/credential-vault')data={available:true,state:'locked'};
    if(p==='/api/terminal-automation/targets')data={targets:[{...session,generation:1}]};
    if(p==='/api/terminal-automation/library')data={macros:[],bindings:[],revision:'fixture'};
    if(p==='/api/terminal-automation/runs'){
     if(method==='POST'){
      if(failNext){failNext=false;return route.fulfill({status:400,contentType:'application/json',body:JSON.stringify({error:'Synthetic macro request rejected'})});}
      await new Promise(resolve=>pending.push(resolve));const run={id:'synthetic-'+(runs.length+1),name:body.macro.name,state:'running',targets:[{id:session.id,label:session.label,state:'running',step:0,total:1}]};runs.unshift(run);data={run};
     }else data={runs};
    }
    if(p.includes('/events')){await delay(80);data={sessions:[session],events:[],after:0};}
    return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
   }
   try{const file=path.join(root,'web',p==='/'?'index.html':p);let content=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));if(p==='/')content=content.toString().replaceAll('__SESSION_TOKEN__','macro-recording-fixture').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__',JSON.stringify(mode==='detached'?view:null));return route.fulfill({body:content,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
  });
  await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);if(mode==='main')await page.evaluate(()=>navigate('terminal'));
  await page.waitForFunction(()=>!!currentTerminal()?.xterm&&currentTerminal().info.state==='connected'&&terminalEntryAcceptsInput(currentTerminal()));
  await page.evaluate(async()=>{NetworkToolkitTerminalMacros.toggle(true);await NetworkToolkitTerminalMacros.refresh(true);});
  await page.locator('#terminalMacroRecord').click();await pushInput('show first\r');
  await page.evaluate(()=>{window.fixturePause=document.querySelector('[data-macro-action="record-pause"]');window.fixtureStop=document.querySelector('[data-macro-action="record-stop"]');});
  await holdClick(pauseSelector,()=>page.evaluate(async()=>{renderTerminalChrome();await NetworkToolkitTerminalMacros.refresh();NetworkToolkitTerminalMacros.sync();}));
  await page.waitForFunction(()=>document.querySelector('[data-macro-action="record-pause"]').textContent==='Resume');
  assert.equal(await page.evaluate(()=>fixturePause===document.querySelector('[data-macro-action="record-pause"]')&&fixtureStop===document.querySelector('[data-macro-action="record-stop"]')),true,'Sync/poll must preserve live actionable nodes');
  await pushInput('skip while paused\r');assert.match(await page.locator('[data-macro-recording-status]').textContent(),/Paused · 1 steps/);
  assert(calls.some(call=>call.p.endsWith('/input')&&Buffer.from(call.body.data,'base64').toString().includes('skip while paused')),'Paused recording must not block ordinary terminal input delivery');
  await page.locator(pauseSelector).focus();await page.keyboard.down('Space');await page.evaluate(()=>renderTerminalChrome());await page.keyboard.up('Space');
  await page.waitForFunction(()=>document.querySelector('[data-macro-action="record-pause"]').textContent==='Pause');await pushInput('show second\r');
  await shot(mode+'-recording');
  await holdClick(stopSelector,async()=>{await pushInput('show during pointer hold\r');await page.evaluate(async()=>{renderTerminalChrome();await NetworkToolkitTerminalMacros.refresh();});});
  await page.locator('[data-macro-edit="commands"]').waitFor();assert.equal(await page.locator('[data-macro-edit="commands"]').inputValue(),'show first\nshow second\nshow during pointer hold');
  await shot(mode+'-review');await page.locator('[data-macro-action="close"]').click();
  assert.equal(await page.locator('#terminalMacroRecording').isVisible(),false);assert(!calls.some(call=>call.method==='POST'&&call.p==='/api/terminal-automation/runs'),'Recording never runs its captured commands');

  const request=()=>page.evaluate(()=>NetworkToolkitTerminalMacros.requestRun({id:'synthetic',name:'Synthetic UI run',steps:[{kind:'command',text:'not executed'}]}));
  await request();await until(()=>pending.length===1);assert.equal(await page.locator('#terminalMacroLibraryTab').getAttribute('aria-selected'),'true');pending.shift()();await page.waitForFunction(()=>document.querySelector('#terminalMacroRunCount').textContent==='1 run');
  for(const status of ['running','paused','completed','failed']){runs[0].state=status;runs[0].targets[0].state=status;await page.evaluate(()=>NetworkToolkitTerminalMacros.refresh());assert.equal(await page.locator('#terminalMacroLibraryTab').getAttribute('aria-selected'),'true',status+' must not navigate away');}
  await request();await until(()=>pending.length===1);await page.locator('#terminalMacroRunsTab').click();pending.shift()();await page.waitForFunction(()=>document.querySelector('#terminalMacroRunCount').textContent==='2 runs');assert.equal(await page.locator('#terminalMacroRunsTab').getAttribute('aria-selected'),'true','Manual Runs choice remains selected after acceptance');
  await page.locator('#terminalMacroLibraryTab').click();failNext=true;await request();await page.waitForFunction(()=>document.body.textContent.includes('Synthetic macro request rejected'));assert.equal(await page.locator('#terminalMacroLibraryTab').getAttribute('aria-selected'),'true');
  await page.setViewportSize({width:768,height:900});await shot(mode+'-library');assert(!calls.some(call=>call.method==='POST'&&call.p==='/api/terminals'),'No connection attempts');assert(!calls.some(call=>call.p.startsWith('/api/credentials/')||call.p==='/api/terminal-automation/send'),'No secret access or command-bar execution');
  report.checks.push(mode+': live-click identity; keyboard resume; capture-only pause; retained review; Library lifecycle; manual Runs; visible error; locked-vault fixture');
  await page.close();page=null;
 }
 assert.deepEqual(report.errors,[]);report.passed=true;console.log(JSON.stringify(report));
})().catch(error=>{report.errors.push(error.stack||String(error));console.error(error.stack||String(error));process.exitCode=1;}).finally(async()=>{
 if(out){fs.writeFileSync(path.join(out,'terminal-macro-recording-report.json'),JSON.stringify(report,null,2)+'\n');if(page&&!report.passed)await page.screenshot({path:path.join(out,'terminal-macro-recording-failure.png'),fullPage:true}).catch(()=>{});}
 if(browser)await browser.close();
});
