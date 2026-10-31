'use strict';
const fs=require('node:fs'),vm=require('node:vm'),{test}=require('node:test'),assert=require('node:assert/strict');
const source=fs.readFileSync(require.resolve('./terminal-native-driver.js'),'utf8');
const helper=source.slice(source.indexOf(' function nativeTabPointVisible('),source.indexOf('\n function reorderPointsVisible('));
const c={};vm.createContext(c);vm.runInContext(helper,c);
test('native reorder rejects offscreen tab rectangle even when inside window',()=>{
 const tab={left:346,top:400,width:97,height:28},strip={left:10,top:395,width:365,height:40};
 assert.equal(c.nativeTabPointVisible(tab,strip,704,645,true),false);
 assert.equal(c.nativeTabPointVisible(tab,{...strip,width:630},704,645,true),true);
 assert.equal(c.nativeTabPointVisible(tab,{...strip,width:630},400,645,true),false);
 assert.equal(c.nativeTabPointVisible(tab,{...strip,width:630},704,410,true),false);
 assert.equal(c.nativeTabPointVisible({...tab,width:0},strip,704,645,true),false);
});
test('native gesture setup collapses sidebar and waits for real geometry without weakening reorder',()=>{
 assert.match(source,/state\.terminalSidebarCollapsed=true;NetworkToolkitSessionSidebar\.render\(\)/);
 assert.match(source,/await terminalDisplayBarrier\(entry\(\)\);await wait\(reorderPointsVisible/);
 const go=fs.readFileSync(require.resolve('../cmd/network-toolkit/terminal_native_gestures_windows_test.go'),'utf8');
 assert.match(go,/if !before\.ReorderPointsVisible/);
 assert.match(go,/s\.Geometry\.Tabs\[2\]\.UID == h\.session && s\.Editable/);
});
