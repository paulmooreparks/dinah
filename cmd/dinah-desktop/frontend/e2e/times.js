// A reopened session shows the times its messages were said, from the
// journal, not the time it was reopened; a session with no journal shows none.
const { launch } = require('./lib/page');
(async () => {
  const ws = 'C:\\W';
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    state: { workspace: ws, recent: [ws], approvals: 'devin', devin: 'd', titles: {}, templates: [] },
    app: () => {
      const ws = 'C:\\W';
      const up = (id, u) => window.__emit('agent:update', { sessionId: id, update: u });
      return {
        HostLocale: async () => 'en-GB-u-hc-h23',
        Sessions: async () => [{ sessionId: 'old', cwd: ws, title: 'Planned yesterday', updatedAt: '2026-10-07T21:00:00Z' }, { sessionId: 'desktop', cwd: ws, title: 'From Devin Desktop', updatedAt: '2026-10-07T20:00:00Z' }],
        OpenSessionAt: async (workspace, id) => {
          up(id, { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'First question' } });
          up(id, { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'First answer' } });
          up(id, { sessionUpdate: 'tool_call', toolCallId: id + 't', title: 'Read a file', status: 'completed' });
          up(id, { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Second part of the answer' } });
          up(id, { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Second question' } });
          up(id, { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Second answer' } });
          return { sessionId: id, cwd: ws, title: '', open: true, working: false, queued: [] };
        },
        SessionTimes: async (id) => id === 'old' ? [
          { kind: 'you', at: '2026-10-07T09:05:00' }, { kind: 'agent', at: '2026-10-07T09:06:00' }, { kind: 'agent', at: '2026-10-07T09:08:00' },
          { kind: 'you', at: '2026-10-07T10:15:00' }, { kind: 'agent', at: '2026-10-07T10:16:00' }] : [],
      };
    },
  });
  const p = require('./lib/windows')(t.page);
  const { ok } = t;
  try {
    await p.waitForTimeout(400);
    await p.click('#sessions a:has-text("Planned yesterday")');
    await p.waitForTimeout(500);
    const times = await p.$$eval('.win[data-chat-current] [data-view-id=logs] ol:not([hidden]) > li > time.activity-time', ts => ts.map(t => t.textContent));
    ok('replayed messages show when they were said', times.join(' ') === '09:05 09:06 09:06 09:08 10:15 10:16', times.join(' '));
    ok('the full date is on the time', (await p.getAttribute('.win[data-chat-current] [data-view-id=logs] ol:not([hidden]) > li > time.activity-time', 'title')).includes('2026'));
    await p.click('#sessions a:has-text("From Devin Desktop")');
    await p.waitForTimeout(500);
    const none = await p.$$eval('.win[data-chat-current] [data-view-id=logs] ol:not([hidden]) > li > time.activity-time', ts => ts.map(t => t.textContent));
    ok('a session with no journal shows no time rather than a wrong one', none.every(t => t === ''), none.join(' '));
  } finally { await t.close(); }
})();
