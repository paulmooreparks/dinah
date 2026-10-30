// Stop asks the engine to end the turn and says so while the agent finishes what it
// is doing, since a tool call can take a while to end; a failure to ask is reported.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => {
      window.__cancels = [];
      return {
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        OpenSessionAt: async (ws, id) => ({ sessionId: id, title: id, provider: 'ollama', cwd: ws, open: true, working: true, chatState: { life: 'open', window: true, exec: { state: 'working', why: 'Turn in progress' } }, queued: [] }),
        Cancel: async id => { window.__cancels.push(id); if (window.__failCancel) throw new Error('Fixture: the stop was not delivered'); },
        SkillsForProviderAt: async () => [], SkillsAt: async () => []
      };
    },
    ready: () => window.dinahWorkbench && window.dinahEditor,
    goto: false,
  });
  const { page, ok } = t;
  const ID = 'ollama:stop';
  try {
    await page.goto(require('./lib/page').PAGE);
    await t.waitReady();
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'ollama:stop' }));
    const win = page.locator('[data-win="' + key + '"]');
    await win.locator('[data-view-id="prompt"]').waitFor();
    const stop = win.locator('[data-command="stop-turn"]').first();
    const line = () => win.locator('.win-execution .win-state').getAttribute('title');
    await stop.waitFor({ state: 'attached' });
    await page.waitForFunction(k => /Working/.test(document.querySelector('[data-win="' + k + '"] .win-execution .win-state').title), key);
    ok('a running turn says it is working', /Working/.test(await line()));

    await page.evaluate(() => { window.__failCancel = true; });
    await stop.click();
    await page.waitForFunction(() => window.__cancels.length === 1);
    await page.waitForFunction(k => /Working/.test(document.querySelector('[data-win="' + k + '"] .win-execution .win-state').title), key);
    ok('a stop that could not be asked for is reported and the chat is back to working', /Working/.test(await line()) && (await page.evaluate(() => document.body.textContent)).includes('the stop was not delivered'));

    await page.evaluate(() => { window.__failCancel = false; });
    await stop.click();
    await page.waitForFunction(k => /Stop was asked for/.test(document.querySelector('[data-win="' + k + '"] .win-execution .win-state').title), key);
    ok('after Stop the chat says it was asked for', /Stop was asked for/.test(await line()));
    ok('the Stop control shows it', (await stop.textContent()).includes('Stopping'));
    ok('the execution badge remains Working while stopping is pending', await win.getAttribute('data-win-state') === 'working');
    ok('the control stays pressable while it says Stopping', await stop.isEnabled());
    await stop.click();
    await page.waitForFunction(() => window.__cancels.length === 3);
    ok('a second press asks the engine again', (await page.evaluate(() => window.__cancels.length)) === 3, String(await page.evaluate(() => window.__cancels.length)));
    await win.locator('[data-view-id="prompt"]').press('Escape');
    await page.waitForFunction(() => window.__cancels.length === 4);
    ok('so does Escape', (await page.evaluate(() => window.__cancels.length)) === 4);

    await page.evaluate(i => __emit('agent:turn', { sessionId: i, stopReason: 'cancelled' }), ID);
    await page.evaluate(i => __emit('agent:session', { sessionId: i, working: false, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } }, open: true, queued: [], provider: 'ollama' }), ID);
    await page.waitForFunction(k => !/Stop was asked for/.test(document.querySelector('[data-win="' + k + '"] .win-execution .win-state').title), key);
    ok('the end of the turn clears it', /Idle/.test(await line()));

    // The Stop command, from the menu or command search, is the same Stop.
    await page.evaluate(i => __emit('agent:session', { sessionId: i, working: true, chatState: { life: 'open', window: true, exec: { state: 'working', why: 'Turn in progress' } }, open: true, queued: [], provider: 'ollama' }), ID);
    await page.waitForFunction(k => /Working/.test(document.querySelector('[data-win="' + k + '"] .win-execution .win-state').title), key);
    await page.evaluate(() => { window.__cancels.length = 0; });
    ok('the Stop command is available while a turn runs', await page.evaluate(() => dinahCommands.enabled ? dinahCommands.enabled('cancel') !== false : true));
    await page.evaluate(() => dinahCommands.run('cancel'));
    await page.waitForFunction(k => /Stop was asked for/.test(document.querySelector('[data-win="' + k + '"] .win-execution .win-state').title), key);
    ok('stopping from the command shows the same feedback and asks the engine', (await page.evaluate(() => window.__cancels.join(','))) === ID, await page.evaluate(() => window.__cancels.join(',')));
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
