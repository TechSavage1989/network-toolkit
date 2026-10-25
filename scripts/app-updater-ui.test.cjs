'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const available={currentVersion:'1.17.59',supported:true,installing:false,busy:[],hasUpdate:true,staged:null,latest:{version:'1.17.60'}};
function ui(options={}){
 const document=makeDocument('<div id="appUpdateNotice"></div><div id="modalRoot"><div id="existingDialog">Existing work</div></div>');
 const calls=[],timers=new Map();let id=0,stored=options.preferences||{},data=structuredClone(available);
 const c={document,AbortController,online:true,OFFLINE:false,shutDown:false,TERMINAL_ONLY:false,DETACHED_TERMINAL:false,status:{os:'windows',arch:'amd64',version:'1.17.59'},VERSION:'1.17.59',state:{appUpdatePreferences:structuredClone(stored)},setTimeout:(fn,delay)=>{timers.set(++id,{fn,delay});return id;},clearTimeout:n=>timers.delete(n),saveState:()=>{stored=structuredClone(c.state.appUpdatePreferences)},flushState:async()=>true,toast:()=>{},api:async(path,method,body)=>{calls.push([path,method,body]);if(path==='/updates/download')data={...data,staged:{version:body.version,sha256:'verified'}};if(path==='/updates/install')return {restarting:true};return structuredClone(data);},openModal:(title,body,footer)=>{document.getElementById('modalRoot').innerHTML='<h2>'+title+'</h2>'+body+footer;},...options.context};
 c.window=c;vm.createContext(c);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/app-update-notes.js'),'utf8'),c);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/app-updater.js'),'utf8'),c);
 return {c,calls,timers,document,stored:()=>stored,async tick(){const [id,timer]=timers.entries().next().value;timers.delete(id);await timer.fn();},async click(action){const target=document.querySelector('[data-app-update="'+action+'"]');assert.ok(target,action+' exists');for(const listener of document.listeners.click||[])await listener({target});},settings(){document.getElementById('modalRoot').innerHTML='<section id="appUpdateSettings">'+c.NetworkToolkitUpdater.html()+'</section>';return c.NetworkToolkitUpdater.load();}};
}
test('startup detection leaves existing dialog untouched and does not download',async()=>{const h=ui();h.c.NetworkToolkitUpdater.start();await h.tick();assert.ok(h.document.getElementById('existingDialog'));assert.match(h.document.getElementById('appUpdateNotice').textContent,/available/);assert.deepEqual(h.calls.map(c=>c[0]),['/updates/status','/updates/check']);});
test('Later hides banner across settings reopen and fresh page with saved preference',async()=>{const h=ui();h.c.NetworkToolkitUpdater.start();await h.tick();await h.click('later');assert.equal(h.document.getElementById('appUpdateNotice').textContent,'');await h.settings();assert.equal(h.document.getElementById('appUpdateNotice').textContent,'');const reloaded=ui({preferences:h.stored()});reloaded.c.NetworkToolkitUpdater.start();await reloaded.tick();assert.equal(reloaded.document.getElementById('appUpdateNotice').textContent,'');await h.click('check');assert.match(h.document.getElementById('appUpdateNotice').textContent,/available/);});
test('download stages then offers separate review; close/cancel does not install and can reopen',async()=>{const h=ui();h.c.NetworkToolkitUpdater.start();await h.tick();await h.click('download');assert.equal(h.calls.some(c=>c[0]==='/updates/download'),false);await h.click('confirm-download');assert.match(h.document.getElementById('appUpdateNotice').textContent,/downloaded and verified/);assert.match(h.document.getElementById('modalRoot').textContent,/What's new/);await h.click('review');assert.match(h.document.getElementById('modalRoot').textContent,/Install and restart/);h.document.getElementById('modalRoot').innerHTML='';assert.equal(h.calls.some(c=>c[0]==='/updates/install'),false);await h.click('review');await h.click('confirm');assert.equal(h.calls.filter(c=>c[0]==='/updates/install').length,1);assert.equal(h.document.getElementById('appUpdateNotice').textContent,'');});
test('automatic toggle persists and scheduler remains single across repeated starts',async()=>{const h=ui();h.c.NetworkToolkitUpdater.start();await h.settings();const checkbox=h.document.querySelector('[data-app-update-auto]');checkbox.checked=false;for(const listener of h.document.listeners.change||[])await listener({target:checkbox});h.c.NetworkToolkitUpdater.start();assert.equal(h.timers.size,1);assert.equal(h.stored().enabled,false);await h.tick();assert.equal(h.calls.some(c=>c[0]==='/updates/check'),false);});
test('preview and secondary terminal windows never schedule automatic checks',()=>{for(const context of [{OFFLINE:true},{TERMINAL_ONLY:true},{DETACHED_TERMINAL:true}]){const h=ui({context});h.c.NetworkToolkitUpdater.start();assert.equal(h.timers.size,0);}});

test('recovered startup without global platform status still detects supported updates',async()=>{const h=ui({context:{status:{}}});h.c.NetworkToolkitUpdater.start();await h.tick();assert.ok(h.calls.some(c=>c[0]==='/updates/check'));assert.match(h.document.getElementById('appUpdateNotice').textContent,/available/);});

test('reopened update shows running version once without starting another install',async()=>{const notices=[];const h=ui({context:{toast:m=>notices.push(m),api:async()=>({...available,currentVersion:'1.17.60',restartedAfterUpdate:true})}});await h.settings();await h.settings();assert.equal(notices.length,1);assert.match(notices[0],/Now running Network Toolkit 1.17.60/);});
test('install review is concise and preserves explicit restart confirmation',async()=>{const h=ui();h.c.NetworkToolkitUpdater.start();await h.tick();await h.click('download');assert.equal(h.calls.some(c=>c[0]==='/updates/download'),false);await h.click('confirm-download');await h.click('review');const text=h.document.getElementById('modalRoot').textContent;assert.match(text,/Save your work before restarting\./);assert.match(text,/Replace the current app with version 1.17.60 and restart\?/);assert.match(text,/Install and restart/);assert.match(text,/What's new/);assert.doesNotMatch(text,/Export any diagnostic results|Updates keep|stable Network_Toolkit.exe filename|personal Desktop and Start Menu|Pinned shortcuts|previous.json|atomic swap|SHA-256 verification checks/);assert.equal(h.calls.some(c=>c[0]==='/updates/install'),false);});

test('filename setup failure is surfaced once and remains visible in settings',async()=>{const notices=[];const h=ui({context:{toast:m=>notices.push(m),api:async()=>({...available,executable:{currentPath:'app.exe',warning:'Conflicting current-version file'}})}});await h.settings();await h.settings();assert.equal(notices.length,1);assert.match(notices[0],/filename setup needs attention/);assert.match(h.document.getElementById('appUpdateSettings').textContent,/Conflicting current-version file/);});

test('closing download review during a request does not reopen it or install automatically',async()=>{
 let resolve;const pending=new Promise(r=>resolve=r);
 const h=ui({context:{api:async(path,method,body)=>path==='/updates/download'?pending:structuredClone(available)}});
 h.c.NetworkToolkitUpdater.start();await h.tick();await h.click('download');
 assert.match(h.document.getElementById('modalRoot').textContent,/What's new/);
 const downloading=h.click('confirm-download');await Promise.resolve();
 h.document.getElementById('modalRoot').innerHTML='<div id="newWork">Unrelated work</div>';
 resolve({...available,staged:{version:'1.17.60',sha256:'verified'}});await downloading;
 assert.ok(h.document.getElementById('newWork'));assert.match(h.document.getElementById('appUpdateNotice').textContent,/downloaded and verified/);
 await h.click('review');assert.match(h.document.getElementById('modalRoot').textContent,/What's new/);
});

test('install notes never borrow the body of a different latest release',async()=>{
 const h=ui({context:{api:async()=>({...available,latest:{version:'1.17.61',notes:'Wrong release secret'},staged:{version:'1.17.60',sha256:'verified'}})}});
 h.c.NetworkToolkitUpdater.start();await h.tick();await h.click('review');
 const text=h.document.getElementById('modalRoot').textContent;
 assert.match(text,/Release notes are not available/);assert.doesNotMatch(text,/Wrong release secret/);
});
