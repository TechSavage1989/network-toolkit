'use strict';
// What made large files stall while scrolling (owner report on v1.21.61) must
// not come back: page-wide :has() rules, the Clear-button script rewriting
// every field on each scroll, and the wrap measure's rows matched against the
// whole page's style sheets.
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const root=path.resolve(__dirname,'..'),read=name=>fs.readFileSync(path.join(root,name),'utf8');

test('no style rule puts :has() on the page root',()=>{
 const files=['web/index.html',...fs.readdirSync(path.join(root,'web/assets')).filter(name=>name.endsWith('.css')).map(name=>'web/assets/'+name)];
 for(const file of files){
  // A :has() on <html> or <body> is re-checked for every change anywhere in
  // the page and restyles whatever its rules could match. Keep such state as
  // a class on <body> (file-editor.js syncEditorPageState).
  const found=read(file).match(/(?:^|[\s,{};>+~])(?:html|body|:root)(?:[.#\[][^\s,{>+~:]*)*:has\(/g);
  assert.equal(found,null,file+' has a :has() rule on the page root: '+found);
 }
});

test('the Clear-button script writes only what changed and ignores the editor\'s own scrolling',()=>{
 const source=read('web/assets/input-clear.js');
 assert(!/button\.hidden=!shown;/.test(source.replace('if(button.hidden===shown)button.hidden=!shown;','')),'the button\'s hidden state is written only when it changes');
 assert(source.includes("if(button.getAttribute('aria-label')!==title)button.setAttribute('aria-label',title)"),'the label is written only when it changes');
 assert(!/\['input','change','focusin','focusout','scroll'\]/.test(source),'not every scroll in the page lays every field out again');
 assert(source.includes("[data-ed-gutter-lines]"),'the editor\'s line numbers are not watched');
});

test('the wrap measure keeps its rows out of the page\'s style sheets',()=>{
 const source=read('web/assets/editor-disk-view.js'),body=source.slice(source.indexOf('function createMeasure('),source.indexOf('function layoutRows('));
 assert(body.includes('attachShadow'),'the measured rows live in a shadow tree');
 assert(/measure\.remove = \(\) => host\.remove\(\)/.test(body),'removing the measure removes its host');
});

test('a dropped file with more lines than a page-held document may have goes to the engine',async()=>{
 const source=read('web/assets/file-editor.js');
 const body=source.match(/async function moreLinesThan\(file,limit\)\{[\s\S]*?\n\}/);
 assert(body,'moreLinesThan is in file-editor.js');
 const moreLinesThan=new Function('return '+body[0])();
 const file=text=>{const bytes=typeof text==='string'?new TextEncoder().encode(text):text;return {size:bytes.length,arrayBuffer:async()=>bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.length)};};
 assert.equal(await moreLinesThan(file('a\n'.repeat(4000)),4000),false,'4,000 lines stay in the page');
 assert.equal(await moreLinesThan(file('a\n'.repeat(4001)),4000),true,'4,001 lines go to the engine');
 assert.equal(await moreLinesThan(file('a\r\n'.repeat(4001)),4000),true,'CRLF lines count once each');
 assert.equal(await moreLinesThan(file('a\r\n'.repeat(3000)),4000),false);
 assert.equal(await moreLinesThan(file('a\r'.repeat(4001)),4000),true,'a file that ends lines with CR alone');
 assert.equal(await moreLinesThan(file('x'.repeat(3000)),4000),false,'too small to hold that many lines: not read');
 // The rule is applied before a small dropped file is read into the page.
 assert(/if\(diskAvailable\(\)&&\(file\.size>memoryEditorUnits\|\|await moreLinesThan\(file,memoryEditorLines\)\)\)\{/.test(source),'importFiles applies the line rule');
});
