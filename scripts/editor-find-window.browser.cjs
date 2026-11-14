'use strict';
// Driven by TestEditorSessionBrowserFindWindow (editor_find_browser_test.go):
// the shipped page of the standalone Text Editor against its production
// engine, through the Find window (owner request v1.21.55).
//  1. Find: Ctrl+F opens the window on Find; Find Next, F3 / Shift+F3,
//     Backward direction, Match case, whole word, Wrap around off, Count, the
//     search modes (Extended, Regular expression with ". matches newline"),
//     the history list, Escape, and the window keeps its place.
//  2. Find All in the current document and in all open documents: the
//     Search results panel, jump (double-click and Enter), collapse, the
//     context menu (Copy, Collapse all, Expand all, Clear all), and a
//     document with 200,000 hits whose rows are drawn only in view.
//  3. Replace: Ctrl+H, swap, Replace, Replace All (one undo step), Replace
//     All in All Opened Documents (skips the file that is not text; nothing
//     is saved; Undo in each document).
//  4. Mark: Ctrl+M, Mark All with Bookmark line and Purge, marks painted on
//     page-held and disk-backed text, bookmarks (Ctrl+F2, F2, Shift+F2) that
//     follow edits, Copy Marked Text, Clear all marks.
//  5. In selection on page-held text and on a 5 MB disk-backed document.
//  6. Transparency is kept across a reload; the window fits a 640×400
//     detached editor window.
// The Go test then checks that no file changed on disk and that no search
// text reached the saved settings. Screenshots (light and dark, at device
// scale 1) go to TOOLKIT_TEST_OUTPUT when it is set.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {NTK_EDITOR_STANDALONE_URL:url,NTK_EDITOR_STANDALONE_TOKEN:token,NTK_EDITOR_FIND_FILES:files}=process.env;
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const file=name=>path.join(files,name);
const step=name=>console.log('FIND WINDOW '+name);
(async()=>{
 const browser=await chromium.launch({headless:true,...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 const errors=[];let page,context;
 const win=()=>page.locator('[data-ed-find]');
 const results=()=>page.locator('[data-ed-find-results]');
 const findMessage=()=>page.locator('[data-ed-find-message]').textContent();
 const settled=()=>page.waitForFunction(()=>{const d=NetworkToolkitFileEditor.active;return d&&!d.loading&&!d.saving&&!d.diskEditing&&!d.diskSession?.pending&&!document.querySelector('.nt-find-window.is-busy');},null,{timeout:60000});
 const open=async name=>{assert.equal(await page.evaluate(path=>NetworkToolkitFileEditor.openLocalPath(path,false,true).then(()=>'',error=>error.message),file(name)),'');await settled();};
 const activate=name=>page.evaluate(name=>{const tab=[...document.querySelectorAll('[data-ed-open]')].find(el=>el.textContent.includes(name));tab.click();},name);
 const active=()=>page.evaluate(()=>{const d=NetworkToolkitFileEditor.active;return {name:d.path.split(/[\\/]/).pop(),disk:!!d.disk,dirty:!!d.dirty,content:d.disk?'':d.content,units:d.disk?d.diskMeta.units:d.content.length};});
 const text=()=>page.locator('.nt-editor-code textarea[data-ed-text]').first();
 const selected=()=>page.evaluate(async()=>{const d=NetworkToolkitFileEditor.active,ta=document.querySelector('.nt-editor-code textarea[data-ed-text]');if(d.disk){return {disk:true};}return {start:ta.selectionStart,end:ta.selectionEnd,text:ta.value.slice(ta.selectionStart,ta.selectionEnd)};});
 const query=value=>page.locator('[data-ed-query]').fill(value);
 const option=name=>win().locator('[data-ed-find-option="'+name+'"]');
 const until=(fn,arg,timeout=60000)=>page.waitForFunction(fn,arg,{timeout});
 const messageIs=re=>until(source=>new RegExp(source).test(document.querySelector('[data-ed-find-message]')?.textContent||''),re.source);
 const shoot=async name=>{if(output)await page.screenshot({path:path.join(output,name)});};
 // The Find window on a tab: the shortcut opens it when it is closed (it
 // closes it when that tab shows, so it is not pressed then).
 const showTab=async name=>{if(!await win().isVisible())await page.keyboard.press({find:'Control+f',replace:'Control+h',mark:'Control+m'}[name]);await win().waitFor({state:'visible'});await win().locator('[data-ed-find-tab="'+name+'"]').click();await until(n=>document.querySelector('[data-ed-find]').dataset.tab===n,name);};
 const theme=async value=>{await page.evaluate(value=>setTheme(value),value);await page.waitForTimeout(120);};
 const load=async()=>{
  await page.goto(url);
  await page.waitForFunction(()=>window.NetworkToolkitEditorWindows?.bufferSupported()&&window.__networkToolkitStartupState===1);
  assert.equal(await page.evaluate(()=>EDITOR_STANDALONE),true);
  await page.waitForFunction(()=>NetworkToolkitFileEditor.session.phase==='ready');
 };
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  context=await browser.newContext({viewport:{width:1280,height:820},deviceScaleFactor:1});
  await context.grantPermissions(['clipboard-read','clipboard-write'],{origin:url});
  page=await context.newPage();
  page.on('pageerror',error=>errors.push(error.message));
  await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},token);
  await load();

  // ---- 1. Find ----
  step('find');
  await open('config.txt');
  assert.equal((await active()).disk,false,'a small file is page-held');
  assert.equal(await page.locator('[data-ed-find]').count(),0,'the window is made when it is first opened');
  await text().focus();await page.keyboard.press('Control+Home');
  await page.keyboard.press('Control+f');
  await win().waitFor({state:'visible'});
  assert.equal(await win().getAttribute('data-tab'),'find');
  assert.equal(await page.evaluate(()=>document.activeElement.matches('[data-ed-query]')),true,'Find what has the focus');
  assert.deepEqual(await win().locator('[data-ed-find-tab]').evaluateAll(list=>list.map(b=>[b.textContent,b.getAttribute('aria-selected')])),[['Find','true'],['Replace','false'],['Mark','false']]);
  const visibleButtons=()=>win().locator('.nt-find-buttons .btn').evaluateAll(list=>list.filter(b=>b.offsetParent).map(b=>b.textContent));
  assert.deepEqual(await visibleButtons(),['Find Next','Count','Find All in Current Document','Find All in All Opened Documents']);
  // The toolbar's Find shows that its tab is open, and closes the window
  // again; Replace switches the tab; Ctrl+F opens and closes it too.
  const toolbarFind=page.locator('.nt-editor-toolbar [data-ed="find"]'),toolbarReplace=page.locator('.nt-editor-toolbar [data-ed="replace"]');
  assert.equal(await toolbarFind.getAttribute('aria-pressed'),'true');assert.equal(await toolbarReplace.getAttribute('aria-pressed'),'false');
  await toolbarReplace.click();await until(()=>document.querySelector('[data-ed-find]').dataset.tab==='replace');
  assert.equal(await toolbarReplace.getAttribute('aria-pressed'),'true');assert.equal(await toolbarFind.getAttribute('aria-pressed'),'false');
  await toolbarReplace.click();await win().waitFor({state:'hidden'});
  assert.equal(await toolbarReplace.getAttribute('aria-pressed'),'false');
  await toolbarFind.click();await win().waitFor({state:'visible'});assert.equal(await win().getAttribute('data-tab'),'find');
  await page.keyboard.press('Control+f');await win().waitFor({state:'hidden'});
  await page.keyboard.press('Control+f');await win().waitFor({state:'visible'});
  assert.equal(await win().locator('[data-ed="hideFind"]').count(),0,'no Close button: the toolbar button and × close the window');
  assert.equal(await option('wrap').isChecked(),true,'Wrap around is on');
  assert.equal(await option('inSelection').isDisabled(),true,'In selection needs a selection');
  assert.equal(await option('dotAll').isDisabled(),true,'. matches newline is for regular expressions');
  await query('uplink');await page.keyboard.press('Enter');
  await until(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]');return ta.value.slice(ta.selectionStart,ta.selectionEnd)==='uplink';});
  const first=await selected();
  await page.keyboard.press('F3');
  await until(start=>document.querySelector('.nt-editor-code textarea[data-ed-text]').selectionStart>start,first.start);
  const second=await selected();
  assert.equal(second.text,'uplink');
  await page.keyboard.press('Shift+F3');
  await until(start=>document.querySelector('.nt-editor-code textarea[data-ed-text]').selectionStart===start,first.start);
  // Backward direction: Find Next goes up; from the first match it wraps.
  await option('backward').check();
  await win().locator('[data-ed="next"]').click();
  await messageIs(/Found the last occurrence from the bottom/);
  await option('backward').uncheck();
  // Wrap around off: past the last match nothing is found.
  await option('wrap').uncheck();
  await page.evaluate(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]');ta.setSelectionRange(ta.value.length,ta.value.length);});
  await win().locator('[data-ed="next"]').click();
  await messageIs(/Can’t find the text “uplink”/);
  await option('wrap').check();
  // Count, Match case and Match whole word only.
  await query('VLAN');await win().locator('[data-ed-find-action="count"]').click();
  await messageIs(/^Count: 12 matches\.$/);
  await option('matchCase').check();await win().locator('[data-ed-find-action="count"]').click();
  await messageIs(/^Count: 0 matches\.$/);
  await option('matchCase').uncheck();
  await query('ge-0/0/1');await option('word').check();await win().locator('[data-ed-find-action="count"]').click();
  await messageIs(/^Count: 1 match\.$/);
  await option('word').uncheck();await win().locator('[data-ed-find-action="count"]').click();
  await messageIs(/^Count: 4 matches\.$/);
  // Extended: \t is a tab.
  await win().locator('[data-ed-search-mode] input[value="extended"]').check();
  assert.match(await win().locator('[data-ed-search-mode]').getAttribute('title'),/Extended/);
  await query('\\tvlan');await win().locator('[data-ed-find-action="count"]').click();
  await messageIs(/^Count: 12 matches\.$/);
  // Regular expression, with ^ at each line and ". matches newline".
  await win().locator('[data-ed-search-mode] input[value="regex"]').check();
  assert.equal(await option('dotAll').isDisabled(),false);
  await query('^interface ge-0/0/\\d+$');await win().locator('[data-ed-find-action="count"]').click();
  await messageIs(/^Count: 12 matches\.$/);
  await query('ge-0/0/12. description');await win().locator('[data-ed-find-action="count"]').click();
  await messageIs(/^Count: 0 matches\.$/);
  await option('dotAll').check();await win().locator('[data-ed-find-action="count"]').click();
  await messageIs(/^Count: 1 match\.$/);
  await option('dotAll').uncheck();
  // The syntax guide: "?" beside Search Mode opens it for the mode, and
  // closes it again; a row inserts its token at the cursor. The line under
  // Find what explains the part at the cursor, or names the problem.
  const guideToggle=win().locator('[data-ed-find-guide-toggle]'),guide=win().locator('[data-ed-find-guide]'),explainLine=win().locator('[data-ed-find-explain]');
  const explained=re=>until(source=>new RegExp(source).test(document.querySelector('[data-ed-find-explain]').textContent),re.source);
  await guideToggle.click();await guide.waitFor({state:'visible'});
  assert.equal(await guideToggle.getAttribute('aria-expanded'),'true');
  const tokens=await guide.locator('.nt-find-guide-row code').evaluateAll(list=>list.map(c=>c.textContent));
  for(const token of ['.','\\d  \\D','\\w  \\W','\\s  \\S','\\b','^  $','*','+','?','{n}  {n,m}','[abc]','[^abc]','[a-z]','( )','(?: )','(?<name> )','|','\\','(?i)'])assert(tokens.includes(token),token+' is in the guide');
  assert.equal(tokens.includes('$1'),false,'the Replace with rows are on the Replace tab');
  assert.match(await guide.textContent(),/Not supported.*lookahead and lookbehind.*backreferences/);
  await query('port ');await win().locator('[data-ed-query]').press('End');
  await guide.locator('.nt-find-guide-row').filter({hasText:'a digit 0–9'}).click();
  assert.equal(await page.locator('[data-ed-query]').inputValue(),'port \\d');
  await explained(/^\\d — a digit 0–9$/);
  await page.keyboard.type('+');
  await explained(/^\+ — the item before, once or more$/);
  await guide.locator('.nt-find-guide-row').filter({hasText:'a group: repeats'}).click();
  assert.equal(await page.locator('[data-ed-query]').inputValue(),'port \\d+()');
  assert.equal(await page.evaluate(()=>document.activeElement.selectionStart),9,'the cursor is inside the group');
  await query('foo(?=bar)');
  await explained(/^Problem at character 4: Lookahead and lookbehind \(\(\?=, \(\?!, \(\?<=, \(\?<!\) are not supported\.$/);
  assert.match(await explainLine.getAttribute('class'),/error/);
  await win().locator('[data-ed-find-action="count"]').click();
  await messageIs(/^Invalid regular expression at character 4: Lookahead and lookbehind/);
  await query('(\\w+');await explained(/^Problem at character 1: A \( has no closing \)\.$/);
  await theme('dark');await shoot('editor-find-window-guide-dark.png');
  await theme('light');await shoot('editor-find-window-guide-light.png');await theme('dark');
  await win().locator('[data-ed-find-tab="replace"]').click();
  assert((await guide.locator('.nt-find-guide-row code').evaluateAll(list=>list.map(c=>c.textContent))).includes('$1'),'Replace with rows on the Replace tab');
  await page.locator('[data-ed-replacement]').fill('$2=$1');await page.locator('[data-ed-replacement]').press('End');
  await explained(/^\$1 — the text of group 1$/);
  await win().locator('[data-ed-find-tab="find"]').click();
  await win().locator('[data-ed-search-mode] input[value="extended"]').check();
  assert.match(await guide.textContent(),/\\uNNNN/);
  await query('a\\tb');await win().locator('[data-ed-query]').press('ArrowLeft');
  await explained(/^\\t — a tab$/);
  await query('\\0');await explained(/NUL/);
  await guideToggle.click();await guide.waitFor({state:'hidden'});
  assert.equal(await guideToggle.getAttribute('aria-expanded'),'false');
  await win().locator('[data-ed-search-mode] input[value="normal"]').check();
  assert.equal(await explainLine.isVisible(),false,'no line in Normal mode');
  await win().locator('[data-ed-search-mode] input[value="normal"]').check();
  // Find Next moves on from a match of empty text (^$ on an empty line),
  // and Replace replaces it and goes on.
  await open('blank.txt');
  await text().focus();await page.keyboard.press('Control+Home');
  await showTab('replace');
  await win().locator('[data-ed-search-mode] input[value="regex"]').check();
  await query('^$');await page.locator('[data-ed-replacement]').fill('-');
  await win().locator('[data-ed="next"]').click();await messageIs(/^Match on line 2\.$/);
  await win().locator('[data-ed="next"]').click();await messageIs(/^Match on line 4\.$/);
  await win().locator('[data-ed="next"]').click();await messageIs(/^Match on line 6\.$/);
  await win().locator('[data-ed="next"]').click();await messageIs(/Found the first occurrence from the top/);
  await win().locator('[data-ed="replaceOne"]').click();
  await until(()=>NetworkToolkitFileEditor.active.content==='a\n-\nb\n\nc\n');
  await messageIs(/^Match on line 4\.$/);
  await text().focus();await page.keyboard.press('Control+z');
  await until(()=>NetworkToolkitFileEditor.active.content==='a\n\nb\n\nc\n'&&!NetworkToolkitFileEditor.active.dirty);
  await page.locator('[data-ed-close-tab][aria-label="Close blank.txt"]').click();
  await until(()=>![...document.querySelectorAll('[data-ed-open]')].some(t=>t.textContent.includes('blank.txt')));
  await win().locator('[data-ed-search-mode] input[value="normal"]').check();
  await showTab('find');
  // Search as you type (Normal mode): a pause in typing selects the first
  // match from the cursor; no button is pressed.
  await activate('config.txt');await settled();await text().focus();await page.keyboard.press('Control+Home');
  await win().locator('[data-ed-query]').fill('');await win().locator('[data-ed-query]').focus();await page.keyboard.type('vlan 3');
  await until(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]');return ta.value.slice(ta.selectionStart,ta.selectionEnd)==='vlan 3';});
  assert.equal(await page.evaluate(()=>document.activeElement.matches('[data-ed-query]')),true,'the focus stays in Find what');
  // The editor's shortcuts work from the Find window (Ctrl+N: a new
  // document); the field keeps its own editing keys (Ctrl+A selects its text).
  const tabs=await page.locator('[data-ed-open]').count();
  await page.keyboard.press('Control+n');
  await until(n=>document.querySelectorAll('[data-ed-open]').length===n+1,tabs);
  await page.locator('[data-ed-close-tab]').last().click();
  await until(n=>document.querySelectorAll('[data-ed-open]').length===n,tabs);
  await activate('config.txt');await settled();
  await win().locator('[data-ed-query]').focus();await page.keyboard.press('Control+a');
  assert.equal(await page.evaluate(()=>{const el=document.activeElement;return el.matches('[data-ed-query]')&&el.selectionStart===0&&el.selectionEnd===el.value.length;}),true);
  // A shortcut nobody took never reaches the browser (Ctrl+P would print
  // the page); the field's own keys keep their default.
  const prevented=key=>page.evaluate(key=>{const e=new KeyboardEvent('keydown',{key,ctrlKey:true,bubbles:true,cancelable:true});document.querySelector('[data-ed-query]').dispatchEvent(e);return e.defaultPrevented;},key);
  assert.equal(await prevented('p'),true,'Ctrl+P from the Find window');
  assert.equal(await prevented('c'),false,'Ctrl+C in Find what');
  await showTab('find');
  // The history list holds the searches, newest first.
  await query('');
  await win().locator('[data-ed-query] ~ .combobox-toggle').click();
  await page.locator('#comboboxPopup').waitFor({state:'visible'});
  const history=await page.locator('#comboboxPopup [role="option"]').evaluateAll(list=>list.map(o=>o.textContent));
  assert.deepEqual(history.slice(0,4),['^$','foo(?=bar)','ge-0/0/12. description','^interface ge-0/0/\\d+$']);
  await page.keyboard.press('Escape');
  await theme('dark');await shoot('editor-find-window-find-dark.png');
  await theme('light');await shoot('editor-find-window-find-light.png');await theme('dark');
  // Escape closes the window; it opens again where the user moved it.
  const head=win().locator('[data-ed-find-drag]'),box=await head.boundingBox();
  await page.mouse.move(box.x+box.width-80,box.y+10);await page.mouse.down();await page.mouse.move(box.x+box.width-200,box.y+80,{steps:4});await page.mouse.up();
  const moved=await win().boundingBox();
  await win().locator('[data-ed-query]').focus();await page.keyboard.press('Escape');
  await win().waitFor({state:'hidden'});
  assert.equal(await page.evaluate(()=>document.activeElement.matches('[data-ed-text]')),true,'the text has the focus again');
  await page.keyboard.press('Control+f');await win().waitFor({state:'visible'});
  const again=await win().boundingBox();
  assert(Math.abs(again.x-moved.x)<2&&Math.abs(again.y-moved.y)<2,'the window opens where it was moved');
  step('find done');

  // ---- 2. Find All and the Search results panel ----
  step('find all');
  await open('second.txt');
  await open('hits.txt');
  assert.equal((await active()).disk,true,'100,000 lines are disk-backed');
  await open('image.bin');
  await activate('config.txt');await settled();
  await showTab('find');
  await query('vlan');
  await win().locator('[data-ed-find-action="all-current"]').click();
  await results().waitFor({state:'visible'});
  // The status bar's Results button shows and hides the panel; it has no
  // close button of its own.
  const resultsToggle=page.locator('[data-ed-find-results-toggle]');
  await resultsToggle.waitFor({state:'visible'});
  assert.equal(await resultsToggle.getAttribute('aria-expanded'),'true');
  assert.equal(await results().locator('button[aria-label^="Close"]').count(),0);
  await messageIs(/^Find All: 12 hits in 1 file of 1 searched\.$/);
  await until(()=>document.querySelector('[data-ed-find-badge]').textContent==='12');
  await resultsToggle.click();await results().waitFor({state:'hidden'});
  assert.equal(await resultsToggle.getAttribute('aria-expanded'),'false');
  await resultsToggle.click();await results().waitFor({state:'visible'});
  const rowsText=()=>results().locator('.nt-find-row').evaluateAll(list=>list.map(r=>r.textContent));
  await until(()=>![...document.querySelectorAll('.nt-find-row')].some(r=>r.classList.contains('is-pending')));
  let rows=await rowsText();
  assert.match(rows[0],/^▾Search “vlan” \(12 hits in 1 file of 1 searched\) \[Normal\]$/);
  assert.match(rows[1],/config\.txt \(12 hits\)$/);
  assert.equal(rows[2],'Line 4: \tvlan 10');
  assert.equal(await results().locator('.nt-find-row.is-line').first().locator('mark').textContent(),'vlan');
  // Double-click jumps to the match and selects it.
  await results().locator('.nt-find-row.is-line').nth(2).dblclick();
  await until(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]');return ta.value.slice(ta.selectionStart,ta.selectionEnd)==='vlan'&&ta.value.slice(0,ta.selectionStart).split('\n').length===10;});
  // Find All in all open documents: the file that is not text is searched too.
  await win().locator('[data-ed-find-action="all-open"]').click();
  await messageIs(/^Find All: 200,016 hits in 4 files of 4 searched\.$/);
  await until(()=>![...document.querySelectorAll('.nt-find-row')].some(r=>r.classList.contains('is-pending')));
  rows=await rowsText();
  assert.match(rows[0],/^▾Search “vlan” \(200,016 hits in 4 files of 4 searched\) \[Normal\]$/);
  // Rows are drawn only in view, however many there are.
  const drawn=await page.evaluate(()=>document.querySelectorAll('.nt-find-row').length);
  assert(drawn<80,'only the rows in view are drawn: '+drawn);
  // The context menu: Collapse all leaves the two searches; expanding the
  // newest shows its four files.
  await results().locator('.nt-find-row.is-search').first().click({button:'right'});
  const menu=page.locator('.nt-find-results-menu');await menu.waitFor();
  assert.deepEqual(await menu.locator('button').evaluateAll(list=>list.map(b=>b.textContent)),['Copy','Copy selected lines','Collapse all','Expand all','Clear all']);
  await menu.locator('[data-ed-find-menu="collapse"]').click();
  await until(()=>document.querySelectorAll('.nt-find-row.is-search').length===2&&!document.querySelector('.nt-find-row.is-file'));
  await results().locator('.nt-find-row.is-search').first().locator('.nt-find-toggle').click();
  await until(()=>document.querySelectorAll('.nt-find-row.is-file').length===4);
  const hitsFile=results().locator('.nt-find-row.is-file',{hasText:'hits.txt'});
  assert.match(await hitsFile.textContent(),/hits\.txt \(200,000 hits\)$/);
  // Its 100,000 rows: scrolled to the end, their text is read a page at a
  // time, and only the rows in view are drawn.
  await hitsFile.locator('.nt-find-toggle').click();
  await until(()=>document.querySelector('[data-ed-find-results-scroll]').scrollHeight>100000*20);
  await page.evaluate(()=>{const s=document.querySelector('[data-ed-find-results-scroll]');s.scrollTop=s.scrollHeight;});
  await until(()=>[...document.querySelectorAll('.nt-find-row.is-line')].some(r=>/^Line 99,9\d\d: vlan \d+ name vlan-999\d\d$/.test(r.textContent)));
  assert(await page.evaluate(()=>document.querySelectorAll('.nt-find-row').length)<80,'only the rows in view are drawn');
  await shoot('editor-find-results-dark.png');await theme('light');await shoot('editor-find-results-light.png');await theme('dark');
  await results().locator('.nt-find-row.is-line').last().dblclick();
  await until(()=>NetworkToolkitFileEditor.active.path.endsWith('hits.txt'));
  await until(()=>/^Ln (99,9\d\d|100,000),/.test(document.querySelector('[data-ed-position]').textContent));
  await results().locator('.nt-find-row').first().click({button:'right'});
  await page.locator('.nt-find-results-menu [data-ed-find-menu="expand"]').click();
  await page.evaluate(()=>{document.querySelector('[data-ed-find-results-scroll]').scrollTop=0;});
  await until(()=>[...document.querySelectorAll('.nt-find-row.is-search')].every(r=>r.getAttribute('aria-expanded')==='true')&&document.querySelector('.nt-find-row.is-search'));
  // Keyboard: with config.txt collapsed, the row after second.txt's is its
  // first match; Enter jumps there.
  await results().locator('.nt-find-row.is-file',{hasText:'config.txt'}).first().locator('.nt-find-toggle').click();
  await until(()=>document.querySelector('.nt-find-row.is-file')?.getAttribute('aria-expanded')==='false');
  await results().locator('.nt-find-row',{hasText:'second.txt'}).first().click();
  await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter');
  await until(()=>NetworkToolkitFileEditor.active.path.endsWith('second.txt'));
  await until(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]');return ta.value.slice(ta.selectionStart,ta.selectionEnd)==='vlan';});
  // Copy selected lines.
  await results().locator('.nt-find-row',{hasText:'second.txt'}).first().click();
  await page.keyboard.press('Shift+ArrowDown');await page.keyboard.press('Shift+ArrowDown');
  await results().locator('.nt-find-row.is-selected').first().click({button:'right',modifiers:['Shift']}).catch(()=>{});
  await page.locator('.nt-find-results-menu [data-ed-find-menu="copy-lines"]').click();
  await messageIs(/^Copied 2 lines\.$/);
  assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),'vlan 99\nvlan 100');
  // A row whose document changed says so instead of jumping.
  await text().focus();await page.keyboard.press('Control+Home');await page.keyboard.type('X');
  await results().locator('.nt-find-row',{hasText:'second.txt'}).first().click();
  await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter');
  await messageIs(/second\.txt changed since this search/);
  await until(()=>[...document.querySelectorAll('.nt-find-row.is-file')].some(r=>r.textContent.includes('second.txt')&&r.textContent.includes('changed since this search')));
  await text().focus();await page.keyboard.press('Control+z');
  await results().locator('.nt-find-row.is-search').first().click({button:'right'});
  await page.locator('.nt-find-results-menu [data-ed-find-menu="clear"]').click();
  await until(()=>!document.querySelector('.nt-find-row'));
  await results().waitFor({state:'hidden'});await resultsToggle.waitFor({state:'hidden'});
  // The results of a search the engine no longer has: the rows say so once
  // and the panel stops asking for them.
  const pageRequests=[];
  const watch=request=>{if(request.url().endsWith('/api/editor-views/find')&&request.postDataJSON()?.action==='page')pageRequests.push(request.postDataJSON().job);};
  page.on('request',watch);
  await activate('hits.txt');await settled();await showTab('find');
  await query('name vlan');await win().locator('[data-ed-find-action="all-current"]').click();
  await messageIs(/^Find All: 100,000 hits in 1 file of 1 searched\.$/);
  await until(()=>![...document.querySelectorAll('.nt-find-row')].some(r=>r.classList.contains('is-pending')));
  const goneJob=pageRequests.at(-1);assert(goneJob,'the panel read a page of rows');
  await page.evaluate(job=>api('/editor-views/find','POST',{action:'release',job}),goneJob);
  const before=pageRequests.length;
  await page.evaluate(()=>{const s=document.querySelector('[data-ed-find-results-scroll]');s.scrollTop=s.scrollHeight/2;});
  await until(()=>[...document.querySelectorAll('.nt-find-row.is-file')].some(r=>r.textContent.includes('results are no longer available')));
  await page.waitForTimeout(1500);
  assert(pageRequests.length-before<=2,'the panel stopped asking for rows of a search that is gone: '+(pageRequests.length-before));
  page.off('request',watch);
  await results().locator('.nt-find-row').first().click({button:'right'});
  await page.locator('.nt-find-results-menu [data-ed-find-menu="clear"]').click();
  step('find all done');

  // ---- 3. Replace ----
  step('replace');
  await activate('config.txt');await settled();
  await text().focus();await page.keyboard.press('Control+Home');
  await page.keyboard.press('Control+h'); // from the Find tab: switches to Replace
  await until(()=>document.querySelector('[data-ed-find]').dataset.tab==='replace');
  assert.deepEqual(await visibleButtons(),['Find Next','Replace','Replace All','Replace All in All Opened Documents']);
  // Swap and the swap menu.
  await query('uplink');
  await until(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]');return ta.value.slice(ta.selectionStart,ta.selectionEnd)==='uplink';});
  await page.locator('[data-ed-replacement]').fill('downlink');
  await win().locator('.nt-find-swap > [data-ed-find-swap="swap"]').click();
  assert.equal(await page.locator('[data-ed-query]').inputValue(),'downlink');assert.equal(await page.locator('[data-ed-replacement]').inputValue(),'uplink');
  await win().locator('[data-ed-find-swap-menu]').click();
  await win().locator('.nt-find-swap-list [data-ed-find-swap="to-find"]').click();
  assert.equal(await page.locator('[data-ed-query]').inputValue(),'uplink');
  await page.locator('[data-ed-replacement]').fill('core');
  await win().locator('[data-ed-find-swap-menu]').click();
  await win().locator('.nt-find-swap-list [data-ed-find-swap="to-replace"]').click();
  assert.equal(await page.locator('[data-ed-replacement]').inputValue(),'uplink');
  await page.locator('[data-ed-replacement]').fill('core');
  // Replace: from the cursor (no selection) the first press finds, the
  // second replaces and finds the next. (Search as you type selected the
  // first match while uplink was typed; the cursor goes back to the start.)
  await text().focus();await page.keyboard.press('Control+Home');
  await win().locator('[data-ed="replaceOne"]').click();
  await until(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]');return ta.value.slice(ta.selectionStart,ta.selectionEnd)==='uplink';});
  await win().locator('[data-ed="replaceOne"]').click();
  await until(()=>NetworkToolkitFileEditor.active.content.includes('description core to 192.0.2.1\n'));
  await shoot('editor-find-window-replace-dark.png');await theme('light');await shoot('editor-find-window-replace-light.png');await theme('dark');
  // Replace All: one undo step.
  await win().locator('[data-ed="replaceAll"]').click();
  await messageIs(/^Replace All: 11 occurrences were replaced in the entire file\.$/);
  assert.equal((await active()).content.includes('uplink'),false);
  await text().focus();await page.keyboard.press('Control+z');
  await until(()=>NetworkToolkitFileEditor.active.content.split('uplink').length===12);
  await page.keyboard.press('Control+z');
  await until(()=>NetworkToolkitFileEditor.active.content.split('uplink').length===13);
  // Replace All in All Opened Documents: page-held and disk-backed
  // documents change and show as unsaved; the file that is not text is
  // skipped and named; nothing is saved.
  await showTab('replace');
  await query('vlan');await page.locator('[data-ed-replacement]').fill('VLAN');await option('matchCase').check();
  await win().locator('[data-ed-find-action="replace-open"]').click();
  await messageIs(/^Replace All in all opened documents: 200,0\d\d occurrences were replaced in 3 documents\. Nothing was saved: .* Skipped: image\.bin \(not a text file\)\./);
  await settled();
  const dirty=await page.evaluate(()=>NetworkToolkitFileEditor.windowAdapter.documents().map(d=>[d.path.split(/[\\/]/).pop(),!!d.dirty]));
  assert.deepEqual(Object.fromEntries(dirty),{'config.txt':true,'second.txt':true,'hits.txt':true,'image.bin':false});
  for(const name of ['config.txt','second.txt','hits.txt']){
   await activate(name);await settled();
   await text().focus();await page.keyboard.press('Control+z');
   await until(()=>!NetworkToolkitFileEditor.active.dirty,null,60000);
  }
  await option('matchCase').uncheck();
  // Replace All right after typing in a disk-backed document: the typing
  // is sent first, and the Replace All applies to it.
  await activate('hits.txt');await settled();
  await text().focus();await page.keyboard.press('Control+Home');
  await until(()=>NetworkToolkitFileEditor.active.diskSession.state().page.start===0);
  // The typing's edit is held on its way until Replace All has started.
  let holdEdit;const editHeld=new Promise(resolve=>{holdEdit=resolve;});let editSeen=false;
  await page.route('**/documents/*/buffer',async route=>{if(route.request().postDataJSON()?.action==='edit'&&!editSeen){editSeen=true;await editHeld;}await route.continue();});
  await page.keyboard.type('Z');await until(()=>NetworkToolkitFileEditor.active.diskSession.pending);
  await showTab('replace');await query('name vlan-99999');await page.locator('[data-ed-replacement]').fill('name last');
  await win().locator('[data-ed="replaceAll"]').click();
  // The edit reaches the route while Replace All waits for it (it is held
  // there); a busy runner may take longer than a fixed pause to send it.
  for(const t0=Date.now();!editSeen&&Date.now()-t0<15000;)await page.waitForTimeout(25);
  assert.equal(editSeen,true,'the typing was still on its way');holdEdit();
  await messageIs(/^Replace All: 1 occurrence was replaced in the entire file\.$/);
  await page.unroute('**/documents/*/buffer');
  await settled();
  assert.equal((await page.evaluate(async()=>{const d=NetworkToolkitFileEditor.active;return (await NetworkToolkitEditorWindows.buffer(d.uid,'window',{start:0,limit:6})).text;})),'Zvlan ');
  await text().focus();await page.keyboard.press('Control+z');await settled();await page.keyboard.press('Control+z');
  await until(()=>!NetworkToolkitFileEditor.active.dirty,null,60000);
  // A single Replace whose selection changes while the engine answers
  // replaces nothing there: it searches again.
  await activate('config.txt');await settled();
  await text().focus();await page.keyboard.press('Control+Home');
  await query('uplink');await page.locator('[data-ed-replacement]').fill('core');
  await until(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]');return ta.value.slice(ta.selectionStart,ta.selectionEnd)==='uplink'&&ta.selectionStart<100;});
  await page.route('**/api/editor-views/find',async route=>{if(route.request().postDataJSON()?.action==='replace-text')await new Promise(resolve=>setTimeout(resolve,700));await route.continue();});
  await page.evaluate(()=>{document.querySelector('[data-ed-find-message]').textContent='';});
  await win().locator('[data-ed="replaceOne"]').click();
  await page.evaluate(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]');ta.setSelectionRange(0,8);document.dispatchEvent(new Event('selectionchange'));});
  await messageIs(/^Match on line \d+\.$/);
  await page.unroute('**/api/editor-views/find');
  assert.equal((await active()).content.split('uplink').length-1,12,'nothing was replaced');
  assert.equal((await active()).content.startsWith('hostname'),true,'the new selection was not replaced');
  // A Replace All that makes a page-held document too large for the page
  // moves it to the engine: the replacement happens once and is reported.
  await activate('second.txt');await settled();
  await query('vlan');await page.locator('[data-ed-replacement]').fill('w'.repeat(300000));
  await win().locator('[data-ed="replaceAll"]').click();
  await messageIs(/^Replace All: 2 occurrences were replaced in the entire file\.$/);
  await until(()=>NetworkToolkitFileEditor.active.disk&&!NetworkToolkitFileEditor.active.promoting,null,60000);
  await settled();
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.diskMeta.units),600000+'vlan 99\n name lab\nvlan 100\n name voice\n'.length-8);
  await text().focus();await page.keyboard.press('Control+z');
  await until(()=>NetworkToolkitFileEditor.active.diskMeta?.units==='vlan 99\n name lab\nvlan 100\n name voice\n'.length,null,60000);
  step('replace done');

  // ---- 4. Mark ----
  step('mark');
  await activate('config.txt');await settled();
  await text().focus();await page.keyboard.press('Control+Home');
  await page.keyboard.press('Control+m'); // from the Replace tab: switches to Mark
  await until(()=>document.querySelector('[data-ed-find]').dataset.tab==='mark');
  assert.deepEqual(await visibleButtons(),['Mark All','Clear all marks','Copy Marked Text']);
  await query('192.0.2.1');await option('bookmark').check();
  await win().locator('[data-ed-find-action="mark"]').click();
  await messageIs(/^Mark: 4 matches\.$/);
  await until(()=>document.querySelectorAll('[data-ed-marks] mark').length===4);
  assert.deepEqual(await page.locator('[data-ed-gutter-lines] .is-bookmarked').evaluateAll(list=>list.map(r=>r.textContent)),['3','30','33','36']);
  // Marks and bookmarks follow an edit above them.
  await text().focus();await page.keyboard.press('Control+Home');await page.keyboard.press('Enter');
  await until(()=>[...document.querySelectorAll('[data-ed-gutter-lines] .is-bookmarked')].map(r=>r.textContent).join()==='4,31,34,37');
  await page.keyboard.press('Control+z');
  await until(()=>[...document.querySelectorAll('[data-ed-gutter-lines] .is-bookmarked')].map(r=>r.textContent).join()==='3,30,33,36');
  // A Replace All of a word on several lines keeps the number of lines:
  // every bookmark stays where it is, also after Undo.
  await showTab('replace');await query('uplink');await page.locator('[data-ed-replacement]').fill('up');
  await win().locator('[data-ed="replaceAll"]').click();
  await messageIs(/^Replace All: 12 occurrences were replaced/);
  assert.equal([...await page.locator('[data-ed-gutter-lines] .is-bookmarked').evaluateAll(list=>list.map(r=>r.textContent))].join(),'3,30,33,36');
  await text().focus();await page.keyboard.press('Control+z');
  await until(()=>NetworkToolkitFileEditor.active.content.includes('uplink'));
  assert.equal([...await page.locator('[data-ed-gutter-lines] .is-bookmarked').evaluateAll(list=>list.map(r=>r.textContent))].join(),'3,30,33,36');
  await showTab('mark');await query('192.0.2.1');
  // F2 and Shift+F2 go between bookmarks; Ctrl+F2 adds and removes one.
  await text().focus();await page.keyboard.press('Control+Home');
  await page.keyboard.press('F2');
  await until(()=>/^Ln 3,/.test(document.querySelector('[data-ed-position]').textContent));
  await page.keyboard.press('F2');
  await until(()=>/^Ln 30,/.test(document.querySelector('[data-ed-position]').textContent));
  await page.keyboard.press('Shift+F2');
  await until(()=>/^Ln 3,/.test(document.querySelector('[data-ed-position]').textContent));
  await page.keyboard.press('Control+F2');
  await until(()=>[...document.querySelectorAll('[data-ed-gutter-lines] .is-bookmarked')].map(r=>r.textContent).join()==='30,33,36');
  await page.keyboard.press('Control+F2');
  await until(()=>[...document.querySelectorAll('[data-ed-gutter-lines] .is-bookmarked')].map(r=>r.textContent).join()==='3,30,33,36');
  // A second Mark All adds its marks; Purge for each search clears the others.
  await query('lab-switch');await win().locator('[data-ed-find-action="mark"]').click();
  await messageIs(/^Mark: 1 match\.$/);
  await until(()=>document.querySelectorAll('[data-ed-marks] mark').length===5);
  await theme('dark');await shoot('editor-find-window-mark-dark.png');
  await theme('light');await shoot('editor-find-window-mark-light.png');await theme('dark');
  // Copy Marked Text: the text of every mark.
  await win().locator('[data-ed-find-action="copy-marks"]').click();
  await messageIs(/^Copied the text of 5 marks\.$/);
  assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),'lab-switch\n192.0.2.1\n192.0.2.1\n192.0.2.1\n192.0.2.1','in document order (lab-switch is on line 1); 192.0.2.1 also marks the start of 192.0.2.10');
  await option('purge').check();
  await query('vlan 120');await win().locator('[data-ed-find-action="mark"]').click();
  await until(()=>document.querySelectorAll('[data-ed-marks] mark').length===1);
  assert.deepEqual(await page.locator('[data-ed-gutter-lines] .is-bookmarked').evaluateAll(list=>list.map(r=>r.textContent)),['37'],'Purge with Bookmark line clears the earlier bookmarks');
  await win().locator('[data-ed-find-action="clear-marks"]').click();
  await until(()=>!document.querySelector('[data-ed-marks] mark'));
  await option('purge').uncheck();
  // A mark made In selection follows its text: a long line typed above it
  // moves the range, and the same "vlan" stays marked and is copied.
  await page.evaluate(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]'),start=ta.value.indexOf('interface ge-0/0/1\n'),end=ta.value.indexOf('interface ge-0/0/2\n');ta.focus();ta.setSelectionRange(start,end);document.dispatchEvent(new Event('selectionchange'));});
  await until(()=>!document.querySelector('[data-ed-find-option="inSelection"]').disabled);
  await option('inSelection').check();await option('bookmark').uncheck();await option('purge').check();
  await query('vlan');await win().locator('[data-ed-find-action="mark"]').click();
  await messageIs(/^Mark: 1 match in the selection\.$/);
  await option('inSelection').uncheck();
  await text().focus();await page.keyboard.press('Control+Home');await page.keyboard.insertText('x'.repeat(90)+'\n');
  await until(()=>{const marks=[...document.querySelectorAll('[data-ed-marks] mark')];return marks.length===1&&marks[0].textContent==='vlan';});
  await win().locator('[data-ed-find-action="copy-marks"]').click();
  await messageIs(/^Copied the text of 1 mark\.$/);
  assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),'vlan');
  await text().focus();await page.keyboard.press('Control+z');
  await until(()=>!NetworkToolkitFileEditor.active.dirty);
  await win().locator('[data-ed-find-action="clear-marks"]').click();await option('purge').uncheck();
  // Marks on a disk-backed document: only the part in view is searched.
  await activate('hits.txt');await settled();await text().focus();await page.keyboard.press('Control+Home');
  await query('name vlan-1');await option('bookmark').uncheck();
  await win().locator('[data-ed-find-action="mark"]').click();
  await messageIs(/^Mark: 11,111 matches\.$/);
  await until(()=>document.querySelectorAll('.nt-editor-marks mark').length>=10);
  assert.equal(await page.evaluate(()=>[...document.querySelectorAll('.nt-editor-marks mark')].every(m=>m.textContent==='name vlan-1')),true);
  await shoot('editor-find-marks-disk-dark.png');await theme('light');await shoot('editor-find-marks-disk-light.png');await theme('dark');
  await win().locator('[data-ed-find-action="clear-marks"]').click();
  step('mark done');

  // ---- 5. In selection ----
  step('in selection');
  await activate('config.txt');await settled();
  await page.evaluate(()=>{const ta=document.querySelector('.nt-editor-code textarea[data-ed-text]'),start=ta.value.indexOf('interface ge-0/0/2\n'),end=ta.value.indexOf('interface ge-0/0/5\n');ta.focus();ta.setSelectionRange(start,end);ta.dispatchEvent(new Event('select',{bubbles:true}));document.dispatchEvent(new Event('selectionchange'));});
  await showTab('replace');
  await until(()=>!document.querySelector('[data-ed-find-option="inSelection"]').disabled);
  await option('inSelection').check();
  await query('uplink');await page.locator('[data-ed-replacement]').fill('trunk');
  await win().locator('[data-ed="replaceAll"]').click();
  await messageIs(/^Replace All: 3 occurrences were replaced in the selection\.$/);
  const content=(await active()).content;
  assert.equal(content.split('trunk').length-1,3);assert(content.includes('description uplink to 192.0.2.1\n'),'before the range is unchanged');assert(content.includes('description uplink to 192.0.2.5\n'),'after the range is unchanged');
  await text().focus();await page.keyboard.press('Control+z');
  await until(()=>!NetworkToolkitFileEditor.active.content.includes('trunk'));
  // On a 5 MB disk-backed document: Count and Replace All of a range.
  await open('large.txt');
  assert.equal((await active()).disk,true);
  const range=await page.evaluate(async()=>{const d=NetworkToolkitFileEditor.active;const w=await NetworkToolkitEditorWindows.buffer(d.uid,'window',{start:0,limit:4000});const start=w.text.indexOf('host-000010'),end=w.text.indexOf('host-000020');return {start,end};});
  await page.evaluate(({start,end})=>NetworkToolkitFileEditor.selectRange(start,end),range);
  await until(()=>!document.querySelector('[data-ed-find-option="inSelection"]').disabled);
  await option('inSelection').check();
  await query('example.com');
  await win().locator('[data-ed-find-tab="find"]').click();
  await win().locator('[data-ed-find-action="count"]').click();
  await messageIs(/^Count: 10 matches in the selection\.$/);
  // Find Next of a disk-backed document, In selection: past the range's end it wraps to its start.
  await win().locator('[data-ed="next"]').click();
  await messageIs(/^Find: Found the first occurrence from the top\. The end of the selection has been reached\.$/);
  await until(()=>/^Ln 11, Col \d+ · 11 selected/.test(document.querySelector('[data-ed-position]').textContent));
  await win().locator('[data-ed="next"]').click();
  await messageIs(/^Match on line 12\.$/);
  await win().locator('[data-ed-find-tab="replace"]').click();
  await page.locator('[data-ed-replacement]').fill('example.net');
  await win().locator('[data-ed="replaceAll"]').click();
  await messageIs(/^Replace All: 10 occurrences were replaced in the selection\.$/);
  await settled();
  const after=await page.evaluate(async()=>{const d=NetworkToolkitFileEditor.active;return (await NetworkToolkitEditorWindows.buffer(d.uid,'window',{start:0,limit:4000})).text;});
  assert.equal(after.split('example.net').length-1,10);assert(after.includes('host-000009.example.com'));assert(after.includes('host-000020.example.com'));
  await text().focus();await page.keyboard.press('Control+z');
  await until(()=>!NetworkToolkitFileEditor.active.dirty,null,60000);
  await option('inSelection').uncheck().catch(()=>{});
  step('in selection done');

  // ---- The Document Map beside marks and the Search results panel ----
  // The map takes the right edge of the document area, the results panel
  // the bottom of the editor: neither covers the other or the text, and
  // the marks stay on their text however wide the text area becomes.
  step('document map');
  await activate('config.txt');await settled();
  await showTab('mark');await option('bookmark').uncheck();await option('purge').check();
  await query('uplink');await win().locator('[data-ed-find-action="mark"]').click();
  await messageIs(/^Mark: 12 matches\.$/);
  await showTab('find');await win().locator('[data-ed-find-action="all-current"]').click();
  await messageIs(/^Find All: 12 hits in 1 file of 1 searched\.$/);
  await page.evaluate(()=>NetworkToolkitFileEditor.setDocumentMap(true));
  await until(()=>{const m=document.querySelector('#networkToolkitFileEditor [data-ed-map]');return m&&!m.hidden&&NetworkToolkitFileEditor.documentMap.layout;});
  const layout=()=>page.evaluate(()=>{
   const box=el=>{const r=el.getBoundingClientRect();return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height};};
   const editor=document.querySelector('#networkToolkitFileEditor'),ta=editor.querySelector('.nt-editor-code textarea[data-ed-text]'),layer=editor.querySelector('[data-ed-marks]');
   const marks=[...layer.querySelectorAll('mark')];
   return {map:box(editor.querySelector('[data-ed-map]')),panel:box(editor.querySelector('[data-ed-find-results]')),panelShown:!editor.querySelector('[data-ed-find-results]').hidden,
    text:box(ta),client:ta.clientWidth,layer:box(layer),layerWidth:layer.style.width,wrapped:layer.classList.contains('is-wrapped'),layerShown:!layer.hidden,
    marks:marks.length,markText:[...new Set(marks.map(m=>m.textContent))],markRight:Math.max(...marks.map(m=>m.getBoundingClientRect().right)),
    workspace:box(editor.querySelector('.nt-editor-workspace'))};
  });
  const check=(g,label)=>{
   assert.equal(g.panelShown,true,label+': the results panel shows');
   assert(Math.abs(g.map.right-g.workspace.right)<=2,label+': the map is at the right edge '+JSON.stringify(g));
   assert(g.map.bottom<=g.panel.top+1,label+': the results panel is below the map '+JSON.stringify(g));
   assert(g.panel.left<=g.text.left+1&&g.panel.right>=g.map.right-1,label+': the results panel spans the text and the map');
   assert(g.text.right<=g.map.left+1,label+': the map does not cover the text');
   assert.equal(g.layerShown,true,label+': the marks show');assert.equal(g.marks,12,label);assert.deepEqual(g.markText,['uplink'],label);
   assert(g.markRight<=g.map.left+1,label+': no mark lies under the map');
   assert(Math.abs(g.layer.left-g.text.left)<=2,label+': the marks start where the text does '+JSON.stringify(g));
   if(g.wrapped)assert.equal(g.layerWidth,g.client+'px',label+': wrapped marks are as wide as the text area');
  };
  const mapOn=await layout();check(mapOn,'map on');
  await shoot('editor-find-window-document-map-dark.png');await theme('light');await shoot('editor-find-window-document-map-light.png');await theme('dark');
  check(await layout(),'map on, light');
  await page.locator('#networkToolkitFileEditor [data-ed-wrap]').check();
  await until(()=>document.querySelector('#networkToolkitFileEditor [data-ed-marks]').classList.contains('is-wrapped'));
  check(await layout(),'map on, word wrap');
  // Hiding the results panel gives the map the height back.
  await page.locator('[data-ed-find-results-toggle]').click();
  await until(()=>document.querySelector('#networkToolkitFileEditor [data-ed-find-results]').hidden);
  await page.waitForTimeout(200);
  const taller=await page.evaluate(()=>document.querySelector('#networkToolkitFileEditor [data-ed-map]').getBoundingClientRect().height);
  assert(taller>mapOn.map.height+20,'the map grows when the results panel hides: '+taller+' after '+mapOn.map.height);
  await page.locator('[data-ed-find-results-toggle]').click();
  await until(()=>!document.querySelector('#networkToolkitFileEditor [data-ed-find-results]').hidden);
  await page.locator('#networkToolkitFileEditor [data-ed-wrap]').uncheck();
  await page.evaluate(()=>NetworkToolkitFileEditor.setDocumentMap(false));
  await until(()=>document.querySelector('#networkToolkitFileEditor [data-ed-map]').hidden);
  // The marks follow the text area back to its full width.
  await until(()=>{const ta=document.querySelector('#networkToolkitFileEditor .nt-editor-code textarea[data-ed-text]'),m=document.querySelector('#networkToolkitFileEditor [data-ed-marks] mark');return m&&m.getBoundingClientRect().left>=ta.getBoundingClientRect().left;});
  await showTab('mark');await win().locator('[data-ed-find-action="clear-marks"]').click();await option('purge').uncheck();await showTab('find');
  await results().locator('.nt-find-row').first().click({button:'right'});
  await page.locator('.nt-find-results-menu [data-ed-find-menu="clear"]').click();
  step('document map done');

  // ---- 6. Transparency across a reload; a 640×400 window ----
  step('transparency');
  await win().locator('[data-ed-find-transparency]').check();
  await win().locator('[data-ed-find-transparency-mode][value="always"]').check();
  await win().locator('[data-ed-find-opacity]').fill('55');
  await until(()=>document.querySelector('[data-ed-find]').style.opacity==='0.55');
  await page.waitForTimeout(400);
  await load();
  await open('config.txt');
  await text().focus();await page.keyboard.press('Control+f');await win().waitFor({state:'visible'});
  assert.equal(await win().locator('[data-ed-find-transparency]').isChecked(),true,'Transparency is kept');
  assert.equal(await win().locator('[data-ed-find-transparency-mode][value="always"]').isChecked(),true);
  assert.equal(await win().locator('[data-ed-find-opacity]').inputValue(),'55');
  assert.equal(await win().evaluate(el=>el.style.opacity),'0.55');
  // The search history is the engine's, kept while it runs.
  await win().locator('[data-ed-query] ~ .combobox-toggle').click();
  await page.locator('#comboboxPopup').waitFor({state:'visible'});
  assert((await page.locator('#comboboxPopup [role="option"]').evaluateAll(list=>list.map(o=>o.textContent))).includes('uplink'),'the history is still there after a reload');
  await page.keyboard.press('Escape');
  await win().locator('[data-ed-find-transparency]').uncheck();
  for(const [width,height] of [[640,400]]){
   await page.setViewportSize({width,height});await page.waitForTimeout(150);
   for(const tab of ['find','replace','mark']){
    await win().locator('[data-ed-find-tab="'+tab+'"]').click();
    const rect=await win().boundingBox();
    assert(rect.x>=0&&rect.y>=0&&rect.x+rect.width<=width&&rect.y+rect.height<=height,tab+' fits '+width+'×'+height+': '+JSON.stringify(rect));
    const last=win().locator('.nt-find-buttons .btn:visible').last();await last.scrollIntoViewIfNeeded();assert.equal(await last.isVisible(),true);
    await shoot('editor-find-window-'+tab+'-640x400-dark.png');
   }
  }
  step('transparency done');
  assert.deepEqual(errors,[]);
  console.log('FIND WINDOW PASS');
 }catch(error){
  if(page)await shoot('editor-find-window-failure.png').catch(()=>{});
  console.error(error);process.exitCode=1;
 }finally{await browser.close();}
})();
