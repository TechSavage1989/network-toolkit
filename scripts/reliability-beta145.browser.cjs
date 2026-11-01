'use strict';
// Full application browser lifecycle, synthetic API only. No real network/device data.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const root=path.resolve(process.env.TOOLKIT_TEST_ROOT||path.join(__dirname,'..')),out=process.env.TOOLKIT_TEST_OUTPUT,origin='https://reliability145.toolkit.test';
const version=/const Version = "([\d.]+)"/.exec(fs.readFileSync(path.join(root,'internal/model/model.go'),'utf8'))[1];
const report={version,passed:false,errors:[],navigation:[],metrics:[],scope:'Synthetic hosted Chromium lifecycle and CPU/heap observations, not live traffic or multi-hour Windows soak'};
let browser,context,page,interrupted=false,delayList=false,releaseList;
const samples=Array.from({length:300},(_,i)=>({sequence:i+1,time:new Date(Date.UTC(2026,0,1)+i*1000).toISOString(),success:true,rttMs:i%17+1,address:'192.0.2.145'}));
const jobs=['ping','trace','dns','tcp','discover','monitor'].map((tool,i)=>({id:'synthetic-'+tool,tool,target:'192.0.2.145',status:'running',started:'2026-01-01T00:00:00Z',options:{},events:[],result:{summary:'Synthetic '+tool+' workload',data:['ping','monitor'].includes(tool)?{samples,statistics:{sent:300,received:300,lost:0,minMs:1,maxMs:17,avgMs:9}}:{}}}));
(async()=>{try{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true});context=await browser.newContext({viewport:{width:1440,height:1000},serviceWorkers:'block'});page=await context.newPage();page.setDefaultTimeout(20000);
 page.on('pageerror',e=>report.errors.push(e.message));
 await context.route('**/*',async route=>{try{if(await handleEditorActivityMetadata(route,origin,report))return;const req=route.request(),url=new URL(req.url()),p=url.pathname,method=req.method();assert.equal(url.origin,origin,'No external requests');
 if(p.startsWith('/api/')){assert(method==='GET'||(p==='/api/state'&&method==='PUT')||(p==='/api/packet-captures/interfaces'&&method==='POST')||(p==='/api/jobs/synthetic-ping/cancel'&&method==='POST'),'Unexpected mutation '+method+' '+p);let data={};
 if(p==='/api/status')data={version,os:'windows',arch:'amd64',system:{hostname:'SYNTHETIC-145',interfaces:[]}};
 if(p==='/api/state'&&method==='GET')data={appUpdatePreferences:{enabled:false},navigationCollapsed:false};
 if(p==='/api/system')data={hostname:'SYNTHETIC-145',interfaces:[],gateways:[],dnsServers:[]};
 if(p==='/api/jobs'){if(delayList)await new Promise(r=>{releaseList=r});if(interrupted)return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Synthetic interruption'})});data=jobs;}
 else if(p==='/api/jobs/synthetic-ping/cancel'){jobs[0].status='cancelled';data={ok:true};}
 else if(p.startsWith('/api/jobs/'))data=jobs.find(j=>p==='/api/jobs/'+j.id)||{};
 if(p==='/api/activity')data={tools:{}};
 if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};
 if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
 if(p==='/api/credentials')data={entries:[],vault:{available:false,state:'unavailable'}};
 if(p==='/api/credential-vault')data={available:false,state:'unavailable'};
 if(p==='/api/local-tools/ip')data={adapters:[]};
 if(p==='/api/local-tools/hosts')data={available:true,editable:true,exists:true,path:'C:\\synthetic\\hosts',content:'127.0.0.1 localhost\n',revision:'synthetic',encoding:'UTF-8',newline:'LF',warnings:[]};
 if(p==='/api/packet-captures')data={captures:[]};
 if(p==='/api/packet-captures/interfaces')data={interfaces:[],engines:[]};
 if(p.includes('/events')){await new Promise(r=>setTimeout(r,100));data={events:[],sessions:[],after:0};}
 return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
 }
 const entry=p==='/',file=path.join(root,'web',entry?'index.html':p);let body=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));if(entry)body=body.toString().replaceAll('__SESSION_TOKEN__','synthetic-145').replaceAll('__WINDOW_MODE__','main').replaceAll('__EDITOR_VIEW__','null').replaceAll('__TERMINAL_VIEW__','null');return route.fulfill({body,contentType:entry?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
 }catch(e){report.errors.push(e.message);return route.fulfill({status:500,body:'Synthetic fixture rejected request'}).catch(()=>{});}});
 await page.goto(origin);await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online&&!polling);await page.evaluate(()=>clearTimeout(nextPoll));
 const cdp=await context.newCDPSession(page);await cdp.send('Performance.enable');
 const metrics=async label=>{const value=await cdp.send('Performance.getMetrics');report.metrics.push({label,...Object.fromEntries(value.metrics.filter(x=>['TaskDuration','JSHeapUsedSize','Nodes','Documents','JSEventListeners'].includes(x.name)).map(x=>[x.name,x.value]))});};
 await metrics('before');
 for(let cycle=0;cycle<5;cycle++)for(const tool of ['overview','ping','trace','dns','tcp','discover','monitor','capture','sftp','ftp','tftp','editor','terminal','reports','ipconfig','hosts']){
 await page.evaluate(async tool=>{navigate(tool);await pollJobs();await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));},tool);
 assert.equal(await page.evaluate(()=>state.activePage),tool);report.navigation.push({cycle,tool});
 }
 await page.evaluate(()=>{appSettings();closeModal();});assert.equal(await page.locator('#modalRoot').textContent(),'');
 await page.evaluate(()=>cancelJob('synthetic-ping'));assert.equal(jobs[0].status,'cancelled');
 interrupted=true;await page.evaluate(()=>pollJobs());assert.equal(await page.evaluate(()=>online),false);assert.equal(await page.evaluate(()=>jobs.length),6);
 interrupted=false;await page.evaluate(()=>pollJobs());assert.equal(await page.evaluate(()=>online),true);
 await metrics('after-navigation');
 // Separate uncollected temporary DOM from retained objects; this is an observation,
 // not a universal memory limit or a production garbage-collection change.
 await cdp.send('HeapProfiler.collectGarbage');await metrics('after-navigation-gc');
 delayList=true;const pending=page.evaluate(()=>pollJobs());const requestDeadline=Date.now()+20000;while(!releaseList&&Date.now()<requestDeadline)await new Promise(r=>setTimeout(r,10));assert(releaseList,'Expected held diagnostic request');await page.evaluate(()=>{shutDown=true;online=false;updateEngine();});releaseList();await pending;const revived=await page.evaluate(()=>online),expectHistoricalRace=process.env.TOOLKIT_EXPECT_SHUTDOWN_RACE==='1';assert.equal(revived,expectHistoricalRace,'Delayed reply must not revive candidate; exact historical control must reproduce the race');report.shutdownLateResponseIgnored=!revived;report.expectedHistoricalRace=expectHistoricalRace;
 assert.deepEqual(report.errors,[]);report.passed=true;
 }catch(e){report.failure=e.stack;process.exitCode=1;}finally{if(out&&page)await page.screenshot({path:path.join(out,'beta145-final.png')}).catch(()=>{});await context?.close();await browser?.close();if(out)fs.writeFileSync(path.join(out,'reliability145.json'),JSON.stringify(report,null,2));console.log(JSON.stringify(report,null,2));}})();
