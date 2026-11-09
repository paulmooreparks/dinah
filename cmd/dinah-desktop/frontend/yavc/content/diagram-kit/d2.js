/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* D2 in the browser, for Diagrams, the Editor and Slides
   (docs/diagrams-design.md). The engine, yavc-d2.wasm, runs in a worker
   (worker.js) under a policy of its own; this script compiles the engine
   once, starts the worker when a diagram is first drawn, sends it one
   request at a time, stops it and starts another when a request runs too
   long, and keeps the drawings it was given, so a diagram drawn again with
   the same text and theme is not compiled again.

     yavcD2Engine.compile(req, key) -> Promise of { svg, width, height, problems, icons },
                                       or null when a newer request with the same key replaced it
     yavcD2Engine.draw(place, source, opts) draws a diagram into place, as an image,
                                       and resolves with what compile gave, or null
     yavcD2Engine.call(name, req, key) -> Promise of another function's answer:
                                       completions, format or outline (internal/d2api)

   draw's opts: alt, the image's text alternative; keep, to leave the last
   good drawing in place, dimmed, when the text has a mistake, as Diagrams'
   preview does (decision 5); report: false, to leave the mistake to the
   caller instead of showing the site's notice above the diagram.

   A drawn diagram is an image (decision 10): the SVG is shown through an
   img element with a data address, which the browser draws in a static
   mode that runs no script, loads nothing and cannot restyle the page. A
   data address, and not a blob one, so that a page exported as a file
   keeps its diagrams. The files sit beside this one, and the page gives
   this script's address a version, which theirs take too. */
