// After a resync each module reads again what its events carry: state through the
// calls it already uses, and requests through the engine's pending lists, each
// shown once however it arrives and withdrawn when it is no longer pending.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => {
      window.__pending = { permissions: [], exit: null, asks: [], reviews: [] };
      window.__answers = [];
      const spy = (name, value) => async (...args) => { window.__reads.push(name); return typeof value === 'function' ? value(...args) : value; };
      window.__reads = [];
      return {
        Sessions: spy('Sessions', []),
        ProviderAccounts: spy('ProviderAccounts', []),
        UpdateState: spy('UpdateState', { ready: false }),
        RecentDocuments: spy('RecentDocuments', []),
        PendingPermissions: async () => window.__pending.permissions,
        PendingExitConfirmation: async () => window.__pending.exit,
        PendingPageAsks: async () => window.__pending.asks,
        PendingExtensionReviews: async () => window.__pending.reviews,
        AnswerPage: async (id, result, err) => { window.__answers.push({ id, err }); },
        AnswerExitConfirmation: async () => {},
        AnswerExtensionCommand: async () => {}
      };
    },
  });
  const { page, ok } = t;
  const resync = async () => { await page.evaluate(() => window.__resync()); await page.waitForTimeout(250); };
  const set = (name, value) => page.evaluate(([n, v]) => { window.__pending[n] = v; }, [name, value]);
  const card = key => page.evaluate(k => dinahHost.notices.list(e => e.id === 'question-' + k).length, key);
  try {
    // State: the calls the modules use at start are made again.
    await page.evaluate(() => { window.__reads.length = 0; });
    await resync();
    const reads = await page.evaluate(() => window.__reads.slice());
    for (const name of ['Sessions', 'ProviderAccounts', 'UpdateState', 'RecentDocuments'])
      ok(name + ' is read again after a resync', reads.includes(name), reads.join(','));

    // Permission questions.
    const question = key => ({ key, sessionId: 'first', toolCall: { title: 'Write ' + key + '.md' }, options: [{ optionId: 'yes', name: 'Allow Once', kind: 'allow_once' }] });
    await set('permissions', [question('q1')]);
    await resync();
    ok('a question the page never heard of is shown after a resync', (await card('q1')) === 1);
    await resync();
    ok('a second resync does not show it twice', (await card('q1')) === 1);
    await page.evaluate(() => __emit('agent:answered', { sessionId: 'first', key: 'q1' }));
    await resync();
    ok('a question answered while the page was told is not shown again by a stale reread', (await card('q1')) === 0);
    await set('permissions', [question('q2')]);
    await resync();
    ok('a new question is shown', (await card('q2')) === 1);
    await set('permissions', []);
    await resync();
    ok('a shown question that is no longer pending is withdrawn', (await card('q2')) === 0);
    await page.evaluate(q => { window.__pending.permissions = [q]; __emit('agent:permission', q); }, question('q3'));
    await resync();
    ok('a question that arrives by event and by reread is shown once', (await card('q3')) === 1);

    // The exit confirmation.
    const exitDialogs = () => page.evaluate(() => document.querySelectorAll('dialog.exit-confirmation').length);
    await set('exit', { id: 'x1', status: { chats: 1, terminals: 0, unfinishedRequests: 0, nonResumableActions: false } });
    await resync();
    ok('an open exit confirmation is shown after a resync', (await exitDialogs()) === 1);
    await resync();
    ok('and not twice', (await exitDialogs()) === 1);
    await set('exit', null);
    await resync();
    ok('a confirmation that is no longer open is closed', (await exitDialogs()) === 0);
    await set('exit', { id: 'x1', status: { chats: 1 } });
    await resync();
    ok('a confirmation that was resolved is not opened again by a stale reread', (await exitDialogs()) === 0);

    // Questions the engine put to the page.
    await set('asks', [{ id: 'a1', op: 'no-such-question', args: {} }]);
    await resync();
    await resync();
    const answers = await page.evaluate(() => window.__answers.filter(a => a.id === 'a1').length);
    ok('a page question is answered once however often it is returned', answers === 1, String(answers));

    // Extension command reviews.
    const reviewDialogs = () => page.evaluate(() => Array.from(document.querySelectorAll('dialog')).filter(d => d.textContent.includes('Review git command')).length);
    const review = { key: 'r1', extension: 'e', input: { program: 'git', explanation: 'Show status', cwd: 'C:/Fixture', args: ['status'] } };
    await set('reviews', [review]);
    await resync();
    await resync();
    ok('a pending review is shown once', (await reviewDialogs()) === 1);
    await set('reviews', []);
    await resync();
    ok('a review that is no longer pending is closed', (await reviewDialogs()) === 0);
    await set('reviews', [review]);
    await resync();
    ok('a review that ended is not shown again by a stale reread', (await reviewDialogs()) === 0);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
