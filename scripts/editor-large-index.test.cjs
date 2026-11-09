'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
const {replace,plan,update}=require('../web/assets/editor-large-index.js');
const starts=text=>{const result=[0];for(let i=0;i<text.length;i++)if(text[i]==='\n')result.push(i+1);return result;};
test('inserting, deleting, selecting and replacing preserve exact logical line starts',()=>{
 for(const [old,from,to,insert] of [
  ['ab\ncd',0,0,'hello\n'],['ab\ncd',3,3,'x'],['ab\ncd',1,2,''],['ab\ncd',0,3,''],['ab\ncd',0,0,'x'],
  ['a\n',2,2,'\n'],['a\n\nb',1,3,'X\nY'],['x\ny\nz',1,4,''],['',0,0,'line\n'],
  ['a\r\nb',0,0,'Z'],['α😊\nλ',0,1,'β'],['x'.repeat(100000)+'\ny',100001,100001,'\n'],
 ]){const next=old.slice(0,from)+insert+old.slice(to);assert.deepEqual(replace(starts(old),old.length,next.length,from,to,insert),starts(next));}
});
test('input plans handle typing, selection replacements, backspace and forward delete',()=>{
 const old='first\nsecond\nthird',cache={text:old,starts:starts(old)};
 const cases=[
  {from:11,to:11,type:'insertText',data:'X',next:'first\nseconXd\nthird',caret:12},
  {from:6,to:12,type:'insertText',data:'middle\n',next:'first\nmiddle\n\nthird',caret:13},
  {from:8,to:8,type:'deleteContentBackward',data:null,next:'first\nscond\nthird',caret:7},
  {from:6,to:6,type:'deleteContentForward',data:null,next:'first\necond\nthird',caret:6},
 ];
 for(const c of cases){const p=plan(old,1,c.from,c.to,c.type,c.data);const result=update(cache,old,c.next,p,c.caret);assert.deepEqual(result,starts(c.next),c.type);}
});
test('rejects inconsistent and invalid edit plans without guessing document positions',()=>{
 assert.equal(replace([0,2],4,6,3,1,'x'),null);
 const old='hello\nworld',cached={text:old,starts:starts(old)},p=plan(old,2,2,2,'insertText','x');
 assert.equal(update(cached,old,'hello\nWORLD',p,3),null);
 assert.equal(update({...cached,text:'wrong'},old,'helxlo\nworld',p,3),null);
 assert.equal(update(cached,old,'helxlo\nworld',p,4),null);
 assert.equal(update(cached,old,'helxlo\nworld',plan(old,2,2,2,'insertText','z'),3),null);
});
test('a 16 MiB file adjusts indices without scanning the entire document',()=>{
 const old=('0123456789'.repeat(100)+'\n').repeat(16760),cached={text:old,starts:starts(old)},at=old.length;
 const next=old+'X\n',result=update(cached,old,next,plan(old,1,at,at,'insertText','X\n'),at+2);
 assert.deepEqual(result,starts(next));assert.equal(result.length,cached.starts.length+1);
});
