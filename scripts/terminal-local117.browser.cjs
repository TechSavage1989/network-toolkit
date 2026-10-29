'use strict';
// Production UI with synthetic local-session APIs. No shell, UAC, device or
// real user file is launched/read by this browser fixture.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT,origin='https://local117.toolkit.test';
const report={passed:false,checks:[],errors:[],cases:[]};let browser,lastPage;
(async()=>{if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
for(const mode of ['main','terminal'])for(const width of [1280,390]){
 const context=await browser.newContext({viewport:{width,height:900}}),page=await context.newPage();lastPage=page;page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));
 let sessions=[],sequence=0,attempt='cancel',calls=[];const folder='C:\\Synthetic folder & tools 日本語';
 await page.route(origin+'/**',async route=>{try{if(await handleEditorActivityMetadata(route,origin,report))return;const req=route.request(),url=new URL(req.url()),p=url.pathname,m=req.method();if(p.startsWith('/api/')){let data={};if(m!=='GET')calls.push({path:p,method:m,body:req.postData()?req.postDataJSON():null});
 if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',localShellAvailable:true};
 else if(p==='/api/state')data={activePage:'terminal',appUpdatePreferences:{enabled:false},terminalSidebarCollapsed:true,terminalProfiles:[{id:'saved-local',name:'Synthetic administrator shell',type:'powershell',workingDirectory:folder,runAsAdministrator:true}]};
 else if(p==='/api/jobs')data=[];
 else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
 else if(p==='/api/terminals/local-directories'){assert.equal(m,'GET');data={path:folder,parent:'C:\\',drives:[{path:'C:\\',label:'Windows (C:)',available:true}],entries:[]};}
 else if(p==='/api/terminals'){
 if(m==='GET')data={sessions};else{assert.equal(m,'POST');const body=req.postDataJSON();assert(['cmd','powershell'].includes(body.type));assert.equal(body.workingDirectory,attempt==='invalid'?'C:\\missing':folder);assert.equal(typeof body.runAsAdministrator,'boolean');assert(!('password'in body));if(attempt==='invalid')return route.fulfill({status:400,contentType:'application/json',body:JSON.stringify({error:'Starting directory does not exist; choose an existing folder.'})});
 const s={id:'local117-'+(++sequence),type:body.type,label:'Synthetic '+body.type,state:'connecting',cols:80,rows:24,workingDirectory:folder,elevated:false};sessions.push(s);data={session:{...s}};if(attempt!=='pending'){const selected=attempt;setTimeout(()=>{s.state=selected==='cancel'?'error':'connected';s.error=selected==='cancel'?'Windows administrator approval was cancelled; no shell was started':'';s.elevated=selected==='success'||selected==='already-elevated';},150);}}
 }
 else if(p==='/api/terminals/events'){await new Promise(r=>setTimeout(r,60));data={sessions,events:[],next:0};}
 else if(/^\/api\/terminals\/local117-\d+\/resize$/.test(p)){assert.equal(m,'POST');}
 else if(/^\/api\/terminals\/local117-\d+\/disconnect$/.test(p)){assert.equal(m,'POST');const s=sessions.find(s=>s.id===p.split('/')[3]);if(s)s.state='closed';}
 else if(m!=='GET'&&!(p==='/api/state'&&m==='PUT'))throw Error('Unexpected mutation '+m+' '+p);
 return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
 }
 let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let bytes=fs.readFileSync(file);if(p==='/')bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','synthetic117').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__','null').replaceAll('__EDITOR_VIEW__','null');return route.fulfill({body:bytes,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.png')?'image/png':'application/octet-stream'});
 }catch(e){report.errors.push(e.message);return route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture rejected request'})});}});
 await page.goto(origin);await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('terminal'));if(mode==='terminal'){await page.locator('[data-terminal-new-type="ssh"]').first().click();await page.locator('#con-type').selectOption('cmd');}else await page.locator('[data-terminal-new-type="cmd"]').first().click();assert.equal(await page.locator('#con-type').inputValue(),'cmd');
 assert.equal(await page.locator('#con-runAsAdministrator').isChecked(),false);assert.equal(calls.filter(c=>c.path==='/api/terminals').length,0,'opening controls or saved metadata must not launch');
 await page.locator('[data-terminal-local-browse]').click();await page.locator('[data-terminal-local-use]').click();assert.equal(await page.locator('#con-workingDirectory').inputValue(),folder);assert.equal(calls.filter(c=>c.path==='/api/terminals').length,0);
 attempt='invalid';await page.locator('#con-workingDirectory').fill('C:\\missing');await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>document.querySelector('#terminalConnectError')?.textContent.includes('does not exist'));assert.equal(await page.locator('#con-workingDirectory').inputValue(),'C:\\missing');
 attempt='cancel';await page.locator('#con-workingDirectory').fill(folder);await page.locator('#con-runAsAdministrator').check();await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>document.querySelector('#terminalConnectError')?.textContent.includes('approval was cancelled'));assert.equal(await page.locator('#con-workingDirectory').inputValue(),folder);assert.equal(await page.locator('#con-runAsAdministrator').isChecked(),true);assert.equal(await page.locator('#terminalConnectButton').isEnabled(),true);
 if(out)await page.screenshot({path:path.join(out,`terminal-local117-${mode}-${width}-cancel.png`)});
 const before=calls.filter(c=>c.path==='/api/terminals').length;await page.waitForTimeout(250);assert.equal(calls.filter(c=>c.path==='/api/terminals').length,before,'cancelled approval must not retry itself');
 attempt='success';await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>!document.querySelector('#terminalConnectForm')&&currentTerminal()?.info.elevated===true);assert.match(await page.locator('#terminalStatusText').textContent(),/Administrator/);assert.match(await page.locator('[data-terminal-tab] .badge').last().textContent(),/Admin/);
 if(out)await page.screenshot({path:path.join(out,`terminal-local117-${mode}-${width}-connected.png`)});
 // A second explicit launch uses PowerShell and can be cancelled while pending.
 await page.evaluate(()=>openTerminalConnect({type:'powershell',workingDirectory:'',runAsAdministrator:false}));assert.equal(await page.locator('#con-runAsAdministrator').isChecked(),false);await page.locator('#con-workingDirectory').fill(folder);attempt='pending';await page.locator('#con-runAsAdministrator').check();await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>document.querySelector('#terminalConnectError')?.textContent.includes('Waiting for Windows'));await page.locator('#modalRoot [data-action="close-modal"]').first().click();await page.waitForFunction(()=>!terminalConnectBusy);assert(calls.some(c=>c.path==='/api/terminals/local117-3/disconnect'));
 // Actual token truth wins over an unchecked request, e.g. Toolkit already admin.
 await page.evaluate(()=>openTerminalConnect({type:'powershell',workingDirectory:'',runAsAdministrator:false}));await page.locator('#con-workingDirectory').fill(folder);attempt='already-elevated';await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>!document.querySelector('#terminalConnectForm')&&currentTerminal()?.info.type==='powershell'&&currentTerminal()?.info.elevated===true);assert.equal(calls.filter(c=>c.path==='/api/terminals').at(-1).body.runAsAdministrator,false);
 report.checks.push(`${mode} ${width}: no automatic launch; explicit directory; invalid-folder retention; simulated UAC Cancel/retry and pending Cancel; actual-token badge`);report.cases.push({mode,width,launches:calls.filter(c=>c.path==='/api/terminals'),disconnects:calls.filter(c=>c.path.endsWith('/disconnect'))});await context.close();
}
assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(async e=>{report.failure=e.stack||String(e);console.error(report.failure);process.exitCode=1;if(out&&lastPage&&!lastPage.isClosed())await lastPage.screenshot({path:path.join(out,'terminal-local117-failure.png')}).catch(()=>{});}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'terminal-local117-results.json'),JSON.stringify(report,null,2));await browser?.close();});
