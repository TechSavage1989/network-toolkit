'use strict';
// Actual app assets and isolated API responses. Selection uses real mouse and
// keyboard input; no device, credential, native launch or user file is touched.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),origin='https://app-interaction.toolkit.test',version=require('../package.json').version;
const out=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const report={passed:false,checks:[],errors:[],dialogs:[],assetFailures:[],screenshots:[],scope:'Production browser assets with synthetic API data; native host settings are tested separately.'};
let browser,page;
const pass=name=>{report.checks.push(name);console.log('PASS '+name);};
const settle=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));

async function make(mode){
 const context=await browser.newContext({viewport:{width:1280,height:900},permissions:['clipboard-read','clipboard-write'],serviceWorkers:'block'});
 page=await context.newPage();page.setDefaultTimeout(15000);
 page.on('pageerror',error=>report.errors.push(mode+': '+error.message));
 page.on('dialog',dialog=>{report.dialogs.push({mode,type:dialog.type(),message:dialog.message()});void dialog.dismiss();});
 page.on('response',response=>{if(response.url().includes('/assets/')&&response.status()>=400)report.assetFailures.push(response.url());});
 let saved={appUpdatePreferences:{enabled:false},terminalInteraction:{copyOnSelect:false,copyOnClick:false,rightClick:'menu'},navigationCollapsed:false,welcomeDismissed:true,firstLaunchWelcome:{dismissed:true}};
 const session={id:'interaction-fixture',label:'Synthetic terminal',type:'ssh',host:'switch.test',port:22,state:'connected',cols:100,rows:28};
 const view={id:'interaction-view',sessionId:session.id,state:'detached',revision:0};
 const editorView={id:'76767676767676767676767676767676',ownerInstanceId:'75757575757575757575757575757575'};
 await context.route('**/*',async route=>{
  if(await handleEditorActivityMetadata(route,origin,report))return;
  const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method();
  assert.equal(url.origin,origin,'No external or device request');
  if(p.startsWith('/api/')){
   const body=request.postData()?request.postDataJSON():null;
   if(method!=='GET'){
    if(p==='/api/state'&&method==='PUT')assert(body&&typeof body==='object'&&!Array.isArray(body));
    else if(p==='/api/remote-browser'&&method==='POST')assert.deepEqual(body,{protocol:'ssh',action:'probe',sessionId:session.id},'Only the synthetic SFTP availability probe is allowed');
    else if(p==='/api/terminals/'+session.id+'/resize'&&method==='POST'){
     assert.deepEqual(Object.keys(body).sort(),['cols','rows']);assert(Number.isInteger(body.cols)&&body.cols>=2&&body.cols<=500);assert(Number.isInteger(body.rows)&&body.rows>=2&&body.rows<=300);Object.assign(session,body);
    }else if(mode==='detached'&&p==='/api/terminal-views/current/ready'&&method==='POST')assert.deepEqual(body,{});
    else if(mode==='detached'&&p==='/api/terminal-views/current/checkpoint'&&method==='POST'){
     assert.deepEqual(Object.keys(body),['snapshot']);const snapshot=body.snapshot;
     assert(snapshot&&typeof snapshot.data==='string'&&Buffer.byteLength(snapshot.data)<1024*1024);
     assert(Object.keys(snapshot).every(key=>['data','emulator','cols','rows','viewportY','promptName','after','config','outputBlocks'].includes(key)));
     assert.equal(snapshot.config?.host,session.host);assert.equal(snapshot.config?.type,'ssh');
    }else assert.fail('Unexpected mutation: '+method+' '+p);
   }
   let data={};
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',windowMode:mode,terminalWindowAvailable:true,editorWindowAvailable:mode==='editor'};
   if(p==='/api/state'){if(method==='PUT')saved=request.postDataJSON();data=saved;}
   if(p==='/api/system')data={hostname:'Synthetic workstation',interfaces:[],gateways:[],dnsServers:[]};
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminals')data={sessions:[session]};
   if(p==='/api/terminal-views')data={available:true,views:mode==='detached'?[view]:[],owners:{[session.id]:{viewId:mode==='detached'?view.id:'',revision:0,ready:true}}};
   if(p==='/api/terminal-views/current')data={view,session,snapshot:{fresh:true,cols:100,rows:28,after:0,config:{type:'ssh',host:'switch.test',port:22}}};
   if(p==='/api/terminal-views/current/ready')data={view};
   if(p==='/api/remote-browser')data=method==='POST'?{supported:false}:{sessions:[]};
   if(p==='/api/editor-views'||p==='/api/editor-views/current')data={available:mode==='editor',ownerInstanceId:editorView.ownerInstanceId,self:mode==='editor'?editorView.id:'',views:[],incoming:[],outgoing:[],owned:{},recovery:[],dragRequests:[]};
   if(p==='/api/credentials')data={entries:[],vault:{available:false,state:'unavailable'}};
   if(p==='/api/credential-vault')data={available:false,state:'unavailable'};
   if(p==='/api/local-tools/ip')data={adapters:[]};
   if(p==='/api/terminal-automation/targets')data={targets:[{...session,generation:1}]};
   if(p==='/api/terminal-automation/library')data={revision:'fixture',macros:[],bindings:[]};
   if(p==='/api/terminal-automation/runs')data={runs:[]};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,80));data={events:[],sessions:[session],after:0,next:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{
   const file=path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));
   if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','interaction-fixture').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__',JSON.stringify(mode==='detached'?view:null)).replaceAll('__EDITOR_VIEW__',JSON.stringify(mode==='editor'?editorView:null));
   return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto(origin+'/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
 await page.evaluate(()=>{
  window.interactionContextEvents=[];
  window.addEventListener('contextmenu',event=>queueMicrotask(()=>interactionContextEvents.push({prevented:event.defaultPrevented,target:event.target.id||event.target.tagName})),true);
 });
 return context;
}

async function dragText(selector){
 const element=page.locator(selector);await element.scrollIntoViewIfNeeded();
 const box=await element.evaluate(element=>{
  const walker=document.createTreeWalker(element,NodeFilter.SHOW_TEXT);let text;
  while(walker.nextNode())if(walker.currentNode.textContent.trim()){text=walker.currentNode;break;}
  if(!text)throw Error('No text in '+element.outerHTML);
  const range=document.createRange();range.selectNodeContents(text);const r=range.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height};
 });
 await page.evaluate(()=>{document.activeElement?.blur?.();getSelection().removeAllRanges();});
 await page.mouse.move(box.x+1,box.y+box.height/2);await page.mouse.down();await page.mouse.move(box.x+box.width-1,box.y+box.height/2,{steps:12});await page.mouse.up();
 return page.evaluate(()=>getSelection().toString());
}

