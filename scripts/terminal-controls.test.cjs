'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test'),{document:makeDocument,Element}=require('./terminal-dom-fixture.cjs');
global.self=global;const {Terminal}=require('../web/assets/xterm.js');const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
function appFunction(name){const tail=html.slice(html.search(new RegExp('^function '+name+'\\(','m')));return tail.slice(0,tail.slice(1).search(/\n(?:async )?function /)+1);}
function harness(options={}){
 let saves=0,fits=0;let entry,clipboard='',inputBlocked=false;const window={Terminal,addEventListener(){},NetworkToolkitTerminalCopy:{suppressesInput:()=>inputBlocked},NetworkToolkitTerminalInteraction:{preferences:()=>({}),suppressesInput:()=>false,copyValue:async text=>{clipboard=text;return true;},noteInput:(e,data)=>window.NetworkToolkitTerminalExtras.noteManualInput(e,data)}};
 const c={window,root:window,TextEncoder,TextDecoder,Date,Math,state:{terminalFontSize:13},saveState(){saves++;},scheduleTerminalFit(){fits++;},TERMINAL_ONLY:options.terminalOnly===true,icon:()=>'',esc:String,setTimeout:()=>0,clearTimeout(){},toast(){},terminalEntryAcceptsInput:e=>!!e&&e.info.state==='connected',terminalLive:e=>e?.info.state==='connected',terminalOwnerCanMutate:()=>true,terminalReconnectAllowed:e=>!!e,terminalSessions:new Map(),currentTerminal:()=>entry,online:true,innerHeight:800,innerWidth:1200};
 c.NetworkToolkitTerminalInteraction=window.NetworkToolkitTerminalInteraction;vm.createContext(c);vm.runInContext(appFunction('terminalToolsHTML'),c);c.document=makeDocument('<div class="terminal-panel"><div id="terminalCommandBar"><div class="terminal-quick-tools"><button id="terminalToolsToggle">Tools</button></div></div>'+c.terminalToolsHTML()+(options.tabs?'<div id="terminalTabs" class="terminal-tabs"><button>Retained tab</button></div>':'')+'<div id="terminalStage"><div id="retainedConsole">Retained console</div></div><div class="terminal-statusbar"></div></div>');vm.runInContext(appFunction('enqueueTerminalInput'),c);vm.runInContext(fs.readFileSync(path.join(__dirname,'../web/assets/terminal-output-archive.js'),'utf8'),c);vm.runInContext(fs.readFileSync(path.join(__dirname,'../web/assets/terminal-extras.js'),'utf8'),c);
 const term=new Terminal({cols:40,rows:6,allowProposedApi:true,scrollback:100});term.focus=()=>{};let selection;term.selectLines=(start,end)=>{selection={start,end};};const scroll=[];const nativeScroll=term.scrollToLine.bind(term);term.scrollToLine=line=>{scroll.push(line);nativeScroll(line);};
 entry={info:{id:'one',state:'connected',type:'ssh'},xterm:term,element:new Element(),disposables:[],inputChunks:[],inputBytes:0};c.terminalSessions.set('one',entry);term.onData(data=>c.enqueueTerminalInput(entry,data));const api=window.NetworkToolkitTerminalExtras;api.sync();
 const input=text=>{term.input(text,true);entry.inputChunks=[];entry.inputBytes=0;};const write=async text=>{const pos=api.beforeOutput(entry);api.observeOutput(entry,text);await new Promise(resolve=>term.write(text,resolve));api.afterOutput(entry,pos);await api.flush(entry);};
 c.openModal=(_title,body)=>{c.document.querySelector('#fixtureModal')?.remove();const modal=new Element();modal.id='fixtureModal';modal.innerHTML=body;c.document.append(modal);};
 return {c,entry,api,input,write,scroll,document:c.document,get saves(){return saves},get fits(){return fits},get clipboard(){return clipboard},get selection(){return selection},blocked:value=>inputBlocked=value,dispose(){for(const d of entry.disposables)d.dispose();term.dispose();}};
}
test('every active session command is in the visible console row, including serial controls',()=>{
 const h=harness(),d=h.document,row=d.querySelector('#terminalActiveActions');assert(row);assert(!row.closest('.hidden'));for(const action of ['clear','disconnect','reconnect','control-c','control-d','control-z','export','serial-dtr','serial-rts','serial-break'])assert(row.querySelector('[data-terminal-action="'+action+'"]'),action+' is directly available');assert.equal(d.querySelector('[data-terminal-action="fit"]'),null);assert.match(row.textContent,/Clear scrollback/);assert.equal(d.querySelector('#terminalSessionPanel [data-terminal-action="clear"]'),null);assert.equal(d.querySelector('#terminalSessionToggle').textContent,'Display');assert(row.querySelector('#terminalLocalEcho'));assert(row.querySelector('#terminalLineEnding'));h.dispose();
});
test('active session commands default to Top and move without replacing controls or console',()=>{
 const h=harness(),d=h.document,row=d.querySelector('#terminalActiveActions'),console=d.querySelector('#retainedConsole'),button=row.querySelector('[data-terminal-action="clear"]'),picker=d.querySelector('#terminalActiveActionsPosition'),stage=d.querySelector('#terminalStage'),bar=d.querySelector('#terminalConsoleBar');
 const previous=node=>node.parentElement.children[node.parentElement.children.indexOf(node)-1];
 assert.equal(previous(stage),row);assert.equal(picker.value,'top');
 picker.value='bottom';d.emit('change',picker);assert.equal(previous(bar),row);assert.equal(h.c.state.terminalActiveActionsPosition,'bottom');assert.equal(h.saves,1);assert.equal(h.fits,1);
 h.api.sync();assert.equal(previous(bar),row);assert.equal(row.querySelector('[data-terminal-action="clear"]'),button);assert.equal(d.querySelector('#retainedConsole'),console);
 h.c.currentTerminal=()=>null;picker.value='top';d.emit('change',picker);assert.equal(previous(stage),row);assert.equal(picker.disabled,false);assert.equal(h.c.state.terminalActiveActionsPosition,'top');
 h.c.state.terminalActiveActionsPosition='invalid';h.api.sync();assert.equal(previous(stage),row);assert.equal(picker.value,'top');h.dispose();
});
test('actual accepted terminal input enables all output actions after parser completion',async()=>{
 const h=harness(),buttons=['output-start','output-end','select-output','copy-output'].map(action=>h.document.querySelector('[data-terminal-extra="'+action+'"]'));assert(buttons.every(b=>b.disabled));await h.write('switch# ');h.input('show version');await h.write('show version');h.input('\r');await h.write('\r\nVERSION ONE\r\nVERSION TWO\r\nswitch# ');assert(buttons.every(b=>!b.disabled));h.document.emit('click',buttons[3]);await Promise.resolve();assert.equal(h.clipboard,'VERSION ONE\nVERSION TWO');h.document.emit('click',buttons[2]);assert.deepEqual(h.selection,{start:1,end:2});h.document.emit('click',buttons[0]);assert.deepEqual(h.selection,{start:1,end:1});h.document.emit('click',buttons[1]);assert.deepEqual(h.selection,{start:2,end:2});assert.deepEqual(h.scroll.slice(-2),[1,2]);h.api.clear(h.entry);assert(buttons.every(b=>b.disabled));h.dispose();
});
test('mouse reporting does not hide command output and copy gestures create no command block',async()=>{
 const h=harness();await h.write('switch# \x1b[?1000h');h.blocked(true);h.input('copy-only gesture');assert.equal(h.entry._ntInputDraft,undefined);h.blocked(false);h.input('\x1b[<0;10;5M');assert.equal(h.entry._ntInputDraft,undefined);h.input('show status');await h.write('show status');h.input('\r');await h.write('\r\nREADY\r\nswitch# ');assert.equal(h.entry._ntOutputHistory.length,1);assert.equal(h.api.output(h.entry,h.api.current(h.entry)).text,'READY');assert.equal(h.document.querySelector('[data-terminal-extra="copy-output"]').disabled,false);h.dispose();
});
test('output jumps select their complete wrapped destination and respect command-echo preference',async()=>{
 const h=harness();await h.write('switch# ');h.input('show wrapped');await h.write('show wrapped');h.input('\r');
 const first='FIRST '+('wide 界 '.repeat(12)),last='LAST '+('tail '.repeat(19));
 await h.write('\r\n'+first+'\r\nMIDDLE\r\n'+last+'\r\nswitch# ');
 const result=h.api.screenOutput(h.entry,h.api.current(h.entry)),start=result.lines[0],end=result.lines.at(-1);
 assert(start.end>start.start);assert(end.end>end.start);assert.equal(start.text,first);assert.equal(end.text,last);
 h.api.jump(h.entry,'start');assert.deepEqual(h.selection,{start:start.start,end:start.end});assert.equal(h.scroll.at(-1),start.start);
 h.api.jump(h.entry,'end');assert.deepEqual(h.selection,{start:end.start,end:end.end});assert.equal(h.scroll.at(-1),end.end);assert.equal(h.api.prefs(h.entry).autoScroll,false);
 h.entry._ntSessionPrefs.pasteIncludeCommands=true;h.api.jump(h.entry,'start');assert.deepEqual(h.selection,{start:0,end:0});
 h.api.jump(h.entry,'end');assert.deepEqual(h.selection,{start:end.start,end:end.end});assert.equal(h.entry.inputBytes,0);assert.equal(h.entry._ntOutputHistory.length,1);
 h.dispose();
});
test('visible session actions update availability without a menu being opened',()=>{
 const h=harness(),d=h.document;assert.equal(d.querySelector('#terminalActiveActions [data-terminal-action="control-c"]').disabled,false);h.entry.info.state='closed';h.api.sync();assert.equal(d.querySelector('#terminalActiveActions [data-terminal-action="control-c"]').disabled,true);assert.equal(d.querySelector('#terminalActiveActions [data-terminal-action="disconnect"]').disabled,true);assert.equal(d.querySelector('#terminalActiveActions [data-terminal-action="clear"]').disabled,false);h.dispose();
});
test('Open SFTP follows the active connected SSH session and its ownership',()=>{
 const h=harness(),button=h.document.querySelector('#terminalActiveActions [data-terminal-action="open-sftp"]');assert(button);assert.equal(button.textContent,'Open SFTP');assert.equal(button.disabled,false);
 for(const type of ['telnet','serial']){h.entry.info.type=type;h.api.sync();assert.equal(button.disabled,true);}h.entry.info.type='ssh';h.c.online=false;h.api.sync();assert.equal(button.disabled,true);h.c.online=true;h.entry.transferBusy=true;h.api.sync();assert.equal(button.disabled,true);h.entry.transferBusy=false;h.c.terminalOwnerCanMutate=()=>false;h.api.sync();assert.equal(button.disabled,true);h.dispose();
});
test('independent Terminal keeps a working Browse SFTP sidebar action',()=>{
 const h=harness({terminalOnly:true}),button=h.document.querySelector('#terminalActiveActions [data-terminal-action="browse-files"]');assert(button);assert.equal(h.document.querySelector('[data-terminal-action="open-sftp"]'),null);assert.equal(button.disabled,true);h.c.window.NetworkToolkitTerminalSFTP={available:()=>true};h.api.sync();assert.equal(button.disabled,false);h.dispose();
});
test('Open SFTP routes the chosen session into client mode without opening another SSH connection',()=>{
 const calls=[],entry={info:{id:'chosen-ssh',type:'ssh',state:'connected'}},c={state:{transferModes:{sftp:'server'}},TERMINAL_ONLY:false,online:true,currentTerminal:()=>entry,terminalOwnerCanMutate:()=>true,window:{NetworkToolkitRemoteBrowser:{useTerminalSession:id=>calls.push(['session',id])}},navigate:page=>calls.push(['page',page])};vm.createContext(c);vm.runInContext(appFunction('openActiveTerminalSFTP'),c);c.openActiveTerminalSFTP();assert.deepEqual(calls,[['session','chosen-ssh'],['page','sftp']]);assert.equal(c.state.transferModes.sftp,'client');
 entry.info.type='telnet';c.openActiveTerminalSFTP();entry.info.type='ssh';c.online=false;c.openActiveTerminalSFTP();assert.equal(calls.length,2);
});
test('command output remains copyable after its first rows leave console scrollback',async()=>{
 const h=harness();await h.write('switch# ');h.input('show running-config');await h.write('show running-config');h.input('\r');const output=Array.from({length:15000},(_,i)=>'configuration line '+i).join('\r\n');await h.write('\r\n'+output+'\r\nswitch# ');assert.equal(h.entry._ntOutputHistory[0].start.isDisposed,true);const button=h.document.querySelector('[data-terminal-extra="copy-output"]');assert.equal(button.disabled,false);assert.equal(h.api.output(h.entry,h.api.current(h.entry)).text,output.replaceAll('\r',''));h.document.emit('click',button);await Promise.resolve();assert.equal(h.clipboard,output.replaceAll('\r',''));for(const action of ['output-start','output-end','select-output']){const b=h.document.querySelector('[data-terminal-extra="'+action+'"]');assert.equal(b.disabled,false);h.document.emit('click',b);const field=h.document.querySelector('#terminalSelectedOutput');assert.equal(field.value,output.replaceAll('\r',''));assert.equal(field.selectionStart,action==='output-end'?field.value.lastIndexOf('\n')+1:0);assert.equal(field.selectionEnd,action==='output-start'?field.value.indexOf('\n'):field.value.length);if(action!=='select-output')assert.equal(field.value.slice(field.selectionStart,field.selectionEnd),action==='output-start'?'configuration line 0':'configuration line 14999');}const snapshot=h.api.snapshot(h.entry);assert.equal(snapshot.blocks[0].start,-1);assert(snapshot.blocks[0].archive.screen.startsWith('switch# show running-config'));assert(!JSON.stringify(snapshot).includes('rawInput'));h.dispose();
});
test('manual wheel and scrollbar scrolling pause follow through queued output and automatic paging',async()=>{
 const h=harness();await h.write(Array.from({length:70},(_,i)=>'old '+i).join('\r\n'));h.entry._ntSessionPrefs={autoPager:true,autoScroll:false};const one=h.api.beforeOutput(h.entry),two=h.api.beforeOutput(h.entry);h.entry.xterm.scrollToLine(15);await new Promise(resolve=>h.entry.xterm.write('\r\nnew output\r\n',resolve));h.api.afterOutput(h.entry,one);h.entry.xterm.scrollToLine(22);h.api.afterOutput(h.entry,two);assert.equal(h.entry.xterm.buffer.normal.viewportY,22);await h.write('--More--');assert.equal(h.api.prefs(h.entry).autoScroll,false);assert.equal(h.entry.xterm.buffer.normal.viewportY,22);
 h.document.querySelector('#terminalQuickFollow').checked=true;h.document.emit('change',h.document.querySelector('#terminalQuickFollow'));h.entry.element.emit('wheel',h.entry.element,{deltaY:-100,ctrlKey:false});assert.equal(h.api.prefs(h.entry).autoScroll,false);h.api.scrollBottom(h.entry);h.entry.element.emit('pointerdown',{closest:()=>true});assert.equal(h.api.prefs(h.entry).autoScroll,false);h.api.scrollBottom(h.entry);assert.equal(h.api.prefs(h.entry).autoScroll,true);assert.equal(h.entry.xterm.buffer.normal.viewportY,h.entry.xterm.buffer.normal.baseY);h.api.pauseFollow(h.entry);h.input('\x03');assert.equal(h.api.prefs(h.entry).autoScroll,true);h.api.pauseFollow(h.entry);h.input('\x1a');assert.equal(h.api.prefs(h.entry).autoScroll,true);h.dispose();
});
test('Auto scroll off at the bottom stays paused when new output arrives',async()=>{
 const h=harness();await h.write(Array.from({length:50},(_,i)=>'row '+i).join('\r\n'));const at=h.entry.xterm.buffer.normal.viewportY;h.api.pauseFollow(h.entry);await h.write('\r\nONE\r\nTWO\r\nTHREE\r\n');assert.equal(h.entry.xterm.buffer.normal.viewportY,at);h.api.scrollBottom(h.entry);await h.write('FOUR\r\n');assert.equal(h.entry.xterm.buffer.normal.viewportY,h.entry.xterm.buffer.normal.baseY);h.dispose();
});
test('macro start follows the current setting without enabling a paused session',async()=>{
 const h=harness();await h.write(Array.from({length:140},(_,i)=>'row '+i).join('\r\n'));
 const epoch=h.entry._ntScrollEpoch;h.entry.xterm.scrollToLine(20);h.api.followCurrent(h.entry);
 assert.equal(h.entry.xterm.buffer.normal.viewportY,h.entry.xterm.buffer.normal.baseY);assert.equal(h.entry._ntScrollEpoch,epoch);assert.equal(h.api.prefs(h.entry).autoScroll,true);
 h.api.pauseFollow(h.entry);h.entry.xterm.scrollToLine(25);const pausedEpoch=h.entry._ntScrollEpoch;h.api.followCurrent(h.entry);
 assert.equal(h.entry.xterm.buffer.normal.viewportY,25);assert.equal(h.entry._ntScrollEpoch,pausedEpoch);assert.equal(h.api.prefs(h.entry).autoScroll,false);assert.equal(h.entry.xterm._core._bufferService.isUserScrolling,true);
 h.api.followCurrent(null);h.dispose();
});
test('terminal fitting retains the visible top line while paused at saturated scrollback',async()=>{
 const h=harness();await h.write(Array.from({length:180},(_,i)=>'row '+i).join('\r\n'));h.api.pauseFollow(h.entry);h.entry.xterm.scrollToLine(25);
 const top=()=>h.entry.xterm.buffer.normal.getLine(h.entry.xterm.buffer.normal.viewportY).translateToString(true),original=top();
 for(const rows of [8,4,6]){const position=h.api.beforeFit(h.entry);assert(position.anchor);h.entry.xterm.resize(40,rows);h.api.afterFit(h.entry,position);assert.equal(top(),original);assert.equal(h.api.prefs(h.entry).autoScroll,false);assert.equal(position.anchor.isDisposed,true);}
 h.dispose();
});
test('terminal fitting follows enabled sessions and respects a later manual pause',async()=>{
 const h=harness();await h.write(Array.from({length:180},(_,i)=>'row '+i).join('\r\n'));h.entry.xterm.scrollToLine(25);
 let position=h.api.beforeFit(h.entry);h.entry.xterm.resize(40,8);h.api.afterFit(h.entry,position);assert.equal(h.entry.xterm.buffer.normal.viewportY,h.entry.xterm.buffer.normal.baseY);
 h.api.pauseFollow(h.entry);h.entry.xterm.scrollToLine(25);position=h.api.beforeFit(h.entry);h.entry.xterm.resize(40,6);h.entry.element.emit('wheel',h.entry.element,{deltaY:-100,ctrlKey:false});h.entry.xterm.scrollToLine(35);h.api.afterFit(h.entry,position);
 assert.equal(h.entry.xterm.buffer.normal.viewportY,35);assert.equal(h.api.prefs(h.entry).autoScroll,false);assert.equal(position.anchor.isDisposed,true);h.dispose();
});
test('Last command and Last paste form an exclusive two-button switch for output actions',async()=>{
 const h=harness(),d=h.document,command=d.querySelector('[data-terminal-output-kind="last-command"]'),paste=d.querySelector('[data-terminal-output-kind="last-paste"]');
 assert.equal(d.querySelector('#terminalOutputBlock').tagName,'DIV');assert.equal(d.querySelectorAll('[data-terminal-output-kind]').length,2);
 await h.write('switch# ');h.input('show command');await h.write('show command');h.input('\r');await h.write('\r\nCOMMAND RESULT\r\nswitch# ');
 h.api.notePaste(h.entry,'show pasted\r');await h.write('show pasted\r\nPASTE RESULT\r\nswitch# ');
 const active=()=>[command,paste].filter(button=>button.getAttribute('aria-pressed')==='true');
 assert.deepEqual(active(),[paste]);
 d.emit('click',command);assert.deepEqual(active(),[command]);d.emit('click',command);assert.deepEqual(active(),[command]);
 d.emit('click',d.querySelector('[data-terminal-extra="copy-output"]'));await Promise.resolve();assert.equal(h.clipboard,'COMMAND RESULT');
 d.emit('click',paste);assert.deepEqual(active(),[paste]);d.emit('click',d.querySelector('[data-terminal-extra="copy-output"]'));await Promise.resolve();assert.equal(h.clipboard,'PASTE RESULT');
 d.emit('click',command);assert.deepEqual(active(),[command]);
 const other={info:{id:'two',state:'connected'},xterm:h.entry.xterm,element:new Element(),disposables:[]};h.c.currentTerminal=()=>other;h.api.sync();d.emit('click',paste);assert.equal(other._ntOutputChoice,'last-paste');assert.equal(d.querySelector('[data-terminal-extra="copy-output"]').disabled,true);
 h.c.currentTerminal=()=>h.entry;h.api.sync();assert.deepEqual(active(),[command]);assert.equal(h.api.snapshot(h.entry).choice,'last-command');
 h.c.currentTerminal=()=>null;h.api.sync();assert(command.disabled&&paste.disabled);assert.equal(active().length,1);h.dispose();
});
test('a running command retains earlier output after handoff and continues recording in the new view',async()=>{
 const a=harness();await a.write('switch# ');a.input('show long');await a.write('show long');a.input('\r');const lines=Array.from({length:12000},(_,i)=>'line '+i).join('\r\n');await a.write('\r\n'+lines+'\r\n');const snapshot=JSON.parse(JSON.stringify(a.api.snapshot(a.entry)));assert.equal(snapshot.blocks[0].start,-1);const b=harness();await b.write('line 11999\r\n');b.api.restore(b.entry,snapshot);await b.api.flush(b.entry);await b.write('CONTINUED\r\nswitch# ');assert.equal(b.api.output(b.entry,b.api.current(b.entry)).text,lines.replaceAll('\r','')+'\nCONTINUED');a.dispose();b.dispose();
});
test('Select and Copy retain every line past the old byte, row and parsed-text limits',async()=>{
 const {createHash}=require('node:crypto'),hash=text=>createHash('sha256').update(text).digest('hex'),h=harness();
 await h.write('switch# ');h.input('show everything');await h.write('show everything');h.input('\r');
 const lines=Array.from({length:90000},(_,i)=>'row '+String(i).padStart(6,'0')+' '+('data '+i+' 界 🌐 ').padEnd(220,'x'));
 lines[0]='FIRST OUTPUT LINE';lines[lines.length-1]='FINAL OUTPUT LINE AFTER ALL FORMER LIMITS';
 const expected=lines.join('\n'),wire=new TextEncoder().encode('\r\n'+lines.map((line,i)=>i%19?'\x1b[32m'+line+'\x1b[0m':line).join('\r\n')+'\r\nswitch# ');
 assert(wire.length>8*1024*1024);assert(Buffer.byteLength(expected)>16*1024*1024);assert(lines.length>50000);
 await h.write(wire);
 const item=h.api.current(h.entry),result=h.api.output(h.entry,item),check=text=>{assert.equal(text.length,expected.length);assert.equal(hash(text),hash(expected));assert(text.startsWith(lines[0]));assert(text.endsWith(lines.at(-1)));};
 assert.equal(item.start.isDisposed,true);assert.equal(result.truncated,false);check(result.text);
 assert.equal(await h.api.copy(h.entry),true);check(h.clipboard);
 assert.equal(await h.api.select(h.entry),true);const field=h.document.querySelector('#terminalSelectedOutput');check(field.value);assert.equal(field.selectionStart,0);assert.equal(field.selectionEnd,expected.length);
 // Closing a command must keep the complete archive, rather than pruning it
 // as soon as its total bytes exceed the former session budget.
 h.input('show next');await h.write('show next');h.input('\r');await h.write('\r\nNEXT COMMAND\r\nswitch# ');
 assert.equal(item.closed,true);check(h.api.output(h.entry,item).text);await h.api.copy(h.entry,false,item.id);check(h.clipboard);
 const transfer=h.api.snapshot(h.entry).blocks.find(b=>b.id===item.id).archive;
 assert.equal(transfer.truncated,true);assert(Buffer.byteLength(transfer.screen)<=4*1024*1024);assert(transfer.screen.endsWith(lines.at(-1)));check(h.api.output(h.entry,item).text);
 h.dispose();
});
test('output actions wait for queued parsing and retain the block chosen at click time',async()=>{
 const h=harness();await h.write('switch# ');h.input('show queued');await h.write('show queued');h.input('\r');await h.write('\r\nFIRST\r\n');
 const lines=Array.from({length:18000},(_,i)=>'queued line '+i),wire=lines.join('\r\n')+'\r\nFINAL QUEUED LINE\r\nswitch# ',expected='FIRST\n'+lines.join('\n')+'\nFINAL QUEUED LINE';
 h.api.observeOutput(h.entry,wire);
 const selected=h.api.select(h.entry),copied=h.api.copy(h.entry);
 assert.equal(h.clipboard,'');assert.equal(h.document.querySelector('#terminalSelectedOutput'),null);
 // Selecting another output source while the parser catches up must not
 // retarget either operation to the other block.
 h.entry._ntOutputChoice='last-paste';
 assert.equal(await copied,true);assert.equal(await selected,true);assert.equal(h.clipboard,expected);assert.equal(h.document.querySelector('#terminalSelectedOutput').value,expected);
 h.dispose();
});
test('sealing wrapped rows preserves very long logical lines and interleaved paste echoes',async()=>{
 const h=harness();await h.write('switch# ');h.api.notePaste(h.entry,'show huge\rshow final\r');
 const long='abc 界 🌐 '.repeat(47000)+'END OF LONG LOGICAL LINE',rows=Array.from({length:11000},(_,i)=>'row '+i).join('\r\n');
 await h.write('show huge\r\n'+long+'\r\n'+rows+'\r\nswitch# show final\r\nLAST PASTED OUTPUT\r\nswitch# ');
 const expected=long+'\n'+rows.replaceAll('\r','')+'\nLAST PASTED OUTPUT',result=h.api.output(h.entry,h.api.current(h.entry));
 let difference=0;while(difference<expected.length&&result.text[difference]===expected[difference])difference++;
 assert.equal(result.text.length,expected.length,JSON.stringify({difference,actual:result.text.slice(difference-15,difference+50),expected:expected.slice(difference-15,difference+50)}));assert.equal(result.text,expected);assert.equal(result.truncated,false);await h.api.copy(h.entry);assert.equal(h.clipboard,expected);
 assert(h.api.output(h.entry,h.api.current(h.entry),true).text.includes('switch# show final\nLAST PASTED OUTPUT'));
 h.dispose();
});

