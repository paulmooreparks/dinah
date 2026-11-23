// This suite checks the menus of the file browser in the Files window and in
// the Open dialog (docs/design/product-applets.md, "Menus of Files and the
// Open Dialog"). It opens the folder's menu on the part of the list that is
// not a row and an entry's menu on a row and on the folder tree, by a
// right-click, Shift+F10 and the menu key, and it uses Copy Path from those
// menus and from the path bar's button.
//
// Go gives a path on the computer in the form of the system it runs on
// (WorkspaceLocalPath, workspace_local_path_test.go). The stub here returns
// what Go would return on the system this suite runs on, built with Node's
// path, and the suite checks that the clipboard is given that path unchanged.
const path = require('path');
const os = require('os');
const { launch, PAGE } = require('./lib/page');
const base = path.join(os.tmpdir(), 'Fixture');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    args: { base, sep: path.sep },
    state: { workspace: base, workspaceId: 'w1', workspaceRecords: [{ id: 'w1', name: 'Alpha Delivery', mountAlias: 'alpha', primarySourceId: 'local', sources: [
      { id: 'local', name: 'Working Documents', mountAlias: 'documents', kind: 'local-folder', path: base },
      { id: 'wiki', name: 'Team Wiki', mountAlias: 'wiki', kind: 'sample-wiki' }] }], recent: [base] },
    app: (args) => {
      // An entry of the local folder is local, as Go marks it, whether or
      // not it can be written; the wiki's entries are not.
      const item = (p, title, directory, kind, source, extra) => Object.assign({ path: p, title, directory, kind, actions: directory ? ['list'] : ['read', 'preview', 'edit-text', 'open-external', 'rename'] },
        source ? { ref: { workspaceId: 'w1', sourceId: source, path: p.split('/').slice(4).join('/') } } : {}, source === 'local' ? { local: true } : {}, extra || {});
      const entries = [
        item('/', '/', true, 'catalog'), item('/workspaces', 'Workspaces', true, 'catalog'),
        Object.assign(item('/workspaces/alpha', 'Alpha Delivery', true, 'workspace'), { ref: { workspaceId: 'w1', sourceId: '', path: '' } }),
        item('/workspaces/alpha/documents', 'Working Documents', true, 'local-folder', 'local', { writable: true }),
        item('/workspaces/alpha/documents/Drafts', 'Drafts', true, 'directory', 'local', { writable: true }),
        item('/workspaces/alpha/documents/locked', 'locked', true, 'directory', 'local'),
        item('/workspaces/alpha/documents/readme.md', 'readme.md', false, 'file', 'local', { writable: true }),
        item('/workspaces/alpha/documents/note.md', 'note.md', false, 'sample-note', 'local', { writable: true }),
        item('/workspaces/alpha/wiki', 'Team Wiki', true, 'sample-wiki', 'wiki'),
        item('/workspaces/alpha/wiki/P-1', 'Design Page', false, 'page', 'wiki', { url: 'https://wiki.example/P-1', actions: ['read', 'preview'] })
      ];
      const stat = p => { const found = entries.find(x => x.path === p); if (!found) throw Error('Resource unavailable'); return JSON.parse(JSON.stringify(found)); };
      return {
        ReadUIState: async () => JSON.parse(localStorage.getItem('fixture-ui') || '{"version":1,"values":{}}'), SaveUIState: async value => localStorage.setItem('fixture-ui', JSON.stringify(value)),
        StatWorkspaceMount: async p => stat(p),
        StatWorkspaceMountExpected: async p => stat(p),
        ResolveWorkspaceResourceMount: async ref => stat(entries.find(x => x.ref && x.ref.workspaceId === ref.workspaceId && x.ref.sourceId === ref.sourceId && x.ref.path === (ref.path || '')).path),
        ListWorkspaceMountsExpected: async (p, start, limit) => window.go.main.App.ListWorkspaceMounts(p, start, limit),
        ListWorkspaceMounts: async p => ({ entries: entries.filter(x => x.path !== '/' && (x.path.slice(0, x.path.lastIndexOf('/')) || '/') === p).map(x => stat(x.path)), nextStart: -1 }),
        PreviewWorkspaceResource: async ref => ({ resource: null, blocks: [{ line: 1, html: '<p>' + ref.path + '</p>' }], problems: [] }),
        // Go's answer on this system: the folder's own path joined with the
        // system's separator, as package filepath joins it.
        WorkspaceLocalPath: async ref => { if (ref.sourceId !== 'local') throw Error('this operation needs a mounted local file'); return [args.base].concat(ref.path ? ref.path.split('/') : []).join(args.sep); },
        ShowWorkspaceResource: async ref => { window.__shown.push(ref); },
        RenameDocumentFile: async () => { throw Error('not renamed in this suite'); },
        OpenWorkspaceResource: async ref => { window.__opened.push(ref); },
        CreateWorkspaceFolder: async (ref, name) => { const made = item('/workspaces/alpha/documents/' + (ref.path ? ref.path + '/' : '') + name, name, true, 'directory', 'local', { writable: true }); entries.push(made); return JSON.parse(JSON.stringify(made)); }
      };
    },
    init: () => { window.__shown = []; window.__opened = []; },
    goto: false,
  });
  const { page, ok } = t;
  const menu = page.locator('.context-menu:popover-open');
  // The rows of the open menu: its words, whether it is disabled, and the
  // reason under a disabled row; separators are left out.
  const rows = async () => { await menu.first().waitFor(); return menu.first().locator('.menu-action').evaluateAll(list => list.map(r => ({ label: r.querySelector('.menubar-words').textContent, disabled: r.getAttribute('aria-disabled') === 'true', reason: (r.querySelector('.menubar-detail') || {}).textContent || '' }))); };
  const labels = list => list.map(r => r.label);
  const choose = async label => { await menu.first().locator('.menu-action').filter({ has: page.locator('.menubar-words', { hasText: new RegExp('^' + label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '$') }) }).click(); await menu.first().waitFor({ state: 'hidden' }); };
  const close = async () => { await page.keyboard.press('Escape'); await menu.first().waitFor({ state: 'hidden' }); };
  const copied = () => page.evaluate(() => { const c = window.__runtime.filter(x => x[0] === 'ClipboardSetText'); return c.length ? c[c.length - 1][1] : null; });
  const copiedAfter = async action => { await page.evaluate(() => { window.__runtime.length = 0; }); await action(); await page.waitForFunction(() => window.__runtime.some(x => x[0] === 'ClipboardSetText')); return copied(); };
  const box = loc => loc.evaluate(el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height].map(Math.round).join(','); });
  try {
    await page.goto(PAGE);
    await t.waitReady();
    // A product's rows for an entry of its kind are in the Files window,
    // and the Open dialog leaves them out.
    await page.evaluate(() => dinahResources.registerEntryCommands({ itemKind: 'sample-note', commands: () => [{ id: 'sample-open-note', label: 'Open in Notes', run: () => {} }] }));
    await page.evaluate(() => dinahResources.openFiles('/workspaces/alpha/documents'));
    const win = page.locator('.win[data-win-kind=files]');
    await win.locator('tr[data-name="readme.md"]').waitFor();
    // Maximized, the window is wide enough to show its folder tree.
    await page.evaluate(() => pudlWindows.snap(dinahWorkbench.key({ kind: 'files', path: '/workspaces/alpha/documents' }), 'maximized'));
    await win.locator('[data-fb="tree-pane"]').waitFor();
    const main = win.locator('[data-fb="main"]');
    const empty = async () => { const b = await main.boundingBox(); return { x: b.x + b.width / 2, y: b.y + b.height - 12 }; };

    /* ---- The Files window ------------------------------------------------ */
    const before = [await box(main), await box(win.locator('.fm-toolbar')), await box(win.locator('tr[data-name="readme.md"]'))];
    let at = await empty();
    await page.mouse.click(at.x, at.y, { button: 'right' });
    const folderMenu = await rows();
    await t.shot('files-menus-folder-menu');
    ok('Files: the folder\'s menu on the empty part of the list has its rows in order', JSON.stringify(labels(folderMenu)) === JSON.stringify(['New Folder…', 'Upload Files…', 'Copy Path', 'Show in Folder', 'New Terminal Here', 'Refresh', 'Show the Preview Pane']), JSON.stringify(labels(folderMenu)));
    ok('Files: nothing is disabled in the folder\'s menu of a folder on the computer that can be written', folderMenu.every(r => !r.disabled), JSON.stringify(folderMenu));
    const during = [await box(main), await box(win.locator('.fm-toolbar')), await box(win.locator('tr[data-name="readme.md"]'))];
    let path1 = await copiedAfter(() => choose('Copy Path'));
    const after = [await box(main), await box(win.locator('.fm-toolbar')), await box(win.locator('tr[data-name="readme.md"]'))];
    ok('Files: nothing moves or resizes as the menu opens and closes', JSON.stringify(before) === JSON.stringify(during) && JSON.stringify(before) === JSON.stringify(after), JSON.stringify([before, during, after]));
    ok('Files: Copy Path of the folder shown gives its path in this system\'s form (' + path.sep + ')', path1 === base, path1);
    await page.mouse.click(at.x, at.y, { button: 'right' });
    await choose('Show in Folder');
    ok('Files: Show in Folder shows the folder shown', await page.evaluate(() => __shown.length === 1 && __shown[0].sourceId === 'local' && __shown[0].path === ''));

    // The path bar's button copies the folder shown's path.
    const pathButton = win.locator('.fm-toolbar [data-copy-path]');
    ok('Files: the path bar ends with a raised Copy Path button with the copy glyph', await pathButton.getAttribute('aria-label') === 'Copy Path' && await pathButton.evaluate(b => b.classList.contains('icon-btn') && b.previousElementSibling.matches('[data-fb="crumbs"]') && !!b.querySelector('.glyph') && b.querySelector('.glyph').style.getPropertyValue('--glyph') === 'var(--glyph-copy)') && await pathButton.isEnabled());
    const barBefore = [await box(pathButton), await box(win.locator('.fm-path'))];
    path1 = await copiedAfter(() => pathButton.click());
    const barAfter = [await box(pathButton), await box(win.locator('.fm-path'))];
    ok('Files: the path bar\'s button copies the folder shown in this system\'s form', path1 === base, path1);
    ok('Files: the copied mark moves nothing in the path bar', JSON.stringify(barBefore) === JSON.stringify(barAfter), JSON.stringify([barBefore, barAfter]));

    // An entry's menu on a row.
    await win.locator('tr[data-name="readme.md"]').click({ button: 'right' });
    const fileMenu = await rows();
    ok('Files: a file\'s menu keeps its rows and gains Show in Folder', JSON.stringify(labels(fileMenu)) === JSON.stringify(['Open', 'Edit as Text', 'Open Externally', 'Show in Folder', 'Copy Path', 'Rename…']), JSON.stringify(labels(fileMenu)));
    path1 = await copiedAfter(() => choose('Copy Path'));
    ok('Files: Copy Path of a file gives its path in this system\'s form', path1 === path.join(base, 'readme.md'), path1);
    // A folder that cannot be written still has a place on the computer.
    await win.locator('tr[data-name="locked"]').click({ button: 'right' });
    const lockedMenu = await rows();
    path1 = await copiedAfter(() => choose('Copy Path'));
    ok('Files: a folder that cannot be written has Show in Folder and Copy Path, which gives its path', labels(lockedMenu).includes('Show in Folder') && path1 === path.join(base, 'locked'), JSON.stringify([labels(lockedMenu), path1]));
    // An entry's menu on the folder tree.
    const treeDrafts = win.locator('[data-fb="tree"] a[data-path="/workspaces/alpha/documents/Drafts"]');
    await treeDrafts.waitFor();
    await treeDrafts.click({ button: 'right' });
    const treeMenu = await rows();
    path1 = await copiedAfter(() => choose('Copy Path'));
    ok('Files: a folder\'s menu on the tree has Open in Files, Show in Folder and Copy Path', ['Open in Files', 'Show in Folder', 'Copy Path'].every(l => labels(treeMenu).includes(l)) && path1 === path.join(base, 'Drafts'), JSON.stringify([labels(treeMenu), path1]));
    await win.locator('tr[data-name="note.md"]').click({ button: 'right' });
    ok('Files: a product\'s rows for an entry of its kind are in the menu', labels(await rows()).includes('Open in Notes'));
    await close();

    // The keyboard: the menu key on a focused row opens the row's menu, and
    // Shift+F10 with focus in the list but on no row opens the folder's.
    await win.locator('tr[data-name="readme.md"]').focus();
    await page.keyboard.press('ContextMenu');
    ok('Files: the menu key on a focused row opens that row\'s menu', JSON.stringify(labels(await rows())) === JSON.stringify(labels(fileMenu)));
    await close();
    at = await empty();
    await page.mouse.click(at.x, at.y);
    ok('Files: a press on the empty part of the list puts focus there', await main.evaluate(el => document.activeElement === el));
    await page.keyboard.press('Shift+F10');
    ok('Files: Shift+F10 with no row focused opens the folder\'s menu', JSON.stringify(labels(await rows())) === JSON.stringify(labels(folderMenu)));
    await close();

    // A folder with no place on the computer: Copy Path is disabled with the
    // reason, Show in Folder is left out, and the path bar's button is
    // disabled with the reason in its tooltip.
    await page.evaluate(() => { const roots = document.querySelectorAll('.dinah-files'); return pudlCommands.run('up', undefined, roots[roots.length - 1]); });
    await win.locator('tr[data-name="documents"]').waitFor();
    at = await empty();
    await page.mouse.click(at.x, at.y, { button: 'right' });
    const awayMenu = await rows();
    const awayCopy = awayMenu.find(r => r.label === 'Copy Path');
    ok('Files: a workspace\'s menu has Copy Path disabled with a short reason, and no Show in Folder', awayCopy && awayCopy.disabled && awayCopy.reason === 'This folder is not on the computer.' && !labels(awayMenu).includes('Show in Folder'), JSON.stringify(awayMenu));
    const newFolder = awayMenu.find(r => r.label === 'New Folder…');
    ok('Files: New Folder… is disabled with its reason where nothing can be written', newFolder && newFolder.disabled && newFolder.reason === 'This folder does not take new files.', JSON.stringify(newFolder));
    await close();
    ok('Files: the path bar\'s button is disabled there, and its tooltip says why', await pathButton.isDisabled() && await pathButton.getAttribute('data-tooltip') === 'Copy Path: This folder is not on the computer.');
    await win.locator('tr[data-name="wiki"]').dblclick();
    await win.locator('tr[data-name="P-1"]').waitFor();
    await win.locator('tr[data-name="P-1"]').click({ button: 'right' });
    const pageMenu = await rows();
    const pageCopy = pageMenu.find(r => r.label === 'Copy Path');
    ok('Files: a page of a wiki has Copy Link, and Copy Path disabled with the reason', labels(pageMenu).includes('Copy Link') && pageCopy && pageCopy.disabled && pageCopy.reason === 'This is not on the computer.' && !labels(pageMenu).includes('Show in Folder'), JSON.stringify(pageMenu));
    await close();
    await t.shot('files-menus-files');
    await page.evaluate(() => { document.querySelectorAll('.win[data-win-kind=files]').forEach(w => pudlWindows.close(w.dataset.win)); });

    /* ---- The Open dialog -------------------------------------------------- */
    const windowsBefore = await page.locator('.win').count();
    await page.evaluate(() => { window.__picked = null; window.__pickDone = dinahFilePicker.pick({ mode: 'open', any: true, title: 'Choose a File', okLabel: 'Choose', start: '/workspaces/alpha/documents', commit: c => { window.__picked = c; return true; } }); });
    const dlg = page.locator('dialog.dinah-picker[open]');
    await dlg.locator('tr[data-name="readme.md"]').waitFor();
    const dmain = dlg.locator('[data-fb="main"]');
    const dempty = async () => { const b = await dmain.boundingBox(); return { x: b.x + b.width / 2, y: b.y + b.height - 12 }; };
    await dlg.locator('tr[data-name="readme.md"]').click({ button: 'right' });
    const pickFile = await rows();
    await t.shot('files-menus-dialog-row-menu');
    ok('Open dialog: a file\'s menu starts with the dialog\'s own action, then Copy Path, Rename… and Show in Folder', JSON.stringify(labels(pickFile)) === JSON.stringify(['Choose', 'Copy Path', 'Rename…', 'Show in Folder']), JSON.stringify(labels(pickFile)));
    path1 = await copiedAfter(() => choose('Copy Path'));
    ok('Open dialog: Copy Path of a file gives its path in this system\'s form, and the dialog stays open', path1 === path.join(base, 'readme.md') && await dlg.isVisible(), path1);
    await dlg.locator('tr[data-name="note.md"]').click({ button: 'right' });
    ok('Open dialog: a product\'s rows are left out', !labels(await rows()).includes('Open in Notes'));
    await close();
    ok('Open dialog: Escape closes the menu and leaves the dialog open', await dlg.isVisible());
    await dlg.locator('tr[data-name="Drafts"]').click({ button: 'right' });
    const pickFolder = await rows();
    ok('Open dialog: a folder\'s menu has Open, Copy Path, Show in Folder and New Folder…', JSON.stringify(labels(pickFolder)) === JSON.stringify(['Open', 'Copy Path', 'Show in Folder', 'New Folder…']), JSON.stringify(labels(pickFolder)));
    await page.evaluate(() => { window.__shown.length = 0; });
    await choose('Show in Folder');
    ok('Open dialog: Show in Folder shows the folder', await page.evaluate(() => __shown.length === 1 && __shown[0].path === 'Drafts'));
    let dat = await dempty();
    await page.mouse.click(dat.x, dat.y, { button: 'right' });
    const pickBack = await rows();
    ok('Open dialog: the folder\'s menu has New Folder…, Copy Path, Show in Folder and Refresh', JSON.stringify(labels(pickBack)) === JSON.stringify(['New Folder…', 'Copy Path', 'Show in Folder', 'Refresh']) && pickBack.every(r => !r.disabled), JSON.stringify(pickBack));
    path1 = await copiedAfter(() => choose('Copy Path'));
    ok('Open dialog: Copy Path of the folder shown gives its path in this system\'s form', path1 === base, path1);
    const dButton = dlg.locator('.fb-pick-bar [data-copy-path]');
    // The button's words are read before it is pressed, since the copied
    // mark stands in for them for a moment after.
    const dShape = await dButton.evaluate(b => b.previousElementSibling.matches('[data-fb="crumbs"]') && b.classList.contains('icon-btn') && b.getAttribute('aria-label') === 'Copy Path');
    path1 = await copiedAfter(() => dButton.click());
    ok('Open dialog: the path bar ends with the Copy Path button, which copies the folder shown', dShape && path1 === base, path1);
    await dlg.locator('tr[data-name="readme.md"]').focus();
    await page.keyboard.press('Shift+F10');
    ok('Open dialog: Shift+F10 on a focused row opens that row\'s menu', JSON.stringify(labels(await rows())) === JSON.stringify(labels(pickFile)));
    await close();
    dat = await dempty();
    await page.mouse.click(dat.x, dat.y);
    await page.keyboard.press('ContextMenu');
    ok('Open dialog: the menu key with no row focused opens the folder\'s menu', JSON.stringify(labels(await rows())) === JSON.stringify(labels(pickBack)));
    await close();
    await t.shot('files-menus-dialog');
    await dlg.locator('tr[data-name="Drafts"]').click({ button: 'right' });
    await choose('Open');
    await dlg.locator('.fm-path [aria-current="location"]').filter({ hasText: 'Drafts' }).waitFor();
    ok('Open dialog: Open on a folder goes into it', true);
    await page.evaluate(() => { const b = document.querySelector('dialog.dinah-picker [data-pick="up"]'); b.click(); });
    await dlg.locator('tr[data-name="readme.md"]').waitFor();
    await dlg.locator('tr[data-name="readme.md"]').click({ button: 'right' });
    await choose('Choose');
    await dlg.waitFor({ state: 'detached' });
    ok('Open dialog: the dialog\'s own action chooses the file and closes the dialog', await page.evaluate(() => !!window.__picked && window.__picked.node && window.__picked.node.ref.path === 'readme.md'));
    ok('Open dialog: its menus opened no window', await page.locator('.win').count() === windowsBefore && await page.evaluate(() => __opened.length === 0));
  } finally { await t.close(); }
})();
