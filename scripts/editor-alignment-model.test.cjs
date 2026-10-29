'use strict';
const test = require('node:test'), assert = require('node:assert/strict');
const model = require('../web/assets/editor-alignment-model.js');
const compare = require('../web/assets/editor-compare.js').compare;
const fragments = (text, ranges) => ranges.map(({from,to}) => text.slice(from,to));
const pairBoundary = (text, offset) => offset > 0 && offset < text.length && /[\uD800-\uDBFF]/.test(text[offset-1]) && /[\uDC00-\uDFFF]/.test(text[offset]);
function validRanges(text, ranges) {
 let previous = -1;
 for (const {from,to} of ranges) {
  assert(Number.isInteger(from) && Number.isInteger(to) && from >= 0 && from < to && to <= text.length);
  assert(from > previous); assert(!pairBoundary(text,from)); assert(!pairBoundary(text,to)); previous = to;
 }
}
function unchanged(text, ranges) {
 let at = 0, out = '';
 for (const {from,to} of ranges) { out += text.slice(at,from); at = to; }
 return out + text.slice(at);
}
function geometry(left, right, leftHeights, rightHeights) { return model.alignedGeometry(compare(left,right), leftHeights,rightHeights); }

test('intraline equal/empty/insert/delete ranges retain source offsets',()=>{
 for (const text of ['', 'unchanged', '😀 e\u0301']) assert.deepEqual(model.intralineRanges(text,text),{left:[],right:[]});
 assert.deepEqual(model.intralineRanges('','abc'),{left:[],right:[{from:0,to:3}]});
 assert.deepEqual(model.intralineRanges('abc',''),{left:[{from:0,to:3}],right:[]});
 assert.deepEqual(model.intralineRanges('ip address','ip new address'),{left:[],right:[{from:3,to:7}]});
 assert.deepEqual(model.intralineRanges('ip new address','ip address'),{left:[{from:3,to:7}],right:[]});
});
test('bounded LCS marks separate changed tokens without marking equal middle',()=>{
 const left='vlan 12 name old enabled',right='vlan 34 name new enabled',r=model.intralineRanges(left,right);
 assert.deepEqual(fragments(left,r.left),['12','old']);assert.deepEqual(fragments(right,r.right),['34','new']);
 assert.equal(unchanged(left,r.left),unchanged(right,r.right));
});
test('shared conservative work budget is exhausted deterministically and never negative',()=>{
 const budget={remaining:9};assert.deepEqual(model.intralineRanges('abc','xyz',budget),{left:[{from:0,to:3}],right:[{from:0,to:3}]});assert.equal(budget.remaining,0);
 assert.deepEqual(model.intralineRanges('prefix a M c suffix','prefix x M y suffix',budget),{left:[{from:7,to:12}],right:[{from:7,to:12}]});assert.equal(budget.remaining,0);
 const cap={remaining:Number.MAX_SAFE_INTEGER};model.intralineRanges('abc','xyz',cap);assert.equal(cap.remaining,model.LIMITS.totalCells-9);
});
test('per-line cell bound falls back before allocating a large matrix',()=>{
 const budget={remaining:1000000},left='p'+ 'a'.repeat(201)+'s',right='p'+'b'.repeat(200)+'s';
 assert.deepEqual(model.intralineRanges(left,right,budget),{left:[{from:1,to:202}],right:[{from:1,to:201}]});assert.equal(budget.remaining,1000000);
 const edge={remaining:40000};model.intralineRanges('a'.repeat(200),'b'.repeat(200),edge);assert.equal(edge.remaining,0);
});
test('long common context scans linearly and small changed middle remains precise',()=>{
 const prefix='p'.repeat(500000),suffix='s'.repeat(500000),budget={remaining:2};
 assert.deepEqual(model.intralineRanges(prefix+'😀'+suffix,prefix+'😃'+suffix,budget),{left:[{from:500000,to:500002}],right:[{from:500000,to:500002}]});assert.equal(budget.remaining,2);
});
test('all Unicode ranges stay on code-point boundaries, including fallback',()=>{
 const pairs=[['x😀tail','x😃tail'],['x😀tail','x🐀tail'],['😀foo😀bar','😃foo😃bar'],['a\uD83D','a\uDE00'],['e\u0301X','e\u0301Y'],['a😀b','aXb'],['😀',''],['','🧑‍💻']];
 for(const [a,b] of pairs)for(const remaining of [0,1000000]){
  const r=model.intralineRanges(a,b,{remaining});validRanges(a,r.left);validRanges(b,r.right);assert.equal(unchanged(a,r.left),unchanged(b,r.right));
 }
});
test('intraline handles adversarial repetitions within one shared budget',()=>{
 const budget={remaining:1000000};for(let n=0;n<50;n++){const a='ab'.repeat(100),b='ba'.repeat(100),r=model.intralineRanges(a,b,budget);validRanges(a,r.left);validRanges(b,r.right);assert.equal(unchanged(a,r.left),unchanged(b,r.right));}assert.equal(budget.remaining,0);
});
test('deterministic mixed-Unicode edits preserve exactly the same unmarked source',()=>{
 let seed=113;const alphabet=['a','b',' ','😀','😃','🐀','e','\u0301','\uD83D','\uDE00'];
 const next=()=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return seed;};
 for(let n=0;n<1000;n++){
  const make=()=>Array.from({length:next()%25},()=>alphabet[next()%alphabet.length]).join('');
  const a=make(),b=make(),r=model.intralineRanges(a,b,{remaining:n%2?1000000:0});
  validRanges(a,r.left);validRanges(b,r.right);assert.equal(unchanged(a,r.left),unchanged(b,r.right));
 }
});
test('invalid intraline arguments fail closed',()=>{
 for(const budget of [null,{}, {remaining:NaN},{remaining:Infinity},{remaining:-1},{remaining:1.5}])assert.throws(()=>model.intralineRanges('a','b',budget),TypeError);
 assert.throws(()=>model.intralineRanges(null,'b'),TypeError);
});

