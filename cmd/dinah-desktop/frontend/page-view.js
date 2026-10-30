/* Page view: the Editor's document edited as the page it will be. The whole document is one
   ProseMirror editor (vendor/prosemirror-1.42): text is typed, selected,
   pasted and moved through across paragraphs, headings, lists and tables
   as in a word processor. Go reads the draft's text into the
   editor's document and writes it back (pageview); a block the person did
   not change is written back as the text it was read from, byte for byte,
   so an edit changes only the block it was made in. Code, diagrams, images
   and other blocks with no rich form are drawn as the preview draws them,
   and a click on one edits its Markdown in place. What is typed reaches the text, and the draft, a moment
   later, so the text stays the store of the document. */
(function () {
  'use strict';

  var pmReady = null;
  function loadProseMirror() {
    if (window.PM && window.PM.EditorView) return Promise.resolve(window.PM);
    if (!pmReady) {
      pmReady = new Promise(function (resolve, reject) {
        // tables.css is not loaded: it gives tables fixed column widths, and
        // the page draws them as the preview does (editor-applet.css).
        ['prosemirror.css', 'gapcursor.css'].forEach(function (name) {
          var link = document.createElement('link');
          link.rel = 'stylesheet';
          link.href = 'vendor/prosemirror-1.42/' + name;
          document.head.appendChild(link);
        });
        var script = document.createElement('script');
        script.src = 'vendor/prosemirror-1.42/prosemirror.js';
        script.onload = function () { resolve(window.PM); };
        script.onerror = function () { pmReady = null; reject(new Error('The page editor could not be loaded.')); };
        document.head.appendChild(script);
      });
    }
    return pmReady;
  }

  /* The blocks Page view inserts: from the Edit menu's Insert row, from the
     / menu on an empty line, and, in Text and Split, as their Markdown. The
     placeholder is selected after inserting, to be typed over. */
  var PLACEHOLDER = 'Type here.';
  var INSERTS = [
    { id: 'insert-checklist', label: 'Checklist', words: 'checklist task checkbox todo', markdown: '- [ ] ' + PLACEHOLDER },
    { id: 'insert-table', label: 'Table', words: 'table grid rows columns', markdown: '| Heading 1 | Heading 2 | Heading 3 |\n| --- | --- | --- |\n| | | |\n| | | |' },
    { id: 'insert-code', label: 'Code Block', words: 'code block snippet', markdown: '```\ncode\n```' }
  ];
  function insertOf(id) { return INSERTS.find(function (i) { return i.id === id; }); }

  /* The editor's schema. Its names and attributes are pageview's (Go), and
     each node and mark is drawn with the tags the preview draws it with. A
     block read from the text keeps where it came from (uid, gap, src) so Go
     can write it back unchanged. What the page puts on the clipboard is its
     blocks' HTML; where a block came from in the text (uid, src) does not go
     with it, so a pasted copy is a new block, and a block kept as its
     Markdown keeps that Markdown in a data attribute. */
  var place = { uid: { default: null }, gap: { default: null }, src: { default: null } };
  function withPlace(attrs) { return Object.assign({}, place, attrs || {}); }
  var schemaCache = null;
  function schemaOf(PM) {
    if (schemaCache) return schemaCache;
    var table = PM.tableNodes({ tableGroup: 'block', cellContent: 'inline*' });
    table.table.attrs = withPlace(table.table.attrs);
    table.table.toDOM = function () {
      var dom = document.createElement('table'), body = document.createElement('tbody');
      dom.appendChild(body);
      return { dom: dom, contentDOM: body };
    };
    table.table.parseDOM = [{ tag: 'table' }];
    /* A cell holds its text directly, so it is a text block, and the gap
       cursor would otherwise take the place between two cells for one where
       a block could go: the right arrow at the end of a cell drew it there,
       as an extra cell that pushed the row over. allowGapCursor, documented
       in prosemirror-gapcursor's README, keeps it out of a row. */
    table.table_row = Object.assign({}, table.table_row, { allowGapCursor: false });
    var headings = [1, 2, 3, 4, 5, 6].map(function (l) { return { tag: 'h' + l, attrs: { level: l } }; });
    schemaCache = new PM.Schema({
      nodes: {
        doc: { content: 'block+', attrs: { front: { default: '' }, tail: { default: '\n' }, crlf: { default: false } } },
        paragraph: { group: 'block', content: 'inline*', attrs: withPlace(), parseDOM: [{ tag: 'p' }], toDOM: function () { return ['p', 0]; } },
        heading: { group: 'block', content: 'inline*', defining: true, attrs: withPlace({ level: { default: 1 } }), parseDOM: headings,
          toDOM: function (n) { return ['h' + n.attrs.level, 0]; } },
        bullet_list: { group: 'block', content: 'list_item+', attrs: withPlace({ tight: { default: true }, marker: { default: '-' } }), parseDOM: [{ tag: 'ul' }],
          toDOM: function (n) { return ['ul', n.attrs.tight ? { class: 'pm-tight' } : {}, 0]; } },
        ordered_list: { group: 'block', content: 'list_item+', attrs: withPlace({ order: { default: 1 }, tight: { default: true } }),
          parseDOM: [{ tag: 'ol', getAttrs: function (d) { return { order: +(d.getAttribute('start') || 1) }; } }],
          toDOM: function (n) {
            var attrs = n.attrs.tight ? { class: 'pm-tight' } : {};
            if (n.attrs.order !== 1) attrs.start = n.attrs.order;
            return ['ol', attrs, 0];
          } },
        /* A task item (checked true or false; null for a plain item) draws
           its box before its blocks, as the preview does. The box is not
           part of the editable text; clicking it ticks the item
           (taskBoxes). A pasted item keeps a box it had. */
        list_item: { content: 'paragraph (paragraph | bullet_list | ordered_list)*', defining: true, attrs: withPlace({ checked: { default: null } }),
          parseDOM: [{ tag: 'li', contentElement: function (d) { return d.querySelector(':scope > .pm-task-body') || d; },
            getAttrs: function (d) {
              var box = d.querySelector(':scope > input[type="checkbox"], :scope > .pm-check > input[type="checkbox"]');
              return { checked: box ? box.checked : null };
            } }],
          toDOM: function (n) {
            if (n.attrs.checked === null) return ['li', 0];
            var box = { type: 'checkbox', tabindex: '0', 'aria-label': n.attrs.checked ? 'Done' : 'Not done' };
            if (n.attrs.checked) box.checked = '';
            return ['li', { class: 'pm-task' }, ['span', { class: 'pm-check', contenteditable: 'false' }, ['input', box]], ['div', { class: 'pm-task-body' }, 0]];
          } },
        table: table.table,
        table_row: table.table_row,
        table_cell: table.table_cell,
        table_header: table.table_header,
        // A block with no rich form, kept as its Markdown and drawn as the
        // preview draws it.
        source_block: { group: 'block', atom: true, selectable: true,
          attrs: withPlace({ kind: { default: 'text' }, html: { default: '' }, diagrams: { default: null } }),
          parseDOM: [{ tag: 'div.pm-source', getAttrs: function (d) {
            // HTML read from a paste is made safe as it is read (safe-html.js),
            // so that whatever draws the block later has only safe HTML.
            return { src: d.getAttribute('data-src'), kind: d.getAttribute('data-kind') || 'text', html: window.dinahHTML.clean(d.getAttribute('data-html') || '') };
          } }],
          toDOM: function (n) { return ['div', { class: 'pm-source', 'data-src': n.attrs.src || '', 'data-kind': n.attrs.kind, 'data-html': n.attrs.html }]; } },
        text: { group: 'inline' },
        // A line break, as HTML pasted from another program can hold one.
        hard_break: { group: 'inline', inline: true, selectable: false, parseDOM: [{ tag: 'br' }], toDOM: function () { return ['br']; } },
        // A line break inside a paragraph, which the page shows as a space.
        soft_break: { group: 'inline', inline: true, selectable: false, parseDOM: [{ tag: 'span.pm-soft' }], toDOM: function () { return ['span', { class: 'pm-soft' }, ' ']; } }
      },
      // In the order pageview writes them, outermost first.
      marks: {
        link: { attrs: { href: {}, title: { default: null }, auto: { default: false } }, inclusive: false,
          // A pasted link to an address that is not safe is pasted as its
          // text, and one written in the document is drawn without its
          // address (safe-html.js).
          parseDOM: [{ tag: 'a[href]', getAttrs: function (d) { var href = d.getAttribute('href'); return window.dinahHTML.attr('href', href) ? { href: href } : false; } }],
          toDOM: function (m) { return ['a', window.dinahHTML.attr('href', m.attrs.href) ? { href: m.attrs.href } : {}, 0]; } },
        strong: { parseDOM: [{ tag: 'strong' }, { tag: 'b' }], toDOM: function () { return ['strong', 0]; } },
        em: { parseDOM: [{ tag: 'em' }, { tag: 'i' }], toDOM: function () { return ['em', 0]; } },
        underline: { parseDOM: [{ tag: 'u' }], toDOM: function () { return ['u', 0]; } },
        strike: { parseDOM: [{ tag: 's' }, { tag: 'del' }], toDOM: function () { return ['s', 0]; } },
        code: { parseDOM: [{ tag: 'code' }], toDOM: function () { return ['code', 0]; } }
      }
    });
    return schemaCache;
  }

  /* o: {
       root, page, scroll       the Editor window, its page and the page's scroller
       text()                   the draft's text as the Editor has it
       setText(text)            puts the page's text into the Editor's text
       docOf(text)              a promise of the editor's document for text
       markdownOf(doc)          a promise of the doc's text
       sourceOf(src)            a promise of { html, diagrams } for a block's source
       blocksOf(markdown)       a promise of new blocks, as the editor's JSON, to insert
       draw(el, html, diagrams) draws preview HTML into el, diagrams and images included
       keys()                   [{ key, run }], the Editor's key table in ProseMirror's spelling
       say(text, warn)          the Editor's status line
       changed()                the Editor's commands may have changed
       failed(text)             the page could not be built; the Editor leaves Page view
     } */
  function attach(o) {
    var PM = null, view = null, on = false, shown = null, loading = 0;
    var dirty = false, syncTimer = 0, syncing = null, reloadTimer = 0;

    function enable(next) {
      next = !!next;
      if (next === on) return Promise.resolve();
      on = next;
      o.page.classList.toggle('page-view', on);
      if (on) return load();
      clearTimeout(reloadTimer);
      return sync().then(close);
    }
    function close() {
      if (view) { view.destroy(); view = null; }
      shown = null;
      o.changed();
    }

    /* Reads the text into a new editor, keeping the place on the page. */
    function load() {
      var ticket = ++loading, text = o.text(), top = o.scroll.scrollTop;
      return Promise.all([loadProseMirror(), o.docOf(text)]).then(function (both) {
        if (ticket !== loading || !on) return;
        PM = both[0];
        var schema = schemaOf(PM), doc = schema.nodeFromJSON(both[1]);
        var state = PM.EditorState.create({ doc: doc, plugins: plugins(schema) });
        if (view) view.updateState(state);
        else {
          o.page.replaceChildren();
          view = new PM.EditorView(o.page, {
            state: state,
            nodeViews: { source_block: sourceView },
            handlePaste: handlePaste,
            // HTML pasted from another program is made safe before the
            // editor reads it (safe-html.js).
            transformPastedHTML: function (html) { return window.dinahHTML.clean(html); },
            dispatchTransaction: dispatch,
            attributes: { 'aria-label': 'The document, as its page' }
          });
        }
        shown = text;
        dirty = false;
        // The suggestions are placed once the page is known to be the text.
        view.dispatch(view.state.tr.setMeta(suggestKey, true));
        o.scroll.scrollTop = top;
        o.changed();
      }).catch(function (err) {
        if (ticket === loading) o.failed('The page could not be built, so it is drawn from the text until Visual is chosen again: ' + String(err && err.message || err));
      });
    }

    function dispatch(tr) {
      if (!view) return;
      view.updateState(view.state.apply(tr));
      if (tr.docChanged) {
        dirty = true;
        clearTimeout(syncTimer);
        syncTimer = setTimeout(sync, 300);
      }
      o.changed();
    }

    /* ---- The clipboard ---------------------------------------------------
       Copy and Cut put the selected blocks' HTML (from the schema's toDOM,
       with the attributes the HTML cannot say) and their plain text on the
       clipboard, which the editor does itself; HTML pasted is read with the
       schema's parse rules, so what was copied from a page comes back as it
       was, and HTML from Word or a browser keeps its headings, lists,
       tables, links and emphasis. Plain text pasted is read as Markdown, as
       the text of the document is, so a passage copied from the Text view or
       a chat keeps its form. A line with no Markdown in it is pasted as it
       is. */
    var MARKDOWN = /[*_`#>\[\]|{}<\\~]|^\s*([-+]|\d+[.)])\s|\n/;
    function handlePaste(v, event, slice) {
      var data = event && event.clipboardData;
      if (!data || (data.files && data.files.length)) return false;
      var html = data.getData('text/html'), text = data.getData('text/plain');
      if (html) return intoCell(slice);
      if (!text || !MARKDOWN.test(text)) return false;
      pasteMarkdown(text);
      return true;
    }
    /* A table's cell holds a line of text, so blocks pasted into one, which
       the editor would put between two halves of the table, go in as their
       words. Returns whether it pasted. */
    function inCell(state) {
      var $f = state.selection.$from;
      for (var d = $f.depth; d > 0; d--) if ($f.node(d).type.spec.tableRole) return true;
      return false;
    }
    function blocksOnly(fragment) {
      return fragment.childCount > 1 || (fragment.childCount === 1 && !fragment.firstChild.isInline && fragment.firstChild.type.name !== 'paragraph');
    }
    function intoCell(slice) {
      if (!view || !slice || !inCell(view.state) || !blocksOnly(slice.content)) return false;
      var words = [];
      slice.content.forEach(function (nd) {
        var t = nd.isLeaf ? nd.textContent : nd.textBetween(0, nd.content.size, ' ', ' ');
        if (t.trim()) words.push(t.trim());
      });
      var sel = view.state.selection;
      view.dispatch(view.state.tr.insertText(words.join(' '), sel.from, sel.to).scrollIntoView().setMeta('uiEvent', 'paste'));
      return true;
    }
    /* Pastes Markdown at the selection: one paragraph's words go into the
       paragraph the cursor is in, and anything more as blocks. */
    function pasteMarkdown(text) {
      var before = view && view.state.doc;
      return Promise.resolve(o.blocksOf(text.replace(/\r\n/g, '\n'))).then(function (json) {
        if (!view) return;
        var nodes = (json || []).map(function (j) { return view.state.schema.nodeFromJSON(j); });
        if (!nodes.length) return;
        var one = nodes.length === 1 && nodes[0].type.name === 'paragraph';
        var slice = one ? new PM.Slice(PM.Fragment.from(nodes[0]), 1, 1) : new PM.Slice(PM.Fragment.fromArray(nodes), 0, 0);
        if (view.state.doc !== before) o.say('The page changed while the text was being pasted; it is pasted where the cursor is now.');
        if (!intoCell(slice)) view.dispatch(view.state.tr.replaceSelection(slice).scrollIntoView().setMeta('uiEvent', 'paste'));
        view.focus();
      }).catch(function (err) { o.say(String(err && err.message || err), true); });
    }
    /* The Edit menu's Paste: what the clipboard holds, read by Go, pasted as
       a paste with the keyboard would paste it. */
    function pasteContent(content) {
      if (!view) return Promise.resolve();
      view.focus();
      if (content && content.html) { view.pasteHTML(content.html); return Promise.resolve(); }
      var text = content && content.text || '';
      if (!text) return Promise.resolve();
      if (MARKDOWN.test(text)) return pasteMarkdown(text);
      view.pasteText(text);
      return Promise.resolve();
    }
    function selectAll() {
      if (!view) return;
      view.focus();
      view.dispatch(view.state.tr.setSelection(new PM.AllSelection(view.state.doc)));
    }

    /* Puts the page's text into the Editor's text, after the last change
       made on the page. */
    function sync() {
      clearTimeout(syncTimer);
      if (syncing) return syncing.then(sync);
      if (!dirty || !view) return Promise.resolve();
      dirty = false;
      var json = view.state.doc.toJSON();
      syncing = Promise.resolve(o.markdownOf(json)).then(function (text) {
        shown = text;
        o.setText(text);
      }).catch(function (err) {
        dirty = true;
        o.say(String(err && err.message || err), true);
      }).finally(function () { syncing = null; });
      return syncing;
    }

    /* The text changed other than from the page (an agent, a flow, a
       suggestion taken): the page is read again from it. */
    function textChanged() {
      if (!on) return;
      clearTimeout(reloadTimer);
      reloadTimer = setTimeout(function () {
        if (on && !dirty && !syncing && o.text() !== shown) load();
      }, 150);
    }

    /* ---- Suggestions on the page ------------------------------------
       A suggestion is a change to the draft's text at a place in it. The
       page shows it block by block, as the Text view shows it character by
       character: each block it changes is struck through, and the blocks it
       would put there are drawn below them as the preview draws them, with
       Accept and Reject. A block's place in the text is worked out from the
       text it was read from and the text before it, the way the page writes
       itself back (pageview.SerializeDocument), so it holds only while the
       page is the text the suggestions were placed in. */
    var suggested = { text: null, list: [] }, suggestKey = null, rereadFor = null;

    /* Each block's place in the text and on the page, or null when a block
       no longer knows where it came from (it was edited or copied since the
       page was read, so the text it remembers is not the text there). */
    function blockPlaces(doc, text) {
      var out = [], ok = true, cursor = (doc.attrs.front || '').length;
      doc.forEach(function (node, pos) {
        if (!ok) return;
        var src = node.attrs.src, gap = node.attrs.gap;
        if (typeof src !== 'string' || typeof gap !== 'string') { ok = false; return; }
        cursor += gap.length;
        if (text.slice(cursor, cursor + src.length) !== src) { ok = false; return; }
        out.push({ from: cursor, to: cursor + src.length, pos: pos, size: node.nodeSize });
        cursor += src.length;
      });
      return ok ? out : null;
    }

    function suggestionWidget(s, markdown) {
      return function () {
        var box = document.createElement('div');
        box.className = 'pm-sugg';
        box.contentEditable = 'false';
        box.dataset.suggestion = s.id;
        var head = document.createElement('div');
        head.className = 'pm-sugg-head';
        var who = document.createElement('span');
        who.className = 'pm-sugg-who';
        who.textContent = 'Suggested by ' + (s.author || 'an agent') + (s.edited ? ', edited by you' : '') + (s.reason ? ': ' + s.reason : '');
        head.appendChild(who);
        var buttons = [];
        [['Accept', true], ['Reject', false], ['Edit', null]].forEach(function (b) {
          if (b[1] === null && !o.editSuggestion) return;
          var button = document.createElement('button');
          button.type = 'button';
          button.className = 'btn btn-sm' + (b[1] ? ' btn-primary' : '');
          button.textContent = b[0];
          button.setAttribute('aria-label', b[0] + ' this suggestion');
          button.addEventListener('mousedown', function (e) { e.preventDefault(); });
          button.addEventListener('click', function () { if (b[1] === null) edit(); else o.decide(s.id, b[1]); });
          buttons.push(button);
          head.appendChild(button);
        });
        var body = document.createElement('div');
        body.className = 'pm-sugg-new';
        box.append(head, body);
        /* Edit, or a double-click on the suggested block, puts its Markdown in
           a box in the card; Save keeps the new wording in the suggestion. */
        function edit() {
          if (!o.editSuggestion || box.querySelector('.sugg-edit')) return;
          var ed = o.editSuggestion(s, function () { ed.el.remove(); body.hidden = false; buttons.forEach(function (b) { b.hidden = false; }); });
          body.hidden = true;
          buttons.forEach(function (b) { b.hidden = true; });
          box.appendChild(ed.el);
          ed.focus();
        }
        body.addEventListener('dblclick', edit);
        if (!markdown.trim()) body.textContent = 'Removes the struck-through text.';
        else Promise.resolve(o.sourceOf(markdown)).then(function (v) { o.draw(body, v.html, v.diagrams); }).catch(function () { body.textContent = markdown; });
        return box;
      };
    }

    /* A short hash of a text, for a widget's key: the card is drawn again
       when its text changes, an edit of the same length included. */
    function hash(text) {
      var h = 0;
      for (var i = 0; i < text.length; i++) h = (h * 31 + text.charCodeAt(i)) | 0;
      return (h >>> 0).toString(36) + '.' + text.length;
    }
    function suggestionDecorations(doc) {
      var none = PM.DecorationSet.empty;
      if (!suggested.list.length || suggested.text !== shown) return none;
      // The page counts a line end as one character; a text with CRLF line
      // ends counts two, so its places are counted again without them.
      var text = suggested.text, crlf = text.indexOf('\r\n') >= 0;
      var lf = crlf ? text.replace(/\r\n/g, '\n') : text;
      function place(at) { return crlf ? at - (text.slice(0, at).match(/\r\n/g) || []).length : at; }
      var blocks = blockPlaces(doc, lf);
      if (!blocks) {
        // Typing on the page since it was read: the page is read again from
        // the text, once for this text, so its blocks know their places.
        if (rereadFor !== text) { rereadFor = text; setTimeout(function () { if (on && !dirty && !syncing) load(); }, 0); }
        return none;
      }
      var decos = [];
      suggested.list.forEach(function (s) {
        var from = place(s.from), to = place(s.to);
        var overlaps = function (b) { return b.from < to && b.to > from; };
        var hit = blocks.filter(overlaps);
        var start = from, end = to, at;
        if (hit.length) {
          start = Math.min(from, hit[0].from);
          end = Math.max(to, hit[hit.length - 1].to);
          hit.forEach(function (b) { decos.push(PM.Decoration.node(b.pos, b.pos + b.size, { class: 'pm-sugg-old', 'data-suggestion': s.id })); });
          var last = hit[hit.length - 1];
          at = last.pos + last.size;
        } else {
          // An addition between blocks goes after the block before it.
          var before = blocks.filter(function (b) { return b.to <= from; }).pop();
          at = before ? before.pos + before.size : 0;
        }
        var markdown = lf.slice(start, from) + s.after.replace(/\r\n/g, '\n') + lf.slice(to, end);
        decos.push(PM.Decoration.widget(at, suggestionWidget(s, markdown), { key: 'sugg-' + s.id + '-' + hash(markdown) + (s.edited ? '-e' : ''), side: 1, ignoreSelection: true, stopEvent: function () { return true; } }));
      });
      return PM.DecorationSet.create(doc, decos);
    }

    function suggestionPlugin() {
      suggestKey = new PM.PluginKey('suggestions');
      return new PM.Plugin({
        key: suggestKey,
        state: {
          init: function (config, state) { return suggestionDecorations(state.doc); },
          apply: function (tr, set) {
            if (tr.getMeta(suggestKey)) return suggestionDecorations(tr.doc);
            return tr.docChanged ? set.map(tr.mapping, tr.doc) : set;
          }
        },
        props: { decorations: function (state) { return suggestKey.getState(state); } }
      });
    }

    /* Find on the page: the matches of the Editor's find bar, marked on the
       page, the current one set apart; a match is found in each block's
       text, so a word split across marks ("**bo**ld") is found, and a word
       across two blocks is not. */
    var findKey = null, found = { spans: [], current: -1 };
    function findDecorations(doc) {
      if (!found.spans.length) return PM.DecorationSet.empty;
      return PM.DecorationSet.create(doc, found.spans.map(function (s, i) {
        return PM.Decoration.inline(s.from, s.to, { class: 'pm-find-match' + (i === found.current ? ' pm-find-current' : '') });
      }));
    }
    function findPlugin() {
      findKey = new PM.PluginKey('find');
      return new PM.Plugin({
        key: findKey,
        state: {
          init: function (config, state) { return findDecorations(state.doc); },
          apply: function (tr, set) {
            if (tr.getMeta(findKey)) return findDecorations(tr.doc);
            return tr.docChanged ? set.map(tr.mapping, tr.doc) : set;
          }
        },
        props: { decorations: function (state) { return findKey.getState(state); } }
      });
    }
    /* Each block's text with, for each of its characters, the page position
       it is at; a leaf that is not text, such as an image, is one character. */
    function blockTexts(doc) {
      var out = [];
      doc.descendants(function (node, pos) {
        if (!node.isTextblock) return true;
        var s = '', map = [];
        node.forEach(function (child, offset) {
          var at = pos + 1 + offset;
          if (child.isText) { for (var i = 0; i < child.text.length; i++) map.push(at + i); s += child.text; }
          else { map.push(at); s += '￼'; }
        });
        out.push({ text: s, map: map });
        return false;
      });
      return out;
    }
    function redrawFind() { if (view && findKey) view.dispatch(view.state.tr.setMeta(findKey, true)); }
    /* find takes a matcher, text to [{ from, to }] in that text, and keeps
       the matches as page spans; null clears them. It returns how many, and
       the index of the first match at or after the selection. */
    function find(matcher) {
      found = { spans: [], current: -1 };
      if (!view || !matcher) { redrawFind(); return { count: 0, after: -1 }; }
      blockTexts(view.state.doc).forEach(function (b) {
        (matcher(b.text) || []).forEach(function (m) {
          if (m.to > m.from) found.spans.push({ from: b.map[m.from], to: b.map[m.to - 1] + 1, text: b.text.slice(m.from, m.to) });
        });
      });
      var head = view.state.selection.from, after = found.spans.findIndex(function (s) { return s.from >= head; });
      redrawFind();
      return { count: found.spans.length, after: after };
    }
    function goToMatch(i) {
      if (!view || i < 0 || i >= found.spans.length) return;
      found.current = i;
      var s = found.spans[i];
      view.dispatch(view.state.tr.setSelection(PM.TextSelection.create(view.state.doc, s.from, s.to)).scrollIntoView().setMeta(findKey, true));
    }
    function matchText(i) { return found.spans[i] ? found.spans[i].text : ''; }
    function replaceMatch(i, text) {
      if (!view || !found.spans[i]) return;
      var s = found.spans[i];
      view.dispatch(view.state.tr.insertText(text, s.from, s.to));
    }
    /* Replaces every match, last first so the earlier positions hold; each
       replacement is given the match's text. */
    function replaceAllMatches(replacementOf) {
      if (!view || !found.spans.length) return 0;
      var tr = view.state.tr, n = 0;
      found.spans.slice().reverse().forEach(function (s) { tr.insertText(replacementOf(s.text), s.from, s.to); n++; });
      view.dispatch(tr);
      return n;
    }
    function selectionText() { return view ? view.state.doc.textBetween(view.state.selection.from, view.state.selection.to, ' ') : ''; }

    /* The draft's suggestions, as the Editor has them: its text, and each
       placed suggestion with where it sits in that text. */
    function suggest(text, list) {
      suggested = { text: text, list: list || [] };
      if (view && suggestKey) view.dispatch(view.state.tr.setMeta(suggestKey, true));
    }

    function plugins(schema) {
      var n = schema.nodes;
      var keys = PM.keymap(Object.fromEntries((o.keys() || []).map(function (k) { return [k.key, function () { return k.run(); }]; })));
      function sink(state, d) { return PM.sinkListItem(n.list_item)(state, d); }
      function lift(state, d) { return PM.liftListItem(n.list_item)(state, d); }
      // Tab in a table's last cell adds a row and moves into it.
      function addRowAtEnd(state, d, v) {
        if (!PM.isInTable(state) || !PM.addRowAfter(state, d)) return false;
        if (d && v) PM.goToNextCell(1)(v.state, v.dispatch);
        return true;
      }
      // Enter in a task item starts another task item, not yet done.
      function splitItem(state, d) {
        var $c = state.selection.$from, task = false;
        for (var depth = $c.depth; depth > 0; depth--) {
          if ($c.node(depth).type === n.list_item) { task = $c.node(depth).attrs.checked !== null; break; }
        }
        return PM.splitListItem(n.list_item, task ? { checked: false } : undefined)(state, d);
      }
      return [
        taskBoxes(schema),
        suggestionPlugin(),
        findPlugin(),
        slashMenu(schema),
        emptyLineHint(schema),
        keys,
        PM.keymap({
          'Tab': PM.chainCommands(PM.goToNextCell(1), addRowAtEnd, sink),
          'Shift-Tab': PM.chainCommands(PM.goToNextCell(-1), lift),
          'Enter': splitItem,
          'Mod-z': PM.undo, 'Mod-y': PM.redo, 'Mod-Shift-z': PM.redo
        }),
        PM.keymap(PM.baseKeymap),
        PM.history(), PM.dropCursor(), PM.gapCursor(), PM.tableEditing(),
        // A block copied or split from another takes none of its place, so
        // only the original is written back as the text it came from.
        new PM.Plugin({
          appendTransaction: function (trs, old, state) {
            if (!trs.some(function (t) { return t.docChanged; })) return null;
            var seen = new Set(), tr = null;
            state.doc.descendants(function (node, pos) {
              var uid = node.attrs && node.attrs.uid;
              if (uid === null || uid === undefined) return;
              if (!seen.has(uid)) { seen.add(uid); return; }
              tr = tr || state.tr;
              var attrs = Object.assign({}, node.attrs, { uid: null, gap: null });
              if (node.type !== n.source_block) attrs.src = null;
              tr.setNodeMarkup(pos, null, attrs);
            });
            return tr;
          }
        })
      ];
    }

    /* Clicking a task item's box ticks or unticks the item, as one change
       to the document that Undo takes back. The box's own click is
       prevented, so the box shows only what the document says. */
    function taskBoxes(schema) {
      function item(view, box) {
        var li = box.closest('li.pm-task');
        if (!li || !view.dom.contains(li)) return null;
        var $p = view.state.doc.resolve(view.posAtDOM(li, 0));
        for (var depth = $p.depth; depth > 0; depth--) {
          if ($p.node(depth).type === schema.nodes.list_item) return { pos: $p.before(depth), node: $p.node(depth) };
        }
        return null;
      }
      return new PM.Plugin({ props: { handleDOMEvents: {
        keydown: function (view, e) {
          if (e.key !== ' ' || !e.target.matches || !e.target.matches('.pm-check input')) return false;
          e.preventDefault();
          var found = item(view, e.target);
          if (found && view.editable) view.dispatch(view.state.tr.setNodeMarkup(found.pos, null, Object.assign({}, found.node.attrs, { checked: !found.node.attrs.checked })));
          return true;
        },
        mousedown: function (view, e) {
          if (!e.target.matches || !e.target.matches('.pm-check input')) return false;
          e.preventDefault();
          var found = item(view, e.target);
          if (!found || !view.editable) return true;
          view.dispatch(view.state.tr.setNodeMarkup(found.pos, null, Object.assign({}, found.node.attrs, { checked: !found.node.attrs.checked })));
          return true;
        },
        click: function (view, e) {
          if (!e.target.matches || !e.target.matches('.pm-check input')) return false;
          e.preventDefault();
          return true;
        }
      } } });
    }

    /* Whether a line can take a block of its own: an empty paragraph (or
       one holding only a / command) directly in the page. */
    function blockLine(state, schema) {
      var sel = state.selection;
      if (!sel.empty) return null;
      var $c = sel.$from, n = schema.nodes;
      if ($c.parent.type !== n.paragraph || $c.depth !== 1) return null;
      return $c;
    }

    /* The / menu: typed at the start of an empty line, it lists the blocks
       that can be inserted, narrowed by what follows the /; the arrow keys
       and Enter choose one, and Escape closes it. */
    function slashMenu(schema) {
      var menu = null, shown = [], active = 0, closed = '';
      /* PUDL places the menu under its anchor (data-menu-anchor) and keeps
         it there as the page scrolls; the anchor is a mark at the cursor. */
      var anchorId = 'pm-slash-anchor-' + Math.random().toString(36).slice(2);
      function close() { if (menu) { menu.remove(); menu = null; } shown = []; }
      function query(state) {
        var $c = blockLine(state, schema);
        if (!$c) return null;
        var text = $c.parent.textContent;
        if (!/^\/[\w ]*$/.test(text) || $c.parentOffset !== text.length) return null;
        return { q: text.slice(1).trim().toLowerCase(), from: $c.start(), to: $c.end(), text: text };
      }
      function paint() {
        Array.from(menu.children).forEach(function (b, i) { b.setAttribute('aria-selected', String(i === active)); });
        var b = menu.children[active];
        if (b) b.scrollIntoView({ block: 'nearest' });
      }
      function choose(v, item) {
        var q = query(v.state);
        close();
        if (q) v.dispatch(v.state.tr.delete(q.from, q.to));
        run(item.id);
      }
      function draw(v, q) {
        shown = INSERTS.filter(function (i) { return !q.q || i.label.toLowerCase().indexOf(q.q) >= 0 || i.words.indexOf(q.q) >= 0; });
        if (!shown.length) { close(); return; }
        if (!menu) {
          menu = document.createElement('div');
          menu.className = 'menu-panel page-slash';
          menu.setAttribute('popover', 'manual');
          menu.setAttribute('role', 'listbox');
          menu.setAttribute('aria-label', 'Insert');
          menu.setAttribute('data-menu-anchor', anchorId);
          document.body.appendChild(menu);
          menu.showPopover();
          active = 0;
        }
        menu.replaceChildren.apply(menu, shown.map(function (item) {
          var b = document.createElement('button');
          b.type = 'button';
          b.className = 'menu-action';
          b.setAttribute('role', 'option');
          b.textContent = item.label;
          // The editor keeps focus, so typing goes on narrowing the list.
          b.addEventListener('mousedown', function (e) { e.preventDefault(); choose(v, item); });
          return b;
        }));
        active = Math.min(active, shown.length - 1);
        paint();
      }
      return new PM.Plugin({
        view: function () {
          return {
            update: function (v) {
              var q = query(v.state);
              // Escape closes the menu until the line changes.
              if (!q || q.text === closed) { if (!q) closed = ''; close(); return; }
              draw(v, q);
            },
            destroy: close
          };
        },
        props: {
          decorations: function (state) {
            var q = query(state);
            if (!q || q.text === closed) return null;
            return PM.DecorationSet.create(state.doc, [PM.Decoration.widget(q.to, function () {
              var mark = document.createElement('span');
              mark.className = 'pm-slash-anchor';
              mark.id = anchorId;
              return mark;
            }, { side: 1, key: anchorId })]);
          },
          handleKeyDown: function (v, e) {
            if (!menu) return false;
            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
              active = (active + (e.key === 'ArrowDown' ? 1 : shown.length - 1)) % shown.length;
              paint();
              return true;
            }
            if (e.key === 'Enter' || e.key === 'Tab') { choose(v, shown[active]); return true; }
            if (e.key === 'Escape') { var q = query(v.state); closed = q ? q.text : ''; close(); return true; }
            return false;
          }
        }
      });
    }

    /* An empty line with the cursor in it says how to insert a block. */
    function emptyLineHint(schema) {
      return new PM.Plugin({
        props: {
          decorations: function (state) {
            var $c = blockLine(state, schema);
            if (!$c || $c.parent.content.size) return null;
            var at = $c.before();
            return PM.DecorationSet.create(state.doc, [PM.Decoration.node(at, at + $c.parent.nodeSize, { class: 'pm-hint', 'data-hint': 'Type / to insert a checklist, table or code block' })]);
          }
        }
      });
    }

    /* Where a new block goes: in place of the empty line the cursor is on,
       or after the top-level block it is in. */
    function insertPlace(state) {
      var sel = state.selection;
      if (sel instanceof PM.NodeSelection) return { from: sel.to, to: sel.to };
      var $f = sel.$from, d = 1;
      if ($f.depth === 0) return { from: sel.from, to: sel.from };
      var block = $f.node(d);
      if (block.isTextblock && block.content.size === 0) return { from: $f.before(d), to: $f.after(d) };
      return { from: $f.after(d), to: $f.after(d) };
    }

    /* Inserts blocks and puts the cursor in them: on the placeholder, in
       the first cell, or, for a code block, in its Markdown; with drawn, a
       block drawn as the preview draws it, such as an image, stays drawn. */
    function insertBlocks(nodes, placeholder, drawn) {
      var v = view, hole = placeholder || PLACEHOLDER;
      if (!v || !nodes.length) return;
      var place = insertPlace(v.state), tr = v.state.tr.replaceWith(place.from, place.to, nodes);
      var end = place.from + nodes.reduce(function (s, nd) { return s + nd.nodeSize; }, 0), sel = null, cell = null;
      // The placeholder anywhere in the new blocks is selected to be typed
      // over; failing that, the cursor goes into the first cell of a table.
      tr.doc.nodesBetween(place.from, end, function (node, pos) {
        if (sel) return false;
        if (node.isText && node.text.indexOf(hole) >= 0) {
          var at = pos + node.text.indexOf(hole);
          sel = PM.TextSelection.create(tr.doc, at, at + hole.length);
        } else if (cell === null && (node.type.spec.tableRole === 'cell' || node.type.spec.tableRole === 'header_cell')) {
          cell = pos + 1;
        }
      });
      if (!sel && cell !== null) sel = PM.TextSelection.create(tr.doc, cell);
      if (sel) tr.setSelection(sel);
      v.dispatch(tr.scrollIntoView());
      v.focus();
      if (!sel && !drawn && nodes[0].type.name === 'source_block') {
        var dom = v.nodeDOM(place.from);
        if (dom && dom.pmEdit) dom.pmEdit();
      }
    }
    function insert(id) {
      var item = insertOf(id), s = view.state.schema, n = s.nodes;
      if (id === 'insert-checklist') {
        var task = n.list_item.create({ checked: false }, n.paragraph.create(null, s.text(PLACEHOLDER)));
        insertBlocks([n.bullet_list.create(null, task)]);
        return;
      }
      if (id === 'insert-table') {
        /* The heading row has text, since an empty one is a table without
           headings and is not drawn; the first heading is selected to be
           typed over. */
        var head = n.table_row.create(null, [1, 2, 3].map(function (i) { return n.table_header.create(null, s.text('Heading ' + i)); }));
        var row = function () { return n.table_row.create(null, [0, 1, 2].map(function () { return n.table_cell.createAndFill(); })); };
        insertBlocks([n.table.create(null, [head, row(), row()])], 'Heading 1');
        return;
      }
      Promise.resolve(o.blocksOf(item.markdown)).then(function (json) {
        if (!view) return;
        insertBlocks((json || []).map(function (j) { return view.state.schema.nodeFromJSON(j); }));
      }).catch(function (err) { o.say(String(err && err.message || err), true); });
    }
    /* Inserts Markdown as blocks where a new block goes, such as an image
       or a link to a file added to the draft. */
    function insertMarkdown(markdown) {
      return Promise.resolve(o.blocksOf(markdown)).then(function (json) {
        if (!view) return;
        insertBlocks((json || []).map(function (j) { return view.state.schema.nodeFromJSON(j); }), null, true);
      });
    }

    /* A block drawn as the preview draws it; a click edits its source in
       place, and leaving the box draws it again. */
    function sourceView(node, v, getPos) {
      var dom = document.createElement('div');
      dom.className = 'pm-source';
      dom.contentEditable = 'false';
      var shownNode = null, box = null;
      function draw(nd) {
        shownNode = nd;
        dom.dataset.kind = nd.attrs.kind;
        var inner = document.createElement('div');
        dom.replaceChildren(inner);
        o.draw(inner, nd.attrs.html || '<pre>' + (nd.attrs.src || '').replace(/&/g, '&amp;').replace(/</g, '&lt;') + '</pre>', nd.attrs.diagrams || []);
      }
      function edit() {
        if (box) return;
        box = document.createElement('textarea');
        box.className = 'form-textarea page-source';
        box.spellcheck = false;
        box.value = shownNode.attrs.src || '';
        box.setAttribute('aria-label', 'The Markdown of this block');
        dom.replaceChildren(box);
        function fit() { box.style.height = 'auto'; box.style.height = box.scrollHeight + 2 + 'px'; }
        box.addEventListener('input', fit);
        box.addEventListener('keydown', function (e) {
          if (e.key === 'Escape' || (e.key === 'Enter' && (e.ctrlKey || e.metaKey))) { e.preventDefault(); done(); v.focus(); }
        });
        box.addEventListener('blur', done);
        fit();
        box.focus();
        o.say('This block is edited as its Markdown. Escape or Ctrl+Enter puts it back on the page.');
      }
      function done() {
        if (!box) return;
        var value = box.value, was = shownNode.attrs.src || '';
        box = null;
        if (value === was) { draw(shownNode); return; }
        var pos = getPos();
        if (typeof pos !== 'number') return;
        v.dispatch(v.state.tr.setNodeMarkup(pos, null, Object.assign({}, shownNode.attrs, { src: value, html: '', diagrams: null })));
        Promise.resolve(o.sourceOf(value)).then(function (r) {
          var at = getPos();
          if (!view || typeof at !== 'number') return;
          var now = view.state.doc.nodeAt(at);
          if (!now || now.attrs.src !== value) return;
          view.dispatch(view.state.tr.setNodeMarkup(at, null, Object.assign({}, now.attrs, { html: r.html, diagrams: r.diagrams }))
            .setMeta('addToHistory', false));
        }).catch(function (err) { o.say(String(err && err.message || err), true); });
      }
      dom.addEventListener('mousedown', function (e) {
        if (box || e.button !== 0 || e.target.closest('a[href]') && e.ctrlKey) return;
        e.preventDefault();
        edit();
      });
      draw(node);
      // Inserting a code block opens its Markdown at once, the code selected.
      dom.pmEdit = function () {
        edit();
        if (!box) return;
        var v = box.value, a = v.indexOf('\n') + 1, b = v.lastIndexOf('\n');
        if (a > 0 && b > a) box.setSelectionRange(a, b);
      };
      return {
        dom: dom,
        update: function (nd) {
          if (nd.type !== node.type) return false;
          if (!box && (nd.attrs.html !== shownNode.attrs.html || nd.attrs.src !== shownNode.attrs.src)) draw(nd);
          else shownNode = nd;
          return true;
        },
        stopEvent: function (e) { return !!box && box.contains(e.target); },
        ignoreMutation: function () { return true; },
        selectNode: function () { dom.classList.add('selected'); },
        deselectNode: function () { dom.classList.remove('selected'); }
      };
    }

    /* A table's shape: rows and columns are added and deleted. Merged
       cells and heading cells outside the first row are what a Markdown
       table cannot hold, so the page does not offer them. */
    var TABLE_COMMANDS = {
      'row-above': function () { return PM.addRowBefore; },
      'row-below': function () { return PM.addRowAfter; },
      'column-before': function () { return PM.addColumnBefore; },
      'column-after': function () { return PM.addColumnAfter; },
      'delete-row': function () { return PM.deleteRow; },
      'delete-column': function () { return PM.deleteColumn; },
      'delete-table': function () { return PM.deleteTable; }
    };
    function can(name) {
      if (!view) return false;
      var make = TABLE_COMMANDS[name];
      return !!make && PM.isInTable(view.state) && make()(view.state);
    }

    /* The Editor's commands, on the page. Each returns true when it acted. */
    function run(name) {
      if (!view) return false;
      var v = view, s = v.state.schema, n = s.nodes, cmd = null;
      function list(type) {
        return function (state, d) {
          var $from = state.selection.$from;
          for (var depth = $from.depth; depth > 0; depth--) {
            var node = $from.node(depth);
            if (node.type === type) return PM.liftListItem(n.list_item)(state, d);
            if (node.type === n.bullet_list || node.type === n.ordered_list) return false;
          }
          return PM.wrapInList(type)(state, d);
        };
      }
      switch (name) {
        case 'undo': cmd = PM.undo; break;
        case 'redo': cmd = PM.redo; break;
        case 'bold': cmd = PM.toggleMark(s.marks.strong); break;
        case 'italic': cmd = PM.toggleMark(s.marks.em); break;
        case 'code': cmd = PM.toggleMark(s.marks.code); break;
        case 'underline': cmd = PM.toggleMark(s.marks.underline); break;
        case 'strike': cmd = PM.toggleMark(s.marks.strike); break;
        case 'h1': case 'h2': case 'h3': case 'h4': cmd = PM.setBlockType(n.heading, { level: +name.charAt(1) }); break;
        case 'paragraph': cmd = PM.setBlockType(n.paragraph); break;
        case 'bullets': cmd = list(n.bullet_list); break;
        case 'numbers': cmd = list(n.ordered_list); break;
        case 'link': linkDialog(); return true;
        default:
          if (TABLE_COMMANDS[name]) { cmd = TABLE_COMMANDS[name](); break; }
          if (!insertOf(name)) return false;
          insert(name);
          return true;
      }
      cmd(v.state, v.dispatch, v);
      v.focus();
      return true;
    }
    function linkDialog() {
      var v = view, s = v.state, mark = s.schema.marks.link, current = '';
      s.doc.nodesBetween(s.selection.from, Math.max(s.selection.to, s.selection.from + 1), function (node) {
        var m = mark.isInSet(node.marks);
        if (m) current = m.attrs.href;
      });
      var dialog = document.createElement('dialog');
      dialog.className = 'dialog editor-dialog';
      dialog.innerHTML = '<h2 class="dialog-title">Link</h2><div class="dialog-body">' +
        '<div class="form-group"><label class="form-label">Address<input class="form-input" placeholder="https://"></label></div>' +
        '<p class="form-help">Leave the address empty to take the link away.</p></div>' +
        '<div class="dialog-actions"><button type="button" class="btn" data-no>Cancel</button><button type="button" class="btn btn-primary" data-yes>Set the Link</button></div>';
      var url = dialog.querySelector('input');
      url.value = current;
      o.root.appendChild(dialog);
      function done() { dialog.close(); }
      dialog.addEventListener('close', function () { dialog.remove(); if (view === v) v.focus(); });
      dialog.querySelector('[data-no]').addEventListener('click', done);
      function set() {
        var address = url.value.trim(), st = v.state, from = st.selection.from, to = st.selection.to;
        done();
        if (view !== v) return;
        var tr = st.tr;
        if (from === to) {
          if (!address) return;
          tr.insertText(address, from).addMark(from, from + address.length, mark.create({ href: address }));
        } else {
          tr.removeMark(from, to, mark);
          if (address) tr.addMark(from, to, mark.create({ href: address }));
        }
        v.dispatch(tr);
      }
      dialog.querySelector('[data-yes]').addEventListener('click', set);
      url.addEventListener('keydown', function (ev) { if (ev.key === 'Enter' && !ev.isComposing) { ev.preventDefault(); set(); } });
      dialog.showModal();
      url.focus();
    }

    // A link on the page is followed with Ctrl+click; a plain click places the caret.
    function click(e) {
      if (!on) return;
      var link = e.target.closest && e.target.closest('a[href]');
      if (link && !e.ctrlKey) { e.preventDefault(); e.stopPropagation(); }
    }
    o.scroll.addEventListener('click', click, true);

    return {
      enable: enable,
      active: function () { return on && !!view; },
      // Changes on the page not yet in the Editor's text.
      pending: function () { return dirty || !!syncing; },
      sync: sync,
      textChanged: textChanged,
      // The draft's suggestions, to show on the page with Accept and Reject.
      suggest: suggest,
      // The Edit menu's Paste and Select All, on the page.
      paste: pasteContent, selectAll: selectAll,
      focus: function () { if (view) view.focus(); },
      // The page's editable element, for the Edit menu's Cut and Copy.
      element: function () { return view ? view.dom : null; },
      // The Editor's find bar, on the page.
      find: find, goToMatch: goToMatch, matchText: matchText, replaceMatch: replaceMatch, replaceAllMatches: replaceAllMatches, selectionText: selectionText,
      run: run,
      insertMarkdown: insertMarkdown,
      /* The page's blocks the selection touches, as a document of their
         own for markdownOf, or null with nothing selected: what a chat
         takes as the passage, widened to whole blocks as
         the text widens a selection to whole lines. */
      selectedBlocks: function () {
        if (!view || view.state.selection.empty) return null;
        var doc = view.state.doc, sel = view.state.selection, picked = [];
        doc.forEach(function (node, offset) {
          if (offset < sel.to && offset + node.nodeSize > sel.from) picked.push(node.toJSON());
        });
        if (!picked.length) return null;
        return { type: 'doc', attrs: Object.assign({}, doc.attrs, { front: '', tail: '\n' }), content: picked };
      },
      inTable: function () { return !!view && PM.isInTable(view.state); },
      // Whether a table command can act where the cursor is.
      can: can,
      // What the cursor is in, for the toolbar's pressed states.
      marks: function () {
        if (!view) return {};
        var st = view.state, s = st.schema, out = {}, $f = st.selection.$from;
        ['strong', 'em', 'code', 'underline', 'strike', 'link'].forEach(function (m) {
          var type = s.marks[m];
          out[m] = st.selection.empty ? !!type.isInSet(st.storedMarks || $f.marks()) : st.doc.rangeHasMark(st.selection.from, st.selection.to, type);
        });
        var block = $f.parent, list = null;
        for (var d = $f.depth; d > 0; d--) { var t = $f.node(d).type.name; if (t === 'bullet_list' || t === 'ordered_list') { list = t; break; } }
        out.block = block.type.name === 'heading' ? 'h' + block.attrs.level : block.type.name === 'paragraph' ? 'paragraph' : block.type.name;
        out.list = list;
        out.table = PM.isInTable(st);
        return out;
      },
      focus: function () { if (view) view.focus(); },
      destroy: function () {
        on = false;
        loading++;
        clearTimeout(syncTimer);
        clearTimeout(reloadTimer);
        o.scroll.removeEventListener('click', click, true);
        if (view) { view.destroy(); view = null; }
      }
    };
  }

  window.dinahPageView = { attach: attach, load: loadProseMirror, insert: insertOf, inserts: INSERTS, placeholder: PLACEHOLDER };
})();
