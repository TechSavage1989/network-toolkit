/* Text tools of the Text Editor (Tools menu, owner request v1.21.53).
 * The tools themselves run in the engine, one Go implementation for every
 * document (internal/texttools, cmd/network-toolkit/editor_text_tools.go):
 *  - page-held text is sent to /api/editor-views/text-tools and the result is
 *    applied with the editor's ordinary edit, so it is one undo step;
 *  - a disk-backed document runs the tool in the engine over its buffer, of
 *    any size, with progress and Cancel; the result is one undo step there.
 * They are general tools: none is for one brand's files or one workflow, and
 * each description says plainly what it does, with a tiny example.
 * A tool works on the selection, or on the whole document when nothing is
 * selected; line tools widen a partial selection to whole lines. A decoder
 * that stops at a character it does not decode changes nothing; when the
 * text before it decodes on its own, a bar offers Decode the part before it
 * (the same tool over just that range). This file also holds the Find bar's
 * Extended escapes and the Replace template of the Regular expression search
 * mode, which the engine expands with the same rules
 * (internal/editorbuffer/template.go). Every list of tools (Tools menu,
 * right-click menu, command palette) comes from TOOLS below; the engine's
 * registry and the entries that name an engine tool are kept the same by
 * TestToolListMatchesThePage. */
