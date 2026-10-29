'use strict';
// Isolated browser fixtures only: synthetic profiles/files, no device or vault.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT,report={passed:false,checks:[],errors:[]},child='c'.repeat(32),hash='a'.repeat(64);let browser;
const profiles=[{id:'verify',type:'ssh',name:'SSH verification',host:'verify.invalid',port:22,username:'fixture',authMethod:'password',credentialId:''},{id:'manual',type:'ssh',name:'SSH sign-in',host:'manual.invalid',port:22,username:'fixture',authMethod:'password',credentialId:''},{id:'locked',type:'ssh',name:'Locked vault',host:'locked.invalid',port:22,username:'fixture',authMethod:'password',credentialId:'synthetic-vault'}];
const ftp={id:'ftp',name:'Manual FTPS',host:'ftp.invalid',port:21,username:'fixture',security:'explicit',credentialId:''};
const catalog=[...profiles.map(p=>({id:'ssh:'+p.id,name:p.name,protocol:'ssh',host:p.host,port:p.port})),{id:'ftp:ftp',name:ftp.name,protocol:'ftps',host:ftp.host,port:ftp.port}];
const save=()=>fs.writeFileSync(path.join(out,'editor-files115-results.json'),JSON.stringify(report,null,2)),check=name=>{report.checks.push(name);console.log('PASS '+name);save();};
async function start(width,detached=false){
 const ctx=await browser.newContext({viewport:{width,height:950}}),page=await ctx.newPage(),calls=[];page.setDefaultTimeout(15000);let allowed=!detached,pending='',owned={},saved={activePage:'editor',appUpdatePreferences:{enabled:false},terminalProfiles:profiles,ftpProfiles:[ftp],theme:'light'},reviewRows=[];
 page.on('pageerror',e=>report.errors.push(e.message));
 await ctx.route('https://saved113.toolkit.test/**',async route=>{const request=route.request(),url=new URL(request.url()),p=url.pathname,b=request.postData()?request.postDataJSON():null;try{
  if(p.startsWith('/api/')){calls.push({path:p,method:request.method(),body:b});let data={};
   if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',editorWindowAvailable:true};
   else if(p==='/api/state'){if(detached)throw Error('Child requested state');if(request.method()==='GET')data=saved;else{saved=b;data={ok:true};}}
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/terminals')data={sessions:[]};
   else if(p==='/api/activity')data={tools:{}};
   else if(p==='/api/editor-views/current/activity'){assert.deepEqual(Object.keys(b),['documents']);assert(b.documents.every(d=>Object.keys(d).sort().join(',')==='lease,loaded,uid'));data={ok:true};}
   else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   else if(p==='/api/editor-views'||p==='/api/editor-views/current')data={available:true,self:detached?child:'',ownerInstanceId:'synthetic113',views:[],incoming:[],outgoing:[],owned,recovery:[],explorerRequests:[],savedConnectionRequests:reviewRows};
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
   else if(p==='/api/transfer/local'){const folder=url.searchParams.get('path')||'C:/Synthetic';if(folder==='Z:/')return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Synthetic disconnected drive is unavailable.'})});data={path:folder,parent:folder.startsWith('D:')?'D:/':'C:/',roots:['C:/','D:/','Z:/'],drives:[{path:'C:/',label:'C: Local disk',kind:'local'},{path:'D:/',label:'D: Synthetic removable',kind:'removable'},{path:'C:/Mounted/',label:'Mounted synthetic volume',kind:'local'},{path:'Z:/',label:'Z: Offline mapping',kind:'network'}],entries:Array.from({length:80},(_,i)=>({name:'synthetic-'+i+'.txt',path:folder+'/synthetic-'+i+'.txt',directory:false,size:30,modified:'2026-10-29T00:00:00Z'}))};}
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
 return {page,ctx,calls,allow:value=>{allowed=value},request:()=>{reviewRows=[{viewId:child,name:'Fixture child',requestId:'b'.repeat(32),profile:catalog[0]}]}};
}
async function retained(page){assert(await page.evaluate(()=>NetworkToolkitFileEditor.active===saved113Doc&&document.querySelector('#networkToolkitFileEditor')===saved113Editor&&NetworkToolkitFileEditor.active.content.includes('UNSAVED_SYNTHETIC_113')&&NetworkToolkitFileEditor.active.dirty));}
async function choose(page,id){
 const files=page.locator('[data-ed="files"]');if(await files.getAttribute('aria-expanded')!=='true')await files.click();
 await page.locator('[data-editor-files-side="remote"]').click();
 await page.locator('[data-ed-page="refreshRemote"]').click();await page.waitForFunction(id=>[...document.querySelector('#ntEditorRemoteSource').options].some(o=>o.value==='saved:'+id),id);
 await page.locator('#ntEditorRemoteSource').selectOption('saved:'+id);await page.locator('[data-ed-page="connectSaved"]').click();
}

(async()=>{assert(out,'TOOLKIT_TEST_OUTPUT required');fs.mkdirSync(out,{recursive:true});const timeout=setTimeout(()=>{report.failure='Files115 exceeded four minutes';save();process.exit(1)},240000);
 try{browser=await chromium.launch({headless:true});
 for(const detached of [false,true])for(const width of [1440,390]){
  const f=await start(width,detached),{page}=f,mode=detached?'detached':'main';
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  const sidebar=page.locator('[data-ed-files-sidebar]'),toggle=page.locator('[data-ed="files"]');
  if(await toggle.getAttribute('aria-expanded')!=='true')await page.locator('[data-editor-files-reopen]').click();
  if(detached){await page.locator('[data-editor-files-request]').waitFor({state:'visible'});assert(!f.calls.some(c=>c.path==='/api/transfer/local'));f.allow(true);await page.locator('[data-editor-files-panel="connections"]').click();await page.locator('[data-editor-files-panel="files"]').click();}
  await page.locator('[data-rb-drive="D:/"]').waitFor({state:'visible'});
  const names=await page.locator('[data-rb-drive]').allTextContents();assert.equal(new Set(names).size,4);assert(names.every(n=>/[A-Z]:/.test(n)));await page.locator('[data-rb-drive="D:/"]').click();await page.waitForFunction(()=>document.querySelector('[data-rb-path="local"]').value==='D:/');assert.equal(await page.locator('[data-rb-drive="D:/"]').getAttribute('aria-pressed'),'true');
  await page.locator('[data-rb-drive="Z:/"]').click();await page.getByText('Synthetic disconnected drive is unavailable.',{exact:false}).first().waitFor();assert.equal(await page.locator('[data-rb-drive="D:/"]').getAttribute('aria-pressed'),'true');await page.locator('[data-rb-drive="C:/"]').click();await page.waitForFunction(()=>document.querySelector('[data-rb-path="local"]').value==='C:/');
  await page.locator('[data-rb-scroll="local"]').evaluate(el=>el.scrollTop=320);const original=await page.evaluate(()=>{window.files115Original=document.querySelector('[data-rb-root]');window.files115Input=document.querySelector('[data-rb-setting="password"]');return NetworkToolkitRemoteBrowser.get('ssh').local.path;});
  await page.locator('[data-editor-files-panel="connections"]').click();assert.equal(await page.locator('[data-rb-pane="local"]').isVisible(),false);assert.equal(await page.locator('#ntEditorRemoteSource').isVisible(),true);
  if(!detached){const password=page.locator('[data-rb-setting="password"]');await password.fill('SYNTHETIC_TEMPORARY_INPUT');await password.dispatchEvent('input');}
  await page.locator('[data-editor-files-panel="files"]').click();assert.equal(await page.locator('.nt-editor-files-connection').isVisible(),false);assert.equal(await page.locator('#ntEditorRemoteSource').isVisible(),false);assert(await page.evaluate(()=>document.querySelector('[data-rb-root]')===files115Original&&document.querySelector('[data-rb-setting="password"]')===files115Input));assert.equal(await page.locator('[data-rb-path="local"]').inputValue(),original);
  await page.locator('[data-editor-files-panel="connections"]').click();if(!detached){assert.equal(await page.locator('[data-rb-setting="password"]').inputValue(),'SYNTHETIC_TEMPORARY_INPUT');await page.locator('[data-rb-setting="password"]').fill('');await page.locator('[data-rb-setting="password"]').dispatchEvent('input');}
  await page.locator('[data-editor-files-panel="files"]').click();const g=await page.locator('[data-rb-scroll="local"]').evaluate(el=>{const r=el.getBoundingClientRect(),side=el.closest('[data-ed-files-sidebar]').getBoundingClientRect();return {height:r.height,sidebarHeight:side.height,right:r.right,outer:document.documentElement.scrollWidth-innerWidth};});assert(g.height>=200,JSON.stringify(g));assert(g.outer<=1,JSON.stringify(g));(report.geometry??=[]).push({mode,width,...g});
  const before=await page.locator('.nt-editor-document-area').boundingBox();await page.locator('[data-editor-files-close]').click();assert.equal(await sidebar.isVisible(),false);const after=await page.locator('.nt-editor-document-area').boundingBox();if(width>680)assert(after.width>=before.width+200);await page.locator('[data-editor-files-reopen]').click();await page.locator('[data-rb-scroll="local"]').waitFor({state:'visible'});await retained(page);assert(!f.calls.some(c=>c.body?.action==='connect'));
  await page.evaluate(theme=>setTheme(theme),width===390?'dark':'light');await page.waitForFunction(()=>[...document.images].every(i=>i.complete&&i.naturalWidth>0));await page.screenshot({path:path.join(out,'editor-files115-'+mode+'-'+width+'.png')});
  await page.locator('[data-editor-files-panel="connections"]').click();await page.screenshot({path:path.join(out,'editor-connections115-'+mode+'-'+width+'.png')});
  if(detached)assert(!f.calls.some(c=>c.path==='/api/state'||c.path==='/api/credentials'));else{assert(await page.evaluate(()=>state.editorFilesPreferences.panel==='connections'));}
  check(mode+' '+width+' named drives, unavailable fallback, persistent tabs/forms, directory height, collapse and dirty retention');await f.ctx.close();
 }
 assert.deepEqual(report.errors,[]);report.passed=true;save();
 }catch(e){report.failure=e.stack;save();throw e;}finally{clearTimeout(timeout);await browser?.close();}
})();
