// Uses the Effort control mockup headless the way Paul would and checks it:
// the plain path opens on the composer; the button's label and size never
// change and nothing in the toolbar moves when the level changes; three, five
// and one levels; no button for a chat with no setting; a change while a turn
// runs; the keyboard; the folded toolbar on a phone.
//   pwsh scripts/with-test-lock.ps1 node docs/mockups/effort-control/explore/walkthrough.js
const { PAGE, withBrowser, newPage } = require('./common');
let failures = 0;
function check(ok, what) { console.log((ok ? 'ok   ' : 'FAIL ') + what); if (!ok) failures++; }
withBrowser(async browser => {
  const page = await newPage(browser, 1440, 900);
  page.setDefaultTimeout(8000);
  const settle = () => page.waitForFunction(() => document.documentElement.dataset.mockSettled === 'true');
  const frame = () => page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
  const btn = () => page.locator('[data-command="effort"]');
  const panel = () => page.locator('.effort-panel');
  const filled = () => page.evaluate(() => { const m = document.querySelector('[data-command="effort"] .effort-mark'); return [m.querySelectorAll('i[data-on]').length, m.children.length]; });
  const rects = () => page.evaluate(() => [...document.querySelectorAll('.composer-toolbar button, .composer-bar, #prompt')].filter(b => b.getClientRects().length).map(b => { const r = b.getBoundingClientRect(); return [r.x, r.y, r.width, r.height].map(Math.round).join(','); }).join(' '));
  const width = () => btn().evaluate(b => Math.round(b.getBoundingClientRect().width));
  const tryIt = v => page.locator('[data-try]').selectOption(v);
  const focusedLevel = () => page.evaluate(() => document.activeElement && document.activeElement.dataset.level || (document.activeElement && document.activeElement.dataset.command) || '');

  await page.goto(PAGE + '?state=index');
  check(await page.evaluate(() => document.body.className === 'index'), '?state=index lists the states');
  await page.goto(PAGE); await settle();
  check(await btn().isVisible() && (await btn().locator('.toolbar-words').textContent()) === 'Effort', 'the plain file path opens on the composer, with an Effort button');
  let f = await filled(); check(f[0] === 2 && f[1] === 3, 'the mark has a cell for each of the three levels, filled up to medium (' + f.join(' of ') + ')');
  check((await btn().getAttribute('aria-label')) === 'Effort' && (await page.locator('#effort-now').textContent()) === 'medium, level 2 of 3', 'a screen reader hears Effort and the level in effect');

  // Opening and choosing with the pointer.
  const w3 = await width(), before = await rects();
  await btn().click();
  await panel().waitFor();
  check(await page.locator('.effort-seg > button').count() === 3 && (await page.locator('.effort-seg [aria-pressed="true"]').textContent()) === 'medium', 'pressing it opens the three levels as a segmented control, medium pressed');
  check((await page.locator('.effort-detail').textContent()).includes('Balances speed'), 'the level’s description shows under it');
  await page.locator('.effort-seg [data-level="high"]').click();
  await frame();
  check(!(await panel().isVisible()) && (await focusedLevel()) === 'effort', 'one press chooses, closes the panel and gives focus back to the button');
  f = await filled(); check(f[0] === 3, 'the mark fills to high');
  check((await rects()) === before && (await btn().locator('.toolbar-words').textContent()) === 'Effort', 'nothing in the toolbar moved or resized, and the label is unchanged');

  // Five levels and one level keep the same button width.
  await tryIt('chat:frontend'); await frame();
  const w5 = await width(); f = await filled();
  check(f[0] === 3 && f[1] === 5, 'five levels: five cells, filled up to High');
  await tryIt('chat:local'); await frame();
  const w1 = await width();
  check(w3 === w5 && w5 === w1, 'the button is one width for three, five and one levels (' + [w3, w5, w1].join(', ') + ' px)');
  await btn().click(); await panel().waitFor();
  check(await page.locator('.effort-seg > button').count() === 1 && (await page.locator('.effort-note').textContent()).includes('one effort level'), 'one level: the panel shows it and says there is nothing to choose');
  await page.keyboard.press('Escape'); await frame();
  await tryIt('chat:notes'); await frame();
  check(!(await btn().isVisible()), 'a chat with no effort setting shows no button');

  // The keyboard.
  await tryIt('chat:frontend'); await frame();
  await btn().focus(); await page.keyboard.press('Enter'); await panel().waitFor(); await frame();
  check((await focusedLevel()) === 'high', 'Enter opens the panel with focus on the level in effect');
  await page.keyboard.press('ArrowRight');
  check((await focusedLevel()) === 'xhigh' && (await page.locator('.effort-detail').textContent()).includes('Extended thinking'), 'Right moves to the next level and shows its description');
  await page.keyboard.press('End');
  check((await focusedLevel()) === 'max', 'End moves to the last level');
  await page.keyboard.press('Escape'); await frame();
  check(!(await panel().isVisible()) && (await focusedLevel()) === 'effort' && (await filled())[0] === 3, 'Escape closes without a change and gives focus back');
  await page.keyboard.press('Enter'); await panel().waitFor(); await frame();
  await page.keyboard.press('ArrowLeft'); await page.keyboard.press('Enter'); await frame();
  check(!(await panel().isVisible()) && (await filled())[0] === 2 && (await focusedLevel()) === 'effort', 'Left then Enter chooses Medium and closes');

  // A change while a turn runs.
  await tryIt('chat:engine'); await tryIt('turn'); await frame();
  await btn().click(); await panel().waitFor();
  check((await page.locator('.effort-note[data-running]').textContent()).includes("Codex applies a level you choose now from the next turn"), "in a Codex chat the panel says a new level applies from the next turn, as Codex documents");
  await page.locator('.effort-seg [data-level="low"]').click(); await frame();
  const m = await page.evaluate(() => window.__mockModel());
  check(m.current === 'low' && m.runLevel === 'high' && (await page.locator('.mock-working').textContent()).includes('the turn started at high effort. The turn already running keeps high; low applies from the next turn'), 'the running turn keeps its level and the next turn gets the new one');
  await tryIt('turn'); await frame();
  check(await page.locator('.mock-working').isHidden(), 'finishing the turn clears the note');
  await tryIt('claude-turn'); await frame();
  await btn().click(); await panel().waitFor();
  const claudeNote = await page.locator('.effort-note[data-running]').textContent();
  check(claudeNote.includes('Claude doesn’t say whether a change applies to the turn already running') && !claudeNote.includes('next turn'), 'in a Claude chat the panel promises nothing Claude does not document');
  await page.locator('.effort-seg [data-level="low"]').click(); await frame();
  const claudeLine = await page.locator('.mock-working').textContent();
  check(claudeLine.includes('Claude doesn’t say whether Low applies to the turn already running') && !claudeLine.includes('next turn'), 'and so does the working line after a change');
  await tryIt('turn'); await frame();

  // The phone: the toolbar folds, and the same panel opens in the Agent menu.
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(PAGE + '?state=five'); await settle();
  const agent = page.locator('.composer-toolbar .menu-btn[aria-label="Agent"]');
  check(await agent.isVisible(), 'on a phone the Agent group folds into a menu');
  await agent.click(); await panel().waitFor();
  check(await page.locator('.menu-command-panel .effort-seg > button').count() === 5 && await page.locator('.menu-command-panel .effort-panel > .md-section-label').isHidden(), 'the Agent menu holds the same five levels, under one Effort heading');
  const fits = await panel().evaluate(p => { const r = p.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth; });
  check(fits, 'the panel fits the phone');
  await page.locator('.effort-seg [data-level="max"]').click(); await frame();
  check(!(await panel().isVisible()) && (await page.evaluate(() => window.__mockModel().current)) === 'max', 'choosing there sets the level and closes the menu');

  check(page.errors.length === 0, 'no page errors' + (page.errors.length ? ': ' + page.errors.join(' | ') : ''));
  console.log(failures ? failures + ' FAILED' : 'all passed');
  process.exitCode = failures ? 1 : 0;
}).catch(e => { console.error(e); process.exitCode = 1; });
