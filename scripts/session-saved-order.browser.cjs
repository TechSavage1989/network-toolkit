'use strict';
// Saved sessions reorder by drag and drop and Alt+Shift+Up/Down; order persists; clicks and Escape behave.
// Synthetic local fixture only: no connections are opened.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const root=path.resolve(__dirname,'..'),output=process.env.TOOLKIT_TEST_OUTPUT,origin='https://saved-order.toolkit.test',report={passed:false,checks:[],errors:[]};
const profile=(id,name,host)=>({id,name,type:'ssh',host,port:22,username:'operator',authMethod:'password',sshCompatibility:'modern',timeoutMs:10000});
(async()=>{if(output)fs.mkdirSync(output,{recursive:true});const browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 try{
  const context=await browser.newContext({viewport:{width:1280,height:800}}),page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));
  let saved={activePage:'terminal',appUpdatePreferences:{enabled:false},terminalSidebarCollapsed:false,terminalSidebarTab:'saved',terminalProfiles:[profile('p-core','switch-core','192.0.2.5'),profile('p-ap','ap-floor2','192.0.2.40'),profile('p-edge','firewall-edge','192.0.2.2')]};
  const puts=[],operations=[];
  await page.route(origin+'/**',async route=>{if(await handleEditorActivityMetadata(route,origin,report))return;const req=route.request(),p=new URL(req.url()).pathname,method=req.method();
   if(p.startsWith('/api/')){let data={};
    if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64'};
    else if(p==='/api/state'){if(method==='PUT'){saved=req.postDataJSON();puts.push(saved);}else data=saved;}
    else if(p==='/api/jobs')data=[];
    else if(p==='/api/terminals')data={sessions:[]};
    else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
    else if(p==='/api/terminals/events'){await new Promise(r=>setTimeout(r,80));data={sessions:[],events:[],after:0};}
    else if(p==='/api/activity')data={tools:{}};
    else if(method!=='GET'){operations.push(method+' '+p);return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture: connections are not opened'})});}
    return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});}
   let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let bytes=fs.readFileSync(file);if(p==='/')bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','savedorder').replaceAll('__WINDOW_MODE__','main').replaceAll('__TERMINAL_VIEW__','null').replaceAll('__EDITOR_VIEW__','null');
   return route.fulfill({body:bytes,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});});
  await page.goto(origin);await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await page.evaluate(()=>{navigate('terminal');NetworkToolkitSessionSidebar.activateTab('saved',{expand:true});});
  const order=()=>page.evaluate(()=>[...document.querySelectorAll('#sessionSavedList [data-session-kind="saved"]')].map(r=>r.dataset.sessionId));
  await page.waitForFunction(()=>document.querySelectorAll('#sessionSavedList [data-session-kind="saved"]').length===3);
  assert.deepEqual(await order(),['p-ap','p-edge','p-core'],'name order until the first reorder');report.checks.push('Saved list starts in name order');
  const row=id=>page.locator('#sessionSavedList [data-session-id="'+id+'"] [data-session-entry]');
  async function drag(id,target,after,escape=false){const a=await row(id).boundingBox(),b=await row(target).boundingBox();const y=after?b.y+b.height-4:b.y+4;
   await page.mouse.move(a.x+a.width/2,a.y+a.height/2);await page.mouse.down();await page.mouse.move(a.x+a.width/2,a.y+a.height/2+8,{steps:3});await page.mouse.move(b.x+b.width/2,y,{steps:10});
   if(escape)await page.keyboard.press('Escape');await page.mouse.up();await page.waitForTimeout(150);}
  // Hovering the near half of the next row would not move anything: no drop line until past its middle.
  {const a=await row('p-edge').boundingBox(),b=await row('p-core').boundingBox();await page.mouse.move(a.x+a.width/2,a.y+a.height/2);await page.mouse.down();await page.mouse.move(a.x+a.width/2,a.y+a.height/2+8,{steps:3});await page.mouse.move(b.x+b.width/2,b.y+3,{steps:6});
   assert.equal(await page.locator('.session-saved-drop-before,.session-saved-drop-after').count(),0,'no line over the near half of the next row');
   await page.mouse.move(b.x+b.width/2,b.y+b.height-3,{steps:4});assert.equal(await page.locator('#sessionSavedList [data-session-id="p-core"].session-saved-drop-after').count(),1,'line after the next row once past its middle');
   await page.keyboard.press('Escape');await page.mouse.up();await page.waitForTimeout(150);assert.deepEqual(await order(),['p-ap','p-edge','p-core']);report.checks.push('Drop line only where the move would change the order');}
  // Drag switch-core above ap-floor2.
  await drag('p-core','p-ap',false);assert.deepEqual(await order(),['p-core','p-ap','p-edge']);
  // The page saves 550 ms after the last change (saveState): wait for that
  // save, not a fixed pause shorter than it.
  await page.waitForFunction(()=>state.terminalSavedManualOrder===true);
  for(const t0=Date.now();!(puts.at(-1)?.terminalSavedManualOrder===true)&&Date.now()-t0<10000;)await page.waitForTimeout(25);
  const last=puts.at(-1);assert(last&&last.terminalSavedManualOrder===true,'manual order flag saved');assert.deepEqual(last.terminalProfiles.map(p=>p.id),['p-core','p-ap','p-edge'],'stored profile order follows the list');
  assert.equal(await page.locator('#modalRoot .modal').count(),0,'a drag does not open the connection');report.checks.push('Drag above reorders, persists the order and opens nothing');
  // Drag ap-floor2 below firewall-edge.
  await drag('p-ap','p-edge',true);assert.deepEqual(await order(),['p-core','p-edge','p-ap']);report.checks.push('Drag below reorders');
  // Escape cancels.
  await drag('p-ap','p-core',false,true);assert.deepEqual(await order(),['p-core','p-edge','p-ap'],'Escape cancels a drag');report.checks.push('Escape cancels');
  // Keyboard: Alt+Shift+Up moves the focused row up one.
  await row('p-ap').focus();await page.keyboard.press('Alt+Shift+ArrowUp');assert.deepEqual(await order(),['p-core','p-ap','p-edge']);
  assert.equal(await page.evaluate(()=>document.activeElement?.closest('[data-session-id]')?.dataset.sessionId),'p-ap','focus follows the moved row');report.checks.push('Alt+Shift+Up moves a focused row');
  // A plain click still selects; double-click still opens the connection dialog.
  assert.deepEqual(operations,[],'no drag or keyboard move opened a connection');
// Headless Chromium does not synthesize dblclick on these rows (same on main), so dispatch it.
  await row('p-edge').dispatchEvent('dblclick');await page.locator('#sessionLibraryAuthForm').waitFor();assert.deepEqual(operations,[],'sign-in is asked before any connection');report.checks.push('Double-click still opens the saved session (sign-in dialog for a password profile)');
  if(output)await page.screenshot({path:path.join(output,'saved-order.png')});
  report.errors=report.errors.filter(e=>!/Synthetic fixture/.test(e));assert.deepEqual(report.errors,[]);report.passed=true;
 }finally{if(output)fs.writeFileSync(path.join(output,'session-saved-order-report.json'),JSON.stringify(report,null,2));await browser.close();}
 console.log(report.checks.join('\n'));
})().catch(error=>{console.error(error);console.error(JSON.stringify(report.errors));process.exit(1);});
