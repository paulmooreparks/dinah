const fs = require('fs');
const path = require('path');
const { launch } = require('./lib/page');
(async () => {
  const results = [];
  for (const count of [10, 150, 500]) {
    const t = await launch({ goto: false, viewport: { width: 1440, height: 900 }, args: { count }, app: ({ count }) => {
      const cwd = 'C:/Fixture';
      window.__catalog = Array.from({ length: count }, (_, i) => ({ sessionId: 'catalog-' + i, title: 'Saved chat ' + i, cwd, provider: 'codex', open: false, updatedAt: '2026-11-04T01:00:00Z', chatState: { life: 'closed', window: false } }));
      return { State: async () => ({ workspace: cwd, recent: [cwd], workspaceId: 'w1', workspaceRecords: [{ id: 'w1', name: 'Fixture', executionPath: cwd, sources: [] }] }),
        Sessions: async () => window.__catalog, SessionsAt: async () => window.__catalog, ChatStates: async () => ({ chats: [], groups: [] }),
        AgentProviders: async () => [__provider('codex'), __provider('claude')] };
    } });
    try {
      const p = t.page;
      if (process.env.CHAT_LIST_BASELINE_SOURCE) await p.route('http://dinah.test/chat-browser.js*', route => route.fulfill({ path: process.env.CHAT_LIST_BASELINE_SOURCE }));
      await p.goto('http://dinah.test/index.html');
      await t.waitReady();
      await p.evaluate(() => dinahWorkbench.open('chat-browser'));
      await p.waitForFunction(count => document.querySelectorAll('#sessions [data-session-row]').length === count, count);
      await p.waitForTimeout(200);
      const result = await p.evaluate(async () => {
        const root = document.getElementById('sessions'), samples = [];
        const enqueue = dinahRender.enqueue;
        let drawMs = 0;
        dinahRender.enqueue = function (key, element, draw, after, label) {
          return enqueue(key, element, function (...args) { const start = performance.now(); const result = draw(...args); if (element === root) drawMs += performance.now() - start; return result; }, after, label);
        };
        const observer = new MutationObserver(() => {});
        observer.observe(root, { subtree: true, childList: true, attributes: true, characterData: true });
        const settle = () => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
        for (let i = 0; i < 8; i++) {
          drawMs = 0; observer.takeRecords();
          let mutations = 0;
          observer.disconnect();
          const observed = new MutationObserver(records => { mutations += records.length; });
          observed.observe(root, { subtree: true, childList: true, attributes: true, characterData: true });
          document.dispatchEvent(new Event('dinah:team-state'));
          await settle();
          mutations += observed.takeRecords().length; observed.disconnect();
          samples.push({ drawMs, mutations });
        }
        dinahRender.enqueue = enqueue; observer.disconnect();
        return { samples, rows: root.querySelectorAll('[data-session-row]').length };
      });
      results.push({ count, ...result });
      t.ok('all ' + count + ' chats remain available', result.rows === count);
      if (!process.env.CHAT_LIST_BASELINE) t.ok('unchanged catalog redraw avoids per-row mutations', result.samples.every(s => s.mutations < 20), result.samples);
      console.log('MEASURE ' + JSON.stringify(results[results.length - 1]));
      if (!process.env.CHAT_LIST_BASELINE) {
        await p.evaluate(() => { window.__unchangedBadge = document.querySelector('#sessions [data-session-row="catalog-0"] .session-badge'); });
        await p.evaluate(count => {
          const info = window.__catalog[count - 1];
          info.title = 'Renamed saved chat'; info.provider = 'claude';
          info.open = true; info.chatState = { life: 'open', window: false, exec: { state: 'working', why: 'Checking the catalog' } };
          __emit('agent:session', info);
        }, count);
        const changed = p.locator('#sessions [data-session-row="catalog-' + (count - 1) + '"]');
        await changed.locator('.session-title', { hasText: 'Renamed saved chat' }).waitFor();
        const repaint = { meta: await changed.locator('.md-meta').innerText(), reason: await changed.locator('.session-reason').innerText(), working: await changed.locator('[data-state="working"]:not(.chat-state-reserve)').count() };
        t.ok('changed chat title, provider, state and reason repaint', repaint.meta.includes('Claude') && repaint.reason === 'Checking the catalog' && repaint.working === 1, JSON.stringify(repaint));
        t.ok('an unrelated chat retains its badge node', await p.evaluate(() => window.__unchangedBadge === document.querySelector('#sessions [data-session-row="catalog-0"] .session-badge')));
        await p.locator('#chat-filter').fill('Renamed saved chat');
        await p.waitForFunction(() => document.querySelectorAll('#sessions [data-session-row]').length === 1);
        await p.locator('#chat-filter').fill('');
        await p.waitForFunction(count => document.querySelectorAll('#sessions [data-session-row]').length === count, count);
        t.ok('clearing the filter restores every chat', await p.locator('#sessions [data-session-row]').count() === count);
      }
    } finally { await t.close(); }
  }
  const out = process.env.E2E_OUT || process.argv[3];
  if (out) fs.writeFileSync(path.join(out, 'chat-list-scaling.json'), JSON.stringify(results, null, 2));
})().catch(e => { console.error(e); process.exitCode = 1; });
