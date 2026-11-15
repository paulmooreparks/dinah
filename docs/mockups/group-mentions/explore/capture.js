// Renders every state of the group mentions mockup headless at 1440 and 390
// pixels, light and dark, and writes one PNG for each.
//   pwsh scripts/with-test-lock.ps1 node docs/mockups/group-mentions/explore/capture.js [out folder] [state,state]
const path = require('path'), fs = require('fs');
const { PAGE, withBrowser, newPage } = require('./common');
const OUT = process.argv[2] || path.join(__dirname, '..', 'pictures');
const ONLY = process.argv[3] ? process.argv[3].split(',') : null;
fs.mkdirSync(OUT, { recursive: true });
withBrowser(async browser => {
  let states = null;
  for (const [w, hgt] of [[1440, 900], [390, 844]]) for (const theme of ['light', 'dark']) {
    const page = await newPage(browser, w, hgt);
    if (!states) { await page.goto(PAGE + '?bare=1&state=list'); states = await page.evaluate(() => window.__mockStates); }
    for (const state of states) {
      if (ONLY && !ONLY.includes(state)) continue;
      await page.goto(PAGE + '?bare=1&state=' + state + '&theme=' + theme);
      await page.waitForFunction(() => document.documentElement.dataset.mockSettled === 'true');
      await page.waitForTimeout(150);
      const name = state + '-' + w + '-' + theme + '.png';
      await page.screenshot({ path: path.join(OUT, name) });
      const overflow = await page.evaluate(() => [...document.querySelectorAll('.group-composer, .slash-pop, .msg, .msg-reach')].filter(el => el.scrollWidth > el.clientWidth + 1).map(el => el.className + ':' + ''));
      console.log(name + (overflow.length ? ' OVERFLOW ' + overflow.join(' | ') : ''));
    }
    if (page.errors.length) console.log('PAGE ERRORS ' + page.errors.join(' | '));
    await page.context().close();
  }
}).catch(e => { console.error(e); process.exitCode = 1; });
