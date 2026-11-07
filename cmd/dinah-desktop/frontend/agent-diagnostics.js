/* Bounded agent diagnostics. Retain scalar measurements, never DOM or event objects. */
(function () {
  'use strict';
  var capacity = 200, events = [], observers = [], timer = 0, expiry = 0, visibilityListener = null;
  var id = '', state = 'idle', deadline = 0, started = 0, overwritten = 0, omitted = 0, scriptsOmitted = 0;
  var rateSecond = -1, rateCount = {}, support = { input: 'unavailable', longTask: 'unavailable', longFrame: 'unavailable' };
  var labels = { 'chat output': 'chat', 'layout follow-up': 'layout', 'unread': 'unread', 'diagnostics': 'diagnostics', 'provider events': 'provider' };
  var shipped = new Map();
  document.querySelectorAll('script[src]').forEach(function (script) {
    var url = new URL(script.src, location.href);
    var base = new URL('.', location.href);
    if (url.origin === base.origin && url.pathname.startsWith(base.pathname)) {
      var path = url.pathname.slice(base.pathname.length);
      if (/^(?:[a-z0-9_-]+\/)*[a-z0-9_.-]+\.js$/i.test(path) && path.length <= 160) shipped.set(url.origin + url.pathname, path);
    }
  });
  function number(value) { return Number.isFinite(value) && value >= 0 ? value : 0; }
  function active() { if (state === 'running' && Date.now() >= deadline) finish('complete'); return state === 'running'; }
  function status() {
    var el = document.getElementById('agent-diagnostics-status'), stop = document.getElementById('agent-diagnostics-stop');
    if (el) el.textContent = state === 'running' ? 'Agent diagnostic capture is recording until ' + new Date(deadline).toLocaleTimeString() + '.' : 'Agent diagnostic capture: ' + state + '.';
    if (stop) stop.disabled = state !== 'running';
  }
  function finish(next) {
    observers.forEach(function (observer) { observer.disconnect(); }); observers = [];
    clearInterval(timer); clearTimeout(expiry); timer = expiry = 0;
    if (visibilityListener) document.removeEventListener('visibilitychange', visibilityListener);
    visibilityListener = null;
    state = next; status();
  }
  function append(kind, at, duration, wait, label, blocking, scripts) {
    if (!active()) return;
    var second = Math.floor(performance.now() / 1000);
    if (second !== rateSecond) { rateSecond = second; rateCount = {}; }
    if ((rateCount[kind] = (rateCount[kind] || 0) + 1) > 100) { omitted++; return; }
    if (events.length === capacity) { events.shift(); overwritten++; }
    events.push({ visible: !document.hidden, kind: kind, atMs: number(at), durationMs: number(duration), waitMs: number(wait), label: label || 'other', blockingMs: blocking == null ? null : number(blocking), scripts: scripts || [] });
  }
  function scriptSource(value) {
    try { var url = new URL(value); return shipped.get(url.origin + url.pathname) || 'unknown'; }
    catch (_) { return 'unknown'; }
  }
  function observe(type, feature, consume, extra) {
    support[feature] = 'unavailable';
    if (!window.PerformanceObserver || !PerformanceObserver.supportedEntryTypes.includes(type)) return;
    try {
      var observer = new PerformanceObserver(function (list) {
        if (!active()) return;
        list.getEntries().forEach(function (entry) { if (entry.startTime >= started) consume(entry); });
      });
      observer.observe(Object.assign({ type: type }, extra)); observers.push(observer); support[feature] = 'measured';
    } catch (_) { /* Feature remains unavailable. */ }
  }
  function start(args) {
    if (active()) throw new Error('A diagnostic capture is already running.');
    if (!Number.isInteger(args.seconds) || args.seconds < 1 || args.seconds > 300 || !/^[a-zA-Z0-9-]{1,128}$/.test(args.captureId)) throw new Error('Invalid capture request.');
    finish('idle'); events = []; overwritten = omitted = scriptsOmitted = 0; rateCount = {}; rateSecond = -1;
    id = args.captureId; started = performance.now(); deadline = Date.now() + args.seconds * 1000; state = 'running';
    observe('event', 'input', function (e) { append('input', e.startTime, e.duration, e.processingStart - e.startTime); }, { durationThreshold: 16 });
    observe('longtask', 'longTask', function (e) { append('long_task', e.startTime, e.duration); });
    observe('long-animation-frame', 'longFrame', function (e) {
      var entries = e.scripts || []; scriptsOmitted += Math.max(0, entries.length - 8);
      append('long_frame', e.startTime, e.duration, 0, 'other', e.blockingDuration, entries.slice(0, 8).map(function (s) {
        return { source: scriptSource(s.sourceURL), position: Number.isInteger(s.sourceCharPosition) && s.sourceCharPosition >= 0 ? s.sourceCharPosition : null, durationMs: number(s.duration), forcedLayoutMs: Number.isFinite(s.forcedStyleAndLayoutDuration) ? number(s.forcedStyleAndLayoutDuration) : null };
      }));
    });
    var expected = performance.now() + 1000;
    visibilityListener = function () { expected = performance.now() + 1000; };
    document.addEventListener('visibilitychange', visibilityListener);
    timer = setInterval(function () {
      var now = performance.now();
      if (!document.hidden) append('event_loop', now, Math.max(0, now - expected));
      expected = now + 1000;
    }, 1000);
    expiry = setTimeout(function () { finish('complete'); }, args.seconds * 1000);
    status(); return snapshot();
  }
  function snapshot() {
    active();
    var before = performance.now();
    var result = {
      anchor: { utc: new Date().toISOString(), monoMs: before },
      capture: { id: id, state: state, startedMonoMs: started, deadlineUtcMs: deadline },
      visible: !document.hidden, features: Object.assign({}, support),
      coverage: { retained: events.length, overwritten: overwritten, rateOmissions: omitted, scriptOmissions: scriptsOmitted },
      counts: { elements: document.getElementsByTagName('*').length, windows: document.querySelectorAll('.win').length, transcriptRows: document.querySelectorAll('.activity-row').length, toolRows: document.querySelectorAll('.tool-row').length },
      scheduler: window.dinahRender.diagnostics(),
      rendererHeap: 'requires_profiler', events: events.slice(), collectionMs: 0
    };
    result.collectionMs = performance.now() - before;
    return result;
  }
  function request(args) {
    if (!Number.isFinite(args.expiresAt) || Date.now() >= args.expiresAt) throw new Error('The diagnostic request expired before the page handled it.');
    if (args.operation === 'start') return start(args);
    if (args.operation === 'stop') {
      active();
      if (!id || args.captureId !== id) throw new Error('The capture ID does not match.');
      if (state === 'running') finish('stopped');
      return snapshot();
    }
    if (args.operation === 'snapshot') return snapshot();
    throw new Error('Unknown diagnostic operation.');
  }
  var stop = document.getElementById('agent-diagnostics-stop');
  if (stop) stop.addEventListener('click', function () { if (active()) finish('stopped'); });
  window.dinahAgentDiagnostics = {
    request: request,
    record: function (label, duration, wait) { if (active() && (duration >= 8 || wait >= 16)) append('render', performance.now() - number(duration), duration, wait, labels[label] || 'other'); }
  };
  status();
})();
