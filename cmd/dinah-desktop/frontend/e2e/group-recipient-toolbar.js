// Recipient actions stay in the top toolbar without shifting the composer.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' };
      window.__calls = [];
      const room = { name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [docs, person] };
      window.__lead = value => { room.lead = value ? docs.id : ''; __emit('bus:changed', {}); };
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'], promptSendKey: 'enter' }),
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: [room], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
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
    await box.waitFor();
    const send = room.getByRole('button', { name: 'Send', exact: true });
    const everyone = room.locator('.group-toolbar').getByRole('button', { name: 'Mention Everyone', exact: true });
    const sends = () => p.evaluate(() => __calls.filter(c => c[0] === 'group-send'));
    const stable = async label => {
      await box.fill(''); const actionsBefore = await room.locator('.composer-actions').boundingBox();
      await box.fill('Draft'); const before = await room.locator('.group-composer').boundingBox();
      await box.fill('Draft text'); const after = await room.locator('.group-composer').boundingBox();
      const actionsAfter = await room.locator('.composer-actions').boundingBox();
      ok(label + ' keeps bottom actions stationary from empty to typed text', Math.abs(actionsBefore.y-actionsAfter.y)<1 && Math.abs(actionsBefore.height-actionsAfter.height)<1, JSON.stringify({actionsBefore,actionsAfter}));
      ok(label + ' typing causes no composer shift', Math.abs(before.y-after.y)<1 && Math.abs(before.height-after.height)<1, JSON.stringify({before,after}));
      ok(label + ' has no bottom routing panel', await room.locator('.group-notice-note, .group-routing-summary, .group-routing-details').count() === 0);
    };
    await stable('group'); await send.click(); await p.waitForFunction(() => __calls.length === 1);
    ok('plain group text never broadens recipients', JSON.stringify((await sends())[0][3]) === '[]');
    await p.evaluate(() => __lead(true)); await stable('team group');
    await box.press('Enter'); await p.waitForFunction(() => __calls.length === 2);
    ok('plain team text preserves server lead routing', JSON.stringify((await sends())[1][3]) === '[]');
    await box.fill('@Docs'); await box.press('End'); await room.locator('.mention-row', { hasText: 'Docs Pass' }).first().click();
    await box.pressSequentially('Review this'); const draft = await box.inputValue(); const key = await room.getAttribute('data-win');
    await p.evaluate(key => pudlWindows.close(key), key); await p.evaluate(() => dinahBus.openGroup('release-review')); await box.waitFor();
    ok('reopening preserves the explicit recipient draft', await box.inputValue() === draft);
    await send.click(); await p.waitForFunction(() => __calls.length === 3);
    ok('pointer mention completion sends only the selected chat', JSON.stringify((await sends())[2][3]) === '["window-d1"]');
    await box.fill('Report for everyone'); await everyone.focus(); await everyone.press('Enter');
    ok('keyboard toolbar activation explicitly adds everyone', (await box.inputValue()).includes('@everyone'));
    await box.press('Enter'); await p.waitForFunction(() => __calls.length === 4);
    ok('only explicit everyone selection requests everyone', JSON.stringify((await sends())[3][3]) === '["everyone"]');
    await box.fill('@Docs'); await box.press('End'); await room.locator('.mention-row', { hasText: 'Docs Pass' }).first().waitFor();
    await box.press('Enter'); await box.pressSequentially('Keyboard recipient'); await box.press('Enter'); await p.waitForFunction(() => __calls.length === 5);
    ok('keyboard mention completion preserves its recipient', JSON.stringify((await sends())[4][3]) === '["window-d1"]');
    await box.fill('Recipients removed'); await send.click(); await p.waitForFunction(() => __calls.length === 6);
    ok('replacing mentioned text removes its explicit recipients', JSON.stringify((await sends())[5][3]) === '[]');
    await box.fill('Visible draft'); await everyone.click();
    ok('pointer toolbar action retains Attach and Send', await room.locator('.group-composer').getByRole('button', { name: /Attach/ }).isVisible() && await send.isVisible());
    await t.shot('group-recipient-toolbar');
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
