/* Keep complete parsed command output independently of console scrollback.
 * Only the small, still-editable tail needs terminal cells. Completed rows are
 * sealed into text chunks before the shadow terminal can discard them.
 */
(function(root){
'use strict';
const archives=new WeakMap(),TRANSFER_LIMIT=4*1024*1024,WRITE_CHUNK=4096;
const size=text=>new TextEncoder().encode(text).length;
function position(term,line=0){const b=term.buffer.normal;return term.registerMarker(line-b.baseY-b.cursorY);}
function notify(a){if(!a.disposed&&currentTerminal()===a.owner)root.NetworkToolkitTerminalExtras.sync();}
function range(a){
 if(a.item.start.isDisposed){a.item.start=position(a.term,0);a.truncated=true;}
 return root.NetworkToolkitTerminalExtras.rows(a.entry,a.item);
}
function isEcho(a,line,index){return a.echo.has(index)||a.item.echo.some(e=>!e.start.isDisposed&&!e.end.isDisposed&&line.start>=e.start.line&&line.start<=e.end.line);}
function tail(a){
 if(!a.term)return a.value;
 if(a.cached)return a.cached;
 const saved=range(a);if(!saved)return {screen:'',echo:[],closed:a.item.closed,truncated:true};
 const echo=[];saved.lines.forEach((line,i)=>{if(isEcho(a,line,a.lines+i))echo.push(i);});
 return a.cached={screen:saved.lines.map(l=>l.text).join('\n'),echo,closed:a.item.closed,truncated:a.truncated};
}
function seal(a){
 const saved=range(a);if(!saved)return;
 const b=a.term.buffer.normal,until=Math.min(b.baseY,saved.end+1),start=a.item.start.line;
 if(until<=start)return;
 let text='',index=a.lines,logical=0;
 for(let y=start;y<until;y++){
   while(logical+1<saved.lines.length&&saved.lines[logical].end<y)logical++;
   if(isEcho(a,saved.lines[logical]||{start:y},index))a.echo.add(index);
   const continues=b.getLine(y+1)?.isWrapped;
   text+=root.NetworkToolkitTerminalExtras.lineText(b,y);
   if(!continues){text+='\n';index++;}
 }
 if(text){a.chunks.push(text);a.bytes+=size(text);}
 a.lines=index;
 a.item.start.dispose();a.item.start=position(a.term,until);
 // Logical echo indices also cover an echo wrapping across the tail boundary.
 a.item.echo=a.item.echo.filter(e=>{if(e.end.isDisposed||e.end.line<until){e.start.dispose();e.end.dispose();return false;}return true;});
 a.cached=null;
}
function write(a,bytes){
 return new Promise((resolve,reject)=>{
   if(a.disposed){resolve();return;}
   a.waiters.add(resolve);
   try{a.term.write(bytes,()=>{a.waiters.delete(resolve);try{if(!a.disposed){a.cached=null;seal(a);}resolve();}catch(error){reject(error);}});}
   catch(error){a.waiters.delete(resolve);reject(error);}
 });
}
async function writeChunks(a,bytes){
 for(let i=0;i<bytes.length&&!a.disposed;i+=WRITE_CHUNK)await write(a,bytes.slice(i,i+WRITE_CHUNK));
}
function queue(a,work){
 a.pending++;
 a.chain=a.chain.then(()=>a.disposed||a.error?undefined:work()).catch(error=>{a.error=error;}).finally(()=>{a.pending--;notify(a);});
}
function value(a){
 const last=tail(a);if(!last)return null;
 const echo=new Set(a.echo);for(const i of last.echo)echo.add(a.lines+i);
 return {screen:a.chunks.join('')+last.screen,echo:[...echo],closed:last.closed,truncated:!!last.truncated};
}
function read(entry,item,include=false){
 const a=archives.get(item);if(!a)return null;const saved=value(a);if(!saved)return null;
 let lines=saved.screen.split('\n');if(!include){const echoes=new Set(saved.echo);lines=lines.filter((_,i)=>!echoes.has(i));}
 if(!saved.closed&&lines.length&&/^[\w.@:/()~\[\] -]{0,150}[#>$%]\s*$/.test(lines.at(-1)))lines.pop();
 while(lines.length&&!lines.at(-1).trim())lines.pop();return {text:lines.join('\n'),truncated:saved.truncated,archived:true};
}
function seed(entry,item,saved,expected=[],initialEcho=false){
 if(typeof root.Terminal!=='function')return;
 const term=new root.Terminal({cols:entry.xterm.cols,rows:entry.xterm.rows,scrollback:8192,allowProposedApi:true,convertEol:false,logLevel:'off'}),shadow={xterm:term};
 const block={start:term.registerMarker(0),end:null,closed:false,echo:[],expected:expected.slice()};
 const a={term,owner:entry,entry:shadow,item:block,value:null,chunks:[],lines:0,bytes:0,echo:new Set(saved.echo||[]),truncated:!!saved.truncated,pending:0,chain:Promise.resolve(),waiters:new Set()};
 if(initialEcho)a.echo.add(0);
 archives.set(item,a);entry._ntArchiveActive=item;
 queue(a,()=>writeChunks(a,saved.screen.replace(/\n/g,'\r\n')));
}
function begin(entry,item,screen,expected=[],echo=false){seed(entry,item,{screen,echo:[],closed:false,truncated:false},expected,echo);}
function feed(entry,bytes){
 const item=entry?._ntArchiveActive,a=item&&archives.get(item);if(!a?.term||a.finishing)return;
 // Bound individual parser writes, never the command's total bytes or rows.
 queue(a,()=>writeChunks(a,bytes));
}
function close(entry,item){
 const a=archives.get(item);if(entry?._ntArchiveActive===item)entry._ntArchiveActive=null;if(!a?.term||a.finishing)return;a.finishing=true;
 queue(a,()=>{a.item.end=position(a.term,a.term.buffer.normal.baseY+a.term.buffer.normal.cursorY);a.item.closed=true;a.cached=null;a.value=tail(a);a.term.dispose();a.term=null;});
}
function dispose(entry,item){const a=archives.get(item);if(a){a.disposed=true;a.term?.dispose();for(const resolve of a.waiters||[])resolve();a.waiters?.clear();}archives.delete(item);if(entry?._ntArchiveActive===item)entry._ntArchiveActive=null;}
function snapshot(entry,item){
 const a=archives.get(item);if(!a)return;
 const last=tail(a);if(!last)return;
 if(a.bytes+size(last.screen)<=TRANSFER_LIMIT)return value(a);
 // The bounded window-transfer payload is separate from complete live capture.
 // Assemble only the transferable suffix instead of duplicating the full
 // command at every background checkpoint.
 const parts=[last.screen];let bytes=size(last.screen);
 for(let i=a.chunks.length-1;i>=0&&bytes<TRANSFER_LIMIT;i--){parts.push(a.chunks[i]);bytes+=size(a.chunks[i]);}
 const encoded=new TextEncoder().encode(parts.reverse().join(''));let start=Math.max(0,encoded.length-TRANSFER_LIMIT);
 while(start<encoded.length&&(encoded[start]&0xc0)===0x80)start++;
 const suffix=new TextDecoder().decode(encoded.subarray(start)),boundary=suffix.indexOf('\n'),screen=boundary<0?suffix:suffix.slice(boundary+1);
 const removed=a.lines+last.screen.split('\n').length-screen.split('\n').length,echo=new Set(a.echo);
 for(const i of last.echo)echo.add(a.lines+i);
 return {screen,echo:[...echo].filter(i=>i>=removed).map(i=>i-removed),closed:last.closed,truncated:true};
}
function restore(entry,item,saved){if(!saved||typeof saved.screen!=='string'||size(saved.screen)>TRANSFER_LIMIT)return;if(item.closed){archives.set(item,{value:saved,chunks:[],lines:0,bytes:0,echo:new Set(),pending:0,chain:Promise.resolve()});return;}seed(entry,item,saved);}
function pending(entry,item){const a=archives.get(item);return !!(a?.pending||a?.error);}
function has(entry,item){const a=archives.get(item);if(!a)return null;return !!(a.bytes||tail(a)?.screen);}
async function flush(entry,item){
 const list=item?[item]:(entry?._ntOutputHistory||[]),active=list.map(i=>archives.get(i)).filter(Boolean);
 await Promise.all(active.map(a=>a.chain));
 for(const a of active)if(a.error)throw new Error('Command output could not finish processing. '+(a.error.message||String(a.error)));
}
root.NetworkToolkitTerminalOutputArchive=Object.freeze({begin,feed,close,dispose,read,snapshot,restore,flush,pending,has});
})(window);
