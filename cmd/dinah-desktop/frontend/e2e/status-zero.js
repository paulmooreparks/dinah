// Notifications and Judgements in the status bar show a neutral 0 once
// they know they have none, and nothing while they are still reading, so
// the space after them is never a blank. A count above 0 keeps its warning
// badge, and Judgements turned off keeps its "off". Nothing moves as the
// counts change.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      let release; window.__gate = new Promise(r => { release = r; }); window.__release = release;
      window.__items = [];
      window.__j = { stateGen: 1, stateVer: 1, state: 'on', why: '', calls: 4, fallbacks: 0, streak: 0, warnStreak: 3, unseen: 0, latest: new Date().toISOString() };
      return {
        Notifications: async () => { await window.__gate; return window.__items; },
        JudgementState: async () => window.__j,
        JudgementSummary: async () => ({ from: '', to: '', calls: 4, fallbacks: 0, groups: [], latest: '' }),
        JudgementRecords: async () => ({ records: [], next: 0 }),
        JudgementKinds: async () => [],
      };
    },
  });
  const { page: p, ok } = t;
  const badge = id => p.evaluate(id => {
    const item = document.getElementById(id), b = item.querySelector('[data-status-badge]'), r = item.getBoundingClientRect();
    return { text: b.hidden ? '' : b.textContent, warn: b.classList.contains('warn'), width: Math.round(r.width) };
  }, id);
  try {
    await p.locator('#status-judgements:not([hidden])').waitFor();
    let needs = await badge('status-needs');
    ok('while notifications are still being read, there is no false 0', needs.text === '', JSON.stringify(needs));
    const before = needs.width;
    // The folded Status tray of PUDL 0.97 went with PUDL's compact bar (902df02), so the counts are read here only unfolded.
    await p.evaluate(() => __release());
    await p.waitForFunction(() => document.querySelector('#status-needs [data-status-badge]').textContent === '0');
    needs = await badge('status-needs');
    let judge = await badge('status-judgements');
    ok('with none waiting, Notifications shows a neutral 0', needs.text === '0' && !needs.warn, JSON.stringify(needs));
    ok('with no undismissed warnings, Judgements shows a neutral 0', judge.text === '0' && !judge.warn, JSON.stringify(judge));
    ok('showing 0 does not change the item\'s width', needs.width === before, JSON.stringify({ before, after: needs.width }));
    await t.shot('status-zero');
    // Counts above 0 keep their warning badges, at the same width.
    await p.evaluate(() => {
      window.__items = [{ id: 'm1', kind: 'mention', severity: 'info', title: 'Engine mentioned you', body: 'In Team A', sessionId: 'x', created: new Date().toISOString() }];
      __emit('notification:delivery');
      window.__j = Object.assign({}, window.__j, { stateVer: 2, unseen: 2 });
      __emit('judgements:changed', window.__j);
    });
    await p.waitForFunction(() => document.querySelector('#status-needs [data-status-badge]').textContent === '1');
    await p.waitForFunction(() => document.querySelector('#status-judgements [data-status-badge]').textContent === '2');
    needs = await badge('status-needs'); judge = await badge('status-judgements');
    ok('a count above 0 keeps its warning badge, at the same width', needs.warn && judge.warn && needs.width === before, JSON.stringify({ needs, judge, before }));
    await t.shot('status-nonzero');
    // Judgements turned off says so.
    await p.evaluate(() => { window.__j = Object.assign({}, window.__j, { stateVer: 3, state: 'off', unseen: 0 }); __emit('judgements:changed', window.__j); });
    await p.waitForFunction(() => document.querySelector('#status-judgements [data-status-badge]').textContent === 'off');
    ok('Judgements turned off keeps its "off", not a 0', (await badge('status-judgements')).text === 'off');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
