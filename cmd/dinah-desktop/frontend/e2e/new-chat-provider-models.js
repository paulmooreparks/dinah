// A new chat of a provider other than the default shows its Model control as soon
// as the provider is chosen. The page is given only the default provider's catalog
// at start, so the others are read from what the engine has cached.
const { launch } = require('./lib/page');
const { choose, shows } = require('./lib/commands');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__reads = [];
      const model = (provider, ...names) => [{ id: 'model', category: 'model', type: 'select', currentValue: names[0][0], options: names.map(n => ({ value: n[0], name: n[1] })) }];
      window.__cached = {
        claude: { provider: 'claude', catalog: model('claude', ['claude-one', 'Claude One']), defaults: {}, pinned: [] },
        ollama: { provider: 'ollama', catalog: model('ollama', ['deepseek-v4.1-flash:cloud', 'Deepseek Flash'], ['gpt-oss:120b-cloud', 'GPT OSS']), defaults: {}, pinned: [] },
        codex: { provider: 'codex', catalog: [], defaults: {}, pinned: [] },
        devin: { provider: 'devin', catalog: model('devin', ['devin-one', 'Devin One']), defaults: {}, pinned: [] }
      };
      const providers = ['claude', 'ollama', 'codex', 'devin'].map(id => ({ id, name: id[0].toUpperCase() + id.slice(1), installed: true, available: true, status: 'detected' }));
      return {
        // Only the default provider's catalog arrives with State, as in the real window.
        State: async () => ({ workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider: 'claude', catalog: window.__cached.claude.catalog, defaults: {}, pinned: [], approvals: 'devin' }),
        AgentProviders: async () => { window.__reads.push(['scan']); return providers; },
        ProviderModelPreferences: async provider => {
          window.__reads.push(['prefs', provider]);
          if (window.__failOnce && window.__failOnce.delete(provider)) throw new Error('Fixture: not available yet');
          return JSON.parse(JSON.stringify(window.__cached[provider]));
        },
        SkillsForProviderAt: async () => [],
        SkillsAt: async () => []
      };
    },
    // Ollama's cache cannot be read at start, so choosing it is what reads it.
    init: () => { window.__reads = []; window.__failOnce = new Set(['ollama']); },
    ready: () => window.__reads.some(c => c[0] === 'scan'),
  });
  const { page, ok } = t;
  const front = () => page.locator('.win.active');
  const reads = provider => page.evaluate(p => window.__reads.filter(c => c[0] === 'prefs' && c[1] === p).length, provider);
  try {
    await page.evaluate(() => dinahWorkbench.open('chat'));
    await front().locator('[data-view-id="prompt"]').waitFor();
    ok('a new chat starts on the default provider with its model', await shows(front(), 'provider', 'Claude') && await shows(front(), 'model', 'Claude One'));

    // The start reads the cache of every provider offered.
    ok('every provider offered is read at start', (await reads('devin')) >= 1 && (await reads('codex')) >= 1 && (await reads('claude')) >= 1 && (await reads('ollama')) === 1);
    await choose(page, front(), 'provider', 'Devin');
    ok('choosing a provider read at start shows its Model control', await shows(front(), 'model', 'Devin One'));
    await choose(page, front(), 'provider', 'Ollama');
    ok('choosing Ollama, whose cache could not be read at start, reads it then and shows the cached models', await shows(front(), 'model', 'Deepseek Flash') && (await reads('ollama')) === 2);
    await choose(page, front(), 'provider', 'Claude');
    ok('and choosing the default again shows its own', await shows(front(), 'model', 'Claude One'));

    // A read that fails is quiet, and a resync reads the providers again.
    await page.evaluate(() => { window.__failOnce = new Set(['ollama']); window.__reads.length = 0; });
    await page.evaluate(() => window.__resync());
    await page.waitForFunction(() => window.__reads.some(c => c[0] === 'prefs' && c[1] === 'ollama'));
    ok('a failed read is quiet', await page.evaluate(() => !document.body.textContent.includes('Fixture: not available yet')));
    await page.evaluate(() => { window.__cached.ollama.catalog[0].options = [{ value: 'new-model', name: 'New Model' }]; window.__cached.ollama.catalog[0].currentValue = 'new-model'; });
    await page.evaluate(() => window.__resync());
    await page.waitForFunction(() => window.__reads.filter(c => c[0] === 'prefs' && c[1] === 'ollama').length >= 2);
    await choose(page, front(), 'provider', 'Ollama');
    ok('a resync reads the providers again', await shows(front(), 'model', 'New Model'));

    // A provider with an empty cached catalog has nothing to choose, and says so by showing no Model control.
    await choose(page, front(), 'provider', 'Codex');
    ok('a provider with no cached models shows no Model control', await front().locator('[data-command="model"]').count() === 0 || !(await front().locator('[data-command="model"]').first().isVisible()));
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
