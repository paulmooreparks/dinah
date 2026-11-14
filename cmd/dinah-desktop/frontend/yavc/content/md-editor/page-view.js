/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Lifted from Dinah Desktop (cmd/dinah-desktop/frontend/page-view.js, Apache-2.0) for yavc.app: the
   Markdown engine is yavc-md.wasm in the browser, through the Editor (editor.js). */
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

  /* The editor's files sit beside this one. */
  var BASE = ((document.currentScript && document.currentScript.src) || '').split('?')[0].replace(/[^/]*$/, '');
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
          link.href = BASE + 'vendor/prosemirror-1.42/' + name;
          document.head.appendChild(link);
        });
        window.yavcUtil.loadScript(BASE + 'vendor/prosemirror-1.42/prosemirror.js?v=2').then(function () { resolve(window.PM); },
          function () { pmReady = null; reject(new Error('The page editor could not be loaded.')); });
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
  var place = { uid: { default: null }, gap: { default: null }, src: { default: null }, canonical: { default: null } };
  function withPlace(attrs) { return Object.assign({}, place, attrs || {}); }
  function jsonAttr(d, name) {
    var v = d.getAttribute(name);
    if (!v || v.length > 262144) return null;
    try { return JSON.parse(v); } catch (err) { return null; }
  }
  /* The attributes of an element of a table read from HTML, as they may be
     drawn on the page: each as safe-html allows it (no handlers, no
     addresses but the web's, a style with only the properties it keeps),
     and none that would reach the editor's own workings. */
  function safeAttrs(list) {
    if (!Array.isArray(list) || !list.length) return [];
    var el = document.createElement('span');
    list.forEach(function (a) {
      if (!Array.isArray(a) || a.length !== 2 || typeof a[0] !== 'string' || typeof a[1] !== 'string') return;
      var name = a[0].toLowerCase();
      if (!/^[a-z][a-z0-9:_-]{0,63}$/.test(name) || /^(contenteditable|id|tabindex|draggable|hidden|role|slot|is|dir|lang|translate|inert|popover)$/.test(name) || /^(data-pm-|aria-)/.test(name) || a[1].length > 4096) return;
      if (name === 'class' && /(^|\s)(pm-|ProseMirror)/.test(a[1])) return;
      try { el.setAttribute(name, a[1]); } catch (err) { }
    });
    var cleaned = window.yavcHTML.fragment(el.outerHTML).firstElementChild, out = [];
    if (cleaned) Array.prototype.forEach.call(cleaned.attributes, function (attr) { out.push([attr.name, attr.value]); });
    return out;
  }
  function attrPairs(list) {
    if (!Array.isArray(list)) return [];
    return list.filter(function (a) { return Array.isArray(a) && a.length === 2 && typeof a[0] === 'string' && typeof a[1] === 'string' && /^[a-z][a-z0-9:_-]{0,63}$/i.test(a[0]) && a[1].length <= 4096; }).slice(0, 64);
  }
  /* What a table read from HTML keeps of its markup, read back from the
     page's own DOM (a copy, a paste from another window of the site). Its
     column group is not taken from a paste: the engine writes it as it
     is, so only one read from the text is trusted. */
  function rawTableAttrs(d) {
    var raw = jsonAttr(d, 'data-pm-raw');
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return null;
    return { open: typeof raw.open === 'string' && raw.open.length <= 1024 ? raw.open : '', close: typeof raw.close === 'string' && raw.close.length <= 1024 ? raw.close : '', attrs: attrPairs(raw.attrs), colgroup: '', thead: raw.thead === true, tbody: raw.tbody === true };
  }
      /* A colour as the engine writes it in a style: the site's names as
       its tokens, anything else as given, kept to what a colour can be. */
    function colorCSS(v) {
      v = String(v || '').trim();
      var named = { ink: 'var(--deck-ink, var(--dinah-ink, var(--text)))', text: 'var(--deck-ink, var(--dinah-ink, var(--text)))', muted: 'var(--deck-muted, var(--dinah-muted-ink, var(--text-muted)))', accent: 'var(--deck-accent, var(--accent))', danger: 'var(--danger)', warn: 'var(--warn)', warning: 'var(--warn)', positive: 'var(--positive)', ok: 'var(--positive)' };
      if (named[v.toLowerCase()]) return named[v.toLowerCase()];
      return /^(#[0-9a-fA-F]{3,8}|[a-zA-Z]{3,24}|(rgb|rgba|hsl|hsla|oklch|oklab)\([0-9a-zA-Z%.,\/ -]{1,60}\))$/.test(v) ? v : 'inherit';
    }
var schemaCache = null;
  function criticAttrs(dom){
    var raw=dom.getAttribute('data-critic-raw');if(!raw||raw.length>65542)return false;
    try{var data=window.yavcMdEngine.current().inlineReview(raw);return {src:data.src,kind:data.kind,html:window.yavcHTML.clean(data.html),cellSafe:data.cellSafe};}catch(err){return false;}
  }
  function schemaOf(PM) {
    if (schemaCache) return schemaCache;
    var table = PM.tableNodes({ tableGroup: 'block', cellContent: 'inline*', cellAttributes: {
      align: { default: '', getFromDOM: function (dom) { var value = dom.style.textAlign || dom.getAttribute('align') || ''; return /^(left|center|right)$/.test(value) ? value : ''; },
        setDOMAttr: function (value, attrs) { if (/^(left|center|right)$/.test(value)) attrs.style = (attrs.style || '') + 'text-align:' + value + ';'; } },
      /* A table read from an HTML block (DESIGN.md, decision 112) keeps
         each cell's attributes (a colour, a class) and whether its text is
         in a paragraph; the page draws what safe-html allows of them, so a
         cell's background colour shows. */
      xattrs: { default: null, getFromDOM: function (d) { return jsonAttr(d, 'data-pm-xattrs'); } },
      para: { default: false, getFromDOM: function (d) { return d.getAttribute('data-pm-para') === '1'; } }
    } });
    table.table.attrs = withPlace(Object.assign({}, table.table.attrs, { raw: { default: null } }));
    table.table.toDOM = function (n) {
      var raw = n.attrs.raw, dom = document.createElement('table'), body = document.createElement('tbody');
      if (raw) {
        dom.className = 'pm-raw-table';
        safeAttrs(raw.attrs).forEach(function (a) { dom.setAttribute(a[0], a[1]); });
        if (raw.colgroup) { var cg = window.yavcHTML.fragment('<table>' + raw.colgroup + '</table>').querySelector('colgroup'); if (cg) dom.appendChild(cg); }
        dom.setAttribute('data-pm-raw', JSON.stringify(raw));
      }
      dom.appendChild(body);
      return { dom: dom, contentDOM: body };
    };
    table.table.parseDOM = [{ tag: 'table', getAttrs: function (d) { return { raw: rawTableAttrs(d) }; } }];
    table.table_row.attrs = Object.assign({}, table.table_row.attrs, { xattrs: { default: null }, head: { default: false } });
    table.table_row.parseDOM = [{ tag: 'tr', getAttrs: function (d) { return { xattrs: jsonAttr(d, 'data-pm-xattrs'), head: d.getAttribute('data-pm-head') === '1' }; } }];
    table.table_row.toDOM = function (n) {
      var attrs = {};
      safeAttrs(n.attrs.xattrs).forEach(function (a) { attrs[a[0]] = a[1]; });
      if (n.attrs.xattrs) attrs['data-pm-xattrs'] = JSON.stringify(n.attrs.xattrs);
      if (n.attrs.head) attrs['data-pm-head'] = '1';
      return ['tr', attrs, 0];
    };
    ['table_cell', 'table_header'].forEach(function (name) {
      var tag = name === 'table_cell' ? 'td' : 'th';
      table[name].toDOM = function (n) {
        var attrs = { dir: 'auto' };
        safeAttrs(n.attrs.xattrs).forEach(function (a) { attrs[a[0]] = a[1]; });
        delete attrs.colspan; delete attrs.rowspan;
        if (n.attrs.colspan !== 1) attrs.colspan = n.attrs.colspan;
        if (n.attrs.rowspan !== 1) attrs.rowspan = n.attrs.rowspan;
        if (n.attrs.colwidth) attrs['data-colwidth'] = n.attrs.colwidth.join(',');
        if (/^(left|center|right)$/.test(n.attrs.align || '')) attrs.style = (attrs.style || '') + 'text-align:' + n.attrs.align + ';';
        if (n.attrs.xattrs) attrs['data-pm-xattrs'] = JSON.stringify(n.attrs.xattrs);
        if (n.attrs.para) attrs['data-pm-para'] = '1';
        return n.attrs.para ? [tag, attrs, ['p', 0]] : [tag, attrs, 0];
      };
    });
    /* A cell holds its text directly, so it is a text block, and the gap
       cursor would otherwise take the place between two cells for one where
       a block could go: the right arrow at the end of a cell drew it there,
       as an extra cell that pushed the row over. allowGapCursor, documented
       in prosemirror-gapcursor's README, keeps it out of a row. */
    table.table_row = Object.assign({}, table.table_row, { allowGapCursor: false });
    /* An attribute block, the braces after a heading or a link
       ({#anchor .class name="value"}), is kept with its node as the text
       between the braces and is not drawn. It is written on the element so
       that the node read back from the page still has it. The engine checks
       it again before writing it (decision 123). */
    function blockOf(d) { var raw = d.getAttribute('data-md-attributes'); return raw && raw.length <= 1024 && !/[{}\r\n]/.test(raw) ? raw : null; }
    function blockAttr(raw) { return raw ? { 'data-md-attributes': raw } : {}; }
    var headings = [1, 2, 3, 4, 5, 6].map(function (l) { return { tag: 'h' + l, getAttrs: function (d) { return { level: l, attributes: blockOf(d) }; } }; });
    schemaCache = new PM.Schema({
      nodes: {
        /* refs says the text could define a link reference when it was
           read; where it could not, the engine writes text in brackets
           without an escape (decision 118). The default is the careful one. */
        doc: { content: 'block+', attrs: { front: { default: '' }, tail: { default: '\n' }, crlf: { default: false }, refs: { default: true } } },
        paragraph: { group: 'block', content: 'inline*', attrs: withPlace(), parseDOM: [{ tag: 'p' }], toDOM: function () { return ['p', { dir: 'auto' }, 0]; } },
        heading: { group: 'block', content: 'inline*', defining: true, attrs: withPlace({ level: { default: 1 }, attributes: { default: null } }), parseDOM: headings,
          toDOM: function (n) { return ['h' + n.attrs.level, Object.assign({ dir: 'auto' }, blockAttr(n.attrs.attributes)), 0]; } },
        blockquote: { group: 'block', content: 'block+', defining: true, attrs: withPlace(), parseDOM: [{ tag: 'blockquote' }], toDOM: function () { return ['blockquote', { dir: 'auto' }, 0]; } },
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
        list_item: { content: 'paragraph (paragraph | blockquote | bullet_list | ordered_list)*', defining: true, attrs: withPlace({ checked: { default: null } }),
          parseDOM: [{ tag: 'li', contentElement: function (d) { return d.querySelector(':scope > .pm-task-body') || d; },
            getAttrs: function (d) {
              var box = d.querySelector(':scope > input[type="checkbox"], :scope > .pm-check > input[type="checkbox"]');
              return { checked: box ? box.checked : null };
            } }],
          toDOM: function (n) {
            if (n.attrs.checked === null) return ['li', { dir: 'auto' }, 0];
            var box = { type: 'checkbox', tabindex: '0', 'aria-label': n.attrs.checked ? 'Done' : 'Not Done' };
            if (n.attrs.checked) box.checked = '';
            return ['li', { class: 'pm-task', dir: 'auto' }, ['span', { class: 'pm-check', contenteditable: 'false' }, ['input', box]], ['div', { class: 'pm-task-body' }, 0]];
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
            return { src: d.getAttribute('data-src'), kind: d.getAttribute('data-kind') || 'text', html: window.yavcHTML.clean(d.getAttribute('data-html') || '') };
          } }],
          toDOM: function (n) { return ['div', { class: 'pm-source', 'data-src': n.attrs.src || '', 'data-kind': n.attrs.kind, 'data-html': n.attrs.html }]; } },
        /* A container shortcode, a notice say, whose body is blocks edited
           here, drawn inside the HTML the preview draws for it (before and
           after); and a column set, a row of columns each a run of blocks
           (docs/page-view-design.md). Both keep their opening and closing
           lines, to be written back as they were. */
        container: { group: 'block', content: 'block+', defining: true, isolating: true,
          attrs: withPlace({ name: { default: 'note' }, args: { default: '' }, open: { default: '' }, close: { default: '' }, before: { default: '' }, after: { default: '' }, tail: { default: '' } }),
          parseDOM: [{ tag: 'div.pm-box[data-name]:not([data-columns])', getAttrs: function (d) {
            return { name: d.getAttribute('data-name'), args: d.getAttribute('data-args') || '', open: d.getAttribute('data-open') || '', close: d.getAttribute('data-close') || '',
              before: window.yavcHTML.clean(d.getAttribute('data-before') || ''), after: window.yavcHTML.clean(d.getAttribute('data-after') || '') };
          } }],
          toDOM: function (n) { return ['div', { class: 'pm-box', 'data-name': n.attrs.name, 'data-args': n.attrs.args, 'data-open': n.attrs.open, 'data-close': n.attrs.close, 'data-before': n.attrs.before, 'data-after': n.attrs.after }, 0]; } },
        columns: { group: 'block', content: 'column+', defining: true, isolating: true,
          attrs: withPlace({ name: { default: 'columns' }, args: { default: '' }, open: { default: '' }, close: { default: '' }, before: { default: '' }, after: { default: '' }, sep: { default: 'column' }, seps: { default: null } }),
          parseDOM: [{ tag: 'div.pm-box[data-columns]', getAttrs: function (d) { return { name: d.getAttribute('data-name') || 'columns', open: d.getAttribute('data-open') || '', close: d.getAttribute('data-close') || '' }; } }],
          toDOM: function (n) { return ['div', { class: 'pm-box sc-columns', 'data-name': n.attrs.name, 'data-columns': '', 'data-open': n.attrs.open, 'data-close': n.attrs.close }, 0]; } },
        column: { content: 'block+', isolating: true, attrs: { tail: { default: '' } },
          parseDOM: [{ tag: 'div.pm-column' }], toDOM: function () { return ['div', { class: 'pm-column sc-column' }, 0]; } },
        /* A shortcode in a line of text, keys or a status label: one piece
           of the line, drawn as the preview draws it, changed through its
           form, and written back as it was written (src). */
        shortcode: { group: 'inline', inline: true, atom: true, selectable: true, draggable: true,
          attrs: { src: { default: '' }, html: { default: '' } },
          parseDOM: [{ tag: 'span.pm-sc[data-src]', getAttrs: function (d) { return { src: d.getAttribute('data-src'), html: window.yavcHTML.clean(d.getAttribute('data-html') || '') }; } }],
          toDOM: function (n) { return ['span', { class: 'pm-sc', 'data-src': n.attrs.src, 'data-html': n.attrs.html }]; } },
        footnote: { group: 'block', content: 'block+', defining: true,
          attrs: withPlace({ label: { default: '' }, number: { default: -1 }, tail: { default: '' } }),
          parseDOM: [{ tag: 'aside.doc-footnote[data-footnote]', contentElement: '.doc-footnote-body', getAttrs: function (d) { return { label: d.getAttribute('data-footnote'), number: Number(d.getAttribute('data-footnote-number')) || -1 }; } }],
          toDOM: function (n) { return ['aside', { class: 'doc-footnote', 'data-footnote': n.attrs.label, 'data-footnote-number': n.attrs.number },
            ['span', { class: 'doc-footnote-label', contenteditable: 'false' }, n.attrs.number > 0 ? String(n.attrs.number) : n.attrs.label], ['div', { class: 'doc-footnote-body' }, 0]]; } },
        footnote_ref: { group: 'inline', inline: true, atom: true, selectable: true,
          attrs: { label: { default: '' }, number: { default: -1 } },
          parseDOM: [{ tag: 'sup.doc-footnote-ref[data-footnote-ref]', priority:60, getAttrs: function (d) { return { label: d.getAttribute('data-footnote-ref'), number: Number(d.getAttribute('data-footnote-number')) || -1 }; } }],
          toDOM: function (n) { var label = n.attrs.number > 0 ? String(n.attrs.number) : '?'; return ['sup', { class: 'doc-footnote-ref', 'data-footnote-ref': n.attrs.label, 'data-footnote-number': n.attrs.number, contenteditable: 'false' }, ['a', { 'aria-label': 'Footnote ' + label }, label]]; } },
        critic: { group:'inline',inline:true,atom:true,selectable:true,
          attrs:{src:{default:''},kind:{default:''},html:{default:''},cellSafe:{default:false}},
          parseDOM:[{tag:'span.critic[data-critic-raw]',priority:60,getAttrs:criticAttrs}],
          leafText:function(node){return node.attrs.src;},
          toDOM:function(node){return ['span',{class:'critic','data-critic-raw':node.attrs.src},node.attrs.src];} },
        text: { group: 'inline' },
        /* Markup in a cell of a table read from HTML that is not simple
           formatting (a wiki's link or user), kept as it was written and
           drawn as safe-html allows it, as one piece that is not edited. */
        raw_inline: { group: 'inline', inline: true, atom: true, selectable: true, attrs: { xml: { default: '' } },
          parseDOM: [{ tag: 'span.pm-raw-inline[data-xml]', getAttrs: function (d) { var xml = d.getAttribute('data-xml') || ''; return xml.length <= 65536 ? { xml: xml } : false; } }],
          toDOM: function (n) {
            var span = document.createElement('span');
            span.className = 'pm-raw-inline';
            span.setAttribute('data-xml', n.attrs.xml);
            span.setAttribute('data-tooltip', 'Markup kept as it was written');
            span.appendChild(window.yavcHTML.fragment(n.attrs.xml));
            if (!span.textContent.trim()) span.textContent = n.attrs.xml;
            return span;
          } },
        // A line break, as HTML pasted from another program can hold one.
        hard_break: { group: 'inline', inline: true, selectable: false, attrs: { form: { default: null } }, parseDOM: [{ tag: 'br' }], toDOM: function () { return ['br']; } },
        // A line break inside a paragraph, which the page shows as a space.
        soft_break: { group: 'inline', inline: true, selectable: false, parseDOM: [{ tag: 'span.pm-soft' }], toDOM: function () { return ['span', { class: 'pm-soft' }, ' ']; } }
      },
      // In the order pageview writes them, outermost first.
      marks: {
        no_export: {
          parseDOM: [{ tag: 'span.sc-no-export[data-export="omit"]' }],
          toDOM: function () { return ['span', { class: 'sc-no-export', 'data-export': 'omit', title: 'Not exported', role: 'note', 'aria-label': 'Not exported' }, 0]; } },
        // Words inside a span shortcode the host application draws: the
        // shortcode's name and its opening as it is written. One span may
        // be inside another, and words typed at its end are outside it.
        host_span: { attrs: { name: { default: '' }, open: { default: '' } }, excludes: '', inclusive: false,
          parseDOM: [{ tag: 'span.sc-host[data-host-kind="span"]', getAttrs: function (dom) {
            var name = dom.getAttribute('data-host-shortcode'), open = dom.getAttribute('data-host-source');
            return name && open ? { name: name, open: open } : false;
          } }],
          toDOM: function (mark) { return ['span', { class: 'sc-host', 'data-host-shortcode': mark.attrs.name, 'data-host-kind': 'span', 'data-host-source': mark.attrs.open }, 0]; } },
        link: { attrs: { href: {}, title: { default: null }, auto: { default: false }, attributes: { default: null } }, inclusive: false,
          // A pasted link to an address that is not safe is pasted as its
          // text, and one written in the document is drawn without its
          // address (safe-html.js).
          parseDOM: [{ tag: 'a[href]', getAttrs: function (d) { var href = d.getAttribute('href'); return window.yavcHTML.attr('href', href) ? { href: href, attributes: blockOf(d) } : false; } }],
          toDOM: function (m) { return ['a', Object.assign(window.yavcHTML.attr('href', m.attrs.href) ? { href: m.attrs.href } : {}, blockAttr(m.attrs.attributes)), 0]; } },
        strong: { parseDOM: [{ tag: 'strong' }, { tag: 'b' }], toDOM: function () { return ['strong', 0]; } },
        em: { parseDOM: [{ tag: 'em' }, { tag: 'i' }], toDOM: function () { return ['em', 0]; } },
        underline: { parseDOM: [{ tag: 'u' }], toDOM: function () { return ['u', 0]; } },
        strike: { parseDOM: [{ tag: 's' }, { tag: 'del' }], toDOM: function () { return ['s', 0]; } },
        code: { parseDOM: [{ tag: 'code' }], toDOM: function () { return ['code', 0]; } },
        /* Words in a colour, {{color:…}}…{{/color}}: the value as written,
           one of the site's names or any CSS colour. */
        color: { attrs: { value: { default: 'accent' }, name: { default: 'color' } },
          parseDOM: [{ tag: 'span.sc-colour', getAttrs: function (d) { return { value: d.getAttribute('data-value') || d.style.color || 'accent' }; } }],
          toDOM: function (m) { return ['span', { class: 'sc-colour', 'data-value': m.attrs.value, style: 'color:' + colorCSS(m.attrs.value) }, 0]; } },
        /* A class of the document's stylesheet, {{class:name}}…{{/class}},
           drawn as u-name as the engine draws it. */
        class: { attrs: { value: { default: '' }, name: { default: 'class' } },
          parseDOM: [{ tag: 'span[data-class]', getAttrs: function (d) { return { value: d.getAttribute('data-class') }; } }],
          toDOM: function (m) { var c = String(m.attrs.value).replace(/[^A-Za-z0-9-]/g, ''); return ['span', { class: c, 'data-class': c }, 0]; } },
        /* Highlighted, raised and lowered words, {{mark}}…{{/mark}},
           {{sup}}…{{/sup}} and {{sub}}…{{/sub}}: each the HTML element of its
           name, as the engine draws it. Raised and lowered exclude each
           other, so words are one or the other. */
        mark: { parseDOM: [{ tag: 'mark' }], toDOM: function () { return ['mark', { class: 'sc-mark' }, 0]; } },
        sup: { excludes: 'sup sub', parseDOM: [{ tag: 'sup' }], toDOM: function () { return ['sup', 0]; } },
        sub: { excludes: 'sup sub', parseDOM: [{ tag: 'sub' }], toDOM: function () { return ['sub', 0]; } }
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
       inlineOf(src)            a promise of { html } for an inline shortcode
       wrapOf(line)             a promise of a container's parts for its opening line
                                ({ name, args, open, before, after }), for its Title dialog
       editBlock(src)           optional: a drawn block pressed; a promise of its new
                                source, '' to take it out, or null; undefined to edit it here
       editBlock(src)           optional: a drawn block pressed; a promise of its new
                                source, '' to take it out, or null; undefined to edit it here
       editBlock(src)           optional: a drawn block pressed; a promise of its new
                                source, '' to take it out, or null; undefined to edit it here
       fill(el, cachedOnly)     optional: puts the host application's drawings into the
                                placeholders in el (kit.js fillHosts)
       hostSpan(name, open)     optional: { dom, contentDOM } for a host's span shortcode
                                whose drawing is kept, or null
       hostTitle(name)          optional: the title of one of the host's shortcodes, or ''
       hostInline(src)          optional: a promise of { html } when src is one of the
                                host's inline shortcodes, and of null when it is not
       changed(edited)          the Editor's commands may have changed; edited is true when the text of the document did
       failed(text)             the page could not be built; the Editor leaves Page view
     } */
  function attach(o) {
    /* The page's edits reach the Editor's text in placements, at times that depend on the host and the load (the timer, a host's take, an
       undo), and the Editor's undo history merges typing that is less than 500 ms apart. So that the placements do not change what one
       undo takes back, each placement says whether the typing it carries went on from the typing of the placement before it with no
       gap of 500 ms or more, as the page's own history would group it, and the Editor's history then joins the two. */
    var PM = null, view = null, on = false, shown = null, loading = 0, mirrored = null;
    var lastEdit = 0, batchFirst = 0, batchBroke = false, batchLinked = false, placedEdit = 0;
    var footnoteNamespace='yavc-notes-'+crypto.randomUUID();
    var dirty = false, syncTimer = 0, syncing = null, reloadTimer = 0, sourceStale = false, refreshFocus = null, frozen = false;
    /* Whether the next page built takes its place from the text's caret
       (o.caret): set when the page is turned on, so that changing from
       Text to Visual keeps the reader's place; a page built again for a
       change to the text keeps its own scroll instead. */
    var placeFromText = false;
    /* The position just inside the top-level block that holds an offset
       into the document's text, worked out from each block's text and the
       text before it, as the page writes itself back; null when the
       blocks are not exactly the text. */
    function blockAtOffset(doc, offset) {
      var at = (doc.attrs.front || '').length, found = null, last = null;
      doc.forEach(function (node, pos) {
        if (found != null) return;
        var a = node.attrs || {};
        if (typeof a.src !== 'string' || typeof a.gap !== 'string') { found = -1; return; }
        var start = at + a.gap.length, end = start + a.src.length;
        last = pos;
        if (offset <= end) found = pos;
        at = end;
      });
      if (found === -1) return null;
      return found != null ? found + 1 : last != null ? last + 1 : null;
    }
    /* The text offset of the top-level block the selection is in, from
       the blocks of a document read afresh from the text (json), for
       keeping the place when the page's own mapping is not exact. */
    function blockOffset(json, index) {
      var at = ((json.attrs || {}).front || '').length, list = json.content || [];
      for (var i = 0; i < list.length && i <= index; i++) {
        var a = list[i].attrs || {};
        if (typeof a.src !== 'string' || typeof a.gap !== 'string') return null;
        at += a.gap.length;
        if (i === index) return at;
        at += a.src.length;
      }
      return null;
    }
    function updating(value) {
      if (value && !sourceStale && view && view.hasFocus()) refreshFocus = view;
      sourceStale = value;
      if (view) { view.setProps({ editable: function () { return on && !sourceStale && !frozen; } }); view.dom.setAttribute('aria-busy', String(value)); }
      if (!value) {
        if (view && refreshFocus === view && document.hasFocus() && document.activeElement === document.body) view.focus();
        refreshFocus = null;
      }
      o.changed();
    }
    /* The top-level blocks exactly as they were read from the text: the
       same node objects, which ProseMirror keeps for a block nobody
       touched. Such a block's text is its src, so it need not be written
       out again, nor read again when the text changes elsewhere
       (performance review, 2026-11-03). */
    var pristine = null;

    function enable(next) {
      next = !!next;
      if (next && frozen) { frozen = false; if (view) view.setProps({ editable: function () { return on && !sourceStale; } }); }
      if (next === on) return Promise.resolve();
      on = next;
      o.page.classList.toggle('page-view', on);
      if (on) { placeFromText = true; return load(); }
      if (view) view.setProps({ editable: function () { return false; } });
      clearTimeout(reloadTimer);
      return sync().then(close).catch(function (err) { on = true; if (view) view.setProps({ editable: function () { return !sourceStale && !frozen; } }); o.page.classList.add('page-view'); throw err; });
    }
    function close() {
      if (view) { view.destroy(); view = null; }
      shown = null;
      o.changed();
    }

    /* Reads the text into a new editor, keeping the place on the page. */
    function load() {
      if (dirty || syncing) return sync().then(load);
      var ticket = ++loading, text = o.text(), top = o.scroll.scrollTop;
      updating(true);
      return Promise.all([loadProseMirror(), o.docOf(text)]).then(function (both) {
        if (ticket !== loading || !on) return;
        if (o.text() !== text) return load();
        PM = both[0];
        var schema = schemaOf(PM), doc = schema.nodeFromJSON(both[1]), first = 0;
        while (first < doc.content.size && hiddenSource(doc.nodeAt(first))) { first += doc.nodeAt(first).nodeSize; }
        // Metadata is retained in the model, but opening a page places the
        // caret in visible content. A metadata-only page needs a typing slot.
        if (first && first === doc.content.size) { doc = doc.copy(doc.content.append(PM.Fragment.from(schema.nodes.paragraph.create()))); }
        /* The place: the block the text's caret is in, when the page is
           being turned on, so that the reader lands where they were. */
        var placed = false, caret = placeFromText && o.caret ? o.caret() : null;
        placeFromText = false;
        if (typeof caret === 'number') { var inBlock = blockAtOffset(doc, caret); if (inBlock != null) { first = inBlock; placed = true; } }
        var state = PM.EditorState.create({ doc: doc, selection: first ? PM.TextSelection.near(doc.resolve(first)) : undefined, plugins: plugins(schema) });
        if (view) view.updateState(state);
        else {
          o.page.replaceChildren();
          view = new PM.EditorView(o.page, {
            state: state,
            nodeViews: { source_block: sourceView, container: boxView, columns: boxView, shortcode: inlineView,critic:criticView,footnote:footnoteView,footnote_ref:footnoteView },
            markViews: { host_span: hostSpanView },
            handlePaste: handlePaste,
            // HTML pasted from another program is made safe before the
            // editor reads it (safe-html.js).
            transformPastedHTML: function (html) { return window.yavcHTML.clean(html); },
            transformPasted: safeReviewPaste,
            dispatchTransaction: dispatch,
            attributes: Object.assign({ 'aria-label': 'The Document, as Its Page', role: 'textbox', 'aria-multiline': 'true' }, o.spellcheck === false ? { spellcheck: 'false' } : {})
          });
          /* A composition that is about to begin starts from a page already written
             into the text, so that a host asking for the draft while it lasts gets the
             text as it was before it. When it ends the host is told of the edit.
             The listener is added after ProseMirror's own, so view.composing is already
             false when it runs, and the last of the composition's changes has been read. */
          view.dom.addEventListener('compositionstart', function () { flush(); }, true);
          view.dom.addEventListener('compositionend', function () {
            setTimeout(function () { if (view && !view.composing) { if (dirty) schedule(); o.changed(true); } }, 0);
          });
        }
        shown = text;
        dirty = false;
        updating(false);
        pristine = new WeakSet();
        view.state.doc.forEach(function (n) { pristine.add(n); });
        // The suggestions are placed once the page is known to be the text.
        view.dispatch(view.state.tr.setMeta(suggestKey, true));
        if (placed) view.dispatch(view.state.tr.scrollIntoView().setMeta('addToHistory', false));
        else o.scroll.scrollTop = top;
        o.changed();
      }).catch(function (err) {
        if (ticket === loading) o.failed('The page could not be built, so it is drawn from the text until Visual is chosen again: ' + String(err && err.message || err));
      });
    }

    function dispatch(tr) {
      if (!view) return;
      if ((!on || sourceStale || frozen) && tr.docChanged && !tr.getMeta('fromText')) return;
      view.updateState(view.state.apply(tr));
      seenSelection = domSelection();
      /* The reader moved the selection (a click, an arrow key): the host maps it to the text. A selection the host itself mirrored is not told back. */
      if (tr.selectionSet && !tr.docChanged && !tr.getMeta('mirror') && o.selected) { o.selected(); }
      var edited = tr.docChanged && !tr.getMeta('fromText');
      if (edited) {
        var editAt = tr.time || Date.now();
        if (!batchFirst) { batchFirst = editAt; batchLinked = !!placedEdit && editAt - placedEdit < 500; }
        else if (editAt - lastEdit >= 500) batchBroke = true;
        lastEdit = editAt;
        dirty = true;
        schedule();
      }
      /* The host is told of an edit at once, so that its copy of the draft is asked for
         when it was made, and not 300 milliseconds later when the text is written. While
         an input method is composing the host is told when it ends instead. */
      o.changed(edited && !view.composing);
    }
    /* Writes the page's text into the Editor's text after a pause in typing, and
       not while an input method is composing, since converting then can break the
       composition. */
    function schedule() {
      clearTimeout(syncTimer);
      syncTimer = setTimeout(function () {
        if (view && view.composing) { schedule(); return; }
        sync().catch(function () { /* sync reports the failure beside the page. */ });
      }, 300);
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
    function safeReviewPaste(slice){
      var literal=false;
      function node(value,cell){
        cell=cell||value.type.spec.tableRole==='cell'||value.type.spec.tableRole==='header_cell';
        if(value.type.name==='critic'){
            var data;try{data=window.yavcMdEngine.current().inlineReview(value.attrs.src);}catch(err){data=null;}
            if(!data||cell&&!data.cellSafe){literal=true;return view.state.schema.text(value.attrs.src,value.marks);}
          return value.type.create({src:data.src,kind:data.kind,html:cell?data.cellHTML:data.html,cellSafe:data.cellSafe},null,value.marks);
        }
        if(!value.childCount)return value;var content=[];value.forEach(function(child){content.push(node(child,cell));});return value.copy(PM.Fragment.from(content));
      }
      var content=[];slice.content.forEach(function(value){content.push(node(value,!!view&&inCell(view.state)));});
      if(literal)o.say('This review annotation was pasted as literal text because it cannot fit the table cell safely.',true);
      return new PM.Slice(PM.Fragment.from(content),slice.openStart,slice.openEnd);
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
      var target = view, before = view && view.state.doc, selection = view && view.state.selection;
      return Promise.resolve(o.blocksOf(text.replace(/\r\n/g, '\n'))).then(function (json) {
        if (!view || view !== target || !on || view.state.doc !== before || !view.state.selection.eq(selection)) { o.say('The page or selection changed before Paste completed. Paste again.', true); return; }
        var nodes = (json || []).map(function (j) { return view.state.schema.nodeFromJSON(j); });
        if (!nodes.length) return;
        var one = nodes.length === 1 && nodes[0].type.name === 'paragraph';
        var slice = one ? new PM.Slice(PM.Fragment.from(nodes[0]), 1, 1) : new PM.Slice(PM.Fragment.fromArray(nodes), 0, 0);
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
      var original = shown, mine = ++syncTicket;
      var running = Promise.resolve(o.markdownOf(syncJSON())).then(function (text) {
        // flush() has written a newer page since, and this one is no longer wanted.
        if (mine !== syncTicket) return;
        place(text, original);
      }).catch(function (err) {
        if (mine !== syncTicket) return;
        dirty = true;
        o.say(String(err && err.message || err), true);
        throw err;
      }).finally(function () { if (syncing === running) syncing = null; });
      syncing = running;
      return running;
    }
    /* Puts text the page was converted to into the Editor's text, as sync and flush both do. */
    function place(text, original) {
      if (o.text() !== original || shown !== original) throw new Error('The source changed while the page was being saved. The page draft was retained; copy it before reloading.');
      shown = text;
      o.setText(text, false, !!(batchFirst && batchLinked && !batchBroke));
      placedEdit = lastEdit; batchFirst = 0; batchBroke = false; batchLinked = false;
    }
    var syncTicket = 0;
    /* The same as sync, at once and without a promise, for a host that asks for the
       draft and needs it now (the snapshot's take). The engine converts the page in
       one synchronous call, so this is the conversion the timer runs, not another.
       It leaves the page, the selection and the undo history as they are. It does
       nothing while an input method is composing, so the text is then what it was
       before the composition began, which flush() made current when it began.
       Returns whether the Editor's text is now the page's. */
    function flush() {
      clearTimeout(syncTimer);
      if (!view || (!dirty && !syncing)) return true;
      if (view.composing) { schedule(); return false; }
      var original = shown, ticket = ++syncTicket;
      try {
        var text = o.markdownNow(syncJSON());
        dirty = false;
        place(text, original);
        return true;
      } catch (err) {
        // A failed conversion leaves the draft on the page and the timer to try again.
        syncTicket = ticket - 1;
        dirty = true;
        schedule();
        return false;
      }
    }

    /* The page as the engine writes it back, with each block still as it
       was read sent as its own Markdown (a source block of its src), so
       only the blocks that changed are written out again. The engine
       writes an untouched block back as its src anyway, so the text is the
       same either way. */
    function syncJSON() {
      var doc = view.state.doc, out = { type: 'doc', attrs: doc.attrs, content: [] };
      doc.forEach(function (node) {
        if (pristine && pristine.has(node) && node.attrs.src != null && node.attrs.gap != null) {
          out.content.push({ type: 'source_block', attrs: { uid: node.attrs.uid, gap: node.attrs.gap, src: node.attrs.src, kind: 'text', html: '' } });
        } else out.content.push(node.toJSON());
      });
      return out;
    }

    /* The text changed other than from the page (the text pane in Split,
       an agent, a suggestion taken): the blocks the change touched are
       read again and put in place, or, where that cannot be done safely,
       the whole page is read again. */
    function textChanged(force) {
      if (!on) return;
      if (!dirty && !syncing && (force === true || o.text() !== shown)) updating(true);
      clearTimeout(reloadTimer);
      reloadTimer = setTimeout(function () {
        if (on && !dirty && !syncing && (force === true || o.text() !== shown)) {
          if (force === true || !patchFromText(o.text())) load(); else updating(false);
        } else if (on && o.text() === shown && sourceStale) updating(false);
      }, 150);
    }
    /* Reads again only the blocks a change to the text touched, with one
       block either side, since a change can join or split blocks; false
       when the page is not exactly the text it was read from, or the
       change reaches the front matter or what follows the last block. */
    function patchFromText(text) {
      // A definition can change invocations anywhere in the document, and
      // a link reference's definition can change text in brackets anywhere.
      if (text.indexOf('{{define:') >= 0 || (shown || '').indexOf('{{define:') >= 0) return false;
      var defines = /(^|\n) {0,3}\[[^\]\n]+\]:/;
      if (defines.test(text) || defines.test(shown || '')) return false;
      if (!view || shown == null || !pristine) return false;
      var doc = view.state.doc, attrs = doc.attrs, front = attrs.front || '', tail = attrs.tail == null ? '\n' : attrs.tail;
      if (attrs.crlf) return false;
      var at = front.length, spans = [], ok = true;
      doc.forEach(function (node, offset) {
        if (!ok) return;
        var a = node.attrs || {};
        if (!pristine.has(node) || a.src == null || a.gap == null) { ok = false; return; }
        var start = at + a.gap.length, end = start + a.src.length;
        spans.push({ start: start, end: end, pos: offset, size: node.nodeSize, gap: a.gap, src: a.src });
        at = end;
      });
      if (!ok || !spans.length || at + tail.length !== shown.length) return false;
      var p = 0, max = Math.min(shown.length, text.length);
      while (p < max && shown.charCodeAt(p) === text.charCodeAt(p)) p++;
      var q = 0;
      while (q < max - p && shown.charCodeAt(shown.length - 1 - q) === text.charCodeAt(text.length - 1 - q)) q++;
      var oldEnd = shown.length - q, delta = text.length - shown.length, last = spans.length - 1;
      if (p < spans[0].start || oldEnd > spans[last].end) return false;
      var i = 0; while (i < last && spans[i].end < p) i++;
      var j = last; while (j > 0 && spans[j].start > oldEnd) j--;
      if (j < i) j = i;
      i = Math.max(0, i - 1); j = Math.min(last, j + 1);
      for (var k = i; k <= j; k++) { if (shown.slice(spans[k].start, spans[k].end) !== spans[k].src) return false; }
      var from = spans[i].start, toNew = spans[j].end + delta;
      if (toNew <= from) return false;
      var slice = text.slice(from, toNew);
      // A slice starting --- would be read as front matter.
      if (/^---[ \t]*(\n|$)/.test(slice)) return false;
      /* A contents list draws the whole document's headings, which a
         slice read on its own would not have: a change that touches a
         heading or a contents list is read with the whole page (decision
         114); any other change leaves every list right. */
      var listOrHeading = /(^|\n)[ \t]{0,3}#|\{\{toc[:}]/;
      if (listOrHeading.test(slice) || listOrHeading.test(shown.slice(from, spans[j].end))) return false;
      var ticket = ++loading, before = shown, gap = spans[i].gap;
      return Promise.resolve(o.docOf(slice)).then(function (json) {
        if (ticket !== loading || !view || shown !== before || !on) return;
        if (!json || (json.attrs && (json.attrs.front || json.attrs.crlf)) || (json.attrs && json.attrs.tail)) { load(); return; }
        // Fresh ids above every one in the page, so no block takes another's place.
        var top = 0;
        view.state.doc.descendants(function (n) { if (n.attrs && typeof n.attrs.uid === 'number' && n.attrs.uid > top) top = n.attrs.uid; });
        (function renumber(list) { (list || []).forEach(function (n) { if (n.attrs && n.attrs.uid != null) n.attrs.uid = ++top; renumber(n.content); }); })(json.content);
        if (json.content && json.content[0] && json.content[0].attrs) json.content[0].attrs.gap = gap;
        var schema = view.state.schema, nodes = (json.content || []).map(function (j) { return schema.nodeFromJSON(j); });
        if (!nodes.length) { load(); return; }
        var cur = view.state.doc, fromPos = 0, toPos = 0;
        cur.forEach(function (n, offset, index) { if (index === i) fromPos = offset; if (index === j) toPos = offset + n.nodeSize; });
        var tr = view.state.tr.replaceWith(fromPos, toPos, nodes).setMeta('addToHistory', false).setMeta('fromText', true);
        view.dispatch(tr);
        view.state.doc.forEach(function (n) { if (nodes.indexOf(n) >= 0) pristine.add(n); });
        shown = text;
        view.dispatch(view.state.tr.setMeta(suggestKey, true).setMeta('fromText', true));
        o.changed();
      }).catch(function () { if (ticket === loading) load(); });
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
    var suggested = { text: null, list: [] }, suggestKey = null, placesLoading=null,placeTexts=new WeakMap();
    function prepareSuggestionPlaces(doc,text){
      if(placesLoading&&placesLoading.doc===doc&&placesLoading.text===text)return;
      var ticket={doc:doc,text:text};placesLoading=ticket;var entries=[];
      doc.forEach(function(node,pos){
        var promise=placeTexts.get(node);
        if(!promise){var json=node.toJSON();json.attrs=Object.assign({},json.attrs,{gap:''});promise=Promise.resolve(o.markdownOf({type:'doc',attrs:{front:'',tail:'',crlf:false},content:[json]}));placeTexts.set(node,promise);}
        entries.push({node:node,pos:pos,promise:promise});
      });
      Promise.all(entries.map(function(entry){return entry.promise;})).then(function(parts){
        if(placesLoading!==ticket||!view||view.state.doc!==doc||shown!==text||suggested.text!==text)return;
        var front=doc.attrs.front||'',out=front,places=[],started=false;
        entries.forEach(function(entry,index){var part=parts[index];if(!part)return;var gap=typeof entry.node.attrs.gap==='string'?entry.node.attrs.gap:started?'\n\n':front?'\n':'';out+=gap;places.push({from:out.length,to:out.length+part.length,pos:entry.pos,size:entry.node.nodeSize});out+=part;started=true;});
        out+=doc.attrs.tail==null?'\n':doc.attrs.tail;
        if(out!==text)return;
        suggested.places=places;suggested.doc=doc;view.dispatch(view.state.tr.setMeta(suggestKey,true));
      }).catch(function(){/* Unplaceable overlays stay in the host's review list. */}).finally(function(){if(placesLoading===ticket)placesLoading=null;});
    }

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

    /* A card for a change to rows of a table is a row of the table, with one cell across every column, so that it comes straight after
       the changed row. It is drawn only, and is no part of the document. */
    function inTableRow(card, columns) {
      var row = document.createElement('tr'), cell = document.createElement('td');
      row.className = 'host-suggestion-row'; row.contentEditable = 'false';
      cell.colSpan = columns; cell.className = 'host-suggestion-row-cell';
      cell.append(card); row.append(cell);
      return row;
    }
    function suggestionWidget(s, markdown, columns) {
      var make = suggestionCard(s, markdown);
      return columns ? function () { return inTableRow(make(), columns); } : make;
    }
    function suggestionCard(s, markdown) {
      return function () {
        if(o.hostSuggestionWidget)return o.hostSuggestionWidget(s);
        var box = document.createElement('div');
        box.className = 'pm-sugg';
        box.setAttribute('role', 'note');
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

    /* Where a text is on the page inside a node, when it is there once
       and inside one paragraph or cell; null otherwise. A shortcode or a
       picture in the line counts as one character that no text matches. */
    function onceOnPage(node, pos, needle) {
      var found = null, many = false;
      function search(block, at) {
        var text = '', plain = true;
        block.forEach(function (child) { if (child.isText) text += child.text; else if (child.isLeaf) text += '￼'; else plain = false; });
        if (!plain) { if (block.textContent.indexOf(needle) >= 0) many = true; return; }
        var first = text.indexOf(needle);
        if (first < 0) return;
        if (found || text.indexOf(needle, first + 1) >= 0) { many = true; return; }
        found = { from: at + 1 + first, to: at + 1 + first + needle.length };
      }
      if (node.isTextblock) search(node, pos);
      else node.descendants(function (child, offset) { if (child.isTextblock) { search(child, pos + 1 + offset); return false; } return true; });
      return many ? null : found;
    }

    /* The marks for a suggestion that changes part of one block, or null
       when the whole block is to be marked. changedFrom and changedTo are
       the part of the text the suggestion changes, without the words it
       leaves as they are at each end. A table whose text has a line for
       each row has only the changed rows marked. Changed words that are
       in the text once, and on the page once, are struck where they
       stand; a place that cannot be found that surely keeps the wider
       mark. */
    function narrowedMarks(doc, block, text, changedFrom, changedTo, id) {
      var node = doc.nodeAt(block.pos);
      if (!node || changedFrom < block.from || changedTo > block.to || changedTo < changedFrom) return null;
      var src = text.slice(block.from, block.to), old = text.slice(changedFrom, changedTo), marks = [];
      function word(within, at, source) {
        if (!old.trim() || /\n/.test(old)) return null;
        var first = source.indexOf(old);
        if (first < 0 || source.indexOf(old, first + 1) >= 0) return null;
        var where = onceOnPage(within, at, old);
        return where ? PM.Decoration.inline(where.from, where.to, { class: 'pm-sugg-word', 'data-suggestion': id }) : null;
      }
      if (node.type.name === 'table') {
        var lines = src.split('\n'), html = /^\s*<tr[\s>].*<\/tr>\s*$/i, piped = lines.length > 1 && /^\s*\|?\s*:?-{1,}:?\s*(\||$)/.test(lines[1]);
        var rows = [], start = block.from, touched = [];
        lines.forEach(function (line, index) {
          var end = start + line.length, isRow = piped ? index !== 1 && !!line.trim() : html.test(line);
          var hit = changedTo > changedFrom ? start < changedTo && end > changedFrom : changedFrom >= start && changedFrom <= end;
          if (isRow) rows.push({ from: start, line: line });
          if (hit) touched.push(isRow ? rows.length - 1 : -1);
          start = end + 1;
        });
        if (rows.length !== node.childCount || !touched.length || touched.indexOf(-1) >= 0) return null;
        var at = block.pos + 1;
        node.forEach(function (row, offset, index) {
          if (touched.indexOf(index) < 0) return;
          marks.push(PM.Decoration.node(at + offset, at + offset + row.nodeSize, { class: 'pm-sugg-old', 'data-suggestion': id }));
          var struck = touched.length === 1 ? word(row, at + offset, rows[index].line) : null;
          if (struck) marks.push(struck);
          marks.rowEnd = at + offset + row.nodeSize;
        });
        marks.columns = 0; node.firstChild.forEach(function (cell) { marks.columns += cell.attrs.colspan || 1; });
        return marks;
      }
      var only = word(node, block.pos, src);
      return only ? [only] : null;
    }

    function suggestionDecorations(doc,previous) {
      var none = PM.DecorationSet.empty;
      if (!suggested.list.length) return none;
      if(suggested.text!==shown)return previous||none;
      // The page counts a line end as one character; a text with CRLF line
      // ends counts two, so its places are counted again without them.
      var text = suggested.text, crlf = text.indexOf('\r\n') >= 0;
      var lf = crlf ? text.replace(/\r\n/g, '\n') : text;
      function place(at) { return crlf ? at - (text.slice(0, at).match(/\r\n/g) || []).length : at; }
      var blocks = suggested.doc===doc?suggested.places:blockPlaces(doc, lf);
      if (!blocks) {
        prepareSuggestionPlaces(doc,lf);
        return previous||none;
      }
      var decos = [];
      suggested.list.forEach(function (s) {
        var from = place(s.from), to = place(s.to);
        var overlaps = function (b) { return b.from < to && b.to > from; };
        var hit = blocks.filter(overlaps);
        var start = from, end = to, at, columns = 0;
        if (hit.length) {
          start = Math.min(from, hit[0].from);
          end = Math.max(to, hit[hit.length - 1].to);
          var narrowed = hit.length === 1 && !s.stale ? narrowedMarks(doc, hit[0], lf, from + (s.head || 0), to - (s.tail || 0), s.id) : null;
          if (narrowed) narrowed.forEach(function (d) { decos.push(d); });
          else hit.forEach(function (b) { decos.push(PM.Decoration.node(b.pos, b.pos + b.size, { class: 'pm-sugg-old'+(s.stale?' host-suggestion-stale':''), 'data-suggestion': s.id })); });
          var last = hit[hit.length - 1];
          at = last.pos + last.size;
          /* A change to rows of a table puts its card straight after the last changed row, so that it is not found at the end of the table. */
          if (narrowed && narrowed.rowEnd && narrowed.columns) { at = narrowed.rowEnd; columns = narrowed.columns; }
        } else {
          // An addition between blocks goes after the block before it.
          var before = blocks.filter(function (b) { return b.to <= from; }).pop();
          at = before ? before.pos + before.size : 0;
        }
        var markdown = lf.slice(start, from) + s.after.replace(/\r\n/g, '\n') + lf.slice(to, end);
        decos.push(PM.Decoration.widget(at, suggestionWidget(s, markdown, columns), { key: 'sugg-'+JSON.stringify([columns,s.id,o.hostSuggestionWidget?null:markdown,s.before,s.after,s.author,s.reason,s.stale,s.edited,s.pending,s.readOnly]), side: 1, ignoreSelection: true, stopEvent: function () { return true; } }));
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
            if (tr.getMeta(suggestKey)) return suggestionDecorations(tr.doc,set);
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
      var prior=suggested;
      suggested = { text: text, list: list || [],places:prior.text===text?prior.places:null,doc:prior.text===text?prior.doc:null };
      if (view && suggestKey) view.dispatch(view.state.tr.setMeta(suggestKey, true));
    }

    function footnoteView(node) {
      var rendered=PM.DOMSerializer.renderSpec(document,node.type.spec.toDOM(node)),dom=rendered.dom,isReference=node.type.name==='footnote_ref';
      function update(next){
        if(next.type!==node.type)return false;
        node=next;
        if(isReference){var label=next.attrs.number>0?String(next.attrs.number):'?';dom.setAttribute('data-footnote-ref',next.attrs.label);dom.setAttribute('data-footnote-number',next.attrs.number);dom.querySelector('a').textContent=label;dom.querySelector('a').setAttribute('aria-label','Footnote '+label);}
        else {dom.setAttribute('data-footnote',next.attrs.label);dom.setAttribute('data-footnote-number',next.attrs.number);dom.querySelector('.doc-footnote-label').textContent=next.attrs.number>0?String(next.attrs.number):next.attrs.label;}
        window.yavcMdEditor.scopeFootnotes(dom,footnoteNamespace);return true;
      }
      update(node);
      return {dom:dom,contentDOM:rendered.contentDOM,update:update,ignoreMutation:function(mutation){return mutation.type!=='selection'&&(isReference||mutation.type==='attributes'&&mutation.target===dom||dom.querySelector('.doc-footnote-label')&&dom.querySelector('.doc-footnote-label').contains(mutation.target));}};
    }
    function footnoteNumbers() {
      return new PM.Plugin({appendTransaction:function(transactions,old,state){
        if(!transactions.some(function(tr){return tr.docChanged;}))return null;
        var definitions=new Map(),numbers=new Map();
        state.doc.descendants(function(node){if(node.type.name==='footnote'&&!definitions.has(node.attrs.label))definitions.set(node.attrs.label,node);});
        function collect(node){if(node.type.name==='footnote')return false;if(node.type.name==='footnote_ref'&&definitions.has(node.attrs.label)&&!numbers.has(node.attrs.label))numbers.set(node.attrs.label,numbers.size+1);}
        state.doc.descendants(collect);
        for(var label of numbers.keys())definitions.get(label).descendants(collect);
        var seen=new Set(),tr=state.tr;
        state.doc.descendants(function(node,pos){
          if(node.type.name!=='footnote'&&node.type.name!=='footnote_ref')return;
          var number=numbers.get(node.attrs.label)||-1;
          if(node.type.name==='footnote'){if(seen.has(node.attrs.label))number=-1;seen.add(node.attrs.label);}
          if(node.attrs.number!==number)tr.setNodeMarkup(pos,null,Object.assign({},node.attrs,{number:number}));
        });
        return tr.steps.length?tr.setMeta('addToHistory',false):null;
      }});
    }
    function plugins(schema) {
      var n = schema.nodes;
      // Consult current preferences without rebuilding the editor state or
      // replacing its undo history when bindings change.
      var keys = new PM.Plugin({ props: { handleKeyDown: function (v, event) {
        var current = PM.keymap(Object.fromEntries((o.keys() || []).map(function (k) { return [k.key, function () { return k.run(); }]; })));
        return current.props.handleKeyDown(v, event);
      } } });
      /* Shift+Enter breaks the line inside the paragraph. The break is written as a backslash before the newline, which CommonMark
         defines and which is visible in the text. A table's cell and a heading have no such break in Markdown, so there the key
         says so and writes nothing. */
      function lineBreak(state, d) {
        var parent = state.selection.$from.parent;
        if (parent.type !== n.paragraph || inCell(state)) {
          if (d) o.say('A line break can be typed inside a paragraph only.', true);
          return true;
        }
        if (d) d(state.tr.replaceSelectionWith(n.hard_break.create()).scrollIntoView());
        return true;
      }
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
      /* Enter on an empty last line of a box leaves the box, as it leaves a
         list: the empty line goes, and a new one follows the box. */
      function exitBox(state, d) {
        var $c = state.selection.$from;
        if (!state.selection.empty || $c.parent.type !== n.paragraph || $c.parent.content.size || $c.depth < 2) return false;
        var depth = $c.depth - 1, box = $c.node(depth);
        if (box.type !== n.container && box.type !== n.column) return false;
        if ($c.index(depth) !== box.childCount - 1 || box.childCount < 2) return false;
        var outer = depth;
        if (box.type === n.column) {
          var cols = $c.node(depth - 1);
          if ($c.index(depth - 1) !== cols.childCount - 1) return false;
          outer = depth - 1;
        }
        var after = $c.after(outer) - $c.parent.nodeSize;
        var tr = state.tr.delete($c.before(depth + 1), $c.after(depth + 1));
        tr.insert(after, n.paragraph.create());
        tr.setSelection(PM.TextSelection.create(tr.doc, after + 1));
        if (d) d(tr.scrollIntoView());
        return true;
      }
      return [
        /* A key is handled with the selection the browser has now. A caret moved by the browser's own key (End, an arrow, a click) reaches the
           state on a later selectionchange event, and a key that follows it at once, as a script sends them, found the old selection and
           typed or broke a line there (docs/backlog.md; syncSelection). This runs before every other key handler of the page. */
        new PM.Plugin({ props: { handleKeyDown: function () { syncSelection(); return false; } } }),
        listShortcuts(schema),
        taskBoxes(schema),
        footnoteNumbers(),
        hostShortcuts(schema),
        window.yavcDocumentPages.plugin(PM, Object.assign({}, o, { syncSelection: function () { syncSelection(); } })),
        suggestionPlugin(),
        findPlugin(),
        mirrorPlugin(),
        slashMenu(schema),
        emptyLineHint(schema),
        keys,
        PM.keymap({
          'Tab': PM.chainCommands(PM.goToNextCell(1), addRowAtEnd, sink),
          'Shift-Tab': PM.chainCommands(PM.goToNextCell(-1), lift),
          'Enter': PM.chainCommands(exitBox, splitItem),
          'Shift-Enter': lineBreak,
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

    /* Typing a list's Markdown at the start of a line makes the list, as it
       would in the source: "- ", "* " or "+ " a bulleted list, "1. " a
       numbered one, "- [ ] " or "- [x] " a checklist item, and "[ ] " or
       "[x] " at the start of a list item makes that item a checklist item.
       The space that finishes the shortcut is not typed; Undo brings the
       typed text back. */
    function listShortcuts(schema) {
      var n = schema.nodes;
      function itemDepth($p) { return $p.depth > 1 && $p.node($p.depth - 1).type === n.list_item && $p.index($p.depth - 1) === 0 ? $p.depth - 1 : -1; }
      function tick(view, box) {
        var $p = view.state.selection.$from, d = itemDepth($p);
        if (d < 0) return;
        var item = $p.node(d);
        view.dispatch(view.state.tr.setNodeMarkup($p.before(d), null, Object.assign({}, item.attrs, { checked: /x/i.test(box) })));
      }
      return new PM.Plugin({
        props: {
          handleTextInput: function (view, from, to, text) {
            if (text !== ' ' || from !== to) return false;
            var state = view.state, $p = state.doc.resolve(from);
            if ($p.parent.type !== n.paragraph) return false;
            var before = $p.parent.textBetween(0, $p.parentOffset, null, '￼');
            var start = $p.start(), m;
            if ((m = /^\[([ xX])\]$/.exec(before)) && itemDepth($p) >= 0 && $p.node(itemDepth($p)).attrs.checked === null) {
              var d = itemDepth($p), item = $p.node(d);
              view.dispatch(state.tr.delete(start, from).setNodeMarkup($p.before(d), null, Object.assign({}, item.attrs, { checked: /x/i.test(m[1]) })));
              return true;
            }
            var bullet = /^([-*+])(?: \[([ xX])\])?$/.exec(before), number = /^([0-9]{1,9})\.$/.exec(before);
            if (!bullet && !number) return false;
            var list = bullet ? n.bullet_list : n.ordered_list;
            var attrs = bullet ? { marker: bullet[1] } : { order: parseInt(number[1], 10) };
            var after = state.apply(state.tr.delete(start, from));
            if (!PM.wrapInList(list, attrs)(after)) return false;
            view.dispatch(state.tr.delete(start, from));
            PM.wrapInList(list, attrs)(view.state, view.dispatch);
            if (bullet && bullet[2] !== undefined) tick(view, bullet[2]);
            return true;
          }
        }
      });
    }

    /* One of the host application's inline shortcodes, typed out on the
       page, becomes the shortcode as its closing brace is typed. The braces
       are typed as any character is, and the host is asked afterwards, so
       text that is not the host's stays as it was typed. Undo brings the
       typed text back. */
    function hostShortcuts(schema) {
      var LEAF = String.fromCharCode(0xfffc);
      return new PM.Plugin({
        props: {
          handleTextInput: function (v, from, to, text) {
            if (text !== '}' || from !== to || !o.hostInline || v.composing) return false;
            var state = v.state, ticket = loading, $p = state.doc.resolve(from);
            if (!$p.parent.isTextblock || $p.parent.type.spec.code) return false;
            var before = $p.parent.textBetween(0, $p.parentOffset, null, LEAF) + '}';
            var m = /\{\{(?:[@#~!$%&+=?^][^{}\s]+|[a-z][a-z0-9-]*(?::[^{}\n]*)?)\}\}$/.exec(before);
            if (!m || m[0].indexOf(LEAF) >= 0) return false;
            var src = m[0], start = from - (src.length - 1);
            if (start < $p.start() || schema.marks.code && state.doc.rangeHasMark(start, from, schema.marks.code)) return false;
            v.dispatch(state.tr.insertText(text, from, to));
            var inserted = v.state.doc;
            Promise.resolve(o.hostInline(src)).then(function (r) {
              if (!r || !r.html || !on || sourceStale || frozen || loading !== ticket || view !== v) return;
              var st = v.state, end = start + src.length;
              if (st.doc !== inserted || end > st.doc.content.size) return;
              var $s = st.doc.resolve(start), $e = st.doc.resolve(end);
              if (!$s.sameParent($e) || !$s.parent.isTextblock || st.doc.textBetween(start, end, null, LEAF) !== src) return;
              v.dispatch(st.tr.replaceWith(start, end, st.schema.nodes.shortcode.create({ src: src, html: r.html })));
            }).catch(function () {});
            return true;
          }
        }
      });
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
      if ($c.parent.type !== n.paragraph || !inBody($c, n)) return null;
      return $c;
    }
    /* Whether a position's block sits directly in the page or in a box's
       body (a container, or a column), where a block of its own can go. */
    function inBody($c, n) {
      if ($c.depth === 1) return true;
      var parent = $c.depth > 1 ? $c.node($c.depth - 1).type : null;
      return parent === n.container || parent === n.column;
    }

    /* The / menu: typed at the start of an empty line, it lists the blocks
       that can be inserted, narrowed by what follows the /; the arrow keys
       and Enter choose one, and Escape closes it. */
    var STRUCTURE = [
      { id: 'h1', label: 'Heading 1', words: 'heading title h1' }, { id: 'h2', label: 'Heading 2', words: 'heading h2' }, { id: 'h3', label: 'Heading 3', words: 'heading h3' },
      { id: 'paragraph', label: 'Paragraph', words: 'text body' }, { id: 'bullets', label: 'Bulleted List', words: 'list bullets' }, { id: 'numbers', label: 'Numbered List', words: 'list numbers ordered' },
      { id: 'quote', label: 'Quote', words: 'blockquote quotation' }
    ];
    function slashItems() {
      var host = typeof o.inserts === 'function' ? (o.inserts() || []) : [];
      return STRUCTURE.concat(INSERTS, host.filter(function (i) { return i && i.label; }));
    }
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
        if (typeof item.run === 'function') { Promise.resolve(item.run()).catch(function(err){if(o.say)o.say(String(err&&err.message||err),true);}); return; }
        if (item.markdown && !insertOf(item.id)) { insertMarkdown(item.markdown); return; }
        run(item.id);
      }
      function draw(v, q) {
        shown = slashItems().filter(function (i) { return !q.q || i.label.toLowerCase().indexOf(q.q) >= 0 || (i.words || '').indexOf(q.q) >= 0; });
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
    /* Where the text's caret or selection is, drawn on the page while the page is not the pane being typed in (Split view): a thin caret, or a
       highlight over a range. It is drawn only while the state's selection is still the one that was mirrored, so any move on the page
       takes it away, and the stylesheet hides it while the page has the focus. */
    function mirrorPlugin() {
      return new PM.Plugin({
        props: {
          decorations: function (state) {
            var m = mirrored, sel = state.selection;
            if (!m || sel.from !== m.from || sel.to !== m.to || m.doc !== state.doc) return null;
            if (sel.empty) return PM.DecorationSet.create(state.doc, [PM.Decoration.widget(sel.from, function () { var c = document.createElement('span'); c.className = 'pm-mirror-caret'; c.setAttribute('aria-hidden', 'true'); return c; }, { key: 'mirror-caret', side: -1, ignoreSelection: true })]);
            return PM.DecorationSet.create(state.doc, [PM.Decoration.inline(sel.from, sel.to, { class: 'pm-mirror-range' })]);
          }
        }
      });
    }
    function emptyLineHint(schema) {
      return new PM.Plugin({
        props: {
          decorations: function (state) {
            var $c = blockLine(state, schema);
            if (!$c || $c.parent.content.size) return null;
            var at = $c.before();
            return PM.DecorationSet.create(state.doc, [PM.Decoration.node(at, at + $c.parent.nodeSize, { class: 'pm-hint', 'data-hint': 'Type / to insert a heading, a list, a picture, or more' })]);
          }
        }
      });
    }

    /* Where a new block goes: in place of the empty line the cursor is on,
       or after the top-level block it is in. */
    function insertPlace(state) {
      var sel = state.selection;
      if (sel instanceof PM.NodeSelection) return { from: sel.to, to: sel.to };
      var $f = sel.$from, d = $f.depth, n = state.schema.nodes;
      if ($f.depth === 0) return { from: sel.from, to: sel.from };
      // The block directly inside the nearest body: the page, or a box's.
      while (d > 1) { var parent = $f.node(d - 1).type; if (parent === n.container || parent === n.column) break; d--; }
      var block = $f.node(d);
      if (block.isTextblock && block.content.size === 0) return { from: $f.before(d), to: $f.after(d) };
      return { from: $f.after(d), to: $f.after(d) };
    }

    /* Inserts blocks and puts the cursor in them: on the placeholder, in
       the first cell, or, for a code block, in its Markdown; with drawn, a
       block drawn as the preview draws it, such as an image, stays drawn. */
    function insertBlocks(nodes, placeholder, drawn, replacing) {
      var v = view, hole = placeholder || PLACEHOLDER;
      if (!v || !nodes.length) return;
      /* With replacing, a selected passage of text goes first, in the same transaction, so that it is one step of Undo. */
      var tr = v.state.tr;
      if (replacing && v.state.selection instanceof PM.TextSelection && !v.state.selection.empty) { tr.deleteSelection(); }
      var place = insertPlace({ selection: tr.selection, schema: v.state.schema });
      tr = tr.replaceWith(place.from, place.to, nodes);
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
      if (!view || !on || sourceStale || frozen) return Promise.resolve(false);
      var target=view, doc=view.state.doc, selection=view.state.selection, ticket=loading;
      return Promise.resolve(o.blocksOf(markdown)).then(function (json) {
        if (view!==target || !on || sourceStale || frozen || loading!==ticket || view.state.doc!==doc || !view.state.selection.eq(selection)) {
          if(on)o.say('The document or selection changed before insertion. Try again.',true);
          return false;
        }
        insertBlocks((json || []).map(function (j) { return view.state.schema.nodeFromJSON(j); }), null, true, true);
        return true;
      }).catch(function(err){if(on)o.say(String(err && err.message || err),true);return false;});
    }

    /* Inserts words as they are, with no reading as Markdown (docs/editing-settings-contract.md, "Inserting text"): inline at the caret, replacing a selection, in one step of Undo, the
       caret after the words. A line break in them is a hard break and a blank line starts a new paragraph. */
    function insertPlain(text) {
      if (!view || !on || sourceStale || frozen || view.composing) return Promise.resolve(false);
      var sel = view.state.selection, s = view.state.schema;
      if (!(sel instanceof PM.TextSelection) || !sel.$from.parent.inlineContent || !sel.$to.parent.inlineContent) { o.say('The caret is not in text, so nothing can be inserted there.', true); return Promise.resolve(false); }
      var marks = sel.$from.marks();
      function inline(paragraph) {
        var nodes = [];
        paragraph.split('\n').forEach(function (line, i) { if (i) { nodes.push(s.nodes.hard_break.create()); } if (line) { nodes.push(s.text(line, marks)); } });
        return nodes;
      }
      var paragraphs = String(text).replace(/\r\n?/g, '\n').split(/\n{2,}/);
      var slice = paragraphs.length === 1 ? new PM.Slice(PM.Fragment.from(inline(paragraphs[0])), 0, 0)
        : new PM.Slice(PM.Fragment.from(paragraphs.map(function (p) { return s.nodes.paragraph.create(null, inline(p)); })), 1, 1);
      view.dispatch(view.state.tr.replaceSelection(slice).scrollIntoView());
      view.focus();
      return Promise.resolve(true);
    }
    /* A block drawn as the preview draws it; a click edits its source in
       place, and leaving the box draws it again. */
    function hiddenStyles(root) {
      return !!root.querySelector('.sc-style[hidden]') && Array.from(root.childNodes).every(function (node) {
        if (node.nodeType === 3) return !node.textContent.trim();
        return node.nodeType === 1 && (node.matches('.sc-style[hidden]') || node.matches('.doc-block') && hiddenStyles(node));
      });
    }
    function hiddenSource(node) {
      if (!node || node.type.name !== 'source_block') return false;
      if (node.attrs.kind === 'definition') return !(node.attrs.html || '').includes('sc-problem');
      return hiddenStyles(window.yavcHTML.fragment(node.attrs.html || ''));
    }
    function sourceView(node, v, getPos) {
      var dom = document.createElement('div');
      dom.className = 'pm-source';
      dom.contentEditable = 'false';
      var shownNode = null, box = null;
      function draw(nd) {
        shownNode = nd;
        dom.dataset.kind = nd.attrs.kind;
        dom.hidden = nd.attrs.kind === 'definition' && !(nd.attrs.html || '').includes('sc-problem');
        var inner = document.createElement('div');
        dom.replaceChildren(inner);
        o.draw(inner, nd.attrs.html || '<pre>' + (nd.attrs.src || '').replace(/&/g, '&amp;').replace(/</g, '&lt;') + '</pre>', nd.attrs.diagrams || []);
        window.yavcMdEditor.scopeFootnotes(inner,footnoteNamespace);
        // A valid stylesheet contributes no drawn block or selectable outline.
        // Keep the source node so unchanged Markdown still serializes verbatim.
        if (hiddenStyles(inner)) { dom.hidden = true; }
      }
      function edit() {
        if (box) return;
        box = document.createElement('textarea');
        box.className = 'form-textarea page-source';
        box.spellcheck = false;
        box.value = shownNode.attrs.src || '';
        box.setAttribute('aria-label', 'The Markdown of This Block');
        dom.replaceChildren(box);
        function fit() { box.style.height = 'auto'; box.style.height = box.scrollHeight + 2 + 'px'; }
        box.addEventListener('input', fit);
        box.addEventListener('keydown', function (e) {
          if (e.key === 'Escape' || (e.key === 'Enter' && (e.ctrlKey || e.metaKey))) { e.preventDefault(); done(); v.focus(); }
        });
        box.addEventListener('blur', done);
        fit();
        box.focus();
        o.say('');
      }
      function done() {
        if (!box) return;
        var value = box.value;
        box = null;
        take(value);
      }
      /* The block's new source, drawn again; the same source is only redrawn. */
      function take(value) {
        var was = shownNode.attrs.src || '';
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
        if (box || e.button !== 0 || e.target.closest('summary') || e.target.closest('a[href]') && e.ctrlKey) return;
        e.preventDefault();
        /* The host may take the block as a form, a picture's say; its
           answer is the new source, '' to take the block out, or null. */
        var original = shownNode, generation = loading;
        function activeSource() { var at = getPos(); return on && view === v && loading === generation && typeof at === 'number' && v.state.doc.nodeAt(at) && v.state.doc.nodeAt(at).eq(original); }
        Promise.resolve(sync()).then(function () {
          if (!activeSource()) return null;
          var asked = o.editBlock ? o.editBlock(original.attrs.src || '') : undefined;
          if (asked === undefined) { edit(); return null; }
          return asked;
        }).then(function (value) {
          if (!activeSource() || value == null) return;
          var pos = getPos();
          if (typeof pos !== 'number') return;
          if (value === '') { v.dispatch(v.state.tr.delete(pos, pos + shownNode.nodeSize)); v.focus(); return; }
          take(value);
          v.focus();
        }).catch(function (err) { if (on && view === v) o.say(String(err && err.message || err), true); });
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
        stopEvent: function (e) { return !!box && box.contains(e.target) || !!e.target.closest('summary'); },
        ignoreMutation: function () { return true; },
        selectNode: function () { dom.classList.add('selected'); },
        deselectNode: function () { dom.classList.remove('selected'); }
      };
    }

    /* A box: a container shortcode or a column set, its body edited as
       blocks inside the HTML the preview draws for it. A bar at its top,
       shown while the pointer or the keyboard is in it, changes its title,
       adds a column, lifts its content onto the page, or turns it back into
       Markdown (docs/page-view-design.md, decision 5). */
    function criticView(node){
      var current=node,dom=document.createElement('span');dom.className='pm-critic';dom.contentEditable='false';
      function draw(){dom.replaceChildren(window.yavcHTML.fragment(current.attrs.html));if(!o.review)return;var button=document.createElement('button');button.type='button';button.className='btn btn-sm pm-critic-open';button.setAttribute('aria-label','Review '+current.attrs.kind);button.title='Review '+current.attrs.kind;button.innerHTML='<span class="glyph" data-glyph="comments" aria-hidden="true"></span>';button.addEventListener('mousedown',function(event){event.preventDefault();});button.addEventListener('click',function(event){event.preventDefault();event.stopPropagation();o.review(current.attrs.src);});dom.appendChild(button);}
      draw();return {dom:dom,update:function(next){if(next.type.name!=='critic')return false;current=next;draw();return true;},ignoreMutation:function(){return true;},stopEvent:function(event){return !!event.target.closest('button');}};
    }
    var BOX_TITLED = { info: 'Title', note: 'Title', tip: 'Title', warning: 'Title', expand: 'Title', 'class': 'Class' };
    var BOX_NAMES = { info: 'Information', note: 'Note', tip: 'Tip', warning: 'Warning', expand: 'Expand', 'class': 'Style Class', 'no-export': 'Not Exported', 'draft-note': 'Note to Self', notes: 'Speaker Notes', columns: 'Columns' };
    function escText(s) { return String(s == null ? '' : s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'); }
    /* A container the host application draws is one whose frame is the
       engine's placeholder; its bar shows the host's title for it. */
    function hostBox(node) { return node.type.name !== 'columns' && /\sdata-host-shortcode="/.test(node.attrs.before || ''); }
    function boxTitle(node) { return hostBox(node) && o.hostTitle && o.hostTitle(node.attrs.name) || BOX_NAMES[node.attrs.name] || node.attrs.name; }
    /* Words inside a host's span shortcode, drawn as the host draws the
       span where its drawing is kept. */
    function hostSpanView(mark) {
      var drawn = o.hostSpan ? o.hostSpan(mark.attrs.name, mark.attrs.open) : null;
      if (drawn && drawn.dom && drawn.contentDOM) {
        if(o.editHostSpan){
          var button=document.createElement('button');button.type='button';button.className='btn btn-sm';button.textContent='…';button.contentEditable='false';button.setAttribute('aria-label','Change '+mark.attrs.name+' shortcode');
          drawn.dom.__hostControl=button;drawn.dom.appendChild(button);
          button.addEventListener('mousedown',function(e){e.preventDefault();});
          button.addEventListener('keydown',function(e){if(e.key==='Enter'||e.key===' '){e.preventDefault();e.stopPropagation();button.click();}});
          button.addEventListener('click',function(e){
            e.preventDefault();e.stopPropagation();if(!view||sourceStale||frozen)return;
            var target=view,doc=view.state.doc,ticket=loading,ranges=[];
            doc.descendants(function(node,pos){if(node.isText&&mark.isInSet(node.marks))ranges.push({from:pos,to:pos+node.nodeSize});});
            var at=view.posAtDOM(drawn.contentDOM,0),range=ranges.find(function(r){return r.from<=at&&r.to>=at;});if(!range)return;
            var start=range.from,end=range.to;for(var i=ranges.indexOf(range)-1;i>=0&&ranges[i].to===start;i--)start=ranges[i].from;for(var j=ranges.indexOf(range)+1;j<ranges.length&&ranges[j].from===end;j++)end=ranges[j].to;
            Promise.resolve(o.editHostSpan(mark.attrs.name,mark.attrs.open)).then(function(next){
              if(next==null||!on||view!==target||view.state.doc!==doc||loading!==ticket||sourceStale||frozen)return;
              var tr=view.state.tr.removeMark(start,end,mark);if(next)tr.addMark(start,end,mark.type.create({name:mark.attrs.name,open:next}));view.dispatch(tr);
            }).catch(function(err){if(o.say)o.say(String(err&&err.message||err),true);});
          });
        }
        return drawn;
      }
      var dom = document.createElement('span');
      dom.className = 'sc-host';
      dom.setAttribute('data-host-shortcode', mark.attrs.name); dom.setAttribute('data-host-kind', 'span'); dom.setAttribute('data-host-source', mark.attrs.open);
      return { dom: dom };
    }
    function boxView(node, v, getPos) {
      var isColumns = node.type.name === 'columns', current = node, contentDOM;
      var dom = document.createElement('div');
      dom.className = 'pm-box';
      dom.dataset.name = node.attrs.name;
      var bar = document.createElement('div');
      bar.className = 'pm-box-bar';
      bar.contentEditable = 'false';
      /* A box the host shows somewhere of its own (a deck's speaker notes,
         in the window's notes pane) has no presence on the page at all:
         it stays in the document, so the text is written back as it was,
         and it is neither drawn nor reachable by the caret here. */
      if (o.hiddenBox && o.hiddenBox(node.attrs.name)) { dom.hidden = true; dom.contentEditable = 'false'; dom.classList.add('pm-box-hidden'); }
      bar.innerHTML = '<span class="pm-box-kind">' + escText(boxTitle(node)) + '</span>' +
        (o.customName && o.customName(node.attrs.name) ? '<button type="button" class="btn btn-sm" data-box="custom-args">Parameters…</button>' : '') +
        (hostBox(node) ? '<button type="button" class="btn btn-sm" data-box="custom-args">Arguments…</button>' : BOX_TITLED[node.attrs.name] ? '<button type="button" class="btn btn-sm" data-box="args">' + escText(BOX_TITLED[node.attrs.name]) + '…</button>' : '') +
        (isColumns ? '<button type="button" class="btn btn-sm" data-box="column">Add a Column</button>' : '') +
        '<button type="button" class="btn btn-sm" data-box="lift">Remove the Box</button>' +
        '<button type="button" class="btn btn-sm" data-box="source">Edit as Markdown</button>';
      bar.addEventListener('mousedown', function (e) { e.preventDefault(); });
      bar.addEventListener('click', function (e) {
        var b = e.target.closest('[data-box]');
        if (b) boxAction(b.getAttribute('data-box'), current, getPos, v);
      });
      var frame;
      if (isColumns) {
        frame = document.createElement('div');
        frame.className = 'pm-box-frame sc-columns';
        contentDOM = frame;
      } else {
        frame = document.createElement('div');
        frame.className = 'pm-box-frame';
        frame.appendChild(window.yavcHTML.fragment((node.attrs.before || '<div>') + '<div class="pm-slot"></div>' + (node.attrs.after || '</div>')));
        contentDOM = document.createElement('div');
        contentDOM.className = 'pm-box-body';
        var slot = frame.querySelector('.pm-slot');
        if (slot) slot.replaceWith(contentDOM); else frame.appendChild(contentDOM);
        // Preserve the same editable body when a delayed host frame arrives.
        if (o.fill && hostBox(node)) o.fill(frame);
        // An expand section stays open while it is edited.
        frame.querySelectorAll('details').forEach(function (det) { det.open = true; });
        frame.querySelectorAll('summary').forEach(function (s) { s.addEventListener('click', function (e) { e.preventDefault(); }); });
      }
      dom.append(bar, frame);
      return {
        dom: dom, contentDOM: contentDOM,
        update: function (nd) {
          if (nd.type !== node.type || nd.attrs.before !== current.attrs.before || nd.attrs.after !== current.attrs.after) return false;
          current = nd;
          return true;
        },
        stopEvent: function (e) { return bar.contains(e.target); },
        ignoreMutation: function (m) { return m.type !== 'selection' && !contentDOM.contains(m.target); }
      };
    }
    /* A block moved to a new place: it loses where it was read from, so it
       is written where it lands, after a blank line, as a new block is; a
       block kept as its Markdown keeps that Markdown. */
    function looseCopy(b, n) {
      if (!('uid' in b.attrs)) return b;
      return b.type.create(Object.assign({}, b.attrs, { uid: null, gap: null, src: b.type === n.source_block ? b.attrs.src : null }), b.content, b.marks);
    }

    /* An inline shortcode on the page: drawn as the preview draws it; a
       press opens its form, whose answer replaces it or takes it out. */
    function inlineView(node, v, getPos) {
      var dom = document.createElement('span'), current = node;
      dom.className = 'pm-sc';
      dom.contentEditable = 'false';
      dom.title = 'Change It';
      function draw(nd) { dom.replaceChildren(window.yavcHTML.fragment(nd.attrs.html || nd.attrs.src)); if (o.fill) o.fill(dom); }
      draw(node);
      dom.addEventListener('mousedown', function (e) {
        if (e.button !== 0) return;
        e.preventDefault();
        var original = current, generation = loading;
        function activeAtom() { var at = getPos(); return on && view === v && loading === generation && typeof at === 'number' && v.state.doc.nodeAt(at) && v.state.doc.nodeAt(at).eq(original); }
        Promise.resolve(sync()).then(function () {
          if (!activeAtom()) return null;
          return o.editBlock ? o.editBlock(original.attrs.src) : null;
        }).then(function (value) {
          if (!activeAtom()) return;
          var pos = getPos();
          if (value == null || typeof pos !== 'number') { v.focus(); return; }
          if (value === '') { v.dispatch(v.state.tr.delete(pos, pos + 1)); v.focus(); return; }
          return Promise.resolve(o.inlineOf ? o.inlineOf(value) : { html: '' }).then(function (r) {
            var at = getPos();
            if (!activeAtom() || typeof at !== 'number') return;
            v.dispatch(v.state.tr.setNodeMarkup(at, null, { src: value, html: r && r.html || '' }));
            v.focus();
          });
        }).catch(function (err) { if (on && view === v) o.say(String(err && err.message || err), true); });
      });
      return {
        dom: dom,
        update: function (nd) { if (nd.type !== node.type) return false; if (nd.attrs.html !== current.attrs.html || nd.attrs.src !== current.attrs.src) draw(nd); current = nd; return true; },
        ignoreMutation: function () { return true; },
        selectNode: function () { dom.classList.add('selected'); },
        deselectNode: function () { dom.classList.remove('selected'); }
      };
    }
    /* An inline shortcode put in at the cursor, in place of the selection. */
    function insertInline(src) {
      if (!view || !on || sourceStale || frozen) return Promise.resolve(false);
      var target=view, doc=view.state.doc, selection=view.state.selection, ticket=loading;
      return Promise.resolve(o.inlineOf ? o.inlineOf(src) : { html: '' }).then(function (r) {
        if (view!==target || !on || sourceStale || frozen || loading!==ticket || view.state.doc!==doc || !view.state.selection.eq(selection)) return false;
        var node = view.state.schema.nodes.shortcode.create({ src: src, html: r && r.html || '' });
        view.dispatch(view.state.tr.replaceSelectionWith(node, true).scrollIntoView());
        view.focus(); return true;
      }).catch(function (err) { o.say(String(err && err.message || err), true); });
    }
    /* The selected blocks, or the block the cursor is in, wrapped in a box
       (a container shortcode, or a column set whose first column they
       become), within whatever body they are in. */
    function wrap(name) {
      if (!view || !o.wrapOf) return Promise.resolve();
      return Promise.resolve(o.wrapOf('{{' + name + '}}')).then(function (info) {
        if (!view) return;
        var st = view.state, n = st.schema.nodes, sel = st.selection;
        var range = sel.$from.blockRange(sel.$to, function (node) { return node.type === n.doc || node.type === n.container || node.type === n.column; });
        if (!range) { o.say('These blocks cannot go into a box together.', true); return; }
        var blocks = [];
        for (var i = range.startIndex; i < range.endIndex; i++) blocks.push(looseCopy(range.parent.child(i), n));
        var attrs = { name: info.name, args: info.args || '', open: '', close: '', before: info.before, after: info.after };
        var box = name === 'columns'
          ? n.columns.create(Object.assign(attrs, { sep: info.sep || 'column' }), [n.column.create(null, blocks), n.column.create(null, n.paragraph.create(null, st.schema.text(PLACEHOLDER)))])
          : n.container.create(attrs, blocks);
        var tr = st.tr.replaceWith(range.start, range.end, box);
        tr.setSelection(PM.TextSelection.near(tr.doc.resolve(range.start + (name === 'columns' ? 3 : 2))));
        view.dispatch(tr.scrollIntoView());
        view.focus();
      }).catch(function (err) { o.say(String(err && err.message || err), true); });
    }

    function boxAction(action, node, getPos, v) {
      var pos = getPos(), n = v.state.schema.nodes;
      if (typeof pos !== 'number') return;
      if (action === 'custom-args') {
        var generation = loading, sourceText = null;
        function currentBox() { var at = getPos(); return on && view === v && generation === loading && (sourceText === null || o.text() === sourceText) && typeof at === 'number' && v.state.doc.nodeAt(at) && v.state.doc.nodeAt(at).eq(node); }
        Promise.resolve(sync()).then(function () {
          if (!currentBox()) return null;
          sourceText = o.text();
          return o.markdownOf({ type: 'doc', attrs: { front: '', tail: '', crlf: false }, content: [node.toJSON()] });
        }).then(function (text) {
          if (text == null || !currentBox()) return null;
          return o.editBlock ? o.editBlock(text.replace(/^\n+|\n+$/g, '')) : null;
        }).then(function (source) {
          if (source == null || !currentBox()) return;
          var at = getPos(), now = typeof at === 'number' && view ? v.state.doc.nodeAt(at) : null;
          if (!now || !now.eq(node)) { o.say('The box changed while its parameters were open. Try again.', true); return; }
          if (source === '') { v.dispatch(v.state.tr.delete(at, at + now.nodeSize)); return; }
          return Promise.resolve(o.blocksOf(source)).then(function (json) {
            var here = getPos(), latest = typeof here === 'number' && view ? v.state.doc.nodeAt(here) : null;
            if (!currentBox() || !latest || !latest.eq(node)) return;
            v.dispatch(v.state.tr.replaceWith(here, here + latest.nodeSize, json.map(function (item) { return v.state.schema.nodeFromJSON(item); })).scrollIntoView());
            v.focus();
          });
        }).catch(function (err) { if (on && view === v) o.say(String(err && err.message || err), true); });
        return;
      }
      if (action === 'args') {
        var ask = window.yavcMdEditor && window.yavcMdEditor.askText;
        if (!ask) return;
        var openingDoc=v.state.doc, openingGeneration=loading;
        function sameBox() {var at=getPos();return on && view===v && !sourceStale && !frozen && loading===openingGeneration && v.state.doc===openingDoc && typeof at==='number' && v.state.doc.nodeAt(at) && v.state.doc.nodeAt(at).eq(node);}
        ask(boxTitle(node), BOX_TITLED[node.attrs.name] || 'Arguments', node.attrs.args,{signal:o.signal,okLabel:'Apply',validate:function () {return sameBox()?null:'The document or box changed. Close and reopen this form.';}}).then(function (args) {
          if (args == null || !o.wrapOf || !sameBox()) return;
          var line = args.trim() ? '{{' + node.attrs.name + ':' + args.trim() + '}}' : '{{' + node.attrs.name + '}}';
          return Promise.resolve(o.wrapOf(line)).then(function (info) {
            var at = getPos(), now = typeof at === 'number' ? v.state.doc.nodeAt(at) : null;
            if (!sameBox() || !now || !now.eq(node)) return;
            v.dispatch(v.state.tr.setNodeMarkup(at, null, Object.assign({}, now.attrs, { args: info.args, open: info.open, before: info.before, after: info.after })));
            v.focus();
          });
        }).catch(function (err) { o.say(String(err && err.message || err), true); });
        return;
      }
      if (action === 'column') {
        var col = n.column.create(null, n.paragraph.create());
        var tr = v.state.tr.insert(pos + node.nodeSize - 1, col);
        tr.setSelection(PM.TextSelection.create(tr.doc, pos + node.nodeSize - 1 + 2));
        v.dispatch(tr.scrollIntoView());
        v.focus();
        return;
      }
      if (action === 'lift') {
        /* The blocks lose their place in the box, so they are written where
           they land, after a blank line, as a new block is. */
        var blocks = [];
        if (node.type === n.columns) node.forEach(function (c) { c.forEach(function (b) { blocks.push(looseCopy(b, n)); }); });
        else node.forEach(function (b) { blocks.push(looseCopy(b, n)); });
        var lifted = v.state.tr.replaceWith(pos, pos + node.nodeSize, PM.Fragment.fromArray(blocks));
        lifted.setSelection(PM.TextSelection.near(lifted.doc.resolve(pos + 1)));
        v.dispatch(lifted.scrollIntoView());
        v.focus();
        return;
      }
      if (action === 'source') {
        Promise.resolve(o.markdownOf({ type: 'doc', attrs: { front: '', tail: '', crlf: false }, content: [node.toJSON()] })).then(function (text) {
          var src = String(text || '').replace(/^\n+|\n+$/g, '');
          var at = getPos(), now = typeof at === 'number' ? v.state.doc.nodeAt(at) : null;
          if (!now || now.type !== node.type) return;
          var block = n.source_block.create({ src: src, kind: 'shortcode', html: '', diagrams: null });
          v.dispatch(v.state.tr.replaceWith(at, at + now.nodeSize, block));
          return Promise.resolve(o.sourceOf(src)).then(function (r) {
            var here = v.state.doc.nodeAt(at);
            if (!here || here.type !== n.source_block || here.attrs.src !== src) return;
            v.dispatch(v.state.tr.setNodeMarkup(at, null, Object.assign({}, here.attrs, { html: r.html, diagrams: r.diagrams })).setMeta('addToHistory', false));
          });
        }).catch(function (err) { o.say(String(err && err.message || err), true); });
      }
    }

    /* A table's shape: rows and columns are added and deleted in any
       table. Merging and splitting cells, heading cells outside the first
       row and a cell's colour are what a Markdown table cannot hold, so a
       Markdown table changed that way becomes an HTML table, written as
       markup with no attributes of its own (DESIGN.md, decision 112), and
       is edited here as before. */
    var TABLE_COMMANDS = {
      'row-above': function () { return tableStructure(PM.addRowBefore, true); },
      'row-below': function () { return tableStructure(PM.addRowAfter, true); },
      'column-before': function () { return tableStructure(PM.addColumnBefore); },
      'column-after': function () { return tableStructure(PM.addColumnAfter); },
      'delete-row': function () { return tableStructure(PM.deleteRow); },
      'delete-column': function () { return tableStructure(PM.deleteColumn); },
      'delete-table': function () { return PM.deleteTable; },
      'merge-cells': function () { return reshape(PM.mergeCells); },
      'split-cell': function () { return reshape(PM.splitCell); },
      'heading-row': function () { return reshape(PM.toggleHeaderRow); },
      'heading-column': function () { return reshape(PM.toggleHeaderColumn); },
      'heading-cell': function () { return reshape(PM.toggleHeaderCell); },
      'cell-colour': function () { return reshape(colourCells); }
    };
    /* The attribute the host keeps equal to a cell's colour in a table of
       its own literal container (docs/host-shortcodes-contract.md), read
       from the container's opening line kept with the table; '' for a
       bare HTML table or a host that gives none. */
    function colourAttribute(table) {
      var raw = table && table.attrs.raw, m = raw && /^\{\{([a-z][a-z0-9-]*)/.exec(raw.open || '');
      var name = m && o.colourAttribute ? o.colourAttribute(m[1]) : '';
      return typeof name === 'string' && /^[a-z][a-z0-9:_-]{0,63}$/.test(name) && name !== 'style' && name !== 'colspan' && name !== 'rowspan' ? name : '';
    }
    /* The beginning of the class names a host uses for a cell's colour in
       a table of its own literal container, or '' (decision 117). */
    function colourClassPrefix(table) {
      var raw = table && table.attrs.raw, m = raw && /^\{\{([a-z][a-z0-9-]*)/.exec(raw.open || '');
      var prefix = m && o.colourClassPrefix ? o.colourClassPrefix(m[1]) : '';
      return typeof prefix === 'string' && /^[A-Za-z][A-Za-z0-9_-]{0,63}$/.test(prefix) ? prefix : '';
    }
    /* Colours the selected cells, or the cell the cursor is in, with the
       colour the page asked for (pendingColour), as a background colour in
       the cell's style; an empty colour takes it away. */
    var pendingColour = '';
    function colourCells(state, dispatch) {
      if (!PM.isInTable(state)) return false;
      var cells = [];
      if (state.selection instanceof PM.CellSelection) state.selection.forEachCell(function (node, pos) { cells.push({ node: node, pos: pos }); });
      else {
        var $c = state.selection.$from;
        for (var d = $c.depth; d > 0; d--) {
          var role = $c.node(d).type.spec.tableRole;
          if (role === 'cell' || role === 'header_cell') { cells.push({ node: $c.node(d), pos: $c.before(d) }); break; }
        }
      }
      if (!cells.length) return false;
      if (!dispatch) return true;
      /* The host may give an attribute of its own that records a cell's
         colour beside the style (a wiki's data-highlight-colour): it is
         written, changed and removed with the colour. */
      var found = PM.findTable(state.selection.$from), attribute = found ? colourAttribute(found.node) : '', prefix = found ? colourClassPrefix(found.node) : '';
      var tr = state.tr;
      cells.forEach(function (c) {
        var keep = (c.node.attrs.xattrs || []).filter(function (a) { return a[0] !== 'style' && a[0] !== attribute; });
        /* The host's classes that stand for a colour (a wiki's
           highlight-grey) go when the colour is changed or removed, and
           the class attribute goes when none of its classes is left.
           Nothing is added. */
        if (prefix) keep = keep.map(function (a) {
          if (a[0] !== 'class') return a;
          var left = a[1].split(/\s+/).filter(function (token) { return token && token.indexOf(prefix) !== 0; });
          return left.length ? ['class', left.join(' ')] : null;
        }).filter(Boolean);
        var style = (c.node.attrs.xattrs || []).find(function (a) { return a[0] === 'style'; });
        var rest = style ? style[1].split(';').map(function (s) { return s.trim(); }).filter(function (s) { return s && !/^background(-color)?\s*:/i.test(s); }).join('; ') : '';
        if (pendingColour) {
          if (attribute) keep.push([attribute, pendingColour]);
          keep.push(['style', 'background-color: ' + pendingColour + ';' + (rest ? ' ' + rest + ';' : '')]);
        } else if (rest) keep.push(['style', rest + ';']);
        tr.setNodeMarkup(c.pos, null, Object.assign({}, c.node.attrs, { xattrs: keep }));
      });
      dispatch(tr);
      return true;
    }
    /* A change only an HTML table can hold: the table becomes one, with
       every row drawn as written from then on. A Markdown table's empty
       heading row, which is how a table without headings is written, is
       taken out rather than shown. */
    /* The host's shortcode that records a cell's colour in a pipe table
       (DESIGN.md, decision 119): { name, example } or null. */
    function cellShortcodeEntry() {
      var e = o.cellColourShortcode ? o.cellColourShortcode() : null;
      return e && typeof e.name === 'string' && /^[a-z][a-z0-9-]{0,63}$/.test(e.name) ? e : null;
    }
    function isCellShortcode(node, name) {
      return !!node && node.type.name === 'shortcode' && new RegExp('^\\{\\{' + name + '(?::[^{}\\r\\n]*)?\\}\\}$').test(node.attrs.src || '');
    }
    /* The cells the selection covers, as { node, pos }. */
    function selectedCells(state) {
      var cells = [];
      if (state.selection instanceof PM.CellSelection) state.selection.forEachCell(function (node, pos) { cells.push({ node: node, pos: pos }); });
      else {
        var $c = state.selection.$from;
        for (var d = $c.depth; d > 0; d--) {
          var role = $c.node(d).type.spec.tableRole;
          if (role === 'cell' || role === 'header_cell') { cells.push({ node: $c.node(d), pos: $c.before(d) }); break; }
        }
      }
      return cells;
    }
    /* The colour shortcode of the cell the caret is in, as it is written, or ''. */
    function cellShortcodeSource() {
      var entry = cellShortcodeEntry();
      if (!entry || !view || !PM.isInTable(view.state)) return '';
      var cells = selectedCells(view.state), first = cells.length ? cells[0].node.firstChild : null;
      return isCellShortcode(first, entry.name) ? first.attrs.src : '';
    }
    /* Writes the host's colour shortcode first in each selected cell of a
       pipe table, in place of one already there; an empty source takes it
       away. html is the engine's drawing of the shortcode. The table
       stays a pipe table. */
    function setCellShortcode(source, html) {
      var entry = cellShortcodeEntry();
      if (!entry || !view || !on || sourceStale || frozen || !PM.isInTable(view.state)) return false;
      var state = view.state, found = PM.findTable(state.selection.$from);
      if (!found || found.node.attrs.raw) return false;
      var cells = selectedCells(state), tr = state.tr, type = state.schema.nodes.shortcode;
      // From the last cell to the first, so earlier positions stay true.
      cells.sort(function (a, b) { return b.pos - a.pos; }).forEach(function (c) {
        var start = c.pos + 1, first = c.node.firstChild;
        if (isCellShortcode(first, entry.name)) {
          var end = start + first.nodeSize, next = c.node.childCount > 1 ? c.node.child(1) : null;
          // The space written after the shortcode goes with it.
          if (!source && next && next.isText && next.text.charAt(0) === ' ') end += 1;
          if (source) tr.replaceWith(start, end, type.create({ src: source, html: html || '' }));
          else tr.delete(start, end);
        } else if (source) {
          var nodes = [type.create({ src: source, html: html || '' })];
          if (c.node.content.size) nodes.push(state.schema.text(' '));
          tr.insert(start, nodes);
        }
      });
      if (!tr.docChanged) return false;
      view.dispatch(tr.scrollIntoView());
      view.focus();
      return true;
    }
    function reshape(cmd) {
      return function (state, dispatch) {
        var tr = null;
        if (!cmd(state, function (t) { tr = t; })) return false;
        if (!dispatch) return true;
        var found = PM.findTable(tr.selection.$from);
        /* A pipe table whose cells have the host's colour shortcode cannot
           become an HTML table, in whose cells a shortcode cannot be written: the change
           is refused until the colours are taken away. */
        var entry = cellShortcodeEntry(), coloured = false;
        if (found && !found.node.attrs.raw && entry) found.node.descendants(function (n) { if (isCellShortcode(n, entry.name)) coloured = true; });
        if (coloured) { o.say('This change would turn the table into an HTML table, which cannot keep its cell colors. Remove the cell colors first.', true); return false; }
        if (found && !found.node.attrs.raw) {
          tr.setNodeMarkup(found.pos, null, Object.assign({}, found.node.attrs, { raw: { open: '', close: '', attrs: [], colgroup: '', thead: false, tbody: true } }));
          var first = found.node.firstChild, emptyHead = !!first;
          if (first) first.forEach(function (cell) { if (cell.type.name !== 'table_header' || cell.content.size) emptyHead = false; });
          found.node.forEach(function (row, offset) { tr.setNodeMarkup(found.start + offset, null, Object.assign({}, row.attrs, { xattrs: row.attrs.xattrs || [] })); });
          if (emptyHead && found.node.childCount > 1) tr.delete(found.start, found.start + first.nodeSize);
          o.say('This table now has a shape Markdown cannot hold, so it is kept as HTML in the text. It is edited here as before.');
        }
        dispatch(tr);
        return true;
      };
    }
    function tableStructure(command, retainColumns) {
      return function (state, dispatch) {
        if (!PM.isInTable(state)) return false;
        var rect = PM.selectedRect(state), alignment = [];
        /* An HTML table's cells keep their own attributes; only a
           Markdown table's column alignment is kept across its rows. */
        if (rect.table.attrs.raw) return command(state, dispatch);
        rect.table.firstChild.forEach(function (cell) { alignment.push(cell.attrs.align || ''); });
        return command(state, dispatch && function (tr) {
          var at = tr.mapping.map(rect.tableStart - 1), table = tr.doc.nodeAt(at);
          if (table && table.type.spec.tableRole === 'table') {
            if (!retainColumns) { alignment = []; table.firstChild.forEach(function (cell) { alignment.push(cell.attrs.align || ''); }); }
            table.forEach(function (row, rowOffset) { row.forEach(function (cell, cellOffset, column) {
              var value = alignment[column] || '';
              if (cell.attrs.align !== value) tr.setNodeMarkup(at + 2 + rowOffset + cellOffset, null, Object.assign({}, cell.attrs, { align: value }));
            }); });
          }
          dispatch(tr);
        });
      };
    }
    function columnAlignment(value) {
      return function (state, dispatch) {
        if (!PM.isInTable(state)) return false;
        var rect = PM.selectedRect(state), valid = !rect.table.attrs.raw;
        rect.table.forEach(function (row) { if (row.childCount !== rect.map.width) valid = false; row.forEach(function (cell) { if (cell.attrs.colspan !== 1 || cell.attrs.rowspan !== 1) valid = false; }); });
        if (!valid) return false;
        if (dispatch) {
          var tr = state.tr;
          rect.table.forEach(function (row, rowOffset) { row.forEach(function (cell, cellOffset, column) { if (column >= rect.left && column < rect.right && cell.attrs.align !== value) tr.setNodeMarkup(rect.tableStart + rowOffset + 1 + cellOffset, null, Object.assign({}, cell.attrs, { align: value })); }); });
          if (tr.docChanged) dispatch(tr.scrollIntoView());
        }
        return true;
      };
    }
    ['', 'left', 'center', 'right'].forEach(function (value) { TABLE_COMMANDS['align-' + (value || 'default')] = function () { return columnAlignment(value); }; });
    function can(name) {
      if (!view || sourceStale) return false;
      var make = TABLE_COMMANDS[name];
      return !!make && PM.isInTable(view.state) && make()(view.state);
    }

    /* The Editor's commands, on the page. Each returns true when it acted. */
    function run(name, arg) {
      if (!view || sourceStale) return false;
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
        case 'highlight': cmd = PM.toggleMark(s.marks.mark); break;
        case 'superscript': cmd = PM.toggleMark(s.marks.sup); break;
        case 'subscript': cmd = PM.toggleMark(s.marks.sub); break;
        case 'strike': cmd = PM.toggleMark(s.marks.strike); break;
        case 'h1': case 'h2': case 'h3': case 'h4': case 'h5': case 'h6':
          /* A heading that changes level keeps its anchor and classes. */
          var $at = view.state.selection.$from, here = $at.sameParent(view.state.selection.$to) && $at.parent.type === n.heading ? $at.parent.attrs.attributes : null;
          cmd = PM.setBlockType(n.heading, { level: +name.charAt(1), attributes: here }); break;
        case 'paragraph': cmd = PM.setBlockType(n.paragraph); break;
        case 'quote':
          cmd = function (state, dispatch) {
            var $from = state.selection.$from;
            for (var depth = $from.depth; depth > 0; depth--) {
              if ($from.node(depth).type !== n.blockquote) continue;
              var node = $from.node(depth), at = $from.before(depth), parent = $from.node(depth - 1), index = $from.index(depth - 1);
              if (state.selection.to > at + node.nodeSize - 1 || !parent.canReplace(index, index + 1, node.content)) return false;
              if (dispatch) dispatch(state.tr.replaceWith(at, at + node.nodeSize, node.content).scrollIntoView());
              return true;
            }
            return PM.wrapIn(n.blockquote)(state, dispatch);
          }; break;
        case 'bullets': cmd = list(n.bullet_list); break;
        case 'numbers': cmd = list(n.ordered_list); break;
        case 'link': linkDialog(); return true;
        case 'color': case 'class':
          /* With a value, the selection takes that colour or class (or the
             next words typed, when nothing is selected); without, it loses
             it. */
          cmd = function (state, dispatch) {
            var sel = state.selection, type = s.marks[name];
            if (!arg) {
              if (dispatch) dispatch(sel.empty ? state.tr.removeStoredMark(type) : state.tr.removeMark(sel.from, sel.to, type).scrollIntoView());
              return true;
            }
            var mark = type.create({ value: String(arg) });
            if (dispatch) dispatch(sel.empty ? state.tr.removeStoredMark(type).addStoredMark(mark) : state.tr.removeMark(sel.from, sel.to, type).addMark(sel.from, sel.to, mark).scrollIntoView());
            return true;
          };
          break;
        default:
          if (name === 'cell-colour') pendingColour = typeof arg === 'string' ? arg : '';
          if (TABLE_COMMANDS[name]) { cmd = TABLE_COMMANDS[name](); break; }
          if (!insertOf(name)) return false;
          insert(name);
          return true;
      }
      var acted = cmd(v.state, v.dispatch, v);
      v.focus();
      return acted;
    }
    /* The link for the selection, asked in the site's dialog
       (static/dialogs.js): an address, or none to take the link away. */
    function linkDialog() {
      var v = view, s = v.state, mark = s.schema.marks.link, current = '', found = [];
      s.doc.nodesBetween(s.selection.from, Math.max(s.selection.to, s.selection.from + 1), function (node) {
        var m = mark.isInSet(node.marks);
        if (m) { current = m.attrs.href; if (found.indexOf(m.attrs.attributes) < 0) found.push(m.attrs.attributes); }
      });
      /* A link given another address keeps its attribute block, when the
         selection held one link's worth of them. */
      var kept = found.length === 1 ? found[0] : null;
      var from = s.selection.from, to = s.selection.to;
      function unchanged() { return on && view === v && !sourceStale && !frozen && v.state.doc === s.doc && v.state.selection.eq(s.selection); }
      window.yavcDialog.askText('Link', 'Address', current, { signal: o.signal, placeholder: 'https://', help: 'Leave the address empty to take the link away.', okLabel: 'Apply', validate: function () { return unchanged() ? null : 'The document or selection changed. Reopen this form.'; } }).then(function (address) {
        if (!unchanged()) return;
        v.focus();
        if (address == null) return;
        var tr = v.state.tr;
        if (from === to) {
          if (!address) return;
          tr.insertText(address, from).addMark(from, from + address.length, mark.create({ href: address }));
        } else {
          tr.removeMark(from, to, mark);
          if (address) tr.addMark(from, to, mark.create({ href: address, attributes: kept }));
        }
        v.dispatch(tr);
      });
    }

    // A link on the page is followed with Ctrl+click; a plain click places the caret.
    function click(e) {
      if (!on) return;
      var reference=e.target.closest&&e.target.closest('.doc-footnote-ref[data-footnote-ref]');
      if(reference&&o.page.contains(reference)){
        e.preventDefault();e.stopPropagation();var label=reference.getAttribute('data-footnote-ref'),found=null;
        view.state.doc.descendants(function(node,pos){if(found==null&&node.type.name==='footnote'&&node.attrs.label===label)found=pos;});
        if(found!=null){view.dispatch(view.state.tr.setSelection(PM.TextSelection.near(view.state.doc.resolve(found+1))).scrollIntoView());view.focus();}
        else {var definition=Array.from(o.page.querySelectorAll('.doc-footnote')).find(function(note){return note.getAttribute('data-footnote')===label;});if(definition)definition.scrollIntoView({block:'nearest'});}
        return;
      }
      var link = e.target.closest && e.target.closest('a[href]');
      if (link && !e.ctrlKey) { e.preventDefault(); e.stopPropagation(); }
    }
    o.scroll.addEventListener('click', click, true);
    function staleInput(event) { if (sourceStale) { event.preventDefault(); event.stopPropagation(); } }
    o.page.addEventListener('mousedown', staleInput, true);
    o.page.addEventListener('beforeinput', staleInput, true);

    /* ProseMirror takes the browser's selection into its state when the browser's selectionchange event comes, which is a task of its own
       and can be several frames after the selection was made when the page is busy. A command that reads the page's selection in that
       time acts on the one before. So each entry of the API that reads or changes the selection first reads the browser's own selection
       (view.root.getSelection and posAtDOM, both documented) and, when it differs from the state's, makes it the state's. Only a text
       selection outside a table is replaced (Firefox moves the browser's selection out of a table cell when the table is drawn again, and the state's is then the right one), and only while the page has the focus, which is where ProseMirror would take the browser's selection too. */
    var seenSelection = null;
    function domSelection() {
      var dom = view && view.root && view.root.getSelection ? view.root.getSelection() : null;
      return dom && dom.anchorNode ? { anchorNode: dom.anchorNode, anchorOffset: dom.anchorOffset, focusNode: dom.focusNode, focusOffset: dom.focusOffset } : null;
    }
    function sameDomSelection(a, b) {
      return !!a && !!b && a.anchorNode === b.anchorNode && a.anchorOffset === b.anchorOffset && a.focusNode === b.focusNode && a.focusOffset === b.focusOffset;
    }
    function syncSelection() {
      if (!view || !on || view.composing || !view.hasFocus()) return;
      var current = view.state.selection;
      if (!(current instanceof PM.TextSelection) || PM.isInTable(view.state)) return;
      var dom = domSelection();
      if (!dom || !view.dom.contains(dom.anchorNode) || !view.dom.contains(dom.focusNode)) return;
      /* The state's selection is the page's own unless the browser's has changed since the state last took or set it: a selection the page set
         itself may not be in the browser's yet, and the browser's then says nothing new. */
      if (sameDomSelection(dom, seenSelection)) return;
      var anchor, head;
      try { anchor = view.posAtDOM(dom.anchorNode, dom.anchorOffset); head = view.posAtDOM(dom.focusNode, dom.focusOffset); } catch (err) { return; }
      if (anchor === current.anchor && head === current.head) return;
      var doc = view.state.doc;
      if (anchor < 0 || head < 0 || anchor > doc.content.size || head > doc.content.size) return;
      var next = PM.TextSelection.between(doc.resolve(anchor), doc.resolve(head));
      if (!next.eq(current)) view.dispatch(view.state.tr.setSelection(next));
    }
    async function mirrorAt(from, to) {
      if (!view || !on || sourceStale || frozen || dirty || syncing || view.composing) return false;
      var target = view, doc = view.state.doc, text = o.text();
      if (typeof from !== 'number' || typeof to !== 'number' || from < 0 || to < from || to > text.length) return false;
      var bytes = new Uint8Array(8); crypto.getRandomValues(bytes);
      var tag = Array.from(bytes, function (value) { return value.toString(16).padStart(2, '0'); }).join(''), begin = 'YavcMirrorBegin' + tag, end = 'YavcMirrorEnd' + tag;
      if (text.includes('YavcMirror')) return false;
      var marked = text.slice(0, from) + begin + text.slice(from, to) + (to > from ? end : '') + text.slice(to);
      try {
        var json = await o.docOf(marked);
        if (view !== target || !on || sourceStale || frozen || dirty || view.state.doc !== doc || o.text() !== text) return false;
        var made = view.state.schema.nodeFromJSON(json), first = -1, last = -1, firstCount = 0, lastCount = 0;
        made.descendants(function (node, pos) {
          if (!node.isText) return;
          var i = node.text.indexOf(begin), j = node.text.indexOf(end);
          if (i >= 0) { first = pos + i; firstCount++; }
          if (j >= 0) { last = pos + j; lastCount++; }
        });
        if (firstCount !== 1 || (to > from && lastCount !== 1)) return false;
        var removal = PM.EditorState.create({ doc: made }).tr;
        if (to > from) removal.delete(last, last + end.length);
        removal.delete(first, first + begin.length);
        /* The two pages are the same page: the same blocks, marks and words. A block's own source and number are left out, since the source of
           the block that holds a marker has the marker in it. */
        function shape(node) {
          if (node.isText) return JSON.stringify([node.text, node.marks.map(function (mark) { return [mark.type.name, mark.attrs]; })]);
          var attrs = Object.assign({}, node.attrs); ['uid', 'gap', 'src', 'canonical', 'html'].forEach(function (name) { delete attrs[name]; });
          var parts = []; node.forEach(function (child) { parts.push(shape(child)); });
          return '[' + node.type.name + JSON.stringify(attrs) + parts.join(',') + ']';
        }
        if (shape(removal.doc) !== shape(doc)) return false;
        var anchor = first, head = to > from ? last - begin.length : first;
        var chosen = PM.TextSelection.between(doc.resolve(anchor), doc.resolve(head));
        mirrored = { from: chosen.from, to: chosen.to, doc: doc };
        view.dispatch(view.state.tr.setSelection(chosen).setMeta('mirror', true).setMeta('addToHistory', false));
        api.scrollToSelection();
        return true;
      } catch (err) { return false; }
    }
    var api = {
      enable: enable,
      /* The place of the page's caret in the text, for keeping the
         reader's place when the page is left for Text: the spot itself
         where the text is exactly the page (sourceSelection), or else
         the start of the block the caret is in. The page is written to
         the text first. null when there is no page. */
      /* Brings the page's selection into view once the page has its
         layout: when the page is turned on it is built before it is
         shown, so a scroll asked for then lands on no layout (decision
         113). A second look follows, for a page whose sheets are laid
         out after it is shown. */
      scrollToSelection: function () {
        if (!view || !on) return;
        var target = view, sel = view.state.selection;
        function bring() {
          if (view !== target || !on || !view.state.selection.eq(sel)) return;
          try {
            var c = view.coordsAtPos(view.state.selection.from), r = o.scroll.getBoundingClientRect();
            if (c.top < r.top || c.bottom > r.bottom) o.scroll.scrollTop += c.top - r.top - r.height / 2;
          } catch (err) { }
        }
        requestAnimationFrame(bring);
        setTimeout(bring, 300);
      },
      placeInText: async function () {
        if (!view || !on) return null;
        var target = view;
        try { await sync(); } catch (err) { return null; }
        if (view !== target || !on) return null;
        var exact = await this.sourceSelection();
        if (exact) return { from: exact.from, to: exact.to };
        if (view !== target || !on) return null;
        var $from = view.state.selection.$from, index = $from.depth > 0 ? $from.index(0) : Math.min($from.index(0), view.state.doc.childCount - 1);
        var text = o.text();
        try {
          var json = await o.docOf(text);
          if (view !== target || !on || o.text() !== text) return null;
          var at = blockOffset(json, index);
          return at == null ? null : { from: at, to: at };
        } catch (err) { return null; }
      },
      /* Whether anything on the page is selected, said at once. */
      /* Shows the text's selection, from and to as offsets in the text, on the page without taking the focus or changing the document: the
         marker is put into the text at each end, the text is read as a page, and the markers' places in it are the places in the page the
         selection is at. It is done only when the page is exactly the text (nothing typed since it was written), the page with the markers
         taken out is the page that is shown, and each marker is found once; otherwise nothing is changed and false is given, so no other place
         stands in for the one asked for. */
      selectAtText: async function (from, to) {
        var text = view && o.text();
        if (typeof from !== 'number' || typeof to !== 'number' || typeof text !== 'string' || from < 0 || to < from || to > text.length) return false;
        /* The place itself first. A caret inside a link's address or an emphasis mark cannot be shown as a place on the page, since a marker there
           changes the page; then the start of the line it is on is the place, which is in the same paragraph, and a selection that cannot be shown
           as it is becomes a caret there. Nothing else stands in for it. */
        var lineStart = text.lastIndexOf(String.fromCharCode(10), Math.max(0, from) - 1) + 1, places = [[from, to]];
        if (lineStart !== from || to > from) places.push([lineStart, lineStart]);
        for (var i = 0; i < places.length; i++) { if (await mirrorAt(places[i][0], places[i][1])) return true; }
        return false;
      },
      hasSelection: function () { return !!(view && on && !view.state.selection.empty); },
      // Preserve a failed rich draft for selection/copy after its host revokes editing.
      freeze: function () { frozen = true; if (view) view.setProps({ editable: function () { return false; } }); },
      active: function () { return on && !!view; },
      insertionTarget: function () {
        if (!view || !on || sourceStale || frozen) return function () { return false; };
        var target=view,doc=view.state.doc,selection=view.state.selection,ticket=loading;
        return function () { return view===target && on && !sourceStale && !frozen && loading===ticket && view.state.doc===doc && view.state.selection.eq(selection); };
      },
      // Changes on the page not yet in the Editor's text.
      pending: function () { return dirty || !!syncing; },
      flush: flush,
      formatSelection: function (name) {
        if (!view || !on || sourceStale || frozen) return null;
        var target = view, doc = view.state.doc, selection = view.state.selection, stored = view.state.storedMarks, type = view.state.schema.marks[name];
        if (!type) return null;
        var values = new Set();
        function value(marks) { var mark = type.isInSet(marks); return mark ? String(mark.attrs.value || '') : ''; }
        if (selection.empty) values.add(value(view.state.storedMarks || selection.$from.marks()));
        else doc.nodesBetween(selection.from, selection.to, function (node) { if (node.isText) values.add(value(node.marks)); });
        if (!values.size) return null;
        function current() { return on && view === target && view.editable && !sourceStale && !frozen && view.state.doc === doc && view.state.selection.eq(selection) && view.state.storedMarks === stored; }
        return { value: values.size === 1 ? Array.from(values)[0] : '', mixed: values.size > 1, empty: selection.empty,
          text: selection.empty ? 'Sample text' : doc.textBetween(selection.from, selection.to, ' ').slice(0, 240), current: current,
          apply: function (next) { return current() && run(name, next || null); } };
      },
      historyAvailable: function (name) { return !!(view && !sourceStale && PM[name](view.state)); },
      clipboardSelection: function () {
        if (!view || sourceStale) return null;
        var target = view, doc = view.state.doc, selection = view.state.selection;
        var data = view.serializeForClipboard(selection.content());
        function current() { return on && view === target && view.editable && !sourceStale && !frozen && view.state.doc === doc && view.state.selection.eq(selection); }
        return { text: data.text, html: data.dom.innerHTML, empty: selection.empty, current: current,
          remove: function () { if (current()) view.dispatch(view.state.tr.deleteSelection().scrollIntoView().setMeta('uiEvent', 'cut')); } };
      },
      sync: sync,
      prepareEdit: async function () {
        await sync();
        if(on&&sourceStale){clearTimeout(reloadTimer);await load();}
        return !!(view&&on&&!sourceStale&&!frozen&&view.editable);
      },
      // Map a rich selection by serializing a temporary public transaction.
      // Removing its unique sentinels must reproduce the current source byte
      // for byte before these offsets can authorize a source edit.
      sourceSelection: async function(){
        if(!view||!on||sourceStale||frozen)return null;
        var target=view,doc=view.state.doc,selection=view.state.selection,source=o.text(),bytes=new Uint8Array(16);crypto.getRandomValues(bytes);
        var nonce=Array.from(bytes,function(value){return value.toString(16).padStart(2,'0');}).join(''),begin='YavcReviewBegin'+nonce,end='YavcReviewEnd'+nonce;
        if(source.includes(begin)||source.includes(end))return null;
        function current(){return view===target&&on&&!sourceStale&&!frozen&&view.state.doc===doc&&view.state.selection.eq(selection)&&o.text()===source;}
        try{
          var temporary=view.state.tr.insertText(end,selection.to).insertText(begin,selection.from),marked=await o.markdownOf(temporary.doc.toJSON());
          if(!current())return null;
          var from=marked.indexOf(begin),to=marked.indexOf(end);
          if(from<0||to<from+begin.length||marked.indexOf(begin,from+begin.length)>=0||marked.indexOf(end,to+end.length)>=0)return null;
          var reconstructed=marked.slice(0,from)+marked.slice(from+begin.length,to)+marked.slice(to+end.length);
          if(reconstructed!==source)return null;
          return {from:from,to:to-begin.length,current:current};
        }catch(err){return null;}
      },
      footnoteSelection: function () {
        if (!view || !on || sourceStale || frozen || !view.editable || !view.state.selection.$to.parent.isTextblock) return null;
        var target=view,doc=view.state.doc,selection=view.state.selection;
        function current(){return on&&view===target&&view.editable&&!sourceStale&&!frozen&&view.state.doc===doc&&view.state.selection.eq(selection);}
        return {current:current,apply:function(text){
          if(!current()||typeof text!=='string'||!text.trim()||text.length>16384)return false;
          var schema=view.state.schema,used=new Set(),number=0;
          doc.descendants(function(node){if(node.type.name==='footnote'||node.type.name==='footnote_ref'){used.add(node.attrs.label);number=Math.max(number,node.attrs.number||0);}});
          if(used.size>=256)return false;
          var index=1;while(used.has('note-'+index))index++;
          var label='note-'+index,paragraphs=text.split(/\n\s*\n/).map(function(part){var content=[];part.split('\n').forEach(function(line,i){if(i)content.push(schema.nodes.soft_break.create());if(line)content.push(schema.text(line));});return schema.nodes.paragraph.create(null,content);});
          var note=schema.nodes.footnote.create({label:label,number:number+1},paragraphs),ref=schema.nodes.footnote_ref.create({label:label,number:number+1});
          var tr=view.state.tr.insert(selection.to,ref);tr.insert(tr.doc.content.size,note);tr.setSelection(PM.TextSelection.near(tr.doc.resolve(selection.to+1)));
          view.dispatch(tr.scrollIntoView());view.focus();return true;
        }};
      },
      textChanged: textChanged,
      reload: function () { return on ? load() : Promise.resolve(); },
      // The draft's suggestions, to show on the page with Accept and Reject.
      suggest: suggest,
      goToSuggestion:function(id){if(!view||!on)return false;var node=Array.from(view.dom.querySelectorAll('[data-host-suggestion]')).find(function(element){return element.dataset.hostSuggestion===id;});if(!node)return false;node.scrollIntoView({block:'center'});var button=Array.from(node.querySelectorAll('button')).find(function(element){return !element.disabled;});if(button)button.focus({preventScroll:true});return true;},
      // The Edit menu's Paste and Select All, on the page.
      paste: pasteContent, selectAll: selectAll,
      focus: function () { if (view) view.focus(); },
      // The page's editable element, for the Edit menu's Cut and Copy.
      repaginate: function () { if (view) { view.dispatch(view.state.tr.setMeta('addToHistory', false)); } },
      element: function () { return view ? view.dom : null; },
      // The Editor's find bar, on the page.
      find: find, goToMatch: goToMatch, matchText: matchText, replaceMatch: replaceMatch, replaceAllMatches: replaceAllMatches, selectionText: selectionText,
      run: run,
      insertMarkdown: insertMarkdown,
      insertInline: insertInline,
      insertPlain: insertPlain,
      /* A shortcode from the Insert menu: an inline one at the cursor, and
         any other as blocks where a new block goes. */
      insertShortcode: function (kind, src) { return kind === 'inline' ? insertInline(src) : insertMarkdown(src); },
      wrap: wrap,
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
      /* The host's cell colour shortcode for the pipe table the caret is
         in: { name, example, source }, source being the one in the
         caret's cell or ''; null in an HTML table or with no such
         shortcode. set(source) writes it to the selected cells, drawn as
         the engine draws it, and resolves to whether it did. */
      cellColourShortcode: function () {
        var entry = cellShortcodeEntry();
        if (!entry || !view || !on || sourceStale || frozen || !PM.isInTable(view.state)) return null;
        var found = PM.findTable(view.state.selection.$from);
        if (!found || found.node.attrs.raw) return null;
        var target = view, doc = view.state.doc, selection = view.state.selection, ticket = loading;
        function current() { return view === target && on && !sourceStale && !frozen && loading === ticket && view.state.doc === doc && view.state.selection.eq(selection); }
        return { name: entry.name, example: typeof entry.example === 'string' ? entry.example : '', source: cellShortcodeSource(), current: current,
          set: function (source) {
            if (!current()) return Promise.resolve(false);
            if (!source) return Promise.resolve(setCellShortcode('', ''));
            if (!isCellShortcode({ type: { name: 'shortcode' }, attrs: { src: source } }, entry.name)) return Promise.resolve(false);
            return Promise.resolve(o.hostInline ? o.hostInline(source) : null).then(function (r) { return current() ? setCellShortcode(source, r && r.html || '') : false; });
          } };
      },
      outlineIndex: function () {
        if (!view) return -1;
        var child = view.state.selection.$from.index(0), index = -1;
        view.state.doc.forEach(function (node, offset, i) { if (i <= child && (node.type.name === 'heading' || node.type.name === 'source_block' && node.attrs.kind === 'heading')) index++; });
        return index;
      },
      goToHeading: function (index) {
        if (!view) return false;
        var found = null, number = -1;
        view.state.doc.forEach(function (node, offset) { if (node.type.name === 'heading' || node.type.name === 'source_block' && node.attrs.kind === 'heading') { number++; if (number === index) found = { node: node, offset: offset }; } });
        if (!found) return false;
        var selection = found.node.type.name === 'heading' ? PM.TextSelection.create(view.state.doc, found.offset + 1) : PM.NodeSelection.create(view.state.doc, found.offset);
        view.dispatch(view.state.tr.setSelection(selection).scrollIntoView()); view.focus(); return true;
      },
      // Whether a table command can act where the cursor is.
      can: can,
      // What the cursor is in, for the toolbar's pressed states.
      marks: function () {
        if (!view) return {};
        var st = view.state, s = st.schema, out = {}, $f = st.selection.$from;
        ['strong', 'em', 'code', 'underline', 'strike', 'link', 'color', 'class', 'mark', 'sup', 'sub'].forEach(function (m) {
          var type = s.marks[m];
          out[m] = st.selection.empty ? !!type.isInSet(st.storedMarks || $f.marks()) : st.doc.rangeHasMark(st.selection.from, st.selection.to, type);
        });
        var block = $f.parent, list = null;
        for (var d = $f.depth; d > 0; d--) { var t = $f.node(d).type.name; if (t === 'bullet_list' || t === 'ordered_list') { list = t; break; } }
        out.block = block.type.name === 'heading' ? 'h' + block.attrs.level : block.type.name === 'paragraph' ? 'paragraph' : block.type.name;
        out.list = list;
        out.table = PM.isInTable(st);
        out.quote = false; for (var depth = $f.depth; depth > 0; depth--) { if ($f.node(depth).type === s.nodes.blockquote) { out.quote = true; break; } }
        if (out.table) {
          var rect = PM.selectedRect(st), values = new Set();
          rect.table.forEach(function (row) { row.forEach(function (cell, offset, column) { if (column >= rect.left && column < rect.right) values.add(cell.attrs.align || ''); }); });
          out.columnAlign = values.size === 1 ? values.values().next().value : null;
          out.rawTable = !!rect.table.attrs.raw;
          /* The background colour of the cell the cursor is in, for the
             Cell Colour dialog to start from. */
          for (var depth = $f.depth; depth > 0; depth--) {
            var role = $f.node(depth).type.spec.tableRole;
            if (role !== 'cell' && role !== 'header_cell') continue;
            var attrs = $f.node(depth).attrs.xattrs || [], attribute = colourAttribute(rect.table), own = attribute && attrs.find(function (a) { return a[0] === attribute; });
            var style = attrs.find(function (a) { return a[0] === 'style'; }), m = style && /(?:^|;)\s*background(?:-color)?\s*:\s*([^;]+)/i.exec(style[1]);
            out.cellColour = own ? own[1] : m ? m[1].trim() : '';
            /* In a pipe table the colour is the host's shortcode's argument. */
            var shortcode = !out.rawTable ? cellShortcodeSource() : '';
            if (shortcode) { var arg = /^\{\{[a-z][a-z0-9-]*:([^{}]*)\}\}$/.exec(shortcode); out.cellColour = arg ? arg[1].trim() : ''; }
            break;
          }
        }
        return out;
      },
      focus: function () { if (view) view.focus(); },
      destroy: function () {
        on = false;
        loading++;
        clearTimeout(syncTimer);
        clearTimeout(reloadTimer);
        o.scroll.removeEventListener('click', click, true);
        o.page.removeEventListener('mousedown', staleInput, true);
        o.page.removeEventListener('beforeinput', staleInput, true);
        if (view) { view.destroy(); view = null; }
      }
    };
    ['run', 'selectionText', 'selectedBlocks', 'sourceSelection', 'prepareEdit', 'insertMarkdown', 'insertInline', 'insertPlain', 'insertShortcode', 'wrap', 'paste'].forEach(function (name) {
      var inner = api[name];
      if (typeof inner === 'function') api[name] = function () { syncSelection(); return inner.apply(this, arguments); };
    });
    return api;
  }

  // Importers provide cleaned inert HTML; the shared schema supplies the
  // same paste semantics without creating or mutating an editor view.
  async function fromHTML(html) {
    var PM=await loadProseMirror(),template=document.createElement('template');
    template.innerHTML=window.yavcHTML.clean(html);
    return PM.DOMParser.fromSchema(schemaOf(PM)).parse(template.content).toJSON();
  }
  window.yavcPageView = { attach: attach, load: loadProseMirror, fromHTML:fromHTML, insert: insertOf, inserts: INSERTS, placeholder: PLACEHOLDER };
})();
