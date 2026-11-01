'use strict';
// Synthetic, isolated UI regression. No real devices, credentials or system actions.
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||path.join(root,'test-results','ux-phase3');
const report={passed:false,checks:[],errors:[],assetFailures:[],screenshots:[],scope:'Synthetic production DOM only; no real device access or physical Windows'};
const hostname='SYNTHETIC-UX-PHASE3';let browser,current;
const settle=p=>p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(r)))));
async function make(repo,viewport,mode='main',dpr=1){
 const context=await browser.newContext({viewport,deviceScaleFactor:dpr,serviceWorkers:'block'}),p=await context.newPage(),version=JSON.parse(fs.readFileSync(path.join(repo,'package.json'))).version;
 p.setDefaultTimeout(15000);p.on('pageerror',e=>report.errors.push({repo:path.basename(repo),error:e.message}));p.on('response',r=>{if(r.status()>=400&&!new URL(r.url()).pathname.startsWith('/api/'))report.assetFailures.push(r.url());});
 await context.route('**/*',async route=>{if(await handleEditorActivityMetadata(route,'https://chrome109.toolkit.test',report))return;
  const request=route.request(),url=new URL(request.url()),name=url.pathname,method=request.method();assert.equal(url.origin,'https://chrome109.toolkit.test');
  if(name.startsWith('/api/')){
   assert(method==='GET'||(name==='/api/state'&&method==='PUT')||(name==='/api/packet-captures/interfaces'&&method==='POST'),'Unexpected operation: '+method+' '+name);let data={};
   if(name==='/api/status')data={version,os:'windows',arch:'amd64',system:{hostname,interfaces:[]}};
   if(name==='/api/state'&&method==='GET')data={appUpdatePreferences:{enabled:false},navigationCollapsed:false};
   if(name==='/api/system')data={hostname,interfaces:[],gateways:[],dnsServers:[]};
   if(name==='/api/jobs')data=[];
   if(name==='/api/activity')data={tools:{}};
   if(name==='/api/terminals'||name==='/api/remote-browser')data={sessions:[]};
   if(name==='/api/terminal-views')data={available:false,views:[],owners:{}};
   if(name==='/api/terminal-window/requests')data={requests:[],after:0};
   if(name==='/api/credentials')data={entries:[],vault:{available:false,state:'unavailable'}};
   if(name==='/api/credential-vault')data={available:false,state:'unavailable'};
   if(name==='/api/local-tools/ip')data={adapters:[]};
   if(name==='/api/local-tools/hosts')data={available:true,editable:true,exists:true,path:'C:\\synthetic\\hosts',content:'127.0.0.1 localhost\n',revision:'synthetic',encoding:'UTF-8',newline:'LF',warnings:[]};
   if(name==='/api/packet-captures')data={captures:[]};
   if(name==='/api/packet-captures/interfaces')data={interfaces:[],engines:[]};
   if(name.includes('/events')){await new Promise(r=>setTimeout(r,70));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{const entry=name==='/',file=path.join(repo,'web',entry?'index.html':name);let body=fs.readFileSync(fs.existsSync(file)?file:path.join(repo,name));if(entry)body=body.toString().replaceAll('__SESSION_TOKEN__','synthetic-chrome109').replaceAll('__WINDOW_MODE__',mode).replaceAll('__EDITOR_VIEW__',mode==='editor'?JSON.stringify({id:'10910910910910910910910910910910',ownerInstanceId:'10810810810810810810810810810810'}):'null').replaceAll('__TERMINAL_VIEW__','null');return route.fulfill({body,contentType:entry?'text/html':name.endsWith('.js')?'application/javascript':name.endsWith('.css')?'text/css':name.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await p.goto('https://chrome109.toolkit.test/');await p.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await p.waitForFunction(()=>Array.from(document.images).every(img=>img.complete&&img.naturalWidth>0));if(mode==='editor')assert(await p.locator('body').evaluate(el=>el.classList.contains('editor-window')),'Synthetic child must enter the real scoped editor shell');if(mode==='terminal')assert(await p.locator('body').evaluate(el=>el.classList.contains('terminal-window')));await settle(p);return{p,context};
}
const check=name=>{report.checks.push(name);console.log('PASS '+name)};
(async()=>{
 fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 const {p,context}=await make(root,{width:1280,height:800});current=p;
 const nav=await p.evaluate(()=>({ids:[...document.querySelectorAll('#navigation [data-page]')].map(b=>b.dataset.page),
  groups:[...document.querySelectorAll('#navigation .nav-label')].map(x=>x.textContent)}));
 assert.deepEqual(nav.groups,['Home','Diagnose','Connect','Transfer','Configure']);
 assert.equal(new Set(nav.ids).size,23);assert(nav.ids.includes('editor')&&nav.ids.includes('terminal')&&nav.ids.includes('sftp'));
 check('All 23 original tool routes retained under five approved groups');
 await p.locator('#uxCommandLauncher').click();
 await p.locator('[role="dialog"][aria-label="Search tools and commands"]').waitFor();
 let count=await p.locator('#uxCommandResults [role=option]').count();
 assert(count>=23,'palette must list all tools and global actions');
 await p.locator('.ux-command-input').fill('trace route');
 assert((await p.locator('#uxCommandResults').innerText()).includes('Trace route'));
 await p.keyboard.press('Enter');
 await p.waitForFunction(()=>state.activePage==='trace');
 assert.equal(await p.locator('#uxCommandOverlay').count(),0);
 check('Search, navigate and close palette using existing tool routing');
 await p.keyboard.press('F1');await p.locator('[aria-label="Keyboard shortcuts and help"][role="dialog"]').waitFor();
 assert.match(await p.locator('.ux-help-list').innerText(),/Alt\+Shift\+Left/);
 await p.keyboard.press('Escape');assert.equal(await p.locator('#uxCommandOverlay').count(),0);
 check('F1 shortcut sheet opens and Escape returns to application');
 const isolated=await p.evaluate(()=>{
  const textarea=document.createElement('textarea');textarea.className='xterm-helper-textarea';document.body.append(textarea);textarea.focus();
  const first=new KeyboardEvent('keydown',{key:'P',ctrlKey:true,shiftKey:true,bubbles:true,cancelable:true});
  const second=new KeyboardEvent('keydown',{key:'F1',bubbles:true,cancelable:true});
  textarea.dispatchEvent(first);textarea.dispatchEvent(second);
  const result={palettePrevented:first.defaultPrevented,helpPrevented:second.defaultPrevented,overlay:!!document.querySelector('#uxCommandOverlay')};
  textarea.remove();return result;
 });
 assert.deepEqual(isolated,{palettePrevented:false,helpPrevented:false,overlay:false});
 check('Focused terminal input keeps Ctrl+Shift+P and F1 for the device');
 await p.locator('#settingsButton').click();await p.locator('#modalTitle').filter({hasText:'App settings'}).waitFor();
 await p.keyboard.press('Control+Shift+P');assert.equal(await p.locator('#uxCommandOverlay').count(),0);
 await p.keyboard.press('Escape');
 check('Sensitive modal workflow cannot be interrupted by command palette');
 await p.evaluate(()=>navigate('editor'));await settle(p);
 await p.evaluate(async()=>{await NetworkToolkitFileEditor.open({id:'ux-doc-one',path:'ux-doc-one.txt',source:'Synthetic',binding:{kind:'local'},create:true,
  initialContent:'Synthetic editor text',read:async()=>({content:'Synthetic editor text'}),write:async()=>{throw Error('Save not requested');}});});
 await p.locator('[data-ed-tab-actions]').first().waitFor();
 await p.locator('[data-ed-tab-actions]').first().click();
 assert.match(await p.locator('.nt-editor-tab-menu').innerText(),/Move one tab left/);
 assert.match(await p.locator('.nt-editor-tab-menu').innerText(),/Move to separate window/);
 await p.keyboard.press('Escape');
 const doc=await p.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].find(d=>d.id==='ux-doc-one')?.content);
 assert.equal(doc,'Synthetic editor text');
 check('Editor tab has visible actions without changing unsaved document bytes');
 await p.evaluate(()=>navigate('terminal'));await settle(p);
 // The real page contains a visible Help button inside Terminal tools.
 await p.evaluate(()=>{if(typeof setTerminalToolsOpen==='function')setTerminalToolsOpen(true,false);});
 const help=p.locator('#uxTerminalHelpLauncher');
 if(await help.count()){await help.click();await p.locator('[aria-label="Keyboard shortcuts and help"][role="dialog"]').waitFor();await p.keyboard.press('Escape');check('Terminal Tools exposes help without a keyboard shortcut');}
 if(report.errors.length||report.assetFailures.length)throw Error(JSON.stringify(report));
 await p.screenshot({path:path.join(out,'phase3-navigation.png')});report.screenshots.push('phase3-navigation.png');
 await context.close();
 report.passed=true;
})().catch(async error=>{report.failure=error.stack||String(error);process.exitCode=1;}).finally(async()=>{fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'ux-phase3-results.json'),JSON.stringify(report,null,2));await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks,errors:report.errors,assets:report.assetFailures,failure:report.failure}));});
