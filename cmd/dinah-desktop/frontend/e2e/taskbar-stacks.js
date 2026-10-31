// Applet/workspace ownership and PUDL overflow projection.
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
        GetDraft: async () => new Promise(() => {}),
        StatWorkspaceMount: async path => ({ path: path === '~' ? '/home/fixture' : path, directory: true, actions: ['list'] }),
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

    const groupKey = (applet, workspace) => 'applet-' + Buffer.from(JSON.stringify(workspace ? [applet, workspace] : [applet])).toString('base64url');
    const chatGroup = id => groupKey('dinah.chat', id);
    ok('production enables overflow-triggered stacking', await page.locator('[data-win-layer]').getAttribute('data-win-taskbar-stacks') === 'full');
    // Hold projection constant while checking ownership and command membership.
    await page.locator('[data-win-layer]').evaluate(layer => layer.setAttribute('data-win-taskbar-stacks', 'always'));
    const bar = 'nav.win-dock[data-win-dock-omit-docked]';
    const chatKey = (ws, id) => page.evaluate(([ws, id]) => dinahWorkbench.key({ kind: 'chat-session', workspace: ws, path: id }), [ws, id]);
    const openChat = (ws, id) => page.evaluate(([ws, id]) => dinahWorkbench.open({ kind: 'chat-session', workspace: ws, path: id }), [ws, id]);
    const stack = id => { const group = chatGroup(id); return page.locator(bar + ' > [data-win-stack="' + group + '"]'); };
    const button = key => page.locator(bar + ' > [data-win-tab="' + key + '"]');
    const stackName = async group => (await stack(group).getAttribute('aria-label')) || '';
    const attrs = key => page.evaluate(key => {
      const win = document.querySelector('.win[data-win="' + CSS.escape(key) + '"]');
      return win && { group: win.getAttribute('data-win-group'), label: win.getAttribute('data-win-group-label'), glyph: win.getAttribute('data-win-group-glyph') };
    }, key);
    // Whether a stack is drawn within a few seconds, without failing the run.
    const appears = locator => locator.waitFor({ timeout: 5000 }).then(() => true, () => false);
    // Waits until a stack says it has n windows.
    const counts = (id, n) => { const group = chatGroup(id); return page.waitForFunction(([bar, group, n]) => {
      const s = document.querySelector(bar + ' > [data-win-stack="' + group + '"]');
      return s && (s.getAttribute('aria-label') || '').includes(', ' + n + ' windows');
    }, [bar, group, n]); };
    const choose = async id => {
      await page.evaluate(() => dinahWorkbench.open('workspace'));
      await page.locator('#ws-select').selectOption({ value: records.find(r => r.id === id).executionPath });
      // New Chat goes with the chosen workspace, so its group shows the choice is drawn.
      await page.waitForFunction(id => document.querySelector('.win[data-win="chat"]').getAttribute('data-win-group') === id, chatGroup(id));
    };

    const a1 = await chatKey('C:/Alpha', 'alpha-1'), a2 = await chatKey('C:/Alpha', 'alpha-2');
    const b1 = await chatKey('C:/Beta', 'beta-1'), b2 = await chatKey('C:/Beta', 'beta-2'), g1 = await chatKey('C:/Gamma', 'gamma-1');
    for (const [ws, id] of [['C:/Alpha', 'alpha-1'], ['C:/Alpha', 'alpha-2'], ['C:/Beta', 'beta-1'], ['C:/Beta', 'beta-2'], ['C:/Gamma', 'gamma-1']]) await openChat(ws, id);
    await page.evaluate(() => dinahWorkbench.open('settings'));
    await counts('ws_BETA', 2);
    await counts('ws_ALPHA', 3);

    ok('the ownership fixture holds projection constant', await page.locator('[data-win-layer]').getAttribute('data-win-taskbar-stacks') === 'always');
    ok('no group is marked open', !await page.locator('[data-win-layer]').evaluate(layer => layer.hasAttribute('data-win-group-open')));
    const beta = await attrs(b1);
    ok('a workspace\'s window carries the applet/workspace identity and applet glyph', beta.group === chatGroup('ws_BETA') && beta.label === 'Chats (Beta)' && beta.glyph === 'comments', JSON.stringify(beta));
    ok('two windows of one workspace form one stack labelled with its name', await stack('ws_BETA').count() === 1 && (await stack('ws_BETA').locator('.win-item-title').textContent()) === 'Chats (Beta)' &&
      (await stackName('ws_BETA')).startsWith('Chats (Beta), 2 windows') && await button(b1).count() === 0 && await button(b2).count() === 0);
    ok('the chosen workspace\'s windows stack like any other, New Chat with them', (await attrs('chat')).group === chatGroup('ws_ALPHA') &&
      (await stackName('ws_ALPHA')).startsWith('Chats (Alpha), 3 windows') && await button(a1).count() === 0 && await button(a2).count() === 0 && await button('chat').count() === 0);
    const settings = await attrs('settings');
    ok('an explicitly workspace-free singleton has its applet group and an ordinary button', settings && settings.group === groupKey('dinah.settings') && settings.label === 'Settings' && await button('settings').count() === 1);
    const gamma = await attrs(g1);
    ok('an ID with other characters becomes a stable safe key', gamma.group === chatGroup('team/gamma') && gamma.label === 'Chats (Gamma)' && await button(g1).count() === 1, JSON.stringify(gamma));
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
    await counts('ws_BETA', 3);
    const renamed = [await attrs(b1), await attrs(b2), await attrs('chat')];
    ok('a rename keeps the key and changes the label on every window of the workspace', renamed.every(a => a.group === chatGroup('ws_BETA') && a.label === 'Chats (Beta Release)'), JSON.stringify(renamed));
    ok('the renamed workspace is still one stack, labelled with its new name', await stack('ws_BETA').count() === 1 && (await stackName('ws_BETA')).startsWith('Chats (Beta Release), 3 windows'));
    await choose('ws_ALPHA');

    // A chat whose window moves to another workspace, as Continue in
    // Another Workspace moves it, moves to that workspace's stack.
    const moved = await page.evaluate(key => dinahWorkbench.rekey(key, { kind: 'chat-session', workspace: 'C:/Beta', path: 'alpha-2' }), a2);
    await counts('ws_BETA', 3);
    ok('a window moved to another workspace joins its stack', (await attrs(moved)).group === chatGroup('ws_BETA') && await button(moved).count() === 0);

    // Starting again: the address has no layout, so the windows come back
    // from the saved layout, as they do when the program starts, and they
    // are stacked with no workspace chosen first.
    await page.evaluate(() => dinahUIState.flush());
    await page.goto(PAGE);
    await t.waitReady();
    await page.locator('#status-workspace-name').getByText('Alpha', { exact: true }).waitFor();
    await page.locator('[data-win-layer]').evaluate(layer => layer.setAttribute('data-win-taskbar-stacks', 'always'));
    const restored = await page.evaluate(() => window.pudlWindows.state().open.length);
    const betaBack = await appears(stack('ws_BETA')), alphaBack = await appears(stack('ws_ALPHA'));
    ok('windows restored at start are stacked with no workspace change', restored >= 9 && betaBack && alphaBack && (await t.calls('SelectWorkspace')).length === 0 &&
      (await stackName('ws_BETA')).startsWith('Chats (Beta Release), 3 windows') && (await stackName('ws_ALPHA')).startsWith('Chats (Alpha), 2 windows') &&
      await button(g1).count() === 1 && await button('settings').count() === 1, JSON.stringify({ restored, betaBack, alphaBack, beta: await attrs(b1) }));
    const extra = await page.evaluate(async () => {
      const editor = await dinahWorkbench.open({ kind: 'editor', workspace: 'ws_BETA', path: 'pending-document' });
      const terminals = [];
      for (const [folder, id] of [['/workspaces/alpha/root', 'terminal-alpha'], ['/workspaces/beta/root', 'terminal-beta']]) {
        terminals.push(await dinahWorkbench.open({ kind: 'terminal', path: JSON.stringify([folder, id]) }));
      }
      return { editor, terminals };
    });
    ok('Documents and Chats in the same workspace have separate identities', (await attrs(extra.editor)).group === groupKey('dinah.documents', 'ws_BETA') && (await attrs(extra.editor)).group !== (await attrs(b1)).group);
    const terminalAttrs = await Promise.all(extra.terminals.map(attrs));
    ok('Terminal groups independently of its launch folders', terminalAttrs.every(a => a.group === groupKey('dinah.terminal') && a.label === 'Terminal'));
    const unknown = await openChat('C:/NotRegistered', 'unresolved-owner');
    ok('an unresolved workspace-owned chat never joins the workspace-free Chat group', !(await attrs(unknown)).group);

    // A delayed declaration must regroup immediately, independently of selection.
    const delayed = await page.evaluate(async () => {
      window.__taskbarDeclaration = null;
      dinahWorkbench.register('late-taskbar-fixture', { title: 'Delayed Owner', applet: () => __taskbarDeclaration, init: () => ({}) });
      return dinahWorkbench.open({ kind: 'late-taskbar-fixture', workspace: 'ws_BETA', path: 'delayed' });
    });
    ok('a declaration that has not resolved has no group', !(await attrs(delayed)).group);
    await page.evaluate(() => { __taskbarDeclaration = { id: 'test.declared', title: 'Declared Applet', glyph: 'app', workspace: true }; dinahWorkbench.regroup(); });
    ok('resolving metadata assigns the existing window without workspace selection', (await attrs(delayed)).group === groupKey('test.declared', 'ws_BETA'));
    await page.evaluate(key => pudlWindows.close(key), delayed);
    await page.evaluate(key => pudlWindows.close(key), unknown);

    const terminalGroup = groupKey('dinah.terminal');
    await page.waitForFunction(([bar, group]) => !!document.querySelector(bar + ' > [data-win-stack="' + group + '"]'), [bar, terminalGroup]);
    await page.evaluate(key => pudlWindows.close(key), extra.terminals[1]);
    await button(extra.terminals[0]).waitFor();
    ok('a stack returns to an ordinary button with one represented window', await page.locator(bar + ' > [data-win-stack="' + terminalGroup + '"]').count() === 0);
    await page.locator('[data-win-layer]').evaluate(layer => layer.setAttribute('data-win-taskbar-stacks', 'full'));
    await page.setViewportSize({ width: 3000, height: 900 });
    await page.locator(bar + ' > [data-win-stack]').waitFor({ state: 'detached' });
    ok('full mode keeps separate buttons when they fit', await button(b1).count() === 1 && await button(b2).count() === 1);
    await page.setViewportSize({ width: 600, height: 900 });
    await counts('ws_BETA', 3);
    ok('overflow combines the same applet/workspace group', await stack('ws_BETA').count() === 1);
    await t.shot('taskbar-stacks-overflow');
    await page.setViewportSize({ width: 1700, height: 900 });
    await page.locator('[data-win-layer]').evaluate(layer => layer.setAttribute('data-win-taskbar-stacks', 'always'));
    // Distinct stable workspace IDs survive equal display names.
    const originalAlpha = (await attrs(a1)).group, originalBeta = (await attrs(b1)).group;
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd="rename-workspace"]').click();
    await page.locator('#rename-workspace-name').fill('Beta Release');
    await page.locator('#rename-workspace-save').click();
    await page.locator('#rename-workspace').waitFor({ state: 'hidden' });
    const sameNamed = [await attrs(a1), await attrs(b1)];
    ok('same-named workspaces remain separate and show distinguishing labels', sameNamed[0].group === originalAlpha && sameNamed[1].group === originalBeta && originalAlpha !== originalBeta && sameNamed[0].label !== sameNamed[1].label && sameNamed.every(a => a.label.includes('Beta Release')), JSON.stringify(sameNamed));
    // PUDL's represented subset excludes a docked member from this taskbar.
    await page.evaluate(key => pudlWindows.dock(key, 'left'), b2);
    await counts('ws_BETA', 2);
    const membership = await page.evaluate(([group, bar]) => ({ logical: pudlWindows.group(group), represented: pudlWindows.group(group, document.querySelector(bar)) }), [chatGroup('ws_BETA'), bar]);
    ok('the stack represents only its nondocked windows', membership.logical.includes(b2) && !membership.represented.includes(b2) && membership.represented.length === 2, JSON.stringify(membership));
  } finally { await t.close(); }
})();
