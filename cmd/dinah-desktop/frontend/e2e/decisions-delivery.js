const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1'), makeRequest('r2', { questions: [makeRequest('r2').questions[0]] })]); const { page: p, ok } = t;
  try {
    await choose(t, 'r1'); await p.getByLabel('Answer 1', { exact: true }).fill('Keep this exact answer.');
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.evaluate(() => { window.__beforeFailure = true; });
    await p.getByRole('button', { name: 'Send Answers', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('unconfirmed'));
    ok('unconfirmed delivery keeps the draft and gates a new answer', await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'Keep this exact answer.' && await p.getByRole('button', { name: 'Send Answers', exact: true }).isDisabled());
    await p.evaluate(() => { window.__beforeFailure = false; });
    await p.getByRole('button', { name: 'Check Delivery', exact: true }).click();
    await p.waitForFunction(() => __delivered.length === 1);
    const calls = await t.calls('AnswerDecision');
    ok('explicit retry preserves operation identity and payload', JSON.stringify(calls[0][1]) === JSON.stringify(calls[1][1]));
    await choose(t, 'r2'); await p.getByLabel('Answer 1', { exact: true }).fill('Already delivered despite a lost receipt.');
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.evaluate(() => { window.__lostReceipt = true; });
    await p.getByRole('button', { name: 'Send Answers', exact: true }).click();
    await p.waitForFunction(() => __delivered.length === 2);
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('unconfirmed'));
    await p.evaluate(() => dinahUIState.flush()); await p.reload(); await p.locator('.decisions').waitFor();
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('Delivered to the bus'));
    ok('reload reconciles a committed reply without resending it', (await t.calls('AnswerDecision')).length === 0 && (await t.calls('DecisionOperation')).length > 0);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
