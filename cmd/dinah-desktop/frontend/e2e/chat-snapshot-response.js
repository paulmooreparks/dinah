// A final response arriving during snapshot rendering must reach the new log.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    app: () => ({
      AgentProviders: async () => [{ id: 'codex', installed: true, available: true }],
      ResolveSessionContinuation: async id => id,
      SessionContinuation: async () => ({ history: [] }),
      SavedConversation: async () => ({ available: true, streamGen: 'snapshot', streamThrough: 0,
        messages: [{ role: 'you', text: 'The final question.' }] }),
      OpenSessionAt: async (cwd, sessionId) => ({ sessionId, cwd, provider: 'codex', open: true, streamGen: 'snapshot' }),
      ChatEvents: async () => {
        window.__journalReads = (window.__journalReads || 0) + 1;
        return { gen: 'snapshot', head: 1, events: [{ n: 1, name: 'agent:update', data: { update: {
          sessionUpdate: 'agent_message_chunk', turnId: 'turn', itemId: 'answer', contentMode: 'snapshot',
          content: { type: 'text', text: 'The final response survives replacement.' }
        } } }] };
      }
    }),
    init: () => {
      let lifecycle;
      Object.defineProperty(window, 'dinahSessionLifecycle', { get: () => lifecycle, set: value => {
        lifecycle = { create: adapter => {
          const draw = adapter.drawSavedCopy;
          adapter.drawSavedCopy = async (owner, saved) => {
            const copy = await draw(owner, saved);
            if (owner.checkpointDraw) {
              window.__snapshotOwner = owner;
              await new Promise(resolve => { window.__releaseSnapshot = resolve; });
            }
            return copy;
          };
          return value.create(adapter);
        } };
      } });
    }
  });
  try {
    const p = t.page;
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:snapshot' }));
    await p.waitForFunction(() => !!window.__releaseSnapshot);
    await p.evaluate(() => __resync());
    // Allow the resync to reach the journal while snapshot adoption is held.
    await p.waitForTimeout(300);
    await p.evaluate(() => __releaseSnapshot());
    await p.waitForFunction(() => !__snapshotOwner.loading && !__snapshotOwner.settling && !__snapshotOwner.checkpointDraw);
    const answer = p.locator('.win[data-win-kind="chat-session"] .msg-row', { hasText: 'The final response survives replacement.' });
    await answer.waitFor({ timeout: 5000 }).catch(() => {});
    t.ok('the final response remains in the adopted transcript', await answer.count() === 1);
    t.ok('the journal was read after the snapshot became ready', await p.evaluate(() => __journalReads > 0));
    await t.shot('chat-snapshot-response');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
