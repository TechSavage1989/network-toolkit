'use strict';
// Optional live checks. Build the Linux development engine and supply
// TOOLKIT_TEST_ENGINE, PLAYWRIGHT_MODULE and CHROMIUM_EXECUTABLE as needed.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),os=require('node:os'),net=require('node:net'),{spawn}=require('node:child_process');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const output=process.env.TOOLKIT_TEST_OUTPUT||fs.mkdtempSync(path.join(os.tmpdir(),'toolkit-sidebar-'));
fs.mkdirSync(output,{recursive:true});const data=fs.mkdtempSync(path.join(output,'engine-data-')),share=path.join(output,'share');fs.mkdirSync(share,{recursive:true});fs.writeFileSync(path.join(share,'configuration.txt'),'hostname fixture\r\n');
let engine,browser;const errors=[];let page;
async function port(){const socket=net.createServer();await new Promise(resolve=>socket.listen(0,'127.0.0.1',resolve));const p=socket.address().port;await new Promise(resolve=>socket.close(resolve));return p;}
async function start(){engine=spawn(process.env.TOOLKIT_TEST_ENGINE,['--no-browser','--data-dir',data]);return new Promise((resolve,reject)=>{let value='';const timer=setTimeout(()=>reject(Error('Engine startup timed out')),15000);engine.stdout.on('data',chunk=>{value+=chunk;const match=value.match(/http:\/\/127\.0\.0\.1:\d+\//);if(match){clearTimeout(timer);resolve(match[0]);}});engine.on('exit',code=>reject(Error('Engine exited '+code)));});}
async function stop(){if(!engine)return;const old=engine;engine=null;await new Promise(resolve=>{old.once('exit',resolve);old.kill('SIGTERM');});}
async function navigate(tool){await page.evaluate(tool=>navigate(tool),tool);}
async function main(){
 let url=await start();browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});page=await browser.newPage({viewport:{width:1600,height:1000}});page.on('pageerror',error=>errors.push(error.message));
 await page.goto(url);await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
 const sftpPort=await port(),ftpPort=await port();
 const statuses={};for(const protocol of ['sftp','ftp'])statuses[protocol]=await page.evaluate(async({protocol,port,share})=>api('/transfer/server','POST',{action:'start',protocol,root:share,bind:'127.0.0.1',port,access:'readwrite',overwrite:true,username:'operator',password:'updated-fixture-'+protocol,security:'explicit'}),{protocol,port:protocol==='sftp'?sftpPort:ftpPort,share});
 await navigate('terminal');await page.locator('[data-session-action="new"]').click();await page.locator('.modal #terminalConnectForm').waitFor({state:'visible'});
 assert.equal(await page.locator('[data-session-tab]').first().getAttribute('data-session-tab'),'saved');assert.equal(await page.locator('#terminalSidebarTabConnection,#terminalSidebarConnection').count(),0);assert.equal(await page.locator('#terminalSidebarVisibilityToggle,[data-session-action="collapse"]').count(),0);
 await page.evaluate(()=>globalThis.__originalTerminalGrid=document.querySelector('#terminalPaneGrid'));
 await page.locator('#con-host').fill('127.0.0.1');await page.locator('#con-port').fill(String(sftpPort));await page.locator('#con-username').fill('operator');await page.locator('#terminalProfileName').fill('Shared SSH profile');await page.locator('#terminalPassword').fill('initial-fixture-sftp');
 await page.locator('[data-terminal-action="save-profile"]').click();const sshProfile=await page.evaluate(()=>terminalProfileID);
 await page.locator('[data-credential-action="save-current"]').click();await page.waitForFunction(id=>!!state.terminalProfiles.find(row=>row.id===id)?.credentialId,sshProfile);
 const sshCredential=await page.evaluate(id=>state.terminalProfiles.find(row=>row.id===id).credentialId,sshProfile);
 await page.locator('#terminalPassword').fill('updated-fixture-sftp');await page.locator('[data-credential-action="save-current"]').click();
 await page.waitForFunction(async id=>(await api('/credentials/'+id)).secret==='updated-fixture-sftp',sshCredential);
 assert.equal(await page.evaluate(id=>state.terminalProfiles.find(row=>row.id===id).credentialId,sshProfile),sshCredential);
 // The original connection modal keeps the terminal panes mounted. Closing
 // it removes secrets, and reopening a saved profile retains its vault link.
 await page.locator('.modal-foot [data-action="close-modal"]').click();
 await page.locator('#terminalSidebarTabSaved').click();assert.equal(await page.locator('#terminalSidebarContent').isVisible(),false);
 await page.locator('#terminalSidebarTabSaved').click();await page.locator('#terminalSidebarTabRecent').click();await page.locator('#terminalSidebarTabSaved').click();
 await page.evaluate(id=>openTerminalConnect(state.terminalProfiles.find(row=>row.id===id),id),sshProfile);
 assert.equal(await page.evaluate(()=>globalThis.__originalTerminalGrid===document.querySelector('#terminalPaneGrid')),true);
 assert.equal(await page.locator('#terminalProfileName').inputValue(),'Shared SSH profile');assert.equal(await page.locator('#terminalPassword').inputValue(),'');
 // Full connection controls remain contained at several sizes and both docks.
 const rendered=[];
 for(const viewport of [{width:1600,height:1000},{width:1000,height:768},{width:820,height:620},{width:570,height:720}])for(const side of ['left','right']){
  await page.setViewportSize(viewport);await page.evaluate(side=>{state.terminalSidebarSide=side;NetworkToolkitSessionSidebar.render();},side);
  const layout=await page.evaluate(()=>{const panel=document.querySelector('.modal');return {viewportOverflow:document.documentElement.scrollWidth>innerWidth+1,fields:[...panel.querySelectorAll('input:not([hidden]),select,textarea,button')].filter(el=>el.getClientRects().length).map(el=>{const b=el.getBoundingClientRect(),parent=el.parentElement.getBoundingClientRect();return {id:el.id,left:b.left,right:b.right,parentLeft:parent.left,parentRight:parent.right};})};});
  assert.equal(layout.viewportOverflow,false);for(const field of layout.fields)assert.ok(field.left>=field.parentLeft-2&&field.right<=field.parentRight+2,'terminal field leaves parent: '+JSON.stringify(field));rendered.push({viewport,side});
 }
 await page.setViewportSize({width:1600,height:1000});await page.screenshot({path:path.join(output,'terminal-connection-dialog.png')});
 // SFTP reuses the profile and credential created in Terminal.
 await page.locator('.modal-foot [data-action="close-modal"]').click();
 await page.evaluate(()=>{state.transferModes={...state.transferModes,sftp:'client'};});await navigate('sftp');assert.equal(await page.locator('#terminalConnectForm').count(),0);
 await page.locator('[data-rb-setting="host"]').fill('127.0.0.1');await page.locator('[data-rb-setting="port"]').fill(String(sftpPort));await page.locator('[data-rb-setting="fingerprint"]').fill(statuses.sftp.fingerprint);
 await page.locator('[data-rb-connection-source]').selectOption('profile:'+sshProfile);await page.locator('[data-rb-action="selectSource"]').click();
 await page.waitForFunction(()=>NetworkToolkitRemoteBrowser.get('ssh').connected);assert.equal(await page.locator('[data-rb-credential]').inputValue(),sshCredential);
 // Owned session tools stay available with the sidebar collapsed. An SFTP-only
 // server can reject an interactive terminal without losing the file session.
 const ownedConnection=await page.evaluate(()=>NetworkToolkitRemoteBrowser.get('ssh').connectionId);assert.ok(ownedConnection);
 await page.locator('.rb-client-sidebar [data-client-tab][aria-selected="true"]').click();assert.ok(await page.locator('.rb-session-tools').isVisible());
 await page.locator('.rb-session-tools [data-rb-action="openTerminal"]').click();await page.waitForFunction(()=>NetworkToolkitRemoteBrowser.get('ssh').lastError.startsWith('Cannot open Terminal:'));
 assert.equal(await page.evaluate(()=>NetworkToolkitRemoteBrowser.get('ssh').connectionId),ownedConnection);assert.equal(await page.evaluate(()=>NetworkToolkitRemoteBrowser.get('ssh').connected),true);
 const retainedListing=await page.evaluate(()=>NetworkToolkitRemoteBrowser.request('ssh','list',{path:NetworkToolkitRemoteBrowser.get('ssh').remote.path||'.'}));assert.ok(Array.isArray(retainedListing.entries));
 await page.locator('.rb-session-tools [data-rb-action="disconnect"]').click();await page.waitForFunction(()=>!NetworkToolkitRemoteBrowser.get('ssh').connected&&!NetworkToolkitRemoteBrowser.get('ssh').connectionId);
 await page.locator('.rb-session-tools [data-rb-action="reconnect"]').click();await page.waitForFunction(()=>NetworkToolkitRemoteBrowser.get('ssh').connected&&!!NetworkToolkitRemoteBrowser.get('ssh').connectionId);
 assert.notEqual(await page.evaluate(()=>NetworkToolkitRemoteBrowser.get('ssh').connectionId),ownedConnection);
 await page.locator('.rb-client-sidebar [data-client-tab="settings"]').click();

 await page.locator('[data-rb-setting="password"]').fill('third-fixture-sftp');await page.locator('[data-rb-action="saveCredential"]').click();
 await page.waitForFunction(async id=>(await api('/credentials/'+id)).secret==='third-fixture-sftp',sshCredential);
 assert.equal(await page.evaluate(()=>NetworkToolkitRemoteBrowser.get('ssh').profileId),sshProfile);
 assert.equal(await page.evaluate(id=>state.terminalProfiles.find(row=>row.id===id).credentialId,sshProfile),sshCredential);
 // Profile editing from the file client and Terminal uses one modal form.
 await page.evaluate(id=>openTerminalConnect(state.terminalProfiles.find(row=>row.id===id),id),sshProfile);assert.equal(await page.locator('#terminalConnectForm').count(),1);await page.locator('[data-action="close-modal"]').first().click();
 await navigate('terminal');assert.equal(await page.locator('#terminalConnectForm').count(),0);await page.evaluate(id=>openTerminalConnect(state.terminalProfiles.find(row=>row.id===id),id),sshProfile);assert.equal(await page.locator('#terminalConnectForm').count(),1);assert.equal(await page.locator('#terminalCredentialId').inputValue(),sshCredential);
 const terminalRequests=[];page.on('request',request=>{if(request.url().endsWith('/api/terminals')&&request.method()==='POST')terminalRequests.push(request.postDataJSON());});
 await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>[...terminalSessions.values()].length>0);assert.equal(terminalRequests.at(-1)?.password,'third-fixture-sftp');
 // FTP uses the same create/update flow and then authenticates with it.
 await page.evaluate(()=>{state.transferModes={...state.transferModes,ftp:'client'};});await navigate('ftp');
 assert.equal(await page.locator('.rb-client-sidebar [data-client-tab]').first().getAttribute('data-client-tab'),'settings');assert.equal(await page.locator('.rb-sidebar-visibility,.rb-sidebar-close').count(),0);
 await page.locator('[data-rb-setting="host"]').fill('127.0.0.1');await page.locator('[data-rb-setting="port"]').fill(String(ftpPort));await page.locator('[data-rb-setting="username"]').fill('operator');await page.locator('[data-rb-setting="fingerprint"]').fill(statuses.ftp.fingerprint);await page.locator('[data-rb-setting="profileName"]').fill('FTPS profile');await page.locator('[data-rb-setting="password"]').fill('initial-fixture-ftp');
 await page.locator('[data-rb-action="saveProfile"]').click();const ftpProfile=await page.evaluate(()=>NetworkToolkitRemoteBrowser.get('ftp').profileId);
 await page.locator('[data-rb-action="saveCredential"]').click();await page.waitForFunction(id=>!!state.ftpProfiles.find(row=>row.id===id)?.credentialId,ftpProfile);
 const ftpCredential=await page.evaluate(id=>state.ftpProfiles.find(row=>row.id===id).credentialId,ftpProfile);
 await page.locator('[data-rb-setting="password"]').fill('updated-fixture-ftp');await page.locator('[data-rb-action="saveCredential"]').click();await page.waitForFunction(async id=>(await api('/credentials/'+id)).secret==='updated-fixture-ftp',ftpCredential);
 assert.equal(await page.evaluate(()=>NetworkToolkitRemoteBrowser.get('ftp').profileId),ftpProfile);
 await page.locator('[data-rb-action="connect"]').click();await page.waitForFunction(()=>NetworkToolkitRemoteBrowser.get('ftp').connected);assert.equal(await page.locator('[data-rb-setting="password"]').inputValue(),'');
 const selected=page.locator('.rb-client-sidebar [data-client-tab][aria-selected="true"]');await selected.click();assert.equal(await page.locator('.rb-client-sidebar .terminal-sidebar-content').isVisible(),false);await selected.click();assert.equal(await page.locator('.rb-client-sidebar .terminal-sidebar-content').isVisible(),true);
 await page.evaluate(()=>flushState());let saved=await page.evaluate(()=>api('/state'));const serialized=JSON.stringify(saved);for(const secret of ['initial-fixture','updated-fixture','third-fixture'])assert.equal(serialized.includes(secret),false);
 assert.equal((await page.evaluate(()=>api('/credentials'))).entries.length,2);
 // A new SFTP profile can create its own protected SSH-key credential.
 await navigate('sftp');await page.locator('[data-client-tab="saved"]').click();await page.locator('[data-rb-library-command="new"]').click();assert.equal(await page.locator('[data-client-tab="settings"]').getAttribute('aria-expanded'),'true');
 assert.equal(await page.locator('[data-rb-credential]').inputValue(),'');
 await page.locator('[data-rb-setting="profileName"]').fill('New SFTP key profile');await page.locator('[data-rb-setting="authMode"]').selectOption('key');
 await page.locator('[data-rb-setting="privateKey"]').fill('fixture-private-key');await page.locator('[data-rb-setting="passphrase"]').fill('fixture-key-passphrase');
 await page.locator('[data-rb-action="saveCredential"]').click();await page.waitForFunction(()=>!!NetworkToolkitRemoteBrowser.configurations.ssh.credentialId);
 const keyCredential=await page.evaluate(()=>NetworkToolkitRemoteBrowser.configurations.ssh.credentialId);
 assert.notEqual(keyCredential,sshCredential);await page.locator('[data-rb-action="saveProfile"]').click();
 const keyProfile=await page.evaluate(()=>NetworkToolkitRemoteBrowser.get('ssh').profileId);assert.notEqual(keyProfile,sshProfile);
 const cleanKeyState=JSON.stringify(await page.evaluate(()=>api('/state')));assert.equal(cleanKeyState.includes('fixture-private-key'),false);assert.equal(cleanKeyState.includes('fixture-key-passphrase'),false);
 const keyRecord=await page.evaluate(id=>api('/credentials/'+id),keyCredential);assert.equal(keyRecord.kind,'ssh-key');assert.equal(JSON.parse(keyRecord.secret).privateKey,'fixture-private-key');
 await navigate('ftp');assert.equal(await page.locator('[data-rb-credential] option[value="'+keyCredential+'"]').count(),0);
 await page.evaluate(()=>flushState());

 // Restart the actual application with the same data directory.
 await stop();url=await start();await page.goto(url);await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);saved=await page.evaluate(()=>api('/state'));
 assert.equal(saved.terminalProfiles.find(row=>row.id===sshProfile).credentialId,sshCredential);assert.equal(saved.ftpProfiles.find(row=>row.id===ftpProfile).credentialId,ftpCredential);
 const secrets=await page.evaluate(async({sshCredential,ftpCredential})=>({ssh:(await api('/credentials/'+sshCredential)).secret,ftp:(await api('/credentials/'+ftpCredential)).secret}),{sshCredential,ftpCredential});assert.equal(secrets.ssh,'third-fixture-sftp');assert.equal(secrets.ftp,'updated-fixture-ftp');
 await navigate('terminal');await page.locator('[data-session-action="new"]').click();await page.locator('#terminalConnectProfileSelect').selectOption(sshProfile);assert.equal(await page.locator('#terminalCredentialId').inputValue(),sshCredential);
 assert.deepEqual(errors,[]);fs.writeFileSync(path.join(output,'connection-settings-results.json'),JSON.stringify({passed:true,terminalCredentialCreateUpdate:true,sftpSharedProfileUpdate:true,ftpCredentialCreateUpdate:true,sftpKeyCredentialCreate:true,ftpFiltersSshKeys:true,ownedToolbarDisconnectReconnect:true,sftpOnlyTerminalFailureSafe:true,ftpLogin:true,terminalUsesUpdatedSftpCredential:true,applicationRestartPersistence:true,tabToggling:true,singleConnectionForm:true,renderedCases:rendered.length,errors},null,2));
 console.log(JSON.stringify({passed:true,renderedCases:rendered.length,credentialCreateUpdate:true,sharedSshProfile:true,ftpLogin:true,restartPersistence:true,errors}));
}
main().catch(async error=>{console.error(error);if(page)try{await page.screenshot({path:path.join(output,'connection-settings-failure.png')});}catch{}process.exitCode=1;}).finally(async()=>{await browser?.close();await stop();});
