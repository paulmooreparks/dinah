// The Terminal: the parkscomputing.com shell over an in-memory stand-in for
// the app's virtual file system, with a writable home and local folder
// under a catalog of workspaces that cannot be written.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1100, height: 760 },
    timeout: 30000,
    app: () => {
      // The file system: a map of paths. Local entries are writable; the
      // catalogs of workspaces are not.
      const fsMap = JSON.parse(localStorage.getItem('fixture-fs') || 'null') || {
        '/home': { dir: true, kind: 'catalog' },
        '/home/pparks': { dir: true, local: true, source: 'dinah-home', workspace: '' },
        '/workspaces': { dir: true, kind: 'catalog' },
        '/workspaces/delivery': { dir: true, kind: 'workspace', workspace: 'delivery' },
        '/workspaces/delivery/files': { dir: true, local: true, source: 'files-source', workspace: 'delivery' },
        '/workspaces/delivery/files/notes.md': { text: '# Notes\nfirst line\nsecond line\n', local: true },
        '/workspaces/delivery/files/evil.md': { text: 'hello\x1b]52;c;secret\x07\x1b[2J\u009b31m‮evil\nworld\n', local: true }
      };
      const persist = () => localStorage.setItem('fixture-fs', JSON.stringify(fsMap));
      const norm = p => p === '~' ? '/home/pparks' : p.startsWith('~/') ? '/home/pparks' + p.slice(1) : p;
      const parent = p => p.slice(0, p.lastIndexOf('/')) || '/';
      const owner = p => { const parts = p.split('/'); for (let i = parts.length; i > 1; i--) { const e = fsMap[parts.slice(0, i).join('/')]; if (e && e.source) return { workspaceId: e.workspace, sourceId: e.source, path: parts.slice(i).join('/') }; } const w = fsMap['/workspaces/' + parts[2]]; return w && w.workspace ? { workspaceId: w.workspace, sourceId: '', path: '' } : undefined; };
      const writable = p => { const parts = p.split('/'); for (let i = parts.length; i > 1; i--) { const e = fsMap[parts.slice(0, i).join('/')]; if (e && e.source) return !!e.local; } return false; };
      const revision = text => 'r' + text.length + ':' + [...text].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7);
      const item = p => { const e = fsMap[p]; if (!e) throw new Error('the source path is unavailable'); return { path: p, title: e.title || p.split('/').pop(), kind: e.kind || (e.dir ? 'directory' : 'file'), directory: !!e.dir, revision: e.text != null ? revision(e.text) : '', url: '', actions: e.text != null ? (e.dir ? ['read', 'preview', 'list', 'edit'] : ['read', 'preview']) : ['list'], ref: owner(p), size: e.dir ? 0 : (e.text || '').length, modified: '2026-10-11T08:30:00Z', writable: writable(p) }; };
      const children = p => Object.keys(fsMap).filter(k => k !== p && parent(k) === p).sort();
      const checkWritable = p => { if (!writable(p)) throw new Error(p + ' is a catalog of workspaces and sources, which cannot be changed here'); };
      window.__fs = fsMap; window.__revision = revision;
      const A = {
        HostLocale: async () => 'en-GB-u-hc-h23',
        StatWorkspaceMount: async p => { p = norm(p); __calls.push(['stat', p]); if (p.endsWith('/slow')) await new Promise(resolve => { window.__resolveSlow = resolve; }); return item(p); },
        ListWorkspaceMounts: async (p, start, limit) => { p = norm(p); __calls.push(['list', p]); if (!fsMap[p] || !fsMap[p].dir) throw new Error('not a folder'); const all = children(p); return { entries: all.slice(start, start + limit).map(item), nextStart: start + limit < all.length ? start + limit : -1 }; },
        ReadWorkspaceMount: async p => { p = norm(p); __calls.push(['read', p]); const e = fsMap[p]; if (!e || e.text == null) throw new Error('not readable'); return { resource: item(p), text: e.text, format: 'markdown', revision: revision(e.text) }; },
        WriteWorkspaceMount: async (p, text, mode, rev) => { checkWritable(p); const e = fsMap[p]; if (!fsMap[parent(p)] || !fsMap[parent(p)].dir) throw new Error('the folder for the file does not exist'); if (e && e.dir) throw new Error(p + ': is a folder'); if (mode === 'create' && e) throw new Error(p + ': the file changed since it was read'); if (mode === 'expect' && (!e || revision(e.text) !== rev)) throw new Error(p + ': the file changed since it was read'); fsMap[p] = { text, local: true }; persist(); __calls.push(['write', p, mode]); return item(p); },
        MakeWorkspaceDirectory: async (p, parents) => { checkWritable(p); if (fsMap[p]) { if (parents && fsMap[p].dir) return item(p); throw new Error(p + ': already exists'); } if (!fsMap[parent(p)]) { if (!parents) throw new Error('the folder above it does not exist'); await window.go.main.App.MakeWorkspaceDirectory(parent(p), true); } fsMap[p] = { dir: true, local: true }; persist(); return item(p); },
        RemoveWorkspaceMount: async (p, recursive) => { checkWritable(p); if (!fsMap[p]) throw new Error(p + ': no such file or folder'); const under = Object.keys(fsMap).filter(k => k.startsWith(p + '/')); if (under.length && !recursive) throw new Error('the folder is not empty'); [p, ...under].forEach(k => delete fsMap[k]); persist(); return under.length + 1; },
        CopyWorkspaceMount: async (from, _fe, to, _te, recursive) => { checkWritable(to); if (!fsMap[from]) throw new Error(from + ': no such file or folder'); if (fsMap[from].dir && !recursive) throw new Error('copying a folder needs -r'); if (fsMap[to] && (fsMap[to].dir || fsMap[from].dir)) throw new Error(to + ': already exists'); const keys = Object.keys(fsMap).filter(k => k === from || k.startsWith(from + '/')); keys.forEach(k => { fsMap[to + k.slice(from.length)] = JSON.parse(JSON.stringify(fsMap[k])); fsMap[to + k.slice(from.length)].local = true; delete fsMap[to + k.slice(from.length)].source; }); persist(); return keys.length; },
        MoveWorkspaceMount: async (from, _fe, to) => { checkWritable(from); checkWritable(to); if (!fsMap[from]) throw new Error(from + ': no such file or folder'); if (fsMap[to] && fsMap[to].dir) throw new Error(to + ': already exists'); Object.keys(fsMap).filter(k => k === from || k.startsWith(from + '/')).forEach(k => { fsMap[to + k.slice(from.length)] = fsMap[k]; delete fsMap[k]; }); persist(); return item(to); }
      };
      A.StatWorkspaceMountExpected = async (p, expected) => { if (window.__replacement && expected.sourceId === 'files-source') throw new Error('The source is no longer attached.'); return A.StatWorkspaceMount(p); };
      A.ListWorkspaceMountsExpected = async (p, s, l, expected) => { if (window.__replacement && expected.sourceId === 'files-source') throw new Error('The source is no longer attached.'); return A.ListWorkspaceMounts(p, s, l); };
      A.ReadWorkspaceMountExpected = async (p, expected) => { if (window.__replacement && expected.sourceId === 'files-source') throw new Error('The source is no longer attached.'); return A.ReadWorkspaceMount(p); };
      return A;
    },
    init: () => {
      let values = JSON.parse(localStorage.getItem('fixture-ui') || '{}');
      window.dinahUIState = { ready: Promise.resolve(), get: (key, fallback) => values[key] || fallback, set: (key, value) => { values[key] = value; localStorage.setItem('fixture-ui', JSON.stringify(values)); }, flush: async () => {} };
      window.__calls = [];
      window.__listeners = new Set();
      window.dinahResources = {
        open: async (item, isCurrent) => { if (window.__holdOpen) await new Promise(resolve => { window.__finishOpen = resolve; }); if (!isCurrent || isCurrent()) __calls.push(['open', item.path]); },
        commands: () => [{ name: 'fixture-applet', usage: 'fixture-applet [arguments]', description: 'Open the fixture applet' }],
        invoke: async (name, args, current) => { if (current()) __calls.push(['invoke', name, args]); return 'Applet opened'; },
        subscribe: listener => { __listeners.add(listener); return () => __listeners.delete(listener); }
      };
      window.dinahEditor = { openResource: async ref => { __calls.push(['editor', ref.path]); } };
      window.pudlWindows = { close: key => { __calls.push(['close', key]); window['__' + key].destroy(); document.getElementById(key).remove(); } };
    },
    goto: false,
  });
  const { page, ok, errors } = t;
  try {
    // The page is a fixture of the terminal scripts alone; the other files
    // come from the shared route.
    await t.context.route('http://dinah.test/fixture.html', route => route.fulfill({ contentType: 'text/html', body: '<html style="--input-bg:#fafafa;--text:#16304c;--accent:#007e87;--danger:#b00020;--positive:#008048;--warn:#996600;--text-muted:#667788;--border:#ccd"><head><link rel="stylesheet" href="terminal.css"></head><body style="display:flex;height:700px;gap:12px"><div id="one" style="width:640px"></div><div id="two" style="width:420px"></div><script src="helpers.js"></script><script src="regional.js"></script><script src="terminal-applet.js"></script></body></html>' }));
    async function setup(first) {
      await page.goto('http://dinah.test/fixture.html');
      if (first) await page.evaluate(() => localStorage.clear());
      if (first) await page.reload();
      await page.evaluate(() => {
        document.getElementById('one').setAttribute('data-win', 'one'); document.getElementById('two').setAttribute('data-win', 'two');
        window.__one = dinahTerminal.init(document.querySelector('#one'), {}, { path: '/home/pparks', instanceId: 'one' });
        window.__two = dinahTerminal.init(document.querySelector('#two'), {}, { path: '/workspaces/delivery/files', instanceId: 'two' });
      });
      await page.locator('#one[data-terminal-ready="true"]').waitFor();
      await page.locator('#two[data-terminal-ready="true"]').waitFor();
    }
    const idle = id => page.waitForFunction(id => !document.querySelector('#' + id) || document.querySelector('#' + id).dataset.terminalBusy === 'false', id);
    async function command(text, id = 'one') { await page.locator('#' + id + ' textarea').focus(); await page.keyboard.type(text); await page.keyboard.press('Enter'); await idle(id); }
    const output = (id = 'one') => page.evaluate(id => dinahUIState.get('dinah-terminal:' + id, {}).output || '', id);
    const fresh = async () => { await command('clear'); };
    const screenText = (id = 'one') => page.locator('#' + id + ' .xterm-rows').textContent();
    const check = async (name, passed) => { ok(name, passed); if (!passed) console.log('  output: ' + JSON.stringify((await output()).slice(-700)) + '\n  screen: ' + JSON.stringify((await screenText().catch(() => '')).slice(-400))); };

    await setup(true);
    await page.waitForFunction(() => document.querySelector('#one .xterm-rows').textContent.includes('$'));
    await check('the prompt is the person\'s, at their home folder', (await screenText()).replace(/\s/g, ' ').includes('pparks@dinah:~$'));
    await check('starting lists nothing', await page.evaluate(() => !__calls.some(call => call[0] === 'list')));

    await command('cd /workspaces/delivery/files');
    await command('echo hello > a.txt'); await command('echo more >> a.txt'); await fresh();
    await command('cat a.txt'); await command('cat a.txt | wc -l');
    await check('> creates a file, >> adds to it, and a pipe counts its lines', (await output()).includes('hello\nmore\n') && /\b2\n/.test(await output()));
    await fresh(); await command('ls *.txt *.md');
    await check('wildcards expand to the matching files', (await output()).includes('a.txt') && (await output()).includes('notes.md') && (await output()).includes('evil.md'));
    await fresh(); await command('ls -l');
    await check('ls -l shows the date in the regional format, the kind and the size', /11\/10\/2026, \d\d:30\s+file\s+a\.txt\s+11 B/.test((await output()).replace(/\x1b\[[0-9;]*m/g, '')));
    await command('mkdir -p d/e && cp a.txt d/e/ && ls d/e');
    await check('mkdir -p, cp into a folder, and && run in order', (await output()).trim().endsWith('a.txt') && await page.evaluate(() => !!__fs['/workspaces/delivery/files/d/e/a.txt']));
    await command('mv a.txt b.txt'); await fresh(); await command('ls');
    await check('mv renames a file', (await output()).includes('b.txt') && !(await output()).includes('a.txt'));
    await command('rm d'); await command('rm -r d');
    await check('rm needs -r for a folder, and rm -r says what it removed', (await output()).includes('is a folder (use -r)') && (await output()).includes('removed d (3 entries)') && await page.evaluate(() => !__fs['/workspaces/delivery/files/d']));
    await fresh(); await command('export NAME=World; echo "Hello $NAME" \'$NAME\''); await command('false || echo fallback'); await command('false && echo never; echo status $?');
    await check('variables, ||, && and $? behave as in a shell', (await output()).includes('Hello World $NAME') && (await output()).includes('fallback') && !(await output()).includes('\nnever\n') && (await output()).includes('status 1'));
    await command('mkdir -p ~/bin'); await command('echo \'echo Hi $1 from $0 with $# argument\' > ~/bin/greet.sh'); await fresh(); await command('greet Paul');
    await check('a script in ~/bin runs by its name with its arguments', (await output()).includes('Hi Paul from greet.sh with 1 argument'));
    await fresh(); await command('cat notes.md | grep -n line | sort -r');
    await check('a three-stage pipeline numbers and sorts matches', (await output()).includes('3:second line\n2:first line'));
    await fresh(); await command('cat evil.md');
    await check('a document\'s control sequences never reach the screen', await page.evaluate(() => { const value = dinahUIState.get('dinah-terminal:one').output; return value.includes('hello␛]52;c;secret') && !/[\x07\u009b‮]/.test(value) && !value.includes('\x1b]52'); }));

    // nano: write a new file, save, leave.
    await page.locator('#one textarea').focus(); await page.keyboard.type('nano draft.md'); await page.keyboard.press('Enter');
    await page.waitForFunction(() => document.querySelector('#one .xterm-rows').textContent.includes('New file'));
    await page.keyboard.type('Line one'); await page.keyboard.press('Enter'); await page.keyboard.type('Line two');
    await page.keyboard.press('Control+s'); await page.waitForFunction(() => document.querySelector('#one .xterm-rows').textContent.includes('Saved'));
    await t.shot('terminal-nano');
    await page.keyboard.press('Control+x'); await idle('one');
    await check('nano creates and saves a file', await page.evaluate(() => __fs['/workspaces/delivery/files/draft.md'].text === 'Line one\nLine two\n'));
    // nano refuses to overwrite a change made meanwhile.
    await page.locator('#one textarea').focus(); await page.keyboard.type('nano draft.md'); await page.keyboard.press('Enter');
    await page.waitForFunction(() => document.querySelector('#one .xterm-rows').textContent.includes('^W Find'));
    await page.keyboard.type('X');
    await page.evaluate(() => { __fs['/workspaces/delivery/files/draft.md'].text = 'changed elsewhere\n'; });
    await page.keyboard.press('Control+s'); await page.waitForFunction(() => document.querySelector('#one .xterm-rows').textContent.includes('Not saved'));
    await page.keyboard.press('Control+x'); await page.keyboard.press('n'); await idle('one');
    await check('nano does not overwrite a change made elsewhere', await page.evaluate(() => __fs['/workspaces/delivery/files/draft.md'].text === 'changed elsewhere\n'));

    // A key the page takes first, as the menu bar takes Alt with a title's
    // letter in the capture phase, does not reach the shell as well.
    await page.evaluate(() => { window.__taken = e => { if (e.key === 'z') e.preventDefault(); }; document.addEventListener('keydown', window.__taken, true); });
    await page.locator('#one textarea').focus(); await page.keyboard.type('echo zaza');
    await page.waitForTimeout(150);
    await check('a key the page has taken is not typed into the Terminal', await page.evaluate(() => { const t = document.querySelector('#one .xterm-rows').textContent; return t.includes('echo aa') && !t.includes('echo zaza'); }));
    await page.evaluate(() => document.removeEventListener('keydown', window.__taken, true));
    await page.keyboard.press('Control+u');

    // A catalog cannot be written; edit -g opens a file in the Editor.
    await fresh(); await command('echo x > ../x'); await command('edit -g notes.md');
    const refused = await output();
    const shown = refused.replace(/[[0-9;]*m/g, '');
    await check('a catalog of workspaces cannot be written, and edit -g opens the Editor', shown.includes('cannot be changed here') && await page.evaluate(() => __calls.some(call => call[0] === 'editor' && call[1] === 'notes.md')));
    await page.locator('#one textarea').focus(); await page.keyboard.type('less notes.md'); await page.keyboard.press('Enter');
    await page.waitForFunction(() => document.querySelector('#one .xterm-rows').textContent.includes('(space: next'));
    await page.keyboard.press('q'); await idle('one'); await fresh(); await command('pwd');
    await check('less shows a file and q returns to the prompt', (await output()).endsWith('/workspaces/delivery/files\n'));

    await page.locator('#one textarea').focus(); await page.keyboard.type('cat no'); await page.keyboard.press('Tab');
    await page.waitForFunction(() => document.querySelector('#one .xterm-rows').textContent.includes('cat notes.md '));
    await page.keyboard.press('Control+u'); await page.keyboard.type('gree'); await page.keyboard.press('Tab');
    await page.waitForFunction(() => document.querySelector('#one .xterm-rows').textContent.includes('$ greet '));
    await page.keyboard.press('Control+u');
    await check('Tab completes paths, and names from the scripts in ~/bin', true);

    await command('fixture-applet "two words"');
    await check('tools run with their parsed arguments', await page.evaluate(() => __calls.some(call => call[0] === 'invoke' && call[1] === 'fixture-applet' && call[2][0] === 'two words')));
    await command('open notes.md');
    await check('open uses the shared resource opener', await page.evaluate(() => __calls.at(-1)[0] === 'open' && __calls.at(-1)[1] === '/workspaces/delivery/files/notes.md'));

    await page.locator('#one textarea').focus(); await page.keyboard.type('cd slow'); await page.keyboard.press('Enter');
    await page.waitForFunction(() => typeof __resolveSlow === 'function'); await page.keyboard.press('Control+c');
    await page.evaluate(() => { __fs['/workspaces/delivery/files/slow'] = { dir: true, local: true }; __resolveSlow(); });
    await command('pwd');
    await check('Ctrl+C stops a command and its late result changes nothing', (await output()).trim().endsWith('/workspaces/delivery/files'));
    await page.locator('#one textarea').focus(); await page.keyboard.insertText('pwd\nclear');
    await check('pasted newlines do not run commands', !(await output()).endsWith('pwd clear\n'));
    await page.keyboard.press('Control+c');
    await command('echo two', 'two');
    await check('each terminal keeps its own folder', await page.locator('#two').getAttribute('data-terminal-cwd') === '/workspaces/delivery/files' && (await output('two')).includes('two'));

    await setup(false);
    await check('folder, history and variables survive a new terminal', await page.locator('#one').getAttribute('data-terminal-cwd') === '/workspaces/delivery/files' && await page.evaluate(() => dinahUIState.get('dinah-terminal:one').history.includes('cd slow') && dinahUIState.get('dinah-terminal:one').env.NAME === 'World'));
    await page.evaluate(() => { window.__replacement = true; });
    await fresh(); await command('ls');
    await check('a detached source cannot be reached through a reused name', (await output()).includes('no longer attached'));
    await page.evaluate(() => { window.__replacement = false; });
    await command('cd /workspaces/delivery/files'); await command('help');
    await check('help lists the shell commands and the tools', ['nano', 'grep', 'mkdir', 'cp', 'mv', 'rm', 'head', 'tail', 'sort', 'uniq', 'wc', 'find', 'tree', 'less', 'export', 'fixture-applet'].every(name => (output !== null)) && await output().then(text => ['nano', 'grep', 'mkdir', 'tee', 'export', 'fixture-applet'].every(name => text.includes(name))));
    await t.shot('terminal');
    await command('exit');
    await page.waitForFunction(() => !document.getElementById('one'));
    await check('exit closes only its own window', await page.locator('#two').count() === 1 && await page.evaluate(() => __calls.at(-1)[0] === 'close' && __calls.at(-1)[1] === 'one'));
    await check('no browser errors or dialogs', errors.length === 0);
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
