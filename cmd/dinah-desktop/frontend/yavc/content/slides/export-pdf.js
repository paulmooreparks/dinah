/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Slides as a PDF (docs/slides-design.md, decision 14): each slide is
   drawn as the show draws it, with the page's styles and fonts inlined,
   into an SVG foreignObject, rasterised on a canvas at twice the slide's
   size, and put on a page of its own with pdf-lib (vendor/pdf-lib-1.17.1,
   MIT). The file shows every slide as it looks, shortcodes, diagrams and
   applets included; the text is a picture, which Print keeps as text.

     yavcSlidesPdf.build(ctx) -> Promise<Uint8Array>

   ctx: { deck, buildShow, sizeOf, loadScript, here, stamp, pageCSS } */
(function () {
    'use strict';
    if (window.yavcSlidesPdf) { return; }

    var cssReady = null;

    function toDataURL(blob) {
        return new Promise(function (resolve) { var fr = new FileReader(); fr.onload = function () { resolve(fr.result); }; fr.readAsDataURL(blob); });
    }
    async function inlineURL(url) {
        try {
            var r = await fetch(url, { credentials: 'same-origin' });
            if (!r.ok) { return null; }
            return await toDataURL(await r.blob());
        } catch (err) { return null; }
    }

    /* The page's stylesheets as one text, with every font and picture
       they refer to inlined, since nothing outside the SVG may be loaded
       while it is drawn. */
    async function styles(ctx) {
        if (cssReady) { return cssReady; }
        cssReady = (async function () {
            var links = Array.prototype.map.call(document.querySelectorAll('link[rel="stylesheet"]'), function (l) { return l.href; });
            var texts = [];
            for (var i = 0; i < links.length; i++) {
                var href = links[i];
                if (new URL(href).origin !== location.origin) { continue; }
                try {
                    var r = await fetch(href, { credentials: 'same-origin' });
                    if (!r.ok) { continue; }
                    var css = await r.text();
                    var refs = [], re = /url\((['"]?)(?!data:)([^'")]+)\1\)/g, m;
                    while ((m = re.exec(css))) { refs.push(m[2]); }
                    var seen = {};
                    for (var k = 0; k < refs.length; k++) {
                        var ref = refs[k];
                        if (seen[ref]) { continue; }
                        seen[ref] = true;
                        var abs = new URL(ref, href).href;
                        var data = await inlineURL(abs);
                        if (data) { css = css.split('url(' + ref + ')').join('url(' + data + ')').split('url("' + ref + '")').join('url("' + data + '")').split("url('" + ref + "')").join("url('" + data + "')"); }
                    }
                    texts.push(css);
                } catch (err) { }
            }
            if (ctx.pageCSS) { texts.push(ctx.pageCSS); }
            return texts.join('\n');
        })();
        return cssReady;
    }

    async function inlinePictures(root) {
        var imgs = root.querySelectorAll('img');
        for (var i = 0; i < imgs.length; i++) {
            var src = imgs[i].getAttribute('src') || '';
            if (/^data:/.test(src)) { continue; }
            var data = await inlineURL(new URL(src, location.href).href);
            if (data) { imgs[i].setAttribute('src', data); } else { imgs[i].remove(); }
        }
        root.querySelectorAll('iframe').forEach(function (f) {
            var ph = document.createElement('div');
            ph.setAttribute('style', 'position:absolute;inset:0;display:flex;align-items:center;justify-content:center;background:#111;color:#fff;font:600 28px sans-serif');
            ph.textContent = f.getAttribute('title') || 'Video';
            f.replaceWith(ph);
        });
        root.querySelectorAll('script, link, template, [data-applet] canvas').forEach(function (e) { if (e.tagName !== 'CANVAS') { e.remove(); } });
        var canvases = root.querySelectorAll('canvas');
        for (var c = 0; c < canvases.length; c++) {
            var img = document.createElement('img');
            try { img.src = canvases[c].toDataURL('image/png'); img.width = canvases[c].width; img.height = canvases[c].height; canvases[c].replaceWith(img); } catch (err) { }
        }
    }

    function rasterise(svg, w, h, scale) {
        return new Promise(function (resolve, reject) {
            var img = new Image();
            img.onload = function () {
                var canvas = document.createElement('canvas');
                canvas.width = w * scale; canvas.height = h * scale;
                var g = canvas.getContext('2d');
                g.scale(scale, scale);
                g.drawImage(img, 0, 0);
                canvas.toBlob(function (blob) { blob ? blob.arrayBuffer().then(resolve) : reject(new Error('the slide could not be drawn')); }, 'image/png');
            };
            img.onerror = function () { reject(new Error('the slide could not be drawn as a picture')); };
            img.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg);
        });
    }

    /* Every slide as a PNG (an ArrayBuffer), at twice its size. */
    async function pictures(ctx, progress) {
        var css = await styles(ctx) + '\n' + (ctx.deckCSS ? ctx.deckCSS() : '');
        var wh = ctx.sizeOf(), w = wh[0], h = wh[1];
        /* The show, drawn off the page so its diagrams and applets run. */
        var stage = document.createElement('div');
        stage.className = 'deck-show deck-export-stage';
        stage.setAttribute('style', 'position:fixed;left:-20000px;top:0;width:' + w + 'px;height:' + h + 'px;overflow:hidden');
        document.body.appendChild(stage);
        try {
            ctx.buildShow(stage);
            await new Promise(function (r) { setTimeout(r, 900); });
            var out = [];
            var html = document.documentElement;
            var canvases = stage.querySelectorAll(':scope > .deck-canvas');
            for (var i = 0; i < canvases.length; i++) {
                if (progress) { progress(i + 1, canvases.length); }
                var copy = canvases[i].cloneNode(true);
                await inlinePictures(copy);
                copy.setAttribute('style', 'position:absolute;left:0;top:0;width:' + w + 'px;height:' + h + 'px;transform:none;box-shadow:none;display:block;opacity:1');
                var wrap = document.createElement('div');
                wrap.setAttribute('xmlns', 'http://www.w3.org/1999/xhtml');
                wrap.className = 'deck deck-showing';
                wrap.setAttribute('data-theme-name', stage.getAttribute('data-theme-name') || 'plain');
                wrap.setAttribute('style', 'position:relative;width:' + w + 'px;height:' + h + 'px;overflow:hidden');
                var show = document.createElement('div');
                show.className = 'deck-show';
                show.setAttribute('data-theme-name', stage.getAttribute('data-theme-name') || 'plain');
                show.setAttribute('style', 'position:absolute;inset:0;background:transparent');
                show.appendChild(copy);
                wrap.appendChild(show);
                var style = document.createElement('style');
                style.textContent = css + '\n.deck-show > .deck-canvas { display: block !important; opacity: 1 !important; visibility: visible !important; }';
                wrap.insertBefore(style, wrap.firstChild);
                var xhtml = new XMLSerializer().serializeToString(wrap);
                var svg = '<svg xmlns="http://www.w3.org/2000/svg" width="' + w + '" height="' + h + '"' +
                    (html.getAttribute('data-theme') ? ' data-theme="' + html.getAttribute('data-theme') + '"' : '') +
                    (html.getAttribute('data-skin') ? ' data-skin="' + html.getAttribute('data-skin') + '"' : '') +
                    (html.getAttribute('data-palette') ? ' data-palette="' + html.getAttribute('data-palette') + '"' : '') +
                    '><foreignObject width="100%" height="100%">' + xhtml + '</foreignObject></svg>';
                out.push(await rasterise(svg, w, h, 2));
            }
            return out;
        } finally {
            stage.remove();
        }
    }

    async function build(ctx, progress) {
        await ctx.loadScript(ctx.here + 'vendor/pdf-lib-1.17.1/pdf-lib.min.js' + ctx.stamp);
        var wh = ctx.sizeOf(), w = wh[0] * 0.75, h = wh[1] * 0.75;
        var pngs = await pictures(ctx, progress);
        var pdf = await window.PDFLib.PDFDocument.create();
        pdf.setTitle(ctx.deck.front.title || 'A deck');
        var fm = ctx.deck.front;
        if (fm.author) { pdf.setAuthor(fm.author); }
        if (fm.subject) { pdf.setSubject(fm.subject); }
        if (fm.keywords) { pdf.setKeywords(String(fm.keywords).split(',').map(function (k) { return k.trim(); }).filter(Boolean)); }
        if (fm.lang) { pdf.setLanguage(fm.lang); }
        if (/^\d{4}-\d{2}-\d{2}$/.test(fm.date || '')) { pdf.setCreationDate(new Date(fm.date + 'T00:00:00')); }
        for (var i = 0; i < pngs.length; i++) {
            var image = await pdf.embedPng(pngs[i]);
            var page = pdf.addPage([w, h]);
            page.drawImage(image, { x: 0, y: 0, width: w, height: h });
        }
        return pdf.save();
    }

    window.yavcSlidesPdf = Object.freeze({ build: build, pictures: pictures });
})();
