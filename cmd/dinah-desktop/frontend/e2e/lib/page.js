// One stubbed window for the browser suites.
//
// Every suite drives ../index.html in Edge with window.go.main.App and
// window.runtime stubbed. Before this file each suite carried its own copy
// of that stub, so a change to a bridge method's shape was an edit across
// many suites. A suite now calls launch() with only what is its own: the
// methods it answers differently, the State it wants, and when the page
// counts as ready.
//
//   const { launch } = require('./lib/page');
//   const t = await launch({
//     viewport: { width: 1280, height: 900 },
//     state: { defaultProvider: 'codex' },
//     app: () => ({ OpenSessionAt: async (ws, id) => ({ sessionId: id, cwd: ws, open: true }) }),
//     ready: 'workbench',
//   });
//   try { ... t.ok('what was checked', condition); ... } finally { await t.close(); }
//
// app is a function that runs in the page before any of Dinah's scripts, and
// the object it returns is laid over the default App below. It must stand
// alone, since it is sent to the page as source: it may use window.__emit,
// window.__bridgeCalls and window.__state, which the stub defines first, and
// nothing from the suite's own scope. A suite that needs more in the page
// passes init, another standalone function, which runs after the stub. A
// value built in Node reaches both as their one argument through args,
// which must be JSON. The stub runs only in the page itself, never in an
// extension's frame, which sees no bridge in the real window either.
//
// In the page:
//   window.__emit(name, data)   fires a runtime event, as Go would.
//   window.__drop(name, n)      swallows the next n emits of an event, as a
//                               stream that lost them would.
//   window.__resync()           what the bridge sends when it could not
//                               replay what the page missed (dinah:resync).
//   window.__bridgeCalls        every App call as [method, ...args]. (Many
//                               suites keep a window.__calls of their own
//                               shape, which this leaves alone.)
//   window.__state              the object State() returns; change it and
//                               the next State() shows the change.
//   window.runtime.EventsOn/EventsOnce/EventsOff/EventsEmit, Quit,
//   BrowserOpenURL and ClipboardSetText are stubbed; the last three record
//   their arguments in window.__runtime.
//
// From the suite:
//   t.page, t.context, t.browser   Playwright's.
//   t.emit(name, data)             fires an event in the page.
//   t.calls(method?)               the recorded calls, or those of one method.
//   t.ok(name, pass, detail?)      prints PASS or FAIL and remembers failures.
//   t.shot(name, options?)         a screenshot in the run's output folder,
//                                  with Playwright's options (fullPage…).
//   t.errors                       page errors and browser dialogs so far.
//   t.close()                      prints "no page errors", closes the
//                                  browser, and exits 1 on any failure.
const { chromium } = require('playwright-core');
const path = require('path');
const { acquire: acquireBrowserLock } = require('./browser-lock');

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe';
const FRONTEND = path.join(__dirname, '..', '..');
const PAGE = 'http://dinah.test/index.html';
// run.ps1 gives the output folder as the third argument; a suite run by
// hand without one writes beside the temporary folder's other runs rather
// than into a folder called "undefined".
const OUT = process.argv[3] || path.join(require('os').tmpdir(), 'dinah-e2e');

