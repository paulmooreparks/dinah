// Closed-schema copy, privacy, bounded storage and recording-control races.
const { launch } = require('./lib/page');
const { execFileSync } = require('child_process');
const path = require('path');
execFileSync(process.execPath, [path.join(__dirname, 'unit/timing-snapshot.test.js')], { stdio: 'inherit', windowsHide: true });
(async () => {
  const t = await launch({
    app: () => {
      let enabled = false, generation = 0;
      const anchor = () => ({ utc: new Date().toISOString(), monoMs: performance.now() });
      window.__timingControls = []; window.__timingGets = 0; window.__timingRetained = [];
      return {
        SetResponsivenessRecording: async wanted => {
          __timingControls.push(wanted);
          if (enabled !== wanted) { enabled = wanted; generation++; if (!wanted) __timingRetained = []; }
          if (window.__holdControl) await new Promise(resolve => { window.__releaseControl = resolve; });
          return { enabled, generation, anchor: anchor(), state: enabled ? 'on' : 'off' };
        },
        GetResponsivenessTimingSnapshot: async () => {
          __timingGets++;
          if (window.__holdGet) await new Promise(resolve => { window.__releaseGet = resolve; });
          const value = { schemaVersion: 1, recording: { enabled, generation, anchor: anchor(), state: enabled ? 'on' : 'off' }, capturedAt: anchor(),
            coverage: { retainedSlots: 0, overwrittenSlots: 0, missedPeriods: __timingRetained.length, disabledObservations: { state: 'unavailable', value: null } }, slots: [] };
          if (window.__badField) value.privateText = window.__canary;
          return value;
        },
      };
    },
    ready: 'workbench',
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahWorkbench.open('settings'));
    await p.getByRole('tab', { name: 'Diagnostics', exact: true }).click();
    const toggle = p.locator('#set-responsiveness'), copy = p.locator('#copy-responsiveness'), status = p.locator('#responsiveness-copy-status');
    const copies = () => p.evaluate(() => __runtime.filter(c => c[0] === 'ClipboardSetText').length);
    await p.waitForFunction(() => __timingControls.length > 0);
    ok('recording begins off and exports nothing automatically', await toggle.getAttribute('aria-checked') === 'false' && await copies() === 0 && await p.evaluate(() => __timingGets === 0));
    await toggle.click();
    await p.waitForFunction(() => __timingControls.at(-1) === true);
    await p.evaluate(() => {
      window.__canary = 'PRIVATE-TIMING-CANARY-C:/Secret/workspace https://private.invalid/ command --token=secret';
      document.title = __canary;
      document.querySelector('#settings').setAttribute('data-private-title', __canary);
      const prompt = document.createElement('textarea'); prompt.value = __canary; prompt.title = __canary; document.body.appendChild(prompt);
      dinahResponsiveness.record('render', __canary, 75, 20);
    });
    await copy.click();
    await p.waitForFunction(() => __runtime.some(c => c[0] === 'ClipboardSetText'));
    let text = await p.evaluate(() => __runtime.filter(c => c[0] === 'ClipboardSetText').at(-1)[1]);
    const exported = JSON.parse(text.slice(text.indexOf('\n') + 1));
    ok('copy reads the backend once and uses the disclosure header', await p.evaluate(() => __timingGets === 1) && text.startsWith('Dinah timing snapshot.') && text.includes('retained input events'));
    ok('copy contains neither canary content nor producer labels', !text.includes('PRIVATE-TIMING-CANARY') && !text.includes('private.invalid') && exported.browser.raw.work.some(r => r.category === 'other'));
    ok('every exported value follows the closed schema', await p.evaluate(value => { dinahTimingSnapshot.validate('Export', value); return true; }, exported));
    ok('copy leaves recording enabled', await toggle.getAttribute('aria-checked') === 'true');
    ok('the clearing warning is visible beside the control', await p.locator('#settings-diagnostics').getByText(/Turning recording off clears/).isVisible());

    // An unknown field from even the backend refuses the entire copy.
    const count = await copies();
    await p.evaluate(() => { window.__badField = true; });
    await copy.click();
    await status.getByText(/could not be copied/).waitFor();
    ok('unexpected backend fields cannot leak into a partial copy', await copies() === count);
    await p.evaluate(() => { window.__badField = false; });

    // An export in flight cannot combine recordings across a toggle.
    await p.evaluate(() => { window.__holdGet = true; });
    await copy.click();
    await p.waitForFunction(() => !!window.__releaseGet);
    await toggle.click();
    await p.evaluate(() => { window.__holdGet = false; window.__releaseGet(); window.__releaseGet = null; });
    await status.getByText(/Recording changed/).waitFor();
    ok('a generation change refuses the pending copy and clears browser data', await copies() === count && await p.evaluate(() => dinahResponsiveness.snapshot().length === 0));

    // Coalesced intent is applied in order, even when an old request resolves late.
    await p.evaluate(() => { window.__holdControl = true; });
    await toggle.click();
    await p.waitForFunction(() => !!window.__releaseControl);
    await toggle.click();
    await p.evaluate(() => { window.__holdControl = false; window.__releaseControl(); window.__releaseControl = null; });
    await p.waitForFunction(() => __timingControls.at(-1) === false);
    ok('a late enabled response cannot turn recording back on', await toggle.getAttribute('aria-checked') === 'false' && await copies() === count);
    await toggle.click();
    await p.waitForFunction(() => __timingControls.at(-1) === true);
    await p.locator('#clear-responsiveness').click();
    await p.waitForFunction(() => document.querySelector('#set-responsiveness').getAttribute('aria-checked') === 'true' && __timingControls.at(-1) === true && __timingControls.slice(-3).join(',') === 'true,false,true');
    ok('clear resets both sources through an acknowledged off/on transition', await copies() === count);
    await copy.click();
    await p.waitForFunction(n => __runtime.filter(c => c[0] === 'ClipboardSetText').length === n + 1, count);
    text = await p.evaluate(() => __runtime.filter(c => c[0] === 'ClipboardSetText').at(-1)[1]);
    ok('recording remains copyable after clear', !text.includes('PRIVATE-TIMING-CANARY'));
    // An off/on intent during a delayed enable acknowledgment must clear backend history.
    await toggle.click();
    await p.waitForFunction(() => __timingControls.at(-1) === false);
    await p.evaluate(() => { window.__holdControl = true; });
    await toggle.click();
    await p.waitForFunction(() => !!window.__releaseControl);
    const delayedStart = await p.evaluate(() => { __timingRetained.push(123); return __timingControls.length; });
    for (let i = 0; i < 10; i++) await toggle.click();
    ok('rapid toggles keep only bounded intent while acknowledgment is pending', await p.evaluate(n => __timingControls.length === n, delayedStart));
    await p.evaluate(() => { window.__holdControl = false; __releaseControl(); window.__releaseControl = null; });
    await p.waitForFunction(() => __timingControls.slice(-3).join(',') === 'true,false,true');
    const resetCopies = await copies();
    await copy.click();
    await p.waitForFunction(n => __runtime.filter(c => c[0] === 'ClipboardSetText').length === n + 1, resetCopies);
    const resetExport = await p.evaluate(() => JSON.parse(__runtime.filter(c => c[0] === 'ClipboardSetText').at(-1)[1].split('\n').slice(1).join('\n')));
    ok('an intervening disable clears the old backend sample before the new copy', resetExport.backend.coverage.missedPeriods === 0 && resetExport.backend.recording.generation >= 3);
    // A stalled control cannot accumulate new reads or change the result after timeout.
    await toggle.click();
    await p.waitForFunction(() => __timingControls.at(-1) === false);
    await p.evaluate(() => { window.__holdControl = true; });
    await toggle.click();
    await p.waitForFunction(() => !!window.__releaseControl);
    const beforeControlCopy = await copies();
    await copy.click();
    await p.waitForFunction(() => document.querySelector('#responsiveness-copy-status').textContent.includes('could not be copied'), null, { timeout: 15000 });
    ok('a stalled control bounds preparation and re-enables Copy', await copy.isEnabled() && await copies() === beforeControlCopy);
    const getsBeforeRetry = await p.evaluate(() => __timingGets);
    await copy.click();
    await status.getByText(/previous timing request is still pending/).waitFor();
    await p.evaluate(() => { window.__holdControl = false; window.__releaseControl(); window.__releaseControl = null; });
    await p.waitForTimeout(100);
    ok('late control settlement starts no export and changes no expired result', await copies() === beforeControlCopy && await p.evaluate(n => __timingGets === n, getsBeforeRetry) && (await status.textContent()).includes('previous timing request'));
    await copy.click();
    await p.waitForFunction(n => __runtime.filter(c => c[0] === 'ClipboardSetText').length === n + 1, beforeControlCopy);

    // The same bound applies to a stalled Get, including a retry before it settles.
    await p.evaluate(() => { window.__holdGet = true; });
    const beforeReadCopy = await copies();
    await copy.click();
    await p.waitForFunction(() => !!window.__releaseGet);
    await p.waitForFunction(() => document.querySelector('#responsiveness-copy-status').textContent.includes('could not be copied'), null, { timeout: 15000 });
    const readsBeforeRetry = await p.evaluate(() => __timingGets);
    await copy.click();
    await status.getByText(/previous timing request is still pending/).waitFor();
    await p.evaluate(() => { window.__holdGet = false; window.__releaseGet(); window.__releaseGet = null; });
    await p.waitForTimeout(100);
    ok('a late read cannot write or replace the result, and retries stay bounded', await copies() === beforeReadCopy && await p.evaluate(n => __timingGets === n, readsBeforeRetry) && (await status.textContent()).includes('previous timing request'));
    await copy.click();
    await p.waitForFunction(n => __runtime.filter(c => c[0] === 'ClipboardSetText').length === n + 1, beforeReadCopy);
    ok('a new Copy works after the timed-out read has settled', await copy.isEnabled());

    // Submission is irreversible. The preparation deadline cannot claim to cancel it.
    await p.evaluate(() => {
      window.runtime.ClipboardSetText = text => new Promise(resolve => {
        window.__releaseClipboard = () => { __runtime.push(['ClipboardSetText', text]); resolve(true); };
      });
    });
    const beforeClipboard = await copies();
    await copy.click();
    await p.waitForFunction(() => !!window.__releaseClipboard);
    await p.waitForTimeout(10500);
    ok('an outstanding clipboard write remains pending beyond the preparation deadline', await copy.isDisabled() && (await status.textContent()).includes('waiting for clipboard') && await copies() === beforeClipboard);
    await toggle.click();
    ok('turning recording off does not claim to cancel a submitted copy', (await status.textContent()).includes('does not cancel it'));
    await p.evaluate(() => window.__releaseClipboard());
    await status.getByText(/requested timing snapshot was copied/).waitFor();
    ok('the submitted copy reports its real completion without restarting recording', await copies() === beforeClipboard + 1 && await toggle.getAttribute('aria-checked') === 'false');

    // The documented CDP heap commands are test-only and never run in the product.
    const cdp = await t.context.newCDPSession(p), retained = [];
    await cdp.send('HeapProfiler.enable');
    await p.evaluate(() => { window.__memoryAt = 1000; window.__memoryRecorder = dinahTimingSnapshot.create({ now: () => __memoryAt }); __memoryRecorder.begin(1); });
    for (const periods of [3000, 3000, 6000]) {
      const shape = await p.evaluate(periods => {
        for (let i = 0; i < periods; i++) { __memoryAt += 1000; for (let j = 0; j < 10; j++) __memoryRecorder.record('chat output', 8, 0); }
        const snapshot = __memoryRecorder.snapshot();
        return { slots: snapshot.slots.length, raw: snapshot.raw.work.length, bytes: JSON.stringify(snapshot).length };
      }, periods);
      await cdp.send('HeapProfiler.collectGarbage');
      retained.push({ ...shape, used: (await cdp.send('Runtime.getHeapUsage')).usedSize });
    }
    const heapSpread = Math.max(...retained.map(x => x.used)) - Math.min(...retained.map(x => x.used));
    ok('retained browser storage plateaus through repeated wraps', retained.every(x => x.slots === 600 && x.raw === 200) && heapSpread < Math.max(65536, retained[0].used * 0.01), JSON.stringify(retained));
    console.log('MEMORY ' + JSON.stringify(retained));
    await p.evaluate(() => { delete window.__memoryRecorder; delete window.__memoryAt; });
    await cdp.detach();
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
