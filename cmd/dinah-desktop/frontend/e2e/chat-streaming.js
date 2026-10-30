// Command-output bursts must not force a layout for each delta. Codex sends
// small output deltas, but every provider uses the same chat renderer.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => ({
      ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
      OpenSessionAt: async (ws, id) => {
        if (id === 'codex:replay') {
          window.__emit('agent:updates', { sessionId: id, updates: [
            { sessionUpdate: 'tool_call', toolCallId: 'replayed', title: 'Replayed command', status: 'in_progress' },
            { sessionUpdate: 'tool_call_update', toolCallId: 'replayed', content: [{ type: 'content', content: { type: 'text', text: 'Saved command output' } }] },
            { sessionUpdate: 'tool_call_update', toolCallId: 'replayed', status: 'completed' }
          ] });
        }
        return { sessionId: id, title: id, provider: 'codex', cwd: ws, open: true, working: false, queued: [] };
      }
    }),
    ready: () => window.dinahWorkbench && window.dinahEditor,
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    // An external source file lets the regression be checked against a
    // previous revision without replacing files in the working tree.
    if (process.env.DINAH_STREAM_APP_SCRIPT) await t.context.route('http://dinah.test/app.js', r => r.fulfill({ path: process.env.DINAH_STREAM_APP_SCRIPT }));
    await p.goto(PAGE);
    await t.waitReady();
    const first = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:stream' }));
    const win = p.locator('[data-win="' + first + '"]');
    await win.locator('[data-view-id="prompt"]').waitFor();
    await p.evaluate(() => {
      for (const id of ['before', 'stream']) __emit('agent:update', { sessionId: 'codex:stream', update: {
        sessionUpdate: 'tool_call', toolCallId: id, title: id === 'stream' ? 'Run tests' : 'Build', kind: 'execute', status: 'in_progress'
      } });
    });
    const cd = await t.context.newCDPSession(p);
    await cd.send('Performance.enable');
    const layouts = async () => (await cd.send('Performance.getMetrics')).metrics.find(m => m.name === 'LayoutCount').value;
    const before = await layouts();
    await p.evaluate(() => {
      for (let i = 0; i < 5000; i++) __emit('agent:update', { sessionId: 'codex:stream', update: {
        sessionUpdate: 'tool_call_update', toolCallId: 'stream', content: [{ type: 'content', content: { type: 'text', text: '\x1b[32mPASS\x1b[0m line ' + i + ' <tags>\n' + 'x'.repeat(70) } }]
      } });
      __emit('agent:update', { sessionId: 'codex:stream', update: { sessionUpdate: 'tool_call_update', toolCallId: 'stream', status: 'completed' } });
      __emit('agent:update', { sessionId: 'codex:stream', update: { sessionUpdate: 'tool_call_update', toolCallId: 'before', status: 'completed' } });
    });
    await p.waitForFunction(() => document.querySelector('.tool-group-words')?.textContent === 'Ran 2 commands' && [...document.querySelectorAll('.tool-row pre')].some(el => el.textContent.includes('The rest of this output')));
    const used = await layouts() - before;
    console.log('MEASURE burst layouts ' + used);
    // This budget leaves room for normal frame/observer work, but catches
    // one synchronous layout per output delta without timing assumptions.
    ok('a burst of 5000 tool updates requires fewer than 40 layouts', used < 40, String(used));
    const rendered = await win.locator('.tool-row pre').allTextContents();
    const output = rendered.find(text => text.includes('line 0')) || '';
    ok('batched output keeps its order, text escaping and display limit', output.includes('line 0 <tags>') && output.includes('line 1 <tags>') && output.indexOf('line 0') < output.indexOf('line 1') && output.length < 40 * 1024 && output.includes('The rest of this output'));
    ok('terminal colors survive batching', await win.locator('.tool-row pre .ansi-fg2').count() > 0 && await win.locator('.tool-row pre tags').count() === 0);
    await p.waitForFunction(k => [...document.querySelectorAll('[data-win="' + CSS.escape(k) + '"] .tool-row .badge')].every(el => el.textContent === 'done'), first);
    ok('the final tool statuses and group summary are current', (await win.locator('.tool-row .badge').allTextContents()).every(text => text === 'done') && (await win.locator('.tool-group-now').textContent()) === '');
    await win.locator('[data-view-id="prompt"]').fill('The composer still accepts input.');
    ok('the composer accepts input after the burst', await win.locator('[data-view-id="prompt"]').inputValue() === 'The composer still accepts input.');

    const second = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'ollama:background' }));
    await p.locator('[data-win="' + second + '"] [data-view-id="prompt"]').waitFor();
    await p.evaluate(k => pudlWindows.raise(k), first);
    await p.evaluate(() => {
      __emit('agent:update', { sessionId: 'ollama:background', update: { sessionUpdate: 'tool_call', toolCallId: 'background', title: 'Background command', kind: 'execute', status: 'in_progress' } });
      for (let i = 0; i < 1000; i++) __emit('agent:update', { sessionId: 'ollama:background', update: { sessionUpdate: 'tool_call_update', toolCallId: 'background', content: [{ type: 'content', content: { type: 'text', text: 'Background line ' + i } }] } });
      __emit('agent:update', { sessionId: 'ollama:background', update: { sessionUpdate: 'tool_call_update', toolCallId: 'background', status: 'failed' } });
    });
    await p.waitForFunction(k => document.querySelector('[data-win="' + CSS.escape(k) + '"] pre')?.textContent.includes('Background line 999'), second);
    ok('background output stays in its own chat', !(await win.locator('[data-view-id="logs"]').textContent()).includes('Background line') && (await p.locator('[data-win="' + second + '"] .tool-row .badge').textContent()) === 'failed');
    ok('background rendering leaves the front chat and its draft alone', await win.locator('[data-view-id="prompt"]').inputValue() === 'The composer still accepts input.' && await p.evaluate(k => pudlWindows.state().top === k, first));
    const prompt = win.locator('[data-view-id="prompt"]');
    await prompt.fill('');
    await prompt.focus();
    await p.evaluate(() => {
      window.__concurrent = { ticks: 0, inputs: 0, active: true };
      document.activeElement.addEventListener('input', () => {
        if (__concurrent.active && __concurrent.ticks > 0) __concurrent.inputs++;
      });
      for (const sessionId of ['codex:stream', 'ollama:background']) __emit('agent:update', { sessionId, update: {
        sessionUpdate: 'tool_call', toolCallId: 'concurrent', title: 'Concurrent command', kind: 'execute', status: 'in_progress'
      } });
      window.__streamTimer = setInterval(() => {
        const tick = ++__concurrent.ticks;
        for (const sessionId of ['codex:stream', 'ollama:background']) {
          for (let i = 0; i < 100; i++) __emit('agent:update', { sessionId, update: {
            sessionUpdate: 'tool_call_update', toolCallId: 'concurrent', content: [{ type: 'content', content: { type: 'text', text: 'Concurrent ' + tick + ':' + i + '\n' } }]
          } });
        }
      }, 16);
    });
    await p.waitForFunction(() => __concurrent.ticks > 1);
    const draft = 'Typing while both chats run commands.';
    await prompt.pressSequentially(draft, { delay: 20 });
    const concurrent = await p.evaluate(() => {
      clearInterval(__streamTimer);
      __concurrent.active = false;
      for (const sessionId of ['codex:stream', 'ollama:background']) __emit('agent:update', { sessionId, update: {
        sessionUpdate: 'tool_call_update', toolCallId: 'concurrent', status: 'completed'
      } });
      return __concurrent;
    });
    ok('typing remains intact while two chats stream command output', await prompt.inputValue() === draft && concurrent.inputs === draft.length && concurrent.ticks > 2, JSON.stringify(concurrent));
    for (const key of [first, second]) await p.waitForFunction(k => {
      const rows = [...document.querySelectorAll('[data-win="' + CSS.escape(k) + '"] .tool-row')];
      return rows.some(row => row.textContent.includes('Concurrent command') && row.querySelector('.badge')?.textContent === 'done' && row.querySelector('pre')?.textContent.includes('Concurrent 1:0'));
    }, key);
    ok('both concurrent streams finish in their own windows without taking focus', await prompt.evaluate(el => el === document.activeElement) && await p.evaluate(k => pudlWindows.state().top === k, first));
    await p.evaluate(k => {
      pudlWindows.minimize(k);
      __emit('agent:update', { sessionId: 'ollama:background', update: { sessionUpdate: 'tool_call', toolCallId: 'hidden', title: 'Hidden command', kind: 'execute', status: 'in_progress' } });
      for (let i = 0; i < 100; i++) __emit('agent:update', { sessionId: 'ollama:background', update: {
        sessionUpdate: 'tool_call_update', toolCallId: 'hidden', content: [{ type: 'content', content: { type: 'text', text: 'Hidden line ' + i } }]
      } });
      __emit('agent:update', { sessionId: 'ollama:background', update: { sessionUpdate: 'tool_call_update', toolCallId: 'hidden', status: 'completed' } });
    }, second);
    await p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    const back = p.locator('[data-win="' + second + '"]');
    ok('a minimized chat defers streamed output rendering', !(await back.locator('pre').allTextContents()).join(' ').includes('Hidden line'));
    await p.evaluate(k => pudlWindows.raise(k), second);
    await back.locator('.tool-row').filter({ hasText: 'Hidden command' }).locator('pre').filter({ hasText: 'Hidden line 99' }).waitFor({ state: 'attached' });
    ok('revealing a chat draws its retained output and final status', await back.locator('.tool-row').filter({ hasText: 'Hidden command' }).locator('.badge').textContent() === 'done');
    const replay = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:replay' }));
    await p.waitForFunction(k => document.querySelector('[data-win="' + CSS.escape(k) + '"] pre')?.textContent.includes('Saved command output'), replay);
    ok('history replay flushes its saved tool output and final status', (await p.locator('[data-win="' + replay + '"] .tool-row .badge').textContent()) === 'done');
  } catch (e) {
    t.errors.push(String(e && e.stack || e));
  } finally {
    await t.close();
  }
})();
