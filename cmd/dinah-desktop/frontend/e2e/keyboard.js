// The keyboard: the Editor's
// keys in the text and in its window, the global keys, and Help › Keyboard
// Shortcuts, which lists them all from the same declarations.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 2560, height: 1200 },
    state: { workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'], windowModes: { editor: 'maximized' }, windowModesShipped: { editor: 'maximized' }, titles: {}, templates: [],
      workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [{ id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] },
    app: () => {
      let n = 0;
      const rev = () => 'r' + (++n);
      const drafts = {};
      const get = id => { const d = drafts[id]; if (!d) throw new Error('the draft no longer exists'); return d; };
      const spans = d => { const out = {}; (d.suggestions || []).forEach(s => { const at = d.text.indexOf(s.before); if (at >= 0) out[s.id] = [at, at + s.before.length]; }); return out; };
      const view = d => Object.assign({}, d, { spans: spans(d), suggestions: d.suggestions || [], changed: d.text !== d.baseline.text, writeBack: 'save', actions: ['update', 'discard', 'save-as', 'save'] });
      return {
        OpenDraft: async ref => {
          let d = Object.values(drafts).find(x => x.origin.path === ref.path);
          if (!d) {
            const text = '# Notes\n\nFirst line.\n\nSecond line.\n';
            const id = 'k' + Object.keys(drafts).length.toString().padStart(31, '0');
            d = drafts[id] = { id, origin: { kind: 'local-folder', workspaceId: 'w1', sourceId: 'local', path: ref.path }, baseline: { text, revision: 'b1', title: ref.path }, title: ref.path, text, revision: rev(),
              suggestions: [{ id: 's1', group: 'g1', author: 'Review', reason: 'clearer', before: 'Second line.', after: 'Second line, better.', suffix: '' },
                { id: 's2', group: 'g2', author: 'Review', reason: 'fuller', before: 'Notes', after: 'Meeting Notes', suffix: '' }] };
          }
          return view(d);
        },
        GetDraft: async id => view(get(id)), ListDrafts: async () => Object.values(drafts).map(view),
        UpdateDraft: async (id, expected, text) => { const d = get(id); if (d.revision !== expected) throw new Error('stale'); d.text = text; d.revision = rev(); return view(d); },
        AcceptSuggestion: async (id, expected, sid) => { window.__calls.push(['accept', sid]); const d = get(id); const s = d.suggestions.find(x => x.id === sid); d.text = d.text.replace(s.before, s.after); d.suggestions = d.suggestions.filter(x => x.id !== sid); d.revision = rev(); return view(d); },
        RejectSuggestion: async (id, sid) => { window.__calls.push(['reject', sid]); const d = get(id); d.suggestions = d.suggestions.filter(x => x.id !== sid); return view(d); },
        // Page view, as the whole text in one block (page-view.js covers it).
        PageDocumentOf: async (id, text) => ({ type: 'doc', attrs: { front: '', tail: '' }, content: [{ type: 'source_block', attrs: { uid: 1, gap: '', src: text, kind: 'text', html: '<p>Text</p>' } }] }),
        PageDocumentMarkdown: async doc => doc.content.map(b => (b.attrs.gap || '') + b.attrs.src).join(''),
        PreviewDraft: async id => ({ revision: get(id).revision, problems: [{ line: 3, severity: 'warning', message: 'Check this line.' }], blocks: get(id).text.split('\n\n').map((part, i) => ({ line: i * 2 + 1, html: '<p>' + part.replace(/</g, '&lt;') + '</p>' })) }),
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
    // so the listener is in place before the page is opened.
    const menuWarnings = [];
    p.on('console', m => { if (m.type() === 'warning' && m.text().startsWith('pudl-menubar')) menuWarnings.push(m.text()); });
    await p.goto(PAGE);
    await t.waitReady();
    const key = await p.evaluate(() => dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'notes.md' }));
    const ed = p.locator('[data-win="' + key + '"]');
    await ed.locator('.cm-content').waitFor();
    await ed.locator('[data-ed-page] .doc-block').first().waitFor();
    const text = () => ed.locator('.cm-content').evaluate(el => [...el.querySelectorAll('.cm-line')].map(l => l.textContent).join('\n'));
    const line = async n => { await ed.locator('.cm-line').nth(n).click(); await p.keyboard.press('Home'); await p.keyboard.press('Shift+End'); };

    // Formatting in the text.
    await line(2);
    await p.keyboard.press('Control+b');
    ok('Ctrl+B makes the selection bold', (await text()).includes('**First line.**'));
    await p.keyboard.press('Control+b');
    ok('Ctrl+B again takes the bold away', (await text()).split('\n')[2] === 'First line.');
    await p.keyboard.press('Control+i');
    ok('Ctrl+I makes it italic', (await text()).split('\n')[2] === '*First line.*');
    await p.keyboard.press('Control+i');
    ok('Ctrl+I again takes the italics away', (await text()).split('\n')[2] === 'First line.');
    await p.keyboard.press('Control+2');
    ok('Ctrl+2 makes the line a second-level heading', (await text()).split('\n')[2] === '## First line.');
    await p.keyboard.press('Control+0');
    ok('Ctrl+0 makes it a paragraph again', (await text()).split('\n')[2] === 'First line.');
    await ed.locator('.cm-line').nth(2).click();
    await p.keyboard.press('Home');
    await p.keyboard.press('Shift+ArrowDown');
    await p.keyboard.press('Control+Shift+l');
    let lines = (await text()).split('\n');
    ok('a selection ending at the start of a line leaves that line out', lines[2] === '- First line.' && lines[3] === '');
    await p.keyboard.press('Control+z');
    await ed.locator('.cm-line').nth(2).click();
    await p.keyboard.press('Home');
    await p.keyboard.press('Shift+ArrowDown');
    await p.keyboard.press('Shift+ArrowDown');
    await p.keyboard.press('Shift+End');
    await p.keyboard.press('Control+Shift+l');
    lines = (await text()).split('\n');
    ok('Ctrl+Shift+L makes a bulleted list, leaving the blank line', lines[2] === '- First line.' && lines[3] === '' && lines[4].startsWith('- Second line.'));
    await p.keyboard.press('Control+Shift+o');
    lines = (await text()).split('\n');
    ok('Ctrl+Shift+O makes it numbered', lines[2] === '1. First line.' && lines[4].startsWith('2. Second line.'));
    await p.keyboard.press('Control+Shift+o');
    ok('Ctrl+Shift+O again takes the numbers away', (await text()).split('\n')[2] === 'First line.');

    // A link, with its own small dialog.
    await line(2);
    await p.keyboard.press('Control+k');
    const linkDialog = p.locator('dialog.editor-dialog[open]');
    await linkDialog.getByLabel('Address').fill('https://example.test/a');
    await p.keyboard.press('Control+b');
    await p.keyboard.press('Control+k');
    ok('no Editor key runs while the Link dialog is open', (await p.locator('dialog.editor-dialog[open]').count()) === 1 && (await text()).split('\n')[2] === 'First line.');
    await linkDialog.getByRole('button', { name: 'Cancel' }).focus();
    await p.keyboard.press('Enter');
    ok('Enter on Cancel adds no link', (await text()).split('\n')[2] === 'First line.' && (await linkDialog.count()) === 0);
    await line(2);
    await p.keyboard.press('Control+k');
    await linkDialog.getByLabel('Address').fill('https://example.test/a');
    await p.keyboard.press('Enter');
    ok('Ctrl+K links the selection', (await text()).split('\n')[2] === '[First line.](https://example.test/a)');
    ok('closing the Link dialog puts focus back in the text', await p.evaluate(() => !!document.activeElement.closest('.cm-content')));
    await p.keyboard.press('Control+z');

    // Suggestions and problems. Typing above a suggestion moves it, and F7
    // still finds it where it is now.
    const selection = () => ed.locator('.cm-content').evaluate(el => { const v = CM.EditorView.findFromDOM(el), s = v.state.selection.main; return v.state.sliceDoc(s.from, s.to); });
    await ed.locator('.cm-line').nth(1).click();
    await p.keyboard.type('Typed above.');
    await p.keyboard.press('F7');
    ok('F7 selects the next suggestion where it is now', await selection() === 'Second line.');
    await p.keyboard.press('Shift+F7');
    const back = await selection();
    ok('Shift+F7 from a selected suggestion goes to the one before', back === 'Notes');
    if (back !== 'Notes') console.log('selected: ' + JSON.stringify(back) + ' in ' + JSON.stringify(await text()));
    await p.keyboard.press('Shift+F7');
    ok('Shift+F7 goes round from the first to the last', await selection() === 'Second line.');
    await p.keyboard.press('Alt+Enter');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'accept' && c[1] === 's1'));
    ok('Alt+Enter accepts the suggestion at the cursor', true);
    await ed.locator('.cm-line').nth(0).click();
    await p.keyboard.press('F8');
    ok('F8 goes to the next problem', (await ed.locator('[data-ed-status]').textContent()).startsWith('Line 3'));

    // Search and Go to Line.
    await p.keyboard.press('Control+h');
    await ed.locator('[data-ed-find]').waitFor();
    ok('Ctrl+H opens find and replace', await ed.locator('[data-ed-find-replace-row]').isVisible());
    const cursorLine = () => ed.locator('.cm-content').evaluate(el => { const v = CM.EditorView.findFromDOM(el); return v.state.doc.lineAt(v.state.selection.main.head).number; });
    await ed.locator('.cm-content').evaluate(el => { const v = CM.EditorView.findFromDOM(el); v.dispatch({ selection: { anchor: 0 } }); });
    await ed.locator('[data-ed-find-input]').focus();
    await p.keyboard.press('F8');
    ok('F8 runs from the search field', await cursorLine() === 3);
    await ed.locator('[data-ed-find-input]').focus();
    await p.keyboard.press('Escape');
    ok('Esc in the find field closes the bar', await ed.locator('[data-ed-find]').isHidden());
    await ed.locator('.cm-content').click();
    await p.keyboard.press('Control+g');
    await ed.locator('.cm-panels input').first().waitFor();
    ok('Ctrl+G asks for a line', true);
    await p.keyboard.press('Escape');

    // The window's keys.
    await ed.locator('.cm-content').click();
    await p.keyboard.press('Control+e');
    ok('Ctrl+E switches the view, in the selector\'s order', await ed.locator('[data-ed-panes]').getAttribute('data-view') === 'preview');
    await p.keyboard.press('Control+e');
    await p.keyboard.press('Control+e');
    await p.keyboard.press('Control+e');
    ok('Ctrl+E goes round the four views', await ed.locator('[data-ed-panes]').getAttribute('data-view') === 'split');
    const wrapped = await ed.locator('.cm-content').evaluate(el => el.classList.contains('cm-lineWrapping'));
    await ed.locator('.cm-content').click();
    await p.keyboard.press('Alt+z');
    ok('Alt+Z turns line wrapping over', await ed.locator('.cm-content').evaluate(el => el.classList.contains('cm-lineWrapping')) !== wrapped);
    await ed.locator('.cm-content').click();
    await p.keyboard.press('F6');
    ok('F6 goes to the next pane', await p.evaluate(() => !!document.activeElement.closest('[data-ed-scroll]')));
    await p.keyboard.press('Control+Shift+m');
    ok('Ctrl+Shift+M shows the problems', await ed.locator('[data-ed-problems]').isVisible());

    // Global keys and the reference.
    await ed.locator('.cm-content').click();
    await p.keyboard.press('F1');
    const help = p.locator('dialog.keyboard-help[open]');
    await help.waitFor();
    const helpText = await help.locator('tbody').textContent();
    ok('F1 lists the keys that work everywhere first, and no applet\'s', helpText.includes('Ctrl+Shift+I') && helpText.includes('Ctrl+PageDown') && helpText.includes('Ctrl+Tab') && helpText.includes('Shift+F10') && !helpText.includes('Bold'));
    const forList = await help.locator('#keyboard-help-for option').allTextContents();
    ok('Keys For lists the applets that declared keys', ['Chat', 'Editor', 'Image Viewer', 'Skills', 'Terminal'].every(a => forList.includes(a)) && forList[0] === 'Everywhere', JSON.stringify(forList));
    await help.locator('#keyboard-help-for').selectOption('Editor');
    const editorKeys = await help.locator('tbody').textContent();
    ok('choosing the Editor lists its commands and the keys of its text and its page', editorKeys.includes('Bold') && editorKeys.includes('In the Text') && editorKeys.includes('Visual View') && !editorKeys.includes('Ctrl+PageDown'));
    await help.locator('#keyboard-help-for').selectOption('Terminal');
    ok('choosing the Terminal lists its keys', (await help.locator('tbody').textContent()).includes('Delete the Word Before the Cursor'));
    await help.getByRole('button', { name: 'Close' }).click();
    await p.keyboard.press('Control+Shift+I');
    await p.waitForFunction(() => pudlWindows.state().top === 'needs-you');
    ok('Ctrl+Shift+I brings Notifications forward', true);
    await p.keyboard.press('Control+PageDown');
    ok('Ctrl+PageDown goes to the next window', await p.evaluate(() => pudlWindows.state().top !== 'needs-you'));
    // Command search says why a command cannot run now: with Notifications
    // in front, no Editor is, so Close Document cannot run.
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await p.waitForFunction(() => pudlWindows.state().top === 'needs-you');
    await p.keyboard.press('Control+Shift+P');
    await p.locator('#go-to-filter').fill('>close document');
    const closeRow = p.locator('#go-to-list tr', { hasText: 'Close Document' });
    await closeRow.waitFor();
    ok('a disabled command says why in command search', (await closeRow.textContent()).includes('No Editor window is in front.'));
    await p.keyboard.press('Escape');
    await p.evaluate(key => pudlWindows.raise(key), key);
    await p.keyboard.press('Control+Shift+P');
    await p.locator('#go-to-filter').fill('>numbered list');
    const listRow = p.locator('#go-to-list tr', { hasText: 'Numbered List' });
    await listRow.waitFor();
    ok('command search lists the Editor\'s own commands, under their menu', (await listRow.textContent()).includes('Format'));
    await p.keyboard.press('Escape');
    await t.shot('keyboard');

    // The menu bar as PUDL 0.61 draws it: every key once, a letter for each
    // title and row, the theme as one of three, a destructive row marked and
    // a disabled row saying why.
    await p.evaluate(() => { const d = document.getElementById('go-to'); if (d.open) d.close(); });
    await p.evaluate(key => pudlWindows.raise(key), key);
    const keys = await p.evaluate(() => pudlMenubar.shortcuts());
    ok('the menu bar lists its keys, Save among them', keys.some(k => k.shown === 'Ctrl+S' && /Save/.test(k.label)) && keys.some(k => k.shown === 'F1'));
    ok('no key is bound to two rows and no letter is refused', menuWarnings.length === 0);
    if (menuWarnings.length) console.log(menuWarnings.join('\n'));
    const panel = p.locator('#menubar-panel');
    const openTitle = async name => {
      const title = p.locator('.menubar-title:visible', { hasText: new RegExp('^' + name + '$') });
      if (!await title.count()) throw new Error('no title ' + name + ' among: ' + (await p.locator('.menubar-title:visible').allTextContents()).join(' | '));
      await title.first().click();
      await panel.waitFor({ state: 'visible' });
    };
    // A document with nothing changed in it cannot be saved.
    const fresh = await p.evaluate(() => dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'unchanged.md' }));
    await p.locator('[data-win="' + fresh + '"] .cm-content').waitFor();
    await openTitle('Document');
    const saveRowMenu = panel.locator('[role^="menuitem"]', { hasText: 'Save' }).first();
    ok('a disabled row says why it cannot run', (await saveRowMenu.textContent()).includes('There are no changes to save.'));
    ok('Discard Draft… is drawn as a destructive row', await panel.locator('[role^="menuitem"]', { hasText: 'Discard Draft' }).evaluate(el => el.matches('.danger, [data-danger], .menu-danger') || el.className.includes('danger')));
    await p.keyboard.press('Escape');
    await p.evaluate(fresh => pudlWindows.close(fresh), fresh);
    await p.evaluate(key => pudlWindows.raise(key), key);
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
    await p.keyboard.press('Alt+o');
    await panel.waitFor({ state: 'visible' });
    ok('Alt+O opens the Editor\'s Document menu', (await panel.textContent()).includes('Discard Draft'));
    await p.keyboard.press('Escape'); await p.keyboard.press('Escape');
    ok('the old Alt+M access key is retired', await p.evaluate(() => !document.querySelector('.menubar [accesskey]')));
    await ed.locator('.cm-content').click();
    await p.keyboard.press('Alt+n');
    await panel.waitFor({ state: 'visible' });
    ok('Alt+N opens the Dinah menu from the text', (await panel.textContent()).includes('Settings'));
    // Its letter is not underlined yet: PUDL draws the first title from the
    // host's own markup (docs/proposals/pudl-brand-title-letter.md).
    await p.keyboard.press('Escape'); await p.keyboard.press('Escape');
  } finally { await t.close(); }
})();
