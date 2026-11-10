'use strict';
// A save of the settings that is refused (another app is saving, the
// settings file cannot be read right now) is tried again while the page is
// open, the message stays until a save works, and closing the app does not
// drop the last changes silently.
const {test}=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const saving=html.slice(html.indexOf('function storageError('),html.indexOf('\nfunction closeApp(){'));
const closing=html.slice(html.indexOf('function closeApp(){'),html.indexOf('\nwindow.__networkToolkitClose='));

// page() runs the page's own save and close code. Timers are collected and
// run by run(); the short waits between attempts at close run at once and
// move the clock.
function page(answers){
 const timers=[],toasts=[],puts=[],paths=[];let now=0;
 const stack={children:[],querySelector(sel){return this.children.find(n=>sel==='[data-save-problem]'&&n.dataset.saveProblem)||null;},append(n){this.children.push(n);n.remove=()=>{this.children.splice(this.children.indexOf(n),1);};}};
 const c={EDITOR_WINDOW:false,DETACHED_TERMINAL:false,OFFLINE:false,online:true,stateLoaded:true,shutDown:false,storageDeleting:false,saveTimer:null,stateRevision:1,savedStateRevision:0,stateWrites:Promise.resolve(true),lastStorageError:'',state:{theme:'dark'},closeRequest:null,nextPoll:null,
  setTimeout(fn,ms){if(ms<1000){now+=ms;Promise.resolve().then(fn);return 0;}timers.push({fn,ms});return timers.length;},clearTimeout(id){if(id&&timers[id-1])timers[id-1].cancelled=true;},Date:{now:()=>now},
  $:sel=>sel==='#toastStack'?stack:{innerHTML:''},document:{createElement:()=>({dataset:{},className:'',innerHTML:''})},icon:()=>'',esc:value=>String(value),
  toast(text,kind){toasts.push([kind||'',String(text)]);},
  api:async(path,method)=>{if(path.startsWith('/state')&&method==='PUT'){puts.push(c.stateRevision);paths.push(path);const next=answers.shift();if(next instanceof Error)throw next;return next||{};}c.calls.push(path);return {detached:false};},
  calls:[],clearWifiSecret(){},closeModal(){},updateEngine(){},
  NetworkToolkitEditorWindows:{prepareMainClose:async()=>{c.calls.push('prepare');return true;},cancelMainClose:async()=>{c.calls.push('restore');return true;},commitMainClose(){c.calls.push('commit');}},
 };
 c.window=c;c.globalThis=c;vm.createContext(c);vm.runInContext(saving+'\n'+closing,c);
 const run=async()=>{const due=timers.filter(t=>!t.cancelled&&!t.ran);for(const t of due){t.ran=true;await t.fn();await c.stateWrites;}return due.map(t=>t.ms);};
 return {c,timers,toasts,puts,paths,stack,run,clock:()=>now};
}
const refused=()=>Object.assign(new Error('Another Network Toolkit app is saving. Try again.'),{status:409});
const tooLarge=()=>Object.assign(new Error('Saved settings would exceed the 2 MiB limit. Reduce the saved data first.'),{status:400});
const reload=()=>Object.assign(new Error('Display-name revision is newer than the stored record. Reload settings before saving.'),{status:409,code:'settings-reload-required'});
const many=(make,n=200)=>Array.from({length:n},make);

