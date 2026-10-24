'use strict';
// Exercise the bundled DOM row factory and its real decoration lookup. Only
// element allocation and glyph measurement are replaced; no browser is needed.
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict');
const {Element}=require('./terminal-dom-fixture.cjs');
const source=fs.readFileSync(require.resolve('../web/assets/xterm.js'),'utf8');
const bootstrap='var s={};return(()=>{var e=s;';
assert.equal(source.split(bootstrap).length,2,'bundled xterm module bootstrap changed');
const context={module:{exports:{}},setTimeout,clearTimeout,queueMicrotask,performance,process,TextDecoder,TextEncoder,navigator:global.navigator};
context.exports=context.module.exports;
vm.runInNewContext(source.replace(bootstrap,'globalThis.xtermModule=i;'+bootstrap),context,{filename:'xterm-render-fixture.js'});
const position=source.indexOf('t.DomRendererRowFactory=void 0');
const moduleID=Number([...source.slice(0,position).matchAll(/(\d+):function\(e,t,i\)\{/g)].at(-1)[1]);
const {DomRendererRowFactory}=context.xtermModule(moduleID),{css}=context.xtermModule(4103);
module.exports=function rendererFor(term){
 const colors={foreground:css.toColor('#eeeeee'),background:css.toColor('#141414'),ansi:Array.from({length:256},()=>css.toColor('#ffffff'))};
 const factory=new DomRendererRowFactory({createElement:tag=>new Element(tag)},{getJoinedCharacters:()=>[]},term._core.optionsService,{isFocused:true},term._core.coreService,term._core._decorationService,{colors});
 return y=>{
  const line=term._core._bufferService.buffer.lines.get(y);
  if(!line)return [];
  return factory.createRow(line,y,false,'block','outline',0,false,8,{get:chars=>8*chars.length},-1,-1).map(span=>({text:span.textContent,color:span.style.color||span.getAttribute('style')?.match(/(?:^|;)color:([^;]+)/)?.[1],classes:span.className}));
 };
};
// Share the same bundled internal implementations with focused renderer and
// index regressions. These exports exist only in the Node test fixture.
module.exports.modules=context.xtermModule;
module.exports.SortedList=context.xtermModule(3087).SortedList;
const renderServicePosition=source.indexOf('t.RenderService=void 0');
const renderServiceID=Number([...source.slice(0,renderServicePosition).matchAll(/(\d+):function\(e,t,i\)\{/g)].at(-1)[1]);
module.exports.RenderService=context.xtermModule(renderServiceID).RenderService;
