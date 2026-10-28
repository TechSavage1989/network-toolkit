/* Parsing runs outside the UI thread. All imports are bundled local assets.
 * No file is executed, evaluated or transmitted to a language service. */
'use strict';
importScripts('editor-vendor/languages.js','network-config-language.js','editor-languages.js','editor-vendor/prism.js');
Prism.languages.jsonc=Prism.languages.extend('json',{'comment':{pattern:/\/\/[^\n]*|\/\*[\s\S]*?(?:\*\/|$)/,greedy:true}});
Prism.languages.jsonlines=Prism.languages.json;
Prism.languages.vue=Prism.languages.markup;
Prism.languages.network={}; // Rendered by the position-aware local lexer below.
Prism.languages.registry={comment:/^[\t ]*;.*/m,keyword:/^Windows Registry Editor Version[^\n]*/m,property:/^\[[^\n]*\]/m,string:/"(?:\\.|[^"\\])*"/,number:/\b(?:dword|hex)(?:\([a-f\d]+\))?:[a-f\d,\\\s]+/i};
for(const row of NTKNetworkConfig.languages)Prism.languages[row.id]={};
const escape=s=>String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
const tokenKind={comment:'comment',prolog:'comment',doctype:'comment',cdata:'string',string:'string',char:'string','template-string':'string','raw-string':'string',regex:'string','attr-value':'string',url:'string',keyword:'keyword',boolean:'keyword',builtin:'keyword',tag:'keyword',important:'keyword',number:'number',property:'property','attr-name':'property',variable:'variable',parameter:'property',function:'function','class-name':'type',namespace:'type',operator:'operator',punctuation:'punctuation',selector:'property',constant:'number',symbol:'number',deleted:'comment',inserted:'string',title:'keyword'};
function render(tokens){return tokens.map(token=>{if(typeof token==='string')return escape(token);const inner=Array.isArray(token.content)?render(token.content):escape(token.content),kind=tokenKind[token.type]||[token.alias||[]].flat().map(alias=>tokenKind[alias]).find(Boolean);return kind?'<span class="nt-ed-'+kind+'">'+inner+'</span>':inner;}).join('');}
function tokenize(text,id){const grammar=Prism.languages[id];return grammar?Prism.tokenize(text,grammar):[text];}
function indentCode(text,id,width,useTabs){
 const tokens=tokenize(text,id),protectedRanges=[];let at=0;
 for(const token of tokens){const length=typeof token==='string'?token.length:token.length;if(typeof token!=='string'&&/comment|string|char|regex|macro|template|heredoc/.test(token.type))protectedRanges.push([at,at+length]);at+=length;}
 let mask='',last=0;for(const [start,end] of protectedRanges){mask+=text.slice(last,start)+text.slice(start,end).replace(/[^\n]/g,' ');last=end;}mask+=text.slice(last);
 const lines=text.split('\n'),code=mask.split('\n'),result=[];let curly=0,round=0,square=0,offset=0,rangeIndex=0;
 for(let i=0;i<lines.length;i++){
  const line=lines[i],trim=code[i].trim(),leading=(trim.match(/^[}\])]+/)||[''])[0],closedCurly=(leading.match(/}/g)||[]).length;
  while(rangeIndex<protectedRanges.length&&protectedRanges[rangeIndex][1]<=offset)rangeIndex++;
  const range=protectedRanges[rangeIndex],inLiteral=!!range&&offset>range[0]&&offset<range[1],preprocessor=/^\s*#/.test(line)&&id!=='powershell';
  const continuation=round>0&&!leading.includes(')')||square>0&&!leading.includes(']');
  const level=Math.max(0,curly-closedCurly)+(continuation?1:0),prefix=useTabs?'\t'.repeat(level):' '.repeat(Math.min(512,level*width));
  result.push(!line.trim()||inLiteral||preprocessor?line:prefix+line.trimStart());
  for(const char of code[i]){if(char==='{')curly++;else if(char==='}')curly--;else if(char==='(')round++;else if(char===')')round--;else if(char==='[')square++;else if(char===']')square--;if(curly<0||round<0||square<0)throw Error('Unbalanced delimiters. Indentation was not changed.');}
  offset+=line.length+1;
 }
 if(curly||round||square)throw Error('Complete the open delimiters before indenting the document.');
 return result.join('\n');
}
let loaded=false;
async function format(text,id,options){const parser=NTKEditorLanguages.parser(id),width=[2,4,8].includes(options.indent)?options.indent:4,useTabs=options.indent===0;
 if(!parser){if(NTKEditorLanguages.formatKind(id)!=='indent')throw Error('Automatic formatting is not available for '+NTKEditorLanguages.label(id)+'.');return {text:indentCode(text,id,width,useTabs),mode:'indent'};}
 if(!loaded){importScripts('editor-vendor/prettier.js','editor-vendor/prettier-babel.js','editor-vendor/prettier-estree.js','editor-vendor/prettier-typescript.js','editor-vendor/prettier-html.js','editor-vendor/prettier-postcss.js','editor-vendor/prettier-markdown.js','editor-vendor/prettier-yaml.js','editor-vendor/prettier-graphql.js');loaded=true;}
 const formatted=await prettier.format(text,{parser,plugins:Object.values(prettierPlugins),tabWidth:width,useTabs,printWidth:100,endOfLine:'lf',embeddedLanguageFormatting:'auto'});return {text:formatted,mode:'format'};
}
self.onmessage=async event=>{const {id,action,language,text,options={}}=event.data||{};
 try{const network=action==='highlight'&&NTKNetworkConfig.supports(language);if(typeof text!=='string'||text.length>(network?NTKNetworkConfig.MAX_PAINT:250000))throw Error(network?'Network syntax supports documents up to 1,000,000 characters.':'Language tools support documents up to 250,000 characters.');if(action==='highlight')self.postMessage(network?{id,...NTKNetworkConfig.paint(text,language)}:{id,html:render(tokenize(text,language))});else if(action==='format')self.postMessage({id,...await format(text,language,options)});else throw Error('Unknown editor operation.');}
 catch(error){self.postMessage({id,error:String(error.message||error)});}
};