(function(root){'use strict';
const GROUPS=Object.freeze([
 {id:'lines',label:'Lines',icon:'comment',description:'Split, join, sort, trim and clean up lines.'},
 {id:'encode',label:'Encode / decode',icon:'textCase',description:'Decode and encode Base64, URL escapes, hex, HTML entities and escapes such as \\n.'},
 {id:'format',label:'Format',icon:'format',description:'Lay out JSON and XML; change letter case and tabs.'}
]);
// A description shows its tiny example between backticks in this list; the
// menus show it without them, its spaces made non-breaking, so an example
// is never broken across two lines.
const withExamples=text=>text.replace(/`([^`]*)`/g,(_,example)=>example.replace(/ /g,'\u00a0'));
// {id, group, label, description (the tooltip too: what it does, plainly,
//  with a tiny example), shortcut, engine tool, lines (whole lines), done
//  and same messages, partial (after Decode the part before it), ask (a
//  dialog first)}. {scope} is "the selection", "the selected lines" or "the
// whole document"; {n} is the count the engine reports; {where} the line and
// column where a decoder stopped.
const TOOLS=Object.freeze([
 {id:'comment',group:'lines',page:true,label:'Toggle comment',description:'Adds or removes the language’s comment mark on the selected lines: `x = 1 → # x = 1`.',shortcut:'Ctrl+/'},
 {id:'duplicate',group:'lines',page:true,label:'Duplicate lines',description:'Copies the current line, or the selected lines, right below them: `a → a, a`.',shortcut:'Ctrl+D'},
 {id:'split',group:'lines',engine:'split',label:'Split at text…',description:'Starts a new line at every place the text appears and removes it: `a&b&c → three lines`.',shortcut:'Ctrl+Shift+L',ask:'split',done:'Split {scope} at {n} place(s).',same:'The text to split at does not occur in {scope}.'},
 {id:'join',group:'lines',engine:'join',lines:true,label:'Join lines…',description:'Joins the selected lines into one, with the text you choose between them: `lines a, b, c → a b c`.',shortcut:'Ctrl+J',ask:'join',done:'Joined {scope} into one line ({n} line break(s) replaced).',same:'There is only one line to join in {scope}.'},
 {id:'sort-asc',group:'lines',engine:'sort-asc',lines:true,label:'Sort A→Z',description:'Sorts lines from A to Z, ignoring letter case, with numbers in number order: `b, a10, a2 → a2, a10, b`.',done:'Sorted {scope} A→Z ({n} lines).',same:'The lines of {scope} are already in A→Z order.'},
 {id:'sort-desc',group:'lines',engine:'sort-desc',lines:true,label:'Sort Z→A',description:'Sorts lines from Z to A, ignoring letter case, with numbers in number order: `a2, b, a10 → b, a10, a2`.',done:'Sorted {scope} Z→A ({n} lines).',same:'The lines of {scope} are already in Z→A order.'},
 {id:'dedupe',group:'lines',engine:'dedupe',lines:true,label:'Remove duplicate lines',description:'Removes lines that repeat an earlier line exactly; the first one stays: `a, b, a → a, b`.',done:'Removed {n} duplicate line(s) from {scope}.',same:'{Scope} has no duplicate lines.'},
 {id:'remove-empty',group:'lines',engine:'remove-empty',lines:true,label:'Remove empty lines',description:'Removes lines that are empty or hold only spaces and tabs: `a, (empty), b → a, b`.',done:'Removed {n} empty line(s) from {scope}.',same:'{Scope} has no empty lines.'},
 {id:'trim-trailing',group:'lines',engine:'trim-trailing',lines:true,label:'Trim trailing spaces',description:'Removes spaces and tabs at the end of each line: `"a  " → "a"`.',done:'Trimmed trailing spaces in {scope} ({n} line(s) changed).',same:'{Scope} has no trailing spaces.'},
 {id:'trim-both',group:'lines',engine:'trim-both',lines:true,label:'Trim leading and trailing spaces',description:'Removes spaces and tabs at both ends of each line: `"  a  " → "a"`.',done:'Trimmed spaces in {scope} ({n} line(s) changed).',same:'{Scope} has no spaces to trim at the start or end of a line.'},
 {id:'reverse',group:'lines',engine:'reverse',lines:true,label:'Reverse line order',description:'Puts the lines in the opposite order: `a, b, c → c, b, a`.',done:'Reversed the order of {scope} ({n} lines).',same:'{Scope} has only one line.'},
 {id:'base64-decode',group:'encode',engine:'base64-decode',label:'Base64 decode',description:'`aGVsbG8= → hello`; spaces and line breaks in the Base64 are ignored.',shortcut:'Ctrl+Shift+B',done:'Decoded {scope} from Base64.',same:'Decoding changed nothing in {scope}.',partial:'Decoded the part before {where} from Base64; the text from there on is unchanged.'},
 {id:'base64-encode',group:'encode',engine:'base64-encode',label:'Base64 encode',description:'Writes the text as Base64 on one line: `hello → aGVsbG8=`.',done:'Encoded {scope} as Base64.',same:'There is nothing to encode in {scope}.'},
 {id:'url-decode',group:'encode',engine:'url-decode',label:'URL decode',description:'`%20 → space`, `%3A → colon (:)`; a + stays a plus sign.',done:'Decoded {n} URL escape(s) in {scope}.',same:'{Scope} has no %XX escapes.'},
 {id:'url-decode-form',group:'encode',engine:'url-decode-form',label:'URL decode form data',description:'Like URL decode, but a + is a space, as in web forms: `a+b%21 → a b!`',done:'Decoded {n} URL escape(s) in {scope}.',same:'{Scope} has no %XX escapes or plus signs.'},
 {id:'url-encode',group:'encode',engine:'url-encode',label:'URL encode',description:'Writes every character except letters, digits and - _ . ~ as %XX: `a b → a%20b`.',done:'URL-encoded {n} character(s) in {scope}.',same:'{Scope} has nothing that needs URL encoding.'},
 {id:'hex-decode',group:'encode',engine:'hex-decode',label:'Hex to text',description:'`68 69 → hi`; spaces, colons, dashes, commas and 0x between the digit pairs are ignored.',done:'Decoded {scope} from hex.',same:'Decoding changed nothing in {scope}.',partial:'Decoded the part before {where} from hex; the text from there on is unchanged.'},
 {id:'hex-encode',group:'encode',engine:'hex-encode',label:'Text to hex',description:'Writes the UTF-8 bytes of the text as pairs of hex digits: `hi → 6869`.',done:'Encoded {scope} as hex.',same:'There is nothing to encode in {scope}.'},
 {id:'html-decode',group:'encode',engine:'html-decode',label:'HTML entities decode',description:'`&lt;b&gt; → <b>`, `&amp; → &`, `&#233; → é`; other text stays as it is.',done:'Decoded {n} HTML entit(ies) in {scope}.',same:'{Scope} has no HTML entities.'},
 {id:'html-encode',group:'encode',engine:'html-encode',label:'HTML entities encode',description:'Writes & < > " and \' as HTML entities: `<b> → &lt;b&gt;`.',done:'Encoded {n} character(s) of {scope} as HTML entities.',same:'{Scope} has no characters that need HTML entities.'},
 {id:'unescape',group:'encode',engine:'unescape',label:'Unescape \\n \\t \\" \\\\',description:'`\\n → line break`, `\\t → tab`, `\\" → "`, `\\\\ → \\`, `\\x41 → A`; other text stays as it is.',done:'Unescaped {n} sequence(s) in {scope}.',same:'{Scope} has no escape sequences.'},
 {id:'escape',group:'encode',engine:'escape',label:'Escape \\n \\t \\" \\\\',description:'`Line break → \\n`, `tab → \\t`, `" → \\"`, `\\ → \\\\`; other control characters → \\xNN.',done:'Escaped {n} character(s) in {scope}.',same:'{Scope} has nothing to escape.'},
 {id:'format',group:'format',page:true,label:'Format document',description:'Lays out the document by the rules of its language, such as JSON, XML, HTML, CSS or JavaScript.',shortcut:'Ctrl+Shift+F'},
 {id:'json-pretty',group:'format',engine:'json-pretty',label:'Pretty-print JSON',description:'One JSON member or item per line, indented with the editor’s indent: `{"a":1} → three lines`. Strings stay as they are.',done:'Formatted {scope} as indented JSON.',same:'{Scope} is already indented JSON.'},
 {id:'json-minify',group:'format',engine:'json-minify',label:'Minify JSON',description:'Removes the white space between JSON values: `{ "a": 1 } → {"a":1}`.',done:'Minified the JSON of {scope}.',same:'The JSON of {scope} is already minified.'},
 {id:'xml-pretty',group:'format',engine:'xml-pretty',label:'Pretty-print XML',description:'`<a><b/></a> → three lines`: one XML element per line, indented; an element with only text stays on one line.',done:'Formatted {scope} as indented XML.',same:'{Scope} is already indented XML.'},
 {id:'xml-minify',group:'format',engine:'xml-minify',label:'Minify XML',description:'Removes the white space between XML tags: `<a> <b/> </a> → <a><b/></a>`.',done:'Minified the XML of {scope}.',same:'The XML of {scope} has no white space between tags.'},
 {id:'tabs-to-spaces',group:'format',engine:'tabs-to-spaces',lines:true,label:'Tabs to spaces',description:'Replaces each tab with spaces up to the next tab stop, the editor’s indent width: `with 4, a tab → 4 spaces`.',done:'Replaced {n} tab(s) in {scope} with spaces.',same:'{Scope} has no tabs.'},
 {id:'spaces-to-tabs',group:'format',engine:'spaces-to-tabs',lines:true,label:'Leading spaces to tabs',description:'Replaces the spaces that indent each line with tabs, one per indent width: `with 4, 8 spaces → 2 tabs`.',done:'Indented {scope} with tabs ({n} line(s) changed).',same:'{Scope} has no indentation to change.'},
 {id:'upper',group:'format',engine:'upper',label:'UPPERCASE',description:'Changes letters to capital letters: `Hello → HELLO`.',shortcut:'Ctrl+Shift+U',done:'Changed {scope} to UPPERCASE.',same:'{Scope} is already UPPERCASE.'},
 {id:'lower',group:'format',engine:'lower',label:'lowercase',description:'Changes letters to small letters: `Hello → hello`.',shortcut:'Ctrl+U',done:'Changed {scope} to lowercase.',same:'{Scope} is already lowercase.'},
 {id:'title',group:'format',engine:'title',label:'Title Case',description:'Starts every word with a capital letter and makes the rest small: `hello wORLD → Hello World`.',done:'Changed {scope} to Title Case.',same:'{Scope} is already in Title Case.'}
].map(entry=>({...entry,description:withExamples(entry.description)})));
const byId=new Map(TOOLS.map(tool=>[tool.id,tool]));
// The standalone Terminal's floating editor (files of its SFTP sidebar) keeps
// the editing actions it always had, now through the Tools menu: no other
// tools and no new shortcuts there.
const TERMINAL_TOOLS=new Set(['comment','duplicate','format','upper','lower','trim-trailing','sort-asc','hex-encode','hex-decode','base64-encode','base64-decode']);
// Action names of earlier versions (toolbar buttons, menus, tests) still work.
const ALIASES=Object.freeze({sortLines:'sort-asc',trim:'trim-trailing',textToHex:'hex-encode',hexToText:'hex-decode',base64Encode:'base64-encode',base64Decode:'base64-decode'});
// "Trim spaces" of earlier versions trimmed the ends of lines: "trim" is
// Trim trailing spaces, and Trim leading and trailing spaces is "trim-both".
function tool(id){return byId.get(ALIASES[id]||id)||null;}

