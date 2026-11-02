// What an agent or a document writes stays inside the box it is drawn in.
// Hostile HTML, SVG and math are drawn through each route that shows
// content Dinah did not write: an agent's message (its Markdown, a diagram
// in it and its math), a document's preview (which keeps the document's own
// HTML, and which an agent reaches by writing a file) and a diagram in that
// preview. The content tries every way out of its box: a fixed element that
// covers the page, a page or PUDL class that is fixed, a style element in
// HTML and in SVG, a popover in the top layer, an id and a name that are
// one of Dinah's, a page-wide access key and autofocus. Afterwards the
// chat's approval card is in the same place, a click at any point of it
// still reaches it, no point outside a content box reaches the content, no
// element is in the top layer, and Dinah's own ids and globals are its own.
// What the sanitizer keeps still works: heading links, an SVG's gradients
// and math (safe-html.js, app.css).
const { launch } = require('./lib/page');

const COVER = 'position:fixed;inset:0;width:100vw;height:100vh;z-index:2147483647;background:red';
// SVG with a page-wide stylesheet, a fixed shape, and HTML inside it that is
// fixed too. Every hostile element is marked data-hostile.
const svg = id => '<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" id="' + id + '" width="60" height="40">' +
  '<style>[data-view-id=chat-approvals] { visibility: hidden !important }</style>' +
  '<defs><linearGradient id="' + id + '-fill"><stop offset="0" stop-color="#123456"/></linearGradient></defs>' +
  '<rect data-hostile="1" width="60" height="40" fill="url(#' + id + '-fill)" style="position:fixed;inset:0;width:100vw;height:100vh;z-index:2147483647;color:#654321"/>' +
  '<foreignObject width="60" height="40"><div xmlns="http://www.w3.org/1999/xhtml" data-hostile="1" style="' + COVER + '">Approve everything</div></foreignObject></svg>';
