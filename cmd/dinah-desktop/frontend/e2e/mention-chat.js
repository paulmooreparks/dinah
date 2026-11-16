const { launch } = require('./lib/page');
(async () => {
  for (const theme of ['light', 'dark']) for (const width of [1440, 390]) {
    const t = await launch({ viewport: { width, height: 900 }, app: () => {
      const room = { name: 'harbor-review', title: 'Review', members: [] };
      window.__mentions = []; window.__acks = []; window.__holdMessages = false;
      window.__messages = Array.from({ length: 35 }, (_, n) => ({ id: n + 1, room: room.name, from: 'Reviewer', fromWorkspace: 'Harbor', fromPerson: false, text: 'Review message ' + (n + 1), createdAt: '2026-11-16T00:00:00Z', attachments: [], mentions: [] }));
      return {
        Notifications: async () => __mentions,
        BusOverview: async () => ({ rooms: [room], workspaces: [], links: [], inbox: [], unread: 0 }),
        BusGroupMessages: async () => __messages.slice(-10),
        BusGroupMessagesFrom: async () => { if (__holdMessages) await new Promise(resolve => window.__releaseMessages = resolve); return __messages; },
        BusThread: async id => __messages.filter(m => m.id === id),
        BusMarkGroupRead: async () => { window.__bulkReads = (window.__bulkReads || 0) + 1; },
        AcknowledgeMention: async id => {
          const row = document.querySelector('.bus-target-message[data-bus-message="' + id + '"]');
          const log = row && row.closest('.bus-log');
          const r = row && row.getBoundingClientRect(), b = log && log.getBoundingClientRect();
          __acks.push({ id, marked: !!row, visible: !!(r && b && r.bottom > b.top && r.top < b.bottom) });
          __mentions = __mentions.filter(n => n.message !== id);
        }
      };
    } });
    const { page: p, ok } = t;
    try {
      await p.evaluate(theme => document.documentElement.dataset.theme = theme, theme);
      await p.evaluate(() => {
        __mentions = [4, 7].map(message => ({ id: 'mention-' + message, kind: 'mention', title: 'Reviewer mentioned you', body: 'Review message ' + message, sessionId: 'reviewer', sessionTitle: 'Reviewer', group: 'harbor-review', groupTitle: 'Review', message, created: '2026-11-16T00:00:00Z' }));
        __mentions.forEach(n => __emit('notifications:added', n));
        return dinahWorkbench.open('needs-you');
      });
      const card = p.locator('#questions .attention-kept');
      await card.getByRole('button', { name: 'Open in Chat', exact: true }).waitFor();
      await p.evaluate(() => { __holdMessages = true; });
      await card.getByRole('button', { name: 'Open in Chat', exact: true }).click();
      await p.waitForFunction(() => !!window.__releaseMessages);
      ok('opening a chat before arrival keeps mentions pending ' + theme + width, await p.evaluate(() => __acks.length === 0 && __mentions.length === 2));
      await p.evaluate(() => { __holdMessages = false; __releaseMessages(); });
      await p.waitForFunction(() => __acks.length === 1);
      ok('acknowledgement follows the exact marked visible target', await p.evaluate(() => __acks[0].id === 7 && __acks[0].marked && __acks[0].visible && __mentions.length === 1 && __mentions[0].message === 4));
      ok('opening a mention does not bulk-read its group', await p.evaluate(() => !window.__bulkReads));
      await p.evaluate(() => dinahWorkbench.open('needs-you'));
      await card.getByRole('button', { name: 'Open in Chat', exact: true }).click();
      await p.waitForFunction(() => __acks.length === 2);
      ok('an existing chat is raised and retargeted without duplication', await p.locator('.group-window').count() === 1 && await p.evaluate(() => __acks[1].id === 4 && __acks[1].visible));
      await t.shot('mention-chat-' + theme + '-' + width);
    } finally { await t.close(); }
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
