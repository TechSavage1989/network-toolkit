'use strict';
// Real bundled UI with isolated local diagnostic responses; no network scans or installation.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT,report={passed:false,checks:[],layouts:[],errors:[],assetFailures:[]};
let browser,page,serial=0,postDelay=0,nextStatus='completed',rejectNext=false;const posted=[],serverJobs=[];
const check=name=>{report.checks.push(name);console.log('PASS '+name);};
const settle=()=>page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
const count=tool=>posted.filter(p=>p.tool===tool).length;
const tab=tool=>page.locator('.local-tool-header [data-local="'+tool+'"]');
const complete=()=>{for(const j of serverJobs)if(j.status==='running'){j.status='completed';j.finished=new Date().toISOString();j.result={summary:{entries:0},data:{rows:[]}};}};
async function waitCount(tool,value){await page.waitForFunction(({tool,value})=>globalThis.__local89Posts?.filter(p=>p.tool===tool).length===value,{tool,value});}
async function metrics(tool){return page.evaluate(tool=>{const box=s=>{const r=document.querySelector(s).getBoundingClientRect();return {top:r.top,left:r.left,width:r.width,bottom:r.bottom};};return {tabs:box('.local-tool-header .tabs'),header:box('.local-tool-header'),content:box(tool==='ipconfig'?'#ipConfiguration':'#toolResult'),statusButtons:document.querySelectorAll('[data-open-diagnostic-colors]').length};},tool);}
function closeEnough(a,b,label){for(const key of ['tabs','header','content'])for(const dim of ['top','left','width','bottom']){if(key==='content'&&dim==='bottom')continue;assert(Math.abs(a[key][dim]-b[key][dim])<1.5,label+' '+key+'.'+dim+': '+a[key][dim]+' vs '+b[key][dim]);}}
async function stickyLocalFilters101(tool,width,height){
 const entries=Array.from({length:180},(_,i)=>({entry:i+1,status:i%2?'down':'up',protocol:i%3?'UDP':'TCP',address:'192.0.2.'+(i%250+1),detail:'record '+i+' '+('long local snapshot field ').repeat(12),time:'2026-10-27T10:00:00Z'}));
 const fixture={id:'sticky101-'+tool,tool,target:'Local computer',options:{},status:'completed',started:'2026-10-27T10:00:00Z',finished:'2026-10-27T10:00:01Z',events:[],result:{summary:'Recorded local snapshot',data:{entries}}};
 const old=serverJobs.findIndex(j=>j.id===fixture.id);if(old>=0)serverJobs.splice(old,1);serverJobs.unshift(fixture);
 await page.setViewportSize({width,height});await page.evaluate(({fixture,tool})=>{jobs=[fixture,...jobs.filter(j=>j.id!==fixture.id)];selected[tool]=fixture.id;state.localTab=tool;if(tool==='wifi')wifiState.view='diagnostic';navigate('local');},{fixture,tool});await settle();
 const list=page.locator('#toolResultsScroll [data-list-view]').filter({has:page.locator('[data-result-filter-type]')}).first(),toolbar=list.locator('.data-list-tools'),query=toolbar.locator('[data-list-query]'),status=toolbar.locator('[data-result-filter-status]'),type=toolbar.locator('[data-result-filter-type]');
 assert.equal(await toolbar.locator('[data-result-filter-controls]').count(),1,'Status and Type share the search toolbar');await query.fill('record');await status.selectOption('up');await type.selectOption('tcp');await settle();await page.waitForFunction(()=>document.querySelector('#toolResultsScroll .data-list-tools [data-list-count]')?.textContent==='30 matching');
 await type.focus();await page.keyboard.press('ArrowUp');await settle();assert.equal(await type.inputValue(),'all','Status/Type remain keyboard accessible');await page.keyboard.press('ArrowDown');await settle();assert.equal(await type.inputValue(),'tcp');
 await page.locator('#toolResultsScroll').evaluate(el=>{el.scrollTop=600;el.scrollLeft=el.scrollWidth;el.dispatchEvent(new Event('scroll'));});await settle();
 const geometry=await toolbar.evaluate(el=>{const bounds=n=>{const b=n.getBoundingClientRect();return {top:b.top,bottom:b.bottom,left:b.left,right:b.right,height:b.height};},area=document.getElementById('toolResultsScroll');return {toolbar:bounds(el),area:bounds(area),scrollY:area.scrollTop,scrollX:area.scrollLeft,header:bounds(el.closest('.data-list').querySelector('thead th')),controls:[...el.querySelectorAll('input,select,button')].filter(n=>n.getClientRects().length).map(bounds)};});
 assert(geometry.scrollY>100,'Fixture genuinely scrolls its result list');assert(Math.abs(geometry.toolbar.top-geometry.area.top)<3,'Complete filter toolbar stays at the result viewport top '+JSON.stringify(geometry));assert(geometry.header.top>=geometry.toolbar.bottom-1,'Measured toolbar height keeps table headings below every wrapped control');
 for(const b of geometry.controls){assert(b.left>=geometry.area.left-1&&b.right<=geometry.area.right+1,'Pinned control remains horizontally reachable');assert(b.top>=geometry.toolbar.top-1&&b.bottom<=geometry.toolbar.bottom+1,'Pinned control stays in toolbar');assert(b.bottom<=geometry.area.bottom,'Controls do not clip in a short window');}
 assert.equal(await query.inputValue(),'record');assert.equal(await status.inputValue(),'up');assert.equal(await type.inputValue(),'tcp');
 const before=geometry.toolbar.top;await page.locator('#toolResultsScroll').evaluate(el=>{el.scrollTop+=120;});await settle();assert(Math.abs((await toolbar.boundingBox()).y-before)<2,'Controls do not move with subsequent list scrolling');
 await page.evaluate(()=>renderCurrentResult(true));await settle();assert.equal(await query.inputValue(),'record');assert.equal(await status.inputValue(),'up');assert.equal(await type.inputValue(),'tcp','Live result rendering preserves combined filter preferences');
 report.layouts.push({stickyFilters:true,tool,width,height,...geometry});if(out&&tool==='connections')await page.screenshot({path:path.join(out,'local101-filters-'+width+'x'+height+'.png')});
}

