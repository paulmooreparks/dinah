/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* A PowerPoint file as a deck (docs/slides-design.md, decision 14): the
   .pptx is a zip of Office Open XML, read with JSZip (vendor/jszip-3.10.1,
   MIT) and the browser's XML parser. Each slide's placeholders and text
   boxes become Markdown: the title placeholder a heading, body text
   paragraphs and lists with their levels and emphasis, tables as tables,
   two bodies side by side as columns, the notes page as {{notes}}. A
   supported picture is retained as a bounded embedded raster. Losses are
   returned for review before a native copy is opened.

     yavcSlidesImport.pptx(arrayBuffer, ctx) -> Promise<{ text, title, slides, pictures }> */
(function () {
    'use strict';
    if (window.yavcSlidesImport) { return; }

    var NS = {
        a: 'http://schemas.openxmlformats.org/drawingml/2006/main',
        p: 'http://schemas.openxmlformats.org/presentationml/2006/main',
        r: 'http://schemas.openxmlformats.org/officeDocument/2006/relationships',
        rel: 'http://schemas.openxmlformats.org/package/2006/relationships'
    };
    function parse(xml) { if(/<!DOCTYPE|<!ENTITY/i.test(xml))throw new Error('Office XML entity declarations are not supported.');var doc=new DOMParser().parseFromString(xml, 'application/xml');if(doc.querySelector('parsererror'))throw new Error('The presentation contains malformed XML.');return doc; }
    function all(node, ns, name) { return node ? Array.prototype.slice.call(node.getElementsByTagNameNS(NS[ns], name)) : []; }
    function first(node, ns, name) { return all(node, ns, name)[0] || null; }
    function children(node, ns, name) { return Array.prototype.filter.call(node.childNodes, function (c) { return c.nodeType === 1 && c.namespaceURI === NS[ns] && c.localName === name; }); }

    /* A part's text, counted as it is decompressed through JSZip's
       documented stream, and stopped at a limit: a file a few kilobytes
       long can hold gigabytes once inflated (security review, 2026-11-03).
       budget is shared by every part one import reads. */
    var PART_MAX = 16 * 1024 * 1024, TOTAL_MAX = 64 * 1024 * 1024, SLIDES_MAX = 1000;
    function partBytes(f, budget) {
        return new Promise(function (resolve, reject) {
            var chunks = [], n = 0, stopped = false, stream = f.internalStream('uint8array');
            stream.on('data', function (c) {
                if (stopped) { return; }
                n += c.length; budget.used += c.length;
                if (n > PART_MAX || budget.used > TOTAL_MAX) {
                    stopped = true; stream.pause();
                    reject(new Error('this file is far larger inside than a presentation can be, so it was not read'));
                    return;
                }
                chunks.push(c);
            }).on('error', function (err) { if (!stopped) { stopped = true; reject(err); } })
              .on('end', function () {
                if (stopped) { return; }
                var all = new Uint8Array(n), at = 0;
                chunks.forEach(function (c) { all.set(c, at); at += c.length; });
                resolve(all);
            }).resume();
        });
    }
    function text(f,budget){return partBytes(f,budget).then(function(bytes){return new TextDecoder('utf-8',{fatal:true}).decode(bytes);});}
    function rels(zip, partPath, budget) {
        var dir = partPath.replace(/[^/]*$/, ''), name = partPath.replace(/^.*\//, '');
        var relPath = dir + '_rels/' + name + '.rels';
        var f = zip.file(relPath);
        if (!f) { return Promise.resolve({}); }
        return text(f, budget).then(function (xml) {
            var doc = parse(xml), out = {};
            all(doc, 'rel', 'Relationship').forEach(function (r) {
                var target = r.getAttribute('Target'), mode = r.getAttribute('TargetMode');
                out[r.getAttribute('Id')] = { type: r.getAttribute('Type'), target: mode === 'External' ? target : resolve(dir, target), external: mode === 'External' };
            });
            return out;
        });
    }
    function resolve(dir, target) {
        var parts = (dir + target).split('/'), out = [];
        parts.forEach(function (p) { if (p === '..') { out.pop(); } else if (p && p !== '.') { out.push(p); } });
        return out.join('/');
    }

    /* A body's words alone, for a title, which carries no emphasis. */
    function plainText(txBody) {
        return children(txBody, 'a', 'p').map(function (p) { return all(p, 'a', 't').map(function (t) { return t.textContent; }).join('').trim(); }).filter(Boolean).join(' ');
    }
    /* Text made safe as Markdown: its markup characters and {{ escaped. */
    function escMarkdown(s) { return String(s).replace(/[!-~]/g,function(char){var n=char.charCodeAt(0);return n>=48&&n<=57||n>=65&&n<=90||n>=97&&n<=122?char:'\\'+char;}); }
    function safeLink(value){if(!value||! /^(https?:|mailto:|#)/i.test(value))return null;return value.replace(/[\s\\()\[\]|<>{}"']/g,function(char){return encodeURIComponent(char);});}

    /* A paragraph's Markdown, with its runs' emphasis and links. */
    function runText(p, links) {
        var out = '';
        Array.prototype.forEach.call(p.childNodes, function (c) {
            if (c.nodeType !== 1) { return; }
            if (c.localName === 'br') { out += '  \n'; return; }
            if (c.localName === 'r' || c.localName === 'fld') {
                var t = first(c, 'a', 't'), text = t ? t.textContent : '';
                if (!text) { return; }
                var pr = first(c, 'a', 'rPr'), bold = pr && pr.getAttribute('b') === '1', italic = pr && pr.getAttribute('i') === '1';
                var link = pr ? first(pr, 'a', 'hlinkClick') : null, href = safeLink(link ? (links[link.getAttributeNS(NS.r, 'id')] || {}).target : null);
                var s = escMarkdown(text);
                if (s.trim()) {
                    var lead = /^\s*/.exec(s)[0], trail = /\s*$/.exec(s)[0], core = s.trim();
                    if (bold && italic) { core = '***' + core + '***'; } else if (bold) { core = '**' + core + '**'; } else if (italic) { core = '*' + core + '*'; }
                    if (href) { core = '[' + core + '](' + href + ')'; }
                    s = lead + core + trail;
                }
                out += s;
            }
        });
        return out;
    }

    /* A text body as Markdown lines: bullets by level, numbers, plain
       paragraphs. A body placeholder's paragraphs are bulleted unless
       they say otherwise; a title's and a text box's are not. */
    /* A paragraph that is one link to a hosted video is the video. */
    var VIDEO = /^\[([^\]]*)\]\((https?:\/\/(?:www\.)?(?:youtube(?:-nocookie)?\.com|youtu\.be|vimeo\.com|player\.vimeo\.com)\/[^\s)]+)\)$/;
    function bodyMarkdown(txBody, links, bulleted) {
        var lines = [], lastBlank = true;
        children(txBody, 'a', 'p').forEach(function (p) {
            var text = runText(p, links).replace(/\s+$/, '');
            var vm = VIDEO.exec(text.trim());
            if (vm) { text = '{{video:' + vm[2] + (vm[1] ? ' title="' + vm[1].replace(/"/g, '') + '"' : '') + '}}'; bulleted = false; }
            var pPr = first(p, 'a', 'pPr'), lvl = pPr ? Number(pPr.getAttribute('lvl') || '0') : 0;
            if(!Number.isInteger(lvl)||lvl<0||lvl>8)throw new Error('A presentation list exceeds the supported nesting depth.');
            var none = pPr && first(pPr, 'a', 'buNone'), auto = pPr && first(pPr, 'a', 'buAutoNum'), chr = pPr && (first(pPr, 'a', 'buChar') || first(pPr, 'a', 'buFont'));
            var bullet = !none && (bulleted || !!chr || !!auto);
            if (!text.trim()) { if (!lastBlank) { lines.push(''); lastBlank = true; } return; }
            if (bullet) { lines.push('  '.repeat(lvl) + (auto ? '1. ' : '- ') + text.trim()); lastBlank = false; }
            else { if (!lastBlank) { lines.push(''); } lines.push(text.trim()); lines.push(''); lastBlank = true; }
        });
        return lines.join('\n').replace(/\n{3,}/g, '\n\n').replace(/^\n+|\n+$/g, '');
    }

    function tableMarkdown(tbl, links) {
        var rows = all(tbl, 'a', 'tr').map(function (tr) {
            return children(tr, 'a', 'tc').map(function (tc) {
                var body = first(tc, 'a', 'txBody');
                return body ? children(body, 'a', 'p').map(function (p) { return runText(p, links).trim().replace(/\r?\n/g,' '); }).join(' ') : '';
            });
        });
        if (!rows.length) { return ''; }
        var n = Math.max.apply(null, rows.map(function (r) { return r.length; }));
        var line = function (r) { while (r.length < n) { r.push(''); } return '| ' + r.join(' | ') + ' |'; };
        return [line(rows[0]), '| ' + rows[0].map(function () { return '---'; }).join(' | ') + ' |'].concat(rows.slice(1).map(line)).join('\n');
    }

    function phType(sp) {
        var ph = first(sp, 'p', 'ph');
        return ph ? (ph.getAttribute('type') || 'body') : null;
    }
    function offsetOf(sp) {
        var off = first(sp, 'a', 'off');
        return off ? { x: +off.getAttribute('x') || 0, y: +off.getAttribute('y') || 0 } : { x: 0, y: 0 };
    }

    async function pptx(buffer, ctx) {
        if(!(buffer instanceof ArrayBuffer)||buffer.byteLength>8*1024*1024)throw new Error('Choose a PowerPoint file no larger than 8 MiB.');
        await ctx.loadScript(ctx.here + 'vendor/jszip-3.10.1/jszip.min.js' + ctx.stamp);
        var kit=window.yavcMdEditor;await ctx.loadScript(kit.here+'import-images.js'+kit.stamp);
        var zip = await window.JSZip.loadAsync(buffer);
        var entries=Object.values(zip.files);if(entries.length>4096)throw new Error('The presentation has too many archive entries.');
        entries.forEach(function(entry){var path=entry.unsafeOriginalName||entry.name;if(path.length>512||/^[\/\\]|[\\:\x00-\x1f]/.test(path)||path.split('/').some(function(part){return part==='..'||part==='.';}))throw new Error('The presentation contains an invalid archive path.');});
        var presFile = zip.file('ppt/presentation.xml');
        if (!presFile) { throw new Error('this is not a PowerPoint file: it has no presentation inside'); }
        var budget = { used: 0 };
        var pres = parse(await text(presFile, budget));
        var presRels = await rels(zip, 'ppt/presentation.xml', budget);
        var size = first(pres, 'p', 'sldSz');
        var ratio = size ? (+size.getAttribute('cx') / +size.getAttribute('cy')) : 16 / 9;
        var ids = all(pres, 'p', 'sldId').map(function (s) { return s.getAttributeNS(NS.r, 'id'); });
        if (ids.length > SLIDES_MAX) { throw new Error('this presentation has ' + ids.length + ' slides, more than the ' + SLIDES_MAX + ' a deck can take'); }
        var slides = [], pictures = 0, title = '',imageBytes=0,omitted=0,report=['Slide positions, fonts, transitions and animations become native deck layout.'];

        for (var i = 0; i < ids.length; i++) {
            var part = (presRels[ids[i]] || {}).target;
            var f = part && zip.file(part);
            if (!f) { throw new Error('A slide listed in the presentation is missing.'); }
            var doc = parse(await text(f, budget)), links = await rels(zip, part, budget);
            var tree = first(doc, 'p', 'spTree');
            var titleText = '', subtitle = '', bodies = [], extras = [], notes = '',pictureParts=[];
            if(first(doc,'p','timing'))report.push('Slide '+(i+1)+' has animations that are not imported.');
            var shapes = tree ? Array.prototype.filter.call(tree.childNodes, function (c) { return c.nodeType === 1; }) : [];
            shapes.forEach(function (sp) {
                if (sp.localName === 'sp') {
                    var type = phType(sp), txBody = first(sp, 'p', 'txBody');
                    if (!txBody) { return; }
                    if (type === 'title' || type === 'ctrTitle') { titleText = plainText(txBody); return; }
                    if (type === 'subTitle') { subtitle = bodyMarkdown(txBody, links, false); return; }
                    if (type === 'sldNum' || type === 'dt' || type === 'ftr') { return; }
                    var md = bodyMarkdown(txBody, links, type === 'body' || type === 'obj');
                    if (md) { bodies.push({ text: md,plain:plainText(txBody), x: offsetOf(sp).x, y: offsetOf(sp).y, placeholder: !!type, lines: children(txBody, 'a', 'p').length }); }
                } else if (sp.localName === 'graphicFrame') {
                    var tbl = first(sp, 'a', 'tbl');
                    if (tbl) { extras.push(tableMarkdown(tbl, links));report.push('Table geometry and cell paragraph breaks on slide '+(i+1)+' are flattened to a native table.'); } else { extras.push('_[A chart or diagram from the original.]_');report.push('A chart or diagram on slide '+(i+1)+' was replaced by a placeholder.'); }
                } else if (sp.localName === 'pic') {
                    var name = (first(sp, 'p', 'cNvPr') || {}).getAttribute ? first(sp, 'p', 'cNvPr').getAttribute('descr') || first(sp, 'p', 'cNvPr').getAttribute('name') : 'picture';
                    var blip=first(sp,'a','blip'),relationship=blip&&links[blip.getAttributeNS(NS.r,'embed')];
                    pictureParts.push({slot:extras.length,name:name||'picture',relationship:relationship});extras.push('_[Picture omitted: '+escMarkdown(name||'picture')+']_');
                } else if (sp.localName === 'grpSp') {
                    if(all(sp,'p','pic').length||all(sp,'p','graphicFrame').length)report.push('Grouped pictures or graphics on slide '+(i+1)+' are not imported.');
                    all(sp, 'p', 'txBody').forEach(function (tb) { var md = bodyMarkdown(tb, links, false); if (md) { bodies.push({ text: md,plain:plainText(tb), x: 0, placeholder: false }); } });
                }
            });
            for(var picture of pictureParts){
                var relationship=picture.relationship,image=relationship&&!relationship.external&&zip.file(relationship.target),extension=image&&/\.([^.]+)$/.exec(image.name),type=extension&&({png:'image/png',jpg:'image/jpeg',jpeg:'image/jpeg',gif:'image/gif'})[extension[1].toLowerCase()];
                if(!image||!type){omitted++;continue;}
                var bytes=await partBytes(image,budget),data=imageBytes+bytes.length<=yavcImportImages.maxBytes&&yavcImportImages.dataURL(bytes,type);
                if(!data){omitted++;continue;}imageBytes+=bytes.length;pictures++;extras[picture.slot]='!['+escMarkdown(picture.name).replace(/\r?\n/g,' ')+']('+data+')';
            }
            for (var id in links) {
                if (links[id].type && /notesSlide$/.test(links[id].type)) {
                    var nf = zip.file(links[id].target);
                    if (nf) {
                        var ndoc = parse(await text(nf, budget)),noteLinks=await rels(zip,links[id].target,budget);
                        all(ndoc, 'p', 'sp').forEach(function (sp) {
                            if (phType(sp) === 'body') { var tb = first(sp, 'p', 'txBody'); if (tb) { notes = bodyMarkdown(tb, noteLinks, false); } }
                        });
                    }
                }
            }
            /* A file from a tool that writes text boxes rather than
               placeholders: the topmost short one-line box is the title. */
            if (!titleText && bodies.length) {
                bodies.sort(function (a, b) { return a.y - b.y; });
                var top = bodies[0], plain = top.plain;
                if (top.lines === 1 && plain.length <= 90 && plain.indexOf('\n') < 0) { titleText = plain; bodies.shift(); }
            }
            var layout = 'content', parts = [];
            var placeholders = bodies.filter(function (b) { return b.placeholder; });
            if (i === 0 && !bodies.length) { layout = 'title'; if (!title) { title = titleText; } }
            else if (!bodies.length && !extras.length && titleText) { layout = 'section'; }
            if (layout === 'title') {
                parts.push('# ' + escMarkdown(titleText || 'Untitled'));
                if (subtitle) { parts.push('## ' + subtitle.replace(/\n+/g, ' ')); }
            } else {
                if (titleText) { parts.push('## ' + escMarkdown(titleText)); }
                if (subtitle) { parts.push(subtitle); }
                if (placeholders.length === 2 && bodies.length === 2) {
                    layout = 'two-column';
                    bodies.sort(function (a, b) { return a.x - b.x; });
                    parts.push('{{columns}}\n' + bodies[0].text + '\n\n{{column}}\n\n' + bodies[1].text + '\n{{/columns}}');
                } else {
                    bodies.forEach(function (b) { parts.push(b.text); });
                }
            }
            extras.forEach(function (e) { parts.push(e); });
            var slideSource = '{{slide' + (layout !== 'content' ? ':layout=' + layout : '') + '}}\n' + parts.join('\n\n') + '\n';
            if (notes) { slideSource += '\n{{notes}}\n' + notes + '\n{{/notes}}\n'; }
            slides.push(slideSource);
            if (!title && titleText) { title = titleText; }
        }
        if (!slides.length) { throw new Error('the file has no slides'); }
        var front = '---\ntitle: ' + JSON.stringify((title || 'An imported deck').replace(/\n/g, ' ')) + '\nsize: ' + JSON.stringify(Math.abs(ratio - 4 / 3) < 0.05 ? '4:3' : '16:9') + '\n---\n';
        if(omitted)report.push(omitted+' image(s) were omitted because their type, relationship or combined size exceeds the native-image budget.');
        var output=front+slides.join('\n---\n\n');if(new TextEncoder().encode(output).length>256*1024)throw new Error('The native deck exceeds the 256 KiB import limit.');
        return { text:output, title:title, slides:slides.length, pictures:pictures,omitted:omitted,report:report.slice(0,100) };
    }

    window.yavcSlidesImport = Object.freeze({ pptx: pptx });
})();
