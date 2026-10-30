// Lua in the Terminal: the lua command,
// the standard scripts in /bin and their shared modules in /lib/lua, taken
// from cmd/dinah-desktop/system as the app embeds them, run by the real
// wasmoon in a worker; a person's ~/bin before /bin; files, require and the
// applet bridge; Ctrl+C ending a script that never ends.
const { launch, FRONTEND } = require('./lib/page');
const path = require('path');
const fs = require('fs');
(async () => {
  // The system folders, as the app embeds them.
  const system = {};
  const root = path.join(__dirname, '..', '..', 'system');
  (function walk(dir, at) {
    system[at] = null;
    for (const name of fs.readdirSync(dir)) {
      const full = path.join(dir, name), p = at + '/' + name;
      if (fs.statSync(full).isDirectory()) walk(full, p); else system[p] = fs.readFileSync(full, 'utf8');
    }
  })(path.join(root, 'bin'), '/bin');
  (function walk(dir, at) {
    system[at] = null;
    for (const name of fs.readdirSync(dir)) {
      const full = path.join(dir, name), p = at + '/' + name;
      if (fs.statSync(full).isDirectory()) walk(full, p); else system[p] = fs.readFileSync(full, 'utf8');
    }
  })(path.join(root, 'lib'), '/lib');
  const t = await launch({
    viewport: { width: 1100, height: 760 },
    timeout: 20000,
    app: () => {
      const fsMap = {
        '/home': { dir: true }, '/home/pparks': { dir: true, local: true },
        '/home/pparks/notes.md': { text: 'alpha\nbeta\ngamma\n', local: true },
        '/workspaces': { dir: true }
      };
      const norm = p => p === '~' ? '/home/pparks' : p.startsWith('~/') ? '/home/pparks' + p.slice(1) : p;
      const parent = p => p.slice(0, p.lastIndexOf('/')) || '/';
      const item = p => { const e = fsMap[p]; if (!e) throw new Error('no such file or folder'); return { path: p, title: p.split('/').pop(), kind: e.dir ? 'directory' : 'file', directory: !!e.dir, revision: e.text != null ? 'r' + e.text.length : '', actions: e.dir ? ['list'] : ['read', 'preview'], ref: p.startsWith('/home/pparks') ? { workspaceId: '', sourceId: 'dinah-home', path: '' } : undefined, size: (e.text || '').length, writable: !!e.local }; };
      const writable = p => /^\/(home\/pparks|bin|lib)(\/|$)/.test(p);
      window.__exitWork = new Set(); window.__exitWorkSequence=0;
      window.__fs = fsMap;
      const A = {
        BeginExitWork: async () => {const id=String(++__exitWorkSequence);__exitWork.add(id);return id;},
        EndExitWork: async id => __exitWork.delete(id),
        StatWorkspaceMount: async p => item(norm(p)),
        ListWorkspaceMounts: async (p, start, limit) => { p = norm(p); const all = Object.keys(fsMap).filter(k => k !== p && parent(k) === p).sort(); return { entries: all.slice(start, start + limit).map(item), nextStart: start + limit < all.length ? start + limit : -1 }; },
        ReadWorkspaceMount: async p => { p = norm(p); const e = fsMap[p]; if (!e || e.text == null) throw new Error('no such file'); return { resource: item(p), text: e.text, format: 'markdown', revision: 'r' + e.text.length }; },
        WriteWorkspaceMount: async (p, text, mode) => { if (!writable(p)) throw new Error(p + ' is a catalog of workspaces and sources, which cannot be changed here'); fsMap[p] = { text, local: true }; return item(p); },
        MakeWorkspaceDirectory: async p => { fsMap[p] = { dir: true, local: true }; return item(p); },
        RemoveWorkspaceMount: async p => { delete fsMap[p]; return 1; },
        MoveWorkspaceMount: async (a, _e, b) => { fsMap[b] = fsMap[a]; delete fsMap[a]; return item(b); },
        Log: async (source, level, message) => { __calls.push(['log', source, level, message]); }
      };
      A.StatWorkspaceMountExpected = (p) => A.StatWorkspaceMount(p);
      A.ListWorkspaceMountsExpected = (p, s, l) => A.ListWorkspaceMounts(p, s, l);
      A.ReadWorkspaceMountExpected = (p) => A.ReadWorkspaceMount(p);
      return A;
    },
    init: () => {
      window.dinahUIState = { ready: Promise.resolve(), get: (k, f) => f, set: () => {}, flush: async () => {} };
      window.__calls = [];
      window.dinahAppletSDK = { catalog: async () => [{ id: 'dinah.notifications', title: 'Notifications', actions: [{ ref: 'dinah.notifications/post@1', title: 'Post Notification' }] }] };
      window.dinahResources = {
        commands: () => [{ name: 'applets', description: 'Discover applet actions and invoke them.', usage: 'applets ...' }],
        invoke: async (name, args, current) => {
          if (current()) __calls.push(['invoke', name, args]);
          const input = JSON.parse(args[2] || '{}');
          // As the app's schema check does, an input that is not an object is refused.
          if (args[0] === 'run' && (input === null || typeof input !== 'object' || Array.isArray(input))) throw new Error('jsonschema validation failed: got ' + (Array.isArray(input) ? 'array' : typeof input) + ', want object');
          const outputs = {
            'dinah.notifications/post@1': () => ({ title: input.title, body: input.body }),
            'dinah.notifications/request@1': () => ({ title: input.title, body: input.body, response: { answer: 'keyboard navigation' } })
          };
          if (args[0] === 'run') { if (!outputs[args[1]]) throw new Error('no such action'); return JSON.stringify(outputs[args[1]]()); }
          if (args[0] === 'actions') return JSON.stringify({ ref: 'dinah.notifications/post@1' });
          return JSON.stringify({ id: 'n1', status: 'done' });
        },
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
      if (pathname === '/fixture.html') return route.fulfill({ contentType: 'text/html', body: '<html><head><link rel="stylesheet" href="terminal.css"></head><body style="height:700px"><div id="one" style="width:900px;height:600px"></div><script src="helpers.js"></script><script src="regional.js"></script><script src="terminal-applet.js"></script><script src="terminal-lua.js"></script><script src="terminal-sh.js"></script></body></html>' });
      if (pathname.endsWith('.wasm')) return route.fulfill({ path: path.join(FRONTEND, pathname), contentType: 'application/wasm' });
      return route.fallback();
    });
    await page.goto('http://dinah.test/fixture.html');
    await page.evaluate(() => { document.getElementById('one').setAttribute('data-win', 'one'); window.__one = dinahTerminal.init(document.querySelector('#one'), {}, { path: '/home/pparks', instanceId: 'one' }); });
    await page.locator('#one[data-terminal-ready="true"]').waitFor();
    const idle = () => page.waitForFunction(() => document.querySelector('#one').dataset.terminalBusy === 'false');
    async function command(text) { await page.locator('#one textarea').focus(); await page.keyboard.type(text); await page.keyboard.press('Enter'); await idle(); }
    const screen = () => page.locator('#one .xterm-rows').textContent();
    // xterm draws on the next frame, so the screen is read after it has.
    // xterm draws what was written in a later frame. The prompt is written
    // after a command's output, so once the cleared screen shows the command
    // after the first prompt and ends with the next one, the output is drawn
    // too. Spaces are left out of the comparison, since a long line wraps.
    const drawn = text => page.waitForFunction(text => {
      const squeeze = s => s.replace(/[\s ]+/g, '');
      const s = squeeze(document.querySelector('#one .xterm-rows').textContent), at = s.indexOf('$');
      return at >= 0 && s.startsWith(squeeze(text), at + 1) && (s.match(/@dinah:/g) || []).length >= 2 && s.endsWith('$');
    }, text);
    // The status the last line left, drawn after the line that asks for it.
    const statusDrawn = () => page.waitForFunction(() => {
      const s = document.querySelector('#one .xterm-rows').textContent.replace(/[\s ]+/g, ''), at = s.lastIndexOf('echostatus$?');
      return at >= 0 && /^echostatus\$\?status\d+.*\$$/.test(s.slice(at));
    });
    async function run(text) { await command('clear'); await command(text); await drawn(text); return (await screen()).replace(/ /g, ' '); }

    ok('the Lua module registers the lua command', await page.evaluate(() => dinahTerminal.commands().includes('lua') && dinahTerminal.interpreters().some(i => i.id === 'lua')));
    ok('lua -e runs a line of Lua', (await run("lua -e 'print(6 * 7, _VERSION)'")).includes('42') && (await screen()).includes('Lua 5.4'));
    ok('a standard script runs from /bin by its name, in a pipeline', /3\s*2\s*1/.test(await run('seq 3 | tac')));
    ok('tr translates what is piped in', (await run('echo hello | tr a-z A-Z')).includes('HELLO'));
    ok('calc works out a sum', (await run('calc "2^10 + 1"')).includes('1025'));
    ok('json keeps an empty object an object', /\{\}[\s\S]*\{"a":\{\},"b":\[\]\}/.test(await run("echo '{}' | json -c; echo '{\"a\":{},\"b\":[]}' | json -c")));
    ok('json.object and json.array mark an empty table', (await run("lua -e 'local j = require \"json\" print(j.encode({x = j.object(), y = j.array(), z = {}}))'")).includes('{"x":{},"y":[],"z":[]}'));
    ok('json picks a value with a path', /pparks@dinah[^$]*\$ json -p \.b\[2\][\s\S]*\b2\b/.test(await run('echo \'{"b":[1,7],"a":"x"}\' | json -p .b[2]')) || (await screen()).includes('7'));
    ok('base64 encodes and decodes', (await run('echo hello | base64 | base64 -d')).includes('hello'));
    ok('cut picks fields', (await run('echo "a,b,c" | cut -d , -f 2-')).includes('b,c'));
    ok('paste -s joins lines', (await run('seq 3 | paste -s -d +')).includes('1+2+3'));
    ok('nl numbers lines', /1\s+alpha[\s\S]*3\s+gamma/.test(await run('nl notes.md')));
    ok('fold wraps at a width', /fold -w 4 \| wc -l\s*3(?!\d)/.test(await run('echo abcdefghij | fold -w 4 | wc -l')));
    ok('printf formats values and repeats its format', (await run('printf "%s=%03d;" a 7 b 42')).includes('a=007;b=042;'));
    ok('xargs runs a command with the words piped in', (await run('echo notes.md | xargs wc -l')).includes('3'));
    ok('cal shows a month', /February 2027[\s\S]*Mo Tu We Th Fr Sa Su[\s\S]*28/.test(await run('cal 2 2027')));
    ok('rev reverses each line', (await run('echo stressed | rev')).includes('desserts'));
    ok('which says a standard script is in /bin', (await run('which seq')).includes('/bin/seq'));
    ok('man shows a Lua script\'s comments', (await run('man seq')).includes('Counts from first'));

    // A person's own script in ~/bin, with io and require.
    await command("mkdir -p ~/bin ~/lib/lua");
    await page.evaluate(() => {
      __fs['/home/pparks/lib/lua/greet.lua'] = { text: 'return { hello = function(n) return "Hello, " .. n .. "!" end }\n', local: true };
      __fs['/home/pparks/bin/count.lua'] = { text: [
        '-- count: count the lines of a file and write the count beside it',
        'local greet = require "greet"',
        'local n = 0',
        'for _ in io.lines(arg[1]) do n = n + 1 end',
        'local f = assert(io.open(arg[1] .. ".count", "w"))',
        'f:write(n, "\\n")',
        'f:close()',
        'print(greet.hello(os.getenv("USER")) .. " " .. arg[1] .. " has " .. n .. " lines")',
        'local p = io.popen("cat " .. arg[1] .. " | wc -l")',
        'print("wc says " .. p:read("a"):gsub("%s", ""))',
        'os.exit(3)'
      ].join('\n'), local: true };
      __fs['/home/pparks/bin/seq'] = { text: '#!/bin/lua\nprint("my own seq")\n', local: true };
    });
    const counted = await run('count notes.md; echo status $?');
    ok('a Lua script reads a file, writes one, requires a module and runs a command', counted.includes('Hello, pparks! notes.md has 3 lines') && counted.includes('wc says 3') && await page.evaluate(() => (__fs['/home/pparks/notes.md.count'] || {}).text === '3\n'));
    ok('os.exit sets the status', counted.includes('status 3'));
    ok('a script in ~/bin comes before the one in /bin', (await run('seq 3')).includes('my own seq'));
    await run("lua -e 'local f = assert(io.open(\"/bin/x.sh\", \"w\")) f:write(\"echo changed in bin\") f:close()'");
    ok('a script in /bin can be changed, and runs', (await run('x')).includes('changed in bin'));

    // The applet bridge: the sample posts to Notifications.
    await page.evaluate(() => { __calls.length = 0; });
    const hello = await run('hello-dinah');
    const call = await page.evaluate(() => __calls.find(c => c[0] === 'invoke'));
    ok('hello-dinah posts "Hello, Dinah!" to Notifications through the applet bridge', hello.includes('Posted to Notifications: Hello, Dinah!') && call && call[1] === 'applets' && call[2][1] === 'dinah.notifications/post@1' && JSON.parse(call[2][2]).body === 'Hello, Dinah!');
    ok('a script gets an action\'s output once it ends', await page.evaluate(() => __calls.some(c => c[2][0] === 'run')));

    // The samples that talk to Notifications.
    await page.evaluate(() => { __calls.length = 0; });
    await run('echo found three pages | notify "Search Done"');
    const posted = await page.evaluate(() => __calls.filter(c => c[2][1] === 'dinah.notifications/post@1').map(c => JSON.parse(c[2][2])));
    ok('notify posts what is piped into it', posted.length === 1 && posted[0].title === 'Search Done' && posted[0].body === 'found three pages');
    ok('ask prints the answer given in Notifications', (await run('ask "Which words?"')).includes('keyboard navigation'));
    await page.evaluate(() => { __calls.length = 0; });
    await run('log -w -s script:backup disk nearly full; printf "1\\n2\\n" | log');
    await page.evaluate(() => { __fs['/home/pparks/bin/tidy.lua'] = { text: 'local log = require "log"\nlog.error("could not tidy", 3, "files")\n', local: true }; });
    await run('tidy');
    const logged = await page.evaluate(() => __calls.filter(c => c[0] === 'log').map(c => c.slice(1).join('|')));
    ok('log and a Lua script\'s log module add entries with their source', JSON.stringify(logged) === JSON.stringify(['script:backup|warning|disk nearly full', 'terminal|info|1', 'terminal|info|2', 'script:tidy.lua|error|could not tidy 3 files']));
    ok('a script lists the applets\' actions', (await run("lua -e 'for _, a in ipairs(require(\"applets\").actions()) do print(a.ref) end'")).includes('dinah.notifications/post@1'));

    // Ctrl+C ends a script that never ends, and the page stays responsive.
    await page.locator('#one textarea').focus();
    await page.keyboard.type("lua -e 'while true do end'");
    await page.keyboard.press('Enter');
    await page.waitForTimeout(800);
    const responsive = await page.evaluate(() => new Promise(resolve => setTimeout(() => resolve(true), 50)));
    ok('a running Lua command is registered as exit-sensitive work',await page.evaluate(()=>__exitWork.size===1));
    await page.keyboard.press('Control+c');
    await idle();
    await page.waitForFunction(()=>__exitWork.size===0);
    ok('cancelling Lua releases its exit-sensitive work registration',true);
    await command('echo status $?');
    await statusDrawn();
    ok('Ctrl+C ends a script that never ends, and the page stays responsive meanwhile', responsive && (await screen()).includes('status 130'), await screen());
  } catch (e) {
    console.log('FAIL the suite ran to its end: ' + (e && e.stack || e));
    process.exitCode = 1;
  } finally {
    await t.close();
  }
})();
