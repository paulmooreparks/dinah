// t.deadline is the page's bound on waits that carry none of their own
// (lib/page.js). Both halves of its promise are pinned here: a wait that
// never settles fails at the deadline with what it waited for in the message,
// and a wait that settles leaves nothing behind — the child process exits
// promptly, which it cannot do while a timer is left running (the 45 s after
// every close that put the gate at 828 s on 30 October 2026).
const { launch } = require('./lib/page');
const { spawnSync } = require('child_process');
const path = require('path');

(async () => {
  // Launch and read at the harness's normal slowness; the short deadline
  // belongs only to the wait under test.
  const t = await launch({ timeout: 10000 });
  try {
    let err = null;
    const at = Date.now();
    try { await t.deadline('the answer that never comes', new Promise(() => {}), 800); } catch (e) { err = e; }
    const ms = Date.now() - at;
    t.ok('a wait that never settles fails at its deadline, with its name', !!err && /Timed out after 800 ms waiting for the answer that never comes/.test(String(err)) && ms >= 750, err ? String(err) + ' after ' + ms + ' ms' : 'it settled');
    t.ok('a wait that settles answers with its value, still at once', await t.deadline('a settling wait', Promise.resolve(42)) === 42);
    t.ok('and a settled rejection still rejects with its error', await t.deadline('a failing wait', Promise.reject(new Error('the wait itself failed'))).then(() => false, e => /the wait itself failed/.test(String(e))));
  } finally {
    await t.close();
  }
  // The child's whole work is a settle and a close; measured by its exit.
  const start = Date.now();
  /* A wedged child can hold the whole batch: it is answered within its own
     named bound, and the check then says so instead of the runner's kill. */
  const run = spawnSync(process.execPath, [path.join(__dirname, 'lib', 'deadline-child.js')], { encoding: 'utf8', timeout: 55000 });
  const wallS = (Date.now() - start) / 1000;
  const timedOut = run.error && run.error.code === 'ETIMEDOUT';
  const out = (run.stdout || '') + (run.stderr || '');
  const passed = run.status === 0 && !timedOut && /settled-and-closed/.test(out) && wallS < 40;
  console.log((passed ? 'PASS ' : 'FAIL ') + 'a suite that settles its waits leaves no timer behind (child exited in ' + wallS.toFixed(1) + ' s)' + (passed ? '' : ' :: ' + JSON.stringify({ status: run.status, out: out.slice(0, 300) })));
  if (!passed) process.exitCode = 1;
})();
