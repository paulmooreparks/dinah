// References to documents in a chat's messages are links that open the
// document where Dinah opens its kind (docs/plans/document-links.md): a
// relative path, an absolute path, a path in inline code, a path with a
// line, a folder, a picture and a PDF each become a link with the glyph and
// tooltip of where it opens, and pressing it opens that place. A path to
// nothing and a path in a fenced block stay text. A message streamed in
// five characters at a time shows what the whole message shows once Go has
// answered, and linking never changes the message's text. Against a stub of
// the Go side, whose resolver returns its results a moment later, as Go's
// does.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    state: { workspace: 'C:/Test', recent: ['C:/Test'] },
    app: () => {
      window.__calls = [];
      const pixel = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';
      const looks = {
        editor: ['Open in the Editor', 'document'], media: ['Open in the Media Viewer', 'image'], files: ['Show in Files', 'folder'],
        system: ["Open with the System's Program", 'open'], reveal: ["Show in the System's File Manager", 'open']
      };
      const resource = p => ({ workspaceId: 'w1', sourceId: 'local', path: p });
      // What is on the stub's disk, by absolute path, and where each opens.
      const disk = {
        'C:/Test/docs/plan.md': { opens: 'editor', view: 'page', resource: resource('docs/plan.md') },
        'C:/Test/src/main.go': { opens: 'editor', view: 'text', resource: resource('src/main.go') },
        'C:/Test/shots/screen.png': { opens: 'media', resource: resource('shots/screen.png') },
        'C:/Test/docs': { opens: 'files', directory: true, resource: resource('docs'), mountPath: '/workspaces/test/local/docs' },
        'C:/Elsewhere/report.pdf': { opens: 'system' },
        'C:/Elsewhere/notes.txt': { opens: 'editor', view: 'text' }
      };
      // The stub resolves as document_links.go does, for these paths.
      const resolve = (cwd, ref) => {
        let p = ref.replace(/\\/g, '/'), line = 0, m = /(?::(\d+)(?::\d+)?|#L(\d+)(?:-L?\d+)?)$/.exec(p);
        if (m) { line = +(m[1] || m[2]); p = p.slice(0, m.index); }
        if (!/^[A-Za-z]:\//.test(p)) p = cwd.replace(/\/$/, '') + '/' + p.replace(/^\.\//, '');
        p = p.replace(/\/$/, '');
        const f = disk[p];
        if (!f) return { ref, found: false, path: p };
        return Object.assign({ ref, found: true, path: p, line: f.opens === 'editor' ? line : 0, label: looks[f.opens][0], glyph: looks[f.opens][1] }, f);
      };
      window.__resolve = resolve;
      return {
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        OpenSessionAt: async (ws, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: ws, open: true, working: false, queued: [] }),
        ResolveDocumentLinks: (cwd, refs) => { __calls.push(['batch', cwd, refs.slice()]); return new Promise(done => setTimeout(() => done(refs.map(r => resolve(cwd, r))), 5)); },
        ResolveDocumentLink: async (cwd, ref) => { __calls.push(['resolve', cwd, ref]); return resolve(cwd, ref); },
        ReadLinkedImage: async (cwd, ref) => { __calls.push(['image', cwd, ref]); return 'data:image/png;base64,' + pixel; },
        OpenComputerFile: async (ws, file) => { __calls.push(['computer', ws, file]); return { id: 'loose', title: 'notes.txt', origin: { kind: 'new', workspaceId: ws } }; },
        OpenPath: async p => { __calls.push(['system', p]); return 'opened'; }
      };
    },
    ready: () => window.dinahWorkbench && window.dinahEditor && window.dinahChatText && window.dinahMediaViewer && window.dinahResources,
  });
  const { page: p, ok } = t;
  try {
    const id = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:links' }));
    const win = '[data-win="' + id + '"]';
    await p.locator(win + ' [data-view-id="prompt"]').waitFor();
    // The Editor, Files and the viewer are watched rather than opened: what
    // matters here is which one a link reaches, with what. An Editor window
    // is stood in for by an element that records the line it is sent to.
    await p.evaluate(win => {
      window.__win = win;
      window.__say = (text, tool) => { if (tool) __emit('agent:update', { sessionId: 'codex:links', update: { sessionUpdate: 'tool_call', toolCallId: tool, title: 'Ran', status: 'completed' } }); __emit('agent:update', { sessionId: 'codex:links', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } } }); };
      window.__last = () => { const all = document.querySelectorAll(win + ' .msg-text'); return all[all.length - 1]; };
      const editorWindow = key => {
        let el = document.querySelector('[data-win="' + key + '"] .editor-applet');
        if (!el) {
          const w = document.createElement('div'); w.dataset.win = key; w.hidden = true;
          el = document.createElement('div'); el.className = 'editor-applet'; w.appendChild(el); document.body.appendChild(w);
          el.addEventListener('dinah:editor-line', e => __calls.push(['line', key, e.detail.line]));
        }
      };
      dinahEditor.openResource = async (ref, intent) => { __calls.push(['editor', ref, intent || '']); editorWindow('ed-' + ref.path); return 'ed-' + ref.path; };
      dinahEditor.open = async v => { __calls.push(['editor-draft', v.id]); editorWindow('ed-' + v.id); return 'ed-' + v.id; };
      dinahResources.openFiles = async (mount, ref) => { __calls.push(['files', mount, ref]); };
      window.__pending = async () => { while (dinahChatText.linksPending()) await new Promise(r => setTimeout(r, 10)); };
      window.__full = async text => { const box = document.createElement('div'); box.appendChild(dinahChatText.nodes(text, null, 'C:/Test')); await __pending(); return box.innerHTML; };
    }, win);

    const message = 'Read docs/plan.md, then `docs/plan.md` in code, and [the plan](docs/plan.md).\n\n' +
      'The screenshot is shots/screen.png; the report is C:\\Elsewhere\\report.pdf (a PDF), with notes at `C:/Elsewhere/notes.txt`.\n\n' +
      'The parser starts at src/main.go:42, and the folder is docs/.\n\n' +
      'Not written yet: docs/missing.md and `docs/later.md`. Words like e.g. and Node.js stay words.\n\n' +
      '```\ncat docs/plan.md\n```\n\nThat is all.';
    await p.evaluate(text => __say(text, 'first'), message);
    await p.waitForFunction(() => __last() && __last().textContent.includes('That is all.'));
    await p.evaluate(() => __pending());
    await p.waitForFunction(() => __last().querySelectorAll('a.smart-chip[data-opens]').length >= 8);
    const drawn = await p.evaluate(() => {
      const el = __last(), links = [...el.querySelectorAll('a.smart-chip')];
      const by = ref => links.filter(a => (a.dataset.chipPath || a.dataset.chipLocal || a.dataset.chipFile) === ref);
      const look = a => a ? { title: a.title, opens: a.dataset.opens, glyph: (a.querySelector(':scope > .link-glyph') || {}).style?.getPropertyValue('--glyph') || '', text: a.textContent, code: !!a.querySelector('code') } : null;
      return {
        plan: by('docs/plan.md').map(look), screen: look(by('shots/screen.png')[0]), pdf: look(by('C:\\Elsewhere\\report.pdf')[0]),
        notes: look(by('C:/Elsewhere/notes.txt')[0]), line: look(by('src/main.go:42')[0]), folder: look(by('docs/')[0]),
        missing: by('docs/missing.md').length + by('docs/later.md').length, words: links.some(a => /e\.g|Node\.js/.test(a.textContent)),
        fence: el.querySelectorAll('pre a').length, fenceText: el.querySelector('pre').textContent,
        text: el.textContent
      };
    });
    const editorLook = { title: 'Open in the Editor', opens: 'editor', glyph: 'var(--glyph-document)' };
    const has = (got, want) => !!got && Object.keys(want).every(k => got[k] === want[k]);
    ok('a relative path in the text is a link to the Editor', has(drawn.plan.find(l => !l.code && l.text === 'docs/plan.md'), editorLook), JSON.stringify(drawn.plan));
    ok('a path in inline code is a link around the code', has(drawn.plan.find(l => l.code), editorLook), JSON.stringify(drawn.plan));
    ok('a Markdown link to a path says where it opens', has(drawn.plan.find(l => l.text === 'the plan'), editorLook), JSON.stringify(drawn.plan));
    ok('a picture is a link to the media viewer', has(drawn.screen, { title: 'Open in the Media Viewer', opens: 'media', glyph: 'var(--glyph-image)', text: 'shots/screen.png' }), JSON.stringify(drawn.screen));
    ok('an absolute Windows path to a PDF is a link to the system\'s program', has(drawn.pdf, { title: "Open with the System's Program", opens: 'system', glyph: 'var(--glyph-open)', text: 'C:\\Elsewhere\\report.pdf' }), JSON.stringify(drawn.pdf));
    ok('an absolute path in code outside the workspaces is a link to the Editor', has(drawn.notes, editorLook), JSON.stringify(drawn.notes));
    ok('a path with a line is a link, the line part of it', has(drawn.line, Object.assign({ text: 'src/main.go:42' }, editorLook)), JSON.stringify(drawn.line));
    ok('a folder is a link to Files', has(drawn.folder, { title: 'Show in Files', opens: 'files', glyph: 'var(--glyph-folder)' }), JSON.stringify(drawn.folder));
    ok('a path to nothing stays text, in the text and in code', drawn.missing === 0);
    ok('words that look like file names but are not there stay text', !drawn.words);
    ok('a path in a fenced block is not linked', drawn.fence === 0 && drawn.fenceText === 'cat docs/plan.md');
    const plainText = await p.evaluate(text => { const box = document.createElement('div'); box.appendChild(dinahHTML.fragment(dinahChatText.markdown(text))); return box.textContent.replace(/\n/g, ''); }, message);
    ok('linking does not change the message\'s text', drawn.text.replace(/\n/g, '') === plainText, drawn.text);
    const batches = await p.evaluate(() => __calls.filter(c => c[0] === 'batch'));
    ok('Go is asked about the paths in batches, from the chat\'s folder', batches.length >= 1 && batches.length <= 3 && batches.every(b => b[1] === 'C:/Test') && batches.some(b => b[2].length > 5), JSON.stringify(batches.map(b => b[2].length)));
    ok('a path in the fence is never asked about', !batches.some(b => b[2].includes('cat docs/plan.md')));

    // Pressing each link opens the right place.
    const press = async (selector, kind) => {
      const before = await p.evaluate(() => __calls.length);
      await p.locator(win + ' ' + selector).last().click();
      await p.waitForFunction(([n, kind]) => __calls.slice(n).some(c => c[0] === kind), [before, kind]);
      return p.evaluate(n => __calls.slice(n).filter(c => c[0] !== 'resolve'), before);
    };
    let got = await press('.msg-text a[data-chip-path="docs/plan.md"]:not(:has(code))', 'editor');
    ok('the relative path opens the workspace\'s document in the Editor', got[0][0] === 'editor' && JSON.stringify(got[0][1]) === JSON.stringify({ workspaceId: 'w1', sourceId: 'local', path: 'docs/plan.md' }) && got[0][2] === '', JSON.stringify(got));
    got = await press('.msg-text a[data-chip-path="docs/plan.md"]:has(code)', 'editor');
    ok('the path in code opens the same document', got[0][0] === 'editor' && got[0][1].path === 'docs/plan.md');
    got = await press('.msg-text a[data-chip-path="src/main.go:42"]', 'line');
    ok('the path with a line opens the Editor at that line', got.some(c => c[0] === 'editor' && c[1].path === 'src/main.go') && got.some(c => c[0] === 'line' && c[2] === 42), JSON.stringify(got));
    got = await press('.msg-text a[data-chip-path="C:/Elsewhere/notes.txt"]', 'editor-draft');
    ok('a file outside the workspaces opens in the Editor as a document of its own', got.some(c => c[0] === 'computer' && c[2] === 'C:/Elsewhere/notes.txt') && got.some(c => c[0] === 'editor-draft' && c[1] === 'loose'), JSON.stringify(got));
    got = await press('.msg-text a[data-chip-path="docs/"]', 'files');
    ok('the folder opens in Files at its place there', got[0][0] === 'files' && got[0][1] === '/workspaces/test/local/docs' && got[0][2].path === 'docs', JSON.stringify(got));
    got = await press('.msg-text a[data-chip-path="C:\\\\Elsewhere\\\\report.pdf"]', 'system');
    ok('the PDF opens with the system\'s program', got[0][0] === 'system' && got[0][1] === 'C:/Elsewhere/report.pdf', JSON.stringify(got));
    got = await press('.msg-text a[data-chip-path="shots/screen.png"]', 'image');
    await p.locator('#media-viewer[open] .lightbox-img').waitFor();
    ok('the picture opens in the media viewer', got[0][1] === 'C:/Test' && got[0][2] === 'shots/screen.png' && (await p.locator('#media-viewer .lightbox-caption').textContent()) === 'screen.png');
    await p.keyboard.press('Escape');
    await p.locator('#media-viewer').waitFor({ state: 'hidden' });

    // Streamed five characters at a time, the message shows after every
    // chunk what the whole text so far shows, once Go has answered.
    let sofar = '', mismatch = '';
    await p.evaluate(() => __say('', 'second'));
    for (let at = 0; at < message.length; at += 5) {
      const piece = message.slice(at, at + 5);
      sofar += piece;
      await p.evaluate(piece => __say(piece), piece);
      const full = await p.evaluate(text => __full(text), sofar);
      const same = await p.waitForFunction(full => { const el = __last(); return !!el && !dinahChatText.linksPending() && el.innerHTML === full; }, full, { timeout: 5000 }).then(() => true, () => false);
      if (!same && !mismatch) mismatch = JSON.stringify(sofar) + ' drew ' + await p.evaluate(() => __last().innerHTML) + ' but whole is ' + full;
    }
    ok('a streamed message with paths shows what the whole text shows after every chunk', !mismatch, mismatch);
    ok('the streamed message ends with the same links', await p.evaluate(() => __last().querySelectorAll('a.smart-chip[data-opens]').length) === drawn.plan.length + 5);

    // A file that appears later is linked when the message is drawn again,
    // once the answer about it has gone stale; nothing asks in between.
    const asked = await p.evaluate(() => __calls.filter(c => c[0] === 'batch').length);
    await p.waitForTimeout(400);
    ok('nothing polls for a path that is not there', await p.evaluate(() => __calls.filter(c => c[0] === 'batch').length) === asked);
  } finally { await t.close(); }
})();
