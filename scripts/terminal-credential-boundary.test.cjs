'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
function context(){const root={};root.window=root;vm.createContext(root);vm.runInContext(html.match(/^const TERMINAL_SAFE_FIELDS=.*$/m)[0],root);for(const name of ['safeTerminalConfig','terminalConnectPayload'])vm.runInContext(html.split('\n').find(line=>line.startsWith('function '+name+'(')),root);return root;}
test('saved SSH authentication payload contains an ID and no plaintext secret fields',()=>{
 const c=context(),config={type:'ssh',host:'router.test',username:'operator',credentialId:'saved-id',authMethod:'key'};
 const body=c.terminalConnectPayload(config,{password:'password-canary',privateKey:'key-canary',passphrase:'passphrase-canary'});
 assert.equal(body.credentialId,'saved-id');for(const field of ['password','privateKey','passphrase','secret'])assert.equal(Object.hasOwn(body,field),false);
 assert(!JSON.stringify(body).includes('canary'));
});
test('manual password and private-key authentication remain transient request inputs',()=>{
 const c=context(),config={type:'ssh',host:'router.test',username:'operator'};
 let body=c.terminalConnectPayload({...config,authMethod:'password'},{password:'manual-canary'});assert.equal(body.password,'manual-canary');assert.equal(Object.hasOwn(body,'credentialId'),false);
 body=c.terminalConnectPayload({...config,authMethod:'key'},{privateKey:'key-canary',passphrase:'passphrase-canary'});assert.equal(body.privateKey,'key-canary');assert.equal(body.passphrase,'passphrase-canary');assert.equal(Object.hasOwn(body,'credentialId'),false);
 assert(!JSON.stringify(c.safeTerminalConfig({...config,password:'manual-canary'})).includes('manual-canary'));
});
test('non-SSH payloads discard saved credential references and all remote secrets',()=>{
 const c=context();for(const type of ['telnet','serial','cmd','powershell']){const body=c.terminalConnectPayload({type,host:'test',credentialId:'saved-id',username:'operator'},{password:'secret'});for(const field of ['password','privateKey','passphrase','credentialId'])assert.equal(Object.hasOwn(body,field),false);}
});
test('native session bootstrap accepts only a 64-character lowercase hex capability',()=>{
 const line=html.match(/^const SESSION=.*$/m)[0];
 for(const [token,expected] of [['a'.repeat(64),'a'.repeat(64)],['A'.repeat(64),'__SESSION_TOKEN__'],['bad','__SESSION_TOKEN__'],[undefined,'__SESSION_TOKEN__']]){const c={window:{__networkToolkitNativeSession:token}};vm.runInNewContext(line+';result=SESSION;',c);assert.equal(c.result,expected);}
});
test('saved-credential consumers contain no old frontend secret-resolution API',()=>{
 for(const file of ['web/index.html','web/assets/remote-browser.js','web/assets/session-sidebar.js']){const text=fs.readFileSync(require.resolve('../'+file),'utf8');assert.doesNotMatch(text,/NetworkToolkitCredentials(?:\?\.)?\.?(?:resolve)/,file);}
 const source=fs.readFileSync(require.resolve('../web/assets/credential-manager.js'),'utf8');assert.doesNotMatch(source,/record\.secret|item\.secret/);assert.doesNotMatch(source,/data-credential-action="reveal"/);
});
