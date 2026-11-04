/* Fixed-schema timing storage. No event objects or producer strings are kept. */
(function (host) {
  'use strict';
  const PERIOD = 1000, CAPACITY = 600, RAW_CAPACITY = 200, RAW_RATE = 100, RETENTION = 600000;
  const LIMIT = Number.MAX_SAFE_INTEGER, EXPORT_LIMIT = 4194304;
  const HEADER = 'Dinah timing snapshot. Contains timings, counts and clock anchors only. It records when retained input events, such as key presses, happened and how long they waited. Omits conversation content, identities, paths and commands. Process-wide runtime totals do not identify the cause of an individual stall.';
  const features = ['eventTiming', 'longTasks', 'longAnimationFrames'];
  const metrics = ['inputDelay', 'renderWork', 'renderWait', 'longTask', 'longFrame'];
  const categories = { 'chat output': 'chat', 'provider events': 'other', 'layout follow-up': 'layout_followup', 'unread': 'unread', 'diagnostics': 'diagnostics' };
  const finite = value => typeof value === 'number' && Number.isFinite(value) && value >= 0;
  const measured = value => ({ state: 'measured', value });
  const missing = (state = 'unavailable') => ({ state, value: null });
  function clock(now, utc) { return { utc: utc(), monoMs: now() }; }
  function span(anchor, startMonoMs, durationMs, complete = true) {
    return { anchor: { utc: anchor.utc, monoMs: anchor.monoMs }, startMonoMs, durationMs, complete };
  }
  function copySpan(value) { return span(value.anchor, value.startMonoMs, value.durationMs, value.complete); }
  function stats() { return { count: 0, total: 0, max: null, offset: null, saturated: false }; }
  function addStats(value, duration, offset) {
    if (value.count === LIMIT || value.total > LIMIT - duration) value.saturated = true;
    value.count = Math.min(LIMIT, value.count + 1);
    value.total = Math.min(LIMIT, value.total + duration);
    if (value.max === null || duration > value.max) { value.max = duration; value.offset = offset; }
  }
  function exportStats(value, state) {
    if (state !== 'measured') return { count: missing(state), totalMs: missing(state), maxMs: missing(state), maxCompletedOffsetMs: missing(state), saturated: false };
    return { count: measured(value.count), totalMs: measured(value.total), maxMs: value.max === null ? missing() : measured(value.max), maxCompletedOffsetMs: value.offset === null ? missing() : measured(value.offset), saturated: value.saturated };
  }
  function create(options = {}) {
    const now = options.now || (() => performance.now()), utc = options.utc || (() => new Date().toISOString());
    const slots = new Array(CAPACITY), raw = { input: new Array(RAW_CAPACITY), work: new Array(RAW_CAPACITY), frames: new Array(RAW_CAPACITY) };
    const positions = { input: 0, work: 0, frames: 0 };
    let enabled = false, generation = 0, anchor = clock(now, utc), supported, coverage, hiddenAt = null;
    function clear() {
      slots.fill(undefined);
      Object.keys(raw).forEach(key => { raw[key].fill(undefined); positions[key] = 0; });
      supported = { eventTiming: 'not_supported', longTasks: 'not_supported', longAnimationFrames: 'not_supported' };
      coverage = { retainedSlots: 0, overwrittenSlots: 0, suspendedPeriods: 0, inputRateOmissions: 0, inputWrapOmissions: 0, workRateOmissions: 0, workWrapOmissions: 0, frameRateOmissions: 0, frameWrapOmissions: 0 };
      hiddenAt = null;
    }
    clear();
    function increment(key, count = 1) { coverage[key] = Math.min(LIMIT, coverage[key] + count); }
    function feature(name, state) { if (features.includes(name) && ['measured', 'unavailable', 'not_supported'].includes(state)) supported[name] = state; }
    function slot(completed) {
      const bucket = Math.floor(completed / PERIOD), index = bucket % CAPACITY;
      let value = slots[index];
      if (value && value.bucket > bucket) return null;
      if (!value || value.bucket !== bucket) {
        if (value) increment('overwrittenSlots');
        value = { bucket, partial: bucket * PERIOD < anchor.monoMs, writes: { input: 0, work: 0, frames: 0 } };
        metrics.forEach(key => { value[key] = stats(); }); slots[index] = value;
      }
      return value;
    }
    function retain(source, value, completed, valueSlot) {
      const prefix = source === 'frames' ? 'frame' : source;
      if (valueSlot.writes[source] >= RAW_RATE) { increment(prefix + 'RateOmissions'); return; }
      valueSlot.writes[source]++;
      const index = positions[source] % RAW_CAPACITY, old = raw[source][index];
      if (old && old.time.startMonoMs + old.time.durationMs >= completed - RETENTION) increment(prefix + 'WrapOmissions');
      raw[source][index] = value;
      positions[source] = (index + 1) % RAW_CAPACITY;
    }
    function eligible(start, duration) { return enabled && hiddenAt === null && finite(start) && finite(duration) && start >= anchor.monoMs && finite(start + duration) && Math.floor((start + duration) / PERIOD) >= Math.floor(now() / PERIOD) - CAPACITY + 1; }
    function record(label, duration, wait) {
      if (!enabled || hiddenAt !== null) return;
      if (!finite(duration) || !finite(wait)) return;
      const completed = now(), value = slot(completed), offset = completed % PERIOD;
      addStats(value.renderWork, duration, offset); addStats(value.renderWait, wait, offset);
      retain('work', { time: span(anchor, Math.max(0, completed - duration), duration), category: Object.hasOwn(categories, label) ? categories[label] : 'other', durationMs: measured(duration), waitMs: measured(wait) }, completed, value);
    }
    function input(entry) {
      if (!enabled || hiddenAt !== null) return;
      const start = entry.startTime, duration = entry.duration;
      if (!eligible(start, duration)) return;
      const delay = entry.processingStart - start, processing = entry.processingEnd - entry.processingStart;
      if (!finite(delay) || !finite(processing)) return;
      const completed = start + duration, value = slot(completed);
      if (!value) return;
      addStats(value.inputDelay, delay, completed % PERIOD);
      retain('input', { time: span(anchor, start, duration), category: 'input_delay', delayMs: measured(delay), processingMs: measured(processing), eventDurationMs: measured(duration) }, completed, value);
    }
    function task(entry) {
      if (!enabled || hiddenAt !== null) return;
      if (!eligible(entry.startTime, entry.duration)) return;
      const completed = entry.startTime + entry.duration, value = slot(completed);
      if (!value) return;
      addStats(value.longTask, entry.duration, completed % PERIOD);
      retain('work', { time: span(anchor, entry.startTime, entry.duration), category: 'long_task', durationMs: measured(entry.duration), waitMs: missing() }, completed, value);
    }
    function frame(entry) {
      if (!enabled || hiddenAt !== null) return;
      if (!eligible(entry.startTime, entry.duration)) return;
      const completed = entry.startTime + entry.duration, value = slot(completed);
      if (!value) return;
      addStats(value.longFrame, entry.duration, completed % PERIOD);
      let forced = 0, exposed = Array.isArray(entry.scripts);
      if (exposed) for (const script of entry.scripts) {
        if (!finite(script.forcedStyleAndLayoutDuration)) { exposed = false; break; }
        forced = Math.min(LIMIT, forced + script.forcedStyleAndLayoutDuration);
      }
      retain('frames', { time: span(anchor, entry.startTime, entry.duration), category: 'long_frame', durationMs: measured(entry.duration), blockingMs: finite(entry.blockingDuration) ? measured(entry.blockingDuration) : missing(), forcedStyleLayoutMs: exposed ? measured(forced) : missing() }, completed, value);
    }
    function visibility(hidden) {
      if (!enabled) return;
      const at = now();
      if (hidden && hiddenAt === null) { hiddenAt = at; const value = slots[Math.floor(at / PERIOD) % CAPACITY]; if (value && value.bucket === Math.floor(at / PERIOD)) value.partial = true; }
      else if (!hidden && hiddenAt !== null) { increment('suspendedPeriods', Math.ceil((at - hiddenAt) / PERIOD)); hiddenAt = null; slot(at).partial = true; }
    }
    function snapshot() {
      const at = now(), cutoff = (Math.floor(at / PERIOD) - CAPACITY + 1) * PERIOD;
      const retained = enabled ? slots.filter(value => value && (value.bucket + 1) * PERIOD > cutoff).sort((a, b) => a.bucket - b.bucket) : [];
      const projected = retained.map(value => {
        const start = value.bucket * PERIOD, out = { time: span(anchor, start, Math.max(0, Math.min(PERIOD, at - start)), !value.partial && start + PERIOD <= at && (hiddenAt === null || start + PERIOD <= hiddenAt)) };
        metrics.forEach(key => {
          const state = key === 'inputDelay' ? supported.eventTiming : key === 'longTask' ? supported.longTasks : key === 'longFrame' ? supported.longAnimationFrames : 'measured';
          out[key] = exportStats(value[key], state);
        }); return out;
      });
      const rawCopy = {};
      Object.keys(raw).forEach(key => {
        rawCopy[key] = enabled ? raw[key].filter(value => value && value.time.startMonoMs + value.time.durationMs >= cutoff).sort((a, b) => a.time.startMonoMs - b.time.startMonoMs).map(value => {
          const out = { time: copySpan(value.time), category: value.category };
          Object.keys(value).forEach(field => { if (field !== 'time' && field !== 'category') out[field] = { state: value[field].state, value: value[field].value }; }); return out;
        }) : [];
      });
      return { schemaVersion: 1, generation, enabled, anchor: { ...anchor }, capturedAt: clock(now, utc), features: { ...supported }, coverage: { ...coverage, retainedSlots: projected.length, suspendedPeriods: Math.min(LIMIT, coverage.suspendedPeriods + (hiddenAt === null ? 0 : Math.ceil((at - hiddenAt) / PERIOD))) }, slots: projected, raw: rawCopy };
    }
    function panel() {
      const at = now(), cutoff = at - 60000, bucket = Math.floor(at / PERIOD);
      const out = { items: [], features: { ...supported }, inputPeak: 0, taskCount: 0, taskPeak: 0, renderPeak: 0, waitPeak: 0, framePeak: 0 };
      for (let i = 0; i <= 60; i++) {
        const value = slots[(bucket - i + CAPACITY) % CAPACITY];
        if (!value || value.bucket !== bucket - i) continue;
        out.inputPeak = Math.max(out.inputPeak, value.inputDelay.max || 0);
        out.taskCount = Math.min(LIMIT, out.taskCount + value.longTask.count);
        out.taskPeak = Math.max(out.taskPeak, value.longTask.max || 0);
        out.renderPeak = Math.max(out.renderPeak, value.renderWork.max || 0);
        out.waitPeak = Math.max(out.waitPeak, value.renderWait.max || 0);
        out.framePeak = Math.max(out.framePeak, value.longFrame.max || 0);
      }
      const words = { chat: 'Chat rendering', layout_followup: 'Layout follow-up', unread: 'Unread bookkeeping', diagnostics: 'Diagnostics', long_task: 'Main-thread task', other: 'Other work' };
      for (const value of raw.input) if (value && value.time.startMonoMs + value.time.durationMs >= cutoff) out.items.push({ at: value.time.startMonoMs + value.time.durationMs, kind: 'input', label: 'Input processing delay', duration: value.delayMs.value, wait: 0 });
      for (const value of raw.work) if (value && value.time.startMonoMs + value.time.durationMs >= cutoff && (value.category === 'long_task' || value.durationMs.value >= 6 || value.waitMs.value >= 16)) out.items.push({ at: value.time.startMonoMs + value.time.durationMs, kind: value.category === 'long_task' ? 'stall' : 'render', label: words[value.category], duration: value.durationMs.value, wait: value.waitMs.value || 0 });
      for (const value of raw.frames) if (value && value.time.startMonoMs + value.time.durationMs >= cutoff) out.items.push({ at: value.time.startMonoMs + value.time.durationMs, kind: 'frame', label: 'Long animation frame', duration: value.durationMs.value, wait: 0 });
      out.items.sort((a, b) => a.at - b.at); out.items = out.items.slice(-200);
      return out;
    }
    return {
      begin(value) { clear(); generation = value; anchor = clock(now, utc); enabled = true; },
      end(value) { enabled = false; generation = value; clear(); anchor = clock(now, utc); },
      feature, record, input, task, frame, visibility, snapshot, panel,
    };
  }

  const enumeration = (...values) => ({ enum: values }), array = (type, max) => ({ array: type, max });
  const state = enumeration('measured', 'unavailable', 'not_supported');
  const schemas = {
    Clock: { utc: 'utc', monoMs: 'number' },
    Span: { anchor: 'Clock', startMonoMs: 'number', durationMs: 'number', complete: 'boolean' },
    DurationStats: { count: 'MeasuredNumber', totalMs: 'MeasuredNumber', maxMs: 'MeasuredNumber', maxCompletedOffsetMs: 'MeasuredNumber', saturated: 'boolean' },
    RecordingState: { enabled: 'boolean', generation: 'integer', anchor: 'Clock', state: enumeration('on', 'off') },
    BackendSnapshot: { schemaVersion: { literal: 1 }, recording: 'RecordingState', capturedAt: 'Clock', coverage: 'BackendCoverage', slots: array('BackendSlot', CAPACITY) },
    BackendCoverage: { retainedSlots: 'integer', overwrittenSlots: 'integer', missedPeriods: 'integer', disabledObservations: 'MeasuredNumber' },
    BackendSlot: { time: 'Span', categories: 'BackendCategories', queues: 'QueueStats', runtime: 'RuntimeInterval' },
    BackendCategories: Object.fromEntries(['agent_text', 'agent_tool', 'chat_state', 'team_state', 'bus_change', 'other'].map(key => [key, 'CategoryStats'])),
    CategoryStats: { appEmit: 'DurationStats', bridgeEnqueue: 'DurationStats' },
    QueueStats: { subscriberPeak: 'MeasuredNumber', depthPeak: 'MeasuredNumber', overflowDisconnects: 'MeasuredNumber' },
    RuntimeInterval: { scope: enumeration('process_wide_interval'), time: 'Span', gcPauseCount: 'MeasuredNumber', gcPauseMaxBucketLowerMs: 'MeasuredNumber', gcPauseMaxBucketUpperMs: 'MeasuredNumber', gcPauseUpperOpen: 'boolean', mutexWaitMs: 'MeasuredNumber' },
    BrowserSnapshot: { schemaVersion: { literal: 1 }, generation: 'integer', enabled: 'boolean', anchor: 'Clock', capturedAt: 'Clock', features: 'BrowserFeatures', coverage: 'BrowserCoverage', slots: array('BrowserSlot', CAPACITY), raw: 'BrowserRaw' },
    BrowserFeatures: { eventTiming: state, longTasks: state, longAnimationFrames: state },
    BrowserCoverage: Object.fromEntries(['retainedSlots', 'overwrittenSlots', 'suspendedPeriods', 'inputRateOmissions', 'inputWrapOmissions', 'workRateOmissions', 'workWrapOmissions', 'frameRateOmissions', 'frameWrapOmissions'].map(key => [key, 'integer'])),
    BrowserSlot: { time: 'Span', inputDelay: 'DurationStats', renderWork: 'DurationStats', renderWait: 'DurationStats', longTask: 'DurationStats', longFrame: 'DurationStats' },
    BrowserRaw: { input: array('InputRecord', RAW_CAPACITY), work: array('WorkRecord', RAW_CAPACITY), frames: array('FrameRecord', RAW_CAPACITY) },
    InputRecord: { time: 'Span', category: enumeration('input_delay'), delayMs: 'MeasuredNumber', processingMs: 'MeasuredNumber', eventDurationMs: 'MeasuredNumber' },
    WorkRecord: { time: 'Span', category: enumeration('chat', 'layout_followup', 'unread', 'diagnostics', 'long_task', 'other'), durationMs: 'MeasuredNumber', waitMs: 'MeasuredNumber' },
    FrameRecord: { time: 'Span', category: enumeration('long_frame'), durationMs: 'MeasuredNumber', blockingMs: 'MeasuredNumber', forcedStyleLayoutMs: 'MeasuredNumber' },
    Export: { schemaVersion: { literal: 1 }, copiedAt: 'Clock', copyRequest: 'Span', browser: 'BrowserSnapshot', backend: 'BackendSnapshot' },
  };
  function valid(type, value) {
    if (type === 'number') return finite(value);
    if (type === 'integer') return Number.isSafeInteger(value) && value >= 0;
    if (type === 'boolean') return typeof value === 'boolean';
    if (type === 'utc') return typeof value === 'string' && /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(value) && Number.isFinite(Date.parse(value));
    if (type === 'MeasuredNumber') return value && typeof value === 'object' && !Array.isArray(value) && Object.keys(value).length === 2 && Object.hasOwn(value, 'state') && Object.hasOwn(value, 'value') && valid(state, value.state) && (value.state === 'measured' ? finite(value.value) : value.value === null);
    if (typeof type === 'string') {
      if (!schemas[type] || !valid(schemas[type], value)) return false;
      if (type === 'RecordingState') return value.state === (value.enabled ? 'on' : 'off');
      if (type === 'DurationStats') {
        if (value.count.state === 'measured' && !Number.isSafeInteger(value.count.value)) return false;
        if (value.maxCompletedOffsetMs.state === 'measured' && value.maxCompletedOffsetMs.value >= PERIOD) return false;
        if (value.count.state === 'measured' && value.count.value === 0) return value.totalMs.state === 'measured' && value.totalMs.value === 0 && value.maxMs.state === 'unavailable' && value.maxCompletedOffsetMs.state === 'unavailable';
      }
      if (type === 'BrowserSnapshot' || type === 'BackendSnapshot') return value.coverage.retainedSlots === value.slots.length;
      return true;
    }
    if (type.enum) return type.enum.includes(value);
    if (Object.hasOwn(type, 'literal')) return value === type.literal;
    if (type.array) return Array.isArray(value) && value.length <= type.max && value.every(item => valid(type.array, item));
    return value && typeof value === 'object' && !Array.isArray(value) && Object.keys(value).length === Object.keys(type).length && Object.keys(type).every(key => Object.hasOwn(value, key) && valid(type[key], value[key]));
  }
  function validate(type, value) { if (!valid(type, value)) throw new Error('Invalid timing snapshot.'); return value; }
  function serialize(value) {
    validate('Export', value);
    const text = HEADER + '\n' + JSON.stringify(value);
    if (new TextEncoder().encode(text).length > EXPORT_LIMIT) throw new Error('The timing snapshot is too large to copy.');
    return text;
  }
  const api = { create, validate, serialize, HEADER, PERIOD, CAPACITY, RAW_CAPACITY, RAW_RATE, RETENTION, EXPORT_LIMIT };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else host.dinahTimingSnapshot = api;
})(typeof window === 'undefined' ? globalThis : window);