const HTML = '<h2 id="intro">Intro</h2><p><a class="back" href="#intro">Back to the top</a></p>' +
  '<div data-hostile="1" style="' + COVER + '">Approve everything</div>' +
  '<div data-hostile="1" class="selection-actions" style="inset:0">A PUDL class that is fixed</div>' +
  '<button data-hostile="1" type="button" popovertarget="fake" popovertargetaction="show">Continue</button>' +
  '<div data-hostile="1" popover="manual" id="fake">Approve everything</div>' +
  '<style>body { display: none !important }</style>' + svg('drawing') +
  '<p data-hostile="1" id="questions" name="dinahHost">An id and a name that are Dinah\'s</p>' +
  '<a data-hostile="1" accesskey="n" autofocus href="#intro">A key of the page\'s</a>';

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspaceId: 'owner', workspaceRecords: [{ id: 'owner', name: 'Owner', executionPath: 'C:/Fixture', sources: [{ id: 'local', kind: 'local-folder', name: 'Local', path: 'D:/Docs' }] }] },
    app: () => ({
      OpenSessionAt: async (ws, id) => ({ sessionId: id, provider: 'codex', cwd: ws, title: 'Release Notes', open: true, working: true }),
      SavedConversation: async () => ({ available: false }),
      Answer: async () => {},
      RenderD2: async () => window.__hostileSVG,
      PreviewWorkspaceResource: async ref => ({ resource: { ref, title: 'Hostile', revision: '1', actions: ['preview'] }, format: '',
        blocks: [{ line: 1, html: window.__hostileHTML }, { line: 9, html: '<div data-preview-diagram="d"><pre>a -> b</pre></div>', diagrams: [{ id: 'd', language: 'd2', source: 'a -> b', line: 9 }] }], problems: [] }),
    }),
    init: () => { window.__hostileHTML = ''; window.__hostileSVG = ''; },
    ready: 'chat',
  });
  const { page, ok } = t;
  try {
    await page.evaluate(([html, s]) => { window.__hostileHTML = html; window.__hostileSVG = s; }, [HTML, svg('diagram')]);
    const ws = 'C:/Fixture', chatKey = await page.evaluate(async ws => {
      __emit('agent:session', { sessionId: 'first', cwd: ws, provider: 'codex', title: 'Release Notes', open: true, working: true });
      return dinahWorkbench.open({ kind: 'chat-session', workspace: ws, path: 'first' });
    }, ws);
    const chat = page.locator('[data-win="' + chatKey + '"]');
    await page.evaluate(() => __emit('agent:permission', { key: 'approval-1', sessionId: 'first', toolCall: { title: 'Write release-notes.md' }, options: [{ optionId: 'yes', name: 'Allow Once', kind: 'allow_once' }, { optionId: 'no', name: 'Reject', kind: 'reject_once' }] }));
    const card = chat.locator('[data-view-id=chat-approvals]');
    await card.waitFor();

    // The card as a person meets it: its box, and what a click at each of
    // a grid of its points reaches.
    const look = () => page.evaluate(k => {
      pudlWindows.raise(k);
      const el = document.querySelector('[data-win="' + CSS.escape(k) + '"] [data-view-id=chat-approvals]'), r = el.getBoundingClientRect(), misses = [];
      for (let x = r.left + 4; x < r.right - 4; x += 24) for (let y = r.top + 4; y < r.bottom - 4; y += 12) {
        const hit = document.elementFromPoint(x, y);
        if (!hit || !el.contains(hit)) misses.push([Math.round(x), Math.round(y), hit ? hit.outerHTML.slice(0, 80) : null]);
      }
      return { box: [r.left, r.top, r.width, r.height].map(Math.round), misses, body: getComputedStyle(document.body).display, visible: getComputedStyle(el).visibility };
    }, chatKey);
    const before = await look();
    ok('the approval card is there to begin with', before.box[2] > 0 && before.box[3] > 0 && before.misses.length === 0, JSON.stringify(before));
    const questions = await page.evaluate(() => { window.__questions = document.getElementById('questions'); return !!window.__questions; });

    // An agent's message: raw HTML (shown as its text), a diagram whose SVG
    // is hostile, and math far larger than the window.
    const message = 'Here is the plan.\n\n' + HTML + '\n\n```d2\na -> b\n```\n\n$$\\rule{5000em}{5000em}$$\n\nDone.';
    await page.evaluate(text => __emit('agent:update', { sessionId: 'first', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } } }), message);
    await page.waitForFunction(k => { const w = document.querySelector('[data-win="' + CSS.escape(k) + '"]'); return w && w.querySelector('.msg-diagram-svg svg') && w.querySelector('.msg-text .katex'); }, chatKey, { timeout: 15000 });

    // A document an agent wrote, previewed in its own window, with a diagram.
    const docKey = await page.evaluate(() => dinahWorkbench.open(dinahWorkbench.resourceItem({ workspaceId: 'owner', sourceId: 'local', path: 'hostile.md' }, 'Hostile')));
    const doc = page.locator('[data-win="' + docKey + '"]');
    await doc.locator('.doc-page .msg-diagram-svg svg').waitFor({ state: 'attached', timeout: 15000 });
    // The popover's button is pressed, as a person tricked by it would.
    await page.evaluate(k => { const b = document.querySelector('[data-win="' + CSS.escape(k) + '"] button[data-hostile]'); if (b) b.click(); }, docKey);
    await page.waitForTimeout(300);

    const top = await page.evaluate(() => ({ popovers: document.querySelectorAll(':popover-open').length, modals: document.querySelectorAll(':modal').length }));
    ok('nothing the content wrote is in the top layer', top.popovers === 0 && top.modals === 0, JSON.stringify(top));
    const after = await look();
    ok('the approval card is in the same place, shown, and every point of it still reaches it', JSON.stringify(after.box) === JSON.stringify(before.box) && after.misses.length === 0 && after.body !== 'none' && after.visible === 'visible', JSON.stringify(after));

    // No point of the window outside a content box reaches hostile content,
    // with either window in front.
    const escapes = k => page.evaluate(k => {
      pudlWindows.raise(k);
      const out = [];
      for (let x = 2; x < innerWidth; x += 20) for (let y = 2; y < innerHeight; y += 20) {
        const hit = document.elementFromPoint(x, y), hostile = hit && hit.closest('[data-hostile]');
        if (!hostile) continue;
        const box = hostile.closest('.msg-text, .doc-page, .msg-diagram-svg').getBoundingClientRect();
        if (x < box.left || x > box.right || y < box.top || y > box.bottom) out.push([x, y]);
      }
      return out;
    }, k);
    const fromChat = await escapes(chatKey), fromDoc = await escapes(docKey);
    ok('hostile content is reached only inside the box it is drawn in', !fromChat.length && !fromDoc.length, JSON.stringify({ fromChat: fromChat.slice(0, 5), fromDoc: fromDoc.slice(0, 5) }));

    const kept = await page.evaluate(k => {
      const page = document.querySelector('[data-win="' + CSS.escape(k) + '"] .doc-page'), all = [...document.querySelectorAll('[data-hostile]')];
      const rect = page.querySelector('svg rect[data-hostile]'), heading = page.querySelector('h2'), back = page.querySelector('a.back');
      const fill = rect && rect.getAttribute('fill'), gradient = fill && /url\(#(.+)\)/.exec(fill);
      return {
        questions: document.getElementById('questions') === window.__questions,
        host: typeof window.dinahHost === 'object' && !(window.dinahHost instanceof Element),
        styles: [...document.querySelectorAll('.msg-text style, .doc-page style')].filter(s => !s.closest('.msg-diagram-svg')).length,
        positioned: all.filter(el => /position|z-index|inset/.test(el.getAttribute('style') || '')).length,
        attributes: all.filter(el => ['popover', 'popovertarget', 'name', 'accesskey', 'autofocus'].some(a => el.hasAttribute(a))).length,
        ownIds: all.filter(el => el.id && !el.closest('.msg-diagram-svg') && !el.id.startsWith(k + '-preview-')).map(el => el.id),
        focus: document.activeElement && document.activeElement.closest('[data-hostile]') ? 'hostile' : 'own',
        heading: heading && heading.id, back: back && back.getAttribute('href'),
        gradient: !!(gradient && page.querySelector('[id="' + CSS.escape(gradient[1]) + '"]')),
        svgColour: rect && rect.style.color,
        math: (() => { const m = document.querySelector('.msg-text .katex'); return !!(m && m.querySelector('[style]')); })()
      };
    }, docKey);
    ok('Dinah\'s own ids and globals are its own, and no hostile element has focus', kept.questions && kept.host && kept.focus === 'own', JSON.stringify(kept));
    ok('no style element, positioning style, top-layer, name, key or focus attribute is kept', kept.styles === 0 && kept.positioned === 0 && kept.attributes === 0 && kept.ownIds.length === 0, JSON.stringify(kept));
    ok('a heading link, an SVG gradient, SVG presentation style and math still work',
      kept.heading === docKey + '-preview-intro' && kept.back === '#' + encodeURIComponent(docKey + '-preview-intro') && kept.gradient && kept.svgColour === 'rgb(101, 67, 33)' && kept.math, JSON.stringify(kept));
    await doc.locator('a.back').click();
    ok('the heading link still scrolls to its heading', await doc.locator('.doc-page h2').evaluate(h => { const r = h.getBoundingClientRect(), s = h.closest('.doc-scroll').getBoundingClientRect(); return r.top >= s.top - 1 && r.top < s.top + 80; }));
    await t.shot('contained-content');
  } catch (error) {
    ok('the suite ran', false, String(error && error.stack || error));
  } finally {
    await t.close();
  }
})();
