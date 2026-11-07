/* Dropdowns that match the app (owner request, v1.21.10).
   1. Select lists: where the engine supports customizable selects, an opened
      select shows its list in the app's light or dark colors (dropdowns.css)
      instead of the system's black popup. Closed selects are untouched.
   2. Suggestion lists for text fields:
   Any <input list="…"> becomes a combobox: the browser's own suggestion popup
   (black on Windows, filtered by what is typed) is replaced by a list in the
   app's colors that always shows every choice. Typing moves to the best
   match instead of hiding the rest. Fields marked data-combobox-font show
   each choice in its own font. The field stays a normal text input, so any
   value can still be typed and the page's input and change handlers run. */
(function(root){
'use strict';
const document=root.document;
const chevron='<svg viewBox="0 0 12 12" aria-hidden="true"><path d="M2.5 4.5 6 8l3.5-3.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>';
const records=new Map();
let popup=null,current=null,sequence=0,started=false,observer=null;
function listFor(input){const id=input.dataset.comboboxList;return id?document.getElementById(id):null;}
function choices(input){
 const list=listFor(input);if(!list)return [];
 return Array.from(list.querySelectorAll('option')).map(option=>{const value=option.getAttribute('value')??option.textContent??'',label=option.getAttribute('label')||'';return {value:String(value),label:label&&label!==value?label:''};}).filter(choice=>choice.value);
}
function fontFamily(name){return /^(?:monospace|sans-serif|serif|system-ui|ui-monospace)$/.test(name)?name:'"'+String(name).replace(/["\\]/g,'')+'", monospace';}
function ensurePopup(){
 if(popup?.isConnected)return popup;
 popup=document.createElement('div');popup.className='combobox-popup';popup.id='comboboxPopup';popup.setAttribute('role','listbox');popup.hidden=true;
 // The list sits at the page level, outside the panel or dialog that holds the field;
 // keep its clicks from reaching "click outside closes the panel" handlers.
 popup.addEventListener('pointerdown',event=>{event.preventDefault();event.stopPropagation();});
 popup.addEventListener('mousedown',event=>{event.preventDefault();event.stopPropagation();});
 popup.addEventListener('click',event=>{event.stopPropagation();const option=event.target.closest?.('[role="option"]');if(option&&current)choose(current,Number(option.dataset.index));});
 popup.addEventListener('pointermove',event=>{const option=event.target.closest?.('[role="option"]');if(option&&current&&Number(option.dataset.index)!==current.active)setActive(current,Number(option.dataset.index),false);});
 document.body.append(popup);return popup;
}
// Best match for typed text: exact, then starts with, then contains (case-insensitive).
function bestMatch(items,text){
 const query=String(text||'').trim().toLowerCase();if(!query)return -1;
 for(const test of [v=>v===query,v=>v.startsWith(query),v=>v.includes(query)]){const index=items.findIndex(item=>test(item.value.toLowerCase()));if(index>=0)return index;}
 return -1;
}
function render(state){
 const box=ensurePopup(),font=state.input.hasAttribute('data-combobox-font'),value=state.input.value.trim().toLowerCase();
 box.innerHTML='';box.setAttribute('aria-label',state.input.getAttribute('aria-label')||state.input.labels?.[0]?.textContent||'Choices');
 if(!state.items.length){const empty=document.createElement('div');empty.className='combobox-empty';empty.textContent='No suggestions available';box.append(empty);return;}
 state.items.forEach((item,index)=>{
  const option=document.createElement('div');option.className='combobox-option'+(font?' combobox-font':'');option.id='comboboxOption'+state.id+'-'+index;option.dataset.index=String(index);option.setAttribute('role','option');
  option.setAttribute('aria-selected',String(item.value.toLowerCase()===value));
  const name=document.createElement('span');name.className='combobox-value';name.textContent=item.value;if(font)name.style.fontFamily=fontFamily(item.value);option.append(name);
  if(item.label){const note=document.createElement('span');note.className='combobox-label';note.textContent=item.label;option.append(note);}
  box.append(option);
 });
}
function place(state){
 if(!popup||popup.hidden)return;
 const rect=state.input.getBoundingClientRect(),view=root.innerHeight||document.documentElement.clientHeight,width=Math.max(rect.width,state.input.hasAttribute('data-combobox-font')?320:180);
 popup.style.width=width+'px';popup.style.left=Math.max(4,Math.min(rect.left,(root.innerWidth||width)-width-4))+'px';
 const below=view-rect.bottom-8,above=rect.top-8,height=Math.min(popup.scrollHeight,340);
 if(below<Math.min(height,160)&&above>below){popup.style.top='';popup.style.bottom=(view-rect.top+4)+'px';popup.style.maxHeight=Math.max(120,Math.min(340,above))+'px';}
 else{popup.style.bottom='';popup.style.top=(rect.bottom+4)+'px';popup.style.maxHeight=Math.max(120,Math.min(340,below))+'px';}
}
function setActive(state,index,scroll=true){
 state.active=index;
 for(const option of popup.querySelectorAll('[role="option"]'))option.classList.toggle('active',Number(option.dataset.index)===index);
 const option=index>=0?popup.querySelector('#comboboxOption'+state.id+'-'+index):null;
 if(option){state.input.setAttribute('aria-activedescendant',option.id);if(scroll)option.scrollIntoView?.({block:'nearest'});}else state.input.removeAttribute('aria-activedescendant');
}
function open(input,{fromTyping=false}={}){
 const record=records.get(input);if(!record||input.disabled||input.readOnly)return;
 if(current&&current.input!==input)close();
 const items=choices(input),state=current?.input===input?current:{input,id:record.id,items,active:-1};
 state.items=items;current=state;render(state);
 popup.hidden=false;input.setAttribute('aria-expanded','true');record.toggle?.setAttribute('aria-expanded','true');
 place(state);
 const match=bestMatch(items,input.value);setActive(state,match>=0?match:fromTyping?-1:Math.max(-1,items.findIndex(item=>item.value===input.value)));
}
function close(){
 if(!current)return;const record=records.get(current.input);
 current.input.setAttribute('aria-expanded','false');current.input.removeAttribute('aria-activedescendant');record?.toggle?.setAttribute('aria-expanded','false');
 current=null;if(popup)popup.hidden=true;
}
function choose(state,index){
 const item=state.items[index];if(!item)return;
 const input=state.input;close();input.value=item.value;
 input.dispatchEvent(new root.Event('input',{bubbles:true}));input.dispatchEvent(new root.Event('change',{bubbles:true}));
 input.focus({preventScroll:true});
}
function keydown(event){
 const input=event.target;if(!records.has(input))return;
 const isOpen=current?.input===input;
 if(event.key==='ArrowDown'||event.key==='ArrowUp'){
  event.preventDefault();if(!isOpen){open(input);return;}if(event.altKey){close();return;}
  const count=current.items.length;if(!count)return;const step=event.key==='ArrowDown'?1:-1;setActive(current,current.active<0?(step>0?0:count-1):(current.active+step+count)%count);return;
 }
 if(!isOpen)return;
 if(event.key==='Enter'){if(current.active>=0){event.preventDefault();event.stopPropagation();choose(current,current.active);}else close();return;}
 if(event.key==='Escape'){event.preventDefault();event.stopPropagation();close();return;}
 if(event.key==='Tab')close();
 if(event.key==='Home'||event.key==='End'){if(event.ctrlKey){event.preventDefault();setActive(current,event.key==='Home'?0:current.items.length-1);}}
}
function enhance(input){
 if(records.has(input)||!input.isConnected)return;
 const listId=input.getAttribute('list');if(!listId)return;
 // Removing list= stops the browser's own suggestion popup; the list element stays in place.
 input.dataset.comboboxList=listId;input.removeAttribute('list');
 const id=++sequence;input.setAttribute('role','combobox');input.setAttribute('aria-autocomplete','list');input.setAttribute('aria-expanded','false');input.setAttribute('aria-controls','comboboxPopup');
 const host=input.parentElement;if(host&&root.getComputedStyle(host).position==='static')host.classList.add('combobox-host');
 const toggle=document.createElement('button');toggle.type='button';toggle.className='combobox-toggle';toggle.tabIndex=-1;toggle.innerHTML=chevron;toggle.setAttribute('aria-label','Show all choices');toggle.setAttribute('aria-expanded','false');toggle.title='Show all choices';
 toggle.addEventListener('pointerdown',event=>event.preventDefault());
 toggle.addEventListener('click',event=>{event.preventDefault();event.stopPropagation();if(current?.input===input)close();else{input.focus({preventScroll:true});open(input);}});
 input.after(toggle);
 records.set(input,{id,toggle,basePadding:parseFloat(root.getComputedStyle(input).paddingRight)||0});layout(input);
}
function layout(input){
 const record=records.get(input);if(!record)return;
 if(!input.isConnected){record.toggle.remove();records.delete(input);return;}
 const rect=input.getBoundingClientRect(),box=input.parentElement?.getBoundingClientRect();
 record.toggle.hidden=!rect.width||input.disabled||input.readOnly;if(!box||record.toggle.hidden)return;
 const size=22,left=(rect.right-box.left-(input.parentElement.clientLeft||0)-size-5)+'px',top=(rect.top-box.top-(input.parentElement.clientTop||0)+(rect.height-size)/2)+'px';
 if(record.toggle.style.left!==left)record.toggle.style.left=left;if(record.toggle.style.top!==top)record.toggle.style.top=top;
 // input-clear.js adds room for its own button and any button at the field's right edge.
 const padding=record.basePadding+size+4;if(parseFloat(input.style.paddingRight||'0')<padding)input.style.paddingRight=padding+'px';
}
let enhanceWatch=null;
const BASE_SELECT=!!root.CSS?.supports?.('appearance','base-select');
// Selects stay native while closed, so their size and layout never change.
// Opening one switches it to the app-styled list (class ntk-picker) with its
// box locked to the exact native size, and switches back once it closes.
function pickerEligible(select){return BASE_SELECT&&select?.tagName==='SELECT'&&!select.multiple&&!(select.size>1)&&!select.disabled&&!select.closest('[data-native-select]');}
function restorePicker(select){
 if(!select.classList.contains('ntk-picker'))return;
 select.classList.remove('ntk-picker');for(const name of ['width','height','min-width','max-width','min-height','max-height','flex']){const key='ntkLock'+name.replace('-','');if(select.dataset[key]!=='keep')select.style.removeProperty(name);delete select.dataset[key];}
}
function preparePicker(select){
 if(!pickerEligible(select)||select.classList.contains('ntk-picker'))return false;
 const rect=select.getBoundingClientRect();if(!rect.width||!rect.height)return false;
 for(const name of ['width','height','min-width','max-width','min-height','max-height','flex'])if(select.style.getPropertyValue(name))select.dataset['ntkLock'+name.replace('-','')]='keep';
 for(const [name,value] of [['width',rect.width+'px'],['min-width',rect.width+'px'],['max-width',rect.width+'px'],['height',rect.height+'px'],['min-height',rect.height+'px'],['max-height',rect.height+'px'],['flex','none']])if(select.dataset['ntkLock'+name.replace('-','')]!=='keep')select.style.setProperty(name,value,'important');
 select.classList.add('ntk-picker');void root.getComputedStyle(select).appearance;
 let opened=false,frames=0;
 const watch=()=>{if(!select.isConnected){restorePicker(select);return;}if(select.matches(':open')){opened=true;frames=0;}else if(opened||++frames>30){restorePicker(select);return;}(root.requestAnimationFrame||setTimeout)(watch);};
 (root.requestAnimationFrame||setTimeout)(watch);return true;
}
// A native list keeps the keyboard to itself while it is open; this one is part
// of the page, so app shortcuts (Escape closing a panel, pane keys) are held back
// until it closes. The list still handles every key itself.
function openPicker(){try{return document.querySelector('select.ntk-picker:open');}catch{return null;}}
function isolateKeys(event){if(openPicker())event.stopImmediatePropagation();}
function pickerPointer(event){if(event.button===0&&!event.ctrlKey&&!event.metaKey&&event.target?.tagName==='SELECT')preparePicker(event.target);}
function pickerKey(event){
 const select=event.target;if(select?.tagName!=='SELECT'||!pickerEligible(select)||select.classList.contains('ntk-picker'))return;
 if(!((event.altKey&&(event.key==='ArrowDown'||event.key==='ArrowUp'))||event.key==='F4'))return;
 if(!preparePicker(select))return;event.preventDefault();try{select.showPicker();}catch{restorePicker(select);}
}
function scan(){for(const input of document.querySelectorAll('input[list]'))(enhanceWatch||enhance)(input);for(const input of records.keys())layout(input);}
function init(){
 if(started||!document?.body)return;started=true;
 document.addEventListener('keydown',keydown,true);
 document.addEventListener('pointerdown',pickerPointer,true);document.addEventListener('keydown',pickerKey,true);
 for(const type of ['keydown','keyup','keypress'])root.addEventListener(type,isolateKeys,true);
 // The list opens from its arrow button or Down/Up, never by itself while typing or
 // clicking in the field: an open list must not cover the dialog's own buttons.
 // While it is open, typing moves to the best match.
 document.addEventListener('input',event=>{if(records.has(event.target)&&current?.input===event.target)open(event.target,{fromTyping:true});},true);
 document.addEventListener('pointerdown',event=>{if(current&&event.target!==current.input&&!popup?.contains(event.target)&&!records.get(current.input)?.toggle.contains(event.target))close();},true);
 document.addEventListener('focusout',event=>{if(current&&event.target===current.input)setTimeout(()=>{if(current&&document.activeElement!==current.input)close();},0);},true);
 document.addEventListener('scroll',event=>{if(current&&!popup.contains(event.target))place(current);},true);
 root.addEventListener('resize',()=>{if(current)place(current);for(const input of records.keys())layout(input);});
 if(typeof root.MutationObserver==='function'){let queued=false;// Terminal output changes the DOM constantly and never contains dropdowns.
  const ignored=mutation=>{const node=mutation.target.nodeType===1?mutation.target:mutation.target.parentElement;return !!node?.closest?.('.xterm,.combobox-popup,.terminal-view');};
  observer=new root.MutationObserver(changes=>{if(changes.every(ignored))return;if(queued)return;queued=true;(root.requestAnimationFrame||setTimeout)(()=>{queued=false;scan();if(current&&!current.input.isConnected)close();});});observer.observe(document.body,{childList:true,subtree:true,characterData:true,attributes:true,attributeFilter:['list','class','hidden','open']});}
 if(typeof root.ResizeObserver==='function'){const resized=new root.ResizeObserver(entries=>{for(const entry of entries)layout(entry.target);});const watch=enhance;enhanceWatch=input=>{watch(input);if(records.has(input))resized.observe(input);};}
 scan();
}
if(document?.readyState==='loading')document.addEventListener('DOMContentLoaded',init,{once:true});else init();
root.NetworkToolkitDropdowns=Object.freeze({init,scan,open,close,choices,bestMatch,preparePicker,restorePicker,baseSelect:BASE_SELECT,isOpen:input=>current?.input===input});
})(typeof window!=='undefined'?window:globalThis);
