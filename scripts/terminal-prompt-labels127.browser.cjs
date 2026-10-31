'use strict';
// Real bundled UI/xterm, synthetic HTTP data only. No SSH connection or user data.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT,version=require('../package.json').version;
(async()=>{if(out)fs.mkdirSync(out,{recursive:true});const browser=await chromium.launch({headless:true,args:['--no-sandbox']}),page=await browser.newPage({viewport:{width:1440,height:950}}),errors=[],checks=[];page.on('pageerror',e=>errors.push(e.message));
 const base={type:'ssh',host:'192.0.2.5',port:22,username:'operator'},sessions=[
  {...base,id:'synthetic-label-one',profileId:'profile-one',label:'Custom West',state:'connected',cols:90,rows:24},
  {...base,id:'synthetic-label-two',host:'192.0.2.6',label:'SSH 192.0.2.6:22',state:'connected',cols:90,rows:24},
  {...base,id:'synthetic-label-three',host:'192.0.2.7',label:'SSH 192.0.2.7:22',state:'connected',cols:90,rows:24},
  {...base,id:'synthetic-label-four',host:'192.0.2.8',label:'SSH 192.0.2.8:22',state:'connected',cols:90,rows:24}];
 const profile=(suffix,host,name)=>({...base,id:'profile-'+suffix,host,name,credentialId:'synthetic-credential-'+suffix,authMethod:'key',sshCompatibility:'legacy'});
 const recent=(suffix,host,name)=>({...base,id:'recent-'+suffix,host,name,credentialId:'synthetic-credential-'+suffix,authMethod:'key',sshCompatibility:'legacy',lastUsed:1});
 let saved={activePage:'terminal',appUpdatePreferences:{enabled:false},terminalSidebarTab:'saved',terminalProfiles:[profile('one',base.host,'Custom West'),profile('two','192.0.2.6','192.0.2.6'),profile('three','192.0.2.7','192.0.2.7')],terminalRecent:[recent('one',base.host,'Custom West'),recent('two','192.0.2.6','SSH 192.0.2.6:22'),recent('three','192.0.2.7','192.0.2.7')]};
 let nameRevision=0,staleEvents=[],staleDeliveries=0,holdLatest=false;const heldFeeds=[];
 const sameEndpoint=(a,b)=>a.type===b.type&&a.host===b.host&&a.port===b.port&&a.username===b.username;
 function renameServer(session,label,clear=false){
  const revision=++nameRevision,explicit=!clear;
  Object.assign(session,{label:explicit?label:'SSH '+session.host+':'+session.port,manualName:explicit?label:'',nameExplicit:explicit,labelSource:explicit?'manual':'default',nameRevision:revision});
  for(const item of [...saved.terminalProfiles,...saved.terminalRecent])if(item.id===session.profileId||sameEndpoint(item,session)){if(explicit)item.name=label;item.nameExplicit=explicit;item.nameRevision=revision;}
 }
 function namesReply(){const fields=p=>({id:p.id,name:p.name,nameExplicit:p.nameExplicit===true,nameRevision:p.nameRevision||0});return {ok:true,terminalNames:{profiles:saved.terminalProfiles.map(fields),recent:saved.terminalRecent.map(fields)}};}
 function putState(incoming){
  const old=saved;saved=incoming;
  for(const profile of saved.terminalProfiles||[]){const prior=old.terminalProfiles.find(x=>x.id===profile.id);if(prior&&(prior.nameRevision||0)>(profile.nameRevision||0)){Object.assign(profile,{name:prior.name,nameExplicit:prior.nameExplicit,nameRevision:prior.nameRevision});continue;}if(prior&&(profile.name!==prior.name||profile.nameExplicit!==prior.nameExplicit)){
   const session=sessions.find(x=>x.profileId===profile.id||sameEndpoint(x,profile));if(session)renameServer(session,profile.name,profile.nameExplicit===false);
  }}
  return namesReply();
 }
 await page.route('http://toolkit.local/**',async route=>{const r=route.request(),u=new URL(r.url()),p=u.pathname;let data={};if(p.startsWith('/api/')){if(p==='/api/status')data={version,os:'windows',arch:'amd64'};if(p==='/api/state'){if(r.method()==='GET')data=saved;else data=putState(r.postDataJSON());}if(p.endsWith('/rename')&&r.method()==='POST'){const session=sessions.find(s=>p==='/api/terminals/'+s.id+'/rename');assert(session);const request=r.postDataJSON();assert(request.label||request.clear===true,'empty name requires explicit clear');renameServer(session,request.label,request.clear===true);data={session};}if(p==='/api/jobs')data=[];if(p==='/api/terminals')data={sessions};if(p==='/api/terminal-views')data={available:true,views:[],owners:{}};if(p==='/api/terminal-automation/targets')data={targets:sessions.map(s=>({...s,generation:1}))};if(p==='/api/terminal-automation/runs')data={runs:[]};if(p==='/api/terminal-automation/library')data={macros:[],bindings:[],revision:'synthetic'};if(p.includes('/events')){await new Promise(r=>setTimeout(r,80));if(staleEvents.length){data={sessions:staleEvents.shift(),events:[],next:0};staleDeliveries++;}else{if(holdLatest)await new Promise(resolve=>heldFeeds.push(resolve));data={sessions,events:[],next:0};}}return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});}try{const file=path.join(root,'web',p==='/'?'index.html':p);let content=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));if(p==='/')content=content.toString().replaceAll('__SESSION_TOKEN__','synthetic-labels').replaceAll('__WINDOW_MODE__','main').replaceAll('__TERMINAL_VIEW__','null');return route.fulfill({body:content,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}});
 try{
 await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(()=>terminalSessions.get('synthetic-label-one')?.xterm);
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-one"] .terminal-device-label').innerText(),'Custom West\n192.0.2.5');
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-two"] .terminal-device-label').innerText(),'192.0.2.6');
 await page.evaluate(()=>{for(const [id,text] of [['synthetic-label-one','Ordinary synthetic output\r\n\x1b[32mEDGE-SW01(config-if)#\x1b[0m '],['synthetic-label-two','7300# '],['synthetic-label-three','\x1b[32moperator@LAB-PRINTER:~$\x1b[0m '],['synthetic-label-four','Unrecognized ordinary output\r\n']])appendTerminalOutput(terminalSessions.get(id),new TextEncoder().encode(text));});
 await page.waitForFunction(()=>terminalSessions.get('synthetic-label-one')._ntPromptName==='EDGE-SW01'&&terminalSessions.get('synthetic-label-two')._ntPromptName==='7300'&&terminalSessions.get('synthetic-label-three')._ntPromptName==='LAB-PRINTER');
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-one"] .terminal-device-label').innerText(),'Custom West\n192.0.2.5');
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-two"] .terminal-device-label').innerText(),'7300\n192.0.2.6');
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-three"] .terminal-device-label').innerText(),'LAB-PRINTER\n192.0.2.7');
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-four"] .terminal-device-label').innerText(),'192.0.2.8');
 for(const width of [1440,820,390])for(const theme of ['dark','light']){await page.setViewportSize({width,height:950});await page.evaluate(theme=>{setTheme(theme);NetworkToolkitSessionSidebar.activateTab('saved',{expand:true});},theme);const row=page.locator('[data-session-kind="saved"][data-session-id="profile-one"]');assert(await row.isVisible(),'Saved row must actually be visible');assert.equal(await row.locator('strong').innerText(),'Custom West');assert.equal(await page.locator('[data-session-id="profile-two"] strong').innerText(),'7300');assert.equal(await page.locator('[data-session-id="profile-three"] strong').innerText(),'LAB-PRINTER');for(const [id,ip] of [['profile-two','192.0.2.6'],['profile-three','192.0.2.7']]){const r=page.locator('[data-session-id="'+id+'"]');assert.equal(await r.locator('.session-library-text small').first().innerText(),ip);assert(await r.evaluate(el=>{const n=el.querySelector('strong'),a=el.querySelector('.session-library-text small');return a.getBoundingClientRect().top>n.getBoundingClientRect().top&&parseFloat(getComputedStyle(a).fontSize)<parseFloat(getComputedStyle(n).fontSize);}),id+' name above smaller IP');}assert.equal(await row.locator('.session-library-text small').first().innerText(),'192.0.2.5');const geometry=await row.evaluate(el=>{const text=el.querySelector('.session-library-text').getBoundingClientRect(),remove=el.querySelector('.session-library-delete').getBoundingClientRect();const name=el.querySelector('strong'),address=el.querySelector('.session-library-text small');return {gap:remove.left-text.right,width:el.getBoundingClientRect().width,vertical:address.getBoundingClientRect().top-name.getBoundingClientRect().top,nameSize:parseFloat(getComputedStyle(name).fontSize),addressSize:parseFloat(getComputedStyle(address).fontSize)};});assert(geometry.width>60&&geometry.gap>=-1&&geometry.vertical>0&&geometry.addressSize<geometry.nameSize,JSON.stringify(geometry));const tabGeometry=await page.locator('[data-terminal-tab="synthetic-label-one"]').evaluate(el=>{const label=el.querySelector('.terminal-device-label'),close=el.parentElement.querySelector('.tab-close');return {gap:close.getBoundingClientRect().left-label.getBoundingClientRect().right,closeWidth:close.getBoundingClientRect().width};});assert(tabGeometry.closeWidth>0&&tabGeometry.gap>=-1,JSON.stringify(tabGeometry));if(out)await page.screenshot({path:path.join(out,`terminal-prompt-labels127-${width}-${theme}.png`),fullPage:true});checks.push({width,theme,...geometry});}
 await page.evaluate(()=>NetworkToolkitSessionSidebar.activateTab('recent',{expand:true}));
 assert.equal(await page.locator('[data-session-id="recent-one"] strong').innerText(),'Custom West');
 assert.equal(await page.locator('[data-session-id="recent-two"] strong').innerText(),'7300');
 assert.equal(await page.locator('[data-session-id="recent-three"] strong').innerText(),'LAB-PRINTER');
 await page.setViewportSize({width:1440,height:950});
 // Exercise the actual Rename tab modal and submit path, not a fixture-only state mutation.
 await page.evaluate(()=>NetworkToolkitTerminalTabs.run('rename',NetworkToolkitTerminalTabs.token('synthetic-label-one')));
 await page.locator('#terminalTabName').fill('Manual East <rack>');
 await page.locator('button[form="terminalTabRenameForm"]').click();
 await page.waitForFunction(()=>terminalSessions.get('synthetic-label-one')?.info.label==='Manual East <rack>');
 await page.waitForFunction(()=>document.querySelector('[data-terminal-tab="synthetic-label-one"] .terminal-device-label span')?.textContent==='Manual East <rack>');
 assert.equal(await page.locator('[data-session-id="recent-one"] strong').innerText(),'Manual East <rack>');
 await page.evaluate(()=>NetworkToolkitSessionSidebar.activateTab('saved',{expand:true}));
 assert.equal(await page.locator('[data-session-id="profile-one"] strong').innerText(),'Manual East <rack>');
 await page.evaluate(()=>flushState());
 for(const rows of [saved.terminalProfiles,saved.terminalRecent]){
  assert.equal(rows.length,3);assert.equal(rows[0].name,'Manual East <rack>');
  assert.equal(rows[0].host,base.host);assert.equal(rows[0].port,22);assert.equal(rows[0].username,'operator');
  assert.equal(rows[0].credentialId,'synthetic-credential-one');assert.equal(rows[0].authMethod,'key');assert.equal(rows[0].sshCompatibility,'legacy');
 }
 assert(saved.terminalPromptNames.some(x=>x.name==='7300'));assert(saved.terminalPromptNames.some(x=>x.name==='LAB-PRINTER'));
 const staleB=JSON.parse(JSON.stringify(sessions));
 await page.evaluate(()=>NetworkToolkitSessionSidebar.run('rename',NetworkToolkitSessionSidebar.token('saved','profile-one')));
 await page.locator('#sessionLibraryName').fill('Profile C');
 await page.locator('button[form="sessionLibraryNameForm"]').click();
 await page.waitForFunction(()=>terminalSessions.get('synthetic-label-one')?.info.manualName==='Profile C');
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-one"] .terminal-device-label').innerText(),'Profile C\n192.0.2.5');
 await page.evaluate(()=>flushState());const revisionC=sessions[0].nameRevision;assert(revisionC>staleB[0].nameRevision);
 await page.evaluate(()=>NetworkToolkitTerminalTabs.run('rename',NetworkToolkitTerminalTabs.token('synthetic-label-one')));
 await page.locator('[data-terminal-clear-name]').click();
 await page.waitForFunction(()=>terminalSessions.get('synthetic-label-one')?.info.nameExplicit===false);
 await page.waitForFunction(()=>document.querySelector('[data-terminal-tab="synthetic-label-one"] .terminal-device-label span')?.textContent==='EDGE-SW01');
 assert(sessions[0].nameRevision>revisionC);await page.evaluate(()=>flushState());
 assert.equal(saved.terminalProfiles[0].name,'Profile C');assert.equal(saved.terminalProfiles[0].nameExplicit,false);
 const clearRevision=saved.terminalProfiles[0].nameRevision;
 holdLatest=true;staleEvents.push(staleB);
 await page.waitForFunction(()=>document.querySelector('[data-terminal-tab="synthetic-label-one"] .terminal-device-label span')?.textContent==='EDGE-SW01');
 // Wait until the real polling route delivered the delayed pre-clear snapshot.
 for(let i=0;i<100&&!staleDeliveries;i++)await page.waitForTimeout(50);
 assert.equal(staleDeliveries,1);await page.waitForTimeout(350);
 assert.equal(await page.evaluate(()=>terminalSessions.get('synthetic-label-one').info.nameRevision),clearRevision);
 assert.equal(await page.evaluate(()=>terminalSessions.get('synthetic-label-one').info.manualName),'');
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-one"] .terminal-device-label').innerText(),'EDGE-SW01\n192.0.2.5');
 await page.evaluate(()=>flushState());for(const rows of [saved.terminalProfiles,saved.terminalRecent]){assert.equal(rows[0].name,'Profile C');assert.equal(rows[0].nameExplicit,false);assert.equal(rows[0].nameRevision,clearRevision);}
 holdLatest=false;for(const resolve of heldFeeds.splice(0))resolve();
 for(const rows of [saved.terminalProfiles,saved.terminalRecent]){assert.equal(rows[0].credentialId,'synthetic-credential-one');assert.equal(rows[0].authMethod,'key');assert.equal(rows[0].host,base.host);assert.equal(rows[0].username,base.username);}

 await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(()=>!!document.querySelector('[data-terminal-tab="synthetic-label-one"]'));
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-one"] .terminal-device-label').innerText(),'EDGE-SW01\n192.0.2.5');
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-two"] .terminal-device-label').innerText(),'7300\n192.0.2.6');
 assert.equal(await page.locator('[data-terminal-tab="synthetic-label-three"] .terminal-device-label').innerText(),'LAB-PRINTER\n192.0.2.7');
 // One viewport, real xterm output/observer/rendering for representative prompt families.
 const familyChecks=[],families=[
  ['operator@JUNIPER-LAB#','JUNIPER-LAB'],
  ['RP/0/RP0/CPU0:XR-LAB(config)#','XR-LAB'],
  ['[operator@ROUTER-LAB] /ip/route<SAFE>','ROUTER-LAB'],
  ['<COMWARE-LAB>','COMWARE-LAB'],
  ['[COMWARE-LAB-GigabitEthernet1/0/1]','COMWARE-LAB'],
  ['(ARUBA-LAB) [mynode] (config) #','ARUBA-LAB'],
  ['*A:NOKIA-LAB>config>router>if#','NOKIA-LAB'],
  ['root@F5-LAB(cfg-sync Standalone)(Active)(/Common)(tmos)#','F5-LAB'],
  ['* Slot-1 STACK-LAB.3 #','STACK-LAB'],
  ['[root@ESXI-LAB:~]','ESXI-LAB'],
  ['[SERVER-LAB]: PS C:\tools>','SERVER-LAB'],
  ['CLUSTER-LAB::>','CLUSTER-LAB']
 ];
 await page.evaluate(()=>selectTerminal('synthetic-label-four'));
 for(const [prompt,want] of families){
  await page.evaluate(prompt=>appendTerminalOutput(terminalSessions.get('synthetic-label-four'),new TextEncoder().encode('\r\n'+prompt)),prompt);
  await page.waitForFunction(prompt=>{const e=terminalSessions.get('synthetic-label-four'),b=e?.xterm?.buffer.active;return b?.getLine(b.baseY+b.cursorY)?.translateToString(true)===prompt&&!e.renderPendingBytes;},prompt);
  await page.waitForFunction(want=>terminalSessions.get('synthetic-label-four')?._ntPromptName===want,want);
  // Same-host configuration contexts must also settle without replacing the known hostname.
  await page.waitForTimeout(350);
  const tab=page.locator('[data-terminal-tab="synthetic-label-four"] .terminal-device-label');
  assert.equal(await tab.innerText(),want+'\n192.0.2.8');
  assert(await tab.evaluate(el=>{const n=el.querySelector('span'),a=el.querySelector('small');return el.getBoundingClientRect().width>0&&a.getBoundingClientRect().top>n.getBoundingClientRect().top&&parseFloat(getComputedStyle(a).fontSize)<parseFloat(getComputedStyle(n).fontSize);}));
  familyChecks.push({prompt,hostname:want});
 }
 if(out)await page.screenshot({path:path.join(out,'terminal-prompt-labels128-families.png'),fullPage:true});
 assert.deepEqual(errors,[]);const report={passed:true,checks,familyChecks,restoredMetadata:true,manualPrecedenceRenameClearAndStaleRevision:true,numericAndLinuxPrompts:true,generatedDefaultsIgnored:true,recentIdentityAndCredentialsPreserved:true,scope:'Synthetic real Chromium/xterm only; no live device or native WebView2 claim',errors};if(out)fs.writeFileSync(path.join(out,'terminal-prompt-labels127-results.json'),JSON.stringify(report,null,2));console.log(JSON.stringify(report));
 }catch(error){
 const diagnostics=await page.evaluate(()=>({startup:window.__networkToolkitStartupState,online,activePage:state.activePage,moduleAvailable:!!window.NetworkToolkitTerminalPromptLabels,registry:state.terminalPromptNames,entries:[...terminalSessions].map(([id,e])=>{const term=e.xterm,b=term?.buffer.active,line=b?.getLine(b.baseY+b.cursorY),text=line?.translateToString(true);return {id,info:e.info,learned:e._ntPromptName,outputRevision:e.outputRevision,renderPendingBytes:e.renderPendingBytes,inputBytes:e.inputBytes,inputSending:e.inputSending,inputEpoch:e.inputEpoch,resizeSending:e.resizeSending,lastResize:e.lastResize,desiredResize:e.desiredResize,resizeDueAt:e.resizeDueAt,transferBusy:e.transferBusy,restoring:e.restoring,awaitingAcceptance:e.awaitingAcceptance,owned:terminalOwnerCanMutate(e),ownerRevision:terminalSessionRevision(id),hasXterm:!!term,normalBuffer:b===term?.buffer.normal,bufferType:b?.type,cursorX:b?.cursorX,cursorY:b?.cursorY,baseY:b?.baseY,cols:term?.cols,rows:term?.rows,bracketedPasteMode:term?.modes?.bracketedPasteMode,lineWrapped:line?.isWrapped,line:text,parsed:window.NetworkToolkitTerminalPromptLabels?.parse(text),cursorRemainder:text?.slice(b.cursorX),tabText:document.querySelector('[data-terminal-tab="'+id+'"]')?.innerText};})})).catch(diagnosticError=>({diagnosticError:diagnosticError.message}));
 const failure={message:error.message,stack:error.stack,errors,diagnostics};console.error('PROMPT_LABELS127_FAILURE '+JSON.stringify(failure));
 if(out){await page.screenshot({path:path.join(out,'terminal-prompt-labels127-failure.png'),fullPage:true}).catch(()=>{});fs.writeFileSync(path.join(out,'terminal-prompt-labels127-failure.json'),JSON.stringify(failure,null,2));}throw error;}finally{holdLatest=false;for(const resolve of heldFeeds.splice(0))resolve();await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
