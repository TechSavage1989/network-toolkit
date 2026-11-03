'use strict';
// Owner request (v1.20.3): Save As opens the Windows Save As dialog for local
// files and a server folder picker for SFTP files. Source-only validation with
// mocked endpoints: the dialog endpoint is a route stub, the SFTP listing is a
// synthetic tree. No Go executable, real shell or device.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT;
const report={passed:false,checks:[],errors:[]};let browser,page;
const check=name=>{report.checks.push(name);console.log('PASS '+name);};
const settle=()=>page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
let dialogMode='path',dialogCalls=[];
(async()=>{try{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});const context=await browser.newContext({viewport:{width:1440,height:1000}});page=await context.newPage();page.setDefaultTimeout(12000);page.on('pageerror',e=>report.errors.push(e.message));
 await context.addInitScript(()=>{window.confirm=window.prompt=()=>{throw Error('Native script dialogs are disabled in the desktop host');};});
 await context.route('**/*',async route=>{const req=route.request(),url=new URL(req.url());const p=url.pathname;let data={};
  if(p.startsWith('/api/')){
   if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',localShellAvailable:true};
   if(p==='/api/state'&&req.method()==='GET')data={appUpdatePreferences:{enabled:false}};
   if(p==='/api/jobs')data=[];if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};if(p==='/api/terminal-views')data={available:false,owners:{},views:[]};
   if(p.includes('/events')){await new Promise(r=>setTimeout(r,100));data={events:[],sessions:[],after:0};}
   if(p==='/api/file-editor/save-dialog'){dialogCalls.push(req.postDataJSON());
    if(dialogMode==='unavailable')return route.fulfill({status:400,contentType:'application/json',body:JSON.stringify({error:'The Windows Save As dialog is available only in the Windows application'})});
    if(dialogMode==='busy')return route.fulfill({status:409,contentType:'application/json',body:JSON.stringify({error:'A Save As dialog is already open. Finish or cancel it first.'})});
    data=dialogMode==='cancel'?{cancelled:true}:{path:'C:\\Chosen\\by-explorer.txt'};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});}
  try{let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','save-as').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto('https://save-as.toolkit.test/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('editor'));
 const action=name=>page.locator('[data-ed="'+name+'"]');
 async function openLocal(id){await page.evaluate(id=>NetworkToolkitFileEditor.open({id,path:'C:\\Work\\'+id+'.txt',source:'Local computer',binding:{kind:'local'},create:true,initialContent:'hello '+id,read:async()=>({content:'hello '+id}),write:async data=>{(globalThis.__writes=globalThis.__writes||[]).push(data);return {hash:'h-'+data.path};}}),id);await page.waitForFunction(()=>!!NetworkToolkitFileEditor.active);}
 const writes=()=>page.evaluate(()=>globalThis.__writes||[]);

 // 1. Local document: the Windows dialog chooses the path; its overwrite prompt is the consent.
 await openLocal('local-one');await action('saveAs').click();await page.waitForFunction(()=>(globalThis.__writes||[]).length===1);
 let w=await writes();assert.equal(w[0].path,'C:\\Chosen\\by-explorer.txt');assert.equal(w[0].overwrite,true);assert.equal(w[0].expectedHash,'');
 assert.deepEqual(dialogCalls[0],{suggested:'local-one.txt',directory:'C:\\Work'});
 assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.path),'C:\\Chosen\\by-explorer.txt');assert.equal(await page.locator('[data-ed-dialog-input]').count(),0,'No typed-path prompt when the dialog is available');
 check('Local Save As uses the native dialog, saves to the chosen path with overwrite consent and adopts the new path');

 // 2. Cancelled dialog: nothing is written and the document keeps its path.
 dialogMode='cancel';await openLocal('local-two');await action('saveAs').click();await settle();await page.waitForTimeout(150);
 assert.equal((await writes()).length,1);assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.path),'C:\\Work\\local-two.txt');assert.equal(await page.locator('[data-ed-dialog-input]').count(),0);
 check('A cancelled native dialog writes nothing and shows no prompt');

 // 3. Busy dialog: a message, no write, no prompt.
 dialogMode='busy';await action('saveAs').click();await settle();await page.waitForTimeout(150);assert.equal((await writes()).length,1);assert.equal(await page.locator('[data-ed-dialog-input]').count(),0);
 check('A busy native dialog is reported without a fallback prompt');

 // 4. Dialog unavailable (browser mode or another OS): the typed-path prompt still works, without overwrite consent.
 dialogMode='unavailable';await action('saveAs').click();await page.locator('[data-ed-dialog-input]').waitFor();await page.locator('[data-ed-dialog-input]').fill('C:\\Typed\\fallback.txt');await page.locator('[data-ed-dialog-action="accept"]').click();await page.waitForFunction(()=>(globalThis.__writes||[]).length===2);
 w=await writes();assert.equal(w[1].path,'C:\\Typed\\fallback.txt');assert.equal(w[1].overwrite,undefined);
 check('Without a native dialog the typed-path prompt remains and never grants overwrite');

 // 5. SFTP document: the folder picker browses the server tree, confirms a replacement, and writes with overwrite.
 await page.evaluate(()=>{const tree={'/home/admin':{parent:'/home',entries:[{name:'configs',path:'/home/admin/configs',directory:true},{name:'notes.txt',path:'/home/admin/notes.txt',directory:false}]},'/home/admin/configs':{parent:'/home/admin',entries:[{name:'switch.cfg',path:'/home/admin/configs/switch.cfg',directory:false}]},'/home':{parent:'/',entries:[{name:'admin',path:'/home/admin',directory:true}]}};globalThis.__lists=[];
  return NetworkToolkitFileEditor.open({id:'sftp:remote:one:/home/admin/notes.txt',path:'/home/admin/notes.txt',source:'SSH files',binding:{kind:'sshConnection',connectionId:'c1'},create:true,initialContent:'remote text',read:async()=>({content:'remote text'}),write:async data=>{(globalThis.__writes=globalThis.__writes||[]).push(data);return {hash:'r-'+data.path};},list:async p=>{globalThis.__lists.push(p);const node=tree[p];if(!node)throw Error('No such folder');return {path:p,parent:node.parent,entries:node.entries};}});});
 await page.waitForFunction(()=>NetworkToolkitFileEditor.active.path==='/home/admin/notes.txt');await action('saveAs').click();
 const picker=page.locator('[data-ed-pick-list]');await picker.waitFor();await page.waitForFunction(()=>document.querySelector('[data-ed-pick-path]')?.textContent==='/home/admin');
 assert.equal(await page.locator('[data-ed-pick-dir]').count(),1,'Only folders are listed');assert.equal(await page.locator('[data-ed-pick-name]').inputValue(),'notes.txt');
 await page.locator('[data-ed-pick-dir="/home/admin/configs"]').click();await page.waitForFunction(()=>document.querySelector('[data-ed-pick-path]')?.textContent==='/home/admin/configs');
 await page.locator('[data-ed-pick-name]').fill('switch.cfg');await page.locator('[data-ed-pick-action="save"]').click();
 await page.locator('[data-ed-dialog-action="accept"]').waitFor();assert.match(await page.locator('.nt-editor-dialog-card h3').last().innerText(),/Replace file/);
 await page.locator('[data-ed-dialog-action="cancel"]').click();await settle();assert.equal(await picker.count(),1,'Declining the replacement keeps the picker open');assert.equal((await writes()).length,2);
 await page.locator('[data-ed-pick-action="save"]').click();await page.locator('[data-ed-dialog-action="accept"]').waitFor();await page.locator('[data-ed-dialog-action="accept"]').click();
 await page.waitForFunction(()=>(globalThis.__writes||[]).length===3);w=await writes();assert.equal(w[2].path,'/home/admin/configs/switch.cfg');assert.equal(w[2].overwrite,true);
 assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.path),'/home/admin/configs/switch.cfg');assert.equal(await picker.count(),0);
 check('SFTP Save As browses server folders, asks before replacing an existing file and saves with overwrite consent');

 // 6. SFTP picker: Up navigates to the parent, a new name saves without any confirmation, Escape cancels.
 await action('saveAs').click();await picker.waitFor();await page.waitForFunction(()=>document.querySelector('[data-ed-pick-path]')?.textContent==='/home/admin/configs');
 await page.locator('[data-ed-pick-action="up"]').click();await page.waitForFunction(()=>document.querySelector('[data-ed-pick-path]')?.textContent==='/home/admin');
 await page.locator('[data-ed-pick-name]').fill('brand-new.cfg');await page.locator('[data-ed-pick-name]').press('Enter');await page.waitForFunction(()=>(globalThis.__writes||[]).length===4);
 w=await writes();assert.equal(w[3].path,'/home/admin/brand-new.cfg');assert.equal(w[3].overwrite,undefined);
 await action('saveAs').click();await picker.waitFor();await page.keyboard.press('Escape');await settle();assert.equal(await picker.count(),0);assert.equal((await writes()).length,4);
 check('The picker navigates up, saves a new name on Enter without a prompt and cancels on Escape');
 if(out)await page.screenshot({path:path.join(out,'editor-save-as.png')});
 report.passed=true;
}catch(e){report.errors.push(e.stack||e.message);console.error(e.stack||e.message);if(page&&out)await page.screenshot({path:path.join(out,'editor-save-as-failure.png')}).catch(()=>{});process.exitCode=1;}
finally{if(out)fs.writeFileSync(path.join(out,'editor-save-as-report.json'),JSON.stringify(report,null,2));await browser?.close();}})();