(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});const context=await browser.newContext({viewport:{width:1440,height:1000},serviceWorkers:'block'});page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));page.on('response',r=>{if(r.url().includes('/assets/')&&r.status()>=400)report.assetFailures.push(r.url());});
 await context.route('**/*',async route=>{const req=route.request(),url=new URL(req.url()),p=url.pathname;assert.equal(url.origin,'https://local89.toolkit.test','Unexpected external request');let data={};
 if(p.startsWith('/api/')){
  if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',system:{hostname:'Layout fixture',interfaces:[]}};
  if(p==='/api/state'&&req.method()==='GET')data={activePage:'overview',localTab:'ipconfig',appUpdatePreferences:{enabled:false}};
  if(p==='/api/system')data={hostname:'Layout fixture',interfaces:[]};
  if(p==='/api/jobs'){
   if(req.method()==='POST'){
    const body=req.postDataJSON();posted.push(body);await page.evaluate(value=>globalThis.__local89Posts=value,posted);
    if(postDelay)await new Promise(r=>setTimeout(r,postDelay));
    if(rejectNext){rejectNext=false;return route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:'Fixture refresh failure'})});}
    const now=new Date().toISOString();data={id:'local89-'+(++serial),tool:body.tool,target:'',options:body.options||{},status:nextStatus,started:now,events:[],eventSequence:0,result:{summary:{entries:0},data:{rows:[]}}};if(nextStatus==='completed')data.finished=now;serverJobs.unshift(data);
   }else data=serverJobs;
  }
  const full=p.match(/^\/api\/jobs\/([^/]+)$/);if(full)data=serverJobs.find(j=>j.id===full[1])||{};
  if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};if(p==='/api/terminal-views')data={available:false,owners:{},views:[]};
  if(p.startsWith('/api/public-ip'))data={provider:'ipify',checkedAt:new Date().toISOString(),expiresAt:new Date(Date.now()+300000).toISOString(),ipv4:{status:'ok',address:'192.0.2.45'},ipv6:{status:'unavailable',error:'No IPv6 fixture'}};
  if(p==='/api/local-tools/ip')data={available:true,adapters:[]};
  if(p.includes('/events')){await new Promise(r=>setTimeout(r,100));data={events:[],sessions:[],after:0};}
  return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
 }
 try{let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','local89').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto('https://local89.toolkit.test/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await page.evaluate(()=>globalThis.__local89Posts=[]);
 for(const width of [1440,820,390])for(const theme of ['dark','light'])for(const tool of ['ipconfig','routes']){
  const initialPosts=posted.length;await page.setViewportSize({width,height:1000});await page.evaluate(({theme,tool})=>{setTheme(theme);navigate('overview');state.localTab=tool;navigate('local');},{theme,tool});await settle();const initial=await metrics(tool);assert.equal(posted.length,initialPosts,'Main navigation does not start a snapshot');assert.equal(initial.statusButtons,1);assert.equal(await page.locator('#pageActions [data-open-diagnostic-colors]').count(),1);
  await tab(tool).click();await settle();if(tool==='routes')await page.waitForFunction(()=>selectedJob('routes')?.status==='completed');await settle();const repeated=await metrics(tool);assert.equal(repeated.statusButtons,1);closeEnough(initial,repeated,width+'/'+theme+'/'+tool);assert(initial.tabs.top<initial.content.top);assert(initial.tabs.top<250,'Local tabs should start near the page heading');
  const rendered=await page.evaluate(()=>{renderLocal();return true;});assert(rendered);await settle();closeEnough(initial,await metrics(tool),'renderLocal '+width+'/'+theme+'/'+tool);
  report.layouts.push({width,theme,tool,initial,repeated});if(out&&theme==='light')await page.screenshot({path:path.join(out,'local89-'+tool+'-'+width+'.png')});
 }
 check('Local Network initial navigation and repeated tab/render retain the same top positions in 12 desktop/compact/narrow theme layouts');
 await page.setViewportSize({width:1440,height:1000});await page.evaluate(()=>{state.localTab='ipconfig';navigate('local');});
 for(const tool of ['routes','arp','connections']){
  const before=count(tool);await tab(tool).click();await waitCount(tool,before+1);await page.waitForFunction(tool=>selectedJob(tool)?.status==='completed',tool);await tab(tool).click();await waitCount(tool,before+2);await page.waitForFunction(tool=>selectedJob(tool)?.status==='completed',tool);
 }
 check('Routing table, ARP and Connections selections each run a fresh snapshot, including selection of an already active tab');
 postDelay=450;nextStatus='running';const before=count('routes');await tab('routes').click();await waitCount('routes',before+1);await page.evaluate(()=>{for(let i=0;i<5;i++)document.querySelector('[data-local="routes"]').click();});await page.waitForTimeout(100);assert.equal(count('routes'),before+1,'In-flight job creation must not duplicate');await page.waitForFunction(()=>selectedJob('routes')?.status==='running');await page.evaluate(()=>{for(let i=0;i<5;i++)document.querySelector('[data-local="routes"]').click();});await page.waitForTimeout(100);assert.equal(count('routes'),before+1,'Active diagnostic must not duplicate');
 complete();postDelay=0;nextStatus='completed';await page.evaluate(()=>pollJobs());await page.waitForFunction(()=>selectedJob('routes')?.status==='completed');await tab('routes').click();await waitCount('routes',before+2);await page.waitForFunction(()=>selectedJob('routes')?.status==='completed');
 check('Rapid clicks while POST is pending and clicks during an active snapshot are deduplicated; completed snapshots can refresh again');
 const total=posted.length;await page.evaluate(async()=>{for(let i=0;i<3;i++){renderLocal();renderPage();updateRunButtons();await pollJobs();}});await settle();assert.equal(posted.length,total,'Ordinary rendering/polling must not trigger scans');
 await page.evaluate(()=>{online=false;renderLocal();for(const tool of ['routes','arp','connections'])document.querySelector('[data-local="'+tool+'"]').click();});await settle();assert.equal(posted.length,total,'Offline selection must not issue diagnostics');await page.evaluate(()=>{online=true;renderPage();});
 check('Normal rendering and job polling do not start scans, and offline tab selection makes no requests');
 rejectNext=true;const failures=count('arp');await tab('arp').click();await waitCount('arp',failures+1);await page.waitForFunction(()=>document.querySelector('#toastStack')?.textContent.includes('Fixture refresh failure'));await tab('arp').click();await waitCount('arp',failures+2);await page.waitForFunction(()=>selectedJob('arp')?.status==='completed');
 check('A failed automatic refresh releases its pending state and can be retried from the tab');
 for(const [tool,width,height] of [['routes',1440,1000],['connections',1440,1000],['connections',820,650],['connections',390,1000],['wifi',820,650]])await stickyLocalFilters101(tool,width,height);
 check('Local Network Status and Type stay with search during vertical/horizontal scrolling, responsive wrapping, keyboard changes and rerenders');
 assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetFailures,[]);report.passed=true;
})().catch(async e=>{report.failure=e.stack||String(e);console.error(report.failure);if(page&&out)await page.screenshot({path:path.join(out,'local89-failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'local89-results.json'),JSON.stringify(report,null,2));await browser?.close();});
