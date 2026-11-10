/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* The shared editing kit (docs/slides-design.md, decision 16; DESIGN.md):
   what the Editor and Slides both build on the page view and the Markdown
   engine, written once. It gives a host

     yavcMdEditor.libs()                CodeMirror, loaded beside this file
     yavcMdEditor.codeTheme(CM)         the site's colours for a code editor, from PUDL's tokens
     yavcMdEditor.codeSearch(CM, onQuery)   CodeMirror's search with the site's find bar
     yavcMdEditor.findBar(driver, onQuery)  the find bar, for a search of the host's own
     yavcMdEditor.colourRows(fields), wireColours(body)   a picker beside a colour's text field
     yavcMdEditor.pageView()            the page view (page-view.js), loaded
     yavcMdEditor.mermaid()             Mermaid, loaded
     yavcMdEditor.drawMermaid(place, source, opts)   a Mermaid diagram drawn into place
     yavcMdEditor.FORMATS, STYLES, INSERTS, SHORTCODES   the tables
     yavcMdEditor.commands(ctx)         PUDL command entries for the formats,
                                        the paragraph style and Insert
     yavcMdEditor.textFormat(view, name, ctx)   a format on a CodeMirror view
     yavcMdEditor.textInsert(view, markdown)    Markdown into a CodeMirror view
     yavcMdEditor.inserts(extra, pictures)   the / menu's items, for the page host
     yavcMdEditor.askText / askChoice / askColour / askColours   PUDL dialogs
     yavcMdEditor.askFigure(existing, pictures)   the picture dialog
     yavcMdEditor.editBlock(src, pictures)   a drawn block or inline shortcode pressed on the page
     yavcMdEditor.askShortcode(name, existing, host)   a standalone shortcode's form
     yavcMdEditor.boxCommand(ctx, opts, extra)   Box, which wraps the selection in a box
     yavcMdEditor.askProperties(front, opts)   the Properties dialog, for front matter
     yavcMdEditor.loadImages(el, pictures)   pictures of the home folder, read
     yavcMdEditor.drawBlocks(el, blocks, opts)   the engine's preview blocks drawn, unchanged ones kept
     yavcMdEditor.pictureFor(src, pictures)   a picture of the home folder as an address, cached
     yavcMdEditor.pictureData(src, pictures)  the same picture as { url } or { bytes }, for a canvas to draw inside its own picture

   ctx says where a format goes: { page(): the page view's handle or null,
   text(): the CodeMirror view or null, onPage(): whether the page takes
   it, readOnly(): whether nothing may change, say(text, isError),
   pictures(): where the document's pictures are }. pictures is { site():
   a promise of { F, FB }, the site file system and its dialogs; root: the
   applet's element; dir(): the document's folder as a path, or null }. The
   command ids are the names the Editor's key bindings use: bold, italic,
   strike, code, link, bullets, numbers, color, paragraph, h1 to h6, and
   insert-… for the blocks. Loaded with the site's static version, as
   apps/md-engine is. */
