const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__caps = [];
      let cap = 65536;
      return {
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'], defaultProvider: 'ollama', catalog: [], defaults: {}, pinned: [],
          ollamaApprovals: 'chat', ollamaFileScope: 'workspace', ollamaContextCap: cap }),
        AgentProviders: async () => [{ id: 'ollama', name: 'Ollama', installed: true, available: true, status: 'detected' }],
        SetOllamaContextCap: async n => { window.__caps.push(n); if (window.__rejectCap) throw Error('Fixture refusal'); cap = n || 65536; },
        SkillsForProviderAt: async () => [], SkillsAt: async () => []
      };
    }
  });
  const { page, ok } = t;
  try {
    await page.evaluate(() => dinahWorkbench.open('settings'));
    const settings = page.locator('#settings');
    await settings.getByRole('tab', { name: 'Agents', exact: true }).click();
    const input = settings.locator('#ollama-context-cap');
    await input.waitFor();
    await page.waitForFunction(() => document.querySelector('#ollama-context-cap').value === '65536');
    ok('the default cap is shown under Ollama', await input.inputValue() === '65536');
    await input.fill('32768');
    await settings.locator('#ollama-context-save').click();
    await page.waitForFunction(() => window.__caps[0] === 32768 && /Saved/.test(document.querySelector('#ollama-context-status').textContent));
    ok('a smaller cap saves with its effect explained', /next model response/.test(await settings.locator('#ollama-context-status').innerText()));
    await settings.locator('#ollama-context-reset').click();
    await page.waitForFunction(() => window.__caps[1] === 0 && document.querySelector('#ollama-context-cap').value === '65536');
    ok('Use Default restores 65536', await input.inputValue() === '65536');
    await input.fill('1023');
    await settings.locator('#ollama-context-save').click();
    ok('invalid sizes are refused before saving', await page.evaluate(() => window.__caps.length === 2));
    await input.fill('8192');
    await page.evaluate(() => { window.__rejectCap = true; });
    await settings.locator('#ollama-context-save').click();
    await page.waitForFunction(() => /could not be saved/.test(document.querySelector('#ollama-context-status').textContent));
    ok('a refused save is visible', /Fixture refusal/.test(await settings.locator('#ollama-context-status').innerText()));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
