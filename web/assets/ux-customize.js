/* Owner-requested customization (v1.20.9):
 * - Test a target / Test a website tabs reorder by drag (6 px threshold, drop
 *   line only where the order would change, Escape cancels) or Alt+Shift+Left/Right.
 *   Order is saved in state.testTabOrder = {target:[...], web:[...]}.
 * - The Overview dashboard is customizable: show/hide and reorder its sections
 *   (state.overviewLayout = {order:[...], hidden:[...]}) from the Customize dialog.
 * - Overview sections also move by dragging their grip handle (same drop-line
 *   rule; the line is drawn on the side being passed) or Alt+Shift+Up/Down on
 *   the handle (v1.20.11). The order is the same state.overviewLayout.order.
 * Tab buttons keep their data-page navigation; a completed drag suppresses the click. */
(function(root){
'use strict';
const document=root.document;if(!document)return;
const call=(name,...args)=>typeof root[name]==='function'?root[name](...args):undefined;
function appState(){return typeof state!=='undefined'?state:null;}
function save(){if(typeof saveState==='function')saveState();}

// ---------- Test tab order ----------
function tabList(node){return node?.closest?.('.nt-target-tests[data-test-tab-group]');}
function tabIds(list){return [...list.querySelectorAll('[data-target-test]')].map(b=>b.dataset.targetTest);}
function storeOrder(list){const s=appState();if(!s)return;s.testTabOrder={...(s.testTabOrder&&typeof s.testTabOrder==='object'?s.testTabOrder:{}),[list.dataset.testTabGroup]:tabIds(list)};save();}
function unchanged(ids,id,target,after){const at=ids.indexOf(id),to=ids.indexOf(target);return at<0||to<0||to===at||(after?to===at-1:to===at+1);}
function moveTab(list,id,target,after){const ids=tabIds(list);if(unchanged(ids,id,target,after))return false;const button=list.querySelector('[data-target-test="'+CSS.escape(id)+'"]'),ref=list.querySelector('[data-target-test="'+CSS.escape(target)+'"]');if(!button||!ref)return false;ref.insertAdjacentElement(after?'afterend':'beforebegin',button);storeOrder(list);return true;}
let drag=null,suppressClick=0;
function clearDrop(){for(const el of document.querySelectorAll('.test-tab-drop-before,.test-tab-drop-after,.test-tab-dragging'))el.classList.remove('test-tab-drop-before','test-tab-drop-after','test-tab-dragging');}
function dropTarget(event){const button=document.elementFromPoint?.(event.clientX,event.clientY)?.closest?.('[data-target-test]');if(!button||tabList(button)!==drag.list)return null;const r=button.getBoundingClientRect(),after=event.clientX>r.left+r.width/2;if(unchanged(tabIds(drag.list),drag.id,button.dataset.targetTest,after))return null;return {button,id:button.dataset.targetTest,after};}
function endDrag(apply){const d=drag;drag=null;clearDrop();if(!d?.moved)return;suppressClick=Date.now()+400;if(apply&&d.drop)moveTab(d.list,d.id,d.drop.id,d.drop.after);}
document.addEventListener('pointerdown',event=>{const button=event.target.closest?.('.nt-target-tests [data-target-test]');if(!button||event.button!==0||event.isPrimary===false||event.pointerType==='touch'||event.ctrlKey||event.metaKey||event.altKey||event.shiftKey)return;drag={id:button.dataset.targetTest,list:tabList(button),button,pointer:event.pointerId,x:event.clientX,y:event.clientY,moved:false,drop:null};},true);
document.addEventListener('pointermove',event=>{if(!drag||event.pointerId!==drag.pointer)return;if(!drag.moved){if(Math.hypot(event.clientX-drag.x,event.clientY-drag.y)<6)return;drag.moved=true;try{drag.button.setPointerCapture?.(event.pointerId);}catch{}}event.preventDefault();clearDrop();drag.button.classList.add('test-tab-dragging');drag.drop=dropTarget(event);drag.drop?.button.classList.add(drag.drop.after?'test-tab-drop-after':'test-tab-drop-before');},true);
document.addEventListener('pointerup',event=>{if(drag&&event.pointerId===drag.pointer)endDrag(true);},true);
document.addEventListener('pointercancel',event=>{if(drag&&event.pointerId===drag.pointer)endDrag(false);},true);
document.addEventListener('click',event=>{if(Date.now()<suppressClick&&event.target.closest?.('.nt-target-tests')){event.preventDefault();event.stopImmediatePropagation();suppressClick=0;}},true);
document.addEventListener('keydown',event=>{
 if(event.key==='Escape'&&drag?.moved){event.preventDefault();event.stopImmediatePropagation();endDrag(false);return;}
 if(!event.altKey||!event.shiftKey||event.ctrlKey||!['ArrowLeft','ArrowRight'].includes(event.key))return;
 const button=event.target.closest?.('.nt-target-tests [data-target-test]');if(!button)return;event.preventDefault();event.stopImmediatePropagation();
 const list=tabList(button),ids=tabIds(list),at=ids.indexOf(button.dataset.targetTest),next=ids[at+(event.key==='ArrowLeft'?-1:1)];
 if(next&&moveTab(list,button.dataset.targetTest,next,event.key==='ArrowRight'))button.focus({preventScroll:true});
},true);

// ---------- Customizable dashboard ----------
function layout(){return typeof overviewLayout==='function'?overviewLayout():{order:[],hidden:[]};}
function sections(){return typeof OVERVIEW_SECTIONS!=='undefined'?OVERVIEW_SECTIONS:[];}
function setLayout(next){const s=appState();if(!s)return;if(next)s.overviewLayout={order:next.order.slice(),hidden:next.hidden.slice()};else delete s.overviewLayout;save();if(s.activePage==='overview'&&typeof renderOverview==='function')renderOverview();renderDialog();}
function dialogHTML(){const {order,hidden}=layout(),names=new Map(sections());return '<p class="help-line">Choose which sections the Overview shows and their order. Network adapters and Recent activity sit side by side when they are next to each other.</p><ol class="overview-customize-list" id="overviewCustomizeList">'+order.map((id,i)=>'<li class="overview-customize-row" data-overview-id="'+esc(id)+'"><label class="toggle-row"><input type="checkbox" data-overview-show="'+esc(id)+'" '+(hidden.includes(id)?'':'checked')+'><span>'+esc(names.get(id)||id)+'</span></label><span class="overview-customize-move"><button type="button" class="iconbtn" data-overview-move="up" data-overview-id="'+esc(id)+'" aria-label="Move '+esc(names.get(id)||id)+' up" title="Move up" '+(i===0?'disabled':'')+'>'+icon('arrowUp')+'</button><button type="button" class="iconbtn" data-overview-move="down" data-overview-id="'+esc(id)+'" aria-label="Move '+esc(names.get(id)||id)+' down" title="Move down" '+(i===order.length-1?'disabled':'')+'>'+icon('arrowDown')+'</button></span></li>').join('')+'</ol>';}
function renderDialog(){const list=document.querySelector('#overviewCustomizeHost');if(list)list.innerHTML=dialogHTML();}
function openCustomize(){if(typeof openModal!=='function')return;openModal('Customize Overview','<div id="overviewCustomizeHost">'+dialogHTML()+'</div>','<button type="button" class="btn ghost" data-overview-reset>Reset to default</button><button class="btn primary" data-action="close-modal">Done</button>');}
document.addEventListener('click',event=>{
 if(event.target.closest?.('[data-action="customize-overview"]')){event.preventDefault();event.stopImmediatePropagation();openCustomize();return;}
 const move=event.target.closest?.('[data-overview-move]');
 if(move){event.preventDefault();const l=layout(),id=move.dataset.overviewId,at=l.order.indexOf(id),to=at+(move.dataset.overviewMove==='up'?-1:1);if(at<0||to<0||to>=l.order.length)return;[l.order[at],l.order[to]]=[l.order[to],l.order[at]];setLayout(l);document.querySelector('[data-overview-move="'+move.dataset.overviewMove+'"][data-overview-id="'+CSS.escape(id)+'"]:not(:disabled)')?.focus();return;}
 if(event.target.closest?.('[data-overview-reset]')){event.preventDefault();setLayout(null);}
},true);
document.addEventListener('change',event=>{const box=event.target.closest?.('[data-overview-show]');if(!box)return;const l=layout(),id=box.dataset.overviewShow;l.hidden=box.checked?l.hidden.filter(x=>x!==id):[...new Set([...l.hidden,id])];setLayout(l);document.querySelector('[data-overview-show="'+CSS.escape(id)+'"]')?.focus();});

// ---------- Drag Overview sections ----------
// The order compares shown sections only: hidden ones keep their place in the
// saved order and are skipped when deciding whether a drop changes anything.
function shownIds(){return [...document.querySelectorAll('#pageContent [data-overview-part]')].map(e=>e.dataset.overviewPart);}
function moveSection(id,target,after){const shown=shownIds();if(unchanged(shown,id,target,after))return false;const l=layout(),order=l.order.filter(x=>x!==id),to=order.indexOf(target);if(to<0)return false;order.splice(after?to+1:to,0,id);setLayout({order,hidden:l.hidden});return true;}
let sectionDrag=null;
function clearSectionDrop(){for(const el of document.querySelectorAll('.overview-drop-before,.overview-drop-after,.overview-drop-left,.overview-drop-right,.overview-dragging'))el.classList.remove('overview-drop-before','overview-drop-after','overview-drop-left','overview-drop-right','overview-dragging');}
function sectionDropTarget(event){const part=document.elementFromPoint?.(event.clientX,event.clientY)?.closest?.('#pageContent [data-overview-part]');if(!part)return null;const r=part.getBoundingClientRect(),side=part.parentElement?.closest?.('.columns[data-overview-section]')&&!part.hasAttribute('data-overview-section'),after=side?event.clientX>r.left+r.width/2:event.clientY>r.top+r.height/2,id=part.dataset.overviewPart;if(unchanged(shownIds(),sectionDrag.id,id,after))return null;return {part,id,after,side};}
function endSectionDrag(apply){const d=sectionDrag;sectionDrag=null;clearSectionDrop();document.documentElement.classList.remove('overview-dragging-active');if(apply&&d?.moved&&d.drop){moveSection(d.id,d.drop.id,d.drop.after);document.querySelector('[data-overview-drag="'+CSS.escape(d.id)+'"]')?.focus({preventScroll:true});}}
document.addEventListener('pointerdown',event=>{const handle=event.target.closest?.('[data-overview-drag]');if(!handle||event.button!==0||event.isPrimary===false||event.pointerType==='touch')return;event.preventDefault();sectionDrag={id:handle.dataset.overviewDrag,handle,pointer:event.pointerId,x:event.clientX,y:event.clientY,moved:false,drop:null};},true);
document.addEventListener('pointermove',event=>{const d=sectionDrag;if(!d||event.pointerId!==d.pointer)return;if(!d.moved){if(Math.hypot(event.clientX-d.x,event.clientY-d.y)<6)return;d.moved=true;document.documentElement.classList.add('overview-dragging-active');try{d.handle.setPointerCapture?.(event.pointerId);}catch{}}event.preventDefault();clearSectionDrop();d.handle.closest('[data-overview-part]')?.classList.add('overview-dragging');d.drop=sectionDropTarget(event);if(d.drop)d.drop.part.classList.add(d.drop.side?(d.drop.after?'overview-drop-right':'overview-drop-left'):(d.drop.after?'overview-drop-after':'overview-drop-before'));},true);
document.addEventListener('pointerup',event=>{if(sectionDrag&&event.pointerId===sectionDrag.pointer)endSectionDrag(true);},true);
document.addEventListener('pointercancel',event=>{if(sectionDrag&&event.pointerId===sectionDrag.pointer)endSectionDrag(false);},true);
document.addEventListener('keydown',event=>{
 if(event.key==='Escape'&&sectionDrag?.moved){event.preventDefault();event.stopImmediatePropagation();endSectionDrag(false);return;}
 if(!event.altKey||!event.shiftKey||event.ctrlKey||!['ArrowUp','ArrowDown'].includes(event.key))return;
 const handle=event.target.closest?.('[data-overview-drag]');if(!handle)return;event.preventDefault();event.stopImmediatePropagation();
 const id=handle.dataset.overviewDrag,shown=shownIds(),at=shown.indexOf(id),up=event.key==='ArrowUp',next=shown[at+(up?-1:1)];
 if(next&&moveSection(id,next,!up))document.querySelector('[data-overview-drag="'+CSS.escape(id)+'"]')?.focus({preventScroll:true});
},true);
root.NetworkToolkitCustomize=Object.freeze({orderedIds:list=>tabIds(list),unchanged,openCustomize,moveSection});
})(window);
