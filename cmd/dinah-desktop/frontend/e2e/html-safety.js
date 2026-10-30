// HTML from outside Dinah cannot run script on the page. The page is served
// with the Content Security Policy the shell server sends (read from
// shell_server.go, so this suite tests the policy Dinah uses), an agent's
// message with hostile HTML in it is drawn as text, and a document's preview
// with hostile HTML in it keeps its content without its script, frames or
// unsafe addresses (safe-html.js).
const path = require('path');
const fs = require('fs');
const { launch, PAGE, FRONTEND } = require('./lib/page');

// The policy is the shellPolicy constant's string literals, joined.
function shellPolicy() {
  const source = fs.readFileSync(path.join(__dirname, '..', '..', 'shell_server.go'), 'utf8');
  const decl = /const shellPolicy = ([\s\S]*?)\n\n/.exec(source);
  if (!decl) throw new Error('shell_server.go has no shellPolicy constant');
  return [...decl[1].matchAll(/"((?:[^"\\]|\\.)*)"/g)].map(m => m[1]).join('');
}

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => ({
      AgentProviders: async () => [{ id: 'codex', installed: true, available: true }], SavedConversation: async () => ({ available: false }),
      OpenSessionAt: async (cwd, sessionId) => ({ sessionId, cwd, provider: 'codex', title: 'Hostile HTML', open: true, working: false, queued: [] })
    }),
    init: () => {
      window.__violations = [];
      document.addEventListener('securitypolicyviolation', e => window.__violations.push(e.effectiveDirective));
    },
    ready: () => window.dinahWorkbench && window.dinahHTML && window.dinahDocumentPreview,
    goto: false,
  });
  const { page, ok } = t;
  try {
    const policy = shellPolicy();
    ok('the shell server\'s policy was read', /script-src 'self'/.test(policy), policy);
    // The page's files are served with the policy, and a file that is not
    // there is not found; this route is tried before the fixture's.
    await t.context.route('http://dinah.test/**', route => {
      const file = path.join(FRONTEND, new URL(route.request().url()).pathname);
      if (!fs.existsSync(file)) return route.fulfill({ status: 404, body: '' });
      return route.fulfill({ path: file, headers: { 'Content-Security-Policy': policy } });
    });
    await page.goto(PAGE);
    await t.waitReady();

    // The policy is in force: an event handler written straight into the
    // page, past the sanitizer, does not run, and the browser reports it.
    await page.evaluate(() => {
      const box = document.createElement('div');
      box.id = 'raw-handler';
      box.innerHTML = '<img src="data:image/png;base64,AAAA" onerror="window.__ran = \'handler\'">';
      document.body.appendChild(box);
    });
    await page.waitForFunction(() => window.__violations.length > 0, null, { timeout: 5000 }).catch(() => {});
    ok('a Content Security Policy is in force on the page', await page.evaluate(() => window.__ran === undefined && window.__violations.some(d => /^script-src/.test(d))),
      JSON.stringify(await page.evaluate(() => ({ ran: window.__ran, violations: window.__violations }))));

    // An agent's message with hostile HTML in it is drawn as its text.
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:hostile' }));
    const win = page.locator('[data-win="' + key + '"]');
    await win.locator('[data-view-id="prompt"]').waitFor();
    const hostile = 'Look <img src=x onerror="window.__ran=\'message\'"> and <script>window.__ran=\'script\'</script>\n\n' +
      '<iframe srcdoc="<script>parent.__ran=1</script>"></iframe> [a link](javascript:window.__ran=\'link\') **done**';
    await page.evaluate(text => __emit('agent:update', { sessionId: 'codex:hostile', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } } }), hostile);
    await page.waitForFunction(k => [...document.querySelectorAll('[data-win="' + CSS.escape(k) + '"] .msg-text')].some(el => el.textContent.includes('done')), key);
    const message = await win.locator('.msg-text').last().evaluate(el => ({
      text: el.textContent, elements: el.querySelectorAll('img,script,iframe').length,
      unsafe: [...el.querySelectorAll('*')].some(n => [...n.attributes].some(a => /^on/i.test(a.name) || /javascript:/i.test(a.value))),
      strong: !!el.querySelector('strong')
    }));
    ok('a hostile agent message is drawn as text', message.text.includes('<img src=x') && message.text.includes('<script>') && message.elements === 0 && !message.unsafe, JSON.stringify(message));
    ok('the message keeps its Markdown', message.strong);

    // A document's preview keeps what is safe in the HTML written in it and
    // loses the rest.
    const preview = await page.evaluate(() => {
      const host = document.createElement('div');
      host.id = 'hostile-preview';
      document.body.appendChild(host);
      const html = '<div class="dinah-block" data-line="1">' +
        '<p id="intro" style="color: red">Intro <a href="https://example.com/">web</a> <a href="#intro">here</a> <a href="mailto:a@example.com">mail</a> <a href="other.md">other</a></p>' +
        '<img src="data:image/png;base64,AAAA" onerror="window.__ran=\'preview\'" alt="pic">' +
        '<a id="js" href="javascript:window.__ran=1">js</a><a id="tabbed" href="java&#9;script:window.__ran=1">tabbed</a><a id="data" href="data:text/html,<script>alert(1)</script>">data</a>' +
        '<script>window.__ran = \'preview script\'</script><iframe src="https://example.com/"></iframe><object data="x"></object><embed src="x">' +
        '<form action="https://example.com/"><button formaction="https://example.com/">Go</button></form><base href="https://example.com/"><meta http-equiv="refresh" content="0; url=https://example.com/">' +
        '<style>body { display: none }</style>' +
        '<svg><style>.k { fill: red }</style><a href="#intro"><text>svg link</text><animate attributeName="href" to="javascript:window.__ran=1"/></a>' +
        '<a xlink:href="javascript:window.__ran=1"><text>xlink</text></a><set attributeName="xlink:href" to="javascript:1"/><script>window.__ran=1</script></svg>' +
        '</div>';
      return dinahDocumentPreview.renderBlocks(host, [{ line: 1, html }], 'safety').then(() => {
        const attrs = [...host.querySelectorAll('*')].flatMap(n => [...n.attributes].map(a => a.name + '=' + a.value));
        return {
          text: host.textContent,
          dropped: host.querySelectorAll('script,iframe,object,embed,form,base,meta,animate,set').length,
          htmlStyle: [...host.querySelectorAll('style')].filter(s => !s.closest('svg')).length,
          svgStyle: host.querySelectorAll('svg style').length,
          handlers: attrs.filter(a => /^on/i.test(a)),
          unsafe: attrs.filter(a => /^(href|src|xlink:href|formaction|action)=\s*(java\s*script|data:text)/i.test(a)),
          web: !!host.querySelector('a[href="https://example.com/"]'),
          mail: !!host.querySelector('a[href="mailto:a@example.com"]'),
          relative: !!host.querySelector('a[href="other.md"]'),
          picture: !!host.querySelector('img[src^="data:image/png"]'),
          style: !!host.querySelector('p[style]'),
          jsLinks: ['js', 'tabbed', 'data'].map(id => host.querySelector('[id$="' + id + '"]')).filter(a => a && a.hasAttribute('href')).length,
          xlink: [...host.querySelectorAll('svg a')].filter(a => a.hasAttribute('xlink:href') || /javascript/i.test(a.getAttribute('href') || '')).length
        };
      });
    });
    ok('a preview loses script, frames, plug-ins, forms, a base address and a page stylesheet', preview.dropped === 0 && preview.htmlStyle === 0, JSON.stringify(preview));
    ok('a preview loses event handlers and unsafe addresses', preview.handlers.length === 0 && preview.unsafe.length === 0 && preview.jsLinks === 0 && preview.xlink === 0, JSON.stringify(preview));
    ok('a preview keeps its text, web, mail and relative links, a picture, inline style and SVG style',
      preview.text.includes('Intro') && preview.text.includes('svg link') && preview.web && preview.mail && preview.relative && preview.picture && preview.style && preview.svgStyle === 1, JSON.stringify(preview));
    ok('the page is still shown', await page.evaluate(() => getComputedStyle(document.body).display !== 'none'));

    await page.waitForTimeout(300);
    ok('nothing hostile ran', await page.evaluate(() => window.__ran === undefined), await page.evaluate(() => String(window.__ran)));
    const at = new URL(page.url());
    ok('the page is still Dinah', at.origin + at.pathname === PAGE, page.url());
  } catch (error) {
    ok('the suite ran', false, String(error && error.stack || error));
  } finally {
    await t.close();
  }
})();
