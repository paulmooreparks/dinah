// A chat that opens shows its saved copy at once, whether the agent resumes or replays: the
// replay runs behind the copy and takes its place when it ends, and the box works meanwhile. A
// message sent while the chat is still opening shows as waiting and goes to the agent when the
// chat is ready.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    app: () => {
      window.__calls = [];
      let release;
      window.__gate = new Promise(r => { release = r; });
      window.__release = release;
      return {
        State: async () => ({ workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider: 'claude', catalog: [], defaults: {}, pinned: [] }),
        AgentProviders: async () => [{ id: 'claude', installed: true, available: true }],
        ResolveSessionContinuation: async id => id,
        SessionContinuation: async () => ({ history: [] }),
        // The first read finds nothing to draw (the chat reopens as it would with no copy); the next finds the copy.
        SavedConversation: async id => (window.__saves = (window.__saves || 0) + 1) < 2 ? { available: false, messages: [] } : ({ available: true, complete: true, messages: [{ role: 'you', text: 'Summarise the plan.' }, { role: 'agent', text: 'SAVED-ANSWER' }] }),
        OpenSessionAt: async (ws, id) => {
          window.__calls.push(['open', id]);
          // The engine announces the chat as soon as it has registered it, before its history is loaded.
          window.__emit('agent:session', { sessionId: id, provider: 'claude', title: 'Waking Chat', cwd: ws, open: true });
          await window.__gate;
          const u = update => window.__emit('agent:update', { sessionId: id, update });
          u({ sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Summarise the plan.' } });
          u({ sessionUpdate: 'agent_thought_chunk', content: { type: 'text', text: 'REPLAYED-THOUGHT' } });
          u({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'REPLAYED-ANSWER' } });
          return { sessionId: id, provider: 'claude', title: 'Waking Chat', cwd: ws, open: true };
        },
        Send: async (id, text) => { window.__calls.push(['send', id, text]); }
      };
    },
  });
  const { page, ok } = t;
  try {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:waking' }));
    await page.waitForFunction(() => __calls.some(c => c[0] === 'open'));
    const win = page.locator('[data-win="' + key + '"]');
    await win.getByText('SAVED-ANSWER').first().waitFor();
    ok('while the agent is still opening, the saved copy is in sight', true);
    ok('and the page does not hide the chat behind a replay line', await win.locator('[data-view-id="replay"]').isHidden());

    // The window says in its title bar that the history is still arriving.
    const chip = win.locator('.win-head .chat-loading-chip');
    // The page draws the chip once the copy is drawn, a moment after its text is in sight.
    await chip.waitFor();
    ok('while the history loads, the title bar has a Loading chip with a spinner', await chip.isVisible() && /Loading/.test(await chip.textContent()) && await chip.locator('.spinner').count() === 1);

    // A message sent now goes to the engine at once, which holds it in the chat's queue and shows it as waiting.
    const box = win.locator('[data-view-id="prompt"]');
    await box.fill('And the next step?');
    await box.press('Control+Enter');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'send'));
    ok('a message sent while the chat is opening goes to the engine at once, once', JSON.stringify(await page.evaluate(() => __calls.filter(c => c[0] === 'send'))) === JSON.stringify([['send', 'claude:waking', 'And the next step?']]));
    ok('the page draws no row of its own for it, so it cannot land before the history', await win.locator('.waiting-row').count() === 0 && !/And the next step\?/.test(await win.locator('#logs, [data-view-id="logs"]').first().textContent()));
    await page.evaluate(() => __emit('agent:queue', { sessionId: 'claude:waking', queued: ['And the next step?'] }));
    await win.locator('#queue:not([hidden]), [data-view-id="queue"]:not([hidden])').first().waitFor();
    ok('the engine\'s queue shows it as waiting', /And the next step\?/.test(await win.locator('[data-view-id="queue"]').textContent()));

    await page.evaluate(() => window.__release());
    await win.getByText('REPLAYED-ANSWER').first().waitFor({ state: 'attached' });
    await chip.waitFor({ state: 'hidden' });
    ok('and the chip goes when the chat is ready', true);
    await page.waitForTimeout(300);
    const text = await win.innerText();
    ok('the replay takes the copy\'s place, with nothing twice', !/SAVED-ANSWER/.test(text) && text.split('Summarise the plan.').length === 2, text);
    // The stub sends no agent:sent for the held message, so any row for it was drawn by the page itself.
    const history = await win.locator('[data-view-id="logs"]').innerText();
    ok('and the page drew no row of its own for the held message among the history', !/And the next step\?/.test(history), history);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
