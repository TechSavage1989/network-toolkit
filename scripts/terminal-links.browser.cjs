'use strict';
// Terminal links (ssh://, telnet://, serial:) delivered to the Terminal window
// open the prefilled connection form and never connect until the user confirms.
// Every API is a synthetic fixture; nothing reaches a network or device.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT;
(async()=>{
 let saved={appUpdatePreferences:{enabled:false},terminalSidebarSide:'right',terminalSidebarWidth:300,terminalConnection:{type:'ssh',host:'previous.test',port:22,username:'previous',authMethod:'key'},terminalProfiles:[
  {id:'core',name:'Core switch',folder:'MDF',type:'ssh',host:'core.test',port:2222,username:'operator',credentialId:'vault-reference'},
  {id:'telnet',name:'Lab Telnet',folder:'Lab',type:'telnet',host:'telnet.test',port:23},
  {id:'serial',name:'Console',folder:'MDF',type:'serial',serialPort:'COM4',baudRate:115200,dataBits:8,stopBits:'1',parity:'none',flowControl:'none',lineEnding:'cr',localEcho:true}
 ]};
 const queue=[],requests=[],errors=[];let seq=0,sessionNumber=0;
 const push=(config,link)=>queue.push({seq:++seq,config,link});
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']}),page=await browser.newPage({viewport:{width:1280,height:860}});
 page.on('pageerror',error=>errors.push(error.message));
 await page.route('http://toolkit.local/**',async route=>{
  const request=route.request(),url=new URL(request.url()),p=url.pathname;
  if(p.startsWith('/api/')){
   const body=request.postData()?request.postDataJSON():null;requests.push({path:p,method:request.method(),body});let data={};
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',windowMode:'terminal',terminalWindowAvailable:true,localShellAvailable:true,system:{hostname:'Synthetic link fixture',interfaces:[]}};
   if(p==='/api/state'){if(request.method()==='GET')data=saved;else saved=body;}
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminal-window/requests'){const after=Number(url.searchParams.get('after')||0);data={requests:queue.filter(r=>r.seq>after),latest:seq,oldest:queue[0]?.seq||0,lost:false,reset:false};}
   if(p==='/api/terminal-links')data={supported:true,registered:true,current:true};
   if(p==='/api/terminal-links/default-apps')data={ok:true};
   if(p==='/api/terminal-views')data={available:true,views:[],owners:{}};
   if(p==='/api/terminals'){
    if(request.method()==='POST')data={session:{id:'link-session-'+(++sessionNumber),type:body.type,host:body.host,port:body.port,username:body.username,serialPort:body.serialPort,state:'connected',profileId:body.profileId}};
    else data={sessions:[]};
   }
   if(p==='/api/terminals/serial-ports')data={ports:[{name:'COM4',description:'Fixture console'},{name:'COM7',description:'Fixture adapter'}]};
   if(p==='/api/credentials')data={vault:{available:true,state:'unlocked',remembered:true},entries:[{id:'vault-reference',name:'Core credential',kind:'password',username:'operator',bindings:[{protocol:'ssh',host:'core.test',port:2222,username:'operator'}]}]};
   if(p==='/api/credentials/vault-reference')throw Error('The browser must never retrieve a saved secret.');
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{
   const file=path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));
   if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','link-fixture').replaceAll('__WINDOW_MODE__','terminal');
   return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.ico')?'image/x-icon':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 const posts=()=>requests.filter(row=>row.path==='/api/terminals'&&row.method==='POST');
 const form=()=>page.locator('.modal #terminalConnectForm');
 const notice=()=>page.locator('#terminalLinkNotice');
 const focused=()=>page.evaluate(()=>document.activeElement?.id||'');
 const settle=ms=>page.waitForTimeout(ms);
 const cancel=async()=>{await page.locator('.modal-foot [data-action="close-modal"]').click();await form().waitFor({state:'detached'});};
 const results={};
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  // 1. SSH link with a password and an exact saved match, delivered at startup.
  push({type:'ssh',host:'core.test',port:2222,username:'operator',authMethod:'password',sshCompatibility:'modern',timeoutMs:10000,serialPort:'',baudRate:9600,dataBits:8,stopBits:'1',parity:'none',flowControl:'none',localEcho:false,lineEnding:'crlf'},{scheme:'ssh',passwordIgnored:true});
  await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  await form().waitFor({state:'visible'});await notice().waitFor({state:'visible'});
  assert.equal(await page.locator('#con-type').inputValue(),'ssh');
  assert.equal(await page.locator('#con-host').inputValue(),'core.test');
  assert.equal(await page.locator('#con-port').inputValue(),'2222');
  assert.equal(await page.locator('#con-username').inputValue(),'operator');
  assert.equal(await page.locator('#con-authMethod').inputValue(),'password','the link starts a new password connection, not the last key setting');
  assert.equal(await page.locator('#terminalPassword').inputValue(),'','the link never fills a password');
  assert.equal(await page.locator('#terminalConnectProfileSelect').inputValue(),'','no saved connection is applied without the user');
  const text=await notice().textContent();
  assert.match(text,/Opened from a link\. SSH to operator@core\.test port 2222\./);
  assert.match(text,/Passwords in links are ignored\./);
  assert.equal(await page.locator('[data-terminal-link-profile="core"]').textContent(),'Use saved connection Core switch');
  await page.waitForFunction(()=>document.activeElement?.id==='terminalPassword');
  await settle(1500);assert.equal(posts().length,0,'a link must not connect by itself');
  if(output)await page.screenshot({path:path.join(output,'terminal-link-ssh-form.png')});
  // Use the saved connection: its credential applies, and Connect gets focus.
  await page.locator('[data-terminal-link-profile="core"]').click();
  assert.equal(await page.locator('#terminalConnectProfileSelect').inputValue(),'core');
  await page.waitForFunction(()=>document.querySelector('#terminalCredentialId')?.value==='vault-reference');
  assert.match(await notice().textContent(),/Using saved connection Core switch/);
  assert.match(await notice().textContent(),/Passwords in links are ignored\./);
  await page.waitForFunction(()=>document.activeElement?.id==='terminalConnectButton');
  await settle(500);assert.equal(posts().length,0,'choosing the saved connection must not connect');
  if(output)await page.screenshot({path:path.join(output,'terminal-link-saved-match.png')});
  await page.keyboard.press('Enter');
  await page.waitForFunction(()=>terminalSessions.has('link-session-1'));
  assert.equal(posts().length,1);
  const sent=posts()[0].body;
  assert.equal(sent.host,'core.test');assert.equal(sent.port,2222);assert.equal(sent.username,'operator');assert.equal(sent.profileId,'core');assert.equal(sent.credentialId,'vault-reference');
  assert.equal(Object.hasOwn(sent,'password'),false,'no password is sent with a saved credential');
  results.ssh={prefilled:true,passwordNote:true,savedMatch:true,focus:'password then Connect',connectedOnlyAfterConfirm:true};

  // 2. Telnet link while the window is open: the form opens by itself.
  push({type:'telnet',host:'192.0.2.20',port:2323},{scheme:'telnet'});
  await form().waitFor({state:'visible'});await notice().waitFor({state:'visible'});
  assert.equal(await page.locator('#con-type').inputValue(),'telnet');
  assert.equal(await page.locator('#con-host').inputValue(),'192.0.2.20');
  assert.equal(await page.locator('#con-port').inputValue(),'2323');
  assert.doesNotMatch(await notice().textContent(),/Passwords in links/);
  assert.equal(await page.locator('[data-terminal-link-profile]').count(),0,'no saved Telnet connection matches this host and port');
  await page.waitForFunction(()=>document.activeElement?.id==='terminalConnectButton');
  await settle(1200);assert.equal(posts().length,1);
  await cancel();await settle(1200);assert.equal(posts().length,1,'cancelling a link form connects nothing');
  results.telnet={opensWhileRunning:true,noPasswordNote:true,cancelConnectsNothing:true};

  // 3. Serial link with line settings and a saved console on the same port.
  push({type:'serial',serialPort:'COM4',baudRate:57600,dataBits:7,stopBits:'2',parity:'even',flowControl:'rtscts'},{scheme:'serial'});
  // 4. A second link arrives while the first form is still open.
  push({type:'ssh',host:'192.0.2.30',port:22,username:''},{scheme:'ssh'});
  await form().waitFor({state:'visible'});await notice().waitFor({state:'visible'});
  assert.equal(await page.locator('#con-type').inputValue(),'serial');
  assert.equal(await page.locator('#con-serialPort').inputValue(),'COM4');
  assert.equal(await page.locator('#con-baudRate').inputValue(),'57600');
  assert.equal(await page.locator('#con-dataBits').inputValue(),'7');
  assert.equal(await page.locator('#con-stopBits').inputValue(),'2');
  assert.equal(await page.locator('#con-parity').inputValue(),'even');
  assert.equal(await page.locator('#con-flowControl').inputValue(),'rtscts');
  assert.match(await notice().textContent(),/Serial COM4 at 57600 baud, 7 data bits, even parity, 2 stop bits/);
  assert.equal(await page.locator('[data-terminal-link-profile="serial"]').textContent(),'Use saved connection Console');
  await page.waitForFunction(()=>document.activeElement?.id==='terminalConnectButton');
  await page.waitForFunction(()=>/1 connection request is ready to review/.test(document.querySelector('#terminalWindowRequests')?.textContent||''));
  if(output)await page.screenshot({path:path.join(output,'terminal-link-serial-form.png')});
  await cancel();
  // The waiting link opens next; with no user in the link, the user name is next.
  await form().waitFor({state:'visible'});await notice().waitFor({state:'visible'});
  assert.equal(await page.locator('#con-host').inputValue(),'192.0.2.30');
  assert.equal(await page.locator('#con-username').inputValue(),'');
  await page.waitForFunction(()=>document.activeElement?.id==='con-username');
  await cancel();await settle(1200);
  assert.equal(await form().count(),0);assert.equal(posts().length,1);
  results.serial={lineSettings:true,savedMatch:true,queuedSecondLink:true,focusMissingUser:true};

  // Links are not settings: nothing about them is saved.
  await page.evaluate(()=>flushState());
  const stored=JSON.stringify(saved);
  for(const marker of ['passwordIgnored','192.0.2.30','2323','57600'])assert(!stored.includes(marker),'saved settings contain '+marker+': '+stored.slice(Math.max(0,stored.indexOf(marker)-200),stored.indexOf(marker)+100));

  // Terminal settings > Features: Links opens Windows Default apps.
  await page.locator('#terminalSessionToggle').click();
  await page.locator('[data-terminal-settings-tab="behavior"]').click();
  const links=page.locator('#terminalLinkSettings');await links.waitFor({state:'visible'});
  assert.match(await links.textContent(),/Choose Network Toolkit Terminal for SSH, TELNET and SERIAL links in Windows Default apps\./);
  const button=links.locator('[data-terminal-link-defaults]');
  assert.equal(await button.textContent(),'Make Network Toolkit the default for SSH, Telnet and serial links');
  await links.scrollIntoViewIfNeeded();
  if(output)await page.screenshot({path:path.join(output,'terminal-link-settings.png')});
  await button.click();
  await page.waitForFunction(()=>/Windows Default apps is open/.test(document.querySelector('#toastStack')?.textContent||''));
  assert.equal(requests.filter(r=>r.path==='/api/terminal-links/default-apps'&&r.method==='POST').length,1);
  assert.equal(posts().length,1);
  results.settings={linksBlock:true,opensDefaultApps:true};
  assert.deepEqual(errors,[]);
  const result={passed:true,...results,errors};
  if(output)fs.writeFileSync(path.join(output,'terminal-links-results.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
 }catch(error){if(output)await page.screenshot({path:path.join(output,'terminal-links-failure.png')}).catch(()=>{});throw error;}
 finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
