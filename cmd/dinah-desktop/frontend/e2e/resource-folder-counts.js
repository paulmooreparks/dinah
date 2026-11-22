// A folder's counts (docs/design/source-kinds.md, "A Folder's Counts").
//
// A source says on a folder's entry how many entries the folder stands for
// and how many of them have changes the person has not seen. Either may be
// not known yet, which is different from zero. The Workspace tree, the
// Files window and the Pinned list draw the total as the count badge the
// section heads have, at the end of the folder's row, and the changed count
// beside it with the changed mark's glyph. A count that is not known draws
// nothing and keeps no place, a changed count of zero draws nothing, and an
// entry that is not a folder has no counts. A count that arrives or changes
// takes its room from the end of the name: the toggle, the name's start and
// the row's actions button stay where they are. The stub is a workspace
// with a folder of files and a source of an invented kind whose folders are
// saved queries.
const { launch } = require('./lib/page');
const OUT = process.argv[3] || require('path').join(require('os').tmpdir(), 'dinah-e2e');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      const pinned = (path, title) => ({ kind: 'resource', title: title, ref: { workspaceId: 'w1', sourceId: 'records', path: path }, itemKind: 'sample-query', directory: true });
      const records = [{ id: 'w1', mountAlias: 'w1', name: 'Delivery', primarySourceId: 'files', executionPath: 'C:/Working',
        sources: [{ id: 'files', mountAlias: 'files', kind: 'local-folder', name: 'Working Files', path: 'C:/Working' }, { id: 'records', mountAlias: 'records', kind: 'sample-tracker', name: 'Records' }],
        pins: [pinned('Q-1', 'Open Records'), pinned('Q-3', 'Not Read Yet'), pinned('Q-5', 'Shared with the Team')] }];
      const state = () => ({ workspace: 'C:/Working', workspaceId: 'w1', workspaceRecords: records, workspaces: ['C:/Working'], recent: ['C:/Working'], defaultProvider: 'devin', catalog: [], defaults: {}, pinned: [] });
      // The folders, which the suite changes while they are listed.
      window.__folders = [
        { name: 'Q-1', title: 'Open Records', count: 12, changedCount: 3 },
        { name: 'Q-2', title: 'Waiting on Review', count: 4, changedCount: 0 },
        { name: 'Q-3', title: 'Not Read Yet' },
        { name: 'Q-4', title: 'Nothing Here', count: 0 },
        { name: 'Q-5', title: 'Shared with the Team', count: 128, changedCount: 17, status: { label: 'In Review', state: 'accent' }, changed: true },
        { name: 'Q-6', title: 'Changes Only', changedCount: 2 },
        { name: 'Q-7', title: 'A saved query whose name is far too long to fit beside its counts in a narrow panel', count: 45, changedCount: 9 },
        { name: 'Q-8', title: 'One Entry', count: 1 },
        { name: 'R-1', title: 'A record, which is not a folder', file: true, count: 5, changedCount: 2 }
      ];
      const entriesOf = source => source.id === 'records'
        ? window.__folders.map(f => ({ ref: { workspaceId: 'w1', sourceId: 'records', path: f.name }, title: f.title, kind: f.file ? 'sample-record' : 'sample-query', directory: !f.file, revision: '1', actions: f.file ? [] : ['list'], status: f.status, changed: f.changed, count: f.count, changedCount: f.changedCount }))
        : [{ ref: { workspaceId: 'w1', sourceId: 'files', path: 'notes' }, title: 'notes', kind: 'file', directory: true, revision: '1', writable: true, actions: ['list'] }, { ref: { workspaceId: 'w1', sourceId: 'files', path: 'guide.md' }, title: 'guide.md', kind: 'file', directory: false, revision: '1', writable: true, actions: ['read', 'preview', 'edit'] }];
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
      return {
        State: async () => state(), ReadUIState: async () => JSON.parse(localStorage.getItem('fixture-ui') || '{"version":1,"values":{}}'), SaveUIState: async value => localStorage.setItem('fixture-ui', JSON.stringify(value)),
        AgentProviders: async () => [{ id: 'devin', name: 'Devin', installed: true, available: true }],
        StatWorkspaceMount: stat, StatWorkspaceMountExpected: stat, ListWorkspaceMounts: list, ListWorkspaceMountsExpected: list,
        ResolveWorkspaceResourceMount: async ref => { const source = records[0].sources.find(s => s.id === ref.sourceId); return stat('/workspaces/w1/' + source.mountAlias + (ref.path ? '/' + ref.path : '')); }
      };
    },
  });
  const { page, ok } = t;
  const frame = () => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
  const changedAtSource = () => page.evaluate(() => window.__emit('workspace:changed', { kind: 'source-changed' }));
  try {
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    const sources = page.locator('#workspace-sources'), pins = page.locator('#workspace-pins');
    await sources.getByRole('button', { name: 'Expand Records', exact: true }).click();
    await sources.getByRole('button', { name: 'Expand Working Files', exact: true }).click();
    await sources.locator('.fm-ti-label', { hasText: /^One Entry$/ }).waitFor();
    await sources.locator('.fm-ti-label', { hasText: /^guide\.md$/ }).waitFor();
    await pins.locator('.resource-count-total').first().waitFor();

    // Every row of a list, with the places of what must not move and what its counts are.
    const rows = () => page.evaluate(() => {
      const round = n => Math.round(n * 10) / 10;
      const box = el => { if (!el) return null; const b = el.getBoundingClientRect(); return { x: round(b.x), y: round(b.y), right: round(b.right), width: round(b.width), height: round(b.height) }; };
      const count = el => el && { text: el.textContent, name: el.getAttribute('aria-label'), role: el.getAttribute('role'), title: el.title, cls: el.className, box: box(el), whole: el.scrollWidth <= el.clientWidth, drawn: el.getBoundingClientRect().width > 2 };
      const read = (row, label, more, toggle, glyph) => {
        const marks = row.querySelector('.resource-marks'), badge = row.querySelector('.badge.resource-status'), slot = row.querySelector('.resource-changed-slot');
        return { name: label.textContent, row: box(row), label: box(label), cut: label.scrollWidth > label.clientWidth, more: box(more), toggle: box(toggle), glyph: box(glyph), marks: box(marks),
          total: count(row.querySelector('.resource-count-total')), changed: count(row.querySelector('.resource-count-changed')), places: row.querySelectorAll('.resource-counts > *').length,
          badge: box(badge), slot: box(slot), mark: !!row.querySelector('.resource-changed') };
      };
      return { tree: Array.from(document.querySelectorAll('#workspace-sources a[data-resource]')).map(a => read(a, a.querySelector('.fm-ti-label'), a.parentElement.querySelector(':scope > [data-row-actions]'), a.querySelector('.tree-toggle'), a.querySelector(':scope > .glyph'))),
        pins: Array.from(document.querySelectorAll('#workspace-pins .browser-row')).map(r => read(r, r.querySelector('.browser-row-title'), r.querySelector('[data-row-actions]'), null, r.querySelector('.pin-glyph'))) };
    });
    const at = (list, name) => list.find(d => d.name.indexOf(name) === 0);
    let g = await rows();

    /* ---- What is drawn ----------------------------------------------------- */
    ok('a folder with both counts shows its total and, beside it, its changed count', at(g.tree, 'Open Records').total.text === '12' && at(g.tree, 'Open Records').changed.text === '3', JSON.stringify(at(g.tree, 'Open Records')));
    ok('each count has an accessible name', at(g.tree, 'Open Records').total.name === '12 entries' && at(g.tree, 'Open Records').total.role === 'img' && at(g.tree, 'Open Records').changed.name === '3 changed since you last looked' && at(g.tree, 'Open Records').changed.role === 'img' && at(g.tree, 'One Entry').total.name === '1 entry', JSON.stringify([at(g.tree, 'Open Records').total, at(g.tree, 'Open Records').changed]));
    ok('a changed count of zero draws nothing, and the total stays', at(g.tree, 'Waiting on Review').total.text === '4' && at(g.tree, 'Waiting on Review').changed === null && at(g.tree, 'Waiting on Review').places === 1);
    ok('a total of zero is drawn, since it is known', at(g.tree, 'Nothing Here').total.text === '0' && at(g.tree, 'Nothing Here').total.name === '0 entries');
    ok('a folder whose counts are not known draws nothing and keeps no place', at(g.tree, 'Not Read Yet').marks === null && at(g.tree, 'Not Read Yet').places === 0 && at(g.tree, 'Records').marks === null);
    ok('a changed count is drawn when the total is not known', at(g.tree, 'Changes Only').changed.text === '2' && at(g.tree, 'Changes Only').total === null && at(g.tree, 'Changes Only').places === 1);
    ok('an entry that is not a folder has no counts', at(g.tree, 'A record').marks === null && at(g.tree, 'guide.md').marks === null);
    ok('the base\'s own folders have no counts', at(g.tree, 'Working Files').marks === null && at(g.tree, 'notes').marks === null);
    ok('every folder has its toggle, which the checks below measure', g.tree.filter(d => d.name !== 'guide.md' && d.name.indexOf('A record') !== 0).every(d => d.toggle && d.toggle.width > 0) && g.tree.every(d => d.glyph && d.more));
    ok('a row with counts and no status keeps no place for a changed mark', at(g.tree, 'Open Records').slot === null && at(g.tree, 'Nothing Here').slot === null);
    const styles = await page.evaluate(() => {
      const look = (el, pseudo) => { const s = getComputedStyle(el, pseudo); return pseudo ? [s.content, (s.maskImage || s.webkitMaskImage)] : [s.backgroundColor, s.color, s.fontSize, s.fontWeight, s.borderTopLeftRadius, s.paddingInlineStart, s.paddingInlineEnd, s.lineHeight]; };
      const head = document.querySelector('#workspace-pins-group .browser-group-count'), total = document.querySelector('#workspace-sources .resource-count-total'), changed = document.querySelector('#workspace-sources .resource-count-changed'), mark = document.querySelector('#workspace-sources .resource-changed');
      const accent = document.createElement('span'); accent.className = 'badge accent'; accent.textContent = '3'; document.body.appendChild(accent);
      const out = { head: look(head), total: look(total), headBefore: look(head, '::before')[0], totalBefore: look(total, '::before')[0], headClass: head.className, changed: look(changed), accent: look(accent), changedGlyph: look(changed, '::before'), accentGlyph: look(accent, '::before'), mark: getComputedStyle(mark).getPropertyValue('--glyph'), changedVar: getComputedStyle(changed, '::before').getPropertyValue('--glyph') };
      accent.remove();
      return out;
    });
    ok('the total is the count badge the section heads have', styles.headClass === 'badge browser-group-count' && at(g.tree, 'Open Records').total.cls.indexOf('badge browser-group-count') === 0 && same(styles.head, styles.total) && styles.headBefore === 'none' && styles.totalBefore === 'none', JSON.stringify(styles));
    ok('the changed count is PUDL\'s accent badge with the changed mark\'s glyph', same(styles.changed, styles.accent) && styles.changedGlyph[0] !== 'none' && styles.changedGlyph[1] !== styles.accentGlyph[1] && styles.changedVar === styles.mark && styles.mark !== '', JSON.stringify(styles));

    // The places in a row: the status, the changed mark, the changed count
    // and the total, which ends where the marks end, before the row's
    // actions button.
    const both = at(g.tree, 'Shared with the Team');
    ok('counts and a status are both shown, in the order status, changed mark, changed count, total', both.badge && both.mark && both.changed.text === '17' && both.total.text === '128' && both.badge.right <= both.slot.x && both.slot.right <= both.changed.box.x && both.changed.box.right <= both.total.box.x, JSON.stringify(both));
    const counted = g.tree.filter(d => d.total || d.changed);
    ok('the last count ends at the row\'s end, before the row\'s actions button, in every row', counted.length === 7 && counted.every(d => { const last = (d.total || d.changed).box; return last.right === d.marks.right && last.right <= d.more.x && last.right === (at(g.tree, 'Open Records').total.box.right); }), JSON.stringify(counted.map(d => [(d.total || d.changed).box.right, d.more.x])));
    ok('a row with counts is as tall as a row without, and its name starts where theirs do', g.tree.filter(d => d.name !== 'Records' && d.name !== 'Working Files' && d.name !== 'notes' && d.name !== 'guide.md').every(d => d.row.height === at(g.tree, 'Not Read Yet').row.height && d.label.x === at(g.tree, 'Not Read Yet').label.x), JSON.stringify(g.tree.map(d => [d.row.height, d.label.x])));
    ok('no count is cut short', counted.every(d => (!d.total || d.total.whole) && (!d.changed || d.changed.whole)));
    ok('every row keeps its actions button in sight', await page.evaluate(() => Array.from(document.querySelectorAll('#workspace-sources li > [data-row-actions]')).every(b => { const s = getComputedStyle(b); return s.visibility === 'visible' && s.opacity === '1' && b.getBoundingClientRect().width > 0; })));

    // The Pinned list shows a pinned folder's counts.
    ok('a pinned folder shows its counts at the end of its line, before its actions button', at(g.pins, 'Open Records').total.text === '12' && at(g.pins, 'Open Records').changed.text === '3' && at(g.pins, 'Open Records').total.box.right <= at(g.pins, 'Open Records').more.x && at(g.pins, 'Open Records').total.name === '12 entries', JSON.stringify(at(g.pins, 'Open Records')));
    ok('a pinned folder whose counts are not known shows none and keeps no place, and its name starts where the others do', at(g.pins, 'Not Read Yet').marks === null && at(g.pins, 'Not Read Yet').label.x === at(g.pins, 'Open Records').label.x && at(g.pins, 'Not Read Yet').glyph.x === at(g.pins, 'Open Records').glyph.x, JSON.stringify(g.pins.map(d => [d.label.x, d.glyph.x])));
    ok('a pinned folder with a status and counts shows both', !!at(g.pins, 'Shared with the Team').badge && at(g.pins, 'Shared with the Team').total.text === '128' && at(g.pins, 'Shared with the Team').changed.text === '17');

    /* ---- The widths: narrow and wide, light and dark ----------------------- */
    const panel = width => page.evaluate(width => {
      const win = document.getElementById('workspace-sources').closest('.win');
      for (const name of ['width', 'min-width', 'max-width']) { if (width) win.style.setProperty(name, width + 'px', 'important'); else win.style.removeProperty(name); }
      if (width) win.style.setProperty('flex', 'none', 'important'); else win.style.removeProperty('flex');
    }, width);
    const shots = async name => {
      for (const theme of ['light', 'dark']) {
        await page.evaluate(theme => pudlSetTheme(theme), theme);
        await frame();
        await page.locator('#workspace-sources').screenshot({ path: require('path').join(OUT, 'resource-folder-counts-tree-' + name + '-' + theme + '.png') });
        await page.locator('#workspace-pins-group').screenshot({ path: require('path').join(OUT, 'resource-folder-counts-pins-' + name + '-' + theme + '.png') });
      }
      await page.evaluate(() => pudlSetTheme('light'));
    };
    // What must stay where it is in a row, measured from the row's own
    // edges, so that two widths of the panel can be compared.
    const fixed = list => list.map(d => ({ name: d.name, toggle: d.toggle && [d.toggle.x, d.toggle.width], glyph: d.glyph && d.glyph.x, label: d.label.x, y: d.row.y, height: d.row.height, more: d.more && [Math.round((d.row.right - d.more.x) * 10) / 10, d.more.width] }));
    await panel(520);
    await frame();
    const wide = await rows();
    ok('in a wide panel every count is drawn and the name of a folder is whole unless it is very long', wide.tree.filter(d => d.total).every(d => d.total.drawn) && !at(wide.tree, 'Open Records').cut && !at(wide.tree, 'Shared with the Team').cut && at(wide.tree, 'A saved query').cut, JSON.stringify(wide.tree.map(d => [d.name, d.cut])));
    await shots('wide');
    await panel(300);
    await frame();
    const narrow = await rows();
    ok('in a panel of ordinary width both counts are drawn, whole, and a name that is cut short keeps a dozen letters', narrow.tree.filter(d => d.total).every(d => d.total.drawn && d.total.whole) && narrow.tree.filter(d => d.changed).every(d => d.changed.whole) && narrow.tree.filter(d => (d.total || d.changed) && !d.badge).every(d => !d.cut || d.label.width >= 80) && narrow.pins.filter(d => d.total).every(d => d.total.drawn && d.total.whole && d.total.box.right <= d.more.x), JSON.stringify(narrow.tree.map(d => [d.name, d.label.width])));
    ok('in a panel of ordinary width a long name is cut short at the counts', at(narrow.tree, 'A saved query').cut && at(narrow.tree, 'A saved query').label.right <= at(narrow.tree, 'A saved query').changed.box.x);
    const statusRow = list => at(list, 'Shared with the Team');
    ok('in a panel of ordinary width a folder with a status and counts keeps its counts and its changed mark, and its status badge gives way and is still read', statusRow(narrow.tree).badge.width <= 2 && statusRow(narrow.tree).mark && statusRow(narrow.tree).total.drawn && statusRow(narrow.tree).total.box.right === at(narrow.tree, 'Nothing Here').total.box.right && statusRow(narrow.tree).label.width >= 50 && !statusRow(wide.tree).cut && statusRow(wide.tree).badge.width > 2 && await page.evaluate(() => { const b = document.querySelector('#workspace-sources .resource-marks-counted > .badge.resource-status'); return b.getAttribute('aria-label') === 'Status: In Review' && getComputedStyle(b).display !== 'none' && getComputedStyle(b).visibility === 'visible'; }), JSON.stringify(statusRow(narrow.tree)));
    await shots('narrow');
    await panel(250);
    await frame();
    const slim = await rows();
    const slimBoth = slim.tree.filter(d => d.total && d.changed), slimTotal = slim.tree.filter(d => d.total && !d.changed);
    ok('in a very narrow panel the total gives way where there are two counts, and the changed count stays whole, before the actions button', slimBoth.length === 3 && slimBoth.every(d => !d.total.drawn && d.changed.whole && d.changed.box.width > 10 && d.changed.box.right <= d.more.x && d.changed.box.right === at(slim.tree, 'Nothing Here').total.box.right), JSON.stringify(slimBoth.map(d => [d.total.box, d.changed.box])));
    ok('the total that gave way is still read, and the changed count\'s tooltip has it', slimBoth.every(d => d.total.name === d.total.text + ' entries' && d.total.role === 'img') && at(slim.tree, 'Open Records').changed.title === '3 changed since you last looked, of 12 entries', JSON.stringify(at(slim.tree, 'Open Records').changed));
    ok('in a very narrow panel a total that stands alone is still drawn', slimTotal.length === 3 && slimTotal.every(d => d.total.drawn && d.total.whole), JSON.stringify(slimTotal.map(d => d.total.box)));
    ok('in a very narrow panel a folder with a status still has its changed mark and its changed count, before the actions button', statusRow(slim.tree).badge.width <= 2 && statusRow(slim.tree).mark && statusRow(slim.tree).slot.right <= statusRow(slim.tree).changed.box.x && statusRow(slim.tree).changed.box.right <= statusRow(slim.tree).more.x, JSON.stringify(statusRow(slim.tree)));
    ok('in a very narrow panel a name that is cut short still keeps about ten letters beside its counts, and more than a few beside a changed mark too', slim.tree.filter(d => (d.total || d.changed) && !d.badge).every(d => !d.cut || d.label.width >= 64) && at(slim.tree, 'Shared with the Team').label.width >= 40, JSON.stringify(slim.tree.map(d => [d.name, d.label.width])));
    await shots('very-narrow');
    ok('at every width the toggle, the glyph, the name\'s start and the actions button are in the same places in a row', same(fixed(wide.tree).map(d => [d.toggle, d.glyph, d.label, d.more]), fixed(narrow.tree).map(d => [d.toggle, d.glyph, d.label, d.more])) && same(fixed(narrow.tree).map(d => [d.toggle, d.glyph, d.label, d.more]), fixed(slim.tree).map(d => [d.toggle, d.glyph, d.label, d.more])), JSON.stringify([fixed(wide.tree), fixed(slim.tree)]));

    /* ---- A count that arrives, changes or goes ----------------------------- */
    // Each width is tried: the counts that are not known become known, a
    // changed count comes and one goes, and a total grows by two figures.
    const arrive = () => page.evaluate(() => {
      window.__kept = JSON.stringify(__folders);
      const f = name => __folders.find(x => x.name === name);
      f('Q-3').count = 7; f('Q-3').changedCount = 1;      // not known, now known
      f('Q-2').changedCount = 5;                           // zero changed, now some
      f('Q-1').changedCount = 0; f('Q-1').count = 1234;    // some changed, now none, and a longer total
      f('Q-6').count = 30;                                 // the total arrives beside the changed count
      f('Q-4').count = 9;                                  // zero, now some
      window.__emit('workspace:changed', { kind: 'source-changed' });
    });
    const restore = () => page.evaluate(() => { const kept = JSON.parse(window.__kept); __folders.splice(0, __folders.length, ...kept); window.__emit('workspace:changed', { kind: 'source-changed' }); });
    const arrived = () => page.waitForFunction(() => { const row = Array.from(document.querySelectorAll('#workspace-sources a[data-resource]')).find(a => a.querySelector('.fm-ti-label').textContent === 'Not Read Yet'), pin = Array.from(document.querySelectorAll('#workspace-pins .browser-row')).find(r => r.querySelector('.browser-row-title').textContent === 'Not Read Yet'); return row && row.querySelector('.resource-count-total') && pin && pin.querySelector('.resource-count-total'); });
    const restored = () => page.waitForFunction(() => { const row = Array.from(document.querySelectorAll('#workspace-sources a[data-resource]')).find(a => a.querySelector('.fm-ti-label').textContent === 'Not Read Yet'), pin = Array.from(document.querySelectorAll('#workspace-pins .browser-row')).find(r => r.querySelector('.browser-row-title').textContent === 'Not Read Yet'); return row && !row.querySelector('.resource-marks') && pin && !pin.querySelector('.resource-marks'); });
    for (const width of [250, 300, 520]) {
      await panel(width);
      await frame();
      const was = await rows();
      await arrive();
      await arrived();
      await frame();
      const now = await rows();
      ok('at ' + width + ' px, counts that arrive, change and go are drawn', at(now.tree, 'Not Read Yet').total.text === '7' && at(now.tree, 'Not Read Yet').changed.text === '1' && at(now.tree, 'Waiting on Review').changed.text === '5' && at(now.tree, 'Open Records').changed === null && at(now.tree, 'Open Records').total.text === '1234' && at(now.tree, 'Changes Only').total.text === '30' && at(now.tree, 'Nothing Here').total.text === '9', JSON.stringify(now.tree.map(d => [d.name, d.total && d.total.text, d.changed && d.changed.text])));
      ok('at ' + width + ' px, a count that arrives or changes moves no toggle, no glyph, no name\'s start, no row and no actions button', same(fixed(was.tree), fixed(now.tree)) && same(fixed(was.pins), fixed(now.pins)), JSON.stringify([fixed(was.tree), fixed(now.tree)]));
      ok('at ' + width + ' px, the last count still ends where the marks ended, and no count is cut short', now.tree.filter(d => d.total || d.changed).every(d => { const drawn = [d.changed, d.total].filter(c => c && c.drawn), last = drawn[drawn.length - 1].box; return last.right === at(was.tree, 'Waiting on Review').total.box.right && drawn.every(c => c.whole); }), JSON.stringify(now.tree.map(d => [d.total && d.total.box.right, d.changed && d.changed.box.right])));
      ok('at ' + width + ' px, a status badge and a changed mark on a row whose counts did not change stay where they were', same([at(was.tree, 'Shared with the Team').badge, at(was.tree, 'Shared with the Team').slot], [at(now.tree, 'Shared with the Team').badge, at(now.tree, 'Shared with the Team').slot]));
      if (width === 300) await shots('arrived');
      await restore();
      await restored();
      await frame();
      ok('at ' + width + ' px, counts that are no longer known are gone and every row is as it was', same(was, await rows()));
    }
    await panel(0);
    await frame();

    /* ---- The Files window --------------------------------------------------- */
    await page.evaluate(() => dinahResources.openFiles('/workspaces/w1/records', { workspaceId: 'w1', sourceId: 'records', path: '' }));
    const filesWin = page.locator('.dinah-files').last();
    await filesWin.locator('.fm-row[data-name="Q-1"] .resource-count-total').waitFor();
    const fileRows = () => filesWin.evaluate(root => Array.from(root.querySelectorAll('.fm-row[data-name]')).map(r => {
      const left = r.getBoundingClientRect().x, x = el => el ? Math.round((el.getBoundingClientRect().x - left) * 10) / 10 : null, w = el => el ? Math.round(el.getBoundingClientRect().width * 10) / 10 : null;
      const total = r.querySelector('.resource-count-total'), changed = r.querySelector('.resource-count-changed'), place = r.querySelector('.resource-counts'), name = r.querySelector('.fm-name');
      return { name: r.dataset.name, y: r.getBoundingClientRect().y, height: r.getBoundingClientRect().height, nameX: x(name), nameY: name.getBoundingClientRect().y, total: total && total.textContent, totalName: total && total.getAttribute('aria-label'), totalX: x(total), changed: changed && changed.textContent, changedName: changed && changed.getAttribute('aria-label'), changedX: x(changed), place: w(place), placeX: x(place), marks: !!r.querySelector('.resource-marks'), badge: !!r.querySelector('.badge.resource-status') };
    }).concat([{ name: 'table', list: root.querySelector('.fm-list').getBoundingClientRect().width, column: root.querySelector('.fm-c-name').getBoundingClientRect().width }]));
    let f = await fileRows();
    const file = (list, name) => list.find(r => r.name === name);
    ok('Files draws a folder\'s counts after its name, with their accessible names', file(f, 'Q-1').total === '12' && file(f, 'Q-1').totalName === '12 entries' && file(f, 'Q-1').changed === '3' && file(f, 'Q-1').changedName === '3 changed since you last looked' && file(f, 'Q-1').totalX > file(f, 'Q-1').nameX, JSON.stringify(file(f, 'Q-1')));
    ok('in Files a folder whose counts are not known, and an entry that is not a folder, have none and keep no place', !file(f, 'Q-3').marks && !file(f, 'R-1').marks && file(f, 'Q-2').changed === null && file(f, 'Q-4').total === '0', JSON.stringify([file(f, 'Q-3'), file(f, 'R-1')]));
    ok('in Files the total comes first and the changed count after it, in a place of one width for every row that has counts', file(f, 'Q-1').totalX < file(f, 'Q-1').changedX && new Set(f.filter(r => r.place).map(r => r.place)).size === 1 && f.filter(r => r.place).length === 7, JSON.stringify(f.map(r => [r.name, r.place, r.totalX, r.changedX])));
    ok('in Files a row with counts starts its name where a row without them does, and is as tall', f.filter(r => r.name !== 'table').every(r => r.nameX === file(f, 'Q-3').nameX && Math.abs(r.height - file(f, 'Q-3').height) < 2), JSON.stringify(f.map(r => [r.nameX, r.height])));
    ok('in Files a folder with a status and counts shows both, the counts last', file(f, 'Q-5').badge && file(f, 'Q-5').total === '128' && file(f, 'Q-5').changed === '17');
    await t.shot('resource-folder-counts-files');
    const steady = list => list.map(r => r.name === 'table' ? r : [r.name, r.y, r.height, r.nameX, r.nameY, r.placeX, r.totalX]);
    await page.evaluate(() => {
      window.__kept = JSON.stringify(__folders);
      const f = name => __folders.find(x => x.name === name);
      f('Q-2').changedCount = 5; f('Q-1').changedCount = 0; f('Q-1').count = 1234; f('Q-4').count = 9;
      window.__emit('workspace:changed', { kind: 'source-changed' });
    });
    await filesWin.locator('.fm-row[data-name="Q-2"] .resource-count-changed').waitFor();
    await frame();
    const fAfter = await fileRows();
    ok('in Files a changed count that comes or goes and a total that grows move no row, no name and no total, and change no column\'s width', same(steady(f), steady(fAfter)) && file(fAfter, 'Q-1').total === '1234' && file(fAfter, 'Q-1').changed === null && file(fAfter, 'Q-2').changed === '5', JSON.stringify([steady(f), steady(fAfter)]));
    await page.evaluate(() => { const f = name => __folders.find(x => x.name === name); f('Q-3').count = 7; window.__emit('workspace:changed', { kind: 'source-changed' }); });
    await filesWin.locator('.fm-row[data-name="Q-3"] .resource-count-total').waitFor();
    await frame();
    const fKnown = await fileRows();
    ok('in Files a count that becomes known moves no row and no name', same(fAfter.filter(r => r.name !== 'table').map(r => [r.name, r.y, r.height, r.nameX, r.nameY]), fKnown.filter(r => r.name !== 'table').map(r => [r.name, r.y, r.height, r.nameX, r.nameY])) && file(fKnown, 'Q-3').total === '7', JSON.stringify(file(fKnown, 'Q-3')));
    await t.shot('resource-folder-counts-files-changed');
    await restore();
    await page.waitForFunction(() => { const roots = document.querySelectorAll('.dinah-files'); return !roots[roots.length - 1].querySelector('.fm-row[data-name="Q-3"] .resource-marks'); });
    await page.evaluate(() => pudlSetTheme('dark'));
    await frame();
    await t.shot('resource-folder-counts-files-dark');
    await page.evaluate(() => pudlSetTheme('light'));
  } finally { await t.close(); }
})();
