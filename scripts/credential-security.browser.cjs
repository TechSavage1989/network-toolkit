'use strict';
// UI-only contract/rendering test. ALL APIs and native-confirmation outcomes are
// synthetic route fixtures. No engine, vault files, OS credentials or network
// servers are used. This is NOT proof of Windows DPAPI/native approval/bootstrap.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT;
const ssh={protocol:'ssh',host:'core.test',port:22,username:'operator',fingerprint:'SHA256:synthetic-host-key'},ftps={protocol:'ftp-explicit',host:'files.test',port:21,username:'operator',fingerprint:'SHA256:synthetic-tls-pin'};
const canaries=['SAVED_PASSWORD_NEVER_SENT_104','SAVED_KEY_NEVER_SENT_104','SAVED_PASSPHRASE_NEVER_SENT_104','LEGACY_SECRET_NEVER_SENT_104','TYPED_NEW_PASSWORD_104','TYPED_REPLACEMENT_PASSWORD_104'];
const metadata=record=>{const {id,name,kind,username,bindings=[],updatedAt='fixture'}=record;return {id,name,kind,username,bindings,updatedAt};};
const sameEndpoint=(a,b)=>['protocol','host','port','username'].every(key=>a[key]===b[key]);
const noAuth=body=>{for(const field of ['password','privateKey','passphrase','keyPath','secret'])assert(!Object.hasOwn(body,field),'Saved reference request contains '+field);};
async function until(predicate,label){const deadline=Date.now()+10000;while(!predicate()){if(Date.now()>deadline)throw Error('Timed out: '+label);await new Promise(resolve=>setTimeout(resolve,20));}}
(async()=>{
 const records=new Map([
  ['password',{id:'password',name:'Core password',username:'operator',kind:'password',secret:canaries[0],bindings:[ssh,ftps]}],
  ['key',{id:'key',name:'Core key',username:'operator',kind:'ssh-key',secret:JSON.stringify({privateKey:canaries[1],passphrase:canaries[2]}),bindings:[ssh]}],
  ['legacy',{id:'legacy',name:'Legacy password',username:'operator',kind:'password',secret:canaries[3],bindings:[]}]
 ]);
 let saved={appUpdatePreferences:{enabled:false},terminalProfiles:[],ftpProfiles:[],transferModes:{sftp:'client',ftp:'client'}},denyNext=false,holdNext=false,releaseApproval=null,sessionNumber=0;
 const calls=[],responses=[],errors=[];
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']}),page=await browser.newPage({viewport:{width:1280,height:1000}});
 page.on('pageerror',error=>errors.push(error.message));
 await page.route('http://toolkit.local/**',async route=>{
  const request=route.request(),p=new URL(request.url()).pathname,method=request.method(),body=request.postData()?request.postDataJSON():null;
  if(p.startsWith('/api/')){
   calls.push({path:p,method,body});let data={};
   const respond=(value,status=200)=>{responses.push({path:p,status,data:value});return route.fulfill({status,contentType:'application/json',body:JSON.stringify(value)});};
   const simulatedNativeApproval=async()=>{if(holdNext){holdNext=false;await new Promise(resolve=>releaseApproval=resolve);releaseApproval=null;}if(denyNext){denyNext=false;return false;}return true;};
   if(p==='/api/status')data={version,os:'windows',arch:'amd64'};
   if(p==='/api/state'){if(method==='GET')data=saved;else saved=body;}
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   if(p==='/api/credentials'){
    if(method==='POST'){
     const prior=records.get(body.id),id=body.id||'created-'+records.size;
     if(body.bindings?.some(binding=>!prior?.bindings?.some(old=>sameEndpoint(old,binding)&&old.fingerprint===binding.fingerprint))&&!await simulatedNativeApproval())return respond({error:'Fixture: native destination approval cancelled.'},403);
     const record={...prior,...body,id};records.set(id,record);data={entry:metadata(record)};
    }else data={entries:[...records.values()].map(metadata),protection:'Synthetic UI-only metadata fixture (not DPAPI verification)'};
   }
   if(/^\/api\/credentials\/[^/]+\/bindings$/.test(p)){
    const record=records.get(decodeURIComponent(p.split('/')[3]));
    if(!await simulatedNativeApproval())return respond({error:'Fixture: native destination approval cancelled.'},403);
    if(!record.bindings.some(binding=>sameEndpoint(binding,body.binding)))record.bindings.push(body.binding);
    data={entry:metadata(record)};
   }else if(/^\/api\/credentials\/[^/]+$/.test(p)){
    if(method==='GET')throw Error('UI must not retrieve an individual saved-secret record.');
    if(method==='DELETE')records.delete(decodeURIComponent(p.split('/')[3]));
   }
   if(p==='/api/terminals')data=method==='POST'?{session:{id:'security-session-'+(++sessionNumber),type:body.type,host:body.host,port:body.port,username:body.username,state:'connected'}}:{sessions:[]};
   if(p==='/api/terminals/known-hosts')data={entries:[]};
   if(p==='/api/remote-browser')data={path:'.',home:'.',parent:'.',entries:[],...(body?.protocol==='ssh'&&body.action==='connect'?{connectionId:'synthetic-sftp'}:{})};
   if(p==='/api/local-browser')data={path:'fixture',home:'fixture',parent:'fixture',entries:[]};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
   return respond(data);
  }
  try{
   const file=path.join(root,'web',p==='/'?'index.html':p);let content=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));
   if(p==='/')content=content.toString().replaceAll('__SESSION_TOKEN__','synthetic-security-ui-fixture').replaceAll('__WINDOW_MODE__','main');
   return route.fulfill({body:content,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 const posts=path=>calls.filter(row=>row.path===path&&row.method==='POST');
 const assertNoSecretsInDOM=async()=>{const snapshot=await page.evaluate(()=>({html:document.documentElement.outerHTML,values:[...document.querySelectorAll('input,textarea')].map(el=>el.value)}));for(const value of canaries)assert(!JSON.stringify(snapshot).includes(value),'Secret canary exposed in rendered DOM');};
 const submit=async()=>{await page.locator('#credentialManagerForm button[type="submit"]').click();};
 const waitEditorCleared=async()=>{await page.waitForFunction(()=>document.querySelector('#credentialID')?.value==='');};
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  await page.evaluate(()=>NetworkToolkitCredentials.show());
  await page.locator('[data-credential-edit="password"]').click();assert.equal(await page.locator('#credentialSecret').inputValue(),'');await assertNoSecretsInDOM();
  await page.locator('#credentialName').fill('Renamed password');await submit();await waitEditorCleared();
  assert.equal(records.get('password').secret,canaries[0]);assert.equal(posts('/api/credentials').at(-1).body.secret,undefined);
  await page.locator('[data-credential-edit="key"]').click();assert.equal(await page.locator('#credentialPrivateKey').inputValue(),'');assert.equal(await page.locator('#credentialPassphrase').inputValue(),'');await assertNoSecretsInDOM();
  await page.locator('[data-credential-action="clear-editor"]').click();
  await page.locator('#credentialName').fill('New password');await page.locator('#credentialUsername').fill('operator');await page.locator('#credentialSecret').fill(canaries[4]);await page.locator('#credentialHost').fill('new.test');
  denyNext=true;await submit();await until(()=>responses.some(row=>row.path==='/api/credentials'&&row.status===403),'denied creation');
  assert.equal(records.size,3);assert.equal(await page.locator('#credentialName').inputValue(),'New password');
  await submit();await until(()=>records.size===4,'approved creation');await waitEditorCleared();
  const created=[...records.values()].find(record=>record.name==='New password');assert.equal(created.secret,canaries[4]);assert.deepEqual(created.bindings,[{protocol:'ssh',host:'new.test',port:22,username:'operator'}]);await assertNoSecretsInDOM();
  // Blank update keeps the server-side secret; denied destination mutation is atomic.
  await page.locator('[data-credential-edit="'+created.id+'"]').click();await page.locator('#credentialHost').fill('extra.test');denyNext=true;await submit();
  await until(()=>responses.filter(row=>row.path==='/api/credentials'&&row.status===403).length===2,'denied update');assert.equal(records.get(created.id).bindings.length,1);assert.equal(records.get(created.id).secret,canaries[4]);
  await submit();await until(()=>records.get(created.id).bindings.length===2,'approved update');await waitEditorCleared();assert.equal(posts('/api/credentials').at(-1).body.secret,undefined);
  await page.locator('[data-credential-edit="'+created.id+'"]').click();await page.locator('#credentialSecret').fill(canaries[5]);await submit();await until(()=>records.get(created.id).secret===canaries[5],'replacement update');await waitEditorCleared();await assertNoSecretsInDOM();
  // Render the longer approval UI at both supported representative widths.
  const rendered=[];
  for(const width of [1280,390]){
   await page.setViewportSize({width,height:1000});
   const layout=await page.locator('.modal').evaluate(el=>{const box=el.getBoundingClientRect(),body=el.querySelector('.modal-body');return {left:box.left,right:box.right,width:innerWidth,overflow:body.scrollWidth-body.clientWidth};});
   assert(layout.left>=-1&&layout.right<=width+1&&layout.overflow<=1,JSON.stringify(layout));rendered.push(width);
   if(output)await page.screenshot({path:path.join(output,'credential-manager-'+width+'.png')});
  }
  await page.setViewportSize({width:1280,height:1000});await page.evaluate(()=>{closeModal();navigate('terminal');openTerminalConnect({type:'ssh',host:'legacy.test',port:22,username:'operator',credentialId:'legacy'});});
  await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>document.querySelector('#terminalConnectError')?.textContent.includes('not approved'));assert.equal(posts('/api/terminals').length,0);
  // Two explicit clicks produce one pending simulated native confirmation. A
  // changed destination while that request is pending does not inherit its grant.
  holdNext=true;await page.locator('[data-credential-action="approve-current"]').click();await until(()=>!!releaseApproval,'held approval');await page.locator('[data-credential-action="approve-current"]').click();
  assert.equal(posts('/api/credentials/legacy/bindings').length,1);await page.locator('#con-host').fill('different.test');releaseApproval();
  await page.waitForFunction(()=>NetworkToolkitCredentials.meta('legacy')?.bindings.length===1);assert.equal(records.get('legacy').bindings[0].host,'legacy.test');
  await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>document.querySelector('#terminalConnectError')?.textContent.includes('DIFFERENT.TEST')||document.querySelector('#terminalConnectError')?.textContent.includes('different.test'));assert.equal(posts('/api/terminals').length,0);
  // Cancelled approval leaves the new destination unbound.
  denyNext=true;await page.locator('[data-credential-action="approve-current"]').click();await until(()=>responses.some(row=>row.path==='/api/credentials/legacy/bindings'&&row.status===403),'cancelled binding');assert.equal(records.get('legacy').bindings.length,1);
  await page.locator('#con-host').fill('legacy.test');await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>terminalSessions.has('security-session-1'));noAuth(posts('/api/terminals').at(-1).body);assert.equal(posts('/api/terminals').at(-1).body.credentialId,'legacy');
  // SFTP/FTPS exercise production request builders but all transport is mocked.
  for(const protocol of ['ssh','ftp']){
   const binding=protocol==='ssh'?ssh:ftps;
   await page.evaluate(({protocol,binding})=>{navigate(protocol==='ssh'?'sftp':'ftp');Object.assign(NetworkToolkitRemoteBrowser.configurations[protocol],{host:binding.host,port:binding.port,username:binding.username,fingerprint:binding.fingerprint,security:'explicit',credentialId:'password',password:'',privateKey:'',passphrase:'',keyPath:''});const model=NetworkToolkitRemoteBrowser.get(protocol);model.profileId='';model.profileManualAuth=false;NetworkToolkitRemoteBrowser.render(protocol);},{protocol,binding});
   await page.evaluate(protocol=>NetworkToolkitRemoteBrowser.connect(protocol),protocol);assert.equal(await page.evaluate(protocol=>NetworkToolkitRemoteBrowser.get(protocol).connected,protocol),true);
   const call=posts('/api/remote-browser').findLast(row=>row.body.protocol===protocol&&row.body.credentialId==='password');assert(call);noAuth(call.body);assert.equal(call.body.fingerprint,binding.fingerprint);assert.equal(call.body.host,binding.host);
  }
  await page.evaluate(()=>flushState());for(const secret of canaries)assert(!JSON.stringify(saved).includes(secret));
  assert.equal(calls.filter(row=>/^\/api\/credentials\/[^/]+$/.test(row.path)&&row.method==='GET').length,0);
  for(const response of responses.filter(row=>row.path.startsWith('/api/credentials')))for(const secret of canaries)assert(!JSON.stringify(response.data).includes(secret));
  await assertNoSecretsInDOM();assert.deepEqual(errors,[]);
  const result={passed:true,scope:'UI-only synthetic route fixture; NOT Windows native/DPAPI/bootstrap verification',blankPasswordAndKeyEditor:true,createUpdateDenialAndSuccess:true,blankUpdatePreservesSecret:true,explicitDestinationApproval:true,duplicateApprovalDeduplicated:true,staleDestinationNotGranted:true,savedConnectionsIDOnly:['terminal','sftp','ftps'],noSavedSecretReads:true,noSecretMetadataOrPersistence:true,renderedWidths:rendered,errors};
  if(output)fs.writeFileSync(path.join(output,'credential-security-results.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
 }catch(error){if(output)await page.screenshot({path:path.join(output,'credential-security-failure.png')}).catch(()=>{});throw error;}
 finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
