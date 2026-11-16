/* Document Map (owner request, like Notepad++'s View > Document Map): a panel
   at the right edge of the Text Editor's document area. It shows the document
   scaled down, with a translucent box over the part on screen. Clicking moves
   the view there, dragging the box scrolls, and the wheel over the map scrolls
   the document.

   Its width (owner request after v1.21.58: "the Document Map must be
   resizeable, and if sized large enough, actual text must be shown in it") is
   set by dragging the separator on its inner edge, with the arrow keys there,
   and reset by a double-click or Enter. It is an editor preference
   (editorPreferences.documentMapWidth, absent = the default 96 px, 64 px in
   narrow windows), between that default and half of the space the map shares
   with the document area, at most WIDTH_MAX.

   It never renders a whole large document. Below the text threshold the map
   has three levels of detail, chosen by how many text rows each map pixel
   holds:
     - every row gets at least one pixel: words are drawn as small blocks in the
       document's syntax colours (only possible for short documents);
     - up to PER_LINE lines per pixel: one thin bar per line (indent, length);
     - more: one bar per map pixel row from the line starts the editor already
       keeps (average length), sampling the first line of each row.
   Every level costs at most a few thousand lines or one pass over the map's
   pixel rows.

   Text mode: the map is the text area zoomed out to the map's width (its
   columns, at most TEXT_REFERENCE of them, fill the map, as the blocks do).
   Once that zoom gives a font of TEXT_FONT px or more, the map draws the document's characters instead of
   blocks, at most TEXT_FONT_MAX px (or TEXT_SHARE of the editor's font) so it
   stays an overview; rows are as tall as the scaled line height. A document
   taller than the map scrolls inside it: the box moves like a scroll bar thumb
   over the map and the text under it is the text on screen. Only the rows the
   map shows are drawn, each at most TEXT_COLUMNS characters (TEXT_ROWS rows at
   most). The colours are the editor's own: the HTML its language worker made
   for a page-held document (read in slices, never in one long task), and the
   same worker on its own channel for the loaded page of a disk-backed one;
   without them (Highlight off, plain text, past the editor's limit) the text
   is drawn in the normal text colour.

   A page-held document is mapped by the text area's own scroll geometry, so
   the box is exactly the visible share. A disk-backed document (only one page
   is in the page) is mapped by position like its scroll bar: by line without
   word wrap, by character offset with word wrap (the v1.21.53 scroll model;
   editor-disk-view.js). Detail (blocks or text) is drawn for the loaded page
   only; the rest is a position overview that fills in with the pages the
   reader has seen at the current revision (in text mode, a light texture).

   The comparison view has its own change map, so this one is hidden there. */
