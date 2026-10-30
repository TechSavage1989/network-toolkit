'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const source=fs.readFileSync(require.resolve('../web/assets/terminal-sftp-sidebar.js'),'utf8');
const nativeText=fs.readFileSync(require.resolve('../web/assets/remote-drag.js'),'utf8'),nativeSource=nativeText.slice(nativeText.indexOf('// Native file-row pointer bridge.'));
const settle=async()=>{for(let i=0;i<5;i++)await new Promise(done=>setImmediate(done));};
function harness(options={}){
 const document=makeDocument('<section id="terminalSidebarSFTP"></section><div id="testModal"></div>');document.body=document;document.elementFromPoint=()=>null;
 const calls=[],notes=[],timers=[];
 const entry={info:{id:'ssh-one',type:'ssh',state:'connected',label:'Switch - East',host:'192.0.2.21',port:22,username:'admin'}};
 let selected=entry,revision=1;
 const state={transferModes:{sftp:'server'}};
 const api=async(path,method,body)=>{
  calls.push([path,method,body&&{...body}]);
  if(path.startsWith('/explorer-transfer'))return options.nativeResponse?options.nativeResponse(path,method,body):{id:'a'.repeat(32),state:'ready'};
  if(path==='/remote-browser'){
   if(body.action==='probe'){if(options.unsupported)throw new Error('SFTP subsystem disabled');return {supported:true,readOnly:options.readOnly};}
   if(body.action==='list')return {path:body.path,entries:[{name:'startup.cfg',path:'./startup.cfg',directory:false,size:37},{name:'configs',path:'./configs',directory:true,size:0}]};
   if(body.action==='preview')return {content:'hostname switch-1\n'};
   return {summary:'ok'};
  }
  throw Error('Unexpected request');
 };
 class PointerEvent{constructor(type,properties){Object.assign(this,{type,isTrusted:true,button:0,isPrimary:true,pointerId:7,pointerType:'mouse',clientX:40,clientY:60,buttons:1,...properties});}preventDefault(){this.prevented=true;}stopPropagation(){}stopImmediatePropagation(){this.stopped=true;}}
 const root={document,console,Map,Set,Math,JSON,Date,Number,String,state,api,PointerEvent,innerWidth:1000,innerHeight:700,setTimeout:fn=>{timers.push(fn);return timers.length;},clearTimeout(){},esc:s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),icon:()=>'[icon]',
  currentTerminal:()=>selected,terminalOwnerCanMutate:e=>e!==null&&!e.unowned,
  terminalSessionRevision:()=>revision,NetworkToolkitSessionSidebar:{render(){notes.push('render');}},
  toast:(...args)=>notes.push(args),navigate:page=>notes.push(['navigate',page]),saveState:()=>notes.push('save'),
  openModal:(title,body,footer)=>{document.getElementById('testModal').innerHTML=body+footer;notes.push(['modal',title]);},
  closeModal:()=>{document.getElementById('testModal').innerHTML='';}
 };
 if(options.native)root.__networkToolkitNativeSession='b'.repeat(64);if(options.detached)root.DETACHED_TERMINAL=true;
 root.globalThis=root;root.window=root;vm.createContext(root);if(options.bridge)vm.runInContext(nativeSource,root);vm.runInContext(source,root);
 const pointer=(type,target,extra={})=>new PointerEvent(type,{target,...extra});
 const emit=(type,target,extra={})=>{const event=pointer(type,target,extra);for(const listener of document.listeners[type]||[])listener(event);return event;};
 return {api:root.NetworkToolkitTerminalSFTP,root,document,entry,calls,notes,state,timers,pointer,emit,selected:e=>{selected=e;},revision:()=>{revision++;}};
}
async function ready(options={}){const h=harness(options);h.panel=h.document.getElementById('terminalSidebarSFTP');h.api.sync(h.entry);await settle();h.api.render(h.panel,h.entry);await settle();h.row=h.panel.querySelector('[data-tsftp-item="./startup.cfg"]');h.folder=h.panel.querySelector('[data-tsftp-item="./configs"]');for(const row of [h.row,h.folder]){row.draggable=true;row.setPointerCapture=()=>{};row.hasPointerCapture=()=>true;row.releasePointerCapture=()=>{};}return h;}
test('dynamic SFTP tab probes only the authenticated current SSH session, without asking for credentials or listing files',async()=>{
 const h=harness(),p=h.document.getElementById('terminalSidebarSFTP');
 assert.equal(h.api.sync(h.entry),false);
 await settle();assert.equal(h.api.available(h.entry),true);
 assert.equal(h.calls.length,1);assert.deepEqual(h.calls[0],[ '/remote-browser','POST',{protocol:'ssh',action:'probe',sessionId:'ssh-one'} ]);
 h.api.render(p,h.entry);await settle();
 assert.equal(h.calls.filter(x=>x[2]?.action==='list').length,1);
 assert.equal(p.dataset.tsftpSession,'ssh-one');
 assert(p.textContent.includes('startup.cfg'));assert(p.textContent.includes('configs'));
 const originalRow=p.querySelector('[data-tsftp-item="./startup.cfg"]');h.api.render(p,h.entry);assert.equal(p.querySelector('[data-tsftp-item="./startup.cfg"]'),originalRow,'refreshing the panel must retain focused file row elements');
 assert.equal(h.calls.every(x=>!('password' in x[2]||'username' in x[2]||'privateKey' in x[2])),true);
});
test('unsupported SSH or non-SSH sessions never expose SFTP browser tools',async()=>{
 const h=harness({unsupported:true}),panel=h.document.getElementById('terminalSidebarSFTP');
 assert.equal(h.api.sync(h.entry),false);await settle();
 assert.equal(h.api.available(h.entry),false);h.api.render(panel,h.entry);
 assert.equal(panel.textContent,'');assert.equal(h.calls.length,1);
 h.entry.info.type='telnet';assert.equal(h.api.sync(h.entry),false);
 h.entry.info.type='ssh';h.entry.info.state='closed';assert.equal(h.api.available(h.entry),false);
});
test('SFTP file operations use a separate subsystem channel and keep the terminal connected',async()=>{
 const h=harness(),panel=h.document.getElementById('terminalSidebarSFTP');
 h.api.sync(h.entry);await settle();h.api.render(panel,h.entry);await settle();
 const file=panel.querySelector('[data-tsftp-item="./startup.cfg"]');assert(file,'file row');
 h.document.emit('click',file);
 const preview=panel.querySelector('[data-tsftp="preview"]');assert(!preview.disabled);
 h.document.emit('click',preview);await settle();
 assert(h.calls.some(x=>x[2]?.action==='preview'&&x[2]?.path==='./startup.cfg'));
 assert(h.notes.some(x=>Array.isArray(x)&&x[0]==='modal'&&x[1].startsWith('Preview')));
 assert.equal(h.entry.info.state,'connected');
 h.document.emit('click',panel.querySelector('[data-tsftp="full"]'));
 assert.equal(h.state.transferModes.sftp,'client');assert(h.notes.some(x=>Array.isArray(x)&&x[0]==='navigate'&&x[1]==='sftp'));
});
test('SFTP sidebar follows the selected SSH session and invalidates disconnected sessions',async()=>{
 const h=harness(),panel=h.document.getElementById('terminalSidebarSFTP');
 h.api.sync(h.entry);await settle();h.api.render(panel,h.entry);await settle();
 const other={info:{id:'ssh-two',type:'ssh',state:'connected',host:'192.0.2.32',port:2222,username:'operator'}};
 h.selected(other);assert.equal(h.api.sync(other),false);await settle();
 assert.equal(h.api.available(other),true);h.api.render(panel,other);await settle();assert.equal(panel.dataset.tsftpSession,'ssh-two');
 other.info.state='closed';assert.equal(h.api.available(other),false);h.api.render(panel,other);
 assert.equal(panel.textContent,'');
 h.selected(h.entry);assert.equal(h.api.available(h.entry),true);h.api.render(panel,h.entry);
 assert.equal(panel.dataset.tsftpSession,'ssh-one');
});

