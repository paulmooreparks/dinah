const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ app: () => ({
    Notifications: async () => [{id: 'restart-left-test', kind: 'restart-left', severity: 'warning', title: 'Chats need your help after restart',
      body: 'Their carry-on messages are waiting.', chats: [{id: 'one', cwd: 'C:/Fixture', title: 'First chat'}, {id: 'two', cwd: 'C:/Fixture', title: 'Second chat'}]}]
  }) });
  try {
    const p = t.page;
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    const card = p.locator('#questions [data-notice-id="restart-left-test"]');
    await card.first().waitFor();
    t.ok('one restart card lists both chats', await card.count() === 1 && (await card.innerText()).includes('First chat') && (await card.innerText()).includes('Second chat'));
    t.ok('each chat has an open button', await card.getByRole('button', {name: 'Open First chat', exact: true}).count() === 1 && await card.getByRole('button', {name: 'Open Second chat', exact: true}).count() === 1);
    await p.evaluate(() => __emit('notification:delivery', null));
    await p.waitForFunction(() => dinahHost.notices.list(e => e.id === 'restart-left-test').length === 1);
    t.ok('refresh does not duplicate the card', await card.count() === 1);
    await p.reload();
    await t.waitReady();
    await p.waitForFunction(() => window.dinahHost && dinahHost.notices.get('restart-left-test'));
    t.ok('reload restores both actions', await p.evaluate(() => dinahHost.notices.get('restart-left-test').actions.length === 2));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
