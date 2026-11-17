// A chat that finishes opening while its window is covered by another, and
// so is not drawn (covered-windows.js), is fully opened when it is raised:
// its loading mark is gone, its history is shown, its team membership is
// known and its message box is there. A window that stayed at Loading on
// 17 November 2026 had been covered at the start, and this suite was written
// to rule the covering out; the cause was another (chat-restoring-mark.js).
//
// It is checked for a chat opened by hand and for one the engine has live
// with a journal checkpoint, which is how a chat comes back after a restart,
// and for a chat covered before its agent answers and after.
const { launch } = require('./lib/page');
(async () => {
  for (const how of ['plain', 'checkpoint']) for (const when of ['opens while covered', 'is covered at once']) {
    const name = how + ', ' + when;
    const t = await launch({
      viewport: { width: 1600, height: 1000 },
      args: { how },
      app: ({ how }) => {
        const checkpoint = how === 'checkpoint';
        const info = (id, ws) => Object.assign({ sessionId: id, title: id, provider: 'codex', cwd: ws, open: true, working: false, queued: [], chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } },
          checkpoint && id === 'codex:slow' ? { streamGen: 'g1', streamHead: 3 } : {});
        const entry = (n, text, role) => ({ n, name: 'agent:update', data: { update: { sessionUpdate: role || 'agent_message_chunk', content: { type: 'text', text } } } });
        const journal = [entry(1, 'Journal question', 'user_message_chunk'), entry(2, 'Journal answer'), entry(3, ' and what followed the checkpoint: Row 39 of codex:slow')];
        const saved = Array.from({ length: 40 }, (_, i) => ({ role: i % 2 ? 'agent' : 'you', text: 'Saved row ' + i + '. ' + 'Words of an answer.\n\n'.repeat(3) }));
        window.__release = {}; window.__returned = false;
        return {
          ResolveSessionContinuation: async id => id,
          SavedConversation: async id => {
            if (id !== 'codex:slow') return { available: false };
            // Once the agent has answered, the copy says which journal entry it holds.
            return Object.assign({ available: true, complete: true, messages: saved.concat(checkpoint && window.__returned ? [{ role: 'you', text: 'Journal question' }, { role: 'agent', text: 'Journal answer' }] : []) },
              checkpoint && window.__returned ? { streamGen: 'g1', streamThrough: 2 } : {});
          },
          ChatEvents: async (id, gen, after) => ({ gen, head: journal.length, events: journal.filter(item => item.n > after) }),
          OpenSessionAt: async (ws, id) => {
            // The slow chat's agent answers only when the suite lets it, as a provider that takes its time does.
            if (id === 'codex:slow') { await new Promise(resolve => { window.__release[id] = resolve; }); window.__returned = true; }
            if (!(checkpoint && id === 'codex:slow')) for (let i = 0; i < 40; i++) window.__emit('agent:update', { sessionId: id, update: { sessionUpdate: i % 2 ? 'agent_message_chunk' : 'user_message_chunk', content: { type: 'text', text: 'Row ' + i + ' of ' + id + '. ' + 'Words of an answer.\n\n'.repeat(3) } } });
            return info(id, ws);
          },
          // The slow chat leads a team and reports to nobody.
          ChatLead: async () => null,
          Team: async id => id === 'codex:slow' ? { lead: { chat: id }, group: 'slow-team', members: [{ bus: 'bus-a', chat: 'codex:member', title: 'Member', life: 'open', exec: { state: 'idle' } }], counts: { top: 'idle', idle: 1 } } : { lead: { chat: id }, members: [], counts: { top: 'idle', idle: 1 } },
        };
      },
    });
    const { page: p, ok } = t;
    try {
      const frames = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
      const key = id => p.evaluate(id => dinahWorkbench.key({ kind: 'chat-session', workspace: 'C:/Fixture', path: id }), id);
      const place = async places => {
        await p.evaluate(places => {
          const query = new URLSearchParams(location.search);
          for (const [k, v] of Object.entries(places)) query.set('p.' + k, 'floating:' + v);
          history.replaceState(history.state, '', location.pathname + '?' + query.toString() + location.hash);
          window.dispatchEvent(new PopStateEvent('popstate'));
        }, places);
        await frames();
      };
      const slow = await key('codex:slow'), over = await key('codex:over');
      p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:slow' })).catch(() => {});
      await p.locator('[data-win="' + slow + '"]').waitFor();
      if (when === 'opens while covered') await p.waitForFunction(() => typeof window.__release['codex:slow'] === 'function');
      await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:over' }));
      await p.waitForFunction(k => document.querySelectorAll('[data-win="' + k + '"] .msg-text').length >= 40, over);
      await place({ [slow]: '0.3,0.1,0.4,0.6', [over]: '0.28,0.07,0.44,0.66' });
      ok(name + ': the slow chat is covered while it opens', await p.evaluate(k => document.querySelector('[data-win="' + k + '"]').hasAttribute('data-dinah-covered'), slow));
      // Its agent answers now, with the window still covered.
      await p.waitForFunction(() => typeof window.__release['codex:slow'] === 'function');
      await p.evaluate(() => window.__release['codex:slow']());
      await p.waitForTimeout(1500);
      ok(name + ': it is still covered when its agent has answered', await p.evaluate(k => document.querySelector('[data-win="' + k + '"]').hasAttribute('data-dinah-covered'), slow));
      await p.evaluate(k => pudlWindows.raise(k), slow);
      const win = p.locator('[data-win="' + slow + '"]');
      const state = () => win.evaluate(w => JSON.stringify({ loading: !(w.querySelector('.chat-loading-chip') || { hidden: true }).hidden, composer: !!(w.querySelector('.composer-row') && w.querySelector('.composer-row').getClientRects().length), pending: !!w.querySelector('[data-team-pending]') || w.matches('[data-team-pending]'), note: (w.querySelector('.team-member-note') || {}).textContent || '', rows: w.querySelectorAll('.msg-text').length, render: dinahRender.diagnostics() }));
      const opened = await p.waitForFunction(k => {
        const w = document.querySelector('[data-win="' + k + '"]'), chip = w.querySelector('.chat-loading-chip'), row = w.querySelector('.composer-row');
        return (!chip || chip.hidden) && row && row.getClientRects().length > 0 && !w.querySelector('[data-team-pending]') && /Row 39 of codex:slow/.test(w.querySelector('[data-view-id="logs"]').textContent);
      }, slow, { timeout: 8000 }).then(() => true, () => false);
      ok(name + ': raised, it has finished opening, shows its history, knows its team and has its message box', opened, await state());
      ok(name + ': and it is at the end of its history', await win.evaluate(w => { const log = w.querySelector('[data-view-id="activity"]'); return log.scrollHeight - log.scrollTop - log.clientHeight < 24; }));
    } finally { await t.close(); }
  }
})();
