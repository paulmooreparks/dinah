const { start, copy, check, removals, ready, finish } = require('./lib/set-aside-fixture');
(async () => {
  const t = await start({ items: [copy(1), copy(2)] }); const { page: p, ok } = t;
  try {
    await check(t, 1).check(); await check(t, 2).check();
    await p.locator('#set-aside-remove').click(); await p.locator('#ask[open]').waitFor();
    ok('confirmation names the selected copies and total before any removal', (await p.locator('#ask').innerText()).includes(copy(1).name) && (await p.locator('#ask').innerText()).includes('3,072 bytes') && (await removals(t)).length === 0 && await check(t, 1).isDisabled());
    await p.locator('#ask-no').click();
    ok('cancellation retains selection and removes nothing', (await removals(t)).length === 0 && await check(t, 1).isChecked());
    await p.evaluate(() => { __asideItems[1].canRemove = false; __asideItems[1].runningPids = [456]; __asideItems[1].reasonCode = 'in_use'; __asideItems[1].reason = 'A running process now uses this copy.'; __asideHold.remove = true; });
    await p.locator('#set-aside-remove').click(); await p.locator('#ask-yes').click();
    await p.waitForFunction(() => !!window.__asideReleaseRemove);
    ok('a pending removal gates selection and refresh', await check(t, 1).isDisabled() && await p.locator('#set-aside-refresh').getAttribute('aria-disabled') === 'true');
    await p.evaluate(() => document.querySelector('#set-aside-remove').click());
    ok('pending work cannot submit another removal', (await removals(t)).length === 1);
    await p.evaluate(() => { __asideHold.remove = false; __asideReleaseRemove(); });
    await p.waitForFunction(() => document.querySelector('#set-aside-results').textContent.includes('A running process now'));
    await ready(t);
    ok('only the confirmed opaque identities are sent', JSON.stringify((await removals(t))[0][1]) === JSON.stringify({ ids: [copy(1).id, copy(2).id] }));
    ok('mixed results show each refusal and retained rows', (await p.locator('#set-aside-results').innerText()).includes('Removed.') && (await p.locator('#set-aside-results').innerText()).includes('1,024 bytes') && await p.locator('#set-aside-list > div').count() === 2 && await check(t, 2).isDisabled() && (await p.locator('#set-aside-total').innerText()).startsWith('0 selected'));
    await t.shot('settings-set-aside-results');
  } finally { await finish(t, 'set-aside-remove'); }
})().catch(error => { console.error(error); process.exitCode = 1; });
