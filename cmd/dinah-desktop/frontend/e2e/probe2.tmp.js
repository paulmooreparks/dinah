const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      window.__j = { state: { stateGen: 1, stateVer: 1, state: 'on', why: '', calls: 12, fallbacks: 3, streak: 0, warnStreak: 3, latest: new Date().toISOString() } };
      return {
        JudgementState: async () => window.__j.state,
        JudgementSummary: async () => ({ from: '', to: '', calls: 12, fallbacks: 3, groups: [], latest: '', streak: 0 }),
        JudgementRecords: async () => ({ records: [], next: 0 }),
        JudgementKinds: async () => [],
        JudgementSettings: async () => ({ settings: { enabled: true, provider: 'ollama', model: 'kimi-k3:cloud', retentionDays: 90, warnStreak: 3, kindsOff: [] }, revision: 'r1', conflict: false }),
        JudgementModels: async () => [], SetJudgementSettings: async s => s,
      };
    },
  });
  try {
    const p = t.page;
    await p.waitForFunction(() => document.querySelector('#status-judgements .badge').textContent === '12');
    await p.evaluate(() => { __emit('judgements:changed', { id: 9, stateGen: 1, stateVer: 2, state: 'off', why: 'no model chosen', calls: 0, fallbacks: 0, streak: 0, warnStreak: 3, latest: '' }); });
    await new Promise(r => setTimeout(r, 400));
    console.log(JSON.stringify(await p.evaluate(() => ({
      badge: document.querySelector('#status-judgements .badge').textContent,
      dataJudgement: document.querySelector('#status-judgements').dataset.judgement,
      held: window.dinahStateOrder.held('judgements:changed', ''),
      hasHandler: !!(window.dinahHelpers && window.dinahHelpers.events),
      directDispatch: (window.dinahHelpers.events.dispatch('judgements:changed', { stateGen: 1, stateVer: 9, state: 'warning', calls: 9, fallbacks: 9, streak: 9, warnStreak: 3, latest: '', why: '' }), document.querySelector('#status-judgements').dataset.judgement),
    }))));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
