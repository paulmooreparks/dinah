// The providers scan says in words only that the engine's call failed. A fault in what the page does with
// the answer is not turned into those words: it is reported as an error, so it fails loudly in the suite
// that causes it.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    expectErrors: /module mount failed/,
    app: () => {
      window.__fail = false;
      return {
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'], defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [] }),
        AgentProviders: async () => { if (window.__fail) throw new Error('the engine did not answer'); return [window.__provider('codex'), window.__provider('broken')]; }
      };
    },
  });
  const { page, ok } = t;
  try {
    // A module for a provider whose mount fails stands for a fault in the page; it fails once, when the suite says so,
    // so a scan the page makes by itself at another time causes nothing.
    await page.evaluate(() => { window.__break = false; window.dinahProviderSettings.register('broken', { mount: () => { if (window.__break) { window.__break = false; throw new Error('module mount failed'); } return {}; } }); });
    await page.evaluate(() => dinahWorkbench.open('settings'));
    const settings = page.locator('#settings');
    await settings.getByRole('tab', { name: 'Agents', exact: true }).click();
    await page.waitForFunction(() => document.getElementById('scan-agents') && !document.getElementById('scan-agents').disabled);
    // Taking the module away makes the next scan mount it again, which is when it fails.
    await page.evaluate(() => { window.dinahProviderSettings.draw([]); window.__break = true; });
    await settings.locator('#scan-agents').click();
    await page.waitForFunction(() => document.getElementById('scan-agents') && !document.getElementById('scan-agents').disabled);
    await page.waitForTimeout(300);
    ok('a fault in what the page does with the answer is reported as an error', t.expected.length === 1, t.expected.join(' | '));
    ok('and is not shown as the words about agents not being found', !/could not be found/.test(await settings.locator('#agents-status').textContent()));
    await page.evaluate(() => { window.__break = false; });

    await page.evaluate(() => { window.__fail = true; });
    await settings.locator('#scan-agents').click();
    await settings.locator('#agents-status', { hasText: 'could not be found' }).waitFor();
    ok('a failed call to the engine is said in words, with what failed', /the engine did not answer/.test(await settings.locator('#agents-status').textContent()) && t.errors.length === 0);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
