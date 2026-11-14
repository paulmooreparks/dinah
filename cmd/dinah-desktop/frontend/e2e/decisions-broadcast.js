const { setup, makeRequest } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1')]); const { page: p, ok } = t;
  const preview = async scope => {
    await p.evaluate(scope => dinahCommands.run('broadcast-' + scope), scope);
    await p.getByLabel('Broadcast Message', { exact: true }).fill('Please pause after the current task.');
    await p.getByRole('button', { name: 'Preview Recipients', exact: true }).click();
    await p.locator('.decisions-preview li').last().waitFor();
    await p.waitForFunction(() => !document.querySelector('[data-broadcast="confirm"]').disabled);
  };
  try {
    await preview('active');
    ok('the active command previews its frozen count and complete titles without sending', (await p.locator('.decisions-preview').innerText()).includes('2 chats') && await p.locator('.decisions-preview li').count() === 2 && (await t.calls('ConfirmDecisionBroadcast')).length === 0 && (await t.calls('PreviewDecisionBroadcast'))[0][1] === 'active');
    await p.getByRole('button', { name: 'Confirm Broadcast…', exact: true }).click();
    await p.locator('#ask[open]').waitFor(); await p.click('#ask-no');
    ok('cancelling the confirmation sends nothing', (await t.calls('ConfirmDecisionBroadcast')).length === 0);
    await p.getByRole('button', { name: 'Confirm Broadcast…', exact: true }).click();
    await p.locator('#ask[open]').waitFor(); await p.click('#ask-yes');
    await p.waitForFunction(() => __delivered.length === 1);
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('Broadcast queued'));
    ok('confirmation sends only the displayed preview token and consumes the preview', (await t.calls('ConfirmDecisionBroadcast'))[0][1].token === 'preview-active' && await p.getByRole('button', { name: 'Confirm Broadcast…', exact: true }).isDisabled());
    await preview('all');
    ok('the All command uses its own scope and discloses bounded admission', (await t.calls('PreviewDecisionBroadcast')).some(c => c[1] === 'all') && (await p.locator('.decisions-detail').innerText()).includes('Closed chats wait for a background place.'));
    await p.evaluate(() => { window.__rejectBroadcast = true; });
    await p.getByRole('button', { name: 'Confirm Broadcast…', exact: true }).click(); await p.locator('#ask[open]').waitFor(); await p.click('#ask-yes');
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('The preview expired'));
    ok('terminal preview refusal preserves the message and allows a fresh preview', await p.getByLabel('Broadcast Message', { exact: true }).inputValue() === 'Please pause after the current task.' && !await p.getByRole('button', { name: 'Preview Recipients', exact: true }).isDisabled() && await p.getByRole('button', { name: 'Confirm Broadcast…', exact: true }).isDisabled());
    await p.evaluate(() => { window.__rejectBroadcast = false; });
    await p.getByRole('button', { name: 'Preview Recipients', exact: true }).click(); await p.waitForFunction(() => !document.querySelector('[data-broadcast="confirm"]').disabled);
    await p.getByRole('button', { name: 'Confirm Broadcast…', exact: true }).click(); await p.locator('#ask[open]').waitFor(); await p.click('#ask-yes');
    await p.waitForFunction(() => __delivered.length === 2);
    const calls = await t.calls('ConfirmDecisionBroadcast');
    ok('a corrected broadcast gets a new operation identity', calls[1][1].operationId !== calls[2][1].operationId);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
