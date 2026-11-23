// A product's own rows in the menu of an entry and of a source
// (docs/design/product-applets.md, "A Product's Rows in an Entry's Menu").
//
// A product's page file registers a function for a kind of entry or for a
// kind of source, and the rows it returns stand in the entry's menu in a
// group of their own after the base's rows, with a danger row among the
// destructive rows. The menu is the one the row's actions button, a
// right-click and the keyboard open, in the Workspace tree, in the Files
// window and in the Pinned list, and in Files the rows are commands of the
// window as the base's are. A page with nothing registered has the menus it
// always had, which is checked first, and the same menus are read again
// after the registration. The stub is a workspace with a folder of files and
// a source of an invented kind whose entries are saved queries, records and
// two folders that are also documents, one of which its source declines to
// open for editing, so its menus have no Edit.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__calls = [];
      const records = [{ id: 'w1', mountAlias: 'w1', name: 'Delivery', primarySourceId: 'files', executionPath: 'C:/Working',
        sources: [{ id: 'files', mountAlias: 'files', kind: 'local-folder', name: 'Working Files', path: 'C:/Working' }, { id: 'records', mountAlias: 'records', kind: 'sample-tracker', name: 'Records' }],
        pins: [{ kind: 'resource', title: 'Open Records', ref: { workspaceId: 'w1', sourceId: 'records', path: 'Q-1' }, itemKind: 'sample-query', directory: true },
          { kind: 'resource', title: 'guide.md', ref: { workspaceId: 'w1', sourceId: 'files', path: 'guide.md' }, itemKind: 'file' },
          { kind: 'resource', title: 'Records', ref: { workspaceId: 'w1', sourceId: 'records', path: '' }, itemKind: 'sample-tracker', directory: true }] }];
      const state = () => ({ workspace: 'C:/Working', workspaceId: 'w1', workspaceRecords: records, workspaces: ['C:/Working'], recent: ['C:/Working'], defaultProvider: 'devin', catalog: [], defaults: {}, pinned: [] });
      // The entries of the source, which the suite changes while they are listed.
      window.__entries = [
        { name: 'Q-1', title: 'Open Records', kind: 'sample-query', directory: true, count: 12, changedCount: 3, status: { label: 'Shared', state: 'accent' } },
        { name: 'Q-2', title: 'Closed Records', kind: 'sample-query', directory: true },
        { name: 'R-1', title: 'Write the import guide', kind: 'sample-record' },
        // Two folders that are also documents. The source declines Edit on
        // the second (resource.Entry.NoEdit), so its item has no edit action.
        { name: 'D-1', title: 'Design notes', kind: 'sample-page', directory: true, actions: ['list', 'read', 'preview', 'edit'] },
        { name: 'D-2', title: 'Issue with files', kind: 'sample-page', directory: true, actions: ['list', 'read', 'preview'] }
      ];
      const files = [{ name: 'guide.md', actions: ['read', 'preview', 'edit', 'edit-text', 'reveal'] }];
      const entriesOf = source => source.id === 'records'
        ? window.__entries.map(e => ({ ref: { workspaceId: 'w1', sourceId: 'records', path: e.name }, title: e.title, kind: e.kind, glyph: e.directory ? 'search' : 'ticket', directory: !!e.directory, revision: '1', actions: e.actions || (e.directory ? ['list'] : []), status: e.status, changed: e.changed, count: e.count, changedCount: e.changedCount }))
        : files.map(f => ({ ref: { workspaceId: 'w1', sourceId: 'files', path: f.name }, title: f.name, kind: 'file', directory: false, revision: '1', writable: true, actions: f.actions }));
      const resolve = virtual => {
        const parts = virtual.split('/').filter(Boolean), record = records.find(r => r.mountAlias === parts[1]);
        const source = record && record.sources.find(s => s.mountAlias === parts[2]);
        if (parts.length > 1 && !record || parts.length > 2 && !source) throw new Error('Mount unavailable');
        return { record, source, rest: parts.slice(3).join('/') };
      };
      const mount = (source, resource) => JSON.parse(JSON.stringify({ ...resource, path: '/workspaces/w1/' + source.mountAlias + (resource.ref.path ? '/' + resource.ref.path : '') }));
      const root = source => ({ ref: { workspaceId: 'w1', sourceId: source.id, path: '' }, title: source.name, kind: source.kind, directory: true, revision: '1', writable: source.kind === 'local-folder', actions: ['list'] });
      const stat = async virtual => {
        const { record, source, rest } = resolve(virtual);
        if (!source) return { path: virtual, title: record ? record.name : 'Workspaces', kind: 'catalog', directory: true, actions: ['list'] };
        if (!rest) return mount(source, root(source));
        const found = entriesOf(source).find(e => e.ref.path === rest);
        if (!found) throw new Error('not found: ' + virtual);
        return mount(source, found);
      };
      const list = async virtual => {
        const { record, source, rest } = resolve(virtual);
        if (source) return { entries: rest ? [] : entriesOf(source).map(e => mount(source, e)), nextStart: -1 };
        return { entries: record ? record.sources.map(s => mount(s, root(s))) : [], nextStart: -1 };
      };
      return {
        State: async () => state(), ReadUIState: async () => JSON.parse(localStorage.getItem('fixture-ui') || '{"version":1,"values":{}}'), SaveUIState: async value => localStorage.setItem('fixture-ui', JSON.stringify(value)),
        AgentProviders: async () => [{ id: 'devin', name: 'Devin', installed: true, available: true }],
        StatWorkspaceMount: stat, StatWorkspaceMountExpected: stat, ListWorkspaceMounts: list, ListWorkspaceMountsExpected: list,
        ResolveWorkspaceResourceMount: async ref => { const source = records[0].sources.find(s => s.id === ref.sourceId); return stat('/workspaces/w1/' + source.mountAlias + (ref.path ? '/' + ref.path : '')); },
        PreviewWorkspaceResource: async ref => ({ resource: { ref, title: ref.path, kind: 'file', actions: ['preview'] }, blocks: [{ line: 1, html: '<p>Preview</p>' }], problems: [] })
      };
    },
  });
  const { page, ok } = t;
  const frame = () => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
  try {
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    const sources = page.locator('#workspace-sources'), pins = page.locator('#workspace-pins');
    await sources.getByRole('button', { name: 'Expand Records', exact: true }).click();
    await sources.getByRole('button', { name: 'Expand Working Files', exact: true }).click();
    const row = name => sources.locator('a[data-resource]').filter({ has: page.locator('.fm-ti-label', { hasText: new RegExp('^' + name) }) }).first();
    const more = name => row(name).locator('xpath=..').locator(':scope > [data-row-actions]');
    const pin = title => pins.locator('.browser-row').filter({ has: page.locator('.browser-row-title', { hasText: new RegExp('^' + title + '$') }) }).first();
    await row('Write the import guide').waitFor();
    await row('guide.md').waitFor();
    await pin('Open Records').waitFor();

    // The rows of the open menu, in order, with '-' for a separator.
    const menu = page.locator('.context-menu:popover-open');
    const read = () => menu.first().evaluate(panel => Array.from(panel.children).map(el => el.matches('hr') ? '-' : {
      label: el.querySelector('.menubar-words').textContent, danger: el.classList.contains('danger'), disabled: el.getAttribute('aria-disabled') === 'true',
      detail: el.querySelector('.menubar-detail') ? el.querySelector('.menubar-detail').textContent : '', glyph: el.querySelector('.glyph') ? el.querySelector('.glyph').style.getPropertyValue('--glyph') : '' }));
    const labels = rows => rows.map(r => r === '-' ? '-' : r.label);
    const close = async () => { await page.keyboard.press('Escape'); await menu.first().waitFor({ state: 'detached' }); };
    const rowsOf = async open => { await open(); await menu.first().waitFor(); const rows = await read(); await close(); return rows; };
    await page.evaluate(() => dinahResources.openFiles('/workspaces/w1/records', { workspaceId: 'w1', sourceId: 'records', path: '' }));
    const filesWin = page.locator('.dinah-files').last();
    await filesWin.locator('.fm-row[data-name="Q-1"]').waitFor();
    // The window is made wide enough to show its folder tree beside its list.
    await filesWin.evaluate(root => { const win = root.closest('.win'); for (const name of ['width', 'min-width', 'max-width']) win.style.setProperty(name, '680px', 'important'); win.style.setProperty('left', '330px', 'important'); });
    await filesWin.locator('[data-fb="tree"] a[data-path="/workspaces/w1/records"]').waitFor();
    // Every menu for an entry or a source, each opened the ways it can be.
    const MENUS = {
      'tree, a query, by its button': () => more('Open Records').click(),
      'tree, a query, by a right-click': () => row('Open Records').click({ button: 'right' }),
      'tree, a query, by the keyboard': async () => { await row('Open Records').focus(); await page.keyboard.press('Shift+F10'); },
      'tree, another query, by its button': () => more('Closed Records').click(),
      'tree, a record, by its button': () => more('Write the import guide').click(),
      'tree, a file, by a right-click': () => row('guide.md').click({ button: 'right' }),
      'tree, the source, by its button': () => more('Records').click(),
      'tree, the source, by a right-click': () => row('Records').click({ button: 'right' }),
      'tree, the folder of files, by its button': () => more('Working Files').click(),
      'Files, a query, by a right-click': () => filesWin.locator('.fm-row[data-name="Q-1"]').click({ button: 'right' }),
      'Files, a query, by the keyboard': async () => { await filesWin.locator('.fm-row[data-name="Q-1"]').click(); await page.keyboard.press('Shift+F10'); },
      'Files, a record, by a right-click': () => filesWin.locator('.fm-row[data-name="R-1"]').click({ button: 'right' }),
      'Files, the source in the folder tree, by a right-click': () => filesWin.locator('[data-fb="tree"] a[data-path="/workspaces/w1/records"]').click({ button: 'right' }),
      'Pinned, a query, by its button': () => pin('Open Records').locator('[data-row-actions]').click(),
      'Pinned, a query, by a right-click': () => pin('Open Records').click({ button: 'right' }),
      'Pinned, a file, by its button': () => pin('guide.md').locator('[data-row-actions]').click(),
      'Pinned, the source, by its button': () => pin('Records').locator('[data-row-actions]').click()
    };
    const readAll = async () => { const out = {}; for (const name of Object.keys(MENUS)) out[name] = await rowsOf(MENUS[name]); return out; };
    // The Files window's commands for what is selected: the Actions menu, and what an agent lists.
    const filesCommands = name => page.evaluate(async name => {
      const roots = document.querySelectorAll('.dinah-files'), root = roots[roots.length - 1];
      root.querySelector('.fm-row[data-name="' + name + '"]').click();
      await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      const list = pudlCommands.list(root), actions = list.find(c => c.id === 'files-actions');
      return { ids: list.filter(c => !c.hidden).map(c => c.id), all: list.map(c => ({ id: c.id, label: c.label, disabled: !!c.disabled, reason: c.reason || '', danger: !!c.danger, hidden: !!c.hidden })), actions: actions ? actions.items || null : null };
    }, name);

    /* ---- A page with nothing registered --------------------------------- */
    const before = await readAll();
    ok('with nothing registered, a query\'s menu in the tree has the base\'s rows', same(labels(before['tree, a query, by its button']), ['Open in Files', 'Unpin from Workspace', 'Refresh Children']), JSON.stringify(labels(before['tree, a query, by its button'])));
    ok('with nothing registered, the source\'s menu ends with its destructive group', same(labels(before['tree, the source, by its button']), ['Open in Files', 'Unpin from Workspace', 'Refresh Source', '-', 'Remove from Workspace…']) && before['tree, the source, by its button'][4].danger, JSON.stringify(labels(before['tree, the source, by its button'])));
    ok('with nothing registered, the button, a right-click and the keyboard open the same menu', same(before['tree, a query, by its button'], before['tree, a query, by a right-click']) && same(before['tree, a query, by its button'], before['tree, a query, by the keyboard']) && same(before['tree, the source, by its button'], before['tree, the source, by a right-click']));
    ok('with nothing registered, a pinned entry\'s menu has the base\'s rows', same(labels(before['Pinned, a query, by its button']), ['Unpin from Workspace', 'Move Up', 'Move Down']), JSON.stringify(labels(before['Pinned, a query, by its button'])));
    // Since 23 November 2026 Files keeps Copy Path in the menu of an entry
    // with no place on the computer, disabled with the reason.
    ok('with nothing registered, an entry\'s menu in Files has the base\'s rows', same(labels(before['Files, a query, by a right-click']), ['Open in Files', 'Copy Path']) && before['Files, a query, by a right-click'][1].disabled, JSON.stringify(before['Files, a query, by a right-click']));
    ok('with nothing registered, nothing is asked for', await page.evaluate(() => dinahResources.entryCommands({ kind: 'sample-query', ref: { workspaceId: 'w1', sourceId: 'records', path: 'Q-1' } }).length === 0));
    const commandsBefore = await filesCommands('Q-1');

    /* ---- A source that declines Edit on a folder that is also a document -- */
    const docTree = await rowsOf(() => more('Design notes').click());
    const declinedTree = await rowsOf(() => more('Issue with files').click());
    ok('a folder that is also a document has Edit, Open in Files and Refresh Children in the tree', ['Edit', 'Open in Files', 'Refresh Children'].every(l => labels(docTree).includes(l)), JSON.stringify(labels(docTree)));
    ok('one whose source declines Edit has the same rows in the tree without Edit', same(labels(declinedTree), labels(docTree).filter(l => l !== 'Edit')), JSON.stringify(labels(declinedTree)));
    const docFiles = await rowsOf(() => filesWin.locator('.fm-row[data-name="D-1"]').click({ button: 'right' }));
    const declinedFiles = await rowsOf(() => filesWin.locator('.fm-row[data-name="D-2"]').click({ button: 'right' }));
    ok('in Files a folder that is also a document has Edit, and one whose source declines it has the same rows without Edit', labels(docFiles).includes('Edit') && same(labels(declinedFiles), labels(docFiles).filter(l => l !== 'Edit')), JSON.stringify([labels(docFiles), labels(declinedFiles)]));
    const declinedCommands = await filesCommands('D-2'), docCommands = await filesCommands('D-1');
    ok('in Files the declined entry has no edit command and keeps the others', docCommands.ids.includes('edit') && !declinedCommands.ids.includes('edit') && same(declinedCommands.ids, docCommands.ids.filter(id => id !== 'edit')), JSON.stringify([docCommands.ids, declinedCommands.ids]));

    /* ---- A product registers its rows ----------------------------------- */
    const registered = await page.evaluate(() => {
      const errors = []; const real = console.error; console.error = function (text) { errors.push(String(text)); };
      window.__ran = []; window.__asked = [];
      const note = name => function (item) { __ran.push({ by: name, path: item.ref && item.ref.path, kind: item.kind || '', title: item.title || '', status: item.status ? item.status.label : '', count: item.count, changedCount: item.changedCount, changed: !!item.changed, mounted: item.path || '', actions: item.actions || null }); };
      const out = {
        kind: dinahResources.registerEntryCommands({ itemKind: 'sample-query', commands: function (item) {
          __asked.push(item.ref.path + ':' + (item.changed ? 'changed' : 'seen'));
          return [
            { id: 'sample-open-board', label: 'Open in Sample Board', glyph: 'board', run: note('open-board') },
            { id: 'sample-mark-seen', label: item.changed ? 'Mark as Seen' : 'Nothing New', disabled: !item.changed, reason: 'Nothing has changed since you last looked.', run: note('mark-seen') },
            { id: 'sample-fail', label: 'Run the Query Again', run: function () { return Promise.reject(new Error('The sample site did not answer.')); } },
            { id: 'sample-remove', label: 'Remove…', danger: true, run: note('remove') },
            { id: 'open', label: 'Takes a Base Row\'s ID', run: note('collides') },
            { id: 'sample-open-board', label: 'Has Another Row\'s ID', run: note('twice') },
            { id: 'sample-no-run', label: 'Has Nothing to Run' }
          ];
        } }),
        source: dinahResources.registerEntryCommands({ sourceKind: 'sample-tracker', commands: function (item) {
          return [{ id: 'sample-choose', label: 'Choose Queries…', glyph: 'gear', run: note('choose') }, { id: 'sample-sign-out', label: 'Sign Out of the Sample Site…', danger: true, run: note('sign-out') }];
        } }),
        again: dinahResources.registerEntryCommands({ itemKind: 'sample-query', commands: function () { return [{ id: 'second', label: 'Second', run: note('second') }]; } }),
        againSource: dinahResources.registerEntryCommands({ sourceKind: 'sample-tracker', commands: function () { return []; } }),
        both: dinahResources.registerEntryCommands({ itemKind: 'x', sourceKind: 'y', commands: function () { return []; } }),
        neither: dinahResources.registerEntryCommands({ commands: function () { return []; } }),
        noFunction: dinahResources.registerEntryCommands({ itemKind: 'z' })
      };
      console.error = real;
      return { out, errors };
    });
    ok('rows register for an item kind and for a source kind', registered.out.kind === true && registered.out.source === true);
    ok('a second registration of the same key is refused with a clear message, and the first stays', registered.out.again === false && registered.out.againSource === false && registered.errors.some(e => /already registered for the item kind "sample-query"/.test(e) && /first registration stays/.test(e)) && registered.errors.some(e => /already registered for the source kind "sample-tracker"/.test(e)), JSON.stringify(registered.errors));
    ok('a registration with both keys, with neither or without a function is refused', !registered.out.both && !registered.out.neither && !registered.out.noFunction && registered.errors.length === 5, JSON.stringify(registered.errors));

    await page.evaluate(() => { window.__errors = []; const real = console.error; console.error = function (text) { __errors.push(String(text)); return real.apply(console, arguments); }; });
    const after = await readAll();
    const QUERY = ['-', 'Open in Sample Board', 'Nothing New', 'Run the Query Again', '-', 'Remove…'];
    const tree = after['tree, a query, by its button'];
    ok('a query\'s menu in the tree has the base\'s rows, then the product\'s in a group of their own, then its danger row in a group at the end', same(labels(tree), labels(before['tree, a query, by its button']).concat(QUERY)), JSON.stringify(labels(tree)));
    ok('the button, a right-click and the keyboard open the same menu with the product\'s rows', same(tree, after['tree, a query, by a right-click']) && same(tree, after['tree, a query, by the keyboard']));
    const named = (rows, label) => rows.find(r => r !== '-' && r.label === label);
    ok('a product\'s row has its glyph, and its danger row is drawn as a destructive row', named(tree, 'Open in Sample Board').glyph === 'var(--glyph-board)' && named(tree, 'Remove…').danger && !named(tree, 'Open in Sample Board').danger, JSON.stringify(tree));
    ok('a disabled row is shown, disabled, with its reason', named(tree, 'Nothing New').disabled && named(tree, 'Nothing New').detail === 'Nothing has changed since you last looked.' && !named(tree, 'Open in Sample Board').disabled, JSON.stringify(named(tree, 'Nothing New')));
    ok('a row whose id a base row has, a second row with one id and a row with nothing to run are left out, with a message', !labels(tree).some(l => /Takes a Base|Has Another|Has Nothing/.test(l)) && await page.evaluate(() => __errors.some(e => /"open" has the id of one of the base's own rows/.test(e)) && __errors.some(e => /"sample-open-board" has the id of another row/.test(e)) && __errors.some(e => /"sample-no-run" needs an id, a label and a run function/.test(e))), JSON.stringify(await page.evaluate(() => __errors)));
    ok('a refused row is said once, however often the menu opens', await page.evaluate(() => __errors.length === 3), JSON.stringify(await page.evaluate(() => __errors)));
    const source = after['tree, the source, by its button'];
    ok('the source\'s menu has the product\'s row in a group of its own, and its danger row in the destructive group before Remove from Workspace…', same(labels(source), ['Open in Files', 'Unpin from Workspace', 'Refresh Source', '-', 'Choose Queries…', '-', 'Sign Out of the Sample Site…', 'Remove from Workspace…']) && source[6].danger && source[7].danger && same(source, after['tree, the source, by a right-click']), JSON.stringify(labels(source)));
    ok('a source\'s rows are not on its entries, and an entry\'s rows are not on its source', !labels(tree).includes('Choose Queries…') && !labels(source).includes('Open in Sample Board'));
    ok('the same rows are in the menu in Files, by a right-click and by the keyboard', same(labels(after['Files, a query, by a right-click']), labels(before['Files, a query, by a right-click']).concat(QUERY)) && same(after['Files, a query, by a right-click'], after['Files, a query, by the keyboard']), JSON.stringify(labels(after['Files, a query, by a right-click'])));
    ok('the source\'s rows are in its menu in the folder tree of Files', same(labels(after['Files, the source in the folder tree, by a right-click']), labels(before['Files, the source in the folder tree, by a right-click']).concat(['-', 'Choose Queries…', '-', 'Sign Out of the Sample Site…'])), JSON.stringify(labels(after['Files, the source in the folder tree, by a right-click'])));
    ok('the same rows are in the menu in the Pinned list, by the button and by a right-click', same(labels(after['Pinned, a query, by its button']), ['Unpin from Workspace', 'Move Up', 'Move Down'].concat(QUERY)) && same(after['Pinned, a query, by its button'], after['Pinned, a query, by a right-click']), JSON.stringify(labels(after['Pinned, a query, by its button'])));
    ok('a pinned source has the source\'s rows', same(labels(after['Pinned, the source, by its button']), labels(before['Pinned, the source, by its button']).concat(['-', 'Choose Queries…', '-', 'Sign Out of the Sample Site…'])), JSON.stringify(labels(after['Pinned, the source, by its button'])));
    const UNTOUCHED = ['tree, a record, by its button', 'tree, a file, by a right-click', 'tree, the folder of files, by its button', 'Files, a record, by a right-click', 'Pinned, a file, by its button'];
    ok('the menu of an entry of a kind with nothing registered is exactly as it was', UNTOUCHED.every(name => same(before[name], after[name])), JSON.stringify(UNTOUCHED.map(name => [labels(before[name]), labels(after[name])])));
    ok('in every menu the base\'s rows are the ones it had, in their order', Object.keys(MENUS).every(name => { const had = labels(before[name]).filter(l => l !== '-'), has = labels(after[name]).filter(l => l !== '-'); return same(has.filter(l => had.includes(l)), had); }));

    // The menu as a person sees it, in both themes.
    for (const theme of ['light', 'dark']) {
      await page.evaluate(theme => pudlSetTheme(theme), theme);
      await more('Open Records').click();
      await menu.first().waitFor();
      await frame();
      await t.shot('resource-entry-commands-tree-' + theme);
      await close();
      await more('Records').click();
      await menu.first().waitFor();
      await frame();
      await t.shot('resource-entry-commands-source-' + theme);
      await close();
    }
    await page.evaluate(() => pudlSetTheme('light'));

    /* ---- Running a row ---------------------------------------------------- */
    const run = async (open, label) => { await page.evaluate(() => { __ran.length = 0; }); await open(); await menu.first().waitFor(); await menu.first().locator('.menu-action').filter({ has: page.locator('.menubar-words', { hasText: new RegExp('^' + label + '$') }) }).click(); await frame(); await page.waitForTimeout(50); return page.evaluate(() => __ran.slice()); };
    let ran = await run(MENUS['tree, a query, by its button'], 'Open in Sample Board');
    ok('a row runs with the entry\'s item: its reference, kind, title, status and counts', ran.length === 1 && ran[0].by === 'open-board' && ran[0].path === 'Q-1' && ran[0].kind === 'sample-query' && ran[0].title === 'Open Records' && ran[0].status === 'Shared' && ran[0].count === 12 && ran[0].changedCount === 3 && ran[0].mounted === '/workspaces/w1/records/Q-1', JSON.stringify(ran));
    await row('Open Records').focus();
    await page.keyboard.press('Shift+F10');
    await menu.first().waitFor();
    await page.evaluate(() => { __ran.length = 0; });
    await page.keyboard.press('End');
    await page.keyboard.press('Enter');
    await frame();
    ok('a row is chosen from the keyboard as a base row is', await page.evaluate(() => __ran.length === 1 && __ran[0].by === 'remove' && __ran[0].path === 'Q-1'), JSON.stringify(await page.evaluate(() => __ran)));
    await page.evaluate(() => { __ran.length = 0; });
    await more('Open Records').click();
    await menu.first().waitFor();
    await menu.first().locator('.menu-action[aria-disabled="true"]').click({ force: true });
    await frame();
    ok('a disabled row does not run, and the menu stays open', await page.evaluate(() => __ran.length === 0) && await menu.count() === 1);
    await close();
    ran = await run(MENUS['tree, the source, by its button'], 'Choose Queries…');
    ok('a source\'s row runs with the source\'s item', ran.length === 1 && ran[0].by === 'choose' && ran[0].kind === 'sample-tracker' && ran[0].mounted === '/workspaces/w1/records', JSON.stringify(ran));
    ran = await run(MENUS['Pinned, a query, by its button'], 'Open in Sample Board');
    ok('a row of a pinned entry runs with the entry\'s whole item, read then', ran.length === 1 && ran[0].path === 'Q-1' && ran[0].kind === 'sample-query' && ran[0].status === 'Shared' && ran[0].count === 12 && Array.isArray(ran[0].actions), JSON.stringify(ran));
    ran = await run(MENUS['Files, a query, by a right-click'], 'Open in Sample Board');
    ok('a row in Files runs with the entry\'s item', ran.length === 1 && ran[0].path === 'Q-1' && ran[0].title === 'Open Records' && ran[0].count === 12, JSON.stringify(ran));

    // A run that fails is reported as a base row's failure is: by the
    // window's own message in the Workspace pane, and in the status line
    // of Files.
    await more('Open Records').click();
    await menu.first().waitFor();
    await menu.first().getByText('Run the Query Again', { exact: true }).click();
    const toast = page.locator('.toast').filter({ hasText: 'The sample site did not answer.' });
    await toast.first().waitFor();
    ok('a run that fails in the tree is reported as a base row\'s failure is', await toast.count() >= 1);
    await t.shot('resource-entry-commands-failure');
    await filesWin.locator('.fm-row[data-name="Q-1"]').click({ button: 'right' });
    await menu.first().waitFor();
    await menu.first().getByText('Run the Query Again', { exact: true }).click();
    await page.waitForFunction(() => { const roots = document.querySelectorAll('.dinah-files'), status = roots[roots.length - 1].querySelector('.fm-status'); return /The sample site did not answer\./.test(status.textContent) && status.classList.contains('fm-status-error'); });
    ok('a run that fails in Files is reported in its status line, as a base row\'s failure is', true);

    // The function is asked when the menu opens, so the rows are for the
    // entry as it is then.
    await page.evaluate(() => { __asked.length = 0; __entries[0].changed = true; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await row('Open Records').locator('.resource-changed').waitFor();
    const fresh = await rowsOf(MENUS['tree, a query, by its button']);
    ok('the rows are made when the menu opens, for the entry as it is then', named(fresh, 'Mark as Seen') && !named(fresh, 'Mark as Seen').disabled && !named(fresh, 'Mark as Seen').detail && await page.evaluate(() => __asked.includes('Q-1:changed')), JSON.stringify(labels(fresh)));
    await pins.locator('.resource-changed').first().waitFor();
    const freshPin = await rowsOf(MENUS['Pinned, a query, by its button']);
    ok('a pinned entry\'s rows are made from what is known of the entry then', !!named(freshPin, 'Mark as Seen') && !named(freshPin, 'Mark as Seen').disabled, JSON.stringify(labels(freshPin)));

    /* ---- The Files window's commands -------------------------------------- */
    const commands = await filesCommands('Q-1');
    const cmd = id => commands.all.find(c => c.id === id);
    ok('in Files the product\'s rows are commands of the window, after the base\'s commands', ['sample-open-board', 'sample-mark-seen', 'sample-fail', 'sample-remove'].every(id => cmd(id) && !cmd(id).hidden) && !commandsBefore.all.some(c => /^sample-/.test(c.id)) && same(commands.all.filter(c => !/^sample-/.test(c.id)).map(c => c.id), commandsBefore.all.map(c => c.id)), JSON.stringify(commands.ids));
    ok('a command of the product has its label, its danger mark and, when disabled, its reason', cmd('sample-open-board').label === 'Open in Sample Board' && cmd('sample-remove').danger && !cmd('sample-mark-seen').disabled, JSON.stringify(commands.all.filter(c => /^sample-/.test(c.id))));
    await filesWin.locator('.fm-row[data-name="Q-1"]').click();
    await filesWin.getByRole('button', { name: 'Actions', exact: true }).click();
    const actions = page.locator('.dinah-files .menu-panel:popover-open').last();
    await actions.waitFor();
    const actionRows = await actions.evaluate(panel => Array.from(panel.children).filter(el => !el.hidden).map(el => el.matches('hr') ? '-' : el.textContent.trim()));
    ok('the Actions menu of Files has the product\'s rows after the base\'s, and its danger row in a group at the end', same(actionRows.slice(-6), ['-', 'Open in Sample Board', 'Mark as Seen', 'Run the Query Again', '-', 'Remove…']) && actionRows.indexOf('New Terminal Here') === actionRows.length - 7, JSON.stringify(actionRows));
    await t.shot('resource-entry-commands-files-actions');
    await page.keyboard.press('Escape');
    // The window's own menu in the menu bar has the same commands.
    await page.locator('nav[data-menubar] .menubar-title').filter({ hasText: /^Files$/ }).click();
    const barPanel = page.locator('.menubar-panel:popover-open').last();
    await barPanel.waitFor();
    const barRows = await barPanel.evaluate(panel => Array.from(panel.children).filter(el => !el.hidden).map(el => el.matches('hr, [role="separator"]') ? '-' : (el.querySelector('.menubar-words') || el).textContent.trim()));
    ok('the window\'s menu in the menu bar has the product\'s rows after the base\'s', same(barRows.slice(-6), ['-', 'Open in Sample Board', 'Mark as Seen', 'Run the Query Again', '-', 'Remove…']), JSON.stringify(barRows));
    await page.keyboard.press('Escape');
    ran = await page.evaluate(async () => { __ran.length = 0; const roots = document.querySelectorAll('.dinah-files'); await pudlCommands.run('sample-open-board', undefined, roots[roots.length - 1], { owner: { kind: 'agent' } }); return __ran.slice(); });
    ok('a command of the product runs through the window\'s commands, as an agent runs one, with the selected entry\'s item', ran.length === 1 && ran[0].by === 'open-board' && ran[0].path === 'Q-1', JSON.stringify(ran));
    await page.evaluate(() => { delete __entries[0].changed; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await page.waitForFunction(() => !document.querySelector('#workspace-sources .resource-changed'));
    await frame();
    const refused = await page.evaluate(async () => { const roots = document.querySelectorAll('.dinah-files'); try { await pudlCommands.run('sample-mark-seen', undefined, roots[roots.length - 1]); return ''; } catch (err) { return String(err && err.message || err); } });
    ok('a disabled command refuses to run and gives its reason', refused === 'Nothing has changed since you last looked.', refused);
    const record = await filesCommands('R-1');
    ok('an entry of a kind with nothing registered adds no command in Files', same(record.all.map(c => c.id), commandsBefore.all.map(c => c.id)), JSON.stringify(record.ids));
    await t.shot('resource-entry-commands-end');
  } finally { await t.close(); }
})();
