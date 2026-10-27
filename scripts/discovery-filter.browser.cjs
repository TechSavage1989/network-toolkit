'use strict';
// Real bundled UI and Chromium clipboard; all device/API responses are synthetic.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR,report={passed:false,checks:[],layouts:[],errors:[],assetFailures:[]};
let browser,page;const stamp='2026-10-27T18:00:00Z';
const host=(address,mac,extra={})=>({address,mac,hostname:'fixture-'+address.replaceAll('.','-'),vendor:'Example Devices',responded:true,completed:true,status:'responded',time:stamp,openPorts:[],...extra});
const hosts=[host('192.0.2.1','AA:BB:CC:00:00:01',{friendlyName:'Main office',vendor:'Aruba Networks',openPorts:[22]}),host('192.0.2.10','AA:BB:CC:00:00:02',{responded:false,status:'no_response'}),host('192.0.2.30','AA:BB:CC:00:00:03',{friendlyName:'South office',vendor:'Wireless Vendor'}),...Array.from({length:180},(_,i)=>host('198.51.100.'+(i+1),'02:00:00:00:'+Math.floor(i/256).toString(16).padStart(2,'0')+':'+(i%256).toString(16).padStart(2,'0'),{interfaceDescription:'Long synthetic adapter description '.repeat(4)}))];
const fixtures=['discover','tcp','ping','arp'].map(tool=>({id:'filter105-'+tool,tool,target:'192.0.2.0/24',options:{scope:'subnet'},status:'completed',started:stamp,finished:stamp,events:[],result:{summary:{entries:hosts.length},data:tool==='arp'?{neighbors:hosts,time:stamp}:{scope:'subnet',total:hosts.length,started:hosts.length,completed:hosts.length,responded:hosts.length-1,hosts}}}));
const check=name=>{report.checks.push(name);console.log('PASS '+name);};
const settle=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
const query=()=>page.locator('#toolResultsScroll [data-network-filter="query"]');
const shown=()=>page.locator('#toolResultsScroll [data-network-body] tbody [data-select-device]').evaluateAll(nodes=>nodes.map(n=>n.getAttribute('aria-label').replace(/^Select | and show device details$/g,'')));
async function show(tool){await page.evaluate(({tool,fixtures})=>{jobs=fixtures;selected[tool]='filter105-'+tool;if(LOCAL_TOOLS.includes(tool)){state.activePage='local';state.localTab=tool;renderPage();}else navigate(tool);renderCurrentResult(true);},{tool,fixtures});await settle();const tab=page.locator('#toolResultsTab');if(await tab.isVisible()&&await tab.getAttribute('aria-selected')!=='true')await tab.click();await query().waitFor();}
async function paste(text){await query().focus();await page.keyboard.press('Control+a');await page.evaluate(text=>navigator.clipboard.writeText(text),text);await page.keyboard.press('Control+v');await settle();assert.equal(await page.evaluate(()=>globalThis.__filter105PasteTrusted),true,'Uses native trusted clipboard paste');}
async function expectRows(expected){await page.waitForFunction(expected=>{const nodes=[...document.querySelectorAll('#toolResultsScroll [data-network-body] tbody [data-select-device]')];return JSON.stringify(nodes.map(n=>n.getAttribute('aria-label').replace(/^Select | and show device details$/g,'')))===JSON.stringify(expected);},expected);assert.deepEqual(await shown(),expected);}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});const context=await browser.newContext({viewport:{width:1440,height:1000},permissions:['clipboard-read','clipboard-write'],serviceWorkers:'block'});page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));page.on('response',r=>{if(r.url().includes('/assets/')&&r.status()>=400)report.assetFailures.push(r.url());});
 await context.route('**/*',async route=>{const request=route.request(),url=new URL(request.url()),p=url.pathname;assert.equal(url.origin,'https://filter105.toolkit.test','No live network/device request');let data={};
  if(p.startsWith('/api/')){
   if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',system:{hostname:'Synthetic filter fixture',interfaces:[]}};
   if(p==='/api/state'&&request.method()==='GET')data={activePage:'overview',appUpdatePreferences:{enabled:false}};
   if(p==='/api/system')data={hostname:'Synthetic filter fixture',interfaces:[]};
   if(p==='/api/jobs'){assert.equal(request.method(),'GET','Filtering must not start a scan');data=fixtures;}
   const full=p.match(/^\/api\/jobs\/([^/]+)$/);if(full)data=fixtures.find(j=>j.id===full[1])||{};
   if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};if(p==='/api/terminal-views')data={available:false,owners:{},views:[]};
   if(p==='/api/local-tools/ip')data={available:true,adapters:[]};
   if(p.startsWith('/api/public-ip'))data={provider:'fixture',checkedAt:stamp,expiresAt:'2026-11-01T00:00:00Z',ipv4:{status:'ok',address:'192.0.2.45'},ipv6:{status:'unavailable'}};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,75));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','filter105').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto('https://filter105.toolkit.test/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await page.evaluate(()=>document.addEventListener('paste',e=>{globalThis.__filter105PasteTrusted=e.isTrusted;},true));
 await show('discover');await paste('AA-BB-CC-00-00-01\r\naabb.cc00.0002\nAABBCC000003\n');await expectRows(['192.0.2.1','192.0.2.30']);assert.equal(await query().inputValue(),'AA-BB-CC-00-00-01 aabb.cc00.0002 AABBCC000003 ');
 await page.locator('[data-network-view="all"]').click();await expectRows(['192.0.2.1','192.0.2.10','192.0.2.30']);
 await page.locator('#toolResultsScroll [data-network-filter="status"]').selectOption('no_response');await expectRows(['192.0.2.10']);await page.locator('#toolResultsScroll [data-network-filter="status"]').selectOption('all');
 if(out)await page.screenshot({path:path.join(out,'discovery-filter105-multiline.png')});
 check('Native multiline MAC paste is intercepted before input sanitation; normalized MAC alternatives intersect with responded-only and Outcome filters');
 for(const text of ['192.0.2.1 192.0.2.30','192.0.2.1,192.0.2.30,','192.0.2.1\n192.0.2.30\n192.0.2.1']){await paste(text);await expectRows(['192.0.2.1','192.0.2.30']);}
 await paste('192.0.2.1');await expectRows(['192.0.2.1']);await paste('"Main office" wireless');await expectRows(['192.0.2.1','192.0.2.30']);
 await query().evaluate(el=>{el.setSelectionRange(0,el.value.length);});await page.evaluate(()=>navigator.clipboard.writeText('192.0.2.30'));await page.keyboard.press('Control+v');await expectRows(['192.0.2.30']);
 check('Space/comma/list input, duplicates, trailing delimiters, exact IPs, quoted phrases and selection replacement filter correctly');
 const long=Array.from({length:700},(_,i)=>'02:01:'+i.toString(16).padStart(8,'0').match(/../g).join(':')).concat(['aabb.cc00.0001','aabb.cc00.0003']).join('\n');await paste(long);await expectRows(['192.0.2.1','192.0.2.30']);assert((await query().inputValue()).length>10000);await page.evaluate(()=>{const saved=JSON.parse(JSON.stringify(state.listPreferences));mergeListPreferences(saved);networkResultFilters.delete('filter105-discover');renderCurrentResult(true);});await settle();assert.equal(await query().inputValue(),long.replaceAll('\n',' '));await expectRows(['192.0.2.1','192.0.2.30']);
 await paste('x'.repeat(32769));await expectRows(['192.0.2.1','192.0.2.30']);assert.equal(await query().inputValue(),long.replaceAll('\n',' '));assert((await page.locator('#toastStack').innerText()).includes('32,768'));
 check('Hundreds of identifiers survive rerender and persistence; oversized paste is explicitly rejected without partial filter loss');
 for(const tool of ['tcp','ping','arp']){await show(tool);await paste('192.0.2.1,192.0.2.30');await expectRows(['192.0.2.1','192.0.2.30']);}check('Discovery, subnet TCP/ping and neighbor address lists use the same query semantics');
 await show('discover');await page.locator('[data-network-reset]').click();await query().fill('198.51.100.');await settle();
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:1000});await settle();await page.locator('#toolResultsScroll').evaluate(el=>{el.scrollTop=500;el.scrollLeft=el.scrollWidth;el.dispatchEvent(new Event('scroll'));});await settle();
  const geometry=await page.evaluate(()=>{const box=n=>{const r=n.getBoundingClientRect();return {left:r.left,right:r.right,top:r.top,bottom:r.bottom};},area=document.getElementById('toolResultsScroll'),toolbar=area.querySelector('.network-list-toolbar');return {area:box(area),toolbar:box(toolbar),scrollTop:area.scrollTop,header:box(area.querySelector('thead th')),controls:[...toolbar.querySelectorAll('input,select,button')].filter(n=>n.getClientRects().length).map(box)};});
  assert(geometry.scrollTop>100,'Genuine vertical scroll');assert(Math.abs(geometry.toolbar.top-geometry.area.top)<3,'Toolbar remains pinned');assert(geometry.header.top>=geometry.toolbar.bottom-1,'Column heading follows measured toolbar');for(const control of geometry.controls){assert(control.left>=geometry.area.left-1&&control.right<=geometry.area.right+1,'Filter controls remain horizontally reachable');assert(control.top>=geometry.toolbar.top-1&&control.bottom<=geometry.toolbar.bottom+1,'Controls remain within wrapped toolbar');}
  report.layouts.push({width,...geometry});if(out)await page.screenshot({path:path.join(out,'discovery-filter105-'+width+'.png')});
 }
 check('Normal and narrow layouts retain pinned query/Outcome/view controls and aligned table headings while scrolled');
 await query().fill(', \n ;');await page.locator('[data-network-reset]').click();assert.equal(await query().inputValue(),'');assert.equal(await page.locator('#toolResultsScroll [data-network-filter="status"]').inputValue(),'all');check('Clearing and reset preserve existing controls and default responded-only discovery view');
 assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetFailures,[]);report.passed=true;
})().catch(async error=>{report.failure=error.stack||String(error);console.error(report.failure);if(page&&out)await page.screenshot({path:path.join(out,'discovery-filter105-failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'discovery-filter105-results.json'),JSON.stringify(report,null,2)+'\n');await browser?.close();});