(function(root){
'use strict';

const ROW=3;              // map pixels per text row while the whole document fits
const MIN_BOX=10;         // the smallest box over the visible part
const PER_LINE=8;         // lines drawn one by one up to this many per map pixel
const SAMPLE=240;         // characters read from one line at most
const LARGE=2*1024*1024;  // a page-held document this long redraws after typing pauses
const PAUSE=250;          // milliseconds after the last edit of a large document
const TEXT=0,COMMENT=1,STRING=2,KEYWORD=3,NUMBER=4,PROPERTY=5;
// Width: the default (and smallest) width, in narrow windows too, the largest,
// and the share of the space beside the document area the map may take.
const WIDTH=96,NARROW_WIDTH=64,NARROW='(max-width:680px)',WIDTH_MAX=640,WIDTH_SHARE=0.5,STEP=16,BIG_STEP=64;
// Text mode: the smallest font that is drawn as text (CSS pixels), the
// largest, and the largest share of the editor's font; at most TEXT_COLUMNS
// characters per row and TEXT_ROWS rows per drawing.
// A text area wider than TEXT_REFERENCE columns is zoomed as if it had that
// many, so a very wide window reaches text mode too (longer rows are cut).
const TEXT_FONT=6,TEXT_FONT_MAX=9,TEXT_SHARE=0.6,TEXT_REFERENCE=120,TEXT_COLUMNS=320,TEXT_ROWS=400;
const SYNTAX_LIMIT=200000; // a disk page coloured for the map at most
const SYNTAX_DELAY=150;    // milliseconds before a disk page is coloured
const PARSE_SLICE=4;       // milliseconds of one slice of reading the editor's colours
const clamp=(value,low,high)=>Math.min(high,Math.max(low,value));

// ---- Text model (pure; scripts/editor-document-map.test.cjs) ----

// Comment markers for the map's colours: the language's own line comment, and
// "!" and "#" for device configurations, which mark comments that way.
function commentMarkers(style,language){
 const markers=[];
 if(Array.isArray(style)&&style[0])markers.push(String(style[0]));
 if(String(language||'').startsWith('network-'))for(const marker of ['!','#'])if(!markers.includes(marker))markers.push(marker);
 return markers;
}
const space=code=>code===32||code===9||code===13;

// The runs of one line as [column, length, kind], up to `limit` columns: words
// (the first word of a line in a language counts as a keyword, a word that
// starts with a digit as a number), quoted strings, and a comment to the end
// of the line. Tabs advance to the next tab stop.
function lineRuns(text,start=0,end=text.length,{tab=4,limit=SAMPLE,markers=[],keywords=false}={}){
 const out=[],stop=Math.min(end,start+limit*2);let col=0,i=start,first=true;
 while(i<stop&&col<limit){
  const code=text.charCodeAt(i);
  if(code===9){col+=tab-(col%tab);i++;continue;}
  if(code===32||code===13){col++;i++;continue;}
  if(markers.length&&(i===start||space(text.charCodeAt(i-1)))){
   let comment=false;for(const marker of markers)if(text.startsWith(marker,i)){comment=true;break;}
   if(comment){out.push([col,Math.max(1,Math.min(limit-col,stop-i)),COMMENT]);return out;}
  }
  if(code===34||code===39){
   let j=i+1;while(j<stop&&text.charCodeAt(j)!==code)j++;j=Math.min(stop,j+1);
   out.push([col,j-i,STRING]);col+=j-i;i=j;first=false;continue;
  }
  let j=i+1;while(j<stop){const next=text.charCodeAt(j);if(space(next)||next===34||next===39)break;j++;}
  out.push([col,j-i,first&&keywords?KEYWORD:code>=48&&code<=57?NUMBER:TEXT]);col+=j-i;i=j;first=false;
 }
 return out;
}

// Indent and length in columns, and whether the line is a comment, reading at
// most `limit` characters. For the bars of the overview levels.
function lineShape(text,start,end,{tab=4,limit=SAMPLE,markers=[]}={}){
 let col=0,i=start;const stop=Math.min(end,start+limit);
 while(i<stop){const code=text.charCodeAt(i);if(code===9)col+=tab-(col%tab);else if(code===32||code===13)col++;else break;i++;}
 let comment=false;if(i<stop)for(const marker of markers)if(text.startsWith(marker,i)){comment=true;break;}
 let length=end-start;if(length>0&&text.charCodeAt(end-1)===13)length--;
 return {indent:col,length:Math.max(col,length+(col-(i-start))),blank:i>=end,comment};
}

function lineStarts(text){const starts=[0];let at=-1;while((at=text.indexOf('\n',at+1))!==-1)starts.push(at+1);return starts;}
// Line starts at or before `offset` in a sorted list, as an index.
function before(starts,offset){let low=0,high=starts.length;while(low<high){const mid=(low+high)>>1;if(starts[mid]<=offset)low=mid+1;else high=mid;}return low;}
function findRow(cumulative,value){let low=0,high=cumulative.length-1;while(low+1<high){const mid=(low+high)>>1;if(cumulative[mid]<=value)low=mid;else high=mid;}return low;}

// Map geometry along one axis (text area pixels, lines or characters):
// `total` axis units hold `rows` text rows of `row` map pixels. Blocks draw
// ROW pixels per row until the document is taller than the map, then fit it to
// the map. Text (`scroll`) keeps its row height: a taller document scrolls
// inside the map by `offset` content pixels, the box moving from the top to the
// bottom of the map as the view moves from the start to the end, so the text
// under the box is the text on screen. `drag` is map pixels per axis unit for
// a dragged box.
function geometry({total,rows,height,start,end,row=ROW,scroll=false}){
 total=Math.max(1,total);
 const natural=Math.max(1,Math.max(1,rows)*row);
 const span=Math.max(0,Math.min(total,end)-Math.max(0,start));
 if(!scroll||natural<=height){
  const used=Math.max(1,Math.min(height,natural)),scale=used/total;
  const size=Math.min(used,Math.max(MIN_BOX,span*scale));
  const top=clamp(Math.max(0,start)*scale,0,Math.max(0,used-size));
  return {total,used,scale,span,offset:0,drag:scale,box:{top,height:size}};
 }
 const used=Math.max(1,height),scale=natural/total,size=Math.min(used,Math.max(MIN_BOX,span*scale)),last=Math.max(0,total-span);
 const share=last>0?clamp(Math.max(0,start)/last,0,1):0;
 return {total,used,scale,span,offset:share*(natural-used),drag:last>0&&used>size?(used-size)/last:scale,box:{top:share*(used-size),height:size}};
}
// The axis position to show at the top of the view for a pointer at map y,
// with `grab` the pointer's distance from the box top (a click centres the
// view on the text under the pointer).
function targetAt(g,y,grab){return clamp((y+(g.offset||0)-grab)/g.scale,0,Math.max(0,g.total-g.span));}
// The axis position after the box was dragged `moved` map pixels from where
// the view was at `from`.
function dragTarget(g,from,moved){return clamp(from+moved/(g.drag||g.scale),0,Math.max(0,g.total-g.span));}

// The map's width: the default, also the smallest, and the largest for the
// space the map shares with the document area.
function widthLimits({narrow=false,available=0}={}){
 const min=narrow?NARROW_WIDTH:WIDTH;
 return {min,max:Math.max(min,Math.min(WIDTH_MAX,Math.floor(Math.max(0,available)*WIDTH_SHARE)))};
}
// A saved width, or 0 (the default) for anything else.
function storedWidth(value){return Number.isInteger(value)&&value>=NARROW_WIDTH&&value<=WIDTH_MAX?value:0;}
function shownWidth(value,limits){return storedWidth(value)?clamp(value,limits.min,limits.max):limits.min;}

// Text mode for a map `width` pixels wide over a text area of `columns`
// columns (TEXT_REFERENCE at most) of `charWidth` pixels at `fontSize`: the
// scaled font (quarter pixels) and the scale, or null below the threshold.
function textZoom({width,columns,charWidth,fontSize}){
 if(!(width>0&&columns>0&&charWidth>0&&fontSize>0))return null;
 const fit=fontSize*width/(Math.min(columns,TEXT_REFERENCE)*charWidth);
 if(fit<TEXT_FONT)return null;
 const cap=Math.min(TEXT_FONT_MAX,Math.max(TEXT_FONT,fontSize*TEXT_SHARE)),font=Math.floor(Math.min(fit,cap)*4)/4;
 return {font,scale:font/fontSize};
}

// The editor's syntax colours: the HTML of its language worker (spans of
// class nt-ed-<kind>, nested, with &...; entities) read as the kind of every
// character offset. step(deadline) reads until then and can go on later;
// result() is {offsets, kinds, count, length} once all of it was read, or null
// if it could not be read.
const CLASS_KINDS={comment:COMMENT,string:STRING,keyword:KEYWORD,number:NUMBER,property:PROPERTY,variable:PROPERTY,type:PROPERTY,function:KEYWORD,operator:TEXT,punctuation:TEXT};
const SPAN='<span class="nt-ed-';
function classKind(tag,outer){
 if(tag.startsWith(SPAN)){const name=tag.slice(SPAN.length,tag.indexOf('"',SPAN.length));if(Object.hasOwn(CLASS_KINDS,name))return CLASS_KINDS[name];}
 const match=/class="([^"]*)"/.exec(tag);
 if(match)for(const name of match[1].split(/\s+/))if(name.startsWith('nt-ed-')&&Object.hasOwn(CLASS_KINDS,name.slice(6)))return CLASS_KINDS[name.slice(6)];
 return outer;
}
function syntaxReader(html){
 const source=String(html??''),stack=[];let at=0,offset=0,count=0,offsets=new Int32Array(64),kinds=new Uint8Array(64),state='reading';
 const mark=kind=>{
  if(count&&kinds[count-1]===kind)return;
  if(count&&offsets[count-1]===offset){count--;if(count&&kinds[count-1]===kind)return;}
  if(count===offsets.length){const o=new Int32Array(count*2),k=new Uint8Array(count*2);o.set(offsets);k.set(kinds);offsets=o;kinds=k;}
  offsets[count]=offset;kinds[count]=kind;count++;
 };
 mark(TEXT);
 return {
  get done(){return state!=='reading';},
  step(deadline=Infinity){
   const clock=root.performance;let n=0;
   while(at<source.length&&state==='reading'){
    const code=source.charCodeAt(at);
    if(code===60){
     const close=source.indexOf('>',at);if(close<0){state='failed';break;}
     if(source.charCodeAt(at+1)===47)stack.pop();
     else if(source.charCodeAt(close-1)!==47)stack.push(classKind(source.slice(at,close+1),stack.length?stack[stack.length-1]:TEXT));
     mark(stack.length?stack[stack.length-1]:TEXT);at=close+1;
    }else if(code===38){const semi=source.indexOf(';',at);at=semi>at&&semi-at<=10?semi+1:at+1;offset++;}
    else{let next=at+1;while(next<source.length){const c=source.charCodeAt(next);if(c===60||c===38)break;next++;}offset+=next-at;at=next;}
    if((++n&255)===0&&deadline!==Infinity&&clock&&clock.now()>deadline)return false;
   }
   if(state==='reading')state='read';
   return true;
  },
  result(){
   if(state!=='read')return null;
   let n=count;while(n>1&&offsets[n-1]>=offset)n--;
   return {offsets:offsets.subarray(0,n),kinds:kinds.subarray(0,n),count:n,length:offset};
  }
 };
}
function parseSyntax(html){const reader=syntaxReader(html);reader.step();return reader.result();}
// The kind of the characters at increasing offsets (offset - base in `runs`).
function syntaxCursor(runs,base=0){
 let index=-1;
 return offset=>{
  const at=offset-base;if(!runs||at<0||at>=runs.length)return TEXT;
  if(index<0||runs.offsets[index]>at)index=Math.max(0,before(runs.offsets,at)-1);
  while(index+1<runs.count&&runs.offsets[index+1]<=at)index++;
  return runs.kinds[index];
 };
}

