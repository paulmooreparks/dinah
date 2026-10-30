/* How a message is drawn, shared by every window that shows messages: a
   chat (app.js), a group chat and a thread (bus-applets.js), and a
   subagent's window (subagents.js). A message is a card with a head, which
   has its label and its tools, its text drawn from Markdown, and a foot that
   repeats the tools when the message is long. markdown-it draws the
   Markdown with HTML in the text shown as text, and the result is made safe
   as well (safe-html.js), since the text is an agent's, another chat's or
   another person's. Web and file addresses become links, a path to a file
   or folder that is there becomes a link that opens it in Dinah, a fenced
   d2 or Mermaid block becomes its diagram (diagrams.js), and TeX math is
   drawn by KaTeX. docs/design/chat-markdown.md records how Markdown is
   drawn, docs/design/chat-rendering.md how a message streaming in is drawn,
   and docs/plans/document-links.md how links to documents work. */
(function () {
  'use strict';
  const app = window.dinahHelpers.app;
  function toast(text, kind) { if (window.dinahHost) window.dinahHost.toast(text, kind); }
  function fail(err) { if (window.dinahHost) window.dinahHost.fail(err); }

  /* ---- Markdown ----------------------------------------------------------
     A message's Markdown is parsed and drawn by markdown-it, vendored in
     vendor/markdown-it-15.0.3: CommonMark with GitHub's tables and
     strikethrough, with HTML in the text shown as text, web and file
     addresses in the text made links, and typographic replacements off.
     Dinah's additions are markdown-it rules on the same parse, never a
     second parser: links become chips, a fenced block says its language and
     whether its closing fence has arrived, a table is wrapped so that it
     scrolls, a picture becomes a chip rather than loading, and TeX math
     between dollars is marked for KaTeX (see Math, below).
     docs/design/chat-markdown.md records the whole of it. */
  var md = window.markdownit({ html: false, linkify: true, typographer: false });
  // A link reference definition changes links written before it, so a
  // message streaming in would draw a settled block differently once the
  // definition arrived. Reference links are rare in agents' messages, and
  // the definition is shown as the text it is.
  md.disable('reference');
  // Only web and file addresses written with their scheme become links: a
  // file such as README.md is not taken for a web site, and neither is an
  // email address. linkify-it's validator for a scheme returns the length
  // of the address after the scheme.
  md.linkify.set({ fuzzyLink: false, fuzzyEmail: false, fuzzyIP: false });
  md.linkify.add('ftp:', null).add('mailto:', null).add('//', null);
  var FILE_TAIL = /^\/\/[^\s<>"'`]*[^\s<>"'`.,;:!?)\]]/;
  md.linkify.add('file:', { validate: function (text, pos) { var m = FILE_TAIL.exec(text.slice(pos)); return m ? m[0].length : 0; } });
  /* A link may go to a web address, a file address, or a path, which is
     read from the folder of the workspace that wrote the message. Any other
     scheme, a place on the page and an address that begins with two slashes
     are not links, and markdown-it then shows the Markdown as text. A
     Windows path such as C:\x is a path, though its drive letter looks like
     a scheme. */
  md.validateLink = function (url) {
    if (/^[a-z]:(?:[\\/]|%5c)/i.test(url)) return true;
    var scheme = /^([a-z][a-z0-9+.-]*):/i.exec(url);
    if (scheme) return /^(?:https?|file)$/i.test(scheme[1]);
    return url !== '' && !/^(?:#|\/\/)/.test(url);
  };
  var esc = md.utils.escapeHtml;
  // The attributes that make a link a chip: the address goes in a data
  // attribute that openChip reads, and the link itself goes nowhere. A link
  // to a file says where it opens once Dinah has said (linkPaths, below).
  function chip(href) {
    if (/^https?:/i.test(href)) return ' class="smart-chip" href="#" data-chip-url="' + esc(href) + '" title="Open in the Browser"';
    if (/^file:/i.test(href)) return ' class="smart-chip" href="#" data-chip-file="' + esc(href) + '" title="Open the File"';
    return ' class="smart-chip" href="#" data-chip-local="' + esc(md.normalizeLinkText(href)) + '" title="Open the File"';
  }
  md.renderer.rules.link_open = function (tokens, i) { return '<a' + chip(String(tokens[i].attrGet('href'))) + '>'; };
  // A picture's address is shown as a chip with its description, so that
  // a message never loads anything from the web.
  md.renderer.rules.image = function (tokens, i, options, env, self) {
    var t = tokens[i], src = String(t.attrGet('src')), alt = self.renderInlineAsText(t.children || [], options, env);
    return '<a' + chip(src) + '>' + esc(alt || src) + '</a>';
  };
  md.renderer.rules.table_open = function () { return '<div class="msg-table"><table>\n'; };
  md.renderer.rules.table_close = function () { return '</table></div>\n'; };
  /* A fence has arrived whole when its source lines are its opening line,
     its content and one line more, which is the closing fence. A fence
     still streaming in runs to the end of the text, or of the list item or
     quotation it is in, so its lines are its opening line and its content
     alone. The count uses only the token's documented map and content. */
  function contentLines(s) { return s === '' ? 0 : s.split('\n').length - (s.endsWith('\n') ? 1 : 0); }
  function fenceClosed(t) { return !!t.map && t.map[1] - t.map[0] - 1 - contentLines(t.content) === 1; }
  /* A fenced block says its language, and whether its closing fence has
     arrived, which decide whether a d2 or Mermaid block is drawn as a
     diagram (drawDiagrams). */
  md.renderer.rules.fence = function (tokens, i) {
    var t = tokens[i], lang = (t.info ? md.utils.unescapeAll(t.info).trim().split(/\s+/)[0] : '').toLowerCase();
    return '<pre' + (lang ? ' data-lang="' + esc(lang) + '"' : '') + (fenceClosed(t) ? ' data-closed="1"' : '') + '><code>' + esc(t.content.replace(/\n$/, '')) + '</code></pre>\n';
  };

  /* ---- Math --------------------------------------------------------------
     TeX between dollars is math, by the rule GitHub and Pandoc follow.
     $...$ is math in the line when no space is just inside either dollar
     and no digit follows the closing one; the closing dollar is the next
     one, so when that one breaks the rule there is no math, and "costs $5
     or $10" and "$PATH" stay text. $$...$$ is display math, in a line or as
     a block of its own. A backslash before a dollar makes it a dollar sign.
     markdown-it tries inline rules at the characters its text rule stops
     at, which include the dollar, a character it keeps for extensions such
     as this one. Code is never math:
     markdown-it reads a code span or block before this rule sees its
     dollars, and math whose TeX would contain a backtick is not taken, so a
     code span that begins inside it stays code. The math is drawn as its
     source in an element marked data-math, which typeset() turns into
     KaTeX's drawing once KaTeX has loaded. */
  function space(c) { return c === 0x20 || c === 0x09 || c === 0x0A || c === 0x0D || isNaN(c); }
  function digit(c) { return c >= 0x30 && c <= 0x39; }
  function mathInline(state, silent) {
    var src = state.src, start = state.pos, max = state.posMax;
    if (src.charCodeAt(start) !== 0x24) return false;
    var display = src.charCodeAt(start + 1) === 0x24, from = start + (display ? 2 : 1), end = -1;
    if (from >= max || (!display && space(src.charCodeAt(from)))) return false;
    for (var at = from; at < max; at++) {
      var c = src.charCodeAt(at);
      if (c === 0x5C) { at++; continue; }
      if (c !== 0x24) continue;
      if (display) {
        if (src.charCodeAt(at + 1) === 0x24 && at > from) { end = at; break; }
        continue;
      }
      if (!space(src.charCodeAt(at - 1)) && !digit(src.charCodeAt(at + 1))) end = at;
      break;
    }
    if (end < 0) return false;
    var tex = src.slice(from, end);
    if (tex.indexOf('`') >= 0 || !tex.trim()) return false;
    if (!silent) {
      var t = state.push('math_inline', 'span', 0);
      t.markup = display ? '$$' : '$';
      t.content = tex;
    }
    state.pos = end + (display ? 2 : 1);
    return true;
  }
  /* A block of display math is a line that is $$...$$, or $$ on a line of
     its own, the TeX, and a line that ends with $$. A line that begins with
     $$ and goes on with more than math is a paragraph, whose math the
     inline rule finds. A block whose end has not arrived runs to the end of
     the text, or of the list item or quotation it is in, as a fence does,
     and is not drawn. */
  function mathBlock(state, startLine, endLine, silent) {
    var pos = state.bMarks[startLine] + state.tShift[startLine], max = state.eMarks[startLine];
    if (state.sCount[startLine] - state.blkIndent >= 4) return false;
    if (state.src.slice(pos, pos + 2) !== '$$') return false;
    var first = state.src.slice(pos + 2, max).trim(), oneLine = first.length >= 2 && first.slice(-2) === '$$';
    if (first && !oneLine) return false;
    if (silent) return true;
    var parts = [], closed = false, next = startLine;
    if (oneLine) { parts.push(first.slice(0, -2)); closed = true; }
    else {
      for (next = startLine + 1; next < endLine; next++) {
        var p = state.bMarks[next] + state.tShift[next], e = state.eMarks[next];
        if (p < e && state.sCount[next] < state.blkIndent) break;
        var line = state.src.slice(p, e).trim();
        if (line.slice(-2) === '$$') { parts.push(line.slice(0, -2)); closed = true; break; }
        parts.push(line);
      }
    }
    state.line = closed ? next + 1 : next;
    var t = state.push('math_block', 'div', 0);
    t.block = true;
    t.markup = '$$';
    t.content = parts.join('\n').trim();
    t.map = [startLine, state.line];
    t.meta = { closed: closed };
    return true;
  }
  md.inline.ruler.before('escape', 'math_inline', mathInline);
  md.block.ruler.before('fence', 'math_block', mathBlock, { alt: ['paragraph', 'reference', 'blockquote', 'list'] });
  function mathSource(t) { return esc(t.markup + t.content + t.markup); }
  md.renderer.rules.math_inline = function (tokens, i) {
    var t = tokens[i], display = t.markup === '$$';
    return '<span class="msg-math' + (display ? ' msg-math-display' : '') + '" data-math="' + (display ? 'display' : 'inline') + '" data-tex="' + esc(t.content) + '">' + mathSource(t) + '</span>';
  };
  md.renderer.rules.math_block = function (tokens, i) {
    var t = tokens[i];
    if (!t.meta || !t.meta.closed) return '<div class="msg-math msg-math-display">' + esc(t.markup + (t.content ? '\n' + t.content : '')) + '</div>\n';
    return '<div class="msg-math msg-math-display" data-math="display" data-tex="' + esc(t.content) + '">' + mathSource(t) + '</div>\n';
  };

  /* markdown(src) returns the message's HTML, which safe-html.js makes safe
     before it is placed (nodes). */
  function markdown(src) { return md.render(String(src)); }

  /* settledLength returns the length of the text up to its last settled
     point: the Markdown before it draws the same whatever follows, and what
     follows draws as if it began the text. The point comes from
     markdown-it's own parse of the text's complete lines (a line still
     arriving can change what the line is). Every top-level block but the
     last is settled, since a later line can change only the block that is
     still open. The last is settled too when nothing can be added to it: a
     heading or a rule; a fence or a block of math whose closing line has
     arrived; or a paragraph, a quotation or a table that a blank line has
     ended. A list or an indented code block can go on after blank lines, so
     it is never settled while it is last. */
  function settledLength(text) {
    var end = text.lastIndexOf('\n') + 1;
    if (!end) return 0;
    var src = text.slice(0, end), tokens = md.parse(src, {}), last = null, lines = 0;
    for (var k = 0; k < src.length; k++) if (src.charCodeAt(k) === 0x0A) lines++;
    tokens.forEach(function (t) { if (t.level === 0 && t.nesting >= 0 && t.map) last = t; });
    if (!last) return end;
    var settled;
    switch (last.type) {
      case 'heading_open': case 'hr': settled = true; break;
      case 'fence': settled = fenceClosed(last); break;
      case 'math_block': settled = !!(last.meta && last.meta.closed); break;
      case 'paragraph_open': case 'blockquote_open': case 'table_open': settled = last.map[1] < lines; break;
      default: settled = false;
    }
    var line = settled ? lines : last.map[0], at = 0;
    for (var n = 0; n < line; n++) at = src.indexOf('\n', at) + 1;
    return at;
  }

  /* typeset(root) draws the math in root with KaTeX, vendored in
     vendor/katex-0.19.1. KaTeX is loaded the first time a message has math,
     its script and stylesheet added to the page then, so that a chat
     without math never loads it; math drawn before it arrives shows its
     source until then. KaTeX is given no trust, so \href, \url and
     \includegraphics make no link and load nothing, and TeX it cannot read
     is shown as its source in the theme's danger colour rather than
     stopping the message. Its HTML is made safe as any message's is. */
  var KATEX = 'vendor/katex-0.19.1/', katexState = '', waiting = [];
  function loadKatex() {
    if (katexState) return;
    katexState = 'loading';
    var css = document.createElement('link');
    css.rel = 'stylesheet'; css.href = KATEX + 'katex.min.css';
    document.head.appendChild(css);
    var script = document.createElement('script');
    script.src = KATEX + 'katex.min.js';
    script.onload = function () {
      katexState = 'ready';
      var list = waiting; waiting = [];
      list.forEach(typesetOne);
    };
    script.onerror = function () { katexState = 'failed'; waiting = []; script.remove(); };
    document.head.appendChild(script);
  }
  // KaTeX takes its error colour as a hex colour, so the theme's danger
  // colour is turned into one by a canvas, whose fillStyle reads back an
  // opaque colour as #rrggbb.
  function errorColor() {
    var v = getComputedStyle(document.documentElement).getPropertyValue('--danger').trim();
    var c = v && document.createElement('canvas').getContext('2d');
    if (!c) return undefined;
    c.fillStyle = '#000000'; c.fillStyle = v;
    return /^#[0-9a-f]{6}$/i.test(c.fillStyle) ? c.fillStyle : undefined;
  }
  function typesetOne(el) {
    if (el.dataset.typeset || el.dataset.tex === undefined) return;
    var options = { displayMode: el.dataset.math === 'display', throwOnError: false, trust: false, strict: 'ignore', output: 'htmlAndMathml' };
    var color = errorColor();
    if (color) options.errorColor = color;
    try {
      el.replaceChildren(window.dinahHTML.fragment(window.katex.renderToString(el.dataset.tex, options)));
      el.dataset.typeset = '1';
    } catch (e) { /* The source stays shown. */ }
  }
  function typeset(root) {
    var found = root.querySelectorAll('[data-math]:not([data-typeset])');
    if (!found.length || katexState === 'failed') return;
    if (katexState === 'ready') { found.forEach(typesetOne); return; }
    found.forEach(function (el) { waiting.push(el); });
    loadKatex();
  }

  /* ---- Links to documents ------------------------------------------------
     A message refers to a file or a folder by a Markdown link, a file:
     address, or a path written as text or as inline code. Each opens where
     Dinah opens that kind of file, which Go decides (document_links.go): the
     Editor, the media viewer, Files, or the system's program. A path
     written as text or code becomes a link only when a file or folder is
     there as the message is drawn, so that ordinary words that look like
     paths do not become links that lead nowhere, and a path in a fenced
     block is never one. Go is asked about everything drawn in one batch, a
     moment after the draw, and its answers are kept for ten seconds; when
     they arrive, what was drawn is linked. A link wraps the path's text and
     adds a glyph with no text of its own, so the message's text does not
     change. Every block drawn is linked: a block still streaming in is
     drawn again with each chunk and linked again with it, and a settled
     block is linked once and kept. A path whose file does not exist yet
     stays text until the message is drawn again, as when the chat is
     opened again; nothing polls for it. */
  var LINK_KEPT = 10000, LINK_BATCH = 200, linkAnswers = new Map(), linkWanted = new Map(), linkTimer = 0, linkAsking = 0;
  function linkKey(cwd, ref) { return cwd + '\u0000' + ref; }
  function linkAnswer(cwd, ref) {
    var k = linkKey(cwd, ref), e = linkAnswers.get(k);
    if (!e) return undefined;
    if (Date.now() - e.at > LINK_KEPT) { linkAnswers.delete(k); return undefined; }
    return e.link;
  }
  /* Whether text looks enough like a path to ask about: a name with an
     extension, or something with a slash, with any line suffix (:42,
     :42:7, #L42) set aside, and no address scheme. A path with spaces is
     taken only from inline code, and only when it begins at a drive, a
     slash or a dot. */
  var LINE_SUFFIX = /(?::\d+(?::\d+)?|#L\d+(?:C\d+)?(?:-L?\d+(?:C\d+)?)?)$/;
  var ROOTED = /^(?:[A-Za-z]:[\\/]|\.{1,2}[\\/]|[\\/])/;
  var ABSOLUTE = /^(?:[A-Za-z]:[\\/]|\/[A-Za-z]:[\\/]|\\\\|file:)/i;
  function pathLike(s, spaced) {
    if (!s || s.length > 1024 || /[\n\r\t<>"|?*`]/.test(s) || /^[a-z][a-z0-9+.-]*:\/\//i.test(s) || !/[A-Za-z]/.test(s)) return false;
    if (/\s/.test(s) && !(spaced && ROOTED.test(s))) return false;
    var bare = s.replace(LINE_SUFFIX, '');
    if (/[\\/]/.test(bare)) return !/^[\\/.]+$/.test(bare);
    return /^[^.].*\.[A-Za-z][A-Za-z0-9_-]{0,15}$/.test(bare);
  }
  // A word's punctuation around a path in running text is not the path's.
  var LEADING = /^[(\[{"'“‘]+/, TRAILING = /[)\]}"'”’,.;:!?]+$/;
  /* Gives a link the words and glyph of where it opens. */
  function linkLook(a, l) {
    a.dataset.opens = l.opens || '';
    a.title = l.label || 'Open the File';
    var glyph = a.querySelector(':scope > .link-glyph');
    if (!glyph) {
      glyph = document.createElement('span');
      glyph.className = 'glyph link-glyph'; glyph.setAttribute('aria-hidden', 'true');
      a.insertBefore(glyph, a.firstChild);
    }
    glyph.style.setProperty('--glyph', 'var(--glyph-' + (/^[a-z0-9-]+$/.test(l.glyph || '') ? l.glyph : 'file') + ')');
  }
  function pathLink(ref, l) {
    var a = document.createElement('a');
    a.className = 'smart-chip'; a.href = '#'; a.dataset.chipPath = ref;
    linkLook(a, l);
    return a;
  }
  function want(cwd, ref, root) {
    var k = linkKey(cwd, ref), w = linkWanted.get(k);
    if (!w) { w = { cwd: cwd, ref: ref, roots: [], asked: false }; linkWanted.set(k, w); }
    if (w.roots.indexOf(root) < 0) w.roots.push(root);
    if (!w.asked && !linkTimer) linkTimer = setTimeout(askLinks, 40);
  }
  /* Asks Go about every path wanted since the last time, by the folder they
     are read from, two hundred at a time, and links what was drawn when the
     answers arrive. An answer that does not come leaves the paths as text. */
  function askLinks() {
    linkTimer = 0;
    var byFolder = new Map();
    linkWanted.forEach(function (w) {
      if (w.asked) return;
      w.asked = true;
      if (!byFolder.has(w.cwd)) byFolder.set(w.cwd, []);
      byFolder.get(w.cwd).push(w);
    });
    byFolder.forEach(function (list, cwd) {
      for (var i = 0; i < list.length; i += LINK_BATCH) {
        var part = list.slice(i, i + LINK_BATCH);
        linkAsking++;
        (function (part) {
          Promise.resolve().then(function () { return app().ResolveDocumentLinks(cwd, part.map(function (w) { return w.ref; })); })
            .catch(function () { return []; })
            .then(function (links) {
              var now = Date.now(), roots = [];
              part.forEach(function (w, j) {
                linkAnswers.set(linkKey(cwd, w.ref), { link: (links && links[j]) || { found: false }, at: now });
                linkWanted.delete(linkKey(cwd, w.ref));
                w.roots.forEach(function (r) { if (roots.indexOf(r) < 0) roots.push(r); });
              });
              linkAsking--;
              roots.forEach(function (r) { linkPaths(r, cwd); });
            });
        })(part);
      }
    });
  }
  /* Links the paths in root, a block of a message, that Go has said are
     there, and asks about those it has not been asked about. cwd is the
     folder of the workspace that wrote the message; with none, only an
     absolute path is asked about. */
  function linkPaths(root, cwd) {
    var a0 = app();
    if (!a0 || !a0.ResolveDocumentLinks || root.nodeType !== Node.ELEMENT_NODE || root.closest('pre')) return;
    cwd = cwd || '';
    function known(ref) {
      if (!cwd && !ABSOLUTE.test(ref)) return null;
      var l = linkAnswer(cwd, ref);
      if (l === undefined) { want(cwd, ref, root); return null; }
      return l && l.found ? l : null;
    }
    // A Markdown link or a file: address is a link already; it is given
    // the words and glyph of where it opens.
    root.querySelectorAll('a.smart-chip[data-chip-local]:not([data-opens]), a.smart-chip[data-chip-file]:not([data-opens])').forEach(function (a) {
      var l = known(a.dataset.chipLocal || a.dataset.chipFile);
      if (l) linkLook(a, l);
    });
    // A path in inline code: the code goes inside the link.
    root.querySelectorAll('code').forEach(function (code) {
      if (code.closest('pre, a, .msg-math')) return;
      var ref = code.textContent.trim(), l = pathLike(ref, true) && known(ref);
      if (!l) return;
      var a = pathLink(ref, l);
      code.replaceWith(a);
      a.appendChild(code);
    });
    // A path in running text.
    var walk = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, { acceptNode: function (n) {
      return n.parentElement && n.parentElement.closest('a, pre, code, .msg-math, .mention') ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT;
    } });
    var texts = [];
    while (walk.nextNode()) texts.push(walk.currentNode);
    texts.forEach(function (t) {
      var text = t.nodeValue, re = /\S+/g, m, at = 0, frag = null;
      while ((m = re.exec(text))) {
        var word = m[0], lead = (LEADING.exec(word) || [''])[0], core = word.slice(lead.length);
        core = core.slice(0, core.length - (TRAILING.exec(core) || [''])[0].length);
        if (!pathLike(core, false)) continue;
        var l = known(core);
        if (!l) continue;
        var start = m.index + lead.length;
        frag = frag || document.createDocumentFragment();
        if (start > at) frag.appendChild(document.createTextNode(text.slice(at, start)));
        frag.appendChild(pathLink(core, l)).appendChild(document.createTextNode(core));
        at = start + core.length;
      }
      if (!frag) return;
      if (at < text.length) frag.appendChild(document.createTextNode(text.slice(at)));
      t.parentNode.replaceChild(frag, t);
    });
  }
  /* The number of paths waiting for Go's answer, for the tests. */
  function linksPending() { return linkWanted.size + linkAsking; }

  /* Opens a document a link points to where Go says it opens. cwd is the
     folder a relative path is read from; trigger is the link, which focus
     returns to when the media viewer closes. */
  function openDocument(cwd, ref, trigger) {
    return Promise.resolve().then(function () { return app().ResolveDocumentLink(cwd || '', ref); }).then(function (l) {
      if (!l || !l.found) throw new Error('There is no file or folder at ' + ((l && l.path) || ref) + '.');
      if (l.opens === 'editor') {
        var ed = window.dinahEditor, host = window.dinahHost;
        var opened = l.resource ? ed.openResource(l.resource) :
          Promise.resolve(app().OpenComputerFile(host ? host.workspaceId() || '' : '', l.path)).then(function (v) {
            if (v.origin && v.origin.kind === 'new') toast('"' + v.title + '" is not in a workspace\'s folder, so it opened as a new document. Save As puts it where you choose.', 'warn');
            return ed.open(v);
          });
        return opened.then(function (key) {
          var win = l.line && key && document.querySelector('[data-win="' + CSS.escape(key) + '"] .editor-applet');
          if (win) win.dispatchEvent(new CustomEvent('dinah:editor-line', { detail: { line: l.line } }));
        });
      }
      if (l.opens === 'media') {
        return app().ReadLinkedImage(cwd || '', ref).then(function (src) {
          var img = new Image(), name = l.path.split(/[\\/]/).pop();
          img.src = src; img.alt = name;
          window.dinahMediaViewer.open(img, trigger, name);
        });
      }
      if (l.opens === 'files') return window.dinahResources.openFiles(l.mountPath, l.resource);
      // An extension's view that declares the file's type opens it, in the
      // file's workspace, with the state Go built for it.
      if (l.opens === 'extension') return window.dinahExtensions.openFile(l);
      return app().OpenPath(l.path).then(function (how) {
        if (how === 'shown') toast('That file is a program or a script, so it is shown in Explorer rather than started.');
      });
    });
  }
  /* Opens a link a message shows: a web address in the browser, and a
     document where Dinah opens its kind (openDocument). cwd is the folder
     of the workspace that wrote the message. */
  function openChip(a, cwd) {
    if (a.dataset.chipUrl) { if (window.runtime && window.runtime.BrowserOpenURL) window.runtime.BrowserOpenURL(a.dataset.chipUrl); return; }
    var ref = a.dataset.chipPath || a.dataset.chipLocal || a.dataset.chipFile;
    if (ref) openDocument(cwd, ref, a).catch(fail);
  }
  /* Makes the links in a list of messages open when pressed. cwd(link)
     returns the workspace folder a path in that link is read from. */
  function openLinks(list, cwd) {
    list.addEventListener('click', function (e) {
      var a = e.target.closest && e.target.closest('a.smart-chip');
      if (!a) return;
      e.preventDefault();
      openChip(a, cwd(a));
    });
  }

  /* ---- Diagrams ----------------------------------------------------------
     A fenced d2 or Mermaid block in a message is drawn as its diagram, with
     its source a click away. d2 is drawn by the same engine that draws d2 in
     previews; Mermaid by the Mermaid library bundled with the app, in its
     strict mode, so that a diagram cannot run script. A block is drawn only
     once its closing fence has arrived, and each drawing is kept by its
     source, so that a message streaming in is not drawn again on every
     chunk. */
  function drawDiagrams(el) {
    el.querySelectorAll('pre[data-lang="d2"][data-closed], pre[data-lang="mermaid"][data-closed]').forEach(function (pre) {
      window.dinahDiagrams.draw(pre, pre.dataset.lang, pre.textContent, function () { return el.contains(pre); });
    });
  }

  /* The Markdown as nodes, made safe, with its math drawn once KaTeX is
     there. A window that marks more in its messages, such as a group chat's
     mentions, passes decorate, which is given the nodes before they are
     placed. cwd is the folder of the workspace that wrote the message, from
     which the paths in it are read and linked (linkPaths). The line breaks
     markdown-it writes between blocks are left out, so that each block is
     one node. */
  function nodes(text, decorate, cwd) {
    var box = document.createElement('div');
    box.replaceChildren(window.dinahHTML.fragment(markdown(text)));
    typeset(box);
    if (decorate) decorate(box);
    var frag = document.createDocumentFragment();
    while (box.firstChild) {
      if (box.firstChild.nodeType === Node.TEXT_NODE && !box.firstChild.nodeValue.trim()) box.firstChild.remove();
      else { linkPaths(box.firstChild, cwd); frag.appendChild(box.firstChild); }
    }
    return frag;
  }
  // The folder a message's paths are read from: m.cwd, or what it returns.
  function folderOf(m) { return (typeof m.cwd === 'function' ? m.cwd() : m.cwd) || ''; }

  /* A message streaming in is not drawn again from the start for every
     chunk, which cost time in proportion to its length and lost a selection
     made in it. The text up to its last settled point is drawn once and
     kept; only the block still arriving is drawn again. The kept nodes are
     drawn again from the start when the text no longer begins with what they
     show, when something removed them, or when the paged view of a large
     message replaced them. The settled point is looked for only in the text
     after the kept part, which draws as if it began the text, so the work
     for a chunk does not grow with the message. */
  function drawMarkdown(m, text, paged) {
    if (paged || m.done === undefined || !text.startsWith(m.done) || m.el.childNodes.length < m.kept) {
      m.el.replaceChildren(); m.done = ''; m.kept = 0;
    }
    while (m.el.childNodes.length > m.kept) m.el.lastChild.remove();
    var settled = m.done.length + settledLength(text.slice(m.done.length)), cwd = folderOf(m);
    if (settled > m.done.length) {
      m.el.appendChild(nodes(text.slice(m.done.length, settled), m.decorate, cwd));
      m.done = text.slice(0, settled);
      m.kept = m.el.childNodes.length;
    }
    if (text.length > settled) m.el.appendChild(nodes(text.slice(settled), m.decorate, cwd));
  }

  /* A message is long when its head is out of sight while its end is read:
     more than 1,500 characters or at least twenty lines. Counting the text
     costs no layout, which a message drawn on every chunk cannot afford,
     and the count stops at twenty. */
  function longMessage(text) {
    if (text.length > 1500) return true;
    var lines = 0;
    for (var i = text.indexOf('\n'); i >= 0 && lines < 20; i = text.indexOf('\n', i + 1)) lines++;
    return lines >= 20;
  }
  /* The foot shows once the message is long. The decision is made again on
     every draw from the text alone, so it follows a message as it grows
     however much of the message a draw redraws. */
  function drawFoot(m) {
    if (!m.foot) return;
    var hide = !longMessage(m.text);
    if (m.foot.hidden !== hide) m.foot.hidden = hide;
  }
  /* Draws a message's text, m.text, into m.el, and shows or hides its
     foot. A text larger than bounded-text.js allows is shown as plain text
     in pages. */
  function draw(m) {
    window.dinahBoundedText.draw(m.el, m.text, function (text, paged) { drawMarkdown(m, text, paged); drawDiagrams(m.el); });
    drawFoot(m);
  }

  /* ---- The message's card ------------------------------------------------
     The card has the message's label and its tools in its head, then its
     text, then a foot with the tools again. The person's own message offers
     Edit, which puts it back in the box to change and send again, and any
     other offers Quote, which puts it in the box as a quotation; both offer
     Copy, which copies the message's Markdown as written, read when pressed
     so that a message still streaming is copied as far as it has come.
     o.you says whether the message is the person's, o.label is its label
     as text or as nodes, and o.act(act, m) does Edit or Quote. */
  var TOOL = {
    edit: { label: 'Edit and send again', glyph: 'pencil' },
    quote: { label: 'Quote in the prompt', glyph: 'quote' },
    copy: { label: 'Copy the message', glyph: 'copy' }
  };
  function tool(act) {
    var t = TOOL[act];
    return '<button type="button" class="icon-btn msg-act' + (act === 'copy' ? ' msg-copy' : '') + '"' + (act === 'copy' ? '' : ' data-act="' + act + '"') +
      ' aria-label="' + t.label + '" title="' + t.label + '"><span class="glyph" style="--glyph: var(--glyph-' + t.glyph + ')" aria-hidden="true"></span></button>';
  }
  function card(o) {
    var div = document.createElement('div');
    div.className = 'msg' + (o.you ? ' msg-you' : '');
    var tools = tool(o.you ? 'edit' : 'quote') + tool('copy');
    /* A long message repeats its actions at its end, where the reader
       finishes it. The buttons have no IDs, so the copy at the foot repeats
       none; its group has its own label so that a screen reader tells the
       two groups apart. draw(m) decides whether the foot shows. */
    div.innerHTML = '<div class="msg-head"><span class="msg-label"></span><span class="msg-tools">' + tools + '</span></div><div class="msg-text"></div>' +
      '<div class="msg-foot" hidden><span class="msg-tools" role="group" aria-label="Actions at the end of the message">' + tools + '</span></div>';
    var labelEl = div.querySelector('.msg-label');
    if (typeof o.label === 'string') labelEl.textContent = o.label;
    else if (o.label) labelEl.appendChild(o.label);
    var m = { div: div, text: o.text || '', el: div.querySelector('.msg-text'), foot: div.querySelector('.msg-foot'), label: labelEl.textContent };
    div.querySelectorAll('.msg-copy').forEach(function (b) { b.addEventListener('click', function () { copy(m.text.trim(), 'The message is copied.'); }); });
    div.querySelectorAll('[data-act]').forEach(function (b) {
      b.addEventListener('click', function () { if (o.act) o.act(b.getAttribute('data-act'), m); });
    });
    return m;
  }

  /* ---- Files and pictures sent with a message ---------------------------
     gallery(items) is the row of pictures a sent message shows under its
     text, which the media viewer opens (media-viewer.js, .sent-images);
     each item is { name, src }. A chat draws the pictures the person sent
     in it so (app.js, drawMessageImages), and a group chat with
     attachments(list) below. */
  function gallery(items) {
    var row = document.createElement('div');
    row.className = 'sent-images';
    items.forEach(function (item) {
      var img = document.createElement('img');
      img.src = item.src; img.alt = item.name || 'Attached image'; img.loading = 'lazy';
      row.appendChild(img);
    });
    return row;
  }
  /* A bus message's attachments (bus_views.go, BusAttachment), put under
     its text: the person's pictures as the gallery, and every other file as
     a link that opens it as a file address in a message opens (openChip),
     with its size. A file whose copy the person removed is listed by name
     as removed and opens nothing. An agent's file, which lives in its
     workspace, is listed by its path there. */
  function attachments(list) {
    var box = document.createElement('div');
    box.className = 'msg-attachments';
    var pictures = list.filter(function (a) { return a.picture && a.src && !a.removed; });
    if (pictures.length) box.appendChild(gallery(pictures.map(function (a) { return { name: a.rel, src: a.src }; })));
    var others = list.filter(function (a) { return pictures.indexOf(a) < 0; });
    if (!others.length) return box;
    var files = document.createElement('ul');
    files.className = 'msg-files';
    others.forEach(function (a) {
      var li = document.createElement('li');
      li.className = 'msg-file' + (a.removed ? ' msg-file-removed' : '');
      li.dataset.attachment = String(a.index);
      var glyph = document.createElement('span');
      glyph.className = 'glyph'; glyph.setAttribute('aria-hidden', 'true');
      glyph.style.setProperty('--glyph', a.removed ? 'var(--glyph-slash)' : 'var(--glyph-file)');
      li.appendChild(glyph);
      var size = window.dinahComposer ? window.dinahComposer.sizeWords(a.size) : a.size + ' bytes';
      if (a.uri && !a.removed) {
        var link = document.createElement('a');
        link.className = 'smart-chip'; link.href = '#'; link.dataset.chipFile = a.uri; link.title = 'Open the File';
        link.textContent = a.rel;
        li.appendChild(link);
      } else {
        var name = document.createElement('span');
        name.className = 'msg-file-name'; name.textContent = a.rel;
        li.appendChild(name);
      }
      var meta = document.createElement('span');
      meta.className = 'md-meta';
      meta.textContent = a.removed ? 'Removed' : size;
      li.appendChild(meta);
      files.appendChild(li);
    });
    // Each file's link says where it opens, as a link in the text does.
    linkPaths(files, '');
    box.appendChild(files);
    return box;
  }

  /* Quote puts a message in the box as a quotation, after what is there. */
  function quoted(t) { return t.trim().split('\n').map(function (l) { return '> ' + l; }).join('\n'); }
  function quoteInto(box, text) {
    box.value = (box.value.trim() ? box.value.replace(/\s+$/, '') + '\n\n' : '') + quoted(text) + '\n\n';
    box.focus();
    box.setSelectionRange(box.value.length, box.value.length);
  }

  /* Copying uses Wails' documented clipboard call, and the browser's where
     the page runs outside the app. */
  function copy(text, said) {
    var done = function () { toast(said || 'Copied.', 'positive'); };
    if (window.runtime && window.runtime.ClipboardSetText) {
      window.runtime.ClipboardSetText(text).then(done, function () { toast('The text could not be copied.', 'danger'); });
    } else if (navigator.clipboard) {
      navigator.clipboard.writeText(text).then(done, function () { toast('The text could not be copied.', 'danger'); });
    }
  }

  /* While the reader is scrolled back, a row's size can still settle after
     the place was kept (its content draws or refines late, while the page is
     busy); the watch then holds the same row at its spot until the reader's
     own wheel, touch or key move ends it (Dinah UI, message 1070). */
  var watches = new WeakMap();
  function releasePlace(log) {
    var w = watches.get(log);
    if (w) { w.want = -1; w.id = ''; w.text = ''; w.row = null; }
  }
  function watchPlace(log, row, id, want) {
    var w = watches.get(log);
    if (!w) {
      w = { id: '', text: '', want: -1 };
      watches.set(log, w);
      w.obs = new ResizeObserver(function () {
        var c = watches.get(log);
        if (!c || c.want < 0) return;
        // The row by its message number, or by its first words for a chat's
        // numberless one; trimmed away is gone, and nothing moves.
        var el = c.row && c.row.isConnected && log.contains(c.row) ? c.row : null;
        if (c.id) {
          var byId = log.querySelector('[data-bus-message="' + CSS.escape(c.id) + '"]');
          if (byId && byId.isConnected) el = byId;
        }
        if (!el && c.text) {
          var rows = log.querySelectorAll('.activity-row, [data-bus-message]');
          for (var i = 0; i < rows.length; i++) { if (rows[i].textContent.slice(0, 32) === c.text) { el = rows[i]; break; } }
        }
        if (!el) return;
        var top = log.getBoundingClientRect().top;
        var now = el.getBoundingClientRect().top - top;
        if (Math.abs(now - c.want) > 1.5) log.scrollTop += now - c.want;
      });
      var rowList = log.querySelector('.bus-log-list') || (function () { var r = log.querySelector('.activity-row, [data-bus-message]'); return r ? r.parentElement : null; })();
      if (rowList) w.obs.observe(rowList);
      w.obs.observe(log);
      ['wheel', 'touchstart', 'keydown', 'pointerdown'].forEach(function (name) {
        log.addEventListener(name, function () { releasePlace(log); }, { passive: true });
      });
    }
    w.row = row; w.id = id || ''; w.text = row ? row.textContent.slice(0, 32) : ''; w.want = want;
  }

  /* A scrolled-back reader keeps their place across a change to the log:
     a visible row's spot is held while the change runs, then the row
     stands there again. At the end, the change follows as ever. A row the
     change itself replaces is found again by its message number, so a full
     redraw of a conversation holds the same message, and a reader whose row
     was trimmed away lands where the change left the view, no farther. */
  function keepPlace(log, run) {
    if (!log || !log.clientHeight) return run();
    if (log.scrollHeight - log.scrollTop - log.clientHeight < 24) { releasePlace(log); return run(); }
    var top = log.getBoundingClientRect().top;
    var held = watches.get(log);
    var row = held && held.want >= 0 && held.row && held.row.isConnected && log.contains(held.row) ? held.row : null;
    var retained = !!row, partial = null;
    var rows = log.querySelectorAll('[data-bus-message], .activity-row');
    for (var i = 0; !row && i < rows.length; i++) {
      var r = rows[i].getBoundingClientRect();
      if (!r.height || r.bottom <= top) continue;
      if (!partial) partial = rows[i];
      // A partially visible row can finish laying out above the message the
      // person reads. Prefer the first row whose start is inside the view.
      if (r.top >= top && r.top < top + log.clientHeight) row = rows[i];
    }
    row = row || partial;
    if (!row) return run();
    var id = row.dataset && row.dataset.busMessage ? row.dataset.busMessage : '';
    var before = retained ? held.want : row.getBoundingClientRect().top - top;
    var topAt = log.scrollTop;
    var out = run();
    var again = id ? log.querySelector('[data-bus-message="' + CSS.escape(id) + '"]') : (row.isConnected ? row : null);
    if (again) {
      var move = (again.getBoundingClientRect().top - log.getBoundingClientRect().top) - before;
      if (move) log.scrollTop += move;
      watchPlace(log, again, id, before);
    } else {
      // A chat's rows carry no number: one whose own change trimmed it away,
      // rather than one replaced by the same message under it, is gone for
      // good, and the view comes back to the pixel where it stood.
      log.scrollTop = topAt;
    }
    return out;
  }

  window.dinahChatText = {
  keepPlace: keepPlace,
  releasePlace: releasePlace,
  markdown: markdown, settledLength: settledLength, nodes: nodes, typeset: typeset,
    drawMarkdown: drawMarkdown, drawDiagrams: drawDiagrams, drawFoot: drawFoot, longMessage: longMessage, draw: draw,
    card: card, quoted: quoted, quoteInto: quoteInto, copy: copy, openChip: openChip, openLinks: openLinks,
    openDocument: openDocument, linkPaths: linkPaths, linksPending: linksPending,
    gallery: gallery, attachments: attachments
  };
})();
