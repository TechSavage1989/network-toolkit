'use strict';
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT;
const profile=(id,name,extra={})=>({id,name,type:'ssh',host:id+'.test',port:22,username:'operator',credentialId:'vault-reference',...extra});
(async()=>{
 let saved={appUpdatePreferences:{enabled:false},terminalProfiles:[profile('core','Core',{folder:'MDF'}),profile('access','Access',{type:'telnet',folder:'IDF',port:23,credentialId:''}),profile('console','Console',{type:'serial',serialPort:'COM4',baudRate:9600,credentialId:''})],ftpProfiles:[{id:'files',name:'Files',host:'files.test',port:990,username:'operator',security:'implicit',folder:'MDF',credentialId:'ftp-vault'}],terminalRecent:[profile('recent','Recent host')],terminalSidebarTab:'saved'};
 const browser=await chromium.launch({headless:true,args:['--no-sandbox']}),page=await browser.newPage({viewport:{width:1280,height:900}}),errors=[],requests=[];
 page.on('pageerror',error=>errors.push(error.message));
 await page.route('http://toolkit.local/**',async route=>{
  const request=route.request(),p=new URL(request.url()).pathname;
  if(p.startsWith('/api/')){
   requests.push({path:p,method:request.method(),body:request.postData()?request.postDataJSON():null});let data={};
   if(p==='/api/status')data={version,os:'windows',arch:'amd64'};
   if(p==='/api/state'){if(request.method()==='GET')data=saved;else saved=request.postDataJSON();}
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminals')data=request.method()==='POST'?{session:{id:'opened-core',type:'ssh',host:'edited-core.test',username:'operator',port:22,state:'connected',profileId:'core'}}:{sessions:[]};
   if(p==='/api/credentials')data={entries:[]};
   if(p==='/api/credentials/vault-reference')data={id:'vault-reference',kind:'password',username:'operator',secret:'fixture-only-secret'};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{
   let body=fs.readFileSync(path.join(root,'web',p==='/'?'index.html':p));
   if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','folder-fixture').replaceAll('__WINDOW_MODE__','main');
   return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 const select=async value=>page.selectOption('#sessionLibraryFolder',value);
 const terminalRows=()=>page.locator('#sessionSavedList [data-session-kind="saved"]');
 const ftpRows=()=>page.locator('#sessionSavedList [data-transfer-session-kind="saved"]');
 const move=async(type,id,folder)=>{
  await page.locator('[data-session-folder="'+type+'"][data-session-folder-id="'+id+'"]').click();
  await page.fill('#sessionLibraryMoveFolder',folder);await page.locator('button[form="sessionLibraryFolderForm"]').click();
 };
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  await page.evaluate(()=>{navigate('terminal');NetworkToolkitSessionSidebar.activateTab('saved');});
  assert.equal(await terminalRows().count(),3);assert.equal(await ftpRows().count(),1);
  await select('folder:MDF');assert.equal(await terminalRows().count(),1);assert.equal(await ftpRows().count(),1);
  await page.fill('#sessionLibrarySearch','mdf');assert.equal(await terminalRows().count(),1);await page.fill('#sessionLibrarySearch','absent');assert.equal(await terminalRows().count(),0);await page.fill('#sessionLibrarySearch','');
  await move('terminal','core','  Branch  ');assert.equal(await page.evaluate(()=>state.terminalProfiles.find(row=>row.id==='core').folder),'Branch');
  await select('folder:Branch');assert.equal(await terminalRows().count(),1);assert.equal(await ftpRows().count(),0);
  await move('terminal','core','');await select('unfiled');assert.equal(await terminalRows().count(),2);
  await move('terminal','core','Branch');await select('folder:Branch');
  await page.locator('[data-session-id="core"] [data-session-entry]').click({button:'right'});await page.locator('[data-session-menu="duplicate"]').click();
  assert.equal(await page.locator('#sessionLibrarySaveFolder').inputValue(),'Branch');await page.fill('#sessionLibraryName','Core copy');await page.locator('button[form="sessionLibraryNameForm"]').click();
  assert.equal(await terminalRows().count(),2);assert.equal(await page.evaluate(()=>state.terminalProfiles.find(row=>row.name==='Core copy').folder),'Branch');
  await page.locator('[data-session-id="core"] [data-session-entry]').click({button:'right'});await page.locator('[data-session-menu="edit"]').click();
  assert.equal(await page.locator('#terminalProfileFolder').inputValue(),'Branch');await page.fill('#con-host','edited-core.test');await page.locator('[data-terminal-action="save-profile"]').click();
  assert.equal(await page.evaluate(()=>state.terminalProfiles.find(row=>row.id==='core').folder),'Branch');
  assert.equal(await page.evaluate(()=>state.terminalProfiles.find(row=>row.id==='core').credentialId),'vault-reference');
  await page.evaluate(()=>NetworkToolkitSessionSidebar.activateTab('recent'));assert.equal(await page.locator('#sessionFolderFilterGroup').isVisible(),false);
  await page.locator('[data-session-id="recent"] [data-session-entry]').click({button:'right'});await page.locator('[data-session-menu="save"]').click();
  await page.fill('#sessionLibraryName','Saved recent');await page.fill('#sessionLibrarySaveFolder','New site');await page.locator('button[form="sessionLibraryNameForm"]').click();
  await page.evaluate(()=>NetworkToolkitSessionSidebar.activateTab('saved'));await select('');await move('ftp','files','Branch');
  assert.equal(await page.evaluate(()=>state.ftpProfiles[0].folder),'Branch');
  // Both shared file-client editors retain a folder when rewriting a profile.
  await page.evaluate(()=>{
   const remote=NetworkToolkitRemoteBrowser;
   for(const kind of ['ssh','ftp']){
    const record=kind==='ssh'?state.terminalProfiles.find(row=>row.id==='core'):state.ftpProfiles[0],draft=remote.configurations[kind],model=remote.get(kind);
    Object.assign(draft,{host:record.host,username:record.username,port:record.port,fingerprint:'',credentialId:record.credentialId});model.profileId=record.id;
    if(kind==='ssh')remote.saveSharedSSHProfile(record.name);else remote.saveFTPProfile(record.name);
   }
   navigate('terminal');NetworkToolkitSessionSidebar.activateTab('saved');
  });
  assert.equal(await page.evaluate(()=>state.terminalProfiles.find(row=>row.id==='core').folder),'Branch');assert.equal(await page.evaluate(()=>state.ftpProfiles[0].folder),'Branch');
  await page.evaluate(()=>flushState());await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  await page.evaluate(()=>{navigate('terminal');NetworkToolkitSessionSidebar.activateTab('saved');});await select('folder:Branch');
  assert.equal(await terminalRows().count(),2);assert.equal(await ftpRows().count(),1);
  assert.equal(await page.evaluate(()=>state.terminalProfiles.find(row=>row.name==='Saved recent').folder),'New site');
  const cases=[];
  for(const width of [1280,768,390])for(const theme of ['light','dark'])for(const side of ['left','right']){
   await page.setViewportSize({width,height:900});await page.evaluate(({theme,side})=>{state.terminalSidebarSide=side;setTheme(theme);NetworkToolkitSessionSidebar.render();},{theme,side});
   const box=await page.locator('#sessionLibraryFolder').evaluate(el=>{const b=el.getBoundingClientRect(),p=el.closest('.terminal-sidebar-content').getBoundingClientRect(),s=getComputedStyle(el);return {left:b.left,right:b.right,parentLeft:p.left,parentRight:p.right,contentHeight:el.clientHeight-parseFloat(s.paddingTop)-parseFloat(s.paddingBottom),lineHeight:parseFloat(s.lineHeight)};});
   assert(box.left>=box.parentLeft-1&&box.right<=box.parentRight+1,JSON.stringify({width,theme,side,box}));assert(box.contentHeight>=box.lineHeight);
   cases.push({width,theme,side});
   if(output&&width===1280&&theme==='light'&&side==='right')await page.screenshot({path:path.join(output,'saved-session-folders-wide.png')});
   if(output&&width===390&&theme==='dark'&&side==='left')await page.screenshot({path:path.join(output,'saved-session-folders-compact.png')});
  }
  await page.setViewportSize({width:1280,height:900});await page.evaluate(()=>{setTheme('light');state.terminalSidebarSide='right';NetworkToolkitSessionSidebar.render();});
  await page.locator('[data-session-id="core"] [data-session-entry]').dblclick();await page.waitForFunction(()=>terminalSessions.has('opened-core'));
  const opened=requests.filter(row=>row.path==='/api/terminals'&&row.method==='POST');assert.equal(opened.length,1);assert.equal(opened[0].body.profileId,'core');assert.equal(opened[0].body.host,'edited-core.test');assert.equal(opened[0].body.password,'fixture-only-secret');assert.equal(Object.hasOwn(opened[0].body,'folder'),false);
  await page.evaluate(()=>flushState());assert(!JSON.stringify(saved).includes('fixture-only-secret'));
  assert.equal(requests.filter(row=>row.path.includes('macro')&&row.method==='PUT').length,0);assert.deepEqual(errors,[]);
  const result={passed:true,createAssignMove:true,unfiled:true,searchAndFilter:true,duplicateAndEdit:true,ftpAndSftpPreserveFolders:true,reloadPersistence:true,credentialLinkPreserved:true,folderExcludedFromConnection:true,renderedCases:cases.length,errors};
  if(output)fs.writeFileSync(path.join(output,'saved-session-folders-results.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
 }catch(error){if(output)await page.screenshot({path:path.join(output,'saved-session-folders-failure.png')}).catch(()=>{});throw error;}
 finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
