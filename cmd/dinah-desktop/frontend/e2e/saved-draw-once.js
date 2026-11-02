// A chat's saved copy is drawn by one draw at a time (chat-applet.js,
// drawSaved). A second draw begun while the first is still filling the log
// stops the first, so the log holds the second draw's rows once and none of
// the first's, and whoever waited on the first draw goes on only once the
// second has drawn the chat's copy: the restore path then starts its
// journal catch-up or reconnect against a finished copy.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspace: 'C:/Test', recent: ['C:/Test'] },
    app: () => ({
      AgentProviders: async () => [{ id: 'claude', name: 'Claude', installed: true, available: true, status: 'detected' }],
      SavedConversation: async id => id === 'claude:a' ? { available: true, complete: true, messages: [{ role: 'you', text: 'Opening copy.' }] } : { available: false },
      ChatEvents: async (id, gen) => ({ gen, head: 0, truncated: false, reset: false, events: [] }),
      ResolveSessionContinuation: async id => id
    }),
    // The lifecycle's adapter holds drawSaved and the session store; the
    // suite keeps the one chat-applet makes, without a hook in the page.
    init: () => {
      let real;
      Object.defineProperty(window, 'dinahSessionLifecycle', { configurable: true, get: () => real, set: value => {
        real = { create: adapter => { window.__lifecycle = adapter; return value.create(adapter); } };
      } });
    },
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => __emit('agent:session', { sessionId: 'claude:a', provider: 'claude', cwd: 'C:/Test', title: 'Chat a', open: false }));
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:a' }));
    await p.locator('.win[data-win-kind="chat-session"]').getByText('Opening copy.', { exact: true }).waitFor();
    const result = await p.evaluate(async () => {
      const life = window.__lifecycle, s = life.store.get('claude:a'), seen = {};
      const many = (word, n) => Array.from({ length: n }, (_, i) => ({ role: 'you', text: word + ' ' + i + '.' }));
      const rows = word => [...s.log.querySelectorAll('.msg-you')].filter(r => r.textContent.includes(word)).length;
      // Enough messages that the first draw spans frames, so the second
      // begins while the first is still filling the log.
      const first = life.drawSaved(s, many('First', 60)).then(() => { seen.first = { second: rows('Second'), first: rows('First') }; });
      const second = life.drawSaved(s, many('Second', 3)).then(() => { seen.second = { second: rows('Second'), first: rows('First') }; });
      await Promise.all([first, second]);
      return seen;
    });
    ok('a second draw stops the first: the log holds the second draw\'s rows once and none of the first\'s', result.second.second === 1 && result.second.first === 0, JSON.stringify(result));
    ok('whoever waited on the stopped draw goes on only once the second has drawn the copy', result.first && result.first.second === 1 && result.first.first === 0, JSON.stringify(result));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
