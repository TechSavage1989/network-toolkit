'use strict';
// Paste warnings like MobaXterm's (v1.21.9).
const assert = require('node:assert/strict'), { test } = require('node:test');
const W = require('../web/assets/terminal-paste-warnings.js');
const kinds = (text, enabled) => W.check(text, enabled).map(w => w.kind);

test('ordinary device and shell commands raise no warning', () => {
  for (const text of ['show running-config', 'show ip int brief | include up', 'ip route 0.0.0.0 0.0.0.0 192.0.2.1', 'source ~/.bashrc', 'tar czf backup.tgz -C /etc .',
    'unzip tools.zip -d /opt/tools', 'copy http://10.0.0.5/ios.bin flash:', 'copy http://tftp-server/ios.bin flash:', 'wget https://example.com/a.tgz | tar xz',
    'curl -o f https://example.com/a; grep x f | sh-check', 'Привет мир', 'description Uplink to core', 'echo hello > /tmp/out.txt', ''])
    assert.deepEqual(kinds(text), [], text);
});

test('download-and-run commands are recognized in shell and PowerShell forms', () => {
  for (const text of ['curl -fsSL https://get.example.com/install.sh | sudo bash', 'curl https://example.com | python3', 'wget -qO- https://example.com/i|sh -s -- --yes',
    'bash <(curl -s https://example.com/x)', 'sh -c "$(wget -qO- https://example.com/i)"', 'iex (New-Object Net.WebClient).DownloadString("https://example.com/a.ps1")', 'irm https://example.com/a.ps1 | iex'])
    assert.deepEqual(kinds(text), ['pipeToShell'], text);
});

test('shell startup file changes, system-folder extraction and links are recognized', () => {
  assert.deepEqual(kinds('echo "alias ll=ls" >> ~/.bashrc'), ['shellProfile']);
  assert.deepEqual(kinds('Add-Content $PROFILE "Set-Alias x y"'), ['shellProfile']);
  assert.deepEqual(kinds("sudo sed -i 's/a/b/' /etc/profile"), ['shellProfile']);
  assert.deepEqual(kinds('sudo tar xzf pkg.tgz -C /usr'), ['systemArchive']);
  assert.deepEqual(kinds('tar xf a.tar -C /'), ['systemArchive']);
  assert.deepEqual(kinds('Expand-Archive x.zip -DestinationPath C:\\Windows\\System32'), ['systemArchive']);
  assert.deepEqual(kinds('copy http://198.51.100.7/ios.bin flash:'), ['link']);
  assert.deepEqual(kinds('see https://bit.ly/abc and http://example.com/x'), ['link', 'link'], 'shortened and plain HTTP are separate warnings');
  for (const host of ['10.1.2.3', '172.16.0.1', '192.168.1.1', '127.0.0.1', '169.254.1.1', 'localhost', 'router', 'nas.local', 'sw1.home.arpa', '[fd00::1]', '[fe80::1]']) assert.equal(W.internalHost(host), true, host);
  for (const host of ['198.51.100.7', '172.32.0.1', 'example.com', '[2001:db8::1]']) assert.equal(W.internalHost(host), false, host);
});

test('look-alike, invisible and typographic characters are recognized with a readable sample', () => {
  const mixed = W.check('ping раypal.com');
  assert.equal(mixed[0].kind, 'homograph'); assert.match(mixed[0].message, /“раypal\.com” mixes Latin letters/);
  assert.match(W.check('ssh admin@192.0.2.1\u200b')[0].message, /U\+200B/);
  assert.match(W.check('echo \u202Egnp.exe')[0].message, /U\+202E/);
  assert.match(W.check('interface Gi1/0/1 – shutdown')[0].message, /Typographic quotes or dashes/);
  assert.match(W.check('snmp-server community “public” RO')[0].message, /Typographic/);
  const long = W.check('curl https://example.com/' + 'a'.repeat(200) + ' | sh')[0];
  assert.equal(Array.from(long.sample).length, W.limits.sample); assert.match(long.sample, /…$/);
});

test('turned-off kinds are skipped, each warning appears once and large pastes stay fast', () => {
  const text = 'curl https://example.com/i | sh\ncurl https://example.com/j | bash\nhttp://example.com/a';
  assert.deepEqual(kinds(text), ['pipeToShell', 'link']);
  assert.deepEqual(kinds(text, { pipeToShell: false }), ['link']);
  assert.deepEqual(kinds(text, { pipeToShell: false, link: false }), []);
  assert.deepEqual(W.ids, ['homograph', 'pipeToShell', 'shellProfile', 'systemArchive', 'link']);
  const started = Date.now();
  W.check('x'.repeat(1024 * 1024)); W.check(('curl a ' + 'b'.repeat(3000) + '\n').repeat(300)); W.check('(a'.repeat(100000)); W.check(('tar -C ' + '/x'.repeat(4000) + '\n').repeat(100));
  assert(Date.now() - started < 1500, 'checking must stay fast on 1 MiB pastes');
});
