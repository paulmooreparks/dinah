// Recovering a chat that could not reopen: the dialog says which chat it
// is, lists the agents itself, and says whether it is still looking, found
// none ready, or could not look, and why Review Recovery cannot go on. A
// ready provider and a model are chosen by the person, by keyboard too;
// Cancel and Escape send nothing, and Review Recovery reviews the choice.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    state: { workspace: 'C:/Fixture', recent: ['C:/Fixture'] },
    app: () => {
      window.__calls = [];
      window.__mode = 'gate';
      let release; window.__gate = new Promise(r => { release = r; }); window.__release = release;
      const ready = [{ id: 'claude', name: 'Claude', installed: true, available: true }, { id: 'codex', name: 'Codex', installed: true, available: true }];
      return {
        Notifications: async () => [{ id: 'waiting_conflict', kind: 'delivery-failure', severity: 'error', title: 'Messages are waiting for Team Lead',
          body: 'Recover the original saved chat to retain its recipient.', details: [], sessionId: 'claude:missing', sessionTitle: 'Team Lead', originTitle: 'Team Lead', originProvider: 'codex', originStarted: '2026-10-30T09:15:00Z', workspace: 'C:/Fixture', provider: 'codex',
          lead: 'codex:older', retryAllowed: false, created: '2026-01-01T00:00:00Z' }],
        OpenSessionAt: async () => { throw new Error('Resource not found: actual target'); },
        SavedConversation: async () => ({ available: true, complete: true, messages: [{ role: 'agent', text: 'The saved history remains readable.' }] }),
        SessionContinuation: async () => ({ history: [] }),
        AgentProviders: async () => {
          __calls.push(['providers', __mode]);
          if (__mode === 'gate') { await __gate; return ready; }
          if (__mode === 'error') { __mode = 'ok'; throw new Error('The agent scan timed out.'); }
          if (__mode === 'none') return [{ id: 'claude', name: 'Claude', installed: false, statusWords: 'Not Installed' }, { id: 'codex', name: 'Codex', installed: false, statusWords: 'Signed Out' }];
          return ready;
        },
        ProviderModelPreferences: async provider => ({ provider, catalog: [{ id: 'model', category: 'model', type: 'select', currentValue: 'fixture-model', options: [{ value: 'fixture-model', name: 'Fixture Model' }, { value: 'other-model', name: 'Other Model' }] }], defaults: {}, pinned: [] }),
        PreviewSessionRecoveryModel: async (id, provider, model) => { __calls.push(['preview', id, provider, model]); return { revision: 'r1', warnings: [], facts: { kind: 'provider', from: {}, to: { providerName: provider } } }; },
        RecoverSessionModel: async () => { __calls.push(['recover']); throw new Error('The fixture never confirms allocation.'); },
      };
    },
    ready: () => document.querySelector('[data-notification-id="waiting_conflict"]'),
  });
  const { page: p, ok } = t;
  const dialog = p.locator('dialog.recovery-dialog[open]');
  const view = () => p.evaluate(() => {
    const d = document.querySelector('dialog.recovery-dialog[open]');
    if (!d) return null;
    const sel = n => d.querySelector('#' + d.querySelector('label.form-label:nth-of-type(1)') && n);
    const [provider, model] = d.querySelectorAll('select.form-select'), next = [...d.querySelectorAll('button')].find(b => b.textContent === 'Review Recovery');
    const why = d.querySelector('.recovery-why'), notice = d.querySelector('.notice:not([hidden])');
    const labels = [...d.querySelectorAll('label.form-label')].map(l => ({ text: l.textContent, for: l.htmlFor, gap: document.getElementById(l.htmlFor).getBoundingClientRect().top - l.getBoundingClientRect().bottom }));
    return { title: d.querySelector('.dialog-title').textContent, chat: d.querySelector('.recovery-chat').textContent, labels,
      provider: { value: provider.value, disabled: provider.disabled, options: [...provider.options].map(o => o.textContent + (o.disabled ? ' [off]' : '')), wrapped: provider.parentElement.classList.contains('form-select-wrap') },
      model: { value: model.value, disabled: model.disabled, options: [...model.options].map(o => o.textContent) },
      next: next.disabled, why: why.hidden ? '' : why.textContent, notice: notice ? notice.className + ': ' + notice.textContent : '', busy: d.getAttribute('aria-busy'), focus: document.activeElement.id };
  });
  async function openRecovery() {
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await p.locator('[data-notification-id="waiting_conflict"]').getByRole('button', { name: 'Recover Original Chat', exact: true }).click();
    const original = p.locator('.win').filter({ has: p.getByText('Recover the original message recipient', { exact: true }) });
    await original.locator('[data-view-id="session-recover"]').click();
    await dialog.waitFor();
  }
  try {
    // Still looking for agents.
    await openRecovery();
    let s = await view();
    ok('while the agents are listed, it says so and Review Recovery says why it waits', s.provider.disabled && /Looking for agents/.test(s.provider.options[0]) && s.next && /Wait while the agents are listed/.test(s.why) && s.busy === 'true', JSON.stringify(s));
    ok('each field has a PUDL label above it, apart from it, and a wrapped select', s.labels.length === 2 && s.labels.every(l => l.for && l.gap >= 2) && s.provider.wrapped, JSON.stringify(s.labels));
    await t.shot('recovery-dialog-loading');
    await p.evaluate(() => __release());
    await p.waitForFunction(() => { const d = document.querySelector('dialog.recovery-dialog[open]'); return d && !d.querySelector('select').disabled && !d.querySelectorAll('select')[1].disabled; });
    s = await view();
    ok('the dialog says which chat it recovers, by title, workspace, provider and start', /Team Lead/.test(s.title) && /Team Lead · Fixture · Codex · started/.test(s.chat), JSON.stringify(s));
    ok('the agents listed, the chat\'s own ready provider is chosen and its models offered', s.provider.value === 'codex' && s.provider.options.includes('Claude') && s.model.options.includes('Fixture Model') && !s.next && !s.why, JSON.stringify(s));
    await t.shot('recovery-dialog-populated');
    // Keyboard: choose a model, then Escape cancels with nothing sent.
    ok('focus starts in the Provider field', /provider/.test(s.focus), s.focus);
    await p.keyboard.press('Tab');
    await p.keyboard.press('ArrowDown');
    await p.keyboard.press('Escape');
    await dialog.waitFor({ state: 'detached' });
    ok('Escape cancels, sending nothing', await p.evaluate(() => !__calls.some(c => c[0] === 'preview' || c[0] === 'recover')));
    // Could not list the agents, then Look Again.
    await p.evaluate(() => { __mode = 'error'; });
    await p.locator('.win').filter({ has: p.getByText('Recover the original message recipient', { exact: true }) }).locator('[data-view-id="session-recover"]').click();
    await dialog.waitFor();
    await p.waitForFunction(() => /Could Not Be Listed/.test((document.querySelector('dialog.recovery-dialog[open] .notice.danger:not([hidden])') || {}).textContent || ''));
    s = await view();
    ok('a failed listing says so with the error, and Review Recovery says why', /The Agents Could Not Be Listed/.test(s.notice) && /timed out/.test(s.notice) && s.next && /could not be listed/.test(s.why), JSON.stringify(s));
    await t.shot('recovery-dialog-error');
    await dialog.getByRole('button', { name: 'Look Again', exact: true }).click();
    await p.waitForFunction(() => { const d = document.querySelector('dialog.recovery-dialog[open]'); return d && d.querySelector('select').value === 'codex' && !d.querySelectorAll('select')[1].disabled; });
    ok('Look Again lists the agents', !(await view()).next);
    await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
    await dialog.waitFor({ state: 'detached' });
    // No agent ready.
    await p.evaluate(() => { __mode = 'none'; });
    await p.locator('.win').filter({ has: p.getByText('Recover the original message recipient', { exact: true }) }).locator('[data-view-id="session-recover"]').click();
    await p.waitForFunction(() => /No Agent Is Ready/.test((document.querySelector('dialog.recovery-dialog[open] .notice.warn:not([hidden])') || {}).textContent || ''));
    s = await view();
    ok('with no agent ready, it says which and why, and how to set one up', /No Agent Is Ready/.test(s.notice) && /Codex: Signed Out/.test(s.notice) && /Settings/.test(s.notice) && s.next && /No agent is ready/.test(s.why), JSON.stringify(s));
    await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
    await dialog.waitFor({ state: 'detached' });
    // Review by keyboard: another provider and a model, then Enter.
    await p.evaluate(() => { __mode = 'ok'; });
    await p.locator('.win').filter({ has: p.getByText('Recover the original message recipient', { exact: true }) }).locator('[data-view-id="session-recover"]').click();
    await p.waitForFunction(() => { const d = document.querySelector('dialog.recovery-dialog[open]'); return d && !d.querySelectorAll('select')[1].disabled; });
    await dialog.getByLabel('Provider', { exact: true }).selectOption('claude');
    await p.waitForFunction(() => { const d = document.querySelector('dialog.recovery-dialog[open]'); return d && !d.querySelectorAll('select')[1].disabled && d.querySelector('select').value === 'claude'; });
    await dialog.getByLabel('Model', { exact: true }).selectOption('other-model');
    await dialog.getByLabel('Model', { exact: true }).focus();
    await p.keyboard.press('Tab'); await p.keyboard.press('Tab');
    ok('Tab goes from Model to Cancel to Review Recovery', await p.evaluate(() => document.activeElement.textContent) === 'Review Recovery');
    await p.keyboard.press('Enter');
    await p.locator('dialog.switch-confirm[open]').waitFor();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'preview'));
    ok('Enter reviews the chosen provider and model, with nothing recovered yet', await p.evaluate(() => __calls.some(c => c[0] === 'preview' && c[1] === 'codex:older' && c[2] === 'claude' && c[3] === 'other-model') && !__calls.some(c => c[0] === 'recover')));
    await p.keyboard.press('Escape');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
