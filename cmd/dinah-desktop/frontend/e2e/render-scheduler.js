// Exercise frame admission with a controlled clock, rather than machine timing.
const fs = require('fs'), path = require('path'), vm = require('vm'), assert = require('assert/strict');
const frames = new Map(), events = {}, errors = [], watched = new Set();
let serial = 0, now = 0, resize;
const document = { hidden: false, addEventListener: (name, fn) => { events[name] = fn; } };
const window = {};
vm.runInNewContext(fs.readFileSync(path.join(__dirname, '../render-scheduler.js'), 'utf8'), {
  window, document, performance: { now: () => now },
  requestAnimationFrame: fn => { frames.set(++serial, fn); return serial; },
  cancelAnimationFrame: id => frames.delete(id), setTimeout: fn => errors.push(fn),
  ResizeObserver: class {
    constructor(fn) { resize = fn; }
    observe(root) { watched.add(root); }
    unobserve(root) { watched.delete(root); }
  }
});
const scheduler = window.dinahRender;
const root = () => ({ isConnected: true, shown: true, getClientRects() { return this.shown ? [{}] : []; } });
function frame() {
  assert.equal(frames.size, 1, 'there is one shared frame callback');
  const [id, fn] = frames.entries().next().value;
  frames.delete(id); fn();
}
function test(name, fn) { fn(); console.log('PASS ' + name); }
test('repeated invalidations coalesce and an idle queue requests no frames', () => {
  let value = 0;
  const key = {}, view = root();
  for (let i = 1; i <= 1000; i++) scheduler.enqueue(key, view, () => { value = i; });
  frame(); assert.equal(value, 1000); assert.equal(frames.size, 0); assert.equal(watched.size, 0);
});
test('the shared budget yields and rotates unfinished views fairly', () => {
  const calls = [], a = {}, b = {};
  scheduler.enqueue(a, root(), () => { calls.push('a'); now += 6; return calls.filter(x => x === 'a').length < 2; });
  scheduler.enqueue(b, root(), () => { calls.push('b'); });
  frame(); assert.deepEqual(calls, ['a']);
  frame(); assert.deepEqual(calls, ['a', 'b', 'a']); assert.equal(frames.size, 0);
});
test('hidden views sleep and resume with their latest state when revealed', () => {
  const view = root(), key = {}; view.shown = false;
  let value = 0;
  scheduler.enqueue(key, view, () => { value = 1; });
  frame(); assert.equal(value, 0); assert.equal(frames.size, 0);
  scheduler.enqueue(key, view, () => { value = 2; });
  assert.equal(value, 0); assert.equal(frames.size, 0);
  view.shown = true; resize(); frame();
  assert.equal(value, 2); assert.equal(frames.size, 0);
});
test('geometry callbacks follow every admitted write in the frame', () => {
  const calls = [];
  scheduler.enqueue({}, root(), () => { calls.push('write a'); }, () => calls.push('read a'));
  scheduler.enqueue({}, root(), () => { calls.push('write b'); }, () => calls.push('read b'));
  frame(); assert.deepEqual(calls, ['write a', 'write b', 'read a', 'read b']);
});
test('moving a parked job to another root wakes it', () => {
  const hidden = root(), key = {}; hidden.shown = false;
  let value = 0;
  scheduler.enqueue(key, hidden, () => { value++; }); frame();
  assert.equal(value, 0);
  scheduler.enqueue(key, root(), () => { value++; }); frame();
  assert.equal(value, 1); assert.equal(frames.size, 0); assert.equal(watched.size, 0);
});
test('an invalidation during rendering survives to the next frame', () => {
  const key = {}, view = root(); let value = 0;
  scheduler.enqueue(key, view, () => { value++; scheduler.enqueue(key, view, () => { value++; }); });
  frame(); assert.equal(value, 1); frame(); assert.equal(value, 2); assert.equal(frames.size, 0);
});
test('document visibility and cancellation preserve the lifecycle', () => {
  const key = {}, view = root(); let value = 0;
  document.hidden = true;
  scheduler.enqueue(key, view, () => { value++; });
  assert.equal(frames.size, 0);
  document.hidden = false; events.visibilitychange(); frame(); assert.equal(value, 1);
  scheduler.enqueue(key, view, () => { value++; }); scheduler.cancel(key);
  assert.equal(frames.size, 0); assert.equal(watched.size, 0); assert.equal(value, 1);
});
test('work scheduled by a geometry callback is not lost', () => {
  let value = 0;
  scheduler.enqueue({}, root(), () => {}, () => scheduler.enqueue({}, root(), () => { value++; }));
  frame(); assert.equal(value, 0); frame(); assert.equal(value, 1); assert.equal(frames.size, 0);
});
test('one broken view does not prevent another view from rendering', () => {
  let value = 0;
  scheduler.enqueue({}, root(), () => { throw new Error('fixture'); });
  scheduler.enqueue({}, root(), () => { value++; });
  frame(); assert.equal(value, 1); assert.equal(errors.length, 1); assert.equal(frames.size, 0);
});
test('one broken geometry callback does not prevent the others from running', () => {
  let value = 0;
  scheduler.enqueue({}, root(), () => {}, () => { throw new Error('geometry fixture'); });
  scheduler.enqueue({}, root(), () => {}, () => { value++; });
  frame(); assert.equal(value, 1); assert.equal(errors.length, 2); assert.equal(frames.size, 0);
});
// idle() is the signal a check waits on (docs/CONTRIBUTING.md). Each case
// records when the promise resolved against the frames that had run.
(async () => {
  const settled = async promise => { let done = false; promise.then(() => { done = true; }); for (let i = 0; i < 5; i++) await null; return done; };
  assert.equal(await settled(scheduler.idle()), true);
  console.log('PASS an empty queue is idle at once');
  let draws = 0;
  scheduler.enqueue({}, root(), () => { draws++; now += 6; return draws < 3; });
  const busy = scheduler.idle();
  assert.equal(await settled(busy), false);
  frame(); assert.equal(await settled(busy), false);
  frame(); assert.equal(await settled(busy), false);
  frame(); assert.equal(draws, 3); assert.equal(await settled(busy), true);
  console.log('PASS idle waits for unfinished work across frames and resolves after its last frame');
  const hidden = root(), key = {}; hidden.shown = false;
  scheduler.enqueue(key, hidden, () => {});
  const parked = scheduler.idle();
  assert.equal(await settled(parked), false);
  frame(); assert.equal(await settled(parked), true);
  hidden.shown = true; resize();
  const woken = scheduler.idle();
  assert.equal(await settled(woken), false);
  frame(); assert.equal(await settled(woken), true);
  console.log('PASS a view that cannot be seen does not hold idle, and waking it does');
  const cancelled = {};
  scheduler.enqueue(cancelled, root(), () => {});
  const removed = scheduler.idle();
  assert.equal(await settled(removed), false);
  scheduler.cancel(cancelled); assert.equal(await settled(removed), true);
  console.log('PASS cancelling the last job resolves idle');
})().catch(error => { console.error(error); process.exitCode = 1; });
