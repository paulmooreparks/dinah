// A chat reopened with session/resume has no replay to draw: its saved copy stays in sight, the
// chat's log (new and empty) is read from its start, and there is no note about it, only a command
// and subagents are not shown. Load Full History asks the agent for the whole conversation,
// which the page treats as the chat closed and opened again; it is unavailable while the chat works.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    app: () => {
      window.__calls = [];
      const entry = (n, text) => ({ n, name: 'agent:update', data: { update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } } } });
      return {
        State: async () => ({ workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider: 'claude', catalog: [], defaults: {}, pinned: [] }),
        AgentProviders: async () => [{ id: 'claude', installed: true, available: true }],
        ResolveSessionContinuation: async id => id,
        SessionContinuation: async () => ({ history: [] }),
        SavedConversation: async id => (window.__calls.push(['saved', id]), { available: true, complete: true, messages: [{ role: 'you', text: 'Summarise the plan.' }, { role: 'agent', text: 'SAVED-ANSWER' }] }),
        OpenSessionAt: async (ws, id) => {
          window.__calls.push(['open', id]);
          return { sessionId: id, provider: 'claude', title: 'Resumed Chat', cwd: ws, open: true, resumed: true, streamGen: 'r1', streamHead: 0 };
        },
        ChatEvents: async (id, gen, after) => {
          window.__calls.push(['events', id, gen, after]);
          return { gen, head: 2, truncated: false, reset: false, events: [entry(1, ' [turn so far one] '), entry(2, ' [turn so far two] ')] };
        },
        LoadFullHistory: async (ws, id) => {
          window.__calls.push(['load', ws, id]);
          // The agent replays the whole conversation, in a generation of its own.
          const u = (n, update) => window.__emit('agent:update', { sessionId: id, gen: 'f1', n, update });
          u(1, { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Summarise the plan.' } });
          u(2, { sessionUpdate: 'agent_thought_chunk', content: { type: 'text', text: 'REPLAYED-THOUGHT' } });
          u(3, { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'REPLAYED-ANSWER' } });
          return { sessionId: id, provider: 'claude', title: 'Resumed Chat', cwd: ws, open: true, streamGen: 'f1', streamHead: 3 };
        }
      };
    },
  });
  const { page, ok } = t;
  try {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:resumed' }));
    const win = page.locator('[data-win="' + key + '"]');
    await win.getByText('[turn so far two]').first().waitFor({ state: 'attached' });
    const text = await win.innerText();
    ok('the saved copy is in sight and the turn so far follows it', /Summarise the plan\./.test(text) && /SAVED-ANSWER/.test(text) && text.indexOf('SAVED-ANSWER') < text.indexOf('[turn so far one]'), text);
    ok('the chat\'s log is read from its start in the generation the reopen made', JSON.stringify(await page.evaluate(() => window.__calls.filter(c => c[0] === 'events'))) === JSON.stringify([['events', 'claude:resumed', 'r1', 0]]));
    ok('it is not shown as a saved copy waiting to reconnect, nor as reopening', await win.locator('#saved-note').count() === 0 || await win.locator('[data-view-id="saved-note"]').isHidden());

    const menuAction = async () => {
      await page.evaluate(k => pudlWindows.raise(k), key);
      await page.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click();
      const item = page.locator('#menubar-panel .menu-action', { hasText: 'Load Full History' }).first();
      await item.waitFor();
      const disabled = await item.evaluate(el => el.disabled || el.getAttribute('aria-disabled') === 'true');
      await page.keyboard.press('Escape');
      return !disabled;
    };
    ok('Load Full History is in the window\'s Actions menu, enabled while the chat is idle', await menuAction());

    // Load Full History is unavailable while the chat works.
    await page.evaluate(() => __emit('agent:session', { sessionId: 'claude:resumed', provider: 'claude', cwd: 'C:/Test', title: 'Resumed Chat', open: true, resumed: true, working: true }));
    await page.waitForTimeout(150);
    ok('the control and the command are disabled while the chat is working', !(await menuAction()));
    await page.evaluate(() => __emit('agent:session', { sessionId: 'claude:resumed', provider: 'claude', cwd: 'C:/Test', title: 'Resumed Chat', open: true, resumed: true, working: false }));
    await page.waitForTimeout(150);

    await page.evaluate(k => pudlWindows.raise(k), key);
    await page.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click();
    await page.locator('#menubar-panel .menu-action', { hasText: 'Load Full History' }).first().click();
    await win.getByText('REPLAYED-ANSWER').first().waitFor({ state: 'attached' });
    await page.waitForFunction(() => __calls.some(c => c[0] === 'load'));
    const after = await win.innerText();
    ok('Load Full History asks the engine for the chat\'s history', JSON.stringify(await page.evaluate(() => window.__calls.filter(c => c[0] === 'load'))) === JSON.stringify([['load', 'C:/Test', 'claude:resumed']]));
    ok('the replay replaces what was drawn, with nothing twice', after.split('Summarise the plan.').length === 2 && !/SAVED-ANSWER/.test(after) && !/\[turn so far/.test(after), after);
    ok('after a full load the command is not offered again, since the chat is no longer a resumed one', await (async () => { await page.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click(); const n = await page.locator('#menubar-panel .menu-action', { hasText: 'Load Full History' }).count(); await page.keyboard.press('Escape'); return n === 0; })());
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
