'use strict';
// Optional rendered geometry checks. Playwright and Chromium are test tools only.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const http=require('node:http');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const web=path.resolve(__dirname,'../web');
const results=[],failures=[],errors=[];
let server,browser;
const settle=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
const close=(a,b)=>Math.abs(a-b)<=1;
function check(condition,message,context){if(!condition)failures.push({message,...context});}

async function render(page,item){
 await page.evaluate(item=>{
  state.activePage=item.page;
  if(item.local)state.localTab=item.local;
  if(item.utility)state.utilityTab=item.utility;
  if(item.wifi)wifiState.view=item.wifi;
  if(item.mode){state.transferModes=state.transferModes||{};state.transferModes[item.page]=item.mode;}
  if(item.mode==='server'){
   state.transferServerSidebar=state.transferServerSidebar||{};
   state.transferServerSidebar[item.page]={width:300,collapsed:false,activityHeight:0};
  }
  // Keep the fixtures deterministic and exercise filled layouts as well as empty ones.
  system={hostname:'WORKSTATION',os:'Windows',arch:'amd64',interfaces:[{id:'ethernet',name:'Ethernet',description:'Intel(R) Ethernet Controller',up:true,addresses:['192.168.10.4/24'],gateways:['192.168.10.1'],dnsServers:['192.168.10.1']}]};
  if(item.mode==='client'&&item.page!=='tftp'){
   const kind=item.page==='sftp'?'ssh':'ftp',model=NetworkToolkitRemoteBrowser.get(kind);
   Object.assign(model,{connected:true,connectionId:kind==='ssh'?'fixture-connection':'',sessionId:''});
   for(const side of ['local','remote'])Object.assign(model[side],{path:'/example',parent:'/',home:'/example',entries:[{name:'configs',path:'/example/configs',directory:true},{name:'switch.cfg',path:'/example/switch.cfg',directory:false,size:36}],initialized:true,loading:false});
   state.transferClientLayout=state.transferClientLayout||{};
   state.transferClientLayout[item.page]={width:300,collapsed:item.collapsed!==false,tab:'settings',side:'left',activityHeight:0};
  }
  renderPage();
 },item);
 await settle(page);
}

async function inspect(page,item,viewport,theme,terminalWindow=false){
 const data=await page.evaluate(()=>{
  const node=selector=>document.querySelector(selector),box=element=>element?.getBoundingClientRect().toJSON(),style=selector=>getComputedStyle(node(selector));
  const content=style('.content'),scroll=style('#workspaceScroll'),topbar=node('.topbar');
  const visible=element=>!!element&&element.getBoundingClientRect().width>0&&element.getBoundingClientRect().height>0&&getComputedStyle(element).visibility!=='hidden';
  const root=node('.terminal-page')?node('#terminalWorkspace'):node('#toolWorkspace');
  const controls=[...document.querySelectorAll('.page-heading button,#toolCommandBar>#mobileMenu,#terminalCommandBar #mobileMenu')].filter(visible).map(element=>({label:element.getAttribute('aria-label')||element.title||element.textContent.trim(),box:box(element)}));
  return {contentPadding:[content.paddingTop,content.paddingRight,content.paddingBottom,content.paddingLeft].map(parseFloat),maxWidth:content.maxWidth,content:box(node('.content')),main:box(node('.main')),workspace:box(node('#workspaceScroll')),root:box(root),scrollPadding:[scroll.paddingTop,scroll.paddingRight,scroll.paddingBottom,scroll.paddingLeft].map(parseFloat),scrollbarGutter:scroll.scrollbarGutter,topbar:box(topbar),documentWidth:document.documentElement.scrollWidth,windowWidth:innerWidth,controls};
 });
 const context={item,viewport,theme,terminalWindow},x=terminalWindow?0:viewport.width<=600?5:10,y=terminalWindow?0:viewport.width<=600?5:8;
 check(data.contentPadding.every((value,index)=>close(value,index%2?x:y)),'Content padding differs from shared workspace gutter',{...context,data});
 check(data.maxWidth==='none','A page caps usable window width',{...context,data});
 check(close(data.content.x,data.main.x)&&close(data.content.width,data.main.width),'Content does not fill the allocated application column',{...context,data});
 check(data.scrollPadding.every(value=>value===0)&&data.scrollbarGutter==='auto','Workspace adds a second edge inset',{...context,data});
 check(close(data.workspace.x,data.content.x+x)&&close(data.workspace.right,data.content.right-x),'Workspace horizontal edges differ from shared gutter',{...context,data});
 check(data.documentWidth<=data.windowWidth+1,'Application overflows the window horizontally',{...context,data});
 check(!data.topbar||data.topbar.height===0,'Redundant topbar consumes tool workspace height',{...context,data});
 for(const control of data.controls)check(control.box.left>=data.main.left-1&&control.box.right<=data.main.right+1,'Page or application header control extends outside its allocated space',{...context,control});
 results.push({...context,...data});
}

