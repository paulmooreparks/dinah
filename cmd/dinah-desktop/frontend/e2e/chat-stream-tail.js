// A new chat whose turn streams text, thinking, tool rows that change as they run and a plan keeps
// following its end while the reader has not scrolled: only the reader's own scrolling stops that.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1200, height: 850 },
    app: () => ({
      AgentProviders: async () => [{ id: 'ollama', installed: true, available: true }],
      OpenSessionAt: async (cwd, sessionId) => ({ sessionId, cwd, provider: 'ollama', open: true, life: 'open', window: true, exec: { state: 'idle', why: '' } })
    }),
    ready: 'chat',
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(async () => {
      await dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'ollama:stream' });
      window.__u = update => __emit('agent:update', { sessionId: 'ollama:stream', update });
    });
    await p.waitForFunction(() => !!document.querySelector('.win[data-chat-current] .win-execution .win-state[data-state="idle"]'));
    const body = p.locator('.win[data-chat-current] [data-view-id=activity]');
    const gap = () => body.evaluate(el => el.scrollHeight - el.clientHeight - el.scrollTop);
    const frames = n => p.evaluate(n => new Promise(resolve => { function frame() { if (--n) requestAnimationFrame(frame); else resolve(); } requestAnimationFrame(frame); }), n);
    const gaps = [];
    const step = async fn => { await p.evaluate(fn); await frames(3); gaps.push(await gap()); };
    for (let i = 0; i < 14; i++) {
      await p.evaluate(i => { window.__i = i; }, i);
      await step(() => __u({ sessionUpdate: 'agent_thought_chunk', content: { type: 'text', text: 'Thinking about step ' + window.__i + '. ' + 'Considering options. '.repeat(30) } }));
    }
    for (let i = 0; i < 12; i++) {
      await p.evaluate(i => { window.__i = i; }, i);
      await step(() => __u({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Streamed text for step ' + window.__i + '.\n\n' + 'More words. '.repeat(40) + '\n\n' } }));
      await step(() => __u({ sessionUpdate: 'tool_call', toolCallId: 'tc' + window.__i, title: 'Run command ' + window.__i, status: 'in_progress', rawInput: { command: 'echo ' + window.__i } }));
      await step(() => __u({ sessionUpdate: 'tool_call_update', toolCallId: 'tc' + window.__i, status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'Output line\n'.repeat(15) } }] }));
      await step(() => __u({ sessionUpdate: 'plan', entries: [1, 2, 3, 4].map(n => ({ content: 'Plan item ' + n + ' of step ' + window.__i, status: n < 3 ? 'completed' : 'pending', priority: 'medium' })) }));
    }
    // Tool calls that arrive together are grouped, and finishing the early ones changes rows above the end.
    for (let g = 0; g < 4; g++) {
      for (let k = 0; k < 8; k++) {
        await p.evaluate(([g, k]) => { window.__g = g; window.__k = k; }, [g, k]);
        await step(() => __u({ sessionUpdate: 'tool_call', toolCallId: 'g' + window.__g + 'k' + window.__k, title: 'Read file ' + window.__k, status: 'in_progress', rawInput: { path: 'src/file' + window.__k + '.go' } }));
      }
      for (let k = 0; k < 8; k++) {
        await p.evaluate(([g, k]) => { window.__g = g; window.__k = k; }, [g, k]);
        await step(() => __u({ sessionUpdate: 'tool_call_update', toolCallId: 'g' + window.__g + 'k' + window.__k, status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'package main\n'.repeat(12) } }] }));
      }
      await step(() => __u({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Read the files in group ' + window.__g + '.\n\n' + 'Then some words. '.repeat(30) + '\n\n' } }));
    }
    await frames(12);
    const worst = Math.max(...gaps.slice(-120));
    ok('after the turn streams, the view is at the end', (await gap()) < 3, String(await gap()));
    ok('and it stayed at the end after every step, with nobody scrolling', gaps.slice(-120).every(g => g < 3), 'worst gap ' + worst + ' of ' + gaps.slice(-120).join(','));
    ok('Jump to the Latest is not offered', await p.locator('.win[data-chat-current] [data-view-id=jump-end]').isHidden());

    // A reader who scrolls up, with the wheel, is left where they are as more arrives.
    await body.hover(); await p.mouse.wheel(0, -1500); await frames(6);
    const place = await body.evaluate(el => el.scrollTop);
    await step(() => __u({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Late text.\n\n'.repeat(30) } }));
    await frames(6);
    ok('a reader who scrolled up is left where they are, and Jump to the Latest shows', Math.abs((await body.evaluate(el => el.scrollTop)) - place) < 40 && await p.locator('.win[data-chat-current] [data-view-id=jump-end]').isVisible());
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
