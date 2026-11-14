const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1'), makeRequest('r2')]); const { page: p, ok } = t;
  try {
    await choose(t, 'r1');
    await p.getByLabel('Answer 1', { exact: true }).fill('My independent answer.');
    await p.locator('[data-question="2"]').getByRole('button', { name: 'As Recommended', exact: true }).click();
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await choose(t, 'r2'); await p.getByLabel('Answer 1', { exact: true }).fill('A different draft.');
    await p.evaluate(() => dinahUIState.flush()); await p.reload();
    await p.locator('.decisions').waitFor(); await choose(t, 'r1');
    ok('per-request text and recommendation drafts survive reload', await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'My independent answer.' && await p.getByLabel('Answer 2', { exact: true }).inputValue() === 'Proceed tomorrow.');
    await choose(t, 'r2');
    ok('another request retains its own draft', await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'A different draft.');
    await p.evaluate(() => { __decisionItems.find(i => i.id === 'r2').destination = 'Changed Destination'; __decisionItems.find(i => i.id === 'r2').revision++; __emit('bus:changed', {}); });
    await p.getByRole('button', { name: 'I Have Reviewed the Updated Request', exact: true }).waitFor();
    ok('changed destination retains text and blocks sending pending review', await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'A different draft.' && await p.getByRole('button', { name: 'Send Answers', exact: true }).isDisabled());
    await p.getByRole('button', { name: 'I Have Reviewed the Updated Request', exact: true }).click();
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    ok('reviewing the new request permits the preserved draft', true);
    await p.evaluate(() => { window.__saveFailure = true; });
    await p.getByLabel('Answer 1', { exact: true }).fill('Unsaved edit.');
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('Draft could not be saved'));
    ok('storage failure is visible and prevents sending', await p.getByRole('button', { name: 'Send Answers', exact: true }).isDisabled() && (await t.calls('AnswerDecision')).length === 0);
    await p.evaluate(() => { window.__saveFailure = false; });
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
