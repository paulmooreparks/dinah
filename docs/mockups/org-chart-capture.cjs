/* Run through scripts/with-test-lock.ps1. This serves only this checkout.
   It opens the Org Chart mockup hidden, in both themes at 1440 and 390 pixels,
   checks that it fits, draws a line to the top of each branch, offers no chat
   a report of its own as its lead, and copies a message and its answer to the
   member's lead, and saves a picture of each. */
const http = require('http');
const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright-core');
const root = path.resolve(__dirname, '../..');
const output = path.resolve(process.argv[2]);
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2' };
const server = http.createServer((req, res) => {
  const pathname = new URL(req.url, 'http://127.0.0.1').pathname;
  const file = path.resolve(root, '.' + decodeURIComponent(pathname));
  if (!file.startsWith(root + path.sep) || !types[path.extname(file)] || !fs.existsSync(file)) { res.writeHead(404); res.end(); return; }
  res.setHeader('Content-Type', types[path.extname(file)]); fs.createReadStream(file).pipe(res);
});
(async () => {
  fs.mkdirSync(output, { recursive: true });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const browser = await chromium.launch({ executablePath: 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', headless: true });
  const results = [];
  try {
    for (const width of [1440, 390]) for (const theme of ['light', 'dark']) {
      const page = await browser.newPage({ viewport: { width, height: 1000 }, deviceScaleFactor: 1 });
      const errors = []; page.on('pageerror', error => errors.push(error.message));
      page.on('requestfailed', request => errors.push('failed ' + request.url()));
      await page.goto('http://127.0.0.1:' + server.address().port + '/docs/mockups/org-chart.html');
      await page.waitForFunction(() => document.documentElement.dataset.mockReady === 'true');
      if (theme === 'dark') await page.click('#theme');
      await page.evaluate(() => document.fonts.ready);
      await page.waitForTimeout(100);
      const state = await page.evaluate(() => ({
        theme: document.documentElement.dataset.theme,
        pageFits: document.documentElement.scrollWidth <= innerWidth,
        lines: document.querySelectorAll('.oc-lines path').length,
        teams: document.querySelectorAll('.oc-team').length,
        free: document.querySelectorAll('.oc-free .oc-chat').length,
        cards: document.querySelectorAll('.oc-chat').length
      }));
      await page.screenshot({ path: path.join(output, 'org-chart-' + theme + '-' + width + '.png'), fullPage: false });
      // Harbor Lead's own reports are not offered as its lead.
      const offered = await page.evaluate(() => { __orgChart.openReports('harbor-lead'); return Array.from(document.querySelectorAll('#reports-to option')).map(o => o.textContent); });
      const noCycle = !offered.some(t => /Gap 0[58] /.test(t));
      await page.keyboard.press('Escape');
      // A message to a member is copied, with its answer, to its lead's team room.
      await page.evaluate(() => __orgChart.openChat('g05-reviewer'));
      await page.fill('#composer-text', 'Check the corrected figures.');
      await page.click('#composer button[type=submit]');
      const note = await page.textContent('#copy-note');
      await page.evaluate(() => __orgChart.openRoom('g05-lead'));
      const copies = await page.evaluate(() => document.querySelectorAll('#window-log .mock-copy').length);
      await page.screenshot({ path: path.join(output, 'org-chart-room-' + theme + '-' + width + '.png'), fullPage: false });
      const ok = !errors.length && state.pageFits && state.lines === 2 && state.teams === 4 && state.free === 2 && state.cards === 12 && noCycle && copies === 2 && /Copied to Gap 05 Team/.test(note);
      results.push({ width, ...state, noCycle, copies, note, errors });
      if (!ok) throw new Error(JSON.stringify(results[results.length - 1]));
      await page.close();
    }
    fs.writeFileSync(path.join(output, 'org-chart-checks.json'), JSON.stringify(results, null, 2));
    console.log('PASS org chart mockup at 1440 and 390 in both themes.');
  } finally { await browser.close(); server.close(); }
})().catch(error => { console.error(error); server.close(); process.exitCode = 1; });
