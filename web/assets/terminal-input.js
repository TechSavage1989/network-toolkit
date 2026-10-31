/* Edit only the visible, unsubmitted command. Keep positions, never keystrokes. */
(function(root){
'use strict';
const editors=new WeakMap(),bindings=new WeakSet(),MAX_EDIT=4096;
function revision(entry){return typeof terminalSessionRevision==='function'?terminalSessionRevision(entry.info.id):0;}
function normal(entry){return !!entry?.xterm&&entry.xterm.buffer.active===entry.xterm.buffer.normal&&entry.xterm.modes.mouseTrackingMode==='none';}
function clear(entry){const value=editors.get(entry);value?.marker?.dispose();editors.delete(entry);}
function startLine(buffer,row){while(row>0&&buffer.getLine(row)?.isWrapped)row--;return row;}
function setAnchor(entry,line,offset){
 clear(entry);if(!Number.isInteger(line)||line<0||line>10300||!Number.isInteger(offset)||offset<0||offset>MAX_EDIT*2)return;const b=entry.xterm.buffer.normal;
 const marker=entry.xterm.registerMarker(line-b.baseY-b.cursorY);
 if(marker)editors.set(entry,{marker,offset,revision:revision(entry)});
}
function noteInput(entry,data,origin='user'){
 if(origin==='edit'||origin==='pager')return;
 if(root.NetworkToolkitTerminalExtras?.pagerInput?.(entry,data)){clear(entry);return;}
 if(!normal(entry)){clear(entry);return;}
 if(!data||!terminalEntryAcceptsInput(entry))return;
 if(/[\r\n\x03\x04\x1a]/.test(data)){clear(entry);return;}
 // Accept printable text and line-editor keys, never terminal/mouse replies.
 if(!/^[^\x00-\x1f\x7f]+$/u.test(data)&&!/^\x1b(?:\[(?:[ABCDHF]|[1-8](?:;\d+)?[~ABCDHF])|O[ABCDHF])$/.test(data)&&!/^\x1b\[200~/.test(data)&&!/^\x7f$/.test(data))return;
 const b=entry.xterm.buffer.normal,row=b.baseY+b.cursorY,line=startLine(b,row),old=editors.get(entry);
 if(old&&!old.marker.isDisposed&&old.marker.line===line&&old.revision===revision(entry))return;
 setAnchor(entry,line,(row-line)*entry.xterm.cols+b.cursorX);
}
function range(entry){
 const value=editors.get(entry);
 if(!normal(entry)||!value||value.marker.isDisposed||value.revision!==revision(entry))return null;
 const term=entry.xterm,b=term.buffer.normal,line=value.marker.line,row=b.baseY+b.cursorY,cols=term.cols;
 if(startLine(b,row)!==line)return null;
 let last=row;while(last+1<b.length&&b.getLine(last+1)?.isWrapped)last++;
 const cursor=(row-line)*cols+b.cursorX,start=value.offset;
 if((last-line)*cols-start>MAX_EDIT*2)return null;
 let end=cursor;
 for(let y=line;y<=last;y++)for(let x=0;x<cols;x++){
   const cell=b.getLine(y)?.getCell(x),pos=(y-line)*cols+x;
   if(pos>=start&&cell?.getChars().trim())end=Math.max(end,pos+Math.max(1,cell.getWidth()));
 }
 if(start<0||cursor<start||end<=start||end-start>MAX_EDIT*2)return null;
 const positions=[];
 for(let pos=start;pos<end;){
   const y=line+Math.floor(pos/cols),x=pos%cols,cell=b.getLine(y)?.getCell(x);
   if(!cell||cell.isInvisible?.())return null;
   if(cell.getWidth()===0){pos++;continue;}
   // A wide glyph may leave a padding cell before wrapping.
   if(!cell.getChars()&&x===cols-1&&b.getLine(y+1)?.isWrapped){pos++;continue;}
   positions.push(pos);pos+=Math.max(1,cell.getWidth());
 }
 positions.push(end);
 if(positions.length>MAX_EDIT+1||!positions.includes(cursor))return null;
 return {line,start,end,cursor,positions,cols,paddingEnd:(last-line+1)*cols,arrow:term.modes.applicationCursorKeysMode?'\x1bO':'\x1b['};
}
function ready(entry){return terminalEntryAcceptsInput(entry)&&!entry.inputSending&&!entry.inputBytes&&!entry.xterm.options.disableStdin&&!root.NetworkToolkitTerminalInteraction?.suppressesInput();}
function movement(value,target){
 const from=value.positions.indexOf(value.cursor),to=value.positions.indexOf(target);
 if(from<0||to<0)return null;
 return (value.arrow+(to<from?'D':'C')).repeat(Math.abs(to-from));
}
function send(entry,data,reserveBytes=0){
 if(!ready(entry)||new TextEncoder().encode(data).length+reserveBytes>1024*1024)return false;
 return !data||enqueueTerminalInput(entry,data,false,'edit')===true;
}
function selection(entry){
 const value=range(entry),selected=entry?.xterm?.getSelectionPosition?.();
 if(!value||!selected)return null;
 const selectedStart=(selected.start.y-value.line)*value.cols+selected.start.x,selectedEnd=(selected.end.y-value.line)*value.cols+selected.end.x;
 // Normalize whitespace around the editable input, within this logical line.
 // A selection may include prompt spacing, but never prompt glyphs or output.
 if(selectedStart<0||selectedEnd>value.paddingEnd)return null;
 for(let pos=selectedStart;pos<value.start;pos++){
   const cell=entry.xterm.buffer.normal.getLine(value.line+Math.floor(pos/value.cols))?.getCell(pos%value.cols);
   if(!cell||cell.isInvisible?.()||cell.getWidth()!==1||cell.getChars().trim())return null;
 }
 const start=Math.max(selectedStart,value.start),end=Math.min(selectedEnd,value.end);
 const first=value.positions.indexOf(start),last=value.positions.indexOf(end);
 if(first<0||last<=first)return null;
 const arrows=movement(value,end);if(arrows===null)return null;
 return {data:arrows+'\x7f'.repeat(last-first),count:last-first};
}
function deleteSelection(entry,reserveBytes=0){
 const edit=selection(entry);if(!edit)return false;
 if(!send(entry,edit.data,reserveBytes))return false;
 root.NetworkToolkitTerminalInteraction?.cancelSelectionCopy?.(entry);
 entry.xterm.clearSelection();return true;
}
// Select only the visible, echoed command, never its prompt or earlier output.
// A password, full-screen app or uncertain/unacknowledged input retains
// the terminal's original Ctrl+A behavior.
function selectInput(entry){
 const value=range(entry);
 if(!value||!ready(entry))return false;
 const length=value.end-value.start;
 if(length<=0)return false;
 root.NetworkToolkitTerminalInteraction?.cancelSelectionCopy?.(entry);
 entry.xterm.select(value.start%value.cols,value.line+Math.floor(value.start/value.cols),length);
 entry.xterm.focus();
 return true;
}
function point(entry,event){
 const value=range(entry),screen=entry?.element?.querySelector('.xterm-screen'),rect=screen?.getBoundingClientRect();
 if(!value||!rect?.width||!rect.height||event.clientX<rect.left||event.clientX>=rect.right||event.clientY<rect.top||event.clientY>=rect.bottom)return null;
 const row=entry.xterm.buffer.normal.viewportY+Math.floor((event.clientY-rect.top)/rect.height*entry.xterm.rows);
 const column=Math.min(value.cols,Math.max(0,Math.round((event.clientX-rect.left)/rect.width*value.cols)));
 const position=(row-value.line)*value.cols+column;
 if(position<value.start||row<value.line||row>value.line+Math.floor(value.end/value.cols))return null;
 const target=value.positions.reduce((best,pos)=>Math.abs(pos-position)<Math.abs(best-position)?pos:best,value.positions[0]);
 return {value,target};
}
function contains(entry,event){return !!point(entry,event);}
function move(entry,event){
 const hit=point(entry,event);if(!hit)return false;
 const data=movement(hit.value,hit.target);if(data===null||!send(entry,data))return false;
 root.NetworkToolkitTerminalInteraction?.cancelSelectionCopy?.(entry);
 entry.xterm.clearSelection();entry.xterm.focus();return true;
}
function snapshot(entry){const value=editors.get(entry);return value&&!value.marker.isDisposed&&normal(entry)?{line:value.marker.line,offset:value.offset}:undefined;}
function restore(entry,value){clear(entry);if(normal(entry)&&Number.isInteger(value?.line)&&value.line>=0&&value.line<entry.xterm.buffer.normal.length&&Number.isInteger(value.offset)&&value.offset>=0&&value.offset<=MAX_EDIT*2)setAnchor(entry,value.line,value.offset);}
function bind(entry){
 if(!entry?.element||bindings.has(entry))return;bindings.add(entry);
 let gesture=null;
 const down=event=>{
   gesture=null;
   if(event.button!==0||event.ctrlKey||event.metaKey||event.altKey||event.shiftKey||event.detail>1||!contains(entry,event))return;
   gesture={x:event.clientX,y:event.clientY,revision:revision(entry)};
 };
 const click=event=>{
   const previous=gesture;gesture=null;
   if(!previous||event.button!==0||event.detail>1||previous.revision!==revision(entry)||Math.hypot(event.clientX-previous.x,event.clientY-previous.y)>4)return;
   if(entry.xterm.hasSelection())return;
   if(move(entry,event)){event.preventDefault();event.stopImmediatePropagation();}
 };
 entry.element.addEventListener('pointerdown',down,true);entry.element.addEventListener('click',click,true);
 const parsed=entry.xterm.onWriteParsed(()=>{
   const value=editors.get(entry),b=entry.xterm.buffer.normal;
   if(value&&(!normal(entry)||value.marker.isDisposed||value.revision!==revision(entry)||startLine(b,b.baseY+b.cursorY)!==value.marker.line))clear(entry);
 });
 entry.disposables.push({dispose(){clear(entry);parsed.dispose();entry.element.removeEventListener('pointerdown',down,true);entry.element.removeEventListener('click',click,true);}});
}
root.NetworkToolkitTerminalInput=Object.freeze({bind,noteInput,contains,move,selection,selectInput,deleteSelection,clear,snapshot,restore});
})(window);
