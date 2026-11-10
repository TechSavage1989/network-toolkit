'use strict';
// Terminal links: a link only prefills the connection form (terminal-links.js).
const fs = require('node:fs'), vm = require('node:vm'), path = require('node:path'), assert = require('node:assert/strict'), { test } = require('node:test');
const { document: makeDocument } = require('./terminal-dom-fixture.cjs');
const source = fs.readFileSync(path.join(__dirname, '../web/assets/terminal-links.js'), 'utf8');
const html = fs.readFileSync(path.join(__dirname, '../web/index.html'), 'utf8');
const links = require('../web/assets/terminal-links.js');

const profiles = [
  { id: 'core', name: 'Core switch', type: 'ssh', host: 'Core.Example.', port: 2222, username: 'operator', credentialId: 'vault-reference' },
  { id: 'core-admin', name: 'Core admin', type: 'ssh', host: 'core.example', port: 2222, username: 'admin' },
  { id: 'lab', name: 'Lab Telnet', type: 'telnet', host: '192.0.2.20' },
  { id: 'v6', name: 'IPv6 router', type: 'ssh', host: '[2001:db8::1]', username: 'operator' },
  { id: 'console', name: 'Console', type: 'serial', serialPort: 'com4', baudRate: 115200 },
];

test('only app-produced link metadata is kept', () => {
  assert.deepEqual({ ...links.normalize({ scheme: 'ssh', passwordIgnored: true, password: 'secret', url: 'ssh://x' }) }, { scheme: 'ssh', passwordIgnored: true });
  assert.deepEqual({ ...links.normalize({ scheme: 'serial', passwordIgnored: 'yes' }) }, { scheme: 'serial', passwordIgnored: false });
  for (const bad of [null, undefined, 'ssh', {}, { scheme: 'http' }, { scheme: 'cmd' }, { scheme: 'powershell' }, { scheme: '__proto__' }, { scheme: 'toString' }]) assert.equal(links.normalize(bad), null);
});

test('a saved connection is offered only on an exact type, host, port and user match', () => {
  assert.equal(links.match(profiles, { type: 'ssh', host: 'core.example', port: 2222, username: 'operator' }).id, 'core');
  assert.equal(links.match(profiles, { type: 'ssh', host: 'core.example', port: 2222, username: 'admin' }).id, 'core-admin');
  assert.equal(links.match(profiles, { type: 'ssh', host: 'core.example', port: 22, username: 'operator' }), null, 'port differs');
  assert.equal(links.match(profiles, { type: 'ssh', host: 'core.example', port: 2222, username: 'Operator' }), null, 'user names are exact');
  assert.equal(links.match(profiles, { type: 'ssh', host: 'core.example', port: 2222, username: '' }), null, 'a link without a user does not pick one');
  assert.equal(links.match(profiles, { type: 'telnet', host: 'core.example', port: 2222 }), null, 'type differs');
  assert.equal(links.match(profiles, { type: 'telnet', host: '192.0.2.20', port: 23 }).id, 'lab', 'Telnet profiles default to port 23');
  assert.equal(links.match(profiles, { type: 'ssh', host: '2001:db8::1', port: 22, username: 'operator' }).id, 'v6', 'IPv6 brackets and the default SSH port');
  assert.equal(links.match(profiles, { type: 'serial', serialPort: 'COM4' }).id, 'console');
  assert.equal(links.match(profiles, { type: 'serial', serialPort: 'COM5' }), null);
  assert.equal(links.match([{ type: 'ssh', host: 'core.example', port: 2222, username: 'operator' }], { type: 'ssh', host: 'core.example', port: 2222, username: 'operator' }), null, 'rows without an id are ignored');
  assert.equal(links.match(null, { type: 'ssh', host: 'x' }), null);
});

test('the notice describes the request, the ignored password and the saved match', () => {
  const config = { type: 'ssh', host: 'core.example', port: 2222, username: 'operator' };
  const notice = links.noticeHTML({ scheme: 'ssh', passwordIgnored: true }, config, profiles[0], false);
  assert.match(notice, /Opened from a link\.<\/strong> SSH to operator@core\.example port 2222\. Check the details, then choose Connect\. Nothing connects until you do\./);
  assert.match(notice, /Passwords in links are ignored\./);
  assert.match(notice, /data-terminal-link-profile="core">Use saved connection Core switch<\/button>/);
  const using = links.noticeHTML({ scheme: 'ssh', passwordIgnored: false }, config, profiles[0], true);
  assert.doesNotMatch(using, /Passwords in links|<button/);
  assert.match(using, /Using saved connection <strong>Core switch<\/strong>/);
  const hostile = links.noticeHTML({ scheme: 'ssh' }, { type: 'ssh', host: '<img src=x onerror=alert(1)>', port: 22, username: '"><b>' }, { id: '"x', name: '<b>n</b>', type: 'ssh' }, false);
  assert.doesNotMatch(hostile, /<img|<b>|"x"/);
  assert.equal(links.describe({ type: 'telnet', host: '2001:db8::5', port: 23 }), 'Telnet to [2001:db8::5] port 23');
  assert.equal(links.describe({ type: 'serial', serialPort: 'COM3', baudRate: 115200, dataBits: 7, parity: 'even', stopBits: '2' }), 'Serial COM3 at 115200 baud, 7 data bits, even parity, 2 stop bits');
});

