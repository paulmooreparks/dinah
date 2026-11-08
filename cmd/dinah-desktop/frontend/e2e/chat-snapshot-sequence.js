// Journal reads must wait for the saved transcript that will receive them.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch();
  try {
    const result = await t.page.evaluate(async () => {
      const delivered = [], calls = [], losses = [];
      let release, resync;
      const host = window.dinahHost;
      window.dinahHost = { onResync: fn => { resync = fn; } };
      const sequence = dinahChatSequence.create({
        loading: () => false, reset: () => {}, loss: id => losses.push(id),
        deliver: (_, data) => delivered.push(data.n),
        app: () => ({ ChatEvents: async (_, gen, after) => {
          calls.push(after);
          if (calls.length === 1) return new Promise(resolve => { release = resolve; });
          return { gen, events: [{ n: 3, name: 'agent:update', data: {} }] };
        } })
      });
      window.dinahHost = host;
      sequence.info('chat', { streamGen: 'generation', streamHead: 1 });
      const oldRead = sequence.catchUp('chat');
      await Promise.resolve();
      sequence.startAt('chat', 'generation', 2);
      release({ gen: 'generation', events: [{ n: 3, name: 'agent:update', data: {} }] });
      await oldRead;
      const duringOldRead = delivered.slice();
      await resync();
      const duringResync = delivered.slice();
      sequence.receive('agent:update', { sessionId: 'chat', gen: 'generation', n: 3 });
      const duringLive = delivered.slice();
      await sequence.catchUp('chat');
      return { duringOldRead, duringResync, duringLive, delivered, calls, losses };
    });
    t.ok('an old journal read cannot update the transcript during replacement', result.duringOldRead.length === 0, JSON.stringify(result));
    t.ok('resync waits while the saved transcript is being drawn', result.duringResync.length === 0, JSON.stringify(result));
    t.ok('live updates wait while the saved transcript is being drawn', result.duringLive.length === 0, JSON.stringify(result));
    t.ok('catch-up delivers the final response once after replacement', JSON.stringify(result.delivered) === '[3]' && result.calls.length === 2 && result.calls[1] === 2 && !result.losses.length, JSON.stringify(result));
    const failure = await t.page.evaluate(async () => {
      let reject, reads = 0, losses = 0;
      const delivered = [];
      const sequence = dinahChatSequence.create({
        loading: () => false, reset: () => {}, loss: () => { losses++; },
        deliver: (_, data) => delivered.push(data.n),
        app: () => ({ ChatEvents: async (_, gen) => {
          if (++reads === 1) return new Promise((_, fail) => { reject = fail; });
          return { gen, events: [{ n: 3, name: 'agent:update', data: {} }] };
        } })
      });
      sequence.info('chat', { streamGen: 'generation', streamHead: 1 });
      const oldRead = sequence.catchUp('chat');
      await Promise.resolve();
      sequence.startAt('chat', 'generation', 2);
      sequence.receive('agent:update', { sessionId: 'chat', gen: 'generation', n: 3 });
      reject(new Error('The previous read failed.'));
      await oldRead;
      const beforeReady = delivered.slice();
      await sequence.catchUp('chat');
      return { beforeReady, delivered, losses };
    });
    t.ok('an obsolete read failure neither releases updates nor reports a false loss', !failure.beforeReady.length && !failure.losses && JSON.stringify(failure.delivered) === '[3]', JSON.stringify(failure));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
