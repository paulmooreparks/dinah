// The Workspaces pane's toolbar is a folding PUDL toolbar at the top of the
// pane: Actions at the far left, whose menu holds New Workspace… and the
// other workspace commands, then Add Source…, and New Chat at the far right
// as the row's end group, each with its glyph. New Workspace… has no button
// of its own, and the Chats and Sources headings have none. The order on
// screen is the order in the page and for the keyboard. Each command does
// what it did, for the workspace chosen; Add Source… is offered only where
// Sources is. The toolbar keeps its words while there is room, New Chat's
// longest, and folds rather than wraps.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 2400, height: 900 },
    app: () => {
      window.__calls = [];
      const records = [
        { id: 'w1', name: 'Delivery', mountAlias: 'w1', primarySourceId: 'p1', executionPath: 'C:/Working', sources: [{ id: 'p1', mountAlias: 'p1', kind: 'local-folder', name: 'Working Files', path: 'C:/Working' }] },
        { id: 'w2', name: 'Other Work', mountAlias: 'w2', primarySourceId: 'p2', executionPath: 'C:/Other', sources: [{ id: 'p2', mountAlias: 'p2', kind: 'local-folder', name: 'Other Folder', path: 'C:/Other' }] },
      ];
      const state = () => ({ workspace: 'C:/Other', workspaceId: 'w2', workspaceRecords: records, workspaces: ['C:/Working', 'C:/Other'], recent: ['C:/Working', 'C:/Other'], defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [] });
      return {
        State: async () => state(),
        ListWorkspaceMounts: async () => ({ entries: [], nextStart: -1 }),
        ChooseWorkspaceSource: async id => { window.__calls.push(['source', id]); return null; },
        ChooseWorkspace: async () => { window.__calls.push(['workspace']); return state(); },
      };
    },
  });
  const { page: p, ok } = t;
  const bar = p.locator('.ws-commands');
  const read = () => p.evaluate(() => {
    const b = document.querySelector('.ws-commands'), r = b.getBoundingClientRect();
    const buttons = [...b.querySelectorAll('.toolbar-group > button, .toolbar-group > .menu:not(.toolbar-fold) > .menu-btn')];
    return {
      width: Math.round(r.width), height: Math.round(r.height), overflow: b.scrollWidth > b.clientWidth + 1,
      stages: [...b.querySelectorAll(':scope > .toolbar-group')].map(g => g.getAttribute('aria-label') + ':' + g.getAttribute('data-toolbar-stage')), more: b.hasAttribute('data-toolbar-more'),
      shown: buttons.filter(x => x.getClientRects().length).map(x => ({ words: x.querySelector('.toolbar-words').textContent, wordsShown: !!x.querySelector('.toolbar-words').getClientRects().length, glyph: x.querySelector('.glyph').style.getPropertyValue('--glyph').trim(), name: x.getAttribute('aria-label') || x.textContent.trim(), left: Math.round(x.getBoundingClientRect().left), right: Math.round(x.getBoundingClientRect().right) })),
      barRight: Math.round(document.getElementById('ws-select').getBoundingClientRect().right), barLeft: Math.round(document.getElementById('ws-select').getBoundingClientRect().left),
      folds: [...b.querySelectorAll('.toolbar-fold > .menu-btn, .toolbar-more > .menu-btn')].filter(x => x.getClientRects().length).map(x => x.getAttribute('aria-label')),
      standalone: b.querySelectorAll('.toolbar-group > [data-cmd="new-workspace"]').length,
    };
  });
  const fitted = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
  const leftToRight = s => s.shown.every((x, i) => i === 0 || x.left >= s.shown[i - 1].right);
  try {
    await bar.waitFor();
    await p.locator('#workspace-source-choose:not([hidden])').waitFor();
    await fitted();
    const wide = await read();
    ok('the toolbar is a PUDL toolbar at the top of the pane', await bar.getAttribute('role') === 'toolbar' && await p.evaluate(() => document.querySelector('.ws-toolbar').compareDocumentPosition(document.getElementById('workspace-pins-group')) & Node.DOCUMENT_POSITION_FOLLOWING));
    ok('wide, it shows Actions, Add Source… and New Chat in that order, each with its glyph and words, on one row',
      wide.shown.map(x => x.words).join('|') === 'Actions|Add Source…|New Chat' && wide.shown.every(x => x.wordsShown) && leftToRight(wide) && wide.height < 40 && !wide.overflow, JSON.stringify(wide));
    // PUDL pads each toolbar group by 8px at its ends, so New Chat sits that far in.
    ok('Actions is at the far left and New Chat at the far right', wide.shown[0].left - wide.barLeft < 4 && wide.barRight - wide.shown[2].right <= 9, JSON.stringify(wide));
    ok('New Workspace… has no button of its own in the toolbar', wide.standalone === 0);
    ok('the glyphs match the same commands elsewhere: menu, plus and comments', wide.shown.map(x => x.glyph.replace(/var\(--glyph-|\)/g, '')).join('|') === 'menu|plus|comments', JSON.stringify(wide.shown.map(x => x.glyph)));
    ok('the Chats and Sources headings have no buttons', await p.locator('[data-section="workspace-chats"] .browser-group-head .btn, [data-section="workspace-sources"] .browser-group-head .btn').count() === 0);
    await t.shot('workspace-toolbar-wide', { clip: await p.locator('#sidebar').boundingBox() });
    // Keyboard: one Tab stop, the arrow keys move along it in screen order.
    await p.locator('#ws-select').focus();
    await p.keyboard.press('Tab');
    const first = await p.evaluate(() => document.activeElement.id);
    await p.keyboard.press('ArrowRight');
    const second = await p.evaluate(() => document.activeElement.id);
    await p.keyboard.press('ArrowRight');
    const third = await p.evaluate(() => document.activeElement.dataset.cmd);
    ok('Tab reaches Actions, and Right moves to Add Source… and on to New Chat', first === 'ws-actions-button' && second === 'workspace-source-choose' && third === 'new-session', JSON.stringify({ first, second, third }));
    // Each command does what it did, for the workspace chosen.
    await p.keyboard.press('Enter');
    const ask = p.locator('dialog.dinah-new-chat-workspace[open]');
    await ask.waitFor();
    const chosen = await ask.locator('#new-chat-workspace').evaluate(s => s.selectedOptions[0].textContent);
    ok('New Chat asks for its workspace with the chosen one, Other Work, selected', /^Other Work/.test(chosen), chosen);
    await ask.getByRole('button', { name: 'Cancel' }).click();
    await ask.waitFor({ state: 'detached' });
    await p.locator('#workspace-source-choose').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'source'));
    ok('Add Source… asks to add a folder to the chosen workspace', await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'source'))) === '["source","w2"]');
    await p.locator('#ws-actions-button').click();
    await p.locator('#ws-actions:popover-open').waitFor();
    const rows = await p.locator('#ws-actions .menu-action').allTextContents();
    ok('Actions holds New Workspace… first, then the other workspace commands', rows.join('|') === 'New Workspace…|New Empty Workspace|Rename Workspace…|Export Workspace…|Import Workspace…|Refresh Workspace|Remove Workspace…', rows.join('|'));
    await p.locator('#ws-actions [data-cmd="new-workspace"]').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'workspace'));
    ok('New Workspace… in Actions asks for a main folder as before', true);
    // The default width: the left group's words go first, New Chat keeps its own.
    await p.setViewportSize({ width: 1400, height: 900 });
    await p.waitForFunction(() => document.querySelector('.ws-commands').getBoundingClientRect().width < 400);
    await fitted();
    const normal = await read();
    const chat = normal.shown.find(x => x.words === 'New Chat');
    ok('at the usual width it keeps one row in the same order, New Chat at the right with its words', normal.height < 40 && !normal.overflow && normal.shown.map(x => x.words).join('|') === 'Actions|Add Source…|New Chat' && leftToRight(normal) && chat.wordsShown && normal.barRight - chat.right <= 9 && normal.shown.every(x => x.name), JSON.stringify(normal));
    await t.shot('workspace-toolbar-normal', { clip: await p.locator('#sidebar').boundingBox() });
    // The narrowest docked pane.
    await p.setViewportSize({ width: 1000, height: 900 });
    await p.waitForFunction(() => document.querySelector('.ws-commands').getBoundingClientRect().width < 260);
    await fitted();
    const docked = await read();
    ok('at the narrowest docked width it keeps one row in the same order without overflowing', docked.height < 40 && !docked.overflow && leftToRight(docked), JSON.stringify(docked));
    await t.shot('workspace-toolbar-docked-narrow', { clip: await p.locator('#sidebar').boundingBox() });
    // Narrower still, as a window can be made: it folds, never wraps or
    // overflows, and New Workspace… is still reached through Actions.
    await p.evaluate(() => { document.getElementById('sidebar').style.maxWidth = '150px'; });
    await p.waitForFunction(() => document.querySelector('.ws-commands').getBoundingClientRect().width < 140);
    await fitted();
    const narrow = await read();
    ok('narrower, it folds into menus on one row without overflowing', narrow.height < 40 && !narrow.overflow && narrow.folds.length > 0, JSON.stringify(narrow));
    const fold = p.locator('.ws-commands .toolbar-fold > .menu-btn:visible, .ws-commands .toolbar-more > .menu-btn:visible').first();
    await fold.click();
    const folded = await p.evaluate(() => [...document.querySelectorAll('.ws-commands .menu-panel:popover-open .menu-action')].map(r => r.textContent.trim()));
    ok('the folded menu lists Actions, Add Source… and New Chat in order', /Actions.*Add Source….*New Chat/.test(folded.join('|')), JSON.stringify(folded));
    const actionsRow = p.locator('.ws-commands .menu-panel:popover-open .menu-action').filter({ hasText: /^Actions/ }).first();
    await actionsRow.click();
    const inner = await p.evaluate(() => [...document.querySelectorAll('.menu-panel:popover-open .menu-action')].map(r => r.textContent.trim()));
    ok('New Workspace… is still reached through Actions when folded', inner.includes('New Workspace…'), JSON.stringify(inner));
    await t.shot('workspace-toolbar-narrow', { clip: await p.locator('#sidebar').boundingBox() });
    await p.keyboard.press('Escape'); await p.keyboard.press('Escape');
    await p.evaluate(() => { document.getElementById('sidebar').style.maxWidth = ''; });
    await p.setViewportSize({ width: 2400, height: 900 });
    await p.waitForFunction(() => [...document.querySelectorAll('.ws-commands > .toolbar-group')].every(g => g.getAttribute('data-toolbar-stage') === '1'));
    const after = await p.evaluate(() => ({ sources: !document.getElementById('workspace-sources').hidden, add: !document.getElementById('workspace-source-choose').hidden, addBox: document.getElementById('workspace-source-choose').getClientRects().length, stages: [...document.querySelectorAll('.ws-commands > .toolbar-group')].map(g => g.getAttribute('data-toolbar-stage')), open: [...document.querySelectorAll(':popover-open')].map(e => e.id) }));
    ok('Add Source… shows with Sources', after.sources && after.add && after.addBox > 0, JSON.stringify(after));
  } finally { await t.close(); }
  // A workspace without Sources has no Add Source….
  const u = await launch({ viewport: { width: 2400, height: 900 }, app: () => ({ ChooseWorkspaceSource: async () => null }) });
  try {
    await u.page.locator('.ws-commands').waitFor();
    await u.page.waitForTimeout(500);
    u.ok('a workspace without Sources has no Add Source… in the toolbar', await u.page.locator('#workspace-sources').isHidden() && await u.page.locator('#workspace-source-choose').isHidden());
  } finally { await u.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
