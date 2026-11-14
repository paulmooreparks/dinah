const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1'), makeRequest('r2', { canDismiss: true, dismissalReason: 'The asking chat was removed. You can dismiss this request.' })]); const { page: p, ok } = t;
  try {
    await choose(t, 'r1'); await p.getByLabel('Answer 1', { exact: true }).fill('Do not lose this answer.');
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.evaluate(() => { window.__rejectAnswer = true; });
    await p.getByRole('button', { name: 'Send Answers', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('The request changed'));
    ok('a terminal refusal preserves the answer and shows its reason', await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'Do not lose this answer.' && !await p.getByLabel('Answer 1', { exact: true }).isDisabled());
    await p.evaluate(() => { window.__rejectAnswer = false; });
    await p.getByLabel('Answer 1', { exact: true }).fill('My corrected answer.'); await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.getByRole('button', { name: 'Send Answers', exact: true }).click(); await p.waitForFunction(() => __delivered.length === 1);
    const calls = await t.calls('AnswerDecision');
    ok('a corrected attempt gets a new identity', calls[0][1].operationId !== calls[1][1].operationId);
    await choose(t, 'r2');
    ok('an orphaned structured request exposes dismissal and its explanation', await p.getByRole('button', { name: 'Dismiss Request', exact: true }).count() === 1 && (await p.locator('.decisions-detail').innerText()).includes('The asking chat was removed') && await p.getByRole('button', { name: 'Send Answers', exact: true }).isDisabled());
    await p.getByRole('button', { name: 'Dismiss Request', exact: true }).click(); await p.locator('#ask[open]').waitFor(); await p.click('#ask-yes');
    await p.waitForFunction(() => !__decisionItems.some(item => item.id === 'r2'));
    ok('orphan dismissal closes it without sending an answer', (await t.calls('DismissDecision'))[0][1].id === 'r2' && (await t.calls('AnswerDecision')).length === 2);
    await p.evaluate(() => { window.__malformedList = true; __emit('bus:changed', {}); });
    await p.waitForFunction(() => document.querySelector('.decisions-status').textContent.includes('request list is invalid'));
    ok('a malformed list is reported without losing the previous rows', await p.locator('.decisions-row[data-request-id="r1"]').count() === 1);
    await p.evaluate(() => { window.__malformedList = false; window.__listFailure = true; __emit('bus:changed', {}); });
    await p.waitForFunction(() => document.querySelector('.decisions-status').textContent.includes('Request service unavailable'));
    ok('a read error remains visible', true);
  } finally { await t.close(); }
  const empty = await setup([]);
  try { await empty.page.waitForFunction(() => document.querySelector('.decisions-status').textContent === 'No open requests.'); empty.ok('empty requests have an honest empty state', await empty.page.locator('.decisions-row').count() === 0); } finally { await empty.close(); }
  const absent = await setup([], { unavailable: true });
  try { await absent.page.waitForFunction(() => document.querySelector('.decisions-status').textContent.includes('not available')); absent.ok('an unavailable engine disables request reads', await absent.page.getByRole('button', { name: 'Refresh', exact: true }).isDisabled()); } finally { await absent.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
