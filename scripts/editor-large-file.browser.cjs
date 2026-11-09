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
  const browser = await chromium.launch({ headless: true, ignoreDefaultArgs:['--hide-scrollbars'],
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
    await page.locator('.nt-editor-disk-highlight:visible .nt-ed-keyword').first().waitFor();
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
    await page.locator('.nt-compare-settings summary').click();
    await page.check('[data-compare-only]');
    const preview = page.locator('.nt-compare-preview');
    await preview.locator('code').first().waitFor();
    assert.match(await preview.innerText(), new RegExp(head));
    assert.equal(await preview.locator('code').count(), 2);
    await preview.getByRole('button', {name:'Return to editable comparison'}).click();
    assert.equal(await preview.isVisible(), false);
    const legend = page.locator('.nt-editor-disk-compare .nt-compare-legend');
    assert.equal(await legend.isVisible(), true);
    assert.match(await legend.innerText(), /\+ Added.*− Removed.*~ Modified/s);
    await page.uncheck('[data-compare-show-map]');
    assert.equal(await page.locator('[data-compare-map]').isVisible(), false);
    await page.check('[data-compare-show-map]');
    await page.selectOption('[data-compare-spaces]', 'all');
    await page.waitForFunction(() => document.querySelector('[data-compare-summary]')?.dataset.compareDone === 'true');
    assert.equal(await summary.getAttribute('data-compare-count'), '2');
    assert.equal(await page.locator('[data-compare-scale]').inputValue(),'-3');
    await page.selectOption('[data-compare-scale]', '-2');
    await page.waitForFunction(() => document.querySelector('[data-compare-map-marks="left"] rect')?.getAttribute('height') === '1');
    await page.selectOption('[data-compare-scale]', '-3');
    await page.waitForFunction(() => {
      const map = document.querySelector('[data-compare-map]');
      return map.scrollHeight === map.clientHeight && map.querySelectorAll('[data-compare-map-marks="left"] rect').length === 2;
    });
    await page.selectOption('[data-compare-scale]', '-1');
    await page.locator('.nt-compare-settings summary').click();
    for (const side of ['left', 'right']) {
      const pane = page.locator('[data-compare-pane="' + side + '"]');
      assert.equal(await pane.locator('.nt-editor-disk-scroll:visible').count(), 1);
      assert.equal(await pane.locator('textarea').evaluate(el => el.offsetWidth - el.clientWidth), 0);
    }
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
    await map.locator('[data-compare-track="right"]').press('Home');
    await page.waitForFunction(() => ['left','right'].every(side =>
      document.querySelector('[data-compare-pane="' + side + '"] .nt-editor-disk-highlight:not([hidden]) .nt-ed-keyword')), null, {timeout:5000}).catch(async error => {
      report.syntax = await page.evaluate(() => ['left','right'].map(side => {
        const d = NetworkToolkitFileEditor.documents.get(NetworkToolkitFileEditor.comparison.selection[side]);
        const pane = document.querySelector('[data-compare-pane="' + side + '"]');
        return {side, language:d.language, detected:d.diskSyntaxLanguage, page:d.diskSession.state().page.line,
          hidden:pane.querySelector('.nt-editor-disk-highlight').hidden, keywords:pane.querySelectorAll('.nt-ed-keyword').length};
      }));
      throw error;
    });
    const leftText = page.locator('[data-compare-text="left"]');
    await leftText.click({position:{x:30,y:30}});
    await page.waitForFunction(() => ['left','right'].every(side =>
      document.querySelector('[data-compare-pane="' + side + '"] .nt-disk-compare-color.is-current')));
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
    assert.equal(await page.locator('.nt-editor-disk-compare .nt-compare-line.changed').first().getAttribute('data-difference-symbol'), '~');
    const changedCharacters = page.locator('.nt-disk-compare-intraline > span');
    await changedCharacters.first().waitFor();
    await leftText.dispatchEvent('contextmenu',{clientX:440,clientY:330,button:2});
    await page.locator('#editorContextMenu [data-editor-menu-action="compareCharacters"]').click();
    assert.equal(await changedCharacters.count(), 0);
    await leftText.dispatchEvent('contextmenu',{clientX:440,clientY:330,button:2});
    await page.locator('#editorContextMenu [data-editor-menu-action="compareOptions"]').click();
    await page.waitForFunction(() => document.querySelector('.nt-compare-settings').open);
    await page.check('[data-compare-intraline]');
    await page.locator('[data-compare="first"]').click();
    await page.waitForFunction(() => document.querySelector('[data-compare-summary]').dataset.compareSelected === '0');
    await page.locator('[data-compare="last"]').click();
    await page.waitForFunction(() => document.querySelector('[data-compare-summary]').dataset.compareSelected === '1');
    await page.locator('.nt-compare-settings summary').click();
    report.compareMs = Date.now() - compareStarted;
    await page.screenshot({ path: path.join(output, 'editor-large-file-comparison.png') });
    await page.evaluate(() => NetworkToolkitFileEditor.closeComparison());
    await page.evaluate(name => {
      const doc = [...NetworkToolkitFileEditor.documents.values()].find(item => item.path === name);
      NetworkToolkitFileEditor.activateDocument(doc.id);
    }, right);

    report.phase = 'alignment';
    const added = '    set alias synthetic-added\n    set status enable\n';
    const savedSize = editedSize + added.length;
    await text.press('Control+Home');
    await page.waitForFunction(() => NetworkToolkitFileEditor.active.diskSession.state().page.start === 0);
    await text.evaluate((el, {at,added}) => {
      el.setSelectionRange(at, at);
      const data = new DataTransfer();
      data.setData('text/plain', added);
      el.dispatchEvent(new ClipboardEvent('paste', {clipboardData:data,bubbles:true,cancelable:true}));
    }, {at:head.length+1024,added});
    await page.waitForFunction(n => NetworkToolkitFileEditor.active.diskMeta.units === n && !NetworkToolkitFileEditor.active.diskSession.pending, savedSize);
    await page.locator('[data-ed="compare"]').click();
    for (const [side,path] of [['left',left],['right',right]]) {
      const id = await page.evaluate(path => [...NetworkToolkitFileEditor.documents.values()].find(d => d.path === path).id, path);
      await page.selectOption('[data-compare-select="'+side+'"]', id);
    }
    await page.waitForFunction(() => document.querySelector('[data-compare-summary]')?.dataset.compareDone === 'true', null, {timeout:180000});
    assert.equal(await page.locator('[data-compare-scale]').inputValue(),'4');
    const leftPane = page.locator('[data-compare-pane="left"]');
    await leftPane.locator('.nt-editor-disk-scroll').press('Home');
    await leftPane.locator('[data-alignment-gap]').first().waitFor({timeout:5000}).catch(async error => {
      report.alignment = await page.evaluate(() => {
        const c = NetworkToolkitFileEditor.comparison;
        return {summary:document.querySelector('[data-compare-summary]')?.textContent, rows:c.result?.rows, sides:['left','right'].map(side => {
          const d = NetworkToolkitFileEditor.documents.get(c.selection[side]);
          return {size:d.diskMeta.units,view:c.diskView(side).capture(),
            gaps:document.querySelectorAll('[data-compare-pane="'+side+'"] [data-alignment-gap]').length};
        })};
      });
      throw error;
    });
    const aligned = () => {
      const row = (side, number) => [...document.querySelectorAll('[data-compare-pane="'+side+'"] .nt-editor-gutter [data-disk-compare-gutter] > div')]
        .find(el => el.textContent === number)?.getBoundingClientRect().top;
      return Math.abs(row('left','2') - row('right','4')) < 1;
    };
    await page.waitForFunction(aligned);
    assert.equal(await leftPane.locator('[data-alignment-gap]').count(), 2);
    await leftPane.locator('textarea').hover();
    await page.mouse.wheel(0, 28);
    await page.waitForFunction(() => {
      const c = NetworkToolkitFileEditor.comparison;
      const left = c.diskView('left').capture(), right = c.diskView('right').capture();
      return left.alignedTop > 0 && left.alignedTop === right.alignedTop && Math.abs(left.fraction-right.fraction) < .05;
    });
    await leftPane.locator('.nt-editor-disk-scroll').press('Home');
    await page.waitForFunction(aligned);
    assert.equal(await leftPane.locator('textarea').inputValue().then(value => value.includes('\n\n\nconfig system global')), true);
    assert.equal(await leftPane.locator('textarea').evaluate(el => getComputedStyle(el.parentElement.querySelector('.nt-disk-compare-color.gap')).backgroundColor !== 'rgba(0, 0, 0, 0)'), true);
    await page.screenshot({path:path.join(output,'editor-aligned-spacers.png')});
    await leftPane.locator('textarea').evaluate(el=>{
      const at = el.value.indexOf('config system global');
      el.focus({preventScroll:true});
      el.setSelectionRange(at,at);
    });
    await leftPane.locator('textarea').pressSequentially('!');
    await page.waitForFunction(({name,size}) => [...NetworkToolkitFileEditor.documents.values()].find(d=>d.path===name).diskMeta.units === size, {name:left,size:Number(size)+1});
    assert.equal(await page.evaluate(name => [...NetworkToolkitFileEditor.documents.values()].find(d=>d.path===name).diskSession.state().page.text.includes('\n!config system global'),left),true);
    await leftPane.locator('textarea').press('Control+z');
    await page.waitForFunction(({name,size}) => [...NetworkToolkitFileEditor.documents.values()].find(d=>d.path===name).diskMeta.units === size, {name:left,size:Number(size)});
    await page.evaluate(name => {
      NetworkToolkitFileEditor.closeComparison();
      const doc = [...NetworkToolkitFileEditor.documents.values()].find(item => item.path === name);
      NetworkToolkitFileEditor.activateDocument(doc.id);
    }, right);

    report.phase = 'save';
    await text.press('Control+End');
    await page.waitForFunction(n => {
      const editor = NetworkToolkitFileEditor.active;
      const textarea = document.querySelector('.nt-editor-code textarea[data-ed-text]');
      return editor.diskSession.state().page.start + textarea.selectionEnd === n;
    }, savedSize);
    const saveStarted = Date.now();
    await page.locator('[data-ed="save"]').click();
    await waitForEngine(page, state => state.snapshotWriting || state.saveReturned, 180000);
    await text.pressSequentially('!');
    await page.waitForFunction(n => NetworkToolkitFileEditor.active.diskMeta.units === n && !NetworkToolkitFileEditor.active.diskSession.pending, savedSize + 1);
    await page.evaluate(() => api('/editor-fixture/release-save', 'POST', {}));
    await page.waitForFunction(() => !NetworkToolkitFileEditor.active.saving, null, { timeout: 180000 });
    report.saveMs = Date.now() - saveStarted;
    assert.ok(await page.evaluate(() => NetworkToolkitFileEditor.active.dirty), 'new typing must remain unsaved');
    assert.ok(await text.inputValue().then(value => value.endsWith(tail + '!')));
    report.editsDuringSave = await page.evaluate(async () => (await api('/editor-fixture/state')).editsDuringSave);
    const documentBar = page.locator('.nt-editor-code .nt-editor-disk-scroll');
    await documentBar.evaluate(el => el.scrollTop = (el.scrollHeight - el.clientHeight) / 2);
    await page.waitForFunction(() => {
      const bar = document.querySelector('.nt-editor-code .nt-editor-disk-scroll');
      return Number(bar.getAttribute('aria-valuenow')) > Number(bar.getAttribute('aria-valuemax')) / 3 &&
        Number(bar.getAttribute('aria-valuenow')) < Number(bar.getAttribute('aria-valuemax')) * 2 / 3;
    });
    const beforeUndo = await documentBar.getAttribute('aria-valuenow');
    await page.keyboard.press('Control+z');
    await page.waitForFunction(n => {
      const editor = NetworkToolkitFileEditor.active;
      return editor.diskMeta.units === n && !editor.dirty && !editor.diskSession.pending;
    }, savedSize);
    await page.waitForFunction(top => document.querySelector('.nt-editor-code .nt-editor-disk-scroll').getAttribute('aria-valuenow') === top, beforeUndo, {timeout:3000}).catch(async error => {
      report.undoViewport = {before:beforeUndo, after:await documentBar.getAttribute('aria-valuenow')};
      throw error;
    });
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
