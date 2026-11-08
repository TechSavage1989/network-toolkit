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
    setTerminalToolsOpen: () => calls.toolsClosed++, picks: [],
    NetworkToolkitColorPicker: { open: options => context.picks.push(options) },
    updateTerminalHighlightStatus() {},
    openModal(title, body, footer) { doc.getElementById('modalRoot').innerHTML = '<h2>' + title + '</h2><div class="modal-body">' + body + '</div><div class="modal-foot">' + footer + '</div>'; },
    download: (data, name, type) => calls.downloads.push({ data, name, type }), toast: (message, kind) => calls.toasts.push([message, kind]),
    NetworkToolkitTerminalAppearance: { theme: () => ({ background: '#101010', foreground: '#eeeeee' }), syntaxColors: () => ({ red: '#ff6060', green: '#18ed93', yellow: '#dedc12', blue: '#1bbae9', magenta: '#ff7dff', cyan: '#8eddf4', underline: '#eeeeee' }),
      universalEditorHTML: () => '<section data-terminal-universal-editor><input type="checkbox" id="syntaxUniversalLinkUnderlines"></section>',
      highlightOptions: () => ({ theme: 'dark', colors: { prompt: '#ffaa00' }, underlines: true, linkUnderlines: true, prompt: initial.prompt || 'auto', rules: {} }), sync: () => calls.synced++ }
  };
  calls.synced = 0;
  context.window = context; context.globalThis = context;
  vm.createContext(context);
  for (const name of ['terminal-shell-syntax.js', 'terminal-syntax-rules.js', 'terminal-highlight.js']) vm.runInContext(fs.readFileSync(require.resolve('../web/assets/' + name), 'utf8'), context);
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
  h.ui.open('standard'); h.click('[data-syntax-act="duplicate"]');
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

test('each profile has its own colors that can be tried without saving and reset', () => {
  const h = harness({ syntax: { active: 'network', custom: [{ id: 'custom-lab', name: 'Lab', rules: { red: 'x' } }] } });
  assert.equal(h.ui.editColor('red'), true);
  const picker = h.context.picks.at(-1);
  assert.equal(picker.title, 'Choose the color for Down, disabled, denied and "no" commands in Network configuration (Cisco and others)', 'named by purpose (v1.21.15)'); assert.equal(picker.value, '#ff6060'); assert.equal(picker.defaultValue, '#ff6060');
  picker.onPreview('#112233');
  assert.equal(h.ui.colorsFor('network').red, '#112233', 'the color being tried is shown');
  assert.equal(h.state.terminalSyntax.colors?.network, undefined, 'a color being tried is never saved');
  picker.onCancel(); assert.equal(h.ui.colorsFor('network').red, undefined);
  h.ui.editColor('cyan'); h.context.picks.at(-1).onApply('#00AAFF');
  assert.equal(h.state.terminalSyntax.colors.network.cyan, '#00aaff'); assert(h.calls.saved > 0); assert(h.calls.themes > 0, 'open terminals repaint');
  h.ui.editColor('green', 'custom-lab'); h.context.picks.at(-1).onApply('#00ff00');
  h.ui.editColor('cyan'); h.context.picks.at(-1).onApply('#ffffff', { reset: true });
  assert.equal(h.state.terminalSyntax.colors.network, undefined, 'choosing the default removes the override');
  h.ui.editColor('blue'); h.context.picks.at(-1).onApply('#0000ff');
  assert.equal(h.ui.resetColors('network'), true); assert.equal(h.state.terminalSyntax.colors.network, undefined);
  assert.equal(h.state.terminalSyntax.colors['custom-lab'].green, '#00ff00', 'other profiles keep their colors');
  assert.equal(h.ui.editColor('nope'), false); assert.equal(h.ui.editColor('red', 'smart'), false, 'device highlighting has its own colors');
});

