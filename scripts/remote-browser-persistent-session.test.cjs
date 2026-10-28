'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8');
function harness(){const calls=[],ui=[],editors=[],state={activePage:'sftp',remoteBrowserSettings:{},terminalProfiles:[]};let serial=0,resolves=0;
 const root={state,document:{addEventListener(){},getElementById(){return null;},querySelector(){return null;}},console,Map,Set,Date,Math,JSON,Number,String,esc:String,icon:name=>'<i>'+name+'</i>',saveState(){},toast(){},openModal(){},
  NetworkToolkitTransfers:{mode:()=> 'client'},NetworkToolkitCredentials:{prepare:async config=>{resolves++;return {credentialId:config.credentialId,username:'operator',authMethod:'password'};}},
  NetworkToolkitFileEditor:{open:async value=>editors.push(value)},
  api:async(path,method,body)=>{calls.push({path,method,body});if(body?.action==='connect')return {connectionId:'connection-'+(++serial),path:'/',entries:[]};return {path:'/',entries:[]};},
  receiveTerminalViewState:value=>ui.push(['viewState',value]),upsertTerminalSession:info=>{ui.push(['upsert',info.id]);return {info,config:{}};},safeTerminalConfig:value=>({...value}),navigate:page=>ui.push(['navigate',page]),selectTerminal:id=>ui.push(['select',id]),startTerminalFeed:()=>ui.push(['feed']),scheduleTerminalFit(){}};
 root.window=root;root.globalThis=root;vm.runInNewContext(source,root);const browser=root.NetworkToolkitRemoteBrowser;Object.assign(browser.configurations.ssh,{host:'switch.test',port:22,username:'operator',password:'temporary-password',fingerprint:'SHA256:verified'});browser.get('ssh').saved=true;
 return {root,state,browser,calls,ui,editors,resolves:()=>resolves};}
