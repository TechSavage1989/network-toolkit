'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const source=fs.readFileSync(require.resolve('../web/assets/terminal-sftp-sidebar.js'),'utf8');
const settle=async()=>{for(let i=0;i<5;i++)await new Promise(done=>setImmediate(done));};
function harness(options={}){
 const document=makeDocument('<section id="terminalSidebarSFTP"></section><div id="testModal"></div>');document.body=document;
 const entries=new Map(),calls=[],notes=[];
 const entry={info:{id:'ssh-one',type:'ssh',state:'connected',label:'Switch - East',host:'192.0.2.21',port:22,username:'admin'}};
 let selected=entry,revision=1;
 const state={transferModes:{sftp:'server'}};
 const api=async(path,method,body)=>{
  calls.push([path,method,body&&{...body}]);
  if(path==='/remote-browser'){
   if(body.action==='probe'){if(options.unsupported)throw new Error('SFTP subsystem disabled');return {supported:true};}
   if(body.action==='list')return {path:body.path,entries:[{name:'startup.cfg',path:'./startup.cfg',directory:false,size:37},{name:'configs',path:'./configs',directory:true,size:0}]};
   if(body.action==='preview')return {content:'hostname switch-1\n'};
   return {summary:'ok'};
  }
  throw Error('Unexpected request');
 };
 const root={document,console,Map,Set,Math,JSON,Date,Number,String,state,api,esc:s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),icon:()=>'[icon]',
  currentTerminal:()=>selected,terminalOwnerCanMutate:e=>e!==null&&!e.unowned,
  terminalSessionRevision:()=>revision,NetworkToolkitSessionSidebar:{render(){notes.push('render');}},
  toast:(...args)=>notes.push(args),navigate:page=>notes.push(['navigate',page]),saveState:()=>notes.push('save'),
  openModal:(title,body,footer)=>{document.getElementById('testModal').innerHTML=body+footer;notes.push(['modal',title]);},
  closeModal:()=>{document.getElementById('testModal').innerHTML='';}
 };
 root.globalThis=root;root.window=root;vm.createContext(root);vm.runInContext(source,root);
 return {api:root.NetworkToolkitTerminalSFTP,root,document,entry,calls,notes,state,selected:e=>{selected=e;},revision:()=>{revision++;}};
}
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
