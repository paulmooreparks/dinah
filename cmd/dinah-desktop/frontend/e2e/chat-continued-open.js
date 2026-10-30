// A window restored for a chat that was continued in another chat (a provider switch) is
// handed over: the engine opens the newest continuation and returns its information with
// another id, and the window opened for the old chat closes while one for the continuation
// opens, so the layout then saves the continuation and the next start asks for it directly.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    app: () => {
      window.__calls = [];
      return {
        State: async () => ({ workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider: 'claude', catalog: [], defaults: {}, pinned: [] }),
        AgentProviders: async () => [{ id: 'claude', installed: true, available: true }],
        ResolveSessionContinuation: async id => id,
        SessionContinuation: async () => ({ history: [] }),
        SavedConversation: async id => ({ available: true, complete: true, streamGen: id === 'claude:head' ? 'h1' : undefined, streamThrough: 0, messages: [{ role: 'you', text: 'Question for ' + id }, { role: 'agent', text: 'Answer for ' + id }] }),
        OpenSessionAt: async (ws, id) => {
          window.__calls.push(['open', id]);
          // The engine resolves to the newest continuation and opens that.
          return { sessionId: 'claude:head', provider: 'claude', title: 'Head Chat', cwd: ws, open: true, resumed: true, streamGen: 'h1', streamHead: 0 };
        },
        ChatEvents: async (id, gen) => ({ gen, head: 0, truncated: false, reset: false, events: [] })
      };
    },
  });
  const { page, ok } = t;
  try {
    const paths = () => page.evaluate(() => window.dinahOpenWindows().filter(w => w.kind === 'chat-session').map(w => (dinahWorkbench.item(w.key) || {}).path));
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:old' }));
    await page.waitForFunction(() => __calls.some(c => c[0] === 'open'));
    const chatPaths = () => window.dinahOpenWindows().filter(w => w.kind === 'chat-session').map(w => (dinahWorkbench.item(w.key) || {}).path);
    await page.waitForFunction(`(${chatPaths})().includes('claude:head') && !(${chatPaths})().includes('codex:old')`);
    const open = await paths();
    ok('the window for the continuation is open and the one for the superseded chat is closed', open.includes('claude:head') && !open.includes('codex:old'), open.join(','));
    const layout = await page.evaluate(() => ({ saved: JSON.stringify(dinahUIState.get('dinah-workbench-layout') || ''), head: dinahWorkbench.key({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:head' }), old: dinahWorkbench.key({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:old' }) }));
    ok('the layout saves the continuation, so the next start asks for it directly', layout.saved.includes(layout.head) && !layout.saved.includes(layout.old), layout.saved);
    const head = page.locator('.win[data-win-kind="chat-session"]').first();
    await head.getByText('Answer for claude:head').first().waitFor({ state: 'attached' });
    ok('and the window shows the continuation\'s own saved copy', true);
    ok('the superseded chat was opened once, and the continuation not again', JSON.stringify(await page.evaluate(() => __calls.filter(c => c[0] === 'open'))) === JSON.stringify([['open', 'codex:old']]));
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
