'use strict';
// Real Chromium integration: keep the browser's native timers for every scenario.
// The older startup suite intentionally controls time; it cannot detect a native
// Window receiver error. This suite must not replace timers or accelerate time.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { performance } = require('node:perf_hooks');
let playwright;
try { playwright = require(process.env.PLAYWRIGHT_MODULE || 'playwright'); }
catch { playwright = require('/opt/codex/cua_node/lib/node_modules/playwright'); }

const root = path.resolve(__dirname, '..');
const output = process.env.TOOLKIT_TEST_OUTPUT || '/tmp/ntk-updater-native-browser';
const origin = 'http://toolkit.local';
const productionUpdater = fs.readFileSync(path.join(root, 'web/assets/app-updater.js'), 'utf8');
const nativeWiring = 'createScheduler({setTimeout:(...args)=>root.setTimeout(...args),clearTimeout:(...args)=>root.clearTimeout(...args),';
const released64Wiring = 'createScheduler({setTimeout,clearTimeout,';
// Restore only the released 1.17.64 integration error. Do not modify the scheduler,
// intercept the exception, or substitute a synthetic failing implementation.
assert.equal(productionUpdater.split(nativeWiring).length, 2, 'Expected exactly one fixed production timer integration');
const negativeUpdater = productionUpdater.replace(nativeWiring, released64Wiring);
const available = {
  currentVersion: '1.17.64', supported: true, hasUpdate: true, busy: [], installing: false,
  latest: { version: '1.17.65', notes: "What's new\n- Native automatic startup checks.\n- Your settings are preserved.\n- Downloads remain your choice.\n<script>window.notesExecuted=true</script>" }
};
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const pause = () => {
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  return { promise, resolve };
};
async function waitFor(predicate, message, timeout = 10000) {
  const deadline = performance.now() + timeout;
  while (!predicate()) {
    assert.ok(performance.now() < deadline, message);
    await sleep(25);
  }
}
const reports = [];
fs.mkdirSync(output, { recursive: true });

