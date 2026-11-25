// The host clipboard transport for PUDL's edit menu (clipboard-transport.js,
// docs/design/chat-clipboard.md). With the PUDL that Dinah ships today the
// script must do nothing. With a PUDL build that has window.pudlClipboard
// (0.109.0 and later) it must give the menu Dinah's clipboard: ClipboardRead
// for Paste and ClipboardSetText for Copy and Cut, with every failure a
// rejection.
//
// The second half needs a PUDL build that Dinah does not ship yet, and its
// files are never committed here. DINAH_PUDL_BRANCH_DIST is the dist folder
// of such a build, for example one extracted with git archive from the PUDL
// repository. The suite serves that folder's files as pudl/ and falls back
// to Dinah's own for a file the folder lacks. Without it the second half
// prints SKIP and checks nothing.
const fs = require('fs');
const path = require('path');
const { launch, PAGE } = require('./lib/page');

const BRANCH = process.env.DINAH_PUDL_BRANCH_DIST || '';
const DOCUMENT = PAGE.replace(/index\.html$/, 'document.html');

const clipboardApp = () => ({
  // The document page starts by attaching to its window. Here it waits for
  // ever, since only its scripts are being checked, not a document.
  Attach: () => new Promise(() => {}),
  ClipboardRead: async () => {
    const c = window.__clip;
    if (c.mode === 'throw') throw 'the clipboard is busy';
    if (c.mode === 'null') return null;
    return { html: '', text: c.text };
  }
});
const clipboardInit = () => {
  window.__clip = { text: 'pasted words', mode: 'ok' };
  window.__writes = [];
  window.__writeMode = 'ok';
  window.runtime.ClipboardSetText = text => {
    window.__writes.push(text);
    if (window.__writeMode === 'false') return Promise.resolve(false);
    if (window.__writeMode === 'throw') return Promise.reject('the clipboard is locked');
    return Promise.resolve(true);
  };
};

async function shippedPudl() {
  const t = await launch({ app: clipboardApp, init: clipboardInit });
  const { page: p, ok } = t;
  try {
    const state = await p.evaluate(() => {
      const srcs = Array.from(document.scripts).map(s => s.getAttribute('src'));
      return { accessor: typeof window.pudlClipboard, transport: typeof window.dinahClipboardTransport,
        order: srcs.indexOf('clipboard-transport.js') > srcs.indexOf('pudl/pudl-menu.js') && srcs.indexOf('pudl/pudl-menu.js') >= 0 };
    });
    ok('shipped PUDL: the page loads the transport after pudl-menu.js', state.order);
    ok('shipped PUDL: there is no clipboard accessor and none is made', state.accessor === 'undefined', state.accessor);
    ok('shipped PUDL: the transport is not set', state.transport === 'undefined', state.transport);
    ok('shipped PUDL: nothing reads or writes the clipboard', (await t.calls('ClipboardRead')).length === 0 && (await p.evaluate(() => window.__writes.length)) === 0);
    await p.goto(DOCUMENT);
    await p.waitForFunction(() => document.readyState === 'complete');
    const docState = await p.evaluate(() => ({ accessor: typeof window.pudlClipboard, transport: typeof window.dinahClipboardTransport,
      loaded: Array.from(document.scripts).some(s => s.getAttribute('src') === 'clipboard-transport.js') }));
    ok('shipped PUDL: the document page loads the transport and it does nothing', docState.loaded && docState.accessor === 'undefined' && docState.transport === 'undefined', JSON.stringify(docState));
  } finally { await t.close(); }
}

