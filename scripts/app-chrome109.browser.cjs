'use strict';
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
// Full published-108 assets and the candidate are rendered at identical sizes.
// Synthetic local API only: no devices, capture start, files or native launches.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),baseline=process.env.NTK_LAYOUT_BASELINE_DIR,out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT;
const report={passed:false,navigation:[],baselineVersion:'1.17.108',checks:[],geometry:[],errors:[],assetFailures:[],screenshots:[],scope:'Actual production DOM/CSS in synthetic browser contexts. DPR2 is not native Windows DPI certification. No live devices or native capabilities exercised.'};
const hostname='SYNTHETIC-WORKSTATION-WITH-A-LONG-NAME';let browser,current;
const pass=x=>report.checks.push(x),settle=p=>p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(r)))));
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
async function geometry(p){return p.evaluate(()=>{
 const rect=e=>{const r=e?.getBoundingClientRect();return r?{x:r.x,y:r.y,width:r.width,height:r.height,bottom:r.bottom,right:r.right}:null;};
 const workspace=document.querySelector('#workspaceScroll');
 const target=[document.querySelector('body.terminal-page #terminalStage'),document.querySelector('#pageContent'),document.querySelector('#toolConfiguration')].find(element=>{const r=element?.getBoundingClientRect();return r&&r.width>0&&r.height>0&&getComputedStyle(element).visibility!=='hidden';});
 if(!target)throw new Error('No visible tool pane is available for geometry measurement');
 const w=rect(workspace),t=rect(target),top=rect(document.querySelector('.topbar'));
 return{workspace:w,target:t,targetID:target.id,availableHeight:w.bottom-t.y,topbar:top,documentWidth:document.documentElement.scrollWidth,windowWidth:innerWidth,main:rect(document.querySelector('.main'))};
});}
async function shot(p,name){if(!out)return;const file='app-chrome109-'+name+'.png';await p.screenshot({path:path.join(out,file),fullPage:true});report.screenshots.push(file);}
async function safeNavigation(p){
 assert.equal(await p.locator('#mobileMenu').count(),1);const placement=await p.locator('#mobileMenu').evaluate(e=>({page:state.activePage,parent:e.parentElement.id||e.parentElement.className,classes:document.body.className}));const box=await p.locator('#mobileMenu').boundingBox();report.navigation.push({...placement,box,viewport:p.viewportSize()});assert(box&&box.width>0&&box.height>0&&box.x>=0&&box.y>=0&&box.x+box.width<=p.viewportSize().width+1&&box.y+box.height<=p.viewportSize().height+1,'Navigation opener must be visible inside the actual tool: '+JSON.stringify({placement,box}));
 await p.locator('#mobileMenu').click();await p.waitForFunction(()=>document.querySelector('#sidebar').classList.contains('open'));await p.keyboard.press('Escape');await p.waitForFunction(()=>!document.querySelector('#sidebar').classList.contains('open')&&document.querySelector('#sidebar').getBoundingClientRect().right<=1);await settle(p);assert(await p.locator('#mobileMenu').evaluate(e=>document.activeElement===e));
}
(async()=>{
 assert(baseline&&fs.existsSync(path.join(baseline,'web/index.html')),'NTK_LAYOUT_BASELINE_DIR must contain the exact published108 checkout');assert.equal(JSON.parse(fs.readFileSync(path.join(baseline,'package.json'))).version,'1.17.108');if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
 for(const viewport of [{width:1440,height:1000},{width:390,height:900}]){
  const old=await make(baseline,viewport),now=await make(root,viewport);current=now.p;
  const pages=await current.locator('#navigation [data-page]').evaluateAll(items=>items.map(x=>x.dataset.page));assert(pages.includes('terminal')&&pages.includes('editor')&&pages.includes('capture'));
  for(const theme of ['dark','light']){
   await Promise.all([old.p,now.p].map(p=>p.evaluate(theme=>setTheme(theme),theme)));
   for(const tool of pages){
    await Promise.all([old.p,now.p].map(async p=>{await p.evaluate(tool=>navigate(tool),tool);await settle(p);}));
    const before=await geometry(old.p),after=await geometry(now.p),gain=after.availableHeight-before.availableHeight;report.geometry.push({viewport,theme,tool,before,after,gain});
    assert.equal(after.targetID,before.targetID,'Compare the same visible pane in108 and109');assert.equal(after.topbar,null,'No replacement topbar layout box');assert(after.documentWidth<=viewport.width+1,JSON.stringify({tool,viewport,theme,overflow:after.documentWidth}));assert(gain>0,JSON.stringify({tool,viewport,theme,before,after,gain}));
    if(viewport.width<=1100)await safeNavigation(now.p);
    if(['monitor','terminal','editor'].includes(tool))await shot(now.p,tool+'-'+viewport.width+'-'+theme);
   }
   for(const tool of ['sftp','ftp','tftp'])for(const mode of ['server','client']){
    await Promise.all([old.p,now.p].map(async p=>{await p.evaluate(({tool,mode})=>{state.transferModes={...state.transferModes,[tool]:mode};navigate(tool);},{tool,mode});await settle(p);}));
    const before=await geometry(old.p),after=await geometry(now.p);assert.equal(after.targetID,before.targetID);assert(after.availableHeight>before.availableHeight,JSON.stringify({tool,mode,viewport,theme,before,after}));report.geometry.push({viewport,theme,tool,mode,before,after,gain:after.availableHeight-before.availableHeight});if(viewport.width<=1100)await safeNavigation(now.p);
    if(viewport.width===390&&theme==='dark'&&tool==='sftp')await shot(now.p,'sftp-'+mode+'-390-dark');
   }
  }
  pass('Every main tool gains measured usable height versus actual108 at '+viewport.width+'px in light and dark; compact navigation survives tool changes');
  await now.p.evaluate(()=>navigate('overview'));if(viewport.width<=1100)await now.p.locator('#mobileMenu').click();await now.p.waitForFunction(()=>document.querySelector('#sidebar').getBoundingClientRect().left>=0);
  assert.equal(await now.p.locator('#hostTop').getAttribute('title'),hostname);assert.equal(await now.p.locator('#themeButton').count(),1);assert.equal(await now.p.locator('#settingsButton').count(),1);
  const beforeTheme=await now.p.evaluate(()=>state.theme);await now.p.locator('#themeButton').focus();await now.p.keyboard.press('Enter');assert.notEqual(await now.p.evaluate(()=>state.theme),beforeTheme);
  await now.p.locator('#settingsButton').focus();await now.p.keyboard.press('Enter');await now.p.locator('#modalTitle').filter({hasText:'App settings'}).waitFor();await now.p.keyboard.press('Escape');
  await now.p.evaluate(()=>{jobs=[{id:'synthetic-job',tool:'ping',status:'running',target:'192.0.2.109',events:[]}];updateEngine();});assert.equal(await now.p.locator('#runningCount').getAttribute('aria-label'),'1 running diagnostic jobs');
  if(viewport.width<=1100&&await now.p.locator('#sidebar').getAttribute('aria-hidden')==='true')await now.p.locator('#mobileMenu').click();await now.p.waitForFunction(()=>document.querySelector('#sidebar').getBoundingClientRect().left>=0);await settle(now.p);
  await shot(now.p,'navigation-footer-'+viewport.width);await now.p.evaluate(()=>{jobs=[];updateEngine();});assert.equal(await now.p.locator('#runningCount').isVisible(),false);
  if(viewport.width>1100){await now.p.evaluate(()=>{state.navigationCollapsed=true;NetworkToolkitNavigation.sync();});await now.p.locator('#themeButton').focus();assert(await now.p.locator('#themeButton').evaluate(e=>document.activeElement===e));await now.p.locator('#settingsButton').focus();assert(await now.p.locator('#settingsButton').evaluate(e=>document.activeElement===e));assert.equal(await now.p.locator('#sidebar .brand-mark').getAttribute('aria-label'),'Network Toolkit. System: '+hostname);await settle(now.p);const a=await now.p.locator('#themeButton').boundingBox(),b=await now.p.locator('#settingsButton').boundingBox(),rail=await now.p.locator('#sidebar').boundingBox();assert(a.width>=24&&a.height>=24&&b.width>=24&&b.height>=24&&Math.abs(a.y-b.y)<=1,'Collapsed utility targets retain usable size on one row');assert(a.x>=rail.x&&b.x+b.width<=rail.x+rail.width,'Collapsed controls remain inside the rail');await shot(now.p,'collapsed-rail');}
  await old.context.close();await now.context.close();
 }
 pass('System identity, engine footer, keyboard theme/settings and live running counts remain accessible without duplicated controls');
 const shortBefore=await make(baseline,{width:820,height:540}),short=await make(root,{width:820,height:540});current=short.p;for(const item of [{tool:'monitor'},{tool:'terminal'},{tool:'editor'},{tool:'capture'},...['sftp','ftp','tftp'].flatMap(tool=>['server','client'].map(mode=>({tool,mode})))]){await Promise.all([shortBefore.p,current].map(async p=>{await p.evaluate(({tool,mode})=>{if(mode)state.transferModes={...state.transferModes,[tool]:mode};navigate(tool);},item);await settle(p);}));const before=await geometry(shortBefore.p),after=await geometry(current);assert.equal(after.targetID,before.targetID);assert(after.availableHeight>before.availableHeight,JSON.stringify({short:true,...item,before,after}));report.geometry.push({short:true,...item,before,after});await safeNavigation(current);}await shot(current,'short-window');await shortBefore.context.close();await short.context.close();
 pass('Short windows keep the navigation opener in existing visible tool controls, including Terminal');
 for(const mode of ['terminal','editor']){
  const old=await make(baseline,{width:1024,height:720},mode),now=await make(root,{width:1024,height:720},mode);current=now.p;const before=await geometry(old.p),after=await geometry(now.p);assert(after.availableHeight>=before.availableHeight*.95,JSON.stringify({mode,before,after}));assert(after.target.height>=before.target.height*.95,JSON.stringify({mode,before,after}));assert.equal(await now.p.locator('#settingsButton').isVisible(),false,'Scoped windows must not expose broader global settings');report.geometry.push({mode,before,after});await shot(now.p,'detached-'+mode);await old.context.close();await now.context.close();
 }
 pass('Independent terminal/editor shell modes do not lose workspace height or acquire broader settings controls');
 const scaleBefore=await make(baseline,{width:1440,height:1000},'main',2),scale=await make(root,{width:1440,height:1000},'main',2);current=scale.p;for(const p of [scaleBefore.p,current]){await p.evaluate(()=>{state.navigationCollapsed=true;NetworkToolkitNavigation.sync();navigate('monitor');});await settle(p);}const beforeScale=await geometry(scaleBefore.p),afterScale=await geometry(current);assert(afterScale.availableHeight>beforeScale.availableHeight);report.geometry.push({dpr:2,collapsed:true,before:beforeScale,after:afterScale});assert.equal(await current.evaluate(()=>devicePixelRatio),2);await shot(current,'rail-dpr2');await scaleBefore.context.close();await scale.context.close();
 assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetFailures,[]);report.passed=true;
})().catch(async error=>{report.failure=error.stack||String(error);console.error(report.failure);if(current&&!current.isClosed())await shot(current,'failure').catch(()=>{});process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'app-chrome109-results.json'),JSON.stringify(report,null,2)+'\n');await browser?.close();});
