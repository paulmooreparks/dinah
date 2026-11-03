// The real OS lock must exclude independent entry points and survive crashes.
// These process checks use an isolated file and open no browser.
const assert = require('assert/strict');
const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const root = fs.mkdtempSync(path.join(os.tmpdir(), 'dinah-lock-contract-'));
const lockPath = path.join(root, 'browser.lock');
const queuePath = path.join(root, 'browser.queue');
const children = [];
function child(command, args, env = {}) {
  const p = spawn(command, args, { windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'], env: { ...process.env, DINAH_BROWSER_TEST_TICKET: '', ...env } });
  const c = { p, text: '', code: null, closed: false, listeners: [] };
  function changed() { c.listeners.slice().forEach(f => f()); }
  p.stdout.on('data', data => { c.text += data; changed(); });
  p.stderr.on('data', data => { c.text += data; changed(); });
  p.on('close', code => { c.code = code; c.closed = true; changed(); });
  p.on('error', error => { c.text += error; changed(); });
  c.wait = (test, name) => new Promise((resolve, reject) => {
    const timer = setTimeout(() => finish(new Error('Timed out waiting for ' + name + ': ' + c.text)), 20000);
    function finish(error) { clearTimeout(timer); c.listeners = c.listeners.filter(f => f !== check); error ? reject(error) : resolve(); }
    function check() { if (test(c)) finish(); else if (c.closed) finish(new Error(name + ' exited early: ' + c.text)); }
    c.listeners.push(check); check();
  });
  children.push(c); return c;
}
function powershell(maxSeconds) {
  const args = ['-NoProfile', '-NonInteractive', '-File', path.join(__dirname, 'lib/browser-lock.ps1'), '-LockPath', lockPath, '-Owner', 'PowerShell lock contract holder'];
  if (maxSeconds) args.push('-MaxSeconds', String(maxSeconds));
  return child('pwsh', args);
}
function tickets() { return fs.existsSync(queuePath) ? fs.readdirSync(queuePath).filter(n => n.endsWith('.ticket')) : []; }
function ticketNumber(c) { return BigInt(c.text.match(/; ticket (\d+);/)[1]); }
function owner() { return JSON.parse(fs.readFileSync(lockPath + '.owner.json', 'utf8')); }
async function until(test, name) {
  const deadline = Date.now() + 20000;
  while (!test()) {
    if (Date.now() > deadline) throw new Error('Timed out waiting for ' + name);
    await new Promise(resolve => setTimeout(resolve, 25));
  }
}
function node(ticket) { return child(process.execPath, [path.join(__dirname, 'lib/lock-client.js'), lockPath], ticket ? { DINAH_BROWSER_TEST_TICKET: ticket } : {}); }
const readyPS = c => c.text.includes('DINAH_BROWSER_LOCK_READY acquired');
const readyNode = c => c.text.includes('CLIENT_READY');
const blocked = c => c.text.includes('Waiting for browser test lock:');
async function release(c) { c.p.stdin.end('release\n'); await c.wait(x => x.closed, 'clean release'); assert.equal(c.code, 0, c.text); }
(async () => {
  try {
    const first = powershell(); await first.wait(readyPS, 'PowerShell acquisition');
    const ticket = fs.readFileSync(lockPath + '.owner.json', 'utf8');
    const parallel = node(ticket); await parallel.wait(readyNode, 'validated batch child'); await release(parallel);
    assert.equal(tickets().length, 1, 'a borrowed lease must not take a ticket');
    console.log('PASS a validated batch child shares the existing run');
    const second = node(); await second.wait(blocked, 'node contender to wait');
    assert(!readyNode(second), second.text);
    assert(second.text.includes('PowerShell lock contract holder') && /since \d{4}-/.test(second.text), second.text);
    await release(first); await second.wait(readyNode, 'node acquisition after release');
    console.log('PASS independent PowerShell and node runs cannot overlap and report the holder');
    const third = powershell(); await third.wait(blocked, 'PowerShell contender to wait'); assert(!readyPS(third));
    second.p.kill('SIGKILL'); await second.wait(c => c.closed, 'node crash');
    await third.wait(readyPS, 'acquisition after node crash');
    console.log('PASS a crashed node run closes its helper pipe and releases the OS lock');
    const fourth = node(ticket); await fourth.wait(blocked, 'stale ticket to wait'); assert(!readyNode(fourth));
    third.p.kill('SIGKILL'); await third.wait(c => c.closed, 'PowerShell crash');
    await fourth.wait(readyNode, 'acquisition after PowerShell crash'); await release(fourth);
    console.log('PASS stale tickets cannot bypass the lock and a crashed PowerShell holder releases it');

    const held = powershell(); await held.wait(readyPS, 'FIFO holder');
    const waiters = [];
    for (let i = 0; i < 3; i++) {
      const c = i === 1 ? node() : powershell();
      await c.wait(blocked, 'FIFO waiter ' + i);
      assert(c.text.includes('queue position ' + (i + 2)), c.text);
      waiters.push(c);
    }
    let stopChurn = false, churnCount = 0;
    const churn = (async () => {
      while (!stopChurn) {
        const c = node();
        await c.wait(x => blocked(x) || readyNode(x), 'later arriving waiter');
        const before = tickets().length;
        c.p.kill('SIGKILL'); await c.wait(x => x.closed, 'later waiter departure');
        await until(() => tickets().length < before, 'dead Node waiter ticket removal');
        churnCount++;
      }
    })();
    await until(() => churnCount > 0, 'first later arrival and departure');
    await release(held);
    const served = [];
    for (const c of waiters) {
      await c.wait(x => readyPS(x) || readyNode(x), 'next FIFO waiter');
      const v = owner(); served.push({ number: ticketNumber(c), since: Date.parse(v.since) });
      assert.equal(BigInt(path.basename(v.queue).split('-')[0]), ticketNumber(c));
      for (const later of waiters.slice(served.length)) assert(!readyPS(later) && !readyNode(later), later.text);
      const previousArrivals = churnCount;
      await until(() => churnCount > previousArrivals, 'later arrival and departure while this waiter runs');
      if (c === waiters[waiters.length - 1]) { stopChurn = true; await churn; }
      await release(c);
    }
    assert(served.every((v, i) => i === 0 || (v.number > served[i - 1].number && v.since >= served[i - 1].since)));
    console.log('PASS three waiters acquire in ticket and recorded-time order while later waiters arrive and leave: ' + JSON.stringify(served.map(v => ({ number: String(v.number), since: v.since }))));

    const crashHolder = powershell(); await crashHolder.wait(readyPS, 'holder for dead-head check');
    const deadHead = powershell(); await deadHead.wait(blocked, 'first queued waiter');
    const next = node(); await next.wait(blocked, 'second queued waiter');
    const deadNumber = ticketNumber(deadHead).toString().padStart(19, '0');
    deadHead.p.kill('SIGKILL'); await deadHead.wait(c => c.closed, 'head waiter killed');
    await until(() => !tickets().some(n => n.startsWith(deadNumber)), 'abandoned head ticket cleanup');
    await release(crashHolder); await next.wait(readyNode, 'second waiter after head death'); await release(next);
    console.log('PASS a killed first waiter stops blocking the second, without a PID liveness check');

    const corruptHolder = powershell(); await corruptHolder.wait(readyPS, 'holder for sequence recovery');
    const earlier = powershell(); await earlier.wait(blocked, 'waiter before corruption');
    fs.writeFileSync(path.join(queuePath, 'sequence'), 'bad');
    const recovered = node(); await recovered.wait(blocked, 'waiter after corrupt sequence');
    assert(ticketNumber(recovered) > ticketNumber(earlier));
    await release(corruptHolder); await earlier.wait(readyPS, 'earlier after recovery'); await release(earlier);
    await recovered.wait(readyNode, 'recovered sequence waiter'); await release(recovered);
    console.log('PASS a partial sequence recovers above every existing ticket');

    const modulePath = path.join(__dirname, 'lib/browser-lock.js');
    const command = `require(${JSON.stringify(modulePath)}).acquire({lockPath:${JSON.stringify(lockPath)}}).then(async release=>{console.log('WRAPPER_READY');await release();process.exit(7);}).catch(e=>{console.error(e);process.exit(1);});`;
    const wrapper = child('pwsh', ['-NoProfile', '-File', path.resolve(__dirname, '../../../../scripts/with-test-lock.ps1'), '-LockPath', lockPath, process.execPath, '-e', command]);
    await wrapper.wait(c => c.closed, 'command wrapper completion');
    assert(wrapper.text.includes('WRAPPER_READY'), wrapper.text);
    assert.equal(wrapper.code, 7, wrapper.text);
    assert.equal(tickets().length, 0);
    console.log('PASS the command wrapper lends its lease and returns the command exit code');

    const timedHolder = powershell(); await timedHolder.wait(readyPS, 'holder for lease timing');
    const timedWaiter = powershell(1); await timedWaiter.wait(blocked, 'one-second lease waiter');
    const waitStarted = Date.now();
    await until(() => Date.now() - waitStarted > 1500, 'waiting longer than the lease cap');
    assert(!timedWaiter.closed && !readyPS(timedWaiter), timedWaiter.text);
    await release(timedHolder); await timedWaiter.wait(readyPS, 'one-second lease after long wait'); await release(timedWaiter);
    console.log('PASS queue waiting does not consume the holder lease limit');
  } finally {
    for (const c of children) if (!c.closed) c.p.kill('SIGKILL');
    await Promise.allSettled(children.map(c => c.wait(x => x.closed, 'child cleanup')));
    for (const file of [lockPath, lockPath + '.owner.json']) { try { fs.unlinkSync(file); } catch {} }
    fs.rmSync(root, { recursive: true, force: true });
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
