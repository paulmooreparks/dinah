// A window opened on a chat that is live in the engine, in the middle of a turn, draws the
// saved copy and then the log's entries after the number the copy holds, so the turn so far
// is shown once and in order. A chat with no such number stays as it is, and a log that has
// lost the entries reports the loss. The engine's calls are stubbed.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => {
      window.__logs = [];
      window.__opened = [];
      const chat = (id, extra) => Object.assign({ sessionId: id, title: id, provider: 'codex', cwd: 'C:/Fixture', open: true, working: true, queued: [] }, extra);
      const list = [
        chat('codex:run', { streamGen: 'g1', streamHead: 12 }),
        chat('codex:cut', { streamGen: 'g2', streamHead: 40 }),
        chat('codex:plain')
      ];
      const entry = (n, text) => ({ n, name: 'agent:update', data: { update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } } } });
      return {
        Sessions: async () => list, SessionsAt: async () => list,
        ResolveSessionContinuation: async id => id,
        SessionContinuation: async () => ({ history: [] }),
        SavedConversation: async id => {
          if (id === 'codex:run') {
            // An update of the turn arrives while the saved copy is being read.
            await new Promise(r => setTimeout(r, 50));
            window.__emit('agent:update', { sessionId: id, gen: 'g1', n: 9, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: ' [live nine] ' } } });
            return { available: true, complete: true, streamGen: 'g1', streamThrough: 4, messages: [{ role: 'you', text: 'Start the long job.' }, { role: 'agent', text: 'SAVED-AGENT-TEXT' }] };
          }
          if (id === 'codex:cut') return { available: true, complete: true, streamGen: 'g2', streamThrough: 30, messages: [{ role: 'you', text: 'Cut chat.' }] };
          return { available: true, complete: true, messages: [{ role: 'you', text: 'Plain chat.' }, { role: 'agent', text: 'PLAIN-SAVED' }] };
        },
        ChatEvents: async (id, gen, after) => {
          window.__logs.push([id, gen, after]);
          if (id === 'codex:run') return { gen: 'g1', head: 8, truncated: false, reset: false, events: [entry(5, ' [five] '), entry(6, ' [six] '), entry(7, ' [seven] '), entry(8, ' [eight] ')] };
          return { gen, head: 40, truncated: true, reset: false, events: [entry(40, ' [forty] ')] };
        },
        OpenSessionAt: async (ws, id) => { window.__opened.push(id); return chat(id); }
      };
    },
    ready: () => window.dinahWorkbench && window.dinahEditor,
  });
  const { page, ok } = t;
  const winText = key => page.locator('[data-win="' + key + '"]').innerText();
  const count = (text, part) => text.split(part).length - 1;
  try {
    const run = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:run' }));
    await page.locator('[data-win="' + run + '"]').getByText('[eight]').waitFor({ state: 'attached' });
    await page.waitForTimeout(300);
    const text = await winText(run);
    ok('the saved copy and the turn so far are both shown', /Start the long job\./.test(text) && /SAVED-AGENT-TEXT/.test(text) && /\[five\]/.test(text) && /\[eight\]/.test(text));
    ok('the log is read from the number the copy holds', JSON.stringify(await page.evaluate(() => window.__logs.filter(l => l[0] === 'codex:run'))) === JSON.stringify([['codex:run', 'g1', 4]]));
    ok('each entry is shown once, in order', ['[five]', '[six]', '[seven]', '[eight]', '[live nine]'].every(p => count(text, p) === 1) &&
      text.indexOf('[five]') < text.indexOf('[six]') && text.indexOf('[eight]') < text.indexOf('[live nine]'), text);
    ok('an update that arrived while the copy was read follows the log\'s entries', text.indexOf('[eight]') < text.indexOf('[live nine]'));
    ok('a live chat is not opened again', !(await page.evaluate(() => window.__opened)).includes('codex:run'));
    ok('and no part of it is reported lost', (await page.evaluate(() => dinahHost.notices.list(e => e.id === 'lost-activity-codex:run').length)) === 0);

    // A later live update goes on from there.
    await page.evaluate(() => __emit('agent:update', { sessionId: 'codex:run', gen: 'g1', n: 10, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: ' [ten] ' } } }));
    await page.locator('[data-win="' + run + '"]').getByText('[ten]').waitFor({ state: 'attached' });
    ok('a later update is shown after them without another read', (await page.evaluate(() => window.__logs.filter(l => l[0] === 'codex:run').length)) === 1);

    // A log that no longer has the entries after the number reports the loss.
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:cut' }));
    await page.waitForFunction(() => dinahHost.notices.list(e => e.id === 'lost-activity-codex:cut').length === 1);
    ok('a log that lost entries after the number is reported as a loss', true);

    // A live chat whose saved copy has no number is left as it was.
    const plain = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:plain' }));
    await page.locator('[data-win="' + plain + '"] [data-view-id="prompt"]').waitFor();
    await page.waitForTimeout(400);
    ok('a live chat without a number is not drawn from the saved copy and reads no log', !/PLAIN-SAVED/.test(await winText(plain)) && (await page.evaluate(() => window.__logs.filter(l => l[0] === 'codex:plain').length)) === 0);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
