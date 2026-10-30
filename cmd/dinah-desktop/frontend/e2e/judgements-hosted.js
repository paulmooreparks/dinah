// Judgement providers and their cost confirmation (judgements-applet.js,
// Engine's JudgementProviders): each provider named with whether it can judge
// now and why when it cannot, the plan note in its own words, the cost
// confirmed in numbers first, Cancel restoring the pick (Paul, Dinah UI).
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__settings = { enabled: true, provider: 'ollama', model: 'kimi-k3:cloud', retentionDays: 90, warnStreak: 3, kindsOff: [], budgets: [{ provider: 'ollama', unit: 'tokens', limit: null }, { provider: 'claude', unit: 'tokens', limit: null }], period: { weekday: 1, hour: 0 }, pool: { min: 1, max: 4 } };
      window.__revision = 'r1';
      window.__rev = 1;
      window.__calls = [];
      window.__providers = [
        { id: 'ollama', name: 'Ollama', unit: 'tokens', available: true, why: '', note: '', typicalCost: 0, calls: 0 },
        { id: 'claude', name: 'Claude Code', unit: 'tokens', available: true, why: '', note: 'Each call starts a session and uses your plan’s usage. Set a budget below.', typicalCost: 41000, calls: 3 },
      ];
      window.__models = {
        ollama: ['kimi-k3:cloud', 'deepseek-v4.1-flash:cloud'],
        claude: ['opus-max', 'sonnet-max'],
      };
      const kind = { name: 'chat-title', version: 'v1', describe: 'The title is the first line of your words, cleaned.', correctable: true, deadlineMs: 3000, enabled: true, input: [] };
      return {
        AgentProviders: async () => [{ id: 'ollama', name: 'Ollama', installed: true, available: true }, { id: 'claude', name: 'Claude Code', installed: true, available: true }],
        JudgementState: async () => ({ stateGen: 'g1', stateVer: 1, state: 'on', why: '', calls: 4, fallbacks: 0, streak: 0, warnStreak: 3, latest: new Date().toISOString(), budget: [], pool: { size: 1, min: 1, max: 4, growAt: 0.5, shrinkAfter: 10, inFlight: 0 }, calibration: { state: 'idle', chosen: '' } }),
        JudgementSummary: async () => ({ from: '', to: '', calls: 0, fallbacks: 0, groups: [], latest: new Date().toISOString(), streak: 0 }),
        JudgementRecords: async () => ({ records: [], next: 0 }),
        JudgementKinds: async () => [kind],
        JudgementSettings: async () => ({ settings: window.__settings, revision: window.__revision, conflict: false }),
        JudgementModels: async () => (window.__models[window.__settings.provider] || []).map(name => ({ name, provider: window.__settings.provider })),
        JudgementProviders: async () => window.__providers,
        SetJudgementSettings: async (settings, revision) => { window.__calls.push([settings, revision]); window.__settings = settings; window.__revision = 'r' + (++window.__rev); return { settings, revision: window.__revision, conflict: false }; },
        JudgementCalibration: async () => ({ state: 'idle', models: [], cases: [] }),
      };
    },
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahWorkbench.open('settings'));
    const settings = p.locator('.win[data-win-kind="settings"]');
    await settings.locator('[data-settings-section="judgements"]').click();
    await settings.locator('#judgement-provider').waitFor();
    ok('each provider is offered by name', (await settings.locator('#judgement-provider option').count()) === 2 && /Claude Code/.test(await settings.locator('#judgement-provider').textContent()));
    ok('the section names Ollama’s judge with no plan note', (await settings.locator('#judgement-provider').inputValue()) === 'ollama' && (await settings.locator('#judgement-provider-note').textContent()) === '');
    // Choose Claude Code: the provider is saved, the plan note shows, its models appear.
    await settings.locator('#judgement-provider').selectOption('claude');
    await p.waitForFunction(() => window.__calls.some(c => c[0].provider === 'claude'));
    await settings.locator('#judgement-model option', { hasText: 'opus-max' }).first().waitFor({ state: 'attached' });
    ok('its models are offered once it is the provider', (await settings.locator('#judgement-model option').allTextContents()).map(t => t.trim()).some(t => t.includes('opus-max')));
    ok('and the note says in its words what its calls are', /Each call starts a session and uses your plan/.test(await settings.locator('#judgement-provider-note').textContent()));
    // Choosing its model is confirmed with the measured cost, Cancel first.
    await settings.locator('#judgement-model').selectOption('');
    await settings.locator('#judgement-model').selectOption('opus-max');
    const conf = p.locator('dialog.judgement-model-confirm[open]');
    await conf.waitFor();
    ok('the cost is confirmed first, in numbers, with the calls it rests on', /A judgement on Claude Code has cost about 41,000 tokens a call \(the median of 3 calls\)/.test(await conf.textContent()) && /No budget is set/.test(await conf.textContent()), (await conf.textContent()).slice(0, 400));
    await t.shot('judgements-hosted-confirm');
    ok('Cancel comes first, Use last', (await conf.locator('.dialog-actions button').first().textContent()) === 'Cancel' && /Use Claude Code/.test(await conf.locator('.dialog-actions .btn-primary').textContent()));
    await conf.getByRole('button', { name: 'Cancel' }).click();
    await conf.waitFor({ state: 'detached' });
    ok('cancelling saves nothing and restores the pick', (await p.evaluate(() => window.__calls.some(c => c[0].model === 'opus-max'))) === false && (await settings.locator('#judgement-model').inputValue()) === '');
    // Confirming saves the model.
    await settings.locator('#judgement-model').selectOption('opus-max');
    await p.locator('dialog.judgement-model-confirm[open]').waitFor();
    await p.locator('dialog.judgement-model-confirm[open]').getByRole('button', { name: 'Use Claude Code' }).click();
    await p.waitForFunction(() => window.__calls.some(c => c[0].model === 'opus-max'));
    ok('choosing it saves it under the revision rule', true);
    // An unknown cost is said plainly, not guessed at.
    await p.evaluate(() => { window.__providers = window.__providers.map(x => x.id === 'claude' ? Object.assign({}, x, { typicalCost: 0, calls: 0 }) : x); __emit('settings:changed', {}); });
    // The list refreshes with its event, then the choice reads the fresh one.
    await p.waitForFunction(() => window.__bridgeCalls.filter(c => c[0] === 'JudgementProviders').length >= 2);
    await settings.locator('#judgement-model').selectOption('sonnet-max');
    await p.locator('dialog.judgement-model-confirm[open]').waitFor();
    ok('no calls yet says so, with no figure invented', /The cost of a call is not known yet\. Each call starts a session on your plan/.test(await p.locator('dialog.judgement-model-confirm[open]').textContent()));
    await p.locator('dialog.judgement-model-confirm[open]').getByRole('button', { name: 'Cancel' }).click();
    // A local model asks nothing.
    await settings.locator('#judgement-provider').selectOption('ollama');
    await p.waitForFunction(() => window.__calls.some(c => c[0].provider === 'ollama'));
    await settings.locator('#judgement-model option', { hasText: 'deepseek-v4.1-flash:cloud' }).first().waitFor({ state: 'attached' });
    const before = await p.evaluate(() => window.__calls.length);
    await settings.locator('#judgement-model').selectOption('deepseek-v4.1-flash:cloud');
    await p.waitForFunction(n => window.__calls.length > n && !document.querySelector('dialog.judgement-model-confirm[open]'), before);
    ok('a local model asks nothing', true);
    // A provider that cannot now says why.
    await p.evaluate(() => { window.__providers = window.__providers.map(x => x.id === 'claude' ? Object.assign({}, x, { available: false, why: 'it is not signed in' }) : x); __emit('settings:changed', {}); });
    await p.waitForFunction(() => /cannot judge now: it is not signed in/.test(document.getElementById('judgement-provider-note').textContent));
    ok('one that cannot judge now says why', true);
    // The note stands under that provider's budget row too.
    await p.evaluate(() => { window.__providers = window.__providers.map(x => x.id === 'claude' ? Object.assign({}, x, { available: true, why: '' }) : x); __emit('settings:changed', {}); });
    await settings.locator('.judgement-budget-edit[data-provider="claude"] input').waitFor();
    ok('the plan note stands with its budget row', await settings.locator('.judgement-budget-note').filter({ hasText: /Set a budget below/ }).count() === 1);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
