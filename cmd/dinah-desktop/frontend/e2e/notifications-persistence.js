const { launch, PAGE } = require('./lib/page');
(async () => {
  const store = [{ id: 'saved_failure', kind: 'turn-failure', severity: 'error', title: 'A saved failure.', body: 'This happened before restart.', details: ['Read failed'], sessionId: 'old-chat', sessionTitle: 'Old Review', workspace: 'C:/Old', provider: 'codex', created: '2026-10-09T01:00:00Z', dismissed: false }];
  const calls = [];
  let failSave = false;
  // Each fresh browser is its own launch; the failures and page errors of
  // the one before are carried into it, so the last close reports them all.
  let t;
  const ok = (name, condition) => t.ok(name, condition);
  async function fresh() {
    const prev = t;
    if (prev) await prev.browser.close();
    t = await launch({
      viewport: { width: 1280, height: 820 },
      state: { workspace: 'C:/Current', recent: ['C:/Current'], approvals: 'all', devin: 'd', defaultProvider: 'devin', titles: {}, templates: [] },
      app: () => ({
        Notifications: () => __notificationAPI('list'), SaveNotification: item => __notificationAPI('save', item), DismissNotification: id => __notificationAPI('dismiss', id),
        OpenSessionAt: (workspace, id) => __notificationAPI('open', { workspace, id }),
        AckUIStateFlush: (id, success) => __notificationAPI('ack', { id, success })
      }),
      goto: false,
    });
    if (prev) { t.errors.push(...prev.errors); t.failures.push(...prev.failures); }
    const page = t.page;
    await page.exposeFunction('__notificationAPI', async (method, input) => {
      calls.push([method, input]);
      if (method === 'list') return store.filter(item => !item.dismissed);
      if (method === 'save') {
        if (failSave) throw new Error('Notification storage unavailable');
        const old = store.find(item => item.id === input.id);
        if (old) return old;
        const item = { ...input, created: new Date().toISOString(), dismissed: false }; store.push(item); return item;
      }
      if (method === 'dismiss') { store.find(item => item.id === input).dismissed = true; return; }
      if (method === 'open') return { sessionId: input.id, cwd: input.workspace, title: 'Old Review', provider: 'codex', open: true, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } };
    });
    await page.goto(PAGE);
    await page.locator('[data-notification-id="saved_failure"]').waitFor({ state: 'attached' });
    return page;
  }
  try {
    let page = await fresh();
    ok('saved failures restore before their chats are opened', calls.filter(call => call[0] === 'open').length === 0 && await page.locator('#questions .heads-up').count() === 1);
    ok('restored failure has an owner link and no stale approval buttons', (await page.locator('[data-notification-id="saved_failure"]').textContent()).includes('Open Old Review') && await page.locator('#questions button:has-text("Allow")').count() === 0);
    await page.evaluate(() => document.querySelector('[data-notification-id="saved_failure"] .notices-open').click());
    await page.waitForFunction(() => document.querySelector('.win[data-win-kind="chat-session"]'));
    await page.waitForFunction(() => [...document.querySelectorAll('.win-execution .win-state')].some(el => /Idle/.test(el.title)));
    ok('restored owner link retains the original workspace', calls.some(call => call[0] === 'open' && call[1].workspace === 'C:/Old' && call[1].id === 'old-chat'));
    failSave = true;
    await page.evaluate(() => { __emit('agent:session', { sessionId: 'new-chat', cwd: 'C:/Current', title: 'New Review', provider: 'devin', open: true }); __emit('agent:turn', { sessionId: 'new-chat', stopReason: 'error', error: 'Temporary model failure' }); });
    await page.waitForFunction(() => document.querySelectorAll('#questions .heads-up').length === 2);
    await page.evaluate(() => __emit('dinah:ui-state-flush', { id: 'failed-close' }));
    for (let i = 0; i < 100 && !calls.some(call => call[0] === 'ack' && call[1].id === 'failed-close'); i++) await page.waitForTimeout(20);
    ok('native close is refused when notification persistence fails', calls.some(call => call[0] === 'ack' && call[1].id === 'failed-close' && call[1].success === false));
    failSave = false;
    await page.evaluate(() => __emit('dinah:ui-state-flush', { id: 'retry-close' }));
    for (let i = 0; i < 100 && !calls.some(call => call[0] === 'ack' && call[1].id === 'retry-close'); i++) await page.waitForTimeout(20);
    // Every toast is kept too, so the failed turns are counted by their kind.
    const turns = () => store.filter(item => item.kind === 'turn-failure');
    ok('close retries a failed save before acknowledging success', turns().length === 2 && calls.some(call => call[0] === 'ack' && call[1].id === 'retry-close' && call[1].success === true));
    await page.evaluate(() => [...document.querySelectorAll('#questions .heads-up')].find(card => card.dataset.notificationId !== 'saved_failure').querySelector('button.btn-primary').click());
    await page.waitForFunction(() => document.querySelectorAll('#questions .heads-up').length === 1);
    ok('a notice can be dismissed after its original save failed', turns()[1].dismissed === true);
    page = await fresh();
    ok('fresh browser restores only undismissed notifications', await page.locator('#questions .heads-up').count() === 1 && turns()[1].dismissed);
    await page.evaluate(() => document.querySelector('[data-notification-id="saved_failure"] button.btn-primary').click());
    await page.waitForFunction(() => !document.querySelector('#questions .heads-up'));
    ok('dismissal is persisted before the card disappears', store[0].dismissed);
  } finally { if (t) await t.close(); }
})();
