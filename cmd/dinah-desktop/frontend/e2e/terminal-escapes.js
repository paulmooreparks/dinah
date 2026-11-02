// Text that reaches the built-in Terminal from someone else cannot drive the
// terminal. A file's name, a tool's description, a Lua script's output and
// a saved transcript shown again each carry escape sequences that would
// clear the screen, erase the line above, move the cursor, set the title or
// the clipboard, ring the bell, or overwrite the line with a carriage
// return. Afterwards the lines written before are still on the screen, the
// hostile text is shown with its escapes as a visible symbol, and the
// Terminal's own colours still work (terminal-applet.js, screen and safe).
const { launch, FRONTEND } = require('./lib/page');
const path = require('path');

const E = '\x1b';
// Invisible marks: zero-width, directional, a line separator and the byte
// order mark, each to be shown as its code point.
const MARKS = 'ZW​LRM‎RLO‮LS BOM﻿END';
const SHOWN = ['<U+200B>', '<U+200E>', '<U+202E>', '<U+2028>', '<U+FEFF>'];
// Every way out at once: escape sequences, a carriage return that would put
// OVERWRITE over the start of the line, and the invisible marks.
const HOSTILE = 'EVIL' + E + '[2J' + E + '[H' + E + '[1A' + E + '[2K' + E + ']0;Owned' + '\x07' + E + ']52;c;b3duZWQ=' + '\x07' + E + '[8mHIDDEN' + E + '[0m' + E + 'P1$r' + E + '\\' + '\x9b2J' + 'SAFE\rOVERWRITE' + MARKS;

