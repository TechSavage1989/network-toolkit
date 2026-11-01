'use strict';
// Hosted-only, synthetic route fixture. No real connection, listener, vault or UAC.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT,child='c'.repeat(32);
const report={passed:false,checks:[],cases:[],errors:[],sourceHashes:{}},connectPath='M8 3v5 M16 3v5 M6 8h12v4a6 6 0 0 1-12 0z M12 18v3';let browser;
const save=()=>fs.writeFileSync(path.join(out,'connect-sidebar118-results.json'),JSON.stringify(report,null,2));
const settle=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
async function start({width=1440,theme='light',detached=false,preference}={}){
 const context=await browser.newContext({viewport:{width,height:950}}),page=await context.newPage(),calls=[];let owned={};page.setDefaultTimeout(12000);
 page.on('pageerror',e=>report.errors.push(e.message));
 if(detached&&preference)await context.addInitScript(value=>sessionStorage.setItem('ntk.editor.files',JSON.stringify(value)),preference);
 await context.route('**/*',async route=>{const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method();let body;try{
  assert.equal(url.origin,'https://connect118.toolkit.test','fixture must not request outside its synthetic origin');body=request.postData()?request.postDataJSON():null;
  if(p.startsWith('/api/')){calls.push({path:p,method,body});let data={};
   if(p==='/api/status')data={version:require(path.join(root,'package.json')).version,os:'windows',arch:'amd64',editorWindowAvailable:true};
   else if(p==='/api/state'){assert(!detached,'child must not access main state');data=method==='GET'?{activePage:'overview',theme,appUpdatePreferences:{enabled:false}}:{ok:true};}
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/terminals')data={sessions:[]};
   else if(p==='/api/activity')data={tools:{}};
   else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   else if(p==='/api/editor-views'||p==='/api/editor-views/current')data={available:true,self:detached?child:'',ownerInstanceId:'synthetic118',views:[],incoming:[],outgoing:[],owned,recovery:[],explorerRequests:[],savedConnectionRequests:[]};
   else if(p==='/api/editor-views/current/state'){for(const doc of body.batch.documents)owned[doc.uid]=1;data={owned};}
   else if(p==='/api/editor-views/current/activity'){assert.deepEqual(Object.keys(body),['documents']);assert(body.documents.every(d=>Object.keys(d).sort().join(',')==='lease,loaded,uid'));data={ok:true};}
   else if(p==='/api/editor-views/current/explorer'){assert.equal(method,'GET');data={allowed:false,pending:false};}
   else if(p==='/api/editor-views/saved-connections')data={profiles:[],pendingRequest:'',detached};
   else if(p==='/api/remote-browser'){assert.equal(method,'GET','no connection or remote mutation permitted');data={sessions:[],connections:[]};}
   else if(p==='/api/terminals/known-hosts')data={hosts:[]};
   else if(p==='/api/credentials'){assert(!detached);assert.equal(method,'GET');data={vault:{available:true,state:'locked'},entries:[]};}
   else if(p==='/api/transfer/server'){assert.equal(method,'GET','no server start permitted');data={protocol:url.searchParams.get('protocol'),running:false,root:'',transfers:[]};}
   else if(p==='/api/transfer/local'){assert.equal(method,'GET');data={path:'C:/Synthetic',parent:'C:/',roots:['C:/'],drives:[{path:'C:/',label:'C: Synthetic',kind:'local'}],entries:[{name:'example.txt',path:'C:/Synthetic/example.txt',directory:false,size:24}]};}
   else if(p.includes('/events'))data={events:[],sessions:[],after:0};
   else if(p.startsWith('/api/editor-views/documents/')&&p.endsWith('/release'))data={ok:true};
   else assert.equal(method,'GET','unexpected mutation '+p);
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  const html=p==='/'||p.startsWith('/editor-view/');let filename=path.join(root,'web',html?'index.html':p);if(!fs.existsSync(filename))filename=path.join(root,p);let content=fs.readFileSync(filename);
  if(html)content=content.toString().replaceAll('__SESSION_TOKEN__','synthetic118').replaceAll('__WINDOW_MODE__',detached?'editor':'main').replaceAll('__EDITOR_VIEW__',detached?JSON.stringify({id:child,ownerInstanceId:'synthetic118'}):'null').replaceAll('__TERMINAL_VIEW__','null').replaceAll('src="assets/','src="/assets/').replaceAll('href="assets/','href="/assets/').replaceAll('src="../assets/','src="/assets/').replaceAll('href="../assets/','href="/assets/');
  return route.fulfill({body:content,contentType:html?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
 }catch(error){report.errors.push(error.message);return route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:error.message})});}});
 await page.goto('https://connect118.toolkit.test/'+(detached?'editor-view/'+child:''));await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
 return {page,context,calls};
}
async function iconCheck(tab){
 const data=await tab.evaluate(el=>{const svg=el.querySelector('svg'),rect=el.getBoundingClientRect(),s=svg.getBoundingClientRect();return {text:el.innerText.trim(),path:svg.querySelector('path').getAttribute('d'),viewBox:svg.getAttribute('viewBox'),hidden:svg.getAttribute('aria-hidden'),iconWidth:s.width,iconHeight:s.height,width:rect.width,height:rect.height,x:rect.x,y:rect.y};});
 assert.equal(data.text,'Connect');assert.equal(data.path,connectPath);assert.equal(data.viewBox,'0 0 24 24');assert.equal(data.hidden,'true');assert.equal(data.iconWidth,17);assert.equal(data.iconHeight,17);assert.equal(data.width,42);return data;
}
function noConnect(calls,detached=false){
 const permitted=new Set(['/api/state','/api/editor-views/current/state','/api/editor-views/current/activity']);
 assert(calls.every(c=>c.method==='GET'||permitted.has(c.path)||/^\/api\/editor-views\/documents\/[^/]+\/release$/.test(c.path)),'unexpected action, connect, listener or elevation request');
 assert(!calls.some(c=>c.body?.action==='connect'||c.body?.runAsAdministrator===true));
 if(detached)assert(!calls.some(c=>['/api/state','/api/credentials','/api/credential-vault'].includes(c.path)));
}
async function tcpIconCheck(page,id,width){
 // Phase 6: TCP ports is a tab in Test a target; its distinct icon moved there.
 const previous=await page.evaluate(()=>state.activePage);await page.evaluate(()=>navigate('ping'));await settle(page);
 const button=page.locator('.nt-target-tests [data-target-test="tcp"]');await button.waitFor({state:'visible'});
 const expanded=await button.evaluate(el=>{const s=el.querySelector('svg'),r=s.getBoundingClientRect(),style=getComputedStyle(s);return {label:el.textContent.trim(),title:el.title,path:s.querySelector('path').getAttribute('d'),width:r.width,height:r.height,stroke:style.stroke,viewBox:s.getAttribute('viewBox')};});
 assert.match(expanded.label,/^TCP ports/);assert.match(expanded.title,/^TCP ports/);assert.equal(expanded.path,await page.evaluate(()=>paths.tcpPorts));assert.notEqual(expanded.path,connectPath);assert.equal(expanded.viewBox,'0 0 24 24');assert(expanded.width>0&&expanded.height>0&&expanded.stroke!=='none');
 await button.screenshot({path:path.join(out,'tcp-icon124-'+id+'-expanded.png')});
 await page.evaluate(page=>navigate(page),previous);await settle(page);
 (report.tcpIcons||(report.tcpIcons=[])).push({id,expanded,collapsed:null});
}
async function editorCase(detached,width,theme){
 const id=(detached?'detached':'main')+'-'+width+'-'+theme,preference=theme==='dark'?{panel:'connections',side:'remote',width:360,collapsed:false}:undefined,f=await start({detached,width,theme,preference}),{page}=f;
 try{
  await page.evaluate(async({theme,preference,detached})=>{if(!detached&&preference)state.editorFilesPreferences=preference;setTheme(theme);navigate('editor');await NetworkToolkitFileEditor.newDocument();window.connect118Document=NetworkToolkitFileEditor.active;},{theme,preference,detached});
  const text=page.locator('[data-ed-text]');await text.fill('UNSAVED_SYNTHETIC_118');await text.dispatchEvent('input');
  const shell=page.locator('.nt-editor-files-shell'),tabs=shell.locator('[data-editor-files-panel]'),connect=shell.locator('[data-editor-files-panel="connections"]'),files=shell.locator('[data-editor-files-panel="files"]'),host=page.locator('[data-ed-files-sidebar]');
  assert.deepEqual(await tabs.evaluateAll(els=>els.map(el=>el.dataset.editorFilesPanel)),['connections','files']);
  const selected=preference?'connections':'files';assert.equal(await shell.locator('[data-editor-files-panel][aria-selected="true"]').getAttribute('data-editor-files-panel'),selected,'DOM order must not replace selected preference');assert.equal(await host.isVisible(),width>680);
  if(width<=680)await shell.locator('[data-editor-files-reopen]').click();await settle(page);
  const geometry=await iconCheck(connect),fileBox=await files.boundingBox();assert(geometry.y+geometry.height<=fileBox.y+1,'Connect must be physically above Files');assert.equal(await connect.getAttribute('aria-label'),'Connections');assert.equal(await shell.locator('.terminal-sidebar-tabs').getAttribute('aria-orientation'),'vertical');assert.equal(await files.locator('svg path').getAttribute('d'),await page.evaluate(()=>paths.folder));assert.equal(await shell.locator('.terminal-sidebar-rail').evaluate(el=>el.getBoundingClientRect().width),43);
  await page.screenshot({path:path.join(out,'connect-sidebar118-editor-'+id+'.png')});
  await files.focus();for(const [key,panel]of [['Home','connections'],['ArrowDown','files'],['ArrowUp','connections'],['End','files']]){await page.keyboard.press(key);await page.waitForFunction(panel=>document.querySelector('.nt-editor-files-shell [data-editor-files-panel][aria-selected="true"]')?.dataset.editorFilesPanel===panel,panel);assert.equal(await page.evaluate(()=>document.activeElement.dataset.editorFilesPanel),panel);assert(await host.isVisible());}
  await files.click();assert(!await host.isVisible());await files.press('Enter');assert(await host.isVisible());await files.press('Space');assert(!await host.isVisible());await connect.click();assert(await host.isVisible());assert.equal(await connect.getAttribute('aria-selected'),'true');
  assert(await page.evaluate(()=>NetworkToolkitFileEditor.active===connect118Document&&NetworkToolkitFileEditor.active.dirty&&NetworkToolkitFileEditor.active.content==='UNSAVED_SYNTHETIC_118'));
  const stored=await page.evaluate(detached=>detached?JSON.parse(sessionStorage.getItem('ntk.editor.files')):state.editorFilesPreferences,detached);assert.equal(stored.panel,'connections');assert.equal(stored.width,preference?360:300);
  if(!detached)await tcpIconCheck(page,id,width);
  noConnect(f.calls,detached);report.cases.push({id,kind:'editor',initialSelection:selected,geometry,passed:true});report.checks.push(id+' Connect-first order, shared SVG, default/saved selection, native rail keyboard/collapse, dirty retention and no connection/UAC');save();
 }finally{await f.context.close();}
}
async function transferCase(protocol,mode){
 const id=protocol+'-'+mode,f=await start(),{page}=f;
 try{
  await page.evaluate(({protocol,mode})=>{state.transferModes={...(state.transferModes||{}),[protocol]:mode};state.transferClientLayout=state.transferClientLayout||{};state.transferClientLayout[protocol]={width:300,collapsed:false,tab:'settings',side:'left',activityHeight:0};state.transferServerSidebar=state.transferServerSidebar||{};state.transferServerSidebar[protocol]={width:320,collapsed:false,activityHeight:0};setTheme('light');navigate(protocol);},{protocol,mode});
  const tab=page.locator(mode==='client'?'.rb-client-sidebar [data-client-tab="settings"]':'.transfer-server-sidebar-button');await tab.waitFor({state:'visible'});await settle(page);const geometry=await iconCheck(tab);
  await page.screenshot({path:path.join(out,'connect-sidebar118-'+id+'.png')});
  if(mode==='client'){const tabs=page.locator('.rb-client-sidebar [data-client-tab]');assert.deepEqual(await tabs.evaluateAll(els=>els.map(el=>el.dataset.clientTab)),['settings','saved','recent']);assert.equal(await tab.getAttribute('aria-selected'),'true');await tab.click();assert(!await page.locator('.rb-client-sidebar .terminal-sidebar-content').isVisible());await tab.press('Enter');assert(await page.locator('.rb-client-sidebar .terminal-sidebar-content').isVisible());}
  else{const body=page.locator('.transfer-server-sidebar-body');assert(await body.isVisible());await tab.click();assert(!await body.isVisible());await tab.press('Enter');assert(await body.isVisible());}
  noConnect(f.calls);report.cases.push({id,kind:'transfer',geometry,passed:true});report.checks.push(id+' actual Connect SVG matches editor and rail collapse does not connect/start/elevate');save();
 }finally{await f.context.close();}
}
(async()=>{
 assert(out,'TOOLKIT_TEST_OUTPUT required');fs.mkdirSync(out,{recursive:true});for(const name of ['web/index.html','web/assets/editor-files-sidebar.js','web/assets/transfer-client-layout.js','web/assets/transfer-server.js'])report.sourceHashes[name]=require('node:crypto').createHash('sha256').update(fs.readFileSync(path.join(root,name))).digest('hex');
 const timer=setTimeout(()=>{report.failure='Connect118 exceeded four minutes';save();process.exit(1);},240000);
 try{browser=await chromium.launch({headless:true});for(const detached of [false,true])for(const width of [1440,390])for(const theme of ['light','dark'])await editorCase(detached,width,theme);for(const protocol of ['sftp','ftp','tftp'])for(const mode of ['client','server'])await transferCase(protocol,mode);assert.deepEqual(report.errors,[]);assert.equal(report.cases.length,14);report.passed=true;save();console.log('PASS Connect118:8 editor contexts,6 transfer surfaces, zero implicit connection/listener/elevation');}
 catch(error){report.failure=error.stack;save();throw error;}
 finally{clearTimeout(timer);await browser?.close();}
})();
