const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const context = vm.createContext({ window: {}, TextEncoder });
vm.runInContext(fs.readFileSync(path.join(__dirname, '../web/assets/terminal-macro-model.js'), 'utf8'), context);
const model = context.window.NetworkToolkitMacroModel;
const plain = value => JSON.parse(JSON.stringify(value));
const definition = () => ({ id: 'macro-1', name: 'Check interfaces', steps: [{ id: 'first', kind: 'command', text: 'show interfaces' }] });

test('macro defaults preserve explicit false and zero; normalization keeps stable IDs', () => {
  const first = model.normalize(definition());
  assert.equal(first.waitForPrompt, true);
  assert.equal(first.timeoutMs, 30000);
  assert.equal(first.pacingMs, 100);
  assert.equal(first.prompt, model.DEFAULT_PROMPT);
  assert.equal(first.steps[0].id, 'first');
  assert.deepEqual(plain(model.normalize(first)), plain(first));
  const custom = model.normalize({ ...definition(), waitForPrompt: false, pacingMs: 0, parameters: [{ name: 'value', required: false }] });
  assert.equal(custom.waitForPrompt, false);
  assert.equal(custom.pacingMs, 0);
  assert.equal(custom.parameters[0].required, false);
  assert.equal(model.normalize({}).id, '');
});

test('validation canonicalizes metadata while preserving literal terminal text', () => {
  const source = {
    ...definition(), name: ' Inspect ', folder: ' Network ', shortcut: 'shift+control+KeyM', tags: [' network ', 'network', '', 'check'],
    parameters: [{ name: 'password', label: ' Password ', secret: true, defaultValue: 'never-store-this' }],
    steps: [{ id: 'existing-step', kind: 'command', text: 'echo <b>{{host}}</b> & {{password}}' }, { kind: 'pause', text: 'Review <details> before continuing' }],
    runtimeValues: { password: 'never-store-this' }, script: 'local-program'
  };
  const clean = model.validate(source);
  assert.equal(clean.name, 'Inspect');
  assert.equal(clean.folder, 'Network');
  assert.equal(clean.shortcut, 'Ctrl+Shift+M');
  assert.deepEqual(plain(clean.tags), ['network', 'check']);
  assert.equal(clean.parameters[0].defaultValue, '');
  assert.equal(source.parameters[0].defaultValue, 'never-store-this');
  assert.equal(clean.steps[0].text, source.steps[0].text);
  assert.equal(clean.steps[1].text, source.steps[1].text);
  assert.equal(clean.steps[0].id, 'existing-step');
  assert.equal(clean.steps[1].id, 'step-2');
  assert.equal('runtimeValues' in clean, false);
  assert.equal('script' in clean, false);
  assert.equal(JSON.stringify(clean).includes('never-store-this'), false);
  clean.steps[0].text = 'changed';
  clean.tags[0] = 'changed';
  assert.notEqual(source.steps[0].text, 'changed');
  assert.notEqual(source.tags[0], 'changed');
});

test('invalid macro structure, actions, patterns and unknown parameters are rejected', () => {
  const edits = [
    value => { value.id = ''; },
    value => { value.name = ''; },
    value => { value.name = 'name\x1b'; },
    value => { value.steps[0].kind = 'script'; },
    value => { value.steps[0] = { kind: 'key', key: 'RunProgram' }; },
    value => { value.steps[0].text = 'show {{interface}}'; },
    value => { value.steps[0].text = 'show {{bad-name}}'; },
    value => { value.parameters = [{ name: 'HOST' }]; },
    value => { value.parameters = [{ name: 'value' }, { name: 'value' }]; },
    value => { value.parameters = [{ name: 'value', defaultValue: 'a\nb' }]; },
    value => { value.steps.push({ ...value.steps[0] }); },
    value => { value.steps[0].disabled = true; },
    value => { value.steps[0].text = 'one\ntwo'; },
    value => { value.steps[0].text = 'one\x00two'; },
    value => { value.prompt = '('; },
    value => { value.steps[0] = { kind: 'wait', text: '(?=ready)' }; },
    value => { value.steps[0] = { kind: 'wait', text: '[' }; },
    value => { value.timeoutMs = -1; },
    value => { value.timeoutMs = NaN; },
    value => { value.steps[0].timeoutMs = 99; },
    value => { value.steps[0] = { kind: 'delay', delayMs: model.MAX_TIMEOUT_MS + 1 }; },
    value => { value.shortcut = 'm'; },
    value => { value.steps = 'command'; },
    value => { value.timeoutMs = 1.5; },
    value => { value.waitForPrompt = 'false'; },
    value => { value.parameters = [{ name: 3 }]; },
    value => { value.steps[0] = null; }
  ];
  for (const edit of edits) {
    const value = definition();
    edit(value);
    assert.throws(() => model.validate(value), { name: 'Error' }, JSON.stringify(value));
  }
});

