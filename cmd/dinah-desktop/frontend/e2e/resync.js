// A resync runs every module's recovery once, after the queued agent events,
// never in two runs at once, and tells the person that activity may be missing.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => ({
      Sessions: async () => [],
      AgentProviders: async () => [],
      OpenSessionAt: async () => ({})
    }),
  });
  const { page, ok } = t;
  try {
    const count = () => page.evaluate(() => dinahHost.notices.list(e => e.id === 'lost-activity').length);
    ok('no notice before a resync', (await count()) === 0);

    // The hub: handlers run in order, one failing does not stop the rest.
    const hub = await page.evaluate(async () => {
      const log = [];
      const failures = [];
      const realFail = window.dinahHost.fail;
      window.dinahHost.fail = err => failures.push(String(err.message || err));
      const off = [
        window.dinahHost.onResync(() => { log.push('a'); }),
        window.dinahHost.onResync(() => { throw new Error('b failed'); }),
        window.dinahHost.onResync(() => new Promise(r => setTimeout(() => { log.push('c'); r(); }, 30)))
      ];
      window.__resync();
      await new Promise(r => setTimeout(r, 150));
      off.forEach(fn => fn());
      window.dinahHost.fail = realFail;
      return { log, failures };
    });
    ok('every handler runs for a resync', hub.log.join(',') === 'a,c');
    ok('a handler that fails is reported and the others still run', hub.failures.includes('b failed'), hub.failures.join(' | '));
    ok('no module handler fails against the stub', hub.failures.every(f => f === 'b failed'), hub.failures.join(' | '));

    // A resync during recovery runs the handlers once more afterwards, not at the same time.
    const overlap = await page.evaluate(async () => {
      let running = 0, most = 0, runs = 0;
      const off = window.dinahHost.onResync(() => new Promise(r => {
        runs++; running++; most = Math.max(most, running);
        setTimeout(() => { running--; r(); }, 40);
      }));
      window.__resync();
      await new Promise(r => setTimeout(r, 10));
      window.__resync(); window.__resync();
      await new Promise(r => setTimeout(r, 300));
      off();
      return { runs, most };
    });
    ok('resyncs during a run are folded into one more run', overlap.runs === 2, JSON.stringify(overlap));
    ok('two runs never overlap', overlap.most === 1, JSON.stringify(overlap));

    // Queued agent events are applied before the reread.
    // A recovery still running from the steps above would take the handler added here
    // and run it before the event below was applied, which is right for that resync,
    // so the step starts when none is running.
    const order = await page.evaluate(async () => {
      await window.dinahHost.settled();
      const log = [];
      const off1 = window.dinahHost.on('agent:queue', () => log.push('event'));
      const off2 = window.dinahHost.onResync(() => { log.push('reread'); });
      window.__emit('agent:queue', { sessionId: 'x', queued: [] });
      window.__resync();
      await new Promise(r => setTimeout(r, 200));
      off1(); off2();
      return log;
    });
    ok('events queued before a resync are applied before the reread', order.join(',') === 'event,reread', order.join(','));

    // The same promise when the queue is long: the render queue applies at most 128
    // events in a turn, so the point where the resync waits for them can fall at the
    // end of a turn, with an event emitted after the resync behind it. Every event
    // emitted before the resync is applied before the reread; the one after may follow it.
    const long = await page.evaluate(async () => {
      await window.dinahHost.settled();
      const log = [];
      const off1 = window.dinahHost.on('agent:queue', d => log.push(d.queued.length ? 'late' : 'early'));
      const off2 = window.dinahHost.onResync(() => { log.push('reread'); });
      for (let i = 0; i < 127; i++) window.__emit('agent:queue', { sessionId: 'x', queued: [] });
      window.__resync();
      window.__emit('agent:queue', { sessionId: 'x', queued: ['late'] });
      await window.dinahHost.settled();
      await new Promise(r => setTimeout(r, 300));
      off1(); off2();
      return log;
    });
    const reread = long.indexOf('reread');
    ok('with a long queue every event before the resync is applied before the reread', long.slice(0, reread).filter(e => e === 'early').length === 127, 'reread at ' + reread + ' of ' + long.length);

    // The lost-activity notice.
    await page.evaluate(() => dinahHost.notices.remove('lost-activity'));
    await page.evaluate(() => window.__resync());
    await page.waitForFunction(() => dinahHost.notices.list(e => e.id === 'lost-activity').length === 1);
    ok('a resync tells the person that activity may not have been shown', (await count()) === 1);
    await page.evaluate(() => window.__resync());
    await page.waitForTimeout(100);
    ok('a second resync updates the notice rather than adding another', (await count()) === 1);
    await page.evaluate(() => dinahHost.notices.remove('lost-activity'));
    await page.evaluate(() => window.__resync());
    await page.waitForFunction(() => dinahHost.notices.list(e => e.id === 'lost-activity').length === 1);
    ok('a dismissed notice returns with the next resync', (await count()) === 1);

    // Ids are the browser's business: events with any ids, in any order, cause no recovery.
    await page.evaluate(() => { window.__recovered = 0; dinahHost.onResync(() => { window.__recovered++; }); });
    await page.evaluate(() => { window.__emit('agent:queue', { sessionId: 'x', queued: [] }); window.__emit('settings:changed', {}); });
    await page.waitForTimeout(100);
    ok('ordinary events never start a recovery', (await page.evaluate(() => window.__recovered)) === 0);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
