'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

const { NTK_EDITOR_FIXTURE_URL: url, NTK_EDITOR_FIXTURE_LEFT: left,
  NTK_EDITOR_FIXTURE_RIGHT: right, NTK_EDITOR_FIXTURE_HEAD: head, NTK_EDITOR_FIXTURE_TAIL: tail,
  NTK_EDITOR_FIXTURE_SIZE: size, NTK_EDITOR_FIXTURE_TOKEN: token,
  NTK_EDITOR_FIXTURE_SHELL: shellFile, NTK_EDITOR_FIXTURE_SHELL_TEXT: shellText } = process.env;
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

async function verifyRepeatedPolicies(page) {
  const policy = (id, {name = 'Synthetic policy ' + id, destination = 'port2', action = 'accept'} = {}) => [
    '    edit ' + id,
    '        set name "' + name + '"',
    '        set uuid 00000000-0000-4000-8000-' + String(id).padStart(12, '0'),
    '        set srcintf "port1"',
    '        set dstintf "' + destination + '"',
    '        set action ' + action,
    '    next',
  ];
  const ids = Array.from({length:12}, (_, i) => 10000 + i);
  const prefix = ['config firewall address', ...Array.from({length:3400}, (_, i) => [
    '    edit "Synthetic address ' + i + '"',
    '        set type ipmask',
    '        set subnet 192.0.2.0 255.255.255.0',
    '        set comment "Synthetic fixture"',
    '    next',
  ]).flat(), 'end'];
  const config = lines => [...prefix, 'config firewall policy', ...lines, 'end', ''].join('\n');
  const before = config(ids.flatMap(id => policy(id)));
  const after = config([
    ...[20000, 20001, 20002].flatMap(id => policy(id)),
    ...policy(10000, {name:'Renamed original policy', destination:'port3'}),
    ...policy(20003),
    ...ids.slice(1, 4).flatMap(id => policy(id)),
    ...policy(20004),
    ...policy(20005),
    ...policy(10005, {name:'Renamed policy after replacements'}),
    ...ids.slice(6).flatMap(id => policy(id, {action:id === ids.at(-1) ? 'deny' : 'accept'})),
  ]);
  // Use the same disk-backed route as the real files without adding another
  // large on-disk fixture just to demonstrate the repeated-policy anchor.
  await page.evaluate(async ({before, after}) => {
    for (const [id, content] of [['policies-before', before], ['policies-after', after]]) {
      const file = new File([content], id + '.conf');
      await NetworkToolkitFileEditor.open({
        id, path:file.name, source:'Synthetic FortiOS fixture', binding:{kind:'localDraft'},
        create:true, diskMode:'upload', diskFile:file, read:async()=>({content}), write:async()=>({}),
      });
    }
  }, {before, after});
  await page.locator('[data-ed="compare"]').click();
  await page.selectOption('[data-compare-select="left"]', 'policies-before');
  await page.selectOption('[data-compare-select="right"]', 'policies-after');
  await page.waitForFunction(() => document.querySelector('[data-compare-summary]')?.dataset.compareDone === 'true');
  await page.locator('.nt-compare-settings summary').click();
  await page.check('[data-compare-only]');
  const preview = page.locator('.nt-compare-preview');
  for (const id of [20000, 20001, 20002]) {
    await preview.locator('.nt-compare-preview-line.added code').filter({hasText:'edit ' + id}).waitFor();
  }
  assert.equal(await preview.locator('code').filter({hasText:'edit 10000'}).count(), 0);
  assert.equal(await preview.locator('code').filter({hasText:'edit 10001'}).count(), 0);
  await preview.getByRole('button', {name:'Return to editable comparison'}).click();
  const policyLines = policy(10000).length;
  const firstPolicyLine = prefix.length + 2;
  for (const [id, leftLine, rightLine, gaps] of [
    [10000, firstPolicyLine, firstPolicyLine + 3 * policyLines, 3 * policyLines],
    [10001, firstPolicyLine + policyLines, firstPolicyLine + 5 * policyLines, policyLines],
    [10005, firstPolicyLine + 5 * policyLines, firstPolicyLine + 10 * policyLines, policyLines],
  ]) {
    const alignedRow = rightLine - 1;
    await page.evaluate(async row => {
      const compare = NetworkToolkitFileEditor.comparison;
      await Promise.all(['left', 'right'].map(side => compare.diskView(side).goAlignedRow(row)));
    }, Math.max(0, alignedRow - gaps));
    await page.waitForFunction(({leftLine, rightLine, gaps}) => {
      const find = (side, number) => [...document.querySelector(`[data-disk-compare-gutter="${side}"]`).children]
        .find(row => row.dataset.sourceLine === String(number));
      const left = find('left', leftLine), right = find('right', rightLine);
      if (!left || !right || Math.abs(left.getBoundingClientRect().top - right.getBoundingClientRect().top) >= 2) return false;
      if (left.dataset.differenceSymbol || right.dataset.differenceSymbol) return false;
      let previous = left.previousElementSibling;
      for (let i = 0; i < gaps; i++, previous = previous?.previousElementSibling) {
        if (!previous?.hasAttribute('data-alignment-gap')) return false;
      }
      return true;
    }, {leftLine, rightLine, gaps}).catch(error => { throw Error(`Policy ${id} did not align below ${gaps} spacer rows`, {cause:error}); });
  }
}

