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
