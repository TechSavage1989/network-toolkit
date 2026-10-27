'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync(require.resolve('../web/assets/credential-manager.js'),'utf8');
const endpoint={protocol:'ssh',host:'switch.test',port:22,username:'operator'};
function harness(){const records=new Map(),calls=[],state={},fields={},document={querySelector:selector=>fields[selector]||null,addEventListener(){},dispatchEvent(){}};
 const root={state,document,URL,OFFLINE:false,console,esc:String,icon:()=>'',CustomEvent:class{constructor(type,options){this.type=type;this.detail=options?.detail;}},api:async(path,method,body)=>{
  calls.push({path,method,body:body&&JSON.parse(JSON.stringify(body))});
  if(path==='/credentials'&&method==='POST'){const id=body.id||'credential-'+(records.size+1),record={...records.get(id),...body,id};records.set(id,record);const {secret,...entry}=record;return {entry};}
  if(path==='/credentials')return {vault:{available:true,state:'unlocked'},entries:[...records.values()].map(({secret,...record})=>record)};
  if(path.endsWith('/bindings')&&method==='POST'){const item=records.get(path.split('/')[2]);item.bindings=[...(item.bindings||[]),body.binding];return {entry:item};}
  throw Error('Unexpected credential request '+path+' '+method);
 }};root.window=root;vm.runInNewContext(source.replace('root.NetworkToolkitCredentials=Object.freeze','root.__test={renderSecret,editorHTML,saveForm};root.NetworkToolkitCredentials=Object.freeze'),root);return {root,tool:root.NetworkToolkitCredentials,records,calls,state,fields};}
