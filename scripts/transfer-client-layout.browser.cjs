'use strict';
// Optional rendered checks. Playwright and Chromium are test tools only.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const web=path.resolve(__dirname,'../web'),calls=[],errors=[],cases=[];
let server,browser;
const files=folder=>[{name:'configs',path:folder+'/configs',directory:true},{name:'switch.cfg',path:folder+'/switch.cfg',directory:false,size:36,modified:'2026-10-24T12:00:00Z'},...Array.from({length:30},(_,i)=>({name:'backup-'+i+'.txt',path:folder+'/backup-'+i+'.txt',directory:false,size:2048}))];
const settle=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
async function setup(page,protocol){await page.evaluate(protocol=>{
 state.activePage=protocol;state.transferModes={sftp:'client',ftp:'client',tftp:'client'};
 state.transferClientLayout=state.transferClientLayout||{};state.transferClientLayout[protocol]={width:300,collapsed:false,tab:'settings',side:'left',activityHeight:0};
 state.terminalProfiles=[{id:'shared-profile',name:'Shared SSH profile',type:'ssh',host:'example.test',port:22,username:'operator',authMethod:'password'}];
 state.ftpProfiles=[{id:'ftp-profile',name:'Backup FTP profile',host:'example.test',port:21,username:'operator',security:'explicit'}];
 if(protocol!=='tftp'){
  const k=protocol==='sftp'?'ssh':'ftp',model=NetworkToolkitRemoteBrowser.get(k),config=NetworkToolkitRemoteBrowser.configurations[k];
  Object.assign(config,{host:'example.test',port:protocol==='sftp'?22:21,username:'operator',password:'fixture',fingerprint:'SHA256:fixture'});
  Object.assign(model,{connected:true,sessionId:'',connectionId:protocol==='sftp'?'fixture-connection':'',profileId:'',saved:true});
  for(const side of ['local','remote'])Object.assign(model[side],{path:'/example',parent:'/',home:'/example',entries:[{name:'configs',path:'/example/configs',directory:true},{name:'switch.cfg',path:'/example/switch.cfg',directory:false,size:36}],initialized:true,loading:false});
  model.activity=Array.from({length:80},(_,i)=>({time:'12:00:00',text:'Downloaded switch-backup-'+i+'.cfg successfully',kind:'success'}));
 }
 renderPage();
 },protocol);await settle(page);}
