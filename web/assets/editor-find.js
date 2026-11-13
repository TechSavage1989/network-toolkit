/* The Find window of the Text Editor (owner request v1.21.55, modelled on
 * Notepad++): a movable window inside the editor with the tabs Find, Replace
 * and Mark, the Search results panel docked at the bottom of the editor, and
 * the marks and bookmarks of the open documents.
 *
 * One search implementation: every search runs in the engine
 * (internal/editorbuffer find.go, cmd/network-toolkit/editor_find.go). Text
 * that a page holds is sent with the request; a disk-backed document is
 * searched on its buffer route, any size, never read whole into the page.
 * Count, Find All, Mark All, Copy Marked Text and Replace All of a disk-backed
 * document run in the background with progress and Cancel.
 *
 * Nothing here is saved: the search history lives in the engine's memory
 * (search text can hold passwords), marks and bookmarks only while their
 * document is open. Only the window's transparency is a saved preference
 * (editorPreferences.findTransparency).
 *
 * The Search results panel never holds every row: the engine keeps the rows
 * of each Find All, the panel reads them a page at a time as it scrolls and
 * draws only the rows in view. */
(function(root){
'use strict';
const MODES=[['normal','Normal'],['extended','Extended (\\n, \\r, \\t, \\0, \\x…)'],['regex','Regular expression']];
const MODE_LABEL={normal:'Normal',extended:'Extended',regex:'Regular expression'};
const MODE_HELP={
 normal:'Normal: finds the text exactly as typed.',
 extended:'Extended: like Normal, but \\n, \\r, \\t, \\xNN and \\uNNNN in Find what and Replace with stand for a line break, a tab and other characters. Find & and replace with \\n to start a new line at every &.',
 regex:'Regular expression: Find what is a regular expression; ^ and $ match at the start and end of each line. Replace with can use $1, ${name} and $& for the text of groups, and \\n and \\t for a line break and a tab.'
};
const ROW=20,PAGE_ROWS=200,PAGE_CACHE=64,MARK_PATTERNS=16,BOOKMARKS=100000,COPY_ROWS=50000;
const TABS=[['find','Find'],['replace','Replace'],['mark','Mark']];
const NONE=Object.freeze([]);
const escapeHTML=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const count=n=>Number(n||0).toLocaleString();
const plural=(n,one,many)=>count(n)+' '+(Number(n)===1?one:many);

// Extended escapes are read here, for every document, by the same rules as
// before the window (editor-text-tools.js decodeExtended).
function decode(text,mode){
 if(mode!=='extended')return {text};
 const tools=root.NTKEditorTextTools;
 return tools?tools.decodeExtended(text):{text};
}

// The rows of a Find All result in the panel: one search block, a file
// block for each document, and the rows of each file.
function layout(searches){
 const out=[];let total=0;
 for(const s of searches){
  out.push({type:'search',s,at:total});total++;
  if(s.collapsed)continue;
  for(const f of s.files){
   out.push({type:'file',s,f,at:total});total++;
   if(!f.collapsed){out.push({type:'lines',s,f,at:total,count:f.rows});total+=f.rows;}
  }
 }
 return {blocks:out,total};
}
function rowAt(model,index){
 let low=0,high=model.blocks.length-1;
 while(low<high){const mid=(low+high+1)>>1;if(model.blocks[mid].at<=index)low=mid;else high=mid-1;}
 const block=model.blocks[low];
 if(!block)return null;
 if(block.type==='lines'){const line=index-block.at;return line<block.count?{type:'line',s:block.s,f:block.f,index:line}:null;}
 return block.at===index?block:null;
}

// The part of a row's text inside a hit, as marked HTML.
function rowHTML(row){
 const text=row.text||'',start=row.start,end=start+text.length,cuts=[];
 for(const [index,[a,b]] of (row.hits||[]).entries()){
  const from=Math.max(a,start)-start,to=Math.min(b,end)-start;
  if(to>from||a===b&&a>=start&&a<=end)cuts.push([from,Math.max(from,to),index]);
 }
 let html=row.cutStart?'<span class="nt-find-cut">…</span>':'',at=0;
 for(const [from,to,index] of cuts){
  if(from<at)continue;
  html+=escapeHTML(text.slice(at,from))+'<mark data-hit="'+index+'">'+escapeHTML(text.slice(from,to))+'</mark>';
  at=to;
 }
 return html+escapeHTML(text.slice(at))+(row.cutEnd?'<span class="nt-find-cut">…</span>':'');
}

// Bookmarks follow an edit: lines after it move by the lines it added or
// removed; a bookmark on a line the edit removed moves to the edit's line.
function shiftLines(lines,edit){
 if(!lines?.length)return lines;
 const {line,removed=0,inserted=0}=edit,delta=inserted-removed,out=[];
 for(const value of lines){
  let next=value;
  if(value>line+removed)next=value+delta;
  else if(value>line&&removed)next=line;
  if(next>=0&&out.at(-1)!==next)out.push(next);
 }
 return out;
}

// ---- The syntax guide and the line that explains the caret ----
// Every search runs in the engine (Go's regular expressions), so this lists
// exactly what works for every document; what the engine refuses is named
// in NOT_SUPPORTED. Each row: the token shown, the text a click inserts
// (with ‸ where the cursor goes), what it means, an example.
const GUIDE={
 extended:[
  ['\\n','\\n','a line break','a\\nb finds “a” and “b” on two lines'],
  ['\\r','\\r','a line break too (the editor keeps one \\n per line)','a\\rb is the same as a\\nb'],
  ['\\t','\\t','a tab','\\tvlan finds an indented “vlan”'],
  ['\\\\','\\\\','a backslash','C:\\\\temp finds “C:\\temp”'],
  ['\\xNN','\\x‸','the character with hex code NN','\\x41 finds “A”'],
  ['\\uNNNN','\\u‸','the character with hex code NNNN','\\u00e9 finds “é”']
 ],
 regex:[
  ['.','.','any character but a line break (any character with . matches newline)','v.an finds “vlan”'],
  ['\\d  \\D','\\d','a digit 0–9 / any other character','\\d+ finds “192” in “port 192”'],
  ['\\w  \\W','\\w','a letter, digit or _ / any other character','\\w+ finds “ge0” in “ge0/1”'],
  ['\\s  \\S','\\s','a space, tab or line break / any other character','a\\s+b finds “a   b”'],
  ['\\b','\\b','the edge of a word','\\bvlan\\b finds “vlan” but not “vlans”'],
  ['^  $','^','the start / end of a line (of each line, also with . matches newline)','^end$ finds a line that is only “end”'],
  ['*','*','the item before, any number of times (also none)','ab*c finds “ac” and “abbc”'],
  ['+','+','the item before, once or more','\\d+ finds “10” and “4094”'],
  ['?','?','the item before, optional; after a repeat: as few as possible','colou?r finds “color” and “colour”; <.+?> finds one tag'],
  ['{n}  {n,m}','{‸}','the item before, exactly n / n to m times','\\d{1,3} finds “192”'],
  ['[abc]','[‸]','one of the characters listed','[ab]c finds “ac” and “bc”'],
  ['[^abc]','[^‸]','one character that is not listed','[^0-9] finds “x” in “x1”'],
  ['[a-z]','[a-z]','one character in a range','[0-9a-f]+ finds “1f” in “0x1f”'],
  ['( )','(‸)','a group: repeats as one item, and is kept as $1, $2…','(ab)+ finds “abab”'],
  ['(?: )','(?:‸)','a group that is not kept','(?:ge|xe)-\\d finds “ge-0”'],
  ['(?<name> )','(?<‸>)','a group kept under a name, for ${name} in Replace with','(?<port>\\d+) keeps “22” as ${port}'],
  ['|','|','or: the part before, or the part after','ssh|telnet finds either'],
  ['\\','\\','takes the next character as it is','192\\.0 finds “192.0”, not “19210”'],
  ['(?i)','(?i)','from here on, upper and lower case are the same','(?i)vlan finds “VLAN”']
 ],
 replace:[
  ['$1','$1','the text of group 1 (up to $99)','(\\w+)=(\\w+) → $2=$1 turns “a=b” into “b=a”'],
  ['${name}','${‸}','the text of a named group','${port}'],
  ['$&  $0','$&','the whole match','[$&] puts it in brackets'],
  ['$$','$$','a dollar sign','$$1 writes “$1”'],
  ['\\n','\\n','a line break','&→\\n starts a line at each &'],
  ['\\t','\\t','a tab','\\t$1 indents the group']
 ]
};
const NOT_SUPPORTED='Not supported (the search says so): lookahead and lookbehind (?= (?! (?<= (?<!, backreferences such as \\1 in Find what (use $1 in Replace with), \\uNNNN in a regular expression (use \\x{NNNN}), possessive repeats such as ++ and atomic groups (?>…).';

// The token of a pattern at a position: [start, end, meaning].
function regexTokens(pattern){
 const out=[];let groups=0;
 for(let i=0;i<pattern.length;){
  const c=pattern[i],start=i;let end=i+1,meaning='the character “'+c+'”';
  if(c==='\\'){
   const n=pattern[i+1];end=Math.min(pattern.length,i+2);
   const named={d:'a digit 0–9',D:'any character but a digit',w:'a letter, digit or _',W:'any character but a letter, digit or _',s:'a space, tab or line break',S:'any character but a space, tab or line break',b:'the edge of a word',B:'not the edge of a word',A:'the start of the document',z:'the end of the document',n:'a line break',t:'a tab',r:'a carriage return (the editor keeps line breaks as \\n)',f:'a form feed',v:'a vertical tab'};
   if(n===undefined)meaning='a lone \\ at the end: write \\\\ for a backslash';
   else if(named[n])meaning=named[n];
   else if(n==='x'){const m=/^\\x(\{[0-9a-fA-F]+\}|[0-9a-fA-F]{2})/.exec(pattern.slice(i));if(m){end=i+m[0].length;meaning='the character with hex code '+m[1].replace(/[{}]/g,'');}else meaning='\\x needs a hex code: \\x41 or \\x{263a}';}
   else if(n==='u')meaning='\\uNNNN is not supported in a regular expression: use \\x{NNNN}';
   else if(/[1-9]/.test(n))meaning='backreferences such as \\'+n+' are not supported: use $'+n+' in Replace with';
   else if(n==='p'||n==='P'){const m=/^\\[pP](\{[^}]*\}|[A-Za-z])/.exec(pattern.slice(i));if(m)end=i+m[0].length;meaning=(n==='P'?'a character not in the Unicode class ':'a character in the Unicode class ')+(m?m[1].replace(/[{}]/g,''):'');}
   else meaning='the character “'+n+'” itself';
  }else if(c==='['){
   let j=i+1;const negate=pattern[j]==='^';if(negate)j++;if(pattern[j]===']')j++;
   while(j<pattern.length&&pattern[j]!==']'){if(pattern[j]==='\\')j++;j++;}
   end=Math.min(pattern.length,j+1);
   const body=pattern.slice(i+(negate?2:1),j);
   meaning=j>=pattern.length?'a [ without its closing ]':(negate?'one character that is not ':'one of ')+(body.includes('-')?'the characters and ranges ':'')+'“'+body+'”';
  }else if(c==='('){
   const rest=pattern.slice(i);
   let m;
   if((m=/^\(\?P?<([A-Za-z_][A-Za-z0-9_]*)>/.exec(rest))){groups++;end=i+m[0].length;meaning='starts group '+groups+', named “'+m[1]+'”: ${'+m[1]+'} in Replace with';}
   else if(/^\(\?[=!]|^\(\?<[=!]/.test(rest)){end=i+(rest[2]==='<'?4:3);meaning='lookahead and lookbehind are not supported';}
   else if(rest.startsWith('(?:')){end=i+3;meaning='starts a group that is not kept';}
   else if((m=/^\(\?([imsU-]+)(\)|:)/.exec(rest))){end=i+m[0].length;meaning=(m[2]===')'?'from here on: ':'a group with: ')+(m[1].includes('i')?'case is ignored':m[1].includes('s')?'. matches line breaks':m[1].includes('m')?'^ and $ match at lines':'these flags');}
   else{groups++;meaning='starts group '+groups+': $'+groups+' in Replace with';}
  }else if(c===')')meaning='ends a group';
  else if(c==='|')meaning='or: the part before, or the part after';
  else if(c==='.')meaning='any character but a line break';
  else if(c==='^')meaning='the start of a line';
  else if(c==='$')meaning='the end of a line';
  else if(c==='*'||c==='+'||c==='?'){
   const lazy=pattern[i+1]==='?';if(lazy)end=i+2;
   meaning=(c==='*'?'the item before, any number of times (also none)':c==='+'?'the item before, once or more':'the item before is optional')+(lazy?', as few as possible':'');
  }else if(c==='{'){
   const m=/^\{(\d+)(,(\d*))?\}(\?)?/.exec(pattern.slice(i));
   if(m){end=i+m[0].length;meaning='the item before, '+(m[2]===undefined?'exactly '+m[1]+' times':m[3]===''?m[1]+' or more times':m[1]+' to '+m[3]+' times')+(m[4]?', as few as possible':'');}
  }
  out.push([start,end,meaning]);i=end;
 }
 return out;
}
function templateTokens(text){
 const out=[];
 for(let i=0;i<text.length;){
  const rest=text.slice(i);let m;
  if((m=/^\$(\d{1,2})/.exec(rest)))out.push([i,i+m[0].length,m[1]==='0'?'the whole match':'the text of group '+m[1]]);
  else if((m=/^\$\{(\w+)\}/.exec(rest)))out.push([i,i+m[0].length,'the text of the group '+m[1]]);
  else if(rest.startsWith('$&'))out.push([i,i+2,'the whole match']);
  else if(rest.startsWith('$$'))out.push([i,i+2,'a dollar sign']);
  else if((m=/^\\([ntr\\$])/.exec(rest)))out.push([i,i+2,{n:'a line break',t:'a tab',r:'a line break','\\':'a backslash',$:'a dollar sign'}[m[1]]]);
  else out.push([i,i+1,'the character “'+rest[0]+'”']);
  i=out.at(-1)[1];
 }
 return out;
}
function extendedTokens(text){
 const out=[];
 for(let i=0;i<text.length;){
  const rest=text.slice(i);let m;
  if((m=/^\\x([0-9a-fA-F]{2})/.exec(rest)))out.push([i,i+4,m[1]==='00'?'\\x00 is a NUL character, which no text can hold':'the character with hex code '+m[1]]);
  else if((m=/^\\u([0-9a-fA-F]{4})/.exec(rest)))out.push([i,i+6,m[1]==='0000'?'\\u0000 is a NUL character, which no text can hold':'the character with hex code '+m[1]]);
  else if((m=/^\\([nrt0\\])/.exec(rest)))out.push([i,i+2,{n:'a line break',r:'a line break (the editor keeps one \\n per line)',t:'a tab','0':'\\0 is a NUL character, which no text can hold','\\':'a backslash'}[m[1]]]);
  else out.push([i,i+1,'the character “'+rest[0]+'”']);
  i=out.at(-1)[1];
 }
 return out;
}
// The meaning of what is at the caret (or just before it) in a field.
function explain(text,caret,mode,field='find'){
 if(!text||mode==='normal')return '';
 const tokens=mode==='extended'?extendedTokens(text):field==='replace'?templateTokens(text):regexTokens(text);
 const token=tokens.find(([a,b])=>caret>a&&caret<=b)||tokens.find(([a])=>a===caret)||tokens.at(-1);
 return token?text.slice(token[0],token[1])+' — '+token[2]:'';
}

function create(a){
 const opts={backward:false,word:false,matchCase:false,wrap:true,inSelection:false,mode:'normal',dotAll:false,bookmark:false,purge:false};
 const transparency={enabled:false,mode:'blur',level:70};
 const marks=new WeakMap(),bookmarks=new WeakMap(),markCache=new WeakMap();
 const searches=[],pages=new Map(),running=new Set();
 let el=null,panel=null,tab='find',pos=null,scope=null,lastFound=null,history={find:[],replace:[]},historyAsked=false;
 let resultsToggle=null,lastField='find',checkTimer=0,checkTicket=0,patternProblem=null;
 let selection={anchor:-1,focus:-1},menu=null,panelHeight=180,panelSized=false,renderFrame=0,searchSeq=0,markVersion=0,statusTimer=0,cancelling=null;

 // ---- Engine calls ----
 const engine=body=>a.request(body);
 function textBody(d){return {text:a.text(d)};}
 async function call(d,action,body,signal){
  if(a.disk(d)){await a.flush(d);return a.diskCall(d,action,body,signal);}
  const page={next:'next','find-matches':'matches','replace-text':'replace-text','find-start':'start'}[action]||action;
  // Page-held text is searched as sent: a revision means nothing there.
  const {revision,...rest}=body;void revision;
  return engine({action:page,...rest,...textBody(d)});
 }
 // A background job (Count, Find All, Mark All, Copy Marked Text, Replace
 // All of a disk-backed document): started, then polled until it ends.
 async function job(d,kind,body,progress){
  // A page-held document is searched as it is now: its revision then is
  // the one the rows belong to.
  const sent=a.revision(d);
  const start=a.disk(d)?await call(d,'find-start',{kind,...body}):await engine({action:'start',kind,...body,...textBody(d)});
  const entry={id:start.job,cancel:()=>engine({action:'cancel',job:start.job}).catch(()=>{})};
  running.add(entry);showProgress();
  try{
   for(;;){
    const state=await engine({action:'status',job:start.job});
    if(state.state==='done')return {...state,job:start.job,revision:a.disk(d)?start.revision:sent};
    if(state.state==='failed')throw Error(state.error||'The search failed.');
    progress?.(state);
    await new Promise(resolve=>root.setTimeout(resolve,90));
   }
  }finally{running.delete(entry);showProgress();}
 }
 function release(id){if(id)void engine({action:'release',job:id}).catch(()=>{});}

 // ---- The search of the window ----
 function search(){
  const input=el?.querySelector('[data-ed-query]'),raw=input?.value||'';
  if(!raw){status('Enter the text to find.',true);input?.focus();return null;}
  const value=decode(raw,opts.mode);
  if(value.error){status(value.error,true);return null;}
  if(!value.text){status('Enter the text to find.',true);return null;}
  if(value.text.length>4096){status('Find what is too long (at most 4,096 characters).',true);return null;}
  const regex=opts.mode==='regex';
  return {raw,query:value.text,search:{query:value.text,regex,matchCase:opts.matchCase,wholeWord:opts.word,dotAll:regex&&opts.dotAll}};
 }
 function replacementText(){
  const raw=el?.querySelector('[data-ed-replacement]')?.value||'';
  if(opts.mode==='regex')return {raw,text:raw};
  const value=decode(raw,opts.mode);
  return value.error?{raw,error:value.error}:{raw,text:value.text};
 }
 const modeTag=()=>'['+MODE_LABEL[opts.mode]+(opts.mode==='regex'&&opts.dotAll?', . matches newline':'')+']';
 // "In selection": the selected range, kept while the selection is the
 // match this window just selected in it.
 function range(d){
  if(!opts.inSelection||!d)return {within:null};
  const sel=a.selection(d);
  if(scope&&scope.uid===d.uid&&scope.revision===a.revision(d)&&lastFound&&lastFound.uid===d.uid&&lastFound.revision===scope.revision&&sel.start===lastFound.from&&sel.end===lastFound.to)return {within:[scope.from,scope.to]};
  if(sel.end>sel.start){scope={uid:d.uid,revision:a.revision(d),from:sel.start,to:sel.end};return {within:[sel.start,sel.end]};}
  return {error:'Select the text to search in first, or clear In selection.'};
 }
 function remember(list,value){
  if(!value)return;
  history[list]=[value,...history[list].filter(item=>item!==value)].slice(0,20);fillHistory();
  void engine({action:'remember',list,value}).catch(()=>{});
 }
 function fillHistory(){
  for(const [list,id] of [['find','ntEditorFindHistory'],['replace','ntEditorReplaceHistory']]){
   const node=el?.querySelector('#'+id);if(node)node.innerHTML=history[list].map(value=>'<option value="'+escapeHTML(value)+'"></option>').join('');
  }
 }
 async function loadHistory(){
  if(historyAsked||!a.engine())return;historyAsked=true;
  try{const value=await engine({action:'history'});for(const list of ['find','replace'])history[list]=[...history[list],...(value[list]||[]).filter(item=>!history[list].includes(item))].slice(0,20);fillHistory();}catch{historyAsked=false;}
 }
 function blockedRead(d){return !d?'Open or create a document first.':a.readBlocked(d);}
 function status(text,error=false,echo=true){
  const node=el?.querySelector('[data-ed-find-message]');
  if(node){node.textContent=text;node.classList.toggle('error',!!error);}
  if(echo)a.setMessage(text,error);
 }
 function showProgress(state){
  const box=el?.querySelector('[data-ed-find-progress]'),head=panel?.querySelector('[data-ed-find-results-cancel]');
  const busy=running.size>0;
  if(box){box.hidden=!busy;if(state&&busy)box.querySelector('[data-ed-find-progress-text]').textContent=state;}
  if(head)head.hidden=!busy;
  if(el)el.classList.toggle('is-busy',busy);
 }
 function progressText(label,state){return label+'… '+Math.round(100*Math.min(1,state?.progress||0))+'%';}

 async function findNext(backwards=opts.backward){
  const d=a.current(),blocked=blockedRead(d);if(blocked){status(blocked,true);return false;}
  const s=search();if(!s)return false;
  const area=range(d);if(area.error){status(area.error,true);return false;}
  remember('find',s.raw);
  const sel=a.selection(d),at=backwards?sel.start:sel.end;
  let result;
  try{result=await call(d,'next',{search:s.search,at,within:area.within,backwards,wrap:opts.wrap});}
  catch(error){status(error.message,true);return false;}
  if(a.current()!==d)return false;
  const where=area.within?' in the selection':'';
  if(!result.found){status('Find: Can’t find the text “'+s.raw+'”'+where+'.',true);return false;}
  lastFound={uid:d.uid,revision:a.revision(d),from:result.from,to:result.to};
  await a.select(d,result.from,result.to,false);
  lastFound.revision=a.revision(d);
  refreshFound();
  if(scope&&area.within)scope.revision=lastFound.revision;
  status(result.wrapped?(backwards?'Find: Found the last occurrence from the bottom. The start of the '+(area.within?'selection':'document')+' has been reached.':'Find: Found the first occurrence from the top. The end of the '+(area.within?'selection':'document')+' has been reached.'):'Match on line '+count(result.line+1)+'.');
  return true;
 }

 async function countMatches(){
  const d=a.current(),blocked=blockedRead(d);if(blocked){status(blocked,true);return;}
  const s=search();if(!s)return;
  const area=range(d);if(area.error){status(area.error,true);return;}
  remember('find',s.raw);
  try{
   const state=await job(d,'count',{search:s.search,within:area.within},value=>showProgress(progressText('Counting',value)));
   release(state.job);
   status('Count: '+plural(state.count,'match','matches')+(area.within?' in the selection':'')+'.');
  }catch(error){status(error.message,true);}
 }

 // ---- Find All and the Search results panel ----
 async function findAll(all){
  const d=a.current();
  const s=search();if(!s)return;
  const docs=all?a.documents():[d].filter(Boolean);
  if(!docs.length){status('Open or create a document first.',true);return;}
  const area=all?{within:null}:range(d);if(area.error){status(area.error,true);return;}
  remember('find',s.raw);
  const block={id:++searchSeq,raw:s.raw,tag:modeTag()+(area.within?' [In selection]':''),collapsed:false,files:[],searched:0,done:false};
  for(const doc of docs){
   const blocked=blockedRead(doc);
   block.files.push({uid:doc.uid,name:a.nameOf(doc),title:a.title(doc),job:'',revision:-1,rows:0,hits:0,state:blocked?'skipped':'running',note:blocked,collapsed:false});
  }
  searches.unshift(block);showPanel(true);selection={anchor:0,focus:0};scheduleRender();
  status('Searching for “'+s.raw+'”…',false,false);
  await Promise.all(block.files.map(async(f,index)=>{
   const doc=docs[index];if(f.state==='skipped')return;
   try{
    const state=await job(doc,'all',{search:s.search,within:area.within},value=>{f.rows=value.rows||0;f.hits=value.hits||0;f.progress=value.progress;scheduleRender();showProgress(progressText('Searching',value));});
    Object.assign(f,{job:state.job,revision:state.revision,rows:state.rows||0,hits:state.hits||0,state:'done'});
    block.searched++;
   }catch(error){f.state='failed';f.note=error.message;}
   scheduleRender();
  }));
  block.done=true;
  scheduleRender();
  const hits=block.files.reduce((sum,f)=>sum+f.hits,0),found=block.files.filter(f=>f.hits>0).length;
  const failed=block.files.find(f=>f.state==='failed');
  if(failed&&!all)status(failed.note,true);
  else status('Find All: '+plural(hits,'hit','hits')+' in '+plural(found,'file','files')+' of '+count(block.searched)+' searched.');
 }
 async function pageRows(f,pageIndex){
  const key=f.job+':'+pageIndex;
  if(pages.has(key)){const value=pages.get(key);pages.delete(key);pages.set(key,value);return value;}
  const promise=engine({action:'page',job:f.job,start:pageIndex*PAGE_ROWS,limit:PAGE_ROWS}).then(value=>{pages.set(key,value.items||[]);trimPages();scheduleRender();return value.items||[];},error=>{pages.delete(key);f.note=error.message;scheduleRender();return [];});
  pages.set(key,promise);trimPages();return promise;
 }
 function trimPages(){while(pages.size>PAGE_CACHE){const key=pages.keys().next().value;pages.delete(key);}}
 function cachedRow(f,index){
  const value=pages.get(f.job+':'+Math.floor(index/PAGE_ROWS));
  if(Array.isArray(value))return value[index%PAGE_ROWS]||null;
  if(!value&&f.job)void pageRows(f,Math.floor(index/PAGE_ROWS));
  return null;
 }
 async function rowData(f,index){
  const cached=cachedRow(f,index);if(cached)return cached;
  const rows=await pageRows(f,Math.floor(index/PAGE_ROWS));return rows[index%PAGE_ROWS]||null;
 }
 function fileState(f){
  const d=a.byUid(f.uid);
  if(!d)return 'closed';
  if(f.revision>=0&&a.revision(d)!==f.revision)return 'changed';
  return '';
 }
 function rowLabel(item){
  if(item.type==='search'){
   const s=item.s,hits=s.files.reduce((sum,f)=>sum+f.hits,0),found=s.files.filter(f=>f.hits>0).length,searched=s.files.filter(f=>f.state!=='skipped').length;
   return {html:'<span class="nt-find-toggle" aria-hidden="true">'+(s.collapsed?'▸':'▾')+'</span><strong>Search “'+escapeHTML(s.raw)+'”</strong> ('+plural(hits,'hit','hits')+' in '+plural(found,'file','files')+' of '+count(searched)+' searched)'+(s.done?'':' · searching…')+' <span class="nt-find-tag">'+escapeHTML(s.tag)+'</span>',text:'Search "'+s.raw+'" ('+plural(hits,'hit','hits')+' in '+plural(found,'file','files')+' of '+count(searched)+' searched) '+s.tag};
  }
  if(item.type==='file'){
   const f=item.f,state=fileState(f),note=f.state==='skipped'?' · skipped: '+f.note:f.state==='failed'?' · '+f.note:state==='closed'?' · closed':state==='changed'?' · changed since this search':'';
   return {html:'<span class="nt-find-toggle" aria-hidden="true">'+(f.collapsed?'▸':'▾')+'</span><span class="nt-find-file" title="'+escapeHTML(f.title)+'">'+escapeHTML(f.title)+'</span> ('+plural(f.hits,'hit','hits')+')<span class="nt-find-note">'+escapeHTML(note)+'</span>',text:'  '+f.title+' ('+plural(f.hits,'hit','hits')+')'+note};
  }
  const row=cachedRow(item.f,item.index);
  if(!row)return {html:'<span class="nt-find-line-number">Line …</span>',text:'',pending:true};
  return {html:'<span class="nt-find-line-number">Line '+count(row.line+1)+':</span> <span class="nt-find-line-text">'+rowHTML(row)+'</span>',text:'    Line '+(row.line+1)+': '+row.text,row};
 }
 function scheduleRender(){if(!renderFrame&&panel)renderFrame=root.requestAnimationFrame(render);}
 function render(){
  renderFrame=0;if(!panel||panel.hidden)return;
  const model=layout(searches),scroll=panel.querySelector('[data-ed-find-results-scroll]'),rows=panel.querySelector('.nt-find-results-rows');
  panel.querySelector('.nt-find-results-space').style.height=(model.total*ROW)+'px';
  const first=Math.max(0,Math.floor(scroll.scrollTop/ROW)-2),last=Math.min(model.total,first+Math.ceil((scroll.clientHeight||200)/ROW)+5);
  const low=Math.min(selection.anchor,selection.focus),high=Math.max(selection.anchor,selection.focus);
  let html='';
  for(let index=first;index<last;index++){
   const item=rowAt(model,index);if(!item)continue;
   const label=rowLabel(item),selected=index>=low&&index<=high&&low>=0;
   const level=item.type==='search'?1:item.type==='file'?2:3,expanded=item.type==='search'?!item.s.collapsed:item.type==='file'?!item.f.collapsed:undefined;
   html+='<div class="nt-find-row is-'+item.type+(selected?' is-selected':'')+(index===selection.focus?' is-focus':'')+(label.pending?' is-pending':'')+'" role="treeitem" id="ntFindRow'+index+'" aria-level="'+level+'" aria-selected="'+selected+'"'+(expanded===undefined?'':' aria-expanded="'+expanded+'"')+' data-row="'+index+'" style="top:'+(index*ROW)+'px">'+label.html+'</div>';
  }
  rows.innerHTML=html;
  scroll.setAttribute('aria-activedescendant',selection.focus>=0&&selection.focus>=first&&selection.focus<last?'ntFindRow'+selection.focus:'');
  const hits=searches.reduce((sum,s)=>sum+s.files.reduce((n,f)=>n+f.hits,0),0);
  panel.querySelector('[data-ed-find-results-summary]').textContent=searches.length?plural(searches.length,'search','searches')+', '+plural(hits,'hit','hits'):'No searches';
  syncResultsToggle();
 }
 function compact(n){return n<1000?String(n):n<1e6?(n/1000).toFixed(n<1e4?1:0).replace(/\.0$/,'')+'K':(n/1e6).toFixed(n<1e7?1:0).replace(/\.0$/,'')+'M';}
 function syncResultsToggle(){
  if(!resultsToggle)return;
  const hits=searches.reduce((sum,s)=>sum+s.files.reduce((n,f)=>n+f.hits,0),0);
  resultsToggle.hidden=!searches.length;
  resultsToggle.setAttribute('aria-expanded',String(!panel.hidden));
  resultsToggle.classList.toggle('is-open',!panel.hidden);
  resultsToggle.querySelector('[data-ed-find-badge]').textContent=compact(hits);
  resultsToggle.title=(panel.hidden?'Show':'Hide')+' the search results: '+plural(searches.length,'search','searches')+', '+plural(hits,'hit','hits');
 }
 function showPanel(show){
  if(!panel)return;
  // A small window starts with a smaller panel; a height the user chose stays.
  if(show&&panel.hidden&&!panelSized){panelHeight=Math.round(Math.min(180,Math.max(90,(a.windowEl().clientHeight||600)*0.3)));panel.style.setProperty('--nt-find-results-height',panelHeight+'px');}
  panel.hidden=!show;a.layoutChanged?.();
  if(show)scheduleRender();
  syncResultsToggle();
 }
 function clearResults(){
  for(const s of searches)for(const f of s.files)release(f.job);
  searches.length=0;pages.clear();selection={anchor:-1,focus:-1};panel.querySelector('.nt-find-results-rows').textContent='';showPanel(false);scheduleRender();status('Search results cleared.',false,false);a.focusText();
 }
 function toggleBlock(item,value){
  if(item.type==='search')item.s.collapsed=value??!item.s.collapsed;
  else if(item.type==='file')item.f.collapsed=value??!item.f.collapsed;
  scheduleRender();
 }
 function setAll(collapsed){for(const s of searches){s.collapsed=collapsed;for(const f of s.files)f.collapsed=collapsed;}selection={anchor:0,focus:0};scheduleRender();}
 async function jump(index,hitIndex=0){
  const item=rowAt(layout(searches),index);if(!item)return;
  if(item.type!=='line'){toggleBlock(item);return;}
  const f=item.f,state=fileState(f);
  if(state==='closed'){status(f.name+' was closed. Search again to find its matches.',true);return;}
  if(state==='changed'){status(f.name+' changed since this search. Search again to go to its matches.',true);scheduleRender();return;}
  if(a.comparing()){status('Close the comparison first.',true);return;}
  const row=await rowData(f,item.index);if(!row)return;
  const hit=row.hits[Math.min(hitIndex,row.hits.length-1)]||[row.start,row.start];
  const d=a.byUid(f.uid);if(!d||fileState(f))return;
  if(!a.activate(d)){status('Could not switch to '+f.name+'.',true);return;}
  lastFound={uid:d.uid,revision:a.revision(d),from:hit[0],to:hit[1]};
  await a.select(d,hit[0],hit[1],true);
  lastFound.revision=a.revision(d);refreshFound();
  status('Line '+count(row.line+1)+' of '+f.name+'.',false);
 }
 function selectedIndexes(){const low=Math.min(selection.anchor,selection.focus),high=Math.max(selection.anchor,selection.focus);return low<0?[]:Array.from({length:high-low+1},(_,k)=>low+k);}
 async function copyRows(linesOnly){
  const model=layout(searches),indexes=selectedIndexes();
  if(!indexes.length){status('Select rows in the search results first.',true);return;}
  if(indexes.length>COPY_ROWS){status('Select at most '+count(COPY_ROWS)+' rows to copy.',true);return;}
  const out=[];
  for(const index of indexes){
   const item=rowAt(model,index);if(!item)continue;
   if(item.type==='line'){const row=await rowData(item.f,item.index);if(row)out.push(linesOnly?row.text:'    Line '+(row.line+1)+': '+row.text);}
   else if(!linesOnly)out.push(rowLabel(item).text);
  }
  if(!out.length){status('There are no lines in the selected rows.',true);return;}
  try{await a.copyText(out.join('\n'));status('Copied '+plural(out.length,'line','lines')+'.',false);}catch(error){status(error.message,true);}
 }
 function closeMenu(){menu?.remove();menu=null;}
 function openMenu(x,y){
  closeMenu();
  const items=[['copy','Copy',selectedIndexes().length>0],['copy-lines','Copy selected lines',selectedIndexes().length>0],null,['collapse','Collapse all',searches.length>0],['expand','Expand all',searches.length>0],null,['clear','Clear all',searches.length>0]];
  menu=document.createElement('div');menu.className='nt-editor-tab-menu nt-find-results-menu';menu.setAttribute('role','menu');menu.setAttribute('aria-label','Search results');
  menu.innerHTML=items.map(item=>item?'<button type="button" role="menuitem" tabindex="-1" data-ed-find-menu="'+item[0]+'"'+(item[2]?'':' disabled')+'>'+item[1]+'</button>':'<div role="separator"></div>').join('');
  menu.addEventListener('click',event=>{const item=event.target.closest('[data-ed-find-menu]');if(item&&!item.disabled)runMenu(item.dataset.edFindMenu);});
  menu.addEventListener('keydown',event=>{menuKey(event);event.stopPropagation();});
  a.windowEl().append(menu);
  const rect=menu.getBoundingClientRect();
  menu.style.left=Math.max(6,Math.min(x,root.innerWidth-rect.width-6))+'px';menu.style.top=Math.max(6,Math.min(y,root.innerHeight-rect.height-6))+'px';
  menu.querySelector('button:not(:disabled)')?.focus({preventScroll:true});
 }
 function runMenu(action){
  closeMenu();panel?.querySelector('[data-ed-find-results-scroll]')?.focus({preventScroll:true});
  if(action==='copy')void copyRows(false);else if(action==='copy-lines')void copyRows(true);else if(action==='collapse')setAll(true);else if(action==='expand')setAll(false);else if(action==='clear')clearResults();
 }
 function menuKey(event){
  const items=[...menu.querySelectorAll('button:not(:disabled)')],at=items.indexOf(document.activeElement);
  if(['ArrowDown','ArrowUp','Home','End'].includes(event.key)){event.preventDefault();const next=event.key==='Home'?0:event.key==='End'?items.length-1:(at+(event.key==='ArrowUp'?-1:1)+items.length)%items.length;items[next]?.focus();}
  else if(event.key==='Escape'||event.key==='Tab'){event.preventDefault();closeMenu();panel?.querySelector('[data-ed-find-results-scroll]')?.focus({preventScroll:true});}
  return true;
 }
 function moveSelection(index,extend){
  const model=layout(searches);if(!model.total)return;
  index=Math.max(0,Math.min(model.total-1,index));
  selection={anchor:extend?selection.anchor<0?index:selection.anchor:index,focus:index};
  const scroll=panel.querySelector('[data-ed-find-results-scroll]');
  if(index*ROW<scroll.scrollTop)scroll.scrollTop=index*ROW;else if((index+1)*ROW>scroll.scrollTop+scroll.clientHeight)scroll.scrollTop=(index+1)*ROW-scroll.clientHeight;
  scheduleRender();
 }
 function panelKey(event){
  if(menu&&menu.contains(event.target))return menuKey(event);
  const ctrl=event.ctrlKey||event.metaKey,model=layout(searches),page=Math.max(1,Math.floor((panel.querySelector('[data-ed-find-results-scroll]').clientHeight||ROW)/ROW)-1);
  const item=selection.focus>=0?rowAt(model,selection.focus):null;
  if(event.key==='ContextMenu'||event.shiftKey&&event.key==='F10'){event.preventDefault();const row=panel.querySelector('.nt-find-row.is-focus')||panel.querySelector('[data-ed-find-results-scroll]'),rect=row.getBoundingClientRect();openMenu(rect.left+12,rect.bottom);return true;}
  if(ctrl&&event.key.toLowerCase()==='c'){event.preventDefault();void copyRows(false);return true;}
  if(ctrl&&event.key.toLowerCase()==='a'){event.preventDefault();selection={anchor:0,focus:model.total-1};scheduleRender();return true;}
  const moves={ArrowDown:1,ArrowUp:-1,PageDown:page,PageUp:-page};
  if(Object.hasOwn(moves,event.key)){event.preventDefault();moveSelection((selection.focus<0?-1:selection.focus)+moves[event.key],event.shiftKey);return true;}
  if(event.key==='Home'||event.key==='End'){event.preventDefault();moveSelection(event.key==='Home'?0:model.total-1,event.shiftKey);return true;}
  if(event.key==='Enter'&&item){event.preventDefault();void jump(selection.focus);return true;}
  if((event.key==='ArrowLeft'||event.key==='ArrowRight')&&item){
   event.preventDefault();
   const collapse=event.key==='ArrowLeft';
   if(item.type==='line'||item.type==='file'&&collapse&&item.f.collapsed){const parent=model.blocks.find(block=>block.type===(item.type==='line'?'file':'search')&&block.s===item.s&&(item.type!=='line'||block.f===item.f));if(parent&&collapse)moveSelection(parent.at,false);}
   else toggleBlock(item,collapse);
   return true;
  }
  if(event.key==='Escape'){event.preventDefault();a.focusText();return true;}
  return false;
 }

 // ---- Replace ----
 const writeBlocked=d=>!d?'Open or create a document first.':a.writeBlocked(d);
 async function replaceOne(){
  const d=a.current(),blocked=writeBlocked(d);if(blocked){status(blocked,true);return;}
  const s=search();if(!s)return;
  const r=replacementText();if(r.error){status(r.error,true);return;}
  const area=range(d);if(area.error){status(area.error,true);return;}
  remember('find',s.raw);if(r.raw)remember('replace',r.raw);
  const sel=a.selection(d);
  if(sel.end>sel.start){
   let result;
   try{result=await call(d,'replace-text',{search:s.search,replacement:r.text,from:sel.start,to:sel.end,revision:a.revision(d)});}
   catch(error){status(error.message,true);return;}
   if(a.current()!==d)return;
   if(result.found){
    const before=a.revision(d);
    try{await a.replaceSelection(d,result.text);}catch(error){status(error.message,true);return;}
    if(scope&&scope.uid===d.uid&&scope.revision===before&&area.within){scope.to+=result.text.length-(sel.end-sel.start);scope.revision=a.revision(d);}
    const after=a.selection(d);
    await a.select(d,after.end,after.end,false);
    if(scope&&area.within)scope.revision=a.revision(d);
    // The caret after the replacement stands for the match: In selection
    // goes on in the same range.
    lastFound={uid:d.uid,revision:a.revision(d),from:after.end,to:after.end};
    if(await findNext(false))return;
    status('Replace: 1 occurrence was replaced. No more matches'+(area.within?' in the selection':'')+'.',false);
    return;
   }
  }
  await findNext(false);
 }
 async function replaceIn(d,s,r,within){
  if(a.disk(d)){
   a.diskBusy(d,true);
   try{
    const state=await job(d,'replace',{search:s.search,replacement:r.text,within,revision:a.revision(d)},value=>showProgress(progressText('Replacing',value)));
    release(state.job);
    await a.diskReplaced(d,state.metadata,state.count);
    return {count:state.count,from:state.from,to:state.to};
   }finally{a.diskBusy(d,false);}
  }
  const before=a.text(d),result=await engine({action:'replace',text:before,search:s.search,replacement:r.text,within});
  if(!result.changed)return {count:result.count,from:within?.[0],to:within?.[1]};
  if(a.text(d)!==before)throw Error(a.nameOf(d)+' changed during Replace All. Nothing was replaced.');
  const to=within?within[1]+result.text.length-before.length:null;
  if(!a.setText(d,result.text,within?within[0]:null,to))throw Error('The result of Replace All could not be applied to '+a.nameOf(d)+'.');
  return {count:result.count,from:within?.[0],to};
 }
 async function replaceAll(){
  const d=a.current(),blocked=writeBlocked(d);if(blocked){status(blocked,true);return;}
  const s=search();if(!s)return;
  const r=replacementText();if(r.error){status(r.error,true);return;}
  const area=range(d);if(area.error){status(area.error,true);return;}
  remember('find',s.raw);if(r.raw)remember('replace',r.raw);
  try{
   const result=await replaceIn(d,s,r,area.within);
   if(area.within&&result.count){scope={uid:d.uid,revision:a.revision(d),from:result.from,to:result.to};await a.select(d,result.from,result.to,false);lastFound=null;}
   status(result.count?'Replace All: '+plural(result.count,'occurrence was','occurrences were')+' replaced in '+(area.within?'the selection':'the entire file')+'.':'Replace All: no occurrences were replaced'+(area.within?' in the selection':'')+'.');
  }catch(error){status(error.message,true);}
 }
 async function replaceAllOpen(){
  const s=search();if(!s)return;
  const r=replacementText();if(r.error){status(r.error,true);return;}
  remember('find',s.raw);if(r.raw)remember('replace',r.raw);
  let total=0;const changed=[],skipped=[],failed=[];
  for(const d of a.documents()){
   const blocked=writeBlocked(d);
   if(blocked){skipped.push(a.nameOf(d)+' ('+a.skipReason(d)+')');continue;}
   try{const result=await replaceIn(d,s,r,null);if(result.count){total+=result.count;changed.push(a.nameOf(d));}}
   catch(error){failed.push(a.nameOf(d)+' ('+error.message.replace(/\.$/,'')+')');}
  }
  a.refresh();
  let text='Replace All in all opened documents: '+plural(total,'occurrence was','occurrences were')+' replaced in '+plural(changed.length,'document','documents')+'.';
  if(changed.length)text+=' Nothing was saved: the changed documents show as unsaved, and Undo takes back each one.';
  if(skipped.length)text+=' Skipped: '+skipped.join(', ')+'.';
  if(failed.length)text+=' Not replaced: '+failed.join(', ')+'.';
  status(text,failed.length>0);
 }

 // ---- Mark ----
 function marksOf(d){return d&&marks.get(d)||[];}
 async function markAll(){
  const d=a.current(),blocked=blockedRead(d);if(blocked){status(blocked,true);return;}
  const s=search();if(!s)return;
  const area=range(d);if(area.error){status(area.error,true);return;}
  remember('find',s.raw);
  try{
   const state=await job(d,'mark',{search:s.search,within:area.within},value=>showProgress(progressText('Marking',value)));
   release(state.job);
   if(opts.purge){marks.delete(d);if(opts.bookmark)bookmarks.delete(d);}
   const list=marksOf(d).filter(mark=>JSON.stringify(mark.search)!==JSON.stringify(s.search)||JSON.stringify(mark.within)!==JSON.stringify(area.within));
   if(state.count)list.push({search:s.search,within:area.within});
   while(list.length>MARK_PATTERNS)list.shift();
   if(list.length)marks.set(d,list);else marks.delete(d);
   let note='';
   if(opts.bookmark&&state.lines?.length){
    const merged=[...new Set([...(bookmarks.get(d)||[]),...state.lines])].sort((x,y)=>x-y);
    if(merged.length>BOOKMARKS){merged.length=BOOKMARKS;note=' Bookmarked the first '+count(BOOKMARKS)+' lines.';}
    else if(state.more)note=' Bookmarked the first '+count(BOOKMARKS)+' lines.';
    bookmarks.set(d,merged);
   }
   markVersion++;markCache.delete(d);a.repaint(d);
   status('Mark: '+plural(state.count,'match','matches')+(area.within?' in the selection':'')+'.'+note);
  }catch(error){status(error.message,true);}
 }
 function clearMarks(){const d=a.current();if(!d)return;marks.delete(d);markVersion++;markCache.delete(d);a.repaint(d);status('Marks cleared.');}
 async function copyMarked(){
  const d=a.current(),blocked=blockedRead(d);if(blocked){status(blocked,true);return;}
  const list=marksOf(d);if(!list.length){status('Nothing is marked in this document. Use Mark All first.',true);return;}
  const parts=[];let total=0;
  try{
   for(const mark of list){
    const state=await job(d,'copy',{search:mark.search,within:mark.within,separator:a.eol(d)},value=>showProgress(progressText('Reading marked text',value)));
    release(state.job);if(state.count){parts.push(state.text);total+=state.count;}
   }
   if(!total){status('No marked text is left in this document.',true);return;}
   await a.copyText(parts.join(a.eol(d)));
   status('Copied the text of '+plural(total,'mark','marks')+'.');
  }catch(error){status(error.message,true);}
 }
 // The marks of [from, to) of a document, for painting: an array of
 // [from, to] when known; otherwise null, and a repaint follows once they
 // are found. Each mark pattern searches only that part of the document.
 // The match Find Next selected, while it is still the selection: shown as
 // the current match, also when the text does not have the focus (a text
 // area hides its selection then).
 let foundShown='',foundBase=null,foundKey='',foundRanges=null;
 function currentMatch(d){
  if(!d||!lastFound||lastFound.uid!==d.uid||lastFound.to<=lastFound.from||a.current()!==d||lastFound.revision!==a.revision(d))return null;
  const sel=a.selection(d);
  return sel.start===lastFound.from&&sel.end===lastFound.to?lastFound:null;
 }
 function refreshFound(){
  const d=a.current(),found=currentMatch(d),key=found?d.uid+':'+found.revision+':'+found.from+':'+found.to:'';
  if(key===foundShown)return;
  foundShown=key;if(d)a.repaint(d,true);
 }
 function marksIn(d,from,to){
  const base=patternMarks(d,from,to),found=currentMatch(d);
  if(!found||found.to<=from||found.from>=to)return base;
  const key=found.from+':'+found.to;
  if(foundBase!==base||foundKey!==key){
   foundBase=base;foundKey=key;
   const others=(base||[]).filter(([x,y])=>y<=found.from||x>=found.to);
   foundRanges=[...others,[found.from,found.to,'current']].sort((x,y)=>x[0]-y[0]);
  }
  return foundRanges;
 }
 function patternMarks(d,from,to){
  const list=marksOf(d);if(!list.length)return NONE;
  const key=[a.revision(d),from,to,markVersion].join(':'),cached=markCache.get(d);
  if(cached?.key===key)return cached.ranges;
  if(cached?.pending===key)return cached.ranges||null;
  const entry={pending:key,ranges:cached?.ranges||null};markCache.set(d,entry);
  void (async()=>{
   const ranges=[];
   for(const mark of list){
    let lo=from,hi=to;if(mark.within){lo=Math.max(lo,mark.within[0]);hi=Math.min(hi,mark.within[1]);}
    if(hi<lo)continue;
    const body={search:mark.search,from:lo,to:hi,within:mark.within};
    const value=a.disk(d)?await a.diskCall(d,'find-matches',body):await engine({action:'matches',...body,...textBody(d)});
    for(const hit of value.hits||[])ranges.push([hit.from,hit.to]);
   }
   ranges.sort((x,y)=>x[0]-y[0]);
   if(markCache.get(d)===entry){markCache.set(d,{key,ranges});a.repaint(d,true);}
  })().catch(()=>{if(markCache.get(d)===entry)markCache.set(d,{key,ranges:[]});});
  return entry.ranges;
 }

 // ---- Bookmarks ----
 function toggleBookmark(){
  const d=a.current();if(!d)return;
  const line=a.caretLine(d);if(line<0)return;
  const list=[...(bookmarks.get(d)||[])],at=list.indexOf(line);
  if(at>=0)list.splice(at,1);
  else{if(list.length>=BOOKMARKS){a.setMessage('A document keeps at most '+count(BOOKMARKS)+' bookmarks.',true);return;}list.push(line);list.sort((x,y)=>x-y);}
  if(list.length)bookmarks.set(d,list);else bookmarks.delete(d);
  a.repaint(d);a.setMessage(at>=0?'Bookmark removed from line '+count(line+1)+'.':'Bookmarked line '+count(line+1)+'.');
 }
 async function nextBookmark(back){
  const d=a.current(),list=d&&bookmarks.get(d);
  if(!list?.length){a.setMessage('There are no bookmarks in this document. Ctrl+F2 bookmarks the cursor’s line.');return;}
  const line=a.caretLine(d);
  let target=back?[...list].reverse().find(value=>value<line):list.find(value=>value>line);
  const wrapped=target===undefined;if(wrapped)target=back?list.at(-1):list[0];
  await a.goLine(d,target);
  a.setMessage('Bookmark on line '+count(target+1)+(wrapped&&list.length>1?' (wrapped around).':'.'));
 }
 function edited(d,edit){
  const list=bookmarks.get(d);
  if(list?.length){
   const next=edit.bulk?list.filter(line=>line<edit.lines):shiftLines(list,edit);
   if(next.length)bookmarks.set(d,next);else bookmarks.delete(d);
  }
 }

 // ---- The syntax guide and the line under Find what ----
 function renderGuide(){
  const guide=el?.querySelector('[data-ed-find-guide]');if(!guide||guide.hidden)return;
  const rows=(list,field)=>'<div class="nt-find-guide-rows">'+list.map(([token,insert,meaning,example])=>'<button type="button" class="nt-find-guide-row" data-ed-find-insert="'+escapeHTML(insert)+'"'+(field?' data-ed-find-field="'+field+'"':'')+' title="Insert '+escapeHTML(insert.replace('‸',''))+' at the cursor"><code>'+escapeHTML(token)+'</code><span>'+escapeHTML(meaning)+'</span><small>'+escapeHTML(example)+'</small></button>').join('')+'</div>';
  let html;
  if(opts.mode==='normal')html='<p>Normal finds the text exactly as typed. Choose Extended or Regular expression to see what their characters mean here.</p>';
  else if(opts.mode==='extended')html='<p>Extended: these stand for other characters in Find what and Replace with. Click one to insert it.</p>'+rows(GUIDE.extended)+'<p class="nt-find-guide-note">\\0 is refused: no text in the editor can hold a NUL character.</p>';
  else html='<p>Regular expression: click one to insert it at the cursor. The same rules apply to every document.</p>'+rows(GUIDE.regex,'find')+(tab==='replace'?'<p><strong>In Replace with</strong></p>'+rows(GUIDE.replace,'replace'):'')+'<p class="nt-find-guide-note">'+escapeHTML(NOT_SUPPORTED)+'</p>';
  guide.innerHTML=html;
 }
 function insertToken(text,field){
  const input=el.querySelector(field==='replace'||!field&&lastField==='replace'?'[data-ed-replacement]':'[data-ed-query]');
  const caret=text.indexOf('‸'),value=text.replace('‸','');
  const start=input.selectionStart??input.value.length,end=input.selectionEnd??start;
  input.focus({preventScroll:true});
  input.setRangeText(value,start,end,'end');
  if(caret>=0)input.setSelectionRange(start+caret,start+caret);
  input.dispatchEvent(new Event('input',{bubbles:true}));
 }
 // One quiet line under Find what: what the token at the cursor means, or
 // what is wrong with the pattern, and where. Hidden in Normal mode.
 function explainCaret(changed=false){
  const line=el?.querySelector('[data-ed-find-explain]');if(!line)return;
  if(opts.mode==='normal'){line.hidden=true;line.textContent='';patternProblem=null;return;}
  line.hidden=false;
  const field=lastField==='replace'&&tab==='replace'?'replace':'find';
  const input=el.querySelector(field==='replace'?'[data-ed-replacement]':'[data-ed-query]'),text=input.value;
  if(changed&&field==='find')checkPattern();
  const problem=field==='find'?patternProblem:null;
  if(problem&&problem.text===text){line.textContent='Problem at character '+count(problem.at+1)+': '+problem.error;line.classList.add('error');return;}
  line.classList.remove('error');
  if(opts.mode==='extended'){const value=decode(text,'extended');if(value.error){line.textContent=value.error;line.classList.add('error');return;}}
  const meaning=explain(text,input.selectionStart??text.length,opts.mode,field);
  line.textContent=meaning||(field==='replace'?'Replace with: what each match becomes.':'Type a pattern; the meaning of each part shows here.');
 }
 // Whether the engine can use the regular expression (it searches every
 // document), checked while it is typed.
 function checkPattern(){
  root.clearTimeout(checkTimer);
  const text=el.querySelector('[data-ed-query]').value,ticket=++checkTicket;
  if(opts.mode!=='regex'||!text||!a.engine()){patternProblem=null;return;}
  checkTimer=root.setTimeout(()=>{void engine({action:'check',search:{query:text,regex:true}}).then(value=>{
   if(ticket!==checkTicket)return;
   patternProblem=value.ok?null:{text,error:value.error,at:value.at};
   explainCaret();
  }).catch(()=>{});},180);
 }

 // ---- The window ----
 function markup(){
  const tabs=TABS.map(([id,label])=>'<button type="button" role="tab" id="ntFindTab-'+id+'" data-ed-find-tab="'+id+'" aria-controls="ntFindPanel" aria-selected="false" tabindex="-1">'+label+'</button>').join('');
  const check=(option,label,extra='',tabs='')=>'<label class="nt-find-check"'+(tabs?' data-tabs="'+tabs+'"':'')+'><input type="checkbox" data-ed-find-option="'+option+'"'+extra+'> '+label+'</label>';
  return '<div class="nt-find-head" data-ed-find-drag title="Drag to move the Find window"><div class="nt-find-tabs" role="tablist" aria-label="Find, Replace or Mark">'+tabs+'</div><button type="button" class="nt-find-x" data-ed-find-close aria-label="Close the Find window" title="Close (Esc)">×</button></div>'+
  '<div class="nt-find-panel" id="ntFindPanel" role="tabpanel" aria-labelledby="ntFindTab-find">'+
   '<div class="nt-find-main"><div class="nt-find-fields">'+
    '<label class="nt-find-field"><span>Find what:</span><span class="nt-find-input"><input type="text" data-ed-query list="ntEditorFindHistory" spellcheck="false" autocomplete="off" autocapitalize="off" aria-label="Find what" aria-describedby="ntFindExplain"></span></label><datalist id="ntEditorFindHistory"></datalist>'+
    '<div class="nt-find-explain" id="ntFindExplain" data-ed-find-explain aria-live="polite" hidden></div>'+
    '<div class="nt-find-field" data-ed-replace-fields data-tabs="replace"><label for="ntFindReplacement">Replace with:</label><span class="nt-find-input"><input type="text" id="ntFindReplacement" data-ed-replacement list="ntEditorReplaceHistory" spellcheck="false" autocomplete="off" autocapitalize="off" aria-label="Replace with"></span>'+
     '<span class="nt-find-swap"><button type="button" class="btn small ghost" data-ed-find-swap="swap" title="Swap Find what and Replace with" aria-label="Swap Find what and Replace with">⇅</button><button type="button" class="btn small ghost nt-find-swap-more" data-ed-find-swap-menu aria-haspopup="menu" aria-expanded="false" aria-label="More swap choices" title="More swap choices">▾</button>'+
     '<span class="nt-find-swap-list" role="menu" aria-label="Swap" hidden><button type="button" role="menuitem" data-ed-find-swap="swap">Swap Find and Replace</button><button type="button" role="menuitem" data-ed-find-swap="to-replace">Copy Find to Replace</button><button type="button" role="menuitem" data-ed-find-swap="to-find">Copy Replace to Find</button></span></span></div><datalist id="ntEditorReplaceHistory"></datalist>'+
   '</div>'+
   '<div class="nt-find-options">'+
    check('bookmark','Bookmark line','','mark')+check('purge','Purge for each search','','mark')+
    check('backward','Backward direction',' aria-keyshortcuts="Shift+F3"','find replace')+check('word','Match whole word only',' data-ed-word')+check('matchCase','Match case',' data-ed-case')+check('wrap','Wrap around',' checked','find replace')+
   '</div></div>'+
   '<fieldset class="nt-find-mode" data-ed-search-mode data-ed-search-mode-label title="'+escapeHTML(MODE_HELP.normal)+'"><legend>Search Mode <button type="button" class="nt-find-guide-toggle" data-ed-find-guide-toggle aria-expanded="false" aria-controls="ntFindGuide" aria-label="Syntax guide" title="What the characters of this search mode mean">?</button></legend>'+
    MODES.map(([value,label])=>'<label class="nt-find-check" title="'+escapeHTML(MODE_HELP[value])+'"><input type="radio" name="ntFindMode" value="'+value+'"'+(value==='normal'?' checked':'')+'> '+escapeHTML(label)+'</label>').join('')+
    check('dotAll','. matches newline',' disabled')+
   '</fieldset>'+
   '<div class="nt-find-buttons">'+
    '<label class="nt-find-check nt-find-in-selection"><input type="checkbox" data-ed-find-option="inSelection" disabled> In selection</label>'+
    '<button type="button" class="btn small primary" data-ed="next" data-tabs="find replace" aria-keyshortcuts="F3" title="Find Next (F3; Shift+F3 finds the previous)">Find Next</button>'+
    '<button type="button" class="btn small ghost" data-ed-find-action="count" data-tabs="find">Count</button>'+
    '<button type="button" class="btn small ghost" data-ed-find-action="all-current" data-tabs="find">Find All in Current Document</button>'+
    '<button type="button" class="btn small ghost" data-ed-find-action="all-open" data-tabs="find">Find All in All Opened Documents</button>'+
    '<button type="button" class="btn small ghost" data-ed="replaceOne" data-tabs="replace">Replace</button>'+
    '<button type="button" class="btn small ghost" data-ed="replaceAll" data-tabs="replace">Replace All</button>'+
    '<button type="button" class="btn small ghost" data-ed-find-action="replace-open" data-tabs="replace" title="Replaces in every open document that can change; nothing is saved">Replace All in All Opened Documents</button>'+
    '<button type="button" class="btn small primary" data-ed-find-action="mark" data-tabs="mark">Mark All</button>'+
    '<button type="button" class="btn small ghost" data-ed-find-action="clear-marks" data-tabs="mark">Clear all marks</button>'+
    '<button type="button" class="btn small ghost" data-ed-find-action="copy-marks" data-tabs="mark">Copy Marked Text</button>'+
   '</div>'+
   '<fieldset class="nt-find-transparency"><legend><label class="nt-find-check"><input type="checkbox" data-ed-find-transparency> Transparency</label></legend>'+
    '<label class="nt-find-check"><input type="radio" name="ntFindTransparency" value="blur" data-ed-find-transparency-mode checked> On losing focus</label>'+
    '<label class="nt-find-check"><input type="radio" name="ntFindTransparency" value="always" data-ed-find-transparency-mode> Always</label>'+
    '<input type="range" min="20" max="95" step="5" value="70" data-ed-find-opacity aria-label="Opacity of the Find window when it is transparent" title="Opacity when transparent">'+
   '</fieldset>'+
   '<div class="nt-find-guide" id="ntFindGuide" data-ed-find-guide role="region" aria-label="Syntax guide" hidden></div>'+
  '</div>'+
  '<div class="nt-find-status" role="status" aria-live="polite"><span data-ed-find-message></span><span class="nt-find-progress" data-ed-find-progress hidden><span data-ed-find-progress-text></span><button type="button" class="btn small ghost" data-ed-find-cancel>Cancel</button></span></div>';
 }
 function panelMarkup(){
  return '<div class="nt-editor-pane-resize nt-find-results-resize" data-ed-find-results-resize role="separator" aria-orientation="horizontal" aria-label="Resize search results" tabindex="0" title="Drag to resize the search results. Arrow keys resize; Home / End use limits; Enter or double-click resets."></div>'+
  '<div class="nt-find-results-head"><strong id="ntFindResultsTitle">Search results</strong><span class="nt-find-results-summary" data-ed-find-results-summary></span><button type="button" class="btn small ghost" data-ed-find-results-cancel hidden>Cancel search</button></div>'+
  '<div class="nt-find-results-scroll" data-ed-find-results-scroll tabindex="0" role="tree" aria-labelledby="ntFindResultsTitle" aria-multiselectable="true"><div class="nt-find-results-space"></div><div class="nt-find-results-rows"></div></div>';
 }
 function mount(){
  if(el)return el;
  const host=a.windowEl();if(!host)return null;
  el=document.createElement('section');el.className='nt-find-window';el.dataset.edFind='';el.setAttribute('role','dialog');el.setAttribute('aria-modal','false');el.setAttribute('aria-label','Find');el.hidden=true;el.dataset.tab='find';
  el.innerHTML=markup();host.append(el);
  panel=document.createElement('section');panel.className='nt-find-results';panel.id='ntFindResults';panel.dataset.edFindResults='';panel.setAttribute('role','region');panel.setAttribute('aria-label','Search results');panel.hidden=true;panel.innerHTML=panelMarkup();
  const statusBar=host.querySelector('.nt-editor-status');if(statusBar)statusBar.before(panel);else host.append(panel);
  // The panel's toggle sits in the status bar, shown once there are results.
  resultsToggle=document.createElement('button');resultsToggle.type='button';resultsToggle.className='nt-find-results-toggle';resultsToggle.dataset.edFindResultsToggle='';resultsToggle.hidden=true;
  resultsToggle.setAttribute('aria-controls','ntFindResults');resultsToggle.setAttribute('aria-expanded','false');
  resultsToggle.innerHTML='Results <span class="nt-find-badge" data-ed-find-badge></span>';
  resultsToggle.addEventListener('click',()=>{showPanel(panel.hidden);if(panel.hidden)a.focusText();});
  statusBar?.querySelector('[data-ed-message]')?.after(resultsToggle);
  panel.style.setProperty('--nt-find-results-height',panelHeight+'px');
  bindWindow();bindPanel();syncControls();applyTransparency();fillHistory();
  return el;
 }
 function syncControls(){
  if(!el)return;
  for(const input of el.querySelectorAll('[data-ed-find-option]'))input.checked=!!opts[input.dataset.edFindOption];
  for(const radio of el.querySelectorAll('input[name="ntFindMode"]'))radio.checked=radio.value===opts.mode;
  el.querySelector('[data-ed-find-option="dotAll"]').disabled=opts.mode!=='regex';
  el.querySelector('[data-ed-search-mode]').title=MODE_HELP[opts.mode];
  el.querySelector('[data-ed-find-transparency]').checked=transparency.enabled;
  for(const radio of el.querySelectorAll('[data-ed-find-transparency-mode]')){radio.checked=radio.value===transparency.mode;radio.disabled=!transparency.enabled;}
  const slider=el.querySelector('[data-ed-find-opacity]');slider.value=String(transparency.level);slider.disabled=!transparency.enabled;
  selectionChanged();
 }
 function applyTransparency(){
  if(!el)return;
  const away=!el.contains(document.activeElement);
  el.style.opacity=transparency.enabled&&(transparency.mode==='always'||away)?String(transparency.level/100):'';
 }
 function setTab(next,focus=true){
  if(!TABS.some(([id])=>id===next))next='find';
  tab=next;if(!el)return;
  el.dataset.tab=next;el.setAttribute('aria-label',TABS.find(([id])=>id===next)[1]);
  for(const button of el.querySelectorAll('[data-ed-find-tab]')){const on=button.dataset.edFindTab===next;button.setAttribute('aria-selected',String(on));button.tabIndex=on?0:-1;}
  el.querySelector('.nt-find-panel').setAttribute('aria-labelledby','ntFindTab-'+next);
  syncToolbar();renderGuide();
  if(focus){const input=el.querySelector('[data-ed-query]');input.focus({preventScroll:true});input.select();}
  place();
 }
 function place(){
  if(!el||el.hidden)return;
  const host=a.windowEl(),box=host.getBoundingClientRect(),rect=el.getBoundingClientRect();
  if(!pos){const area=host.querySelector('.nt-editor-document-area')?.getBoundingClientRect()||box;pos={left:area.right-box.left-rect.width-28,top:area.top-box.top+34};}
  pos.left=Math.max(4,Math.min(pos.left,box.width-rect.width-4));pos.top=Math.max(4,Math.min(pos.top,box.height-Math.min(rect.height,box.height-8)-4));
  el.style.left=pos.left+'px';el.style.top=pos.top+'px';
 }
 async function open(next=tab){
  if(!mount())return;
  if(a.comparing()){a.setMessage('Find is not available while documents are compared. Close the comparison first.',true);return;}
  const d=a.current(),input=el.querySelector('[data-ed-query]');
  const wasHidden=el.hidden;el.hidden=false;
  if(d){
   const sel=a.selection(d);
   if(sel.end>sel.start&&sel.end-sel.start<=1024){
    try{const text=await a.selectionText(d);if(text&&!text.includes('\n'))input.value=text;}catch{}
   }
  }
  setTab(next,true);
  if(wasHidden)status('',false,false);
  syncControls();applyTransparency();void loadHistory();explainCaret();
 }
 function close(){
  if(!el||el.hidden)return;
  closeSwapMenu();el.hidden=true;syncToolbar();a.focusText();
 }
 // The toolbar's Find and Replace, and Ctrl+F, Ctrl+H and Ctrl+M, open the
 // window on their tab; the one of the tab that shows closes it.
 function toggle(next){
  if(el&&!el.hidden&&tab===next){close();return;}
  void open(next);
 }
 function syncToolbar(){
  const host=a.windowEl(),shown=!!el&&!el.hidden;
  for(const [action,id] of [['find','find'],['replace','replace']]){
   const button=host?.querySelector('.nt-editor-toolbar [data-ed="'+action+'"]');if(!button)continue;
   const on=shown&&tab===id;button.setAttribute('aria-pressed',String(on));button.setAttribute('aria-expanded',String(on));button.setAttribute('aria-controls','ntFindPanel');
   button.title=(on?'Close the Find window':'Open the Find window on '+(id==='find'?'Find':'Replace'))+' ('+(id==='find'?'Ctrl+F':'Ctrl+H')+')';
  }
 }
 function closeSwapMenu(){const list=el?.querySelector('.nt-find-swap-list');if(list&&!list.hidden){list.hidden=true;el.querySelector('[data-ed-find-swap-menu]').setAttribute('aria-expanded','false');}}
 function swap(kind){
  const find=el.querySelector('[data-ed-query]'),replace=el.querySelector('[data-ed-replacement]');
  if(kind==='to-replace')replace.value=find.value;
  else if(kind==='to-find')find.value=replace.value;
  else [find.value,replace.value]=[replace.value,find.value];
  closeSwapMenu();
 }
 function selectionChanged(){
  refreshFound();
  if(!el)return;
  const d=a.current(),box=el.querySelector('[data-ed-find-option="inSelection"]');
  if(!box)return;
  const sel=d?a.selection(d):{start:0,end:0};
  const keep=scope&&d&&scope.uid===d.uid&&lastFound&&sel.start===lastFound.from&&sel.end===lastFound.to;
  box.disabled=!(sel.end>sel.start)&&!keep;
  if(box.disabled&&opts.inSelection){opts.inSelection=false;box.checked=false;}
 }
 async function action(name){
  if(name==='count')return countMatches();
  if(name==='all-current')return findAll(false);
  if(name==='all-open')return findAll(true);
  if(name==='replace-open')return replaceAllOpen();
  if(name==='mark')return markAll();
  if(name==='clear-marks')return clearMarks();
  if(name==='copy-marks')return copyMarked();
 }
 function bindWindow(){
  el.addEventListener('click',event=>{
   const tabButton=event.target.closest('[data-ed-find-tab]');if(tabButton){setTab(tabButton.dataset.edFindTab,true);return;}
   if(event.target.closest('[data-ed-find-close]')){close();return;}
   if(event.target.closest('[data-ed-find-swap-menu]')){const list=el.querySelector('.nt-find-swap-list'),openNow=list.hidden;list.hidden=!openNow;event.target.closest('[data-ed-find-swap-menu]').setAttribute('aria-expanded',String(openNow));if(openNow)list.querySelector('button').focus();return;}
   const swapButton=event.target.closest('[data-ed-find-swap]');if(swapButton){swap(swapButton.dataset.edFindSwap);return;}
   if(event.target.closest('[data-ed-find-cancel]')){for(const entry of running)entry.cancel();return;}
   const guideToggle=event.target.closest('[data-ed-find-guide-toggle]');
   if(guideToggle){const guide=el.querySelector('[data-ed-find-guide]');guide.hidden=!guide.hidden;guideToggle.setAttribute('aria-expanded',String(!guide.hidden));renderGuide();place();return;}
   const row=event.target.closest('[data-ed-find-insert]');if(row){insertToken(row.dataset.edFindInsert,row.dataset.edFindField);return;}
   const act=event.target.closest('[data-ed-find-action]');if(act&&!act.disabled)void action(act.dataset.edFindAction);
  });
  el.addEventListener('change',event=>{
   const t=event.target;
   if(t.matches('[data-ed-find-option]')){opts[t.dataset.edFindOption]=t.checked;if(t.dataset.edFindOption==='inSelection'&&t.checked){scope=null;lastFound=null;}syncControls();return;}
   if(t.matches('input[name="ntFindMode"]')){opts.mode=t.value;syncControls();renderGuide();explainCaret(true);return;}
   if(t.matches('[data-ed-find-transparency]')){transparency.enabled=t.checked;syncControls();applyTransparency();a.savePreferences();return;}
   if(t.matches('[data-ed-find-transparency-mode]')){transparency.mode=t.value==='always'?'always':'blur';applyTransparency();a.savePreferences();return;}
   if(t.matches('[data-ed-find-opacity]')){transparency.level=Math.max(20,Math.min(95,Number(t.value)||70));applyTransparency();a.savePreferences();}
  });
  for(const type of ['input','keyup','click','focusin','select'])el.addEventListener(type,event=>{
   if(!event.target.matches?.('[data-ed-query],[data-ed-replacement]'))return;
   lastField=event.target.matches('[data-ed-replacement]')?'replace':'find';
   explainCaret(type==='input');
  });
  el.addEventListener('input',event=>{if(event.target.matches('[data-ed-find-opacity]')){transparency.level=Math.max(20,Math.min(95,Number(event.target.value)||70));applyTransparency();}});
  el.addEventListener('focusin',applyTransparency);
  el.addEventListener('focusout',()=>root.setTimeout(applyTransparency,0));
  el.addEventListener('pointerdown',event=>{
   if(!event.target.closest('.nt-find-swap'))closeSwapMenu();
   const head=event.target.closest('[data-ed-find-drag]');
   if(!head||event.button!==0||event.target.closest('button,input'))return;
   event.preventDefault();
   const start={x:event.clientX,y:event.clientY,left:pos?.left||0,top:pos?.top||0};
   head.setPointerCapture?.(event.pointerId);el.classList.add('is-moving');
   const move=e=>{pos={left:start.left+e.clientX-start.x,top:start.top+e.clientY-start.y};place();};
   const end=()=>{head.removeEventListener('pointermove',move);head.removeEventListener('pointerup',end);head.removeEventListener('pointercancel',end);el.classList.remove('is-moving');};
   head.addEventListener('pointermove',move);head.addEventListener('pointerup',end);head.addEventListener('pointercancel',end);
  });
  if(typeof root.ResizeObserver==='function')new root.ResizeObserver(()=>place()).observe(a.windowEl());
 }
 function bindPanel(){
  const scroll=panel.querySelector('[data-ed-find-results-scroll]');
  scroll.addEventListener('scroll',scheduleRender);
  if(typeof root.ResizeObserver==='function')new root.ResizeObserver(scheduleRender).observe(scroll);
  panel.addEventListener('click',event=>{
   if(event.target.closest('[data-ed-find-results-cancel]')){for(const entry of running)entry.cancel();return;}
   const row=event.target.closest('[data-row]');if(!row)return;
   const index=Number(row.dataset.row);
   if(event.target.closest('.nt-find-toggle')){const item=rowAt(layout(searches),index);if(item)toggleBlock(item);}
   if(event.shiftKey&&selection.anchor>=0)selection={anchor:selection.anchor,focus:index};else selection={anchor:index,focus:index};
   scroll.focus({preventScroll:true});scheduleRender();
  });
  panel.addEventListener('dblclick',event=>{const row=event.target.closest('[data-row]');if(!row)return;event.preventDefault();const hit=event.target.closest('mark[data-hit]');void jump(Number(row.dataset.row),hit?Number(hit.dataset.hit):0);});
  panel.addEventListener('contextmenu',event=>{
   if(!event.target.closest('[data-ed-find-results-scroll]'))return;
   event.preventDefault();
   const row=event.target.closest('[data-row]');
   if(row){const index=Number(row.dataset.row),low=Math.min(selection.anchor,selection.focus),high=Math.max(selection.anchor,selection.focus);if(index<low||index>high)selection={anchor:index,focus:index};scheduleRender();}
   openMenu(event.clientX,event.clientY);
  });
  document.addEventListener('pointerdown',event=>{if(menu&&!menu.contains(event.target))closeMenu();},true);
  // Resizing, as the editor's other dividers.
  const handle=panel.querySelector('[data-ed-find-results-resize]');
  const setHeight=value=>{panelSized=true;const max=Math.max(120,(a.windowEl().clientHeight||600)*0.7);panelHeight=Math.round(Math.max(80,Math.min(max,value)));panel.style.setProperty('--nt-find-results-height',panelHeight+'px');handle.setAttribute('aria-valuenow',String(panelHeight));scheduleRender();};
  handle.addEventListener('pointerdown',event=>{
   if(event.button!==0)return;event.preventDefault();
   const startY=event.clientY,startHeight=panelHeight;handle.setPointerCapture?.(event.pointerId);handle.classList.add('is-resizing');
   const move=e=>setHeight(startHeight-(e.clientY-startY));
   const end=()=>{handle.removeEventListener('pointermove',move);handle.removeEventListener('pointerup',end);handle.removeEventListener('pointercancel',end);handle.classList.remove('is-resizing');};
   handle.addEventListener('pointermove',move);handle.addEventListener('pointerup',end);handle.addEventListener('pointercancel',end);
  });
  handle.addEventListener('dblclick',()=>setHeight(180));
  handle.addEventListener('keydown',event=>{
   const steps={ArrowUp:16,ArrowDown:-16};
   if(Object.hasOwn(steps,event.key)){event.preventDefault();setHeight(panelHeight+steps[event.key]);}
   else if(event.key==='Home'){event.preventDefault();setHeight(80);}
   else if(event.key==='End'){event.preventDefault();setHeight(1e6);}
   else if(event.key==='Enter'){event.preventDefault();setHeight(180);}
  });
  handle.setAttribute('aria-valuemin','80');handle.setAttribute('aria-valuenow',String(panelHeight));
 }

 // Keys of the Find window, the results panel, marks and bookmarks.
 // Returns true when the editor should not handle the key itself.
 function key(event){
  const inWindow=!!el&&el.contains(event.target),inPanel=!!panel&&panel.contains(event.target)||!!menu&&menu.contains(event.target);
  if(event.isComposing)return inWindow||inPanel;
  const ctrl=event.ctrlKey||event.metaKey,k=event.key.length===1?event.key.toLowerCase():event.key;
  if(a.comparing())return false;
  if(ctrl&&!event.shiftKey&&!event.altKey&&(k==='f'||k==='h'||k==='m')){event.preventDefault();toggle(k==='f'?'find':k==='h'?'replace':'mark');return true;}
  if(event.key==='F3'&&!ctrl&&!event.altKey){event.preventDefault();if(!el?.querySelector('[data-ed-query]')?.value){void open('find');return true;}void findNext(event.shiftKey);return true;}
  if(inPanel)return panelKey(event)||true;
  if(inWindow){
   if(event.key==='Escape'){event.preventDefault();if(!el.querySelector('.nt-find-swap-list').hidden){closeSwapMenu();el.querySelector('[data-ed-find-swap-menu]').focus();return true;}close();return true;}
   if(event.key==='Enter'&&event.target.matches('input[type="text"]')&&!event.defaultPrevented){event.preventDefault();if(tab==='mark')void markAll();else void findNext(event.shiftKey?!opts.backward:opts.backward);return true;}
   const tabs=event.target.closest('[data-ed-find-tab]');
   if(tabs&&['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();const ids=TABS.map(([id])=>id),at=ids.indexOf(tab),next=event.key==='Home'?0:event.key==='End'?ids.length-1:(at+(event.key==='ArrowLeft'?-1:1)+ids.length)%ids.length;setTab(ids[next],false);el.querySelector('[data-ed-find-tab="'+ids[next]+'"]').focus();return true;}
   return true;
  }
  if(event.key==='F2'&&!event.altKey&&!event.shiftKey&&ctrl){event.preventDefault();toggleBookmark();return true;}
  if(event.key==='F2'&&!event.altKey&&!ctrl){event.preventDefault();void nextBookmark(event.shiftKey);return true;}
  return false;
 }
 function draw(){
  if(!el)return;
  if(a.comparing()&&!el.hidden){closeSwapMenu();el.hidden=true;}
  selectionChanged();place();
 }
 function setPreferences(value){
  const v=value&&typeof value==='object'?value:{};
  transparency.enabled=v.enabled===true;transparency.mode=v.mode==='always'?'always':'blur';
  transparency.level=Number.isInteger(v.level)&&v.level>=20&&v.level<=95?v.level:70;
  syncControls();applyTransparency();
 }
 function closed(d){marks.delete(d);bookmarks.delete(d);markCache.delete(d);if(scope?.uid===d.uid)scope=null;scheduleRender();}
 return Object.freeze({
  open,close,toggle,isOpen:()=>!!el&&!el.hidden,key,draw,
  findNext,replaceOne,replaceAll,
  selectionChanged,edited,closed,
  marksIn,hasMarks:d=>marksOf(d).length>0||!!currentMatch(d),
  bookmarks:d=>{const list=bookmarks.get(d);return list?.length?new Set(list):null;},
  hasBookmarks:d=>!!bookmarks.get(d)?.length,
  toggleBookmark,nextBookmark,
  showResults:show=>{mount();showPanel(show===undefined?panel.hidden:!!show);},
  resultsShown:()=>!!panel&&!panel.hidden,
  preferences:()=>({...transparency}),setPreferences,
  get element(){return el;},get resultsPanel(){return panel;},
  // Test and diagnostics hooks: the rows in the DOM stay few however many
  // rows a search has.
  get renderedRows(){return panel?panel.querySelectorAll('.nt-find-row').length:0;}
 });
}
root.NTKEditorFind=Object.freeze({create,layout,rowAt,rowHTML,shiftLines,explain,GUIDE,MODE_HELP});
if(typeof module==="object"&&module.exports)module.exports={create,layout,rowAt,rowHTML,shiftLines,explain,GUIDE,MODE_HELP};
})(globalThis);
