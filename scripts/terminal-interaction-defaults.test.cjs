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
test('terminal font size limits (8 to 36 px) agree in zoom, the Display field, startup restore and Go',()=>{
 const sizes=JSON.parse(source.match(/const FONT_SIZES = (\[[^\]]+\])/)[1]);assert.equal(sizes[0],8);assert.equal(sizes.at(-1),36);
 assert.match(source,/Math\.max\(8,Math\.min\(36,/);
 const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
 assert.match(html,/id="terminalFontSize" class="terminal-font" min="8" max="36" step="1" data-input-clear="off"/);assert.match(html,/el\.id==='terminalFontSize'\)\{const size=Number\(el\.value\);if\(Number\.isInteger\(size\)&&size>=8&&size<=36\)/);
 assert.match(fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/terminal_settings.go'),'utf8'),/size < 8 \|\| size > 36|size >= 8 && size <= 36/);
 // The separate Terminal window uses the shared font size (shared_data.go), which passes the same sanitizer; it is no longer copied when the window is first used.
 const shared=fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/shared_data.go'),'utf8');assert.match(shared,/\{Name: "terminalFontSize", Kind: sharedValue, Live: true\}/);assert.match(shared,/sanitizeTerminalSettings\(picked\)/);
});
