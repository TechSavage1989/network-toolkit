'use strict';
// Optional installed-app layout regression. Every app asset and API response
// comes from the executable's HTTP server. No asset injection or API mocks.
// NTK_APP_EXECUTABLE=/path/to/app CHROME_EXECUTABLE=/path/to/chrome \
//   NTK_BASELINE_EXECUTABLE=/path/to/1.17.13 node scripts/terminal-sidebar-layout-live.cjs
const assert = require('node:assert/strict');
const fs = require('node:fs');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

const HELP = 'Usage: NTK_APP_EXECUTABLE=/absolute/path/to/app [CHROME_EXECUTABLE=/path/to/chrome] [NTK_BASELINE_EXECUTABLE=/path/to/1.17.13] [NTK_BROWSER_OUTPUT_DIR=/path/to/results] node scripts/terminal-sidebar-layout-live.cjs\nUses installed Playwright/Chromium, temporary app data and a real local Telnet peer. Nothing is downloaded or injected.';
if (process.argv.includes('--help')) { console.log(HELP); process.exit(0); }
if (!process.env.NTK_APP_EXECUTABLE) { console.error(HELP); process.exit(2); }
function loadPlaywright() {
  try { return require('playwright'); }
  catch (error) {
    if (error.code !== 'MODULE_NOT_FOUND') throw error;
    if (process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES) return require(path.join(process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES, 'playwright'));
    throw new Error('Install Playwright in the development environment to run this optional check.');
  }
}

const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
const viewports = [{ width: 1450, height: 1000 }, { width: 1024, height: 768 }];
const report = { binary: path.resolve(process.env.NTK_APP_EXECUTABLE), checks: [], geometry: [], baseline: [], comparisons: [], skipped: [] };
const applications = [], contexts = [], sockets = new Set();
const output = process.env.NTK_BROWSER_OUTPUT_DIR ? path.resolve(process.env.NTK_BROWSER_OUTPUT_DIR) : null;
let browser, peer, failure;
function pass(label) { report.checks.push(label); console.log('PASS ' + label); }

function makePeer() {
  return net.createServer(socket => {
    sockets.add(socket); socket.on('close', () => sockets.delete(socket)); socket.on('error', () => {});
    socket.write('layout-switch# ');
    let line = '', negotiation = 0, sub = false, subIAC = false;
    socket.on('data', data => {
      for (const byte of data) {
        if (sub) { if (subIAC && byte === 240) { sub = false; subIAC = false; } else subIAC = byte === 255; continue; }
        if (negotiation === 1) { if (byte === 250) { sub = true; negotiation = 0; } else if ([251, 252, 253, 254].includes(byte)) negotiation = 2; else negotiation = 0; continue; }
        if (negotiation === 2) { negotiation = 0; continue; }
        if (byte === 255) { negotiation = 1; continue; }
        if (byte === 3) { socket.write('\r\nInterrupted\r\nlayout-switch# '); line = ''; }
        else if (byte === 13) { socket.write(line + '\r\nRESULT ' + line + '\r\nlayout-switch# '); line = ''; }
        else if (byte === 127 || byte === 8) line = line.slice(0, -1);
        else if (byte !== 0 && byte !== 10) line += String.fromCharCode(byte);
      }
    });
  });
}

