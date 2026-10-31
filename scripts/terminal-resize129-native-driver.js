/* Test-only native resize129 extension. Never embedded in the application. */
(function(){
'use strict';
window.__nativeResize129Snapshot=function(e){
 const x=e?.xterm,b=x?.buffer.active,view=e?.element?.getBoundingClientRect(),screen=e?.element?.querySelector('.xterm-screen')?.getBoundingClientRect(),body=e?.element?.parentElement?.getBoundingClientRect(),d=document.querySelector('[data-split-axis="column"]')?.getBoundingClientRect();
 return {cols:x?.cols||0,rows:x?.rows||0,buffer:b?.type||'',firstLine:b?.getLine(0)?.translateToString(true)||'',dragging:!!terminalPaneDrag,settled:!!(e&&!e.desiredResize&&!e.resizeSending&&e.lastResize===x?.cols+'x'+x?.rows),fits:!!(view&&screen&&body&&view.width>0&&view.height>0&&screen.right<=view.right+1&&screen.bottom<=view.bottom+1&&view.right<=body.right+1&&view.bottom<=body.bottom+1),divider:d?{left:d.left,top:d.top,width:d.width,height:d.height}:null};
};
window.__nativeResize129=async function(c,snapshot,entry,wait){
 const action=c.action.slice('resize129:'.length),e=entry();
 if(action==='prepare'){NetworkToolkitSessionSidebar.setCollapsed(true);setTerminalLayout('columns');assignTerminalPane(0,e.info.id);}
 else if(action==='maximize-pane')maximizeTerminalPane(0);
 else if(action==='restore-pane')maximizeTerminalPane(0);
 else if(action==='hide'){navigate('overview');return snapshot();}
 else if(action==='reveal')navigate('terminal');
 else if(action==='single')setTerminalLayout('single');
 else if(action==='columns')setTerminalLayout('columns');
 else throw Error('Unknown bounded native resize129 action');
 await wait(()=>{const s=window.__nativeResize129Snapshot(entry());return s.fits&&s.settled;},'local resize129 settled '+action);
 return snapshot();
};
})();
