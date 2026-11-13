'use strict';
// The Document Map's and the Document List's models (editor-document-map.js,
// editor-document-list.js): what a map row shows of a line, how the map fits
// a document and its visible part, and the rows of the list. The drawing and
// the pointer, wheel and keys are checked in the shipped page by
// scripts/editor-map-list.browser.cjs.
const assert=require('node:assert/strict'),{test}=require('node:test');
const map=require('../web/assets/editor-document-map.js'),list=require('../web/assets/editor-document-list.js');
const {TEXT,COMMENT,STRING,KEYWORD,NUMBER}=map.kinds;

test('comment markers: the language\'s own, plus ! and # for device configurations',()=>{
 assert.deepEqual(map.commentMarkers(null,'network-cisco'),['!','#']);
 assert.deepEqual(map.commentMarkers(['//'],'javascript'),['//']);
 assert.deepEqual(map.commentMarkers(['#'],'python'),['#']);
 assert.deepEqual(map.commentMarkers(['<!--','-->'],'network-panos-xml'),['<!--','!','#']);
 assert.deepEqual(map.commentMarkers(null,'text'),[]);
});

test('a line becomes word blocks: keyword, text, number, string and a comment to the end',()=>{
 const line='interface Gi0/1 ip 192.0.2.1 "uplink" ! checked';
 assert.deepEqual(map.lineRuns(line,0,line.length,{markers:['!'],keywords:true}),[[0,9,KEYWORD],[10,5,TEXT],[16,2,TEXT],[19,9,NUMBER],[29,8,STRING],[38,9,COMMENT]]);
 assert.deepEqual(map.lineRuns(line,0,line.length,{markers:['!'],keywords:false})[0],[0,9,TEXT],'plain text has no keywords');
 // Tabs move to the next tab stop; a marker inside a word is not a comment.
 assert.deepEqual(map.lineRuns('\tgo http://example.com',0,22,{tab:4,markers:['//']}),[[4,2,TEXT],[7,18,TEXT]]);
 assert.deepEqual(map.lineRuns('  // note',0,9,{markers:['//']}),[[2,7,COMMENT]]);
 // Part of a larger text, without its line break.
 const text='first\r\n  second 7\nthird';
 assert.deepEqual(map.lineRuns(text,7,text.indexOf('\n',7),{}),[[2,6,TEXT],[9,1,NUMBER]]);
});

test('a line is read only up to its limit, however long it is',()=>{
 const line='x'.repeat(10*1024*1024),started=performance.now();
 const runs=map.lineRuns(line,0,line.length,{limit:120});
 assert.deepEqual(runs,[[0,240,TEXT]],'one word, read to twice the limit at most');
 assert(performance.now()-started<20,'a 10 MB line costs nothing');
 const words=Array.from({length:5000},(_,i)=>'w'+i).join(' ');
 const many=map.lineRuns(words,0,words.length,{limit:100});
 assert(many.every(([col])=>col<100)&&many.length<40,'only the columns the map shows');
 const shape=map.lineShape(' '.repeat(5000)+'x',0,5001,{limit:240});
 assert.equal(shape.blank,false);assert.equal(shape.indent,240,'indent is counted up to the limit');
});

test('line shape: indent, length, blank and comment lines',()=>{
 assert.deepEqual(map.lineShape('    description x',0,17,{markers:['!']}),{indent:4,length:17,blank:false,comment:false});
 assert.deepEqual(map.lineShape('\tx',0,2,{tab:8}),{indent:8,length:9,blank:false,comment:false});
 assert.equal(map.lineShape('   ',0,3).blank,true);
 assert.equal(map.lineShape('',0,0).blank,true);
 assert.equal(map.lineShape(' ! interface notes',0,18,{markers:['!']}).comment,true);
 assert.equal(map.lineShape('line\r',0,5).length,4,'a carriage return is not text');
});

test('geometry: three pixels per row while the document fits, the whole document otherwise',()=>{
 // 100 rows of 20 px in a 600 px map: 300 px, a 10-row view at row 40.
 let g=map.geometry({total:2000,rows:100,height:600,start:800,end:1000});
 assert.equal(g.used,300);assert.equal(g.scale,0.15);assert.deepEqual(g.box,{top:120,height:30});
 assert.equal(map.targetAt(g,g.box.top+5,5),800,'a pointer on the box top maps back to the view');
 // 100,000 rows: fitted to the map height; the box keeps a minimum size.
 g=map.geometry({total:2e6,rows:1e5,height:600,start:1e6,end:1e6+400});
 assert.equal(g.used,600);assert.equal(g.box.height,map.MIN_BOX);assert.equal(g.box.top,300);
 // At the very end the box stays inside the map.
 g=map.geometry({total:2e6,rows:1e5,height:600,start:2e6-400,end:2e6});
 assert.equal(g.box.top+g.box.height,600);
 // Clicks never ask for a position before the start or past the last view.
 assert.equal(map.targetAt(g,-50,0),0);assert.equal(map.targetAt(g,5000,0),2e6-400);
 // A document shorter than the view: the box covers all of it.
 g=map.geometry({total:300,rows:15,height:600,start:0,end:600});
 assert.equal(g.used,45);assert.deepEqual(g.box,{top:0,height:45});
});

test('document list: tab order, extension column, and the views of a split editor',()=>{
 assert.deepEqual(list.splitName('core-router.cfg'),{base:'core-router',ext:'cfg'});
 assert.deepEqual(list.splitName('archive.tar.gz'),{base:'archive.tar',ext:'gz'});
 assert.deepEqual(list.splitName('.bashrc'),{base:'.bashrc',ext:''},'a leading dot is part of the name');
 assert.deepEqual(list.splitName('Makefile'),{base:'Makefile',ext:''});
 assert.deepEqual(list.splitName('trailing.'),{base:'trailing.',ext:''});
 assert.deepEqual(list.splitName(''),{base:'Untitled',ext:''});
 const doc=(uid,name,extra={})=>({uid,id:'local:'+uid,name,title:name,dirty:false,saving:false,active:false,shown:false,...extra});
 const single=list.rows({views:[{side:'single',label:'',documents:[doc('a','a.txt',{active:true}),doc('b','b.json',{dirty:true})]}]});
 assert.deepEqual(single.map(row=>[row.uid,row.base,row.ext,row.group===true]),[['a','a','txt',false],['b','b','json',false]],'one view: no headings');
 const split=list.rows({views:[{side:'left',label:'Left view',documents:[doc('a','a.txt')]},{side:'right',label:'Right view',documents:[doc('b','b.json',{active:true}),doc('c','c.cfg')]}]});
 assert.deepEqual(split.map(row=>row.group?'['+row.label+' '+row.count+']':row.uid+':'+row.side),['[Left view 1]','a:left','[Right view 2]','b:right','c:right']);
 assert.deepEqual(list.rows({}),[]);
});
