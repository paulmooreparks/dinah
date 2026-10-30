// Taskbar stacks (docs/plans/taskbar-stacks.md): every workspace's windows
// stack on the taskbar under its name, the chosen workspace's included;
// windows of no workspace stay ordinary buttons; a rename keeps one stack
// and changes its label; and the windows the saved layout brings back when
// the program starts are stacked at once, with no workspace chosen first.
// The workspaces and the saved layout live in a backend fixture that
// outlasts the page.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const records = [
    { id: 'ws_ALPHA', name: 'Alpha', executionPath: 'C:/Alpha', mountAlias: 'alpha', primarySourceId: 'root-a', sources: [{ id: 'root-a', kind: 'local-folder', path: 'C:/Alpha' }] },
    { id: 'ws_BETA', name: 'Beta', executionPath: 'C:/Beta', mountAlias: 'beta', primarySourceId: 'root-b', sources: [{ id: 'root-b', kind: 'local-folder', path: 'C:/Beta' }] },
    // An ID with characters a PUDL key cannot have.
    { id: 'team/gamma', name: 'Gamma', executionPath: 'C:/Gamma', mountAlias: 'gamma', primarySourceId: 'root-g', sources: [{ id: 'root-g', kind: 'local-folder', path: 'C:/Gamma' }] }
  ];
  let selected = 'ws_ALPHA', uiState = { version: 1, values: {} };
  const t = await launch({
    viewport: { width: 1700, height: 900 },
    app: () => {
      const state = async () => {
        const stored = await window.__store('read');
        const record = stored.records.find(r => r.id === stored.selected);
        return { workspace: record.executionPath, workspaceId: record.id, workspaceRecords: stored.records, recent: stored.records.map(r => r.executionPath), workspaces: stored.records.map(r => r.executionPath), defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [] };
      };
      const info = (id, ws) => ({ sessionId: id, provider: 'codex', title: id, cwd: ws, open: true, working: false, queued: [] });
      return {
        State: state,
        Sessions: async () => [], SessionsAt: async () => [],
        OpenSessionAt: async (ws, id) => info(id, ws),
        SelectWorkspace: async id => { await window.__store('select', id); return state(); },
        SetWorkspace: async () => { throw new Error('Workspace selection must use its ID'); },
        RenameWorkspace: async (id, name) => (await window.__store('rename', id, name)).records.find(r => r.id === id),
        // The saved layout outlasts the page too, as the program's file does.
        ReadUIState: async () => (await window.__store('ui-read')).ui,
        SaveUIState: async value => { await window.__store('ui-save', null, null, value); },
      };
    },
    goto: false,
  });
  const { page, ok } = t;
  try {
    await t.context.exposeBinding('__store', (source, operation, id, name, value) => {
      if (operation === 'rename') records.find(r => r.id === id).name = name;
      if (operation === 'select') selected = id;
      if (operation === 'ui-save') uiState = value;
      return { selected, records: structuredClone(records), ui: structuredClone(uiState) };
    });
    await page.goto(PAGE);
    await t.waitReady();
    await page.locator('#status-workspace-name').getByText('Alpha', { exact: true }).waitFor();

    const bar = 'nav.win-dock[data-win-dock-omit-docked]';
    const chatKey = (ws, id) => page.evaluate(([ws, id]) => dinahWorkbench.key({ kind: 'chat-session', workspace: ws, path: id }), [ws, id]);
    const openChat = (ws, id) => page.evaluate(([ws, id]) => dinahWorkbench.open({ kind: 'chat-session', workspace: ws, path: id }), [ws, id]);
    const stack = group => page.locator(bar + ' > [data-win-stack="' + group + '"]');
    const button = key => page.locator(bar + ' > [data-win-tab="' + key + '"]');
    const stackName = async group => (await stack(group).getAttribute('aria-label')) || '';
    const attrs = key => page.evaluate(key => {
      const win = document.querySelector('.win[data-win="' + CSS.escape(key) + '"]');
      return win && { group: win.getAttribute('data-win-group'), label: win.getAttribute('data-win-group-label'), glyph: win.getAttribute('data-win-group-glyph') };
    }, key);
    // Whether a stack is drawn within a few seconds, without failing the run.
    const appears = locator => locator.waitFor({ timeout: 5000 }).then(() => true, () => false);
    // Waits until a stack says it has n windows.
    const counts = (group, n) => page.waitForFunction(([bar, group, n]) => {
      const s = document.querySelector(bar + ' > [data-win-stack="' + group + '"]');
      return s && (s.getAttribute('aria-label') || '').includes(', ' + n + ' windows');
    }, [bar, group, n]);
    const choose = async id => {
      await page.evaluate(() => dinahWorkbench.open('workspace'));
      await page.locator('#ws-select').selectOption({ value: records.find(r => r.id === id).executionPath });
      // New Chat goes with the chosen workspace, so its group shows the choice is drawn.
      await page.waitForFunction(id => document.querySelector('.win[data-win="chat"]').getAttribute('data-win-group') === id, id);
    };

    const a1 = await chatKey('C:/Alpha', 'alpha-1'), a2 = await chatKey('C:/Alpha', 'alpha-2');
    const b1 = await chatKey('C:/Beta', 'beta-1'), b2 = await chatKey('C:/Beta', 'beta-2'), g1 = await chatKey('C:/Gamma', 'gamma-1');
    for (const [ws, id] of [['C:/Alpha', 'alpha-1'], ['C:/Alpha', 'alpha-2'], ['C:/Beta', 'beta-1'], ['C:/Beta', 'beta-2'], ['C:/Gamma', 'gamma-1']]) await openChat(ws, id);
    await page.evaluate(() => dinahWorkbench.open('settings'));
    await counts('ws_BETA', 2);
    await counts('ws_ALPHA', 3);

    ok('the layer stacks its taskbar always', await page.locator('[data-win-layer]').getAttribute('data-win-taskbar-stacks') === 'always');
    ok('no group is marked open', !await page.locator('[data-win-layer]').evaluate(layer => layer.hasAttribute('data-win-group-open')));
    const beta = await attrs(b1);
    ok('a workspace\'s window carries its ID, its name and the folder glyph', beta.group === 'ws_BETA' && beta.label === 'Beta' && beta.glyph === 'folder', JSON.stringify(beta));
    ok('two windows of one workspace form one stack labelled with its name', await stack('ws_BETA').count() === 1 && (await stack('ws_BETA').locator('.win-item-title').textContent()) === 'Beta' &&
      (await stackName('ws_BETA')).startsWith('Beta, 2 windows') && await button(b1).count() === 0 && await button(b2).count() === 0);
    ok('the chosen workspace\'s windows stack like any other, New Chat with them', (await attrs('chat')).group === 'ws_ALPHA' &&
      (await stackName('ws_ALPHA')).startsWith('Alpha, 3 windows') && await button(a1).count() === 0 && await button(a2).count() === 0 && await button('chat').count() === 0);
    const settings = await attrs('settings');
    ok('a window of no workspace has no group and stays an ordinary button', settings && !settings.group && !settings.label && !settings.glyph && await button('settings').count() === 1);
    const gamma = await attrs(g1);
    ok('an ID with other characters becomes a stable safe key', /^ws-[0-9a-f]{16}$/.test(gamma.group) && gamma.label === 'Gamma' && await button(g1).count() === 1, JSON.stringify(gamma));
    await t.shot('taskbar-stacks-taskbar', { clip: { x: 0, y: 820, width: 1700, height: 80 } });

    // Pressing the chosen workspace's stack pins its list: it stays open
    // after the pointer leaves.
    await stack('ws_ALPHA').click();
    const panel = page.locator('.win-stack-panel');
    await panel.waitFor();
    await page.mouse.move(850, 300);
    await page.waitForTimeout(600);
    ok('pressing the chosen workspace\'s stack pins its list of windows', await panel.isVisible() && await stack('ws_ALPHA').getAttribute('aria-expanded') === 'true' &&
      await panel.locator('.win-stack-link').count() === 3 && await panel.locator('.win-stack-link[data-win-tab="' + a1 + '"]').count() === 1 &&
      await panel.locator('.win-stack-link[data-win-tab="chat"]').count() === 1);
    await t.shot('taskbar-stacks-popup');
    await page.keyboard.press('Escape');
    await panel.waitFor({ state: 'detached' });

    await choose('ws_BETA');
    await counts('ws_BETA', 3);
    await counts('ws_ALPHA', 2);
    ok('choosing another workspace moves New Chat to its stack and both stay stacked', await button(b1).count() === 0 && await button(a1).count() === 0 && await button('chat').count() === 0);

    // A rename keeps one stack and changes its label.
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd="rename-workspace"]').click();
    await page.locator('#rename-workspace-name').fill('Beta Release');
    await page.locator('#rename-workspace-save').click();
    await page.locator('#rename-workspace').waitFor({ state: 'hidden' });
    await page.waitForFunction(bar => document.querySelector(bar + ' > [data-win-stack="ws_BETA"] .win-item-title').textContent === 'Beta Release', bar);
    const renamed = [await attrs(b1), await attrs(b2), await attrs('chat')];
    ok('a rename keeps the key and changes the label on every window of the workspace', renamed.every(a => a.group === 'ws_BETA' && a.label === 'Beta Release'), JSON.stringify(renamed));
    ok('the renamed workspace is still one stack, labelled with its new name', await stack('ws_BETA').count() === 1 && (await stackName('ws_BETA')).startsWith('Beta Release, 3 windows'));
    await choose('ws_ALPHA');

    // A chat whose window moves to another workspace, as Continue in
    // Another Workspace moves it, moves to that workspace's stack.
    const moved = await page.evaluate(key => dinahWorkbench.rekey(key, { kind: 'chat-session', workspace: 'C:/Beta', path: 'alpha-2' }), a2);
    await counts('ws_BETA', 3);
    ok('a window moved to another workspace joins its stack', (await attrs(moved)).group === 'ws_BETA' && await button(moved).count() === 0);

    // Starting again: the address has no layout, so the windows come back
    // from the saved layout, as they do when the program starts, and they
    // are stacked with no workspace chosen first.
    await page.evaluate(() => dinahUIState.flush());
    await page.goto(PAGE);
    await t.waitReady();
    await page.locator('#status-workspace-name').getByText('Alpha', { exact: true }).waitFor();
    const restored = await page.evaluate(() => window.pudlWindows.state().open.length);
    const betaBack = await appears(stack('ws_BETA')), alphaBack = await appears(stack('ws_ALPHA'));
    ok('windows restored at start are stacked with no workspace change', restored >= 9 && betaBack && alphaBack && (await t.calls('SelectWorkspace')).length === 0 &&
      (await stackName('ws_BETA')).startsWith('Beta Release, 3 windows') && (await stackName('ws_ALPHA')).startsWith('Alpha, 2 windows') &&
      await button(g1).count() === 1 && await button('settings').count() === 1, JSON.stringify({ restored, betaBack, alphaBack, beta: await attrs(b1) }));
  } finally { await t.close(); }
})();
