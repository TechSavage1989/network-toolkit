'use strict';

// Replay the unchanged editor gate twice. Only B's synthetic CSS response omits
// the body's selection rule; neither the source file nor test assertions change.
// This runner records failures as evidence, not as replacement acceptance gates.
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),{spawn}=require('node:child_process');
const root=path.resolve(__dirname,'..'),original=path.join(__dirname,'editor-layout-map.browser.cjs');
const stylesheet=path.join(root,'web/assets/app-interaction.css');
const rule='body{-webkit-user-select:none;user-select:none}';
const variants=['current','body-rule-removed'];
const hash=bytes=>crypto.createHash('sha256').update(bytes).digest('hex');
const read=fs.readFileSync.bind(fs);
const variant=process.env.NTK_EDITOR_SELECTION_DIAGNOSTIC_VARIANT;

function installProbe(){
 const doc=document,events=[],prior=[],counts={};let active=false,startedAt=null,before=null,stopped=false;
 const now=()=>performance.timeOrigin+performance.now();
 function node(value){
  if(!value)return null;
  const element=value.nodeType===1?value:value.parentElement;
  return {type:value.nodeType,tag:element?.tagName,id:element?.id||'',classes:typeof element?.className==='string'?element.className:'',length:value.nodeType===3?value.length:undefined};
 }
 function selection(){
  const s=doc.getSelection();return s?{ranges:s.rangeCount,collapsed:s.isCollapsed,anchor:node(s.anchorNode),anchorOffset:s.anchorOffset,focus:node(s.focusNode),focusOffset:s.focusOffset}:null;
 }
 function ancestry(element){
  const result=[];
  for(let e=element;e;e=e.parentElement){
   const style=getComputedStyle(e),box=e.getBoundingClientRect();
   result.push({tag:e.tagName,id:e.id,classes:typeof e.className==='string'?e.className:'',role:e.getAttribute('role'),contenteditable:e.getAttribute('contenteditable'),isContentEditable:e.isContentEditable,userSelect:style.userSelect,webkitUserSelect:style.webkitUserSelect,display:style.display,visibility:style.visibility,width:box.width,height:box.height});
  }
  return result;
 }
 function state(){
  const editor=doc.querySelector('[data-compare-text="left"]'),line=editor?.querySelector('.cm-line');
  const app=window.NetworkToolkitFileEditor,comparison=app?.comparison,current=app?.documents?.get(comparison?.selection?.left);
  return {at:now(),active:node(doc.activeElement),selection:selection(),ancestry:ancestry(editor),lineAncestry:ancestry(line),documentLength:current?.content?.length,documentLines:current?.content?.split('\n').length,documentPrefix:current?.content?.slice(0,60),mapDisabled:doc.querySelector('[data-compare-map]')?.getAttribute('aria-disabled')};
 }
 function relevant(event){
  if(event.target?.closest?.('[data-compare-text="left"]'))return true;
  const selected=doc.getSelection(),anchor=selected?.anchorNode;
  return !!doc.querySelector('[data-compare-text="left"]')?.contains(anchor)||!!doc.activeElement?.closest?.('[data-compare-text="left"]');
 }
 function record(event){
  if(stopped||!relevant(event))return;
  const item={at:now(),type:event.type,inputType:event.inputType||'',dataLength:typeof event.data==='string'?event.data.length:undefined,isComposing:event.isComposing,active:node(doc.activeElement),selection:selection()};
  if(!active&&event.type==='beforeinput'&&typeof event.data==='string'&&event.data.startsWith('const synthetic0 = 0;')){
   active=true;startedAt=item.at;before=state();
  }
  if(active){counts[event.type]=(counts[event.type]||0)+1;if(events.length<120)events.push(item);}
  else {prior.push(item);if(prior.length>24)prior.shift();}
 }
 const types=['focusin','focusout','beforeinput','input','selectionchange'];
 for(const type of types)doc.addEventListener(type,record,true);
 Object.defineProperty(window,'__ntkEditorSelectionDiagnostic',{value:{
  snapshot(){return {startedAt,before,prior,events,counts,eventLimit:120,after:state()};},
  stop(){stopped=true;for(const type of types)doc.removeEventListener(type,record,true);}
 },configurable:true});
}

