/* Read-only live activity. Saved profiles, tabs and cached results are not sources. */
(function(root){
 'use strict';
 const pages=new Set(['quick','ping','trace','dns','smtp','tcp','http','tls','identify','discover','monitor','capture','local','hosts','wol','terminal','sftp','tftp']);
 const kinds=['connections','servers','operations','captures'];
 const interval=1000,timeout=2500,freshness=4000;
 let initialized=false,running=false,request=null,available=()=>false,lastAvailable=false;
 let generation=0,pending=null,pollTimer=null,expiryTimer=null,lastSuccess=0,snapshot=null;
 const annotations=new WeakMap(),clock=()=>root.performance?.now?.()??Date.now();
 function normalize(value){
  if(!value||typeof value!=='object'||Array.isArray(value)||!value.tools||typeof value.tools!=='object'||Array.isArray(value.tools))throw new Error('Invalid activity snapshot');
  const tools=Object.create(null);
  for(const page of pages){
   const source=value.tools[page];if(!source||typeof source!=='object'||Array.isArray(source))continue;
   const counts=Object.create(null);
   for(const kind of kinds){const n=source[kind];if(Number.isSafeInteger(n)&&n>0&&n<=1000000)counts[kind]=n;}
   if(Object.keys(counts).length)tools[page]=counts;
  }
  return tools;
 }
 function summary(counts){
  if(!counts)return '';
  const parts=[];
  if(counts.connections)parts.push(counts.connections+' connected session'+(counts.connections===1?'':'s'));
  if(counts.servers)parts.push(counts.servers===1?'Server running':counts.servers+' servers running');
  if(counts.operations)parts.push(counts.operations===1?'Operation running':counts.operations+' operations running');
  if(counts.captures)parts.push(counts.captures===1?'Capture running':counts.captures+' captures running');
  return parts.join('; ');
 }
 function canRead(){try{return running&&available()===true;}catch{return false;}}
 function attribute(node,name,value){if(value===null)node.removeAttribute(name);else node.setAttribute(name,value);}
 function sync(){
  const fresh=snapshot&&clock()-lastSuccess<freshness&&canRead();
  for(const button of root.document?.querySelectorAll?.('#navigation .nav-item[data-page]')||[]){
   const page=button.dataset.page,description=fresh?summary(snapshot[page]):'';
   let record=annotations.get(button);
   const aria=button.getAttribute('aria-label'),title=button.getAttribute('title');
   if(!record||record.page!==page){record={page,aria,title,appliedAria:null,appliedTitle:null};annotations.set(button,record);}
   else{if(aria!==record.appliedAria)record.aria=aria;if(title!==record.appliedTitle)record.title=title;}
   let dot=button.querySelector('.sidebar-activity-dot');
   if(description){
    const name=button.querySelector('.nav-item-text')?.textContent.trim()||[...button.children].find(n=>n.tagName==='SPAN'&&!n.classList.contains('badge')&&!n.classList.contains('sidebar-activity-dot'))?.textContent.trim()||page;
    if(!dot){dot=root.document.createElement('span');dot.className='sidebar-activity-dot';dot.setAttribute('aria-hidden','true');button.append(dot);}
    record.appliedAria=(record.aria||name)+', '+description;
    record.appliedTitle=(record.title||name)+'\n'+description+'.';
    attribute(button,'aria-label',record.appliedAria);attribute(button,'title',record.appliedTitle);
    button.setAttribute('data-activity-summary',description);
   }else{
    dot?.remove();button.removeAttribute('data-activity-summary');
    if(aria===record.appliedAria)attribute(button,'aria-label',record.aria);
    if(title===record.appliedTitle)attribute(button,'title',record.title);
    record.appliedAria=null;record.appliedTitle=null;
   }
  }
 }
 function clear(){snapshot=null;lastSuccess=0;root.clearTimeout(expiryTimer);expiryTimer=null;sync();}
 function invalidate(){
  generation++;root.clearTimeout(pollTimer);pollTimer=null;
  const old=pending;pending=null;old?.controller?.abort();old?.cancel?.();clear();
 }
 function schedule(){root.clearTimeout(pollTimer);pollTimer=null;if(canRead())pollTimer=root.setTimeout(()=>{pollTimer=null;void refresh();},interval);}
 function refresh(){
  if(!canRead()){lastAvailable=false;invalidate();return Promise.resolve(false);}
  lastAvailable=true;if(pending)return pending.promise;
  root.clearTimeout(pollTimer);pollTimer=null;
  const token=generation,controller=typeof root.AbortController==='function'?new root.AbortController():null;
  const task={controller,cancel:null,timer:null,promise:null};pending=task;
  const deadline=new Promise((resolve,reject)=>{task.cancel=()=>reject(new Error('Activity request superseded'));task.timer=root.setTimeout(()=>{controller?.abort();reject(new Error('Activity request timed out'));},timeout);});
  const response=Promise.resolve().then(()=>{if(token!==generation||pending!==task||!canRead())throw new Error('Activity request superseded');return request(controller?.signal);});
  task.promise=Promise.race([response,deadline]).then(value=>{
   if(token!==generation||pending!==task||!canRead())return false;
   snapshot=normalize(value);lastSuccess=clock();sync();
   root.clearTimeout(expiryTimer);expiryTimer=root.setTimeout(()=>{expiryTimer=null;sync();},freshness);
   return true;
  }).catch(()=>{if(token===generation&&pending===task)clear();return false;}).finally(()=>{
   root.clearTimeout(task.timer);
   if(pending===task){pending=null;schedule();}
  });
  return task.promise;
 }
 function connectionChanged(){
  const now=canRead();if(!now){if(lastAvailable||pending||snapshot)invalidate();lastAvailable=false;return;}
  if(!lastAvailable){lastAvailable=true;void refresh();}else sync();
 }
 function start(){if(!initialized||running)return;running=true;lastAvailable=false;connectionChanged();}
 function stop(){running=false;lastAvailable=false;invalidate();}
 function init(options={}){
  if(initialized)return;
  if(typeof options.request!=='function'||typeof options.available!=='function')throw new Error('Activity reader requires read-only request and availability callbacks');
  initialized=true;request=options.request;available=options.available;
  root.addEventListener?.('focus',()=>{if(canRead())void refresh();else connectionChanged();});
  root.document?.addEventListener?.('visibilitychange',()=>{if(root.document.visibilityState==='visible'){sync();if(canRead())void refresh();else connectionChanged();}});
  root.addEventListener?.('pagehide',stop);root.addEventListener?.('pageshow',start);start();
 }
 const api=Object.freeze({init,start,stop,refresh,connectionChanged,sync,normalize,summary});
 root.NetworkToolkitSidebarActivity=api;if(typeof module==='object'&&module.exports)module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
