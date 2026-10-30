// A scrolled-back reader keeps their place while the chat changes (Paul's
// report, Dinah UI's diagnosis): a new row at the end, a reply streaming
// and the oldest rows let go must not move what is being read; and in a
// group the same message holds its place across a full redraw (chat-text.js,
// keepPlace).
const { launch } = require('./lib/page');

(async () => {
  // Establish a real reading position in the owning window before measuring.
  async function visibleTop(sel, text, leg) {
    await p.locator(sel).waitFor();
    await scrollTo(sel, text);
    const result = await p.waitForFunction(([sel, text]) => {
      const log = document.querySelector(sel);
      const row = [...log.querySelectorAll('[data-bus-message], .activity-row')].find(el => el.textContent.includes(text));
      if (!row) return false;
      const view = log.getBoundingClientRect(), spot = row.getBoundingClientRect();
      const top = Math.max(0, view.top), bottom = Math.min(innerHeight, view.bottom);
      return spot.height > 0 && spot.bottom > top && spot.top < bottom && view.height > 0
        ? { top: spot.top - view.top, height: view.height, rowTop: spot.top, rowBottom: spot.bottom } : false;
    }, [sel, text], { timeout: 15000 }).catch(() => false);
    ok('the reader target is inside its log before ' + leg, !!result);
    if (!result) throw new Error('The reading target was outside its log before ' + leg);
    return (await result.jsonValue()).top;
  }
  function rowTop(sel, text) {
    return p.evaluate(([sel, text]) => {
      const log = document.querySelector(sel);
      const row = [...log.querySelectorAll('[data-bus-message], .activity-row')].find(el => el.textContent.includes(text));
      return row ? row.getBoundingClientRect().top - log.getBoundingClientRect().top : null;
    }, [sel, text]);
  }
  // That row still at that spot after the change settles.
  async function holds(sel, text, before, what) {
    const after = await p.waitForFunction(([sel, text, want]) => {
      const log = document.querySelector(sel);
      const top = log.getBoundingClientRect().top;
      const row = [...log.querySelectorAll('[data-bus-message], .activity-row')].find(el => el.textContent.includes(text));
      if (!row) return false;
      /* An eight-chunk burst moves the anchored row under ten pixels once,
         under a text line. */
      const view = log.getBoundingClientRect(), spot = row.getBoundingClientRect();
      return spot.bottom > Math.max(0, view.top) && spot.top < Math.min(innerHeight, view.bottom) && Math.abs((spot.top - top) - want) < 12;
    }, [sel, text, before], { timeout: 15000 }).catch(() => false);
    const read = await rowTop(sel, text);
    /* Hidden rows' estimated heights refine as they draw near (content-
       visibility); that is sub-pixel slack the browser leaves even with the
       page holding the anchored row exactly. The check allows one line's
       worth, far under the message heights that were being lost. */
    ok('the place holds across ' + what, after !== false && read !== null && Math.abs(read - before) < 12, 'before ' + before + ' after ' + read);
  }
  async function scrollTo(log, text) {
    await p.locator(log).focus();
    await p.keyboard.press('Control+Home');
    await p.waitForFunction(sel => document.querySelector(sel).dataset.following === 'false', log);
    return p.evaluate(([sel, text]) => {
      const log = document.querySelector(sel);
      const top = log.getBoundingClientRect().top;
      const rows = [...log.querySelectorAll('[data-bus-message], .activity-row')];
      const row = rows.find(el => el.textContent.includes(text));
      if (!row) return false;
      log.scrollTop += row.getBoundingClientRect().top - top - Math.min(120, log.clientHeight / 3);
      return true;
    }, [log, text]);
  }

  /* ---- A chat: deliver, stream, trim ---- */
  const t = await launch({
    viewport: { width: 1280, height: 760 },
    app: () => {
      const ws = 'C:/Fixture';
      window.__calls = [];
      return {
        State: async () => ({ workspace: ws, recent: [ws] }),
        Sessions: async () => [{ sessionId: 'long', title: 'Long Chat', provider: 'codex', cwd: ws, open: false }, { sessionId: 'side', title: 'Side Chat', provider: 'codex', cwd: ws, open: false }],
        OpenSessionAt: async (w, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: w, open: true, working: false, queued: [] }),
        Send: async () => {}, Queue: async () => {}, Steer: async () => {}, Answer: async () => {},
      };
    },
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    await p.goto(require('./lib/page').PAGE);
    await t.waitReady();
    const longKey = await p.evaluate(() => window.dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'long' }));
    const logSel = '[data-win="' + longKey + '"] [data-view-id="activity"]';
    await p.locator(logSel).waitFor();
    // Turns arrive: a thought, a tool call and a message of each, the shape
    // of a long chat, past the page's kept-row count.
    await p.evaluate(() => {
      for (let i = 0; i < 240; i++) {
        const up = u => window.__emit('agent:update', { sessionId: 'long', update: u });
        up({ sessionUpdate: 'agent_thought_chunk', content: { type: 'text', text: 'thinking of ' + i } });
        up({ sessionUpdate: 'tool_call', toolCallId: 'tc' + i, title: 'Read file ' + i, kind: 'read', status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'row of file ' + i } }] });
        up({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Message number ' + i + '. A paragraph long enough to read: the answer spans a line or two here.\n\n' } });
      }
    });
    await p.waitForFunction(() => [...document.querySelectorAll('.activity-row')].some(el => el.textContent.includes('Message number 239.')), null, { timeout: 30000 });
    ok('a long chat is there to read back', (await p.evaluate(() => document.querySelectorAll('.activity-row').length)) > 400);
    ok('the reader is scrolled back to a middle message', await scrollTo(logSel, 'Message number 160.'));
    await t.shot('scroll-place-readback');
    // Leg 1: a new row delivered at the end.
    let before = await visibleTop(logSel, 'Message number 160.', 'delivery');
    await p.evaluate(() => {
      const up = u => window.__emit('agent:update', { sessionId: 'long', update: u });
      up({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Message number 240. The newest, delivered while reading back.\n\n' } });
    });
    await holds(logSel, 'Message number 160.', before, 'a message delivered at the end');
    // Leg 2: a reply streaming in chunks after it.
    await t.shot('scroll-place-stream');
    // Dinah UI's load: another window streams through the same frame queue.
    await p.evaluate(() => window.dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'side' }));
    await p.evaluate(key => pudlWindows.raise(key), longKey);
    before = await visibleTop(logSel, 'Message number 160.', 'streaming with the side chat open');
    await p.evaluate(async () => {
      const up = (id, u) => window.__emit('agent:update', { sessionId: id, update: u });
      for (let i = 0; i < 8; i++) {
        up('long', { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Streamed piece ' + i + '. ' } });
        up('side', { sessionUpdate: 'tool_call', toolCallId: 's' + i, title: 'busy work ' + i, kind: 'execute', status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'x'.repeat(2000) } }] });
        await new Promise(r => setTimeout(r, 30));
      }
    });
    await holds(logSel, 'Message number 160.', before, "a reply streaming in chunks, with another window's streaming beside it");
    // Leg 3: rows added until the oldest leave the page.
    before = await visibleTop(logSel, 'Message number 160.', 'trimming');
    await p.evaluate(() => {
      for (let i = 241; i < 285; i++) {
        const up = u => window.__emit('agent:update', { sessionId: 'long', update: u });
        up({ sessionUpdate: 'tool_call', toolCallId: 'tt' + i, title: 'Read ' + i, kind: 'read', status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'row ' + i } }] });
        up({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Message number ' + i + '.\n\n' } });
      }
    });
    await p.waitForFunction(() => document.querySelector('.trimmed-row') || [...document.querySelectorAll('.activity-row')].length > 500, null, { timeout: 15000 });
    await holds(logSel, 'Message number 160.', before, 'the oldest rows leaving the page');
    await t.shot('scroll-place-trim');

    /* ---- A group chat: deliver, then a full redraw ---- */
    const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
    const rooms = [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [person] }];
    await p.evaluate(() => {
      window.__roomMessages = {};
      window.go.main.App.BusOverview = async () => ({ inbox: [], unread: 0, rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [{ id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' }] }], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 });
      window.go.main.App.BusGroupMessages = async key => window.__roomMessages[key] || [];
      window.go.main.App.BusMarkGroupRead = async () => {};
      window.__roomMessages['release-review'] = Array.from({ length: 120 }, (_, i) => ({
        id: i + 1, createdAt: '2026-10-22T08:00:00Z', kind: 'text', from: 'Docs Pass', fromPerson: false, fromSession: 'window-d1', fromWorkspace: 'docs', room: 'release-review', roomTitle: 'Release Review',
        text: 'Group note ' + (i + 1) + '. Each note stands a couple of lines tall on the page, enough to read its place by.',
      }));
    });
    await p.evaluate(() => window.dinahBus.openGroup('release-review'));
    const glog = '.win[data-win-kind="bus-room"] .bus-log';
    await p.locator(glog + ' [data-bus-message="120"]').waitFor();
    ok('the group is drawn', (await p.evaluate(() => document.querySelectorAll('[data-bus-message]').length)) === 120);
    ok('scrolled back into its history', await scrollTo(glog, 'Group note 40.'));
    // Leg 4: a new message delivered into the group.
    before = await visibleTop(glog, 'Group note 40.', 'the group change');
    await p.evaluate(() => {
      window.__roomMessages['release-review'].push({ id: 121, createdAt: '2026-10-22T09:00:00Z', kind: 'text', from: 'Docs Pass', fromPerson: false, fromSession: 'window-d1', fromWorkspace: 'docs', room: 'release-review', roomTitle: 'Release Review', text: 'Group note 121. Newest.' });
      window.__emit('bus:changed', { counter: 2 });
    });
    await p.locator(glog + ' [data-bus-message="121"]').waitFor();
    await holds(glog, 'Group note 40.', before, 'a group message delivered');
    await t.shot('scroll-place-group-deliver');
    // Leg 5: a message up the list changes, so the whole conversation is drawn again.
    before = await visibleTop(glog, 'Group note 40.', 'the group change');
    await p.evaluate(() => {
      const m = window.__roomMessages['release-review'];
      m[1].text = 'Group note 2. Now says something else, and longer than before by a line or two, so its height is not the same.';
      window.__emit('bus:changed', { counter: 3 });
    });
    await holds(glog, 'Group note 40.', before, 'a group drawn again after a change up the list');
    await t.shot('scroll-place-group-redraw');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
