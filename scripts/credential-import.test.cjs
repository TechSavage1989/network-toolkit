'use strict';
// The password manager's offer to import saved credentials from the vault an
// earlier version kept for the standalone Terminal (or Text Editor). The
// previous vault's password is asked by the native dialog only; the page
// sends an action and a source, never a secret.
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync(require.resolve('../web/assets/credential-manager.js'),'utf8');
const endpoint={protocol:'ssh',host:'switch.test',port:22,username:'operator'};
function harness({state='unlocked',previous=[{source:'terminal',label:'Terminal',imported:false}],entries=[]}={}){
 const calls=[],toasts=[],modals=[],events=[],document={querySelector:()=>null,querySelectorAll:()=>[],addEventListener(){},dispatchEvent:event=>events.push(event)};
 const server={state,previous,entries,importResult:{ok:true,state:'unlocked',available:true,imported:2,alreadyPresent:1,skipped:0}};
 const root={state:{},document,URL,OFFLINE:false,console,esc:String,icon:()=>'',toast:(message,type)=>toasts.push([message,type]),openModal:(title,body)=>modals.push(body),CustomEvent:class{constructor(type,options){this.type=type;this.detail=options?.detail;}},api:async(path,method,body)=>{
  calls.push({path,method,body:body&&JSON.parse(JSON.stringify(body))});
  if(path==='/credentials'&&!method)return {vault:{available:true,state:server.state},entries:server.entries,previousVaults:server.previous};
  if(path==='/credential-vault'&&method==='POST'&&body.action==='import-previous'){if(server.fail)throw Error(server.fail);server.previous=server.previous.map(item=>item.source===body.source?{...item,imported:true}:item);server.entries=[...server.entries,{id:'cred-terminal',name:'Terminal only',kind:'password',bindings:[endpoint]}];return {...server.importResult,previousVaults:server.previous};}
  throw Error('Unexpected credential request '+path+' '+method);
 }};root.window=root;vm.runInNewContext(source.replace('root.NetworkToolkitCredentials=Object.freeze','root.__test={vaultControlsHTML,previousVaultsHTML};root.NetworkToolkitCredentials=Object.freeze'),root);
 return {root,tool:root.NetworkToolkitCredentials,calls,toasts,modals,events,server};
}
test('a previous Terminal vault is offered for import in plain words until it has been imported',async()=>{
 const h=harness();await h.tool.refresh();
 const html=h.root.__test.vaultControlsHTML();
 assert(html.includes('Import saved credentials from the previous Terminal vault'));assert(html.includes('data-vault-import="terminal"'));
 assert(html.includes('leaves the previous vault as it is'));assert(!/<input[^>]*type="password"/.test(html),'the page never asks for the previous vault\'s password itself');
 assert.deepEqual(JSON.parse(JSON.stringify(h.tool.previousVaults())),[{source:'terminal',label:'Terminal',imported:false}]);
 await h.tool.importPrevious('terminal');
 const sent=h.calls.find(call=>call.path==='/credential-vault');
 assert.deepEqual(sent.body,{action:'import-previous',source:'terminal'},'only an action and a source are sent');
 assert.match(h.toasts.at(-1)[0],/2 saved credentials were imported\. 1 was already in the shared vault\. The previous vault was not changed\./);
 assert.equal(h.root.__test.previousVaultsHTML(),'','an imported vault is no longer offered');
 assert(h.tool.meta('cred-terminal'),'the imported credential is listed');
 assert(h.events.some(event=>event.type==='credentials-updated'));
});
test('the import waits for an unlocked shared vault and reports a wrong password without changing anything',async()=>{
 const locked=harness({state:'locked'});await locked.tool.refresh();
 const html=locked.root.__test.previousVaultsHTML();
 assert(html.includes('disabled'));assert(html.includes('Set up or unlock the shared vault first.'));
 await assert.rejects(()=>locked.tool.importPrevious('terminal'),/locked/);assert(!locked.calls.some(call=>call.path==='/credential-vault'));
 const h=harness();await h.tool.refresh();h.server.fail='That master password did not open the previous Terminal vault. Nothing was imported and neither vault was changed.';
 await assert.rejects(()=>h.tool.importPrevious('terminal'),/did not open the previous Terminal vault/);
 assert(h.root.__test.previousVaultsHTML().includes('data-vault-import="terminal"'),'the offer stays after a failed import');
 assert.equal(h.tool.meta('cred-terminal'),null);
});
test('a connection whose credential is still in the previous vault says so instead of failing obscurely',async()=>{
 const h=harness();await h.tool.refresh();
 await assert.rejects(()=>h.tool.prepare({...endpoint,credentialId:'cred-terminal'}),/still be in the previous Terminal vault: open Password manager and choose Import saved credentials from the previous Terminal vault/);
 await assert.rejects(()=>h.tool.approveConnection({...endpoint,credentialId:'cred-terminal'}),/previous Terminal vault/);
 await h.tool.importPrevious('terminal');
 assert.deepEqual(JSON.parse(JSON.stringify(await h.tool.prepare({...endpoint,credentialId:'cred-terminal'}))),{credentialId:'cred-terminal',username:'operator',authMethod:'password'});
 // Without a previous vault the ordinary message is kept.
 const plain=harness({previous:[]});await plain.tool.refresh();
 await assert.rejects(()=>plain.tool.prepare({...endpoint,credentialId:'gone'}),/The saved credential is unavailable\./);
 assert.equal(plain.root.__test.previousVaultsHTML(),'');
 // Unknown sources from a response are ignored.
 const odd=harness({previous:[{source:'../other',label:'<b>x</b>',imported:false},{source:'editor',label:'ignored',imported:false}]});await odd.tool.refresh();
 assert.deepEqual(JSON.parse(JSON.stringify(odd.tool.previousVaults())),[{source:'editor',label:'Text Editor',imported:false}]);
});
test('credentials that did not fit in a full vault are named and the import stays on offer',async()=>{
 const h=harness();await h.tool.refresh();
 // The engine does not record an import that left credentials out.
 h.root.api=async(path,method,body)=>{
  if(path==='/credentials'&&!method)return {vault:{available:true,state:'unlocked'},entries:[],previousVaults:h.server.previous};
  if(path==='/credential-vault'&&body.action==='import-previous')return {ok:true,state:'unlocked',available:true,imported:1,alreadyPresent:0,skipped:2,skippedNames:['Lab console','<b>Switch</b> admin'],previousVaults:h.server.previous};
  throw Error('Unexpected '+path);
 };
 await h.tool.importPrevious('terminal');
 const [message,type]=h.toasts.at(-1);
 assert.match(message,/1 saved credential was imported\./);
 assert.match(message,/2 were not imported because the password manager is full: Lab console, <b>Switch<\/b> admin\./);
 assert.match(message,/Remove saved credentials you no longer need, then import again\./);
 assert.equal(type,'error','something was left out: this is not a plain success');
 assert(h.root.__test.previousVaultsHTML().includes('data-vault-import="terminal"'),'the import stays on offer');
});
