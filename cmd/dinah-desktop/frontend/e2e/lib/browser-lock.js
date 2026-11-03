// Node does not expose Windows FileShare.None. The helper owns that handle
// and its FIFO queue ticket while stdin is open. Terminating node closes
// the pipe and releases both, including while it is still waiting. Queue
// position and holder notices come from that one PowerShell implementation.
const { spawn } = require('child_process');
const path = require('path');
const readline = require('readline');
const DEFAULT_PATH = path.join(process.env.ProgramData || 'C:/ProgramData', 'Dinah', 'browser-tests.lock');

// One lease per lock path per process. A suite that launches the page more
// than once (notifications-persistence, packages) asks again while its
// first launch still holds the lock; a second helper would wait on that
// lease forever. Later asks in the same process share the first lease, and
// the lock is released when the last of them releases.
const shared = new Map();

function acquire(options = {}) {
  const lockPath = options.lockPath || DEFAULT_PATH;
  let entry = shared.get(lockPath);
  if (!entry) {
    entry = { users: 0, lease: acquireOnce({ ...options, lockPath }) };
    shared.set(lockPath, entry);
    entry.lease.catch(() => { if (shared.get(lockPath) === entry) shared.delete(lockPath); });
  }
  entry.users++;
  return entry.lease.then(release => {
    let released = false;
    return async function releaseShare() {
      if (released) return;
      released = true;
      if (--entry.users > 0) return;
      if (shared.get(lockPath) === entry) shared.delete(lockPath);
      await release();
    };
  }, error => { entry.users--; throw error; });
}

// maxSeconds bounds the lease (browser-lock.ps1, 900 by default): a suite
// still holding the lock after it is stopped, so a stuck suite cannot hold
// every other run.
function acquireOnce({ lockPath = DEFAULT_PATH, owner = process.argv[1], ticket = process.env.DINAH_BROWSER_TEST_TICKET, maxSeconds } = {}) {
  return new Promise((resolve, reject) => {
    const args = ['-NoProfile', '-NonInteractive', '-File', path.join(__dirname, 'browser-lock.ps1'), '-LockPath', lockPath, '-Owner', owner || 'node browser suite'];
    if (ticket) args.push('-Ticket', ticket);
    if (maxSeconds) args.push('-MaxSeconds', String(maxSeconds));
    const holder = spawn('pwsh', args, { windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] });
    let ready = false, releasing = false;
    const lines = readline.createInterface({ input: holder.stdout });
    holder.stderr.on('data', data => process.stderr.write(data));
    holder.on('error', reject);
    holder.stdin.on('error', error => { if (!releasing && !ready) reject(error); });
    holder.on('close', code => {
      lines.close();
      if (!ready) reject(new Error('Browser test lock helper exited before acquisition (' + code + ').'));
      else if (!releasing) {
        console.error('Browser test lock holder exited unexpectedly. Stopping this suite.');
        process.exit(1);
      }
    });
    lines.on('line', line => {
      if (!line.startsWith('DINAH_BROWSER_LOCK_READY ')) return;
      ready = true;
      const borrowed = line.endsWith('borrowed');
      if (borrowed) releasing = true;
      resolve(async function release() {
        if (releasing) return;
        releasing = true;
        await new Promise(done => { holder.once('close', done); holder.stdin.end('release\n'); });
      });
    });
  });
}
module.exports = { acquire, DEFAULT_PATH };
