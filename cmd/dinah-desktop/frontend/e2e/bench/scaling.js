// How the page's cost of one ordinary action grows with the number of open
// chats. A measurement, not a pass-or-fail suite, so run.ps1 leaves this
// folder out.
//
// For each count it opens that many chats, each in its own window with a
// saved history, lets the page settle, and then times a fixed set of actions
// with the browser's own counters (Performance.getMetrics over the DevTools
// protocol: main-thread task time, script, style and layout). Each action
// touches one chat, so an action whose cost rises with the count is doing
// work for chats it does not concern.
//
//   node bench/scaling.js [outFolder] [counts] [messagesPerChat]
//   PROFILE=action node bench/scaling.js ...   also writes a CPU profile of
//                                               that action at the largest count
const { launch } = require('../lib/page');
const fs = require('fs');
const path = require('path');
const OUT = process.argv[2] || require('os').tmpdir();
const COUNTS = (process.argv[3] || '1,8,16,32').split(',').map(Number);
const MESSAGES = Number(process.argv[4] || 120);
const CATALOG = Number(process.env.CATALOG || 100);
// STATE_KB is the size of the saved UI state the page starts with.
const STATE_KB = Number(process.env.STATE_KB || 600);
const PROFILE = process.env.PROFILE || '';
// TRACE=action sums the browser's own timeline events for that action at the largest count.
const TRACE = process.env.TRACE || '';
const ONLY = (process.env.ONLY || '').split(',').filter(Boolean);

const KEYS = ['TaskDuration', 'ScriptDuration', 'RecalcStyleDuration', 'LayoutDuration', 'RecalcStyleCount', 'LayoutCount', 'Nodes', 'JSEventListeners'];
async function metrics(cdp) {
  const all = (await cdp.send('Performance.getMetrics')).metrics, out = {};
  for (const m of all) if (KEYS.includes(m.name)) out[m.name] = m.value;
  return out;
}
const settle = page => page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 30)))));

