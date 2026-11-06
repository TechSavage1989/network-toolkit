/* Linux window behaviour (v1.21.0, docs/LINUX_PLAN.md).
 * - "browser-tab" mode (no Chromium-family browser installed): closing a
 *   browser tab cannot stop the app, so the sidebar shows a Quit Network
 *   Toolkit button. It uses the normal close path (closeApp), which saves
 *   settings and asks the editor about unsaved documents.
 * - Outside the Windows WebView2 host, closing the window while a terminal
 *   session is live asks first (the editor already guards unsaved documents).
 * Windows (uiHost "webview2") is unchanged. */
(function(root){
'use strict';
const document=root.document;if(!document)return;
const host=()=>{try{return typeof status!=='undefined'&&status?status.uiHost||'':'';}catch{return '';}};
const liveSessions=()=>{try{return typeof terminalSessions!=='undefined'?[...terminalSessions.values()].filter(e=>typeof terminalLive==='function'&&terminalLive(e)).length:0;}catch{return 0;}};
const ico=name=>{try{return typeof icon==='function'?icon(name):'';}catch{return '';}};
function sync(){
 const tab=host()==='browser-tab',bar=document.querySelector('.app-utility-actions');if(!bar)return;
 let button=document.getElementById('linuxQuitButton');
 if(tab&&!button){button=document.createElement('button');button.type='button';button.className='iconbtn';button.id='linuxQuitButton';button.title='Quit Network Toolkit';button.setAttribute('aria-label','Quit Network Toolkit');button.innerHTML=ico('power');bar.append(button);}
 if(button)button.hidden=!tab;
}
function quit(){
 const live=liveSessions(),close=()=>{try{if(typeof closeApp==='function')void closeApp();}catch{}};
 if(!live){close();return;}
 if(typeof openModal!=='function'){close();return;}
 openModal('Quit Network Toolkit','<p>'+live+' terminal session'+(live===1?' is':'s are')+' still connected. Quitting disconnects '+(live===1?'it':'them')+'. Export any output you need first.</p>','<button class="btn ghost" data-action="close-modal">Cancel</button><button class="btn danger" type="button" data-linux-quit-confirm>Quit</button>',true);
}
document.addEventListener('click',event=>{
 if(event.target.closest?.('#linuxQuitButton')){event.preventDefault();quit();return;}
 if(event.target.closest?.('[data-linux-quit-confirm]')){event.preventDefault();try{closeModal();}catch{}try{void closeApp();}catch{}}
},true);
root.addEventListener('beforeunload',event=>{
 const h=host();if(!h||h==='webview2')return;
 let stopped=false;try{stopped=typeof shutDown!=='undefined'&&shutDown;}catch{}
 if(!stopped&&liveSessions()){event.preventDefault();event.returnValue='';}
});
root.setInterval(sync,1500);root.addEventListener('load',sync);
root.NetworkToolkitLinuxShell=Object.freeze({sync,quit});
})(window);