test('standalone SFTP authenticates once and sends only its connection ID for later file operations',async()=>{
 const h=harness();h.browser.configurations.ssh.credentialId='vault-id';h.browser.configurations.ssh.password='';await h.browser.connect('ssh');
 assert.equal(h.browser.get('ssh').connectionId,'connection-1');assert.equal(h.resolves(),1);
 const first=h.calls.find(x=>x.body?.action==='connect').body;assert.equal(first.credentialId,'vault-id');assert.equal(Object.hasOwn(first,'password'),false);assert.equal(first.fingerprint,'SHA256:verified');
 for(const action of ['list','read-text','stat','download','write-text','mkdir','rename','remove'])await h.browser.request('ssh',action,{path:'/config.txt'});
 for(const call of h.calls.filter(x=>x.body?.action!=='connect')){assert.equal(call.body.connectionId,'connection-1');for(const field of ['host','username','password','privateKey','passphrase','fingerprint','port'])assert.equal(Object.hasOwn(call.body,field),false);}
 assert.equal(h.resolves(),1);assert(!JSON.stringify(h.state).includes('vault-password'));
});
test('Disconnect closes the owned SSH connection once; Reconnect explicitly authenticates a new connection',async()=>{
 const h=harness();h.browser.configurations.ssh.credentialId='vault-id';h.browser.configurations.ssh.password='';await h.browser.connect('ssh');await h.browser.disconnect('ssh');
 assert.equal(h.calls.filter(x=>x.body?.action==='disconnect').length,1);assert.equal(h.browser.get('ssh').connected,false);
 await h.browser.reconnect('ssh');assert.equal(h.browser.get('ssh').connectionId,'connection-2');assert.equal(h.calls.filter(x=>x.body?.action==='connect').length,2);
});
test('Disconnect browser leaves a borrowed Terminal SSH connection running and refresh uses its existing session',async()=>{
 const h=harness();Object.assign(h.browser.get('ssh'),{connected:true,sessionId:'terminal-one'});
 assert.match(h.browser.sessionTools('ssh'),/Disconnect browser/);assert.match(h.browser.sessionTools('ssh'),/Refresh connection/);
 await h.browser.reconnect('ssh');const list=h.calls.find(x=>x.body?.action==='list');assert.equal(list.body.sessionId,'terminal-one');
 await h.browser.disconnect('ssh');assert.equal(h.calls.filter(x=>x.body?.action==='disconnect'||x.path.includes('/reconnect')).length,0);assert.equal(h.browser.get('ssh').sessionId,'');
});
test('Open terminal promotes the authenticated connection without reopening SSH or invalidating open editor tabs',async()=>{
 const h=harness();await h.browser.connect('ssh');await h.browser.edit('ssh',false,'remote','/configuration.txt');const editor=h.editors[0],identity=h.browser.connectionIdentity('ssh'),dragIdentity=h.browser.dragIdentity('ssh');
 const original=h.root.api;h.root.api=async(path,method,body)=>{if(body?.action==='open-terminal'){h.calls.push({path,method,body});return {session:{id:'promoted-terminal',state:'connected',type:'ssh'},viewState:{revision:1}};}return original(path,method,body);};
 await h.browser.openTerminal('ssh');assert.equal(h.browser.get('ssh').connectionId,'');assert.equal(h.browser.get('ssh').sessionId,'promoted-terminal');assert.equal(h.browser.connectionIdentity('ssh'),identity);assert.notEqual(h.browser.dragIdentity('ssh'),dragIdentity);
 assert.equal(h.calls.filter(x=>x.body?.action==='connect').length,1);assert.equal(h.calls.filter(x=>x.body?.action==='open-terminal')[0].body.connectionId,'connection-1');
 await editor.read('/configuration.txt');assert.equal(h.calls.at(-1).body.sessionId,'promoted-terminal');assert(!Object.hasOwn(h.calls.at(-1).body,'password'));
 assert(h.ui.some(x=>x[0]==='navigate'&&x[1]==='terminal'));assert(h.ui.some(x=>x[0]==='select'&&x[1]==='promoted-terminal'));
 await h.browser.disconnect('ssh');await assert.rejects(()=>editor.write({path:'/configuration.txt',content:'changed'}),/Connection changed or disconnected/);
});
test('a server that rejects a shell keeps the original SFTP connection usable',async()=>{
 const h=harness();await h.browser.connect('ssh');const original=h.root.api;h.root.api=async(path,method,body)=>{if(body?.action==='open-terminal')throw Error('Server rejected interactive shell');return original(path,method,body);};
 await h.browser.openTerminal('ssh');assert.equal(h.browser.get('ssh').connected,true);assert.equal(h.browser.get('ssh').connectionId,'connection-1');await h.browser.request('ssh','list',{path:'/'});assert.equal(h.calls.at(-1).body.connectionId,'connection-1');
});
test('cancelled connection attempts close late server handles instead of replacing the newer browser identity',async()=>{
 const h=harness();let respond;h.root.api=async(path,method,body)=>{h.calls.push({path,method,body});if(body?.action==='connect')return new Promise(resolve=>respond=resolve);return {};};
 const opening=h.browser.connect('ssh');while(!respond)await new Promise(resolve=>setImmediate(resolve));h.browser.invalidateConnection('ssh');respond({connectionId:'late-connection',path:'/',entries:[]});await opening;
 assert.equal(h.browser.get('ssh').connected,false);assert.equal(h.browser.get('ssh').connectionId,'');assert.equal(h.calls.filter(x=>x.body?.action==='disconnect'&&x.body.connectionId==='late-connection').length,1);
});
test('SFTP file operations cannot silently fall back to repeated credential-based SSH logins',async()=>{
 const h=harness();await assert.rejects(()=>h.browser.request('ssh','list',{path:'/'}),/Connect to an SSH\/SFTP session/);assert.equal(h.calls.length,0);
});
test('opening another active Terminal starts at its home instead of using a previous server path',async()=>{
 const h=harness(),m=h.browser.get('ssh');Object.assign(m,{sessionId:'previous-terminal',connected:true});Object.assign(m.remote,{path:'/only-on-previous-server',home:'/previous-home',parent:'/',history:['/previous-home','/only-on-previous-server'],index:1,initialized:true});
 h.browser.useTerminalSession('chosen-terminal');assert.equal(m.remote.path,'');assert.equal(m.remote.home,'');assert.equal(m.remote.history.length,0);
 await h.browser.connectTerminal('ssh','chosen-terminal',{preserveFolder:true});const list=h.calls.find(call=>call.body?.action==='list');assert.equal(list.body.sessionId,'chosen-terminal');assert.equal(list.body.path,'.');assert.equal(h.calls.some(call=>call.body?.action==='connect'),false);assert.equal(h.calls.some(call=>Object.hasOwn(call.body||{},'password')),false);
});
test('reopening SFTP for the same Terminal preserves its current folder',async()=>{
 const h=harness(),m=h.browser.get('ssh');Object.assign(m,{sessionId:'chosen-terminal',connected:true});Object.assign(m.remote,{path:'/home/operator/configs',home:'/home/operator',history:['/home/operator','/home/operator/configs'],index:1});
 h.browser.useTerminalSession('chosen-terminal');assert.equal(m.remote.path,'/home/operator/configs');assert.equal(m.remote.home,'/home/operator');
 await h.browser.connectTerminal('ssh','chosen-terminal',{preserveFolder:true});assert.equal(h.calls.find(call=>call.body?.action==='list').body.path,'/home/operator/configs');assert.equal(h.calls.some(call=>call.body?.action==='connect'),false);
});
test('selecting another Terminal directly also clears the old remote folder',async()=>{
 const h=harness(),m=h.browser.get('ssh');Object.assign(m,{sessionId:'previous-terminal',connected:true});m.remote.path='/previous-only';await h.browser.connectTerminal('ssh','new-terminal');assert.equal(h.calls.find(call=>call.body?.action==='list').body.path,'.');assert.equal(m.sessionId,'new-terminal');
});
test('first Open SFTP does not restore a saved folder from an unrelated server',()=>{
 const h=harness(),m=h.browser.get('ssh');m.saved=false;h.state.remoteBrowserSettings={ssh:{host:'previous.test',remotePath:'/previous-server-only'}};h.browser.useTerminalSession('chosen-terminal');assert.equal(m.remote.path,'');assert.equal(m.autoTerminal,'chosen-terminal');
});

