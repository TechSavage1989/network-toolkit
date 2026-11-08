/* A bounded textarea viewport. Native selection/IME operate on a small page;
   the scrollbar maps document lines to a bounded browser scroll range. */
(function(root){'use strict';
const RANGE=1000000,BUFFER_LINES=40;
function create({textarea:ta,gutter,container,session,changed=()=>{},message=()=>{}}){
 let disposed=false,navigating=0,frame=0,composing=false,selection={start:0,end:0,direction:'none'},virtualSelection=false;
 let page=session.state().page,starts=[0],topLine=0,requestedLine=null,barTarget=0,horizontalTarget=0;
 const abort=new AbortController(),listeners={signal:abort.signal};
 const bar=document.createElement('div'),spacer=document.createElement('div');
 bar.className='nt-editor-disk-scroll';bar.tabIndex=0;bar.setAttribute('role','scrollbar');bar.setAttribute('aria-label','Document position');bar.setAttribute('aria-orientation','vertical');spacer.style.height=RANGE+'px';bar.append(spacer);container.append(bar);container.classList.add('nt-editor-disk');
 const horizontal=document.createElement('div'),wide=document.createElement('div');horizontal.className='nt-editor-disk-horizontal';horizontal.tabIndex=0;horizontal.setAttribute('role','scrollbar');horizontal.setAttribute('aria-label','Position in long line');horizontal.setAttribute('aria-orientation','horizontal');wide.style.width=RANGE+'px';wide.style.height='1px';horizontal.append(wide);container.append(horizontal);
 const state=()=>session.state(),lineHeight=()=>parseFloat(getComputedStyle(ta).lineHeight)||20;
 const bounded=(n,a,b)=>Math.min(b,Math.max(a,n));
 const fail=error=>{if(!disposed)message(error.message,true);};
 const run=promise=>{void promise.catch(fail);};
 function index(){starts=[0];for(let at=-1;(at=page.text.indexOf('\n',at+1))>=0;)starts.push(at+1);}
 function atLine(offset){let a=0,b=starts.length;while(a+1<b){const m=Math.floor((a+b)/2);if(starts[m]<=offset)a=m;else b=m;}return a;}
 function capture(){if(!virtualSelection){selection={start:page.start+ta.selectionStart,end:page.start+ta.selectionEnd,direction:ta.selectionDirection||'none'};}return {...selection,top:topLine,left:ta.scrollLeft};}
 function select(start,end=start,direction='none',focus=false){selection={start,end,direction};virtualSelection=start<page.start||end>page.start+page.text.length;ta.setSelectionRange(bounded(start-page.start,0,page.text.length),bounded(end-page.start,0,page.text.length),direction);if(focus)ta.focus({preventScroll:true});changed(capture());}
 function paint(){if(disposed)return;frame=0;page={...page,text:ta.value};index();const height=lineHeight(),first=Math.max(0,Math.floor(ta.scrollTop/height)-3),last=Math.min(starts.length,first+Math.ceil(ta.clientHeight/height)+8);
  gutter.style.paddingTop=(first*height)+'px';gutter.style.transform='translateY('+(-ta.scrollTop)+'px)';
  gutter.replaceChildren(...Array.from({length:last-first},(_,i)=>{const row=document.createElement('div');row.style.height=height+'px';row.style.lineHeight=height+'px';row.textContent=String(page.line+first+i+1);return row;}));
  topLine=page.line+Math.max(0,Math.floor(ta.scrollTop/height));const total=state().metadata.lines;
  gutter.parentElement.style.width=(String(total).length+3)+'ch';gutter.parentElement.style.flexBasis=(String(total).length+3)+'ch';
  barTarget=total>1?bounded(topLine/(total-1),0,1)*(RANGE-bar.clientHeight):0;if(Math.abs(bar.scrollTop-barTarget)>1)bar.scrollTop=barTarget;
  bar.setAttribute('aria-valuemin','1');bar.setAttribute('aria-valuemax',String(total));bar.setAttribute('aria-valuenow',String(Math.min(total,topLine+1)));bar.hidden=total<=Math.ceil(ta.clientHeight/height);
  const long=page.lineUnits>root.NTKEditorDiskSession.PAGE_UNITS;horizontal.hidden=!long;container.classList.toggle('nt-editor-disk-long-line',long);
  if(long){const charWidth=(parseFloat(getComputedStyle(ta).fontSize)||13)*0.6,column=page.column+ta.scrollLeft/charWidth;horizontalTarget=column/Math.max(1,page.lineUnits)*(RANGE-horizontal.clientWidth);if(Math.abs(horizontal.scrollLeft-horizontalTarget)>1)horizontal.scrollLeft=horizontalTarget;horizontal.setAttribute('aria-valuemin','1');horizontal.setAttribute('aria-valuemax',String(page.lineUnits+1));horizontal.setAttribute('aria-valuenow',String(Math.floor(column)+1));}
  changed(capture());
 }
 function schedulePaint(){if(!frame)frame=requestAnimationFrame(paint);}
 function install(value,{line,caret,focus=false,keepSelection=true}={}){
  if(!value||disposed)return;const previous=capture();page=value;ta.value=page.text;index();
  if(caret!==undefined)select(caret,caret,'none',focus);else if(keepSelection)select(previous.start,previous.end,previous.direction);else select(page.start);
  ta.scrollTop=line===undefined?0:Math.max(0,line-page.line)*lineHeight();ta.scrollLeft=previous.left||0;paint();
 }
 async function load(target,options={}){if(disposed||composing)return;const ticket=++navigating;try{const value=await session.load(target);if(ticket!==navigating||disposed)return;install(value,options);}finally{if(ticket===navigating)requestedLine=null;}}
 async function goLine(line,focus=false){line=bounded(Math.floor(line),0,state().metadata.lines-1);requestedLine=line;const before=Math.max(0,line-BUFFER_LINES);await load({line:before},{line,keepSelection:true});
  if(disposed||requestedLine!==null&&requestedLine!==line)return;
  // A huge earlier line may occupy the complete page; ask for the actual
  // target line instead of allocating that line just to reach the next one.
  if(line>=page.line+starts.length)await load({line},{line});
  if(focus){const local=starts[bounded(line-page.line,0,starts.length-1)];select(page.start+local,page.start+local,'none',true);}
 }
 async function position(start,end=start,focus=true){start=bounded(start,0,state().metadata.units);end=bounded(end,start,state().metadata.units);
  if(start<page.start||end>page.start+page.text.length)await load({start:Math.max(0,start-4096)},{caret:start,focus});
  select(start,end,'none',focus);const local=atLine(Math.max(0,start-page.start));ta.scrollTop=Math.max(0,(local-2)*lineHeight());paint();
 }
 async function replace(text){if(composing)throw Error('Finish text composition before replacing text.');const view=capture();ta.readOnly=true;try{await session.replace(view.start,view.end,String(text).replace(/\r\n|\r/g,'\n'));const value=state().page;
  install(value,{caret:view.start+String(text).replace(/\r\n|\r/g,'\n').length,focus:true});await position(selection.start);}finally{ta.readOnly=state().blocked;}}
 async function history(redo){ta.readOnly=true;try{const view=capture(),value=await session.history(redo);install(value,{caret:Math.min(view.start,state().metadata.units),focus:true});await position(selection.start);}finally{ta.readOnly=state().blocked;}}
 function scroll(){schedulePaint();if(composing||requestedLine!==null||virtualSelection)return;const height=lineHeight(),first=page.line+Math.floor(ta.scrollTop/height),visible=Math.ceil(ta.clientHeight/height);
  if(page.start>0&&ta.scrollTop<height*8&&first>0)run(goLine(Math.max(0,first-1)));
  else if(page.start+page.text.length<state().metadata.units&&ta.scrollTop+ta.clientHeight>=ta.scrollHeight-height*8&&starts.length>visible+BUFFER_LINES)run(goLine(first));
 }
 function nativeSelection(){virtualSelection=false;capture();schedulePaint();}
 ta.addEventListener('pointerdown',()=>{virtualSelection=false;},listeners);
 ta.addEventListener('pointerup',nativeSelection,listeners);ta.addEventListener('keyup',()=>{if(!virtualSelection)nativeSelection();},listeners);
 ta.addEventListener('compositionstart',()=>{composing=true;},listeners);
 ta.addEventListener('compositionend',()=>{composing=false;session.change(ta.value);page={...page,text:ta.value};nativeSelection();},listeners);
 ta.addEventListener('input',event=>{event.stopImmediatePropagation();if(!composing){session.change(ta.value);page={...page,text:ta.value};}nativeSelection();},listeners);
 ta.addEventListener('beforeinput',event=>{
  if(event.inputType==='historyUndo'||event.inputType==='historyRedo'){event.preventDefault();event.stopImmediatePropagation();run(history(event.inputType==='historyRedo'));return;}
  if(!virtualSelection)return;
  if(event.isComposing){event.preventDefault();message('Place the cursor in the document before composing text across pages.',true);return;}
  const type=event.inputType;
  if(type.startsWith('delete')||type==='insertText'||type==='insertLineBreak'||type==='insertParagraph'){event.preventDefault();event.stopImmediatePropagation();run(replace(type.startsWith('delete')?'':type==='insertText'?event.data||'':'\n'));}
 },listeners);
 ta.addEventListener('paste',event=>{if(!virtualSelection)return;event.preventDefault();event.stopImmediatePropagation();run(replace(event.clipboardData.getData('text/plain')));},listeners);
 ta.addEventListener('scroll',scroll,listeners);
 ta.addEventListener('keydown',event=>{
  const ctrl=event.ctrlKey||event.metaKey,key=event.key;
  if(ctrl&&key.toLowerCase()==='a'){event.preventDefault();event.stopImmediatePropagation();select(0,state().metadata.units,'none',true);return;}
  if(ctrl&&['Home','End'].includes(key)){event.preventDefault();event.stopImmediatePropagation();const at=key==='Home'?0:state().metadata.units;run(position(at));return;}
  if(ctrl&&['z','y'].includes(key.toLowerCase())){event.preventDefault();event.stopImmediatePropagation();run(history(key.toLowerCase()==='y'||event.shiftKey));return;}
  if(composing||event.shiftKey||ctrl)return;
  const caret=page.start+ta.selectionEnd,atStart=ta.selectionStart===0&&ta.selectionEnd===0,atEnd=ta.selectionStart===page.text.length&&ta.selectionEnd===page.text.length;
  if(atStart&&page.start>0&&['ArrowLeft','ArrowUp','PageUp','Backspace'].includes(key)){event.preventDefault();event.stopImmediatePropagation();run(load({start:Math.max(0,page.start-16384)},{caret,focus:true}).then(()=>{const left=caret-page.start,step=left>=2&&/[\uDC00-\uDFFF]/.test(page.text[left-1])?2:1;if(key==='Backspace'){select(caret-step,caret);return replace('');}return position(Math.max(0,caret-step));}));}
  else if(atEnd&&caret<state().metadata.units&&['ArrowRight','ArrowDown','PageDown','Delete'].includes(key)){event.preventDefault();event.stopImmediatePropagation();run(load({start:Math.max(0,caret-4096)},{caret,focus:true}).then(()=>{const right=caret-page.start,step=/[\uD800-\uDBFF]/.test(page.text[right])?2:1;if(key==='Delete'){select(caret,caret+step);return replace('');}return position(Math.min(state().metadata.units,caret+step));}));}
 },listeners);
 bar.addEventListener('scroll',()=>{if(disposed||Math.abs(bar.scrollTop-barTarget)<=1)return;const line=Math.round(bar.scrollTop/Math.max(1,RANGE-bar.clientHeight)*(state().metadata.lines-1));if(line!==requestedLine)run(goLine(line));},listeners);
 bar.addEventListener('keydown',event=>{const total=state().metadata.lines,step=Math.max(1,Math.floor(ta.clientHeight/lineHeight()));const targets={Home:0,End:total-1,ArrowUp:topLine-1,ArrowDown:topLine+1,PageUp:topLine-step,PageDown:topLine+step};if(Object.hasOwn(targets,event.key)){event.preventDefault();run(goLine(targets[event.key]));}},listeners);
 horizontal.addEventListener('scroll',()=>{if(disposed||Math.abs(horizontal.scrollLeft-horizontalTarget)<=1)return;const column=Math.floor(horizontal.scrollLeft/Math.max(1,RANGE-horizontal.clientWidth)*page.lineUnits),lineStart=page.start-page.column;run(load({start:Math.min(state().metadata.units,lineStart+column)},{keepSelection:true}));},listeners);
 const resize=typeof ResizeObserver==='function'?new ResizeObserver(schedulePaint):null;resize?.observe(ta);
 return Object.freeze({load,goLine,position,replace,history,capture,selectAll:()=>select(0,state().metadata.units,'none',true),refresh(){if(ta.value!==state().page.text)install(state().page);else paint();},selection:()=>capture(),get composing(){return composing;},get position(){const offset=bounded(selection.start-page.start,0,page.text.length),line=atLine(offset);return {line:page.line+line,column:offset-starts[line]+(line===0?page.column:0)};},dispose(){disposed=true;navigating++;abort.abort();resize?.disconnect();if(frame)cancelAnimationFrame(frame);bar.remove();horizontal.remove();container.classList.remove('nt-editor-disk','nt-editor-disk-long-line');}});
}
root.NTKEditorDiskView=Object.freeze({create});
if(typeof module==='object'&&module.exports)module.exports={create};
})(globalThis);
