'use strict';
// Syntax highlighting menu and Customize dialog (v1.21.6).
const fs = require('node:fs'), vm = require('node:vm'), assert = require('node:assert/strict'), { test } = require('node:test');
const { document: makeDocument } = require('./terminal-dom-fixture.cjs');
const Rules = require('../web/assets/terminal-syntax-rules.js');

function harness(initial = {}) {
  const doc = makeDocument('<div id="display"></div><div id="terminalSessionPanel" class="terminal-session-panel"><button data-terminal-extra="close-session"></button></div><div id="modalRoot"></div>');
  const calls = { saved: 0, themes: 0, highlight: [], downloads: [], toasts: [], toolsClosed: 0, sessionClosed: 0 };
  doc.querySelector('[data-terminal-extra="close-session"]').click = () => calls.sessionClosed++;
  const context = {
    document: doc, console, TextDecoder, TextEncoder, Uint8Array, Date, JSON, Math,
    state: { terminalHighlight: initial.highlight !== false, terminalSyntax: initial.syntax, terminalAppearance: {}, theme: 'dark' },
    saveState: () => calls.saved++, updateTerminalTheme: () => calls.themes++, setTimeout: fn => { fn(); return 1; }, clearTimeout() {},
    setTerminalHighlight(value) { context.state.terminalHighlight = value; calls.highlight.push(value); calls.saved++; },
    setTerminalToolsOpen: () => calls.toolsClosed++,
    updateTerminalHighlightStatus() {},
    openModal(title, body, footer) { doc.getElementById('modalRoot').innerHTML = '<h2>' + title + '</h2><div class="modal-body">' + body + '</div><div class="modal-foot">' + footer + '</div>'; },
    download: (data, name, type) => calls.downloads.push({ data, name, type }), toast: (message, kind) => calls.toasts.push([message, kind]),
    NetworkToolkitTerminalAppearance: { theme: () => ({ background: '#101010', foreground: '#eeeeee' }), syntaxColors: () => ({ red: '#ff6060', green: '#18ed93', yellow: '#dedc12', blue: '#1bbae9', magenta: '#ff7dff', cyan: '#8eddf4', underline: '#eeeeee' }) }
  };
  context.window = context; context.globalThis = context;
  vm.createContext(context);
  vm.runInContext(fs.readFileSync(require.resolve('../web/assets/terminal-syntax-rules.js'), 'utf8'), context);
  vm.runInContext(fs.readFileSync(require.resolve('../web/assets/terminal-syntax-ui.js'), 'utf8'), context);
  const ui = context.NetworkToolkitTerminalSyntaxUI;
  doc.getElementById('display').innerHTML = ui.controlHTML();
  const $ = id => doc.getElementById(id), editor = () => doc.querySelector('[data-syntax-editor]');
  const click = selector => doc.emit('click', doc.querySelector(selector));
  const type = (id, value) => { const el = $(id); el.value = value; editor().emit('input', el); };
  const change = (id, props) => { const el = $(id); Object.assign(el, props); editor().emit('change', el); };
  return { doc, context, ui, calls, $, editor, click, type, change, state: context.state };
}

test('the menu lists None, device highlighting, built-in and own rule sets and keeps None tied to the highlight switch', () => {
  const h = harness({ syntax: { active: 'custom-mine', custom: [{ id: 'custom-mine', name: 'Mine', rules: { red: 'x' } }] } });
  assert.deepEqual(h.$('terminalSyntaxSelect').querySelectorAll('option').map(o => o.getAttribute('value')), ['none', 'smart', 'standard', 'shell', 'network', 'perl', 'sql', 'custom-mine']);
  h.ui.sync(); assert.equal(h.$('terminalSyntaxSelect').value, 'custom-mine');
  h.ui.choose('none');
  assert.deepEqual(h.calls.highlight, [false]); assert.equal(h.state.terminalSyntax.active, 'custom-mine', 'None must not forget the chosen rule set');
  h.ui.choose('network');
  assert.deepEqual(h.calls.highlight, [false, true]); assert.equal(h.state.terminalSyntax.active, 'network'); assert.equal(h.calls.themes, 1, 'open terminals were not refreshed');
  h.ui.choose('does-not-exist'); assert.equal(h.state.terminalSyntax.active, 'network');
  h.ui.choose('smart'); assert.equal(h.state.terminalSyntax.active, 'smart');
});