async function dragField(selector){
 const field=page.locator(selector);await field.scrollIntoViewIfNeeded();const b=await field.boundingBox();
 await page.mouse.move(b.x+10,b.y+15);await page.mouse.down();await page.mouse.move(b.x+110,b.y+15,{steps:12});await page.mouse.up();
 return field.evaluate(element=>element.value.slice(element.selectionStart,element.selectionEnd));
}

async function checkShared(mode){
 await page.evaluate(()=>{
  openModal('Interaction check','<h2 id="fixtureHeading">Ordinary interface heading</h2><div data-selectable-text><button type="button" id="fixtureControl"><span class="value">Result action</span></button><label id="fixtureLabel">Input label <input id="fixtureInput" value="EDITABLE_FIELD_VALUE" style="font:14px monospace;padding:8px;width:300px"></label><details><summary id="fixtureSummary"><code>Result disclosure</code></summary></details><table><thead><tr><th id="fixtureHeader">Column heading</th></tr></thead><tbody><tr><td id="fixtureCell">198.51.100.45</td></tr></tbody></table></div><div class="data-item"><div class="key">Recorded value</div><div class="val" id="fixtureValue" data-copy-label="Fixture">COPYABLE_RESULT_VALUE</div></div><pre id="fixtureOutput">SELECTABLE_OUTPUT_TEXT</pre><textarea id="fixtureReadonly" readonly style="font:14px monospace;padding:8px;width:300px;height:50px">READONLY_OUTPUT_VALUE</textarea><div id="fixtureEditable" contenteditable="true">Editable document text</div>','<button class="btn" data-action="close-modal">Done</button>');
  NetworkToolkitValueCopy.prepare(document.querySelector('#modalRoot'));
 });
 await settle();
 for(const selector of ['#fixtureHeading','#fixtureControl','#fixtureLabel','#fixtureSummary','#fixtureHeader'])assert.equal(await dragText(selector),'',mode+' chrome selected: '+selector);
 for(const selector of ['#fixtureCell','#fixtureOutput','#fixtureValue'])assert((await dragText(selector)).length>3,mode+' data selection unavailable: '+selector);
 assert((await dragField('#fixtureInput')).length>3,mode+' editable field mouse selection');
 assert((await dragField('#fixtureReadonly')).length>3,mode+' readonly output mouse selection');
 await page.locator('#fixtureInput').focus();await page.keyboard.press('Control+a');await page.keyboard.type('edited by keyboard');assert.equal(await page.locator('#fixtureInput').inputValue(),'edited by keyboard');
 await page.locator('#fixtureEditable').focus();await page.keyboard.press('Control+a');await page.keyboard.type('Edited content');assert.equal(await page.locator('#fixtureEditable').innerText(),'Edited content');
 await page.getByRole('button',{name:'Copy Fixture value',exact:true}).click();assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),'COPYABLE_RESULT_VALUE');
 for(const selector of ['#fixtureHeading','#fixtureInput','#fixtureReadonly','#fixtureOutput']){
  const before=await page.evaluate(()=>interactionContextEvents.length);await page.locator(selector).click({button:'right'});
  await page.waitForFunction(before=>interactionContextEvents.length>before,before);
  assert.equal(await page.evaluate(()=>interactionContextEvents.at(-1).prevented),true,mode+' native context menu: '+selector);
 }
 if(out){const name='app-interaction-'+mode+'.png';await page.screenshot({path:path.join(out,name)});report.screenshots.push(name);}
 await page.evaluate(()=>closeModal());
 pass(mode+': chrome rejects mouse selection; fields, output and explicit Copy retain selection/editing; native context menu is cancelled');
}

