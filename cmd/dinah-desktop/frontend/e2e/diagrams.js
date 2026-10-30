// Diagrams and file links in messages: a d2 block drawn through the Go side,
// a Mermaid block drawn by the bundled library, nothing drawn before the
// closing fence, a bad diagram shown with its error, and file links opened.
const { launch, PAGE, OUT } = require('./lib/page');
const path = require('path');
(async () => {
  // A drawing that brings its fonts as d2 does, in an @font-face with a data
  // URL: one real font, one fetched from elsewhere, and one under the page's
  // own font's name.
  const font = require('fs').readFileSync(path.join(__dirname, '../pudl/fonts/Inter-latin.woff2')).toString('base64');
  const fontSVG = '<svg xmlns="http://www.w3.org/2000/svg" class="d2-fonts" width="200" height="60"><style type="text/css"><![CDATA[' +
    '.d2-fonts .text-bold { font-family: "d2-1-font-bold"; }\n@font-face { font-family: d2-1-font-bold; src: url("data:font/woff2;base64,' + font + '"); }\n' +
    '.d2-fonts .text-remote { font-family: "d2-1-font-remote"; }\n@font-face { font-family: d2-1-font-remote; src: url("https://example.test/font.woff"); }\n' +
    '@font-face { font-family: Inter; src: url("data:font/woff2;base64,' + font + '"); }]]></style>' +
    '<text class="text-bold" y="20">Bold Label</text><text class="text-remote" y="50">Remote</text></svg>';
  const t = await launch({
    viewport: { width: 1280, height: 1000 },
    app: () => {
      window.__calls = [];
      let n = 0;
      return {
        State: async () => ({ workspace: 'C:\\W', recent: ['C:\\W'], approvals: 'devin', devin: 'd', catalog: [], defaults: {}, pinned: [], titles: {}, templates: [] }),
        NewSessionAt: async () => ({ sessionId: 's' + (++n), cwd: 'C:\\W', title: '', open: true, working: false, queued: [] }),
        Send: async () => {},
        RenderD2: async (src) => { window.__calls.push(['RenderD2', src]); if (src.includes('broken')) throw 'd2: bad syntax'; if (src.includes('fonts')) return window.__fontSVG; return '<svg xmlns="http://www.w3.org/2000/svg" class="d2-drawn" width="100" height="40"><text y="20">D2</text></svg>'; },
        // Go turns a file address into its path and says the system opens it
        // (document_links.go); a program is shown in its folder.
        ResolveDocumentLink: async (cwd, ref) => { const p = decodeURIComponent(ref.replace(/^file:\/\/\//i, '').replace(/#.*$/, '')).replace(/\//g, '\\'); return { ref, found: true, path: p, opens: p.endsWith('.exe') ? 'reveal' : 'system' }; },
        OpenPath: async (p) => { window.__calls.push(['OpenPath', p]); return p.endsWith('.exe') ? 'shown' : 'opened'; }
      };
    },
    goto: false,
  });
  // The drawing's SVG is built in Node, so it goes to the page by an init
  // script of its own, which the fixture's init cannot take an argument for.
  await t.context.addInitScript(fontSVG => { window.__fontSVG = fontSVG; }, fontSVG);
  const p = require('./lib/windows')(t.page);
  const ok = t.ok;
  await p.goto(PAGE);
  await t.waitReady();
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Draw it.');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  await p.waitForTimeout(300);
  const say = (t) => p.evaluate((t) => window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: t } } }), t);
  await say('The process as d2:\n\n```d2\na -> b\n');
  await p.waitForTimeout(300);
  ok('an unfinished block is not drawn', (await p.evaluate(() => window.__calls)).filter(c => c[0] === 'RenderD2').length === 0);
  await say('```\n\nAnd as Mermaid:\n\n```mermaid\ngraph TD\n  Lexer --> Check\n  Check -->|ok| Tree\n```\n\nA bad one:\n\n```d2\nbroken {\n```\n\nThe design is [Parser Design  Draft v0.11.md:705-803](file:///C:/Work/Parser/docs/Parser%20Design%20%20Draft%20v0.11.md), and the tool is file:///C:/Tools/run.exe here.');
  await p.waitForTimeout(3000);
  ok('the d2 block is drawn', (await p.$$('.win[data-chat-current] [data-view-id=logs] .msg-diagram svg.d2-drawn')).length === 1);
  ok('the d2 source is folded under it', (await p.textContent('.win[data-chat-current] [data-view-id=logs] .msg-diagram details')).includes('a -> b'));
  const mm = await p.$$eval('.win[data-chat-current] [data-view-id=logs] .msg-diagram svg', s => s.filter(x => !x.classList.contains('d2-drawn')).length);
  ok('the Mermaid block is drawn by the library', mm === 1 && (await p.textContent('.win[data-chat-current] [data-view-id=logs]')).includes('Lexer'));
  await p.getByRole('button',{name:'Expand Mermaid Diagram',exact:true}).click();
  await p.locator('#media-viewer [data-media-stage] svg').waitFor();
  ok('Chat diagrams open the shared vector viewer',(await p.locator('#media-viewer [data-media-stage] svg').textContent()).includes('Lexer'));
  await p.keyboard.press('Escape');
  ok('a bad diagram says why and keeps its source', (await p.textContent('.win[data-chat-current] [data-view-id=logs] .msg-diagram-error')).includes('bad syntax'));
  const d2calls = (await p.evaluate(() => window.__calls)).filter(c => c[0] === 'RenderD2').length;
  await say(' More words.');
  await p.waitForTimeout(500);
  ok('streaming more text does not draw again', (await p.evaluate(() => window.__calls)).filter(c => c[0] === 'RenderD2').length === d2calls && (await p.$$('.win[data-chat-current] [data-view-id=logs] .msg-diagram svg.d2-drawn')).length === 1);
  await say('\n\nWith its fonts:\n\n```d2\nfonts -> drawn\n```\n');
  await p.locator('.win[data-chat-current] svg.d2-fonts').waitFor();
  const fonts = await p.locator('.win[data-chat-current] svg.d2-fonts').evaluate(async svg => {
    await document.fonts.ready;
    const style = [...svg.querySelectorAll('style')].map(s => s.textContent).join('\n');
    const bold = getComputedStyle(svg.querySelector('.text-bold')).fontFamily.replace(/"/g, '');
    const faces = [...document.fonts].map(f => ({ family: f.family.replace(/"/g, ''), status: f.status }));
    return { style, bold, faces, pageFont: getComputedStyle(document.body).fontFamily };
  });
  ok('a font the drawing brings is kept under a name of its own, and its labels use it', fonts.bold.endsWith('-font-1') && fonts.faces.some(f => f.family === fonts.bold && f.status === 'loaded'), JSON.stringify({ bold: fonts.bold }));
  ok('a font fetched from elsewhere is dropped', !fonts.style.includes('example.test') && (fonts.style.match(/@font-face/g) || []).length === 2);
  ok('a drawing\'s font never takes the name of one the page uses', !fonts.style.includes('font-family:"Inter"') && fonts.style.includes('-font-2"'));
  // The whole page, which t.shot does not offer.
  await p.screenshot({ path: path.join(OUT, 'diagrams.png'), fullPage: true });
  await p.click('.win[data-chat-current] [data-view-id=logs] a.smart-chip:has-text("Parser Design")');
  await p.waitForTimeout(200);
  let o = (await p.evaluate(() => window.__calls)).filter(c => c[0] === 'OpenPath').pop();
  ok('a Markdown file link opens the file by its path', o && o[1] === 'C:\\Work\\Parser\\docs\\Parser Design  Draft v0.11.md', o && o[1]);
  await p.click('.win[data-chat-current] [data-view-id=logs] a.smart-chip:has-text("file:///C:/Tools/run.exe")');
  await p.waitForTimeout(200);
  o = (await p.evaluate(() => window.__calls)).filter(c => c[0] === 'OpenPath').pop();
  ok('a bare file address is a link too', o && o[1] === 'C:\\Tools\\run.exe');
  ok('a program is shown, not started, and the person is told', (await p.textContent('.toast-region')).includes('shown in Explorer'));
  await t.close();
})();