async function run(count, last) {
  const t = await launch({
    viewport: { width: 1600, height: 1000 }, timeout: 120000,
    args: { count, messages: MESSAGES, catalog: CATALOG, stateKB: STATE_KB },
    app: ({ count, messages, catalog, stateKB }) => {
      const cwd = 'C:/Fixture';
      const ids = Array.from({ length: count }, (_, i) => 'codex:chat' + i);
      const info = (id, open) => ({ sessionId: id, title: 'Chat ' + id.split(':')[1], provider: 'codex', cwd, open, working: false, queued: [], updatedAt: '2026-11-04T01:00:00Z', chatState: { life: open ? 'open' : 'closed', window: open, exec: { state: 'idle', why: 'No work in progress' } } });
      const closed = Array.from({ length: catalog }, (_, i) => info('codex:saved' + i, false));
      const opened = new Set();
      window.__ids = ids;
      window.__saves = 0; window.__saveBytes = 0;
      return {
        State: async () => ({ workspace: cwd, recent: [cwd], defaultProvider: 'codex', workspaceId: 'w1', workspaceRecords: [{ id: 'w1', name: 'Fixture', executionPath: cwd, sources: [] }] }),
        Sessions: async () => ids.map(id => info(id, opened.has(id))).concat(closed), SessionsAt: async () => ids.map(id => info(id, opened.has(id))).concat(closed),
        ChatStates: async () => ({ chats: [], groups: [] }),
        // The provider replays the chat's history as it opens, as a resumed chat's does.
        OpenSessionAt: async (ws, id) => {
          for (let i = 0; i < messages; i++) window.__emit('agent:update', { sessionId: id, update: { sessionUpdate: i % 2 ? 'agent_message_chunk' : 'user_message_chunk', content: { type: 'text', text: 'Saved row ' + i + ' of ' + id + '. ' + 'Some words of an ordinary answer, with `code` and a list.\n\n- one\n- two\n\n'.repeat(3) } } });
          opened.add(id);
          return info(id, true);
        },
        // A saved UI state of the size a long-used installation has: about 600 KB in 600 values.
        ReadUIState: async () => {
          const values = {};
          for (let i = 0; i < stateKB / 6; i++) values['dinah-decisions:request%3Amention%3A' + i] = { text: 'x'.repeat(3400), at: i };
          for (let i = 0; i < stateKB * 2 / 3; i++) values['dinah-scroll:old-' + i] = { top: i * 10, left: 0, end: false };
          for (let i = 0; i < stateKB / 8; i++) values['content-snapshot:item-' + i] = 'x'.repeat(1000);
          return { version: 1, values };
        },
        SaveUIState: async value => { window.__saves++; window.__saveBytes += JSON.stringify(value).length; },
        SaveUIStateChanges: async value => { window.__saves++; window.__saveBytes += JSON.stringify(value).length; },
        Send: async () => {}, Queue: async () => {},
      };
    },
    ready: 'ui-state', goto: false,
  });
  const p = t.page, cdp = await t.context.newCDPSession(p);
  // PUDL_DIR serves another copy of the vendored library, to measure a candidate change to it without touching the vendored files.
  if (process.env.PUDL_DIR) await p.route('http://dinah.test/pudl/**', route => { const file = path.join(process.env.PUDL_DIR, new URL(route.request().url()).pathname.replace(/^\/pudl\//, '')); return fs.existsSync(file) ? route.fulfill({ path: file }) : route.fulfill({ status: 404, body: 'Not found' }); });
  await p.goto('http://dinah.test/index.html'); await t.waitReady();
  await cdp.send('Performance.enable');
  const result = { count, actions: {} };
  try {
    const openStart = Date.now();
    // Open the chats one after another, as a restored layout does.
    for (let i = 0; i < count; i++) {
      const before = i === count - 1 ? await metrics(cdp) : null, at = Date.now();
      await p.evaluate(async i => { await dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: __ids[i] }); }, i);
      await p.waitForFunction(([i, n]) => { const w = document.querySelectorAll('.win[data-win-kind="chat-session"]'); return w.length === i + 1 && [...w].every(x => x.querySelectorAll('.msg-text').length >= Math.min(n, 2)); }, [i, MESSAGES], { timeout: 20000 }).catch(async e => { console.log('STATE ' + JSON.stringify(await p.evaluate(() => [...document.querySelectorAll('.win')].map(w => [w.dataset.winKind, w.querySelectorAll('.msg-text').length, w.querySelectorAll('.activity-row').length])))); throw e; });
      if (before) { await settle(p); const after = await metrics(cdp); result.actions['open-last-chat'] = { per: 1, wall: Date.now() - at, task: (after.TaskDuration - before.TaskDuration) * 1000, script: (after.ScriptDuration - before.ScriptDuration) * 1000, style: (after.RecalcStyleDuration - before.RecalcStyleDuration) * 1000, layout: (after.LayoutDuration - before.LayoutDuration) * 1000 }; }
    }
    await p.waitForTimeout(1500); await settle(p);
    result.openAllMs = Date.now() - openStart;
    // LAYOUT=stack arranges the chat windows as a long-used desktop has them: windows of much the same size
    // laid over one another at sixteen places, so that most show a strip or nothing. The default is the
    // layer's own cascade, in which every window shows a strip.
    if (process.env.LAYOUT === 'stack') {
      await p.evaluate(() => {
        const places = ['0.342,0.062,0.48,0.687', '0.418,0.232,0.55,0.75', '0.274,0.142,0.499,0.641', '0.448,0.267,0.552,0.733', '0.028,0.194,0.55,0.75', '0.324,0.061,0.55,0.75', '0.135,0.19,0.552,0.733', '0.326,0.199,0.552,0.733', '0.387,0.039,0.55,0.75', '0.577,0,0.404,0.719', '0.003,0.079,0.55,0.75', '0.45,0.025,0.55,0.75', '0.101,0.134,0.55,0.713', '0.338,0.049,0.55,0.75', '0,0.264,0.437,0.7', '0.481,0.183,0.516,0.757'];
        const query = new URLSearchParams(location.search);
        [...document.querySelectorAll('.win[data-win-kind="chat-session"]')].forEach((w, i) => query.set('p.' + w.dataset.win, 'floating:' + places[i % places.length]));
        history.replaceState(history.state, '', location.pathname + '?' + query.toString() + location.hash);
        window.dispatchEvent(new PopStateEvent('popstate'));
      });
      await p.waitForTimeout(800); await settle(p);
    }
    result.covered = await p.evaluate(() => document.querySelectorAll('.win[data-dinah-covered]').length);
    // EXPERIMENT is page script run once the chats are open, to try a change before building it.
    if (process.env.EXPERIMENT) { await p.evaluate(process.env.EXPERIMENT); await p.waitForTimeout(500); await settle(p); }
    const base = await metrics(cdp);
    result.nodes = base.Nodes; result.listeners = base.JSEventListeners;

    // PROBE is a module that takes over once the chats are open: it is called with the page and the protocol session.
    if (process.env.PROBE) { await require(path.resolve(process.env.PROBE))({ p, cdp, count, settle, metrics }); return result; }
    // VARIANTS is a JSON list of [name, css]. Each stylesheet is tried in turn in the same page, and the cost of
    // one frame that repaints a few words of the front window's title is reported: the median of five rounds of forty.
    if (process.env.VARIANTS) {
      for (const [name, css] of [['as it is', '']].concat(JSON.parse(process.env.VARIANTS))) {
        await p.evaluate(css => { const s = document.createElement('style'); s.id = 'variant'; s.textContent = css; document.head.appendChild(s); }, css);
        await p.waitForTimeout(400); await settle(p);
        const rounds = [];
        for (let r = 0; r < 5; r++) {
          const before = await metrics(cdp);
          await p.evaluate(async () => { for (let i = 0; i < 40; i++) { document.querySelector('.win.active .win-title').firstChild.data = 'Chat ' + i; await new Promise(r => requestAnimationFrame(r)); } });
          const after = await metrics(cdp);
          rounds.push((after.TaskDuration - before.TaskDuration) * 1000 / 40);
        }
        rounds.sort((a, b) => a - b);
        console.log('VARIANT ' + count + ' chats  ' + rounds[2].toFixed(2).padStart(7) + ' ms a frame (' + rounds[0].toFixed(1) + ' to ' + rounds[4].toFixed(1) + ')  ' + name);
        await p.evaluate(() => document.getElementById('variant').remove());
      }
      return result;
    }
    const front = count - 1, back = 0;
    const chunk = id => ({ sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Streamed words that make up part of an answer. ' } } });
    const actions = {
      // Nothing happens: what the page spends by itself in two seconds.
      'idle-2s': { per: 1, run: () => p.waitForTimeout(2000) },
      // Forty chunks to the chat in front, one a frame.
      'stream-front': { per: 40, run: () => p.evaluate(async ([id]) => { for (let i = 0; i < 40; i++) { __emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Streamed words that make up part of an answer. ' } } }); await new Promise(r => requestAnimationFrame(r)); } }, ['codex:chat' + front]) },
      // Forty chunks to a chat behind the others.
      'stream-behind': { per: 40, run: () => p.evaluate(async ([id]) => { for (let i = 0; i < 40; i++) { __emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Streamed words that make up part of an answer. ' } } }); await new Promise(r => requestAnimationFrame(r)); } }, ['codex:chat' + back]) },
      // Every chat streams at once: ten frames, a chunk to each chat a frame. Reported for each chunk.
      'stream-all': { per: 10 * count, run: () => p.evaluate(async () => { for (let i = 0; i < 10; i++) { for (const id of __ids) __emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Streamed words that make up part of an answer. ' } } }); await new Promise(r => requestAnimationFrame(r)); } }) },
      // One chat's state changes: working, then idle, ten times.
      'session-state': { per: 20, run: () => p.evaluate(async ([id]) => { for (let i = 0; i < 20; i++) { const working = i % 2 === 0; __emit('agent:session', { sessionId: id, title: 'Chat ' + id.split(':')[1], provider: 'codex', cwd: 'C:/Fixture', open: true, working, queued: [], chatState: { life: 'open', window: true, exec: { state: working ? 'working' : 'idle', why: working ? 'Working' : 'No work in progress' } } }); await new Promise(r => requestAnimationFrame(r)); } }, ['codex:chat' + back]) },
      // A turn ends in one chat.
      'turn-end': { per: 10, run: () => p.evaluate(async ([id]) => { for (let i = 0; i < 10; i++) { __emit('agent:turn', { sessionId: id, stopReason: 'end_turn' }); await new Promise(r => requestAnimationFrame(r)); } }, ['codex:chat' + back]) },
      // The person brings ten windows to the front in turn.
      'raise-window': { per: 10, run: () => p.evaluate(async () => { const wins = [...document.querySelectorAll('.win[data-win-kind="chat-session"]')].map(w => w.dataset.win); for (let i = 0; i < 10; i++) { pudlWindows.raise(wins[i % wins.length]); await new Promise(r => requestAnimationFrame(r)); } }) },
      // The person types forty characters into the chat in front.
      // The keys go through the browser's own input path; a locator for each key would search the whole page.
      'typing': { per: 40, run: async () => { await p.evaluate(() => document.querySelector('.win.active [data-view-id="prompt"]').focus()); for (const key of 'The quick brown fox jumps over the lazy d') { await p.keyboard.press(key === ' ' ? 'Space' : key); } } },
      // The person scrolls the transcript in front: twenty wheel steps up, then back to the end.
      'scroll-transcript': { per: 40, run: async () => { const box = await p.evaluate(() => { const r = document.querySelector('.win.active [data-view-id="activity"]').getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; }); await p.mouse.move(box[0], box[1]); for (let i = 0; i < 20; i++) { await p.mouse.wheel(0, -120); await p.evaluate(() => new Promise(r => requestAnimationFrame(r))); } for (let i = 0; i < 20; i++) { await p.mouse.wheel(0, 240); await p.evaluate(() => new Promise(r => requestAnimationFrame(r))); } await p.waitForTimeout(400); } },
      // A team's state event, which names no chat.
      'team-state': { per: 10, run: () => p.evaluate(async () => { for (let i = 0; i < 10; i++) { document.dispatchEvent(new Event('dinah:team-state')); await new Promise(r => requestAnimationFrame(r)); } }) },
      // The bus says its overview changed.
      'bus-changed': { per: 10, run: () => p.evaluate(async () => { for (let i = 0; i < 10; i++) { __emit('bus:changed', {}); await new Promise(r => requestAnimationFrame(r)); } }) },
    };
    // CUSTOM is page script run forty times, once a frame, as an action of its own (with i as the round).
    if (process.env.CUSTOM) actions.custom = { per: 40, run: () => p.evaluate(async source => { const step = new Function('i', source); for (let i = 0; i < 40; i++) { step(i); await new Promise(r => requestAnimationFrame(r)); } }, process.env.CUSTOM) };
    for (const [name, action] of Object.entries(actions)) {
      if (ONLY.length && !ONLY.includes(name)) continue;
      await settle(p);
      const callsBefore = await p.evaluate(() => __bridgeCalls.length), savesBefore = await p.evaluate(() => [__saves, __saveBytes]);
      const profiling = last && PROFILE.split(',').includes(name);
      if (profiling) { await cdp.send('Profiler.enable'); await cdp.send('Profiler.setSamplingInterval', { interval: 200 }); await cdp.send('Profiler.start'); }
      const tracing = last && TRACE.split(',').includes(name), events = [];
      if (tracing) { cdp.on('Tracing.dataCollected', d => events.push(...d.value)); await cdp.send('Tracing.start', { transferMode: 'ReportEvents', traceConfig: { includedCategories: ['devtools.timeline', 'disabled-by-default-devtools.timeline', 'blink', 'cc', 'toplevel'].concat(process.env.REASONS ? ['disabled-by-default-devtools.timeline.invalidationTracking'] : []) } }); }
      const before = await metrics(cdp), at = Date.now();
      await action.run();
      await settle(p);
      const after = await metrics(cdp), wall = Date.now() - at;
      if (tracing) {
        const done = new Promise(r => cdp.once('Tracing.tracingComplete', r)); await cdp.send('Tracing.end'); await done;
        // Self time by event name on the renderer's main thread: each complete event's duration less its children's.
        const main = events.find(e => e.name === 'thread_name' && e.args && e.args.name === 'CrRendererMain');
        const mine = events.filter(e => e.ph === 'X' && main && e.pid === main.pid && e.tid === main.tid).sort((a, b) => a.ts - b.ts || b.dur - a.dur);
        const self = {}, stack = [];
        for (const e of mine) {
          while (stack.length && stack[stack.length - 1].ts + stack[stack.length - 1].dur <= e.ts) stack.pop();
          if (stack.length) stack[stack.length - 1].child += e.dur;
          e.child = 0; stack.push(e);
        }
        for (const e of mine) self[e.name] = (self[e.name] || 0) + (e.dur - e.child) / 1000;
        // How many elements each style recalculation restyled, and why style was invalidated.
        const recalcs = events.filter(e => e.name === 'UpdateLayoutTree' && e.args && e.args.elementCount != null).map(e => e.args.elementCount);
        if (recalcs.length) console.log('TRACE ' + name + ': ' + recalcs.length + ' style recalculations, elements restyled: total ' + recalcs.reduce((a, b) => a + b, 0) + ', largest ' + Math.max(...recalcs));
        const reasons = {};
        events.filter(e => /InvalidationTracking/.test(e.name) && e.args && e.args.data).forEach(e => { const d = e.args.data, key = e.name.replace('InvalidationTracking', '') + ' | ' + (d.reason || '') + ' | ' + (d.nodeName || '') + ' | ' + (d.extraData || '') + ' | ' + (d.selectors ? d.selectors.map(x => x.selector).join(' ; ').slice(0, 160) : '') + (d.invalidationList ? JSON.stringify(d.invalidationList).slice(0, 200) : '') + (d.changedClass || d.changedAttribute || d.changedId || d.changedPseudo || ''); reasons[key] = (reasons[key] || 0) + 1; });
        const why = Object.entries(reasons).sort((a, b) => b[1] - a[1]).slice(0, Number(process.env.REASONS || 0));
        for (const [k, v] of why) console.log('  ' + String(v).padStart(7) + '  ' + k);
        const top = Object.entries(self).sort((a, b) => b[1] - a[1]).slice(0, 25);
        console.log('TRACE ' + name + ' at ' + count + ' chats: self ms by timeline event, for the whole action (' + action.per + ' units)');
        for (const [k, v] of top) console.log('  ' + v.toFixed(1).padStart(9) + '  ' + k);
      }
      if (profiling) {
        const { profile } = await cdp.send('Profiler.stop'); fs.writeFileSync(path.join(OUT, 'scaling-' + name + '-' + count + '.cpuprofile'), JSON.stringify(profile));
        // Self time by function, and the same summed by file.
        const byId = new Map(profile.nodes.map(n => [n.id, n])), self = {}, files = {};
        profile.samples.forEach((id, i) => { const n = byId.get(id), f = n.callFrame, file = (f.url || '').split('/').pop().split('?')[0] || '(native)', key = (f.functionName || '(anonymous)') + ' ' + file + ':' + (f.lineNumber + 1), d = (profile.timeDeltas[i] || 0) / 1000; self[key] = (self[key] || 0) + d; files[file] = (files[file] || 0) + d; });
        console.log('PROFILE ' + name + ' at ' + count + ' chats: self ms by function');
        for (const [k, v] of Object.entries(self).filter(e => !/^\((idle|program)\)/.test(e[0])).sort((a, b) => b[1] - a[1]).slice(0, 22)) console.log('  ' + v.toFixed(1).padStart(9) + '  ' + k);
        console.log('  by file: ' + Object.entries(files).sort((a, b) => b[1] - a[1]).slice(0, 12).map(e => e[0] + ' ' + e[1].toFixed(0)).join(', '));
      }
      const calls = await p.evaluate(n => { const out = {}; __bridgeCalls.slice(n).forEach(c => { out[c[0]] = (out[c[0]] || 0) + 1; }); return out; }, callsBefore);
      const saves = await p.evaluate(() => [__saves, __saveBytes]);
      const ms = k => (after[k] - before[k]) * 1000;
      result.actions[name] = { per: action.per, wall, task: ms('TaskDuration'), script: ms('ScriptDuration'), style: ms('RecalcStyleDuration'), layout: ms('LayoutDuration'), styles: after.RecalcStyleCount - before.RecalcStyleCount, layouts: after.LayoutCount - before.LayoutCount, calls, saves: saves[0] - savesBefore[0], saveBytes: saves[1] - savesBefore[1] };
    }
  } finally { await t.close(); }
  return result;
}

(async () => {
  const results = [];
  for (const count of COUNTS) {
    const r = await run(count, count === COUNTS[COUNTS.length - 1]);
    results.push(r);
    console.log('MEASURE ' + JSON.stringify(r));
  }
  fs.writeFileSync(path.join(OUT, 'scaling.json'), JSON.stringify(results, null, 2));
  // One row for each action: main-thread milliseconds for one unit of it at each count.
  const names = Object.keys(results[results.length - 1].actions);
  const pad = (s, n) => String(s).padEnd(n);
  console.log('\nMain-thread ms for one unit of each action, by open chats (' + MESSAGES + ' messages each, ' + CATALOG + ' closed chats listed)');
  console.log(pad('action', 18) + results.map(r => pad(r.count, 10)).join('') + 'growth');
  for (const name of names) {
    const per = results.map(r => r.actions[name] ? r.actions[name].task / r.actions[name].per : NaN);
    console.log(pad(name, 18) + per.map(v => pad(v.toFixed(2), 10)).join('') + (per[per.length - 1] / per[0]).toFixed(1) + 'x');
  }
  console.log(pad('DOM nodes', 18) + results.map(r => pad(r.nodes, 10)).join(''));
  console.log(pad('listeners', 18) + results.map(r => pad(r.listeners, 10)).join(''));
  console.log(pad('open all (ms)', 18) + results.map(r => pad(r.openAllMs, 10)).join(''));
  console.log(pad('covered windows', 18) + results.map(r => pad(r.covered, 10)).join(''));
})().catch(e => { console.error(e); process.exitCode = 1; });
