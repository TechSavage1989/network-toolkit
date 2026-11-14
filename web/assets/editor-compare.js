/* Local-only, bounded line comparison. It never reads files or sends document text. */
(function(root){'use strict';
// Chromium 151 (fixed by 153): once a click on a button has taken the focus
// from a textarea, the page's selection stays at that textarea, and each later
// click on a button puts back the selection the textarea had then, undoing one
// set by script since. Dropping that stale page selection keeps the one set
// here (the same function as in file-editor.js).
function keepScriptSelection(ta){
 const doc=ta?.ownerDocument;if(!doc||doc.activeElement===ta)return;
 const s=doc.getSelection?.();if(!s?.rangeCount)return;
 const at=(node,offset)=>node===ta||node===ta.parentNode&&(node.childNodes[offset]===ta||node.childNodes[offset-1]===ta);
 if(at(s.anchorNode,s.anchorOffset)||at(s.focusNode,s.focusOffset))s.removeAllRanges();
}
const LIMITS=Object.freeze({characters:16*1024*1024,lines:30000,cells:2000000});
const splitPreferences=new WeakMap();
const clamp=(v,a,b)=>Math.max(a,Math.min(b,Number(v)||0));
const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
// Find unique exact lines, or unique three-line contexts when every matching
// line repeats. Keep their order with a longest increasing sequence.
function uniqueAnchors(a,b,fromA,toA,fromB,toB){
 const leftPositions=new Map(),rightPositions=new Map();
 const remember=(positions,key,index)=>positions.set(key,positions.has(key)?-1:index);
 const candidates=[];
 const collect=()=>{
  for(const [key,left] of leftPositions){
   const right=rightPositions.get(key);
   if(left>=0&&right>=0)candidates.push({left,right});
  }
 };
 for(let i=fromA;i<toA;i++)remember(leftPositions,a[i],i);
 for(let j=fromB;j<toB;j++)remember(rightPositions,b[j],j);
 collect();
 if(!candidates.length&&toA-fromA>=3&&toB-fromB>=3){
  leftPositions.clear();
  rightPositions.clear();
  // Intern lines so a context key does not copy long line text.
  const ids=new Map();
  const id=line=>{
   if(!ids.has(line))ids.set(line,ids.size);
   return ids.get(line);
  };
  const indexContexts=(rows,from,to,positions)=>{
   let first=id(rows[from]),second=id(rows[from+1]);
   for(let i=from;i+2<to;i++){
    const third=id(rows[i+2]);
    remember(positions,first+','+second+','+third,i);
    first=second;
    second=third;
   }
  };
  indexContexts(a,fromA,toA,leftPositions);
  indexContexts(b,fromB,toB,rightPositions);
  collect();
 }
 const tails=[],previous=[],atTail=[];
 for(let i=0;i<candidates.length;i++){const v=candidates[i].right;let lo=0,hi=tails.length;while(lo<hi){const mid=(lo+hi)>>1;if(tails[mid]<v)lo=mid+1;else hi=mid;}previous[i]=lo?atTail[lo-1]:-1;tails[lo]=v;atTail[lo]=i;}
 const ordered=[];for(let i=atTail.at(-1);i!==undefined&&i>=0;i=previous[i])ordered.push(candidates[i]);return ordered.reverse();
}
function appendRegion(a,b,fromA,toA,fromB,toB,ops,budget){
 const n=toA-fromA,m=toB-fromB;
 if(n&&m){const cells=n*m;if(cells>budget.remaining)return false;budget.remaining-=cells;const width=m+1,table=new Uint32Array((n+1)*width);
  for(let i=n-1;i>=0;i--)for(let j=m-1;j>=0;j--)table[i*width+j]=a[fromA+i]===b[fromB+j]?1+table[(i+1)*width+j+1]:Math.max(table[(i+1)*width+j],table[i*width+j+1]);
  let i=0,j=0;while(i<n||j<m){if(i<n&&j<m&&a[fromA+i]===b[fromB+j])ops.push({type:'equal',left:fromA+i++,right:fromB+j++});else if(i<n&&(j===m||table[(i+1)*width+j]>=table[i*width+j+1]))ops.push({type:'removed',left:fromA+i++,right:null});else ops.push({type:'added',left:null,right:fromB+j++});}
 }else{for(let i=fromA;i<toA;i++)ops.push({type:'removed',left:i,right:null});for(let j=fromB;j<toB;j++)ops.push({type:'added',left:null,right:j});}
 return true;
}
// Match a unique adjacent pair among unmatched operations, then extend through
// identical neighboring lines. Detect moves before pairing changed lines, so
// an edit next to a moved block does not consume one of its source lines.
function markMovedOperations(ops, lines){
 function index(side, kind){
  const pairs=new Map();
  for(let i=0;i+1<ops.length;i++){
   const a=ops[i],b=ops[i+1];
   if(a.type!==kind||b.type!==kind||b[side]!==a[side]+1)continue;
   const first=lines[side][a[side]],second=lines[side][b[side]];
   if(!first.trim()&&!second.trim())continue;
   const key=first+'\n'+second;
   pairs.set(key,pairs.has(key)?-1:i);
  }
  return pairs;
 }
 const left=index('left','removed'),right=index('right','added');
 const matches=(i,j)=>ops[i]?.type==='removed'&&ops[j]?.type==='added'&&lines.left[ops[i].left]===lines.right[ops[j].right];
 for(const [key,leftAt] of left){
  const rightAt=right.get(key);
  if(leftAt<0||!(rightAt>=0)||!matches(leftAt,rightAt)||!matches(leftAt+1,rightAt+1))continue;
  let i=leftAt,j=rightAt;
  while(matches(i-1,j-1)){i--;j--;}
  for(;matches(i,j);i++,j++){
   const from=ops[i],to=ops[j];
   from.type=to.type='moved';
   from.counterpart=to.right;to.counterpart=from.left;
  }
 }
}
// A line much shorter than its counterpart reads as a replacement rather
// than a character edit. Keep their row paired so later lines stay aligned.
function isLowSimilarityReplacement(left,right){
 if(typeof left!=='string'||typeof right!=='string')return false;
 const a=left.trim().length,b=right.trim().length,larger=Math.max(a,b);
 return larger>=20&&Math.min(a,b)*10<larger*3;
}
function compare(left,right,spaces){
 left=String(left);right=String(right);const limitMessage={limited:true,message:'Comparison supports up to 16 MiB and 30,000 lines per document. Both documents remain editable; compare smaller sections for larger files.'};
 if(left.length>LIMITS.characters||right.length>LIMITS.characters)return limitMessage;
 const tooManyLines=text=>{let count=1,at=-1;while((at=text.indexOf('\n',at+1))!==-1)if(++count>LIMITS.lines)return true;return false;};
 if(tooManyLines(left)||tooManyLines(right))return limitMessage;
 const normalize=spaces==='all'?line=>line.replace(/[ \t]/g,''):spaces==='changed'?line=>line.replace(/[ \t]+/g,' ').replace(/^ | $/g,''):null;
 const lines={left:left.split('\n'),right:right.split('\n')};
 const a=normalize?lines.left.map(normalize):lines.left,b=normalize?lines.right.map(normalize):lines.right;
 let prefix=0,suffix=0;while(prefix<a.length&&prefix<b.length&&a[prefix]===b[prefix])prefix++;
 while(suffix<a.length-prefix&&suffix<b.length-prefix&&a[a.length-1-suffix]===b[b.length-1-suffix])suffix++;
 const n=a.length-prefix-suffix,m=b.length-prefix-suffix,budget={remaining:LIMITS.cells};
 const limited={limited:true,message:'This changed region exceeds the bounded comparison budget. Compare smaller sections; both original files remain intact.'};
 const ops=[];for(let i=0;i<prefix;i++)ops.push({type:'equal',left:i,right:i});
 const startA=prefix,endA=a.length-suffix,startB=prefix,endB=b.length-suffix;
 const anchors=n*m>LIMITS.cells?uniqueAnchors(a,b,startA,endA,startB,endB):[];
 if(n*m>LIMITS.cells&&!anchors.length)return limited;
 let fromA=startA,fromB=startB;
 for(const item of anchors){if(!appendRegion(a,b,fromA,item.left,fromB,item.right,ops,budget))return limited;ops.push({type:'equal',left:item.left,right:item.right});fromA=item.left+1;fromB=item.right+1;}
 if(!appendRegion(a,b,fromA,endA,fromB,endB,ops,budget))return limited;
 for(let i=0;i<suffix;i++)ops.push({type:'equal',left:a.length-suffix+i,right:b.length-suffix+i});
 markMovedOperations(ops,lines);
 const rows=[],hunks=[],kinds={left:new Array(a.length).fill('equal'),right:new Array(b.length).fill('equal')};
 for(let i=0;i<ops.length;){if(ops[i].type==='equal'){rows.push(ops[i++]);continue;}const removed=[],added=[];while(i<ops.length&&ops[i].type!=='equal'){const op=ops[i++];(op.left!==null?removed:added).push(op);}const start=rows.length,type=removed.length&&added.length?'changed':removed.length?'removed':'added';
  for(let i=0,j=0;i<removed.length||j<added.length;){
   const before=removed[i],after=added[j];
   const move=before?.type==='moved'?before:after?.type==='moved'?after:null;
   const l=move?move.left:before?.left??null,r=move?move.right:after?.right??null;
   const rowType=move?'moved':l!==null&&r!==null?'changed':l!==null?'removed':'added';
   const replacement=rowType==='changed'&&isLowSimilarityReplacement(lines.left[l],lines.right[r]);
   rows.push(move||{type:replacement?'replacement':rowType,left:l,right:r});
   if(l!==null){kinds.left[l]=replacement?'removed':rowType;i++;}
   if(r!==null){kinds.right[r]=replacement?'added':rowType;j++;}
  }
  hunks.push({start,end:rows.length,type,left:removed[0]?.left??null,right:added[0]?.right??null});
 }
 const positions={left:[0],right:[0]};for(const row of rows)for(const side of ['left','right'])positions[side].push(positions[side].at(-1)+(row[side]===null?0:1));
 return {limited:false,rows,hunks,kinds,positions,lineCounts:{left:a.length,right:b.length},identical:hunks.length===0};
}
function lineAtRow(positions,row){if(!positions?.length)return 0;row=clamp(row,0,positions.length-1);const i=Math.floor(row),j=Math.min(i+1,positions.length-1);return positions[i]+(positions[j]-positions[i])*(row-i);}
function rowAtLine(positions,line){if(!positions?.length)return 0;line=clamp(line,0,positions.at(-1));let low=0,high=positions.length-1;while(low<high){const mid=Math.floor((low+high)/2);if(positions[mid]<line)low=mid+1;else high=mid;}if(low===0||positions[low]===line)return low;const prev=low-1;return prev+(line-positions[prev])/(positions[low]-positions[prev]);}
function viewport(positions,scrollTop,clientHeight,scrollHeight,lineHeight,padding=10){const max=Math.max(0,scrollHeight-clientHeight),top=clamp(scrollTop,0,max),first=clamp((top-padding)/Math.max(1,lineHeight),0,positions.at(-1)),last=clamp((top+clientHeight-padding)/Math.max(1,lineHeight),0,positions.at(-1));return {start:rowAtLine(positions,first),end:rowAtLine(positions,last)};}
function offsetAtLine(offsets,line){return lineAtRow(offsets,line);}
function lineAtOffset(offsets,pixel){return rowAtLine(offsets,pixel);}
function mapHeight(rows,height,scale){return Math.max(1,height)*(1+Math.max(0,scale));}
function rowFromPointer(clientY,top,scrollTop,height,rows){return clamp((clientY-top+scrollTop)/Math.max(1,height)*rows,0,rows);}
// Both editing modes share one bounded separator; no document/settings payload changes.
function splitBounds(width){width=Math.max(0,Number(width)||0);const minimum=Math.min(160,Math.max(80,width*.2),width/2);return width?{min:minimum/width,max:1-minimum/width}:{min:.5,max:.5};}
function splitRatio(value,width){const bounds=splitBounds(width);value=Number(value);return clamp(Number.isFinite(value)?value:.5,bounds.min,bounds.max);}
function createDivider(host,grid,comparing,onResize){
 const q=selector=>host.querySelector(selector),divider=q('[data-compare-divider]'),splitOwner=host.closest?.('.nt-editor')||host;let preferredSplit=splitPreferences.get(splitOwner)??.5,splitDrag=null;
 divider.className='nt-editor-pane-resize';divider.dataset.compareDivider='';divider.tabIndex=0;
 for(const [key,value] of Object.entries({role:'separator','aria-label':'Resize document panes','aria-orientation':'vertical','aria-controls':'ntCompare-left ntCompare-right',title:'Drag to resize document panes. Arrow keys resize; Home / End use limits; Enter or double-click resets.'}))divider.setAttribute(key,value);
 function splitWidth(){return Math.max(0,grid.clientWidth-divider.getBoundingClientRect().width-(comparing?q('.nt-compare-map-panel').getBoundingClientRect().width:0));}
 function applySplit(){const width=splitWidth(),ratio=splitRatio(preferredSplit,width),bounds=splitBounds(width),left=ratio+'fr',right=(1-ratio)+'fr';
  if(grid.style.getPropertyValue('--nt-editor-left-pane')!==left){grid.style.setProperty('--nt-editor-left-pane',left);grid.style.setProperty('--nt-editor-right-pane',right);}
  divider.setAttribute('aria-valuemin',String(Math.round(bounds.min*100)));divider.setAttribute('aria-valuemax',String(Math.round(bounds.max*100)));divider.setAttribute('aria-valuenow',String(Math.round(ratio*100)));divider.setAttribute('aria-valuetext','Left pane '+Math.round(ratio*100)+' percent; right pane '+Math.round((1-ratio)*100)+' percent');return ratio;
 }
 function resizeSplit(value){preferredSplit=splitRatio(value,splitWidth());splitPreferences.set(splitOwner,preferredSplit);applySplit();onResize?.();}
 function stopSplit(event,cancel=false){if(!splitDrag||event.pointerId!==undefined&&splitDrag.id!==event.pointerId)return;const previous=splitDrag;splitDrag=null;divider.classList.remove('is-resizing');if(cancel)resizeSplit(previous.ratio);try{if(divider.hasPointerCapture?.(previous.id))divider.releasePointerCapture(previous.id);}catch{}}
 divider.addEventListener('pointerdown',event=>{if(event.button!==0||splitDrag)return;event.preventDefault();event.stopPropagation();splitDrag={id:event.pointerId,offset:event.clientX-divider.getBoundingClientRect().left,ratio:preferredSplit};divider.classList.add('is-resizing');divider.focus({preventScroll:true});divider.setPointerCapture?.(event.pointerId);});
 divider.addEventListener('pointermove',event=>{if(splitDrag?.id!==event.pointerId)return;event.preventDefault();event.stopPropagation();const width=splitWidth();if(width)resizeSplit((event.clientX-grid.getBoundingClientRect().left-splitDrag.offset)/width);});
 divider.addEventListener('pointerup',event=>stopSplit(event));divider.addEventListener('pointercancel',event=>stopSplit(event,true));divider.addEventListener('lostpointercapture',event=>stopSplit(event));
 divider.addEventListener('dblclick',event=>{event.preventDefault();event.stopPropagation();resizeSplit(.5);});
 divider.addEventListener('keydown',event=>{if(event.key==='Escape'&&splitDrag){event.preventDefault();event.stopPropagation();stopSplit(event,true);return;}if(!['ArrowLeft','ArrowRight','Home','End','Enter'].includes(event.key))return;event.preventDefault();event.stopPropagation();const width=splitWidth(),ratio=applySplit(),bounds=splitBounds(width);resizeSplit(event.key==='Home'?bounds.min:event.key==='End'?bounds.max:event.key==='Enter'?.5:ratio+(event.key==='ArrowLeft'?-20:20)/Math.max(1,width));});


 return {apply:applySplit,dispose:()=>stopSplit({},false)};
}
let settingsSequence=0;
const differenceControls='[data-compare="previous"],[data-compare="next"],[data-compare="first"],[data-compare="last"]';
function differenceIndex(current,step,count,wrap=true){
 if(step==='first')return 0;
 if(step==='last')return count-1;
 if(current<0)return step>0?0:count-1;
 return wrap?(current+step+count)%count:clamp(current+step,0,count-1);
}
function mountExtraSettings(host,panel,options){
 panel.insertAdjacentHTML('beforeend','<label>Whitespace <select data-compare-spaces><option value="">Exact</option><option value="changed">Ignore changed spaces</option><option value="all">Ignore all spaces</option></select></label>'+
  '<label><input type="checkbox" data-compare-only> Show only differences</label>'+
  '<label><input type="checkbox" data-compare-show-map checked> Show change map</label>'+
  '<label><input type="checkbox" data-compare-wrap-nav checked> Wrap around differences</label>'+
  '<label><input type="checkbox" data-compare-move-caret checked> Select changed text on navigation</label>'+
  '<label><input type="checkbox" data-compare-show-summary checked> Show comparison summary</label>'+
  '<button type="button" class="btn small ghost" data-compare="first" disabled>First difference</button>'+
  '<button type="button" class="btn small ghost" data-compare="last" disabled>Last difference</button>');
 const prefs=options?.comparisonPreferences?.()||{};
 for(const [attribute,key] of [['wrap-nav','compareWrapNavigation'],['move-caret','compareMoveCaret'],['show-summary','compareShowSummary']]){
  const control=panel.querySelector('[data-compare-'+attribute+']');
  const update=()=>{
   if(attribute==='show-summary')host.classList.toggle('is-summary-hidden',!control.checked);
  };
  control.checked=prefs[key]!==false;
  if(attribute==='move-caret'&&options?.compareCanSelect===false)control.closest('label').hidden=true;
  update();
  control.addEventListener('change',()=>{update();options?.setComparisonPreference?.(key,control.checked);});
 }
 panel.querySelector('[data-compare-show-map]').addEventListener('change',event=>{
  const hide=!event.target.checked;
  host.classList.toggle('is-map-hidden',hide);
  host.querySelector('.nt-compare-map-panel').hidden=hide;
 });
 host.addEventListener('keydown', event => {
  if (!event.altKey || event.shiftKey || event.isComposing || !['PageUp','PageDown'].includes(event.key)) return;
  const earlier = event.key === 'PageUp';
  const action = event.ctrlKey ? (earlier ? 'first' : 'last') : (earlier ? 'previous' : 'next');
  const button = host.querySelector('[data-compare="' + action + '"]');
  if (!button || button.disabled) return;
  event.preventDefault();
  event.stopPropagation();
  button.click();
 }, true);
}

// One continuous scroll range with a bounded window of source rows.
function mountDifferencePreview(host, read, getCount) {
 const toggle = host.querySelector('[data-compare-only]');
 const grid = host.querySelector('.nt-compare-grid');
 const panel = document.createElement('section');
 panel.className = 'nt-compare-preview';
 panel.hidden = true;
 panel.innerHTML = '<div class="nt-compare-preview-tools"><span role="status"></span>' +
  '<button type="button" class="btn small ghost" data-preview-edit>Return to editable comparison</button></div>' +
  '<div class="nt-compare-preview-scroll" tabindex="0" aria-label="Changed lines"><div class="nt-compare-preview-spacer">' +
  '<div class="nt-compare-preview-window"><div class="nt-compare-preview-rows"></div></div></div></div>';
 const covered = [...grid.children];
 grid.append(panel);
 const status = panel.querySelector('[role="status"]'), scroll = panel.querySelector('.nt-compare-preview-scroll'), spacer = scroll.firstElementChild;
 const viewport = spacer.firstElementChild, rows = viewport.firstElementChild;
 let logicalTop = 0, expectedTop = 0, controller, cache, requested, frame = 0;
 let rowHeight = 25;

 function extent() {
  return Math.max(0, (getCount() || 0) * rowHeight - scroll.clientHeight);
 }
 function move(top) {
  logicalTop = clamp(top, 0, extent());
  const range = scroll.scrollHeight - scroll.clientHeight;
  scroll.scrollTop = extent() ? logicalTop / extent() * range : 0;
  expectedTop = scroll.scrollTop;
  queue();
 }
 function queue() {
  if (!panel.hidden && !frame) frame = root.requestAnimationFrame(() => {frame = 0; void paint();});
 }
 function paintRows(data, first) {
  const fragment = document.createDocumentFragment();
  for (const pair of data) {
   for (let side = 0; side < 2; side++) {
    const value = pair[side], cell = document.createElement('div');
    const kind = value ? value.kind || (value.moved ? 'moved' : pair[1-side] ? 'changed' : side === 0 ? 'removed' : 'added') : 'gap';
    cell.className = 'nt-compare-preview-line ' + kind;
    if (value) {
     const symbol = document.createElement('b');
     symbol.textContent = {changed:'~',removed:'−',added:'+',moved:'↔'}[kind];
     symbol.setAttribute('aria-label', {changed:'Modified',removed:'Removed',added:'Added',moved:'Moved'}[kind]);
     if(Number.isInteger(value.counterpart))symbol.title='Other document line '+(value.counterpart+1);
     const label = document.createElement('span');
     label.textContent = String(value.line + 1);
     const text = document.createElement('code');
     text.textContent = value.text;
     cell.append(symbol, label, text);
    }
    fragment.append(cell);
   }
  }
  rows.replaceChildren(fragment);
  rows.dataset.first = String(first);
 }
 async function paint() {
  if (panel.hidden) return;
  const count = getCount();
  const oldHeight = rowHeight;
  rowHeight = Math.max(25, Math.ceil((parseFloat(getComputedStyle(rows).getPropertyValue('--nt-editor-font')) || 13) * 1.55 + 4));
  rows.style.setProperty('--nt-preview-row-height', rowHeight + 'px');
  spacer.style.height = Math.min(1000000, (count || 0) * rowHeight) + 'px';
  viewport.style.height = Math.min(scroll.clientHeight, (count || 0) * rowHeight) + 'px';
  if (oldHeight !== rowHeight) move(logicalTop / oldHeight * rowHeight);
  const first = Math.floor(logicalTop / rowHeight), last = Math.min(count || 0, Math.ceil((logicalTop + scroll.clientHeight) / rowHeight));
  rows.style.transform = 'translateY(' + (Number(rows.dataset.first || 0) * rowHeight - logicalTop) + 'px)';
  if (!count || last <= first) return;
  if (cache && first >= cache.start && last <= cache.end) return;
  if (requested && first >= requested.start && last <= requested.end) return;
  const start = Math.max(0, first - 24), end = Math.min(count, last + 40);
  controller?.abort();
  const pending = new AbortController();
  controller = pending;
  requested = {start,end};
  try {
   const data = await read(start, end - start, pending.signal);
   if (pending.signal.aborted || !data) return;
   cache = {start, end:start+data.length};
   paintRows(data, start);
   rows.style.transform = 'translateY(' + (start * rowHeight - logicalTop) + 'px)';
  } catch (error) {
   if (!pending.signal.aborted) status.textContent = error.message;
  } finally {
   if (controller === pending) requested = null;
  }
 }
 function refresh(reset = false) {
  controller?.abort();
  cache = requested = null;
  panel.hidden = !toggle.checked;
  for (const child of covered) child.inert = !panel.hidden;
  if (panel.hidden) return;
  if (reset) { logicalTop = 0; scroll.scrollTop = expectedTop = 0; }
  const count = getCount();
  status.textContent = count == null ? 'Choose two documents to compare.' :
   count ? count.toLocaleString() + ' changed rows' : 'No differences.';
  rows.replaceChildren();
  queue();
 }
 scroll.addEventListener('scroll', () => {
  if (scroll.scrollTop !== expectedTop) {
   const range = scroll.scrollHeight - scroll.clientHeight;
   logicalTop = range ? scroll.scrollTop / range * extent() : 0;
   expectedTop = scroll.scrollTop;
  }
  queue();
 });
 scroll.addEventListener('wheel', event => {
  if (!event.deltaY || event.ctrlKey || extent() <= scroll.scrollHeight - scroll.clientHeight) return;
  event.preventDefault();
  move(logicalTop + event.deltaY * (event.deltaMode === 1 ? rowHeight : event.deltaMode === 2 ? scroll.clientHeight : 1));
 }, {passive:false});
 scroll.addEventListener('keydown', event => {
  if (event.target !== scroll || event.altKey || event.ctrlKey || event.metaKey) return;
  const targets = {Home:0,End:extent(),PageUp:logicalTop-scroll.clientHeight,PageDown:logicalTop+scroll.clientHeight,
   ArrowUp:logicalTop-rowHeight,ArrowDown:logicalTop+rowHeight};
  if (!Object.hasOwn(targets,event.key)) return;
  event.preventDefault();
  move(targets[event.key]);
 });
 const resize = typeof ResizeObserver === 'function' ? new ResizeObserver(queue) : null;
 resize?.observe(scroll);
 toggle.addEventListener('change', () => {
  refresh(true);
  if (!panel.hidden) {
   host.querySelector('.nt-compare-settings').open = false;
   scroll.focus({preventScroll:true});
  }
 });
 panel.querySelector('[data-preview-edit]').addEventListener('click', () => {
  toggle.checked = false;
  refresh(true);
  host.querySelector('.nt-compare-settings summary').focus();
 });
 return {refresh, reflow:queue, reveal(row){if (!panel.hidden && row != null) move(row*rowHeight);},
  dispose(){controller?.abort();resize?.disconnect();if(frame)root.cancelAnimationFrame(frame);}};
}

const inlinePreviewCache = new WeakMap();
function inlinePreviewRows(result, getDocument) {
 if (!result || result.limited) return null;
 if (!inlinePreviewCache.has(result)) {
  const changed = result.rows.filter(row => row.type !== 'equal');
  const lines={left:getDocument('left').content.split('\n'),right:getDocument('right').content.split('\n')};
  inlinePreviewCache.set(result, changed.map(row => ['left','right'].map(side => {
   const line=row[side];
   return line==null?null:{line,text:lines[side][line],kind:result.kinds[side][line],counterpart:row.counterpart};
  })));
 }
 return inlinePreviewCache.get(result);
}
function inlineChangedStart(result, index) {
 let start = 0;
 for (let i = 0; i < index; i++) start += result.hunks[i].end - result.hunks[i].start;
 return start;
}

function previewSourceLines(text, line, count, first = 0) {
 const result = [];
 for (let start = 0; first < line + count && start <= text.length; first++) {
  const end = text.indexOf('\n', start);
  if (first >= line) result.push({line:first, text:text.slice(start, end < 0 ? text.length : end)});
  if (end < 0) break;
  start = end + 1;
 }
 return result;
}
function inlineDifferencePage(result, getDocument, index, offset, limit) {
 const hunk = result.hunks[index], length = hunk.end - hunk.start;
 const rows = result.rows.slice(hunk.start + offset, hunk.start + Math.min(offset + limit, length));
 const read = side => {
  const first = rows.find(row => row[side] !== null)?.[side];
  if (first === undefined) return [];
  const last = rows.findLast(row => row[side] !== null)[side];
  const lines = previewSourceLines(getDocument(side).content, first, last - first + 1);
  return rows.map(row => row[side] === null ? null : lines[row[side] - first]);
 };
 return {length,left:read('left'),right:read('right')};
}
function placeToolbarDropdown(trigger,panel){
 const toolbar=trigger.closest('.nt-editor-toolbar,.nt-compare-tools');
 if(!toolbar||panel.hidden)return;
 const anchor=trigger.getBoundingClientRect(), bounds=toolbar.getBoundingClientRect();
 panel.style.left=Math.max(8,Math.min(anchor.left-bounds.left,bounds.width-panel.offsetWidth-8))+'px';
 panel.style.top=(anchor.bottom-bounds.top+2)+'px';
}
function mountSettings(host,options){
 const disclosure=host.querySelector('.nt-compare-settings'),panel=disclosure?.querySelector('div'),summary=disclosure?.querySelector('summary');
 if(!panel||!summary)return;
 mountExtraSettings(host,panel,options);panel.className='nt-compare-settings-actions';panel.id='ntCompareSettings'+(++settingsSequence);
 panel.hidden=!disclosure.open;summary.setAttribute('aria-controls',panel.id);summary.setAttribute('aria-expanded',String(disclosure.open));
 disclosure.addEventListener('toggle',()=>{
  if(!disclosure.open&&panel.contains(document.activeElement))summary.focus({preventScroll:true});
  panel.hidden=!disclosure.open;summary.setAttribute('aria-expanded',String(disclosure.open));
  placeToolbarDropdown(summary,panel);
 });
 disclosure.addEventListener('keydown',event=>{
  if(event.key!=='Escape'||!disclosure.open)return;
  event.stopPropagation();summary.focus({preventScroll:true});disclosure.open=false;
 });
}
// Source-owned horizontal pixels, independent of aligned vertical geometry.
// Peer clamping must never pull a longer source backward or become a new gesture.
function createHorizontalSync(read,write,enabled){let source=null;const previous={left:0,right:0},expected={left:null,right:null};
 function sync(side=source){if(!side||!enabled())return false;source=side;const own=read(side),other=side==='left'?'right':'left',peer=read(other),target=clamp(own.left,0,Math.max(0,peer.width-peer.client));previous[side]=own.left;if(Math.abs(peer.left-target)<.5)return false;expected[other]=target;previous[other]=target;write(other,target);return true;}
 return {input(side){source=side;expected[side]=null;},scroll(side){const value=read(side).left,old=previous[side];previous[side]=value;if(expected[side]!==null&&Math.abs(value-expected[side])<.5){expected[side]=null;return false;}expected[side]=null;if(Math.abs(value-old)<.5||source!==side)return false;return sync(side);},sync,reset(){source=null;for(const side of ['left','right']){previous[side]=read(side).left;expected[side]=null;}}};
}
function create(options){
 if(options.comparing!==false&&root.NTKEditorAlignedCompare&&root.NetworkToolkitCodeMirror&&root.NTKEditorAlignmentModel)return root.NTKEditorAlignedCompare.create(options);
 const host=options.container,comparing=options.comparing!==false;let result=null,leftID='',rightID='',signature='',listSignature='',timer=null,selected=-1,syncing=false,disposed=false,scale=0,drag=null,lastSide='left',layoutGeneration=0,pixelReady=false;const offsets={left:[0],right:[0]},geometryCache={left:null,right:null},syntaxKeys={left:'',right:''},programScroll={left:null,right:null};
 host.className='nt-editor-compare'+(comparing?'':' is-split-view');
 const alignmentUnavailable=comparing&&(!root.NTKEditorAlignedCompare||!root.NetworkToolkitCodeMirror||!root.NTKEditorAlignmentModel);
 const selectors=['left','right'].map(side=>'<label class="nt-compare-selector">'+(side==='left'?'Left':'Right')+' <select data-compare-select="'+side+'" aria-label="'+side+' comparison document"></select></label>').join('');
 host.innerHTML='<div class="nt-compare-tools">'+selectors+'<button type="button" class="btn small ghost" data-compare="swap" title="Swap left and right documents">Swap sides</button><button type="button" class="btn small ghost" data-compare="previous" title="Previous difference">Previous</button><button type="button" class="btn small ghost" data-compare="next" title="Next difference">Next</button><details class="nt-compare-settings"><summary class="btn small ghost">Options</summary><div><label><input type="checkbox" data-compare-sync checked> Sync scroll</label><label title="Fit shows the entire comparison. Zoom expands only the map, never the editor text.">Map zoom <select data-compare-scale aria-label="Map zoom"><option value="0">Fit entire document</option><option value="0.5">1.5×</option><option value="1">2×</option><option value="2">3×</option><option value="4">5×</option></select></label><span class="nt-compare-legend"><span class="added">+ Added</span><span class="removed">− Removed</span><span class="changed">~ Modified</span><span>= Unchanged</span><span class="moved">↔ Moved</span><span>Blank = alignment gap</span></span></div></details></div><div class="nt-compare-summary" data-compare-summary role="status"></div><div class="nt-compare-grid">'+['left','right'].map(side=>'<section class="nt-compare-pane" data-compare-pane="'+side+'"><div class="nt-editor-tabs" data-ed-tabs data-ed-view="'+side+'" role="tablist" aria-label="'+side+' view document tabs"></div><div class="nt-compare-surface"><pre data-compare-lines="'+side+'" aria-hidden="true"></pre><pre class="nt-compare-highlight" data-compare-highlight="'+side+'" aria-hidden="true" hidden></pre><textarea id="ntCompare-'+side+'" data-compare-text="'+side+'" data-input-clear="off" wrap="off" spellcheck="false" aria-label="Edit '+side+' comparison document"></textarea></div><div class="nt-compare-document-state" data-compare-state="'+side+'"></div><div class="nt-compare-syntax-notice" data-compare-syntax-notice="'+side+'" role="status" hidden></div></section>').join('<div class="nt-editor-pane-resize" data-compare-divider></div>')+'<div class="nt-compare-map-panel"><div class="nt-compare-map-labels" aria-hidden="true"><span>Left</span><span>Right</span></div><div class="nt-compare-map" data-compare-map><div data-compare-map-content>'+['left','right'].map(side=>'<div class="nt-compare-track" data-compare-track="'+side+'" tabindex="0" role="scrollbar" aria-label="'+side+' document map" aria-orientation="vertical" aria-controls="ntCompare-'+side+'" aria-valuemin="0" title="Click or drag anywhere to navigate this document. Fit: wheel scrolls the document. Zoom: wheel pans the map and document."><svg data-compare-map-marks="'+side+'" aria-hidden="true" preserveAspectRatio="none"></svg><div data-compare-map-viewport="'+side+'" aria-hidden="true"></div></div>').join('')+'</div></div></div></div>';
 mountSettings(host,options);
 const q=sel=>host.querySelector(sel),panes={left:q('[data-compare-text="left"]'),right:q('[data-compare-text="right"]')},map=q('[data-compare-map]'),mapContent=q('[data-compare-map-content]');
 const tracks={left:q('[data-compare-track="left"]'),right:q('[data-compare-track="right"]')};
 const preview=comparing&&mountDifferencePreview(host,(start,count)=>inlinePreviewRows(result,doc).slice(start,start+count),()=>inlinePreviewRows(result,doc)?.length);
 const splitControl=createDivider(host,q('.nt-compare-grid'),comparing,()=>refresh(true));

 const docs=()=>options.documents(),doc=side=>docs().find(d=>d.id===(side==='left'?leftID:rightID));
 const metrics=side=>{const ta=panes[side],s=root.getComputedStyle(ta);return {height:Math.max(1,parseFloat(s.lineHeight)||20),padding:parseFloat(s.paddingTop)||10,ta};};
 function remember(side){const d=doc(side),ta=panes[side];if(d)d[comparing?'compareView':'view']={start:ta.selectionStart,end:ta.selectionEnd,direction:ta.selectionDirection,top:ta.scrollTop,left:ta.scrollLeft};}
 function fill(side,d,changed){const ta=panes[side],view=changed?(d?.[comparing?'compareView':'view']||d?.view):null;ta.disabled=(!d&&(comparing||docs().length>=12))||!!d?.loading||!!d?.frozen;ta.placeholder=!d&&!comparing?(docs().length>=12?'Move a tab here, or close a tab before creating another.':'Type here or move a tab to this view.'):'';ta.wrap=options.wordWrap?.()?'soft':'off';ta.classList.toggle('is-wrapped',!!options.wordWrap?.());if(ta.value!==(d?.content||'')){const own={start:ta.selectionStart,end:ta.selectionEnd,top:ta.scrollTop,left:ta.scrollLeft};ta.value=d?.content||'';const v=view||(!changed?own:null);if(v){ta.setSelectionRange(clamp(v.start,0,ta.value.length),clamp(v.end,0,ta.value.length),v.direction);ta.scrollTop=v.top;ta.scrollLeft=v.left;}}else if(view){ta.setSelectionRange(clamp(view.start,0,ta.value.length),clamp(view.end,0,ta.value.length),view.direction);ta.scrollTop=view.top;ta.scrollLeft=view.left;}q('[data-compare-state="'+side+'"]').textContent=d?(d.source+' · '+d.path+(d.dirty?' · Unsaved':' · Saved')+(d.saving?' · Saving…':'')):'Open or create a second document.';}
 function refresh(force=false){if(disposed)return;const all=docs(),prevLeft=leftID,prevRight=rightID;if(options.selection){const chosen=options.selection();leftID=chosen.left||'';rightID=chosen.right||'';}else if(!all.some(d=>d.id===leftID))leftID=options.active()?.id||all[0]?.id||'';if(!options.selection&&!all.some(d=>d.id===rightID))rightID=all.find(d=>d.id!==leftID)?.id||'';
  const key=all.map(d=>[d.id,d.path,d.dirty,d.loading,d.saving].join(':')).join('|');if(key!==listSignature){listSignature=key;for(const side of ['left','right']){const select=q('[data-compare-select="'+side+'"]');select.innerHTML='<option value="">Choose document</option>'+all.map(d=>'<option value="'+escape(d.id)+'">'+escape((d.dirty?'* ':'')+d.path+' · '+d.source)+'</option>').join('');select.value=side==='left'?leftID:rightID;}}
  for(const side of ['left','right'])q('[data-compare-select="'+side+'"]').value=side==='left'?leftID:rightID;
  fill('left',doc('left'),prevLeft!==leftID);fill('right',doc('right'),prevRight!==rightID);
  const l=doc('left'),r=doc('right'),next=[l?.uid||l?.id,l?.editRevision,r?.uid||r?.id,r?.editRevision].join(':');if(force||next!==signature){signature=next;layoutGeneration++;pixelReady=false;map.setAttribute('aria-disabled','true');for(const side of ['left','right'])tracks[side].setAttribute('aria-disabled','true');updateMap();clearTimeout(timer);timer=setTimeout(compute,force?0:120);}else updateMap();
 }
 async function compute(){timer=null;const generation=++layoutGeneration;pixelReady=false;if(disposed)return;const l=doc('left'),r=doc('right');result=comparing&&l&&r&&!l.loading&&!r.loading?compare(l.content,r.content,q('[data-compare-spaces]')?.value):null;selected=-1;
  q('[data-compare-summary]').textContent=(alignmentUnavailable?'Aligned editing is unavailable: a local component could not load. Unaligned source editors remain usable. ':'')+(!result?'Choose two open documents to compare.':result.limited?result.message:result.identical?'Documents are identical.':result.hunks.length+' difference'+(result.hunks.length===1?'':'s')+'. Use Previous / Next or the change map.');
  if(result?.limited){for(const side of ['left','right']){q('[data-compare-lines="'+side+'"]').innerHTML='';paintSyntax(side);}drawMap();return;}
  await Promise.all(['left','right'].map(side=>measure(side,generation)));if(disposed||generation!==layoutGeneration)return;pixelReady=true;for(const side of ['left','right']){paintPane(side);paintSyntax(side);}
  for(const el of host.querySelectorAll(differenceControls))el.disabled=!result?.hunks?.length;preview?.refresh?.();
  drawMap();syncScroll(lastSide);
 }
 async function measure(side,generation){const d=doc(side),{ta,height}=metrics(side),lines=(d?.content||'').split('\n');ta.style.setProperty('--nt-compare-gutter',String(Math.max(3,String(lines.length).length))+'ch');const style=root.getComputedStyle(ta),width=Math.max(1,ta.clientWidth-parseFloat(style.paddingLeft)-parseFloat(style.paddingRight)),wrapped=!!options.wordWrap?.(),key=[d?.uid||d?.id,d?.editRevision,width,style.font,style.lineHeight,style.tabSize,wrapped].join(':');if(geometryCache[side]?.key===key){offsets[side]=geometryCache[side].offsets;return;}const values=[0];if(!wrapped){for(let i=0;i<lines.length;i++)values.push(values.at(-1)+height);}else{const node=document.createElement('div');node.className='nt-compare-measure';Object.assign(node.style,{width:width+'px',font:style.font,lineHeight:style.lineHeight,tabSize:style.tabSize||'4'});host.append(node);try{for(let at=0;at<lines.length;at+=250){if(disposed||generation!==layoutGeneration)return;node.innerHTML=lines.slice(at,at+250).map(line=>'<div>'+escape(line||'\u200b')+'</div>').join('');for(const line of node.children)values.push(values.at(-1)+Math.max(height,line.getBoundingClientRect().height));if(lines.length>500)await new Promise(resolve=>root.requestAnimationFrame(resolve));}}finally{node.remove();}}if(generation===layoutGeneration){offsets[side]=values;geometryCache[side]={key,offsets:values};}}
 function paintSyntax(side){const d=doc(side),ta=panes[side],pre=q('[data-compare-highlight="'+side+'"]'),language=d?options.language?.(d)||'text':'text',limit=root.NTKNetworkConfig?.supports(language)?root.NTKNetworkConfig.MAX_PAINT:250000,notice=q('[data-compare-syntax-notice="'+side+'"]'),enabled=!!options.syntax?.()&&!!d&&d.content.length<=limit&&language!=='text';notice.textContent=options.syntax?.()&&d&&d.content.length>limit?'Plain text: syntax highlighting is limited to '+limit.toLocaleString()+' characters; contents unchanged.':'';notice.hidden=!notice.textContent;pre.hidden=true;ta.classList.remove('highlight-active');if(!enabled){syntaxKeys[side]='';root.NTKEditorLanguageService?.cancelHighlight?.('compare-'+side);return;}const text=d.content,key=[d.uid||d.id,d.editRevision,language].join(':');syntaxKeys[side]=key;const apply=(html,limited=false,reason='')=>{if(disposed||syntaxKeys[side]!==key||doc(side)!==d||d.content!==text||(options.language?.(d)||'text')!==language||!options.syntax?.())return;notice.textContent=limited?'Plain text: '+reason:'';notice.hidden=!notice.textContent;pre.innerHTML=html+'\n';pre.hidden=false;ta.classList.add('highlight-active');pre.classList.toggle('is-wrapped',!!options.wordWrap?.());pre.style.width=options.wordWrap?.()?ta.clientWidth+'px':'';pre.style.setProperty('--nt-compare-gutter',ta.style.getPropertyValue('--nt-compare-gutter'));pre.style.transform='translate('+(-ta.scrollLeft)+'px,'+(-ta.scrollTop)+'px)';};if(d.syntaxCache?.text===text&&d.syntaxCache.language===language){apply(d.syntaxCache.html,d.syntaxCache.limited,d.syntaxCache.reason);return;}void root.NTKEditorLanguageService?.highlight(text,language,{channel:'compare-'+side}).then(value=>{if(doc(side)===d&&d.content===text){d.syntaxCache={text,language,html:value.html,limited:!!value.limited,reason:value.reason||''};apply(value.html,value.limited,value.reason);}}).catch(()=>{});}
 function paintPane(side){const d=doc(side),{ta,padding}=metrics(side),values=offsets[side],count=d?Math.max(0,offsets[side].length-1):0,first=Math.max(0,Math.floor(lineAtOffset(values,Math.max(0,ta.scrollTop-padding)))-4),last=Math.min(count,Math.ceil(lineAtOffset(values,ta.scrollTop+ta.clientHeight))+5),pre=q('[data-compare-lines="'+side+'"]');pre.style.paddingTop=(padding+(values[first]||0))+'px';pre.innerHTML=Array.from({length:Math.max(0,last-first)},(_,at)=>{const i=first+at;return '<span class="nt-compare-line '+(result?.kinds?.[side]?.[i]||'equal')+'" style="height:'+((values[i+1]-values[i])||metrics(side).height)+'px">'+(i+1)+'</span>';}).join('');pre.style.transform='translateY('+(-ta.scrollTop)+'px)';ta.style.setProperty('--nt-compare-gutter',String(Math.max(3,String(count||1).length))+'ch');const syntax=q('[data-compare-highlight="'+side+'"]');syntax.style.transform='translate('+(-ta.scrollLeft)+'px,'+(-ta.scrollTop)+'px)';}

 // Both tracks share comparison rows, but each maps to its own real document lines.
 function drawMap(){
  const rows=result?.rows?.length||1,height=mapHeight(rows,Math.max(1,map.clientHeight),scale);
  mapContent.style.height=height+'px';
  for(const side of ['left','right']){
   const svg=q('[data-compare-map-marks="'+side+'"]');svg.setAttribute('viewBox','0 0 20 '+rows);
   const runs=[];
   for(const [i,row] of (result?.rows||[]).entries()){
    const kind=row[side]===null?'gap':result.kinds[side][row[side]],last=runs.at(-1);
    if(last?.kind===kind)last.end=i+1;else runs.push({kind,start:i,end:i+1});
   }
   svg.innerHTML=runs.map(r=>'<rect class="'+r.kind+'" x="1" width="18" y="'+r.start+'" height="'+(r.end-r.start)+'"/>').join('');
   tracks[side].setAttribute('aria-disabled',String(!result?.positions||!pixelReady));
   tracks[side].setAttribute('aria-valuemax',String(Math.max(0,(result?.lineCounts?.[side]||1)-1)));
  }
  map.setAttribute('aria-disabled',String(!result?.positions||!pixelReady));updateMap();
 }
 function updateMap(){
  for(const side of ['left','right']){
   const marker=q('[data-compare-map-viewport="'+side+'"]');marker.hidden=!result?.positions||!pixelReady;
   if(marker.hidden)continue;
   const {padding,ta}=metrics(side),first=lineAtOffset(offsets[side],Math.max(0,ta.scrollTop-padding)),last=lineAtOffset(offsets[side],ta.scrollTop+ta.clientHeight-padding);
   const view={start:rowAtLine(result.positions[side],first),end:rowAtLine(result.positions[side],last)},total=result.rows.length||1,h=mapContent.offsetHeight;
   const top=view.start/total*h;marker.style.top=top+'px';marker.style.height=Math.min(h-top,Math.max(4,(view.end-view.start)/total*h))+'px';
   tracks[side].setAttribute('aria-valuenow',String(Math.min(result.lineCounts[side]-1,Math.floor(first))));
   tracks[side].setAttribute('aria-valuetext',side+' document lines '+(Math.floor(first)+1)+' to '+Math.min(result.lineCounts[side],Math.ceil(last)));
  }
 }
 function revealMap(side){if(!scale||drag!==null)return;const marker=q('[data-compare-map-viewport="'+side+'"]'),top=parseFloat(marker.style.top)||0,bottom=top+parseFloat(marker.style.height||0);if(top<map.scrollTop)map.scrollTop=top;else if(bottom>map.scrollTop+map.clientHeight)map.scrollTop=Math.min(top,bottom-map.clientHeight);}
 function syncScroll(side,other=true){if(syncing)return;lastSide=side;const ta=panes[side];paintPane(side);remember(side);if(other&&q('[data-compare-sync]').checked&&result?.positions&&pixelReady){syncing=true;const target=side==='left'?'right':'left',a=metrics(side),b=metrics(target),row=rowAtLine(result.positions[side],lineAtOffset(offsets[side],Math.max(0,ta.scrollTop-a.padding)));b.ta.scrollTop=row<=0?0:clamp(offsetAtLine(offsets[target],lineAtRow(result.positions[target],row))+b.padding,0,b.ta.scrollHeight-b.ta.clientHeight);programScroll[target]=b.ta.scrollTop;paintPane(target);remember(target);syncing=false;}updateMap();revealMap(side);}
 function jump(row,center=false,side=lastSide,both=q('[data-compare-sync]').checked){if(!result?.positions||!pixelReady)return;row=clamp(row,0,result.rows.length);lastSide=side;const sides=both?['left','right']:[side];syncing=true;for(const side of sides){const {ta,height,padding}=metrics(side);ta.scrollTop=row<=0?0:clamp(offsetAtLine(offsets[side],lineAtRow(result.positions[side],row))+padding-(center?ta.clientHeight/2:0),0,ta.scrollHeight-ta.clientHeight);programScroll[side]=ta.scrollTop;paintPane(side);remember(side);}syncing=false;updateMap();}
 function difference(step){if(!result?.hunks?.length||!pixelReady)return;const wrap=q('[data-compare-wrap-nav]').checked,index=differenceIndex(selected,step,result.hunks.length,wrap);if(!wrap&&typeof step==='number'&&index===selected)return;selected=index;preview.reveal(inlineChangedStart(result,selected));const h=result.hunks[selected];jump(h.start,false,lastSide,true);revealMap(lastSide);if(q('[data-compare-move-caret]').checked)for(const side of ['left','right']){const d=doc(side),line=h[side];if(d&&line!==null){const start=d.content.split('\n').slice(0,line).reduce((n,s)=>n+s.length+1,0);panes[side].setSelectionRange(start,start+(d.content.split('\n')[line]||'').length);remember(side);}}q('[data-compare-summary]').textContent='Difference '+(selected+1)+' of '+result.hunks.length+' · '+h.type+'.';}
 function mapPoint(e,side){const r=map.getBoundingClientRect();jump(rowFromPointer(e.clientY,r.top,map.scrollTop,mapContent.offsetHeight,result.rows.length),true,side);}
 host.addEventListener('click',async e=>{const action=e.target.closest?.('[data-compare]');if(!action)return;e.preventDefault();e.stopPropagation();if(action.dataset.compare==='first'||action.dataset.compare==='last')difference(action.dataset.compare);if(action.dataset.compare==='next')difference(1);if(action.dataset.compare==='previous')difference(-1);if(action.dataset.compare==='swap'){for(const side of ['left','right'])remember(side);options.swap?.();[leftID,rightID]=[rightID,leftID];listSignature='';for(const side of ['left','right'])fill(side,doc(side),true);refresh(true);}});
 host.addEventListener('change',e=>{if(e.target.dataset.compareSelect){const side=e.target.dataset.compareSelect;remember(side);if(options.choose)options.choose(side,e.target.value);if(side==='left')leftID=e.target.value;else rightID=e.target.value;signature='';fill(side,doc(side),true);refresh(true);}if(e.target.matches('[data-compare-scale]')){scale=Number(e.target.value)||0;drawMap();const marker=q('[data-compare-map-viewport="'+lastSide+'"]');map.scrollTop=Math.max(0,parseFloat(marker.style.top)-map.clientHeight/2);}if(e.target.matches('[data-compare-spaces]'))refresh(true);if(e.target.matches('[data-compare-sync]'))syncScroll(lastSide);});
 host.addEventListener('focusin',e=>{const side=e.target.dataset.compareText;if(side&&doc(side)){lastSide=side;options.activate(doc(side),side);for(const name of ['left','right'])q('[data-compare-pane="'+name+'"]').classList.toggle('is-active-view',name===side);updateMap();}});
 host.addEventListener('input',async e=>{const side=e.target.dataset.compareText;if(!side)return;let d=doc(side);if(!d&&!comparing){const text=e.target.value,start=e.target.selectionStart,end=e.target.selectionEnd;d=await options.createInView?.(side);if(disposed)return;if(d){options.edit(d,text);refresh(true);panes[side].setSelectionRange(start,end);panes[side].focus();remember(side);}return;}if(d){options.edit(d,e.target.value);remember(side);refresh();}});
 host.addEventListener('scroll',e=>{const side=e.target.dataset.compareText;if(side){if(programScroll[side]!==null&&Math.abs(e.target.scrollTop-programScroll[side])<1){programScroll[side]=null;paintPane(side);remember(side);updateMap();return;}programScroll[side]=null;syncScroll(side);}},true);
 host.addEventListener('keydown',e=>{if(e.target.closest?.('[data-ed-tabs]')||(!comparing&&e.target.dataset.compareText)||((e.ctrlKey||e.metaKey)&&['n','o'].includes(e.key.toLowerCase())))return;const track=e.target.dataset.compareTrack;if(track){const keys={ArrowUp:-1,ArrowDown:1,PageUp:-Math.max(1,panes[track].clientHeight/metrics(track).height),PageDown:Math.max(1,panes[track].clientHeight/metrics(track).height)};if(Object.hasOwn(keys,e.key)||['Home','End'].includes(e.key)){e.preventDefault();if(!result?.positions||!pixelReady)return;const line=Number(tracks[track].getAttribute('aria-valuenow'))||0;const row=e.key==='Home'?0:e.key==='End'?result.rows.length:rowAtLine(result.positions[track],line+keys[e.key]);jump(row,false,track);revealMap(track);}}

  const side=e.target.dataset.compareText;if(side){const d=doc(side),ctrl=e.ctrlKey||e.metaKey,key=e.key.toLowerCase();if(ctrl&&['s','z','y'].includes(key)){e.preventDefault();if(key==='s')void options.save(d,e.shiftKey);else{options.undo(d,key==='y'||e.shiftKey);refresh(true);}}if(e.key==='Tab'){e.preventDefault();const ta=e.target,start=ta.selectionStart,end=ta.selectionEnd;ta.setRangeText('\t',start,end,'end');options.edit(d,ta.value);remember(side);refresh();}}
  e.stopPropagation();});
 for(const side of ['left','right']){
  const track=tracks[side];
  track.addEventListener('pointerdown',e=>{if(e.button!==0||!result?.positions||!pixelReady)return;e.preventDefault();drag=e.pointerId;track.setPointerCapture?.(drag);track.focus({preventScroll:true});mapPoint(e,side);});
  track.addEventListener('pointermove',e=>{if(drag!==e.pointerId)return;const r=map.getBoundingClientRect();if(scale&&(e.clientY<r.top||e.clientY>r.bottom))map.scrollTop+=e.clientY<r.top?-20:20;mapPoint(e,side);});
  const release=e=>{if(drag!==e.pointerId)return;drag=null;track.releasePointerCapture?.(e.pointerId);};
  track.addEventListener('pointerup',release);track.addEventListener('pointercancel',release);track.addEventListener('lostpointercapture',()=>{drag=null;});
  track.addEventListener('wheel',e=>{e.preventDefault();e.stopPropagation();if(!result?.positions||!pixelReady)return;
   const delta=e.deltaY*(e.deltaMode===1?metrics(side).height:e.deltaMode===2?map.clientHeight:1);
   if(scale){map.scrollTop+=delta;const r=map.getBoundingClientRect();jump(rowFromPointer(e.clientY,r.top,map.scrollTop,mapContent.offsetHeight,result.rows.length),true,side);}
   else{const ta=panes[side];ta.scrollTop+=delta;programScroll[side]=ta.scrollTop;syncScroll(side);}
  },{passive:false});
 }

 const observer=typeof ResizeObserver==='function'?new ResizeObserver(()=>{splitControl.apply();refresh(true);}):null;observer?.observe(map);observer?.observe(q('.nt-compare-grid'));splitControl.apply();for(const side of ['left','right'])observer?.observe(panes[side]);refresh(true);
 return {refresh,difference,jump,get textarea(){return panes[options.activeSide?.()||lastSide];},get activeSide(){return lastSide;},scrollActive(){syncScroll(options.activeSide?.()||lastSide,false);},positionAt(offset,focus=true){const side=options.activeSide?.()||lastSide,ta=panes[side],d=doc(side);if(!d)return;lastSide=side;ta.setSelectionRange(offset,offset);if(focus)ta.focus({preventScroll:true});else keepScriptSelection(ta);const line=d.content.slice(0,offset).split('\n').length-1;ta.scrollTop=Math.max(0,(offsets[side][line]||0)-metrics(side).height);syncScroll(side,false);},showDocument(id){if(!docs().some(d=>d.id===id))return;const side=options.sideOf?.(docs().find(d=>d.id===id)?.uid)||(leftID===id?'left':rightID===id?'right':lastSide);lastSide=side;remember(side);if(side==='left')leftID=id;else rightID=id;listSignature='';fill(side,doc(side),true);refresh(true);panes[side].focus({preventScroll:true});},undo(redo=false){const d=doc(lastSide);if(d){options.undo(d,redo);refresh(true);}},snapshot(){for(const side of ['left','right'])remember(side);return {left:leftID,right:rightID,scale,sync:q('[data-compare-sync]').checked};},restore(value){if(!value)return;const activeBefore=options.active()?.id;leftID=String(value.left||'');rightID=String(value.right||'');options.choose?.('left',leftID);options.choose?.('right',rightID);if([leftID,rightID].includes(activeBefore)){const activeDocument=docs().find(d=>d.id===activeBefore);if(activeDocument)options.activate(activeDocument,options.sideOf?.(activeDocument.uid));}scale=[0,.5,1,2,4].includes(value.scale)?value.scale:0;q('[data-compare-scale]').value=String(scale);q('[data-compare-sync]').checked=value.sync!==false;refresh(true);},get result(){return result;},get selection(){return {left:leftID,right:rightID};},dispose(){preview?.dispose?.();splitControl.dispose();disposed=true;layoutGeneration++;clearTimeout(timer);root.NTKEditorLanguageService?.cancelHighlight?.('compare-left');root.NTKEditorLanguageService?.cancelHighlight?.('compare-right');for(const side of ['left','right'])remember(side);observer?.disconnect();host.replaceChildren();}};
}
const api=Object.freeze({LIMITS,compare,isLowSimilarityReplacement,lineAtRow,rowAtLine,viewport,offsetAtLine,lineAtOffset,mapHeight,rowFromPointer,splitBounds,splitRatio,createDivider,placeToolbarDropdown,mountSettings,mountDifferencePreview,inlinePreviewRows,inlineChangedStart,inlineDifferencePage,previewSourceLines,differenceIndex,differenceControls,createHorizontalSync,create});root.NTKEditorCompare=api;if(typeof module==='object'&&module.exports)module.exports=api;
})(typeof window==='undefined'?globalThis:window);