// ---- Find bar: Extended escapes and the Replace template ----
// Extended mode turns \n \r \t \0 \\ \xNN and \uNNNN in Find and Replace into
// the characters. The editor holds every line break as one \n, so \r and
// \r\n are a line break too; \0 is refused because no text can hold NUL.
function decodeExtended(text){
 let out='';
 for(let i=0;i<text.length;i++){
  const c=text[i];
  if(c!=='\\'||i+1>=text.length){out+=c;continue;}
  const n=text[i+1];
  if(n==='n'){out+='\n';i++;}
  else if(n==='r'){out+='\n';i++;if(text.startsWith('\\n',i+1))i+=2;}
  else if(n==='t'){out+='\t';i++;}
  else if(n==='\\'){out+='\\';i++;}
  else if(n==='0')return {error:'\\0 stands for a NUL character, which no text in the editor holds.'};
  else if(n==='x'&&/^[0-9a-f]{2}$/i.test(text.slice(i+2,i+4))){const v=parseInt(text.slice(i+2,i+4),16);if(!v)return {error:'\\x00 stands for a NUL character, which no text in the editor holds.'};out+=v===13?'\n':String.fromCharCode(v);i+=3;}
  else if(n==='u'&&/^[0-9a-f]{4}$/i.test(text.slice(i+2,i+6))){const v=parseInt(text.slice(i+2,i+6),16);if(!v)return {error:'\\u0000 stands for a NUL character, which no text in the editor holds.'};out+=v===13?'\n':String.fromCharCode(v);i+=5;}
  else out+=c;
 }
 return {text:out};
}
// The Replace text of a regular-expression search (internal/editorbuffer/
// template.go has the same rules; testdata/replace-template.json is read by
// both tests): $1..$99, $0 and $& the whole match, ${name} or ${n}, $$ a
// dollar sign, \n \t (\r and \r\n a line break), \\ \$, \xNN \uNNNN. A
// reference to a group that does not exist, and any other backslash, stay as
// written; \0 is refused.
function parseTemplate(template,count,names){
 const parts=[];let text='';
 const group=n=>{if(text){parts.push({text});text='';}parts.push({group:n});};
 for(let i=0;i<template.length;){
  const c=template[i];
  if(c==='$'){
   const n=template[i+1];
   if(n===undefined){text+='$';i++;continue;}
   if(n==='$'){text+='$';i+=2;continue;}
   if(n==='&'){group(0);i+=2;continue;}
   if(n>='0'&&n<='9'){
    const d=template[i+2];
    if(d>='0'&&d<='9'){const two=Number(n+d);if(two>=1&&two<=count){group(two);i+=3;continue;}}
    if(Number(n)<=count)group(Number(n));else text+=template.slice(i,i+2);
    i+=2;continue;
   }
   if(n==='{'){
    const end=template.indexOf('}',i+2);
    if(end<0){text+='$';i++;continue;}
    const name=template.slice(i+2,end);let found=-1;
    if(/^(0|[1-9]\d*)$/.test(name)&&Number(name)<=count)found=Number(name);
    else if(name)found=names.indexOf(name);
    if(found<0)text+=template.slice(i,end+1);else group(found);
    i=end+1;continue;
   }
   text+='$';i++;continue;
  }
  if(c==='\\'){
   const n=template[i+1];
   if(n===undefined){text+='\\';i++;continue;}
   if(n==='n'){text+='\n';i+=2;continue;}
   if(n==='r'){text+='\n';i+=2;if(template.startsWith('\\n',i))i+=2;continue;}
   if(n==='t'){text+='\t';i+=2;continue;}
   if(n==='\\'){text+='\\';i+=2;continue;}
   if(n==='$'){text+='$';i+=2;continue;}
   if(n==='0')throw Error('The replacement would add a NUL character, which the Text Editor cannot hold.');
   if(n==='x'||n==='u'){
    const width=n==='x'?2:4,digits=template.slice(i+2,i+2+width);
    if(digits.length===width&&/^[0-9a-f]+$/i.test(digits)){
     let v=parseInt(digits,16);
     if(v===0)throw Error('The replacement would add a NUL character, which the Text Editor cannot hold.');
     if(v===13)v=10;if(v>=0xd800&&v<0xe000)v=0xfffd;
     text+=String.fromCharCode(v);i+=2+width;continue;
    }
   }
   text+=template.slice(i,i+2);i+=2;continue;
  }
  text+=c;i++;
 }
 if(text||!parts.length)parts.push({text});
 return parts;
}
// The group names of a regular expression, by number (index 0 unused).
function groupNames(source){
 const names=[''];
 // Count capturing groups in order: '(' not escaped, not in a class, not (?: (?= (?! (?<= (?<!.
 let inClass=false;
 for(let i=0;i<source.length;i++){
  const c=source[i];
  if(c==='\\'){i++;continue;}
  if(inClass){if(c===']')inClass=false;continue;}
  if(c==='['){inClass=true;continue;}
  if(c!=='(')continue;
  if(source[i+1]!=='?'){names.push('');continue;}
  const named=/^\?(?:P?<)([A-Za-z_][A-Za-z0-9_]*)>/.exec(source.slice(i+1));
  if(named)names.push(named[1]);
 }
 return names;
}
// expandReplacement(template, match) for a JavaScript RegExp match array.
function expandReplacement(template,match,source){
 const names=groupNames(source),count=match.length-1,parts=parseTemplate(template,count,names);
 return parts.map(part=>part.group===undefined?part.text:(match[part.group]??'')).join('');
}

