const { setup, makeRequest } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([], { entryPoints: true, noOpen: true }); const { page: p, ok } = t;
  try {
    await p.waitForFunction(() => document.querySelector('#status-decisions').dataset.statusCountKnown !== undefined);
    const rects = () => p.evaluate(() => ['status-decisions', 'status-needs', 'status-agent'].map(id => document.getElementById(id).getBoundingClientRect().toJSON()));
    const initial = await rects();
    ok('empty count is visible without opening Decisions', await p.locator('#status-decisions [data-status-badge]').innerText() === '0' && await p.locator('.decisions').count() === 0);
    for (const count of [1, 99, 100, 103]) {
      await p.evaluate(items => { __decisionItems = items; __emit('bus:changed', {}); }, Array.from({ length: count }, (_, i) => makeRequest('opaque-' + (count - i))));
      await p.waitForFunction(n => document.querySelector('#status-decisions').dataset.statusCount === String(n), count);
      ok('count ' + count + ' preserves the status geometry', JSON.stringify(await rects()) === JSON.stringify(initial));
    }
    ok('large count has a capped badge and full accessible count', await p.locator('#status-decisions [data-status-badge]').innerText() === '99+' && (await p.locator('#status-decisions').getAttribute('aria-label')).includes('103 open requests'));
    await p.locator('#status-decisions').click(); await p.locator('.decisions').waitFor();
    ok('status opens Decisions', true);
    await p.evaluate(() => { __entriesFailure = true; __emit('bus:changed', {}); });
    await p.waitForFunction(() => document.querySelector('#status-decisions [data-status-badge]').textContent === '?');
    ok('read failure is unknown rather than a zero count', await p.locator('#status-decisions').evaluate(e => !e.hasAttribute('data-status-count-known')) && JSON.stringify(await rects()) === JSON.stringify(initial));
    await p.evaluate(() => { __entriesFailure = false; __malformedEntries = true; __emit('bus:changed', {}); });
    await p.evaluate(() => dinahDecisionEntries.refresh().catch(() => {}));
    ok('malformed complete inventory fails closed', await p.locator('#status-decisions [data-status-badge]').innerText() === '?');
    await p.evaluate(() => { __malformedEntries = false; __holdEntries = true; window.__read = dinahDecisionEntries.refresh(); });
    await p.waitForFunction(() => window.__releaseEntries);
    await p.evaluate(() => { __decisionItems = []; __emit('bus:changed', {}); __holdEntries = false; __releaseEntries(); });
    await p.evaluate(() => __read);
    ok('a bus change invalidates an in-flight stale count', await p.locator('#status-decisions [data-status-badge]').innerText() === '0');
  } finally { await t.close(); }
  const absent = await setup([], { noOpen: true });
  try { absent.ok('older hosts without the query have no status entry', await absent.page.locator('#status-decisions').isHidden()); } finally { await absent.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
