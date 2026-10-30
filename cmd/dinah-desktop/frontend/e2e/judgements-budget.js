// The judging budget and the pool: the widget's popup lines, the page's
// budget card with its allowance mark, the Reason filter including the
// budget spent, and the Settings rows saved inside the revision envelope
// (judgements-applet.js, step 4 of judgement-agents.md).
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      // Everything a fixture needs is built in the page: this function goes
      // there as source, and nothing from the suite's scope reaches it.
      const week = new Date(); week.setHours(0, 0, 0, 0); week.setTime(week.getTime() - 2 * 86400000);
      window.__j = { state: {
        stateGen: 'g1', stateVer: 1, state: 'on', why: '', calls: 12, fallbacks: 3, streak: 0, warnStreak: 3, latest: new Date().toISOString(),
        budget: [{ provider: 'ollama', unit: 'tokens', limit: 10000, spent: 1200, allowance: 2300.5, left: 8800, periodStart: week.toISOString(), periodEnd: new Date(week.getTime() + 7 * 86400000).toISOString() }],
        pool: { size: 2, min: 1, max: 4, growAt: 0.5, shrinkAfter: 10, inFlight: 1, changed: { at: new Date().toISOString(), from: 1, to: 2, why: 'grew: a call waited' } },
        calibration: { state: 'idle', chosen: '' },
      } };
      window.__setCalls = [];
      window.__recordArgs = [];
      window.__settings = { enabled: true, provider: 'ollama', model: 'kimi-k3:cloud', retentionDays: 90, warnStreak: 3, kindsOff: [],
        budgets: [{ provider: 'ollama', unit: 'tokens', limit: 10000 }], period: { weekday: 1, hour: 0 }, pool: { min: 1, max: 4 } };
      window.__revision = 'r1';
      window.__rev = 1;
      const records = Array.from({ length: 6 }, (_, i) => ({
        id: i + 1, time: new Date().toISOString(), kind: 'chat-title', version: 'v1', input: '{}',
        answer: i < 3 ? 'A Title' : '', ruleAnswer: '"Rule"', out: i < 3 ? '"A Title"' : '"Rule"', used: i < 3 ? 'judge' : 'rule',
        why: i < 3 ? '' : (i < 5 ? 'late' : 'budget'), provider: 'ollama', model: 'kimi-k3:cloud', temperature: 0, seed: 1,
        deadlineMs: 3000, elapsedMs: 200, costUnit: 'tokens', costIn: 100, costOut: 20, effectKind: 'chat', effectId: 'c1', correction: '',
      }));
      return {
        AgentProviders: async () => [{ id: 'ollama', name: 'Ollama', installed: true, available: true }],
        JudgementState: async () => window.__j.state,
        JudgementSummary: async () => ({ from: '', to: '', calls: 6, fallbacks: 3, groups: [], latest: new Date().toISOString(), streak: 0 }),
        JudgementRecords: async f => { window.__recordArgs.push(f); const list = records.slice().reverse(); const rows = f.why ? list.filter(r => r.why === f.why) : list; return { records: rows, next: 0 }; },
        JudgementKinds: async () => [{ name: 'chat-title', version: 'v1', describe: 'The title is the first line of your words, cleaned.', correctable: true, deadlineMs: 3000, enabled: true, inputCap: 12000, input: [{ name: 'line', type: 'string', doc: 'the first line' }] }],
        JudgementSettings: async () => ({ settings: window.__settings, revision: window.__revision, conflict: false }),
        JudgementModels: async () => [{ name: 'kimi-k3:cloud', provider: 'ollama' }],
        SetJudgementSettings: async (settings, revision) => { window.__setCalls.push([settings, revision]); window.__settings = settings; window.__revision = 'r' + (++window.__rev); return { settings, revision: window.__revision, conflict: false }; },
        JudgementCalibration: async () => ({ state: 'idle', models: [], cases: [] }),
      };
    },
  });
  const { page: p, ok } = t;
  try {
    // The page: the budget row with its mark, and the pool line.
    await p.evaluate(() => dinahWorkbench.open('judgements'));
    const win = p.locator('.win[data-win-kind="judgements"]');
    await win.locator('.judgement-budget-row').waitFor();
    ok('the page names the budget, spent and left in the provider’s unit', /Ollama/.test(await win.locator('.judgement-budget-name').textContent()) && /1,200 of 10,000 tokens spent, 8,800 left/.test(await win.locator('.judgement-budget-row').textContent()), await win.locator('.judgement-budget-row').textContent());
    await t.shot('judgements-budget-panel');
    const geometry = await win.locator('.judgement-budget-track').evaluate(el => {
      const fillW = parseFloat(el.querySelector('.judgement-budget-fill').style.width);
      const markL = parseFloat(el.querySelector('.judgement-budget-allowance').style.left);
      const markTitle = el.querySelector('.judgement-budget-allowance').title;
      return { fillW, markL, markTitle };
    });
    ok('the track fills with the spent share and marks the allowance by now', Math.abs(geometry.fillW - 12) < 0.5 && Math.abs(geometry.markL - 23.005) < 0.01 && /2,300 tokens allowed by now/.test(geometry.markTitle), JSON.stringify(geometry));
    const poolText = await win.locator('.judgement-pool-line').textContent();
    ok('the pool line says its size, its bounds, the in-flight call and the last change', /Pool: 2 with the judge at once, between 1 and 4; 1 in flight now; it grew: a call waited/.test(poolText), poolText);
    // The Reason filter carries the budget's reason.
    await win.getByLabel('Reason', { exact: true }).selectOption('budget');
    await p.waitForFunction(() => __recordArgs[__recordArgs.length - 1].why === 'budget');
    ok('the Reason filter finds the calls the budget refused', (await win.locator('.judgements-list [data-judgement]').count()) === 1 && /rule: budget/.test(await win.locator('.judgements-list').textContent()));
    // The widget's popup has the budget and pool lines.
    await p.locator('#status-judgements').click();
    await p.locator('#judgements-status-list .judgements-popup-budget').waitFor();
    ok('the popup carries the budget line', /Ollama: 1,200 of 10,000 tokens spent this week, 8,800 left, 2,300 allowed by now/.test(await p.locator('#judgements-status-list .judgements-popup-budget').textContent()));
    ok('and the pool line', /Pool: 2 with the judge at once/.test(await p.locator('#judgements-status-list .judgements-popup-pool').textContent()));
    await p.keyboard.press('Escape');

    // The Settings rows: budget, week, pool, each saved in the envelope.
    await p.evaluate(() => dinahWorkbench.open('settings'));
    const settings = p.locator('.win[data-win-kind="settings"]');
    await settings.locator('[data-settings-section="judgements"]').click();
    const budgetInput = settings.locator('#judgement-budgets .judgement-budget-edit input');
    await budgetInput.waitFor();
    ok('the budget row names the provider and its unit, with the limit set', /Ollama \(tokens\)/.test(await settings.locator('#judgement-budgets').textContent()) && (await budgetInput.inputValue()) === '10000');
    ok('a kind’s row names its cap beside its deadline', /allowed 3 s; the call shows it at most 12,000 bytes/.test(await settings.locator('#judgement-kinds > li').textContent()), await settings.locator('#judgement-kinds > li').textContent());
    await budgetInput.fill('');
    await settings.locator('#judgement-period-weekday').selectOption('5');
    await settings.locator('#judgement-period-hour').selectOption('9');
    await settings.locator('#judgements-save-budgets').click();
    await p.waitForFunction(() => __setCalls.length === 1);
    const saved = await p.evaluate(() => __setCalls[0][0]);
    ok('an empty budget saves as none, and the week begins when chosen', saved.budgets.length === 1 && saved.budgets[0].limit === null && saved.period.weekday === 5 && saved.period.hour === 9 && (await p.evaluate(() => __setCalls[0][1])) === 'r1', JSON.stringify(saved.budgets) + JSON.stringify(saved.period));
    ok('and the status says so', /budgets and the week they run on are saved/.test(await settings.locator('#judgements-status').textContent()));
    await settings.locator('#judgement-period-weekday').selectOption('1');
    await budgetInput.fill('-5');
    await settings.locator('#judgements-save-budgets').click();
    ok('a negative budget is refused before any save', /whole number, or empty for none/.test(await settings.locator('#judgements-status').textContent()) && (await p.evaluate(() => __setCalls.length)) === 1);
    await budgetInput.fill('20000');
    await settings.locator('#judgements-save-budgets').click();
    await p.waitForFunction(() => __setCalls.length === 2);
    ok('a whole number saves in the row’s unit', (await p.evaluate(() => __setCalls[1][0].budgets[0].limit)) === 20000 && (await p.evaluate(() => __setCalls[1][0].budgets[0].unit)) === 'tokens');
    // The pool: bounds checked before saving.
    await settings.locator('#judgement-pool-min').fill('4');
    await settings.locator('#judgement-pool-max').fill('2');
    await settings.locator('#judgements-save-pool').click();
    ok('a pool whose smallest passes its largest is refused', /whole numbers of 1 at least, with at most not below at least/.test(await settings.locator('#judgements-status').textContent()) && (await p.evaluate(() => __setCalls.length)) === 2);
    await settings.locator('#judgement-pool-max').fill('6');
    await settings.locator('#judgements-save-pool').click();
    await p.waitForFunction(() => __setCalls.length === 3);
    ok('a good pool saves, with no ceiling on the largest size', JSON.stringify(await p.evaluate(() => __setCalls[2][0].pool)) === JSON.stringify({ min: 4, max: 6 }));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
