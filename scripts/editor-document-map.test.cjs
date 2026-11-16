'use strict';
// The Document Map's and the Document List's models (editor-document-map.js,
// editor-document-list.js): what a map row shows of a line, how the map fits
// a document and its visible part, its width and when it shows text, how it
// reads the editor's syntax colours, and the rows of the list. The drawing and
// the pointer, wheel and keys are checked in the shipped page by
// scripts/editor-map-list.browser.cjs.
const assert=require('node:assert/strict'),{test}=require('node:test');
const map=require('../web/assets/editor-document-map.js'),list=require('../web/assets/editor-document-list.js');
const {TEXT,COMMENT,STRING,KEYWORD,NUMBER,PROPERTY}=map.kinds;

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

test('text mode geometry: the map keeps its row height and scrolls; the box is a thumb over the text on screen',()=>{
 // 1000 rows of 20 px (20,000 text-area px), 12 map px per row: 12,000 px of
 // map content in a 600 px map, a 500 px view.
 const at=start=>map.geometry({total:20000,rows:1000,height:600,start,end:start+500,row:12,scroll:true});
 let g=at(0);
 assert.equal(g.scale,0.6);assert.equal(g.offset,0);assert.deepEqual(g.box,{top:0,height:300});
 g=at(19500);
 assert.equal(g.offset,12000-600,'at the end the map shows its last rows');assert.equal(g.box.top+g.box.height,600,'and the box is at the bottom');
 g=at(9750);
 assert.equal(g.box.top,150,'half way: the box is half way down the map');assert.equal(g.offset,(12000-600)/2);
 // The text under the box is the text on screen: its first row maps to the box top.
 for(const start of [0,1234,9750,15000,19500]){const v=at(start);assert(Math.abs(start*v.scale-v.offset-v.box.top)<1e-6,'the view top is at the box top for '+start);}
 // A click centres the view on the text under the pointer.
 g=at(9750);assert.equal(map.targetAt(g,g.box.top+g.box.height/2,g.box.height/2),9750);
 assert.equal(map.targetAt(g,0,0),g.offset/g.scale,'the map top is the text at the map offset');
 // Dragging the box: it follows the pointer over the whole map.
 assert.equal(map.dragTarget(g,9750,(600-300)/2),19500,'half the free height is half the document');
 assert.equal(map.dragTarget(g,9750,-1e4),0);assert.equal(map.dragTarget(g,9750,1e4),19500);
 // A document that fits keeps the blocks' geometry with the text row height.
 g=map.geometry({total:2000,rows:100,height:1600,start:0,end:500,row:12,scroll:true});
 assert.equal(g.used,1200);assert.equal(g.offset,0);assert.deepEqual(g.box,{top:0,height:300});
 assert.equal(map.dragTarget(g,0,60),100,'without scrolling the box moves at the map scale');
});

test('width: the default, the clamps and saved values',()=>{
 assert.deepEqual(map.widthLimits({available:1100}),{min:96,max:550});
 assert.deepEqual(map.widthLimits({available:4000}),{min:96,max:map.WIDTH_MAX},'a pixel cap on wide windows');
 assert.deepEqual(map.widthLimits({narrow:true,available:630}),{min:64,max:315},'a 640 px window: half of the space beside the document');
 assert.deepEqual(map.widthLimits({available:120}),{min:96,max:96},'never below the default');
 for(const value of [64,96,300,640])assert.equal(map.storedWidth(value),value);
 for(const value of [0,63,641,96.5,'200',null,undefined,NaN,-1])assert.equal(map.storedWidth(value),0,String(value));
 const limits=map.widthLimits({available:800});
 assert.equal(map.shownWidth(0,limits),96,'nothing saved: the default');
 assert.equal(map.shownWidth(64,limits),96,'a narrow window\'s width is at least the default here');
 assert.equal(map.shownWidth(500,limits),400,'a saved width larger than this window allows');
 assert.equal(map.shownWidth(250,limits),250);
});

test('text mode starts where the scaled font is legible, with a cap',()=>{
 // A 13 px font with 7.8 px characters over 80 columns (624 px of text).
 const zoom=width=>map.textZoom({width,columns:80,charWidth:7.8,fontSize:13});
 assert.equal(zoom(96),null,'the default width draws blocks');
 assert.equal(zoom(287),null,'just under 6 px');
 assert.deepEqual(zoom(288),{font:6,scale:6/13},'6 px: text');
 assert.equal(zoom(330).font,6.75,'quarter pixels');
 assert.equal(zoom(640).font,7.75,'at most 60% of the editor font');
 assert.equal(map.textZoom({width:640,columns:20,charWidth:14.4,fontSize:24}).font,map.TEXT_FONT_MAX,'and at most TEXT_FONT_MAX px');
 assert.equal(map.textZoom({width:600,columns:20,charWidth:6,fontSize:10}).font,6,'a small editor font still gets 6 px');
 // A very wide text area is zoomed as if it had TEXT_REFERENCE columns.
 assert.deepEqual(map.textZoom({width:432,columns:290,charWidth:7.8,fontSize:13}),map.textZoom({width:432,columns:map.TEXT_REFERENCE,charWidth:7.8,fontSize:13}));
 assert.notEqual(map.textZoom({width:432,columns:290,charWidth:7.8,fontSize:13}),null);
 for(const bad of [{width:0,columns:80,charWidth:7.8,fontSize:13},{width:300,columns:0,charWidth:7.8,fontSize:13},{width:300,columns:80,charWidth:0,fontSize:13}])assert.equal(map.textZoom(bad),null);
});

