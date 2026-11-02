// Counts judged once the display has settled.
//
// Dinah draws transcripts, folds, replayed history and agent events through
// a frame-budgeted renderer (dinahRender, app.js queueAgentEvent), so a list
// can be part drawn for a frame or more. A check that waits for the first of
// several elements and counts at once can catch that frame on a loaded
// machine: it fails now and then although nothing is wrong, and a check that
// something appears once, or not at all, can pass although a duplicate or a
// stray element arrives a frame later. chat-placement, taskbar-state-stability
// and teams-traffic each met this in November 2026.
//
//   const { settled, settledCount } = require('./lib/settled');
//   const r = await settled(page, { person: [win.getByText('Hello'), 1], agent: [win.locator('.msg-row'), 2] });
//   t.ok('the window shows the saved copy', r.ok, r.detail);
//   const folds = await settledCount(page, page.locator('.team-traffic-fold'), 2);
//   t.ok('restored history retains the folds', folds.ok, folds.detail);
//
// settled waits up to `within` milliseconds for every locator to reach its
// expected count, then pauses `pause` milliseconds and counts them all
// again, and the second count decides. A doubled, cleared or late element
// fails, while a frame caught mid-draw does not. On failure, detail holds
// each count and, for up to ten of each locator's elements, its text, its
// tag and classes, the row or view it is in and whether it is shown, so the
// log of a failed run says what the page showed and where. Absence is an expected
// count of 0, judged the same way.
async function settled(page, expected, { within = 5000, pause = 500 } = {}) {
  const names = Object.keys(expected);
  const counts = async () => {
    const out = {};
    for (const name of names) out[name] = await expected[name][0].count();
    return out;
  };
  const matches = c => names.every(name => c[name] === expected[name][1]);
  const deadline = Date.now() + within;
  let seen = await counts();
  while (!matches(seen) && Date.now() < deadline) {
    await page.waitForTimeout(100);
    seen = await counts();
  }
  await page.waitForTimeout(pause);
  seen = await counts();
  const ok = matches(seen);
  if (ok) return { ok, counts: seen, detail: undefined };
  const report = {};
  for (const name of names) {
    // Each element's text and where it is: its tag and classes, its nearest
    // row or view, and whether it is shown, so two matches with the same
    // words can be told apart.
    const found = await expected[name][0].evaluateAll(els => els.slice(0, 10).map(el => {
      const where = el.closest('[data-view-id], .activity-row, [data-win]');
      return {
        text: (el.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 80),
        element: el.tagName.toLowerCase() + (el.className && typeof el.className === 'string' ? '.' + el.className.trim().split(/\s+/).join('.') : ''),
        within: where ? where.tagName.toLowerCase() + (where.dataset.viewId ? '[data-view-id=' + where.dataset.viewId + ']' : where.className ? '.' + String(where.className).trim().split(/\s+/)[0] : '') : '',
        shown: el.getClientRects().length > 0
      };
    }));
    report[name] = { expected: expected[name][1], counted: seen[name], text: found.map(f => f.text), elements: found };
  }
  return { ok, counts: seen, detail: JSON.stringify(report) };
}

// settledCount is settled for one locator.
function settledCount(page, locator, want, options) {
  return settled(page, { count: [locator, want] }, options);
}

module.exports = { settled, settledCount };
