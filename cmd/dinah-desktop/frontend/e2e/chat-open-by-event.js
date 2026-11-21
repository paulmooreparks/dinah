// The page opens a saved chat with a call that returns at once, and takes the outcome from an event
// (docs/design/chat-open-rules.md). Six chats whose opens never finish hold no call open, so the page
// still gets answers. A chat whose open ends at the engine's load limit shows that in its window and
// can be tried again, and a chat whose outcome the page missed learns it after a resync.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 950 },
    app: () => {
      // The engine as the page sees it: BeginChatOpen records the open and answers at once. The suite
      // ends an open with __finish, which sends the event, or with __end, which only records the
      // outcome, as when the page's stream lost the event.
      window.__begun = []; window.__direct = 0;
      const known = new Map();
      window.__end = (token, outcome) => known.set(token, Object.assign({ open: token }, outcome));
      window.__finish = (token, outcome) => { window.__end(token, outcome); window.__emit('chat:open-result', known.get(token)); };
      const never = () => { window.__direct++; return new Promise(() => {}); };
      return {
        ResolveSessionContinuation: async id => id, SessionContinuation: async () => null,
        SavedConversation: async () => ({ available: false, messages: [] }),
        OpenSessionAt: never, OpenWaitingSession: never, LoadFullHistory: never,
        BeginChatOpen: async (token, method, args) => {
          if (known.has(token)) return known.get(token) ? { state: 'done', outcome: known.get(token) } : { state: 'opening' };
          known.set(token, null);
          window.__begun.push({ token, method, args });
          return { state: 'opening' };
        },
      };
    },
    // Every call to the engine is counted while it has no answer.
    init: () => {
      window.__unanswered = 0;
      const App = window.go.main.App;
      for (const name of Object.keys(App)) {
        const fn = App[name];
        App[name] = (...args) => {
          const answer = Promise.resolve().then(() => fn(...args));
          window.__unanswered++;
          answer.then(() => { window.__unanswered--; }, () => { window.__unanswered--; });
          return answer;
        };
      }
    },
  });
  const { page: p, ok } = t;
  try {
    const ids = [1, 2, 3, 4, 5, 6].map(n => 'codex:stuck-' + n);
    const keys = [];
    for (const id of ids) keys.push(await p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: id }), id));
    const win = n => p.locator('[data-win="' + keys[n] + '"]');
    const loading = n => win(n).locator('.chat-loading-chip').evaluate(el => !el.hidden);
    const begun = id => p.evaluate(id => window.__begun.filter(open => open.args[1] === id), id);
    const info = id => ({ sessionId: id, provider: 'codex', cwd: 'C:/Fixture', title: id, open: true, working: false, queued: [] });

    await p.waitForFunction(() => window.__begun.length === 6);
    await p.waitForFunction(keys => keys.every(key => { const chip = document.querySelector('[data-win="' + key + '"] .chat-loading-chip'); return chip && !chip.hidden; }), keys);
    ok('six chats are opening', (await p.evaluate(() => window.__begun.map(open => open.method).join())) === Array(6).fill('OpenSessionAt').join());
    ok('no opening method was called and left waiting', await p.evaluate(() => window.__direct) === 0);
    await p.waitForFunction(() => window.__unanswered === 0);
    ok('no call to the engine is unanswered while six opens never finish', await p.evaluate(() => window.__unanswered) === 0);
    ok('another call is answered meanwhile', await t.deadline('a call beside the opens', p.evaluate(() => window.go.main.App.State().then(state => !!state))));

    // The first chat's open ends at the load limit.
    await p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: id }), ids[0]);
    const box = () => win(0).evaluate(el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height].join(); });
    const before = await box();
    const first = (await begun(ids[0]))[0];
    await p.evaluate(token => window.__finish(token, { method: 'OpenSessionAt', error: 'The agent did not answer the request to load this chat (session/load) within 90 seconds, so the chat stays closed. Its saved history and any messages waiting for it are kept.', detail: { kind: 'open-timeout', method: 'session/load', seconds: 90 } }), first.token);
    const note = win(0).locator('[data-view-id=session-note]');
    await note.waitFor();
    await p.waitForFunction(key => document.querySelector('[data-win="' + key + '"] .chat-loading-chip').hidden, keys[0]);
    const words = await note.textContent();
    ok('the window says the open did not finish, in the engine\'s words', words.includes('did not finish opening this session') && words.includes('within 90 seconds') && !words.includes('reported:'), words);
    ok('the window offers to try again', await win(0).getByRole('button', { name: 'Try Again', exact: true }).isVisible());
    ok('the window kept its place and size', await box() === before, before + ' then ' + await box());
    ok('the other chats are still opening', (await Promise.all([1, 2, 3, 4, 5].map(loading))).every(Boolean));

    // Try Again starts a new open, and its outcome opens the chat.
    await win(0).getByRole('button', { name: 'Try Again', exact: true }).click();
    await p.waitForFunction(id => window.__begun.filter(open => open.args[1] === id).length === 2, ids[0]);
    const second = (await begun(ids[0]))[1];
    ok('the retry is a new open of the same chat', second.token !== first.token && second.method === 'OpenSessionAt');
    await p.waitForFunction(key => !document.querySelector('[data-win="' + key + '"] .chat-loading-chip').hidden, keys[0]);
    ok('the failure is gone while the retry runs', await note.isHidden());
    await p.evaluate(([token, info]) => window.__finish(token, { method: 'OpenSessionAt', info }), [second.token, info(ids[0])]);
    await p.waitForFunction(key => document.querySelector('[data-win="' + key + '"] .chat-loading-chip').hidden, keys[0]);
    ok('the retried chat is open and shows no failure', await note.isHidden());
    ok('the window kept its place and size through the retry', await box() === before, before + ' then ' + await box());

    // The second chat's outcome is lost to the page. The resync asks again and is told it.
    const lost = (await begun(ids[1]))[0];
    await p.evaluate(([token, info]) => window.__end(token, { method: 'OpenSessionAt', info }), [lost.token, info(ids[1])]);
    ok('a chat whose outcome was lost is still shown as opening', await loading(1));
    const asked = await p.evaluate(() => window.__bridgeCalls.filter(call => call[0] === 'BeginChatOpen').length);
    await p.evaluate(() => window.__resync());
    await p.waitForFunction(key => document.querySelector('[data-win="' + key + '"] .chat-loading-chip').hidden, keys[1]);
    ok('the resync gave the chat its outcome', !(await loading(1)) && await win(1).locator('[data-view-id=session-note]').isHidden());
    ok('the resync asked for the five opens still waited for, each with its own token', await p.evaluate(asked => window.__bridgeCalls.filter(call => call[0] === 'BeginChatOpen').length - asked, asked) === 5);
    ok('the resync started no open again', await p.evaluate(() => window.__begun.length) === 7);
    ok('the chats that are still opening stay so', (await Promise.all([2, 3, 4, 5].map(loading))).every(Boolean));
    await p.waitForFunction(() => window.__unanswered === 0);
    ok('no opening method was ever called and left waiting', await p.evaluate(() => window.__direct) === 0);
    await t.shot('chat-open-by-event');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
