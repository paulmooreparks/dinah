const { start, card, quiet } = require('./lib/whats-new-fixture');
(async () => {
  const t = await start({ later: true }); const { page: p, ok } = t;
  const geometry = () => p.evaluate(() => ['[data-notification-id="earlier-report"]', '.needs-filters', '#needs-empty', '#questions', '.win-bar', '#status-needs', '#status-agent'].map(selector => { const r = document.querySelector(selector).getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; }));
  try {
    await p.locator('#questions [data-notification-id="earlier-report"]').waitFor();
    const before = await geometry();
    await p.evaluate(() => { __featureStore.push(__feature); sessionStorage.setItem('whats-new-items', JSON.stringify(__featureStore)); __emit('notification:delivery', {}); });
    await card(t).waitFor();
    ok('arrival leaves existing cards, filters, pane and taskbar bounds unchanged', JSON.stringify(before) === JSON.stringify(await geometry()));
    ok('the feature card is appended after activity', await card(t).evaluate(el => el === el.parentElement.lastElementChild));
    const featureBounds = await card(t).boundingBox();
    await p.evaluate(() => { __emit('notification:delivery', {}); __emit('notification:delivery', {}); });
    await p.waitForFunction(() => document.querySelectorAll('.whats-new-card').length === 1);
    ok('repeated restoration keeps one card', await card(t).count() === 1);
    await p.evaluate(() => { __featureUpdates = true; __emit('notification:delivery', {}); });
    await card(t).getByRole('button', { name: 'Open Updates', exact: true }).waitFor();
    ok('late capability discovery does not move existing regions', JSON.stringify(before) === JSON.stringify(await geometry()));
    ok('the optional Updates link keeps the feature card bounds fixed', JSON.stringify(featureBounds) === JSON.stringify(await card(t).boundingBox()));
    await p.evaluate(() => { __featureDiscoveryFailure = true; __emit('notification:delivery', {}); });
    await card(t).getByRole('button', { name: 'Open Updates', exact: true }).waitFor({ state: 'detached' });
    ok('discovery failure leaves the other links and geometry usable', await card(t).locator('.notice-actions > button').count() === 5 && JSON.stringify(before) === JSON.stringify(await geometry()));
    await p.evaluate(() => { __featureDiscoveryFailure = false; __emit('notification:delivery', {}); });
    await card(t).getByRole('button', { name: 'Open Updates', exact: true }).waitFor();
    await t.shot('whats-new-desktop');
    await card(t).getByRole('button', { name: 'Dismiss', exact: true }).click(); await card(t).waitFor({ state: 'detached' });
    ok('dismissal leaves earlier activity and neighbours fixed', JSON.stringify(before) === JSON.stringify(await geometry()));
  } finally { await quiet(t); }
  const narrow = await start();
  try {
    await card(narrow).waitFor(); await narrow.page.evaluate(() => pudlWindows.dock('needs-you', null));
    await narrow.page.evaluate(() => pudlWindows.snap('needs-you', 'maximized'));
    await narrow.page.setViewportSize({ width: 390, height: 844 });
    await narrow.page.waitForFunction(() => document.querySelector('[data-win="needs-you"]').getBoundingClientRect().right <= innerWidth);
    narrow.ok('narrow feature content scrolls without horizontal overflow', await card(narrow).evaluate(el => el.scrollWidth <= el.clientWidth + 1));
    await narrow.shot('whats-new-narrow');
  } finally { await quiet(narrow); }
  const pending = await start({ hold: true });
  try {
    const p = pending.page;
    await p.locator('#questions [data-notification-id="earlier-report"]').waitFor();
    pending.ok('slow feature discovery does not delay existing notifications', await card(pending).count() === 0 && await p.locator('#status-needs').evaluate(el => el.hasAttribute('data-status-count-known')));
    await p.evaluate(() => { __featureHold = false; __featureRelease(); }); await card(pending).waitFor();
    await p.evaluate(() => { __featureHold = true; __emit('notification:delivery', {}); });
    await p.waitForFunction(() => __featurePending.length > 0);
    await card(pending).getByRole('button', { name: 'Dismiss', exact: true }).click(); await card(pending).waitFor({ state: 'detached' });
    await p.evaluate(() => { __featureHold = false; __featureRelease(); });
    await p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    pending.ok('a delayed discovery cannot resurrect a dismissed card', await card(pending).count() === 0);
  } finally { await quiet(pending); }
})().catch(error => { console.error(error); process.exitCode = 1; });
