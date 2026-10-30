// The POSIX shell in the Terminal: sh -c
// and .sh scripts run by mvdan.cc/sh, built for WebAssembly from
// tools/shwasm and run in a worker; the shell language itself; Terminal
// commands, tools and Lua scripts run from it, in pipelines and $( ); files
// through the Terminal; a script's own folder and variables; PATH; source;
// ~/.profile when a terminal starts; and Ctrl+C.
const { launch, FRONTEND } = require('./lib/page');
const path = require('path');
const fs = require('fs');
(async () => {
  // The system folders, as the app installs them.
  const system = {};
  const root = path.join(__dirname, '..', '..', 'system');
  for (const top of ['bin', 'lib']) {
    (function walk(dir, at) {
      system[at] = null;
      for (const name of fs.readdirSync(dir)) {
        const full = path.join(dir, name), p = at + '/' + name;
        if (fs.statSync(full).isDirectory()) walk(full, p); else system[p] = fs.readFileSync(full, 'utf8');
      }
    })(path.join(root, top), '/' + top);
  }
  const t = await launch({
    viewport: { width: 1100, height: 760 },
    timeout: 30000,
    app: () => {
      const fsMap = {
        '/home': { dir: true }, '/home/pparks': { dir: true, local: true },
        '/home/pparks/notes.md': { text: 'alpha\nbeta\ngamma\n', local: true },
        '/home/pparks/bin': { dir: true, local: true },
        '/workspaces': { dir: true }, '/workspaces/parser': { dir: true }
      };
      const norm = p => p === '~' ? '/home/pparks' : p.startsWith('~/') ? '/home/pparks' + p.slice(1) : p;
      const parent = p => p.slice(0, p.lastIndexOf('/')) || '/';
      const item = p => { const e = fsMap[p]; if (!e) throw new Error('no such file or folder'); return { path: p, title: p.split('/').pop(), kind: e.dir ? 'directory' : 'file', directory: !!e.dir, revision: e.text != null ? 'r' + e.text.length : '', actions: e.dir ? ['list'] : ['read', 'preview', 'edit'], ref: p.startsWith('/home/pparks') ? { workspaceId: '', sourceId: 'dinah-home', path: p.slice('/home/pparks/'.length) } : undefined, size: (e.text || '').length, writable: !!e.local }; };
      const writable = p => /^\/(home\/pparks|bin|lib)(\/|$)/.test(p);
      window.__fs = fsMap;
      const A = {
        StatWorkspaceMount: async p => item(norm(p)),
        ListWorkspaceMounts: async (p, start, limit) => { p = norm(p); const all = Object.keys(fsMap).filter(k => k !== p && parent(k) === p).sort(); return { entries: all.slice(start, start + limit).map(item), nextStart: start + limit < all.length ? start + limit : -1 }; },
        ReadWorkspaceMount: async p => { p = norm(p); const e = fsMap[p]; if (!e || e.text == null) throw new Error('no such file'); return { resource: item(p), text: e.text, format: 'markdown', revision: 'r' + e.text.length }; },
        WriteWorkspaceMount: async (p, text, mode) => { if (!writable(p)) throw new Error(p + ' is a catalog of workspaces and sources, which cannot be changed here'); fsMap[p] = { text, local: true }; return item(p); },
        MakeWorkspaceDirectory: async p => { fsMap[p] = { dir: true, local: true }; return item(p); },
        RemoveWorkspaceMount: async p => { delete fsMap[p]; return 1; },
        MoveWorkspaceMount: async (a, _e, b) => { fsMap[b] = fsMap[a]; delete fsMap[a]; return item(b); },
        Log: async () => {}
      };
      A.StatWorkspaceMountExpected = (p) => A.StatWorkspaceMount(p);
      A.ListWorkspaceMountsExpected = (p, s, l) => A.ListWorkspaceMounts(p, s, l);
      A.ReadWorkspaceMountExpected = (p) => A.ReadWorkspaceMount(p);
      return A;
    },
    init: () => {
      window.dinahUIState = { ready: Promise.resolve(), get: (k, f) => f, set: () => {}, flush: async () => {} };
      window.__calls = [];
      window.dinahAppletSDK = { catalog: async () => [] };
      window.dinahResources = {
        commands: () => [{ name: 'applets', description: 'Discover applet actions and invoke them.', usage: 'applets ...' }],
        // A test may hold a tool's answer back, to stop its line while it waits.
        invoke: async (name, args, current) => { if (window.__hold) { window.__held = true; await window.__hold; } if (current()) __calls.push(['invoke', name, args]); return JSON.stringify({}); },
        subscribe: () => () => {}
      };
    },
    goto: false,
  });
  const { page, ok, errors } = t;
  try {
    // The fixture's init takes no argument, so the system folders read in
    // Node reach the page through a second init script, run after the stub
    // has made window.__fs.
    await t.context.addInitScript(system => { for (const [p, text] of Object.entries(system)) window.__fs[p] = text == null ? { dir: true } : { text }; }, system);
    // The page is a fixture of the terminal scripts alone, and the WebAssembly
    // module is served as such; everything else comes from the shared route.
    await t.context.route('http://dinah.test/**', route => {
      const pathname = new URL(route.request().url()).pathname;
      if (pathname === '/fixture.html') return route.fulfill({ contentType: 'text/html', body: '<html><head><link rel="stylesheet" href="terminal.css"></head><body style="height:700px"><div id="one" style="width:900px;height:600px"></div><div id="two" style="width:900px;height:600px"></div><script src="helpers.js"></script><script src="regional.js"></script><script src="terminal-applet.js"></script><script src="terminal-lua.js"></script><script src="terminal-sh.js"></script></body></html>' });
      if (pathname.endsWith('.wasm')) return route.fulfill({ path: path.join(FRONTEND, pathname), contentType: 'application/wasm' });
      return route.fallback();
    });
    await page.goto('http://dinah.test/fixture.html');
    async function start(id) {
      await page.evaluate(id => { document.getElementById(id).setAttribute('data-win', id); window['__' + id] = dinahTerminal.init(document.getElementById(id), {}, { path: '/home/pparks', instanceId: id }); }, id);
      await page.locator('#' + id + '[data-terminal-ready="true"]').waitFor();
    }
    await start('one');
    const idle = (id = 'one') => page.waitForFunction(id => document.getElementById(id).dataset.terminalBusy !== 'true', id);
    async function command(text, id = 'one') { await page.locator('#' + id + ' textarea').focus(); await page.keyboard.type(text); await page.keyboard.press('Enter'); await page.waitForTimeout(50); await idle(id); }
    const screen = (id = 'one') => page.locator('#' + id + ' .xterm-rows').textContent();
    // xterm draws what was written in a later frame. The prompt is written
    // after a command's output, so once the cleared screen shows the command
    // after the first prompt and ends with the next one, the output is drawn
    // too. Spaces are left out of the comparison, since a long line wraps.
    const drawn = (text, id) => page.waitForFunction(([text, id]) => {
      const squeeze = s => s.replace(/[\s ]+/g, '');
      const s = squeeze(document.querySelector('#' + id + ' .xterm-rows').textContent), at = s.indexOf('$');
      return at >= 0 && s.startsWith(squeeze(text), at + 1) && (s.match(/@dinah:/g) || []).length >= 2 && s.endsWith('$');
    }, [text, id]);
    async function run(text, id = 'one') { await command('clear', id); await command(text, id); await drawn(text, id); return (await screen(id)).replace(/ /g, ' '); }
    // The status the last line left, read without clearing the screen first,
    // because clear is a command and sets $? itself. The check waits for the
    // answer to be drawn after the line that asks for it.
    async function status() {
      await command('echo status $?');
      await page.waitForFunction(() => {
        const s = document.querySelector('#one .xterm-rows').textContent.replace(/[\s ]+/g, ''), at = s.lastIndexOf('echostatus$?');
        return at >= 0 && /^echostatus\$\?status\d+.*\$$/.test(s.slice(at));
      });
      return [...(await screen()).replace(/ /g, ' ').matchAll(/status (\d+)/g)].map(m => m[1]).pop(); }

    ok('the shell module takes over sh', await page.evaluate(() => dinahTerminal.interpreters().some(i => i.id === 'sh') && dinahTerminal.commands().includes('sh')));
    let out = await run("sh -c 'echo $((6 * 7)); for i in 1 2 3; do printf \"n%s \" $i; done; echo'");
    ok('sh -c runs arithmetic and a for loop', out.includes('42') && out.includes('n1 n2 n3'), out);
    out = await run("sh -c 'greet() { echo \"hello, $1\"; }; case $1 in a*) greet apples;; *) greet other;; esac' sh apricot");
    ok('functions, case and arguments work', out.includes('hello, apples'), out);
    out = await run("sh -c 'x=$(echo stressed | rev); echo got $x; echo shout | tr a-z A-Z'");
    ok('Terminal commands and Lua scripts run in $( ) and in pipelines', out.includes('got desserts') && out.includes('SHOUT'), out);
    await page.evaluate(() => { __fs['/home/pparks/input.sh'] = { text: 'wc -l < ~/notes.md\ngrep gam ~/notes.md\ncat <<EOF\nhere $USER\nEOF\n', local: true }; });
    out = await run('sh ~/input.sh');
    ok('redirection and here-documents give a command its input, and ~ is the home folder', /3[\s\S]*gamma[\s\S]*here pparks/.test(out), out);
    await run("sh -c 'echo saved > ~/out.txt; echo more >> ~/out.txt; cat ~/out.txt'");
    ok('> and >> write a file', await page.evaluate(() => (__fs['/home/pparks/out.txt'] || {}).text === 'saved\nmore\n'));
    out = await run("sh -c 'if [ -f ~/notes.md ] && [ -d /bin ] && [ ! -e ~/nothing ]; then echo tests-pass; fi; echo /bin/se*'");
    ok('test and globbing see Dinah\'s files', out.includes('tests-pass') && out.includes('/bin/seq'), out);
    out = await run("sh -c 'cd /workspaces/parser && pwd'; pwd");
    ok('cd in a script changes the script\'s folder and not the prompt\'s', /\/workspaces\/parser\s*\/home\/pparks/.test(out), out);
    await page.evaluate(() => { __fs['/home/pparks/bin/child.sh'] = { text: 'echo "child sees $GREETING in $(pwd)"\n', local: true }; });
    out = await run("sh -c 'export GREETING=hi; cd /workspaces/parser; child'");
    ok('a command a script runs sees its exported variables and its folder', out.includes('child sees hi in /workspaces/parser'), out);
    out = await run("sh -c 'false || echo recovered; exit 3'; echo status $?");
    ok('|| and exit statuses work', out.includes('recovered') && out.includes('status 3'), out);
    out = await run("sh -c 'nosuchcommand'; echo status $?");
    ok('a command that is not found says so with status 127', out.includes('nosuchcommand') && out.includes('status 127'), out);

    // PATH: ~/bin, then /bin, unless it is changed.
    out = await run('echo $PATH');
    ok('PATH is ~/bin then /bin', out.includes('/home/pparks/bin:/bin'), out);
    await page.evaluate(() => { __fs['/home/pparks/bin/seq'] = { text: 'echo my own seq\n', local: true }; });
    ok('a script in ~/bin comes before /bin', (await run('seq 2')).includes('my own seq'));
    out = await run('export PATH=/bin; seq 2; child');
    ok('PATH decides where names are found', /1\s*2/.test(out) && out.includes('child: command not found'), out);
    await command('unset PATH');
    ok('unsetting PATH returns to the default', (await run('echo $PATH')).includes('/home/pparks/bin:/bin'));
    await page.evaluate(() => { delete __fs['/home/pparks/bin/seq']; });

    // source keeps what a script sets.
    await page.evaluate(() => { __fs['/home/pparks/vars.sh'] = { text: 'export PROJECT=parser\ncd /workspaces/parser\n', local: true }; });
    out = await run('sh ~/vars.sh; echo [$PROJECT]; pwd');
    ok('a script run on its own leaves the prompt as it was', out.includes('[]') && out.includes('/home/pparks'), out);
    out = await run('source ~/vars.sh; echo [$PROJECT]; pwd');
    ok('source keeps a script\'s variables and folder', out.includes('[parser]') && out.includes('/workspaces/parser'), out);
    await command('cd ~; unset PROJECT');

    // A .sh script by its name, and a standard one written for the shell.
    await page.evaluate(() => { __fs['/home/pparks/bin/count.sh'] = { text: '#!/bin/sh\nn=0\nwhile read -r line; do n=$((n + 1)); done < "$1"\necho "$1 has $n lines"\n', local: true }; });
    out = await run('count ~/notes.md');
    ok('a .sh script runs by its name with while and read', out.includes('has 3 lines'), out);

    // ~/.profile is sourced as a terminal starts.
    await page.evaluate(() => { __fs['/home/pparks/.profile'] = { text: 'export EDITOR_NAME=dinah\nPATH="$HOME/tools:$PATH"\nexport PATH\necho welcome from profile\n', local: true }; });
    await start('two');
    out = await run('echo [$EDITOR_NAME] $PATH', 'two');
    ok('~/.profile runs when a terminal starts and its exports stay', out.includes('[dinah]') && out.includes('/home/pparks/tools:/home/pparks/bin:/bin'), out);

    // Ctrl+C ends a script that never ends, and the page stays responsive.
    await page.locator('#one textarea').focus();
    await page.keyboard.type("sh -c 'while true; do :; done'");
    await page.keyboard.press('Enter');
    await page.waitForTimeout(1500);
    const responsive = await page.evaluate(() => new Promise(resolve => setTimeout(() => resolve(true), 50)));
    await page.keyboard.press('Control+c');
    await idle();
    out = await status();
    ok('Ctrl+C ends a shell script that never ends', responsive && out === '130', out);

    // A line stopped with Ctrl+C can end after the lines typed next have run;
    // the status it ends with does not take the place of theirs.
    await page.evaluate(() => { window.__hold = new Promise(resolve => { window.__release = resolve; }); window.__held = false; });
    await command('clear');
    await page.locator('#one textarea').focus();
    await page.keyboard.type('applets list');
    await page.keyboard.press('Enter');
    await page.waitForFunction(() => window.__held);
    await page.keyboard.press('Control+c');
    await idle();
    await command('nosuchcommand');
    await page.evaluate(() => { window.__hold = null; window.__release(); });
    out = await status();
    ok('a line stopped with Ctrl+C that ends later leaves $? to the line after it', out === '127', out);
  } catch (e) {
    console.log('FAIL the suite ran to its end: ' + (e && e.stack || e));
    process.exitCode = 1;
  } finally {
    await t.close();
  }
})();