(function () {
    'use strict';
    if (window.yavcMdEditor) { return; }

    var HERE = ((document.currentScript && document.currentScript.src) || '/applets/md-editor/kit.js').split('?')[0].replace(/[^/]*$/, '');
    var STAMP = (function () { var s = (document.currentScript && document.currentScript.src) || ''; return s.indexOf('?') >= 0 ? s.slice(s.indexOf('?')) : ''; })();
    var PLACEHOLDER = 'Type here.';

    var loadScript = window.yavcUtil.loadScript;

    /* Colours from PUDL's tokens, as CSS variables, so an editor follows
       the theme with no code of its own: the Editor's and Diagrams'. */
    function codeTheme(CM) {
        var tint = function (pct) { return 'color-mix(in srgb, var(--accent) ' + pct + '%, transparent)'; };
        return [
            CM.EditorView.theme({
                '&': { color: 'var(--text)', backgroundColor: 'var(--input-bg)', height: '100%', fontSize: '14px' },
                '.cm-scroller': { fontFamily: 'var(--mono)', lineHeight: '1.5' },
                '.cm-content': { caretColor: 'var(--accent)' },
                '.cm-gutters': { backgroundColor: 'var(--surface)', color: 'var(--text-muted)', borderRight: '1px solid var(--border)' },
                '.cm-activeLine': { backgroundColor: tint(6) },
                '.cm-activeLineGutter': { backgroundColor: tint(12), color: 'var(--text)' },
                '&.cm-focused .cm-cursor': { borderLeftColor: 'var(--accent)' },
                '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': { backgroundColor: tint(26) + ' !important' },
                '.cm-selectionMatch': { backgroundColor: tint(14) },
                '.cm-matchingBracket, &.cm-focused .cm-matchingBracket': { backgroundColor: tint(22), outline: '1px solid ' + tint(50) },
                '.cm-searchMatch': { backgroundColor: 'color-mix(in srgb, var(--warn) 30%, transparent)' },
                '.cm-searchMatch.cm-searchMatch-selected': { backgroundColor: 'color-mix(in srgb, var(--warn) 55%, transparent)' },
                '.cm-panels': { backgroundColor: 'var(--surface)', color: 'var(--text)', borderColor: 'var(--border)' },
                /* CodeMirror's own panels (Go to Line); the find bar is PUDL's. */
                '.cm-panels input:not(.form-input), .cm-panels button:not(.btn):not(.icon-btn)': { font: 'inherit', fontSize: '13px' },
                '.cm-tooltip': { backgroundColor: 'var(--surface)', color: 'var(--text)', border: '1px solid var(--border)' },
                '.cm-diagnostic-error': { borderLeftColor: 'var(--syntax-error)' },
                '.cm-lintRange-error': { backgroundImage: 'none', textDecoration: 'underline wavy var(--syntax-error)' },
                '&.cm-focused': { outline: 'none' }
            }),
            /* PUDL's syntax tokens (0.26.0), after the README's CodeMirror
               recipe, with the Markdown and shell tags this editor meets. */
            CM.syntaxHighlighting(CM.HighlightStyle.define([
                { tag: [CM.tags.keyword, CM.tags.operator], color: 'var(--syntax-keyword)' },
                { tag: [CM.tags.string, CM.tags.special(CM.tags.string), CM.tags.regexp, CM.tags.monospace], color: 'var(--syntax-string)' },
                { tag: [CM.tags.number, CM.tags.bool, CM.tags.null], color: 'var(--syntax-number)' },
                { tag: [CM.tags.comment, CM.tags.meta, CM.tags.quote, CM.tags.processingInstruction], color: 'var(--syntax-comment)', fontStyle: 'italic' },
                { tag: [CM.tags.function(CM.tags.variableName), CM.tags.variableName, CM.tags.definition(CM.tags.variableName)], color: 'var(--syntax-name)' },
                { tag: [CM.tags.tagName, CM.tags.typeName, CM.tags.angleBracket], color: 'var(--syntax-tag)' },
                { tag: [CM.tags.attributeName, CM.tags.propertyName], color: 'var(--syntax-attr)' },
                { tag: CM.tags.heading, color: 'var(--syntax-heading)', fontWeight: '700' },
                { tag: [CM.tags.link, CM.tags.url], color: 'var(--syntax-link)', textDecoration: 'underline' },
                { tag: CM.tags.invalid, color: 'var(--syntax-error)' },
                { tag: CM.tags.strong, fontWeight: '700' },
                { tag: CM.tags.emphasis, fontStyle: 'italic' }
            ]))
        ];
    }

    var cmReady = null, pvReady = null, mmReady = null;
    function libs() {
        if (window.CM && window.CM.EditorView) { return Promise.resolve(window.CM); }
        if (!cmReady) { cmReady = loadScript(HERE + 'vendor/codemirror-6.43/codemirror.js?v=9').then(function () { return window.CM; }).catch(function (e) { cmReady = null; throw e; }); }
        return cmReady;
    }
    var pagesReady = null;
    function pages() {
        if (!pagesReady) { pagesReady = loadScript(HERE + 'paper-sizes.js' + STAMP).then(function () { return loadScript(HERE + 'page-lines.js' + STAMP); }).then(function () { return loadScript(HERE + 'page-footnotes.js' + STAMP); }).then(function () { return loadScript(HERE + 'pages.js' + STAMP); }).then(function () { return loadScript(HERE + 'document-settings.js' + STAMP); }).then(function () { return window.yavcDocumentPages; }).catch(function (err) { pagesReady = null; throw err; }); }
        return pagesReady;
    }
    var pageSetupReady = null;
    function pageSetup() {
        if (!pageSetupReady) { pageSetupReady = Promise.all([pages(), styles()]).then(function () { return loadScript(HERE + 'page-setup.js' + STAMP); }).then(function () { return window.yavcPageSetup; }).catch(function (err) { pageSetupReady = null; throw err; }); }
        return pageSetupReady;
    }
    var exportsReady = null;
    function documentExports() {
        if (!exportsReady) { exportsReady = pages().then(function () { return loadScript(HERE + 'export.js' + STAMP); }).then(function () { return window.yavcDocumentExport; }).catch(function (err) { exportsReady = null; throw err; }); }
        return exportsReady;
    }
    var stylesReady = null;
    function styles() {
        if (!stylesReady) {
            if (!document.querySelector('link[data-md-styles-ui]')) { var css = document.createElement('link'); css.rel = 'stylesheet'; css.href = HERE + 'styles.css' + STAMP; css.setAttribute('data-md-styles-ui', ''); document.head.appendChild(css); }
            stylesReady = Promise.all([loadScript(HERE + 'document-style.js' + STAMP), loadScript(HERE + 'styles-ui.js' + STAMP)]).then(function () { return window.yavcDocumentStyle; }).catch(function (err) { stylesReady = null; throw err; });
        }
        return stylesReady;
    }
    var languageReady = null;
    var macrosReady = null;
    function macros() {
        if (!macrosReady) { macrosReady = loadScript(HERE + 'macros.js' + STAMP).then(function () { return window.yavcMacroTools; }).catch(function (err) { macrosReady = null; throw err; }); }
        return macrosReady;
    }
    function languageServices() {
        if (!languageReady) { languageReady = Promise.all([libs(), safeHTML(), loadScript(HERE + 'language-service/bridge.js' + STAMP)]).then(function () { return window.yavcLanguageServices; }).catch(function (err) { languageReady = null; throw err; }); }
        return languageReady;
    }
    function pageView() {
        if (window.yavcPageView) { return Promise.resolve(window.yavcPageView); }
        if (!pvReady) { pvReady = pages().then(function () { return loadScript(HERE + 'page-view.js' + STAMP); }).then(function () { return window.yavcPageView; }).catch(function (e) { pvReady = null; throw e; }); }
        return pvReady;
    }
    /* The HTML cleaner (apps/md-engine/safe-html.js), which a page with the
       Markdown engine has already, and which Diagrams, drawing Mermaid with
       no Markdown engine, loads here. */
    var htmlReady = null;
    function safeHTML() {
        if (window.yavcHTML) { return Promise.resolve(window.yavcHTML); }
        if (!htmlReady) { htmlReady = loadScript(window.yavcUtil.versioned(new URL('../md-engine/safe-html.js', HERE).href)).then(function () { return window.yavcHTML; }).catch(function (e) { htmlReady = null; throw e; }); }
        return htmlReady;
    }
    function mermaid() {
        if (window.mermaid) { return Promise.resolve(window.mermaid); }
        if (!mmReady) { mmReady = loadScript(HERE + 'vendor/mermaid/mermaid.min.js').then(function () { return window.mermaid; }).catch(function (e) { mmReady = null; throw e; }); }
        return mmReady;
    }

    /* === The tables ======================================================= */

    var FORMATS = [
        ['bold', 'Bold', 'bold', 'Mod-b'], ['italic', 'Italic', 'italic', 'Mod-i'], ['underline', 'Underline', 'underline', 'Mod-u'], ['strike', 'Strike Through', 'strike'],
        ['code', 'Code', 'code', 'Mod-`'], ['highlight', 'Highlight', 'yavc-highlight', 'Mod-Shift-h'],
        ['superscript', 'Superscript', 'yavc-superscript'], ['subscript', 'Subscript', 'yavc-subscript'], ['link', 'Link…', 'link', 'Mod-k'],
        ['bullets', 'Bulleted List', 'list'], ['numbers', 'Numbered List', 'list-numbered'], ['quote', 'Quote', 'quote'], ['color', 'Text Color…', 'colour'], ['class', 'Text Style…', 'tag']
    ];
    /* The classes a stylesheet defines, for the Style dialog. */
    function classesOf(css) {
        var out = {}, re = /\.([A-Za-z][A-Za-z0-9-]*)/g, m;
        while ((m = re.exec(css || ''))) { if (!/^(deck|sc|doc|pm|dinah|md|win|btn|menu|notice|form)(-|$)/.test(m[1])) { out[m[1]] = true; } }
        return Object.keys(out).sort();
    }
    /* The classes the site's page stylesheet (apps/md-engine/page.css)
       defines for every document, so a person who writes no CSS still has
       alignment, indenting, a smaller or a larger size, and a block that
       stays on one page. */
    var STOCK_CLASSES = ['center', 'right', 'justify', 'indent', 'small', 'lead', 'no-break'];
    var STYLES = [['paragraph', 'Paragraph', 'Mod-0']].concat([1, 2, 3, 4, 5, 6].map(function (n) { return ['h' + n, 'Heading ' + n, 'Mod-' + n]; }));
    var INSERTS = [
        { id: 'insert-checklist', label: 'Checklist', markdown: '- [ ] ' + PLACEHOLDER },
        { id: 'insert-table', label: 'Table', markdown: '| Heading 1 | Heading 2 | Heading 3 |\n| --- | --- | --- |\n| | | |\n| | | |' },
        { id: 'insert-code', label: 'Code Block', markdown: '```\ncode\n```' }
    ];
    var SHORTCODES = [
        ['figure', 'Picture', '{{figure:src="" alt="What it shows" width=60%}}'],
        ['columns', 'Columns', '{{columns}}\n' + PLACEHOLDER + '\n\n{{column}}\n\nThe second column.\n{{/columns}}'],
        ['tip', 'Tip', '{{tip:A title}}\n' + PLACEHOLDER + '\n{{/tip}}'],
        ['note', 'Note', '{{note:A title}}\n' + PLACEHOLDER + '\n{{/note}}'],
        ['info', 'Information Notice', '{{info:A title}}\n' + PLACEHOLDER + '\n{{/info}}'],
        ['warning', 'Warning', '{{warning:A title}}\n' + PLACEHOLDER + '\n{{/warning}}'],
        ['expand', 'Expand', '{{expand:The title}}\n' + PLACEHOLDER + '\n{{/expand}}'],
        ['video', 'Video…', null],
        ['applet', 'Applet…', null],
        ['diagram', 'Diagram from a File…', null],
        ['code', 'Code Listing from a File…', null],
        ['color', 'Colored Words', '{{color:accent}}these words{{/color}}'],
        ['toc', 'Contents…', null],
        ['status', 'Status Label…', null],
        ['kbd', 'Keys…', null],
        ['glyph', 'Glyph…', null],
        ['page', 'Page Link…', null],
        ['anchor', 'Anchor…', null],
        ['draft-note', 'Note to Self', '{{draft-note}}\n' + PLACEHOLDER + '\n{{/draft-note}}'],
        ['caption', 'Caption', '{{caption}}\n' + PLACEHOLDER + '\n{{/caption}}'],
        ['page-break', 'Page Break', '{{page-break}}']
    ].map(function (x) { return { id: 'insert-sc-' + x[0], name: x[0], label: x[1], markdown: x[2], shortcode: true }; });

    /* The Markdown an insert item puts in: its example, or for a shortcode
       with a form, what its form is filled in with (null for Cancel). */
    function markdownFor(item, ctx) {
        var host = ctx && ctx.pictures ? ctx.pictures() : null;
        if (item.name === 'figure') { return askFigure(null, host); }
        if (FORMS[item.name]) { return askShortcode(item.name, null, host); }
        return Promise.resolve(item.markdown);
    }
    /* Whether a shortcode goes in a line of text, rather than on a line of its own. */
    function isInlineForm(name) { return !!(FORMS[name] && FORMS[name].inline); }

    /* === Formats on the Markdown ========================================= */

    /* The text a format writes around words: what opens and what closes.
       Underline is the HTML element the page view already reads and writes;
       the other three are the engine's span shortcodes. */
    var WRAPS = {
        bold: ['**', '**'], italic: ['*', '*'], strike: ['~~', '~~'], code: ['`', '`'], underline: ['<u>', '</u>'],
        highlight: ['{{mark}}', '{{/mark}}'], superscript: ['{{sup}}', '{{/sup}}'], subscript: ['{{sub}}', '{{/sub}}']
    };
    function textFormat(view, name, ctx) {
        if (!view) { return; }
        var state = view.state, sel = state.selection.main;
        if (WRAPS[name]) {
            var open = WRAPS[name][0], shut = WRAPS[name][1], text = state.sliceDoc(sel.from, sel.to);
            var before = state.sliceDoc(Math.max(0, sel.from - open.length), sel.from), after = state.sliceDoc(sel.to, Math.min(state.doc.length, sel.to + shut.length));
            if (text.length >= open.length + shut.length && text.startsWith(open) && text.endsWith(shut)) {
                view.dispatch({ changes: { from: sel.from, to: sel.to, insert: text.slice(open.length, text.length - shut.length) }, selection: { anchor: sel.from, head: sel.to - open.length - shut.length }, userEvent: 'input' });
            } else if (before === open && after === shut) {
                view.dispatch({ changes: [{ from: sel.from - open.length, to: sel.from }, { from: sel.to, to: sel.to + shut.length }], selection: { anchor: sel.from - open.length, head: sel.to - open.length }, userEvent: 'input' });
            } else {
                view.dispatch({ changes: { from: sel.from, to: sel.to, insert: open + text + shut }, selection: { anchor: sel.from + open.length, head: sel.to + open.length }, userEvent: 'input' });
            }
            view.focus();
            return;
        }
        if (name === 'link') {
            var chosen = state.sliceDoc(sel.from, sel.to);
            var owner = ctx.identity && ctx.identity();
            function currentLink() { return !(ctx.signal && ctx.signal.aborted) && (!ctx.identity || ctx.identity() === owner) && view.state.doc === state.doc && view.state.selection.eq(state.selection) && !(ctx.readOnly && ctx.readOnly()); }
            askText('Link', 'Address', /^https?:\/\//.test(chosen) ? chosen : 'https://', { signal: ctx.signal, okLabel: 'Apply', validate: function () { return currentLink() ? null : 'The document or selection changed. Reopen this form.'; } }).then(function (href) {
                if (!href || !currentLink()) { return; }
                var label = chosen && !/^https?:\/\//.test(chosen) ? chosen : href;
                var md = '[' + label + '](' + href + ')';
                view.dispatch({ changes: { from: sel.from, to: sel.to, insert: md }, selection: { anchor: sel.from + 1, head: sel.from + 1 + label.length }, userEvent: 'input' });
                view.focus();
            });
            return;
        }
        if (name === 'class' || name === 'color') { return formatDialog(name, ctx, false); }
        var first = state.doc.lineAt(sel.from), last = state.doc.lineAt(sel.to), changes = [];
        if (name === 'quote') {
            if (sel.to > sel.from && sel.to === last.from) { last = state.doc.lineAt(sel.to - 1); }
            var quoted = true;
            for (var lineNumber = first.number; lineNumber <= last.number; lineNumber++) { if (!/^ {0,3}>[ \t]?/.test(state.doc.line(lineNumber).text)) { quoted = false; } }
            for (var number = first.number; number <= last.number; number++) {
                var quoteLine = state.doc.line(number), marker = /^ {0,3}>[ \t]?/.exec(quoteLine.text);
                changes.push({ from: quoteLine.from, to: quoteLine.from + (quoted ? marker[0].length : 0), insert: quoted ? '' : '> ' });
            }
            view.dispatch({ changes: changes, userEvent: 'input' }); view.focus(); return;
        }
        var level = /^h([1-6])$/.exec(name), prefix = level ? '#'.repeat(+level[1]) + ' ' : name === 'bullets' ? '- ' : null;
        if (!level && name !== 'bullets' && name !== 'numbers' && name !== 'paragraph') { return; }
        var every = true;
        for (var n = first.number; n <= last.number; n++) {
            var line = state.doc.line(n), m = /^(\s*)(#{1,6}\s+|[-*+]\s+|\d+[.)]\s+)?/.exec(line.text), marked = m[2] || '';
            if (name === 'numbers') { if (!/^\d+[.)]\s+$/.test(marked)) { every = false; } }
            else if (prefix && marked !== prefix) { every = false; }
        }
        var k = 1;
        for (var n2 = first.number; n2 <= last.number; n2++) {
            var ln = state.doc.line(n2), mm = /^(\s*)(#{1,6}\s+|[-*+]\s+|\d+[.)]\s+)?/.exec(ln.text);
            var indent = mm[1], old = mm[2] || '', put = '';
            /* A list already on every line comes off; a heading level chosen
               again stays, as a paragraph style does. */
            if (name === 'paragraph' || (every && !level)) { put = ''; }
            else if (name === 'numbers') { put = (k++) + '. '; }
            else { put = prefix; }
            if (old !== put) { changes.push({ from: ln.from + indent.length, to: ln.from + indent.length + old.length, insert: put }); }
        }
        if (changes.length) { view.dispatch({ changes: changes, userEvent: 'input' }); }
        view.focus();
    }
    /* An inline shortcode into a CodeMirror view, at the cursor. */
    function textInsertInline(view, markdown) {
        var sel = view.state.selection.main;
        view.dispatch({ changes: { from: sel.from, to: sel.to, insert: markdown }, selection: { anchor: sel.from + markdown.length }, userEvent: 'input' });
        view.focus();
    }
    function textInsert(view, markdown) {
        if (!view) { return; }
        var sel = view.state.selection.main, at = sel.to;
        var line = view.state.doc.lineAt(at), lead = line.text.trim() ? '\n\n' : '\n';
        view.dispatch({ changes: { from: at, insert: lead + markdown + '\n' }, selection: { anchor: at + lead.length }, userEvent: 'input' });
        view.focus();
    }

    /* === Where a format goes ============================================ */

    function apply(name, ctx) {
        if (ctx.readOnly && ctx.readOnly()) { if (ctx.say) { ctx.say('This is read-only, so it cannot be changed.', true); } return; }
        var page = ctx.page && ctx.page();
        if (name === 'color' || name === 'class') return formatDialog(name, ctx, !!(page && ctx.onPage()));
        if (page && ctx.onPage()) {
            page.run(name);
            return;
        }
        textFormat(ctx.text && ctx.text(), name, ctx);
    }
    async function sourceFormatTarget(view, name) {
        if (!view) return null;
        var state = view.state, selection = state.selection, sel = selection.main, source = state.doc.toString();
        if(sel.empty)return null;
        var engine = await window.yavcMdEngine(), parsed = engine.doc(source), offset = parsed.attrs && parsed.attrs.front ? parsed.attrs.front.length : 0, block = null, blockStart=0,blockEnd=0;
        (parsed.content || []).forEach(function (node) {
            var attrs=node.attrs || {};offset+=(attrs.gap || '').length;
            var end=offset+(attrs.src || '').length;
            if(sel.from>=offset && sel.to<=end && (node.type==='paragraph' || node.type==='heading')) {block=node;blockStart=offset;blockEnd=end;}
            offset=end;
        });
        if(!block)return null;
        // Source edits are bounded to parser-confirmed, uncomplicated rich text.
        // Complex Markdown remains editable through the rich surface or direct source editing.
        var plain=block.attrs.src.replace(/\{\{(class|color):[^}\r\n]{1,128}\}\}([^{}]*?)\{\{\/\1\}\}/g,'$2');
        if(/[\\`<>{}\[\]*_~]/.test(plain))return null;
        var pattern = new RegExp('\\{\\{' + name + ':([^}\\r\\n]{1,128})\\}\\}([^{}]*?)\\{\\{/' + name + '\\}\\}', 'g'), match, existing = null;
        while ((match = pattern.exec(source))) {
            if(match.index<blockStart || match.index+match[0].length>blockEnd)continue;
            var start = match.index + match[0].indexOf('}}') + 2, end = start + match[2].length;
            if (sel.from >= start && sel.to <= end) { existing = { start: match.index, from: start, to: end, end: match.index + match[0].length, value: match[1] }; break; }
        }
        if (sel.empty || /[{}]/.test(source.slice(sel.from, sel.to))) return null;
        function current() { return view.state.doc === state.doc && view.state.selection.eq(selection); }
        return { value: existing ? existing.value : '', mixed: false, empty: false, text: source.slice(sel.from, sel.to) || (existing ? source.slice(existing.from, existing.to) : ''), current: current,
            apply: function (value) {
                if (!current()) return false;
                var changes, offset = 0;
                if (existing) {
                    var close = '{{/' + name + '}}', oldOpen = '{{' + name + ':' + existing.value + '}}';
                    var before = source.slice(existing.from, sel.from), after = source.slice(sel.to, existing.to);
                    var left = before ? oldOpen + before + close : '', open = value ? '{{' + name + ':' + value + '}}' : '';
                    var replacement = left + open + source.slice(sel.from, sel.to) + (value ? close : '') + (after ? oldOpen + after + close : '');
                    changes = [{ from: existing.start, to: existing.end, insert: replacement }];
                    offset = existing.start + left.length + open.length - sel.from;
                } else {
                    if (!value) return true;
                    var prefix = '{{' + name + ':' + value + '}}'; offset = prefix.length;
                    changes = [{ from: sel.from, insert: prefix }, { from: sel.to, insert: '{{/' + name + '}}' }];
                }
                view.dispatch({ changes: changes, selection: { anchor: sel.from + offset, head: sel.to + offset }, userEvent: 'input' }); view.focus(); return true;
            } };
    }
    async function formatDialog(name, ctx, rich) {
        var owner = ctx.identity && ctx.identity(), page = rich && ctx.page(), view = !rich && ctx.text();
        var target = rich ? page.formatSelection(name) : await sourceFormatTarget(view, name);
        if (!target) { if (ctx.say) ctx.say('Select text in an editable paragraph. Complex Markdown can be formatted in Visual view or edited directly in Text view.', true); return; }
        function current() { return !(ctx.signal && ctx.signal.aborted) && (!ctx.identity || owner === ctx.identity()) && !(ctx.readOnly && ctx.readOnly()) && (rich ? ctx.page() === page && ctx.onPage() : ctx.text() === view) && target.current(); }
        if (ctx.prepareFormats) await ctx.prepareFormats();
        if (!current()) {if(ctx.say)ctx.say('The document or selection changed. Reopen formatting for the intended text.',true);return;}
        var options = { value: target.value, mixed: target.mixed, sample: target.text, empty: target.empty, current: current, signal: ctx.signal, scope: ctx.styleScope ? ctx.styleScope() : '' };
        var answer = name === 'class' ? await askClass(ctx.classes ? ctx.classes() : [], options) : await askColour(options);
        if (answer != null && current()) target.apply(answer);
    }
    function insertItem(item, ctx) {
        if (ctx.readOnly && ctx.readOnly()) { if (ctx.say) { ctx.say('This is read-only, so nothing can be added to it.', true); } return; }
        var owner = ctx.identity && ctx.identity(), originalPage = ctx.page && ctx.page(), rich = originalPage && ctx.onPage();
        var bookmark = rich && originalPage.clipboardSelection(), originalView = !rich && ctx.text && ctx.text();
        var state = originalView && originalView.state;
        markdownFor(item, ctx).then(function (md) {
            if (!md) { return; }
            var valid = !(ctx.signal && ctx.signal.aborted) && (!ctx.identity || ctx.identity() === owner) && !(ctx.readOnly && ctx.readOnly());
            valid = valid && (rich ? ctx.page() === originalPage && ctx.onPage() && bookmark && bookmark.current() : ctx.text() === originalView && (!state || originalView.state.doc === state.doc && originalView.state.selection.eq(state.selection)));
            if (!valid) { if (ctx.say) ctx.say('The document or selection changed. Insert again at the intended location.',true); return; }
            var page = ctx.page && ctx.page();
            if (page && ctx.onPage()) {
                if (!item.shortcode && !item.name) { page.run(item.id); }
                else if (isInlineForm(item.name) && page.insertInline) { page.insertInline(md); }
                else { page.insertMarkdown(md); }
                return;
            }
            var view = ctx.text && ctx.text();
            if (view && isInlineForm(item.name)) { textInsertInline(view, md); }
            else if (view) { textInsert(view, md); } else if (ctx.append) { ctx.append(md); }
        }).catch(function (err) { if (ctx.say && !(ctx.signal && ctx.signal.aborted)) ctx.say(err.message || String(err),true); });
    }

    /* The command entries for a host's command table: the formats, the
       paragraph style as one choice, and Insert as a submenu. extra adds
       the host's own insert items ({ id, label, markdown } or { id, label,
       run }), after the kit's. */
    function commands(ctx, extra) {
        var out = [];
        FORMATS.forEach(function (f) {
            out.push({ id: f[0], label: f[1], glyph: f[2], shortcut: window.yavcUtil.shortcut(f[3]), run: function () { apply(f[0], ctx); }, glyphOnly: f[0] !== 'link' && f[0] !== 'color', checked: f[0] === 'quote' ? function () { return quoteState(ctx); } : undefined });
        });
        out.push({ id: 'style', label: 'Paragraph Style', glyph: 'paragraph',
            current: function () { var s = ctx.blockStyle ? ctx.blockStyle() : null, f = STYLES.find(function (x) { return x[0] === s; }); return f ? f[1] : null; },
            items: STYLES.map(function (st) { return { id: st[0], label: st[1], shortcut: window.yavcUtil.shortcut(st[2]), choice: 'md-style', value: st[0], checked: function () { return ctx.blockStyle ? ctx.blockStyle() === st[0] : false; }, run: function () { apply(st[0], ctx); } }; }) });
        var items = INSERTS.map(function (i) { return { id: i.id, label: i.label, run: function () { insertItem(i, ctx); } }; });
        items.push('-');
        SHORTCODES.forEach(function (i) { items.push({ id: i.id, label: i.label, run: function () { insertItem(i, ctx); } }); });
        (extra || []).forEach(function (i) {
            if (i === '-') { items.push('-'); return; }
            items.push({ id: i.id, label: i.label, run: function () { if (i.run) { i.run(); } else { insertItem(i, ctx); } } });
        });
        out.push({ id: 'insert', label: 'Other', glyph: 'more', items: items });
        out.push(tableCommand(ctx));
        out.push(boxCommand(ctx));
        return out;
    }
    /* Table: a new table, and, while the cursor is in one on the page, rows
       and columns added and deleted there. Merging and splitting cells,
       heading cells outside the first row and a cell's colour, which a
       Markdown table cannot hold, turn the table into an HTML table in the
       text (DESIGN.md, decision 112), which the page goes on editing.
       opts (hidden, disabled) apply to the command and every row, as
       boxCommand's do. */
    var TABLE_OPS = [
        ['row-above', 'Insert Row Above'], ['row-below', 'Insert Row Below'], ['column-before', 'Insert Column Before'], ['column-after', 'Insert Column After'],
        ['delete-row', 'Delete Row'], ['delete-column', 'Delete Column'], ['delete-table', 'Delete Table'], '-',
        ['merge-cells', 'Merge Cells'], ['split-cell', 'Split Cell'], ['heading-row', 'Heading Row'], ['heading-column', 'Heading Column'], ['heading-cell', 'Heading Cell']
    ];
    /* The colour of the selected cells, asked in the colour dialog; an
       empty answer takes the colour away. */
    function askCellColour(page, pictures) {
        /* On a pipe table, a host that keeps a cell's colour in a
           shortcode of its own is asked with its own form, which refuses
           a colour it does not know, and the table stays a pipe table
           (DESIGN.md, decision 119). */
        var held = page.cellColourShortcode ? page.cellColourShortcode() : null, service = pictures && hostService(pictures);
        /* A host that marks the shortcode and gives no form for it: the
           plain dialog would turn the table into an HTML one, so the
           command says so and changes nothing. */
        if (held && !(service && typeof service.form === 'function' && typeof service.build === 'function')) {
            if (pictures && pictures.say) { pictures.say('The application gave no form for a cell color, so nothing was changed.', true); }
            return Promise.resolve();
        }
        if (held) {
            var asked = held.source || held.example || '{{' + held.name + '}}';
            return hostCall(new Map(), function (signal) { return service.form(asked, hostContext(pictures, signal)); }, pictures).then(function (form) {
                if (!held.current()) { return; }
                if (!form) { if (pictures.say) { pictures.say('The application gave no form for a cell color, so nothing was changed.', true); } return; }
                return askHostShortcode(form, pictures, { existing: !!held.source }).then(function (source) {
                    if (source == null || !held.current()) { return; }
                    return held.set(String(source).trim()).then(function (done) { if (!done && source && pictures.say) { pictures.say('The cell color could not be written to this table.', true); } });
                });
            });
        }
        var current = page.marks().cellColour || '';
        return window.yavcDialog.askText('Cell Color', 'Background Color', current, {
            placeholder: '#DEEBFF', help: 'A CSS color for the background of the selected cells. Leave it empty to take the color away.', okLabel: 'Apply',
            validate: function (v) { v = String(v || '').trim(); return !v || (v.length <= 64 && /^[#()%.,\/\sA-Za-z0-9-]+$/.test(v) && CSS.supports('background-color', v)) ? null : 'That is not a color the browser knows.'; }
        }).then(function (v) {
            if (v == null) { return; }
            page.run('cell-colour', String(v).trim());
            page.focus();
        });
    }
    function quoteState(ctx) {
        var page = ctx.page && ctx.page(); if (page && ctx.onPage && ctx.onPage()) { return !!page.marks().quote; }
        var view = ctx.text && ctx.text(); return !!view && /^ {0,3}>/.test(view.state.doc.lineAt(view.state.selection.main.from).text);
    }
    function tableCommand(ctx, opts) {
        var newTable = INSERTS.find(function (i) { return i.id === 'insert-table'; });
        function pageHere() { var p = ctx.page && ctx.page(); return p && ctx.onPage && ctx.onPage() ? p : null; }
        function row(item, offered) {
            var r = Object.assign({}, item, opts || {});
            if (offered) {
                // A row that changes a table is there only where the cursor is in one.
                r.hidden = function () { return !!(opts && opts.hidden && opts.hidden()) || !(pageHere() && pageHere().inTable && pageHere().inTable()); };
                r.disabled = function () { var p = pageHere(); return !!(opts && opts.disabled && opts.disabled()) || !(p && p.can && p.can(item.op)); };
            }
            return r;
        }
        var items = [row({ id: 'table-new', label: 'Insert Table', run: function () { insertItem(newTable, ctx); } }, false)];
        TABLE_OPS.forEach(function (t) {
            if (t === '-') { items.push('-'); return; }
            items.push(row({ id: 'table-' + t[0], label: t[1], op: t[0], run: function () { var p = pageHere(); if (p) { p.run(t[0]); } } }, true));
        });
        items.push(row({ id: 'table-cell-colour', label: 'Cell Color…', op: 'cell-colour', run: function () { var p = pageHere(); if (p) { askCellColour(p, ctx.pictures ? ctx.pictures() : null); } } }, true));
        items.push('-');
        [['default', 'Default Alignment'], ['left', 'Align Column Left'], ['center', 'Center Column'], ['right', 'Align Column Right']].forEach(function (option) {
            var value = option[0] === 'default' ? '' : option[0];
            items.push(row({ id: 'table-align-' + option[0], label: option[1], op: 'align-' + option[0],
                checked: function () { var p = pageHere(); return !!p && p.marks().columnAlign === value; },
                run: function () { var p = pageHere(); if (p) { p.run('align-' + option[0]); } } }, true));
        });
        return Object.assign({ id: 'table', label: 'Table', glyph: 'table', items: items }, opts || {});
    }

    /* The words in a document's text, for the status bar. The front matter,
       the shortcodes' own tags and arguments, Markdown's marks and the
       addresses of links and pictures are not words; what words are is the
       browser's, through Intl.Segmenter (ECMA-402), which also finds them
       in text written without spaces. Where a browser lacks it, words are
       runs of letters and digits. */
    var segmenter = typeof Intl !== 'undefined' && Intl.Segmenter ? new Intl.Segmenter(undefined, { granularity: 'word' }) : null;
    function wordCount(text) {
        var s = String(text || '').replace(/^---\r?\n[\s\S]*?\r?\n---[ \t]*(\r?\n|$)/, '')
            .replace(/\{\{\/?[a-z][a-z0-9-]*(?::(?:[^}]|\}(?!\}))*)?\}\}/g, ' ')
            .replace(/\]\([^)\s]*(?:\s+"[^"]*")?\)/g, ']')
            .replace(/\bhttps?:\/\/\S+/g, ' ');
        var n = 0;
        if (segmenter) {
            for (var seg of segmenter.segment(s)) { if (seg.isWordLike) { n++; } }
            return n;
        }
        return (s.match(/[\p{L}\p{N}]+/gu) || []).length;
    }
    /* Minutes to read the words, at the 238 words a minute that Brysbaert's
       2019 review found for adults silently reading non-fiction; never less
       than one minute for any words at all. */
    function readingMinutes(words) { return words ? Math.max(1, Math.round(words / 238)) : 0; }
    /* Box: the selection, or the line the cursor is on, wrapped in a box,
       a container shortcode (docs/page-view-design.md). On the page the
       selected blocks go into the box; in the text the selected lines go
       between its opening and closing lines. */
    var BOXES = [['tip', 'Tip'], ['note', 'Note'], ['info', 'Information Notice'], ['warning', 'Warning'], ['expand', 'Expand'], ['draft-note', 'Note to Self'], ['columns', 'Columns']];
    function boxCommand(ctx, opts, extra) {
        var list = BOXES.concat(extra || []);
        return Object.assign({ id: 'box', label: 'Box', glyph: 'square',
            items: list.map(function (b) { return Object.assign({ id: 'box-' + b[0], label: b[1], run: function () { wrapBox(b[0], ctx); } }, opts || {}); }) }, opts || {});
    }
    function wrapBox(name, ctx) {
        if (ctx.readOnly && ctx.readOnly()) { if (ctx.say) { ctx.say('This is read-only, so it cannot be changed.', true); } return; }
        var page = ctx.page && ctx.page();
        if (page && ctx.onPage() && page.wrap) { page.wrap(name); return; }
        var view = ctx.text && ctx.text();
        if (!view) { return; }
        var st = view.state, sel = st.selection.main, first = st.doc.lineAt(sel.from), last = st.doc.lineAt(sel.to);
        var text = st.sliceDoc(first.from, last.to);
        if (!text.trim()) { text = PLACEHOLDER; }
        var open = '{{' + name + '}}\n', close = name === 'columns' ? '\n\n{{column}}\n\n' + PLACEHOLDER + '\n{{/columns}}' : '\n{{/' + name + '}}';
        view.dispatch({ changes: { from: first.from, to: last.to, insert: open + text + close }, selection: { anchor: first.from + open.length, head: first.from + open.length + text.length }, userEvent: 'input' });
        view.focus();
    }
    /* The ids a toolbar or menu lists, in the kit's order. */
    /* Commands retain an editor selection while browser clipboard permission
       is pending. No mutation may cross a document or selection change. */
    function editing(ctx) {
        function target() {
            var identity = ctx.identity(), preview = ctx.preview && ctx.preview();
            var rich = ctx.rich(), page = ctx.page(), source = ctx.source();
            function owns() { return ctx.alive() && ctx.identity() === identity; }
            if (preview) {
                var selected = window.getSelection(), text = selected && preview.contains(selected.anchorNode) && preview.contains(selected.focusNode) ? selected.toString() : '';
                return { text: text, empty: !text, current: owns, select: function () { var range = document.createRange(); range.selectNodeContents(preview); var sel = window.getSelection(); sel.removeAllRanges(); sel.addRange(range); } };
            }
            if (rich && page) {
                var selection = page.clipboardSelection();
                if (!selection) return null;
                return Object.assign({}, selection, { current: function () { return owns() && ctx.rich() && ctx.page() === page && selection.current(); },
                    select: function () { page.selectAll(); }, paste: function (data) { return page.paste(data); } });
            }
            if (!source) return null;
            var doc = source.state.doc, sel = source.state.selection;
            function current() { return owns() && ctx.source() === source && source.state.doc === doc && source.state.selection.eq(sel); }
            return { text: sel.ranges.map(function (r) { return doc.sliceString(r.from, r.to); }).join('\n'), empty: sel.ranges.every(function (r) { return r.empty; }), current: current,
                remove: function () { if (current()) source.dispatch(Object.assign(source.state.replaceSelection(''), { userEvent: 'delete.cut', annotations: window.CM.isolateHistory.of('full'), scrollIntoView: true })); },
                select: function () { source.dispatch({ selection: { anchor: 0, head: doc.length } }); source.focus(); },
                paste: function (data) { source.dispatch(Object.assign(source.state.replaceSelection(data.text || ''), { userEvent: 'input.paste', annotations: window.CM.isolateHistory.of('full'), scrollIntoView: true })); source.focus(); } };
        }
        async function clipboard(name) {
            var t = target();
            if (!t || ((name === 'cut' || name === 'paste') && ctx.readOnly())) return;
            try {
                if (name === 'select-all') { t.select(); ctx.changed(); return; }
                var api = navigator.clipboard;
                if (!api) throw new Error('Clipboard access is unavailable. Use the keyboard shortcut.');
                if (name === 'paste') {
                    var data = { text: '' };
                    async function clipboardText(item, type) {
                        var blob = await item.getType(type);
                        if (blob.size > (type === 'text/html' ? 4194304 : 1048576)) throw new Error('The clipboard content is too large to paste.');
                        return blob.text();
                    }
                    if (ctx.rich() && api.read) {
                        var items = await api.read();
                        for (var i = 0; i < items.length; i++) {
                            if (items[i].types.indexOf('text/html') >= 0) data.html = await clipboardText(items[i], 'text/html');
                            if (items[i].types.indexOf('text/plain') >= 0) data.text = await clipboardText(items[i], 'text/plain');
                            if (data.html || data.text) break;
                        }
                    } else { data.text = await api.readText(); }
                    if (data.text.length > 262144 || (data.html || '').length > 1048576) throw new Error('The clipboard content is too large to paste.');
                    if (!t.current()) throw new Error('The document or selection changed. Paste again.');
                    if (!data.text && !data.html) return;
                    await t.paste(data);
                } else {
                    if (t.empty) return;
                    if (t.text.length > 262144 || (t.html || '').length > 1048576) throw new Error('The selected content is too large to copy.');
                    if (t.html && api.write && window.ClipboardItem) {
                        await api.write([new ClipboardItem({ 'text/plain': new Blob([t.text], { type: 'text/plain' }), 'text/html': new Blob([t.html], { type: 'text/html' }) })]);
                    } else { await api.writeText(t.text); }
                    if (name === 'cut') { if (!t.current() || ctx.readOnly()) throw new Error('The document or selection changed. The text was copied but was not removed.'); t.remove(); }
                }
                ctx.changed();
            } catch (err) { if (ctx.alive()) ctx.say((err && err.name === 'NotAllowedError' ? 'Clipboard access was denied. Use the keyboard shortcut or allow clipboard access in the browser.' : err.message || String(err)), true); }
        }
        function available(name) {
            if (!ctx.alive() || (name !== 'copy' && name !== 'select-all' && ctx.readOnly())) return false;
            var t = target(); return !!t && (name === 'paste' || name === 'select-all' || !t.empty);
        }
        function canHistory(name) {
            if (!ctx.alive() || (ctx.historyReadOnly ? ctx.historyReadOnly() : ctx.readOnly())) return false;
            var page = ctx.page(), source = ctx.source();
            if (ctx.unified) {
                if (page && page.active() && page.pending()) return name === 'undo';
            } else if (ctx.rich() && page) return page.historyAvailable(name);
            return !!(source && (ctx.sourceHistory ? ctx.sourceHistory.can(name,source.state) : window.CM && window.CM[name + 'Depth'](source.state)));
        }
        async function history(name) {
            if (!canHistory(name)) return;
            var identity = ctx.identity(), page = ctx.page(), source = ctx.source(), rich = ctx.rich();
            try {
                if (page && page.active()) await page.sync();
                if (!ctx.alive() || (ctx.historyReadOnly ? ctx.historyReadOnly() : ctx.readOnly()) || ctx.identity() !== identity || ctx.source() !== source) return;
                if (!ctx.unified && rich) { if (ctx.page() === page && page.active()) { page.run(name); await page.sync(); } }
                else if(ctx.sourceHistory)ctx.sourceHistory.run(name,source);
                else window.CM[name](source);
                ctx.changed();
            } catch (err) { if (ctx.alive()) ctx.say(err.message || String(err), true); }
        }
        return { clipboard: clipboard, available: available, history: history, canHistory: canHistory,
            commands: function () { return [['cut', 'Cut', 'Mod-x'], ['copy', 'Copy', 'Mod-c'], ['paste', 'Paste', 'Mod-v'], ['select-all', 'Select All', 'Mod-a']].map(function (item) {
                return { id: item[0], label: item[1], glyph: item[0] === 'copy' ? 'copy' : undefined, shortcut: window.yavcUtil.shortcut(item[2]), bind: false,
                    disabled: function () { return !available(item[0]); }, run: function () { return clipboard(item[0]); } };
            }); } };
    }

    var MENU = {
        format: ['style', '-', 'bold', 'italic', 'underline', 'strike', 'code', '-', 'highlight', 'superscript', 'subscript', '-', 'link', 'color', 'class', '-', 'bullets', 'numbers', 'quote', '-', 'insert', 'table', 'box'],
        toolbar: ['style', 'bold', 'italic', 'underline', 'strike', 'code', 'link', 'color', 'class', 'bullets', 'numbers', 'insert', 'table', 'box']
    };

    /* The / menu's items, for the page host's inserts(): the shortcodes,
       and the host's own after them. */
    function inserts(extra, pictures) {
        function onPage(md) { if (md && window.yavcMdEditor._page) { window.yavcMdEditor._page.insertMarkdown(md); } }
        var out = SHORTCODES.map(function (i) {
            var it = { id: i.id, label: i.label.replace(/…$/, ''), words: i.name + ' ' + i.label.toLowerCase() };
            if (i.name === 'figure') { it.run = function () { askFigure(null, pictures ? pictures() : null).then(onPage); }; }
            else if (FORMS[i.name]) {
                it.run = function () {
                    askShortcode(i.name, null, pictures ? pictures() : null).then(function (md) {
                        var page = window.yavcMdEditor._page;
                        if (!md || !page) { return; }
                        if (isInlineForm(i.name) && page.insertInline) { page.insertInline(md); } else { page.insertMarkdown(md); }
                    });
                };
            }
            else { it.markdown = i.markdown; }
            return it;
        });
        return out.concat((extra || []).map(function (i) { return { id: i.id, label: i.label, words: (i.words || i.label || '').toLowerCase(), markdown: i.markdown, run: i.run }; }));
    }

    /* === Dialogs ========================================================== */

    /* The kit's questions are the site's dialogs (static/dialogs.js), with
       the kit's own bodies: swatches, colour pickers, several fields. */
    var dlgCount = 0;
    var esc = window.yavcUtil.esc;
    function dialog(title, bodyHTML, buttons, onOpen) {
        dlgCount++;
        return window.yavcDialog.open({ title: title, body: bodyHTML, buttons: buttons, className: 'md-editor-dialog', onOpen: onOpen });
    }
    function askText(title, label, value, opts) { return window.yavcDialog.askText(title, label, value, opts); }
    function askChoice(title, body, buttons) { return window.yavcDialog.askChoice(title, body, buttons); }
    /* Several text fields in one dialog: fields is [{ key, label, value,
       placeholder, help }]; resolves to the values by key, or null. */
    function askFields(title, fields, okLabel) {
        var n = dlgCount + 1;
        var html = fields.map(function (f, k) {
            var id = 'md-dlg-' + n + '-' + k;
            return '<div class="form-group"><label class="form-label" for="' + id + '">' + esc(f.label) + '</label>' +
                '<input class="form-input" id="' + id + '" data-key="' + esc(f.key) + '" value="' + esc(f.value || '') + '" autocomplete="off"' + (f.placeholder ? ' placeholder="' + esc(f.placeholder) + '"' : '') + '>' +
                (f.help ? '<p class="form-help">' + esc(f.help) + '</p>' : '') + '</div>';
        }).join('');
        return dialog(title, html, [{ id: 'cancel', label: 'Cancel' }, { id: 'ok', label: okLabel || 'OK', primary: true }]).then(function (r) {
            if (r.value !== 'ok') { return null; }
            var out = {};
            r.body.querySelectorAll('[data-key]').forEach(function (i) { out[i.getAttribute('data-key')] = i.value.trim(); });
            return out;
        });
    }
    var SWATCHES = [['ink', 'Text'], ['muted', 'Muted'], ['accent', 'Accent'], ['danger', 'Danger'], ['warn', 'Warning'], ['positive', 'Positive']];
    function swatchCSS(name) {
        return { ink: 'var(--text)', muted: 'var(--text-muted)', accent: 'var(--accent)', danger: 'var(--danger)', warn: 'var(--warn)', positive: 'var(--positive)' }[name] || name;
    }
    function pickerRows(fields) {
        return fields.map(function (f, k) {
            var id = f.id || ('md-col-' + dlgCount + '-' + k), v = f.value || '';
            return '<div class="form-group md-colour-row"><label class="form-label" for="' + id + '">' + esc(f.label) + '</label>' +
                '<div class="md-colour-pair"><input type="color" class="md-colour-pick" aria-label="' + esc(f.label) + ' Picker" data-for="' + id + '" value="' + esc(/^#[0-9a-f]{6}$/i.test(v) ? v : (f.fallback || '#000000')) + '">' +
                '<input class="form-input" id="' + id + '" data-key="' + esc(f.key) + '" value="' + esc(v) + '" placeholder="' + esc(f.placeholder || '') + '" autocomplete="off" spellcheck="false"' + (f.help ? ' aria-describedby="' + id + '-help"' : '') + '></div>' +
                (f.help ? '<p class="form-help" id="' + id + '-help">' + esc(f.help) + '</p>' : '') + '</div>';
        }).join('');
    }
    function wirePickers(body) {
        body.querySelectorAll('.md-colour-pick').forEach(function (picker) { if (picker.type !== 'color') picker.hidden = true; });
        body.addEventListener('input', function (e) {
            if (e.target.classList.contains('md-colour-pick')) { var t = body.querySelector('#' + e.target.getAttribute('data-for')); if (t) { t.value = e.target.value; } }
            else if (e.target.matches('.form-input') && /^#[0-9a-f]{6}$/i.test(e.target.value)) { var pk = body.querySelector('[data-for="' + e.target.id + '"]'); if (pk) { pk.value = e.target.value; } }
        });
    }
    /* The colour for some words: a swatch, or anything typed; '' takes the
       colour off; null is Cancel. */
    function formatPreview(opts) {
        return '<p class="form-help">' + esc(opts.empty ? 'Applies to the next words you type.' : 'Applies to the selected text.') + (opts.mixed ? ' The selection has mixed values.' : '') + '</p>' +
            '<div class="md-format-preview-boundary"><div class="doc-page deck-body md-format-preview" data-format-preview><p><span data-format-sample>' + esc((opts.sample || 'Sample text').slice(0, 240)) + '</span></p></div></div>';
    }
    function preparePreview(api, opts) {
        if (opts.scope) api.body.querySelector('[data-format-preview]').setAttribute('data-md-style', opts.scope);
        api.body.addEventListener('input',function () {api.body.querySelector('[data-format-error]').hidden=true;api.body.querySelectorAll('[aria-invalid]').forEach(function (field) {field.removeAttribute('aria-invalid');});});
    }
    function formatError(api,input,message) {
        var error=api.body.querySelector('[data-format-error]');error.id=input.id+'-error';error.textContent=message;error.hidden=false;
        input.setAttribute('aria-invalid','true');input.setAttribute('aria-describedby',error.id);input.focus();
    }
    async function askColour(opts) {
        opts = opts || {}; await styles();
        var value = opts.value || '', preset = SWATCHES.some(function (s) { return s[0] === value; }), id = 'md-format-' + (++dlgCount);
        var initial = opts.mixed ? '@mixed' : !value ? '' : preset ? value : '@custom';
        var body = formatPreview(opts) + '<div class="form-group"><label class="form-label" for="' + id + '">Text Color</label><span class="form-select-wrap"><select class="form-select" id="' + id + '" data-format-choice>' +
            (opts.mixed ? '<option value="@mixed">Mixed colors (unchanged)</option>' : '') + '<option value="">Automatic (remove explicit color)</option>' +
            SWATCHES.map(function (s) { return '<option value="' + s[0] + '">Theme ' + s[1] + '</option>'; }).join('') + '<option value="@custom">Custom Color</option></select></span></div>' +
            '<div data-format-custom>' + pickerRows([{ key: 'custom', label: 'Custom Color', value: preset ? '' : value, fallback: '#000000', placeholder: '#336699 or a CSS color name' }]) + '</div><p class="form-error" data-format-error role="alert" hidden></p>';
        return ownForm({ title: 'Text Color', body: body, okLabel: 'Apply', signal: opts.signal, current: opts.current, setup: function (api) {
            var choice = api.body.querySelector('[data-format-choice]'), input = api.body.querySelector('[data-key="custom"]'); choice.value = initial;
            preparePreview(api, opts); wirePickers(api.body);
            function refresh() { api.body.querySelector('[data-format-custom]').hidden = choice.value !== '@custom'; var color = choice.value === '@custom' ? input.value.trim() : choice.value; api.body.querySelector('[data-format-sample]').style.color = color.charAt(0) === '@' ? '' : swatchCSS(color); }
            api.body.addEventListener('input', refresh); api.body.addEventListener('change', refresh); refresh();
        }, submit: function (api) {
            var value = api.body.querySelector('[data-format-choice]').value;
            if (value === '@mixed') return null;
            if (value === '@custom') {
                value = api.body.querySelector('[data-key="custom"]').value.trim();
                if (!value || /[{};\r\n]/.test(value) || !CSS.supports('color', value)) { formatError(api,api.body.querySelector('[data-key="custom"]'),'Choose a valid color or select Automatic.'); return undefined; }
            }
            return value;
        } });
    }
    /* A class of the document's stylesheet: one of those it defines, or
       any name; '' takes the class off; null is Cancel. */
    async function askClass(names, opts) {
        opts = opts || {}; await styles();
        var blockClasses = ['center', 'right', 'justify', 'indent', 'no-break'];
        var own = Array.from(new Set((names || []).filter(function (name) { return /^[A-Za-z_][A-Za-z0-9_-]{0,63}$/.test(name) && !blockClasses.includes(name); }))).sort();
        var choices = [['', 'No Text Style'], ['small', 'Smaller Text'], ['lead', 'Larger Text']].concat(own.filter(function (name) { return name !== 'small' && name !== 'lead'; }).map(function (name) { return [name, name]; }));
        var value = opts.value || '', initial = opts.mixed ? '@mixed' : choices.some(function (c) { return c[0] === value; }) ? value : '@custom', id = 'md-format-' + (++dlgCount);
        var body = formatPreview(opts) + '<div class="form-group"><label class="form-label" for="' + id + '">Text Style</label><span class="form-select-wrap"><select class="form-select" id="' + id + '" data-format-choice>' +
            (opts.mixed ? '<option value="@mixed">Mixed styles (unchanged)</option>' : '') + choices.map(function (choice) { return '<option value="' + esc(choice[0]) + '">' + esc(choice[1]) + '</option>'; }).join('') + '<option value="@custom">Custom CSS Class</option></select></span><p class="form-help">Styles available for this text are listed here. Paragraph layout is separate.</p></div>' +
            '<div class="form-group" data-format-custom><label class="form-label" for="' + id + '-custom">CSS Class Name</label><input class="form-input" id="' + id + '-custom" data-format-class autocomplete="off" spellcheck="false" value="' + esc(value) + '"><p class="form-help">Use one class defined in the active stylesheet.</p></div><p class="form-error" data-format-error role="alert" hidden></p>';
        return ownForm({ title: 'Text Style', body: body, okLabel: 'Apply', signal: opts.signal, current: opts.current, setup: function (api) {
            var choice = api.body.querySelector('[data-format-choice]'); choice.value = initial; preparePreview(api, opts);
            function refresh() { api.body.querySelector('[data-format-custom]').hidden = choice.value !== '@custom'; var name = choice.value === '@custom' ? api.body.querySelector('[data-format-class]').value.trim() : choice.value; api.body.querySelector('[data-format-sample]').className = /^[A-Za-z_][A-Za-z0-9_-]{0,63}$/.test(name) ? name : ''; }
            api.body.addEventListener('input', refresh); api.body.addEventListener('change', refresh); refresh();
        }, submit: function (api) {
            var value = api.body.querySelector('[data-format-choice]').value;
            if (value === '@mixed') return null;
            if (value === '@custom') value = api.body.querySelector('[data-format-class]').value.trim();
            if (value && (!/^[A-Za-z_][A-Za-z0-9_-]{0,63}$/.test(value) || blockClasses.includes(value))) { formatError(api,api.body.querySelector('[data-format-class]'),'Use one text-style class name. Alignment, indentation and page layout classes cannot be applied to words.'); return undefined; }
            return value;
        } });
    }
    /* Several colours at once, by key: { key, label, value, fallback,
       placeholder }; resolves to the values by key, or null. */
    function askColours(title, fields, opts) {
        opts = opts || {};
        return ownForm({ title: title, body: pickerRows(fields), signal: opts.signal, current: opts.current,okLabel:'Apply',
            setup: function (api) { wirePickers(api.body); }, submit: function (api) {
                var out = {},bad=null;
                api.body.querySelectorAll('[data-key]').forEach(function (i) {var value=i.value.trim();out[i.getAttribute('data-key')]=value;if(value && (/[{};\r\n]/.test(value) || !CSS.supports('color',swatchCSS(value))))bad=bad || i;});
                if(bad){window.yavcUtil.notice(bad.closest('.form-group'),'Enter a CSS color or leave it empty for the theme default.',true);bad.focus();return undefined;}
                return out;
            } });
    }

    /* A form in the site's dialog: the kit's own fields as the body, a
       Remove button drawn as destructive when something exists to remove,
       Cancel, and the action. setup(api) wires the body once it shows;
       submit(api) gives the answer, or undefined to keep the dialog open
       (it shows its own problem). Resolves to the answer, '' for Remove, or
       null for Cancel. */
    function ownForm(o) {
        if (o.signal && o.signal.aborted || o.current && !o.current()) { return Promise.resolve(null); }
        var answer, api = null;
        function abort() { if (api) { api.close('cancel'); } }
        dlgCount++;
        return window.yavcDialog.open({
            title: o.title, className: 'md-editor-dialog ' + (o.className || ''), body: o.body, signal: o.signal,
            buttons: (o.remove ? [{ id: 'remove', label: 'Remove', danger: true, submit: false }] : []).concat([{ id: 'cancel', label: 'Cancel' }, { id: 'ok', label: o.okLabel || 'OK', primary: true }]),
            onOpen: function (a) {
                api = a;api.dialog.querySelector('form').noValidate=true;
                api.body.addEventListener('input',function (event) {api.body.querySelectorAll('[data-own-validation]').forEach(function (notice) {notice.hidden=true;});if(event.target.dataset.ownInvalid){event.target.removeAttribute('aria-invalid');delete event.target.dataset.ownInvalid;}});
                if (o.signal) { o.signal.addEventListener('abort', abort, { once: true }); } if (o.setup) { o.setup(a); } if (o.signal && o.signal.aborted) { abort(); }
            },
            check: function (v) {
                if (v !== 'ok') { return null; }
                if (o.current && !o.current()) { window.yavcUtil.notice(api.body, 'The document changed. Close and reopen this form.', true); return 'kept open'; }
                var invalid=Array.from(api.body.querySelectorAll('input,select,textarea')).find(function (input) {
                    if(input.matches(':disabled'))return false;
                    var group=input.closest('.form-group'),label=group && group.querySelector('label'),visible=label && label.control || input;
                    return input.willValidate && !input.validity.valid || input.type==='number' && visible.getAttribute('aria-invalid')==='true';
                });
                if(invalid) {
                    var group=invalid.closest('.form-group'),label=group && group.querySelector('label'),visible=label && label.control || invalid;
                    var notice=window.yavcUtil.notice(group || invalid,invalid.validationMessage || 'Enter a valid value for '+(label?label.textContent:'this field')+'.',true);
                    if(invalid.type!=='number'){invalid.setAttribute('aria-invalid','true');invalid.dataset.ownInvalid='true';}
                    if(notice)notice.setAttribute('data-own-validation','');visible.focus();return 'kept open';
                }
                var out = o.submit(api);
                if (out === undefined) { return 'kept open'; }
                answer = out;
                return null;
            }
        }).then(function (r) { if (o.current && !o.current()) { return null; } return r.value === 'ok' ? answer : r.value === 'remove' ? '' : null; }).finally(function () { if (o.signal) { o.signal.removeEventListener('abort', abort); } });
    }

    /* === Pictures ========================================================
       A picture in a document is a {{figure}} shortcode (or a plain
       Markdown image), asked for and changed as a form: its address, with
       Browse for one in the home folder or on the site and Upload for one
       from the computer, what it shows, its largest width and a caption
       (DESIGN.md, decision 14). A picture uploaded goes beside the document
       when the document is saved under ~, else into ~/Pictures, and is
       written as a path the page reads from the home folder. */

    /* A shortcode's call taken apart: {{name:positional key=value
       key="a value"}}, with \" inside quotes, as the engine reads it. */
    function parseShortcode(src) {
        var m = /^\{\{(\/?)([a-z][a-z0-9-]*)(?::([\s\S]*?))?\}\}$/.exec((src || '').trim());
        if (!m) { return null; }
        var out = { name: m[2], args: (m[3] || '').trim(), positional: [], named: {} }, t = m[3] || '', i = 0;
        while (i < t.length) {
            while (i < t.length && /\s/.test(t[i])) { i++; }
            if (i >= t.length) { break; }
            var key = null, start = i;
            while (i < t.length && /[A-Za-z0-9_-]/.test(t[i])) { i++; }
            if (t[i] === '=' && i > start) { key = t.slice(start, i); i++; } else { i = start; }
            var value = '';
            if (t[i] === '"') {
                i++;
                while (i < t.length && t[i] !== '"') { if (t[i] === '\\' && i + 1 < t.length) { i++; } value += t[i]; i++; }
                i++;
            } else {
                while (i < t.length && !/\s/.test(t[i])) { value += t[i]; i++; }
            }
            if (key === null) { out.positional.push(value); } else { out.named[key] = value; }
        }
        return out;
    }
    function quoteArg(v) { return /[\s"]/.test(v) || v === '' ? '"' + v.replace(/(["\\])/g, '\\$1') + '"' : v; }
    function buildFigure(f) {
        var parts = ['{{figure:' + quoteArg(f.src), 'alt=' + quoteArg(f.alt || '')];
        if (f.width) { parts.push('width=' + quoteArg(f.width)); }
        if (f.caption) { parts.push('caption=' + quoteArg(f.caption)); }
        return parts.join(' ') + '}}';
    }
    /* The picture a block is, as { src, alt, width, caption }, or null for
       a block that is not one picture: a {{figure}} on its own, or a
       Markdown image on its own. */
    function parseFigure(src) {
        var t = (src || '').trim();
        if (!t || t.indexOf('\n') >= 0) { return null; }
        var sc = parseShortcode(t);
        if (sc) {
            if (sc.name !== 'figure') { return null; }
            return { src: sc.named.src || sc.positional[0] || '', alt: sc.named.alt || '', width: sc.named.width || '', caption: sc.named.caption || '' };
        }
        var m = /^!\[([^\]]*)\]\(\s*<?([^\s)>]+)>?(?:\s+"[^"]*")?\s*\)$/.exec(t);
        if (m) { return { src: m[2], alt: m[1], width: '', caption: '' }; }
        return null;
    }
    var PICTURE_EXT = /\.(png|jpe?g|gif|webp|svg|avif|bmp)$/i;
    /* A width such as 80% or 600px as its number and its unit, for the
       dialog's number field and unit choice; % when there is none. */
    var WIDTH_UNITS = ['%', 'px', 'rem', 'em', 'ch'];
    function splitWidth(w) {
        var m = /^(\d+(?:\.\d+)?)\s*(%|px|rem|em|ch)?$/.exec((w || '').trim());
        return m ? { n: m[1], u: m[2] || '%' } : { n: '', u: '%' };
    }
    /* How a picture is written in the document: by name when it sits in
       the document's folder, by its ~ path elsewhere in the home folder,
       and by its address on the site otherwise. */
    function addressOf(F, node, dirPath) {
        var display = F.displayPath(node), dir = dirPath ? dirPath.replace(/\/$/, '') + '/' : null;
        if (dir && display.indexOf(dir) === 0) { return display.slice(dir.length); }
        if (display.indexOf('~/') === 0) { return display; }
        var url = F.publicUrl(node);
        if (url) { try { return new URL(url).pathname; } catch (err) { } }
        return display;
    }
    /* The picture dialog: a promise of the {{figure}} to write, '' to take
       the picture out, or null for no change. existing is the block being
       changed, or null for a new picture. */
    function askFigure(existing, pictures) {
        var owner = pictures && pictures.identity && pictures.identity(), source = pictures && pictures.text && pictures.text();
        function current() { return !(pictures && pictures.signal && pictures.signal.aborted) && (!pictures || (!pictures.identity || pictures.identity() === owner) && (!pictures.text || pictures.text() === source)); }
        var f = existing ? parseFigure(existing) : null, id = 'md-fig-' + (dlgCount + 1) + '-', width = splitWidth(f ? f.width : '');
        var body = '<div class="md-figure-fields">' +
            '<div class="form-group"><label class="form-label" for="' + id + 'src">Picture</label>' +
            '<div class="yavc-path-field"><input class="form-input" id="' + id + 'src" data-key="src" data-file-path data-accept=".png,.jpg,.jpeg,.gif,.webp,.svg" value="' + esc(f ? f.src : '') + '" autocomplete="off" spellcheck="false" placeholder="cat.png, ~/Pictures/cat.png or https://…" required>' +
            (pictures ? '<button type="button" class="btn" data-browse-for="' + id + 'src">Browse…</button>' : '') + '</div>' +
            '<p class="form-help">A picture beside the document, one in your home folder, one of the site\'s, or an address on the web.</p></div>' +
            '<div class="form-group"><label class="form-label" for="' + id + 'alt">What It Shows</label>' +
            '<input class="form-input" id="' + id + 'alt" data-key="alt" value="' + esc(f ? f.alt : '') + '" autocomplete="off">' +
            '<p class="form-help">For someone who can\'t see it. Leave it empty for a picture that only decorates.</p></div>' +
            '<div class="form-group"><label class="form-label" for="' + id + 'width">Largest Width</label>' +
            '<div class="md-figure-width"><span class="number-field"><input class="form-input" type="number" id="' + id + 'width" data-key="width" value="' + esc(width.n) + '" min="1" step="1" placeholder="Column"></span>' +
            '<span class="form-select-wrap"><select class="form-select" data-key="unit" aria-label="Unit of the Width">' + WIDTH_UNITS.map(function (u) { return '<option value="' + u + '"' + (u === width.u ? ' selected' : '') + '>' + u + '</option>'; }).join('') + '</select></span></div>' +
            '<p class="form-help">Of the column (%), or in px, rem, em or ch; empty takes the column\'s width.</p></div>' +
            '<div class="form-group"><label class="form-label" for="' + id + 'caption">Caption</label>' +
            '<input class="form-input" id="' + id + 'caption" data-key="caption" value="' + esc(f ? f.caption : '') + '" autocomplete="off">' +
            '<p class="form-help">Under the picture, in Markdown.</p></div>' +
            '<p class="form-error" data-role="error" hidden></p>' +
            '</div>';
        var dlg = null, errorLine = null;
        function field(k) { return dlg.querySelector('[data-key="' + k + '"]'); }
        function problem(text, focus) { errorLine.textContent = text; errorLine.hidden = false; if (focus) { focus.focus(); } }
        return ownForm({
            title: f ? 'Change the Picture' : 'Insert a Picture', className: 'md-figure', body: body, remove: !!f, okLabel: f ? 'Change' : 'Insert', signal:pictures && pictures.signal,current:current,
            setup: function (api) {
                dlg = api.dialog; errorLine = dlg.querySelector('[data-role="error"]');
                window.yavcDialog.wirePaths(api,{signal:pictures && pictures.signal,current:current});
                if (f) { field('caption').focus(); }
            },
            submit: function () {
                var src = field('src').value.trim(), number = field('width').value.trim(), unit = field('unit').value;
                if (!src) { problem('The picture needs an address, such as cat.png or ~/Pictures/cat.png.', field('src')); return undefined; }
                if (number && !(parseFloat(number) > 0)) { problem('The width is a number above zero, such as 80.', field('width')); return undefined; }
                return buildFigure({ src: src, alt: field('alt').value.trim(), width: number ? String(parseFloat(number)) + unit : '', caption: field('caption').value.trim() });
            }
        });
    }
    /* A drawn block pressed on the page: a picture opens its dialog, and
       resolves to the block's new source, '' to take it out, or null to
       leave it; any other block is undefined, for the page to edit as its
       Markdown. */
    function editBlock(src, pictures) {
        if (parseFigure(src)) { return askFigure(src, pictures); }
        if (window.yavcMacroTools && pictures && pictures.macroContext) {
            var custom = window.yavcMacroTools.editBlock(src, pictures.macroContext(), pictures);
            if (custom !== undefined) { return custom; }
        }
        /* A shortcode the host application draws has the host's form. */
        var hosted = editHostBlock(src, pictures);
        if (hosted !== undefined) { return hosted; }
        var sc = parseShortcode((src || '').trim());
        if (sc && FORMS[sc.name] && (src || '').trim().indexOf('\n') < 0) { return askShortcode(sc.name, src.trim(), pictures); }
        return undefined;
    }

    /* === Forms for the standalone shortcodes ==============================
       Each is { title, inline, fields, read(sc) -> values, write(values) ->
       the shortcode or { problem, field } }. A field is { key, label, kind
       (text, select, number), help, placeholder, options (a list of [value,
       label], or a function of the host giving a promise of one), min, max,
       browse (a picker for a file) }. */
    function plain(v) { return String(v == null ? '' : v).trim().replace(/[{}\n]/g, ''); }
    var STATUS = [['green', 'Green, Done or Good'], ['blue', 'Blue, Information'], ['yellow', 'Yellow, a Warning'], ['red', 'Red, Failed or Blocked'], ['grey', 'Gray, Neutral']];
    function appletOptions() {
        var defs = window.pudlApplets && window.pudlApplets.definitions ? window.pudlApplets.definitions() : {};
        return Promise.resolve(Object.keys(defs).sort().map(function (k) { return [k, (defs[k] && defs[k].title) || k]; }));
    }
    function glyphOptions() {
        return (window.yavcMdEngine ? window.yavcMdEngine().then(function (m) { return m.glyphs(''); }) : Promise.resolve([])).then(function (list) {
            return (list || []).map(function (g) { return [g, g]; });
        });
    }
    function pageOptions(host) {
        if (!host || !host.site) { return Promise.resolve(null); }
        return host.site().then(function (site) {
            var dir = site.F.resolve(null, '/site/pages');
            if (!dir || !dir.children) { return null; }
            return dir.children.filter(function (c) { return !c.children; }).map(function (c) { return [c.name, c.title ? c.title + ' (' + c.name + ')' : c.name]; });
        }).catch(function () { return null; });
    }
    var FORMS = {
        video: { title: 'A Hosted Video', fields: [
            { key: 'src', label: 'Link', placeholder: 'https://youtu.be/… or https://vimeo.com/…', help: 'The video\'s link from YouTube or Vimeo. YouTube plays without tracking until it is played.' },
            { key: 'title', label: 'Title', help: 'What the video is, for someone who can\'t see it.' }],
            read: function (sc) {
                var p = sc.positional, src = sc.named.src || (p.length > 1 && /^(youtube|vimeo)$/i.test(p[0]) ? p[0] + ' ' + p[1] : p[0] || '');
                return { src: src, title: sc.named.title || '' };
            },
            write: function (v) {
                var src = plain(v.src);
                if (!src) { return { problem: 'The video needs its link.', field: 'src' }; }
                return '{{video:' + src + (plain(v.title) ? ' title=' + quoteArg(plain(v.title)) : '') + '}}';
            } },
        applet: { title: 'An Applet on the Page', fields: [
            { key: 'name', label: 'Applet', kind: 'select', options: appletOptions },
            { key: 'fit', label: 'Size', kind: 'select', options: [['flow', 'As Tall as It Needs'], ['fill', 'Fills the Space It Is Given']] },
            { key: 'state', label: 'Starting State', help: 'Optional, as the applet understands it, such as a board for Conway\'s Game of Life.' }],
            read: function (sc) { return { name: sc.named.name || sc.positional[0] || '', fit: sc.named.fit || 'flow', state: sc.named.state || '' }; },
            write: function (v) {
                var name = plain(v.name);
                if (!/^[a-z][a-z0-9-]*$/.test(name)) { return { problem: 'Choose an applet.', field: 'name' }; }
                return '{{applet:' + name + ' fit=' + (v.fit === 'fill' ? 'fill' : 'flow') + (plain(v.state) ? ' state=' + quoteArg(plain(v.state)) : '') + '}}';
            } },
        diagram: { title: 'A Diagram from a File', fields: [
            {key:'src',label:'Diagram File',browse:true,maxBytes:2*1024*1024,accept:'.d2,.mmd,.mermaid,.canvas',placeholder:'~/diagrams/plan.canvas',help:'Choose a saved D2, Mermaid or Canvas file from this host, or upload one.'},
            {key:'alt',label:'Description',help:'What the diagram shows, for readers who cannot see the image.'},
            {key:'width',label:'Width (%)',kind:'number',min:1,max:100,help:'Leave empty to use the available space, up to the image size.'},
            {key:'caption',label:'Caption'}],
            read:function(sc){return {src:sc.named.src||sc.positional[0]||'',alt:sc.named.alt||'',width:(sc.named.width||'').replace(/%$/,''),caption:sc.named.caption||''};},
            write:function(v){var src=plain(v.src);if(src.length>4096||!/\.(d2|mmd|mermaid|canvas)$/i.test(src)||/[:\\?#\u0000-\u001f\u007f]/.test(src)||src.startsWith('//')||src.startsWith('~')&&!src.startsWith('~/'))return {problem:'Choose a local D2, Mermaid or Canvas file.',field:'src'};
                var width=plain(v.width);if(width&&(!/^\d+$/.test(width)||+width<1||+width>100))return {problem:'Use a whole percentage from 1 to 100.',field:'width'};
                return '{{diagram:'+quoteArg(src)+(plain(v.alt)?' alt='+quoteArg(plain(v.alt)):'')+(width?' width='+width+'%':'')+(plain(v.caption)?' caption='+quoteArg(plain(v.caption)):'')+'}}';}
        },
        code: { title: 'A Code Listing from a File', fields: [
            { key: 'src', label: 'File', placeholder: '~/code/example.js', browse: true, maxBytes: 256 * 1024, help: 'A file accepted by this host; the page shows it as it is when the page is read.' },
            { key: 'lang', label: 'Language', placeholder: 'From the file name', help: 'Optional, such as javascript or go, for syntax coloring.' }],
            read: function (sc) { return { src: sc.named.src || sc.positional[0] || '', lang: sc.named.lang || '' }; },
            write: function (v) {
                var src = plain(v.src);
                if (!/^(~\/|\/(?!\/))/.test(src) || src.indexOf('..') >= 0) { return { problem: 'The file is a path from / or under ~, such as ~/bin/hello.', field: 'src' }; }
                return '{{code:' + quoteArg(src) + (plain(v.lang) ? ' lang=' + quoteArg(plain(v.lang)) : '') + '}}';
            } },
        toc: { title: 'Contents', fields: [
            { key: 'min', label: 'From Heading Level', kind: 'number', min: 1, max: 6 },
            { key: 'max', label: 'To Heading Level', kind: 'number', min: 1, max: 6, help: 'The contents list the headings from the one level to the other, linked.' }],
            read: function (sc) { return { min: sc.named.min || '2', max: sc.named.max || '3' }; },
            write: function (v) {
                var a = parseInt(v.min, 10) || 2, b = parseInt(v.max, 10) || 3;
                if (b < a) { return { problem: 'The last level is at or below the first.', field: 'max' }; }
                return a === 2 && b === 3 ? '{{toc}}' : '{{toc:min=' + a + ' max=' + b + '}}';
            } },
        status: { title: 'A Status Label', inline: true, fields: [
            { key: 'colour', label: 'Color', kind: 'select', options: STATUS },
            { key: 'label', label: 'Label', placeholder: 'Done' }],
            read: function (sc) { var c = (sc.positional[0] || 'green').toLowerCase(); return { colour: c === 'gray' ? 'grey' : c === 'amber' ? 'yellow' : c, label: sc.args.slice((sc.positional[0] || '').length).trim() }; },
            write: function (v) {
                var label = plain(v.label);
                if (!label) { return { problem: 'The label needs some words.', field: 'label' }; }
                return '{{status:' + (v.colour || 'green') + ' ' + label + '}}';
            } },
        kbd: { title: 'Keys', inline: true, fields: [
            { key: 'keys', label: 'Keys', placeholder: 'Ctrl+Shift+P', help: 'The keys to press together, joined by +.' }],
            read: function (sc) { return { keys: sc.args }; },
            write: function (v) {
                var keys = plain(v.keys);
                if (!keys) { return { problem: 'Name the keys, such as Ctrl+C.', field: 'keys' }; }
                return '{{kbd:' + keys + '}}';
            } },
        glyph: { title: 'A Glyph', inline: true, fields: [
            { key: 'name', label: 'Glyph', kind: 'glyph', options: glyphOptions, help: 'Choose a symbol. Each preview is shown beside its name.' }],
            read: function (sc) { return { name: sc.named.name || sc.positional[0] || '' }; },
            write: function (v) { return plain(v.name) ? '{{glyph:' + plain(v.name) + '}}' : { problem: 'Choose a glyph.', field: 'name' }; } },
        page: { title: 'A Link to a Page', inline: true, fields: [
            { key: 'slug', label: 'Page', kind: 'select', options: pageOptions, placeholder: 'about' },
            { key: 'text', label: 'Text', help: 'The words of the link; the page\'s name if empty.' }],
            read: function (sc) {
                var slug = sc.named.slug || sc.positional[0] || '';
                return { slug: slug, text: sc.named.text || (sc.positional.length > 1 ? sc.args.slice(slug.length).trim() : '') };
            },
            write: function (v) {
                var slug = plain(v.slug);
                if (!/^[a-z0-9][a-z0-9-]*(\/[a-z0-9][a-z0-9-]*)*$/.test(slug)) { return { problem: 'Choose a page.', field: 'slug' }; }
                return '{{page:' + slug + (plain(v.text) ? ' text=' + quoteArg(plain(v.text)) : '') + '}}';
            } },
        anchor: { title: 'An Anchor', inline: true, fields: [
            { key: 'name', label: 'Name', placeholder: 'section-two', help: 'A link to #name goes here. Letters, digits, - and _, starting with a letter.' }],
            read: function (sc) { return { name: sc.named.name || sc.positional[0] || '' }; },
            write: function (v) {
                var name = plain(v.name);
                return /^[A-Za-z][A-Za-z0-9_-]*$/.test(name) ? '{{anchor:' + name + '}}' : { problem: 'An anchor\'s name is letters, digits, - and _, starting with a letter.', field: 'name' };
            } }
    };
    /* A shortcode's form: a promise of the shortcode to write, '' to take
       it out, or null for no change. existing is the shortcode being
       changed, or null for a new one. host is where files are, for Browse
       and for the site's pages (as for the picture dialog). */
    function askShortcode(name, existing, host, customForm) {
        var form = customForm || FORMS[name];
        if (!form) { return Promise.resolve(null); }
        var owner = host && host.identity && host.identity(), openingSource = host && host.text && host.text();
        function unchanged() { return (!form.current || form.current()) && !(host && host.signal && host.signal.aborted) && (!host || (!host.identity || host.identity() === owner) && (!host.text || host.text() === openingSource)); }
        var sc = existing ? parseShortcode(existing) : null;
        var values = form.read(sc || { name: name, args: '', positional: [], named: {} });
        var id = 'md-sc-' + (dlgCount + 1) + '-';
        var body = '<div class="md-form-fields">' + form.fields.map(function (f) {
                var fid = id + f.key, v = values[f.key] == null ? '' : values[f.key], help = f.help ? '<p class="form-help">' + esc(f.help) + '</p>' : '';
                var control;
                if (f.kind === 'glyph') { control = '<input type="hidden" data-key="' + f.key + '" value="' + esc(v) + '"><input type="search" class="form-input" id="' + fid + '" aria-label="Search Glyphs" aria-describedby="' + id + 'error" data-glyph-search placeholder="Filter by name"><div class="md-glyph-options" role="listbox" tabindex="0" aria-label="Glyphs" aria-describedby="' + id + 'error"><p data-glyph-loading>Loading glyphs…</p></div><p class="form-help" data-glyph-empty hidden>No matching glyphs.</p>'; }
                else if (f.kind === 'select') { control = '<span class="form-select-wrap"><select class="form-select" id="' + fid + '" data-key="' + f.key + '" data-value="' + esc(v) + '"><option value="' + esc(v) + '">' + esc(v || 'Loading…') + '</option></select></span>'; }
                else if (f.kind === 'number') { control = '<span class="number-field"><input class="form-input" type="number" id="' + fid + '" data-key="' + f.key + '" value="' + esc(v) + '" min="' + f.min + '" max="' + f.max + '" step="1"></span>'; }
                else { control = '<input class="form-input" id="' + fid + '" data-key="' + f.key + '" value="' + esc(v) + '" autocomplete="off" spellcheck="false"' + (f.placeholder ? ' placeholder="' + esc(f.placeholder) + '"' : '') + '>'; }
                if (f.browse && host && host.site) { control = '<div class="yavc-path-field">' + control.replace('data-key=', 'data-file-path' + (f.accept ? ' data-accept="' + esc(f.accept) + '"' : '') + (f.maxBytes ? ' data-max-bytes="' + f.maxBytes + '"' : '') + ' data-key=') + '<button type="button" class="btn" data-browse-for="' + fid + '">Browse…</button></div>'; }
                return '<div class="form-group"><label class="form-label" for="' + fid + '">' + esc(f.label) + '</label>' + control + help + '</div>';
            }).join('') + '<p class="form-error" id="' + id + 'error" data-role="error" role="alert" hidden></p></div>';
        var dlg = null, errorLine = null;
        function field(k) { return dlg.querySelector('[data-key="' + k + '"]'); }
        return ownForm({
            signal: form.signal || host && host.signal, current: unchanged,
            title: existing ? 'Change ' + form.title.replace(/^(A|An) /, 'the ') : 'Insert ' + form.title.replace(/^(A|An) /, 'a ').replace(/^a a/, 'an a'),
            className: 'md-form', body: body, remove: !!existing, okLabel: existing ? 'Change' : 'Insert',
            setup: function (api) {
                dlg = api.dialog; errorLine = dlg.querySelector('[data-role="error"]');
                window.yavcDialog.wirePaths(api,{signal:form.signal || host && host.signal,current:unchanged});
                if (name === 'glyph') {
                    var choices = dlg.querySelector('.md-glyph-options'), search = dlg.querySelector('[data-glyph-search]');
                    glyphOptions().then(function (list) {
                        if (!dlg.open || !unchanged()) { return; }
                        choices.replaceChildren();
                        list.forEach(function (option) {
                            var key = option[0]; if (!/^[a-z][a-z0-9-]*$/.test(key)) { return; }
                            var button = document.createElement('div'), glyph = document.createElement('span'), label = document.createElement('span');
                            button.className = 'md-glyph-row'; button.dataset.glyphChoice = key;button.id=id+'glyph-'+key;button.setAttribute('role','option');button.setAttribute('aria-selected',String(field('name').value===key));
                            glyph.className = 'glyph'; glyph.style.setProperty('--glyph', 'var(--glyph-' + key + ')'); glyph.setAttribute('aria-hidden', 'true');
                            label.textContent = option[1]; button.append(glyph, label); choices.append(button);
                        });
                        filterGlyphs();
                    }).catch(function () { window.yavcUtil.notice(api.body, 'The glyph list could not be loaded. Close and reopen the dialog to retry.', true); });
                    function filterGlyphs() {
                        var query = search.value.trim().toLowerCase(), visible = 0;
                        choices.querySelectorAll('[data-glyph-choice]').forEach(function (button) { button.hidden = !button.dataset.glyphChoice.includes(query) && !button.textContent.toLowerCase().includes(query); if (!button.hidden) { visible++; } });
                        dlg.querySelector('[data-glyph-empty]').hidden = visible > 0;
                        var selected=choices.querySelector('[aria-selected="true"]:not([hidden])');if(selected)choices.setAttribute('aria-activedescendant',selected.id);else choices.removeAttribute('aria-activedescendant');
                    }
                    search.addEventListener('input', filterGlyphs);
                    function choose(row) {
                        choices.querySelectorAll('[data-glyph-choice]').forEach(function (item) {item.setAttribute('aria-selected',String(item===row));});
                        field('name').value=row.dataset.glyphChoice;choices.setAttribute('aria-activedescendant',row.id);errorLine.hidden=true;
                        var outer=choices.getBoundingClientRect(),inner=row.getBoundingClientRect();if(inner.top<outer.top)choices.scrollTop-=outer.top-inner.top;else if(inner.bottom>outer.bottom)choices.scrollTop+=inner.bottom-outer.bottom;
                    }
                    choices.addEventListener('click',function (event) {var row=event.target.closest('[data-glyph-choice]');if(row && choices.contains(row) && !row.hidden){choose(row);choices.focus();}});
                    var typed='',typedAt=0;
                    choices.addEventListener('keydown',function (event) {
                        if(event.altKey || event.ctrlKey || event.metaKey)return;
                        var rows=Array.from(choices.querySelectorAll('[data-glyph-choice]:not([hidden])'));if(!rows.length)return;
                        var at=rows.findIndex(function (row) {return row.getAttribute('aria-selected')==='true';}),next;
                        if(event.key==='ArrowDown')next=Math.min(rows.length-1,at+1);else if(event.key==='ArrowUp')next=Math.max(0,at-1);else if(event.key==='Home')next=0;else if(event.key==='End')next=rows.length-1;
                        else if(event.key==='PageDown')next=Math.min(rows.length-1,Math.max(0,at)+Math.max(1,Math.floor(choices.clientHeight/rows[0].offsetHeight)));else if(event.key==='PageUp')next=Math.max(0,at-Math.max(1,Math.floor(choices.clientHeight/rows[0].offsetHeight)));
                        else if(event.key===' '){next=Math.max(0,at);}else if(event.key==='Enter'){event.preventDefault();api.dialog.querySelector('button[value="ok"]').click();return;}
                        else if(event.key.length===1){typed=Date.now()-typedAt>700?'':typed;typed+=event.key.toLowerCase();typedAt=Date.now();next=rows.findIndex(function (row) {return row.dataset.glyphChoice.startsWith(typed);});if(next<0)return;}else return;
                        if(event.key.length!==1 || event.key===' ')typed='';event.preventDefault();choose(rows[next]);
                    });
                }
                /* A choice fills from its list; a value it does not list is kept as a choice of its own. */
                form.fields.forEach(function (f) {
                    if (f.kind !== 'select') { return; }
                    var sel = field(f.key), want = sel.getAttribute('data-value') || '';
                    Promise.resolve(typeof f.options === 'function' ? f.options(host) : f.options).then(function (list) {
                        if (!list) {
                            /* No list to choose from: a text field takes the choice's place. */
                            var input = document.createElement('input');
                            input.className = 'form-input'; input.id = sel.id; input.setAttribute('data-key', f.key); input.value = want; input.autocomplete = 'off'; input.spellcheck = false;
                            if (f.placeholder) { input.placeholder = f.placeholder; }
                            (sel.closest('.form-select-wrap') || sel).replaceWith(input);
                            return;
                        }
                        var opts = list.slice();
                        if (want && !opts.some(function (o) { return o[0] === want; })) { opts.unshift([want, want]); }
                        sel.innerHTML = (want ? '' : '<option value="">Choose…</option>') + opts.map(function (o) { return '<option value="' + esc(o[0]) + '"' + (o[0] === want ? ' selected' : '') + '>' + esc(o[1]) + '</option>'; }).join('');
                    }).catch(function () { });
                });
            },
            submit: function () {
                var v = {};
                form.fields.forEach(function (f) { var el = field(f.key); v[f.key] = el ? el.value : ''; });
                var out = form.write(v);
                if (typeof out !== 'string') { errorLine.textContent = out.problem; errorLine.hidden = false; var bad = name === 'glyph' ? dlg.querySelector('[data-glyph-search]') : field(out.field); if (bad) { bad.focus(); } return undefined; }
                return out;
            }
        });
    }
    /* === Shortcodes the host draws =========================================
       A host application may draw shortcodes of its own
       (docs/content-workspace-contract.md, shortcodeHost). The engine writes
       a placeholder where one is (internal/shortcodes/host.go); the page asks
       the host for the drawing and puts it in the placeholder. A drawing is
       kept in pictures.hostFills, a Map the workspace owns, so a placeholder
       drawn again is filled at once and nothing on the page moves. Until a
       drawing arrives, and where the host draws none, an inline or block
       shortcode shows as it is written and a container shows its body. */
    var HOST_FILL_LIMIT = 512, HOST_FILL_UNITS = 8 * 1024 * 1024, HOST_FILL_AGE = 5 * 60 * 1000, HOST_FILL_RETRY = 30 * 1000, HOST_CALLS = 6, HOST_PENDING = 256, HOST_CALL_WAIT = 20 * 1000, HOST_HTML_LIMIT = 512 * 1024;
    var hostQueues = new WeakMap();
    var hostOwners = new WeakMap(), hostOwnerSequence = 0;
    function hostService(pictures) {
        var s = pictures && pictures.shortcodeHost;
        return s && s.version === 1 && typeof s.render === 'function' && pictures.hostFills instanceof Map ? s : null;
    }
    function hostLive(pictures) { return !(pictures.signal && pictures.signal.aborted) && !(pictures.current && !pictures.current()); }
    function hostContext(pictures, signal) { return { resource: pictures.resource || null, host: pictures.root, signal: signal, current: function () { return !signal.aborted && hostLive(pictures); } }; }
    function hostKey(pictures, request) { var owner=pictures.owner,key=0;if(owner&&typeof owner==='object'){if(!hostOwners.has(owner))hostOwners.set(owner,++hostOwnerSequence);key=hostOwners.get(owner);}return JSON.stringify([pictures.resource || '', key, request.kind, request.name, request.source]); }
    /* What a placeholder asks the host to draw. */
    function hostRequest(el) {
        var name = el.getAttribute('data-host-shortcode'), kind = el.getAttribute('data-host-kind');
        var args = el.getAttribute('data-host-args') || '', source = el.getAttribute('data-host-source') || '';
        if (!/^[a-z][a-z0-9-]{0,63}$/.test(name || '') || !/^(inline|block|container|span)$/.test(kind) || args.length > 65536 || source.length > 65536) { return null; }
        return { name: name, kind: kind, args: args, source: source };
    }
    /* The same for a span the page view keeps as a mark: its name and its
       opening as it is written. */
    function hostSpanRequest(name, open) {
        if (typeof name !== 'string' || !/^[a-z][a-z0-9-]{0,63}$/.test(name) || typeof open !== 'string' || open.length > 65536) { return null; }
        var m = /^\{\{[a-z][a-z0-9-]*(?::([\s\S]*))?\}\}$/.exec(open);
        return { name: name, kind: 'span', args: m && m[1] != null ? m[1] : '', source: open };
    }
    /* A few calls to the host at a time, each given up on after a while so
       one that never returns does not stop the rest. */
    function hostCall(store, fn, pictures) {
        var q = hostQueues.get(store);
        if (!q) { q = { active: 0, waiting: [] }; hostQueues.set(store, q); }
        return new Promise(function (resolve) {
            function run() {
                q.active++;
                var timer = null, settled = false, controller = new AbortController();
                function abort() { controller.abort(); done(null); }
                function done(value) {
                    if (settled) { return; }
                    settled = true; clearTimeout(timer); if (pictures.signal) pictures.signal.removeEventListener('abort', abort); q.active--; resolve(value);
                    var next = q.waiting.shift(); if (next) { next(); }
                }
                timer = setTimeout(abort, HOST_CALL_WAIT);
                if (pictures.signal) { if (pictures.signal.aborted) { abort(); return; } pictures.signal.addEventListener('abort', abort, { once: true }); }
                Promise.resolve().then(function () { return fn(controller.signal); }).then(done, function () { done(null); });
            }
            if (q.active < HOST_CALLS) { run(); } else if (q.waiting.length < HOST_PENDING) { q.waiting.push(run); } else { resolve(null); }
        });
    }
    function trimHostCache(store) {
        var units = 0; store.forEach(function (entry, key) { units += key.length + (entry.html || '').length; });
        while (store.size > HOST_FILL_LIMIT || units > HOST_FILL_UNITS) { var key = store.keys().next().value, entry = store.get(key); units -= key.length + (entry.html || '').length; store.delete(key); }
    }
    /* A promise of a host shortcode's drawing, as HTML, or null where the
       host draws none. A drawing kept for a while is asked for again, and
       the one kept is given until the new one arrives. */
    function hostDrawing(pictures, request) {
        var service = hostService(pictures);
        if (!service || !request || !Array.isArray(pictures.hostCatalog) || !pictures.hostCatalog.some(function(h){return h.name===request.name;})) { return Promise.resolve(null); }
        var store = pictures.hostFills, key = hostKey(pictures, request), entry = store.get(key), now = Date.now();
        if (entry) {
            if (entry.promise) { return entry.promise; }
            if (now - entry.at < (entry.html == null ? HOST_FILL_RETRY : HOST_FILL_AGE)) { return Promise.resolve(entry.html); }
        } else { entry = { html: null, at: 0, promise: null }; }
        store.delete(key); store.set(key, entry);
        trimHostCache(store);
        var gone = {};
        entry.promise = hostCall(store, function (signal) {
            if (!hostLive(pictures)) { return gone; }
            return service.render({ name: request.name, kind: request.kind, args: request.args, source: request.source }, hostContext(pictures, signal));
        }, pictures).then(function (out) {
            entry.promise = null;
            /* A document that has closed keeps nothing new. */
            if (out === gone || !hostLive(pictures)) { if (entry.html == null && store.get(key) === entry) { store.delete(key); } return entry.html; }
            var html = out && typeof out.html === 'string' && !out.error && out.html.length <= HOST_HTML_LIMIT ? out.html : null;
            if (html != null) { entry.html = html; entry.at = Date.now(); }
            else { entry.at = entry.html == null ? Date.now() : Date.now() - HOST_FILL_AGE + HOST_FILL_RETRY; }
            trimHostCache(store); return entry.html;
        });
        return entry.promise;
    }
    /* The drawing kept for a host shortcode, or null. */
    function hostDrawn(pictures, request) {
        if (!hostService(pictures) || !request) { return null; }
        var entry = pictures.hostFills.get(hostKey(pictures, request));
        return entry && entry.html != null ? entry.html : null;
    }
    /* A drawing as nodes for the page. It is made safe as all drawn HTML
       is, and nothing in it is read as a placeholder of its own. */
    function hostFragment(html) {
        var drawing = window.yavcHTML.fragment(html);
        drawing.querySelectorAll('*').forEach(function (n) { Array.from(n.attributes).forEach(function (a) { if (/^data-(?:host-|command|action|applet|choice)/.test(a.name)) n.removeAttribute(a.name); }); });
        drawing.querySelectorAll('.sc-host-body').forEach(function (n) { n.classList.remove('sc-host-body'); });
        return drawing;
    }
    function hostBody(el) {
        return Array.prototype.find.call(el.querySelectorAll('.sc-host-body'), function (b) { return b.closest('[data-host-shortcode]') === el; }) || null;
    }
    /* Puts a drawing in its placeholder. A container's or a span's drawing
       says where the body goes with <slot name="content">, and the body
       that is on the page is moved there, so what is being edited in it is
       kept. False when the drawing has no place for the body. */
    function placeHost(el, html) {
        if (el.__hostHtml === html) { return true; }
        var drawing = hostFragment(html), kind = el.getAttribute('data-host-kind');
        if (kind === 'container' || kind === 'span') {
            var body = hostBody(el), slots = drawing.querySelectorAll('slot[name="content"]');
            if (!body || slots.length !== 1) { return false; }
            slots[0].replaceWith(body);
        }
        el.replaceChildren(drawing);
        el.__hostHtml = html;
        el.setAttribute('data-host-drawn', 'true');
        el.classList.remove('sc-host-failed');
        return true;
    }
    /* Fills the host placeholders in drawn HTML: at once where the drawing
       is kept, and when it arrives where it is not. cachedOnly fills only
       from what is kept, for a part of the page that must not change after
       it is drawn. */
    function fillHosts(target, pictures, cachedOnly) {
        if (!target || !target.querySelectorAll || !hostService(pictures)) { return; }
        var list = Array.from(target.querySelectorAll('[data-host-shortcode]'));
        if (target.matches && target.matches('[data-host-shortcode]')) { list.unshift(target); }
        list.forEach(function (el) {
            var request = hostRequest(el);
            if (!request) { return; }
            var kept = hostDrawn(pictures, request);
            if (kept != null && !placeHost(el, kept)) { el.classList.add('sc-host-failed'); }
            if (cachedOnly) { return; }
            hostDrawing(pictures, request).then(function (html) {
                if (el.getAttribute('data-host-source') !== request.source || !hostLive(pictures)) { return; }
                if (html == null || !placeHost(el, html)) { if (!el.hasAttribute('data-host-drawn')) { el.classList.add('sc-host-failed'); } }
            });
        });
    }
    /* Asks for the drawings of every host shortcode in what the engine
       made (a document for the page view, blocks, a block's HTML, a
       container's parts) and resolves with the same value once they are
       kept, or after a few seconds whichever is first, so the page view
       draws each in place as it builds. */
    function prefillHosts(value, pictures) {
        if (!value || !hostService(pictures)) { return Promise.resolve(value); }
        var requests = [], seen = {}, holder = null;
        function add(r) {
            if (!r || requests.length >= 500) { return; }
            var k = hostKey(pictures, r);
            if (!seen[k]) { seen[k] = true; requests.push(r); }
        }
        function fromHTML(html) {
            if (html.indexOf('data-host-shortcode') < 0) { return; }
            holder = holder || document.createElement('template');
            holder.innerHTML = html;
            holder.content.querySelectorAll('[data-host-shortcode]').forEach(function (el) { add(hostRequest(el)); });
        }
        (function walk(v, depth) {
            if (!v || typeof v !== 'object' || depth > 256) { return; }
            if (Array.isArray(v)) { v.forEach(function (x) { walk(x, depth + 1); }); return; }
            if (v.type === 'host_span' && v.attrs) { add(hostSpanRequest(v.attrs.name, v.attrs.open)); }
            Object.keys(v).forEach(function (k) {
                var x = v[k];
                if (typeof x === 'string') { if (k === 'html' || k === 'before') { fromHTML(x); } }
                else { walk(x, depth + 1); }
            });
        })(value, 0);
        if (!requests.length) { return Promise.resolve(value); }
        var timer = null;
        function finished() { clearTimeout(timer); return value; }
        return Promise.race([
            Promise.all(requests.map(function (r) { return hostDrawing(pictures, r); })),
            new Promise(function (resolve) { timer = setTimeout(resolve, 4000); })
        ]).then(finished, finished);
    }
    /* Whether HTML the engine drew for one shortcode is a host's inline
       shortcode and nothing else. */
    function hostInlineOnly(html) {
        if (typeof html !== 'string' || html.indexOf('data-host-shortcode') < 0) { return false; }
        var holder = document.createElement('template');
        holder.innerHTML = html;
        var only = holder.content.childNodes.length === 1 ? holder.content.firstElementChild : null;
        return !!only && only.getAttribute('data-host-kind') === 'inline' && only.hasAttribute('data-host-shortcode');
    }
    /* A plain span with the look of an element of a drawing: its classes,
       its style and its title, and nothing the page view's schema reads as
       formatting (a strong, a link, a colour), so words typed inside a host
       span take no formatting from the drawing around them. */
    var HOST_SHELL_CLASSES = { 'sc-colour': true, 'sc-no-export': true, 'sc-host': true, 'sc-host-body': true };
    function hostShell(node) {
        var shell = document.createElement('span');
        Array.prototype.forEach.call(node.classList, function (c) { if (!HOST_SHELL_CLASSES[c]) { shell.classList.add(c); } });
        ['style', 'title', 'role', 'aria-label'].forEach(function (a) { if (node.hasAttribute(a)) { shell.setAttribute(a, node.getAttribute(a)); } });
        Array.prototype.forEach.call(node.attributes, function (a) {
            if (/^data-/.test(a.name) && !/^data-(class|export|host-)/.test(a.name)) { shell.setAttribute(a.name, a.value); }
        });
        return shell;
    }
    /* A host span as the page view draws a mark: { dom, contentDOM }, the
       look of the drawing's elements that are around its body with the
       page's words to go inside, or null when no drawing is kept. Only
       plain spans are used, so the editor finds nothing in the span but
       the words it put there. */
    function hostSpan(name, open, pictures) {
        var request = hostSpanRequest(name, open);
        if (!hostService(pictures) || !request) return null;
        var content = document.createElement('span'), dom = document.createElement('span');
        content.className = 'sc-host-body';
        dom.className = 'sc-host';
        dom.setAttribute('data-host-shortcode', name); dom.setAttribute('data-host-kind', 'span'); dom.setAttribute('data-host-source', open);
        dom.appendChild(content);
        function draw(html) {
            if (html == null || dom.__hostHtml === html) return;
            var drawing=hostFragment(html),slots=drawing.querySelectorAll('slot[name="content"]');if(slots.length!==1)return;
            var inner=content;
            for(var node=slots[0].parentNode;node&&node.nodeType===1;node=node.parentNode){var shell=hostShell(node);shell.appendChild(inner);inner=shell;}
            dom.replaceChildren(inner);if(dom.__hostControl)dom.appendChild(dom.__hostControl);dom.__hostHtml=html;dom.setAttribute('data-host-drawn','true');
        }
        draw(hostDrawn(pictures, request));
        hostDrawing(pictures, request).then(function(html){if(hostLive(pictures)&&dom.isConnected)draw(html);});
        return { dom: dom, contentDOM: content, ignoreMutation:function(m){return m.type!=='selection'&&!content.contains(m.target);} };
    }
    function editHostSpan(name, open, pictures) {
        var service=hostService(pictures),entry=(pictures.hostCatalog||[]).find(function(h){return h.name===name;});
        if(!service||!entry||entry.managed||typeof service.form!=='function'||typeof service.build!=='function')return Promise.resolve(null);
        var source=pictures.text&&pictures.text();
        return hostCall(new Map(),function(signal){return service.form(open,hostContext(pictures,signal));},pictures).then(function(form){
            if(!hostLive(pictures)||pictures.text&&pictures.text()!==source||!form)return null;
            return askHostShortcode(form,pictures,{existing:true}).then(function(next){
                if(next==='')return '';
                var parsed=typeof next==='string'&&parseShortcode(next.trim());
                return parsed&&parsed.name===name&&!/[\r\n]/.test(next)&&next.trim()==='{{'+parsed.name+(parsed.args?':'+parsed.args:'')+'}}'?next.trim():null;
            });
        });
    }
    /* The host's own shortcode that a block's source is, read with the
       document's engine: { name, kind, source, managed }, or null. */
    function hostPlaceholder(src, pictures) {
        var text = (src || '').trim(), engine = hostService(pictures) && pictures.macroContext && pictures.macroContext();
        if (!text || !engine || !engine.hosts || !engine.hosts().length) { return null; }
        var html;
        try { html = (engine.source(text) || {}).html; } catch (err) { return null; }
        if (typeof html !== 'string' || html.indexOf('data-host-shortcode') < 0) { return null; }
        var holder = document.createElement('template');
        holder.innerHTML = html;
        /* One element and no words beside it, inside a paragraph or not. */
        function lone(parent) {
            var el = parent.children.length === 1 ? parent.firstElementChild : null;
            return el && parent.textContent.trim() === el.textContent.trim() ? el : null;
        }
        var only = lone(holder.content);
        if (only && only.tagName === 'P' && !only.hasAttribute('data-host-shortcode')) { only = lone(only); }
        var request = only && only.hasAttribute('data-host-shortcode') ? hostRequest(only) : null;
        if (!request || request.kind === 'span') { return null; }
        if (request.kind === 'inline' && request.source !== text) { return null; }
        return { name: request.name, kind: request.kind, source: text, managed: only.getAttribute('data-host-managed') === 'true' };
    }
    /* A host shortcode pressed on the page: the host's form for it where
       it has one. Resolves as editBlock does, and is undefined where the
       source is not the host's. */
    function editHostBlock(src, pictures) {
        var found = hostPlaceholder(src, pictures);
        if(!found&&hostService(pictures)&&pictures.macroContext){
            var engine=pictures.macroContext(),drawing=engine&&engine.source(src),holder=document.createElement('template');
            holder.innerHTML=drawing&&drawing.html||'';
            if(holder.content.querySelector('[data-host-managed="true"]'))found={managed:true};
        }
        if (!found) { return undefined; }
        if (found.managed) {
            if (pictures.say) { pictures.say('The application keeps this part of the document itself, so it is not changed on the page.'); }
            return Promise.resolve(null);
        }
        var service = hostService(pictures);
        var opening = pictures.text && pictures.text();
        return hostCall(new Map(), function (signal) { return typeof service.form === 'function' && typeof service.build === 'function' ? service.form(found.source, hostContext(pictures, signal)) : null; }, pictures).then(function (form) {
            if (!hostLive(pictures) || pictures.text && pictures.text() !== opening) return null;
            if (form) { return askHostShortcode(form, pictures, { existing: true, markdown: found.kind !== 'inline' }).then(function(next){var parsed=typeof next==='string'&&parseShortcode(next.trim());if(found.kind==='container'&&parsed&&parsed.name===found.name&&next.trim().indexOf('\n')<0){var first=found.source.split(/\r?\n/,1)[0];return next.trim()+found.source.slice(first.length);}return next;}); }
            if (found.kind !== 'inline') { return undefined; }
            /* An inline shortcode with no form is changed as it is written. */
            return askText('Change the Shortcode', 'As It Is Written', found.source, { signal: pictures.signal, okLabel: 'Change' }).then(function (next) {
                next = next == null ? '' : String(next).trim();
                if (!next || next === found.source) { return null; }
                if (!/^\{\{[^{}\r\n]+\}\}$/.test(next)) { if (pictures.say) { pictures.say('That is not one shortcode, so nothing was changed.', true); } return null; }
                return next;
            });
        });
    }
    /* The host's form for one of its shortcodes (shortcodeHost.form): a
       promise of the shortcode to write, '' to take it out, null for no
       change, or undefined to edit it as its Markdown. The host writes the
       shortcode from the values (shortcodeHost.build) and says which field
       is wrong when it cannot. opts.existing is true for a shortcode that
       is being changed; for a new one each field's value is shown as an
       example to type over. opts.markdown offers Edit as Markdown. */
    function askHostShortcode(form, pictures, opts) {
        opts = opts || {};
        var service = hostService(pictures);
        if (!service || typeof service.build !== 'function' || !form || typeof form.kind !== 'string' || !Array.isArray(form.fields) || !hostLive(pictures)) { return Promise.resolve(null); }
        var owner = pictures.identity && pictures.identity(), opening = pictures.text && pictures.text();
        function unchanged() { return hostLive(pictures) && (!pictures.identity || pictures.identity() === owner) && (!pictures.text || pictures.text() === opening); }
        if (form.kind.length > 64 || form.fields.length > 32 || typeof form.title === 'string' && form.title.length > 256 || typeof form.description === 'string' && form.description.length > 4096) return Promise.resolve(null);
        if (JSON.stringify(form).length > 262144) return Promise.resolve(null);
        var fieldNames = new Set(), invalid = false;
        var fields = form.fields.filter(function (f) { if (!f || typeof f.name !== 'string' || !/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(f.name) || /^(?:__proto__|constructor|prototype)$/.test(f.name) || fieldNames.has(f.name) || typeof f.value === 'string' && f.value.length > 65536 || typeof f.label === 'string' && f.label.length > 256 || typeof f.description === 'string' && f.description.length > 4096 || Array.isArray(f.values) && (f.values.length > 256 || f.values.some(function (v) { return typeof v !== 'string' || v.length > 4096; }))) { invalid = true; return false; } fieldNames.add(f.name); return true; });
        if (invalid) return Promise.resolve(null);
        var id = 'md-host-' + (++dlgCount) + '-';
        var body = '<div class="md-form-fields">' + (typeof form.description === 'string' && form.description ? '<p class="form-help">' + esc(form.description) + '</p>' : '') + fields.map(function (f, i) {
            var fid = id + i, v = typeof f.value === 'string' ? f.value : '', label = typeof f.label === 'string' && f.label ? f.label : f.name;
            var help = typeof f.description === 'string' && f.description ? '<p class="form-help" id="' + fid + '-help">' + esc(f.description) + '</p>' : '';
            var described = ' aria-describedby="' + fid + '-error' + (help ? ' ' + fid + '-help' : '') + '"' + (f.required === true ? ' aria-required="true"' : '');
            var list = Array.isArray(f.values) ? f.values.filter(function (x) { return typeof x === 'string'; }).slice(0, 256) : [], control;
            if (list.length) {
                /* A value the list does not have is kept as a choice of its own. */
                if (list.indexOf(v) < 0) { list.unshift(v); }
                control = '<span class="form-select-wrap"><select class="form-select" id="' + fid + '" data-host-field="' + i + '"' + described + '>' + list.map(function (x) {
                    return '<option value="' + esc(x) + '"' + (x === v ? ' selected' : '') + '>' + esc(x || 'Choose…') + '</option>';
                }).join('') + '</select></span>';
            } else {
                control = '<input class="form-input" id="' + fid + '" data-host-field="' + i + '"' + described + ' value="' + esc(v) + '" autocomplete="off" spellcheck="false">';
            }
            return '<div class="form-group"><label class="form-label" for="' + fid + '">' + esc(label) + '</label>' + control + '<p class="form-error" id="' + fid + '-error" data-host-error="' + i + '" role="alert" hidden></p>' + help + '</div>';
        }).join('') + '<p class="form-error" data-role="error" role="alert" hidden></p></div>';
        var title = typeof form.title === 'string' && form.title ? form.title : 'Shortcode';
        var buttons = [];
        if (opts.existing) { buttons.push({ id: 'remove', label: 'Remove', danger: true, submit: false }); }
        if (opts.markdown) { buttons.push({ id: 'markdown', label: 'Edit as Markdown' }); }
        buttons.push({ id: 'cancel', label: 'Cancel' }, { id: 'ok', label: opts.existing ? 'Change' : 'Insert', primary: true });
        var api = null, built = null, busy = false;
        function control(i) { return api.body.querySelector('[data-host-field="' + i + '"]'); }
        function clear() {
            api.body.querySelectorAll('.form-error').forEach(function (line) { line.hidden = true; line.textContent = ''; });
            api.body.querySelectorAll('[aria-invalid]').forEach(function (c) { c.removeAttribute('aria-invalid'); });
        }
        function problem(name, message) {
            var i = -1;
            fields.forEach(function (f, at) { if (i < 0 && f.name === name) { i = at; } });
            var line = i >= 0 ? api.body.querySelector('[data-host-error="' + i + '"]') : api.body.querySelector('[data-role="error"]'), c = control(i >= 0 ? i : 0);
            line.textContent = message || 'The shortcode could not be written.';
            line.hidden = false;
            if (c) { if (i >= 0) { c.setAttribute('aria-invalid', 'true'); } c.focus(); }
        }
        function working(on) {
            busy = on;
            api.dialog.querySelectorAll('input, textarea, select, button:not([data-choice="cancel"])').forEach(function (c) {
                if (on) { if (!c.disabled) { c.disabled = true; c.setAttribute('data-host-working', ''); } }
                else if (c.hasAttribute('data-host-working')) { c.disabled = false; c.removeAttribute('data-host-working'); }
            });
        }
        return window.yavcDialog.open({
            title: (opts.existing ? 'Change ' : 'Insert ') + title,
            className: 'md-editor-dialog md-form', body: body, signal: pictures.signal, buttons: buttons,
            onOpen: function (a) {
                api = a;
                api.dialog.querySelector('form').noValidate = true;
                api.body.addEventListener('input', function (event) {
                    var c = event.target.closest('[data-host-field]');
                    if (!c) { return; }
                    c.removeAttribute('aria-invalid');
                    var line = api.body.querySelector('[data-host-error="' + c.getAttribute('data-host-field') + '"]');
                    if (line) { line.hidden = true; }
                });
            },
            check: function (v) {
                if (v !== 'ok') { return null; }
                if (busy) { return 'working'; }
                if (!unchanged()) { window.yavcUtil.notice(api.body, 'The document changed. Close and reopen this form.', true); return 'kept open'; }
                clear();
                var values = Object.create(null);
                fields.forEach(function (f, i) { var c = control(i); values[f.name] = c ? c.value : ''; });
                working(true);
                hostCall(new Map(), function (signal) { return service.build(form.kind, values, hostContext(pictures, signal)); }, pictures).then(function (out) {
                    if (!api.dialog.isConnected || !api.dialog.open) return;
                    if (!unchanged()) { working(false); problem('', 'The document changed. Close and reopen this form.'); return; }
                    if (out && typeof out.source === 'string' && out.source && out.source.length <= 65536 && !/[\u0000]/.test(out.source) && !out.error) { built = out.source; api.close('built'); return; }
                    working(false);
                    var e = out && out.error || {};
                    problem(typeof e.field === 'string' ? e.field : '', typeof e.message === 'string' ? e.message : '');
                }, function (err) { if (!api.dialog.isConnected || !api.dialog.open) return; working(false); problem('', String(err && err.message || err || '')); });
                return 'working';
            }
        }).then(function (result) {
            var v = result && result.value;
            if (v === 'built' && built != null) { return unchanged() ? built : null; }
            if (v === 'remove') { return unchanged() ? '' : null; }
            if (v === 'markdown') { return unchanged() ? undefined : null; }
            return null;
        });
    }
    /* A host shortcode to put into a document: the host's form for its
       example where it has one, and the example as it is otherwise.
       Resolves to the shortcode, or to null. */
    function askHostInsert(example, pictures) {
        var service = hostService(pictures);
        if (!service || typeof service.form !== 'function' || typeof service.build !== 'function' || !example) { return Promise.resolve(example || null); }
        var opening = pictures.text && pictures.text();
        return hostCall(new Map(), function (signal) { return service.form(example, hostContext(pictures, signal)); }, pictures).then(function (form) {
            if (!hostLive(pictures) || pictures.text && pictures.text() !== opening) return null;
            if (!form) { return example; }
            return askHostShortcode(form, pictures, {}).then(function (src) {
                if(!src)return null;
                var opening=/^\{\{([a-z][a-z0-9-]*)(?::[^{}]*)?\}\}/.exec(example),replacement=parseShortcode(src.trim());
                if(opening&&replacement&&replacement.name===opening[1]&&/^\{\{[^{}\r\n]+\}\}$/.test(src.trim())&&example.includes('{{/'+opening[1]+'}}'))return src.trim()+example.slice(opening[0].length);
                return src;
            });
        });
    }

    /* === Drawing a page ===================================================
       One renderer for the Editor's preview and page, and Slides' canvas,
       filmstrip, notes and show (review of 2026-11-03: there were two). */

    /* A picture of the home folder as an address this page can draw. The
       address is made once for each version of the file and kept, so
       drawing again (as typing does) makes no new copy of the bytes; when
       the file changes, the old address is released (performance review,
       2026-11-03). Resolves to '' for a file that is not there. */
    var pictureCache = {}, picturePaths = {};
    function pictureURL(node, F) {
        var r = F.realOf(node), path = F.displayPath(r);
        var version = path + '|' + (r.blob || '') + '|' + (r.mtime ? +r.mtime : '') + '|' + (r.size || '');
        if (pictureCache[version]) { return pictureCache[version]; }
        var made = F.readBytes(node).then(function (bytes) { return URL.createObjectURL(new Blob([bytes], { type: (F.typeOf && F.typeOf(node)) || '' })); });
        var was = picturePaths[path];
        if (was && was !== version && pictureCache[was]) {
            pictureCache[was].then(function (u) { if (u) { URL.revokeObjectURL(u); } });
            delete pictureCache[was];
        }
        picturePaths[path] = version;
        pictureCache[version] = made.catch(function () { delete pictureCache[version]; return ''; });
        return pictureCache[version];
    }
    /* A picture's address from a document's own words (cat.png beside the
       document, ~/Pictures/cat.png), for a slide's background say. */
    function pictureFor(src, pictures) {
        if(!pictures)return Promise.resolve('');
        return pictureFound(src,pictures).then(function(found){return !found?'':found.url||pictureURL(found.node,found.F);}).catch(function () { return ''; });
    }
    /* The same picture for a caller that needs its bytes, as a canvas does
       to draw it inside its own picture: { url } for one the host resolved,
       { bytes } for one of the home folder, null for one that is not there. */
    function pictureData(src, pictures) {
        if(!pictures)return Promise.resolve(null);
        return pictureFound(src,pictures).then(function(found){return !found?null:found.url?{url:found.url}:found.F.readBytes(found.node).then(function(bytes){return {bytes:bytes};});}).catch(function () { return null; });
    }
    /* Where a picture is: { url } from the host, { node, F } in the home
       folder, or null. It rejects for a reference or an address that is
       not allowed. */
    function pictureFound(src, pictures) {
        var resolver=pictures.resources&&pictures.resources.resolve;
        return Promise.resolve().then(function(){
            if(pictures.signal&&pictures.signal.aborted||pictures.current&&!pictures.current())return null;
            if(!resolver||!pictures.resource)return null;
            if(typeof src!=='string'||!src||src.length>4096||/[\x00-\x1f\x7f]/.test(src)||/^(?:[a-z][a-z0-9+.-]*:|\/\/)/i.test(src))throw new Error('Invalid document picture reference.');
            return resolver.call(pictures.resources,{resource:pictures.resource,href:src,kind:'image'},{signal:pictures.signal,current:pictures.current,host:pictures.root});
        }).then(function(result){
            if(pictures.signal&&pictures.signal.aborted||pictures.current&&!pictures.current())return null;
            if(result!=null){
                if(typeof result.url!=='string'||!result.url||result.url.length>16384||/[\x00-\x1f\x7f]/.test(result.url))throw new Error('Invalid host picture URL.');
                var url=new URL(result.url,location.href);
                if(!/^https?:$/.test(url.protocol)||url.origin!==location.origin||url.username||url.password)throw new Error('Host pictures require a same-origin web URL.');
                return {url:url.href};
            }
            if(!pictures.site)return null;
            return pictures.site().then(function (site) {
            var F = site.F, dir = pictures.dir ? pictures.dir() : null, base = (dir && F.resolve(null, dir)) || F.home();
            var node = base ? F.resolve(base, src) : null;
            return node && !node.children ? {node:node,F:F} : null;
            });
        });
    }
    /* The pictures of the home folder in drawn HTML (the engine marks each
       with data-attachment, its path from the document's folder), read
       from the folder and shown; one not found is marked. */
    var imageRequests=new WeakMap();
    function loadImages(target, pictures, force) {
        return Promise.all(Array.from(target.querySelectorAll('img[data-attachment]')).map(function(img){
            var name=img.dataset.attachment,owner=pictures&&pictures.identity?pictures.identity():null,resource=pictures&&pictures.resource,previous=imageRequests.get(img);
            if(!force&&previous&&previous.name===name&&previous.owner===owner&&previous.resource===resource)return previous.promise;
            var request={name:name,owner:owner,resource:resource};imageRequests.set(img,request);img.removeAttribute('src');
            request.promise=pictureFor(name,pictures).then(function(url){
                if(imageRequests.get(img)!==request||img.dataset.attachment!==name||pictures&&(pictures.signal&&pictures.signal.aborted||pictures.current&&!pictures.current()))return;
                img.classList.toggle('ed-missing-image',!url);
                if(url)img.src=url;else img.alt=img.alt||name;
            });return request.promise;
        }));
    }
    /* A listing ({{code:path}}) is fetched as the page draws: a path on the
       site from /, or one of the reader's own files under ~. */
    function loadListings(target) {
        target.querySelectorAll('code[data-code-src]:not([data-code-loaded])').forEach(function (code) {
            code.setAttribute('data-code-loaded', '');
            var src = code.getAttribute('data-code-src'), pre = code.closest('pre');
            if (pre) { pre.classList.add('code'); pre.setAttribute('data-code-filename', src.split('?')[0].split('/').pop()); }
            var got = src.charAt(0) === '~'
                ? window.yavcUtil.siteFs().then(function (F) { var n = F.resolve(null, src); if (!n || n.children) { throw new Error('no such file'); } return F.read(n); })
                : fetch(src, { credentials: 'same-origin' }).then(function (r) { if (!r.ok) { throw new Error('HTTP ' + r.status); } return r.text(); });
            got.then(function (text) { code.textContent = text; if (pre && window.pudlCode && window.pudlCode.enhance) { window.pudlCode.enhance(pre); } })
               .catch(function (err) { code.textContent = 'The file could not be loaded: ' + (err && err.message || err); });
        });
    }
    /* === Find and replace ===========================================
       The Editor's and Diagrams' find bar, kept here so there is one:
       One find bar for the text and the page, in PUDL's parts: the two
       fields sunken, the buttons raised, Match Case, Whole Word and
       Regular Expression as latching buttons (as Chat's Thinking is),
       the count of matches, and Close. In the text the bar is
       CodeMirror's search panel (search({ createPanel })), so CodeMirror
       highlights the matches and keeps the query with the tab; on the
       page it sits over the frame and drives the visual editor's own
       search (js/editor-visual.js). The query carries over when the view
       changes. Enter finds the next match and Shift+Enter the one
       before; in the Replace field Enter replaces it and Ctrl+Enter
       replaces them all; Escape closes the bar. */
    var FIND_OPTS = [['caseSensitive', 'Aa', 'Match Case'], ['wholeWord', 'W', 'Whole Word'], ['regexp', '.*', 'Regular Expression']];
    function findBar(driver, onQuery) {
        var dom = document.createElement('div');
        dom.className = 'ed-find';
        dom.setAttribute('role', 'search');
        dom.setAttribute('aria-label', 'Find and Replace');
        var g = function (name) { return '<span class="glyph" style="--glyph: var(--glyph-' + name + ')" aria-hidden="true"></span>'; };
        /* The grid inside the bar, so the bar's own width can set it out
           (a container query in the site's stylesheet). */
        dom.innerHTML = '<div class="ed-find-grid">' +
            '<div class="ed-find-row">' +
              '<input class="form-input ed-find-field" type="text" data-find="search" placeholder="Find" aria-label="Find" autocomplete="off" spellcheck="false" main-field="true" />' +
              '<span class="ed-find-tools">' +
                '<span class="ed-find-count" data-find="count" aria-live="polite"></span>' +
                '<button type="button" class="btn btn-sm" data-find="prev" aria-label="Previous Match" title="Previous Match (Shift+Enter)">' + g('arrow-up') + '</button>' +
                '<button type="button" class="btn btn-sm" data-find="next" aria-label="Next Match" title="Next Match (Enter)">' + g('arrow-down') + '</button>' +
                '<span class="ed-find-opts" role="group" aria-label="Options">' + FIND_OPTS.map(function (o) {
                    return '<button type="button" class="btn btn-sm ed-find-opt" data-opt="' + o[0] + '" aria-pressed="false" aria-label="' + o[2] + '" title="' + o[2] + '">' + o[1] + '</button>';
                }).join('') + '</span>' +
                '<button type="button" class="icon-btn ed-find-close" data-find="close" aria-label="Close" title="Close (Escape)">' + g('close') + '</button>' +
              '</span>' +
            '</div>' +
            '<div class="ed-find-row" data-find="replace-row">' +
              '<input class="form-input ed-find-field" type="text" data-find="replace" placeholder="Replace With" aria-label="Replace With" autocomplete="off" spellcheck="false" />' +
              '<span class="ed-find-tools">' +
                '<button type="button" class="btn btn-sm" data-find="replace-one" title="Replace This Match (Enter)">Replace</button>' +
                '<button type="button" class="btn btn-sm" data-find="replace-all" title="Replace Every Match (Ctrl+Enter)">Replace All</button>' +
              '</span>' +
            '</div>' +
        '</div>';
        var f = function (k) { return dom.querySelector('[data-find="' + k + '"]'); };
        var search = f('search'), replace = f('replace');
        var opt = function (k) { return dom.querySelector('[data-opt="' + k + '"]'); };
        function query() {
            var q = { search: search.value, replace: replace.value };
            FIND_OPTS.forEach(function (o) { q[o[0]] = opt(o[0]).getAttribute('aria-pressed') === 'true'; });
            return q;
        }
        function show(q) {
            if (!q) { return; }
            if (search.value !== (q.search || '')) { search.value = q.search || ''; }
            if (replace.value !== (q.replace || '')) { replace.value = q.replace || ''; }
            FIND_OPTS.forEach(function (o) { opt(o[0]).setAttribute('aria-pressed', String(!!q[o[0]])); });
        }
        function refresh() {
            var c = driver.count(), count = f('count');
            count.textContent = !search.value ? '' : c.error ? 'Not a valid expression' : !c.total ? 'No matches'
                : c.at ? c.at + ' of ' + (c.total >= 1000 ? '1000+' : c.total) : (c.total >= 1000 ? '1000+' : c.total) + (c.total === 1 ? ' match' : ' matches');
            count.classList.toggle('ed-find-none', !!search.value && (!!c.error || !c.total));
            f('prev').disabled = f('next').disabled = !c.total;
            f('replace-row').hidden = driver.readOnly();
            f('replace-one').disabled = f('replace-all').disabled = !c.total;
        }
        function apply() { var q = query(); if (onQuery) { onQuery(q); } driver.set(q); refresh(); }
        search.addEventListener('input', apply);
        replace.addEventListener('input', apply);
        /* A press on a button leaves the keyboard in the field. */
        dom.addEventListener('mousedown', function (e) { if (e.target.closest('button')) { e.preventDefault(); } });
        dom.addEventListener('click', function (e) {
            var b = e.target.closest('button');
            if (!b || b.disabled) { return; }
            if (b.hasAttribute('data-opt')) { b.setAttribute('aria-pressed', String(b.getAttribute('aria-pressed') !== 'true')); apply(); return; }
            switch (b.getAttribute('data-find')) {
                case 'prev': driver.prev(); break;
                case 'next': driver.next(); break;
                case 'replace-one': driver.replace(); break;
                case 'replace-all': driver.replaceAll(); break;
                case 'close': driver.close(); return;
            }
            refresh();
        });
        dom.addEventListener('keydown', function (e) {
            if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); driver.close(); return; }
            if (e.key !== 'Enter' || e.isComposing) { return; }
            e.preventDefault(); e.stopPropagation();
            if (e.target === replace) { if (e.ctrlKey || e.metaKey) { driver.replaceAll(); } else { driver.replace(); } }
            else if (e.shiftKey) { driver.prev(); } else { driver.next(); }
            refresh();
        });
        return { dom: dom, show: show, refresh: refresh, focus: function () { search.focus(); search.select(); } };
    }

    /* The text's search, which is CodeMirror's. */
    function textFind(CM, v) {
        return {
            set: function (q) { v.dispatch({ effects: CM.setSearchQuery.of(new CM.SearchQuery(q)) }); },
            next: function () { CM.findNext(v); },
            prev: function () { CM.findPrevious(v); },
            replace: function () { CM.replaceNext(v); },
            replaceAll: function () { CM.replaceAll(v); },
            close: function () { CM.closeSearchPanel(v); v.focus(); },
            readOnly: function () { return v.state.readOnly; },
            count: function () {
                var q = CM.getSearchQuery(v.state);
                if (!q.search) { return { total: 0 }; }
                if (!q.valid) { return { total: 0, error: true }; }
                var sel = v.state.selection.main, total = 0, at = 0, cur = q.getCursor(v.state);
                for (var r = cur.next(); !r.done && total < 1000; r = cur.next()) {
                    total++;
                    if (r.value.from === sel.from && r.value.to === sel.to) { at = total; }
                }
                return { total: total, at: at };
            }
        };
    }
    /* CodeMirror's search panel, made as the find bar. */
    function findPanel(CM, onQuery, v) {
        var bar = findBar(textFind(CM, v), onQuery), shown = null;
        return {
            dom: bar.dom, top: true,
            /* As CodeMirror's own panel does, the bar takes the keyboard
               when it first appears; openSearchPanel focuses it after. */
            mount: function () { shown = CM.getSearchQuery(v.state); bar.show(shown); bar.refresh(); bar.focus(); },
            update: function (u) {
                var q = CM.getSearchQuery(u.state);
                if (q !== shown) { shown = q; bar.show(q); }
                bar.refresh();
            }
        };
    }

    /* CodeMirror's search, with the find bar as its panel: the extension
       an editor adds. onQuery hears each query the person types. */
    function codeSearch(CM, onQuery) {
        return CM.search({ top: true, createPanel: function (v) { return findPanel(CM, onQuery, v); } });
    }

    /* A Mermaid diagram drawn into its place, made safe with its own
       stylesheet kept under its id (apps/md-engine/safe-html.js). */
    /* opts, as for D2 (apps/diagram-kit/d2.js draw): alt, the drawing's
       text alternative; keep leaves the last good drawing, dimmed, when the
       text has a mistake; and report: false leaves the mistake to the
       caller. Resolves with { svg } or with { problem }. While Mermaid
       loads, which the first time is about 3 MB, the place shows PUDL's
       loading state with words, after 400 ms as D2's does. */
    var diagramCount = 0;
    function drawDiagram(place, source, opts) {
        opts = opts || {};
        var id = 'md-diagram-' + (++diagramCount), loading = null;
        var cue = setTimeout(function () {
            if (!place.isConnected || place.querySelector(':scope > .loading')) { return; }
            loading = document.createElement('div');
            loading.className = 'loading';
            loading.setAttribute('role', 'status');
            loading.innerHTML = '<span class="spinner" aria-hidden="true"></span><span>' + (window.mermaid ? 'Drawing the diagram…' : 'Loading Mermaid, which takes a moment the first time…') + '</span>';
            place.insertBefore(loading, place.firstChild);
        }, 400);
        function settled() { clearTimeout(cue); if (loading) { loading.remove(); loading = null; } }
        return Promise.all([mermaid(), safeHTML()]).then(function () {
            window.mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', suppressErrorRendering: true,
                theme: !opts.paper && !place.closest('[data-ed-page]') && document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'default' });
            return window.mermaid.render(id, source);
        }).then(function (out) {
            settled();
            if (!place.isConnected) { return { svg: out.svg }; }
            place.replaceChildren(window.yavcHTML.fragment(out.svg, { diagram: true }));
            var svg = place.querySelector('svg');
            if (svg && opts.alt) { svg.setAttribute('role', 'img'); svg.setAttribute('aria-label', opts.alt); }
            place.classList.add('ed-diagram');
            place.classList.remove('sc-diagram-stale');
            return { svg: out.svg };
        }).catch(function (err) {
            settled();
            /* Mermaid leaves a scratch element behind when it fails. */
            var scratch = document.getElementById('d' + id);
            if (scratch) { scratch.remove(); }
            var problem = String((err && err.message) || err);
            if (opts.keep && place.classList.contains('ed-diagram') && place.firstElementChild) { place.classList.add('sc-diagram-stale'); }
            else if (opts.report === false) { place.replaceChildren(); }
            if (opts.report !== false) { diagramFailed(place, err); }
            return { problem: problem };
        });
    }
    /* A diagram that could not be drawn at all says so in the site's
       notice above it, as the house rules ask of every error. */
    function diagramFailed(place, err) {
        if (!place.isConnected) { return; }
        var ref = place.firstElementChild;
        if (!ref) { ref = document.createElement('span'); place.appendChild(ref); }
        window.yavcUtil.notice(ref, 'The diagram could not be drawn: ' + ((err && err.message) || err), 'problem');
    }
    /* A D2 diagram drawn into its place by the D2 engine in its worker
       (apps/diagram-kit/d2.js; docs/diagrams-design.md), loaded the first
       time a page has one, since the engine is large. */
    var d2Ready = null;
    var canvasReady = null;
    /* pictures is the host's picture service, for the canvas's picture
       nodes. A picture's path is from the folder the canvas is in: the
       document's own for a canvas written in it, and the folder of the
       reference for a saved one. */
    function drawCanvas(place, source, pictures, reference) {
        if (!canvasReady) {
            canvasReady = loadScript(window.yavcUtil.versioned(new URL('../diagram-kit/canvas-embed.js', HERE).href)).then(function () { return window.yavcCanvasEmbed; });
            canvasReady.catch(function () { canvasReady = null; });
        }
        var folder = reference ? String(reference).replace(/[^/]*$/, '') : '';
        var options = pictures ? { signal: pictures.signal, pictures: function (path) { return pictureData(/^(?:~\/|\/)/.test(path) ? path : folder + path, pictures); } } : null;
        return canvasReady.then(function (engine) { return engine.draw(place, source, options); }).catch(function (error) { if (place.isConnected) diagramFailed(place, error); });
    }
    function drawD2(place, source) {
        if (!d2Ready) {
            d2Ready = loadScript(window.yavcUtil.versioned(new URL('../diagram-kit/d2.js', HERE).href)).then(function () { return window.yavcD2Engine; });
            d2Ready.catch(function () { d2Ready = null; });
        }
        /* The engine reports a fault in the diagram itself; what reaches
           this catch is the engine failing to load. */
        return d2Ready.then(function (engine) { return engine.draw(place, source); })
            .catch(function () { diagramFailed(place, 'the diagram engine did not load. Reloading the page tries again.'); });
    }
    /* Draws the engine's preview blocks into an element. A block whose
       HTML is the same as one already there keeps its element, with its
       diagram, its pictures, its listing and any applet in it running, so
       typing redraws only the blocks it changed (performance review,
       2026-11-03). opts: { pictures: the host's picture service,
       diagrams: false to leave diagrams undrawn, applets: false to leave
       applets unstarted, as a thumbnail does }. */
    function refreshReferences(target, host) {
        loadImages(target,host);
        if(!target.querySelector('[data-diagram-src]'))return;
        Promise.all([styles(),loadScript(window.yavcUtil.versioned(new URL('../diagram-kit/references.js',HERE).href))]).then(function(){
            window.yavcDiagramReferences.attach(target,host,function(kind,place,source,reference){return kind==='canvas'?drawCanvas(place,source,host,reference):kind==='d2'?drawD2(place,source):drawDiagram(place,source);});
        }).catch(function(error){if(target.isConnected)target.querySelectorAll('[data-diagram-body]').forEach(function(body){window.yavcUtil.notice(body,'The diagram loader could not start: '+error.message,'warn');});});
    }
    var footnoteScopes=new WeakMap();
    function scopeFootnotes(target,namespace){
        namespace=namespace||footnoteScopes.get(target)||'yavc-notes-'+crypto.randomUUID();
        if(!/^[a-z][a-z0-9_-]{1,96}$/i.test(namespace))throw new Error('Invalid footnote surface identity.');
        footnoteScopes.set(target,namespace);
        var definitions=Array.from(target.querySelectorAll('.doc-footnote[data-footnote-number]')),references=Array.from(target.querySelectorAll('.doc-footnote-ref[data-footnote-number]'));
        if(target.matches&&target.matches('.doc-footnote'))definitions.unshift(target);
        if(target.matches&&target.matches('.doc-footnote-ref'))references.unshift(target);
        definitions.forEach(function(note){var number=Number(note.getAttribute('data-footnote-number'));if(Number.isSafeInteger(number)&&number>0)note.id=namespace+'-note-'+number;else note.removeAttribute('id');});
        references.forEach(function(reference){var number=Number(reference.getAttribute('data-footnote-number')),link=reference.querySelector('a');if(link){if(Number.isSafeInteger(number)&&number>0)link.setAttribute('href','#'+namespace+'-note-'+number);else link.removeAttribute('href');}});
        return namespace;
    }
    function drawBlocks(target, blocks, opts) {
        opts = opts || {};
        var old = Array.prototype.filter.call(target.children, function (c) { return c.classList && c.classList.contains('doc-block'); });
        var byHtml = {};
        old.forEach(function (c) { var k = c.__mdHtml; if (k != null) { (byHtml[k] = byHtml[k] || []).push(c); } });
        var used = new Set(), fresh = [], order = [];
        (blocks || []).forEach(function (b) {
            var kept = (byHtml[b.html] || []).shift();
            if (kept) { used.add(kept); kept.dataset.line = b.line; order.push(kept); return; }
            var block = document.createElement('div');
            block.className = 'doc-block';
            block.dataset.line = b.line;
            block.__mdHtml = b.html;
            block.replaceChildren(window.yavcHTML.fragment(b.html));
            order.push(block);
            fresh.push({ el: block, diagrams: b.diagrams });
        });
        old.forEach(function (c) {
            if (used.has(c)) { return; }
            if (window.pudlApplets && window.pudlApplets.destroy && c.querySelector('[data-applet]')) { try { window.pudlApplets.destroy(c); } catch (err) { } }
            c.remove();
        });
        Array.prototype.forEach.call(target.childNodes, function (c) { if (!(c.classList && c.classList.contains('doc-block'))) { c.remove(); } });
        order.forEach(function (el, i) { if (target.children[i] !== el) { target.insertBefore(el, target.children[i] || null); } });
        fresh.forEach(function (f) {
            if (opts.diagrams !== false) {
                (f.diagrams || []).forEach(function (d) {
                    var place = f.el.querySelector('[data-preview-diagram="' + CSS.escape(d.id) + '"]');
                    if (place && d.language === 'mermaid') { drawDiagram(place, d.source); }
                    else if (place && d.language === 'd2') { drawD2(place, d.source); }
                    else if (place && d.language === 'canvas') { drawCanvas(place, d.source, opts.pictures); }
                });
            }
            if (opts.applets !== false && window.pudlApplets && window.pudlApplets.boot && f.el.querySelector('[data-applet]')) { try { window.pudlApplets.boot(f.el); } catch (err) { } }
            loadImages(f.el, opts.pictures);
            loadListings(f.el);
            fillHosts(f.el, opts.pictures);
        });
        scopeFootnotes(target);
        if(opts.diagrams!==false)refreshReferences(target,opts.pictures);
    }

    /* === Properties =======================================================
       A document's front matter as a form (DESIGN.md, decision 17): the
       stock properties, then any others, each a name and a value, which
       may be added, changed and taken out. front is the engine's reading
       of the front matter (internal/frontmatter): { entries: [{ key,
       value, kind, editable }], problem }. opts.hidden are names the host
       edits elsewhere, such as a deck's theme, left out of the form and
       left as they are. Resolves to { set: { name: value, or null to take
       it out }, order: [names] } with only what changed, or null. The
       engine checks every name and value again, and reads the result back,
       before anything is written. */
    var STOCK = [
        { key: 'title', label: 'Title' },
        { key: 'author', label: 'Author' },
        { key: 'subject', label: 'Subject', help: 'What it is about, in a line.' },
        { key: 'keywords', label: 'Keywords', help: 'Words to find it by, separated by commas.' },
        { key: 'lang', label: 'Language', placeholder: 'en-GB', help: 'The language it is written in, as a code such as en-GB or fr, for spelling and for screen readers.' },
        { key: 'date', label: 'Date', kind: 'date', help: 'When it was written or is for.' }
    ];
    var PROP_NAME = /^[A-Za-z_][A-Za-z0-9_-]{0,63}$/;
    var PROP_MAX = 2000;
    function propProblem(name, value) {
        if (!PROP_NAME.test(name)) { return '"' + name + '" is not a property name: use letters, digits, - and _, starting with a letter or _, up to 64 of them.'; }
        if (value.length > PROP_MAX) { return 'The value of ' + name + ' is longer than ' + PROP_MAX + ' characters.'; }
        if (/[\u0000-\u0008\u000a-\u001f\u007f]/.test(value)) { return 'The value of ' + name + ' holds a character that cannot be kept, such as a line break.'; }
        return null;
    }
    function askProperties(front, opts) {
        opts = opts || {};
        if (front && front.problem) {
            return askChoice('Properties', front.problem + ' Fix it in the text, and the properties can be edited here again.', [['ok', 'OK', true]]).then(function () { return null; });
        }
        var hidden = {}; (opts.hidden || []).forEach(function (k) { hidden[k] = true; });
        var byKey = {}; ((front && front.entries) || []).forEach(function (e) { byKey[e.key] = e; });
        var stockKeys = {}; STOCK.forEach(function (f) { stockKeys[f.key] = true; });
        var custom = ((front && front.entries) || []).filter(function (e) { return !stockKeys[e.key] && !hidden[e.key]; });
        var n = ++dlgCount, id = 'md-props-' + n + '-', customIndex = 0;
        function customRow(e, k) {
            var locked = e && !e.editable, rowId = id + 'custom-' + (++customIndex);
            return '<div class="md-prop-row" data-row data-was="' + esc(e ? e.key : '') + '"' + (locked ? ' data-locked' : '') + '>' +
                '<div class="form-group"><label class="form-label" for="' + rowId + '-name">Name</label><input class="form-input" id="' + rowId + '-name" data-name value="' + esc(e ? e.key : '') + '" autocomplete="off" spellcheck="false"' + (locked ? ' readonly' : '') + '></div>' +
                '<div class="form-group"><label class="form-label" for="' + rowId + '-value">Value</label><input class="form-input" id="' + rowId + '-value" data-value value="' + esc(e ? e.value : '') + '" autocomplete="off"' + (locked ? ' readonly title="Written in a form this dialog does not change; edit it in the text."' : '') + '></div>' +
                (locked ? '<span class="md-prop-note">In the text</span>' : '<button type="button" class="btn" data-act="remove-row" aria-label="Take Out This Property">Remove</button>') + '</div>';
        }
        var html = '<div class="md-props-stock">' + STOCK.map(function (f) {
            var e = byKey[f.key], v = e ? e.value : '', locked = e && !e.editable, fid = id + f.key;
            var input = f.kind === 'date' && !locked
                ? '<input class="form-input" type="date" data-date id="' + fid + '" data-key="' + f.key + '" value="' + esc(/^\d{4}-\d{2}-\d{2}$/.test(v) ? v : '') + '">'
                : '<input class="form-input" id="' + fid + '" data-key="' + f.key + '" value="' + esc(v) + '" autocomplete="off"' + (f.placeholder ? ' placeholder="' + esc(f.placeholder) + '"' : '') + (locked ? ' readonly' : '') + '>';
            var help = locked ? 'Written in a form this dialog does not change; edit it in the text.' : (f.kind === 'date' && v && !/^\d{4}-\d{2}-\d{2}$/.test(v) ? 'It is now "' + v + '", which is not a date this field reads; choosing one replaces it.' : f.help);
            return '<div class="form-group"><label class="form-label" for="' + fid + '">' + esc(f.label) + '</label>' + input + (help ? '<p class="form-help">' + esc(help) + '</p>' : '') + '</div>';
        }).join('') + '</div>' +
            '<div class="form-group"><span class="form-label">Other Properties</span><div class="md-props-custom" data-role="custom">' + custom.map(customRow).join('') + '</div>' +
            '<button type="button" class="btn" data-act="add-row">Add a Property</button>' +
            '<p class="form-help">Any name and value of your own, kept in the front matter.' + (opts.hidden && opts.hidden.length ? ' ' + esc(opts.hiddenNote || '') : '') + '</p></div>' +
            '<p class="form-error" data-role="error" hidden></p>';
        var dlg = null, list = null, errorLine = null;
        function problem(text, focus) { errorLine.textContent = text; errorLine.hidden = false; if (focus) { focus.focus(); } }
        return ownForm({
            title: opts.title || 'Properties', className: 'md-props', body: html, signal: opts.signal, current: opts.current,okLabel:'Apply',
            setup: function (api) {
                dlg = api.dialog; list = dlg.querySelector('[data-role="custom"]'); errorLine = dlg.querySelector('[data-role="error"]');
                api.body.addEventListener('click', function (e) {
                    var b = e.target.closest('[data-act]');
                    if (!b) { return; }
                    var act = b.getAttribute('data-act');
                    if (act === 'add-row') { list.insertAdjacentHTML('beforeend', customRow(null)); list.lastElementChild.querySelector('[data-name]').focus(); }
                    else if (act === 'remove-row') { var row = b.closest('[data-row]'); var next = row.nextElementSibling || row.previousElementSibling; row.remove(); (next ? next.querySelector('input') : dlg.querySelector('[data-act="add-row"]')).focus(); }
                });
                var first = dlg.querySelector('.md-props-stock input:not([readonly])');
                if (first) { first.focus(); }
            },
            submit: function () {
                errorLine.hidden = true;
                var set = {}, order = [], names = {};
                for (var i = 0; i < STOCK.length; i++) {
                    var f = STOCK[i], input = dlg.querySelector('[data-key="' + f.key + '"]'), was = byKey[f.key], v = input.value.trim();
                    if (input.readOnly) { continue; }
                    if (f.kind === 'date' && !v && was && !/^\d{4}-\d{2}-\d{2}$/.test(was.value)) { continue; }
                    var bad = v ? propProblem(f.key, v) : null;
                    if (bad) { problem(bad, input); return undefined; }
                    if (f.key === 'lang' && v && !/^[A-Za-z]{2,3}(-[A-Za-z0-9]{1,8})*$/.test(v)) { problem('The language is a code such as en, en-GB or pt-BR.', input); return undefined; }
                    if (f.key === 'date' && v && !/^\d{4}-\d{2}-\d{2}$/.test(v)) { problem('The date is a day, such as 2026-11-03.', input); return undefined; }
                    if (v !== (was ? was.value : '')) { set[f.key] = v ? v : null; order.push(f.key); }
                }
                var rows = list.querySelectorAll('[data-row]'), kept = {};
                for (var r = 0; r < rows.length; r++) {
                    var row = rows[r], nameEl = row.querySelector('[data-name]'), valueEl = row.querySelector('[data-value]');
                    var name = nameEl.value.trim(), value = valueEl.value.trim(), wasKey = row.getAttribute('data-was');
                    if (row.hasAttribute('data-locked')) { kept[wasKey] = true; names[wasKey] = true; continue; }
                    if (!name && !value) { continue; }
                    if (!name) { problem('A property needs a name.', nameEl); return undefined; }
                    if (stockKeys[name]) { problem(name + ' has its own field above.', nameEl); return undefined; }
                    if (hidden[name]) { problem(name + ' is set elsewhere' + (opts.hiddenWhere ? ', ' + opts.hiddenWhere : '') + '.', nameEl); return undefined; }
                    if (names[name]) { problem('There are two properties called ' + name + '.', nameEl); return undefined; }
                    var bad2 = propProblem(name, value);
                    if (bad2) { problem(bad2, PROP_NAME.test(name) ? valueEl : nameEl); return undefined; }
                    names[name] = true;
                    if (!value) { continue; }
                    if (wasKey && wasKey === name) { kept[wasKey] = true; }
                    var before = byKey[name];
                    if (!before || before.value !== value) { set[name] = value; order.push(name); }
                }
                custom.forEach(function (c) {
                    if (!kept[c.key] && !(c.key in set)) { set[c.key] = null; order.push(c.key); }
                });
                return { set: set, order: order };
            }
        });
    }

    window.yavcMdEditor = {
        here: HERE, stamp: STAMP, placeholder: PLACEHOLDER, pages: pages, pageSetup: pageSetup, styles: styles, macros: macros, exports: documentExports,
        colourRows: pickerRows, wireColours: wirePickers,
        libs: libs, languageServices: languageServices, codeTheme: codeTheme, codeSearch: codeSearch, findBar: findBar, pageView: pageView, mermaid: mermaid, drawMermaid: drawDiagram, loadScript: loadScript,
        FORMATS: FORMATS, STYLES: STYLES, INSERTS: INSERTS, SHORTCODES: SHORTCODES, MENU: MENU, STOCK_CLASSES: STOCK_CLASSES,
        tableCommand: tableCommand, quoteState: quoteState, wordCount: wordCount, readingMinutes: readingMinutes,
        commands: commands, editing: editing, apply: apply, classesOf: classesOf, insertItem: insertItem, inserts: inserts, textFormat: textFormat, textInsert: textInsert,
        askText: askText, askFields: askFields, askChoice: askChoice, askProperties: askProperties, STOCK: STOCK, askColour: askColour, askColours: askColours, askClass: askClass,
        askFigure: askFigure, editBlock: editBlock, drawBlocks: drawBlocks, scopeFootnotes:scopeFootnotes, refreshReferences:refreshReferences, drawDiagram: drawDiagram, pictureFor: pictureFor, pictureData: pictureData, askShortcode: askShortcode, FORMS: FORMS, boxCommand: boxCommand, BOXES: BOXES, loadImages: loadImages, parseFigure: parseFigure, buildFigure: buildFigure, parseShortcode: parseShortcode,
        fillHosts: fillHosts, prefillHosts: prefillHosts, hostSpan: hostSpan, editHostSpan: editHostSpan, hostInlineOnly: hostInlineOnly, askHostShortcode: askHostShortcode, askHostInsert: askHostInsert,
        _page: null
    };
})();
