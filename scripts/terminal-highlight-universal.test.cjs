'use strict';
// Universal device highlighting (v1.21.11): severities, CLI errors and results,
// ping, routing and service states, more vendors' interfaces and prompts.
const fs = require('node:fs'), vm = require('node:vm'), path = require('node:path'), assert = require('node:assert/strict'), { test } = require('node:test');
const context = {}; context.global = context; vm.createContext(context);
for (const name of ['terminal-shell-syntax.js', 'terminal-syntax-rules.js', 'terminal-highlight.js']) vm.runInContext(fs.readFileSync(path.join(__dirname, '../web/assets', name), 'utf8'), context);
const H = context.NTTerminalHighlight;
const spans = line => Array.from(H.analyze(line, 'ssh'), hit => hit.kind + ':' + line.slice(hit.start, hit.end));
const has = (line, expected) => { const got = spans(line); for (const item of expected) assert(got.includes(item), line + '\n  expected ' + item + '\n  got ' + JSON.stringify(got)); };

test('log and system-message severities are classified wherever logs put them', () => {
  has('*Nov  7 10:00:01.123: %LINK-3-UPDOWN: Interface GigabitEthernet1/0/2, changed state to down', ['error:%LINK-3-UPDOWN', 'interface:GigabitEthernet1/0/2', 'error:down']);
  has('*Nov  7 10:00:02: %SYS-5-CONFIG_I: Configured from console by admin', ['info:%SYS-5-CONFIG_I']);
  has('%BGP-4-MSGDUMP: unsupported or mal-formatted message', ['warning:%BGP-4-MSGDUMP']);
  has('2026-11-07T10:00:00Z ERROR db connection lost', ['error:ERROR']);
  has('2026-11-07 10:00:00,123 WARN  [main] cache nearly full', ['warning:WARN']);
  has('Nov  7 10:00:00 web01 nginx[812]: INFO started', ['info:INFO']);
  has('level=error msg="disk full"', ['error:error']);
  has('{"time":"x","level":"debug","msg":"tick"}', ['comment:debug']);
  has('[WARNING] low memory', ['warning:WARNING']);
  has('INFO: Server started', ['info:INFO']);
});

test('CLI errors and confirmations from network devices, shells and Windows', () => {
  has('% Invalid input detected at \'^\' marker.', ['error:Invalid input detected']);
  has('% Incomplete command.', ['error:Incomplete command']);
  has('bash: foo: command not found', ['error:command not found']);
  has("ls: cannot access '/x': No such file or directory", ['error:cannot access', 'error:No such file or directory']);
  has("'foo' is not recognized as an internal or external command,", ['error:is not recognized as an internal or external command']);
  has('error: syntax error, expecting <command>: foo', ['error:syntax error']);
  for (const line of ['[OK]', 'commit complete', 'Configuration committed successfully.']) assert.equal(spans(line)[0]?.startsWith('success:'), true, line);
  has('Building configuration...', ['info:Building configuration...']);
});

