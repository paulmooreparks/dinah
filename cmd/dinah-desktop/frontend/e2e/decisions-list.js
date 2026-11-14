const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r1'), makeRequest('r2', { kind: 'mention', questions: [], text: 'Please review this message.\nMore detail.' })]);
  const { page: p, ok } = t;
  try {
    await choose(t, 'r1');
    ok('numbered questions show recommendations and optional choices', await p.locator('.decisions-question').count() === 2 && (await p.locator('.decisions-detail').innerText()).includes('Use the reviewed release.') && await p.getByLabel('Suggested Answers 1').count() === 1);
    ok('the destination is visible before sending', (await p.locator('.decisions-destination').innerText()).includes('Review Group, replying to Review r1'));
    const row = await p.locator('.decisions-row').first().boundingBox(), detail = await p.locator('.decisions-detail').boundingBox();
    await p.getByLabel('Answer 1', { exact: true }).fill('Keep my draft.');
    await p.evaluate(item => { __decisionItems.unshift(item); __emit('bus:changed', {}); }, makeRequest('r3'));
    await p.locator('.decisions-row[data-request-id="r3"]').waitFor();
    ok('an arrival appends without moving the earlier row or detail', JSON.stringify(row) === JSON.stringify(await p.locator('.decisions-row').first().boundingBox()) && JSON.stringify(detail) === JSON.stringify(await p.locator('.decisions-detail').boundingBox()));
    ok('arrival preserves selection, draft and focus', await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'Keep my draft.' && await p.getByLabel('Answer 1', { exact: true }).evaluate(e => e === document.activeElement));
    await choose(t, 'r2');
    ok('a plain mention has a reply and no invented recommendation', await p.getByLabel('Your Reply', { exact: true }).count() === 1 && await p.locator('.decisions-recommendation').count() === 0 && await p.getByRole('button', { name: 'Dismiss Message', exact: true }).count() === 1);
    await p.getByLabel('Filter Requests', { exact: true }).fill('Review r1');
    ok('filter and selected request are addressable', await p.evaluate(() => new URL(location.href).searchParams.get('view.decisions').includes('request=r2') && new URL(location.href).searchParams.get('view.decisions').includes('q=Review+r1')));
    await t.shot('decisions-list');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
