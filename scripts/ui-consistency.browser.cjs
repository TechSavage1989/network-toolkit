'use strict';
const {openAppSettings}=require('./browser-settings-navigation.cjs');
// Full application assets with isolated API fixtures. No devices or system files are changed.
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'../web'),out=process.env.TOOLKIT_TEST_OUTPUT,version=require('../package.json').version;
const hosts='# UI review fixture\n127.0.0.1 localhost\n::1 localhost\n192.0.2.20 switch.test\n'.replaceAll('\n','\r\n');
const adapters=[{index:7,name:'Ethernet',description:'Realtek PCIe 5GbE Family Controller',up:true,addresses:['192.0.2.4/24','2001:db8:1234:5678::4/64'],mac:'02:00:00:00:00:04',mtu:1500,speedMbps:1000,gateway:'192.0.2.1',gateways:['192.0.2.1'],dns:['192.0.2.53','2001:db8:1234:5678::53']},{index:8,name:'Ethernet 5',description:'Virtual adapter with a deliberately long description for responsive layout review',up:true,addresses:['198.51.100.4/24'],mac:'02:00:00:00:00:08',mtu:1500,speedMbps:1000,dns:['198.51.100.53']}];
const snapshot=()=>({available:true,editable:true,exists:true,path:'C:\\Windows\\System32\\drivers\\etc\\hosts',content:hosts,revision:crypto.createHash('sha256').update(hosts).digest('hex'),encoding:'UTF-8',newline:'CRLF',warnings:[]});
const result={passed:false,pages:[],cases:[],copies:[],hostsMenu:[],errors:[],assetFailures:[]};
const settle=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
let browser,page,saved={appUpdatePreferences:{enabled:false},terminalSidebarSide:'right',terminalSidebarTab:'saved'},writes=0;
async function fit(){return page.locator('.main .btn,.main .tab,.main .scope-switch button,.main .tool-panel-tabs button,.main .iconbtn').evaluateAll(elements=>elements.flatMap(element=>{
 const b=element.getBoundingClientRect();if(!b.width||!b.height)return [];
 const walker=document.createTreeWalker(element,NodeFilter.SHOW_TEXT),bad=[];
 while(walker.nextNode()){
  const text=walker.currentNode;if(!text.textContent.trim()||text.parentElement.closest('.sr-only,[aria-hidden="true"]'))continue;
  const range=document.createRange();range.selectNodeContents(text);
  for(const r of range.getClientRects())if(r.width&&(r.left<b.left-2||r.right>b.right+2||r.top<b.top-2||r.bottom>b.bottom+2))bad.push(text.textContent.trim());
 }
 for(const svg of element.querySelectorAll(':scope > svg')){const r=svg.getBoundingClientRect();if(r.width&&(r.left<b.left-1||r.right>b.right+1||r.top<b.top-1||r.bottom>b.bottom+1))bad.push('icon outside control');}
 return bad.length?[{text:element.innerText,tag:element.tagName,bad}]:[];
}));}
async function copy(button,expected,keyboard=false){
 await button.scrollIntoViewIfNeeded();if(keyboard){await button.focus();await page.keyboard.press('Enter');}else await button.click();
 await page.waitForFunction(async value=>(await navigator.clipboard.readText())===value,expected);
 assert.equal(await page.locator('#toastStack .toast').filter({hasText:'Copied to clipboard.'}).count(),1);
 result.copies.push({expected,keyboard});
}
(async()=>{
 browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 const context=await browser.newContext({viewport:{width:1600,height:1000},permissions:['clipboard-read','clipboard-write'],acceptDownloads:true});page=await context.newPage();
 page.on('pageerror',e=>result.errors.push(e.message));page.on('response',r=>{if(r.url().includes('/assets/')&&r.status()>=400)result.assetFailures.push(r.url());});
 await page.route('https://toolkit.test/**',async route=>{
  const request=route.request(),p=new URL(request.url()).pathname,body=request.postData()?request.postDataJSON():null;
  if(p.startsWith('/api/')){
   let data={};
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',terminalWindowAvailable:true};
   if(p==='/api/state'){if(request.method()==='GET')data=saved;else saved=body;}
   if(p==='/api/system')data={hostname:'Workstation.test',os:'Windows',arch:'amd64',interfaces:adapters,gateways:['192.0.2.1'],dnsServers:['192.0.2.53','2001:db8:1234:5678::53']};
   if(p==='/api/jobs')data=[];
   if(p==='/api/local-tools/ip')data={adapters};
   if(p==='/api/local-tools/hosts'){if(request.method()!=='GET')writes++;data=snapshot();}
   if(p==='/api/terminals')data={sessions:[]};
   if(p==='/api/terminals/serial-ports')data={ports:[{name:'COM4',description:'Fixture console'}]};
   if(p==='/api/terminal-views')data={available:true,views:[],owners:{}};
   if(p==='/api/remote-browser')data={sessions:[]};
   if(p==='/api/credentials')data={entries:[]};
   if(p==='/api/updates/status')data={currentVersion:version,supported:true,hasUpdate:false};
   if(p==='/api/terminal-automation/library')data={revision:'fixture',macros:[],bindings:[]};
   if(p==='/api/terminal-automation/targets')data={targets:[]};
   if(p==='/api/terminal-automation/runs')data={runs:[]};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  let file=path.resolve(root,'.'+(p==='/'?'/index.html':p));if(!file.startsWith(root+path.sep))return route.fulfill({status:403,body:''});
  if(!fs.existsSync(file)&&/^\/assets\/Network_Toolkit_Icon\.(?:svg|ico)$/.test(p))file=path.join(root,'../assets',path.basename(p));
  try{let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','ui-fixture').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 if(out)fs.mkdirSync(out,{recursive:true});
 await page.goto('https://toolkit.test/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
 result.pages=await page.locator('#navigation [data-page]').evaluateAll(items=>[...new Set(items.map(item=>item.dataset.page))]);
 assert(result.pages.includes('terminal')&&result.pages.includes('hosts')&&result.pages.includes('capture'));
 for(const width of [1600,820,390])for(const theme of ['light','dark']){
  await page.setViewportSize({width,height:1000});await page.evaluate(theme=>setTheme(theme),theme);
  for(const id of result.pages){
   await page.evaluate(id=>navigate(id),id);await settle(page);
   if(id==='hosts')await page.waitForSelector('.he-toolbar');
   const bad=await fit();assert.deepEqual(bad,[],JSON.stringify({width,theme,page:id,bad}));
   assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,JSON.stringify({width,theme,page:id,overflow:true}));
   result.cases.push({width,theme,page:id});
   if(out&&id==='terminal'&&(width===1600||width===390))await page.screenshot({path:path.join(out,'ui-terminal-'+theme+'-'+width+'.png')});
  }
  await page.evaluate(()=>navigate('overview'));await settle(page);
  assert.equal(await page.locator('#pageContent .tool-card,#adapterList .adapter-address button:not([data-value-copy-control])').count(),0);
  assert.equal(await page.locator('#navigation [data-page="dns"]').count(),1);
  const address=page.locator('#adapterList .adapter-address-value').first();await address.locator('[data-value-copy-control]').waitFor();
  const placements=await page.locator('#adapterList [data-value-copy-control],#overviewStats [data-value-copy-control]').evaluateAll(buttons=>buttons.map(button=>{
   const b=button.getBoundingClientRect(),p=button.parentElement.getBoundingClientRect();return {label:button.getAttribute('aria-label'),left:b.left,top:b.top,right:b.right,bottom:b.bottom,parent:{left:p.left,top:p.top,right:p.right,bottom:p.bottom}};
  }));
  for(const r of placements)assert(r.left>=r.parent.left-1&&r.right<=r.parent.right+1&&r.top>=r.parent.top-1&&r.bottom<=r.parent.bottom+1,JSON.stringify({width,theme,r}));
  await copy(address.locator('[data-value-copy-control]'),'192.0.2.4/24',theme==='dark');
  await copy(page.locator('#adapterList [aria-label="Copy MAC address value"]').first(),'02:00:00:00:00:04');
  assert.equal(await page.locator('#entryContextMenu:not(.hidden)').count(),0);
  if(out&&(width===1600||width===390)){await page.evaluate(()=>document.querySelector('#workspaceScroll').scrollTo({top:0,left:0}));await settle(page);await page.screenshot({path:path.join(out,'ui-overview-'+theme+'-'+width+'.png')});}
  await page.evaluate(()=>navigate('hosts'));await page.waitForSelector('.he-toolbar');await settle(page);
  await copy(page.locator('[data-hosts-action="copy-path"]'),'C:\\Windows\\System32\\drivers\\etc\\hosts',theme==='dark');
  const summary=page.locator('.he-more>summary'),find=page.locator('[data-hosts-action="find"]');
  const a=await summary.boundingBox(),b=await find.boundingBox();
  if(Math.abs(a.y-b.y)<Math.min(a.height,b.height))assert(Math.abs((a.y+a.height/2)-(b.y+b.height/2))<=1,JSON.stringify({width,theme,a,b}));
  await summary.focus();await page.keyboard.press('Enter');await page.waitForFunction(()=>document.querySelector('.he-more').open);await settle(page);
  const menu=await page.locator('.he-menu').boundingBox(),panel=await page.locator('.he-app').boundingBox();assert(menu.x>=panel.x-1&&menu.x+menu.width<=panel.x+panel.width+1,JSON.stringify({width,theme,menu,panel}));
  result.hostsMenu.push({width,theme,keyboardOpen:true,aligned:true,contained:true});
  if(out&&(width===1600||width===390))await page.screenshot({path:path.join(out,'ui-hosts-menu-'+theme+'-'+width+'.png')});
  if(width===1600&&theme==='light'){
   const downloadPromise=page.waitForEvent('download');await page.locator('[data-hosts-action="export"]').click();const download=await downloadPromise;assert.equal(fs.readFileSync(await download.path(),'utf8'),hosts);
  }else{await page.keyboard.press('Escape');assert.equal(await page.locator('.he-more').getAttribute('open'),null);assert.equal(await summary.evaluate(el=>document.activeElement===el),true);}
  if(out&&(width===1600||width===390)){
   await openAppSettings(page);await page.locator('#modalTitle').filter({hasText:'App settings'}).waitFor();await settle(page);
   await page.screenshot({path:path.join(out,'ui-settings-gear-'+theme+'-'+width+'.png')});await page.keyboard.press('Escape');
  }
 }
 assert.equal(writes,0);assert.deepEqual(result.errors,[]);assert.deepEqual(result.assetFailures,[]);result.passed=true;
 console.log(JSON.stringify({passed:true,pages:result.pages.length,renderedCases:result.cases.length,copies:result.copies.length,hostsMenuCases:result.hostsMenu.length,noHostsWrites:true}));
})().catch(async error=>{result.failure=error.stack||String(error);console.error(result.failure);if(out&&page)await page.screenshot({path:path.join(out,'ui-consistency-failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'ui-consistency-results.json'),JSON.stringify(result,null,2)+'\n');await browser?.close();});
