'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const file=fs.readFileSync(path.join(__dirname,'terminal-profiles130-native-driver.js'),'utf8');
test('native profile snapshot observes noninteractive protocol badge and automatic provenance without credentials',()=>{
 const p={id:'native-profile114',name:'127.0.0.1',nameExplicit:false,host:'127.0.0.1',password:'synthetic-not-for-report'};
 const badge={textContent:'Telnet',tagName:'SPAN',hasAttribute:()=>false,getBoundingClientRect:()=>({width:32,height:14})};const row={querySelector:()=>badge};
 const root={state:{terminalProfiles:[p]},safeTerminalConfig:v=>v,document:{querySelector:()=>row,querySelectorAll:()=>[row]},Map,Date};root.window=root;vm.runInNewContext(file,root);
 const s=root.__nativeProfiles130Snapshot({info:{id:'session'}});assert.equal(s.explicit,false);assert.equal(s.tabBadge.text,'Telnet');assert.equal(s.savedBadge.visible,true);assert.equal(s.recentBadge.noninteractive,true);assert.doesNotMatch(JSON.stringify(s),/synthetic-not-for-report|password/);
});
test('profile native flow uses actual Save, authenticated preferences read, reconnect and scoped backend without fabricated API responses',()=>{
 assert.match(file,/data-terminal-action="save-profile"/);assert.match(file,/await api\('\/state'\)/);assert.match(file,/mergeTerminalPreferences\(saved\)/);assert.doesNotMatch(file,/location\.reload\(/);assert.match(file,/data-terminal-action="reconnect"/);assert.doesNotMatch(file,/fetch\s*=|\.route\(|dispatchEvent/);
 const go=fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/terminal_native_profiles130_windows_test.go'),'utf8');
 assert.match(go,/accepted\.Load\(\) != 1/);assert.match(go,/accepted\.Load\(\) != 2/);assert.match(go,/s\.Selection == before\.Selection/);assert.match(go,/reflect\.DeepEqual\(identity, after\)/);assert.match(go,/s\.Profiles130\.BootAt != oldBoot/);assert.match(go,/s\.Profiles130\.PreferencesReads > oldReads/);
});
