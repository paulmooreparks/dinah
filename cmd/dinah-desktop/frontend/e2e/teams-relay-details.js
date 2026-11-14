const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ app: () => ({
    Notifications: async () => JSON.parse(sessionStorage.getItem('relay-notices') || '[]'),
    DismissNotification: async id => {
      const items = JSON.parse(sessionStorage.getItem('relay-notices') || '[]');
      sessionStorage.setItem('relay-notices', JSON.stringify(items.filter(item => item.id !== id)));
    }
  }) });
  const { page: p, ok } = t;
  const emit = async (number, details) => p.evaluate(({ number, details }) => {
    const item = { id: 'relay-' + number, kind: 'relay', severity: 'warning', title: 'A relay could not run',
      body: 'Decision ' + number, sessionId: 'worker', sessionTitle: 'Worker', group: 'fixture-team',
      groupTitle: 'Fixture Team', lead: 'lead', message: number, created: '2026-11-14T00:00:00Z', details };
    const items = JSON.parse(sessionStorage.getItem('relay-notices') || '[]');
    items.push(item); sessionStorage.setItem('relay-notices', JSON.stringify(items));
    __emit('notifications:added', item);
  }, { number, details });
  try {
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await emit(1, ['First failure.']);
    await emit(2, ['The provider could not run.', 'Retry after choosing an available provider.']);
    const card = p.locator('#questions [data-notice-id="relay-1"]');
    await card.getByRole('button', { name: 'Open Lead', exact: true }).waitFor();
    const verify = async phase => {
      const details = await card.locator('.notices-details li').allTextContents();
      ok(phase + ' retains the latest reason and remedy alongside the count',
        details.includes('The provider could not run.') && details.includes('Retry after choosing an available provider.') &&
        details.some(line => line.includes('2 requests')) && !details.includes('First failure.'), JSON.stringify(details));
      ok(phase + ' retains both navigation actions and one card',
        await card.getByRole('button', { name: 'Open Group', exact: true }).count() === 1 &&
        await card.getByRole('button', { name: 'Open Lead', exact: true }).count() === 1 &&
        await p.locator('#questions .attention-kept').count() === 1);
    };
    await verify('arrival');
    await p.reload();
    await card.getByRole('button', { name: 'Open Lead', exact: true }).waitFor();
    await verify('reload');
    await p.evaluate(() => dinahHost.notices.add({ id: 'refresh-trigger', type: 'info', title: 'Fixture refresh' }));
    await verify('redraw');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
