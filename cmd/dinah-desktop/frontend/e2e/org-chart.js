const { start } = require('./lib/org-chart-fixture');
(async () => {
  const t = await start(), p = t.page, ok = t.ok, chart = t.chart;
  try {
    ok('Org Chart is reachable from the Window menu', await p.evaluate(() => dinahCommands.rows('window').some(row => row.label === 'Org Chart')));
    ok('the live chart has no manual refresh control', await chart.getByRole('button', { name: 'Refresh', exact: true }).count() === 0 && !await chart.locator('button').filter({ hasText: 'Retry Loading' }).isVisible());
    ok('logical chats appear once despite native session references', await chart.locator('[data-org-chat]').evaluateAll(rows => new Set(rows.map(row => row.dataset.orgChat)).size === rows.length) && await chart.locator('[data-org-chat="mara"]').count() === 1);
    ok('Diagrams draws the reporting and dotted group connections', await chart.locator('svg path').count() > 0 && await chart.locator('svg [stroke-dasharray]').count() > 0);
    ok('folded teams use raised controls and hide their descendants', await chart.locator('[data-org-fold="research"]').evaluate(el => el.matches('button.btn') && el.getAttribute('aria-expanded') === 'false') && await chart.locator('[data-org-chat="bo"]').count() === 0);
    ok('repair entries explain the problem and offer no management menu', await chart.locator('[data-org-chat="repair"] [data-row-actions]').count() === 0 && (await chart.locator('[data-org-chat="repair"]').getAttribute('aria-label')).includes('Needs attention'));
    await chart.locator('[data-org-chat="nina"]').hover();
    await chart.locator('[data-org-chat="nina"] [data-row-actions]').click();
    await p.locator('.context-menu:popover-open, .menu-panel:popover-open').last().waitFor();
    await p.locator('.context-menu:popover-open, .menu-panel:popover-open').last().getByText('Unavailable in this fixture.').first().waitFor();
    ok('unavailable management actions use the host reason', await p.locator('.context-menu:popover-open, .menu-panel:popover-open').last().evaluate(el => el.querySelectorAll('.menu-action').length === 6 && Array.from(el.querySelectorAll('.menu-action')).every(item => item.getAttribute('aria-disabled') === 'true')));
    await p.keyboard.press('Escape');
    await chart.locator('[data-org-chat="nina"]').focus(); await p.keyboard.press('Shift+F10');
    ok('the chat context menu is available from the keyboard', await p.locator('.context-menu:popover-open, .menu-panel:popover-open').count() === 1); await p.keyboard.press('Escape');
    await chart.locator('[data-org-fold="research"]').click(); await chart.locator('[data-org-chat="bo"]').waitFor();
    ok('unfolding preserves one box for every chat', await chart.locator('[data-org-chat]').count() === 11);
    await chart.getByRole('searchbox', { name: 'Find a Chat' }).fill('Nina');
    ok('Find a Chat shows the same matching chats as the Chats panel', await chart.locator('[data-org-chat="nina"]').count() === 1 && await chart.locator('[data-org-chat="mara"]').count() === 0 && await chart.locator('[data-org-chat="lena"]').count() === 0);
    ok('the filter has an address', await p.evaluate(() => location.search.includes('view.') && decodeURIComponent(location.search).includes('Nina')));
    await p.reload(); await chart.getByRole('searchbox', { name: 'Find a Chat' }).waitFor();
    await p.waitForFunction(() => document.querySelector('.org-chart-canvas').dataset.generation);
    ok('reloading the address restores its filter and chart', await chart.getByRole('searchbox').inputValue() === 'Nina' && await chart.locator('[data-org-chat="nina"]').count() === 1 && await chart.locator('[data-org-chat="lena"]').count() === 0);
    await chart.getByRole('searchbox').fill(''); await chart.getByRole('combobox', { name: 'Workspace' }).selectOption('C:/Orchard');
    ok('workspace filtering leaves out other workspaces', await chart.locator('[data-org-chat="mara"]').count() === 0 && await chart.locator('[data-org-chat="uma"]').count() === 1);
    await chart.getByRole('combobox').selectOption('');
    await p.evaluate(() => { __orgModel.teams.push({ id: 'nested', title: 'Reviewers', leadId: 'nina', memberIds: ['cal'], resolution: 'resolved', reason: '' }); __orgModel.generation = 'fixture:nested'; __emit('bus:changed', {}); });
    await chart.locator('[data-org-fold="nested"]').waitFor();
    ok('nested teams retain one box for the shared lead and member', await chart.locator('[data-org-chat="nina"]').count() === 1 && await chart.locator('[data-org-chat="cal"]').count() === 1);
    for (const width of [1440, 390]) for (const theme of ['light', 'dark']) {
      await p.setViewportSize({ width, height: 1000 }); await p.evaluate(theme => document.documentElement.dataset.theme = theme, theme);
      await p.waitForFunction(() => { const el = document.querySelector('.org-chart-scroll'); return el && el.scrollWidth <= el.clientWidth + 1; });
      ok(theme + ' chart at ' + width + ' fits its reading surface', await chart.evaluate(el => el.scrollWidth <= el.clientWidth + 1) && await chart.locator('.org-chart-chat').evaluateAll(rows => rows.every(el => el.clientHeight === 100 && el.clientWidth >= 120 && el.scrollWidth <= el.clientWidth + 1)));
      const crossings = await chart.evaluate(root => {
        const boxes = Array.from(root.querySelectorAll('.org-chart-chat,.org-chart-group,.org-chart-fold')).map(el => ({ id: el.dataset.orgChat || el.dataset.orgGroup || el.dataset.orgFold, r: el.getBoundingClientRect() })), hits = [];
        root.querySelectorAll('svg [data-edge] path').forEach(path => {
          const matrix = path.getScreenCTM(), length = path.getTotalLength();
          for (let distance = 2; distance < length; distance += 4) {
            const point = path.getPointAtLength(distance).matrixTransform(matrix), box = boxes.find(({ r }) => point.x > r.left + 2 && point.x < r.right - 2 && point.y > r.top + 2 && point.y < r.bottom - 2);
            if (box) { hits.push([path.parentElement.dataset.edge, box.id]); break; }
          }
        }); return hits;
      });
      ok(theme + ' connectors at ' + width + ' never cross a chat, group or fold control', crossings.length === 0, JSON.stringify(crossings));
      const groupGeometry = await chart.evaluate(root => {
        const frames = Array.from(root.querySelectorAll('svg [data-node^="frame:"]')).map(el => el.getBoundingClientRect()), overlap = [], interiors = [], shared = [], endpoints = [], connected = new Set();
        function point(path, distance) { return path.getPointAtLength(distance).matrixTransform(path.getScreenCTM()); }
        const runs = [];
        const paths = Array.from(root.querySelectorAll('svg [data-edge] path[stroke-dasharray]'));
        paths.forEach(path => {
          const a = point(path, 0), b = point(path, path.getTotalLength());
          const route = path.parentElement.dataset.edge.split(':').slice(0, 3).join(':'), groupKey = route.split(':')[1];
          const sourcePath = root.querySelector('svg [data-edge="' + CSS.escape(route + ':start') + '"] path'), origin = point(sourcePath, 0);
          const horizontal = Math.abs(a.y - b.y) < 0.1;
          for (let distance = 2; distance < path.getTotalLength(); distance += 3) {
            const q = point(path, distance);
            if (frames.some(f => q.x > f.left + 2 && q.x < f.right - 2 && q.y > f.top + 2 && q.y < f.bottom - 2 && !(horizontal && Math.abs(q.y - origin.y) < 0.2 && origin.x >= f.left && origin.x <= f.right && origin.y > f.top && origin.y < f.bottom))) { interiors.push(route); break; }
          }
          runs.forEach(r => {
            if (r.groupKey === groupKey || r.horizontal !== horizontal) return;
            const parallel = horizontal ? Math.abs(a.y - r.a.y) < 15.9 : Math.abs(a.x - r.a.x) < 15.9;
            const extent = horizontal ? Math.min(Math.max(a.x,b.x),Math.max(r.a.x,r.b.x))-Math.max(Math.min(a.x,b.x),Math.min(r.a.x,r.b.x)) : Math.min(Math.max(a.y,b.y),Math.max(r.a.y,r.b.y))-Math.max(Math.min(a.y,b.y),Math.min(r.a.y,r.b.y));
            if (parallel && extent > 2) shared.push([route,r.route]);
          });
          runs.push({a,b,horizontal,groupKey,route});
          if (frames.some(f => Math.abs(a.x - b.x) < 0.1 ? (Math.abs(a.x - f.left) < 2 || Math.abs(a.x - f.right) < 2) && Math.min(Math.max(a.y, b.y), f.bottom) - Math.max(Math.min(a.y, b.y), f.top) > 2 : Math.abs(a.y - b.y) < 0.1 && (Math.abs(a.y - f.top) < 2 || Math.abs(a.y - f.bottom) < 2) && Math.min(Math.max(a.x, b.x), f.right) - Math.max(Math.min(a.x, b.x), f.left) > 2)) overlap.push(path.parentElement.dataset.edge);
          const id = path.parentElement.dataset.edge; if (!id.endsWith(':end')) return;
          const start = root.querySelector('svg [data-edge="' + CSS.escape(id.slice(0, -4) + ':start') + '"] path');
          const chat = Array.from(root.querySelectorAll('[data-org-chat]')).find(el => { const r = el.getBoundingClientRect(); return (Math.abs(origin.x - r.right) < 2 || Math.abs(origin.x - r.left) < 2) && origin.y > r.top && origin.y < r.bottom; });
          const target = Array.from(root.querySelectorAll('.org-chart-group')).find(el => { const r = el.getBoundingClientRect(); return Math.abs(b.x - r.right) < 2 && Math.abs(b.y - r.y - r.height / 2) < 2; });
          const group = target && __orgModel.groups.find(g => g.id === target.dataset.orgGroup), member = chat && chat.dataset.orgChat;
          const valid = group && member && group.memberIds.concat(group.ownerId || []).includes(member) && target.checkVisibility() && target.querySelector('strong').textContent === group.title;
          endpoints.push(!!valid); if (valid) connected.add(group.id + ':' + member);
        });
        const expected = __orgModel.groups.flatMap(g => Array.from(new Set(g.memberIds.concat(g.ownerId || []))).map(id => g.id + ':' + id));
        return { overlap, interiors, shared, solid: root.querySelectorAll('svg [data-node^="frame:"] [stroke-dasharray]').length === 0, endpoints, complete: expected.every(id => connected.has(id)) };
      });
      ok(theme + ' group segments at ' + width + ' stay clear of quiet solid container edges', groupGeometry.solid && groupGeometry.overlap.length === 0, JSON.stringify(groupGeometry));
      ok(theme + ' group routes at ' + width + ' never traverse panel interiors or share another group lane', groupGeometry.interiors.length === 0 && groupGeometry.shared.length === 0, JSON.stringify(groupGeometry));
      ok(theme + ' group lines at ' + width + ' end at their labelled group boxes', groupGeometry.complete && groupGeometry.endpoints.length > 0 && groupGeometry.endpoints.every(Boolean), JSON.stringify(groupGeometry));
      await t.shot('org-chart-' + theme + '-' + width);
      await chart.locator('[data-org-group="review"]').scrollIntoViewIfNeeded();
      await t.shot('org-chart-groups-' + theme + '-' + width);
      await chart.locator('.org-chart-scroll').evaluate(el => { el.scrollTop = 0; });
    }
    await p.evaluate(() => { __orgError = 'The chart could not be read.'; __emit('bus:changed', {}); }); await chart.getByRole('status').getByText('The chart could not be read.', { exact: true }).waitFor();
    ok('a failed read keeps the previous chart visible', await chart.locator('[data-org-chat="nina"]').count() === 1);
    ok('only an error reveals Retry Loading', await chart.getByRole('button', { name: 'Retry Loading', exact: true }).isVisible());
    await p.evaluate(() => { __orgError = ''; __orgGate = new Promise(resolve => { window.__releaseOrg = resolve; }); });
    const reads = await p.evaluate(() => __orgReads);
    await chart.getByRole('button', { name: 'Retry Loading', exact: true }).click();
    await p.waitForFunction(reads => __orgReads === reads + 1, reads);
    await p.evaluate(() => __emit('chat:restored', { sessionId: 'mara-current' }));
    await p.evaluate(() => __releaseOrg());
    // Retry first refreshes the common inventory, then reads relationships.
    // The event during that work contributes one coalesced relationship read.
    await p.waitForFunction(reads => __orgReads === reads + 2 && document.querySelector('.org-chart button[aria-hidden="true"]'), reads);
    ok('refreshes are serialized and coalesced after the current read', await p.evaluate(reads => __orgReads === reads + 2, reads));
    const baseline = await t.calls();
    const beforeRemoval = await p.evaluate(() => __orgReads);
    await p.evaluate(() => __emit('chat:removed', { sessionId: 'mara-current' }));
    await p.waitForFunction(reads => __orgReads > reads, beforeRemoval);
    const extra = (await t.calls()).slice(baseline.length);
    ok('chart refresh does not discover history or open a chat', extra.every(call => !/Sessions|OpenSession|AgentDirectory/.test(call[0])));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
