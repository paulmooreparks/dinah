// Cut, Copy and Paste in the Editor: with the keys and with the Edit menu,
// on the page (Visual), in the text and in a box. On the page, what is
// copied keeps its form when it is pasted (code, tables, lists, emphasis),
// HTML from another program keeps its headings, lists, tables and links, and
// plain text is read as Markdown. The menu's Paste takes the clipboard Go
// reads (ClipboardRead). The keys are pressed as a person presses them, on
// the test browser's own clipboard (never the computer's): the page is
// served from an https address, where the browser lets the test put what
// another program would put there and read what a copy put there. The
// fixture reads and writes the document as pageview (Go) does, simplified,
// as page-view.js does.
const { launch, FRONTEND } = require('./lib/page');
const path = require('path');
(async () => {
  const t = await launch({
    viewport: { width: 2560, height: 1200 },
    state: { workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'], defaultProvider: 'devin', windowModes: { editor: 'maximized' }, windowModesShipped: { editor: 'maximized' }, titles: {}, templates: [],
      workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [{ id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] },
    app: () => {
      window.__calls = [];
      // What Go's ClipboardRead finds on the clipboard.
      window.__clipboard = { html: '', text: '' };
      let n = 0;
      const rev = () => 'r' + (++n);
      const drafts = {};
      const TEXT = ['---', 'title: Scanner', '---', '', '# Scanner', '', 'The lexer reads the **token** number.', '', '- one', '- two', '',
        '```', 'code here', '```', '', 'Last paragraph.', ''].join('\n');
      const esc = s => s.replace(/&/g, '&amp;').replace(/</g, '&lt;');
      const inl = s => {
        const out = [];
        s.split('\n').forEach((line, li) => {
          if (li) out.push({ type: 'soft_break' });
          line.split('**').forEach((t, k) => { if (t) out.push(k % 2 ? { type: 'text', text: t, marks: [{ type: 'strong' }] } : { type: 'text', text: t }); });
        });
        return out;
      };
      const marked = x => {
        const m = (x.marks || []).map(k => k.type);
        let t = x.text;
        if (m.includes('code')) t = '`' + t + '`';
        if (m.includes('em')) t = '*' + t + '*';
        if (m.includes('strong')) t = '**' + t + '**';
        const link = (x.marks || []).find(k => k.type === 'link');
        if (link) t = '[' + t + '](' + link.attrs.href + ')';
        return t;
      };
      const outl = nodes => (nodes || []).map(x => x.type === 'soft_break' ? '\n' : marked(x)).join('');
      let uid = 0;
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
            else if (block[0].startsWith('- ')) node = { type: 'bullet_list', attrs: { tight: true }, content: block.map(l => ({ type: 'list_item', content: [{ type: 'paragraph', content: inl(l.slice(2)) }] })) };
            else node = { type: 'paragraph', content: inl(block.join('\n')) };
          }
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
        : b.type === 'bullet_list' ? b.content.map(li => '- ' + outl(li.content[0].content)).join('\n')
          : b.type === 'source_block' ? b.attrs.src
              : b.type === 'table' ? b.content.map((row, i) => row.content.map(c => '| ' + (outl(c.content) ? outl(c.content) + ' ' : '')).join('') + '|' +
                (i === 0 ? '\n' + row.content.map(() => '| --- ').join('') + '|' : '')).join('\n')
              : outl(b.content);
      const seq = (nodes, first) => { let s = '', started = false; nodes.forEach(nd => { const t = write(nd); if (!t) return; s += (nd.attrs && typeof nd.attrs.gap === 'string' ? nd.attrs.gap : started ? '\n\n' : first) + t; started = true; }); return s; };
      const markdownOf = doc => doc.attrs.front + seq(doc.content, '\n') + doc.attrs.tail;
      const view = d => Object.assign({}, d, { spans: {}, suggestions: [], changed: d.text !== d.baseline.text, writeBack: 'save', actions: ['update', 'discard', 'save-as', 'save'] });
      const get = id => drafts[id];
      window.__text = () => Object.values(drafts)[0].text;
      window.__change = text => { const d = Object.values(drafts)[0]; d.text = text; d.revision = rev(); window.__emit('draft:changed', { id: d.id, revision: d.revision, kind: 'updated' }); };
      return {
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
          if (d.revision !== expected) throw new Error('stale');
          d.text = text; d.revision = rev();
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'updated' });
          return view(d);
        },
        SaveDraft: async (id, expected) => {
          const d = get(id);
          if (d.revision !== expected) throw new Error('stale');
          d.baseline = { text: d.text, revision: 'b' + rev(), title: d.title };
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'saved' });
          return Object.assign(view(d), { notes: [] });
        },
        DraftReadiness: async id => ({ ready: true, revision: get(id).revision, items: [], summary: '' }),
        PreviewDraft: async id => { const d = get(id); return { revision: d.revision, problems: [], blocks: [{ line: 1, html: '<p>' + esc(d.text).replace(/\n/g, '<br>') + '</p>' }] }; },
        PageDocumentOf: async (id, text) => docOf(text),
        PageDocumentMarkdown: async doc => markdownOf(doc),
        // Markdown as blocks, as pageview reads a pasted passage or an
        // inserted block.
        PageBlocksOf: async (id, md) => {
          window.__calls.push(['blocks', md]);
          const lines = md.split('\n');
          return read(lines, 0, lines.length).map(b => { delete b.attrs.uid; return b; });
        },
        PageSourceOf: async (id, src) => ({ html: '<pre><code>' + esc(src.split('\n').slice(1, -1).join('\n')) + '</code></pre>', diagrams: [] }),
        ClipboardRead: async () => { window.__calls.push(['clipboard-read']); return window.__clipboard; },
        ListDrafts: async () => Object.values(drafts).map(view)
      };
    },
    init: () => {
      // What a copy or a cut put on the clipboard, as the editor set it.
      window.__clip = null;
      for (const kind of ['copy', 'cut']) window.addEventListener(kind, e => { window.__clip = { kind, html: e.clipboardData.getData('text/html'), text: e.clipboardData.getData('text/plain') }; });
    },
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    // The browser's clipboard is open to the page only under https, so this
    // suite serves the page from an https address of its own.
    await t.context.route('https://dinah.test/**', route => route.fulfill({ path: path.join(FRONTEND, new URL(route.request().url()).pathname) }));
    await t.context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: 'https://dinah.test' });
    await p.goto('https://dinah.test/index.html');
    await t.waitReady(() => window.dinahEditor && window.dinahWorkspaceBrowser.records().length === 1);
    await p.evaluate(() => window.dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'note.md' }));
    const editor = p.locator('.win[data-win-kind="editor"]').first();
    const pm = editor.locator('[data-ed-page] .ProseMirror');
    await pm.waitFor();
    // A document opens in Split; the page alone is Visual.
    await editor.locator('[data-command="view-page"]').click();
    await editor.locator('[data-command="view-page"][aria-pressed="true"]').waitFor();
    const text = () => p.evaluate(() => window.__text());
    const waitText = (test, label) => p.waitForFunction(t => new Function('s', 'return ' + t)(window.__text()), test.toString().replace(/^s\s*=>\s*/, ''), { timeout: 5000 }).catch(() => { throw new Error('the text never ' + label + ': ' + JSON.stringify(null)); });
    const until = async (fn, label) => {
      for (let i = 0; i < 50; i++) { if (fn(await text())) return true; await p.waitForTimeout(100); }
      console.log('  text was: ' + JSON.stringify(await text()));
      return false;
    };
    const clip = () => p.evaluate(() => window.__clip);
    // Puts the cursor at the end of a block, and waits until it is there and
    // the editor has taken it: the editor takes the place of a click a
    // moment after it, sooner than a person moves on to the next key.
    const caretAtEnd = async (locator, words) => {
      for (let i = 0; i < 5; i++) {
        await p.waitForTimeout(150);
        await locator.click();
        await p.keyboard.press('End');
        if (await p.evaluate(w => {
          const s = getSelection(), n = s.anchorNode;
          const block = n && (n.nodeType === 3 ? n.parentElement : n).closest('p, li, h1, h2, h3, td, th');
          return s.isCollapsed && !!block && block.textContent.includes(w);
        }, words)) { await p.waitForTimeout(300); return; }
      }
      throw new Error('the cursor could not be put after "' + words + '"');
    };
    // Selects a word by double-clicking it, and waits until the page has it.
    const selectWord = async (locator, word) => {
      await locator.dblclick();
      await p.waitForFunction(w => getSelection().toString().trim() === w, word);
      await p.waitForTimeout(300);
    };
    // Ctrl+V, of what is on the clipboard.
    const pasteClip = () => p.keyboard.press('Control+v');
    // Puts on the clipboard what another program would, and pastes it with
    // Ctrl+V.
    const keyPaste = async (html, plain) => {
      await p.evaluate(([h, t]) => {
        const parts = { 'text/plain': new Blob([t], { type: 'text/plain' }) };
        if (h) parts['text/html'] = new Blob([h], { type: 'text/html' });
        return navigator.clipboard.write([new ClipboardItem(parts)]);
      }, [html, plain]);
      await p.keyboard.press('Control+v');
    };
    const original = await text();
    /* Puts the document back as it was, and waits until the page shows it:
       the page is built again from the text a moment after the text changes,
       which puts the cursor at its start. */
    const pageOriginal = await pm.evaluate(el => el.textContent);
    const reset = async () => {
      // The Editor sends what was typed or pasted first; a change made
      // elsewhere while it has changes not yet sent asks which to keep.
      // While it has such changes its line says it is keeping them.
      let last = await text(), still = 0;
      for (let i = 0; i < 80 && still < 6; i++) {
        await p.waitForTimeout(100);
        const now = await text(), keeping = (await editor.locator('[data-ed-meta]').textContent()).includes('keeping');
        still = now === last && !keeping ? still + 1 : 0;
        last = now;
      }
      await p.evaluate(t => window.__change(t), original);
      await until(s => s === original);
      await p.waitForFunction(t => document.querySelector('.win[data-win-kind="editor"] [data-ed-page] .ProseMirror').textContent === t, pageOriginal, { timeout: 5000 })
        .catch(async () => { throw new Error('the page did not show the document again: ' + JSON.stringify(await pm.evaluate(el => el.textContent)) + ' / notice: ' + JSON.stringify(await editor.locator('[data-ed-notice]').isVisible() && await editor.locator('[data-ed-notice]').textContent()) + ' / meta: ' + JSON.stringify(await editor.locator('[data-ed-meta]').textContent()) + ' / status: ' + JSON.stringify(await editor.locator('[data-ed-status]').textContent().catch(() => ''))); });
      await p.waitForTimeout(300);
    };

    // ---- With the keys, on the page ----
    await selectWord(pm.locator('strong').filter({ hasText: 'token' }), 'token');
    await p.keyboard.press('Control+c');
    const word = await clip();
    ok('Ctrl+C on the page puts the selected words on the clipboard, as HTML with their emphasis and as text',
      word && word.kind === 'copy' && /<strong>token<\/strong>/.test(word.html) && word.text.trim() === 'token', JSON.stringify(word));
    await caretAtEnd(pm.locator('p').filter({ hasText: 'Last paragraph.' }), 'Last paragraph.');
    await pasteClip();
    ok('pasting them into another paragraph puts them there, emphasis and all', await until(s => s.includes('Last paragraph.**token**')));
    await selectWord(pm.locator('p').filter({ hasText: 'Last paragraph.' }).locator('strong'), 'token');
    await p.keyboard.press('Control+x');
    ok('Ctrl+X cuts the selection from the page and the text', (await clip()).kind === 'cut' && await until(s => s.includes('Last paragraph.\n') && !s.includes('Last paragraph.**token**')));

    // The whole page copied and pasted over itself is the same document,
    // code and list included.
    await pm.locator('li').first().click();
    await p.keyboard.press('Control+a');
    await p.keyboard.press('Control+c');
    const whole = await clip();
    ok('Select All and Ctrl+C copy every block, saying in the HTML what each is', whole.html.includes('pm-source') && whole.html.includes('data-src') && /<h1[ >]/.test(whole.html), whole.html.slice(0, 300));
    await pasteClip();
    ok('pasting the whole page back gives the same document, code and list kept', await until(s => s === original), JSON.stringify(original));
    await t.shot('page-clipboard');
    await reset();

    // HTML from another program, such as a word processor or a browser.
    await caretAtEnd(pm.locator('p').filter({ hasText: 'Last paragraph.' }), 'Last paragraph.');
    await p.keyboard.press('Enter');
    await keyPaste('<h2>From Word</h2><p>One <b>bold</b>, <i>italic</i> and <a href="https://x.test/">a link</a>.</p><ul><li>alpha</li><li>beta</li></ul><table><tr><th>H1</th><th>H2</th></tr><tr><td>a</td><td>b</td></tr></table>', 'From Word');
    ok('HTML from another program keeps its heading, emphasis, link, list and table',
      await until(s => s.includes('## From Word') && s.includes('One **bold**, *italic* and [a link](https://x.test/).') && s.includes('- alpha\n- beta') && s.includes('| H1 | H2 |\n| --- | --- |\n| a | b |')));
    await reset();

    // Plain text is read as Markdown.
    await caretAtEnd(pm.locator('p').filter({ hasText: 'Last paragraph.' }), 'Last paragraph.');
    await p.keyboard.press('Enter');
    await keyPaste('', '## Pasted Heading\n\n- first\n- second\n\nA **strong** close.');
    ok('plain text with Markdown in it is pasted as blocks: a heading, a list and emphasis',
      await until(s => s.includes('## Pasted Heading\n\n- first\n- second\n\nA **strong** close.')));
    await caretAtEnd(pm.locator('p').filter({ hasText: 'The lexer reads' }), 'The lexer reads');
    await keyPaste('', ' and a **bold** word');
    ok('one line of Markdown goes into the paragraph the cursor is in', await until(s => s.includes('number. and a **bold** word')));
    const asked = await p.evaluate(() => window.__calls.filter(c => c[0] === 'blocks').length);
    await keyPaste('', ' plain words');
    ok('a line with no Markdown in it is pasted as it is, without reading it as Markdown',
      await until(s => s.includes('number. and a **bold** word plain words')) && await p.evaluate(n => window.__calls.filter(c => c[0] === 'blocks').length === n, asked));
    await reset();

    // A paste replaces the selection, and Undo takes a paste or a cut back.
    await selectWord(pm.locator('strong').filter({ hasText: 'token' }), 'token');
    await keyPaste('<p>line</p>', 'line');
    ok('a paste over a selection replaces it', await until(s => s.includes('The lexer reads the line')));
    await p.keyboard.press('Control+z');
    ok('Undo takes the paste back', await until(s => s.includes('The lexer reads the **token** number.')));
    await selectWord(pm.locator('strong').filter({ hasText: 'token' }), 'token');
    await p.keyboard.press('Control+x');
    await until(s => !s.includes('token'));
    await p.keyboard.press('Control+z');
    ok('Undo takes a cut back', await until(s => s.includes('The lexer reads the **token** number.')));

    // Blocks pasted into a table's cell, which holds only a line of text,
    // leave the table whole.
    await caretAtEnd(pm.locator('p').filter({ hasText: 'Last paragraph.' }), 'Last paragraph.');
    await p.keyboard.press('Enter');
    await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Insert$/ }).click();
    await p.locator('#menubar-panel').getByText('Table', { exact: true }).click();
    await until(s => s.includes('| Heading 1 | Heading 2 | Heading 3 |'));
    await caretAtEnd(pm.locator('td').first(), '');
    await keyPaste('', '## A heading\n\n- and\n- a list');
    ok('Markdown blocks pasted into a table cell go in as their words, and the table stays whole',
      await until(s => s.includes('| Heading 1 | Heading 2 | Heading 3 |\n| --- | --- | --- |\n| A heading and a list |')) && (await pm.locator('table').count()) === 1);
    await caretAtEnd(pm.locator('td').nth(1), '');
    await keyPaste('<h3>Word heading</h3><p>and a paragraph</p>', 'Word heading\n\nand a paragraph');
    ok('HTML blocks pasted into a table cell go in as their words too',
      await until(s => s.includes('| A heading and a list | Word heading and a paragraph |')) && (await pm.locator('table').count()) === 1);
    await reset();

    // ---- With the Edit menu ----
    // Escape would also close the find bar, so it is pressed only to close
    // a menu left open.
    const editMenu = async () => {
      if (await p.locator('#menubar-panel').isVisible()) await p.keyboard.press('Escape');
      await p.locator('.menubar-row .menubar-title').filter({ hasText: /^Edit$/ }).click();
      await p.locator('#menubar-panel .menu-action').first().waitFor();
    };
    const choose = async label => { await editMenu(); await p.locator('#menubar-panel .menu-action').filter({ hasText: new RegExp('^' + label) }).first().click(); };
    await editMenu();
    const rows = await p.locator('#menubar-panel').textContent();
    ok('Edit has Cut, Copy, Paste and Select All', ['Cut', 'Copy', 'Paste', 'Select All'].every(w => rows.includes(w)), rows);
    // PUDL keeps these keys for the field being typed in and shows them on
    // no row; Help › Keyboard Shortcuts lists them.
    await p.keyboard.press('Escape');
    await p.keyboard.press('F1');
    const helpDialog = p.locator('dialog.keyboard-help[open]');
    await helpDialog.waitFor();
    await helpDialog.locator('#keyboard-help-for').selectOption('Editor');
    const help = await helpDialog.locator('tbody').textContent();
    ok('Keyboard Shortcuts lists Cut, Copy, Paste and Select All with their keys', ['CutEditorCtrl+X', 'CopyEditorCtrl+C', 'PasteEditorCtrl+V', 'Select AllEditorCtrl+A'].every(k => help.replace(/\s+/g, '').includes(k.replace(/\s+/g, ''))), help.slice(0, 300));
    await helpDialog.locator('[data-close]').click();
    await p.keyboard.press('Escape');

    await selectWord(pm.locator('strong').filter({ hasText: 'token' }), 'token');
    await p.evaluate(() => { window.__clip = null; });
    await choose('Copy');
    const menuCopy = await clip();
    ok('Edit › Copy copies the selection on the page, as Ctrl+C does', menuCopy && menuCopy.kind === 'copy' && /<strong>token<\/strong>/.test(menuCopy.html), JSON.stringify(menuCopy));
    ok('the page has the focus again after the menu', await p.evaluate(() => !!document.activeElement.closest('.ProseMirror')));
    await choose('Cut');
    // A double-click selects the word and the space after it.
    ok('Edit › Cut cuts the selection on the page', (await clip()).kind === 'cut' && await until(s => s.includes('The lexer reads the number.')));
    // A space at the end of pasted HTML is dropped, as a browser drops it.
    await p.evaluate(() => { window.__clipboard = { html: '<p><b>Menu</b> paste </p>', text: 'Menu paste ' }; });
    await choose('Paste');
    ok('Edit › Paste pastes the HTML Go read from the clipboard on the page', await until(s => s.includes('The lexer reads the **Menu** pastenumber.')));
    await p.evaluate(() => { window.__clipboard = { html: '', text: '- menu one\n- menu two' }; });
    await caretAtEnd(pm.locator('p').filter({ hasText: 'Last paragraph.' }), 'Last paragraph.');
    await p.keyboard.press('Enter');
    await choose('Paste');
    ok('Edit › Paste of plain text reads it as Markdown on the page', await until(s => s.includes('- menu one\n- menu two')));
    await choose('Select All');
    await choose('Copy');
    ok('Edit › Select All then Copy copies the whole page', (await clip()).html.includes('pm-source') && (await clip()).text.includes('Last paragraph.'));
    await reset();

    // A code block's Markdown is edited in a box on the page. Ctrl+V pastes
    // into the box. The box closes when the menu is opened, so the menu's
    // Paste then says so and pastes nothing, rather than over the block.
    await pm.locator('.pm-source[data-kind="code"]').click();
    const codeBox = pm.locator('textarea.page-source');
    await codeBox.waitFor();
    await codeBox.press('Control+End');
    await codeBox.press('ArrowUp');
    await codeBox.press('End');
    await keyPaste('<p>ignored</p>', ' pasted');
    ok('Ctrl+V in a code block\'s Markdown pastes there, as text', await codeBox.inputValue() === '```\ncode here pasted\n```', await codeBox.inputValue());
    await p.evaluate(() => { window.__clipboard = { html: '<p>menu</p>', text: 'menu' }; });
    await choose('Paste');
    ok('leaving the box for the menu puts the pasted Markdown into the text', await until(s => s.includes('```\ncode here pasted\n```')));
    ok('the menu\'s Paste then pastes nothing and says why, so the code block is not pasted over',
      !(await text()).includes('menu') && (await editor.locator('[data-ed-status]').textContent()).includes('has closed'), await editor.locator('[data-ed-status]').textContent());
    await reset();

    // ---- In the text ----
    await editor.locator('[data-command="view-text"]').click();
    const cm = editor.locator('.cm-content');
    await cm.waitFor();
    await cm.locator('.cm-line').filter({ hasText: 'Last paragraph.' }).click();
    await p.keyboard.press('End');
    await p.keyboard.press('Shift+Home');
    await p.evaluate(() => { window.__clip = null; });
    await choose('Copy');
    ok('Edit › Copy in the text copies the selected Markdown', (await clip()) && (await clip()).text === 'Last paragraph.', JSON.stringify(await clip()));
    await choose('Cut');
    ok('Edit › Cut in the text cuts it', await until(s => !s.includes('Last paragraph.')));
    await p.evaluate(() => { window.__clipboard = { html: '<p>ignored</p>', text: 'Pasted **as Markdown**' }; });
    await choose('Paste');
    ok('Edit › Paste in the text pastes the clipboard\'s text, as it is', await until(s => s.includes('Pasted **as Markdown**') && !s.includes('ignored')));
    await choose('Select All');
    ok('Edit › Select All in the text selects all of it', await p.evaluate(() => getSelection().toString().includes('# Scanner') && getSelection().toString().includes('Pasted')));
    await p.keyboard.press('Control+End');

    // ---- In a box: the find bar ----
    await p.keyboard.press('Control+f');
    const find = editor.locator('[data-ed-find-input]');
    await find.waitFor();
    await p.evaluate(() => { window.__clipboard = { html: '', text: 'token' }; });
    await choose('Paste');
    ok('Edit › Paste in a box, such as Find, pastes the text there', await find.inputValue() === 'token');
    await choose('Select All');
    await choose('Cut');
    // A box fills the clipboard itself after the cut, so the clipboard is read.
    ok('Edit › Select All and Cut in a box act on the box', await find.inputValue() === '' && await p.evaluate(() => navigator.clipboard.readText()) === 'token');
  } catch (e) {
    t.errors.push(String(e && e.stack || e));
  } finally {
    await t.close();
  }
})();
