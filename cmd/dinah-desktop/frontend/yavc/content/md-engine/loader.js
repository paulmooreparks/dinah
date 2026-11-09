/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* The Markdown engine, yavc-md.wasm (cmd/yavc-md): Dinah Desktop's preview
   renderer and page view with parkscomputing.com's shortcodes, run here in
   the browser so a document's text never leaves it. The Editor and Slides
   share it: window.yavcMdEngine() is a promise of an object whose functions
   take and return values rather than JSON:

     preview(text)  -> { blocks: [{ line, html, diagrams }], problems: [{ line, severity, message }] }
     doc(text)      -> the page view's document (ProseMirror JSON)
     markdown(doc)  -> the document's text
     blocks(text)   -> new blocks to insert
     source(src)    -> { html, diagrams }

   The files sit beside this one; the page gives this script's address a
   version, which the engine's takes too. */
(function () {
    'use strict';
    if (window.yavcMdEngine) { return; }
    var self = (document.currentScript && document.currentScript.src) || '/applets/md-engine/loader.js';
    var base = self.split('?')[0].replace(/[^/]*$/, ''), stamp = self.indexOf('?') >= 0 ? self.slice(self.indexOf('?')) : '';
    var ready = null, initialized = null;
    var load = window.yavcUtil.loadScript;
    window.yavcMdEngine = function () {
        if (ready) { return ready; }
        ready = (async function () {
            if (!document.querySelector('link[data-md-page]')) {
                var link = document.createElement('link');
                link.rel = 'stylesheet'; link.href = base + 'page.css' + stamp; link.setAttribute('data-md-page', '');
                document.head.appendChild(link);
            }
            if (!window.yavcHTML) { await load(base + 'safe-html.js' + stamp); }
            if (!window.Go) { await load(base + 'wasm_exec.js' + stamp); }
            if (!window.yavcMd) {
                var go = new window.Go();
                var url = base + 'yavc-md.wasm' + stamp, result;
                /* Compiled as it downloads; instantiateStreaming needs the
                   response served as application/wasm, which the engine
                   does, and where it cannot stream the bytes are compiled
                   once they are all here. */
                if (WebAssembly.instantiateStreaming) {
                    try { result = await WebAssembly.instantiateStreaming(fetch(url, { credentials: 'same-origin' }), go.importObject); }
                    catch (err) { result = null; }
                }
                if (!result) {
                    var r = await fetch(url, { credentials: 'same-origin' });
                    if (!r.ok) { throw new Error('the Markdown engine could not be fetched (' + r.status + ')'); }
                    result = await WebAssembly.instantiate(await r.arrayBuffer(), go.importObject);
                }
                go.run(result.instance);
            }
            function call(name) {
                return function (arg) {
                    var out = JSON.parse(window.yavcMd[name](typeof arg === 'string' ? arg : JSON.stringify(arg)));
                    if (out && typeof out === 'object' && !Array.isArray(out) && out.error) { throw new Error(out.error); }
                    return out;
                };
            }
            var api = { preview: call('preview'), doc: call('doc'), markdown: call('markdown'), blocks: call('blocks'), source: call('source'), container: call('container'), inline: call('inline'), glyphs: call('glyphs'), front: call('front'), frontset: call('frontset'), styles: call('styles') };
            api.outline = call('outline'); api.outlineState = call('outline-state'); api.moveSection = call('section-move');
            api.shortcodes = call('shortcodes');
            api.review=call('review');api.resolveReview=call('review-resolve');api.proposeReview=call('review-propose');api.inlineReview=call('review-inline');api.editReview=call('review-edit');
            api.withMacros = function (definitions, shared) {
                var context = Object.create(api), run = call('macro-context');
                context.exportPreview = function (text) { return call('export-preview')({ text: text, definitions: definitions || '', shared: shared || '' }); };
                ['preview', 'doc', 'blocks', 'source', 'inline', 'container'].forEach(function (op) {
                    context[op] = function (text) { return run({ op: op, text: text, definitions: definitions || '', shared: shared || '' }); };
                });
                context.macros = function () { return run({ op: 'catalog', definitions: definitions || '', shared: shared || '' }); };
                return context;
            };
            initialized = api;
            return api;
        })();
        ready.catch(function () { ready = null; });
        return ready;
    };
    // Synchronous DOM parse hooks run only after the shared engine has loaded.
    window.yavcMdEngine.current = function () {
        if (!initialized) throw new Error('The Markdown engine is not ready.');
        return initialized;
    };
})();
