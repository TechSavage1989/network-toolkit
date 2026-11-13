/* Document Map (owner request, like Notepad++'s View > Document Map): a narrow
   panel at the right edge of the Text Editor's document area. It shows the
   whole document scaled down, with a translucent box over the part on screen.
   Clicking moves the view there, dragging the box scrolls, and the wheel over
   the map scrolls the document.

   It never renders a whole large document. The map has three levels of detail,
   chosen by how many text rows each map pixel holds:
     - every row gets at least one pixel: words are drawn as small blocks in the
       document's syntax colours (only possible for short documents);
     - up to PER_LINE lines per pixel: one thin bar per line (indent, length);
     - more: one bar per map pixel row from the line starts the editor already
       keeps (average length), sampling the first line of each row.
   Every level costs at most a few thousand lines or one pass over the map's
   pixel rows. A page-held document is mapped by the text area's own scroll
   geometry, so the box is exactly the visible share. A disk-backed document
   (only one page is in the page) is mapped by position like its scroll bar:
   by line without word wrap, by character offset with word wrap (the
   v1.21.53 scroll model; editor-disk-view.js). Detail is drawn for the loaded
   page only; the rest is a position overview that fills in with the pages the
   reader has seen at the current revision.

   The comparison view has its own change map, so this one is hidden there. */