test('the editor\'s syntax colours: kinds by offset from its HTML, nested spans and entities',()=>{
 const text='interface Gi0/1\n description "a<b" & c ! note';
 const html='<span class="nt-ed-keyword">interface</span> Gi0/1\n <span class="nt-ed-property">description</span> <span class="nt-ed-string">&quot;a&lt;b&quot;</span> &amp; c <span class="nt-ed-comment">! <span class="nt-ed-number">note</span></span>';
 const runs=map.parseSyntax(html);
 assert.equal(runs.length,text.length,'every character is counted once, entities as one');
 const kind=map.syntaxCursor(runs),at=word=>kind(text.indexOf(word));
 assert.equal(at('interface'),KEYWORD);assert.equal(at('Gi0'),TEXT);assert.equal(at('description'),PROPERTY);
 assert.equal(at('"a'),STRING);assert.equal(at('<b'),STRING);assert.equal(at('&'),TEXT);assert.equal(at('!'),COMMENT);assert.equal(at('note'),NUMBER,'the innermost span wins');
 assert.equal(kind(text.length+5),TEXT,'past the end: plain text');
 // Operators and punctuation are the normal text colour; unknown classes keep the outer kind.
 const other=map.parseSyntax('<span class="nt-ed-operator">=</span><span class="nt-ed-function">f</span><span class="token x">y</span><span class="nt-ed-type">T</span>');
 assert.deepEqual([...other.kinds.slice(0,other.count)],[TEXT,KEYWORD,TEXT,PROPERTY]);
 // Unfinished markup is refused, not guessed.
 assert.equal(map.parseSyntax('<span class="nt-ed-string"'),null);
 // Read in slices: the result is the same as in one go.
 const big=Array.from({length:4000},(_,i)=>'<span class="nt-ed-keyword">k'+i+'</span> &lt;x&gt;\n').join('');
 const reader=map.syntaxReader(big);let steps=0;while(!reader.step(performance.now()-1))steps++;
 assert(steps>3,'a deadline in the past reads a few hundred pieces at a time ('+steps+' steps)');
 assert.deepEqual(reader.result(),map.parseSyntax(big));
 const huge='<span class="nt-ed-number">1</span>'.repeat(200000),started=performance.now(),parsed=map.parseSyntax(huge);
 assert.equal(parsed.length,200000);assert.equal(parsed.count,1,'equal neighbours share one run');
 assert(performance.now()-started<250,'200,000 spans read quickly: '+(performance.now()-started).toFixed(1)+' ms');
});

test('a text row: tabs, control characters, colour runs and the column bound',()=>{
 const text='\tif x {\r\n  y\u0001z';
 const kinds=map.parseSyntax('\t<span class="nt-ed-keyword">if</span> x {\r\n  y\u0001z');
 const row=map.rowCells(text,0,text.indexOf('\n'),{tab:4,kindAt:map.syntaxCursor(kinds)});
 assert.equal(row.text,'    if x {','a tab to the next stop; a carriage return is nothing');
 assert.deepEqual(row.cells,[[4,'if',KEYWORD],[6,' x {',TEXT]]);
 const second=map.rowCells(text,text.indexOf('\n')+1,text.length,{});
 assert.equal(second.text,'  y z','another control character is a blank');
 assert.deepEqual(second.cells,[[0,'  y',TEXT],[4,'z',TEXT]]);
 // Never more than the columns asked for, and never more than TEXT_COLUMNS.
 const long='abcdefghij'.repeat(100000);
 assert.equal(map.rowCells(long,0,long.length,{columns:50}).text,long.slice(0,50));
 const started=performance.now(),wide=map.rowCells(long,0,long.length,{columns:1e6});
 assert.equal(wide.text.length,map.TEXT_COLUMNS);assert(performance.now()-started<20,'a 1 MB line costs nothing');
 assert.equal(map.rowCells('\t\t\tx',0,4,{tab:8,columns:12}).text,' '.repeat(12),'tabs stop at the bound');
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

test('blocks take their colours from the editor only, and never draw a row under one pixel',()=>{
 const map=require('../web/assets/editor-document-map.js'),{TEXT,NUMBER}=map.kinds;
 // Without the editor's colours every word is normal text, whatever it holds.
 assert.deepEqual(map.kindRuns('05:05:49.185: source \'Start\' 42',0,31),[[0,13,TEXT],[14,6,TEXT],[21,7,TEXT],[29,2,TEXT]]);
 // With them, runs follow the editor's kinds; blanks and tabs split words.
 assert.deepEqual(map.kindRuns('ab 12\tx',0,7,{kindAt:i=>i>=3&&i<5?NUMBER:TEXT}),[[0,2,TEXT],[3,2,NUMBER],[8,1,TEXT]]);
 assert.deepEqual(map.kindRuns('zz ab',3,5,{kindAt:i=>i===13?NUMBER:TEXT,base:10}),[[0,1,NUMBER],[1,1,TEXT]]);
 // 100 rows in a 600 px map: 3 px each. 400 rows: all of them, 1.5 px each.
 let g=map.blockGeometry({total:2000,rows:100,height:600,start:0,end:500});
 assert.equal(g.used,300);assert.equal(g.offset,0);
 g=map.blockGeometry({total:8000,rows:400,height:600,start:0,end:500});
 assert.equal(g.used,600);assert.equal(g.offset,0);assert.equal(g.scale*20,1.5);
 // 5,000 rows: one pixel each, the map scrolls, the box a thumb from top to bottom.
 g=map.blockGeometry({total:100000,rows:5000,height:600,start:0,end:500});
 assert.equal(g.scale*20,1);assert.equal(g.used,600);assert.equal(g.offset,0);assert.equal(g.box.top,0);
 g=map.blockGeometry({total:100000,rows:5000,height:600,start:99500,end:100000});
 assert.equal(g.offset,4400);assert.equal(Math.round(g.box.top+g.box.height),600);
});