test('shared password save/update binds endpoint and prepares only a credential reference',async()=>{
 const h=harness(),config={...endpoint,name:'Core login',authMethod:'password',password:'first-fixture-password'};
 const created=await h.tool.saveConnection(config);
 const updated=await h.tool.saveConnection({...config,credentialId:created.id,password:'updated-fixture-password'});
 assert.equal(created.id,updated.id);assert.equal(h.records.size,1);
 assert.deepEqual(JSON.parse(JSON.stringify(h.records.get(created.id).bindings)),[endpoint]);
 const result=await h.tool.prepare({...endpoint,credentialId:updated.id});
 assert.deepEqual(JSON.parse(JSON.stringify(result)),{credentialId:created.id,username:'operator',authMethod:'password'});
 assert.equal(h.tool.resolve,undefined);assert.equal(h.tool.meta(updated.id).secret,undefined);
 assert(h.calls.every(call=>call.path==='/credentials'));assert(!JSON.stringify(h.state).includes('fixture-password'));
});
test('SSH key and passphrase save never return key material from prepare or metadata',async()=>{
 const h=harness(),entry=await h.tool.saveConnection({...endpoint,name:'SSH key',authMethod:'key',privateKey:'fixture-private-key',passphrase:'fixture-passphrase'});
 const result=await h.tool.prepare({...endpoint,credentialId:entry.id});assert.equal(result.authMethod,'key');
 for(const field of ['password','privateKey','passphrase','secret']){assert.equal(result[field],undefined);assert.equal(h.tool.meta(entry.id)[field],undefined);}
 assert.equal(JSON.parse(h.records.get(entry.id).secret).privateKey,'fixture-private-key');
});
test('unapproved host, port, username and transport cannot silently reuse a saved credential',async()=>{
 const h=harness(),entry=await h.tool.saveConnection({...endpoint,name:'Bound',password:'fixture'});
 for(const changed of [{host:'other.test'},{port:2222},{username:'other-user'},{protocol:'ftp-explicit'}])await assert.rejects(()=>h.tool.prepare({...endpoint,...changed,credentialId:entry.id}),/not approved/);
 assert.equal(h.calls.filter(x=>x.path.endsWith('/bindings')).length,0);
});
test('legacy unbound credential remains unusable until explicit native approval request completes',async()=>{
 const h=harness();h.records.set('legacy',{id:'legacy',name:'Legacy',kind:'password',username:'operator',secret:'legacy-canary'});await h.tool.refresh();
 await assert.rejects(()=>h.tool.prepare({...endpoint,credentialId:'legacy'}),/Approve destination/);
 assert.equal(h.calls.filter(x=>x.method==='POST').length,0);
 const approved=await h.tool.approveConnection({...endpoint,credentialId:'legacy'});assert.equal(approved.credentialId,'legacy');
 const call=h.calls.find(x=>x.path==='/credentials/legacy/bindings');assert.deepEqual(call.body,{binding:endpoint});assert(!JSON.stringify(call).includes('legacy-canary'));
});
test('native approval rejection leaves legacy credential unbound and sends no connection request',async()=>{
 const h=harness();h.records.set('legacy',{id:'legacy',kind:'password',username:'operator'});await h.tool.refresh();const original=h.root.api;
 h.root.api=async(path,...rest)=>{if(path.endsWith('/bindings'))throw Error('Destination approval cancelled.');return original(path,...rest);};
 await assert.rejects(()=>h.tool.approveConnection({...endpoint,credentialId:'legacy'}),/cancelled/);
 await assert.rejects(()=>h.tool.prepare({...endpoint,credentialId:'legacy'}),/not approved/);
});
test('duplicate approval clicks share one pending native confirmation',async()=>{
 const h=harness();h.records.set('legacy',{id:'legacy',kind:'password',username:'operator'});await h.tool.refresh();const original=h.root.api;let finish,count=0;
 h.root.api=async(path,...rest)=>{if(path.endsWith('/bindings')){count++;await new Promise(resolve=>finish=resolve);}return original(path,...rest);};
 const one=h.tool.approveConnection({...endpoint,credentialId:'legacy'}),two=h.tool.approveConnection({...endpoint,credentialId:'legacy'});assert.equal(count,1);finish();await Promise.all([one,two]);assert.equal(count,1);
});
test('plain FTP saved-credential use/save/approval all fail before submission',async()=>{
 const h=harness();h.records.set('id',{id:'id',kind:'password',username:'operator'});await h.tool.refresh();const config={...endpoint,protocol:'ftp',security:'plain',credentialId:'id',password:'fixture'};
 await assert.rejects(()=>h.tool.prepare(config),/Plain FTP/);await assert.rejects(()=>h.tool.approveConnection(config),/Plain FTP/);await assert.rejects(()=>h.tool.saveConnection(config),/Plain FTP/);
 assert.equal(h.calls.filter(x=>x.method==='POST').length,0);
});
test('endpoint comparison normalizes DNS case, terminal dot and IPv6 brackets without wildcards',()=>{
 const h=harness();assert.deepEqual(JSON.parse(JSON.stringify(h.tool.endpoint({...endpoint,host:' SWITCH.TEST. '}))),endpoint);
 assert.equal(h.tool.endpoint({...endpoint,host:'[2001:DB8::1]'}).host,'2001:db8::1');
 for(const host of ['','https://host.test','operator@host.test','host.test/path','host test'])assert.throws(()=>h.tool.endpoint({...endpoint,host}),/valid host/);
});
test('missing credentials and empty replacements fail before update submission',async()=>{
 const h=harness();await assert.rejects(()=>h.tool.saveConnection({...endpoint,credentialId:'removed',password:'fixture'}),/unavailable/);
 await assert.rejects(()=>h.tool.saveConnection({...endpoint,password:''}),/Enter the password/);
 await assert.rejects(()=>h.tool.saveConnection({...endpoint,authMethod:'key',privateKey:''}),/Enter or load the private key/);
 assert.equal(h.calls.filter(x=>x.method==='POST').length,0);
});
test('unexpected secret fields in metadata responses are discarded and editor never populates them',async()=>{
 const h=harness();h.root.api=async()=>({vault:{available:true,state:'unlocked'},entries:[{id:'id',kind:'password',secret:'MUST-NOT-RENDER',password:'MUST-NOT-RENDER',privateKey:'MUST-NOT-RENDER'}]});await h.tool.refresh();
 assert(!JSON.stringify(h.tool.meta('id')).includes('MUST-NOT-RENDER'));
 const node={innerHTML:''};h.fields['#credentialSecretFields']=node;h.fields['#credentialKind']={value:'password'};h.root.__test.renderSecret({id:'id',kind:'password',secret:'MUST-NOT-RENDER'});
 assert(!node.innerHTML.includes('MUST-NOT-RENDER'));assert.match(node.innerHTML,/never displayed/);assert.doesNotMatch(node.innerHTML,/data-credential-action="reveal"/);
 h.fields['#credentialKind'].value='ssh-key';h.root.__test.renderSecret({id:'id',kind:'ssh-key',secret:JSON.stringify({privateKey:'MUST-NOT-RENDER',passphrase:'MUST-NOT-RENDER'})});assert(!node.innerHTML.includes('MUST-NOT-RENDER'));
});
test('destination metadata and explicit approval preserve the approved server fingerprint',async()=>{
 const h=harness(),binding={...endpoint,fingerprint:'SHA256:synthetic-pin'};
 const entry=await h.tool.saveConnection({...binding,password:'fixture'});
 assert.equal(h.records.get(entry.id).bindings[0].fingerprint,binding.fingerprint);assert.equal(h.tool.meta(entry.id).bindings[0].fingerprint,binding.fingerprint);
 assert.equal((await h.tool.prepare({...endpoint,credentialId:entry.id})).credentialId,entry.id);
 h.records.set('legacy',{id:'legacy',kind:'password',username:'operator',bindings:[]});await h.tool.refresh();await h.tool.approveConnection({...binding,credentialId:'legacy'});
 assert.equal(h.calls.find(row=>row.path==='/credentials/legacy/bindings').body.binding.fingerprint,binding.fingerprint);
});

