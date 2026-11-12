'use strict';
// App settings > Updates and > Editor of the installed program: the notice
// that Windows lists the Text Editor for only some file types, and its steps
// to run this version's verified Setup (web/assets/windows-integration.js;
// the engine side is text_editor_types.go and full_install.go). The page in a
// browser is scripts/windows-integration.browser.cjs.
const {test}=require('node:test'),assert=require('node:assert/strict');
const {installHTML,typesHTML,editorHTML}=require('../web/assets/windows-integration.js');
const version='1.21.51',name='Network_Toolkit_'+version+'_Windows_x64_Setup.exe';
const text=html=>html.replace(/<[^>]+>/g,' ').replace(/&#39;/g,"'").replace(/\s+/g,' ').trim();
const install=(extra={})=>({supported:true,portable:false,version,installed:true,installedExists:true,busy:false,started:false,...extra});
const types=(extra={})=>({total:108,missing:103,setupVersion:'1.21.20',uninstallerRecords:false,setupNeeded:true,...extra});
const view=(data,extra={})=>({busy:false,error:'',message:'',confirmDownload:false,confirmStart:false,data,...extra});
const editor={state:'setup',thisIsInstalled:true,installed:true,message:'This is the installed program. Setup registered its Text Editor with Windows; run Setup again or uninstall to change that.'};

test('the installed program with missing file types shows the plain notice and one button',()=>{
 const html=typesHTML(view({supported:true,textEditor:editor,install:install(),types:types()}),true);
 assert.match(text(html),new RegExp('Windows lists the Text Editor for only some file types\\. Run Setup '+version.replace(/\./g,'\\.')+' once to update Windows integration \\(your settings and data stay\\)\\.'));
 assert.match(html,/<h4>Windows integration<\/h4>/);
 assert.match(html,/data-windows-install="find"[^>]*>Run Setup 1\.21\.51…</);
 assert.equal((html.match(/data-windows-install=/g)||[]).length,1,'nothing but the one button');
 // In App settings > Editor, under the section's own heading.
 assert.doesNotMatch(typesHTML(view({supported:true,install:install(),types:types()}),false),/<h4>/);
 // The portable copy's section is unchanged: it has its own install steps.
 assert.equal(installHTML(view({supported:true,install:install(),types:types()})),'','the installed program has no "Install the full version"');
});

test('nothing is shown when every type is registered, for a portable copy or without the engine\'s answer',()=>{
 for(const data of [{supported:true,install:install(),types:types({missing:0,setupNeeded:false})},{supported:true,install:install({portable:true,installed:false}),types:types()},{supported:true,install:install()},null])
  assert.equal(typesHTML(view(data),true),'',JSON.stringify(data));
 // Types added after an update are reported, without a notice.
 const added=typesHTML(view({supported:true,install:install(),types:types({missing:0,setupNeeded:false,added:103,uninstallerRecords:true})}),false);
 assert.match(text(added),/After the update, Windows also offers the Text Editor for 103 more file types\./);
 assert.doesNotMatch(added,/data-windows-install=/);
});

test('download and start each need their own confirmation, and the start says the app closes',()=>{
 const found={version,name,size:12582912,beta:false,fallback:false,running:version};
 let html=typesHTML(view({supported:true,install:install({found}),types:types()},{confirmDownload:true}),true);
 assert.match(html,/data-windows-install-confirm="download"/);
 assert.match(text(html),/check it against the release's SHA256SUMS\? Nothing is installed yet/);
 assert.match(html,/data-windows-install="download-confirm" data-version="1\.21\.51"/);
 assert.doesNotMatch(html,/start-confirm/);
 const download={version,name,size:12582912,sha256:'b'.repeat(64)};
 html=typesHTML(view({supported:true,install:install({found,download}),types:types()},{confirmStart:true}),true);
 assert.match(html,/data-windows-install-confirm="start"/);
 assert.match(text(html),/Network Toolkit then closes so that Setup can update it: you are asked about unsaved documents first, and your settings and saved data stay\./);
 assert.match(html,/data-windows-install="start-confirm" data-version="1\.21\.51"[^>]*>Start Setup and close Network Toolkit</);
 html=typesHTML(view({supported:true,install:install({found,download,started:true}),types:types()}),true);
 assert.match(text(html),/Setup is running in its own window\. Network Toolkit closes so that Setup can update it/);
 // Never an older release; a newer one only as a named fallback.
 html=typesHTML(view({supported:true,install:install({found:{...found,version:'1.21.40',fallback:true,newer:false}}),types:types()},{confirmDownload:true}),true);
 assert.match(html,/data-windows-install-none/);assert.doesNotMatch(html,/download-confirm/);
 html=typesHTML(view({supported:true,install:install({found:{...found,version:'1.21.52',fallback:true,newer:true}}),types:types()}),true);
 assert.match(text(html),/The stable release 1\.21\.52 is newer and has one/);
});

test('the Editor section of the installed program is unchanged',()=>{
 const html=editorHTML(view({supported:true,textEditor:editor,install:install(),types:types()}));
 assert.match(text(html),/This is the installed program\./);
 assert.doesNotMatch(html,/data-windows-editor=/,'nothing to register from the installed program');
});
