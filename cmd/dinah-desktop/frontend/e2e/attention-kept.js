const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1400, height: 900 }, app: () => {
    window.__calls = []; window.__hold = false;
    return {
      Notifications: async () => {
        const snapshot = JSON.parse(sessionStorage.getItem('attention') || '[]');
        if (window.__holdRestore) await new Promise(resolve => { window.__restoreDone = resolve; });
        return snapshot;
      },
      DismissNotification: async id => {
        __calls.push(id);
        if (window.__rejectDismiss) throw new Error('Dismissal unavailable');
        if (__hold) await new Promise(resolve => { window.__settle = resolve; });
        sessionStorage.setItem('attention', JSON.stringify(JSON.parse(sessionStorage.getItem('attention') || '[]').filter(n => n.id !== id)));
      }
    };
  }});
  const { page: p, ok } = t;
  const emit = (n, chat = 'review', kind = 'mention', group = '') => p.evaluate(({n, chat, kind, group}) => {
    const item = { id: 'kept-' + n, kind, severity: 'info', title: chat + ' asks you', body: 'First line ' + n + '\nPrivate second line',
      group, groupTitle: group, sessionId: chat, sourceId: chat, sessionTitle: chat, message: kind === 'attention' ? 0 : n, permissionKey: kind === 'attention' ? 'q' + n : '', created: '2026-11-14T00:00:00Z' };
    const store = JSON.parse(sessionStorage.getItem('attention') || '[]').filter(i => i.id !== item.id); store.push(item);
    sessionStorage.setItem('attention', JSON.stringify(store)); __emit('notifications:added', item);
  }, { n, chat, kind, group });
  try {
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await emit(1);
    const cards = p.locator('#questions .attention-kept');
    await cards.first().waitFor();
    const box = await cards.first().boundingBox();
    const windowBox = await p.locator('.win[data-win="needs-you"]').boundingBox();
    const filterBox = await p.locator('#needs-filter').boundingBox();
    await p.locator('#needs-filter').focus();
    await emit(2); await emit(2);
    await p.waitForFunction(() => dinahHost.notices.list(e => e.data.requestCount === 2).length === 1);
    ok('one source has one counted card without duplicate requests', await cards.count() === 1 && (await cards.first().innerText()).includes('2 requests'));
    ok('the card names the chat and shows only the first line', (await cards.first().innerText()).includes('review') && !(await cards.first().innerText()).includes('Private second'));
    await emit(3, 'other');
    await p.waitForFunction(() => document.querySelectorAll('#questions .attention-kept').length === 2);
    ok('arrival preserves the earlier card geometry', JSON.stringify(box) === JSON.stringify(await cards.first().boundingBox()));
    ok('arrival preserves window and filter geometry', JSON.stringify(windowBox) === JSON.stringify(await p.locator('.win[data-win="needs-you"]').boundingBox()) && JSON.stringify(filterBox) === JSON.stringify(await p.locator('#needs-filter').boundingBox()));
    ok('arrival preserves keyboard focus', await p.locator('#needs-filter').evaluate(e => e === document.activeElement));
    await emit(4, 'permissions', 'attention');
    await p.evaluate(() => { __emit('agent:permission', { sessionId: 'permissions', key: 'q4', options: [] }); __emit('agent:answered', { sessionId: 'permissions', key: 'q4' }); });
    ok('durable permission attention has no duplicate live card', await p.locator('#questions [data-permission-key="q4"]').count() === 0);
    await p.reload();
    await p.waitForFunction(() => window.dinahHost && dinahHost.notices && dinahHost.notices.list(e => e.data.requestCount).length === 3);
    ok('all sources return after reload', true);
    await p.evaluate(() => { dinahWorkbench.open('needs-you'); __hold = true; });
    await p.locator('#questions [data-notice-id="kept-3"]').getByRole('button', { name: 'Dismiss', exact: true }).click();
    await p.waitForFunction(() => __calls.includes('kept-3'));
    await emit(5, 'other');
    await p.evaluate(() => { __hold = false; __settle(); });
    await p.waitForFunction(() => dinahHost.notices.list(e => e.data.attentionChat === 'other' && e.data.requestCount === 1).length === 1);
    ok('a late dismissal preserves a newer request', (await p.locator('#questions').innerText()).includes('First line 5'));
    await p.evaluate(() => __emit('notifications:dismissed', { id: 'kept-1' }));
    await p.waitForFunction(() => dinahHost.notices.list(e => e.data.attentionChat === 'review' && e.data.requestCount === 1).length === 1);
    ok('answering one request leaves its grouped neighbour', true);
    await emit(6, 'group-source', 'mention', 'fixture-group');
    await p.evaluate(() => { dinahBus.openGroup = (key, target) => { window.__openRequested = [key, target.message]; return new Promise(resolve => { window.__shown = resolve; }); }; });
    await p.locator('#questions [data-notice-id="kept-6"]').getByRole('button', { name: 'Open Group' }).click();
    await p.waitForFunction(() => window.__openRequested);
    await emit(7, 'group-source', 'mention', 'fixture-group');
    await p.evaluate(() => __shown(true));
    await p.waitForFunction(() => dinahHost.notices.list(e => e.data.attentionChat === 'group-source' && e.data.requestCount === 1).length === 1);
    ok('navigation targets the requested message and preserves later arrivals', await p.evaluate(() => __openRequested[0] === 'fixture-group' && __openRequested[1] === 6 && __calls.includes('kept-6') && !__calls.includes('kept-7')));
    await p.evaluate(() => { window.__rejectDismiss = true; });
    await p.locator('#questions [data-notice-id="kept-4"]').getByRole('button', { name: 'Dismiss', exact: true }).click();
    await p.waitForFunction(() => __calls.includes('kept-4'));
    await p.locator('.toast', { hasText: 'Dismissal unavailable' }).first().waitFor();
    ok('a failed dismissal keeps the request', await p.locator('#questions [data-notice-id="kept-4"]').count() === 1);
    await p.evaluate(() => { window.__holdRestore = true; __emit('notification:delivery', {}); });
    await p.waitForFunction(() => window.__restoreDone);
    await emit(8, 'during-refresh');
    await p.evaluate(() => { window.__holdRestore = false; __restoreDone(); });
    await p.waitForFunction(() => dinahHost.notices.list(e => e.data.attentionChat === 'during-refresh').length === 1);
    ok('a delayed stored snapshot cannot erase a live arrival', true);

  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