// The characters of one map row: text[begin,end) from column 0, tabs to the
// next tab stop, other control characters as a blank and carriage returns as
// nothing, at most `columns` columns (never more than TEXT_COLUMNS). Returns
// the runs of one colour as [column, characters, kind] and the row as shown.
// The strings are built from character codes, never sliced from `text`: a
// slice keeps a whole (possibly 16 MB, possibly replaced) document alive.
const rowCodes=new Uint16Array(TEXT_COLUMNS);
const codesText=(from,to)=>{let out='';for(let at=from;at<to;at+=1024)out+=String.fromCharCode.apply(null,rowCodes.subarray(at,Math.min(to,at+1024)));return out;};
function rowCells(text,begin,end,{tab=4,columns=TEXT_COLUMNS,kindAt=null}={}){
 const cells=[],limit=Math.max(0,Math.min(columns,TEXT_COLUMNS)),stop=Math.min(end,begin+limit*2);
 let col=0,i=begin,runStart=-1,runKind=TEXT;
 const flush=()=>{if(runStart>=0&&col>runStart)cells.push([runStart,codesText(runStart,col),runKind]);runStart=-1;};
 while(i<stop&&col<limit){
  const code=text.charCodeAt(i);
  if(code===13||code===10){i++;continue;}
  if(code===9||code<32||code===127){flush();const next=code===9?Math.min(limit,col+tab-(col%tab)):col+1;rowCodes.fill(32,col,next);col=next;i++;continue;}
  const kind=kindAt?kindAt(i):TEXT;
  if(runStart<0||kind!==runKind){flush();runStart=col;runKind=kind;}
  rowCodes[col++]=code;i++;
 }
 flush();
 return {cells,text:codesText(0,col)};
}

