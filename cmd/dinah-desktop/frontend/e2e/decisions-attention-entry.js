const { setup, makeRequest } = require('./lib/decisions-fixture');
(async () => {
  // The opaque ID deliberately differs from its message number.
  const older = makeRequest('opaque-target', { messageId: 10 });
  const t = await setup([makeRequest('r40'), makeRequest('r30'), makeRequest('r20'), older], { entryPoints: true, noOpen: true }); const { page: p, ok } = t;
  const emit = (id, message, source = 'review') => p.evaluate(({ id, message, source }) => {
    const item = { id, kind: 'mention', severity: 'info', title: 'Review asks you', body: 'Please decide.\nA second line.', group: 'review-group', groupTitle: 'Review Group', sessionId: source, sourceId: source, sessionTitle: 'Review', message, created: '2026-11-14T00:00:00Z' };
    sessionStorage.setItem('attention', JSON.stringify(JSON.parse(sessionStorage.getItem('attention') || '[]').concat(item))); __emit('notifications:added', item);
  }, { id, message, source });
  try {
    await p.evaluate(() => { __pageSize = 1; return dinahDecisionEntries.refresh(); });
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await emit('target-card', 10);
    const card = p.locator('#questions [data-notice-id="target-card"]');
    await card.getByRole('button', { name: 'Open Decision', exact: true }).click();
    await p.locator('.decisions-row[data-request-id="opaque-target"] > a[aria-current="true"]').waitFor();
    await p.waitForFunction(() => (window.__dismissed || []).includes('target-card'));
    ok('card opens the opaque request beyond two earlier pages', (await t.calls('Decisions')).some(c => c[1] === 20));
    ok('showing a request clears its attention without answering it', await p.evaluate(() => __decisionItems.length === 4 && __delivered.length === 0));
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await emit('failure-card', 40, 'failure');
    await p.evaluate(() => { __entriesFailure = true; });
    await p.locator('#questions [data-notice-id="failure-card"]').getByRole('button', { name: 'Open Decision', exact: true }).click();
    await p.locator('.toast', { hasText: 'Open requests could not be read' }).first().waitFor();
    ok('lookup failure keeps the retained card', await p.evaluate(() => !(__dismissed || []).includes('failure-card')));
    await p.evaluate(() => { __entriesFailure = false; __listFailure = true; });
    await p.locator('#questions [data-notice-id="failure-card"]').getByRole('button', { name: 'Open Decision', exact: true }).click();
    await p.locator('.toast', { hasText: 'could not be shown' }).first().waitFor();
    ok('a row read failure keeps the retained card', await p.evaluate(() => !(__dismissed || []).includes('failure-card')));
    await p.evaluate(() => {
      __listFailure = false;
      window.__draftRead = dinahDecisionDrafts.read;
      dinahDecisionDrafts.read = () => { throw new Error('Draft read unavailable'); };
    });
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await p.locator('#questions [data-notice-id="failure-card"]').getByRole('button', { name: 'Open Decision', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.decisions-detail').textContent.includes('saved draft could not be read'));
    ok('a failed question-form display keeps the retained card', await p.evaluate(() => !(__dismissed || []).includes('failure-card')));
    await p.evaluate(() => { dinahDecisionDrafts.read = __draftRead; });
    await p.evaluate(() => { __listFailure = false; dinahBus.openGroup = (group, target) => { window.__fallback = [group, target.message]; return Promise.resolve(true); }; });
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await emit('source-card', 999, 'source');
    await p.locator('#questions [data-notice-id="source-card"]').getByRole('button', { name: 'Open Group', exact: true }).click();
    await p.waitForFunction(() => (window.__dismissed || []).includes('source-card'));
    ok('a message outside Decisions still opens its group at the message', await p.evaluate(() => __fallback[0] === 'review-group' && __fallback[1] === 999));
    await p.evaluate(() => { __decisionItems.unshift({ ...__decisionItems[0], id: 'plain-mention', messageId: 900, kind: 'mention', questions: [], canDismiss: true }); __emit('bus:changed', {}); });
    await emit('plain-card', 900, 'plain');
    await p.waitForFunction(() => dinahDecisionEntries.get(900) === 'plain-mention');
    await p.locator('#questions [data-notice-id="plain-card"]').getByRole('button', { name: 'Open Decision', exact: true }).click();
    await p.waitForFunction(() => (window.__dismissed || []).includes('plain-card'));
    ok('plain group attention opens its Decisions row', await p.locator('.decisions-row[data-request-id="plain-mention"]').count() === 1);
    await p.evaluate(() => {
      window.__realOpen = dinahDecisions.openRequest;
      dinahDecisions.openRequest = id => new Promise(resolve => { window.__continueOpen = () => resolve(__realOpen(id)); });
      return dinahWorkbench.open('needs-you');
    });
    await emit('concurrent-old', 30, 'concurrent');
    await p.locator('#questions [data-notice-id="concurrent-old"]').getByRole('button', { name: 'Open Decision', exact: true }).click();
    await p.waitForFunction(() => window.__continueOpen);
    await emit('concurrent-new', 20, 'concurrent');
    await p.evaluate(() => __continueOpen());
    await p.waitForFunction(() => (window.__dismissed || []).includes('concurrent-old'));
    ok('a newer request survives an older card navigation', await p.evaluate(() => !__dismissed.includes('concurrent-new') && dinahHost.notices.list(e => e.data.attentionChat === 'concurrent' && e.data.requestCount === 1).length === 1));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
