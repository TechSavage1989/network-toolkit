(function(root){
  'use strict';
  const RELEASES='https://github.com/TechSavage1989/network-toolkit/releases/latest';
  const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  function createController(env){
    let data=null,busy=false,error='',message='',backgroundError='',revision=0;
    const changed=()=>env.changed?.(snapshot());
    const snapshot=()=>({data,busy,error,message,backgroundError});
    const render=()=>env.render(snapshot());
    async function load(background=false){
      if(busy)return 'skip';const turn=++revision;busy=true;if(!background)error='';render();
      let outcome='retry';
      try{const result=await env.api('/updates/status');if(turn===revision){data=result;outcome='ok';}}
      catch(e){if(turn===revision){if(background)backgroundError='Automatic update status could not be read: '+e.message;else error='Could not read update status: '+e.message;}}
      finally{if(turn===revision){busy=false;render();}}
      return outcome;
    }
    async function importPackage(file,checksums){
      if(busy)return;
      if(!file||!checksums){error='Choose both the portable ZIP and SHA256SUMS from the same official release.';render();return;}
      if(!/\.zip$/i.test(file.name)||file.size>128*1024*1024||checksums.size>64*1024){error='Choose a portable ZIP under 128 MiB and a SHA256SUMS file under 64 KiB.';render();return;}
      busy=true;error='';message='Uploading and verifying the update. The installed app is unchanged.';render();
      try{data=await env.upload(file,checksums);if(!data.staged)throw new Error('No staged package was returned. Refresh update status before retrying.');message='Update verified and staged. Review it before installing.';}
      catch(e){error='Update was not imported: '+e.message;message='';}
      finally{busy=false;render();changed();}
    }
    async function check(background=false){
      if(busy||data?.installing)return 'skip';
      busy=true;if(!background){error='';message='Checking the official release feed…';}render();
      let outcome='retry';
      try{data=await env.api('/updates/check','POST',{});backgroundError='';if(!background){message=data.hasUpdate?'Update available: '+data.latest.version+'.':'No newer update is available.';env.reveal?.();}outcome='ok';}
      catch(e){if(background)backgroundError='Automatic update check could not complete: '+e.message;else{error='Could not check for updates: '+e.message;message='';}}
      finally{busy=false;render();changed();}
      return outcome;
    }
    async function download(version){
      if(busy||data?.installing)return;
      if(!data?.latest||data.latest.version!==version){error='The available release changed. Check again before downloading.';render();return;}
      busy=true;error='';message='Downloading and verifying the update. The installed app is unchanged.';render();
      try{data=await env.api('/updates/download','POST',{version});if(!data.staged)throw new Error('No staged package was returned. Refresh update status before retrying.');message='Update downloaded, verified and staged. Review it before installing.';}
      catch(e){error='The update could not be downloaded: '+e.message;message='';}
      finally{busy=false;render();changed();}
    }
    async function install(version){
      if(busy||data?.installing)return;
      if(!data?.staged||data.staged.version!==version){error='The staged update changed. Refresh and review it again.';render();return;}
      busy=true;error='';message='Saving settings before restart…';render();
      try{
        if(!await env.save())throw new Error('Settings could not be saved. Installation was not started.');
        const current=await env.api('/updates/status');data=current;
        if(!current.staged||current.staged.version!==version)throw new Error('The staged update changed. Review it again.');
        if(current.busy?.length)throw new Error('Finish or stop active operations first: '+current.busy.join(', '));
        message='Requesting installation. Do not close the app until it restarts.';render();
        const result=await env.api('/updates/install','POST',{confirm:true,version});
        if(!result?.restarting)throw new Error('The app did not confirm a restart. Refresh status before trying again.');
        data={...data,installing:true};message='Installation requested. The app will close and restart. Settings are kept.';
        env.restarting();
      }catch(e){error=e.message;message='';}
      finally{busy=false;render();changed();}
    }
    return {snapshot,load,importPackage,check,download,install};
  }
  function html(state,available,currentVersion){
    const {data,busy,error,message,backgroundError}=state,disabled=!available||busy||data?.installing||data?.supported===false,staged=data?.staged;
    return '<h3>App updates</h3><p class="help-line">Installed version: '+escape(data?.currentVersion||currentVersion)+'</p>'+
      (data?.executable?'<p class="help-line" style="overflow-wrap:anywhere">Running file: '+escape(data.executable.currentPath)+'</p>'+(data.executable.canonicalPath&&data.executable.canonicalPath!==data.executable.currentPath?'<p class="help-line" style="overflow-wrap:anywhere">Stable executable: '+escape(data.executable.canonicalPath)+'. Pinned shortcuts, shared shortcuts, and shortcuts elsewhere may need to be pointed to Network_Toolkit.exe manually.</p>':'')+(data.executable.warning?'<div class="notice warn">'+escape(data.executable.warning)+'</div>':''):'')+
      '<p class="help-line">Check the official public binary-release feed. Downloads are verified before installation; the public main repository provides a binary-only update package. Nothing installs until you confirm.</p>'+
      '<div class="row gap6 wrap"><button type="button" class="btn small primary" data-app-update="check" '+(disabled?'disabled':'')+'>Check for updates</button><button type="button" class="btn small ghost" data-app-update="releases" '+(disabled?'disabled':'')+'>Open release notes in browser</button></div>'+
      (data?.latest?'<div class="notice info mt">Latest release: '+escape(data.latest.version)+'</div>'+(data.latest.notes?'<p class="help-line" style="white-space:pre-wrap;overflow-wrap:anywhere">'+escape(data.latest.notes)+'</p>':'')+(data.hasUpdate===true?'<button type="button" class="btn small" data-app-update="download" data-version="'+escape(data.latest.version)+'" '+(disabled?'disabled':'')+'>Download update</button>':'<p class="help-line">You are up to date.</p>'):'')+
      '<details class="mt"><summary>Import a downloaded update instead</summary><p class="help-line">Select the binary-only Update.zip and SHA256SUMS from the same official release. No GitHub credentials are needed.</p><p class="help-line" style="overflow-wrap:anywhere">Official feed: '+RELEASES+'</p>'+
      (!available?'<p class="help-line">Open the executable to import and install updates.</p>':data?.supported===false?'<p class="help-line">In-app installation is supported only by the Windows x64 executable.</p>':'<div class="form-stack mt"><div class="field"><label for="appUpdatePackage">Official binary-only Update ZIP</label><input type="file" id="appUpdatePackage" accept=".zip" '+(disabled?'disabled':'')+'></div><div class="field"><label for="appUpdateChecksums">SHA256SUMS from the same release</label><input type="file" id="appUpdateChecksums" '+(disabled?'disabled':'')+'></div><div><button type="button" class="btn small" data-app-update="import" '+(disabled?'disabled':'')+'>Verify and stage update</button></div></div>')+
      '</details><p class="help-line">SHA-256 checks detect corruption, not publisher identity. Use files only from the official release page. Only the app executable is replaced; your app data folder is not changed.</p>'+
      (staged?'<div class="notice info mt"><span>Ready to install version '+escape(staged.version)+'.</span></div><p class="help-line" style="overflow-wrap:anywhere">Executable SHA-256: '+escape(staged.sha256)+'</p><button type="button" class="btn primary" data-app-update="review" '+(disabled?'disabled':'')+'>Review install and restart</button>':'')+
      (data?.busy?.length?'<p class="help-line">Active operations: '+escape(data.busy.join(', '))+'. Finish or stop them before installing.</p>':'')+
      '<div role="status" aria-live="polite">'+(message?'<p class="help-line">'+escape(message)+'</p>':'')+(busy&&!message?'<p class="help-line">Reading update status…</p>':'')+(backgroundError?'<p class="help-line">'+escape(backgroundError)+' Use Check for updates to retry.</p>':'')+(error?'<div class="notice warn mt">'+escape(error)+'</div>':'')+'</div>';
  }
  // Timers are independent of navigation. No timer may download, install, or restart.
  function createScheduler(env){
    let timer=null,started=false,generation=0,failures=0;
    let lastAttempt=0,lastResult='',lastSummary='',nextAt=0,nextKind='',running=false;
    const normal=6*60*60*1000,now=()=>env.now?.()??Date.now();
    const snapshot=()=>({lastAttempt,lastResult,lastSummary,nextAt,nextKind,running,failures});
    const changed=()=>env.changed?.(snapshot());
    // Brief startup/network interruptions get two early retries, then bounded backoff.
    const retryDelay=()=>failures===1?30000:failures===2?120000:Math.min(normal,15*60*1000*2**Math.min(failures-3,5));
    function schedule(delay,epoch,kind){
      nextAt=now()+delay;nextKind=kind;
      timer=env.setTimeout(async()=>{
        timer=null;if(!started||epoch!==generation)return;
        nextAt=0;nextKind='';
        let result='skip';
        if(env.enabled()&&env.ready()){
          lastAttempt=now();running=true;changed();
          try{result=await env.check();}catch{result='retry';}
          if(!started||epoch!==generation)return;
          running=false;lastResult=result;lastSummary=env.describeResult?.(result)||'';
        }
        if(!started||epoch!==generation)return;
        if(result==='ok')failures=0;else if(result==='retry')failures++;
        const enabled=env.enabled();
        schedule(!enabled?normal:result==='skip'?15000:result==='retry'?retryDelay():normal,epoch,!enabled?'disabled':result==='skip'?'waiting':result==='retry'?'retry':'check');
      },delay);
      changed();
    }
    function stop(){started=false;generation++;if(timer!==null)env.clearTimeout(timer);timer=null;nextAt=0;nextKind='';running=false;changed();}
    return {
      snapshot,
      start(){if(started)return;started=true;schedule(5000,++generation,'check');},
      stop,
      reset(){stop();failures=0;this.start();},
      checked(result){
        if(result==='skip')return;
        // A manual check replaces a pending automatic check, rather than duplicating it.
        stop();started=true;if(result==='ok')failures=0;else failures++;
        schedule(result==='ok'?normal:retryDelay(),generation,result==='ok'?'check':'retry');
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
    return '<div class="notice info row wrap gap6" role="status" aria-live="polite"><span style="flex:1;min-width:180px">Network Toolkit '+escape(item.version)+(item.kind==='install'?' is downloaded and verified. Install and restart?':' is available. Download it now?')+'</span><button type="button" class="btn small primary" data-app-update="'+(item.kind==='install'?'review':'download')+'" data-version="'+escape(item.version)+'" '+(state.busy?'disabled':'')+'>'+(item.kind==='install'?'Review install and restart':'Download update')+'</button><button type="button" class="btn small ghost" data-app-update="later">Later</button>'+(state.error?'<span style="flex-basis:100%" role="alert">'+escape(state.error)+'</span>':'')+(state.busy&&state.message?'<span style="flex-basis:100%">'+escape(state.message)+'</span>':'')+'</div>';
  }
  if(typeof module==='object'&&module.exports)module.exports={createController,createScheduler,notice,noticeHTML,html};
  if(!root.document)return;
  let controller,scheduler,updateWelcomeShown=false,namingWarningShown=false;
  let preferencesLoaded=true,preferencesPending=false,preferencesError='',preferenceWritesPending=false,pendingManualResult='';
  const preferenceChanges={};
  const preferences=()=>state.appUpdatePreferences||{};
  function automaticStatusText(){
    const info=scheduler?.snapshot(),stamp=value=>new Date(value).toLocaleString();
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
  function preferenceHTML(){return '<div class="setting-row"><div><div class="setting-title">Automatic update checks</div><div class="setting-desc">Check after startup and every six hours, with earlier retries after temporary failures. Downloads and installation always need your approval. Later hides that reminder for 24 hours; App settings stays available.</div></div><input type="checkbox" aria-label="Automatically check for updates" data-app-update-auto '+(preferences().enabled!==false?'checked':'')+' '+(!preferencesLoaded?'disabled':'')+'></div>'+automaticStatusHTML();}
  function setPreferences(changes){Object.assign(preferenceChanges,changes);state.appUpdatePreferences={...preferences(),...changes};if(preferencesLoaded)saveState();else preferenceWritesPending=true;}
  function mergePreferences(value){
    const p=value&&typeof value==='object'&&!Array.isArray(value)?value:{};
    state.appUpdatePreferences={enabled:p.enabled!==false,dismissed:typeof p.dismissed==='string'?p.dismissed.slice(0,80):'',until:Number.isFinite(p.until)?Math.min(p.until,Date.now()+86400000):0,...preferenceChanges};
  }
  function preferencesReady(){preferencesLoaded=true;preferencesPending=false;preferencesError='';if(preferenceWritesPending){preferenceWritesPending=false;saveState();}render();}
  function dismiss(){const item=notice(getController().snapshot(),preferences());if(!item)return;setPreferences({dismissed:item.key,until:Date.now()+24*60*60*1000});render();}
  function reveal(){setPreferences({dismissed:'',until:0});}
  function renderNotice(){const host=document.getElementById('appUpdateNotice');if(!host)return;const content=online&&!OFFLINE&&!shutDown&&!TERMINAL_ONLY?noticeHTML(getController().snapshot(),preferences()):'';if(host.innerHTML!==content)host.innerHTML=content;}
  function start(savedPreferences,reloadSavedState){
    if(OFFLINE||TERMINAL_ONLY||DETACHED_TERMINAL||scheduler)return;
    preferencesPending=!!savedPreferences;preferencesLoaded=!savedPreferences;
    scheduler=createScheduler({setTimeout,clearTimeout,enabled:()=>!preferencesLoaded||preferences().enabled!==false,ready:()=>online&&!shutDown,changed:renderAutomaticStatus,
      describeResult:result=>result==='retry'?getController().snapshot().backgroundError||preferencesError:result==='ok'?(getController().snapshot().data?.supported===false?'Automatic updates are not supported on this platform.':getController().snapshot().data?.hasUpdate?'Update available: '+getController().snapshot().data.latest.version+'.':'No newer update is available.'):'',
      check:async()=>{
        // Never contact the release feed using default preferences when the saved
        // choice has not loaded. Retry only this local read until it succeeds.
        if(!preferencesLoaded){
          try{if(!reloadSavedState)throw new Error('Reopen the app to load saved settings before saving changes.');await reloadSavedState();preferencesReady();}
          catch(error){preferencesError='Saved update preferences could not be loaded: '+error.message;return 'retry';}
        }
        if(preferences().enabled===false)return 'skip';
        const c=getController(),result=await c.load(true);
        if(result!=='ok')return result;
        if(preferences().enabled===false)return 'skip';
        if(c.snapshot().data?.supported!==true)return 'ok';
        return c.check(true);
      }});
    if(savedPreferences)Promise.resolve(savedPreferences).then(()=>preferencesReady(),error=>{preferencesPending=false;preferencesError='Saved update preferences could not be loaded: '+error.message;render();}).then(()=>{scheduler.start();if(preferencesLoaded&&pendingManualResult){scheduler.checked(pendingManualResult);pendingManualResult='';}});
    else scheduler.start();
  }
  function getController(){
    if(controller)return controller;
    controller=createController({
      api:(...args)=>api(...args),save:()=>flushState({closing:true}),
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
  function render(){const section=document.getElementById('appUpdateSettings');if(section)section.innerHTML=preferenceHTML()+html(getController().snapshot(),online&&!OFFLINE,status.version||VERSION);renderNotice();}
  function review(){
    const state=getController().snapshot(),version=state.data?.staged?.version;if(state.busy||!version)return;
    openModal('Install Network Toolkit '+version,
      '<p>Replace the current app with version '+escape(version)+' and restart?</p><div class="notice warn mt">Export any diagnostic results and save any edited files first. Restart closes this app and its connections. Finish diagnostics, transfers, packet captures, and terminal sessions before continuing. Close other Network Toolkit windows too.</div><p class="help-line">Updates keep the stable Network_Toolkit.exe filename. During one-time migration, matching shortcuts on your personal Desktop and Start Menu are updated. Pinned shortcuts, shared shortcuts, and shortcuts elsewhere may need to be pointed to Network_Toolkit.exe manually. Custom-named executables are left unchanged. Update state and the previous-version backup are kept in your settings folder. Saved preferences and connection profiles stay in the app data folder. Brief atomic swap files may exist during an update; error recovery may preserve verified files. Managed staging files and older backups are cleaned after confirmed success; locked files remain pending until safely removable. If reopening fails, a local error explains recovery. Installation logs are kept in the update staging folder while installing; after successful cleanup, previous.json beside the settings-local backup retains the latest outcome and progress. SHA-256 verification checks integrity, not the identity of the publisher.</p><div id="appUpdateInstallError" class="help-line" role="alert"></div>',
      '<button class="btn ghost" data-action="close-modal">Cancel</button><button class="btn primary" data-app-update="confirm" data-version="'+escape(version)+'">Install and restart</button>');
  }
  document.addEventListener('click',async event=>{
    const button=event.target.closest?.('[data-app-update]');if(!button||button.disabled)return;
    const action=button.dataset.appUpdate,c=getController();
    if(action==='releases'){button.disabled=true;try{await api('/updates/open-releases','POST',{});}catch(error){toast('Could not open releases: '+error.message,'error');}finally{if(button.isConnected)button.disabled=false;}}
    else if(action==='later')dismiss();
    else if(action==='check'){const result=await c.check();if(preferencesLoaded)scheduler?.checked(result);else if(result!=='skip')pendingManualResult=result;}
    else if(action==='download')await c.download(button.dataset.version);
    else if(action==='refresh')await c.load();
    else if(action==='import')await c.importPackage(document.getElementById('appUpdatePackage')?.files?.[0],document.getElementById('appUpdateChecksums')?.files?.[0]);
    else if(action==='review')review();
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
  root.NetworkToolkitUpdater={html:()=>preferenceHTML()+html(getController().snapshot(),online&&!OFFLINE,status.version||VERSION),load:()=>getController().load(),start,mergePreferences};
})(typeof window==='object'?window:globalThis);
