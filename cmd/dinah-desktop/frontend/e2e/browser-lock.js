// The real OS lock must exclude independent entry points and survive crashes.
// These process checks use an isolated file and open no browser.
const assert = require('assert/strict');
const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const root = fs.mkdtempSync(path.join(os.tmpdir(), 'dinah-lock-contract-'));
const lockPath = path.join(root, 'browser.lock');
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
function powershell() {
  return child('pwsh', ['-NoProfile', '-NonInteractive', '-File', path.join(__dirname, 'lib/browser-lock.ps1'), '-LockPath', lockPath, '-Owner', 'PowerShell lock contract holder']);
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
    console.log('PASS a validated batch child shares the existing run');
    const second = node(); await second.wait(blocked, 'node contender to wait');
    assert(!readyNode(second), second.text);
    assert(second.text.includes('PowerShell lock contract holder') && /since \d{4}-/.test(second.text), second.text);
    await release(first); await second.wait(readyNode, 'node acquisition after release');
    console.log('PASS independent PowerShell and node runs cannot overlap and report the holder');
    const third = powershell(); await third.wait(blocked, 'PowerShell contender to wait'); assert(!readyPS(third));
    second.p.kill(); await second.wait(c => c.closed, 'node crash');
    await third.wait(readyPS, 'acquisition after node crash');
    console.log('PASS a crashed node run closes its helper pipe and releases the OS lock');
    const fourth = node(ticket); await fourth.wait(blocked, 'stale ticket to wait'); assert(!readyNode(fourth));
    third.p.kill(); await third.wait(c => c.closed, 'PowerShell crash');
    await fourth.wait(readyNode, 'acquisition after PowerShell crash'); await release(fourth);
    console.log('PASS stale tickets cannot bypass the lock and a crashed PowerShell holder releases it');
  } finally {
    for (const c of children) if (!c.closed) c.p.kill();
    await Promise.allSettled(children.map(c => c.wait(x => x.closed, 'child cleanup')));
    for (const file of [lockPath, lockPath + '.owner.json']) { try { fs.unlinkSync(file); } catch {} }
    try { fs.rmdirSync(root); } catch {}
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
