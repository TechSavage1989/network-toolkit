'use strict';
// Driven by TestEditorSessionBrowserAnyFile (editor_any_file_browser_test.go):
// the shipped page of the standalone Text Editor against its production
// engine, with the kinds of file Windows hands to a default editor. The Go
// test generates the files and afterwards compares every one of them on
// disk, byte for byte, with what this script typed and saved.
//  1. A program (tool.exe) arrives as the start file: it is the active tab,
//     shown read-only, and nothing the page or the keyboard does changes it.
//  2. Plain text in an unknown type opens as plain text.
//  3. Text without an extension whose lines end in different ways keeps each
//     line's ending through an edit and a save.
//  4. Windows-1252 text is saved in Windows-1252; a character it cannot hold
//     stops the save until UTF-8 is chosen.
//  5. A folder and a missing file give a message and open nothing.
//  6. An empty file opens.
//  7. A UTF-16 file handed over by a second start becomes the active tab.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {NTK_EDITOR_STANDALONE_URL:url,NTK_EDITOR_STANDALONE_TOKEN:token,NTK_EDITOR_ANY_FILES:files}=process.env;
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const file=name=>path.join(files,name);
(async()=>{
 const browser=await chromium.launch({headless:true,...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 const errors=[];let page;
 const text=()=>page.locator('.nt-editor-code textarea[data-ed-text]').first();
 const message=()=>page.locator('[data-ed-message]').first().textContent();
 const settled=()=>page.waitForFunction(()=>{const d=NetworkToolkitFileEditor.active;return d&&!d.loading&&!d.saving&&!d.diskSession?.pending&&!document.querySelector('.nt-editor-code textarea').disabled;});
 const open=name=>page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file,false,true).then(()=>'',error=>error.message),file(name));
 const active=()=>page.evaluate(()=>{const d=NetworkToolkitFileEditor.active,q=s=>document.querySelector(s);
  return {path:d.path,disk:!!d.disk,dirty:!!d.dirty,encoding:d.encoding,bom:d.bom,eol:d.eol,units:d.disk?d.diskMeta.units:d.content.length,
   readOnly:document.querySelector('.nt-editor-code textarea[data-ed-text]').readOnly,
   eolChoices:[...q('[data-ed-eol]').options].map(o=>o.value),eolValue:q('[data-ed-eol]').value,eolDisabled:q('[data-ed-eol]').disabled,
   encodingChoices:[...q('[data-ed-encoding]').options].map(o=>o.value),encodingValue:q('[data-ed-encoding]').value,encodingDisabled:q('[data-ed-encoding]').disabled,
   bomDisabled:q('[data-ed-bom]').disabled,saveDisabled:q('[data-ed="save"]').disabled,saveAsDisabled:q('[data-ed="saveAs"]').disabled,reloadDisabled:q('[data-ed="reload"]').disabled,
   size:q('[data-ed-size]').textContent,selected:[...document.querySelectorAll('[data-ed-tabs] [data-ed-open]')].filter(tab=>tab.offsetParent&&tab.getAttribute('aria-selected')==='true').map(tab=>tab.querySelector('.nt-editor-tab-name').textContent)};});
 const count=()=>page.evaluate(()=>NetworkToolkitFileEditor.documents.size);
 const engine=(action,body={})=>page.evaluate(([action,body])=>NetworkToolkitEditorWindows.buffer(NetworkToolkitFileEditor.active.uid,action,body).then(result=>({result}),error=>({error:error.message})),[action,body]);
 const save=async()=>{await page.locator('[data-ed="save"]').first().click();await page.waitForFunction(()=>!NetworkToolkitFileEditor.active.saving);};
 const typeAtStart=async value=>{const before=(await active()).units;await text().focus();await page.keyboard.press('Control+Home');await text().pressSequentially(value,{delay:10});
  await page.waitForFunction(units=>{const d=NetworkToolkitFileEditor.active;return d.diskMeta.units===units&&!d.diskSession.pending;},before+value.length);};
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  const context=await browser.newContext({viewport:{width:1280,height:820}});page=await context.newPage();
  page.on('pageerror',error=>errors.push(error.message));
  await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},token);
  await page.goto(url);
  await page.waitForFunction(()=>window.NetworkToolkitEditorWindows?.bufferSupported()&&window.__networkToolkitStartupState===1);
  assert.equal(await page.evaluate(()=>EDITOR_STANDALONE),true);
  await page.waitForFunction(()=>NetworkToolkitFileEditor.session.phase==='ready');

  // 1. The program Explorer started the editor with.
  await page.waitForFunction(()=>NetworkToolkitFileEditor.documents.size===1&&!NetworkToolkitFileEditor.active?.loading);await settled();
  let shown=await active();
  assert.equal(shown.path,file('tool.exe'));assert.deepEqual(shown.selected,['tool.exe'],'the file from Explorer is the active tab');
  assert.equal(shown.encoding,'Binary');assert.equal(shown.disk,true);assert.equal(shown.dirty,false);
  assert.equal(shown.readOnly,true,'the text area of a file that is not text is read-only');
  assert.deepEqual([shown.saveDisabled,shown.saveAsDisabled,shown.reloadDisabled],[true,true,false],'Save and Save As are off; Reload stays');
  assert.deepEqual([shown.eolDisabled,shown.encodingDisabled,shown.bomDisabled,shown.encodingValue],[true,true,true,'Binary']);
  assert.match(shown.size,/Read-only: not a text file/);
  assert.match(await message(),/^tool\.exe is not a text file\. It is shown read-only and cannot be changed or saved in the Text Editor\.$/);
  assert.match(await page.locator('.toast').last().textContent(),/tool\.exe is not a text file/,'the user is told when the file opens');
  const head=await engine('window',{start:0,limit:32});
  assert.match(head.result.text,/^MZ/);assert(!head.result.text.includes('\0'),'no NUL reaches the page');
  if(output)await page.screenshot({path:path.join(output,'editor-any-file-binary.png')});
  // The keyboard, a paste, undo and the save shortcut change nothing.
  await text().focus();await page.keyboard.press('Control+Home');await page.keyboard.type('typed');await page.keyboard.press('Backspace');await page.keyboard.press('Delete');await page.keyboard.press('Enter');
  await page.evaluate(()=>{const data=new DataTransfer();data.setData('text/plain','pasted');document.querySelector('.nt-editor-code textarea[data-ed-text]').dispatchEvent(new ClipboardEvent('paste',{clipboardData:data,bubbles:true,cancelable:true}));});
  await page.keyboard.press('Control+z');await page.keyboard.press('Control+s');await page.waitForTimeout(300);
  let after=await active();
  assert.deepEqual([after.units,after.dirty],[shown.units,false],'nothing typed or pasted reached the document');
  assert.equal((await engine('status')).result.metadata.revision,head.result.revision,'the engine saw no edit');
  // The page asking the engine directly is refused as well.
  for(const [action,body] of [['edit',{revision:head.result.revision,from:0,to:2,text:'ZM'}],['save',{revision:head.result.revision,format:{encoding:'UTF-8',bom:false,eol:'LF'}}],['save',{revision:head.result.revision,path:file('tool-copy.exe'),overwrite:true,format:{encoding:'UTF-8',bom:false,eol:'LF'}}]])
   assert.match((await engine(action,body)).error||'',/not a text file/,action+' of a file that is not text is refused by the engine');
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.session.save().then(()=>api('/editor-session')).then(kept=>kept.tabs.map(tab=>[tab.kind,tab.dirty,!!tab.backup]).join())),'local,false,false','the program is a clean tab of the session, with no kept copy');

  // 2. Plain text in a type nobody knows.
  assert.equal(await open('notes.zz9'),'');await settled();
  shown=await active();
  assert.deepEqual([shown.path,shown.encoding,shown.eol,shown.readOnly,shown.saveDisabled,shown.encodingDisabled],[file('notes.zz9'),'UTF-8','LF',false,false,false]);
  assert.deepEqual([shown.eolChoices,shown.encodingChoices],[['LF','CRLF','CR'],['UTF-8','UTF-16LE','UTF-16BE']],'an ordinary file offers the usual choices');
  assert.equal(await text().inputValue(),'plain words in an unknown type\n');
  assert.equal(await page.evaluate(()=>NTKEditorLanguages.detect(NetworkToolkitFileEditor.active.path,NetworkToolkitFileEditor.active.content)),'text','an unknown type is plain text');

  // 3. No extension, lines that end in different ways.
  assert.equal(await open('running-config'),'');await settled();
  shown=await active();
  assert.deepEqual([shown.disk,shown.encoding,shown.eol,shown.eolValue,shown.dirty],[true,'UTF-8','Mixed','Mixed',false],'the engine keeps a file with differing line endings');
  assert.deepEqual(shown.eolChoices,['Mixed','LF','CRLF','CR']);assert.match(shown.size,/Mixed line endings/);
  assert.match(await message(),/^Opened running-config\. Its lines end in different ways\. Each line keeps its own ending and new lines use CRLF, unless you choose one under Line endings\.$/);
  await typeAtStart('X');await save();
  await page.waitForFunction(()=>!NetworkToolkitFileEditor.active.dirty);
  assert.equal(fs.readFileSync(file('running-config'),'latin1'),'Xinterface a\r\n shutdown\n!\r\ninterface b\r\n','only the typed character is new; every line kept its ending');

  // 4. Windows-1252.
  assert.equal(await open('legacy.dat'),'');await settled();
  shown=await active();
  assert.deepEqual([shown.disk,shown.encoding,shown.bom,shown.eol,shown.bomDisabled],[true,'Windows-1252',false,'CRLF',true]);
  assert.deepEqual(shown.encodingChoices,['UTF-8','UTF-16LE','UTF-16BE','Windows-1252']);
  assert.match(await message(),/read as Windows-1252/);
  assert.equal((await engine('window',{start:0,limit:64})).result.text,'name=café\nprice=5€\n');
  await typeAtStart('é');await save();await page.waitForFunction(()=>!NetworkToolkitFileEditor.active.dirty);
  assert.equal(fs.readFileSync(file('legacy.dat'),'latin1'),'\xe9name=caf\xe9\r\nprice=5\x80\r\n','saved in Windows-1252, one byte for each character');
  fs.copyFileSync(file('legacy.dat'),file('legacy-saved.dat'));
  await typeAtStart('→');await save();
  assert.match(await message(),/cannot be saved as Windows-1252 text; choose UTF-8 under Encoding/);
  assert.equal((await active()).dirty,true,'the text stays in the tab');
  assert.deepEqual(fs.readFileSync(file('legacy.dat')),fs.readFileSync(file('legacy-saved.dat')),'the refused save left the file as it was');
  await page.locator('[data-ed-encoding]').first().selectOption('UTF-8');await save();await page.waitForFunction(()=>!NetworkToolkitFileEditor.active.dirty);
  assert.equal(fs.readFileSync(file('legacy.dat'),'utf8'),'→éname=café\r\nprice=5€\r\n');

  // 5. Nothing opens half-way.
  const before=await count(),activeBefore=(await active()).path;
  assert.equal(await open('a folder.txt'),'This is a folder, not a file.');
  assert.equal(await open('missing.cfg'),'The file does not exist.');
  assert.deepEqual([await count(),(await active()).path],[before,activeBefore],'a failed open adds no tab and keeps the active one');
  assert.match(await message(),/^Cannot open missing\.cfg: The file does not exist\.$/);

  // 6. An empty file.
  assert.equal(await open('empty'),'');await settled();
  shown=await active();assert.deepEqual([shown.path,shown.units,shown.readOnly,shown.dirty],[file('empty'),0,false,false]);

  // 7. A second start hands over a UTF-16 file of an unknown type.
  assert.equal(await page.evaluate(file=>fetch('/api/editor-fixture/second-start',{method:'POST',headers:{'X-Session-Token':window.__networkToolkitNativeSession,'Content-Type':'application/json'},body:JSON.stringify({open:file})}).then(r=>r.status),file('wide.unknown')),200);
  await page.waitForFunction(file=>NetworkToolkitFileEditor.active?.path===file&&!NetworkToolkitFileEditor.active.loading,file('wide.unknown'));await settled();
  shown=await active();
  assert.deepEqual([shown.selected,shown.encoding,shown.bom,shown.eol],[['wide.unknown'],'UTF-16LE',true,'CRLF'],'the file handed over is the active tab, in its own encoding');
  assert.equal(await text().inputValue(),'wide text\nsecond\n');
  // The program is still there, still clean and still read-only.
  await page.evaluate(id=>NetworkToolkitFileEditor.activateDocument(id),'local:'+file('tool.exe'));await settled();
  after=await active();assert.deepEqual([after.encoding,after.readOnly,after.dirty,after.saveDisabled,after.encodingDisabled],['Binary',true,false,true,true]);
  await page.evaluate(id=>NetworkToolkitFileEditor.activateDocument(id),'local:'+file('notes.zz9'));await settled();
  after=await active();assert.deepEqual([after.readOnly,after.saveDisabled,after.encodingDisabled,after.encodingChoices],[false,false,false,['UTF-8','UTF-16LE','UTF-16BE']],'the next text tab is editable again');
  assert.deepEqual(errors,[]);
  console.log(JSON.stringify({passed:true,binaryReadOnly:true,unknownTypePlainText:true,mixedLineEndingsKept:true,legacyEncodingKept:true,failedOpenAddsNothing:true,emptyFile:true,handedOverFileActive:true}));
 }catch(error){
  if(output&&page)await page.screenshot({path:path.join(output,'editor-any-file-failure.png')}).catch(()=>{});
  console.error(error);console.error(errors);process.exitCode=1;
 }finally{await browser.close();}
})();
