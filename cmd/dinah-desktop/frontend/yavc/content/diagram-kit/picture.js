/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* A picture of a diagram from its text, for a host that sends a document
   somewhere a diagram cannot be drawn (docs/shortcode-destinations.md,
   DESIGN.md decision 127). It needs no applet window: a host's page loads
   this file beside yavc-util.js and calls it.

     yavcDiagramPicture.draw(request) -> Promise of
         { format: 'svg', svg, width, height, problems }
         { format: 'png', blob, width, height, problems }

   request: {
     kind      'canvas', 'd2' or 'mermaid'
     text      the diagram file's text
     format    'svg' (the default) or 'png'
     scale     for a PNG, 1 to 3 times the drawing's size (2 by default)
     pictures  for a canvas, function (path) resolving to { url } on this
               page's origin, to { bytes }, or to null. path is as the
               canvas has it, so the caller resolves it against the folder
               the canvas file is in. Without it a picture is drawn as a
               box with its name.
     signal    an AbortSignal that ends the work
   }

   The drawing is the one a document shows on paper: light, whatever the
   page's theme. width and height are in CSS pixels. problems are warnings
   about a drawing that was still made, such as a picture that could not
   be read. A diagram that cannot be drawn rejects with its first mistake.
   Fonts are the page's own, as they are in a window. */