test('equal lines align unequal wrapping by presentation padding only',()=>{
 const r=geometry('same\nlast','same\nlast',[20,40],[60,20]);
 assert.deepEqual(r,{rowOffsets:[0,60,100],sourceOffsets:{left:[0,60,100],right:[0,60,100]},paddingAfter:{left:[40,0],right:[0,20]},leading:{left:0,right:0}});
});
test('removed block pauses right source indices until next matching line',()=>{
 const left='anchor\nremove1\nremove2\nend',right='anchor\nend',r=geometry(left,right,[20,40,20,20],[20,20]);
 assert.deepEqual(r.rowOffsets,[0,20,60,80,100]);assert.deepEqual(r.sourceOffsets,{left:[0,20,60,80,100],right:[0,80,100]});assert.deepEqual(r.paddingAfter,{left:[0,0,0,0],right:[60,0]});assert.deepEqual(r.leading,{left:0,right:0});
 assert.equal(left,'anchor\nremove1\nremove2\nend');assert.equal(right,'anchor\nend');
});
test('leading and trailing insertions map to zero-source-length spacers',()=>{
 const r=geometry('keep','added\nkeep\ntail',[20],[30,40,50]);
 assert.deepEqual(r,{rowOffsets:[0,30,70,120],sourceOffsets:{left:[30,120],right:[0,30,70,120]},paddingAfter:{left:[70],right:[0,0,0]},leading:{left:30,right:0}});
});
test('paired replacement plus remaining removed row uses one common row model',()=>{
 const r=geometry('start\nold1\nold2\nend','start\nnew\nend',[20,20,40,20],[20,60,20]);
 assert.deepEqual(r.rowOffsets,[0,20,80,120,140]);assert.deepEqual(r.sourceOffsets.right,[0,20,120,140]);assert.deepEqual(r.paddingAfter.left,[0,40,0,0]);assert.deepEqual(r.paddingAfter.right,[0,40,0]);
});
test('empty physical source remains its real empty line and no phantom source lines are added',()=>{
 const r=geometry('','',[20],[20]);assert.deepEqual(r.sourceOffsets,{left:[0,20],right:[0,20]});assert.deepEqual(r.paddingAfter,{left:[0],right:[0]});
 const missing=model.alignedGeometry({rows:[{left:null,right:0},{left:null,right:1}],lineCounts:{left:0,right:2}},[],[10,30]);assert.deepEqual(missing.sourceOffsets.left,[40]);assert.equal(missing.leading.left,40);assert.deepEqual(missing.paddingAfter.left,[]);
 assert.deepEqual(model.alignedGeometry({rows:[]},[],[]),{rowOffsets:[0],sourceOffsets:{left:[0],right:[0]},paddingAfter:{left:[],right:[]},leading:{left:0,right:0}});
});
test('fractional measured heights reconstruct both sides without rounding source rows',()=>{
 const left='a\nx\nz',right='a\nz',heights={left:[20.25,40.5,20.125],right:[30.5,25.75]},r=geometry(left,right,heights.left,heights.right);
 for(const side of ['left','right']){let y=r.leading[side];for(let i=0;i<heights[side].length;i++){assert(Math.abs(y-r.sourceOffsets[side][i])<1e-8);y+=heights[side][i]+r.paddingAfter[side][i];}assert(Math.abs(y-r.rowOffsets.at(-1))<1e-8);}
});
test('duplicate equal lines keep ordered source identities from the existing line engine',()=>{
 const left='same\nsame\nleft\nsame',right='same\nright\nsame',comparison=compare(left,right),r=model.alignedGeometry(comparison,[20,20,20,20],[20,20,20]);
 for(const side of ['left','right'])for(let i=0;i<comparison.rows.length;i++){const index=comparison.rows[i][side];if(index!==null)assert.equal(r.sourceOffsets[side][index],r.rowOffsets[i]);}
});
test('geometry validates missing, duplicate, reordered and out-of-range source rows',()=>{
 const invalid=[{rows:[{left:1,right:0}]},{rows:[{left:0,right:0},{left:0,right:null}]},{rows:[{left:null,right:null}]},{rows:[{left:0}]},{rows:[]},{rows:[{left:-1,right:0}]},{limited:true,rows:[{left:0,right:0}]},{rows:[{left:0,right:0}],lineCounts:{left:2,right:1}}];
 for(const r of invalid)assert.throws(()=>model.alignedGeometry(r,[20],[20]),TypeError);
});
test('geometry rejects nonfinite/zero/negative heights and overflowing totals',()=>{
 for(const value of [NaN,Infinity,-1,0,'20',Number.MAX_SAFE_INTEGER+1])assert.throws(()=>model.alignedGeometry({rows:[{left:0,right:0}]},[value],[20]),TypeError);
 assert.throws(()=>model.alignedGeometry({rows:[{left:0,right:0},{left:1,right:1}]},[Number.MAX_SAFE_INTEGER,20],[20,20]),RangeError);
 assert.throws(()=>model.alignedGeometry({rows:[{left:0,right:0},{left:1,right:1}]},[Number.MAX_SAFE_INTEGER,0.1],[20,0.1]),RangeError);
 assert.throws(()=>model.alignedGeometry({rows:[]},new Array(30001).fill(1),[]),TypeError);
 assert.throws(()=>model.alignedGeometry({rows:new Array(60001).fill({left:null,right:0})},[],[]),TypeError);
});
test('geometry reads frozen inputs without mutation',()=>{
 const result=compare('a\nx\nb','a\nb');result.rows.forEach(Object.freeze);Object.freeze(result.rows);Object.freeze(result.lineCounts);Object.freeze(result);
 const before=JSON.stringify(result);model.alignedGeometry(result,Object.freeze([20,40,20]),Object.freeze([20,20]));assert.equal(JSON.stringify(result),before);
});
