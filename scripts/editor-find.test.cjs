'use strict';
// The Find window's page helpers (web/assets/editor-find.js): the row model
// of the Search results panel, the marked text of a row, bookmarks that
// follow edits, the syntax guide and the line that explains the cursor.
// The searches themselves run in the engine (internal/editorbuffer).
const test=require('node:test'),assert=require('node:assert/strict');
const find=require('../web/assets/editor-find.js');

test('the results panel lays out searches, files and rows, collapsed or not',()=>{
 const searches=[{collapsed:false,files:[{rows:3,collapsed:false},{rows:200000,collapsed:false}]},{collapsed:true,files:[{rows:5,collapsed:false}]}];
 const model=find.layout(searches);
 assert.equal(model.total,1+1+3+1+200000+1);
 assert.equal(find.rowAt(model,0).type,'search');
 assert.equal(find.rowAt(model,1).type,'file');
 assert.deepEqual([find.rowAt(model,2).type,find.rowAt(model,2).index],['line',0]);
 assert.equal(find.rowAt(model,5).type,'file');
 const last=find.rowAt(model,model.total-2);
 assert.deepEqual([last.type,last.index],['line',199999]);
 assert.equal(find.rowAt(model,model.total-1).type,'search','the collapsed search shows only its header');
 searches[0].files[1].collapsed=true;
 assert.equal(find.layout(searches).total,1+1+3+1+1);
});

test('a row marks its hits, cut to its text, and escapes the text',()=>{
 const row={line:3,start:100,text:'a <b> vlan vlan',cutStart:true,cutEnd:true,hits:[[106,110],[111,115],[90,101]]};
 const html=find.rowHTML({...row,hits:row.hits.slice(0,2)});
 assert.equal(html,'<span class="nt-find-cut">…</span>a &lt;b&gt; <mark data-hit="0">vlan</mark> <mark data-hit="1">vlan</mark><span class="nt-find-cut">…</span>');
 // A hit that starts before the row's text is cut to it.
 assert.match(find.rowHTML({line:0,start:100,text:'abc',hits:[[90,101]]}),/^<mark data-hit="0">a<\/mark>bc$/);
});

test('bookmarks move with the lines an edit adds or removes',()=>{
 assert.deepEqual(find.shiftLines([2,10,20],{line:5,removed:0,inserted:2}),[2,12,22]);
 assert.deepEqual(find.shiftLines([2,10,20],{line:5,removed:6,inserted:1}),[2,6,15],'a bookmark on a removed line moves to the edit\'s last line');
 assert.deepEqual(find.shiftLines([2,6,20],{line:5,removed:6,inserted:3}),[2,6,17],'one inside the edit stays on its line while it exists');
 assert.deepEqual(find.shiftLines([5,6,7],{line:5,removed:2,inserted:0}),[5],'and never twice');
 assert.deepEqual(find.shiftLines([0,3],{line:3,removed:0,inserted:4}),[0,3],'the edited line itself keeps its bookmark');
 // Replace All of a word on lines 5 and 40, a sort, Undo of such a change:
 // as many lines as before, every bookmark stays (one stretch of 35 lines).
 assert.deepEqual(find.shiftLines([10,20,30],{line:5,removed:35,inserted:35}),[10,20,30]);
});

test('the range of a mark made In selection follows the text it holds',()=>{
 assert.deepEqual(find.shiftRange([10,20],{from:0,to:0,length:5}),[15,25],'text typed before it');
 assert.deepEqual(find.shiftRange([10,20],{from:25,to:30,length:0}),[10,20],'text removed after it');
 assert.deepEqual(find.shiftRange([10,20],{from:12,to:14,length:6}),[10,24],'an edit inside it');
 assert.deepEqual(find.shiftRange([10,20],{from:5,to:12,length:1}),[5,14],'an edit across its start');
 assert.deepEqual(find.shiftRange([10,20],{from:20,to:20,length:3}),[10,20],'text typed at its end stays outside');
 assert.deepEqual(find.shiftRange([10,20],{from:10,to:10,length:3}),[13,23],'text typed at its start stays outside');
});

test('the syntax guide lists each mode, and Replace with on the Replace tab',()=>{
 const tokens=list=>list.map(row=>row[0]);
 assert.deepEqual(tokens(find.GUIDE.extended),['\\n','\\r','\\t','\\\\','\\xNN','\\uNNNN']);
 for(const token of ['.','\\d  \\D','\\w  \\W','\\s  \\S','\\b','^  $','*','+','?','{n}  {n,m}','[abc]','[^abc]','[a-z]','( )','(?: )','(?<name> )','|','\\','(?i)'])assert(tokens(find.GUIDE.regex).includes(token),token);
 assert.deepEqual(tokens(find.GUIDE.replace),['$1','${name}','$&  $0','$$','\\n','\\t']);
 for(const list of Object.values(find.GUIDE))for(const [token,insert,meaning,example] of list){assert(token&&insert&&meaning&&example,token);assert(insert.split('‸').length<=2,token);}
});

test('the line under Find what explains the part at the cursor',()=>{
 const regex=(text,at)=>find.explain(text,at,'regex');
 assert.equal(regex('port \\d+',7),'\\d — a digit 0–9');
 assert.equal(regex('port \\d+',8),'+ — the item before, once or more');
 assert.equal(regex('a.*?b',4),'*? — the item before, any number of times (also none), as few as possible');
 assert.equal(regex('(?<port>\\d+)',3),'(?<port> — starts group 1, named “port”: ${port} in Replace with');
 assert.equal(regex('(a)(b)',4),'( — starts group 2: $2 in Replace with');
 assert.equal(regex('[^0-9]',2),'[^0-9] — one character that is not the characters and ranges “0-9”');
 assert.equal(regex('\\d{2,4}',4),'{2,4} — the item before, 2 to 4 times');
 assert.equal(regex('foo(?=x)',5),'(?= — lookahead and lookbehind are not supported');
 assert.equal(regex('(\\w+) \\1',8),'\\1 — backreferences such as \\1 are not supported: use $1 in Replace with');
 assert.equal(regex('^end$',5),'$ — the end of a line');
 assert.equal(find.explain('$2=$1',2,'regex','replace'),'$2 — the text of group 2');
 assert.equal(find.explain('a\\tb',3,'extended'),'\\t — a tab');
 assert.match(find.explain('\\0',2,'extended'),/NUL/);
 assert.equal(find.explain('anything',3,'normal'),'','nothing to explain in Normal mode');
});
