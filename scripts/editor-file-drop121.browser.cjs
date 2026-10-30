'use strict';
// Full product page, genuine pointer gestures, synthetic files/transports only.
// This catches document-wide drag listeners that isolated native fixtures miss.
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const current=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT,origin='https://editor-file-drop121.toolkit.test';
const child='c'.repeat(32),connection='synthetic-editor-file-drop121',session='synthetic-terminal-drop121';
const LOCAL='C:/Synthetic121',REMOTE='/synthetic121',RB='application/x-networktoolkit-remote-browser',TS='application/x-networktoolkit-sidebar-sftp';
const report={passed:false,checks:[],cases:[],errors:[],limits:['Synthetic authenticated API responses; no real files, live network, Windows Explorer or native OLE drag-out tested.','This fixture proves internal HTML file drops only; native pointer/OLE behavior has separate validation.']};
let browser,lastPage;
const watchdog=setTimeout(()=>{report.failure='Editor file drop acceptance exceeded three minutes';if(out){fs.mkdirSync(out,{recursive:true});save();}process.exit(1);},180000);
const save=()=>fs.writeFileSync(path.join(out,'editor-file-drop121-results.json'),JSON.stringify(report,null,2));
function listing(side,folder){const base=side==='local'?LOCAL:REMOTE,location=folder&&folder!=='.'?folder:base;return {path:location,parent:base,roots:['C:/'],drives:[{path:'C:/',label:'C: Synthetic drive',kind:'local'}],entries:location.endsWith('-folder')?[{name:'child.txt',path:location+'/child.txt',directory:false,size:21,modified:'2026-10-30T00:00:00Z'}]:[{name:side+'-file.txt',path:base+'/'+side+'-file.txt',directory:false,size:21,modified:'2026-10-30T00:00:00Z'},{name:side+'-folder',path:base+'/'+side+'-folder',directory:true,size:0,modified:'2026-10-30T00:00:00Z'},...(side==='local'?[{name:'binary.bin',path:base+'/binary.bin',directory:false,size:42,modified:'2026-10-30T00:00:00Z'}]:[])]};}
async function setup(root,{surface='full',width=1440}={}){
 const detached=surface==='detached',context=await browser.newContext({viewport:{width,height:950}}),page=await context.newPage();lastPage=page;page.setDefaultTimeout(15000);
 const operations=[],owned={},sessions=[{id:session,type:'ssh',host:'terminal.invalid',port:22,username:'synthetic',label:'Synthetic drag terminal',state:'connected',cols:80,rows:24}],connections=[{id:connection,host:'files.invalid',port:22,username:'synthetic',connected:true}];let closed=false,saved={activePage:surface==='terminal'?'terminal':surface==='full'?'sftp':'editor',theme:width===390?'dark':'light',appUpdatePreferences:{enabled:false},terminalSidebarCollapsed:false,terminalSidebarTab:'saved',transferModes:{sftp:'client'},fileTransferProtocol:'sftp',editorFilesPreferences:{panel:'files',side:'local',collapsed:false}};
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
   else if(p==='/api/file-editor/local'){assert.equal(body.action,'read-text');operations.push({path:p,body});data=body.path.endsWith('.bin')?{binary:true}:{content:'SYNTHETIC_OPEN121 '+body.path+'\n',hash:'local-hash',encoding:'UTF-8'};}
   else if(p==='/api/transfer/local-files'){assert.equal(body.action,'mkdir');assert(body.path.startsWith(LOCAL+'/remote-folder'));operations.push({path:p,body});data={ok:true};}
   else if(p==='/api/remote-browser'){
    if(method==='GET')data={sessions,connections};
    else{assert.equal(body.protocol,'ssh');assert(body.connectionId===connection||body.sessionId===session,'Only fixture-owned existing transport');assert(!Object.keys(body).some(k=>/password|credential|privateKey|passphrase/.test(k)),'No credentials in file operations');
     if(body.action==='probe')data={supported:true};
     else if(body.action==='list')data=listing('remote',body.path);
     else if(body.action==='read-text'){operations.push({path:p,body});data={content:'SYNTHETIC_OPEN121 '+body.path+'\n',hash:'remote-hash',encoding:'UTF-8'};}
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
 await page.goto(origin+(detached?'/editor-view/'+child:'/'));await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
 await page.evaluate(async({connection,surface,remote})=>{
  // Seed a fixture-owned, already-authenticated connection, not a new login.
  const browser=NetworkToolkitRemoteBrowser,m=browser.get('ssh');Object.assign(m,{saved:true,connected:true,connectionId:connection,borrowedConnection:true,sourceChoice:'connection:'+connection});Object.assign(browser.configurations.ssh,{host:'files.invalid',port:22,username:'synthetic'});
  if(surface==='full'){state.transferModes={...state.transferModes,sftp:'client'};navigate('sftp');browser.render('ssh');}
  if(surface!=='terminal')await browser.load('ssh','remote',remote);
 },{connection,surface,remote:REMOTE});
 if(surface==='editor'||surface==='detached'){
  await page.evaluate(async()=>{navigate('editor');await NetworkToolkitFileEditor.newDocument();window.drop121Document=NetworkToolkitFileEditor.active;});
  const ta=page.locator('[data-ed-text]');await ta.fill('SYNTHETIC DROP121 DIRTY DOCUMENT\nKeep edits intact');await ta.dispatchEvent('input');
  const panel=page.locator('[data-ed-files-sidebar]');if(!await panel.isVisible())await page.locator('[data-editor-files-reopen]').click();
  if(await page.locator('[data-editor-files-panel="files"]').getAttribute('aria-selected')!=='true')await page.locator('[data-editor-files-panel="files"]').click();
 }else if(surface==='terminal'){
  await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(()=>NetworkToolkitTerminalSFTP.available(currentTerminal()));await page.locator('[data-session-tab="files"]').click();await page.locator('[data-tsftp-item]').first().waitFor({state:'visible'});
 }
 await page.waitForFunction(()=>[...document.images].every(i=>i.complete&&i.naturalWidth>0));
 await page.evaluate(()=>{
  window.drop121Events=[];
  for(const name of ['dragstart','drop','dragend'])document.addEventListener(name,event=>{
   const row=event.target.closest?.('[data-rb-item],[data-tsftp-item]'),values={};for(const type of event.dataTransfer?.types||[])if(type.startsWith('application/x-networktoolkit-'))values[type]=event.dataTransfer.getData(type);
   drop121Events.push({type:name,trusted:event.isTrusted,prevented:event.defaultPrevented,row:row?.dataset.rbItem||row?.dataset.tsftpItem||'',targetTag:event.target.tagName,targetEditor:!!event.target.closest?.('.nt-editor-document-area'),targetPane:event.target.closest?.('[data-rb-pane]')?.dataset.rbPane||'',types:[...(event.dataTransfer?.types||[])],values,effectAllowed:event.dataTransfer?.effectAllowed,dropEffect:event.dataTransfer?.dropEffect});
  },true);
 });
 return {page,context,operations,async close(){closed=true;await context.close();}};
}
async function realDrag(f,selector,{cancel=true,target=null,expectCancelled=false,mime=RB,reject=false,beforeDrop=null}={}){
 const {page}=f,row=page.locator(selector);await row.waitFor({state:'visible'});await row.scrollIntoViewIfNeeded();const before=await page.evaluate(()=>drop121Events.length),box=await row.boundingBox();assert(box&&box.width>20&&box.height>10,'Visible source row');
 // Start on the file name, avoiding text selection and the icon's SVG edge.
 const x=box.x+Math.min(75,box.width/2),y=box.y+box.height/2;await page.mouse.move(x,y);await page.mouse.down();await page.mouse.move(x+9,y+3,{steps:4});await page.mouse.move(x+31,y+14,{steps:8});
 await page.waitForFunction(n=>drop121Events.slice(n).some(e=>e.type==='dragstart'),before);
 const start=await page.evaluate(n=>drop121Events.slice(n).find(e=>e.type==='dragstart'),before);assert.equal(start.trusted,true,'Browser must generate the dragstart');assert.equal(start.prevented,expectCancelled,'Non-tab drag cancellation');assert(start.types.includes(mime),'File handler owns its MIME payload');assert(!start.types.includes('application/x-network-toolkit-editor-tab'),'No editor ownership ticket');const payload=JSON.parse(start.values[mime]);assert.equal(payload.files.length,1);assert.equal(payload.files[0].path,start.row);assert.equal(start.effectAllowed,'copy');
 if(cancel||expectCancelled){await page.keyboard.press('Escape');await page.mouse.up();}
 else{if(beforeDrop)await beforeDrop();const destination=page.locator(target);await destination.waitFor({state:'visible'});const point=await destination.evaluate(node=>{const r=node.getBoundingClientRect(),left=Math.max(0,r.left),right=Math.min(innerWidth,r.right),top=Math.max(0,r.top),bottom=Math.min(innerHeight,r.bottom);for(const y of [Math.min(bottom-8,top+150),(top+bottom)/2,top+8])for(const x of [right-8,(left+right)/2,left+8]){const hit=document.elementFromPoint(x,y);if(x>left&&x<right&&y>top&&y<bottom&&(hit===node||node.contains(hit)))return {x,y,rect:{left,right,top,bottom}};}return null;});assert(point,'Actual editor text surface must have an exposed drop point, not a center hidden beneath the compact Files panel');const tx=point.x,ty=point.y;await page.mouse.move(tx,ty,{steps:16});await page.mouse.move(tx,ty);if(reject){await page.keyboard.press('Escape');await page.mouse.up();}else{await page.mouse.up();await page.waitForFunction(n=>drop121Events.slice(n).some(e=>e.type==='drop'&&e.trusted),before);await page.waitForFunction(()=>!NetworkToolkitRemoteBrowser.get('ssh').busy);}}
 return {start,events:await page.evaluate(n=>drop121Events.slice(n),before)};
}
async function selectSide(page,side){const button=page.locator('[data-editor-files-side="'+side+'"]');if(await button.getAttribute('aria-selected')!=='true')await button.click();await page.locator('[data-rb-pane="'+side+'"] [data-rb-item]').first().waitFor({state:'visible'});}

async function runCase({surface,width,theme}){
 const f=await setup(current,{surface,width}),{page}=f,id=surface+'-'+width+'-'+theme,rows=[];
 try{
  await page.evaluate(theme=>{state.theme=theme;document.documentElement.dataset.theme=theme;},theme);
  const snapshot=await page.evaluate(()=>{const d=NetworkToolkitFileEditor.active,ta=document.querySelector('[data-ed-text]');ta.setSelectionRange(3,9);ta.scrollTop=0;return {uid:d.uid,text:d.content,undo:[...d.undo],count:NetworkToolkitFileEditor.documents.size};});
  for(const side of ['local','remote']){
   await selectSide(page,side);const path=(side==='local'?LOCAL:REMOTE)+'/'+side+'-file.txt',selector='[data-rb-item="'+path+'"]',before=f.operations.length;
   const gesture=await realDrag(f,selector,{cancel:false,target:'.nt-editor-document-area [data-ed-text]'});
   await page.waitForFunction(path=>NetworkToolkitFileEditor.active?.path===path&&!NetworkToolkitFileEditor.active.loading,path);
   assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.content),'SYNTHETIC_OPEN121 '+path+'\n');
   assert.equal(f.operations.slice(before).filter(op=>op.body.action==='read-text').length,1,'One existing validated read, no transfer');
   const opened=await page.evaluate(()=>({uid:NetworkToolkitFileEditor.active.uid,count:NetworkToolkitFileEditor.documents.size}));
   // Repeating the genuine drop activates the same dirty tab, without a reread.
   await page.locator('.nt-editor-document-area [data-ed-text]').fill('SYNTHETIC_OPEN121 DIRTY '+side);
   const edits=await page.evaluate(()=>{const d=NetworkToolkitFileEditor.active;return {text:d.content,undo:[...d.undo]};}),beforeRepeat=f.operations.length;
   await realDrag(f,selector,{cancel:false,target:'.nt-editor-document-area [data-ed-text]'});
   assert.deepEqual(await page.evaluate(()=>({uid:NetworkToolkitFileEditor.active.uid,count:NetworkToolkitFileEditor.documents.size})),opened);
   assert.deepEqual(await page.evaluate(()=>({text:NetworkToolkitFileEditor.active.content,undo:[...NetworkToolkitFileEditor.active.undo]})),edits);
   assert.equal(f.operations.length,beforeRepeat,'Already-open dirty document must not reread or write');
   rows.push({side,path,gesture,opened});
  }
  const beforeReject=f.operations.length,count=await page.evaluate(()=>NetworkToolkitFileEditor.documents.size);
  await selectSide(page,'local');
  await realDrag(f,'[data-rb-item="'+LOCAL+'/local-folder"]',{cancel:false,reject:true,target:'.nt-editor-document-area [data-ed-text]'});
  assert.match(await page.locator('[data-ed-message]').innerText(),/Folders cannot/);
  assert.equal(f.operations.length,beforeReject);
  await realDrag(f,'[data-rb-item="'+LOCAL+'/binary.bin"]',{cancel:false,target:'.nt-editor-document-area [data-ed-text]'});
  await page.waitForFunction(()=>document.querySelector('[data-ed-message]').textContent.includes('Binary'));
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.documents.size),count,'Binary read cannot leave a broken tab');
  await selectSide(page,'remote');const beforeStale=f.operations.length;
  await realDrag(f,'[data-rb-item="'+REMOTE+'/remote-file.txt"]',{cancel:false,reject:true,target:'.nt-editor-document-area [data-ed-text]',beforeDrop:()=>page.evaluate(()=>{NetworkToolkitRemoteBrowser.get('ssh').connectionEpoch++;})});
  assert.match(await page.locator('[data-ed-message]').innerText(),/SFTP connection changed/);assert.equal(f.operations.length,beforeStale,'Stale source must not read or reconnect');
  const original=await page.evaluate(uid=>{const d=[...NetworkToolkitFileEditor.documents.values()].find(d=>d.uid===uid);NetworkToolkitFileEditor.activateDocument(d.id);return {uid:d.uid,text:d.content,undo:[...d.undo],dirty:d.dirty};},snapshot.uid);
  assert.deepEqual(original,{uid:snapshot.uid,text:snapshot.text,undo:snapshot.undo,dirty:true});
  assert(f.operations.every(op=>op.body.action==='read-text'),'No writes, transfers or connection creation');
  await page.screenshot({path:path.join(out,'editor-file-drop121-'+id+'.png')});
  report.cases.push({id,passed:true,rows,operations:f.operations,dirtyOriginalRetained:true});report.checks.push(id+': genuine local/SFTP drop opens and reuses dirty tabs; folder/binary/stale guards');save();
 }catch(error){report.cases.push({id,passed:false,rows,operations:f.operations,failure:error.stack});await page.screenshot({path:path.join(out,'editor-file-drop121-'+id+'-failure.png')}).catch(()=>{});save();throw error;}finally{await f.close();}
}
(async()=>{assert(out,'TOOLKIT_TEST_OUTPUT required');fs.mkdirSync(out,{recursive:true});report.sourceSHA256=crypto.createHash('sha256').update(fs.readFileSync(path.join(current,'web/assets/file-editor.js'))).digest('hex');browser=await chromium.launch({headless:true});try{
 const failures=[];for(const surface of ['editor','detached'])for(const width of [1440,390]){try{await runCase({surface,width,theme:width===390?'dark':'light'});}catch(error){failures.push(error.message);}}
 assert.deepEqual(report.errors,[]);assert.deepEqual(failures,[],'Every actual main/detached drop case must pass');report.passed=true;save();
 }finally{clearTimeout(watchdog);await browser.close();}})().catch(error=>{clearTimeout(watchdog);report.failure=error.stack||String(error);if(out){fs.mkdirSync(out,{recursive:true});save();}console.error(error);process.exitCode=1;});
