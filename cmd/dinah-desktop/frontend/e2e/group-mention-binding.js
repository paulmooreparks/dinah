// A group message asks the chats its text mentions to answer. A mention
// chosen from the @ list binds, by Enter, Tab or a click, and so does one
// the person typed or pasted whole without choosing it: "@everyone" or "@"
// and a member's title, at the start of a word. Nothing the text says with
// an @ is sent unbound, so no member is silently left asleep (Paul, 2
// November 2026: a typed "@everyone" woke nobody).
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    app: () => {
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } };
      const review = { id: 'window-r1', chat: 'chat-review', title: 'Review', workspace: 'docs', address: 'Review (docs)', life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } };
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      let next = 100;
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'] }),
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [docs, review, person], counts: { top: 'idle', idle: 2, working: 0, waiting: 0, stuck: 0, closed: 0, unknown: 0 } }], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusGroupMessages: async () => [], BusMarkGroupRead: async () => {},
        BusSendToGroup: async (key, text, mentions) => { window.__sent.push({ text, mentions }); return { id: next++, from: 'Paul', fromPerson: true, kind: 'text', text, mentions: [], attachments: [], createdAt: new Date().toISOString(), room: key }; }
      };
    },
    init: () => { window.__sent = []; },
  });
  const { page: p, ok } = t;
  try {
    await p.locator('[data-win="workspace"]').waitFor({ state: 'attached' });
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    const room = p.locator('.win[data-win-kind="bus-room"]');
    const box = room.locator('.group-composer textarea'), list = room.locator('.group-mention-pop');
    await box.waitFor();
    const sendAs = async (how, run) => {
      const before = await p.evaluate(() => __sent.length);
      await box.fill('');
      await run();
      await box.press('Control+Enter');
      await p.waitForFunction(n => __sent.length > n, before);
      const sent = await p.evaluate(() => __sent[__sent.length - 1]);
      return Object.assign({ how }, sent);
    };
    const pick = key => async () => { await box.pressSequentially('@every'); await list.locator('[role="option"]').first().waitFor(); await box.press(key); await box.pressSequentially('please answer'); };
    const cases = [
      await sendAs('chosen with Enter', pick('Enter')),
      await sendAs('chosen with Tab', pick('Tab')),
      await sendAs('chosen with a click', async () => { await box.pressSequentially('@every'); await list.locator('[data-mention="everyone"] a').click(); await box.pressSequentially('please answer'); }),
      await sendAs('typed whole', async () => { await box.pressSequentially('@everyone please answer'); }),
      await sendAs('pasted', async () => { await box.fill('@everyone please answer'); }),
    ];
    cases.forEach(c => ok('"@everyone" ' + c.how + ' asks every member', JSON.stringify(c.mentions) === JSON.stringify(['everyone']), JSON.stringify(c)));
    const member = await sendAs('typed title', async () => { await box.pressSequentially('@Docs Pass and @Review, please look.'); });
    ok('a member\'s title typed whole, spaces and all, asks that member, as does a second', JSON.stringify((member.mentions || []).slice().sort()) === JSON.stringify(['window-d1', 'window-r1']), JSON.stringify(member));
    const inside = await sendAs('not at a word start', async () => { await box.fill('mail docs@everyone.example and @Reviewer'); });
    ok('an @ inside a word, or a title that runs on into longer text, asks no one', JSON.stringify(inside.mentions) === '[]', JSON.stringify(inside));
    const bracketed = await sendAs('after punctuation', async () => { await box.fill('(@everyone) please'); });
    ok('a mention after punctuation, as in "(@everyone)", asks every member', JSON.stringify(bracketed.mentions) === JSON.stringify(['everyone']), JSON.stringify(bracketed));
    const quoted = await sendAs('in quotation marks', async () => { await box.fill('She wrote "@everyone, look" and ‘@Review’ replied.'); });
    ok('a quoted "@everyone" and a quoted title bind', JSON.stringify((quoted.mentions || []).slice().sort()) === JSON.stringify(['everyone', 'window-r1']), JSON.stringify(quoted));
    // A letter that grows when lowercased, before the mention, does not
    // shift where its edges are read.
    const grown = await sendAs('after a letter that grows', async () => { await box.fill('İİİ @everyone please, and İİ@Review'); });
    ok('a letter that changes length when lowercased does not move a mention\'s edges', JSON.stringify(grown.mentions) === JSON.stringify(['everyone']), JSON.stringify(grown));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
