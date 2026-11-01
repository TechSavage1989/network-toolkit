'use strict';
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||path.join(root,'test-results','ux-audit');
const report={screenshots:[],errors:[],assetFailures:[],checks:[],scope:'Production UI rendered against synthetic local API fixtures only; no live devices, local file changes or native Windows host'};
const hostname='SYNTHETIC-AUDIT-WORKSTATION';let browser,current;
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
async function capture(page,name){
 const pathName=path.join(out,name+'.png');await page.screenshot({path:pathName,fullPage:false});
 report.screenshots.push({name,viewport:page.viewportSize(),activePage:await page.evaluate(()=>state.activePage)});
}
(async()=>{
 fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 for(const viewport of [{width:1280,height:800},{width:1920,height:1080}]){
  const item=await make(root,viewport),page=item.p;current=page;
  const ids=await page.evaluate(()=>Array.from(document.querySelectorAll('#navigation [data-page]')).map(x=>x.dataset.page));
  for(const theme of ['light','dark']){
   await page.evaluate(t=>setTheme(t),theme);await settle(page);
   for(const id of ids){
    await page.evaluate(id=>navigate(id),id);await settle(page);
    await capture(page,'main-'+viewport.width+'x'+viewport.height+'-'+theme+'-'+id);
   }
   await page.evaluate(()=>navigate('overview'));await settle(page);
   await page.locator('#settingsButton').click();
   await page.locator('#modalTitle').filter({hasText:'App settings'}).waitFor();
   await capture(page,'dialog-'+viewport.width+'x'+viewport.height+'-'+theme+'-app-settings');
   await page.keyboard.press('Escape');await settle(page);
   await page.locator('#themeButton').waitFor();
   await capture(page,'main-'+viewport.width+'x'+viewport.height+'-'+theme+'-overview-return');
  }
  await item.context.close();
  for(const mode of ['terminal','editor']){
    const child=await make(root,viewport,mode);current=child.p;
    for(const theme of ['light','dark']){
      await child.p.evaluate(t=>setTheme(t),theme);await settle(child.p);
      await capture(child.p,'detached-'+viewport.width+'x'+viewport.height+'-'+theme+'-'+mode);
    }
    await child.context.close();
  }
 }
 if(report.errors.length||report.assetFailures.length)throw Error(JSON.stringify({errors:report.errors,assetFailures:report.assetFailures}));
 report.passed=true;
})().catch(async err=>{report.failure=err.stack||String(err);process.exitCode=1;}).finally(async()=>{fs.writeFileSync(path.join(out,'ux-audit-results.json'),JSON.stringify(report,null,2));await browser?.close();console.log(JSON.stringify({passed:report.passed,count:report.screenshots.length,errors:report.errors,assetFailures:report.assetFailures,failure:report.failure}));});
