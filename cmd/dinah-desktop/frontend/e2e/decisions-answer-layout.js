const { start, origin, entry, deliver } = require('./lib/decision-answer-fixture');
(async () => {
  const t = await start(); const { page: p, ok } = t;
  try {
    await t.chat.locator('.msg:not(.msg-you)').waitFor();
    const geometry = () => p.evaluate(key => ['[data-win="' + key + '"]', '[data-win="' + key + '"] .composer', '.win-bar'].map(selector => { const r = document.querySelector(selector).getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; }), t.chatKey);
    const before = await geometry();
    await p.evaluate(() => __emit('agent:arriving', { sessionId: 'live', messages: [{ id: 501, from: 'paul', text: 'Answer preview.', decisionAnswer: true }], more: 0 }));
    const waiting = t.chat.locator('.arriving-row[data-arriving="501"]'); await waiting.waitFor();
    await p.evaluate(() => __emit('agent:updates', { sessionId: 'live', updates: [{ sessionUpdate: 'agent_message_chunk', turnId: 'later-turn', itemId: 'later-item', contentMode: 'snapshot', content: { type: 'text', text: 'Later transcript row.' } }] }));
    const later = t.chat.locator('.msg:not(.msg-you)').last(); await later.getByText('Later transcript row.', { exact: true }).waitFor();
    const laterBefore = await later.boundingBox(), waitingBefore = await waiting.boundingBox();
    const large = origin({ questions: [{ number: 2, text: 'Question line.\n'.repeat(40), answer: 'Answer text. '.repeat(1000), asRecommended: true }] });
    await deliver(t, entry(501, large)); const card = t.chat.locator('.decision-answer-card'); await card.waitFor();
    ok('replacement keeps the waiting card height and later rows in place', Math.abs(waitingBefore.height - (await card.locator('..').boundingBox()).height) < 1 && JSON.stringify(laterBefore) === JSON.stringify(await later.boundingBox()));
    ok('large snapshots scroll inside the compact card', await card.locator('.decision-answer-body').evaluate(el => el.scrollHeight > el.clientHeight && el.scrollWidth <= el.clientWidth + 1));
    ok('arrival keeps the window, composer and taskbar bounds fixed', JSON.stringify(before) === JSON.stringify(await geometry()));
    const savedKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'saved' }));
    const saved = p.locator('[data-win="' + savedKey + '"]'); await saved.locator('.decision-answer-card').waitFor();
    ok('saved transcripts retain typed answers and recommendation marks', await saved.locator('.decision-answer-mode').count() === 1 && (await saved.locator('.decision-answer-card').innerText()).includes('Question 7'));
    await p.evaluate(key => dinahWorkbench.open(dinahWorkbench.item(key)), t.chatKey);
    await t.shot('decisions-answer-transcript');
    await p.evaluate(key => pudlWindows.snap(key, 'maximized'), t.chatKey); await p.setViewportSize({ width: 390, height: 844 });
    await p.waitForFunction(key => document.querySelector('[data-win="' + key + '"]').getBoundingClientRect().right <= innerWidth, t.chatKey);
    ok('a narrow answer card stays inside its transcript', await card.evaluate(el => el.scrollWidth <= el.clientWidth + 1) && await card.locator('.decision-answer-body').evaluate(el => el.scrollWidth <= el.clientWidth + 1));
    await t.shot('decisions-answer-narrow');
    await p.evaluate(value => __emit('agent:updates', { sessionId: 'live', updates: [{ sessionUpdate: 'dinah_delivered', message: value }] }), entry(507));
    await t.chat.locator('.decision-answer-card').nth(1).waitFor();
    ok('a replayed delivered entry retains its typed snapshots', await t.chat.locator('.decision-answer-card').nth(1).locator('.decision-answer-mode').count() === 1 && (await t.chat.locator('.decision-answer-card').nth(1).innerText()).includes('Question 7'));
    ok('no native or external action occurred', await p.evaluate(() => __runtime.length === 0));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
