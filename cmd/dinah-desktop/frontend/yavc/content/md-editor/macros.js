/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Document-local shortcode metadata and editing. No registry is global. */
(function () {
    'use strict';
    var K = window.yavcMdEditor, esc = window.yavcUtil.esc, count = 0;
    async function prepare(engine, text, host) {
        var shared = '', path = '', front = engine.front(text), directory = host.dir ? host.dir() : '~';
        await K.styles();
        (front.entries || []).forEach(function (entry) { if (entry.key === 'shortcodes') { path = entry.value; } });
        var reference = path, problem = '';
        /* The shared file is read as a diagram a document shows by path is
           read: through the host's own reading where it has one, and
           through the site's files otherwise. A file that is missing or
           cannot be read does not stop the document from being drawn. Its
           shortcodes stay as they are written, and problem says why. */
        if (path) {
            try {
                path = host.resolve ? host.resolve(path, 'shortcodes') : window.yavcStyleTools.resolve(path, directory);
                if (!/^(~\/|\/(?!\/)).+\.md$/i.test(path)) { throw new Error('The shared shortcode file must be a local Markdown file: ' + reference); }
                var found;
                if (typeof host.read === 'function') { found = await host.read(path, { signal: host.signal, maxBytes: 256 * 1024 }); }
                else {
                    if (!host.site) { throw new Error('Shared shortcodes need a way to read files here.'); }
                    var site = await host.site(), node = site.F && site.F.resolve(null, path);
                    found = node && !node.children ? await site.F.read(node) : null;
                }
                if (found == null) { throw new Error('The shared shortcode file was not found: ' + reference); }
                if (typeof found !== 'string' || new TextEncoder().encode(found).length > 256 * 1024) { throw new Error('The shared shortcode file must be text no larger than 256 KiB: ' + reference); }
                shared = found;
            } catch (err) { shared = ''; problem = String(err && err.message || err); }
        }
        return { engine: engine.withMacros(text, shared), shared: shared, path: path, reference: reference, problem: problem };
    }
    function catalog(engine) {
        var entries = engine ? engine.macros().entries : [], byName = new Map();
        entries.forEach(function (entry) { byName.set(entry.name, entry); });
        return Array.from(byName.values()).filter(function (entry) { return !entry.problem; });
    }
    var metadataCache=new WeakMap();
    function metadata(engine) {
        if(engine&&metadataCache.has(engine))return metadataCache.get(engine);
        var items=catalog(engine).map(function (entry) {
            var d = entry.definition;
            return { name: d.name, title: d.name, description: 'Custom shortcode' + (entry.shared ? ' from the shared definitions file.' : ' defined in this document.'),
                kind: d.container ? 'container' : d.inline ? 'inline' : 'block',
                params: (d.parameters || []).map(function (p) { return { name: p.name, description: p.help || '', required: false }; }),
                example: '{{' + d.name + '}}' + (d.container ? '\nText.\n{{/' + d.name + '}}' : '') };
        });
        if(engine)metadataCache.set(engine,items);
        return items;
    }
    function find(engine, name) { return catalog(engine).find(function (entry) { return entry.name === name; }); }
    function ask(engine, name, existing, host) {
        if (host.signal && host.signal.aborted) { return Promise.resolve(null); }
        var entry = find(engine, name);
        if (!entry) { return undefined; }
        var d = entry.definition, first = existing ? existing.split('\n')[0] : null;
        var identity = host.identity ? host.identity() : null, text = host.text ? host.text() : null;
        var form = { title: name, inline: d.inline, signal: host.signal,
            current: function () { return !(host.signal && host.signal.aborted) && (!host.identity || host.identity() === identity) && (!host.text || host.text() === text); },
            fields: (d.parameters || []).map(function (p) { return { key: p.name, label: p.name.replace(/(^|-)([a-z])/g, function (_, space, c) { return (space ? ' ' : '') + c.toUpperCase(); }), help: p.help }; }),
            read: function (sc) {
                var values = {}; (d.parameters || []).forEach(function (p, i) { values[p.name] = Object.prototype.hasOwnProperty.call(sc.named, p.name) ? sc.named[p.name] : sc.positional[i] == null ? p.default : sc.positional[i]; }); return values;
            },
            write: function (values) {
                var args = [];
                for (var p of (d.parameters || [])) {
                    var value = values[p.name];
                    if (/[{}\r\n\u0000]/.test(value)) { return { problem: 'Arguments cannot contain braces, line breaks or null characters in shortcode source.', field: p.name }; }
                    args.push(p.name + '=' + '"' + value.replace(/(["\\])/g, '\\$1') + '"');
                }
                return '{{' + name + (args.length ? ':' + args.join(' ') : '') + '}}';
            }
        };
        return K.askShortcode(name, first, host, form).then(function (opening) {
            if (opening == null || opening === '' || !d.container) { return opening; }
            return opening + (existing ? existing.slice(first.length) : '\nText.\n{{/' + name + '}}');
        });
    }
    function editBlock(source, engine, host) {
        var match = /^\{\{([a-z][a-z0-9-]*)(?::[^\n]*)?\}\}/.exec(source.trim());
        if (!match || !find(engine, match[1])) { return undefined; }
        return ask(engine, match[1], source, host);
    }
    function completions(CM, getEngine) {
        return CM.autocompletion({ override: [function (context) {
            var engine = getEngine(); if (!engine) { return null; }
            var before = context.matchBefore(/\{\{\/?[a-z0-9-]*$/), list = metadata(engine);
            if (before) {
                var closing = before.text.indexOf('{{/') === 0;
                return { from: before.from + (closing ? 3 : 2), validFor: /^[a-z0-9-]*$/,
                    options: list.filter(function (item) { return !closing || item.kind === 'container'; }).map(function (item) { return { label: item.name, type: 'keyword', detail: 'Custom Shortcode', info: item.description, apply: item.name + (!closing && item.params.length ? ':' : '') }; }) };
            }
            var args = context.matchBefore(/\{\{([a-z][a-z0-9-]*):[^}]*$/);
            if (!args) { return null; }
            var name = /^\{\{([a-z][a-z0-9-]*)/.exec(args.text)[1], item = list.find(function (entry) { return entry.name === name; }), word = /[a-z0-9-]*$/.exec(args.text)[0];
            return item ? { from: context.pos - word.length, validFor: /^[a-z0-9-]*$/, options: item.params.map(function (param) { return { label: param.name, type: 'property', info: param.description, apply: param.name + '=' }; }) } : null;
        }], icons: false });
    }
    async function manage(opts) {
        var context = await prepare(opts.engine, opts.text, opts.host);
        if (!opts.current() || opts.signal.aborted) { return; }
        var entries = context.engine.macros().entries.filter(function (entry) { return !entry.shared; });
        var drafts = entries.map(function (entry) { return { name: entry.name, body: entry.body }; });
        drafts.push({ name: '', body: '{{param:title default="A title" help="Heading text"}}\n## {{arg:title}}\n\n{{content}}\n' });
        var newline = opts.text.indexOf('\r\n') >= 0 && opts.text.replace(/\r\n/g, '').indexOf('\n') < 0 ? '\r\n' : '\n';
        function bodyText(body) { var value = body.replace(/\r\n/g, '\n').replace(/\n/g, newline); return value.endsWith('\n') ? value : value + newline; }
        var id = 'md-macros-' + (++count), api = null, selected = 0, busy = false, closed = false;
        function capture() { var body = api.body.querySelector('[data-macro-body]').value, old = drafts[selected].body; drafts[selected] = { name: api.body.querySelector('[data-macro-name]').value.trim(), body: body === old.replace(/\r\n/g, '\n') ? old : body }; }
        function show() { var draft = drafts[selected]; api.body.querySelector('[data-macro-name]').value = draft.name; api.body.querySelector('[data-macro-name]').readOnly = selected < entries.length; api.body.querySelector('[data-macro-body]').value = draft.body; }
        function abort() { if (api && !closed) { api.close('cancel'); } }
        try {
            return await window.yavcDialog.open({ title: 'Custom Shortcodes', className: 'md-style-dialog',
                body: '<p class="form-help">' + esc(opts.help || 'Definitions belong to this document.') + ' Each parameter becomes a field when you insert the shortcode. Put {{content}} on its own template line to edit the body as rich text.</p>' +
                    '<div class="form-group"><label class="form-label" for="' + id + '-select">Definition</label><span class="form-select-wrap"><select class="form-select" id="' + id + '-select" data-macro-select>' + drafts.map(function (draft, i) { return '<option value="' + i + '">' + esc(i === entries.length ? 'New Definition' : draft.name) + '</option>'; }).join('') + '</select></span></div>' +
                    '<div class="form-group"><label class="form-label" for="' + id + '-name">Shortcode Name</label><input class="form-input" id="' + id + '-name" data-macro-name placeholder="callout" aria-describedby="' + id + '-name-error"><p class="form-error" id="' + id + '-name-error" data-macro-name-error role="alert" hidden></p></div>' +
                    '<div class="form-group"><label class="form-label" for="' + id + '-body">Template Markdown</label><textarea class="form-textarea" id="' + id + '-body" data-macro-body rows="12" spellcheck="false"></textarea></div>' +
                    window.yavcDialog.pathField({id:id+'-shared',label:'Shared Definitions File',value:context.reference,accept:'.md',maxBytes:256*1024,help:'Optional local Markdown file. Edit it in Editor; this document can override its definitions.'}),
                buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'apply', label: 'Apply Definitions', primary: true }],
                onOpen: function (opened) {
                    api = opened; show(); opts.signal.addEventListener('abort', abort, { once: true });
                    api.body.querySelector('#'+id+'-shared').setAttribute('data-macro-shared','');
                    window.yavcDialog.wirePaths(api,Object.assign({},opts,{base:opts.host && opts.host.dir ? opts.host.dir() : null}));
                    api.body.querySelector('[data-macro-select]').addEventListener('change', function (event) { capture(); selected = +event.target.value; show(); });
                    api.dialog.addEventListener('cancel', function (event) { if (busy) { event.preventDefault(); } });
                    api.dialog.querySelector('[data-choice="cancel"]').addEventListener('click', function (event) { if (busy) { event.preventDefault(); event.stopImmediatePropagation(); } }, true);
                },
                check: function () {
                    if (busy) { return true; }
                    capture();
                    var nameField = api.body.querySelector('[data-macro-name]'), nameError = api.body.querySelector('[data-macro-name-error]'), addedName = drafts[entries.length].name;
                    if (addedName && !/^[a-z][a-z0-9-]*$/.test(addedName)) {
                        nameError.textContent = 'A shortcode name must start with a lowercase letter and contain only lowercase letters, digits and hyphens.'; nameError.hidden = false;
                        api.body.querySelector('[data-macro-select]').value = String(entries.length); selected = entries.length; show(); nameField.setAttribute('aria-invalid', 'true'); nameField.focus(); return true;
                    }
                    nameError.hidden = true; nameField.removeAttribute('aria-invalid');
                    busy = true; api.dialog.querySelectorAll('input, textarea, select, button').forEach(function (element) { element.disabled = true; });
                    Promise.resolve().then(async function () {
                        if (!opts.current()) { throw new Error('The document changed. Close and reopen Custom Shortcodes.'); }
                        var text = opts.text;
                        for (var i = entries.length - 1; i >= 0; i--) { if (drafts[i].body !== entries[i].body) { text = text.slice(0, entries[i].from) + bodyText(drafts[i].body) + text.slice(entries[i].to); } }
                        var added = drafts[entries.length];
                        if (added.name) { text += (text.endsWith('\n') ? newline : newline + newline) + '{{define:' + added.name + '}}' + newline + bodyText(added.body) + '{{/define}}' + newline; }
                        var shared = api.body.querySelector('[data-macro-shared]').value.trim();
                        if (shared !== context.reference) { text = opts.engine.frontset({ text: text, set: { shortcodes: shared || null }, order: ['shortcodes'] }).text; if (newline === '\r\n') { text = text.replace(/\r?\n/g, newline); } }
                        var next = await prepare(opts.engine, text, opts.host), problem = next.engine.macros().entries.find(function (entry) { return entry.problem; });
                        if (next.problem) { throw new Error(next.problem); }
                        if (problem) { throw new Error(problem.name + ': ' + problem.problem); }
                        var local = next.engine.macros().entries.filter(function (entry) { return !entry.shared; });
                        if (local.length !== entries.length + (added.name ? 1 : 0) || local.some(function (entry, i) { return entry.name !== drafts[i].name || entry.body !== (i < entries.length && drafts[i].body === entries[i].body ? entries[i].body : bodyText(drafts[i].body)); })) {
                            throw new Error('The definitions did not round-trip as written. Close unfinished Markdown blocks and keep {{/define}} outside the template body.');
                        }
                        if (closed || !opts.current() || opts.signal.aborted) { throw new Error('The document changed. Close and reopen Custom Shortcodes.'); }
                        await opts.save(text); api.close('saved');
                    }).catch(function (err) { if (!closed) { window.yavcUtil.notice(api.body, err.message, true); } }).finally(function () { busy = false; if (!closed) { api.dialog.querySelectorAll('input, textarea, select, button').forEach(function (element) { element.disabled = false; }); } });
                    return true;
                }
            });
        } finally { closed = true; opts.signal.removeEventListener('abort', abort); }
    }
    window.yavcMacroTools = Object.freeze({ prepare: prepare, catalog: catalog, metadata: metadata, ask: ask, editBlock: editBlock, manage: manage, completions: completions });
})();
