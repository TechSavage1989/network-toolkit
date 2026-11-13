'use strict';
// Chromium keeps one style invalidation set for every :has() rule of a page.
// A selector with :has() before its last compound ("non-subject" :has(), as
// in `.page:has(.x) .panel>div`) adds the features of its subject compound to
// that set. When a DOM insertion may change the :has() state of an ancestor
// (body is one: `body:has(...)`, `.diagnostic-page:has(...)`), Chromium
// restyles every element below it that matches any feature of the set. A bare
// `div` subject therefore restyled all ~570 divs of the main window for every
// line the Text Editor inserted: Playwright's multi-line fill of a comparison
// editor took ~20 s, at the 20 s action limit of editor-layout-map.browser.cjs
// (the flake of 2026-11-13). The subject of such a selector must carry a
// class, id or attribute. Only a few rare elements may stay bare tags.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{test}=require('node:test');
const web=path.join(__dirname,'..','web');

// Bare tags already used this way, each a few dozen elements per page at most.
// Never add div, span, *, td, tr, li, p, a, button or SVG shapes: the editor,
// the terminal, result tables and graphs create those by the hundred.
const RARE_TAGS=new Set(['input','select','label','details','strong','h2']);

function stripComments(css){return css.replace(/\/\*[\s\S]*?\*\//g,'');}
function stylesOf(file){
 const text=fs.readFileSync(file,'utf8');
 if(!file.endsWith('.html'))return stripComments(text);
 return stripComments([...text.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map(match=>match[1]).join('\n'));
}
// Style rule preludes, also inside @media/@supports/@container/@layer blocks.
function preludes(css){
 const out=[];let buffer='',quote='';
 for(const ch of css){
  if(quote){buffer+=ch;if(ch===quote)quote='';continue;}
  if(ch==='"'||ch==="'"){quote=ch;buffer+=ch;continue;}
  if(ch==='{'){const prelude=buffer.trim();if(prelude&&!prelude.startsWith('@'))out.push(prelude);buffer='';continue;}
  if(ch==='}'||ch===';'){buffer='';continue;}
  buffer+=ch;
 }
 return out;
}
// Split at top level only: never inside (), [] or quotes.
function splitTop(text,isSeparator){
 const out=[];let depth=0,quote='',current='';
 for(const ch of text){
  if(quote){current+=ch;if(ch===quote)quote='';continue;}
  if(ch==='"'||ch==="'"){quote=ch;current+=ch;continue;}
  if(ch==='('||ch==='[')depth++;
  if(ch===')'||ch===']')depth--;
  if(!depth&&isSeparator(ch)){out.push(current);current='';continue;}
  current+=ch;
 }
 out.push(current);return out.map(part=>part.trim()).filter(Boolean);
}
const compounds=selector=>splitTop(selector.replace(/\s*([>+~])\s*/g,' '),ch=>/\s/.test(ch));
// Remove a functional pseudo-class with its balanced argument.
function removeFunctional(compound,names){
 let out='';
 for(let i=0;i<compound.length;){
  const match=new RegExp('^::?(?:'+names+')\\(','i').exec(compound.slice(i));
  if(!match){out+=compound[i++];continue;}
  let depth=0,j=i+match[0].length-1;
  for(;j<compound.length;j++){if(compound[j]==='(')depth++;else if(compound[j]===')'&&--depth===0)break;}
  i=j+1;
 }
 return out;
}
function functionalArguments(compound,names){
 const out=[],pattern=new RegExp(':(?:'+names+')\\(','gi');let match;
 while((match=pattern.exec(compound))){let depth=0,j=match.index+match[0].length-1;for(;j<compound.length;j++){if(compound[j]==='(')depth++;else if(compound[j]===')'&&--depth===0)break;}out.push(compound.slice(match.index+match[0].length,j));pattern.lastIndex=j+1;}
 return out;
}
// The bare type selectors Chromium would have to use for this compound
// (empty when it has an id, class or attribute to key on).
function bareTags(compound){
 const own=removeFunctional(compound,'is|where|not|has|nth-child|nth-last-child|nth-of-type|nth-last-of-type|host|host-context|dir|lang|state|slotted|part|highlight')
  .replace(/::?[a-z-]+/gi,'');
 if(/[.#[]/.test(own))return [];
 const nested=functionalArguments(compound,'is|where');
 if(nested.length){const tags=nested.flatMap(list=>splitTop(list,ch=>ch===',')).flatMap(part=>bareTags(compounds(part).at(-1)));return [...new Set(tags)];}
 const tag=/^[a-z][a-z0-9-]*/i.exec(own)?.[0]?.toLowerCase();
 return [tag||'*'];
}
// Selectors whose subject is a bare tag although a :has() comes before it.
function violations(css,allowed=RARE_TAGS){
 const out=[];
 for(const prelude of preludes(css))for(const selector of splitTop(prelude,ch=>ch===',')){
  const parts=compounds(selector);
  if(parts.length<2||!parts.slice(0,-1).some(part=>/:has\(/i.test(part)))continue;
  const tags=bareTags(parts.at(-1)).filter(tag=>!allowed.has(tag));
  if(tags.length)out.push({selector,tags});
 }
 return out;
}

test('a bare tag after a non-subject :has() is found; classes, ids and attributes are not',()=>{
 const flagged=css=>violations(css).map(item=>item.tags.join(','));
 assert.deepEqual(flagged('.diagnostic-page #toolResult:has(.result-graph-entry) .graph-run-details[open]>div{position:absolute}'),['div']);
 assert.deepEqual(flagged('@media(max-width:600px){body:has(.a) span{color:red}}'),['span']);
 assert.deepEqual(flagged('body:has(.a) *{color:red}body:has(.a)>:hover{color:red}'),['*','*']);
 assert.deepEqual(flagged('body:has(.a) :is(div,.b){color:red}'),['div']);
 assert.deepEqual(flagged('.x:has(.y) td,.z{color:red}'),['td']);
 assert.deepEqual(flagged('.x:has(.y) div::before{content:"{"}'),['div']);
 assert.deepEqual(flagged([
  'body:has(.a) .b{}','body:has(.a) #b{}','body:has(.a) div.b{}','body:has(.a)>[data-x]{}','body:has(.a) :is(.b,#c){}',
  'div:has(>.b){}','.x:has(.y)::before{}','.a .b div{}','.x:has(.y) :not(.z).w{}','.modal:has(#settingsTheme) :is(input:not([type="checkbox"]),select){}',
  '@keyframes spin{from{opacity:0}to{opacity:1}}'
 ].join('')),[]);
});

test('no stylesheet of the app keys a :has() rule on a frequent bare tag',()=>{
 const files=[path.join(web,'index.html'),...fs.readdirSync(path.join(web,'assets')).filter(name=>name.endsWith('.css')).sort().map(name=>path.join(web,'assets',name))];
 assert(files.length>40,'the app stylesheets were found');
 let checked=0;
 const found=files.flatMap(file=>{const css=stylesOf(file);checked+=preludes(css).filter(prelude=>prelude.includes(':has(')).length;return violations(css).map(item=>path.relative(web,file)+': '+item.selector.trim()+' (bare '+item.tags.join(', ')+')');});
 assert(checked>40,'the :has() rules were parsed');
 assert.deepEqual(found,[],'Give the subject of these selectors a class, id or attribute; a bare tag after :has() restyles every such element on each DOM insertion');
});
