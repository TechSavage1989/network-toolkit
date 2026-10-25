'use strict';
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT;
(async()=>{
 let saved={appUpdatePreferences:{enabled:false},terminalSidebarTab:'settings',terminalSidebarSide:'right',terminalSidebarWidth:300,terminalProfiles:[
  {id:'core',name:'Core switch',folder:'MDF',type:'ssh',host:'core.test',port:2222,username:'operator',credentialId:'vault-reference',sshCompatibility:'legacy'},
  {id:'telnet',name:'Lab Telnet',folder:'Lab',type:'telnet',host:'telnet.test',port:23},
  {id:'serial',name:'Console',folder:'MDF',type:'serial',serialPort:'COM4',baudRate:115200,dataBits:8,stopBits:'1',parity:'none',flowControl:'none',lineEnding:'cr',localEcho:true}
 ]},rejectNext=false,sessionNumber=0;
 const detachedViews=[],detachmentSides=[];
 const browser=await chromium.launch({headless:true,args:['--no-sandbox']}),page=await browser.newPage({viewport:{width:1280,height:900}}),errors=[],requests=[];
 page.on('pageerror',error=>errors.push(error.message));
 await page.route('http://toolkit.local/**',async route=>{
  const request=route.request(),p=new URL(request.url()).pathname;
  if(p.startsWith('/api/')){
   const body=request.postData()?request.postDataJSON():null;requests.push({path:p,method:request.method(),body});let data={};
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',terminalWindowAvailable:true};
   if(p==='/api/state'){if(request.method()==='GET')data=saved;else saved=body;}
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminal-views'){
    if(request.method()==='POST'){
     const view={id:'detached-'+body.sessionId,sessionId:body.sessionId,state:'opening',revision:0};
     detachedViews.push(view);detachmentSides.push(saved.terminalSidebarSide);data={view};
    }else data={available:true,views:detachedViews,owners:{}};
   }
   if(p==='/api/terminals'){
    if(request.method()==='POST'){
     if(rejectNext){rejectNext=false;return route.fulfill({status:400,contentType:'application/json',body:JSON.stringify({error:'Fixture connection rejected'})});}
     data={session:{id:'dialog-session-'+(++sessionNumber),type:body.type,host:body.host,port:body.port,username:body.username,serialPort:body.serialPort,state:'connected',profileId:body.profileId}};
    }else data={sessions:[]};
   }
   if(p==='/api/terminals/serial-ports')data={ports:[{name:'COM4',description:'Fixture console'}]};
   if(p==='/api/credentials')data={entries:[{id:'vault-reference',name:'Core credential',kind:'password',username:'operator'}]};
   if(p==='/api/credentials/vault-reference')data={id:'vault-reference',kind:'password',username:'operator',secret:'dialog-fixture-secret'};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{
   const file=path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));
   if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','dialog-fixture').replaceAll('__WINDOW_MODE__','main');
   return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.ico')?'image/x-icon':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 const posts=()=>requests.filter(row=>row.path==='/api/terminals'&&row.method==='POST');
 const cancel=async()=>{await page.locator('.modal-foot [data-action="close-modal"]').click();assert.equal(await page.locator('#terminalConnectForm').count(),0);};
 async function dialog(){
  await page.locator('.modal #terminalConnectForm').waitFor({state:'visible'});
  assert.equal(await page.locator('#terminalConnectForm').count(),1);assert.equal(await page.locator('.modal.wide[role="dialog"][aria-modal="true"]').count(),1);
  assert.equal(await page.locator('#terminalSessionSidebar #terminalConnectForm,#terminalSidebarTabConnection,#terminalSidebarConnection,[data-session-tab="settings"]').count(),0);
  assert.equal(await page.evaluate(()=>globalThis.__dialogGrid===document.querySelector('#terminalPaneGrid')),true);
 }
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('terminal'));
  await page.evaluate(()=>globalThis.__dialogGrid=document.querySelector('#terminalPaneGrid'));
  assert.equal(await page.evaluate(()=>NetworkToolkitSessionSidebar.activeTab()),'saved');
  assert.equal(await page.locator('#terminalConnectForm,#terminalSidebarTabConnection,#terminalSidebarConnection').count(),0);
  assert.equal(await page.locator('[data-session-tab]').first().getAttribute('data-session-tab'),'saved');
  // New tab retains its existing empty-tab behavior; choosing a protocol opens
  // the original modal instead of redirecting into the sidebar.
  await page.locator('[data-terminal-action="new"]').click();assert.equal(await page.locator('#terminalConnectForm').count(),0);
  await page.locator('[data-terminal-new-type="ssh"]').first().click();await dialog();
  assert.equal(await page.locator('#modalTitle').textContent(),'New terminal connection');
  await page.locator('#terminalPassword').fill('cancelled-dialog-secret');await cancel();assert.equal(posts().length,0);
  // The Saved/New action opens the same modal and leaves no parked secret form.
  await page.locator('[data-session-action="new"]').click();await dialog();assert.equal(await page.locator('#terminalPassword').inputValue(),'');
  await page.locator('#terminalPassword').fill('switched-dialog-secret');await page.locator('#terminalConnectProfileSelect').selectOption('serial');
  assert.equal(await page.locator('#con-serialPort').inputValue(),'COM4');assert.equal(await page.locator('#con-baudRate').inputValue(),'115200');assert.equal(await page.locator('#con-lineEnding').inputValue(),'cr');assert.equal(await page.locator('#con-localEcho').isChecked(),true);assert.equal(await page.locator('[data-terminal-secret]').count(),0);
  await page.locator('#terminalConnectProfileSelect').selectOption('core');assert.equal(await page.locator('#terminalPassword').inputValue(),'');
  assert.equal(await page.locator('#terminalProfileFolder').inputValue(),'MDF');assert.equal(await page.locator('#con-sshCompatibility').inputValue(),'legacy');await page.waitForFunction(()=>document.querySelector('#terminalCredentialId')?.value==='vault-reference');
  await page.locator('#con-host').fill('updated-core.test');await page.locator('[data-terminal-action="save-profile"]').click();
  assert.deepEqual(await page.evaluate(()=>{const row=state.terminalProfiles.find(p=>p.id==='core');return {host:row.host,folder:row.folder,credentialId:row.credentialId};}),{host:'updated-core.test',folder:'MDF',credentialId:'vault-reference'});
  await cancel();
  // Mouse, keyboard and focus remain usable in compact and short windows.
  const layouts=[];
  for(const width of [1280,768,390])for(const theme of ['light','dark'])for(const side of ['left','right'])for(const type of ['ssh','telnet','serial']){
   await page.setViewportSize({width,height:width===1280?600:900});await page.evaluate(({theme,side})=>{setTheme(theme);state.terminalSidebarSide=side;NetworkToolkitSessionSidebar.render();},{theme,side});
   await page.locator('[data-terminal-new-type="'+type+'"]').first().click();await dialog();assert.equal(await page.locator('#con-type').inputValue(),type);
   const layout=await page.locator('.modal').evaluate(el=>{
    const b=el.getBoundingClientRect(),body=el.querySelector('.modal-body'),foot=el.querySelector('.modal-foot').getBoundingClientRect(),fields=[...el.querySelectorAll('input:not([hidden]),select,textarea,button')].filter(e=>e.getClientRects().length);
    return {left:b.left,right:b.right,top:b.top,bottom:b.bottom,viewport:innerWidth,height:innerHeight,overflow:body.scrollWidth-body.clientWidth,columns:getComputedStyle(el.querySelector('.connect-grid')).gridTemplateColumns.split(' ').length,footerBottom:foot.bottom,fields:fields.map(e=>{const r=e.getBoundingClientRect();return {id:e.id,left:r.left,right:r.right};})};
   });
   assert(layout.left>=0&&layout.right<=width+1&&layout.top>=0&&layout.bottom<=layout.height+1,JSON.stringify({width,theme,side,type,layout}));assert(layout.overflow<=1);assert(layout.footerBottom<=layout.height+1);assert.equal(layout.columns,width>720?2:1);
   for(const field of layout.fields)assert(field.left>=layout.left-1&&field.right<=layout.right+1,JSON.stringify({width,theme,side,type,field}));
   layouts.push({width,theme,side,type});
   if(output&&width===1280&&theme==='light'&&side==='right'&&type==='ssh')await page.screenshot({path:path.join(output,'terminal-new-session-wide.png')});
   if(output&&width===390&&theme==='dark'&&side==='left'&&type==='serial')await page.screenshot({path:path.join(output,'terminal-new-session-compact.png')});
   await cancel();
  }
  assert.equal(posts().length,0);
  await page.setViewportSize({width:1280,height:900});await page.evaluate(()=>{setTheme('light');state.terminalSidebarSide='right';NetworkToolkitSessionSidebar.render();});
  await page.locator('[data-terminal-new-type="telnet"]').first().click();await dialog();await page.locator('#terminalConnectButton').focus();await page.keyboard.press('Tab');assert.equal(await page.evaluate(()=>document.activeElement?.getAttribute('aria-label')),'Close dialog');await page.keyboard.press('Escape');assert.equal(await page.locator('#terminalConnectForm').count(),0);
  // Edit and connect use the retained profile IDs/credentials and the original
  // destination guards. APIs are isolated fixtures, with no network transport.
  await page.locator('[data-session-id="core"] [data-session-entry]').click({button:'right'});await page.locator('[data-session-menu="edit"]').click();await dialog();assert.equal(await page.locator('#modalTitle').textContent(),'Connection: Core switch');
  rejectNext=true;await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>document.querySelector('#terminalConnectError')?.textContent.includes('Fixture connection rejected'));await dialog();assert.equal(await page.locator('#terminalPassword').inputValue(),'');
  await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>terminalSessions.has('dialog-session-1'));assert.equal(await page.locator('#terminalConnectForm').count(),0);assert.equal(posts().length,2);
  assert.equal(posts()[1].body.host,'updated-core.test');assert.equal(posts()[1].body.profileId,'core');assert.equal(posts()[1].body.password,'dialog-fixture-secret');assert.equal(Object.hasOwn(posts()[1].body,'folder'),false);
  await page.locator('[data-session-action="new"]').click();await dialog();await page.locator('#terminalConnectProfileSelect').selectOption('telnet');await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>terminalSessions.has('dialog-session-2'));assert.equal(posts().at(-1).body.type,'telnet');assert.equal(posts().at(-1).body.port,23);
  await page.locator('[data-session-action="new"]').click();await dialog();await page.locator('#terminalConnectProfileSelect').selectOption('serial');await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>terminalSessions.has('dialog-session-3'));assert.equal(posts().at(-1).body.type,'serial');assert.equal(posts().at(-1).body.serialPort,'COM4');assert.equal(posts().at(-1).body.baudRate,115200);

  const sidebarTransfers=[];
  for(const [index,side] of ['left','right'].entries()){
   await page.evaluate(side=>{
    state.terminalSidebarSide=side;NetworkToolkitSessionSidebar.render();
   },side);
   const snapshot=await page.evaluate(()=>captureTerminalWorkspace());assert.equal(snapshot.display.terminalSidebarSide,side);
   await page.evaluate(display=>{
    state.terminalSidebarSide=display.terminalSidebarSide==='left'?'right':'left';
    applyTerminalWorkspaceDisplay(display);NetworkToolkitSessionSidebar.render();
   },snapshot.display);
   const placement=await page.evaluate(()=>{
    const sidebar=document.querySelector('#terminalSessionSidebar').getBoundingClientRect(),terminal=document.querySelector('.terminal-panel').getBoundingClientRect();
    return {side:state.terminalSidebarSide,left:sidebar.left,right:sidebar.right,terminalLeft:terminal.left,terminalRight:terminal.right};
   });
   assert.equal(placement.side,side);assert(side==='left'?placement.right<=placement.terminalLeft+1:placement.left>=placement.terminalRight-1,JSON.stringify(placement));
   if(output)await page.screenshot({path:path.join(output,'terminal-sidebar-'+side+'.png')});
   // Change side and detach within the same event turn, ahead of the 550ms
   // autosave. The detach endpoint must observe the just-selected preference.
   await page.evaluate(async({side,index})=>{
    state.terminalSidebarSide=side==='left'?'right':'left';NetworkToolkitSessionSidebar.render();await flushState();
    document.querySelector('[data-session-action="swap-side"]').click();
    await detachTerminal(terminalSessions.get('dialog-session-'+(index+1)));
   },{side,index});
   assert.equal(detachmentSides[index],side);sidebarTransfers.push({side,workspaceSnapshot:true,renderedPlacement:true,immediateSessionDetach:true});
  }
  await page.evaluate(()=>flushState());for(const secret of ['dialog-fixture-secret','cancelled-dialog-secret','switched-dialog-secret'])assert(!JSON.stringify(saved).includes(secret));
  await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('terminal'));assert.equal(await page.locator('#terminalConnectForm,#terminalSidebarTabConnection,#terminalSidebarConnection').count(),0);assert.equal(await page.evaluate(()=>state.terminalSidebarTab),'saved');
  assert.deepEqual(errors,[]);
  const result={passed:true,originalModal:true,noSetupSidebar:true,migratedPreference:true,protocols:['ssh','telnet','serial'],profilesAndCredentials:true,folderPreserved:true,cancelAndEscape:true,singleForm:true,retainedTerminalGrid:true,retryAndConnect:true,noPersistedSecrets:true,renderedCases:layouts.length,sidebarTransfers,errors};
  if(output)fs.writeFileSync(path.join(output,'terminal-connection-dialog-results.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
 }catch(error){if(output)await page.screenshot({path:path.join(output,'terminal-connection-dialog-failure.png')}).catch(()=>{});throw error;}
 finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