test('screenshot pager and manual Space Enter q Ctrl+C never split complete command archives',async()=>{
 const pager='-- MORE --, next page: Space, next line: Enter, quit: q';
 for(const key of [' ','\r','q','\x03']){
  const h=harness();await h.write('switch# ');h.input('show running-config');await h.write('show running-config');h.input('\r');
  await h.write('\r\nFIRST\r\n'+pager);h.input(key);assert.equal(h.entry._ntOutputHistory.length,1);assert.equal(h.entry._ntInputDraft,null);
  // The 40-column fixture wraps this prompt over two physical rows.
  await h.write('\r\x1b[2K\x1b[A\x1b[2KSECOND\r\nswitch# ');assert.equal(h.api.output(h.entry,h.api.current(h.entry)).text,'FIRST\nSECOND');
  await h.api.copy(h.entry);assert.equal(h.clipboard,'FIRST\nSECOND');h.dispose();
 }
});
test('manual multiline paste paging retains all output across ANSI redraw and scrollback trim',async()=>{
 const h=harness(),pager='-- MORE --, next page: Space, next line: Enter, quit: q';await h.write('switch# ');h.api.notePaste(h.entry,'show one\rshow two\r');
 const first=Array.from({length:500},(_,i)=>'FIRST '+i),second=Array.from({length:500},(_,i)=>'SECOND '+i);
 await h.write('show one\r\n'+first.join('\r\n')+'\r\n'+pager);h.input(' ');
 await h.write('\r\x1b[2K\x1b[A\x1b[2Kswitch# show two\r\n'+second.join('\r\n')+'\r\n'+pager);h.input('\r');
 await h.write('\r\x1b[2K\x1b[A\x1b[2Kswitch# ');
 const item=h.api.current(h.entry,'last-paste');assert.equal(h.entry._ntOutputHistory.length,1);assert.equal(item.start.isDisposed,true);assert.equal(h.api.output(h.entry,item).text,[...first,...second].join('\n'));
 await h.api.select(h.entry);assert.equal(h.document.querySelector('#terminalSelectedOutput').value,[...first,...second].join('\n'));await h.api.copy(h.entry);assert.equal(h.clipboard,[...first,...second].join('\n'));h.dispose();
});
test('non-echoing command and paste do not lose their first output line',async()=>{
 for(const pasted of [false,true]){
  const h=harness();await h.write('switch# ');if(pasted)h.api.notePaste(h.entry,'show hidden-echo\r');else{h.input('show hidden-echo');h.input('\r');}
  await h.write('\r\nFIRST WITHOUT ECHO\r\nLAST WITHOUT ECHO\r\nswitch# ');
  // A no-echo paste has no command to strip; the original idle prompt is not output.
  assert.equal(h.api.output(h.entry,h.api.current(h.entry)).text,'FIRST WITHOUT ECHO\nLAST WITHOUT ECHO');h.dispose();
 }
});
test('ANSI redraw over the input row preserves output instead of treating the old marker as echo',async()=>{
 const h=harness();await h.write('switch# ');h.input('show redraw');await h.write('show redraw');h.input('\r');
 await h.write('\r\x1b[2KFIRST REDRAWN OUTPUT\r\nLAST OUTPUT\r\nswitch# ');
 const item=h.api.current(h.entry);assert.equal(h.api.output(h.entry,item).text,'FIRST REDRAWN OUTPUT\nLAST OUTPUT');assert.equal(h.api.screenOutput(h.entry,item).text,'FIRST REDRAWN OUTPUT\nLAST OUTPUT');
 await h.api.select(h.entry);assert.deepEqual(h.selection,{start:0,end:1});h.dispose();
});
test('no-echo paste output identical to a submitted command remains real output',async()=>{
 const h=harness();await h.write('switch# ');h.api.notePaste(h.entry,'show version\r');await h.write('\r\nshow version\r\nRESULT\r\nswitch# ');
 assert.equal(h.api.output(h.entry,h.api.current(h.entry)).text,'show version\nRESULT');h.dispose();
});
test('paste and delayed echo archives preserve the prompt cursor spacing',async()=>{
 for(const paste of [true,false]){
  const h=harness();await h.write('switch# ');if(paste)h.api.notePaste(h.entry,'show version\r');else{h.input('show version');h.input('\r');}
  await h.write('show version\r\nRESULT\r\nswitch# ');const item=h.api.current(h.entry);
  assert.equal(h.api.output(h.entry,item,true).text,'switch# show version\nRESULT');assert.equal(h.api.output(h.entry,item).text,'RESULT');h.dispose();
 }
});
test('restored paused pagers keep manual continuation in the original output archive',async()=>{
 const a=harness();await a.write('switch# ');a.input('show long');await a.write('show long');a.input('\r');await a.write('\r\nFIRST\r\n--More--');
 const snapshot=JSON.parse(JSON.stringify(a.api.snapshot(a.entry))),b=harness();await b.write('switch# show long\r\nFIRST\r\n--More--');b.api.restore(b.entry,snapshot);await b.api.flush(b.entry);
 assert.equal(b.api.pagerInput(b.entry,' '),true);b.input(' ');assert.equal(b.entry._ntInputDraft,null);assert.equal(b.api.current(b.entry).closed,false);
 await b.write('\r\x1b[2KSECOND\r\nswitch# ');assert.equal(b.entry._ntOutputHistory.length,1);assert.equal(b.api.output(b.entry,b.api.current(b.entry)).text,'FIRST\nSECOND');a.dispose();b.dispose();
});

