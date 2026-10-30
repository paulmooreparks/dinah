const { launch } = require('./lib/page');
const { choose, shows } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      let provider = 'codex';
      const catalog = [{ id: 'model', category: 'model', type: 'select', currentValue: 'codex-model', options: [{ value: 'codex-model', name: 'Codex Model' }] }];
      const devinCatalog = [{ id: 'model', category: 'model', type: 'select', currentValue: 'devin-model', options: [{ value: 'devin-model', name: 'Devin Model' }] }];
      const providers = [
        window.__provider('devin', { installed: false, available: false, status: 'choose', detail: 'More than one Devin command line works.',
          choices: [{ path: 'C:/Old/devin.exe', release: 'devin 2026.4.15-0' }, { path: 'C:/Users/p/AppData/Local/devin/cli/bin/devin.exe', release: 'devin 3000.10.48 (7e8e528a)' }],
          likely: 'C:/Users/p/AppData/Local/devin/cli/bin/devin.exe' }),
        window.__provider('codex', { status: 'not_checked', path: 'C:/Tools/codex.exe' }),
        window.__provider('claude', { installed: false, available: false, status: 'adapter-missing', statusWords: 'Not Set Up', path: 'C:/Program Files/nodejs/node.exe', detail: 'Node.js was found. Install the Claude adapter 0.87.0 to use Claude.',
          claude: { adapterVersion: '0.87.0', adapterInstalled: false, canInstall: true, apiKey: false } }),
        window.__provider('ollama', { installed: false, available: false, status: 'not-running', statusWords: 'Not Running', path: 'http://localhost:11434', detail: 'Ollama did not answer at http://localhost:11434.' })
      ];
      window.__installDevin = () => { providers[0].installed = true; providers[0].available = true; };
      return {
        State: async () => ({ workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider: provider, catalog: provider === 'codex' ? catalog : devinCatalog, defaults: {}, pinned: [], approvals: 'devin' }),
        AgentProviders: async () => { window.__calls.push(['scan']); return providers; },
        AgentCheck: async id => { window.__calls.push(['check', id]); return { ...providers.find(p => p.id === id), status: 'ready', detail: 'Signed in', catalog }; },
        AgentSignIn: async id => { window.__calls.push(['signin', id]); },
        ChooseAgentExecutable: async id => { window.__calls.push(['choose', id]); if (window.__cancelChoice) return false; if (window.__failChoice) throw new Error('The selected path is not an executable file'); const p = providers.find(p => p.id === id); p.executableOverride = 'C:/Selected Agent/codex.exe'; p.path = p.executableOverride; return true; },
        SetAgentExecutable: async (id, path) => { window.__calls.push(['executable', id, path]); const p = providers.find(p => p.id === id); p.executableOverride = path; p.path = 'C:/Tools/codex.exe'; },
        SetDefaultProvider: async id => { provider = id; window.__calls.push(['default', id]); },
        InstallClaudeAdapter: async () => {
          window.__calls.push(['install-claude']);
          Object.assign(providers[2], { installed: true, available: true, status: 'detected', detail: 'Node.js and the Claude adapter 0.87.0 are installed. Claude uses your own Claude Code sign-in.' });
          providers[2].claude = { ...providers[2].claude, adapterInstalled: true };
          return providers[2];
        },
        SetClaudeAPIKey: async key => {
          window.__calls.push(['claude-key', key]);
          providers[2].claude = { ...providers[2].claude, apiKey: !!key };
          providers[2].detail = key ? 'Claude is billed to the API key you stored.' : 'Claude uses your own Claude Code sign-in.';
          return providers[2];
        },
        SkillsForProviderAt: async (ws, id) => { window.__calls.push(['skills', ws, id]); return [{ name: 'workspace-task', source: 'workspace' }].concat(id === 'devin' ? [{ name: 'devin-command', source: 'devin' }] : []); },
        SkillsAt: async () => [{ name: 'workspace-task', source: 'workspace' }, { name: 'devin-command', source: 'devin' }],
        NewSessionWithProviderAt: async (ws, id) => { window.__calls.push(['new', ws, id]); return { sessionId: 'codex-chat', provider: id, cwd: ws, title: 'Codex Chat', open: true }; },
        NewSessionAt: async () => { throw new Error('Legacy session creation must not be called'); },
        Send: async (id, text) => { window.__calls.push(['send', id, text]); }
      };
    },
    init: () => { window.__calls = []; },
    ready: () => window.__calls.some(c => c[0] === 'scan'),
  });
  const { page, ok } = t;
  try {
    await page.evaluate(() => dinahWorkbench.open('settings'));
    const settings = page.locator('#settings');
    ok('Codex default labels the status control before any connection or sign-in event', await page.locator('#status-agent .status-words').textContent() === 'Codex');
    await page.locator('#status-agent').click();
    ok('the initial agent menu agrees with the configured provider', await page.locator('#agent-menu [data-cmd="restart"]').textContent().then(text => text.includes('Restart Codex')) && await page.locator('#agent-menu [data-cmd="sign-in"]').textContent().then(text => text.includes('Sign In to Codex')));
    await page.keyboard.press('Escape');
    await settings.getByRole('tab', {name:'Agents',exact:true}).click();
    await settings.locator('[data-provider="codex"]').waitFor();
    ok('Codex-only startup discovers installed providers without requiring Devin', (await settings.locator('[data-provider="codex"]').textContent()).includes('Ready') && !(await page.locator('body').textContent()).includes('The Devin command line was not found'));
    // Two Devin command lines that work are listed for the person to choose,
    // the newer marked, and nothing is chosen for them.
    const devinCard = settings.locator('[data-provider="devin"]');
    const choiceRows = devinCard.locator('.agent-choice');
    ok('working Devin command lines are listed with their versions, the newer marked', await choiceRows.count() === 2
      && (await choiceRows.nth(0).textContent()).includes('2026.4.15-0') && !(await choiceRows.nth(0).textContent()).includes('likely')
      && (await choiceRows.nth(1).textContent()).includes('3000.10.48') && (await choiceRows.nth(1).textContent()).includes('likely the current one')
      && (await devinCard.textContent()).includes('More than one Devin command line works')
      && await page.evaluate(() => !__calls.some(c => c[0] === 'executable')));
    await choiceRows.nth(1).getByRole('button', { name: 'Use This One' }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'executable' && c[1] === 'devin'));
    ok('Use This One saves that command line for Devin', await page.evaluate(() => __calls.some(c => c[0] === 'executable' && c[1] === 'devin' && c[2] === 'C:/Users/p/AppData/Local/devin/cli/bin/devin.exe')));
    await page.evaluate(() => go.main.App.SetAgentExecutable('devin', ''));
    // Claude runs on Node.js through an adapter Dinah installs, and
    // is billed to the person's own sign-in unless they store an API key.
    const claudeCard = settings.locator('[data-provider="claude"]');
    ok('Claude without its adapter cannot be chosen yet, and its card says what is missing, with no Sign In', await page.locator('#default-provider option[value="claude"]').isDisabled() &&
      (await page.locator('#default-provider option[value="claude"]').textContent()).includes('Not Set Up') && (await claudeCard.textContent()).includes('Not Set Up') && (await claudeCard.textContent()).includes('Install the Claude adapter') &&
      (await claudeCard.textContent()).includes('Node.js: C:/Program Files/nodejs/node.exe') && await claudeCard.getByRole('button', { name: 'Choose Node.js…' }).count() === 1 &&
      await claudeCard.getByRole('button', { name: 'Sign In' }).count() === 0 && (await claudeCard.textContent()).includes('never asks for your Claude password'));
    await claudeCard.getByRole('button', { name: 'Install the Claude Adapter' }).click();
    await settings.locator('[data-provider="claude"]').getByText('Ready', { exact: false }).first().waitFor();
    ok('Install the Claude Adapter installs it, and Claude can then be chosen', await page.evaluate(() => __calls.some(c => c[0] === 'install-claude')) && !(await page.locator('#default-provider option[value="claude"]').isDisabled()) &&
      await settings.locator('[data-provider="claude"]').getByRole('button', { name: 'Install the Claude Adapter' }).count() === 0);
    await t.shot('agent-providers-claude');
    const key = settings.locator('#claude-api-key');
    ok('the key field is a password field, and Use This Key waits for a key', await key.getAttribute('type') === 'password' && await settings.locator('[data-provider="claude"]').getByRole('button', { name: 'Use This Key' }).isDisabled());
    await key.fill('sk-ant-test');
    await settings.locator('[data-provider="claude"]').getByRole('button', { name: 'Use This Key' }).click();
    await settings.locator('[data-provider="claude"]').getByRole('button', { name: 'Remove the Key' }).waitFor();
    ok('Use This Key stores the key, and the card says Claude is billed to it', await page.evaluate(() => __calls.some(c => c[0] === 'claude-key' && c[1] === 'sk-ant-test')) &&
      (await settings.locator('[data-provider="claude"]').textContent()).includes('billed to the API key you stored') && await settings.locator('#claude-api-key').count() === 0);
    await settings.locator('[data-provider="claude"]').getByRole('button', { name: 'Remove the Key' }).click();
    await page.locator('#ask-yes').click();
    await settings.locator('#claude-api-key').waitFor();
    ok('Remove the Key asks first, then goes back to the sign-in', await page.evaluate(() => __calls.some(c => c[0] === 'claude-key' && c[1] === '')));
    // Ollama is a server at an address: when it does not answer it cannot be
    // chosen and says so, and its card offers Test Connection only.
    const ollamaCard = settings.locator('[data-provider="ollama"]');
    ok('an Ollama that does not answer is shown as not running, with no executable or sign-in', await page.locator('#default-provider option[value="ollama"]').isDisabled() &&
      (await page.locator('#default-provider option[value="ollama"]').textContent()).includes('Not Running') && (await ollamaCard.textContent()).includes('Not Running') &&
      await ollamaCard.getByRole('button', { name: 'Test Connection' }).count() === 1 && await ollamaCard.getByRole('button', { name: 'Choose Executable…' }).count() === 0 &&
      await ollamaCard.getByRole('button', { name: 'Sign In' }).count() === 0);
    const codex = settings.locator('[data-provider="codex"]');
    await page.evaluate(() => { window.__cancelChoice = true; });
    await codex.getByRole('button', { name: 'Choose Executable…', exact: true }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'choose'));
    ok('canceling the file picker preserves automatic discovery', await codex.getByRole('button', { name: 'Use Automatic Discovery', exact: true }).count() === 0);
    await page.evaluate(() => { window.__cancelChoice = false; window.__failChoice = true; });
    await codex.getByRole('button', { name: 'Choose Executable…', exact: true }).click();
    await codex.getByText(/not an executable file/).waitFor();
    ok('a rejected executable shows an actionable error', await codex.getByRole('button', { name: 'Choose Executable…', exact: true }).isEnabled());
    await page.evaluate(() => { window.__failChoice = false; });
    await codex.getByRole('button', { name: 'Choose Executable…', exact: true }).click();
    await codex.getByText('Selected executable: C:/Selected Agent/codex.exe', { exact: true }).waitFor();
    ok('the selected executable is displayed with a reset command', await codex.getByRole('button', { name: 'Use Automatic Discovery', exact: true }).isVisible());
    await t.shot('agent-executable-override');
    await codex.getByRole('button', { name: 'Use Automatic Discovery', exact: true }).click();
    await codex.getByText('C:/Tools/codex.exe', { exact: true }).waitFor();
    ok('reset restores automatic discovery for this provider only', await page.evaluate(() => __calls.some(c => c[0] === 'executable' && c[1] === 'codex' && c[2] === '')));
    await settings.locator('[data-provider="codex"]').getByRole('button', { name: 'Sign In', exact: true }).click();
    await settings.getByText('Sign-in was requested. Test Connection checks whether it completed.').waitFor();
    ok('sign-in does not claim authentication succeeded', await page.evaluate(() => __calls.some(c => c[0] === 'signin' && c[1] === 'codex')));
    await settings.locator('[data-provider="codex"]').getByRole('button', { name: 'Test Connection', exact: true }).click();
    await settings.getByText(/Signed in/).waitFor();
    ok('connection checks load a provider catalog without a prompt', await page.evaluate(() => __calls.some(c => c[0] === 'check') && !__calls.some(c => c[0] === 'send')));
    await t.shot('agent-providers-settings');
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    await page.locator('[data-section="workspace-chats"] .browser-group-head').getByRole('button', { name: 'New Chat', exact: true }).click();
    const front = () => page.locator('.win.active');
    ok('New Session opens a draft with provider choice before creating an agent session', await shows(front(), 'provider', 'Codex') && await front().locator('[data-command="provider"]').isEnabled() && await page.evaluate(() => !__calls.some(c => c[0] === 'new')));
    ok('a new chat selects the configured provider and its own model', await shows(front(), 'provider', 'Codex') && await shows(front(), 'model', 'Codex Model'));
    await front().locator('[data-view-id="prompt"]').fill('Hello Codex');
    await front().locator('[data-view-id="prompt"]').press('Control+Enter');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'send'));
    ok('first prompt creates an explicit Codex session', await page.evaluate(() => __calls.some(c => c[0] === 'new' && c[1] === 'C:/Test' && c[2] === 'codex')));
    ok('an idle conversation can change providers', await front().locator('[data-command="provider"]:not(:disabled)').first().waitFor({ state: 'attached', timeout: 3000 }).then(() => true, () => false));
    await page.evaluate(() => {
      __emit('agent:update', { sessionId: 'codex-chat', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Hello from Codex' } } });
      __emit('agent:turn', { sessionId: 'codex-chat', stopReason: 'end_turn' });
    });
    await front().getByText('Codex finished its turn.', { exact: true }).waitFor();
    ok('chat output identifies the actual provider', (await front().locator('[data-view-id="logs"]').textContent()).includes('Codex'));
    await front().locator('[data-cmd="remove-session"]').click();
    ok('Codex sessions offer hiding without promising deletion', await page.locator('#remove-hide').isVisible() && await page.locator('#remove-go').isHidden() && (await page.locator('#remove-body').textContent()).includes('Codex keeps'));
    await page.locator('#remove').press('Escape');
    await t.shot('agent-providers-chat');
    await page.evaluate(() => { __installDevin(); return dinahWorkbench.open('settings'); });
    await page.locator('#scan-agents').click();
    await page.waitForFunction(() => !document.querySelector('#default-provider option[value="devin"]').disabled);
    await page.locator('#default-provider').selectOption('devin');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'default' && c[1] === 'devin'));
    await page.locator('#default-provider').isEnabled();
    await page.evaluate(() => dinahWorkbench.open('chat'));
    ok('changing the default affects a new chat', await shows(front(), 'provider', 'Devin'));
    await choose(page, front(), 'provider', 'Codex');
    ok('switching a draft uses only the selected provider catalog', await shows(front(), 'model', 'Codex Model'));
    ok('skills are loaded for the chosen provider', await page.evaluate(() => __calls.some(c => c[0] === 'skills' && c[2] === 'codex')));
    ok('changing the default leaves an existing chat on Codex', await shows(page.locator('.win[data-win-kind="chat-session"]'), 'provider', 'Codex'));
    // The agent menu in the status area shows which agent new chats use and changes it.
    await page.locator('#status-agent').click();
    const rows = page.locator('#default-agent-rows .menu-action');
    ok('the agent menu shows the agent new chats use', await rows.filter({ hasText: 'Devin' }).getAttribute('aria-checked') === 'true');
    await rows.filter({ hasText: 'Codex' }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'default' && c[1] === 'codex'));
    ok('choosing in the agent menu sets the default as Settings does', await page.locator('#default-provider').inputValue() === 'codex');
    await page.setViewportSize({ width: 760, height: 800 });
    await page.evaluate(() => dinahWorkbench.open('settings'));
    await t.shot('agent-providers-narrow');
  } finally { await t.close(); }
})();
