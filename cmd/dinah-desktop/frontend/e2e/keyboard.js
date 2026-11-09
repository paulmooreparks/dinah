// The keyboard: the keys that work everywhere, the Editor's own keys as
// its menus declare them to the menu bar, Help › Keyboard Shortcuts, which
// lists Dinah's from the same declarations, the menu bar's letters, and a
// desktop host's Alt and F10. The Editor is yavc.app's
// (docs/design/content-applets.md), so its keys inside the text are its
// own to test; what is Dinah's here is that they reach the bar once each.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 2560, height: 1200 },
    state: { workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'], windowModes: { editor: 'maximized' }, windowModesShipped: { editor: 'maximized' }, titles: {}, templates: [],
      documentViews: { 'text/markdown': 'text' },
      workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [{ id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] },
    app: () => {
      let n = 0;
      const rev = () => 'r' + (++n);
      const drafts = {};
      const get = id => { const d = drafts[id]; if (!d) throw new Error('the draft no longer exists'); return d; };
      const view = d => Object.assign({}, d, { spans: {}, suggestions: [], changed: d.text !== d.baseline.text, writeBack: 'save', actions: ['update', 'discard', 'save-as', 'save'] });
      return {
        OpenDraft: async ref => {
          let d = Object.values(drafts).find(x => x.origin.path === ref.path);
          if (!d) {
            const text = '# Notes\n\nFirst line.\n\nSecond line.\n';
            const id = 'k' + Object.keys(drafts).length.toString().padStart(31, '0');
            d = drafts[id] = { id, origin: { kind: 'local-folder', workspaceId: 'w1', sourceId: 'local', path: ref.path }, baseline: { text, revision: 'b1', title: ref.path }, title: ref.path, text, revision: rev() };
          }
          return view(d);
        },
        GetDraft: async id => view(get(id)), ListDrafts: async () => Object.values(drafts).map(view),
        UpdateDraft: async (id, expected, text) => { const d = get(id); if (d.revision !== expected) throw new Error('stale'); d.text = text; d.revision = rev(); return view(d); },
        SaveDraft: async id => { const d = get(id); window.__calls.push(['save', d.origin.path]); d.baseline = { text: d.text, revision: 'b2', title: d.title }; return Object.assign(view(d), { notes: [] }); },
        DraftStamp: async id => get(id).revision,
        CompareDraft: async id => ({ moved: false, revision: 'b1', title: get(id).title, theirs: '', mine: '', merged: get(id).text, conflicts: [] })
      };
    },
    init: () => { window.__calls = []; },
    ready: () => window.dinahEditor && window.dinahWorkspaceBrowser && window.dinahWorkspaceBrowser.records().length === 1,
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    // The menu bar's warnings are gathered from the page's first script on,
    // so the listener is in place before the page is opened. The applets'
    // own menus are theirs to keep clean; Dinah's are checked here.
    const menuWarnings = [];
    p.on('console', m => { if (m.type() === 'warning' && m.text().startsWith('pudl-menubar: Dinah:')) menuWarnings.push(m.text()); });
    await p.goto(PAGE);
    await t.waitReady();
    const key = await p.evaluate(() => dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'notes.md' }));
    const ed = p.locator('[data-win="' + key + '"]');
    await ed.locator('.cm-content').waitFor({ timeout: 30000 });
    await p.waitForTimeout(500);
    ok('a Markdown document set to open as text opens in the Editor', await ed.getAttribute('data-win-kind') === 'editor');
    const text = () => ed.locator('.cm-content').evaluate(el => [...el.querySelectorAll('.cm-line')].map(l => l.textContent).join('\n'));

    // The Editor's keys reach the menu bar once each, with Dinah's.
    const keys = await p.evaluate(() => pudlMenubar.shortcuts().map(k => ({ key: k.shown, label: k.label })));
    const has = (key, label) => keys.some(k => k.key === key && new RegExp(label).test(k.label));
    ok('the menu bar lists Save, Undo and Find from the Editor\'s menus', has('Ctrl+S', '^Save') && has('Ctrl+Z', 'Undo') && has('Ctrl+F', 'Find'), JSON.stringify(keys));
    ok('and Save As…, Find and Open and Close from Dinah\'s', has('Ctrl+Shift+S', 'Save As') && has('Ctrl+P', 'Find and Open') && has('Ctrl+F4', 'Close'));
    const counts = {};
    keys.forEach(k => { counts[k.key] = (counts[k.key] || 0) + 1; });
    // Ctrl+O is the Editor's File › Open… while the Editor is in front; Dinah's Documents › Open… gives it up then.
    const twice = Object.keys(counts).filter(k => counts[k] > 1);
    ok('no key is bound to two rows', twice.length === 0, JSON.stringify(twice));

    // Ctrl+S in the text saves through the Editor's File menu; Dinah's
    // Save row stands aside for it (documents.js).
    await ed.locator('.cm-content').click();
    await p.keyboard.press('Control+End');
    await p.keyboard.type(' Typed.');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'save' && c[1] === 'notes.md'), null, { timeout: 15000 });
    const saves = await p.evaluate(() => __calls.filter(c => c[0] === 'save').length);
    await p.keyboard.type(' More.');
    await p.keyboard.press('Control+s');
    await p.waitForFunction(n => __calls.filter(c => c[0] === 'save').length > n, saves, { timeout: 15000 });
    ok('Ctrl+S in the text saves the document', (await text()).includes('More.'));
    ok('the window\'s title stays the document\'s name while typing', (await ed.locator('.win-title').textContent()).trim() === 'notes.md · Fixture');

    // Global keys and the reference.
    await ed.locator('.cm-content').click();
    await p.keyboard.press('F1');
    const help = p.locator('dialog.keyboard-help[open]');
    await help.waitFor();
    const helpText = await help.locator('tbody').textContent();
    ok('F1 lists the keys that work everywhere first, and no applet\'s', helpText.includes('Ctrl+Shift+I') && helpText.includes('Ctrl+PageDown') && helpText.includes('Ctrl+Tab') && helpText.includes('Shift+F10') && !helpText.includes('Bold'));
    const forList = await help.locator('#keyboard-help-for option').allTextContents();
    ok('Keys For lists the applets that declared keys', ['Chat', 'Image Viewer', 'Skills', 'Terminal'].every(a => forList.includes(a)) && forList[0] === 'Everywhere', JSON.stringify(forList));
    await help.locator('#keyboard-help-for').selectOption('Terminal');
    ok('choosing the Terminal lists its keys', (await help.locator('tbody').textContent()).includes('Delete the Word Before the Cursor'));
    await help.getByRole('button', { name: 'Close' }).click();
    await p.keyboard.press('Control+Shift+I');
    await p.waitForFunction(() => pudlWindows.state().top === 'needs-you');
    ok('Ctrl+Shift+I brings Notifications forward', true);
    await p.keyboard.press('Control+PageDown');
    ok('Ctrl+PageDown goes to the next window', await p.evaluate(() => pudlWindows.state().top !== 'needs-you'));
    // Command search says why a command cannot run now: with Notifications
    // in front, no Editor is, so Discard Draft cannot run.
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await p.waitForFunction(() => pudlWindows.state().top === 'needs-you');
    await p.keyboard.press('Control+Shift+P');
    await p.locator('#go-to-filter').fill('>discard draft');
    const discardRow = p.locator('#go-to-list tr', { hasText: 'Discard Draft' });
    await discardRow.waitFor();
    ok('a disabled command says why in command search', (await discardRow.textContent()).includes('No Editor window is in front.'));
    await p.keyboard.press('Escape');
    await p.evaluate(key => pudlWindows.raise(key), key);
    await p.keyboard.press('Control+Shift+P');
    await p.locator('#go-to-filter').fill('>attach file');
    const attachRow = p.locator('#go-to-list tr', { hasText: 'Attach File' });
    await attachRow.waitFor();
    ok('command search lists the document\'s commands under Document', (await attachRow.textContent()).includes('Document'));
    await p.keyboard.press('Escape');
    await t.shot('keyboard');

    // The menu bar as PUDL draws it: every key once, a letter for each
    // title and row, the theme as one of three, a destructive row marked and
    // a disabled row saying why.
    await p.evaluate(() => { const d = document.getElementById('go-to'); if (d.open) d.close(); });
    await p.evaluate(key => pudlWindows.raise(key), key);
    ok('the menu bar lists its keys, Save among them', keys.some(k => k.key === 'Ctrl+S' && /Save/.test(k.label)) && keys.some(k => k.key === 'F1'));
    ok('no letter of Dinah\'s menus is refused', menuWarnings.length === 0, menuWarnings.join('\n'));
    const panel = p.locator('#menubar-panel');
    // The bar redraws when the front window's commands change, so the title is found and pressed in one step.
    const openTitle = async name => {
      const found = await p.evaluate(name => { const el = [...document.querySelectorAll('.menubar-title')].find(e => e.textContent.trim() === name && e.offsetParent); if (el) el.click(); return !!el; }, name);
      if (!found) throw new Error('no title ' + name + ' among: ' + (await p.locator('.menubar-title:visible').allTextContents()).join(' | '));
      await panel.waitFor({ state: 'visible' });
    };
    await openTitle('Document');
    ok('Discard Draft… is drawn as a destructive row', await panel.locator('[role^="menuitem"]', { hasText: 'Discard Draft' }).evaluate(el => el.matches('.danger, [data-danger], .menu-danger') || el.className.includes('danger')));
    ok('Rename… is in the Document menu, with no key, since F2 is the window\'s own', await panel.locator('[role^="menuitem"]', { hasText: 'Rename' }).count() === 1 && !(await panel.textContent()).includes('F2'));
    await p.keyboard.press('Escape');
    await openTitle('View');
    await panel.locator('[role^="menuitem"]', { hasText: /^Color Mode$/ }).click();
    await p.waitForTimeout(100);
    ok('the theme rows are one of three', await p.locator('[role="menuitemradio"]', { hasText: /^Light/ }).count() === 1 && await p.locator('[role="menuitemradio"]', { hasText: /^Follow the System/ }).count() === 1);
    await p.keyboard.press('Escape'); await p.keyboard.press('Escape');
    // A title's letter opens it from the bar, and a row's letter runs it.
    await p.locator('.menubar-title:visible').first().focus();
    await p.keyboard.press('h');
    await panel.waitFor({ state: 'visible' });
    await p.keyboard.press('k');
    await p.locator('dialog.keyboard-help[open]').waitFor();
    ok('H opens Help from the bar, and K runs Keyboard Shortcuts', true);
    ok('Help says how to reach the menu bar', (await p.locator('dialog.keyboard-help[open]').textContent()).includes('Alt, pressed and released alone, or F10 moves to the menu bar'));
    await p.locator('dialog.keyboard-help[open]').getByRole('button', { name: 'Close' }).click();

    // A desktop host (Paul, 15 October 2026): Alt alone and F10 move to the
    // bar, Alt with a title's letter opens it from the text, and Escape goes
    // back to the text.
    const inBar = () => p.evaluate(() => !!document.activeElement.closest('.menubar') && document.activeElement.classList.contains('menubar-title'));
    await ed.locator('.cm-content').click();
    await p.keyboard.press('Alt');
    ok('Alt pressed and released alone moves to the menu bar', await inBar());
    await p.keyboard.press('Escape');
    ok('Escape from the bar goes back to the text', await p.evaluate(() => !!document.activeElement.closest('.cm-content')));
    await p.keyboard.press('F10');
    ok('F10 moves to the menu bar', await inBar());
    await p.keyboard.press('Escape');
    await ed.locator('.cm-content').click();
    await p.keyboard.press('Alt+d');
    await panel.waitFor({ state: 'visible' });
    ok('Alt+D opens Documents from the text', (await panel.textContent()).includes('New Document'));
    await p.keyboard.press('Escape'); await p.keyboard.press('Escape');
    await ed.locator('.cm-content').click();
    await p.keyboard.press('Alt+m');
    await panel.waitFor({ state: 'visible' });
    ok('Alt+M opens the document\'s own Document menu', (await panel.textContent()).includes('Discard Draft'));
    await p.keyboard.press('Escape'); await p.keyboard.press('Escape');
    ok('no access key is left in the bar', await p.evaluate(() => !document.querySelector('.menubar [accesskey]')));
    // The Dinah menu has no letter yet: PUDL draws the first title from the
    // host's own markup (docs/proposals/pudl-brand-title-letter.md).
  } finally { await t.close(); }
})();
