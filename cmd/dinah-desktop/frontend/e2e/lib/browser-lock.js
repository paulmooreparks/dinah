// Node does not expose Windows FileShare.None. The helper owns that handle
// while stdin is open; terminating node closes the pipe and releases it.
const { spawn } = require('child_process');
const path = require('path');
const readline = require('readline');
const DEFAULT_PATH = path.join(process.env.ProgramData || 'C:/ProgramData', 'Dinah', 'browser-tests.lock');

function acquire({ lockPath = DEFAULT_PATH, owner = process.argv[1], ticket = process.env.DINAH_BROWSER_TEST_TICKET } = {}) {
  return new Promise((resolve, reject) => {
    const args = ['-NoProfile', '-NonInteractive', '-File', path.join(__dirname, 'browser-lock.ps1'), '-LockPath', lockPath, '-Owner', owner || 'node browser suite'];
    if (ticket) args.push('-Ticket', ticket);
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
