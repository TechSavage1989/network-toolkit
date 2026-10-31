'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const file=fs.readFileSync(path.join(__dirname,'terminal-profiles130-native-driver.js'),'utf8');
test('native profile snapshot observes noninteractive protocol badge and automatic provenance without credentials',()=>{
 const p={id:'native-profile114',name:'127.0.0.1',nameExplicit:false,host:'127.0.0.1',password:'synthetic-not-for-report'};
 const badge={textContent:'Telnet',tagName:'SPAN',hasAttribute:()=>false,getBoundingClientRect:()=>({width:32,height:14})};const row={querySelector:()=>badge};
 const root={state:{terminalProfiles:[p]},safeTerminalConfig:v=>v,document:{querySelector:()=>row,querySelectorAll:s=>s.includes('toast')?[]:[row]},Map,Date,lastStorageError:''};root.window=root;vm.runInNewContext(file,root);
 const s=root.__nativeProfiles130Snapshot({info:{id:'session'}});assert.equal(s.explicit,false);assert.equal(s.tabBadge.text,'Telnet');assert.equal(s.savedBadge.visible,true);assert.equal(s.recentBadge.noninteractive,true);assert.doesNotMatch(JSON.stringify(s),/synthetic-not-for-report|password/);
});
test('profile native flow uses actual Save, authenticated preferences read, reconnect and scoped backend without fabricated API responses',()=>{
 assert.match(file,/data-terminal-action="save-profile"/);assert.match(file,/await api\('\/state'\)/);assert.match(file,/mergeTerminalPreferences\(saved\)/);assert.doesNotMatch(file,/location\.reload\(/);assert.match(file,/data-terminal-action="reconnect"/);assert.doesNotMatch(file,/fetch\s*=|\.route\(|dispatchEvent/);
 const go=fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/terminal_native_profiles130_windows_test.go'),'utf8');
 assert.match(go,/accepted\.Load\(\) != 1/);assert.match(go,/accepted\.Load\(\) != 2/);assert.match(go,/s\.Selection == before\.Selection/);assert.match(go,/reflect\.DeepEqual\(identity, after\)/);assert.match(go,/s\.Profiles130\.BootAt != oldBoot/);assert.match(go,/s\.Profiles130\.PreferencesReads > oldReads/);
});

test('single-session native child asserts scoped session metadata, never global saved-profile exposure',()=>{
 const go=fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/terminal_native_profiles130_windows_test.go'),'utf8');
 assert.match(go,/s\.Profiles130\.ProfileID == "" && s\.Profiles130\.StoredName == ""/);
 assert.match(go,/!s\.NameExplicit && s\.ManualName == "" && s\.NameRevision == before\.NameRevision/);
 assert.match(go,/parent persisted profile provenance changed across narrow child bootstrap\/return/);
});

 test('native reconnect waits real serialized persistence and rejects asynchronous errors',()=>{
 assert.match(file,/const persisted=await flushState\(\);await stateWrites/);
 assert.match(file,/savedStateRevision!==stateRevision\|\|lastStorageError/);
 assert.match(file,/stored.nameRevision!==local.nameRevision/);
 assert.match(file,/stored.nameExplicit!==false/);
 const go=fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/terminal_native_profiles130_windows_test.go'),'utf8');
 assert.match(go,/reconnected Recent automatic provenance was not persisted/);
 assert.match(go,/final.Profiles130.ErrorToastCount != 0/);
 });

test('actual native reconnect action rejects failed save even when transport connected',async()=>{
 const entry={info:{id:'synthetic',state:'connected',type:'telnet',host:'127.0.0.1',port:23},reconnectBusy:false};
 let writes=0;
 const root={state:{terminalProfiles:[],terminalRecent:[]},safeTerminalConfig:v=>v,Map,Date,lastStorageError:'revision rejected',stateRevision:3,savedStateRevision:2,stateWrites:Promise.resolve(),flushState:async()=>{writes++;return false;},document:{querySelectorAll:s=>s.includes('disconnect')?[{disabled:false,getBoundingClientRect:()=>({width:1}),click(){entry.info.state='closed';}}]:s.includes('reconnect')?[{disabled:false,getBoundingClientRect:()=>({width:1}),click(){entry.info.state='connected';}}]:[]}};
 root.window=root;vm.runInNewContext(file,root);
 await assert.rejects(root.__nativeProfiles130({action:'profiles130:reconnect'},()=>({}),()=>entry,async f=>{assert.ok(f());}),/Reconnect preferences did not persist cleanly/);
 assert.equal(writes,1);assert.equal(entry.info.state,'connected');
});
