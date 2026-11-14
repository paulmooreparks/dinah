const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1'), makeRequest('r2')]); const { page: p, ok } = t;
  try {
    await choose(t, 'r1'); await p.getByLabel('Answer 1', { exact: true }).fill('A mobile draft.');
    await p.setViewportSize({ width: 390, height: 844 });
    await p.waitForFunction(() => getComputedStyle(document.querySelector('.decisions-list')).display === 'none');
    ok('a narrow window shows the selected request at usable width', await p.locator('.decisions-detail').isVisible() && await p.locator('.decisions-detail').evaluate(el => el.scrollWidth <= el.clientWidth + 1));
    await p.getByRole('button', { name: 'Requests', exact: true }).click(); await p.locator('.decisions-list').waitFor({ state: 'visible' }); await choose(t, 'r1');
    ok('returning through the list preserves the answer', await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'A mobile draft.');
    await t.shot('decisions-narrow');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
