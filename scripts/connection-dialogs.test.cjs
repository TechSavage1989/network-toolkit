'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const folders=require('../web/assets/saved-session-folders.js');
const settle=async()=>{for(let i=0;i<3;i++)await new Promise(resolve=>setImmediate(resolve));};
function harness(module){
 const document=makeDocument('<div id="testModal"></div>');document.body=document;
 const questions=[],writes=[],requests=[],notes=[];
 const ask=(kind,options)=>new Promise(resolve=>questions.push({kind,options,resolve}));
 const state={activePage:'',terminalProfiles:[],terminalRecent:[],sftpRecent:[],ftpProfiles:[],ftpRecent:[],tftpProfiles:[],tftpRecent:[],target:'192.0.2.10',forms:{tftp:{operation:'download',remotePath:'startup.cfg',localPath:'C:/files/startup.cfg',port:69}}};
 const root={document,state,console,Date,Math,JSON,Map,Set,Number,String,Object,Array,
  esc:String,icon:()=>'',NetworkToolkitSessionFolders:folders,NetworkToolkitTransfers:{mode:()=> 'client'},
  NetworkToolkitDialogs:{prompt:options=>ask('prompt',options),confirm:options=>ask('confirm',options)},
  prompt:()=>{throw Error('Native prompt must not be used');},confirm:()=>{throw Error('Native confirm must not be used');},
  saveState:()=>writes.push(JSON.stringify(state)),toast:(...args)=>notes.push(args),
  openModal:(title,body,footer)=>{document.getElementById('testModal').innerHTML=body+footer;},closeModal(){},
  api:(path,method,body)=>{requests.push({path,method,body});return new Promise(()=>{});}};
 root.window=root;root.globalThis=root;vm.createContext(root);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/'+module+'.js'),'utf8'),root);
 const mount=markup=>{const area=document.createElement('section');area.innerHTML=markup;document.append(area);return area;};
 return {root,state,document,questions,writes,requests,notes,mount};
}
const ftp=(id='ftp-one')=>({id,name:'Files',host:'files.example.test',port:21,username:'operator',security:'explicit',fingerprint:'',credentialId:'credential-one',folder:'Branch'});
const ssh=(id='ssh-one')=>({id,name:'Switch',type:'ssh',host:'192.0.2.10',port:22,username:'operator',authMethod:'password',sshCompatibility:'modern',folder:'Branch'});

for(const [kind,key,profile,method] of [['ftp','ftpProfiles',ftp,'saveFTPProfile'],['ssh','terminalProfiles',ssh,'saveSharedSSHProfile']]){
 test(kind+' profile naming waits for an app dialog and rejects a changed destination or saved row',async()=>{
  for(const change of ['cancel','destination','profile','accept']){
   const h=harness('remote-browser'),rb=h.root.NetworkToolkitRemoteBrowser,row=profile();h.state[key]=[row];
   Object.assign(rb.configurations[kind],row);rb.get(kind).profileId=row.id;
   const pending=rb[method]();assert.equal(h.questions.length,1);assert.equal(h.questions[0].kind,'prompt');assert.equal(h.writes.length,0);
   if(change==='destination')rb.configurations[kind].host='different.example.test';
   if(change==='profile')row.folder='Changed elsewhere';
   h.questions[0].resolve(change==='cancel'?null:'Reviewed name');await pending;
   assert.equal(h.writes.length,change==='accept'?1:0,change);
   assert.equal(h.state[key][0].name,change==='accept'?'Reviewed name':row.name);
   if(change==='accept'){assert.equal(h.state[key][0].host,row.host);assert.equal(h.state[key][0].folder,'Branch');}
  }
 });
 test(kind+' explicit profile names still save immediately without opening a dialog',()=>{
  const h=harness('remote-browser'),rb=h.root.NetworkToolkitRemoteBrowser,row=profile();h.state[key]=[row];Object.assign(rb.configurations[kind],row);rb.get(kind).profileId=row.id;
  rb[method]('Immediate');assert.equal(h.state[key][0].name,'Immediate');assert.equal(h.writes.length,1);assert.equal(h.questions.length,0);
 });
}

for(const action of ['rename','delete'])test('FTP profile '+action+' preserves the reviewed identity through the async answer',async()=>{
 for(const change of ['cancel','changed','removed','retargeted']){
  const h=harness('remote-browser'),one=ftp(),two=ftp('ftp-two');h.state.ftpProfiles=[one,two];
  const node=h.mount('<button data-ftp-profile-action="'+action+'" data-id="ftp-one">Action</button>').querySelector('button');
  h.document.emit('click',node);assert.equal(h.questions.length,1);assert.equal(h.writes.length,0);
  if(change==='changed')one.host='changed.example.test';
  if(change==='removed')h.state.ftpProfiles=[two];
  if(change==='retargeted')node.dataset.id='ftp-two';
  h.questions[0].resolve(change==='cancel'?(action==='rename'?null:false):action==='rename'?'Renamed':true);await settle();
  assert.equal(h.writes.length,change==='retargeted'?1:0,change);
  assert.equal(h.state.ftpProfiles.find(x=>x.id==='ftp-two').name,'Files','a changed DOM row cannot redirect the accepted action');
  if(change==='retargeted')assert.equal(h.state.ftpProfiles.some(x=>x.id==='ftp-one'),action==='rename');
 }
});

