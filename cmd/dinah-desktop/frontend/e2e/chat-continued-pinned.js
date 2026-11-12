// A chat continued under another session has a pinned window for its old
// session, and the continuation has a window of its own. When the engine
// says the old session was continued, the old window gives way: it closes,
// the continuation's window is the one left, and the pinned button is the
// continuation's. The same holds when the continuation's window is not
// listed as open at that moment, as a window of the layout is not while
// the start is still opening it: asking only for an open window left the
// old one half-followed, empty and refused its new name (Paul, 12 November
// 2026).
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
        ResolveSessionContinuation: async id => { if (id === 'codex:old') { if (!window.__released) await new Promise(r => { window.__go = r; }); return 'claude:head'; } return id; },
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
    const BS = String.fromCharCode(92), small = 'C:' + BS + 'Test', capital = 'C:' + BS + 'Test';
    const chats = () => page.evaluate(() => window.dinahOpenWindows().filter(w => w.kind === 'chat-session').map(w => { const i = dinahWorkbench.item(w.key) || {}; return i.workspace + ' ' + i.path; }));
    // The continuation's own window, as the layout restores it.
    const headKey = await page.evaluate(ws => dinahWorkbench.open({ kind: 'chat-session', workspace: ws, path: 'claude:head' }), capital);
    await page.locator('.win[data-win="' + headKey + '"]').getByText('Answer for claude:head').first().waitFor({ state: 'attached' });
    ok('the same chat asked for with the other spelling is the same window', await page.evaluate(([ws, key]) => dinahWorkbench.key({ kind: 'chat-session', workspace: ws, path: 'claude:head' }) === key, [small, headKey]));
    if (process.env.HEAD_CLOSED) await page.evaluate(key => { pudlWindows.close(key); window.__pending = true; }, headKey);
    // The old session's window, pinned, with the folder spelled the other way.
    const oldKey = await page.evaluate(async ws => {
      const key = await dinahWorkbench.open({ kind: 'chat-session', workspace: ws, path: 'codex:old' });
      await pudlWindows.refreshTaskbarDestinations();
      pudlWindows.pinTaskbar(key, true);
      // The old window waits for word of its continuation.
      for (let i = 0; i < 100 && !window.__go; i++) await new Promise(r => setTimeout(r, 50));
      window.__errors = [];
      window.addEventListener('unhandledrejection', e => __errors.push(String(e.reason && e.reason.message || e.reason)));
      window.addEventListener('error', e => __errors.push(String(e.message)));
      // The continuation's window is opening as the word comes.
      if (window.__pending) window.__opening = dinahWorkbench.open({ kind: 'chat-session', workspace: ws, path: 'claude:head' });
      window.__released = true; if (window.__go) window.__go();
      if (window.__opening) await window.__opening;
      return key;
    }, small);
    await page.waitForTimeout(1500);
    ok('nothing is refused on the way', (await page.evaluate(() => __errors)).length === 0, JSON.stringify(await page.evaluate(() => __errors)));
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
