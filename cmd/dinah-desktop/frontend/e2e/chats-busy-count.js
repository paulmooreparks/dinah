// The Chats window's busy count (docs/design/notifications.md). The window
// gets data-win-count with the number of open chats, in every workspace, that
// have a turn in progress or any background task or subagent still running,
// and data-win-count-label="busy" for PUDL's accessible text. No busy chat
// removes data-win-count. A chat under Needs You with a task still running
// counts, an idle chat does not, a chat in a workspace the list does not show
// counts, a chat running in another window does not, and the value follows
// the engine's states as a turn ends. The attribute is written only when its
// value changes.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 950 }, app: () => {
    const alpha = 'C:/Alpha', beta = 'C:/Beta';
    const chat = (id, cwd) => ({ sessionId: id, title: id, cwd, provider: 'codex', open: true, updatedAt: '2026-11-26T00:00:00Z' });
    const lists = { [alpha]: [chat('turn', alpha), chat('task', alpha), chat('asks', alpha), chat('quiet', alpha)], [beta]: [chat('far-turn', beta), chat('far-quiet', beta)] };
    const idle = () => ({ state: 'idle', why: 'No work in progress' });
    window.__execs = { turn: idle(), task: idle(), asks: idle(), quiet: idle(), 'far-turn': idle(), 'far-quiet': idle() };
    let version = 1;
    const snapshot = () => ({ stateGen: 'busy', stateVer: version,
      chats: Object.keys(__execs).map(id => ({ sessionId: id, bus: 'bus-' + id, life: 'open', window: false, exec: structuredClone(__execs[id]) }))
        .concat([{ sessionId: '', bus: 'bus-elsewhere', life: 'open', window: false, elsewhere: 'Running in another Dinah window' }]),
      groups: [] });
    // Sets some chats' execution states and sends the engine's next snapshot.
    window.__busy = changes => { Object.assign(__execs, changes); version++; __emit('agent:chatstate', snapshot()); };
    return { State: async () => ({ workspace: alpha, recent: [alpha, beta], workspaces: [alpha, beta] }),
      SavedChats: async () => ({ generation: 'busy-fixture', chats: Object.values(lists).flat() }),
      Sessions: async () => lists[alpha], SessionsAt: async ws => lists[ws] || [],
      OpenSessionAt: async (_, id) => Object.values(lists).flat().find(c => c.sessionId === id),
      ChatStates: async () => snapshot() };
  } });
  const { page: p, ok } = t;
  const settle = () => p.evaluate(() => new Promise(r => setTimeout(() => requestAnimationFrame(() => requestAnimationFrame(r)), 50)));
  const read = () => p.evaluate(() => {
    const w = dinahHelpers.windowOf('chat-browser');
    return { has: w.hasAttribute('data-win-count'), count: w.getAttribute('data-win-count'), label: w.getAttribute('data-win-count-label'),
      rows: [...document.querySelectorAll('#sessions [data-session-row]')].map(r => r.dataset.sessionRow),
      groupOf: Object.fromEntries([...document.querySelectorAll('#sessions [data-session-row]')].map(r => [r.dataset.sessionRow, r.closest('[data-section]').dataset.section])) };
  });
  const exec = (state, more) => Object.assign({ state, why: state }, more || {});
  const step = async (changes, want) => {
    await p.evaluate(c => __busy(c), changes);
    await p.waitForFunction(n => { const w = dinahHelpers.windowOf('chat-browser'); return n ? w.getAttribute('data-win-count') === n : !w.hasAttribute('data-win-count'); }, want, { timeout: 3000 }).catch(() => {});
    await settle();
    return read();
  };
  try {
    // A chat busy before the window opens is counted as it opens.
    await p.evaluate(() => { if (dinahHelpers.windowOf('chat-browser')) pudlWindows.close('chat-browser'); });
    await p.evaluate(() => __busy({ turn: { state: 'working', why: 'working' } }));
    await settle();
    const closedFirst = await p.evaluate(() => !dinahHelpers.windowOf('chat-browser'));
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.waitForFunction(() => dinahHelpers.windowOf('chat-browser') && dinahHelpers.windowOf('chat-browser').getAttribute('data-win-count') === '1', null, { timeout: 3000 }).catch(() => {});
    ok('a chat busy before the window opens is counted when it opens', closedFirst && await p.evaluate(() => dinahHelpers.windowOf('chat-browser').getAttribute('data-win-count')) === '1', JSON.stringify({ closedFirst }));
    await p.evaluate(() => __busy({ turn: { state: 'idle', why: 'No work in progress' } }));
    await p.selectOption('#chat-scope', 'current');
    await p.waitForFunction(() => document.querySelectorAll('#sessions [data-session-row]').length === 4);
    // Counts every write of the attribute from here on.
    await p.evaluate(() => {
      window.__countWrites = [];
      new MutationObserver(records => records.forEach(r => __countWrites.push(r.target.getAttribute('data-win-count'))))
        .observe(dinahHelpers.windowOf('chat-browser'), { attributes: true, attributeFilter: ['data-win-count'] });
    });
    let s = await step({}, null);
    ok('no busy chat: the window has no count, and the label says busy', !s.has && s.label === 'busy', JSON.stringify(s));

    s = await step({ turn: exec('working') }, '1');
    ok('a chat in a turn counts', s.count === '1', JSON.stringify(s));

    s = await step({ task: exec('background', { tasks: 1 }) }, '2');
    ok('a chat with only a background task counts', s.count === '2', JSON.stringify(s));

    s = await step({ asks: exec('waiting', { subagents: 1 }) }, '3');
    ok('a Needs You chat with a subagent still running counts', s.count === '3' && s.groupOf.asks === 'chats-Needs You', JSON.stringify(s));
    s = await step({ asks: exec('waiting', { tasks: 1 }) }, '3');
    ok('a Needs You chat with a background task counts', s.count === '3' && s.groupOf.asks === 'chats-Needs You', JSON.stringify(s));
    s = await step({ asks: exec('waiting') }, '2');
    ok('a Needs You chat with nothing running does not count', s.count === '2', JSON.stringify(s));
    s = await step({ asks: exec('stuck', { tasks: 2 }) }, '3');
    ok('a stuck chat with tasks still running counts once', s.count === '3', JSON.stringify(s));

    s = await step({ quiet: exec('idle') }, '3');
    ok('an idle chat does not count', s.count === '3' && s.groupOf.quiet === 'chats-Chats', JSON.stringify(s));

    s = await step({ 'far-turn': exec('working', { tasks: 1 }) }, '4');
    ok('a chat in another workspace counts while the list shows one workspace', s.count === '4' && !s.rows.includes('far-turn'), JSON.stringify(s));
    ok('a chat running in another window is not counted', s.count === '4', JSON.stringify(s));

    s = await step({ turn: exec('idle') }, '3');
    ok('the count drops when a turn ends', s.count === '3', JSON.stringify(s));
    s = await step({ 'far-turn': exec('background', { tasks: 1 }) }, '3');
    ok('a turn ending with a task still running leaves the chat counted', s.count === '3', JSON.stringify(s));

    await p.selectOption('#chat-scope', 'all');
    await p.waitForFunction(() => document.querySelectorAll('#sessions [data-session-row]').length === 6);
    s = await read();
    ok('changing the list to every workspace leaves the count as it was', s.count === '3', JSON.stringify(s));

    const before = await p.evaluate(() => __countWrites.length);
    await step({ quiet: exec('idle'), 'far-quiet': exec('idle') }, '3');
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await settle();
    const after = await p.evaluate(() => __countWrites.length);
    ok('a snapshot or window change that leaves the count alone writes nothing', after === before, JSON.stringify({ before, after }));

    s = await step({ task: exec('idle'), asks: exec('idle'), 'far-turn': exec('idle') }, null);
    ok('the last busy chat going idle removes the count', !s.has && s.label === 'busy', JSON.stringify(s));
    const writes = await p.evaluate(() => __countWrites);
    ok('each write changed the value', writes.every((v, i) => i === 0 || v !== writes[i - 1]), JSON.stringify(writes));
    await t.shot('chats-busy-count');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
