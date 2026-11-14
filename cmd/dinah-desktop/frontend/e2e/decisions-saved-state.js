const { setup, makeRequest } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r9')]); const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahUIState.set('dinah-decisions:request%3Ar9', { version: 1, answers: {}, reviewed: '', operation: { method: 'Quit', status: 'submitted', input: { operationId: 'bad' } } }));
    await p.locator('.decisions-row[data-request-id="r9"] > a').click();
    await p.waitForFunction(() => document.querySelector('.decisions-status').textContent.includes('saved delivery identity is invalid'));
    ok('an unsupported saved operation fails closed without invoking it', (await t.calls('Quit')).length === 0 && await p.locator('.decisions-detail textarea').count() === 0);
    ok('the invalid saved record is preserved for recovery', await p.evaluate(() => dinahUIState.get('dinah-decisions:request%3Ar9').operation.method) === 'Quit');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
