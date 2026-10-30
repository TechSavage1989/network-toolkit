'use strict';
const test = require('node:test'), assert = require('node:assert/strict'), fs = require('node:fs');
const go = fs.readFileSync('cmd/network-toolkit/terminal_sftp123_windows_test.go', 'utf8');
const driver = fs.readFileSync('scripts/terminal-sftp123-native-driver.js', 'utf8');
const input = fs.readFileSync('scripts/native-terminal-sftp123.ps1', 'utf8');
test('Terminal123 acceptance uses full native Terminal workspace and retained SSH, not an Editor stand-in', () => {
  assert.match(go, /a\.launchTerminalView = h\.launch/);
  assert.match(go, /ExpectedHostFingerprint: fingerprint/);
  assert.match(go, /accepted\.Load\(\) != 1/);
  assert.match(driver, /void detachTerminalWorkspace\(\)/);
  assert.match(driver, /NetworkToolkitTerminalSFTP\.load\(c\.destination, '\/'\)/);
  assert.match(driver, /#terminalSidebarSFTP \[data-tsftp-item=/);
  assert.doesNotMatch(driver, /new (?:DragEvent|DataTransfer|PointerEvent|MouseEvent)|dispatchEvent\(|\.prepareCopy\(/);
});
test('Terminal123 native copy cases retain source, exact hierarchy, cancellation and own-window finish checks', () => {
  for (const mode of ['Copy', 'CancelBefore', 'CancelDuring']) assert.ok(go.includes('"' + mode + '"'));
  assert.match(go, /filepath\.Join\(dest, "tree", "empty"\)/);
  assert.match(go, /!bytes\.Equal\(got, want\)/);
  assert.match(go, /expectedOwner = "terminal:" \+ view/);
  assert.match(go, /input\.SaveDialogCount != 0 \|\| input\.FixtureHostWindowCount != 1/);
  assert.match(driver, /current\(\) !== original\.entry/);
  assert.match(go, /len\(cases\) != 8/);
});
test('negative control only cancels the exact fixture native Save dialog and checks original122 tree', () => {
  assert.match(go, /60b02cae25c1bfaf14d0c15a038706d193c99a37/);
  assert.match(go, /"git", "diff", "--exit-code", "HEAD"/);
  assert.match(input, /owner==pid&&IsWindowVisible\(h\)&&s\.ToString\(\)=="#32770"/);
  assert.match(input, /\$title -ne 'Save Network Toolkit export'/);
  assert.match(input, /Fixture-Window \$OwnerProcessID \$ExecutablePath/);
  assert.match(input, /PostMessage\(\$dialog,0x111,\[IntPtr\]2,\[IntPtr\]::Zero\)/);
  assert.match(input, /\$saveDialogs\.Count -ne 1/);
  assert.doesNotMatch(input, /SendKeys|Stop-Process|taskkill|Set-Clipboard|VK_RETURN/);
});
test('normal123 physical drag timings preserve the original121 gesture, not a speed workaround', () => {
  const old = fs.readFileSync('scripts/native-sftp-direct121.ps1', 'utf8');
  assert.equal(input.match(/\$timing=@\{press=150;[^\n]+/)[0], old.match(/\$timing=@\{press=150;[^\n]+/)[0]);
  assert.match(input, /if\(\$Mode -eq 'BaselineSaveDialog'\)\{Mark-Input 'baseline-real-hover';Start-Sleep -Milliseconds 1500\}/);
  assert.match(input, /'UIItemsView'/);
  assert.match(input, /Visible-Point \$owner \$source;Visible-Point \$explorer \$to/);
});
