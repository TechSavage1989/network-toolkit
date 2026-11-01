'use strict';
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const root=path.resolve(__dirname,'..'),output=process.env.TOOLKIT_TEST_OUTPUT,origin='https://terminal116.toolkit.test',report={passed:false,checks:[],errors:[],operations:[],clickTimings:[]};let browser,lastPage;
async function setup(mode,width){
 const context=await browser.newContext({viewport:{width,height:900}}),page=await context.newPage();lastPage=page;page.setDefaultTimeout(15000);
 let sessions=['one','two'].map(id=>({id:'click116-'+id,type:'ssh',host:'192.0.2.'+(id==='one'?1:2),port:22,username:'synthetic',label:'Synthetic '+id,state:'connected',cols:80,rows:24}));
 const operations=[];page.on('pageerror',e=>report.errors.push(e.message));
 await page.route(origin+'/**',async route=>{try{if(await handleEditorActivityMetadata(route,origin,report))return;const req=route.request(),p=new URL(req.url()).pathname,method=req.method();if(p.startsWith('/api/')){let data={};if(method!=='GET')operations.push({path:p,method});
  if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64'};
  else if(p==='/api/state')data={activePage:'terminal',appUpdatePreferences:{enabled:false},terminalSidebarCollapsed:true,terminalConfirmClose:true};
  else if(p==='/api/jobs')data=[];
  else if(p==='/api/terminals'){assert.equal(method,'GET','No connection creation');data={sessions};}
  else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
  else if(p==='/api/terminals/events'){await new Promise(r=>setTimeout(r,80));data={sessions,events:[],after:0};}
  else if(p==='/api/activity')data={tools:{}};
  else if(p==='/api/remote-browser'&&method==='POST'){const body=req.postDataJSON();assert.deepEqual(Object.keys(body).sort(),['action','protocol','sessionId']);assert.equal(body.protocol,'ssh');assert.equal(body.action,'probe');assert(['click116-one','click116-two'].includes(body.sessionId));data={supported:false};}
  else if(p.match(/^\/api\/terminals\/click116-(one|two)\/resize$/)){assert.equal(method,'POST');}
  else if(p.match(/^\/api\/terminals\/click116-(one|two)\/disconnect$/)){assert.equal(method,'POST');const id=p.split('/')[3];sessions=sessions.map(s=>s.id===id?{...s,state:'closed'}:s);}
  else if(p.match(/^\/api\/terminals\/click116-(one|two)$/)){assert.equal(method,'DELETE');sessions=sessions.filter(s=>s.id!==p.split('/')[3]);}
  else if(method!=='GET'&&!(p==='/api/state'&&method==='PUT'))throw Error('Unexpected mutation '+method+' '+p);
  return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
 }
 let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let bytes=fs.readFileSync(file);if(p==='/')bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','synthetic116').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__','null').replaceAll('__EDITOR_VIEW__','null');return route.fulfill({body:bytes,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.png')?'image/png':'application/octet-stream'});
 }catch(e){report.errors.push(e.message);await route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture rejected operation'})});}});
 await page.goto(origin);await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(()=>terminalSessions.size===2);await page.waitForFunction(()=>[...document.images].every(i=>i.complete&&i.naturalWidth>0));
 await page.evaluate(async()=>{for(const e of terminalSessions.values())await new Promise(resolve=>e.xterm.write('SYNTHETIC retained output '+e.info.id+'\r\n',resolve));});
 return {page,context,operations};
}
(async()=>{if(output)fs.mkdirSync(output,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 for(const mode of ['main','terminal'])for(const width of [1280,390]){
  const f=await setup(mode,width),p=f.page,tab=id=>p.locator('[data-terminal-tab="'+id+'"]'),x=id=>p.locator('[data-terminal-close="'+id+'"]');
  for(const id of ['click116-two','click116-one']){await tab(id).click();await p.waitForFunction(id=>terminalActive===id,id);assert.equal(await tab(id).getAttribute('aria-selected'),'true');}
  assert(!f.operations.some(o=>/disconnect|reconnect/.test(o.path)));
  // No-drag X uses the existing confirmation, Cancel retains both sessions.
  await x('click116-two').click();await p.locator('[data-terminal-remove-confirm="click116-two"]').waitFor({state:'visible'});assert(!f.operations.some(o=>o.method==='DELETE'||/disconnect/.test(o.path)));await p.locator('#modalRoot [data-action="close-modal"]').first().click();assert.equal(await p.evaluate(()=>terminalSessions.size),2);
  await x('click116-two').click();await p.locator('[data-terminal-remove-confirm="click116-two"]').click();await p.waitForFunction(()=>!terminalSessions.has('click116-two'));assert.deepEqual(f.operations.filter(o=>o.method==='DELETE'||/disconnect/.test(o.path)).map(o=>[o.method,o.path]),[['POST','/api/terminals/click116-two/disconnect'],['DELETE','/api/terminals/click116-two']]);
  // One draft gives a second reorder target without opening a connection.
  const draft=await p.evaluate(()=>{newTerminalTab();return terminalActive;});await tab('click116-one').click();await p.waitForFunction(()=>terminalActive==='click116-one');const before=await p.evaluate(()=>terminalSessions.get('click116-one').xterm.buffer.active.getLine(0).translateToString(true));
  await p.evaluate(id=>{const t=window.__terminal116ClickTiming={};document.addEventListener('pointerup',e=>{if(e.isTrusted&&t.release===undefined)t.release=performance.now();},true);document.addEventListener('pointerdown',e=>{if(e.isTrusted&&e.target.closest('[data-terminal-close]')?.dataset.terminalClose===id)t.freshDown=performance.now();},true);},draft);
  await p.evaluate(()=>{window.__uxDragTrace=[];for(const type of ['pointerdown','pointermove','pointerup','lostpointercapture'])document.addEventListener(type,event=>{if(window.__uxDragTrace.length>=70)return;window.__uxDragTrace.push({type,x:Math.round(event.clientX),y:Math.round(event.clientY),target:event.target?.getAttribute?.('data-terminal-tab')||event.target?.className||'',buttons:event.buttons});},true);});
  const a=await tab('click116-one').boundingBox(),b=await tab(draft).boundingBox();assert(a&&b);const y=a.y+a.height/2;await p.mouse.move(a.x+Math.min(a.width/2,35),y);await p.mouse.down();await p.mouse.move(b.x+b.width-3,y,{steps:12});await p.mouse.up();const tabOrder=await p.locator('#terminalTabs [data-terminal-tab]').evaluateAll(nodes=>nodes.map(n=>n.dataset.terminalTab));
  if(JSON.stringify(tabOrder)!==JSON.stringify([draft,'click116-one'])){const probe=await p.evaluate(()=>({trace:window.__uxDragTrace,order:NetworkToolkitTerminalTabDrag?.ids?.(),rects:[...document.querySelectorAll('#terminalTabs .terminal-tab')].map(t=>({text:t.innerText,rect:t.getBoundingClientRect().toJSON(),buttons:[...t.querySelectorAll('button')].map(b=>({name:b.getAttribute('aria-label'),rect:b.getBoundingClientRect().toJSON()}))}))}));console.log('TERMINAL_DRAG116_PROBE '+JSON.stringify({mode,width,a,b,y,tabOrder,probe}));}
  assert.deepEqual(tabOrder,[draft,'click116-one']);
  // A fresh click immediately after the drag must not inherit its suppression.
  const closeBox=await x(draft).boundingBox();assert(closeBox);await p.mouse.click(closeBox.x+closeBox.width/2,closeBox.y+closeBox.height/2);const clickDelay=await p.evaluate(()=>__terminal116ClickTiming.freshDown-__terminal116ClickTiming.release);report.clickTimings.push({mode,width,dragReleaseToFreshCloseDownMs:clickDelay});assert(Number.isFinite(clickDelay)&&clickDelay>=0&&clickDelay<500,'Timing-invalid fresh-click trial: expected trusted new X down within500ms, observed '+clickDelay);await p.waitForFunction(id=>!terminalDraftTabs.has(id),draft);assert.equal(await p.evaluate(()=>terminalSessions.get('click116-one').xterm.buffer.active.getLine(0).translateToString(true)),before);
  await x('click116-one').click();await p.locator('[data-terminal-remove-confirm="click116-one"]').waitFor({state:'visible'});await p.locator('#modalRoot [data-action="close-modal"]').first().click();assert(await p.evaluate(()=>terminalSessions.has('click116-one')));
  if(output)await p.screenshot({path:path.join(output,'terminal-tab-click116-'+mode+'-'+width+'.png')});report.checks.push(mode+' '+width+': actual tab selection, no-drag X confirmation/Cancel/close, reorder then immediate X, preserved remaining output and no reconnect');report.operations.push({mode,width,operations:f.operations});await f.context.close();
 }
 assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(async e=>{report.failure=e.stack||String(e);console.error(report.failure);if(output&&lastPage&&!lastPage.isClosed())await lastPage.screenshot({path:path.join(output,'terminal-tab-click116-failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(output)fs.writeFileSync(path.join(output,'terminal-tab-click116-results.json'),JSON.stringify(report,null,2));await browser?.close();});
