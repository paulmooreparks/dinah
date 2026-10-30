// Ctrl+Tab, which switches windows as Alt+Tab does on the Windows desktop:
// the two most recent windows, a longer walk while Ctrl is held, docked
// windows left out, a minimised window restored, Escape, and Ctrl+PageDown
// still going through every window in order.
const { launch, OUT } = require('./lib/page');
const path = require('path');
(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    state: { workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'], documentViews: { 'text/markdown': 'split' }, titles: {}, templates: [],
      workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [{ id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] },
    app: () => {
      let n = 0;
      const drafts = {};
      const get = id => { const d = drafts[id]; if (!d) throw new Error('the draft no longer exists'); return d; };
      const view = d => Object.assign({}, d, { spans: {}, suggestions: [], changed: d.text !== d.baseline.text, writeBack: 'save', actions: ['update', 'discard', 'save-as', 'save'] });
      return {
        OpenDraft: async ref => {
          let d = Object.values(drafts).find(x => x.origin.path === ref.path);
          if (!d) {
            const text = '# ' + ref.path + '\n\nFirst line.\n';
            const id = 'k' + Object.keys(drafts).length.toString().padStart(31, '0');
            d = drafts[id] = { id, origin: { kind: 'local-folder', workspaceId: 'w1', sourceId: 'local', path: ref.path }, baseline: { text, revision: 'b1', title: ref.path }, title: ref.path, text, revision: 'r' + (++n) };
          }
          return view(d);
        },
        GetDraft: async id => view(get(id)), ListDrafts: async () => Object.values(drafts).map(view),
        UpdateDraft: async (id, expected, text) => { const d = get(id); d.text = text; d.revision = 'r' + (++n); return view(d); },
        PageDocumentOf: async (id, text) => ({ type: 'doc', attrs: { front: '', tail: '' }, content: [{ type: 'source_block', attrs: { uid: 1, gap: '', src: text, kind: 'text', html: '<p>Text</p>' } }] }),
        PageDocumentMarkdown: async doc => doc.content.map(b => (b.attrs.gap || '') + b.attrs.src).join(''),
        PreviewDraft: async id => ({ revision: get(id).revision, problems: [], blocks: [{ line: 1, html: '<p>Text</p>' }] }),
        CompareDraft: async id => ({ moved: false, revision: 'b1', title: get(id).title, theirs: '', mine: '', merged: get(id).text, conflicts: [] }),
      };
    },
    ready: () => window.dinahEditor && window.dinahWorkspaceBrowser && window.dinahWorkspaceBrowser.records().length === 1,
  });
  const { page: p, ok } = t;
  try {
    const open = async file => {
      const key = await p.evaluate(path => dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path }), file);
      await p.locator('[data-win="' + key + '"] .cm-content').waitFor();
      return key;
    };
    const a = await open('alpha.md');
    const b = await open('beta.md');
    const c = await open('gamma.md');
    const top = () => p.evaluate(() => pudlWindows.state().top);
    const text = key => p.locator('[data-win="' + key + '"] .cm-content').evaluate(el => CM.EditorView.findFromDOM(el).state.doc.toString());
    // PUDL's switcher (pudl-windows.js), which the layer turns on.
    const panel = p.locator('.win-switcher');
    const shown = () => panel.evaluate(el => el.matches(':popover-open'));
    const rows = () => panel.locator('li[data-switch-key]').evaluateAll(els => els.map(el => el.dataset.switchKey));
    const selected = () => panel.locator('li.active').getAttribute('data-switch-key');
    const flip = async (shift) => { await p.keyboard.down('Control'); await p.keyboard.press(shift ? 'Shift+Tab' : 'Tab'); await p.keyboard.up('Control'); };

    // The two most recent windows, from inside the Editor's text, which the
    // key must not reach.
    await p.locator('[data-win="' + c + '"] .cm-content').click();
    const before = await text(c);
    await flip();
    ok('a quick Ctrl+Tab goes to the window in front before this one', await top() === b);
    ok('Ctrl+Tab in the Editor\'s text changes nothing in the text', await text(c) === before);
    ok('the list does not show for a quick Ctrl+Tab', !await shown());
    await flip();
    ok('a second quick Ctrl+Tab flips back', await top() === c);
    await flip();
    await flip();
    ok('flipping goes on between the same two windows', await top() === c);

    // Holding Ctrl: each Tab goes one further, Shift+Tab one back, and the
    // list shows the windows not docked, most recent first.
    await p.keyboard.down('Control');
    await p.keyboard.press('Tab');
    await panel.waitFor({ state: 'visible' });
    const listed = await rows();
    ok('the list shows the undocked windows, most recent first', JSON.stringify(listed) === JSON.stringify([c, b, a, 'chat']));
    ok('docked windows are left out of the list', !listed.some(k => ['workspace', 'chat-browser', 'needs-you'].includes(k)));
    ok('the first Tab selects the previous window', await selected() === b);
    await p.keyboard.press('Tab');
    ok('a further Tab selects the next one down', await selected() === a);
    ok('the list shows each window\'s title and detail', (await panel.locator('li.active').textContent()).includes('alpha.md'));
    await t.shot('window-switcher');
    await p.keyboard.press('Tab');
    await p.keyboard.press('Shift+Tab');
    ok('Ctrl+Shift+Tab moves back', await selected() === a);
    ok('nothing changes while Ctrl is held', await top() === c);
    await p.keyboard.up('Control');
    ok('letting go of Ctrl brings the selected window to the front', await top() === a && !await shown());
    await flip();
    ok('the window left is now the most recent', await top() === c);

    // Escape while Ctrl is held.
    await p.keyboard.down('Control');
    await p.keyboard.press('Tab');
    await p.keyboard.press('Tab');
    await p.keyboard.press('Escape');
    ok('Escape takes the list away', !await shown());
    await p.keyboard.up('Control');
    ok('Escape changes no window', await top() === c);

    // A docked window in front: Ctrl+Tab goes to the most recent undocked one.
    await p.evaluate(() => pudlWindows.raise('needs-you'));
    await flip();
    ok('from a docked window, Ctrl+Tab goes to the most recent undocked window', await top() === c);

    // A minimised window is in the list, and comes back.
    await p.evaluate(key => pudlWindows.minimize(key), b);
    ok('the window is minimised', await p.evaluate(key => !!pudlWindows.state().min[key], b));
    await p.keyboard.down('Control');
    await p.keyboard.press('Tab');
    await panel.waitFor({ state: 'visible' });
    ok('a minimised window is in the list', (await rows()).includes(b));
    for (let i = 0; i < 4 && await selected() !== b; i++) await p.keyboard.press('Tab');
    await p.keyboard.up('Control');
    ok('choosing a minimised window restores it and brings it to the front', await top() === b && await p.evaluate(key => !pudlWindows.state().min[key], b));

    // Tab without Ctrl is left alone.
    const plainTab = await p.evaluate(() => new Promise(resolve => {
      const input = document.createElement('input'); document.body.appendChild(input); input.focus();
      window.addEventListener('keydown', function seen(e) { if (e.key !== 'Tab') return; window.removeEventListener('keydown', seen, true); input.remove(); resolve(e.defaultPrevented); }, true);
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }));
    }));
    ok('Tab without Ctrl in a text field is left alone', plainTab === false);

    // Ctrl+PageDown still goes through every window in the Window menu's order.
    const order = await p.evaluate(() => dinahOpenWindows().map(w => w.key));
    const from = await top();
    await p.keyboard.press('Control+PageDown');
    ok('Ctrl+PageDown goes to the next window in order, not the most recent', await top() === order[(order.indexOf(from) + 1) % order.length]);
    await p.keyboard.press('Control+PageUp');
    ok('Ctrl+PageUp goes back in order', await top() === from);

    // Help › Keyboard Shortcuts.
    await p.keyboard.press('F1');
    const help = p.locator('dialog.keyboard-help[open]');
    await help.waitFor();
    const helpText = await help.textContent();
    ok('Keyboard Shortcuts lists Ctrl+Tab and Ctrl+Shift+Tab', helpText.includes('Switch to the Last Window') && helpText.includes('Ctrl+Tab') && helpText.includes('Ctrl+Shift+Tab'));
    const helpRows = await help.locator('tbody tr').evaluateAll(trs => trs.map(tr => [...tr.cells].map(td => td.textContent)));
    const at = label => helpRows.findIndex(r => r[0] === label);
    ok('they are listed with the Window menu\'s commands, not after every menu', at('Switch to the Last Window') > at('Previous Window') && at('Switch to the Last Window') < at('Keyboard Shortcuts') && helpRows[at('Switch to the Last Window')][1] === 'Window');
    // Measured to the button, since PUDL 0.65.0's band of actions begins
    // above it.
    const gap = await help.evaluate(d => d.querySelector('.dialog-actions .btn').getBoundingClientRect().top - d.querySelector('.keyboard-help-table').getBoundingClientRect().bottom);
    ok('Close stands apart from the table above it', gap >= 16);
    await help.screenshot({ path: path.join(OUT, 'keyboard-help.png') });
    await p.keyboard.down('Control');
    await p.keyboard.press('Tab');
    await p.keyboard.up('Control');
    ok('Ctrl+Tab does nothing while a dialog is open', await top() === from && !await shown());
    await help.getByRole('button', { name: 'Close' }).click();
  } catch (err) {
    t.errors.push(String(err && err.stack || err));
  } finally {
    await t.close();
  }
})();
