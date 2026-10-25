'use strict';
const {test}=require('node:test');
const assert=require('node:assert/strict');
const M=require('../web/assets/hosts-editor-model.js');

test('hosts parser distinguishes active/disabled mappings from ordinary comments',()=>{
 const d=M.parse('# Example hosts file\r\n127.0.0.1 localhost local\r\n\r\n # 192.0.2.7 switch.example sw # lab\r\n::1 localhost\r\n');
 assert.equal(d.rows.length,3);assert.equal(d.enabled,2);assert.equal(d.disabled,1);assert.equal(d.errors,0);assert.equal(d.warnings,0);
 assert.equal(d.rows[1].line,4);assert.equal(d.rows[1].comment,'lab');assert.deepEqual(d.rows[1].hosts,['switch.example','sw']);
});
for(const ip of ['127.0.0.1','0.0.0.0','255.255.255.255','::1','2001:db8::7','::ffff:192.0.2.1'])test('accepts supported address '+ip,()=>assert.ok(M.address(ip)));
for(const ip of ['256.0.0.1','01.2.3.4','192.0.2.1/24','fe80::1%3','::ffff:192.000.2.1','1::2::3','http://example.test','1.2.3'])test('rejects invalid or non-hosts address '+ip,()=>assert.equal(M.address(ip),null));
test('same-family conflicts and exact duplicates report both affected lines; dual stack is allowed',()=>{
 const d=M.parse('192.0.2.1 Example.test\n2001:db8::1 example.test\n192.0.2.1 EXAMPLE.TEST.\n192.0.2.2 example.test\n');
 assert.equal(d.errors,0);assert.equal(d.warnings,2);assert.deepEqual(d.issues.map(x=>x.code),['duplicate','conflict']);assert.deepEqual([...d.related],[3,1,4]);
});
test('invalid active entries, control characters in comments and oversize files cannot pass validation',()=>{
 const d=M.parse('192.0.2.1\nbad-address host\n192.0.2.2 bad/name\n# comment\0\n');
 assert.equal(d.errors,4);assert.ok(d.issues.find(x=>x.code==='control'&&x.line===4));
 assert.equal(M.parse('#'+'x'.repeat(M.LIMIT)).issues[0].code,'size');
});
test('hostnames use the backend ASCII, label-length and alias rules',()=>{
 for(const value of ['localhost','Host-1.example.test.','_service.local','xn--bcher-kva.test'])assert.ok(M.hostname(value));
 for(const value of ['-bad','bad-','bad..name','https://host','host/name','a'.repeat(64)+'.test','bad name','bücher.test',''])assert.equal(M.hostname(value),false);
});
test('entry replacement changes only its mapping line and preserves unrelated comments and order',()=>{
 const text='# heading\n\n  192.0.2.1 old alias  # old note\n# leave this alone\n192.0.2.9 other\n';
 assert.equal(M.replaceLine(text,3,{address:'2001:db8::2',hosts:['new.test','new'],comment:'new note',enabled:false}),'# heading\n\n  # 2001:db8::2\tnew.test new\t# new note\n# leave this alone\n192.0.2.9 other\n');
 assert.throws(()=>M.replaceLine(text,2,{address:'127.0.0.1',hosts:['x']}),/changed/);
});
test('entry input cannot introduce extra lines through aliases or comments',()=>{
 assert.throws(()=>M.lineText({address:'127.0.0.1',hosts:['good\nbad']}),/hostnames/);
 assert.throws(()=>M.lineText({address:'127.0.0.1',hosts:['good'],comment:'one\ntwo'}),/one line/);
 assert.equal(M.append('# header',{address:'0.0.0.0',hosts:['blocked.test'],enabled:false}),'# header\n# 0.0.0.0\tblocked.test\n');
});
test('enable/disable retains whitespace, aliases and inline comments exactly',()=>{
 const text='# heading\n  192.0.2.1  host alias # note\n';
 const disabled=M.toggle(text,new Set([2]),false);
 assert.equal(disabled,'# heading\n  # 192.0.2.1  host alias # note\n');assert.equal(M.toggle(disabled,new Set([2]),true),text);
 assert.throws(()=>M.toggle('invalid line\n',new Set([1]),true),/invalid/);
});
test('bulk delete removes only selected mappings and keeps standalone comments',()=>{
 assert.equal(M.remove('# header\n192.0.2.1 one\n# note\n192.0.2.2 two\n',new Set([2])),'# header\n# note\n192.0.2.2 two\n');
});
test('format selection leaves all unselected lines byte-for-byte in the normalized draft',()=>{
 assert.equal(M.format('# section\n192.0.2.1   one # comment\n192.0.2.2   two\n',new Set([2])),'# section\n192.0.2.1\tone\t# comment\n192.0.2.2   two\n');
});
test('sorting respects standalone comment and blank-line group boundaries',()=>{
 const source='# first\n192.0.2.2 zebra\n192.0.2.1 alpha\n\n# second\n192.0.2.4 delta\n# 192.0.2.3 beta\n';
 assert.equal(M.sort(source),'# first\n192.0.2.1 alpha\n192.0.2.2 zebra\n\n# second\n# 192.0.2.3 beta\n192.0.2.4 delta\n');
});
test('deduplication retains different comments, alias order and disabled copies',()=>{
 const source='192.0.2.1 one two # same\n192.0.2.1 ONE two # same\n192.0.2.1 one two # different\n192.0.2.1 two one # same\n# 192.0.2.1 one two # same\n';
 const result=M.deduplicate(source);assert.equal(result.count,1);assert.equal(result.text,source.replace('192.0.2.1 ONE two # same\n',''));
});
test('find and replace treat patterns and dollar replacements as literal text',()=>{
 assert.deepEqual(M.matches('one.test oneXtest','one.test'),[{start:0,end:8}]);
 assert.equal(M.replaceAll('one.test ONE.TEST','one.test','$&').text,'$& $&');
 assert.equal(M.replaceAll('a+b [x]','a+b','host').text,'host [x]');
});
test('find/replace keeps real offsets after Unicode comments',()=>{
 const text='# İ and \u{1D400}\nhost.test\n';const at=text.indexOf('host.test');
 assert.deepEqual(M.matches(text,'HOST.TEST'),[{start:at,end:at+9}]);assert.equal(M.replaceAll(text,'HOST.TEST','new.test').text,'# İ and \u{1D400}\nnew.test\n');
});
test('whole-hostname and case-sensitive matching avoid partial aliases',()=>{
 assert.equal(M.matches('host subhost host.test Host','host',false,true).length,2);
 assert.equal(M.matches('Host host','host',true).length,1);assert.deepEqual(M.matches('anything',''),[]);
});
test('preview excludes unchanged leading/trailing lines and retains the changed region',()=>{
 assert.deepEqual(M.diff('# head\nold\n# tail\n','# head\nnew\n# tail\n'),{start:2,removed:['old'],added:['new']});
});
test('large conflict reports remain bounded while all counts and first-line references remain accurate',()=>{
 const text=Array.from({length:12000},(_,i)=>'10.'+Math.floor(i/65536)+'.'+Math.floor(i/256)%256+'.'+i%256+' same.test').join('\n');
 const d=M.parse(text);assert.equal(d.rows.length,12000);assert.equal(d.warnings,11999);assert.equal(d.issues.length,200);assert.ok(d.issues.every(x=>x.otherLine===1));
});
