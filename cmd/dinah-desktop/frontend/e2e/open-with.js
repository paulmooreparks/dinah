// Open With… (option_substitutions.go, docs/design/chat-recovery.md). A chat
// refused because its agent no longer offers its saved model offers Open
// With…, listing the models offered, the one whose name matches the saved
// model's first and marked Suggested, never chosen by itself. Choosing one
// reopens the chat with it; ticking Always stores the choice as a
// substitution, which Settings › Agents lists with Remove and leaves out
// when there are none. A chat a stored substitution opened says so in a line.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    state: { workspace: 'C:/Test', workspaces: ['C:/Test'], recent: ['C:/Test'], defaultProvider: 'claude', approvals: 'devin', promptSendKey: 'enter' },
    app: () => {
      window.__calls = [];
      // The refusal as the bridge gives it: an Error with the detail
      // (internal/bridge, unofferedOptionError).
      const refusal = () => {
        const e = new Error("this chat's saved model setting is no longer available from its provider");
        e.detail = { kind: 'unoffered-option', provider: 'claude', option: 'model', optionName: 'Model', saved: 'fable', savedName: 'Fable',
          offered: [{ value: 'claude-fable-5-1[1m]', name: 'Fable', suggested: true }, { value: 'default', name: 'Default' }, { value: 'opus', name: 'Opus' }] };
        return e;
      };
      window.__subs = {};
      const info = (id, notes) => ({ sessionId: id, provider: 'claude', cwd: 'C:/Test', title: 'Fable Chat', open: true, notes });
      const providers = [{ id: 'claude', name: 'Claude', installed: true, available: true, status: 'detected', path: 'C:/Tools/node.exe', claude: { adapterVersion: '0.87.0', adapterInstalled: true, canInstall: true, apiKey: false } }];
      return {
        State: async () => Object.assign({}, window.__state, { optionSubstitutions: JSON.parse(JSON.stringify(window.__subs)) }),
        AgentProviders: async () => providers,
        SessionContinuation: async () => ({ history: [] }),
        SavedConversation: async () => ({ available: true, complete: true, messages: [{ role: 'you', text: 'Write the plan.' }, { role: 'agent', provider: 'claude', text: 'The plan is written.' }] }),
        OpenSessionAt: async (ws, id) => {
          __calls.push(['open', id]);
          // claude:noted is a chat a stored substitution opened.
          if (id === 'claude:noted') return info(id, ["Opened with Fable (claude-fable-5-1[1m]) because Fable wasn't offered; you set this in Settings › Agents › Claude."]);
          throw refusal();
        },
        OpenSessionWith: async (ws, id, option, value, always) => {
          __calls.push(['open-with', id, option, value, always]);
          // Storing a substitution tells the page its settings changed, as
          // storeSubstitution does.
          if (always) { window.__subs = { claude: [{ option, optionName: 'Model', from: 'fable', fromName: 'Fable', to: value, toName: 'Fable' }] }; setTimeout(() => window.__emit('settings:changed', {}), 0); }
          return info(id);
        },
        RemoveOptionSubstitution: async (provider, option, from) => { __calls.push(['remove', provider, option, from]); window.__subs = {}; },
      };
    },
    ready: () => window.dinahWorkbench && window.dinahEditor,
  });
  const { page: p, ok } = t;
  try {
    const front = () => p.locator('.win.active');
    // The engine reports a chat's provider before its window is opened.
    await p.evaluate(() => ['claude:fable', 'claude:noted', 'claude:other'].forEach(id => __emit('agent:session', { sessionId: id, provider: 'claude', cwd: 'C:/Test', open: false })));

    // The refusal offers Open With…, with the matching model first.
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:fable' }));
    await front().locator('[data-view-id="session-note-title"]').getByText('Claude could not open this session.', { exact: true }).waitFor();
    const openWith = front().locator('[data-view-id="session-open-with"]');
    await openWith.waitFor();
    ok('the refusal for an unoffered model offers Open With…', await openWith.isVisible() && (await openWith.innerText()) === 'Open With…');
    ok('the list is closed until Open With… is pressed', await front().locator('[data-view-id="session-open-with-panel"]').isHidden());
    await openWith.click();
    const choices = front().locator('[data-view-id="session-open-with-list"] button');
    await choices.first().waitFor();
    const labels = await choices.allInnerTexts();
    ok('the offered models are listed with their names, the suggestion first', labels.join('|') === 'Fable (claude-fable-5-1[1m])|Default|Opus', labels.join('|'));
    ok('the suggestion is marked and nothing was chosen by itself', (await front().locator('[data-view-id="session-open-with-list"] li').first().innerText()).includes('Suggested') &&
      await p.evaluate(() => !__calls.some(c => c[0] === 'open-with')));
    const always = front().locator('[data-view-id="session-open-with-always"]');
    ok('the checkbox reads as Paul asked', (await front().locator('[data-view-id="session-open-with-always-label"]').innerText()) === "Always Use This When 'Fable' Isn't Offered" && !(await always.isChecked()));

    // Choosing a model without Always reopens the chat with it.
    await choices.nth(2).click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'open-with'));
    await front().locator('[data-view-id="session-note"]').waitFor({ state: 'hidden' });
    ok('choosing a model reopens the chat with it, storing nothing', await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'open-with')) === JSON.stringify(['open-with', 'claude:fable', 'model', 'opus', false])));

    // Ticking Always stores the choice as a substitution.
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:other' }));
    await front().locator('[data-view-id="session-open-with"]').waitFor();
    await front().locator('[data-view-id="session-open-with"]').click();
    await front().locator('[data-view-id="session-open-with-always"]').check();
    await front().locator('[data-view-id="session-open-with-list"] button').first().click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'open-with' && c[1] === 'claude:other'));
    ok('the checkbox stores a substitution for the provider', await p.evaluate(() => {
      const c = __calls.find(c => c[0] === 'open-with' && c[1] === 'claude:other');
      return c[3] === 'claude-fable-5-1[1m]' && c[4] === true && window.__subs.claude.length === 1;
    }));

    // Settings › Agents lists it with Remove, and leaves the list out when
    // there is none.
    await p.evaluate(() => dinahWorkbench.open('settings'));
    const settings = p.locator('#settings');
    await settings.getByRole('tab', { name: 'Agents', exact: true }).click();
    const subs = settings.locator('[data-provider="claude"] [data-substitutions="claude"]');
    await subs.waitFor({ state: 'attached' });
    await p.waitForFunction(() => { const el = document.querySelector('#settings [data-substitutions="claude"]'); return el && !el.hidden; });
    const text = await subs.innerText();
    ok('Settings › Agents › Claude lists the substitution', text.includes('Saved Setting Substitutions') && text.includes("Model: Fable (claude-fable-5-1[1m]) when Fable isn't offered."), text);
    await subs.getByRole('button', { name: /^Remove/ }).click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'remove'));
    await p.waitForFunction(() => document.querySelector('#settings [data-substitutions="claude"]').hidden);
    ok('Remove forgets it and the list goes', await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'remove')) === JSON.stringify(['remove', 'claude', 'model', 'fable'])));

    // A chat a stored substitution opened says so in a line.
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:noted' }));
    const note = front().locator('.opening-note');
    await note.waitFor();
    ok('the chat shows the substitution note', (await note.innerText()).includes("Opened with Fable (claude-fable-5-1[1m]) because Fable wasn't offered; you set this in Settings › Agents › Claude."));
    ok('a chat opened normally has no Open With', await front().locator('[data-view-id="session-open-with"]').isHidden());
  } catch (e) {
    console.log('FAIL ' + e.message);
    t.failures.push('exception');
  } finally { await t.close(); }
})();