// A partial selection widened to the whole lines it touches, with the line
// break that ends the last of them (Snapshot.LineRange does the same): so an
// empty last line is a line of the range.
function lineRange(content,a,b){
 const start=a===0?0:content.lastIndexOf('\n',a-1)+1,limit=b>a&&content[b-1]==='\n'?b-1:b,last=content.indexOf('\n',limit);
 return [start,Math.max(start,last<0?content.length:last+1)];
}
// Line and column (from 1) of a text position.
function lineColumn(content,at){
 let line=1,from=0;
 for(let i=content.indexOf('\n');i>=0&&i<at;i=content.indexOf('\n',i+1)){line++;from=i+1;}
 return [line,at-from+1];
}
function scopeText(scope,lines){return scope==='document'?'the whole document':lines?'the selected lines':'the selection';}
function message(template,scope,lines,count,at=''){
 const where=scopeText(scope,lines);
 return String(template||'').replace('{Scope}',where[0].toUpperCase()+where.slice(1)).replace('{scope}',where).replace('{n}',Number(count||0).toLocaleString()).replace('{where}',at);
}

function create(adapter){
 let running=null;
 // Every tool belongs to every Text Editor: the main window's, detached
 // editor windows and the standalone Text Editor. The standalone Terminal's
 // floating editor offers only TERMINAL_TOOLS.
 const full=()=>adapter.scope?.()!=='terminal';
 const allows=id=>{const t=tool(id);return !!t&&(full()||TERMINAL_TOOLS.has(t.id));};
 function bar(kind){return adapter.overlay()?.querySelector('[data-ed-tool-bar="'+kind+'"]');}
 function removeBar(kind){bar(kind)?.remove();}
 function showBar(kind,html,label){
  removeBar(kind);
  const host=adapter.overlay();if(!host)return null;
  const el=root.document.createElement('div');el.className='nt-editor-insert-bar nt-editor-tool-bar';el.dataset.edToolBar=kind;el.setAttribute('role','region');el.setAttribute('aria-label',label);el.innerHTML=html;
  const anchor=host.querySelector('.nt-editor-options');if(anchor)anchor.after(el);else host.prepend(el);
  return el;
 }
 // ---- Progress of a tool running in the engine ----
 function showProgress(label){
  const el=showBar('progress','<span class="nt-editor-insert-text" data-ed-tool-progress-text role="status" aria-live="polite"></span><progress max="100" value="0" data-ed-tool-progress-bar aria-label="Progress"></progress><span class="nt-editor-insert-actions"><button type="button" class="btn small ghost" data-ed-tool-cancel title="Stop the tool. Nothing is changed.">Cancel</button></span>','Text tool progress');
  if(!el)return;
  el.querySelector('[data-ed-tool-cancel]').addEventListener('click',()=>void cancel());
  updateProgress(label,0);
 }
 function updateProgress(label,progress){
  const el=bar('progress');if(!el)return;
  const percent=Math.max(0,Math.min(100,Math.round((progress||0)*100)));
  el.querySelector('[data-ed-tool-progress-text]').textContent=label.replace(/…$/,'')+'… '+percent+'%';
  el.querySelector('[data-ed-tool-progress-bar]').value=percent;
 }
 async function cancel(){
  const job=running;if(!job||job.cancelled)return;
  job.cancelled=true;
  if(job.id){try{await adapter.diskCall(job.d,'tool-cancel',{job:job.id});}catch{}}
 }
 // ---- Options asked before a tool runs ----
 async function options(t){
  const indent=adapter.indent();
  if(t.ask==='split'){
   const answer=await adapter.ask({title:'Split at text',message:'Every place this text appears ends a line, and the text itself is removed: a&b&c becomes three lines. Type it exactly; it is not a regular expression.',value:'&',accept:'Split',check:{label:'Keep the separator at the end of each line',checked:false},validate:value=>value?'':'Enter the text to split at.'});
   if(answer===null)return null;
   return {separator:answer.value,keepSeparator:answer.checked===true,indent};
  }
  if(t.ask==='join'){
   const answer=await adapter.ask({title:'Join lines',message:'This text goes between the joined lines. Leave it empty to join them directly.',value:' ',accept:'Join'});
   if(answer===null)return null;
   return {separator:typeof answer==='string'?answer:answer.value,indent};
  }
  return {indent};
 }
 // ---- Running a tool ----
 async function run(id){
  const t=tool(id);
  if(!t||!allows(t.id))return false;
  dropPartial();
  if(t.page)return adapter.pageAction(t.id);
  const d=adapter.current();
  if(!d){adapter.setMessage('Open or create a document first.');return false;}
  if(!ready(d))return false;
  const chosen=await options(t);
  if(chosen===null||adapter.current()!==d)return false;
  return d.disk?runDisk(d,t,chosen):runPage(d,t,chosen);
 }
 // Whether a tool may run on d now; if not, the message says why.
 function ready(d){
  if(running){adapter.setMessage('Wait for the running text tool to finish, or cancel it.',true);return false;}
  const blocked=adapter.blocked(d);
  if(blocked){adapter.setMessage(blocked,true);return false;}
  if(!adapter.engine()){adapter.setMessage('Text tools need the Network Toolkit engine, which is not available here.',true);return false;}
  return true;
 }
 // part: {from, to, scope, where} when Decode the part before it runs the
 // tool over the range up to the character a decoder stopped at.
 async function runPage(d,t,opts,part=null){
  const ta=adapter.textarea();if(!ta)return false;
  let start=Math.min(ta.selectionStart,ta.selectionEnd),end=Math.max(ta.selectionStart,ta.selectionEnd),scope='selection';
  if(part){start=part.from;end=part.to;scope=part.scope;}
  else if(start===end){start=0;end=d.content.length;scope='document';}
  else if(t.lines)[start,end]=lineRange(d.content,start,end);
  const revision=d.editRevision,text=d.content.slice(start,end);
  running={d,t};
  adapter.setMessage(t.label+'…');
  try{
   const result=await adapter.request('/editor-views/text-tools','POST',{action:'run',tool:t.engine,text,options:opts});
   if(adapter.current()!==d||d.editRevision!==revision||d.content.slice(start,end)!==text){adapter.setMessage('The document changed while the tool ran. Nothing was changed; run the tool again.',true);return false;}
   if(!result.ok){
    let why=String(result.error||'The tool failed.'),at=null;
    if(Number.isSafeInteger(result.offset)){const [line,column]=lineColumn(d.content,start+result.offset);at={line,column};why+=' (line '+line+', column '+column+')';}
    why+=' Nothing was changed.';
    adapter.setMessage(why,true);
    if(!part&&at&&result.partial===true&&result.offset>0)offerPartial(d,t,{from:start,to:start+result.offset,...at,scope},why);
    return false;
   }
   if(!result.changed){adapter.setMessage(message(t.same||'The tool would not change {scope}.',scope,t.lines));return false;}
   adapter.applyRange(d,start,end,result.text);
   adapter.setMessage((part?message(t.partial,scope,t.lines,result.count,part.where):message(t.done,scope,t.lines,result.count))+' Undo (Ctrl+Z) takes it back.');
   return true;
  }catch(error){adapter.setMessage(error.message+' Nothing was changed.',true);return false;}
  finally{running=null;}
 }
 async function runDisk(d,t,opts,part=null){
  const view=adapter.diskView();if(!view)return false;
  const session=adapter.session(d);
  const job={d,t,id:'',cancelled:false};running=job;
  d.diskEditing=true;view.setReadOnly(true);adapter.changed();
  showProgress(t.label);
  let offer=null;
  try{
   await view.ready;await session.flush();
   const range=part?{start:part.from,end:part.to}:view.selection();
   const started=await adapter.diskCall(d,'tool-start',{tool:t.engine,options:opts,revision:d.editRevision,from:range.start,to:range.end});
   job.id=started.job;
   if(job.cancelled)await adapter.diskCall(d,'tool-cancel',{job:job.id}).catch(()=>{});
   let state;
   for(;;){
    state=await adapter.diskCall(d,'tool-status',{job:job.id});
    if(state.state!=='running')break;
    updateProgress(t.label,state.progress);
    await new Promise(resolve=>root.setTimeout(resolve,120));
   }
   if(state.state==='failed'){
    if(adapter.current()===d){
     adapter.setMessage(state.error,!/cancelled/.test(state.error));
     const p=state.partial;
     if(!part&&p&&[p.from,p.to,p.line,p.column].every(Number.isSafeInteger)&&p.to>p.from)offer={from:p.from,to:p.to,line:p.line,column:p.column,scope:p.scope==='document'?'document':'selection'};
    }
    if(offer)offer.error=state.error;
    return false;
   }
   if(!state.changed){if(adapter.current()===d)adapter.setMessage(message(t.same||'The tool would not change {scope}.',state.scope,t.lines));return false;}
   // As after Replace All: the engine changed the buffer, so the view reads
   // the changed text again (its cache is keyed by revision) and selects
   // the result.
   session.initialize(state.metadata);
   if(adapter.diskView()===view){
    await view.load({start:Math.min(state.from,state.metadata.units)});
    await view.position(state.from,state.to,true);
   }
   if(adapter.current()===d)adapter.setMessage((part?message(t.partial,part.scope,t.lines,state.count,part.where):message(t.done,state.scope,t.lines,state.count))+' Undo (Ctrl+Z) takes it back.');
   return true;
  }catch(error){if(adapter.current()===d)adapter.setMessage(error.message,true);return false;}
  finally{
   running=null;removeBar('progress');
   d.diskEditing=false;if(adapter.diskView()===view)view.setReadOnly(adapter.readOnly());adapter.changed();
   // Offered once the document is editable again, for its revision now.
   if(offer&&adapter.current()===d)offerPartial(d,t,offer,offer.error);
  }
 }
 // ---- Decode the part before it ----
 // A decoder (Base64 decode, Hex to text) that meets a character it does not
 // decode changes nothing and names it. When the text before that character
 // decodes on its own, the engine says so and a bar above the text offers
 // one button, Decode the part before it: the same tool over the range from
 // the start of the tool's range (the selection or the document) up to that
 // character, as one undo step. The character and everything after it stay
 // exactly as they were. The offer belongs to that document as it was: an
 // edit, another tab or another tool takes it away.
 let partial=null;
 function offerPartial(d,t,range,error){
  partial={d,revision:d.editRevision,t,from:range.from,to:range.to,scope:range.scope,where:'line '+range.line+', column '+range.column,error};
  draw();
 }
 function dropPartial(){partial=null;removeBar('partial');}
 function draw(){
  const d=adapter.current();
  if(!partial||d!==partial.d||d.editRevision!==partial.revision){dropPartial();return;}
  if(bar('partial'))return;
  const el=showBar('partial','<span class="nt-editor-insert-text" data-ed-tool-partial-text role="status" aria-live="polite"></span><span class="nt-editor-insert-actions"><button type="button" class="btn small primary" data-ed-tool-partial>Decode the part before it</button></span>',partial.t.label+' stopped');
  if(!el)return;
  el.querySelector('[data-ed-tool-partial-text]').textContent=partial.error;
  const button=el.querySelector('[data-ed-tool-partial]');
  button.title='Decodes '+scopeText(partial.scope)+' from its start up to '+partial.where+'. That character and everything after it stay as they are; Undo (Ctrl+Z) takes it back.';
  button.addEventListener('click',event=>{event.preventDefault();event.stopPropagation();void decodeBefore();});
 }
 async function decodeBefore(){
  const p=partial,d=adapter.current();
  // The bar and its button go; the text keeps the keyboard focus.
  dropPartial();adapter.focus();
  if(!p)return false;
  if(d!==p.d||d.editRevision!==p.revision){adapter.setMessage('The document changed after '+p.t.label+' stopped. Nothing was changed; run it again.',true);return false;}
  if(!ready(d))return false;
  const part={from:p.from,to:p.to,scope:p.scope,where:p.where},opts={indent:adapter.indent()};
  return d.disk?runDisk(d,p.t,opts,part):runPage(d,p.t,opts,part);
 }
 return Object.freeze({run,draw,cancel,allows,full,get busy(){return !!running;}});
}
root.NTKEditorTextTools=Object.freeze({GROUPS,TOOLS,tool,decodeExtended,parseTemplate,groupNames,expandReplacement,lineRange,lineColumn,create});
if(typeof module==='object'&&module.exports)module.exports=root.NTKEditorTextTools;
})(globalThis);
