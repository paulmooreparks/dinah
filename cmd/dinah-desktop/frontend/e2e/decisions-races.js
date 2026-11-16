const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1'), makeRequest('r2'), makeRequest('r3', { text: 'A plain request.' })]); const { page: p, ok } = t;
  try {
    await choose(t, 'r1'); await p.getByLabel('Answer 1', { exact: true }).fill('Only send this to the first request.');
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.evaluate(() => { window.__holdSave = true; });
    await p.getByRole('button', { name: 'Send', exact: true }).click();
    await p.waitForFunction(() => window.__releaseSave);
    await choose(t, 'r2');
    await p.evaluate(() => { window.__holdSave = false; __releaseSave(); });
    await p.waitForFunction(() => __delivered.length === 1);
    ok('navigation during the pre-send save cannot change the destination', (await t.calls('AnswerDecision'))[0][1].id === 'r1' && (await p.locator('.decisions-about').innerText()).includes('Review r2'));
    await p.getByLabel('Answer 1', { exact: true }).fill('Malformed receipts remain unconfirmed.'); await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.evaluate(() => { window.__malformedReceipt = true; }); await p.getByRole('button', { name: 'Send', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('not been confirmed'));
    ok('a malformed terminal receipt keeps the shared send gate held', await p.getByRole('button', { name: 'Send', exact: true }).isDisabled());
    await p.getByRole('button', { name: 'Check Send Result', exact: true }).click(); await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('Your reply was sent'));
    ok('the stored receipt reconciles without duplicate send', (await t.calls('AnswerDecision')).length === 2);
    await choose(t, 'r3');
    ok('dismissal remains disabled pending settlement support and sends nothing', await p.getByRole('button', { name: 'Dismiss', exact: true }).isDisabled() && (await t.calls('DismissDecision')).length === 0);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
