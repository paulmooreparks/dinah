// The tray widget (judgements-applet.js): hidden where the engine did not
// build the judgements, its glyph, badge and colour by state, the popup with
// the day in words, and the two ways in to the page and its Settings section.
const { launch } = require('./lib/page');
(async () => {
  // With no judgement methods at all the tray stays without the widget.
  const plain = await launch({ viewport: { width: 1440, height: 900 } });
  try {
    await plain.page.locator('#status-needs').waitFor();
    plain.ok('a build without judgements shows no widget in the tray', await plain.page.locator('#status-judgements').isHidden());
  } finally { await plain.close(); }

  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      window.__j = {
        state: { stateGen: 1, stateVer: 1, state: 'on', why: '', calls: 12, fallbacks: 3, streak: 0, warnStreak: 3, latest: new Date().toISOString() },
        today: { from: '', to: '', calls: 12, fallbacks: 3, groups: [{ kind: 'chat-title', model: 'kimi-k3:cloud', costUnit: 'tokens', calls: 12, fallbacks: 3, disagreed: 2, corrected: 0, avgMs: 640, maxMs: 1200, costIn: 3100, costOut: 220 }], latest: new Date().toISOString(), streak: 0 },
      };
      return {
        JudgementState: async () => window.__j.state,
        JudgementSummary: async () => window.__j.today,
        JudgementRecords: async () => ({ records: [], next: 0 }),
        JudgementKinds: async () => [],
        JudgementSettings: async () => ({ settings: { enabled: true, provider: 'ollama', model: 'kimi-k3:cloud', retentionDays: 90, warnStreak: 3, kindsOff: [] }, revision: 'r1', conflict: false }),
        JudgementModels: async () => [],
        SetJudgementSettings: async s => s,
      };
    },
  });
  try {
    const p = t.page, ok = t.ok;
    const item = p.locator('#status-judgements');
    await item.waitFor();
    ok('on, the badge is today’s calls', (await item.locator('.badge').textContent()) === '12');
    ok('and its name says the counts', /on, 12 calls today, 3 answered by the rule/.test(await item.getAttribute('aria-label')), await item.getAttribute('aria-label'));
    // The popup: the day in words, then the two ways in.
    await item.click();
    await p.locator('#judgements-status-list .judgements-popup-line').waitFor();
    await p.waitForFunction(() => /Today: 12 calls, 3 by the rule/.test(document.querySelector('#judgements-status-list .judgements-popup-line').textContent));
    ok('the popup gives the day in words, ending with the latest call', /Today: 12 calls, 3 by the rule\. The latest was/.test(await p.locator('#judgements-status-list .judgements-popup-line').textContent()));
    await p.locator('#judgements-status-list').getByText('Open Judgements').click();
    await p.locator('.win[data-win-kind="judgements"]').waitFor({ state: 'attached' });
    ok('Open Judgements opens the page', true);
    await p.locator('.win[data-win-kind="judgements"] .win-head button[aria-label="Close"]').click().catch(() => {});
    await item.click();
    await p.locator('#judgements-status-list').getByText('Judgement Settings…').click();
    await p.locator('.win[data-win-kind="settings"]').waitFor({ state: 'attached' });
    await p.waitForFunction(() => { const tab = document.querySelector('[data-settings-section="judgements"]'); return tab && tab.getAttribute('aria-selected') === 'true' && !document.getElementById('settings-judgements').hidden; });
    ok('Judgement Settings… opens Settings at the section', true);
    // A recorded call refreshes the widget through the event.
    await p.evaluate(() => { __emit('judgements:changed', { id: 9, stateGen: 1, stateVer: 2, state: 'off', why: 'no model chosen', calls: 0, fallbacks: 0, streak: 0, warnStreak: 3, latest: '' }); });
    await p.waitForFunction(() => document.querySelector('#status-judgements .badge').textContent === 'off');
    ok('off shows the off badge and says why', (await item.getAttribute('data-judgement')) === 'off' && /off \(no model chosen\)/.test(await item.getAttribute('aria-label')));
    // A run of fallbacks is the warning: the badge goes amber with the streak.
    await p.evaluate(() => { __emit('judgements:changed', { id: 10, stateGen: 1, stateVer: 3, state: 'warning', why: '', calls: 9, fallbacks: 5, streak: 5, warnStreak: 3, latest: new Date().toISOString() }); });
    await p.waitForFunction(() => document.querySelector('#status-judgements').dataset.judgement === 'warning');
    await t.shot('judgements-widget-warning');
    ok('a warning carries the streak in an amber badge', (await item.locator('.badge').textContent()) === '5' && await item.locator('.badge.warn').count() === 1 && /warning, the last 5 calls were answered by the rule/.test(await item.getAttribute('aria-label')));
    // An event older than the one applied is dropped, as every numbered event is.
    await p.evaluate(() => { __emit('judgements:changed', { id: 11, stateGen: 1, stateVer: 2, state: 'off', why: 'no model chosen', calls: 0, fallbacks: 0, streak: 0, warnStreak: 3, latest: '' }); });
    await new Promise(r => setTimeout(r, 300));
    ok('a stale state is not applied over a newer one', (await item.getAttribute('data-judgement')) === 'warning' && (await item.locator('.badge').textContent()) === '5');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
