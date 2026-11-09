/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Measured, selectable PDF text with page-owned notes. Only public PDF-lib APIs are used. */
(function () {
    'use strict';
    async function write(model, K) {
        await K.loadScript(K.here + 'vendor/pdf-lib-1.17.1/pdf-lib.min.js' + K.stamp);
        await K.loadScript(K.here + 'vendor/fontkit-1.1.1/fontkit.js' + K.stamp);
        var D = window.PDFLib, pdf = await D.PDFDocument.create(), fonts = [];
        pdf.registerFontkit(window.fontkit);
        for (var name of ['Roboto-Regular.ttf', 'Roboto-Medium.ttf', 'Roboto-Italic.ttf', 'Roboto-MediumItalic.ttf']) {
            var response = await fetch(K.here + 'vendor/pdfmake-0.2.23/' + name + K.stamp);
            if (!response.ok) throw new Error('The bundled PDF font is missing.');
            fonts.push(await pdf.embedFont(await response.arrayBuffer(), { subset: true }));
        }
        var coverage = fonts.map(function (f) { return new Set(f.getCharacterSet()); });
        var page = model.page, margins = page.margins, width = page.width - margins[0] - margins[2], height = page.height - margins[1] - margins[3];
        var notes = new Map(), pages = [], current = null, seen = new Set(), pending = [], links = [], destinations = new Map();
        function color(hex) { hex = hex || '202124'; return D.rgb(parseInt(hex.slice(0, 2), 16) / 255, parseInt(hex.slice(2, 4), 16) / 255, parseInt(hex.slice(4, 6), 16) / 255); }
        function font(run) { return fonts[(run.bold ? 1 : 0) + (run.italic ? 2 : 0)]; }
        function size(run) { return (run.size || 11) * (run.sup || run.sub || run.footnote ? .75 : 1); }
        function measure(text, run) {
            var f = font(run), allowed = coverage[fonts.indexOf(f)];
            for (var c of text) if (!allowed.has(c.codePointAt(0))) throw new Error('The bundled PDF font cannot display ' + c + '. Export to Word or replace that character.');
            return f.widthOfTextAtSize(text, size(run));
        }
        function lines(runs, available, style, indent) {
            var result = [], parts = [], used = 0, maximum = 11, refs = new Set();
            function flush(force, hard) {
                if (!parts.length && !force) return;
                if(!style.preserve)while (parts.length && /^ +$/.test(parts[parts.length - 1].text)) { used -= parts.pop().width; }
                result.push({ kind: 'line', parts: parts, width: used, height: maximum * (style.leading || 1.5), align: style.align || 'left', hard:!!hard, indent: indent || 0, references: Array.from(refs) });
                parts = []; used = 0; maximum = 11; refs = new Set();
            }
            runs.forEach(function (run) {
                String(run.text || '').replace(/\r/g, '').split(/(\n|[^\S\n]+|[^\s]+)/u).filter(Boolean).forEach(function (token) {
                    if (token === '\n') { flush(true,true); return; }
                    token = token.replace(/\t/g, '    ');
                    if (!style.preserve&&!parts.length && /^ +$/.test(token)) return;
                    var w = measure(token, run);
                    if (used + w > available + .01 && parts.length) flush(false);
                    if (w > available + .01) {
                        for (var character of token) {
                            var cw = measure(character, run);
                            if (cw > available + .01) throw new Error('A PDF character is wider than the available page space.');
                            if (used + cw > available + .01 && parts.length) flush(false);
                            parts.push({ text: character, run: run, width: cw }); used += cw; maximum = Math.max(maximum, run.size || 11); if (run.footnote) refs.add(run.footnote);
                        }
                    } else if (style.preserve||parts.length || !/^ +$/.test(token)) {
                        parts.push({ text: token, run: run, width: w }); used += w; maximum = Math.max(maximum, run.size || 11); if (run.footnote) refs.add(run.footnote);
                    }
                });
            });
            flush(false);if(result.length)result[result.length-1].hard=true;return result;
        }
        function units(blocks, indent) {
            var result = [];
            blocks.forEach(function (block) {
                var available = width - indent;
                if (available < 12) throw new Error('PDF indentation leaves too little room for text.');
                if (block.kind === 'paragraph') {
                    var paragraph = lines(block.runs, available, Object.assign({},block.style||{},{preserve:!!block.code}), indent);
                    paragraph.forEach(function (line, i) { line.keepNext = i < Math.min(2, paragraph.length) - 1 || i >= paragraph.length - 2 && i < paragraph.length - 1 || i === paragraph.length - 1 && !!(block.style || {}).keepNext; });
                    if (paragraph.length) { paragraph[0].id = block.id; paragraph[paragraph.length - 1].height += (block.style || {}).after || 0; }
                    result.push.apply(result, paragraph);
                } else if (block.kind === 'list') {
                    block.items.forEach(function (item, index) {
                        var items = units(item, indent + 18), marker = { text: block.ordered ? String((block.start || 1) + index) + '.' : '\u2022', run: {}, width: 0 };
                        if (items.length) items[0].marker = marker;
                        result.push.apply(result, items);
                    });
                } else if (block.kind === 'quote') result.push.apply(result, units(block.blocks, indent + 12));
                else if (block.kind === 'table') {
                    var cellWidth = available / block.rows[0].length, header = [], table = {};
                    block.rows.forEach(function (row, ri) {
                        var cells = row.map(function (cell) { return lines(cell.runs, cellWidth - 8, cell.style || {}, 0); }), count = Math.max.apply(null, cells.map(function (cell) { return cell.length; }));
                        for (var i = 0; i < count; i++) {
                            var entry = { kind: 'row', cells: cells.map(function (cell) { return cell[i]; }), indent: indent, cellWidth: cellWidth, table: table, header: ri === 0, references: [], height: 6 };
                            entry.cells.forEach(function (cell) { if (cell) { entry.height = Math.max(entry.height, cell.height + 6); entry.references.push.apply(entry.references, cell.references); } });
                            if (ri === 0) header.push(entry); result.push(entry);
                        }
                    });
                    table.headers = header;
                } else if (block.kind === 'image') { var scale = Math.min(1, available / block.width, (height - 6) / block.height); result.push(Object.assign({}, block, { indent: indent, width: block.width * scale, references: [], height: block.height * scale + 6 })); }
                else if (block.kind === 'rule') result.push({ kind: 'rule', height: 16, indent: indent, references: [] });
                else if (block.kind === 'break') result.push({ kind: 'break', height: 0, references: [] });
                else throw new Error('This PDF block is not supported.');
                if (result.length > 200000) throw new Error('PDF export is limited to 200,000 measured lines.');
            });
            return result;
        }
        (model.footnotes || []).forEach(function (note) {
            var body = units(note.blocks, 18);
            if (!body.length) body = lines([{ text: ' ' }], width - 18, {}, 18);
            body[0].marker = { text: String(note.number) + '.', run: {}, width: 0 };
            notes.set(note.label, body);
        });
        function newPage() { current = { body: [], notes: [], used: 0, reserved: 0 }; pages.push(current); if (pages.length > 1000) throw new Error('PDF export is limited to 1,000 pages.'); }
        function proposal(references) {
            var queue = pending.slice(), fresh = [];
            references.forEach(function (label) { if (!seen.has(label) && fresh.indexOf(label) < 0) { var body = notes.get(label); if (!body) throw new Error('A PDF footnote reference has no definition.'); queue.push({ label: label, units: body, offset: 0 }); fresh.push(label); } });
            var mandatory = queue.reduce(function (sum, entry) { return sum + (fresh.indexOf(entry.label) >= 0 ? entry.units[entry.offset].height : 0); }, 0);
            var taken = [], used = current.reserved, limit = Math.max(height * .45, mandatory + 8, 32), rest = [];
            queue.forEach(function (entry) {
                var at = entry.offset;
                if(fresh.indexOf(entry.label)>=0)mandatory-=entry.units[at].height;
                while (at < entry.units.length && used + entry.units[at].height + (used ? 0 : 8) + mandatory <= limit + .01) { if (!used) used = 8; taken.push(entry.units[at++]); used += entry.units[at - 1].height; }
                if (at < entry.units.length) rest.push({ label: entry.label, units: entry.units, offset: at });
            });
            if (fresh.some(function (label) { return !taken.some(function (unit) { return notes.get(label).indexOf(unit) === 0; }); })) return null;
            return { taken: taken, reserved: used, rest: rest, fresh: fresh };
        }
        function commit(p) { current.notes.push.apply(current.notes, p.taken); current.reserved = p.reserved; pending = p.rest; p.fresh.forEach(function (label) { seen.add(label); }); }
        var body = units(model.blocks, 0), ends=[],heights=[],references=[];
        for(var bi=body.length-1;bi>=0;bi--){
            var follows=body[bi].keepNext&&body[bi+1]&&body[bi+1].kind!=='break';
            ends[bi]=follows?ends[bi+1]:bi;
            heights[bi]=body[bi].height+(follows?heights[bi+1]:0);
            references[bi]=new Set((body[bi].references||[]).concat(follows?Array.from(references[bi+1]):[]));
        }
        newPage();
        for (var index = 0; index < body.length; index++) {
            var unit = body[index];
            if (unit.kind === 'break') { if (current.body.length || current.notes.length) newPage(); continue; }
            var attempts = 0;
            for (;;) {
                if (++attempts > 1001) throw new Error('A PDF item cannot fit on the page.');
                var keepHeight=heights[index]<=height?heights[index]:unit.height,p=proposal(keepHeight>unit.height?Array.from(references[index]):unit.references||[]);
                if(keepHeight>unit.height&&!current.body.length&&!current.notes.length&&!pending.length&&(!p||keepHeight+p.reserved>height)){keepHeight=unit.height;p=proposal(unit.references||[]);}
                var headers = unit.kind === 'row' && !unit.header && !current.body.some(function (item) { return item.table === unit.table; }) ? unit.table.headers : [];
                var repeatHeight = headers.reduce(function (n, h) { return n + h.height; }, 0), required = repeatHeight + keepHeight;
                if (p && current.used + required + p.reserved <= height + .01) { commit(p); headers.forEach(function (h) { current.body.push(h); current.used += h.height; }); current.body.push(unit); current.used += unit.height; break; }
                if (!current.body.length&&!current.notes.length) {
                    if (!pending.length) throw new Error('A PDF item or its first footnote line is too large for the page.');
                    var continuation = proposal([]); if (!continuation || !continuation.taken.length) throw new Error('A PDF footnote item is too large for the page.'); commit(continuation);
                }
                newPage();
            }
        }
        while (pending.length) { if (current.body.length || current.notes.length) newPage(); var continuation = proposal([]); if (!continuation || !continuation.taken.length) throw new Error('A PDF footnote item is too large for the page.'); commit(continuation); }
        var output = pages.map(function () { return pdf.addPage([page.width, page.height]); });
        function line(target, item, x, top, available) {
            var offset = item.align === 'right' ? available - item.width : item.align === 'center' ? (available - item.width) / 2 : 0;
            x += Math.max(0, offset);
            var spaces=item.parts.reduce(function(sum,part){return sum+(/^ +$/.test(part.text)?part.text.length:0);},0),extra=item.align==='justify'&&!item.hard&&spaces?Math.max(0,available-item.width)/spaces:0;
            if (item.marker) target.drawText(item.marker.text, { x: x - 16, y: page.height - top - 11, font: fonts[0], size: 11, color: color() });
            item.parts.forEach(function (part) {
                var r = part.run, s = size(r), baseline = page.height - top - (r.size || 11) + (r.sup || r.footnote ? s * .35 : r.sub ? -s * .2 : 0);
                if (r.mark) target.drawRectangle({ x: x, y: baseline - s * .2, width: part.width, height: s * 1.2, color: D.rgb(1, 1, 0) });
                target.drawText(part.text, { x: x, y: baseline, font: font(r), size: s, color: color(r.color) });
                [r.underline ? baseline - 1 : null, r.strike ? baseline + s * .3 : null].forEach(function (y) { if (y !== null) target.drawLine({ start: { x: x, y: y }, end: { x: x + part.width, y: y }, thickness: .5, color: color(r.color) }); });
                if (r.link || r.footnote) links.push({ page: target, rect: [x, baseline - 2, x + part.width, baseline + s], href: r.footnote ? '#footnote-' + r.footnote : r.link });
                x += part.width+(extra&&/^ +$/.test(part.text)?extra*part.text.length:0);
            });
        }
        async function draw(target, item, top) {
            if (item.id && !destinations.has(item.id)) destinations.set(item.id, { page: target, y: page.height - top });
            if (item.kind === 'line') line(target, item, margins[0] + item.indent, top, width - item.indent);
            else if (item.kind === 'row') item.cells.forEach(function (cell, i) {
                var x = margins[0] + item.indent + i * item.cellWidth;
                target.drawRectangle({ x: x, y: page.height - top - item.height, width: item.cellWidth, height: item.height, borderColor: color('b8bcc2'), borderWidth: .5, color: item.header ? color('e6e8ec') : color(page.background) });
                if (cell) line(target, cell, x + 4, top + 3, item.cellWidth - 8);
            });
            else if (item.kind === 'image') target.drawImage(await pdf.embedPng(item.data), { x: margins[0] + item.indent, y: page.height - top - item.height + 6, width: item.width, height: item.height - 6 });
            else if (item.kind === 'rule') target.drawLine({ start: { x: margins[0], y: page.height - top - 8 }, end: { x: page.width - margins[2], y: page.height - top - 8 }, thickness: .5, color: color('808080') });
        }
        for (var pi = 0; pi < pages.length; pi++) {
            var target = output[pi], sheet = pages[pi], top = margins[1];
            target.drawRectangle({ x: 0, y: 0, width: page.width, height: page.height, color: color(page.background || 'ffffff') });
            for (var unit of sheet.body) { await draw(target, unit, top); top += unit.height; }
            top = page.height - margins[3] - sheet.reserved;
            if (sheet.notes.length) { target.drawLine({ start: { x: margins[0], y: page.height - top }, end: { x: margins[0] + Math.min(width / 3, 144), y: page.height - top }, thickness: .5, color: color('808080') }); top += 8; }
            for (var noteUnit of sheet.notes) {
                notes.forEach(function (items, label) { if (items[0] === noteUnit) destinations.set('footnote-' + label, { page: target, y: page.height - top }); });
                await draw(target, noteUnit, top); top += noteUnit.height;
            }
            ['header', 'footer'].forEach(function (kind) {
                if (kind === 'header' && pi === 0 && !page.furniture.first) return;
                var cells = model.furniture[kind] || [], cellWidth = width / Math.max(1, cells.length);
                cells.forEach(function (cell, ci) {
                    var runs = cell.runs.map(function (r) { return r.field ? Object.assign({}, r, { text: String(r.field === 'page' ? pi + 1 : pages.length) }) : r; });
                    var y = kind === 'header' ? Math.max(0, margins[1] / 2 - 6) : page.height - margins[3] / 2 - 6;
                    lines(runs, cellWidth, { align: cell.align }, 0).forEach(function (item) { line(target, item, margins[0] + ci * cellWidth, y, cellWidth); y += item.height; });
                });
            });
        }
        links.forEach(function (link) {
            var annotation = { Type: 'Annot', Subtype: 'Link', Rect: link.rect, Border: [0, 0, 0] };
            if (link.href[0] === '#') { var dest = destinations.get(link.href.slice(1)); if (!dest) return; annotation.Dest = [dest.page.ref, D.PDFName.of('XYZ'), null, dest.y, null]; }
            else { var url = new URL(link.href, location.href); if (!/^https?:$/.test(url.protocol) && url.protocol !== 'mailto:') return; annotation.A = { Type: 'Action', S: 'URI', URI: D.PDFString.of(url.href) }; }
            link.page.node.addAnnot(pdf.context.register(pdf.context.obj(annotation)));
        });
        pdf.setTitle(model.properties.title || ''); pdf.setAuthor(model.properties.author || ''); pdf.setSubject(model.properties.subject || '');
        return new Blob([await pdf.save()], { type: 'application/pdf' });
    }
    window.yavcFootnotePDF = Object.freeze({ write: write });
})();
