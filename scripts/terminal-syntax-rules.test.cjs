'use strict';
// MobaXterm-style terminal syntax rule sets (v1.21.6).
const assert = require('node:assert/strict'), { test } = require('node:test');
const R = require('../web/assets/terminal-syntax-rules.js');
const spans = (set, line) => R.compile(set).match(line).map(hit => hit.kind.slice(4) + ':' + line.slice(hit.start, hit.end));
const regexSet = (rules, extra = {}) => ({ name: 'Test', regex: true, caseSensitive: false, rules, ...extra });

test('POSIX bracket expressions from MobaXterm files convert to equivalent JavaScript', () => {
  // "[]...]" starts with a literal ], "[^\]" means "not a backslash".
  assert.deepEqual(spans(regexSet({ red: '[=>"\':.,;({\\[][ ]*(false|no)[ ]*[]=>"\':.,;)} ]' }), 'state = false; done'), ['red:false']);
  assert.deepEqual(spans(regexSet({ red: '[^\\](\\$\\([^(\\))]+\\))' }), 'x=$(date) y=\\$(skip)'), ['red:$(date)']);
  assert.deepEqual(spans(regexSet({ green: '([[:digit:]]+ms)' }), 'time 12ms'), ['green:12ms']);
  assert.deepEqual(spans(regexSet({ cyan: '\\<info\\>' }), 'some info here, informal'), ['cyan:info']);
  const converted = R.convert('[^A-Za-z_&-](ok)[]x]');
  assert.doesNotThrow(() => new RegExp(converted.source));
  assert.equal(R.convert('(?i)ERROR').caseless, true);
});

test('the ¨ line markers match the start and end of each line', () => {
  const set = regexSet({ green: '(¨( *)?#[^¨]+)+', red: '[^A-Za-z_&-](error)[^A-Za-z_-]' });
  assert.deepEqual(spans(set, '  # a comment line'), ['green:# a comment line']);
  assert.deepEqual(spans(set, 'error at start'), ['red:error']);
  assert.deepEqual(spans(set, 'ends with error'), ['red:error']);
  assert.deepEqual(spans(set, 'echo "# not a comment"'), []);
});

test('the first capture group is colored, without surrounding blanks; otherwise the whole match', () => {
  assert.deepEqual(spans(regexSet({ red: '[^A-Za-z_&-](failed)[^A-Za-z_-]' }), 'login failed now'), ['red:failed']);
  assert.deepEqual(spans(regexSet({ red: 'fail(ed)?' }), 'it failed'), ['red:failed']);
  assert.deepEqual(spans(regexSet({ green: '[=:] *(true|yes)' }), 'enabled: yes'), ['green:yes']);
  assert.deepEqual(spans(regexSet({ yellow: 'warn[a-z]*' }), 'a warning'), ['yellow:warning']);
});

test('word lists match whole words and phrases, with optional case sensitivity', () => {
  const set = { name: 'Words', regex: false, caseSensitive: false, rules: { cyan: 'select, from, inner join', red: 'drop' } };
  assert.deepEqual(spans(set, 'SELECT a FROM t INNER   JOIN u; dropped drop'), ['cyan:SELECT', 'cyan:FROM', 'cyan:INNER   JOIN', 'red:drop']);
  assert.deepEqual(spans({ ...set, caseSensitive: true }, 'SELECT select'), ['cyan:select']);
  assert.deepEqual(spans({ ...set, rules: { cyan: 'a.b, (x)' } }, 'a.b axb (x)'), ['cyan:a.b', 'cyan:(x)']);
});

test('earlier colors win where matches overlap; underline can add to a colored word', () => {
  const set = regexSet({ red: '(refused)', green: '(connection refused)', underline: '(https://[^ ]+)', cyan: '(example)' });
  assert.deepEqual(spans(set, 'connection refused'), ['red:refused']);
  const hits = spans(regexSet({ cyan: '(example)', underline: '(https://[^ ]+)' }), 'see https://example.com now');
  assert.deepEqual(hits.sort(), ['cyan:example', 'underline:https://example.com']);
});

test('slow or invalid patterns are reported per color and never applied', () => {
  for (const pattern of ['(a+)+$', '(x+x+)+y', '(a|aa)+$']) {
    const compiled = R.compile(regexSet({ red: pattern, green: '(ok)' }));
    assert.match(compiled.errors.red || '', /too slow/, pattern);
    assert.deepEqual(compiled.match('ok aaaaaaaaaaaaaaaaaaaaaaaaaaaaa!').map(hit => hit.kind), ['syn-green']);
  }
  const invalid = R.compile(regexSet({ yellow: '(unclosed', cyan: 'x{2,1}' }));
  assert.match(invalid.errors.yellow, /not a valid regular expression/);
  assert.match(invalid.errors.cyan, /not a valid regular expression/);
  assert.match(R.compileColor('x'.repeat(8001), true, false).error, /longer than 8000/);
  assert.equal(R.normalize({ rules: { red: 'x'.repeat(9000) } }).rules.red.length, 8000);
});