test('sidebar editor carries only its captured terminal ownership binding',async()=>{
 const h=harness(),opened=[];h.root.NetworkToolkitFileEditor={open:async value=>opened.push(value)};const panel=h.document.getElementById('terminalSidebarSFTP');h.api.sync(h.entry);await settle();h.api.render(panel,h.entry);await settle();h.document.emit('click',panel.querySelector('[data-tsftp-item="./startup.cfg"]'));await h.api.editor();assert.equal(opened.length,1);
 assert.deepEqual(JSON.parse(JSON.stringify(opened[0].binding)),{kind:'terminal',sessionId:'ssh-one',revision:1});h.revision();assert.equal(opened[0].binding.revision,1,'A later ownership epoch must not silently retarget the captured descriptor');assert.equal(Object.hasOwn(opened[0].binding,'host'),false);assert.equal(Object.hasOwn(opened[0].binding,'password'),false);
});

test('terminal file and folder native snapshots carry only the exact owned session revision and path',async()=>{
 const h=await ready();for(const row of [h.row,h.folder]){const before=h.calls.length,s=h.api.capturePointerFileDrag(h.pointer('pointerdown',row));assert(s);assert.equal(h.api.validPointerFileDrag(s),true);assert.deepEqual(JSON.parse(JSON.stringify(h.api.pointerFileDragPayload(s))),{binding:{kind:'terminal',sessionId:'ssh-one',revision:1},paths:[row.dataset.tsftpItem]});assert.equal(s.files[0].directory,row===h.folder);assert.equal(row.getAttribute('aria-pressed'),'true');assert.equal(h.calls.length,before,'snapshot does not read, prepare, or connect');}
});
test('terminal native capture rejects fabricated, untrusted, non-left, foreign and unowned sources',async()=>{
 const h=await ready();for(const event of [{type:'pointerdown',isTrusted:true,button:0,target:h.row},h.pointer('pointerdown',h.row,{isTrusted:false}),h.pointer('pointerup',h.row),h.pointer('pointerdown',h.row,{button:2})])assert.equal(h.api.capturePointerFileDrag(event),null);
 const foreign=h.document.createElement('button');foreign.dataset.tsftpItem='./startup.cfg';h.document.append(foreign);assert.equal(h.api.capturePointerFileDrag(h.pointer('pointerdown',foreign)),null);
 h.entry.unowned=true;assert.equal(h.api.capturePointerFileDrag(h.pointer('pointerdown',h.row)),null);assert.equal(h.api.validPointerFileDrag({}),false);assert.throws(()=>h.api.pointerFileDragPayload({}),/SSH session changed/);
});
test('terminal snapshots invalidate on session, ownership, visibility, listing, row and operation changes',async()=>{
 for(const mutate of [h=>h.revision(),h=>{h.entry.info.host='192.0.2.99';},h=>{h.entry.info.state='closed';},h=>{h.entry.unowned=true;},h=>h.selected(null),h=>h.panel.setAttribute('hidden',''),h=>h.row.remove(),(h,s)=>{s.model.busy=true;},(h,s)=>{s.model.loading=true;},(h,s)=>{s.model.folder='/other';},(h,s)=>{s.model.entries=[];},(h,s)=>{s.original.path='./different';},(h,s)=>{s.original.directory=true;},async h=>h.api.load(h.entry.info.id,'.')]){const h=await ready(),s=h.api.capturePointerFileDrag(h.pointer('pointerdown',h.row));await mutate(h,s);assert.equal(h.api.validPointerFileDrag(s),false);assert.throws(()=>h.api.pointerFileDragPayload(s),/SSH session changed/);}
});
test('native terminal rows never warm or apply DownloadURL; ordinary browser fallback remains file-only',async()=>{
 for(const native of [true,false]){const h=await ready({native}),legacy=[];h.root.NetworkToolkitNativeDrag={hover:()=>legacy.push('hover'),apply:()=>legacy.push('apply')};for(const row of [h.row,h.folder]){const formats=[];h.document.emit('pointerover',row);h.document.emit('dragstart',row,{dataTransfer:{setData:(type,value)=>formats.push([type,JSON.parse(value)])}});assert.deepEqual(formats,[['application/x-networktoolkit-sidebar-sftp',{sessionId:'ssh-one',files:[{name:row===h.row?'startup.cfg':'configs',path:row.dataset.tsftpItem,directory:row===h.folder,size:row===h.row?37:0}]}]]);}
 assert.deepEqual(legacy,native?[]:['hover','apply']);}
});
test('native terminal pointer internal drops retain local-pane transfer and same-sidebar copy semantics',async()=>{
 const h=await ready(),transfers=[];h.root.NetworkToolkitRemoteBrowser={dropTransfer:(...args)=>transfers.push(args)};
 const manager=h.document.createElement('section');manager.dataset.rbRoot='ssh';const pane=h.document.createElement('div');pane.dataset.rbPane='local';manager.append(pane);h.document.append(manager);
 const s=h.api.capturePointerFileDrag(h.pointer('pointerdown',h.folder));h.document.elementFromPoint=()=>pane;h.api.finishPointerFileDrag(h.pointer('pointerup',h.folder),s);assert.deepEqual(JSON.parse(JSON.stringify(transfers)),[['ssh','remote','local',{sessionId:'ssh-one',files:[{path:'./configs',name:'configs',directory:true}]}]]);
 pane.dataset.rbPane='remote';h.api.finishPointerFileDrag(h.pointer('pointerup',h.folder),s);assert.equal(transfers.length,1);
 h.document.elementFromPoint=()=>h.panel.querySelector('[data-tsftp-list]');const before=h.calls.length;h.api.finishPointerFileDrag(h.pointer('pointerup',h.folder),s);await settle();assert.equal(h.calls.length,before);assert.match(h.panel.querySelector('[data-tsftp-note]').textContent,/different destination/);
 h.revision();pane.dataset.rbPane='local';h.document.elementFromPoint=()=>pane;h.api.finishPointerFileDrag(h.pointer('pointerup',h.folder),s);assert.equal(transfers.length,1);
});
test('manual terminal folder Download keeps its explicit destination dialog',async()=>{
 const h=await ready({native:true});h.document.emit('click',h.folder);h.document.emit('click',h.panel.querySelector('[data-tsftp="download"]'));assert(h.notes.some(x=>Array.isArray(x)&&x[0]==='modal'&&x[1]==='Download configs'));assert.match(h.document.getElementById('testModal').textContent,/Destination parent folder/);assert.equal(h.calls.some(x=>x[0].startsWith('/explorer-transfer')),false);
});
test('shared native bridge routes terminal files and folders through exact one-use handoff, without prepared copy',async()=>{
 for(const folder of [false,true]){const h=await ready({native:true,bridge:true}),row=folder?h.folder:h.row;let released=0;row.releasePointerCapture=()=>released++;h.emit('pointerdown',row);assert.equal(row.draggable,false);h.emit('pointermove',row,{clientX:44});assert.equal(h.calls.some(x=>x[0].startsWith('/explorer-transfer')),false);h.emit('pointermove',row,{clientX:120});await settle();let calls=h.calls.filter(x=>x[0].startsWith('/explorer-transfer'));assert.deepEqual(calls.map(x=>x[2]?.action),['direct']);assert.deepEqual(JSON.parse(JSON.stringify(calls[0][2])),{action:'direct',binding:{kind:'terminal',sessionId:'ssh-one',revision:1},paths:[row.dataset.tsftpItem]});h.emit('pointermove',row,{clientX:1005});await settle();calls=h.calls.filter(x=>x[0].startsWith('/explorer-transfer'));assert.deepEqual(calls.filter(x=>x[2]).map(x=>x[2].action),['direct','arm']);assert.equal(released,0);const release=h.root.NetworkToolkitNativeDrag.releaseFileRowCapture;assert.equal(release('a'.repeat(32),2),false);assert.equal(release('a'.repeat(32),1),true);assert.equal(released,1);assert.equal(release('a'.repeat(32),1),false);assert.equal(row.draggable,true);assert.equal(h.notes.some(x=>Array.isArray(x)&&x[0]==='modal'),false);}
});
test('terminal click, browser-mode and modified gesture do not register native transfers',async()=>{
 for(const options of [{native:true,bridge:true},{native:false,bridge:true}]){const h=await ready(options);h.emit('pointerdown',h.row);h.emit('pointerup',h.row);await settle();assert.equal(h.calls.some(x=>x[0].startsWith('/explorer-transfer')),false);for(const key of ['altKey','ctrlKey','metaKey','shiftKey']){h.emit('pointerdown',h.row,{[key]:true});h.emit('pointermove',h.row,{clientX:1005});}assert.equal(h.calls.some(x=>x[0].startsWith('/explorer-transfer')),false);assert.equal(h.row.draggable,true);}
});
test('terminal shared bridge cancels before native handoff on release, Escape or changed ownership',async()=>{
 for(const end of ['pointerup','Escape','revision']){const h=await ready({native:true,bridge:true});h.emit('pointerdown',h.row);h.emit('pointermove',h.row,{clientX:100});await settle();if(end==='Escape')h.emit('keydown',h.row,{key:'Escape'});else if(end==='revision'){h.revision();h.emit('pointermove',h.row,{clientX:1005});}else h.emit('pointerup',h.row);await settle();assert.deepEqual(h.calls.filter(x=>x[0].startsWith('/explorer-transfer')).map(x=>x[2]?.action),['direct','cancel']);assert.equal(h.row.draggable,true);assert.equal(h.root.NetworkToolkitNativeDrag.releaseFileRowCapture('a'.repeat(32),1),false);}
});
test('terminal late registration after release is cancelled without native arming or a download dialog',async()=>{
 let resolve;const h=await ready({native:true,bridge:true,nativeResponse:async(path,method,body)=>body?.action==='direct'?new Promise(done=>{resolve=done;}):{state:'cancelled'}});h.emit('pointerdown',h.row);h.emit('pointermove',h.row,{clientX:1005});h.emit('pointerup',h.row,{clientX:1005});resolve({id:'a'.repeat(32),state:'ready'});await settle();assert.deepEqual(h.calls.filter(x=>x[0].startsWith('/explorer-transfer')).map(x=>x[2]?.action),['direct','cancel']);assert.equal(h.row.draggable,true);assert.equal(h.notes.some(x=>Array.isArray(x)&&x[0]==='modal'),false);
});
test('detached read-only capability exposes navigation, preview and native copy, not unsupported write or local-path actions',async()=>{
 for(const options of [{detached:true},{readOnly:true}]){const h=await ready({...options,native:true,bridge:true});assert.deepEqual(h.panel.querySelectorAll('[data-tsftp]').map(button=>button.dataset.tsftp),['up','home','root','refresh','preview']);assert.equal(h.panel.querySelector('[data-tsftp-os-files]'),null);assert.match(h.panel.textContent,/Read-only · Drag to Windows Explorer/);
 h.document.emit('dblclick',h.row);await settle();assert(h.calls.some(x=>x[2]?.action==='preview'));assert(h.notes.some(x=>Array.isArray(x)&&x[0]==='modal'&&x[1]==='Preview startup.cfg'));await h.api.editor();await h.api.editor(true);assert.equal(h.calls.some(x=>x[2]?.action==='write-text'),false);
 h.document.emit('contextmenu',h.row,{clientX:40,clientY:60});assert.deepEqual(h.document.querySelectorAll('[data-tsftp-menu]').map(button=>button.dataset.tsftpMenu),['preview','copyPath','refresh']);
 const forged=h.document.createElement('button');forged.dataset.tsftp='download';h.panel.append(forged);const before=h.notes.length;h.document.emit('click',forged);assert.equal(h.notes.length,before);
 const currentRow=h.panel.querySelector('[data-tsftp-item="./startup.cfg"]');currentRow.setPointerCapture=()=>{};currentRow.hasPointerCapture=()=>true;currentRow.releasePointerCapture=()=>{};h.emit('pointerdown',currentRow);h.emit('pointermove',currentRow,{clientX:1005});await settle();assert.deepEqual(h.calls.filter(x=>x[0].startsWith('/explorer-transfer')).filter(x=>x[2]).map(x=>x[2].action),['direct','arm']);assert.equal(h.root.NetworkToolkitNativeDrag.releaseFileRowCapture('a'.repeat(32),1),true);}
});
test('detached read-only sidebar rejects write requests and incoming file drops without advertising a drop target',async()=>{
 const h=await ready({detached:true,native:true}),list=h.panel.querySelector('[data-tsftp-list]'),before=h.calls.length;
 for(const action of ['download','upload','copy','rename','remove','mkdir','write-text'])await assert.rejects(h.api.request(h.entry.info.id,action,{path:'./startup.cfg',localPath:'C:\\synthetic'}),/read-only/);
 h.document.emit('dragover',list);assert.equal(list.classList.contains('tsftp-drop-target'),false);h.document.emit('drop',list,{dataTransfer:{files:[{name:'synthetic.txt'}]}});await settle();assert.equal(h.calls.length,before);
});
test('ordinary Terminal selection and repeated refreshes retain exact row elements and selected accessibility state',async()=>{
 const h=await ready(),list=h.panel.querySelector('[data-tsftp-list]'),structure=list._tsftpHTML;
 for(const row of [h.row,h.folder,h.row]){h.document.emit('click',row);for(let i=0;i<4;i++)h.api.render(h.panel,h.entry);assert.equal(h.panel.querySelector('[data-tsftp-item="./startup.cfg"]'),h.row);assert.equal(h.panel.querySelector('[data-tsftp-item="./configs"]'),h.folder);assert.equal(list._tsftpHTML,structure);assert.equal(h.row.getAttribute('aria-pressed'),String(row===h.row));assert.equal(h.folder.getAttribute('aria-pressed'),String(row===h.folder));assert.equal(row.classList.contains('selected'),true);}
});
test('routine main and detached Terminal redraws preserve file/folder capture before threshold and after metadata registration',async()=>{
 for(const detached of [false,true])for(const folder of [false,true]){const h=await ready({native:true,bridge:true,detached}),row=folder?h.folder:h.row;let released=0;row.releasePointerCapture=()=>released++;h.emit('pointerdown',row);
 const redraw=()=>{for(let i=0;i<4;i++)h.api.render(h.panel,h.entry);assert.equal(h.panel.querySelector('[data-tsftp-item="'+row.dataset.tsftpItem+'"]'),row);assert.equal(row.draggable,false);assert.equal(released,0);assert.equal(row.getAttribute('aria-pressed'),'true');};
 redraw();h.emit('pointermove',row,{clientX:100});await settle();assert.deepEqual(h.calls.filter(x=>x[0].startsWith('/explorer-transfer')).map(x=>x[2]?.action),['direct']);redraw();h.emit('pointermove',row,{clientX:1005});await settle();assert.deepEqual(h.calls.filter(x=>x[0].startsWith('/explorer-transfer')&&x[2]).map(x=>x[2].action),['direct','arm']);assert.equal(h.root.NetworkToolkitNativeDrag.releaseFileRowCapture('a'.repeat(32),1),true);assert.equal(released,1);}
});
test('selection-stable cache still rebuilds real filter, listing and folder changes',async()=>{
 const h=await ready(),filter=h.panel.querySelector('[data-tsftp-filter]'),snapshot=h.api.capturePointerFileDrag(h.pointer('pointerdown',h.row));filter.value='configs';h.document.emit('input',filter);assert.equal(h.panel.querySelector('[data-tsftp-item="./startup.cfg"]'),null);assert.notEqual(h.panel.querySelector('[data-tsftp-item="./configs"]'),h.folder);
 filter.value='';h.document.emit('input',filter);assert.notEqual(h.panel.querySelector('[data-tsftp-item="./startup.cfg"]'),h.row);await h.api.load(h.entry.info.id,'/another-folder');assert.equal(h.api.validPointerFileDrag(snapshot),false);assert.equal(h.panel.querySelector('[data-tsftp-address]').value,'/another-folder');assert.equal(h.panel.querySelector('[data-tsftp-item="./startup.cfg"]').getAttribute('aria-pressed'),'false');
});
