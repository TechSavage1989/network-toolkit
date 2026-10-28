'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
function harness({short=false}={}){
 const document=makeDocument('<aside id="sidebar"><div class="brand-mark"></div><span id="hostTop">SYNTHETIC-LONG-HOST</span><div class="app-utility-actions"><button id="themeButton"></button><button id="settingsButton"></button><span id="runningCount">3 running</span></div></aside><main><span id="appNavigationSlot"></span><span id="appNavigationFooterSlot"></span><div id="toolCommandBar"></div><div id="terminalCommandBar"><div class="terminal-command-start"></div></div></main>');document.body=document;const c={document,matchMedia:()=>({matches:short,addEventListener(){}})};c.globalThis=c;vm.createContext(c);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/app-chrome.js'),'utf8'),c);return{document,api:c.NetworkToolkitAppChrome};
}
test('identity and running counts remain explicit accessible text without duplicating actions',()=>{
 const {document:d,api}=harness();api.init();api.sync();assert(d.classList.contains('app-chrome-ready'));
 assert.equal(d.getElementById('hostTop').title,'SYNTHETIC-LONG-HOST');assert.equal(d.querySelector('.brand-mark').getAttribute('aria-label'),'Network Toolkit. System: SYNTHETIC-LONG-HOST');
 const count=d.getElementById('runningCount');assert.equal(count.dataset.count,'3');assert.equal(count.getAttribute('aria-label'),'3 running diagnostic jobs');assert.equal(count.tabIndex,0);assert.equal(d.querySelectorAll('#themeButton').length,1);assert.equal(d.querySelectorAll('#settingsButton').length,1);
 d.classList.add('app-nav-collapsed');api.sync();assert.equal(d.querySelector('.brand-mark').tabIndex,0);count.hidden=true;api.sync();assert.equal(count.tabIndex,-1);d.classList.remove('app-nav-collapsed');api.sync();assert.equal(d.querySelector('.brand-mark').tabIndex,-1);
});
test('compact navigation reuses stable tool slots including terminal and short diagnostic layouts',()=>{
 const {document:d,api}=harness();assert.equal(api.navigationHost().id,'appNavigationSlot');d.classList.add('diagnostic-page');assert.equal(api.navigationHost().id,'toolCommandBar');d.classList.add('terminal-page');assert.equal(api.navigationHost().className,'terminal-command-start');
 for(const mode of ['terminal-window','editor-window']){d.classList.add(mode);assert.equal(api.navigationHost(),null);d.classList.remove(mode);}
});
test('missing or untrusted-looking display metadata does not create markup or actions',()=>{
 const {document:d,api}=harness();d.getElementById('hostTop').textContent='<script>not HTML</script>';d.getElementById('runningCount').textContent='no count';api.sync();assert.equal(d.querySelectorAll('script').length,0);assert.equal(d.getElementById('hostTop').title,'<script>not HTML</script>');assert.equal(d.getElementById('runningCount').dataset.count,'0');d.getElementById('hostTop').remove();d.getElementById('runningCount').remove();assert.doesNotThrow(()=>api.sync());
});
test('shell removes the redundant header and retains one original control of each kind',()=>{
 const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');assert(!html.includes('<div class="topbar">'));for(const id of ['crumb','hostTop','runningCount','mobileMenu','themeButton','settingsButton'])assert.equal(html.split('id="'+id+'"').length-1,1,id);assert.match(html,/<span id="crumb" hidden>/);assert.match(html,/app-utility-actions[^]*id="themeButton"[^]*id="settingsButton"/);assert.match(html,/window\.NetworkToolkitTFTPConnections\?\.afterLayout\?\.\(\);updateRunButtons\(\);NetworkToolkitNavigation\.sync\(\)/);
});


test('short local workspaces use the existing footer only while their command bar is hidden',()=>{
 const {document:d,api}=harness({short:true});api.init();d.classList.add('diagnostic-page');d.getElementById('toolCommandBar').classList.add('hidden');assert.equal(api.navigationHost().id,'appNavigationFooterSlot');d.getElementById('toolCommandBar').classList.remove('hidden');assert.equal(api.navigationHost().id,'toolCommandBar');
});
