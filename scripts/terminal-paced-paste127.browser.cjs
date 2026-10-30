'use strict';
// Actual bundled application and xterm; all HTTP responses and SSH sessions are
// synthetic. This verifies UI dispatch and rendering, not a real device's echo.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,out=process.env.TOOLKIT_TEST_OUTPUT;
const report={passed:false,checks:[],screenshots:[],errors:[],scope:'Actual application/xterm in Chromium with intercepted API responses; no live SSH connection or native Windows claim.'};
async function until(predicate){for(let i=0;i<200;i++){if(predicate())return;await new Promise(resolve=>setTimeout(resolve,20));}throw Error('Synthetic request did not reach route');}
const text='  description {{host}}\r\n\r\n  show version\r\n';
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
 try{for(const mode of ['main','terminal','detached']){
  const page=await browser.newPage({viewport:{width:1440,height:1000}});page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(mode+': '+e.message));
  const session={id:'paced-fixture',label:'Synthetic lab switch',type:'ssh',host:'switch.test',port:22,state:'connected',cols:100,rows:28},view={id:'paced-view',sessionId:'paced-fixture',state:'detached',revision:0};
  const calls=[],runs=[];let response='accept',release=null,saved={appUpdatePreferences:{enabled:false},terminalInteraction:{pasteReview:true,copyOnSelect:false},activePage:'terminal'},revision=0;
  const owners=()=>({[session.id]:{viewId:mode==='detached'?view.id:'',revision,ready:true}});
  await page.route('**/*',async route=>{
   const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method(),body=request.postData()?request.postDataJSON():null;
   if(url.origin!=='http://toolkit.local')return route.abort();
   if(p.startsWith('/api/')){
    calls.push({p,method,body,headers:request.headers()});let data={};
    if(p==='/api/status')data={version,os:'windows',arch:'amd64',terminalWindowAvailable:true};
    if(p==='/api/state'){if(method==='GET')data=saved;else saved=body;}
    if(p==='/api/jobs')data=[];
    if(p==='/api/terminals')data={sessions:[session]};
    if(p==='/api/terminal-views')data={available:true,views:mode==='detached'?[view]:[],owners:owners()};
    if(p==='/api/terminal-views/current')data={view,session,snapshot:{fresh:true,cols:100,rows:28,after:0,config:{type:'ssh',host:'switch.test',port:22}}};
    if(p==='/api/terminal-views/current/ready')data={view};
    if(p==='/api/terminal-automation/targets')data={targets:[{...session,generation:7}]};
    if(p==='/api/terminal-automation/library')data={macros:[],bindings:[],revision:'fixture'};
    if(p==='/api/terminal-automation/runs')data={runs};
    if(p==='/api/terminal-automation/paste'){
     if(response==='hold')await new Promise(resolve=>release=resolve);
     if(response==='uncertain')return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Synthetic start response unavailable'})});
     const run={id:'paced-'+(runs.length+1),name:'Paced paste',state:response==='failed'?'failed':'running',targets:[{id:session.id,label:session.label,state:response==='failed'?'failed':'running',error:response==='failed'?'Connection changed before start':'',step:1,total:3}]};runs.unshift(run);data={run};
    }
    if(p.endsWith('/control')){const run=runs.find(r=>p.includes('/'+r.id+'/'));run.state=body.action==='stop'?'cancelled':body.action==='pause'?'paused':'running';run.targets[0].state=run.state;data={run};}
    if(p.includes('/events')){await new Promise(r=>setTimeout(r,80));data={sessions:[session],events:[],next:0};}
    return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
   }
   try{const file=path.join(root,'web',p==='/'?'index.html':p);let content=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));if(p==='/')content=content.toString().replaceAll('__SESSION_TOKEN__','paced-fixture').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__',JSON.stringify(mode==='detached'?view:null));return route.fulfill({body:content,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
  });
  await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);if(mode==='main')await page.evaluate(()=>navigate('terminal'));
  await page.waitForFunction(()=>!!currentTerminal()?.xterm&&terminalEntryAcceptsInput(currentTerminal()));
  await page.evaluate(async()=>{await NetworkToolkitTerminalMacros.refresh(true);});
  const paced=()=>calls.filter(c=>c.p==='/api/terminal-automation/paste'),inputs=()=>calls.filter(c=>/\/input$/.test(c.p));
  async function ready(){await page.evaluate(async()=>{const e=currentTerminal();e.xterm.reset();e.xterm.clearSelection();await new Promise(resolve=>e.xterm.write('lab-switch(config)# ',resolve));});}
  async function open(value=text){await ready();await page.evaluate(value=>NetworkToolkitTerminalInteraction.pasteText(currentTerminal(),value),value);await page.locator('#terminalPasteForm').waitFor();}
  async function close(){await page.locator('#modalRoot [data-action="close-modal"]').click();await page.locator('#terminalPasteForm').waitFor({state:'detached'});}
  async function screenshot(label){if(!out)return;const file='terminal-paced127-'+mode+'-'+label+'.png';await page.screenshot({path:path.join(out,file),fullPage:true});report.screenshots.push(file);}
  await open();assert.equal(await page.locator('#terminalPasteMode').inputValue(),'paced');assert.equal(await page.locator('#terminalPasteText').inputValue(),text.replaceAll('\r',''));await close();assert.equal(paced().length,0);assert.equal(inputs().length,0);report.checks.push(mode+': cancel sends nothing');
  for(const width of [1440,820,390]){await page.setViewportSize({width,height:1000});await open();assert.equal(await page.locator('#terminalPasteMode').inputValue(),'paced');const metrics=await page.locator('#terminalPasteForm').evaluate(el=>({width:el.getBoundingClientRect().width,scroll:el.scrollWidth,client:el.clientWidth}));assert(metrics.scroll<=metrics.client+1,JSON.stringify(metrics));await screenshot('review-'+width);await close();}
  await page.setViewportSize({width:1440,height:1000});await open();response='hold';await page.locator('#terminalPasteConfirm').click();await page.waitForFunction(()=>document.querySelector('#terminalPasteConfirm').disabled);assert.equal(await page.locator('#terminalPasteText').getAttribute('readonly'),'');await page.evaluate(()=>submitTerminalPaste(document.querySelector('#terminalPasteForm')));await until(()=>!!release);assert.equal(paced().length,1);assert.equal(inputs().length,0);assert.deepEqual(paced()[0].body,{target:{id:session.id,generation:7},text});assert.equal(paced()[0].headers['x-terminal-revision'],'0');response='accept';assert(release);release();await page.locator('#terminalPasteForm').waitFor({state:'detached'});await page.locator('[data-macro-action="control"][data-control="pause"]').click();await page.locator('[data-macro-action="control"][data-control="resume"]').waitFor();await page.locator('[data-macro-action="control"][data-control="stop"]').click();await page.locator('[data-macro-action="control"][data-control="stop"]').waitFor({state:'detached'});report.checks.push(mode+': exact snapshot, one start, literal bytes, Pause and Stop');
  await open('show one\nshow two\n');await page.locator('#terminalPasteMode').selectOption('raw');await page.locator('#terminalPasteConfirm').click();await page.waitForFunction(()=>!currentTerminal().inputBytes&&!currentTerminal().inputSending);assert.equal(Buffer.from(inputs().at(-1).body.data,'base64').toString(),'show one\rshow two\r');assert.equal(paced().length,1);report.checks.push(mode+': raw path unchanged');
  await ready();await page.evaluate(async()=>{await new Promise(resolve=>currentTerminal().xterm.write('\x1b[?2004h',resolve));NetworkToolkitTerminalInteraction.pasteText(currentTerminal(),'alpha\nbeta\n');});await page.locator('#terminalPasteForm').waitFor();assert.equal(await page.locator('#terminalPasteMode').inputValue(),'raw');await page.locator('#terminalPasteConfirm').click();await page.waitForFunction(()=>!currentTerminal().inputBytes&&!currentTerminal().inputSending);assert.equal(Buffer.from(inputs().at(-1).body.data,'base64').toString(),'\x1b[200~alpha\rbeta\r\x1b[201~');report.checks.push(mode+': bracketed paste preserved');
  await open('show one\nunfinished');assert.equal(await page.locator('#terminalPasteMode').inputValue(),'raw');await page.locator('#terminalPasteMode').selectOption('paced');assert.equal(await page.locator('#terminalPasteConfirm').isEnabled(),false);assert.match(await page.locator('#terminalPasteModeHint').innerText(),/line break/);await close();
  await open();revision++;await page.evaluate(revision=>{const id=currentTerminal().info.id;terminalViewRegistry.owners[id]={...terminalViewRegistry.owners[id],revision};NetworkToolkitTerminalInteraction.sync();},revision);assert.equal(await page.locator('#terminalPasteConfirm').isEnabled(),false);await close();assert.equal(paced().length,1);report.checks.push(mode+': unterminated commands and ownership changes fail closed');
  await open();response='uncertain';await page.locator('#terminalPasteConfirm').click();await page.waitForFunction(()=>document.querySelector('#terminalPasteStatus').textContent.includes('not confirmed'));assert.equal(await page.locator('#terminalPasteConfirm').isEnabled(),false);await page.evaluate(()=>submitTerminalPaste(document.querySelector('#terminalPasteForm')));assert.equal(paced().length,2);await screenshot('uncertain');await close();await open();assert.equal(await page.locator('#terminalPasteConfirm').isEnabled(),false);assert.equal(await page.locator('#terminalPasteResolve').isVisible(),true);await page.locator('#terminalPasteResolve').click();assert.equal(await page.locator('#terminalPasteConfirm').isEnabled(),true);assert.equal(paced().length,2);await close();report.checks.push(mode+': uncertain response survives reopen; acknowledgement sends nothing');
  response='hold';release=null;await open();await page.locator('#terminalPasteConfirm').click();await until(()=>!!release);assert.equal(paced().length,3);assert.equal(await page.locator('#terminalPasteDismiss').innerText(),'Close');await close();await open();assert.equal(await page.locator('#terminalPasteConfirm').isEnabled(),false);await page.evaluate(()=>submitTerminalPaste(document.querySelector('#terminalPasteForm')));assert.equal(paced().length,3);response='accept';release();await page.waitForFunction(()=>!document.querySelector('#terminalPasteConfirm').disabled);await close();await page.locator('[data-macro-action="control"][data-control="stop"]').click();await page.locator('[data-macro-action="control"][data-control="stop"]').waitFor({state:'detached'});report.checks.push(mode+': pending start survives reopen and releases without duplicate');
  response='failed';await open();await page.locator('#terminalPasteConfirm').click();await page.waitForFunction(()=>document.querySelector('#terminalPasteModeHint').textContent.includes('Connection changed'));assert.equal(await page.locator('#terminalPasteText').inputValue(),text.replaceAll('\r',''));assert.equal(await page.locator('#terminalPasteConfirm').isEnabled(),true);assert.equal(paced().length,4);await close();report.checks.push(mode+': failed run retains text and reports rejection');
  assert.equal(saved.terminalInteraction.pasteReview,true);await page.close();
 }
 assert.deepEqual(report.errors,[]);report.passed=true;
 }finally{if(out)fs.writeFileSync(path.join(out,'terminal-paced127-results.json'),JSON.stringify(report,null,2));await browser.close();}
 console.log(JSON.stringify(report));
})().catch(error=>{console.error(error);process.exitCode=1;});
