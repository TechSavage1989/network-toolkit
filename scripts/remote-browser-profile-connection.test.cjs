'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8');
const saved=(id,{credentialId='vault-password',fingerprint='SHA256:known',authMethod='password',host='192.0.2.100'}={})=>({id,type:'ssh',name:'Test SSH host',host,port:22,username:'operator',authMethod,credentialId,fingerprint});
function harness({profile=saved('test'),trusted=true}={}){
 const state={terminalProfiles:[profile],remoteBrowserSettings:{},activePage:'sftp'};
 const events=[],notifications=[],dialogs=[],writes=[];
 const document={addEventListener(){},getElementById(){return null;},querySelector(){return null;}};
 const root={document,state,Map,Set,Date,Math,JSON,Number,String,console,esc:String,icon:()=>'',saveState:()=>writes.push(JSON.stringify(state)),
   window:null,api:async(path,method,body)=>{
     events.push({path,method,body});
     if(path==='/terminals/known-hosts')return {hosts:trusted?[{host:profile.host+':22',fingerprint:'SHA256:known'}]:[]};
     if(path==='/remote-browser'&&body?.action==='fingerprint')return {fingerprint:'SHA256:observed'};
     return {connectionId:'owned-connection',path:'/',entries:[]};
   },openModal:(title,body,footer)=>dialogs.push({title,body,footer}),toast:(msg)=>notifications.push(msg),
   NetworkToolkitCredentials:{prepare:async record=>{if(record.credentialId==='vault-fails')throw Error('credential missing');return {credentialId:record.credentialId,authMethod:'password',username:'operator'};}}
 };
 root.window=root;root.globalThis=root;
 vm.createContext(root);vm.runInContext(source,root);
 const browser=root.NetworkToolkitRemoteBrowser;
 browser.get('ssh').saved=true;
 return {root,profile,state,browser,events,dialogs,writes};
}
test('Use connection completes saved SSH profile connection with linked vault password and a known host fingerprint',async()=>{
 const h=harness();h.browser.get('ssh').sourceChoice='profile:test';
 await h.browser.useSavedSSHProfile('test');
 assert.equal(h.browser.get('ssh').connected,true);
 assert.equal(h.browser.get('ssh').connecting,false);
 const sent=h.events.find(x=>x.path==='/remote-browser'&&x.body?.action==='connect').body;
 assert.equal(sent.host,'192.0.2.100');assert.equal(sent.port,22);
 assert.equal(sent.fingerprint,'SHA256:known');assert.equal(sent.credentialId,'vault-password');assert.equal(Object.hasOwn(sent,'password'),false);
 assert.equal(h.dialogs.length,0);
 assert(!JSON.stringify(h.state).includes('protected-vault-password'));
 assert(h.writes.every(x=>!x.includes('protected-vault-password')));
});
test('Use connection with saved SSH profile lacking a credential opens a sign-in form instead of a blank authentication attempt',async()=>{
 const h=harness({profile:saved('no-cred',{credentialId:''})});h.browser.get('ssh').sourceChoice='profile:no-cred';
 await h.browser.useSavedSSHProfile('no-cred');
 assert.equal(h.browser.get('ssh').connected,false);
 assert.equal(h.events.filter(x=>x.path==='/remote-browser'&&x.method==='POST').length,0);
 assert.match(h.dialogs[0].body,/id="rbSSHProfilePassword"/);
 assert.match(h.dialogs[0].footer,/Connect \/ Browse/);
});
test('Use connection with saved untrusted SSH host requires independent fingerprint verification before authentication',async()=>{
 const h=harness({trusted:false});h.browser.get('ssh').sourceChoice='profile:test';
 await h.browser.useSavedSSHProfile('test');
 assert.equal(h.browser.get('ssh').connected,false);
 assert.equal(h.events.filter(x=>x.path==='/remote-browser'&&x.body?.action==='connect').length,0);
 assert.match(h.dialogs[0].body,/id="rbSSHProfileVerifiedFingerprint"/);
 assert.equal(h.browser.configurations.ssh.fingerprint,'');
});
test('Use connection retry uses the same authenticated saved profile without clearing a password entered earlier',async()=>{
 const h=harness({profile:saved('no-cred',{credentialId:''})});const m=h.browser.get('ssh');m.sourceChoice='profile:no-cred';
 await h.browser.loadProfile('ssh','no-cred');
 h.browser.configurations.ssh.password='temporary-in-memory';m.profileManualAuth=true;
 await h.browser.useSavedSSHProfile('no-cred');
 assert.equal(m.connected,true);assert.equal(h.dialogs.length,0);
 assert.equal(h.events.find(x=>x.path==='/remote-browser'&&x.body?.action==='connect').body.password,'temporary-in-memory');
 assert(!JSON.stringify(h.state).includes('temporary-in-memory'));
});
test('saved SSH profile reuses a matching authenticated terminal SFTP connection without requesting new credentials',async()=>{
 const h=harness({profile:saved('live',{credentialId:'',fingerprint:'SHA256:outdated'}),trusted:false});
 const original=h.root.api;
 h.root.api=async(path,method,body)=>{
  if(path==='/remote-browser'&&method===undefined)return {sessions:[{id:'active-xyz',host:'192.0.2.100',port:22,username:'operator',label:'Active terminal'}]};
  return original(path,method,body);
 };
 h.browser.get('ssh').sourceChoice='profile:live';
 await h.browser.useSavedSSHProfile('live');
 const model=h.browser.get('ssh');
 assert.equal(model.connected,true);
 assert.equal(model.sessionId,'active-xyz');
 assert.equal(h.dialogs.length,0);
 const calls=h.events.filter(x=>x.path==='/remote-browser'&&x.body?.action==='list');
 assert(calls.length>=1);
 assert(calls.every(x=>x.body.sessionId==='active-xyz'&&!x.body.password&&!x.body.fingerprint));
});
test('terminal trusted host fingerprint takes precedence over stale SFTP preference and old profile fingerprint',async()=>{
 const h=harness({profile:saved('test',{fingerprint:'SHA256:oldStale'}),trusted:true});
 h.state.sftpVerifiedHostKeys={'192.0.2.100:22':'SHA256:otherStale'};
 h.browser.get('ssh').sourceChoice='profile:test';
 await h.browser.useSavedSSHProfile('test');
 assert.equal(h.browser.get('ssh').connected,true);
 const sent=h.events.find(x=>x.path==='/remote-browser'&&x.body?.action==='connect').body;
 assert.equal(sent.fingerprint,'SHA256:known');
 assert.equal(h.dialogs.length,0);
});
test('FTP and FTPS library shares saved profile and recent entries with terminal connection collection',()=>{
 const h=harness();
 h.state.ftpProfiles=[{id:'f-save',name:'Main switch backup',host:'192.0.2.31',port:21,username:'backup',security:'explicit',credentialId:'vault-ftp'}];
 h.state.ftpRecent=[{id:'f-recent',name:'Old router',host:'192.0.2.32',port:990,username:'backup',security:'implicit',lastUsed:1000}];
 assert.deepEqual(Array.from(h.browser.libraryRows('ftp','saved'),x=>x.id),['f-save']);
 assert.deepEqual(Array.from(h.browser.libraryRows('ftp','recent'),x=>x.id),['f-recent']);
 assert.deepEqual(Array.from(h.browser.libraryRows('ssh','saved'),x=>x.id),['test']);
});


test('saved SSH profile inherits legacy SSH compatibility in standalone SFTP requests',async()=>{
 const h=harness({profile:{...saved('legacy'),sshCompatibility:'legacy'}});
 h.browser.get('ssh').sourceChoice='profile:legacy';
 await h.browser.useSavedSSHProfile('legacy');
 assert.equal(h.browser.get('ssh').connected,true);
 const sent=h.events.find(x=>x.path==='/remote-browser'&&x.body?.action==='connect').body;
 assert.equal(sent.sshCompatibility,'legacy');
});

test('new SFTP connection defaults to modern SSH algorithm compatibility',async()=>{
 const h=harness();h.browser.get('ssh').sourceChoice='profile:test';
 await h.browser.useSavedSSHProfile('test');
 const sent=h.events.find(x=>x.path==='/remote-browser'&&x.body?.action==='connect').body;
 assert.equal(sent.sshCompatibility,'modern');
});
