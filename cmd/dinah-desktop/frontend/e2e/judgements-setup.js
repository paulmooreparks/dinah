// The first-start question of the judgements (judge_setup.go,
// docs/mockups/judgements-first-start): until the person answers, the bottom
// bar's item says Set Up, the flyout opens by itself once and asks, nothing is
// chosen, and the flyout is the size it is when it shows its facts.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      const offer = { provider: 'codex', providerName: 'Codex', billing: 'plan', callsPerDay: 200, limit: 14000000, unit: 'tokens', shown: false };
      window.__j = { calls: [], state: { stateGen: 1, stateVer: 1, state: 'setup', why: 'not set up', calls: 0, fallbacks: 0, streak: 0, warnStreak: 3, latest: '', provider: 'codex', model: '', warnings: [], unseen: 0, setup: offer } };
      const next = change => { window.__j.state = Object.assign({}, window.__j.state, change, { stateVer: window.__j.state.stateVer + 1 }); };
      return {
        JudgementState: async () => window.__j.state,
        JudgementSummary: async () => ({ calls: 0, fallbacks: 0, groups: [] }),
        JudgementRecords: async () => ({ records: [], next: 0 }),
        JudgementKinds: async () => [],
        JudgementSettings: async () => ({ settings: { enabled: true, provider: 'codex', model: '', retentionDays: 90, warnStreak: 3, kindsOff: [] }, revision: 'r1', conflict: false }),
        JudgementModels: async () => [],
        SetJudgementSettings: async s => s,
        NoteJudgementSetupShown: async () => { window.__j.calls.push('shown'); next({ setup: Object.assign({}, window.__j.state.setup, { shown: true }) }); },
        DeferJudgementSetup: async () => { window.__j.calls.push('later'); },
        AcceptJudgementSetup: async () => { window.__j.calls.push('on'); next({ state: 'off', why: 'choosing a model', setup: null }); },
      };
    },
  });
  try {
    const p = t.page, ok = t.ok;
    const item = p.locator('#status-judgements'), flyout = p.locator('#judgements-status-menu');
    await item.waitFor();
    await p.waitForFunction(() => document.getElementById('status-judgements').dataset.statusText === 'Set Up');
    ok('before any answer the item says Set Up, not Off', (await item.locator('[data-status-badge]').textContent()) === 'set up' && /not set up yet/.test(await item.getAttribute('aria-label')), await item.getAttribute('aria-label'));
    // It opens by itself, once, a moment after the window has started.
    await p.locator('#judgements-status-menu:popover-open').waitFor({ timeout: 15000 });
    ok('the flyout opens by itself and the engine is told it has', await p.evaluate(() => window.__j.calls.join(',')) === 'shown');
    const asked = await p.evaluate(() => ({
      question: document.getElementById('judgements-ask-question').textContent,
      cost: document.getElementById('judgements-ask-cost').textContent,
      factsShown: getComputedStyle(document.getElementById('judgements-flyout-facts')).visibility,
      askShown: getComputedStyle(document.getElementById('judgements-flyout-ask')).visibility,
      buttons: [...document.querySelectorAll('#judgements-status-menu .popover-actions button')].filter(b => !b.hidden).map(b => b.textContent),
    }));
    ok('it asks the question with who would answer and what a day may cost', asked.question === 'May Codex answer small questions for you in the background?' && /At most 200 calls a day, about 14 million tokens a week of your plan/.test(asked.cost) && asked.askShown === 'visible' && asked.factsShown === 'hidden', JSON.stringify(asked));
    ok('its answers are Judgement Settings, Not Now and Turn On', asked.buttons.join('|') === 'Judgement Settings…|Not Now|Turn On', asked.buttons.join('|'));
    const sizeAsking = await flyout.boundingBox();
    await t.shot('judgements-setup');
    // Not Now closes it and leaves the item at Set Up.
    await p.locator('#judgements-ask-later').click();
    await p.waitForFunction(() => !document.getElementById('judgements-status-menu').matches(':popover-open'));
    ok('Not Now closes the flyout, keeps the question, and chooses nothing', await p.evaluate(() => window.__j.calls.join(',')) === 'shown,later' && (await item.getAttribute('data-status-text')) === 'Set Up');
    // It does not open by itself again; the person opens it and turns judgements on.
    await p.waitForTimeout(5000);
    ok('it does not open by itself a second time', !(await flyout.evaluate(el => el.matches(':popover-open'))));
    await item.click();
    await p.locator('#judgements-status-menu:popover-open').waitFor();
    await p.locator('#judgements-ask-on').click();
    await p.waitForFunction(() => document.getElementById('status-judgements').dataset.statusText === 'Off');
    const after = await p.evaluate(() => ({
      why: document.getElementById('judgements-flyout-why').textContent,
      factsShown: getComputedStyle(document.getElementById('judgements-flyout-facts')).visibility,
      askShown: getComputedStyle(document.getElementById('judgements-flyout-ask')).visibility,
      buttons: [...document.querySelectorAll('#judgements-status-menu .popover-actions button')].filter(b => !b.hidden).map(b => b.textContent),
      open: document.getElementById('judgements-status-menu').matches(':popover-open'),
    }));
    ok('Turn On tells the engine and the flyout shows its facts, saying a model is being chosen', await p.evaluate(() => window.__j.calls.join(',')) === 'shown,later,on' && after.open && after.factsShown === 'visible' && after.askShown === 'hidden' && /model for judging is being chosen/.test(after.why) && after.buttons.join('|') === 'Judgement Settings…|Open Judgements', JSON.stringify(after));
    const sizeAfter = await flyout.boundingBox();
    ok('the flyout is the same size asking and showing its facts', Math.abs(sizeAsking.width - sizeAfter.width) < 1 && Math.abs(sizeAsking.height - sizeAfter.height) < 1, JSON.stringify([sizeAsking, sizeAfter]));
  } finally { await t.close(); }
})();
