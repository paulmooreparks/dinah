const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    state: { workspace: 'C:/Fixture', recent: ['C:/Fixture'] },
    app: () => {
      window.__calls = [];
      window.__waiting = [{ id: 'waiting_conflict', kind: 'delivery-failure', severity: 'error', title: 'Messages are waiting for Retained Claude work',
        body: 'The two recipients require explicit routing recovery. All messages remain waiting. Recover the original saved chat to retain its recipient. The separate continuation keeps its own recipient and history.', details: ['Original recipient retained', 'Actual target retained'],
        sessionId: 'claude:missing', sessionTitle: 'Retained Claude work', workspace: 'C:/Fixture', provider: 'claude', lead: 'codex:older', retryAllowed: false,
        created: '2026-01-01T00:00:00Z' },
      { id: 'waiting_retry', kind: 'delivery-failure', severity: 'error', title: 'Messages are waiting for Retryable work', body: 'The previous opening failed.',
        details: ['Both references own the same recipient'], sessionId: 'codex:retry', sessionTitle: 'Retryable work', workspace: 'C:/Fixture', provider: 'codex',
        lead: 'codex:retry', retryAllowed: true, created: '2026-01-01T00:00:00Z' }];
      return {
        Notifications: async () => __waiting,
        ReloadWaitingChatState: async () => { __calls.push(['reload']); __waiting = __waiting.filter(n => n.kind !== 'delivery-store-error'); },
        RetryWaitingSession: async origin => { __calls.push(['retry', origin]); __waiting = __waiting.filter(n => n.lead !== origin); return { sessionId: origin, open: true }; },
        OpenWaitingSession: async (origin, attempt) => { __calls.push(['attempt', origin, attempt]); throw new Error('Resource not found: actual target'); },
        OpenSessionAt: async (cwd, id) => { __calls.push(['open', cwd, id]); throw new Error('Resource not found: actual target'); },
        SavedConversation: async () => ({ available: true, complete: true, messages: [{ role: 'agent', text: 'The saved history remains readable.' }] }),
        SessionContinuation: async () => ({ history: [] }),
        AgentProviders: async () => [{ id: 'claude', name: 'Claude', installed: true, available: true }, { id: 'codex', name: 'Codex', installed: true, available: true }],
        ProviderModelPreferences: async provider => ({ provider, catalog: [{ id: 'model', category: 'model', type: 'select', currentValue: 'fixture-model', options: [{ value: 'fixture-model', name: 'Fixture Model' }] }], defaults: {}, pinned: [] }),
        PreviewSessionRecoveryModel: async (id, provider, model) => { __calls.push(['preview-recovery', id, provider, model]); return { revision: 'chosen-destination', warnings: ['The original recipient keeps its waiting messages.'], facts: { kind: 'provider', from: {}, to: { providerName: provider, model } } }; },
        RecoverSessionModel: async () => { __calls.push(['recover']); throw new Error('The fixture never confirms allocation.'); },
      };
    },
    ready: () => document.querySelector('[data-notification-id="waiting_conflict"]'),
  });
  const { page, ok } = t;
  try {
    await page.evaluate(() => dinahWorkbench.open('needs-you'));
    const conflict = page.locator('[data-notification-id="waiting_conflict"]');
    const retry = page.locator('[data-notification-id="waiting_retry"]');
    // Paul asked on 2 November 2026 that these cards can be dismissed.
    ok('routing recovery has actual target details, review and Dismiss, without retry',
      await conflict.getByRole('button', { name: 'Review Chat', exact: true }).count() === 1 && await conflict.getByRole('button', { name: 'Recover Original Chat', exact: true }).count() === 1 &&
      await conflict.getByRole('button', { name: /Retry/ }).count() === 0 && await conflict.getByRole('button', { name: 'Dismiss', exact: true }).count() === 1 && (await conflict.textContent()).includes('Actual target retained'));
    await retry.getByRole('button', { name: 'Retry Waiting Delivery', exact: true }).click();
    await retry.waitFor({ state: 'detached' });
    ok('explicit retry addresses its original owner and removes only completed state', await page.evaluate(() => __calls.some(c => c[0] === 'retry' && c[1] === 'codex:retry')) && await conflict.count() === 1);
    await conflict.getByRole('button', { name: 'Review Chat', exact: true }).click();
    await page.getByText('The saved history remains readable.', { exact: true }).waitFor();
    ok('review preserves saved content and opens the actual target with its workspace', await page.evaluate(() => __calls.some(c => c[0] === 'open' && c[1] === 'C:/Fixture' && c[2] === 'claude:missing')));
    await page.evaluate(() => __emit('chat:connect', { sessionId: 'codex:delivery', provider: 'codex', title: 'Delivery target', cwd: 'C:/Fixture', deliveryOrigin: 'codex:origin', deliveryAttempt: 'attempt-7', background: true }));
    await page.waitForFunction(() => __calls.some(c => c[0] === 'attempt'));
    ok('background page opening uses the origin and generation without uncorrelated open', await page.evaluate(() => __calls.some(c => c[0] === 'attempt' && c[1] === 'codex:origin' && c[2] === 'attempt-7') && !__calls.some(c => c[0] === 'open' && c[2] === 'codex:delivery')));
    ok('the routing recovery card survives another chat opening failure', await conflict.count() === 1);
    await page.evaluate(() => dinahWorkbench.open('needs-you'));
    await page.evaluate(() => {
      __waiting.push({ id: 'ordinary_notice', kind: 'app', severity: 'info', title: 'Ordinary notices remain available', created: '2026-03-01T00:00:00Z' });
      __waiting.push({ id: 'waiting_store_error', kind: 'delivery-store-error', severity: 'error', title: 'Waiting-chat recovery state could not be read', body: 'Automatic delivery is paused. Ordinary notifications remain available.', details: ['Invalid recovery JSON'], created: '2026-03-02T00:00:00Z' });
      __emit('notification:delivery');
    });
    const storeError = page.locator('[data-notification-id="waiting_store_error"]');
    await storeError.waitFor();
    ok('recovery read failure coexists with unrelated notices and retained routing details', await page.locator('[data-notification-id="ordinary_notice"]').count() === 1 && await conflict.count() === 1);
    await storeError.getByRole('button', { name: 'Reload Recovery State', exact: true }).click();
    await storeError.waitFor({ state: 'detached' });
    ok('explicit reload clears only the repaired store diagnostic', await page.evaluate(() => __calls.some(c => c[0] === 'reload')) && await conflict.count() === 1);
    await conflict.getByRole('button', { name: 'Recover Original Chat', exact: true }).click();
    await page.getByText('Recover the original message recipient', { exact: true }).waitFor();
    const original = page.locator('.win').filter({ has: page.getByText('Recover the original message recipient', { exact: true }) });
    ok('recovery reviews the original owner offline and offers saved-history continuation',
      await original.locator('[data-view-id="session-recover"]').isEnabled() &&
      await page.evaluate(() => !__calls.some(c => c[0] === 'open' && c[2] === 'codex:older')));
    await original.locator('[data-view-id="session-recover"]').click();
    await page.locator('dialog.recovery-dialog[open]').getByLabel('Provider', { exact: true }).selectOption('codex');
    await page.waitForFunction(() => !document.querySelectorAll('dialog.recovery-dialog[open] select')[1].disabled);
    await page.locator('dialog.recovery-dialog[open]').getByLabel('Model', { exact: true }).selectOption('fixture-model');
    await page.getByRole('button', { name: 'Review Recovery', exact: true }).click();
    await page.locator('dialog.switch-confirm[open]').getByRole('button', { name: 'Continue', exact: true }).waitFor();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'preview-recovery'));
    ok('the original owner, provider and model are explicit before allocation', await page.evaluate(() => __calls.some(c => c[0] === 'preview-recovery' && c[1] === 'codex:older' && c[2] === 'codex' && c[3] === 'fixture-model') && !__calls.some(c => c[0] === 'recover')));
    await page.keyboard.press('Escape');
    await page.screenshot({ path: require('path').join(t.out || require('./lib/page').OUT, 'waiting-chat-recovery.png'), fullPage: true });
  } finally { await t.close(); }
})();
