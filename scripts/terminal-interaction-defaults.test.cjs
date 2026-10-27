'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(path.join(__dirname,'../web/assets/terminal-interaction.js'),'utf8');
function harness(value){const c={window:{},state:{terminalInteraction:value}};vm.runInNewContext(source,c);return {api:c.window.NetworkToolkitTerminalInteraction,state:c.state};}
const requested={copyOnSelect:true,copyOnClick:true,pasteReview:true,rightClick:'paste',ctrlC:'copy-selection',autoPager:true,pagerKey:'auto'};
function check(actual,want){for(const [key,value] of Object.entries(want))assert.equal(actual[key],value,key);}
test('new and unset terminal preferences match every visible screenshot default',()=>{
 for(const value of [undefined,null,{}, {autoPager:'false',ctrlC:'invalid'}])check(harness(value).api.preferences(),requested);
});
test('explicit saved off/send choices and nondefault dropdowns are not overwritten',()=>{
 const saved={copyOnSelect:false,copyOnClick:false,pasteReview:false,pasteIncludeCommands:true,autoPager:false,ctrlC:'send',rightClick:'menu',pagerKey:'enter'},h=harness(saved);
 check(h.api.preferences(),saved);check(h.api.safePreferences(JSON.parse(JSON.stringify(h.api.preferences()))),saved);assert.deepEqual(h.state.terminalInteraction,saved,'Normalization does not mutate the stored object');
});
test('partially saved settings fill only absent defaults and empty reset restores requested defaults',()=>{
 const h=harness({autoPager:false});check(h.api.preferences(),{...requested,autoPager:false});h.state.terminalInteraction={ctrlC:'send'};check(h.api.preferences(),{...requested,ctrlC:'send'});h.state.terminalInteraction={};check(h.api.preferences(),requested);
});
test('existing preference load and window transfer use the common normalization path',()=>{
 const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
 assert.match(html,/state\.terminalInteraction=NetworkToolkitTerminalInteraction\.safePreferences\(value\.terminalInteraction\)/);
 assert.match(html,/terminalInteraction:NetworkToolkitTerminalInteraction\.preferences\(\)/);
 const go=fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/terminal_settings.go'),'utf8');
 assert.match(go,/"autoPager":\s+true/);assert.match(go,/"ctrlC":\s+"copy-selection"/);assert.match(go,/if source\["ctrlC"\] == "send"/);
});
