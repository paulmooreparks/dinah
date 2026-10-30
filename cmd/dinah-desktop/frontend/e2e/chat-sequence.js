// A chat's numbered events are applied in order, once; a gap is filled from the
// engine's log, a loss is reported, a new generation reopens the chat, and a
// resync asks the log for what followed. The log is stubbed here.
const { launch, PAGE } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => {
      window.__logs = [];       // calls of ChatEvents: [id, gen, after]
      window.__view = null;     // what ChatEvents answers next (a value, a function or a promise)
      window.__opened = [];
      return {
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        OpenSessionAt: async (ws, id) => { window.__opened.push(id); return { sessionId: id, title: id, provider: 'codex', cwd: ws, open: true, working: false, queued: [] }; },
        ChatEvents: async (id, gen, after) => {
          window.__logs.push([id, gen, after]);
          const v = typeof window.__view === 'function' ? window.__view(id, gen, after) : window.__view;
          return v;
        }
      };
    },
    ready: () => window.dinahWorkbench && window.dinahEditor,
    goto: false,
  });
  const { page, ok } = t;
  const ID = 'codex:seq';
  const message = n => ({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: ' m' + n + ' ' } });
  const live = (gen, n) => page.evaluate(([i, g, k, u]) => __emit('agent:update', { sessionId: i, gen: g, n: k, update: u }), [ID, gen, n, message(n)]);
  const entry = n => ({ n, name: 'agent:update', data: { update: message(n) } });
  const got = () => page.evaluate(() => window.__got.map(g => g.join(':')).join(','));
  const settle = () => page.waitForTimeout(250);
  const text = win => page.locator('[data-win="' + win + '"] .run-log').first().textContent();
  try {
    await page.goto(PAGE);
    await t.waitReady();
    const win = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:seq' }));
    await page.locator('[data-win="' + win + '"] [data-view-id="prompt"]').waitFor();
    await page.evaluate(() => { window.__got = []; dinahHost.on('agent:update', u => { if (u.sessionId === 'codex:seq') window.__got.push([u.gen === undefined ? 'x' : u.gen, u.n === undefined ? '-' : u.n]); }); });

    // In order, once.
    await live('g1', 1); await live('g1', 2); await live('g1', 2); await live('g1', 3);
    await settle();
    ok('numbered events are applied in order and a repeat is ignored', (await got()) === 'g1:1,g1:2,g1:3', await got());
    ok('no log call is made while the numbers follow each other', (await page.evaluate(() => window.__logs.length)) === 0);

    // A gap is read from the log before anything newer is applied.
    await page.evaluate(([e2, e3]) => { window.__view = { gen: 'g1', head: 5, events: [e2, e3], truncated: false, reset: false }; }, [entry(4), entry(5)]);
    await live('g1', 6);
    await settle();
    ok('a gap is filled from the log in order', (await got()) === 'g1:1,g1:2,g1:3,g1:4,g1:5,g1:6', await got());
    ok('the log is asked from the last applied number', JSON.stringify(await page.evaluate(() => window.__logs)) === JSON.stringify([[ID, 'g1', 3]]));
    ok('the filled text appears once, in order', /m4\s+m5\s+m6/.test(await text(win)) && (await text(win)).split('m5').length === 2);

    // Events that arrive while the log is read wait their turn.
    await page.evaluate(() => { window.__logs.length = 0; window.__got.length = 0; window.__release = null; window.__view = new Promise(r => { window.__release = r; }); });
    await live('g1', 9);
    await live('g1', 10);
    await settle();
    ok('events arriving during a read are held', (await got()) === '');
    await page.evaluate(entries => window.__release({ gen: 'g1', head: 8, events: entries, truncated: false, reset: false }), [entry(7), entry(8)]);
    await settle();
    ok('held events follow the filled ones in order', (await got()) === 'g1:7,g1:8,g1:9,g1:10', await got());
    ok('and one read serves them all', (await page.evaluate(() => window.__logs.length)) === 1, String(await page.evaluate(() => window.__logs.length)));

    // A log that no longer has the entries reports the loss and goes on.
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; window.__view = { gen: 'g1', head: 14, events: [], truncated: true, reset: false }; });
    await live('g1', 14);
    await settle();
    ok('a truncated log still lets the newer event through', (await got()) === 'g1:14', await got());
    ok('and the loss is reported', (await page.evaluate(() => dinahHost.notices.list(e => e.id === 'lost-activity-codex:seq').length)) === 1);
    await page.evaluate(() => dinahHost.notices.remove('lost-activity-codex:seq'));

    // A failed call is a loss too, and the held event still arrives.
    await page.evaluate(() => { window.__got.length = 0; window.__view = () => { throw new Error('call failed'); }; });
    await live('g1', 17);
    await settle();
    ok('a failed read does not lose the event that revealed the gap', (await got()) === 'g1:17', await got());
    ok('and the loss is reported', (await page.evaluate(() => dinahHost.notices.list(e => e.id === 'lost-activity-codex:seq').length)) === 1);
    await page.evaluate(() => dinahHost.notices.remove('lost-activity-codex:seq'));

    // A resync asks the log for what followed.
    await page.evaluate(([e]) => { window.__got.length = 0; window.__logs.length = 0; window.__view = { gen: 'g1', head: 18, events: [e], truncated: false, reset: false }; }, [entry(18)]);
    await page.evaluate(() => window.__resync());
    await settle();
    ok('after a resync the chat reads the log from its last number', JSON.stringify(await page.evaluate(() => window.__logs)) === JSON.stringify([[ID, 'g1', 17]]));
    ok('and applies what it returns', (await got()) === 'g1:18', await got());
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; window.__view = { gen: 'g1', head: 18, events: [], truncated: false, reset: false }; });
    await page.evaluate(() => window.__resync());
    await settle();
    ok('a page that missed nothing applies nothing', (await got()) === '' && (await page.evaluate(() => window.__logs.length)) === 1);

    // A resync that arrives while a read is running is not lost: the log is read again afterwards.
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; window.__view = new Promise(r => { window.__release = r; }); });
    await live('g1', 21);
    await settle();
    await page.evaluate(() => window.__resync());
    await settle();
    await page.evaluate(([a, b, c]) => {
      // The first read answers as of before the end of the stream was lost; the second finds what followed.
      window.__view = (id, gen, after) => ({ gen, head: 22, events: after < 22 ? [c] : [], truncated: false, reset: false });
      window.__release({ gen: 'g1', head: 20, events: [a, b], truncated: false, reset: false });
    }, [entry(19), entry(20), entry(22)]);
    await settle();
    ok('an event emitted after a running read and lost in the gap is read after a resync', (await got()) === 'g1:19,g1:20,g1:21,g1:22', await got());
    ok('with one more read, not one per request', (await page.evaluate(() => window.__logs.length)) === 2, String(await page.evaluate(() => window.__logs.length)));

    // Events held behind a resync's read that fails are not stranded.
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; window.__view = new Promise((res, rej) => { window.__reject = rej; }); });
    await page.evaluate(() => window.__resync());
    await settle();
    await live('g1', 30);
    await settle();
    ok('an event is held while the resync reads', (await got()) === '', await got());
    await page.evaluate(() => { window.__view = () => { throw new Error('call failed'); }; window.__reject(new Error('call failed')); });
    await settle();
    ok('held events are drawn when the resync read fails', (await got()) === 'g1:30', await got());
    ok('and the loss is reported', (await page.evaluate(() => dinahHost.notices.list(e => e.id === 'lost-activity-codex:seq').length)) === 1);
    await page.evaluate(() => dinahHost.notices.remove('lost-activity-codex:seq'));

    // A new generation: the chat opens again and the old generation's events are ignored.
    await page.evaluate(() => { window.__got.length = 0; window.__opened.length = 0; });
    await live('g2', 1);
    await settle();
    ok('the first event of another generation is kept, not lost to the reopening', (await got()) === 'g2:1', await got());
    await live('g1', 19);
    await settle();
    ok('events of the replaced generation are dropped', (await got()) === 'g2:1', await got());
    await live('g2', 1); await live('g2', 2);
    await settle();
    ok('the new generation goes on from its first event, once each', (await got()) === 'g2:1,g2:2', await got());
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; window.__view = { gen: 'g2', head: 5, events: [], truncated: false, reset: false }; });
    await live('g2', 5);
    await settle();
    // A gap that the first read does not close gets one more read, and only then is reported lost.
    const reads = async expected => { const l = await page.evaluate(() => window.__logs); return l.length === 2 && l.every(c => JSON.stringify(c) === JSON.stringify(expected)); };
    ok('a gap in the generation is read from the last number, and once more if it stays open', await reads([ID, 'g2', 2]), JSON.stringify(await page.evaluate(() => window.__logs)));
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; window.__view = { gen: 'g2b', head: 4, events: [], truncated: false, reset: false }; });
    await live('g2b', 4);
    await settle();
    ok('and from zero when the generation is new to the page', await reads([ID, 'g2b', 0]), JSON.stringify(await page.evaluate(() => window.__logs)));

    // A resync that finds the chat closed in the engine.
    await page.evaluate(() => { window.__got.length = 0; window.__view = { gen: '', head: 0, events: [], truncated: false, reset: true }; });
    await page.evaluate(() => window.__resync());
    await settle();
    await live('g3', 1);
    await settle();
    ok('a reset answered to a resync starts a new generation', (await got()) === 'g3:1', await got());

    // A resync that finds the chat open in the engine under another generation: nothing replays, so the log is read from its start.
    await page.evaluate(() => {
      window.__got.length = 0; window.__logs.length = 0;
      window.__view = (id, gen, after) => gen === 'g3'
        ? { gen: 'g4', head: 2, events: [], truncated: false, reset: true }
        : { gen, head: 2, events: [1, 2].map(n => ({ n, name: 'agent:update', data: { update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: ' r' + n + ' ' } } } })), truncated: false, reset: false };
    });
    await page.evaluate(() => window.__resync());
    await settle();
    ok('a chat the engine opened again is refilled from its log', (await got()) === 'g4:1,g4:2', await got());

    // The Info a page reads starts a chat it has no number for; it never moves one it has.
    const OTHER = 'codex:other';
    const liveOther = n => page.evaluate(([i, k]) => __emit('agent:update', { sessionId: i, gen: 'h1', n: k, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'o' + k } } }), [OTHER, n]);
    await page.evaluate(i => __emit('agent:session', { sessionId: i, title: i, provider: 'codex', cwd: 'C:/Fixture', open: true, streamGen: 'h1', streamHead: 7 }), OTHER);
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; dinahHost.on('agent:update', u => { if (u.sessionId === 'codex:other') window.__got.push(['o', u.n]); }); });
    await liveOther(8);
    await settle();
    ok('a chat starts from the head its Info reports', (await got()) === 'o:8' && (await page.evaluate(() => window.__logs.length)) === 0, await got());
    await page.evaluate(i => __emit('agent:session', { sessionId: i, streamGen: 'h1', streamHead: 40 }), OTHER);
    await page.evaluate(() => { window.__view = { gen: 'h1', head: 9, events: [{ n: 9, name: 'agent:update', data: { update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'o9' } } } }], truncated: false, reset: false }; });
    await liveOther(10);
    await settle();
    ok('a later Info never moves the number a page holds', (await got()) === 'o:8,o:9,o:10', await got());

    // A batch carries a number for each update.
    await page.evaluate(() => { window.__got.length = 0; });
    await page.evaluate(i => __emit('agent:updates', { sessionId: i, gen: 'h1', ns: [11, 13], updates: [
      { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'b11' } }, { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'b13' } }] }), OTHER);
    await page.evaluate(() => { window.__view = { gen: 'h1', head: 12, events: [{ n: 12, name: 'agent:turn', data: {} }], truncated: false, reset: false }; });
    await settle();
    ok('the updates of a batch are applied by their own numbers', (await got()) === 'o:11,o:13', await got());

    // Events without numbers go through as they did.
    await page.evaluate(i => { window.__got.length = 0; __emit('agent:update', { sessionId: i, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'plain' } } }); }, ID);
    await settle();
    ok('an event with no number is delivered as before', (await got()) === 'x:-', await got());
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
