'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(path.join(__dirname,'../web/assets/terminal-extras.js'),'utf8');

function harness(){
 const frames=new Map();let id=0,queries=0;
 const active={_ntSessionPrefs:{autoScroll:false},xterm:{}},inactive={_ntSessionPrefs:{autoScroll:false},xterm:{}};
 const window={
  requestAnimationFrame(fn){const key=++id;frames.set(key,fn);return key;},
  cancelAnimationFrame(key){frames.delete(key);},
  NetworkToolkitTerminalInteraction:{preferences:()=>({autoScroll:false})}
 };
 const document={querySelector(selector){if(selector==='#terminalCommandBar')queries++;return null;}};
 const ctx={window,document,currentTerminal:()=>active};
 vm.runInNewContext(source,ctx);
 const api=window.NetworkToolkitTerminalExtras;
 return {api,active,inactive,frames,get queries(){return queries;},runFrame(){
  const next=[...frames.entries()];frames.clear();
  next.forEach(([,fn])=>fn());
 }};
}

test('many output callbacks in the active terminal schedule one toolbar refresh',()=>{
 const h=harness();
 for(let i=0;i<100;i++)h.api.afterOutput(h.active,{epoch:0});
 assert.equal(h.frames.size,1);
 assert.equal(h.queries,0,'output processing should not synchronously query the toolbar');
 h.runFrame();
 assert.equal(h.queries,1);
 h.api.afterOutput(h.active,{epoch:0});
 assert.equal(h.frames.size,1,'subsequent frames must still update');
 h.runFrame();
 assert.equal(h.queries,2);
});

test('inactive terminal output does not redraw controls for the active terminal',()=>{
 const h=harness();
 for(let i=0;i<100;i++)h.api.afterOutput(h.inactive,{epoch:0});
 assert.equal(h.frames.size,0);
 assert.equal(h.queries,0);
 h.api.afterOutput(h.active,{epoch:0});
 h.api.sync();
 assert.equal(h.queries,1,'explicit refresh remains immediate');
 assert.equal(h.frames.size,0,'explicit refresh cancels stale scheduled redraw');
 h.runFrame();assert.equal(h.queries,1);
});
