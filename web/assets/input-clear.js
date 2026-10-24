/* Input clear controls reuse each field's normal input/change handlers. */
(function(root){
'use strict';
const document=root.document,types=new Set(['text','search','password','url','email','tel','number']),records=new Map(),hosts=new WeakMap();
const legacy={globalTarget:'#clearCurrentTarget',terminalSearchInput:'#terminalSearchClear',macInput:'[data-mac-action="clear"]',possibleFilter:'#possibleClearFilter'};
const glyph='<svg class="input-clear-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18"/></svg>';
let started=false,scheduled=false,needsScan=false,nextID=0,observer,resizeObserver;
function eligible(control){
 if(!control||!['INPUT','TEXTAREA'].includes(control.tagName)||control.disabled||control.readOnly||control.hasAttribute('readonly')||control.matches?.(':disabled'))return false;
 if(control.closest('.xterm,.xterm-helpers,[data-input-clear="off"],[data-output-only]'))return false;
 // A 4-5 digit network listener port must remain fully readable in the compact sidebar.
 if(control.matches?.('[data-transfer-setting="port"],[data-rb-setting="port"],#opt-tftp-port'))return false;
 return control.tagName==='TEXTAREA'||types.has((control.getAttribute('type')||'text').toLowerCase());
}
function name(control){
 const described=control.getAttribute('aria-label')||control.labels?.[0]?.textContent||document.querySelector('label[for="'+control.id+'"]')?.textContent||control.getAttribute('placeholder')||control.id||'field';
 return String(described).replace(/\s+/g,' ').trim().slice(0,90)||'field';
}
function focus(control){if(!control?.isConnected)return;control.focus({preventScroll:true});try{control.setSelectionRange(0,0);}catch(_){}}
function clear(record){
 const control=record.control;if(!eligible(control))return;
 control.value='';focus(control);
 control.dispatchEvent(new root.Event('input',{bubbles:true}));control.dispatchEvent(new root.Event('change',{bubbles:true}));
 control.dispatchEvent(new root.CustomEvent('input-clear',{bubbles:true,detail:{source:'clear-button'}}));
 focus(control.isConnected?control:document.getElementById(control.id));schedule();
}
function parentFor(control){
 const host=control.parentElement;let saved=hosts.get(host);
 if(!saved){saved={count:0,added:root.getComputedStyle(host).position==='static'&&!host.classList.contains('input-clear-host')};hosts.set(host,saved);if(saved.added)host.classList.add('input-clear-host');}
 saved.count++;return host;
}
function releaseHost(host){const saved=hosts.get(host);if(saved&&!--saved.count){if(saved.added)host.classList.remove('input-clear-host');hosts.delete(host);}}
function attach(record){
 if(record.existing)return;
 if(record.host!==record.control.parentElement){if(record.host)releaseHost(record.host);record.host=parentFor(record.control);}
 else if(hosts.get(record.host)?.added)record.host.classList.add('input-clear-host');
 record.control.classList.add('input-clear-enhanced');
 if(record.button.parentElement!==record.host)record.host.append(record.button);
}
function remove(record){
 records.delete(record.control);resizeObserver?.unobserve(record.control);
 record.control.classList.remove('input-clear-enhanced');
 if(record.existing)return;
 record.button.remove();
 if(record.padding)record.control.style.setProperty('padding-right',record.padding,record.priority);else record.control.style.removeProperty('padding-right');
 releaseHost(record.host);
}
function enhance(control){
 const previous=records.get(control);if(!eligible(control)){if(previous)remove(previous);return;}if(previous){if(previous.existing&&!previous.button.isConnected)remove(previous);else{control.classList.add('input-clear-enhanced');attach(previous);return;}}
 if(!control.id){do{control.id='input-clear-field-'+(++nextID);}while(document.getElementById(control.id)!==control);}
 const existing=legacy[control.id]&&document.querySelector(legacy[control.id]),button=existing||document.createElement('button'),existingPosition=existing?root.getComputedStyle(existing).position:'';
 const record={control,button,existing:!!existing,host:null,padding:control.style.getPropertyValue('padding-right'),priority:control.style.getPropertyPriority('padding-right'),basePadding:parseFloat(root.getComputedStyle(control).paddingRight)||8};
 button.classList.add('input-clear-button');button.setAttribute('type','button');button.setAttribute('data-input-clear-for',control.id);
 control.classList.add('input-clear-enhanced');
 if(existing)button.style.position=existingPosition;else button.hidden=true;
 if(!button.querySelector('.input-clear-icon'))button.innerHTML=glyph;
 if(!existing){
  attach(record);
  button.addEventListener('pointerdown',event=>{event.preventDefault();event.stopPropagation();});
  button.addEventListener('click',event=>{event.preventDefault();event.stopPropagation();clear(record);});
 }
 records.set(control,record);resizeObserver?.observe(control);
}
function visible(control){return control.isConnected&&eligible(control)&&!control.closest('[hidden],[inert],.hidden')&&control.getClientRects().length>0;}
function layout(record){
 const {control,button}=record;if(!control.isConnected||!eligible(control)){remove(record);return;}
 if(record.existing&&!button.isConnected){remove(record);enhance(control);return;}attach(record);
 const shown=visible(control)&&String(control.value).length>0;button.hidden=!shown;
 const title=record.existing?(button.getAttribute('aria-label')||button.title||'Clear '+name(control)):'Clear '+name(control);button.setAttribute('aria-label',title);button.title=title;
 if(!button.querySelector('.input-clear-icon'))button.innerHTML=glyph;
 if(record.existing||!shown)return;
 const rect=control.getBoundingClientRect(),host=record.host,box=host.getBoundingClientRect();if(rect.width<48||rect.height<18){button.hidden=true;return;}
 const size=Math.min(22,rect.height-4),type=(control.getAttribute('type')||'text').toLowerCase();let end=type==='password'?24:type==='number'?16:0;
 if(control.tagName==='TEXTAREA')end=Math.max(end,(control.offsetWidth||rect.width)-(control.clientWidth||rect.width));
 for(const other of host.querySelectorAll('button')){if(other===button||other.hidden)continue;const bounds=other.getBoundingClientRect();if(bounds.width&&bounds.left>rect.left+rect.width/2&&bounds.right<=rect.right+2&&bounds.top<rect.bottom&&bounds.bottom>rect.top)end=Math.max(end,rect.right-bounds.left+3);}
 const left=rect.right-box.left-(host.clientLeft||0)+(host.scrollLeft||0)-end-size-3,top=rect.top-box.top-(host.clientTop||0)+(host.scrollTop||0)+(control.tagName==='TEXTAREA'?4:(rect.height-size)/2),padding=record.basePadding+size+end+6;
 for(const [key,value]of Object.entries({left:left+'px',top:top+'px',width:size+'px',height:size+'px'}))if(button.style[key]!==value)button.style[key]=value;
 if(control.style.getPropertyValue('padding-right')!==padding+'px')control.style.setProperty('padding-right',padding+'px','important');
}
function flush(){scheduled=false;if(needsScan){needsScan=false;for(const control of document.querySelectorAll('input,textarea'))enhance(control);}for(const record of [...records.values()])layout(record);}
function schedule(scan=false){needsScan=needsScan||scan;if(scheduled)return;scheduled=true;(root.requestAnimationFrame||root.setTimeout)(flush);}
function refresh(){schedule(true);}
function init(){
 if(started||!document?.body)return;started=true;
 if(typeof root.ResizeObserver==='function')resizeObserver=new root.ResizeObserver(()=>schedule());
 for(const event of ['input','change','focusin','focusout','scroll'])document.addEventListener(event,()=>schedule(),true);
 root.addEventListener?.('resize',()=>schedule());document.addEventListener('transitionend',()=>schedule(),true);
 if(typeof root.MutationObserver==='function'){
  observer=new root.MutationObserver(changes=>{
   let changed=false;for(const mutation of changes){if(mutation.target.closest?.('.input-clear-button'))continue;
    if(mutation.type==='attributes'){if(['INPUT','TEXTAREA','FIELDSET'].includes(mutation.target.tagName)){changed=true;needsScan=true;}else if(mutation.attributeName==='hidden'||mutation.attributeName==='inert')changed=true;}
    else if(mutation.target.closest?.('.xterm'))continue;
    else{for(const added of mutation.addedNodes||[]){if(added.nodeType!==1||added.classList?.contains('input-clear-button'))continue;if(added.matches?.('input,textarea')||added.querySelector?.('input,textarea'))needsScan=true;}changed=true;}
   }if(changed)schedule();
  });observer.observe(document.body,{subtree:true,childList:true,attributes:true,attributeFilter:['disabled','readonly','hidden','inert','type','value']});
 }
 refresh();
}
root.NetworkToolkitInputClear=Object.freeze({init,refresh});
if(document?.readyState==='loading')document.addEventListener('DOMContentLoaded',init,{once:true});else init();
})(globalThis);
