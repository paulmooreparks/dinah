// Settings › Judgements (judgements-applet.js): the off switch, the model
// picker over the models Ollama reports, the retention and the warn streak,
// each kind's switch and what the judge may see, the settings saved against
// their revision, and a stale page losing to the settings as they now stand.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__settings = { enabled: true, provider: 'ollama', model: 'kimi-k3:cloud', retentionDays: 90, warnStreak: 3, kindsOff: [] };
      window.__revision = 'rev1';
      window.__setCalls = [];
      window.__rev = 1;
      window.__conflictNext = false;
      const kinds = [
        { name: 'chat-title', version: 'v1', describe: 'The title is the first line of your words, cleaned.', correctable: true, deadlineMs: 2000, enabled: true, input: [{ name: 'line', type: 'string', doc: 'the first line of the message that started the chat' }] },
        { name: 'mention-choice', version: 'v2', describe: 'The members a message with no mentions wakes, as a suggestion.', correctable: false, deadlineMs: 3000, enabled: true, input: [{ name: 'members', type: '[]string', doc: 'the members of the group' }, { name: 'text', type: 'string', doc: 'the first line of the message' }] },
      ];
      return {
        JudgementState: async () => ({ stateGen: 1, stateVer: 1, state: window.__settings.enabled ? 'on' : 'off', why: window.__settings.enabled ? '' : 'turned off', calls: 0, fallbacks: 0, streak: 0, warnStreak: 3, latest: '' }),
        JudgementSummary: async () => ({ from: '', to: '', calls: 0, fallbacks: 0, groups: [], latest: '', streak: 0 }),
        JudgementRecords: async () => ({ records: [], next: 0 }),
        JudgementKinds: async () => kinds,
        JudgementSettings: async () => ({ settings: window.__settings, revision: window.__revision, conflict: false }),
        JudgementModels: async () => [{ name: 'kimi-k3:cloud', provider: 'ollama' }, { name: 'deepseek-v4.1-flash:cloud', provider: 'ollama' }],
        SetJudgementSettings: async (s, revision) => {
          window.__setCalls.push([s, revision]);
          if (window.__conflictNext) {
            // Another writer moved first: nothing is written, and the answer
            // carries the settings as they now stand.
            window.__conflictNext = false;
            window.__revision = 'rev-other';
            return { settings: window.__settings, revision: window.__revision, conflict: true };
          }
          if (!Number.isInteger(s.retentionDays) || s.retentionDays < 1) throw new Error('keep records for at least a day');
          window.__settings = s;
          window.__revision = 'rev' + (++window.__rev);
          return { settings: window.__settings, revision: window.__revision, conflict: false };
        },
      };
    },
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahWorkbench.open('settings'));
    const win = p.locator('.win[data-win-kind="settings"]');
    await win.waitFor({ state: 'attached' });
    await win.locator('[data-settings-section="judgements"]').click();
    const card = win.locator('#judgements-card');
    await card.waitFor();
    const toggle = card.locator('#judgements-enabled');
    const model = card.locator('#judgement-model');
    const status = card.locator('#judgements-status');
    await p.waitForFunction(() => document.querySelectorAll('#judgement-model option').length === 3);
    ok('the section shows the on switch, the chosen model and the two numbers', (await toggle.getAttribute('aria-checked')) === 'true' && (await model.inputValue()) === 'kimi-k3:cloud' && (await card.locator('#judgement-retention').inputValue()) === '90' && (await card.locator('#judgement-warn-streak').inputValue()) === '3');
    ok('the picker offers the models Ollama reports, with none chosen first', /No model yet; every call uses its rule/.test(await model.locator('option').first().textContent()));
    // The kinds are listed with what they say about themselves, and their switches.
    const kinds = card.locator('#judgement-kinds > li');
    await kinds.nth(1).waitFor();
    const mention = card.locator('#judgement-kinds > li[data-kind="mention-choice"]');
    ok('each kind is listed with its description, its deadline and a switch', /The members a message/.test(await mention.textContent()) && /allowed 3 s/.test(await mention.textContent()) && (await mention.locator('[role="switch"]').getAttribute('aria-checked')) === 'true');
    await mention.locator('button.disclosure').click();
    ok('what the judge may see is one disclosure per kind', /the members of the group/.test(await mention.locator('.judgement-may-see').textContent()));
    const disclosure = mention.locator('button.disclosure');
    const controlsMatch = () => disclosure.evaluate(button => {
      const body = document.getElementById(button.getAttribute('aria-controls'));
      return !!body && body.hidden === (button.getAttribute('aria-expanded') !== 'true');
    });
    ok('the open disclosure names its visible body', await controlsMatch());
    const bodyId = await disclosure.getAttribute('aria-controls');
    await disclosure.click();
    ok('closing the disclosure hides the same named body', await controlsMatch() && await disclosure.getAttribute('aria-controls') === bodyId && await disclosure.getAttribute('aria-expanded') === 'false');
    await disclosure.click();

    ok('switches use tracks and the disclosure keeps programming types out', await toggle.locator('.switch-thumb').count() === 1 && await mention.locator('.switch-thumb').count() === 1 && !/string|—/.test(await mention.locator('.judgement-may-see').textContent()));
    await t.shot('judgements-settings-section');
    // Every control saves the values against the revision they were read at.
    await toggle.click();
    await p.waitForFunction(() => __setCalls.length === 1);
    ok('the off switch saves at once, against the revision read', (await p.evaluate(() => __setCalls[0][0].enabled)) === false && (await p.evaluate(() => __setCalls[0][1])) === 'rev1' && /Every call uses its rule now/.test(await status.textContent()));
    ok('and the switch reads off', (await toggle.getAttribute('aria-checked')) === 'false');
    await model.selectOption('deepseek-v4.1-flash:cloud');
    await p.waitForFunction(() => __setCalls.length === 2);
    ok('choosing a model saves it against the newer revision', (await p.evaluate(() => __setCalls[1][0].model)) === 'deepseek-v4.1-flash:cloud' && (await p.evaluate(() => __setCalls[1][1])) === 'rev2' && /deepseek-v4.1-flash:cloud answers the judgement calls now/.test(await status.textContent()));
    // The two numbers are checked before they are saved.
    await card.locator('#judgement-retention').fill('0');
    await card.locator('#judgements-save-numbers').click();
    ok('a retention under a day is refused before any save', /at least/i.test(await status.textContent()) && (await p.evaluate(() => __setCalls.length)) === 2);
    await card.locator('#judgement-retention').fill('30');
    await card.locator('#judgement-warn-streak').fill('5');
    await card.locator('#judgements-save-numbers').click();
    await p.waitForFunction(() => __setCalls.length === 3);
    ok('good numbers save and the status says them back', (await p.evaluate(() => __setCalls[2][0].retentionDays)) === 30 && (await p.evaluate(() => __setCalls[2][0].warnStreak)) === 5 && /kept for 30 days/.test(await status.textContent()) && /after 5 fallbacks in a row/.test(await status.textContent()), await status.textContent());
    // A kind's switch keeps the kind's name in kindsOff.
    await mention.locator('[role="switch"]').click();
    await p.waitForFunction(() => __setCalls.length === 4);
    ok('a kind’s switch writes kindsOff', JSON.stringify(await p.evaluate(() => __setCalls[3][0].kindsOff)) === JSON.stringify(['mention-choice']) && /mention-choice now uses its rule/.test(await status.textContent()));
    ok('and shows the kind off after the save', (await mention.locator('[role="switch"]').getAttribute('aria-checked')) === 'false');
    // A stale page loses: nothing is written, and the settings as they now
    // stand come back with conflict true and are shown.
    await p.evaluate(() => { window.__conflictNext = true; });
    await toggle.click();
    await p.waitForFunction(() => __setCalls.length === 5);
    await p.waitForFunction(() => /changed since you read them/.test(document.getElementById('judgements-status').textContent));
    ok('a stale save writes nothing and the newer settings are shown', /changed since you read them; the newer ones are shown/.test(await status.textContent()) && (await toggle.getAttribute('aria-checked')) === 'false' && (await p.evaluate(() => __revision)) === 'rev-other', await status.textContent());
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
