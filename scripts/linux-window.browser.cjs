'use strict';
// Linux edition window (v1.21.0, docs/LINUX_PLAN.md): runs the real Linux build of the app.
// A stand-in browser records the app window it is asked to open; Chromium (Playwright) then
// opens that one-time launch link. Synthetic only: no network device, no real desktop.
const fs=require('node:fs'),path=require('node:path'),os=require('node:os'),assert=require('node:assert/strict'),{spawn,execFileSync}=require('node:child_process');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),output=process.env.TOOLKIT_TEST_OUTPUT,report={passed:false,checks:[],errors:[],scope:'Linux build in a hosted runner with a stand-in window launcher; not a physical Linux desktop.'};
if(process.platform!=='linux'){console.log('Linux only; skipped');process.exit(0);}
const work=fs.mkdtempSync(path.join(os.tmpdir(),'ntk-linux-window-')),bin=path.join(work,'network-toolkit');
const until=async(check,ms=15000,what='condition')=>{const end=Date.now()+ms;for(;;){const v=await check();if(v)return v;if(Date.now()>end)throw Error('Timed out waiting for '+what);await new Promise(r=>setTimeout(r,100));}};
function fakeLauncher(dir,name){fs.mkdirSync(dir,{recursive:true});const file=path.join(dir,name);fs.writeFileSync(file,'#!/bin/sh\nprintf "%s\\n" "$@" > "'+path.join(dir,name+'.args')+'"\nwhile [ ! -e "'+path.join(dir,'close')+'" ]; do sleep 0.1; done\n',{mode:0o755});return file;}
function start(env,label){const child=spawn(bin,['--data-dir',path.join(work,label+'-data')],{env:{...process.env,HOME:path.join(work,label+'-home'),...env},stdio:['ignore','pipe','pipe']});let log='';child.stdout.on('data',d=>log+=d);child.stderr.on('data',d=>log+=d);child.exited=new Promise(r=>child.on('exit',code=>r(code)));child.log=()=>log;return child;}
(async()=>{
 if(output)fs.mkdirSync(output,{recursive:true});
 execFileSync('go',['build','-mod=vendor','-o',bin,'./cmd/network-toolkit'],{cwd:root,env:{...process.env,CGO_ENABLED:'0',GOOS:'linux'},stdio:'inherit'});
 const browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 const children=[];
 try{
  // ---- App window mode ----
  const bdir=path.join(work,'browser');const fake=fakeLauncher(bdir,'chromium');
  const app=start({NETWORK_TOOLKIT_BROWSER:fake},'window');children.push(app);
  const args=(await until(()=>fs.existsSync(path.join(bdir,'chromium.args'))&&fs.readFileSync(path.join(bdir,'chromium.args'),'utf8').trim(),15000,'the app window launch')).split('\n');
  const appArg=args.find(a=>a.startsWith('--app='));assert(appArg,'the window opens in app mode: '+args.join(' '));
  assert(args.includes('--class=NetworkToolkit')&&args.some(a=>a.startsWith('--user-data-dir=')),'dedicated profile and window class');
  assert(!args.some(a=>/no-sandbox|disable-web-security/.test(a)),'browser protections stay on');
  const launch=appArg.slice('--app='.length),base=new URL(launch).origin;
  assert.match(launch,/^http:\/\/127\.0\.0\.1:\d+\/launch\?ticket=[0-9a-f]{64}$/,'one-time launch link on loopback');
  report.checks.push('The app opens a dedicated app-mode window through a one-time loopback launch link');
  // Another local reader gets no token.
  const outsider=await fetch(base+'/');const outsiderBody=await outsider.text();
  assert.equal(outsider.status,403);assert.doesNotMatch(outsiderBody,/[0-9a-f]{64}/,'no token for other local readers');
  const context=await browser.newContext({viewport:{width:1280,height:800}}),page=await context.newPage();page.on('pageerror',e=>report.errors.push(e.message));
  await page.goto(launch);await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online,null,{timeout:20000});
  assert.equal(new URL(page.url()).pathname,'/','the launch link redirects to the app');
  const status=await page.evaluate(()=>status);assert.equal(status.os,'linux');assert.equal(status.uiHost,'linux-window');
  assert.equal(await page.locator('#linuxQuitButton').isVisible().catch(()=>false),false,'no Quit button when closing the window stops the app');
  const reuse=await fetch(launch,{redirect:'manual'});assert.equal(reuse.status,403,'a launch link works once');
  report.checks.push('Only the launched window receives the page; other local readers and a reused link are refused');
  // The interface works: a local tool and saving settings.
  await page.evaluate(()=>navigate('ping'));await page.waitForFunction(()=>state.activePage==='ping');
  await page.evaluate(()=>{state.theme='dark';saveState();});await until(async()=>fs.existsSync(path.join(work,'window-data')),5000,'settings folder');
  if(output)await page.screenshot({path:path.join(output,'linux-window.png')});
  report.checks.push('The interface runs on Linux and saves settings in the app-data folder');
  // App settings → Add to applications menu writes the user's own desktop entry and icon.
  await page.locator('#settingsButton').click();await page.locator('#linuxDesktopEntry [data-linux-desktop-entry="add"]').waitFor();
  const desktop=path.join(work,'window-home','.local','share','applications','network-toolkit.desktop'),svg=path.join(work,'window-home','.local','share','icons','hicolor','scalable','apps','network-toolkit.svg');
  await page.locator('#linuxDesktopEntry [data-linux-desktop-entry="add"]').click();await page.waitForFunction(()=>/is in your applications menu/.test(document.querySelector('#linuxDesktopEntryStatus')?.textContent||''));
  const entry=fs.readFileSync(desktop,'utf8');assert.match(entry,/^Exec="[^"]*network-toolkit"$/m);assert.match(entry,/^StartupWMClass=NetworkToolkit$/m);assert(fs.readFileSync(svg,'utf8').includes('<svg'));
  if(output)await page.screenshot({path:path.join(output,'linux-settings-menu.png')});
  await page.locator('#linuxDesktopEntry [data-linux-desktop-entry="add"]').count().then(n=>assert.equal(n,0,'no Add button once the entry is current'));await page.locator('#linuxDesktopEntry [data-linux-desktop-entry="remove"]').click();await page.waitForFunction(()=>/not in your applications menu/.test(document.querySelector('#linuxDesktopEntryStatus')?.textContent||''));
  assert(!fs.existsSync(desktop)&&!fs.existsSync(svg),'removed');await page.keyboard.press('Escape');
  report.checks.push('App settings adds Network Toolkit to the applications menu (user files only) and removes it');
  // Closing the window (its page goes away, then the window process exits) stops the app.
  await context.close();
  fs.writeFileSync(path.join(bdir,'close'),'');const code=await Promise.race([app.exited,new Promise(r=>setTimeout(()=>r('timeout'),15000))]);
  assert.notEqual(code,'timeout','the app stops when its window closes: '+app.log().slice(-400));report.checks.push('Closing the app window stops the app');

  // ---- Browser tab mode (no Chromium-family browser) ----
  const tdir=path.join(work,'tab');const opener=fakeLauncher(tdir,'xdg-open');
  const tab=start({NETWORK_TOOLKIT_BROWSER:'',PATH:tdir+':/usr/bin:/bin'},'tab');children.push(tab);
  const tabLaunch=(await until(()=>fs.existsSync(path.join(tdir,'xdg-open.args'))&&fs.readFileSync(path.join(tdir,'xdg-open.args'),'utf8').trim(),15000,'the browser tab launch')).split('\n')[0];
  assert.match(tabLaunch,/\/launch\?ticket=[0-9a-f]{64}$/);
  const tabContext=await browser.newContext({viewport:{width:1280,height:800}}),tabPage=await tabContext.newPage();tabPage.on('pageerror',e=>report.errors.push(e.message));
  await tabPage.goto(tabLaunch);await tabPage.waitForFunction(()=>window.__networkToolkitStartupState===1&&online,null,{timeout:20000});
  assert.equal(await tabPage.evaluate(()=>status.uiHost),'browser-tab');
  await tabPage.locator('#linuxQuitButton').waitFor({timeout:5000});
  if(output)await tabPage.screenshot({path:path.join(output,'linux-browser-tab.png')});
  await tabPage.locator('#linuxQuitButton').click();
  const tabCode=await Promise.race([tab.exited,new Promise(r=>setTimeout(()=>r('timeout'),15000))]);assert.notEqual(tabCode,'timeout','Quit Network Toolkit stops the app');
  report.checks.push('Without a Chromium-family browser the app opens a tab and Quit Network Toolkit stops it');
  await tabContext.close();
  report.errors=report.errors.filter(e=>!/Failed to fetch|NetworkError|engine/i.test(e));assert.deepEqual(report.errors,[]);report.passed=true;
 }finally{
  for(const c of children){try{c.kill('SIGKILL');}catch{}}
  fs.writeFileSync(path.join(work,'browser','close'),'');
  await browser.close();if(output)fs.writeFileSync(path.join(output,'linux-window-report.json'),JSON.stringify(report,null,2));
 }
 console.log(report.checks.join('\n'));
})().catch(error=>{console.error(error);console.error(JSON.stringify(report.errors));process.exit(1);});
