'use strict';
// Isolated browser fixtures only: synthetic profiles/files, no device or vault.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const current=path.resolve(__dirname,'..'),baseline=process.env.NTK_FILES_BASELINE_DIR,out=process.env.TOOLKIT_TEST_OUTPUT,report={passed:false,checks:[],errors:[],layouts:[],failures:[]},child='c'.repeat(32),hash='a'.repeat(64);let browser,activePage;
const profiles=[{id:'verify',type:'ssh',name:'SSH verification',host:'verify.invalid',port:22,username:'fixture',authMethod:'password',credentialId:''},{id:'manual',type:'ssh',name:'SSH sign-in',host:'manual.invalid',port:22,username:'fixture',authMethod:'password',credentialId:''},{id:'locked',type:'ssh',name:'Locked vault',host:'locked.invalid',port:22,username:'fixture',authMethod:'password',credentialId:'synthetic-vault'}];
const ftp={id:'ftp',name:'Manual FTPS',host:'ftp.invalid',port:21,username:'fixture',security:'explicit',credentialId:''};
const catalog=[...profiles.map(p=>({id:'ssh:'+p.id,name:p.name,protocol:'ssh',host:p.host,port:p.port})),{id:'ftp:ftp',name:ftp.name,protocol:'ftps',host:ftp.host,port:ftp.port}];
const save=()=>fs.writeFileSync(path.join(out,'editor-local119-results.json'),JSON.stringify(report,null,2)),check=name=>{report.checks.push(name);console.log('PASS '+name);save();};
async function start(root,width,detached=false){
 const ctx=await browser.newContext({viewport:{width,height:950}}),page=await ctx.newPage(),calls=[];activePage=page;page.setDefaultTimeout(15000);let allowed=!detached,explorerPending=false,pending='',owned={},saved={activePage:'editor',appUpdatePreferences:{enabled:false},terminalProfiles:profiles,ftpProfiles:[ftp],theme:'light'},reviewRows=[],localFiles=new Map([['C:/Synthetic/synthetic-0.txt','SYNTHETIC LOCAL119 SOURCE']]);
 page.on('pageerror',e=>report.errors.push(e.message));
 await ctx.route('https://saved113.toolkit.test/**',async route=>{const request=route.request(),url=new URL(request.url()),p=url.pathname,b=request.postData()?request.postDataJSON():null;try{
  if(p.startsWith('/api/')){calls.push({path:p,method:request.method(),body:b});let data={};
   if(p==='/api/status')data={version:require(path.join(root,'package.json')).version,os:'windows',arch:'amd64',editorWindowAvailable:true};
   else if(p==='/api/state'){if(detached)throw Error('Child requested state');if(request.method()==='GET')data=saved;else{saved=b;data={ok:true};}}
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/terminals')data={sessions:[]};
   else if(p==='/api/activity')data={tools:{}};
   else if(p==='/api/editor-views/current/activity'){assert.deepEqual(Object.keys(b),['documents']);assert(b.documents.every(d=>Object.keys(d).sort().join(',')==='lease,loaded,uid'));data={ok:true};}
   else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   else if(p==='/api/editor-views'||p==='/api/editor-views/current')data={available:true,self:detached?child:'',ownerInstanceId:'synthetic113',views:[],incoming:[],outgoing:[],owned,recovery:[],explorerRequests:[],savedConnectionRequests:reviewRows};
   else if(p==='/api/editor-views/current/state'){for(const d of b.batch.documents)owned[d.uid]=1;data={owned};}
   else if(p==='/api/editor-views/current/explorer'){if(request.method()==='POST')explorerPending=true;data={allowed,pending:explorerPending};}
   else if(p==='/api/editor-views/saved-connections'){
    if(!allowed)return route.fulfill({status:403,contentType:'application/json',body:JSON.stringify({error:'Request file explorer access in the main window first.'})});
    data={profiles:catalog.filter(x=>!detached||x.protocol==='ssh'),pendingRequest:pending,detached};
   }else if(p==='/api/editor-views/saved-connections/request'){assert(detached&&allowed);assert.equal(Object.keys(b).join(','),'id');pending='b'.repeat(32);data={requestId:pending,pending:true};}
   else if(p==='/api/editor-views/saved-connections/cancel'){assert.equal(b.requestId,pending);pending='';data={ok:true};}
   else if(p==='/api/editor-views/saved-connections/review'){assert(!detached);data={connect:b.connect,profile:catalog[0],ok:true};reviewRows=[];}
   else if(p==='/api/terminals/known-hosts')data={hosts:profiles.filter(x=>x.id!=='verify').map(x=>({host:x.host+':22',fingerprint:'SHA256:known'}))};
   else if(p==='/api/credentials'){assert(!detached);data={vault:{available:true,state:'locked'},entries:[]};}
   else if(p==='/api/credential-vault'){assert(!detached);assert.deepEqual(b,{action:'unlock'});return route.fulfill({status:409,contentType:'application/json',body:JSON.stringify({error:'Native unlock cancelled (synthetic fixture).'})});}
   else if(p==='/api/file-editor/local'){assert.equal(b.action,'read-text','child raw writes forbidden');data={path:b.path,content:localFiles.get(b.path)||'',hash,encoding:'UTF-8',bom:false,eol:'LF'};}
   else if(/^\/api\/editor-views\/documents\/[^/]+\/file$/.test(p)){assert.equal(b.action,'write-text');assert.equal(b.lease,1);assert(b.path.startsWith('C:/Synthetic/'));localFiles.set(b.path,b.content);data={path:b.path,hash,bytes:b.content.length,encoding:'UTF-8'};}
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
 return {page,ctx,calls,localFiles,allow:value=>{allowed=value},request:()=>{reviewRows=[{viewId:child,name:'Fixture child',requestId:'b'.repeat(32),profile:catalog[0]}]}};
}

(async()=>{assert(out,'Isolated output required');fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true});try{
 for(const width of [1440,390])for(const theme of ['light','dark']){const f=await start(current,width,true),{page}=f,id='child-'+width+'-'+theme;try{
 await page.evaluate(theme=>setTheme(theme),theme);const host=page.locator('[data-ed-files-sidebar]');if(!await host.isVisible())await page.locator('[data-editor-files-reopen]').click();
 const row=page.locator('[data-rb-rows="local"] [data-rb-item]').filter({hasText:'synthetic-0.txt'}).first();await row.waitFor();assert(f.calls.some(c=>c.path==='/api/transfer/local'));assert(!f.calls.some(c=>c.path==='/api/editor-views/current/explorer'&&c.method==='POST'));assert.equal(await page.locator('[data-editor-files-request]').count(),0);
 await row.dblclick();await page.waitForFunction(()=>NetworkToolkitFileEditor.active?.path==='C:/Synthetic/synthetic-0.txt');await page.waitForFunction(()=>!NetworkToolkitEditorWindows.state.running);
 const geometry=()=>page.evaluate(()=>{const rect=node=>{const r=node.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height};},save=document.querySelector('[data-ed=save]'),r=save.getBoundingClientRect(),hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2),shell=document.querySelector('.nt-editor-files-shell');return {sidebarOpen:!document.querySelector('[data-ed-files-sidebar]').hidden,shellPosition:getComputedStyle(shell).position,shell:rect(shell),save:rect(save),saveHit:hit?.outerHTML?.slice(0,500),saveReceivesPointer:hit===save||save.contains(hit)};});
 const opened=await geometry();if(width<=680){await page.screenshot({path:path.join(out,'editor-local119-'+id+'-browse.png')});assert.equal(opened.shellPosition,'absolute','Compact Files is the existing explicit drawer');await page.locator('[data-editor-files-close]').click();await host.waitFor({state:'hidden'});}
 const editing=await geometry();assert(editing.saveReceivesPointer,'Save is reachable after normal Files collapse: '+JSON.stringify(editing));report.layouts.push({id,opened,editing});
 const ta=page.locator('[data-ed-text]');await ta.click();await ta.fill('SYNTHETIC LOCAL119 EDIT');await ta.dispatchEvent('input');await page.locator('[data-ed="save"]').click();await page.waitForFunction(()=>NetworkToolkitFileEditor.active.dirty===false&&!NetworkToolkitFileEditor.active.saving);assert.equal(f.localFiles.get('C:/Synthetic/synthetic-0.txt'),'SYNTHETIC LOCAL119 EDIT');
 assert(await page.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].includes(saved113Doc)&&saved113Doc.dirty&&saved113Doc.content.includes('UNSAVED_SYNTHETIC_113')));
 if(!await host.isVisible())await page.locator('[data-editor-files-reopen]').click();await page.locator('[data-rb-drive="D:/"]').click();await page.waitForFunction(()=>NetworkToolkitRemoteBrowser.get('ssh').local.path==='D:/');
 await page.screenshot({path:path.join(out,'editor-local119-'+id+'.png')});
 await page.locator('[data-editor-files-panel="connections"]').click();await page.locator('[data-editor-files-request]').waitFor({state:'visible'});assert(!f.calls.some(c=>c.path==='/api/editor-views/current/explorer'&&c.method==='POST'));await page.locator('[data-editor-files-panel="files"]').click();await page.locator('[data-rb-drive="D:/"]').waitFor();
 assert(!f.calls.some(c=>['/api/state','/api/credentials','/api/credential-vault'].includes(c.path)));assert(!f.calls.some(c=>c.path==='/api/remote-browser'&&c.method==='POST'));check(id+' direct local browse/drives/open/lease-save; dirty sibling retained; remote permission independent');
 }catch(error){report.failures.push({id,failure:error.stack||String(error)});await page.screenshot({path:path.join(out,'editor-local119-'+id+'-failure.png')}).catch(()=>{});save();}finally{await f.ctx.close();}}
 assert.deepEqual(report.failures,[],'Every detached local Files case must pass');assert.deepEqual(report.errors,[]);report.passed=true;save();}catch(e){report.failure=e.stack;save();if(activePage&&!activePage.isClosed())await activePage.screenshot({path:path.join(out,'editor-local119-failure.png')}).catch(()=>{});throw e;}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
