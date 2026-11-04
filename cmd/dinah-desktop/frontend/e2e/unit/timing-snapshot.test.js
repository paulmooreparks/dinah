const assert = require('node:assert/strict');
const timing = require('../../timing-snapshot');
let now = 1000, reads = 0;
const clock = () => { reads++; return now; };
const utc = () => '2026-11-04T03:00:00.000Z';
const store = timing.create({ now: clock, utc });
const canary = 'PRIVATE-CANARY-C:/Workspace/person prompt https://private.invalid/';
store.begin(1);
store.feature('eventTiming', 'measured'); store.feature('longTasks', 'measured'); store.feature('longAnimationFrames', 'measured');
now = 1200;
store.input({ startTime: 1100, duration: 80, processingStart: 1120, processingEnd: 1123, target: canary, name: canary });
store.record(canary, 8, 17);
store.frame({ startTime: 1100, duration: 60, blockingDuration: 10, scripts: [{ forcedStyleAndLayoutDuration: 12, sourceURL: canary, sourceFunctionName: canary }] });
store.task({ startTime: 1100, duration: 50, attribution: canary });
let result = timing.validate('BrowserSnapshot', store.snapshot());
assert.equal(JSON.stringify(result).includes(canary), false);
assert.equal(result.raw.input[0].eventDurationMs.value, 80);
assert.equal(result.raw.input[0].delayMs.value, 20);
assert.equal(result.raw.input[0].processingMs.value, 3);
assert.equal(result.raw.frames[0].forcedStyleLayoutMs.value, 12);
assert.equal(result.raw.work.find(x => x.category === 'other').category, 'other');
assert.equal(result.slots[0].inputDelay.maxCompletedOffsetMs.value, 180);
store.frame({ startTime: 1100, duration: 70, scripts: [{ sourceURL: canary }] });
assert.equal(store.snapshot().raw.frames[1].forcedStyleLayoutMs.state, 'unavailable');

// A flood preserves every aggregate observation, but raw storage and its rate stay bounded.
store.begin(2); now = 1500;
for (let i = 0; i < 20000; i++) store.record(canary + i, i, 0);
result = timing.validate('BrowserSnapshot', store.snapshot());
assert.equal(result.raw.work.length, 100);
assert.equal(result.coverage.workRateOmissions, 19900);
assert.equal(result.slots[0].renderWork.count.value, 20000);
assert.equal(result.slots[0].renderWork.maxMs.value, 19999);
assert.equal(result.slots[0].renderWork.maxCompletedOffsetMs.value, 500);

// Thousands of wraps keep the retained shape flat and discard old labels and timings.
let sizes = [];
for (let second = 2; second < 2002; second++) {
  now = second * 1000 + 500;
  for (let i = 0; i < 110; i++) store.record(canary + second, 8, 0);
  if (second === 1000 || second === 2000) {
    result = timing.validate('BrowserSnapshot', store.snapshot());
    sizes.push(JSON.stringify(result).length);
    assert.equal(result.slots.length, 600); assert.equal(result.raw.work.length, 200);
  }
}
assert.ok(Math.abs(sizes[1] - sizes[0]) < 20000, sizes.join('/'));
assert.ok(result.coverage.workWrapOmissions > 0);
assert.ok(result.coverage.overwrittenSlots > 0);
assert.equal(JSON.stringify(result).includes(canary), false);

// Disabled hot paths read no clock and retain nothing, including after a flood.
store.end(3); const before = reads;
for (let i = 0; i < 20000; i++) { store.record(canary, 8, 0); store.input({}); store.task({}); store.frame({}); store.visibility(true); }
assert.equal(reads, before);
result = timing.validate('BrowserSnapshot', store.snapshot());
assert.deepEqual(result.slots, []); assert.deepEqual(result.raw, { input: [], work: [], frames: [] });

// A suspension marks partial coverage, while completed earlier slots remain complete.
now = 3000000; store.begin(4); store.feature('eventTiming', 'measured');
now += 100; store.record('chat output', 7, 0);
now += 1000; store.record('chat output', 7, 0);
store.visibility(true); now += 3000;
result = store.snapshot();
assert.equal(result.slots[0].time.complete, true);
assert.equal(result.slots[1].time.complete, false);
assert.equal(result.coverage.suspendedPeriods, 3);
store.visibility(false); store.record('chat output', 7, 0);
assert.equal(store.snapshot().slots.at(-1).time.complete, false);

// Unsupported and absent values cannot turn into measured zero.
store.feature('longAnimationFrames', 'not_supported');
result = timing.validate('BrowserSnapshot', store.snapshot());
assert.equal(result.slots.at(-1).longFrame.count.state, 'not_supported');
assert.equal(result.slots.at(-1).inputDelay.maxMs.state, 'unavailable');

