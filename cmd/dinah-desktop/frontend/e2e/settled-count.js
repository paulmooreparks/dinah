// The settled-count helper (lib/settled.js) judges what the page shows once
// it has settled. A list drawn a few elements a frame passes once it is
// whole; a duplicate, a cleared list or a stray element that arrives after
// the first look fails, and the failure says what was counted and shown.
const { launch } = require('./lib/page');
const { settled, settledCount } = require('./lib/settled');
(async () => {
  const t = await launch({ viewport: { width: 1000, height: 700 } });
  const { page: p, ok } = t;
  try {
    // A fixture list outside Dinah's own markup, drawn by the test.
    await p.evaluate(() => {
      const box = document.createElement('div');
      box.id = 'settled-fixture';
      document.body.append(box);
      window.__add = (cls, text) => { const el = document.createElement('div'); el.className = cls; el.textContent = text; box.append(el); };
      window.__clear = () => box.replaceChildren();
      window.__frames = n => new Promise(r => { const step = k => k ? requestAnimationFrame(() => step(k - 1)) : r(); step(n); });
    });
    const items = cls => p.locator('#settled-fixture .' + cls);

    // A list drawn one element every four frames is judged when whole.
    await p.evaluate(async () => { __add('drawn', 'one'); for (const word of ['two', 'three']) { await __frames(4); __add('drawn', word); } });
    await items('drawn').first().waitFor();
    const drawn = await settledCount(p, items('drawn'), 3);
    ok('a list drawn over several frames is judged once it is whole', drawn.ok, drawn.detail);

    // A duplicate that arrives after the first look fails a check for one.
    await p.evaluate(() => { __add('once', 'only'); setTimeout(() => __add('once', 'only again'), 200); });
    await items('once').first().waitFor();
    const doubled = await settledCount(p, items('once'), 1);
    ok('a late duplicate fails a check for exactly one, with its count and text', !doubled.ok && doubled.counts.count === 2 && /only again/.test(doubled.detail), doubled.detail);

    // A list cleared after it appeared fails.
    await p.evaluate(() => { __add('cleared', 'a'); __add('cleared', 'b'); setTimeout(() => document.querySelectorAll('#settled-fixture .cleared').forEach(el => el.remove()), 200); });
    await items('cleared').first().waitFor();
    const cleared = await settledCount(p, items('cleared'), 2);
    ok('a list cleared after it appeared fails', !cleared.ok && cleared.counts.count === 0, cleared.detail);

    // Absence is judged the same way: a stray element arriving late fails.
    await p.evaluate(() => setTimeout(() => __add('stray', 'late'), 300));
    const stray = await settledCount(p, items('stray'), 0);
    ok('a stray element arriving after the first look fails a check for none', !stray.ok && stray.counts.count === 1, stray.detail);

    // Several locators are judged together.
    const both = await settled(p, { drawn: [items('drawn'), 3], once: [items('once'), 2] });
    ok('several counts are judged together', both.ok && both.counts.drawn === 3 && both.counts.once === 2, both.detail);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
