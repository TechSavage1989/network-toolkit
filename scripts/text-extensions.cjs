'use strict';
// The file types Setup offers Network Toolkit Text Editor for live in one
// place, scripts/installer/text-extensions.nsh. This module reads that list
// for scripts/text-extensions.test.cjs and writes the list shown in
// docs/guide/editor.md:
//     node scripts/text-extensions.cjs --write
const fs=require('node:fs'),path=require('node:path');
const ROOT=path.resolve(__dirname,'..'),LIST=path.join(ROOT,'scripts/installer/text-extensions.nsh'),GUIDE=path.join(ROOT,'docs/guide/editor.md');
const START='<!-- text-extensions:start (generated from scripts/installer/text-extensions.nsh; run node scripts/text-extensions.cjs --write) -->',END='<!-- text-extensions:end -->';

// [{title, extensions:['.txt', ...]}, ...] in the order of the list.
function groups(text=fs.readFileSync(LIST,'utf8')){
 const lines=text.split(/\r?\n/),from=lines.findIndex(line=>/^!macro TextExtensions ACTION\s*$/.test(line)),to=lines.findIndex((line,index)=>index>from&&/^!macroend\s*$/.test(line));
 if(from<0||to<0)throw Error('text-extensions.nsh must define the macro TextExtensions.');
 const result=[];
 for(const line of lines.slice(from+1,to)){
  const title=/^\s*; == (.+) ==\s*$/.exec(line),entry=/^\s*!insertmacro \$\{ACTION\} "(\.[^"]+)"\s*$/.exec(line);
  if(title)result.push({title:title[1],extensions:[]});
  else if(entry){if(!result.length)throw Error('Start the list with a "; == Title ==" line.');result.at(-1).extensions.push(entry[1]);}
  else if(line.trim())throw Error('Unexpected line in the TextExtensions macro: '+line.trim());
 }
 return result;
}
const extensions=list=>(list||groups()).flatMap(group=>group.extensions);
const render=list=>(list||groups()).map(group=>'- **'+group.title+':** '+group.extensions.map(ext=>'`'+ext+'`').join(' ')).join('\n');
const block=list=>START+'\n'+render(list)+'\n'+END;
// The generated block as it stands in the guide, or null when the markers are missing.
function guideBlock(text=fs.readFileSync(GUIDE,'utf8')){
 const from=text.indexOf(START),to=text.indexOf(END);
 return from<0||to<from?null:text.slice(from,to+END.length);
}
if(require.main===module){
 if(process.argv.includes('--write')){
  const text=fs.readFileSync(GUIDE,'utf8'),current=guideBlock(text);
  if(current===null)throw Error('docs/guide/editor.md has no text-extensions markers.');
  fs.writeFileSync(GUIDE,text.replace(current,block()));
 }else console.log(render());
}
module.exports={ROOT,LIST,GUIDE,START,END,groups,extensions,render,block,guideBlock};
