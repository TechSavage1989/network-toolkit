'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
global.self=global;const {Terminal}=require('../web/assets/xterm.js');
function harness(options,dimensions={},runtime={}){
 const timers=new Map(),decorations=[],refreshes=[],frames=[],painted=new Map();let next=0,clock=0,renders=0;const context={setTimeout:fn=>{timers.set(++next,fn);return next;},clearTimeout:id=>timers.delete(id),performance:{now:()=>clock+=runtime.clockStep||0}};
 vm.createContext(context);
 for(const name of ['terminal-shell-syntax.js','terminal-highlight.js'])vm.runInContext(fs.readFileSync(name==='terminal-highlight.js'&&process.env.TOOLKIT_HIGHLIGHT_SOURCE?process.env.TOOLKIT_HIGHLIGHT_SOURCE:require.resolve('../web/assets/'+name),'utf8'),context);
 const term=new Terminal({cols:80,rows:8,scrollback:100,allowProposedApi:true,...dimensions});Object.defineProperty(term,'element',{value:{getClientRects:()=>[{}]}});
 const nativeDecoration=term.registerDecoration.bind(term),renderRow=runtime.realDecorations?require('./terminal-render-fixture.cjs')(term):null;
 term.registerDecoration=options=>{
  if(runtime.realDecorations){const d=nativeDecoration(options);d.disposed=false;d.onDispose(()=>{d.disposed=true;});decorations.push(d);return d;}
  const element={style:{},setAttribute(){}},d={options,element,disposed:false,onRender:fn=>{fn(element);return {dispose(){}};},dispose(){this.disposed=true;}};decorations.push(d);return d;
 };
 term.refresh=(start,end)=>{refreshes.push({start,end});if(runtime.renderer)frames.push({start,end});};
 function render(start=0,end=term.rows-1){
  renders++;
  for(let row=start;row<=end;row++){
   const y=term.buffer.active.viewportY+row;
   painted.set(row,renderRow?renderRow(y):decorations.filter(d=>!d.disposed&&!d.options.marker.isDisposed&&d.options.marker.line===y).map(d=>({x:d.options.x,width:d.options.width,color:d.options.foregroundColor})));
  }
  term._core._onRender.fire({start,end});
 }
 const controller=context.NTTerminalHighlight.attach(term,options);
 function flush(){
  for(let n=0;(timers.size||frames.length)&&n<512;n++){
   const jobs=[...timers.values()];timers.clear();jobs.forEach(fn=>fn());
   if(frames.length){const ranges=frames.splice(0);render(Math.min(...ranges.map(r=>r.start)),Math.max(...ranges.map(r=>r.end)));}
  }
  assert.equal(timers.size,0,'highlight work did not settle');assert.equal(frames.length,0,'renderer kept requesting frames');
 }
 async function writeParsed(text){await new Promise(resolve=>term.write(text,()=>{if(runtime.renderer)render();resolve();}));await new Promise(resolve=>setTimeout(resolve,20));}
 async function write(text){await writeParsed(text);flush();}
 return {term,controller,decorations,refreshes,painted,render,get renders(){return renders;},flush,write,writeParsed,analyze:context.NTTerminalHighlight.analyze,legend:context.NTTerminalHighlight.legend,dispose(){controller.dispose();term.dispose();}};
}
test('new parsed output repaints cached text without another write or scroll and settles when unchanged',async()=>{
 const h=harness({underlines:false},{},{renderer:true});h.flush();
 await h.writeParsed('    edit "Office tunnel"\r\n        set type tunnel\r\n        set vdom "root"\r\n    next');
 assert(h.decorations.some(d=>!d.disposed&&d.options.marker.line===0),'first parsed rows must not wait for a later timer');
 assert.equal(h.painted.get(0).length,0,'fixture paints the text before parsed-write decorations');
 assert(h.refreshes.some(range=>range.start===0&&range.end>=3),'changed rows must explicitly request repaint');
 h.flush();
 for(let row=0;row<4;row++)assert(h.painted.get(row).some(part=>part.color),'missing rendered color on row '+row);
 const marks=h.decorations.filter(d=>!d.disposed),refreshes=h.refreshes.length,renders=h.renders;
 h.controller.refresh();h.flush();assert.equal(h.refreshes.length,refreshes);assert.equal(h.renders,renders);assert(marks.every(d=>!d.disposed));
 h.controller.setEnabled(false);h.flush();for(let row=0;row<4;row++)assert.equal(h.painted.get(row).length,0,'disabled colors remained cached');
 h.dispose();
});
test('slow frames always advance through every visible configuration row without requiring scroll',async()=>{
 const h=harness({underlines:false},{cols:100,rows:65},{renderer:true,clockStep:7});h.flush();
 const rows=Array.from({length:60},(_,i)=>'set interface-name "Tunnel-'+i+'"');
 await h.write(rows.join('\r\n'));
 assert.equal(h.controller.getState().limited,false);
 for(let row=0;row<rows.length;row++){
  assert(h.controller.tokensAtRow(row).some(token=>token.kind==='field'),'unprocessed row '+row);
  assert(h.painted.get(row).some(part=>part.color),'unpainted row '+row);
 }
 h.dispose();
});
test('queued highlight paints follow viewport changes and trimmed scrollback during continuing output',async()=>{
 const h=harness({underlines:false},{cols:80,rows:6,scrollback:12},{renderer:true});h.flush();
 for(let batch=0;batch<10;batch++){
  await h.writeParsed(Array.from({length:5},(_,i)=>'set mtu '+(1400+batch*5+i)).join('\r\n')+'\r\n');
  if(batch%2===0){h.term.scrollToTop();h.render();}
  else{h.term.scrollToBottom();h.render();}
  h.flush();
  const first=h.term.buffer.active.viewportY;
  for(let row=0;row<h.term.rows;row++){
   const line=h.term.buffer.active.getLine(first+row).translateToString(true);
   if(line.startsWith('set mtu'))assert(h.painted.get(row).some(part=>part.x===0&&part.color),'missing live color '+batch+' row '+row);
  }
 }
 h.dispose();
});
test('existing marks honor ANSI and hidden rewrites immediately beyond a slow frame budget',async()=>{
 const h=harness({underlines:false},{rows:60},{renderer:true,clockStep:7});h.flush();
 await h.write('192.0.2.1\x1b[59;1H192.0.2.59');
 const first=h.decorations.find(d=>!d.disposed&&d.options.marker.line===0),last=h.decorations.find(d=>!d.disposed&&d.options.marker.line===58);
 assert(first&&last);await h.writeParsed('\x1b[59;1H\x1b[31m192.0.2.59\x1b[0m');assert(last.disposed);assert(!first.disposed);
 const remote=h.decorations.find(d=>!d.disposed&&d.options.marker.line===58);assert(remote);assert.equal(remote.options.foregroundColor,undefined);
 await h.writeParsed('\x1b[59;1H\x1b[8m192.0.2.59\x1b[0m');assert(remote.disposed);assert(!h.decorations.some(d=>!d.disposed&&d.options.marker.line===58));
 h.flush();assert.equal(h.painted.get(58).length,0);assert(!first.disposed);h.dispose();
});
test('unchanged semantic colors keep their decoration identity through parsed writes and refreshes',async()=>{
 const h=harness();await h.write('192.0.2.1\r\nGi1/0/1  up\r\n');const marks=h.decorations.filter(d=>!d.disposed);assert(marks.length>=3);for(let i=0;i<12;i++){await h.write('plain output '+i+'\r');h.controller.refresh();h.flush();assert(marks.every(d=>!d.disposed),'an unchanged mark was removed during output or refresh');}assert.equal(h.decorations.length,marks.length);assert.equal(h.controller.tokenAt(0,0).text,'192.0.2.1');h.dispose();
});
test('actual xterm scrollback renderer colors every newly revealed row on the first repaint',async()=>{
 const h=harness({underlines:false},{cols:180,rows:52,scrollback:10000},{renderer:true,realDecorations:true,clockStep:7});
 await h.write(Array.from({length:6000},(_,i)=>'        set snmp-index '+i).join('\r\n'));
 h.term.scrollLines(-22);h.render();
 for(let row=0;row<h.term.rows;row++)assert(h.painted.get(row).some(span=>span.color),'first scroll repaint left row '+row+' plain');
 h.flush();h.dispose();
});
test('real renderer keeps scroll destinations colored through rapid upward/downward jumps without timers',async()=>{
 const h=harness({underlines:false},{cols:120,rows:60,scrollback:2000},{renderer:true,realDecorations:true});
 await h.write(Array.from({length:1600},(_,i)=>'set interface "port'+i+'"').join('\r\n'));
 for(const destination of [1400,1378,1390,0,700,20,1540,100,1200]){
  const previous=h.decorations.filter(d=>!d.disposed);h.term.scrollToLine(destination);h.render();
  const first=h.term.buffer.active.viewportY,last=first+h.term.rows-1;
  for(let row=0;row<h.term.rows;row++)assert(h.painted.get(row).some(span=>span.color),'uncolored first repaint at '+destination+' row '+row);
  for(const mark of previous)if(!mark.options.marker.isDisposed&&mark.options.marker.line>=first&&mark.options.marker.line<=last)assert(!mark.disposed,'overlapping marks were replaced');
 }
 h.flush();h.dispose();
});
test('bidirectional scroll pruning cannot corrupt the native decoration index or leave a plain block',async t=>{
 const h=harness({underlines:false},{cols:120,rows:52,scrollback:500},{renderer:true,realDecorations:true});
 t.after(()=>h.dispose());
 await h.write(Array.from({length:300},(_,i)=>'        set snmp-index '+i).join('\r\n'));
 const plain=[],invalid=[];
 // Alternating overlaps put retained/new records in a different insertion
 // order than their buffer rows. Do not drain timers between viewport moves:
 // the bundled index must handle lazy deletions while marker keys change.
 for(const destination of [130,100,40,20,60,40,0]){
  h.term.scrollToLine(destination);h.render();
  for(let row=0;row<h.term.rows;row++)if(!h.painted.get(row).some(span=>span.color==='#f0a3d0'))plain.push({destination,row});
  const index=[...h.term._core._decorationService.decorations];
  if(index.some(mark=>mark.disposed||mark.marker.isDisposed)||index.some((mark,i)=>i&&mark.marker.line<index[i-1].marker.line))invalid.push(destination);
 }
 assert.deepEqual(plain,[],'scrolling left recognized configuration rows without semantic foreground styles');
 assert.deepEqual(invalid,[],'consumed native decoration index retained disposed or unsorted entries');
 h.flush();
});
test('neutral ANSI formatting before a pager reset keeps semantic colors throughout scrollback',async()=>{
 const h=harness({underlines:false},{cols:120,rows:52,scrollback:500},{renderer:true,realDecorations:true});
 const rows=Array.from({length:180},(_,i)=>(i===0?'\x1b[37m':i===130?'\x1b[0m':'')+'        set snmp-index '+i);
 await h.write(rows.join('\r\n'));
 for(const destination of [105,80,0,120]){
  h.term.scrollToLine(destination);h.render();h.flush();
  for(let row=0;row<h.term.rows;row++){
   const spans=h.painted.get(row);
   assert(spans.some(span=>span.color==='#f0a3d0'),'neutral foreground hid the command color at '+destination+' row '+row);
   assert(spans.some(span=>span.color==='#bddf86'),'neutral foreground hid the number color at '+destination+' row '+row);
  }
 }
 assert.equal(h.term.buffer.normal.getLine(0).getCell(8).getFgColor(),7,'display colors must not rewrite remote attributes');h.dispose();
});
test('neutral text formats share semantic colors while distinct device formatting remains authoritative',async()=>{
 const theme={foreground:'#abcdee',black:'#171717',white:'#c8d4e8',brightBlack:'#808080',brightWhite:'#f1f6ff'};
 const formats=['30','37','90','97','38;5;244','38;5;59','38;2;180;180;180','38;2;171;205;238','31','38;2;255;40;40','41','7','8'];
 const h=harness({underlines:false},{cols:80,rows:16,theme},{renderer:true,realDecorations:true});
 await h.write(formats.map(code=>'\x1b['+code+'mset mtu 1500\x1b[0m').join('\r\n'));
 for(let row=0;row<formats.length;row++){
  const command=h.decorations.find(d=>!d.disposed&&d.options.marker.line===row&&d.options.x===0);
  if(row<8)assert.equal(command?.options.foregroundColor,'#f0a3d0','neutral format '+formats[row]);
  else assert.equal(command?.options.foregroundColor,undefined,'preserve device format '+formats[row]);
 }
 h.term.options.theme={...theme,white:'#00ff00'};h.controller.refresh();h.flush();
 assert.equal(h.decorations.find(d=>!d.disposed&&d.options.marker.line===1&&d.options.x===0)?.options.foregroundColor,undefined,'custom chromatic white slot must be preserved');
 assert.equal(h.term.buffer.normal.getLine(7).getCell(0).getFgColor(),0xabcdee);h.dispose();
});
test('explicit console-colored backgrounds do not hide syntax colors while contrasting backgrounds are preserved',async()=>{
 const theme={foreground:'#cccccc',background:'#141414',black:'#000000',red:'#cc0000',white:'#cccccc'};
 const codes=['37;40','37;48;2;20;20;20','37;48;5;233','37;41','37;48;2;120;20;20','37;40;7','37;40;8'];
 const h=harness({underlines:false},{cols:80,rows:10,theme},{renderer:true,realDecorations:true});
 await h.write(codes.map(code=>'\x1b['+code+'mset mtu 1500\x1b[0m').join('\r\n'));
 for(let row=0;row<codes.length;row++){
  const command=h.decorations.find(d=>!d.disposed&&d.options.marker.line===row&&d.options.x===0);
  assert.equal(command?.options.foregroundColor,row<3?'#f0a3d0':undefined,'background format '+codes[row]);
 }
 assert.equal(h.term.buffer.normal.getLine(0).getCell(0).isBgDefault(),false);h.dispose();
});
test('output-driven scrolling and paused scrollback trimming do not decorate on each incoming newline',async()=>{
 const h=harness({underlines:false},{cols:80,rows:8,scrollback:20},{renderer:true,realDecorations:true});
 await h.write(Array.from({length:35},(_,i)=>'set mtu '+i).join('\r\n'));
 let last=h.decorations.length,sampling=false;const eager=[];
 const scroll=h.term.onScroll(()=>{if(sampling&&h.decorations.length!==last)eager.push(h.decorations.length-last);last=h.decorations.length;});
 const parsed=h.term.onWriteParsed(()=>{last=h.decorations.length;});
 sampling=true;await h.write('\r\n'+Array.from({length:100},(_,i)=>'set mtu '+i).join('\r\n'));sampling=false;
 h.term.scrollToLine(15);h.render();h.flush();last=h.decorations.length;
 sampling=true;await h.write('\r\n'+Array.from({length:10},(_,i)=>'set mtu '+i).join('\r\n'));sampling=false;
 assert.deepEqual(eager,[]);scroll.dispose();parsed.dispose();h.dispose();
});
test('ANSI changes, hidden text and erased output reconcile marks without clearing unaffected rows',async()=>{
 const h=harness();await h.write('192.0.2.1\r\n192.0.2.2');const first=h.decorations.find(d=>d.options.marker.line===0),second=h.decorations.find(d=>d.options.marker.line===1);assert(first.options.foregroundColor);await h.write('\r\x1b[31m192.0.2.2\x1b[0m');assert.equal(first.disposed,false);assert.equal(second.disposed,true);const colored=h.decorations.find(d=>!d.disposed&&d.options.marker.line===1);assert.equal(colored.options.foregroundColor,undefined);await h.write('\r\x1b[8m192.0.2.2\x1b[0m');assert.equal(colored.disposed,true);assert.equal(h.controller.tokensAtRow(1).length,0);assert.equal(first.disposed,false);await h.write('\x1b[1;1H\x1b[2K');assert.equal(first.disposed,true);h.dispose();
});
test('underlines can be disabled without replacing colors, decorations or device formatting',async()=>{
 const h=harness();await h.write('192.0.2.1\r\nGi1/0/1  up\r\n\x1b[4m192.0.2.99\x1b[0m');const marks=h.decorations.filter(d=>!d.disposed),colors=marks.map(d=>d.options.foregroundColor);assert(marks.every(d=>d.element.style.borderBottom!=='none'));assert(h.term.buffer.normal.getLine(2).getCell(0).isUnderline());
 h.controller.setAppearance({theme:'dark',underlines:false});h.flush();assert(marks.every(d=>!d.disposed&&d.element.style.borderBottom==='none'));assert.deepEqual(marks.map(d=>d.options.foregroundColor),colors);assert(h.term.buffer.normal.getLine(2).getCell(0).isUnderline());assert.equal(h.controller.tokenAt(0,0).text,'192.0.2.1');
 h.controller.setAppearance({theme:'dark',underlines:true});h.flush();assert(marks.every(d=>!d.disposed&&d.element.style.borderBottom!=='none'));h.dispose();
});
test('custom semantic colors recolor default text while preserving explicit device colors',async()=>{
 const h=harness({underlines:false,colors:{address:'#ABCDEF',error:'url(secret-canary)'}});await h.write('192.0.2.1\r\n\x1b[31m192.0.2.2\x1b[0m');const first=h.decorations.find(d=>!d.disposed&&d.options.marker.line===0),remote=h.decorations.find(d=>!d.disposed&&d.options.marker.line===1);assert.equal(first.options.foregroundColor,'#abcdef');assert.equal(remote.options.foregroundColor,undefined);assert.equal(first.element.style.borderBottom,'none');
 h.controller.setTheme('light');h.flush();const current=h.decorations.find(d=>!d.disposed&&d.options.marker.line===0);assert.equal(current.options.foregroundColor,'#abcdef');assert.equal(current.element.style.borderBottom,'none');assert.equal(h.term.buffer.normal.getLine(0).translateToString(true),'192.0.2.1');assert.equal(h.term.buffer.normal.getLine(1).getCell(0).getFgColor(),1);h.dispose();
});
test('FortiOS interface table colors remain consistent across single gaps, tabs and compact address/state columns',()=>{
 const h=harness(),rows=[
  'SiteA-port1 static 0.0.0.0 0.0.0.0 192.0.2.1 255.255.255.255 up disable tunnel disable',
  'SiteB-port17  static  0.0.0.0  0.0.0.0  0.0.0.0  0.0.0.0  down  disable  tunnel  disable',
  'Remote-Access\tstatic\t0.0.0.0\t0.0.0.0\t192.0.2.2\t255.255.255.255\tup\tdisable\ttunnel\tdisable',
  'VPN-17 static 0.0.0.0 0.0.0.0down disable tunnel disable',
  'fortilink static 0.0.0.0 0.0.0.0 192.0.2.1 255.255.255.0 up disable aggregate disable'
 ];
 for(const line of rows){const hits=h.analyze(line),labels=hits.map(hit=>[line.slice(hit.start,hit.end),hit.kind]);assert(labels.some(([text,kind])=>text===line.split(/\s/)[0]&&kind==='interface'),line);for(const word of ['static','disable',line.includes('aggregate')?'aggregate':'tunnel'])assert(labels.some(([text,kind])=>text===word&&kind==='value'),line+' '+word);assert(labels.some(([text,kind])=>/^(up|down)$/.test(text)&&kind===(text==='up'?'success':'error')),line);assert(labels.filter(([,kind])=>kind==='address').length>=2,line);}
 h.dispose();
});
test('configuration syntax roles cover FortiOS, IOS, AOS-CX, ProCurve, Junos, PAN-OS and SonicOS forms',()=>{
 const h=harness(),cases=[
  ['config system interface',[['config','command'],['system','field'],['interface','field']]],
  ['firewall (interface) # set allowaccess ping https ssh',[['set','command'],['allowaccess','field'],['ping','protocol'],['https','protocol'],['ssh','protocol']]],
  ['set unknown-future-setting "named object"',[['set','command'],['unknown-future-setting','field'],['"named object"','value']]],
  ['set mtu 1500',[['set','command'],['mtu','field'],['1500','number']]],
  ['set srcintf "Office uplink"',[['srcintf','field'],['"Office uplink"','interface']]],
  ['interface GigabitEthernet1/0/24',[['interface','field'],['GigabitEthernet1/0/24','interface']]],
  ['switchport access vlan 20',[['switchport','field'],['vlan','field'],['20','number']]],
  ['interface 1/1/25',[['interface','field'],['1/1/25','interface']]],
  ['vlan trunk allowed 10,20,30',[['vlan','field'],['allowed','field'],['10,20,30','number']]],
  ['untagged A1-A24',[['untagged','field'],['A1-A24','interface']]],
  ['tagged Trk1',[['tagged','field'],['Trk1','interface']]],
  ['set interfaces ge-0/0/0 unit 20 family inet address 192.0.2.1/24',[['set','command'],['ge-0/0/0','interface'],['unit','field'],['20','number'],['192.0.2.1/24','address']]],
  ['set network interface ethernet ethernet1/9 layer3 ip 192.0.2.2/24',[['set','command'],['ethernet1/9','interface'],['layer3','field'],['192.0.2.2/24','address']]],
  ['address-object ipv4 "Web server" host 192.0.2.3 zone LAN',[['address-object','field'],['ipv4','protocol'],['"Web server"','value'],['192.0.2.3','address'],['zone','field'],['LAN','value']]],
  ['interface X0',[['interface','field'],['X0','interface']]],
  ['router bgp 65001',[['router','field'],['bgp','protocol'],['65001','number']]],
  ['set ip 2001:db8::1/64',[['set','command'],['ip','field'],['2001:db8::1/64','address']]]
 ];
 for(const [line,expected]of cases){const hits=h.analyze(line).map(hit=>[line.slice(hit.start,hit.end),hit.kind]);for(const [text,kind]of expected)assert(hits.some(hit=>hit[0]===text&&hit[1]===kind),JSON.stringify({line,expected:[text,kind],hits}));}
 h.dispose();
});
test('setting choices and quoted descriptions are distinguished without implying failures or false IPs',()=>{
 const h=harness();
 for(const line of ['set status disable','set action deny','set nat enable','set policy-name "ERROR down"','set description "disabled backup interface"','no shutdown','no errors','0 input errors','errors: 0','not down'])assert(!h.analyze(line).some(hit=>hit.kind==='error'),line);
 for(const line of ['! Uplink disabled for maintenance','# no errors in this example','set comments "error down disabled"']){const hits=h.analyze(line);assert(hits.some(hit=>hit.kind==='comment'),line);assert(!hits.some(hit=>['success','error','warning'].includes(hit.kind)),line);}
 for(const line of ['Firmware version 1.2.3.4','object-name 999.2.3.4','https://down.example.invalid/interfaces/port1','a-connection-refused-host'])assert(!h.analyze(line).some(hit=>['address','success','error'].includes(hit.kind)),line);
 const paired='Gi1/0/1 up/down';assert.deepEqual(Array.from(h.analyze(paired).filter(hit=>['success','error'].includes(hit.kind)),hit=>[paired.slice(hit.start,hit.end),hit.kind]),[['up','success'],['down','error']]);h.dispose();
});
test('dense visible configuration remains colored past the former 512-decoration cap without flicker',async()=>{
 const h=harness({underlines:false},{cols:140,rows:90}),rows=Array.from({length:85},(_,i)=>'Site-'+i+' static 192.0.2.1 255.255.255.0 198.51.100.1 255.255.255.255 '+(i%2?'down':'up')+' disable tunnel disable');
 await h.write(rows.join('\r\n'));const marks=h.decorations.filter(mark=>!mark.disposed);assert(marks.length>512);assert.equal(h.controller.getState().limited,false);
 for(let i=0;i<rows.length;i++){const tokens=h.controller.tokensAtRow(i);assert(tokens.some(token=>token.text==='Site-'+i&&token.kind==='interface'),'missing interface row '+i);assert(tokens.some(token=>token.text===(i%2?'down':'up')&&token.kind===(i%2?'error':'success')),'missing state row '+i);}
 h.controller.refresh();h.flush();assert(marks.every(mark=>!mark.disposed));assert(marks.every(mark=>mark.element.style.borderBottom==='none'));h.dispose();
});
test('local shells color prompt syntax using the selected ANSI palette without changing cells or copies',async()=>{
 const selected={foreground:'#e0e0e0',background:'#101010',brightYellow:'#f0a000',brightGreen:'#00c040',green:'#407040',cyan:'#00b0d0',brightBlack:'#708090'};
 for(const language of ['powershell','cmd']){
  const h=harness({language,underlines:true},{cols:120,theme:selected},{realDecorations:true,renderer:true});
  const line=language==='powershell'?'PS C:\\Users\\Operator> Get-ChildItem -Path $env:TEMP | Where-Object { $_.Length -gt 12 }':'C:\\Users\\Operator> echo %PATH% && dir /a';
  await h.write(line);assert.equal(h.term.buffer.active.getLine(0).translateToString(true),line);assert(h.controller.tokensAtRow(0).some(t=>t.kind==='shell-variable'&&t.text===(language==='powershell'?'$env:TEMP':'%PATH%')));
  const variable=h.decorations.find(d=>!d.disposed&&d.options.foregroundColor==='#00c040');assert(variable,'ANSI variable slot was ignored');const stringCopy=h.term.buffer.active.getLine(0).translateToString(true);assert.equal(stringCopy,line);
  assert(h.controller.tokensAtRow(0).every(t=>t.kind.startsWith('shell-')));h.dispose();
 }
});
test('shell-provided colors, hidden output and alternate screens retain their rendering',async()=>{
 const h=harness({language:'powershell',underlines:false},{cols:100},{realDecorations:true,renderer:true});
 await h.write('PS C:\\> \x1b[31mGet-Process\x1b[0m -Name "app"');
 const command=h.controller.tokensAtRow(0).find(t=>t.text==='Get-Process');assert(command);const red=h.term.buffer.active.getLine(0).getCell(8);assert(red.isFgPalette());assert.equal(red.getFgColor(),1);
 const covering=h.decorations.filter(d=>!d.disposed&&d.options.x>=8&&d.options.x<19);assert(covering.length);assert(covering.every(d=>d.options.foregroundColor===undefined),'fallback replaced shell red');
 await h.write('\r\nplain output with Get-Process -Name "app"');assert.equal(h.controller.tokensAtRow(1).length,0,'ordinary output was mistaken for input');
 await h.write('\r\nPS C:\\> \x1b[8mGet-Date\x1b[0m');assert.equal(h.controller.tokensAtRow(2).length,0,'hidden text was decorated');
 await h.write('\x1b[?1049hPS C:\\> Get-Date');assert.equal(h.controller.getState().suspended,'alternate');await h.write('\x1b[?1049l');assert.notEqual(h.controller.getState().suspended,'alternate');h.dispose();
});
test('local syntax follows live palette edits while SSH uses its original device highlights',async()=>{
 const h=harness({language:'cmd',underlines:false},{cols:100,theme:{brightGreen:'#123456'}});await h.write('C:\\> echo %TEMP%');assert(h.decorations.some(d=>!d.disposed&&d.options.foregroundColor==='#123456'));
 h.term.options.theme={brightGreen:'#654321'};h.controller.setAppearance({theme:'dark',language:'cmd',underlines:false});h.flush();assert(h.decorations.some(d=>!d.disposed&&d.options.foregroundColor==='#654321'));assert(!h.decorations.some(d=>!d.disposed&&d.options.foregroundColor==='#123456'));h.dispose();
 const remote=harness({language:'ssh',underlines:false});await remote.write('switch# set mtu 1500');assert(remote.controller.tokensAtRow(0).some(t=>t.kind==='field'));assert(remote.controller.tokensAtRow(0).every(t=>!t.kind.startsWith('shell-')));remote.dispose();
});
test('ConPTY white-on-black console attributes receive syntax colors in the default terminal theme',async()=>{
 const colors={background:'#0b1220',foreground:'#dce6f6',black:'#243149',white:'#c8d4e8',brightWhite:'#f1f6ff',brightGreen:'#8ce2b6',brightYellow:'#ffda91',cyan:'#67cdd1',green:'#6aca9e',brightBlack:'#73829a'};
 for(const language of ['cmd','powershell'])for(const ansi of ['\x1b[37;40m','\x1b[97;40m']){
  const h=harness({language,underlines:false},{cols:110,theme:colors},{realDecorations:true,renderer:true}),line=language==='cmd'?'C:\\> echo %TEMP% && dir /a':'PS C:\\> Get-ChildItem -Path "$env:TEMP"';await h.write(ansi+line);assert(h.decorations.some(d=>!d.disposed&&d.options.foregroundColor),'recognized native syntax remained white');assert.equal(h.term.buffer.active.getLine(0).translateToString(true),line);assert.equal(h.term.buffer.active.getLine(0).getCell(0).getBgColor(),0,'native background was rewritten');h.dispose();
 }
});
test('recorded native Windows cmd and PowerShell output renders syntax and custom role colors without changing buffers',async()=>{
 const recordings=require('./fixtures/windows-local-shells.json');
 for(const language of ['cmd','powershell']){
  const h=harness({language,underlines:false},{cols:120,rows:20},{realDecorations:true,renderer:true});await h.write(Buffer.from(recordings.captures[language],'base64').toString('utf8'));
  const rows=Array.from({length:h.term.buffer.active.length},(_,i)=>h.term.buffer.active.getLine(i).translateToString(true)),promptRow=rows.findIndex(line=>line.includes(language==='cmd'?'%TEMP%':'$env:TEMP'));assert(promptRow>=0,language);const tokens=h.controller.tokensAtRow(promptRow);assert(tokens.some(t=>t.kind==='shell-variable'),language+' native variable was not recognized');
  h.controller.setAppearance({language,theme:'dark',underlines:false,shellColors:{command:'#bb44ee',keyword:'#11aaee',variable:'#00ab91'}});h.flush();assert(h.decorations.some(d=>!d.disposed&&d.options.foregroundColor==='#00ab91'),language+' custom variable color was not applied');assert.deepEqual(Array.from({length:h.term.buffer.active.length},(_,i)=>h.term.buffer.active.getLine(i).translateToString(true)),rows);h.dispose();
 }
});
test('local command output colors ipconfig fields, addresses and statuses without altering ANSI or buffer cells',async()=>{
 for(const language of ['cmd','powershell']){
  const h=harness({language,underlines:false},{cols:120,rows:12},{realDecorations:true,renderer:true});
  const text='Windows IP Configuration\r\nEthernet adapter Ethernet 2:\r\n   IPv4 Address. . . . . . . . . . : 192.0.2.35(Preferred)\r\n   DNS Servers . . . . . . . . . . : 192.0.2.53\r\n   Media State . . . . . . . . . . : Media disconnected\r\n\x1b[31mERROR: native red\x1b[0m\r\nordinary unstructured output';await h.write(text);
  for(const [row,kind]of [[0,'outputHeading'],[1,'outputHeading'],[2,'outputField'],[2,'outputAddress'],[3,'outputAddress'],[4,'outputWarning']])assert(h.controller.tokensAtRow(row).some(t=>t.kind==='shell-'+kind),language+' missing '+kind);
  assert(h.decorations.some(d=>!d.disposed&&d.options.marker.line===2&&d.options.foregroundColor));assert.equal(h.controller.tokensAtRow(6).length,0);
  const rows=Array.from({length:h.term.buffer.normal.length},(_,i)=>h.term.buffer.normal.getLine(i).translateToString(false));assert(h.term.buffer.normal.getLine(5).getCell(0).isFgPalette());assert.equal(h.term.buffer.normal.getLine(5).getCell(0).getFgColor(),1);
  h.controller.setAppearance({language,theme:'light',shellColors:{outputAddress:'#aa33dd'}});h.flush();assert(h.decorations.some(d=>!d.disposed&&d.options.foregroundColor==='#aa33dd'));assert.deepEqual(Array.from({length:h.term.buffer.normal.length},(_,i)=>h.term.buffer.normal.getLine(i).translateToString(false)),rows);h.dispose();
 }
});