function harness(registration) {
  const doc = makeDocument('<div id="panel"><div data-terminal-settings-panel="behavior"><section id="terminalBehaviorSettings"></section></div></div><div id="modal"><form id="terminalConnectForm"><input id="con-host" value="core.example"><input id="con-username" value="operator"><input id="terminalPassword" value=""></form><button id="terminalConnectButton"></button></div>');
  for (const el of doc.querySelectorAll('input,button')) { el.getClientRects = () => [1]; el.value = el.getAttribute('value') || ''; }
  const calls = { api: [], chosen: [], toasts: [], focused: [] };
  doc.getElementById('terminalConnectButton').focus = () => calls.focused.push('connect');
  doc.getElementById('terminalPassword').focus = () => calls.focused.push('password');
  doc.getElementById('con-host').focus = () => calls.focused.push('host');
  const form = doc.getElementById('terminalConnectForm');
  form.prepend = node => { node.parentElement = form; form.children.unshift(node); };
  const c = { document: doc, state: { terminalProfiles: profiles }, terminalDraft: { type: 'ssh', host: 'core.example', port: 2222, username: 'operator', authMethod: 'password' }, setTimeout: fn => fn(), Promise,
    api: async (p, method = 'GET', body) => { calls.api.push({ p, method, body }); if (p === '/terminal-links') return registration; return { ok: true }; },
    toast: (message, kind) => calls.toasts.push({ message, kind }),
    chooseTerminalConnectionProfile: id => { calls.chosen.push(id); c.terminalDraft = { ...profiles.find(p => p.id === id) }; } };
  c.window = c; vm.createContext(c);
  vm.runInContext(source, c);
  return { c, doc, calls, api: c.NetworkToolkitTerminalLinks };
}

test('a link opens the form with the notice, focus on what is still needed, and no connection', () => {
  const h = harness(null);
  assert.equal(h.api.decorate({ scheme: 'ssh', passwordIgnored: true }, h.c.terminalDraft), true);
  const notice = h.doc.getElementById('terminalLinkNotice');
  assert.ok(notice && notice.parentElement === h.doc.getElementById('terminalConnectForm'));
  assert.match(notice.innerHTML, /Passwords in links are ignored\./);
  assert.match(notice.innerHTML, /Use saved connection Core switch/);
  assert.deepEqual(h.calls.focused, ['password'], 'a password connection without a saved credential waits for the password');
  assert.deepEqual(h.calls.api, [], 'opening the form makes no request');
  h.doc.emit('click', notice.querySelector('[data-terminal-link-profile]'));
  assert.deepEqual(h.calls.chosen, ['core']);
  assert.match(notice.innerHTML, /Using saved connection <strong>Core switch<\/strong>/);
  assert.match(notice.innerHTML, /Passwords in links are ignored\./);
  assert.equal(h.calls.focused.at(-1), 'connect', 'with the saved credential the form is ready for Connect');
  assert.deepEqual(h.calls.api, [], 'choosing the saved connection does not connect');
  assert.equal(h.api.decorate({ scheme: 'http' }, h.c.terminalDraft), false);
});

test('Terminal settings show the Default apps pointer only where it applies', async () => {
  let h = harness({ supported: true, registered: true, current: true });
  h.api.mount(h.doc.getElementById('panel'));
  await new Promise(resolve => setImmediate(resolve));
  const section = h.doc.getElementById('terminalLinkSettings');
  assert.equal(section.hidden, false);
  assert.match(section.innerHTML, /<h4>Links<\/h4>/);
  assert.match(section.innerHTML, /Make Network Toolkit the default for SSH, Telnet and serial links/);
  h.doc.emit('click', section.querySelector('[data-terminal-link-defaults]'));
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(h.calls.api.map(r => r.p + ' ' + r.method), ['/terminal-links GET', '/terminal-links/default-apps POST', '/terminal-links GET']);
  assert.match(h.calls.toasts[0].message, /Choose Network Toolkit Terminal/);

  h = harness({ supported: true, registered: true, current: false });
  h.api.mount(h.doc.getElementById('panel'));
  await new Promise(resolve => setImmediate(resolve));
  assert.match(h.doc.getElementById('terminalLinkSettings').innerHTML, /Links open the installed copy of Network Toolkit\./);

  h = harness({ supported: true, registered: false });
  h.api.mount(h.doc.getElementById('panel'));
  await new Promise(resolve => setImmediate(resolve));
  const portable = h.doc.getElementById('terminalLinkSettings');
  assert.match(portable.innerHTML, /The Windows setup adds this\./);
  assert.equal(portable.querySelector('[data-terminal-link-defaults]'), null);

  for (const value of [{ supported: false }, null, {}]) {
    h = harness(value);
    h.api.mount(h.doc.getElementById('panel'));
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(h.doc.getElementById('terminalLinkSettings').hidden, true);
  }
});

test('the app keeps link metadata with each request and opens link forms through the prefilled dialog', () => {
  assert.match(html, /<script src="assets\/terminal-links\.js"><\/script>/);
  assert.match(html, /link:window\.NetworkToolkitTerminalLinks\?\.normalize\(request\.link\)\|\|null/);
  assert.match(html, /openTerminalConnect\(request\.config,request\.profileId\|\|'',request\.link\?\{link:request\.link\}:\{\}\)/);
  assert.match(html, /if\(options\.link\)window\.NetworkToolkitTerminalLinks\?\.decorate\(options\.link,terminalDraft\);/);
  // The link path must never call the connect routine on its own.
  assert.doesNotMatch(source, /connectTerminal|\/terminals['"]/);
});
