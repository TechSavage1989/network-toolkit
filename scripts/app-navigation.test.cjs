'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
function harness({small=false,collapsed=false}={}){
 const document=makeDocument('<aside id="sidebar"><div class="brand"><div class="brand-mark"><img></div><div class="nav-brand-copy"><div class="brand-name">Network Toolkit</div></div></div><div class="nav-scroll"><div class="nav-label">Tools</div><button class="nav-item active" data-page="overview" aria-current="page"><svg></svg><span>Overview</span></button><button class="nav-item" data-page="ping"><svg></svg><span>Ping</span></button></div><div class="sidebar-foot"><div id="engineLabel"><span>Ready</span></div><div id="engineSub">Portable edition</div></div></aside><main class="main"><div id="topbar"><button id="mobileMenu" data-action="toggle-nav"></button><span>Workspace</span></div><div id="workspaceScroll"></div></main>');document.body=document;
 let clock=0;
 const state={navigationCollapsed:collapsed,activePage:'overview'},calls=[],frames=[],media={matches:small,addEventListener(type,callback){this.change=callback;}},c={document,state,console,Date:{now:()=>clock},TERMINAL_ONLY:false,MutationObserver:class{observe(){}},matchMedia:()=>media,requestAnimationFrame:callback=>frames.push(callback),saveState:()=>calls.push('save'),closeEntryContextMenu(){},titleFor:page=>page,NetworkToolkitWorkspace:{layoutDetails(){}},applyToolLayout(){},scheduleTerminalFit(){}};
 c.window=c;vm.createContext(c);
 for(const file of ['panel-controls.js','app-navigation.js'])vm.runInContext(fs.readFileSync(require.resolve('../web/assets/'+file),'utf8'),c);
 function all(owner){return owner.children.flatMap(child=>[child,...all(child)]);}
 function focusable(){for(const element of all(document)){element.focus=()=>{document.activeElement=element;};element.scrollIntoView=()=>{};element.getClientRects=()=>[{}];}}
 function flush(){while(frames.length)frames.shift()();}
 const api=c.NetworkToolkitNavigation;api.init();focusable();
 const sidebar=document.getElementById('sidebar'),toggle=document.getElementById('mobileMenu'),brand=sidebar.querySelector('.brand'),main=document.querySelector('.main'),drawerClose=document.getElementById('sidebarDrawerClose');
 function click(target,button=0,advance=600){
  clock+=advance;
  if(target.disabled)return;
  const event={target,button,defaultPrevented:false,stopped:false,immediate:false,preventDefault(){this.defaultPrevented=true;},stopPropagation(){this.stopped=true;},stopImmediatePropagation(){this.stopped=this.immediate=true;}};
  for(const callback of document.listeners.click||[]){callback(event);if(event.immediate)break;}
  if(!event.stopped)for(const callback of target.listeners.click||[]){callback(event);if(event.immediate)break;}
  if(!event.stopped&&sidebar.contains(target))for(const callback of sidebar.listeners.click||[]){callback(event);if(event.immediate)break;}
  if(!event.stopped&&button===0){const action=target.closest('[data-action="toggle-nav"]'),page=target.closest('[data-page]');if(action)api.toggle();else if(page){state.activePage=page.dataset.page;calls.push('page:'+page.dataset.page);}}
  flush();
 }
 toggle.click=()=>click(toggle);drawerClose.click=()=>click(drawerClose);flush();
 return {api,c,document,state,calls,sidebar,toggle,brand,main,drawerClose,click,flush,resize(value){media.matches=value;media.change();flush();}};
}
test('single desktop clicks preserve the rail until an explicit panel action',()=>{
 const h=harness(),mark=h.sidebar.querySelector('.brand-mark'),divider=h.document.getElementById('appNavigationDivider');
 assert.equal(h.brand.firstElementChild,h.toggle);assert.equal(h.document.querySelectorAll('#mobileMenu').length,1);assert.equal(h.toggle.getAttribute('aria-label'),'Hide navigation');
 h.click(h.toggle.querySelector('svg'));assert(h.state.navigationCollapsed);assert(!h.sidebar.classList.contains('panel-reveal-surface'));assert.equal(divider.getAttribute('aria-valuenow'),'0');
 for(let i=0;i<5;i++)h.api.sync();assert.equal(h.sidebar.listeners.click.length,1);
 for(const el of [mark.querySelector('img'),h.sidebar,h.sidebar.querySelector('.sidebar-foot'),divider])h.click(el);assert(h.state.navigationCollapsed);assert.equal(h.calls.filter(x=>x==='save').length,1);
 divider.emit('dblclick',divider);assert(!h.state.navigationCollapsed);assert.equal(divider.getAttribute('aria-valuenow'),'1');assert.equal(h.calls.filter(x=>x==='save').length,2);
 divider.emit('keydown',divider,{key:'Enter'});assert(h.state.navigationCollapsed);divider.emit('keydown',divider,{key:' '});assert(!h.state.navigationCollapsed);assert.equal(h.calls.filter(x=>x==='save').length,4);
});
test('collapsed navigation tool icons select pages while keeping the rail collapsed',()=>{
 const h=harness({collapsed:true}),ping=h.sidebar.querySelector('[data-page="ping"]'),overview=h.sidebar.querySelector('[data-page="overview"]');
 h.click(ping.querySelector('svg'));assert(h.state.navigationCollapsed);assert.equal(h.state.activePage,'ping');assert.equal(h.document.activeElement,ping);assert.equal(h.calls.filter(x=>x==='save').length,0);assert.equal(h.calls.filter(x=>x==='page:ping').length,1);
 h.click(ping);h.click(overview);assert(h.state.navigationCollapsed);assert.equal(h.state.activePage,'overview');assert.equal(h.document.activeElement,overview);assert.equal(h.calls.filter(x=>x==='save').length,0);
 h.click(h.toggle);assert(!h.state.navigationCollapsed);h.click(ping);assert(!h.state.navigationCollapsed);assert.equal(h.calls.filter(x=>x==='save').length,1);
});
test('mobile navigation retains its main opener and a leading drawer close with inert and focus restoration',()=>{
 const h=harness({small:true}),topbar=h.document.getElementById('topbar');
 assert.equal(h.toggle.parentElement,topbar);assert.equal(h.brand.firstElementChild,h.drawerClose);assert(h.sidebar.inert);assert.equal(h.toggle.getAttribute('aria-label'),'Show navigation');
 h.click(h.toggle);assert(h.sidebar.classList.contains('open'));assert(h.main.inert);assert(!h.sidebar.inert);assert.equal(h.document.activeElement,h.sidebar.querySelector('[data-page="overview"]'));
 h.click(h.drawerClose);assert(!h.sidebar.classList.contains('open'));assert.equal(h.main.inert,undefined);assert(h.sidebar.inert);assert.equal(h.document.activeElement,h.toggle);
 h.click(h.toggle);h.document.emit('keydown',h.drawerClose,{key:'Escape'});assert(!h.sidebar.classList.contains('open'));assert.equal(h.document.activeElement,h.toggle);
 h.click(h.toggle);h.click(h.sidebar.querySelector('[data-page="ping"]'));assert(!h.sidebar.classList.contains('open'));assert.equal(h.state.activePage,'ping');assert.equal(h.document.activeElement,h.document.getElementById('workspaceScroll'));assert.equal(h.state.navigationCollapsed,false);
});
test('breakpoint transitions relocate the same toggle and release drawer inert state',()=>{
 const h=harness({collapsed:true});h.resize(true);assert.equal(h.toggle.parentElement.id,'topbar');assert.equal(h.brand.firstElementChild,h.drawerClose);assert(h.state.navigationCollapsed);
 h.click(h.toggle);assert(h.main.inert);h.drawerClose.focus();h.resize(false);assert.equal(h.brand.firstElementChild,h.toggle);assert.equal(h.main.inert,undefined);assert(!h.sidebar.classList.contains('open'));assert(h.state.navigationCollapsed);assert.equal(h.document.activeElement,h.toggle);assert.equal(h.document.querySelectorAll('#mobileMenu').length,1);
 h.click(h.sidebar.querySelector('.nav-scroll'));assert(h.state.navigationCollapsed);h.click(h.toggle);assert(!h.state.navigationCollapsed);h.resize(true);h.main.inert=true;h.click(h.toggle);h.click(h.drawerClose);assert.equal(h.main.inert,true,'Closing the drawer preserves an existing inert owner');
});

