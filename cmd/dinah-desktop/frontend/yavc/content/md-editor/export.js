/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Shared, browser-local document preparation and DOCX/PDF writers. */
(function () {
    'use strict';
    var LIMIT = 1024 * 1024, IMAGE_LIMIT = 16 * 1024 * 1024, fontReady = null;
    function pdfFont(K) {
        if (!fontReady) { fontReady = K.loadScript(K.here + 'vendor/fontkit-1.1.1/fontkit.js' + K.stamp).then(function () {
            return fetch(K.here + 'vendor/pdfmake-0.2.23/Roboto-Regular.ttf' + K.stamp).then(function (r) { if (!r.ok) { throw new Error('The PDF font could not be loaded.'); } return r.arrayBuffer(); });
        }).then(function (buffer) { return window.fontkit.create(new Uint8Array(buffer)); }).catch(function (err) { fontReady = null; throw err; }); }
        return fontReady;
    }
    function checkGlyphs(blocks, font) {
        blocks.forEach(function (b) {
            (b.runs || []).forEach(function (r) { for (var char of r.text) { if (!/[\n\r\t]/.test(char) && !font.hasGlyphForCodePoint(char.codePointAt(0))) { throw new Error('The bundled PDF font cannot display ' + char + '. Export to Word or replace that character before exporting PDF.'); } } });
            if (b.blocks) { checkGlyphs(b.blocks, font); }
            (b.items || []).forEach(function (item) { checkGlyphs(item, font); });
            (b.rows || []).forEach(function (row) { row.forEach(function (cell) { checkGlyphs([{ runs: cell.runs }], font); }); });
        });
    }
    function settings(front) { return window.yavcDocumentPages.settings(front); }
    function rgb(value) {
        var canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
        var context = canvas.getContext('2d', { colorSpace: 'srgb' });
        context.fillStyle = value || '#202124'; context.fillRect(0, 0, 1, 1);
        var rgba = context.getImageData(0, 0, 1, 1).data;
        if (rgba[3] !== 255) { throw new Error('Export paper and text colors must be opaque. Use a solid color.'); }
        return Array.from(rgba).slice(0, 3).map(function (n) { return n.toString(16).padStart(2, '0'); }).join('');
    }
    function typography(element) {
        var s = getComputedStyle(element), decoration = s.textDecorationLine;
        return { size: Math.max(6, Math.min(72, parseFloat(s.fontSize) * 0.75 || 11)), color: rgb(s.color),
            bold: +s.fontWeight >= 600 || s.fontWeight === 'bold', italic: s.fontStyle === 'italic',
            underline: decoration.includes('underline') || !!element.closest('u'), strike: decoration.includes('line-through') || !!element.closest('del,s'),
            sup: !!element.closest('sup'), sub: !!element.closest('sub'), mark: !!element.closest('mark'),
            font: s.fontFamily.split(',')[0].replace(/["']/g, ''), leading: parseFloat(s.lineHeight) / parseFloat(s.fontSize) || 1.5, after: parseFloat(s.marginBottom) * .75 || 0, keepNext:/^(avoid|avoid-page)$/.test(s.breakAfter), align: /^(left|right|center|justify)$/.test(s.textAlign) ? s.textAlign : 'left' };
    }
    function inline(element) {
        var out = [];
        function walk(node) {
            if (node.nodeType === 3) {
                if (!node.textContent) { return; }
                var parent = node.parentElement, link = parent.closest('a[href]'), whiteSpace = getComputedStyle(parent).whiteSpace, collapse = /^(normal|nowrap)$/.test(whiteSpace), text = node.textContent;
                if (collapse) { text = text.replace(/[\t\n\r\f ]+/g, ' '); if (!out.length || out[out.length - 1].text === '\n' || out[out.length - 1].collapse && / $/.test(out[out.length - 1].text)) { text = text.replace(/^ /, ''); } }
                if (text) { out.push(Object.assign({ text: text, collapse: collapse, field: parent.dataset.docField, link: link ? link.getAttribute('href') : null }, typography(parent))); } return;
            }
            if (node.nodeType !== 1) { return; }
            if (node.matches('sup.doc-footnote-ref[data-footnote-ref]')) { out.push(Object.assign({text:node.textContent,footnote:node.getAttribute('data-footnote-ref')},typography(node))); return; }
            if (node.matches('br')) { if (out.length && out[out.length - 1].collapse) { out[out.length - 1].text = out[out.length - 1].text.replace(/ $/, ''); } out.push({ text: '\n' }); return; }
            if (node.matches('img')) { throw new Error('Place pictures on their own line before exporting.'); }
            if (node.matches('input[type="checkbox"]')) { out.push({ text: node.checked ? '[x] ' : '[ ] ' }); return; }
            if (getComputedStyle(node).display === 'none') { return; }
            node.childNodes.forEach(walk);
        }
        element.childNodes.forEach(walk);
        if (out.length && out[out.length - 1].collapse) { out[out.length - 1].text = out[out.length - 1].text.replace(/ $/, ''); }
        return out;
    }
    async function hostRaster(src,options){
        var url=new URL(src,location.href);
        if(!/^https?:$/.test(url.protocol)||url.origin!==location.origin||url.username||url.password)throw new Error('Export needs a same-origin host picture.');
        var controller=new AbortController(),signal=options.pictures&&options.pictures.signal,timer=setTimeout(function(){controller.abort();},10000);
        function abort(){controller.abort();}if(signal){if(signal.aborted)abort();else signal.addEventListener('abort',abort,{once:true});}
        try{
            var response=await fetch(url.href,{credentials:'same-origin',redirect:'error',signal:controller.signal});
            if(!response.ok||!/^image\/(png|jpeg|gif|webp)(?:;|$)/i.test(response.headers.get('Content-Type')||''))throw new Error('The host picture is not a supported raster image.');
            if(Number(response.headers.get('Content-Length'))>IMAGE_LIMIT)throw new Error('A picture exceeds the 16 MiB export limit.');
            if(!response.body)throw new Error('The host picture has no image content.');
            var reader=response.body.getReader(),chunks=[],size=0;
            try{while(true){var part=await reader.read();if(part.done)break;size+=part.value.length;if(size>IMAGE_LIMIT||options.current&&!options.current())throw new Error('The picture exceeds its limit or the document changed.');chunks.push(part.value);}}
            catch(error){await reader.cancel().catch(function(){});throw error;}
            return URL.createObjectURL(new Blob(chunks,{type:response.headers.get('Content-Type')}));
        }finally{clearTimeout(timer);if(signal)signal.removeEventListener('abort',abort);}
    }
    async function image(src, width, height) {
        if (!/^(data:image\/(png|jpe?g|gif|webp);base64,|blob:)/i.test(src) || src.length > IMAGE_LIMIT * 1.4) { throw new Error('Export pictures must be embedded raster images or local home-folder pictures, at most 16 MiB.'); }
        var url = src, owned = false;
        if (src.startsWith('data:')) {
            var bytes = Uint8Array.from(atob(src.split(',')[1]), function (c) { return c.charCodeAt(0); });
            if (bytes.length > IMAGE_LIMIT) { throw new Error('A picture exceeds the 16 MiB export limit.'); }
            url = URL.createObjectURL(new Blob([bytes], { type: src.slice(5, src.indexOf(';')) })); owned = true;
        }
        try {
            var img = await new Promise(function (resolve, reject) { var i = new Image(); i.onload = function () { resolve(i); }; i.onerror = function () { reject(new Error('A picture could not be decoded.')); }; i.src = url; });
            if (!img.naturalWidth || img.naturalWidth * img.naturalHeight > 24000000) { throw new Error('A decoded export picture exceeds 24 megapixels.'); }
            var canvas = document.createElement('canvas'); canvas.width = img.naturalWidth; canvas.height = img.naturalHeight;
            canvas.getContext('2d').drawImage(img, 0, 0);
            var scale = Math.min(1, width / img.naturalWidth, height / img.naturalHeight);
            return { kind: 'image', data: canvas.toDataURL('image/png'), width: img.naturalWidth * scale, height: img.naturalHeight * scale };
        } finally { if (owned) { URL.revokeObjectURL(url); } }
    }
    async function prepare(options) {
        var source = String(options.text || ''), K = window.yavcMdEditor, engine = options.engine;
        if (new TextEncoder().encode(source).length > LIMIT) { throw new Error('Document exports are limited to 1 MiB of Markdown.'); }
        var front = engine.front(source), props = {};
        if (front.problem) { throw new Error(front.problem); }
        (front.entries || []).forEach(function (entry) { if (entry.kind !== 'complex') { props[entry.key] = entry.value; } });
        var S = window.yavcDocumentSettings, values = S.properties(front), page = settings(values), rendered = engine.exportPreview(source), fragment = window.yavcHTML.fragment(rendered.blocks.map(function (b) { return b.html; }).join(''));
        page.furniture = S.furniture(values, await window.yavcMdEngine()); S.check(page);
        if ((rendered.problems || []).length) { throw new Error('Fix the document warnings before exporting.'); }
        // The HTML cleaner uses an inert template, so omitted assets have not loaded.
        fragment.querySelectorAll('[data-export="omit"],.sc-draft-note').forEach(function (node) { node.remove(); });
        if (fragment.querySelector('[data-host-shortcode]')) { throw new Error('This document contains host-rendered shortcodes. This host must provide their export representation before printing or Office export.'); }
        // Browser printing includes every disclosure body without its controls.
        // Office conversion retains its separate structural representation gate.
        if (options.renderPages) {
            Array.from(fragment.querySelectorAll('details.sc-expand')).reverse().forEach(function (details) {
                var section = document.createElement('section'); section.className = 'sc-expand';
                var summary = details.querySelector(':scope > summary');
                if (summary) { var title = document.createElement('p'); title.className = 'sc-expand-print-title'; title.append.apply(title, Array.from(summary.childNodes)); summary.replaceWith(title); }
                section.append.apply(section, Array.from(details.childNodes)); details.replaceWith(section);
            });
        }
        var printDiagrams = [];
        if (options.renderPages) {
            rendered.blocks.forEach(function (block) { (block.diagrams || []).forEach(function (diagram) {
                if (diagram.language !== 'mermaid') { return; }
                var matches = fragment.querySelectorAll('[data-preview-diagram="' + CSS.escape(diagram.id) + '"]'); // check-ignore(one-shared-implementation): Validate export metadata before awaiting the shared Mermaid renderer; no local rendering implementation.
                if (matches.length !== 1) { throw new Error('A print diagram has an ambiguous source reference.'); }
                var place = matches[0]; place.classList.remove('dinah-diagram-placeholder'); place.classList.add('doc-print-diagram');
                printDiagrams.push({ place: place, source: diagram.source });
            }); });
        }
        if (fragment.querySelector('.sc-problem,.sc-glyph,.dinah-diagram-placeholder,[data-diagram-src],[data-applet],iframe,video,audio,svg,canvas,math,details,code[data-code-src]')) { throw new Error('This document contains an invalid shortcode, diagram, glyph, embedded app, SVG, interactive element or external code listing that this export cannot represent yet.'); }
        var pictures = new Map();
        fragment.querySelectorAll('img').forEach(function (img) { pictures.set(img, { source: img.getAttribute('src'), path: img.dataset.attachment }); img.removeAttribute('src'); img.removeAttribute('srcset'); img.removeAttribute('loading'); });
        var boundary = document.createElement('div'), surface = document.createElement('div');
        boundary.style.cssText = 'position:fixed;left:-100000px;top:0;contain:paint;pointer-events:none;width:' + (page.width - page.margins[0] - page.margins[2]) / 0.75 + 'px;';
        boundary.setAttribute('aria-hidden', 'true'); boundary.inert = true;
        surface.className = 'doc-page'; surface.style.cssText = 'padding:0;min-height:0;min-width:0;';
        surface.append(fragment); boundary.append(surface); document.body.append(boundary);
        K.scopeFootnotes(surface);
        var controller;
        try {
            var D = await K.styles();
            controller = D.attach({ surfaces: [surface], boundaries: [boundary], read: options.read,
                base: window.yavcDocumentPages.paper + ';' + S.typeCSS(values) });
            await controller.update({ identity: source, inline: engine.styles(source).map(function (b) { return b.css; }), stylesheet: options.stylesheet || '' });
            if (controller.state().problems.length) { throw new Error(controller.state().problems.join(' ')); }
            page.background = rgb(getComputedStyle(surface).backgroundColor);
            for (var diagram of printDiagrams) {
                if (options.current && !options.current()) { throw new Error('The document changed during print preparation.'); }
                var drawn = await K.drawMermaid(diagram.place, diagram.source, { paper: true, report: false });
                if (drawn.problem || !diagram.place.querySelector('svg')) { throw new Error('The Mermaid diagram could not be printed: ' + (drawn.problem || 'No drawing was produced.')); }
            }
            var noteContext=rendered.blocks.reduce(function(notes,block){return Object.assign(notes,block.footnotes||{});},Object.create(null)),noteNodes=new Map(),trustedNotes=new WeakSet();
            surface.querySelectorAll('aside.doc-footnote[data-footnote]').forEach(function(node){
                var label=node.getAttribute('data-footnote'),number=noteContext[label];
                if(!Number.isInteger(number)||number<1||number>256||number!==Number(node.getAttribute('data-footnote-number'))||noteNodes.has(label)||!node.querySelector(':scope > .doc-footnote-body'))throw new Error('A footnote definition is ambiguous or unsupported. Check its label in the source.');
                noteNodes.set(label,node);trustedNotes.add(node);
            });
            var images = new Map(), totalImageBytes = 0;
            for (var pair of pictures) {
                if (options.current && !options.current()) { throw new Error('The document changed during export.'); }
                var spec = pair[1], siteImage = spec.source && /^\/(?!\/)/.test(spec.source), local = spec.path || (spec.source && !siteImage && !/^(?:[a-z][a-z0-9+.-]*:|\/\/)/i.test(spec.source) ? spec.source : null), src = local ? await K.pictureFor(local, options.pictures) : spec.source;
                var owned=null,picture;
                try{if(siteImage || local&&/^https?:/i.test(src||'')){owned=await hostRaster(src,options);src=owned;}
                    picture=await image(src||'',(page.width-page.margins[0]-page.margins[2])/.75,(page.height-page.margins[1]-page.margins[3]-12)/.75);
                }finally{if(owned)URL.revokeObjectURL(owned);}
                totalImageBytes += picture.data.length;
                if (totalImageBytes > 32 * 1024 * 1024) { throw new Error('Export pictures exceed the combined 32 MiB limit.'); }
                picture.width *= 0.75; picture.height *= 0.75;
                picture.alt = pair[0].alt || ''; images.set(pair[0], picture);
            }
            var running = {}, layer = document.createElement('div'); layer.className = 'doc-furniture'; layer.style.position = 'static'; surface.append(layer);
            try {
                ['header', 'footer'].forEach(function (kind) {
                    var row = S.row(page, kind, 9999, 9999); row.style.position = 'static'; layer.append(row); S.checkRows(layer);
                    running[kind] = Array.from(row.children).map(function (cell) { return { align: cell.style.textAlign, runs: inline(cell) }; }); row.remove();
                });
            } finally { layer.remove(); }
            if (options.renderPages) {
                // Keep authored sizing and responsive column constraints. Fixed
                // Office dimensions would pin height while CSS narrows width.
                images.forEach(function (picture, node) { node.src = picture.data; });
                await Promise.all(Array.from(surface.querySelectorAll('img')).map(function (node) { return node.decode(); }));
                return await options.renderPages(surface, page);
            }
            // Browser printing retains browser layout. Structural Office writers
            // must refuse CSS they cannot preserve in their output model.
            [surface].concat(Array.from(surface.querySelectorAll('*'))).forEach(function (element) {
                var css = getComputedStyle(element);
                if (css.display === 'none') { return; }
                if (/^(flex|grid|inline-flex|inline-grid)$/.test(css.display) && !(trustedNotes.has(element)&&css.display==='grid') || css.float !== 'none' || css.columnCount !== 'auto' && +css.columnCount > 1 || css.columnWidth !== 'auto' || +css.opacity !== 1) { throw new Error('This document uses a layout that Office export cannot represent yet.'); }
            });
            var blocks = [],notes=[];
            function children(parent, target) { parent.childNodes.forEach(function (node) { block(node, target); }); }
            function block(node, target) {
                if (node.nodeType === 3) { if (node.textContent.trim()) { target.push({ kind: 'paragraph', runs: [{ text: node.textContent }], style: typography(node.parentElement) }); } return; }
                if (node.nodeType !== 1 || getComputedStyle(node).display === 'none') { return; }
                if(trustedNotes.has(node))return;
                if (node.matches('img')) { target.push(images.get(node)); return; }
                if (node.matches('.sc-page-break')) { target.push({ kind: 'break' }); return; }
                if (node.matches('hr')) { target.push({ kind: 'rule' }); return; }
                if (node.matches('p,h1,h2,h3,h4,h5,h6,pre,figcaption')) {
                    var imgs = node.querySelectorAll('img');
                    if (imgs.length && !node.textContent.trim() && imgs.length === 1) { target.push(images.get(imgs[0])); return; }
                    var runs = inline(node); if (!runs.length && !node.matches('pre')) { return; }
                    target.push({ kind: 'paragraph', runs: runs, level: /^H[1-6]$/.test(node.tagName) ? +node.tagName[1] : 0, code: node.matches('pre'), style: typography(node), id: node.id || undefined }); return;
                }
                if (node.matches('table')) {
                    var rows = Array.from(node.rows).map(function (row) { return Array.from(row.cells).map(function (cell) {
                        if (cell.colSpan !== 1 || cell.rowSpan !== 1) { throw new Error('Merged table cells are not supported by document export.'); }
                        return { runs: inline(cell), style: typography(cell) };
                    }); });
                    if (rows.length && rows.some(function (r) { return r.length !== rows[0].length; })) { throw new Error('Export tables must be rectangular.'); }
                    if (rows.length) { target.push({ kind: 'table', rows: rows }); } return;
                }
                if (node.matches('ul,ol')) {
                    var items = Array.from(node.children).filter(function (li) { return li.tagName === 'LI'; }).map(function (li) {
                        var item = [], run = [];
                        function flush() { if (run.length) { item.push({ kind: 'paragraph', runs: run, style: typography(li) }); run = []; } }
                        li.childNodes.forEach(function (child) { if (child.nodeType === 3 || child.nodeType === 1 && !/^(P|UL|OL|BLOCKQUOTE|DIV|PRE)$/.test(child.tagName)) { var holder = document.createElement('span'); if (child.nodeType === 3) { run.push(Object.assign({ text: child.textContent }, typography(li))); } else { holder = child; if (child.matches('input')) { run.push({ text: child.checked ? '[x] ' : '[ ] ' }); } else { run = run.concat(inline(holder)); } } } else { flush(); block(child, item); } }); flush(); return item;
                    });
                    if (items.length) { target.push({ kind: 'list', ordered: node.tagName === 'OL', start: Math.max(1, Number(node.getAttribute('start')) || 1), items: items }); } return;
                }
                if (node.matches('blockquote')) { var quoted = []; children(node, quoted); if (quoted.length) { target.push({ kind: 'quote', blocks: quoted }); } return; }
                var css = getComputedStyle(node);
                if (/^(flex|grid|inline-flex|inline-grid)$/.test(css.display) || css.float !== 'none') { throw new Error('This document uses a layout that Office export cannot represent yet.'); }
                children(node, target);
            }
            children(surface, blocks);
            noteNodes.forEach(function(node,label){var content=[];children(node.querySelector(':scope > .doc-footnote-body'),content);function check(items){items.forEach(function(b){if(b.kind==='table'||b.kind==='break')throw new Error('Footnote export supports paragraphs, lists, quotations and pictures. Move tables or page breaks into the document body.');if((b.runs||[]).some(function(r){return r.footnote;}))throw new Error('A footnote cannot contain another footnote reference in Office export.');if(b.blocks)check(b.blocks);(b.items||[]).forEach(check);});}check(content);notes.push({label:label,number:noteContext[label],blocks:content});});
            return { blocks: blocks, footnotes:notes, page: page, furniture: running, properties: props, warnings: ['PDF uses Roboto instead of the document font.'] };
        } finally { if (controller) { controller.destroy(); } boundary.remove(); }
    }
    function docxDocument(model, D) {
        var numbering = [], listCount = 0, bookmarks = new Map(),notes=new Map((model.footnotes||[]).map(function(note){return [note.label,note];})),referenced=new Set();
        function bookmark(id) { if (!bookmarks.has(id)) { bookmarks.set(id, 'yavc_' + (bookmarks.size + 1)); } return bookmarks.get(id); }
        function runs(items) {
            return items.flatMap(function (r) {
                var options = { bold: r.bold, italics: r.italic, underline: r.underline ? {} : undefined, strike: r.strike, superScript: r.sup, subScript: r.sub, highlight: r.mark ? 'yellow' : undefined, size: (r.size || 11) * 2, color: r.color || '202124', font: r.font || 'Arial' };
                if(r.footnote){var note=notes.get(r.footnote);if(!note)throw new Error('A footnote reference has no exported definition.');var id='yavc_note_'+note.number;if(referenced.has(r.footnote))return [new D.SimpleField('NOTEREF '+id+' \\h \\f',String(note.number))];referenced.add(r.footnote);return [new D.Bookmark({id:id,children:[new D.FootnoteReferenceRun(note.number)]})];}
                if (r.field) { return [new D.TextRun(Object.assign({}, options, { children: [r.field === 'page' ? D.PageNumber.CURRENT : D.PageNumber.TOTAL_PAGES] }))]; }
                var children = String(r.text).split('\n').map(function (text, i) { return new D.TextRun(Object.assign({}, options, { text: text, break: i ? 1 : undefined })); });
                if (r.link && r.link[0] === '#') { return [new D.InternalHyperlink({ anchor: bookmark(r.link.slice(1)), children: children })]; }
                if (r.link) { return [new D.ExternalHyperlink({ link: new URL(r.link, location.href).href, children: children })]; }
                return children;
            });
        }
        function write(blocks, depth, list) {
            return blocks.flatMap(function (b, index) {
                if (b.kind === 'paragraph') {
                    var content = runs(b.runs), style = b.style || {};
                    if (b.id) { content = [new D.Bookmark({ id: bookmark(b.id), children: content })]; }
                    return [new D.Paragraph({ children: content, heading: b.level ? D.HeadingLevel['HEADING_' + b.level] : undefined, alignment: style.align,
                        numbering: list && index === 0 ? { reference: list, level: 0 } : undefined,
                        indent: depth ? { left: depth * 360 } : undefined, spacing: { after: Math.round((style.after || 0) * 20), line: Math.round((style.leading || 1.5) * 240) }, keepNext: !!style.keepNext })];
                }
                if (b.kind === 'list') {
                    var reference = 'list-' + (++listCount);
                    numbering.push({ reference: reference, levels: [{ level: 0, format: b.ordered ? D.LevelFormat.DECIMAL : D.LevelFormat.BULLET, text: b.ordered ? '%1.' : '\u2022', start: b.start, alignment: D.AlignmentType.LEFT, style: { paragraph: { indent: { left: (depth + 1) * 360, hanging: 180 } } } }] });
                    return b.items.flatMap(function (item) { return write(item, depth + 1, reference); });
                }
                if (b.kind === 'quote') { return write(b.blocks, depth + 1); }
                if (b.kind === 'table') { return [new D.Table({ width: { size: 100, type: D.WidthType.PERCENTAGE }, rows: b.rows.map(function (row, i) { return new D.TableRow({ tableHeader: i === 0, children: row.map(function (cell) { return new D.TableCell({ children: [new D.Paragraph({ children: runs(cell.runs), alignment: cell.style.align })] }); }) }); }) })]; }
                if (b.kind === 'image') { return [new D.Paragraph({ children: [new D.ImageRun({ type: 'png', data: Uint8Array.from(atob(b.data.split(',')[1]), function (c) { return c.charCodeAt(0); }), transformation: { width: b.width / 0.75, height: b.height / 0.75 }, altText: { title: b.alt, description: b.alt, name: b.alt } })] })]; }
                if (b.kind === 'break') { return [new D.Paragraph({ children: [new D.PageBreak()] })]; }
                if (b.kind === 'rule') { return [new D.Paragraph({ border: { bottom: { color: '808080', style: D.BorderStyle.SINGLE, size: 6 } } })]; }
                throw new Error('Unsupported export block.');
            });
        }
        var children = write(model.blocks, 0), p = model.properties, m = model.page.margins;
        function running(kind, empty) {
            var cells = empty ? [] : (model.furniture || {})[kind] || [];
            var content = cells.length ? [new D.Table({ width: { size: 100, type: D.WidthType.PERCENTAGE }, borders: Object.fromEntries(['top','bottom','left','right','insideHorizontal','insideVertical'].map(function (key) { return [key, { style: D.BorderStyle.NONE, size: 0 }]; })), rows: [new D.TableRow({ children: cells.map(function (cell) { return new D.TableCell({ children: [new D.Paragraph({ children: runs(cell.runs), alignment: cell.align })] }); }) })] })] : [new D.Paragraph({})];
            return kind === 'header' ? new D.Header({ children: content }) : new D.Footer({ children: content });
        }
        var footnotes=Object.create(null);notes.forEach(function(note){footnotes[note.number]={children:write(note.blocks,0)};});
        return new D.Document({ title: p.title || '', creator: p.author || '', subject: p.subject || '', keywords: p.keywords || '', description: '', background: { color: model.page.background || 'ffffff' }, numbering: { config: numbering }, footnotes:footnotes,features:{updateFields:notes.size>0},
            sections: [{ headers: { default: running('header'), first: running('header', !model.page.furniture.first) }, footers: { default: running('footer'), first: running('footer') }, properties: { titlePage: !model.page.furniture.first, page: { size: { width: Math.round(model.page.width * 20), height: Math.round(model.page.height * 20) }, margin: { header: Math.max(0, m[1] / 2 - 6) * 20, footer: Math.max(0, m[3] / 2 - 6) * 20, left: m[0] * 20, top: m[1] * 20, right: m[2] * 20, bottom: m[3] * 20 } } }, children: children.length ? children : [new D.Paragraph({})] }] });
    }
    function pdfDefinition(model) {
        function runs(items) { return items.map(function (r) { return { text: r.text, bold: r.bold, italics: r.italic, color: '#' + (r.color || '202124'), fontSize: r.size || 11,
            decoration: [r.underline ? 'underline' : null, r.strike ? 'lineThrough' : null].filter(Boolean), background: r.mark ? '#ffff00' : undefined,
            link: r.link && r.link[0] !== '#' ? new URL(r.link, location.href).href : undefined, linkToDestination: r.link && r.link[0] === '#' ? r.link.slice(1) : undefined,
            sup: r.sup, sub: r.sub }; }); }
        function write(blocks) { return blocks.map(function (b) {
            if (b.kind === 'paragraph') { return { text: runs(b.runs), alignment: (b.style || {}).align, lineHeight: (b.style || {}).leading, margin: [0, b.level ? 8 : 0, 0, (b.style || {}).after || 0], id: b.id, headlineLevel:(b.style||{}).keepNext?b.level||1:undefined, preserveLeadingSpaces: !!b.code }; }
            if (b.kind === 'list') { var list = {}; list[b.ordered ? 'ol' : 'ul'] = b.items.map(function (item) { return { stack: write(item) }; }); list.start = b.start; list.margin = [0, 0, 0, 6]; return list; }
            if (b.kind === 'quote') { return { stack: write(b.blocks), margin: [12, 0, 0, 6] }; }
            if (b.kind === 'table') { return { table: { headerRows: 1, widths: b.rows[0].map(function () { return '*'; }), body: b.rows.map(function (row) { return row.map(function (cell) { return { text: runs(cell.runs), alignment: cell.style.align, margin: [3, 3, 3, 3] }; }); }) }, margin: [0, 3, 0, 8] }; }
            if (b.kind === 'image') { return { image: b.data, width: b.width, height: b.height, margin: [0, 0, 0, 6] }; }
            if (b.kind === 'break') { return { text: '', pageBreak: 'after' }; }
            if (b.kind === 'rule') { return { canvas: [{ type: 'line', x1: 0, y1: 0, x2: model.page.width - model.page.margins[0] - model.page.margins[2], y2: 0, lineWidth: 0.5 }], margin: [0, 8, 0, 8] }; }
            throw new Error('Unsupported export block.');
        }); }
        function running(kind) { return function (number, total) {
            if (kind === 'header' && number === 1 && !model.page.furniture.first) { return ''; }
            return { columns: ((model.furniture || {})[kind] || []).map(function (cell) { return { text: runs(cell.runs.map(function (r) { return r.field ? Object.assign({}, r, { text: String(r.field === 'page' ? number : total) }) : r; })), alignment: cell.align }; }), margin: [model.page.margins[0], Math.max(0, model.page.margins[kind === 'header' ? 1 : 3] / 2 - 6), model.page.margins[2], 0] };
        }; }
        return { content: write(model.blocks), pageSize: { width: model.page.width, height: model.page.height }, pageMargins: model.page.margins, header: running('header'), footer: running('footer'),
            pageBreakBefore:function(current,following,next,previous){return !!current.headlineLevel&&!following.length&&next.length>0&&previous.length>0;},
            background: { canvas: [{ type: 'rect', x: 0, y: 0, w: model.page.width, h: model.page.height, color: '#' + (model.page.background || 'ffffff') }] },
            defaultStyle: { font: 'Roboto', fontSize: 11, color: '#202124' },
            info: { title: model.properties.title || '', author: model.properties.author || '', subject: model.properties.subject || '', keywords: model.properties.keywords || '' } };
    }
    async function build(format, options) {
        if (format !== 'docx' && format !== 'pdf') { throw new Error('Export format is docx or pdf.'); }
        if (options.current && !options.current()) { throw new Error('The document changed during export.'); }
        var model = await prepare(options), K = window.yavcMdEditor;
        if (options.current && !options.current()) { throw new Error('The document changed during export. Export it again.'); }
        var blob;
        if (format === 'docx') {
            await K.loadScript(K.here + 'vendor/docx-9.15.2/index.js' + K.stamp);
            blob = await window.docx.Packer.toBlob(docxDocument(model, window.docx));
        } else {
            if((model.footnotes||[]).length){
                await K.loadScript(K.here+'pdf-notes.js'+K.stamp);
                blob=await window.yavcFootnotePDF.write(model,K);
            }else{
            checkGlyphs(model.blocks.concat([].concat(model.furniture.header, model.furniture.footer)), await pdfFont(K));
            await K.loadScript(K.here + 'vendor/pdfmake-0.2.23/pdfmake.min.js' + K.stamp);
            await K.loadScript(K.here + 'vendor/pdfmake-0.2.23/vfs_fonts.js' + K.stamp);
            blob = await new Promise(function (resolve, reject) { var timer = setTimeout(function () { reject(new Error('PDF generation did not finish. Try a smaller document.')); }, 30000); try { window.pdfMake.createPdf(pdfDefinition(model)).getBlob(function (value) { clearTimeout(timer); resolve(value); }); } catch (err) { clearTimeout(timer); reject(err); } });
            }
        }
        if (options.current && !options.current()) { throw new Error('The document changed during export. Export it again.'); }
        return { blob: blob, warnings: format === 'pdf' ? model.warnings : [], blocks: model.blocks.length };
    }
    window.yavcDocumentExport = Object.freeze({ prepare: prepare, build: build, settings: settings, pdfDefinition: pdfDefinition });
})();
