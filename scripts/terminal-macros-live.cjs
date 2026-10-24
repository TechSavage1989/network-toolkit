'use strict';
// Optional shipped-app regression. All UI code/CSS must arrive over the app's
// own HTTP routes. This test never injects scripts/styles or intercepts requests.
// NTK_APP_EXECUTABLE=/path/to/native/app CHROME_EXECUTABLE=/path/to/chrome \
//   node scripts/terminal-macros-live.cjs
const fs=require('node:fs'),path=require('node:path'),os=require('node:os'),net=require('node:net'),assert=require('node:assert/strict'),{spawn}=require('node:child_process');
const ASSETS=[['/assets/terminal-macro-model.js','text/javascript'],['/assets/terminal-macros.js','text/javascript'],['/assets/terminal-macros.css','text/css']];
const HELP='Usage: NTK_APP_EXECUTABLE=/absolute/path/to/app [CHROME_EXECUTABLE=/path/to/chrome] [NTK_BROWSER_OUTPUT_DIR=/path/to/results] node scripts/terminal-macros-live.cjs\nThe installed executable, Playwright and browser are used as-is; nothing is downloaded. A temporary app data folder and local Telnet peers are removed on exit.';
if(process.argv.includes('--help')){console.log(HELP);process.exit(0);}
if(!process.env.NTK_APP_EXECUTABLE){console.error(HELP);process.exit(2);}
function loadPlaywright(){try{return require('playwright');}catch(error){if(error.code!=='MODULE_NOT_FOUND')throw error;const base=process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES;if(base)return require(path.join(base,'playwright'));throw new Error('Install Playwright in the development environment to run this optional check.');}}
const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));
const dataDir=fs.mkdtempSync(path.join(os.tmpdir(),'ntk-macro-live-')),connections=new Set(),received=[],report={binary:path.resolve(process.env.NTK_APP_EXECUTABLE),assets:[],checks:[]};
let app,browser,page,peer,appLog='',failure;
function check(label){report.checks.push(label);console.log('PASS '+label);}
function makePeer(){
 let serial=0;
 return net.createServer(socket=>{
  const session=++serial;connections.add(socket);socket.on('close',()=>connections.delete(socket));socket.on('error',()=>{});socket.write('test-switch# ');let line='',negotiation=0,sub=false,subIAC=false;
  socket.on('data',data=>{for(const b of data){
   if(sub){if(subIAC&&b===240){sub=false;subIAC=false;}else subIAC=b===255;continue;}
   if(negotiation===1){if(b===250){sub=true;negotiation=0;}else if([251,252,253,254].includes(b))negotiation=2;else negotiation=0;continue;}
   if(negotiation===2){negotiation=0;continue;}if(b===255){negotiation=1;continue;}
   if(b===3){socket.write('\r\nInterrupted\r\ntest-switch# ');line='';continue;}
   if(b===13){received.push({session,command:line});socket.write(line+'\r\nRESULT '+line+'\r\ntest-switch# ');line='';}
   else if(b===127||b===8)line=line.slice(0,-1);else if(b!==10&&b!==0)line+=String.fromCharCode(b);
  }});
 });
}
async function startApp(){
 app=spawn(report.binary,['-no-browser','-port','0','-data-dir',dataDir],{stdio:['ignore','pipe','pipe']});app.stdout.on('data',data=>{appLog+=data;});app.stderr.on('data',data=>{appLog+=data;});let spawnError;app.on('error',error=>{spawnError=error;});
 for(let n=0;n<150;n++){
  if(spawnError)throw spawnError;if(app.exitCode!==null)throw new Error('App exited before serving HTTP:\n'+appLog);
  const base=appLog.match(/http:\/\/127\.0\.0\.1:\d+\//)?.[0];if(base){try{if((await fetch(base)).ok){report.app=appLog.split('\n').find(line=>line.startsWith('Network Toolkit'))||'';return base;}}catch{}}
  await delay(100);
 }
 throw new Error('App did not start HTTP within 15 seconds:\n'+appLog);
}
async function run(){
 const base=await startApp();
 // Gather every status before asserting, so a missing allowlist entry is an
 // explicit HTTP failure and cannot be masked by later browser actions.
 for(const [asset,type] of ASSETS){const response=await fetch(new URL(asset,base)),text=await response.text(),result={path:asset,status:response.status,contentType:response.headers.get('content-type')||'',bytes:Buffer.byteLength(text),expectedType:type};if(!response.ok)result.response=text.trim().slice(0,160);report.assets.push(result);console.log('HTTP '+JSON.stringify(result));}
 for(const asset of report.assets){assert.equal(asset.status,200,asset.path+' must be served by the shipped binary');assert(asset.contentType.startsWith(asset.expectedType),asset.path+' content type');assert(asset.bytes>100,asset.path+' body must contain the actual asset');}
 check('all three shipped macro assets return HTTP 200 with the correct types');
 browser=await loadPlaywright().chromium.launch({executablePath:process.env.CHROME_EXECUTABLE||undefined,headless:true,args:['--disable-dev-shm-usage','--no-sandbox']});page=await browser.newPage({viewport:{width:1450,height:1120}});page.setDefaultTimeout(15000);
 const pageErrors=[],assetFailures=[];page.on('pageerror',error=>pageErrors.push(error.message));page.on('response',response=>{if(response.url().includes('/assets/')&&response.status()>=400)assetFailures.push({url:response.url(),status:response.status()});});
 await page.goto(base,{waitUntil:'domcontentloaded'});await page.locator('[data-page="terminal"]').click();
 assert.deepEqual(await page.evaluate(()=>[typeof window.NetworkToolkitMacroModel,typeof window.NetworkToolkitTerminalMacros]),['object','object']);
 for(const selector of ['#terminalSidebarTabMacros','#terminalMacroComposer','#terminalMacroTargetMode','#terminalMacroCommand'])assert(await page.locator(selector).isVisible(),selector+' must be visible before any connection');
 assert.equal(await page.locator('#terminalMacrosToggle,#sessionSidebarToggle').count(),0,'duplicate sidebar toolbar buttons are absent');
 assert.equal(await page.evaluate(async()=> (await api('/terminals')).sessions.length),0);check('macro modules and controls load over HTTP and are visible before connections');
 await page.click('#terminalSidebarTabMacros');
 await page.click('#terminalMacroMore');
 assert.deepEqual(await page.locator('#terminalMacroMenu button').evaluateAll(els=>els.map(el=>el.textContent)),['Import macros','Export library']);
 assert(await page.locator('#terminalMacroMenu button').evaluateAll(els=>els.every(el=>{const r=el.getBoundingClientRect(),hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return hit===el||el.contains(hit);})), 'More actions must receive clicks, outside the sidebar clipping');
 await page.keyboard.press('Escape');assert(await page.locator('#terminalMacroMore').evaluate(el=>document.activeElement===el));
 await page.click('#terminalMacroMore');await page.click('#terminalMacroMenu [data-macro-action="import"]');assert(await page.locator('#terminalMacroImport').isVisible());await page.click('[data-macro-action="close"]');
 assert.equal(await page.locator('[data-macro-action="save-command"]').count(),1);assert.equal(await page.locator('[data-macro-action="save-paste"]').count(),1);check('More menu actions are visible and clickable through the normal app layout, with no duplicate input actions');
 await page.locator('#terminalMacroPanel [data-macro-action="new"]').first().click();await page.fill('[data-macro-edit="name"]','Live HTTP macro');await page.fill('[data-macro-edit="commands"]','show version\nshow uptime');await page.click('[data-macro-action="save-editor"]');await page.waitForFunction(async()=> (await api('/terminal-automation/library')).macros.some(m=>m.name==='Live HTTP macro'));
 const macroId=await page.evaluate(async()=> (await api('/terminal-automation/library')).macros.find(m=>m.name==='Live HTTP macro').id);const item=page.locator('.macro-library-item[data-macro-id="'+macroId+'"]');await item.locator('[data-macro-action="edit"]').click();await page.fill('[data-macro-edit="description"]','Created and edited through the shipped UI');await page.click('[data-macro-action="editor-steps"]');await page.click('[data-macro-action="add-step"]');await page.selectOption('[data-step-index="2"] [data-macro-step="kind"]','pause');await page.click('[data-macro-action="step-up"][data-index="2"]');await page.click('[data-macro-action="save-editor"]');
 await page.waitForFunction(async id=>{const m=(await api('/terminal-automation/library')).macros.find(m=>m.id===id);return m?.description==='Created and edited through the shipped UI'&&m.steps.map(s=>s.kind).join(',')==='command,pause,command';},macroId);check('create, save and advanced-step edit work through the shipped UI before connections');
 peer=makePeer();await new Promise(resolve=>peer.listen(0,'127.0.0.1',resolve));const port=peer.address().port;
 for(let count=1;count<=2;count++){
  if(count===2)await page.locator('#terminalCommandBar [data-terminal-action="new"]').click();
  await page.locator('#terminalStage [data-terminal-new-type="telnet"]').first().click();await page.fill('#con-host','127.0.0.1');await page.fill('#con-port',String(port));await page.click('#terminalConnectButton');await page.waitForSelector('#terminalConnectForm',{state:'detached'});await page.waitForFunction(async count=> (await api('/terminals')).sessions.filter(s=>s.state==='connected').length===count,count);
 }
 await page.selectOption('#terminalMacroTargetMode','all');await page.waitForFunction(()=>document.querySelector('#terminalMacroTargetCount').textContent==='2 terminals');await page.fill('#terminalMacroCommand','show clock');await page.locator('#terminalMacroComposer [data-macro-action="send"]').click();
 await page.waitForFunction(async()=> (await api('/terminal-automation/runs')).runs.some(r=>r.targets.length===2&&r.state==='completed'));assert.equal(received.filter(r=>r.command==='show clock').length,2);assert.equal(new Set(received.filter(r=>r.command==='show clock').map(r=>r.session)).size,2);check('shared composer broadcasts once to each of two real Telnet peers');
 await page.locator('#terminalMacroPanel [data-macro-action="record"]').click();await page.locator('.terminal-pane.active .xterm-screen').click();await page.keyboard.type('show recorded',{delay:5});await page.keyboard.press('Enter');
 for(let n=0;n<50&&!received.some(r=>r.command==='show recorded');n++)await delay(50);assert.equal(received.filter(r=>r.command==='show recorded').length,1);await page.click('[data-macro-action="record-stop"]');assert.equal(await page.inputValue('[data-macro-edit="commands"]'),'show recorded');await page.fill('[data-macro-edit="name"]','Recorded HTTP task');await page.click('[data-macro-action="save-editor"]');await page.waitForFunction(async()=> (await api('/terminal-automation/library')).macros.some(m=>m.name==='Recorded HTTP task'&&m.steps[0].text==='show recorded'));check('recording captures real keyboard input from the active terminal and saves it through HTTP');
 const targets=await page.evaluate(async()=> (await api('/terminal-automation/targets')).targets.map(t=>({id:t.id,generation:t.generation})));await item.locator('[data-macro-action="preview"]').first().click();assert.equal(await page.locator('.macro-preview .macro-target-name').count(),2);await page.click('[data-macro-action="run-preview"]');await page.waitForFunction(async()=> (await api('/terminal-automation/runs')).runs.some(r=>r.name==='Live HTTP macro'&&r.state==='paused'));
 const runElement=page.locator('.macro-run').filter({hasText:'Live HTTP macro'});await runElement.locator('[data-control="resume"]').click();await page.waitForFunction(async()=> (await api('/terminal-automation/runs')).runs.some(r=>r.name==='Live HTTP macro'&&r.state==='completed'));
 assert.equal(received.filter(r=>r.command==='show version').length,2);assert.equal(received.filter(r=>r.command==='show uptime').length,2);const actual=await page.evaluate(async()=> (await api('/terminal-automation/runs')).runs.find(r=>r.name==='Live HTTP macro').targets.map(t=>({id:t.id,generation:t.generation})));assert.deepEqual(actual.sort((a,b)=>a.id.localeCompare(b.id)),targets.sort((a,b)=>a.id.localeCompare(b.id)));check('saved macro runs, pauses and resumes on both original backend target generations');
 await runElement.locator('[data-macro-action="output"]').click();await page.waitForFunction(()=>document.querySelector('#terminalMacroOutput')?.value.includes('RESULT show uptime'));const output=await page.inputValue('#terminalMacroOutput');assert.equal((output.match(/RESULT show version/g)||[]).length,2);assert.equal((output.match(/RESULT show uptime/g)||[]).length,2);await page.click('[data-macro-action="output-select"]');assert(await page.evaluate(()=>{const field=document.querySelector('#terminalMacroOutput');return field.selectionStart===0&&field.selectionEnd===field.value.length;}));check('complete grouped backend output is displayed and selectable for both terminals');
 await page.click('[data-macro-action="close"]');
 const history=await page.evaluate(async()=>{
  const library=await api('/terminal-automation/library'),all=(await api('/terminal-automation/runs')).runs,finished=all.find(r=>r.name==='Live HTTP macro');
  const active=await api('/terminal-automation/runs','POST',{macro:{...library.macros[0],id:'history-active',name:'Keep this run active',steps:[{id:'pause',kind:'pause'}]},targets:(await api('/terminal-automation/targets')).targets.map(t=>({id:t.id,generation:t.generation})),values:{}});
  window.historyActiveId=active.run.id;return {clearedId:finished.id,macroCount:library.macros.length};
 });
 await page.waitForFunction(async()=> (await api('/terminal-automation/runs')).runs.find(r=>r.id===historyActiveId)?.state==='paused');await page.evaluate(()=>NetworkToolkitTerminalMacros.refresh());
 const activeRun=page.locator('[data-macro-run-id="'+(await page.evaluate(()=>historyActiveId))+'"]');await activeRun.waitFor();assert(await activeRun.locator('[data-macro-action="clear-run"]').isDisabled());
 await page.click('[data-macro-action="clear-run"][data-run="'+history.clearedId+'"]');await page.waitForFunction(async id=>!(await api('/terminal-automation/runs')).runs.some(r=>r.id===id),history.clearedId);
 assert.equal(await page.evaluate(async id=>{try{await api('/terminal-automation/runs/'+id+'/output?format=json');return 200;}catch(error){return error.status;}},history.clearedId),404);check('individual Clear removes a real backend run and its captured output');
 await page.click('[data-macro-action="clear-runs"]');await page.waitForFunction(async()=>{const runs=(await api('/terminal-automation/runs')).runs;return runs.length===1&&runs[0].id===historyActiveId;});await page.waitForFunction(()=>document.querySelectorAll('.macro-run').length===1);assert(await page.locator('[data-macro-action="clear-runs"]').isDisabled());assert(await activeRun.locator('[data-control="resume"]').isVisible());
 assert.equal(await page.evaluate(async()=> (await api('/terminal-automation/library')).macros.length),history.macroCount);assert.equal(await page.evaluate(async()=> (await api('/terminals')).sessions.filter(s=>s.state==='connected').length),2);check('Clear all removes finished history while preserving a paused run, both Telnet connections and saved macros');
 await activeRun.locator('[data-control="stop"]').click();await page.waitForFunction(()=>!document.querySelector('[data-macro-action="clear-run"][data-run="'+historyActiveId+'"]').disabled);await activeRun.locator('[data-macro-action="clear-run"]').click();await page.waitForFunction(async()=> (await api('/terminal-automation/runs')).runs.length===0);await page.waitForFunction(()=>document.querySelectorAll('.macro-run').length===0);check('stopped runs can be cleared individually and the run history becomes empty');
 assert.deepEqual(assetFailures,[],'No browser asset requests may fail');assert.deepEqual(pageErrors,[],'No browser JavaScript errors');check('the shipped page reports zero failed assets and zero JavaScript errors');report.passed=true;
}
run().catch(error=>{failure=error;report.passed=false;report.error=error.stack||String(error);console.error(report.error);process.exitCode=1;}).finally(async()=>{
 if(process.env.NTK_BROWSER_OUTPUT_DIR){fs.mkdirSync(process.env.NTK_BROWSER_OUTPUT_DIR,{recursive:true});fs.writeFileSync(path.join(process.env.NTK_BROWSER_OUTPUT_DIR,'terminal-macros-live-report.json'),JSON.stringify(report,null,2)+'\n');if(page)await page.screenshot({path:path.join(process.env.NTK_BROWSER_OUTPUT_DIR,'terminal-macros-live.png'),fullPage:true}).catch(()=>{});}
 if(browser)await browser.close();for(const socket of connections)socket.destroy();if(peer)await new Promise(resolve=>peer.close(resolve));if(app&&app.exitCode===null){app.kill('SIGTERM');await Promise.race([new Promise(resolve=>app.once('exit',resolve)),delay(2000)]);if(app.exitCode===null)app.kill('SIGKILL');}fs.rmSync(dataDir,{recursive:true,force:true});if(!failure)console.log('PASS shipped HTTP macro regression: '+report.checks.length+' checks.');
});
