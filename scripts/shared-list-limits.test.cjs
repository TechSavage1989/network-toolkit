'use strict';
// Saved lists (saved sessions, saved FTP connections, saved targets, saved
// TFTP profiles) are shared by every app of a data folder and merged by
// entry. A page that sends a shorter list than it was given is therefore
// read as "the user removed these". So no load path and no save path may
// shorten a saved list: a full list refuses another entry and says so.
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const web=path.join(__dirname,'..','web'),read=file=>fs.readFileSync(path.join(web,file),'utf8');
const html=read('index.html'),engine=fs.readFileSync(path.join(__dirname,'..','cmd','network-toolkit','shared_data.go'),'utf8');
const folders=require('../web/assets/saved-session-folders.js');
const sharedSource=read('assets/shared-data.js').replace("if(typeof document!=='undefined')start();",'');
function sharedData(){
 const c={console,JSON,Set,Map,Number,Math,Object,Array,String,Promise,state:{},document:undefined};
 c.window=c;vm.createContext(c);vm.runInContext(sharedSource,c);return c.NetworkToolkitSharedData;
}
const shared=sharedData();
const SAVED=Object.keys(shared.SAVED_LIMITS);
const many=(count,make)=>Array.from({length:count},(_,index)=>make(index));
const ssh=index=>({id:'ssh-'+index,name:'Device '+index,type:'ssh',host:'192.0.2.'+(index%250+1),port:22,username:'operator',folder:''});
const ftp=index=>({id:'ftp-'+index,name:'Files '+index,host:'files'+index+'.example.com',port:21,username:'operator',security:'explicit',fingerprint:'',credentialId:'',folder:''});

test('the page and the engine use the same limit for every saved list',()=>{
 const rows=[...engine.matchAll(/\{Name: "([A-Za-z]+)", Kind: sharedList([^}]*)\}/g)].map(match=>({name:match[1],max:Number((/MaxSaved: (\d+)/.exec(match[2])||[])[1]||0),rolling:Number((/Limit: (\d+)/.exec(match[2])||[])[1]||0)}));
 assert(rows.length>=8,'the engine table was read');
 const saved=Object.fromEntries(rows.filter(row=>row.max).map(row=>[row.name,row.max])),rolling=Object.fromEntries(rows.filter(row=>row.rolling).map(row=>[row.name,row.rolling]));
 assert.deepEqual({...shared.SAVED_LIMITS},saved);assert.deepEqual({...shared.ROLLING_LIMITS},rolling);
 // Every list is one or the other: nothing is cut without a rule.
 for(const row of rows)assert(row.max>0!==row.rolling>0,row.name+' is either a saved list (MaxSaved) or a rolling list (Limit)');
 assert.deepEqual(SAVED.sort(),['ftpProfiles','profiles','terminalProfiles','tftpProfiles']);
});

