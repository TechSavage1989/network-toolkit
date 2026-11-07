'use strict';
// Device prompt name color and URL / network-path underlines (v1.21.12).
// Synthetic text only; no connections or devices are used.
const fs = require('node:fs'), vm = require('node:vm'), path = require('node:path'), assert = require('node:assert/strict'), { test } = require('node:test');
global.self = global; const { Terminal } = require('../web/assets/xterm.js');
function load() {
  const timers = new Map(); let next = 0;
  const context = { setTimeout: fn => { timers.set(++next, fn); return next; }, clearTimeout: id => timers.delete(id), performance: { now: () => 0 } };
  vm.createContext(context);
  for (const name of ['terminal-shell-syntax.js', 'terminal-syntax-rules.js', 'terminal-highlight.js']) vm.runInContext(fs.readFileSync(path.join(__dirname, '../web/assets', name), 'utf8'), context);
  return { context, timers, H: context.NTTerminalHighlight, R: context.NTKTerminalSyntaxRules };
}
const { H, R } = load();
const spans = (line, prompt = 'auto', set = null) => Array.from(H.analyze(line, 'ssh', {}, set, { prompt }), hit => hit.kind + ':' + line.slice(hit.start, hit.end));

test('the device name inside prompts is found for Linux, network devices and vendors', () => {
  const names = {
    'biqu@CR10SPro:~$ ls -la': 'biqu@CR10SPro', 'root@web01:/var/log# tail syslog': 'root@web01', 'user@host ~ % ls': 'user@host',
    'switch#show run': 'switch', 'SW1(config-if)#no shut': 'SW1', 'admin@PA-VM> show system info': 'admin@PA-VM',
    'RP/0/RSP0/CPU0:ios#show ver': 'ios', '[admin@MikroTik] > /ip address print': 'admin@MikroTik', '<HUAWEI>display version': 'HUAWEI',
    '[~HUAWEI-GigabitEthernet0/0/1]display this': 'HUAWEI', '[Core-SW1]': 'Core-SW1', 'FGT60E (global) # get system status': 'FGT60E'
  };
  for (const [line, name] of Object.entries(names)) {
    assert(spans(line, 'always').includes('prompt-name:' + name), line + ' → ' + JSON.stringify(spans(line, 'always')));
    assert(spans(line, 'off').includes('prompt-plain:' + name), line);
    assert(!spans(line).some(item => item.startsWith('prompt-name') || item.startsWith('prompt-plain')), 'Automatic keeps the whole prompt: ' + line);
  }
  assert.deepEqual(spans('biqu@CR10SPro:~$ ls', 'always'), ['prompt-name:biqu@CR10SPro', 'prompt::~$'], 'the rest of the prompt keeps the Device prompt rule');
  assert.deepEqual(spans('biqu@CR10SPro:~$ ls', 'off'), ['prompt-plain:biqu@CR10SPro'], 'No color leaves the whole prompt uncolored');
  assert.deepEqual(spans('PS C:\\Users\\admin> dir', 'always'), ['prompt:PS C:\\Users\\admin>'], 'a drive prompt has no device name');
  for (const line of ['The switch is down.', 'Last login: Sat Nov  7 10:00:00 2026 from 192.0.2.60', '<html><head>', '<p>Hello'])
    assert(!spans(line, 'always').some(item => item.startsWith('prompt')), line);
});

test('the prompt name option also works with keyword rule sets', () => {
  const set = R.compile(R.find(R.preferences({}), 'standard'));
  assert.deepEqual(spans('biqu@CR10SPro:~$ cat error.log', 'auto', set), ['syn-red:error']);
  assert.deepEqual(spans('biqu@CR10SPro:~$ cat error.log', 'always', set), ['prompt-name:biqu@CR10SPro', 'syn-red:error']);
  assert.deepEqual(spans('switch#show interfaces', 'off', set), ['prompt-plain:switch']);
  const legend = H.legend('dark', { ruleset: set, ruleSet: R.find(R.preferences({}), 'standard'), syntaxColors: {}, prompt: 'always', colors: { prompt: '#123456' } });
  assert.deepEqual(JSON.parse(JSON.stringify(legend.at(-1))), { kind: 'prompt', label: 'Device prompt name', color: '#123456', borderStyle: 'none', borderWidth: 0 });
  assert(!H.legend('dark', { ruleset: set, ruleSet: R.find(R.preferences({}), 'standard'), syntaxColors: {}, prompt: 'auto' }).some(item => item.kind === 'prompt'));
});

