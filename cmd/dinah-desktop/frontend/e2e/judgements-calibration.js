// Choosing the judge's model: the calibration line by whose choice it is, the
// last run's report with what the models were tested on, running a calibration,
// and taking its choice as one's own (judgements-applet.js, step 5).
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__settings = { enabled: true, provider: 'ollama', model: 'deepseek-v4.1-flash:cloud', modelSource: 'person', retentionDays: 90, warnStreak: 3, kindsOff: [],
        budgets: [{ provider: 'ollama', unit: 'tokens', limit: null }], period: { weekday: 1, hour: 0 }, pool: { min: 1, max: 4 } };
      window.__revision = 'r1';
      window.__rev = 1;
      window.__setCalls = [];
      window.__runs = 0;
      window.__calibrationState = { state: 'chosen', at: new Date(Date.now() - 3600000).toISOString(), chosen: 'kimi-k3:cloud' };
      window.__report = {
        at: new Date(Date.now() - 3600000).toISOString(), state: 'chosen', running: false,
        note: 'Tried 3 models; 2 were candidates; 1 answered all six calls correctly.',
        models: [
          { name: 'kimi-k3:cloud', candidate: true, loaded: false, passed: 6, total: 6, medianMs: 210, why: '' },
          { name: 'deepseek-v4.1-flash:cloud', candidate: true, loaded: false, passed: 6, total: 6, medianMs: 640, why: '' },
          { name: 'qwen3.5:4b', candidate: false, loaded: true, passed: 4, total: 6, medianMs: 900, why: 'wrong on the haiku' },
        ],
        cases: [
          { kind: 'chat-title', name: 'the parser title', input: 'Please rename the parser’s tokenizer module to lexer and update the imports', accepts: ['lexer', 'tokenizer'] },
          { kind: 'document-name', name: 'the maintenance summary', input: 'Quarterly maintenance cost summary for the Lisbon office', accepts: ['maintenance', 'cost', 'Lisbon'] },
        ],
      };
      return {
        JudgementState: async () => ({ stateGen: 'g1', stateVer: 1, state: 'on', why: '', calls: 12, fallbacks: 0, streak: 0, warnStreak: 3, latest: new Date().toISOString(), budget: [], pool: { size: 1, min: 1, max: 4, growAt: 0.5, shrinkAfter: 10, inFlight: 0 }, calibration: window.__calibrationState }),
        JudgementSummary: async () => ({ from: '', to: '', calls: 12, fallbacks: 0, groups: [], latest: new Date().toISOString(), streak: 0 }),
        JudgementRecords: async () => ({ records: [], next: 0 }),
        JudgementKinds: async () => [{ name: 'chat-title', version: 'v1', describe: 'The title is the first line of your words, cleaned.', correctable: true, deadlineMs: 3000, enabled: true, input: [] }],
        JudgementSettings: async () => ({ settings: window.__settings, revision: window.__revision, conflict: false }),
        JudgementModels: async () => [{ name: 'kimi-k3:cloud', provider: 'ollama' }, { name: 'deepseek-v4.1-flash:cloud', provider: 'ollama' }],
        SetJudgementSettings: async (settings, revision) => { window.__setCalls.push([settings, revision]); window.__settings = settings; window.__revision = 'r' + (++window.__rev); return { settings, revision: window.__revision, conflict: false }; },
        JudgementCalibration: async () => window.__report,
        RunJudgementCalibration: async () => {
          window.__runs++;
          window.__calibrationState = { state: 'running', at: '', chosen: '' };
          __emit('judgements:changed', { stateGen: 'g1', stateVer: 2, state: 'on', why: '', calls: 12, fallbacks: 0, streak: 0, warnStreak: 3, latest: new Date().toISOString(), budget: [], pool: { size: 1, min: 1, max: 4, growAt: 0.5, shrinkAfter: 10, inFlight: 0 }, calibration: window.__calibrationState });
        },
      };
    },
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahWorkbench.open('settings'));
    const settings = p.locator('.win[data-win-kind="settings"]');
    await settings.locator('[data-settings-section="judgements"]').click();
    const line = settings.locator('#judgement-calibration-line');
    await line.waitFor();
    await p.waitForFunction(() => /kimi-k3:cloud/.test(document.getElementById('judgement-calibration-line').textContent));
    ok('the line names the judge and whose choice it is, then calibration’s passing finding', /The judge is deepseek-v4.1-flash:cloud \(your choice\)/.test(await line.textContent()) && /Calibration found kimi-k3:cloud passing and faster/.test(await line.textContent()), await line.textContent());
    // modelSource is the person's, but the model differs from calibration's choice.
    ok('the calibration choice differing from the person’s model offers its use', await settings.locator('#judgements-use-chosen').isVisible() && /Use kimi-k3:cloud/.test(await settings.locator('#judgements-use-chosen').textContent()));
    ok('the report opens with its note, every model marked, and its scoring', (await settings.locator('#judgement-calibration-details').isHidden()) === false &&
      /Tried 3 models/.test(await settings.locator('#judgement-calibration-report').textContent()) &&
      /6 of 6/.test(await settings.locator('#judgement-calibration-report').textContent()) &&
      /not a candidate, loaded for this run/.test(await settings.locator('#judgement-calibration-report').textContent()) &&
      /wrong on the haiku/.test(await settings.locator('#judgement-calibration-report').textContent()), (await settings.locator('#judgement-calibration-report').textContent()).slice(0, 300));
    await settings.locator('#judgement-calibration-toggle').first().click();
    await settings.locator('.judgement-calibration-cases > button.disclosure').click();
    await t.shot('judgements-calibration-report');
    ok('the report shows what every model was tested on', /put in “Please rename the parser’s tokenizer module/.test(await settings.locator('.judgement-calibration-cases').textContent()) && /must offer lexer or tokenizer/.test(await settings.locator('.judgement-calibration-cases').textContent()));
    // Use the Chosen Model is an ordinary settings save, the person choosing it.
    await settings.locator('#judgements-use-chosen').click();
    await p.waitForFunction(() => __setCalls.length === 1);
    ok('taking the choice is the person’s own save with the revision', (await p.evaluate(() => __setCalls[0][0].model)) === 'kimi-k3:cloud' && (await p.evaluate(() => __setCalls[0][1])) === 'r1');
    // A run starts and returns at once, and the line and button follow the state.
    await settings.locator('#judgements-run-calibration').click();
    await p.waitForFunction(() => __runs === 1);
    await p.waitForFunction(() => /running now/.test(document.getElementById('judgement-calibration-line').textContent));
    ok('running a calibration says so and holds the button', await settings.locator('#judgements-run-calibration').isDisabled());
    // With no run in flight the button works again; when the person turned the judges off it is refused by design.
    await p.evaluate(() => { window.__calibrationState = { state: 'idle', at: '', chosen: '' }; __emit('judgements:changed', { stateGen: 'g1', stateVer: 3, state: 'on', why: '', calls: 12, fallbacks: 0, streak: 0, warnStreak: 3, latest: new Date().toISOString(), budget: [], pool: { size: 1, min: 1, max: 4, growAt: 0.5, shrinkAfter: 10, inFlight: 0 }, calibration: window.__calibrationState }); });
    await p.waitForFunction(() => !/running now/.test(document.getElementById('judgement-calibration-line').textContent));
    ok('the button is offered again when no run is in flight', await settings.locator('#judgements-run-calibration').isEnabled());
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
