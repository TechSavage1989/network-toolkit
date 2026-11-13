/* Text tools of the Text Editor (Tools menu, owner request v1.21.53).
 * The tools themselves run in the engine, one Go implementation for every
 * document (internal/texttools, cmd/network-toolkit/editor_text_tools.go):
 *  - page-held text is sent to /api/editor-views/text-tools and the result is
 *    applied with the editor's ordinary edit, so it is one undo step;
 *  - a disk-backed document runs the tool in the engine over its buffer, of
 *    any size, with progress and Cancel; the result is one undo step there;
 *  - "Make SonicWall export readable" opens a new untitled tab that the
 *    engine fills from the export; the export is never changed.
 * A tool works on the selection, or on the whole document when nothing is
 * selected; line tools widen a partial selection to whole lines. This file
 * also holds the Find bar's Extended escapes and the Replace template of the
 * Regular expression search mode, which the engine expands with the same
 * rules (internal/editorbuffer/template.go). Every list of tools (Tools menu,
 * right-click menu, command palette) comes from TOOLS below; the engine's
 * registry and the entries that name an engine tool are kept the same by
 * TestToolListMatchesThePage. */
(function(root){'use strict';
const GROUPS=Object.freeze([
 {id:'lines',label:'Lines',icon:'comment',description:'Sort, split, join, trim and clean up lines.'},
 {id:'encode',label:'Encode / decode',icon:'textCase',description:'Base64, URL, hex, HTML entities and escapes.'},
 {id:'format',label:'Format',icon:'format',description:'Format JSON and XML, letter case and tabs.'},
 {id:'device',label:'Device configs',icon:'file',description:'Make device configuration exports readable.'}
]);
// {id, group, label, description (the tooltip too), shortcut, engine tool,
//  lines (whole lines), done and same messages, ask (a dialog first)}.
// {scope} is "the selection", "the selected lines" or "the whole document";
// {n} is the count the engine reports.
const TOOLS=Object.freeze([
 {id:'comment',group:'lines',page:true,label:'Toggle comment',description:'Comments or uncomments the selected lines in the document’s language.',shortcut:'Ctrl+/'},
 {id:'duplicate',group:'lines',page:true,label:'Duplicate lines',description:'Copies the line with the cursor, or the selected lines, below themselves.',shortcut:'Ctrl+D'},
 {id:'split',group:'lines',engine:'split',label:'Split at text…',description:'Starts a new line at every occurrence of a text you choose, such as &, and removes that text (like replacing & with \\n).',shortcut:'Ctrl+Shift+L',ask:'split',done:'Split {scope} at {n} place(s).',same:'The text to split at does not occur in {scope}.'},
 {id:'join',group:'lines',engine:'join',lines:true,label:'Join lines…',description:'Joins the selected lines into one line, with a separator you choose (a space unless you change it).',shortcut:'Ctrl+J',ask:'join',done:'Joined {scope} into one line ({n} line break(s) replaced).',same:'There is only one line to join in {scope}.'},
 {id:'sort-asc',group:'lines',engine:'sort-asc',lines:true,label:'Sort A→Z',description:'Sorts lines from A to Z, ignoring letter case, with numbers in number order (2 before 10).',done:'Sorted {scope} A→Z ({n} lines).',same:'The lines of {scope} are already in A→Z order.'},
 {id:'sort-desc',group:'lines',engine:'sort-desc',lines:true,label:'Sort Z→A',description:'Sorts lines from Z to A, ignoring letter case, with numbers in number order (10 before 2).',done:'Sorted {scope} Z→A ({n} lines).',same:'The lines of {scope} are already in Z→A order.'},
 {id:'dedupe',group:'lines',engine:'dedupe',lines:true,label:'Remove duplicate lines',description:'Removes lines that repeat an earlier line exactly; the first one stays where it is.',done:'Removed {n} duplicate line(s) from {scope}.',same:'{Scope} has no duplicate lines.'},
 {id:'remove-empty',group:'lines',engine:'remove-empty',lines:true,label:'Remove empty lines',description:'Removes lines that are empty or hold only spaces and tabs.',done:'Removed {n} empty line(s) from {scope}.',same:'{Scope} has no empty lines.'},
 {id:'trim-trailing',group:'lines',engine:'trim-trailing',lines:true,label:'Trim trailing spaces',description:'Removes spaces and tabs at the end of each line.',done:'Trimmed trailing spaces in {scope} ({n} line(s) changed).',same:'{Scope} has no trailing spaces.'},
 {id:'trim-both',group:'lines',engine:'trim-both',lines:true,label:'Trim leading and trailing spaces',description:'Removes spaces and tabs at the start and at the end of each line.',done:'Trimmed spaces in {scope} ({n} line(s) changed).',same:'{Scope} has no spaces to trim at the start or end of a line.'},
 {id:'reverse',group:'lines',engine:'reverse',lines:true,label:'Reverse line order',description:'Puts the lines in the opposite order: the last line first.',done:'Reversed the order of {scope} ({n} lines).',same:'{Scope} has only one line.'},
 {id:'base64-decode',group:'encode',engine:'base64-decode',label:'Base64 decode',description:'Decodes Base64 back to the text it holds; line breaks and spaces in the Base64 are ignored.',shortcut:'Ctrl+Shift+B',done:'Decoded {scope} from Base64.',same:'Decoding changed nothing in {scope}.'},
 {id:'base64-encode',group:'encode',engine:'base64-encode',label:'Base64 encode',description:'Encodes the UTF-8 text as Base64 on one line.',done:'Encoded {scope} as Base64.',same:'There is nothing to encode in {scope}.'},
 {id:'url-decode',group:'encode',engine:'url-decode',label:'URL decode',description:'Turns %XX escapes back into characters (%20 is a space). A + stays a plus sign.',done:'Decoded {n} URL escape(s) in {scope}.',same:'{Scope} has no %XX escapes.'},
 {id:'url-decode-form',group:'encode',engine:'url-decode-form',label:'URL decode form data',description:'Like URL decode, but a + is a space, as in submitted web forms.',done:'Decoded {n} URL escape(s) in {scope}.',same:'{Scope} has no %XX escapes or plus signs.'},
 {id:'url-encode',group:'encode',engine:'url-encode',label:'URL encode',description:'Writes every character except letters, digits and - _ . ~ as %XX.',done:'URL-encoded {n} character(s) in {scope}.',same:'{Scope} has nothing that needs URL encoding.'},
 {id:'hex-decode',group:'encode',engine:'hex-decode',label:'Hex to text',description:'Turns pairs of hexadecimal digits into the text they encode; spaces, colons, dashes and 0x are ignored.',done:'Decoded {scope} from hex.',same:'Decoding changed nothing in {scope}.'},
 {id:'hex-encode',group:'encode',engine:'hex-encode',label:'Text to hex',description:'Writes the UTF-8 bytes of the text as hexadecimal digit pairs.',done:'Encoded {scope} as hex.',same:'There is nothing to encode in {scope}.'},
 {id:'html-decode',group:'encode',engine:'html-decode',label:'HTML entities decode',description:'Turns entities such as &amp;amp; &amp;lt; &amp;nbsp; and &amp;#233; into the characters they stand for.',done:'Decoded {n} HTML entit(ies) in {scope}.',same:'{Scope} has no HTML entities.'},
 {id:'html-encode',group:'encode',engine:'html-encode',label:'HTML entities encode',description:'Writes & < > " and \' as HTML entities.',done:'Encoded {n} character(s) of {scope} as HTML entities.',same:'{Scope} has no characters that need HTML entities.'},
 {id:'unescape',group:'encode',engine:'unescape',label:'Unescape \\n \\t \\" \\\\',description:'Turns escape sequences such as \\n, \\t, \\", \\\\, \\xNN and \\uNNNN into the characters they stand for.',done:'Unescaped {n} sequence(s) in {scope}.',same:'{Scope} has no escape sequences.'},
 {id:'escape',group:'encode',engine:'escape',label:'Escape \\n \\t \\" \\\\',description:'Writes line breaks, tabs, double quotes and backslashes as \\n, \\t, \\" and \\\\ (other control characters as \\xNN).',done:'Escaped {n} character(s) in {scope}.',same:'{Scope} has nothing to escape.'},
 {id:'format',group:'format',page:true,label:'Format document',description:'Formats the document with the rules of its language (JSON, XML, HTML, CSS, JavaScript and more).',shortcut:'Ctrl+Shift+F'},
 {id:'json-pretty',group:'format',engine:'json-pretty',label:'Pretty-print JSON',description:'Puts each JSON member and item on its own line, indented with the editor’s indent setting. Text inside strings is not changed.',done:'Formatted {scope} as indented JSON.',same:'{Scope} is already indented JSON.'},
 {id:'json-minify',group:'format',engine:'json-minify',label:'Minify JSON',description:'Removes all white space between JSON values, giving one line.',done:'Minified the JSON of {scope}.',same:'The JSON of {scope} is already minified.'},
 {id:'xml-pretty',group:'format',engine:'xml-pretty',label:'Pretty-print XML',description:'Puts each XML element on its own line, indented; an element that holds only text stays on one line.',done:'Formatted {scope} as indented XML.',same:'{Scope} is already indented XML.'},
 {id:'xml-minify',group:'format',engine:'xml-minify',label:'Minify XML',description:'Removes the white space between XML tags.',done:'Minified the XML of {scope}.',same:'The XML of {scope} has no white space between tags.'},
 {id:'tabs-to-spaces',group:'format',engine:'tabs-to-spaces',lines:true,label:'Tabs to spaces',description:'Replaces each tab with spaces up to the next tab stop (the editor’s indent width).',done:'Replaced {n} tab(s) in {scope} with spaces.',same:'{Scope} has no tabs.'},
 {id:'spaces-to-tabs',group:'format',engine:'spaces-to-tabs',lines:true,label:'Leading spaces to tabs',description:'Replaces the spaces that indent each line with tabs (the editor’s indent width per tab).',done:'Indented {scope} with tabs ({n} line(s) changed).',same:'{Scope} has no indentation to change.'},
 {id:'upper',group:'format',engine:'upper',label:'UPPERCASE',description:'Changes letters to capital letters.',shortcut:'Ctrl+Shift+U',done:'Changed {scope} to UPPERCASE.',same:'{Scope} is already UPPERCASE.'},
 {id:'lower',group:'format',engine:'lower',label:'lowercase',description:'Changes letters to small letters.',shortcut:'Ctrl+U',done:'Changed {scope} to lowercase.',same:'{Scope} is already lowercase.'},
 {id:'title',group:'format',engine:'title',label:'Title Case',description:'Starts every word with a capital letter and writes the rest of it in small letters.',done:'Changed {scope} to Title Case.',same:'{Scope} is already in Title Case.'},
 {id:'sonicwall-readable',group:'device',engine:'sonicwall-readable',label:'Make SonicWall export readable',description:'Opens a readable copy of a SonicWall settings export (.exp) in a new tab: one setting per line, values decoded. Line breaks and other control characters inside a value show as \\n, \\r or \\xNN. The export is not changed.'}
]);
const byId=new Map(TOOLS.map(tool=>[tool.id,tool]));
// The standalone Terminal's floating editor (files of its SFTP sidebar) keeps
// the editing actions it always had, now through the Tools menu: no other
// tools, no new shortcuts and no SonicWall offer there.
const TERMINAL_TOOLS=new Set(['comment','duplicate','format','upper','lower','trim-trailing','sort-asc','hex-encode','hex-decode','base64-encode','base64-decode']);
// Action names of earlier versions (toolbar buttons, menus, tests) still work.
const ALIASES=Object.freeze({sortLines:'sort-asc',trim:'trim-trailing',textToHex:'hex-encode',hexToText:'hex-decode',base64Encode:'base64-encode',base64Decode:'base64-decode'});
// "Trim spaces" of earlier versions trimmed the ends of lines: "trim" is
// Trim trailing spaces, and Trim leading and trailing spaces is "trim-both".
function tool(id){return byId.get(ALIASES[id]||id)||null;}

// ---- Find window: Extended escapes ----
// (The Replace text of a regular expression is a template that the engine
// expands for every document: internal/editorbuffer/template.go.)
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
function message(template,scope,lines,count){
 const where=scopeText(scope,lines);
 return String(template||'').replace('{Scope}',where[0].toUpperCase()+where.slice(1)).replace('{scope}',where).replace('{n}',Number(count||0).toLocaleString());
}

function create(adapter){
 let running=null;
 // Every tool belongs to every Text Editor: the main window's, detached
 // editor windows and the standalone Text Editor. The standalone Terminal's
 // floating editor offers only TERMINAL_TOOLS.
 const full=()=>adapter.scope?.()!=='terminal';
 const allows=id=>{const t=tool(id);return !!t&&(full()||TERMINAL_TOOLS.has(t.id));};
 const nameOf=path=>String(path||'Untitled').split(/[\\/]/).pop()||'Untitled';
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
   const answer=await adapter.ask({title:'Split at text',message:'Each occurrence of this text ends a line. Type it exactly; it is not a regular expression.',value:'&',accept:'Split',check:{label:'Keep the separator at the end of each line',checked:false},validate:value=>value?'':'Enter the text to split at.'});
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
  if(t.page)return adapter.pageAction(t.id);
  const d=adapter.current();
  if(!d){adapter.setMessage('Open or create a document first.');return false;}
  if(running){adapter.setMessage('Wait for the running text tool to finish, or cancel it.',true);return false;}
  const blocked=adapter.blocked(d,t.id==='sonicwall-readable');
  if(blocked){adapter.setMessage(blocked,true);return false;}
  if(!adapter.engine()){adapter.setMessage('Text tools need the Network Toolkit engine, which is not available here.',true);return false;}
  if(t.id==='sonicwall-readable')return makeReadable(d);
  const chosen=await options(t);
  if(chosen===null||adapter.current()!==d)return false;
  return d.disk?runDisk(d,t,chosen):runPage(d,t,chosen);
 }
 async function runPage(d,t,opts){
  const ta=adapter.textarea();if(!ta)return false;
  let start=Math.min(ta.selectionStart,ta.selectionEnd),end=Math.max(ta.selectionStart,ta.selectionEnd),scope='selection';
  if(start===end){start=0;end=d.content.length;scope='document';}
  else if(t.lines)[start,end]=lineRange(d.content,start,end);
  const revision=d.editRevision,text=d.content.slice(start,end);
  running={d,t};
  adapter.setMessage(t.label+'…');
  try{
   const result=await adapter.request('/editor-views/text-tools','POST',{action:'run',tool:t.engine,text,options:opts});
   if(adapter.current()!==d||d.editRevision!==revision||d.content.slice(start,end)!==text){adapter.setMessage('The document changed while the tool ran. Nothing was changed; run the tool again.',true);return false;}
   if(!result.ok){
    let why=String(result.error||'The tool failed.');
    if(Number.isSafeInteger(result.offset)){const [line,column]=lineColumn(d.content,start+result.offset);why+=' (line '+line+', column '+column+')';}
    adapter.setMessage(why+' Nothing was changed.',true);return false;
   }
   if(!result.changed){adapter.setMessage(message(t.same||'The tool would not change {scope}.',scope,t.lines));return false;}
   adapter.applyRange(d,start,end,result.text);
   adapter.setMessage(message(t.done,scope,t.lines,result.count)+' Undo (Ctrl+Z) takes it back.');
   return true;
  }catch(error){adapter.setMessage(error.message+' Nothing was changed.',true);return false;}
  finally{running=null;}
 }
 async function runDisk(d,t,opts){
  const view=adapter.diskView();if(!view)return false;
  const session=adapter.session(d);
  const job={d,t,id:'',cancelled:false};running=job;
  d.diskEditing=true;view.setReadOnly(true);adapter.changed();
  showProgress(t.label);
  try{
   await view.ready;await session.flush();
   const range=view.selection();
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
   if(state.state==='failed'){if(adapter.current()===d)adapter.setMessage(state.error,!/cancelled/.test(state.error));return false;}
   if(!state.changed){if(adapter.current()===d)adapter.setMessage(message(t.same||'The tool would not change {scope}.',state.scope,t.lines));return false;}
   // As after Replace All: the engine changed the buffer, so the view reads
   // the changed text again (its cache is keyed by revision) and selects
   // the result.
   session.initialize(state.metadata);
   if(adapter.diskView()===view){
    await view.load({start:Math.min(state.from,state.metadata.units)});
    await view.position(state.from,state.to,true);
   }
   if(adapter.current()===d)adapter.setMessage(message(t.done,state.scope,t.lines,state.count)+' Undo (Ctrl+Z) takes it back.');
   return true;
  }catch(error){if(adapter.current()===d)adapter.setMessage(error.message,true);return false;}
  finally{
   running=null;removeBar('progress');
   d.diskEditing=false;if(adapter.diskView()===view)view.setReadOnly(adapter.readOnly());adapter.changed();
  }
 }
 // ---- SonicWall settings export ----
 function readableName(d){const name=nameOf(d.path).replace(/\.exp$/i,'');return name+' (readable).txt';}
 async function makeReadable(d){
  removeBar('offer');d.textToolOffer='done';
  running={d,t:byId.get('sonicwall-readable')};
  adapter.setMessage('Making '+nameOf(d.path)+' readable…');
  try{
   if(d.disk){
    await adapter.session(d).flush();
    const copy=await adapter.openDerived(d,readableName(d),{uid:d.uid,tool:'sonicwall-readable',revision:d.editRevision});
    const info=await adapter.diskCall(copy,'status');
    adapter.setMessage('Opened a readable copy of '+nameOf(d.path)+': '+Number(info.derived||0).toLocaleString()+' settings, one per line. The export was not changed.');
    return true;
   }
   const result=await adapter.request('/editor-views/text-tools','POST',{action:'run',tool:'sonicwall-readable',text:d.content,options:{}});
   if(!result.ok){let why=String(result.error||'');if(Number.isSafeInteger(result.offset)){const [line,column]=lineColumn(d.content,result.offset);why+=' (line '+line+', column '+column+')';}adapter.setMessage(why,true);return false;}
   await adapter.openText(readableName(d),result.text);
   adapter.setMessage('Opened a readable copy of '+nameOf(d.path)+': '+Number(result.count||0).toLocaleString()+' settings, one per line. The export was not changed.');
   return true;
  }catch(error){adapter.setMessage('Could not make a readable copy: '+error.message,true);return false;}
  finally{running=null;}
 }
 // A document that was opened is checked once, on its first few kilobytes,
 // for the shape of a SonicWall export; a match offers the readable copy.
 async function opened(d){
  if(!d||d.textToolOffer||d.untitled||d.encoding==='Binary'||!full()||!adapter.engine())return;
  d.textToolOffer='checking';
  // Page-held text that does not start with lines of Base64 characters (no
  // spaces) is not sent to the engine to be checked.
  const head=d.disk?'':d.content.slice(0,4096);
  if(!d.disk&&!/^[A-Za-z0-9+/=_-]{16,}(?:\n[A-Za-z0-9+/=_-]*)*\s*$/.test(head)){d.textToolOffer='no';return;}
  try{
   const result=d.disk?await adapter.diskCall(d,'tool-detect',{}):await adapter.request('/editor-views/text-tools','POST',{action:'detect',text:head});
   d.textToolOffer=result?.sonicwall===true?'offered':'no';
  }catch{d.textToolOffer='no';}
  draw();
 }
 function draw(){
  const d=adapter.current();
  if(!d||d.textToolOffer!=='offered'){removeBar('offer');return;}
  const existing=bar('offer');if(existing&&existing.dataset.uid===d.uid)return;
  const el=showBar('offer','<span class="nt-editor-insert-text">This looks like a SonicWall settings export.</span><span class="nt-editor-insert-actions"><button type="button" class="btn small primary" data-ed-tool-offer="readable" title="'+byId.get('sonicwall-readable').description.replace(/"/g,'&quot;')+'">Make readable</button><button type="button" class="btn small ghost" data-ed-tool-offer="dismiss" title="Hide this bar for this tab.">Dismiss</button></span>','SonicWall settings export');
  if(!el)return;
  el.dataset.uid=d.uid;
  el.addEventListener('click',event=>{
   const action=event.target.closest?.('[data-ed-tool-offer]')?.dataset.edToolOffer;if(!action)return;
   event.preventDefault();event.stopPropagation();
   const doc=adapter.current();if(!doc||doc.uid!==el.dataset.uid)return;
   if(action==='dismiss'){doc.textToolOffer='dismissed';removeBar('offer');adapter.focus();return;}
   void run('sonicwall-readable');
  });
 }
 return Object.freeze({run,opened,draw,cancel,allows,full,get busy(){return !!running;}});
}
root.NTKEditorTextTools=Object.freeze({GROUPS,TOOLS,tool,decodeExtended,lineRange,lineColumn,create});
if(typeof module==='object'&&module.exports)module.exports=root.NTKEditorTextTools;
})(globalThis);
