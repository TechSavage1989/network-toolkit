'use strict';
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const server=fs.readFileSync(path.join(__dirname,'../web/assets/transfer-server.js'),'utf8');
const explorer=fs.readFileSync(path.join(__dirname,'../web/assets/transfer-explorer.js'),'utf8');
test('TFTP server startup never uses browser confirmation suppressed by the native Windows host',()=>{
 assert.doesNotMatch(server,/[^a-zA-Z]confirm\(/);
 assert.match(server,/data-transfer-confirm-start/);
 assert.match(server,/openModal\('Start '\+p\.toUpperCase\(\)\+' server'/);
 assert.match(server,/start:async function\(p,confirmed=false\)/);
});
test('SFTP and TFTP retain a visible root and status when settings are collapsed',()=>{
 assert.match(server,/data-transfer-current-root/);
 assert.match(server,/data-transfer-current-bind/);
 assert.match(server,/data-transfer-current-access/);
 assert.match(server,/data-transfer-current-active/);
 assert.doesNotMatch(server,/data-transfer-mode-root/); // Compact server UI shows the root only once in its summary
 assert.match(server,/id="transferServerFeedback"/);
 assert.match(server,/Enter an SSH username before starting/);
 assert.match(server,/Enter a session password or select/);
});
test('server settings remain open until user explicitly toggles them, across redraws',()=>{
 assert.match(explorer,/settingsVisible\?\.\(p\)/);
 assert.match(explorer,/setSettingsVisible\?\.\(p,!d.open\)/);
 assert.doesNotMatch(explorer,/d\.open=!rootPath/);
 assert.match(server,/settingsVisible:function\(p\)/);
});

for(const protocol of ['sftp','tftp','ftp'])test(protocol+' native adapter selection persists and reaches server startup unchanged',async()=>{
 const document=makeDocument(''),state={activePage:'overview'},requests=[];
 const c={document,state,Set,Number,String,Date,Math,setInterval(){},saveState(){},toast(){},
  esc:value=>String(value),interfaces:()=>[
   {name:'Ethernet',description:'Primary LAN adapter',up:true,addresses:['192.0.2.10/24','192.0.2.10/24','2001:db8::1/64']},
   {name:'Offline',up:false,addresses:['192.0.2.20/24']}
  ],addresses:a=>a.addresses,adapterLabel:a=>a.name+' - '+a.description,
  $:selector=>document.querySelector(selector),
  api:async(url,method,body)=>{requests.push({url,method,body});return {running:true,root:body.root,address:body.bind+':'+body.port};}};
 c.window=c;vm.createContext(c);vm.runInContext(server,c);
 const ui=c.NetworkToolkitTransfers;
 document.innerHTML=ui.bindPicker(protocol,'127.0.0.1','')+
  '<input data-transfer-setting="root" data-transfer-proto="'+protocol+'">'+
  '<input data-transfer-setting="username" data-transfer-proto="'+protocol+'">'+
  '<input id="'+protocol+'ServerPassword">';
 const select=document.querySelector('select[data-transfer-setting="bind"]');
 assert.ok(select);assert.equal(select.hasAttribute('hidden'),false);
 assert.equal(document.querySelectorAll('select').length,1);
 assert.equal(document.querySelectorAll('details,button').length,0);
 assert.equal(select.querySelectorAll('option').length,3,'only loopback, all-adapter and one available IPv4 entry');
 select.value='192.0.2.10';select.type='select-one';
 select.selectedOptions=[select.querySelector('option[value="192.0.2.10"]')];
 document.emit('input',select);
 assert.equal(state.transferServerSettings[protocol].bind,'192.0.2.10');
 assert.equal(select.title,'Ethernet - Primary LAN adapter | 192.0.2.10');
 for(const [setting,value] of [['root','/server-root'],['username','operator']]){
  const input=document.querySelector('[data-transfer-setting="'+setting+'"]');input.value=value;document.emit('input',input);
 }
 const password=document.getElementById(protocol+'ServerPassword');password.value='temporary-password';document.emit('input',password);
 ui.render=()=>{};
 await ui.start(protocol,true);
 assert.equal(requests.length,1);assert.equal(requests[0].url,'/transfer/server');
 assert.equal(requests[0].body.bind,'192.0.2.10');assert.equal(requests[0].body.root,'/server-root');
 assert.equal(state.transferServerSettings[protocol].password,undefined);
 const locked=makeDocument(ui.bindPicker(protocol,'192.0.2.10',' disabled')).querySelector('select');
 assert.ok(locked.hasAttribute('disabled'),'running server keeps the actual selector disabled');
 const missing=makeDocument(ui.bindPicker(protocol,'192.0.2.99','')).querySelector('option[value="192.0.2.99"]');
 assert.ok(missing.hasAttribute('selected'));assert.match(missing.textContent,/Unavailable adapter address/);
});
