'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const section=html.slice(html.indexOf('function dnsRecordMeta('),html.indexOf('function eventValues('));
function render(record){const scope={row:record,esc:value=>String(value).replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]))};vm.runInNewContext(section+'\nresult=dnsRecordMeta(row);',scope);return scope.result;}
test('DNS result rows show MX preferences without occupying a full details panel',()=>{
 const markup=render({type:'MX',details:{preference:'10',mailExchanger:'d367636a.ess.barracudanetworks.com.'}});
 assert.match(markup,/Priority 10/);assert.match(markup,/Mail server/);assert.match(markup,/barracudanetworks/);
});
test('DNS result rows expose SOA serial and timing values',()=>{
 const markup=render({type:'SOA',details:{serial:'1',refreshSeconds:'7200',retrySeconds:'900',expireSeconds:'1209600'}});
 assert.match(markup,/Serial 1/);assert.match(markup,/Refresh/);assert.match(markup,/Expire/);
});
test('DNS value metadata escapes untrusted names and omits empty fields',()=>{
 assert.equal(render({type:'A',details:null}),'<span class="muted">-</span>');
 const markup=render({type:'MX',details:{preference:'10',mailExchanger:'<img src=x onerror=alert(1)>'}});
 assert.doesNotMatch(markup,/<img/);assert.match(markup,/&lt;img/);
});
