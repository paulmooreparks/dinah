const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1100 },
    app: () => {
      let cap = 65536;
      window.__memoryMode = 'matching';
      window.__memoryReads = 0;
      return {
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'], defaultProvider: 'ollama', catalog: [], defaults: {}, pinned: [],
          ollamaApprovals: 'chat', ollamaFileScope: 'workspace', ollamaContextCap: cap }),
        AgentProviders: async () => [{ id: 'ollama', name: 'Ollama', installed: true, available: true, status: 'detected' }],
        SetOllamaContextCap: async n => { cap = n || 65536; },
        OllamaMemory: async () => {
          window.__memoryReads++;
          if (window.__memoryMode === 'failed') throw Error('Fixture offline');
          if (window.__memoryMode === 'empty') return { limit: cap, models: [] };
          if (window.__memoryMode === 'late') return new Promise(resolve => { window.__oldReading = () => resolve({ limit: 8192, models: [{ name: 'late:4b', context: 8192, targetContext: 8192, size: 1073741824, sizeVRAM: 0 }] }); });
          return { limit: cap, models: [{ name: 'fixture:4b', digest: 'fixture', context: window.__memoryMode === 'other' ? 8192 : cap,
            targetContext: cap, size: 3221225472, sizeVRAM: 2147483648 }] };
        },
        SkillsForProviderAt: async () => [], SkillsAt: async () => []
      };
    }
  });
  const { page, ok } = t;
  try {
    await page.evaluate(() => dinahWorkbench.open('settings'));
    const settings = page.locator('#settings');
    await settings.getByRole('tab', { name: 'Agents', exact: true }).click();
    const reading = settings.locator('#ollama-memory-reading'), refresh = settings.locator('#ollama-memory-refresh');
    await page.waitForFunction(() => document.querySelector('#ollama-memory-reading')?.textContent.includes('3.00 GiB'));
    ok('the current model shows reported total and GPU memory at the saved ceiling', /fixture:4b at 65,536 tokens reports 3.00 GiB loaded, including 2.00 GiB/.test(await reading.innerText()) && /size used at the saved ceiling/.test(await reading.innerText()));
    const dimensions = async () => reading.evaluate(el => { const r = el.getBoundingClientRect(); return [r.width, r.height]; });
    const before = await dimensions();
    await page.evaluate(() => { window.__memoryMode = 'other'; });
    await refresh.click();
    await page.waitForFunction(() => document.querySelector('#ollama-memory-reading').textContent.includes('has not been measured'));
    ok('a reading at another context is labelled without an estimate', /8,192 tokens/.test(await reading.innerText()) && /65,536 tokens has not been measured/.test(await reading.innerText()));
    await page.evaluate(() => { window.__memoryMode = 'empty'; });
    await refresh.click();
    await page.waitForFunction(() => document.querySelector('#ollama-memory-reading').textContent.startsWith('No local model'));
    ok('an unloaded model has no invented memory figure', !/GiB/.test(await reading.innerText()));
    await page.evaluate(() => { window.__memoryMode = 'failed'; });
    await refresh.click();
    await page.waitForFunction(() => document.querySelector('#ollama-memory-reading').textContent.includes('could not be read'));
    ok('an unavailable server has a visible explanation', /could not be read/.test(await reading.innerText()));
    ok('readings and failures keep the same space', JSON.stringify(before) === JSON.stringify(await dimensions()));
    await page.evaluate(() => { window.__memoryMode = 'late'; });
    await refresh.click();
    await page.waitForFunction(() => typeof window.__oldReading === 'function');
    await page.evaluate(() => { window.__memoryMode = 'matching'; });
    await settings.locator('#ollama-context-cap').fill('32768');
    await settings.locator('#ollama-context-save').click();
    await page.waitForFunction(() => document.querySelector('#ollama-memory-reading').textContent.includes('32,768 tokens'));
    await page.evaluate(() => window.__oldReading());
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    ok('a late response cannot replace a reading for the new ceiling', /32,768 tokens/.test(await reading.innerText()) && !/late:4b/.test(await reading.innerText()));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
