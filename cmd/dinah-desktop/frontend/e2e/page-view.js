// Page view: the whole document is one editor on its page. A document opens
// in Split, and Visual shows the page alone; typing, Enter and the arrow keys
// work across blocks; an edit
// reaches the text, changing only its block; a code block's Markdown is
// edited in place; Undo works on the page; Save takes the last typing;
// a change made elsewhere is read into the page; Ctrl+E goes on to
// Preview. The fixture reads and writes the document as pageview (Go) does,
// simplified; Go's own tests cover pageview.
const { launch, OUT } = require('./lib/page');
const path = require('path');
(async () => {
  const t = await launch({
    viewport: { width: 2560, height: 1200 },
    state: { workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'], defaultProvider: 'devin', documentViews: { 'text/markdown': 'split' }, windowModes: { editor: 'maximized' }, windowModesShipped: { editor: 'maximized' }, titles: {}, templates: [],
      workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [{ id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] },
    app: () => {
      window.__calls = [];
      let n = 0;
      const rev = () => 'r' + (++n);
      const drafts = {};
      const TEXT = ['---', 'title: Scanner', '---', '', '# Scanner', '', 'The lexer reads the **token** number.', '', '- one', '- two', '',
        '- [ ] Take a nap', '',
        '```', 'code here', '```', '', 'Last paragraph.', ''].join('\n');
      const esc = s => s.replace(/&/g, '&amp;').replace(/</g, '&lt;');
      // Inline text with **bold** and line breaks, as the editor's nodes and back.
      const inl = s => {
        const out = [];
        s.split('\n').forEach((line, li) => {
          if (li) out.push({ type: 'soft_break' });
          line.split('**').forEach((t, k) => { if (t) out.push(k % 2 ? { type: 'text', text: t, marks: [{ type: 'strong' }] } : { type: 'text', text: t }); });
        });
        return out;
      };
      const outl = nodes => (nodes || []).map(x => x.type === 'soft_break' ? '\n' : (x.marks || []).some(m => m.type === 'strong') ? '**' + x.text + '**' : (x.marks || []).some(m => m.type === 'em') ? '*' + x.text + '*' : x.text).join('');
      let uid = 0;
      // Blocks between lines a and b, each with the text before it.
      const read = (lines, a, b) => {
        const out = [];
        let i = a;
        while (i < b) {
          if (!lines[i].trim()) { i++; continue; }
          let j = i, node;
          if (lines[i].startsWith('```')) {
            j = i + 1; while (!lines[j].startsWith('```')) j++; j++;
            node = { type: 'source_block', attrs: { kind: 'code', html: '<pre><code>' + esc(lines.slice(i + 1, j - 1).join('\n')) + '</code></pre>' } };
          } else {
            while (j < b && lines[j].trim()) j++;
            const block = lines.slice(i, j), h = /^(#+) (.*)$/.exec(block[0]);
            if (h) node = { type: 'heading', attrs: { level: h[1].length }, content: inl(h[2]) };
            else if (block[0].startsWith('- ')) node = { type: 'bullet_list', attrs: { tight: true }, content: block.map(l => {
              const task = /^- \[([ x])\] /.exec(l);
              return { type: 'list_item', attrs: { checked: task ? task[1] === 'x' : null }, content: [{ type: 'paragraph', content: inl(l.slice(task ? 6 : 2)) }] };
            }) };
            else node = { type: 'paragraph', content: inl(block.join('\n')) };
          }
          // The fixture's blocks are one blank line apart; docOf sets the gaps.
          node.attrs = Object.assign(node.attrs || {}, { uid: ++uid, src: lines.slice(i, j).join('\n') });
          out.push(node);
          i = j;
        }
        return out;
      };
      const docOf = text => {
        const lines = text.split('\n'), front = lines.slice(0, 3).map(l => l + '\n').join('');
        const content = read(lines, 3, lines.length);
        content.forEach((c, k) => { c.attrs.gap = k === 0 ? '\n' : '\n\n'; });
        return { type: 'doc', attrs: { front, tail: '\n' }, content };
      };
      const write = b => b.type === 'heading' ? '#'.repeat(b.attrs.level) + ' ' + outl(b.content)
        : b.type === 'bullet_list' ? b.content.map(li => '- ' + (li.attrs && typeof li.attrs.checked === 'boolean' ? (li.attrs.checked ? '[x] ' : '[ ] ') : '') + outl(li.content[0].content)).join('\n')
          : b.type === 'source_block' ? b.attrs.src
              : b.type === 'table' ? b.content.map((row, i) => row.content.map(c => '| ' + (outl(c.content) ? outl(c.content) + ' ' : '')).join('') + '|' +
                (i === 0 ? '\n' + row.content.map(() => '| --- ').join('') + '|' : '')).join('\n')
              : outl(b.content);
      const seq = (nodes, first) => { let s = '', started = false; nodes.forEach(nd => { const t = write(nd); if (!t) return; s += (nd.attrs && typeof nd.attrs.gap === 'string' ? nd.attrs.gap : started ? '\n\n' : first) + t; started = true; }); return s; };
      const markdownOf = doc => doc.attrs.front + seq(doc.content, '\n') + doc.attrs.tail;
      // Suggestions replace the first place their text is found, as the
      // draft store places them.
      const placed = d => (d.sugg || []).filter(s => d.text.indexOf(s.base) >= 0);
      const view = d => Object.assign({}, d, {
        spans: Object.fromEntries(placed(d).map(s => [s.id, [d.text.indexOf(s.base), d.text.indexOf(s.base) + s.base.length]])),
        suggestions: placed(d).map(s => ({ id: s.id, author: s.author, reason: s.reason, after: s.after, group: s.id, edited: !!s.edited })),
        changed: d.text !== d.baseline.text, writeBack: 'save', actions: ['update', 'discard', 'save-as', 'save'] });
      window.__suggest = (base, after, reason) => {
        const d = Object.values(drafts)[0];
        (d.sugg = d.sugg || []).push({ id: 's' + (d.sugg.length + 1), base, after, author: 'Fixture', reason });
        d.revision = rev();
        window.__emit('draft:changed', { id: d.id, revision: d.revision, kind: 'suggested' });
      };
      const get = id => drafts[id];
      window.__text = () => Object.values(drafts)[0].text;
      window.__change = text => { const d = Object.values(drafts)[0]; d.text = text; d.revision = rev(); window.__emit('draft:changed', { id: d.id, revision: d.revision, kind: 'updated' }); };
      window.__answers = {};
      return {
        AnswerPage: async (id, result, err) => { window.__answers[id] = { result: result ? JSON.parse(result) : null, err }; },
        OpenDraft: async ref => {
          let d = Object.values(drafts).find(x => x.origin.path === ref.path);
          if (!d) {
            const id = 'd' + Object.keys(drafts).length.toString().padStart(31, '0');
            d = drafts[id] = { id, origin: { kind: 'local-folder', workspaceId: 'w1', sourceId: 'local', path: ref.path }, baseline: { text: TEXT, revision: 'b1', title: ref.path },
              title: ref.path, text: TEXT, revision: rev() };
          }
          return view(d);
        },
        GetDraft: async id => view(get(id)),
        UpdateDraft: async (id, expected, text) => {
          const d = get(id);
          window.__calls.push(['update', text]);
          if (d.revision !== expected) throw new Error('stale');
          d.text = text; d.revision = rev();
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'updated' });
          return view(d);
        },
        SaveDraft: async (id, expected) => {
          const d = get(id);
          window.__calls.push(['save', d.text]);
          if (d.revision !== expected) throw new Error('stale');
          d.baseline = { text: d.text, revision: 'b2', title: d.title };
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'saved' });
          return Object.assign(view(d), { notes: [] });
        },
        AcceptSuggestion: async (id, expected, sid) => {
          const d = get(id), s = d.sugg.find(x => x.id === sid);
          window.__calls.push(['accept', sid]);
          d.text = d.text.replace(s.base, s.after); d.sugg = d.sugg.filter(x => x !== s); d.revision = rev();
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'updated' });
          return view(d);
        },
        ReviseSuggestion: async (id, sid, after) => {
          const d = get(id), s = d.sugg.find(x => x.id === sid);
          window.__calls.push(['revise', sid, after]);
          if (s.after !== after) { s.after = after; s.edited = true; }
          return view(d);
        },
        RejectSuggestion: async (id, sid) => {
          const d = get(id);
          window.__calls.push(['reject', sid]);
          d.sugg = d.sugg.filter(x => x.id !== sid); d.revision = rev();
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'updated' });
          return view(d);
        },
        DraftReadiness: async id => ({ ready: true, revision: get(id).revision, items: [], summary: '' }),
        PreviewDraft: async id => { const d = get(id); return { revision: d.revision, problems: [], blocks: [{ line: 1, html: '<p>' + esc(d.text).replace(/\n/g, '<br>') + '</p>' }] }; },
        PageDocumentOf: async (id, text) => { window.__calls.push(['doc']); return docOf(text); },
        PageDocumentMarkdown: async doc => { window.__lastDoc = doc; return markdownOf(doc); },
        PageBlocksOf: async (id, md) => {
          window.__calls.push(['blocks', md]);
          return [{ type: 'source_block', attrs: { kind: 'code', src: md, html: '<pre><code>' + esc(md.split('\n').slice(1, -1).join('\n')) + '</code></pre>' } }];
        },
        PageSourceOf: async (id, src) => {
          window.__calls.push(['source', src]);
          return { html: '<pre><code>' + esc(src.split('\n').slice(1, -1).join('\n')) + '</code></pre>', diagrams: [] };
        },
        ListDrafts: async () => Object.values(drafts).map(view)
      };
    },
    ready: () => window.dinahEditor && window.dinahWorkspaceBrowser.records().length === 1,
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => window.dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'note.md' }));
    const editor = p.locator('.win[data-win-kind="editor"]').first();
    const page = editor.locator('[data-ed-page]');
    const pm = page.locator('.ProseMirror');
    await pm.waitFor();
    ok('a document opens in Split', await editor.locator('[data-ed-panes]').getAttribute('data-view') === 'split');
    await editor.evaluate(win => pudlCommands.run('view-page', undefined, win.querySelector('.editor-applet')));
    await editor.locator('[data-command="view-page"][aria-pressed="true"]').waitFor();
    const text = () => p.evaluate(() => window.__text());
    const waitText = s => p.waitForFunction(s => window.__text().includes(s), s);
    ok('Visual shows the document in Page view, as one editor', await editor.locator('[data-ed-panes]').getAttribute('data-view') === 'page' &&
      await page.locator('.ProseMirror').count() === 1 && await pm.getAttribute('contenteditable') === 'true');
    ok('the page shows the document and not its front matter', (await pm.locator('h1').textContent()) === 'Scanner' && !(await pm.textContent()).includes('title: Scanner'));

    // Typing at the end of a paragraph reaches the text; nothing else changes.
    await pm.locator('p').filter({ hasText: 'The lexer reads' }).click();
    await p.keyboard.press('End');
    await p.waitForTimeout(80);
    await p.keyboard.type(' It is checked.');
    await waitText('number. It is checked.');
    const after = await text();
    ok('typing reaches the text, and only its block changes', after === (await p.evaluate(() => window.go.main.App.OpenDraft({ path: 'note.md' }).then(d => d.baseline.text))).replace('number.', 'number. It is checked.'), after);

    // The arrow keys move into the next block, a list, and typing goes there.
    await p.keyboard.press('ArrowDown');
    await p.keyboard.press('End');
    await p.waitForTimeout(80);
    await p.keyboard.type(' more');
    await waitText('- one more\n- two');
    ok('the arrow keys move from a paragraph into a list', true);

    // A task item's box ticks it, and the text says so; Undo unticks it.
    const taskBox = pm.locator('li.pm-task .pm-check input');
    ok('a task item shows its box on the page, not as text', await taskBox.count() === 1 && !(await pm.locator('li.pm-task').textContent()).includes('[ ]'));
    await taskBox.click();
    await waitText('- [x] Take a nap');
    ok('clicking the box ticks the item in the text', await taskBox.isChecked());
    await p.keyboard.press('Control+z');
    await waitText('- [ ] Take a nap');
    ok('Undo unticks it', !(await taskBox.isChecked()));
    await taskBox.focus(); await p.keyboard.press('Space');
    await waitText('- [x] Take a nap');
    ok('a focused checklist box toggles with Space', await taskBox.isChecked());
    await p.keyboard.press('Control+z'); await waitText('- [ ] Take a nap');

    // Enter at the end of a paragraph starts a new paragraph.
    await pm.locator('p').filter({ hasText: 'Last paragraph.' }).click();
    await p.keyboard.press('End');
    await p.waitForTimeout(80);
    await p.keyboard.press('Enter');
    await p.keyboard.type('A new paragraph.');
    await waitText('Last paragraph.\n\nA new paragraph.\n');
    ok('Enter starts a new paragraph, written after a blank line', true);

    // Undo on the page takes the typing back, in the page and the text.
    await p.keyboard.press('Control+z');
    await p.waitForFunction(() => !window.__text().includes('A new paragraph.'));
    ok('Undo takes back typing on the page and in the text', !(await pm.textContent()).includes('A new paragraph.'));

    // Bold from the Editor's key acts on the page.
    await pm.locator('h1').click();
    await p.keyboard.press('End');
    await p.keyboard.down('Shift');
    for (let i = 0; i < 'Scanner'.length; i++) await p.keyboard.press('ArrowLeft');
    await p.keyboard.up('Shift');
    await p.waitForTimeout(100);
    await p.keyboard.press('Control+b');
    await waitText('# **Scanner**');
    ok('Ctrl+B bolds the selection', await pm.locator('h1 strong').count() === 1);

    // A code block's Markdown is edited in place.
    await pm.locator('.pm-source[data-kind="code"]').click();
    const box = pm.locator('textarea.page-source');
    await box.waitFor();
    ok('a click on a code block shows its Markdown', (await box.inputValue()) === '```\ncode here\n```');
    await box.press('Control+End');
    await box.press('ArrowUp');
    await box.press('End');
    await p.keyboard.type(' and more');
    await box.press('Escape');
    await waitText('```\ncode here and more\n```');
    await pm.locator('.pm-source[data-kind="code"] code').filter({ hasText: 'code here and more' }).waitFor();
    ok('leaving it puts the Markdown into the text and draws the block again', true);

    // An empty line says how to insert; / opens a menu, typing narrows it,
    // and Escape closes it and leaves what was typed.
    await pm.locator('p').filter({ hasText: 'Last paragraph.' }).click();
    await p.keyboard.press('End');
    await p.waitForTimeout(80);
    await p.keyboard.press('Enter');
    await pm.locator('.pm-hint').waitFor();
    ok('an empty line says how to insert a block', (await pm.locator('.pm-hint').getAttribute('data-hint')).startsWith('Type / to insert'));
    await p.keyboard.type('/');
    const slash = p.locator('.page-slash');
    await slash.waitFor();
    ok('/ opens a menu of the blocks to insert', await slash.locator('.menu-action').count() === 3);
    await p.keyboard.type('tabl');
    await p.waitForFunction(() => document.querySelectorAll('.page-slash .menu-action').length === 1);
    ok('typing after the / narrows the menu', (await slash.locator('.menu-action').textContent()) === 'Table');
    await p.keyboard.press('Escape');
    await slash.waitFor({ state: 'detached' });
    ok('Escape closes the menu and leaves the /', (await pm.textContent()).includes('/tabl'));
    for (let i = 0; i < 5; i++) await p.keyboard.press('Backspace');

    // Insert › Table from the Edit menu, then typing in its cells.
    const command = id => p.evaluate(id => window.dinahCommands.searchable().find(c => c.id === id).run(), id);
    await command('editor-insert-table');
    await pm.locator('table').nth(0).waitFor();
    await p.keyboard.type('Name');
    await p.keyboard.press('Tab');
    await p.keyboard.type('Value');
    await waitText('| Name | Value | Heading 3 |\n| --- | --- | --- |\n| | | |\n| | | |');
    ok('Insert › Table puts a table in with its first heading selected to type over', true);
    // The right arrow at the end of a cell goes into the next cell, with no
    // gap cursor drawn between them as an extra cell.
    await pm.locator('table th').first().click();
    await p.keyboard.press('End');
    await p.waitForTimeout(80);
    await p.keyboard.press('ArrowRight');
    await p.waitForTimeout(150);
    ok('the right arrow at the end of a cell moves into the next cell', await pm.locator('.ProseMirror-gapcursor').count() === 0 &&
      await pm.locator('table tr').first().locator('> *').count() === 3 &&
      await p.evaluate(() => { const n = getSelection().anchorNode; const cell = n && (n.nodeType === 1 ? n : n.parentElement).closest('th, td'); return !!cell && cell.textContent === 'Value'; }));
    await command('editor-column-after');
    // The cursor is in the second column, so the new column comes after it.
    await waitText('| Name | Value | | Heading 3 |');
    await pm.locator('table td').first().click();
    await p.waitForTimeout(80);
    await command('editor-delete-row');
    await p.waitForFunction(() => window.__text().includes('| Name | Value | | Heading 3 |\n| --- | --- | --- | --- |\n| | | | |\n') && !window.__text().includes('| | | | |\n| | | | |'));
    ok('the Table row adds a column and deletes a row', true);

    // A Markdown table cannot hold merged cells or a heading column, so
    // the page does not offer them.
    ok('the Table commands offer no shape a Markdown table cannot hold', await p.evaluate(() => !window.dinahCommands.searchable().some(c => /merge|split-cell|heading-c|colour/.test(c.id))));
    await pm.locator('table td').nth(0).click();
    await p.waitForTimeout(80);
    const tableOff = await p.evaluate(() => { const c = window.dinahCommands.searchable().find(c => c.id === 'editor-delete-table'); return c.enabled; });
    await pm.locator('h1').click();
    await p.waitForTimeout(80);
    ok('the Table commands are off outside a table', tableOff && !(await p.evaluate(() => window.dinahCommands.searchable().find(c => c.id === 'editor-delete-table').enabled)));

    // A code block from the / menu opens its Markdown with the code selected.
    await pm.locator('p').filter({ hasText: 'Last paragraph.' }).click();
    await p.keyboard.press('End');
    await p.waitForTimeout(80);
    await p.keyboard.press('Enter');
    await p.keyboard.type('/code');
    await p.waitForFunction(() => document.querySelectorAll('.page-slash .menu-action').length === 1);
    await p.keyboard.press('Enter');
    const codeBox = pm.locator('textarea.page-source');
    await codeBox.waitFor();
    await p.keyboard.type('x = 1');
    await codeBox.press('Escape');
    await waitText('Last paragraph.\n\n```\nx = 1\n```');
    ok('a code block from the / menu opens with its code ready to type over', true);

    // The formatting toolbar: PUDL icon buttons that follow the cursor,
    // the Table menu always there with only Insert a Table enabled outside
    // a table.
    const toolbar = editor.locator('[data-ed-toolbar]');
    await toolbar.waitFor();
    ok('Page view shows PUDL\'s folding toolbar of the Editor\'s commands, in named groups', await toolbar.evaluate(el => el.matches('.toolbar[data-toolbar-fold]')) &&
      await toolbar.locator('.toolbar-group[role="group"][data-toolbar-group]').count() >= 5 && await toolbar.locator('button[data-command]').count() >= 8);
    // PUDL writes a tab index only when it changes (0.69.3), so the tab stop
    // may have no attribute: a button's own tab index is 0.
    ok('the toolbar is one tab stop', await toolbar.evaluate(el => [...el.querySelectorAll('button, a[href], input, select')].filter(c => !c.closest('.menu-panel') && c.tabIndex === 0).length) === 1);
    ok('the toolbar\'s glyphs are PUDL\'s', await toolbar.getByRole('button', { name: 'Bold', exact: true }).locator('.glyph').evaluate(el => el.style.getPropertyValue('--glyph') === 'var(--glyph-bold)'));
    await pm.locator('h1').click();
    await p.waitForTimeout(120);
    ok('the paragraph style button says where the cursor is', (await toolbar.locator('.editor-toolbar-style .toolbar-words').textContent()) === 'Heading 1');
    await pm.locator('p').filter({ hasText: 'The lexer reads' }).locator('strong').click();
    await p.waitForTimeout(120);
    ok('Bold is pressed in while the cursor is in bold text', await toolbar.getByRole('button', { name: 'Bold', exact: true }).getAttribute('aria-pressed') === 'true' &&
      await toolbar.getByRole('button', { name: 'Italic', exact: true }).getAttribute('aria-pressed') === 'false');
    const tableMenu = toolbar.getByRole('button', { name: 'Table', exact: true });
    async function tableRows() {
      await tableMenu.click();
      const panel = p.locator('.menu-panel:popover-open');
      await panel.waitFor();
      const rows = await panel.locator('.menu-action:has(.menu-words)').evaluateAll(list => list.map(b => [b.querySelector('.menu-words').textContent, b.getAttribute('aria-disabled') === 'true']));
      await panel.evaluate(el => el.hidePopover());
      await p.waitForFunction(() => !document.querySelector('.menu-panel:popover-open'));
      return rows;
    }
    let rows = await tableRows();
    ok('outside a table the Table menu is there with only Insert a Table enabled', await tableMenu.isVisible() && !(await tableMenu.isDisabled()) &&
      rows[0][0] === 'Insert a Table' && !rows[0][1] && rows.slice(1).every(r => r[1]));
    await pm.locator('table td').first().click();
    await p.waitForTimeout(120);
    rows = await tableRows();
    ok('in a table the Table menu enables the table commands', rows.find(r => r[0] === 'Add a Row Below') && !rows.find(r => r[0] === 'Add a Row Below')[1] && !rows[0][1]);
    await pm.locator('table td').first().click();
    await p.waitForTimeout(80);
    await toolbar.getByRole('button', { name: 'Italic', exact: true }).click();
    await p.keyboard.type('em');
    await waitText('*em*');
    ok('a toolbar button formats what is typed next', true);
    await t.shot('page-toolbar');

    // As the toolbar narrows its groups fold: to their glyphs, then each
    // into a menu with its name, then into More; Text, the most used, last.
    // A folded group's menu holds every command it had.
    const stages = () => toolbar.evaluate(el => Object.fromEntries([...el.querySelectorAll(':scope > .toolbar-group:not([hidden])')].map(g => [g.dataset.toolbarGroup, +g.dataset.toolbarStage])));
    // PUDL folds the toolbar in a frame after its width changes and then
    // sends pudl:toolbar-fit (0.69.1), so the stages are read after that
    // event rather than after a guessed pause. Should it never come, the
    // stages are read anyway and the checks say what they found.
    async function narrow(width) {
      await toolbar.evaluate((el, w) => new Promise(done => {
        const fitted = () => { el.removeEventListener('pudl:toolbar-fit', fitted); clearTimeout(timer); done(); };
        const timer = setTimeout(fitted, 30000);
        el.addEventListener('pudl:toolbar-fit', fitted);
        el.style.maxWidth = w ? w + 'px' : '';
      }), width);
      return stages();
    }
    const wide = await stages(), mid = await narrow(620), slim = await narrow(260);
    ok('a narrower toolbar folds its groups, the lowest priority first, keeping Text open longest', Object.values(mid).some(s => s >= 3) &&
      Object.values(mid).every(s => s >= mid.Text) && mid.Table >= mid.Style, JSON.stringify({ wide, mid, slim }));
    ok('at its narrowest the toolbar puts groups in its More menu, and never runs over its width', Object.values(slim).some(s => s === 5) && await toolbar.evaluate(el => el.scrollWidth <= el.clientWidth + 1) &&
      await toolbar.locator('.toolbar-more .menu-btn').isVisible(), JSON.stringify(slim));
    await toolbar.locator('.toolbar-more .menu-btn').click();
    const more = p.locator('.menu-panel:popover-open');
    await more.waitFor();
    const moreRows = await more.locator('.menu-action').allTextContents();
    ok('the More menu holds the folded groups\' commands', moreRows.some(t => t.includes('Code Block')) && moreRows.some(t => t.includes('Insert a Table')), moreRows.join(' | '));
    await more.evaluate(el => el.hidePopover());
    await narrow(0);
    ok('widened again, the groups come back', JSON.stringify(await stages()) === JSON.stringify(wide), JSON.stringify(await stages()));

    // An agent reaches the same commands by id, as data with their state
    // now, and runs one as pressing it would; a command that cannot run
    // says why.
    const listed = await editor.evaluate(win => pudlCommands.list(win.querySelector('.editor-applet')));
    const byId = Object.fromEntries(listed.map(c => [c.id, c]));
    ok('the Editor\'s commands are listed for an agent, with their keys, state and whether a toolbar shows them', byId.bold && byId.bold.shortcut === 'Ctrl+B' && byId.bold.shown === 'toolbar' &&
      byId['view-page'] && byId['view-page'].checked === true && byId['table-menu'] && byId['table-menu'].items.length > 5 && byId.save,
      JSON.stringify(Object.keys(byId)));
    await editor.evaluate(win => pudlCommands.run('view-split', undefined, win.querySelector('.editor-applet')));
    ok('running a view\'s command by its id switches to it, and its control says so', await editor.locator('[data-ed-panes]').getAttribute('data-view') === 'split' &&
      await editor.locator('[data-command="view-split"][aria-pressed="true"]').waitFor({ timeout: 3000 }).then(() => true, () => false));
    await editor.evaluate(win => pudlCommands.run('view-page', undefined, win.querySelector('.editor-applet')));
    await editor.locator('[data-command="view-page"][aria-pressed="true"]').waitFor();
    const refused = await editor.evaluate(win => pudlCommands.run('accept', undefined, win.querySelector('.editor-applet')).then(() => 'ran', e => String(e.message || e)));
    ok('a command that cannot run now is refused, saying so', refused !== 'ran', refused);

    // An agent asks through the Windows applet: the Go side sends
    // page:ask and the page answers. It sees only its workspace's windows,
    // runs a command in the window it gives, and is refused a command that
    // would destroy something.
    const ask = (op, args) => p.evaluate(async ([op, args]) => {
      const id = 'q' + Math.random();
      window.__emit('page:ask', { id, op, args });
      for (let i = 0; i < 100 && !window.__answers[id]; i++) await new Promise(r => setTimeout(r, 20));
      return window.__answers[id];
    }, [op, args]);
    const mine = { workspace: 'C:\\Fixture\\', workspaceId: 'w1' };
    const wins = await ask('windows', mine);
    const ed = wins.result && wins.result.windows.find(w => w.kind === 'editor');
    ok('the workspace\'s windows are listed for an agent, with the Editor in front', ed && ed.front === true && ed.title.length > 0, JSON.stringify(wins));
    const other = await ask('windows', { workspace: 'C:/Elsewhere', workspaceId: 'w2' });
    ok('another workspace\'s agent sees none of them', other.result && other.result.windows.length === 0, JSON.stringify(other));
    const cmds = await ask('commands', Object.assign({ window: ed.window }, mine));
    ok('a window\'s commands are listed through the bridge', cmds.result && cmds.result.commands.some(c => c.id === 'view-split'), JSON.stringify(cmds).slice(0, 200));
    const ran = await ask('run', Object.assign({ window: ed.window, command: 'view-split' }, mine));
    ok('a command run through the bridge switches the view', ran.result && ran.result.ran && await editor.locator('[data-ed-panes]').getAttribute('data-view') === 'split', JSON.stringify(ran));
    await ask('run', Object.assign({ window: ed.window, command: 'view-page' }, mine));
    const elsewhere = await ask('run', { window: ed.window, command: 'view-split', workspace: 'C:/Elsewhere', workspaceId: 'w2' });
    ok('another workspace cannot run the window\'s commands', !!elsewhere.err && !elsewhere.result, JSON.stringify(elsewhere));
    await editor.evaluate(win => { window.__undoTest = window.pudlCommands.register(win, [{ id: 'test-destroy', label: 'Destroy Everything', danger: true, run: () => { window.__destroyed = true; } }]); });
    const danger = await ask('run', Object.assign({ window: ed.window, command: 'test-destroy' }, mine));
    ok('a command that would destroy something is left to the person', !!danger.err && !(await p.evaluate(() => window.__destroyed)), JSON.stringify(danger));
    await p.evaluate(() => window.__undoTest());
    const missing = await ask('run', Object.assign({ window: ed.window, command: 'no-such-command' }, mine));
    ok('an unknown command is refused, saying so', !!missing.err && /no command/.test(missing.err), JSON.stringify(missing));

    // Save takes typing that has not yet reached the text.
    await pm.locator('p').filter({ hasText: 'Last paragraph.' }).click();
    await p.keyboard.press('End');
    await p.waitForTimeout(80);
    await p.keyboard.type(' Saved.');
    await editor.locator('[data-ed-save]').click();
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'save'));
    const saved = await p.evaluate(() => window.__calls.filter(c => c[0] === 'save').pop()[1]);
    ok('Save takes the last typing on the page', saved.includes('Last paragraph. Saved.'), saved);

    // A change made elsewhere is read into the page.
    await p.waitForTimeout(400);
    await p.evaluate(t => window.__change(t.replace('Last paragraph. Saved.', 'Rewritten by an agent.')), await text());
    await pm.locator('p').filter({ hasText: 'Rewritten by an agent.' }).waitFor();
    ok('a change made elsewhere appears on the page', true);
    await t.shot('page-view');

    // Find and Replace on the page: Ctrl+F stays in Visual, marks the matches
    // on the page and counts them, and a replacement on the page reaches the
    // text.
    await pm.locator('p').filter({ hasText: /^Rewritten by an agent\.$/ }).click();
    await p.keyboard.press('Control+f');
    await editor.locator('[data-ed-find]').waitFor();
    ok('Ctrl+F in Visual opens the find bar and stays in Visual', await editor.locator('[data-ed-panes]').getAttribute('data-view') === 'page' && await editor.locator('[data-ed-find-replace-row]').isHidden());
    await editor.locator('[data-ed-find-input]').fill('by an agent');
    await editor.locator('[data-ed-find-count]').getByText('1 of 1').waitFor();
    ok('the matches are marked on the page, the current one set apart', await pm.locator('.pm-find-current').count() === 1);
    await t.shot('page-find');
    await editor.locator('[data-ed-find-show-replace]').click();
    await editor.locator('[data-ed-replace-input]').fill('by a person');
    await editor.locator('[data-ed-replace-one]').click();
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'update' && c[1].includes('Rewritten by a person.')));
    ok('Replace on the page reaches the text', (await editor.locator('[data-ed-find-count]').textContent()) === 'No matches');
    await editor.locator('[data-ed-find-input]').fill('by a person');
    await editor.locator('[data-ed-replace-input]').fill('by an agent');
    await editor.locator('[data-ed-replace-all]').click();
    await pm.locator('p').filter({ hasText: /^Rewritten by an agent\.$/ }).waitFor();
    await p.waitForFunction(() => window.__calls.filter(c => c[0] === 'update').pop()[1].includes('Rewritten by an agent.'));
    ok('Replace All on the page reaches the text', true);
    await editor.locator('[data-ed-find-close]').click();
    ok('Close hides the bar and its marks on the page', await editor.locator('[data-ed-find]').isHidden() && await pm.locator('.pm-find-match').count() === 0);

    // Ctrl+E goes on to Text, and the Preview button draws the page from
    // the text, with no editor on it.
    await editor.locator('[data-ed-scroll]').focus();
    await p.keyboard.press('Control+e');
    ok('Ctrl+E goes from Visual to Text', await editor.locator('[data-ed-panes]').getAttribute('data-view') === 'text' && await editor.locator('.cm-content').isVisible());
    await editor.locator('[data-command="view-preview"]').click();
    await page.locator('.doc-block').first().waitFor();
    ok('Preview is the page drawn from the text, with no editor on it', await editor.locator('[data-ed-panes]').getAttribute('data-view') === 'preview' &&
      await page.locator('.ProseMirror').count() === 0 && (await page.textContent()).includes('Rewritten by an agent.'));

    await editor.locator('[data-command="view-page"]').click();
    await pm.waitFor();

    // Suggestions are shown on the page, block by block: the block a
    // suggestion changes struck through, the suggested block below it with
    // Accept and Reject, and the window stays on the page.
    await p.evaluate(() => window.__suggest('Rewritten by an agent.', 'The last paragraph, as suggested.', 'a clearer ending'));
    const sugg = pm.locator('.pm-sugg[data-suggestion="s1"]');
    await sugg.waitFor();
    ok('a suggestion is shown on the page, below the block it changes', await pm.locator('.pm-sugg-old[data-suggestion="s1"]').textContent() === 'Rewritten by an agent.'
      && (await sugg.textContent()).includes('Fixture: a clearer ending') && (await p.evaluate(() => window.__calls.filter(c => c[0] === 'source').pop()[1])) === 'The last paragraph, as suggested.');
    ok('the card is laid out so that a struck-through block around it does not strike it, on a solid ground', await sugg.evaluate(el => { const s = getComputedStyle(el); return s.display === 'inline-block' && !/transparent|rgba\(0, 0, 0, 0\)/.test(s.backgroundColor); }));
    ok('the window stays on the page when suggestions arrive', await p.locator('.win[data-win-kind="editor"]').first().locator('[data-ed-panes]').getAttribute('data-view') === 'page');
    // Go To on the proposal's row takes the reader to the change.
    await pm.evaluate(el => { const s = el.closest('[data-ed-scroll]') || el.parentElement; s.scrollTop = 0; });
    const proposal = p.locator('.win[data-win-kind="editor"]').first().locator('.editor-proposal').first();
    await proposal.getByRole('button', { name: 'Go To' }).click();
    await p.waitForTimeout(150);
    ok('Go To on a proposal scrolls the page to its change and focuses its Accept', await sugg.evaluate(el => {
      const r = el.getBoundingClientRect(), box = (el.closest('[data-ed-scroll]') || document.documentElement).getBoundingClientRect();
      return r.top >= box.top && r.bottom <= box.bottom + 1 && el.classList.contains('flash') && document.activeElement === el.querySelector('.btn-primary');
    }));
    await t.shot('page-suggestion');
    // The suggestion follows the dark document surface and keeps light text.
    const wasTheme = await p.evaluate(() => document.documentElement.getAttribute('data-theme'));
    await p.evaluate(() => document.documentElement.setAttribute('data-theme', 'dark'));
    const ground = await sugg.evaluate(el => { const c = getComputedStyle(el); return [c.backgroundColor, c.color]; });
    const light = rgb => { const n = (rgb.match(/[\d.]+/g) || []).map(Number); return (n[0] + n[1] + n[2]) / 3 > 200; };
    ok('in the dark theme the suggestion is dark with light text', !light(ground[0]) && light(ground[1]), JSON.stringify(ground));
    await sugg.screenshot({ path: path.join(OUT, 'page-suggestion-dark.png') });
    await p.evaluate(t => { if (t == null) document.documentElement.removeAttribute('data-theme'); else document.documentElement.setAttribute('data-theme', t); }, wasTheme);
    // Edit puts the suggested words in a box; Escape leaves them as they were.
    await sugg.getByRole('button', { name: 'Edit this suggestion' }).click();
    const editBox = sugg.locator('textarea.sugg-edit-text');
    ok('Edit puts the suggestion\'s words in a box in its card', await editBox.inputValue() === 'The last paragraph, as suggested.' && await editBox.evaluate(el => el === document.activeElement));
    await p.keyboard.press('Escape');
    ok('Escape leaves the suggestion as it was', await sugg.locator('textarea').count() === 0 && !(await p.evaluate(() => window.__calls.some(c => c[0] === 'revise'))));
    // Double-clicking the suggested block edits it too; Ctrl+Enter keeps the
    // new wording in the suggestion, and the text stays as it was.
    await sugg.locator('.pm-sugg-new').dblclick();
    await editBox.fill('The last paragraph, in my words.');
    await t.shot('page-suggestion-edit');
    await p.keyboard.press('Control+Enter');
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'revise' && c[2] === 'The last paragraph, in my words.'));
    const edited = pm.locator('.pm-sugg[data-suggestion="s1"]');
    await edited.filter({ hasText: 'edited by you' }).waitFor();
    await p.waitForTimeout(200);
    ok('the edited suggestion shows the new wording and says it was edited, and the text is unchanged', (await edited.textContent()).includes('Suggested by Fixture, edited by you') &&
      (await p.evaluate(() => window.__calls.filter(c => c[0] === 'source').pop()[1])) === 'The last paragraph, in my words.' && !(await p.evaluate(() => window.__text())).includes('in my words'));
    await edited.getByRole('button', { name: 'Accept this suggestion' }).click();
    await p.waitForFunction(() => window.__text().includes('The last paragraph, in my words.') && !document.querySelector('.doc-page .pm-sugg'));
    await pm.locator('p').filter({ hasText: 'The last paragraph, in my words.' }).waitFor();
    ok('Accept on the page puts the edited suggestion in the text and clears its marks', await pm.locator('p').filter({ hasText: 'The last paragraph, in my words.' }).count() === 1 && await pm.locator('.pm-sugg-old').count() === 0);
    await p.evaluate(() => window.__change(window.__text().replace('The last paragraph, in my words.', 'The last paragraph, as suggested.')));
    await pm.locator('p').filter({ hasText: 'The last paragraph, as suggested.' }).waitFor();
    await p.evaluate(() => window.__suggest('The last paragraph, as suggested.', '', 'not needed'));
    await pm.locator('.pm-sugg').waitFor();
    ok('a suggestion that removes a block says so', (await pm.locator('.pm-sugg').textContent()).includes('Removes the struck-through text.'));
    await pm.locator('.pm-sugg').getByRole('button', { name: 'Reject this suggestion' }).click();
    await p.waitForFunction(() => !document.querySelector('.doc-page .pm-sugg'));
    ok('Reject on the page drops the suggestion and keeps the text', (await p.evaluate(() => window.__text())).includes('The last paragraph, as suggested.') && (await p.evaluate(() => window.__calls.some(c => c[0] === 'reject'))));

    // Another document opens in its own window, in Split.
    await p.evaluate(() => window.dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'notes.md' }));
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="editor"]').length === 2);
    const second = p.locator('.win[data-win-kind="editor"]').nth(1);
    await second.locator('.cm-content').waitFor();
    ok('another document opens in its own window, in Split', await second.locator('[data-ed-panes]').getAttribute('data-view') === 'split');
    // In Split the page beside the text is an editor too, and typing on
    // either side reaches the other.
    await second.locator('[data-ed-page] .ProseMirror').waitFor();
    ok('Split shows the page as an editor beside the text', await second.locator('[data-ed-page] .ProseMirror').isVisible() && await second.locator('.cm-content').isVisible());
    await second.locator('.cm-content').click();
    await p.keyboard.press('Control+End');
    await p.keyboard.type('\n\nTyped in the text.');
    await second.locator('[data-ed-page] p').filter({ hasText: 'Typed in the text.' }).waitFor();
    ok('typing in the text is read into the page', true);
    await second.locator('[data-ed-page] p').filter({ hasText: 'Typed in the text.' }).click();
    await p.keyboard.press('End');
    await p.keyboard.type(' And on the page.');
    await p.waitForFunction(() => Array.from(document.querySelectorAll('.win[data-win-kind="editor"]'))[1].querySelector('.cm-content').textContent.includes('Typed in the text. And on the page.'));
    ok('typing on the page reaches the text', true);
    await t.shot('page-split');

    // In Split, Insert puts the block's Markdown after the cursor's line.
    await second.locator('.cm-content').click();
    await p.keyboard.press('Control+Home');
    await p.evaluate(() => window.dinahCommands.searchable().find(c => c.id === 'editor-insert-code').run());
    ok('in Split, Insert puts the Markdown in after the cursor\'s line', (await second.locator('.cm-content').textContent()).includes('---```code```'));
    await p.evaluate(() => window.dinahCommands.searchable().find(c => c.id === 'editor-insert-checklist').run());
    ok('in Split, Insert Checklist adds checkbox Markdown', (await second.locator('.cm-content').textContent()).includes('- [ ] Type here.'));

    // A table whose cells hold long links stays within the page: the links
    // break rather than widen it.
    const fits = await p.evaluate(() => {
      const page = document.createElement('div');
      page.className = 'doc-page page-view';
      page.style.width = '600px';
      const link = 'https://tracker.example.com/browse/' + 'VERYLONGKEY'.repeat(12) + '-50';
      page.innerHTML = '<table><tbody><tr><th>Dependency</th><th>Type</th><th>Status</th><th>Reference</th></tr><tr><td>One</td><td>Two</td><td>Three</td><td><a href="#">' + link + '</a> <code>registerTenderLineItemWithAVeryLongName</code></td></tr></tbody></table>';
      document.body.appendChild(page);
      const r = [Math.round(page.querySelector('table').getBoundingClientRect().right), Math.round(page.getBoundingClientRect().right)];
      page.remove();
      return r;
    });
    ok('a table with long links stays within the page', fits[0] <= fits[1], fits.join(' > '));

    // The fixture chose Split in Settings; with no choice, Markdown opens in
    // Visual (document_views.go).
    await p.evaluate(() => window.dinahDocumentViews({ 'text/markdown': 'page' }));
    await p.evaluate(() => window.dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'third.md' }));
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="editor"]').length === 3);
    const third = p.locator('.win[data-win-kind="editor"]').nth(2);
    await third.locator('[data-ed-page] .ProseMirror').waitFor();
    ok('with no choice in Settings, Markdown opens in Visual', await third.locator('[data-ed-panes]').getAttribute('data-view') === 'page');

    await editor.evaluate(el => pudlWindows.raise(el.dataset.win));
    await pm.locator('p').filter({ hasText: 'The last paragraph, as suggested.' }).click();
    await p.keyboard.press('End');
    await command('editor-insert-checklist');
    await p.keyboard.type('New checklist item');
    await waitText('- [ ] New checklist item');
    const newTask = pm.locator('li.pm-task').filter({ hasText: 'New checklist item' });
    ok('Insert Checklist creates a rich item with its placeholder selected', await newTask.locator('input').count() === 1 && await pm.locator('textarea.page-source').count() === 0);
    await p.keyboard.press('Enter');
    await p.keyboard.type('Next checklist item');
    await waitText('- [ ] New checklist item\n- [ ] Next checklist item');
    await newTask.locator('input').click();
    await waitText('- [x] New checklist item');
    await p.keyboard.press('Control+z');
    await waitText('- [ ] New checklist item');
    ok('an inserted checklist supports Enter, checking and Undo', !(await newTask.locator('input').isChecked()));

    await pm.locator('p').filter({ hasText: 'The last paragraph, as suggested.' }).click();
    await p.keyboard.press('End');
    await p.waitForTimeout(80);
    await p.keyboard.press('Enter');
    await pm.locator('.pm-hint').waitFor();
    await p.keyboard.type('/checklist');
    await slash.getByText('Checklist', { exact: true }).waitFor();
    await p.keyboard.press('Enter');
    await p.keyboard.type('Slash checklist item');
    await waitText('- [ ] Slash checklist item');
    ok('/checklist creates an editable checkbox item', await pm.locator('li.pm-task').filter({ hasText: 'Slash checklist item' }).locator('input').count() === 1);
  } catch (e) {
    t.errors.push(String(e && e.stack || e));
  } finally {
    await t.close();
  }
})();