test('locked or uninitialized vault drops metadata and refuses saved credential use',async()=>{
 for(const state of ['locked','legacy','setup-required','unavailable']){
  const h=harness();h.root.api=async()=>({vault:{available:state!=='unavailable',state},entries:[{id:'id',name:'HIDDEN_WHILE_LOCKED',kind:'password',username:'operator',bindings:[endpoint]}]});
  await h.tool.refresh();assert.equal(h.tool.meta('id'),null);assert(!h.tool.options().includes('HIDDEN_WHILE_LOCKED'));
  await assert.rejects(()=>h.tool.prepare({...endpoint,credentialId:'id'}),/locked|Set up/);
  await assert.rejects(()=>h.tool.saveConnection({...endpoint,password:'typed'}),/locked|Set up/);
 }
});
test('native vault actions send only the action and clear editor secrets after successful lock',async()=>{
 const h=harness();h.records.set('id',{id:'id',kind:'password',username:'operator',bindings:[endpoint]});await h.tool.refresh();
 for(const id of ['credentialSecret','credentialPrivateKey','credentialPassphrase'])h.fields['#'+id]={value:'TRANSIENT_EDITOR_CANARY'};
 h.root.api=async(path,method,body)=>{h.calls.push({path,method,body});return path==='/credential-vault'?{available:true,state:'locked',remembered:false}:{vault:{available:true,state:'locked'},entries:[]};};
 await h.tool.vaultAction('lock');assert.equal(h.tool.meta('id'),null);assert.equal(h.tool.vaultStatus().state,'locked');
 assert.deepEqual(JSON.parse(JSON.stringify(h.calls.find(x=>x.path==='/credential-vault').body)),{action:'lock'});
 for(const id of ['credentialSecret','credentialPrivateKey','credentialPassphrase'])assert.equal(h.fields['#'+id].value,'');
 assert(!JSON.stringify(h.calls).includes('TRANSIENT_EDITOR_CANARY'));
});
test('cancelled native vault operation preserves current editor input and unlocked metadata',async()=>{
 const h=harness();h.records.set('id',{id:'id',kind:'password'});await h.tool.refresh();h.fields['#credentialSecret']={value:'keep-typed-input'};
 const original=h.root.api;h.root.api=async(path,...args)=>{if(path==='/credential-vault')throw Error('Native vault dialog cancelled');return original(path,...args);};await assert.rejects(()=>h.tool.vaultAction('change-password'),/cancelled/);
 assert.equal(h.fields['#credentialSecret'].value,'keep-typed-input');assert.equal(h.tool.meta('id').id,'id');assert.equal(h.tool.vaultStatus().state,'unlocked');
});
test('duplicate vault clicks share one native operation and stale metadata response cannot undo lock',async()=>{
 const h=harness();let finishOld,finishAction,posts=0,gets=0;
 h.root.api=async(path,method)=>{if(path==='/credential-vault'){posts++;await new Promise(resolve=>finishAction=resolve);return {available:true,state:'locked'};}if(++gets===1)return new Promise(resolve=>finishOld=resolve);return {vault:{available:true,state:'locked'},entries:[]};};
 const old=h.tool.refresh(),one=h.tool.vaultAction('lock'),two=h.tool.vaultAction('lock');assert.equal(posts,1);finishAction();await Promise.all([one,two]);
 finishOld({vault:{available:true,state:'unlocked'},entries:[{id:'stale',kind:'password'}]});await old;
 assert.equal(h.tool.vaultStatus().state,'locked');assert.equal(h.tool.meta('stale'),null);
});
test('vault UI has no master-password or recovery input fields and unknown actions are rejected',async()=>{
 const h=harness();await assert.rejects(()=>h.tool.vaultAction('arbitrary'),/Unknown/);assert.equal(h.calls.length,0);
 assert.doesNotMatch(source,/id=["'](?:masterPassword|recoveryKey)|type=["']password[^\n]*master/i);
 assert.match(source,/api\('\/credential-vault','POST',\{action\}\)/);
 assert.match(source,/Resetting app configuration forgets automatic unlock but keeps encrypted vault files/);
});
test('partly completed lock failure still removes rendered secrets and stale metadata',async()=>{
 const h=harness();h.records.set('id',{id:'id',kind:'password'});await h.tool.refresh();h.fields['#credentialSecret']={value:'MUST_CLEAR_AFTER_PARTIAL_LOCK'};
 h.root.api=async(path)=>{if(path==='/credential-vault')throw Error('Remembered record removal failed');return {vault:{available:true,state:'locked',warning:'Future launch may still unlock'},entries:[]};};
 await assert.rejects(()=>h.tool.vaultAction('lock'),/removal failed/);
 assert.equal(h.fields['#credentialSecret'].value,'');assert.equal(h.tool.meta('id'),null);assert.equal(h.tool.vaultStatus().state,'locked');
});

test('missing vault status never turns metadata-only fixture data into an unlocked vault',async()=>{
 const h=harness();h.root.api=async()=>({entries:[{id:'id',name:'MISSING_STATUS_MUST_HIDE',kind:'password',username:'operator',bindings:[endpoint]}]});
 await h.tool.refresh();assert.equal(h.tool.meta('id'),null);assert(!h.tool.options().includes('MISSING_STATUS_MUST_HIDE'));assert.equal(h.tool.vaultStatus().available,false);
 await assert.rejects(()=>h.tool.prepare({...endpoint,credentialId:'id'}),/locked|Set up/);
 assert.equal(h.calls.filter(row=>row.method==='POST').length,0);
});