async function main(){
 server=http.createServer((req,res)=>{
  const pathname=new URL(req.url,'http://localhost').pathname;
  if(pathname.startsWith('/api/')){res.setHeader('Content-Type','application/json');res.end(JSON.stringify({path:'/example',parent:'/',entries:[],sessions:[]}));return;}
  const filename=path.resolve(web,'.'+(pathname==='/'||pathname==='/terminal-window'?'/index.html':pathname));
  if(!filename.startsWith(web+path.sep)){res.writeHead(403);res.end();return;}
  try{
   let content=fs.readFileSync(filename);
   if(pathname==='/terminal-window')content=Buffer.from(content.toString().replace("const WINDOW_MODE='__WINDOW_MODE__';","const WINDOW_MODE='terminal';"));
   res.setHeader('Content-Type',filename.endsWith('.js')?'text/javascript':filename.endsWith('.css')?'text/css':filename.endsWith('.svg')?'image/svg+xml':'text/html');res.end(content);
  }catch{res.writeHead(404);res.end();}
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
 const page=await browser.newPage();page.on('pageerror',error=>errors.push(error.message));
 const origin='http://127.0.0.1:'+server.address().port;
 await page.goto(origin+'/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1);
 if(process.env.TOOLKIT_SCREENSHOT_ONLY){
  const output=process.env.TOOLKIT_TEST_OUTPUT||path.resolve(__dirname,'../../../validation/workspace-spacing');
  fs.mkdirSync(output,{recursive:true});
  await page.evaluate(()=>setTheme('dark'));
  const captures=[
   {name:'sftp-client-1366',viewport:{width:1366,height:900},item:{page:'sftp',mode:'client',collapsed:false}},
   {name:'sftp-server-1366',viewport:{width:1366,height:900},item:{page:'sftp',mode:'server'}},
   {name:'overview-1366',viewport:{width:1366,height:900},item:{page:'overview'}},
   {name:'diagnostic-1366',viewport:{width:1366,height:900},item:{page:'ping'}},
   {name:'terminal-1366',viewport:{width:1366,height:900},item:{page:'terminal'}},
   {name:'sftp-client-570',viewport:{width:570,height:720},item:{page:'sftp',mode:'client',collapsed:false}},
   {name:'diagnostic-570',viewport:{width:570,height:720},item:{page:'ping'}}
  ];
  for(const capture of captures){await page.setViewportSize(capture.viewport);await render(page,capture.item);await page.waitForTimeout(180);await page.screenshot({path:path.join(output,capture.name+'.png')});}
  assert.deepEqual(errors,[],'Browser script errors');
  console.log(JSON.stringify({passed:true,screenshots:captures.map(item=>item.name+'.png'),errors}));
  return;
 }
 const pages=['overview','quick','ping','trace','dns','smtp','tcp','http','tls','identify','discover','monitor','capture','editor','hosts','wol','profiles','reports','terminal'].map(page=>({page}));
 for(const local of ['ipconfig','routes','arp','connections','wifi'])pages.push({page:'local',local});
 pages.push({page:'local',local:'wifi',wifi:'diagnostic'});
 for(const utility of ['subnet','bandwidth','mac'])pages.push({page:'subnet',utility});
 for(const page of ['sftp','ftp','tftp'])for(const mode of ['client','server'])pages.push({page,mode});
 const viewports=[{width:2560,height:1440},{width:1920,height:1080},{width:1366,height:900},{width:570,height:720}];
 for(const viewport of viewports){
  await page.setViewportSize(viewport);
  for(const theme of ['light','dark']){
   await page.evaluate(theme=>setTheme(theme),theme);
   for(const item of pages){await render(page,item);await inspect(page,item,viewport,theme);}
  }
 }
 // Independent Terminal windows preserve their edge-to-edge console.
 await page.goto(origin+'/terminal-window');await page.waitForFunction(()=>window.__networkToolkitStartupState===1);
 for(const viewport of viewports){await page.setViewportSize(viewport);for(const theme of ['light','dark']){await page.evaluate(theme=>setTheme(theme),theme);await render(page,{page:'terminal'});await inspect(page,{page:'terminal'},viewport,theme,true);}}
 if(process.env.TOOLKIT_TEST_OUTPUT){fs.mkdirSync(process.env.TOOLKIT_TEST_OUTPUT,{recursive:true});fs.writeFileSync(path.join(process.env.TOOLKIT_TEST_OUTPUT,'workspace-spacing-results.json'),JSON.stringify({passed:failures.length===0&&errors.length===0,renderedCases:results.length,results,failures,errors},null,2));}
 assert.deepEqual(errors,[],'Browser script errors');
 assert.deepEqual(failures,[],'Workspace geometry inconsistencies');
 console.log(JSON.stringify({passed:true,renderedCases:results.length,toolVariants:pages.length,sharedGutters:true,fullWindowWidth:true,headerControlsContained:true,independentTerminal:true}));
}
main().catch(error=>{console.error(error);process.exitCode=1;}).finally(async()=>{await browser?.close();if(server)await new Promise(resolve=>server.close(resolve));});
