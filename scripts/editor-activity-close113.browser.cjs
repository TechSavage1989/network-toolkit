'use strict';
// Isolated browser fixtures only: synthetic profiles/files, no device or vault.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT,report={passed:false,checks:[],errors:[]},child='c'.repeat(32),hash='a'.repeat(64);let browser;
const profiles=[{id:'verify',type:'ssh',name:'SSH verification',host:'verify.invalid',port:22,username:'fixture',authMethod:'password',credentialId:''},{id:'manual',type:'ssh',name:'SSH sign-in',host:'manual.invalid',port:22,username:'fixture',authMethod:'password',credentialId:''},{id:'locked',type:'ssh',name:'Locked vault',host:'locked.invalid',port:22,username:'fixture',authMethod:'password',credentialId:'synthetic-vault'}];
const ftp={id:'ftp',name:'Manual FTPS',host:'ftp.invalid',port:21,username:'fixture',security:'explicit',credentialId:''};
const catalog=[...profiles.map(p=>({id:'ssh:'+p.id,name:p.name,protocol:'ssh',host:p.host,port:p.port})),{id:'ftp:ftp',name:ftp.name,protocol:'ftps',host:ftp.host,port:ftp.port}];
const save=()=>fs.writeFileSync(path.join(out,'editor-activity-close113-results.json'),JSON.stringify(report,null,2)),check=name=>{report.checks.push(name);console.log('PASS '+name);save();};
async function start(width,detached=false){
 const ctx=await browser.newContext({viewport:{width,height:950}}),page=await ctx.newPage(),calls=[];page.setDefaultTimeout(15000);let activityRows=[],allowed=!detached,pending='',owned={},saved={activePage:'editor',appUpdatePreferences:{enabled:false},terminalProfiles:profiles,ftpProfiles:[ftp],theme:'light'},reviewRows=[];
 page.on('pageerror',e=>report.errors.push(e.message));
 await ctx.route('https://saved113.toolkit.test/**',async route=>{const request=route.request(),url=new URL(request.url()),p=url.pathname,b=request.postData()?request.postDataJSON():null;try{
  if(p.startsWith('/api/')){calls.push({path:p,method:request.method(),body:b});let data={};
   if(p==='/api/status')data={version:'1.17.113',os:'windows',arch:'amd64',editorWindowAvailable:true};
   else if(p==='/api/state'){if(detached)throw Error('Child requested state');if(request.method()==='GET')data=saved;else{saved=b;data={ok:true};}}
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/terminals')data={sessions:[]};
   else if(p==='/api/activity')data={tools:{editor:{documents:activityRows.filter(d=>d.loaded).length}}};
   else if(p==='/api/editor-views/current/activity'){assert.deepEqual(Object.keys(b),['documents']);assert(b.documents.every(d=>Object.keys(d).sort().join(',')==='lease,loaded,uid'));activityRows=b.documents;data={ok:true};}
   else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   else if(p==='/api/editor-views'||p==='/api/editor-views/current')data={askSaveBeforeClose:saved.editorPreferences?.askSaveBeforeClose!==false,available:true,self:detached?child:'',ownerInstanceId:'synthetic113',views:[],incoming:[],outgoing:[],owned,recovery:[],explorerRequests:[],savedConnectionRequests:reviewRows};
   else if(p==='/api/editor-views/current/state'){for(const d of b.batch.documents)owned[d.uid]=1;data={owned};}
   else if(p==='/api/editor-views/current/explorer')data={allowed,pending:false};
   else if(p==='/api/editor-views/saved-connections'){
    if(!allowed)return route.fulfill({status:403,contentType:'application/json',body:JSON.stringify({error:'Request file explorer access in the main window first.'})});
    data={profiles:catalog.filter(x=>!detached||x.protocol==='ssh'),pendingRequest:pending,detached};
   }else if(p==='/api/editor-views/saved-connections/request'){assert(detached&&allowed);assert.equal(Object.keys(b).join(','),'id');pending='b'.repeat(32);data={requestId:pending,pending:true};}
   else if(p==='/api/editor-views/saved-connections/cancel'){assert.equal(b.requestId,pending);pending='';data={ok:true};}
   else if(p==='/api/editor-views/saved-connections/review'){assert(!detached);data={connect:b.connect,profile:catalog[0],ok:true};reviewRows=[];}
   else if(p==='/api/terminals/known-hosts')data={hosts:profiles.filter(x=>x.id!=='verify').map(x=>({host:x.host+':22',fingerprint:'SHA256:known'}))};
   else if(p==='/api/credentials'){assert(!detached);data={vault:{available:true,state:'locked'},entries:[]};}
   else if(p==='/api/credential-vault'){assert(!detached);assert.deepEqual(b,{action:'unlock'});return route.fulfill({status:409,contentType:'application/json',body:JSON.stringify({error:'Native unlock cancelled (synthetic fixture).'})});}
   else if(p==='/api/editor-views/current/prepare-close'||p==='/api/editor-views/current/cancel-close')data={ok:true};
   else if(p==='/api/transfer/local')data={path:'C:/Synthetic',parent:'C:/',entries:[]};
   else if(p==='/api/remote-browser'){
    if(request.method()==='GET')data={sessions:[],connections:[]};else if(b.action==='fingerprint'){assert(!detached);data={fingerprint:'SHA256:observed'};}else throw Error('Unexpected network connection/file action '+b.action);
   }else if(p.includes('/events')){await new Promise(r=>setTimeout(r,100));data={events:[],sessions:[],after:0};}
   else if(p.startsWith('/api/editor-views/documents/')&&p.endsWith('/release'))data={ok:true};
   else if(request.method()!=='GET')throw Error('Unexpected mutation '+p);
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  const html=p==='/'||p.startsWith('/editor-view/');let file=path.join(root,'web',html?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let body=fs.readFileSync(file);
  if(html){body=body.toString();assert(body.includes('editor-saved-connections.js'),'actual product script include missing');body=body.replaceAll('__SESSION_TOKEN__','synthetic113').replaceAll('__WINDOW_MODE__',detached?'editor':'main').replaceAll('__EDITOR_VIEW__',detached?JSON.stringify({id:child,ownerInstanceId:'synthetic113'}):'null').replaceAll('__TERMINAL_VIEW__','null').replaceAll('src="assets/','src="/assets/').replaceAll('href="assets/','href="/assets/').replaceAll('src="../assets/','src="/assets/').replaceAll('href="../assets/','href="/assets/');}
  return route.fulfill({body,contentType:html?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
 }catch(e){report.errors.push(e.message);return route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:e.message})});}});
 await page.goto('https://saved113.toolkit.test/'+(detached?'editor-view/'+child:''));await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
 await page.evaluate(async()=>{navigate('editor');await NetworkToolkitFileEditor.newDocument();const d=NetworkToolkitFileEditor.active;window.saved113Doc=d;window.saved113Editor=document.querySelector('#networkToolkitFileEditor');});
 const text=page.locator('[data-ed-text]');await text.fill('UNSAVED_SYNTHETIC_113\nretain this text');await text.dispatchEvent('input');
 return {page,ctx,calls,preference:value=>{saved.editorPreferences={askSaveBeforeClose:value}},allow:value=>{allowed=value},request:()=>{reviewRows=[{viewId:child,name:'Fixture child',requestId:'b'.repeat(32),profile:catalog[0]}]}};
}
async function retained(page){assert(await page.evaluate(()=>NetworkToolkitFileEditor.active===saved113Doc&&document.querySelector('#networkToolkitFileEditor')===saved113Editor&&NetworkToolkitFileEditor.active.content.includes('UNSAVED_SYNTHETIC_113')&&NetworkToolkitFileEditor.active.dirty));}
async function choose(page,id){
 const close=page.locator('[data-editor-files-close]');if(await close.isVisible())await close.click();
 const toggle=page.locator('[data-ed-panel-toggle="remote"]');if(await toggle.getAttribute('aria-expanded')!=='true')await toggle.click();
 await page.locator('[data-ed-page="refreshRemote"]').click();await page.waitForFunction(id=>[...document.querySelector('#ntEditorRemoteSource').options].some(o=>o.value==='saved:'+id),id);
 await page.locator('#ntEditorRemoteSource').selectOption('saved:'+id);await page.locator('[data-ed-page="connectSaved"]').click();
}
(async()=>{assert(out,'Set isolated TOOLKIT_TEST_OUTPUT');fs.mkdirSync(out,{recursive:true});const watchdog=setTimeout(()=>{report.failure='Editor activity/close fixture exceeded five minutes';save();process.exit(1)},300000);
 try{browser=await chromium.launch({headless:true});
 for(const width of [1440,390]){const f=await start(width),{page}=f;
  const indicator=page.locator('#navigation [data-page="editor"]');await page.waitForFunction(()=>document.querySelector('#navigation [data-page="editor"]')?.getAttribute('data-activity-summary')==='1 loaded document');
  assert((await indicator.getAttribute('aria-label')).includes('1 loaded document'));await page.locator('.nt-editor-more>summary').click();const box=page.locator('[data-ed-ask-save-close]');assert(await box.isChecked());
  await box.click();await page.locator('.nt-editor-dialog').waitFor({state:'visible'});assert(await box.isChecked(),'Policy remains enabled until disabling is explicitly confirmed');await page.locator('[data-ed-dialog-action="cancel"]').click();assert(await box.isChecked());await retained(page);
  await box.click();await page.locator('.nt-editor-dialog').waitFor({state:'visible'});assert(await box.isChecked());assert.equal(await page.locator('[data-ed-dialog-action="accept"]').textContent(),'Allow discard');await page.locator('[data-ed-dialog-action="accept"]').click();await page.waitForFunction(()=>state.editorPreferences.askSaveBeforeClose===false);assert(!await box.isChecked());await retained(page);
  await page.screenshot({path:path.join(out,'editor-close113-preference-'+width+'.png')});
  const original=await page.evaluate(()=>NetworkToolkitFileEditor.active.content);
  for(const flag of ['saving','frozen']){assert.equal(await page.evaluate(async flag=>{const d=NetworkToolkitFileEditor.active;d[flag]=true;try{return await NetworkToolkitFileEditor.closeAll()}finally{d[flag]=false}},flag),false);}
  for(const flag of ['loading','formatting','reloading']){assert.equal(await page.evaluate(async flag=>{const d=NetworkToolkitFileEditor.active;d[flag]=true;try{return await NetworkToolkitFileEditor.windowAdapter.prepareWindowClose()}finally{d[flag]=false}},flag),false);}
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.content),original);
  await page.evaluate(()=>{const d=NetworkToolkitFileEditor.active;d.untitled=false;d.source='Local computer';d.write=async()=>{throw Error('SYNTHETIC_SAVE_FAILURE')};});await page.locator('[data-ed="save"]').click();await page.waitForFunction(()=>NetworkToolkitFileEditor.active.saveFailed===true);
  await page.locator('[data-ed-close-tab]').first().click();await page.locator('.nt-editor-dialog').waitFor({state:'visible'});assert((await page.locator('.nt-editor-dialog').textContent()).includes('previous save failed'));await page.locator('[data-ed-dialog-action="cancel"]').click();await retained(page);
  await page.screenshot({path:path.join(out,'editor-close113-save-failure-'+width+'.png')});
  await page.evaluate(()=>{const d=NetworkToolkitFileEditor.active;d.write=async()=>({hash:'b'.repeat(64)});});await page.locator('[data-ed="save"]').click();await page.waitForFunction(()=>NetworkToolkitFileEditor.active.saveFailed===false&&!NetworkToolkitFileEditor.active.saving);
  assert.equal(await page.evaluate(async()=>{const d=NetworkToolkitFileEditor.active;d.dirty=true;const a=NetworkToolkitFileEditor.windowAdapter;const ok=await a.prepareWindowClose();if(!ok)return false;const retained=NetworkToolkitFileEditor.documents.size===1&&d.frozen;a.cancelWindowClose();return retained&&!d.frozen}),true);
  await page.locator('[data-ed-close-tab]').first().click();await page.waitForFunction(()=>NetworkToolkitFileEditor.documents.size===0);await page.waitForFunction(()=>!document.querySelector('#navigation [data-page="editor"] .sidebar-activity-dot'));
  await page.evaluate(()=>NetworkToolkitFileEditor.newDocument());await page.waitForTimeout(1200);assert(!await indicator.locator('.sidebar-activity-dot').count());
  await page.locator('[data-ed-text]').fill('touched draft');await page.locator('[data-ed-text]').dispatchEvent('input');await page.waitForFunction(()=>!!document.querySelector('#navigation [data-page="editor"] .sidebar-activity-dot'));await page.locator('[data-ed="undo"]').click();await page.waitForFunction(()=>NetworkToolkitFileEditor.active.content==='');await page.waitForTimeout(1200);assert(await indicator.locator('.sidebar-activity-dot').count());
  check(width+' loaded badge, untouched/touched empty draft, close warning persistence, failed-save/active-operation guards and nondestructive native prepare');await f.ctx.close();
 }
 const f=await start(1440,true),{page}=f;f.preference(false);await page.evaluate(()=>NetworkToolkitEditorWindows.poll());await page.locator('.nt-editor-more>summary').click();const box=page.locator('[data-ed-ask-save-close]');await box.waitFor({state:'visible'});await page.waitForFunction(()=>document.querySelector('.nt-editor-more>summary')?.getAttribute('aria-expanded')==='true'&&document.querySelector('[data-ed-ask-save-close]')?.checked===false);assert(await box.isDisabled());assert((await page.locator('[data-ed-close-policy-note]').textContent()).includes('main window'));assert(!f.calls.some(c=>c.path==='/api/state'));
 await page.screenshot({path:path.join(out,'editor-close113-child-policy.png')});check('child receives readonly main preference without a settings-write capability');await f.ctx.close();assert.deepEqual(report.errors,[]);report.passed=true;save();
 }catch(e){report.failure=e.stack;save();throw e}finally{clearTimeout(watchdog);await browser?.close();}
})().catch(e=>{console.error(e);process.exitCode=1});
