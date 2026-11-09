'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

const { NTK_EDITOR_FIXTURE_URL: url, NTK_EDITOR_FIXTURE_LEFT: left,
  NTK_EDITOR_FIXTURE_RIGHT: right, NTK_EDITOR_FIXTURE_HEAD: head, NTK_EDITOR_FIXTURE_TAIL: tail,
  NTK_EDITOR_FIXTURE_SIZE: size, NTK_EDITOR_FIXTURE_TOKEN: token } = process.env;
const output = process.env.TOOLKIT_TEST_OUTPUT || path.resolve('test-results/editor-large-file');
const report = { bytesPerFile: Number(size), browserErrors: [], passed: false };
fs.mkdirSync(output, { recursive: true });

async function waitForEngine(page, predicate, timeout = 30000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await page.evaluate(() => api('/editor-fixture/state'));
    if (predicate(state)) return;
    await new Promise(resolve => setTimeout(resolve, 25));
  }
  throw Error('The editor engine did not reach the expected fixture state.');
}

async function main() {
  const browser = await chromium.launch({ headless: true,
    ...(process.env.TOOLKIT_BROWSER_EXECUTABLE ? { executablePath: process.env.TOOLKIT_BROWSER_EXECUTABLE } : {}) });
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    page.on('pageerror', error => report.browserErrors.push(error.message));
    await page.addInitScript(value => { window.__networkToolkitNativeSession = value; }, token);
    report.phase = 'startup';
    await page.goto(url);
    await page.waitForFunction(() => window.NetworkToolkitEditorWindows?.bufferSupported());
    await page.evaluate(() => navigate('editor'));
    const text = page.locator('.nt-editor-code textarea[data-ed-text]').first();
    await text.waitFor();
    await page.evaluate(() => {
      const marks = window.editorTimings = { gaps: [], input: [], long: [] };
      let last = performance.now();
      const tick = now => { marks.gaps.push(now - last); last = now; requestAnimationFrame(tick); };
      requestAnimationFrame(tick);
      new PerformanceObserver(list => marks.long.push(...list.getEntries().map(entry => entry.duration))).observe({ type: 'longtask' });
      document.addEventListener('input', event => {
        if (!event.target.matches('textarea[data-ed-text]')) return;
        const start = performance.now();
        requestAnimationFrame(now => marks.input.push(Math.max(0, now - start)));
      }, true);
    });

    report.phase = 'open';
    const prefetched = new Set();
    page.on('response', response => {
      const request = response.request();
      if (!request.url().endsWith('/buffer') || !response.ok()) return;
      const body = request.postDataJSON();
      if (body?.action === 'window' && body.line > 0) prefetched.add(request.url());
    });
    const opened = Date.now();
    await page.evaluate(async name => { await NetworkToolkitFileEditor.openLocalPath(name); }, left);
    await page.evaluate(async name => { await NetworkToolkitFileEditor.openLocalPath(name); }, right);
    await page.waitForFunction(n => NetworkToolkitFileEditor.active?.diskMeta?.units === n &&
      !document.querySelector('.nt-editor-code textarea[data-ed-text]').disabled, Number(size));
    assert.match(await page.locator('[data-ed-message]').textContent(), /^Opened edited\.txt\.$/);
    report.openBothMs = Date.now() - opened;
    const activeUID = await page.evaluate(() => NetworkToolkitFileEditor.active.uid);
    assert.ok([...prefetched].some(url => url.endsWith('/' + activeUID + '/buffer')) ||
      await page.waitForResponse(response => response.ok() &&
        response.url().endsWith('/' + activeUID + '/buffer') &&
        response.request().postDataJSON()?.action === 'window' &&
        response.request().postDataJSON().line > 0, {timeout: 3000}));
    const firstScroll = await text.evaluate(editor => {
      editor.scrollTop = (editor.scrollHeight - editor.clientHeight) * 0.85;
      return editor.scrollTop / parseFloat(getComputedStyle(editor).lineHeight);
    });
    await page.waitForFunction(expected => {
      const page = NetworkToolkitFileEditor.active.diskSession.state().page;
      const editor = document.querySelector('.nt-editor-code textarea[data-ed-text]');
      const height = parseFloat(getComputedStyle(editor).lineHeight);
      return page.start > 0 && Math.abs((page.line - expected) * height + editor.scrollTop) <= 1;
    }, firstScroll, {timeout: 3000});

    report.phase = 'edit';
    await text.press('Control+End');
    await page.waitForFunction(n => {
      const editor = NetworkToolkitFileEditor.active;
      const textarea = document.querySelector('.nt-editor-code textarea[data-ed-text]');
      return editor.diskSession.state().page.start + textarea.selectionEnd === n;
    }, Number(size));
    await text.pressSequentially(tail, { delay: 12 });
    await page.waitForFunction(n => NetworkToolkitFileEditor.active.diskMeta.units === n && !NetworkToolkitFileEditor.active.diskSession.pending,
      Number(size) + tail.length);
    assert.ok(await text.inputValue().then(value => value.endsWith(tail)));
    await text.press('Control+Home');
    await page.waitForFunction(() => {
      const editor = NetworkToolkitFileEditor.active;
      const textarea = document.querySelector('.nt-editor-code textarea[data-ed-text]');
      return editor.diskSession.state().page.start === 0 && textarea.selectionStart === 0;
    });
    await text.pressSequentially(head, { delay: 12 });
    const editedSize = Number(size) + head.length + tail.length;
    await page.waitForFunction(n => NetworkToolkitFileEditor.active.diskMeta.units === n && !NetworkToolkitFileEditor.active.diskSession.pending, editedSize);
    assert.ok(await text.inputValue().then(value => value.startsWith(head)));

    report.phase = 'compare';
    const compareStarted = Date.now();
    await page.locator('[data-ed="compare"]').click();
    const summary = page.locator('.nt-editor-disk-compare [data-compare-summary]');
    await page.waitForFunction(() => document.querySelector('.nt-editor-disk-compare [data-compare-summary]')?.dataset.compareDone === 'true', null, { timeout: 180000 });
    assert.equal(await summary.getAttribute('data-compare-count'), '2');
    const map = page.locator('.nt-editor-disk-compare [data-compare-map]');
    const leftTrack = map.locator('[data-compare-track="left"]');
    const leftMark = leftTrack.locator('rect.changed');
    await leftMark.waitFor();
    assert.equal(await leftMark.getAttribute('height'), '1');
    const sourceLine = await leftTrack.getAttribute('aria-valuenow');
    await map.hover();
    await page.mouse.wheel(0, 100);
    await page.waitForFunction(() => document.querySelector('[data-compare-map]').scrollTop > 0);
    assert.equal(await leftTrack.getAttribute('aria-valuenow'), sourceLine, 'wheel pans only the change map');
    await leftMark.waitFor({state:'hidden'});
    await page.locator('.nt-compare-settings summary').click();
    await page.selectOption('[data-compare-scale]', '4');
    await page.locator('.nt-compare-settings summary').click();
    const requestedBottom = await map.evaluate(el => {
      el.scrollTop = el.scrollHeight;
      return {top:el.scrollTop, height:el.clientHeight, scrollHeight:el.scrollHeight};
    });
    await leftMark.waitFor().catch(async error => {
      report.map = {requestedBottom, settled:await map.evaluate(el => ({
        top:el.scrollTop, height:el.clientHeight, scrollHeight:el.scrollHeight,
        markerTop:el.querySelector('[data-compare-map-viewport]').style.top,
        marks:el.querySelector('[data-compare-map-marks]').innerHTML
      }))};
      throw error;
    });
    const mark = await leftMark.boundingBox();
    await page.mouse.click(mark.x + mark.width / 2, mark.y + mark.height / 2);
    await page.waitForFunction(marker => [...document.querySelectorAll('.nt-editor-disk-compare textarea')].some(el => el.value.includes(marker)), tail);
    await leftTrack.press('Home');
    await page.waitForFunction(() => document.querySelector('[data-compare-track="left"]').getAttribute('aria-valuenow') === '1');
    // The first scroll should preserve the partial row on both sides. Matching
    // only the integer line leaves the peer up to a full row out of alignment.
    await page.evaluate(() => {
      const source = document.querySelector('[data-compare-text="left"]');
      const height = parseFloat(getComputedStyle(source).lineHeight);
      source.dispatchEvent(new WheelEvent('wheel', {bubbles: true, deltaY: height * 8.5}));
      source.scrollTop = height * 8.5;
    });
    await page.waitForFunction(() => {
      const left = document.querySelector('[data-compare-text="left"]');
      const right = document.querySelector('[data-compare-text="right"]');
      return left.scrollTop > 0 && Math.abs(left.scrollTop - right.scrollTop) <= 1;
    }, null, {timeout: 3000});
    const next = page.locator('.nt-editor-disk-compare [data-compare="next"]');
    await next.click();
    await page.waitForFunction(() => document.querySelector('.nt-editor-disk-compare [data-compare-summary]')?.dataset.compareSelected === '0');
    await page.waitForFunction(marker => [...document.querySelectorAll('.nt-editor-disk-compare textarea')].some(el => el.value.startsWith(marker)), head);
    await next.click();
    await page.waitForFunction(() => document.querySelector('.nt-editor-disk-compare [data-compare-summary]')?.dataset.compareSelected === '1');
    await page.waitForFunction(marker => [...document.querySelectorAll('.nt-editor-disk-compare textarea')].some(el => el.value.includes(marker)), tail);
    await page.locator('.nt-editor-disk-compare .nt-compare-line.changed').first().waitFor();
    report.compareMs = Date.now() - compareStarted;
    await page.screenshot({ path: path.join(output, 'editor-large-file-comparison.png') });
    await page.evaluate(() => NetworkToolkitFileEditor.closeComparison());
    await page.evaluate(name => {
      const doc = [...NetworkToolkitFileEditor.documents.values()].find(item => item.path === name);
      NetworkToolkitFileEditor.activateDocument(doc.id);
    }, right);

    report.phase = 'save';
    await text.press('Control+End');
    await page.waitForFunction(n => {
      const editor = NetworkToolkitFileEditor.active;
      const textarea = document.querySelector('.nt-editor-code textarea[data-ed-text]');
      return editor.diskSession.state().page.start + textarea.selectionEnd === n;
    }, editedSize);
    const saveStarted = Date.now();
    await page.locator('[data-ed="save"]').click();
    await waitForEngine(page, state => state.snapshotWriting || state.saveReturned, 180000);
    await text.pressSequentially('!');
    await page.waitForFunction(n => NetworkToolkitFileEditor.active.diskMeta.units === n && !NetworkToolkitFileEditor.active.diskSession.pending, editedSize + 1);
    await page.evaluate(() => api('/editor-fixture/release-save', 'POST', {}));
    await page.waitForFunction(() => !NetworkToolkitFileEditor.active.saving, null, { timeout: 180000 });
    report.saveMs = Date.now() - saveStarted;
    assert.ok(await page.evaluate(() => NetworkToolkitFileEditor.active.dirty), 'new typing must remain unsaved');
    assert.ok(await text.inputValue().then(value => value.endsWith(tail + '!')));
    report.editsDuringSave = await page.evaluate(async () => (await api('/editor-fixture/state')).editsDuringSave);
    await text.press('Control+z');
    await page.waitForFunction(n => {
      const editor = NetworkToolkitFileEditor.active;
      return editor.diskMeta.units === n && !editor.dirty && !editor.diskSession.pending;
    }, editedSize);
    report.timing = await page.evaluate(() => {
      const { gaps, input, long } = window.editorTimings;
      const percentile = (values, fraction) => values.slice().sort((a, b) => a - b)[Math.max(0, Math.ceil(values.length * fraction) - 1)];
      const d = NetworkToolkitFileEditor.active;
      return { frameP99Ms: percentile(gaps, 0.99), maxFrameGapMs: Math.max(...gaps),
        inputP95Ms: percentile(input, 0.95), inputSamples: input.length, longTasksMs: long,
        pageUnits: d.diskSession.state().page.text.length, fullUnits: d.diskMeta.units };
    });
    assert.ok(report.timing.pageUnits < 128 * 1024, 'renderer received a bounded page');
    assert.ok(report.timing.inputSamples >= head.length + tail.length + 1, 'timed each edit event');
    assert.ok(report.timing.frameP99Ms < 100 && report.timing.inputP95Ms < 100, JSON.stringify(report.timing));
    assert.deepEqual(report.browserErrors, []);
    report.phase = 'close';
    assert.ok(await page.evaluate(() => NetworkToolkitFileEditor.closeAll()));
    await waitForEngine(page, state => state.grants === 0 && state.comparisons === 0);
    report.passed = true;
    report.phase = 'verified';
  } finally {
    await browser.close();
    fs.writeFileSync(path.join(output, 'editor-large-file-report.json'), JSON.stringify(report, null, 2));
    console.log(JSON.stringify(report));
  }
}

main().catch(error => { console.error(error); process.exitCode = 1; });
