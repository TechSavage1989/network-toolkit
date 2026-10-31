'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
function functionSource(name){const line=html.split('\n').find(s=>s.startsWith('function '+name+'('));assert(line,'Missing '+name);return line;}
function setup(){
 const fields={terminalProfileSelect:{value:''},terminalConnectProfileSelect:{value:''},terminalProfileName:{value:''},terminalProfileFolder:{value:''},'con-type':{value:'ssh'},terminalConnectError:{textContent:''}};
 const profiles=[{id:'ssh',name:'Core switch',type:'ssh',host:'192.0.2.10',port:2222,username:'admin',sshCompatibility:'legacy',credentialId:'credential-reference'},{id:'serial',name:'Console',type:'serial',serialPort:'COM8',baudRate:115200,lineEnding:'cr',localEcho:true}];
 const destination={pane:2,panes:['a','b','c','d']};let clears=0,renders=0,saves=0;
 const c={state:{terminalProfiles:profiles},terminalProfileID:'',terminalConnectBusy:false,terminalDraft:{},terminalConnectDestination:destination,TERMINAL_SAFE_FIELDS:['type','host','port','username','authMethod','credentialId','sshCompatibility','timeoutMs','serialPort','baudRate','dataBits','stopBits','parity','flowControl','localEcho','lineEnding'],document:{getElementById:id=>fields[id]},$:id=>fields[id.slice(1)],clearTerminalSecrets:()=>clears++,saveState:()=>saves++,renderTerminalConnectFields:()=>renders++,terminalProfileListConfig:()=>({rows:profiles,columns:[],view:'profiles',defaults:{}}),sortDataRows:rows=>rows,listPreference:()=>({}),terminalProtocolLabel:p=>p.type,esc:String,toast:()=>{},captureTerminalConnection:()=>c.terminalDraft};
 c.window=c;c.renderTerminalChrome=()=>{};vm.createContext(c);vm.runInContext(fs.readFileSync(path.join(__dirname,'../web/assets/terminal-prompt-labels.js'),'utf8'),c);vm.runInContext(fs.readFileSync(path.join(__dirname,'../web/assets/saved-session-folders.js'),'utf8'),c);for(const name of ['safeTerminalConfig','renderTerminalProfiles','chooseTerminalConnectionProfile','saveTerminalProfile'])vm.runInContext(functionSource(name),c);
 return {c,fields,counts:()=>({clears,renders,saves}),destination};
}
test('connection dialog exposes saved connection selection and reloads full profile metadata',()=>{
 const {c,fields,counts,destination}=setup();c.chooseTerminalConnectionProfile('ssh');assert.equal(c.terminalDraft.host,'192.0.2.10');assert.equal(c.terminalDraft.port,2222);assert.equal(c.terminalDraft.sshCompatibility,'legacy');assert.equal(c.terminalDraft.credentialId,'credential-reference');assert.equal(fields.terminalProfileName.value,'Core switch');assert.equal(fields.terminalConnectProfileSelect.value,'ssh');assert.match(fields.terminalConnectProfileSelect.innerHTML,/Saved credential/);assert.equal(c.terminalConnectDestination,destination);assert.equal(counts().clears,1);assert.equal(counts().renders,1);assert(html.includes('<label for="terminalConnectProfileSelect">Saved connections</label>'));
});
test('serial profiles load the selected hardware settings and New connection clears old credentials',()=>{
 const {c,fields}=setup();c.chooseTerminalConnectionProfile('serial');assert.equal(c.terminalDraft.serialPort,'COM8');assert.equal(c.terminalDraft.baudRate,115200);assert.equal(c.terminalDraft.lineEnding,'cr');assert.equal(c.terminalDraft.localEcho,true);c.chooseTerminalConnectionProfile('');assert.equal(c.terminalDraft.type,'ssh');assert.equal(c.terminalDraft.host,'');assert.equal(c.terminalDraft.credentialId,'');assert.equal(fields.terminalProfileName.value,'');
});
test('saving a selected profile updates it and keeps it selected without saving secrets',()=>{
 const {c,fields}=setup();c.chooseTerminalConnectionProfile('ssh');c.terminalDraft.host='192.0.2.99';c.terminalDraft.password='DO NOT SAVE';fields.terminalProfileName.value='Updated core';c.saveTerminalProfile();const updated=c.state.terminalProfiles.find(p=>p.id==='ssh');assert.equal(c.state.terminalProfiles.length,2);assert.equal(updated.host,'192.0.2.99');assert.equal(updated.name,'Updated core');assert.equal(updated.credentialId,'credential-reference');assert.equal(updated.password,undefined);assert.equal(fields.terminalConnectProfileSelect.value,'ssh');
});
test('editing an existing connection retains its folder and a new connection starts unfiled',()=>{
 const {c,fields}=setup();c.state.terminalProfiles[0].folder='MDF';c.chooseTerminalConnectionProfile('ssh');assert.equal(fields.terminalProfileFolder.value,'MDF');
 fields.terminalProfileFolder.value=' Branch ';c.saveTerminalProfile();const profile=c.state.terminalProfiles.find(row=>row.id==='ssh');assert.equal(profile.folder,'Branch');assert.equal(profile.credentialId,'credential-reference');assert.equal(c.terminalDraft.folder,undefined);
 c.chooseTerminalConnectionProfile('');assert.equal(fields.terminalProfileFolder.value,'');assert.equal(c.terminalDraft.folder,undefined);
});
