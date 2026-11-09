/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Shared stylesheet editing, using the host's dialogs and virtual files. */
(function () {
    'use strict';
    var esc = window.yavcUtil.esc, count = 0;
    function resolve(path, dir) {
        path = String(path || '').trim();
        if (!path) { return ''; }
        if (/^[a-z][a-z0-9+.-]*:/i.test(path) || /[\\\u0000-\u001f?#]/.test(path) || path.indexOf('//') === 0) { return 'invalid stylesheet path'; }
        var full = /^(~\/|\/)/.test(path) ? path : (dir || '~').replace(/\/$/, '') + '/' + path;
        var home = full.indexOf('~/') === 0, parts = full.slice(home ? 2 : 1).split('/'), out = [];
        for (var part of parts) { if (!part || part === '.') { continue; } if (part === '..') { if (!out.length) { return 'invalid stylesheet path'; } out.pop(); } else { out.push(part); } }
        return (home ? '~/' : '/') + out.join('/');
    }
    async function editor(opts) {
        if (!opts.current() || (opts.signal && opts.signal.aborted)) { return null; }
        var CM = await window.yavcMdEditor.libs(), view = null, api = null, pending = false, closed = false, selected = 0;
        if (!opts.current() || (opts.signal && opts.signal.aborted)) { return null; }
        var editability = new CM.Compartment();
        var entries = opts.entries, states = [], field = null, id = 'md-style-' + (++count);
        function abort() { if (api && !closed) { api.close('cancel'); } }
        function busy(value) {
            pending = value;
            api.dialog.querySelectorAll('input, select, button').forEach(function (control) { control.disabled = value; });
            view.dispatch({ effects: editability.reconfigure([CM.EditorState.readOnly.of(value), CM.EditorView.editable.of(!value)]) });
        }
        function state(i) {
            return states[i] || (states[i] = CM.EditorState.create({ doc: entries[i].css, extensions: [editability.of([]), CM.lineNumbers(), CM.history(), CM.css(), CM.autocompletion(),
                CM.keymap.of([{ key: 'Mod-s', run: function () { if (!pending) { api.dialog.querySelector('form').requestSubmit(); } return true; } }].concat(CM.defaultKeymap, CM.historyKeymap, CM.completionKeymap, [CM.indentWithTab])), CM.EditorView.lineWrapping,
                CM.EditorView.contentAttributes.of({ 'aria-label': 'Stylesheet CSS' }), window.yavcMdEditor.codeTheme(CM),
                CM.EditorView.updateListener.of(function (u) { if (u.docChanged) { diagnostics(); } })] }));
        }
        function diagnostics() {
            if (!view || !api) { return; }
            var problems = window.yavcDocumentStyle.compile('validation', view.state.doc.toString()).problems;
            var notice = api.body.querySelector('[data-style-warning]'); notice.hidden = !problems.length; notice.querySelector('.notice-body').textContent = problems.join(' ');
        }
        function snapshot() {
            states[selected] = view.state;
            return { css: entries.map(function (entry, i) {
                var value = states[i] ? states[i].doc.toString() : entry.css;
                return value === entry.css.replace(/\r\n/g, '\n') ? entry.css : value;
            }), stylesheet: field ? field.value.trim() : '' };
        }
        try {
            return await window.yavcDialog.open({ title: opts.title, className: 'md-style-dialog',
                body: (opts.help ? '<p class="form-help">' + esc(opts.help) + '</p>' : '') +
                    (entries.length > 1 ? '<div class="form-group"><label class="form-label" for="' + id + '-block">Style Block</label><span class="form-select-wrap"><select class="form-select" id="' + id + '-block" data-style-block>' + entries.map(function (e, i) { return '<option value="' + i + '">' + esc(e.label) + '</option>'; }).join('') + '</select></span></div>' : '') +
                    (opts.stylesheet != null ? window.yavcDialog.pathField({id:id+'-shared',label:'Shared Stylesheet',value:opts.stylesheet,accept:'.css',maxBytes:256*1024,help:'Optional CSS file, applied before these style blocks.'}) : '') +
                    '<div class="md-style-code" data-style-code></div><div class="notice warn" data-style-warning hidden role="status"><div class="notice-content"><p class="notice-body"></p></div></div>',
                buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'apply', label: opts.okLabel || 'Apply Styles', primary: true }],
                check: function () {
                    if (pending) { return true; }
                    var draft = snapshot(); busy(true);
                    Promise.resolve().then(function () { if (!closed && opts.current()) { return opts.save(draft); } throw new Error('The document changed. Close and reopen the stylesheet.'); }).then(function () { if (!closed) { api.close('saved'); } }).catch(function (err) {
                        if (!closed) { window.yavcUtil.notice(api.body, err.message || String(err), true); }
                    }).finally(function () { if (!closed) { busy(false); } });
                    return true;
                },
                onOpen: function (opened) {
                    api = opened; field = api.body.querySelector('#' + id + '-shared');
                    if (field) field.setAttribute('data-style-shared','');
                    window.yavcDialog.wirePaths(api,opts);
                    api.dialog.querySelector('[type="submit"]').title = (opts.okLabel || 'Apply Styles') + ' (' + window.yavcUtil.shortcut('Mod+S') + ')';
                    if (opts.signal) { opts.signal.addEventListener('abort', abort, { once: true }); }
                    view = new CM.EditorView({ parent: api.body.querySelector('[data-style-code]'), state: state(0) });
                    var select = api.body.querySelector('[data-style-block]');
                    if (select) { select.addEventListener('change', function () { states[selected] = view.state; selected = +select.value; view.setState(state(selected)); diagnostics(); }); }
                    api.dialog.addEventListener('cancel', function (e) { if (pending) { e.preventDefault(); } });
                    api.dialog.querySelector('[data-choice="cancel"]').addEventListener('click', function (e) { if (pending) { e.preventDefault(); e.stopImmediatePropagation(); } }, true);
                    diagnostics(); Promise.resolve().then(function () { if (!closed && api.dialog.open) { view.focus(); } });
                }
            });
        } finally { closed = true; if (opts.signal) { opts.signal.removeEventListener('abort', abort); } if (view) { view.destroy(); } }
    }
    async function documentStyle(opts) {
        var blocks = opts.engine.styles(opts.text), front = opts.engine.front(opts.text), original = '';
        (front.entries || []).forEach(function (e) { if (e.key === 'stylesheet') { original = e.value; } });
        var entries = blocks.map(function (b, i) { return { label: 'Style Block ' + (i + 1) + (b.slide ? ' (This Slide)' : ''), css: b.css }; });
        entries.push({ label: opts.slide ? 'New Style Block (Whole Deck)' : 'New Style Block', css: '' });
        if (opts.slide) { entries.push({ label: 'New Style Block (This Slide)', css: '' }); }
        return editor({ title: opts.title || 'Document Stylesheet', current: opts.current, signal: opts.signal, entries: entries, stylesheet: opts.stylesheet == null ? original : opts.stylesheet,
            help: 'Styles affect this document. Unsupported CSS is kept in the source and omitted from the preview.',
            save: function (draft) {
                if (!opts.current()) { throw new Error('The document changed while the stylesheet editor was open. Close and reopen it.'); }
                var text = opts.text;
                for (var i = blocks.length - 1; i >= 0; i--) { if (draft.css[i] !== blocks[i].css) { text = text.slice(0, blocks[i].from) + (draft.css[i].endsWith('\n') ? draft.css[i] : draft.css[i] + '\n') + text.slice(blocks[i].to); } }
                var added = draft.css[blocks.length];
                if (added.trim()) { text += (text.endsWith('\n') ? '\n' : '\n\n') + '{{style}}\n' + added + (added.endsWith('\n') ? '' : '\n') + '{{/style}}\n'; }
                var local = opts.slide ? draft.css[blocks.length + 1] : '';
                if (local.trim()) { text += (text.endsWith('\n') ? '\n' : '\n\n') + '{{style:slide}}\n' + local + (local.endsWith('\n') ? '' : '\n') + '{{/style}}\n'; }
                if (draft.stylesheet !== original && opts.stylesheet == null) { text = opts.engine.frontset({ text: text, set: { stylesheet: draft.stylesheet || null }, order: ['stylesheet'] }).text; }
                return opts.save(text, draft.stylesheet);
            }
        });
    }
    async function sharedStyle(opts) {
        if (!opts.current() || (opts.signal && opts.signal.aborted)) { return null; }
        var path = resolve(opts.path, opts.dir);
        if (!/^~\/.+\.css$/i.test(path)) { throw new Error('Choose a .css file in your home folder to edit shared styles.'); }
        var before = await opts.read(path);
        if (!opts.current()) { return; }
        return editor({ title: 'Shared Stylesheet', current: opts.current, signal: opts.signal, entries: [{ label: path, css: before || '' }], help: path, okLabel: 'Save Stylesheet', save: async function (draft) {
            if (!opts.current()) { throw new Error('The document changed. Close and reopen the stylesheet.'); }
            var now = await opts.read(path);
            if (!opts.current() || now !== before) { throw new Error('The stylesheet or document changed. Close and reopen it before saving.'); }
            var error = await opts.write(path, draft.css[0]); if (error) { throw new Error(error); }
        } });
    }
    window.yavcStyleTools = Object.freeze({ resolve: resolve, document: documentStyle, shared: sharedStyle });
})();
