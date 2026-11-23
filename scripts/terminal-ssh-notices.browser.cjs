'use strict';
// Production UI with synthetic API responses. This does not connect to SSH
// devices, resolve real credentials, or claim native Windows coverage.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,out=process.env.TOOLKIT_TEST_OUTPUT;
const report={passed:false,cases:[],screenshots:[],layouts:[],profileVisibility:[],errors:[]};
let browser,page;
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined});
 for(const mode of ['main','terminal','detached']){
  const context=await browser.newContext({viewport:{width:1440,height:1000}});page=await context.newPage();page.setDefaultTimeout(15000);
  page.on('pageerror',error=>report.errors.push(mode+': '+error.message));
  const initial={id:'notice-session',label:'Synthetic SSH lab',type:'ssh',host:'lab.test',port:22,username:'operator',state:'connected',cols:100,rows:28,sshAutoContinueNotices:true};
  const sessions=[initial],view={id:'notice-view',sessionId:initial.id,state:'detached',revision:0};
  const calls=[];let ownerView=mode==='detached'?view.id:'',promptSequence=0;
  let saved={activePage:'terminal',theme:'dark',appUpdatePreferences:{enabled:false},terminalConnection:{type:'ssh',host:'previous.test',sshAutoContinueNotices:false},terminalProfiles:[
   {id:'notice-profile',name:'Lab SSH profile',folder:'Lab',type:'ssh',host:'lab.test',port:22,username:'operator',credentialId:'synthetic-credential'},
   {id:'serial-profile',name:'Lab console',type:'serial',serialPort:'COM4',baudRate:115200}
  ]};
  const owners=()=>Object.fromEntries(sessions.map(session=>[session.id,{viewId:ownerView,revision:0,ready:true}]));
  await page.route('**/*',async route=>{
   const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method(),body=request.postData()?request.postDataJSON():null;
   if(url.origin!=='http://toolkit.local')return route.abort();
   if(p.startsWith('/api/')){
    calls.push({p,method,body});let data={};
    if(p==='/api/status')data={version,os:'windows',arch:'amd64',terminalWindowAvailable:true};
    if(p==='/api/state'){if(method==='GET')data=saved;else saved=body;}
    if(p==='/api/jobs')data=[];
    if(p==='/api/terminals'){
     if(method==='POST'){
      const session={id:'profile-session',label:'Lab SSH profile',state:'connected',type:body.type,host:body.host,port:body.port,username:body.username,profileId:body.profileId,sshAutoContinueNotices:body.sshAutoContinueNotices===true,cols:80,rows:24};
      sessions.push(session);data={session};
     }else data={sessions};
    }
    if(p==='/api/terminal-views')data={available:true,views:mode==='detached'?[view]:[],owners:owners()};
    if(p==='/api/terminal-views/current')data={view,session:initial,snapshot:{fresh:true,cols:100,rows:28,after:0,config:{type:'ssh',host:'lab.test',port:22,sshAutoContinueNotices:true}}};
    if(p==='/api/terminal-views/current/ready')data={view};
    if(p==='/api/credentials')data={vault:{available:true,state:'unlocked',remembered:true},entries:[{id:'synthetic-credential',name:'Lab credential',kind:'password',username:'operator',bindings:[{protocol:'ssh',host:'lab.test',port:22,username:'operator'}]}]};
    if(p==='/api/credentials/synthetic-credential')throw Error('Frontend requested a saved credential secret');
    if(p==='/api/terminals/serial-ports')data={ports:[{name:'COM4',description:'Synthetic console'}]};
    if(p==='/api/terminal-automation/targets')data={targets:sessions.map(session=>({...session,generation:1}))};
    if(p==='/api/terminal-automation/library')data={macros:[],bindings:[],revision:'notice-fixture'};
    if(p==='/api/terminal-automation/runs')data={runs:[]};
    if(p.endsWith('/respond')&&method==='POST'){
     const session=sessions.find(entry=>p==='/api/terminals/'+entry.id+'/respond');assert(session);
     assert.equal(body.promptId,session.prompt?.id);session.prompt=null;session.state=body.action==='cancel'||body.action==='reject'?'closed':'connected';
    }
    if(p.endsWith('/reconnect')&&method==='POST'){
     const session=sessions.find(entry=>p==='/api/terminals/'+entry.id+'/reconnect');assert(session);session.state='connected';data={session};
    }
    if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,80));data={sessions,events:[],next:0};}
    return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
   }
   try{
    const file=path.join(root,'web',p==='/'?'index.html':p);let content=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));
    if(p==='/')content=content.toString().replaceAll('__SESSION_TOKEN__','notice-fixture').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__',JSON.stringify(mode==='detached'?view:null)).replaceAll('__EDITOR_VIEW__','null');
    return route.fulfill({body:content,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
   }catch{return route.fulfill({status:404,body:''});}
  });
  async function ready(){await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);if(mode==='main')await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(()=>!!terminalSessions.get('notice-session')?.xterm&&terminalOwnerCanMutate(terminalSessions.get('notice-session')));}
  async function screenshot(label){if(!out)return;const name='terminal-ssh-notices-'+mode+'-'+label+'.png';await page.screenshot({path:path.join(out,name),fullPage:true});report.screenshots.push(name);}
  async function cancelDialog(){await page.locator('#modalRoot .modal-foot [data-action="close-modal"]').click();await page.locator('#terminalConnectForm').waitFor({state:'detached'});}
  async function showPrompt(value){initial.state='awaiting_input';initial.sshAutoContinueNotices=false;initial.prompt={id:'notice-'+(++promptSequence),kind:'keyboard-interactive',title:'SSH authentication challenge',fields:[],actions:['cancel','submit'],...value};await page.evaluate(info=>{upsertTerminalSession(info);selectTerminal(info.id);renderTerminalChrome();},initial);await page.locator('#terminalPromptForm').waitFor({state:'visible'});}
  const responses=()=>calls.filter(call=>call.p.endsWith('/respond'));
  await page.goto('http://toolkit.local/');await ready();
  assert.equal(await page.evaluate(()=>terminalSessions.get('notice-session').config.sshAutoContinueNotices),true,'Session info and detached snapshots retain the preference');
  if(mode!=='detached'){
   await page.evaluate(()=>openTerminalConnect());assert.equal(await page.locator('#con-sshAutoContinueNotices').isChecked(),true,'New connections default on instead of inheriting another server opt-out');await cancelDialog();
   await page.evaluate(()=>openTerminalConnect(state.terminalProfiles.find(profile=>profile.id==='notice-profile'),'notice-profile'));
   const checkbox=page.locator('#con-sshAutoContinueNotices');assert.equal(await checkbox.isChecked(),true,'Existing profiles without a notice setting default on');
   assert.match(await checkbox.locator('..').innerText(),/server asks for no answer.*Password and verification-code requests keep their normal handling/s);
   await page.locator('[data-terminal-action="save-profile"]').click();await page.evaluate(()=>flushState());
   const profile=saved.terminalProfiles.find(row=>row.id==='notice-profile');assert.equal(profile.sshAutoContinueNotices,true);assert.equal(profile.folder,'Lab');assert.equal(profile.credentialId,'synthetic-credential');
   // A click scrolls the checkbox itself into view, while its explanatory text
   // can remain below the pinned footer. Capture and verify the entire row.
   const optionRow=checkbox.locator('..');await optionRow.evaluate(label=>label.scrollIntoView({block:'center',inline:'nearest',behavior:'instant'}));
   const optionVisibility=await optionRow.evaluate(label=>{
    const row=label.getBoundingClientRect(),body=label.closest('.modal-body').getBoundingClientRect(),footer=label.closest('.modal').querySelector('.modal-foot').getBoundingClientRect();
    const controls=[label.querySelector('input'),label.querySelector('strong'),label.querySelector('p')].map(element=>{
     const r=element.getBoundingClientRect(),x=r.left+Math.min(4,r.width/2),y=r.top+r.height/2,hit=document.elementFromPoint(x,y);
     return {tag:element.tagName,top:r.top,bottom:r.bottom,unobscured:hit===element||element.contains(hit)};
    });
    return {top:row.top,bottom:row.bottom,bodyTop:body.top,bodyBottom:body.bottom,footerTop:footer.top,controls};
   });
   assert(optionVisibility.top>=optionVisibility.bodyTop&&optionVisibility.bottom<=Math.min(optionVisibility.bodyBottom,optionVisibility.footerTop),JSON.stringify(optionVisibility));
   assert(optionVisibility.controls.every(control=>control.unobscured),JSON.stringify(optionVisibility));report.profileVisibility.push({mode,...optionVisibility});await screenshot('profile-option');
   await checkbox.uncheck();await page.locator('[data-terminal-action="save-profile"]').click();await page.evaluate(()=>flushState());
   const disabledProfile=saved.terminalProfiles.find(row=>row.id==='notice-profile');assert.equal(disabledProfile.sshAutoContinueNotices,false);assert.equal(disabledProfile.folder,'Lab');assert.equal(disabledProfile.credentialId,'synthetic-credential');
   if(mode==='main'){
    await page.locator('[data-terminal-action="connection-window"]').click();
    const request=calls.find(call=>call.p==='/api/terminal-window'&&call.method==='POST');assert(request);assert.equal(request.body.config.sshAutoContinueNotices,false);assert.equal(request.body.config.profileId,'notice-profile');assert.equal(Object.hasOwn(request.body.config,'credentialId'),false);await page.waitForFunction(()=>!terminalWindowLaunchBusy);assert.equal(await page.locator('[data-terminal-action="separate-window"]:disabled').count(),0);
   }
   await page.locator('#terminalConnectButton').click();await page.waitForFunction(()=>terminalSessions.has('profile-session')&&!document.querySelector('#terminalConnectForm'));
   const connection=calls.find(call=>call.p==='/api/terminals'&&call.method==='POST');assert.equal(connection.body.sshAutoContinueNotices,false);assert.equal(connection.body.credentialId,'synthetic-credential');assert.equal(Object.hasOwn(connection.body,'password'),false);
   const captured=await page.evaluate(async()=>({snapshot:(await captureTerminalSnapshot(terminalSessions.get('profile-session'))).config,recent:state.terminalRecent.find(entry=>entry.host==='lab.test')}));
   assert.equal(captured.snapshot.sshAutoContinueNotices,false);assert.equal(captured.recent.sshAutoContinueNotices,false);
   await page.evaluate(()=>reconnectTerminal(terminalSessions.get('profile-session')));assert.equal(await page.evaluate(()=>terminalSessions.get('profile-session').config.sshAutoContinueNotices),false);
   await page.evaluate(()=>flushState());await page.reload();await ready();
   await page.evaluate(()=>openTerminalConnect(state.terminalProfiles.find(profile=>profile.id==='notice-profile'),'notice-profile'));assert.equal(await checkbox.isChecked(),false,'Saved opt-out survives reload');
   await page.locator('#terminalConnectProfileSelect').selectOption('serial-profile');assert.equal(await checkbox.count(),0,'No notice setting is offered for serial');
   await page.locator('#con-type').selectOption('ssh');assert.equal(await checkbox.isChecked(),true,'Changing a non-SSH connection to SSH applies the enabled default');
   await page.locator('#terminalConnectProfileSelect').selectOption('');assert.equal(await checkbox.isChecked(),true,'A new profile returns to the enabled default');await cancelDialog();
   report.cases.push(mode+': enabled new and missing-setting defaults; explicit opt-out, credential link, recent, snapshot, reconnect, reload and new-profile default');
  }
  await showPrompt({});
  assert.equal(await page.locator('#terminalPrompt .prompt-title').innerText(),'SSH sign-in notice');
  assert.match(await page.locator('#terminalPrompt').innerText(),/server asks for no answer.*Select Continue to acknowledge/s);
  assert.equal(await page.locator('#terminalPrompt input').count(),0);
  for(const [width,theme] of [[1440,'dark'],[mode==='main'?960:820,'light']]){
   await page.setViewportSize({width,height:900});await page.evaluate(theme=>setTheme(theme),theme);
   // Crossing the main-window navigation breakpoint animates the drawer out
   // while the workspace immediately expands. Wait for the observed geometry,
   // then check hitpoints so a temporary overlay cannot pass a box-only check.
   if(mode==='main')await page.waitForFunction(()=>{
    const sidebar=document.getElementById('sidebar'),prompt=document.getElementById('terminalPrompt');if(!sidebar||!prompt)return false;
    const bounds=sidebar.getBoundingClientRect();return matchMedia('(max-width:1100px)').matches?sidebar.inert&&bounds.right<=0.5:bounds.right<=prompt.getBoundingClientRect().left+1;
   });
   const layout=await page.locator('#terminalPrompt').evaluate(element=>{
    const bounds=element.getBoundingClientRect(),targets=[element.querySelector('.prompt-title'),...element.querySelectorAll('.help-line,button')].filter(node=>node&&node.textContent.trim());
    const visibility=targets.map(node=>{const r=node.getBoundingClientRect();return {name:node.className||node.tagName,left:r.left,right:r.right,top:r.top,bottom:r.bottom,unobscured:[r.left+Math.min(4,r.width/2),(r.left+r.right)/2,r.right-Math.min(4,r.width/2)].every(x=>{const hit=document.elementFromPoint(x,r.top+r.height/2);return hit===node||node.contains(hit);})};});
    return {left:bounds.left,right:bounds.right,overflow:element.scrollWidth-element.clientWidth,scrollX,documentOverflow:document.documentElement.scrollWidth-innerWidth,sidebarRight:document.getElementById('sidebar')?.getBoundingClientRect().right,visibility,buttons:[...element.querySelectorAll('button')].map(button=>{const r=button.getBoundingClientRect();return {left:r.left,right:r.right,bottom:r.bottom};})};
   });
   assert(layout.left>=0&&layout.right<=width+1&&layout.overflow<=1,JSON.stringify(layout));for(const button of layout.buttons)assert(button.left>=0&&button.right<=width+1&&button.bottom<=900,JSON.stringify(button));
   assert(Math.abs(layout.scrollX)<=1&&layout.documentOverflow<=1,JSON.stringify(layout));assert(layout.visibility.every(element=>element.unobscured&&element.left>=0&&element.right<=width+1&&element.top>=0&&element.bottom<=900),JSON.stringify(layout));report.layouts.push({mode,width,theme,...layout});
   await screenshot('empty-'+theme+'-'+width);
  }
  const firstNoticeId=initial.prompt.id;await page.locator('[data-terminal-prompt-action="submit"]').click();await page.locator('#terminalPromptForm').waitFor({state:'detached'});
  assert.equal(responses().length,1);assert.deepEqual(responses()[0].body,{promptId:firstNoticeId,action:'submit',answers:[]});
  await showPrompt({serverName:'Lab access notice',message:'This synthetic server requires an acknowledgement before continuing.'});
  assert.match(await page.locator('#terminalPrompt').innerText(),/Lab access notice.*synthetic server requires an acknowledgement/s);await screenshot('server-notice');
  await page.locator('[data-terminal-prompt-action="submit"]').click();await page.locator('#terminalPromptForm').waitFor({state:'detached'});assert.deepEqual(responses().at(-1).body.answers,[]);
  await showPrompt({serverName:'<img src=x onerror=alert(1)>',message:'Use <Continue> & proceed.'});
  assert.equal(await page.locator('#terminalPrompt img').count(),0);assert.match(await page.locator('#terminalPrompt').innerText(),/<img src=x onerror=alert\(1\)>/);
  assert.equal(await page.locator('#terminalPrompt .prompt-message').last().innerText(),'Use <Continue> & proceed.');
  await page.locator('[data-terminal-prompt-action="cancel"]').click();await page.locator('#terminalPromptForm').waitFor({state:'detached'});assert.equal(responses().at(-1).body.action,'cancel');
  const before=responses().length;
  await showPrompt({serverName:'Lab sign-in',message:'Enter the requested values.',fields:[{label:'Password:',echo:false},{label:'Verification code:',echo:false}]});
  assert.equal(await page.locator('#terminalPrompt .prompt-title').innerText(),'SSH authentication challenge');assert.equal(await page.locator('#terminalPrompt input[type="password"]').count(),2);assert.equal(responses().length,before);
  await page.locator('#terminalPromptField-0').fill('synthetic-password');await page.locator('#terminalPromptField-1').fill('654321');await page.locator('[data-terminal-prompt-action="submit"]').click();await page.locator('#terminalPromptForm').waitFor({state:'detached'});
  assert.deepEqual(responses().at(-1).body.answers,['synthetic-password','654321']);
  await showPrompt({kind:'host-key',title:'Verify SSH server',message:'New server identity',fingerprint:'SHA256:synthetic',actions:['reject','once','save']});
  assert.equal(await page.locator('#terminalPrompt .prompt-title').innerText(),'Verify SSH server');assert.equal(await page.locator('[data-terminal-prompt-action="save"]').isVisible(),true);assert.equal(await page.locator('[data-terminal-prompt-action="submit"]').count(),0);
  assert.match(await page.locator('#terminalPrompt').innerText(),/Verify the fingerprint through a trusted channel/);
  const priorResponses=responses().length;ownerView='another-view';
  await page.evaluate(owners=>{terminalViewRegistry.owners=owners;terminalPromptSignature='';renderTerminalPrompt();},owners());
  assert.equal(await page.locator('[data-terminal-prompt-action]').count(),0);await page.evaluate(()=>respondTerminalPrompt('save'));assert.equal(responses().length,priorResponses);
  ownerView=mode==='detached'?view.id:'';await page.evaluate(owners=>{terminalViewRegistry.owners=owners;terminalPromptSignature='';renderTerminalPrompt();},owners());
  await page.locator('[data-terminal-prompt-action="reject"]').click();await page.locator('#terminalPromptForm').waitFor({state:'detached'});
  await page.evaluate(()=>flushState());assert(!JSON.stringify(saved).includes('synthetic-password'));assert(!JSON.stringify(saved).includes('654321'));
  report.cases.push(mode+': explained blank notice, visible escaped server text, empty acknowledgement, cancel, real MFA fields, unchanged host trust, ownership guard');
  await context.close();
 }
 assert.deepEqual(report.errors,[]);report.passed=true;console.log(JSON.stringify(report));
})().catch(async error=>{report.failure=error.stack||String(error);console.error(report.failure);process.exitCode=1;if(out&&page&&!page.isClosed())await page.screenshot({path:path.join(out,'terminal-ssh-notices-failure.png')}).catch(()=>{});
}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'terminal-ssh-notices-results.json'),JSON.stringify(report,null,2));await browser?.close();});
