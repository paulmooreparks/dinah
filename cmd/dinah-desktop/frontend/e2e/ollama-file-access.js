// Settings > Agents > Ollama File Access: where Ollama's file tools may go. The choice
// and its folders are saved together; a refusal or a failure puts the control back.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__calls = [];
      const state = { scope: 'anywhere', folders: [] };
      return {
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'], defaultProvider: 'ollama', catalog: [], defaults: {}, pinned: [], approvals: 'devin',
          ollamaApprovals: 'chat', ollamaFileScope: state.scope, ollamaFileFolders: state.folders.slice() }),
        AgentProviders: async () => [{ id: 'ollama', name: 'Ollama', installed: true, available: true, status: 'detected' }],
        SetOllamaFileScope: async (scope, folders) => {
          window.__calls.push([scope, folders.slice()]);
          if (window.__failSave) throw new Error('Fixture: the choice was refused');
          if (!['anywhere', 'workspace', 'folders'].includes(scope)) throw new Error('unknown scope');
          state.scope = scope; state.folders = folders.slice();
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
    const scope = settings.locator('#ollama-file-scope');
    const checked = () => scope.locator('[aria-checked="true"]').getAttribute('data-value');
    const folders = settings.locator('#ollama-file-folders-group');
    const status = settings.locator('#ollama-file-status');
    await scope.waitFor();
    ok('an existing install shows Anywhere, with no folder list', await checked() === 'anywhere' && await folders.isHidden());
    ok('and says what that means', /wherever you can/.test(await settings.locator('#ollama-file-describe').textContent()));
    ok('the note says commands are not confined and a limit does not make them ask', /does not confine what a command reads once it runs/.test(await settings.locator('.form-help', { hasText: 'confines Ollama' }).textContent()));

    await scope.getByRole('radio', { name: 'Workspace Folders', exact: true }).click();
    await page.waitForFunction(() => window.__calls.length === 1);
    ok('choosing Workspace Folders saves the choice with no folders', JSON.stringify(await page.evaluate(() => window.__calls[0])) === JSON.stringify(['workspace', []]));
    await status.filter({ hasText: 'Saved' }).waitFor();
    ok('it is checked, described, and the folder list stays hidden', await checked() === 'workspace' && /workspace folder/.test(await settings.locator('#ollama-file-describe').textContent()) && await folders.isHidden());

    await scope.getByRole('radio', { name: 'Workspace Folders and These', exact: true }).click();
    await folders.waitFor();
    ok('choosing the third shows the folder list, empty', await settings.locator('#ollama-file-folders').innerText() === 'No other folders.');
    await settings.locator('#ollama-file-folder-add').click();
    ok('adding nothing says so and saves nothing', await status.innerText() === 'Type the full path of a folder to add.' && (await page.evaluate(() => window.__calls.length)) === 2);
    await settings.locator('#ollama-file-folder-input').fill('C:/Data');
    await settings.locator('#ollama-file-folder-add').click();
    await page.waitForFunction(() => window.__calls.length === 3);
    ok('adding a folder saves the choice with the folder', JSON.stringify(await page.evaluate(() => window.__calls[2])) === JSON.stringify(['folders', ['C:/Data']]));
    await settings.locator('#ollama-file-folders li', { hasText: 'C:/Data' }).waitFor();
    ok('and lists it with a Remove', await settings.getByRole('button', { name: 'Remove C:/Data' }).isVisible());
    await settings.locator('#ollama-file-folder-input').fill('C:/Data');
    await settings.locator('#ollama-file-folder-add').click();
    ok('adding the same folder again says so and saves nothing', (await status.innerText()) === 'That folder is already listed.' && (await page.evaluate(() => window.__calls.length)) === 3);

    await settings.getByRole('button', { name: 'Remove C:/Data' }).click();
    await page.waitForFunction(() => window.__calls.length === 4);
    ok('Remove saves the list without it', JSON.stringify(await page.evaluate(() => window.__calls[3])) === JSON.stringify(['folders', []]));

    // A save the engine refuses puts the control back as it was.
    await page.evaluate(() => { window.__failSave = true; });
    await scope.getByRole('radio', { name: 'Anywhere', exact: true }).click();
    await status.filter({ hasText: 'could not be saved' }).waitFor();
    ok('a refused save puts the choice back and says so', await checked() === 'folders' && /the choice was refused/.test(await status.innerText()));
    ok('and leaves the control usable', await scope.getByRole('radio', { name: 'Anywhere', exact: true }).isEnabled());
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
