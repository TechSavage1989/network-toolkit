'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
function harness(available=true){const c={status:{localShellAvailable:available},DETACHED_TERMINAL:false,DETACHED_WORKSPACE:false,esc:String,icon:()=>''};vm.createContext(c);vm.runInContext(html.match(/^const TERMINAL_SAFE_FIELDS=.*$/m)[0],c);for(const name of ['safeTerminalConfig','terminalConnectPayload','validateTerminalConfig','terminalProtocolLabel','terminalConnectionButtonsHTML']){const line=html.split('\n').find(s=>s.startsWith('function '+name+'('));assert(line,name);vm.runInContext(line,c);}return c;}
for(const type of ['cmd','powershell'])test(type+' local session shares terminal routes without remote settings or credentials',()=>{
 const h=harness(),config=h.safeTerminalConfig({type,host:'remote.test',port:2222,username:'admin',credentialId:'remote-vault',serialPort:'COM4',password:'secret',privateKey:'secret',localEcho:true});
 assert.equal(config.type,type);assert.equal(config.host,'');assert.equal(config.port,0);assert.equal(config.username,'');assert.equal(config.serialPort,'');assert(!config.credentialId);assert.equal(config.lineEnding,'cr');assert.equal(config.localEcho,false);
 assert.equal(h.validateTerminalConfig(config),config);
 const body=JSON.parse(JSON.stringify(h.terminalConnectPayload(config,{password:'secret',privateKey:'secret'})));
 assert.deepEqual(body,{type,cols:80,rows:24,terminal:'xterm-256color',timeoutMs:10000,workingDirectory:'',runAsAdministrator:false});
 assert.match(h.terminalProtocolLabel(config),/\(local\)/);assert.match(h.terminalConnectionButtonsHTML(0),new RegExp('data-terminal-new-type="'+type+'"'));
 const unsupported=harness(false);assert.doesNotMatch(unsupported.terminalConnectionButtonsHTML(0),new RegExp('data-terminal-new-type="'+type+'"'));assert.throws(()=>unsupported.validateTerminalConfig(config),/require Windows/);
});

test('local directory is literal nonsecret data, admin defaults off and remote types ignore it',()=>{
 const h=harness(),directory='C:\\Lab & tools\\日本語';
 assert.equal(h.safeTerminalConfig({type:'cmd'}).runAsAdministrator,false);
 const config=h.safeTerminalConfig({type:'powershell',workingDirectory:directory,runAsAdministrator:true});
 const body=JSON.parse(JSON.stringify(h.terminalConnectPayload(config)));
 assert.equal(body.workingDirectory,directory);assert.equal(body.runAsAdministrator,true);
 assert.equal(h.safeTerminalConfig({type:'cmd',runAsAdministrator:'true'}).runAsAdministrator,false);
 const remote=h.terminalConnectPayload({type:'ssh',host:'example.test',workingDirectory:directory,runAsAdministrator:true});
 assert(!('workingDirectory' in remote));assert(!('runAsAdministrator' in remote));
});