test('a refused save is tried again with a growing, bounded wait until it works',async()=>{
 const p=page([refused(),refused(),refused(),refused(),refused(),refused(),refused(),refused(),{}]);
 assert.equal(await p.c.flushState(),false);
 assert.equal(p.stack.children.length,1,'the problem stays on screen');
 assert.match(p.stack.children[0].innerHTML,/not saved yet/);assert.match(p.stack.children[0].innerHTML,/Another Network Toolkit app is saving/);assert.match(p.stack.children[0].innerHTML,/keeps trying/);
 const waits=[];
 for(let n=0;n<8;n++){const ran=await p.run();assert.equal(ran.length,1,'one retry is scheduled at a time');waits.push(ran[0]);}
 assert.deepEqual(waits.slice(0,5),[1000,2000,4000,8000,16000]);
 assert(waits.every(ms=>ms<=30000)&&waits.at(-1)===30000,'the wait is bounded: '+waits);
 assert.equal(p.puts.length,9);
 assert.equal(p.c.savedStateRevision,1,'the last attempt saved');
 assert.equal(p.stack.children.length,0,'the message goes away once a save worked');
 assert.deepEqual(await p.run(),[],'nothing is scheduled after a successful save');
 // The next failure starts with the short wait again.
 const again=page([refused(),{}]);await again.c.flushState();assert.deepEqual(await again.run(),[1000]);
});
test('a retry never runs after the app has stopped, and the message never promises a retry that is not coming',async()=>{
 const p=page([refused(),{}]);
 await p.c.flushState();p.c.shutDown=true;await p.run();
 assert.equal(p.puts.length,1);
 // "Delete app data" started (and may fail): nothing is saved meanwhile,
 // so the message goes away instead of promising a retry.
 const d=page([refused(),{}]);await d.c.flushState();d.c.storageDeleting=true;await d.run();
 assert.equal(d.puts.length,1);assert.equal(d.stack.children.length,0,'no "keeps trying" without a retry');
 assert.deepEqual(await d.run(),[]);
 // The engine is not connected, or the settings have not loaded, when the
 // retry is due: it is scheduled again and the message stays true.
 for(const [set,reset] of [[c=>{c.online=false;},c=>{c.online=true;}],[c=>{c.stateLoaded=false;},c=>{c.stateLoaded=true;}]]){
  const o=page([refused(),{}]);await o.c.flushState();set(o.c);
  assert.deepEqual(await o.run(),[1000]);assert.equal(o.puts.length,1);
  assert.equal(o.stack.children.length,1);assert.equal(o.timers.filter(t=>!t.cancelled&&!t.ran).length,1,'the retry is scheduled again');
  reset(o.c);await o.run();
  assert.equal(o.puts.length,2);assert.equal(o.stack.children.length,0);
 }
});
test('a save made for the updater or at close that fails also schedules the retry it announces',async()=>{
 const p=page([refused(),{}]);
 assert.equal(await p.c.flushState({closing:true}),false);
 assert.match(p.stack.children[0].innerHTML,/keeps trying/);
 assert.deepEqual(await p.run(),[1000]);
 assert.equal(p.puts.length,2);assert.equal(p.stack.children.length,0);
});
test('a save that can never succeed is not retried and does not say so',async()=>{
 for(const failure of [tooLarge,reload]){
  const p=page([failure(),failure(),{}]);
  assert.equal(await p.c.flushState(),false);
  assert.deepEqual(await p.run(),[],'no retry is scheduled');
  assert.equal(p.stack.children.length,1);
  assert(!/keeps trying/.test(p.stack.children[0].innerHTML),p.stack.children[0].innerHTML);
  assert.match(p.stack.children[0].innerHTML,/are not saved/);
  // The user's next change is saved as usual.
  p.c.stateRevision=2;assert.equal(await p.c.flushState(),false);p.c.stateRevision=3;assert.equal(await p.c.flushState(),true);
  assert.equal(p.stack.children.length,0);
 }
});
test('closing tries the save again, briefly, before the app stops',async()=>{
 const p=page([refused(),refused(),{}]);
 assert.equal(await p.c.closeApp(),true);
 assert.equal(p.puts.length,3);
 assert(p.paths.every(path=>path==='/state?closing=1'),'the closing save asks the engine not to wait long: '+p.paths);
 assert.deepEqual(p.c.calls,['prepare','/shutdown','commit']);
 assert.equal(p.c.shutDown,true);
 assert.equal(p.toasts.filter(t=>/could not be saved/.test(t[1])).length,1,'one message for the round, not one per attempt');
});
test('closing does not stop the app silently, and asks well inside the host window\'s five seconds',async()=>{
 const p=page(many(refused));
 const asked=[];p.c.askUnsavedClose=async()=>{asked.push(p.clock());return asked.length===1?'retry':'cancel';};
 assert.equal(await p.c.closeApp(),false);
 assert.equal(asked.length,2,'the user was asked, chose to try again, and was asked again');
 assert(asked[0]<=2500,'asked after '+asked[0]+' ms');assert(asked[1]-asked[0]<=2500);
 assert(p.puts.length>=4&&p.puts.length<30,'each round tries a few times: '+p.puts.length);
 assert.equal(p.toasts.filter(t=>/could not be saved/.test(t[1])).length,2,'one message per round: '+p.toasts.length);
 assert.deepEqual(p.c.calls,['prepare','restore'],'no shutdown; the prepared editor close is undone');
 assert.equal(p.c.shutDown,false);assert.equal(p.c.closeRequest,null,'closing can be asked for again');
 assert.equal(p.timers.filter(t=>!t.cancelled&&!t.ran).length,1,'the app stays open and keeps trying');
 // Close anyway: the user's decision, and only then.
 p.c.calls.length=0;p.c.askUnsavedClose=async()=>'close';
 assert.equal(await p.c.closeApp(),true);
 assert.deepEqual(p.c.calls,['prepare','/shutdown','commit']);
});
test('at close a save that can never succeed is tried once and asked about once per session',async()=>{
 const p=page(many(tooLarge));
 const asked=[];p.c.askUnsavedClose=async permanent=>{asked.push(permanent);return 'cancel';};
 assert.equal(await p.c.closeApp(),false);
 assert.equal(p.puts.length,1,'no retries for a save that cannot succeed');
 assert.deepEqual(asked,[true],'the user is told once that these changes are not saved');
 assert.equal(await p.c.closeApp(),true,'the second close is not held up again');
 assert.equal(asked.length,1);assert.equal(p.puts.length,2);
 assert.deepEqual(p.c.calls,['prepare','restore','prepare','/shutdown','commit']);
});
test('closing is not held up when there is nothing that could be saved',async()=>{
 for(const set of [c=>{c.stateLoaded=false;},c=>{c.online=false;}]){
  const p=page([]);set(p.c);p.c.askUnsavedClose=async()=>{throw new Error('must not ask');};
  assert.equal(await p.c.closeApp(),true);
  assert.equal(p.puts.length,0);assert.deepEqual(p.c.calls,['prepare','/shutdown','commit']);
 }
});
test('the question names what happened and offers to try again, close anyway or stay',async()=>{
 const p=page([]);const modals=[];let handler=null;
 const root={children:[1],addEventListener(type,fn){handler=fn;},removeEventListener(){handler=null;}};
 p.c.$=sel=>sel==='#modalRoot'?root:p.stack;p.c.openModal=(title,body,footer)=>{modals.push({title,body,footer});};p.c.closeModal=()=>{root.children=[];};
 p.c.MutationObserver=class{constructor(fn){this.fn=fn;root.changed=fn;}observe(){}disconnect(){root.changed=null;}};
 p.c.lastStorageError='The settings file could not be read.';
 const answer=p.c.askUnsavedClose(false);
 assert.equal(modals[0].title,'Your last changes were not saved');
 assert.match(modals[0].body,/The settings file could not be read\./);assert.match(modals[0].body,/Open documents are not affected/);
 for(const choice of ['retry','close','cancel'])assert(modals[0].footer.includes('data-unsaved-close="'+choice+'"'));
 handler({target:{closest:()=>({dataset:{unsavedClose:'retry'}})}});
 assert.equal(await answer,'retry');
 // Dismissing the dialog any other way keeps the app open.
 root.children=[1];const second=p.c.askUnsavedClose(false);root.children=[];root.changed();
 assert.equal(await second,'cancel');
 // For a save that cannot succeed, trying again is not offered.
 root.children=[1];void p.c.askUnsavedClose(true);
 assert(!modals.at(-1).footer.includes('data-unsaved-close="retry"'));assert.match(modals.at(-1).body,/cannot be saved as they are/);
});
