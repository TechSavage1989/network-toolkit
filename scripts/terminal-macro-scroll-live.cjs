'use strict';
// Installed-app regression: real HTTP assets/API, xterm and a real Telnet peer.
// No injected assets, mocked app functions or intercepted responses.
const assert = require('node:assert/strict'), fs = require('node:fs'), path = require('node:path'), os = require('node:os'), net = require('node:net'), { spawn } = require('node:child_process');
const HELP = 'Usage: NTK_APP_EXECUTABLE=/path/to/app [CHROME_EXECUTABLE=/path/to/chrome] [NTK_BROWSER_OUTPUT_DIR=/path/to/results] [NTK_PAGER_ONLY=1] node scripts/terminal-macro-scroll-live.cjs';
if (process.argv.includes('--help')) { console.log(HELP); process.exit(0); }
if (!process.env.NTK_APP_EXECUTABLE) { console.error(HELP); process.exit(2); }
function playwright() { try { return require('playwright'); } catch (error) { if (error.code !== 'MODULE_NOT_FOUND') throw error; return require(path.join(process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES || '', 'playwright')); } }
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
const report = { binary: path.resolve(process.env.NTK_APP_EXECUTABLE), checks: [], positions: [], pagerKeys: [], commands: [], failures: [] }, sockets = new Set();
const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ntk-macro-scroll-')), output = process.env.NTK_BROWSER_OUTPUT_DIR;
let app, browser, page, peer, log = '', number = 0, failure;
function pass(label) { report.checks.push(label); console.log('PASS ' + label); }
function makePeer() {
  let serial = 0;
  return net.createServer(socket => {
    const session = ++serial; sockets.add(socket); socket.on('error', () => {});
    let line = '', negotiation = 0, sub = false, subIAC = false, timer = null, waiting = false, waitingKey = 'space', more = null;
    const stop = () => { clearInterval(timer); timer = null; waiting = false; more = null; };
    socket.on('close', () => { stop(); sockets.delete(socket); });
    socket.write('scroll-switch-' + session + '# ');
    function run(command) {
      const count = command.startsWith('seed') ? 12050 : command.startsWith('stream') ? 1200 : 300;
      let sent = 0; socket.write(command + '\r\n');
      timer = setInterval(() => {
        let text = '';
        for (let row = 0; row < 100 && sent < count; row++, sent++) text += 'S' + session + ' ' + command + ' output ' + String(sent).padStart(6, '0') + ' interface connected\r\n';
        socket.write(text);
        if (sent >= count) {
          clearInterval(timer); timer = null;
          if (command.startsWith('paged')) {
            const fragments = command.includes('fragmented') ? ['\x1b[7', 'm--Mo', 're--', '\x1b[0m \b\b'] : command.includes('return') ? ['Press RETURN to continue: '] : command.includes('percentage') ? ['---(more 43%)--- '] : ['--More-- '];
            let phase = 0;
            const prompt = () => {
              waiting = true; waitingKey = command.includes('return') ? 'enter' : 'space';
              more = () => {
                phase++; socket.write('\r\nPAGE-CONTINUED ' + command + ' page ' + phase + '\r\n');
                if (command.includes('multi') && phase < 3) { socket.write(Array.from({ length: 60 }, (_, index) => 'PAGE ' + phase + ' ROW ' + index + '\r\n').join('')); prompt(); }
                else socket.write('scroll-switch-' + session + '# ');
              };
              fragments.forEach((text, index) => setTimeout(() => { if (!socket.destroyed && waiting) socket.write(text); }, index * 25));
            };
            prompt();
          } else socket.write('FINAL ' + command + '\r\nscroll-switch-' + session + '# ');
        }
      }, 45);
    }
    socket.on('data', data => { for (const byte of data) {
      if (sub) { if (subIAC && byte === 240) { sub = false; subIAC = false; } else subIAC = byte === 255; continue; }
      if (negotiation === 1) { if (byte === 250) { sub = true; negotiation = 0; } else if ([251, 252, 253, 254].includes(byte)) negotiation = 2; else negotiation = 0; continue; }
      if (negotiation === 2) { negotiation = 0; continue; } if (byte === 255) { negotiation = 1; continue; }
      if (byte === 3) { stop(); line = ''; socket.write('\r\nInterrupted\r\nscroll-switch-' + session + '# '); continue; }
      if (waiting && (waitingKey === 'space' && byte === 32 || waitingKey === 'enter' && byte === 13)) { report.pagerKeys.push({ session, key: byte === 32 ? 'Space' : 'Enter' }); waiting = false; const resume = more; more = null; resume(); continue; }
      if (byte === 13) { const command = line; line = ''; if (command) { report.commands.push({ session, command }); run(command); } }
      else if (byte === 127 || byte === 8) line = line.slice(0, -1); else if (byte !== 10 && byte !== 0) line += String.fromCharCode(byte);
    } });
  });
}
async function start() {
  app = spawn(report.binary, ['-no-browser', '-port', '0', '-data-dir', dataDir], { stdio: ['ignore', 'pipe', 'pipe'] });
  app.stdout.on('data', data => { log += data; }); app.stderr.on('data', data => { log += data; });
  let spawnError; app.on('error', error => { spawnError = error; });
  for (let attempt = 0; attempt < 150; attempt++) { if (spawnError) throw spawnError; if (app.exitCode !== null) throw new Error(log); const base = log.match(/http:\/\/127\.0\.0\.1:\d+\//)?.[0]; if (base) { try { if ((await fetch(base)).ok) return base; } catch (_) {} } await delay(100); }
  throw new Error('App failed to start: ' + log);
}
async function position(label, id, quiet = false) {
  const result = await page.evaluate(({ label, id }) => {
    const entry = id ? terminalSessions.get(id) : currentTerminal(), b = entry.xterm.buffer.active;
    const wrapper = entry.xterm._core._viewport?._scrollableElement, viewport = entry.element.querySelector('.xterm-scrollable-element'), rendered = entry.element.querySelector('.xterm-rows'), clean = value => String(value || '').replace(/\u00a0/g, ' ').trimEnd();
    const scrollPosition = wrapper?.getScrollPosition?.(), dimensions = wrapper?.getScrollDimensions?.();
    return { label, id: entry.info.id, visible: !!entry.element.getClientRects().length, follow: NetworkToolkitTerminalExtras.prefs(entry).autoScroll, consoleAutoPager: NetworkToolkitTerminalExtras.prefs(entry).autoPager, macroOwnsPager: NetworkToolkitTerminalMacros.ownsPager(entry), checkbox: currentTerminal() === entry ? document.querySelector('#terminalQuickFollow').checked : null, nativeUserScrolling: entry.xterm._core._bufferService.isUserScrolling, base: b.baseY, viewport: b.viewportY, rows: entry.xterm.rows, top: b.getLine(b.viewportY)?.translateToString(true), bottom: b.getLine(b.baseY + b.cursorY)?.translateToString(true), renderedTop: clean(rendered?.firstElementChild?.textContent), renderedBottom: clean(rendered?.lastElementChild?.textContent), expectedRenderedBottom: clean(b.getLine(b.viewportY + entry.xterm.rows - 1)?.translateToString(true)), domScrollTop: scrollPosition?.scrollTop, domScrollHeight: dimensions?.scrollHeight, domClientHeight: dimensions?.height, domBottomGap: dimensions && scrollPosition ? dimensions.scrollHeight - dimensions.height - scrollPosition.scrollTop : null, actualViewportPresent: !!viewport, pending: entry.renderPendingBytes || 0, epoch: entry._ntScrollEpoch || 0, bytes: entry.info.bytesReceived };
  }, { label, id });
  report.positions.push(result); if (!quiet) console.log('POSITION ' + JSON.stringify(result)); return result;
}
async function settled() { await page.waitForFunction(() => [...terminalSessions.values()].every(entry => !(entry.renderPendingBytes || 0))); await page.waitForTimeout(120); }
async function connect(newTab = false) {
  if (newTab) await page.locator('[data-terminal-action="new"]').first().click();
  await page.locator('#terminalStage [data-terminal-new-type="telnet"]').first().click();
  await page.fill('#con-host', '127.0.0.1'); await page.fill('#con-port', String(peer.address().port)); await page.click('#terminalConnectButton');
  await page.locator('#terminalConnectForm').waitFor({ state: 'detached' });
  await page.waitForFunction(() => currentTerminal()?.info?.state === 'connected' && !!currentTerminal()?.xterm);
  await page.waitForFunction(() => document.querySelector('#terminalMacroTargetCount')?.textContent === '1 terminal');
  return page.evaluate(() => currentTerminal().info.id);
}
async function run(commands, name, delayBefore = 0, timeoutMs = 15000) {
  const title = name + ' ' + (++number);
  await page.evaluate(({ title, commands, delayBefore, timeoutMs }) => NetworkToolkitTerminalMacros.requestRun({ id: 'scroll-' + title.replace(/\W/g, '-'), name: title, steps: [...(delayBefore ? [{ id: 'initial-delay', kind: 'delay', delayMs: delayBefore }] : []), ...commands.map((text, index) => ({ id: 'step-' + index, kind: 'command', text }))], waitForPrompt: true, timeoutMs, pacingMs: 100, autoPager: true }), { title, commands, delayBefore, timeoutMs });
  const preview = page.locator('[data-macro-action="run-preview"]');
  if (await preview.isVisible()) await preview.click();
  await page.waitForFunction(async title => (await api('/terminal-automation/runs')).runs.some(run => run.name === title), title);
  return page.evaluate(async title => (await api('/terminal-automation/runs')).runs.find(run => run.name === title).id, title);
}
async function finish(id) {
  const deadline = Date.now() + 25000;
  let result;
  do {
    result = await page.evaluate(async id => (await api('/terminal-automation/runs/' + id)).run, id);
    if (!['running', 'paused'].includes(result.state)) break;
    await delay(80);
  } while (Date.now() < deadline);
  assert.equal(result.state, 'completed', 'Macro must complete: ' + JSON.stringify(result)); await settled();
}
async function bottom() { await page.locator('[data-terminal-extra="scroll-bottom"]').click(); await settled(); }
async function follow(enabled) { await page.locator('#terminalQuickFollow').setChecked(enabled); await settled(); }
async function outputStarted(before, id) { await page.waitForFunction(({ before, id }) => (id ? terminalSessions.get(id) : currentTerminal()).info.bytesReceived > before + 18000, { before, id }); await settled(); }
async function wheelUp() {
  const box = await page.locator('.terminal-view:not(.hidden) .xterm-screen').boundingBox();
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2); await page.mouse.wheel(0, -700); await page.waitForTimeout(160);
}
async function scrollbarUp() {
  const area = await page.evaluate(() => { const element = currentTerminal().element.querySelector('.xterm-viewport'), rect = element.getBoundingClientRect(); return { x: rect.right - 4, y: rect.top + rect.height - 6, to: rect.top + rect.height * .4 }; });
  await page.mouse.move(area.x, area.y); await page.mouse.down(); await page.mouse.move(area.x, area.to, { steps: 7 }); await page.mouse.up(); await page.waitForTimeout(160);
}
function atBottom(value, label) { assert.equal(value.follow, true, label + ': follow preference should stay ON'); assert(Math.abs(value.base - value.viewport) <= 1, label + ': viewport must follow the bottom ' + JSON.stringify(value)); if (value.visible) { assert.equal(typeof value.domBottomGap, 'number', label + ': actual scrollbar wrapper must be measured'); assert(value.domBottomGap <= 2, label + ': actual DOM scrollbar must be at bottom ' + JSON.stringify(value)); assert.equal(value.renderedTop, String(value.top || '').trimEnd(), label + ': rendered first row differs from buffer'); assert.equal(value.renderedBottom, value.expectedRenderedBottom, label + ': rendered last row differs from buffer'); } }
async function check(label, operation) { try { await operation(); pass(label); } catch (error) { report.failures.push({ label, error: error.stack || String(error) }); console.error('FAIL ' + label + ': ' + error.message); } }
async function interrupt() { await page.locator('[data-terminal-action="control-c"]').first().click(); await page.waitForTimeout(180); }
async function startSaved(name) {
  await page.locator('#terminalSidebarTabMacros').click(); await page.locator('#terminalMacroLibraryTab').click();
  const previous = await page.evaluate(async () => (await api('/terminal-automation/runs')).runs.map(run => run.id));
  await page.locator('.macro-item-name').filter({ hasText: name }).click();
  if (await page.locator('[data-macro-action="run-preview"]').isVisible()) await page.locator('[data-macro-action="run-preview"]').click();
  for (let attempt = 0; attempt < 100; attempt++) {
    const id = await page.evaluate(async ({ previous, name }) => (await api('/terminal-automation/runs')).runs.find(run => run.name === name && !previous.includes(run.id))?.id, { previous, name });
    if (id) return id;
    await delay(50);
  }
  throw new Error('Saved macro did not start: ' + name);
}
async function pagerChecks() {
  await check('Visible macro paging control: OFF pauses at More; ON advances three pages and the next command', async () => {
    await bottom(); await page.locator('#terminalQuickPager').uncheck();
    await page.locator('#terminalSidebarTabMacros').click(); await page.locator('#terminalMacroLibraryTab').click();
    await page.locator('#terminalMacroPanel [data-macro-action="new"]').first().click();
    const paging = page.locator('[data-macro-edit="autoPager"]');
    assert(await paging.isVisible(), 'Paging checkbox must be visible without expanding Timing');
    assert.match(await paging.evaluate(element => element.closest('label').textContent), /--More--/i, 'Paging label should name the actual More prompt');
    assert.equal(await page.getByText('Timing and terminal behavior', { exact: true }).evaluate(element => element.closest('details').open), false, 'Timing stays collapsed');
    await page.fill('[data-macro-edit="name"]', 'Saved pager control'); await page.fill('[data-macro-edit="commands"]', 'paged-ui-multi\nafter-ui-pager');
    await paging.uncheck();
    await page.locator('[data-macro-action="save-editor"]').click();
    await page.waitForFunction(async () => (await api('/terminal-automation/library')).macros.some(m => m.name === 'Saved pager control'));
    const macro = await page.evaluate(async () => (await api('/terminal-automation/library')).macros.find(m => m.name === 'Saved pager control'));
    report.uiCreatedMacro = { autoPager: macro.autoPager, waitForPrompt: macro.waitForPrompt, timeoutMs: macro.timeoutMs };
    assert.equal(macro.autoPager, false);
    const id = await startSaved('Saved pager control'), keysBefore = report.pagerKeys.length;
    try {
      await page.waitForFunction(() => { const b = currentTerminal().xterm.buffer.active; return b.getLine(b.baseY + b.cursorY)?.translateToString(true).includes('--More--'); });
      await page.waitForTimeout(700);
      const state = await page.evaluate(async id => (await api('/terminal-automation/runs/' + id)).run.state, id);
      const paused = await position('saved-ui-paging-off'); atBottom(paused, 'Paging OFF with Auto scroll ON');
      assert.equal(state, 'running'); assert(paused.bottom.includes('--More--')); assert.equal(report.pagerKeys.length, keysBefore, 'Paging OFF must not send continuation');
    } finally { await interrupt(); }
    await page.locator('#terminalSidebarTabMacros').click(); await page.locator('#terminalMacroLibraryTab').click();
    await page.locator('.macro-library-item').filter({ hasText: 'Saved pager control' }).locator('[data-macro-action="edit"]').click();
    assert(await paging.isVisible()); await paging.check(); await page.locator('[data-macro-action="save-editor"]').click();
    await page.waitForFunction(async () => (await api('/terminal-automation/library')).macros.find(m => m.name === 'Saved pager control')?.autoPager === true);
    const automaticKeys = report.pagerKeys.length, commandsBefore = report.commands.length;
    await finish(await startSaved('Saved pager control'));
    atBottom(await position('saved-ui-paging-on'), 'Paging ON');
    assert.equal(report.pagerKeys.length - automaticKeys, 3, 'All three pages continue without manual Space');
    assert(report.commands.slice(commandsBefore).some(item => item.command === 'after-ui-pager'), 'Next command must execute without stray continuation input');
  });
  for (const [label, command] of [['plain trailing-space More', 'paged-plain'], ['fragmented ANSI/backspace More', 'paged-fragmented']]) await check(label + ' advances with macro autoPager enabled', async () => {
    try { await finish(await run([command, 'after-pager'], label, 0, 2200)); }
    finally { await position(label); await interrupt(); }
  });
}

(async () => {
  peer = makePeer(); await new Promise(resolve => peer.listen(0, '127.0.0.1', resolve));
  const base = await start(); browser = await playwright().chromium.launch({ executablePath: process.env.CHROME_EXECUTABLE || undefined, headless: true, args: ['--no-sandbox', '--disable-dev-shm-usage'] });
  const context = await browser.newContext({ viewport: { width: 1450, height: 1000 } });
  page = await context.newPage(); page.setDefaultTimeout(15000);
  report.browserErrors = []; page.on('pageerror', error => report.browserErrors.push(error.message));
  await page.goto(base, { waitUntil: 'domcontentloaded' }); await page.locator('[data-page="terminal"]').click();
  const first = await connect();
  await pagerChecks();
  if (process.env.NTK_PAGER_ONLY === '1') { if (report.failures.length) throw new Error(report.failures.length + ' pager checks failed'); return; }
  await finish(await run(['seed-history'], 'Seed history'));
  const seeded = await position('seeded'); assert(seeded.base >= 9900, 'Peer must fill the actual 10,000-line scrollback');

  await check('ON at bottom follows a macro through long output and pager steps', async () => {
    await bottom(); const id = await run(['stream-on', 'paged-on'], 'Follow ON'); await finish(id); atBottom(await position('on-at-bottom'), 'ON-at-bottom');
  });
  await check('ON while programmatically scrolled up returns to bottom for macro output', async () => {
    await bottom(); await page.evaluate(() => currentTerminal().xterm.scrollLines(-80));
    const initial = await position('on-programmatic-up-before'); assert(initial.follow && initial.base > initial.viewport);
    const id = await run(['stream-programmatic'], 'ON programmatic up', 1200);
    await settled();
    const started = await position('on-start-before-output');
    await finish(id); atBottom(await position('on-programmatic-up-after'), 'ON-programmatic-up');
    atBottom(started, 'ON-macro-start');
  });
  await check('ON remains at bottom after viewport and toolbar resize during a macro', async () => {
    await bottom(); const before = await position('resize-before'), id = await run(['stream-resize', 'paged-resize'], 'ON resizing');
    await outputStarted(before.bytes); await page.setViewportSize({ width: 1024, height: 768 }); await page.waitForTimeout(250);
    await position('resize-narrow'); await page.setViewportSize({ width: 1450, height: 1000 });
    await finish(id); atBottom(await position('resize-after'), 'ON-resize');
  });
  await check('Composer Send + Enter follows current ON preference from an earlier viewport', async () => {
    await bottom(); await page.evaluate(() => currentTerminal().xterm.scrollLines(-80));
    await page.fill('#terminalMacroCommand', 'stream-composer'); await page.locator('[data-macro-action="send"]').click();
    await page.waitForFunction(async () => (await api('/terminal-automation/runs')).runs.some(run => run.name === 'Sent commands'));
    const id = await page.evaluate(async () => (await api('/terminal-automation/runs')).runs.find(run => run.name === 'Sent commands').id);
    await finish(id); atBottom(await position('composer-on-after'), 'Composer-ON');
  });
  await check('OFF preserves visible output and stays OFF across commands and pager', async () => {
    await bottom(); await follow(false); await page.evaluate(() => currentTerminal().xterm.scrollLines(-80));
    const before = await position('off-before'); await finish(await run(['stream-off', 'paged-off'], 'Follow OFF')); const after = await position('off-after');
    assert.equal(after.follow, false); assert.equal(after.top, before.top, 'OFF must keep the same retained line in view');
  });
  for (const [label, interaction] of [['mouse wheel', wheelUp], ['scrollbar', scrollbarUp]]) await check(label + ' during a macro pauses follow through later commands and paging', async () => {
    await bottom(); const before = await position(label + '-start'), id = await run(['stream-' + label.replace(' ', '-'), 'paged-next'], label + ' manual pause');
    await outputStarted(before.bytes); await interaction(); const paused = await position(label + '-paused'); assert.equal(paused.follow, false, label + ' must disable follow');
    await finish(id); const after = await position(label + '-after'); assert.equal(after.follow, false); assert.equal(after.top, paused.top, label + ' must retain visible line through later output');
  });
  await check('Scroll to bottom resumes follow and Ctrl+C resumes it without closing session', async () => {
    await bottom(); atBottom(await position('explicit-bottom'), 'Explicit bottom');
    await follow(false); await page.evaluate(() => currentTerminal().xterm.scrollLines(-80)); await page.locator('[data-terminal-action="control-c"]').first().click();
    await settled(); atBottom(await position('ctrl-c'), 'Ctrl+C'); assert.equal(await page.evaluate(() => currentTerminal().info.state), 'connected');
  });
  await check('Two terminals keep independent follow preferences during one broadcast macro', async () => {
    await bottom(); const second = await connect(true); await follow(false);
    await page.selectOption('#terminalMacroTargetMode', 'all');
    await page.waitForFunction(() => document.querySelector('#terminalMacroTargetCount')?.textContent === '2 terminals');
    await finish(await run(['stream-broadcast', 'paged-broadcast'], 'Independent follow'));
    atBottom(await position('broadcast-first', first), 'First terminal'); assert.equal((await position('broadcast-second', second)).follow, false);
  });
  await check('A genuine detached session follows macro output with ON', async () => {
    const home = page;
    await page.locator('#terminalTabs .terminal-tab.active').click({ button: 'right' });
    const [response] = await Promise.all([
      page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === '/api/terminal-views'),
      page.locator('#terminalTabContextMenu [data-terminal-tab-action="move"]').click()
    ]);
    const result = await response.json(); assert(result.url, 'Real detached bootstrap URL');
    const view = await home.context().newPage(); view.setDefaultTimeout(15000);
    try {
      await view.goto(result.url, { waitUntil: 'domcontentloaded' });
      await view.waitForFunction(() => terminalDetachedReady && !!currentTerminal()?.xterm);
      page = view; await page.selectOption('#terminalMacroTargetMode', 'current');
      await page.waitForFunction(() => document.querySelector('#terminalMacroTargetCount')?.textContent === '1 terminal');
      await bottom(); await finish(await run(['stream-detached', 'paged-detached'], 'Detached ON'));
      atBottom(await position('detached-on'), 'Detached-ON');
    } finally { page = home; await view.close(); }
  });
  assert.deepEqual(report.browserErrors, []); if (report.failures.length) throw new Error(report.failures.length + ' scroll checks failed');
})().catch(error => { failure = error; report.error = error.stack || String(error); console.error(report.error); process.exitCode = 1; }).finally(async () => {
  report.passed = !failure;
  if (output) { fs.mkdirSync(output, { recursive: true }); fs.writeFileSync(path.join(output, 'terminal-macro-scroll-report.json'), JSON.stringify(report, null, 2) + '\n'); if (page) await page.screenshot({ path: path.join(output, 'terminal-macro-scroll.png') }).catch(() => {}); }
  if (browser) await browser.close(); for (const socket of sockets) socket.destroy(); if (peer) await new Promise(resolve => peer.close(resolve));
  if (app && app.exitCode === null) { app.kill('SIGTERM'); await Promise.race([new Promise(resolve => app.once('exit', resolve)), delay(2000)]); if (app.exitCode === null) app.kill('SIGKILL'); }
  fs.rmSync(dataDir, { recursive: true, force: true }); if (!failure) console.log('PASS installed-app macro scrolling: ' + report.checks.length + ' checks.');
});
