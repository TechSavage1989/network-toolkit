'use strict';
// Driven by TestEditorSessionBrowserTextTools (editor_text_tools_browser_test.go):
// the shipped page of the standalone Text Editor against its production
// engine, through the Tools menu (owner request v1.21.53).
//  1. The Tools button opens the tool groups (Lines, Encode / decode,
//     Format); each item shows a description, the same as its tooltip, in at
//     most two lines, and its shortcut.
//  2. Split at text on page-held text: & becomes a line break; Undo.
//  3. The Find bar's Extended mode: Replace all & with \n; Undo. Regular
//     expression mode: $2=$1. Both on page-held and disk-backed text.
//  4. Base64 decode of a whole 1.7 MB single-line disk-backed document with
//     nothing selected (Ctrl+Shift+B); Undo.
//  5. The editor guide's recipe for a one-line encoded settings file, on a
//     synthetic 1.5 MB file: Base64 decode stops at the "&&" after the Base64
//     and changes nothing; its bar's one button, Decode the part before it,
//     decodes up to there; Split at text &; URL decode; Undo of each step. The
//     same on page-held text with a selection.
// The Go test then checks the files on disk. Screenshots (light and dark, at
// device scale 1) go to TOOLKIT_TEST_OUTPUT when it is set.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {NTK_EDITOR_STANDALONE_URL:url,NTK_EDITOR_STANDALONE_TOKEN:token,NTK_EDITOR_TOOL_FILES:files}=process.env;
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const file=name=>path.join(files,name);
(async()=>{
 const browser=await chromium.launch({headless:true,...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 const errors=[];let page;
 const message=()=>page.locator('[data-ed-message]').first().textContent();
 const settled=()=>page.waitForFunction(()=>{const d=NetworkToolkitFileEditor.active;return d&&!d.loading&&!d.saving&&!d.diskEditing&&!d.diskSession?.pending&&!document.querySelector('[data-ed-tool-bar="progress"]');});
 const open=async name=>{assert.equal(await page.evaluate(path=>NetworkToolkitFileEditor.openLocalPath(path,false,true).then(()=>'',error=>error.message),file(name)),'');await settled();};
 const active=()=>page.evaluate(()=>{const d=NetworkToolkitFileEditor.active;return {name:d.path.split(/[\\/]/).pop(),disk:!!d.disk,dirty:!!d.dirty,units:d.disk?d.diskMeta.units:d.content.length,content:d.disk?'':d.content,revision:d.disk?d.diskMeta.revision:d.editRevision,untitled:!!d.untitled};});
 const engine=(action,body={})=>page.evaluate(([action,body])=>NetworkToolkitEditorWindows.buffer(NetworkToolkitFileEditor.active.uid,action,body),[action,body]);
 const diskText=async units=>{let text='';for(let start=0;start<units;){const w=await engine('window',{start,limit:131072});text+=w.text;start+=w.text.length;if(!w.text.length)break;}return text;};
 const text=()=>page.locator('.nt-editor-code textarea[data-ed-text]').first();
 const shoot=async name=>{if(output)await page.screenshot({path:path.join(output,name)});};
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  const context=await browser.newContext({viewport:{width:1280,height:820},deviceScaleFactor:1});page=await context.newPage();
  page.on('pageerror',error=>errors.push(error.message));
  await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},token);
  await page.goto(url);
  await page.waitForFunction(()=>window.NetworkToolkitEditorWindows?.bufferSupported()&&window.__networkToolkitStartupState===1);
  assert.equal(await page.evaluate(()=>EDITOR_STANDALONE),true);
  await page.waitForFunction(()=>NetworkToolkitFileEditor.session.phase==='ready');

  // 1. The Tools menu.
  await open('settings.txt');
  assert.equal((await active()).disk,false,'a small file is page-held');
  const tools=page.locator('.nt-editor-toolbar [data-ed="tools"]');
  assert.equal(await tools.textContent(),'Tools');
  const more=await page.locator('#ntEditorMoreActions [data-ed]').evaluateAll(list=>list.map(el=>el.dataset.ed));
  for(const gone of ['comment','duplicate','trim','upper','lower','sortLines','format','base64Decode'])assert(!more.includes(gone),gone+' moved from More to Tools');
  await tools.click();
  const menu=page.locator('#editorContextMenu.nt-editor-tools-menu');
  await menu.waitFor({state:'visible'});
  assert.equal(await tools.getAttribute('aria-expanded'),'true');
  const groups=await menu.locator('.nt-editor-menu-root [data-editor-submenu]').evaluateAll(list=>list.map(el=>[el.querySelector('.nt-editor-menu-label').textContent,el.title,el.querySelector('small').textContent]));
  assert.deepEqual(groups.map(g=>g[0]),['Lines','Encode / decode','Format']);
  for(const [label,title,shown] of groups)assert(title&&title===shown,label+' shows its description, also as its tooltip');
  // No description wraps beyond two lines, in the groups or their items.
  const lineCount=()=>menu.locator('small').evaluateAll(list=>list.filter(el=>el.offsetParent).map(el=>[el.closest('[data-editor-menu-action],[data-editor-submenu]')?.querySelector('.nt-editor-menu-label')?.textContent,Math.round(el.getBoundingClientRect().height/parseFloat(getComputedStyle(el).lineHeight))]));
  for(const group of ['tools-encode','tools-format','tools-lines']){
   await menu.locator('[data-editor-submenu="'+group+'"]').click();
   await menu.locator('[data-editor-submenu-panel="'+group+'"]').waitFor({state:'visible'});
   for(const [label,count] of await lineCount())assert(count>=1&&count<=2,label+' takes '+count+' lines in the menu');
   await shoot('editor-'+group+'.png');
  }
  const lines=menu.locator('[data-editor-submenu-panel="tools-lines"]');
  await lines.waitFor({state:'visible'});
  const items=await lines.locator('[data-editor-menu-action]').evaluateAll(list=>list.map(el=>({action:el.dataset.editorMenuAction,title:el.title,shown:el.querySelector('small')?.textContent,kbd:el.querySelector('kbd')?.textContent||''})));
  assert.deepEqual(items.map(i=>i.action),['comment','duplicate','split','join','sort-asc','sort-desc','dedupe','remove-empty','trim-trailing','trim-both','reverse']);
  for(const item of items)assert(item.title&&item.title===item.shown,item.action+' tooltip and description');
  assert.equal(items.find(i=>i.action==='join').kbd,'Ctrl+J');assert.equal(items.find(i=>i.action==='comment').kbd,'Ctrl+/');assert.equal(items.find(i=>i.action==='duplicate').kbd,'Ctrl+D');
  await shoot('editor-tools-menu-dark.png');
  await page.evaluate(()=>setTheme('light'));
  await page.waitForTimeout(150);
  await shoot('editor-tools-menu-light.png');
  await page.evaluate(()=>setTheme('dark'));

  // 2. Split at text on page-held text, and Undo.
  await lines.locator('[data-editor-menu-action="split"]').click();
  const dialog=page.locator('.nt-editor-dialog');
  await dialog.waitFor({state:'visible'});
  assert.equal(await dialog.locator('[data-ed-dialog-input]').inputValue(),'&');
  assert.equal(await dialog.locator('[data-ed-dialog-check]').isChecked(),false);
  await dialog.locator('[data-ed-dialog-action="accept"]').click();
  await page.waitForFunction(()=>NetworkToolkitFileEditor.active.content==='a=1\nb=2\nc=3');
  assert.match(await message(),/^Split the whole document at 2 place\(s\)\./);
  assert.equal(await page.evaluate(()=>document.querySelector('#editorContextMenu')),null,'the menu closed');
  await text().focus();await page.keyboard.press('Control+z');
  await page.waitForFunction(()=>NetworkToolkitFileEditor.active.content==='a=1&b=2&c=3');
  // A tool with nothing to change says so.
  await page.evaluate(()=>NetworkToolkitFileEditor.textMenu.runTool('sort-asc'));
  assert.match(await message(),/already in A→Z order/);
  // A failing tool changes nothing and names the position.
  await page.evaluate(()=>NetworkToolkitFileEditor.textMenu.runTool('json-pretty'));
  assert.match(await message(),/line 1, column 1\) Nothing was changed\./);
  assert.equal((await active()).content,'a=1&b=2&c=3');

  // 3. Extended mode: Find & and Replace \n with Replace all; Undo.
  await open('replace.txt');
  await text().focus();await page.keyboard.press('Control+h');
  const mode=page.locator('[data-ed-search-mode]');
  assert.deepEqual(await mode.locator('option').evaluateAll(list=>list.map(o=>[o.value,!!o.title])),[['normal',true],['extended',true],['regex',true]]);
  await mode.selectOption('extended');
  assert.match(await page.locator('[data-ed-search-mode-label]').getAttribute('title'),/Extended/);
  await page.locator('[data-ed-query]').fill('&');
  await page.locator('[data-ed-replacement]').fill('\\n');
  await page.locator('[data-ed="replaceAll"]').click();
  await page.waitForFunction(()=>NetworkToolkitFileEditor.active.content==='host=a\nport=22\nuser=admin');
  await shoot('editor-find-extended.png');
  await text().focus();await page.keyboard.press('Control+z');
  await page.waitForFunction(()=>NetworkToolkitFileEditor.active.content==='host=a&port=22&user=admin');
  // Regular expression mode: groups in the Replace text.
  await mode.selectOption('regex');
  await page.locator('[data-ed-query]').fill('(\\w+)=(\\w+)');
  await page.locator('[data-ed-replacement]').fill('$2:$1');
  await page.locator('[data-ed="replaceAll"]').click();
  await page.waitForFunction(()=>NetworkToolkitFileEditor.active.content==='a:host&22:port&admin:user');
  // The same on a disk-backed document (one line of 60,000 settings): the
  // engine's Replace All, and a single Replace that expands $2=$1.
  await page.locator('[data-ed="hideFind"]').click();
  await open('pairs.txt');
  let pairs=await active();
  assert.equal(pairs.disk,true);
  await text().focus();await page.keyboard.press('Control+Home');await page.keyboard.press('Control+h');
  await mode.selectOption('extended');
  await page.locator('[data-ed-query]').fill('&');
  await page.locator('[data-ed-replacement]').fill('\\n');
  await page.locator('[data-ed="replaceAll"]').click();
  await page.waitForFunction(()=>/^Replaced 60,?000 occurrence/.test(document.querySelector('[data-ed-message]').textContent),null,{timeout:60000});
  await settled();
  pairs=await active();
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.diskMeta.lines),60001);
  assert.equal((await engine('window',{start:0,limit:16})).text,'k0=0\nk1=1\nk2=2\nk');
  await text().focus();await page.keyboard.press('Control+z');
  await page.waitForFunction(()=>NetworkToolkitFileEditor.active.diskMeta.lines===1&&!NetworkToolkitFileEditor.active.dirty,null,{timeout:30000});
  await mode.selectOption('regex');
  await page.locator('[data-ed-query]').fill('(\\w+)=(\\d)');
  await page.locator('[data-ed-replacement]').fill('$2=$1');
  await text().focus();await page.keyboard.press('Control+Home');
  await page.locator('[data-ed="replaceOne"]').click();
  await page.waitForFunction(()=>/^Match on line 1/.test(document.querySelector('[data-ed-message]').textContent));
  await page.locator('[data-ed="replaceOne"]').click();
  await page.waitForFunction(()=>NetworkToolkitFileEditor.active.dirty&&!NetworkToolkitFileEditor.active.diskSession.pending);
  assert.equal((await engine('window',{start:0,limit:12})).text,'0=k0&k1=1&k2');
  await page.locator('[data-ed="hideFind"]').click();

  // 4. Base64 decode of a whole 1.7 MB single-line document, nothing selected.
  const plain=fs.readFileSync(file('expected-b64.txt'),'utf8');
  await open('large.b64');
  let shown=await active();
  assert.equal(shown.disk,true,'a 1.7 MB file is disk-backed');assert(shown.units>1048576,'more than the old 1 MiB limit');
  await text().focus();await page.keyboard.press('Control+Home');
  await page.keyboard.press('Control+Shift+B');
  await page.waitForFunction(()=>/^Decoded the whole document from Base64\./.test(document.querySelector('[data-ed-message]').textContent),null,{timeout:60000});
  await settled();
  shown=await active();
  assert.equal(shown.units,plain.length,'every character was decoded');assert.equal(shown.dirty,true);
  assert.equal(await diskText(shown.units),plain);
  await text().focus();await page.keyboard.press('Control+z');
  await page.waitForFunction(units=>NetworkToolkitFileEditor.active.diskMeta.units===units&&!NetworkToolkitFileEditor.active.dirty,fs.statSync(file('large.b64')).size,{timeout:30000});

  // A long tool shows its progress above the text and can be cancelled;
  // the document is then unchanged.
  await open('hosts.txt');
  const hosts=await active();
  await text().focus();await page.keyboard.press('Control+Home');
  await page.evaluate(()=>{globalThis.__sorting=NetworkToolkitFileEditor.textMenu.runTool('sort-asc');});
  const progress=page.locator('[data-ed-tool-bar="progress"]');
  await progress.waitFor({state:'visible'});
  assert.match(await progress.locator('[data-ed-tool-progress-text]').textContent(),/^Sort A→Z… \d+%$/);
  await shoot('editor-tool-progress.png');
  assert.equal(await page.evaluate(()=>document.querySelector('.nt-editor-code textarea[data-ed-text]').readOnly),true,'the document is read-only while the tool runs');
  await progress.locator('[data-ed-tool-cancel]').click().catch(()=>{});
  await page.evaluate(()=>__sorting);
  await progress.waitFor({state:'detached'});
  const outcome=await message();
  if(/cancelled/.test(outcome)){
   const now=await active();
   assert.deepEqual([now.revision,now.dirty,now.units],[hosts.revision,false,hosts.units],'a cancelled sort changed nothing');
  }else{
   // The sort finished before Cancel reached it: one undo step takes it back.
   assert.match(outcome,/^Sorted the whole document A→Z/);
   await text().focus();await page.keyboard.press('Control+z');
   await page.waitForFunction(()=>!NetworkToolkitFileEditor.active.dirty,null,{timeout:30000});
  }

  // 5. The guide's recipe for a one-line encoded settings file (synthetic
  // key=value& settings with %XX escapes as Base64, followed by "&&"; 1.5 MB,
  // so disk-backed): Base64 decode stops at the "&&" and changes nothing;
  // Decode the part before it; Split at text &; URL decode; Undo.
  const encodedFile=fs.readFileSync(file('settings.b64'),'utf8');
  const decoded=Buffer.from(encodedFile.slice(0,-2),'base64').toString('utf8')+'&&';
  const readable=fs.readFileSync(file('expected-readable.txt'),'utf8');
  await open('settings.b64');
  const encodedTab=await active();
  assert.equal(encodedTab.disk,true,'a 1.5 MB file is disk-backed');
  await text().focus();await page.keyboard.press('Control+Home');
  await page.keyboard.press('Control+Shift+B');
  const partBar=page.locator('[data-ed-tool-bar="partial"]');
  await partBar.waitFor({state:'visible',timeout:60000});
  const stopped="'&' is not a Base64 character. (line 1, column "+(encodedFile.length-1)+') Nothing was changed.';
  assert.equal(await message(),stopped,'the message names the character, its line and column');
  assert.equal(await partBar.locator('[data-ed-tool-partial-text]').textContent(),stopped);
  assert.equal(await partBar.locator('button').count(),1,'the bar offers one button');
  const decodeBefore=partBar.locator('[data-ed-tool-partial]');
  assert.equal(await decodeBefore.textContent(),'Decode the part before it');
  assert.equal(await decodeBefore.getAttribute('title'),'Decodes the whole document from its start up to line 1, column '+(encodedFile.length-1)+'. That character and everything after it stay as they are; Undo (Ctrl+Z) takes it back.');
  shown=await active();
  assert.deepEqual([shown.revision,shown.dirty,shown.units],[encodedTab.revision,false,encodedFile.length],'Base64 decode changed nothing');
  await shoot('editor-decode-part-before.png');
  await decodeBefore.click();
  await page.waitForFunction(()=>/^Decoded the part before line 1, column \d+ from Base64; the text from there on is unchanged\. Undo \(Ctrl\+Z\) takes it back\.$/.test(document.querySelector('[data-ed-message]').textContent),null,{timeout:60000});
  await settled();
  assert.equal(await partBar.count(),0,'the bar is gone');
  shown=await active();
  assert.equal(shown.dirty,true);
  assert.equal(await diskText(shown.units),decoded,'the Base64 is decoded and the "&&" after it is unchanged');
  // Split at text & (Tools → Lines), with the dialog's own default. The
  // decoded part is still selected, so the next steps work on it and the
  // "&&" after it stays as it is.
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.textMenu.state().selection),true,'the decoded part is selected');
  await tools.click();await menu.waitFor({state:'visible'});
  await menu.locator('[data-editor-submenu="tools-lines"]').click();
  await lines.locator('[data-editor-menu-action="split"]').click();
  await dialog.waitFor({state:'visible'});
  assert.equal(await dialog.locator('[data-ed-dialog-input]').inputValue(),'&');
  await dialog.locator('[data-ed-dialog-action="accept"]').click();
  await page.waitForFunction(()=>/^Split the selection at [\d,.]+ place\(s\)\./.test(document.querySelector('[data-ed-message]').textContent),null,{timeout:60000});
  await settled();
  const split=await diskText((await active()).units);
  assert.equal(split,decoded.slice(0,-2).replaceAll('&','\n')+'&&','one setting per line');
  // URL decode (Tools → Encode / decode).
  await tools.click();await menu.waitFor({state:'visible'});
  await menu.locator('[data-editor-submenu="tools-encode"]').click();
  await menu.locator('[data-editor-submenu-panel="tools-encode"] [data-editor-menu-action="url-decode"]').click();
  await page.waitForFunction(()=>/^Decoded [\d,.]+ URL escape\(s\) in the selection\./.test(document.querySelector('[data-ed-message]').textContent),null,{timeout:60000});
  await settled();
  shown=await active();
  assert.equal(await diskText(shown.units),readable,'every setting readable, one per line, in order');
  await shoot('editor-recipe-readable.png');
  // Undo takes back each step: URL decode, the split, the partial decode.
  for(const [step,want] of [['URL decode',split],['Split at text',decoded],['Decode the part before it',encodedFile]]){
   const units=want.length;
   await text().focus();await page.keyboard.press('Control+z');
   await page.waitForFunction(units=>NetworkToolkitFileEditor.active.diskMeta.units===units&&!NetworkToolkitFileEditor.active.diskSession?.pending,units,{timeout:30000});
   assert.equal(await diskText(units),want,'Undo of '+step);
  }
  assert.equal((await active()).dirty,false,'back to the file as it was read');

  // The same on page-held text, with a selection: Decode the part before it
  // decodes from the start of the selection; the rest stays as it is.
  const small=fs.readFileSync(file('small.b64'),'utf8');
  await open('small.b64');
  assert.equal((await active()).disk,false,'a small file is page-held');
  await text().focus();
  await text().evaluate((el,end)=>el.setSelectionRange(0,end),small.length-1);
  await page.keyboard.press('Control+Shift+B');
  await partBar.waitFor({state:'visible'});
  assert.equal(await message(),"'&' is not a Base64 character. (line 1, column "+(small.length-1)+') Nothing was changed.');
  assert.match(await decodeBefore.getAttribute('title'),/^Decodes the selection from its start up to line 1, column \d+\./);
  assert.equal((await active()).content,small,'nothing changed');
  // Another tab takes the offer away; it is not shown again.
  await page.locator('[data-ed-open]').filter({hasText:'settings.txt'}).click();
  assert.equal(await partBar.count(),0);
  await page.locator('[data-ed-open]').filter({hasText:'small.b64'}).click();
  assert.equal(await partBar.count(),0,'the offer belongs to the moment the decoder stopped');
  await text().focus();
  await text().evaluate((el,end)=>el.setSelectionRange(0,end),small.length-1);
  await page.keyboard.press('Control+Shift+B');
  await partBar.waitFor({state:'visible'});
  await decodeBefore.click();
  await page.waitForFunction(want=>NetworkToolkitFileEditor.active.content===want,fs.readFileSync(file('expected-small.txt'),'utf8'));
  assert.match(await message(),/^Decoded the part before line 1, column \d+ from Base64; the text from there on is unchanged\./);
  await text().focus();await page.keyboard.press('Control+z');
  await page.waitForFunction(want=>NetworkToolkitFileEditor.active.content===want,small);
  // With nothing before the problem, nothing is offered.
  await text().evaluate(el=>el.setSelectionRange(el.value.length-2,el.value.length));
  await page.keyboard.press('Control+Shift+B');
  await page.waitForFunction(()=>/^'&' is not a Base64 character\./.test(document.querySelector('[data-ed-message]').textContent));
  assert.equal(await partBar.count(),0,'nothing to decode before it, so no offer');

  // The command palette lists the tools of the editor.
  await page.keyboard.press('Control+Shift+P');
  await page.locator('.ux-command-input').fill('base64 decode');
  assert.match(await page.locator('.ux-command-row').first().textContent(),/Text tools: Base64 decode/);
  await page.keyboard.press('Escape');

  assert.deepEqual(errors,[],'no page errors');
  console.log('editor text tools: ok');
 }catch(error){
  if(page&&output)await page.screenshot({path:path.join(output,'editor-text-tools-failure.png')}).catch(()=>{});
  console.error(error);process.exitCode=1;
 }finally{await browser.close();}
})();
