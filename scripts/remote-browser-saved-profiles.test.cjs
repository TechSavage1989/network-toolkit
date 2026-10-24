'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8');
function harness({authMethod='password'}={}){
 const profile={id:'router-one',name:'Lab Router',type:'ssh',host:'192.0.2.44',port:2202,username:'operator',authMethod,credentialId:'win-credential-id'};
 const state={terminalProfiles:[profile],remoteBrowserSettings:{}};
 const outgoing=[],saved=[];
 const document={addEventListener(){},getElementById(){return null;},querySelector(){return null;}};
 const root={state,document,Map,Set,Date,Math,JSON,Number,String,console,esc:String,icon:()=>'',saveState:()=>saved.push(JSON.stringify(state)),
   api:async(path,method,body)=>{outgoing.push({path,method,body});if(path==='/terminals/known-hosts')return{hosts:[{host:'192.0.2.44:2202',fingerprint:'SHA256:trusted'}]};return {entries:[]};},
   NetworkToolkitCredentials:{resolve:async given=>{assert.equal(given.id,profile.id);return authMethod==='key'?{authMethod:'key',username:'operator',privateKey:'-----BEGIN OPENSSH PRIVATE KEY-----\nredacted\n-----END OPENSSH PRIVATE KEY-----',passphrase:'key-passphrase'}:{authMethod:'password',username:'operator',password:'vault-only-secret'};}}
 };
 root.window=root;root.globalThis=root;
 vm.createContext(root);vm.runInContext(source,root);root.NetworkToolkitRemoteBrowser.get('ssh').saved=true;
 return {root,profile,state,outgoing,saved,api:root.NetworkToolkitRemoteBrowser};
}
test('a saved SSH terminal profile supplies SFTP endpoint, trusted host fingerprint and vault password only to the outbound request',async()=>{
 const h=harness();await h.api.loadProfile('ssh',h.profile.id);
 const config=h.api.configurations.ssh;
 assert.equal(config.host,'192.0.2.44');assert.equal(config.port,2202);assert.equal(config.username,'operator');assert.equal(config.fingerprint,'SHA256:trusted');assert.equal(h.api.get('ssh').profileId,'router-one');
 await h.api.request('ssh','connect',{path:'.'});
 const operation=h.outgoing.at(-1).body;
 assert.equal(operation.protocol,'ssh');assert.equal(operation.password,'vault-only-secret');assert.equal(operation.username,'operator');assert.equal(operation.fingerprint,'SHA256:trusted');
 assert(h.saved.every(value=>!value.includes('vault-only-secret')));
 assert(!JSON.stringify(h.state).includes('vault-only-secret'));
});
test('saved SSH terminal private keys are forwarded transiently as inline key material rather than requiring a key file',async()=>{
 const h=harness({authMethod:'key'});await h.api.loadProfile('ssh',h.profile.id);await h.api.request('ssh','connect',{path:'startup.cfg'});
 const operation=h.outgoing.at(-1).body;
 assert.equal(operation.authMode,'key');assert.equal(operation.passphrase,'key-passphrase');assert(operation.privateKey.includes('BEGIN OPENSSH PRIVATE KEY'));assert.equal(operation.keyPath,'');
 assert(!JSON.stringify(h.state).includes('PRIVATE KEY'));assert(!JSON.stringify(h.state).includes('key-passphrase'));
});
test('changed SSH host and port invalidate a previously selected saved profile before contacting a remote server',async()=>{
 const h=harness();await h.api.loadProfile('ssh',h.profile.id);
 h.api.configurations.ssh.host='192.0.2.45';
 await assert.rejects(h.api.request('ssh','connect',{path:'.'}),/Saved SSH profile changed/);
 assert.equal(h.outgoing.filter(x=>x.path==='/remote-browser').length,0);
});
