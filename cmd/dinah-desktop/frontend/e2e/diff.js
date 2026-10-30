// The Editor's diff view: a comparison opened with fixture texts, drawn
// inline and side by side with line numbers and a sign column; hunk, line
// and file actions reaching the opener with the right ranges; ticked lines
// enabling the line button; a second open with the same key reusing the
// window; the theme following the application's; screenshots.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1600, height: 1000 },
    app: () => {
      const state = () => ({ workspaceId: 'w1', workspace: 'C:/Fixture', workspaces: ['C:/Fixture'], recent: ['C:/Fixture'], windowModes: { diff: 'maximized' }, windowModesShipped: { diff: 'maximized' },
        workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [{ id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] });
      return {
        State: async () => state(),
        // The workspace's main folder has cmd/main.go and nothing else.
        ResolveWorkspaceResourceMount: async ref => { if (ref.path === 'cmd/main.go') return { path: '/workspaces/Fixture/Fixture/cmd/main.go', kind: 'file' }; throw new Error('not mounted'); }
      };
    },
    init: () => {
      window.__actions = [];
      window.__before = 'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("hello")\n\told := 1\n\tfmt.Println(old)\n}\n\nfunc helper() int {\n\treturn 1\n}\n\n// trailing comment\n';
      window.__after = 'package main\n\nimport (\n\t"fmt"\n\t"os"\n)\n\nfunc main() {\n\tfmt.Println("hello, world")\n\tfmt.Println(os.Args)\n}\n\nfunc helper() int {\n\treturn 1\n}\n\nfunc extra() {}\n\n// trailing comment\n';
      window.__open = (over) => window.dinahDiff.open(Object.assign({
        key: 'C:/Fixture|cmd/main.go|unstaged', title: 'main.go', detail: 'cmd · Modified, unstaged', path: 'cmd/main.go', language: 'main.go',
        before: window.__before, after: window.__after, mode: 'inline', readOnly: true,
        actions: [
          { id: 'stage-file', label: 'Stage File', glyph: 'plus', scope: 'file', primary: true },
          { id: 'discard-file', label: 'Discard…', glyph: 'close', scope: 'file', danger: true, confirm: { title: 'Discard the changes?', body: 'The changes to main.go will be lost.', yes: 'Discard' } },
          { id: 'stage-hunk', label: 'Stage Hunk', scope: 'hunk' },
          { id: 'stage', label: 'Stage', scope: 'line', primary: true },
          { id: 'discard', label: 'Discard', scope: 'line', danger: true }
        ],
        onAction: (id, where) => { window.__actions.push({ id, where }); }
      }, over || {}));
    },
    ready: () => window.dinahDiff && window.dinahWorkspaceBrowser && window.dinahWorkspaceBrowser.records().length === 1,
  });
  const { page: p } = t;
  const ok = (name, passed, detail) => t.ok(name, passed, detail === undefined ? undefined : JSON.stringify(detail));
  p.on('console', m => { if (m.type() === 'error') console.log('console: ' + m.text()); });
  try {
    // A diff opens in its own window, inline, read only, with the file's
    // path and the count of hunks under the toolbar.
    const key = await p.evaluate(() => window.__open());
    const win = p.locator('.win[data-win-kind="diff"]').first();
    await win.locator('.diff-body[data-mode="inline"] .cm-content').waitFor();
    await win.locator('.diff-hunk').first().waitFor();
    ok('open resolves with the window key', typeof key === 'string' && await p.evaluate(k => pudlWindows.state().open.includes(k), key), key);
    ok('the window has the title and detail it was given', (await win.locator('.win-title').textContent()).trim() === 'main.go' && await win.getAttribute('data-win-detail') === 'cmd · Modified, unstaged');
    const meta = await win.locator('[data-df-meta]').textContent();
    const hunkCount = await win.locator('.diff-hunk').count();
    ok('the meta line says the path, the detail and the hunks', meta.startsWith('cmd/main.go · cmd · Modified, unstaged · ') && /\d+ hunks? · \+\d+ −\d+/.test(meta) && hunkCount >= 2, { meta, hunkCount });
    ok('the text is read only', await win.locator('.cm-content[contenteditable="false"]').count() === 1);
    ok('the text is coloured as Go', await win.locator('.cm-content .ͼ1, .cm-content [class*="ͼ"]').count() > 0);

    // Line numbers: an unchanged line shows its number on both sides, an
    // added line only the after number with a plus, and a block of removed
    // lines the before numbers with a minus each.
    // CodeMirror keeps a hidden spacer element in each gutter for its width;
    // the rows are those of the elements that are shown.
    await p.evaluate(() => { window.__rows = sel => Array.from(document.querySelectorAll(sel + ' .cm-gutterElement')).filter(e => getComputedStyle(e).visibility !== 'hidden').flatMap(e => Array.from(e.querySelectorAll('.diff-row'), r => r.textContent)); });
    const numbers = await win.locator('.diff-gutter-number').evaluateAll(gs => gs.map(g => Array.from(g.querySelectorAll('.cm-gutterElement')).filter(e => getComputedStyle(e).visibility !== 'hidden').flatMap(e => Array.from(e.querySelectorAll('.diff-row'), r => r.textContent))));
    ok('the inline view has two number gutters, before and after', numbers.length === 2 && numbers[0][0] === '1' && numbers[1][0] === '1', numbers.map(n => n.slice(0, 6)));
    const firstHunk = await win.locator('.diff-hunk').first().locator('code').textContent();
    const m = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@$/.exec(firstHunk);
    ok('a hunk strip has a unified diff header', !!m, firstHunk);
    const marks = await p.evaluate(() => window.__rows('.win[data-win-kind="diff"] .diff-gutter-mark'));
    ok('the sign column has pluses and minuses', marks.includes('+') && marks.includes('-'), marks.slice(0, 20));
    const added = await win.locator('.cm-content .cm-changedLine').count(), removed = await win.locator('.cm-content .cm-deletedChunk .cm-deletedLine').count();
    const summary = /\+(\d+) −(\d+)/.exec(meta);
    ok('added and removed lines are drawn, and counted in the summary', added === +summary[1] && removed === +summary[2], { added, removed, summary: summary[0] });
    const tints = await win.locator('.cm-content .cm-changedLine').first().evaluate(el => getComputedStyle(el).backgroundColor);
    ok('an added line is tinted', tints !== 'rgba(0, 0, 0, 0)' && tints !== 'transparent', tints);
    // PUDL's diff tokens (0.93.0): the lines and gutters take --diff-add-bg
    // and --diff-del-bg, every changed line's sign is PUDL's plus or minus
    // glyph in --diff-add or --diff-del, and changed words are underlined.
    const tokens = await p.evaluate(() => {
      const probe = (value) => { const d = document.createElement('div'); d.style.background = value; document.body.append(d); const c = getComputedStyle(d).backgroundColor; d.remove(); return c; };
      const win = document.querySelector('.win[data-win-kind="diff"]');
      const line = win.querySelector('.cm-content .cm-changedLine'), chunk = win.querySelector('.cm-deletedChunk');
      const sign = kind => { const row = win.querySelector('.diff-gutter-mark .' + kind + ' .diff-row'), s = getComputedStyle(row, '::before'); return { mask: s.maskImage || s.webkitMaskImage, fill: s.backgroundColor, text: getComputedStyle(row).color }; };
      const word = win.querySelector('.cm-changedText');
      return { add: getComputedStyle(line).backgroundColor === probe('var(--diff-add-bg)'), del: getComputedStyle(chunk).backgroundColor === probe('var(--diff-del-bg)'),
        plus: sign('diff-add'), minus: sign('diff-del'), addColour: probe('var(--diff-add)'), delColour: probe('var(--diff-del)'),
        word: word ? getComputedStyle(word).textDecorationLine : 'none', tint: dinahAppearance.diffTints(dinahAppearance.document('mineral').palettes.light) };
    });
    ok('added and removed lines take --diff-add-bg and --diff-del-bg', tokens.add && tokens.del, tokens);
    ok('each changed line shows PUDL\'s plus or minus glyph in --diff-add or --diff-del', /data:image\/svg/.test(tokens.plus.mask) && /data:image\/svg/.test(tokens.minus.mask) && tokens.plus.mask !== tokens.minus.mask && tokens.plus.fill === tokens.addColour && tokens.minus.fill === tokens.delColour && tokens.plus.text === 'rgba(0, 0, 0, 0)', tokens);
    ok('changed words are underlined', tokens.word === 'underline', tokens.word);
    ok('Mineral keeps PUDL\'s 10 percent tints', tokens.tint.add === 10 && tokens.tint.del === 10, tokens.tint);
    // Each theme's tints are the strongest amount up to 10 percent at which
    // its syntax colours keep 4.5:1 and its signs 3:1 (theming.js checks
    // the amount each built-in palette reaches).
    ok('every built-in theme\'s tints keep its syntax colours at 4.5:1 and its signs at 3:1', await p.evaluate(() => {
      const rgb = h => [1, 3, 5].map(i => parseInt(h.slice(i, i + 2), 16) / 255), lin = c => c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4;
      const lum = c => .2126 * lin(c[0]) + .7152 * lin(c[1]) + .0722 * lin(c[2]), ratio = (a, b) => { const x = lum(a), y = lum(b); return (Math.max(x, y) + .05) / (Math.min(x, y) + .05); };
      return Object.values(dinahAppearance.builtins).every(doc => ['light', 'dark'].every(m => {
        const p = doc.palettes[m], t = dinahAppearance.diffTints(p), lit = parseFloat(p.lit) / 100, input = rgb(p.light).map((v, i) => v * lit + rgb(p.surface)[i] * (1 - lit));
        return [[p.positive, t.add], [p.danger, t.del]].every(([colour, percent]) => {
          const c = rgb(colour), tint = c.map((v, i) => v * percent / 100 + input[i] * (1 - percent / 100));
          return percent >= 0 && percent <= 10 && ratio(c, tint) >= 3 && Object.keys(p).filter(k => k.startsWith('syntax-')).every(k => ratio(rgb(p[k]), tint) >= 4.5);
        });
      }));
    }));
    await p.emulateMedia({ forcedColors: 'active' });
    const forced = await p.evaluate(() => {
      const win = document.querySelector('.win[data-win-kind="diff"]'), line = win.querySelector('.cm-content .cm-changedLine'), chunk = win.querySelector('.cm-deletedChunk');
      return { add: getComputedStyle(line).borderInlineStartStyle, del: getComputedStyle(chunk).borderInlineStartStyle };
    });
    ok('in forced colours added lines have a solid edge and removed ones a dashed edge', forced.add === 'solid' && forced.del === 'dashed', forced);
    await p.emulateMedia({ forcedColors: 'none' });
    ok('the gutters are not selectable, so copying copies text alone', await win.locator('.cm-gutters').first().evaluate(el => getComputedStyle(el).userSelect) === 'none' &&
      await win.locator('.diff-hunk').first().evaluate(el => getComputedStyle(el).userSelect) === 'none');
    await t.shot('diff-inline');

    // The head bar: the layout, the file's actions with one primary, and
    // Edit the File for a file of the workspace.
    const cmd = require('./lib/commands');
    ok('the layout control offers Inline and Side by Side', await win.locator('[data-command="diff-inline"][aria-pressed="true"]').count() === 1 && await win.locator('[data-command="diff-split"][aria-pressed="false"]').count() === 1);
    ok('the file actions are in the head bar, the first primary', await cmd.shown(win, 'action-stage-file') === 'Stage File' && await win.locator('[data-command="action-stage-file"].btn-primary').count() === 1 &&
      await win.locator('[data-command="action-discard-file"].btn-danger').count() === 1);
    ok('Edit the File is offered for a file of the workspace', await cmd.shown(win, 'edit-file') === 'Edit the File' && await win.locator('[data-command="edit-file"]:not([hidden])').count() === 1);

    // A hunk action comes back with the hunk's index and its lines on both
    // sides, as the strip's header gives them.
    await win.locator('.diff-hunk[data-hunk="0"] [data-hunk-action="stage-hunk"]').click();
    let actions = await p.evaluate(() => window.__actions);
    const h = actions[0] && actions[0].where;
    ok('a hunk action reaches the opener with the hunk and its ranges', actions.length === 1 && actions[0].id === 'stage-hunk' && h.hunk === 0 &&
      h.before.line === +m[1] && h.before.count === +(m[2] || 1) && h.after.line === +m[3] && h.after.count === +(m[4] || 1), actions);

    // The line button is disabled until lines are ticked, then says how
    // many, and runs once for each hunk with ticked lines.
    const stageLines = win.locator('[data-line-action="stage"]');
    ok('the line action waits for ticked lines', await stageLines.isDisabled() && (await stageLines.textContent()) === 'Stage lines' && (await win.locator('[data-df-hint]').textContent()).includes('Tick lines'));
    const ticks = win.locator('.diff-tick');
    const tickCount = await ticks.count();
    ok('every changed line has a tick box', tickCount === added + removed, { tickCount, added, removed });
    await ticks.nth(0).click();
    await ticks.nth(1).click();
    await p.waitForFunction(() => document.querySelectorAll('.diff-tick:checked').length === 2);
    ok('ticking lines enables the line button and counts them', !(await stageLines.isDisabled()) && (await stageLines.textContent()) === 'Stage 2 lines' && await stageLines.evaluate(b => b.classList.contains('btn-primary')));
    const tickedLabels = await p.evaluate(() => Array.from(document.querySelectorAll('.diff-tick:checked'), t => t.getAttribute('aria-label')));
    await p.evaluate(() => { window.__actions = []; });
    await stageLines.click();
    await p.waitForFunction(() => window.__actions.length > 0);
    actions = await p.evaluate(() => window.__actions);
    const lines = actions.flatMap(a => a.where.lines);
    const expected = tickedLabels.map(l => { const mm = /Choose (removed|added) line (\d+)/.exec(l); return { side: mm[1] === 'removed' ? 'before' : 'after', line: +mm[2] }; });
    ok('a line action reaches the opener with the ticked lines, by side and number, once per hunk', actions.every(a => a.id === 'stage' && typeof a.where.hunk === 'number' && a.where.lines.length) &&
      lines.length === 2 && expected.every(e => lines.some(l => l.side === e.side && l.line === e.line)), { actions, expected });
    ok('the ticks are cleared after the action', await p.evaluate(() => document.querySelectorAll('.diff-tick:checked').length === 0) && await stageLines.isDisabled());

    // A file action comes back with nothing; a danger action with a
    // confirmation asks first in Dinah's own dialog.
    await p.evaluate(() => { window.__actions = []; });
    await cmd.press(p, win, 'action-stage-file');
    await p.waitForFunction(() => window.__actions.length === 1);
    actions = await p.evaluate(() => window.__actions);
    ok('a file action reaches the opener with no range', actions[0].id === 'stage-file' && actions[0].where === undefined, actions);
    await cmd.press(p, win, 'action-discard-file');
    await p.locator('#ask[open]').waitFor();
    ok('a danger action with a confirmation asks first', (await p.locator('#ask-title').textContent()) === 'Discard the changes?' && await p.locator('#ask-yes.btn-danger').count() === 1);
    await p.locator('#ask-no').click();
    await p.waitForTimeout(100);
    ok('cancelling the question runs nothing', (await p.evaluate(() => window.__actions.length)) === 1);

    // Side by side: two editors, the before side with minuses, the after
    // side with pluses, the strips on both sides, and the layout kept in
    // the window's state.
    await win.locator('[data-command="diff-split"]').click();
    await win.locator('.diff-body[data-mode="split"] .cm-mergeView').waitFor();
    await win.locator('.cm-merge-b .diff-hunk').first().waitFor();
    const sideMarks = await p.evaluate(() => ({
      a: window.__rows('.cm-merge-a .diff-gutter-mark'),
      b: window.__rows('.cm-merge-b .diff-gutter-mark'),
      aNumbers: document.querySelectorAll('.cm-merge-a .diff-gutter-number').length, bNumbers: document.querySelectorAll('.cm-merge-b .diff-gutter-number').length,
      strips: [document.querySelectorAll('.cm-merge-a .diff-hunk').length, document.querySelectorAll('.cm-merge-b .diff-hunk').length],
      hunkButtons: [document.querySelectorAll('.cm-merge-a [data-hunk-action]').length, document.querySelectorAll('.cm-merge-b [data-hunk-action]').length]
    }));
    ok('side by side draws the before side with minuses and the after side with pluses', sideMarks.a.includes('-') && !sideMarks.a.includes('+') && sideMarks.b.includes('+') && !sideMarks.b.includes('-') && sideMarks.aNumbers === 1 && sideMarks.bNumbers === 1, sideMarks);
    ok('each hunk has a strip on both sides, with the actions on the after side', sideMarks.strips[0] === hunkCount && sideMarks.strips[1] === hunkCount && sideMarks.hunkButtons[0] === 0 && sideMarks.hunkButtons[1] === hunkCount, sideMarks);
    const aligned = await p.evaluate(() => {
      const a = document.querySelector('.cm-merge-a .diff-hunk[data-hunk="1"]'), b = document.querySelector('.cm-merge-b .diff-hunk[data-hunk="1"]');
      return a && b ? Math.abs(a.getBoundingClientRect().top - b.getBoundingClientRect().top) : -1;
    });
    ok('the sides stay aligned at a hunk', aligned >= 0 && aligned < 2, aligned);
    ok('the layout is in the window state', await p.evaluate(k => (new URLSearchParams(location.search).get('view.' + k) || '').startsWith('split'), key));
    await t.shot('diff-split');
    await win.locator('.cm-merge-a .diff-tick').first().click();
    ok('a tick on the before side is a removed line', (await stageLines.textContent()) === 'Stage 1 line' && await p.evaluate(() => document.querySelector('.cm-merge-a .diff-tick:checked').getAttribute('aria-label').startsWith('Choose removed line')));

    // A second open with the same key reuses the window and shows the new
    // texts; a different key opens another window.
    await p.evaluate(() => window.__open({ after: window.__after.replace('func extra() {}\n\n', ''), detail: 'cmd · Modified, unstaged, again' }));
    await p.waitForFunction(() => (document.querySelector('.win[data-win-kind="diff"] [data-df-meta]') || {}).textContent.includes('again'));
    const diffWindows = await p.evaluate(() => pudlWindows.state().open.filter(k => (dinahWorkbench.item(k) || {}).kind === 'diff').length);
    const meta2 = await win.locator('[data-df-meta]').textContent();
    ok('a second open with the same key reuses the window and updates it', diffWindows === 1 && /\+(\d+) −(\d+)/.exec(meta2)[1] < +summary[1] && (await win.locator('.diff-body[data-mode="split"]').count()) === 1 && (await p.evaluate(() => document.querySelectorAll('.diff-tick:checked').length)) === 0, { diffWindows, meta2 });
    await p.evaluate(() => window.__open({ key: 'C:/Fixture|docs/notes.md|staged', title: 'notes.md', path: 'docs/notes.md', detail: 'docs · Added, staged', before: '', after: '# Notes\n\nOne line.\n', actions: [] }));
    await p.waitForFunction(() => pudlWindows.state().open.filter(k => (dinahWorkbench.item(k) || {}).kind === 'diff').length === 2);
    const second = p.locator('.win[data-win-kind="diff"]').filter({ hasText: 'docs · Added, staged' });
    await second.locator('.diff-hunk').first().waitFor();
    ok('a diff with no actions has no tick boxes, no hunk buttons and no line button', await second.locator('.diff-tick').count() === 0 && await second.locator('[data-hunk-action]').count() === 0 && await second.locator('[data-line-action]').count() === 0);
    ok('Edit the File is not offered for a path that is not a workspace file', await second.locator('[data-command="edit-file"][hidden]').count() === 1);
    ok('a new file is one hunk of added lines', (await second.locator('[data-df-meta]').textContent()).includes('1 hunk · +3 −0'));

    // The theme: the text's colours follow the application's theme.
    const light = await win.locator('.cm-editor').first().evaluate(el => getComputedStyle(el).backgroundColor);
    await p.evaluate(() => window.pudlSetTheme('dark'));
    await p.waitForTimeout(200);
    const dark = await win.locator('.cm-editor').first().evaluate(el => getComputedStyle(el).backgroundColor);
    ok('the theme switches with the application', light !== dark, { light, dark });
    await t.shot('diff-dark');
    await p.evaluate(() => window.pudlSetTheme('light'));

    // Next Change and Previous Change move through the hunks with F7. The
    // first window is brought back in front of the second.
    await p.evaluate(k => pudlWindows.raise(k), key);
    await win.locator('[data-command="diff-inline"]').click();
    await win.locator('.diff-body[data-mode="inline"] .cm-content').waitFor();
    await cmd.press(p, win, 'next-change');
    await cmd.press(p, win, 'next-change');
    const at = await win.locator('.diff-body').evaluate(body => ({ inside: body.contains(document.activeElement), active: document.activeElement && document.activeElement.className }));
    ok('Next Change moves the cursor into the text', at.inside === true, at);
  } catch (err) {
    t.errors.push(String(err && err.stack || err));
  } finally {
    await t.close();
  }
})();
