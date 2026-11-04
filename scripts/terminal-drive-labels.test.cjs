'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
function appFunction(name){const tail=html.slice(html.search(new RegExp('^function '+name+'\\(','m')));return tail.slice(0,tail.slice(1).search(/\n(?:async )?function /)+1);}
const c={};vm.createContext(c);vm.runInContext(appFunction('terminalDriveLabel'),c);
test('starting-directory drive buttons always show the drive letter',()=>{
 assert.equal(c.terminalDriveLabel({path:'D:\\',label:'Data HDD'}),'D: Data HDD');
 assert.equal(c.terminalDriveLabel({path:'E:\\',label:'Games SSD Micron'}),'E: Games SSD Micron');
 assert.equal(c.terminalDriveLabel({path:'C:\\',label:'Local drive'}),'C: · Local drive');
 assert.equal(c.terminalDriveLabel({path:'F:\\',label:''}),'F:');
 assert.equal(c.terminalDriveLabel({path:'G:\\',label:'G:\\'}),'G:');
 assert.equal(c.terminalDriveLabel({path:'H:\\',label:'H: Backup'}),'H: Backup');
 assert.equal(c.terminalDriveLabel({path:'C:\\',label:'Windows (C:)'}),'Windows (C:)');
});
test('the picker renders the labelled drive buttons with the full path as tooltip',()=>{
 assert.match(html,/data-terminal-local-path="'\+esc\(d\.path\)\+'" title="'\+esc\(d\.path\)\+'">'\+esc\(terminalDriveLabel\(d\)\)/);
});