// What the page is given before Dinah's scripts run. This function is sent
// to the page as source with its argument, so it uses nothing from Node.
function stub({ state, app, init, args }) {
  // Only the page itself has the bridge. An extension's sandboxed frame
  // sees no window.go in the real window, and the suites check that.
  if (window !== window.parent) return;
  const handlers = {};
  window.__bridgeCalls = [];
  window.__runtime = [];
  window.__state = Object.assign({ workspace: 'C:/Fixture', recent: ['C:/Fixture'], defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [] }, state);
  /* A provider's entry as the engine's list gives it (providers.go, provider_registry.go): its name, its
     facts as the table declares them, and the status words for what is wrong. A suite takes one with
     window.__provider(id, changes) and changes what its case needs. */
  const providerFacts = {
    devin: { imageInput: 'none', removal: 'deletes', prices: true, usage: 'link', ownCommands: true, takesSkills: true, executable: 'Devin', chooseExecutable: true, signIn: true },
    codex: { imageInput: 'initialize', removal: 'hides', prices: false, usage: 'read', ownCommands: false, takesSkills: true, executable: 'Codex', chooseExecutable: true, signIn: true },
    claude: { imageInput: 'initialize', removal: 'deletes', prices: false, usage: 'none', ownCommands: false, takesSkills: false, executable: 'Node.js', chooseExecutable: true, signIn: false },
    ollama: { imageInput: 'adapter', removal: 'deletes', prices: false, usage: 'none', ownCommands: false, takesSkills: false, executable: '', chooseExecutable: false, signIn: false }
  };
  const providerNames = { devin: 'Devin', codex: 'Codex', claude: 'Claude', ollama: 'Ollama' };
  window.__provider = (id, changes) => Object.assign({ id, name: providerNames[id] || id, installed: true, available: true, status: 'detected', statusWords: '', default: false, facts: providerFacts[id] || {} }, changes);
  // __drop(name, n) swallows the next n emits of an event, as a stream that lost
  // them would; __resync() is what the bridge sends when it could not replay.
  const drops = {};
  window.__drop = (name, n = 1) => { drops[name] = (drops[name] || 0) + n; };
  window.__emit = (name, data) => {
    if (drops[name] > 0) { drops[name]--; return; }
    (handlers[name] || []).slice().forEach(fn => fn(data));
  };
  window.__resync = () => window.__emit('dinah:resync', {});
  window.runtime = {
    EventsOn(name, fn) { (handlers[name] ||= []).push(fn); return () => window.runtime.EventsOff(name, fn); },
    EventsOnce(name, fn) { const once = data => { window.runtime.EventsOff(name, once); fn(data); }; return window.runtime.EventsOn(name, once); },
    EventsOff(name, fn) { if (!handlers[name]) return; handlers[name] = fn ? handlers[name].filter(h => h !== fn) : []; },
    EventsEmit(name, data) { window.__emit(name, data); },
    Quit() { window.__runtime.push(['Quit']); },
    BrowserOpenURL(url) { window.__runtime.push(['BrowserOpenURL', url]); },
    ClipboardSetText(text) { window.__runtime.push(['ClipboardSetText', text]); },
  };
  let uiState = { version: 1, values: {} };
  const none = async () => [];
  const App = {
    State: async () => window.__state,
    Sessions: none, SessionsAt: none, History: none, SessionTimes: none,
    SkillsAt: none, SkillsForProviderAt: none, Skills: none, Templates: none,
    AgentProviders: async () => [{ id: 'codex', name: 'Codex', installed: true, available: true }],
    ProviderAccounts: none, ProviderModelPreferences: async () => ({}),
    ReadUIState: async () => uiState, SaveUIState: async value => { uiState = value; }, AckUIStateFlush: async () => {},
    RecentDocuments: none, ListDrafts: none, Notifications: none,
    ReportWindowFocus: async () => {}, Log: async () => {},
    HostLocale: async () => ({ tag: 'en-US' }),
    BusOverview: async () => ({ inbox: [], rooms: [], workspaces: [], links: [], unread: 0, defaultHops: 3, defaultMax: 3, noHopLimit: -1 }),
    BusResolveGroup: async key => {
      const rooms = (await App.BusOverview()).rooms || [];
      const address = String(key).trim().replace(/^#/, '').trim();
      const room = rooms.find(r => r.name === address.toLowerCase()) || rooms.find(r => String(r.title || r.name).toLowerCase() === address.toLowerCase());
      return room ? { state: 'resolved', room } : { state: 'not-found' };
    },
    AppletCatalog: none, AppletRequests: none, AppletResults: none, AppletAttention: async () => ({}),
    Extensions: none, TerminalShells: none, TerminalProfiles: none,
    UpdateState: async () => ({}), NativeWindowState: async () => ({}),
  };
  const own = app ? app(args) : {};
  Object.assign(App, own);
  // Every call is recorded with its arguments, whichever stub answers it.
  for (const name of Object.keys(App)) {
    const fn = App[name];
    App[name] = (...args) => { window.__bridgeCalls.push([name, ...args]); return fn(...args); };
  }
  window.go = { main: { App } };
  if (init) init(args);
}

// The page is ready when: 'ui-state', the saved UI state has been read and
// the layout restored (the usual choice, since boxes read their drafts and
// windows their places only then); 'workbench', the workbench and menu bar
// exist; 'chat', a chat window exists; or a function run in the page until
// it is true.
const READY = {
  'ui-state': () => document.documentElement.dataset.uiStateReady === 'true',
  workbench: () => window.dinahWorkbench && window.dinahHost && document.querySelector('.menubar-ready'),
  chat: () => window.dinahWorkbench && document.querySelector('.chat-instance'),
};

async function launch(options = {}) {
  const { viewport = { width: 1280, height: 900 }, state = {}, app, init, args = null, ready = 'ui-state', dialogs = 'fail', goto = true, timeout = 10000, expectErrors = null } = options;
  const releaseLock = await acquireBrowserLock();
  let browser;
  // Native scrollbar input requires the control that Playwright normally hides
  // in headless runs. The public launch option filters that default argument:
  // https://playwright.dev/docs/api/class-browsertype#browser-type-launch-option-ignore-default-args
  try { browser = await chromium.launch({ executablePath: EDGE, ignoreDefaultArgs: options.nativeScrollbars ? ['--hide-scrollbars'] : undefined }); }
  catch (error) { await releaseLock(); throw error; }
  let t;
  try {
  // expectErrors is the message of a page error a suite causes on purpose; such an error is kept in expected,
  // and is not printed or counted as one the page should not have.
  const errors = [], failures = [], expected = [];
  const context = await browser.newContext({ viewport });
  await context.route('http://dinah.test/**', route => route.fulfill({ path: path.join(FRONTEND, new URL(route.request().url()).pathname) }));
  // The stub and the suite's functions are sent as one script's source, so
  // nothing is evaluated from a string in the page and a page served under
  // its real Content Security Policy runs them as any init script.
  await context.addInitScript({ content: '(' + stub.toString() + ')({ state: ' + JSON.stringify(state) + ', args: ' + JSON.stringify(args) + ', app: ' + (app ? app.toString() : 'null') + ', init: ' + (init ? init.toString() : 'null') + ' });' });
  const page = await context.newPage();
  page.setDefaultTimeout(timeout);
  page.on('pageerror', error => {
    if (expectErrors && expectErrors.test(String(error))) { expected.push(String(error)); return; }
    errors.push(String(error)); console.log(error.stack || String(error));
  });
  page.on('dialog', dialog => {
    if (dialogs === 'fail') errors.push('Browser dialog: ' + dialog.message());
    dialog.dismiss().catch(() => {});
  });
  t = {
    browser, context, page, errors, failures, expected,
    ok(name, pass, detail) {
      pass = !!pass;
      console.log((pass ? 'PASS ' : 'FAIL ') + name + (pass || detail === undefined ? '' : ' :: ' + detail));
      if (!pass) failures.push(name);
      return pass;
    },
    emit: (name, data) => page.evaluate(([n, d]) => window.__emit(n, d), [name, data]),
    calls: method => page.evaluate(m => m ? window.__bridgeCalls.filter(c => c[0] === m) : window.__bridgeCalls, method || null),
    shot: (name, options) => page.screenshot(Object.assign({ path: path.join(OUT, name.endsWith('.png') ? name : name + '.png') }, options)),
    async waitReady(which = ready) {
      if (which === false) return;
      await page.waitForFunction(typeof which === 'function' ? which : READY[which] || READY['ui-state']);
    },
    /* page.evaluate and browser.close take no timeout, neither their own nor the
       page's default: a page promise that never settles, or a browser that never
       exits, hangs its suite forever, as ui-state hung for over twenty minutes
       on 30 October 2026. deadline bounds such a wait and names what it waited for. */
    deadline(name, promise, ms = timeout) {
      /* The timer would hold node's event loop for its full length after the
         wait settled, which put 45 s on every suite's exit (the 828 s gate,
         30 October 2026), so it is cancelled as the wait ends. */
      let timer;
      const bound = new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('Timed out after ' + ms + ' ms waiting for ' + name)), ms); });
      return Promise.race([Promise.resolve(promise).then(value => { clearTimeout(timer); return value; }, error => { clearTimeout(timer); throw error; }), bound]);
    },
    async close() {
      t.ok('no page errors', errors.length === 0, errors.join(' | '));
      /* browser.close() has no deadline of its own, and a wedged browser must
         never hold a suite (30 October 2026). t.deadline names the wait. */
      await t.deadline('the browser to close', browser.close(), 45000).catch(error => {
        t.ok('the browser closes', false, String(error));
        try { const proc = browser.process(); if (proc) proc.kill(); } catch (killError) {}
      });
      await releaseLock();
      if (failures.length) process.exitCode = 1;
    },
  };
  if (goto) {
    await page.goto(PAGE);
    await t.waitReady();
  }
  return t;
  } catch (error) {
    try {
      if (t) await t.close();
      else {
        let timer;
        try {
          await Promise.race([browser.close(), new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('Timed out closing the browser after setup failed')), 45000); })]);
        } finally { clearTimeout(timer); }
      }
    } finally { await releaseLock(); }
    throw error;
  }
}

module.exports = { launch, EDGE, PAGE, OUT, FRONTEND };
