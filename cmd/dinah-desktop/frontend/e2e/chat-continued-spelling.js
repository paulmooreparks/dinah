// A chat that was continued under another session has a window and a pinned
// button for its old session, saved with its folder spelled with a small
// drive letter, while the engine and the continuation's own window spell the
// folder with a capital. The old window is handed over all the same: it
// closes, the continuation's window stays, and the pinned button is the
// continuation's. The two spellings used to leave the old window open and
// empty, and its pinned button without its window (Paul, 12 November 2026).
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    app: () => {
      window.__calls = [];
      const capital = 'C:' + String.fromCharCode(92) + 'Test';
      return {
        State: async () => ({ workspace: capital, recent: [capital], defaultProvider: 'claude', catalog: [], defaults: {}, pinned: [] }),
        AgentProviders: async () => [{ id: 'claude', installed: true, available: true }],
        ResolveSessionContinuation: async id => id,
        SessionContinuation: async () => ({ history: [] }),
        SavedConversation: async id => ({ available: true, complete: true, streamGen: id === 'claude:head' ? 'h1' : undefined, streamThrough: 0, messages: [{ role: 'you', text: 'Question for ' + id }, { role: 'agent', text: 'Answer for ' + id }] }),
        // The engine opens the newest continuation, and spells the folder its own way.
        OpenSessionAt: async (ws, id) => { window.__calls.push(['open', id, ws]); return { sessionId: 'claude:head', provider: 'claude', title: 'Head Chat', cwd: capital, open: true, resumed: true, streamGen: 'h1', streamHead: 0 }; },
        ChatEvents: async (id, gen) => ({ gen, head: 0, truncated: false, reset: false, events: [] })
      };
    },
  });
  const { page, ok } = t;
  try {
    const BS = String.fromCharCode(92), small = 'c:' + BS + 'Test', capital = 'C:' + BS + 'Test';
    const chats = () => page.evaluate(() => window.dinahOpenWindows().filter(w => w.kind === 'chat-session').map(w => { const i = dinahWorkbench.item(w.key) || {}; return i.workspace + ' ' + i.path; }));
    // The continuation's own window, as the layout restores it.
    const headKey = await page.evaluate(ws => dinahWorkbench.open({ kind: 'chat-session', workspace: ws, path: 'claude:head' }), capital);
    await page.locator('.win[data-win="' + headKey + '"]').getByText('Answer for claude:head').first().waitFor({ state: 'attached' });
    ok('the same chat asked for with the other spelling is the same window', await page.evaluate(([ws, key]) => dinahWorkbench.key({ kind: 'chat-session', workspace: ws, path: 'claude:head' }) === key, [small, headKey]));
    // The old session's window, pinned, with the folder spelled the other way.
    const oldKey = await page.evaluate(async ws => {
      const key = await dinahWorkbench.open({ kind: 'chat-session', workspace: ws, path: 'codex:old' });
      await pudlWindows.refreshTaskbarDestinations();
      pudlWindows.pinTaskbar(key, true);
      return key;
    }, small);
    await page.waitForFunction(() => __calls.some(c => c[1] === 'codex:old'));
    await page.waitForFunction(key => !pudlWindows.state().open.includes(key), oldKey, { timeout: 8000 }).catch(() => {});
    const open = await chats();
    ok('the old session\'s window closes and the continuation\'s stays', open.length === 1 && open[0] === capital + ' claude:head', JSON.stringify(open));
    const pins = await page.evaluate(() => pudlWindows.taskbarState().pins);
    ok('the pinned button is the continuation\'s', pins.includes(headKey) && !pins.includes(oldKey), JSON.stringify({ pins, headKey, oldKey }));
    await page.locator('.win[data-win="' + headKey + '"]').getByText('Answer for claude:head').first().waitFor({ state: 'attached' });
    ok('and the continuation\'s window still shows its conversation', true);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
