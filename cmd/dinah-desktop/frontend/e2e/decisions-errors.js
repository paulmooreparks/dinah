const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1'), makeRequest('r2', { canDismiss: true, dismissalReason: 'The asking chat was removed. You can dismiss this request.' })]); const { page: p, ok } = t;
  try {
    await choose(t, 'r1'); await p.getByLabel('Answer 1', { exact: true }).fill('Do not lose this answer.');
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.evaluate(() => { window.__rejectAnswer = true; });
    await p.getByRole('button', { name: 'Send', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('The request changed'));
    ok('a terminal refusal preserves the answer and shows its reason', await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'Do not lose this answer.' && !await p.getByLabel('Answer 1', { exact: true }).isDisabled());
    await p.evaluate(() => { window.__rejectAnswer = false; });
    await p.getByLabel('Answer 1', { exact: true }).fill('My corrected answer.'); await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.getByRole('button', { name: 'Send', exact: true }).click(); await p.waitForFunction(() => __delivered.length === 1);
    const calls = await t.calls('AnswerDecision');
    ok('a corrected attempt gets a new identity', calls[0][1].operationId !== calls[1][1].operationId);
    await choose(t, 'r2');
    ok('an orphaned request shows the host\'s reason, blocks answers and can be dismissed', await p.getByRole('button', { name: 'Dismiss', exact: true }).isEnabled() && (await p.locator('.decisions-detail').innerText()).includes('The asking chat was removed') && await p.getByLabel('Answer 1', { exact: true }).isDisabled() && await p.getByRole('button', { name: 'Send', exact: true }).isDisabled() && (await p.locator('.decisions-foot-status').innerText()) === 'An answer cannot be delivered. Dismissing closes the request.' && (await t.calls('DismissDecision')).length === 0 && (await t.calls('DismissRequest')).length === 0);
    await p.evaluate(() => { window.__malformedList = true; __emit('bus:changed', {}); });
    await p.waitForFunction(() => document.querySelector('.decisions-status').textContent.includes('request list is invalid'));
    ok('a malformed list is reported without losing the previous rows', await p.locator('.decisions-row[data-request-id="r1"]').count() === 1);
    await p.evaluate(() => { window.__malformedList = false; window.__listFailure = true; __emit('bus:changed', {}); });
    await p.waitForFunction(() => document.querySelector('.decisions-status').textContent.includes('Request service unavailable'));
    ok('a read error remains visible in the fixed footer', (await p.locator('.decisions-foot-status').innerText()).includes('Request service unavailable'));
  } finally { await t.close(); }
  const empty = await setup([]);
  try { await empty.page.waitForFunction(() => document.querySelector('.decisions-empty').textContent.includes('There are no open requests.')); empty.ok('empty requests have an honest empty state', await empty.page.locator('.decisions-row').count() === 0); } finally { await empty.close(); }
  const absent = await setup([], { unavailable: true });
  try { await absent.page.waitForFunction(() => document.querySelector('.decisions-status').textContent.includes('not available')); await absent.page.locator('.menubar-row .menubar-title').filter({ hasText: /^Actions$/ }).click(); absent.ok('an unavailable engine explains the refusal and disables request reads', (await absent.page.locator('.decisions-empty').innerText()).includes('not available') && await absent.page.locator('#menubar-panel .menu-action').filter({ hasText: /^Refresh Requests$/ }).getAttribute('aria-disabled') === 'true'); } finally { await absent.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
