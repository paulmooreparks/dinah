/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Saved diagram dependencies remain under the receiving host's authority. */
(function () {
    'use strict';
    var sessions = new WeakMap(), MAX_FILE = 2 * 1024 * 1024, MAX_TOTAL = 8 * 1024 * 1024, MAX_FILES = 16;
    function notice(figure, message) { var body = figure.querySelector('[data-diagram-body]'); if (body) window.yavcUtil.notice(body, message, message ? 'warn' : undefined); }
    function create(host, render) {
        var root = host.root, entries = new Map(), life = new AbortController(), epoch = 0, timer = null, stopWatch = null, stopped = false;
        var owner = host.identity(), directory = host.dir ? host.dir() : null;
        function alive() { return !stopped && root.isConnected && !host.signal.aborted && host.identity() === owner && (host.dir ? host.dir() : null) === directory; }
        function stop() { if (stopped) return; stopped = true; epoch++; clearTimeout(timer); if (stopWatch) stopWatch(); observer.disconnect(); life.abort(); host.signal.removeEventListener('abort', stop); entries.clear(); sessions.delete(root); }
        function cleanup() { if(!alive()){stop();return;} entries.forEach(function (_, figure) { if (!figure.isConnected || !root.contains(figure)) entries.delete(figure); }); if (!entries.size) stop(); }
        var observer = new MutationObserver(cleanup); observer.observe(root, {childList:true,subtree:true});
        host.signal.addEventListener('abort', stop, {once:true});
        function request() { var token = ++epoch; clearTimeout(timer); timer = setTimeout(function () { refresh(token); }, 100); }
        async function refresh(token) {
            if (!alive() || token !== epoch) return;
            var reads = new Map(), total = 0;
            for (var entry of Array.from(entries.values())) {
                if (!alive() || token !== epoch) return;
                var figure = entry.figure;
                if (!figure.isConnected || !root.contains(figure)) continue;
                figure.setAttribute('aria-busy', 'true');
                try {
                    var reference = figure.getAttribute('data-diagram-src') || '';
                    if (!/\.(d2|mmd|mermaid|canvas)$/i.test(reference) || /[:\\?#\u0000-\u001f\u007f]/.test(reference) || reference.startsWith('//') || reference.startsWith('~') && !reference.startsWith('~/')) throw new Error('Use a local D2, Mermaid or Canvas file.');
                    if (!/^(~\/|\/)/.test(reference) && !directory) throw new Error('Save this document first, or choose an absolute virtual file path.');
                    var path = window.yavcStyleTools.resolve(reference, directory);
                    if (!/^(~\/|\/(?!\/))/.test(path)) throw new Error('The diagram path cannot be resolved by this host.');
                    if (!reads.has(path)) {
                        if (reads.size >= MAX_FILES) throw new Error('A document can reference at most 16 different diagram files.');
                        if (total >= MAX_TOTAL) throw new Error('The referenced diagrams exceed the 8 MiB document budget.');
                        var record = {}; reads.set(path, record);
                        try {
                            var text = await host.read(path, {signal:life.signal,maxBytes:MAX_FILE});
                            if (!alive() || token !== epoch || !figure.isConnected) return;
                            if (typeof text !== 'string') throw new Error('The diagram file was not found: ' + reference);
                            if (text.length > MAX_FILE) throw new Error('A referenced diagram must be no larger than 2 MiB.');
                            var bytes = new TextEncoder().encode(text).byteLength;
                            if(bytes>MAX_FILE)throw new Error('A referenced diagram must be no larger than 2 MiB.');
                            total += bytes;
                            if (total > MAX_TOTAL) throw new Error('The referenced diagrams exceed the 8 MiB document budget.');
                            record.text = text;
                        } catch(error) {record.error=error;}
                    }
                    if(reads.get(path).error)throw reads.get(path).error;
                    var source = reads.get(path).text, signature = path + '\n' + source;
                    if (entry.signature !== signature) {
                        var stage = document.createElement('div'); stage.hidden = true; figure.append(stage);
                        try {
                            var result = await render(/\.canvas$/i.test(path) ? 'canvas' : /\.d2$/i.test(path) ? 'd2' : 'mermaid', stage, source, reference);
                            if (!alive() || token !== epoch || !figure.isConnected || !entries.has(figure)) { stage.remove(); return; }
                            if (result && (result.problem || (result.problems || []).length)) throw new Error(result.problem || result.problems[0].message);
                            if(!stage.querySelector('img,svg'))throw new Error(stage.textContent.trim()||'The diagram renderer returned no image.');
                            var image = stage.querySelector('img'); if (image && figure.dataset.diagramAlt) image.alt = figure.dataset.diagramAlt;
                            var svg = stage.querySelector('svg'); if(svg && figure.dataset.diagramAlt)svg.setAttribute('aria-label',figure.dataset.diagramAlt);
                            var body = figure.querySelector('[data-diagram-body]'); body.replaceChildren.apply(body, Array.from(stage.childNodes)); entry.signature = signature;
                        } finally { stage.remove(); }
                    }
                    notice(figure, '');
                } catch (error) {
                    if (alive() && token === epoch && figure.isConnected) notice(figure, 'The diagram could not be loaded: ' + error.message + (entry.signature ? ' The image is the last saved version that loaded.' : ''));
                } finally { if (alive() && token === epoch && figure.isConnected) figure.removeAttribute('aria-busy'); }
            }
            cleanup();
        }
        window.addEventListener('focus', request, {signal:life.signal});
        document.addEventListener('visibilitychange', function () { if (!document.hidden) request(); }, {signal:life.signal});
        try { if (typeof host.watch === 'function') stopWatch = host.watch(request); } catch(error) {stop();throw error;}
        return {owner:owner,directory:directory,stop:stop,stopped:function(){return stopped;},add:function(target,nextHost){
            if(stopped)return;host=nextHost;
            entries.forEach(function(_,figure){if(!figure.isConnected||!root.contains(figure))entries.delete(figure);});
            target.querySelectorAll('[data-diagram-src]').forEach(function(figure){
                if(entries.has(figure))return;if(entries.size>=32){notice(figure,'A document can display at most 32 diagram references.');return;}
                var body=figure.querySelector('[data-diagram-body]');if(!body)return;body.replaceChildren(document.createTextNode('Diagram: '+figure.getAttribute('data-diagram-src')));notice(figure,'');
                entries.set(figure,{figure:figure,signature:null});
            });request();}};
    }
    function attach(target, host, render) {
        if (!target.isConnected) return;
        var figures = target.querySelectorAll('[data-diagram-src]'); if (!figures.length) return;
        if (!host || !host.root || !host.signal || !host.identity || !host.read) { figures.forEach(function (figure) { notice(figure, 'This host cannot read saved diagram dependencies.'); }); return; }
        if(host.signal.aborted)return;
        var session = sessions.get(host.root), owner = host.identity(), directory = host.dir ? host.dir() : null;
        if (session && (session.owner !== owner || session.directory !== directory)) { session.stop(); session = null; }
        if (!session || session.stopped()) { session = create(host, render); sessions.set(host.root, session); }
        session.add(target, host);
    }
    window.yavcDiagramReferences = Object.freeze({attach:attach});
})();
