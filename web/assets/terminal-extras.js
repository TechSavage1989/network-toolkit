/* Per-session output blocks and frequently used terminal controls.
 * History is transient. Only positions in retained terminal output are transferred.
 */
(function(root){
'use strict';
const MAX_HISTORY=50, bindings=new WeakSet();
let sessionOpen=false, initialized=false, outputSyncFrame=null;
function withoutInput(entry,callback){return root.NetworkToolkitTerminalCopy?.withInputSuppressed?root.NetworkToolkitTerminalCopy.withInputSuppressed(entry,callback):callback();}
function prefs(entry){return {...(root.NetworkToolkitTerminalInteraction?.preferences?.()||{}),autoScroll:true,...entry?._ntSessionPrefs};}
function history(entry){return entry?(entry._ntOutputHistory||(entry._ntOutputHistory=[])):[];}
function marker(entry,line){try{const b=entry.xterm.buffer.active;return entry.xterm.registerMarker(line===undefined?0:line-b.baseY-b.cursorY)||null}catch{return null}}
function idlePrompt(text){return /^[\w.@:/()~\[\] -]{0,150}[#>$%]\s*$/.test(text);}
function echoMatches(line,echo){return echo.text===undefined||line.text.trimEnd()===echo.text.trimEnd()||echo.prefix&&line.text.startsWith(echo.text);}
function live(m){return !!m&&!m.isDisposed&&m.line>=0;}
function normal(entry){return !!entry?.xterm&&entry.xterm.buffer.active===entry.xterm.buffer.normal;}
function disposeItem(item){for(const m of [item.start,item.end,...(item.echo||[]).flatMap(e=>[e.start,e.end])])try{m?.dispose()}catch{}}
function touch(entry){entry.outputRevision=(entry.outputRevision||0)+1;}
function finalize(entry){const item=history(entry).at(-1);if(item&&!item.closed){item.end=marker(entry);item.closed=true;root.NetworkToolkitTerminalOutputArchive?.close(entry,item);}}
function add(entry,kind,start,expected=[]){const list=history(entry);const item={id:'block-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,7),kind,start,end:null,closed:false,at:Date.now(),echo:[],expected,submitted:kind==='command'};list.push(item);while(list.length>MAX_HISTORY){const old=list.shift();root.NetworkToolkitTerminalOutputArchive?.dispose(entry,old);disposeItem(old);}entry._ntOutputChoice=kind==='paste'?'last-paste':'last-command';touch(entry);sync();return item;}
function notePaste(entry,text){if(!normal(entry)||!text)return;if(pagerInput(entry,text)&&/^[ \r\nqQ\x03]+$/.test(text)){consumePagerInput(entry);return;}entry._ntInputDraft?.start?.dispose();entry._ntInputDraft=null;finalize(entry);const expected=String(text).replace(/\r\n?/g,'\n').split('\n').map(s=>s.trim()).filter(Boolean);const item=add(entry,'paste',marker(entry),expected);root.NetworkToolkitTerminalOutputArchive?.begin(entry,item,inputLine(entry,entry.xterm.buffer.normal.baseY+entry.xterm.buffer.normal.cursorY).text,expected);item.submitted=/[\r\n]/.test(text);entry._ntPendingPaste=item.submitted?null:item;}
function lineText(buffer,row){
 const line=buffer.getLine(row),next=buffer.getLine(row+1);if(!line)return '';
 const wrapped=next?.isWrapped,last=line.getCell(line.length-1);
 // A wide glyph can leave an empty padding cell at a wrapped row's end.
 // It is not a printed space and must not be added to the logical text.
 const padding=wrapped&&last?.getWidth()===1&&last.getCode()===0&&next.getCell(0)?.getWidth()===2;
 return line.translateToString(!wrapped,0,line.length-(padding?1:0));
}
function logicalLine(entry,row){const b=entry.xterm.buffer.normal;let start=row,end=row;while(start>0&&b.getLine(start)?.isWrapped)start--;while(end+1<b.length&&b.getLine(end+1)?.isWrapped)end++;let text='';for(let y=start;y<=end;y++)text+=lineText(b,y);return{start,end,text};}
function inputLine(entry,row){
 const line=logicalLine(entry,row),b=entry.xterm.buffer.normal;
 if(line.end===row){const beforeCursor=b.getLine(row)?.translateToString(false,0,b.cursorX)||'',tail=lineText(b,row);if(beforeCursor.length>tail.length)line.text+=' '.repeat(beforeCursor.length-tail.length);}
 return line;
}
function noteManualInput(entry,data){
 if(!normal(entry)||!data||!terminalEntryAcceptsInput(entry)||root.NetworkToolkitTerminalInteraction?.suppressesInput?.())return;
 // Pager responses, mouse reports, focus reports and terminal replies are not commands.
 if(pagerInput(entry,data)){consumePagerInput(entry);return;}
 if(data.startsWith('\x1b')&&!/^\x1b(?:\[(?:[ABCDHF]|[1-8](?:;\d+)?[~ABCDHF])|O[ABCDHF])$/.test(data))return;
 if(entry.xterm.buffer.active.type==='alternate')return;
 if(/^[\x03\x04\x1a]$/.test(data)){entry._ntInputDraft?.start?.dispose();entry._ntInputDraft=null;entry._ntPendingPaste=null;return;}
 if(/^[\x11\x13\t]$/.test(data))return;
 const enter=/[\r\n]/.test(data), b=entry.xterm.buffer.normal;
 if(entry._ntPendingPaste&&history(entry).includes(entry._ntPendingPaste)){
   if(enter){const item=entry._ntPendingPaste,line=logicalLine(entry,b.baseY+b.cursorY);if(line.text.trim())item.echo.push({start:marker(entry,line.start),end:marker(entry,line.end),text:line.text,prefix:idlePrompt(line.text)});item.submitted=true;entry._ntPendingPaste=null;touch(entry);sync();}return;
 }
 let draft=entry._ntInputDraft;
 if(!draft){
   if(/^[\x08\x7f]$/.test(data))return;
   finalize(entry);draft=entry._ntInputDraft={start:marker(entry),row:b.baseY+b.cursorY};
 }
 if(enter){
   const line=logicalLine(entry,b.baseY+b.cursorY),item=add(entry,'command',draft.start);root.NetworkToolkitTerminalOutputArchive?.begin(entry,item,inputLine(entry,b.baseY+b.cursorY).text,[],true);
   // Save the displayed echo's positions, never keystrokes or hidden password input.
   if(line.text.trim())item.echo.push({start:marker(entry,line.start),end:marker(entry,line.end),text:line.text,prefix:idlePrompt(line.text)});
   entry._ntInputDraft=null;
 }
}
function current(entry,choice=entry?._ntOutputChoice||'last-command'){
 const list=history(entry);if(choice==='last-command')return [...list].reverse().find(i=>i.kind==='command')||null;
 if(choice==='last-paste')return [...list].reverse().find(i=>i.kind==='paste')||null;
 return list.find(i=>i.id===choice)||null;
}
function rows(entry,item){
 if(!normal(entry)||!live(item?.start))return null;
 const b=entry.xterm.buffer.normal,start=item.start.line;
 let end=item.closed?(live(item.end)?item.end.line-1:-1):b.baseY+b.cursorY;
 if(end<start)return {buffer:b,start,end,lines:[]};
 end=Math.min(end,b.length-1);const lines=[];
 for(let y=start;y<=end;y++){
   const line=b.getLine(y);if(!line)continue;const text=lineText(b,y);
   if(line.isWrapped&&lines.length){lines.at(-1).text+=text;lines.at(-1).end=y;}
   else lines.push({start:y,end:y,text});
 }
 // Recognize pasted echoes in their original order. Do not strip arbitrary suffixes
 // or bare output lines that happen to repeat a submitted command. Without a
 // visible prompt prefix an echo cannot safely be distinguished from real output.
 if(item.expected?.length){let next=item.matched||0;for(const line of lines){
   if(item.echo.some(e=>live(e.start)&&line.start===e.start.line))continue;
   const cmd=item.expected[next];if(!cmd)break;
   const t=line.text.trim();const at=t.length-cmd.length;
   const prefix=at>=0?t.slice(0,at):'';
   if(at>0&&t.endsWith(cmd)&&/^[^\r\n]{1,160}[#>$%]\s*$/.test(prefix)){
     item.echo.push({start:marker(entry,line.start),end:marker(entry,line.end),text:line.text,prefix:idlePrompt(line.text)});next++;
   }
 }item.matched=next;if(next>=item.expected.length){item.expected=[];item.matched=0;}}
 return {buffer:b,start,end,lines};
}
function screenOutput(entry,item,includeCommands=false){
 const range=rows(entry,item);if(!range)return null;
 let lines=range.lines.filter(line=>!matchPagerLine(line.text));
 if(!includeCommands&&lines[0]?.start===range.start&&idlePrompt(lines[0].text))lines=lines.slice(1);
 if(!includeCommands)lines=lines.filter(line=>!item.echo.some(e=>live(e.start)&&live(e.end)&&line.start>=e.start.line&&line.start<=e.end.line&&echoMatches(line,e)));
 // The final idle prompt is a boundary, rather than generated command output.
 if(lines.length&&!item.closed&&/^[\w.@:/()~\[\] -]{0,150}[#>$%]\s*$/.test(lines.at(-1).text))lines=lines.slice(0,-1);
 while(lines.length&&!lines.at(-1).text.trim())lines=lines.slice(0,-1);
 return {...range,lines,text:lines.map(l=>l.text).join('\n')};
}
function output(entry,item,includeCommands=false){return root.NetworkToolkitTerminalOutputArchive?.read(entry,item,includeCommands)||screenOutput(entry,item,includeCommands);}
function available(entry,item){return root.NetworkToolkitTerminalOutputArchive?.has(entry,item)??!!screenOutput(entry,item,true)?.text;}
function afterCapture(entry,item,action){
 const archive=root.NetworkToolkitTerminalOutputArchive;
 if(!archive?.pending(entry,item))return action();
 return archive.flush(entry,item).then(()=>history(entry).includes(item)?action():false).catch(error=>{toast(error.message,'error');return false;});
}
function preview(entry,item,which='select',includeCommands=prefs(entry).pasteIncludeCommands===true){const result=output(entry,item,includeCommands);if(!result?.text){toast('No captured output is available for this block yet.');return false;}withoutInput(entry,()=>{openModal('Command output','<p class="hint">'+(result.truncated?'This window received a shortened output archive. The retained portion is shown.':'This output is retained separately from the console scrollback.')+'</p><textarea id="terminalSelectedOutput" class="mono terminal-selected-output" readonly spellcheck="false"></textarea>','<button class="btn ghost" data-action="close-modal">Close</button><button class="btn primary" data-terminal-extra="copy-selection-preview">'+icon('copy')+'Copy output</button>',true);const field=document.querySelector('#terminalSelectedOutput');field.value=result.text;field.focus();if(which==='select')field.select();else{const end=which==='end',firstBreak=field.value.indexOf('\n');const start=end?field.value.lastIndexOf('\n')+1:0,finish=end||firstBreak<0?field.value.length:firstBreak;field.setSelectionRange(start,finish);field.scrollTop=end?field.scrollHeight:0;}});return true;}
function nativeFollow(entry,enabled){const service=entry?.xterm?._core?._bufferService;if(service)service.isUserScrolling=!enabled;}
function followCurrent(entry){if(!entry?.xterm)return;const follow=prefs(entry).autoScroll;nativeFollow(entry,follow);if(follow)entry.xterm.scrollToBottom();}
function beforeFit(entry){if(!entry?.xterm)return null;const follow=prefs(entry).autoScroll;nativeFollow(entry,follow);return {epoch:entry._ntScrollEpoch||0,anchor:!follow&&normal(entry)?marker(entry,entry.xterm.buffer.normal.viewportY):null};}
function afterFit(entry,position){try{if(!entry?.xterm||!position)return;const follow=prefs(entry).autoScroll;nativeFollow(entry,follow);if(position.epoch!==(entry._ntScrollEpoch||0))return;if(follow)entry.xterm.scrollToBottom();else if(normal(entry)&&live(position.anchor)){if(root.NetworkToolkitTerminalState?.restoreViewport)root.NetworkToolkitTerminalState.restoreViewport(entry.xterm,Math.min(position.anchor.line,entry.xterm.buffer.normal.baseY));else entry.xterm.scrollToLine(position.anchor.line);}}finally{position?.anchor?.dispose();}}
function setFollow(entry,enabled){if(!entry)return;entry._ntScrollEpoch=(entry._ntScrollEpoch||0)+1;entry._ntSessionPrefs={...entry._ntSessionPrefs,autoScroll:!!enabled};nativeFollow(entry,enabled);touch(entry);}
function pauseFollow(entry){setFollow(entry,false);sync();}
function followInput(entry,data){if(!entry||!data)return;if(data.startsWith('\x1b')&&!/^\x1b(?:\[(?:[ABCDHF]|[1-8](?:;\d+)?[~ABCDHF])|O[ABCDHF]|\[200~)/.test(data))return;if(/^[\x00\x05\x11\x13]$/.test(data))return;setFollow(entry,true);entry.xterm.scrollToBottom();sync();}
function jump(entry,which='start',choice){
 const item=current(entry,choice);return afterCapture(entry,item,()=>{
  const includeCommands=prefs(entry).pasteIncludeCommands===true,range=screenOutput(entry,item,includeCommands);pauseFollow(entry);
  if(!range)return preview(entry,item,which,includeCommands);
  // Use the same echo/prompt boundaries as Select and Copy. A wrapped output
  // line is one destination even when it occupies several terminal rows.
  const line=which==='end'?range.lines.at(-1):range.lines[0],fallback=which==='end'?Math.max(range.start,range.end):range.start;
  const start=line?.start??fallback,end=line?.end??start;
  withoutInput(entry,()=>{entry.xterm.selectLines(start,end);entry.xterm.scrollToLine(which==='end'?end:start);entry.xterm.focus();});return true;
 });
}
function pasteChoice(entry){const item=current(entry);return item?.kind==='paste'?item.id:'last-paste';}
function step(entry,delta){const list=history(entry).filter(i=>i.kind==='paste');if(!list.length)return false;const old=current(entry,'last-paste'),selected=current(entry),at=Math.max(0,list.indexOf(selected?.kind==='paste'?selected:old)),item=list[Math.max(0,Math.min(list.length-1,at+delta))];entry._ntOutputChoice=item.id;return jump(entry,'start');}
function select(entry,includeCommands=prefs(entry).pasteIncludeCommands===true,choice){
 const item=current(entry,choice);return afterCapture(entry,item,()=>selectReady(entry,item,includeCommands));
}
function selectReady(entry,item,includeCommands){
 if(!rows(entry,item)&&available(entry,item))return preview(entry,item,'select',includeCommands);
 const result=output(entry,item,includeCommands);
 if(!result?.text){toast('No retained output is available for this block yet.');return false;}
 pauseFollow(entry);const displayed=screenOutput(entry,item,includeCommands),contiguous=displayed?.text===result.text&&displayed.lines.every((line,i)=>!i||line.start===displayed.lines[i-1].end+1);
 if(contiguous){withoutInput(entry,()=>{entry.xterm.selectLines(displayed.lines[0].start,displayed.lines.at(-1).end);entry.xterm.scrollToLine(displayed.lines[0].start);entry.xterm.focus();});return true;}
 // xterm supports one continuous selection. A plain-text selection preserves
 // output without reintroducing interleaved command echoes.
 withoutInput(entry,()=>{openModal('Select output','<div class="field"><label for="terminalSelectedOutput">Output with recognized command echoes excluded</label><textarea id="terminalSelectedOutput" class="mono terminal-selected-output" readonly spellcheck="false"></textarea></div>','<button class="btn ghost" data-action="close-modal">Close</button><button class="btn primary" type="button" data-terminal-extra="copy-selection-preview">'+icon('copy')+'Copy output</button>',true);
 const field=document.querySelector('#terminalSelectedOutput');field.value=result.text;field.focus();field.select();});return true;
}
async function copy(entry,includeCommands=prefs(entry).pasteIncludeCommands===true,choice){const item=current(entry,choice);return afterCapture(entry,item,async()=>{const result=output(entry,item,includeCommands);if(!result?.text){toast('No retained output is available for this block yet.');return false;}const copied=await root.NetworkToolkitTerminalInteraction.copyValue(result.text,{entry});if(copied&&result.truncated)toast('Copied the retained portion. This window received a shortened output archive.');return copied;});}
function scrollTop(entry){pauseFollow(entry);withoutInput(entry,()=>{entry?.xterm?.scrollToTop?.();entry?.xterm?.focus?.();});}
function scrollBottom(entry){setFollow(entry,true);withoutInput(entry,()=>{entry?.xterm?.scrollToBottom?.();entry?.xterm?.focus?.();});sync();}
// Match complete visible pager lines, never prose suffixes or confirmation prompts.
// In particular, a bare "Press Enter to continue" is ambiguous and is NOT paging.
function matchPagerLine(text){
 const line=String(text||'').trim();if(!line||line.length>512)return null;
 const marker='(?:-{2,}\\s*More\\s*-{2,}|<---\\s*More\\s*--->|\\(more\\)|More:)';
 const help='(?:,\\s*next page:\\s*Space,\\s*next line:\\s*(?:Enter|Return(?: key)?),\\s*quit:\\s*(?:q|Control-C|Ctrl-C|\\^C))?';
 if(new RegExp('^'+marker+help+'$','i').test(line)||/^---\(more(?:\s+\d{1,3}%)?\)---$/i.test(line)||/^--More--\(\d{1,3}%\)$/i.test(line))return {key:'space',signature:line};
 if(/^\[?Press\s+(?:the\s+)?space(?:bar)?\s+(?:to\s+continue,?\s*['"]?q['"]?\s+to\s+quit\.?|for\s+(?:the\s+)?(?:next\s+)?page)\]?$/i.test(line))return {key:'space',signature:line};
 if(/^Press\s+(?:Enter|Return)\s+(?:for\s+more|for\s+(?:the\s+)?next\s+(?:page|line))$/i.test(line))return {key:'enter',signature:line};
 return null;
}
// Small incremental visible-line parser. It is intentionally not a second terminal:
// xterm remains authoritative for cells/selection/archive. This parser only recognizes
// bounded pager lines, handling split UTF-8, CSI/OSC, CR overwrite and backspace.
function createPagerRecognizer(){
 let cells=[],column=0,state='text',sequence='',overflow=false,epoch=0,used=-1,rewrite=false,decoder=new TextDecoder();
 const reset=()=>{cells=[];column=0;overflow=false;rewrite=false;epoch++;};
 function csi(final){
  const values=sequence.split(';').map(n=>Number(n)||0),n=values[0]||1;
  if(final==='K'){
   const before=cells.join('').trimEnd();
   if(values[0]===2){cells=[];overflow=false;}else if(values[0]===1){for(let i=0;i<=column&&i<cells.length;i++)cells[i]=' ';}else cells.length=Math.min(cells.length,column);
   if(cells.join('').trimEnd()!==before)epoch++;rewrite=false;
  }else if(final==='J'){reset();}
  else if(final==='G'||final==='`')column=Math.max(0,n-1);
  else if(final==='D')column=Math.max(0,column-n);
  else if(final==='C')column=Math.min(4096,column+n);
  else if(final==='H'||final==='f'){reset();column=Math.max(0,(values[1]||1)-1);}
  else if('ABEFd'.includes(final)){reset();}
  else if(!'mhlcn'.includes(final)){reset();}
  // SGR, private modes and reports change no visible pager characters.
 }
 function feed(bytes){
  const text=typeof bytes==='string'?bytes:decoder.decode(bytes,{stream:true});
  for(const c of text){
   if(state==='osc'||state==='string'){if(c==='\x9c'||c==='\x07'&&state==='osc')state='text';else if(c==='\x1b')state=state==='osc'?'osc-escape':'string-escape';continue;}
   if(state==='osc-escape'||state==='string-escape'){state=c==='\\'?'text':state==='osc-escape'?'osc':'string';continue;}
   if(state==='escape'){
    if(c==='['){state='csi';sequence='';}else if(c===']')state='osc';else if('P_^X'.includes(c))state='string';else if('()*+-./'.includes(c))state='charset';else{state='text';if(c==='c'||c==='D'||c==='E')reset();}continue;
   }
   if(state==='charset'){state='text';continue;}
   if(state==='csi'){
    if(c>='@'&&c<='~'){csi(c);state='text';sequence='';}
    else if(sequence.length<128)sequence+=c;else{state='text';reset();overflow=true;}continue;
   }
   if(c==='\x1b'){state='escape';continue;}
   if(c==='\x9b'){state='csi';sequence='';continue;}
   if(c==='\x9d'){state='osc';continue;}
   if(c==='\x90'){state='string';continue;}
   if(c==='\r'){column=0;rewrite=true;continue;}
   if(c==='\n'){reset();continue;}
   if(c==='\b'||c==='\x7f'){column=Math.max(0,column-1);rewrite=true;continue;}
   if(c==='\t'){column=Math.min(4096,(Math.floor(column/8)+1)*8);continue;}
   if(c<' '||c==='\x9c')continue;
   if(column>=4096){overflow=true;continue;}
   if(rewrite&&used===epoch&&cells[column]!==c){epoch++;rewrite=false;}
   while(cells.length<column)cells.push(' ');cells[column++]=c;
  }
  return current();
 }
 function current(){return overflow||state!=='text'?null:matchPagerLine(cells.join(''));}
 return {feed,current,consume(){used=epoch;},ready(){return used!==epoch&&!!current();},reset(){reset();state='text';sequence='';decoder=new TextDecoder();},get epoch(){return epoch;}};
}
function pagerPrompt(text){return createPagerRecognizer().feed(String(text||''));}
function pagerState(entry){return entry._ntPagerState||(entry._ntPagerState=createPagerRecognizer());}
function pagerInput(entry,data){return !!data&&!!entry?._ntPagerState?.current();}
function cancelPagerTimer(entry){if(entry?._ntPagerTimer){clearTimeout(entry._ntPagerTimer);entry._ntPagerTimer=null;}}
function consumePagerInput(entry){cancelPagerTimer(entry);entry._ntPagerState?.consume();root.NetworkToolkitTerminalInput?.clear(entry);}
function schedulePager(entry){
 cancelPagerTimer(entry);const state=pagerState(entry),match=state.current(),p=prefs(entry);
 if(!match||!p.autoPager||!state.ready()||root.NetworkToolkitTerminalMacros?.ownsPager?.(entry)||!terminalEntryAcceptsInput(entry))return;
 const epoch=state.epoch;
 // Allow a split explanatory suffix or confirmation to finish before deciding.
 entry._ntPagerTimer=setTimeout(()=>{
  entry._ntPagerTimer=null;const now=state.current(),settings=prefs(entry);
  if(entry._ntPagerState!==state||state.epoch!==epoch||!now||!state.ready()||!settings.autoPager||root.NetworkToolkitTerminalMacros?.ownsPager?.(entry)||!terminalEntryAcceptsInput(entry))return;
  const key=['space','enter'].includes(settings.pagerKey)?settings.pagerKey:now.key;
  if(enqueueTerminalInput(entry,key==='enter'?'\r':' ',false,'pager')===true)state.consume();
 },35);
}
function observeOutput(entry,bytes){
 root.NetworkToolkitTerminalOutputArchive?.feed(entry,bytes);
 if(!entry||!bytes)return;try{pagerState(entry).feed(bytes);}catch{return;}
 schedulePager(entry);
}
function beforeOutput(entry){if(!entry?.xterm)return null;nativeFollow(entry,prefs(entry).autoScroll);return {epoch:entry._ntScrollEpoch||0};}
function afterOutput(entry,position){if(!entry?.xterm||!position)return;const follow=prefs(entry).autoScroll;nativeFollow(entry,follow);if(follow&&position.epoch===(entry._ntScrollEpoch||0))entry.xterm.scrollToBottom();scheduleOutputSync(entry);}
function clear(entry){if(!entry)return;cancelPagerTimer(entry);entry._ntPagerState?.reset();root.NetworkToolkitTerminalInput?.clear(entry);for(const item of history(entry)){root.NetworkToolkitTerminalOutputArchive?.dispose(entry,item);disposeItem(item);}entry._ntOutputHistory=[];entry._ntInputDraft?.start?.dispose();entry._ntInputDraft=null;entry._ntPendingPaste=null;touch(entry);sync();}
function snapshot(entry){
 const list=history(entry);for(const item of list)rows(entry,item);let budget=4*1024*1024;const saved=new Map();
 for(const item of [...list].reverse()){const value=root.NetworkToolkitTerminalOutputArchive?.snapshot(entry,item);if(!value)continue;const bytes=new TextEncoder().encode(value.screen).length;if(bytes<=budget){saved.set(item,value);budget-=bytes;}}
 return {version:1,input:root.NetworkToolkitTerminalInput?.snapshot(entry),choice:entry._ntOutputChoice||'last-command',settings:{...entry._ntSessionPrefs},blocks:list.filter(i=>live(i.start)&&(!i.closed||live(i.end))||saved.has(i)).map(i=>({id:i.id,kind:i.kind,start:live(i.start)?i.start.line:-1,end:i.closed&&live(i.end)?i.end.line:-1,closed:i.closed,at:i.at,archive:saved.get(i),echo:i.echo.filter(e=>live(e.start)&&live(e.end)).slice(0,1024).map(e=>({start:e.start.line,end:e.end.line}))}))};
}
function restore(entry,value){clear(entry);entry._ntSessionPrefs={};if(!value||value.version!==1)return;
 for(const k of ['autoPager','autoScroll','pasteReview','pasteIncludeCommands'])if(typeof value.settings?.[k]==='boolean')entry._ntSessionPrefs[k]=value.settings[k];
 if(['auto','space','enter'].includes(value.settings?.pagerKey))entry._ntSessionPrefs.pagerKey=value.settings.pagerKey;
 for(const i of (value.blocks||[]).slice(-MAX_HISTORY)){if(!['paste','command'].includes(i.kind)||!Number.isInteger(i.start)||i.start<0&&!i.archive||i.start>=entry.xterm.buffer.normal.length)continue;
 history(entry).push({...i,start:i.start>=0?marker(entry,i.start):null,end:i.closed&&i.end>=0?marker(entry,i.end):null,echo:(i.echo||[]).filter(e=>e.start>=0&&e.end>=e.start&&e.end<entry.xterm.buffer.normal.length).map(e=>({start:marker(entry,e.start),end:marker(entry,e.end)})),expected:[]});root.NetworkToolkitTerminalOutputArchive?.restore(entry,history(entry).at(-1),i.archive);}
 entry._ntOutputChoice=value.choice||'last-command';root.NetworkToolkitTerminalInput?.restore(entry,value.input);
 // A restored screen can already be paused at a pager with no new bytes due.
 // Recognize manual responses, but do not replay an automatic key that the old
 // window may already have queued before handing ownership over.
 if(normal(entry)){const b=entry.xterm.buffer.normal,line=logicalLine(entry,b.baseY+b.cursorY).text;if(matchPagerLine(line)){pagerState(entry).feed(line);pagerState(entry).consume();}}
 sync();}
function bottomHTML(){return '<div id="terminalConsoleBar" class="terminal-consolebar" aria-label="Console output controls"><div class="terminal-output-actions" role="group" aria-label="Command and paste output"><div id="terminalOutputBlock" class="terminal-output-switch" role="group" aria-label="Output source"><button type="button" class="btn small ghost" data-terminal-output-kind="last-command" aria-pressed="true" title="Use the latest typed command output">Last command</button><button type="button" class="btn small ghost" data-terminal-output-kind="last-paste" aria-pressed="false" title="Use the latest pasted input output">Last paste</button></div><button class="iconbtn" data-terminal-extra="output-start" aria-label="Jump to selected output start" title="Jump to selected output start">'+icon('outputStart')+'</button><button class="iconbtn" data-terminal-extra="output-end" aria-label="Jump to selected output end" title="Jump to selected output end">'+icon('outputEnd')+'</button><button class="btn small ghost" data-terminal-extra="select-output" title="Select the chosen command or paste output">Select output</button><button class="btn small ghost" data-terminal-extra="copy-output" title="Copy the chosen command or paste output">'+icon('copy')+'Copy output</button><label class="terminal-quick-check" title="Include recognized command echoes when selecting or copying"><input type="checkbox" id="terminalIncludeInput">Include commands</label></div><div class="terminal-live-actions" role="group" aria-label="Active session controls"><label class="terminal-quick-check" title="Automatically continue --More-- prompts for manually entered commands. Macros use their own automatic paging setting."><input type="checkbox" id="terminalQuickPager">Auto pager</label><label class="sr-only" for="terminalQuickPagerKey">Pager key</label><select id="terminalQuickPagerKey" title="Pager advance key for this session"><option value="auto">Auto key</option><option value="space">Space</option><option value="enter">Enter</option></select><label class="terminal-quick-check" title="Follow new output in this session. Jumping to earlier output pauses this."><input type="checkbox" id="terminalQuickFollow">Auto scroll</label><label class="terminal-quick-check" title="Review and edit multiline pastes before sending in this session"><input type="checkbox" id="terminalQuickReview">Review paste</label><div class="terminal-scroll-actions" role="group" aria-label="Scroll console"><button class="iconbtn" data-terminal-extra="scroll-top" aria-label="Scroll to top" title="Scroll to top">'+icon('arrowUp')+'</button><button class="iconbtn" data-terminal-extra="scroll-bottom" aria-label="Scroll to bottom" title="Scroll to bottom">'+icon('arrowDown')+'</button></div></div></div>';}
function setSessionOpen(open){sessionOpen=!!open;const panel=document.querySelector('#terminalSessionPanel'),button=document.querySelector('#terminalSessionToggle');if(!panel||!button)return;panel.classList.toggle('hidden',!sessionOpen);button.setAttribute('aria-expanded',String(sessionOpen));const r=button.getBoundingClientRect(),width=panel.getBoundingClientRect().width;panel.style.top=Math.min(innerHeight-100,r.bottom+5)+'px';panel.style.right=Math.max(6,Math.min(innerWidth-width-6,innerWidth-r.right))+'px';panel.style.maxHeight=Math.max(100,innerHeight-r.bottom-12)+'px';if(sessionOpen)panel.querySelector('button,select,input')?.focus({preventScroll:true});}
// Parsing and output callbacks can arrive many times per frame across sessions.
// Draw the active session's controls once; inactive sessions do not own the toolbar.
function scheduleOutputSync(entry){
 if(!entry||currentTerminal()!==entry)return;
 if(typeof root.requestAnimationFrame!=='function'){sync();return;}
 if(outputSyncFrame!==null)return;
 outputSyncFrame=root.requestAnimationFrame(()=>{
  outputSyncFrame=null;
  if(currentTerminal())sync();
 });
}
function sync(){
 if(outputSyncFrame!==null){root.cancelAnimationFrame?.(outputSyncFrame);outputSyncFrame=null;}
 const bar=document.querySelector('#terminalCommandBar'),tools=document.querySelector('#terminalToolsPanel');if(!bar||!tools)return;
 if(!document.querySelector('#terminalConsoleBar')){
   document.querySelector('.terminal-statusbar').insertAdjacentHTML('beforebegin',bottomHTML());
   // Keep the output itself free of hovering navigation and copy buttons.
   for(const action of ['scroll-top','scroll-bottom','paste-start'])bar.querySelector('[data-terminal-action="'+action+'"]')?.remove();
   bar.querySelector('#terminalToolsToggle').insertAdjacentHTML('beforebegin','<button class="btn small ghost" id="terminalSessionToggle" data-terminal-extra="session" aria-haspopup="dialog" aria-expanded="false" aria-controls="terminalSessionPanel">Display</button>');
   const panel=document.createElement('div');panel.id='terminalSessionPanel';panel.className='terminal-session-panel hidden';panel.setAttribute('role','dialog');panel.setAttribute('aria-label','Console display settings');panel.innerHTML='<div class="terminal-tools-heading"><strong>Console display settings</strong><button class="iconbtn" data-terminal-extra="close-session" aria-label="Close display settings">'+icon('close')+'</button></div>';
   for(const section of [...tools.children]){
     const heading=section.querySelector('h3')?.textContent;
     if(heading==='Active session'){
       section.id='terminalActiveActions';section.className='terminal-active-actions';section.setAttribute('aria-label','Active session commands');
       section.insertAdjacentHTML('beforeend','<label class="terminal-active-position" for="terminalActiveActionsPosition"><span>Position</span><select id="terminalActiveActionsPosition" aria-label="Active session button position"><option value="top">Top</option><option value="bottom">Bottom</option></select></label>');
     }else if(heading==='Display')panel.append(section);
   }
   panel.insertAdjacentHTML('beforeend','<p class="hint">Auto pager, Auto scroll, Review paste and Include commands in the console bar apply to the focused session.</p><button class="btn small ghost" data-terminal-extra="mouse-settings">Mouse and keyboard defaults</button>');
   document.querySelector('.terminal-panel').append(panel);
   root.NetworkToolkitTerminalAppearance?.mount(panel);
 }
 const entry=currentTerminal(),p=prefs(entry),choice=entry?._ntOutputChoice||'last-command';
 const actions=document.querySelector('#terminalActiveActions');
 const position=state.terminalActiveActionsPosition==='bottom'?'bottom':'top',anchor=position==='bottom'?document.querySelector('#terminalConsoleBar'):(document.querySelector('#terminalTabs')||document.querySelector('#terminalStage'));
 if(anchor&&[...anchor.parentElement.children][[...anchor.parentElement.children].indexOf(anchor)-1]!==actions)anchor.before(actions);
 actions.dataset.position=position;
 const positionPicker=document.querySelector('#terminalActiveActionsPosition');if(positionPicker&&document.activeElement!==positionPicker)positionPicker.value=position;
 root.NetworkToolkitTerminalAppearance?.sync();
 for(const el of actions.querySelectorAll('[data-terminal-session]'))el.disabled=!entry?.xterm;
 for(const el of actions.querySelectorAll('[data-terminal-input]'))el.disabled=!terminalEntryAcceptsInput(entry);
 for(const el of actions.querySelectorAll('[data-terminal-action="open-sftp"],[data-terminal-action="browse-files"]'))el.disabled=!online||!entry||entry.info.type!=='ssh'||entry.info.state!=='connected'||!terminalOwnerCanMutate(entry)||!!entry.transferBusy||TERMINAL_ONLY&&(el.dataset.terminalAction==='open-sftp'||!root.NetworkToolkitTerminalSFTP?.available?.(entry));
 for(const el of actions.querySelectorAll('[data-terminal-connected]'))el.disabled=!entry||!terminalLive(entry)||entry.info.state==='disconnecting'||!online||!terminalOwnerCanMutate(entry)||!!entry.transferBusy;
 const selectedKind=choice==='last-paste'||current(entry)?.kind==='paste'?'last-paste':'last-command';
 for(const button of document.querySelectorAll('[data-terminal-output-kind]')){button.disabled=!entry;button.setAttribute('aria-pressed',String(button.dataset.terminalOutputKind===selectedKind));}
 for(const [id,value]of [['terminalQuickPager',p.autoPager],['terminalQuickFollow',p.autoScroll],['terminalQuickReview',p.pasteReview],['terminalIncludeInput',p.pasteIncludeCommands]]){const el=document.getElementById(id);el.checked=!!value;el.disabled=!entry;}
 const key=document.querySelector('#terminalQuickPagerKey');key.value=p.pagerKey||'auto';key.disabled=!entry||!p.autoPager;
 const item=current(entry),range=rows(entry,item);for(const el of document.querySelectorAll('[data-terminal-extra]'))if(['output-start','output-end','select-output','copy-output'].includes(el.dataset.terminalExtra))el.disabled=!available(entry,item);
 for(const el of document.querySelectorAll('.terminal-scroll-actions button'))el.disabled=!entry?.xterm;
 const old=tools.querySelector('#terminalPasteTools');if(old)old.remove();
 if(!initialized){initialized=true;document.addEventListener('pointerdown',e=>{if(e.target.closest('[data-terminal-extra="output-start"],[data-terminal-extra="output-end"],[data-terminal-extra="select-output"],[data-terminal-extra="copy-output"],[data-terminal-extra="scroll-top"],[data-terminal-extra="scroll-bottom"]'))e.preventDefault();},true);document.addEventListener('change',changed);document.addEventListener('click',clicked);document.addEventListener('keydown',e=>{if(e.key==='Escape'&&sessionOpen&&!document.querySelector('.modal')){e.preventDefault();setSessionOpen(false);document.querySelector('#terminalSessionToggle')?.focus();}});document.addEventListener('pointerdown',e=>{if(sessionOpen&&!e.target.closest('#terminalSessionPanel,#terminalSessionToggle'))setSessionOpen(false);});root.addEventListener('resize',()=>{if(sessionOpen)setSessionOpen(false);});}
 bind(entry);
 for(const button of document.querySelectorAll('[data-terminal-action="reconnect"]')){
   const target=button.dataset.terminalId?terminalSessions.get(button.dataset.terminalId):entry;
   button.disabled=!terminalReconnectAllowed(target);
 }
}
function changed(event){const entry=currentTerminal(),el=event.target;if(el.id==='terminalActiveActionsPosition'){state.terminalActiveActionsPosition=el.value==='bottom'?'bottom':'top';saveState();sync();scheduleTerminalFit();return;}
 const fields={terminalQuickPager:'autoPager',terminalQuickFollow:'autoScroll',terminalQuickReview:'pasteReview',terminalIncludeInput:'pasteIncludeCommands',terminalQuickPagerKey:'pagerKey'},key=fields[el.id];if(!key||!entry)return;
 entry._ntSessionPrefs={...entry._ntSessionPrefs,[key]:el.type==='checkbox'?el.checked:el.value};touch(entry);if(key==='autoScroll'){setFollow(entry,el.checked);if(el.checked)entry.xterm.scrollToBottom();}if(key==='autoPager'||key==='pagerKey')schedulePager(entry);sync();
}
function clicked(event){const source=event.target.closest('[data-terminal-output-kind]');if(source){event.preventDefault();const entry=currentTerminal(),kind=source.dataset.terminalOutputKind;if(!source.disabled&&entry&&['last-command','last-paste'].includes(kind)){entry._ntOutputChoice=kind;touch(entry);sync();}return;}const button=event.target.closest('[data-terminal-extra]');if(!button||button.disabled)return;event.preventDefault();const entry=currentTerminal();switch(button.dataset.terminalExtra){
 case'session':setTerminalToolsOpen(false,false);setSessionOpen(!sessionOpen);return;
 case'close-session':setSessionOpen(false);document.querySelector('#terminalSessionToggle')?.focus();return;
 case'mouse-settings':setSessionOpen(false);setTerminalToolsOpen(true,false);document.querySelector('#terminalInteractionSettings')?.scrollIntoView({block:'nearest'});return;
 case'copy-selection-preview':void root.NetworkToolkitTerminalInteraction.copyValue(document.querySelector('#terminalSelectedOutput')?.value||'',{entry});return;
 case'output-start':jump(entry);break;case'output-end':jump(entry,'end');break;case'select-output':select(entry);break;case'copy-output':void copy(entry);break;case'scroll-top':scrollTop(entry);break;case'scroll-bottom':scrollBottom(entry);break;
 }sync();
}
function bind(entry){if(!entry?.xterm||bindings.has(entry))return;bindings.add(entry);
 const wheel=e=>{if(!e.ctrlKey&&e.deltaY)pauseFollow(entry);};
 const pointer=e=>{const target=e.target;if(target?.closest?.('.scrollbar,.slider')||target?.classList?.contains('xterm-viewport')&&e.clientX>=target.getBoundingClientRect().right-Math.max(16,target.offsetWidth-target.clientWidth))pauseFollow(entry);};
 const key=e=>{if(e.shiftKey&&['PageUp','PageDown','Home'].includes(e.key))pauseFollow(entry);if(e.shiftKey&&e.key==='End')scrollBottom(entry);};
 entry.element.addEventListener('wheel',wheel,{capture:true,passive:true});entry.element.addEventListener('pointerdown',pointer,true);entry.element.addEventListener('keydown',key,true);
 const scrolled=entry.xterm.onScroll?.(()=>{if(!prefs(entry).autoScroll)nativeFollow(entry,false);});const parsed=entry.xterm.onWriteParsed?.(()=>scheduleOutputSync(entry));
 entry.disposables.push({dispose(){entry.element?.removeEventListener('wheel',wheel,true);entry.element?.removeEventListener('pointerdown',pointer,true);entry.element?.removeEventListener('keydown',key,true);scrolled?.dispose();parsed?.dispose();clear(entry);}});
}
root.NetworkToolkitTerminalExtras=Object.freeze({notePaste,noteManualInput,observeOutput,beforeOutput,afterOutput,beforeFit,afterFit,followCurrent,scrollTop,scrollBottom,jump,step,select,copy,sync,finalize,prefs,clear,snapshot,restore,current,output,rows,lineText,screenOutput,followInput,pauseFollow,flush:entry=>root.NetworkToolkitTerminalOutputArchive?.flush(entry),pagerPrompt,matchPagerLine,idlePrompt,echoMatches,createPagerRecognizer,pagerInput,pasteChoice});
})(window);
