'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync(require.resolve('../web/assets/credential-manager.js'),'utf8');
function harness(){const records=new Map(),calls=[],state={},document={querySelector:()=>null,addEventListener(){},dispatchEvent(){}};
 const root={state,document,OFFLINE:false,console,CustomEvent:class{constructor(type,options){this.type=type;this.detail=options?.detail;}},api:async(path,method,body)=>{
  calls.push({path,method,body});
  if(path==='/credentials'&&method==='POST'){const id=body.id||'credential-'+(records.size+1),record={...body,id};records.set(id,record);const {secret,...entry}=record;return {entry};}
  if(path==='/credentials')return {entries:[...records.values()].map(({secret,...record})=>record)};
  if(path.startsWith('/credentials/'))return {...records.get(path.split('/').at(-1))};
 }};root.window=root;vm.runInNewContext(source,root);return {tool:root.NetworkToolkitCredentials,records,calls,state};}
test('shared credential workflow creates and updates the same password entry',async()=>{
 const h=harness();const created=await h.tool.saveConnection({name:'Core login',host:'switch.test',username:'operator',authMethod:'password',password:'first-fixture-password'});
 const updated=await h.tool.saveConnection({credentialId:created.id,name:'Core login',host:'switch.test',username:'operator',authMethod:'password',password:'updated-fixture-password'});
 assert.equal(created.id,updated.id);assert.equal(h.records.size,1);
 const result=await h.tool.resolve({credentialId:updated.id,username:'different-user'});assert.equal(result.username,'different-user');assert.equal(result.password,'updated-fixture-password');
 assert.equal(h.tool.meta(updated.id).secret,undefined);assert(!JSON.stringify(h.state).includes('fixture-password'));
});
test('shared credential workflow saves SSH keys and passphrases without putting secrets in metadata',async()=>{
 const h=harness(),entry=await h.tool.saveConnection({name:'SSH key',username:'operator',authMethod:'key',privateKey:'fixture-private-key',passphrase:'fixture-passphrase'});
 const result=await h.tool.resolve({credentialId:entry.id});assert.equal(result.authMethod,'key');assert.equal(result.privateKey,'fixture-private-key');assert.equal(result.passphrase,'fixture-passphrase');
 assert.equal(h.tool.meta(entry.id).secret,undefined);
});
test('missing linked credentials and empty secrets fail before an update is submitted',async()=>{
 const h=harness();await assert.rejects(()=>h.tool.saveConnection({credentialId:'removed',authMethod:'password',password:'fixture'}),/unavailable/);
 await assert.rejects(()=>h.tool.saveConnection({authMethod:'password',password:''}),/Enter the password/);
 await assert.rejects(()=>h.tool.saveConnection({authMethod:'key',privateKey:''}),/Enter or load the private key/);
 assert.equal(h.calls.filter(x=>x.method==='POST').length,0);
});
