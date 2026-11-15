// Types into the mockup at a person's pace and records every frame: Send's
// enabled state and the position and size of the head, the scrolling middle,
// the foot, the status line, Send itself and the first question.
//   pwsh scripts/with-test-lock.ps1 node docs/mockups/decisions/explore/mockup-frames.js <out folder>
const path = require('path'), fs = require('fs');
const E2E = path.join(__dirname, '..', '..', '..', '..', 'cmd', 'dinah-desktop', 'frontend', 'e2e');
const { chromium } = require('playwright-core');
const { acquire } = require(path.join(E2E, 'lib', 'browser-lock'));
const ROOT = path.join(__dirname, '..', '..', '..', '..');
const OUT = process.argv[2] || require('os').tmpdir();
const EDGE = 'msedge';
(async () => {
  const release = await acquire();
  const browser = await chromium.launch({ channel: EDGE, headless: true });
  const results = {};
  try {
    for (const [w, h] of [[1440, 1000], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width: w, height: h } });
      await context.route('http://mock.test/**', route => route.fulfill({ path: path.join(ROOT, decodeURIComponent(new URL(route.request().url()).pathname)) }));
      const page = await context.newPage();
      await page.goto('http://mock.test/docs/mockups/decisions/decisions-redesign.html?bare=1&state=one');
      await page.waitForFunction(() => document.documentElement.dataset.mockSettled === 'true');
      await page.evaluate(() => {
        window.__frames = []; window.__on = true;
        const box = s => { const el = document.querySelector(s); const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height].map(Math.round).join(','); };
        const tick = () => { if (!window.__on) return; const send = document.querySelector('.decisions-foot .btn-primary');
          window.__frames.push({ send: send.disabled, boxes: ['.decisions-head', '.decisions-scroll', '.decisions-foot', '.decisions-foot-status', '.decisions-foot .btn-primary', '.decisions-question'].map(box).join(' '), status: document.querySelector('.decisions-foot-status').textContent });
          requestAnimationFrame(tick); };
        requestAnimationFrame(tick);
      });
      await page.locator('#answer-1-text').click();
      await page.keyboard.type('Ship it in 2.0.2, and tell me the timing after.', { delay: 70 });
      await page.locator('input[name="answer-2"][value="rec"]').check();
      await page.waitForTimeout(300);
      const frames = await page.evaluate(() => { window.__on = false; return window.__frames; });
      const flips = frames.reduce((n, f, i) => n + (i && f.send !== frames[i - 1].send ? 1 : 0), 0);
      const layouts = new Set(frames.map(f => f.boxes)).size;
      results[w] = { layouts: [...new Set(frames.map(f => f.boxes))], frames: frames.length, sendChanges: flips, distinctLayouts: layouts, statusTexts: [...new Set(frames.map(f => f.status))] };
      console.log(w + ' ' + JSON.stringify(results[w]));
      await context.close();
    }
  } finally { await browser.close(); await release(); }
  fs.writeFileSync(path.join(OUT, 'mockup-typing-frames.json'), JSON.stringify(results, null, 1));
})().catch(e => { console.error(e); process.exitCode = 1; });
