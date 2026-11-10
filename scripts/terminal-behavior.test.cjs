'use strict';
// Terminal features and Advanced settings (v1.21.8).
const fs = require('node:fs'), vm = require('node:vm'), path = require('node:path'), assert = require('node:assert/strict'), { test } = require('node:test');
const { document: makeDocument, Element } = require('./terminal-dom-fixture.cjs');
global.self = global; const { Terminal } = require('../web/assets/xterm.js');
const source = fs.readFileSync(path.join(__dirname, '../web/assets/terminal-behavior.js'), 'utf8');
const html = fs.readFileSync(path.join(__dirname, '../web/index.html'), 'utf8');

function harness(saved) {
  const doc = makeDocument('<div id="panel"></div><p class="terminal-transcript-note">Output remains in this app session. Transcript export includes the retained screen and up to 10,000 scrollback lines.</p>');
  const calls = { saved: 0, input: [] };
  const c = { document: doc, Date, setTimeout: (fn) => { c.pending = fn; return 1; }, clearTimeout() {}, state: { terminalBehavior: saved }, terminalSessions: new Map(),
    saveState: () => calls.saved++, enqueueTerminalInput: (entry, data, binary, origin) => calls.input.push({ data, binary, origin }) };
  c.window = c; vm.createContext(c); vm.runInContext(source, c);
  const api = c.NetworkToolkitTerminalBehavior; api.mount(doc.getElementById('panel'));
  const $ = id => doc.getElementById(id);
  const change = (id, props) => { const field = $(id); Object.assign(field, props); doc.emit('change', field); };
  return { c, api, doc, calls, $, change, state: c.state };
}
const plain = value => JSON.parse(JSON.stringify(value));

test('defaults keep today\'s terminal and invalid saved values fall back', () => {
  const h = harness(undefined);
  assert.deepEqual(plain(h.api.preferences()), { scrollback: 10000, scrollbar: true, bell: 'none', bracketedPaste: true, wordSeparators: ' ()[]{}\',"`', lfImpliesCr: false, crImpliesLf: false, reconnectKeys: true, closeOnOtherKey: false, answerback: '' });
  assert.deepEqual(plain(h.api.options()), { scrollback: 10000, convertEol: false, ignoreBracketedPasteMode: false, wordSeparator: ' ()[]{}\',"`' });
  assert.equal(new Terminal({}).options.wordSeparator, h.api.wordSeparators, 'the default must match xterm\'s own word separators');
  const bad = h.api.normalize({ scrollback: 100000, scrollbar: 'false', bell: 'loud', bracketedPaste: 0, wordSeparators: 'ab1 ,\u0007', lfImpliesCr: 'true', answerback: 'id\r\né-ok', extra: 'secret-canary' });
  assert.deepEqual(plain(bad), { scrollback: 10000, scrollbar: true, bell: 'none', bracketedPaste: true, wordSeparators: ' ,', lfImpliesCr: false, crImpliesLf: false, reconnectKeys: true, closeOnOtherKey: false, answerback: 'id-ok' });
  assert.equal(h.api.normalize({ answerback: 'x'.repeat(100) }).answerback.length, 64);
});

test('implicit LF in every CR changes only what is shown, including CR LF split across chunks', async () => {
  const h = harness({ crImpliesLf: true }), entry = {};
  const show = bytes => Buffer.from(h.api.displayBytes(entry, Buffer.from(bytes, 'latin1'))).toString('latin1');
  assert.equal(show('a\rb\r\nc'), 'a\r\nb\r\nc');
  assert.equal(show('end\r'), 'end\r\n'); assert.equal(show('\nnext'), 'next', 'the LF that completes a split CR LF is not doubled');
  assert.equal(show('\nkept'), '\nkept', 'only the LF right after a completed CR is dropped');
  const original = Buffer.from('a\rb');
  assert.equal(harness({}).api.displayBytes({}, original), original, 'off: the same bytes are written');
  const term = new Terminal({ cols: 20, rows: 4, allowProposedApi: true });
  await new Promise(resolve => term.write(h.api.displayBytes({}, Buffer.from('one\rtwo')), resolve));
  assert.deepEqual([0, 1].map(row => term.buffer.active.getLine(row).translateToString(true)), ['one', 'two']); term.dispose();
});