(function () {
    'use strict';
    if (window.yavcD2Engine) { return; }
    var here = (document.currentScript && document.currentScript.src) || '/applets/diagram-kit/d2.js';
    var base = here.split('?')[0].replace(/[^/]*$/, ''), stamp = here.indexOf('?') >= 0 ? here.slice(here.indexOf('?')) : '';
    /* How long one request may run before the worker is stopped (decision
       3). Measured in Chromium on 2026-11-03, dagre took 0.13 s for 10
       shapes, 0.35 s for 200 and 1.5 s for 1,000, growing about linearly,
       so the 2,000 the engine allows (MaxObjects) take about 3 s, and 20 s
       leaves room for a device several times slower. */
    var LIMIT_MS = 20000;
    /* The site's light and dark modes draw with D2's default theme and its
       Dark Mauve; a diagram's own theme-id wins (decision 12). */
    var THEME = { light: 0, dark: 200 };
    var CACHE_MAX = 40;

    var engine = null, workerReady = null, worker = null;
    var queue = [], busy = null, seq = 0, cache = new Map();

    function fetchCompiled(url) {
        return fetch(url, { credentials: 'same-origin' }).then(function (r) {
            if (!r.ok) { throw new Error('the diagram engine could not be fetched (' + r.status + ')'); }
            return r.arrayBuffer();
        }).then(function (bytes) { return WebAssembly.compile(bytes); });
    }
    /* The engine is compiled here, once, and handed to each worker, which
       can fetch nothing itself. compileStreaming compiles as it downloads,
       which needs the file served as application/wasm, as the site does. */
    function compiledEngine() {
        if (!engine) {
            var url = base + 'yavc-d2.wasm' + stamp;
            engine = WebAssembly.compileStreaming
                ? WebAssembly.compileStreaming(fetch(url, { credentials: 'same-origin' })).catch(function () { return fetchCompiled(url); })
                : fetchCompiled(url);
            engine.catch(function () { engine = null; });
        }
        return engine;
    }
    function startWorker() {
        if (workerReady) { return workerReady; }
        workerReady = compiledEngine().then(function (module) {
            return new Promise(function (resolve, reject) {
                var w = new Worker(base + 'worker.js' + stamp);
                var timer = setTimeout(function () { w.terminate(); reject(new Error('the diagram engine did not start in time')); }, LIMIT_MS);
                w.onmessage = function (e) {
                    var m = e.data || {};
                    if (m.type === 'ready') { clearTimeout(timer); w.onmessage = onResult; worker = w; resolve(w); }
                    else if (m.type === 'failed') { clearTimeout(timer); w.terminate(); reject(new Error(m.error || 'the diagram engine did not start')); }
                };
                w.onerror = function (e) { clearTimeout(timer); w.terminate(); reject(new Error((e && e.message) || 'the diagram engine could not start')); };
                w.postMessage({ type: 'init', engine: module, scripts: { go: new URL(base + 'wasm_exec.js' + stamp, location.href).href } });
            });
        });
        workerReady.catch(function () { workerReady = null; worker = null; });
        return workerReady;
    }

    /* One request at a time: the engine is synchronous inside the worker. */
    function pump() {
        if (busy || !queue.length) { return; }
        var job = queue.shift();
        busy = job;
        startWorker().then(function (w) {
            if (busy !== job) { return; }
            job.timer = setTimeout(function () {
                stopWorker();
                finish(job, null, new Error('The diagram took longer than ' + (LIMIT_MS / 1000) + ' seconds to draw, so drawing it was stopped.'));
            }, LIMIT_MS);
            w.postMessage({ type: 'call', id: job.id, name: job.name, arg: job.arg });
        }, function (err) { finish(job, null, err); });
    }
    function onResult(e) {
        var m = e.data || {};
        if (m.type !== 'result' || !busy || m.id !== busy.id) { return; }
        var out;
        try { out = JSON.parse(m.out); } catch (err) { finish(busy, null, err); return; }
        if (out && out.error) { finish(busy, null, new Error(out.error)); } else { finish(busy, out); }
    }
    function finish(job, out, err) {
        clearTimeout(job.timer);
        if (busy === job) { busy = null; }
        if (err) { job.reject(err); } else { job.resolve(out); }
        pump();
    }
    function stopWorker() {
        if (worker) { worker.terminate(); }
        worker = null; workerReady = null;
    }
    /* A request with a key replaces one with the same key that has not
       been sent yet, which resolves with null, so typing in a diagram
       draws only its latest text. */
    function request(name, arg, key) {
        return new Promise(function (resolve, reject) {
            if (key != null) {
                for (var i = 0; i < queue.length; i++) {
                    if (queue[i].key === key) { queue[i].resolve(null); queue.splice(i, 1); break; }
                }
            }
            queue.push({ id: ++seq, name: name, arg: arg, key: key, resolve: resolve, reject: reject });
            pump();
        });
    }

    function compile(req, key) {
        var arg = JSON.stringify(req);
        if (cache.has(arg)) { var hit = cache.get(arg); cache.delete(arg); cache.set(arg, hit); return Promise.resolve(hit); }
        return request('compile', arg, key).then(function (out) {
            if (out && !(out.problems && out.problems.length)) {
                cache.set(arg, out);
                if (cache.size > CACHE_MAX) { cache.delete(cache.keys().next().value); }
            }
            return out;
        });
    }

    function dark() {
        var mode = document.documentElement.getAttribute('data-theme');
        if (mode === 'dark') { return true; }
        if (mode === 'light') { return false; }
        return !!(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
    }
    /* The colour behind a diagram, as #rrggbb, from the nearest ancestor
       that paints one, which D2 then uses for its canvas so the diagram
       has no box of its own; a slide's page is transparent over the
       slide's own colour. Null where nothing paints a solid colour. */
    function backgroundOf(place) {
        for (var el = place; el && el.nodeType === 1; el = el.parentElement) {
            var m = /^rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)$/.exec(getComputedStyle(el).backgroundColor);
            if (m && (m[4] === undefined || +m[4] === 1)) {
                return '#' + [m[1], m[2], m[3]].map(function (n) { return ('0' + (+n).toString(16)).slice(-2); }).join('');
            }
            if (m && +m[4] > 0) { return null; }
        }
        return null;
    }
    /* The site's colours where the diagram stands, as D2's palette by the
       role D2 gives each colour (docs/diagrams-design.md, decision 12): the
       text as N1 down to the background as N7, the accent as B1 and B2, and
       tints of the accent toward the background as B3 to B6. They are read
       through the page's own tokens in place, so on a slide a deck's
       colours win, and mixed here, so every value is a plain #rrggbb. Null
       where something cannot be read. */
    function rgbOf(css) {
        var m = /^rgba?\((\d+(?:\.\d+)?),\s*(\d+(?:\.\d+)?),\s*(\d+(?:\.\d+)?)/.exec(css);
        if (m) { return [+m[1], +m[2], +m[3]]; }
        m = /^color\(srgb ([\d.]+) ([\d.]+) ([\d.]+)/.exec(css);
        return m ? [m[1] * 255, m[2] * 255, m[3] * 255] : null;
    }
    function hex(c) { return '#' + c.map(function (v) { return ('0' + Math.round(Math.max(0, Math.min(255, v))).toString(16)).slice(-2); }).join(''); }
    function mix(a, b, t) { return [0, 1, 2].map(function (i) { return a[i] * t + b[i] * (1 - t); }); }
    function paletteOf(place, bgHex) {
        if (!bgHex) { return null; }
        var probe = document.createElement('span');
        probe.setAttribute('aria-hidden', 'true');
        probe.style.cssText = 'position:absolute;visibility:hidden;pointer-events:none';
        place.appendChild(probe);
        function read(css) { probe.style.color = css; return rgbOf(getComputedStyle(probe).color); }
        var text = read('var(--dinah-ink, var(--text))'), muted = read('var(--dinah-muted-ink, var(--text-muted))');
        var accent = read('var(--dinah-link-on-surface, var(--accent))'), rule = read('var(--dinah-content-border, var(--border))');
        probe.remove();
        var bg = rgbOf('rgb(' + [1, 3, 5].map(function (i) { return parseInt(bgHex.substr(i, 2), 16); }).join(',') + ')');
        if (!text || !muted || !accent || !rule || !bg) { return null; }
        var tint = function (t) { return hex(mix(accent, bg, t)); };
        return {
            n1: hex(text), n2: hex(muted), n3: hex(mix(text, bg, 0.55)), n4: hex(rule), n5: hex(mix(text, bg, 0.18)), n6: hex(mix(text, bg, 0.08)), n7: bgHex,
            b1: hex(accent), b2: hex(accent), b3: tint(0.30), b4: tint(0.22), b5: tint(0.14), b6: tint(0.07),
            aa2: hex(accent), aa4: tint(0.14), aa5: tint(0.07), ab4: tint(0.14), ab5: tint(0.07)
        };
    }
    /* A mistake in the text is the site's notice (yavcUtil.notice) above
       the diagram's source, which stays in view so the place can be found.
       It is a 'problem', announced politely, since a preview redraws as the
       person types. */
    function showProblems(place, problems) {
        var first = problems[0] || { message: 'unknown' };
        var where = first.line > 0 ? 'Line ' + first.line + ': ' : '';
        var more = problems.length > 1 ? ' (and ' + (problems.length - 1) + ' more)' : '';
        var ref = place.firstElementChild;
        if (!ref) { ref = document.createElement('span'); place.appendChild(ref); }
        window.yavcUtil.notice(ref, 'The diagram could not be drawn. ' + where + first.message + more, 'problem');
    }

    var drawCount = 0;
    function call(name, req, key) {
        return request(name, JSON.stringify(req), key);
    }

    function draw(place, source, opts) {
        opts = opts || {};
        if (!place.__yavcD2) { place.__yavcD2Key = 'd2-' + (++drawCount); }
        place.__yavcD2 = { source: source, opts: opts };
        place.setAttribute('data-d2', '');
        place.setAttribute('aria-busy', 'true');
        var req = { text: source, themeID: dark() ? THEME.dark : THEME.light, pad: 16 };
        var bg = backgroundOf(place);
        if (bg) { req.background = bg; }
        var palette = paletteOf(place, bg);
        if (palette) { req.theme = palette; }
        /* PUDL's loading state (README, empty and loading) while a diagram
           takes a moment, which the first one on a page always does while
           the engine starts. It waits 400 ms, so a drawing that is ready at
           once never flashes it, and it has words, so it is announced and
           never rests on the spinner alone. */
        var loading = null;
        var cue = setTimeout(function () {
            if (!place.isConnected || place.getAttribute('aria-busy') !== 'true' || place.querySelector(':scope > .loading')) { return; }
            loading = document.createElement('div');
            loading.className = 'loading';
            loading.setAttribute('role', 'status');
            loading.innerHTML = '<span class="spinner" aria-hidden="true"></span><span>' + (worker ? 'Drawing the diagram…' : 'Starting the diagram engine, which takes a few seconds the first time…') + '</span>';
            place.insertBefore(loading, place.firstChild);
        }, 400);
        function settled() { clearTimeout(cue); if (loading) { loading.remove(); loading = null; } }
        return compile(req, place.__yavcD2Key).then(function (out) {
            settled();
            if (!out || !place.isConnected) { return out; }
            place.removeAttribute('aria-busy');
            if (out.problems && out.problems.length) { failed(out.problems); return out; }
            var img = document.createElement('img');
            img.className = 'sc-diagram-image';
            img.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(out.svg);
            img.width = out.width;
            img.height = out.height;
            img.alt = opts.alt || 'A diagram drawn from D2 source';
            place.replaceChildren(img);
            place.classList.remove('sc-diagram-stale');
            place.classList.add('ed-diagram');
            /* Icons left out are a fault the author can fix, so where the
               document is being edited they are a warning beside the
               diagram; on a slide, which an audience sees, they are a quiet
               line under it. */
            if (out.icons && out.icons.length) {
                var words = out.icons.length === 1 ? 'One icon from another site was left out of this diagram.' : out.icons.length + ' icons from other sites were left out of this diagram.';
                if (place.closest('.deck-slide')) {
                    var note = document.createElement('p');
                    note.className = 'sc-diagram-note';
                    note.textContent = words;
                    place.appendChild(note);
                } else {
                    window.yavcUtil.notice(img, words + ' Diagrams draws only icons written into the diagram itself.', 'warn');
                }
            }
            return out;
        }, function (err) {
            settled();
            var out = { problems: [{ message: String((err && err.message) || err), line: 0, column: 0 }] };
            if (!place.isConnected) { return out; }
            place.removeAttribute('aria-busy');
            failed(out.problems);
            return out;
        });
        /* A mistake keeps the last good drawing, dimmed, where the caller
           asked, and is reported unless the caller reports it itself. */
        function failed(problems) {
            var old = opts.keep && place.querySelector(':scope > img.sc-diagram-image');
            if (old) { place.classList.add('sc-diagram-stale'); } else if (opts.report === false) { place.replaceChildren(); }
            if (opts.report !== false) { showProblems(place, problems); }
        }
    }

    var redrawTimer = 0;
    function redrawAll() {
        clearTimeout(redrawTimer);
        redrawTimer = setTimeout(function () {
            document.querySelectorAll('[data-d2]').forEach(function (place) {
                if (place.__yavcD2) { draw(place, place.__yavcD2.source, place.__yavcD2.opts); }
            });
        }, 50);
    }
    /* The site's mode, colour theme and skin choose the colours, so a
       change of any of them draws every diagram on the page again. */
    new MutationObserver(redrawAll).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'data-palette', 'data-skin'] });
    if (window.matchMedia) { window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', redrawAll); }

    window.yavcD2Engine = Object.freeze({ compile: compile, draw: draw, call: call, version: function () { return request('version', '').then(function (o) { return o && o.d2; }); } });
})();