test('TFTP naming cancels cleanly and rejects changed form data or an edited matching profile',async()=>{
 for(const change of ['cancel','target','form','profile','accept']){
  const h=harness('tftp-connections');h.state.tftpProfiles=[{...h.state.forms.tftp,id:'profile-one',name:'Transfer',host:h.state.target}];
  const pending=h.root.NetworkToolkitTFTPConnections.save();assert.equal(h.writes.length,0);assert.equal(h.questions.length,1);
  if(change==='target')h.state.target='192.0.2.20';
  if(change==='form')h.state.forms.tftp.remotePath='changed.cfg';
  if(change==='profile')h.state.tftpProfiles[0].localPath='C:/changed.cfg';
  h.questions[0].resolve(change==='cancel'?null:'Transfer');await pending;
  assert.equal(h.writes.length,change==='accept'?1:0,change);assert.equal(h.state.tftpProfiles.length,1);
 }
});

for(const action of ['rename','delete'])test('TFTP profile '+action+' cannot act on a changed entry or a different DOM selection',async()=>{
 for(const change of ['cancel','changed','retargeted']){
  const h=harness('tftp-connections');h.state.tftpProfiles=[{id:'one',name:'One',host:'one.test',port:69},{id:'two',name:'Two',host:'two.test',port:69}];
  const node=h.mount('<button data-tftp-entry="'+action+'" data-kind="profile" data-id="one">Action</button>').querySelector('button');
  h.document.emit('click',node);assert.equal(h.questions.length,1);assert.equal(h.writes.length,0);
  if(change==='changed')h.state.tftpProfiles[0].host='changed.test';
  if(change==='retargeted'){node.dataset.kind='recent';node.dataset.id='two';}
  h.questions[0].resolve(change==='cancel'?(action==='rename'?null:false):action==='rename'?'Renamed':true);await settle();
  assert.equal(h.writes.length,change==='retargeted'?1:0,change);assert.equal(h.state.tftpProfiles.find(x=>x.id==='two').name,'Two');
  if(change==='retargeted')assert.equal(h.state.tftpProfiles.some(x=>x.id==='one'),action==='rename');
 }
});

for(const kind of ['ssh','ftp','tftp'])test(kind+' clear-recent confirmation keeps newly added or changed records',async()=>{
 const h=harness(kind==='tftp'?'tftp-connections':'remote-browser'),key=kind==='tftp'?'tftpRecent':kind==='ftp'?'ftpRecent':'sftpRecent';
 h.state.activePage=kind==='ssh'?'sftp':kind;h.state[key]=[{id:'remove',name:'Reviewed'},{id:'keep',name:'Old'}];
 if(kind==='ssh')h.state.terminalRecent=[{id:'ssh-old',type:'ssh'},{id:'telnet',type:'telnet'}];
 const node=h.mount(kind==='tftp'?'<button data-tftp-library="clear-recent">Clear</button>':'<div data-rb-library="'+kind+'"><button data-rb-library-command="clear">Clear</button></div>').querySelector('button');
 h.document.emit('click',node);assert.equal(h.questions.length,1);assert.equal(h.writes.length,0);assert.equal(h.questions[0].options.danger,true);
 h.state[key][1].name='Updated elsewhere';h.state[key].push({id:'new',name:'New'});
 if(kind==='ssh')h.state.terminalRecent.push({id:'ssh-new',type:'ssh'});
 h.questions[0].resolve(true);await settle();
 assert.deepEqual(Array.from(h.state[key],row=>row.id),['keep','new']);assert.equal(h.writes.length,1);
 if(kind==='ssh')assert.deepEqual(Array.from(h.state.terminalRecent,row=>row.id),['telnet','ssh-new']);
});

test('remote permission prompts preserve the exact selected connection and file until approval',async()=>{
 for(const change of ['cancel','connection','revision','listed-revision','folder','selection','invalid','accept']){
  const h=harness('remote-browser'),rb=h.root.NetworkToolkitRemoteBrowser,m=rb.get('ssh');h.state.activePage='sftp';
  let revision=1;h.root.terminalSessionRevision=()=>revision;
  Object.assign(m,{connected:true,sessionId:'ssh-one',sessionOptions:[{id:'ssh-one',revision:1}],active:'remote'});Object.assign(m.remote,{path:'/config',entries:[{name:'startup.cfg',path:'/config/startup.cfg'},{name:'other.cfg',path:'/config/other.cfg'}],selected:new Set(['/config/startup.cfg'])});
  const node=h.mount('<div data-rb-root="ssh"><button data-rb-action="permissions">Permissions</button></div>').querySelector('button');
  h.document.emit('click',node);assert.equal(h.questions.length,1);assert.equal(h.requests.length,0);
  if(change==='connection')m.sessionId='ssh-two';if(change==='revision')revision++;if(change==='listed-revision')m.sessionOptions[0].revision++;
  if(change==='folder')m.remote.path='/other';if(change==='selection')m.remote.selected=new Set(['/config/other.cfg']);
  h.questions[0].resolve(change==='cancel'?null:change==='invalid'?'9999':'0600');await settle();
  assert.equal(h.requests.length,change==='accept'?1:0,change);
  if(change==='accept')assert.deepEqual(JSON.parse(JSON.stringify(h.requests[0])),{path:'/remote-browser',method:'POST',body:{protocol:'ssh',action:'chmod',sessionId:'ssh-one',path:'/config/startup.cfg',permissions:'0600'}});
 }
});
