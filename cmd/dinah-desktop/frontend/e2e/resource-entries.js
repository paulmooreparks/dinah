// Two seams a product uses for the entries of its own sources
// (docs/design/source-kinds.md and product-applets.md).
//
// A status and a changed mark on an entry: the Workspace tree, the Files
// window and the Pinned list draw the status as PUDL's badge at the end of
// the row, with the changed mark beside it. An entry without them is drawn
// as it always was, the name starts where it does in a row without marks,
// and a status that changes on a live entry moves nothing. A narrow list
// shows the badge compactly, as its glyph and colour with the label as its
// accessible name and tooltip, and a wide one shows the label whole; the
// badge's words are never cut short.
//
// An opener a product registers for a kind of entry or for the end of a
// file's name: every way of opening an entry asks it first. With none
// registered each open goes where it did before. The stub is a workspace
// with a folder of files and a source of records; the windows a document
// would open in are recorded and not opened, since the routing is what is
// checked here.
const { launch } = require('./lib/page');
const OUT = process.argv[3] || require('path').join(require('os').tmpdir(), 'dinah-e2e');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__calls = [];
      const records = [{ id: 'w1', mountAlias: 'w1', name: 'Delivery', primarySourceId: 'files', executionPath: 'C:/Working',
        sources: [{ id: 'files', mountAlias: 'files', kind: 'local-folder', name: 'Working Files', path: 'C:/Working' }, { id: 'records', mountAlias: 'records', kind: 'sample-tracker', name: 'Records' }],
        pins: [{ kind: 'resource', title: 'Check the totals', ref: { workspaceId: 'w1', sourceId: 'records', path: 'R-2' }, itemKind: 'sample-record' },
          { kind: 'resource', title: 'notes.sample.md', ref: { workspaceId: 'w1', sourceId: 'files', path: 'notes.sample.md' }, itemKind: 'file' }] }];
      const state = () => ({ workspace: 'C:/Working', workspaceId: 'w1', workspaceRecords: records, workspaces: ['C:/Working'], recent: ['C:/Working'], defaultProvider: 'devin', catalog: [], defaults: {}, pinned: [] });
      // The records, which the suite changes while they are listed.
      window.__records = [
        { name: 'R-1', title: 'Write the import guide', status: { label: 'To Do', state: '' } },
        { name: 'R-2', title: 'Check the totals', status: { label: 'In Review', state: 'accent' }, changed: true },
        { name: 'R-3', title: 'Publish the notes', status: { label: 'Done', state: 'positive' } },
        { name: 'R-4', title: 'A record whose title is far too long to fit beside its status in a narrow panel', status: { label: 'Needs Attention', state: 'warn' }, changed: true },
        { name: 'R-5', title: 'Repair the export', status: { label: 'Blocked', state: 'danger' } },
        { name: 'R-6', title: 'Changed and without a status', changed: true },
        { name: 'R-7', title: 'Plain record' }
      ];
      const files = [
        { name: 'guide.md', actions: ['read', 'preview', 'edit', 'edit-text', 'reveal'] },
        { name: 'notes.sample.md', actions: ['read', 'preview', 'edit', 'edit-text', 'reveal'] },
        { name: 'long.other.sample.md', actions: ['read', 'preview', 'edit', 'edit-text', 'reveal'] },
        { name: 'table.csv', actions: ['read', 'preview', 'edit', 'reveal'] }
      ];
      const entriesOf = source => source.id === 'records'
        ? window.__records.map(r => ({ ref: { workspaceId: 'w1', sourceId: 'records', path: r.name }, title: r.title, kind: 'sample-record', glyph: 'ticket', directory: false, revision: '1', actions: [], status: r.status, changed: r.changed }))
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
      const draft = ref => ({ id: 'draft:' + ref.sourceId + ':' + ref.path, title: ref.path.split('/').pop(), revision: 'r1', text: '', origin: { kind: ref.sourceId === 'files' ? 'local-folder' : 'sample-tracker', workspaceId: ref.workspaceId, sourceId: ref.sourceId, path: ref.path } });
      return {
        State: async () => state(), ReadUIState: async () => JSON.parse(localStorage.getItem('fixture-ui') || '{"version":1,"values":{}}'), SaveUIState: async value => localStorage.setItem('fixture-ui', JSON.stringify(value)),
        AgentProviders: async () => [{ id: 'devin', name: 'Devin', installed: true, available: true }],
        StatWorkspaceMount: stat, StatWorkspaceMountExpected: stat, ListWorkspaceMounts: list, ListWorkspaceMountsExpected: list,
        ResolveWorkspaceResourceMount: async ref => { window.__calls.push(['resolve', ref.sourceId, ref.path]); const source = records[0].sources.find(s => s.id === ref.sourceId); return stat('/workspaces/w1/' + source.mountAlias + (ref.path ? '/' + ref.path : '')); },
        PreviewWorkspaceResource: async ref => ({ resource: { ref, title: ref.path, kind: 'file', actions: ['preview'] }, blocks: [{ line: 1, html: '<p>Preview</p>' }], problems: [] }),
        OpenWorkspaceResource: async ref => { window.__calls.push(['external', ref.path]); },
        OpenDraft: async ref => { window.__calls.push(['draft', ref.sourceId, ref.path]); return draft(ref); },
        RecentDocuments: async () => [{ title: 'notes.sample.md', workspaceName: 'Delivery', ref: { workspaceId: 'w1', sourceId: 'files', path: 'notes.sample.md' } }, { title: 'guide.md', workspaceName: 'Delivery', ref: { workspaceId: 'w1', sourceId: 'files', path: 'guide.md' } }],
        ResolveDocumentLink: async (cwd, ref) => {
          const m = /:(\d+)$/.exec(ref), line = m ? +m[1] : 0;
          if (m) ref = ref.slice(0, m.index);
          const name = ref.replace(/^.*\//, ''), at = ref.indexOf('records/') === 0 ? { workspaceId: 'w1', sourceId: 'records', path: name } : { workspaceId: 'w1', sourceId: 'files', path: name };
          return { ref, found: true, path: 'C:/Working/' + name, opens: 'editor', label: 'Open in the Editor', glyph: 'document', resource: at, line };
        },
        ResolveDocumentLinks: async (cwd, refs) => refs.map(ref => ({ ref, found: true, path: 'C:/Working/' + ref, opens: 'editor', label: 'Open in the Editor', glyph: 'document', resource: { workspaceId: 'w1', sourceId: 'files', path: ref } }))
      };
    },
  });
  const { page, ok } = t;
  const frame = () => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  try {
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    const sources = page.locator('#workspace-sources');
    await sources.getByRole('button', { name: 'Expand Records', exact: true }).click();
    await sources.getByRole('button', { name: 'Expand Working Files', exact: true }).click();
    const row = name => sources.locator('a[data-resource]').filter({ has: page.locator('.fm-ti-label', { hasText: new RegExp('^' + name) }) }).first();
    await row('Plain record').waitFor();
    await row('guide.md').waitFor();

    /* ---- A status on an entry ------------------------------------------ */
    const drawn = await page.evaluate(() => Array.from(document.querySelectorAll('#workspace-sources a[data-resource]')).map(a => {
      const badge = a.querySelector('.resource-marks > .badge'), mark = a.querySelector('.resource-changed');
      return { name: a.querySelector('.fm-ti-label').textContent, badge: badge && badge.textContent, cls: badge && badge.className, label: badge && badge.getAttribute('aria-label'), role: badge && badge.getAttribute('role'),
        mark: mark && mark.getAttribute('aria-label'), markRole: mark && mark.getAttribute('role'), marks: a.querySelectorAll('.resource-marks').length };
    }));
    const of = name => drawn.find(d => d.name.indexOf(name) === 0);
    ok('each status is drawn as PUDL\'s badge in its state', of('Write the import guide').cls === 'badge resource-status' && of('Check the totals').cls === 'badge resource-status accent' && of('Publish the notes').cls === 'badge resource-status positive' &&
      of('A record whose').cls === 'badge resource-status warn' && of('Repair the export').cls === 'badge resource-status danger', JSON.stringify(drawn.map(d => d.cls)));
    ok('the badge has the status\'s words and an accessible name', of('Check the totals').badge === 'In Review' && of('Check the totals').label === 'Status: In Review' && of('Check the totals').role === 'img');
    ok('a changed entry has the changed mark, with an accessible name', of('Check the totals').mark === 'Changed since you last looked' && of('Check the totals').markRole === 'img' && of('Changed and without').mark === 'Changed since you last looked' && of('Changed and without').badge === null);
    ok('an entry that is not changed has no mark', of('Publish the notes').mark === null && of('Write the import guide').mark === null);
    ok('an entry without a status or a change has no marks at all', of('Plain record').marks === 0 && of('guide.md').marks === 0 && of('Records').marks === 0 && of('Working Files').marks === 0);
    const glyphs = await page.evaluate(() => {
      const before = sel => { const el = document.querySelector('#workspace-sources ' + sel), s = el && getComputedStyle(el, '::before'); return s ? [s.content !== 'none' && parseFloat(s.width) > 0, (s.maskImage || s.webkitMaskImage) + ' ' + (s.maskSize || s.webkitMaskSize)] : null; };
      const plain = document.createElement('span'); plain.className = 'badge'; plain.textContent = 'x'; document.body.appendChild(plain);
      const own = getComputedStyle(plain, '::before'), base = (own.maskImage || own.webkitMaskImage) + ' ' + (own.maskSize || own.webkitMaskSize); plain.remove();
      return { neutral: before('.badge.resource-status:not(.accent):not(.positive):not(.warn):not(.danger)'), accent: before('.badge.resource-status.accent'), warn: before('.badge.resource-status.warn'), danger: before('.badge.resource-status.danger'), positive: before('.badge.resource-status.positive'), base: base };
    });
    const STATES = ['neutral', 'accent', 'warn', 'danger', 'positive'];
    ok('each badge leads with its state\'s glyph, a different one for each state, and the plain badge is the page\'s own plain badge', STATES.every(k => glyphs[k] && glyphs[k][0]) && new Set(STATES.map(k => glyphs[k][1])).size === 5 && glyphs.neutral[1] === glyphs.base, JSON.stringify(STATES.map(k => [k, glyphs[k] && glyphs[k][0]])));

    const geometry = () => page.evaluate(() => Array.from(document.querySelectorAll('#workspace-sources a[data-resource]')).map(a => {
      const r = el => { const b = el.getBoundingClientRect(); return { x: Math.round(b.x * 10) / 10, y: Math.round(b.y * 10) / 10, right: Math.round(b.right * 10) / 10, width: Math.round(b.width * 10) / 10, height: Math.round(b.height * 10) / 10 }; };
      const label = a.querySelector('.fm-ti-label'), badge = a.querySelector('.resource-marks > .badge'), more = a.parentElement.querySelector(':scope > [data-row-actions]');
      return { name: label.textContent, row: r(a), label: r(label), cut: label.scrollWidth > label.clientWidth, badge: badge && r(badge), badgeCut: badge ? badge.lastElementChild.scrollWidth > badge.lastElementChild.clientWidth : false, more: more && r(more), depth: a.closest('ul').className };
    }));
    let g = await geometry();
    const at = (list, name) => list.find(d => d.name.indexOf(name) === 0);
    ok('the name starts where it does in a row without marks', at(g, 'Check the totals').label.x === at(g, 'Plain record').label.x && at(g, 'Write the import guide').label.x === at(g, 'Plain record').label.x, JSON.stringify([at(g, 'Check the totals').label.x, at(g, 'Plain record').label.x]));
    ok('a row with marks is as tall as a row without', g.filter(d => d.name !== 'Records' && d.name !== 'Working Files').every(d => d.row.height === at(g, 'Plain record').row.height), JSON.stringify(g.map(d => d.row.height)));
    ok('the badge ends at the row\'s end, before the row\'s actions button', g.filter(d => d.badge).every(d => d.more && d.badge.right <= d.more.x && d.badge.right === at(g, 'Check the totals').badge.right), JSON.stringify(g.filter(d => d.badge).map(d => [d.badge.right, d.more && d.more.x])));
    ok('every row keeps its actions button in sight', await page.evaluate(() => Array.from(document.querySelectorAll('#workspace-sources li > [data-row-actions]')).every(b => { const s = getComputedStyle(b); return s.visibility === 'visible' && s.opacity === '1' && b.getBoundingClientRect().width > 0; })));
    for (const theme of ['light', 'dark']) {
      await page.evaluate(theme => pudlSetTheme(theme), theme);
      await frame();
      await sources.screenshot({ path: require('path').join(OUT, 'resource-entries-tree-1440-' + theme + '.png') });
    }
    await page.evaluate(() => pudlSetTheme('light'));

    // A status that changes on a live entry: the words, the state and the
    // mark change, and no row, name or button moves.
    const before = await geometry();
    await page.evaluate(() => {
      __records[0].status = { label: 'In Progress', state: 'accent' }; __records[0].changed = true;
      __records[1].status = { label: 'Done', state: 'positive' }; delete __records[1].changed;
      delete __records[4].status;
      window.__emit('workspace:changed', { kind: 'source-changed' });
    });
    await page.waitForFunction(() => { const b = document.querySelectorAll('#workspace-sources .resource-marks > .badge'); return b.length === 4 && b[0].textContent === 'In Progress'; });
    await frame();
    const after = await geometry();
    const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
    ok('a changed status moves no row, no name\'s start and no actions button', before.every((b, i) => same(b.row, after[i].row) && b.label.x === after[i].label.x && b.label.y === after[i].label.y && same(b.more, after[i].more)), JSON.stringify([before.map(b => b.row), after.map(b => b.row)]));
    ok('the changed status is drawn: its words, its state and its mark', await page.evaluate(() => {
      const rows = Array.from(document.querySelectorAll('#workspace-sources a[data-resource]')), find = name => rows.find(a => a.querySelector('.fm-ti-label').textContent === name);
      const first = find('Write the import guide'), second = find('Check the totals'), fifth = find('Repair the export');
      return first.querySelector('.badge').className === 'badge resource-status accent' && !!first.querySelector('.resource-changed') && second.querySelector('.badge').textContent === 'Done' && !second.querySelector('.resource-changed') && !fifth.querySelector('.resource-marks');
    }));
    ok('the badge still ends where it did, and a mark that came or went left the badge in place', after.filter(d => d.badge).every(d => d.badge.right === at(before, 'Check the totals').badge.right) && at(after, 'Publish the notes').badge.x === at(before, 'Publish the notes').badge.x);

    // The Pinned list shows a pinned entry's status, and follows it.
    const pins = page.locator('#workspace-pins');
    await pins.locator('.resource-marks > .badge').first().waitFor();
    const pinned = await page.evaluate(() => Array.from(document.querySelectorAll('#workspace-pins .browser-row')).map(r => {
      const badge = r.querySelector('.browser-row-line > .resource-marks > .badge'), more = r.querySelector('[data-row-actions]'), title = r.querySelector('.browser-row-title'), glyph = r.querySelector('.pin-glyph');
      return { title: title.textContent, badge: badge && badge.textContent, label: badge && badge.getAttribute('aria-label'), beforeMore: !badge || badge.getBoundingClientRect().right <= more.getBoundingClientRect().x, titleX: title.getBoundingClientRect().x, glyphX: glyph.getBoundingClientRect().x };
    }));
    ok('a pinned entry shows its status at the end of its line, and a pinned file shows none', pinned[0].badge === 'Done' && pinned[0].label === 'Status: Done' && pinned[0].beforeMore && pinned[1].badge === null, JSON.stringify(pinned));
    ok('a pinned row with a status starts its name where a row without one does', pinned[0].titleX === pinned[1].titleX && pinned[0].glyphX === pinned[1].glyphX, JSON.stringify(pinned));
    await page.evaluate(() => { __records[1].status = { label: 'Reopened', state: 'warn' }; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await page.waitForFunction(() => { const b = document.querySelector('#workspace-pins .resource-marks > .badge'); return b && b.textContent === 'Reopened' && b.classList.contains('warn'); });
    ok('the pinned entry\'s status follows a change at the source', true);

    // The two forms of the badge. A list too narrow for a name and a badge
    // with its words shows the badge compactly, as its glyph and colour in
    // a box of one size, and the label is its accessible name and its
    // tooltip. A wider list shows the label whole. The words are never cut
    // short, and the name's start, the actions button and the changed mark
    // are where they were in both forms. The panel itself is resized, since
    // the form follows the width of the list. The records are set to have
    // each state and each length of label, and are put back afterwards.
    const panel = width => page.evaluate(width => {
      const win = document.getElementById('workspace-sources').closest('.win');
      for (const name of ['width', 'min-width', 'max-width']) { if (width) win.style.setProperty(name, width + 'px', 'important'); else win.style.removeProperty(name); }
      if (width) win.style.setProperty('flex', 'none', 'important'); else win.style.removeProperty('flex');
    }, width);
    const forms = () => page.evaluate(() => {
      const round = n => Math.round(n * 10) / 10;
      const read = (row, label, more, end) => {
        const badge = row.querySelector('.resource-marks > .badge'), slot = row.querySelector('.resource-changed-slot'), edge = end.getBoundingClientRect().right;
        const b = badge && badge.getBoundingClientRect(), before = badge && getComputedStyle(badge, '::before');
        return { name: label.textContent, labelX: round(label.getBoundingClientRect().x), labelWidth: round(label.getBoundingClientRect().width), cut: label.scrollWidth > label.clientWidth, room: row.querySelector('.resource-marks') ? round(row.querySelector('.resource-marks').getBoundingClientRect().x - label.getBoundingClientRect().x) : null, height: round(row.getBoundingClientRect().height),
          more: more ? round(edge - more.getBoundingClientRect().x) : null, slot: slot ? round(edge - slot.getBoundingClientRect().x) : null,
          badge: badge ? { words: badge.textContent, shown: badge.querySelector('.resource-status-words').getClientRects().length > 0, name: badge.getAttribute('aria-label'), title: badge.title, role: badge.getAttribute('role'),
            width: round(b.width), height: round(b.height), end: round(edge - b.right), whole: badge.scrollWidth <= badge.clientWidth, glyph: before.content !== 'none' && parseFloat(before.width) > 0, colour: getComputedStyle(badge).backgroundColor } : null };
      };
      return { tree: Array.from(document.querySelectorAll('#workspace-sources a[data-resource]')).map(a => read(a, a.querySelector('.fm-ti-label'), a.parentElement.querySelector(':scope > [data-row-actions]'), a.parentElement)),
        pins: Array.from(document.querySelectorAll('#workspace-pins .browser-row')).map(r => read(r, r.querySelector('.browser-row-title'), r.querySelector('[data-row-actions]'), r)) };
    });
    const LONG = 'Blocked until the supplier answers', LONGEST = 'A status with a great many words that no row has the room to show';
    await page.evaluate(([long, longest]) => {
      window.__kept = JSON.stringify(__records);
      __records[0].status = { label: 'To Do', state: '' }; delete __records[0].changed;
      __records[1].status = { label: 'In Review', state: 'accent' }; __records[1].changed = true;
      __records[4].status = { label: long, state: 'danger' };
      __records[6].status = { label: longest, state: 'warn' };
      window.__emit('workspace:changed', { kind: 'source-changed' });
    }, [LONG, LONGEST]);
    await page.waitForFunction(() => { const b = document.querySelectorAll('#workspace-sources .resource-marks > .badge'); return b.length === 6 && b[0].textContent === 'To Do' && document.querySelector('#workspace-pins .resource-marks > .badge').textContent === 'In Review'; });
    const badged = list => list.filter(d => d.badge), named = (list, name) => list.find(d => d.name.indexOf(name) === 0);
    const one = values => new Set(values).size === 1;

    await panel(300);
    await frame();
    const slim = await forms(), compact = badged(slim.tree)[0].badge.width;
    ok('in a narrow panel every badge is compact: it shows no words', badged(slim.tree).length === 6 && badged(slim.tree).concat(badged(slim.pins)).every(d => !d.badge.shown), JSON.stringify(badged(slim.tree).map(d => [d.badge.words, d.badge.shown])));
    ok('a compact badge keeps its accessible name and its tooltip, which are the whole label', badged(slim.tree).concat(badged(slim.pins)).every(d => d.badge.role === 'img' && d.badge.name === 'Status: ' + d.badge.words && d.badge.title === 'Status: ' + d.badge.words) && named(slim.tree, 'Plain record').badge.name === 'Status: ' + LONGEST, JSON.stringify(badged(slim.tree).map(d => [d.badge.name, d.badge.title])));
    ok('a compact badge of every state has a box of the same size', one(badged(slim.tree).concat(badged(slim.pins)).map(d => d.badge.width + 'x' + d.badge.height)) && badged(slim.tree)[0].badge.width > 0, JSON.stringify(badged(slim.tree).map(d => [d.badge.width, d.badge.height])));
    ok('a compact badge is still a glyph and a colour together, and nothing in it is cut short', badged(slim.tree).every(d => d.badge.glyph && d.badge.whole) && new Set(badged(slim.tree).slice(0, 5).map(d => d.badge.colour)).size === 5, JSON.stringify(badged(slim.tree).map(d => [d.badge.glyph, d.badge.whole, d.badge.colour])));
    ok('in a narrow panel the name has the room: a long name is cut short only at the badge, and a row is as tall as one without marks', named(slim.tree, 'A record whose').cut && named(slim.tree, 'A record whose').labelWidth >= 100 && slim.tree.filter(d => d.name !== 'Records' && d.name !== 'Working Files').every(d => d.height === named(slim.tree, 'guide.md').height), JSON.stringify(named(slim.tree, 'A record whose')));
    for (const theme of ['light', 'dark']) {
      await page.evaluate(theme => pudlSetTheme(theme), theme);
      await frame();
      await sources.screenshot({ path: require('path').join(OUT, 'resource-entries-tree-narrow-' + theme + '.png') });
    }
    await page.evaluate(() => pudlSetTheme('light'));
    // A status that changes within the compact form changes the glyph and
    // the colour, and the badge's box is the one it was.
    await page.evaluate(() => { __records[2].status = { label: 'Rejected by the reviewer', state: 'danger' }; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await page.waitForFunction(() => { const b = document.querySelectorAll('#workspace-sources .resource-marks > .badge'); return b[2] && b[2].textContent === 'Rejected by the reviewer'; });
    await frame();
    const slimAfter = await forms();
    const moved = (a, b) => a.labelX !== b.labelX || a.labelWidth !== b.labelWidth || a.more !== b.more || a.slot !== b.slot || a.height !== b.height || !!a.badge !== !!b.badge || a.badge && (a.badge.width !== b.badge.width || a.badge.height !== b.badge.height || a.badge.end !== b.badge.end);
    ok('a status that changes in the compact form moves nothing and changes no box', slim.tree.every((d, i) => !moved(d, slimAfter.tree[i])) && named(slimAfter.tree, 'Publish the notes').badge.colour !== named(slim.tree, 'Publish the notes').badge.colour, JSON.stringify([slim.tree.map(d => d.badge && d.badge.width), slimAfter.tree.map(d => d.badge && d.badge.width)]));
    await page.evaluate(() => { __records[2].status = { label: 'Done', state: 'positive' }; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await page.waitForFunction(() => { const b = document.querySelectorAll('#workspace-sources .resource-marks > .badge'); return b[2] && b[2].textContent === 'Done'; });
    await panel(250);
    await frame();
    const slimmest = await forms();
    ok('in a very narrow panel the name keeps a few letters and the badge is whole, before the actions button', named(slimmest.tree, 'A record whose').labelWidth >= 30 && badged(slimmest.tree).every(d => d.badge.whole && !d.badge.shown && d.badge.end >= d.more && d.badge.width === compact), JSON.stringify(named(slimmest.tree, 'A record whose')));

    // A wider panel shows a short label whole, and a longer label only
    // where the list is wide enough for it.
    await panel(440);
    await frame();
    const mid = await forms();
    const short = d => d.badge.words.length <= 12;
    ok('in a wider panel a short label is shown whole, in the tree and in the Pinned list', badged(mid.tree).filter(short).length === 3 && badged(mid.tree).concat(badged(mid.pins)).filter(short).every(d => d.badge.shown && d.badge.whole && d.badge.width > compact), JSON.stringify(badged(mid.tree).map(d => [d.badge.words, d.badge.shown, d.badge.whole, d.badge.width])));
    ok('a label too long for that width stays compact, in the same box, and is not cut short', badged(mid.tree).filter(d => !short(d)).length === 3 && badged(mid.tree).filter(d => !short(d)).every(d => !d.badge.shown && d.badge.whole && d.badge.width === compact && d.badge.name === 'Status: ' + d.badge.words), JSON.stringify(badged(mid.tree).map(d => [d.badge.words, d.badge.shown, d.badge.width])));
    ok('with the full badge the name still has room for about 24 letters', badged(mid.tree).concat(badged(mid.pins)).every(d => d.room >= 150), JSON.stringify(mid.tree.concat(mid.pins).map(d => d.room)));
    const sameEnds = (a, b) => a.every((d, i) => d.labelX === b[i].labelX && d.more === b[i].more && d.slot === b[i].slot && (!d.badge || d.badge.end === b[i].badge.end) && d.height === b[i].height);
    ok('the name\'s start, the actions button, the changed mark and the badge\'s end are where they were in the compact form', sameEnds(slim.tree, mid.tree) && sameEnds(slim.pins, mid.pins) && named(mid.tree, 'Check the totals').slot !== null && named(mid.pins, 'Check the totals').slot !== null, JSON.stringify([slim.tree.map(d => [d.labelX, d.more, d.slot]), mid.tree.map(d => [d.labelX, d.more, d.slot])]));
    await panel(600);
    await frame();
    const wide = await forms();
    ok('in a wide panel a label of up to 40 characters is shown whole, and a longer one never is', badged(wide.tree).filter(d => d.badge.words.length <= 40).length === 5 && badged(wide.tree).every(d => d.badge.whole && d.badge.shown === (d.badge.words.length <= 40)) && named(wide.tree, 'Plain record').badge.width === compact, JSON.stringify(badged(wide.tree).map(d => [d.badge.words, d.badge.shown, d.badge.whole])));
    ok('in a wide panel everything at a row\'s ends is still where it was', sameEnds(slim.tree, wide.tree) && sameEnds(slim.pins, wide.pins), JSON.stringify([slim.tree.map(d => [d.labelX, d.more, d.slot]), wide.tree.map(d => [d.labelX, d.more, d.slot])]));
    for (const theme of ['light', 'dark']) {
      await page.evaluate(theme => pudlSetTheme(theme), theme);
      await frame();
      await sources.screenshot({ path: require('path').join(OUT, 'resource-entries-tree-wide-' + theme + '.png') });
    }
    await page.evaluate(() => pudlSetTheme('light'));
    // A status that changes within the full form: a longer label takes its
    // room from the end of the name, and nothing else moves.
    await page.evaluate(() => { __records[0].status = { label: 'In Progress', state: 'accent' }; __records[0].changed = true; delete __records[1].changed; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await page.waitForFunction(() => { const b = document.querySelectorAll('#workspace-sources .resource-marks > .badge'); return b[0].textContent === 'In Progress' && !document.querySelector('#workspace-pins .resource-changed'); });
    await frame();
    const wideAfter = await forms();
    ok('a status that changes in the full form moves no name\'s start, no actions button, no changed mark and no badge\'s end', wide.tree.every((d, i) => d.labelX === wideAfter.tree[i].labelX && d.more === wideAfter.tree[i].more && d.height === wideAfter.tree[i].height && (!d.badge || d.badge.end === wideAfter.tree[i].badge.end)) && named(wideAfter.tree, 'Write the import guide').slot === named(wide.tree, 'Check the totals').slot && named(wideAfter.tree, 'Write the import guide').badge.shown && named(wideAfter.tree, 'Write the import guide').badge.whole, JSON.stringify([wide.tree.map(d => [d.labelX, d.more, d.badge && d.badge.end]), wideAfter.tree.map(d => [d.labelX, d.more, d.badge && d.badge.end])]));
    await panel(0);
    await page.evaluate(() => { const kept = JSON.parse(window.__kept); __records.splice(0, __records.length, ...kept); window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await page.waitForFunction(() => { const b = document.querySelectorAll('#workspace-sources .resource-marks > .badge'), pin = document.querySelector('#workspace-pins .resource-marks > .badge'); return b.length === 4 && b[0].textContent === 'In Progress' && pin && pin.textContent === 'Reopened'; });
    await frame();

    // The Files window lists the same entries with the same badge.
    await page.evaluate(() => dinahResources.openFiles('/workspaces/w1/records', { workspaceId: 'w1', sourceId: 'records', path: '' }));
    const filesWin = page.locator('.dinah-files').last();
    await filesWin.locator('.fm-row[data-name="R-3"] .resource-marks > .badge').waitFor();
    const fileRows = await filesWin.evaluate(root => Array.from(root.querySelectorAll('.fm-row[data-name]')).map(r => {
      const badge = r.querySelector('.fm-cell > .resource-marks > .badge'), mark = r.querySelector('.resource-changed'), name = r.querySelector('.fm-name');
      return { name: r.dataset.name, badge: badge && badge.textContent, cls: badge && badge.className, label: badge && badge.getAttribute('aria-label'), mark: !!mark, nameX: name.getBoundingClientRect().x, height: r.getBoundingClientRect().height, last: !badge || badge.parentElement === r.querySelector('.fm-cell').lastElementChild };
    }));
    const fileRow = name => fileRows.find(r => r.name === name);
    ok('Files draws each entry\'s status as the same badge, last in the name cell', fileRow('R-3').badge === 'Done' && fileRow('R-3').cls === 'badge resource-status positive' && fileRow('R-3').label === 'Status: Done' && fileRow('R-4').mark && fileRow('R-6').mark && fileRow('R-6').badge === null && fileRow('R-7').badge === null && !fileRow('R-7').mark && fileRows.every(r => r.last), JSON.stringify(fileRows));
    ok('in Files a row with a status starts its name where a row without one does', fileRows.every(r => r.nameX === fileRow('R-7').nameX), JSON.stringify(fileRows.map(r => r.nameX)));
    const inSight = await filesWin.evaluate(root => {
      const pane = root.querySelector('[data-fb="main"]').getBoundingClientRect(), badge = root.querySelector('.fm-row[data-name="R-3"] .resource-marks > .badge').getBoundingClientRect();
      const widths = Array.from(root.querySelectorAll('.fm-cell > .resource-marks')).map(m => m.getBoundingClientRect().width);
      return { seen: badge.x >= pane.x && badge.right <= pane.right && badge.width > 0, widths: widths };
    });
    ok('in Files the status follows the name, in sight, in a place of one width for every row', inSight.seen && new Set(inSight.widths).size === 1, JSON.stringify(inSight));
    ok('in Files a row with marks keeps its name on one line, as a row without them does', fileRows.every(r => Math.abs(r.height - fileRow('R-7').height) < 2), JSON.stringify(fileRows.map(r => r.height)));
    // A status that comes to a listed entry, or goes, leaves every row of Files where it was.
    const rowBoxes = () => filesWin.evaluate(root => Array.from(root.querySelectorAll('.fm-row[data-name]')).map(r => { const b = r.getBoundingClientRect(), n = r.querySelector('.fm-name').getBoundingClientRect(); return [r.dataset.name, b.y, b.height, n.x, n.y]; }).concat([['table', root.querySelector('.fm-list').getBoundingClientRect().width, root.querySelector('.fm-c-name').getBoundingClientRect().width]]));
    const filesBefore = await rowBoxes();
    await page.evaluate(() => { __records[6].status = { label: 'To Do', state: '' }; __records[6].changed = true; delete __records[2].status; __records[3].status = { label: 'A status with a good many more words', state: 'danger' }; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await filesWin.locator('.fm-row[data-name="R-7"] .resource-marks > .badge').waitFor();
    await page.waitForFunction(() => !document.querySelector('.dinah-files .fm-row[data-name="R-3"] .resource-marks'));
    await frame();
    const filesAfter = await rowBoxes();
    ok('a status that comes, goes or grows in Files moves no row and no name, and changes no column\'s width', JSON.stringify(filesBefore) === JSON.stringify(filesAfter), JSON.stringify([filesBefore, filesAfter]));
    await page.evaluate(() => { delete __records[6].status; delete __records[6].changed; __records[2].status = { label: 'Done', state: 'positive' }; __records[3].status = { label: 'Needs Attention', state: 'warn' }; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await filesWin.locator('.fm-row[data-name="R-3"] .resource-marks > .badge').waitFor();
    await page.waitForFunction(() => !document.querySelector('.dinah-files .fm-row[data-name="R-7"] .resource-marks'));
    await t.shot('resource-entries-files');
    // The two forms in Files. The marks' place has one width for every row
    // in each form: the compact badge's until the window is wide, and from
    // there a place that shows a short label whole. A label too long for
    // the place stays compact. The name and the changed mark do not move:
    // their places are measured from the row's start, since the window is
    // widened here.
    const filesForms = () => filesWin.evaluate(root => Array.from(root.querySelectorAll('.fm-row[data-name]')).map(r => {
      const badge = r.querySelector('.fm-cell > .resource-marks > .badge'), slot = r.querySelector('.resource-changed-slot'), place = r.querySelector('.fm-cell > .resource-marks'), left = r.getBoundingClientRect().x;
      return { name: r.dataset.name, nameX: r.querySelector('.fm-name').getBoundingClientRect().x - left, slotX: slot && slot.getBoundingClientRect().x - left, place: place && place.getBoundingClientRect().width,
        badge: badge && { words: badge.textContent, shown: badge.querySelector('.resource-status-words').getClientRects().length > 0, name: badge.getAttribute('aria-label'), title: badge.title, x: badge.getBoundingClientRect().x - left, width: badge.getBoundingClientRect().width, whole: badge.scrollWidth <= badge.clientWidth } };
    }));
    const filesWidth = width => filesWin.evaluate((root, width) => { root.style.inlineSize = width ? width + 'px' : ''; root.style.flex = width ? 'none' : ''; }, width);
    const slimFiles = await filesForms(), fileBadges = list => list.filter(r => r.badge);
    ok('in a Files window of ordinary width every badge is compact, in a box of one size, with its label as its accessible name and tooltip', fileBadges(slimFiles).length === 4 && fileBadges(slimFiles).every(r => !r.badge.shown && r.badge.whole && r.badge.name === 'Status: ' + r.badge.words && r.badge.title === r.badge.name) && one(fileBadges(slimFiles).map(r => r.badge.width)) && one(slimFiles.filter(r => r.place).map(r => r.place)), JSON.stringify(slimFiles));
    await filesWidth(1000);
    await frame();
    const wideFiles = await filesForms();
    ok('in a wide Files window a short label is shown whole and a longer one stays compact', fileBadges(wideFiles).every(r => r.badge.whole && r.badge.shown === (r.badge.words.length <= 12)) && wideFiles.find(r => r.name === 'R-3').badge.shown && !wideFiles.find(r => r.name === 'R-4').badge.shown && wideFiles.find(r => r.name === 'R-4').badge.width === slimFiles.find(r => r.name === 'R-4').badge.width, JSON.stringify(wideFiles));
    ok('in Files the marks\' place still has one width for every row, and the name, the changed mark and the badge\'s start are where they were', one(wideFiles.filter(r => r.place).map(r => r.place)) && wideFiles.every((r, i) => r.nameX === slimFiles[i].nameX && r.slotX === slimFiles[i].slotX && (!r.badge || r.badge.x === slimFiles[i].badge.x)) && wideFiles.find(r => r.name === 'R-4').slotX !== null, JSON.stringify([slimFiles.map(r => [r.nameX, r.slotX]), wideFiles.map(r => [r.nameX, r.slotX])]));
    const wideBefore = await rowBoxes();
    await page.evaluate(() => { __records[2].status = { label: 'In Progress', state: 'accent' }; __records[2].changed = true; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await page.waitForFunction(() => { const b = document.querySelector('.dinah-files .fm-row[data-name="R-3"] .resource-marks > .badge'); return b && b.textContent === 'In Progress'; });
    await frame();
    ok('a status that changes in the full form in Files moves no row and no name, and changes no column\'s width', JSON.stringify(wideBefore) === JSON.stringify(await rowBoxes()));
    await t.shot('resource-entries-files-wide');
    await page.evaluate(() => { __records[2].status = { label: 'Done', state: 'positive' }; delete __records[2].changed; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await page.waitForFunction(() => { const b = document.querySelector('.dinah-files .fm-row[data-name="R-3"] .resource-marks > .badge'); return b && b.textContent === 'Done' && !document.querySelector('.dinah-files .fm-row[data-name="R-3"] .resource-changed'); });
    await filesWidth(0);
    await frame();

    /* ---- The window that opens an entry --------------------------------- */
    // The windows a document or an entry would open in are recorded.
    await page.evaluate(() => {
      window.__opened = []; window.__product = [];
      const real = dinahWorkbench.open;
      window.__realOpen = real;
      dinahWorkbench.open = function (item) { if (item && (/^(editor|content-)/.test(item.kind) || item.kind === 'workspace-resource')) { __opened.push(item.kind + ' ' + item.path); return Promise.resolve('recorded'); } return real.apply(this, arguments); };
    });
    const settle = async fn => { await page.evaluate(() => { __opened.length = 0; __product.length = 0; }); await fn(); await page.waitForFunction(() => __opened.length + __product.length > 0); await frame(); return page.evaluate(() => ({ opened: __opened.slice(), product: __product.slice() })); };

    // With no opener registered, every open goes where it did before.
    let went = await settle(() => row('notes.sample.md').dblclick());
    ok('with no opener, a Markdown file double-clicked in the tree opens in Docs', went.opened.length === 1 && went.opened[0] === 'content-docs draft:files:notes.sample.md' && !went.product.length, JSON.stringify(went));
    went = await settle(() => page.evaluate(() => dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'files', path: 'table.csv' })));
    ok('with no opener, a text file opens in the Editor', went.opened[0] === 'editor draft:files:table.csv', JSON.stringify(went));
    ok('with no opener, nothing is read to look for one', await page.evaluate(() => dinahResources.openerFor({ kind: 'sample-record', ref: { path: 'R-1' } }) === null && !dinahResources.kindOpeners()));
    await row('Plain record').dblclick();
    await frame();
    ok('with no opener, an entry with no action of its own opens nothing', await page.evaluate(() => __opened.length === 1 && __product.length === 0));

    // A product registers its openers after the page and its layout are up.
    const registered = await page.evaluate(() => {
      const errors = []; const real = console.error; console.error = function (text) { errors.push(String(text)); };
      const record = (name) => function (item, opts) { __product.push({ by: name, path: item.ref && item.ref.path, kind: item.kind || '', title: item.title || '', intent: opts.intent, draft: !!opts.draft, line: opts.line || 0, status: item.status ? item.status.label : '' }); return 'product-window'; };
      const out = {
        kind: dinahResources.registerOpener({ itemKind: 'sample-record', title: 'Records', glyph: 'ticket', open: record('kind') }),
        suffix: dinahResources.registerOpener({ suffix: '.sample.md', title: 'Samples', glyph: 'board', open: record('suffix') }),
        longer: dinahResources.registerOpener({ suffix: '.other.sample.md', open: record('longer') }),
        again: dinahResources.registerOpener({ itemKind: 'sample-record', open: record('second') }),
        againSuffix: dinahResources.registerOpener({ suffix: '.SAMPLE.md', open: record('second') }),
        both: dinahResources.registerOpener({ itemKind: 'x', suffix: '.x', open: record('both') }),
        neither: dinahResources.registerOpener({ open: record('neither') }),
        noOpen: dinahResources.registerOpener({ itemKind: 'y' }),
        badSuffix: dinahResources.registerOpener({ suffix: 'sample', open: record('bad') })
      };
      console.error = real;
      return { out, errors };
    });
    ok('an opener registers for an item kind and for a suffix', registered.out.kind === true && registered.out.suffix === true && registered.out.longer === true);
    ok('a second registration of the same key is refused with a clear message, and the first stays', registered.out.again === false && registered.out.againSuffix === false && registered.errors.some(e => /already registered for the item kind "sample-record"/.test(e) && /first registration stays/.test(e)) && registered.errors.some(e => /already registered for the suffix "\.sample\.md"/.test(e)), JSON.stringify(registered.errors));
    ok('a registration with both keys, neither key, no open function or a suffix without its dot is refused', !registered.out.both && !registered.out.neither && !registered.out.noOpen && !registered.out.badSuffix && registered.errors.length === 6, JSON.stringify(registered.errors));
    const precedence = await page.evaluate(() => {
      const by = item => { const o = dinahResources.openerFor(item); return o ? (o.itemKind || o.suffix) : null; };
      return { kindOverSuffix: by({ kind: 'sample-record', ref: { path: 'R-9.sample.md' } }), longest: by({ kind: 'file', ref: { path: 'a/long.other.sample.md' } }), short: by({ ref: { path: 'a/b/Notes.SAMPLE.md' } }), none: by({ kind: 'file', ref: { path: 'guide.md' } }), folder: by({ kind: 'directory', directory: true, ref: { path: 'folder.sample.md' } }), title: dinahResources.openerFor({ ref: { path: 'x.sample.md' } }).title };
    });
    ok('an item kind is chosen before a suffix, the longest suffix is chosen, and case is ignored', precedence.kindOverSuffix === 'sample-record' && precedence.longest === '.other.sample.md' && precedence.short === '.sample.md' && precedence.none === null && precedence.folder === null && precedence.title === 'Samples', JSON.stringify(precedence));

    // The tree: a double-click by item kind, and Enter by suffix.
    went = await settle(() => row('Plain record').dblclick());
    ok('a double-click in the tree opens an entry of the kind in the product\'s window, with the whole item', went.product.length === 1 && went.product[0].by === 'kind' && went.product[0].path === 'R-7' && went.product[0].kind === 'sample-record' && went.product[0].intent === 'read' && !went.opened.length, JSON.stringify(went));
    const draftsBefore = await page.evaluate(() => __calls.filter(c => c[0] === 'draft').length);
    went = await settle(async () => { await row('notes.sample.md').focus(); await page.keyboard.press('Enter'); });
    ok('Enter in the tree opens a file with the suffix in the product\'s window, and no draft is made', went.product.length === 1 && went.product[0].by === 'suffix' && went.product[0].path === 'notes.sample.md' && !went.opened.length && await page.evaluate(n => __calls.filter(c => c[0] === 'draft').length === n, draftsBefore), JSON.stringify(went));
    went = await settle(() => row('long.other.sample.md').dblclick());
    ok('the longer suffix\'s opener takes the file both suffixes end', went.product[0].by === 'longer' && await page.evaluate(n => __calls.filter(c => c[0] === 'draft').length === n, draftsBefore), JSON.stringify(went));
    went = await settle(() => row('guide.md').dblclick());
    ok('a file no opener matches still opens in Docs', went.opened[0] === 'content-docs draft:files:guide.md' && !went.product.length, JSON.stringify(went));
    // The entry's context menu is what it was.
    await row('notes.sample.md').click({ button: 'right' });
    const menu = page.locator('.context-menu:popover-open');
    await menu.waitFor();
    const menuRows = (await menu.locator('.menu-action').allTextContents()).map(s => s.trim());
    ok('the entry\'s context menu keeps its rows and gains none', JSON.stringify(menuRows) === JSON.stringify(['Open', 'Edit', 'Edit as Text', 'Show in Folder', 'Copy Path', 'Unpin from Workspace']), JSON.stringify(menuRows));
    went = await settle(() => menu.getByText('Edit as Text', { exact: true }).click());
    ok('Edit as Text asks for the source by name and keeps the Editor', went.opened[0] === 'editor draft:files:notes.sample.md' && !went.product.length, JSON.stringify(went));
    await row('notes.sample.md').click({ button: 'right' });
    await menu.waitFor();
    went = await settle(() => menu.getByText('Edit', { exact: true }).click());
    ok('Edit opens the product\'s window, saying that the person asked to edit', went.product[0].by === 'suffix' && went.product[0].intent === 'edit' && !went.opened.length, JSON.stringify(went));

    // The Pinned list.
    went = await settle(() => pins.locator('a[data-pin-key]').first().click());
    ok('a pinned entry of the kind opens in the product\'s window', went.product.length === 1 && went.product[0].by === 'kind' && went.product[0].path === 'R-2' && went.product[0].status === 'Reopened', JSON.stringify(went));
    went = await settle(() => pins.locator('a[data-pin-key]').nth(1).click());
    ok('a pinned file with the suffix opens in the product\'s window', went.product.length === 1 && went.product[0].by === 'suffix' && !went.opened.length, JSON.stringify(went));

    // The Files window.
    await page.evaluate(() => pudlWindows.raise(pudlWindows.state().open.find(k => (dinahWorkbench.item(k) || {}).kind === 'files')));
    went = await settle(() => filesWin.locator('.fm-row[data-name="R-3"]').dblclick());
    ok('a double-click in Files opens an entry of the kind in the product\'s window', went.product.length === 1 && went.product[0].by === 'kind' && went.product[0].path === 'R-3' && went.product[0].title === 'Publish the notes', JSON.stringify(went));
    await page.evaluate(() => dinahResources.openFiles('/workspaces/w1/files', { workspaceId: 'w1', sourceId: 'files', path: '' }));
    const folderWin = page.locator('.dinah-files').filter({ has: page.locator('.fm-row[data-name="notes.sample.md"]') }).last();
    await folderWin.locator('.fm-row[data-name="notes.sample.md"]').waitFor();
    went = await settle(() => folderWin.locator('.fm-row[data-name="notes.sample.md"]').dblclick());
    ok('a double-click in Files opens a file with the suffix in the product\'s window', went.product.length === 1 && went.product[0].by === 'suffix' && !went.opened.length, JSON.stringify(went));

    // Recent documents.
    went = await settle(async () => {
      await page.keyboard.press('Escape');
      await page.locator('nav[data-menubar] .menubar-title').filter({ hasText: /^Documents$/ }).click();
      await page.locator('.menubar-panel .menu-action').filter({ hasText: /^Recent/ }).first().hover();
      await page.locator('.menubar-panel .menu-action').filter({ hasText: /notes\.sample\.md/ }).first().click();
    });
    ok('a Recent document with the suffix opens in the product\'s window', went.product.length === 1 && went.product[0].by === 'suffix' && went.product[0].path === 'notes.sample.md' && !went.opened.length, JSON.stringify(went));

    // A link to a workspace file, as a chat's message or a document has it.
    went = await settle(() => page.evaluate(() => { dinahChatText.openDocument('C:/Working', 'notes.sample.md:12'); }));
    ok('a link to a file with the suffix opens in the product\'s window, with the line it asks for', went.product.length === 1 && went.product[0].by === 'suffix' && went.product[0].line === 12 && went.product[0].intent === 'read' && !went.opened.length, JSON.stringify(went));
    went = await settle(() => page.evaluate(() => { dinahChatText.openDocument('C:/Working', 'records/R-5'); }));
    ok('a link to an entry of the kind opens in the product\'s window', went.product.length === 1 && went.product[0].by === 'kind' && went.product[0].path === 'R-5', JSON.stringify(went));
    went = await settle(() => page.evaluate(() => { dinahChatText.openDocument('C:/Working', 'guide.md'); }));
    ok('a link to a file no opener matches opens where Go says', went.opened[0] === 'content-docs draft:files:guide.md' && !went.product.length, JSON.stringify(went));
    const chip = await page.evaluate(async () => {
      const host = document.createElement('div');
      host.innerHTML = '<a class="smart-chip" data-chip-local="notes.sample.md">notes</a><a class="smart-chip" data-chip-local="guide.md">guide</a>';
      document.body.appendChild(host);
      dinahChatText.linkPaths(host, 'C:/Working');
      for (let i = 0; i < 100 && !Array.from(host.querySelectorAll('a')).every(a => a.dataset.opens); i++) await new Promise(r => setTimeout(r, 50));
      const out = Array.from(host.querySelectorAll('a')).map(a => [a.title, a.querySelector('.link-glyph') ? a.querySelector('.link-glyph').style.getPropertyValue('--glyph') : '']);
      host.remove();
      return out;
    });
    ok('a link an opener takes by its name says so, with the opener\'s title and glyph', chip[0][0] === 'Open in Samples' && chip[0][1] === 'var(--glyph-board)' && chip[1][0] === 'Open in the Editor', JSON.stringify(chip));

    // A draft that is already at hand, as In Progress and a notice have it.
    went = await settle(() => page.evaluate(() => dinahEditor.open({ id: 'd1', title: 'notes.sample.md', revision: 'r', origin: { kind: 'local-folder', workspaceId: 'w1', sourceId: 'files', path: 'notes.sample.md' } })));
    ok('a draft of a file with the suffix opens in the product\'s window, which is given the draft', went.product.length === 1 && went.product[0].by === 'suffix' && went.product[0].draft === true && went.product[0].intent === 'edit', JSON.stringify(went));
    went = await settle(() => page.evaluate(() => dinahEditor.open({ id: 'd2', title: 'R-1', revision: 'r', origin: { kind: 'sample-tracker', workspaceId: 'w1', sourceId: 'records', path: 'R-1' } })));
    ok('a draft of an entry of the kind opens in the product\'s window', went.product.length === 1 && went.product[0].by === 'kind' && went.product[0].draft === true && went.product[0].kind === 'sample-record', JSON.stringify(went));
    went = await settle(() => page.evaluate(() => dinahContentWindows.open({ id: 'd1', title: 'notes.sample.md', revision: 'r', origin: { kind: 'local-folder', workspaceId: 'w1', sourceId: 'files', path: 'notes.sample.md' } }, 'docs')));
    ok('an open that says the application, as its own Open dialog does, opens there', went.opened[0] === 'content-docs d1' && !went.product.length, JSON.stringify(went));

    // The generic viewer named by a saved layout or an address still opens:
    // a window's own key is not an entry to route.
    await page.evaluate(() => { dinahWorkbench.open = window.__realOpen; });
    const viewer = await page.evaluate(() => dinahWorkbench.open(dinahWorkbench.resourceItem({ workspaceId: 'w1', sourceId: 'files', path: 'notes.sample.md' }, 'notes.sample.md')).then(key => (dinahWorkbench.item(key) || {}).kind));
    ok('a window a layout or an address names by its own kind opens as that kind', viewer === 'workspace-resource');
    await t.shot('resource-entries-end');
  } finally { await t.close(); }
})();
