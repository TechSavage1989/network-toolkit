'use strict';
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
// Full production UI with intercepted APIs and the browser's native clipboard.
// No transfer, server start, file mutation, connection or credential use occurs.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT,origin='https://locations106.toolkit.test';
const filename='AOS-CX_6200_10_18_1010.swi',address='172.16.18.26',disk='C:\\TFTP',report={passed:false,checks:[],errors:[],screenshots:[],scope:'Synthetic transfer locations in production browser assets; native clipboard, no live servers or native Windows acceptance.'};
const adapters=[{name:'LAN',description:'Synthetic equipment LAN',up:true,addresses:[address+'/24','172.16.18.27/24','2001:db8::26/64']},{name:'Down',up:false,addresses:['192.0.2.4/24']}];
const servers=Object.fromEntries(['tftp','sftp','ftp'].map(protocol=>[protocol,{protocol,running:true,bind:address,port:protocol==='tftp'?69:protocol==='sftp'?2222:2121,root:disk,security:protocol==='ftp'?'explicit':'',transfers:[]}]));
const posts=[];let browser,page;
const pass=text=>{report.checks.push(text);console.log('PASS '+text);};
async function close(){if(await page.locator('[data-transfer-location-dialog]').count())await page.keyboard.press('Escape');await page.locator('[data-transfer-location-dialog]').waitFor({state:'detached'});}
async function value(id){return page.locator('[data-location-value="'+id+'"]').textContent();}
async function copy(id,expected,keyboard=false){const button=page.locator('[data-transfer-location-copy="'+id+'"]');if(keyboard){await button.focus();await page.keyboard.press('Enter');}else await button.click();await page.waitForFunction(async expected=>(await navigator.clipboard.readText())===expected,expected);assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),expected);}
async function shot(name){if(!out)return;const file='transfer-locations-'+name+'.png';await page.screenshot({path:path.join(out,file),fullPage:true});report.screenshots.push(file);}
async function serverView(protocol='tftp'){await page.evaluate(async protocol=>{closeModal();state.transferModes={...state.transferModes,[protocol]:'server'};navigate(protocol);await NetworkToolkitTransfers.refresh(protocol);},protocol);await page.locator('[data-transfer-copy-location="'+protocol+'"][data-transfer-path="'+filename+'"]').waitFor();}
async function clientView(kind){await page.evaluate(({kind,filename,address,disk})=>{
 closeModal();const pageName=kind==='ssh'?'sftp':'ftp';state.transferModes={...state.transferModes,[pageName]:'client'};
 const api=NetworkToolkitRemoteBrowser,m=api.get(kind);Object.assign(api.configurations[kind],{host:address,port:kind==='ssh'?2222:2121,security:'explicit',username:'DO_NOT_COPY_USER',password:'DO_NOT_COPY_PASSWORD'});Object.assign(m,{saved:true,connected:true,connectionId:'synthetic-client',sessionId:'',active:'remote'});
 for(const side of ['local','remote']){const base=side==='local'?disk:'/firmware',join=side==='local'?'\\':'/';Object.assign(m[side],{path:base,parent:side==='local'?'C:\\':'/',initialized:true,loading:false,entries:[{name:filename,path:base+join+filename,directory:false,size:1024},{name:'different.bin',path:base+join+'different.bin',directory:false,size:256}],selected:new Set(),filter:''});}
 navigate(pageName);api.render(kind);
 },{kind,filename,address,disk});await page.locator('[data-rb-pane="remote"] [data-rb-item]').first().waitFor();}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});page=await browser.newPage({viewport:{width:1440,height:1000},serviceWorkers:'block',permissions:['clipboard-read','clipboard-write']});page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));
 await page.route('**/*',async route=>{if(await handleEditorActivityMetadata(route,origin,report))return;
  const req=route.request(),url=new URL(req.url()),p=url.pathname;assert.equal(url.origin,origin,'No external requests permitted');
  if(p.startsWith('/api/')){const method=req.method(),body=req.postData()?req.postDataJSON():null;let data={};if(method!=='GET'){posts.push({p,method,body});assert.equal(p,'/api/state','Copying must not launch, transfer, modify files or use credentials');}
   if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',system:{hostname:'Location fixture',interfaces:adapters}};
   if(p==='/api/system')data={hostname:'Location fixture',interfaces:adapters};
   if(p==='/api/state'&&method==='GET')data={appUpdatePreferences:{enabled:false},transferModes:{tftp:'server',sftp:'client',ftp:'client'},transferServerSettings:Object.fromEntries(Object.entries(servers).map(([key,s])=>[key,{bind:s.bind,port:s.port,root:s.root,security:s.security}]))};
   if(p==='/api/jobs')data=[];if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};if(p==='/api/terminal-views')data={available:false,owners:{},views:[]};
   if(p==='/api/credentials')data={vault:{available:false,state:'unavailable'},entries:[]};if(p==='/api/credential-vault')data={available:false,state:'unavailable'};
   if(p==='/api/transfer/server')data=servers[url.searchParams.get('protocol')]||servers.tftp;
   if(p==='/api/transfer/files')data={root:disk,path:'',parent:'',entries:[{name:filename,path:filename,directory:false,size:1024,modified:'2026-10-27T20:00:00Z'}]};
   if(p==='/api/transfer/local')data={path:disk,parent:'C:\\',entries:[]};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,80));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{const file=path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','locations106-fixture').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto(origin+'/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);assert.equal(await page.evaluate(()=>!!window.NetworkToolkitTransferLocations),true,'Final index must include the production transfer-location helper');
 await page.evaluate(()=>navigator.clipboard.writeText('UNCHANGED BEFORE COPY'));
 await serverView();await page.locator('[data-transfer-copy-location="tftp"][data-transfer-path="'+filename+'"]').focus();await page.keyboard.press('Enter');
 const expected={network:address+'/'+filename,url:'tftp://'+address+'/'+filename,path:filename,local:disk+'\\'+filename,host:address,endpoint:address+':69'};
 for(const [id,text]of Object.entries(expected)){assert.equal(await value(id),text);await copy(id,text,id==='network');}
 assert(!await page.locator('.transfer-locations').textContent().then(text=>text.includes('DO_NOT_COPY')));await shot('tftp-file-normal');await close();
 pass('Screenshot TFTP filename copies six explicit formats through the native clipboard without transfer');

 await page.locator('[data-transfer-copy-listener="tftp"]').click();assert.equal(await value('endpoint'),address+':69');assert.equal(await page.locator('[data-location-value="network"]').count(),0);await close();
 servers.tftp.bind='0.0.0.0';await serverView();await page.locator('[data-transfer-copy-location="tftp"][data-transfer-path="'+filename+'"]').click();assert.equal(await page.locator('[data-transfer-location-copy="network"]').count(),0);assert.equal(await page.locator('#transferLocationAddress option').count(),3,'Only matching-family active non-loopback adapters are choices');
 await page.evaluate(()=>{system.interfaces=[{name:'Changed later',up:true,addresses:['192.0.2.99/24']}];state.forms.tftp.remotePath='different.bin';});
 await page.locator('#transferLocationAddress').selectOption('172.16.18.27');await copy('network','172.16.18.27/'+filename);await copy('local',disk+'\\'+filename);await shot('wildcard-normal');
 for(const width of [1440,390]){await page.setViewportSize({width,height:1000});await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));const box=await page.locator('.modal').boundingBox();assert(box.x>=-1&&box.x+box.width<=width+1);assert.equal(await page.locator('.modal-body').evaluate(el=>el.scrollWidth<=el.clientWidth+1),true,'Location values fit or wrap in the narrow dialog');await shot('wildcard-'+width);}
 const clipboard=await page.evaluate(()=>navigator.clipboard.readText());await close();assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),clipboard,'Escape cancels without copying');await page.setViewportSize({width:1440,height:1000});
 pass('Wildcard listener requires explicit adapter choice; frozen file/address choices survive later state changes and Escape leaves clipboard unchanged');

 for(const protocol of ['sftp','ftp']){await serverView(protocol);await page.locator('[data-transfer-copy-location="'+protocol+'"][data-transfer-path="'+filename+'"]').click();await copy('network',address+':'+servers[protocol].port+'/'+filename);await copy('path',filename);await copy('local',disk+'\\'+filename);if(protocol==='ftp'){assert.equal(await page.locator('[data-location-value="url"]').count(),0);assert.match(await page.locator('.transfer-locations').textContent(),/Explicit FTPS/);}await close();}
 pass('SFTP and FTPS server locations preserve nondefault ports, server-relative and disk paths without inventing FTPS URLs');

 for(const kind of ['ssh','ftp']){
  await clientView(kind);const remote='/firmware/'+filename,row=page.locator('[data-rb-pane="remote"] [data-rb-item="'+remote+'"]');await row.click();await page.locator('.rb-toolbar [data-rb-action="copyLocation"]').click();assert.equal(await value('path'),remote);await copy('network',address+':'+(kind==='ssh'?2222:2121)+'/firmware/'+filename);if(kind==='ftp')assert.equal(await page.locator('[data-location-value="url"]').count(),0);await close();
  await row.click({button:'right'});await page.evaluate(kind=>{NetworkToolkitRemoteBrowser.get(kind).remote.selected=new Set(['/firmware/different.bin']);NetworkToolkitRemoteBrowser.configurations[kind].host='changed.example';},kind);await page.locator('[data-rb-menu="copyLocation"]').click();assert.equal(await value('host'),address);assert.equal(await value('path'),remote);await copy('path',remote);await shot(kind+'-context-snapshot');await close();
  const local=disk+'\\'+filename;await page.locator('[data-rb-pane="local"] [data-rb-item]').filter({hasText:filename}).click();await page.locator('.rb-toolbar [data-rb-action="copyLocation"]').click();await copy('local',local);assert.equal(await page.locator('[data-location-value="network"]').count(),0);await close();
 }
 pass('SFTP/FTP client toolbar and right-click actions preserve clicked-file/endpoint snapshots; local paths remain local only');

 for(const operation of ['upload','download']){await page.evaluate(({operation,address,filename,disk})=>{closeModal();state.transferModes.tftp='client';state.target=address;Object.assign(state.forms.tftp,{operation,port:69,remotePath:filename,localPath:disk+'\\'+filename});navigate('tftp');},{operation,address,filename,disk});await page.locator('[data-tftp-copy-location]').click();assert.equal(await value('path'),filename);await copy('network',address+'/'+filename);await copy('url','tftp://'+address+'/'+filename);await copy('local',disk+'\\'+filename);await close();}
 pass('TFTP send and receive configuration copy actions use the same explicit remote/local formats without running either transfer');
 await page.locator('[data-tftp-copy-location]').click();const beforeDenied=await page.evaluate(()=>navigator.clipboard.readText());
 await page.evaluate(()=>{document.querySelector('#toastStack')?.replaceChildren();window.fixtureClipboardWrite=navigator.clipboard.writeText;window.fixtureExecCommand=document.execCommand;Object.defineProperty(navigator.clipboard,'writeText',{configurable:true,value:async()=>{throw new Error('Synthetic clipboard denied');}});document.execCommand=()=>false;});
 await page.locator('[data-transfer-location-copy="network"]').click();await page.waitForFunction(()=>document.querySelector('#toastStack')?.textContent.includes('Copy is unavailable.'));
 assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),beforeDenied,'Clipboard failure leaves the previous value unchanged');assert(!await page.locator('#toastStack').textContent().then(text=>text.includes('Copied to clipboard.')),'No false success on denied clipboard and fallback');assert.equal(await page.locator('[data-transfer-location-dialog]').isVisible(),true);
 await page.evaluate(()=>{Object.defineProperty(navigator.clipboard,'writeText',{configurable:true,value:window.fixtureClipboardWrite});document.execCommand=window.fixtureExecCommand;delete window.fixtureClipboardWrite;delete window.fixtureExecCommand;});await close();
 pass('Clipboard denial plus failed fallback reports an honest error, keeps the dialog and leaves clipboard unchanged');
 assert(!posts.some(call=>call.p!=='/api/state'));assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.errors.push(error.stack||String(error));console.error(error.stack||String(error));process.exitCode=1;}).finally(async()=>{if(out){fs.writeFileSync(path.join(out,'transfer-locations-report.json'),JSON.stringify(report,null,2)+'\n');if(page&&!report.passed)await page.screenshot({path:path.join(out,'transfer-locations-failure.png'),fullPage:true}).catch(()=>{});}await browser?.close();});
