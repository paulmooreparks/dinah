// A plain mention whose message the person can see makes no card, and one
// already carded ends once its message is on screen in the conversation in
// front while the program's window is active. A request keeps its card.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 900 }, app: () => {
    const room = { name: 'harbor-review', title: 'Review', members: [] };
    window.__mentions = []; window.__acks = [];
    window.__messages = Array.from({ length: 10 }, (_, n) => ({ id: n + 1, room: room.name, from: 'Reviewer', fromWorkspace: 'Harbor', fromPerson: false, text: 'Review message ' + (n + 1), createdAt: '2026-11-25T00:00:00Z', attachments: [], mentions: [] }));
    return {
      Notifications: async () => __mentions,
      BusOverview: async () => ({ rooms: [room], workspaces: [], links: [], inbox: [], unread: 0 }),
      BusGroupMessages: async () => __messages.slice(),
      BusGroupMessagesFrom: async () => __messages.slice(),
      BusMarkGroupRead: async () => {},
      AcknowledgeMention: async id => { __acks.push(id); __mentions = __mentions.filter(n => n.message !== id); }
    };
  }, init: () => { document.hasFocus = () => window.__focused !== false; } });
  const { page: p, ok } = t;
  const frames = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
  const settle = async () => { await p.waitForTimeout(1300); await frames(); };
  try {
    // Every card ever drawn is recorded, so a card shown and then taken away is caught.
    await p.evaluate(() => {
      window.__drawn = [];
      const add = dinahHost.notices.add.bind(dinahHost.notices);
      dinahHost.notices.add = e => { if (e && e.data && e.data.message) __drawn.push(e.data.message); return add(e); };
    });
    const mention = (message, extra) => Object.assign({ id: 'mention-' + message, kind: 'mention', title: 'Reviewer mentioned you in Review', body: 'Review message ' + message, sessionId: 'reviewer', sessionTitle: 'Reviewer', group: 'harbor-review', groupTitle: 'Review', message, created: '2026-11-25T00:00:00Z' }, extra || {});
    const arrive = (message, cardFirst, extra) => p.evaluate(([m, first, n]) => {
      const msg = { id: m, room: 'harbor-review', from: 'Reviewer', fromWorkspace: 'Harbor', fromPerson: false, text: 'Review message ' + m, createdAt: '2026-11-25T00:00:00Z', attachments: [], mentions: [] };
      const card = n; __mentions.push(card);
      if (first) { __emit('notifications:added', card); __messages.push(msg); __emit('bus:changed', {}); }
      else { __messages.push(msg); __emit('bus:changed', {}); setTimeout(() => __emit('notifications:added', card), 50); }
    }, [message, cardFirst, mention(message, extra)]);
    const cards = () => p.evaluate(() => dinahHost.notices.list(n => n.data && n.data.attentionChat === 'reviewer').length);

    await p.evaluate(() => dinahBus.openGroup('harbor-review'));
    await p.waitForSelector('.group-window [data-bus-message="10"]');
    await settle();

    await arrive(11, false); await settle();
    ok('a mention drawn before its card arrives makes no card', await p.evaluate(() => __acks.includes(11) && !__drawn.includes(11)), await p.evaluate(() => ({ acks: __acks, drawn: __drawn })));
    await arrive(12, true); await settle();
    ok('a mention whose card arrives first is never shown', await p.evaluate(() => __acks.includes(12) && !__drawn.includes(12)), await p.evaluate(() => ({ acks: __acks, drawn: __drawn })));
    ok('no card is left for the chat', await cards() === 0);

    await arrive(13, true, { id: 'request-13', requestId: 'request:13', title: 'Reviewer asks you' }); await settle();
    ok('a request in the conversation in front keeps its card', await p.evaluate(() => !__acks.includes(13) && dinahHost.notices.list(n => n.data && n.data.message === 13).length === 1));

    await p.evaluate(() => { window.__focused = false; });
    await arrive(14, true); await settle();
    ok('a mention arriving while the window is not active is carded', await p.evaluate(() => !__acks.includes(14) && __drawn.includes(14)));
    await p.evaluate(() => { window.__focused = true; window.dispatchEvent(new Event('focus')); });
    await frames();
    await p.waitForFunction(() => __acks.includes(14), null, { timeout: 3000 }).catch(() => {});
    ok('the card ends when the window becomes active with the message on screen', await p.evaluate(() => __acks.includes(14) && dinahHost.notices.list(n => n.data && n.data.message === 14).length === 0));

    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await settle();
    await arrive(15, true); await settle();
    ok('a mention arriving while another window is in front is carded', await p.evaluate(() => !__acks.includes(15) && __drawn.includes(15)));
    await p.evaluate(() => dinahBus.openGroup('harbor-review'));
    await p.waitForFunction(() => __acks.includes(15), null, { timeout: 3000 }).catch(() => {});
    ok('raising the conversation with the message on screen ends the card', await p.evaluate(() => __acks.includes(15) && dinahHost.notices.list(n => n.data && n.data.message === 15).length === 0));
    ok('the request card is still there after all of that', await p.evaluate(() => dinahHost.notices.list(n => n.data && n.data.message === 13).length === 1));
    ok('the chat counts only the request', await cards() === 1);
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
