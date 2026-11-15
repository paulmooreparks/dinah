const fs = require('fs');
const path = require('path');
const { start } = require('./lib/org-chart-fixture');
const { OUT } = require('./lib/page');
(async () => {
  const t = await start(), p = t.page, ok = t.ok, chart = t.chart;
  try {
    const evidence = [];
    for (const width of [1440, 390]) {
      await p.setViewportSize({ width, height: 1000 });
      await p.waitForFunction(() => { const el = document.querySelector('.org-chart-scroll'); return el && el.scrollWidth <= el.clientWidth + 1; });
      await chart.locator('[data-org-chat="nina"]').focus();
      const frames = await p.evaluate(async () => {
        const root = document.querySelector('.org-chart'), rows = Array.from(root.querySelectorAll('[data-org-chat]')), svg = root.querySelector('svg'), first = rows[0], canvas = root.querySelector('.org-chart-canvas');
        function measure() { return [canvas.getBoundingClientRect().height, root.querySelector('.org-chart-scroll').scrollTop, ...rows.flatMap(el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; }), ...Array.from(document.querySelectorAll('.win-bar')).flatMap(el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; })]; }
        const baseline = measure(), samples = [];
        for (let step = 0; step < 4; step++) {
          __orgModel.generation = 'state:' + step; __orgModel.chats.forEach(c => { c.state = { life: step === 3 ? 'closed' : 'open', exec: { state: ['working', 'waiting', 'idle', 'stuck'][step], why: 'A long explanation. '.repeat(60) } }; c.title += step === 1 ? ' A long title'.repeat(20) : ''; c.providerName = step === 1 ? 'An agent program with a long name'.repeat(10) : 'Codex'; });
          const previous = __orgReads; __emit('agent:chatstate', { stateGen: 'chart-only', stateVer: step + 1, chats: [] });
          samples.push(measure());
          for (let frame = 0; frame < 12; frame++) { await new Promise(requestAnimationFrame); samples.push(measure()); }
          if (__orgReads <= previous) throw new Error('The state event did not refresh the chart.');
        }
        return { baseline, samples, sameSVG: svg === root.querySelector('svg'), sameRows: rows.every(el => el === root.querySelector('[data-org-chat="' + CSS.escape(el.dataset.orgChat) + '"]')), focus: document.activeElement.dataset.orgChat, first: first.dataset.orgChat };
      });
      evidence.push({ width, ...frames });
      ok('every frame keeps chat boxes, canvas, scroll and taskbar fixed at ' + width, frames.samples.every(sample => JSON.stringify(sample) === JSON.stringify(frames.baseline)));
      ok('state refresh keeps the drawing, chat elements and keyboard focus at ' + width, frames.sameSVG && frames.sameRows && frames.focus === 'nina');
    }
    fs.writeFileSync(path.join(OUT, 'org-chart-frames.json'), JSON.stringify(evidence, null, 2));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