async function main() {
  const browser = await playwright.chromium.launch({
    headless: true,
    ...(process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH }
      : fs.existsSync('/usr/bin/chromium') ? { executablePath: '/usr/bin/chromium' } : {}),
    args: ['--no-sandbox']
  });
  try {
    async function scenario(name, options, verify) {
      const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
      page.setDefaultTimeout(10000);
      const report = { name, browser: browser.version(), started: new Date().toISOString(), console: [], errors: [], requests: [], checks: [] };
      reports.push(report);
      let latest = { ...available, hasUpdate: false, latest: null };
      let activeChecks = 0, maxActiveChecks = 0;
      const stateGate = options.holdState ? pause() : null;
      const unrelatedGate = options.holdUnrelated ? pause() : null;
      const checkGate = options.holdCheck ? pause() : null;
      const started = performance.now();
      page.on('pageerror', error => report.errors.push({ message: error.message, stack: error.stack }));
      page.on('console', message => report.console.push({ type: message.type(), text: message.text(), location: message.location() }));
      await page.addInitScript(() => {
        // Save identities before application code runs, without wrapping or binding.
        window.__nativeTimerEvidence = { setTimeout: window.setTimeout, clearTimeout: window.clearTimeout };
      });
      async function nativeTimersIntact() {
        const evidence = await page.evaluate(() => ({
          setTimeoutUnchanged: window.setTimeout === window.__nativeTimerEvidence.setTimeout,
          clearTimeoutUnchanged: window.clearTimeout === window.__nativeTimerEvidence.clearTimeout,
          setTimeoutSource: Function.prototype.toString.call(window.setTimeout),
          clearTimeoutSource: Function.prototype.toString.call(window.clearTimeout)
        }));
        report.nativeTimers = evidence;
        assert.equal(evidence.setTimeoutUnchanged, true);
        assert.equal(evidence.clearTimeoutUnchanged, true);
        assert.match(evidence.setTimeoutSource, /\[native code\]/);
        assert.match(evidence.clearTimeoutSource, /\[native code\]/);
      }
      await page.route('**/*', async route => {
        const url = new URL(route.request().url()), pathname = url.pathname;
        if (url.origin !== origin) {
          report.requests.push({ path: url.href, method: route.request().method(), blockedExternal: true });
          return route.abort('blockedbyclient');
        }
        if (pathname.startsWith('/api/')) {
          report.requests.push({ path: pathname, method: route.request().method(), elapsedMs: Math.round(performance.now() - started) });
          // Never let a test accidentally perform a download/install, even in the
          // fixture. All application endpoints are intercepted, not forwarded.
          if (/^\/api\/updates\/(download|install|import|open-releases)$/.test(pathname)) {
            return route.fulfill({ status: 403, contentType: 'application/json', body: JSON.stringify({ error: 'Forbidden in native timer regression' }) });
          }
          let data = {};
          if (pathname === '/api/status') data = { version: '1.17.64', os: 'windows', arch: 'amd64' };
          if (pathname === '/api/state' && route.request().method() === 'GET') {
            if (stateGate) await stateGate.promise;
            data = { appUpdatePreferences: options.preferences || {} };
          }
          if (pathname === '/api/system' || pathname === '/api/jobs') {
            if (unrelatedGate) await unrelatedGate.promise;
            data = pathname === '/api/jobs' ? [] : {};
          }
          if (pathname.includes('/events')) { await sleep(100); data = { events: [], sessions: [], after: 0 }; }
          if (pathname === '/api/updates/status') data = latest;
          if (pathname === '/api/updates/check') {
            const check = { number: report.checks.length + 1, startedMs: performance.now(), elapsedMs: Math.round(performance.now() - started) };
            report.checks.push(check);
            activeChecks++;
            maxActiveChecks = Math.max(maxActiveChecks, activeChecks);
            try {
              if (check.number === options.holdCheck) await checkGate.promise;
              if (options.failFirst && check.number === 1) {
                check.failed = true;
                return await route.fulfill({ status: 502, contentType: 'application/json', body: JSON.stringify({ error: 'Temporary network interruption' }) });
              }
              latest = available;
              return await route.fulfill({ contentType: 'application/json', body: JSON.stringify(latest) });
            } finally {
              activeChecks--;
              check.completedMs = performance.now();
            }
          }
          if (!page.isClosed()) return route.fulfill({ contentType: 'application/json', body: JSON.stringify(data) });
          return;
        }
        const filename = path.resolve(root, 'web', '.' + (pathname === '/' ? '/index.html' : pathname));
        if (!filename.startsWith(path.join(root, 'web') + path.sep)) return route.fulfill({ status: 403, body: '' });
        try {
          let body = pathname === '/assets/app-updater.js' && options.negative ? negativeUpdater : fs.readFileSync(filename);
          if (pathname === '/') body = body.toString().replaceAll('__SESSION_TOKEN__', 'native-browser-test').replaceAll('__WINDOW_MODE__', 'main');
          return route.fulfill({ body, contentType: pathname === '/' ? 'text/html' : pathname.endsWith('.js') ? 'application/javascript' : pathname.endsWith('.css') ? 'text/css' : 'application/octet-stream' });
        } catch { return route.fulfill({ status: 404, body: '' }); }
      });
      try {
        await page.goto(origin + '/');
        await page.waitForFunction(() => typeof NetworkToolkitUpdater === 'object' && online);
        await nativeTimersIntact();
        const h = {
          page, report,
          get checks() { return report.checks.length; },
          get activeChecks() { return activeChecks; },
          get maxActiveChecks() { return maxActiveChecks; },
          releaseState: () => stateGate?.resolve(),
          releaseCheck: () => checkGate?.resolve(),
          settings: async () => { await page.evaluate(() => appSettings()); await page.locator('[data-app-update-schedule]').waitFor(); },
          schedule: () => page.locator('[data-app-update-schedule]').innerText(),
          notice: () => page.locator('#appUpdateNotice').innerText(),
          waitForNotice: () => page.waitForFunction(() => document.querySelector('#appUpdateNotice').textContent.includes('1.17.65 is available')),
          waitForSchedule: async text => {
            await page.waitForFunction(value => document.querySelector('[data-app-update-schedule]')?.textContent.includes(value), text);
            if (text === 'Next automatic retry:' && !report.originalRetryObservedMs) report.originalRetryObservedMs = performance.now();
          },
          screenshot: suffix => page.screenshot({ path: path.join(output, 'native-' + name + '-' + suffix + '.png') }),
          waitPastOriginalRetry: async () => {
            // The production backoff is 30 seconds. Wait beyond the original
            // native deadline so a missed clearTimeout cannot pass unnoticed.
            assert.ok(report.originalRetryObservedMs, 'Must observe the original retry before replacing it');
            await sleep(Math.max(0, report.originalRetryObservedMs + 31500 - performance.now()));
          }
        };
        await verify(h);
        await nativeTimersIntact();
        if (!options.negative) assert.deepEqual(report.errors, [], 'Unexpected browser errors');
        assert.equal(report.requests.some(request => request.blockedExternal), false, 'Unexpected external network request');
        assert.equal(report.requests.some(request => /^\/api\/updates\/(download|install|import|open-releases)$/.test(request.path)), false, 'An update action was requested without approval');
        assert.ok(maxActiveChecks <= 1, 'Overlapping release-feed checks');
        report.passed = true;
        console.log('PASS native ' + name + ' (' + report.checks.length + ' release checks)');
      } catch (error) {
        report.failure = error.stack || String(error);
        throw error;
      } finally {
        report.durationMs = Math.round(performance.now() - started);
        report.maxActiveChecks = maxActiveChecks;
        await page.screenshot({ path: path.join(output, 'native-' + name + '-final.png') }).catch(() => {});
        fs.writeFileSync(path.join(output, 'native-' + name + '-console.json'), JSON.stringify(report, null, 2));
        stateGate?.resolve(); unrelatedGate?.resolve(); checkGate?.resolve();
        await page.close();
      }
    }

    // Required negative control: a test that passes only after replacing native
    // browser timers would fail this explicit reproduction of the released bug.
    await scenario('released64-negative-control', { negative: true, holdUnrelated: true }, async h => {
      await waitFor(() => h.report.errors.length > 0, 'Released bare timer wiring did not throw');
      assert.ok(h.report.errors.some(error => /Illegal invocation/.test(error.message)), 'Expected the native Window receiver exception');
      assert.ok(h.report.errors.every(error => /Illegal invocation/.test(error.message)), 'Negative control produced an unrelated exception');
      assert.ok(h.report.errors.some(error => /app-updater\.js/.test(error.stack)), 'Expected the production updater in the native error stack');
      await sleep(350);
      assert.equal(h.checks, 0, 'The broken startup must not contact the release feed');
      assert.equal(await h.notice(), '');
      await h.settings();
      assert.match(await h.schedule(), /No automatic update check has run.*Next automatic check/s, 'Expected the stale scheduled state caused by the native exception');
      await h.screenshot('stale-schedule');
    });

    await scenario('startup-and-settings', { holdUnrelated: true, holdCheck: 1 }, async h => {
      await waitFor(() => h.checks === 1, 'Native startup did not reach the release feed');
      await h.page.evaluate(() => openModal('Keep my work', '<input id="nativeTestDraft" value="not lost">'));
      h.releaseCheck();
      await h.waitForNotice();
      assert.equal(await h.page.locator('#nativeTestDraft').inputValue(), 'not lost');
      assert.equal(await h.page.evaluate(() => window.__networkToolkitStartupState), 0, 'System/jobs fixture should still be pending');
      await h.page.evaluate(() => { closeModal(); navigate('subnet'); });
      assert.match(await h.notice(), /1\.17\.65 is available/);
      await h.screenshot('automatic-banner');
      await h.settings();
      await h.waitForSchedule('Next automatic check:');
      assert.match(await h.schedule(), /Last automatic attempt.*Update available: 1\.17\.65.*Next automatic check/s);
      await h.screenshot('settings');
      await h.page.evaluate(() => closeModal());
      for (const width of [390, 1280]) {
        await h.page.setViewportSize({ width, height: 900 });
        await h.page.locator('#appUpdateNotice [data-app-update="download"]').click();
        const card = h.page.locator('.nt-update-notes');
        await card.waitFor();
        assert.equal(await card.locator('a,script,img,iframe').count(), 0);
        assert.equal(await h.page.evaluate(() => window.notesExecuted === true), false);
        assert.equal(await card.evaluate(element => element.scrollWidth <= element.clientWidth + 1), true);
        await h.screenshot('notes-' + width);
        await h.page.getByRole('button', { name: 'Not now', exact: true }).click();
      }
      assert.equal(h.checks, 1);
    });

    await scenario('saved-opt-out', { holdState: true, preferences: { enabled: false } }, async h => {
      await h.settings();
      assert.equal(await h.page.locator('[data-app-update-auto]').isDisabled(), true);
      await sleep(250);
      assert.equal(h.checks, 0);
      h.releaseState();
      await h.waitForSchedule('Automatic checks are turned off.');
      await sleep(250);
      assert.equal(h.checks, 0);
      await h.page.locator('[data-app-update="check"]').click();
      await h.waitForNotice();
      assert.equal(h.checks, 1, 'Manual check should still work with saved automatic opt-out');
    });

    // Run the three real 30-second windows together, without changing browser
    // time, production delays, timer functions or scheduling implementation.
    const delayed = await Promise.allSettled([
      scenario('real-backoff-retry', { failFirst: true }, async h => {
        await waitFor(() => h.checks === 1 && h.activeChecks === 0, 'First release check did not complete');
        await h.settings();
        await h.waitForSchedule('Next automatic retry:');
        assert.match(await h.schedule(), /Temporary network interruption/);
        assert.equal(await h.notice(), '');
        await h.screenshot('pending-retry');
        await waitFor(() => h.checks >= 2, 'Native 30-second retry never ran', 45000);
        await h.waitForNotice();
        assert.equal(h.checks, 2);
        const gap = h.report.checks[1].startedMs - h.report.checks[0].completedMs;
        assert.ok(gap >= 29500, 'Retry happened before the real native 30-second backoff: ' + gap);
        assert.ok(gap < 45000, 'Native retry took unexpectedly long: ' + gap);
        await h.waitForSchedule('Next automatic check:');
        assert.match(await h.schedule(), /Update available: 1\.17\.65/);
      }),
      scenario('manual-cancels-native-retry', { failFirst: true }, async h => {
        await waitFor(() => h.checks === 1 && h.activeChecks === 0, 'First release check did not complete');
        await h.settings();
        await h.waitForSchedule('Next automatic retry:');
        await h.page.locator('[data-app-update="check"]').click();
        await h.waitForNotice();
        await h.waitForSchedule('Next automatic check:');
        assert.equal(h.checks, 2);
        await h.waitPastOriginalRetry();
        assert.equal(h.checks, 2, 'Manual success failed to cancel the original native retry');
      }),
      scenario('toggle-cancellation-no-overlap', { failFirst: true, holdCheck: 2 }, async h => {
        await waitFor(() => h.checks === 1 && h.activeChecks === 0, 'First release check did not complete');
        await h.settings();
        await h.waitForSchedule('Next automatic retry:');
        const toggle = h.page.locator('[data-app-update-auto]');
        await toggle.uncheck();
        await h.waitForSchedule('Automatic checks are turned off.');
        await toggle.check();
        await waitFor(() => h.checks === 2 && h.activeChecks === 1, 'Re-enabling did not begin one native check');
        assert.equal(await h.page.locator('[data-app-update="check"]').isDisabled(), true);
        await h.page.locator('[data-app-update="check"]').dispatchEvent('click');
        await h.page.locator('[data-app-update="check"]').dispatchEvent('click');
        await toggle.uncheck();
        await toggle.check();
        await h.waitForSchedule('Next readiness check:');
        assert.equal(h.checks, 2, 'Toggling or manual clicks overlapped the in-flight check');
        await toggle.uncheck();
        h.releaseCheck();
        await waitFor(() => h.activeChecks === 0, 'Held release check did not finish');
        await h.waitForSchedule('Automatic checks are turned off.');
        await h.waitPastOriginalRetry();
        assert.equal(h.checks, 2, 'Disabling failed to cancel the original native retry');
        await toggle.check();
        await waitFor(() => h.checks === 3 && h.activeChecks === 0, 'Re-enabled scheduler did not recover');
        await h.waitForSchedule('Next automatic check:');
        assert.match(await h.notice(), /1\.17\.65 is available/);
        assert.equal(h.maxActiveChecks, 1);
      })
    ]);
    const failures = delayed.filter(result => result.status === 'rejected');
    if (failures.length) throw new AggregateError(failures.map(result => result.reason), 'Native timer scenarios failed');
  } finally {
    await browser.close();
    fs.writeFileSync(path.join(output, 'native-timer-summary.json'), JSON.stringify(reports, null, 2));
  }
}
main().catch(error => {
  console.error(error);
  fs.writeFileSync(path.join(output, 'native-timer-failure.txt'), error.stack || String(error));
  process.exitCode = 1;
});