test('tool-tab double clicks toggle each desktop panel state once despite node redraw and native dblclick',()=>{
 for(const collapsed of [false,true]){
  const h=harness({collapsed});
  h.click(h.sidebar.querySelector('[data-page="ping"] svg'));
  assert.equal(h.state.navigationCollapsed,collapsed,'First click only navigates');
  // The real navigate() replaces the tab DOM after its first click.
  h.sidebar.querySelector('.nav-scroll').innerHTML='<button class="nav-item active" data-page="ping"><svg></svg><span>Ping</span></button>';
  const icon=h.sidebar.querySelector('[data-page="ping"] svg');
  h.click(icon,0,100);
  assert.equal(h.state.navigationCollapsed,!collapsed,'Second click toggles the panel');
  assert.equal(h.state.activePage,'ping');
  assert.equal(h.calls.filter(x=>x==='save').length,1);
  h.sidebar.emit('dblclick',icon);
  assert.equal(h.state.navigationCollapsed,!collapsed,'Native dblclick does not toggle again');
  assert.equal(h.calls.filter(x=>x==='save').length,1);
 }
});
test('rapid clicks on different tabs and delayed repeats remain ordinary navigation',()=>{
 const h=harness(),overview=h.sidebar.querySelector('[data-page="overview"]'),ping=h.sidebar.querySelector('[data-page="ping"]');
 h.click(ping);h.click(overview,0,100);assert.equal(h.state.navigationCollapsed,false);
 h.click(overview,0,600);assert.equal(h.state.navigationCollapsed,false);
 h.click(overview,0,100);assert.equal(h.state.navigationCollapsed,true);
 h.click(h.toggle);assert.equal(h.state.navigationCollapsed,false);
 h.click(overview,0,100);assert.equal(h.state.navigationCollapsed,false,'Arrow toggle clears the previous click pair');
});

test('an intervening control click breaks a tool-tab double-click sequence',()=>{
 const h=harness(),ping=h.sidebar.querySelector('[data-page="ping"]');
 h.click(ping);h.click(h.document.getElementById('workspaceScroll'),0,100);h.click(ping,0,100);
 assert.equal(h.state.navigationCollapsed,false);
});
