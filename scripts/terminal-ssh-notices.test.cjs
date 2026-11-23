'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const functionSource=name=>{const line=html.split('\n').find(value=>value.startsWith('function '+name+'(')||value.startsWith('async function '+name+'('));assert(line,'Missing '+name);return line;};
function harness(prompt={id:'notice',kind:'keyboard-interactive',title:'SSH authentication challenge',fields:[],actions:['cancel','submit']}){
 const elements={terminalPrompt:{innerHTML:''},terminalPromptError:{textContent:''}},requests=[];
 const entry={info:{id:'session',type:'ssh',host:'lab.test',port:22,state:'awaiting_input',prompt},answeredPrompts:new Set()};
 const c={online:true,terminalPromptSignature:'',terminalWorkspaceTransferBusy:false,terminalDetachedReturning:false,DETACHED_TERMINAL:false,owner:{viewId:'',ready:true},canMutate:true,
  state:{terminalConnection:{type:'ssh',host:'old.test',sshAutoContinueNotices:true}},icon:()=>'',label:String,
  $:selector=>elements[selector.slice(1)],$$:()=>[],currentTerminal:()=>entry,terminalSessionOwner:()=>c.owner,terminalOwnerCanMutate:()=>c.canMutate,
  kv:(key,value)=>'<span>'+key+'</span><span>'+c.escapeText(value)+'</span>',toast:()=>{},renderTerminalChrome:()=>{},
  api:async(path,method,body)=>{requests.push({path,method,body:JSON.parse(JSON.stringify(body))});if(c.pending)await c.pending;},
  deviceTarget:row=>row.host,endpointPort:value=>Number(value)||0};
 c.window=c;vm.createContext(c);
 vm.runInContext(html.match(/^const esc=.*$/m)[0]+'\nthis.escapeText=esc;',c);
 vm.runInContext(html.match(/^const TERMINAL_SAFE_FIELDS=.*$/m)[0],c);
 for(const name of ['safeTerminalConfig','terminalConnectPayload','terminalTransferConfig','terminalConfigForDevice','openSeparateTerminal','respondTerminalPrompt'])vm.runInContext(functionSource(name),c);
 vm.runInContext(html.slice(html.indexOf('function renderTerminalPrompt(){'),html.indexOf('\nasync function respondTerminalPrompt(')),c);
 return {c,entry,elements,requests};
}
test('SSH notices are an explicit boolean preference, off for old profiles and all other protocols',()=>{
 const {c}=harness();
 assert.equal(c.safeTerminalConfig().sshAutoContinueNotices,false);
 for(const value of [undefined,false,'true','false',1,{},[]])assert.equal(c.safeTerminalConfig({sshAutoContinueNotices:value}).sshAutoContinueNotices,false);
 assert.equal(c.safeTerminalConfig({type:'ssh',sshAutoContinueNotices:true}).sshAutoContinueNotices,true);
 for(const type of ['telnet','serial','cmd','powershell']){
  assert.equal(c.safeTerminalConfig({type,sshAutoContinueNotices:true}).sshAutoContinueNotices,false);
  assert.equal(Object.hasOwn(c.terminalConnectPayload({type,sshAutoContinueNotices:true}),'sshAutoContinueNotices'),false);
 }
});
test('separate terminal presets preserve the notice preference without exporting credentials or secrets',()=>{
 const {c}=harness(),config=c.terminalTransferConfig({type:'ssh',host:'lab.test',sshAutoContinueNotices:true,credentialId:'protected-id',password:'secret-canary',privateKey:'key-canary'});
 assert.equal(config.sshAutoContinueNotices,true);assert.equal(config.host,'lab.test');
 for(const name of ['credentialId','password','privateKey','passphrase'])assert.equal(Object.hasOwn(config,name),false);
 assert(!JSON.stringify(config).includes('canary'));
});
test('an unrelated device connection does not inherit another server notice preference',()=>{
 const {c}=harness();
 assert.equal(c.terminalConfigForDevice({host:'new.test',sourceTool:'scan'},'ssh').sshAutoContinueNotices,false);
 assert.equal(c.terminalConfigForDevice({host:'new.test',sourceTool:'terminal-profile',sshAutoContinueNotices:true},'ssh').sshAutoContinueNotices,true);
 assert.equal(c.terminalConfigForDevice({host:'new.test',sourceTool:'terminal-profile',sshAutoContinueNotices:true},'telnet').sshAutoContinueNotices,false);
});
test('separate-window controls are re-enabled after both successful and failed launches',async()=>{
 for(const rejected of [false,true]){
  const {c,requests}=harness(),buttons=[{disabled:false},{disabled:false}];
  c.status={terminalWindowAvailable:true};c.OFFLINE=false;c.terminalWindowLaunchBusy=false;c.$$=()=>buttons;
  const send=c.api;c.api=async(...args)=>{assert(buttons.every(button=>button.disabled));if(rejected)throw new Error('Synthetic window launch rejected');return send(...args);};
  await c.openSeparateTerminal({type:'ssh',host:'lab.test',sshAutoContinueNotices:true},'profile');
  assert.equal(c.terminalWindowLaunchBusy,false);assert(buttons.every(button=>!button.disabled));
  if(!rejected)assert.deepEqual(requests[0].body.config.sshAutoContinueNotices,true);
 }
});
test('an empty keyboard-interactive notice explains the acknowledgement without inventing an answer field',()=>{
 const {c,elements}=harness();c.renderTerminalPrompt();const content=elements.terminalPrompt.innerHTML;
 assert.match(content,/SSH sign-in notice/);assert.match(content,/server asks for no answer/);assert.match(content,/Select Continue to acknowledge/);
 assert.doesNotMatch(content,/terminalPromptField-/);assert.match(content,/data-terminal-prompt-action="submit"/);assert.match(content,/Cancel authentication/);
});
test('server notice heading and instruction are displayed as escaped text',()=>{
 const {c,elements}=harness({id:'server-notice',kind:'keyboard-interactive',serverName:'<img src=x onerror=alert(1)>',message:'Use <Continue> & proceed.',fields:[],actions:['cancel','submit']});
 c.renderTerminalPrompt();const content=elements.terminalPrompt.innerHTML;
 assert.match(content,/&lt;img src=x onerror=alert\(1\)&gt;/);assert.match(content,/Use &lt;Continue&gt; &amp; proceed\./);assert.doesNotMatch(content,/<img/);
});
test('Continue sends one empty answer list and suppresses repeat submissions while it is pending',async()=>{
 const {c,entry,requests}=harness();let release;c.pending=new Promise(resolve=>{release=resolve;});
 const submitted=c.respondTerminalPrompt('submit');await c.respondTerminalPrompt('submit');
 assert.equal(requests.length,1);assert.deepEqual(requests[0],{path:'/terminals/session/respond',method:'POST',body:{promptId:'notice',action:'submit',answers:[]}});
 release();await submitted;assert.equal(entry.info.prompt,null);assert.equal(entry.info.state,'connecting');assert(entry.answeredPrompts.has('notice'));
});
test('password and verification questions still require their original fields',async()=>{
 const {c,elements,requests}=harness({id:'mfa',kind:'keyboard-interactive',title:'SSH authentication challenge',serverName:'Lab sign-in',message:'Enter the requested values.',fields:[{label:'Password:',echo:false},{label:'Verification code:',echo:false}],actions:['cancel','submit']});
 c.renderTerminalPrompt();assert.match(elements.terminalPrompt.innerHTML,/SSH authentication challenge/);assert.doesNotMatch(elements.terminalPrompt.innerHTML,/SSH sign-in notice/);
 assert.equal((elements.terminalPrompt.innerHTML.match(/type="password"/g)||[]).length,2);
 assert.equal(requests.length,0);elements['terminalPromptField-0']={value:'password-canary'};elements['terminalPromptField-1']={value:'654321'};
 await c.respondTerminalPrompt('submit');assert.deepEqual(requests[0].body.answers,['password-canary','654321']);
});
test('host trust prompts remain explicit and are not rendered as sign-in notices',()=>{
 const {c,elements,requests}=harness({id:'host',kind:'host-key',title:'Verify SSH server',message:'New host key',fingerprint:'SHA256:synthetic',fields:[],actions:['reject','once','save']});
 c.renderTerminalPrompt();assert.match(elements.terminalPrompt.innerHTML,/Verify SSH server/);assert.match(elements.terminalPrompt.innerHTML,/SHA256:synthetic/);assert.match(elements.terminalPrompt.innerHTML,/Verify the fingerprint through a trusted channel/);
 assert.doesNotMatch(elements.terminalPrompt.innerHTML,/SSH sign-in notice|asks for no answer/);assert.equal(requests.length,0);
});
test('a notice cannot be answered from an inactive owner or disconnected terminal view',async()=>{
 for(const condition of ['offline','other-owner','transfer','returning']){
  const {c,entry,elements,requests}=harness();
  if(condition==='offline')c.online=false;
  if(condition==='other-owner'){c.canMutate=false;c.owner={viewId:'detached',ready:true};}
  if(condition==='transfer')entry.transferBusy=true;
  if(condition==='returning')c.terminalDetachedReturning=true;
  c.renderTerminalPrompt();assert.doesNotMatch(elements.terminalPrompt.innerHTML,/data-terminal-prompt-action/);
  await c.respondTerminalPrompt('submit');assert.equal(requests.length,0,condition);
 }
});
