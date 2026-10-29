'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8');
const saved=(id,{credentialId='vault-password',fingerprint='SHA256:known',authMethod='password',host='192.0.2.100'}={})=>({id,type:'ssh',name:'Test SSH host',host,port:22,username:'operator',authMethod,credentialId,fingerprint});
function harness({profile=saved('test'),trusted=true}={}){
 const state={terminalProfiles:[profile],remoteBrowserSettings:{},activePage:'editor'};
 const events=[],notifications=[],dialogs=[],writes=[];
 const document={addEventListener(){},getElementById(id){return id==='pageContent'?{set innerHTML(value){throw Error('Editor page clobbered');}}:null;},querySelector(){return null;}};
 const root={document,state,Map,Set,Date,Math,JSON,Number,String,console,esc:String,icon:()=>'',saveState:()=>writes.push(JSON.stringify(state)),
   window:null,api:async(path,method,body)=>{
     events.push({path,method,body});
     if(path==='/terminals/known-hosts')return {hosts:trusted?[{host:profile.host+':22',fingerprint:'SHA256:known'}]:[]};
     if(path==='/remote-browser'&&body?.action==='fingerprint')return {fingerprint:'SHA256:observed'};
     return {connectionId:'owned-connection',path:'/',entries:[]};
   },openModal:(title,body,footer)=>dialogs.push({title,body,footer}),toast:(msg)=>notifications.push(msg),
   NetworkToolkitEditorFiles:{detached:()=>false,showConnection:k=>events.push({mounted:k})},
   NetworkToolkitCredentials:{prepare:async record=>{if(record.credentialId==='vault-fails')throw Error('credential missing');return {credentialId:record.credentialId,authMethod:'password',username:'operator'};}}
 };
 root.window=root;root.globalThis=root;
 vm.createContext(root);vm.runInContext(source,root);
 const browser=root.NetworkToolkitRemoteBrowser;
 browser.get('ssh').saved=true;
 return {root,profile,state,browser,events,dialogs,writes};
}

test('editor saved SSH mounts before existing verification and never replaces editor page',async()=>{
 const h=harness({trusted:false});const result=await h.browser.useSavedEditorProfile('ssh','test');
 assert.equal(h.events[0].mounted,'ssh');assert.equal(result.pending,true);assert.equal(h.state.activePage,'editor');
 assert.match(h.dialogs[0].body,/Independently verified/);assert(!h.events.some(e=>e.body?.action==='connect'));
});
test('editor manual authentication uses existing visible sign-in; cancellation does not connect',async()=>{
 const h=harness({profile:saved('test',{credentialId:''})});const result=await h.browser.useSavedEditorProfile('ssh','test');
 assert(result.pending);assert.match(h.dialogs[0].body,/rbSSHProfilePassword/);assert(!h.events.some(e=>e.body?.action==='connect'));
});
test('editor linked SSH uses credential ID only and preserves document/page state',async()=>{
 const h=harness();h.state.editorCanary='dirty-document-retained';const result=await h.browser.useSavedEditorProfile('ssh','test');
 assert.equal(result.source,'client:ssh');assert.equal(h.state.editorCanary,'dirty-document-retained');assert.equal(h.state.activePage,'editor');
 const sent=h.events.find(e=>e.body?.action==='connect').body;assert.equal(sent.credentialId,'vault-password');assert.equal(sent.password,undefined);
});
test('editor rejects replacing active clients and every existing pending operation',async()=>{
 for(const flag of ['connected','connecting','busy','profileSelectionPending','pendingProfileLoad','pendingCredentialSave','pendingClose','pendingKeyRead']){
  const h=harness();h.browser.get('ssh')[flag]=true;await assert.rejects(h.browser.useSavedEditorProfile('ssh','test'),/active connection|operation/);assert.equal(h.events.length,0);
 }
});
test('editor reuses same live selected profile without touching transport',async()=>{
 const h=harness(),m=h.browser.get('ssh');m.connected=true;m.profileId='test';m.connectionId='original';Object.assign(h.browser.configurations.ssh,h.profile);
 const result=await h.browser.useSavedEditorProfile('ssh','test');assert.equal(result.source,'client:ssh');assert.equal(m.connectionId,'original');assert.equal(h.events.length,0);
});
test('editor profile change while matching an active session never redirects authentication',async()=>{
 const h=harness();let resolve;h.root.api=()=>new Promise(r=>resolve=r);
 const pending=h.browser.useSavedEditorProfile('ssh','test');await Promise.resolve();h.state.terminalProfiles=[{...h.profile,host:'different.invalid'}];resolve({sessions:[]});
 await assert.rejects(pending,/changed/);assert.equal(h.browser.get('ssh').connected,false);assert.equal(h.dialogs.length,0);
});
test('editor profile change while native credential preparation waits fails closed',async()=>{
 const h=harness();let resolve,started;const ready=new Promise(r=>started=r);h.root.NetworkToolkitCredentials.prepare=()=>{started();return new Promise(r=>resolve=r)};
 const pending=h.browser.useSavedEditorProfile('ssh','test');await ready;h.state.terminalProfiles=[{...h.profile,host:'different.invalid'}];resolve({credentialId:'vault-password'});
 await assert.rejects(pending,/changed/);assert(!h.events.some(e=>e.body?.action==='connect'));
});
test('editor FTP manual sign-in reveals existing settings without implicit connection',async()=>{
 const h=harness();h.state.ftpProfiles=[{id:'ftp1',name:'FTP manual',host:'ftp.invalid',port:21,username:'operator',security:'plain',credentialId:''}];h.browser.get('ftp').saved=true;
 const result=await h.browser.useSavedEditorProfile('ftp','ftp1');assert(result.pending);assert.equal(h.events[0].mounted,'ftp');assert.equal(h.browser.get('ftp').showSettings,true);assert(!h.events.some(e=>e.body));
});
test('detached editor cannot invoke main connection wrapper or FTP sidebar mount',async()=>{
 const h=harness();h.root.NetworkToolkitEditorFiles.detached=()=>true;
 await assert.rejects(h.browser.useSavedEditorProfile('ssh','test'),/main window/);assert.throws(()=>h.browser.mountSidebar({},'remote','ftp'),/approved SFTP/);assert.equal(h.events.length,0);
});
test('late remote-browser render after sidebar closes cannot overwrite another page',()=>{
 const h=harness();for(const page of ['editor','terminal','discover']){h.state.activePage=page;h.browser.render('ssh');h.browser.render('ftp');}
});

