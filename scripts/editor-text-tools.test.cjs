'use strict';
// Unit checks for the Text Editor's text tools page module (web/assets/
// editor-text-tools.js): the list every menu shows, the Find bar's Extended
// escapes, and the Replace template of the Regular expression search mode,
// which must expand exactly as the engine does (the cases are shared with
// internal/editorbuffer/rewrite_test.go).
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{test}=require('node:test');
const tools=require('../web/assets/editor-text-tools.js');

test('every tool has a group, a label, a description and a message; shortcuts are unique',()=>{
 const ids=new Set(),shortcuts=new Set(),groups=new Set(tools.GROUPS.map(group=>group.id));
 for(const tool of tools.TOOLS){
  assert(!ids.has(tool.id),tool.id+' twice');ids.add(tool.id);
  assert(groups.has(tool.group),tool.id+' has no group');
  assert(tool.label&&tool.description&&tool.description.length<=240,tool.id+' needs a label and a short description');
  if(tool.engine&&tool.id!=='sonicwall-readable')assert(tool.done&&tool.same,tool.id+' needs its messages');
  if(tool.shortcut){assert(!shortcuts.has(tool.shortcut),tool.shortcut+' twice');shortcuts.add(tool.shortcut);}
 }
 for(const group of tools.GROUPS)assert(tools.TOOLS.some(tool=>tool.group===group.id),group.id+' is empty');
 // The existing actions and shortcuts are still there.
 for(const [id,shortcut] of [['comment','Ctrl+/'],['duplicate','Ctrl+D'],['format','Ctrl+Shift+F']])assert.equal(tools.tool(id).shortcut,shortcut);
 // Names the toolbar and menus used before still run the same tools.
 for(const [old,now] of [['sortLines','sort-asc'],['trim','trim-trailing'],['textToHex','hex-encode'],['hexToText','hex-decode'],['base64Encode','base64-encode'],['base64Decode','base64-decode'],['upper','upper'],['lower','lower']])assert.equal(tools.tool(old).id,now);
 // The shortcuts do not clash with the editor's own or the app's.
 for(const taken of ['Ctrl+Z','Ctrl+Y','Ctrl+S','Ctrl+Shift+S','Ctrl+F','Ctrl+H','Ctrl+G','Ctrl+N','Ctrl+O','Ctrl+Shift+P','Ctrl+Shift+T','Ctrl+Shift+Enter'])assert(!shortcuts.has(taken),taken+' is taken');
});

test('Extended escapes in Find and Replace',()=>{
 assert.deepEqual(tools.decodeExtended('&'),{text:'&'});
 assert.deepEqual(tools.decodeExtended('a\\nb\\tc\\\\d'),{text:'a\nb\tc\\d'});
 assert.deepEqual(tools.decodeExtended('\\r\\n|\\r|\\x41\\u00e9|\\q'),{text:'\n|\n|Aé|\\q'});
 for(const nul of ['\\0','\\x00','\\u0000'])assert.match(tools.decodeExtended(nul).error,/NUL/);
});

test('the Replace template expands as the engine does',()=>{
 const file=JSON.parse(fs.readFileSync(path.join(__dirname,'..','internal','editorbuffer','testdata','replace-template.json'),'utf8'));
 for(const c of file.cases){
  const pattern=new RegExp(c.pattern),match=pattern.exec(c.text);
  if(c.error){assert.throws(()=>tools.expandReplacement(c.replacement,match,pattern.source),/NUL/,c.replacement);continue;}
  assert.equal(tools.expandReplacement(c.replacement,match,pattern.source),c.want,c.pattern+' with '+c.replacement);
  // Replace All over page-held text, as the Find bar does it.
  const all=c.text.replace(new RegExp(c.pattern,'g'),(...args)=>tools.expandReplacement(c.replacement,args.slice(0,typeof args.at(-1)==='object'?args.length-3:args.length-2),c.pattern));
  assert.equal(all,c.text.slice(0,match.index)+c.want+c.text.slice(match.index+match[0].length));
 }
 assert.deepEqual(tools.groupNames('(a)(?:b)(?<name>c)[(]\\(d(?=e)'),['','','name']);
});

test('line tools widen a selection to whole lines; positions are lines and columns',()=>{
 const text='first\nsecond\nthird';
 assert.deepEqual(tools.lineRange(text,2,3),[0,5]);
 assert.deepEqual(tools.lineRange(text,2,8),[0,12]);
 assert.deepEqual(tools.lineRange(text,6,13),[6,12]);
 assert.deepEqual(tools.lineRange(text,14,15),[13,18]);
 assert.deepEqual(tools.lineColumn(text,8),[2,3]);
 assert.deepEqual(tools.lineColumn(text,0),[1,1]);
});
