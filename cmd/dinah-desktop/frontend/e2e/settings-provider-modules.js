// Each provider's own part of Settings, Agents is mounted only for a provider the engine's list has,
// in the list's order, so the shared page draws nothing for a provider the product does not offer.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    app: () => {
      window.__providers = [{ id: 'codex', name: 'Codex', installed: true, available: true, status: 'detected' }];
      return {
        AgentProviders: async () => window.__providers,
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'], defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [], codexApprovals: 'workspace', ollamaApprovals: 'all' })
      };
    },
  });
  const { page, ok } = t;
  try {
    await page.evaluate(() => dinahWorkbench.open('settings'));
    const settings = page.locator('#settings');
    await settings.getByRole('tab', { name: 'Agents', exact: true }).click();
    await settings.locator('#codex-approvals').waitFor();
    const has = id => settings.locator('#' + id).count();
    ok('a provider in the list gets its own part of Settings', await has('codex-approvals') === 1 && await has('codex-applet-approvals') === 1);
    ok('a provider not in the list gets none', await has('ollama-approvals') === 0 && await has('ollama-file-scope') === 0 && await has('usage-save') === 0 && await has('claude-api-key') === 0);
    ok('and it shows what State says', await settings.locator('#codex-approvals [aria-checked="true"]').getAttribute('data-value') === 'workspace');

    // A provider that joins the list is mounted, in the list's order, with the saved values it already has.
    await page.evaluate(() => {
      window.__providers = [
        { id: 'ollama', name: 'Ollama', installed: true, available: true, status: 'detected' },
        { id: 'codex', name: 'Codex', installed: true, available: true, status: 'detected' }];
    });
    await settings.locator('#scan-agents').click();
    await settings.locator('#ollama-approvals').waitFor();
    ok('a provider that joins the list is mounted with the value State gave', await settings.locator('#ollama-approvals [aria-checked="true"]').getAttribute('data-value') === 'all');
    const order = await settings.locator('#provider-modules > [data-provider-module]').evaluateAll(els => els.map(e => e.dataset.providerModule));
    ok('the modules follow the list\'s order', order.join(',') === 'ollama,codex', order.join(','));

    // One that leaves it is taken away.
    await page.evaluate(() => { window.__providers = [{ id: 'ollama', name: 'Ollama', installed: true, available: true, status: 'detected' }]; });
    await settings.locator('#scan-agents').click();
    await page.waitForFunction(() => !document.querySelector('#codex-approvals'));
    ok('a provider that leaves the list loses its part of Settings', await has('codex-approvals') === 0 && await has('ollama-approvals') === 1);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
