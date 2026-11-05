/* Scrollable tab strips (owner request, v1.20.13).
 * The Terminal session strip (#terminalTabs) and every Text Editor document
 * strip ([data-ed-tabs]) scroll sideways when their tabs do not fit:
 * - the mouse wheel over the strip scrolls it left and right (vertical wheel
 *   movement is turned into horizontal scrolling; a horizontal wheel or
 *   touchpad swipe works as is);
 * - a pair of quiet chevron buttons sits at the end of the strip, pinned to the
 *   right edge while the tabs scroll underneath, and appears only when the
 *   tabs overflow. Each press scrolls by most of the visible width; a button is
 *   disabled at its end.
 * The strips are re-rendered by their owners; renderTerminalChrome and the
 * editor's drawTabs call attach() afterwards, which (re)appends the controls
 * as the last child and refreshes their state. Tabs, ids, drag and keyboard
 * behaviour are untouched; the buttons are not tab stops (keyboard users move
 * between tabs with the arrow keys, which already bring the tab into view). */
(function(root){
'use strict';
const document=root.document;if(!document)return;
const LINE=40,STEP=0.7,EDGE=1;
const wired=new WeakSet(),selected=new WeakMap();
const chevron=dir=>'<svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true" focusable="false"><path d="'+(dir==='left'?'M14.5 6 8.5 12l6 6':'M9.5 6l6 6-6 6')+'" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
function controlsOf(strip){return strip?.querySelector?.(':scope>[data-tab-scroll-controls]')||null;}
function reduced(){try{return root.matchMedia?.('(prefers-reduced-motion: reduce)').matches===true;}catch{return false;}}
function controlsWidth(strip){const c=controlsOf(strip);if(!c||c.hidden)return 0;const gap=parseFloat(root.getComputedStyle?.(strip).columnGap)||0;return c.offsetWidth+gap;}
// Overflow is measured without the controls, so showing them cannot keep them shown.
function overflowing(strip){return strip.scrollWidth-controlsWidth(strip)-strip.clientWidth>EDGE;}
function maxScroll(strip){return Math.max(0,strip.scrollWidth-strip.clientWidth);}
function update(strip){const c=controlsOf(strip);if(!c)return;const over=overflowing(strip);if(c.hidden===over)c.hidden=!over;strip.classList.toggle('tab-strip-overflow',over);if(!over)return;
 const left=c.querySelector('[data-tab-scroll="left"]'),right=c.querySelector('[data-tab-scroll="right"]'),at=strip.scrollLeft,max=maxScroll(strip);
 left.disabled=at<=EDGE;right.disabled=at>=max-EDGE;}
function ensureControls(strip){let c=controlsOf(strip);
 if(!c){c=document.createElement('div');c.className='tab-scroll-controls';c.dataset.tabScrollControls='';c.hidden=true;c.setAttribute('role','presentation');
  c.innerHTML='<button type="button" class="tab-scroll-button" data-tab-scroll="left" tabindex="-1" aria-label="Scroll tabs left" title="Scroll tabs left">'+chevron('left')+'</button><button type="button" class="tab-scroll-button" data-tab-scroll="right" tabindex="-1" aria-label="Scroll tabs right" title="Scroll tabs right">'+chevron('right')+'</button>';}
 if(strip.lastElementChild!==c)strip.append(c);return c;}
function scrollBy(strip,amount){const behavior=reduced()?'auto':'smooth';if(typeof strip.scrollBy==='function')strip.scrollBy({left:amount,behavior});else strip.scrollLeft+=amount;}
// Keep a newly selected tab clear of the pinned controls.
function revealSelected(strip){const tab=strip.querySelector('[role="tab"][aria-selected="true"]');const key=tab?(tab.dataset.terminalTab||tab.dataset.edOpen||tab.id||''):'';if(selected.get(strip)===key)return;selected.set(strip,key);if(!tab||!overflowing(strip))return;
 const row=tab.closest('.terminal-tab,.nt-editor-tab')||tab,area=strip.getBoundingClientRect(),item=row.getBoundingClientRect(),c=controlsOf(strip),right=c&&!c.hidden?Math.min(area.right,c.getBoundingClientRect().left):area.right;
 if(item.left<area.left)strip.scrollLeft+=item.left-area.left;else if(item.right>right)strip.scrollLeft+=item.right-right;}
function wheel(event){const strip=event.currentTarget;if(event.ctrlKey||!overflowing(strip))return;
 const raw=Math.abs(event.deltaX)>Math.abs(event.deltaY)?event.deltaX:event.deltaY;if(!raw)return;
 const delta=raw*(event.deltaMode===1?LINE:event.deltaMode===2?strip.clientWidth:1),max=maxScroll(strip);
 if(delta<0&&strip.scrollLeft<=EDGE||delta>0&&strip.scrollLeft>=max-EDGE)return;
 event.preventDefault();strip.scrollLeft=Math.min(max,Math.max(0,strip.scrollLeft+delta));update(strip);}
function wire(strip){if(wired.has(strip))return;wired.add(strip);
 strip.addEventListener('wheel',wheel,{passive:false});
 strip.addEventListener('scroll',()=>update(strip),{passive:true});
 strip.addEventListener('click',event=>{const button=event.target.closest?.('[data-tab-scroll]');if(!button||controlsOf(strip)!==button.parentElement)return;event.preventDefault();event.stopPropagation();if(button.disabled)return;
  const visible=Math.max(LINE,strip.clientWidth-controlsWidth(strip));scrollBy(strip,(button.dataset.tabScroll==='left'?-1:1)*Math.round(visible*STEP));});
 // Pressing a scroll button does not move focus away from the tab or console.
 strip.addEventListener('pointerdown',event=>{if(event.target.closest?.('[data-tab-scroll-controls]')&&event.pointerType!=='touch')event.preventDefault();},true);
 strip.addEventListener('dragstart',event=>{if(event.target.closest?.('[data-tab-scroll-controls]'))event.preventDefault();},true);
 if(typeof root.ResizeObserver==='function')new root.ResizeObserver(()=>update(strip)).observe(strip);}
function attach(strip){if(!strip||!strip.isConnected)return;ensureControls(strip);wire(strip);update(strip);revealSelected(strip);}
function refreshAll(){for(const strip of document.querySelectorAll('#terminalTabs,[data-ed-tabs]'))attach(strip);}
root.addEventListener?.('resize',()=>{for(const strip of document.querySelectorAll('#terminalTabs,[data-ed-tabs]'))update(strip);});
root.NetworkToolkitTabScroll=Object.freeze({attach,update,refreshAll,overflowing});
})(window);
