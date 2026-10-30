const { launch } = require('./lib/page');
const { choose, shows } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    app: () => {
      const modelOptions = (provider, current) => [{ id: 'model', category: 'model', type: 'select', currentValue: current, options: [
        { value: provider + '-old', name: provider + ' Original Model' },
        { value: provider + '-new', name: provider + ' New Default' }
      ] }];
      window.__calls = [];
      let defaultProvider = localStorage.getItem('fixture-default-provider') || 'devin';
      return {
        State: async () => ({ workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider, catalog: modelOptions(defaultProvider, defaultProvider + '-new'), defaults: { model: defaultProvider + '-new' }, pinned: [] }),
        AgentProviders: async () => ['devin', 'codex'].map(id => ({ id, name: { devin: 'Devin', codex: 'Codex' }[id], installed: true, available: true })),
        AgentSignIn: async id => { window.__calls.push(['signin', id]); },
        SetDefaultProvider: async id => { defaultProvider = id; localStorage.setItem('fixture-default-provider', id); },
        Sessions: async () => [{ sessionId: 'saved-devin', provider: 'devin', title: 'Earlier Devin Chat', cwd: 'C:/Test', open: false }],
        SkillsForProviderAt: async (ws, id) => { window.__calls.push(['skills', id]); return [{ name: id + '-skill', source: 'workspace' }]; },
        OpenSessionAt: async (ws, id) => {
          window.__calls.push(['open', id]);
          const provider = id === 'saved-codex' ? 'codex' : 'devin';
          // Options precede metadata, as they can during asynchronous creation or replay.
          window.__emit('agent:options', { sessionId: id, options: modelOptions(provider, provider + '-old') });
          if (window.__holdOpen) await new Promise(resolve => { window.__finishOpen = resolve; });
          return { sessionId: id, provider, title: provider + ' Existing Chat', cwd: ws, open: true };
        },
        NewSessionWithProviderAt: async (ws, provider) => {
          window.__calls.push(['new', provider]);
          const id = provider + '-fresh';
          window.__emit('agent:options', { sessionId: id, options: modelOptions(provider, provider + '-new') });
          return { sessionId: id, provider, title: 'New Chat', cwd: ws, open: true };
        },
        SetSessionOption: async (id, option, value) => { window.__calls.push(['option', id, value]); return modelOptions(id.includes('codex') ? 'codex' : 'devin', value); },
        Send: async (id, text) => { window.__calls.push(['send', id, text]); }
      };
    },
    ready: () => window.dinahWorkbench && document.querySelector('#default-provider')?.value === 'devin',
  });
  const { page, ok } = t;
  try {
    const front = () => page.locator('.win.active');
    // The provider and the model are menus in the composer's toolbar, each
    // showing the one in effect.
    const modelShows = words => front().locator('[data-command="model"] .toolbar-words').filter({ hasText: new RegExp('^' + words + '$') }).first().waitFor({ state: 'attached' });
    const provider = name => shows(front(), 'provider', name);
    const offered = (id, yes) => front().locator('[data-command="' + id + '"]' + (yes ? ':not([hidden])' : '[hidden]')).first()
      .waitFor({ state: 'attached', timeout: 3000 }).then(() => true, () => false);
    await page.evaluate(() => { window.__holdOpen = true; return dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'saved-codex' }); });
    await page.waitForFunction(() => typeof __finishOpen === 'function');
    ok('reopening does not display the application default model during replay', await offered('model', false));
    await page.evaluate(() => __finishOpen());
    await modelShows('codex Original Model');
    ok('existing chat restores its provider and previous model despite changed defaults', await provider('Codex'));
    ok('late session metadata refreshes the status provider without a connection event', await page.locator('#status-agent .status-words').textContent() === 'Codex');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'skills' && c[1] === 'codex'));
    ok('late provider metadata reloads the correct skills', await page.evaluate(() => __calls.some(c => c[0] === 'skills' && c[1] === 'codex')));
    const restoredURL = page.url();
    await page.evaluate(() => dinahWorkbench.open('chat'));
    await modelShows('devin New Default');
    ok('early replay options cannot overwrite the other provider catalog', await provider('Devin'));
    await choose(page, front(), 'provider', 'Codex');
    ok('the reopened provider catalog becomes available after metadata arrives', await offered('model', true));
    await page.evaluate(() => dinahWorkbench.open('settings'));
    await page.locator('#settings').getByRole('tab', { name: 'Agents', exact: true }).click();
    await page.locator('#default-provider').selectOption('codex');
    await page.waitForFunction(() => !document.querySelector('#default-provider').disabled);
    await page.goto(restoredURL);
    await modelShows('codex Original Model');
    ok('a restored workbench window keeps its previous model even when its provider is now the default', await provider('Codex'));
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    await page.locator('#workspace-chats [data-session="saved-devin"]').click();
    await modelShows('devin Original Model');
    ok('a chat reopened from Workspace remains Devin when the default is Codex', await provider('Devin'));
    ok('the focused chat controls the status provider', await page.locator('#status-agent .status-words').textContent() === 'Devin');
    ok('reopening never creates a new session or reapplies a default model', await page.evaluate(() => !__calls.some(c => ['new', 'option', 'send'].includes(c[0]))));
    await page.evaluate(() => dinahWorkbench.open('settings'));
    ok('Settings focus returns the status provider to the default agent', await page.locator('#status-agent .status-words').textContent() === 'Codex');
    await page.locator('#status-agent').click();
    await page.locator('#agent-menu [data-cmd="sign-in"]').click();
    await page.waitForFunction(() => __calls.some(call => call[0] === 'signin'));
    ok('the status sign-in command targets the provider shown in its menu', await page.evaluate(() => __calls.some(call => call[0] === 'signin' && call[1] === 'codex')));
    await page.evaluate(() => dinahWorkbench.open('chat'));
    await modelShows('codex New Default');
    await front().locator('[data-view-id="prompt"]').fill('New conversation fixture');
    await front().locator('[data-view-id="prompt"]').press('Control+Enter');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'send'));
    ok('new conversations still use the configured default provider and model', await provider('Codex') && await shows(front(), 'model', 'codex New Default') && await page.evaluate(() => __calls.some(c => c[0] === 'new' && c[1] === 'codex')));
    await page.evaluate(() => __emit('agent:options', { sessionId: 'codex-fresh', options: [], error: 'The selected model could not be saved.' }));
    // The error is a toast and, as every toast is, an entry under Notifications.
    await page.locator('.toast').getByText('The selected model could not be saved.', { exact: true }).waitFor();
    ok('asynchronous option persistence failures are visible', await page.locator('.toast').getByText('The selected model could not be saved.', { exact: true }).isVisible() &&
      await page.locator('#questions .notification-report').getByText('The selected model could not be saved.', { exact: true }).count() === 1);
  } finally { await t.close(); }
})();