test('URLs and network paths: share and transfer schemes, UNC shares, NFS and scp locations', () => {
  const urls = {
    'copy to \\\\fileserver\\backups\\core.cfg, done': '\\\\fileserver\\backups\\core.cfg', 'open \\\\192.0.2.5\\c$\\temp': '\\\\192.0.2.5\\c$\\temp',
    'smb://nas.example.com/share': 'smb://nas.example.com/share', 'nfs://192.0.2.4/export': 'nfs://192.0.2.4/export', 'tftp://192.0.2.9/startup.cfg': 'tftp://192.0.2.9/startup.cfg',
    'file:///C:/temp/x.log': 'file:///C:/temp/x.log', 'mount 192.0.2.10:/exports/home /mnt': '192.0.2.10:/exports/home', 'mount nas.local:/export /mnt': 'nas.local:/export',
    'scp file admin@backup.example.com:/srv/backups/': 'admin@backup.example.com:/srv/backups/', 'Status page: https://example.com/status.': 'https://example.com/status'
  };
  for (const [line, url] of Object.entries(urls)) assert(spans(line).includes('url:' + url), line + ' → ' + JSON.stringify(spans(line)));
  for (const line of ['see e.g.:/path and notes', 'grep file.txt:/x', 'time 10:00:/ ok', 'C:\\Users\\admin\\Documents', '\\\\.\\pipe\\docker_engine', '\\\\?\\C:\\temp', 'Ratio 3.2:/1'])
    assert(!spans(line).some(item => item.startsWith('url:')), line + ' → ' + JSON.stringify(spans(line)));
  assert.equal(H.legend('dark', {}).find(item => item.kind === 'url').label, 'URL / network path');
});

test('URLs and network paths have their own underline switch', () => {
  const pick = (options, kind) => H.legend('dark', options).find(item => item.kind === kind);
  assert.equal(pick({ underlines: false }, 'url').borderWidth, 1, 'link underlines stay on when other underlines are off');
  assert.equal(pick({ underlines: false }, 'address').borderWidth, 0);
  assert.equal(pick({ underlines: true, linkUnderlines: false }, 'url').borderWidth, 0, 'link underlines turn off on their own');
  assert.equal(pick({ underlines: true, linkUnderlines: false }, 'address').borderWidth, 2);
  assert(!H.legend('dark', { prompt: 'off' }).some(item => item.kind === 'prompt'), 'No color hides the prompt chip');
  assert(H.legend('dark', { prompt: 'always', rules: { prompt: false } }).some(item => item.kind === 'prompt'), 'Always my color shows the prompt chip');
});

function attach(options) {
  const { context, timers } = load(), decorations = [];
  const term = new Terminal({ cols: 60, rows: 4, allowProposedApi: true, theme: { foreground: '#dce6f6', background: '#0b1220' } });
  Object.defineProperty(term, 'element', { value: { getClientRects: () => [{}] } });
  term.registerDecoration = settings => { const element = { style: {}, setAttribute() {} }, d = { options: settings, element, disposed: false, onRender: fn => { fn(element); return { dispose() {} }; }, dispose() { this.disposed = true; } }; decorations.push(d); return d; };
  term.refresh = () => {};
  const controller = context.NTTerminalHighlight.attach(term, options);
  const flush = () => { for (let n = 0; timers.size && n < 64; n++) { const jobs = [...timers.values()]; timers.clear(); jobs.forEach(fn => fn()); } };
  const write = text => new Promise(resolve => term.write(text, resolve)).then(() => new Promise(resolve => setTimeout(resolve, 20))).then(flush);
  const at = (x, row = 0) => decorations.find(d => !d.disposed && d.options.marker.line === row && d.options.x === x);
  return { term, controller, flush, write, at, dispose() { controller.dispose(); term.dispose(); } };
}

test('a prompt name the device colors itself takes your color, or plain text, only when asked', async () => {
  // Debian-style PS1: green user@host, blue path.
  const line = '\x1b[01;32mbiqu@CR10SPro\x1b[00m:\x1b[01;34m~\x1b[00m$ ';
  const h = attach({ theme: 'dark' });
  await h.write(line);
  assert.equal(h.at(0)?.options.foregroundColor, undefined, 'Automatic keeps the color the device sent');
  h.controller.setAppearance({ theme: 'dark', prompt: 'always', colors: { prompt: '#ffaa00' } }); h.flush();
  assert.equal(h.at(0)?.options.foregroundColor, '#ffaa00'); assert.equal(h.at(0)?.options.width, 13);
  assert.equal(h.at(14)?.options.foregroundColor, undefined, 'the path keeps the device color');
  h.controller.setAppearance({ theme: 'dark', prompt: 'off' }); h.flush();
  assert.equal(h.at(0)?.options.foregroundColor, '#dce6f6', 'No color shows the name in the normal text color');
  assert.equal(h.at(0)?.element.style.borderBottom, 'none');
  assert.equal(h.term.buffer.active.getLine(0).getCell(0).getFgColor(), 2, 'the received cells are never changed (still ANSI green)');
  h.dispose();
});

test('URL underlines follow their own switch in a live terminal', async () => {
  const h = attach({ theme: 'dark', underlines: true, linkUnderlines: false });
  await h.write('See https://example.com/x from 192.0.2.10');
  assert.equal(h.at(4)?.element.style.borderBottom, 'none', 'URL underline is off');
  assert.match(h.at(31)?.element.style.borderBottom || '', /2px solid/, 'other highlights keep their underline');
  h.controller.setAppearance({ theme: 'dark', underlines: false, linkUnderlines: true }); h.flush();
  assert.match(h.at(4)?.element.style.borderBottom || '', /1px solid/, 'URL underline is on while others are off');
  assert.equal(h.at(31)?.element.style.borderBottom, 'none');
  h.dispose();
});
