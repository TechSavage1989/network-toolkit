'use strict';
// UI-only contract/rendering test. ALL APIs and native-confirmation outcomes are
// synthetic route fixtures. No engine, vault files, OS credentials or network
// servers are used. This is NOT proof of Windows DPAPI/native approval/bootstrap.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT;
const ssh={protocol:'ssh',host:'core.test',port:22,username:'operator',fingerprint:'SHA256:synthetic-host-key'},ftps={protocol:'ftp-explicit',host:'files.test',port:21,username:'operator',fingerprint:'SHA256:synthetic-tls-pin'};
const nativeCanaries=['MASTER_NATIVE_ONLY_104','RECOVERY_NATIVE_ONLY_104','ROTATED_MASTER_NATIVE_ONLY_104'];
const canaries=['SAVED_PASSWORD_NEVER_SENT_104','SAVED_KEY_NEVER_SENT_104','SAVED_PASSPHRASE_NEVER_SENT_104','LEGACY_SECRET_NEVER_SENT_104','TYPED_NEW_PASSWORD_104','TYPED_REPLACEMENT_PASSWORD_104'];
const metadata=record=>{const {id,name,kind,username,bindings=[],updatedAt='fixture'}=record;return {id,name,kind,username,bindings,updatedAt};};
const sameEndpoint=(a,b)=>['protocol','host','port','username'].every(key=>a[key]===b[key]);
const noAuth=body=>{for(const field of ['password','privateKey','passphrase','keyPath','secret'])assert(!Object.hasOwn(body,field),'Saved reference request contains '+field);};
async function until(predicate,label){const deadline=Date.now()+10000;while(!predicate()){if(Date.now()>deadline)throw Error('Timed out: '+label);await new Promise(resolve=>setTimeout(resolve,20));}}
async function stableCredentialLayout(page){
 return page.locator('.modal').evaluate(async el=>{
  // Viewport changes can precede the renderer's completed responsive layout.
  // Settle geometry, NOT the assertion: a stable overflowing layout is returned
  // unchanged and fails the same strict test below.
  await document.fonts.ready;
  const frame=()=>new Promise(resolve=>requestAnimationFrame(resolve));
  const body=el.querySelector('.modal-body'),history=[];
  let previous='',stable=0,layout;
  for(let i=0;i<90;i++){
   await frame();
   const box=el.getBoundingClientRect();
   layout={left:box.left,right:box.right,width:innerWidth,clientWidth:body.clientWidth,scrollWidth:body.scrollWidth,overflow:body.scrollWidth-body.clientWidth};
   history.push(layout);
   const current=JSON.stringify(layout);
   stable=current===previous?stable+1:0;previous=current;
   if(stable>=2)break;
  }
  const bodyBox=body.getBoundingClientRect();
  const overflowing=[...body.querySelectorAll('*')].map(node=>{const box=node.getBoundingClientRect();return {tag:node.tagName,id:node.id,className:String(node.className||'').slice(0,100),left:box.left,right:box.right,width:box.width,clientWidth:node.clientWidth,scrollWidth:node.scrollWidth};}).filter(box=>box.width>0&&(box.left<bodyBox.left-1||box.right>bodyBox.right+1)).slice(0,30);
  return {...layout,settled:stable>=2,frames:history.length,history,overflowing};
 });
}
(async()=>{
 const records=new Map([
  ['password',{id:'password',name:'Core password',username:'operator',kind:'password',secret:canaries[0],bindings:[ssh,ftps]}],
  ['key',{id:'key',name:'Core key',username:'operator',kind:'ssh-key',secret:JSON.stringify({privateKey:canaries[1],passphrase:canaries[2]}),bindings:[ssh]}],
  ['legacy',{id:'legacy',name:'Legacy password',username:'operator',kind:'password',secret:canaries[3],bindings:[]}]
 ]);
 let saved={appUpdatePreferences:{enabled:false},terminalProfiles:[],ftpProfiles:[],transferModes:{sftp:'client',ftp:'client'}},denyNext=false,holdNext=false,releaseApproval=null,sessionNumber=0;
 let vault={available:true,state:'legacy',remembered:false,hasRecovery:false,legacyBackup:true,warning:'',protection:'Synthetic UI-only v2 fixture (not cryptographic verification)'},failLockAfterCompletion=false,vaultGeneration=0;
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
   if(p==='/api/credential-vault'){
    if(method==='GET')return respond({...vault});
    assert.equal(method,'POST');assert.deepEqual(Object.keys(body),['action'],'Vault requests must contain only an action, never native secrets');
    const action=body.action;assert(['setup','unlock','lock','forget','change-password','recover','delete-vault','cleanup-legacy'].includes(action));
    if(!await simulatedNativeApproval())return respond({error:'Fixture: native vault dialog cancelled; existing data was kept.'},400);
    if(['setup','unlock','change-password','recover'].includes(action)){
     if(action!=='unlock')vaultGeneration++;
     vault={...vault,state:'unlocked',remembered:true,hasRecovery:true,warning:''};
    }else if(action==='lock'||action==='forget'){
     vault={...vault,state:'locked',remembered:false,warning:''};
     if(failLockAfterCompletion){failLockAfterCompletion=false;vault.remembered=true;vault.warning='Fixture: memory locked, but a remembered record could not be removed. It may still unlock a future launch.';return respond({error:vault.warning},400);}
    }else if(action==='cleanup-legacy')vault={...vault,legacyBackup:false,warning:''};
    else if(action==='delete-vault'){records.clear();vault={...vault,state:vault.legacyBackup?'legacy':'setup-required',remembered:false,hasRecovery:false,warning:''};}
    return respond({...vault});
   }
   if(p==='/api/credentials'){
    if(method==='POST'){
     if(vault.state!=='unlocked')return respond({error:'Fixture: vault locked.'},423);
     const prior=records.get(body.id),id=body.id||'created-'+records.size;
     if(body.bindings?.some(binding=>!prior?.bindings?.some(old=>sameEndpoint(old,binding)&&old.fingerprint===binding.fingerprint))&&!await simulatedNativeApproval())return respond({error:'Fixture: native destination approval cancelled.'},403);
     const record={...prior,...body,id};records.set(id,record);data={entry:metadata(record)};
    }else data={vault:{...vault},entries:vault.state==='unlocked'?[...records.values()].map(metadata):[],protection:vault.protection};
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
 const assertNoSecretsInDOM=async()=>{const snapshot=await page.evaluate(()=>({html:document.documentElement.outerHTML,values:[...document.querySelectorAll('input,textarea')].map(el=>el.value)}));for(const value of [...canaries,...nativeCanaries])assert(!JSON.stringify(snapshot).includes(value),'Secret canary exposed in rendered DOM');};
 const vaultAction=action=>page.evaluate(async action=>{try{return {ok:true,value:await NetworkToolkitCredentials.vaultAction(action)};}catch(error){return {ok:false,error:error.message};}},action);
 const assertNativeOnlyEntry=async()=>{assert.equal(await page.locator('#credentialVaultPanel input,#credentialVaultPanel textarea').count(),0);await assertNoSecretsInDOM();};
 const stateLayouts=[];
 const captureVaultState=async label=>{
  for(const width of [1280,390]){
   await page.setViewportSize({width,height:1000});const layout=await stableCredentialLayout(page);stateLayouts.push({state:label,...layout});
   assert(layout.settled&&layout.width===width&&layout.left>=-1&&layout.right<=width+1&&layout.overflow<=1,JSON.stringify({state:label,...layout}));
   if(output)await page.screenshot({path:path.join(output,'credential-vault-'+label+'-'+width+'.png')});
  }
  if(output)fs.writeFileSync(path.join(output,'credential-vault-layouts.json'),JSON.stringify(stateLayouts,null,2));
  await page.setViewportSize({width:1280,height:1000});
 };
 const submit=async()=>{await page.locator('#credentialManagerForm button[type="submit"]').click();};
 const waitEditorCleared=async()=>{await page.waitForFunction(()=>document.querySelector('#credentialID')?.value===''&&document.querySelector('#credentialName')?.value===''&&document.querySelector('#credentialSecret')?.value==='');};
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  await page.evaluate(()=>NetworkToolkitCredentials.show());
  assert.equal(await page.evaluate(()=>NetworkToolkitCredentials.meta('password')),null);assert.equal(await page.locator('#credentialManagerForm').count(),0);
  await assertNativeOnlyEntry();await captureVaultState('legacy');
  vault={...vault,state:'setup-required',legacyBackup:false};await page.evaluate(()=>NetworkToolkitCredentials.show());
  assert.equal(await page.locator('[data-vault-action="setup"]').innerText(),'Set up vault');await captureVaultState('setup');
  vault={...vault,state:'legacy',legacyBackup:true};await page.evaluate(()=>NetworkToolkitCredentials.show());
  denyNext=true;const deniedSetup=await vaultAction('setup');assert.equal(deniedSetup.ok,false);assert.match(deniedSetup.error,/cancelled/);assert.equal(vault.state,'legacy');assert.equal(records.size,3);assert.equal(vaultGeneration,0);await assertNativeOnlyEntry();
  await page.locator('[data-vault-action="setup"]').click();await page.waitForFunction(()=>NetworkToolkitCredentials.vaultStatus().state==='unlocked'&&!!document.querySelector('#credentialManagerForm')&&!document.querySelector('[data-vault-action="lock"]')?.disabled);assert.equal(vaultGeneration,1);assert.equal(vault.remembered,true);assert.equal(await page.locator('[data-credential-edit="password"]').count(),1);
  assert.match(await page.locator('#credentialVaultPanel').innerText(),/master password is not saved/);
  assert.match(await page.locator('#credentialVaultPanel').innerText(),/Software running as you/);
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
  // Repeated resize transitions cover stale responsive geometry, not just a
  // fresh page at each size. Persistent overflow still fails immediately once
  // the layout itself has settled; no assertion threshold is relaxed.
  const rendered=[],layouts=[];
  for(const [round,width] of [1280,390,1280,390,1280,390].entries()){
   await page.setViewportSize({width,height:1000});
   const layout=await stableCredentialLayout(page);layouts.push({round,...layout});
   if(output)fs.writeFileSync(path.join(output,'credential-security-layouts.json'),JSON.stringify(layouts,null,2));
   assert(layout.settled&&layout.width===width&&layout.left>=-1&&layout.right<=width+1&&layout.overflow<=1,JSON.stringify({round,...layout}));if(!rendered.includes(width))rendered.push(width);
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
  // Native vault operations use actions only. Cancellation must not discard
  // an unsaved credential editor, while a completed/partially failed lock must.
  await page.evaluate(()=>NetworkToolkitCredentials.show());
  await page.locator('#credentialName').fill('Unsaved credential');await page.locator('#credentialSecret').fill('UNSAVED_EDITOR_ONLY_104');
  denyNext=true;const deniedRotation=await vaultAction('change-password');assert.equal(deniedRotation.ok,false);assert.equal(vaultGeneration,1);
  assert.equal(await page.locator('#credentialName').inputValue(),'Unsaved credential');assert.equal(await page.locator('#credentialSecret').inputValue(),'UNSAVED_EDITOR_ONLY_104');
  const connectionSnapshot=await page.evaluate(()=>({terminalIDs:[...terminalSessions.keys()],sftp:NetworkToolkitRemoteBrowser.get('ssh').connected,ftps:NetworkToolkitRemoteBrowser.get('ftp').connected}));
  assert(connectionSnapshot.terminalIDs.includes('security-session-1'),'Established SSH fixture must be present before lock');assert.equal(connectionSnapshot.sftp,true);assert.equal(connectionSnapshot.ftps,true);
  const beforeLockCalls=calls.length;
  assert.equal((await vaultAction('lock')).ok,true);assert.equal(vault.state,'locked');assert.equal(vault.remembered,false);
  assert.equal(await page.locator('#credentialManagerForm').count(),0);assert.equal(await page.evaluate(()=>NetworkToolkitCredentials.meta('password')),null);
  assert.equal(await page.locator('.modal input,.modal textarea').count(),0);await assertNativeOnlyEntry();await captureVaultState('locked');
  assert.deepEqual(await page.evaluate(()=>({terminalIDs:[...terminalSessions.keys()],sftp:NetworkToolkitRemoteBrowser.get('ssh').connected,ftps:NetworkToolkitRemoteBrowser.get('ftp').connected})),connectionSnapshot);
  assert.equal(calls.slice(beforeLockCalls).filter(row=>(row.method==='DELETE'&&row.path.startsWith('/api/terminals'))||(row.path==='/api/remote-browser'&&row.body?.action==='disconnect')).length,0,'Lock must preserve established connections');
  const blockedUse=await page.evaluate(async()=>{try{await NetworkToolkitCredentials.prepare({type:'ssh',host:'core.test',port:22,username:'operator',credentialId:'password'});return '';}catch(error){return error.message;}});assert.match(blockedUse,/locked/);
  denyNext=true;assert.equal((await vaultAction('unlock')).ok,false);assert.equal(vault.state,'locked');assert.equal(await page.locator('#credentialManagerForm').count(),0);
  // Dismissal during native entry must not reopen a closed manager. A repeated
  // command is deduplicated even when invoked programmatically (not just disabled).
  holdNext=true;const beforeUnlock=posts('/api/credential-vault').length;
  await page.evaluate(()=>{window.__vaultPendingFirst=NetworkToolkitCredentials.vaultAction('unlock').then(()=>({ok:true}),error=>({ok:false,error:error.message}));});
  await until(()=>!!releaseApproval,'held native vault unlock');
  await page.evaluate(()=>{window.__vaultPendingSecond=NetworkToolkitCredentials.vaultAction('unlock').then(()=>({ok:true}),error=>({ok:false,error:error.message}));closeModal();});
  assert.equal(posts('/api/credential-vault').length,beforeUnlock+1);releaseApproval();
  assert.deepEqual(await page.evaluate(async()=>Promise.all([window.__vaultPendingFirst,window.__vaultPendingSecond])),[{ok:true},{ok:true}]);
  assert.equal(await page.locator('#credentialVaultPanel').count(),0);assert.equal(await page.locator('#modalRoot .modal').count(),0);assert.equal(vault.state,'unlocked');
  await page.evaluate(()=>NetworkToolkitCredentials.show());
  assert.equal((await vaultAction('change-password')).ok,true);assert.equal(vaultGeneration,2);assert.equal(vault.remembered,true);await assertNativeOnlyEntry();
  // A backend may have cleared keys before a remembered-file removal fails.
  // Refresh state after the error; never leave stale unlocked metadata/editor.
  await page.locator('#credentialName').fill('Must disappear after partial lock');await page.locator('#credentialSecret').fill('PARTIAL_LOCK_EDITOR_ONLY_104');failLockAfterCompletion=true;
  const partialLock=await vaultAction('lock');assert.equal(partialLock.ok,false);assert.match(partialLock.error,/could not be removed/);
  assert.equal(await page.evaluate(()=>NetworkToolkitCredentials.vaultStatus().state),'locked');assert.equal(await page.evaluate(()=>NetworkToolkitCredentials.meta('password')),null);assert.equal(await page.locator('#credentialManagerForm').count(),0);assert.match(await page.locator('#credentialVaultPanel').innerText(),/may still unlock a future launch/);
  denyNext=true;assert.equal((await vaultAction('recover')).ok,false);assert.equal(vaultGeneration,2);assert.equal(vault.state,'locked');
  assert.equal((await vaultAction('recover')).ok,true);assert.equal(vaultGeneration,3);assert.equal(vault.state,'unlocked');await assertNativeOnlyEntry();
  denyNext=true;assert.equal((await vaultAction('cleanup-legacy')).ok,false);assert.equal(vault.legacyBackup,true);
  assert.equal((await vaultAction('cleanup-legacy')).ok,true);assert.equal(vault.legacyBackup,false);assert.equal(await page.locator('[data-vault-action="cleanup-legacy"]').count(),0);
  denyNext=true;assert.equal((await vaultAction('delete-vault')).ok,false);assert.equal(records.size,4);assert.equal(vault.state,'unlocked');
  assert.equal((await vaultAction('delete-vault')).ok,true);assert.equal(vault.state,'setup-required');assert.equal(records.size,0);assert.equal(await page.locator('#credentialManagerForm').count(),0);await assertNativeOnlyEntry();
  for(const row of posts('/api/credential-vault'))assert.deepEqual(Object.keys(row.body),['action']);
  for(const secret of nativeCanaries)assert(!JSON.stringify({calls,responses,saved}).includes(secret),'Native secret crossed the UI/API fixture boundary');
  await page.evaluate(()=>flushState());for(const secret of canaries)assert(!JSON.stringify(saved).includes(secret));
  assert.equal(calls.filter(row=>/^\/api\/credentials\/[^/]+$/.test(row.path)&&row.method==='GET').length,0);
  for(const response of responses.filter(row=>row.path.startsWith('/api/credentials')))for(const secret of canaries)assert(!JSON.stringify(response.data).includes(secret));
  await assertNoSecretsInDOM();assert.deepEqual(errors,[]);
  const result={passed:true,scope:'UI-only synthetic route fixture; NOT Windows native/DPAPI/bootstrap verification',blankPasswordAndKeyEditor:true,createUpdateDenialAndSuccess:true,blankUpdatePreservesSecret:true,explicitDestinationApproval:true,duplicateApprovalDeduplicated:true,staleDestinationNotGranted:true,savedConnectionsIDOnly:['terminal','sftp','ftps'],noSavedSecretReads:true,noSecretMetadataOrPersistence:true,renderedWidths:rendered,vaultStatesRendered:['legacy','setup','locked'],nativeVaultActionOnly:true,vaultSetupCancellation:true,rotationAndRecoveryMocks:true,lockPreservesConnections:true,lockHidesMetadata:true,partialLockFailureRefreshed:true,cancelledRotationPreservesEditor:true,pendingVaultActionDeduplicated:true,dismissedManagerStaysClosed:true,legacyCleanupAndDeleteConfirmationMocks:true,errors};
  if(output)fs.writeFileSync(path.join(output,'credential-security-results.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
 }catch(error){if(output)await page.screenshot({path:path.join(output,'credential-security-failure.png')}).catch(()=>{});throw error;}
 finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
