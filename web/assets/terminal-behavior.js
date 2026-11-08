/* Terminal features and Advanced settings modeled on MobaXterm (T3, v1.21.8):
   scrollback and scrollbar, bell, bracketed paste, word delimiters, implicit
   CR/LF and the ^E answerback. Display-only: saved output, transcripts and
   macros keep the bytes the device sent. */
(function(root){
'use strict';
const document=root.document;
// xterm.js's own word separators; the default keeps today's double-click selection.
const WORD_SEPARATORS=' ()[]{}\',"`';
const LIMITS=Object.freeze({scrollbackMin:500,scrollbackMax:10000,answerback:64,separators:64,bellMs:250,answerbackMs:500});
const BELLS=['none','visual','sound'];
function text(value,max,pattern){return typeof value==='string'?Array.from(value).filter(ch=>pattern.test(ch)).slice(0,max).join(''):null;}
function normalize(value){
 const v=value&&typeof value==='object'&&!Array.isArray(value)?value:{};
 const scrollback=Number.isInteger(v.scrollback)&&v.scrollback>=LIMITS.scrollbackMin&&v.scrollback<=LIMITS.scrollbackMax?v.scrollback:LIMITS.scrollbackMax;
 const separators=text(v.wordSeparators,LIMITS.separators,/^[^\p{Cc}\p{L}\p{N}]$/u);
 return {scrollback,scrollbar:v.scrollbar!==false,bell:BELLS.includes(v.bell)?v.bell:'none',bracketedPaste:v.bracketedPaste!==false,wordSeparators:separators===null?WORD_SEPARATORS:separators,lfImpliesCr:v.lfImpliesCr===true,crImpliesLf:v.crImpliesLf===true,reconnectKeys:v.reconnectKeys!==false,closeOnOtherKey:v.closeOnOtherKey===true,
  // Printable ASCII only: the reply is sent as raw bytes whenever the device asks.
  answerback:text(v.answerback,LIMITS.answerback,/^[\x20-\x7e]$/)||''};
}
// The app's state, saveState and terminalSessions are top-level script bindings, not window properties.
function preferences(){return normalize(typeof state==='undefined'?undefined:state.terminalBehavior);}
function options(value=preferences()){const p=normalize(value);return {scrollback:p.scrollback,convertEol:p.lfImpliesCr,ignoreBracketedPasteMode:!p.bracketedPaste,wordSeparator:p.wordSeparators};}
// "Implicit LF in every CR": a CR that is not already followed by LF gains one.
// A CR at the end of a chunk is completed at once; an LF that then starts the
// next chunk is dropped so CR LF split across chunks is not doubled.
function displayBytes(entry,bytes,value=preferences()){
 if(!normalize(value).crImpliesLf||!bytes?.length){if(entry)entry._ntCrCompleted=false;return bytes;}
 let start=entry?._ntCrCompleted&&bytes[0]===10?1:0,count=0;
 for(let i=start;i<bytes.length;i++)if(bytes[i]===13&&bytes[i+1]!==10)count++;
 if(!count&&!start){entry._ntCrCompleted=false;return bytes;}
 const out=new Uint8Array(bytes.length-start+count);let o=0;
 for(let i=start;i<bytes.length;i++){out[o++]=bytes[i];if(bytes[i]===13&&bytes[i+1]!==10)out[o++]=10;}
 if(entry)entry._ntCrCompleted=bytes[bytes.length-1]===13;
 return out;
}
// ^E (ENQ) asks the terminal to identify itself. Reply only to live output,
// only when an answerback is set, and at most twice a second per session.
function observeOutput(entry,bytes){
 const reply=preferences().answerback;if(!reply||!entry||!bytes?.includes?.(5))return false;
 const now=Date.now();if(now-(entry._ntAnswerbackAt||0)<LIMITS.answerbackMs)return false;
 entry._ntAnswerbackAt=now;if(typeof enqueueTerminalInput==='function')enqueueTerminalInput(entry,reply,true,'answerback');return true;
}
let audio=null;
function beep(){
 try{const Context=root.AudioContext||root.webkitAudioContext;if(!Context)return;audio??=new Context();if(audio.state==='suspended')void audio.resume?.();
  const osc=audio.createOscillator(),gain=audio.createGain(),t=audio.currentTime;osc.type='sine';osc.frequency.value=880;gain.gain.setValueAtTime(0.06,t);gain.gain.exponentialRampToValueAtTime(0.0001,t+0.12);osc.connect(gain).connect(audio.destination);osc.start(t);osc.stop(t+0.13);}catch{}
}
function ring(entry){
 const bell=preferences().bell;if(bell==='none'||!entry)return false;
 const now=Date.now();if(now-(entry._ntBellAt||0)<LIMITS.bellMs)return false;entry._ntBellAt=now;
 if(bell==='sound'){beep();return true;}
 const element=entry.element;if(!element)return false;
 element.classList.remove('terminal-bell-flash');void element.offsetWidth;element.classList.add('terminal-bell-flash');
 clearTimeout(entry._ntBellTimer);entry._ntBellTimer=setTimeout(()=>element.classList.remove('terminal-bell-flash'),200);return true;
}
function applyEntry(entry,p=preferences()){
 if(!entry?.xterm)return;
 const next=options(p),current=entry.xterm.options;
 for(const [key,value] of Object.entries(next))if(current[key]!==value)current[key]=value;
 entry.element?.classList.toggle('terminal-hide-scrollbar',!p.scrollbar);
}
function attach(entry){
 if(!entry?.xterm||entry._ntBehaviorBound)return;entry._ntBehaviorBound=true;
 entry.element?.classList.toggle('terminal-hide-scrollbar',!preferences().scrollbar);
 const bell=entry.xterm.onBell?.(()=>ring(entry));if(bell)entry.disposables?.push(bell);
}
function apply(){
 const p=preferences();for(const entry of (typeof terminalSessions==='undefined'?null:terminalSessions)?.values?.()||[])applyEntry(entry,p);
 const note=document.querySelector('.terminal-transcript-note');if(note)note.textContent='Output remains in this app session. Transcript export includes the retained screen and up to '+p.scrollback.toLocaleString('en-US')+' scrollback lines. Saved profiles contain settings and optional protected credential links.';
}
// Reconnection keys (T3b, v1.21.9): on a disconnected session's pane, R reconnects
// and S saves the session; optionally any other character key closes the tab.
// Only plain keys count, never in a field, a dialog or another page.
function disconnectedEntry(){
 if(typeof currentTerminal!=='function'||typeof state==='undefined')return null;
 if(typeof TERMINAL_ONLY!=='undefined'&&!TERMINAL_ONLY&&state.activePage!=='terminal')return null;
 if(document.querySelector('#modalRoot')?.children?.length)return null;
 const entry=currentTerminal();if(!entry||!['closed','error'].includes(entry.info?.state)||entry.reconnectBusy)return null;
 return typeof terminalReconnectAllowed!=='function'||terminalReconnectAllowed(entry)?entry:null;
}
function editable(target){if(!target?.closest)return false;if(target.closest('.xterm-helper-textarea'))return false;return !!target.closest('input,textarea,select,[contenteditable="true"],[contenteditable=""]');}
function sessionKey(event){
 const p=preferences();if(!p.reconnectKeys||event.defaultPrevented||event.ctrlKey||event.altKey||event.metaKey||event.isComposing||event.repeat||editable(event.target))return '';
 const entry=disconnectedEntry();if(!entry)return '';
 const key=String(event.key||'');
 if(key==='r'||key==='R'){event.preventDefault();event.stopImmediatePropagation();if(typeof reconnectTerminal==='function')void reconnectTerminal(entry);return 'reconnect';}
 if(key==='s'||key==='S'){event.preventDefault();event.stopImmediatePropagation();root.NetworkToolkitSessionSidebar?.save(entry);return 'save';}
 if(p.closeOnOtherKey&&Array.from(key).length===1&&key!==' '){event.preventDefault();event.stopImmediatePropagation();if(typeof removeTerminal==='function')removeTerminal(entry);return 'close';}
 return '';
}
document.addEventListener('keydown',sessionKey,true);
function keysHint(){const p=preferences();return p.reconnectKeys?'Press R to reconnect or S to save this session'+(p.closeOnOtherKey?'; any other key closes the tab.':'.'):'';}
function settingsHTML(){
 return '<section id="terminalBehaviorSettings" class="terminal-tools-section terminal-behavior-settings"><h3>Terminal features</h3><div class="terminal-behavior-grid">'
 +'<div class="field"><label for="terminalScrollback">Scrollback lines</label><input id="terminalScrollback" type="number" data-terminal-behavior="scrollback" min="'+LIMITS.scrollbackMin+'" max="'+LIMITS.scrollbackMax+'" step="500" data-input-clear="off"><span class="hint">'+LIMITS.scrollbackMin+' to '+LIMITS.scrollbackMax.toLocaleString('en-US')+' lines kept above the screen. Export transcript includes them.</span></div>'
 +'<div class="field"><label for="terminalBell">Bell</label><select id="terminalBell" data-terminal-behavior="bell"><option value="none">None</option><option value="visual">Flash the terminal</option><option value="sound">Play a sound</option></select></div>'
 +'<div class="field terminal-behavior-wide"><label for="terminalWordSeparators">Word delimiters for double-click selection</label><input id="terminalWordSeparators" data-terminal-behavior="wordSeparators" maxlength="'+LIMITS.separators+'" autocomplete="off" spellcheck="false" data-input-clear="off"><span class="hint">Characters that end a word. Letters and digits are always part of a word.</span></div></div>'
 +'<label class="terminal-behavior-check"><input type="checkbox" id="terminalScrollbar" data-terminal-behavior="scrollbar">Show scrollbar</label>'
 +'<label class="terminal-behavior-check"><input type="checkbox" id="terminalBracketedPaste" data-terminal-behavior="bracketedPaste">Bracketed paste (programs that ask for it can tell pasted text from typing)</label>'
 +'<label class="terminal-behavior-check"><input type="checkbox" id="terminalReconnectKeys" data-terminal-behavior="reconnectKeys">After a session ends, press R to reconnect or S to save it</label>'
 +'<label class="terminal-behavior-check terminal-behavior-sub"><input type="checkbox" id="terminalCloseOnOtherKey" data-terminal-behavior="closeOnOtherKey">Any other key closes the ended session’s tab</label>'
 +'<h4>Advanced</h4>'
 +'<label class="terminal-behavior-check"><input type="checkbox" id="terminalLfImpliesCr" data-terminal-behavior="lfImpliesCr">Implicit CR in every LF</label>'
 +'<label class="terminal-behavior-check"><input type="checkbox" id="terminalCrImpliesLf" data-terminal-behavior="crImpliesLf">Implicit LF in every CR</label>'
 +'<p class="hint">Turn these on only for devices whose lines overlap or run together; they change how output is shown, not what is saved.</p>'
 +'<div class="field"><label for="terminalAnswerback">Answerback to ^E</label><input id="terminalAnswerback" data-terminal-behavior="answerback" maxlength="'+LIMITS.answerback+'" autocomplete="off" spellcheck="false" placeholder="No reply"><span class="hint">Sent automatically whenever a device asks with ^E (ENQ). Letters, digits and symbols only; never put a password here.</span></div>'
 +'<button type="button" class="btn small ghost" data-terminal-behavior-reset>Reset terminal features</button></section>';
}
function sync(){
 const section=document.getElementById('terminalBehaviorSettings');if(!section)return;
 const p=preferences();
 for(const field of section.querySelectorAll('[data-terminal-behavior]')){const key=field.dataset.terminalBehavior;if(field.getAttribute('type')==='checkbox')field.checked=p[key]===true;else if(field!==document.activeElement)field.value=String(p[key]);}
 const close=section.querySelector('#terminalCloseOnOtherKey');if(close)close.disabled=!p.reconnectKeys;
}
function mount(panel){
 if(!panel||document.getElementById('terminalBehaviorSettings'))return;
 (panel.querySelector('[data-terminal-settings-panel="behavior"]')||panel).insertAdjacentHTML('beforeend',settingsHTML());
 const section=document.getElementById('terminalBehaviorSettings'),note=panel.querySelector('.terminal-transcript-note'),grid=section?.querySelector('.terminal-behavior-grid');if(note&&grid)grid.after(note);
 const confirm=document.getElementById('terminalConfirmSettings');if(confirm&&section&&confirm.parentElement===section.parentElement)section.after(confirm);sync();
}
function save(next){state.terminalBehavior=normalize(next);if(typeof saveState==='function')saveState();apply();sync();if(typeof renderTerminalChrome==='function')renderTerminalChrome();}
function changed(event){
 const field=event.target,key=field?.dataset?.terminalBehavior;if(!key||!field.closest?.('#terminalBehaviorSettings'))return;
 const next={...preferences()};
 if(field.getAttribute('type')==='checkbox')next[key]=field.checked;
 else if(key==='scrollback'){const lines=Number(field.value);if(!Number.isInteger(lines)||lines<LIMITS.scrollbackMin||lines>LIMITS.scrollbackMax){field.value=String(preferences().scrollback);return;}next.scrollback=lines;}
 else next[key]=field.value;
 save(next);
}
document.addEventListener('change',changed);
document.addEventListener('click',event=>{if(event.target.closest?.('[data-terminal-behavior-reset]')){event.preventDefault();save({});}});
root.NetworkToolkitTerminalBehavior=Object.freeze({limits:LIMITS,wordSeparators:WORD_SEPARATORS,normalize,preferences,options,displayBytes,observeOutput,ring,attach,apply,applyEntry,mount,sync,settingsHTML,sessionKey,keysHint});
if(typeof module==='object'&&module.exports)module.exports=root.NetworkToolkitTerminalBehavior;
})(typeof window!=='undefined'?window:globalThis);
