'use strict';
// App settings > Editor "Use this copy's Text Editor in Windows" and App
// settings > Updates "Install the full version" in the portable main window.
// Every API is a synthetic fixture: no registry, network or Setup is touched.
// The engine side (registry plans, download and verification against a local
// test server) is covered by text_editor_registration_test.go and
// full_install_test.go.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT;
const copyA='C:\\Tools\\Network Toolkit\\Network_Toolkit.exe',copyB='D:\\Portable\\Network_Toolkit.exe';
const editorStatus=(state,extra={})=>({supported:true,executable:copyA,nameOk:true,installed:false,thisIsInstalled:false,state,owner:'',ownerMissing:false,complete:state==='this',contextMenu:false,canRegister:state!=='setup',needsTakeover:state==='other'||state==='unrecorded',canRemove:state==='this',message:{none:'Windows does not offer this copy\'s Text Editor.',this:'Windows offers this copy\'s Text Editor.',other:'Another copy of Network Toolkit is registered, at '+copyB+'.',setup:'The installed Network Toolkit at C:\\Users\\u\\AppData\\Local\\Programs\\Network Toolkit is registered with Windows. This copy does not change that registration. To use this copy instead, uninstall the installed program first.'}[state]||'',...extra});
const installStatus=(extra={})=>({supported:true,portable:true,version,installed:false,installedExists:false,data:{rootDir:'C:\\Users\\u\\AppData\\Local\\NetworkToolkit',explicit:false,custom:false,same:true,note:'The installed program uses the same data folder, C:\\Users\\u\\AppData\\Local\\NetworkToolkit, with the same settings and saved data.'},busy:false,started:false,...extra});
const setupName='Network_Toolkit_'+version+'_Windows_x64_Setup.exe';

