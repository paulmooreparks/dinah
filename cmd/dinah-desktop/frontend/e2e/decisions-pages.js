const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r5'), makeRequest('r4'), makeRequest('r3'), makeRequest('r2')]); const { page: p, ok } = t;
  try {
    await p.evaluate(() => { window.__pageSize = 2; __emit('bus:changed', {}); });
    await p.waitForFunction(() => document.querySelectorAll('.decisions-row').length === 2);
    await p.getByRole('button', { name: 'Earlier Requests', exact: true }).click(); await p.locator('.decisions-row[data-request-id="r2"]').waitFor();
    await choose(t, 'r2'); await p.getByLabel('Answer 1', { exact: true }).fill('Keep this earlier draft editable.');
    await p.evaluate(item => { __decisionItems.unshift(item); __emit('bus:changed', {}); }, makeRequest('r6'));
    await p.locator('.decisions-row[data-request-id="r6"]').waitFor();
    await p.waitForFunction(() => !document.querySelector('.decisions-detail textarea').disabled);
    ok('refresh retains the selected earlier page and its answer', await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'Keep this earlier draft editable.' && await p.locator('.decisions-row[data-request-id="r2"]').count() === 1);
    ok('exclusive cursors are used for earlier pages', (await t.calls('Decisions')).some(c => c[1] === 4));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
