/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* One browser-local LSP session per host application. Hosts own documents,
   notices and storage. A new extension gets a fresh URI for its open instance. */
(function () {
    'use strict';
    if (window.yavcLanguageServices) { return; }
    var self = document.currentScript.src;
    var workerURL = self.replace(/bridge\.js(?=\?|$)/, 'worker.js');
    // onError receives {persistent:true} for ongoing availability problems,
    // and {persistent:false} for a refused individual operation.
    function create(CM, onError) {
        var worker = null, client = null, disposed = false, generation = 0, failed = false, reports = new Map(), views = new Map(), endRequests = null;
        var prefix = 'yavc-lsp://' + crypto.randomUUID() + '/';
        var diagnosticsChanged = CM.StateEffect.define();
        function fallback(kind) { return kind === 'json' || kind === 'canvas' ? [CM.linter(CM.jsonParseLinter()), CM.lintGutter()] : []; }
        function stop() {
            if (endRequests) { endRequests(); endRequests = null; }
            if (client) { client.disconnect(); client = null; }
            if (worker) { worker.terminate(); worker = null; }
            reports.clear();
        }
        function fail() {
            if (disposed || failed) { return; }
            failed = true; stop();
            Array.from(views.entries()).forEach(function (entry) {
                Promise.resolve().then(function () {
                    var view = entry[0], mounted = entry[1];
                    if (!disposed && views.get(view) === mounted && mounted.scope.get(view.state) !== undefined) { view.dispatch({ effects: mounted.scope.reconfigure(fallback(mounted.kind)) }); }
                });
            });
            if (onError) { onError('Language assistance is unavailable. Your text is kept; use Restart Language Assistance to retry.', {persistent:true}); }
        }
        function ensure() {
            if (disposed || failed) { return null; }
            if (client) { return client; }
            try {
                worker = new Worker(workerURL);
                var ownWorker = worker, handlers = new Set(), pending = new Map(), transport = {
                    send: function (message) {
                        if (worker !== ownWorker) { return; }
                        if (message.length > 8388608) { fail(); return; }
                        var packet = JSON.parse(message);
                        if (packet.id != null && packet.method) { pending.set(packet.id, { method: packet.method, timer: setTimeout(function () { if (worker === ownWorker) { fail(); } }, 4500) }); }
                        ownWorker.postMessage(message);
                    },
                    subscribe: function (handler) { handlers.add(handler); },
                    unsubscribe: function (handler) { handlers.delete(handler); }
                };
                endRequests = function () {
                    pending.forEach(function (request, id) {
                        clearTimeout(request.timer);
                        var packet = request.method === 'initialize' ? { jsonrpc: '2.0', id: id, error: { code: -32800, message: 'Language session ended.' } } : { jsonrpc: '2.0', id: id, result: null };
                        handlers.forEach(function (handler) { handler(JSON.stringify(packet)); });
                    });
                    pending.clear();
                };
                worker.onmessage = function (e) {
                    if (typeof e.data !== 'string' || worker !== ownWorker) { return; }
                    var packet;
                    try { packet = JSON.parse(e.data); } catch (err) { fail(); return; }
                    var request = pending.get(packet.id);
                    if (request) { clearTimeout(request.timer); pending.delete(packet.id); }
                    handlers.forEach(function (handler) { handler(e.data); });
                };
                worker.onerror = function (e) { e.preventDefault(); if (worker === ownWorker) { fail(); } };
                worker.onmessageerror = function () { if (worker === ownWorker) { fail(); } };
                var active = new CM.LSPClient({ timeout: 5000, sanitizeHTML: function (html) { return window.yavcHTML.clean(html); },
                    extensions: [CM.hoverTooltips(), CM.serverDiagnostics()],
                    notificationHandlers: { 'textDocument/publishDiagnostics': function (source, params) {
                        var file = source.workspace.getFile(params.uri), view = file && file.getView();
                        if (file && view && params.version === file.version && source === client) {
                            reports.set(params.uri, { doc: file.doc, version: params.version, diagnostics: params.diagnostics });
                            view.dispatch({ effects: diagnosticsChanged.of(null) });
                            CM.forceLinting(view);
                        }
                        return true;
                    } }
                }).connect(transport);
                client = active;
                active.initializing.catch(function () { if (client === active) { fail(); } });
                return client;
            } catch (err) { fail(); return null; }
        }
        function extension(kind, readOnly) {
            if (['json','canvas','css','html'].indexOf(kind) < 0) { return []; }
            var active = ensure();
            if (!active) { return fallback(kind); }
            var scope = new CM.Compartment();
            function complete(context) {
                if (disposed || failed || client !== active) { return null; }
                return Promise.resolve(CM.serverCompletionSource(context)).then(function (result) {
                    if (!result) { return null; }
                    if (kind === 'css' || kind === 'html') { return result; }
                    // LSP edits replace the whole JSON token, including a closing
                    // quote after the cursor. Match only the prefix being typed,
                    // while retaining the complete replacement range for edits.
                    var prefix = context.state.sliceDoc(result.from, context.pos).replace(/^"/, '').toLowerCase();
                    return Object.assign({}, result, { filter: false, validFor: undefined, options: result.options.filter(function (option) { return option.label.replace(/^"/, '').toLowerCase().startsWith(prefix); }) });
                }).catch(function () { return null; });
            }
            function services() {
                var uri = prefix + 'document-' + (++generation) + '.' + kind;
                return [active.plugin(uri, kind === 'canvas' ? 'json' : kind), readOnly ? [] : CM.autocompletion({ override: [complete], activateOnTyping: true }), CM.keymap.of([{ key: 'Shift-Alt-f', run: format }]),
                CM.linter(function (view) {
                    var report = reports.get(uri), file = active.workspace.getFile(uri);
                    if (!report || !file || file.version !== report.version || !view.state.doc.eq(report.doc)) { return kind === 'json' || kind === 'canvas' ? CM.jsonParseLinter()(view) : []; }
                    function offset(p) { var line = view.state.doc.line(Math.max(1, Math.min(view.state.doc.lines, p.line + 1))); return line.from + Math.max(0, Math.min(line.length, p.character)); }
                    return report.diagnostics.map(function (d) { return { from: offset(d.range.start), to: offset(d.range.end), severity: d.severity === 2 ? 'warning' : d.severity === 3 ? 'info' : d.severity === 4 ? 'hint' : 'error', message: d.message }; });
                }, { delay: 250, needsRefresh: function (update) { return update.transactions.some(function (tr) { return tr.effects.some(function (effect) { return effect.is(diagnosticsChanged); }); }); } }),
                CM.ViewPlugin.define(function () { return { destroy: function () { reports.delete(uri); } }; })];
            }
            return [scope.of([]), CM.ViewPlugin.define(function (view) {
                var mounted = { scope: scope, suspended: null, kind: kind }; views.set(view, mounted);
                function checkSize() {
                    Promise.resolve().then(function () {
                        if (disposed || failed || client !== active || views.get(view) !== mounted) { return; }
                        var oversized = view.state.doc.length > 2097152;
                        if (oversized === mounted.suspended) { return; }
                        mounted.suspended = oversized;
                        view.dispatch({ effects: scope.reconfigure(oversized ? [] : services()) });
                        if (oversized && onError) { onError('Language assistance is paused for this large document. It resumes automatically when the document is smaller.', {persistent:true}); }
                    });
                }
                checkSize();
                return { update: function (update) { if (update.docChanged) { checkSize(); } }, destroy: function () { if (views.get(view) === mounted) { views.delete(view); } } };
            })];
        }
        function format(view) {
            if (!view || view.state.readOnly || !client || !client.connected) { return false; }
            var active = client, plugin = CM.LSPPlugin.get(view), doc = view.state.doc;
            if (!plugin) { return false; }
            active.initializing.then(function () {
                if (disposed || client !== active || CM.LSPPlugin.get(view) !== plugin || view.state.doc !== doc || view.state.readOnly) { return; }
                active.sync();
                return active.request('textDocument/formatting', { textDocument: { uri: plugin.uri }, options: { tabSize: 2, insertSpaces: true } }).then(function (edits) {
                    if (!edits || disposed || client !== active || CM.LSPPlugin.get(view) !== plugin || view.state.doc !== doc || view.state.readOnly) { return; }
                    function offset(p) {
                        if (!p || !Number.isInteger(p.line) || p.line < 0 || p.line >= doc.lines) { throw new Error('Invalid edit position.'); }
                        var line = doc.line(p.line + 1);
                        if (!Number.isInteger(p.character) || p.character < 0 || p.character > line.length) { throw new Error('Invalid edit position.'); }
                        return line.from + p.character;
                    }
                    var changes = edits.map(function (edit) {
                        var from = offset(edit.range.start), to = offset(edit.range.end);
                        if (to < from || typeof edit.newText !== 'string') { throw new Error('Invalid language edit.'); }
                        return { from: from, to: to, insert: edit.newText };
                    }).sort(function (a, b) { return a.from - b.from; });
                    var end = 0, length = doc.length;
                    changes.forEach(function (change) { if (change.from < end) { throw new Error('Overlapping language edits.'); } end = change.to; length += change.insert.length - (change.to - change.from); });
                    if (length > 2097152) { throw new Error('Formatted text exceeds the language-service size limit.'); }
                    view.dispatch({ changes: changes, userEvent: 'format' });
                });
            }).catch(function () { if (!disposed && client === active && CM.LSPPlugin.get(view) === plugin && onError) { onError('The document could not be formatted. Your text is unchanged.', {persistent:false}); } });
            return true;
        }
        return {
            extension: extension, format: format,
            restart: function () { if (!disposed) { stop(); failed = false; } },
            dispose: function () { disposed = true; stop(); },
            ready: function () { return !!(client && client.serverCapabilities); }
        };
    }
    window.yavcLanguageServices = Object.freeze({ create: create });
})();
