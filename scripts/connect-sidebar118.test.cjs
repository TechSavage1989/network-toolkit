'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const read=name=>fs.readFileSync(require.resolve('../web/'+name),'utf8');
const html=read('index.html'),start=html.indexOf('const paths='),end=html.indexOf('const label=',start),registry={esc:String};
assert(start>0&&end>start);vm.runInNewContext(html.slice(start,end)+';globalThis.icons={paths,icon};',registry);
const {paths,icon}=registry.icons;
function editor(child,preference){
 const document=makeDocument('<section id="networkToolkitFileEditor"><button data-ed="files"></button><div id="ntEditorRemotePanel"></div><aside data-ed-files-sidebar></aside></section>');document.body=document;
 const calls=[],root={document,icon,EDITOR_WINDOW:child,innerWidth:1440,clearTimeout,setTimeout,esc:String,api:async(p,method)=>{calls.push({p,method});return {allowed:false,pending:false};},state:{editorFilesPreferences:preference},sessionStorage:{getItem:()=>JSON.stringify(preference||{}),setItem:()=>{}},saveState(){},addEventListener(){},NetworkToolkitFileEditor:{refreshConnectedSources:async()=>{}},NetworkToolkitRemoteBrowser:{mountSidebar(){},unmountSidebar(){}}};
 vm.runInNewContext(read('assets/editor-files-sidebar.js'),root);const host=document.querySelector('[data-ed-files-sidebar]');root.NetworkToolkitEditorFiles.initialize(document.querySelector('section'));return {root,host,shell:host.closest('.nt-editor-files-shell'),calls};
}
test('Connect has a dedicated centered plug path; old plug, play and settings remain distinct',()=>{
 assert.equal(paths.connect,'M8 3v5 M16 3v5 M6 8h12v4a6 6 0 0 1-12 0z M12 18v3');
 assert.equal(paths.plug,'M6 3v5 M14 3v5 M4 8h12v4a6 6 0 0 1-12 0z M10 18v3 M18 5h3 M19 3v4');
 assert.equal(paths.play,'M8 4l12 8-12 8z');for(const key of ['plug','play','settings','folder'])assert.notEqual(paths.connect,paths[key]);
 assert.equal(makeDocument(icon('connect')).querySelector('svg').getAttribute('aria-hidden'),'true');
});
test('actual editor rail puts Connect before Files without changing default or saved selection',async()=>{
 for(const child of [false,true])for(const panel of [undefined,'files','connections']){
  const preference=panel?{panel,side:'remote',width:380,collapsed:true}:undefined,h=editor(child,preference),tabs=h.shell.querySelectorAll('[role="tab"]');
  const primary=tabs.filter(el=>el.dataset.editorFilesPanel);assert.deepEqual(primary.map(el=>el.dataset.editorFilesPanel),['connections','files']);
  assert.equal(primary[0].textContent,'Connect');assert.equal(primary[0].getAttribute('aria-label'),'Connections');assert.equal(primary[0].querySelector('path').getAttribute('d'),paths.connect);assert.equal(primary[1].querySelector('path').getAttribute('d'),paths.folder);
  const selected=primary.find(el=>el.getAttribute('aria-selected')==='true');assert.equal(selected.dataset.editorFilesPanel,panel||'files');assert.equal(selected.tabIndex,0);assert.equal(h.host.hidden,!!preference);assert.equal(h.shell.style.width,preference?'380px':'300px');
  await new Promise(resolve=>setImmediate(resolve));assert(h.calls.every(c=>c.p==='/editor-views/current/explorer'&&!c.method));
 }
});
test('Connect-first keyboard Home/End and arrows match DOM order, without connecting',()=>{
 const h=editor(false),files=h.shell.querySelector('[data-editor-files-panel="files"]'),connect=h.shell.querySelector('[data-editor-files-panel="connections"]');
 for(const [target,key,panel]of [[files,'Home','connections'],[connect,'ArrowDown','files'],[files,'ArrowUp','connections'],[connect,'End','files']]){let prevented=false;h.shell.emit('keydown',target,{key,preventDefault(){prevented=true;}});assert(prevented);assert.equal(h.host.dataset.editorSidebarPanel,panel);assert(!h.host.hidden);}
 for(const key of ['Enter',' ','Tab']){let prevented=false;h.shell.emit('keydown',files,{key,preventDefault(){prevented=true;}});assert(!prevented);}
 assert.deepEqual(h.calls,[]);
});
test('all client/server Connect render sites use the same registry key, not gear or serial plug',()=>{
 const client=read('assets/transfer-client-layout.js'),server=read('assets/transfer-server.js');
 assert(client.includes("[['settings','Connect','connect'],['saved','Saved','bookmark'],['recent','Recent','clock']]"));
 const rail=server.split('\n').find(line=>line.includes('sidebar.innerHTML=')&&line.includes('transfer-server-sidebar-button'));
 assert(rail.includes("icon('connect')+'<span>Connect</span>"));assert(!rail.includes("icon('settings')"));
});
test('Terminal Saved/Recent/Macros/SFTP rail retains its existing icons and has no new Connect tab',()=>{
 const terminal=read('assets/session-sidebar.js');assert(terminal.includes("icon(tab==='saved'?'bookmark':tab==='recent'?'clock':tab==='files'?'folder':'play')"));assert(!terminal.includes("icon('connect')"));assert(!terminal.includes("<span>Connect</span>"));
});

test('TCP Ports uses a distinct port-inspection symbol in tool and navigation, not Connect',()=>{
 assert.equal(paths.tcpPorts,'M3 3h18v8H3z M7 6v2 M12 6v2 M17 6v2 M15 12a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M18 19l4 3');
 for(const key of ['connect','plug','server','network'])assert.notEqual(paths.tcpPorts,paths[key]);
 assert(html.includes("tcp:{name:'TCP ports',icon:'tcpPorts'"));assert(html.includes("['tcp','TCP ports','tcpPorts']"));
 assert(!html.includes("tcp:{name:'TCP ports',icon:'plug'"));assert(!html.includes("['tcp','TCP ports','plug']"));
 const svg=makeDocument(icon('tcpPorts')).querySelector('svg');assert.equal(svg.getAttribute('viewBox'),'0 0 24 24');assert.equal(svg.getAttribute('aria-hidden'),'true');
});
