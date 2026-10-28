'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),crypto=require('node:crypto'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs'),assets=path.resolve(__dirname,'../web/assets');
const read=name=>fs.readFileSync(path.join(assets,name),'utf8');
function load(context,names){vm.createContext(context);for(const name of names)vm.runInContext(read(name),context,{filename:name});return context;}
const context=load({},['editor-vendor/languages.js','network-config-language.js','editor-languages.js']),languages=context.NTKEditorLanguages;
function worker(){let reply;const c={console,TextEncoder,TextDecoder};c.window=c;c.self=c;c.postMessage=value=>reply=value;c.importScripts=(...names)=>names.forEach(name=>vm.runInContext(read(name),c,{filename:name}));load(c,['editor-language-worker.js']);return {c,async request(action,language,text,options={}){reply=null;await c.onmessage({data:{id:1,action,language,text,options}});assert(reply);return reply;}};}
test('bundled language assets match their provenance and every offered language has a grammar',()=>{
 const source=JSON.parse(read('editor-vendor/SOURCE.json'));assert(source.grammars>=290);
 for(const [name,digest]of Object.entries(source.files))assert.equal(crypto.createHash('sha256').update(fs.readFileSync(path.join(assets,'editor-vendor',name))).digest('hex'),digest,name);
 const w=worker();for(const row of languages.languages)assert(row.id==='text'||w.c.Prism.languages[row.id],row.id+' has no grammar');assert.equal(languages.languages.length,new Set(languages.languages.map(row=>row.id)).size);
});
test('file associations, dotfiles, names and script headers distinguish language families',()=>{
 const cases=[['build.PS1','','powershell'],['module.psm1','','powershell'],['data.psd1','','powershell'],['test.cmd','','batch'],['test.bat','','batch'],['main.c','','c'],['header.h','class App {};','cpp'],['App.hpp','','cpp'],['App.cs','','csharp'],['App.java','','java'],['App.kt','','kotlin'],['data.json','','json'],['data.jsonc','','jsonc'],['data.json5','','json5'],['data.ndjson','','jsonlines'],['index.html','','html'],['image.svg','','xml'],['App.csproj','','xml'],['README.md','','markdown'],['doc.rst','','rest'],['file.toml','','toml'],['script.py','','python'],['style.scss','','scss'],['thing.vbs','','visual-basic'],['C:\\folder\\Dockerfile.dev','','docker'],['Makefile','','makefile'],['CMakeLists.txt','','cmake'],['go.mod','','go-module'],['.gitignore','','ignore'],['.editorconfig','','editorconfig'],['.env.local','','bash'],['.zshrc','','bash'],['nginx.conf','','nginx'],['settings.config','<?xml version="1.0"?><root/>','xml'],['main.m','@interface Hello','objectivec'],['plot.m','x = 2;','matlab'],['tool','#!/usr/bin/env -S python3 -u\nprint(1)','python'],['tool','#!/usr/bin/pwsh\nGet-Date','powershell'],['unknown','{"valid":true}','json'],['unknown','<html><body>hi</body></html>','html'],['notes.txt','#!/usr/bin/python','text'],['unknown.blob','ordinary prose','text']];
 for(const [file,text,id]of cases)assert.equal(languages.detect(file,text),id,file);assert.equal(languages.valid('unknown'),'text');
});
test('comments round-trip with real line/block syntax and JSON remains comment-free',()=>{
 const source='  first line\n\tsecond line\n';for(const id of ['powershell','batch','python','html','xml','css','sql','java','ini','visual-basic']){const value=languages.toggleComment(source,id);assert.notEqual(value,source,id);assert.equal(languages.toggleComment(value,id),source,id);}
 assert.equal(languages.toggleComment('"key": 1','json'),null);assert.match(languages.toggleComment('x','html'),/^<!-- x -->$/);assert.match(languages.toggleComment('x','batch'),/^REM x$/);
});
test('less common language extensions and server markup remain automatically recognizable',()=>{
 for(const [name,id]of [['program.abap','abap'],['contract.sol','solidity'],['page.aspx','aspnet'],['component.cshtml','cshtml'],['file.wgsl','wgsl'],['source.wren','wren'],['file.psrc','powershell']])assert.equal(languages.detect(name),id,name);
});
test('new line indentation recognizes code, Python, markup and matching closing braces',()=>{
 assert.equal(languages.newline('if ($ok) {',10,'powershell','  ').text,'\n  ');
 assert.equal(languages.newline('  if ready:',11,'python','    ').text,'\n      ');
 const insert=languages.newline('{}',1,'c','\t');assert.equal(insert.text,'\n\t\n');assert.equal(insert.back,1);
 assert.equal(languages.newline('<section>',9,'html','  ').text,'\n  ');assert.equal(languages.newline('<br>',4,'html','  ').text,'\n');
});
test('real grammars escape HTML and classify PowerShell, Batch, C, Java and embedded markup',async()=>{
 const w=worker();const cases=[['powershell','<# block comment #>\nfunction Test-Foo { $env:TEMP; Write-Output "value # still a string" }'],['batch','@echo off\nREM comment\nset "name=%USERNAME%"\nif defined name echo !name!'],['c','#include <stdio.h>\n/* comment */\nint main(void) { puts("hi"); return 0; }'],['cpp','template<class T> struct Box { T value; };'],['csharp','public class App { string name = "value"; }'],['java','public class App { private int value = 2; }'],['html','<div id="test"><script>const n=2;</script><style>b {color:red}</style></div>'],['jsonc','{ // comment\n"value":true }'],['markdown','# Title\n**bold** and `code`'],['yaml','key: true\n# comment']];
 for(const [id,text]of cases){const result=await w.request('highlight',id,text);assert(!result.error,id+': '+result.error);assert.match(result.html,/class="nt-ed-/);assert(!/<(?:script|style|div)(?:\s|>)/i.test(result.html),id);}
 const hostile=await w.request('highlight','html','<img src=x onerror="globalThis.executed=true">');assert(hostile.html.includes('&lt;'));assert.equal(w.c.executed,undefined);
});
test('parser formatting preserves comments and values, rejects invalid syntax and never evaluates code',async()=>{
 const w=worker();for(const [id,text]of [['javascript','const value={a:1};'],['typescript','const value: number=2;'],['json','{"value":1,"value":2,"big":12345678901234567890}'],['jsonc','{ // preserve me\n"value":1,}'],['html','<div><b>hello</b><script>let a=1;</script></div>'],['css','b{color:red;margin:0}'],['markdown','# Heading\n\n* a\n* b'],['yaml','a: {x: 2, y: 3}'],['graphql','query{viewer{id name}}']]){const result=await w.request('format',id,text,{indent:2});assert(!result.error,id+': '+result.error);assert(result.text.endsWith('\n'),id);if(id==='json'){assert(result.text.includes('12345678901234567890'));assert.equal((result.text.match(/"value"/g)||[]).length,2);}if(id==='jsonc')assert(result.text.includes('// preserve me'));}
 assert((await w.request('format','json','{"broken": }')).error);assert((await w.request('format','text','ordinary text')).error);
 const safe=await w.request('format','javascript','globalThis.executed=true;');assert(!safe.error);assert.equal(w.c.executed,undefined);
});
test('brace indentation preserves PowerShell here-strings and C raw strings/macros byte for byte',async()=>{
 const w=worker(),ps='function Test-Foo {\n$x = @"\n   { keep literal spacing }\n"@\nWrite-Output $x\n}';
 const result=await w.request('format','powershell',ps,{indent:2});assert(!result.error);assert.equal(result.mode,'indent');assert(result.text.includes('\n   { keep literal spacing }\n"@\n'));assert(result.text.includes('\n  Write-Output $x\n'));
 const cpp='#define VALUE(x) \\\n   { x; }\nint main() {\nauto text = R"raw(\n   { stay }\n)raw";\nreturn 0;\n}';
 const c=await w.request('format','cpp',cpp,{indent:2});assert(!c.error,c.error);assert(c.text.includes('#define VALUE(x) \\\n   { x; }'));assert(c.text.includes('R"raw(\n   { stay }\n)raw"'));assert(c.text.includes('\n  return 0;'));
 const bad=await w.request('format','c','int main() {\nreturn 0;');assert(bad.error);
});
function editorHarness(){const document=makeDocument('');document.body=document;const create=document.createElement;document.createElement=tag=>{const el=create(tag);el.style.setProperty=(k,v)=>el.style[k]=v;el.style.removeProperty=k=>delete el.style[k];return el;};let complete;
 const c={document,console,Map,Set,TextDecoder,TextEncoder,Uint8Array,DataView,Blob,URL,setTimeout,clearTimeout,state:{},saveState(){},confirm:()=>true,esc:s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),addEventListener(){},NTKEditorLanguageService:{highlight:async text=>({html:text}),format:()=>new Promise(resolve=>complete=resolve),dispose(){}}};
 load(c,['editor-vendor/languages.js','network-config-language.js','editor-languages.js','file-editor.js']);const api=c.NetworkToolkitFileEditor;
 const root=()=>document.getElementById('networkToolkitFileEditor');const ta=()=>document.querySelector('[data-ed-text]');
 return {api,c,document,ta,root,complete:result=>complete(result),async open(id,text){await api.open({id,path:id,create:true,initialContent:text,read:async()=>({content:text}),write:async()=>({})});},click:action=>root().emit('click',root().querySelector('[data-ed="'+action+'"]')),settle:()=>new Promise(resolve=>setImmediate(resolve))};
}
test('formatting cannot overwrite a changed, closed or switched editor document',async()=>{
 const h=editorHarness();await h.open('one.json','{"x":1}');h.click('format');h.ta().value='{"x":2}';h.root().emit('input',h.ta());h.complete({text:'{"x":1}\n',mode:'format'});await h.settle();assert.equal(h.api.active.content,'{"x":2}');
 h.click('format');await h.open('other.json','{"other":true}');h.complete({text:'overwritten',mode:'format'});await h.settle();assert.equal(h.api.active.content,'{"other":true}');assert.equal(h.api.documents.get('one.json').content,'{"x":2}');
 h.click('format');const closing=h.api.closeAll();h.root().emit('click',h.root().querySelector('[data-ed-dialog-action="accept"]'));await closing;h.complete({text:'overwritten',mode:'format'});await h.settle();assert.equal(h.api.documents.size,0);
});
test('manual syntax override, indentation selection and formatting undo preserve file metadata',async()=>{
 const h=editorHarness();await h.open('script.ps1','if ($ok) {\nWrite-Output 1\n}');const select=h.document.querySelector('[data-ed-language-select]');assert.equal(select.value,'');select.value='java';h.root().emit('change',select);assert.equal(h.api.active.language,'java');select.value='';h.root().emit('change',select);assert.equal(h.api.active.language,'');
 h.ta().setSelectionRange(0,h.ta().value.length);h.root().emit('keydown',h.ta(),{key:'Tab'});assert(h.api.active.content.startsWith('    if'));h.root().emit('keydown',h.ta(),{key:'Tab',shiftKey:true});assert(h.api.active.content.startsWith('if'));
 const before=h.api.active.content;h.click('format');h.complete({text:before.replace('Write-Output','    Write-Output'),mode:'indent'});await h.settle();assert(h.api.active.dirty);h.click('undo');assert.equal(h.api.active.content,before);assert.equal(h.api.active.encoding,'UTF-8');assert.equal(h.api.active.eol,'LF');h.api.closeAll();
});
test('Save As immediately updates automatic language recognition without modifying the saved source',async()=>{
 const h=editorHarness(),source='Write-Output "hello"';await h.open('Untitled.txt',source);h.c.prompt=()=>{throw Error('Native dialogs are disabled');};h.click('saveAs');h.root().querySelector('[data-ed-dialog-input]').value='/tmp/example.ps1';h.root().emit('click',h.root().querySelector('[data-ed-dialog-action="accept"]'));await h.settle();assert.equal(h.api.active.path,'/tmp/example.ps1');assert.equal(h.api.active.language,'');assert.equal(h.api.active.content,source);assert.equal(h.api.active.dirty,false);assert.match(h.document.querySelector('[data-ed-auto-language]').textContent,/PowerShell/);h.api.closeAll();
});