// Whether a statement cuts an array short: ".slice(0,n)" on an array literal
// or on the result of an array method. ".slice(0,n)" on a text field (an id
// or a name) is not a shorter list.
function cutsList(statement){
 const arrays=new Set(['filter','map','concat','sort','flat','flatMap','reverse','from','toSorted','profiles','rows','ftpProfiles','list','savedOrder']);
 for(const match of statement.matchAll(/\.slice\(\s*0\s*,/g)){
  const before=statement.slice(0,match.index),last=before.at(-1);
  if(last===']')return true;
  if(last!==')')continue;
  let depth=0,open=-1;
  for(let i=before.length-1;i>=0;i--){if(before[i]===')')depth++;else if(before[i]==='('&&--depth===0){open=i;break;}}
  const method=(/([A-Za-z_$][\w$]*)$/.exec(before.slice(0,open))||[])[1];
  if(arrays.has(method))return true;
 }
 return /\.splice\(|\.length\s*=(?!=)/.test(statement);
}
test('no page code shortens a saved list when it loads or saves it',()=>{
 assert(cutsList("state.x=[a,...b.filter(p=>p.id!==id)].slice(0,100)")&&cutsList("state.x=v.filter(ok).slice(0,200).map(p=>p)")&&!cutsList("state.x=v.filter(ok).map(p=>({id:p.id.slice(0,80),name:name.trim().slice(0,100)}))"),'the check tells a shorter list from a shorter text');
 const files=['index.html',...fs.readdirSync(path.join(web,'assets')).filter(name=>name.endsWith('.js')).map(name=>'assets/'+name)];
 const offenders=[];let found=0;
 for(const file of files){
  const text=read(file);
  for(const key of SAVED){
   // Every statement that gives the list a new value.
   const assignment=new RegExp('(?:state|out)\\.'+key+'\\s*=(?!=)[^;]*','g');let match;
   while((match=assignment.exec(text))){found++;if(cutsList(match[0]))offenders.push(file+': '+match[0].slice(0,160));}
  }
 }
 assert(found>=12,'the statements that assign saved lists were found: '+found);
 assert.deepEqual(offenders,[],'a saved list is cut while it is loaded or saved');
 // The limit is written in one place.
 for(const file of files)assert(!/limit is 100|100-item limit|maximum of 200 saved targets/i.test(read(file)),file+' states a limit of its own');
});

test('a list longer than the limit is loaded whole',()=>{
 const values=shared.normalize({terminalProfiles:many(260,ssh),ftpProfiles:many(230,ftp)},{config:value=>({type:value.type,host:value.host,port:value.port,username:value.username}),labels:{revision:()=>0,normalize:value=>value},folders});
 assert.equal(values.terminalProfiles.length,260);assert.equal(values.ftpProfiles.length,230);
});

test('a full list refuses another entry in plain words',()=>{
 assert.equal(shared.full('terminalProfiles',shared.SAVED_LIMITS.terminalProfiles-1),'');
 const message=shared.full('terminalProfiles',shared.SAVED_LIMITS.terminalProfiles);
 assert.match(message,/saved sessions/i);assert(message.includes(String(shared.SAVED_LIMITS.terminalProfiles)));
 assert.equal(shared.full('terminalRecent',5000),'','a rolling list is never full');
});

// The file browser's "save profile" for SSH and FTP.
function remoteBrowser(state){
 const notes=[],writes=[];
 const c={state,document:{addEventListener(){},querySelector(){return null},getElementById(){return null}},esc:String,icon:()=>'',saveState:()=>writes.push(JSON.stringify(state)),console,NetworkToolkitSessionFolders:folders,NetworkToolkitSharedData:shared};
 c.window=c;vm.createContext(c);vm.runInContext(read('assets/remote-browser.js'),c);
 return {remote:c.NetworkToolkitRemoteBrowser,writes,notes};
}
for(const [kind,key,make] of [['ssh','terminalProfiles',ssh],['ftp','ftpProfiles',ftp]])test('editing one of 150 '+kind+' file-browser profiles keeps all 150, and a full list refuses a new one',()=>{
 const state={terminalProfiles:[],ftpProfiles:[]};state[key]=many(150,make);
 const {remote,writes}=remoteBrowser(state),save=name=>kind==='ssh'?remote.saveSharedSSHProfile(name):remote.saveFTPProfile(name);
 const edited=state[key][120];
 Object.assign(remote.configurations[kind],edited);remote.get(kind).profileId=edited.id;save('Renamed');
 assert.equal(state[key].length,150);assert.equal(state[key].find(row=>row.id===edited.id).name,'Renamed');
 assert.deepEqual(Array.from(state[key],row=>row.id).sort(),many(150,make).map(row=>row.id).sort());
 // Full: a new profile is refused and nothing changes; an edit still works.
 const limit=shared.SAVED_LIMITS[key];state[key]=many(limit,make);const before=JSON.stringify(state[key]),saves=writes.length;
 Object.assign(remote.configurations[kind],make(9999),{host:'new.example.com'});remote.get(kind).profileId='';remote.get(kind).editingProfileId='';save('One too many');
 assert.equal(JSON.stringify(state[key]),before,'a full list is not changed');assert.equal(writes.length,saves,'nothing is saved');
 Object.assign(remote.configurations[kind],state[key][limit-1]);remote.get(kind).profileId=state[key][limit-1].id;save('Still editable');
 assert.equal(state[key].length,limit);assert.equal(state[key].find(row=>row.id===make(limit-1).id).name,'Still editable');
});

// "Save" in the terminal connection form (saveTerminalProfile in index.html).
test('saving one of 150 sessions from the connection form keeps all 150, and a full list refuses a new one',()=>{
 const toasts=[];let saves=0;
 const fields={terminalProfileName:{value:'Renamed'},terminalProfileFolder:{value:''}};
 const c={state:{terminalProfiles:many(150,ssh)},terminalProfileID:'ssh-120',console,Date,Math,
  $:selector=>fields[selector.slice(1)]||null,captureTerminalConnection:()=>({type:'ssh',host:'192.0.2.7',port:22,username:'operator'}),
  safeTerminalConfig:value=>({type:value.type,host:value.host,port:value.port,username:value.username}),
  NetworkToolkitTerminalPromptLabels:{fallback:()=>'Fallback',revision:()=>0},NetworkToolkitSessionFolders:folders,NetworkToolkitSharedData:shared,
  saveState:()=>{saves++;},renderTerminalProfiles(){},renderTerminalChrome(){},toast:(text,kind)=>toasts.push([kind,text])};
 c.globalThis=c;vm.createContext(c);
 vm.runInContext('let terminalProfileID="ssh-120";'+html.split('\n').find(line=>line.startsWith('function saveTerminalProfile(')),c);
 vm.runInContext('saveTerminalProfile()',c);
 assert.equal(c.state.terminalProfiles.length,150);assert.equal(c.state.terminalProfiles.find(row=>row.id==='ssh-120').name,'Renamed');assert.equal(saves,1);
 const limit=shared.SAVED_LIMITS.terminalProfiles;c.state.terminalProfiles=many(limit,ssh);const before=JSON.stringify(c.state.terminalProfiles);
 vm.runInContext('terminalProfileID="";saveTerminalProfile()',c);
 assert.equal(JSON.stringify(c.state.terminalProfiles),before,'a full list is not changed');assert.equal(saves,1,'nothing is saved');
 assert.equal(toasts.at(-1)[0],'error');assert.match(toasts.at(-1)[1],/saved sessions/i);
 // A session another app removed meanwhile is a new entry again.
 vm.runInContext('terminalProfileID="gone-elsewhere";saveTerminalProfile()',c);
 assert.equal(JSON.stringify(c.state.terminalProfiles),before,'re-adding a removed session is refused when the list is full');
 vm.runInContext('terminalProfileID="ssh-3";saveTerminalProfile()',c);
 assert.equal(c.state.terminalProfiles.length,limit,'an existing session can still be changed');assert.equal(saves,2);
});

// Saved TFTP profiles (main window only).
test('a full list of TFTP profiles refuses a new one and keeps the others',()=>{
 const toasts=[];let saves=0;
 const limit=shared.SAVED_LIMITS.tftpProfiles,profile=index=>({id:'tftp-'+index,name:'Transfer '+index,host:'192.0.2.9',operation:'download',remotePath:'a',localPath:'b',port:69,blockSize:512,timeoutSec:5,retries:3,overwrite:false});
 const state={target:'192.0.2.50',forms:{tftp:{operation:'download',remotePath:'x',localPath:'y',port:69}},tftpProfiles:many(limit,profile),tftpRecent:[],activePage:'tftp'};
 const c={state,console,Date,Math,Object,String,Number,Array,document:{addEventListener(){},getElementById:()=>null,querySelector:()=>null},prompt:()=>'One too many',toast:(text,kind)=>toasts.push([kind,text]),saveState:()=>{saves++;},NetworkToolkitSharedData:shared};
 c.window=c;vm.createContext(c);vm.runInContext(read('assets/tftp-connections.js'),c);
 const before=JSON.stringify(state.tftpProfiles);
 c.NetworkToolkitTFTPConnections.save();
 assert.equal(JSON.stringify(state.tftpProfiles),before);assert.equal(saves,0);assert.equal(toasts.at(-1)[0],'error');
 // Saving over a profile with the same name and host is an edit.
 state.target='192.0.2.9';c.prompt=()=>'Transfer 4';c.NetworkToolkitTFTPConnections.save();
 assert.equal(state.tftpProfiles.length,limit);assert.equal(saves,1);
});

test('the Saved sidebar and the saved targets form ask the same limit',()=>{
 assert(/NetworkToolkitSharedData\?\.full\('terminalProfiles'/.test(read('assets/session-sidebar.js')),'Save session in the sidebar');
 assert(/NetworkToolkitSharedData\?\.full\('profiles'/.test(html),'the saved target form');
});

// Rolling lists (recent connections, learned names) are cut on purpose: the
// oldest entries leave. Wherever the page cuts one, it cuts at the number of
// the engine's table, so no part of the page keeps fewer than another.
test('every cut of a rolling list uses the number of the engine table',()=>{
 const files=['index.html',...fs.readdirSync(path.join(web,'assets')).filter(name=>name.endsWith('.js')).map(name=>'assets/'+name)];
 const cuts=[];
 for(const file of files){
  const text=read(file);
  for(const key of Object.keys(shared.ROLLING_LIMITS)){
   const assignment=new RegExp('(?:state|out)\\.'+key+'\\s*=(?!=)[^;]*','g');let match;
   while((match=assignment.exec(text))){
    // The cut of the list itself: ".slice(0,n)" after an array literal or an array method.
    for(const cut of match[0].matchAll(/(?:\]|\)\s*)\.slice\(\s*0\s*,\s*([A-Za-z_.$\d]+)\s*\)/g)){
     const before=match[0].slice(0,cut.index+1);
     if(/\.(?:id|name|host|username|fingerprint|credentialId|trim\(\)|toString\(\d*\))$/.test(before))continue; // a text, not the list
     cuts.push([file,key,cut[1]]);
    }
   }
  }
 }
 assert(cuts.length>=6,'the cuts of rolling lists were found: '+JSON.stringify(cuts));
 for(const [file,key,amount] of cuts){
  const number=/^\d+$/.test(amount)?Number(amount):amount==='ROLLING_LIMITS.'+key?shared.ROLLING_LIMITS[key]:NaN;
  assert.equal(number,shared.ROLLING_LIMITS[key],file+' cuts '+key+' at '+amount+', the table says '+shared.ROLLING_LIMITS[key]);
 }
 // Learned prompt names are cut inside their own module.
 assert.equal(Number((/const LIMIT=(\d+)/.exec(read('assets/terminal-prompt-labels.js'))||[])[1]),shared.ROLLING_LIMITS.terminalPromptNames);
});