async function checkEditors(){
 await page.evaluate(()=>navigate('editor'));await page.locator('[data-ed-text]').waitFor();
 const editor=page.locator('[data-ed-text]');await editor.fill('EDITOR_SELECTION_VALUE');await editor.focus();await page.keyboard.press('Control+a');
 assert.equal(await editor.evaluate(el=>el.value.slice(el.selectionStart,el.selectionEnd)),'EDITOR_SELECTION_VALUE');
 await editor.click({button:'right'});await page.locator('#editorContextMenu').waitFor();await page.keyboard.press('Escape');
 await editor.focus();await page.keyboard.press('Shift+F10');await page.locator('#editorContextMenu').waitFor();await page.keyboard.press('Escape');
 assert.equal(await editor.inputValue(),'EDITOR_SELECTION_VALUE');
 await page.evaluate(()=>{
  openModal('Editor selection fixtures','<div id="fixtureCM"></div><div class="nt-editor-code" id="fixtureDisk" style="height:90px;min-height:90px"><div class="nt-editor-gutter"><div id="fixtureGutter"></div></div><div class="nt-editor-surface"><textarea id="fixtureDiskText" style="font:14px monospace;padding:8px" aria-label="Synthetic disk text"></textarea></div></div>','<button class="btn" data-action="close-modal">Done</button>');
  const C=NetworkToolkitCodeMirror;
  window.interactionCM=new C.EditorView({state:C.EditorState.create({doc:'CODEMIRROR_SELECTION_VALUE'}),parent:document.getElementById('fixtureCM')});
  const text='DISK_SELECTION_VALUE\nSecond synthetic line';
  const session=window.interactionDiskSession=NTKEditorDiskSession.create({request:async(action,p)=>{
   if(action!=='window')throw Error('Unexpected synthetic disk edit: '+action);
   const start=p.start||0;return {revision:1,units:text.length,bytes:text.length,lines:2,start,line:0,column:start,lineUnits:20,text:text.slice(start,start+p.limit)};
  }});
  session.initialize({revision:1,units:text.length,bytes:text.length,lines:2});
  window.interactionDisk=NTKEditorDiskView.create({textarea:document.getElementById('fixtureDiskText'),gutter:document.getElementById('fixtureGutter'),container:document.getElementById('fixtureDisk'),session,readOnly:true});
  void interactionDisk.load({start:0});
 });
 await page.waitForFunction(()=>document.getElementById('fixtureDiskText').value.startsWith('DISK_SELECTION_VALUE'));
 assert((await dragText('#fixtureCM .cm-content')).length>3,'CodeMirror native mouse selection');
 assert((await dragField('#fixtureDiskText')).length>3,'Disk viewport mouse selection');
 await page.locator('#fixtureCM .cm-content').focus();await page.keyboard.press('Control+a');await page.keyboard.type('CM_EDITED');assert.equal(await page.evaluate(()=>interactionCM.state.doc.toString()),'CM_EDITED');
 await page.evaluate(()=>{interactionCM.destroy();interactionDisk.dispose();interactionDiskSession.close();closeModal();});
 pass('Textarea editor, CodeMirror and disk viewport retain selection/editing; app editor context menu opens with mouse and Shift+F10');
}

