// The Group Chats window's own commands, as its window menu and agents see
// them (PUDL's pudlApplets.commandsIn). New Group Chat, New Message and
// Trust a Link are always there. Mark All Read and Dismiss All are there
// when the inbox's own buttons are, run the same confirmed action, and are
// absent when there is nothing to mark. Building the list raises no error;
// an error there once left the window with no commands at all.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      window.__calls = [];
      const message = id => ({ id, kind: 'message', from: 'Release', fromWorkspace: 'studio', to: 'Paul', text: 'Install finished ' + id + '.', createdAt: new Date(Date.UTC(2026, 10, 1, 8, id)).toISOString(), inbox: true, unread: !window.__read, dismissed: false });
      const items = () => window.__read ? [] : [message(2), message(1)];
      return {
        BusOverview: async () => ({ inbox: items(), unread: items().length, rooms: [], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusInboxPage: async view => ({ items: view === 'unread' ? items() : [], unread: items().length, nextBefore: 0, newest: 2 }),
        BusMarkInboxReadThrough: async id => { window.__calls.push(['readThrough', id]); window.__read = true; },
        BusDismissInboxThrough: async id => { window.__calls.push(['dismissThrough', id]); },
      };
    },
  });
  const { page, ok } = t;
  try {
    await page.evaluate(() => {
      window.__errors = [];
      const log = console.error;
      console.error = (...args) => { window.__errors.push(args.map(String).join(' ')); log.apply(console, args); };
    });
    await page.evaluate(() => dinahWorkbench.open('bus'));
    await page.locator('[data-section="bus-inbox"] .bus-inbox-row').first().waitFor();
    const labels = () => page.evaluate(() => pudlApplets.commandsIn(document.querySelector('[data-win="bus"]')).map(c => c.label));
    const withUnread = await labels();
    ok('the window\'s commands include New Group Chat, New Message and Trust a Link', ['New Group Chat…', 'New Message…', 'Trust a Link…'].every(l => withUnread.includes(l)), JSON.stringify(withUnread));
    ok('with unread messages they include Mark All Read and Dismiss All, as the inbox\'s buttons do', withUnread.includes('Mark All Read') && withUnread.includes('Dismiss All') && await page.locator('.bus-inbox-tools button', { hasText: 'Mark All Read' }).count() === 1, JSON.stringify(withUnread));

    // Mark All Read from the window's commands asks first, then marks every
    // message up to the newest read. The command's promise waits for the
    // person's answer, so it is not awaited here.
    await page.evaluate(() => { pudlApplets.commandsIn(document.querySelector('[data-win="bus"]')).find(c => c.label === 'Mark All Read').run(); });
    const dialog = page.locator('dialog[open]').filter({ hasText: 'Mark All Read?' });
    await dialog.waitFor();
    ok('nothing is marked before the person confirms', (await page.evaluate(() => __calls.length)) === 0);
    await dialog.getByRole('button', { name: 'Mark Read' }).click();
    await page.waitForFunction(() => __calls.length === 1);
    ok('confirming marks every message up to the newest read', JSON.stringify(await page.evaluate(() => __calls)) === '[["readThrough",2]]', JSON.stringify(await page.evaluate(() => __calls)));
    await page.waitForFunction(() => !document.querySelector('[data-section="bus-inbox"] .bus-inbox-row'));
    const none = await labels();
    ok('with nothing unread the bulk actions are absent and the rest remain', !none.includes('Mark All Read') && !none.includes('Dismiss All') && none.includes('New Group Chat…'), JSON.stringify(none));
    ok('building the commands raised no error', (await page.evaluate(() => __errors)).length === 0, JSON.stringify(await page.evaluate(() => __errors)));
  } catch (error) {
    ok('the suite ran', false, String(error && error.stack || error));
  } finally {
    await t.close();
  }
})();
