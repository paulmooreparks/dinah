const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1')]); const { page: p, ok } = t;
  try {
    await choose(t, 'r1');
    await p.locator('[data-question="1"]').getByRole('button', { name: 'As Recommended', exact: true }).click();
    await p.getByRole('button', { name: 'Send Answers', exact: true }).waitFor({ state: 'visible' });
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    ok('As Recommended prepares an answer without sending', await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'Use the reviewed release.' && (await t.calls('AnswerDecision')).length === 0);
    await p.getByRole('button', { name: 'Send Answers', exact: true }).click();
    await p.waitForFunction(() => window.__delivered.length === 1);
    await p.waitForFunction(() => document.querySelectorAll('.decisions-question textarea').length === 1);
    const calls = await t.calls('AnswerDecision');
    ok('partial answers use the reviewed identity and leave the other question open', calls[0][1].id === 'r1' && calls[0][1].revision === 1 && calls[0][1].answers.length === 1 && calls[0][1].answers[0].asRecommended && calls[0][1].answers[0].text === '' && await p.getByLabel('Answer 2', { exact: true }).count() === 1);
    await p.getByLabel('Answer 2', { exact: true }).fill('Next week.');
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.getByRole('button', { name: 'Send Answers', exact: true }).click();
    await p.waitForFunction(() => window.__delivered.length === 2);
    ok('the second answer uses the new revision and closes the request', (await t.calls('AnswerDecision'))[1][1].revision === 2 && await p.evaluate(() => __decisionItems.length === 0));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
