// Signing in to Devin: the notice when Devin refuses a request, the wait for
// the browser, success, failure and the menu entry.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    state: { workspace: 'C:\\W', recent: ['C:\\W'], approvals: 'dinah', devin: 'd', titles: {}, templates: [] },
    app: () => ({
      Sessions: async () => { window.__listed++; return []; },
      SignIn: () => { window.__signins++; return new Promise((ok, fail) => { window.__finish = { ok, fail }; }); }
    }),
    init: () => { window.__signins = 0; window.__listed = 0; window.__finish = null; },
  });
  const { page: p, ok } = t;
  require('./lib/windows')(p);
  try {
    await p.waitForTimeout(400);
    ok('no notice while nothing was refused', await p.isHidden('.win[data-chat-current] [data-view-id=signin-note]'));
    await p.evaluate(() => window.__emit('agent:auth', { state: 'required', method: 'Log in with browser' }));
    await p.waitForTimeout(100);
    await p.locator('.win[data-chat-current] [data-view-id=signin-note]').waitFor();
    ok('a refused request shows the notice', await p.isVisible('.win[data-chat-current] [data-view-id=signin-note]') && (await p.textContent('.win[data-chat-current] [data-view-id=signin-body]')).includes('Log in with browser'));
    await t.shot('signin-required');
    await p.click('.win[data-chat-current] [data-view-id=signin-go]');
    await p.waitForTimeout(100);
    ok('signing in waits for the browser', (await p.textContent('.win[data-chat-current] [data-view-id=signin-go]')) === 'Waiting for the Browser…' && await p.isDisabled('.win[data-chat-current] [data-view-id=signin-go]') && (await p.evaluate(() => window.__signins)) === 1);
    await p.evaluate(() => window.__emit('agent:auth', { state: 'required' }));
    ok('another refusal while waiting does not reset the wait', await p.isDisabled('.win[data-chat-current] [data-view-id=signin-go]'));
    await t.shot('signin-waiting');
    const listed = await p.evaluate(() => window.__listed);
    await p.evaluate(() => window.__finish.ok());
    await p.waitForTimeout(200);
    ok('success hides the notice and lists the sessions again', await p.isHidden('.win[data-chat-current] [data-view-id=signin-note]') && (await p.evaluate(() => window.__listed)) > listed);
    await p.evaluate(() => document.querySelector('[data-cmd="sign-in"]').click());
    await p.waitForTimeout(100);
    ok('the menu entry starts a sign-in', (await p.evaluate(() => window.__signins)) === 2 && await p.isVisible('.win[data-chat-current] [data-view-id=signin-note]'));
    await p.evaluate(() => window.__finish.fail('the browser window was closed'));
    await p.waitForTimeout(150);
    ok('a failure says so and offers to try again', (await p.textContent('.win[data-chat-current] [data-view-id=signin-title]')) === 'The sign-in did not complete.' && (await p.textContent('.win[data-chat-current] [data-view-id=signin-go]')) === 'Try Again' && (await p.textContent('.win[data-chat-current] [data-view-id=signin-body]')).includes('browser window was closed'));
    await p.click('.win[data-chat-current] [data-view-id=signin-go]');
    await p.waitForTimeout(100);
    ok('Try Again starts another', (await p.evaluate(() => window.__signins)) === 3);
    // A provider that signs in with its own program: the note says how,
    // offers no sign-in of the window's own, and can be put away.
    await p.evaluate(() => window.__finish.ok());
    await p.waitForTimeout(200);
    const note = '.win[data-chat-current] [data-view-id=signin-note]';
    await p.evaluate(() => window.__emit('agent:auth', { state: 'required', outside: true, detail: 'Run the program in a terminal and sign in there.' }));
    await p.locator(note).waitFor();
    ok('a provider that signs in outside says how and offers no sign-in', (await p.textContent('.win[data-chat-current] [data-view-id=signin-body]')).includes('in a terminal') && await p.isHidden('.win[data-chat-current] [data-view-id=signin-go]') && await p.isVisible('.win[data-chat-current] [data-view-id=signin-dismiss]'));
    await t.shot('signin-outside');
    await p.click('.win[data-chat-current] [data-view-id=signin-dismiss]');
    ok('Dismiss puts the note away', await p.isHidden(note));
    await p.evaluate(() => window.__emit('agent:auth', { state: 'required', outside: true, detail: 'Run the program in a terminal and sign in there.' }));
    await p.locator(note).waitFor();
    await p.evaluate(() => window.__emit('agent:auth', { state: 'signed-in' }));
    await p.waitForTimeout(100);
    ok('the note goes when the provider is signed in', await p.isHidden(note));
    await p.evaluate(() => window.__emit('agent:auth', { state: 'required', method: 'Log in with browser' }));
    await p.locator(note).waitFor();
    ok('a provider the window signs in to has its button again', await p.isVisible('.win[data-chat-current] [data-view-id=signin-go]'));
  } finally { await t.close(); }
})();
