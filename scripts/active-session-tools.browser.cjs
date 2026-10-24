'use strict';
// Optional rendered checks. Reuses the authenticated Terminal session ID and
// compares the actual session-row styles instead of maintaining CSS copies.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const web=path.resolve(__dirname,'../web'),calls=[],errors=[],cases=[];
const sessions=[{id:'other-ssh',type:'ssh',state:'connected',host:'first.test',port:22,username:'other',label:'Other SSH'},{id:'active-ssh',type:'ssh',state:'connected',host:'chosen.test',port:22,username:'operator',label:'Selected SSH'}];
let browser,server;
const settle=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
async function terminal(page,position){await page.evaluate(({sessions,position})=>{
 online=false;startTerminalFeed=()=>{};startTerminalWindowRequests=()=>{};startTerminalViews=()=>{};queueTerminalResize=()=>{};
 state.activePage='terminal';state.terminalActiveActionsPosition=position;state.terminalSidebarCollapsed=true;state.transferModes={sftp:'server',ftp:'client',tftp:'client'};
 for(const info of sessions)upsertTerminalSession(info);
 terminalActive='active-ssh';terminalPanes=['active-ssh','','',''];renderPage();online=true;renderTerminalChrome();
 },{sessions,position});await settle(page);}
async function rowStyle(page,selector){return page.locator(selector).evaluate(row=>{const css=getComputedStyle(row),button=getComputedStyle(row.querySelector('.terminal-toolgroup .btn')),heading=getComputedStyle(row.querySelector('h3')),picker=getComputedStyle(row.querySelector('.terminal-active-position select')),symbol=getComputedStyle(row.querySelector('.terminal-toolgroup .btn svg'));return {background:css.backgroundColor,padding:css.padding,gap:css.gap,borderTop:css.borderTop,borderBottom:css.borderBottom,button:{fontSize:button.fontSize,minHeight:button.minHeight,padding:button.padding},heading:{fontSize:heading.fontSize,color:heading.color},picker:{fontSize:picker.fontSize,height:picker.height},symbol:{width:symbol.width,height:symbol.height}};});}
async function main(){
 server=http.createServer((req,res)=>{const pathname=new URL(req.url,'http://localhost').pathname;
  if(pathname.startsWith('/api/')){let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{let body={};try{body=JSON.parse(raw||'{}');}catch{}calls.push({path:pathname,method:req.method,body});res.setHeader('Content-Type','application/json');if(pathname==='/api/remote-browser'&&req.method==='GET')return res.end(JSON.stringify({sessions,connections:[]}));if(body.action==='probe')return res.end(JSON.stringify({supported:true}));if(body.action==='list')return res.end(JSON.stringify({path:body.path==='.'?'/home/operator':body.path||'/home/operator',parent:'/',entries:[{name:'switch.cfg',path:'/home/operator/switch.cfg',size:40,directory:false}]}));res.end(JSON.stringify({sessions:[],events:[],after:0}));});return;}
  const filename=path.resolve(web,'.'+(pathname==='/'?'/index.html':pathname));if(!filename.startsWith(web+path.sep)){res.writeHead(403);return res.end();}try{res.setHeader('Content-Type',filename.endsWith('.js')?'text/javascript':filename.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(filename));}catch{res.writeHead(404);res.end();}
 });await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});const page=await browser.newPage();page.on('pageerror',error=>errors.push(error.message));await page.goto('http://127.0.0.1:'+server.address().port+'/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1);
 for(const viewport of [{width:1600,height:1000},{width:1000,height:768},{width:820,height:620},{width:570,height:720}])for(const theme of ['dark','light'])for(const position of ['top','bottom']){
  await page.setViewportSize(viewport);await page.evaluate(theme=>setTheme(theme),theme);await terminal(page,position);
  const terminalRow=await rowStyle(page,'#terminalActiveActions');const button=page.locator('#terminalActiveActions [data-terminal-action="open-sftp"]');assert.equal(await button.textContent(),'Open SFTP');assert.equal(await button.isEnabled(),true);
  const before=calls.length;await button.click();await page.waitForFunction(()=>state.activePage==='sftp'&&NetworkToolkitRemoteBrowser.get('ssh').connected);await settle(page);
  assert.equal(await page.evaluate(()=>state.transferModes.sftp),'client');assert.equal(await page.evaluate(()=>NetworkToolkitRemoteBrowser.get('ssh').sessionId),'active-ssh');
  const requests=calls.slice(before).filter(call=>call.path==='/api/remote-browser'&&call.method==='POST');assert.ok(requests.some(call=>call.body.action==='list'&&call.body.sessionId==='active-ssh'));assert.equal(requests.some(call=>call.body.action==='connect'),false);assert.equal(requests.some(call=>Object.hasOwn(call.body,'password')||Object.hasOwn(call.body,'privateKey')),false);
  const row=page.locator('.rb-session-tools');assert.equal(await row.getAttribute('data-position'),position);assert.deepEqual(await rowStyle(page,'.rb-session-tools'),terminalRow,'SFTP action row differs from Terminal');
  assert.equal(await row.evaluate((el,position)=>position==='top'?el.nextElementSibling?.hasAttribute('data-rb-panes'):el.previousElementSibling?.hasAttribute('data-rb-panes'),position),true);
  await page.locator('.rb-client-sidebar [data-client-tab="settings"]').click();assert.equal(await page.locator('.rb-session-tools').isVisible(),true,'sidebar toggle hid active tools');
  const opposite=position==='top'?'bottom':'top';await row.locator('[data-rb-session-position]').selectOption(opposite);assert.equal(await row.getAttribute('data-position'),opposite);assert.equal(await page.evaluate(()=>state.terminalActiveActionsPosition),opposite);
  const retained=await page.evaluate(()=>{globalThis.__retainedFiles=document.querySelector('[data-rb-panes]');return !!__retainedFiles;});assert.equal(retained,true);
  await row.locator('[data-rb-session-position]').selectOption(position);assert.equal(await page.evaluate(()=>document.querySelector('[data-rb-panes]')===__retainedFiles),true,'moving row replaced file panes');
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);cases.push({viewport,theme,position,sessionId:'active-ssh',sharedStyle:true});
 }
 // Non-SSH, closed, unavailable and transferred sessions do not offer Open SFTP.
 await terminal(page,'top');for(const change of [{type:'telnet',state:'connected'},{type:'serial',state:'connected'},{type:'ssh',state:'closed'}]){await page.evaluate(change=>{Object.assign(currentTerminal().info,change);renderTerminalChrome();},change);assert.equal(await page.locator('#terminalActiveActions [data-terminal-action="open-sftp"]').isDisabled(),true);}
 await page.evaluate(()=>{Object.assign(currentTerminal().info,{type:'ssh',state:'connected'});online=false;renderTerminalChrome();});assert.equal(await page.locator('#terminalActiveActions [data-terminal-action="open-sftp"]').isDisabled(),true);
 assert.deepEqual(errors,[]);if(process.env.TOOLKIT_TEST_OUTPUT){fs.mkdirSync(process.env.TOOLKIT_TEST_OUTPUT,{recursive:true});fs.writeFileSync(path.join(process.env.TOOLKIT_TEST_OUTPUT,'active-session-tools-results.json'),JSON.stringify({passed:true,cases,errors},null,2));}
 console.log(JSON.stringify({passed:true,renderedCases:cases.length,usesSelectedAuthenticatedSession:true,noCredentialReconnect:true,sharedToolbarStyles:true,sharedPositionPreference:true,sidebarIndependent:true,unavailableSessionsDisabled:true,errors}));
}
main().catch(error=>{console.error(error);process.exitCode=1;}).finally(async()=>{await browser?.close();if(server)await new Promise(resolve=>server.close(resolve));});
