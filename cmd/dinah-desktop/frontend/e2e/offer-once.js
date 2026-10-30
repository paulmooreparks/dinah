// The bus pane and the In Progress pane each offer themselves once, and a
// close that beats the layout debounce must never bring the offer back
// (bus-applets.js, in-progress.js): the flag recording the offer is a fact,
// flushed the moment it is set rather than with layout that may be lost.
//
// This suite ends the first session the moment the offer's window appears,
// restarts on what the fastest close can keep, and checks that no offer
// repeats.
const { launch, PAGE } = require('./lib/page');
(async () => {
  let nativeState = { version: 1, values: {} };
  async function start() {
    const t = await launch({
      viewport: { width: 1440, height: 900 },
      app: () => ({ ReadUIState: () => nativeReadOffer(), SaveUIState: state => nativeSaveOffer(state) }),
      goto: false,
    });
    await t.context.exposeFunction('nativeReadOffer', () => nativeState);
    await t.context.exposeFunction('nativeSaveOffer', state => { nativeState = state; });
    await t.page.goto(PAGE);
    await t.waitReady();
    return t;
  }
  let t = await start();
  try {
    const page = t.page;
    // The offer opens the pane as a hidden background tab, so presence, not
    // visibility, is what proves it ran.
    await page.locator('[data-win="bus"]').waitFor({ state: 'attached' });
    // The session ends the moment the window is there; the close alone takes
    // longer than the 150 ms layout debounce, so nothing but the offer's own
    // eager write is guaranteed to be in the store.
    const offerAt = Date.now();
    await t.deadline('the first browser to close at once', t.browser.close(), 45000).catch(error => { t.ok('the first browser closes', false, String(error)); });
    const closedIn = Date.now() - offerAt;
    t.ok('the bus offer’s flag is a fact in the native store (closed ' + closedIn + ' ms after its window appeared)', nativeState.values['dinah-bus-offered'] === true, JSON.stringify(nativeState.values));
    t.ok('the in-progress offer’s flag is too', nativeState.values['dinah-in-progress-offered'] === true, JSON.stringify(nativeState.values));
    // What the fastest close keeps: the flags, and none of the layout that
    // still waited on the debounce.
    nativeState = { version: 1, values: { 'dinah-bus-offered': true, 'dinah-in-progress-offered': true } };
    t = await start();
    // Let the restore and every once-only listener fire before counting.
    await t.deadline('the restarted page to finish loading', t.page.evaluate(() => document.readyState === 'complete' ? true : new Promise(resolve => window.addEventListener('load', () => resolve(true), { once: true }))), 30000);
    await t.deadline('two frames after the restart', t.page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))), 30000);
    t.ok('the bus offer is not repeated when only the offer-time writes survive', await t.page.locator('[data-win="bus"]').count() === 0);
    t.ok('the in-progress pane is not opened twice', await t.page.locator('[data-win="in-progress"]').count() <= 1);
  } finally {
    if (t) await t.deadline('the last browser to close', t.close(), 45000).catch(error => { console.log('FAIL the last browser to close :: ' + error); process.exitCode = 1; });
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