(function(root){
'use strict';

const ROW=3;              // map pixels per text row while the whole document fits
const MIN_BOX=10;         // the smallest box over the visible part
const PER_LINE=8;         // lines drawn one by one up to this many per map pixel
const SAMPLE=240;         // characters read from one line at most
const LARGE=2*1024*1024;  // a page-held document this long redraws after typing pauses
const PAUSE=250;          // milliseconds after the last edit of a large document
const TEXT=0,COMMENT=1,STRING=2,KEYWORD=3,NUMBER=4;
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
// `total` axis units hold `rows` text rows; the map draws ROW pixels per row
// until the document is taller than the map, then fits it to the map.
function geometry({total,rows,height,start,end}){
 total=Math.max(1,total);
 const used=Math.max(1,Math.min(height,Math.max(1,rows)*ROW)),scale=used/total;
 const span=Math.max(0,Math.min(total,end)-Math.max(0,start));
 const size=Math.min(used,Math.max(MIN_BOX,span*scale));
 const top=clamp(Math.max(0,start)*scale,0,Math.max(0,used-size));
 return {total,used,scale,span,box:{top,height:size}};
}
// The axis position to show at the top of the view for a pointer at map y,
// with `grab` the pointer's distance from the box top.
function targetAt(g,y,grab){return clamp((y-grab)/g.scale,0,Math.max(0,g.total-g.span));}

// ---- The panel ----
function create(adapter){
 let host=null,canvas=null,view=null,frame=0,timer=0,drawn='',colors=null,colorKey='',drag=null,seen=null,widths=new Map(),pageCache=null,wrapCache=null,shown=null,lastLayout=null,pending=null,seekFrame=0;
 const document=root.document;

 function mount(element){
  if(!element||host===element)return;
  host=element;
  host.innerHTML='<canvas class="nt-editor-map-canvas" aria-hidden="true"></canvas><div class="nt-editor-map-view" data-ed-map-view tabindex="0" role="scrollbar" aria-orientation="vertical" aria-label="Visible part of the document" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0"></div>';
  canvas=host.querySelector('canvas');view=host.querySelector('[data-ed-map-view]');
  host.addEventListener('pointerdown',pointerDown);
  host.addEventListener('pointermove',pointerMove);
  host.addEventListener('pointerup',pointerUp);
  host.addEventListener('pointercancel',pointerUp);
  host.addEventListener('lostpointercapture',()=>{drag=null;host?.classList.remove('is-dragging');});
  host.addEventListener('wheel',wheel,{passive:false});
  view.addEventListener('keydown',keydown);
  // Scroll events do not bubble: listen in the capture phase for every text
  // area and scroll bar of the editor (single view, split panes, disk pages).
  (host.closest('.nt-editor')||host.parentElement)?.addEventListener('scroll',schedule,true);
  if(typeof root.ResizeObserver==='function')new root.ResizeObserver(()=>schedule()).observe(host);
  if(typeof root.MutationObserver==='function'&&document?.documentElement)new root.MutationObserver(()=>schedule()).observe(document.documentElement,{attributes:true,attributeFilter:['data-theme']});
 }

 function schedule(){if(!frame&&host)frame=root.requestAnimationFrame(update);}

 function visible(){return !!adapter.enabled()&&!adapter.comparing();}

 function update(){
  frame=0;if(!host)return;
  const show=visible();
  if(shown!==show){shown=show;host.hidden=!show;adapter.layoutChanged?.();}
  if(!show){drag=null;lastLayout=null;return;}
  const layout=measure();lastLayout=layout;
  if(!layout){clear();return;}
  placeBox(layout);
  if(layout.key===drawn)return;
  // A large page-held document is redrawn once typing pauses; the box moves at once.
  if(layout.deferred&&drawn&&drawn.split('|')[0]===layout.key.split('|')[0]){root.clearTimeout(timer);timer=root.setTimeout(()=>{timer=0;drawn='';schedule();},PAUSE);return;}
  draw(layout);drawn=layout.key;
 }

 function clear(){
  if(drawn==='empty')return;drawn='empty';view.hidden=true;
  const ctx=prepare();if(ctx){ctx.fillStyle=palette().bg;ctx.fillRect(0,0,host.clientWidth,host.clientHeight);}
 }

 function prepare(){
  const width=host.clientWidth,height=host.clientHeight,ratio=root.devicePixelRatio||1;
  if(!width||!height)return null;
  const w=Math.round(width*ratio),h=Math.round(height*ratio);
  if(canvas.width!==w)canvas.width=w;if(canvas.height!==h)canvas.height=h;
  canvas.style.width=width+'px';canvas.style.height=height+'px';
  const ctx=canvas.getContext('2d');if(!ctx)return null;
  ctx.setTransform(ratio,0,0,ratio,0,0);return ctx;
 }

 function palette(){
  const theme=document.documentElement?.dataset?.theme||'',overlay=adapter.overlay?.(),key=theme+'|'+(overlay?.dataset?.palette||'')+'|'+(overlay?.getAttribute?.('style')||'');
  if(colors&&key===colorKey)return colors;
  const style=root.getComputedStyle(host),value=(name,fallback)=>style.getPropertyValue(name).trim()||fallback;
  const fg=value('--nt-code-fg',style.color||'#888');
  colors={bg:value('--nt-code-bg','transparent'),fg,band:fg,kinds:[fg,value('--nt-code-comment',fg),value('--nt-code-string',fg),value('--nt-code-keyword',fg),value('--nt-code-number',fg)]};
  colorKey=key;drawn='';return colors;
 }

 function charWidth(font){
  if(widths.has(font))return widths.get(font);
  const ctx=document.createElement('canvas').getContext('2d');let width=8;
  if(ctx){ctx.font=font;width=ctx.measureText('0').width||8;}
  widths.set(font,width);return width;
 }

 function textMetrics(textarea){
  const style=root.getComputedStyle(textarea),number=(value,fallback)=>{const n=parseFloat(value);return Number.isFinite(n)?n:fallback;};
  const lineHeight=number(style.lineHeight,20),font=style.font||style.fontSize+' '+style.fontFamily;
  const left=number(style.paddingLeft,0),right=number(style.paddingRight,0);
  return {lineHeight,top:number(style.paddingTop,0),bottom:number(style.paddingBottom,0),font,columns:Math.max(16,Math.floor(Math.max(1,textarea.clientWidth-left-right)/charWidth(font)))};
 }

 function markersFor(d){const language=adapter.language(d)||'text';return {language,markers:commentMarkers(adapter.commentStyle?.(language),language),keywords:language!=='text'&&!!adapter.syntax()};}

 // The layout of the active document for this frame, or null.
 function measure(){
  const d=adapter.document();if(!d||d.loading||d.staged)return null;
  const textarea=adapter.textarea();if(!textarea||!textarea.isConnected)return null;
  const height=host.clientHeight,width=host.clientWidth;if(!height||!width)return null;
  if(d.disk){
   const paged=adapter.diskView();const state=paged?.mapState?.();
   if(!state||state.aligned||!state.lines)return null;
   if(state.whole)return textLayout(d,textarea,state.page.text,null,state.revision,height,width,paged);
   return state.flow?offsetLayout(d,textarea,state,height,width,paged):lineLayout(d,textarea,state,height,width,paged);
  }
  return textLayout(d,textarea,d.content,adapter.lineStarts(d),d.editRevision,height,width,null);
 }

 function common(d,height,width){
  const c=palette();
  return [height,width,root.devicePixelRatio||1,colorKey,adapter.tabSize(),adapter.language(d),adapter.syntax()].join(':');
 }

 // A page-held document, or a disk-backed one whose page is all of it: the
 // text area's scroll geometry is the axis.
 function textLayout(d,textarea,text,starts,revision,height,width,paged){
  const m=textMetrics(textarea),wrapped=textarea.wrap==='soft';
  if(!starts){if(pageCache?.text!==text)pageCache={text,starts:lineStarts(text)};starts=pageCache.starts;}
  const total=Math.max(1,textarea.scrollHeight),start=textarea.scrollTop,end=start+textarea.clientHeight;
  const g=geometry({total,rows:total/m.lineHeight,height,start,end});
  const large=!d.disk&&text.length>LARGE;
  return {kind:'text',g,deferred:large,
   key:[d.uid+':'+(d.disk?'p':'t'),revision,text.length,total,textarea.clientWidth,m.lineHeight,m.columns,wrapped,common(d,height,width)].join('|'),
   label:()=>Math.round(clamp(start/Math.max(1,total-textarea.clientHeight),0,1)*100),
   draw:ctx=>drawText(ctx,d,text,starts,m,wrapped,g,width,total),
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


 // Rows the reader has seen at this revision and size: bar width (0..1) and
 // whether the row starts with a comment. Unknown rows stay a faint band.
 function seenRows(key,used){
  if(seen?.key!==key)seen={key,widths:new Float32Array(Math.ceil(used)).fill(-1),comments:new Uint8Array(Math.ceil(used))};
  return seen;
 }
 function drawSeen(ctx,c,record,g,width){
  ctx.fillStyle=c.band;ctx.globalAlpha=0.08;ctx.fillRect(0,0,width,g.used);
  ctx.globalAlpha=0.6;
  for(let y=0;y<record.widths.length;y++){const w=record.widths[y];if(w>0){ctx.fillStyle=record.comments[y]?c.kinds[COMMENT]:c.fg;ctx.fillRect(0,y,Math.max(0.6,w*width),1);}}
  ctx.globalAlpha=1;
 }

 // A disk-backed document without word wrap: the axis is the line number.
 function lineLayout(d,textarea,state,height,width,paged){
  const m=textMetrics(textarea),columns=Math.max(16,Math.floor(textarea.clientWidth/Math.max(1,state.characterWidth||charWidth(m.font))));
  const start=state.top+(state.fraction||0),g=geometry({total:state.lines,rows:state.lines,height,start,end:start+state.rows});
  const page=state.page;if(pageCache?.text!==page.text)pageCache={text:page.text,starts:lineStarts(page.text)};
  const base=[d.uid+':l',state.revision,state.lines,common(d,height,width),columns].join('|');
  return {kind:'lines',g,deferred:false,key:[base,page.start,page.text.length].join('|'),
   label:()=>Math.round(clamp(state.top/Math.max(1,state.lines-state.rows),0,1)*100),
   draw:ctx=>{
    const c=palette(),record=seenRows(base,g.used),{markers,keywords}=markersFor(d),tab=adapter.tabSize(),starts=pageCache.starts,text=page.text,n=starts.length,cw=width/columns;
    const end=i=>i+1<n?starts[i+1]-1:text.length,rowHeight=g.scale;
    // Record what this page shows, then draw the known rows and the page.
    for(let i=0;i<n;i++){
     const y0=Math.floor((page.line+i)*g.scale),y1=Math.min(record.widths.length,Math.max(y0+1,Math.floor((page.line+i+1)*g.scale)));
     const shape=lineShape(text,starts[i],end(i),{tab,markers}),w=shape.blank?0:Math.min(1,Math.max(0,shape.length)/columns);
     for(let y=y0;y<y1;y++)if(record.widths[y]<w||record.widths[y]<0){record.widths[y]=w;record.comments[y]=shape.comment?1:0;}
    }
    ctx.fillStyle=c.bg;ctx.fillRect(0,0,width,g.used);drawSeen(ctx,c,record,g,width);
    // The loaded page in detail while it has at most PER_LINE lines per pixel.
    if(rowHeight*PER_LINE>=1){
     const top=page.line*g.scale,detail=rowHeight>=1,block=detail?Math.max(1,Math.min(rowHeight-0.5,rowHeight*0.7)):Math.max(0.35,rowHeight),alpha=detail?1:0.55;
     ctx.globalAlpha=1;ctx.fillStyle=c.bg;ctx.fillRect(0,top,width,Math.max(1,n*rowHeight));
     for(let i=0;i<n;i++){const y=(page.line+i)*g.scale;if(y>g.used)break;for(const [col,length,kind] of lineRuns(text,starts[i],end(i),{tab,limit:columns,markers,keywords})){ctx.fillStyle=c.kinds[kind];ctx.globalAlpha=alpha*(kind===TEXT?0.75:1);ctx.fillRect(col*cw,y,Math.max(0.6,Math.min(length,columns-col)*cw),block);}}
     ctx.globalAlpha=1;
    }
   },
   seek:value=>paged.mapSeek({line:value}),
   scroll:delta=>paged.wheelBy(delta),
   step:{line:1,page:Math.max(1,state.rows-1)}};
 }

 // A disk-backed document with word wrap: the axis is the character offset,
 // and the map is as tall as the document's estimated wrapped rows.
 function offsetLayout(d,textarea,state,height,width,paged){
  const start=state.offset,g=geometry({total:state.units,rows:state.estimatedRows,height,start,end:start+state.visible});
  const page=state.page,base=[d.uid+':o',state.revision,state.units,common(d,height,width),state.wrapColumns].join('|');
  return {kind:'offsets',g,deferred:false,key:[base,page.start,page.text.length].join('|'),
   label:()=>Math.round(clamp(start/Math.max(1,state.units-state.visible),0,1)*100),
   draw:ctx=>{
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
    ctx.fillStyle=c.bg;ctx.fillRect(0,0,width,g.used);drawSeen(ctx,c,record,g,width);
   },
   seek:value=>paged.mapSeek({offset:value}),
   scroll:delta=>paged.wheelBy(delta),
   step:{line:state.rowHeight||20,page:Math.max(20,textarea.clientHeight-(state.rowHeight||20))}};
 }

 function draw(layout){
  const ctx=prepare();if(!ctx)return;
  const c=palette();ctx.globalAlpha=1;ctx.fillStyle=c.bg;ctx.fillRect(0,0,host.clientWidth,host.clientHeight);
  try{layout.draw(ctx);}catch(error){root.console?.error?.('The document map could not be drawn.',error);}
  ctx.globalAlpha=1;
 }

 function placeBox(layout){
  const {box}=layout.g;view.hidden=false;
  view.style.top=box.top.toFixed(2)+'px';view.style.height=box.height.toFixed(2)+'px';
  const value=String(layout.label());if(view.getAttribute('aria-valuenow')!==value){view.setAttribute('aria-valuenow',value);view.setAttribute('aria-valuetext',value+'%');}
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
  // A press on the box drags it; anywhere else first centres the view there
  // and then drags from the middle of the box. Focus stays in the text.
  const y=pointY(event),{box}=layout.g,inside=y>=box.top&&y<=box.top+box.height;
  const grab=inside?y-box.top:box.height/2;
  drag={id:event.pointerId,grab};host.classList.add('is-dragging');
  try{host.setPointerCapture(event.pointerId);}catch{}
  if(!inside)seekTo(layout,targetAt(layout.g,y,grab));
 }
 function pointerMove(event){
  if(drag?.id!==event.pointerId)return;
  const layout=measure();if(!layout)return;
  seekTo(layout,targetAt(layout.g,pointY(event),drag.grab));
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

 return Object.freeze({mount,schedule,get layout(){return lastLayout;},redraw(){drawn='';colors=null;schedule();},dispose(){if(frame)root.cancelAnimationFrame(frame);root.clearTimeout(timer);host=null;}});
}

const api={create,commentMarkers,lineRuns,lineShape,lineStarts,geometry,targetAt,ROW,MIN_BOX,PER_LINE,SAMPLE,kinds:{TEXT,COMMENT,STRING,KEYWORD,NUMBER}};
root.NTKEditorDocumentMap=Object.freeze(api);
if(typeof module==='object'&&module.exports)module.exports=api;
})(globalThis);