async function main(){
 server=http.createServer((req,res)=>{
  const pathname=new URL(req.url,'http://localhost').pathname;
  if(pathname.startsWith('/api/')){
   let text='';req.on('data',chunk=>text+=chunk);req.on('end',()=>{
    let body={};try{body=JSON.parse(text||'{}');}catch{}
    calls.push({path:pathname,body});res.setHeader('Content-Type','application/json');
    if(pathname==='/api/remote-browser'&&req.method==='GET')return res.end(JSON.stringify({sessions:[]}));
    if(body.action==='read-text')return setTimeout(()=>res.end(JSON.stringify({content:'hostname switch\r\ninterface 1/1/1\r\n',hash:'a'.repeat(64),encoding:'UTF-8',bom:false})),250);
    const folder=body.path||'/example',delay=folder==='/slow'?350:0;
    setTimeout(()=>res.end(JSON.stringify({path:folder,parent:'/',entries:files(folder)})),delay);
   });return;
  }
  const filename=path.resolve(web,'.'+(pathname==='/'?'/index.html':pathname));
  if(!filename.startsWith(web+path.sep)){res.writeHead(403);return res.end();}
  try{res.setHeader('Content-Type',filename.endsWith('.js')?'text/javascript':filename.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(filename));}catch{res.writeHead(404);res.end();}
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
 const page=await browser.newPage();page.on('pageerror',error=>errors.push(error.message));
 await page.goto('http://127.0.0.1:'+server.address().port+'/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1);
 for(const viewport of [{width:1920,height:1080},{width:1366,height:900},{width:1000,height:768},{width:820,height:620},{width:570,height:720}]){
  await page.setViewportSize(viewport);
  for(const protocol of ['sftp','ftp','tftp'])for(const theme of ['light','dark']){
   await page.evaluate(theme=>setTheme(theme),theme);await setup(page,protocol);
   for(const side of ['left','right']){
   if(side==='right')await page.locator('[data-client-action="swap-side"]').click();
   assert.equal(await page.evaluate(protocol=>state.transferClientLayout[protocol].side,protocol),side);
   const position=await page.evaluate(()=>{const app=document.querySelector('.rb-client-layout').getBoundingClientRect(),sidebar=document.querySelector('.rb-client-sidebar').getBoundingClientRect(),primary=document.querySelector('.rb-client-primary').getBoundingClientRect();return {app:{left:app.left,right:app.right},sidebar:{left:sidebar.left,right:sidebar.right},primary:{left:primary.left,right:primary.right}};});
   assert.ok(Math.abs(side==='left'?position.sidebar.left-position.app.left:position.sidebar.right-position.app.right)<=1,'sidebar did not dock to '+side+': '+JSON.stringify(position));
   if(viewport.width>800)assert.ok(side==='left'?position.sidebar.right<position.primary.left:position.sidebar.left>position.primary.right,'sidebar order incorrect');
   assert.match(await page.locator('[data-client-action="swap-side"]').getAttribute('aria-label'),new RegExp('to '+(side==='left'?'right':'left')+'$'));
   assert.equal(await page.locator('.rb-client-sidebar .terminal-sidebar-tab').count(),3);assert.equal(await page.locator('.rb-client-sidebar .terminal-sidebar-tab').first().getAttribute('data-client-tab'),'settings');assert.equal(await page.locator('.rb-sidebar-visibility,.rb-sidebar-close').count(),0);if(protocol!=='tftp'){assert.ok(await page.locator('.rb-session-tools').isVisible());assert.ok(await page.locator('.rb-session-tools [data-rb-action="reconnect"]').isVisible());}
   assert.ok(await page.locator('.rb-client-sidebar .terminal-sidebar-content').isVisible());
   await page.locator('.rb-client-sidebar [data-client-tab][aria-selected="true"]').click();
   if(protocol!=='tftp')assert.ok(await page.locator('.rb-session-tools').isVisible(),'session toolbar disappears with sidebar');
   const data=await page.evaluate(()=>{
    const log=document.querySelector('.rb-activity'),list=log.querySelector('.rb-activity-list'),primary=document.querySelector('.rb-client-primary');
    return {height:log.getBoundingClientRect().height,listHeight:list.getBoundingClientRect().height,scrollHeight:primary.scrollHeight,clientHeight:primary.clientHeight,viewportOverflow:document.documentElement.scrollWidth>innerWidth+1,max:getComputedStyle(list).maxHeight};
   });
   assert.ok(data.height>=99,protocol+' activity is too small');assert.ok(data.listHeight>=45,protocol+' activity content is too small: '+JSON.stringify({viewport,data}));assert.equal(data.max,'none');assert.equal(data.viewportOverflow,false,'workspace overflows horizontally');
   if(protocol==='tftp'){
    await page.locator('[data-client-tab="settings"]').click();assert.ok(await page.locator('#runButton').isVisible(),'TFTP transfer button missing');assert.equal(await page.locator('#globalTarget').count(),1);await page.locator('.rb-client-sidebar [data-client-tab][aria-selected="true"]').click();
   }
   cases.push({protocol,theme,side,viewport,...data});
   await page.locator('.rb-client-sidebar [data-client-tab][aria-selected="true"]').click();
   assert.ok(await page.locator('[data-client-action="swap-side"]').isVisible(),'swap control did not return after reopening sidebar');
   }
  }
 }
 await page.setViewportSize({width:1600,height:1100});await setup(page,'sftp');
 // Sidebar tabs and collapse must never re-request the remote directory.
 const listings=()=>calls.filter(x=>x.path==='/api/remote-browser'&&x.body.action==='list').length;
 const before=listings();
 // Docking and width changes preserve the active connection and use physical
 // drag/key direction on either side, including the narrow overlay layout.
 for(const viewport of [{width:1600,height:1100},{width:570,height:720}]){
  await page.setViewportSize(viewport);await settle(page);
  for(const side of ['left','right']){
   if(await page.evaluate(()=>state.transferClientLayout.sftp.side)!==side)await page.locator('[data-client-action="swap-side"]').click();
   await page.evaluate(()=>{state.transferClientLayout.sftp.width=300;renderPage();});await settle(page);
   const split=page.locator('.rb-client-sidebar-divider'),sidebar=page.locator('.rb-client-sidebar'),box=await split.boundingBox(),initial=(await sidebar.boundingBox()).width,delta=side==='left'?48:-48;
   await page.mouse.move(box.x+box.width/2,box.y+box.height/2);await page.mouse.down();await page.mouse.move(box.x+box.width/2+delta,box.y+box.height/2,{steps:5});await page.mouse.up();
   assert.ok((await sidebar.boundingBox()).width>initial+45,side+' pointer resize failed at '+viewport.width+': '+JSON.stringify({initial,box,after:await sidebar.boundingBox(),state:await page.evaluate(()=>state.transferClientLayout.sftp),hit:await page.evaluate(({x,y})=>document.elementFromPoint(x,y)?.outerHTML.slice(0,200),{x:box.x+box.width/2,y:box.y+box.height/2})}));
   const enlarged=Number(await split.getAttribute('aria-valuenow'));await split.focus();await page.keyboard.press(side==='left'?'ArrowLeft':'ArrowRight');
   assert.equal(Number(await split.getAttribute('aria-valuenow')),enlarged-12,side+' keyboard resize direction incorrect');
   await page.keyboard.press('Home');assert.equal(Number(await split.getAttribute('aria-valuenow')),300);
   await page.locator('[data-client-tab="settings"]').click();assert.equal(await sidebar.getAttribute('class').then(x=>x.includes('is-collapsed')),true);
   await page.locator('[data-client-tab="settings"]').click();assert.ok(await page.locator('[data-client-action="swap-side"]').isVisible());
  }
 }
 await page.setViewportSize({width:1600,height:1100});await settle(page);
 assert.equal(listings(),before,'sidebar docking or resize reloaded files');
 await page.locator('[data-client-tab="saved"]').click();assert.ok(await page.getByText('Shared SSH profile',{exact:true}).isVisible());
 await page.locator('[data-client-tab="recent"]').click();await page.locator('[data-client-tab="settings"]').click();assert.equal(listings(),before);
 await page.locator('.rb-client-sidebar [data-client-tab][aria-selected="true"]').click();
 const divider=page.locator('.transfer-activity-splitter'),height=Number(await divider.getAttribute('aria-valuenow')),box=await divider.boundingBox();
 await page.mouse.move(box.x+box.width/2,box.y+3);await page.mouse.down();await page.mouse.move(box.x+box.width/2,box.y-75,{steps:7});await page.mouse.up();
 const enlarged=Number(await divider.getAttribute('aria-valuenow'));assert.ok(enlarged>height+65,'pointer activity resize failed');
 await divider.focus();await page.keyboard.press('ArrowDown');assert.equal(Number(await divider.getAttribute('aria-valuenow')),enlarged-12);
 const preference=await page.evaluate(()=>state.transferClientLayout.sftp.activityHeight);
 await page.setViewportSize({width:820,height:620});await settle(page);await page.setViewportSize({width:1600,height:1100});await settle(page);
 assert.equal(await page.evaluate(()=>state.transferClientLayout.sftp.activityHeight),preference);assert.equal(Number(await divider.getAttribute('aria-valuenow')),preference);
 // A directory double-click makes exactly one list request; file double-click
 // opens a loading editor immediately and reads the selected remote path once.
 // Scroll the target row beneath the sticky file table heading after pane resizing.
 // Keep the real pointer double-click and the single-request assertion.
 const remoteDirectory=page.locator('[data-rb-side="remote"][data-rb-item="/example/configs"]');
 await remoteDirectory.evaluate(row=>row.scrollIntoView({block:'center',inline:'nearest'}));
 await remoteDirectory.dblclick();
 await page.waitForFunction(()=>NetworkToolkitRemoteBrowser.get('ssh').remote.path==='/example/configs');assert.equal(listings(),before+1);
 const remoteFile=page.locator('[data-rb-side="remote"][data-rb-item="/example/configs/switch.cfg"]');
 const readBefore=calls.filter(x=>x.body.action==='read-text').length;
 await remoteFile.dblclick();await page.waitForSelector('#networkToolkitFileEditor:not([hidden])');
 assert.equal(await page.locator('[data-ed-text]').isDisabled(),true,'editor did not show loading immediately');
 await page.waitForFunction(()=>!document.querySelector('[data-ed-text]').disabled);
 assert.equal(await page.locator('[data-ed-text]').inputValue(),'hostname switch\ninterface 1/1/1\n');assert.equal(calls.filter(x=>x.body.action==='read-text').length,readBefore+1);
 assert.equal(calls.filter(x=>x.path==='/api/remote-browser/drag-ticket').length,0,'double-click prepared an unnecessary drag request');
 await page.locator('[data-ed="close"]').click();
 // Old responses must not replace the last requested folder.
 await page.evaluate(async()=>{const b=NetworkToolkitRemoteBrowser;await Promise.all([b.load('ssh','remote','/slow'),b.load('ssh','remote','/fast')]);});
 assert.equal(await page.evaluate(()=>NetworkToolkitRemoteBrowser.get('ssh').remote.path),'/fast');
 // The layout state survives application preference reload.
 await page.evaluate(()=>flushState());await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1);
 assert.equal(await page.evaluate(()=>state.transferClientLayout.sftp.activityHeight),preference);assert.equal(await page.evaluate(()=>state.transferClientLayout.sftp.side),'right');
 for(const protocol of ['tftp','sftp','ftp','tftp','sftp']){await setup(page,protocol);assert.equal(await page.locator('#toolConfigScroll').count(),1);assert.equal(await page.locator('#targetForm').count(),1);}
 assert.deepEqual(errors,[]);
 if(process.env.TOOLKIT_TEST_OUTPUT){fs.mkdirSync(process.env.TOOLKIT_TEST_OUTPUT,{recursive:true});await page.screenshot({path:path.join(process.env.TOOLKIT_TEST_OUTPUT,'sftp-client-layout.png')});fs.writeFileSync(path.join(process.env.TOOLKIT_TEST_OUTPUT,'client-layout-results.json'),JSON.stringify({passed:true,cases,calls: calls.filter(x=>x.body.action).map(x=>({path:x.path,action:x.body.action})),errors},null,2));}
 console.log(JSON.stringify({passed:true,renderedCases:cases.length,sidebarTabs:true,sidebarDocking:true,pointerAndKeyboardResize:true,persistence:true,singleDoubleClickRequest:true,immediateEditorFeedback:true,staleFolderResponse:true,errors}));
}
main().catch(error=>{console.error(error);process.exitCode=1;}).finally(async()=>{await browser?.close();if(server)await new Promise(resolve=>server.close(resolve));});