(async () => {
  const t = await launch({
    viewport: { width: 1100, height: 760 },
    timeout: 20000,
    app: () => {
      const fsMap = { '/home': { dir: true }, '/home/pparks': { dir: true, local: true }, '/home/pparks/plain.md': { text: 'plain\n', local: true }, '/workspaces': { dir: true } };
      window.__fs = fsMap;
      const norm = p => p === '~' ? '/home/pparks' : p.startsWith('~/') ? '/home/pparks' + p.slice(1) : p;
      const parent = p => p.slice(0, p.lastIndexOf('/')) || '/';
      const item = p => { const e = fsMap[p]; if (!e) throw new Error('no such file or folder'); return { path: p, title: p.split('/').pop(), kind: e.dir ? 'directory' : 'file', directory: !!e.dir, revision: e.text != null ? 'r' + e.text.length : '', size: e.text ? e.text.length : 0, modified: '2026-11-02T00:00:00Z', writable: true, actions: e.dir ? ['read'] : ['read', 'edit'] }; };
      const A = {
        BeginExitWork: async () => '1', EndExitWork: async () => true,
        StatWorkspaceMount: async p => item(norm(p)),
        ListWorkspaceMounts: async (p, start, limit) => { p = norm(p); const all = Object.keys(fsMap).filter(k => k !== p && parent(k) === p).sort(); return { entries: all.slice(start, start + limit).map(item), nextStart: start + limit < all.length ? start + limit : -1 }; },
        ReadWorkspaceMount: async p => { p = norm(p); const e = fsMap[p]; if (!e || e.text == null) throw new Error('no such file'); return { resource: item(p), text: e.text, format: 'markdown', revision: 'r' + e.text.length }; },
        WriteWorkspaceMount: async (p, text) => { fsMap[p] = { text, local: true }; return item(p); },
        Log: async () => {}
      };
      A.StatWorkspaceMountExpected = p => A.StatWorkspaceMount(p);
      A.ListWorkspaceMountsExpected = (p, s, l) => A.ListWorkspaceMounts(p, s, l);
      A.ReadWorkspaceMountExpected = p => A.ReadWorkspaceMount(p);
      return A;
    },
    init: () => {
      window.__saved = {};
      window.dinahUIState = { ready: Promise.resolve(), get: (k, f) => window.__saved[k] || f, set: () => {}, flush: async () => {} };
      window.dinahResources = { commands: () => [{ name: 'hostile', description: window.__hostile || '', usage: 'hostile' }], invoke: async () => '{}', subscribe: () => () => {} };
    },
    goto: false,
  });
  const { page, ok } = t;
  try {
    await t.context.addInitScript(([hostile]) => {
      window.__hostile = hostile;
      window.__fs['/home/pparks/' + hostile + '.md'] = { text: 'x\n', local: true };
    }, [HOSTILE]);
    await t.context.route('http://dinah.test/**', route => {
      const pathname = new URL(route.request().url()).pathname;
      if (pathname === '/fixture.html') return route.fulfill({ contentType: 'text/html', body: '<html><head><link rel="stylesheet" href="terminal.css"></head><body style="height:700px"><div id="one" style="width:900px;height:600px"></div><div id="two" style="width:900px;height:600px"></div><script src="helpers.js"></script><script src="regional.js"></script><script src="terminal-applet.js"></script><script src="terminal-lua.js"></script></body></html>' });
      if (pathname.endsWith('.wasm')) return route.fulfill({ path: path.join(FRONTEND, pathname), contentType: 'application/wasm' });
      return route.fallback();
    });
    await page.goto('http://dinah.test/fixture.html');
    await page.evaluate(() => { document.getElementById('one').setAttribute('data-win', 'one'); dinahTerminal.init(document.querySelector('#one'), {}, { path: '/home/pparks', instanceId: 'one' }); });
    await page.locator('#one[data-terminal-ready="true"]').waitFor();
    const idle = id => page.waitForFunction(id => document.querySelector('#' + id).dataset.terminalBusy === 'false', id);
    async function command(text) { await page.locator('#one textarea').focus(); await page.keyboard.type(text); await page.keyboard.press('Enter'); await idle('one'); await page.waitForTimeout(250); }
    const screen = id => page.locator('#' + id + ' .xterm-rows').textContent().then(s => s.replace(/ /g, ' '));
    const squeeze = s => s.replace(/\s+/g, '');

    // Each route, with a line written just before it that must survive it.
    async function route(name, text) {
      await command('clear');
      await command('echo KEEP-' + name);
      await command(text);
      const s = await screen('one');
      ok(name + ': the line before is still on the screen', s.includes('KEEP-' + name), s.slice(0, 400));
      ok(name + ': the hostile text is shown, its escapes as a symbol and its carriage return gone', squeeze(s).includes('EVIL\u241b[2J') && squeeze(s).includes('SAFEOVERWRITE'), s.slice(0, 600));
      ok(name + ': invisible marks are shown as their code points', SHOWN.every(m => squeeze(s).includes(m)), s.slice(0, 700));
      ok(name + ': the concealed text is not hidden', s.includes('HIDDEN') && await page.evaluate(() => [...document.querySelectorAll('#one .xterm-rows span')].filter(el => el.textContent.includes('HIDDEN')).every(el => getComputedStyle(el).opacity !== '0' && getComputedStyle(el).visibility !== 'hidden' && !el.classList.contains('xterm-invisible'))));
    }
    await route('a file name', 'ls');
    await route('a tool description', 'man hostile');
    await route('Lua output', "lua -e 'io.write(string.char(27) .. \"[0m\" .. " + JSON.stringify(HOSTILE).replace(/\\u001b/g, '\\27').replace(/\\u0007/g, '\\7').replace(/\\u009b/g, '\\194\\155').replace(/[\u200b-\u200f\u2028\u2029\u202a-\u202e\u2066-\u2069\ufeff]/g, ch => '\\u{' + ch.charCodeAt(0).toString(16) + '}') + " .. \"\\n\")'");
    ok('the Terminal\'s own colours still work', await page.evaluate(() => [...document.querySelectorAll('#one .xterm-rows span')].some(el => /xterm-fg-\d|xterm-bold/.test(el.className))));

    // A saved transcript shown again passes the same barrier.
    await page.evaluate(h => {
      window.__saved['dinah-terminal:two'] = { cwd: '/home/pparks', output: 'KEEP-restored\n' + h + '\n', history: [h] };
      document.getElementById('two').setAttribute('data-win', 'two');
      dinahTerminal.init(document.querySelector('#two'), {}, { path: '/home/pparks', instanceId: 'two' });
    }, HOSTILE);
    await page.locator('#two[data-terminal-ready="true"]').waitFor();
    await page.waitForTimeout(300);
    const restored = await screen('two');
    ok('a restored transcript keeps the line before and shows the hostile text inert', restored.includes('KEEP-restored') && SHOWN.every(m => squeeze(restored).includes(m)) && squeeze(restored).includes('EVIL\u241b[2J') && squeeze(restored).includes('SAFEOVERWRITE'), restored.slice(0, 600));
    await page.locator('#two textarea').focus();
    await page.keyboard.press('ArrowUp');
    await page.waitForTimeout(250);
    const recalled = await screen('two');
    ok('a recalled command from the saved history is inert', recalled.includes('KEEP-restored') && squeeze(recalled).includes('EVIL\u241b[2J'), recalled.slice(0, 600));
  } catch (error) {
    ok('the suite ran', false, String(error && error.stack || error));
  } finally {
    await t.close();
  }
})();
