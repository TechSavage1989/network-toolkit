/* Client connection sidebars use the terminal sidebar components. Activity
   dividers share the server controls, including keyboard and reset behavior. */
(function(root){'use strict';
const mounted=new Map();
const number=(value,min,max,fallback)=>Number.isInteger(value)&&value>=min&&value<=max?value:fallback;
function preferences(protocol){
 state.transferClientLayout=state.transferClientLayout||{};
 const value=state.transferClientLayout[protocol]||{};
 const clean={width:number(value.width,260,480,300),collapsed:value.collapsed===true,side:value.side==='right'?'right':'left',
  tab:['saved','recent','settings'].includes(value.tab)?value.tab:'settings',
  activityHeight:number(value.activityHeight,180,1200,0)};
 state.transferClientLayout[protocol]=clean;return clean;
}
function activity(primary,logs,protocol,minimumBrowser=220){
 const old=mounted.get(protocol);old?.disconnect();
 const divider=document.createElement('div');divider.className='transfer-activity-splitter';divider.tabIndex=0;
 logs.id='transferClientActivity-'+protocol;
 divider.setAttribute('role','separator');divider.setAttribute('aria-orientation','horizontal');
 divider.setAttribute('aria-label','Resize files and client activity');divider.setAttribute('aria-controls',logs.id);
 divider.title='Drag up to enlarge activity. Arrow keys resize. Double-click or Home resets.';logs.before(divider);
 function bounds(){
  const css=getComputedStyle(primary),gap=parseFloat(css.rowGap)||0;
  const fixed=[...primary.children].filter(el=>el!==logs&&el!==divider&&!el.classList.contains('rb-explorer')&&!el.classList.contains('transfer-client-content'));
  const used=fixed.reduce((sum,el)=>sum+el.offsetHeight,0);
  return {min:180,max:Math.max(180,Math.min(1200,Math.floor(primary.clientHeight-used-minimumBrowser-divider.offsetHeight-gap*(primary.children.length-1)-(parseFloat(css.paddingTop)||0)-(parseFloat(css.paddingBottom)||0))))};
 }
 function apply(){const limits=bounds(),wanted=preferences(protocol).activityHeight||260,height=Math.max(limits.min,Math.min(limits.max,wanted));
  primary.style.setProperty('--transfer-activity-height',height+'px');
  for(const [key,value] of Object.entries({valuemin:limits.min,valuemax:limits.max,valuenow:height,valuetext:height+' pixels for client activity'}))divider.setAttribute('aria-'+key,String(value));
 }
 let pointer,startY,startHeight;
 divider.addEventListener('pointerdown',event=>{if(event.button!==0)return;event.preventDefault();pointer=event.pointerId;startY=event.clientY;startHeight=logs.getBoundingClientRect().height;divider.setPointerCapture(pointer);divider.classList.add('is-resizing');root.NetworkToolkitNativeDrag?.cancelHover?.();});
 divider.addEventListener('pointermove',event=>{if(pointer!==event.pointerId)return;const limits=bounds();preferences(protocol).activityHeight=Math.max(limits.min,Math.min(limits.max,Math.round(startHeight+startY-event.clientY)));apply();});
 const finish=event=>{if(pointer!==event.pointerId)return;pointer=undefined;divider.classList.remove('is-resizing');saveState();};
 for(const event of ['pointerup','pointercancel','lostpointercapture'])divider.addEventListener(event,finish);
 divider.addEventListener('dblclick',()=>{preferences(protocol).activityHeight=0;apply();saveState();});
 divider.addEventListener('keydown',event=>{if(!['ArrowUp','ArrowDown','Home','End'].includes(event.key))return;event.preventDefault();const limits=bounds(),current=Number(divider.getAttribute('aria-valuenow'));preferences(protocol).activityHeight=event.key==='Home'?0:event.key==='End'?limits.max:Math.max(limits.min,Math.min(limits.max,current+(event.key==='ArrowUp'?1:-1)*(event.shiftKey?40:12)));apply();saveState();});
 const observer=typeof ResizeObserver==='function'?new ResizeObserver(()=>{if(!primary.isConnected){observer.disconnect();return;}apply();}):null;
 observer?.observe(primary);for(const el of primary.children)if(el!==logs&&el!==divider)observer?.observe(el);
 mounted.set(protocol,{disconnect:()=>observer?.disconnect()});apply();return {apply};
}
function mount(k,{app,settings,library,libraryHTML}){
 const protocol=k==='ssh'?'sftp':k,pref=preferences(protocol);
 const primary=document.createElement('main');primary.className='rb-client-primary';
 const head=settings.querySelector('.rb-connect-head'),notice=settings.querySelector('[data-rb-feedback]');
 if(head)primary.append(head);if(notice)primary.append(notice);
 const logs=app.querySelector('.rb-activity');for(const child of [...app.children])if(child!==settings)primary.append(child);
 library.remove();
 const aside=document.createElement('aside');aside.className='terminal-session-sidebar rb-client-sidebar';aside.setAttribute('aria-label',protocol.toUpperCase()+' connections and settings');
 aside.innerHTML='<div class="terminal-sidebar-rail"><div class="terminal-sidebar-tabs" role="tablist" aria-orientation="vertical" aria-label="Client connection sidebar">'+[['settings','Connect','settings'],['saved','Saved','bookmark'],['recent','Recent','clock']].map(([id,label,symbol])=>'<button type="button" class="terminal-sidebar-tab" role="tab" data-client-tab="'+id+'" aria-controls="transferClientSidebarBody-'+protocol+'">'+icon(symbol)+'<span>'+label+'</span></button>').join('')+'</div></div><div class="terminal-sidebar-content" id="transferClientSidebarBody-'+protocol+'"><div class="session-library-heading"><strong data-client-title></strong><button type="button" class="iconbtn terminal-sidebar-side-button" data-client-action="swap-side" title="Move sidebar to other side" aria-label="Move connection sidebar to other side">'+icon('arrowSwap')+'</button></div><div class="rb-sidebar-page" data-client-settings></div><div class="rb-sidebar-library" data-client-library></div></div>';
 aside.querySelector('[data-client-settings]').append(settings);aside.querySelector('[data-client-library]').append(library);
 const splitter=document.createElement('div');splitter.className='rb-client-sidebar-divider';splitter.tabIndex=0;splitter.setAttribute('role','separator');splitter.setAttribute('aria-label','Resize or hide connection sidebar');splitter.setAttribute('aria-orientation','vertical');splitter.title='Drag to resize. Double-click or Enter hides or shows. Arrow keys resize; Home resets.';
 app.classList.add('rb-client-layout');app.replaceChildren(aside,splitter,primary);
 function widthLimits(){const maximum=Math.max(44,Math.min(480,app.clientWidth-(matchMedia('(max-width:800px)').matches?30:250)));return {minimum:Math.min(260,maximum),maximum};}
 function apply(){const current=preferences(protocol),{minimum,maximum}=widthLimits();
  const width=current.collapsed?44:Math.max(minimum,Math.min(maximum,current.width));
  app.classList.toggle('rb-sidebar-right',current.side==='right');app.style.setProperty('--transfer-sidebar-width',width+'px');
  const sideButton=aside.querySelector('[data-client-action="swap-side"]'),sideLabel='Move '+protocol.toUpperCase()+' sidebar to '+(current.side==='right'?'left':'right');sideButton.title=sideLabel;sideButton.setAttribute('aria-label',sideLabel);
  aside.style.flexBasis=width+'px';aside.classList.toggle('is-collapsed',current.collapsed);app.classList.toggle('rb-sidebar-collapsed',current.collapsed);
  aside.querySelector('.terminal-sidebar-content').hidden=current.collapsed;
  aside.querySelector('.terminal-sidebar-content').inert=current.collapsed;
  aside.querySelector('[data-client-settings]').hidden=current.tab!=='settings';aside.querySelector('[data-client-library]').hidden=current.tab==='settings';
  aside.querySelector('[data-client-title]').textContent=current.tab==='settings'?'Connection settings':current.tab==='saved'?'Saved connections':'Recent connections';
  for(const button of aside.querySelectorAll('[data-client-tab]')){button.setAttribute('aria-selected',String(button.dataset.clientTab===current.tab));button.setAttribute('aria-expanded',String(button.dataset.clientTab===current.tab&&!current.collapsed));button.title=(current.collapsed||button.dataset.clientTab!==current.tab?'Open ':'Close ')+(button.dataset.clientTab==='settings'?'connection settings':button.dataset.clientTab+' connections');button.tabIndex=button.dataset.clientTab===current.tab?0:-1;}
  splitter.setAttribute('aria-valuemin',String(minimum));splitter.setAttribute('aria-valuemax',String(maximum));splitter.setAttribute('aria-valuenow',String(width));
 }
 function select(tab,toggle=false){const current=preferences(protocol);current.collapsed=toggle&&current.tab===tab?!current.collapsed:false;current.tab=tab;
  if(tab!=='settings')aside.querySelector('[data-client-library]').innerHTML=libraryHTML(tab);
  apply();saveState();
 }
 for(const button of aside.querySelectorAll('[data-client-tab]'))button.addEventListener('click',event=>{event.stopPropagation();select(button.dataset.clientTab,true);});
 aside.querySelector('.terminal-sidebar-tabs').addEventListener('keydown',event=>{if(!['ArrowUp','ArrowDown','Home','End'].includes(event.key))return;event.preventDefault();const tabs=['settings','saved','recent'],index=tabs.indexOf(preferences(protocol).tab),next=event.key==='Home'?0:event.key==='End'?2:(index+(event.key==='ArrowDown'?1:2))%3;select(tabs[next]);aside.querySelector('[data-client-tab="'+tabs[next]+'"]').focus();});
 aside.querySelector('[data-client-action="swap-side"]').addEventListener('click',event=>{event.stopPropagation();const current=preferences(protocol);current.side=current.side==='left'?'right':'left';apply();saveState();});
 function toggle(){const current=preferences(protocol);current.collapsed=!current.collapsed;apply();saveState();}
 splitter.addEventListener('dblclick',toggle);
 splitter.addEventListener('keydown',event=>{if(['Enter',' '].includes(event.key)){event.preventDefault();toggle();return;}if(!['ArrowLeft','ArrowRight','Home'].includes(event.key))return;event.preventDefault();const current=preferences(protocol);current.collapsed=false;current.width=event.key==='Home'?300:Math.max(260,Math.min(480,current.width+(event.key==='ArrowRight'?1:-1)*(current.side==='left'?1:-1)*(event.shiftKey?40:12)));apply();saveState();});
 let pointer,startX,startWidth;
 splitter.addEventListener('pointerdown',event=>{if(event.button!==0||preferences(protocol).collapsed)return;event.preventDefault();pointer=event.pointerId;startX=event.clientX;startWidth=aside.getBoundingClientRect().width;splitter.setPointerCapture(pointer);splitter.classList.add('dragging');});
 splitter.addEventListener('pointermove',event=>{if(pointer!==event.pointerId)return;const current=preferences(protocol);current.width=Math.max(260,Math.min(480,Math.round(startWidth+(current.side==='left'?1:-1)*(event.clientX-startX))));apply();});
 const finish=event=>{if(pointer!==event.pointerId)return;pointer=undefined;splitter.classList.remove('dragging');saveState();};
 for(const event of ['pointerup','pointercancel','lostpointercapture'])splitter.addEventListener(event,finish);
 if(pref.tab!=='settings')aside.querySelector('[data-client-library]').innerHTML=libraryHTML(pref.tab);
 apply();const size=activity(primary,logs,protocol);
 const observer=typeof ResizeObserver==='function'?new ResizeObserver(()=>{if(!app.isConnected){observer.disconnect();return;}apply();size.apply();}):null;observer?.observe(app);
 app.clientSidebarSelect=select;
}
root.NetworkToolkitTransferClientLayout=Object.freeze({preferences,mount,activity});
})(globalThis);