async function startApp(executable, terminal = false) {
  const app = { executable, dataDir: fs.mkdtempSync(path.join(os.tmpdir(), 'ntk-sidebar-layout-')), log: '' };
  app.process = spawn(executable, ['-no-browser', '-port', '0', '-data-dir', app.dataDir, ...(terminal ? ['-terminal'] : [])], { stdio: ['ignore', 'pipe', 'pipe'] });
  applications.push(app);
  app.process.stdout.on('data', data => { app.log += data; });
  app.process.stderr.on('data', data => { app.log += data; });
  let spawnError; app.process.on('error', error => { spawnError = error; });
  for (let attempt = 0; attempt < 150; attempt++) {
    if (spawnError) throw spawnError;
    if (app.process.exitCode !== null) throw new Error('App exited before serving HTTP:\n' + app.log);
    const base = app.log.match(/http:\/\/127\.0\.0\.1:\d+\//)?.[0];
    if (base) { try { if ((await fetch(base)).ok) { app.base = base; return app; } } catch (_) {} }
    await delay(100);
  }
  throw new Error('App did not serve HTTP within 15 seconds:\n' + app.log);
}

async function pageFor(app, viewport = viewports[0]) {
  const context = await browser.newContext({ viewport }); contexts.push(context);
  const page = await context.newPage(); page.setDefaultTimeout(15000);
  page.appErrors = []; page.assetFailures = [];
  page.on('pageerror', error => page.appErrors.push(error.message));
  page.on('response', response => { if (response.url().includes('/assets/') && response.status() >= 400) page.assetFailures.push({ url: response.url(), status: response.status() }); });
  await page.goto(app.base, { waitUntil: 'domcontentloaded' });
  const navigation = page.locator('[data-page="terminal"]');
  if (await navigation.isVisible()) await navigation.click();
  await page.locator('#terminalStage').waitFor({ state: 'visible' });
  await settle(page);
  return page;
}
async function settle(page) {
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  await page.waitForTimeout(130);
}
async function measure(page, label) {
  return page.evaluate(label => {
    const geometry = selector => {
      const element = document.querySelector(selector);
      if (!element || !element.getClientRects().length) return null;
      const r = element.getBoundingClientRect();
      return { x: r.x, y: r.y, width: r.width, height: r.height, bottom: r.bottom, right: r.right, scrollWidth: element.scrollWidth, clientWidth: element.clientWidth, scrollHeight: element.scrollHeight, clientHeight: element.clientHeight };
    };
    return { label, viewport: { width: innerWidth, height: innerHeight }, stage: geometry('#terminalStage'), composer: geometry('#terminalMacroComposer'), sidebar: geometry('#terminalSessionSidebar'), workspace: geometry('#terminalWorkspace'), panel: geometry('#terminalMacroPanel'), documentWidth: document.documentElement.scrollWidth };
  }, label);
}
function assertFits(result) {
  assert(result.stage && result.stage.height >= 100, result.label + ': terminal must remain usable ' + JSON.stringify(result));
  assert(result.documentWidth <= result.viewport.width + 2, result.label + ': document horizontal overflow');
  for (const key of ['workspace', 'sidebar']) {
    const area = result[key];
    if (area) assert(area.scrollWidth <= area.clientWidth + 2, result.label + ': ' + key + ' horizontal overflow ' + JSON.stringify(area));
  }
  assert(result.stage.bottom <= result.viewport.height + 2, result.label + ': terminal extends below the viewport');
}
async function connect(page) {
  await page.locator('#terminalStage [data-terminal-new-type="telnet"]').first().click();
  await page.fill('#con-host', '127.0.0.1'); await page.fill('#con-port', String(peer.address().port));
  await page.fill('#terminalProfileName', 'Layout QA peer');
  await page.click('[data-terminal-action="save-profile"]');
  await page.click('#terminalConnectButton');
  await page.locator('#terminalConnectForm').waitFor({ state: 'detached' });
  await page.waitForFunction(() => currentTerminal()?.info?.state === 'connected' && !!currentTerminal()?.xterm);
  await settle(page);
}
async function createMacro(page, { legacy = false } = {}) {
  if (legacy) await page.click('#terminalMacrosToggle');
  else await selectRail(page, 'macros');
  const libraryTab = page.locator('#terminalMacroLibraryTab');
  if (await libraryTab.isVisible()) await libraryTab.click();
  await page.locator('#terminalMacroPanel [data-macro-action="new"]').first().click();
  await page.fill('[data-macro-edit="name"]', 'Layout QA macro');
  await page.fill('[data-macro-edit="commands"]', 'show clock');
  await page.check('[data-macro-edit="favorite"]');
  await page.click('[data-macro-action="editor-steps"]');
  await page.click('[data-macro-action="add-step"]');
  await page.selectOption('[data-step-index="1"] [data-macro-step="kind"]', 'pause');
  await page.click('[data-macro-action="save-editor"]');
  await page.waitForFunction(async () => (await api('/terminal-automation/library')).macros.some(m => m.name === 'Layout QA macro'));
  await settle(page);
}

async function captureIdentity(page) {
  return page.evaluateHandle(() => ({ root: document.querySelector('#terminalWorkspace'), stage: document.querySelector('#terminalStage'), grid: document.querySelector('#terminalPaneGrid'), entry: currentTerminal(), xterm: currentTerminal()?.xterm, element: currentTerminal()?.element }));
}
async function assertIdentity(page, identity, label) {
  assert(await page.evaluate(ref => ref.root === document.querySelector('#terminalWorkspace') && ref.stage === document.querySelector('#terminalStage') && ref.grid === document.querySelector('#terminalPaneGrid') && ref.entry === currentTerminal() && ref.xterm === currentTerminal()?.xterm && ref.element === currentTerminal()?.element && ref.element?.isConnected, identity), label + ': terminal DOM/session/xterm was replaced');
}
async function selectRail(page, tab) {
  await page.click('#terminalSidebarTab' + tab[0].toUpperCase() + tab.slice(1));
  await settle(page);
  assert.equal(await page.locator('[data-session-tab="' + tab + '"]').getAttribute('aria-selected'), 'true');
}
async function collapseSidebar(page) {
  await page.locator('#terminalSidebarContent .session-library-heading [data-session-action="collapse"]').click();
  await settle(page);
  assert.equal(await page.locator('#terminalSidebarContent').isVisible(), false, 'Heading arrow collapses sidebar content');
  assert(await page.locator('.terminal-sidebar-rail').isVisible(), 'Collapsed sidebar keeps its reopening tabs');
  assert(await page.locator('.terminal-sidebar-rail>#terminalSidebarVisibilityToggle').isVisible(), 'The same visibility button remains beside the closed sidebar');
  assert.equal(await page.locator('#terminalSidebarVisibilityToggle').getAttribute('aria-label'), 'Show sidebar');
  assert.equal(await page.locator('#terminalSidebarVisibilityToggle [data-panel-direction]').getAttribute('data-panel-direction'), 'right');
  assert.equal(await page.evaluate(() => document.activeElement?.dataset?.sessionTab), await page.evaluate(() => NetworkToolkitSessionSidebar.activeTab()), 'Collapse returns keyboard focus to the active rail tab');
}
async function checkSidebarResize(page, mode) {
  await page.setViewportSize(viewports[0]); await selectRail(page, 'saved');
  const identity = await captureIdentity(page), divider = page.locator('#terminalSidebarDivider'), sidebar = page.locator('#terminalSessionSidebar');
  assert.equal(await divider.getAttribute('role'), 'separator');
  const initial = await sidebar.evaluate(element => element.getBoundingClientRect().width), handle = await divider.boundingBox();
  await page.mouse.move(handle.x + handle.width / 2, handle.y + handle.height / 2); await page.mouse.down();
  await page.mouse.move(handle.x + handle.width / 2 + 70, handle.y + handle.height / 2, { steps: 6 }); await page.mouse.up(); await settle(page);
  const resized = await sidebar.evaluate(element => element.getBoundingClientRect().width);
  assert(resized >= initial + 50 && resized <= 480, 'Divider drag resizes within the saved width bounds');
  await divider.dblclick(); await settle(page);
  assert(await sidebar.evaluate(element => element.classList.contains('is-collapsed')), 'Double-click hides sidebar content');
  assert(await divider.isVisible(), 'Collapsed divider remains available to restore the sidebar');
  await divider.dblclick(); await settle(page);
  assert(Math.abs(await sidebar.evaluate(element => element.getBoundingClientRect().width) - resized) <= 1, 'Double-click restores the previous expanded width');
  await divider.press('Enter'); await settle(page);
  assert.equal(await page.locator('#terminalSidebarVisibilityToggle').getAttribute('aria-label'), 'Show sidebar');
  await page.locator('#terminalSidebarVisibilityToggle').click(); await settle(page);
  assert(Math.abs(await sidebar.evaluate(element => element.getBoundingClientRect().width) - resized) <= 1, 'Shared Show button restores the same expanded width');
  await divider.press('Home'); await divider.press('ArrowRight'); await settle(page);
  assert.equal(Math.round(await sidebar.evaluate(element => element.getBoundingClientRect().width)), 310, 'Arrow keys resize from the default width');
  await divider.press('Shift+ArrowRight'); await settle(page);
  assert.equal(Math.round(await sidebar.evaluate(element => element.getBoundingClientRect().width)), 350, 'Shift resizes in larger increments');
  await divider.press('End'); await settle(page);
  assert.equal(Math.round(await sidebar.evaluate(element => element.getBoundingClientRect().width)), 480);
  await page.setViewportSize(viewports[1]); await settle(page);
  const narrow = await measure(page, mode + ' responsive sidebar width'); assertFits(narrow); report.geometry.push(narrow);
  assert(narrow.sidebar.width <= 480 && narrow.sidebar.width <= narrow.workspace.width / 2, 'Window resizing bounds the visible sidebar width');
  assert.equal(await page.evaluate(() => state.terminalSidebarWidth), 480, 'Responsive layout retains the expanded width preference');
  await page.setViewportSize(viewports[0]); await settle(page);
  assert.equal(Math.round(await sidebar.evaluate(element => element.getBoundingClientRect().width)), 480, 'Widening window restores the saved expanded width');
  await page.waitForFunction(async () => (await api('/state')).terminalSidebarWidth === 480);
  await assertIdentity(page, identity, mode + ' divider resizing'); await identity.dispose();
  pass(mode + ': divider drag, keyboard and double-click resize/collapse preserve width and the live terminal');
}
async function contextMenu(page, kind) {
  await selectRail(page, kind);
  const entry = page.locator('[data-session-kind="' + kind + '"] [data-session-entry]').first();
  await entry.focus(); await page.keyboard.press('Shift+F10');
  const menu = page.locator('#sessionLibraryContextMenu'); await menu.waitFor({ state: 'visible' });
  for (const action of kind === 'saved' ? ['open', 'edit', 'rename', 'duplicate', 'remove'] : ['open', 'save', 'edit', 'remove']) assert(await menu.locator('[data-session-menu="' + action + '"]').isVisible(), kind + ': missing context action ' + action);
  await page.keyboard.press('Escape');
  assert(await entry.evaluate(element => element === document.activeElement), kind + ': Escape should return focus to the session');
}
async function checkGeometry(page, mode) {
  assert.equal(await page.locator('#sessionSidebarToggle').count(), 0, 'Toolbar has no redundant Sidebar toggle');
  assert.equal(await page.locator('#terminalMacrosToggle').count(), 0, 'Toolbar has no redundant Macros toggle');
  assert.equal(await page.locator('[data-session-action="collapse"]').count(), 1, 'Sidebar has one heading collapse arrow');
  for (const viewport of viewports) {
    await page.setViewportSize(viewport); await selectRail(page, 'saved');
    const identity = await captureIdentity(page), initial = await measure(page, mode + ' Saved ' + viewport.width);
    assertFits(initial); report.geometry.push(initial);
    for (const tab of ['recent', 'macros']) {
      await selectRail(page, tab);
      if (tab === 'macros') {
        await page.click('#terminalMacroLibraryTab'); await settle(page);
      }
      const result = await measure(page, mode + ' ' + tab + ' ' + viewport.width); assertFits(result); report.geometry.push(result);
      assert(Math.abs(result.stage.height - initial.stage.height) <= 2, result.label + ': sidebar tab changed terminal height ' + JSON.stringify({ initial: initial.stage.height, actual: result.stage.height }));
      await assertIdentity(page, identity, result.label);
    }
    for (const view of ['Runs', 'Library']) {
      await page.click('#terminalMacro' + view + 'Tab'); await settle(page);
      const result = await measure(page, mode + ' Macros/' + view + ' ' + viewport.width); assertFits(result); report.geometry.push(result);
      assert(Math.abs(result.stage.height - initial.stage.height) <= 2, result.label + ': macro view changed terminal height');
      await assertIdentity(page, identity, result.label);
    }
    const composer = (await measure(page, 'composer')).composer;
    assert(composer && composer.height <= (viewport.width >= 1400 ? 48 : 88), 'Composer must stay compact ' + JSON.stringify({ viewport, composer }));
    await collapseSidebar(page);
    assert(await page.locator('#terminalSessionSidebar').evaluate(element => element.classList.contains('is-collapsed')), 'Collapse must keep a rail');
    assert(await page.locator('#terminalSidebarTabSaved').isVisible(), 'Saved rail tab remains available collapsed');
    const collapsed = await measure(page, mode + ' collapsed ' + viewport.width); assertFits(collapsed); report.geometry.push(collapsed);
    assert(collapsed.sidebar.width <= 50, 'Collapsed sidebar should be a narrow tab rail');
    assert(collapsed.stage.height >= initial.stage.height - 2, 'Collapsing sidebar reduced console height');
    await assertIdentity(page, identity, collapsed.label);
    await selectRail(page, 'saved');
    await identity.dispose();
    if (output) {
      await selectRail(page, 'macros'); await page.click('#terminalMacroLibraryTab'); await settle(page);
      await page.screenshot({ path: path.join(output, mode + '-' + viewport.width + '.png') });
    }
  }
  pass(mode + ': sidebar and macro tabs preserve stage height and live terminal identity at both desktop sizes');
}

async function checkActions(page) {
  await page.setViewportSize(viewports[0]);
  await contextMenu(page, 'saved'); await contextMenu(page, 'recent');
  await selectRail(page, 'saved');
  const saved = page.locator('#terminalSidebarTabSaved'); await saved.focus();
  for (const [key, expected] of [['ArrowDown', 'recent'], ['ArrowDown', 'macros'], ['Home', 'saved'], ['End', 'macros']]) {
    await page.keyboard.press(key);
    assert.equal(await page.evaluate(() => document.activeElement?.dataset?.sessionTab), expected, 'Rail keyboard focus ' + key);
    assert.equal(await page.locator('[data-session-tab="' + expected + '"]').getAttribute('aria-selected'), 'true');
  }
  await page.click('#terminalMacroLibraryTab'); await page.locator('#terminalMacroLibraryTab').focus();
  await page.keyboard.press('ArrowRight');
  assert.equal(await page.locator('#terminalMacroRunsTab').getAttribute('aria-selected'), 'true', 'Macro tabs support keyboard selection');
  await page.keyboard.press('Home');
  assert.equal(await page.locator('#terminalMacroLibraryTab').getAttribute('aria-selected'), 'true');
  const macroItem = page.locator('.macro-library-item').first();
  await macroItem.focus(); await page.keyboard.press('Shift+F10');
  for (const action of ['preview-confirm', 'edit', 'duplicate', 'favorite', 'export-one', 'delete']) assert(await page.locator('#terminalMacroMenu [data-macro-action="' + action + '"]').isVisible(), 'Macro context action ' + action);
  await page.keyboard.press('End');
  assert.equal(await page.evaluate(() => document.activeElement?.dataset?.macroAction), 'delete', 'Macro context menu keyboard navigation');
  await page.keyboard.press('Escape');
  assert(await macroItem.evaluate(element => element === document.activeElement), 'Macro context menu Escape returns item focus');
  pass('Saved, Recent and Macro context actions remain accessible; tab rails work with keyboard and return menu focus');

  await page.click('#terminalMacroRecord');
  await selectRail(page, 'saved');
  assert(await page.locator('#terminalMacroActivity').isVisible(), 'Recording activity visible while Saved is selected');
  assert.match(await page.locator('#terminalMacroActivity').innerText(), /record/i);
  await collapseSidebar(page);
  assert(await page.locator('#terminalMacroActivity').isVisible(), 'Recording activity visible with sidebar collapsed');
  await page.click('#terminalMacroActivity');
  await page.locator('[data-macro-action="record-discard"]').click();
  pass('Recording stays visible outside the Macros tab and with the sidebar collapsed');

  await page.locator('.macro-item-name').first().click(); await page.click('[data-macro-action="run-preview"]');
  await page.waitForFunction(async () => (await api('/terminal-automation/runs')).runs.some(run => run.state === 'paused'));
  if (output) {
    await selectRail(page, 'macros'); await page.click('#terminalMacroRunsTab'); await settle(page);
    await page.screenshot({ path: path.join(output, 'main-runs-1450.png') });
  }
  await selectRail(page, 'recent'); assert(await page.locator('#terminalMacroActivity').isVisible(), 'Run activity visible outside Macros');
  await collapseSidebar(page);
  assert(await page.locator('#terminalMacroActivity').isVisible(), 'Run activity visible with sidebar collapsed');
  await page.click('#terminalMacroActivity');
  await page.locator('[data-macro-action="control"][data-control="stop"]').first().click();
  await page.waitForFunction(async () => (await api('/terminal-automation/runs')).runs.every(run => !['running', 'paused'].includes(run.state)));
  pass('Real backend run activity remains visible outside Macros and stops without closing its Telnet session');
}

async function checkPreferences(page) {
  const sidebarWidth = await page.evaluate(() => state.terminalSidebarWidth);
  await selectRail(page, 'macros');
  await page.click('#terminalMacroLibraryTab');
  const confirmation = page.locator('#terminalMacroConfirmation');
  if (await confirmation.count()) {
    await confirmation.selectOption('multiple');
  }
  await selectRail(page, 'recent');
  await page.locator('[data-macro-action="hide-composer"]').click();
  // Wait on the public state endpoint so a navigation cannot race debounce.
  await page.waitForFunction(async () => { const saved = await api('/state'); return saved.terminalSidebarTab === 'recent' && saved.terminalComposerCollapsed === true && (!document.querySelector('#terminalMacroConfirmation') || saved.terminalMacroConfirmation === 'multiple'); });
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.locator('#terminalSidebarTabRecent').waitFor({ state: 'visible' }); await settle(page);
  assert.equal(await page.locator('#terminalSidebarTabRecent').getAttribute('aria-selected'), 'true');
  assert.equal(await page.evaluate(() => state.terminalSidebarWidth), sidebarWidth, 'Expanded sidebar width persists through reload');
  assert.equal(await page.locator('#terminalMacroComposer').isVisible(), false, 'Composer hide preference persists');
  if (await confirmation.count()) assert.equal(await page.evaluate(async () => (await api('/state')).terminalMacroConfirmation), 'multiple');
  await page.click('#terminalMacroComposerToggle'); await settle(page);
  pass('Selected sidebar tab, hidden composer and available confirmation preference persist through a real reload');
}

async function baseline() {
  if (!process.env.NTK_BASELINE_EXECUTABLE) return;
  const app = await startApp(path.resolve(process.env.NTK_BASELINE_EXECUTABLE)), page = await pageFor(app);
  await connect(page); await createMacro(page, { legacy: true });
  for (const viewport of viewports) {
    await page.setViewportSize(viewport); await settle(page);
    const result = await measure(page, '1.17.13 open macro library ' + viewport.width); report.baseline.push(result);
    console.log('BASELINE ' + JSON.stringify({ viewport, stageHeight: result.stage.height, macroPanelHeight: result.panel?.height, composerHeight: result.composer?.height }));
    if (output) await page.screenshot({ path: path.join(output, 'baseline-' + viewport.width + '.png') });
  }
  await page.context().close();
}

async function checkDetached(app, page) {
  const available = await page.evaluate(async () => (await api('/status')).terminalDetachAvailable);
  if (!available) { report.skipped.push('Detached view unavailable in this executable; lifecycle coverage remains in unit tests.'); return; }
  await page.waitForFunction(() => currentTerminal()?.info?.state === 'connected' && !!currentTerminal()?.xterm);
  // The per-pane header is intentionally hidden in a single-pane layout.
  // Its tab context menu exposes the same single-session detach action.
  await page.locator('#terminalTabs .terminal-tab.active').click({ button: 'right' });
  const move = page.locator('#terminalTabContextMenu [data-terminal-tab-action="move"]');
  await move.waitFor({ state: 'visible' });
  await page.waitForFunction(() => !document.querySelector('#terminalTabContextMenu [data-terminal-tab-action="move"]')?.disabled);
  const requests = [];
  const track = request => { if (request.method() === 'POST') requests.push(new URL(request.url()).pathname); };
  page.on('request', track);
  let response;
  try {
    [response] = await Promise.all([
      page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === '/api/terminal-views'),
      move.click()
    ]);
  } catch (error) {
    report.detachedDiagnostics = { posts: requests, state: await page.evaluate(() => ({ current: currentTerminal()?.info?.state, label: document.querySelector('[data-terminal-pane-move="0"]')?.textContent, disabled: document.querySelector('[data-terminal-pane-move="0"]')?.disabled, title: document.querySelector('[data-terminal-pane-move="0"]')?.title, notices: document.querySelector('#terminalNotices')?.textContent, toast: Array.from(document.querySelectorAll('.toast')).map(element => element.textContent) })) };
    if (output) await page.screenshot({ path: path.join(output, 'detached-failure.png') });
    throw new Error(error.message + '\nDetached diagnostics: ' + JSON.stringify(report.detachedDiagnostics));
  } finally { page.off('request', track); }
  const detached = await response.json();
  assert(detached.url, 'Headless app must return its real detached-view bootstrap URL');
  const view = await page.context().newPage(); view.setDefaultTimeout(15000);
  const viewErrors = []; view.on('pageerror', error => viewErrors.push(error.message));
  await view.goto(detached.url, { waitUntil: 'domcontentloaded' });
  await view.waitForFunction(() => !!currentTerminal()?.xterm && terminalDetachedReady === true);
  assert(await view.locator('#terminalSidebarTabMacros').isVisible(), 'Single-session detached view exposes Macros rail');
  assert.equal(await view.locator('#terminalSidebarTabSaved').isVisible(), false, 'Restricted detached view must hide Saved');
  await view.click('#terminalSidebarTabMacros'); await settle(view);
  const identity = await captureIdentity(view), initial = await measure(view, 'detached Macros/Library'); assertFits(initial); report.geometry.push(initial);
  for (const tab of ['Runs', 'Library']) {
    await view.click('#terminalMacro' + tab + 'Tab'); await settle(view);
    const current = await measure(view, 'detached Macros/' + tab); assertFits(current); report.geometry.push(current);
    assert(Math.abs(current.stage.height - initial.stage.height) <= 2, 'Detached macro tabs must preserve console height');
    await assertIdentity(view, identity, 'detached ' + tab);
  }
  await identity.dispose();
  assert.deepEqual(viewErrors, [], 'Detached browser runtime errors');
  if (output) await view.screenshot({ path: path.join(output, 'detached-sidebar.png') });
  await view.close();
  pass('Actual detached-view HTTP bootstrap retains its live xterm and Macros-only sidebar');
}