(function () {
    'use strict';
    if (window.yavcDiagramPicture) { return; }
    var here = new URL('.', document.currentScript.src), MAX_TEXT = 2 * 1024 * 1024, MAX_SIDE = 32768, MAX_SVG = 16 * 1024 * 1024;
    /* The palette canvas-embed.js draws a canvas with in a document. */
    var PAPER = Object.freeze({ text: '#202124', muted: '#5f6368', line: '#5f6368', surface: '#ffffff', bg: '#ffffff', accent: '#1f5fbf', presets: ['#c62828', '#c25a00', '#9a7b00', '#2e7d32', '#00838f', '#7b3fc4'] });
    var scripts = {};
    function load(path, ready) {
        if (ready()) { return Promise.resolve(ready()); }
        if (!scripts[path]) {
            scripts[path] = window.yavcUtil.loadScript(window.yavcUtil.versioned(new URL(path, here).href)).then(ready).catch(function (error) { delete scripts[path]; throw error; });
        }
        return scripts[path];
    }
    function stopped(signal) { if (signal && signal.aborted) { throw new DOMException('The picture was cancelled.', 'AbortError'); } }
    function sized(out) {
        if (!Number.isFinite(out.width) || !Number.isFinite(out.height) || out.width < 1 || out.height < 1 || out.width > MAX_SIDE || out.height > MAX_SIDE || out.svg.length > MAX_SVG) { throw new Error('The diagram is too large to make a picture of.'); }
        return out;
    }

    async function canvas(request) {
        var J = await load('jcanvas.js', function () { return window.yavcCanvas; });
        var parsed = J.read(request.text), mistake = parsed.problems.find(function (p) { return !p.warning; });
        if (!parsed.doc || mistake) { throw new Error(mistake ? mistake.message : 'The canvas could not be read.'); }
        var read = { pictures: {}, problems: [] };
        if (parsed.doc.nodes.some(J.isPicture)) {
            var reader = await load('canvas-pictures.js', function () { return window.yavcCanvasPictures; });
            read = await reader.collect(parsed.doc, typeof request.pictures === 'function' ? request.pictures : null, { signal: request.signal, canvas: J });
        }
        stopped(request.signal);
        var out = J.render(parsed.doc, { palette: PAPER, pad: 24, title: 'Canvas diagram', pictures: read.pictures });
        return { svg: out.svg, width: out.width, height: out.height, problems: read.problems.slice() };
    }

    async function d2(request) {
        var engine = await load('d2.js', function () { return window.yavcD2Engine; });
        /* Theme 0 is D2's default light theme, which d2.js uses for a light page. */
        var out = await engine.compile({ text: request.text, themeID: 0, pad: 16, background: '#ffffff' });
        stopped(request.signal);
        if (!out || out.problems && out.problems.length) {
            var first = out && out.problems[0];
            throw new Error(first ? (first.line > 0 ? 'Line ' + first.line + ': ' : '') + first.message : 'The diagram engine gave no drawing.');
        }
        var problems = out.icons && out.icons.length ? [out.icons.length === 1 ? 'One icon from another site was left out of this diagram.' : out.icons.length + ' icons from other sites were left out of this diagram.'] : [];
        return { svg: out.svg, width: out.width, height: out.height, problems: problems };
    }

    /* Mermaid measures its text on the page, so it draws into a place that
       is on the page and out of sight, and the drawing is read back from
       there after the shared renderer has sanitised it. */
    async function mermaid(request) {
        var K = await load('../md-editor/kit.js', function () { return window.yavcMdEditor; });
        var stage = document.createElement('div');
        stage.setAttribute('aria-hidden', 'true');
        stage.style.cssText = 'position:fixed;left:-100000px;top:0;width:1200px;visibility:hidden;pointer-events:none;';
        document.body.appendChild(stage);
        try {
            var drawn = await K.drawMermaid(stage, request.text, { paper: true, report: false });
            stopped(request.signal);
            var svg = stage.querySelector('svg');
            if (drawn.problem || !svg) { throw new Error(drawn.problem || 'Mermaid gave no drawing.'); }
            var box = svg.viewBox && svg.viewBox.baseVal && svg.viewBox.baseVal.width ? svg.viewBox.baseVal : svg.getBoundingClientRect();
            var width = Math.ceil(box.width), height = Math.ceil(box.height);
            svg.setAttribute('width', width); svg.setAttribute('height', height);
            svg.style.maxWidth = '';
            if (!svg.getAttribute('xmlns')) { svg.setAttribute('xmlns', 'http://www.w3.org/2000/svg'); }
            return { svg: new XMLSerializer().serializeToString(svg), width: width, height: height, problems: [] };
        } finally { stage.remove(); }
    }

    /* The drawing as a PNG, through an image, which runs no script and
       loads nothing from outside. */
    function png(out, scale) {
        return new Promise(function (resolve, reject) {
            var image = new Image();
            image.onload = function () {
                try {
                    var surface = document.createElement('canvas');
                    surface.width = Math.max(1, Math.round(out.width * scale)); surface.height = Math.max(1, Math.round(out.height * scale));
                    if (surface.width > MAX_SIDE || surface.height > MAX_SIDE) { throw new Error('The diagram is too large for a picture at this scale.'); }
                    surface.getContext('2d').drawImage(image, 0, 0, surface.width, surface.height);
                    surface.toBlob(function (blob) { if (blob) { resolve(blob); } else { reject(new Error('The browser could not make the picture.')); } }, 'image/png');
                } catch (error) { reject(new Error('The browser could not make a PNG of this drawing. Ask for SVG.')); }
            };
            image.onerror = function () { reject(new Error('The drawing could not be read as a picture.')); };
            var svg = out.svg;
            if (!/<svg[^>]*\swidth=/.test(svg)) { svg = svg.replace(/<svg\b/, '<svg width="' + out.width + '" height="' + out.height + '"'); }
            image.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg);
        });
    }

    async function draw(request) {
        if (!request || typeof request.text !== 'string') { throw new Error('A diagram picture needs the diagram\'s text.'); }
        if (request.text.length > MAX_TEXT) { throw new Error('A diagram must be no larger than 2 MiB.'); }
        var kinds = { canvas: canvas, d2: d2, mermaid: mermaid }, make = Object.prototype.hasOwnProperty.call(kinds, request.kind) ? kinds[request.kind] : null;
        if (!make) { throw new Error('A diagram is a canvas, d2 or mermaid.'); }
        var format = request.format == null ? 'svg' : request.format;
        if (format !== 'svg' && format !== 'png') { throw new Error('A diagram picture is svg or png.'); }
        stopped(request.signal);
        var out = sized(await make(request));
        if (format === 'svg') { return { format: 'svg', svg: out.svg, width: out.width, height: out.height, problems: out.problems }; }
        var scale = request.scale == null ? 2 : Number(request.scale);
        if (!(scale >= 1 && scale <= 3)) { throw new Error('A picture\'s scale is from 1 to 3.'); }
        var blob = await png(out, scale);
        stopped(request.signal);
        return { format: 'png', blob: blob, width: Math.round(out.width * scale), height: Math.round(out.height * scale), problems: out.problems };
    }

    /* The kind of diagram a file is, by its name, or null. */
    function kindOf(name) { var m = /\.(d2|mmd|mermaid|canvas)$/i.exec(String(name || '')); return !m ? null : /^canvas$/i.test(m[1]) ? 'canvas' : /^d2$/i.test(m[1]) ? 'd2' : 'mermaid'; }

    window.yavcDiagramPicture = Object.freeze({ draw: draw, kindOf: kindOf });
})();
