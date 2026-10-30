// A chat message streaming in is drawn without drawing it again from the
// start for every chunk. After every chunk it must show what a full redraw
// of the text so far would show, including Markdown whose meaning changes as
// later text arrives (a code fence split across chunks, a blank line inside
// a fence, a table whose rule comes later, a list that goes on after a blank
// line, a block of math whose closing dollars come later), and it must end
// as a message drawn in one piece. The blocks it has already drawn are kept,
// so the work for each chunk does not grow with the message. The suite also
// checks what markdown-it and KaTeX draw: nested and numbered lists, rules,
// quotations, math and the dollar rule, math kept out of code, hostile TeX,
// a diagram fence not drawn before it closes, and KaTeX not loaded until a
// message has math. The page is served under Dinah's own Content Security
// Policy, read from shell_server.go, and the suite fails on any violation.
// Against a stub of the Go side.
const { launch, PAGE, FRONTEND } = require('./lib/page');
const fs = require('fs');
const path = require('path');
(async () => {
  const outside = [];
  const policy = /const shellPolicy = ([\s\S]*?)\n\n/.exec(fs.readFileSync(path.join(__dirname, '..', '..', 'shell_server.go'), 'utf8'))[1]
    .match(/"([^"]*)"/g).map(s => s.slice(1, -1)).join('');
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => ({
      ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
      OpenSessionAt: async (ws, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: ws, open: true, working: false, queued: [] })
    }),
    init: () => {
      window.__violations = [];
      document.addEventListener('securitypolicyviolation', e => window.__violations.push(e.violatedDirective + ' ' + e.blockedURI));
    },
    ready: () => window.dinahWorkbench && window.dinahEditor && window.dinahChatText,
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    // The page's files under the Content Security Policy; this route is
    // registered after the fixture's, so it is the one that answers.
    await t.context.route('http://dinah.test/**', r => {
      const file = path.join(FRONTEND, new URL(r.request().url()).pathname);
      const headers = { 'Content-Security-Policy': policy };
      return r.fulfill({ path: file, headers });
    });
    // Nothing outside the page is fetched: a hostile picture or link in TeX
    // would show here.
    await t.context.route(url => !String(url).startsWith('http://dinah.test/'), r => { outside.push(r.request().url()); return r.abort(); });
    // An external source file lets the regression be checked against a
    // previous revision without replacing files in the working tree.
    if (process.env.DINAH_STREAM_APP_SCRIPT) await t.context.route('http://dinah.test/app.js', r => r.fulfill({ path: process.env.DINAH_STREAM_APP_SCRIPT }));
    await p.goto(PAGE);
    await t.waitReady();
    const id = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:markdown' }));
    const win = '[data-win="' + id + '"]';
    await p.locator(win + ' [data-view-id="prompt"]').waitFor();
    await p.evaluate(win => {
      window.__win = win;
      window.__say = text => __emit('agent:update', { sessionId: 'codex:markdown', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } } });
      window.__break = toolId => __emit('agent:update', { sessionId: 'codex:markdown', update: { sessionUpdate: 'tool_call', toolCallId: toolId, title: 'Ran', status: 'completed' } });
      window.__last = () => { const all = document.querySelectorAll(win + ' .msg-text'); return all[all.length - 1]; };
      // A full redraw of the text: what the message shows when it is drawn
      // in one piece, by the same path a chunk is drawn by.
      window.__full = text => { const box = document.createElement('div'); box.appendChild(dinahChatText.nodes(text)); return box.innerHTML; };
    }, win);

    // A message with dollars that are not math does not load KaTeX.
    await p.evaluate(() => { __break('plain'); __say('It costs $5 or $10, and $PATH is set. `$x$` is code.'); });
    await p.waitForFunction(() => __last() && __last().textContent.includes('is code'));
    await p.waitForTimeout(300);
    const plain = await p.evaluate(() => ({ katex: !!window.katex, tags: document.querySelectorAll('script[src*="katex"], link[href*="katex"]').length, math: __last().querySelectorAll('.msg-math').length, text: __last().textContent }));
    ok('KaTeX is not loaded for a message without math', !plain.katex && plain.tags === 0, JSON.stringify(plain));
    ok('"costs $5 or $10", "$PATH" and a code span stay text', plain.math === 0 && plain.text.includes('It costs $5 or $10, and $PATH is set.') && plain.text.includes('$x$ is code'), JSON.stringify(plain));

    // Each chunk is drawn before the next arrives, so every intermediate
    // state is compared, and the cut points fall inside fence markers, table
    // rows, list markers, dollars and words.
    const whole = 'A first paragraph with **bold** and `code`.\n\n- one\n- two\n\n' +
      '```js\nlet a = 1;\n\nlet b = 2;\n```\n\n' +
      '| A | B |\n| - | - |\n| 1 | 2 |\n\n' +
      '1. first\n2. second\n\n```\nunclosed until the end\n\nstill code\n```\nThe end.\n\n' +
      '3. third\n   1. nested one\n   2. nested two\n      - deeper\n\n   still the third item\n4. fourth\n\n' +
      '- loose\n\n- list\n  - nested\n\n' +
      '---\n\n' +
      '> A quotation\n> that goes on,\n>\n> - with a list\n\n' +
      'Inline $\\rightarrow$ and $>5\\text{ MD}$, but costs $5 or $10 and $PATH stay text.\n\n' +
      '$$\n\\frac{a}{b} = \\sqrt{c}\n$$\n\n' +
      'Code `$x$` stays code.\n\n```\n$y$ in a fence\n```\n\n###### Six hashes\n\nAfter.';
    await p.evaluate(() => { __break('first'); });
    let sofar = '', mismatch = '';
    for (let at = 0; at < whole.length; at += 5) {
      const piece = whole.slice(at, at + 5);
      sofar += piece;
      await p.evaluate(piece => __say(piece), piece);
      const same = await p.waitForFunction(text => { const el = __last(); return !!el && el.innerHTML === __full(text); }, sofar, { timeout: 5000 }).then(() => true, () => false);
      if (!same && !mismatch) mismatch = JSON.stringify(sofar) + ' drew ' + await p.evaluate(() => __last().innerHTML);
      if (at === 45) await p.evaluate(() => { __last().querySelector('p').__kept = true; });
      if (at === 105) await p.evaluate(() => { __last().querySelector('pre').__kept = true; });
    }
    ok('after every chunk the message shows what a full redraw shows', !mismatch, mismatch);
    const kept = await p.evaluate(() => { const el = __last(); return { p: !!el.querySelector('p').__kept, pre: !!el.querySelector('pre').__kept }; });
    ok('a settled paragraph and code block are kept, not drawn again', kept.p && kept.pre, JSON.stringify(kept));
    ok('the streamed message ends as a full redraw of its text', await p.evaluate(text => __last().innerHTML === __full(text), whole));
    const streamed = await p.evaluate(() => __last().innerHTML);
    await p.evaluate(text => { __break('second'); __say(text); }, whole);
    await p.waitForFunction(() => document.querySelectorAll(__win + ' .msg-text').length >= 3 && __last().textContent.includes('After.'));
    ok('the streamed message equals the same text arriving in one chunk', await p.evaluate(() => __last().innerHTML) === streamed);

    // What markdown-it and KaTeX drew.
    const drawn = await p.evaluate(() => {
      const el = __last(), q = s => el.querySelector(s), all = s => [...el.querySelectorAll(s)];
      const third = all('ol[start="3"] > li')[0];
      return {
        start: !!third && third.parentElement.getAttribute('start') === '3',
        nested: !!third && !!third.querySelector(':scope > ol > li > ul > li') && third.querySelector(':scope > ol').children.length === 2,
        lazyItem: !!third && third.textContent.includes('still the third item'),
        fourth: all('ol[start="3"] > li').length === 2,
        looseNested: !!q('ul > li > ul > li'),
        hr: !!q(':scope > hr'),
        quote: !!q(':scope > blockquote > p') && !!q(':scope > blockquote > ul > li'),
        inline: all('p .msg-math[data-math="inline"][data-typeset] .katex').length === 2,
        display: !!q(':scope > .msg-math[data-math="display"][data-typeset] .katex-display'),
        mathml: !!q('.msg-math math annotation'),
        dollars: all('p').some(p => p.textContent.includes('but costs $5 or $10 and $PATH stay text.')),
        codeSpan: all('code').some(c => c.textContent === '$x$' && !c.querySelector('.msg-math')),
        fence: all('pre').some(pre => pre.textContent === '$y$ in a fence' && !pre.querySelector('.msg-math')),
        h6: !!q('h6') && q('h6').textContent === 'Six hashes'
      };
    });
    const missing = Object.keys(drawn).filter(k => !drawn[k]);
    ok('nested and numbered lists, a rule, a quotation, math and six-hash headings are drawn', !missing.length, missing.join(', '));
    const fonts = await p.evaluate(async () => { await document.fonts.ready; return [...document.fonts].filter(f => /KaTeX/.test(f.family) && f.status === 'loaded').length; });
    ok('KaTeX loads its fonts from the page', fonts > 0, String(fonts));

    // KaTeX's HTML passes through safe-html.js unchanged.
    const untouched = await p.evaluate(() => {
      const tex = '\\frac{1}{2} + \\sqrt[3]{x} + \\overrightarrow{AB} + \\begin{pmatrix} a & b \\\\ c & d \\end{pmatrix} + \\color{red}{x} + \\text{a}';
      const html = katex.renderToString(tex, { displayMode: true, throwOnError: false, trust: false, strict: 'ignore', output: 'htmlAndMathml' });
      const box = document.createElement('div'); box.innerHTML = html;
      return box.innerHTML === dinahHTML.clean(html);
    });
    ok('safe-html.js keeps everything KaTeX draws', untouched);

    // Hostile TeX makes no link and loads nothing, and TeX KaTeX cannot read
    // is shown as its source.
    await p.evaluate(() => { __break('hostile'); __say('Click $\\href{javascript:alert(1)}{here}$ or $\\url{https://example.com/u}$, see $\\includegraphics{https://example.com/x.png}$, and $\\frac{1}{$ broken.\n\n$$\n\\href{https://example.com/d}{display}\n$$\n'); });
    await p.waitForFunction(() => __last().textContent.includes('broken') && __last().querySelectorAll('[data-typeset]').length === 5);
    await p.waitForTimeout(300);
    const hostile = await p.evaluate(() => ({
      links: __last().querySelectorAll('a').length, images: __last().querySelectorAll('img').length,
      hrefs: [...__last().querySelectorAll('[href], [src]')].length, text: __last().textContent
    }));
    ok('\\href, \\url and \\includegraphics in math make no link and load nothing', hostile.links === 0 && hostile.images === 0 && hostile.hrefs === 0 && !outside.length, JSON.stringify(hostile) + ' ' + outside.join(' '));
    ok('TeX KaTeX cannot read is shown as its source', hostile.text.includes('\\frac{1}{'), hostile.text);

    // A diagram fence still streaming in is not drawn, and is drawn once it
    // closes.
    await p.evaluate(() => { __break('diagram'); __say('```mermaid\ngraph TD\n  A-->B\n'); });
    await p.waitForFunction(() => __last().querySelector('pre[data-lang="mermaid"]'));
    await p.waitForTimeout(500);
    const open = await p.evaluate(() => ({ closed: __last().querySelector('pre[data-lang="mermaid"]').hasAttribute('data-closed'), figure: !!__last().querySelector('figure.msg-diagram') }));
    ok('a half-streamed diagram fence is not drawn', !open.closed && !open.figure, JSON.stringify(open));
    await p.evaluate(() => __say('```\n\nDone.'));
    const drew = await p.waitForFunction(() => __last().querySelector('figure.msg-diagram svg'), null, { timeout: 8000 }).then(() => true, () => false);
    ok('the diagram is drawn once its fence closes', drew);

    // The work for each chunk is bounded: the paragraphs already drawn stay
    // in place, and only the part still arriving is replaced.
    const count = 150;
    await p.evaluate(() => {
      __break('third'); __say('P');
    });
    await p.waitForFunction(() => __last().textContent === 'P');
    await p.evaluate(() => {
      window.__removed = 0;
      window.__observer = new MutationObserver(list => list.forEach(r => { window.__removed += r.removedNodes.length; }));
      window.__observer.observe(__last(), { childList: true });
    });
    let text = 'P';
    for (let i = 0; i < count; i++) {
      const piece = 'aragraph ' + i + '.\n\nP';
      text += piece;
      await p.evaluate(piece => __say(piece), piece);
      await p.waitForFunction(n => __last().children.length === n + 1, i + 1, { timeout: 5000 }).catch(() => {});
      if (i === 0) await p.evaluate(() => { __last().firstElementChild.__kept = true; });
    }
    const bounded = await p.evaluate(text => ({ removed: window.__removed, first: !!__last().firstElementChild.__kept, same: __last().innerHTML === __full(text) }), text);
    ok('each chunk replaces only the block still arriving', bounded.removed <= 2 * count && bounded.first, JSON.stringify(bounded));
    ok('the long streamed message ends as a full redraw of its text', bounded.same);
    const violations = await p.evaluate(() => window.__violations);
    ok('the page breaks none of its Content Security Policy', !violations.length, violations.join(' | '));
  } catch (e) {
    t.errors.push(String(e && e.stack || e));
  } finally {
    await t.close();
  }
})();
