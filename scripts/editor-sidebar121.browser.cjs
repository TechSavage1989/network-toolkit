'use strict';
// Full product sidebar: actual controls and geometry, synthetic retained transports.
// Baseline 120 is measured separately; no credentials, live files or native automation.
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const current=path.resolve(__dirname,'..'),baseline=process.env.NTK_SIDEBAR121_BASELINE_DIR,out=process.env.TOOLKIT_TEST_OUTPUT,origin='https://file-sidebar121.toolkit.test';
const child='c'.repeat(32),connection='synthetic-file-sidebar121',session='synthetic-terminal-sidebar121';
const LOCAL='C:/Synthetic121',REMOTE='/synthetic121',RB='application/x-networktoolkit-remote-browser',TS='application/x-networktoolkit-sidebar-sftp';
const report={passed:false,checks:[],cases:[],errors:[],limits:['Synthetic authenticated API responses; no real files, live network, Windows Explorer or native OLE drag-out tested.','This browser fixture does not validate native Windows Explorer delivery; separate Windows acceptance is required.']};
let browser,lastPage;
const watchdog=setTimeout(()=>{report.failure='File-row drag acceptance exceeded three minutes';if(out){fs.mkdirSync(out,{recursive:true});save();}process.exit(1);},180000);
const save=()=>fs.writeFileSync(path.join(out,'editor-sidebar121-results.json'),JSON.stringify(report,null,2));
function listing(side,folder){const base=side==='local'?LOCAL:REMOTE,location=folder&&folder!=='.'?folder:base;return {path:location,parent:base,roots:['C:/'],drives:[{path:'C:/',label:'C: Synthetic drive',kind:'local'}],entries:location.endsWith('-folder')?[{name:'child.txt',path:location+'/child.txt',directory:false,size:21,modified:'2026-10-30T00:00:00Z'}]:[{name:side+'-file.txt',path:base+'/'+side+'-file.txt',directory:false,size:21,modified:'2026-10-30T00:00:00Z'},{name:side+'-folder',path:base+'/'+side+'-folder',directory:true,size:0,modified:'2026-10-30T00:00:00Z'}]};}
async function setup(root,{surface='editor',width=1440,theme='light'}={}){
 const detached=surface==='detached',context=await browser.newContext({viewport:{width,height:950}}),page=await context.newPage();lastPage=page;page.setDefaultTimeout(15000);
 const operations=[],owned={},sessions=[{id:session,type:'ssh',host:'terminal.invalid',port:22,username:'synthetic',label:'Synthetic drag terminal',state:'connected',cols:80,rows:24}],connections=[{id:connection,host:'files.invalid',port:22,username:'synthetic',connected:true}];let closed=false,saved={activePage:surface==='terminal'?'terminal':surface==='full'?'sftp':'editor',theme,appUpdatePreferences:{enabled:false},terminalSidebarCollapsed:false,terminalSidebarTab:'saved',transferModes:{sftp:'client'},fileTransferProtocol:'sftp',editorFilesPreferences:{panel:'files',side:'local',collapsed:false}};
 page.on('pageerror',e=>report.errors.push(e.message));
 await context.route(origin+'/**',async route=>{try{
  if(await handleEditorActivityMetadata(route,origin,report))return;
  const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method(),body=request.postData()?request.postDataJSON():null;
  if(p.startsWith('/api/')){let data={};
   if(p==='/api/status')data={version:require(path.join(root,'package.json')).version,os:'windows',arch:'amd64',editorWindowAvailable:true};
   else if(p==='/api/state'){assert(!detached,'Detached editor must not read/write application state');if(method==='GET')data=saved;else{assert.equal(method,'PUT');saved=body;data={ok:true};}}
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/activity')data={tools:{}};
   else if(p==='/api/terminals'){assert.equal(method,'GET');data={sessions};}
   else if(p==='/api/terminals/events'){await new Promise(r=>setTimeout(r,80));data={events:[],sessions,after:0};}
   else if(p==='/api/terminal-views')data={available:false,owners:{},views:[]};
   else if(p==='/api/terminals/'+session+'/resize'){assert.equal(method,'POST');data={ok:true};}
   else if(p==='/api/editor-views'||p==='/api/editor-views/current')data={available:true,self:detached?child:'',ownerInstanceId:'synthetic121',views:[],incoming:[],outgoing:[],owned,recovery:[],explorerRequests:[],savedConnectionRequests:[],dragRequests:[]};
   else if(p==='/api/editor-views/current/state'){assert.equal(method,'POST');for(const d of body.batch.documents)owned[d.uid]=1;data={owned};}
   else if(p==='/api/editor-views/current/explorer'){assert.equal(method,'GET','No new delegation requested');data={allowed:true,pending:false};}
   else if(p==='/api/editor-views/saved-connections')data={profiles:[],pendingRequest:'',detached};
   else if(p==='/api/credentials'){assert(!detached);data={vault:{available:true,state:'locked'},entries:[]};}
   else if(p==='/api/transfer/local'){assert.equal(method,'GET');data=listing('local',url.searchParams.get('path'));}
   else if(p==='/api/transfer/local-files'){assert.equal(body.action,'mkdir');assert(body.path.startsWith(LOCAL+'/remote-folder'));operations.push({path:p,body});data={ok:true};}
   else if(p==='/api/remote-browser'){
    if(method==='GET')data={sessions,connections};
    else{assert.equal(body.protocol,'ssh');assert(body.connectionId===connection||body.sessionId===session,'Only fixture-owned existing transport');assert(!Object.keys(body).some(k=>/password|credential|privateKey|passphrase/.test(k)),'No credentials in file operations');
     if(body.action==='probe')data={supported:true};
     else if(body.action==='list'){if(body.path==='/synthetic-denied')return route.fulfill({status:403,contentType:'application/json',body:JSON.stringify({error:'Synthetic permission denied; choose an accessible folder.'})});data=listing('remote',body.path);}
     else if(['upload','download','mkdir'].includes(body.action)){operations.push({path:p,body});if(body.action!=='mkdir')assert.equal(body.overwrite,false,'Drag copies must retain default no-overwrite protection');data={ok:true};}
     else throw Error('Unexpected remote action '+body.action);
    }
   }else if(p==='/api/remote-browser/drag-ticket'){assert.equal(method,'POST');assert.equal(body.action,'stat');data={ticket:'d'.repeat(48),filename:'synthetic-file.txt'};}
   else if(p.includes('/events'))data={events:[],sessions:[],after:0};
   else if(method!=='GET')throw Error('Unexpected mutation '+method+' '+p);
   return await route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  const html=p==='/'||p==='/editor-view/'+child;let file=path.join(root,'web',html?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let bytes=fs.readFileSync(file);
  if(html){bytes=bytes.toString();for(const name of ['editor-drag.js','remote-browser.js','terminal-sftp-sidebar.js'])assert(bytes.includes(name),'Full production script missing: '+name);bytes=bytes.replaceAll('__SESSION_TOKEN__','synthetic121').replaceAll('__WINDOW_MODE__',detached?'editor':'main').replaceAll('__EDITOR_VIEW__',detached?JSON.stringify({id:child,ownerInstanceId:'synthetic121'}):'null').replaceAll('__TERMINAL_VIEW__','null').replaceAll('src="assets/','src="/assets/').replaceAll('href="assets/','href="/assets/').replaceAll('src="../assets/','src="/assets/').replaceAll('href="../assets/','href="/assets/');}
  return await route.fulfill({body:bytes,contentType:html?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.png')?'image/png':'application/octet-stream'});
 }catch(e){if(!closed){report.errors.push(e.stack||e.message);await route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture rejected operation'})}).catch(()=>{});}}});
 await page.addInitScript(theme=>{sessionStorage.setItem('networkToolkit.theme',theme);},theme);await page.goto(origin+(detached?'/editor-view/'+child:'/'));await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(theme=>{setTheme(theme);},theme);
 await page.evaluate(async({connection,surface,remote})=>{
  // Seed a fixture-owned, already-authenticated connection, not a new login.
  const browser=NetworkToolkitRemoteBrowser,m=browser.get('ssh');Object.assign(m,{saved:true,connected:true,connectionId:connection,borrowedConnection:true,sourceChoice:'connection:'+connection});Object.assign(browser.configurations.ssh,{host:'files.invalid',port:22,username:'synthetic'});
  if(surface==='full'){state.transferModes={...state.transferModes,sftp:'client'};navigate('sftp');browser.render('ssh');}
  if(surface!=='terminal')await browser.load('ssh','remote',remote);
 },{connection,surface,remote:REMOTE});
 if(surface==='editor'||surface==='detached'){
  await page.evaluate(async()=>{navigate('editor');await NetworkToolkitFileEditor.newDocument();window.sidebar121Document=NetworkToolkitFileEditor.active;});
  const ta=page.locator('[data-ed-text]');await ta.fill('SYNTHETIC SIDEBAR121 DIRTY DOCUMENT\nKeep edits intact');await ta.dispatchEvent('input');
  const panel=page.locator('[data-ed-files-sidebar]');if(!await panel.isVisible())await page.locator('[data-editor-files-reopen]').click();
  if(await page.locator('[data-editor-files-panel="files"]').getAttribute('aria-selected')!=='true')await page.locator('[data-editor-files-panel="files"]').click();
 }else if(surface==='terminal'){
  await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(()=>NetworkToolkitTerminalSFTP.available(currentTerminal()));await page.locator('[data-session-tab="files"]').click();await page.locator('[data-tsftp-item]').first().waitFor({state:'visible'});
 }
 await page.waitForFunction(()=>[...document.images].every(i=>i.complete&&i.naturalWidth>0));
 return {page,context,operations,async close(){closed=true;await context.close();}};
}
async function settle(page){await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));}
async function panel(page,name){const tab=page.locator('[data-editor-files-panel="'+name+'"]');if(await tab.getAttribute('aria-selected')!=='true'||!await page.locator('[data-ed-files-sidebar]').isVisible())await tab.click();await page.waitForFunction(name=>document.querySelector('[data-rb-root="ssh"]')?.dataset.sidebarPanel===name,name);await settle(page);}
async function prepare(f,detached){
 await panel(f.page,'files');await f.page.locator('[data-editor-files-side="remote"]').click();await f.page.locator('[data-rb-rows="remote"] tr').first().waitFor();
 if(!detached)await f.page.evaluate(async session=>{await NetworkToolkitRemoteBrowser.connectTerminal('ssh',session);},session);
 await settle(f.page);
}
async function geometry(page){return page.evaluate(()=>{
 const app=document.querySelector('.is-editor-sidebar[data-rb-root="ssh"]'),box=e=>{if(!e||!e.getClientRects().length)return null;const r=e.getBoundingClientRect(),s=getComputedStyle(e);return {x:r.x,y:r.y,width:r.width,height:r.height,bottom:r.bottom,marginTop:parseFloat(s.marginTop)||0,marginBottom:parseFloat(s.marginBottom)||0};};
 return {list:box(app.querySelector('[data-rb-scroll="remote"]')),hint:box(app.querySelector('[data-rb-drop-destination]')),feedback:box(app.querySelector('[data-rb-feedback]')),overflow:document.documentElement.scrollWidth-innerWidth};
});}
async function controls(f,detached,id){
 const page=f.page;await panel(page,'connections');
 // A standalone authenticated client has editable fields, unlike a borrowed terminal.
 if(!detached){await page.evaluate(connection=>{const b=NetworkToolkitRemoteBrowser,m=b.get('ssh');m.sessionId='';m.connectionId=connection;m.borrowedConnection=true;b.render('ssh');},connection);await settle(page);}
 const toolbar=page.locator('.nt-editor-files-session-tools'),summary=toolbar.locator('.nt-editor-files-session-menu>summary'),gear=toolbar.locator('[data-rb-action="connectionSettings"]');
 const dimensions=await toolbar.evaluate(el=>[...el.querySelectorAll(':scope>button,:scope>.nt-editor-files-session-menu>summary')].filter(x=>x.getClientRects().length).map(x=>{const r=x.getBoundingClientRect();return {label:x.getAttribute('aria-label'),top:r.top,width:r.width,height:r.height};}));
 assert(dimensions.length>=2);assert(Math.max(...dimensions.map(x=>x.top))-Math.min(...dimensions.map(x=>x.top))<=1,JSON.stringify(dimensions));assert(dimensions.every(x=>x.height===29&&x.width===29),JSON.stringify(dimensions));
 if(detached)assert(!await gear.isVisible(),'Child cannot edit connection credentials');else{
  await gear.click();await page.waitForFunction(()=>document.activeElement?.matches('[data-rb-setting="host"]'));
  await gear.focus();await page.keyboard.press('Enter');await page.waitForFunction(()=>document.activeElement?.matches('[data-rb-setting="host"]'));
 }
 await summary.click();await page.waitForFunction(()=>document.querySelector('.nt-editor-files-session-menu')?.open);
 const copy=toolbar.locator('.nt-editor-files-session-menu [data-rb-action="copyEndpoint"]');await copy.waitFor({state:'visible'});
 assert(await copy.evaluate(el=>{const r=el.getBoundingClientRect();return el.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));}),'More action must be painted/hit-testable, not clipped');
 await page.screenshot({path:path.join(out,'editor-sidebar121-actions-'+id+'.png')});
 await copy.click();await page.locator('[data-transfer-location-dialog]').waitFor({state:'visible'});await page.locator('[data-action="close-modal"]').last().click();
 await summary.focus();await page.keyboard.press('Enter');await page.waitForFunction(()=>document.querySelector('.nt-editor-files-session-menu')?.open);await page.keyboard.press('Escape');assert.equal(await summary.evaluate(el=>el.parentElement.open),false);assert(await summary.evaluate(el=>document.activeElement===el));
 await summary.press('Space');await page.waitForFunction(()=>document.querySelector('.nt-editor-files-session-menu')?.open);await page.locator('[data-editor-files-panel="files"]').click();assert.equal(await summary.evaluate(el=>el.parentElement.open),false);await panel(page,'files');
 assert(await page.evaluate(()=>NetworkToolkitFileEditor.active===sidebar121Document&&NetworkToolkitFileEditor.active.dirty&&NetworkToolkitFileEditor.active.content.includes('SYNTHETIC SIDEBAR121 DIRTY DOCUMENT')));
 return dimensions;
}
(async()=>{
 try{assert(out&&baseline,'TOOLKIT_TEST_OUTPUT and exact120 NTK_SIDEBAR121_BASELINE_DIR required');assert.equal(require(path.join(baseline,'package.json')).version,'1.17.120');report.source={baselineVersion:'1.17.120',assets:Object.fromEntries(['remote-browser.js','file-editor.css'].map(name=>[name,{before:crypto.createHash('sha256').update(fs.readFileSync(path.join(baseline,'web/assets',name))).digest('hex'),after:crypto.createHash('sha256').update(fs.readFileSync(path.join(current,'web/assets',name))).digest('hex')}]))};fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true});
  for(const width of [1440,390])for(const theme of ['light','dark'])for(const detached of [false,true]){
   const id=(detached?'detached':'main')+'-'+width+'-'+theme;let old,now;const row={id,passed:false};report.cases.push(row);
   try{
    old=await setup(baseline,{surface:detached?'detached':'editor',width,theme});await prepare(old,detached);row.before=await geometry(old.page);assert(row.before.hint?.height>0,'Baseline renders redundant instruction');if(!detached)assert(row.before.feedback?.height>0,'Baseline renders routine connected feedback');await old.page.screenshot({path:path.join(out,'editor-sidebar121-before-'+id+'.png')});
    if(id==='main-1440-light'){
     await panel(old.page,'connections');const gear=old.page.locator('.nt-editor-files-session-tools [data-rb-action="connectionSettings"]');await gear.click();row.baselineGearFocusedSettings=await old.page.evaluate(()=>document.activeElement?.matches('[data-rb-setting="host"],#ntEditorRemoteSource')||false);assert.equal(row.baselineGearFocusedSettings,false,'Baseline gear never focuses settings');
     const summary=old.page.locator('.nt-editor-files-session-menu>summary');await summary.click();row.baselineMenu=await summary.evaluate(el=>{const menu=el.parentElement,copy=menu.querySelector('[data-rb-action="copyEndpoint"]'),r=copy.getBoundingClientRect();return {open:menu.open,marginTop:getComputedStyle(menu).marginTop,overflow:getComputedStyle(menu).overflow,hit:copy.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))};});assert.equal(row.baselineMenu.open,true);assert.equal(row.baselineMenu.hit,false);await old.page.screenshot({path:path.join(out,'editor-sidebar121-before-actions-'+id+'.png')});
    }await old.close();old=null;
    now=await setup(current,{surface:detached?'detached':'editor',width,theme});await prepare(now,detached);row.after=await geometry(now.page);assert.equal(row.after.hint,null);assert.equal(row.after.feedback,null);assert(row.after.list.height>row.before.list.height+20,JSON.stringify(row));row.heightGain=row.after.list.height-row.before.list.height;assert(row.after.overflow<=1);
    const endpoint=now.page.locator('[data-rb-editor-connections]');assert.equal(await endpoint.getAttribute('data-connected'),'true');assert((await endpoint.getAttribute('aria-label')).startsWith('Connected · SFTP'));const badge=await endpoint.locator('span').evaluate(el=>{const s=getComputedStyle(el,'::before');return {content:s.content,width:s.width,height:s.height,color:s.backgroundColor};});assert.equal(badge.width,'7px');assert.notEqual(badge.content,'none');row.badge=badge;
    await now.page.screenshot({path:path.join(out,'editor-sidebar121-after-'+id+'.png')});row.controls=await controls(now,detached,id);
    // Real error remains visible; presence clears only when the actual connection is lost.
    await now.page.evaluate(()=>NetworkToolkitRemoteBrowser.load('ssh','remote','/synthetic-denied'));await now.page.locator('[data-rb-feedback].error').waitFor({state:'visible'});assert((await now.page.locator('[data-rb-feedback]').innerText()).includes('Synthetic permission denied'));
    await now.page.evaluate(()=>{const b=NetworkToolkitRemoteBrowser;b.invalidateConnection('ssh',{release:false});b.render('ssh');});assert.equal(await now.page.locator('[data-rb-editor-connections]').getAttribute('data-connected'),'false');assert.equal(await now.page.locator('[data-rb-drop-destination]').count(),0);
    assert.equal(now.operations.length,0,'No transfers/authentication as a consequence of these UI actions');row.passed=true;report.checks.push(id+' measured '+row.heightGain+'px gain, actual More and gear activation, active-only badge, errors and dirty document retained');console.log('PASS '+report.checks.at(-1));save();
   }catch(e){row.failure=e.stack;if(now?.page&&!now.page.isClosed())await now.page.screenshot({path:path.join(out,'editor-sidebar121-failure-'+id+'.png')}).catch(()=>{});save();}finally{await old?.close();await now?.close();}
  }
  assert(report.cases.every(x=>x.passed),JSON.stringify(report.cases.filter(x=>!x.passed)));assert.deepEqual(report.errors,[]);report.passed=true;save();
 }catch(error){report.failure=error.stack;save();throw error;}finally{clearTimeout(watchdog);await browser?.close();}
})();
