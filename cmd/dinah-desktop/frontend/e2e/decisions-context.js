const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  const t = await setup([makeRequest('r100', { kind: 'mention', questions: [], text: 'Please review the proposal.' }), makeRequest('r200'), makeRequest('r300', { group: '', groupTitle: '', destination: 'Direct Review' })]);
  const { page: p, ok } = t;
  const bounds = () => p.evaluate(() => ['[data-win="decisions"]', '.decisions-list', '.decisions-detail', '.decisions-source', '.decisions-delivery', '.decisions-context', '.decisions-question', '.decisions-destination', '.decisions-detail .btn-primary', '.win-bar'].map(selector => {
    const r = document.querySelector(selector).getBoundingClientRect(); return [r.x, r.y, r.width, r.height];
  }));
  const messages = Array.from({ length: 5 }, (_, index) => ({ id: 90 + index, room: '#Review', from: 'Reviewer ' + index, createdAt: '2026-11-14T00:0' + index + ':00Z', text: 'First line.\nSecond line.\nThird line with <script>literal text</script>.\n' + 'Further context. '.repeat(35) }));
  try {
    await p.evaluate(() => { __holdContext = true; });
    await choose(t, 'r100'); await p.waitForFunction(() => window.__contextPending?.length === 1);
    ok('slow context has a fixed placeholder', await p.locator('.decisions-context-body').getAttribute('aria-busy') === 'true' && (await p.locator('.decisions-context').innerText()).includes('Reading the group conversation.'));
    await p.getByLabel('Your Reply', { exact: true }).fill('Keep this draft while reading.');
    const before = await bounds();
    await p.evaluate(messages => { __contextMessages = messages; __holdContext = false; __contextPending.splice(0).forEach(resolve => resolve()); }, messages);
    await p.locator('[data-context-message="94"]').waitFor();
    ok('five earlier messages keep chronological order, sender and time', await p.locator('[data-context-message]').evaluateAll(nodes => nodes.map(el => el.dataset.contextMessage).join(',') === '90,91,92,93,94' && nodes.every(el => el.querySelector('strong').textContent.startsWith('Reviewer ') && el.querySelector('time').dateTime)));
    ok('loading context changes no measured bounds, draft or focus', JSON.stringify(before) === JSON.stringify(await bounds()) && await p.getByLabel('Your Reply', { exact: true }).inputValue() === 'Keep this draft while reading.' && await p.getByLabel('Your Reply', { exact: true }).evaluate(el => document.activeElement === el));
    ok('message text is literal and collapsed to two lines', await p.locator('.decisions-context-text').first().evaluate(el => !el.querySelector('script') && el.textContent.includes('<script>') && el.clientHeight <= parseFloat(getComputedStyle(el).lineHeight) * 2 + 1 && el.scrollHeight > el.clientHeight));
    await p.getByRole('button', { name: 'Expand Message', exact: true }).first().click();
    ok('expansion stays inside the fixed conversation area', JSON.stringify(before) === JSON.stringify(await bounds()) && await p.locator('.decisions-context-message').first().getAttribute('data-expanded') === 'true');
    await t.shot('decisions-context-mention');
    await p.evaluate(() => { window.__groupOpens = []; window.dinahBus.openGroup = async (group, target) => { __groupOpens.push({ group, target }); }; });
    const link = p.getByRole('link', { name: 'Open Message', exact: true }).first();
    ok('message links carry their destination in the address', (await link.getAttribute('href')).includes('message'));
    await link.click();
    ok('opening context targets its own group and message', await p.evaluate(() => __groupOpens.length === 1 && __groupOpens[0].group === '#Review' && __groupOpens[0].target.message === 90));
    await p.evaluate(() => { __holdContext = true; }); await choose(t, 'r200');
    await p.waitForFunction(() => __contextPending.length === 1);
    await choose(t, 'r300');
    await p.evaluate(() => { __holdContext = false; __contextPending.splice(0).forEach(resolve => resolve()); });
    await p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    ok('late context cannot replace a direct request', await p.locator('[data-context-message]').count() === 0 && (await p.locator('.decisions-context').innerText()).includes('sent directly'));
    await p.evaluate(() => { __contextFailure = true; }); await choose(t, 'r200');
    await p.getByRole('button', { name: 'Retry Conversation', exact: true }).waitFor(); const failed = await bounds();
    ok('structured requests retain recommendations when context fails', await p.locator('.decisions-recommendation').count() === 2 && (await p.locator('.decisions-context').innerText()).includes('could not be read'));
    await t.shot('decisions-context-error');
    await p.evaluate(() => { __contextFailure = false; __contextMessages = []; });
    await p.getByRole('button', { name: 'Retry Conversation', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.decisions-context').textContent.includes('no earlier messages'));
    ok('empty context and retry retain the same bounds', JSON.stringify(failed) === JSON.stringify(await bounds()));
    for (const invalid of [[...messages, messages[0]], [{ ...messages[0], room: 'another-group' }], [{ ...messages[0], id: 200 }], [messages[1], messages[0]], [{ ...messages[0], createdAt: 'invalid' }]]) {
      await choose(t, 'r300'); await p.evaluate(value => { __contextMessages = value; }, invalid); await choose(t, 'r200');
      await p.getByRole('button', { name: 'Retry Conversation', exact: true }).waitFor();
      ok('invalid context is refused without exposing messages', await p.locator('[data-context-message]').count() === 0 && JSON.stringify(failed) === JSON.stringify(await bounds()));
    }
    await p.evaluate(messages => { __contextMessages = messages; }, messages);
    await choose(t, 'r100'); await choose(t, 'r200'); await p.locator('[data-context-message="94"]').waitFor();
    ok('structured requests use their carrying message', (await t.calls('BusGroupMessageContext')).some(call => call[1] === '#Review' && call[2] === 200));
    await p.setViewportSize({ width: 390, height: 844 });
    await p.waitForFunction(() => getComputedStyle(document.querySelector('.decisions-list')).display === 'none');
    ok('narrow context has no horizontal overflow', await p.locator('.decisions-context').evaluate(el => el.scrollWidth <= el.clientWidth + 1) && await p.locator('.decisions-context-body').evaluate(el => el.scrollWidth <= el.clientWidth + 1));
    await t.shot('decisions-context-narrow');
    ok('context reads never mark messages read or deliver anything', !(await t.calls()).some(call => /Mark.*Read|AnswerDecision|DismissDecision|ConfirmDecisionBroadcast/.test(call[0])) && await p.evaluate(() => __delivered.length === 0));
  } finally { await t.close(); }
  const linkedMessages = [89, 90, 100, 101].map(id => ({ id, room: 'review', from: 'Reviewer', fromSession: 'reviewer', fromPerson: false, fromWorkspace: 'C:/Fixture', createdAt: '2026-11-14T00:00:00Z', text: 'Review message ' + id, attachments: [], mentions: [] }));
  const linked = await setup([makeRequest('r100', { group: 'review', groupTitle: 'Review Group' })], { contextRoom: linkedMessages });
  try {
    await linked.page.evaluate(message => { __contextMessages = [message]; }, linkedMessages[1]);
    await choose(linked, 'r100'); await linked.page.locator('[data-context-message="90"]').waitFor();
    linked.ok('showing context makes no read-state call', !(await linked.calls()).some(call => /Mark.*Read/.test(call[0])));
    const address = await linked.page.getByRole('link', { name: 'Open Message', exact: true }).getAttribute('href');
    await linked.page.getByRole('link', { name: 'Open Message', exact: true }).click();
    await linked.page.locator('.bus-target-message[data-bus-message="90"]').waitFor();
    linked.ok('the real group view opens at the chosen earlier message', true);
    await linked.shot('decisions-context-group-target');
    await linked.page.goto('http://dinah.test/index.html' + address);
    await linked.page.locator('.bus-target-message[data-bus-message="90"]').waitFor();
    linked.ok('the message link also works as a direct address', true);
  } finally { await linked.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
