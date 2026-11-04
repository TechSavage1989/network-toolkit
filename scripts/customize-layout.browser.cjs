'use strict';
// Customizable Overview sections and reorderable Test a target / Test a website tabs (owner request, v1.20.9).
// Synthetic local fixture only: no connections are opened.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const root=path.resolve(__dirname,'..'),output=process.env.TOOLKIT_TEST_OUTPUT,origin='https://customize.toolkit.test',report={passed:false,checks:[],errors:[]};
(async()=>{if(output)fs.mkdirSync(output,{recursive:true});const browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 try{
  const context=await browser.newContext({viewport:{width:1280,height:800}}),page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));
  let saved={activePage:'overview',appUpdatePreferences:{enabled:false},target:'192.0.2.10'};
  const puts=[],operations=[];const savedValue=async(pick,ok,ms=5000)=>{const until=Date.now()+ms;while(Date.now()<until){const v=pick(puts.at(-1)||{});if(ok(v))return v;await new Promise(r=>setTimeout(r,100));}return pick(puts.at(-1)||{});};
  await page.route(origin+'/**',async route=>{if(await handleEditorActivityMetadata(route,origin,report))return;const req=route.request(),p=new URL(req.url()).pathname,method=req.method();
   if(p.startsWith('/api/')){let data={};
    if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',system:{hostname:'EXAMPLE-PC',os:'windows',arch:'amd64',gateways:['192.0.2.1'],dnsServers:['192.0.2.53'],interfaces:[{index:7,name:'Ethernet',up:true,addresses:['192.0.2.60/24'],gateways:['192.0.2.1']}]}};
    else if(p==='/api/system')data={hostname:'EXAMPLE-PC',os:'windows',arch:'amd64',gateways:['192.0.2.1'],dnsServers:['192.0.2.53'],interfaces:[{index:7,name:'Ethernet',up:true,addresses:['192.0.2.60/24'],gateways:['192.0.2.1']}]};
    else if(p==='/api/state'){if(method==='PUT'){saved=req.postDataJSON();puts.push(saved);}else data=saved;}
    else if(p==='/api/jobs')data=[];
    else if(p==='/api/terminals')data={sessions:[]};
    else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
    else if(p==='/api/terminals/events'){await new Promise(r=>setTimeout(r,80));data={sessions:[],events:[],after:0};}
    else if(p==='/api/activity')data={tools:{}};
    else if(method!=='GET'){operations.push(method+' '+p);return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture: connections are not opened'})});}
    return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});}
   let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let bytes=fs.readFileSync(file);if(p==='/')bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','customize').replaceAll('__WINDOW_MODE__','main').replaceAll('__TERMINAL_VIEW__','null').replaceAll('__EDITOR_VIEW__','null');
   return route.fulfill({body:bytes,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});});
  await page.goto(origin);await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('overview'));
  const sections=()=>page.evaluate(()=>[...document.querySelectorAll('#pageContent [data-overview-section]')].map(e=>e.dataset.overviewSection));
  await page.waitForFunction(()=>document.querySelectorAll('#pageContent [data-overview-section]').length>=4);
  const initial=await sections();assert.deepEqual(initial.filter(x=>x!=='network'),['stats','publicIP','discovery','adapters activity'],'default dashboard order: '+JSON.stringify(initial));report.checks.push('Overview renders its default sections');
  await page.locator('[data-action="customize-overview"]').click();await page.locator('#overviewCustomizeList').waitFor();
  await page.locator('[data-overview-show="publicIP"]').uncheck();await page.waitForFunction(()=>!document.querySelector('#pageContent [data-overview-section="publicIP"]'));
  await page.locator('[data-overview-move="up"][data-overview-id="activity"]').click();await page.locator('[data-overview-move="up"][data-overview-id="activity"]').click();
  let now=await sections();assert(!now.includes('publicIP'),'Public IP hidden');assert(now.indexOf('activity')<now.indexOf('adapters')||now.includes('activity adapters')||now.some(s=>s==='activity')&&now.indexOf('activity')<now.findIndex(s=>s.startsWith('adapters')),'Recent activity moved above Network adapters: '+JSON.stringify(now));
  const stored=await savedValue(x=>x.overviewLayout,v=>v&&v.hidden?.includes('publicIP')&&v.order.indexOf('activity')<v.order.indexOf('adapters'));assert(stored&&stored.hidden.includes('publicIP')&&stored.order.indexOf('activity')<stored.order.indexOf('adapters'),'layout saved: '+JSON.stringify(stored));report.checks.push('Hide and reorder dashboard sections; layout saved');
  if(output)await page.screenshot({path:path.join(output,'customize-overview-dialog.png')});
  await page.locator('[data-overview-reset]').click();await page.waitForFunction(()=>!!document.querySelector('#pageContent [data-overview-section="publicIP"]'));assert.equal(await page.evaluate(()=>state.overviewLayout),undefined);report.checks.push('Reset restores the default dashboard');
  await page.locator('#modalRoot [data-action="close-modal"]').last().click();
  // Hide everything: a clear empty state, not a blank page.
  await page.evaluate(()=>{state.overviewLayout={order:OVERVIEW_SECTIONS.map(x=>x[0]),hidden:OVERVIEW_SECTIONS.map(x=>x[0])};renderOverview();});assert.match(await page.locator('#pageContent').innerText(),/All dashboard sections are hidden/);await page.evaluate(()=>{delete state.overviewLayout;renderOverview();});report.checks.push('All sections hidden shows a clear empty state');
  // Test a target tabs.
  await page.evaluate(()=>navigate('ping'));await page.locator('.nt-target-tests [data-target-test="ping"]').waitFor();
  const tabs=()=>page.evaluate(()=>[...document.querySelectorAll('.nt-target-tests [data-target-test]')].map(b=>b.dataset.targetTest));
  assert.deepEqual(await tabs(),['monitor','ping','trace','dns','tcp','quick','identify'],'owner default order');report.checks.push('Default order: Monitor, Ping, Trace route, DNS, Ports, Device check, Details');
  const tab=id=>page.locator('.nt-target-tests [data-target-test="'+id+'"]');
  {const a=await tab('monitor').boundingBox(),b=await tab('ping').boundingBox();await page.mouse.move(a.x+a.width/2,a.y+a.height/2);await page.mouse.down();await page.mouse.move(a.x+a.width/2+8,a.y+a.height/2,{steps:3});await page.mouse.move(b.x+4,b.y+b.height/2,{steps:6});
   assert.equal(await page.locator('.test-tab-drop-before,.test-tab-drop-after').count(),0,'no line over the near half of the next tab');
   await page.mouse.move(b.x+b.width-4,b.y+b.height/2,{steps:4});assert.equal(await page.locator('[data-target-test="ping"].test-tab-drop-after').count(),1);await page.mouse.up();await page.waitForTimeout(200);}
  assert.deepEqual(await tabs(),['ping','monitor','trace','dns','tcp','quick','identify']);assert.equal(await page.evaluate(()=>state.activePage),'ping','a drag does not switch tools');
  assert.deepEqual(await savedValue(x=>x.testTabOrder?.target,v=>JSON.stringify(v)===JSON.stringify(['ping','monitor','trace','dns','tcp','quick','identify'])),['ping','monitor','trace','dns','tcp','quick','identify']);report.checks.push('Drag reorders Test a target tabs, saves the order and does not switch tools');
  await page.evaluate(()=>navigate('dns'));await page.locator('.nt-target-tests [data-target-test="dns"]').waitFor();assert.deepEqual(await tabs(),['ping','monitor','trace','dns','tcp','quick','identify'],'order kept after navigation');
  await tab('dns').focus();await page.keyboard.press('Alt+Shift+ArrowLeft');assert.deepEqual(await tabs(),['ping','monitor','dns','trace','tcp','quick','identify']);assert.equal(await page.evaluate(()=>document.activeElement?.dataset.targetTest),'dns');report.checks.push('Alt+Shift+Left moves a focused tab and keeps focus');
  await tab('tcp').click();await page.waitForFunction(()=>state.activePage==='tcp');report.checks.push('A plain click still opens the tool');
  // Test a website tabs have their own order.
  await page.evaluate(()=>navigate('http'));await page.locator('.nt-target-tests [data-target-test="tls"]').waitFor();await tab('tls').focus();await page.keyboard.press('Alt+Shift+ArrowLeft');assert.deepEqual(await tabs(),['tls','http']);
  const order=await savedValue(x=>x.testTabOrder,v=>v?.web?.[0]==='tls');assert.deepEqual(order.web,['tls','http']);assert.deepEqual(order.target,['ping','monitor','dns','trace','tcp','quick','identify']);report.checks.push('Test a website tabs reorder separately');
  // Restart: reload the app from the saved settings; the order and a dashboard layout must come back.
  await page.evaluate(()=>{state.overviewLayout={order:OVERVIEW_SECTIONS.map(x=>x[0]).reverse(),hidden:['publicIP']};saveState();});await savedValue(x=>x.overviewLayout,v=>v?.hidden?.includes('publicIP'));
  await page.reload();await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);
  assert.deepEqual(await page.evaluate(()=>state.testTabOrder),{target:['ping','monitor','dns','trace','tcp','quick','identify'],web:['tls','http']},'tab order restored after restart');
  await page.evaluate(()=>navigate('ping'));await page.locator('.nt-target-tests [data-target-test="ping"]').waitFor();assert.deepEqual(await tabs(),['ping','monitor','dns','trace','tcp','quick','identify']);
  await page.evaluate(()=>navigate('overview'));await page.waitForFunction(()=>document.querySelectorAll('#pageContent [data-overview-section]').length>=3);
  const restored=await sections();assert(!restored.includes('publicIP')&&restored[0].startsWith('activity'),'dashboard layout restored after restart: '+JSON.stringify(restored));report.checks.push('Tab order and dashboard layout survive a restart');
  if(output)await page.screenshot({path:path.join(output,'customize-tabs.png')});
  assert.deepEqual(report.errors,[]);report.passed=true;
 }finally{if(output)fs.writeFileSync(path.join(output,'customize-layout-report.json'),JSON.stringify(report,null,2));await browser.close();}
 console.log(report.checks.join('\n'));
})().catch(error=>{console.error(error);console.error(JSON.stringify(report.errors));process.exit(1);});
