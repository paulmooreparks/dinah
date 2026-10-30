// Large output and large histories: paged oversized messages, offscreen
// rendering of a long saved copy, scroll discovery, and the responsiveness
// diagnostic in Settings.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1200, height: 850 },
    app: () => ({
      SavedConversation: async id => id === 'codex:history' ? { available: true, complete: true, messages: Array.from({ length: 1200 }, (_, i) => ({ role: i % 2 ? 'agent' : 'you', text: 'Saved row ' + i + ' ' + 'x'.repeat(1000) })) } : { available: false },
      // A restored chat reconnects by itself a moment after its saved copy
      // is drawn, and the agent's empty replay then takes the saved copy's
      // place. On a busy machine that can come in the middle of the checks
      // on the saved copy, so the reconnect waits here until they are done.
      OpenSessionAt: async (cwd, sessionId) => { if (sessionId === 'codex:history') await window.__reconnect; return { sessionId, cwd, provider: 'codex', open: true, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } }; }
    }),
    init: () => { window.__reconnect = new Promise(resolve => { window.__letReconnect = resolve; }); },
    ready: () => document.querySelector('.chat-instance'),
  });
  const { page: p, ok, errors } = t;
  try {
    await p.evaluate(async () => {
      await dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:tail' });
      pudlWindows.snap(dinahWorkbench.key({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:tail' }), 'maximized');
      window.__more = () => __emit('agent:update', { sessionId: 'codex:tail', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'A paragraph of streamed output.\n\n'.repeat(60) } } });
    });
    await p.waitForFunction(() => /Idle/.test(document.querySelector('.win[data-chat-current] .chat-working-chip').title));
    const prompt = p.locator('.win[data-chat-current] [data-view-id=prompt]');
    await p.evaluate(() => {
      window.__huge = 'BEGIN\n' + ('A large message line.\n'.repeat(30000)) + 'END';
      __emit('agent:update', { sessionId: 'codex:tail', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: __huge } } });
    });
    const content = p.locator('.win[data-chat-current] .msg-text').last();
    await content.getByRole('button', { name: 'First', exact: true }).waitFor();
    ok('an oversized message has a bounded visible page ending in the newest output', (await content.textContent()).length < 17000 && (await content.textContent()).endsWith('END'));
    await content.getByRole('button', { name: 'First', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.win[data-chat-current] .msg-text').textContent.includes('BEGIN'));
    await prompt.fill('Draft survives large output');
    await p.evaluate(() => __emit('agent:update', { sessionId: 'codex:tail', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'MORE' } } }));
    await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
    ok('reading an earlier page remains stable while output continues', (await content.textContent()).includes('BEGIN') && await prompt.inputValue() === 'Draft survives large output');
    await content.getByRole('button', { name: 'Latest', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.win[data-chat-current] .msg-text').textContent.endsWith('ENDMORE'));
    ok('the latest page includes subsequent output', true);
    await content.getByRole('button', { name: 'Previous', exact: true }).click();
    await content.getByRole('button', { name: 'Next', exact: true }).waitFor({ state: 'visible' });
    await p.waitForFunction(() => ![...document.querySelectorAll('.win[data-chat-current] .msg-text button')].find(b => b.textContent === 'Next').disabled);
    await content.getByRole('button', { name: 'Next', exact: true }).click();
    await p.waitForFunction(() => [...document.querySelectorAll('.win[data-chat-current] .msg-text button')].find(b => b.textContent === 'Next').disabled);
    ok('Latest remains available after manually reaching the last page', await content.getByRole('button', { name: 'Latest', exact: true }).isEnabled());
    await content.getByRole('button', { name: 'Latest', exact: true }).click();
    await p.waitForFunction(() => [...document.querySelectorAll('.win[data-chat-current] .msg-text button')].find(b => b.textContent === 'Latest').disabled);
    await p.evaluate(() => __emit('agent:update', { sessionId: 'codex:tail', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'x'.repeat(20000) + 'FOLLOWED' } } }));
    await p.waitForFunction(() => document.querySelector('.win[data-chat-current] .msg-text').textContent.endsWith('FOLLOWED'));
    ok('Latest resumes following across a new page boundary', true);
    const history = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:history' }));
    const historyPrompt = p.locator('[data-win="' + history + '"] [data-view-id=prompt]');
    await historyPrompt.waitFor();
    await historyPrompt.pressSequentially('Typing during restoration', { delay: 5 });
    ok('the composer accepts typing while a large history restores', await historyPrompt.inputValue() === 'Typing during restoration');
    await p.waitForFunction(k => document.querySelector('[data-win="' + CSS.escape(k) + '"] [data-view-id=logs]').textContent.includes('Saved row 1199'), history);
    ok('saved history reconstruction eventually reaches its final entry', true);
    const historyWin = p.locator('[data-win="' + history + '"]');
    await historyWin.locator('.msg-text').last().waitFor();
    // A saved copy draws its latest 400 messages (drawSaved); those are kept
    // on the page and rendered offscreen rather than removed.
    ok('messages use offscreen rendering without removing DOM', await historyWin.locator('.activity-row:not(.trimmed-row)').first().evaluate(el => getComputedStyle(el).contentVisibility === 'auto') && await historyWin.locator('.msg-text').count() >= 400);
    ok('offscreen message text remains selectable', await historyWin.locator('.msg-text').first().evaluate(el => { const range = document.createRange(); range.selectNodeContents(el); const selection = getSelection(); selection.removeAllRanges(); selection.addRange(range); const matches = selection.toString().includes('Saved row'); selection.removeAllRanges(); return matches; }));
    await historyWin.locator('.msg-head .msg-copy').first().focus();
    ok('offscreen message controls remain focusable', await historyWin.locator('.msg-head .msg-copy').first().evaluate(el => el === document.activeElement));
    await p.evaluate(() => window.__letReconnect());

    const watches = await p.evaluate(async k => {
      const win = document.querySelector('[data-win="' + CSS.escape(k) + '"]'), original = dinahUIState.watchScroll, watched = [];
      dinahUIState.watchScroll = function(el, ...args) { watched.push(el.id); return original.call(this,el,...args); };
      const make = id => { const el=document.createElement('div');el.id=id;el.style.cssText='overflow:auto;height:20px';el.textContent='fixture';return el; };
      const transcript=make('transcript-scroll-fixture'), applet=make('applet-scroll-fixture');
      win.querySelector('.run-log').append(transcript);win.querySelector('.dinah-applet').append(applet);
      await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
      dinahUIState.watchScroll=original;transcript.remove();applet.remove();return watched;
    }, history);
    ok('scroll discovery skips transcript internals but still watches applet scrollers', !watches.includes('transcript-scroll-fixture') && watches.includes('applet-scroll-fixture'));
    await p.evaluate(() => dinahWorkbench.open('settings'));
    await p.getByRole('tab', { name: 'Diagnostics', exact: true }).click();
    const toggle = p.locator('#set-responsiveness');
    ok('responsiveness recording defaults off', await toggle.getAttribute('aria-checked') === 'false');
    await toggle.click();
    await p.evaluate(() => dinahRender.enqueue({}, null, () => { const end = performance.now() + 75; while (performance.now() < end) {} }, null, 'fixture work'));
    await p.waitForFunction(() => dinahResponsiveness.snapshot().some(s => s.kind === 'render' && s.duration >= 60));
    await p.waitForFunction(() => document.querySelector('#responsiveness-results tbody').textContent.includes('fixture work'));
    ok('a slow scheduled task appears in the diagnostic without message content', !(await p.locator('#responsiveness-results').textContent()).includes('BEGIN'));
    await toggle.click();
    ok('turning recording off clears the retained samples', await p.evaluate(() => dinahResponsiveness.snapshot().length === 0));
    await p.evaluate(() => dinahResponsiveness.record('render', 'disabled', 100, 100));
    ok('disabled diagnostics retain no new measurements', await p.evaluate(() => dinahResponsiveness.snapshot().length === 0));
  } finally { await t.close(); }
})();
