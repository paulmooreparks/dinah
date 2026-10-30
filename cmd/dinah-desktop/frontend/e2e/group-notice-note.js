// A group message that mentions nobody reaches the members only as a notice, and the box says so while that is
// what it would do, offering @everyone as one press (docs/plans/group-attention.md).
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' };
      window.__calls = [];
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'], promptSendKey: 'enter' }),
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [docs, person] }], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusGroupMessages: async () => [],
        BusMarkGroupRead: async () => {},
        BusSendToGroup: async (key, text, mentions) => { __calls.push(['group-send', key, text, mentions]); return { id: 1 }; }
      };
    },
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    const room = p.locator('.win[data-win-kind="bus-room"]');
    const box = room.locator('textarea');
    const note = room.locator('.group-notice-note');
    await box.waitFor();
    ok('with nothing typed the box says nothing about notices', await note.isHidden());
    await box.fill('Status for everybody');
    await note.waitFor();
    ok('the compact hint says no replies are requested', /No replies requested/.test(await note.innerText()));
    await note.getByText('Who receives this?', { exact: true }).click();
    ok('the disclosure explains how to request replies', /Each member receives a notice/.test(await note.innerText()) && /Mention a chat/.test(await note.innerText()));
    await t.shot('group-notice-explanation');
    await box.fill('Docs Pass please look');
    await note.waitFor();
    await box.pressSequentially(' @Docs');
    await room.locator('.mention-row', { hasText: 'Docs Pass' }).first().click();
    await note.waitFor({ state: 'hidden' });
    ok('a message that mentions a chat does not show it', await note.isHidden());
    await box.fill('Status for everybody');
    await note.waitFor();
    await note.getByRole('button', { name: 'Mention Everyone' }).click();
    ok('Mention Everyone puts @everyone in the box and the note goes', /^@everyone Status for everybody/.test(await box.inputValue()) && await note.isHidden());
    await box.press('Enter');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'group-send'));
    ok('the message goes with everyone as its mention', JSON.stringify(await p.evaluate(() => __calls.filter(c => c[0] === 'group-send')[0].slice(3))) === JSON.stringify([['everyone']]), JSON.stringify(await p.evaluate(() => __calls)));
    await t.shot('group-notice-note');
    ok('and after it is sent the box is empty with no note', (await box.inputValue()) === '' && await note.isHidden());
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