test('stale FTPS to plain FTP selection refuses before UI or authentication',async()=>{
 const h=harness();h.state.ftpProfiles=[{id:'ftp1',name:'Files',host:'ftp.invalid',port:21,username:'operator',security:'plain'}];
 await assert.rejects(h.browser.useSavedEditorProfile('ftp','ftp1',{name:'Files',host:'ftp.invalid',port:21,protocol:'ftps'}),/changed/);assert.equal(h.events.length,0);
});
test('same profile with changed FTP security cannot be reused as if new protocol is active',async()=>{
 const h=harness();h.state.ftpProfiles=[{id:'ftp1',name:'Files',host:'ftp.invalid',port:21,username:'operator',security:'explicit'}];const m=h.browser.get('ftp');m.connected=true;m.profileId='ftp1';Object.assign(h.browser.configurations.ftp,{...h.state.ftpProfiles[0],security:'plain'});
 await assert.rejects(h.browser.useSavedEditorProfile('ftp','ftp1'),/active connection/);assert.equal(h.events.length,0);
});
test('late render cannot replace FTP or SFTP server pages',()=>{
 const h=harness();h.root.NetworkToolkitTransfers={mode:()=> 'server'};for(const page of ['sftp','ftp']){h.state.activePage=page;h.browser.render('ssh');h.browser.render('ftp');}
});

test('editor configuration changed while credential preparation waits cannot redirect login',async()=>{
 const h=harness();let resolve,started;const ready=new Promise(r=>started=r);h.root.NetworkToolkitCredentials.prepare=()=>{started();return new Promise(r=>resolve=r)};
 const pending=h.browser.useSavedEditorProfile('ssh','test');await ready;h.browser.configurations.ssh.host='changed-during-approval.invalid';h.browser.get('ssh').profileId='';h.browser.get('ssh').sourceChoice='manual';resolve({credentialId:'vault-password'});
 await assert.rejects(pending,/changed/);assert(!h.events.some(e=>e.body?.action==='connect'));assert.equal(h.browser.get('ssh').editorConnectionPending,false);
});

test('editor saved profile can reuse an existing matching authenticated terminal by ID only',async()=>{
 const h=harness(),original=h.root.api;h.root.api=async(path,method,body)=>path==='/remote-browser'&&!method?{sessions:[{id:'existing',host:h.profile.host,port:22,username:'operator'}]}:original(path,method,body);
 const result=await h.browser.useSavedEditorProfile('ssh','test');assert.equal(result.source,'client:ssh');assert.equal(h.browser.get('ssh').sessionId,'existing');assert(!h.events.some(e=>e.body?.action==='connect'));assert(h.events.filter(e=>e.body?.action==='list').every(e=>e.body.sessionId==='existing'&&!e.body.password));
});
