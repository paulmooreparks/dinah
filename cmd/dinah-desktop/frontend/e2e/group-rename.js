// Rename Group… in a group window's commands (bus-applets.js, Paul's message
// 935): the rules of a chat's rename — the engine keeps members, messages,
// budget and unread marks, and the window takes the new title.
const { launch } = require('./lib/page');

(async () => {
  /* The room's shape lives INSIDE the app fixture: it goes to the page as
     source, and nothing of the suite's own scope reaches there. */
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    app: () => {
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' };
      const rooms = [{ name: 'release-review', title: 'Release Review', lastAt: '2026-10-22T09:00:00Z', unread: 1, latest: { id: 9, from: 'Docs Pass', line: 'The guide is ready' }, members: [docs, person] }];
      window.__calls = [];
      window.__rooms = rooms;
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'] }),
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: window.__rooms, workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusGroupMessages: async () => [],
        BusMarkGroupRead: async () => {},
        BusRenameGroup: async (key, title) => {
          window.__calls.push(['rename', key, title]);
          if (window.__renameFails) throw new Error('another group already has that title');
          window.__rooms.find(r => r.name === key).title = title;
          return window.__rooms.find(r => r.name === key);
        },
      };
    },
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    const room = p.locator('.win[data-win-kind="bus-room"]');
    await room.locator('textarea').waitFor();
    await room.locator('.member-chip').first().waitFor();
    /* The room window's commands answer from its Actions menu; the bar's own
       focus discipline at the last hop is PUDL's domain, so the suite presses
       the row's command as the bar's act() would, with the panel open and
       visible as checkable proof that a reader finds it there. */
    async function clickRename() {
      await p.getByRole('menuitem', { name: 'Actions' }).click();
      await p.waitForFunction(() => [...document.querySelectorAll('.menu-action')].some(b => /Rename Group/.test(b.textContent) && (b.offsetWidth || b.offsetHeight)));
      await p.evaluate(() => [...document.querySelectorAll('.menu-action')].find(b => /Rename Group/.test(b.textContent) && (b.offsetWidth || b.offsetHeight))._item.run());
    }
    // The window's own commands sit under Actions while it is in front.
    const dlg = p.locator('dialog.dinah-ask-name');
    await clickRename();
    await dlg.waitFor();
    await t.shot('group-rename-dialog');
    ok('the rename asks with the group’s title as its start', (await dlg.locator('.dialog-title').textContent()) === 'Rename the Group' && (await dlg.locator('input').inputValue()) === 'Release Review');
    await dlg.locator('input').fill('Dinah Development');
    await dlg.getByRole('button', { name: 'Rename' }).click();
    await p.waitForFunction(() => window.__calls.length === 1);
    ok('the engine is asked to rename it', JSON.stringify(await p.evaluate(() => window.__calls)) === JSON.stringify([['rename', 'release-review', 'Dinah Development']]));
    await p.waitForFunction(() => /Dinah Development/.test(document.querySelector('.win[data-win-kind="bus-room"] .win-title').textContent));
    ok('the window takes the new title', true);
    await p.waitForFunction(() => [...document.querySelectorAll('.menubar-title')].some(b => /Dinah Development/.test(b.textContent)));
    ok('the menu bar’s copy of the title follows it', true);
    await t.shot('group-rename-result');
    // Esc leaves it as it was.
    await clickRename();
    await dlg.waitFor();
    await dlg.locator('input').fill('A Cast-Off Name');
    await p.keyboard.press('Escape');
    await dlg.waitFor({ state: 'detached' });
    ok('Escape leaves the title as it was', (await p.evaluate(() => window.__calls.length)) === 1);
    // A refusal is said in the dialog, which stays open.
    await clickRename();
    await dlg.waitFor();
    await p.evaluate(() => { window.__renameFails = true; });
    await dlg.locator('input').fill('Dinah Cleanup');
    await dlg.getByRole('button', { name: 'Rename' }).click();
    await dlg.locator('.form-error').waitFor();
    await t.shot('group-rename-refusal');
    ok('a refusal is said in the dialog, which stays open', /another group already has that title/i.test(await dlg.locator('.form-error').textContent()) && await dlg.isVisible());
    await p.keyboard.press('Escape');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
