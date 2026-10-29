'use strict';
// Bundled UI with isolated API fixtures. No real devices, files or installations are changed.
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT,report={passed:false,checks:[],layouts:[],headers:[],errors:[],assetFailures:[]};
let browser,page,serial=0;const check=name=>{report.checks.push(name);console.log('PASS '+name);};
const settle=()=>page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
const hosts='# UI fixture\r\n127.0.0.1 localhost\r\n192.0.2.20 switch.test\r\n';
const entries=[{name:'configs',path:'/configs',directory:true},{name:'config.txt',path:'/config.txt',directory:false,size:1234,modified:'2026-10-25T12:30:00Z',kind:'File'}];
const stamp='2026-10-26T00:00:00Z';
const fixture=(tool,data)=>({id:'ui90-'+tool,tool,target:tool==='discover'?'192.0.2.0/24':'',status:'completed',started:stamp,finished:stamp,options:{},events:[],result:{summary:'UI review fixture',data}});
const fixtures=[fixture('discover',{scope:'subnet',total:1,completed:1,responded:1,hosts:[{address:'192.0.2.20',hostname:'switch.test',responded:true,completed:true,status:'responded',mac:'02:00:00:00:00:20',manufacturer:'Fixture maker',openPorts:[22,443]}]}),fixture('connections',{connections:[{local:'192.0.2.4:53100',pid:1234,protocol:'TCP',remote:'192.0.2.20:443',state:'ESTABLISHED'}]}),fixture('routes',{routes:[{destination:'192.0.2.0/24',gateway:'192.0.2.1',metric:10,interface:'Ethernet'}]})];
async function headerMetrics(selector,label){
 await page.locator(selector+' th').first().waitFor();await settle();
 const values=await page.locator(selector+' th').evaluateAll(items=>items.filter(e=>e.getBoundingClientRect().width&&e.textContent.trim()).map(e=>{const style=getComputedStyle(e),button=e.querySelector('button'),b=e.getBoundingClientRect(),issues=[],walker=document.createTreeWalker(e,NodeFilter.SHOW_TEXT);while(walker.nextNode()){const node=walker.currentNode;if(!node.textContent.trim()||node.parentElement.closest('.sr-only,[aria-hidden="true"]'))continue;const r=document.createRange();r.selectNodeContents(node);for(const rect of r.getClientRects())if(rect.width&&(rect.left<b.left-2||rect.right>b.right+2||rect.top<b.top-2||rect.bottom>b.bottom+2))issues.push(node.textContent.trim());}return {text:e.textContent.trim(),font:parseFloat(style.fontSize),buttonFont:button?parseFloat(getComputedStyle(button).fontSize):null,issues};}));
 assert(values.length,label+' has headers');for(const v of values){assert(v.font>=12,label+' '+v.text+' header font '+v.font);if(v.buttonFont!==null)assert(v.buttonFont>=12,label+' sort font '+v.buttonFont);assert.deepEqual(v.issues,[],label+' clipped header '+v.text);}report.headers.push({label,values});
}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});const context=await browser.newContext({viewport:{width:1440,height:1000},serviceWorkers:'block'});page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));page.on('response',r=>{if(r.url().includes('/assets/')&&r.status()>=400)report.assetFailures.push(r.url());});
 await context.route('**/*',async route=>{const req=route.request(),url=new URL(req.url()),p=url.pathname;assert.equal(url.origin,'https://ui90.toolkit.test','Unexpected external request');let data={};
 if(p.startsWith('/api/')){
  if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',system:{hostname:'UI fixture',interfaces:[]}};
  if(p==='/api/state'&&req.method()==='GET')data={appUpdatePreferences:{enabled:false}};
  if(p==='/api/system')data={hostname:'UI fixture',interfaces:[]};
  if(p==='/api/jobs'){if(req.method()==='POST'){const body=req.postDataJSON();data={...fixture(body.tool,{}),id:'ui90-run-'+(++serial),options:body.options};fixtures.unshift(data);}else data=fixtures;}
  const full=p.match(/^\/api\/jobs\/([^/]+)$/);if(full)data=fixtures.find(j=>j.id===full[1])||{};
  if(p==='/api/terminals')data={sessions:[]};if(p==='/api/terminal-views')data={available:false,owners:{},views:[]};
  if(p==='/api/remote-browser')data=req.method()==='GET'?{sessions:[{id:'fixture',host:'192.0.2.10',username:'test',port:22,label:'Test SSH'}]}:{path:'/',entries};
  if(p.startsWith('/api/public-ip'))data={provider:'ipify',checkedAt:new Date().toISOString(),expiresAt:new Date(Date.now()+300000).toISOString(),ipv4:{status:'ok',address:'192.0.2.45'},ipv6:{status:'unavailable',error:'No IPv6 fixture'}};
  if(p==='/api/local-tools/ip')data={available:true,adapters:[]};
  if(p==='/api/local-tools/hosts')data={available:true,editable:true,exists:true,path:'C:\\Windows\\System32\\drivers\\etc\\hosts',content:hosts,revision:crypto.createHash('sha256').update(hosts).digest('hex'),encoding:'UTF-8',newline:'CRLF',warnings:[]};
  if(p.includes('/events')){await new Promise(r=>setTimeout(r,100));data={events:[],sessions:[],after:0};}
  return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
 }
 try{let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','ui90').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto('https://ui90.toolkit.test/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);
 for(const width of [1440,820,390])for(const theme of ['dark','light']){
  await page.setViewportSize({width,height:1000});await page.evaluate(theme=>setTheme(theme),theme);
  const pages=[];
  for(const tool of ['quick','ping','discover','local','http','tls']){
   await page.evaluate(tool=>navigate(tool),tool);await settle();assert.equal(await page.locator('[data-open-diagnostic-colors]').count(),1,tool+' one colors button');assert.equal(await page.locator('#pageActions [data-open-diagnostic-colors]').count(),1,tool+' heading placement');assert.equal(await page.locator('#pageContent [data-open-diagnostic-colors]').count(),0,tool+' no content shortcut');
   const controls=await page.locator('#pageActions').evaluate(el=>{const a=el.querySelector('[data-open-diagnostic-colors]'),b=el.querySelector('[data-reset-tool-layout]');const size=e=>({height:e.getBoundingClientRect().height,font:parseFloat(getComputedStyle(e).fontSize),icon:e.querySelector('svg').getBoundingClientRect().width});return {colors:size(a),reset:size(b),overflow:document.documentElement.scrollWidth>innerWidth+1,frameBottom:document.querySelector('.main').getBoundingClientRect().bottom};});
   assert(Math.abs(controls.colors.height-controls.reset.height)<=2,tool+' heading button heights');assert.equal(controls.colors.icon,controls.reset.icon,tool+' heading icon scale');assert(controls.colors.height>=32&&controls.colors.font>=12);assert.equal(controls.overflow,false,tool+' page overflow');assert(controls.frameBottom<=await page.evaluate(()=>innerHeight+2),tool+' main frame exceeds window: '+JSON.stringify(controls));
   await page.evaluate(()=>renderPage());await settle();assert.equal(await page.locator('#pageActions [data-open-diagnostic-colors]').count(),1,tool+' rerender placement');pages.push({tool,controls});
  }
  await page.evaluate(()=>{state.localTab='routes';navigate('local');renderLocal();state.localTab='connections';renderLocal();});await settle();assert.equal(await page.locator('#pageActions [data-open-diagnostic-colors]').count(),1);report.layouts.push({width,theme,pages});if(out&&theme==='light')await page.screenshot({path:path.join(out,'ui90-shared-controls-'+width+'.png')});
 }
 check('Status colors stays in the shared page-heading actions with consistent button/icon sizing across tools, themes and six layouts');
 await page.setViewportSize({width:1440,height:1000});await page.evaluate(()=>{navigate('overview');if(state.navigationCollapsed)NetworkToolkitNavigation.toggle();});
 await page.locator('#sidebar [data-page="ping"]').click();assert.equal(await page.evaluate(()=>state.navigationCollapsed),false,'Expanded single click stays expanded');await page.waitForTimeout(550);
 await page.locator('#sidebar [data-page="http"]').dblclick();await page.waitForFunction(()=>state.navigationCollapsed===true);await page.waitForTimeout(100);assert.equal(await page.evaluate(()=>state.navigationCollapsed),true,'Native dblclick must not undo collapse');assert.equal(await page.evaluate(()=>state.activePage),'http');
 await page.waitForTimeout(550);await page.locator('#sidebar [data-page="ping"]').click();assert.equal(await page.evaluate(()=>state.navigationCollapsed),true,'Rail single click stays collapsed');await page.waitForTimeout(550);
 await page.locator('#sidebar [data-page="editor"]').dblclick();await page.waitForFunction(()=>state.navigationCollapsed===false);await page.waitForTimeout(100);assert.equal(await page.evaluate(()=>state.navigationCollapsed),false,'Native dblclick must not undo expansion');assert.equal(await page.evaluate(()=>state.activePage),'editor');
 check('Tool-tab double-click toggles expanded and collapsed navigation exactly once while single-click navigation preserves its state');
 for(const collapsed of [false,true]){
  await page.evaluate(collapsed=>{if(state.navigationCollapsed!==collapsed)NetworkToolkitNavigation.toggle();},collapsed);await settle();
  const divider=await page.locator('#appNavigationDivider').evaluate(el=>{const b=el.getBoundingClientRect(),before=getComputedStyle(el,'::before');const hit=x=>{const target=document.elementFromPoint(x,b.top+Math.min(150,b.height/2));return target===el||el.contains(target);};return {width:b.width,line:parseFloat(before.width),leftHit:hit(b.left+1),rightHit:hit(b.right-1)};});assert(divider.width>=16,'Wide divider hit area');assert(divider.line>=4,'Visible divider line');assert(divider.leftHit&&divider.rightHit,'Divider target accepts both edges');report.layouts.push({divider,collapsed});
 }
 await page.locator('#appNavigationDivider').focus();await page.keyboard.press('Enter');assert.equal(await page.evaluate(()=>state.navigationCollapsed),false);await page.locator('#appNavigationDivider').dblclick();assert.equal(await page.evaluate(()=>state.navigationCollapsed),true);await page.waitForTimeout(550);await page.locator('#sidebar [data-page="local"]').click();assert.equal(await page.evaluate(()=>state.activePage),'local');assert.equal(await page.evaluate(()=>state.navigationCollapsed),true,'Wider divider does not intercept tool icons');
 check('Sidebar divider has a wider hit target and visible line in both states without blocking navigation or keyboard toggles');
 await page.evaluate(data=>{if(state.navigationCollapsed)NetworkToolkitNavigation.toggle();jobs=data;},fixtures);
 // Fixtures are supplied through the normal job API and selected with the application's own result renderer.
 await page.evaluate(()=>{selected.discover='ui90-discover';navigate('discover');});await headerMetrics('#toolResult table','Network discovery');
 await page.evaluate(()=>{selected.connections='ui90-connections';state.localTab='connections';navigate('local');});await headerMetrics('#toolResult table','Connections');if(out)await page.screenshot({path:path.join(out,'ui90-connections-1440.png')});
 await page.evaluate(()=>{document.querySelector('#pageContent').innerHTML=simpleTable(['Name','Recorded value'],[['Fixture','192.0.2.20']]);});await headerMetrics('#pageContent table','Simple result table');
 await page.evaluate(()=>navigate('hosts'));await page.locator('.he-table th').first().waitFor();await headerMetrics('.he-table','Hosts entries');
 await page.evaluate(()=>navigate('editor'));await page.waitForFunction(()=>document.querySelector('#ntEditorRemoteSource option[value="terminal:fixture"]'));if(await page.locator('[data-ed="files"]').getAttribute('aria-expanded')!=='true')await page.locator('[data-ed="files"]').click();await page.locator('[data-editor-files-side="remote"]').click();await page.locator('#ntEditorRemoteSource').selectOption('terminal:fixture');if(!await page.locator('.nt-editor-file-path-actions').evaluate(e=>e.open))await page.locator('.nt-editor-file-path-actions>summary').click();await page.locator('[data-ed-page="browseRemote"]').click();await headerMetrics('.nt-editor-picker-table','Remote directory');if(out)await page.screenshot({path:path.join(out,'ui90-remote-1440.png')});await page.locator('.modal [data-action="close-modal"]').last().click();
 check('Discovery, connection, simple result, Hosts and remote directory headers use readable proportional text with matching sortable controls and no clipped header text');
 assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetFailures,[]);report.passed=true;
})().catch(async e=>{report.failure=e.stack||String(e);console.error(report.failure);if(page&&out)await page.screenshot({path:path.join(out,'ui90-failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'ui90-results.json'),JSON.stringify(report,null,2));await browser?.close();});
