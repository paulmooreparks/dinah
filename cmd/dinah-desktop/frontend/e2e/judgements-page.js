// The Judgements page (judgements-applet.js): the graph over the days, the
// breakdown by kind and model, the paged and filtered list of decisions, the
// detail with what the judge saw and what the rule would have said, the
// correction, and the empty state before anything has been judged.
const { launch } = require('./lib/page');
(async () => {
  function fixture() {
    // Records are made in the page, since the fixture's app function is sent
    // there on its own and nothing from the suite's scope reaches it.
    window.__mk = i => {
      const at = new Date(); at.setDate(at.getDate() - Math.floor((i - 1) / 8)); at.setHours(Math.min(9, new Date().getHours()), 0, 0, 0);
      const judge = i % 4 !== 0;
      return {
        id: i, time: at.toISOString(), kind: i % 2 ? 'chat-title' : 'mention-choice', version: 'v' + ((i % 2) + 1),
        input: JSON.stringify({ line: 'Message ' + i }), answer: judge ? 'Judge Answer ' + i : '',
        ruleAnswer: JSON.stringify('Rule ' + i), out: JSON.stringify(judge ? 'Judge Answer ' + i : 'Rule ' + i),
        used: judge ? 'judge' : 'rule', why: judge ? '' : 'late',
        provider: judge ? 'ollama' : '', model: judge ? (i % 8 < 5 ? 'kimi-k3:cloud' : 'deepseek-v4.1-flash:cloud') : '',
        temperature: 0, seed: 42, deadlineMs: 2000, elapsedMs: 100 + i, costUnit: judge ? 'tokens' : '', costIn: judge ? 200 + i : 0, costOut: judge ? 20 : 0,
        effectKind: 'chat', effectId: 'chat-' + i, correction: i === 5 ? 'A Fixed Title' : '', ...(i === 5 ? { correctedAt: at.toISOString() } : {}),
      };
    };
    window.__records = sessionStorage.getItem('judgements-fixture') ? Array.from({ length: 52 }, (_, i) => window.__mk(i + 1)) : [];
    window.__recordArgs = [];
    window.__corrected = [];
    const kinds = [
      { name: 'chat-title', version: 'v1', describe: 'The title is the first line of your words, cleaned.', correctable: true, deadlineMs: 2000, enabled: true, input: [{ name: 'line', type: 'string', doc: 'the first line of the message that started the chat' }] },
      { name: 'mention-choice', version: 'v2', describe: 'The members a message with no mentions wakes, as a suggestion.', correctable: false, deadlineMs: 3000, enabled: true, input: [{ name: 'members', type: '[]string', doc: 'the members of the group' }] },
    ];
    return {
      JudgementState: async () => ({ state: 'on', why: '', calls: window.__records.length, fallbacks: window.__records.filter(r => r.used === 'rule').length, streak: 0, latest: '' }),
      JudgementSummary: async (from, to) => {
        const f = from ? Date.parse(from) : null, t2 = to ? Date.parse(to) : null;
        const rows = window.__records.filter(r => (!f || Date.parse(r.time) >= f) && (!t2 || Date.parse(r.time) < t2));
        const groups = {};
        rows.forEach(r => {
          const key = r.kind + '|' + r.model + '|' + r.costUnit;
          const g = groups[key] || (groups[key] = { kind: r.kind, model: r.model, costUnit: r.costUnit, calls: 0, fallbacks: 0, disagreed: 0, corrected: 0, avgMs: 0, maxMs: 0, costIn: 0, costOut: 0 });
          g.calls++; if (r.used === 'rule') g.fallbacks++;
          if (r.used === 'judge' && r.out !== r.ruleAnswer) g.disagreed++;
          if (r.correction) g.corrected++;
          g.avgMs += (r.elapsedMs - g.avgMs * (g.calls - 1)) / g.calls;
          g.maxMs = Math.max(g.maxMs, r.elapsedMs);
          g.costIn += r.costIn; g.costOut += r.costOut;
        });
        const sorted = rows.slice().sort((a, b) => b.id - a.id);
        let streak = 0;
        for (const r of sorted) { if (r.used === 'judge') break; if (r.why !== 'off') streak++; }
        return { from, to, calls: rows.length, fallbacks: rows.filter(r => r.used === 'rule').length,
          groups: Object.values(groups).map(g => Object.assign(g, { avgMs: Math.round(g.avgMs) })),
          latest: sorted.length ? sorted[0].time : '', streak };
      },
      JudgementRecords: async f => {
        window.__recordArgs.push(f);
        let list = window.__records.slice().sort((a, b) => b.id - a.id);
        if (f.kind) list = list.filter(r => r.kind === f.kind);
        if (f.used) list = list.filter(r => r.used === f.used);
        if (f.model) list = list.filter(r => r.model === f.model);
        if (f.from) list = list.filter(r => Date.parse(r.time) >= Date.parse(f.from));
        if (f.to) list = list.filter(r => Date.parse(r.time) < Date.parse(f.to));
        if (f.before) list = list.filter(r => r.id < f.before);
        const limit = f.limit || 50;
        return { records: list.slice(0, limit), next: list.length > limit ? list[limit - 1].id : 0 };
      },
      JudgementKinds: async () => kinds,
      JudgementSettings: async () => ({ settings: { enabled: true, provider: 'ollama', model: 'kimi-k3:cloud', retentionDays: 90, warnStreak: 3, kindsOff: [] }, revision: 'r1', conflict: false }),
      JudgementModels: async () => [{ name: 'kimi-k3:cloud', provider: 'ollama' }],
      SetJudgementSettings: async s => s,
      CorrectJudgement: async (id, value) => {
        window.__corrected.push([id, value]);
        const r = window.__records.find(x => x.id === id);
        r.correction = value; r.correctedAt = new Date().toISOString();
        return r;
      },
    };
  }
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: fixture });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahWorkbench.open('judgements'));
    const win = p.locator('.win[data-win-kind="judgements"]');
    await win.waitFor({ state: 'attached' });
    await win.locator('#win-max-judgements').click();
    await win.locator('.judgements-total').waitFor();
    ok('before any call, the page says so plainly', /Nothing has been judged yet/.test(await win.locator('.judgements-total').textContent()));
    ok('the graph and the tables have nothing to draw, and the list says it', (await win.locator('.judgements-graph').count()) === 0 && (await win.locator('.judgements-breakdown .data-table-empty').count()) === 2 && (await win.locator('.judgements-list .judgements-empty').textContent()).includes('No decisions to show') && await win.locator('.judgements-more').isHidden());

    // Calls arrive; the numbered event refreshes the page in place.
    await p.evaluate(() => { sessionStorage.setItem('judgements-fixture', '1'); window.__records = Array.from({ length: 52 }, (_, i) => window.__mk(i + 1)); __emit('judgements:changed', { id: 52, stateGen: 1, stateVer: 1, state: 'on', why: '', calls: 52, fallbacks: 13, streak: 0, warnStreak: 3, latest: new Date().toISOString() }); });
    await win.locator('.judgements-graph .jbar').first().waitFor();
    ok('the graph draws one bar per day', (await win.locator('.judgements-graph .jbar').count()) === 7);
    ok('graph controls have raised button surfaces and a pattern legend', await win.locator('.jbar.btn').count() === 7 && /solid.*striped/.test(await win.locator('.judgements-graph-legend').textContent()) && await win.locator('.jbar-fallback').first().evaluate(n => getComputedStyle(n).backgroundImage.includes('gradient')));

    const todaybar = win.locator('.judgements-graph .jbar').last();
    ok('a bar with fallbacks shows the rule’s share and says the counts', (await todaybar.locator('.jbar-fallback').count()) === 1 && /8 calls, 2 by the rule/.test(await todaybar.getAttribute('aria-label')), await todaybar.getAttribute('aria-label'));
    ok('the totals line counts the range', /52 calls in the last 7 days: 39 answered by the judge, 13 by the rule/.test(await win.locator('.judgements-total').textContent()), await win.locator('.judgements-total').textContent());
    const breakdown = await win.locator('.judgements-breakdown').textContent();
    ok('the breakdown is by kind and by model', /chat-title/.test(breakdown) && /kimi-k3:cloud/.test(breakdown) && /deepseek-v4.1-flash:cloud/.test(breakdown), breakdown.slice(0, 400));
    const rows = win.locator('.judgements-list [data-judgement]');
    await rows.nth(49).waitFor();
    ok('the list pages at fifty with More under it', (await rows.count()) === 50 && await win.locator('.judgements-more').isVisible());
    await win.locator('.judgements-more').click();
    await rows.nth(51).waitFor();
    ok('More brings the rest with the cursor', (await rows.count()) === 52 && await win.locator('.judgements-more').isHidden() && (await p.evaluate(() => __recordArgs[__recordArgs.length - 1].before)) === 3, JSON.stringify(await p.evaluate(() => __recordArgs.slice(-2))));
    // The filters go to the bridge as the filter's fields.
    await win.getByLabel('Answered by', { exact: true }).selectOption('rule');
    await p.waitForFunction(() => __recordArgs[__recordArgs.length - 1].used === 'rule');
    ok('the answer filter narrows the list to the rule’s', (await rows.count()) === 13);
    await win.getByLabel('Answered by', { exact: true }).selectOption('');
    await win.getByLabel('Kind', { exact: true }).selectOption('chat-title');
    await p.waitForFunction(() => { const a = __recordArgs[__recordArgs.length - 1]; return a.kind === 'chat-title' && !a.used; });
    ok('the kind filter narrows it to the titles', (await rows.count()) === 26, String(await rows.count()));
    // A bar filters the list to its day, pressed as its state says.
    await todaybar.click();
    await p.waitForFunction(() => { const a = __recordArgs[__recordArgs.length - 1]; return !!a.from && !!a.to; });
    ok('pressing a bar filters the list to its day and stays pressed', (await todaybar.getAttribute('aria-pressed')) === 'true' && (await rows.count()) === 4, String(await rows.count()));
    const dayBefore = await p.evaluate(() => ({ query: __recordArgs[__recordArgs.length - 1], ids: Array.from(document.querySelectorAll('.judgements-list [data-judgement]'), n => n.dataset.judgement) }));
    await p.reload();
    await t.waitReady();
    await win.locator('.jbar[aria-pressed="true"]').waitFor();
    await rows.nth(3).waitFor();
    const dayAfter = await p.evaluate(() => ({ query: __recordArgs[__recordArgs.length - 1], ids: Array.from(document.querySelectorAll('.judgements-list [data-judgement]'), n => n.dataset.judgement) }));
    ok('reloading a chosen day keeps local midnight bounds, the pressed bar and morning records', dayBefore.query.from === dayAfter.query.from && dayBefore.query.to === dayAfter.query.to && JSON.stringify(dayBefore.ids) === JSON.stringify(dayAfter.ids) && dayAfter.ids.includes('1'), JSON.stringify({ dayBefore, dayAfter }));
    await p.evaluate(() => { document.documentElement.dataset.theme = 'dark'; });
    await t.shot('judgements-page-dark-selected-day');
    await p.evaluate(() => { document.documentElement.dataset.theme = 'light'; });
    await win.locator('.jbar[aria-pressed="true"]').click();
    await p.waitForFunction(() => !__recordArgs[__recordArgs.length - 1].from);
    ok('pressing it again clears the day', (await rows.count()) === 26);

    // A decision's detail: what the judge saw, its answer, the rule's, what was used.
    await p.evaluate(() => { const tr = document.querySelector('[data-judgement="51"]'); tr.scrollIntoView(); tr.querySelector('a').click(); });
    const detail = win.locator('.judgement-detail');
    await detail.waitFor();
    ok('the detail says the kind, the version and how it was answered', /chat-title/.test(await detail.locator('.judgement-detail-title').textContent()) && /version v2/.test(await detail.textContent()) && /ollama kimi-k3:cloud \(temperature 0, seed 42\)/.test(await detail.textContent()));
    const detailText = await detail.textContent();
    await t.shot('judgements-page-detail');
    ok('the detail shows what the judge saw, what it answered, the rule’s answer and what was used', /Message 51/.test(detailText) && /Judge Answer 51/.test(detailText) && /Rule 51/.test(detailText), detailText.slice(0, 500));
    // Where the decision went is a link to the chat it named.
    ok('the decision names where it went', /Where It Went/.test(detailText) && /chat-51/.test(detailText));
    await p.evaluate(() => { window.__opened = []; window.dinahOpenChat = id => { window.__opened.push(id); return Promise.resolve(); }; });
    await detail.getByRole('link', { name: /chat-51/ }).click();
    ok('and the link opens that chat', JSON.stringify(await p.evaluate(() => window.__opened)) === JSON.stringify(['chat-51']), JSON.stringify(await p.evaluate(() => window.__opened)));
    // J3 and J6: the decision opens beside its row, and the page's address
    // says the filters, the day and the decision, so a reload keeps them.
    await win.locator('[data-judgement="51"] a').click();
    await p.waitForFunction(() => { const w = document.querySelector('.win[data-win-kind="judgements"]'); return w && w.querySelector('.judgement-detail') && !w.querySelector('.judgement-detail').hidden; });
    const sideBySide = await p.evaluate(() => {
      const win = document.querySelector('.win[data-win-kind="judgements"]');
      const list = win.querySelector('.judgements-list').getBoundingClientRect();
      const detail = win.querySelector('.judgement-detail').getBoundingClientRect();
      return { listRight: list.right, detailLeft: detail.left, selected: !!win.querySelector('.judgements-list a[aria-current="true"]') };
    });
    ok('a decision opens beside its row, marked in it', sideBySide.selected && sideBySide.detailLeft > sideBySide.listRight - 14, JSON.stringify(sideBySide));
    const address = await p.evaluate(() => decodeURIComponent(location.search));
    ok('and it is in the address its page would reload from', /"k":"chat-title"/.test(address) && /"open":\d+/.test(address), address.slice(0, 300));
    await t.shot('judgements-page-master-detail');
    // The same address, re-read after a page reload.
    await p.goto(p.url());
    await t.waitReady();
    await p.waitForFunction(() => {
      const win = document.querySelector('.win[data-win-kind="judgements"]');
      return win && win.querySelector('.judgements-list [data-judgement]') && !!win.querySelector('.judgement-detail:not([hidden])');
    }, null, { timeout: 25000 });
    ok('a reload keeps the filters and the decision open on its row', await p.evaluate(() => {
      const win = document.querySelector('.win[data-win-kind="judgements"]');
      if (!win) return false;
      const kindSel = win.querySelector('[aria-label="Kind"]');
      const detail = win.querySelector('.judgement-detail:not([hidden])');
      const selected = win.querySelector('.judgements-list a[aria-current="true"]');
      return !!detail && !!selected && kindSel && kindSel.value === 'chat-title';
    }));
    await detail.locator('button', { hasText: /^Correct/ }).click();
    await detail.getByLabel('Your correction').fill('A Better Title');
    await detail.getByRole('button', { name: 'Save' }).click();
    await p.waitForFunction(() => __corrected.length === 1);
    await detail.getByText('Your Correction').waitFor();
    ok('a correction is saved to the record and shown with it', JSON.stringify(await p.evaluate(() => __corrected)) === JSON.stringify([[51, 'A Better Title']]) && /A Better Title/.test(await detail.textContent()) && await win.locator('[data-judgement="51"] .badge', { hasText: 'Corrected' }).count() === 1);
    // A record already corrected shows it; a kind that is not correctable offers none.
    await p.evaluate(() => { const tr = document.querySelector('[data-judgement="5"]'); tr.scrollIntoView(); tr.querySelector('a').click(); });
    await detail.getByText('Your Correction').waitFor();
    ok('a corrected record shows the correction and when', /A Fixed Title/.test(await detail.textContent()));
    await win.getByLabel('Kind', { exact: true }).selectOption('');
    await rows.nth(49).waitFor();
    await win.locator('.judgements-more').click();
    await rows.nth(51).waitFor();
    await p.evaluate(() => { const tr = document.querySelector('[data-judgement="52"]'); tr.scrollIntoView(); tr.querySelector('a').click(); });
    await detail.getByText('What the Judge Saw').waitFor();
    ok('a kind that cannot be corrected offers no correction', (await detail.locator('button', { hasText: /^Correct/ }).count()) === 0, await detail.textContent());
    // A decision on a later page must also survive a direct address.
    await win.locator('[data-judgement="2"] a').click();
    await p.reload();
    await t.waitReady();
    await win.locator('[data-judgement="2"] a[aria-current="true"]').waitFor();
    ok('a reload restores the second page and its selected decision', await rows.count() === 52 && /Message 2/.test(await detail.textContent()));
    // PUDL chooses one pane at a time in a narrow window.
    await p.setViewportSize({ width: 560, height: 900 });
    await win.locator('.md-back').waitFor();
    await win.locator('.md-back').click();
    await win.locator('[data-judgement="2"] a').click();
    await detail.scrollIntoViewIfNeeded();
    ok('a narrow window shows the selected record with a way back', await detail.isVisible() && await win.locator('.md-sidebar').isHidden());
    await t.shot('judgements-page-narrow-detail');
    await win.locator('.md-back').click();
    await win.locator('.md-sidebar').waitFor();
    ok('Back to Decisions restores the narrow list and clears the address selection', await detail.isHidden() && JSON.parse(new URL(await p.url()).searchParams.get('view.judgements')).open === 0);
    await t.shot('judgements-page-narrow-list');

  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