test('successful manual SSH authentication clears password, private key and passphrase but not failed retries',async()=>{
 const h=harness(),c=h.browser.configurations.ssh;Object.assign(c,{privateKey:'temporary-private-key',passphrase:'temporary-passphrase'});
 const original=h.root.api;h.root.api=async(path,method,body)=>{if(body?.action==='connect')throw Error('Synthetic authentication rejected');return original(path,method,body);};
 await h.browser.connect('ssh');assert.equal(c.password,'temporary-password');assert.equal(c.privateKey,'temporary-private-key');assert.equal(c.passphrase,'temporary-passphrase');
 h.root.api=original;await h.browser.connect('ssh');assert.equal(h.browser.get('ssh').connected,true);
 for(const field of ['password','privateKey','passphrase'])assert.equal(c[field],'');
 const count=h.calls.filter(row=>row.body?.action==='connect').length;await h.browser.reconnect('ssh');
 assert.equal(h.calls.filter(row=>row.body?.action==='connect').length,count);assert.equal(h.browser.get('ssh').showSettings,true);assert.equal(h.browser.get('ssh').connected,true);
});
test('manual FTPS password remains transiently available for per-operation authentication',async()=>{
 const h=harness(),c=h.browser.configurations.ftp;h.browser.get('ftp').saved=true;Object.assign(c,{host:'files.test',port:21,username:'operator',password:'manual-ftps-password',security:'explicit'});
 await h.browser.connect('ftp');assert.equal(h.browser.get('ftp').connected,true);assert.equal(c.password,'manual-ftps-password');
 await h.browser.request('ftp','list',{path:'/'});assert.equal(h.calls.at(-1).body.password,'manual-ftps-password');assert(!JSON.stringify(h.state).includes('manual-ftps-password'));
});
test('late SSH success does not erase a newly entered unrelated draft secret',async()=>{
 const h=harness();let finish;h.root.api=async(path,method,body)=>body?.action==='connect'?new Promise(resolve=>finish=resolve):{};
 const pending=h.browser.connect('ssh');while(!finish)await new Promise(resolve=>setImmediate(resolve));
 h.browser.configurations.ssh.password='new-draft-not-used-in-authentication';finish({connectionId:'authenticated',path:'/',entries:[]});await pending;
 assert.equal(h.browser.configurations.ssh.password,'new-draft-not-used-in-authentication');
});

test('existing file-browser editors carry exact local or ID-only SSH bindings, never FTP credentials',async()=>{
 const h=harness();await h.browser.connect('ssh');await h.browser.edit('ssh',false,'remote','/configuration.txt');const retained=h.editors.at(-1);assert.deepEqual(JSON.parse(JSON.stringify(retained.binding)),{kind:'sshConnection',connectionId:'connection-1'});
 await h.browser.edit('ssh',false,'local','C:\\Synthetic\\configuration.txt');assert.deepEqual(JSON.parse(JSON.stringify(h.editors.at(-1).binding)),{kind:'local'});
 h.root.terminalSessionRevision=()=>7;Object.assign(h.browser.get('ssh'),{connected:true,connectionId:'',sessionId:'terminal-binding'});await h.browser.edit('ssh',false,'remote','/terminal.txt');const terminal=h.editors.at(-1);assert.deepEqual(JSON.parse(JSON.stringify(terminal.binding)),{kind:'terminal',sessionId:'terminal-binding',revision:7});h.root.terminalSessionRevision=()=>8;assert.equal(terminal.binding.revision,7);
 Object.assign(h.browser.get('ftp'),{connected:true});await h.browser.edit('ftp',false,'remote','/ftp.txt');assert.equal(h.editors.at(-1).binding,null);for(const editor of [retained,terminal])for(const secret of ['password','privateKey','passphrase','credentialId','host'])assert.equal(Object.hasOwn(editor.binding,secret),false);
});