test('Universal has its own Customize page with rules, colors and a live preview (v1.21.12)', () => {
  const h = harness({ syntax: { active: 'smart' }, prompt: 'always' });
  h.click('[data-terminal-syntax="customize"]');
  const selected = () => h.$('syntaxEditSet').value || h.$('syntaxEditSet').querySelector('option[selected]').getAttribute('value');
  assert.equal(selected(), 'smart'); assert(h.doc.querySelector('[data-terminal-universal-editor]'), 'the Universal page holds the rules and colors');
  assert(h.calls.synced > 0, 'its controls are synced from saved settings');
  assert.equal(h.doc.querySelector('[data-syntax-act="duplicate"]'), null); assert.equal(h.doc.querySelector('[data-syntax-act="export"]'), null); assert(h.doc.querySelector('[data-syntax-act="import"]'));
  assert.equal(h.doc.querySelector('[data-syntax-act="use"]').textContent, 'Use Universal');
  assert.match(h.$('syntaxPreview').innerHTML, /data-universal-kind="prompt-name" style="color:#ffaa00;[^"]*">biqu@CR10SPro</);
  assert.match(h.$('syntaxPreview').innerHTML, /data-universal-kind="url"[^>]*>\\\\fileserver\\backups\\core-1\.cfg</);
  h.type('syntaxSample', 'switch#show run'); assert.match(h.$('syntaxPreview').innerHTML, /data-universal-kind="prompt-name"[^>]*>switch</);
  h.change('syntaxEditSet', { value: 'network' }); assert(h.$('syntaxRule-red')); assert.equal(h.doc.querySelector('[data-syntax-act="use"]').textContent, 'Use this rule set');
  h.change('syntaxEditSet', { value: 'smart' }); assert(h.doc.querySelector('[data-terminal-universal-editor]'));
  assert.equal(h.$('syntaxSample').textContent, 'switch#show run', 'your sample text is kept while you switch');
  h.click('[data-syntax-act="use"]'); assert.equal(h.state.terminalSyntax.active, 'smart'); assert.match(h.calls.toasts.at(-1)[0], /Using Universal/);
  const other = harness({ syntax: { active: 'network' } });
  other.doc.getElementById('display').insertAdjacentHTML('beforeend', '<button data-terminal-syntax="customize" data-terminal-syntax-set="smart">Rules and colors…</button>');
  other.click('[data-terminal-syntax-set="smart"]'); assert(other.doc.querySelector('[data-terminal-universal-editor]'), 'Rules and colors… opens Universal while a rule set is active');
  const plain = harness({ syntax: { active: 'network' } }); plain.ui.open(); assert.equal(plain.doc.querySelector('[data-terminal-universal-editor]'), null, 'Customize still opens the active rule set');
});