test('answerback replies to ^E only when set, as raw input, at most twice a second', () => {
  const none = harness({});
  assert.equal(none.api.observeOutput({}, Buffer.from('\x05')), false); assert.equal(none.calls.input.length, 0);
  const h = harness({ answerback: 'lab-term' }), entry = {};
  assert.equal(h.api.observeOutput(entry, Buffer.from('ready')), false);
  assert.equal(h.api.observeOutput(entry, Buffer.from('x\x05y')), true);
  assert.equal(h.api.observeOutput(entry, Buffer.from('\x05')), false, 'repeated ENQ within 500 ms is ignored');
  assert.deepEqual(plain(h.calls.input), [{ data: 'lab-term', binary: true, origin: 'answerback' }]);
  entry._ntAnswerbackAt -= 600; assert.equal(h.api.observeOutput(entry, Buffer.from('\x05')), true);
});

test('the bell flashes the terminal or does nothing, and xterm bells reach it', async () => {
  const off = harness({}), element = new Element('div');
  assert.equal(off.api.ring({ element }), false); assert.equal(element.classList.contains('terminal-bell-flash'), false);
  const h = harness({ bell: 'visual' }), term = new Terminal({ cols: 20, rows: 4, allowProposedApi: true }), entry = { xterm: term, element, disposables: [] };
  h.api.attach(entry);
  await new Promise(resolve => term.write('\x07', resolve));
  assert.equal(element.classList.contains('terminal-bell-flash'), true); h.c.pending(); assert.equal(element.classList.contains('terminal-bell-flash'), false);
  assert.equal(h.api.ring(entry), false, 'bells within 250 ms are merged');
  assert.equal(entry.disposables.length, 1); term.dispose();
});

test('settings save, apply to open terminals and restore; invalid scrollback snaps back', () => {
  const h = harness({}), term = new Terminal({ cols: 20, rows: 4, allowProposedApi: true, ...harness({}).api.options() }), element = new Element('div');
  const entry = { xterm: term, element, disposables: [] }; h.c.terminalSessions.set('one', entry); h.api.attach(entry);
  h.change('terminalScrollback', { value: '2500' });
  assert.equal(h.state.terminalBehavior.scrollback, 2500); assert.equal(term.options.scrollback, 2500); assert.equal(h.calls.saved, 1);
  assert.match(h.doc.querySelector('.terminal-transcript-note').textContent, /up to 2,500 scrollback lines/);
  h.change('terminalScrollback', { value: '99' }); assert.equal(h.$('terminalScrollback').value, '2500'); assert.equal(h.calls.saved, 1);
  h.change('terminalScrollbar', { checked: false }); assert.equal(element.classList.contains('terminal-hide-scrollbar'), true);
  h.change('terminalBracketedPaste', { checked: false }); assert.equal(term.options.ignoreBracketedPasteMode, true);
  h.change('terminalLfImpliesCr', { checked: true }); assert.equal(term.options.convertEol, true);
  h.change('terminalWordSeparators', { value: ' :' }); assert.equal(term.options.wordSeparator, ' :');
  h.change('terminalBell', { value: 'sound' }); h.change('terminalAnswerback', { value: 'lab-term' });
  assert.deepEqual(plain(h.state.terminalBehavior), { scrollback: 2500, scrollbar: false, bell: 'sound', bracketedPaste: false, wordSeparators: ' :', lfImpliesCr: true, crImpliesLf: false, reconnectKeys: true, closeOnOtherKey: false, answerback: 'lab-term' });
  const restored = harness(plain(h.state.terminalBehavior));
  assert.equal(restored.$('terminalScrollback').value, '2500'); assert.equal(restored.$('terminalScrollbar').checked, false); assert.equal(restored.$('terminalBell').value, 'sound'); assert.equal(restored.$('terminalAnswerback').value, 'lab-term');
  h.doc.emit('click', h.doc.querySelector('[data-terminal-behavior-reset]'));
  assert.equal(h.state.terminalBehavior.scrollback, 10000); assert.equal(element.classList.contains('terminal-hide-scrollbar'), false); assert.equal(term.options.convertEol, false); term.dispose();
});

