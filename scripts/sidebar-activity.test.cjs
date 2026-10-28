'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const source=fs.readFileSync(require.resolve('../web/assets/sidebar-activity.js'),'utf8');
const deferred=()=>{let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b;});return {promise,resolve,reject};};
async function flush(){for(let i=0;i<12;i++)await Promise.resolve();}
function harness(){
 const document=makeDocument('<nav id="navigation">'+[['terminal','Terminal'],['sftp','SFTP / FTP / FTPS'],['tftp','TFTP'],['discover','Network discovery'],['capture','Packet capture'],['profiles','Saved targets']].map(([id,name])=>'<button class="nav-item" data-page="'+id+'" aria-label="'+name+'" title="'+name+'. Double-click to collapse navigation."><svg></svg><span class="nav-item-text">'+name+'</span></button>').join('')+'</nav>');
 let now=0,id=0,enabled=false;const timers=new Map(),events={},calls=[];
 const context={document,console,Promise,Set,WeakMap,Date,AbortController,performance:{now:()=>now},setTimeout:(fn,delay)=>{timers.set(++id,{fn,at:now+delay});return id;},clearTimeout:key=>timers.delete(key),addEventListener:(name,fn)=>(events[name]||=[]).push(fn)};
 context.window=context;vm.runInNewContext(source,context);const api=context.NetworkToolkitSidebarActivity;
 const request=signal=>{const d=deferred();calls.push({...d,signal});return d.promise;};
 return {api,document,calls,events,timers,get enabled(){return enabled;},set enabled(v){enabled=v;},init(){api.init({request,available:()=>enabled});},button:page=>document.querySelector('[data-page="'+page+'"]'),dots:()=>document.querySelectorAll('.sidebar-activity-dot'),async tick(ms){now+=ms;for(const [key,timer]of [...timers])if(timer.at<=now){timers.delete(key);timer.fn();}await flush();},async respond(value,index=calls.length-1){calls[index].resolve(value);await flush();},async reject(index=calls.length-1){calls[index].reject(new Error('engine unavailable'));await flush();},async event(name){for(const fn of events[name]||[])fn();await flush();}};
}
test('only bounded numeric activity categories and supported pages survive; descriptions contain no source strings',()=>{
 const h=harness();const value=h.api.normalize({tools:{terminal:{connections:2,password:'secret',state:'connected'},sftp:{servers:1,operations:3},capture:{captures:1},hosts:{operations:1},profiles:{connections:9},tftp:{servers:-1},discover:{operations:1.5},ping:{operations:Infinity},http:{operations:1000001}}});
 assert.deepEqual(JSON.parse(JSON.stringify(value)),{terminal:{connections:2},sftp:{servers:1,operations:3},capture:{captures:1},hosts:{operations:1}});
 assert.equal(h.api.summary(value.terminal),'2 connected sessions');assert.equal(h.api.summary(value.sftp),'Server running; 3 operations running');assert.equal(h.api.summary(value.capture),'Capture running');assert(!JSON.stringify(value).includes('secret'));assert.throws(()=>h.api.normalize({}));assert.throws(()=>h.api.normalize({tools:[]}));
});
test('initial/offline and saved profiles alone do not create a request or dot; online snapshot is authoritative',async()=>{
 const h=harness();h.init();await flush();assert.equal(h.calls.length,0);assert.equal(h.dots().length,0);
 h.enabled=true;h.api.connectionChanged();await flush();assert.equal(h.calls.length,1);await h.respond({tools:{profiles:{connections:3}}});assert.equal(h.dots().length,0);h.api.stop();
});
test('session counts, idle listening servers and running operations update in place and last close clears',async()=>{
 const h=harness();h.enabled=true;h.init();await flush();const terminal=h.button('terminal'),label=terminal.querySelector('.nav-item-text');
 await h.respond({tools:{terminal:{connections:2},sftp:{connections:1,servers:1},tftp:{servers:1},discover:{operations:2},capture:{captures:1}}});
 assert.equal(h.dots().length,5);assert.equal(terminal.getAttribute('aria-label'),'Terminal, 2 connected sessions');assert.match(h.button('sftp').getAttribute('title'),/1 connected session; Server running/);assert.equal(terminal.querySelector('.nav-item-text'),label);
 const dot=terminal.querySelector('.sidebar-activity-dot');await h.tick(1000);await h.respond({tools:{terminal:{connections:1},tftp:{servers:1}}});assert.equal(terminal.querySelector('.sidebar-activity-dot'),dot);assert.match(terminal.getAttribute('aria-label'),/1 connected session$/);assert.equal(h.dots().length,2);
 await h.tick(1000);await h.respond({tools:{}});assert.equal(h.dots().length,0);assert.equal(terminal.getAttribute('aria-label'),'Terminal');assert.equal(terminal.getAttribute('title'),'Terminal. Double-click to collapse navigation.');h.api.stop();
});
test('refresh requests coalesce and navigation metadata remains accurate across rerenders and collapse',async()=>{
 const h=harness();h.enabled=true;h.init();await flush();const a=h.api.refresh(),b=h.api.refresh();assert.equal(a,b);assert.equal(h.calls.length,1);await h.respond({tools:{terminal:{connections:1}}});
 const button=h.button('terminal');button.setAttribute('aria-label','Terminal');button.setAttribute('title','Terminal. Double-click to expand navigation.');h.api.sync();h.api.sync();assert.equal(button.getAttribute('aria-label'),'Terminal, 1 connected session');assert.equal(button.getAttribute('title'),'Terminal. Double-click to expand navigation.\n1 connected session.');
 h.document.getElementById('navigation').innerHTML='<button class="nav-item" data-page="terminal" aria-label="Terminal"><svg></svg><span class="nav-item-text">Terminal</span></button>';h.api.sync();assert.equal(h.dots().length,1);assert.equal(h.button('terminal').getAttribute('aria-label'),'Terminal, 1 connected session');h.api.stop();assert.equal(h.dots().length,0);assert.equal(h.button('terminal').getAttribute('title'),null);
});
test('failed or malformed reads clear existing activity and recover only after a fresh successful snapshot',async()=>{
 const h=harness();h.enabled=true;h.init();await flush();await h.respond({tools:{terminal:{connections:1}}});await h.tick(1000);await h.reject();assert.equal(h.dots().length,0);
 await h.tick(1000);await h.respond({});assert.equal(h.dots().length,0);await h.tick(1000);await h.respond({tools:{terminal:{connections:1}}});assert.equal(h.dots().length,1);h.api.stop();
});
test('request timeout clears a stale dot and a late response cannot revive it or overwrite newer state',async()=>{
 const h=harness();h.enabled=true;h.init();await flush();await h.respond({tools:{terminal:{connections:1}}});await h.tick(1000);const old=h.calls.at(-1);await h.tick(2500);assert.equal(h.dots().length,0);assert(old.signal.aborted);
 await h.tick(1000);await h.respond({tools:{tftp:{servers:1}}});old.resolve({tools:{terminal:{connections:99}}});await flush();assert.equal(h.dots().length,1);assert(h.button('tftp').querySelector('.sidebar-activity-dot'));assert(!h.button('terminal').querySelector('.sidebar-activity-dot'));h.api.stop();
});
test('offline/restart generation invalidates a pending response without network fanout',async()=>{
 const h=harness();h.enabled=true;h.init();await flush();const old=h.calls[0];h.enabled=false;h.api.connectionChanged();await flush();assert(old.signal.aborted);assert.equal(h.dots().length,0);
 h.enabled=true;h.api.connectionChanged();await flush();assert.equal(h.calls.length,2);await h.respond({tools:{}});old.resolve({tools:{terminal:{connections:7}}});await flush();assert.equal(h.dots().length,0);h.api.stop();
});
test('focus/visibility refresh coalesces, pagehide clears and pageshow starts a fresh snapshot',async()=>{
 const h=harness();h.enabled=true;h.init();await flush();await h.respond({tools:{terminal:{connections:1}}});await h.event('focus');h.document.visibilityState='visible';h.document.emit('visibilitychange',h.document);await flush();assert.equal(h.calls.length,2);
 await h.event('pagehide');assert.equal(h.dots().length,0);assert(h.calls[1].signal.aborted);await h.event('pageshow');assert.equal(h.calls.length,3);await h.respond({tools:{}});h.api.stop();
});
test('stop before the request microtask does not issue even a read',async()=>{
 const h=harness();h.enabled=true;h.init();h.api.stop();await flush();assert.equal(h.calls.length,0);assert.equal(h.dots().length,0);
});
test('activity CSS overlays the existing icon instead of reserving space or hiding in collapsed navigation',()=>{
 const css=fs.readFileSync(require.resolve('../web/assets/sidebar-activity.css'),'utf8');assert.match(css,/position:absolute/);assert.match(css,/pointer-events:none/);assert.match(css,/forced-colors:active/);assert(!/padding-(?:left|right)|app-nav-collapsed[^}]*display:none/.test(css));
 const nav=fs.readFileSync(require.resolve('../web/assets/app-navigation.js'),'utf8');assert.match(nav,/NetworkToolkitSidebarActivity\?\.sync\(\)/);
});