(async () => {
  if (output) fs.mkdirSync(output, { recursive: true });
  peer = makePeer(); await new Promise(resolve => peer.listen(0, '127.0.0.1', resolve));
  browser = await loadPlaywright().chromium.launch({ executablePath: process.env.CHROME_EXECUTABLE || undefined, headless: true, args: ['--disable-dev-shm-usage', '--no-sandbox'] });
  await baseline();
  const mainApp = await startApp(report.binary), page = await pageFor(mainApp);
  await connect(page); await createMacro(page); await checkGeometry(page, 'main'); await checkSidebarResize(page, 'main'); await checkActions(page); await checkPreferences(page);
  assert.deepEqual(page.appErrors, [], 'Main browser runtime errors'); assert.deepEqual(page.assetFailures, [], 'Shipped asset HTTP failures');
  for (const old of report.baseline) {
    const current = report.geometry.find(item => item.label === 'main Macros/Library ' + old.viewport.width);
    const comparison = { viewport: old.viewport, previousStageHeight: old.stage.height, currentStageHeight: current.stage.height, gainedPixels: current.stage.height - old.stage.height };
    report.comparisons.push(comparison); console.log('COMPARISON ' + JSON.stringify(comparison));
    assert(comparison.gainedPixels > 0, 'Moving macro library to sidebar should increase console height');
  }
  await checkDetached(mainApp, page);
  const terminalApp = await startApp(report.binary, true), terminalPage = await pageFor(terminalApp);
  await connect(terminalPage); await createMacro(terminalPage); await checkGeometry(terminalPage, 'terminal-mode'); await checkSidebarResize(terminalPage, 'terminal-mode');
  assert.deepEqual(terminalPage.appErrors, [], 'Terminal-mode browser runtime errors'); assert.deepEqual(terminalPage.assetFailures, [], 'Terminal-mode asset HTTP failures');
})().catch(error => { failure = error; report.error = error.stack || String(error); console.error(report.error); process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close().catch(() => {});
  for (const socket of sockets) socket.destroy();
  if (peer) await new Promise(resolve => peer.close(resolve));
  for (const app of applications) {
    if (app.process.exitCode === null) { app.process.kill('SIGTERM'); await Promise.race([new Promise(resolve => app.process.once('exit', resolve)), delay(2000)]); if (app.process.exitCode === null) app.process.kill('SIGKILL'); }
    fs.rmSync(app.dataDir, { recursive: true, force: true });
  }
  report.passed = !failure;
  if (output) fs.writeFileSync(path.join(output, 'terminal-sidebar-layout-report.json'), JSON.stringify(report, null, 2) + '\n');
  if (!failure) console.log('PASS shipped sidebar layout regression: ' + report.checks.length + ' checks.');
});
