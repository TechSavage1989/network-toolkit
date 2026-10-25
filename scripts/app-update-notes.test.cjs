'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const {html}=require('../web/assets/app-update-notes.js');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const render=notes=>makeDocument(html({currentVersion:'1.17.63',version:'1.17.64',notes}));

test('release card shows a title and current-to-next version with accessible labels',()=>{
  const doc=render('- Automatic checks at startup.');
  assert.equal(doc.querySelector('h4').textContent,"What's new");
  const labels=doc.querySelector('.nt-update-notes-versions').querySelectorAll('span');
  assert.equal(labels.find(node=>node.getAttribute('aria-label')==='Installed version').textContent,'1.17.63');
  assert.equal(labels.find(node=>node.getAttribute('aria-label')==='Release version').textContent,'1.17.64');
  assert.match(doc.querySelector('.nt-update-notes-versions').textContent,/1\.17\.63→1\.17\.64/);
  assert.equal(doc.querySelector('details'),null);
});

test('headings do not consume the four highlight slots, and full notes keep later changes',()=>{
  const doc=render('# Network Toolkit 1.17.64\n\n## Improvements\n- One\n* Two\n+ Three\n4. Four\n5) Five\n\n### Fixes ###\n- Six');
  assert.deepEqual(doc.querySelectorAll('li').map(node=>node.textContent),['One','Two','Three','Four']);
  const full=doc.querySelector('.nt-update-notes-full').textContent;
  assert.doesNotMatch(full,/Network Toolkit 1\.17\.64|• One|• Two|• Three|• Four/);assert.match(full,/Improvements/);assert.match(full,/• Five/);assert.match(full,/Fixes\n• Six/);
  assert.doesNotMatch(full,/^#/m);
});

test('all release content and version labels are escaped without rendering active elements',()=>{
  const attack='<img src=x onerror="alert(1)"><script>alert(2)</script><a href="javascript:alert(3)">click</a>';
  const output=html({currentVersion:attack,version:'"><svg onload=alert(4)>',notes:'# <iframe src="javascript:alert(5)">\n- '+attack+'\n- [Click](javascript:alert(6))\n- <style>body{display:none}</style>'});
  const doc=makeDocument(output);
  for(const tag of ['script','img','a','svg','iframe','style'])assert.equal(doc.querySelectorAll(tag).length,0,tag+' must not be created');
  assert.match(output,/&lt;img src=x onerror=&quot;alert\(1\)&quot;&gt;/);
  assert.match(output,/\[Click\]\(javascript:alert\(6\)\)/);
  assert.doesNotMatch(output,/<[^>]+\s(?:onerror|onload|href|src)=/i);
});

test('closing tags, entity payloads and quotes cannot escape into HTML or CSS',()=>{
  const output=html({version:"' & \" </span>",notes:'- </div></section><script>alert(1)</script>\n- &lt;img src=x onerror=alert(2)&gt;\n- \'quoted\' & "double"'});
  assert.match(output,/&#39; &amp; &quot; &lt;\/span&gt;/);
  assert.match(output,/&lt;\/div&gt;&lt;\/section&gt;&lt;script&gt;/);
  assert.match(output,/&amp;lt;img/);
  assert.equal(makeDocument(output).querySelectorAll('section').length,1);
});

test('simple emphasis is readable without losing punctuation or file-name underscores',()=>{
  const doc=render('- **Fix:** Network_Toolkit.exe keeps A_B and C_D.\n- `ping -t` uses 1.1.1.1; IPv6: ::1.\n- __New__ checks preserve “quotes”, (parentheses), 50%, and C:\\Tools\\app.exe.\n- ~~Old~~ → new.');
  const items=doc.querySelectorAll('li').map(node=>node.textContent);
  assert.equal(items[0],'Fix: Network_Toolkit.exe keeps A_B and C_D.');
  assert.equal(items[1],'ping -t uses 1.1.1.1; IPv6: ::1.');
  assert.equal(items[2],'New checks preserve “quotes”, (parentheses), 50%, and C:\\Tools\\app.exe.');
  assert.equal(items[3],'Old → new.');
});

test('plain paragraphs become concise highlights and support Windows newlines',()=>{
  const doc=render('## Changes\r\n\r\nFaster checks,\r\nwith clearer status.\r\n\r\nSettings are retained.');
  assert.deepEqual(doc.querySelectorAll('li').map(node=>node.textContent),['Faster checks, with clearer status.','Settings are retained.']);
  assert.equal(doc.querySelector('.nt-update-notes-full'),null);assert.doesNotMatch(doc.textContent,/\r/);
});

test('missing, blank and non-string notes use an honest fallback instead of invented changes',()=>{
  for(const notes of [undefined,null,'',' \n\t ',{},[],23]){
    const doc=render(notes);
    assert.match(doc.textContent,/Release notes are not available for this update\./);
    assert.equal(doc.querySelectorAll('li').length,0);assert.equal(doc.querySelectorAll('details').length,0);
  }
  assert.doesNotThrow(()=>html());
  assert.match(html(),/Upcoming release/);
  assert.doesNotMatch(html(),/Installed version/);
});

test('heading-only bodies remain reviewable without being advertised as change highlights',()=>{
  const doc=render('# Version 1.17.64');
  assert.equal(doc.querySelectorAll('li').length,0);
  assert.equal(doc.querySelector('.nt-update-notes-full').textContent,'Version 1.17.64');
});

test('fenced code stays inert and does not displace actual change highlights',()=>{
  const doc=render('## Changes\n```html\n<script>alert(1)</script>\n- not a change\n```\n- Real improvement');
  assert.deepEqual(doc.querySelectorAll('li').map(node=>node.textContent),['Real improvement']);
  assert.match(doc.querySelector('.nt-update-notes-full').textContent,/&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.equal(doc.querySelectorAll('script').length,0);
});

test('long notes have four bounded highlights and a scrollable, keyboard-accessible full view',()=>{
  const long='Word'.repeat(3000),doc=render(Array.from({length:30},(_,i)=>'- '+i+' '+long).join('\n'));
  const items=doc.querySelectorAll('li');assert.equal(items.length,4);
  for(const item of items){assert.ok(Array.from(item.textContent).length<=180);assert.ok(item.textContent.endsWith('…'));assert.match(item.getAttribute('style'),/overflow-wrap:anywhere/);}
  const full=doc.querySelector('.nt-update-notes-full');assert.ok(full.textContent.includes('29 '+long));
  assert.match(full.getAttribute('style'),/max-height:16rem;overflow:auto/);
  assert.match(full.getAttribute('style'),/white-space:pre-wrap/);
  assert.equal(full.getAttribute('role'),'region');assert.equal(full.getAttribute('tabindex'),'0');
  assert.match(doc.querySelector('.nt-update-notes').getAttribute('style'),/width:100%/);
  for(const selector of ['.nt-update-notes','.nt-update-notes-header','.nt-update-notes-versions','.nt-update-notes-full']){
    const style=doc.querySelector(selector).getAttribute('style');assert.match(style,/min-width:0/);assert.match(style,/max-width:100%/);
  }
  assert.match(doc.querySelector('.nt-update-notes-header').getAttribute('style'),/flex-wrap:wrap/);
  assert.match(doc.querySelector('.nt-update-notes-versions').getAttribute('style'),/flex-wrap:wrap/);
});

test('long Unicode highlights do not split surrogate pairs',()=>{
  const doc=render('- '+('🚀'.repeat(200)));
  const text=doc.querySelector('li').textContent;
  assert.equal(Array.from(text).length,178);assert.equal(text.endsWith('🚀…'),true);
  assert.doesNotMatch(text,/\ufffd/);
});

test('helper exposes the same contract in a browser without requiring Node',()=>{
  const context={window:{}};vm.createContext(context);
  vm.runInContext(fs.readFileSync(require.resolve('../web/assets/app-update-notes.js'),'utf8'),context);
  assert.equal(typeof context.window.NetworkToolkitUpdateNotes.html,'function');
  assert.match(context.window.NetworkToolkitUpdateNotes.html({version:'1.2.3',notes:'- Safe text'}),/Safe text/);
});

test('same-version notes have one badge and never imply an upgrade',()=>{
  const doc=makeDocument(html({currentVersion:'1.17.65',version:'1.17.65',notes:'- Improved status.'}));
  assert.equal(doc.querySelector('.nt-update-notes-versions').textContent,'1.17.65');
  assert.equal(doc.querySelectorAll('[aria-label="Installed version"]').length,0);assert.doesNotMatch(doc.textContent,/→/);
});
test('summary content appears once and disclosure contains only additional notes',()=>{
  const doc=makeDocument(html({currentVersion:'1.17.65',version:'1.17.66',notes:"Network Toolkit 1.17.66\n\nWhat's new\n- One\n- Two\n- Three\n- Four\n\nDownloads still require your action.\n\n## Compatibility\n- Five"}));
  for(const text of ['One','Two','Three','Four'])assert.equal(doc.textContent.split(text).length-1,1);
  assert.equal(doc.querySelector('summary').textContent,'Read more');
  assert.match(doc.querySelector('.nt-update-notes-full').textContent,/Downloads still require your action\.\nCompatibility\n• Five/);
  assert.doesNotMatch(doc.querySelector('.nt-update-notes-full').textContent,/What's new|Network Toolkit 1.17.66/);
});
test('four complete highlights without extra content have no redundant disclosure',()=>{
  for(const heading of ["What's new", "## What's new",'## Changes']){
    const doc=render('Network Toolkit 1.17.64\n\n'+heading+'\n- One\n- Two\n- Three\n- Four');
    assert.equal(doc.querySelector('details'),null);assert.equal(doc.querySelectorAll('li').length,4);
  }
});
test('deduplication ignores presentation markers and whitespace but retains extra historical headings',()=>{
  const doc=render('## Improvements\n- **One**\n- One\n- Two\n- Three\n- Four\n\n## Network Toolkit 1.17.63\nOlder compatibility information.');
  assert.deepEqual(doc.querySelectorAll('li').map(node=>node.textContent),['One','Two','Three','Four']);
  const extra=doc.querySelector('.nt-update-notes-full').textContent;assert.match(extra,/Network Toolkit 1.17.63\nOlder compatibility information/);assert.doesNotMatch(extra,/• One/);
});
