'use strict';
// A save of the settings that is refused (another app is saving, the
// settings file cannot be read right now) is tried again while the page is
// open, the message stays until a save works, and closing the app does not
// drop the last changes silently.
const {test}=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const saving=html.slice(html.indexOf('function storageError('),html.indexOf('\nfunction closeApp(){'));
const closing=html.slice(html.indexOf('function closeApp(){'),html.indexOf('\nwindow.__networkToolkitClose='));

function page(answers){
 const timers=[],toasts=[],puts=[];
 const stack={children:[],querySelector(sel){return this.children.find(n=>sel==='[data-save-problem]'&&n.dataset.saveProblem)||null;},append(n){this.children.push(n);n.remove=()=>{this.children.splice(this.children.indexOf(n),1);};}};
 const c={EDITOR_WINDOW:false,DETACHED_TERMINAL:false,OFFLINE:false,online:true,stateLoaded:true,shutDown:false,storageDeleting:false,saveTimer:null,stateRevision:1,savedStateRevision:0,stateWrites:Promise.resolve(true),lastStorageError:'',state:{theme:'dark'},closeRequest:null,nextPoll:null,
  setTimeout(fn,ms){timers.push({fn,ms});return timers.length;},clearTimeout(id){if(id&&timers[id-1])timers[id-1].cancelled=true;},Date,
  $:sel=>sel==='#toastStack'?stack:{innerHTML:''},document:{createElement:()=>({dataset:{},className:'',innerHTML:''})},icon:()=>'',esc:value=>String(value),
  toast(text,kind){toasts.push([kind||'',String(text)]);},
  api:async(path,method)=>{if(path==='/state'&&method==='PUT'){puts.push(c.stateRevision);const next=answers.shift();if(next instanceof Error)throw next;return next||{};}c.calls.push(path);return {detached:false};},
  calls:[],clearWifiSecret(){},closeModal(){},updateEngine(){},
  NetworkToolkitEditorWindows:{prepareMainClose:async()=>{c.calls.push('prepare');return true;},cancelMainClose:async()=>{c.calls.push('restore');return true;},commitMainClose(){c.calls.push('commit');}},
 };
 c.window=c;c.globalThis=c;vm.createContext(c);vm.runInContext(saving+'\n'+closing,c);
 const run=async()=>{const due=timers.filter(t=>!t.cancelled&&!t.ran);for(const t of due){t.ran=true;await t.fn();await c.stateWrites;}return due.map(t=>t.ms);};
 return {c,timers,toasts,puts,stack,run};
}
const refused=()=>Object.assign(new Error('Another Network Toolkit app is saving. Try again.'),{status:409});

test('a refused save is tried again with a growing, bounded wait until it works',async()=>{
 const p=page([refused(),refused(),refused(),refused(),refused(),refused(),refused(),refused(),{}]);
 assert.equal(await p.c.flushState(),false);
 assert.equal(p.stack.children.length,1,'the problem stays on screen');
 assert.match(p.stack.children[0].innerHTML,/not saved yet/);assert.match(p.stack.children[0].innerHTML,/Another Network Toolkit app is saving/);
 const waits=[];
 for(let n=0;n<8;n++){const ran=await p.run();assert.equal(ran.length,1,'one retry is scheduled at a time');waits.push(ran[0]);}
 assert.deepEqual(waits.slice(0,5),[1000,2000,4000,8000,16000]);
 assert(waits.every(ms=>ms<=30000)&&waits.at(-1)===30000,'the wait is bounded: '+waits);
 assert.equal(p.puts.length,9);
 assert.equal(p.c.savedStateRevision,1,'the last attempt saved');
 assert.equal(p.stack.children.length,0,'the message goes away once a save worked');
 assert.deepEqual(await p.run(),[],'nothing is scheduled after a successful save');
 // The next failure starts with the short wait again.
 p.c.stateRevision=2;
 const again=page([refused(),{}]);await again.c.flushState();assert.deepEqual(await again.run(),[1000]);
});
test('a retry never runs after the app has stopped or while app data is deleted',async()=>{
 const p=page([refused(),{}]);
 await p.c.flushState();p.c.shutDown=true;await p.run();
 assert.equal(p.puts.length,1);
 const d=page([refused()]);d.c.storageDeleting=true;await d.c.flushState();
 assert.deepEqual(await d.run(),[]);
});
test('closing tries the save again for a few seconds before the app stops',async()=>{
 const p=page([refused(),refused(),{}]);
 p.c.setTimeout=(fn)=>{Promise.resolve().then(fn);return 1;};
 assert.equal(await p.c.closeApp(),true);
 assert.equal(p.puts.length,3);
 assert.deepEqual(p.c.calls,['prepare','/shutdown','commit']);
 assert.equal(p.c.shutDown,true);
});
test('closing does not stop the app silently when the last changes cannot be saved',async()=>{
 const answers=[];for(let n=0;n<200;n++)answers.push(refused());
 const p=page(answers);let now=0;
 p.c.Date={now:()=>now};p.c.setTimeout=(fn,ms)=>{now+=ms;Promise.resolve().then(fn);return 1;};
 const asked=[];p.c.askUnsavedClose=async()=>{asked.push(p.c.lastStorageError);return asked.length===1?'retry':'cancel';};
 assert.equal(await p.c.closeApp(),false);
 assert.equal(asked.length,2,'the user was asked, chose to try again, and was asked again');
 assert.match(asked[0],/Another Network Toolkit app is saving/);
 assert(p.puts.length>=4&&p.puts.length<60,'each round tries for a few seconds: '+p.puts.length);
 assert.deepEqual(p.c.calls,['prepare','restore'],'no shutdown; the prepared editor close is undone');
 assert.equal(p.c.shutDown,false);assert.equal(p.c.closeRequest,null,'closing can be asked for again');
 // Close anyway: the user's decision, and only then.
 p.c.calls.length=0;p.c.askUnsavedClose=async()=>'close';
 assert.equal(await p.c.closeApp(),true);
 assert.deepEqual(p.c.calls,['prepare','/shutdown','commit']);
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
 const answer=p.c.askUnsavedClose();
 assert.equal(modals[0].title,'Your last changes were not saved');
 assert.match(modals[0].body,/The settings file could not be read\./);assert.match(modals[0].body,/Open documents are not affected/);
 for(const choice of ['retry','close','cancel'])assert(modals[0].footer.includes('data-unsaved-close="'+choice+'"'));
 handler({target:{closest:()=>({dataset:{unsavedClose:'retry'}})}});
 assert.equal(await answer,'retry');
 // Dismissing the dialog any other way keeps the app open.
 root.children=[1];const second=p.c.askUnsavedClose();root.children=[];root.changed();
 assert.equal(await second,'cancel');
});
