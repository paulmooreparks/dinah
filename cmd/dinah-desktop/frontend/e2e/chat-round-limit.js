// A turn that stops at the round limit keeps the protocol's stop reason, and
// its row offers Continue, which sends an ordinary message through the chat's
// own send path. The stop's own words are the adapter's sentence in the chat,
// not the row's (chat-applet.js).
const { launch, PAGE } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => {
      window.__calls = [];
      return {
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        Sessions: async () => [
          { sessionId: 'ollama-chat', title: 'Titled by Ollama', provider: 'ollama', cwd: 'C:/Fixture' },
        ],
        OpenSessionAt: async (ws, id) => ({ sessionId: id, title: id, provider: 'ollama', cwd: ws, open: true, working: false, queued: [] }),
        Send: async (id, text) => { window.__calls.push(['send', id, text]); return {}; },
        AgentProviders: async () => [{ id: 'ollama', name: 'Ollama', installed: true, available: true }],
        SkillsForProviderAt: async () => [], SkillsAt: async () => []
      };
    },
    goto: false,
  });
  const { page, ok } = t;
  try {
    await page.goto(PAGE);
    await t.waitReady();
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'ollama-chat' }));
    const win = page.locator('[data-win="' + key + '"]');
    await win.locator('[data-view-id="prompt"]').waitFor();
    // The adapter writes its sentence into the chat as the turn ends.
    await page.evaluate(() => {
      __emit('agent:update', { sessionId: 'ollama-chat', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'The turn stopped at the limit of 60 rounds of tool calls that you set in Settings. Send a message to continue.' } } });
      __emit('agent:turn', { sessionId: 'ollama-chat', stopReason: 'max_turn_requests' });
    });
    await win.getByText('stopped at the limit of 60 rounds of tool calls that you set in Settings').waitFor();
    const row = win.locator('.turn-row').last();
    await row.locator('.turn-continue').waitFor();
    ok('the stop is its own sentence in the chat, and the row keeps the protocol’s words with one offer', /stopped at its limit of steps for one turn/.test(await row.textContent()) && (await row.locator('.turn-continue').textContent()) === 'Continue', await row.textContent());
    await row.locator('.turn-continue').click();
    await page.waitForFunction(() => window.__calls.length === 1);
    ok('Continue sends the plain message through the chat’s own send', JSON.stringify(await page.evaluate(() => window.__calls)) === JSON.stringify([['send', 'ollama-chat', 'Keep going from where you stopped.']]), JSON.stringify(await page.evaluate(() => window.__calls)));
    ok('and the box is emptied as after any message', (await win.locator('[data-view-id="prompt"]').inputValue()) === '');
    // A turn that ends in error keeps the error and offers nothing.
    await page.evaluate(() => __emit('agent:turn', { sessionId: 'ollama-chat', stopReason: 'max_turn_requests', error: 'connection dropped' }));
    const row2 = win.locator('.turn-row').last();
    await row2.getByText('The turn ended with an error: connection dropped').waitFor();
    ok('the same stop reason with an error offers no Continue', (await row2.locator('.turn-continue').count()) === 0);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