function preload(){
 if(!variants.includes(variant))throw Error('Unknown editor selection diagnostic variant');
 const output=process.env.NTK_BROWSER_OUTPUT_DIR;
 if(!output)throw Error('Editor selection diagnostic output is required');
 fs.mkdirSync(output,{recursive:true});
 const css=read(stylesheet,'utf8');
 if(css.split(rule).length!==2)throw Error('The exact body selection rule was not found once');
 const report={diagnostic:true,variant,originalScriptSHA256:hash(read(original)),stylesheetSHA256:hash(css),servedStylesheetSHA256:hash(variant==='current'?css:css.replace(rule,'/* Diagnostic B: body selection rule omitted. */')),stylesheetReads:0,fill:null};
 const save=()=>fs.writeFileSync(path.join(output,'selection-diagnostic.json'),JSON.stringify(report,null,2)+'\n');
 fs.readFileSync=function(file,...options){
  const bytes=read(file,...options);
  if(typeof file==='string'&&path.resolve(file)===stylesheet){
   report.stylesheetReads++;
   if(variant==='body-rule-removed'){
    const text=Buffer.isBuffer(bytes)?bytes.toString('utf8'):bytes;
    if(text.split(rule).length!==2)throw Error('The diagnostic CSS response changed unexpectedly');
    const changed=text.replace(rule,'/* Diagnostic B: body selection rule omitted. */');
    return Buffer.isBuffer(bytes)?Buffer.from(changed):changed;
   }
  }
  return bytes;
 };
 const playwright=require(process.env.PLAYWRIGHT_MODULE||'playwright');
 const launch=playwright.chromium.launch;
 playwright.chromium.launch=async function(...args){
  const browser=await launch.apply(this,args),newContext=browser.newContext;
  browser.newContext=async function(...contextArgs){
   const context=await newContext.apply(this,contextArgs);
   await context.addInitScript(installProbe);
   const newPage=context.newPage;
   context.newPage=async function(...pageArgs){
    const page=await newPage.apply(this,pageArgs),locator=page.locator;
    page.locator=function(selector,...locatorArgs){
     const result=locator.call(this,selector,...locatorArgs);
     if(selector==='[data-compare-text="left"]'){
      const fill=result.fill;
      result.fill=async function(value,...fillArgs){
       if(report.fill||typeof value!=='string'||!value.startsWith('const synthetic0 = 0;\n')||value.split('\n').length!==600)return fill.call(this,value,...fillArgs);
       report.fill={startedAt:Date.now(),textLength:value.length,textSHA256:hash(value),status:'running'};
       // Start the exact original action immediately. There is no diagnostic
       // await, focus, selection adjustment or changed timeout before it.
       const action=fill.call(this,value,...fillArgs);
       try{const answer=await action;report.fill.status='completed';return answer;}
       catch(error){report.fill.status='failed';report.fill.error=error.stack||String(error);throw error;}
       finally{
        report.fill.finishedAt=Date.now();
        report.fill.elapsedMs=report.fill.finishedAt-report.fill.startedAt;
        try{report.fill.trace=await page.evaluate(()=>{const probe=window.__ntkEditorSelectionDiagnostic;const snapshot=probe?.snapshot();probe?.stop();return snapshot;});}
        catch(error){report.fill.traceError=String(error);}
        save();
       }
      };
     }
     return result;
    };
    return page;
   };
   return context;
  };
  return browser;
 };
 save();process.on('exit',save);
}

async function run(){
 const base=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT;
 if(!base)throw Error('Set NTK_BROWSER_OUTPUT_DIR for the editor selection diagnostic');
 const output=path.join(path.resolve(base),'editor-selection-diagnostic');fs.mkdirSync(output,{recursive:true});
 const originalHash=hash(read(original)),cssHash=hash(read(stylesheet));
 const report={diagnostic:true,purpose:'A/B evidence only. The unchanged editor-layout-map test remains the acceptance gate.',originalScriptSHA256:originalHash,stylesheetSHA256:cssHash,cases:[],status:'running'};
 const save=()=>fs.writeFileSync(path.join(output,'ab-results.json'),JSON.stringify(report,null,2)+'\n');save();
 for(const item of variants){
  const destination=path.join(output,item);fs.mkdirSync(destination,{recursive:true});
  const stdout=fs.createWriteStream(path.join(destination,'stdout.log')),stderr=fs.createWriteStream(path.join(destination,'stderr.log'));
  const result={variant:item,startedAt:new Date().toISOString(),status:'running'};report.cases.push(result);save();
  const child=spawn(process.execPath,['--require',__filename,original],{cwd:root,env:{...process.env,NTK_EDITOR_SELECTION_DIAGNOSTIC_VARIANT:item,NTK_BROWSER_OUTPUT_DIR:destination,TOOLKIT_TEST_OUTPUT:destination},stdio:['ignore','pipe','pipe']});
  child.stdout.pipe(stdout);child.stderr.pipe(stderr);
  await new Promise(resolve=>{child.once('error',error=>{result.spawnError=String(error);});child.once('close',(code,signal)=>{result.exitCode=code;result.signal=signal;resolve();});});
  result.finishedAt=new Date().toISOString();result.status='completed';
  for(const [name,key] of [['editor-layout-map-results.json','test'],['selection-diagnostic.json','selection']]){
   try{const value=JSON.parse(read(path.join(destination,name),'utf8'));result[key]=key==='test'?{passed:value.passed,checks:value.checks?.length,failure:value.failure?.split('\n')[0]}:{fillStatus:value.fill?.status||'not-reached',elapsedMs:value.fill?.elapsedMs,stylesheetReads:value.stylesheetReads,traceCaptured:!!value.fill?.trace};}
   catch(error){result[key+'ReadError']=String(error);}
  }
  if(hash(read(original))!==originalHash||hash(read(stylesheet))!==cssHash)throw Error('Diagnostic changed a source file');
  save();console.log(JSON.stringify(result));
 }
 report.status='completed';
 report.interpretation=report.cases.some(item=>item.spawnError||item.testReadError||item.selectionReadError)?'incomplete':report.cases.some(item=>item.selection.fillStatus==='not-reached')?'fill-not-reached':report.cases[0].selection.fillStatus==='failed'&&report.cases[1].selection.fillStatus==='completed'?'body-rule-dependent-fill-failure':report.cases[0].selection.fillStatus===report.cases[1].selection.fillStatus?'same-fill-outcome':'different-fill-outcomes';
 save();console.log(JSON.stringify({diagnostic:true,status:report.status,interpretation:report.interpretation,output}));
 // An observed test failure is data, not an approval. Infrastructure failures
 // still fail this diagnostic step; the original gate's exit is untouched.
 if(report.interpretation==='incomplete')process.exitCode=1;
}

if(variant)preload();
else if(require.main===module)run().catch(error=>{console.error(error.stack||error);process.exitCode=1;});
