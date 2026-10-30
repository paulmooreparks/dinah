const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1200, height: 850 },
    app: () => ({
      AgentProviders: async () => [{ id: 'codex', installed: true, available: true }],
      OpenSessionAt: async (cwd, sessionId) => ({ sessionId, cwd, provider: 'codex', open: true, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } })
    }),
    ready: 'chat',
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    // An external source file lets the regression be checked against a
    // previous revision without replacing files in the working tree.
    if (process.env.DINAH_TAIL_APP_SCRIPT) await t.context.route('http://dinah.test/app.js', route => route.fulfill({ path: process.env.DINAH_TAIL_APP_SCRIPT }));
    await p.goto(PAGE);
    await t.waitReady();
    await p.evaluate(async () => {
      await dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:tail' });
      pudlWindows.snap(dinahWorkbench.key({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:tail' }), 'maximized');
      window.__more = () => __emit('agent:update', { sessionId: 'codex:tail', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'A paragraph of streamed output.\n\n'.repeat(60) } } });
    });
    await p.waitForFunction(() => /Idle/.test(document.querySelector('.win[data-chat-current] .win-execution .win-state').title));
    await p.evaluate(() => __more());
    const body = p.locator('.win[data-chat-current] [data-view-id=activity]');
    await p.waitForFunction(() => { const el = document.querySelector('.win[data-chat-current] [data-view-id=activity]'); return el && el.scrollHeight > el.clientHeight + 500; });
    const settle = () => p.evaluate(() => new Promise(resolve => { let n = 12; function frame() { if (--n) requestAnimationFrame(frame); else resolve(); } requestAnimationFrame(frame); }));
    const atEnd = () => body.evaluate(el => el.scrollHeight - el.clientHeight - el.scrollTop < 3);
    await settle(); ok('new output tails initially', await atEnd());
    await p.locator('.win[data-chat-current] [data-view-id=composer]').evaluate(el => el.style.minHeight = '250px');
    await settle(); ok('growing the composer keeps the latest output visible', await atEnd());
    await p.locator('.win[data-chat-current] [data-view-id=composer]').evaluate(el => el.style.minHeight = '');
    await settle();
    await p.evaluate(() => __emit('agent:permission', { sessionId: 'codex:tail', key: 'tail-question', toolCall: { title: 'A long pending question. '.repeat(80) }, options: [{ optionId: 'yes', name: 'Allow Once', kind: 'allow_once' }] }));
    await settle(); ok('a question opening keeps the output at the end', await atEnd());
    await p.evaluate(() => __emit('agent:answered', { sessionId: 'codex:tail', key: 'tail-question' }));
    await settle();
    await body.hover(); await p.mouse.wheel(0, -500); await settle();
    const before = await body.evaluate(el => el.scrollTop);
    await p.evaluate(() => __more()); await settle();
    ok('scrolling upward suspends following while output continues', !await atEnd() && Math.abs(await body.evaluate(el => el.scrollTop) - before) < 3);
    await p.locator('.win[data-chat-current] [data-view-id=jump-end]').click(); await settle();
    await p.evaluate(() => __more()); await settle();
    ok('Jump to the Latest resumes following subsequent output', await atEnd());
    await p.evaluate(() => { const log = document.querySelector('.win[data-chat-current] [data-view-id=logs]'); const late = document.createElement('div'); late.id = 'late-content'; late.style.height = '2000px'; log.appendChild(late); });
    await settle(); ok('late content growth tails without a provider update', await atEnd());
    await p.locator('#late-content').evaluate(el => el.style.height = '10px'); await settle();
    await p.evaluate(() => __more()); await settle();
    ok('content shrinking does not disable following', await atEnd());

    // The reader scrolls away from the end in the frame of a layout change, with the wheel as a reader
    // does: the chat must not take them back to the end. (The list's own scroll events that come with
    // growth are not read as the reader, since the browser moves the view by itself as content changes.)
    await p.evaluate(() => {
      const log = document.querySelector('.win[data-chat-current] [data-view-id=logs]'), grow = document.createElement('div');
      grow.id = 'grown-content'; grow.style.height = '900px'; log.appendChild(grow);
    });
    await body.hover(); await p.mouse.wheel(0, -20000);
    await settle();
    const left = await body.evaluate(el => el.scrollTop);
    ok('a scroll away from the end in the frame of a layout change is left where the reader put it', !await atEnd() && left < 400, String(left));
    ok('and following is off, so Jump to the Latest shows', await p.locator('.win[data-chat-current] [data-view-id=jump-end]').isVisible());
    await p.evaluate(() => __more()); await settle();
    ok('later output does not take the reader back to the end', !await atEnd() && Math.abs(await body.evaluate(el => el.scrollTop) - left) < 3);
  } finally { await t.close(); }
})();
