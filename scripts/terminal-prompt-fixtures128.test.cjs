'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
test('128 actual rendered prompt-family fixture bytes are printable and match the production parser',()=>{
 const browser=fs.readFileSync(require.resolve('./terminal-prompt-labels127.browser.cjs'),'utf8');
 const literal=browser.match(/const familyChecks=\[\],families=(\[[\s\S]*?\n \]);/);
 assert(literal,'Locate the actual browser fixture array, not a duplicated copy');
 const families=vm.runInNewContext('('+literal[1]+')');assert.equal(families.length,12);
 const window={};vm.runInNewContext(fs.readFileSync(require.resolve('../web/assets/terminal-prompt-labels.js'),'utf8'),{window});
 let previous='';for(const [prompt,want] of families){
  assert(!/[\x00-\x1f\x7f-\x9f]/.test(prompt),'Unexpected control in fixture '+JSON.stringify(prompt));
  assert.equal(window.NetworkToolkitTerminalPromptLabels.parse(prompt,previous),want,prompt);previous=want;
 }
 assert(families.some(([prompt])=>prompt===String.raw`[SERVER-LAB]: PS C:\tools>`),'PowerShell path must retain its literal backslash');
});
