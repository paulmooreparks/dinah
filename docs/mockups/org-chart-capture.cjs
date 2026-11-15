/* Run through scripts/with-test-lock.ps1. This serves only this checkout. */
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
      const errors = []; const pageFailure = new Promise((resolve, reject) => page.on('pageerror', error => { errors.push(error.message); reject(error); }));
      pageFailure.catch(() => {});
      await page.goto('http://127.0.0.1:' + server.address().port + '/docs/mockups/org-chart.html?theme=' + theme);
      await Promise.race([page.waitForFunction(() => document.documentElement.dataset.mockReady === 'true'), pageFailure]);
      await page.evaluate(() => document.fonts.ready);
      const state = await page.evaluate(() => ({
        renderer: __orgChart.renderer, theme: document.documentElement.dataset.theme,
        pageFits: document.documentElement.scrollWidth <= innerWidth,
        textNodes: document.querySelectorAll('svg text').length,
        dotted: __orgChart.doc.edges.filter(edge => edge.yavc.line === 'dotted').length,
        reporting: __orgChart.doc.edges.filter(edge => edge.yavc.line === 'solid').length,
        menuFits: (() => { const r = document.querySelector('.mock-menu').getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth; })(),
        height: document.documentElement.scrollHeight
      }));
      if (errors.length || !state.pageFits || !state.menuFits || state.dotted !== (width === 390 ? 5 : 4) || state.reporting !== 4) throw new Error(JSON.stringify({ width, theme, state, errors }));
      await page.screenshot({ path: path.join(output, 'org-chart-' + theme + '-' + width + '.png'), fullPage: true });
      results.push({ width, ...state, errors }); await page.close();
    }
    fs.writeFileSync(path.join(output, 'org-chart-checks.json'), JSON.stringify(results, null, 2));
    console.log('PASS four org chart pictures, no page errors, drawing and menu fit both widths.');
  } finally { await browser.close(); server.close(); }
})().catch(error => { console.error(error); server.close(); process.exitCode = 1; });
