'use strict';
// Optional rendered regression checks. Install Playwright separately or set
// PLAYWRIGHT_MODULE to an installed copy. CHROMIUM_EXECUTABLE is optional.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const http=require('node:http');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'../web');
const out=process.env.TOOLKIT_TEST_OUTPUT;
const results=[];
const errors=[];
let browser,server;
const settle=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
async function render(page,protocol,width=320,commander=false){
 await page.evaluate(({protocol,width,commander})=>{
  system.interfaces=Array.from({length:18},(_,i)=>({name:'Ethernet '+i,description:i===0?'Realtek PCIe 5GbE Family Controller':i===1?'Intel(R) Ethernet Connection (17) I219-LM':('Virtual adapter with a very long network interface description and additional connection identifier '+i),up:true,addresses:['192.168.'+i+'.4/24']}));
  state.activePage=protocol;state.transferModes={sftp:'server',tftp:'server',ftp:'server'};
  state.transferServerSidebar=state.transferServerSidebar||{};
  state.transferServerSidebar[protocol]={width,collapsed:false,activityHeight:0};
  renderPage();
  if(commander&&protocol!=='ftp')window.NetworkToolkitTransferExplorer.mount(protocol,'/example',{root:'/example'});
  document.querySelector('#transferLogItems').innerHTML=window.NetworkToolkitTransfers.logs({transfers:Array.from({length:80},(_,i)=>({time:'2026-10-24T17:00:00Z',operation:'Download',peer:'192.168.1.5',file:'device-backup-'+i+'.cfg',bytes:2048,status:'Completed',detail:'Transfer completed successfully'}))});
 },{protocol,width,commander});
 await settle(page);
}
async function main(){
 if(out)fs.mkdirSync(out,{recursive:true});
 server=http.createServer((req,res)=>{
  const pathname=new URL(req.url,'http://localhost').pathname;
  if(pathname.startsWith('/api/')){
   res.setHeader('Content-Type','application/json');
   return res.end(JSON.stringify({path:'/example',parent:'/',roots:['/'],entries:Array.from({length:20},(_,i)=>({path:'/example/config-'+i+'.txt',name:'config-'+i+'.txt',directory:false,size:2048,modified:'2026-10-24T17:00:00Z'}))}));
  }
  const file=path.resolve(root,'.'+(pathname==='/'?'/index.html':pathname));
  if(!file.startsWith(root+path.sep)){res.writeHead(403);return res.end();}
  try{res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':file.endsWith('.svg')?'image/svg+xml':'text/html');res.end(fs.readFileSync(file));}
  catch{res.writeHead(404);res.end();}
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
 const page=await browser.newPage();page.on('pageerror',e=>errors.push(e.message));
 await page.goto('http://127.0.0.1:'+server.address().port+'/');
 await page.waitForFunction(()=>window.__networkToolkitStartupState===1);
 for(const viewport of [{width:1920,height:1080},{width:1366,height:900},{width:1000,height:768},{width:820,height:620},{width:570,height:720}]){
  await page.setViewportSize(viewport);
  for(const protocol of ['sftp','tftp','ftp'])for(const theme of ['light','dark']){
   await page.evaluate(theme=>setTheme(theme),theme);
   await render(page,protocol,320,true);
   const select=page.locator('select[data-transfer-setting="bind"]');
   assert.ok(await select.isVisible());
   assert.equal(await select.getAttribute('class'),'connection-source-select transfer-bind-picker');
   assert.equal(await select.locator('option').count(),20);
   assert.equal(await page.locator('[data-transfer-bind-value],details.transfer-bind-picker').count(),0);
   const data=await page.evaluate(()=>{
    const box=e=>e.getBoundingClientRect().toJSON();
    const select=document.querySelector('select[data-transfer-setting="bind"]'),body=document.querySelector('.transfer-server-sidebar-body');
    return {select:box(select),body:box(body),port:box(document.querySelector('[data-transfer-setting="port"]')),log:box(document.querySelector('.transfer-log-panel')),console:box(document.querySelector('.transfer-console')),settings:box(document.querySelector('.transfer-server-settings')),innerMax:getComputedStyle(document.querySelector('.transfer-log-scroll')).maxHeight,overflow:document.documentElement.scrollWidth>innerWidth+1};
   });
   assert.equal(data.overflow,false,'server workspace overflows horizontally');
   assert.equal(data.select.height,34,'server selector must match the client source control height');
   assert.equal(data.select.top,data.port.top,'adapter and listening port must align');
   assert.ok(data.select.right<=data.port.left,'adapter overlaps listening port');
   assert.ok(data.select.right<=data.body.right+1,'adapter exceeds sidebar');
   assert.ok(data.log.height>=179,'activity pane too small');
   assert.ok(data.console.height>=79,'activity content too small');
   assert.equal(data.innerMax,'none');
   await select.click();await page.keyboard.press('Escape');
   assert.equal(await page.locator('.transfer-server-settings').evaluate(e=>e.getBoundingClientRect().height),data.settings.height,'opening adapter choices resized the form');
   await select.selectOption('192.168.1.4');
   assert.match(await select.getAttribute('title'),/Intel\(R\) Ethernet Connection \(17\) I219-LM/);
   assert.equal(await page.evaluate(protocol=>state.transferServerSettings[protocol].bind,protocol),'192.168.1.4');
   if(out&&viewport.width===1366&&theme==='light'&&protocol==='sftp')await page.screenshot({path:path.join(out,'sftp-server-adapter-select.png')});
   results.push({protocol,theme,viewport,activityHeight:data.log.height,readableActivityHeight:data.console.height});
  }
 }
 // Keyboard and pointer paths operate on the same input and saved settings.
 await page.setViewportSize({width:1600,height:1100});await render(page,'sftp',270,true);
 await page.locator('select.transfer-bind-picker').focus();await page.keyboard.press('End');await page.keyboard.press('Enter');
 assert.equal(await page.locator('[data-transfer-setting="bind"]').inputValue(),'192.168.17.4');
 await page.locator('select.transfer-bind-picker').click();await page.keyboard.press('Escape');
 await page.locator('.transfer-server-sidebar-header strong').click();
 assert.equal(await page.locator('select.transfer-bind-picker').inputValue(),'192.168.17.4');
 // Compare actual SFTP/FTP client controls with the server selector, in both themes.
 for(const theme of ['light','dark'])for(const protocol of ['sftp','ftp']){
  await page.evaluate(theme=>setTheme(theme),theme);await render(page,protocol,320,true);
  const serverStyle=await page.locator('select.transfer-bind-picker').evaluate(e=>{const c=getComputedStyle(e);return [c.height,c.fontSize,c.paddingTop,c.paddingRight,c.borderRadius,c.color,c.backgroundColor];});
  await page.evaluate(protocol=>{state.transferModes[protocol]='client';renderPage();},protocol);await settle(page);
  const clientStyle=await page.locator('[data-rb-connection-source]').evaluate(e=>{const c=getComputedStyle(e);return [c.height,c.fontSize,c.paddingTop,c.paddingRight,c.borderRadius,c.color,c.backgroundColor];});
  assert.deepEqual(serverStyle,clientStyle,protocol+' server and client dropdown styling differs');
 }
 await render(page,'sftp',270,true);
 await page.locator('select.transfer-bind-picker').selectOption('192.168.17.4');
 const divider=page.locator('.transfer-activity-splitter'),initial=Number(await divider.getAttribute('aria-valuenow'));
 const b=await divider.boundingBox();await page.mouse.move(b.x+b.width/2,b.y+b.height/2);await page.mouse.down();await page.mouse.move(b.x+b.width/2,b.y-90,{steps:8});await page.mouse.up();
 const enlarged=Number(await divider.getAttribute('aria-valuenow'));assert.ok(enlarged>=initial+80,'dragging does not enlarge activity');
 await divider.focus();await page.keyboard.press('ArrowDown');assert.equal(Number(await divider.getAttribute('aria-valuenow')),enlarged-12);
 const saved=await page.evaluate(()=>state.transferServerSidebar.sftp.activityHeight);
 await page.evaluate(()=>{state.activePage='tftp';renderPage();state.activePage='sftp';renderPage();});
 assert.equal(await page.evaluate(()=>state.transferServerSidebar.sftp.activityHeight),saved);
 await page.evaluate(()=>flushState());await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1);
 assert.equal(await page.evaluate(()=>state.transferServerSidebar.sftp.activityHeight),saved);
 assert.equal(await page.evaluate(()=>state.transferServerSidebar.sftp.width),270);
 assert.equal(await page.locator('[data-transfer-setting="bind"]').inputValue(),'192.168.17.4');
 assert.ok(Number(await page.locator('.transfer-activity-splitter').getAttribute('aria-valuenow'))>=saved-1);
 // A small viewport constrains the rendered height without erasing the saved
 // height, so it is restored when the window grows again.
 await page.setViewportSize({width:820,height:620});await settle(page);
 assert.equal(await page.evaluate(()=>state.transferServerSidebar.sftp.activityHeight),saved);
 await page.setViewportSize({width:1600,height:1100});await settle(page);
 assert.equal(Number(await page.locator('.transfer-activity-splitter').getAttribute('aria-valuenow')),saved);
 await page.locator('.transfer-activity-splitter').dblclick();
 assert.equal(await page.evaluate(()=>state.transferServerSidebar.sftp.activityHeight),0);
 if(out)await page.screenshot({path:path.join(out,'sftp-server-activity.png')});
 assert.deepEqual(errors,[]);
 const summary={passed:true,renderedCases:results.length,adapterSelection:true,pointerAndKeyboardResize:true,reloadPersistence:true,viewportRecovery:true,errors,results};
 if(out)fs.writeFileSync(path.join(out,'server-layout-results.json'),JSON.stringify(summary,null,2));
 console.log(JSON.stringify(summary));
}
main().catch(error=>{console.error(error);process.exitCode=1;}).finally(async()=>{await browser?.close();if(server)await new Promise(resolve=>server.close(resolve));});
