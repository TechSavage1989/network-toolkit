'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(path.join(__dirname,'../web/assets/terminal-extras.js'),'utf8');
function harness(){
 const sent=[],window={NetworkToolkitTerminalInteraction:{preferences:()=>({autoPager:true,pagerKey:'auto'})}},timers=new Map();let timer=0;
 const c={window,TextDecoder,TextEncoder,document:{querySelector:()=>null},terminalEntryAcceptsInput:e=>e.info.state==='connected',enqueueTerminalInput:(e,data,binary,origin)=>{sent.push({id:e.info.id,data,origin});return true;},setTimeout:fn=>{timers.set(++timer,fn);return timer;},clearTimeout:id=>timers.delete(id)};
 vm.runInNewContext(source,c);const api=window.NetworkToolkitTerminalExtras,entry={info:{id:'one',state:'connected'}};
 return {api,entry,sent,window,feed:text=>api.observeOutput(entry,text),tick(){const run=[...timers.values()];timers.clear();for(const fn of run)fn();}};
}
const aruba='-- MORE --, next page: Space, next line: Enter, quit: q';
for(const [vendor,prompt,key] of [
 ['supplied Aruba CX screenshot',aruba,'space'],
 ['ArubaOS-Switch / ProCurve','-- MORE --, next page: Space, next line: Enter, quit: Control-C','space'],
 ['Cisco IOS','--More--','space'],['Cisco ASA','<--- More --->','space'],
 ['Junos','---(more 42%)---','space'],['dash-delimited More','---- More ----','space'],
 ['legacy More label','More:','space'],['explicit next-line Enter','Press Enter for more','enter'],
 ['explicit next-line Return','Press Return for next line','enter'],
 ['space continuation with quit',"[Press space to continue, 'q' to quit.]",'space']
])test(vendor+' complete prompt and every possible byte split',()=>{
 const {api}=harness(),wire=new TextEncoder().encode('\x1b[7m'+prompt+'\x1b[0m');
 assert.equal(api.pagerPrompt(prompt)?.key,key);
 for(let split=0;split<=wire.length;split++){const p=api.createPagerRecognizer();p.feed(wire.slice(0,split));p.feed(wire.slice(split));assert.equal(p.current()?.key,key,'split '+split);}
});
test('ordinary text, confirmation, action and substring traps never advance',()=>{
 for(const text of ['there is more output','show more','more','More information:','Configuration description --More--','Are you sure? [yes/no]','Delete startup configuration? [confirm]','Press Enter to continue','Press space to continue','Press Enter to confirm','--More-- [yes/no]','-- MORE --, next page: Space, next line: Enter, quit: q [y/n]']){
  const h=harness();h.feed(text);h.tick();assert.equal(h.sent.length,0,text);
 }
});
test('a fragmented extended suffix must settle before the automatic key',()=>{
 const h=harness();h.feed('-- MORE --');h.feed(', next page: Space, next line: Enter, quit: ');h.tick();assert.equal(h.sent.length,0);h.feed('q');h.tick();assert.deepEqual(h.sent,[{id:'one',data:' ',origin:'pager'}]);
 const c=harness();c.feed('--More--');c.feed(' [yes/no]');c.tick();assert.equal(c.sent.length,0);
});
test('incremental OSC, control strings and broken CSI never inject pager text',()=>{
 const {api}=harness();for(const prefix of ['\x1b]0;','\x1bP','\x9d']){
  const p=api.createPagerRecognizer();for(const c of prefix+aruba)p.feed(c);assert.equal(p.current(),null);p.feed('\x1b');p.feed('\\');assert.equal(p.current(),null);p.feed(aruba);assert.equal(p.current()?.key,'space');
 }
 const p=api.createPagerRecognizer();p.feed('\x1b['+'1'.repeat(200)+aruba);assert.equal(p.current(),null);p.feed('\r\n'+aruba);assert.equal(p.current()?.key,'space');
});
test('CR overwrite, ANSI erase and backspace split across packets keep visible prompt state',()=>{
 const {api}=harness(),p=api.createPagerRecognizer();p.feed('stale status'.padEnd(90));p.feed('\r\x1b[');p.feed('2K'+aruba);assert.equal(p.current()?.key,'space');p.consume();p.feed('\b'.repeat(aruba.length));assert.equal(p.ready(),false);p.feed(' '.repeat(aruba.length));p.feed('\rSECOND PAGE\r\n');p.feed('--Mo');p.feed('re--');assert.equal(p.ready(),true);
});
test('identical rapid pages advance once per visible episode, not a time throttle',()=>{
 const h=harness();for(let i=0;i<5;i++){h.feed('\r\x1b[2Ksame page contents\r\n'+aruba);h.tick();h.feed('\x1b[0m');h.tick();assert.equal(h.sent.length,i+1);}
});
test('manual paging cancels queued advance and never needs auto pager enabled',()=>{
 const h=harness();h.feed(aruba);h.entry.xterm={buffer:{active:{},normal:{}}};h.entry.xterm.buffer.active=h.entry.xterm.buffer.normal;
 for(const key of [' ','\r','q','\x03','h']){assert.equal(h.api.pagerInput(h.entry,key),true);h.api.noteManualInput(h.entry,key);h.tick();assert.equal(h.sent.length,0);assert.equal(h.entry._ntOutputHistory,undefined);}
 h.feed('\r\x1b[2Kswitch# ');assert.equal(h.api.pagerInput(h.entry,'show version'),false);
});
test('session overrides, disabling, disconnect and macro ownership are rechecked at send time',()=>{
 for(const mutate of [h=>h.entry._ntSessionPrefs={autoPager:false},h=>h.entry.info.state='closed',h=>h.window.NetworkToolkitTerminalMacros={ownsPager:()=>true}]){const h=harness();h.feed(aruba);mutate(h);h.tick();assert.equal(h.sent.length,0);}
 const h=harness();h.entry._ntSessionPrefs={pagerKey:'enter'};h.feed(aruba);h.tick();assert.equal(h.sent[0].data,'\r');
});
test('one bounded line cannot turn a clipped prose suffix into a pager',()=>{
 const {api}=harness(),p=api.createPagerRecognizer();p.feed('x'.repeat(100000)+aruba);assert.equal(p.current(),null);p.feed('\r\n'+aruba);assert.equal(p.current()?.key,'space');
});
test('separate UTF-8 and parser state are isolated per session',()=>{
 const h=harness(),other={info:{id:'two',state:'connected'}};h.feed(new TextEncoder().encode('界 status\r\n--Mo'));h.api.observeOutput(other,'re--');h.tick();assert.equal(h.sent.length,0);h.feed(new TextEncoder().encode('re--'));h.tick();assert.equal(h.sent[0].id,'one');
});
test('non-changing erase-to-end and identical CR redraw do not re-advance the same pager',()=>{
 const h=harness();h.feed('--More--');h.tick();h.feed('\x1b[K');h.tick();h.feed('  ');h.feed('\b\b\x1b[K');h.tick();h.feed('\r--More--');h.tick();assert.equal(h.sent.length,1);
 h.feed('\r\x1b[2KNEXT PAGE\r\n--More--');h.tick();assert.equal(h.sent.length,2);
});
