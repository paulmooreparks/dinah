const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1'), makeRequest('r2', { kind: 'mention', questions: [], text: 'Please read this group message.\nIts original wording stays available.' })]);
  const { page: p, ok } = t;
  const bounds = () => p.evaluate(() => ['[data-win="decisions"]', '.decisions-toolbar', '.decisions-status', '.decisions-list', '.decisions-detail', '.decisions-row'].map(selector => {
    const r = document.querySelector(selector).getBoundingClientRect(); return [r.x, r.y, r.width, r.height];
  }));
  const readingBounds = () => p.evaluate(() => ['.decisions-source', '.decisions-delivery', '.decisions-content'].map(selector => {
    const r = document.querySelector(selector).getBoundingClientRect(); return [r.x, r.y, r.width, r.height];
  }));
  try {
    await p.waitForFunction(() => document.querySelector('.decisions-detail').clientWidth >= 960);
    const initial = await bounds();
    ok('automatic docks give Decisions the reference reading width', initial[0][2] >= 1344 && initial[3][2] >= 340 && initial[4][2] >= 960, JSON.stringify(initial));
    const docks = await p.evaluate(() => pudlWindows.state().docks);
    ok('reading space leaves dock preferences automatic', ['left', 'right'].every(edge => !docks[edge] || docks[edge].mode === 'auto'));
    await choose(t, 'r1');
    const reading = await readingBounds();
    ok('selection keeps window, toolbar, list, reading pane and first row bounds', JSON.stringify(initial) === JSON.stringify(await bounds()));
    const order = await p.evaluate(() => {
      const form = document.querySelector('.decisions-detail form'), q = form.querySelector('.decisions-question');
      const before = (a, b) => !!(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);
      return form.querySelector('h2').textContent === 'Review r1' && before(form.querySelector('.decisions-source'), q) && before(form.querySelector('.decisions-delivery'), q) && before(q.querySelector('.decisions-question-text'), q.querySelector('.decisions-recommendation')) && before(q.querySelector('.decisions-recommendation'), q.querySelector('textarea'));
    });
    ok('the asking chat precedes the question, recommendation and answer', order);
    await p.getByLabel('Answer 1', { exact: true }).fill('Keep the explicit sending step.');
    await p.evaluate(item => { __decisionItems.unshift(item); __emit('bus:changed', {}); }, makeRequest('r3'));
    await p.locator('.decisions-row[data-request-id="r3"]').waitFor();
    ok('an arrival preserves all main bounds and the focused draft', JSON.stringify(initial) === JSON.stringify(await bounds()) && await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'Keep the explicit sending step.' && await p.getByLabel('Answer 1', { exact: true }).evaluate(el => el === document.activeElement));
    await t.shot('decisions-reading-draft');
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.evaluate(() => { window.__beforeFailure = true; });
    await p.getByRole('button', { name: 'Send Answers', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('unconfirmed') && !document.querySelector('[data-operation]').disabled);
    const recovery = await p.evaluate(() => {
      const panel = document.querySelector('.decisions-delivery'), pane = document.querySelector('.decisions-detail').getBoundingClientRect(), r = panel.getBoundingClientRect();
      const content = document.querySelector('.decisions-content').getBoundingClientRect();
      return panel.contains(document.querySelector('.decisions-receipt')) && panel.contains(document.querySelector('[data-operation]')) && r.top >= pane.top && r.bottom <= pane.bottom && content.top >= r.bottom;
    });
    ok('unconfirmed delivery and its recovery action stay in view without covering answers', recovery && JSON.stringify(reading) === JSON.stringify(await readingBounds()));
    await t.shot('decisions-reading-error');
    await p.getByRole('button', { name: 'Requests', exact: true }).click(); await choose(t, 'r2');
    ok('selecting a plain mention keeps source, delivery and scrolling question bounds', JSON.stringify(reading) === JSON.stringify(await readingBounds()));
    ok('the plain mention keeps its origin and original message', await p.locator('.decisions-source').innerText().then(text => text.includes('Review r2') && text.includes('Group Mention') && text.includes('Review Group')) && (await p.locator('.decisions-question-text').innerText()).includes('Its original wording stays available.'));
    await t.shot('decisions-reading-mention');
    await p.evaluate(() => { pudlWindows.dockMode('left', 'open'); pudlWindows.dockMode('right', 'open'); });
    await p.waitForFunction(() => document.querySelector('.decisions').clientWidth <= 740);
    ok('pinned-open docks are respected without horizontal overflow', await p.locator('.decisions-detail').evaluate(el => el.scrollWidth <= el.clientWidth + 1) && await p.locator('.decisions-content').evaluate(el => el.scrollWidth <= el.clientWidth + 1) && await p.evaluate(() => ['left', 'right'].every(edge => pudlWindows.state().docks[edge].mode === 'open')));
    await p.evaluate(() => { pudlWindows.dockMode('left', 'auto'); pudlWindows.dockMode('right', 'auto'); });
    await p.setViewportSize({ width: 390, height: 844 });
    await p.waitForFunction(() => getComputedStyle(document.querySelector('.decisions-list')).display === 'none');
    ok('the narrow reading pane fits its source and delivery controls', await p.locator('.decisions-detail').evaluate(el => el.scrollWidth <= el.clientWidth + 1) && await p.locator('.decisions-content').evaluate(el => el.scrollWidth <= el.clientWidth + 1) && await p.locator('.decisions-delivery').evaluate(el => el.scrollWidth <= el.clientWidth + 1));
    await t.shot('decisions-reading-narrow');
    await p.setViewportSize({ width: 1440, height: 1000 });
    await p.evaluate(() => pudlWindows.close('decisions'));
    await p.waitForFunction(() => !document.querySelector('.win-dock-group[data-dock-rail]'));
    ok('automatic side docks return when Decisions closes', true);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
