// A taskbar stack's rows give the window's title the row: one line from the
// glyph to the close button, ending in an ellipsis, with no column for the
// workspace the stack button already shows. A detail that says something
// else follows the title, muted, in the same line.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const records = [
    { id: 'ws_ALPHA', name: 'Alpha', executionPath: 'C:/Alpha', mountAlias: 'alpha', primarySourceId: 'root-a', sources: [{ id: 'root-a', kind: 'local-folder', path: 'C:/Alpha' }] },
    { id: 'ws_BETA', name: 'Beta', executionPath: 'C:/Beta', mountAlias: 'beta', primarySourceId: 'root-b', sources: [{ id: 'root-b', kind: 'local-folder', path: 'C:/Beta' }] }
  ];
  const t = await launch({
    viewport: { width: 1400, height: 500 },
    args: { records },
    app: ({ records }) => {
      const state = async () => ({ workspace: 'C:/Alpha', workspaceId: 'ws_ALPHA', workspaceRecords: records, recent: records.map(r => r.executionPath), workspaces: records.map(r => r.executionPath), defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [] });
      return {
        State: state, Sessions: async () => [], SessionsAt: async () => [],
        GetDraft: async () => new Promise(() => {}),
        StatWorkspaceMount: async path => ({ path, directory: true, actions: ['list'] }),
        ReadUIState: async () => ({ version: 1, values: {} }), SaveUIState: async () => {},
      };
    },
    goto: false,
  });
  const { page, ok } = t;
  const titles = [
    ['release-notes-for-the-november-desktop-build-with-hyphenated-words.md', 'Alpha'],
    ['a.md', 'Alpha'],
    ['Untitled', 'Alpha'],
    ['notes-from-another-workspace.md', 'Beta'],
  ];
  for (let i = 0; i < 12; i++) titles.push(['scroll-filler-' + i + '.txt', 'Alpha']);
  try {
    await page.goto(PAGE);
    await t.waitReady();
    await page.locator('[data-win-layer]').evaluate(layer => layer.setAttribute('data-win-taskbar-stacks', 'always'));
    // Plain item windows of a kind the suite registers: a content window
    // keeps its title as Dinah sets it and ignores a retitle from elsewhere
    // (workbench.js), and the rows here are PUDL's to lay out.
    await page.evaluate(() => dinahWorkbench.register('fixture-doc', { title: 'Doc', glyph: 'document', applet: { id: 'fixture.docs', title: 'Docs', glyph: 'document', workspace: true }, init: function () { return { destroy: function () {} }; } }));
    const keys = [];
    for (let i = 0; i < titles.length; i++) {
      keys.push(await page.evaluate(i => dinahWorkbench.open({ kind: 'fixture-doc', workspace: 'C:/Alpha', path: 'doc-' + i }), i));
    }
    await page.waitForFunction(keys => keys.every(k => { const w = document.querySelector('.win[data-win="' + CSS.escape(k) + '"]'); return w && w.getAttribute('data-dinah-group-workspace') === 'Alpha'; }), keys);
    await page.evaluate(([keys, titles]) => keys.forEach((k, i) => pudlWindows.retitle(k, titles[i][0], titles[i][1])), [keys, titles]);
    const group = await page.evaluate(k => document.querySelector('.win[data-win="' + CSS.escape(k) + '"]').getAttribute('data-win-group'), keys[0]);
    const stack = page.locator('nav.win-dock > [data-win-stack="' + group + '"]');

    for (const [width, name] of [[1400, 'normal'], [420, 'narrow']]) {
      await page.setViewportSize({ width, height: 500 });
      await page.keyboard.press('Escape');
      await stack.click();
      const panel = page.locator('.win-stack-panel');
      await panel.waitFor();
      await page.waitForTimeout(150);
      const rows = await panel.evaluate(p => [...p.querySelectorAll('.win-stack-row')].map(row => {
        const link = row.querySelector('.win-stack-link'), title = row.querySelector('.win-item-title'), detail = row.querySelector('.win-item-detail'), close = row.querySelector('.win-stack-close'), glyph = row.querySelector('.win-item-glyph'), state = row.querySelector('.win-item-state');
        const tr = title.getBoundingClientRect(), dr = detail.getBoundingClientRect(), cr = close.getBoundingClientRect(), gr = glyph.getBoundingClientRect(), sr = state.getBoundingClientRect();
        // PUDL 0.96.5 (a2385ab) keeps a state column before Close, sized to
        // the widest badge in the list; the title's room ends where it starts.
        return { text: title.textContent, lines: Math.round(tr.height / parseFloat(getComputedStyle(title).lineHeight || 16)), titleH: tr.height, linkH: link.getBoundingClientRect().height,
          detailShown: dr.width > 0, detailRight: dr.right, titleRight: tr.right, titleW: tr.width, natural: title.scrollWidth, space: Math.min(sr.left, cr.left) - gr.right, stateW: sr.width, closeGap: cr.left - gr.right, label: link.getAttribute('aria-label'), tip: link.getAttribute('title') };
      }));
      const byText = s => rows.find(r => r.text === s);
      const long = byText(titles[0][0]), short = byText('a.md'), changed = byText('Untitled'), other = byText(titles[3][0]);
      ok(name + ': every title is one line', rows.every(r => r.titleH < 30), JSON.stringify(rows.map(r => r.titleH)));
      ok(name + ': the redundant workspace is not shown', !long.detailShown && !short.detailShown && !changed.detailShown);
      ok(name + ': with no window in a state, the state column takes no width', rows.every(r => r.stateW < 1), JSON.stringify(rows.map(r => r.stateW)));
      ok(name + ': a long title takes the room from glyph to close button', long.titleW >= long.space - 24, JSON.stringify({ w: long.titleW, space: long.space, closeGap: long.closeGap }));
      // A document's title no longer carries a mark for unsaved typing (9 November 2026); a short title is wholly visible.
      ok(name + ': a short title is wholly visible', changed.natural <= changed.titleW + 1);
      ok(name + ': the other workspace follows the title in the same line', other.detailShown || other.natural > other.titleW, JSON.stringify(other));
      ok(name + ': the full title stays in the tooltip and spoken name', long.label.includes(titles[0][0]) && long.tip.includes(titles[0][0]) && other.label.includes('Beta'));
      const scroll = await panel.evaluate(p => ({ sh: p.scrollHeight, ch: p.clientHeight, overflow: getComputedStyle(p).overflowY }));
      ok(name + ': a long stack scrolls', scroll.sh > scroll.ch && scroll.overflow === 'auto', JSON.stringify(scroll));
      await t.shot('stack-row-titles-' + name);
      await page.locator('.win-stack-link').filter({ hasText: 'a.md' }).focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(k => pudlWindows.state().top === k, keys[1]);
      ok(name + ': Enter on a row brings its window forward', true);
      await page.keyboard.press('Escape');
    }
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