test('Customize closes Display, shows built-ins read-only and previews colors live', () => {
  const h = harness({ syntax: { active: 'network' } });
  h.click('[data-terminal-syntax="customize"]');
  assert.equal(h.calls.toolsClosed, 1); assert.equal(h.calls.sessionClosed, 1, 'Display stayed open above the dialog');
  assert.equal(h.$('syntaxEditSet').value || h.$('syntaxEditSet').querySelector('option[selected]').getAttribute('value'), 'network');
  assert.equal(h.$('syntaxRule-red').hasAttribute('readonly'), true); assert.equal(h.$('syntaxEditRegex').disabled || h.$('syntaxEditRegex').hasAttribute('disabled'), true);
  assert.match(h.editor().textContent, /Built-in rule sets can't be changed/);
  assert.match(h.$('syntaxPreview').innerHTML, /color:#ff6060/); assert.match(h.$('syntaxPreview').innerHTML, /text-decoration:underline/);
  h.type('syntaxSample', 'GigabitEthernet1/0/2 is down');
  assert.match(h.$('syntaxPreview').innerHTML, /<span style="color:#dedc12;">GigabitEthernet1\/0\/2<\/span>/);
  assert.match(h.$('syntaxPreview').innerHTML, /<span style="color:#ff6060;">down<\/span>/);
});

test('duplicating, editing, using and deleting an own rule set saves as you go', () => {
  const h = harness({ syntax: { active: 'smart' } });
  h.ui.open(); h.click('[data-syntax-act="duplicate"]');
  const id = h.state.terminalSyntax.custom[0].id;
  assert.match(id, /^custom-standard-keywords/); assert.equal(h.$('syntaxRule-red').hasAttribute('readonly'), false);
  h.type('syntaxEditName', 'Lab devices'); h.type('syntaxRule-red', '(link down)'); h.type('syntaxRule-green', '');
  let saved = h.state.terminalSyntax.custom[0];
  assert.equal(saved.name, 'Lab devices'); assert.equal(saved.rules.red, '(link down)'); assert.equal(saved.rules.green, '');
  h.type('syntaxRule-yellow', '(a+)+$');
  assert.match(h.$('syntaxError-yellow').textContent, /too slow/);
  h.change('syntaxEditRegex', { checked: false });
  assert.equal(h.state.terminalSyntax.custom[0].regex, false);
  h.click('[data-syntax-act="use"]');
  assert.equal(h.state.terminalSyntax.active, id); assert.match(h.calls.toasts.at(-1)[0], /Using Lab devices/);
  h.click('[data-syntax-act="delete"]'); assert.equal(h.state.terminalSyntax.custom.length, 1, 'Delete must ask for confirmation first');
  assert.equal(h.doc.querySelector('[data-syntax-act="delete"]').textContent, 'Confirm delete');
  h.click('[data-syntax-act="delete"]');
  assert.equal(h.state.terminalSyntax.custom.length, 0); assert.equal(h.state.terminalSyntax.active, 'smart', 'deleting the active set returns to device highlighting');
});

test('importing .msyn files names sets after the file and export writes Windows-1252 bytes', async () => {
  const h = harness();
  h.ui.open();
  const bytes = Rules.encode('[CustomSyntax]\r\nName=Custom: OK/warning/error keywords\r\nUseRegex=1\r\nRed=(¨!+[^¨]*)\r\nCaseSensitive=0\r\n');
  const file = (name, data, size = data.length) => ({ name, size, arrayBuffer: async () => data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength) });
  h.change('syntaxImportFile', { files: [file('Network keywords.msyn', bytes), file('Shell_keywords.msyn', bytes), file('broken.msyn', Rules.encode('hello')), file('huge.msyn', bytes, 300 * 1024)] });
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(Array.from(h.state.terminalSyntax.custom, set => set.name), ['Network keywords', 'Shell keywords']);
  assert.equal(h.calls.toasts.filter(([, kind]) => kind === 'error').length, 2);
  assert.match(h.calls.toasts.at(-1)[0], /Imported 2 rule sets/);
  h.click('[data-syntax-act="export"]');
  const out = h.calls.downloads.at(-1);
  assert.equal(out.name, 'Shell keywords.msyn'); assert.equal(out.data.includes(0xa8), true, '¨ must be saved as the Windows-1252 byte');
  assert.match(Rules.decode(out.data), /^\[CustomSyntax\]\r\nName=Shell keywords\r\nUseRegex=1\r\n/);
});
