// Renders every state of decisions-redesign.html headless at 1440 and 390
// pixels, light and dark, and writes one PNG for each.
//   pwsh scripts/with-test-lock.ps1 node docs/mockups/decisions/explore/capture-mockup.js <out folder> [state,state]
const path = require('path'), fs = require('fs');
const E2E = path.join(__dirname, '..', '..', '..', '..', 'cmd', 'dinah-desktop', 'frontend', 'e2e');
const { chromium } = require('playwright-core');
const { acquire } = require(path.join(E2E, 'lib', 'browser-lock'));
const ROOT = path.join(__dirname, '..', '..', '..', '..');
const OUT = process.argv[2] || path.join(__dirname, '..', 'pictures');
const ONLY = process.argv[3] ? process.argv[3].split(',') : null;
const EDGE = 'msedge';
const PAGE = 'http://mock.test/docs/mockups/decisions/decisions-redesign.html';
fs.mkdirSync(OUT, { recursive: true });
(async () => {
  const release = await acquire();
  const browser = await chromium.launch({ channel: EDGE, headless: true });
  try {
    const sizes = [{ w: 1440, h: 1000 }, { w: 390, h: 844 }];
    let states = null;
    for (const size of sizes) for (const theme of ['light', 'dark']) {
      const context = await browser.newContext({ viewport: { width: size.w, height: size.h } });
      await context.route('http://mock.test/**', route => {
        const file = path.join(ROOT, decodeURIComponent(new URL(route.request().url()).pathname));
        if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) return route.fulfill({ status: 404, body: 'Not found' });
        return route.fulfill({ path: file });
      });
      const page = await context.newPage();
      const errors = []; page.on('pageerror', e => errors.push(String(e)));
      if (!states) { await page.goto(PAGE + '?bare=1&state=empty'); states = await page.evaluate(() => window.__mockStates); }
      for (const state of states) {
        if (ONLY && !ONLY.includes(state)) continue;
        // On a phone, a list-and-detail state is shown twice: the list pane and the request pane.
        const panes = size.w <= 640 && !['empty', 'last-done'].includes(state) ? ['detail'].concat(['several', 'filter', 'filter-none', 'earlier', 'sent-leaves', 'task'].includes(state) ? ['list'] : []) : ['detail'];
        for (const pane of panes) {
          await page.goto(PAGE + '?bare=1&state=' + state + '&theme=' + theme + (pane === 'list' ? '&pane=list' : ''));
          await page.evaluate(() => document.fonts.ready); await page.waitForFunction(() => document.documentElement.dataset.mockSettled === 'true');
          await page.waitForTimeout(80);
          const name = state + (pane === 'list' ? '-list' : '') + '-' + size.w + '-' + theme + '.png';
          await page.screenshot({ path: path.join(OUT, name) });
          const overflow = await page.evaluate(() => [...document.querySelectorAll('.md-detail, .decisions-scroll, .decisions-foot, .md-sidebar')].filter(el => el.scrollWidth > el.clientWidth + 1).map(el => el.className));
          console.log(name + (overflow.length ? ' OVERFLOW ' + overflow.join(' | ') : ''));
        }
      }
      if (errors.length) console.log('PAGE ERRORS ' + errors.join(' | '));
      await context.close();
    }
  } finally { await browser.close(); await release(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