async function branchPudl() {
  const t = await launch({ app: clipboardApp, init: clipboardInit, goto: false });
  const { page: p, ok } = t;
  try {
    await t.context.route('http://dinah.test/pudl/**', route => {
      const inside = new URL(route.request().url()).pathname.replace(/^\/pudl\//, '');
      const file = path.join(BRANCH, inside);
      if (fs.existsSync(file) && fs.statSync(file).isFile()) return route.fulfill({ path: file });
      return route.fallback();
    });
    await p.goto(PAGE);
    await t.waitReady();

    ok('branch PUDL: the accessor exists', await p.evaluate(() => !!(window.pudlClipboard && window.pudlClipboard.setTransport)));
    ok('branch PUDL: the transport is set', await p.evaluate(() => !!window.dinahClipboardTransport));

    // The accessor's own readText and writeText go through the transport.
    const read = await p.evaluate(() => window.pudlClipboard.readText());
    ok('readText gives the text ClipboardRead read', read === 'pasted words', read);
    ok('readText called ClipboardRead', (await t.calls('ClipboardRead')).length === 1);
    const empty = await p.evaluate(async () => { __clip.text = ''; const a = await pudlClipboard.readText(); __clip.mode = 'null'; const b = await pudlClipboard.readText(); __clip.mode = 'ok'; __clip.text = 'pasted words'; return [a, b]; });
    ok('an empty clipboard reads as ""', empty[0] === '' && empty[1] === '', JSON.stringify(empty));
    const readFailure = await p.evaluate(() => { __clip.mode = 'throw'; return pudlClipboard.readText().then(() => 'resolved', e => (e instanceof Error) + ':' + e.message).finally(() => { __clip.mode = 'ok'; }); });
    ok('a failed read rejects with an Error', readFailure === 'true:the clipboard is busy', readFailure);

    const write = await p.evaluate(() => pudlClipboard.writeText('copied words').then(v => ({ v: v === undefined, writes: __writes.slice() })));
    ok('writeText calls ClipboardSetText with the text and resolves with no result', write.v && write.writes.length === 1 && write.writes[0] === 'copied words', JSON.stringify(write));
    const refused = await p.evaluate(() => { __writeMode = 'false'; return pudlClipboard.writeText('x').then(() => 'resolved', e => (e instanceof Error) + ':' + e.message).finally(() => { __writeMode = 'ok'; }); });
    ok('a write the runtime reports as false rejects with an Error', refused === 'true:The clipboard could not be written.', refused);
    const thrown = await p.evaluate(() => { __writeMode = 'throw'; return pudlClipboard.writeText('x').then(() => 'resolved', e => (e instanceof Error) + ':' + e.message).finally(() => { __writeMode = 'ok'; }); });
    ok('a write that throws rejects with an Error', thrown === 'true:the clipboard is locked', thrown);

    // Loading the script a second time sets nothing.
    const again = await p.evaluate(() => new Promise(resolve => {
      let n = 0; const set = pudlClipboard.setTransport;
      pudlClipboard.setTransport = function () { n++; return set.apply(this, arguments); };
      const s = document.createElement('script'); s.src = 'clipboard-transport.js?again';
      s.onload = () => { pudlClipboard.setTransport = set; resolve(n); }; s.onerror = () => resolve(-1);
      document.head.appendChild(s);
    }));
    ok('a second run of the script sets the transport once only', again === 0, String(again));

    // PUDL's edit menu on a plain input, opened from the keyboard.
    await p.evaluate(() => {
      const box = document.createElement('div');
      box.style.cssText = 'position:fixed;left:40px;top:120px;z-index:2147483000';
      box.innerHTML = '<input id="edit-probe" type="text" aria-label="Probe">';
      document.body.appendChild(box);
      window.__editErrors = [];
      document.addEventListener('pudl:edit-error', e => { __editErrors.push(e.detail.operation + ':' + e.detail.error.message); e.preventDefault(); });
    });
    const field = p.locator('#edit-probe');
    const choose = async (label, value, start, end) => {
      await p.evaluate(([v, a, b]) => { const f = document.getElementById('edit-probe'); f.value = v; f.focus(); f.setSelectionRange(a, b); }, [value, start, end]);
      await p.keyboard.press('Shift+F10');
      const row = p.locator('.context-menu .menu-action', { hasText: new RegExp('^' + label) });
      await row.first().waitFor({ state: 'visible' });
      await row.first().click();
      await p.waitForTimeout(150);
    };
    await choose('Copy', 'hello world', 6, 11);
    ok('Copy writes the selection', await p.evaluate(() => __writes[__writes.length - 1] === 'world'), JSON.stringify(await p.evaluate(() => __writes)));
    ok('Copy leaves the field alone', (await field.inputValue()) === 'hello world');

    await p.evaluate(() => { __clip.text = 'PASTE'; });
    await choose('Paste', 'hello world', 6, 11);
    ok('Paste inserts the clipboard text', (await field.inputValue()) === 'hello PASTE', await field.inputValue());

    const writesBefore = await p.evaluate(() => __writes.length);
    await choose('Cut', 'hello world', 6, 11);
    ok('Cut writes the selection', await p.evaluate(n => __writes.length === n + 1 && __writes[n] === 'world', writesBefore));
    ok('Cut deletes the selection after the write', (await field.inputValue()) === 'hello ', await field.inputValue());

    await p.evaluate(() => { __writeMode = 'false'; __editErrors = []; });
    await choose('Cut', 'hello world', 6, 11);
    ok('Cut keeps the text when the write is refused', (await field.inputValue()) === 'hello world', await field.inputValue());
    ok('the refused Cut is reported as an edit error', await p.evaluate(() => __editErrors.length === 1 && __editErrors[0] === 'cut:The clipboard could not be written.'), JSON.stringify(await p.evaluate(() => __editErrors)));
    await p.evaluate(() => { __writeMode = 'ok'; });

    // A document window's page sets its own transport.
    await p.goto(DOCUMENT);
    await p.waitForFunction(() => document.readyState === 'complete');
    const doc = await p.evaluate(() => window.dinahClipboardTransport && window.pudlClipboard ? pudlClipboard.readText() : 'no transport');
    ok('the document page sets its own transport', doc === 'pasted words', doc);
  } finally { await t.close(); }
}

(async () => {
  await shippedPudl();
  if (!BRANCH || !fs.existsSync(path.join(BRANCH, 'pudl-menu.js'))) {
    console.log('SKIP the PUDL edit-menu checks: set DINAH_PUDL_BRANCH_DIST to the dist folder of a PUDL build with window.pudlClipboard (0.109.0 or later).');
    return;
  }
  await branchPudl();
})().catch(error => { console.error(error); process.exitCode = 1; });
