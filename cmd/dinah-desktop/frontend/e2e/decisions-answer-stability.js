const fs = require('fs'), path = require('path');
const { OUT } = require('./lib/page');
const { start, origin, entry } = require('./lib/decision-answer-fixture');
(async () => {
  const t = await start(); const { page: p, ok } = t;
  try {
    await t.chat.locator('.msg:not(.msg-you)').waitFor();
    await p.evaluate(() => __emit('agent:arriving', { sessionId: 'live', messages: [{ id: 501, from: 'paul', text: 'Answer preview.', decisionAnswer: true }], more: 0 }));
    const waiting = t.chat.locator('[data-arriving="501"]'); await waiting.waitFor();
    await p.evaluate(() => __emit('agent:updates', { sessionId: 'live', updates: [{ sessionUpdate: 'agent_message_chunk', turnId: 'later-turn', itemId: 'later-item', contentMode: 'snapshot', content: { type: 'text', text: 'Later transcript row.' } }] }));
    await t.chat.getByText('Later transcript row.', { exact: true }).waitFor();
    const value = entry(501, origin({ questions: [{ number: 2, text: 'Question line.\n'.repeat(40), answer: 'Answer text. '.repeat(1000), asRecommended: true }] }));
    const result = await p.evaluate(async ({ key, value }) => {
      const root = document.querySelector('[data-win="' + key + '"]');
      const row = root.querySelector('[data-arriving="501"]'), log = row.parentElement;
      const later = [...root.querySelectorAll('.msg')].find(el => el.textContent.includes('Later transcript row.'));
      const body = root.querySelector('[data-view-id="activity"]');
      // Finish fixture setup before the delivery whose frames are measured.
      await new Promise(requestAnimationFrame); await new Promise(requestAnimationFrame);
      function box(el) { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; }
      function sample() {
        const card = root.querySelector('.decision-answer-card');
        return { row: box(card.parentElement), shown: card.checkVisibility({ contentVisibilityAuto: true }), later: box(later), scroll: body.scrollTop, chrome: [box(root), box(root.querySelector('.composer')), box(document.querySelector('.win-bar'))] };
      }
      const before = sample(), intrinsic = getComputedStyle(row).containIntrinsicBlockSize;
      const cardHeight = root.querySelector('.decision-answer-card').getBoundingClientRect().height;
      let removed = false;
      const observer = new MutationObserver(changes => { if (changes.some(change => [...change.removedNodes].includes(row))) removed = true; });
      observer.observe(log, { childList: true });
      __emit('agent:sent', { sessionId: 'live', kind: 'bus', delivered: [value] });
      const frames = [sample()];
      for (let i = 0; i < 8; i++) { await new Promise(requestAnimationFrame); frames.push(sample()); }
      observer.disconnect();
      return { before, frames, intrinsic, cardHeight, sameRow: root.querySelector('.delivered-row') === row, removed, answer: !!row.querySelector('.decision-answer-body') };
    }, { key: t.chatKey, value });
    ok('delivery retains the attached waiting row', result.sameRow && !result.removed && result.answer);
    ok('the answer stays visible in every delivery frame', result.frames.every(frame => frame.shown));
    ok('the initial row estimate equals the fixed card height', Math.abs(parseFloat(result.intrinsic.replace(/^auto\s+/, '')) - result.cardHeight) < 1, result.intrinsic);
    const close = (a, b) => a.length === b.length && a.every((v, i) => Math.abs(v - b[i]) < 1);
    ok('the card and later text stay in place in every delivery frame', result.frames.every(frame => close(frame.row, result.before.row) && close(frame.later, result.before.later) && frame.scroll === result.before.scroll), JSON.stringify(result));
    ok('the window, composer and taskbar stay fixed in every frame', result.frames.every(frame => frame.chrome.every((box, i) => close(box, result.before.chrome[i]))));
    fs.mkdirSync(OUT, { recursive: true }); fs.writeFileSync(path.join(OUT, 'decisions-answer-stability.json'), JSON.stringify(result, null, 2));
    await t.shot('decisions-answer-stability');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
