const { start, copy, check, removals, ready, finish } = require('./lib/set-aside-fixture');
(async () => {
  const t = await start({ holdList: true }); const { page: p, ok } = t;
  try {
    await p.waitForFunction(() => !!window.__asideReleaseList);
    ok('loading disables removal without collapsing the list', await p.locator('#set-aside-remove').getAttribute('aria-disabled') === 'true' && (await p.locator('#set-aside-list').boundingBox()).height >= 280);
    await p.evaluate(() => { __asideHold.list = false; __asideReleaseList(); }); await ready(t);
    await check(t, 1).check();
    await p.evaluate(() => { __asideLostResponse = true; });
    await p.locator('#set-aside-remove').click(); await p.locator('#ask-yes').click();
    await p.waitForFunction(() => document.querySelector('#set-aside-results').textContent.includes('completion is unconfirmed')); await ready(t);
    ok('a lost response refreshes without retrying or claiming removal', (await removals(t)).length === 1 && await check(t, 1).isDisabled() && (await p.locator('#set-aside-total').innerText()).startsWith('0 selected'));
    await p.evaluate(item => { __asideItems.push(item); __asideLostResponse = false; __asideMalformedRemoval = true; }, copy(4));
    await p.locator('#set-aside-refresh').click(); await ready(t); await check(t, 4).check();
    await p.locator('#set-aside-remove').click(); await p.locator('#ask-yes').click();
    await p.waitForFunction(() => document.querySelector('#set-aside-results').textContent.includes('result is invalid')); await ready(t);
    ok('a malformed result also stays unconfirmed', (await removals(t)).length === 2 && await check(t, 4).isDisabled());
    await p.evaluate(() => { __asideMalformedList = true; }); await p.locator('#set-aside-refresh').click(); await ready(t);
    ok('malformed inventory fails closed with the prior rows retained', (await p.locator('#set-aside-status').innerText()).includes('inventory is invalid') && await p.locator('#set-aside-remove').getAttribute('aria-disabled') === 'true' && await p.locator('#set-aside-list > div').count() === 4);
    await p.evaluate(() => { __asideMalformedList = false; __asideFail.list = true; }); await p.locator('#set-aside-refresh').click(); await ready(t);
    ok('service failure stays visible and gated', (await p.locator('#set-aside-status').innerText()).includes('Synthetic inventory failure') && await check(t, 2).isDisabled());
  } finally { await finish(t, 'set-aside-errors'); }
  const limit = await start({ items: Array.from({ length: 101 }, (_, i) => copy(i + 1)) });
  try {
    await limit.page.evaluate(() => Array.from(document.querySelectorAll('#set-aside-list input')).slice(0, 100).forEach(el => el.click()));
    limit.ok('a batch stops at one hundred explicit identities', await check(limit, 101).isDisabled() && (await limit.page.locator('#set-aside-total').innerText()).startsWith('100 selected'));
  } finally { await finish(limit, 'set-aside-limit'); }
})().catch(error => { console.error(error); process.exitCode = 1; });
