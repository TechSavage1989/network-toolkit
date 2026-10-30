(function (root) {
  'use strict';

  const MAX_PASTE_BYTES = 1024 * 1024;
  const FONT_SIZES = [10,11,12,13,14,16,18,20,22,24];
  const MENU_ITEMS = [
    {id:'copy',label:'Copy selection',icon:'copy',key:'Ctrl+Shift+C',group:'clipboard'},
    {id:'copy-line',label:'Copy line',icon:'copy',group:'clipboard'},
    {id:'copy-token',label:'Copy highlighted value',icon:'copy',group:'clipboard'},
    {id:'paste',label:'Paste',icon:'clipboard',key:'Ctrl+V',group:'clipboard'},
    {id:'scroll-top',label:'Scroll to top',icon:'arrowUp',group:'output'},
    {id:'scroll-bottom',label:'Scroll to bottom',icon:'arrowDown',group:'output'},
    {id:'paste-start',label:'Jump to paste start',icon:'outputStart',group:'output'},
    {id:'paste-previous',label:'Previous paste',icon:'arrowUp',group:'output'},
    {id:'paste-next',label:'Next paste',icon:'arrowDown',group:'output'},
    {id:'select-paste-output',label:'Select paste output',icon:'terminal',group:'output'},
    {id:'copy-paste-output',label:'Copy paste output',icon:'copy',group:'output'},
    {id:'copy-paste-all',label:'Copy commands + output',icon:'copy',group:'output'},
    {id:'select-all',label:'Select all output',icon:'terminal',group:'clipboard'},
    {id:'search',label:'Find in output',icon:'search',key:'Ctrl+Shift+F',group:'output'},
    {id:'clear',label:'Clear scrollback',icon:'trash',group:'output'},
    {id:'export',label:'Export transcript',icon:'download',group:'output'},
    {id:'zoom-in',label:'Larger text',icon:'plus',key:'Ctrl+wheel up',group:'display'},
    {id:'zoom-out',label:'Smaller text',icon:'minus',key:'Ctrl+wheel down',group:'display'},
    {id:'reset-font',label:'Reset text size',icon:'refresh',group:'display'},
    {id:'maximize',label:'Maximize / restore pane',icon:'maximize',group:'display'},
    {id:'control-c',label:'Send Ctrl+C',icon:'stop',group:'session'},
    {id:'pager-toggle',label:'Toggle auto pager',icon:'play',group:'session'},
    {id:'reconnect',label:'Reconnect',icon:'refresh',group:'session'},
    {id:'disconnect',label:'Disconnect',icon:'stop',group:'session'},
    {id:'move',label:'Detach / reattach',icon:'external',group:'session'}
  ];
  const DEFAULT_MENU = ['copy','copy-line','copy-token','paste','scroll-top','scroll-bottom','paste-start','copy-paste-output','select-all','search','export','zoom-in','zoom-out','control-c','move'];
  const bound = new WeakMap(), liveBindings = new Set(), pasteStarts = new Map();
  let initialized = false, observed = null, focusEpoch = 0, pasteSequence = 0, pendingNativePaste = null;
  let review = null, approvedPaste = false, menu = null, menuDestination = null, modalObserver = null;
  let copyQueue = Promise.resolve(false), copySequence = 0, latestCopy = 0;

  function safePreferences(value) {
    const v = value && typeof value === 'object' ? value : {};
    return {
      copyOnSelect:v.copyOnSelect !== false,
      copyOnClick:v.copyOnClick !== false,
      pasteReview:v.pasteReview !== false,
      pasteIncludeCommands:v.pasteIncludeCommands === true,
      autoPager:v.autoPager !== false,
      pagerKey:['auto','space','enter'].includes(v.pagerKey) ? v.pagerKey : 'auto',
      rightClick:v.rightClick === 'menu' ? 'menu' : 'paste',
      ctrlC:v.ctrlC === 'send' ? 'send' : 'copy-selection',
      menuItems:Array.isArray(v.menuItems) ? MENU_ITEMS.filter(item=>v.menuItems.includes(item.id)).map(item=>item.id) : DEFAULT_MENU.slice()
    };
  }
  function preferences() { return safePreferences(state.terminalInteraction); }
  function withoutCopyInput(entry,callback) {
    return root.NetworkToolkitTerminalCopy?.withInputSuppressed ? root.NetworkToolkitTerminalCopy.withInputSuppressed(entry,callback) : callback();
  }
  function currentContext() {
    const entry = currentTerminal();
    return {entry,xterm:entry?.xterm,page:state.activePage,pane:terminalPaneFocus,context:terminalPaneContext,
      revision:entry ? terminalSessionRevision(entry.info.id) : -1,state:entry?.info.state,
      owner:entry ? terminalOwnerCanMutate(entry) : false,online:online === true,
      transfer:!!(entry?.transferBusy || terminalWorkspaceTransferBusy || terminalDetachedReturning)};
  }
  function observeContext() {
    const next = currentContext();
    if (!observed || Object.keys(next).some(key=>observed[key] !== next[key])) { focusEpoch++; observed = next; }
    return focusEpoch;
  }
  function destination(entry) {
    observeContext();
    return entry?.xterm ? {entry,xterm:entry.xterm,id:entry.info.id,revision:terminalSessionRevision(entry.info.id),epoch:focusEpoch,label:entry.info.label || entry.info.host || entry.info.serialPort || 'Terminal'} : null;
  }
  function validDestination(token, input = true) {
    observeContext();
    const modal = document.querySelector('#modalRoot .modal');
    const modalAllowed = !modal || review?.destination === token && review.form.closest('.modal') === modal;
    return !!token && token.epoch === focusEpoch && terminalSessions.get(token.id) === token.entry &&
      token.entry.xterm === token.xterm && terminalSessionRevision(token.id) === token.revision &&
      state.activePage === 'terminal' && currentTerminal() === token.entry && terminalEntryVisible(token.entry) &&
      (!input || modalAllowed && terminalEntryAcceptsInput(token.entry) && !terminalWorkspaceTransferBusy && !terminalDetachedReturning);
  }
  function changedDestination() { toast('The terminal destination changed. Nothing was pasted. Focus the intended console and paste again.','error'); }
  function byteLength(text) { return new TextEncoder().encode(text).length; }
  function pasteStats(text) {
    const breaks = text.match(/\r\n|\r|\n/g) || [];
    return {bytes:byteLength(text),breaks:breaks.length,lines:breaks.length + 1,trailing:/[\r\n]$/.test(text)};
  }
  function reviewText() {
    const field = review?.form.querySelector('#terminalPasteText');
    return field ? field.value === review.editorOriginal ? review.original : field.value : '';
  }
  // Paced paste is an explicit command-list operation, never a replacement for
  // bracketed/raw paste into editors, shells or an unfinished command.
  function pacedPasteReason(token, text, target) {
    const entry=token?.entry, term=token?.xterm;
    if (!target || target.id!==token?.id || !Number.isSafeInteger(target.generation)) return 'Waiting for this connection. Reopen paste review when it is ready.';
    if (entry?.info.type!=='ssh' || !term || term.buffer.active!==term.buffer.normal || term.modes?.bracketedPasteMode) return 'Use raw paste for this terminal mode.';
    if (entry.inputSending || entry.inputBytes || entry.inputError || entry.renderPendingBytes || term.hasSelection?.()) return 'Finish pending input/output and clear the selection first, or use raw paste.';
    const b=term.buffer.active, line=b.getLine(b.baseY+b.cursorY)?.translateToString(false)||'';
    if (!/^[^\r\n]{0,160}[#>$%][ \t]*$/.test(line.slice(0,b.cursorX)) || line.slice(b.cursorX).trim()) return 'Start at an empty command prompt, or use raw paste.';
    if (byteLength(text)>65536) return 'Paced paste supports up to 64 KiB; use raw paste or a smaller list.';
    if (/[\x00-\x09\x0b\x0c\x0e-\x1f\x7f-\x9f]/.test(text)) return 'Control characters need raw paste; no commands have been changed.';
    const lines=text.replace(/\r\n?/g,'\n').split('\n');
    if (lines.pop()!=='') return 'End the last command with a line break, or use raw paste without adding Enter.';
    if (lines.length<2 || lines.length>1024) return 'Paced paste needs 2–1,024 complete lines.';
    return '';
  }
  function updateReview() {
    if (!review) return;
    if (!review.form.isConnected) { review = null; return; }
    if (!validDestination(review.destination)) review.invalid = true;
    const start=pasteStarts.get(review.destination.id);
    review.pending=!!(start&&start.generation===review.target?.generation&&start.pending);review.uncertain=!!(start&&start.generation===review.target?.generation&&start.uncertain);
    const value = reviewText(), stats = pasteStats(value), mode=review.form.querySelector('#terminalPasteMode'), paced=mode?.value==='paced', reason=pacedPasteReason(review.destination,value,review.target), blocked = review.invalid || review.pending || review.uncertain || stats.bytes > MAX_PASTE_BYTES || !value || paced && !!reason;
    const status = review.form.querySelector('#terminalPasteStatus');
    const count = review.form.querySelector('#terminalPasteStats');
    const submit = document.querySelector('#terminalPasteConfirm');
    count.textContent = stats.lines + (stats.lines === 1 ? ' line' : ' lines') + ' · ' + stats.breaks + (stats.breaks === 1 ? ' line break' : ' line breaks') + ' · ' + stats.bytes.toLocaleString() + ' bytes' + (stats.trailing ? ' · Ends with a line break' : '');
    status.textContent = review.invalid ? 'This destination is no longer ready or the focused pane changed. Nothing has been sent. Cancel and paste again in the intended console.' : stats.bytes > MAX_PASTE_BYTES ? 'This paste exceeds the 1 MiB limit. Reduce the text before sending.' : stats.breaks ? 'Line breaks may execute commands on the device. Review or edit every line before sending.' : 'The edited text contains no line breaks. It will be pasted without pressing Enter.';
    status.classList.toggle('terminal-paste-invalid', blocked && !!value);
    const hint=review.form.querySelector('#terminalPasteModeHint');
    if(hint)hint.textContent=review.error || (paced ? reason || 'Sends each complete line literally, then waits for a fresh prompt (30-second timeout, 100 ms stable output). Blank lines are kept. No auto pager. Stop or pause in Runs; stopping cannot undo commands already sent.' : 'Sends all text at once using the original paste behavior. Large command lists may overwhelm a device’s command echo.');
    if(review.pending)status.textContent='Start request sent. Closing this review does not stop commands. Check Runs for Pause or Stop; do not submit again.';
    if(review.uncertain)status.textContent='Start was not confirmed. Check Runs before sending anything again; some commands may have run.';
    const resolve=review.form.querySelector('#terminalPasteResolve');if(resolve)resolve.hidden=!review.uncertain;
    const dismiss=document.querySelector('#terminalPasteDismiss');if(dismiss)dismiss.textContent=review.pending||review.uncertain?'Close':'Cancel';
    if(mode)mode.disabled=!!review.pending||!!review.uncertain;
    const field=review.form.querySelector('#terminalPasteText');if(field)field.readOnly=!!review.pending||!!review.uncertain;
    if (submit) { submit.disabled = !!blocked; submit.textContent=paced?'Send lines · wait for prompt':'Paste all at once'; }
  }
  function openReview(token, text) {
    closeMenu(false);
    setTerminalToolsOpen(false,false);
    openModal('Review paste to ' + token.label,
      '<form id="terminalPasteForm" class="terminal-paste-review" data-terminal-id="' + esc(token.id) + '"><div class="terminal-paste-destination">' + icon('terminal') + '<span>Destination: <strong>' + esc(token.label) + '</strong></span></div><p id="terminalPasteStatus" class="terminal-paste-warning" role="status"></p><div class="field"><label for="terminalPasteText">Text to send</label><textarea id="terminalPasteText" class="mono" spellcheck="false" autocomplete="off" autocapitalize="off" aria-describedby="terminalPasteStats terminalPasteStatus"></textarea></div><div id="terminalPasteStats" class="terminal-paste-stats" aria-live="polite"></div><div class="field mt"><label for="terminalPasteMode">Send method</label><select id="terminalPasteMode" aria-describedby="terminalPasteModeHint"><option value="paced">One line at a time · wait for prompt</option><option value="raw">Raw paste · all at once</option></select><p id="terminalPasteModeHint" class="help-line" role="status"></p><button id="terminalPasteResolve" type="button" class="btn small ghost" hidden>I checked Runs · allow a new paste</button></div></form>',
      '<button id="terminalPasteDismiss" type="button" class="btn ghost" data-action="close-modal">Cancel</button><button id="terminalPasteConfirm" class="btn primary" type="submit" form="terminalPasteForm">Send to this console</button>',true);
    const form = document.querySelector('#terminalPasteForm'), field = form.querySelector('#terminalPasteText');
    field.value = text;
    const target=root.NetworkToolkitTerminalMacros?.pasteTarget?.(token.id);
    review = {destination:token,original:text,editorOriginal:field.value,form,invalid:false,target,pending:false,uncertain:false,error:''};
    form.querySelector('#terminalPasteMode').value=pacedPasteReason(token,text,target)?'raw':'paced';
    updateReview();
    requestAnimationFrame(()=>{ if (review?.form === form) { field.focus({preventScroll:true}); field.setSelectionRange(0,0); } });
  }
  function sendPaste(token, text) {
    const start=pasteStarts.get(token?.id),target=root.NetworkToolkitTerminalMacros?.pasteTarget?.(token?.id);
    if(start&&(start.pending||start.uncertain)&&(!target||start.generation===target.generation)){toast('Check the pending paced paste in Runs before pasting again.','error');return false;}
    if (!validDestination(token)) { changedDestination(); return false; }
    const binding = bound.get(token.entry);
    if (!binding || binding.xterm !== token.xterm || binding.disposed) return false;
    if (token.xterm.options.disableStdin) { toast('This console is waiting for queued input to finish. Try pasting again when it is ready.'); return false; }
    const bytes = byteLength(text), framing = token.xterm.modes?.bracketedPasteMode ? 12 : 0;
    if (bytes > MAX_PASTE_BYTES || (token.entry.inputBytes || 0) + bytes + framing > MAX_PASTE_BYTES) {
      toast('That paste exceeds the 1 MiB input limit. Reduce the text or wait for queued input to finish.','error');
      return false;
    }
    if (!text) return false;
    approvedPaste = true;
    try {
      // The accepted paste and its selection edit share the review gate.
      if (root.NetworkToolkitTerminalInput?.selection(token.entry) && !root.NetworkToolkitTerminalInput.deleteSelection(token.entry,bytes+framing)) return false;
      root.NetworkToolkitTerminalExtras?.notePaste?.(token.entry,text);
      binding.nativePaste(text);
    } finally { approvedPaste = false; }
    token.xterm.focus();
    return true;
  }
  function acceptText(token, text) {
    if (!validDestination(token)) { changedDestination(); return false; }
    if (typeof text !== 'string' || !text) return false;
    if (review?.form.isConnected) { toast('Finish or cancel the open paste review before pasting again.'); return false; }
    if (byteLength(text) > MAX_PASTE_BYTES) { toast('Paste no more than 1 MiB of text at a time.','error'); return false; }
    if (/[\r\n]/.test(text) && root.NetworkToolkitTerminalExtras.prefs(token.entry).pasteReview) { openReview(token,text); return false; }
    return sendPaste(token,text);
  }
  function pasteText(entry, text) {
    pasteSequence++;
    return acceptText(destination(entry),text);
  }
  async function paste(entry = currentTerminal()) {
    const token = destination(entry), sequence = ++pasteSequence;
    if (!validDestination(token)) { changedDestination(); return false; }
    if (review?.form.isConnected) { toast('Finish or cancel the open paste review before pasting again.'); return false; }
    try {
      await flushSelectionCopy(entry);
      if (sequence !== pasteSequence) return false;
      if (!validDestination(token)) { changedDestination(); return false; }
      if (!navigator.clipboard?.readText) throw new Error('Clipboard API unavailable');
      const text = await navigator.clipboard.readText();
      if (sequence !== pasteSequence) return false;
      if (!validDestination(token)) { changedDestination(); return false; }
      return acceptText(token,text);
    } catch {
      if (sequence === pasteSequence && validDestination(token)) {
        token.xterm.focus();
        toast('Clipboard access is unavailable here. Focus this console and press Ctrl+V to paste.');
      }
      return false;
    }
  }
  function submitPaste(form) {
    if (!review || review.form !== form) return false;
    updateReview();
    const value = reviewText(), token = review.destination;
    if (review.invalid || review.pending || review.uncertain || !value || byteLength(value) > MAX_PASTE_BYTES) return false;
    if(form.querySelector('#terminalPasteMode')?.value==='paced') {
      if(pacedPasteReason(token,value,review.target))return false;
      void submitPacedPaste(review,value);return true;
    }
    if (!sendPaste(token,value)) return false;
    form.querySelector('#terminalPasteText').value = '';
    closeModal();
    token.xterm.focus();
    review = null;
    return true;
  }
  async function submitPacedPaste(pending, text) {
    if(review!==pending || !validDestination(pending.destination))return;
    const start={generation:pending.target.generation,pending:true,uncertain:false};pasteStarts.set(pending.destination.id,start);
    pending.pending=true;pending.error='';updateReview();
    try {
      await root.NetworkToolkitTerminalMacros.startPacedPaste(pending.target,text,pending.destination.revision);
      if(pasteStarts.get(pending.destination.id)===start)pasteStarts.delete(pending.destination.id);
      // Never close a different dialog or send again after navigation/transfer.
      if(review===pending && pending.form.isConnected) {
        pending.form.querySelector('#terminalPasteText').value='';review=null;closeModal();
      }
    } catch(error) {
      pending.error=error.message||'Could not start paced paste.';
      pending.uncertain=!Number.isInteger(error.status) || error.status>=500;
      start.uncertain=pending.uncertain;
      if(!pending.uncertain&&pasteStarts.get(pending.destination.id)===start)pasteStarts.delete(pending.destination.id);
      if(review!==pending || !pending.form.isConnected)toast(pending.uncertain?'Paced paste start was not confirmed. Check Runs before sending again.':pending.error,'error');
    } finally {start.pending=false;pending.pending=false;if(review?.destination.id===pending.destination.id)updateReview();}
  }
  function suppressesInput() { return !!review && !approvedPaste; }

  function cancelSelectionCopy(entry) {
    const list = entry ? [bound.get(entry)] : liveBindings;
    for (const binding of list) if (binding) {
      clearTimeout(binding.selectionTimer);
      binding.selectionTimer = null;
      binding.blockedGesture = binding.selectionGesture;
    }
  }
  async function writeClipboard(value,entry) {
    try { if (navigator.clipboard?.writeText) { await navigator.clipboard.writeText(value); return true; } } catch {}
    const previous = document.activeElement, selection = root.getSelection?.(), ranges = [];
    if (selection) for (let i=0;i<selection.rangeCount;i++) ranges.push(selection.getRangeAt(i).cloneRange());
    const field = document.createElement('textarea');
    field.value = value; field.readOnly = true; field.tabIndex = -1; field.className = 'terminal-clipboard-transfer';
    field.setAttribute('aria-hidden','true');
    let copied = false;
    const transfer = () => {
      try { document.body.append(field); field.select(); copied = document.execCommand('copy') === true; } catch {}
      finally { field.remove(); if (previous?.isConnected) previous.focus({preventScroll:true}); if (selection && ranges.length) { selection.removeAllRanges(); for (const range of ranges) try { selection.addRange(range); } catch {} } }
    };
    if (entry && root.NetworkToolkitTerminalCopy?.withInputSuppressed) root.NetworkToolkitTerminalCopy.withInputSuppressed(entry,transfer); else transfer();
    return copied;
  }
  function copyValue(text, {entry = null,silent = false,automatic = false} = {}) {
    const value = String(text ?? '');
    if (!value) return Promise.resolve(false);
    if (!automatic) cancelSelectionCopy();
    const sequence = ++copySequence; latestCopy = sequence;
    copyQueue = copyQueue.catch(()=>false).then(async()=>{
      if (sequence !== latestCopy || entry && terminalSessions.get(entry.info.id) !== entry) return false;
      const copied = await writeClipboard(value,entry);
      if (!silent) toast(copied ? 'Copied to clipboard.' : 'Clipboard copy is unavailable. Try Ctrl+Shift+C with terminal text selected.',copied ? 'success' : 'error');
      return copied;
    });
    return copyQueue;
  }
  function copy(entry = currentTerminal()) {
    const text = entry?.xterm?.getSelection() || '';
    if (!text) { toast('Select terminal text to copy it.'); return Promise.resolve(false); }
    return copyValue(text,{entry});
  }
  function scheduleSelectionCopy(binding) {
    clearTimeout(binding.selectionTimer);
    if (!preferences().copyOnSelect || binding.disposed || binding.selecting || binding.selectionGesture <= binding.blockedGesture) return;
    const text = binding.xterm.getSelection(), gesture = binding.selectionGesture;
    if (!text || text === binding.lastCopiedSelection) return;
    binding.selectionTimer = setTimeout(()=>{
      if (binding.disposed || !preferences().copyOnSelect || binding.selectionGesture !== gesture || gesture <= binding.blockedGesture || currentTerminal() !== binding.entry || !terminalEntryVisible(binding.entry) || binding.xterm.getSelection() !== text) return;
      binding.lastCopiedSelection = text;
      binding.blockedGesture = gesture;
      void copyValue(text,{entry:binding.entry,silent:true,automatic:true});
    },90);
  }
  function flushSelectionCopy(entry) {
    const binding = bound.get(entry);
    if (binding && !binding.disposed && !binding.selecting && preferences().copyOnSelect &&
        binding.selectionGesture > binding.blockedGesture && currentTerminal() === entry && terminalEntryVisible(entry)) {
      const text = binding.xterm.getSelection();
      if (text && text !== binding.lastCopiedSelection) {
        clearTimeout(binding.selectionTimer); binding.selectionTimer = null;
        binding.lastCopiedSelection = text; binding.blockedGesture = binding.selectionGesture;
        return copyValue(text,{entry,silent:true,automatic:true});
      }
    }
    // A prior copy may already be in the asynchronous clipboard API. Preserve its
    // ordering before reading, even if the selection debounce has just fired.
    return copyQueue.catch(()=>false);
  }

  function setFontSize(size) {
    const value = Math.max(10,Math.min(24,Math.round(size)));
    if (state.terminalFontSize === value) return;
    state.terminalFontSize = value;
    const select = document.querySelector('#terminalFontSize');
    if (select) select.value = String(value);
    saveState(); updateTerminalTheme();
  }
  function zoom(direction) {
    const size = state.terminalFontSize || 13;
    const next = direction > 0 ? FONT_SIZES.find(value=>value > size) : [...FONT_SIZES].reverse().find(value=>value < size);
    if (next !== undefined) setFontSize(next);
  }
  function keyHandler(entry,event) {
    if (event.type !== 'keydown') return true;
    if (terminalPaneShortcut(event)) return false;
    const key = event.key.toLowerCase(), primary = event.ctrlKey || event.metaKey;
    if (primary && !event.altKey && key === 'c') {
      event.preventDefault(); event.stopPropagation();
      if (event.shiftKey || preferences().ctrlC === 'copy-selection' && entry.xterm.hasSelection()) void copy(entry).then(ok=>{if(ok)entry.xterm.clearSelection();});
      else enqueueTerminalInput(entry,'\x03');
      return false;
    }
    if (!event.altKey && (primary && key === 'v' || event.shiftKey && !primary && key === 'insert')) {
      // Returning false without cancelling the browser event preserves native paste even
      // when the asynchronous Clipboard API is unavailable in the native app window.
      pendingNativePaste = {destination:destination(entry),at:Date.now()};
      return false;
    }
    if (primary && event.shiftKey && !event.altKey && key === 'f') {
      event.preventDefault(); event.stopPropagation(); showTerminalSearch(); return false;
    }
    if (!primary && !event.altKey && !event.isComposing && (['backspace','delete'].includes(key) || Array.from(event.key).length === 1) && root.NetworkToolkitTerminalInput?.selection(entry)) {
      const deleted = root.NetworkToolkitTerminalInput.deleteSelection(entry);
      if (!deleted || ['backspace','delete'].includes(key)) { event.preventDefault(); event.stopPropagation(); return false; }
    }
    // Console control keys belong to the device, including Ctrl+W, Ctrl+R,
    // Ctrl+D and Ctrl+Z. Suppress the browser action and send exactly once.
    if (event.ctrlKey && !event.metaKey && !event.altKey) {
      const code = key.length === 1 && /^[a-z]$/.test(key) ? key.charCodeAt(0) & 31 :
        ({' ':0,'@':0,'[':27,'\\':28,']':29,'^':30,'_':31,'?':127})[key];
      if (code !== undefined) {
        event.preventDefault(); event.stopPropagation();
        const data = String.fromCharCode(code);
        enqueueTerminalInput(entry,data);
        return false;
      }
    }
    if (key === 'contextmenu' || event.shiftKey && key === 'f10') {
      event.preventDefault(); event.stopPropagation();
      const rect = entry.element.getBoundingClientRect(); openMenu(entry,rect.left+16,rect.top+16); return false;
    }
    return true;
  }

  function settingsHTML() {
    return '<section id="terminalInteractionSettings" class="terminal-tools-section terminal-interaction-settings"><h3>Mouse and keyboard defaults</h3><div class="terminal-interaction-fields"><label><input type="checkbox" id="terminalCopyOnSelect">Copy selected text automatically</label><label><input type="checkbox" id="terminalCopyOnClick">Click highlighted values to copy</label><label><input type="checkbox" id="terminalPasteReview">Review multiline pastes before sending</label><div class="field"><label for="terminalRightClick">Right-click in the console</label><select id="terminalRightClick"><option value="paste">Paste</option><option value="menu">Context menu</option></select><span class="hint">Shift + right-click temporarily does the opposite.</span></div><div class="field"><label for="terminalCtrlC">Ctrl+C behavior</label><select id="terminalCtrlC"><option value="send">Always send Ctrl+C to the device</option><option value="copy-selection">Copy selection; otherwise send Ctrl+C</option></select></div><label><input type="checkbox" id="terminalAutoPager">Automatically advance known More/pager prompts</label><div class="field"><label for="terminalPagerKey">Pager key</label><select id="terminalPagerKey"><option value="auto">Automatic</option><option value="space">Space</option><option value="enter">Enter</option></select></div></div><p class="terminal-interaction-help">Ctrl+Shift+C copies. Ctrl+V, Ctrl+Shift+V, and Shift+Insert paste. Ctrl+wheel changes console text size. Single-line pastes go directly to the console. Multiline review is optional and enabled by default. Session controls in the console bar can override Review paste and Auto pager for the focused session.</p><details id="terminalContextMenuSettings"><summary>Choose context-menu tools</summary><div class="terminal-menu-options">' + MENU_ITEMS.map(item=>'<label><input type="checkbox" data-terminal-menu-option="' + item.id + '">' + esc(item.label) + '</label>').join('') + '</div><p class="hint">Mouse and keyboard settings always remain in the menu.</p></details></section>';
  }
  function updateSettings() {
    const panel = document.querySelector('#terminalToolsPanel');
    if (panel && !panel.querySelector('#terminalInteractionSettings')) panel.insertAdjacentHTML('beforeend',settingsHTML());
    const p = preferences();
    for (const [id,value] of [['terminalCopyOnSelect',p.copyOnSelect],['terminalCopyOnClick',p.copyOnClick],['terminalPasteReview',p.pasteReview],['terminalAutoPager',p.autoPager],['terminalPagerKey',p.pagerKey],['terminalRightClick',p.rightClick],['terminalCtrlC',p.ctrlC]]) {
      const field = document.getElementById(id); if (!field) continue;
      if (field.type === 'checkbox') field.checked = value; else if (document.activeElement !== field) field.value = value;
    }
    for (const field of document.querySelectorAll('[data-terminal-menu-option]')) field.checked = p.menuItems.includes(field.dataset.terminalMenuOption);
  }
  function settingsChanged(event) {
    const field = event.target, p = preferences();
    if (field.id === 'terminalCopyOnSelect') { p.copyOnSelect = field.checked; if (!p.copyOnSelect) cancelSelectionCopy(); }
    else if (field.id === 'terminalCopyOnClick') p.copyOnClick = field.checked;
    else if (field.id === 'terminalPasteReview') p.pasteReview = field.checked;
    else if (field.id === 'terminalPasteIncludeCommands') p.pasteIncludeCommands = field.checked;
    else if (field.id === 'terminalAutoPager') p.autoPager = field.checked;
    else if (field.id === 'terminalPagerKey') p.pagerKey = field.value;
    else if (field.id === 'terminalRightClick') p.rightClick = field.value;
    else if (field.id === 'terminalCtrlC') p.ctrlC = field.value;
    else if (field.dataset.terminalMenuOption) p.menuItems = MENU_ITEMS.filter(item=>item.id === field.dataset.terminalMenuOption ? field.checked : p.menuItems.includes(item.id)).map(item=>item.id);
    else return;
    state.terminalInteraction = safePreferences(p); saveState();
    root.dispatchEvent(new CustomEvent('network-toolkit-terminal-preferences',{detail:preferences()}));
  }
  function menuEnabled(id,token) {
    if (!validDestination(token,false)) return false;
    const entry = token.entry;
    if (['paste','control-c','pager-toggle'].includes(id)) return validDestination(token);
    if (id === 'copy') return entry.xterm.hasSelection();
    if (id === 'copy-line') return !!token.copySnapshot?.line;
    if (id === 'copy-token') return !!token.copySnapshot?.token;
    if (id === 'disconnect') return terminalLive(entry) && terminalOwnerCanMutate(entry) && !entry.transferBusy;
    if (id === 'reconnect') return terminalReconnectAllowed(entry);
    if (id === 'maximize') return terminalPaneCount() > 1;
    if (id === 'zoom-in') return (state.terminalFontSize || 13) < 24;
    if (id === 'zoom-out') return (state.terminalFontSize || 13) > 10;
    if (id === 'move') { const button = document.querySelector('[data-terminal-action="move"]'); return !!button && !button.disabled; }
    return true;
  }
  function closeMenu(restoreFocus = true) {
    if (!menu) return;
    const token = menuDestination; menu.remove(); menu = null; menuDestination = null;
    if (restoreFocus && validDestination(token,false)) withoutCopyInput(token.entry,()=>token.xterm.focus());
  }
  function openMenu(entry,x,y,event) {
    closeMenu(false);
    const token = destination(entry); if (!validDestination(token,false)) return;
    token.copySnapshot = root.NetworkToolkitTerminalCopy?.context(entry,event) || null;
    const selected = preferences().menuItems; menuDestination = token;
    menu = document.createElement('div'); menu.id = 'terminalContextMenu'; menu.className = 'terminal-context-menu';
    menu.setAttribute('role','menu'); menu.setAttribute('aria-label','Terminal tools for ' + token.label);
    let previousGroup = '';
    menu.innerHTML = '<div class="terminal-context-label">' + esc(token.label) + '</div>' + MENU_ITEMS.filter(item=>selected.includes(item.id)).map(item=>{
      const separator = previousGroup && previousGroup !== item.group ? '<div role="separator" class="terminal-menu-separator"></div>' : ''; previousGroup = item.group;
      return separator + '<button type="button" role="menuitem" tabindex="-1" data-terminal-menu-action="' + item.id + '" ' + (menuEnabled(item.id,token) ? '' : 'disabled') + '>' + icon(item.icon) + '<span>' + esc(item.label) + '</span>' + (item.key ? '<kbd>' + esc(item.key) + '</kbd>' : '') + '</button>';
    }).join('') + '<div role="separator" class="terminal-menu-separator"></div><button type="button" role="menuitem" tabindex="-1" data-terminal-menu-action="settings">' + icon('settings') + '<span>Mouse and keyboard settings</span></button>';
    document.body.append(menu);
    const rect = menu.getBoundingClientRect();
    menu.style.left = Math.max(6,Math.min(x,innerWidth-rect.width-6)) + 'px';
    menu.style.top = Math.max(6,Math.min(y,innerHeight-rect.height-6)) + 'px';
    withoutCopyInput(entry,()=>menu.querySelector('button:not(:disabled)')?.focus({preventScroll:true}));
  }
  function runMenuAction(id) {
    const token = menuDestination, entry = token?.entry;
    if (!token || !validDestination(token,false)) { closeMenu(false); return; }
    if (id !== 'settings' && !menuEnabled(id,token)) return;
    closeMenu(false);
    switch (id) {
      case 'copy': void copy(entry); break;
      case 'copy-line': void root.NetworkToolkitTerminalCopy?.copyLine(entry,token.copySnapshot.line); break;
      case 'copy-token': void root.NetworkToolkitTerminalCopy?.copyToken(entry,token.copySnapshot.token); break;
      case 'paste': void paste(entry); break;
      case 'scroll-top': root.NetworkToolkitTerminalExtras?.scrollTop?.(entry); break;
      case 'scroll-bottom': root.NetworkToolkitTerminalExtras?.scrollBottom?.(entry); break;
      case 'paste-start': root.NetworkToolkitTerminalExtras?.jump?.(entry,'start',root.NetworkToolkitTerminalExtras.pasteChoice(entry)); break;
      case 'paste-previous': root.NetworkToolkitTerminalExtras?.step?.(entry,-1); break;
      case 'paste-next': root.NetworkToolkitTerminalExtras?.step?.(entry,1); break;
      case 'select-paste-output': root.NetworkToolkitTerminalExtras?.select?.(entry,root.NetworkToolkitTerminalExtras.prefs(entry).pasteIncludeCommands,root.NetworkToolkitTerminalExtras.pasteChoice(entry)); break;
      case 'copy-paste-output': void root.NetworkToolkitTerminalExtras?.copy?.(entry,root.NetworkToolkitTerminalExtras.prefs(entry).pasteIncludeCommands,root.NetworkToolkitTerminalExtras.pasteChoice(entry)); break;
      case 'copy-paste-all': void root.NetworkToolkitTerminalExtras?.copy?.(entry,true,root.NetworkToolkitTerminalExtras.pasteChoice(entry)); break;
      case 'select-all': entry.xterm.selectAll(); if (preferences().copyOnSelect) void copyValue(entry.xterm.getSelection(),{entry,silent:true}); break;
      case 'search': showTerminalSearch(); return;
      case 'clear': root.NetworkToolkitTerminalExtras.clear(entry); entry.xterm.clear(); entry.outputRevision = (entry.outputRevision || 0) + 1; break;
      case 'export': exportTerminalTranscript(entry); break;
      case 'zoom-in': zoom(1); break;
      case 'zoom-out': zoom(-1); break;
      case 'reset-font': setFontSize(13); break;
      case 'maximize': maximizeTerminalPane(terminalPaneIndex(entry.info.id)); break;
      case 'control-c': enqueueTerminalInput(entry,'\x03'); break;
      case 'pager-toggle': { const p=root.NetworkToolkitTerminalExtras.prefs(entry); entry._ntSessionPrefs={...entry._ntSessionPrefs,autoPager:!p.autoPager}; entry.outputRevision=(entry.outputRevision||0)+1; sync(); toast('Auto pager '+(!p.autoPager?'enabled.':'disabled.')); break; }
      case 'disconnect': void disconnectTerminal(entry); break;
      case 'reconnect': void reconnectTerminal(entry); return;
      case 'move': moveTerminalWindow(); return;
      case 'settings': setTerminalToolsOpen(true,false); document.querySelector('#terminalInteractionSettings')?.scrollIntoView({block:'nearest'}); document.querySelector('#terminalRightClick')?.focus({preventScroll:true}); return;
    }
    if (['copy','copy-line','copy-token','select-all'].includes(id)) withoutCopyInput(entry,()=>entry.xterm.focus());
    else if (id !== 'paste') entry.xterm.focus();
  }

  function bind(entry) {
    init();
    if (!entry?.xterm || !entry.element || bound.has(entry)) return;
    const xterm = entry.xterm, element = entry.element;
    const binding = {entry,xterm,nativePaste:xterm.paste.bind(xterm),disposed:false,selecting:false,selectionGesture:0,blockedGesture:0,lastCopiedSelection:'',selectionTimer:null,listeners:[]};
    bound.set(entry,binding); liveBindings.add(binding);
    const listen = (target,type,handler,options) => { target.addEventListener(type,handler,options); binding.listeners.push(()=>target.removeEventListener(type,handler,options)); };
    const wrappedPaste = text => pasteText(entry,text); xterm.paste = wrappedPaste;
    xterm.attachCustomKeyEventHandler(event=>keyHandler(entry,event));
    listen(element,'paste',event=>{
      event.preventDefault(); event.stopImmediatePropagation();
      const native = pendingNativePaste && Date.now()-pendingNativePaste.at < 5000 ? pendingNativePaste : null;
      pendingNativePaste = null; pasteSequence++;
      const token = native?.destination || destination(entry);
      if (!token || token.entry !== entry || !validDestination(token)) { changedDestination(); return; }
      const text = event.clipboardData?.getData('text/plain');
      if (typeof text === 'string') acceptText(token,text);
      else toast('The clipboard did not contain text.');
    },true);
    listen(element,'pointerdown',event=>{
      if (event.button === 0 && !event.target.closest('[data-terminal-copy-action]')) { binding.selecting = true; binding.selectionGesture++; binding.lastCopiedSelection = ''; clearTimeout(binding.selectionTimer); }
    },true);
    listen(element,'mousedown',event=>{
      if (event.target.closest('[data-terminal-copy-action]')) return;
      if (event.button !== 2) return;
      event.preventDefault(); event.stopImmediatePropagation();
      if (terminalEntryVisible(entry)) withoutCopyInput(entry,()=>selectTerminal(entry.info.id,true));
    },true);
    listen(element,'contextmenu',event=>{
      if (event.target.closest('[data-terminal-copy-action]')) return;
      event.preventDefault(); event.stopImmediatePropagation();
      if (!terminalEntryVisible(entry)) return;
      withoutCopyInput(entry,()=>selectTerminal(entry.info.id,true));
      const useMenu = (preferences().rightClick === 'menu') !== event.shiftKey;
      if (useMenu) openMenu(entry,event.clientX,event.clientY,event); else void paste(entry);
    },true);
    listen(element,'wheel',event=>{
      if (!event.ctrlKey) return;
      event.preventDefault(); event.stopImmediatePropagation();
      if (event.deltaY) zoom(event.deltaY < 0 ? 1 : -1);
    },{capture:true,passive:false});
    binding.selectionDisposable = xterm.onSelectionChange(()=>scheduleSelectionCopy(binding));
    entry.disposables.push({dispose(){
      binding.disposed = true; clearTimeout(binding.selectionTimer); binding.selectionDisposable.dispose();
      for (const remove of binding.listeners) remove();
      if (xterm.paste === wrappedPaste) xterm.paste = binding.nativePaste;
      liveBindings.delete(binding); bound.delete(entry); sync();
    }});
    sync();
  }
  function sync() {
    init(); observeContext(); updateSettings(); updateReview(); root.NetworkToolkitTerminalExtras?.sync?.();
    if (menu && !validDestination(menuDestination,false)) closeMenu(false);
  }
  function init() {
    if (initialized) return; initialized = true;
    document.addEventListener('click',event=>{if(event.target.id!=='terminalPasteResolve'||!review?.uncertain)return;const start=pasteStarts.get(review.destination.id);if(start?.pending)return;pasteStarts.delete(review.destination.id);review.pending=false;review.uncertain=false;review.error='';updateReview();});
    document.addEventListener('change',event=>{settingsChanged(event);if(event.target.id==='terminalPasteMode')updateReview();});
    document.addEventListener('input',event=>{ if (event.target.id === 'terminalPasteText') updateReview(); });
    document.addEventListener('submit',event=>{ if (event.target.id === 'terminalPasteForm') { event.preventDefault(); event.stopImmediatePropagation(); submitPaste(event.target); } },true);
    document.addEventListener('pointerup',()=>{ for (const binding of liveBindings) if (binding.selecting) { binding.selecting = false; scheduleSelectionCopy(binding); } });
    document.addEventListener('pointercancel',()=>{ for (const binding of liveBindings) { binding.selecting = false; cancelSelectionCopy(binding.entry); } });
    document.addEventListener('pointerdown',event=>{ if (menu && !menu.contains(event.target)) closeMenu(false); },true);
    document.addEventListener('click',event=>{ const button = event.target.closest('[data-terminal-menu-action]'); if (!button) return; event.preventDefault(); event.stopPropagation(); if (!button.disabled) runMenuAction(button.dataset.terminalMenuAction); });
    document.addEventListener('keydown',event=>{
      if (!menu) return;
      if (event.key === 'Escape') { event.preventDefault(); event.stopImmediatePropagation(); closeMenu(); return; }
      if (event.key === 'Tab') { closeMenu(); return; }
      if (!['ArrowDown','ArrowUp','Home','End'].includes(event.key)) return;
      event.preventDefault(); event.stopImmediatePropagation();
      const buttons = [...menu.querySelectorAll('button:not(:disabled)')], at = buttons.indexOf(document.activeElement);
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length-1 : (at + (event.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length;
      buttons[next]?.focus();
    },true);
    root.addEventListener('resize',()=>closeMenu(false));
    const modalRoot = document.querySelector('#modalRoot');
    if (modalRoot) {
      modalObserver = new MutationObserver(()=>{
        if (review?.form.isConnected) updateReview();
        else { review = null; focusEpoch++; }
      });
      modalObserver.observe(modalRoot,{childList:true});
    }
  }
  function noteInput(entry,data){if(!approvedPaste)root.NetworkToolkitTerminalExtras?.noteManualInput?.(entry,data);}
  root.NetworkToolkitTerminalInteraction = Object.freeze({init,bind,sync,preferences,safePreferences,paste,pasteText,submitPaste,suppressesInput,copy,copyValue,cancelSelectionCopy,settingsHTML,pasteStats,noteInput});
})(window);
