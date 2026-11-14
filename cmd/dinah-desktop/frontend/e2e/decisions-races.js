const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1'), makeRequest('r2'), makeRequest('r3', { kind: 'mention', questions: [], text: 'A plain request.' })]); const { page: p, ok } = t;
  try {
    await choose(t, 'r1'); await p.getByLabel('Answer 1', { exact: true }).fill('Only send this to the first request.');
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.evaluate(() => { window.__holdSave = true; });
    await p.getByRole('button', { name: 'Send Answers', exact: true }).click();
    await p.waitForFunction(() => window.__releaseSave);
    await choose(t, 'r2');
    await p.evaluate(() => { window.__holdSave = false; __releaseSave(); });
    await p.waitForFunction(() => __delivered.length === 1);
    ok('navigation during the pre-send save cannot change the destination', (await t.calls('AnswerDecision'))[0][1].id === 'r1' && (await p.locator('.decisions-destination').innerText()).includes('Review r2'));
    await p.getByLabel('Answer 1', { exact: true }).fill('Malformed receipts remain unconfirmed.'); await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.evaluate(() => { window.__malformedReceipt = true; }); await p.getByRole('button', { name: 'Send Answers', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('unconfirmed'));
    ok('a malformed terminal receipt keeps the shared send gate held', await p.getByRole('button', { name: 'Send Answers', exact: true }).isDisabled());
    await p.getByRole('button', { name: 'Check Delivery', exact: true }).click(); await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('Delivered to the bus'));
    ok('the stored receipt reconciles without duplicate send', (await t.calls('AnswerDecision')).length === 2);
    await choose(t, 'r3'); await p.getByRole('button', { name: 'Dismiss Message', exact: true }).click(); await p.locator('#ask[open]').waitFor(); await p.click('#ask-no');
    ok('dismissal confirmation cancellation changes nothing', (await t.calls('DismissDecision')).length === 0);
    await p.getByRole('button', { name: 'Dismiss Message', exact: true }).click(); await p.locator('#ask[open]').waitFor(); await p.click('#ask-yes'); await p.waitForFunction(() => __delivered.length === 3);
    ok('a confirmed plain dismissal uses the exact request without an answer', (await t.calls('DismissDecision'))[0][1].id === 'r3' && !(await t.calls('DismissDecision'))[0][1].answers);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
