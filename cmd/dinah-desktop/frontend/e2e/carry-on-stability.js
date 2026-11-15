const fs = require('fs'), path = require('path');
const { OUT } = require('./lib/page');
const { start } = require('./lib/decision-answer-fixture');
// These are the four resumeText bodies used by Quality's independent audit.
const messages = {
  cancelled: 'The restart of Dinah was cancelled. If you paused your work for it, carry on from where you stopped.',
  paused: 'Dinah has restarted with its update. You paused your work for the restart; carry on from where you stopped.',
  ended: 'Dinah has restarted with its update. Your last turn ended just before it; if work was left to do, carry on.',
  interrupted: 'Dinah has restarted with its update, and your turn was cut short by the restart. Look at what you had done, since the last step may be half finished, and carry on from there.'
};
(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  for (const [name, text] of Object.entries(messages)) {
    const t = await start();
    try {
      await t.chat.locator('.msg:not(.msg-you)').waitFor();
      const result = await t.page.evaluate(async ({ key, text }) => {
        const root = document.querySelector('[data-win="' + key + '"]'), earlier = root.querySelector('.msg:not(.msg-you)');
        const frame = () => new Promise(requestAnimationFrame);
        const box = el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; };
        await frame(); await frame();
        const sample = () => ({ earlier: box(earlier), row: root.querySelector('[data-arriving="701"], .delivered-row') && box(root.querySelector('[data-arriving="701"], .delivered-row')), chrome: [box(root), box(root.querySelector('.composer')), box(document.querySelector('.win-bar'))] });
        const before = sample();
        __emit('agent:arriving', { sessionId: 'live', messages: [{ id: 701, from: 'Dinah', kind: 'host_result', text }], more: 0 });
        const arrival = [sample()];
        for (let i = 0; i < 10; i++) { await frame(); arrival.push(sample()); }
        const row = root.querySelector('[data-arriving="701"]'), waiting = sample();
        __emit('agent:sent', { sessionId: 'live', kind: 'bus', delivered: [{ role: 'delivered', text, provider: 'codex', delivered: { id: 701, from: 'Dinah', kind: 'host_result', time: '2026-11-15T04:00:00Z' } }] });
        const delivery = [sample()];
        for (let i = 0; i < 10; i++) { await frame(); delivery.push(sample()); }
        return { before, arrival, waiting, delivery, sameRow: root.querySelector('.delivered-row') === row, text: root.querySelector('.delivered-text').textContent };
      }, { key: t.chatKey, text });
      fs.writeFileSync(path.join(OUT, 'carry-on-' + name + '.json'), JSON.stringify(result, null, 2));
      const close = (a, b) => a.every((v, i) => Math.abs(v - b[i]) < 1);
      const positions = [result.before, ...result.arrival].map(f => f.earlier[1]);
      const moves = positions.slice(1).filter((v, i) => Math.abs(v - positions[i]) >= 1).length;
      t.ok(name + ' reserves its full space once on arrival', moves <= 1 && result.waiting.row[3] > 0, positions.join(','));
      t.ok(name + ' retains the waiting row through delivery', result.sameRow);
      t.ok(name + ' keeps one height and earlier text in place through body rendering', result.delivery.every(f => f.row && close(f.row, result.waiting.row) && close(f.earlier, result.waiting.earlier)));
      t.ok(name + ' keeps all carry-on text available', result.text === text);
      t.ok(name + ' keeps window, composer and taskbar fixed', [...result.arrival, ...result.delivery].every(f => f.chrome.every((b, i) => close(b, result.before.chrome[i]))));
      await t.shot('carry-on-' + name);
    } finally { await t.close(); }
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