async function scenario(browser,name,options,run){
 const page=await browser.newPage({viewport:{width:1280,height:900}}),errors=[],requests=[];
 const fixture={integration:{supported:true,textEditor:editorStatus('none'),install:installStatus()},...options};
 page.on('pageerror',error=>errors.push(error.message));
 await page.route('http://toolkit.local/**',async route=>{
  const request=route.request(),url=new URL(request.url()),p=url.pathname;
  if(p.startsWith('/api/')){
   const body=request.postData()?request.postDataJSON():null;requests.push({path:p,method:request.method(),body});let data={},status=200;
   if(p==='/api/status')data={version,os:fixture.os||'windows',arch:'amd64',windowMode:fixture.windowMode||'main',system:{hostname:'Synthetic Windows fixture',interfaces:[]}};
   if(p==='/api/state')data=request.method()==='GET'?{appUpdatePreferences:{enabled:false,beta:fixture.beta===true}}:{ok:true};
   if(p==='/api/jobs')data=[];
   if(p==='/api/storage')data={rootDir:'C:\\Users\\u\\AppData\\Local\\NetworkToolkit',defaultDir:'C:\\Users\\u\\AppData\\Local\\NetworkToolkit',explicit:false};
   if(p==='/api/vendors/status')data={};
   if(p==='/api/updates/status')data={currentVersion:version,supported:true,busy:[],installing:false};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
   if(p.startsWith('/api/windows-integration')){
    if(fixture.handle){const result=await fixture.handle(p,body,fixture);if(result){status=result.status||200;data=result.data;}}
    else data=fixture.integration;
    if(p==='/api/windows-integration'&&request.method()==='GET')data=fixture.integration;
   }
   return route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
  }
  try{
   const file=path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));
   if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','windows-fixture').replaceAll('__WINDOW_MODE__',fixture.windowMode||'main');
   return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 try{
  await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  await run(page,requests,fixture);
  assert.deepEqual(errors,[],name+': page errors');
  console.log('ok - '+name);
 }finally{await page.close();}
}
const openSettings=async(page,section)=>{
 await page.locator('#settingsButton').click();
 await page.locator('.ux-settings-dialog').waitFor();
 await page.locator('[data-settings-section="'+section+'"]').click();
};
const posts=(requests,suffix)=>requests.filter(r=>r.method==='POST'&&r.path==='/api/windows-integration'+suffix);
const shot=async(page,name)=>{if(output){fs.mkdirSync(output,{recursive:true});await page.screenshot({path:path.join(output,name+'.png')});}};

(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
 try{
  await scenario(browser,'register this copy with the right-click entry, then remove it',{
   handle:async(p,body,f)=>{
    if(p.endsWith('/text-editor/register')){f.integration={...f.integration,textEditor:editorStatus('this',{contextMenu:body.contextMenu})};return {data:{textEditor:f.integration.textEditor}};}
    if(p.endsWith('/text-editor/remove')){f.integration={...f.integration,textEditor:editorStatus('none')};return {data:{textEditor:f.integration.textEditor}};}
   }},async(page,requests)=>{
   await openSettings(page,'editor');
   const block=page.locator('#windowsTextEditorSettings');
   await block.locator('[data-windows-editor="register"]').waitFor();
   assert.match(await block.textContent(),/Use this copy's Text Editor in Windows/);
   assert.match(await block.locator('#windowsEditorStatus').textContent(),/does not offer/);
   assert.equal(await block.locator('[data-windows-editor="remove"]').count(),0,'nothing to remove yet');
   assert.equal(posts(requests,'/text-editor/register').length,0,'nothing is written when the section opens');
   await shot(page,'windows-editor-none');
   await block.locator('[data-windows-editor-menu]').check();
   await block.locator('[data-windows-editor="register"]').click();
   await block.locator('[data-windows-editor="remove"]').waitFor();
   assert.deepEqual(posts(requests,'/text-editor/register').map(r=>r.body),[{contextMenu:true,takeover:false}]);
   assert.match(await block.textContent(),/Registered\./);
   assert.equal(await block.locator('[data-windows-editor="register"]').textContent(),'Register again');
   assert.equal(await block.locator('[data-windows-editor-menu]').isChecked(),true,'the right-click choice is shown');
   assert.match(await block.textContent(),/Before you move or delete this folder, choose Remove/);
   await block.locator('[data-windows-editor="remove"]').click();
   await block.locator('[data-windows-editor="remove"]').waitFor({state:'detached'});
   assert.equal(posts(requests,'/text-editor/remove').length,1);
   assert.match(await block.textContent(),/Removed\./);
   // Not in the Updates section of an installed copy, and not twice.
   assert.equal(await page.locator('#windowsTextEditorSettings').count(),1);
  });

  await scenario(browser,'another copy is registered: replacing it needs a second click',{
   integration:{supported:true,textEditor:editorStatus('other',{owner:copyB,canRemove:false}),install:installStatus()},
   handle:async(p,body,f)=>{if(p.endsWith('/text-editor/register')){f.integration={...f.integration,textEditor:editorStatus('this')};return {data:{textEditor:f.integration.textEditor}};}}
  },async(page,requests)=>{
   await openSettings(page,'editor');
   const block=page.locator('#windowsTextEditorSettings');
   await block.locator('[data-windows-editor="takeover"]').waitFor();
   assert.match(await block.textContent(),/Another copy of Network Toolkit is registered, at D:\\Portable/);
   assert.equal(await block.locator('[data-windows-editor="register"]').count(),0);
   assert.equal(await block.locator('[data-windows-editor="remove"]').count(),0,'a present copy\'s registration is not removed from here');
   await block.locator('[data-windows-editor="takeover"]').click();
   await block.locator('[data-windows-editor-confirm]').waitFor();
   assert.equal(posts(requests,'/text-editor/register').length,0,'the first click only asks');
   await shot(page,'windows-editor-takeover');
   await block.locator('[data-windows-editor="takeover-cancel"]').click();
   await block.locator('[data-windows-editor-confirm]').waitFor({state:'detached'});
   assert.equal(posts(requests,'/text-editor/register').length,0,'cancel writes nothing');
   await block.locator('[data-windows-editor="takeover"]').click();
   await block.locator('[data-windows-editor="takeover-confirm"]').click();
   await block.locator('[data-windows-editor="remove"]').waitFor();
   assert.deepEqual(posts(requests,'/text-editor/register').map(r=>r.body),[{contextMenu:false,takeover:true}]);
  });

  await scenario(browser,'the installed program is registered: nothing to click',{
   integration:{supported:true,textEditor:editorStatus('setup',{installed:true,installLocation:'C:\\Users\\u\\AppData\\Local\\Programs\\Network Toolkit',canRegister:false,needsTakeover:false}),install:installStatus({installed:true,installLocation:'C:\\Users\\u\\AppData\\Local\\Programs\\Network Toolkit',installedExists:true})}
  },async(page,requests)=>{
   await openSettings(page,'editor');
   const block=page.locator('#windowsTextEditorSettings');
   await block.locator('#windowsEditorStatus').waitFor();
   assert.match(await block.textContent(),/does not change that registration/);
   assert.equal(await block.locator('button').count(),0,'no action can take over the installed registration');
   assert.equal(await block.locator('[data-windows-editor-menu]').count(),0);
   await shot(page,'windows-editor-installed');
   // Updates: already installed; offer to open it, or reinstall anyway.
   await page.locator('[data-settings-section="updates"]').click();
   const install=page.locator('#windowsFullInstall');
   await install.locator('[data-windows-install-installed]').waitFor();
   assert.match(await install.textContent(),/already installed at C:\\Users\\u\\AppData\\Local\\Programs\\Network Toolkit/);
   assert.equal(await install.locator('[data-windows-install="find"]').count(),0);
   await install.locator('[data-windows-install="open-installed"]').click();
   await page.waitForFunction(()=>/installed program is starting/.test(document.getElementById('windowsFullInstall').textContent));
   assert.deepEqual(posts(requests,'/open-installed').map(r=>r.body),[{confirm:true}]);
   await install.locator('[data-windows-install="reinstall"]').click();
   await install.locator('[data-windows-install="find"]').waitFor();
  });

  await scenario(browser,'a registration whose program is gone is offered for clean up at startup',{
   integration:{supported:true,textEditor:editorStatus('other',{owner:copyB,ownerMissing:true,canRemove:true}),install:installStatus()},
   handle:async(p,body,f)=>{if(p.endsWith('/text-editor/cleanup')){f.integration={...f.integration,textEditor:editorStatus('none')};return {data:{textEditor:f.integration.textEditor}};}}
  },async(page,requests)=>{
   const notice=page.locator('#windowsEditorCleanup');
   await notice.waitFor({timeout:10000});
   assert.match(await notice.textContent(),/D:\\Portable\\Network_Toolkit\.exe, which no longer exists/);
   assert.equal(posts(requests,'/text-editor/cleanup').length+posts(requests,'/text-editor/remove').length,0,'the startup check only reads');
   await shot(page,'windows-editor-cleanup');
   await notice.locator('[data-windows-cleanup="now"]').click();
   await notice.waitFor({state:'detached'});
   // Clean up has its own request naming the program the user saw; Remove is not used.
   assert.deepEqual(posts(requests,'/text-editor/cleanup').map(r=>r.body),[{owner:copyB}]);
   assert.equal(posts(requests,'/text-editor/remove').length,0);
  });

  await scenario(browser,'Clean up in the section names the program shown',{
   integration:{supported:true,textEditor:editorStatus('other',{owner:copyB,ownerMissing:true,canRemove:true}),install:installStatus()},
   handle:async(p,body,f)=>{if(p.endsWith('/text-editor/cleanup')){f.integration={...f.integration,textEditor:editorStatus('none')};return {data:{textEditor:f.integration.textEditor}};}}
  },async(page,requests)=>{
   await page.locator('#windowsEditorCleanup [data-windows-cleanup="later"]').click({timeout:10000});
   await openSettings(page,'editor');
   const block=page.locator('#windowsTextEditorSettings');
   await block.locator('[data-windows-editor="cleanup"]').click();
   await block.locator('[data-windows-editor="cleanup"]').waitFor({state:'detached'});
   assert.deepEqual(posts(requests,'/text-editor/cleanup').map(r=>r.body),[{owner:copyB}]);
  });

  await scenario(browser,'a program on a drive that is not connected is not offered for clean up',{
   integration:{supported:true,textEditor:editorStatus('other',{owner:copyB,ownerMissing:false,ownerUnreachable:true,canRemove:false,message:'Another copy of Network Toolkit is registered, at '+copyB+'. Its drive or network location is not connected (or could not be checked), so this copy cannot tell whether that program still exists.'}),install:installStatus()}
  },async(page,requests)=>{
   await page.waitForTimeout(2500);
   assert.equal(await page.locator('#windowsEditorCleanup').count(),0,'no startup clean up offer');
   await openSettings(page,'editor');
   const block=page.locator('#windowsTextEditorSettings');
   await block.locator('#windowsEditorStatus').waitFor();
   assert.match(await block.textContent(),/not connected/);
   assert.equal(await block.locator('[data-windows-editor="cleanup"]').count(),0);
  });

  await scenario(browser,'a copy that others can change, or with % in its path, offers no Register',{
   integration:{supported:true,textEditor:editorStatus('none',{canRegister:false,pathOk:false,locationProblem:'BUILTIN\\Users can change C:\\Shared. Once registered, Windows would run this program for every text file you double-click.',message:'Windows does not offer this copy\'s Text Editor. BUILTIN\\Users can change C:\\Shared. Once registered, Windows would run this program for every text file you double-click.'}),install:installStatus()}
  },async(page,requests)=>{
   await openSettings(page,'editor');
   const block=page.locator('#windowsTextEditorSettings');
   await block.locator('#windowsEditorStatus').waitFor();
   assert.match(await block.textContent(),/BUILTIN\\Users can change/);
   assert.equal(await block.locator('[data-windows-editor="register"]').count(),0);
  });

  // The download returns while the fixture reports progress in between.
  const found={version,name:setupName,size:12582912,beta:false,fallback:false};
  const downloaded={version,name:setupName,size:12582912,sha256:'a'.repeat(64),folder:'C:\\Users\\u\\AppData\\Local\\NetworkToolkit\\updates\\x\\.network-toolkit-setup-1'};
  await scenario(browser,'install the full version: find, consent, verify, second confirmation, start',{
   handle:async(p,body,f)=>{
    if(p.endsWith('/full-install/find')){f.integration={...f.integration,install:installStatus({found})};return {data:{install:f.integration.install}};}
    if(p.endsWith('/full-install/download')){
     f.integration={...f.integration,install:installStatus({found,busy:true,progress:{phase:'download',version,received:4194304,total:12582912}})};
     await new Promise(resolve=>setTimeout(resolve,1200));
     f.integration={...f.integration,install:installStatus({found,download:downloaded,progress:{phase:'ready',version}})};return {data:{install:f.integration.install}};
    }
    if(p.endsWith('/full-install/start')){f.integration={...f.integration,install:installStatus({found,download:downloaded,started:true})};return {data:{install:f.integration.install}};}
   }},async(page,requests)=>{
   await openSettings(page,'updates');
   const install=page.locator('#windowsFullInstall');
   await install.locator('[data-windows-install="find"]').waitFor();
   const text=await install.textContent();
   assert.match(text,/Start menu entries/);assert.match(text,/same data folder/);assert.match(text,/portable copy is left where it is/);
   assert.match(await install.locator('[data-windows-install-data]').getAttribute('data-windows-install-data'),/same/);
   await install.locator('[data-windows-install="find"]').click();
   await install.locator('[data-windows-install="download"]').waitFor();
   assert.deepEqual(posts(requests,'/full-install/find').map(r=>r.body),[{channel:'stable'}]);
   assert.match(await install.textContent(),/12\.0 MiB/);
   await install.locator('[data-windows-install="download"]').click();
   await install.locator('[data-windows-install-confirm="download"]').waitFor();
   assert.equal(posts(requests,'/full-install/download').length,0,'nothing is downloaded before consent');
   await shot(page,'windows-install-consent');
   await install.locator('[data-windows-install="download-confirm"]').click();
   await install.locator('[data-windows-install-progress]').waitFor();
   assert.match(await install.locator('[data-windows-install-progress]').textContent(),/4\.0 MiB of 12\.0 MiB \(33%\)/);
   await install.locator('[data-windows-install="start"]').waitFor();
   assert.deepEqual(posts(requests,'/full-install/download').map(r=>r.body),[{version,confirm:true}]);
   assert.match(await install.textContent(),/matches the release's SHA256SUMS/);
   await install.locator('[data-windows-install="start"]').click();
   await install.locator('[data-windows-install-confirm="start"]').waitFor();
   assert.equal(posts(requests,'/full-install/start').length,0,'Setup never starts without the second confirmation');
   await shot(page,'windows-install-start');
   await install.locator('[data-windows-install="start-confirm"]').click();
   await install.locator('[data-windows-install-started]').waitFor();
   assert.deepEqual(posts(requests,'/full-install/start').map(r=>r.body),[{version,confirm:true}]);
   assert.match(await install.textContent(),/close this portable copy and delete its folder/);
  });

  await scenario(browser,'a checksum mismatch keeps nothing and offers no start',{
   handle:async(p,body,f)=>{
    if(p.endsWith('/full-install/find')){f.integration={...f.integration,install:installStatus({found})};return {data:{install:f.integration.install}};}
    if(p.endsWith('/full-install/download'))return {status:502,data:{error:'Checksum mismatch or missing entry. Download the package and SHA256SUMS from the same official release.'}};
   }},async(page,requests)=>{
   await openSettings(page,'updates');
   const install=page.locator('#windowsFullInstall');
   await install.locator('[data-windows-install="find"]').click();
   await install.locator('[data-windows-install="download"]').click();
   await install.locator('[data-windows-install="download-confirm"]').click();
   await page.waitForFunction(()=>/Setup was not downloaded: Checksum mismatch/.test(document.getElementById('windowsFullInstall').textContent));
   assert.equal(await install.locator('[data-windows-install="start"]').count(),0,'no start without a verified download');
   assert.equal(posts(requests,'/full-install/start').length,0);
  });

  await scenario(browser,'Setup of this version is missing: a newer release is offered and the page names both versions',{
   beta:true,
   handle:async(p,body,f)=>{if(p.endsWith('/full-install/find')){f.integration={...f.integration,install:installStatus({found:{version:'99.0.0',name:'Network_Toolkit_99.0.0_Windows_x64_Setup.exe',size:1048576,beta:true,fallback:true,running:version,newer:true}})};return {data:{install:f.integration.install}};}}
  },async(page,requests)=>{
   await openSettings(page,'updates');
   const install=page.locator('#windowsFullInstall');
   await install.locator('[data-windows-install="find"]').click();
   await install.locator('[data-windows-install-fallback]').waitFor();
   const v=version.replaceAll('.','\\.');
   assert.match(await install.textContent(),new RegExp('This copy is version '+v+'\\. Its release has no Windows Setup\\. The beta release 99\\.0\\.0 is newer and has one: installing it also updates the program from '+v+' to 99\\.0\\.0'));
   assert.deepEqual(posts(requests,'/full-install/find').map(r=>r.body),[{channel:'beta'}]);
  });

  await scenario(browser,'no installer for this version and nothing newer: nothing is offered',{
   handle:async(p,body,f)=>{if(p.endsWith('/full-install/find'))return {status:502,data:{error:'No installer was found for version '+version+': its release has no Windows Setup, and no newer release offers one. no newer release with Setup'}};}
  },async(page,requests)=>{
   await openSettings(page,'updates');
   const install=page.locator('#windowsFullInstall');
   await install.locator('[data-windows-install="find"]').click();
   await page.waitForFunction(()=>/No installer was found for version/.test(document.getElementById('windowsFullInstall').textContent));
   assert.equal(await install.locator('[data-windows-install="download"]').count(),0,'no download is offered');
   assert.doesNotMatch(await install.textContent(),/updates the program/);
  });

  await scenario(browser,'an older release is never presented as an update, even if one were reported',{
   handle:async(p,body,f)=>{if(p.endsWith('/full-install/find')){f.integration={...f.integration,install:installStatus({found:{version:'1.0.1',name:'Network_Toolkit_1.0.1_Windows_x64_Setup.exe',size:1048576,beta:false,fallback:true,running:version,newer:false}})};return {data:{install:f.integration.install}};}}
  },async page=>{
   await openSettings(page,'updates');
   const install=page.locator('#windowsFullInstall');
   await install.locator('[data-windows-install="find"]').click();
   await install.locator('[data-windows-install-none]').waitFor();
   assert.equal(await install.locator('[data-windows-install="download"]').count(),0);
   assert.doesNotMatch(await install.textContent(),/updates the program|1\.0\.1/);
  });

  await scenario(browser,'a run with --data-dir is told the installed program will not see that data',{
   integration:{supported:true,textEditor:editorStatus('none'),install:installStatus({data:{rootDir:'E:\\Work Data',explicit:true,custom:true,same:false,note:'This copy was started with --data-dir and uses E:\\Work Data. The installed program\'s shortcuts do not pass that switch, so it will use C:\\Users\\u\\AppData\\Local\\NetworkToolkit and will not see this copy\'s settings and saved data.'}})}
  },async page=>{
   await openSettings(page,'updates');
   const note=page.locator('[data-windows-install-data]');
   await note.waitFor();
   assert.equal(await note.getAttribute('data-windows-install-data'),'different');
   assert.match(await note.textContent(),/will not see this copy's settings/);
  });

  await scenario(browser,'the installed program shows no install section',{
   integration:{supported:true,textEditor:editorStatus('setup',{installed:true,thisIsInstalled:true,canRegister:false,needsTakeover:false,message:'This is the installed program.'}),install:installStatus({portable:false,installed:true})}
  },async page=>{
   await openSettings(page,'updates');
   await page.locator('#windowsFullInstall').waitFor({state:'attached'});
   await page.waitForTimeout(300);
   assert.equal(await page.locator('#windowsFullInstall').isHidden(),true);
  });

  for(const [name,options] of [['Linux',{os:'linux'}]]){
   await scenario(browser,'no Windows sections in '+name,options,async(page,requests)=>{
    await openSettings(page,'editor');
    await page.waitForTimeout(400);
    assert.equal(await page.locator('#windowsTextEditorSettings').count(),0);
    assert.equal(await page.locator('#windowsFullInstall').count(),0);
    assert.equal(requests.filter(r=>r.path.startsWith('/api/windows-integration')).length,0,'nothing is asked of the engine');
   });
  }
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exit(1);});
