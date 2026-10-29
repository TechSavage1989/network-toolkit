/* Network Toolkit live terminal display handoff.
 *
 * Version-specific adjunct to the bundled xterm 6.0.0 and SerializeAddon
 * 0.14.0, upstream commit f447274f430fd22513f6adbf9862d19524471c04.
 * Reviewed source: src/common/{InputHandler,parser/*,input/TextDecoder,
 * buffer/*,services/*}, src/browser/services/ThemeService and
 * addons/addon-serialize/src/SerializeAddon.ts in that upstream commit.
 *
 * Capture is read-only. The caller must pause output/resize and await an
 * empty Uint8Array write barrier before capture or restore. Restore validates
 * first, then replaces destination display state while keeping its xterm,
 * addons and registered handlers. It never reconnects a transport. If restore
 * fails, keep input blocked and retry from the retained handoff snapshot.
 * Snapshots contain terminal output; keep them transient and never persist.
 */
(function (global) {
  'use strict';

  const VERSION = 'xterm-6.0.0/serialize-0.14.0/f447274f/1';
  const LIMITS = Object.freeze({ dataBytes: 8 * 1024 * 1024, emulatorBytes: 256 * 1024,
    requestBytes: 10 * 1024 * 1024, cols: 500, rows: 300, scrollback: 10000,
    attrs: 4096, runs: 65536, runBytes: 180 * 1024, combined: 4096, title: 4096,
    restoreMs: 15000 });
  const BOOL_MODES = ['applicationCursorKeys', 'applicationKeypad', 'bracketedPasteMode',
    'origin', 'reverseWraparound', 'sendFocus', 'synchronizedOutput', 'wraparound'];
  const MOUSE_PROTOCOLS = ['NONE', 'X10', 'VT200', 'DRAG', 'ANY'];
  const MOUSE_ENCODINGS = ['DEFAULT', 'SGR', 'SGR_PIXELS'];
  const HAS_EXTENDED = 0x10000000;
  const COMBINED = 0x200000;
  const restores = new WeakSet();

  function fail(code, message) {
    const error = new Error(message);
    error.code = 'terminal_snapshot_' + code;
    throw error;
  }
  function requireValue(test, name) {
    if (!test) fail('incompatible', 'The terminal display state is incompatible (' + name + ').');
  }
  function int(value, min, max, name) {
    requireValue(Number.isInteger(value) && value >= min && value <= max, name);
    return value;
  }
  function u32(value, name) { return int(value, 0, 0xffffffff, name); }
  function bits(value, name) {
    int(value, -0x80000000, 0xffffffff, name);
    return value >>> 0;
  }
  function bool(value, name) {
    requireValue(typeof value === 'boolean', name);
    return value;
  }
  function array(value, max, name) {
    requireValue(Array.isArray(value), name);
    if (value.length > max) fail('limit', 'The terminal display state exceeds its ' + name + ' limit.');
    return value;
  }
  function object(value, name) {
    requireValue(value !== null && typeof value === 'object' && !Array.isArray(value), name);
    return value;
  }
  function text(value, max, name) {
    requireValue(typeof value === 'string', name);
    if (value.length > max) fail('limit', 'The terminal display state exceeds its ' + name + ' limit.');
    return value;
  }
  function utf8Length(value, max) {
    let count = 0;
    for (let i = 0; i < value.length; i++) {
      const code = value.charCodeAt(i);
      if (code < 0x80) count++;
      else if (code < 0x800) count += 2;
      else if (code >= 0xd800 && code <= 0xdbff && i + 1 < value.length &&
        value.charCodeAt(i + 1) >= 0xdc00 && value.charCodeAt(i + 1) <= 0xdfff) { count += 4; i++; }
      else count += 3;
      if (count > max) return count;
    }
    return count;
  }
  function bytesFit(value, max, label) {
    if (utf8Length(value, max) > max) fail('limit', 'The terminal display state exceeds the ' + label + ' limit.');
  }
  function hashAdd(hash, value) {
    hash[0] = Math.imul(hash[0] ^ value, 16777619) >>> 0;
    hash[1] = (Math.imul(hash[1] ^ value, 2246822519) + 3266489917) >>> 0;
  }
  function charset(value) {
    if (value === undefined || value === null) return null;
    object(value, 'character set');
    const keys = Object.keys(value);
    requireValue(keys.length <= 128, 'character set size');
    const result = {};
    for (const key of keys) {
      requireValue(key.length === 1 && key.charCodeAt(0) >= 0x20 && key.charCodeAt(0) <= 0x7e, 'character set key');
      const replacement = text(value[key], 8, 'character set value');
      requireValue(replacement.length > 0 && !/[\x00-\x1f\x7f]/.test(replacement), 'character set value');
      result[key] = replacement;
    }
    return result;
  }
  function attr(value) {
    object(value, 'attributes');
    object(value.extended, 'extended attributes');
    requireValue(value.extended.urlId === 0, 'hyperlink state');
    return [bits(value.fg, 'foreground'), bits(value.bg, 'background'), bits(value.extended._ext, 'extended flags')];
  }
  function attrArray(value) {
    array(value, 3, 'attributes');
    requireValue(value.length === 3, 'attribute length');
    return value.map((v, i) => u32(v, 'attribute ' + i));
  }
  function applyAttr(target, value) {
    target.fg = value[0];
    target.bg = value[1];
    target.extended = target.extended.clone();
    target.extended.ext = value[2];
    target.extended.urlId = 0;
  }
  function modeState(service) {
    object(service.modes, 'modes');
    object(service.decPrivateModes, 'private modes');
    const modes = { insertMode: bool(service.modes.insertMode, 'insert mode') };
    for (const key of BOOL_MODES) modes[key] = bool(service.decPrivateModes[key], key);
    const blink = service.decPrivateModes.cursorBlink, style = service.decPrivateModes.cursorStyle;
    requireValue(blink === undefined || typeof blink === 'boolean', 'cursor blink');
    requireValue(style === undefined || ['block', 'underline', 'bar'].includes(style), 'cursor style');
    modes.cursorBlink = blink === undefined ? null : blink;
    modes.cursorStyle = style === undefined ? null : style;
    modes.cursorHidden = bool(service.isCursorHidden, 'cursor hidden');
    modes.cursorInitialized = bool(service.isCursorInitialized, 'cursor initialized');
    return modes;
  }
  function inspect(term) {
    requireValue(term && typeof term.write === 'function' && typeof term.reset === 'function' &&
      typeof term.resize === 'function' && typeof term.refresh === 'function', 'terminal API');
    const core = object(term._core, 'xterm core'), input = object(core._inputHandler, 'input handler');
    const parser = object(input._parser, 'parser'), params = object(parser._params, 'parser parameters');
    const buffers = object(core._bufferService && core._bufferService.buffers, 'buffers');
    requireValue(params.params instanceof Int32Array && params.params.length === 32 &&
      params._subParams instanceof Int32Array && params._subParams.length === 32 &&
      params._subParamsIdx instanceof Uint16Array && params._subParamsIdx.length === 32 &&
      params.maxLength === 32 && params.maxSubParamsLength === 32, 'pinned parameter layout');
    requireValue(parser.initialState === 0 && parser._transitions &&
      parser._transitions.table instanceof Uint8Array && parser._transitions.table.length === 4095 &&
      typeof parser.reset === 'function', 'pinned parser layout');
    int(parser.currentState, 0, 13, 'parser state');
    object(parser._parseStack, 'parser continuation');
    object(input._parseStack, 'input continuation');
    for (const name of ['_oscParser', '_dcsParser']) {
      const subparser = object(parser[name], 'string parser');
      requireValue(subparser._stack && typeof subparser._stack.paused === 'boolean' &&
        Array.isArray(subparser._active), 'string parser continuation');
    }
    requireValue(input._utf8Decoder && input._utf8Decoder.interim instanceof Uint8Array &&
      input._utf8Decoder.interim.length === 3 && typeof input._utf8Decoder.clear === 'function' &&
      input._stringDecoder && typeof input._stringDecoder.clear === 'function', 'decoder layout');
    const write = object(core._writeBuffer, 'write buffer');
    requireValue(Array.isArray(write._writeBuffer) && Number.isInteger(write._bufferOffset) &&
      write._bufferOffset >= 0 && write._bufferOffset <= write._writeBuffer.length &&
      Number.isInteger(write._pendingData) && write._pendingData >= 0, 'write queue layout');
    requireValue(core.unicodeService && core.unicodeService.activeVersion === '6', 'Unicode provider');
    const chars = object(core._charsetService, 'charset service');
    requireValue(Array.isArray(chars._charsets) && chars._charsets.length <= 4, 'character set slots');
    const service = object(core.coreService, 'core service'), mouse = object(core.coreMouseService, 'mouse service');
    modeState(service);
    requireValue(MOUSE_PROTOCOLS.includes(mouse.activeProtocol) && MOUSE_ENCODINGS.includes(mouse.activeEncoding), 'mouse mode');
    const theme = object(core._themeService, 'theme service');
    requireValue(theme.colors && theme._restoreColors && Array.isArray(theme.colors.ansi) &&
      theme.colors.ansi.length === 256 && Array.isArray(theme._restoreColors.ansi) &&
      theme._restoreColors.ansi.length === 256 && typeof theme.modifyColors === 'function' &&
      theme.colors.contrastCache && typeof theme.colors.contrastCache.clear === 'function' &&
      theme.colors.halfContrastCache && typeof theme.colors.halfContrastCache.clear === 'function', 'theme layout');
    requireValue(typeof input._windowTitle === 'string' && typeof input._iconName === 'string' &&
      Array.isArray(input._windowTitleStack) && Array.isArray(input._iconNameStack), 'title state');
    bool(core._bufferService.isUserScrolling, 'scroll follow state');
    attr(input._curAttrData); attr(input._eraseAttrDataInternal);
    requireValue(typeof input._curAttrData.extended.clone === 'function', 'attribute clone');
    for (const name of ['normal', 'alt']) {
      const buffer = object(buffers[name], name + ' buffer');
      requireValue(buffer.lines && typeof buffer.lines.get === 'function' &&
        typeof buffer.getBlankLine === 'function', 'buffer line layout');
    }
    requireValue(buffers.active === buffers.normal || buffers.active === buffers.alt, 'active buffer');
    return { term, core, input, parser, params, buffers, write, chars, service, mouse, theme };
  }
  function drained(runtime) {
    const { input, parser, write } = runtime;
    if (input._parseStack.paused || parser._parseStack.state !== 0 || write._isSyncWriting ||
      write._pendingData !== 0 || write._writeBuffer.slice(write._bufferOffset).some(chunk => chunk.length !== 0)) {
      fail('busy', 'Terminal output is still being parsed. Let the current output finish, then try moving the session again.');
    }
  }
  function safeBoundary(runtime) {
    const parser = runtime.parser;
    if (parser.currentState > 6 || runtime.service.decPrivateModes.synchronizedOutput ||
      parser._oscParser._stack.paused || parser._dcsParser._stack.paused ||
      parser._oscParser._active.length || parser._dcsParser._active.length) {
      fail('busy', 'The device is in the middle of a terminal control string or screen update. Let it finish, then try moving the session again.');
    }
  }
  function bufferState(buffer, cols, rows, normal) {
    const length = int(buffer.lines.length, normal ? rows : 0, (normal ? LIMITS.scrollback : 0) + rows, 'buffer length');
    const tabs = Object.keys(buffer.tabs).filter(key => buffer.tabs[key]).map(key => {
      requireValue(/^(0|[1-9][0-9]*)$/.test(key), 'tab stop');
      return int(Number(key), 0, LIMITS.cols - 1, 'tab stop');
    }).sort((a, b) => a - b);
    return { length, x: int(buffer.x, 0, cols, 'cursor column'), y: int(buffer.y, 0, rows - 1, 'cursor row'),
      ybase: int(buffer.ybase, 0, LIMITS.scrollback, 'scrollback position'),
      ydisp: int(buffer.ydisp, 0, buffer.ybase, 'viewport position'),
      scrollTop: int(buffer.scrollTop, 0, rows - 1, 'scroll top'),
      scrollBottom: int(buffer.scrollBottom, buffer.scrollTop, rows - 1, 'scroll bottom'),
      savedX: int(buffer.savedX, 0, LIMITS.cols, 'saved column'),
      savedY: int(buffer.savedY, 0, LIMITS.scrollback + LIMITS.rows, 'saved row'),
      savedAttr: attr(buffer.savedCurAttrData), savedCharset: charset(buffer.savedCharset), tabs };
  }
  function parserState(runtime) {
    const p = runtime.params, length = int(p.length, 0, 32, 'parameter count');
    const subLength = int(p._subParamsLength, 0, 32, 'subparameter count');
    return { state: runtime.parser.currentState, collect: bits(runtime.parser._collect, 'parser collect'),
      preceding: bits(runtime.parser.precedingJoinState, 'preceding Unicode state'),
      params: Array.from(p.params.subarray(0, length)),
      subParams: Array.from(p._subParams.subarray(0, subLength)),
      subIndices: Array.from(p._subParamsIdx.subarray(0, length)),
      rejectDigits: bool(p._rejectDigits, 'parameter overflow'),
      rejectSubDigits: bool(p._rejectSubDigits, 'subparameter overflow'),
      digitIsSub: bool(p._digitIsSub, 'subparameter continuation'),
      utf8: Array.from(runtime.input._utf8Decoder.interim),
      utf16: int(runtime.input._stringDecoder._interim, 0, 0xdbff, 'UTF16 continuation') };
  }
  function putVarint(bytes, value) {
    do { const part = value % 128; value = Math.floor(value / 128); bytes.push(part | (value ? 128 : 0)); } while (value);
    if (bytes.length > LIMITS.runBytes) fail('limit', 'This terminal has too many distinct styled spans to move safely.');
  }
  function encodeBytes(bytes) {
    let raw = '';
    for (let i = 0; i < bytes.length; i += 4096) raw += String.fromCharCode.apply(null, bytes.slice(i, i + 4096));
    return global.btoa(raw);
  }
  function readBufferData(runtime, cols, recordAttributes) {
    const hash = [2166136261, 0x9e3779b9], palette = [], ids = new Map(), bytes = [];
    let offset = 0, runStart = 0, runLength = 0, runId = 0, previousEnd = 0, runCount = 0;
    let estimated = 1024, previousStyle = '', previousEmpty = false;
    function flushRun() {
      if (!runLength || !runId) { runLength = 0; return; }
      if (++runCount > LIMITS.runs) fail('limit', 'This terminal has too many styled spans to move safely.');
      putVarint(bytes, runStart - previousEnd); putVarint(bytes, runLength); putVarint(bytes, runId);
      previousEnd = runStart + runLength; runLength = 0;
    }
    for (const buffer of [runtime.buffers.normal, runtime.buffers.alt]) {
      hashAdd(hash, buffer.lines.length);
      for (let y = 0; y < buffer.lines.length; y++) {
        const line = buffer.lines.get(y);
        requireValue(line && line._data instanceof Uint32Array && line._data.length >= cols * 3 &&
          line.length === cols && typeof line.isWrapped === 'boolean', 'pinned cell layout');
        hashAdd(hash, Number(line.isWrapped));
        estimated += line.isWrapped ? cols + 100 : 64;
        for (let x = 0; x < cols; x++, offset++) {
          const content = line._data[x * 3], width = content >>> 22;
          requireValue(width <= 2, 'cell width');
          let value;
          if (content & COMBINED) {
            value = text(line._combined[x], LIMITS.combined, 'combined character');
          } else {
            const code = content & 0x1fffff;
            requireValue(code <= 0x10ffff, 'cell code point');
            value = code ? String.fromCodePoint(code) : '';
          }
          hashAdd(hash, width); hashAdd(hash, value.length);
          for (let n = 0; n < value.length; n++) hashAdd(hash, value.charCodeAt(n));
          if (!recordAttributes) continue;
          const fg = line._data[x * 3 + 1], bg = line._data[x * 3 + 2];
          let ext = 0;
          if (bg & HAS_EXTENDED) {
            const attributes = object(line._extendedAttrs[x], 'cell extended attributes');
            requireValue(attributes.urlId === 0, 'cell hyperlink');
            ext = bits(attributes._ext, 'cell extended flags');
          }
          const style = fg + ':' + bg + ':' + ext;
          if (value) estimated += utf8Length(value, LIMITS.dataBytes);
          if (style !== previousStyle) estimated += 160;
          if (!value && !previousEmpty) estimated += 64;
          previousStyle = style; previousEmpty = !value;
          if (estimated > LIMITS.dataBytes) fail('limit', 'This terminal display is too large or too densely styled to move safely.');
          let id = 0;
          if (fg || bg || ext) {
            id = ids.get(style);
            if (id === undefined) {
              if (palette.length >= LIMITS.attrs) fail('limit', 'This terminal uses too many distinct text styles to move safely.');
              palette.push([fg, bg, ext]); id = palette.length; ids.set(style, id);
            }
          }
          if (id === runId && runStart + runLength === offset) runLength++;
          else { flushRun(); runStart = offset; runLength = 1; runId = id; }
        }
      }
    }
    flushRun();
    return { hash, palette, runs: encodeBytes(bytes) };
  }
  function paletteState(theme) {
    const current = theme.colors, defaults = theme._restoreColors, changes = [];
    for (const slot of ['foreground', 'background', 'cursor']) {
      const rgba = u32(current[slot].rgba, 'theme color'), original = u32(defaults[slot].rgba, 'theme default');
      if (rgba !== original) changes.push([slot, rgba]);
    }
    for (let i = 0; i < 256; i++) {
      const rgba = u32(current.ansi[i].rgba, 'ANSI color'), original = u32(defaults.ansi[i].rgba, 'ANSI default');
      if (rgba !== original) changes.push([i, rgba]);
    }
    return changes;
  }
  // Public xterm selection APIs only. Coordinates stay in the transient,
  // already bounded emulator checkpoint; no additional output text is copied.
  function selectionHash(value) {
    let a=2166136261,b=5381;
    for(let i=0;i<value.length;i++){a=Math.imul(a^value.charCodeAt(i),16777619);b=Math.imul(b,33)^value.charCodeAt(i);}
    return [a>>>0,b>>>0];
  }
  function validateSelection(value,cols,length) {
    if(value==null)return null;
    object(value,'selection');
    const point=p=>{object(p,'selection point');return {x:int(p.x,0,cols,'selection column'),y:int(p.y,0,length-1,'selection row')};};
    const start=point(value.start),end=point(value.end),hash=array(value.hash,2,'selection hash').map(v=>u32(v,'selection hash'));
    requireValue(hash.length===2&&(end.y>start.y||end.y===start.y&&end.x>start.x),'selection extent');
    return {start,end,hash,length:int(value.length,1,LIMITS.dataBytes,'selected text length')};
  }
  function captureSelection(term) {
    const position=term.getSelectionPosition?.();if(!position)return null;
    const selected=term.getSelection?.()||'';if(!selected)return null;
    return validateSelection({...position,hash:selectionHash(selected),length:selected.length},term.cols,term.buffer.active.length);
  }
  function applySelection(term,value,expected) {
    if(!value){term.clearSelection?.();return;}
    requireValue(typeof term.select==='function'&&typeof term.getSelection==='function','public selection API');
    term.select(value.start.x,value.start.y,(value.end.y-value.start.y)*term.cols+value.end.x-value.start.x);
    const selected=term.getSelection(),hash=selectionHash(selected);
    const matches=selected.length===value.length&&hash[0]===value.hash[0]&&hash[1]===value.hash[1]&&(expected===undefined||selected===expected);
    if(!matches)term.clearSelection();
    requireValue(matches,'restored selected output');
  }
  function selectionLineWidth(buffer,y,cols) {
    const line=buffer.getLine(y),next=buffer.getLine(y+1);
    // A wide character moved to the next wrapped row leaves one empty pad.
    return next?.isWrapped&&next.getCell(0)?.getWidth()===2&&line?.getCell(cols-1)?.getCode()===0?cols-1:cols;
  }
  function selectionOffset(buffer,start,point,cols) {
    let count=0;
    for(let y=start;y<=point.y;y++){const line=buffer.getLine(y),end=y===point.y?point.x:selectionLineWidth(buffer,y,cols);for(let x=0;x<end;x++){const cell=line?.getCell(x);if(cell?.getWidth()!==0)count+=(cell?.getChars()||' ').length;}}
    return count;
  }
  function beforeResize(term) {
    const selection=captureSelection(term);if(!selection)return null;
    const buffer=term.buffer.active;if(buffer!==term.buffer.normal)return {selection,alternate:true,buffer,text:term.getSelection()};
    const anchors=[];
    try{for(const point of [selection.start,selection.end]){let y=point.y;while(y>0&&buffer.getLine(y)?.isWrapped)y--;const marker=term.registerMarker(y-buffer.baseY-buffer.cursorY);requireValue(marker&&!marker.isDisposed,'selection resize anchor');anchors.push({marker,offset:selectionOffset(buffer,y,point,term.cols)});}return {selection,anchors,buffer,text:term.getSelection()};}
    catch(error){for(const a of anchors)a.marker.dispose();throw error;}
  }
  function selectionPoint(buffer,anchor,cols) {
    requireValue(!anchor.marker.isDisposed&&anchor.marker.line>=0,'retained selection anchor');
    let remaining=anchor.offset,y=anchor.marker.line;
    for(;y<buffer.length;y++){
      const line=buffer.getLine(y),width=selectionLineWidth(buffer,y,cols);
      for(let x=0;x<width;x++){if(remaining===0)return {x,y};const cell=line.getCell(x);if(cell?.getWidth()!==0)remaining-=(cell?.getChars()||' ').length;requireValue(remaining>=0,'selection character boundary');}
      if(remaining===0)return {x:width,y};
      requireValue(buffer.getLine(y+1)?.isWrapped,'selection logical line');
    }
    fail('selection','The selected output was trimmed during terminal resizing.');
  }
  function afterResize(term,position) {
    if(!position)return;
    try{requireValue(term.buffer.active===position.buffer,'same selection buffer');const value=position.alternate?position.selection:{...position.selection,start:selectionPoint(term.buffer.active,position.anchors[0],term.cols),end:selectionPoint(term.buffer.active,position.anchors[1],term.cols)};applySelection(term,value,position.text);}
    catch(error){term.clearSelection?.();throw error;}
    finally{for(const a of position.anchors||[])a.marker.dispose();}
  }

  function capture(term, serializeAddon) {
    if (restores.has(term)) fail('busy', 'This terminal display is still being restored.');
    const r = inspect(term); drained(r); safeBoundary(r);
    requireValue(serializeAddon && typeof serializeAddon.serialize === 'function' &&
      serializeAddon._terminal === term, 'serializer binding');
    const cols = int(term.cols, 2, LIMITS.cols, 'columns'), rows = int(term.rows, 2, LIMITS.rows, 'rows');
    const normal = bufferState(r.buffers.normal, cols, rows, true), alternate = bufferState(r.buffers.alt, cols, rows, false);
    const content = readBufferData(r, cols, true);
    const emulator = { version: VERSION, active: r.buffers.active === r.buffers.alt ? 'alternate' : 'normal',
      normal, alternate, attrs: content.palette, runs: content.runs, contentHash: content.hash,
      attr: attr(r.input._curAttrData), eraseAttr: attr(r.input._eraseAttrDataInternal),
      parser: parserState(r), modes: modeState(r.service),
      charset: { level: r.chars.glevel, current: charset(r.chars.charset),
        slots: Array.from({ length: 4 }, (_, i) => charset(r.chars._charsets[i])) },
      mouse: { protocol: r.mouse.activeProtocol, encoding: r.mouse.activeEncoding },
      titles: { window: r.input._windowTitle, icon: r.input._iconName,
        windows: r.input._windowTitleStack.slice(), icons: r.input._iconNameStack.slice() },
      palette: paletteState(r.theme), scrollback: int(term.options.scrollback, 0, LIMITS.scrollback, 'scrollback'),
      convertEol: bool(term.options.convertEol, 'line ending mode'),
      userScrolling: bool(r.core._bufferService.isUserScrolling, 'scroll follow state'), selection: captureSelection(term) };
    // Validate the bounded adjunct before allocating the serialized screen.
    validateEmulator(emulator, cols, rows);
    const data = serializeAddon.serialize({ excludeModes: true });
    return validate({ data, emulator, cols, rows, viewportY: r.buffers.active.ydisp });
  }
  function validateBuffer(value, cols, rows, normal) {
    object(value, 'buffer state');
    const length = int(value.length, normal ? rows : 0, rows + (normal ? LIMITS.scrollback : 0), 'buffer length');
    const ybase = int(value.ybase, 0, normal ? LIMITS.scrollback : 0, 'buffer base');
    requireValue(!length || ybase + rows <= length, 'buffer extent');
    const tabs = array(value.tabs, LIMITS.cols, 'tab stops').map(v => int(v, 0, LIMITS.cols - 1, 'tab stop'));
    requireValue(tabs.every((v, i) => !i || v > tabs[i - 1]), 'tab stop ordering');
    return { length, ybase, x: int(value.x, 0, cols, 'cursor column'), y: int(value.y, 0, rows - 1, 'cursor row'),
      ydisp: int(value.ydisp, 0, ybase, 'buffer viewport'),
      scrollTop: int(value.scrollTop, 0, rows - 1, 'scroll top'),
      scrollBottom: int(value.scrollBottom, value.scrollTop, rows - 1, 'scroll bottom'),
      savedX: int(value.savedX, 0, LIMITS.cols, 'saved column'),
      savedY: int(value.savedY, 0, LIMITS.scrollback + LIMITS.rows, 'saved row'),
      savedAttr: attrArray(value.savedAttr), savedCharset: charset(value.savedCharset), tabs };
  }
  function validateParser(value) {
    object(value, 'parser state');
    const params = array(value.params, 32, 'parameters').map(v => int(v, -1, 0x7fffffff, 'parameter'));
    const subs = array(value.subParams, 32, 'subparameters').map(v => int(v, -1, 0x7fffffff, 'subparameter'));
    const indices = array(value.subIndices, 32, 'subparameter indices').map(v => int(v, 0, 0xffff, 'subparameter index'));
    requireValue(indices.length === params.length && indices.every(v => (v >>> 8) <= (v & 255) && (v & 255) <= subs.length), 'subparameter ranges');
    const utf8 = array(value.utf8, 3, 'UTF8 continuation').map(v => int(v, 0, 255, 'UTF8 byte'));
    requireValue(utf8.length === 3, 'UTF8 continuation size');
    const utf16 = int(value.utf16, 0, 0xdbff, 'UTF16 continuation');
    requireValue(utf16 === 0 || utf16 >= 0xd800, 'UTF16 high surrogate');
    return { state: int(value.state, 0, 6, 'safe parser state'), collect: u32(value.collect, 'collect'),
      preceding: u32(value.preceding, 'preceding Unicode state'), params, subParams: subs, subIndices: indices,
      rejectDigits: bool(value.rejectDigits, 'parameter overflow'),
      rejectSubDigits: bool(value.rejectSubDigits, 'subparameter overflow'),
      digitIsSub: bool(value.digitIsSub, 'subparameter continuation'), utf8, utf16 };
  }
  function decodeRuns(encoded, cells, paletteLength) {
    text(encoded, Math.ceil(LIMITS.runBytes / 3) * 4, 'style runs');
    requireValue(/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(encoded), 'style run encoding');
    const raw = global.atob(encoded), result = [];
    requireValue(global.btoa(raw) === encoded, 'canonical style encoding');
    if (raw.length > LIMITS.runBytes) fail('limit', 'The styled display exceeds its handoff limit.');
    let pos = 0, previousEnd = 0;
    function read() {
      let value = 0, multiplier = 1;
      for (let count = 0; count < 5; count++) {
        requireValue(pos < raw.length, 'truncated style run');
        const next = raw.charCodeAt(pos++); value += (next & 127) * multiplier;
        requireValue(value <= 0xffffffff, 'style run integer');
        if (!(next & 128)) { requireValue(count === 0 || next !== 0, 'canonical style integer'); return value; }
        multiplier *= 128;
      }
      fail('incompatible', 'The terminal style run is malformed.');
    }
    while (pos < raw.length) {
      if (result.length >= LIMITS.runs) fail('limit', 'The styled display has too many handoff spans.');
      const start = previousEnd + read(), length = read(), id = read();
      requireValue(length > 0 && start + length <= cells && id > 0 && id <= paletteLength, 'style run bounds');
      result.push([start, length, id]); previousEnd = start + length;
    }
    return result;
  }
  function validateEmulator(value, cols, rows) {
    object(value, 'emulator');
    requireValue(value.version === VERSION, 'snapshot version');
    requireValue(value.active === 'normal' || value.active === 'alternate', 'active screen');
    const normal = validateBuffer(value.normal, cols, rows, true), alternate = validateBuffer(value.alternate, cols, rows, false);
    requireValue(value.active === 'alternate' ? alternate.length === rows : alternate.length === 0, 'alternate screen extent');
    const attrs = array(value.attrs, LIMITS.attrs, 'text styles').map(attrArray);
    const runs = text(value.runs, Math.ceil(LIMITS.runBytes / 3) * 4, 'style runs');
    decodeRuns(runs, (normal.length + alternate.length) * cols, attrs.length);
    const contentHash = array(value.contentHash, 2, 'content hash').map(v => u32(v, 'content hash'));
    requireValue(contentHash.length === 2, 'content hash size');
    const modes = {}, source = object(value.modes, 'modes');
    for (const key of ['insertMode', ...BOOL_MODES, 'cursorHidden', 'cursorInitialized']) modes[key] = bool(source[key], key);
    requireValue(!modes.synchronizedOutput, 'complete screen update');
    requireValue(source.cursorBlink === null || typeof source.cursorBlink === 'boolean', 'cursor blink');
    requireValue(source.cursorStyle === null || ['block', 'underline', 'bar'].includes(source.cursorStyle), 'cursor style');
    modes.cursorBlink = source.cursorBlink; modes.cursorStyle = source.cursorStyle;
    const chars = object(value.charset, 'charsets'), slots = array(chars.slots, 4, 'character set slots').map(charset);
    requireValue(slots.length === 4, 'character set slot count');
    const mouse = object(value.mouse, 'mouse');
    requireValue(MOUSE_PROTOCOLS.includes(mouse.protocol) && MOUSE_ENCODINGS.includes(mouse.encoding), 'mouse mode');
    const titles = object(value.titles, 'titles');
    const palette = array(value.palette, 259, 'palette overrides').map(entry => {
      array(entry, 2, 'palette entry'); requireValue(entry.length === 2, 'palette entry size');
      const slot = entry[0];
      requireValue(['foreground', 'background', 'cursor'].includes(slot) || Number.isInteger(slot) && slot >= 0 && slot < 256, 'palette slot');
      return [slot, u32(entry[1], 'palette color')];
    });
    requireValue(new Set(palette.map(entry => entry[0])).size === palette.length, 'palette duplicates');
    const result = { version: VERSION, active: value.active, normal, alternate, attrs, runs, contentHash,
      attr: attrArray(value.attr), eraseAttr: attrArray(value.eraseAttr), parser: validateParser(value.parser), modes,
      charset: { level: int(chars.level, 0, 3, 'character set level'), current: charset(chars.current), slots },
      mouse: { protocol: mouse.protocol, encoding: mouse.encoding },
      titles: { window: text(titles.window, LIMITS.title, 'window title'), icon: text(titles.icon, LIMITS.title, 'icon title'),
        windows: array(titles.windows, 10, 'window title stack').map(v => text(v, LIMITS.title, 'window title')),
        icons: array(titles.icons, 10, 'icon title stack').map(v => text(v, LIMITS.title, 'icon title')) },
      palette, scrollback: int(value.scrollback, 0, LIMITS.scrollback, 'scrollback'),
      convertEol: bool(value.convertEol, 'line ending mode'),
      userScrolling: bool(value.userScrolling, 'scroll follow state'),
      selection: validateSelection(value.selection,cols,(value.active==='normal'?normal:alternate).length) };
    requireValue(normal.length <= rows + result.scrollback, 'scrollback capacity');
    bytesFit(JSON.stringify(result), LIMITS.emulatorBytes, '256 KiB emulator');
    return result;
  }
  function validate(snapshot) {
    object(snapshot, 'snapshot');
    const cols = int(snapshot.cols, 2, LIMITS.cols, 'columns'), rows = int(snapshot.rows, 2, LIMITS.rows, 'rows');
    const data = text(snapshot.data, LIMITS.dataBytes, 'screen data');
    bytesFit(data, LIMITS.dataBytes, '8 MiB screen');
    const emulator = validateEmulator(snapshot.emulator, cols, rows);
    const viewportY = int(snapshot.viewportY, 0, emulator[emulator.active === 'normal' ? 'normal' : 'alternate'].ybase, 'viewport');
    const result = { data, emulator, cols, rows, viewportY };
    bytesFit(JSON.stringify(result), LIMITS.requestBytes, '10 MiB handoff');
    return result;
  }
  function applyBuffer(buffer, value) {
    for (const key of ['x', 'y', 'ybase', 'ydisp', 'scrollTop', 'scrollBottom', 'savedX', 'savedY']) buffer[key] = value[key];
    applyAttr(buffer.savedCurAttrData, value.savedAttr);
    buffer.savedCharset = value.savedCharset === null ? undefined : value.savedCharset;
    buffer.tabs = {};
    for (const col of value.tabs) buffer.tabs[col] = true;
  }
  function applyStyles(runtime, state, cols) {
    const buffers = [runtime.buffers.normal, runtime.buffers.alt], lines = [];
    for (const buffer of buffers) for (let y = 0; y < buffer.lines.length; y++) {
      const line = buffer.lines.get(y); lines.push(line); line._extendedAttrs = {};
      for (let x = 0; x < cols; x++) { line._data[x * 3 + 1] = 0; line._data[x * 3 + 2] = 0; }
    }
    const extended = state.attrs.map(value => {
      const result = runtime.input._curAttrData.extended.clone(); result.ext = value[2]; result.urlId = 0; return result;
    });
    for (const [start, length, id] of decodeRuns(state.runs, lines.length * cols, state.attrs.length)) {
      const value = state.attrs[id - 1];
      for (let at = start, end = start + length; at < end; at++) {
        const line = lines[Math.floor(at / cols)], x = at % cols;
        line._data[x * 3 + 1] = value[0]; line._data[x * 3 + 2] = value[1];
        if (value[1] & HAS_EXTENDED) line._extendedAttrs[x] = extended[id - 1];
      }
    }
  }
  function applyPalette(theme, changes) {
    theme.modifyColors(colors => {
      for (const key of ['foreground', 'background', 'cursor']) colors[key] = theme._restoreColors[key];
      colors.ansi = theme._restoreColors.ansi.slice();
      for (const [slot, rgba] of changes) {
        const color = { rgba, css: '#' + rgba.toString(16).padStart(8, '0') };
        if (typeof slot === 'number') colors.ansi[slot] = color;
        else colors[slot] = color;
      }
      colors.contrastCache.clear(); colors.halfContrastCache.clear();
    });
  }
  function replay(term, data) {
    return new Promise((resolve, reject) => {
      const timer = global.setTimeout(() => reject(Object.assign(new Error('The terminal display restore timed out. Keep input blocked and retry the saved handoff.'), { code: 'terminal_snapshot_busy' })), LIMITS.restoreMs);
      try { term.write(data, () => { global.clearTimeout(timer); resolve(); }); }
      catch (error) { global.clearTimeout(timer); reject(error); }
    });
  }
  // The pinned browser terminal keeps a separate absolute scrollbar position.
  // Public scrollToLine computes a relative delta from buffer.ydisp, which is
  // stale with respect to that scrollbar immediately after restore/row resize.
  function restoreViewport(term,row) {
    const buffer=term?.buffer?.active;
    int(row,0,buffer?.baseY,'viewport restore row');
    const viewport=term._core?._viewport;
    if(!viewport){requireValue(!term.element,'browser viewport');term.scrollToLine(row);return;}
    requireValue(typeof viewport._sync==='function'&&typeof viewport.scrollToLine==='function'&&typeof viewport.queueSync==='function'&&typeof term._core._bufferService?.scrollLines==='function','pinned viewport API');
    // The resize may also have moved ydisp while the scrollbar stayed put.
    term._core._bufferService.scrollLines(row-buffer.viewportY,false);
    viewport._sync(row); // Update dimensions before applying the absolute row.
    viewport.scrollToLine(row,true); // Immediate: never animate an ownership restore.
    viewport.queueSync(row); // A queued resize refresh must use the same anchor.
  }

  async function restore(term, snapshot) {
    const clean = validate(snapshot), state = clean.emulator;
    let r = inspect(term); drained(r);
    if (restores.has(term)) fail('busy', 'This terminal display is already being restored.');
    restores.add(term);
    const disableStdin = term.options.disableStdin;
    let restored = false;
    try {
      term.options.disableStdin = true;
      term.reset();
      r = inspect(term);
      r.parser.reset(); r.input._utf8Decoder.clear(); r.input._stringDecoder.clear();
      term.options.scrollback = state.scrollback;
      term.options.convertEol = false;
      term.resize(clean.cols, clean.rows);
      await replay(term, clean.data);
      r = inspect(term); drained(r);
      requireValue((r.buffers.active === r.buffers.alt ? 'alternate' : 'normal') === state.active &&
        r.buffers.normal.lines.length === state.normal.length &&
        r.buffers.alt.lines.length === state.alternate.length, 'restored buffer extent');
      const hash = readBufferData(r, clean.cols, false).hash;
      requireValue(hash[0] === state.contentHash[0] && hash[1] === state.contentHash[1], 'restored screen content');
      applyStyles(r, state, clean.cols);
      applyBuffer(r.buffers.normal, state.normal); applyBuffer(r.buffers.alt, state.alternate);
      applyAttr(r.input._curAttrData, state.attr); applyAttr(r.input._eraseAttrDataInternal, state.eraseAttr);
      r.chars._charsets = state.charset.slots.map(value => value === null ? undefined : value);
      r.chars.glevel = state.charset.level; r.chars.charset = state.charset.current === null ? undefined : state.charset.current;
      r.service.modes.insertMode = state.modes.insertMode;
      for (const key of BOOL_MODES) r.service.decPrivateModes[key] = state.modes[key];
      r.service.decPrivateModes.cursorBlink = state.modes.cursorBlink === null ? undefined : state.modes.cursorBlink;
      r.service.decPrivateModes.cursorStyle = state.modes.cursorStyle === null ? undefined : state.modes.cursorStyle;
      r.service.isCursorHidden = state.modes.cursorHidden; r.service.isCursorInitialized = state.modes.cursorInitialized;
      r.mouse.activeProtocol = state.mouse.protocol; r.mouse.activeEncoding = state.mouse.encoding;
      r.input._windowTitle = state.titles.window; r.input._iconName = state.titles.icon;
      r.input._windowTitleStack = state.titles.windows.slice(); r.input._iconNameStack = state.titles.icons.slice();
      applyPalette(r.theme, state.palette);
      const parser = state.parser, params = r.params;
      params.params.fill(0); params.params.set(parser.params); params.length = parser.params.length;
      params._subParams.fill(0); params._subParams.set(parser.subParams); params._subParamsLength = parser.subParams.length;
      params._subParamsIdx.fill(0); params._subParamsIdx.set(parser.subIndices);
      params._rejectDigits = parser.rejectDigits; params._rejectSubDigits = parser.rejectSubDigits; params._digitIsSub = parser.digitIsSub;
      r.parser._collect = parser.collect | 0; r.parser.precedingJoinState = parser.preceding | 0;
      r.parser.currentState = parser.state;
      r.input._utf8Decoder.interim.set(parser.utf8); r.input._stringDecoder._interim = parser.utf16;
      term.options.convertEol = state.convertEol;
      restoreViewport(term,clean.viewportY);
      applySelection(term,state.selection);
      r.core._bufferService.isUserScrolling = state.userScrolling;
      term.refresh(0, clean.rows - 1);
      restored = true;
    } finally {
      // A partially restored destination remains blocked until a valid retry
      // and the caller's ownership acknowledgement. No replay-generated
      // terminal reply can reach the transport.
      term.options.disableStdin = restored ? disableStdin : true;
      restores.delete(term);
    }
  }

  global.NetworkToolkitTerminalState = Object.freeze({ capture, restore, validate, captureSelection, beforeResize, afterResize, restoreViewport, limits: LIMITS, version: VERSION });
})(typeof globalThis !== 'undefined' ? globalThis : window);
