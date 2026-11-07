const { launch, PAGE } = require('./lib/page');
const fs = require('fs');
const path = require('path');
(async () => {
  const t = await launch({
    app: () => ({ OpenSessionAt: async (cwd, sessionId) => ({ sessionId, cwd, provider: 'codex', open: true, working: true }) }),
    ready: 'chat', goto: false,
  });
  try {
    if (process.env.DINAH_TOOL_BASELINE) await t.context.route('**/chat-applet.js', route => route.fulfill({path: process.env.DINAH_TOOL_BASELINE}));
    await t.page.goto(PAGE); await t.waitReady();
    const cdp = await t.context.newCDPSession(t.page);
    await t.page.evaluate(async () => {
      await dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:bounded-tools' });
    });
    const samples = [];
    for (let batch = 0; batch < Number(process.env.DINAH_TOOL_BATCHES || 4); batch++) {
      await t.page.evaluate(batch => {
        const updates = [];
        for (let i = 0; i < 250; i++) {
          updates.push({ sessionUpdate: 'tool_call', toolCallId: 'tool-' + (batch * 250 + i), title: 'Measured tool', kind: 'execute', status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'output '.repeat(150) } }] });
        }
        __emit('agent:updates', { sessionId: 'codex:bounded-tools', updates });
      }, batch);
      await t.page.waitForTimeout(500);
      samples.push(await t.page.evaluate(() => {
        const log = document.querySelector('.win[data-chat-current]');
        return { tools: log.querySelectorAll('.tool-row').length, nodes: log.querySelectorAll('*').length, groups: log.querySelectorAll('.tool-group').length, trimmed: log.querySelector('.trimmed-words')?.textContent || '' };
      }));
    }
    if (process.env.DINAH_TOOL_MEASURE === '1') {
      await t.page.evaluate(() => {
        window.__toolKeys = [];
        const box = document.querySelector('.win[data-chat-current] [data-view-id=prompt]');
        box.addEventListener('keydown', e => {
          const queued = performance.now() - e.timeStamp;
          requestAnimationFrame(() => __toolKeys.push({queued, frame: performance.now() - e.timeStamp}));
        });
      });
      const box = t.page.locator('.win[data-chat-current] [data-view-id=prompt]');
      await box.focus();
      for (let i = 0; i < 60; i++) {
        await box.press('a');
        await t.page.evaluate(() => new Promise(resolve => requestAnimationFrame(resolve)));
      }
      fs.writeFileSync(path.join(process.argv[3], 'tool-input-timings.json'), JSON.stringify(await t.page.evaluate(() => __toolKeys), null, 2));
    }
    await cdp.send('HeapProfiler.collectGarbage');
    const heap = await cdp.send('Runtime.getHeapUsage');
    fs.writeFileSync(path.join(process.argv[3], 'tool-heap.json'), JSON.stringify(heap, null, 2));
    fs.writeFileSync(path.join(process.argv[3], 'tool-retention.json'), JSON.stringify(samples, null, 2));
    t.ok('a continuous tool run stays within the transcript row bound', samples.at(-1).tools <= 600, JSON.stringify(samples));
    t.ok('the latest tool is retained', await t.page.evaluate(last => Array.from(document.querySelectorAll('.tool-row')).some(row => row._tool && row._tool === row._tool.s.rows['tool-' + last]), Number(process.env.DINAH_TOOL_BATCHES || 4) * 250 - 1));
  } finally { await t.close(); }
})();