test('all supported actions and RE2 flags validate with canonical fields', () => {
  const value = definition();
  value.parameters = [{ name: 'interface' }];
  value.steps = [
    { kind: 'command', text: 'show interface {{interface}}' },
    { kind: 'text', text: 'one\ntwo' },
    { kind: 'key', key: 'Ctrl+C', text: 'discard', delayMs: 1 },
    { kind: 'wait', text: '(?im){{host}}.*ready$', timeoutMs: 100 },
    { kind: 'delay', delayMs: model.MAX_TIMEOUT_MS },
    { kind: 'pause', text: 'Review {{session}}' }
  ];
  const clean = model.validate(value);
  assert.equal(clean.steps[2].text, '');
  assert.equal(clean.steps[2].delayMs, 0);
  assert.equal(clean.steps[2].timeoutMs, 0);
  assert.doesNotThrow(() => model.validate({ ...definition(), prompt: '(?P<prompt>router)[#>]$' }));
  assert.doesNotThrow(() => model.validate({ ...definition(), prompt: '\\Qliteral(\\E[#>]$' }));
});

test('64 KiB text and 1024-step boundaries are enforced without a low count limit', () => {
  const value = definition();
  value.steps[0].text = 'a'.repeat(model.MAX_STEP_TEXT_BYTES);
  assert.doesNotThrow(() => model.validate(value));
  value.steps[0].text = 'é'.repeat(model.MAX_STEP_TEXT_BYTES / 2 + 1);
  assert.throws(() => model.validate(value), /64 KiB/);
  value.steps = Array.from({ length: model.MAX_STEPS }, () => ({ kind: 'command', text: 'show clock' }));
  assert.doesNotThrow(() => model.validate(value));
  value.steps.push({ kind: 'command', text: 'show clock' });
  assert.throws(() => model.validate(value), /1024/);
  value.steps = Array.from({ length: 65 }, () => ({ kind: 'text', text: 'a'.repeat(model.MAX_STEP_TEXT_BYTES) }));
  assert.throws(() => model.validate(value), /4 MiB/);
});

test('parameter expansion is literal, single pass and never looks up inherited properties', () => {
  const text = 'show {{interface}} on {{Host}}: {{interface}}';
  const values = { interface: '<tag>$HOME;$(touch file) {{username}}', host: 'router-1' };
  assert.equal(model.expand(text, values), 'show <tag>$HOME;$(touch file) {{username}} on router-1: <tag>$HOME;$(touch file) {{username}}');
  assert.deepEqual(plain(model.detectParams(text)), ['interface', 'Host']);
  for (const [text, value] of [
    ['{{missing}}', ''], ['{{value', ''], ['{{bad-name}}', ''], ['{{value}}', 'one\ntwo'],
    ['{{value}}', 'one\rtwo'], ['{{value}}', 'one\x00two'], ['{{value}}', 'a'.repeat(model.MAX_STEP_TEXT_BYTES + 1)]
  ]) assert.throws(() => model.expand(text, { value }));
  assert.throws(() => model.expand('{{constructor}}', {}), /Enter a value/);
  assert.throws(() => model.expand('{{value}}', Object.create({ value: 'inherited' })), /Enter a value/);
  assert.equal(model.expand('{{constructor}}', { constructor: '<literal>' }), '<literal>');
});

test('command splitting preserves text; supported remote keys map to input bytes', () => {
  const steps = model.fromCommands(' show version \r\n\r\necho <details>&value\rshow clock\n');
  assert.deepEqual(plain(steps.map(step => ({ id: step.id, kind: step.kind, text: step.text }))), [
    { id: 'step-1', kind: 'command', text: ' show version ' },
    { id: 'step-2', kind: 'command', text: 'echo <details>&value' },
    { id: 'step-3', kind: 'command', text: 'show clock' }
  ]);
  for (const [name, value] of Object.entries({ Enter: '\r', Tab: '\t', Escape: '\x1b', 'Ctrl+C': '\x03', 'Ctrl+Z': '\x1a', 'Ctrl+D': '\x04', Up: '\x1b[A', Down: '\x1b[B', Left: '\x1b[D', Right: '\x1b[C', Backspace: '\x7f' })) assert.equal(model.keyBytes(name), value);
  assert.equal(model.keyBytes('unknown'), undefined);
});
