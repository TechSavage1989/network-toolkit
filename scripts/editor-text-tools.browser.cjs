'use strict';
// Driven by TestEditorSessionBrowserTextTools (editor_text_tools_browser_test.go):
// the shipped page of the standalone Text Editor against its production
// engine, through the Tools menu (owner request v1.21.53).
//  1. The Tools button opens the tool groups; each item shows a description,
//     the same as its tooltip, and its shortcut.
//  2. Split at text on page-held text: & becomes a line break; Undo.
//  3. The Find bar's Extended mode: Replace all & with \n; Undo. Regular
//     expression mode: $2=$1. Both on page-held and disk-backed text.
//  4. Base64 decode of a whole 1.7 MB single-line disk-backed document with
//     nothing selected (Ctrl+Shift+B); Undo.
//  5. A 1.5 MB SonicWall export offers its readable copy; Make readable opens
//     a new tab with every setting and leaves the export unchanged. Another
//     export's offer can be dismissed.
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
  assert.deepEqual(groups.map(g=>g[0]),['Lines','Encode / decode','Format','Device configs']);
  for(const [label,title,shown] of groups)assert(title&&title===shown,label+' shows its description, also as its tooltip');
  await menu.locator('[data-editor-submenu="tools-lines"]').click();
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

  // 5. The SonicWall export: the offer, and its readable copy in a new tab.
  const readable=fs.readFileSync(file('expected.txt'),'utf8');
  await open('firewall.exp');
  const exportTab=await active();
  assert.equal(exportTab.disk,true);
  const offer=page.locator('[data-ed-tool-bar="offer"]');
  await offer.waitFor({state:'visible'});
  assert.equal(await offer.locator('.nt-editor-insert-text').textContent(),'This looks like a SonicWall settings export.');
  assert.match(await offer.locator('[data-ed-tool-offer="readable"]').getAttribute('title'),/new tab/);
  await shoot('editor-sonicwall-offer.png');
  await offer.locator('[data-ed-tool-offer="readable"]').click();
  await page.waitForFunction(()=>/^Opened a readable copy of firewall\.exp: /.test(document.querySelector('[data-ed-message]').textContent),null,{timeout:60000});
  await settled();
  shown=await active();
  assert.deepEqual([shown.name,shown.untitled,shown.disk,shown.dirty],['firewall (readable).txt',true,true,true]);
  assert.equal(shown.units,readable.length);
  assert.equal(await diskText(shown.units),readable,'every setting, one per line, in order');
  assert.match(await message(),new RegExp('Opened a readable copy of firewall\\.exp: '+(readable.split('\n').length-1).toLocaleString('en-US').replace(/,/g,'.?')+' settings'));
  await shoot('editor-sonicwall-readable.png');
  assert.equal(await page.locator('[data-ed-tool-bar="offer"]').count(),0,'no offer on the readable copy');
  // The export tab is unchanged and offers nothing more.
  await page.locator('[data-ed-open]').filter({hasText:'firewall.exp'}).click();
  await settled();
  const after=await active();
  assert.deepEqual([after.name,after.dirty,after.revision],['firewall.exp',false,exportTab.revision]);
  assert.equal(await page.locator('[data-ed-tool-bar="offer"]').count(),0);
  // Another export: Dismiss hides the offer for that tab.
  await open('branch.exp');
  await offer.waitFor({state:'visible'});
  await offer.locator('[data-ed-tool-offer="dismiss"]').click();
  assert.equal(await page.locator('[data-ed-tool-bar="offer"]').count(),0);
  await page.locator('[data-ed-open]').filter({hasText:'settings.txt'}).click();
  await page.locator('[data-ed-open]').filter({hasText:'branch.exp'}).click();
  assert.equal(await page.locator('[data-ed-tool-bar="offer"]').count(),0,'a dismissed offer stays dismissed');

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
