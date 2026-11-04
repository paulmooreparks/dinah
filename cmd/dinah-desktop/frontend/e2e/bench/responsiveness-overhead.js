// Typing lag under agent load (docs/plans/recurring-chat-faults.md, Typing
// Lag). A measurement, not a pass-or-fail suite, so run.ps1 leaves this
// folder out. For 0, 1, 2 and 4 synthetic agents, each a chat open in its
// own window with a long saved history, it streams text, tool calls and
// group traffic at the rates agreed with Dinah Engine (bus message 4834),
// types into a separate idle chat in front, and reports each keystroke's
// delay, the long tasks, and the render scheduler's jobs by label.
//
//   node bench/typing-lag.js [outFolder] [agents,...]
const { launch } = require('../lib/page');
const fs = require('fs');
const path = require('path');
const OUT = process.argv[2] || require('os').tmpdir();
const CASES = (process.argv[3] || '0,1,2,4').split(',').map(Number);
const RECORDING = process.env.TIMING_RECORDING === '1';
const SECONDS = Number(process.env.LAG_SECONDS || 30);

async function run(agents) {
  const t = await launch({
    viewport: { width: 1600, height: 1000 },
    args: { agents, recording: RECORDING },
    app: ({ agents, recording }) => {
      const ids = Array.from({ length: agents }, (_, i) => 'codex:agent' + i);
      const member = id => ({ id: 'window-' + id, chat: id, title: 'Agent ' + id.slice(-1), workspace: 'fixture', address: 'Agent', life: 'open', window: true, exec: { state: 'working', why: 'Streaming' } });
      window.__groupMessages = [];
      return {
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace', 'dinah-responsiveness': recording } }), SaveUIState: async () => {},
        SetResponsivenessRecording: async enabled => ({ enabled, generation: enabled ? 1 : 0, anchor: { utc: new Date().toISOString(), monoMs: performance.now() }, state: enabled ? 'on' : 'off' }),
        OpenSessionAt: async (ws, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: ws, open: true, working: ids.includes(id), queued: [] }),
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: agents ? [{ name: 'lag-team', title: 'Lag Team', lastAt: '', unread: 0, members: ids.map(member), counts: { top: 'working', idle: 0, working: agents, waiting: 0, stuck: 0, closed: 0, unknown: 0 } }] : [], workspaces: [], links: [], defaultHops: 3, defaultMax: 3, noHopLimit: -1 }),
        BusGroupMessages: async () => window.__groupMessages.slice(), BusMarkGroupRead: async () => {},
      };
    },
    init: () => {
      window.__eventTimings = []; window.__longFrames = []; window.__measurementSince = Infinity;
      if (PerformanceObserver.supportedEntryTypes.includes('event')) new PerformanceObserver(list => list.getEntries().filter(e => e.startTime >= __measurementSince).forEach(e => __eventTimings.push({ name: e.name, duration: e.duration, delay: e.processingStart - e.startTime, processing: e.processingEnd - e.processingStart }))).observe({ type: 'event', durationThreshold: 16 });
      if (PerformanceObserver.supportedEntryTypes.includes('long-animation-frame')) new PerformanceObserver(list => list.getEntries().filter(e => e.startTime >= __measurementSince).forEach(e => __longFrames.push({ duration: e.duration, blocking: e.blockingDuration }))).observe({ type: 'long-animation-frame' });
    },
    ready: () => window.dinahWorkbench && window.dinahBus,
  });
  const { page: p } = t;
  try {
    // The agents' chats, each with its history replayed, then the idle chat
    // the person types in, in front.
    await p.evaluate(async agents => {
      const ws = 'C:/Fixture';
      for (let a = 0; a < agents; a++) {
        const id = 'codex:agent' + a;
        await dinahWorkbench.open({ kind: 'chat-session', workspace: ws, path: id });
        const updates = [];
        for (let i = 0; i < 1500; i++) {
          updates.push({ sessionUpdate: i % 2 ? 'agent_message_chunk' : 'user_message_chunk', content: { type: 'text', text: 'Saved row ' + i + ' ' + 'lorem ipsum dolor sit amet '.repeat(14) + '\n\n' } });
          if (i % 25 === 0) {
            const tc = 'h' + a + '-' + i;
            updates.push({ sessionUpdate: 'tool_call', toolCallId: tc, title: 'Ran tests', kind: 'execute', status: 'in_progress' });
            updates.push({ sessionUpdate: 'tool_call_update', toolCallId: tc, status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'output line\n'.repeat(170) } }] });
          }
        }
        __emit('agent:updates', { sessionId: id, updates });
      }
      if (agents) await dinahBus.openGroup('lag-team');
      await dinahWorkbench.open({ kind: 'chat-session', workspace: ws, path: 'codex:typist' });
    }, agents);
    await p.locator('.win[data-chat-current] [data-view-id=prompt]').waitFor();
    await p.waitForTimeout(3000);

    // Measurement: every render job the scheduler reports, long tasks, and
    // each keystroke's delay to its handler and to the next frame.
    await p.evaluate(() => {
      window.__jobs = []; window.__tasks = []; window.__keys = [];
      window.__measurementSince = performance.now();
      const recorded = window.dinahResponsiveness.record;
      window.dinahResponsiveness.record = (kind, label, duration, wait) => { recorded(kind, label, duration, wait); __jobs.push({ kind, label, duration, wait }); };
      new PerformanceObserver(list => list.getEntries().forEach(e => __tasks.push(e.duration))).observe({ type: 'longtask' });
      const box = document.querySelector('.win[data-chat-current] [data-view-id=prompt]');
      box.addEventListener('keydown', e => {
        const handled = performance.now(), queued = handled - e.timeStamp;
        requestAnimationFrame(() => __keys.push({ queued, toFrame: performance.now() - e.timeStamp }));
      });
    });

    // The load, at the agreed rates, until the end of the run.
    await p.evaluate(({ agents, seconds }) => {
      const ids = Array.from({ length: agents }, (_, i) => 'codex:agent' + i);
      const until = performance.now() + seconds * 1000;
      const words = 'the quick brown fox jumps over the lazy dog and keeps on running ';
      ids.forEach((id, a) => {
        let n = 0, tools = 0;
        const text = setInterval(() => {
          if (performance.now() > until) return clearInterval(text);
          n++;
          __emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: words.slice(0, 30 + (n * 7) % 30) + (n % 15 ? '' : '\n\n') } } });
        }, 40);
        const tool = setInterval(() => {
          if (performance.now() > until) return clearInterval(tool);
          const tc = 'live' + a + '-' + (++tools);
          __emit('agent:update', { sessionId: id, update: { sessionUpdate: 'tool_call', toolCallId: tc, title: 'Run step ' + tools, kind: 'execute', status: 'in_progress' } });
          for (let k = 1; k <= 8; k++) setTimeout(() => __emit('agent:update', { sessionId: id, update: { sessionUpdate: 'tool_call_update', toolCallId: tc, content: [{ type: 'content', content: { type: 'text', text: 'x'.repeat(511) + '\n' } }] } }), k * 100);
          setTimeout(() => __emit('agent:update', { sessionId: id, update: { sessionUpdate: 'tool_call_update', toolCallId: tc, status: 'completed' } }), 900);
        }, 3000);
      });
      if (agents) {
        let g = 0;
        const group = setInterval(() => {
          if (performance.now() > until) return clearInterval(group);
          const from = ids[g % agents];
          window.__groupMessages.push({ id: 1000 + g, from: 'Agent ' + from.slice(-1), fromSession: 'window-' + from, kind: 'text', text: 'Progress note ' + g + ': ' + words.repeat(3), mentions: [], attachments: [], createdAt: new Date().toISOString(), room: 'lag-team', roomTitle: 'Lag Team' });
          g++;
          __emit('bus:changed', { counter: g });
        }, 5000);
      }
    }, { agents, seconds: SECONDS });

    // The browser's own layout and style counts over ten seconds of the load,
    // with the frames drawn in that time.
    const metricsCdp = await t.context.newCDPSession(p);
    await metricsCdp.send('Performance.enable');
    const metric = async () => Object.fromEntries((await metricsCdp.send('Performance.getMetrics')).metrics.map(m => [m.name, m.value]));
    await p.waitForTimeout(3000);
    // No frame-count sampler: it would add idle frame requests to the control.
    const m0 = await metric();
    await p.waitForTimeout(10000);
    const m1 = await metric();
    const frames = null;
    const layout = { frames, layouts: m1.LayoutCount - m0.LayoutCount, layoutMs: Math.round((m1.LayoutDuration - m0.LayoutDuration) * 1000), styles: m1.RecalcStyleCount - m0.RecalcStyleCount, styleMs: Math.round((m1.RecalcStyleDuration - m0.RecalcStyleDuration) * 1000), scriptMs: Math.round((m1.ScriptDuration - m0.ScriptDuration) * 1000), nodes: m1.Nodes };
    // With LAG_MUTATIONS set, five seconds of every DOM change outside the
    // chats' message logs, counted by where it happened and what changed.
    let mutations = null;
    if (process.env.LAG_MUTATIONS) {
      mutations = await p.evaluate(() => new Promise(resolve => {
        const counts = new Map();
        const where = el => { el = el && el.nodeType === 1 ? el : el && el.parentElement; if (!el) return '?'; const win = el.closest('.win'); const area = win ? 'win:' + (win.dataset.winKind || '') : el.closest('.topbar') ? 'topbar' : el.closest('.win-bar') ? 'taskbar' : el.closest('#sidebar') ? 'sidebar' : 'page'; return area + ' ' + el.tagName.toLowerCase() + (el.className && typeof el.className === 'string' ? '.' + el.className.trim().split(/\s+/).slice(0, 3).join('.') : ''); };
        const obs = new MutationObserver(records => records.forEach(r => {
          const target = r.target.nodeType === 1 ? r.target : r.target.parentElement;
          if (target && target.closest('[data-view-id="logs"], .msg-text, .activity-row')) return;
          const k = where(r.target) + ' ' + r.type + (r.attributeName ? ':' + r.attributeName : '');
          counts.set(k, (counts.get(k) || 0) + 1);
        }));
        obs.observe(document.documentElement, { subtree: true, childList: true, attributes: true, characterData: true });
        setTimeout(() => { obs.disconnect(); resolve(Array.from(counts.entries()).sort((a, b) => b[1] - a[1]).slice(0, 25).map(([k, v]) => v + ' x ' + k)); }, 5000);
      }));
    }
    // With LAG_TRACE set, two seconds of the browser's trace with style
    // invalidation tracking: how many elements each style recalculation
    // touched, and what invalidated them.
    let trace = null;
    if (process.env.LAG_TRACE) {
      const cdp = await t.context.newCDPSession(p);
      const events = [];
      cdp.on('Tracing.dataCollected', d => events.push(...d.value));
      const done = new Promise(r => cdp.once('Tracing.tracingComplete', r));
      await cdp.send('Tracing.start', { transferMode: 'ReportEvents', traceConfig: { includedCategories: ['devtools.timeline', 'disabled-by-default-devtools.timeline', 'disabled-by-default-devtools.timeline.invalidationTracking'] } });
      await p.waitForTimeout(2000);
      await cdp.send('Tracing.end');
      await done;
      // The whole trace, for Chrome's or Edge's performance panel.
      fs.writeFileSync(path.join(OUT, 'typing-lag-trace-' + agents + '.json'), JSON.stringify({ traceEvents: events, metadata: { browser: t.browser.version(), agents } }));
      const recalcs = events.filter(e => e.name === 'UpdateLayoutTree' && e.ph === 'X');
      const counts = recalcs.map(e => (e.args && e.args.elementCount) || 0);
      const why = new Map();
      events.filter(e => /StyleRecalcInvalidationTracking|StyleInvalidatorInvalidationTracking|ScheduleStyleInvalidationTracking/.test(e.name)).forEach(e => {
        const d = (e.args && e.args.data) || {};
        const k = e.name.replace('InvalidationTracking', '') + ' | ' + (d.reason || d.invalidationSet && 'set' || '') + ' | ' + (d.nodeName || '') + ' | ' + (d.changedClass || d.changedAttribute || d.changedId || d.changedPseudo || '') + ' | ' + ((d.selectors || []).map(s => s.selector).slice(0, 2).join(' , ') || '');
        why.set(k, (why.get(k) || 0) + 1);
      });
      trace = { recalcs: recalcs.length, elementsMedian: counts.sort((a, b) => a - b)[Math.floor(counts.length / 2)] || 0, elementsMax: Math.max(0, ...counts), msTotal: Math.round(recalcs.reduce((a, e) => a + (e.dur || 0), 0) / 1000),
        why: Array.from(why.entries()).sort((a, b) => b[1] - a[1]).slice(0, 25).map(([k, v]) => v + ' x ' + k) };
    }
    // With LAG_PROFILE set, a CPU profile of ten seconds of the load, summed
    // by function, before the typing.
    let profile = null;
    if (process.env.LAG_PROFILE) {
      await p.waitForTimeout(5000);
      const cdp = await t.context.newCDPSession(p);
      await cdp.send('Profiler.enable');
      await cdp.send('Profiler.setSamplingInterval', { interval: 200 });
      await cdp.send('Profiler.start');
      await p.waitForTimeout(10000);
      const { profile: prof } = await cdp.send('Profiler.stop');
      const byId = new Map(prof.nodes.map(n => [n.id, n]));
      const self = new Map();
      const dt = prof.timeDeltas || [];
      (prof.samples || []).forEach((id, i) => { const n = byId.get(id); const f = n.callFrame; const k = (f.functionName || '(anonymous)') + ' ' + path.basename(f.url || '') + ':' + (f.lineNumber + 1); self.set(k, (self.get(k) || 0) + (dt[i] || 0) / 1000); });
      profile = Array.from(self.entries()).sort((a, b) => b[1] - a[1]).slice(0, 25).map(([fn, ms]) => fn + ' ' + Math.round(ms) + ' ms');
      // Line-level ticks for the transcript's functions, to see which reads
      // force layout.
      const lines = new Map();
      prof.nodes.forEach(n => { if (!/transcript\.js|chat-applet\.js/.test(n.callFrame.url || '')) return; (n.positionTicks || []).forEach(pt => { const k = path.basename(n.callFrame.url) + ':' + pt.line; lines.set(k, (lines.get(k) || 0) + pt.ticks); }); });
      profile.push('lines: ' + Array.from(lines.entries()).sort((a, b) => b[1] - a[1]).slice(0, 15).map(([k, v]) => k + '=' + v).join(' '));
    }
    // Type during the last ten seconds.
    await p.waitForTimeout(Math.max(0, (SECONDS - 23 - (profile ? 15 : 0)) * 1000));
    const box = p.locator('.win[data-chat-current] [data-view-id=prompt]');
    await box.click();
    await box.pressSequentially('The quick brown fox jumps over the lazy d', { delay: 50 });
    await p.waitForTimeout(1500);

    const r = await p.evaluate(() => ({ jobs: __jobs, tasks: __tasks, keys: __keys, eventTiming: __eventTimings, longFrames: __longFrames }));
    const pct = (xs, q) => { if (!xs.length) return 0; const s = xs.slice().sort((a, b) => a - b); return Math.round(s[Math.min(s.length - 1, Math.floor(q * s.length))]); };
    const byLabel = {};
    r.jobs.forEach(j => { const b = byLabel[j.label] || (byLabel[j.label] = { runs: 0, total: 0, max: 0, waitMax: 0 }); b.runs++; b.total += j.duration; b.max = Math.max(b.max, j.duration); b.waitMax = Math.max(b.waitMax, j.wait || 0); });
    const labels = Object.entries(byLabel).sort((a, b) => b[1].total - a[1].total).slice(0, 12)
      .map(([label, b]) => ({ label, runs: b.runs, totalMs: Math.round(b.total), maxMs: Math.round(b.max), waitMaxMs: Math.round(b.waitMax) }));
    return {
      agents, recording: RECORDING, eventTiming: r.eventTiming, longAnimationFrames: r.longFrames,
      pageErrors: t.errors.slice(),
      browser: t.browser.version(),
      keys: { count: r.keys.length, queuedP50: pct(r.keys.map(k => k.queued), 0.5), queuedP95: pct(r.keys.map(k => k.queued), 0.95), toFrameP50: pct(r.keys.map(k => k.toFrame), 0.5), toFrameP95: pct(r.keys.map(k => k.toFrame), 0.95), toFrameMax: pct(r.keys.map(k => k.toFrame), 1) },
      longTasks: { count: r.tasks.length, totalMs: Math.round(r.tasks.reduce((a, b) => a + b, 0)), maxMs: Math.round(Math.max(0, ...r.tasks)) },
      renderTotalMs: Math.round(r.jobs.reduce((a, j) => a + j.duration, 0)),
      labels,
      layout,
      trace,
      mutations,
      profile,
    };
  } finally {
    await t.browser.close().catch(() => {});
  }
}

(async () => {
  const results = [];
  for (const n of CASES) {
    const r = await run(n);
    results.push(r);
    console.log(JSON.stringify(r));
  }
  fs.writeFileSync(path.join(OUT, 'responsiveness-overhead.json'), JSON.stringify(results, null, 2));
})().catch(e => { console.error(e); process.exitCode = 1; });
