// A chat's numbered events are applied in order, once; a gap is filled from the
// engine's log, a loss is reported, a new generation reopens the chat, and a
// resync asks the log for what followed. The log is stubbed here.
const { launch, PAGE, OUT } = require('./lib/page');
const fs = require('fs');
const path = require('path');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    init: () => {
      window.__sequenceTrace = [];
      const emit = window.__emit;
      window.__emit = (name, data) => {
        if (name === 'agent:update' && data.sessionId === 'codex:seq') {
          window.__sequenceTrace.push({ phase: 'emit', gen: data.gen, n: data.n, at: performance.now(), visibility: document.visibilityState });
        }
        return emit(name, data);
      };
    },
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
  // The page applies agent events from its render queue, which a loaded machine can
  // hold for a few hundred milliseconds, so a check that expects events waits for them
  // to arrive and then a further 250 ms in which a repeat or a stray could follow.
  const settle = async expected => {
    if (expected) await page.waitForFunction(e => window.__got.map(g => g.join(':')).join(',') === e, expected, { timeout: 5000 }).catch(() => {});
    await page.waitForTimeout(250);
  };
  const settleReads = async count => { await page.waitForFunction(n => window.__logs.length >= n, count, { timeout: 5000 }).catch(() => {}); await page.waitForTimeout(250); };
  const check = async (name, expected) => { const seen = await got(); ok(name, seen === expected, 'expected ' + (expected || 'nothing') + ', saw ' + (seen || 'nothing')); };
  const text = win => page.locator('[data-win="' + win + '"] .run-log').first().textContent();
  try {
    await page.goto(PAGE);
    await t.waitReady();
    const win = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:seq' }));
    await page.locator('[data-win="' + win + '"] [data-view-id="prompt"]').waitFor();
    await page.evaluate(() => {
      window.__sequenceFrames = [];
      window.__sequenceSampling = true;
      function sample(at) {
        if (!window.__sequenceSampling) return;
        window.__sequenceFrames.push({ at, delivered: window.__got.length });
        requestAnimationFrame(sample);
      }
      requestAnimationFrame(sample);
      window.__got = []; dinahHost.on('agent:update', u => { if (u.sessionId === 'codex:seq') {
      window.__got.push([u.gen === undefined ? 'x' : u.gen, u.n === undefined ? '-' : u.n]);
      window.__sequenceTrace.push({ phase: 'deliver', gen: u.gen, n: u.n, at: performance.now(), visibility: document.visibilityState });
    } }); });

    // In order, once.
    await live('g1', 1); await live('g1', 2); await live('g1', 2); await live('g1', 3);
    await settle('g1:1,g1:2,g1:3');
    const initial = await page.evaluate(() => {
      window.__sequenceSampling = false;
      return { expected: 'g1:1,g1:2,g1:3', observed: window.__got.map(g => g.join(':')).join(','), trace: window.__sequenceTrace.slice(), frames: window.__sequenceFrames, logs: window.__logs.slice(), opened: window.__opened.slice(), at: performance.now(), visibility: document.visibilityState };
    });
    fs.mkdirSync(OUT, { recursive: true });
    fs.writeFileSync(path.join(OUT, 'chat-sequence-initial.json'), JSON.stringify(initial, null, 2));
    ok('numbered events are applied in order and a repeat is ignored', initial.observed === initial.expected, JSON.stringify(initial));
    ok('no log call is made while the numbers follow each other', (await page.evaluate(() => window.__logs.length)) === 0);

    // A gap is read from the log before anything newer is applied.
    await page.evaluate(([e2, e3]) => { window.__view = { gen: 'g1', head: 5, events: [e2, e3], truncated: false, reset: false }; }, [entry(4), entry(5)]);
    await live('g1', 6);
    await settle('g1:1,g1:2,g1:3,g1:4,g1:5,g1:6');
    await check('a gap is filled from the log in order', 'g1:1,g1:2,g1:3,g1:4,g1:5,g1:6');
    ok('the log is asked from the last applied number', JSON.stringify(await page.evaluate(() => window.__logs)) === JSON.stringify([[ID, 'g1', 3]]));
    ok('the filled text appears once, in order', /m4\s+m5\s+m6/.test(await text(win)) && (await text(win)).split('m5').length === 2);

    // Events that arrive while the log is read wait their turn.
    await page.evaluate(() => { window.__logs.length = 0; window.__got.length = 0; window.__release = null; window.__view = new Promise(r => { window.__release = r; }); });
    await live('g1', 9);
    await live('g1', 10);
    await settle('');
    await check('events arriving during a read are held', '');
    await page.evaluate(entries => window.__release({ gen: 'g1', head: 8, events: entries, truncated: false, reset: false }), [entry(7), entry(8)]);
    await settle('g1:7,g1:8,g1:9,g1:10');
    await check('held events follow the filled ones in order', 'g1:7,g1:8,g1:9,g1:10');
    ok('and one read serves them all', (await page.evaluate(() => window.__logs.length)) === 1, String(await page.evaluate(() => window.__logs.length)));

    // A log that no longer has the entries reports the loss and goes on.
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; window.__view = { gen: 'g1', head: 14, events: [], truncated: true, reset: false }; });
    await live('g1', 14);
    await settle('g1:14');
    await check('a truncated log still lets the newer event through', 'g1:14');
    ok('and the loss is reported', (await page.evaluate(() => dinahHost.notices.list(e => e.id === 'lost-activity-codex:seq').length)) === 1);
    await page.evaluate(() => dinahHost.notices.remove('lost-activity-codex:seq'));

    // A failed call is a loss too, and the held event still arrives.
    await page.evaluate(() => { window.__got.length = 0; window.__view = () => { throw new Error('call failed'); }; });
    await live('g1', 17);
    await settle('g1:17');
    await check('a failed read does not lose the event that revealed the gap', 'g1:17');
    ok('and the loss is reported', (await page.evaluate(() => dinahHost.notices.list(e => e.id === 'lost-activity-codex:seq').length)) === 1);
    await page.evaluate(() => dinahHost.notices.remove('lost-activity-codex:seq'));

    // A resync asks the log for what followed.
    await page.evaluate(([e]) => { window.__got.length = 0; window.__logs.length = 0; window.__view = { gen: 'g1', head: 18, events: [e], truncated: false, reset: false }; }, [entry(18)]);
    await page.evaluate(() => window.__resync());
    await settle('g1:18');
    ok('after a resync the chat reads the log from its last number', JSON.stringify(await page.evaluate(() => window.__logs)) === JSON.stringify([[ID, 'g1', 17]]));
    await check('and applies what it returns', 'g1:18');
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
    await settle('g1:19,g1:20,g1:21,g1:22');
    await check('an event emitted after a running read and lost in the gap is read after a resync', 'g1:19,g1:20,g1:21,g1:22');
    ok('with one more read, not one per request', (await page.evaluate(() => window.__logs.length)) === 2, String(await page.evaluate(() => window.__logs.length)));

    // Events held behind a resync's read that fails are not stranded.
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; window.__view = new Promise((res, rej) => { window.__reject = rej; }); });
    await page.evaluate(() => window.__resync());
    await settle();
    await live('g1', 30);
    await settle('');
    await check('an event is held while the resync reads', '');
    await page.evaluate(() => { window.__view = () => { throw new Error('call failed'); }; window.__reject(new Error('call failed')); });
    await settle('g1:30');
    await check('held events are drawn when the resync read fails', 'g1:30');
    ok('and the loss is reported', (await page.evaluate(() => dinahHost.notices.list(e => e.id === 'lost-activity-codex:seq').length)) === 1);
    await page.evaluate(() => dinahHost.notices.remove('lost-activity-codex:seq'));

    // A new generation: the chat opens again and the old generation's events are ignored.
    await page.evaluate(() => { window.__got.length = 0; window.__opened.length = 0; });
    await live('g2', 1);
    await settle('g2:1');
    await check('the first event of another generation is kept, not lost to the reopening', 'g2:1');
    await live('g1', 19);
    await settle('g2:1');
    await check('events of the replaced generation are dropped', 'g2:1');
    await live('g2', 1); await live('g2', 2);
    await settle('g2:1,g2:2');
    await check('the new generation goes on from its first event, once each', 'g2:1,g2:2');
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; window.__view = { gen: 'g2', head: 5, events: [], truncated: false, reset: false }; });
    await live('g2', 5);
    await settleReads(2);
    // A gap that the first read does not close gets one more read, and only then is reported lost.
    const reads = async expected => { const l = await page.evaluate(() => window.__logs); return l.length === 2 && l.every(c => JSON.stringify(c) === JSON.stringify(expected)); };
    ok('a gap in the generation is read from the last number, and once more if it stays open', await reads([ID, 'g2', 2]), JSON.stringify(await page.evaluate(() => window.__logs)));
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; window.__view = { gen: 'g2b', head: 4, events: [], truncated: false, reset: false }; });
    await live('g2b', 4);
    await settleReads(2);
    ok('and from zero when the generation is new to the page', await reads([ID, 'g2b', 0]), JSON.stringify(await page.evaluate(() => window.__logs)));

    // A resync that finds the chat closed in the engine.
    await page.evaluate(() => { window.__got.length = 0; window.__view = { gen: '', head: 0, events: [], truncated: false, reset: true }; });
    await page.evaluate(() => window.__resync());
    await settle();
    await live('g3', 1);
    await settle('g3:1');
    await check('a reset answered to a resync starts a new generation', 'g3:1');

    // A resync that finds the chat open in the engine under another generation: nothing replays, so the log is read from its start.
    await page.evaluate(() => {
      window.__got.length = 0; window.__logs.length = 0;
      window.__view = (id, gen, after) => gen === 'g3'
        ? { gen: 'g4', head: 2, events: [], truncated: false, reset: true }
        : { gen, head: 2, events: [1, 2].map(n => ({ n, name: 'agent:update', data: { update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: ' r' + n + ' ' } } } })), truncated: false, reset: false };
    });
    await page.evaluate(() => window.__resync());
    await settle('g4:1,g4:2');
    await check('a chat the engine opened again is refilled from its log', 'g4:1,g4:2');

    // The Info a page reads starts a chat it has no number for; it never moves one it has.
    const OTHER = 'codex:other';
    const liveOther = n => page.evaluate(([i, k]) => __emit('agent:update', { sessionId: i, gen: 'h1', n: k, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'o' + k } } }), [OTHER, n]);
    await page.evaluate(i => __emit('agent:session', { sessionId: i, title: i, provider: 'codex', cwd: 'C:/Fixture', open: true, streamGen: 'h1', streamHead: 7 }), OTHER);
    await page.evaluate(() => { window.__got.length = 0; window.__logs.length = 0; dinahHost.on('agent:update', u => { if (u.sessionId === 'codex:other') window.__got.push(['o', u.n]); }); });
    await liveOther(8);
    await settle('o:8');
    await check('a chat starts from the head its Info reports', 'o:8');
    ok('without reading the log', (await page.evaluate(() => window.__logs.length)) === 0);
    await page.evaluate(i => __emit('agent:session', { sessionId: i, streamGen: 'h1', streamHead: 40 }), OTHER);
    await page.evaluate(() => { window.__view = { gen: 'h1', head: 9, events: [{ n: 9, name: 'agent:update', data: { update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'o9' } } } }], truncated: false, reset: false }; });
    await liveOther(10);
    await settle('o:8,o:9,o:10');
    await check('a later Info never moves the number a page holds', 'o:8,o:9,o:10');

    // A batch carries a number for each update.
    await page.evaluate(() => { window.__got.length = 0; });
    await page.evaluate(i => __emit('agent:updates', { sessionId: i, gen: 'h1', ns: [11, 13], updates: [
      { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'b11' } }, { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'b13' } }] }), OTHER);
    await page.evaluate(() => { window.__view = { gen: 'h1', head: 12, events: [{ n: 12, name: 'agent:turn', data: {} }], truncated: false, reset: false }; });
    await settle('o:11,o:13');
    await check('the updates of a batch are applied by their own numbers', 'o:11,o:13');

    // Events without numbers go through as they did.
    await page.evaluate(i => { window.__got.length = 0; __emit('agent:update', { sessionId: i, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'plain' } } }); }, ID);
    await settle('x:-');
    await check('an event with no number is delivered as before', 'x:-');
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
