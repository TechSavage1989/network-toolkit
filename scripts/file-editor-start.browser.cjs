'use strict';
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT;
const text='# START OF DOCUMENT\n'+Array.from({length:300},(_,i)=>'Line '+(i+2)+' '+ 'readable file contents '.repeat(12)).join('\n')+'\n# END OF DOCUMENT';
(async()=>{
 const browser=await chromium.launch({headless:true,args:['--no-sandbox']}),page=await browser.newPage({viewport:{width:1280,height:900}}),errors=[],writes=[];
 page.on('pageerror',error=>errors.push(error.message));page.on('dialog',dialog=>dialog.accept());
 await page.route('https://toolkit.local/**',async route=>{
  const request=route.request(),p=new URL(request.url()).pathname;
  if(p.startsWith('/api/')){
   const body=request.postData()?request.postDataJSON():null;let data={};
   if(p==='/api/status')data={version,os:'windows',arch:'amd64'};
   if(p==='/api/state'&&request.method()==='GET')data={appUpdatePreferences:{enabled:false}};
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminals')data={sessions:[]};
   if(p==='/api/remote-browser')data=request.method()==='GET'?{sessions:[{id:'ssh-fixture',host:'fixture.test',username:'operator',port:22,revision:1}]}:{content:text,encoding:'UTF-8',hash:'fixture-hash'};
   if(p==='/api/file-editor/local'){
    if(body.action==='write-text')writes.push(body);
    data={content:text,encoding:'UTF-8',hash:'fixture-hash'};
   }
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{
   const file=path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));
   if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','editor-start-fixture').replaceAll('__WINDOW_MODE__','main');
   return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.ico')?'image/x-icon':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 const input=()=>page.locator('[data-ed-text]');
 async function settled(){await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));}
 async function atStart(label){
  await settled();const state=await input().evaluate(el=>{const r=el.getBoundingClientRect(),style=getComputedStyle(el),firstLineY=r.top+parseFloat(style.paddingTop)+parseFloat(style.lineHeight)/2;return {scrollTop:el.scrollTop,scrollLeft:el.scrollLeft,start:el.selectionStart,end:el.selectionEnd,position:document.querySelector('[data-ed-position]').textContent,highlight:document.querySelector('[data-ed-highlight]').style.transform,gutter:document.querySelector('[data-ed-gutter]').style.transform,scrollHeight:el.scrollHeight,clientHeight:el.clientHeight,activeId:NetworkToolkitFileEditor.active?.id,startPending:NetworkToolkitFileEditor.active?.showStart,firstLineOnScreen:firstLineY>=0&&firstLineY<innerHeight&&r.right>0&&r.left<innerWidth};});
  assert.equal(state.scrollTop,0,label+': '+JSON.stringify(state));assert.equal(state.scrollLeft,0,label);assert.equal(state.start,0,label+': '+JSON.stringify(state));assert.equal(state.end,0,label+': '+JSON.stringify(state));assert.equal(state.position,'Ln 1, Col 1');assert.equal(state.highlight,'translate(0px, 0px)');assert.equal(state.gutter,'translateY(0px)');assert(state.firstLineOnScreen,label+': first line is outside the visible window');return state;
 }
 async function bottom(){await input().focus();await input().press('Control+End');await settled();assert(await input().evaluate(el=>el.scrollTop>0));}
 async function openLocal(file){await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),file);await atStart(file);assert.equal(await input().inputValue(),text);}
 async function close(){await page.evaluate(()=>{NetworkToolkitFileEditor.closeAll();navigate('overview');});}
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  await page.goto('https://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  assert.equal(await page.evaluate(()=>isSecureContext&&typeof crypto.subtle?.digest==='function'),true,'Desktop import fixture requires a secure browser origin');
  const layouts=[];
  for(const width of [1280,768,390])for(const theme of ['light','dark'])for(const mode of ['embedded','overlay']){
   await page.setViewportSize({width,height:900});await page.evaluate(({theme,mode})=>{setTheme(theme);navigate(mode==='embedded'?'editor':'overview');},{theme,mode});
   const file='C:\\fixture\\'+width+'-'+theme+'-'+mode+'.txt';
   if(mode==='embedded'){
    await page.fill('#ntEditorLocalPath',file);await page.locator('[data-ed-page="openPath"]').click();await page.waitForFunction(file=>NetworkToolkitFileEditor.active?.path===file&&!NetworkToolkitFileEditor.active.loading,file);await atStart(file);
   }else await openLocal(file);
   assert.equal(await input().inputValue(),text);assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.dirty),false);
   assert((await input().evaluate(el=>el.scrollHeight>el.clientHeight)),'Fixture must overflow the editor');
   if(output&&width===1280&&theme==='light'&&mode==='embedded')await page.screenshot({path:path.join(output,'file-editor-start-wide.png')});
   if(output&&width===390&&theme==='dark'&&mode==='overlay')await page.screenshot({path:path.join(output,'file-editor-start-compact.png')});
   await bottom();await page.locator('[data-ed="reload"]').click();await page.waitForFunction(()=>document.querySelector('[data-ed-text]').selectionStart===0);await atStart('reload');assert.equal(await input().inputValue(),text);
   await bottom();await openLocal(file+'-second');await bottom();await openLocal(file);
   // Ordinary Find and manual navigation still reach the end of the file.
   await page.locator('[data-ed="find"]').click();await page.fill('[data-ed-query]','END OF DOCUMENT');await settled();assert(await input().evaluate(el=>el.selectionStart>0&&el.scrollTop>0));await page.locator('[data-ed="hideFind"]').click();assert(await input().evaluate(el=>el.scrollTop>0));
   layouts.push({width,theme,mode});await close();
  }
  await page.setViewportSize({width:1280,height:900});await page.evaluate(()=>navigate('editor'));
  // Connected SSH uses the same load path through its actual selector/reader.
  await page.waitForFunction(()=>document.querySelector('#ntEditorRemoteSource option[value="terminal:ssh-fixture"]'));
  await page.selectOption('#ntEditorRemoteSource','terminal:ssh-fixture');await page.fill('#ntEditorRemotePath','/etc/fixture.conf');await page.locator('[data-ed-page="openRemotePath"]').click();await page.waitForFunction(()=>NetworkToolkitFileEditor.active?.path==='/etc/fixture.conf'&&!NetworkToolkitFileEditor.active.loading);await atStart('connected SSH');assert.equal(await input().inputValue(),text);
  await bottom();
  // Desktop imports enter through the populated create path, retaining CRLF/BOM.
  await page.evaluate(async text=>{await NetworkToolkitFileEditor.importFiles([new File([new Uint8Array([239,187,191]),text.replaceAll('\n','\r\n')],'desktop.txt',{type:'text/plain'})]);},text);
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.path),'desktop.txt');await atStart('desktop import');assert.equal(await input().inputValue(),text);assert.deepEqual(await page.evaluate(()=>({encoding:NetworkToolkitFileEditor.active.encoding,bom:NetworkToolkitFileEditor.active.bom,eol:NetworkToolkitFileEditor.active.eol,dirty:NetworkToolkitFileEditor.active.dirty})),{encoding:'UTF-8',bom:true,eol:'CRLF',dirty:false});
  await input().press('P');assert((await input().inputValue()).startsWith('P# START'));assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.dirty),true);
  await close();await page.evaluate(()=>navigate('editor'));
  // A slow read/reload must not move or replace another active document.
  await page.evaluate(()=>{globalThis.__editorPendingOpen=NetworkToolkitFileEditor.open({id:'slow',path:'slow.txt',read:()=>new Promise(resolve=>globalThis.__finishEditorRead=resolve),write:async()=>({})});});
  await openLocal('C:\\fixture\\other.txt');await bottom();const before=await input().evaluate(el=>({top:el.scrollTop,left:el.scrollLeft,start:el.selectionStart}));
  await page.evaluate(async text=>{__finishEditorRead({content:text});await __editorPendingOpen;},text);assert.deepEqual(await input().evaluate(el=>({top:el.scrollTop,left:el.scrollLeft,start:el.selectionStart})),before);
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.documents.get('slow').showStart),true);await page.locator('[data-ed-open="slow"]').click();await atStart('background file first display');
  await page.evaluate(()=>NetworkToolkitFileEditor.active.read=()=>new Promise(resolve=>globalThis.__finishEditorReload=resolve));await page.locator('[data-ed="reload"]').click();await openLocal('C:\\fixture\\other.txt');await bottom();const beforeReload=await input().evaluate(el=>({top:el.scrollTop,start:el.selectionStart}));
  await page.evaluate(text=>__finishEditorReload({content:text}),text);await settled();assert.deepEqual(await input().evaluate(el=>({top:el.scrollTop,start:el.selectionStart})),beforeReload);await page.locator('[data-ed-open="slow"]').click();await atStart('background reload first display');
  // Wrapping and the large-file plain-text path also start at line one.
  await close();await page.evaluate(()=>navigate('editor'));await page.evaluate(async text=>NetworkToolkitFileEditor.open({id:'large',path:'large.txt',create:true,initialContent:text.repeat(6),read:async()=>({content:''}),write:async()=>({})}),text);await atStart('large plain-text file');assert.equal(await page.locator('[data-ed-highlight]').isVisible(),false);
  await page.check('[data-ed-wrap]');await page.evaluate(async text=>NetworkToolkitFileEditor.open({id:'wrapped',path:'wrapped.txt',create:true,initialContent:text,read:async()=>({content:''}),write:async()=>({})}),text);await atStart('wrapped file');
  assert.equal(writes.length,0);assert.deepEqual(errors,[]);
  const result={passed:true,renderedCases:layouts.length,firstLineVisible:true,caretAtStart:true,bothScrollAxesReset:true,highlightAndGutterAligned:true,localAndConnectedSsh:true,desktopImport:true,bomAndLineEndingsRetained:true,reloadAndReopen:true,backgroundLoadsDoNotMoveActiveFile:true,findAndTyping:true,largeAndWrappedFiles:true,noFileWrites:true,errors};
  if(output)fs.writeFileSync(path.join(output,'file-editor-start-results.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
 }catch(error){if(output)await page.screenshot({path:path.join(output,'file-editor-start-failure.png')}).catch(()=>{});throw error;}
 finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