async function verifyMovedLogBlock(page) {
  const repeated = 'INFO Probe sent';
  const moved = ['INFO Starting replay', repeated, repeated, 'INFO Replay complete'];
  const stable = Array.from({length:8}, (_, i) => [
    'INFO Checking channel ' + i, repeated, 'INFO Channel check complete',
  ]).flat();
  const longRecord = 'WARN Ignored noisy probes ' + Array.from({length:20}, (_, i) => 'synthetic-probe-' + i).join(', ');
  const shortRecord = 'WARN Ignored noisy probe';
  const wrappedRecord = value => 'INFO Replay samples ' + 'synthetic probe '.repeat(275) + ' route=' + value + ' recorded';
  const before = ['INFO Run started', ...moved, ...stable, 'INFO Applying results', longRecord,
    'INFO Results ready', 'INFO Status before', wrappedRecord('Internal'), 'INFO Run finished'].join('\n');
  const after = ['INFO Run started', ...stable, ...moved, 'INFO Applying results', shortRecord,
    'INFO Results ready', 'INFO Status after', wrappedRecord('port2'), 'INFO Run finished'].join('\n');
  await page.evaluate(async ({before, after}) => {
    for (const [id, content] of [['moves-before', before], ['moves-after', after]]) {
      const file = new File([content], id + '.log');
      await NetworkToolkitFileEditor.open({
        id, path:file.name, source:'Synthetic log fixture', binding:{kind:'localDraft'},
        create:true, diskMode:'upload', diskFile:file, read:async()=>({content}), write:async()=>({}),
      });
    }
  }, {before, after});
  await page.locator('[data-ed="compare"]').click();
  await page.selectOption('[data-compare-select="left"]', 'moves-before');
  await page.selectOption('[data-compare-select="right"]', 'moves-after');
  await page.waitForFunction(() => document.querySelector('[data-compare-summary]')?.dataset.compareDone === 'true');
  await page.locator('.nt-compare-settings summary').click();
  await page.check('[data-compare-only]');
  const preview = page.locator('.nt-compare-preview');
  await preview.locator('.nt-compare-preview-line.moved').first().waitFor();
  const rows = await preview.locator('.nt-compare-preview-line.moved').allTextContents();
  assert.equal(rows.length, 8, 'both ends of the entire moved block are marked');
  assert.equal(rows.filter(row => row.includes(repeated)).length, 4);
  assert.equal(await preview.locator('.nt-compare-preview-line.gap').count(), 8);
  await preview.locator('.nt-compare-preview-line.removed code').filter({hasText:longRecord}).waitFor();
  await preview.locator('.nt-compare-preview-line.added code').filter({hasText:shortRecord}).waitFor();
  await preview.locator('.nt-compare-preview-line.changed code').filter({hasText:'INFO Status before'}).waitFor();
  assert.equal(await preview.locator('code').filter({hasText:'INFO Results ready'}).count(), 0);
  await preview.getByRole('button', {name:'Return to editable comparison'}).click();
  const source = page.locator('[data-compare-pane="left"] [data-difference-symbol="↔"]');
  await source.first().waitFor();
  assert.equal(await source.first().textContent(), '2');
  await page.waitForFunction(moveLength => {
    const rows = side => [...document.querySelector(`[data-disk-compare-gutter="${side}"]`).children];
    const rightRows = rows('right');
    const left = rows('left').find(row => row.dataset.sourceLine === String(moveLength + 3));
    const right = rightRows.find(row => row.dataset.sourceLine === '3');
    const firstStable = rightRows.findIndex(row => row.dataset.sourceLine === '2');
    if (!left || !right || firstStable <= moveLength) return false;
    const gaps = rightRows.slice(firstStable - moveLength, firstStable);
    const bounds = right.getBoundingClientRect();
    return Math.abs(left.getBoundingClientRect().top - bounds.top) < bounds.height / 2 &&
      !left.dataset.differenceSymbol && !right.dataset.differenceSymbol &&
      gaps.every(gap => gap.hasAttribute('data-alignment-gap') && gap.textContent === '') &&
      gaps.every(gap => getComputedStyle(gap).backgroundColor !== 'rgba(0, 0, 0, 0)');
  }, moved.length);
  const replacementLine = stable.length + moved.length + 3;
  await page.evaluate(async line => {
    const compare = NetworkToolkitFileEditor.comparison;
    await Promise.all(['left', 'right'].map(side => compare.diskView(side).goLine(line - 1, false)));
  }, replacementLine);
  await page.waitForFunction(line => {
    const row = side => document.querySelector('[data-disk-compare-gutter="' + side + '"] [data-source-line="' + line + '"]');
    return row('left')?.dataset.differenceSymbol === '−' && row('right')?.dataset.differenceSymbol === '+';
  }, replacementLine);
  const wrap = page.locator('[data-ed-wrap]');
  await wrap.check();
  await page.evaluate(async line => {
    const compare = NetworkToolkitFileEditor.comparison;
    await Promise.all(['left', 'right'].map(side => compare.diskView(side).goLine(line - 1, false)));
  }, replacementLine + 3);
  for (const [side, value] of [['left','Internal'], ['right','port2']]) {
    const highlight = page.locator(`[data-compare-pane="${side}"] .nt-disk-compare-intraline > span`).filter({hasText:value});
    await highlight.waitFor();
    assert.equal(await highlight.textContent(), value);
    assert.equal(await highlight.locator('..').textContent(), wrappedRecord(value));
  }
}