async function checkTerminal(mode){
 if(mode==='main')await page.evaluate(()=>navigate('terminal'));
 await page.waitForFunction(()=>!!currentTerminal()?.xterm&&terminalEntryAcceptsInput(currentTerminal()));
 await page.evaluate(()=>{const t=currentTerminal().xterm;t.reset();window.interactionTerminalWritten=false;t.write('TERMINAL_SELECTION_VALUE\r\nswitch# ',()=>{window.interactionTerminalWritten=true;});});
 await page.waitForFunction(()=>interactionTerminalWritten);await settle();
 const screen=page.locator('.xterm:visible .xterm-screen').first(),b=await screen.boundingBox(),size=await page.evaluate(()=>({cols:currentTerminal().xterm.cols,rows:currentTerminal().xterm.rows}));
 const w=b.width/size.cols,h=b.height/size.rows;
 await page.mouse.move(b.x+1,b.y+h/2);await page.mouse.down();await page.mouse.move(b.x+w*10,b.y+h/2,{steps:12});await page.mouse.up();
 const selected=await page.evaluate(()=>currentTerminal().xterm.getSelection());
 assert(selected.startsWith('TERMINAL'),'xterm mouse selection remains app-owned');
 await page.keyboard.press('Control+Shift+C');await page.waitForFunction(()=>!currentTerminal().xterm.hasSelection());
 assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),selected,'Terminal copy retains the exact selected text and sends no input');
 await screen.click({button:'right',position:{x:30,y:30}});await page.locator('#terminalContextMenu').waitFor();await page.keyboard.press('Escape');
 await page.evaluate(()=>currentTerminal().xterm.focus());await page.keyboard.press('Shift+F10');await page.locator('#terminalContextMenu').waitFor();await page.keyboard.press('Escape');
 pass(mode+': actual xterm drag selection, exact clipboard copy and app context menus work with mouse and Shift+F10');
}

(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined});
 for(const mode of ['main','editor-standalone','editor','terminal','detached']){
  const context=await make(mode);await checkShared(mode);
  if(mode==='main'){
   report.checks.push(...await require('./app-dialogs.browser-cases.cjs')(page));
   await checkEditors();
  }
  if(!mode.startsWith('editor'))await checkTerminal(mode);
  await context.close();
 }
 assert.deepEqual(report.dialogs,[],'No browser alert, prompt or confirmation is presented');assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetFailures,[]);report.passed=true;
})().catch(async error=>{report.failure=error.stack||String(error);console.error(report.failure);if(page&&out)await page.screenshot({path:path.join(out,'app-interaction-failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'app-interaction-results.json'),JSON.stringify(report,null,2)+'\n');await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,errors:report.errors,dialogs:report.dialogs}));});