// Delayed entries at a modulo boundary cannot evict the newest bucket or reset its rate budget.
now = 0; store.begin(5); store.feature('eventTiming', 'measured');
now = 600100;
for (let i = 0; i < 100; i++) store.record('chat output', i, 0);
store.input({ startTime: 200, duration: 24, processingStart: 210, processingEnd: 211 });
store.task({ startTime: 100, duration: 60 });
store.frame({ startTime: 200, duration: 60, scripts: [] });
store.record('chat output', 777, 0);
result = timing.validate('BrowserSnapshot', store.snapshot());
assert.equal(result.slots.length, 1);
assert.equal(result.slots[0].time.startMonoMs, 600000);
assert.equal(result.slots[0].renderWork.count.value, 101);
assert.equal(result.slots[0].renderWork.maxMs.value, 777);
assert.equal(result.raw.work.length, 100);
assert.equal(result.coverage.workRateOmissions, 1);
assert.equal(result.raw.input.length, 0);
assert.equal(result.raw.frames.length, 0);
// Out-of-order reports inside the retained window still update their own slot.
store.input({ startTime: 1100, duration: 24, processingStart: 1110, processingEnd: 1111 });
result = timing.validate('BrowserSnapshot', store.snapshot());
assert.equal(result.slots.length, 2);
assert.equal(result.slots[0].inputDelay.maxMs.value, 10);
assert.equal(result.slots[1].renderWork.maxMs.value, 777);
const anchor = { utc: utc(), monoMs: 0 };
const backend = { schemaVersion: 1, recording: { enabled: true, generation: 42, anchor, state: 'on' }, capturedAt: anchor,
  coverage: { retainedSlots: 0, overwrittenSlots: 0, missedPeriods: 0, disabledObservations: { state: 'unavailable', value: null } }, slots: [] };
const exported = { schemaVersion: 1, copiedAt: anchor, copyRequest: { anchor, startMonoMs: 0, durationMs: 1, complete: true }, browser: result, backend };
// A populated backend response exercises every field of the shared contract.
const measured = value => ({ state: 'measured', value });
const absent = () => ({ state: 'unavailable', value: null });
const stats = () => ({ count: measured(1), totalMs: measured(2), maxMs: measured(2), maxCompletedOffsetMs: measured(3), saturated: false });
const span = { anchor, startMonoMs: 0, durationMs: 1000, complete: true };
backend.slots = [{ time: span,
  categories: Object.fromEntries(['agent_text', 'agent_tool', 'chat_state', 'team_state', 'bus_change', 'other'].map(key => [key, { appEmit: stats(), bridgeEnqueue: stats() }])),
  queues: { subscriberPeak: measured(0), depthPeak: measured(0), overflowDisconnects: measured(0) },
  runtime: { scope: 'process_wide_interval', time: span, gcPauseCount: measured(0), gcPauseMaxBucketLowerMs: absent(), gcPauseMaxBucketUpperMs: absent(), gcPauseUpperOpen: false, mutexWaitMs: measured(0) } }];
backend.coverage.retainedSlots = 1;
const text = timing.serialize(exported);
assert.ok(text.startsWith(timing.HEADER + '\n'));
assert.equal(text.includes(canary), false);
assert.ok(new TextEncoder().encode(text).length < timing.EXPORT_LIMIT);
// The copy limit is checked against UTF-8 bytes before any clipboard call.
const oversized = structuredClone(exported);
const largeStats = () => ({ count: measured(Number.MAX_SAFE_INTEGER), totalMs: measured(1.2345678901234567e+200), maxMs: measured(1.2345678901234567e+200), maxCompletedOffsetMs: measured(123.45678901234567), saturated: true });
const backendSlot = oversized.backend.slots[0];
for (const value of Object.values(backendSlot.categories)) { value.appEmit = largeStats(); value.bridgeEnqueue = largeStats(); }
oversized.backend.slots = Array.from({ length: 600 }, () => structuredClone(backendSlot)); oversized.backend.coverage.retainedSlots = 600;
const browserSlot = oversized.browser.slots[0];
for (const key of ['inputDelay', 'renderWork', 'renderWait', 'longTask', 'longFrame']) browserSlot[key] = largeStats();
oversized.browser.slots = Array.from({ length: 600 }, () => structuredClone(browserSlot)); oversized.browser.coverage.retainedSlots = 600;
timing.validate('Export', oversized);
const actualBytes = new TextEncoder().encode(timing.HEADER + '\n' + JSON.stringify(oversized)).length;
if (actualBytes > timing.EXPORT_LIMIT) assert.throws(() => timing.serialize(oversized), /too large/);
else {
  // The closed schema currently fits below the cap even with full large-number rings.
  assert.ok(actualBytes < timing.EXPORT_LIMIT);
  const NativeEncoder = global.TextEncoder;
  try {
    global.TextEncoder = class { encode() { return { length: timing.EXPORT_LIMIT + 1 }; } };
    assert.throws(() => timing.serialize(oversized), /too large/);
  } finally { global.TextEncoder = NativeEncoder; }
}
// Every object, including objects nested inside records, rejects unknown fields.
function unknownFields(value) {
  if (!value || typeof value !== 'object') return;
  if (!Array.isArray(value)) { value.secret = canary; assert.throws(() => timing.serialize(exported), /Invalid timing snapshot/); delete value.secret; }
  for (const child of Object.values(value)) unknownFields(child);
}
unknownFields(exported);
const input = result.raw.input;
result.raw.input = [{ time: { anchor, startMonoMs: 0, durationMs: 80, complete: true }, category: 'input_delay', delayMs: { state: 'unavailable', value: 0 }, processingMs: { state: 'measured', value: 0 }, eventDurationMs: { state: 'measured', value: 80 } }];
assert.throws(() => timing.serialize(exported), /Invalid timing snapshot/);
result.raw.input = input;
result.raw.work[0].category = canary;
assert.throws(() => timing.serialize(exported), /Invalid timing snapshot/);
console.log('PASS timing storage privacy, rates, wrap, suspension, disabled path and closed schema');
