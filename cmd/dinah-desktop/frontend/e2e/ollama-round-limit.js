// Settings > Agents > Ollama Steps per Turn: the round limit the person sets,
// empty for none, saved and cleared with its status said (provider-ollama.js).
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__calls = [];
      const state = { limit: 60, chosen: true };
      return {
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'], defaultProvider: 'ollama', catalog: [], defaults: {}, pinned: [],
          ollamaApprovals: 'chat', ollamaFileScope: 'anywhere', ollamaFileFolders: [],
          ollamaRoundLimit: { limit: state.limit, chosen: state.chosen } }),
        AgentProviders: async () => [{ id: 'ollama', name: 'Ollama', installed: true, available: true, status: 'detected' }],
        SetOllamaRoundLimit: async n => {
          window.__calls.push(n);
          if (window.__failSave) throw new Error('Fixture: the limit was refused');
          if (!Number.isInteger(n) || n < 0) throw new Error('the limit is a whole number of steps');
          state.limit = n; state.chosen = n > 0;
        },
        SkillsForProviderAt: async () => [], SkillsAt: async () => []
      };
    },
  });
  const { page, ok } = t;
  try {
    await page.evaluate(() => dinahWorkbench.open('settings'));
    const settings = page.locator('#settings');
    await settings.getByRole('tab', { name: 'Agents', exact: true }).click();
    const field = settings.locator('#ollama-round-limit');
    const save = settings.locator('#ollama-round-save');
    const clear = settings.locator('#ollama-round-clear');
    const status = settings.locator('#ollama-round-status');
    await field.waitFor();
    await page.waitForFunction(() => document.querySelector('#ollama-round-limit').value === '60');
    ok('a chosen limit shows in the field, and No Limit is offered', await clear.isEnabled());
    ok('and the note says the turn stops at the count with Continue offered', /turn stops at that count, the chat says so, and it offers Continue/.test(await settings.locator('.form-help', { hasText: 'counted in steps' }).textContent()));

    await field.fill('120');
    await save.click();
    await page.waitForFunction(() => window.__calls[0] === 120);
    ok('a whole number saves and says what happens at it', /stops after 120 steps, and the chat offers to continue/.test(await status.textContent()), await status.textContent());

    await clear.click();
    await page.waitForFunction(() => window.__calls[1] === 0);
    await page.waitForFunction(() => document.querySelector('#ollama-round-limit').value === '');
    ok('No Limit clears the field and saves none', /runs until the model finishes/.test(await status.textContent()) && await clear.isDisabled());

    await field.fill('12.5');
    await save.click();
    ok('a fraction is refused before any save', /whole number of steps/.test(await status.textContent()) && (await page.evaluate(() => window.__calls.length)) === 2);

    await field.fill('90');
    await page.evaluate(() => { window.__failSave = true; });
    await save.click();
    await page.waitForFunction(() => /could not be saved/.test(document.querySelector('#ollama-round-status').textContent));
    // As with every Settings save that is refused, the field returns to the
    // saved value rather than keeping the typed one.
    ok('a refused save says so and the field returns to the saved value', /The limit could not be saved/.test(await status.textContent()) && (await page.evaluate(() => document.querySelector('#ollama-round-limit').value)) === '' && (await page.evaluate(() => window.__calls.length)) === 3, await status.textContent());
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