test('rule set pages use cards: switch each color, change its color, keep editing your own patterns (v1.21.14)', () => {
  const h = harness({ syntax: { active: 'standard' } });
  h.ui.open('standard');
  const cards = h.doc.querySelectorAll('[data-syntax-slot]');
  assert.deepEqual(cards.map(card => card.dataset.syntaxSlot), ['red', 'green', 'yellow', 'magenta', 'cyan', 'underline'], 'built-in sets show only the colors they use');
  assert.equal(h.$('syntaxSlotOn-red').hasAttribute('checked'), true); assert.match(h.doc.querySelector('[data-syntax-swatch="red"]').textContent, /#ff6060/);
  assert(h.$('syntaxRule-red').closest('details'), 'built-in patterns are folded away'); assert.equal(h.$('syntaxRule-red').hasAttribute('readonly'), true);
  assert.match(h.$('syntaxPreview').innerHTML, /color:#ff6060/);
  h.change('syntaxSlotOn-red', { checked: false });
  assert.deepEqual(Array.from(h.state.terminalSyntax.off.standard), ['red'], 'built-in colors can be switched off'); assert(h.calls.saved > 0);
  assert.doesNotMatch(h.$('syntaxPreview').innerHTML, /color:#ff6060/, 'the preview leaves a switched-off color out');
  assert.equal(h.$('syntaxSlotOn-red').hasAttribute('checked'), false, 'the card shows it is off');
  const picks = h.context.picks.length;
  h.editor().emit('click', h.$('syntaxRule-red')); assert.equal(h.context.picks.length, picks, 'clicking a pattern does not open the color picker');
  h.editor().emit('click', h.doc.querySelector('[data-syntax-swatch="green"]')); assert.equal(h.context.picks.length, picks + 1); assert.equal(h.context.picks.at(-1).title, 'Choose the color for Success in Standard keywords (OK/warning/error/…)');
  h.context.picks.at(-1).onApply('#00ff88'); assert.equal(h.state.terminalSyntax.colors.standard.green, '#00ff88');
  h.click('[data-syntax-act="duplicate"]');
  const copy = h.state.terminalSyntax.custom[0].id;
  assert.deepEqual(Array.from(h.state.terminalSyntax.off[copy]), ['red'], 'the copy keeps the switches'); assert.equal(h.state.terminalSyntax.colors[copy].green, '#00ff88', 'and the colors');
  assert.equal(h.$('syntaxRule-red').closest('details'), null, 'your own patterns are shown for editing'); assert.equal(h.$('syntaxRule-red').hasAttribute('readonly'), false);
  h.type('syntaxRule-blue', 'uplink'); assert.equal(h.state.terminalSyntax.custom[0].rules.blue, 'uplink');
  h.click('[data-syntax-act="all-on"]'); assert.equal(h.state.terminalSyntax.off[copy], undefined);
  h.click('[data-syntax-act="reset-colors"]'); assert.equal(h.state.terminalSyntax.colors[copy], undefined);
  h.click('[data-syntax-act="delete"]'); h.click('[data-syntax-act="delete"]');
  assert.equal(h.state.terminalSyntax.custom.length, 0); assert.equal(h.state.terminalSyntax.off[copy], undefined);
  assert.deepEqual(Array.from(h.state.terminalSyntax.off.standard), ['red'], 'other sets keep their switches');
});

test('cards are named by purpose, own sets can rename them, and every rule set has the Device prompts card (v1.21.15)', () => {
  const h = harness({ syntax: { active: 'standard' } });
  h.context.NetworkToolkitTerminalAppearance.promptCardHTML = toggle => '<div data-terminal-prompt-card>' + toggle + '<label><strong>Device prompts</strong></label></div>';
  h.ui.open('standard');
  const red = h.doc.querySelector('[data-syntax-slot="red"]');
  assert.equal(red.querySelector('strong').textContent, 'Errors and failures', 'built-in cards use their purpose'); assert.match(red.textContent, /Red color in \.msyn files/);
  assert.equal(h.doc.querySelector('[data-syntax-slot="blue"]'), null, 'unused built-in colors are not listed');
  const prompt = h.$('syntaxSlotOn-prompt'); assert(prompt, 'the Device prompts card has its own switch'); assert.equal(prompt.hasAttribute('checked'), true);
  h.change('syntaxSlotOn-prompt', { checked: false }); assert.deepEqual(Array.from(h.state.terminalSyntax.off.standard), ['prompt']);
  h.change('syntaxSlotOn-prompt', { checked: true }); assert.equal(h.state.terminalSyntax.off.standard, undefined);
  h.click('[data-syntax-act="duplicate"]');
  const copy = h.state.terminalSyntax.custom[0];
  assert.equal(copy.labels.red, 'Errors and failures', 'a copy keeps the purposes'); assert.equal(copy.labels.blue, 'Other keywords');
  assert.equal(h.$('syntaxLabel-red').getAttribute('value'), 'Errors and failures'); assert(h.doc.querySelector('[data-syntax-slot="blue"]'), 'your own sets list all seven colors');
  h.type('syntaxLabel-red', 'Link failures'); assert.equal(h.state.terminalSyntax.custom[0].labels.red, 'Link failures', 'renamed as you type');
  h.type('syntaxSample', 'switch#show logging');
  assert.match(h.$('syntaxPreview').innerHTML, /<span style="color:#ffaa00;">switch#<\/span>/, 'the preview colors the device prompt');
});
