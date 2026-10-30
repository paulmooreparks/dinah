// Child process used by the lock contract suite. It opens no browser.
const { acquire } = require('./browser-lock');
(async () => {
  const release = await acquire({ lockPath: process.argv[2], owner: 'node lock contract child' });
  console.log('CLIENT_READY');
  process.stdin.once('data', async () => { await release(); process.exit(0); });
  process.stdin.resume();
})().catch(error => { console.error(error); process.exit(1); });