test('MobaXterm .msyn files load from Windows-1252 and save back without losing characters', () => {
  const text = '[CustomSyntax]\r\nName=Ma liste\r\nUseRegex=1\r\nUnderline=\r\nRed=(¨#[^¨]+)|informação\r\nGreen=framgångsrik\r\nYellow=\r\nBlue=\r\nMagenta=\r\nCyan=\r\nBlinking=\r\nCaseSensitive=1\r\n\r\n';
  const bytes = R.encode(text);
  assert.equal(bytes.includes(0xa8), true, '¨ is byte A8 in Windows-1252');
  assert.equal(bytes.includes(0xe3) && bytes.includes(0xe5), true);
  const set = R.parseMsyn(bytes);
  assert.equal(set.name, 'Ma liste'); assert.equal(set.regex, true); assert.equal(set.caseSensitive, true);
  assert.equal(set.rules.red, '(¨#[^¨]+)|informação'); assert.equal(set.rules.green, 'framgångsrik');
  assert.deepEqual(spans(set, '# note'), ['red:# note']);
  const again = R.parseMsyn(R.encode(R.toMsyn(set)));
  assert.deepEqual(again, set);
  assert.equal(R.parseMsyn(new TextEncoder().encode('﻿[CustomSyntax]\nName=UTF\nUseRegex=0\nCyan=a,b\n')).rules.cyan, 'a,b');
  assert.throws(() => R.parseMsyn(R.encode('[Other]\nName=x')), /not a MobaXterm syntax file/);
});

test('saved preferences keep only valid custom rule sets and fall back to device highlighting', () => {
  const custom = Array.from({ length: 55 }, (_, i) => ({ id: 'custom-' + i, name: 'Set ' + i, rules: { red: 'x' } }));
  const prefs = R.preferences({ active: 'custom-3', custom: [{ id: 'standard', name: 'Clash' }, { id: 'Bad Id' }, { id: 'custom-3', name: 'Dup' }, ...custom] });
  assert.equal(prefs.custom.length, 50); assert.equal(prefs.custom[0].name, 'Dup'); assert.equal(prefs.active, 'custom-3');
  assert.equal(R.preferences({ active: 'missing' }).active, 'smart');
  assert.equal(R.preferences({ active: 'network' }).active, 'network');
  assert.equal(R.preferences(null).active, 'smart');
  assert.equal(R.normalize({ name: 'x'.repeat(200), rules: { red: 'a\nb' } }).name.length, 80);
  assert.equal(R.normalize({ rules: { red: 'a\nb' } }).rules.red, 'a b');
  assert.equal(R.newId(prefs, 'Set 1'), 'custom-set-1');
});

test('built-in rule sets compile and recognize their kinds of words', () => {
  for (const set of R.BUILT_IN) assert.deepEqual(R.compile(set).errors, {}, set.id);
  const standard = R.builtIn('standard'), network = R.builtIn('network'), shell = R.builtIn('shell'), perl = R.builtIn('perl'), sql = R.builtIn('sql');
  assert.deepEqual(spans(standard, 'Connection refused by 192.0.2.10'), ['red:Connection refused', 'magenta:192.0.2.10']);
  assert.deepEqual(spans(standard, 'Login successful; warning: low disk; info, see https://example.com/x'), ['green:successful', 'yellow:warning', 'yellow:low disk', 'cyan:info', 'underline:https://example.com/x']);
  assert.deepEqual(spans(network, 'GigabitEthernet1/0/2 is down, line protocol is down'), ['yellow:GigabitEthernet1/0/2', 'red:down', 'red:down']);
  assert.deepEqual(spans(network, ' no shutdown'), ['red:no', 'red:shutdown']);
  assert.deepEqual(spans(network, ' description Uplink to core'), ['green:description Uplink to core']);
  assert.deepEqual(spans(network, '! end'), ['blue:! end']);
  assert.deepEqual(spans(network, 'interface Vlan10'), ['magenta:interface', 'yellow:Vlan10']);
  assert.deepEqual(spans(shell, 'if [ -f "$(which bash)" ]; then echo $HOME; fi # done'), ['magenta:if', 'red:$(which bash)', 'magenta:then', 'cyan:echo', 'yellow:$HOME', 'magenta:fi', 'green:# done']);
  assert.deepEqual(spans(perl, 'my $x = shift; die unless defined $x;'), ['yellow:my', 'cyan:$x', 'yellow:shift', 'red:die', 'magenta:unless', 'yellow:defined', 'cyan:$x']);
  assert.deepEqual(spans(sql, 'DROP TABLE logs; select * from t'), ['red:DROP', 'magenta:TABLE', 'cyan:select', 'cyan:from']);
  assert.deepEqual(R.legend(standard).map(item => item.color), ['red', 'green', 'yellow', 'magenta', 'cyan', 'underline']);
});

test('long lines and many matches stay bounded', () => {
  const set = R.builtIn('standard'), line = 'error '.repeat(600);
  assert.equal(R.compile(set).match(line).length <= R.LIMITS.hitsPerColor, true);
  assert.deepEqual(R.compile(set).match('x'.repeat(R.LIMITS.text + 1)), []);
});