test('the app creates terminals with these settings, restores them at startup and passes them to separate windows', () => {
  assert.match(html, /<script src="assets\/terminal-behavior\.js"><\/script>/);
  assert.match(html, /\.\.\.\(window\.NetworkToolkitTerminalBehavior\?\.options\(\)\|\|\{scrollback:10000,convertEol:false\}\)/);
  assert.match(html, /attachTerminalHighlighter\(entry\);window\.NetworkToolkitTerminalBehavior\?\.attach\(entry\);/);
  assert.match(html, /terminal\.write\(window\.NetworkToolkitTerminalBehavior\?\.displayBytes\(entry,bytes\)\|\|bytes,/);
  assert.match(html, /appendTerminalOutput\(entry,liveBytes,!!outputBatchEntries\);window\.NetworkToolkitTerminalBehavior\?\.observeOutput\(entry,liveBytes\);/);
  assert.match(html, /if\(s\.terminalBehavior&&typeof s\.terminalBehavior==='object'/);
  assert.match(html, /terminalBehavior:window\.NetworkToolkitTerminalBehavior\.preferences\(\)/);
  assert.match(html, /if\(value\.terminalBehavior!==undefined&&window\.NetworkToolkitTerminalBehavior\)/);
  for (const file of ['terminal_views.go', 'terminal_workspace_view.go']) assert.match(fs.readFileSync(path.join(__dirname, '../cmd/network-toolkit/' + file), 'utf8'), /"terminalBehavior"/, file);
  // The separate Terminal window no longer gets a copy when it is first used:
  // these settings are shared by every app of the data folder (shared_data.go).
  assert.match(fs.readFileSync(path.join(__dirname, '../cmd/network-toolkit/shared_data.go'), 'utf8'), /\{Name: "terminalBehavior", Kind: sharedMap, Live: true\}/);
  assert.doesNotMatch(fs.readFileSync(path.join(__dirname, '../cmd/network-toolkit/terminal_window.go'), 'utf8'), /"terminalBehavior"/, 'shared settings are not copied into the Terminal workspace');
  assert.match(fs.readFileSync(path.join(__dirname, '../cmd/network-toolkit/web_assets.go'), 'utf8'), /"\/assets\/terminal-behavior\.js"/);
  // state, saveState and terminalSessions are top-level bindings in index.html, not window properties.
  assert.doesNotMatch(source, /root\.(state|saveState|terminalSessions|enqueueTerminalInput)\b/);
  assert.match(html, /let state=|const state=|var state=/);
});

test('after a session ends, R reconnects and S saves it; other keys close the tab only when chosen', () => {
  const h = harness({}), calls = [];
  const entry = { info: { id: 'one', state: 'closed' } };
  Object.assign(h.c, { currentTerminal: () => entry, TERMINAL_ONLY: false, terminalReconnectAllowed: () => true, reconnectTerminal: e => calls.push(['reconnect', e.info.id]), removeTerminal: e => calls.push(['close', e.info.id]), NetworkToolkitSessionSidebar: { save: e => calls.push(['save', e.info.id]) } });
  h.state.activePage = 'terminal'; h.doc.getElementById('panel').insertAdjacentHTML('afterend', '<div id="modalRoot"></div><input id="field">');
  const key = (k, extra = {}) => { let prevented = false; const result = h.api.sessionKey({ key: k, target: h.doc, preventDefault() { prevented = true; }, stopImmediatePropagation() {}, ...extra }); return [result, prevented]; };
  assert.deepEqual(key('r'), ['reconnect', true]); assert.deepEqual(key('S'), ['save', true]);
  assert.deepEqual(key('x'), ['', false], 'other keys do nothing unless chosen');
  assert.deepEqual(key('r', { ctrlKey: true }), ['', false]); assert.deepEqual(key('r', { target: h.$('field') }), ['', false], 'typing in a field is never taken');
  h.state.activePage = 'editor'; assert.deepEqual(key('r'), ['', false], 'another page is never affected'); h.state.activePage = 'terminal';
  entry.info.state = 'connected'; assert.deepEqual(key('r'), ['', false], 'a connected session receives R as normal input'); entry.info.state = 'error';
  h.$('modalRoot').innerHTML = '<div></div>'; assert.deepEqual(key('r'), ['', false], 'not while a dialog is open'); h.$('modalRoot').innerHTML = '';
  h.change('terminalCloseOnOtherKey', { checked: true }); assert.deepEqual(key('q'), ['close', true]); assert.deepEqual(key('Shift'), ['', false], 'modifier keys alone do not close');
  assert.match(h.api.keysHint(), /R to reconnect or S to save this session; any other key closes the tab/);
  h.change('terminalReconnectKeys', { checked: false }); assert.deepEqual(key('r'), ['', false]); assert.equal(h.api.keysHint(), ''); assert.equal(h.$('terminalCloseOnOtherKey').disabled, true);
  assert.deepEqual(calls, [['reconnect', 'one'], ['save', 'one'], ['close', 'one']]);
  assert.match(html, /keysHint\?'<p class="terminal-disconnected-keys">'\+esc\(keysHint\)\+'<\/p>'/);
  assert.match(html, /const keysHint=entry\.reconnectBusy\|\|!terminalReconnectAllowed\(entry\)\?'':/);
});
