// The page's side of an Ollama chat's context window, built against the fields the engine agreed to send
// (docs/plans/ollama-context-window.md): the meter says whether its window is known; a send held because the
// context is nearly full shows a card with the courses, picking one remembers it, a remembered choice is
// changed from the chat's menu; and what Dinah did (a dinah_status update) is drawn once as a status row.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    app: () => {
      window.__calls = [];
      return {
        AgentProviders: async () => [window.__provider('ollama')],
        OpenSessionAt: async (ws, id) => ({ sessionId: id, provider: 'ollama', cwd: ws, open: true, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } }),
        ResolveHold: async (id, action) => { window.__calls.push(['resolve', id, action]); if (window.__failResolve) throw new Error('Fixture: nothing is held'); return action === 'compact' ? { sessionId: 'ollama:ctx2', provider: 'ollama', cwd: 'C:/Fixture', open: true } : { sessionId: id }; },
        ForgetContextChoice: async id => { window.__calls.push(['forget', id]); }
      };
    },
  });
  const { page, ok } = t;
  try {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'ollama:ctx' }));
    const win = page.locator('[data-win="' + key + '"]');
    await page.waitForFunction(k => /Idle/.test(document.querySelector('[data-win="' + k + '"] [data-view-id=page-state]').textContent), key);
    const session = (extra, ver) => page.evaluate(([extra, ver]) => __emit('agent:session', Object.assign({ sessionId: 'ollama:ctx', provider: 'ollama', cwd: 'C:/Fixture', open: true, stateGen: 'g', stateVer: ver }, extra)), [extra, ver]);
    const usage = (used, size, sizeKind, windowKnown) => page.evaluate(([used, size, sizeKind, windowKnown]) => __emit('agent:update', { sessionId: 'ollama:ctx', update: { sessionUpdate: 'usage_update', used, size, sizeKind, windowKnown } }), [used, size, sizeKind, windowKnown]);
    const meter = k => win.locator('[data-view-id="usage-button"]');
    const title = async () => meter().getAttribute('title');
    const wait = pattern => page.waitForFunction(([k, pattern]) => new RegExp(pattern).test(document.querySelector('[data-win="' + k + '"] [data-view-id="usage-button"]').title), [key, pattern]);

    // The meter's states.
    await usage(40000, 262144, 'model', true);
    await wait('context window');
    ok('a cloud model on the provider\'s setting has a known window, said as the model\'s context window', /the model's context window of 262,144 tokens/.test(await title()) && !/may be lower/.test(await title()), await title());
    await usage(40000, 262144, 'model', false);
    await wait('may be lower');
    ok('a local model on the provider\'s setting has only the model\'s limit, with the caveat', /the model's limit of 262,144 tokens/.test(await title()) && /may be lower/.test(await title()), await title());
    await usage(70000, 65536, 'chosen', true);
    await wait('over');
    ok('a known window passed is Over, and says so in words', /Over/.test(await meter().innerText()) && /context length of 65,536/.test(await title()), await title());
    await usage(300000, 262144, 'model', false);
    await wait('may be lower');
    ok('an unknown window is never called Over, since the limit is not the window', !/Over/.test(await meter().innerText()), await meter().innerText());

    // What Dinah did is a status row, drawn once.
    const say = text => page.evaluate(text => __emit('agent:update', { sessionId: 'ollama:ctx', gen: 'g1', n: window.__n = (window.__n || 0) + 1, update: { sessionUpdate: 'dinah_status', text, kind: 'warning' } }), text);
    await say('The earliest 14 turns are set aside, and the rest of the chat goes on.');
    await win.getByText('The earliest 14 turns are set aside').waitFor({ state: 'attached' });
    ok('a status update is drawn as one status row', await win.locator('.status-row', { hasText: 'earliest 14 turns' }).count() === 1);

    // The hold.
    ok('with nothing held there is no card', await win.locator('[data-view-id="context-hold"]').isHidden());
    await session({ hold: { kind: 'context', used: 235000, size: 262144, actions: ['set-aside', 'compact'] } }, 1);
    const card = win.locator('[data-view-id="context-hold"]');
    await card.waitFor();
    ok('a held send shows the card with the numbers, and the choice is said to be remembered', /235,000 of 262,144/.test(await card.innerText()) && /remembered/.test(await card.innerText()), await card.innerText());
    ok('it offers the courses, with setting turns aside first and primary', await card.locator('button').allInnerTexts().then(l => l.join('|') === 'Set the Oldest Turns Aside and Continue|Compact Into a Continuation|Start a New Chat') &&
      await card.locator('[data-view-id="hold-set-aside"]').evaluate(el => el.classList.contains('btn-primary')));
    ok('the card names no provider', !/Ollama|Claude|Codex|Devin/.test(await card.innerText()));
    await card.getByRole('button', { name: 'Compact Into a Continuation' }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'resolve'));
    ok('picking a course asks the engine to resolve the hold with it', JSON.stringify(await page.evaluate(() => __calls.filter(c => c[0] === 'resolve'))) === JSON.stringify([['resolve', 'ollama:ctx', 'compact']]));
    ok('a compaction opens the chat the held message went to', await page.waitForFunction(() => document.querySelector('[data-win]') && [...document.querySelectorAll('[data-win]')].length > 1).then(() => true));
    await session({ choice: 'compact' }, 2);
    await card.waitFor({ state: 'hidden' });
    ok('when the engine has released the hold the card goes', true);

    await page.evaluate(k => pudlWindows.raise(k), key);
    // A refusal is said in the card and the courses stay.
    await page.evaluate(() => { window.__failResolve = true; });
    await session({ hold: { kind: 'context', used: 240000, size: 262144, actions: ['set-aside'] }, choice: '' }, 3);
    await card.waitFor();
    ok('a hold that offers one course shows only that one', await card.locator('[data-view-id="hold-compact"]').isHidden() && await card.locator('[data-view-id="hold-set-aside"]').isVisible());
    await card.locator('[data-view-id="hold-set-aside"]').click();
    await card.locator('[data-view-id="context-hold-status"]', { hasText: 'could not be done' }).waitFor();
    ok('a refused choice is said in the card, which stays', /nothing is held/.test(await card.locator('[data-view-id="context-hold-status"]').innerText()) && await card.isVisible());
    await session({ choice: '' }, 4);

    // The remembered choice is changed from the chat's menu.
    const menu = async () => {
      await page.evaluate(k => pudlWindows.raise(k), key);
      await page.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click();
      const item = page.locator('#menubar-panel .menu-action', { hasText: 'Ask Me Again About a Full Context' });
      const n = await item.count();
      if (n) await item.first().click(); else await page.keyboard.press('Escape');
      return n;
    };
    ok('with no choice remembered the menu does not offer to ask again', await menu() === 0);
    await session({ choice: 'set-aside' }, 5);
    await page.waitForTimeout(200);
    ok('with a choice remembered it does, and choosing it asks the engine to forget the choice', await menu() === 1 && await page.evaluate(() => __calls.some(c => c[0] === 'forget' && c[1] === 'ollama:ctx')));
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