// A synthetic Base64 export of 1.7 MB on one line, opened disk-backed: with
// word wrap the document bar reaches its last characters, and the status bar
// counts the selected characters of disk-backed and page-held documents.
async function verifyLongLineStatus(page) {
  const units = 1748322;
  await page.evaluate(async units => {
    const letters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
    let seed = 0x6d2b79f5;
    const random = () => { seed ^= seed << 13; seed ^= seed >>> 17; seed ^= seed << 5; return (seed >>> 0) / 4294967296; };
    const parts = new Array(units - 2);
    for (let i = 0; i < parts.length; i++) parts[i] = letters[(random() * 64) | 0];
    const content = parts.join('') + '==';
    const file = new File([content], 'synthetic-export.txt');
    await NetworkToolkitFileEditor.open({
      id:'long-line', path:file.name, source:'Synthetic export', binding:{kind:'localDraft'},
      create:true, diskMode:'upload', diskFile:file, read:async()=>({content}), write:async()=>({}),
    });
  }, units);
  await page.waitForFunction(n => NetworkToolkitFileEditor.active?.diskMeta?.units === n &&
    !document.querySelector('.nt-editor-code textarea[data-ed-text]').disabled, units);
  const text = page.locator('.nt-editor-code textarea[data-ed-text]').first();
  const position = page.locator('[data-ed-position]');
  const wrap = page.locator('[data-ed-wrap]');
  await wrap.check();
  const bar = page.locator('.nt-editor-code .nt-editor-disk-scroll');
  await bar.waitFor();
  await bar.press('End');
  await page.waitForFunction(n => {
    const ta = document.querySelector('.nt-editor-code textarea[data-ed-text]');
    const view = NetworkToolkitFileEditor.active.diskSession.state().page;
    return view.start + view.text.length === n && ta.scrollTop + ta.clientHeight >= ta.scrollHeight - 2 &&
      ta.value.endsWith('==') && document.querySelector('.nt-editor-code .nt-editor-disk-scroll').getAttribute('aria-valuenow') === '100';
  }, units);
  await text.press('Control+a');
  await page.waitForFunction(() => document.querySelector('[data-ed-position]').textContent.endsWith(' selected'));
  assert.equal(await position.textContent(), 'Ln 1, Col 1,748,323 · 1,748,322 selected');
  await text.click({position:{x:40, y:40}});
  await page.waitForFunction(() => !document.querySelector('[data-ed-position]').textContent.includes('selected'));
  await text.press('Control+Shift+Home');
  await page.waitForFunction(() => {
    const match = / · ([\d,]+) selected$/.exec(document.querySelector('[data-ed-position]').textContent);
    return match && Number(match[1].replace(/,/g, '')) > 49152;
  });
  await wrap.uncheck();
  await page.evaluate(async () => {
    await NetworkToolkitFileEditor.open({
      id:'held', path:'held.txt', source:'Synthetic', binding:{kind:'localDraft'}, create:true,
      initialContent:'alpha\nbravo\ncharlie\n', read:async()=>({content:''}), write:async()=>({}),
    });
  });
  await page.waitForFunction(() => NetworkToolkitFileEditor.active?.path === 'held.txt' && !NetworkToolkitFileEditor.active.disk);
  await text.evaluate(el => { el.focus(); el.setSelectionRange(6, 17); });
  await page.waitForFunction(() => document.querySelector('[data-ed-position]').textContent === 'Ln 2, Col 1 · 11 selected');
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
    if(process.env.NTK_EDITOR_STANDALONE_FIXTURE==='1'){
      assert.equal(await page.locator('#sidebar').isVisible(),false);
      await page.evaluate(()=>navigate('terminal'));
      assert.equal(await page.evaluate(()=>state.activePage),'editor');
    }
    const text = page.locator('.nt-editor-code textarea[data-ed-text]').first();
    await text.waitFor();
    await page.waitForFunction(({shellFile, shellText}) => {
      const active = NetworkToolkitFileEditor.active;
      return active?.path === shellFile && active.disk &&
        document.querySelector('.nt-editor-code textarea[data-ed-text]')?.value.startsWith(shellText);
    }, {shellFile, shellText});
    assert.ok(await page.evaluate(() => NetworkToolkitFileEditor.closeAll()));
    await waitForEngine(page, state => state.grants === 0);
    await page.evaluate(() => navigate('editor'));
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

    report.phase = 'single-word-wrap';
    const wrap = page.locator('[data-ed-wrap]');
    const documentBar = page.locator('.nt-editor-code .nt-editor-disk-scroll');
    await wrap.check();
    await documentBar.press('Home');
    await page.waitForFunction(() => {
      const editor = document.querySelector('.nt-editor-code textarea[data-ed-text]');
      const rows = [...document.querySelectorAll('[data-ed-gutter-lines] > div')];
      const first = rows.filter(row => row.textContent === '1');
      const second = rows.find(row => row.textContent === '2');
      const style = getComputedStyle(editor);
      return ['pre-wrap', 'break-spaces'].includes(style.whiteSpace) && first.length === 1 &&
        second && second.getBoundingClientRect().top - first[0].getBoundingClientRect().top > 2 * parseFloat(style.lineHeight);
    });
    assert.equal(await text.evaluate(editor => editor.scrollWidth <= editor.clientWidth + 1), true);
    await documentBar.press('End');
    await page.waitForFunction(() => {
      const bar = document.querySelector('.nt-editor-code .nt-editor-disk-scroll');
      return Number(bar.getAttribute('aria-valuenow')) >= .99 * Number(bar.getAttribute('aria-valuemax'));
    });

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
    await wrap.uncheck();

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
    assert.match(await preview.innerText(), new RegExp(tail));
    assert.equal(await preview.locator('code').count(), 4);
    await preview.getByRole('button', {name:'Return to editable comparison'}).click();
    assert.equal(await preview.isVisible(), false);
    const legend = page.locator('.nt-editor-disk-compare .nt-compare-legend');
    assert.equal(await legend.isVisible(), true);
    assert.match(await legend.innerText(), /\+ Added.*− Removed.*~ Modified/s);
    await page.locator('.nt-compare-settings summary').click();
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
    const editorMenu = page.locator('#editorContextMenu');
    const compareMenu = editorMenu.locator('[data-editor-submenu="compare"]');
    const comparePanel = editorMenu.locator('[data-editor-submenu-panel="compare"]');
    await leftText.dispatchEvent('contextmenu',{clientX:440,clientY:330,button:2});
    await comparePanel.waitFor({state:'hidden'});
    await compareMenu.click();
    const characterToggle = comparePanel.locator('[data-editor-menu-action="compareCharacters"]');
    assert.equal(await characterToggle.getAttribute('role'), 'menuitemcheckbox');
    assert.equal(await characterToggle.getAttribute('aria-checked'), 'true');
    await characterToggle.click();
    assert.equal(await changedCharacters.count(), 0);
    await leftText.dispatchEvent('contextmenu',{clientX:440,clientY:330,button:2});
    await compareMenu.hover();
    assert.equal(await characterToggle.getAttribute('aria-checked'), 'false');
    await comparePanel.locator('[data-editor-menu-action="compareOptions"]').click();
    await page.waitForFunction(() => document.querySelector('.nt-compare-settings').open);
    await page.check('[data-compare-intraline]');
    await page.locator('[data-compare="first"]').click();
    await page.waitForFunction(() => document.querySelector('[data-compare-summary]').dataset.compareSelected === '0');
    await page.locator('[data-compare="last"]').click();
    await page.waitForFunction(() => document.querySelector('[data-compare-summary]').dataset.compareSelected === '1');
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
    const initialFontSize = Number(await page.locator('[data-ed-font]').inputValue());
    const zoomDiskCompare = async (side, delta, size) => {
      await page.locator(`[data-compare-text="${side}"]`).hover();
      await page.keyboard.down('Control');
      try { await page.mouse.wheel(0, delta); }
      finally { await page.keyboard.up('Control'); }
      await page.waitForFunction(expected => [...document.querySelectorAll('[data-compare-text]')]
        .every(editor => parseFloat(getComputedStyle(editor).fontSize) === expected), size);
      assert.equal(await page.locator('[data-ed-font]').inputValue(), String(size));
      await page.waitForFunction(aligned);
    };
    await zoomDiskCompare('left', -120, initialFontSize + 1);
    await zoomDiskCompare('right', 120, initialFontSize);
    await page.screenshot({path:path.join(output,'editor-aligned-spacers.png')});

    report.phase = 'comparison-word-wrap';
    await page.setViewportSize({width:1920,height:1100});
    const divider = page.locator('[data-compare-divider]');
    await wrap.check();
    for (let i = 0; i < 4; i++) await divider.press('ArrowRight');
    await leftPane.locator('.nt-editor-disk-scroll').press('Home');
    await page.waitForFunction(() => {
      const left = document.querySelector('[data-compare-pane="left"]');
      const right = document.querySelector('[data-compare-pane="right"]');
      const editor = pane => pane.querySelector('textarea');
      const rows = pane => [...pane.querySelector('[data-disk-compare-gutter]').children];
      const number = (pane, n) => rows(pane).find(row => row.textContent === n);
      const y = el => el?.getBoundingClientRect().top;
      const style = getComputedStyle(editor(left));
      const height = parseFloat(style.lineHeight);
      const leftHead = number(left, '1'), rightHead = number(right, '1');
      return ['pre-wrap', 'break-spaces'].includes(style.whiteSpace) &&
        ['pre-wrap', 'break-spaces'].includes(getComputedStyle(editor(right)).whiteSpace) &&
        editor(left).clientWidth > editor(right).clientWidth + 100 &&
        Math.abs(y(leftHead) - y(rightHead)) < height / 2 &&
        y(number(left, '2')) - y(leftHead) > 2 * height &&
        Math.abs(y(number(left, '2')) - y(number(right, '4'))) < height / 2 &&
        left.querySelector('[data-wrap-gap]') && leftHead.dataset.differenceSymbol === '~' &&
        rows(left).filter(row => row.textContent === '1').length === 1;
    }, null, {timeout:10000}).catch(async error => {
      report.wrappedComparison = await page.evaluate(() => ['left','right'].map(side => {
        const pane = document.querySelector('[data-compare-pane="'+side+'"]');
        const textarea = pane.querySelector('textarea');
        return {
          side, width:textarea.clientWidth, wrap:getComputedStyle(textarea).whiteSpace, scrollTop:textarea.scrollTop,
          rows:[...pane.querySelector('[data-disk-compare-gutter]').children].slice(0, 20).map(row => ({
            text:row.textContent, y:row.getBoundingClientRect().top, height:row.getBoundingClientRect().height,
            gap:row.hasAttribute('data-wrap-gap'), diff:row.dataset.differenceSymbol
          }))
        };
      }));
      await page.screenshot({path:path.join(output,'editor-wrapped-comparison-failed.png')});
      throw error;
    });
    await page.screenshot({path:path.join(output,'editor-wrapped-comparison.png')});
    await leftPane.locator('textarea').hover();
    await page.mouse.wheel(0, 84);
    await page.waitForFunction(() => {
      const left = document.querySelector('[data-compare-pane="left"]');
      const right = document.querySelector('[data-compare-pane="right"]');
      const y = (pane, number) => [...pane.querySelector('[data-disk-compare-gutter]').children]
        .find(row => row.textContent === number)?.getBoundingClientRect().top;
      return left.querySelector('textarea').scrollTop > 30 && right.querySelector('textarea').scrollTop > 30 &&
        Math.abs(y(left, '2') - y(right, '4')) < 3;
    });
    // A slow source-line lookup may finish after both panes have moved into
    // a spacer. Its result must never replace the current aligned-row anchor.
    const compareRoute = '**/buffers/compare';
    let releaseLocate, locateStarted;
    const locateDelay = new Promise(resolve => { releaseLocate = resolve; });
    const locateRequested = new Promise(resolve => { locateStarted = resolve; });
    const isLeftLocate = request => {
      const body = request.postDataJSON();
      return body?.action === 'locate' && body.side === 'left';
    };
    const sync = page.locator('[data-compare-sync]');
    await page.locator('.nt-compare-settings summary').click();
    await sync.uncheck();
    await page.evaluate(async () => {
      const compare = NetworkToolkitFileEditor.comparison;
      await Promise.all(['left', 'right'].map(side => compare.diskView(side).goAlignedRow(1)));
      document.querySelector('[data-compare-text="left"]').focus({preventScroll:true});
    });
    const delayLocate = async route => {
      if (isLeftLocate(route.request())) {
        locateStarted();
        await locateDelay;
      }
      await route.continue();
    };
    await page.route(compareRoute, delayLocate);
    try {
      const response = page.waitForResponse(value => value.url().endsWith('/buffers/compare') && isLeftLocate(value.request()));
      await sync.check();
      await Promise.race([locateRequested, response.then(() => { throw Error('The source lookup was not held.'); })]);
      await page.evaluate(() => {
        const source = document.querySelector('[data-compare-text="left"]');
        source.dispatchEvent(new WheelEvent('wheel', {bubbles:true, deltaY:7}));
        source.scrollTop += 7;
      });
      const insideSpacer = () => {
        const compare = NetworkToolkitFileEditor.comparison;
        const left = compare.diskView('left').capture(), right = compare.diskView('right').capture();
        return left.alignedTop === 1 && right.alignedTop === 1 && left.fraction > .15 &&
          Math.abs(left.fraction - right.fraction) < .05;
      };
      await page.waitForFunction(insideSpacer);
      releaseLocate();
      await response;
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      assert.equal(await page.evaluate(insideSpacer), true, 'late source lookup moved an aligned pane out of the spacer');
    } finally {
      releaseLocate();
      await page.unroute(compareRoute, delayLocate);
    }
    await page.locator('.nt-compare-settings summary').click();
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
    await wrap.uncheck();
    await divider.press('Enter');
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
    report.phase = 'close';
    assert.ok(await page.evaluate(() => NetworkToolkitFileEditor.closeAll()));
    await waitForEngine(page, state => state.grants === 0 && state.comparisons === 0);
    report.phase = 'fortios-policy-alignment';
    await verifyRepeatedPolicies(page);
    assert.ok(await page.evaluate(() => NetworkToolkitFileEditor.closeAll()));
    await waitForEngine(page, state => state.grants === 0 && state.comparisons === 0);
    report.phase = 'generic-log-comparison';
    await verifyMovedLogBlock(page);
    assert.ok(await page.evaluate(() => NetworkToolkitFileEditor.closeAll()));
    await waitForEngine(page, state => state.grants === 0 && state.comparisons === 0);
    report.phase = 'long-line-status';
    await verifyLongLineStatus(page);
    assert.ok(await page.evaluate(() => NetworkToolkitFileEditor.closeAll()));
    await waitForEngine(page, state => state.grants === 0 && state.comparisons === 0);
    assert.deepEqual(report.browserErrors, []);
    report.passed = true;
    report.phase = 'verified';
  } finally {
    await browser.close();
    fs.writeFileSync(path.join(output, 'editor-large-file-report.json'), JSON.stringify(report, null, 2));
    console.log(JSON.stringify(report));
  }
}

main().catch(error => { console.error(error); process.exitCode = 1; });
