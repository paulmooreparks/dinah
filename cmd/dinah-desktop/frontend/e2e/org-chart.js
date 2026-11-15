const { start } = require('./lib/org-chart-fixture');
(async () => {
  const t = await start(), p = t.page, ok = t.ok, chart = t.chart;
  try {
    ok('Org Chart is reachable from the Window menu', await p.evaluate(() => dinahCommands.rows('window').some(row => row.label === 'Org Chart')));
    ok('logical chats appear once despite native session references', await chart.locator('[data-org-chat]').evaluateAll(rows => new Set(rows.map(row => row.dataset.orgChat)).size === rows.length) && await chart.locator('[data-org-chat="mara"]').count() === 1);
    ok('Diagrams draws the reporting and dotted group connections', await chart.locator('svg path').count() > 0 && await chart.locator('svg [stroke-dasharray]').count() > 0);
    ok('folded teams use raised controls and hide their descendants', await chart.locator('[data-org-fold="research"]').evaluate(el => el.matches('button.btn') && el.getAttribute('aria-expanded') === 'false') && await chart.locator('[data-org-chat="bo"]').count() === 0);
    ok('repair entries explain the problem and offer no management menu', await chart.locator('[data-org-chat="repair"] [data-row-actions]').count() === 0 && (await chart.locator('[data-org-chat="repair"]').getAttribute('aria-label')).includes('Needs attention'));
    await chart.locator('[data-org-chat="nina"]').hover();
    await chart.locator('[data-org-chat="nina"] [data-row-actions]').click();
    await p.locator('.context-menu:popover-open, .menu-panel:popover-open').last().waitFor();
    ok('management actions are disabled and explain that they are coming next', await p.locator('.context-menu:popover-open, .menu-panel:popover-open').last().evaluate(el => el.textContent.includes('Coming next') && el.querySelectorAll('.menu-action').length === 6 && Array.from(el.querySelectorAll('.menu-action')).every(item => item.getAttribute('aria-disabled') === 'true')));
    await p.keyboard.press('Escape');
    await chart.locator('[data-org-chat="nina"]').focus(); await p.keyboard.press('Shift+F10');
    ok('the chat context menu is available from the keyboard', await p.locator('.context-menu:popover-open, .menu-panel:popover-open').count() === 1); await p.keyboard.press('Escape');
    await chart.locator('[data-org-fold="research"]').click(); await chart.locator('[data-org-chat="bo"]').waitFor();
    ok('unfolding preserves one box for every chat', await chart.locator('[data-org-chat]').count() === 11);
    await chart.getByRole('searchbox', { name: 'Find a Chat' }).fill('Nina');
    ok('Find a Chat retains the reporting context of a match', await chart.locator('[data-org-chat="nina"]').count() === 1 && await chart.locator('[data-org-chat="mara"]').count() === 1 && await chart.locator('[data-org-chat="lena"]').count() === 0);
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
      ok(theme + ' chart at ' + width + ' fits its reading surface', await chart.evaluate(el => el.scrollWidth <= el.clientWidth + 1) && await chart.locator('.org-chart-chat').evaluateAll(rows => rows.every(el => el.clientHeight === 100 && el.scrollWidth <= el.clientWidth + 1)));
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
      await t.shot('org-chart-' + theme + '-' + width);
      await chart.locator('[data-org-group="review"]').scrollIntoViewIfNeeded();
      await t.shot('org-chart-groups-' + theme + '-' + width);
      await chart.locator('.org-chart-scroll').evaluate(el => { el.scrollTop = 0; });
    }
    await p.evaluate(() => { __orgError = 'The chart could not be read.'; __emit('bus:changed', {}); }); await chart.getByRole('status').getByText('The chart could not be read.', { exact: true }).waitFor();
    ok('a failed read keeps the previous chart visible', await chart.locator('[data-org-chat="nina"]').count() === 1);
    await p.evaluate(() => { __orgError = ''; __orgGate = new Promise(resolve => { window.__releaseOrg = resolve; }); __emit('bus:changed', {}); __emit('chat:restored', { sessionId: 'mara-current' }); });
    const reads = await p.evaluate(() => __orgReads); await p.evaluate(() => __releaseOrg());
    await p.waitForFunction(reads => __orgReads === reads + 1, reads);
    ok('refreshes are serialized and coalesced after the current read', await p.evaluate(reads => __orgReads === reads + 1, reads));
    const baseline = await t.calls();
    await p.evaluate(() => __emit('chat:removed', { sessionId: 'mara-current' }));
    await p.waitForFunction(reads => __orgReads > reads + 1, reads);
    const extra = (await t.calls()).slice(baseline.length);
    ok('chart refresh does not discover history or open a chat', extra.every(call => !/Sessions|OpenSession|AgentDirectory/.test(call[0])));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
