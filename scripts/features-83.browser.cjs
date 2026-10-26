'use strict';
// Full bundled app assets with isolated API fixtures; no real shells, scans,
// updates or system files are invoked by this browser test.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,out=process.env.TOOLKIT_TEST_OUTPUT;
const report={passed:false,checks:[],layouts:[],errors:[]};let browser,page;
let saved={appUpdatePreferences:{enabled:false},terminalSidebarTab:'macros'},sessions=[],library={revision:'revision-1',macros:[{id:'core',name:'Core commands',folder:'Branch',shortcut:'Ctrl+Alt+1',steps:[{kind:'command',text:'show version'}]},{id:'named',name:'Named unfiled',folder:'unfiled',steps:[{kind:'command',text:'show clock'}]},{id:'loose',name:'Loose commands',folder:'',steps:[{kind:'command',text:'show interfaces'}]}],bindings:[{profileId:'saved-core',macroId:'core',onConnect:true}]};
let revision=1,downloadFinish,downloadProgress=null,staged=null;const requests=[];
const latest={version:'1.17.99',notes:'Browser update fixture'};
const updateStatus=()=>({currentVersion:version,supported:true,hasUpdate:true,latest,staged,progress:downloadProgress,busy:[],installing:false});
const settle=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
const check=text=>{report.checks.push(text);console.log('PASS '+text);};
async function copyGeometry(){return page.locator('#toolResult [data-value-copy-control]').evaluateAll(buttons=>buttons.flatMap(button=>{
 const b=button.getBoundingClientRect();if(!b.width||!b.height)return [];
 const parent=button.parentElement,p=parent.getBoundingClientRect(),bad=[];
 if(b.left<p.left-1||b.right>p.right+1||b.top<p.top-1||b.bottom>p.bottom+1)bad.push('outside parent');
 const overlaps=r=>r.width>0&&r.height>0&&Math.min(b.right,r.right)-Math.max(b.left,r.left)>1&&Math.min(b.bottom,r.bottom)-Math.max(b.top,r.top)>1;
 const walk=document.createTreeWalker(parent,NodeFilter.SHOW_TEXT);
 while(walk.nextNode()){const text=walk.currentNode;if(!text.textContent.trim()||button.contains(text)||text.parentElement.closest('.sr-only,[aria-hidden="true"]'))continue;const range=document.createRange();range.selectNodeContents(text);if([...range.getClientRects()].some(overlaps))bad.push('text: '+text.textContent);}
 for(const control of parent.querySelectorAll('button,a,input,select,summary'))if(control!==button&&!button.contains(control)&&overlaps(control.getBoundingClientRect()))bad.push('control');
 return bad.length?[{label:button.getAttribute('aria-label'),bad}]:[];
}));}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 const context=await browser.newContext({viewport:{width:1440,height:1000},permissions:['clipboard-read','clipboard-write']});page=await context.newPage();page.setDefaultTimeout(12000);page.on('pageerror',e=>report.errors.push(e.message));
 await page.route('https://features.toolkit.test/**',async route=>{
  const req=route.request(),p=new URL(req.url()).pathname,body=req.postData()?req.postDataJSON():null;
  if(p.startsWith('/api/')){
   requests.push({path:p,method:req.method(),body});let data={};
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',localShellAvailable:true,terminalWindowAvailable:true};
   if(p==='/api/state'){if(req.method()==='GET')data=saved;else saved=body;}
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminals'){if(req.method()==='POST'){const info={id:'local-'+(sessions.length+1),type:body.type,label:body.type==='cmd'?'Command Prompt (local)':'PowerShell (local)',state:'connected',cols:80,rows:24};sessions.push(info);data={session:info};}else data={sessions};}
   if(p==='/api/terminal-views')data={available:true,views:[],owners:{}};
   if(p==='/api/credentials')data={entries:[]};
   if(p==='/api/terminal-automation/library'){
    if(req.method()==='PUT'){if(body.revision!==library.revision)return route.fulfill({status:409,contentType:'application/json',body:JSON.stringify({error:'Library changed'})});library={...body,revision:'revision-'+(++revision)};}
    data=library;
   }
   if(p==='/api/terminal-automation/targets')data={targets:[]};
   if(p==='/api/terminal-automation/runs')data={runs:[]};
   if(p==='/api/updates/status'||p==='/api/updates/check')data=updateStatus();
   if(p==='/api/updates/download'){
    downloadProgress={phase:'download',version:latest.version,received:1048576,total:4194304};
    await new Promise(resolve=>downloadFinish=resolve);staged={version:latest.version,sha256:'a'.repeat(64),size:123};downloadProgress={phase:'ready',version:latest.version};data=updateStatus();
   }
   if(p==='/api/updates/install')data={restarting:true};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions,after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{const file=path.join(root,'web',p==='/'?'index.html':p),fallback=path.join(root,p);let data=fs.readFileSync(fs.existsSync(file)?file:fallback);if(p==='/')data=data.toString().replaceAll('__SESSION_TOKEN__','features-83').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body:data,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto('https://features.toolkit.test/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(()=>{navigate('terminal');NetworkToolkitSessionSidebar.activateTab('macros');});
 await page.locator('[data-macro-id="core"]').first().waitFor();
 assert.deepEqual(await page.locator('#terminalMacroFolder option').evaluateAll(options=>options.map(o=>o.value)),['','unfiled','folder:Branch','folder:unfiled']);
 await page.selectOption('#terminalMacroFolder','unfiled');assert.equal(await page.locator('.macro-library-item').count(),1);assert.equal(await page.locator('.macro-library-item').getAttribute('data-macro-id'),'loose');
 await page.selectOption('#terminalMacroFolder','folder:Branch');await page.locator('[data-macro-action="move-folder"]').click();await page.fill('#terminalMacroMoveFolder','Site West');await page.locator('[data-macro-action="move-folder-confirm"]').click();await page.locator('#terminalMacroMoveFolder').waitFor({state:'detached'});
 assert.equal(library.macros.find(m=>m.id==='core').folder,'Site West');assert.equal(library.bindings[0].macroId,'core');assert.equal(library.macros.find(m=>m.id==='core').shortcut,'Ctrl+Alt+1');
 await page.selectOption('#terminalMacroFolder','folder:Site West');await page.locator('[data-macro-action="new"]').click();assert.equal(await page.inputValue('#terminalMacroEditorFolder'),'Site West');await page.locator('[data-macro-action="close"]').click();
 await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(()=>{navigate('terminal');NetworkToolkitSessionSidebar.activateTab('macros');});await page.locator('#terminalMacroFolder option[value="folder:Site West"]').waitFor({state:'attached'});check('macro folder filters, move, inherited editor folder and reload preserve the library');
 for(const type of ['cmd','powershell']){
  await page.evaluate(type=>openTerminalConnect({type,host:'discarded.test',username:'discarded',credentialId:'discarded'}),type);
  assert.equal(await page.locator('#con-host,#terminalPassword,#terminalCredentialId,#con-serialPort').count(),0);await page.fill('#terminalProfileName',type+' local');await page.locator('[data-terminal-action="save-profile"]').click();await page.click('#terminalConnectButton');await page.locator('#terminalConnectForm').waitFor({state:'detached'});
  await page.waitForFunction(type=>[...terminalSessions.values()].some(e=>e.info.type===type&&e.xterm),type);
 }
 const localPosts=requests.filter(r=>r.path==='/api/terminals'&&r.method==='POST');assert.equal(localPosts.length,2);for(const post of localPosts)assert.deepEqual(Object.keys(post.body).filter(k=>['host','username','password','privateKey','credentialId','serialPort'].includes(k)),[]);
 await page.evaluate(()=>{state.terminalAppearance=NetworkToolkitTerminalAppearance.normalize({scheme:'custom',colors:{background:'#123456',foreground:'#fedcba',red:'#f02020'}});updateTerminalTheme();});
 const colors=await page.evaluate(()=>[...terminalSessions.values()].map(e=>({type:e.info.type,bg:e.xterm.options.theme.background,fg:e.xterm.options.theme.foreground,red:e.xterm.options.theme.red})));
 assert.equal(colors.length,2);for(const color of colors){assert.equal(color.bg,'#123456');assert.equal(color.fg,'#fedcba');assert.equal(color.red,'#f02020');}check('local shell choices use the common terminal and apply custom theme and ANSI colors');
 await page.evaluate(()=>{
  navigate('discover');const hosts=Array.from({length:60},(_,i)=>({address:'192.0.2.'+(i+1),hostname:i===59?'very-long-hostname-for-the-last-row-outside-old-sizing-samples.branch.example.test':'host-'+(i+1)+'.test',mac:'00:11:22:33:44:55',vendor:i===59?'A long manufacturer name that must not overlap any copy control':'Vendor',responded:true,completed:true,status:'responded',openPorts:i===0?[22,80,443,3389,5985,8080,8443]:[443],latencyMs:1.2}));
  const job={id:'scan83',tool:'discover',status:'completed',target:'192.0.2.0/26',events:[],options:{scope:'subnet'},result:{data:{scope:'subnet',hosts,total:60,started:60,completed:60,responded:60,requestedProbes:360,completedProbes:360}}};globalThis.featureScan=job;document.querySelector('#toolResult').innerHTML=renderNetworkResults(job);
 });
 await page.locator('#toolResult table.auto-fit-results').waitFor();await page.locator('.network-open-ports [data-value-copy-control]').first().waitFor();
 for(const width of [1600,820,390])for(const theme of ['light','dark']){
  await page.setViewportSize({width,height:1000});await page.evaluate(theme=>setTheme(theme),theme);await settle();
  assert.deepEqual(await copyGeometry(),[],JSON.stringify({width,theme}));
  const metrics=await page.locator('#toolResult table').first().evaluate(table=>({layout:getComputedStyle(table).tableLayout,latency:table.rows[1].cells[6].getBoundingClientRect().width,rows:table.tBodies[0].rows.length,handles:table.querySelectorAll('[data-column-resize]').length}));
  assert.equal(metrics.layout,'auto');assert.equal(metrics.rows,60);assert.equal(metrics.handles,0);assert(metrics.latency<140,JSON.stringify(metrics));
  assert.equal(await page.locator('.network-port-badge').first().getAttribute('title'),'TCP 22 open · Conventional SSH port, service not verified');
  report.layouts.push({width,theme,metrics});if(out&&(width===1600||width===390))await page.screenshot({path:path.join(out,'features83-scan-'+theme+'-'+width+'.png')});
 }
 await page.locator('.network-open-ports [data-value-copy-control]').first().click();await page.waitForFunction(async()=>await navigator.clipboard.readText()==='22, 80, 443, 3389, 5985, 8080, 8443');
 await page.evaluate(()=>{featureScan.result.data.hosts[59].hostname='updated-live-hostname-with-more-text.branch.example.test';refreshNetworkResultBody('scan83');});await settle();assert.deepEqual(await copyGeometry(),[]);check('all 60 scan rows auto fit, TCP badges copy exact ports, live changes and copy controls stay clear in six layouts');
 await page.setViewportSize({width:1280,height:1000});await page.evaluate(()=>NetworkToolkitUpdater.load());await page.locator('#appUpdateNotice [data-app-update="download"]').click();await page.locator('[data-app-update="confirm-download"]').click();
 await page.waitForFunction(()=>document.querySelector('[data-app-update-progress-host] progress')?.value===1048576);assert.match(await page.locator('[data-app-update-progress-host]').innerText(),/1.0 MiB of 4.0 MiB \(25%\)/);
 downloadProgress={phase:'download',version:latest.version,received:2097152,total:0};await page.waitForFunction(()=>document.querySelector('[data-app-update-progress-host] progress')&&!document.querySelector('[data-app-update-progress-host] progress').hasAttribute('value'));
 downloadProgress={phase:'verifying',version:latest.version};await page.waitForFunction(()=>document.querySelector('[data-app-update-progress-host]')?.textContent.includes('Verifying update'));
 if(out)await page.screenshot({path:path.join(out,'features83-update-verification.png')});assert(!requests.some(r=>r.path==='/api/updates/install'));downloadFinish();await page.locator('#appUpdateInstallError').waitFor();assert(!requests.some(r=>r.path==='/api/updates/install'));check('download shows measured bytes, unknown totals and verification before separate install consent');
 assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.failure=error.stack||String(error);console.error(report.failure);process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'features83-report.json'),JSON.stringify(report,null,2));if(browser)await browser.close();});
