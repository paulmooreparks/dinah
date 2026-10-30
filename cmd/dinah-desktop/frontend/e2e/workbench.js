// The workbench: the first-run layout, the Workspace pane's scoped chats,
// the browsers' docking, menus, scope and filters, the Tools launchers and
// their search, Go To, Settings as a window, the taskbar, bottom docks, the
// Window menu, the narrow layout, Close All, split docks, outline resizing,
// the ribbon and the taskbar's order.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      let workspace = 'C:/Alpha';
      const workspaces = ['C:/Alpha', 'C:/Beta'];
      const sessions = ws => [{ sessionId: ws + '-chat', provider: 'devin', cwd: ws, title: ws.endsWith('Alpha') ? 'Alpha Chat' : 'Beta Chat', open: false }];
      const state = () => ({ defaultProvider: 'devin', workspace, workspaces, recent: workspaces, devin: 'devin', approvals: 'devin' });
      return {
        AgentProviders: async () => [{ id: 'devin', name: 'Devin', installed: true, available: true, status: 'detected' }, { id: 'codex', name: 'Codex', installed: true, available: true, status: 'detected' }],
        State: async () => state(), SetWorkspace: async ws => { workspace = ws; return state(); },
        Sessions: async () => sessions(workspace), SessionsAt: async ws => sessions(ws),
        RecentDocuments: async () => [{ title: 'Beta Guide', ref: { workspaceId: 'C:/Beta', sourceId: 'local', path: 'guide.md' }, workspaceName: 'Beta', opened: '2026-10-01T09:00:00Z' }],
        DocumentRefAt: async (ws, file) => ({ workspaceId: ws, sourceId: 'local', path: file }),
        PreviewWorkspaceResource: async ref => ({ resource: { ref, title: ref.path, revision: '1', actions: ['preview'] }, title: ref.path, blocks: [{ line: 1, html: '<p>' + ref.workspaceId + '</p>' }], problems: [] }),
      };
    },
    ready: () => document.querySelector('[data-win="workspace"] #workspace-chats a'),
  });
  const { page: p, ok } = t;
  try {
    // A first run opens the default windows: the browsers docked on the
    // left, In Progress among them, with Workspace in front.
    await t.shot('first-run');
    const firstRun = await p.evaluate(() => { const s = pudlWindows.state(); return { open: s.open, left: s.docks && s.docks.left && s.docks.left.selected, top: s.top }; });
    console.log('first run: ' + JSON.stringify(firstRun));
    ok('a first run opens In Progress with the other browsers', firstRun.open.includes('in-progress'));
    ok('Workspace shows scoped chats without duplicate Chat groups', await p.locator('#workspace-chats a').count() === 1 && (await p.locator('#workspace-chats').textContent()).includes('Alpha Chat') && await p.locator('#tree, #workspace-chats summary').count() === 0);
    await p.evaluate(() => window.pudlWindows.close('workspace'));
    ok('Workspace cannot be closed', await p.locator('[data-win="workspace"]').count() === 1);
    ok('menu and status fonts are consistent', await p.evaluate(() => {
      const font = el => { const s = getComputedStyle(el); return [s.fontSize, s.fontWeight, s.fontFamily].join('|'); };
      const titles = [...document.querySelectorAll('.menubar-row .menubar-title:not(.menubar-one .menubar-title)')];
      const items = [...document.querySelectorAll('.status-area .status-item')];
      const glyph = el => getComputedStyle(el.querySelector('.glyph')).width;
      return titles.every(el => font(el) === font(titles[0])) && items.every(el => font(el) === font(items[0]) && glyph(el) === glyph(items[0]));
    }));
    await p.evaluate(() => window.pudlWindows.raise('workspace'));
    await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Actions$/ }).click();
    const refreshWorkspace = p.locator('#menubar-panel').getByText('Refresh Workspace', { exact: true });
    ok('Workspace offers a usable Refresh command in its Actions menu', await refreshWorkspace.isVisible() && await refreshWorkspace.isEnabled());
    await p.keyboard.press('Escape');
    ok('docked browsers have no taskbar entries or title bars', await p.locator('[data-win-dock] [data-win-tab="workspace"], [data-win-dock] [data-win-tab="chat-browser"]').count() === 0 && !await p.locator('[data-win="workspace"] > .win-head').isVisible());
    ok('Notifications has its own glyph', await p.locator('[data-win="needs-you"]').getAttribute('data-win-glyph') !== await p.locator('[data-win="chat-browser"]').getAttribute('data-win-glyph'));
    ok('browser commands appear in View instead of Window', await p.locator('[data-menubar-source] [data-menubar-id="view"] > ul > li > [data-cmd="chat-browser"]').count() === 1 && await p.locator('[data-menubar-source] [data-menubar-id="view"] > ul > li > [data-cmd="skills-browser"]').count() === 1 && await p.locator('#host-windows [data-cmd]').count() === 0);
    ok('Tools launchers have one shared searchable catalog, with the Editor', await p.evaluate(() => dinahWorkbench.launchers().map(l => l.id).sort().join() === 'editor,extensions,files,terminal,theme-editor' && !document.querySelector('[data-menubar-id="session"], [data-menubar-id="go"]')));
    // Tools › Editor starts an empty, untitled document.
    await p.evaluate(() => { window.__emptyDraft = null; go.main.App.NewEmptyDraft = async ws => { window.__emptyDraft = ws; return { id: 'e'.repeat(32), title: 'Untitled', text: '', revision: 'r', documentType: '', origin: { kind: 'new', workspaceId: ws }, actions: ['update', 'discard', 'save-as'], suggestions: [], spans: [], changed: false }; }; return dinahWorkbench.launchers().find(l => l.id === 'editor').run(); });
    ok('Tools › Editor starts an empty document', await p.evaluate(() => window.__emptyDraft !== null));
    await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Tools$/ }).click();
    await p.locator('#menubar-panel').getByText('Find a Tool…', { exact: true }).click();
    await p.fill('#go-to-filter', 'notes or a script');
    ok('tool search matches descriptions', await p.locator('#go-to-list tr').count() === 1 && (await p.locator('#go-to-list').textContent()).includes('Editor'));
    await p.evaluate(() => document.getElementById('go-to').close());
    await p.evaluate(() => dinahWorkbench.open('chat'));
    await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Actions$/ }).click();
    // The chat's Actions menu has the toolbar's Provider command, whose
    // providers are a submenu of choices.
    await p.locator('#menubar-panel').getByText('Provider', { exact: true }).click();
    const devinRow = p.locator('#menubar-sub .menu-action[role="menuitemradio"]').filter({ hasText: /^Devin/ }).first();
    ok('Chat Actions offers the providers as a submenu of choices', await devinRow.waitFor({ timeout: 3000 }).then(() => true, () => false) && await devinRow.getAttribute('aria-checked') === 'true');
    await p.keyboard.press('Escape');
    await p.keyboard.press('Escape');
    await p.evaluate(() => dinahWorkbench.open('skills-browser'));
    await p.locator('[data-win="skills-browser"]').waitFor();
    ok('the selected tab keeps its menu visible in a crowded dock', await p.locator('[data-dock-menu="skills-browser"]').isVisible());
    await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Skills$/ }).click();
    const rows = await p.locator('#menubar-panel .menu-action').allTextContents();
    const windowRows = await p.evaluate(() => pudlWindows.menuCommands('skills-browser').map(c => c.label));
    ok('a pane\'s own menu has its window menu\'s rows', windowRows.length > 2 && windowRows.every(label => rows.some(row => row.includes(label))), JSON.stringify(rows));
    await p.keyboard.press('Escape');
    await p.evaluate(() => dinahWorkbench.open('workspace'));
    // The pane's menu title arrives a moment after the pane opens.
    await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Workspace$/ }).first().waitFor({ timeout: 5000 }).catch(() => {});
    ok('the Workspace pane has a Workspace menu', await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Workspace$/ }).count() === 1);
    await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Workspace$/ }).click();
    ok('the Workspace menu has the window menu\'s rows', (await p.locator('#menubar-panel .menu-action').allTextContents()).some(row => row.includes('Collapse')));
    await p.keyboard.press('Escape');
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    ok('the current scope excludes other workspaces', await p.locator('#sessions a[data-session]').count() === 1);
    await p.selectOption('#chat-scope', 'all');
    await p.locator('#sessions a[data-session]').filter({ hasText: 'Beta Chat' }).waitFor();
    ok('all open workspaces includes every workspace\'s chats', await p.locator('#sessions a[data-session]').count() === 2);
    await t.shot('pane-chats');
    await p.evaluate(() => dinahWorkbench.open('workspace'));
    await t.shot('pane-workspace');
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.fill('#chat-filter', 'Beta');
    await p.waitForFunction(() => document.querySelectorAll('#sessions a[data-session]').length === 1 && document.querySelector('#sessions').textContent.includes('Beta'));
    ok('the browser filters by name', await p.locator('#sessions a[data-session]').count() === 1);
    ok('browser filters leave workspace lists unchanged', await p.locator('#workspace-chats a').count() === 1 && (await p.locator('#workspace-chats').textContent()).includes('Alpha Chat'));
    await p.evaluate(() => window.pudlWindows.close('chat-browser'));
    await p.evaluate(() => window.dinahWorkbench.open('chat-browser'));
    ok('reopening the browser keeps its filter', await p.inputValue('#chat-filter') === 'Beta');
    await p.fill('#chat-filter', '');
    await p.selectOption('#chat-scope', 'current');
    // The Editor's own opening is the editor suite's; here it is recorded.
    await p.evaluate(() => { window.dinahEditor.openResource = ref => { window.__openedRef = ref; return Promise.resolve(); }; });
    await p.keyboard.press('Control+p');
    await p.fill('#go-to-filter', 'Beta Guide');
    await p.locator('#go-to-list tr').dblclick();
    await p.waitForFunction(() => !!window.__openedRef);
    ok('Go To opens a document in its own workspace', await p.evaluate(() => window.__openedRef.workspaceId === 'C:/Beta'));
    await p.evaluate(() => window.dinahWorkbench.open('settings'));
    ok('Settings is a window', await p.locator('[data-win="settings"] #settings').isVisible() && await p.locator('dialog#settings').count() === 0);
    const taskButton = await p.locator('[data-win-tab="settings"]').boundingBox();
    // The taskbar is PUDL's (.win-bar): a rule along its top, and a press at
    // the screen's edge goes to the nearest control on it.
    ok('the taskbar has a visible pane rule', await p.locator('.win-bar').evaluate(el => {
      const s = getComputedStyle(el); return parseFloat(s.borderTopWidth) >= 1 && s.borderTopStyle === 'solid' && s.borderTopColor !== 'rgba(0, 0, 0, 0)';
    }));
    const bottom = p.viewportSize().height - 1;
    ok('taskbar buttons stand clear of the screen\'s edge', taskButton.y + taskButton.height < bottom);
    await p.mouse.click(taskButton.x + taskButton.width / 2, bottom);
    ok('a click at the bottom edge activates the taskbar button', await p.evaluate(() => !!window.pudlWindows.state().min.settings));
    await p.mouse.click(taskButton.x + taskButton.width / 2, bottom);
    const firstTask = p.locator('[data-win-dock] [data-win-tab]').first();
    const firstTaskKey = await firstTask.getAttribute('data-win-tab');
    ok('the taskbar leaves visible padding on the left', (await firstTask.boundingBox()).x >= 4);
    await p.mouse.click(0, bottom);
    ok('the bottom-left corner activates the first task', await p.evaluate(key => pudlWindows.state().top === key, firstTaskKey));
    await p.mouse.click(p.viewportSize().width - 1, bottom);
    ok('the bottom-right corner minimizes every window, as Show Desktop does', await p.evaluate(() => { const s = pudlWindows.state(); return s.open.filter(k => !s.min[k] && document.querySelector('[data-win-tab="' + k + '"]')).length === 0; }));
    await p.locator('.win-bar a[data-win-restore]').click();
    ok('Restore All brings the windows back', await p.evaluate(() => !pudlWindows.state().min.settings));
    const wide = p.viewportSize();
    await p.evaluate(async () => {
      for (let i = 0; i < 6; i++) await dinahWorkbench.open(dinahWorkbench.resourceItem({ workspaceId: 'C:/Alpha', sourceId: 'local', path: 'padding-' + i + '.md' }, 'padding-' + i + '.md'));
      await document.fonts.ready;
    });
    await p.setViewportSize({ width: 390, height: 720 });
    // Since PUDL 0.81.2 a full taskbar puts the buttons it has no room for
    // into its "N more" button, and the front window's button always stays.
    // Since 0.82.1 that button shrinks below its floor to fit what is left,
    // so the dock never overflows, even at a phone's width.
    await p.waitForFunction(() => {
      const dock = document.querySelector('[data-win-dock]'), more = document.querySelector('.win-bar-more');
      const shown = [...dock.querySelectorAll('[data-win-tab]')].filter(b => b.offsetParent);
      return shown.length >= 1 && new Set(shown.map(b => Math.round(b.getBoundingClientRect().top))).size === 1 &&
        shown.some(b => b.dataset.winTab === pudlWindows.state().top) && more && !more.hidden &&
        dock.scrollWidth <= dock.clientWidth + 1;
    });
    ok('a full taskbar keeps one row, keeps the front window\'s button, puts the rest under "N more", and does not overflow', true);
    await p.setViewportSize(wide);
    await p.evaluate(() => {
      document.querySelectorAll('[data-win-kind="document"]').forEach(win => {
        if (dinahWorkbench.item(win.dataset.win).path.startsWith('padding-')) pudlWindows.close(win.dataset.win);
      });
    });
    await p.evaluate(() => {
      window.__emit('agent:permission', { sessionId: 'C:/Alpha-chat', key: 'a', options: [{ kind: 'allow_once', name: 'Allow Alpha', optionId: 'yes' }] });
      window.__emit('agent:permission', { sessionId: 'C:/Beta-chat', key: 'b', options: [{ kind: 'allow_once', name: 'Allow Beta', optionId: 'yes' }] });
    });
    await p.waitForFunction(() => document.querySelectorAll('#questions .question').length === 2 && document.querySelector('.toast-region .toast'));
    ok('toasts stand above the taskbar and never cover its tray', await p.evaluate(() => {
      const toasts = [...document.querySelectorAll('.toast-region .toast')], bar = document.querySelector('.win-bar').getBoundingClientRect();
      return toasts.length > 0 && toasts.every(t => t.getBoundingClientRect().bottom <= bar.top);
    }));
    await p.click('#status-needs');
    await p.click('#notifications-status-menu [data-cmd="needs-you"]');
    ok('Notifications shows questions from every chat', await p.locator('#questions .question:visible').count() === 2 && await p.locator('[data-win="needs-you"]').getAttribute('data-win-attention') === '2');
    await p.evaluate(() => window.pudlWindows.dock('workspace', 'right'));
    ok('Workspace can move to the opposite dock', await p.evaluate(() => window.pudlWindows.state().place.workspace.mode) === 'dock-right');
    // PUDL 0.57: windows docked at the bottom share the edge under tabs, as
    // at the sides, and a tab can be moved along its strip.
    const docked = await p.evaluate(() => ({ skills: pudlWindows.state().place['skills-browser'].mode, chats: pudlWindows.state().place['chat-browser'].mode }));
    await p.evaluate(() => { pudlWindows.dock('skills-browser', 'bottom'); pudlWindows.dock('chat-browser', 'bottom'); });
    const bottomDock = p.locator('.win-dock-group[data-dock-edge="bottom"]');
    await bottomDock.waitFor();
    ok('windows docked at the bottom share one tab strip', await bottomDock.locator('[data-dock-key]').count() === 2);
    await t.shot('workbench-bottom-dock');
    const bottomOrder = () => p.evaluate(() => (pudlWindows.state().docks.bottom || {}).order || []);
    const firstTab = await bottomDock.locator('[data-dock-key]').first().getAttribute('data-dock-key');
    await bottomDock.locator('[data-dock-key="' + firstTab + '"]').focus();
    await p.keyboard.press('Alt+Shift+ArrowRight');
    await p.waitForFunction(first => ((pudlWindows.state().docks.bottom || {}).order || [])[1] === first, firstTab);
    ok('a bottom tab moves along its strip from the keyboard', (await bottomOrder())[1] === firstTab);
    await p.evaluate(b => { pudlWindows.dock('skills-browser', b.skills.replace('dock-', '')); pudlWindows.dock('chat-browser', b.chats.replace('dock-', '')); }, docked);
    await p.evaluate(() => dinahWorkbench.open('workspace'));
    await p.evaluate(async () => {
      dinahWorkbench.register('menu-fixture', { title: 'Menu Fixture', init: () => ({}) });
      for (let i = 0; i < 11; i++) await dinahWorkbench.open({ kind: 'menu-fixture', path: String(i), workspace: 'Fixture ' + i });
    });
    await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Window$/ }).click();
    ok('Window bounds its direct list and disambiguates duplicate titles', await p.locator('#host-windows > li > button').allTextContents().then(labels => labels.filter(label => /^\d+\./.test(label)).length === 9) && await p.evaluate(() => dinahOpenWindows().filter(item => item.title.startsWith('Menu Fixture (Fixture ')).length === 11));
    await t.shot('menus-window');
    await p.locator('#menubar-panel').getByText('All Windows…', { exact: true }).click();
    await p.fill('#go-to-filter', 'Fixture 10');
    await p.locator('#go-to-list tr').dblclick();
    ok('All Windows finds and raises a window beyond the direct list', await p.evaluate(() => dinahWorkbench.item(pudlWindows.state().top).path === '10'));
    await p.evaluate(() => { document.querySelectorAll('[data-win-kind="menu-fixture"]').forEach(win => pudlWindows.close(win.dataset.win)); pudlWindows.raise('workspace'); });
    await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Tools$/ }).click();
    await t.shot('menus-tools');
    await p.keyboard.press('Escape');
    await t.shot('workbench-wide');
    await p.setViewportSize({ width: 820, height: 700 });
    await p.locator('.win-dock-group[data-dock-rail]').first().waitFor();
    await t.shot('workbench-rails');
    ok('the narrow workbench does not overflow horizontally', await p.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    await p.reload();
    ok('the dock placement is restored from the address', await p.evaluate(() => window.pudlWindows.state().place.workspace.mode) === 'dock-right');
    ok('no duplicate mounted IDs', await p.evaluate(() => { const ids = [...document.querySelectorAll('[id]')].map(el => el.id); return ids.length === new Set(ids).size; }));
    // Close All closes each window as its own Close would: a window that
    // refuses, as an Editor with unsaved changes does, stays, and so does
    // the Workspace pane, which the layer requires.
    await p.evaluate(async () => {
      await dinahWorkbench.open('settings');
      const key = await dinahWorkbench.open(dinahWorkbench.resourceItem({ workspaceId: 'C:/Alpha', sourceId: 'local', path: 'unsaved.md' }, 'unsaved.md'));
      document.querySelector('[data-win="' + CSS.escape(key) + '"]').addEventListener('pudl:window-closing', e => e.preventDefault());
      window.__refusing = key;
      dinahCommands.run('window-close-all');
    });
    await p.waitForFunction(() => !pudlWindows.state().open.includes('settings'));
    ok('Close All Windows leaves a window that refuses and the Workspace pane', await p.evaluate(() => { const open = pudlWindows.state().open; return open.includes(window.__refusing) && open.includes('workspace'); }));
    await p.setViewportSize({ width: 2000, height: 1400 });
    await p.evaluate(async () => { await dinahWorkbench.open('settings'); await dinahWorkbench.open('chat-browser'); });
    for (const edge of ['left', 'right', 'top', 'bottom']) {
      const result = await p.evaluate(edge => {
        pudlWindows.dock('settings', edge); pudlWindows.dockArrange(edge, 'merge'); pudlWindows.dockMode(edge, 'open');
        pudlWindows.dock('chat-browser', edge, { group: 'main', position: 'after' });
        const tree = pudlWindows.state().docks[edge].tree;
        return !!tree.first && !!tree.second && new URLSearchParams(location.search).has('ds.' + edge);
      }, edge);
      ok('split dock and URL state work on ' + edge, result);
      await p.evaluate(edge => pudlWindows.dockArrange(edge, 'merge'), edge);
    }
    ok('desktop opts into outline resizing', await p.locator('[data-win-layer]').getAttribute('data-win-resize-mode') === 'outline');
    const outlineKey = await p.evaluate(() => { dinahWorkbench.register('resize-fixture',{title:'Resize Fixture',mode:'floating',init:()=>({})}); return dinahWorkbench.open({kind:'resize-fixture', path:'outline-resize'}); });
    const outlineWin = p.locator('[data-win="' + outlineKey + '"]');
    const beforeOutline = await outlineWin.boundingBox();
    const resizePoint = await outlineWin.locator('.win-rh[data-edge="se"]').evaluate(el => {
      const r=el.getBoundingClientRect();
      for (const y of [.5,.9,.1]) for (const x of [.5,.9,.1]) {
        const point={x:r.x+r.width*x,y:r.y+r.height*y};
        if(document.elementFromPoint(point.x,point.y)===el)return point;
      }
      return null;
    });
    if (!resizePoint) throw Error('outline resize handle has no exposed target');
    await p.mouse.move(resizePoint.x,resizePoint.y); await p.mouse.down();
    await p.mouse.move(resizePoint.x+70,resizePoint.y+40,{steps:4});
    ok('outline pointer movement keeps mounted content geometry fixed', JSON.stringify(await outlineWin.boundingBox()) === JSON.stringify(beforeOutline) && await p.locator('.win-ghost:not([hidden])').isVisible());
    await t.shot('outline-resize-preview');
    await p.mouse.up();
    const afterOutline = await outlineWin.boundingBox();
    ok('outline release commits geometry', afterOutline.width > beforeOutline.width+60 && afterOutline.height > beforeOutline.height+30);
    await p.evaluate(key => pudlWindows.close(key),outlineKey);
    const ribbonToggle=p.getByRole('button',{name:'Show as ribbon',exact:true});
    await ribbonToggle.click();
    // Since PUDL 0.87.1 the chooser appears only when the ribbon's tabs do
    // not fit: none at full width, one in a narrow window.
    const ribbonChooser=p.getByRole('button',{name:'Ribbon tabs',exact:true});
    await p.waitForTimeout(300);
    ok('the ribbon has no tabs chooser while its tabs fit', !(await ribbonChooser.isVisible().catch(() => false)));
    const ribbonSize=p.viewportSize();
    await p.setViewportSize({width:560,height:ribbonSize.height});
    await ribbonChooser.waitFor({timeout:5000});
    await ribbonChooser.focus(); await p.keyboard.press('ArrowDown');
    ok('ribbon tabs chooser opens from the keyboard', await ribbonChooser.getAttribute('aria-expanded')==='true');
    await t.shot('ribbon-tabs-chooser');
    await p.keyboard.press('Escape');
    ok('ribbon chooser regains focus after Escape', await ribbonChooser.evaluate(el=>document.activeElement===el));
    // The taskbar's entries can be put in order (PUDL 0.89.0), and the order
    // is part of the address Dinah saves with the layout.
    const order = await p.evaluate(() => {
      const before = pudlWindows.state().taskbarOrder, last = before[before.length - 1];
      const moved = pudlWindows.reorderTaskbar(last, { edge: 'start' });
      return { before, after: pudlWindows.state().taskbarOrder, moved, last, url: new URLSearchParams(location.search).get('tb.order') || '' };
    });
    ok('a taskbar entry moves to the start, and the order goes into the window address', order.moved && order.after[0] === order.last && order.url.split(',')[0] === order.last, JSON.stringify(order));
    ok('the taskbar offers Arrange taskbar', await p.locator('.win-bar').getByRole('button', { name: /Arrange taskbar/i }).count() > 0);
  } finally { await t.close(); }
})();
