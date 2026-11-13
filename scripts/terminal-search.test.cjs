'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
global.self=global;const {Terminal}=require('../web/assets/xterm.js'),{SearchAddon}=require('../web/assets/addon-search.js');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
test('search clear button removes the query, matches and selections in split sessions and keeps search focus',async t=>{
 const markup=html.match(/<div id="terminalSearch"[\s\S]*?<div id="terminalNotices"/)[0].replace(/<div id="terminalNotices"$/,'');
 const document=makeDocument(markup),sessions=new Map();let focused=false;
 t.after(()=>{for(const entry of sessions.values())entry.xterm.dispose();});
 for(const id of ['one','two']){
  const xterm=new Terminal({cols:60,rows:6,allowProposedApi:true}),search=new SearchAddon();xterm.loadAddon(search);
  // Headless xterm has no DOM selection service. Keep its real buffer and
  // search algorithm, replacing only the selection display operations.
  let selection=null;
  xterm.select=(column,row,length)=>{selection={column,row,length};};
  xterm.clearSelection=()=>{selection=null;};xterm.hasSelection=()=>!!selection;
  xterm.getSelection=()=>selection?xterm.buffer.active.getLine(selection.row).translateToString(true,selection.column,selection.column+selection.length):'';
  xterm.getSelectionPosition=()=>selection?{start:{x:selection.column,y:selection.row},end:{x:selection.column+selection.length,y:selection.row}}:undefined;
  await new Promise(resolve=>xterm.write('first result\r\nfind needle here\r\nlast result',resolve));
  sessions.set(id,{xterm,search});
 }
 const context={document,terminalSessions:sessions,currentTerminal:()=>sessions.get('one'),$:selector=>document.querySelector(selector),handleListClick:()=>false,maximizeTerminalPane(){},closeTerminalPane(){}};
 vm.createContext(context);
 for(const name of ['clearTerminalSearch','searchTerminal','handleExtendedClick'])vm.runInContext(html.split('\n').find(line=>line.startsWith('function '+name+'(')),context);
 const input=document.getElementById('terminalSearchInput'),clear=document.getElementById('terminalSearchClear');input.focus=()=>{focused=true;};
 input.value='needle';context.searchTerminal();assert.equal(clear.disabled,false);assert.equal(sessions.get('one').xterm.getSelection(),'needle');
 sessions.get('two').search.findNext('needle');
 context.handleExtendedClick({target:clear,preventDefault(){}});
 assert.equal(input.value,'');assert.equal(clear.disabled,true);assert.equal(document.getElementById('terminalSearchResult').textContent,'');assert.equal(focused,true);
 for(const entry of sessions.values())assert.equal(entry.xterm.hasSelection(),false);
});
test('Find is a toggle: the Find button opens and closes the search row, which has no separate close button',()=>{
 // Owner rule: a button that reveals a row is a toggle that also closes it.
 const markup=html.match(/<div id="terminalSearch"[\s\S]*?<div id="terminalNotices"/)[0].replace(/<div id="terminalNotices"$/,'');
 const find=html.match(/<button class="iconbtn reveal-toggle" data-terminal-action="search"[^>]*>/)?.[0];
 assert(find,'Find button is a reveal toggle');assert.match(find,/aria-controls="terminalSearch"/);assert.match(find,/aria-expanded="false"/);assert.match(find,/aria-keyshortcuts="Control\+Shift\+F"/);
 assert.match(markup,/^<div id="terminalSearch" class="terminal-search hidden"/,'Find starts closed');
 assert(!html.includes('close-search'),'No separate close button or action for Find');
 const document=makeDocument('<div id="terminalCommandBar">'+find+'</button></div>'+markup),decorations=[];let terminalFocus=0,inputFocus=0;
 const entry={xterm:{focus(){terminalFocus++;}},search:{clearDecorations(){decorations.push('one');}}},sessions=new Map([['one',entry],['two',{search:{clearDecorations(){decorations.push('two');}}}]]);
 const context={document,terminalSessions:sessions,currentTerminal:()=>entry,$:selector=>document.querySelector(selector),$$:selector=>document.querySelectorAll(selector),handleListClick:()=>false,maximizeTerminalPane(){},closeTerminalPane(){}};
 vm.createContext(context);
 for(const name of ['terminalSearchOpen','syncTerminalSearchToggle','showTerminalSearch','hideTerminalSearch','toggleTerminalSearch','handleExtendedClick'])vm.runInContext(html.split('\n').find(line=>line.startsWith('function '+name+'(')),context);
 const bar=document.getElementById('terminalSearch'),button=document.querySelector('[data-terminal-action="search"]');document.getElementById('terminalSearchInput').focus=()=>{inputFocus++;};
 const click=()=>context.handleExtendedClick({target:button,preventDefault(){}});
 click();assert.equal(bar.classList.contains('hidden'),false);assert.equal(button.getAttribute('aria-expanded'),'true');assert.equal(inputFocus,1);
 click();assert.equal(bar.classList.contains('hidden'),true);assert.equal(button.getAttribute('aria-expanded'),'false');assert.equal(terminalFocus,1,'Closing returns focus to the terminal');assert.deepEqual(decorations,['one','two'],'Closing clears search highlights');
 // Menu entries named Find only open it; Ctrl+Shift+F (terminal-interaction.js) toggles like the button.
 context.showTerminalSearch();context.showTerminalSearch();assert.equal(bar.classList.contains('hidden'),false);assert.equal(button.getAttribute('aria-expanded'),'true');
 context.toggleTerminalSearch();assert.equal(bar.classList.contains('hidden'),true);assert.equal(button.getAttribute('aria-expanded'),'false');
 context.toggleTerminalSearch();assert.equal(bar.classList.contains('hidden'),false);
 context.hideTerminalSearch(false);assert.equal(bar.classList.contains('hidden'),true);assert.equal(terminalFocus,2,'A quiet close leaves focus where it is');
 assert.match(fs.readFileSync(require.resolve('../web/assets/terminal-interaction.js'),'utf8'),/key === 'f'\) \{[^}]*toggleTerminalSearch\(\)/,'Ctrl+Shift+F toggles Find');
});