// ---- The panel ----
function create(adapter){
 let host=null,handle=null,canvas=null,view=null,frame=0,timer=0,drawn='',drawnContent='',drawnDoc='',colors=null,colorKey='',drag=null,seen=null,widths=new Map(),pageCache=null,wrapCache=null,flowCache=null,shown=null,lastLayout=null,pending=null,seekFrame=0;
 let inner=null,resize=null,dragWidth=null,range=null,textRows=[],stamps=0,parsing=null,diskSyntaxState=null,diskTimer=0;
 const parsed=new WeakMap();
 const document=root.document;

 function mount(element){
  if(!element||host===element)return;
  host=element;
  if(!host.id)host.id='ntEditorDocumentMap';
  host.innerHTML='<div class="nt-editor-map-inner"><canvas class="nt-editor-map-canvas" aria-hidden="true"></canvas><div class="nt-editor-map-view" data-ed-map-view tabindex="0" role="scrollbar" aria-orientation="vertical" aria-label="Visible part of the document" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0"></div></div>';
  inner=host.firstElementChild;canvas=host.querySelector('canvas');view=host.querySelector('[data-ed-map-view]');
  // The separator on the map's inner edge, as the editor's other dividers.
  handle=document.createElement('div');
  handle.className='nt-editor-pane-resize nt-editor-map-resize';handle.hidden=host.hidden;
  for(const [name,value] of Object.entries({'data-ed-map-resize':'',role:'separator',tabindex:'0','aria-orientation':'vertical','aria-label':'Resize document map','aria-controls':host.id,title:'Drag to resize the document map. Wide enough, it shows the text. Arrow keys resize; Home / End use the limits; Enter or double-click resets.'}))handle.setAttribute(name,value);
  host.before(handle);
  handle.addEventListener('pointerdown',resizeDown);
  handle.addEventListener('pointermove',resizeMove);
  handle.addEventListener('pointerup',resizeUp);
  handle.addEventListener('pointercancel',resizeUp);
  handle.addEventListener('lostpointercapture',()=>{if(resize)finishResize();});
  handle.addEventListener('dblclick',event=>{event.preventDefault();adapter.resizing?.(true);commit(0);adapter.resizing?.(false);});
  handle.addEventListener('keydown',resizeKey);
  host.addEventListener('pointerdown',pointerDown);
  host.addEventListener('pointermove',pointerMove);
  host.addEventListener('pointerup',pointerUp);
  host.addEventListener('pointercancel',pointerUp);
  host.addEventListener('lostpointercapture',()=>{drag=null;host?.classList.remove('is-dragging');});
  host.addEventListener('wheel',wheel,{passive:false});
  view.addEventListener('keydown',keydown);
  // The map follows the editor without being told: scroll events (they do
  // not bubble, so in the capture phase) of every text area and scroll bar,
  // changed controls (Wrap, font size, indent, colours), the editor's palette,
  // font and view classes, the theme, and the size of the document area.
  // Edits and document switches arrive through schedule() from the tab strip.
  const editor=host.closest('.nt-editor')||host.parentElement;
  editor?.addEventListener('scroll',schedule,true);
  editor?.addEventListener('change',schedule);
  if(typeof root.ResizeObserver==='function'){const resized=new root.ResizeObserver(()=>{if(!resize)range=null;schedule();});resized.observe(host);const area=editor?.querySelector('.nt-editor-document-area');if(area)resized.observe(area);}
  root.addEventListener?.('resize',()=>{if(!resize)range=null;schedule();});
  if(typeof root.MutationObserver==='function'){
   if(document?.documentElement)new root.MutationObserver(()=>schedule()).observe(document.documentElement,{attributes:true,attributeFilter:['data-theme']});
   if(editor)new root.MutationObserver(()=>schedule()).observe(editor,{attributes:true,attributeFilter:['style','class','data-palette']});
  }
 }

 function schedule(){if(!frame&&host)frame=root.requestAnimationFrame(update);}
 // The width the map draws at: the width being dragged, else its own.
 function paintWidth(){return resize?dragWidth??resize.width:host.clientWidth;}

 function visible(){return !!adapter.enabled()&&!adapter.comparing();}

 // How long the map's own work took (checked by the browser tests); the
 // slices that read the editor's colours count as updates.
 const stats={updates:0,draws:0,maxUpdate:0,maxDraw:0,textDraws:0,maxTextDraw:0};
 const now=()=>root.performance?.now?.()??0;
 function timed(task){const started=now();try{return task();}finally{const spent=now()-started;stats.updates++;if(spent>stats.maxUpdate)stats.maxUpdate=spent;}}
 function update(){timed(run);}
 function run(){
  frame=0;if(!host)return;
  const show=visible();
  if(shown!==show){shown=show;host.hidden=!show;if(handle)handle.hidden=!show;range=null;adapter.layoutChanged?.();}
  if(!show){drag=null;lastLayout=null;textRows=[];return;}
  applyWidth();
  const layout=measure();lastLayout=layout;
  if(!layout){clear();return;}
  placeBox(layout);
  if(layout.key===drawn)return;
  // A large page-held document is redrawn once typing pauses; the box moves at once.
  if(layout.deferred&&drawn&&drawnDoc===layout.doc&&drawnContent!==layout.content){root.clearTimeout(timer);timer=root.setTimeout(()=>{timer=0;drawn='';schedule();},PAUSE);return;}
  draw(layout);drawn=layout.key;drawnContent=layout.content;drawnDoc=layout.doc;
 }

 function clear(){
  if(drawn==='empty')return;drawn='empty';drawnContent=drawnDoc='';view.hidden=true;textRows=[];
  const ctx=prepare();if(ctx){ctx.fillStyle=palette().bg;ctx.fillRect(0,0,paintWidth(),host.clientHeight);}
 }

 function prepare(){
  // While the separator is dragged the canvas keeps the drawing layer's
  // largest width and the map is drawn in its right-hand part.
  const width=paintWidth(),full=resize?resize.full:width,height=host.clientHeight,ratio=root.devicePixelRatio||1;
  if(!width||!height)return null;
  const w=Math.round(full*ratio),h=Math.round(height*ratio);
  if(canvas.width!==w)canvas.width=w;if(canvas.height!==h)canvas.height=h;
  if(canvas.style.width!==full+'px')canvas.style.width=full+'px';if(canvas.style.height!==height+'px')canvas.style.height=height+'px';
  const ctx=canvas.getContext('2d');if(!ctx)return null;
  ctx.setTransform(ratio,0,0,ratio,(full-width)*ratio,0);return ctx;
 }

 function palette(){
  const theme=document.documentElement?.dataset?.theme||'',overlay=adapter.overlay?.(),key=theme+'|'+(overlay?.dataset?.palette||'')+'|'+(overlay?.getAttribute?.('style')||'');
  if(colors&&key===colorKey)return colors;
  const style=root.getComputedStyle(host),value=(name,fallback)=>style.getPropertyValue(name).trim()||fallback;
  const fg=value('--nt-code-fg',style.color||'#888');
  colors={bg:value('--nt-code-bg','transparent'),fg,band:fg,kinds:[fg,value('--nt-code-comment',fg),value('--nt-code-string',fg),value('--nt-code-keyword',fg),value('--nt-code-number',fg),value('--nt-code-property',fg)]};
  colorKey=key;drawn='';return colors;
 }

 function charWidth(font){
  if(widths.has(font))return widths.get(font);
  const ctx=document.createElement('canvas').getContext('2d');let width=8;
  if(ctx){ctx.font=font;width=ctx.measureText('0').width||8;}
  if(widths.size>64)widths.clear();
  widths.set(font,width);return width;
 }

 function textMetrics(textarea){
  const style=root.getComputedStyle(textarea),number=(value,fallback)=>{const n=parseFloat(value);return Number.isFinite(n)?n:fallback;};
  const lineHeight=number(style.lineHeight,20),font=style.font||style.fontSize+' '+style.fontFamily;
  const left=number(style.paddingLeft,0),right=number(style.paddingRight,0),width=charWidth(font);
  return {lineHeight,top:number(style.paddingTop,0),bottom:number(style.paddingBottom,0),font,fontSize:number(style.fontSize,13),family:style.fontFamily||'monospace',weight:style.fontWeight||'',charWidth:width,columns:Math.max(16,Math.floor(Math.max(1,textarea.clientWidth-left-right)/width))};
 }

 // Text mode for this map width, with the font the canvas draws, its
 // character width and the columns that fit; or null (blocks).
 function zoomFor(m,width,columns,editorChar){
  const zoom=textZoom({width,columns,charWidth:editorChar||m.charWidth,fontSize:m.fontSize});if(!zoom)return null;
  const weight=m.weight&&!['400','normal'].includes(m.weight)?m.weight+' ':'',css=weight+zoom.font+'px '+m.family,cw=charWidth(css);
  return {...zoom,css,cw,columns:Math.max(1,Math.min(TEXT_COLUMNS,Math.floor(width/Math.max(0.1,cw))))};
 }

 function markersFor(d){const language=adapter.language(d)||'text';return {language,markers:commentMarkers(adapter.commentStyle?.(language),language),keywords:language!=='text'&&!!adapter.syntax()};}

 // The layout of the active document for this frame, or null.
 function measure(){
  const d=adapter.document();if(!d||d.loading||d.staged)return null;
  const textarea=adapter.textarea();if(!textarea||!textarea.isConnected)return null;
  const height=host.clientHeight,width=paintWidth();if(!height||!width)return null;
  if(d.disk){
   const paged=adapter.diskView();const state=paged?.mapState?.();
   if(!state||state.aligned||!state.lines)return null;
   if(state.whole)return textLayout(d,textarea,state.page.text,null,state.revision,height,width,paged,state.page);
   return state.flow?offsetLayout(d,textarea,state,height,width,paged):lineLayout(d,textarea,state,height,width,paged);
  }
  return textLayout(d,textarea,d.content,adapter.lineStarts(d),d.editRevision,height,width,null,null);
 }

 // What every layout's drawing depends on besides the text (palette() first,
 // so the colour key is current).
 function common(d,height,width){
  palette();
  return [height,width,root.devicePixelRatio||1,colorKey,adapter.tabSize(),adapter.language(d),adapter.syntax()].join(':');
 }

 // ---- The editor's colours for text mode ----
 // A page-held document: the HTML the editor's language worker made for
 // exactly this text (adapter.highlighted), read in slices between frames.
 function pageSyntax(d,text){
  const cache=adapter.highlighted?.(d,text);if(!cache||typeof cache.html!=='string')return null;
  let entry=parsed.get(cache);
  if(!entry){entry={reader:syntaxReader(cache.html),runs:null,stamp:0,length:text.length};parsed.set(cache,entry);}
  if(entry.runs)return entry;
  if(!entry.reader.done)readLater(entry);
  return null;
 }
 function readLater(entry){
  if(parsing===entry)return;
  parsing=entry;
  const slice=()=>{
   if(parsing!==entry)return;
   const done=timed(()=>entry.reader.step(now()+PARSE_SLICE));
   if(!done){root.setTimeout(slice,0);return;}
   parsing=null;const runs=entry.reader.result();
   if(runs&&runs.length===entry.length){entry.runs=runs;entry.stamp=++stamps;schedule();}
  };
  root.setTimeout(slice,0);
 }
 // A disk-backed page: the same worker colours the page on its own channel,
 // so it never cancels the editor's own request.
 function diskSyntax(d,page){
  const language=adapter.language(d)||'text',text=page.text;
  if(!adapter.syntax()||language==='text'||!text||text.length>SYNTAX_LIMIT)return null;
  const known=diskSyntaxState;
  if(known&&known.text===text&&known.language===language)return known.runs?known:null;
  const entry={text,language,runs:null,stamp:0};diskSyntaxState=entry;
  root.clearTimeout(diskTimer);
  diskTimer=root.setTimeout(()=>{
   const service=root.NTKEditorLanguageService;if(!service||diskSyntaxState!==entry)return;
   service.highlight(text,language,{channel:'map'}).then(value=>{
    if(diskSyntaxState!==entry||!value||value.limited)return;
    const runs=parseSyntax(value.html);
    if(runs&&runs.length===text.length){entry.runs=runs;entry.stamp=++stamps;schedule();}
   }).catch(()=>{});
  },SYNTAX_DELAY);
  return null;
 }

 // ---- Text mode drawing ----
 function textStart(ctx,zoom){ctx.font=zoom.css;ctx.textBaseline='middle';ctx.textAlign='left';ctx.globalAlpha=1;}
 // One row of text, its middle at y + height / 2; recorded for the tests.
 function textRow(ctx,c,text,from,to,y,height,zoom,tab,kindAt,line,row,out,base=0){
  const shown=rowCells(text,from,to,{tab,columns:zoom.columns,kindAt}),middle=y+height/2;
  for(const [col,part,kind] of shown.cells){ctx.fillStyle=c.kinds[kind]||c.fg;ctx.fillText(part,col*zoom.cw,middle);}
  out.push({line,row,offset:base+from,y:Math.round(y*100)/100,text:shown.text});
 }
 function timedText(task){const started=now();try{task();}finally{const spent=now()-started;stats.textDraws++;if(spent>stats.maxTextDraw)stats.maxTextDraw=spent;}}

 // A page-held document, or a disk-backed one whose page is all of it: the
 // text area's scroll geometry is the axis.
 function textLayout(d,textarea,text,starts,revision,height,width,paged,page){
  const m=textMetrics(textarea),wrapped=textarea.wrap==='soft';
  if(!starts){if(pageCache?.text!==text)pageCache={text,starts:lineStarts(text)};starts=pageCache.starts;}
  const total=Math.max(1,textarea.scrollHeight),start=textarea.scrollTop,end=start+textarea.clientHeight,rows=total/m.lineHeight;
  const zoom=zoomFor(m,width,m.columns);
  const g=zoom?geometry({total,rows,height,start,end,row:m.lineHeight*zoom.scale,scroll:true}):geometry({total,rows,height,start,end});
  const syntax=!zoom?null:page?diskSyntax(d,page):pageSyntax(d,text);
  const content=[d.uid+':'+(d.disk?'p':'t'),revision,text.length].join('|');
  const rest=[total,textarea.clientWidth,m.lineHeight,m.columns,wrapped,common(d,height,width),zoom?'text:'+zoom.font+':'+zoom.columns+':'+(syntax?.stamp||0):'blocks'].join('|');
  return {kind:'text',mode:zoom?'text':'blocks',zoom,g,start,columns:m.columns,wrapped,deferred:!d.disk&&text.length>LARGE,doc:d.uid,content,
   key:content+'|'+rest+'#'+(zoom?Math.round(g.offset):''),
   label:()=>Math.round(clamp(start/Math.max(1,total-textarea.clientHeight),0,1)*100),
   draw:ctx=>zoom?drawTextRows(ctx,d,text,starts,m,wrapped,g,width,total,zoom,syntax):drawText(ctx,d,text,starts,m,wrapped,g,width,total),
   seek:value=>{textarea.scrollTop=value;},
   scroll:delta=>{if(paged?.wheelBy)paged.wheelBy(delta);else textarea.scrollTop+=delta;},
   step:{line:m.lineHeight,page:Math.max(m.lineHeight,textarea.clientHeight-m.lineHeight)}};
 }

 // Rows of each line with word wrap: measured heights when the editor keeps
 // them for this text, otherwise characters per row. Cached per text and width.
 function wrapRows(d,text,starts,columns,tab,lineHeight){
  const exact=adapter.wrapHeights?.(d,text,starts.length);
  const key=[text.length,starts.length,columns,tab,lineHeight,exact?exact.length:0].join(':');
  if(wrapCache?.text===text&&wrapCache.key===key&&wrapCache.exact===exact)return wrapCache;
  const n=starts.length,cumulative=new Float64Array(n+1);
  if(exact&&exact.length===n){let sum=0;for(let i=0;i<n;i++){cumulative[i]=sum;sum+=Math.max(1,exact[i]/lineHeight);}cumulative[n]=sum;}
  else{let sum=0;for(let i=0;i<n;i++){cumulative[i]=sum;const length=(i+1<n?starts[i+1]-1:text.length)-starts[i];sum+=Math.max(1,Math.ceil(length/columns));}cumulative[n]=sum;}
  wrapCache={text,key,exact,cumulative,total:cumulative[n]};return wrapCache;
 }

 function drawText(ctx,d,text,starts,m,wrapped,g,width,total){
  textRows=[];
  const c=palette(),{markers,keywords}=markersFor(d),tab=adapter.tabSize(),n=starts.length;
  const columns=m.columns,cw=width/columns,usable=Math.max(1,total-m.top-m.bottom);
  const cumulative=wrapped?wrapRows(d,text,starts,columns,tab,m.lineHeight).cumulative:null;
  // Text-area pixels per text row: rows are spread over the scroll height.
  const rowPixels=usable/Math.max(1,wrapped?cumulative[n]:n),rowHeight=rowPixels*g.scale;
  const lineTop=i=>(m.top+(wrapped?cumulative[i]:i)*rowPixels)*g.scale;
  const lineEnd=i=>i+1<n?starts[i+1]-1:text.length;
  ctx.fillStyle=c.bg;ctx.fillRect(0,0,width,g.used);
  // The words of line i (from wrapped row `from`, `rows` rows of it) as blocks.
  const paint=(i,y,height,alpha,from=0,rows=Infinity)=>{
   const begin=wrapped&&from?Math.min(lineEnd(i),starts[i]+from*columns):starts[i];
   const limit=wrapped?Math.min(SAMPLE*8,Math.max(1,Math.min(rows,Math.ceil(cumulative[i+1]-cumulative[i])-from))*columns):columns;
   for(const [col,length,kind] of lineRuns(text,begin,lineEnd(i),{tab,limit,markers,keywords:keywords&&!from})){
    ctx.fillStyle=c.kinds[kind];ctx.globalAlpha=alpha*(kind===TEXT?0.75:1);
    if(!wrapped){ctx.fillRect(col*cw,y,Math.max(0.6,Math.min(length,columns-col)*cw),height);continue;}
    for(let at=col,left=length;left>0;){const row=Math.floor(at/columns),x=at%columns,take=Math.min(left,columns-x);ctx.fillRect(x*cw,y+row*rowHeight,Math.max(0.6,take*cw),height);at+=take;left-=take;}
   }
  };
  if(rowHeight>=1){
   // Every text row has a pixel row or more: the words of every line.
   const block=Math.max(1,Math.min(rowHeight-0.5,rowHeight*0.7));
   for(let i=0;i<n;i++){const y=lineTop(i);if(y>g.used)break;paint(i,y,block,1);}
  }else if(n<=g.used*PER_LINE){
   // A few lines per pixel row: every line, thin and translucent.
   for(let i=0;i<n;i++)paint(i,lineTop(i),Math.max(0.35,rowHeight),0.55);
  }else{
   // More: the line at the top of each pixel row.
   for(let y=0;y<g.used;y++){
    const at=(y/g.scale-m.top)/rowPixels,line=clamp(wrapped?findRow(cumulative,at):Math.floor(at),0,n-1);
    paint(line,y,1,0.55,wrapped?Math.max(0,Math.floor(at-cumulative[line])):0,1);
   }
  }
  ctx.globalAlpha=1;
 }

 // Text mode of a page-held document: the rows in the map's window only,
 // from the row at its top, as the text area wraps them.
 function drawTextRows(ctx,d,text,starts,m,wrapped,g,width,total,zoom,syntax){
  timedText(()=>{
   const c=palette(),tab=adapter.tabSize(),n=starts.length,usable=Math.max(1,total-m.top-m.bottom),out=[];
   const cumulative=wrapped?wrapRows(d,text,starts,m.columns,tab,m.lineHeight).cumulative:null;
   const rowPixels=usable/Math.max(1,wrapped?cumulative[n]:n),height=rowPixels*g.scale;
   const lineEnd=i=>i+1<n?starts[i+1]-1:text.length,kindAt=syntax?syntaxCursor(syntax.runs):null;
   ctx.fillStyle=c.bg;ctx.fillRect(0,0,width,g.used);textStart(ctx,zoom);
   const at=Math.max(0,(g.offset/g.scale-m.top)/rowPixels);
   let line=clamp(wrapped?findRow(cumulative,at):Math.floor(at),0,n-1),row=wrapped?Math.max(0,Math.floor(at-cumulative[line])):0;
   for(let count=0;line<n&&count<TEXT_ROWS;){
    const y=(m.top+((wrapped?cumulative[line]:line)+row)*rowPixels)*g.scale-g.offset;
    if(y>g.used)break;
    const stop=lineEnd(line),from=wrapped?Math.min(stop,starts[line]+row*m.columns):starts[line],to=wrapped?Math.min(stop,from+m.columns):stop;
    if(y+height>=0){textRow(ctx,c,text,from,to,y,height,zoom,tab,kindAt,line,row,out);count++;}
    if(wrapped&&row+1<Math.round(cumulative[line+1]-cumulative[line])&&from+m.columns<stop)row++;else{line++;row=0;}
   }
   textRows=out;
  });
 }

 // Rows the reader has seen at this revision and size: bar width (0..1) and
 // whether the row starts with a comment. The rows of pages not read are a
 // faint texture as wide as the document's average line (`estimate`).
 function seenRows(key,used){
  if(seen?.key!==key)seen={key,widths:new Float32Array(Math.ceil(used)).fill(-1),comments:new Uint8Array(Math.ceil(used))};
  return seen;
 }
 function drawSeen(ctx,c,record,g,width,estimate){
  texture(ctx,c,g,width,estimate);
  for(let y=0;y<record.widths.length;y++){
   const w=record.widths[y];if(w<0)continue;
   ctx.globalAlpha=1;ctx.fillStyle=c.bg;ctx.fillRect(0,y,width,1);
   if(w>0){ctx.globalAlpha=0.6;ctx.fillStyle=record.comments[y]?c.kinds[COMMENT]:c.fg;ctx.fillRect(0,y,Math.max(0.6,w*width),1);}
  }
  ctx.globalAlpha=1;
 }
 function texture(ctx,c,g,width,estimate){ctx.fillStyle=c.band;ctx.globalAlpha=0.16;ctx.fillRect(0,0,Math.max(1,clamp(estimate,0,1)*width),g.used);ctx.globalAlpha=1;}

 // A disk-backed document without word wrap: the axis is the line number.
 function lineLayout(d,textarea,state,height,width,paged){
  const m=textMetrics(textarea),cw=state.characterWidth||m.charWidth,columns=Math.max(16,Math.floor(textarea.clientWidth/Math.max(1,cw)));
  const start=state.top+(state.fraction||0),end=start+state.rows,zoom=zoomFor(m,width,columns,cw);
  const g=zoom?geometry({total:state.lines,rows:state.lines,height,start,end,row:(state.rowHeight||m.lineHeight)*zoom.scale,scroll:true}):geometry({total:state.lines,rows:state.lines,height,start,end});
  const page=state.page;if(pageCache?.text!==page.text)pageCache={text:page.text,starts:lineStarts(page.text)};
  const syntax=zoom?diskSyntax(d,page):null;
  const base=[d.uid+':l',state.revision,state.lines,common(d,height,width),columns].join('|'),content=[base,page.start,page.text.length].join('|');
  return {kind:'lines',mode:zoom?'text':'blocks',zoom,g,start,columns,deferred:false,doc:d.uid,content,
   key:content+'|'+(zoom?'text:'+zoom.font+':'+zoom.columns+':'+(syntax?.stamp||0)+'#'+Math.round(g.offset):'blocks#'),
   label:()=>Math.round(clamp(state.top/Math.max(1,state.lines-state.rows),0,1)*100),
   draw:ctx=>zoom?drawPageLines(ctx,d,state,g,width,zoom,syntax):drawLines(ctx,d,state,g,width,columns,base),
   seek:value=>paged.mapSeek({line:value}),
   scroll:delta=>paged.wheelBy(delta),
   step:{line:1,page:Math.max(1,state.rows-1)}};
 }
 function drawLines(ctx,d,state,g,width,columns,base){
  textRows=[];
  const page=state.page,c=palette(),record=seenRows(base,g.used),{markers,keywords}=markersFor(d),tab=adapter.tabSize(),starts=pageCache.starts,text=page.text,n=starts.length,cw=width/columns;
  const end=i=>i+1<n?starts[i+1]-1:text.length,rowHeight=g.scale;
  // Record what this page shows, then draw the known rows and the page.
  for(let i=0;i<n;i++){
   const y0=Math.floor((page.line+i)*g.scale),y1=Math.min(record.widths.length,Math.max(y0+1,Math.floor((page.line+i+1)*g.scale)));
   const shape=lineShape(text,starts[i],end(i),{tab,markers}),w=shape.blank?0:Math.min(1,Math.max(0,shape.length)/columns);
   for(let y=y0;y<y1;y++)if(record.widths[y]<w||record.widths[y]<0){record.widths[y]=w;record.comments[y]=shape.comment?1:0;}
  }
  ctx.fillStyle=c.bg;ctx.fillRect(0,0,width,g.used);drawSeen(ctx,c,record,g,width,(state.units/Math.max(1,state.lines)-1)/columns);
  // The loaded page in detail while it has at most PER_LINE lines per pixel.
  if(rowHeight*PER_LINE>=1){
   const top=page.line*g.scale,detail=rowHeight>=1,block=detail?Math.max(1,Math.min(rowHeight-0.5,rowHeight*0.7)):Math.max(0.35,rowHeight),alpha=detail?1:0.55;
   ctx.globalAlpha=1;ctx.fillStyle=c.bg;ctx.fillRect(0,top,width,Math.max(1,n*rowHeight));
   for(let i=0;i<n;i++){const y=(page.line+i)*g.scale;if(y>g.used)break;for(const [col,length,kind] of lineRuns(text,starts[i],end(i),{tab,limit:columns,markers,keywords})){ctx.fillStyle=c.kinds[kind];ctx.globalAlpha=alpha*(kind===TEXT?0.75:1);ctx.fillRect(col*cw,y,Math.max(0.6,Math.min(length,columns-col)*cw),block);}}
   ctx.globalAlpha=1;
  }
 }
 // Text mode without word wrap: the loaded page's lines in the map's window;
 // the rest of the document is a light texture as wide as its average line.
 function drawPageLines(ctx,d,state,g,width,zoom,syntax){
  timedText(()=>{
   const page=state.page,c=palette(),tab=adapter.tabSize(),starts=pageCache.starts,text=page.text,n=starts.length,height=g.scale,out=[];
   const end=i=>i+1<n?starts[i+1]-1:text.length,kindAt=syntax?syntaxCursor(syntax.runs):null;
   ctx.fillStyle=c.bg;ctx.fillRect(0,0,width,g.used);texture(ctx,c,g,width,(state.units/Math.max(1,state.lines)-1)/zoom.columns);
   const top=page.line*height-g.offset,bottom=top+n*height;
   if(Math.min(g.used,bottom)>Math.max(0,top)){ctx.fillStyle=c.bg;ctx.fillRect(0,Math.max(0,top),width,Math.min(g.used,bottom)-Math.max(0,top));}
   textStart(ctx,zoom);
   for(let i=clamp(Math.floor(-top/height),0,n),count=0;i<n&&count<TEXT_ROWS;i++){
    const y=top+i*height;if(y>g.used)break;if(y+height<0)continue;
    textRow(ctx,c,text,starts[i],end(i),y,height,zoom,tab,kindAt,page.line+i,0,out,page.start);count++;
   }
   textRows=out;
  });
 }

 // A disk-backed document with word wrap: the axis is the character offset,
 // and the map is as tall as the document's estimated wrapped rows.
 function offsetLayout(d,textarea,state,height,width,paged){
  const m=textMetrics(textarea),columns=Math.max(1,state.wrapColumns||80),zoom=zoomFor(m,width,columns,state.characterWidth||m.charWidth);
  const start=state.offset,end=start+state.visible,rowHeight=state.rowHeight||m.lineHeight;
  const g=zoom?geometry({total:state.units,rows:state.estimatedRows,height,start,end,row:rowHeight*zoom.scale,scroll:true}):geometry({total:state.units,rows:state.estimatedRows,height,start,end});
  const page=state.page,base=[d.uid+':o',state.revision,state.units,common(d,height,width),state.wrapColumns].join('|'),content=[base,page.start,page.text.length].join('|');
  const syntax=zoom?diskSyntax(d,page):null;
  return {kind:'offsets',mode:zoom?'text':'blocks',zoom,g,start,columns,deferred:false,doc:d.uid,content,
   key:content+'|'+(zoom?'text:'+zoom.font+':'+zoom.columns+':'+(syntax?.stamp||0)+':'+start+'#'+Math.round(g.offset):'blocks#'),
   label:()=>Math.round(clamp(start/Math.max(1,state.units-state.visible),0,1)*100),
   draw:ctx=>zoom?drawPageFlow(ctx,d,state,g,width,zoom,columns,rowHeight*zoom.scale,syntax):drawOffsets(ctx,state,g,width,base),
   seek:value=>paged.mapSeek({offset:value}),
   scroll:delta=>paged.wheelBy(delta),
   step:{line:state.rowHeight||20,page:Math.max(20,textarea.clientHeight-(state.rowHeight||20))}};
 }
 function drawOffsets(ctx,state,g,width,base){
  textRows=[];
  const page=state.page;
  if(pageCache?.text!==page.text)pageCache={text:page.text,starts:lineStarts(page.text)};
  const c=palette(),record=seenRows(base,g.used),text=page.text,starts=pageCache.starts,columns=Math.max(1,state.wrapColumns||80);
  const first=Math.floor(page.start*g.scale),last=Math.min(record.widths.length,Math.ceil((page.start+text.length)*g.scale));
  for(let y=first;y<last;y++){
   const a=clamp(Math.floor(y/g.scale)-page.start,0,text.length),b=clamp(Math.ceil((y+1)/g.scale)-page.start,a,text.length);if(b<=a)continue;
   // Line breaks in [a,b): line starts in (a,b].
   const breaks=before(starts,b)-before(starts,a);
   const chars=b-a-breaks,rows=Math.max(1,breaks+chars/columns);
   record.widths[y]=Math.min(1,chars/(rows*columns));record.comments[y]=0;
  }
  ctx.fillStyle=c.bg;ctx.fillRect(0,0,width,g.used);drawSeen(ctx,c,record,g,width,(state.units-state.lines+1)/Math.max(1,state.estimatedRows*columns));
 }
 // Wrapped rows of the loaded page (characters per row, as the blocks wrap):
 // the first row of each line. Cached per page and width.
 function pageRows(text,starts,columns){
  if(flowCache?.text===text&&flowCache.columns===columns)return flowCache.cumulative;
  const n=starts.length,cumulative=new Float64Array(n+1);let sum=0;
  for(let i=0;i<n;i++){cumulative[i]=sum;const length=(i+1<n?starts[i+1]-1:text.length)-starts[i];sum+=Math.max(1,Math.ceil(length/columns));}
  cumulative[n]=sum;flowCache={text,columns,cumulative};return cumulative;
 }
 // Text mode with word wrap: the row at the top of the view is drawn at the
 // top of the box, the page's other rows around it; elsewhere the texture.
 function drawPageFlow(ctx,d,state,g,width,zoom,columns,height,syntax){
  timedText(()=>{
   const page=state.page;
   if(pageCache?.text!==page.text)pageCache={text:page.text,starts:lineStarts(page.text)};
   const c=palette(),tab=adapter.tabSize(),text=page.text,starts=pageCache.starts,n=starts.length,cumulative=pageRows(text,starts,columns),out=[];
   const end=i=>i+1<n?starts[i+1]-1:text.length,kindAt=syntax?syntaxCursor(syntax.runs):null;
   const at=clamp(state.offset-page.start,0,text.length),line=clamp(before(starts,at)-1,0,n-1),anchor=cumulative[line]+Math.floor((at-starts[line])/columns);
   const rowY=j=>g.box.top+(j-anchor)*height;
   ctx.fillStyle=c.bg;ctx.fillRect(0,0,width,g.used);texture(ctx,c,g,width,(state.units-state.lines+1)/Math.max(1,state.estimatedRows*columns));
   const top=rowY(0),bottom=rowY(cumulative[n]);
   if(Math.min(g.used,bottom)>Math.max(0,top)){ctx.fillStyle=c.bg;ctx.fillRect(0,Math.max(0,top),width,Math.min(g.used,bottom)-Math.max(0,top));}
   textStart(ctx,zoom);
   for(let j=Math.max(0,Math.floor(anchor-g.box.top/height)-1),count=0;j<cumulative[n]&&count<TEXT_ROWS;j++){
    const y=rowY(j);if(y>g.used)break;if(y+height<0)continue;
    const i=findRow(cumulative,j),row=j-cumulative[i],stop=end(i),from=Math.min(stop,starts[i]+row*columns),to=Math.min(stop,from+columns);
    textRow(ctx,c,text,from,to,y,height,zoom,tab,kindAt,page.line+i,row,out,page.start);count++;
   }
   textRows=out;
  });
 }

 function draw(layout){
  const started=now();
  try{paint(layout);}finally{const spent=now()-started;stats.draws++;if(spent>stats.maxDraw)stats.maxDraw=spent;}
 }
 function paint(layout){
  const ctx=prepare();if(!ctx)return;
  const c=palette();ctx.globalAlpha=1;ctx.fillStyle=c.bg;ctx.fillRect(0,0,paintWidth(),host.clientHeight);
  try{layout.draw(ctx);}catch(error){root.console?.error?.('The document map could not be drawn.',error);}
  ctx.globalAlpha=1;
 }

 function placeBox(layout){
  const {box}=layout.g;view.hidden=false;
  view.style.top=box.top.toFixed(2)+'px';view.style.height=box.height.toFixed(2)+'px';
  const value=String(layout.label());if(view.getAttribute('aria-valuenow')!==value){view.setAttribute('aria-valuenow',value);view.setAttribute('aria-valuetext',value+'%');}
 }

 // ---- Width ----
 // The map shares the width beside the document area; a narrow window has a
 // narrower default (the same media query as file-editor.css).
 // Measured again only when the map or the document area changed size.
 function limits(){
  if(range)return range;
  const area=host?.parentElement?.querySelector(':scope>.nt-editor-document-area');
  range=widthLimits({narrow:!!root.matchMedia?.(NARROW)?.matches,available:(area?.offsetWidth||0)+(host&&!host.hidden?host.offsetWidth:0)});
  return range;
 }
 // While the separator is dragged nothing is laid out again (a page-held
 // 16 MB text area makes every layout of the page slow, and the map, a flex
 // item, cannot be a layout boundary): at the start the map's drawing layer
 // (absolutely placed, right-aligned) takes the largest width over the text;
 // then only its clip and the separator's transform follow the pointer. The
 // map's box, and with it the text area, changes once, on release; meanwhile
 // the map previews its new width, text mode included, at the cost of drawing
 // the map.
 function applyWidth(){
  if(!host||host.hidden)return;
  if(resize){
   const width=paintWidth(),clip='inset(0 0 0 '+(resize.full-width)+'px)',shift=resize.width-width;
   if(inner.style.width!==resize.full+'px')inner.style.width=resize.full+'px';
   if(inner.style.clipPath!==clip)inner.style.clipPath=clip;
   handle.style.transform=shift?'translateX('+shift+'px)':'';
   if(handle.getAttribute('aria-valuenow')!==String(width))handle.setAttribute('aria-valuenow',String(width));
   return;
  }
  const bounds=limits(),value=dragWidth??adapter.width?.()??0,chosen=dragWidth!==null||storedWidth(value)>0;
  const px=chosen?Math.round(dragWidth!==null?clamp(dragWidth,bounds.min,bounds.max):shownWidth(value,bounds))+'px':'';
  if(host.style.width!==px){host.style.width=px;host.style.flexBasis=px;}
  if(handle){
   const now=String(chosen?parseFloat(px):bounds.min);
   for(const [name,v] of [['aria-valuemin',String(bounds.min)],['aria-valuemax',String(bounds.max)],['aria-valuenow',now]])if(handle.getAttribute(name)!==v)handle.setAttribute(name,v);
  }
 }
 // Save a width (0, or the default width, is the default) and show it.
 function commit(value){
  const bounds=limits(),width=value?Math.round(clamp(value,bounds.min,bounds.max)):0;
  adapter.setWidth?.(width&&width>bounds.min?width:0);
  applyWidth();schedule();
 }
 function resizeDown(event){
  if(event.button!==0||!host||host.hidden)return;
  event.preventDefault();
  range=null;
  const bounds=limits(),width=Math.round(host.getBoundingClientRect().width);
  resize={id:event.pointerId,x:event.clientX,width,full:Math.max(width,bounds.max)};dragWidth=null;
  handle.classList.add('is-resizing');host.classList.add('is-resizing');try{handle.setPointerCapture(event.pointerId);}catch{}
  applyWidth();drawn='';schedule();
  adapter.resizing?.(true);
 }
 function resizeMove(event){
  if(resize?.id!==event.pointerId)return;
  const bounds=limits();dragWidth=Math.round(clamp(resize.width+(resize.x-event.clientX),bounds.min,bounds.max));
  applyWidth();schedule();
 }
 function resizeUp(event){if(resize?.id===event.pointerId)finishResize(event.pointerId);}
 function finishResize(id){
  const width=dragWidth;resize=null;dragWidth=null;
  handle?.classList.remove('is-resizing');host?.classList.remove('is-resizing');
  if(inner){inner.style.width='';inner.style.clipPath='';}if(handle)handle.style.transform='';
  if(id!==undefined)try{handle.releasePointerCapture(id);}catch{}
  range=null;drawn='';
  if(width!==null)commit(width);else{applyWidth();schedule();}
  adapter.resizing?.(false);
 }
 function resizeKey(event){
  if(!host||host.hidden)return;
  const bounds=limits(),current=Math.round(host.getBoundingClientRect().width),step=event.shiftKey?BIG_STEP:STEP;
  const next={ArrowLeft:current+step,ArrowRight:current-step,Home:bounds.min,End:bounds.max,Enter:0}[event.key];
  if(next===undefined)return;
  event.preventDefault();event.stopPropagation();
  adapter.resizing?.(true);commit(next);adapter.resizing?.(false);
 }

 // ---- Pointer, wheel and keys ----
 function pointY(event){return event.clientY-host.getBoundingClientRect().top;}
 function seekTo(layout,value){
  pending={layout,value};
  if(layout.kind==='text'){flush();return;}
  if(!seekFrame)seekFrame=root.requestAnimationFrame(flush);
 }
 function flush(){seekFrame=0;const next=pending;pending=null;if(!next)return;try{const result=next.layout.seek(next.value);if(result?.catch)result.catch(error=>adapter.message?.(error.message,true));}catch(error){adapter.message?.(error.message,true);}schedule();}
 function pointerDown(event){
  if(event.button!==0||!visible())return;
  const layout=measure();if(!layout)return;
  event.preventDefault();
  // A press on the box drags it; anywhere else first centres the view on the
  // text there and then drags from there. Focus stays in the text.
  const y=pointY(event),{box}=layout.g,inside=y>=box.top&&y<=box.top+box.height;
  const from=inside?clamp(layout.start,0,Math.max(0,layout.g.total-layout.g.span)):targetAt(layout.g,y,box.height/2);
  drag={id:event.pointerId,y,from};host.classList.add('is-dragging');
  try{host.setPointerCapture(event.pointerId);}catch{}
  if(!inside)seekTo(layout,from);
 }
 function pointerMove(event){
  if(drag?.id!==event.pointerId)return;
  const layout=measure();if(!layout)return;
  seekTo(layout,dragTarget(layout.g,drag.from,pointY(event)-drag.y));
 }
 function pointerUp(event){if(drag?.id!==event.pointerId)return;drag=null;host.classList.remove('is-dragging');try{host.releasePointerCapture(event.pointerId);}catch{}}
 function wheel(event){
  if(event.ctrlKey||!event.deltaY||!visible())return;
  const layout=measure();if(!layout)return;
  event.preventDefault();
  const unit=event.deltaMode===1?layout.step.line:event.deltaMode===2?layout.step.page:1;
  layout.scroll(event.deltaY*unit);schedule();
 }
 function keydown(event){
  const layout=measure();if(!layout)return;
  const step={ArrowUp:-layout.step.line,ArrowDown:layout.step.line,PageUp:-layout.step.page,PageDown:layout.step.page}[event.key];
  if(step!==undefined)layout.scroll(step);
  else if(event.key==='Home')seekTo(layout,0);
  else if(event.key==='End')seekTo(layout,layout.g.total);
  else return;
  event.preventDefault();event.stopPropagation();schedule();
 }

 return Object.freeze({mount,schedule,stats,
  get layout(){return lastLayout;},
  // The rows text mode drew last ({line, row, y, text}); empty for blocks.
  get textRows(){return textRows;},
  get width(){return host&&!host.hidden?host.getBoundingClientRect().width:0;},
  limits:()=>host?limits():null,
  redraw(){drawn='';colors=null;schedule();},
  dispose(){if(frame)root.cancelAnimationFrame(frame);root.clearTimeout(timer);root.clearTimeout(diskTimer);parsing=null;host=null;}});
}

const api={create,commentMarkers,lineRuns,lineShape,lineStarts,geometry,targetAt,dragTarget,widthLimits,storedWidth,shownWidth,textZoom,parseSyntax,syntaxReader,syntaxCursor,rowCells,
 ROW,MIN_BOX,PER_LINE,SAMPLE,WIDTH,NARROW_WIDTH,WIDTH_MAX,TEXT_FONT,TEXT_FONT_MAX,TEXT_REFERENCE,TEXT_COLUMNS,TEXT_ROWS,kinds:{TEXT,COMMENT,STRING,KEYWORD,NUMBER,PROPERTY}};
root.NTKEditorDocumentMap=Object.freeze(api);
if(typeof module==='object'&&module.exports)module.exports=api;
})(globalThis);
