// Shared by the capture and walk-through scripts: headless Edge through the
// shared browser lock, serving the repository at http://mock.test/.
const path = require('path'), fs = require('fs'), { execFileSync } = require('child_process');
const ROOT = path.join(__dirname, '..', '..', '..', '..');
const E2E = path.join(ROOT, 'cmd', 'dinah-desktop', 'frontend', 'e2e');
// A worktree has no node_modules of its own; use the main checkout's.
function modules() {
  const local = path.join(E2E, 'node_modules');
  if (fs.existsSync(local)) return local;
  const common = execFileSync('git', ['rev-parse', '--path-format=absolute', '--git-common-dir'], { cwd: ROOT, windowsHide: true }).toString().trim();
  return path.join(path.dirname(common), 'cmd', 'dinah-desktop', 'frontend', 'e2e', 'node_modules');
}
const { chromium } = require(path.join(modules(), 'playwright-core'));
const { acquire } = require(path.join(E2E, 'lib', 'browser-lock'));
const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe';
const PAGE = 'http://mock.test/docs/mockups/group-mentions/index.html';
async function withBrowser(run) {
  const release = await acquire();
  const browser = await chromium.launch({ executablePath: EDGE, headless: true });
  try { return await run(browser); } finally { await browser.close(); await release(); }
}
async function newPage(browser, width, height) {
  const context = await browser.newContext({ viewport: { width, height } });
  await context.route('http://mock.test/**', route => {
    const file = path.join(ROOT, decodeURIComponent(new URL(route.request().url()).pathname));
    if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) return route.fulfill({ status: 404, body: 'Not found' });
    return route.fulfill({ path: file });
  });
  const page = await context.newPage();
  page.errors = []; page.on('pageerror', e => page.errors.push(String(e.stack || e)));
  return page;
}
module.exports = { ROOT, PAGE, withBrowser, newPage };