test('session tabs stay directly above console while top and bottom actions retain identity',()=>{
 const h=harness({tabs:true}),d=h.document,tabs=d.querySelector('#terminalTabs'),stage=d.querySelector('#terminalStage'),row=d.querySelector('#terminalActiveActions'),picker=d.querySelector('#terminalActiveActionsPosition'),console=d.querySelector('#retainedConsole');
 const previous=node=>node.parentElement.children[node.parentElement.children.indexOf(node)-1];
 assert.equal(previous(stage),tabs);assert.equal(previous(tabs),row);
 for(const position of ['bottom','top','bottom','top']){picker.value=position;d.emit('change',picker);h.api.sync();assert.equal(previous(stage),tabs);assert.equal(d.querySelector('#retainedConsole'),console);assert.equal(d.querySelector('#terminalActiveActions'),row);}
 h.dispose();
});
test('terminal workspace puts tabs after notices, prompts and search directly above panes',()=>{
 const source=appFunction('renderTerminalWorkspace'),tabs=source.indexOf('id="terminalTabs"'),stage=source.indexOf('id="terminalStage"');
 assert(tabs>source.indexOf('id="terminalSearch"'));assert(tabs>source.indexOf('id="terminalPrompt"'));assert(tabs>source.indexOf('id="terminalNotices"'));
 assert.match(source.slice(tabs,stage),/Terminal sessions"><\/div><div $/);
 const macros=fs.readFileSync(path.join(__dirname,'../web/assets/terminal-macros.js'),'utf8');assert(macros.includes("($('#terminalTabs')||stage).before(el)"));
});
test('repeated narrow/wide pane fits reflow normal history without moving a paused logical anchor',async()=>{
 const h=harness();await h.write(Array.from({length:35},(_,i)=>'ROW-'+String(i).padStart(2,'0')+' synthetic terminal history').join('\r\n'));
 h.api.pauseFollow(h.entry);h.entry.xterm.scrollToLine(8);const original=h.entry.xterm.buffer.normal.getLine(8).translateToString(true);
 for(const [cols,rows] of [[20,8],[40,4],[22,9],[40,6]]){const pos=h.api.beforeFit(h.entry);h.entry.xterm.resize(cols,rows);h.api.afterFit(h.entry,pos);const b=h.entry.xterm.buffer.normal;let text=b.getLine(b.viewportY).translateToString(true);for(let y=b.viewportY+1;b.getLine(y)?.isWrapped;y++)text+=b.getLine(y).translateToString(true);assert.equal(text,original);assert.equal(h.api.prefs(h.entry).autoScroll,false);}
 h.api.scrollBottom(h.entry);for(const cols of [20,40,24,40]){const pos=h.api.beforeFit(h.entry);h.entry.xterm.resize(cols,6);h.api.afterFit(h.entry,pos);assert.equal(h.entry.xterm.buffer.normal.viewportY,h.entry.xterm.buffer.normal.baseY);}
 h.dispose();
});
test('alternate screen fits retain buffer mode and signal geometry without fabricating remote redraw',async()=>{
 const h=harness();await h.write('normal history\r\n\x1b[?1049h\x1b[2J\x1b[HREMOTE TUI');const sizes=[];h.entry.xterm.onResize(size=>sizes.push(size));
 for(const [cols,rows] of [[20,8],[60,12],[40,6]]){const pos=h.api.beforeFit(h.entry);assert.equal(pos.anchor,null);h.entry.xterm.resize(cols,rows);h.api.afterFit(h.entry,pos);assert.equal(h.entry.xterm.buffer.active.type,'alternate');assert.equal(h.entry.xterm.cols,cols);assert.equal(h.entry.xterm.rows,rows);}
 assert.equal(sizes.length,3);await h.write('\x1b[2J\x1b[HREDRAW AT NEW SIZE');assert.equal(h.entry.xterm.buffer.active.getLine(0).translateToString(true),'REDRAW AT NEW SIZE');await h.write('\x1b[?1049l');assert.equal(h.entry.xterm.buffer.active.type,'normal');assert.match(h.entry.xterm.buffer.normal.getLine(0).translateToString(true),/normal history/);h.dispose();
});


test('Reconnect stays primary with a separate accessible arrow and Reconnect all below',()=>{
 const h=harness(),d=h.document,row=d.querySelector('#terminalActiveActions'),group=row.querySelector('.terminal-reconnect-split');
 assert(group,'split reconnect group should be rendered in the active session bar');
 const primary=group.querySelector('[data-terminal-action="reconnect"]');
 const arrow=group.querySelector('#terminalReconnectMore');
 const secondary=group.querySelector('#terminalReconnectOptions [data-terminal-action="reconnect-all"]');
 assert(primary&&arrow&&secondary,'primary button, arrow and secondary button must exist');
 assert.equal(primary.textContent,'Reconnect');
 assert.equal(secondary.textContent,'Reconnect all');
 assert.equal(arrow.getAttribute('aria-label'),'More reconnect options');
 assert.equal(arrow.getAttribute('aria-controls'),'terminalReconnectOptions');
 assert.equal(arrow.getAttribute('aria-expanded'),'false');
 assert(group.querySelector('#terminalReconnectOptions').classList.contains('hidden'));
 assert.equal(primary.parentElement,arrow.parentElement);
 assert.equal(arrow.parentElement,secondary.parentElement.parentElement);
 let closed=0;
 h.c.window.NetworkToolkitTerminalTabs={reconnectAllAvailable:()=>true,reconnectAllBusy:()=>false,setReconnectOptions:()=>{closed++;}};
 h.api.sync();
 assert.equal(arrow.disabled,false);
 assert.equal(secondary.disabled,false);
 h.c.window.NetworkToolkitTerminalTabs.reconnectAllAvailable=()=>false;
 h.api.sync();
 assert.equal(arrow.disabled,true);
 assert.equal(secondary.disabled,true);
 assert.equal(closed,1);
 h.dispose();
});


test('Disconnect split group sits immediately beside Reconnect and exposes a separately enabled Disconnect all',()=>{
 const h=harness(),d=h.document,row=d.querySelector('#terminalActiveActions');
 const disconnect=row.querySelector('.terminal-disconnect-split'),reconnect=row.querySelector('.terminal-reconnect-split');
 assert(disconnect&&reconnect);
 assert.equal(disconnect.parentElement,reconnect.parentElement);
 const children=[...disconnect.parentElement.children];
 assert.equal(children.indexOf(reconnect),children.indexOf(disconnect)+1,'Reconnect must be directly next to Disconnect');
 const primary=disconnect.querySelector('[data-terminal-action="disconnect"]');
 const arrow=disconnect.querySelector('#terminalDisconnectMore');
 const secondary=disconnect.querySelector('#terminalDisconnectOptions [data-terminal-action="disconnect-all"]');
 assert(primary&&arrow&&secondary);
 assert.equal(primary.textContent,'Disconnect');
 assert.equal(secondary.textContent,'Disconnect all');
 assert.equal(arrow.getAttribute('aria-label'),'More disconnect options');
 assert.equal(arrow.getAttribute('aria-controls'),'terminalDisconnectOptions');
 assert.equal(arrow.getAttribute('aria-expanded'),'false');
 assert(disconnect.querySelector('#terminalDisconnectOptions').classList.contains('hidden'));
 let closed=0;
 h.c.window.NetworkToolkitTerminalTabs={disconnectAllAvailable:()=>true,reconnectAllAvailable:()=>true,reconnectAllBusy:()=>false,windowBulkBusy:()=>false,setDisconnectOptions:()=>{closed++;}};
 h.api.sync();
 assert.equal(arrow.disabled,false);assert.equal(secondary.disabled,false);
 h.c.window.NetworkToolkitTerminalTabs.disconnectAllAvailable=()=>false;
 h.api.sync();
 assert.equal(arrow.disabled,true);assert.equal(secondary.disabled,true);assert.equal(closed,1);
 h.dispose();
});
