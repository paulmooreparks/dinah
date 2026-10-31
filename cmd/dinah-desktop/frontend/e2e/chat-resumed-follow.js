// A chat opened behind its drawn saved copy, by resuming, keeps following its end as new output
// arrives: only a scroll by the reader stops that.
const { launch } = require('./lib/page');

(async () => {
  const long = n => 'Message ' + n + '.\n\n' + Array.from({ length: 10 }, (_, i) => 'Line ' + i + ' of message ' + n + '.').join('\n\n');
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    app: () => {
      window.__calls = [];
      const messages = [];
      for (let i = 1; i <= 8; i++) { messages.push({ role: 'you', text: 'Question ' + i }); messages.push({ role: 'agent', text: 'Answer ' + i + '.\n\n' + Array.from({ length: 8 }, (_, k) => 'Line ' + k + ' of answer ' + i + '.').join('\n\n') }); }
      let release; window.__gate = new Promise(r => { release = r; }); window.__release = release;
      return {
        State: async () => ({ workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider: 'claude', catalog: [], defaults: {}, pinned: [] }),
        AgentProviders: async () => [{ id: 'claude', installed: true, available: true }],
        ResolveSessionContinuation: async id => id,
        SessionContinuation: async () => ({ history: [] }),
        SavedConversation: async () => ({ available: true, complete: true, streamGen: 'g1', streamThrough: 0, messages }),
        OpenSessionAt: async (ws, id) => {
          await window.__gate;
          return { sessionId: id, provider: 'claude', title: 'Follow Chat', cwd: ws, open: true, resumed: true, streamGen: 'g1', streamHead: 0 };
        },
        ChatEvents: async (id, gen) => ({ gen, head: 0, truncated: false, reset: false, events: [] })
      };
    },
  });
  const { page, ok } = t;
  const live = (n, text) => page.evaluate(([n, text]) => __emit('agent:update', { sessionId: 'claude:follow', gen: 'g1', n, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } } }), [n, text]);
  try {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:follow' }));
    const win = page.locator('[data-win="' + key + '"]');
    await win.getByText('Answer 8.').first().waitFor({ state: 'attached' });
    const gap = () => win.locator('[data-view-id="activity"], #activity').first().evaluate(el => el.scrollHeight - el.scrollTop - el.clientHeight);
    await page.waitForTimeout(600);
    ok('the drawn copy opens at its end', await gap() < 24, String(await gap()));
    // The agent finishes resuming while the copy is in sight.
    await page.evaluate(() => window.__release());
    await page.waitForFunction(() => document.querySelector('.win .chat-loading-chip[hidden]') !== null || !document.querySelector('.win .chat-loading-chip:not([hidden])'));
    await page.waitForTimeout(500);
    ok('after the chat resumes the view is still at the end', await gap() < 24, String(await gap()));
    // New output arrives, one long piece after another.
    for (let n = 1; n <= 4; n++) { await live(n, ' ' + long(n) + ' '); await page.waitForTimeout(250); }
    await win.getByText('Message 4.').first().waitFor({ state: 'attached' });
    await page.waitForTimeout(400);
    ok('new output keeps the view at the end', await gap() < 24, String(await gap()));
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