test('ping, routing neighbor and service states', () => {
  has('5 packets transmitted, 5 received, 0% packet loss, time 4005ms', ['success:0% packet loss']);
  has('5 packets transmitted, 3 received, 40% packet loss', ['warning:40% packet loss']);
  has('5 packets transmitted, 0 received, 100% packet loss', ['error:100% packet loss']);
  has('    Packets: Sent = 4, Received = 0, Lost = 4 (100% loss),', ['error:(100% loss)']);
  has('Success rate is 100 percent (5/5), round-trip min/avg/max = 1/1/2 ms', ['success:Success rate is 100 percent']);
  has('Success rate is 0 percent (0/5)', ['error:Success rate is 0 percent']);
  has('192.0.2.2       4 65002       0       0        0    0    0 never    Idle', ['address:192.0.2.2', 'warning:Idle']);
  has('192.0.2.3       65003      120     118       0       0    1:02:03 Establ', ['success:Establ']);
  has('192.0.2.10        1   FULL/DR         00:00:35    10.0.0.2        Gi0/0', ['success:FULL/DR', 'interface:Gi0/0']);
  has('192.0.2.11        1   INIT/DROTHER    00:00:31    10.0.0.3        Gi0/1', ['warning:INIT/DROTHER']);
  has('     Active: active (running) since Fri 2026-11-07 09:00:00 UTC; 1h ago', ['success:active (running)']);
  has('     Active: failed (Result: exit-code) since Fri 2026-11-07', ['error:failed']);
  has('Gi1/0/1             Desg FWD 4         128.1    P2p', ['success:FWD']);
  has('Gi1/0/2             Altn BLK 4         128.2    P2p', ['warning:BLK']);
  has('Gi1/0/3   err-disabled 1  auto   auto 10/100/1000BaseTX', ['error:err-disabled']);
});

test('interfaces and prompts from more vendors and from Windows hosts', () => {
  has('XGigabitEthernet0/0/1   up    up       0%  0%  0  0', ['interface:XGigabitEthernet0/0/1', 'success:up']);
  has('10GE1/0/1                  up       up        0.01%  0.01%', ['interface:10GE1/0/1']);
  has('Eth-Trunk1                 down     down', ['interface:Eth-Trunk1', 'error:down']);
  has('Ethernet1                  connected    1      full   10G', ['interface:Ethernet1', 'success:connected']);
  has('sfp-sfpplus1  R  10Gbps', ['interface:sfp-sfpplus1']);
  has('interface Po1', ['interface:Po1']);
  has('switch#copy running-config startup-config', ['prompt:switch#', 'command:copy']);
  has('PS C:\\Users\\admin> Get-Service sshd', ['prompt:PS C:\\Users\\admin>']);
  has('C:\\Users\\admin>ipconfig', ['prompt:C:\\Users\\admin>']);
  has('[~HUAWEI-GigabitEthernet0/0/1]display this', ['prompt:[~HUAWEI-GigabitEthernet0/0/1]', 'command:display']);
  assert.equal(spans('admin@PA-VM> show interface all').includes('interface:all'), false, 'a named value after "interface" is not an interface');
});

test('ordinary prose and banners stay plain', () => {
  for (const line of ['The server is active and the team reported an error last week.', 'Please read the INFO file before installing.', 'We saw a 2% loss in revenue this quarter.',
    'Active users: 42', 'Warning signs include fatigue and stress.', 'The command not found in the manual was added later.', 'Copyright 2017-2026 Example Vendor. User "admin" has logged in.',
    'Welcome to Ubuntu 24.04 LTS (GNU/Linux 6.8.0 x86_64)', 'This system is for authorized users only. All activity may be monitored.', 'Remember to check level=high water marks in the tank.',
    'Notice: the office will be closed on Friday.', 'The Po1 project starts soon.', '[1]+  Done                    sleep 5', '[OK] means the step passed in our checklist', 'Idle hands are the devil\'s workshop', 'In C# you can write async code.'])
    assert.deepEqual(spans(line), [], line);
});

test('the new Info kind is part of the legend, colors and rule switches', () => {
  const items = H.legend('dark', {});
  const info = items.find(item => item.kind === 'info');
  assert(info, 'legend lists Info / notice'); assert.match(info.color, /^#[0-9a-f]{6}$/);
  assert.equal(H.legend('light', { colors: { info: '#123456' } }).find(item => item.kind === 'info').color, '#123456');
  assert.equal(H.analyze('INFO: x', 'ssh', { info: false }).length, 0, 'Info can be turned off like any other rule');
  const go = fs.readFileSync(path.join(__dirname, '../cmd/network-toolkit/terminal_appearance.go'), 'utf8');
  assert.match(go, /"success", "info"\}/, 'Go keeps Info colors and switches');
});
