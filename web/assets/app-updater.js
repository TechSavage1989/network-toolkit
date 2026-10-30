(function(root){
  'use strict';
  const RELEASES='https://github.com/TechSavage1989/network-toolkit/releases/latest';
  const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const notesView=typeof module==='object'&&module.exports?require('./app-update-notes.js'):root.NetworkToolkitUpdateNotes;
  function releaseNotes(data,version){return notesView.html({version,currentVersion:data?.currentVersion,notes:data?.latest?.version===version?data.latest.notes:''});}
  function editorInstallBlockReason(editor=root.NetworkToolkitFileEditor,windows=root.NetworkToolkitEditorWindows){
    const state=windows?.state;
    return editor?.documents?.size||editor?.windowAdapter?.hasPendingWindowClose?.()||state?.closing||state?.outgoing||state?.incoming?'Save your work and close all Text Editor documents and editor windows before installing. Finish or cancel any pending editor move or close; installation was not started.':'';
  }
  function createController(env){
    let data=null,busy=false,error='',message='',backgroundError='',revision=0,checkAfter=0,progress=null;
    const changed=()=>env.changed?.(snapshot());
    const snapshot=()=>({data,busy,error,message,backgroundError,checkAfter,progress});
    const render=()=>env.render(snapshot());
    function watchProgress(turn){
      let active=true,timer=null;
      const schedule=()=>{if(active&&env.setTimeout)timer=env.setTimeout(async()=>{
        timer=null;if(!active||turn!==revision||!busy)return;
        try{const status=await env.api('/updates/status');if(active&&turn===revision&&busy){if(status.progress){progress=status.progress;data={...data,progress};render();}}}catch{}
        if(active&&turn===revision&&busy)schedule();
      },400);};
      schedule();return ()=>{active=false;if(timer!==null)env.clearTimeout?.(timer);};
    }
    async function load(background=false){
      if(busy)return 'skip';const turn=++revision;busy=true;if(!background)error='';render();
      let outcome='retry';
      try{const result=await env.api('/updates/status');if(turn===revision){data=result;checkAfter=Number(result.checkAfter)||0;outcome='ok';}}
      catch(e){if(turn===revision){if(background)backgroundError='Automatic update status could not be read: '+e.message;else error='Could not read update status: '+e.message;}}
      finally{if(turn===revision){busy=false;render();}}
      return outcome;
    }
    async function importPackage(file,checksums){
      if(busy)return;
      if(!file||!checksums){error='Choose both the portable ZIP and SHA256SUMS from the same official release.';render();return;}
      if(!/\.zip$/i.test(file.name)||file.size>128*1024*1024||checksums.size>64*1024){error='Choose a portable ZIP under 128 MiB and a SHA256SUMS file under 64 KiB.';render();return;}
      busy=true;error='';message='Uploading and verifying the update. The installed app is unchanged.';render();
      try{data=await env.upload(file,checksums);if(!data.staged)throw new Error('No staged package was returned. Refresh update status before retrying.');message='Update verified and staged. Ready to install when you choose.';}
      catch(e){error='Update was not imported: '+e.message;message='';}
      finally{busy=false;render();changed();}
    }
    async function check(background=false){
      if(busy||data?.installing)return 'skip';
      if(checkAfter>(env.now?.()??Date.now())){if(!background){error='Update checks are paused until '+(globalThis.NetworkToolkitResultTime?.format(new Date(checkAfter),{milliseconds:false})||new Date(checkAfter).toLocaleString(undefined,{hour12:true}))+'. Please try again then.';render();}return 'retry';}
      busy=true;if(!background){error='';message='Checking the official release feed…';}render();
      let outcome='retry';
      try{data=await env.api('/updates/check','POST',{});checkAfter=Number(data.checkAfter)||0;backgroundError='';if(!background){message=data.hasUpdate?'Update available: '+data.latest.version+'.':'No newer update was found at the last successful check.';env.reveal?.();}outcome='ok';}
      catch(e){checkAfter=Math.max(checkAfter,Number(e.checkAfter)||0);if(background){backgroundError='Automatic update check could not complete: '+e.message;message='';}else{error='Could not check for updates: '+e.message;message='';}}
      finally{busy=false;render();changed();}
      return outcome;
    }
    async function download(version){
      if(busy||data?.installing)return;
      if(!data?.latest||data.latest.version!==version){error='The available release changed. Check again before downloading.';render();return;}
      const turn=++revision;busy=true;error='';progress={phase:'checksums',version};message='Downloading and verifying the update. The installed app is unchanged.';render();const stop=watchProgress(turn);
      try{data=await env.api('/updates/download','POST',{version});if(!data.staged)throw new Error('No staged package was returned. Refresh update status before retrying.');progress={phase:'ready',version};message='Update downloaded, verified and staged. Ready to install when you choose.';}
      catch(e){progress=null;error='The update could not be downloaded: '+e.message;message='';}
      finally{stop();busy=false;render();changed();}
    }
    async function install(version){
      const guardEditor=()=>{const reason=env.editorInstallBlockReason?.();if(reason)throw new Error(reason);};
      if(busy||data?.installing)return;
      if(!data?.staged||data.staged.version!==version){error='The staged update changed. Refresh update status and try again.';render();return;}
      try{guardEditor();}catch(e){error=e.message;message='';render();return;}
      busy=true;error='';progress={phase:'saving',version};message='Saving settings before restart…';render();
      try{
        if(!await env.save())throw new Error('Settings could not be saved. Installation was not started.');
        guardEditor();
        const current=await env.api('/updates/status');data=current;guardEditor();
        if(!current.staged||current.staged.version!==version)throw new Error('The staged update changed. Refresh update status and try again.');
        if(current.busy?.length)throw new Error('Finish or stop active operations first: '+current.busy.join(', '));
        progress={phase:'installing',version};message='Requesting installation. Do not close the app until it restarts.';render();
        guardEditor();const result=await env.api('/updates/install','POST',{confirm:true,version});
        if(!result?.restarting)throw new Error('The app did not confirm a restart. Refresh status before trying again.');
        data={...data,installing:true};progress={phase:'restarting',version};message='Installation requested. The app will close and restart. Settings are kept.';
        env.restarting();
      }catch(e){progress=null;error=e.message;message='';}
      finally{busy=false;render();changed();}
    }
    return {snapshot,load,importPackage,check,download,install};
  }
  function progressHTML(state){
    const p=state.progress;if(!p)return '';
    const phases={checksums:'Preparing download',download:'Downloading update',verifying:'Verifying update',staging:'Staging update',ready:'Update verified and ready',saving:'Saving settings',installing:'Preparing installation',restarting:'Installing and restarting'};
    if(!phases[p.phase])return '';
    const received=Math.max(0,Number(p.received)||0),total=Math.max(0,Number(p.total)||0),known=p.phase==='download'&&total>0&&received<=total;
    const bytes=n=>n<1048576?(n/1024).toFixed(0)+' KiB':(n/1048576).toFixed(1)+' MiB';
    const detail=p.phase==='download'?(known?bytes(received)+' of '+bytes(total)+' ('+Math.floor(received/total*100)+'%)':bytes(received)+' received'):'',label=phases[p.phase];
    return '<div class="app-update-progress"><div class="row between gap6"><span>'+escape(label)+'</span>'+(detail?'<span>'+escape(detail)+'</span>':'')+'</div><progress aria-label="'+escape(label)+'" '+(p.phase==='ready'?'value="1" max="1"':known?'value="'+received+'" max="'+total+'"':'')+'></progress>'+(p.phase==='restarting'?'<small>The installer keeps a progress window open until the updated app starts.</small>':'')+'</div>';
  }
  function html(state,available,currentVersion){
    const {data,busy,error,message,backgroundError}=state,disabled=!available||busy||data?.installing||data?.supported===false,staged=data?.staged;
    return '<h3>Network Toolkit updates</h3><p class="help-line">Installed version: '+escape(data?.currentVersion||currentVersion)+'</p>'+
      (data?.executable?'<p class="help-line" style="overflow-wrap:anywhere">Running file: '+escape(data.executable.currentPath)+'</p>'+(data.executable.canonicalPath&&data.executable.canonicalPath!==data.executable.currentPath?'<p class="help-line" style="overflow-wrap:anywhere">Stable executable: '+escape(data.executable.canonicalPath)+'. Pinned shortcuts, shared shortcuts, and shortcuts elsewhere may need to be pointed to Network_Toolkit.exe manually.</p>':'')+(data.executable.warning?'<div class="notice warn">'+escape(data.executable.warning)+'</div>':''):'')+
      '<p class="help-line">Check the official public binary-release feed. Downloads are verified before installation; the public main repository provides a binary-only update package. Nothing installs until you confirm.</p>'+
      '<div class="row gap6 wrap"><button type="button" class="btn small primary" data-app-update="check" '+(disabled?'disabled':'')+'>Check for updates</button><button type="button" class="btn small ghost" data-app-update="releases" '+(disabled?'disabled':'')+'>Open release notes in browser</button></div>'+
      (data?.latest?'<div class="notice info mt">Latest release: '+escape(data.latest.version)+'</div>'+releaseNotes(data,data.latest.version)+(data.hasUpdate===true?'<button type="button" class="btn small" data-app-update="download" data-version="'+escape(data.latest.version)+'" '+(disabled?'disabled':'')+'>Download update</button>':'<p class="help-line">'+'No newer update was found at the last successful check.'+'</p>'):'')+
      '<details class="mt"><summary>Import a downloaded update instead</summary><p class="help-line">Select the binary-only Update.zip and SHA256SUMS from the same official release. No GitHub credentials are needed.</p><p class="help-line" style="overflow-wrap:anywhere">Official feed: '+RELEASES+'</p>'+
      (!available?'<p class="help-line">Open the executable to import and install updates.</p>':data?.supported===false?'<p class="help-line">In-app installation is supported only by the Windows x64 executable.</p>':'<div class="form-stack mt"><div class="field"><label for="appUpdatePackage">Official binary-only Update ZIP</label><input type="file" id="appUpdatePackage" accept=".zip" '+(disabled?'disabled':'')+'></div><div class="field"><label for="appUpdateChecksums">SHA256SUMS from the same release</label><input type="file" id="appUpdateChecksums" '+(disabled?'disabled':'')+'></div><div><button type="button" class="btn small" data-app-update="import" '+(disabled?'disabled':'')+'>Verify and stage update</button></div></div>')+
      '</details><p class="help-line">SHA-256 checks detect corruption, not publisher identity. Use files only from the official release page. Only the app executable is replaced; your app data folder is not changed.</p>'+
      (staged?'<div class="notice info mt"><span>Ready to install version '+escape(staged.version)+'. Save your work before restarting.</span></div><p class="help-line" style="overflow-wrap:anywhere">Executable SHA-256: '+escape(staged.sha256)+'</p><button type="button" class="btn primary" data-app-update="confirm" data-version="'+escape(staged.version)+'" '+(disabled?'disabled':'')+'>Install and restart</button>':'')+
      (data?.busy?.length?'<p class="help-line">Active operations: '+escape(data.busy.join(', '))+'. Finish or stop them before installing.</p>':'')+
      progressHTML(state)+'<div role="status" aria-live="polite">'+(message?'<p class="help-line">'+escape(message)+'</p>':'')+(busy&&!message?'<p class="help-line">Reading update status…</p>':'')+(backgroundError?'<p class="help-line">'+escape(backgroundError)+' Use Check for updates to retry.</p>':'')+(error?'<div class="notice warn mt">'+escape(error)+'</div>':'')+'</div>';
  }
  // Timers are independent of navigation. No timer may download, install, or restart.
  function createScheduler(env){
    let timer=null,started=false,generation=0,failures=0,lastContact=null;
    let lastAttempt=0,lastResult='',lastSummary='',nextAt=0,nextKind='',running=false;
    const normal=5*60*1000,maximumBackoff=6*60*60*1000,now=()=>env.now?.()??Date.now();
    const active=()=>env.active?.()!==false,notBefore=()=>Math.max(0,Number(env.notBefore?.())||0);
    const snapshot=()=>({lastAttempt,lastResult,lastSummary,nextAt,nextKind,running,failures});
    const changed=()=>env.changed?.(snapshot());
    // Brief startup/network interruptions get two early retries, then bounded backoff.
    const retryDelay=()=>failures===1?30000:failures===2?120000:Math.min(maximumBackoff,15*60*1000*2**Math.min(failures-3,5));
    function schedule(delay,epoch,kind){
      delay=Math.min(2147483647,Math.max(delay,notBefore()-now()));
      nextAt=now()+delay;nextKind=kind;
      timer=env.setTimeout(async()=>{
        timer=null;if(!started||epoch!==generation)return;
        nextAt=0;nextKind='';
        let result='skip';
        if(env.enabled()&&env.ready()&&active()&&now()>=notBefore()){
          lastAttempt=lastContact=now();running=true;changed();
          try{result=await env.check();}catch{result='retry';}
          if(!started||epoch!==generation)return;
          running=false;lastResult=result;lastSummary=env.describeResult?.(result)||'';
        }
        if(!started||epoch!==generation)return;
        if(result==='ok')failures=0;else if(result==='retry')failures++;
        const enabled=env.enabled();
        schedule(!enabled?maximumBackoff:!active()?normal:result==='skip'?15000:result==='retry'?retryDelay():normal,epoch,!enabled?'disabled':result==='skip'?'waiting':result==='retry'?'retry':'check');
      },delay);
      changed();
    }
    function stop(){started=false;generation++;if(timer!==null)env.clearTimeout(timer);timer=null;nextAt=0;nextKind='';running=false;changed();}
    return {
      snapshot,
      start(){if(started)return;started=true;schedule(0,++generation,'check');},
      stop,
      reset(){stop();failures=0;this.start();},
      wake(){
        // Focus, visibility and reconnect share the timer, freshness and retry gates.
        if(!started||running||!env.enabled()||!env.ready()||!active()||now()<notBefore()||nextKind==='retry')return;
        if(lastContact!==null&&now()-lastContact<120000)return;
        if(timer!==null)env.clearTimeout(timer);
        timer=null;schedule(0,generation,'check');
      },
      checked(result){
        if(result==='skip')return;
        // A manual check replaces a pending automatic check, rather than duplicating it.
        stop();started=true;lastContact=now();if(result==='ok')failures=0;else failures++;
        schedule(!env.enabled()?maximumBackoff:result==='ok'?normal:retryDelay(),generation,!env.enabled()?'disabled':result==='ok'?'check':'retry');
      }
    };
  }
  function notice(state,preferences={},now=Date.now()){
    const data=state.data;if(!data||data.supported===false||data.installing)return null;
    const staged=data.staged,version=staged?.version||(data.hasUpdate?data.latest?.version:null);
    if(!version)return null;
    const kind=staged?'install':'download',key=kind+':'+version;
    if(preferences.dismissed===key&&Number(preferences.until)>now)return null;
    return {kind,version,key};
  }
  function noticeHTML(state,preferences,now){
    const item=notice(state,preferences,now);if(!item)return '';
    return '<div class="notice info row wrap gap6" role="status" aria-live="polite"><span style="flex:1;min-width:180px">Network Toolkit '+escape(item.version)+(item.kind==='install'?' is downloaded and verified. Save your work before restarting.':' is available. See what’s new before updating.')+'</span><button type="button" class="btn small primary" data-app-update="'+(item.kind==='install'?'confirm':'download')+'" data-version="'+escape(item.version)+'" '+(state.busy?'disabled':'')+'>'+(item.kind==='install'?'Install and restart':"See what's new")+'</button><button type="button" class="btn small ghost" data-app-update="later">Remind me tomorrow</button>'+(state.error?'<span style="flex-basis:100%" role="alert">'+escape(state.error)+'</span>':'')+(state.busy&&state.message?'<span style="flex-basis:100%">'+escape(state.message)+'</span>':'')+(state.busy?progressHTML(state):'')+'</div>';
  }
  if(typeof module==='object'&&module.exports)module.exports={createController,createScheduler,notice,noticeHTML,progressHTML,html,editorInstallBlockReason};
  if(!root.document)return;
  let controller,scheduler,updateWelcomeShown=false,namingWarningShown=false;
  let preferencesLoaded=true,preferencesPending=false,preferencesError='',preferenceWritesPending=false,pendingManualResult='';
  const preferenceChanges={};
  const preferences=()=>state.appUpdatePreferences||{};
  function automaticStatusText(){
    const info=scheduler?.snapshot(),stamp=value=>globalThis.NetworkToolkitResultTime?.format(new Date(value),{milliseconds:false})||new Date(value).toLocaleString(undefined,{hour12:true});
    let text='';
    if(preferencesPending)text='Automatic checks are waiting for your saved update preferences.';
    else if(preferences().enabled===false&&preferencesLoaded)text='Automatic checks are turned off. You can still check manually.';
    else if(!info)text='Automatic checks have not started in this window.';
    else{
      if(info.lastAttempt)text='Last automatic attempt: '+stamp(info.lastAttempt)+'. '+(info.running?'Checking…':info.lastSummary||({ok:'Check completed.',retry:'Could not complete the check.',skip:'Deferred while another update action was busy.'}[info.lastResult]||''));
      else text='No automatic update check has run in this window yet.';
      if(preferencesError)text+=' '+preferencesError;
      if(info.nextAt)text+=' '+(info.nextKind==='retry'?'Next automatic retry: ':info.nextKind==='waiting'?'Next readiness check: ':'Next automatic check: ')+stamp(info.nextAt)+'.';
    }
    return text;
  }
  function automaticStatusHTML(){return '<p class="help-line" data-app-update-schedule role="status" aria-live="polite">'+escape(automaticStatusText())+'</p>';}
  function renderAutomaticStatus(){const host=document.querySelector('[data-app-update-schedule]');if(host)host.textContent=automaticStatusText();}
  function preferenceHTML(){return '<div class="setting-row"><div><div class="setting-title">Automatic update checks</div><div class="setting-desc">Check after startup, every five minutes while active, and when you return to the app. Temporary failures and service limits may delay checks. Downloads and installation always need your approval. Remind me tomorrow hides that version for 24 hours, including after reopening; App settings stays available.</div></div><input type="checkbox" aria-label="Automatically check for updates" data-app-update-auto '+(preferences().enabled!==false?'checked':'')+' '+(!preferencesLoaded?'disabled':'')+'></div>'+automaticStatusHTML();}
  function setPreferences(changes){Object.assign(preferenceChanges,changes);state.appUpdatePreferences={...preferences(),...changes};if(preferencesLoaded)saveState();else preferenceWritesPending=true;}
  function mergePreferences(value){
    const p=value&&typeof value==='object'&&!Array.isArray(value)?value:{};
    state.appUpdatePreferences={enabled:p.enabled!==false,dismissed:typeof p.dismissed==='string'?p.dismissed.slice(0,80):'',until:Number.isFinite(p.until)?Math.min(p.until,Date.now()+86400000):0,...preferenceChanges};
  }
  function preferencesReady(canSave=true){preferencesLoaded=true;preferencesPending=false;preferencesError='';if(preferenceWritesPending){preferenceWritesPending=false;if(canSave)saveState();}render();}
  function dismiss(){const item=notice(getController().snapshot(),preferences());if(!item)return;setPreferences({dismissed:item.key,until:Date.now()+24*60*60*1000});render();}
  function reveal(){setPreferences({dismissed:'',until:0});}
  function renderNotice(){const host=document.getElementById('appUpdateNotice');if(!host)return;const content=!OFFLINE&&!shutDown&&!TERMINAL_ONLY?noticeHTML(getController().snapshot(),preferences()):'';if(host.innerHTML!==content)host.innerHTML=content;}
  function start(savedPreferences,reloadSavedState){
    if(OFFLINE||TERMINAL_ONLY||DETACHED_TERMINAL||scheduler)return;
    preferencesPending=!!savedPreferences;preferencesLoaded=!savedPreferences;
    // Native Window timers require their Window receiver, not the scheduler env.
    // Resolve them through root so injected browser/test clocks still work.
    scheduler=createScheduler({setTimeout:(...args)=>root.setTimeout(...args),clearTimeout:(...args)=>root.clearTimeout(...args),enabled:()=>!preferencesLoaded||preferences().enabled!==false,ready:()=>!shutDown,active:()=>document.hidden!==true&&root.navigator?.onLine!==false,notBefore:()=>getController().snapshot().checkAfter,changed:renderAutomaticStatus,
      describeResult:result=>result==='retry'?getController().snapshot().backgroundError||preferencesError:result==='ok'?(getController().snapshot().data?.supported===false?'Automatic updates are not supported on this platform.':getController().snapshot().data?.hasUpdate?'Update available: '+getController().snapshot().data.latest.version+'.':'No newer update was found at the last successful check.'):'',
      check:async()=>{
        // Never contact the release feed using default preferences when the saved
        // choice has not loaded. Retry only this local read until it succeeds.
        if(!preferencesLoaded){
          try{if(!reloadSavedState)throw new Error('Reopen the app to load saved settings before saving changes.');await reloadSavedState();preferencesReady();}
          catch(error){if(error.updatePreferencesLoaded){preferencesReady(false);preferencesError=error.message;}else{preferencesError='Saved update preferences could not be loaded: '+error.message;return 'retry';}}
        }
        if(preferences().enabled===false)return 'skip';
        const c=getController(),result=await c.load(true);
        if(result!=='ok')return result;
        if(preferences().enabled===false)return 'skip';
        if(c.snapshot().data?.supported!==true)return 'ok';
        return c.check(true);
      }});
    root.addEventListener?.('focus',()=>scheduler.wake());
    root.addEventListener?.('online',()=>scheduler.wake());
    document.addEventListener('visibilitychange',()=>scheduler.wake());
    if(savedPreferences)Promise.resolve(savedPreferences).then(()=>preferencesReady(),error=>{if(error.updatePreferencesLoaded){preferencesReady(false);preferencesError=error.message;}else{preferencesPending=false;preferencesError='Saved update preferences could not be loaded: '+error.message;render();}}).then(()=>{scheduler.start();if(preferencesLoaded&&pendingManualResult){scheduler.checked(pendingManualResult);pendingManualResult='';}});
    else scheduler.start();
  }
  // Startup/local reads must not hang the update workflow. Aborting also cancels
  // server-side feed work; Promise.race protects against an unresponsive transport.
  async function boundedAPI(path,method='GET',body){
    if(!['/state','/status','/updates/status','/updates/check'].includes(path))return api(path,method,body);
    const abort=new AbortController();let timer;
    try{return await Promise.race([
      api(path,method,body,false,abort.signal),
      new Promise((_,reject)=>{timer=setTimeout(()=>{abort.abort();reject(new Error('The update or saved-settings request timed out. It will be retried.'));},path==='/updates/check'?25000:10000);})
    ]);}finally{clearTimeout(timer);}
  }
  function getController(){
    if(controller)return controller;
    controller=createController({
      api:(...args)=>boundedAPI(...args),setTimeout:(...args)=>root.setTimeout(...args),clearTimeout:(...args)=>root.clearTimeout(...args),save:()=>flushState({closing:true}),editorInstallBlockReason:()=>editorInstallBlockReason(),
      upload:async(file,checksums)=>{
        const form=new FormData();form.append('package',file);form.append('checksums',checksums);
        const response=await fetch('/api/updates/import',{method:'POST',headers:{'X-Session-Token':SESSION},body:form});
        let value;try{value=await response.json();}catch{throw new Error('Unreadable server response. Refresh status before retrying.');}
        if(!response.ok)throw new Error(value.error||'Import failed ('+response.status+').');return value;
      },
      render:()=>{render();const data=controller?.snapshot().data;if(data?.restartedAfterUpdate&&!updateWelcomeShown){updateWelcomeShown=true;toast('Now running Network Toolkit '+data.currentVersion+'. Your saved settings are kept.');}if(data?.executable?.warning&&!namingWarningShown){namingWarningShown=true;toast('The app is running, but stable filename setup needs attention. Open App settings > Updates.','error');}},reveal,
      restarting:()=>{toast('Installing update. The app will close, then reopen. Allow up to 90 seconds.');}
    });return controller;
  }
  function render(){for(const host of document.querySelectorAll('[data-app-update-progress-host]'))host.innerHTML=progressHTML(getController().snapshot());const section=document.getElementById('appUpdateSettings');if(section)section.innerHTML=preferenceHTML()+html(getController().snapshot(),online&&!OFFLINE,status.version||VERSION);renderNotice();}
  function reviewDownload(version){
    const current=getController().snapshot();
    if(current.busy||!current.data?.hasUpdate||current.data.latest?.version!==version)return;
    openModal('Update Network Toolkit',releaseNotes(current.data,version)+
      '<p class="help-line">Download and verify this update now? Your current app stays running. Installation and restart require a separate confirmation.</p><div data-app-update-progress-host></div><div id="appUpdateDownloadResult" role="status" aria-live="polite"></div>',
      '<button class="btn ghost" data-action="close-modal">Not now</button><button class="btn primary" data-app-update="confirm-download" data-version="'+escape(version)+'">Download update</button>');
  }
  function showInstallConfirmation(version){
    const state=getController().snapshot();if(state.busy||state.data?.staged?.version!==version)return;
    openModal('Install Network Toolkit '+version,
      releaseNotes(state.data,version)+'<p>Replace the current app with version '+escape(version)+' and restart?</p><div class="notice warn mt">Save your work before restarting.</div><div data-app-update-progress-host></div><div id="appUpdateInstallError" class="help-line" role="alert"></div>',
      '<button class="btn ghost" data-action="close-modal">Cancel</button><button class="btn primary" data-app-update="confirm" data-version="'+escape(version)+'">Install and restart</button>');
  }
  document.addEventListener('click',async event=>{
    const button=event.target.closest?.('[data-app-update]');if(!button||button.disabled)return;
    const action=button.dataset.appUpdate,c=getController();
    if(action==='releases'){button.disabled=true;try{await api('/updates/open-releases','POST',{});}catch(error){toast('Could not open releases: '+error.message,'error');}finally{if(button.isConnected)button.disabled=false;}}
    else if(action==='later')dismiss();
    else if(action==='check'){const result=await c.check();if(preferencesLoaded)scheduler?.checked(result);else if(result!=='skip')pendingManualResult=result;}
    else if(action==='download')reviewDownload(button.dataset.version);
    else if(action==='confirm-download'){
      const result=document.getElementById('appUpdateDownloadResult');button.disabled=true;
      if(result)result.textContent='Downloading and verifying…';
      await c.download(button.dataset.version);
      if(result?.isConnected&&document.getElementById('appUpdateDownloadResult')===result){
        result.textContent=c.snapshot().error||c.snapshot().message;
        if(!c.snapshot().error&&c.snapshot().data?.staged?.version===button.dataset.version)showInstallConfirmation(button.dataset.version);
      }
      if(button.isConnected)button.disabled=false;
    }
    else if(action==='refresh')await c.load();
    else if(action==='import')await c.importPackage(document.getElementById('appUpdatePackage')?.files?.[0],document.getElementById('appUpdateChecksums')?.files?.[0]);
    else if(action==='confirm'){
      const error=document.getElementById('appUpdateInstallError');button.disabled=true;await c.install(button.dataset.version);
      if(error?.isConnected)error.textContent=c.snapshot().error||c.snapshot().message;
      if(button.isConnected)button.disabled=c.snapshot().data?.installing===true;
    }
  });
  document.addEventListener('change',event=>{
    if(!event.target.matches?.('[data-app-update-auto]'))return;
    if(!preferencesLoaded)return;setPreferences({enabled:event.target.checked});scheduler?.reset();render();
  });
  root.NetworkToolkitUpdater={html:()=>preferenceHTML()+html(getController().snapshot(),online&&!OFFLINE,status.version||VERSION),load:()=>getController().load(),start,mergePreferences,request:boundedAPI};
})(typeof window==='object'?window:globalThis);
