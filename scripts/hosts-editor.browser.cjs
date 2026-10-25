'use strict';
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),http=require('node:http'),crypto=require('node:crypto');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'../web'),output=process.env.TOOLKIT_TEST_OUTPUT;
const html=fs.readFileSync(path.join(root,'index.html'),'utf8'),version=html.match(/const VERSION='([^']+)'/)[1];
const initial='# Hosts editor fixture\n127.0.0.1 localhost\n::1 localhost\n192.0.2.1 one.test one # office mapping\n# 192.0.2.2 disabled.test # spare\n';
const requests=[],errors=[],cases=[];let server,browser,page,current=initial,operation=null,polls=0,generation=0;
const revision=()=>crypto.createHash('sha256').update(current).digest('hex');
const snapshot=()=>({available:true,editable:true,exists:true,path:'C:\\Windows\\System32\\drivers\\etc\\hosts',content:current,revision:revision(),encoding:'UTF-8',newline:'CRLF',warnings:[]});
const writes=()=>requests.filter(x=>x.path==='/api/local-tools/hosts'&&x.method==='POST');
const settle=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
async function setup(page,text=initial){
 current='# Session '+(++generation)+'\n'+text;operation=null;polls=0;
 await page.evaluate(snapshot=>{online=true;state.activePage='hosts';localConfiguration.hosts={snapshot,draft:snapshot.content,loading:false,attempted:true,error:'',operation:null};renderPage();},snapshot());await settle(page);
 await page.locator('[data-hosts-view="text"]').click();
 if(await page.locator('#hostsFindPanel').isVisible())await page.locator('[data-hosts-action="close-find"]').click();
 await page.locator('[data-hosts-view="entries"]').click();await page.fill('#hostsEntrySearch','');await page.selectOption('#hostsEntryFilter','all');
 if(await page.locator('#hostsIssueButton').getAttribute('aria-expanded')==='true')await page.locator('#hostsIssueButton').click();
}
async function more(page,action){await page.locator('.he-more>summary').click();await page.locator('[data-hosts-action="'+action+'"]').click();}
async function compactMetrics(page,selector){
 return page.locator(selector).evaluate(e=>{const s=getComputedStyle(e),ctx=document.createElement('canvas').getContext('2d');ctx.font=[s.fontStyle,s.fontWeight,s.fontSize,s.fontFamily].join(' ');const select=e.tagName==='SELECT',label=select?e.selectedOptions[0].text:e.value||e.placeholder||'Ag',m=ctx.measureText(label),font=ctx.measureText('Ag'),fontHeight=(font.fontBoundingBoxAscent??font.actualBoundingBoxAscent)+(font.fontBoundingBoxDescent??font.actualBoundingBoxDescent);return {label,contentHeight:e.clientHeight-parseFloat(s.paddingTop)-parseFloat(s.paddingBottom),requiredHeight:Math.max(parseFloat(s.lineHeight)||0,fontHeight),contentWidth:e.clientWidth-parseFloat(s.paddingLeft)-parseFloat(s.paddingRight)-(select?24:0),textWidth:m.width};});
}
async function checkCompactText(page,view){
 if(view==='entries'){
  for(const value of ['all','enabled','disabled','issues']){await page.selectOption('#hostsEntryFilter',value);const m=await compactMetrics(page,'#hostsEntryFilter');assert.ok(m.contentHeight+.25>=m.requiredHeight,'Entry filter clips text vertically: '+JSON.stringify(m));assert.ok(m.contentWidth+.25>=m.textWidth,'Entry filter clips label horizontally: '+JSON.stringify(m));}
  await page.selectOption('#hostsEntryFilter','all');const search=await compactMetrics(page,'#hostsEntrySearch');assert.ok(search.contentHeight+.25>=search.requiredHeight,'Hosts search clips text vertically: '+JSON.stringify(search));
 }else{
  await page.locator('[data-hosts-action="find"]').click();for(const selector of ['#hostsFind','#hostsReplacement']){const m=await compactMetrics(page,selector);assert.ok(m.contentHeight+.25>=m.requiredHeight,'Hosts find/replace clips text vertically: '+JSON.stringify(m));}await page.locator('[data-hosts-action="close-find"]').click();
 }
}
async function main(){
 if(output)fs.mkdirSync(output,{recursive:true});
 server=http.createServer((req,res)=>{
  const pathname=new URL(req.url,'http://localhost').pathname;
  if(pathname.startsWith('/api/')){
   let body='';req.on('data',part=>body+=part);req.on('end',()=>{
    let data={};try{data=JSON.parse(body||'{}');}catch{}
    requests.push({path:pathname,method:req.method,body:data});res.setHeader('Content-Type','application/json');
    const send=value=>res.end(JSON.stringify(value));
    if(pathname==='/api/local-tools/hosts'&&req.method==='GET')return send(snapshot());
    if(pathname==='/api/local-tools/hosts'&&req.method==='POST'){
     if(data.revision!==revision()){res.statusCode=409;return send({error:'The configuration changed. Reload it.',errorCode:'conflict'});}
     current=data.content.replace(/\r\n?/g,'\n');operation={id:'hosts-operation',status:'running',message:'Fixture is applying the reviewed change.'};polls=0;return send({operation});
    }
    if(pathname==='/api/local-tools/operations/hosts-operation'){
     if(++polls>1)operation={...operation,status:'completed',message:'The hosts file was saved.',backupPath:'C:\\Windows\\System32\\drivers\\etc\\hosts.NetworkToolkit-fixture.bak',result:{hosts:snapshot()}};
     return send({operation});
    }
    if(pathname==='/api/status')return send({version,platform:'windows',uiHost:'webview2',tools:[],features:{}});
    if(pathname==='/api/system')return send({platform:'windows',interfaces:[],hostname:'fixture'});
    if(pathname==='/api/jobs')return send({jobs:[]});
    if(pathname==='/api/state')return send({});
    if(pathname.startsWith('/api/updates'))return send({currentVersion:version,latestVersion:version,checked:true,available:false,networkBusy:false});
    return send({});
   });return;
  }
  const name=path.resolve(root,'.'+(pathname==='/'?'/index.html':pathname));
  if(!name.startsWith(root+path.sep)){res.writeHead(403);return res.end();}
  try{res.setHeader('Content-Type',name.endsWith('.js')?'text/javascript':name.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(name));}catch{res.writeHead(404);res.end();}
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
 page=await browser.newPage({viewport:{width:1366,height:900},acceptDownloads:true});page.on('pageerror',error=>errors.push(error.message));
 await page.goto('http://127.0.0.1:'+server.address().port+'/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1);await setup(page);
 // Reproduce the released 67 padding/line-height defect before checking the fix.
 const releasedPadding=await page.addStyleTag({content:'#hostsEntryFilter{padding:9px 11px!important;line-height:1.5!important}'}),clipped=await compactMetrics(page,'#hostsEntryFilter');assert.ok(clipped.contentHeight<clipped.requiredHeight,'Released padding control no longer reproduces clipped text');await releasedPadding.evaluate(e=>e.remove());await checkCompactText(page,'entries');
 assert.equal(await page.locator('#hostsEntryRows tr').count(),4);
 assert.match(await page.locator('#hostsEntryCounts').textContent(),/3 enabled.*1 disabled/);
 assert.match(await page.locator('#hostsIssueButton').textContent(),/0 errors.*0 warnings/);
 assert.equal(await page.locator('#hostsReviewButton').isDisabled(),true);
 const original=await page.evaluate(()=>localConfiguration.hosts.draft);
 await page.locator('[data-hosts-enabled="5"]').uncheck();assert.match(await page.evaluate(()=>localConfiguration.hosts.draft),/# 192\.0\.2\.1 one\.test/);
 await page.locator('[data-hosts-action="undo"]').click();assert.equal(await page.evaluate(()=>localConfiguration.hosts.draft),original);
 await page.locator('[data-hosts-action="redo"]').click();assert.match(await page.evaluate(()=>localConfiguration.hosts.draft),/# 192\.0\.2\.1 one\.test/);
 await page.locator('[data-hosts-action="undo"]').click();
 // Entry edits are draft-only; HTML-looking comments remain inert.
 await page.locator('[data-hosts-action="add"]').click();await page.fill('#hostsEntryAddress','192.0.2.20');await page.fill('#hostsEntryNames','new.test\nnew-alias');await page.fill('#hostsEntryComment','<img src=x onerror="window.hostsInjected=true">');await page.locator('[data-hosts-action="entry-commit"]').click();
 assert.equal(writes().length,0);assert.ok((await page.evaluate(()=>localConfiguration.hosts.draft)).includes('new.test new-alias'));
 assert.equal(await page.evaluate(()=>window.hostsInjected),undefined);assert.equal(await page.locator('#hostsEntries img').count(),0);
 await page.fill('#hostsEntrySearch','new-alias');assert.equal(await page.locator('#hostsEntryRows tr').count(),1);
 await page.locator('#hostsEntryRows [data-hosts-edit]').first().click();await page.fill('#hostsEntryAddress','2001:db8::20');await page.locator('[data-hosts-action="entry-commit"]').click();assert.ok((await page.locator('#hostsEntryRows').textContent()).includes('2001:db8::20'));
 await page.fill('#hostsEntrySearch','');
 // Existing review and administrator-approved operation paths remain in use.
 await page.locator('#hostsReviewButton').click();assert.equal(writes().length,0);await page.locator('[data-local-config-action="apply"]').click();
 await page.waitForFunction(()=>localConfiguration.hosts.operation?.status==='running');
 assert.equal(await page.locator('[data-hosts-action="add"]').isDisabled(),true);
 await page.waitForFunction(()=>localConfiguration.hosts.operation?.status==='completed');assert.equal(writes().length,1);
 assert.deepEqual(Object.keys(writes()[0].body).sort(),['content','revision']);assert.equal(await page.locator('#hostsReviewButton').isDisabled(),true);
 // Reloading cannot silently discard edits.
 await page.locator('[data-hosts-action="add"]').click();await page.fill('#hostsEntryAddress','192.0.2.30');await page.fill('#hostsEntryNames','unsaved.test');await page.locator('[data-hosts-action="entry-commit"]').click();
 const edited=await page.evaluate(()=>localConfiguration.hosts.draft);await page.locator('[data-local-config-action="hosts-reload"]').click();assert.ok(await page.locator('[data-local-config-action="hosts-reload-confirm"]').isVisible());await page.locator('[data-action="close-modal"]').last().click();assert.equal(await page.evaluate(()=>localConfiguration.hosts.draft),edited);
 // Text tools retain literal replacement semantics, undo and invalid-line navigation.
 await page.locator('[data-hosts-view="text"]').click();assert.equal(await page.locator('[data-input-clear-for="hostsSourceEditor"]').count(),0);await page.locator('[data-hosts-action="add"]').click();await page.keyboard.press('F3');assert.equal(await page.locator('#hostsEntryAddress').evaluate(e=>e===document.activeElement),true,'Find shortcut stole focus from entry dialog');await page.locator('[data-hosts-action="cancel-modal"]').click();await page.locator('#hostsSourceEditor').focus();await page.keyboard.press('Control+f');await page.fill('#hostsFind','office');await page.fill('#hostsReplacement','$&');await page.locator('[data-hosts-action="replace-all"]').click();assert.ok((await page.locator('#hostsSourceEditor').inputValue()).includes('# $& mapping'));
 await page.locator('[data-hosts-action="undo"]').click();assert.ok((await page.locator('#hostsSourceEditor').inputValue()).includes('# office mapping'));
 const valid=await page.locator('#hostsSourceEditor').inputValue();await page.fill('#hostsSourceEditor',valid+'invalid mapping\n');await page.waitForTimeout(250);assert.equal(await page.locator('#hostsReviewButton').isDisabled(),true);
 await page.locator('#hostsIssueButton').click();await page.locator('.he-issue.error').first().click();assert.equal(await page.locator('#hostsSourceEditor').evaluate(e=>e.value.slice(e.selectionStart,e.selectionEnd)),'invalid mapping');
 await page.locator('[data-hosts-action="undo"]').click();
 // Cancelled reviews cannot leave Ctrl+S blocked by an old review object.
 await page.locator('#hostsReviewButton').click();await page.locator('[data-action="close-modal"]').last().click();await page.locator('#hostsSourceEditor').focus();await page.keyboard.press('Control+s');assert.ok(await page.locator('[data-local-config-action="apply"]').isVisible());await page.locator('[data-action="close-modal"]').last().click();
 // Restore a UTF-16 backup through an explicit draft import, with no OS write.
 const beforeImport=await page.evaluate(()=>localConfiguration.hosts.draft),beforeWrites=writes().length;
 const backup='# restored backup\r\n192.0.2.40 restored.test\r\n';
 await page.locator('#hostsImportFile').setInputFiles({name:'hosts.backup.bak',mimeType:'application/octet-stream',buffer:Buffer.concat([Buffer.from([255,254]),Buffer.from(backup,'utf16le')])});
 await page.locator('[data-hosts-action="import-commit"]').waitFor();assert.equal(await page.evaluate(()=>localConfiguration.hosts.draft),beforeImport);await page.locator('[data-hosts-action="import-commit"]').click();assert.equal(await page.evaluate(()=>localConfiguration.hosts.draft),backup.replace(/\r\n/g,'\n'));assert.equal(writes().length,beforeWrites);
 await page.locator('[data-hosts-action="undo"]').click();assert.equal(await page.evaluate(()=>localConfiguration.hosts.draft),beforeImport);
 // Export is an explicit browser download of the draft only.
 const downloadPromise=page.waitForEvent('download');await more(page,'export');const downloaded=await downloadPromise;assert.equal(downloaded.suggestedFilename(),'hosts-draft.txt');assert.equal(fs.readFileSync(await downloaded.path(),'utf8').replace(/\r\n/g,'\n'),beforeImport);
 // Large files use a bounded table, search and bulk selection across pages.
 const large='# large fixture\n'+Array.from({length:230},(_,i)=>'192.0.2.'+(i%250+1)+' host-'+i+'.test').join('\n')+'\n';await setup(page,large);
 assert.equal(await page.locator('#hostsEntryRows tr').count(),100);await page.locator('[data-hosts-action="select-matching"]').click();assert.equal(await page.locator('#hostsSelectionCount').textContent(),'230 selected');await page.locator('[data-hosts-action="disable"]').click();assert.match(await page.locator('#hostsEntryCounts').textContent(),/0 enabled.*230 disabled/);await page.locator('[data-hosts-action="undo"]').click();
 await page.locator('[data-hosts-action="next-page"]').click();assert.match(await page.locator('#hostsPageSummary').textContent(),/^101-200/);await page.fill('#hostsEntrySearch','host-229.test');assert.equal(await page.locator('#hostsEntryRows tr').count(),1);
 // Render all primary views at narrow, short and desktop sizes, in both themes.
 for(const viewport of [{width:1920,height:1080},{width:1366,height:768},{width:1024,height:768},{width:768,height:720},{width:390,height:844}]){
  await page.setViewportSize(viewport);
  for(const theme of ['light','dark'])for(const view of ['entries','text']){
   await setup(page);await page.evaluate(theme=>setTheme(theme),theme);await page.locator('[data-hosts-view="'+view+'"]').click();await checkCompactText(page,view);await settle(page);
   const geometry=await page.evaluate(()=>{const panel=document.querySelector('#hostsConfiguration'),work=document.querySelector('#hostsEntries:not([hidden]),#hostsText:not([hidden])'),r=panel.getBoundingClientRect();return {documentWidth:document.documentElement.scrollWidth,width:innerWidth,bottom:r.bottom,height:innerHeight,workHeight:work.getBoundingClientRect().height};});
   assert.ok(geometry.documentWidth<=geometry.width+2,'application horizontal overflow: '+JSON.stringify({viewport,theme,view,geometry}));assert.ok(geometry.bottom<=geometry.height+2,'hosts panel exceeds window height');assert.ok(geometry.workHeight>=95,'file workspace too short');
   await page.locator('.he-more>summary').click();await settle(page);
   const menu=await page.locator('.he-menu').boundingBox(),panel=await page.locator('#hostsConfiguration').boundingBox();
   assert.ok(menu.x>=panel.x-1&&menu.x+menu.width<=panel.x+panel.width+1,'More menu is clipped horizontally');
   assert.ok(menu.y+menu.height<=panel.y+panel.height+1,'More menu is clipped vertically');
   await page.keyboard.press('Escape');assert.equal(await page.locator('.he-more').getAttribute('open'),null);
   cases.push({viewport,theme,view});
   if(output&&((viewport.width===1366&&theme==='dark')||(viewport.width===390&&theme==='light')))await page.screenshot({path:path.join(output,'hosts-'+view+'-'+theme+'-'+viewport.width+'.png')});
  }
 }
 for(const deviceScaleFactor of [1,1.25,1.5,2]){
  const scaled=await browser.newPage({viewport:{width:1366,height:900},deviceScaleFactor});scaled.on('pageerror',error=>errors.push(error.message));
  try{await scaled.goto('http://127.0.0.1:'+server.address().port+'/');await scaled.waitForFunction(()=>window.__networkToolkitStartupState===1);for(const theme of ['light','dark']){await setup(scaled);await scaled.evaluate(theme=>setTheme(theme),theme);await checkCompactText(scaled,'entries');await scaled.locator('[data-hosts-view="text"]').click();await checkCompactText(scaled,'text');if(output){await scaled.locator('[data-hosts-view="entries"]').click();await scaled.selectOption('#hostsEntryFilter','issues');await scaled.locator('#hostsEntryControls').screenshot({path:path.join(output,'hosts-filter-'+theme+'-scale-'+deviceScaleFactor+'.png')});}}}finally{await scaled.close();}
 }
 assert.deepEqual(errors,[]);
 const result={passed:true,renderedCases:cases.length,entryEditing:true,reviewedSave:true,activeOperationLock:true,reloadProtection:true,undoRedo:true,findReplace:true,validationNavigation:true,backupImport:true,explicitExport:true,pagination:true,compactTextFits:true,filterLabels:4,displayScales:[1,1.25,1.5,2],releasedPaddingReproduced:true,errors};
 if(output)fs.writeFileSync(path.join(output,'hosts-editor-results.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
}
main().catch(async error=>{console.error(error);if(output&&page)try{await page.screenshot({path:path.join(output,'hosts-editor-failure.png')});}catch{}process.exitCode=1;}).finally(async()=>{await browser?.close();if(server)await new Promise(resolve=>server.close(resolve));});
