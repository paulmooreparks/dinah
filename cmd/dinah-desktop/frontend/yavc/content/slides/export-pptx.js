/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Slides as a PowerPoint file (docs/slides-design.md, decision 14), built
   with PptxGenJS (vendor/pptxgenjs-4.0.1, MIT) from the slides as the
   Markdown engine draws them: each slide's drawn HTML is read back as
   headings, paragraphs, lists, code, quotes, tables, pictures, videos and
   columns, and laid out in the layout's regions, so the file is editable
   in PowerPoint rather than a picture of each slide. Loaded by slides.js
   when the command or the action asks for it.

     yavcSlidesPptx.build(ctx) -> Promise<Blob>

   ctx: { deck, settingsOf, titleOf, drawInto, sizeOf, loadScript, here, stamp } */
(function () {
    'use strict';
    if (window.yavcSlidesPptx) { return; }

    var THEMES = {
        plain: { bg: 'FFFFFF', ink: '1A1A1A', muted: '5C6470', accent: '1F5FBF', code: 'F1F3F6' },
        paper: { bg: 'F6F1E8', ink: '22201C', muted: '6B665C', accent: '8A3A1E', code: 'EBE5D8' },
        midnight: { bg: '0F1724', ink: 'E8EDF5', muted: '9FB0C8', accent: '6CC3FF', code: '1A2536' }
    };
    var FONT = 'Calibri', MONO = 'Consolas';

    function colourOf(s) {
        s = String(s || '').trim();
        var rgb = /^rgba?\((\d+)[, ]+(\d+)[, ]+(\d+)/.exec(s);
        if (rgb) { return [rgb[1], rgb[2], rgb[3]].map(function (x) { return ('0' + (+x).toString(16)).slice(-2); }).join('').toUpperCase(); }
        if (/^[a-z]{3,24}$/i.test(s)) { var probe = document.createElement('span'); probe.style.color = s; document.body.appendChild(probe); var got = getComputedStyle(probe).color; probe.remove(); return got && got !== 'rgb(0, 0, 0)' || s.toLowerCase() === 'black' ? colourOf(got) : null; }
        var m = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(s);
        if (!m) { return null; }
        var h = m[1];
        if (h.length === 3) { h = h[0] + h[0] + h[1] + h[1] + h[2] + h[2]; }
        return h.toUpperCase();
    }

    /* === Reading a drawn slide ========================================= */

    /* Inline content as runs: { text, bold, italic, code, href }. */
    function runsOf(node, style, out) {
        style = style || {};
        out = out || [];
        node.childNodes.forEach(function (c) {
            if (c.nodeType === 3) { if (c.nodeValue) { out.push(Object.assign({ text: c.nodeValue }, style)); } return; }
            if (c.nodeType !== 1) { return; }
            var tag = c.tagName.toLowerCase(), next = Object.assign({}, style);
            if (tag === 'br') { out.push(Object.assign({ text: '', breakLine: true }, style)); return; }
            if (tag === 'strong' || tag === 'b') { next.bold = true; }
            if (tag === 'em' || tag === 'i') { next.italic = true; }
            if (tag === 'code' || tag === 'kbd') { next.code = true; }
            if (tag === 'a' && c.getAttribute('href')) { next.href = c.getAttribute('href'); }
            if (cls_has(c, 'sc-colour')) { var col = colourOf(c.style.color); if (col) { next.color = col; } }
            if (tag === 'img') { out.push({ text: c.getAttribute('alt') || '', italic: true }); return; }
            runsOf(c, next, out);
        });
        return out;
    }
    function cls_has(el, name) { return el.classList && el.classList.contains(name); }
    function textOf(node) { return runsOf(node).map(function (r) { return r.text; }).join(''); }

    /* The slide's content as items, the wrappers the engine draws seen
       through: heading, para, list, code, quote, table, image, video,
       columns. */
    function itemsOf(root) {
        var items = [];
        function walk(node, listLevel) {
            Array.prototype.forEach.call(node.children, function (el) {
                var tag = el.tagName.toLowerCase(), cls = el.classList;
                if (cls.contains('sc-notes') || cls.contains('deck-notes-view')) { return; }
                if (cls.contains('sc-columns')) {
                    var cols = Array.prototype.filter.call(el.children, function (c) { return c.classList.contains('sc-column'); }).map(function (c) { return itemsOf(c); });
                    items.push({ kind: 'columns', columns: cols });
                    return;
                }
                if (cls.contains('sc-figure') || tag === 'figure') {
                    var img = el.querySelector('img');
                    if (img) { items.push({ kind: 'image', src: img.getAttribute('src'), alt: img.getAttribute('alt') || '', caption: el.querySelector('figcaption') ? textOf(el.querySelector('figcaption')) : '' }); }
                    return;
                }
                if (cls.contains('sc-video')) {
                    var f = el.querySelector('iframe');
                    items.push({ kind: 'video', url: f ? f.getAttribute('src') : '', title: f ? f.getAttribute('title') : 'Video' });
                    return;
                }
                if (/^h[1-6]$/.test(tag)) { items.push({ kind: 'heading', level: +tag[1], runs: runsOf(el) }); return; }
                if (tag === 'p') { var r = runsOf(el); if (r.some(function (x) { return x.text.trim(); })) { items.push({ kind: 'para', runs: r }); } return; }
                if (tag === 'ul' || tag === 'ol') {
                    Array.prototype.forEach.call(el.children, function (li) {
                        if (li.tagName.toLowerCase() !== 'li') { return; }
                        var own = document.createElement('span'), subs = [];
                        Array.prototype.forEach.call(li.childNodes, function (c) {
                            if (c.nodeType === 1 && (c.tagName === 'UL' || c.tagName === 'OL')) { subs.push(c); } else { own.appendChild(c.cloneNode(true)); }
                        });
                        var checked = own.querySelector('input[type="checkbox"]');
                        var runs = runsOf(own);
                        if (checked) { runs.unshift({ text: checked.checked ? '☑ ' : '☐ ' }); }
                        items.push({ kind: 'list', ordered: tag === 'ol', level: listLevel || 0, runs: runs });
                        subs.forEach(function (s) { var tmp = document.createElement('div'); tmp.appendChild(s.cloneNode(true)); walk(tmp, (listLevel || 0) + 1); });
                    });
                    return;
                }
                if (tag === 'pre') { items.push({ kind: 'code', text: el.textContent.replace(/\n$/, '') }); return; }
                if (tag === 'blockquote') { items.push({ kind: 'quote', runs: runsOf(el).filter(function (x) { return x.text; }) }); return; }
                if (tag === 'table') {
                    var rows = Array.prototype.map.call(el.querySelectorAll('tr'), function (tr) {
                        return Array.prototype.map.call(tr.children, function (td) { return { text: textOf(td), bold: td.tagName === 'TH' }; });
                    });
                    items.push({ kind: 'table', rows: rows });
                    return;
                }
                if (tag === 'img') { items.push({ kind: 'image', src: el.getAttribute('src'), alt: el.getAttribute('alt') || '' }); return; }
                if (tag === 'hr' || tag === 'script' || tag === 'style') { return; }
                /* A wrapper, a notice, an applet's mount: what is inside. */
                if (cls.contains('notice') || cls.contains('sc-notice')) {
                    var t = el.querySelector('.notice-title'), body = el.querySelector('.notice-body') || el;
                    if (t) { items.push({ kind: 'para', runs: [{ text: textOf(t), bold: true }] }); }
                    walk(body, listLevel);
                    return;
                }
                if (el.hasAttribute('data-applet')) { items.push({ kind: 'para', runs: [{ text: '[' + (el.getAttribute('aria-label') || el.getAttribute('data-applet') || 'applet') + ']', italic: true }] }); return; }
                walk(el, listLevel);
            });
        }
        walk(root, 0);
        return items;
    }

    /* === Writing the file ============================================= */

    /* Options that start a paragraph (a bullet, an indent, space before)
       belong to its first run alone; on a later run they would start
       another paragraph. */
    var PARA = ['bullet', 'indentLevel', 'paraSpaceBefore', 'paraSpaceAfter', 'align'];
    function toRuns(runs, theme, base) {
        var out = [];
        runs.forEach(function (r, k) {
            var o = Object.assign({}, base || {});
            if (k > 0) { PARA.forEach(function (key) { delete o[key]; }); }
            if (r.bold) { o.bold = true; }
            if (r.italic) { o.italic = true; }
            if (r.color) { o.color = r.color; }
            if (r.code) { o.fontFace = MONO; if (!r.color) { o.color = theme.accent; } }
            if (r.href) { o.hyperlink = { url: r.href }; o.color = theme.accent; o.underline = { style: 'sng' }; }
            if (r.breakLine) { o.breakLine = true; }
            out.push({ text: r.text, options: o });
        });
        return out;
    }

    async function imageData(src, base) {
        try {
            var url = new URL(src, base || location.href);
            if (url.protocol === 'data:') { return { data: src }; }
            var r = await fetch(url.href, { credentials: 'same-origin' });
            if (!r.ok) { return null; }
            var blob = await r.blob();
            var data = await new Promise(function (resolve) { var fr = new FileReader(); fr.onload = function () { resolve(fr.result); }; fr.readAsDataURL(blob); });
            var size = await new Promise(function (resolve) { var im = new Image(); im.onload = function () { resolve([im.naturalWidth, im.naturalHeight]); }; im.onerror = function () { resolve([4, 3]); }; im.src = data; });
            return { data: data, w: size[0], h: size[1] };
        } catch (err) { return null; }
    }

    function fit(iw, ih, bw, bh) {
        var k = Math.min(bw / iw, bh / ih);
        return { w: iw * k, h: ih * k };
    }

    async function build(ctx) {
        await ctx.loadScript(ctx.here + 'vendor/pptxgenjs-4.0.1/pptxgen.bundle.js' + ctx.stamp);
        var PptxGenJS = window.PptxGenJS;
        var deck = ctx.deck, wide = deck.front.size !== '4:3';
        var pptx = new PptxGenJS();
        pptx.layout = wide ? 'LAYOUT_16x9' : 'LAYOUT_4x3';
        var W = 10, H = wide ? 5.625 : 7.5;
        pptx.title = deck.front.title || 'A deck';
        if (deck.front.author) { pptx.author = deck.front.author; }
        if (deck.front.subject) { pptx.subject = deck.front.subject; }
        var theme = Object.assign({}, THEMES[deck.front.theme] || THEMES.plain);
        ['ink', 'muted', 'accent', 'heading'].forEach(function (k) { var c = colourOf(deck.front[k]); if (c) { theme[k] = c; } });
        if (colourOf(deck.front.background)) { theme.bg = colourOf(deck.front.background); }
        var ink = theme.ink;
        var text = function (name, type, x, y, w, h, more) { return { placeholder: { options: Object.assign({ name: name, type: type, x: x, y: y, w: w, h: h, color: ink, fontFace: FONT, margin: 4 }, more) } }; };
        pptx.defineSlideMaster({ title: 'TITLE', background: { color: theme.bg }, objects: [
            text('title', 'title', 0.6, H * 0.3, W - 1.2, 1.3, { fontSize: 40, bold: true, align: 'center', valign: 'middle' }),
            text('subtitle', 'body', 0.8, H * 0.3 + 1.4, W - 1.6, 1.2, { fontSize: 22, color: theme.muted, align: 'center', valign: 'top' })
        ] });
        pptx.defineSlideMaster({ title: 'SECTION', background: { color: theme.bg }, objects: [
            text('title', 'title', 0.6, H * 0.35, W - 1.2, 1.3, { fontSize: 34, bold: true, align: 'center', valign: 'middle' })
        ] });
        pptx.defineSlideMaster({ title: 'CONTENT', background: { color: theme.bg }, objects: [
            text('title', 'title', 0.5, 0.35, W - 1, 0.9, { fontSize: 30, bold: true, valign: 'middle' }),
            { line: { x: 0.5, y: 1.3, w: W - 1, h: 0, line: { color: theme.accent, width: 2.5 } } },
            text('body', 'body', 0.5, 1.5, W - 1, H - 1.8, { fontSize: 20, valign: 'top' })
        ] });
        pptx.defineSlideMaster({ title: 'TWO', background: { color: theme.bg }, objects: [
            text('title', 'title', 0.5, 0.35, W - 1, 0.9, { fontSize: 30, bold: true, valign: 'middle' }),
            { line: { x: 0.5, y: 1.3, w: W - 1, h: 0, line: { color: theme.accent, width: 2.5 } } },
            text('left', 'body', 0.5, 1.5, (W - 1.4) / 2, H - 1.8, { fontSize: 18, valign: 'top' }),
            text('right', 'body', 0.5 + (W - 1.4) / 2 + 0.4, 1.5, (W - 1.4) / 2, H - 1.8, { fontSize: 18, valign: 'top' })
        ] });
        pptx.defineSlideMaster({ title: 'QUOTE', background: { color: theme.bg }, objects: [
            text('title', 'title', 0.5, 0.35, W - 1, 0.9, { fontSize: 30, bold: true, valign: 'middle' }),
            text('body', 'body', 0.9, 1.5, W - 1.8, H - 1.8, { fontSize: 28, italic: true, align: 'center', valign: 'middle' })
        ] });
        pptx.defineSlideMaster({ title: 'BLANK', background: { color: theme.bg }, objects: [] });

        for (var i = 0; i < deck.slides.length; i++) {
            var s = deck.slides[i], st = ctx.settingsOf(s), layout = st.layout || 'content';
            var box = document.createElement('div');
            box.className = 'deck-body doc-page';
            ctx.drawInto(box, s.text);
            var items = itemsOf(box);
            var notesText = (ctx.notesOf ? ctx.notesOf(s.text) : '');
            var heading = null, rest = [];
            items.forEach(function (it) { if (!heading && it.kind === 'heading') { heading = it; } else { rest.push(it); } });
            var twoCols = layout === 'two-column' || rest.some(function (it) { return it.kind === 'columns'; });
            var master = layout === 'title' ? 'TITLE' : layout === 'section' ? 'SECTION' : layout === 'quote' ? 'QUOTE' : twoCols ? 'TWO' : layout === 'blank' && !heading ? 'BLANK' : 'CONTENT';
            var slide = pptx.addSlide({ masterName: master });
            var bg = colourOf(st.background);
            if (bg) { slide.background = { color: bg }; }
            var slideInk = colourOf(st.color) || ink, slideHeading = colourOf(st.heading) || theme.heading || slideInk;
            if (st.image) {
                var bgImg = await imageData(st.image);
                if (bgImg) { slide.background = { data: bgImg.data }; }
            }
            if (notesText) { slide.addNotes(notesText); }

            if (master === 'TITLE' || master === 'SECTION') {
                var sub = rest.filter(function (it) { return it.kind === 'heading' || it.kind === 'para'; });
                slide.addText(heading ? toRuns(heading.runs, theme) : [{ text: ctx.titleOf(s, i) }], { placeholder: 'title', fit: 'shrink', color: slideHeading });
                if (master === 'TITLE' && sub.length) {
                    var subRuns = [];
                    sub.forEach(function (it, k) { var r = toRuns(it.runs, theme, { bullet: false }); if (k && r.length) { r[0].options.breakLine = true; } subRuns = subRuns.concat(r); });
                    slide.addText(subRuns, { placeholder: 'subtitle', fit: 'shrink' });
                }
                continue;
            }
            if (heading) { slide.addText(toRuns(heading.runs, theme), { placeholder: 'title', fit: 'shrink', color: slideHeading }); }
            var areaY = 1.5, areaH = H - 1.8;
            if (master === 'QUOTE') {
                var q = rest.filter(function (it) { return it.kind === 'quote' || it.kind === 'para'; });
                var qr = [];
                q.forEach(function (it, k) { var r = toRuns(it.runs, theme, { italic: true, bullet: false }); if (k && r.length) { r[0].options.breakLine = true; } qr = qr.concat(r); });
                slide.addText(qr.length ? qr : [{ text: '' }], { placeholder: 'body', fit: 'shrink' });
                continue;
            }
            if (master === 'TWO') {
                var colsItem = rest.find(function (it) { return it.kind === 'columns'; });
                var cols = colsItem ? colsItem.columns : [rest, []];
                var cw = (W - 1.4) / 2;
                await placeItems(slide, pptx, cols[0] || [], { x: 0.5, y: areaY, w: cw, h: areaH, placeholder: 'left' }, theme, slideInk, 18);
                await placeItems(slide, pptx, cols[1] || [], { x: 0.5 + cw + 0.4, y: areaY, w: cw, h: areaH, placeholder: 'right' }, theme, slideInk, 18);
                continue;
            }
            await placeItems(slide, pptx, rest, { x: 0.5, y: areaY, w: W - 1, h: areaH, placeholder: master === 'CONTENT' ? 'body' : null }, theme, slideInk, layout === 'image' ? 16 : 20);
        }
        return pptx.write({ outputType: 'blob' });
    }

    /* Items into a box: text first, pictures after, sharing the height. */
    async function placeItems(slide, pptx, items, box, theme, ink, size) {
        var text = [], pics = [], tables = [];
        items.forEach(function (it) {
            if (it.kind === 'image') { pics.push(it); }
            else if (it.kind === 'table') { tables.push(it); }
            else { text.push(it); }
        });
        var y = box.y, h = box.h;
        var picH = pics.length ? (text.length ? h * 0.55 : h) : 0;
        var textH = h - picH - (tables.length ? Math.min(h * 0.4, 0.35 * (tables[0].rows.length + 1)) : 0);
        if (text.length && textH > 0.3) {
            var runs = [];
            text.forEach(function (it, k) {
                var r;
                if (it.kind === 'heading') { r = toRuns(it.runs, theme, { bold: true, fontSize: size + 4, bullet: false }); }
                else if (it.kind === 'list') { r = toRuns(it.runs, theme, { bullet: it.ordered ? { type: 'number' } : true, indentLevel: it.level }); }
                else if (it.kind === 'code') { r = [{ text: it.text, options: { fontFace: MONO, fontSize: Math.max(10, size - 6), color: ink, bullet: false } }]; }
                else if (it.kind === 'quote') { r = toRuns(it.runs, theme, { italic: true, color: theme.muted, bullet: false }); }
                else if (it.kind === 'video') { r = [{ text: it.title || 'Video', options: { hyperlink: { url: it.url }, color: theme.accent, underline: { style: 'sng' }, bullet: false } }]; }
                else { r = toRuns(it.runs, theme, { bullet: false }); }
                if (!r.length) { return; }
                if (runs.length) { r[0].options = Object.assign({}, r[0].options); runs[runs.length - 1].options = Object.assign({}, runs[runs.length - 1].options, { breakLine: true }); }
                if (it.kind !== 'list' && runs.length) { r[0].options.paraSpaceBefore = 6; }
                runs = runs.concat(r);
            });
            if (runs.length) {
                var where = box.placeholder && textH >= box.h - 0.01 ? { placeholder: box.placeholder } : { x: box.x, y: y, w: box.w, h: textH };
                slide.addText(runs, Object.assign(where, { fontSize: size, color: ink, fontFace: FONT, valign: 'top', fit: 'shrink', paraSpaceAfter: 4 }));
            }
            y += textH;
        }
        if (tables.length) {
            var t = tables[0], th = Math.min(h * 0.4, 0.35 * (t.rows.length + 1));
            slide.addTable(t.rows.map(function (row) { return row.map(function (cell) { return { text: cell.text, options: { bold: !!cell.bold, color: ink, fontFace: FONT, fontSize: Math.max(10, size - 6) } }; }); }),
                { x: box.x, y: y, w: box.w, h: th, border: { type: 'solid', color: theme.muted, pt: 0.5 }, fill: { color: theme.code }, autoPage: false });
            y += th;
        }
        if (pics.length) {
            var each = picH / pics.length;
            for (var p = 0; p < pics.length; p++) {
                var pic = pics[p], img = await imageData(pic.src);
                if (!img) { slide.addText([{ text: pic.alt || pic.src }], { x: box.x, y: y, w: box.w, h: each, fontSize: 14, italic: true, color: theme.muted, fontFace: FONT, align: 'center', valign: 'middle' }); y += each; continue; }
                var f = fit(img.w || 4, img.h || 3, box.w, each - 0.1);
                slide.addImage({ data: img.data, x: box.x + (box.w - f.w) / 2, y: y + 0.05, w: f.w, h: f.h });
                y += each;
            }
        }
    }

    window.yavcSlidesPptx = Object.freeze({ build: build, itemsOf: itemsOf });
})();
