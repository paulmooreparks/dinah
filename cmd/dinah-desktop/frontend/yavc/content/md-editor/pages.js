/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Physical page settings and measured source-free pagination. */
(function () {
    'use strict';
    var GAP = 24, LIMIT = 1000;
    function paper(value) {
        var name = String(value || 'a4').toLowerCase();
        if (name === 'ledger') { name = 'tabloid'; }
        return window.yavcPaperSizes.find(function (item) { return item.id === name || item.pwg === name; });
    }
    function length(value) {
        var m = /^(\d+(?:\.\d+)?)\s*(pt|in|cm|mm|px)?$/.exec(String(value).trim());
        if (!m) { throw new Error('Page lengths use pt, in, cm, mm or px.'); }
        return +m[1] * ({ pt: 1, in: 72, cm: 72 / 2.54, mm: 72 / 25.4, px: 0.75 }[m[2] || 'pt']);
    }
    function settings(front) {
        var stock = paper(front.page), size = stock ? [length(stock.width + stock.unit), length(stock.height + stock.unit)] : null;
        if (!size) {
            var custom = /^(\d+(?:\.\d+)?\s*(?:pt|in|cm|mm|px))\s*[x×]\s*(\d+(?:\.\d+)?\s*(?:pt|in|cm|mm|px))$/i.exec(String(front.page || ''));
            if (!custom) { throw new Error('Choose a standard paper size or two lengths such as 210mm x 297mm.'); }
            size = [length(custom[1]), length(custom[2])];
        }
        size = size.slice();
        if (front.orientation && !/^(portrait|landscape)$/.test(front.orientation)) { throw Object.assign(new Error('Page orientation is portrait or landscape.'), { field: 'orientation' }); }
        var landscape = front.orientation ? front.orientation === 'landscape' : size[0] > size[1];
        size.sort(function (a, b) { return a - b; }); if (landscape) { size.reverse(); }
        if (Math.min.apply(null, size) < 72 || Math.max.apply(null, size) > 8640) { throw new Error('Page dimensions must be between 1 and 120 inches.'); }
        var values;
        try { values = String(front.margins || '1in').trim().split(/\s+/).map(length); }
        catch (err) { err.field = 'margins'; throw err; }
        if (values.length > 4) { throw Object.assign(new Error('Margins contain one to four lengths.'), { field: 'margins' }); }
        var top = values[0], right = values[1] == null ? top : values[1], bottom = values[2] == null ? top : values[2], left = values[3] == null ? right : values[3];
        if (size[0] - left - right < 72 || size[1] - top - bottom < 72) { throw Object.assign(new Error('Page margins leave too little space for content.'), { field: 'margins' }); }
        return { width: size[0], height: size[1], margins: [left, top, right, bottom] };
    }
    function properties(front) {
        if (front.problem) { throw new Error(front.problem); }
        var props = {};
        (front.entries || []).forEach(function (entry) {
            if (['page', 'orientation', 'margins'].indexOf(entry.key) >= 0) {
                if (entry.kind === 'complex') { throw new Error('Page settings must be plain values.'); }
                props[entry.key] = entry.value;
            }
        });
        return props;
    }
    function dimensions(page) {
        return { width: page.width / .75, height: page.height / .75, left: page.margins[0] / .75, top: page.margins[1] / .75,
            right: page.margins[2] / .75, bottom: page.margins[3] / .75, content: (page.height - page.margins[1] - page.margins[3]) / .75 };
    }
    function surface(host, page) {
        var d = dimensions(page);
        host.classList.add('doc-paginated');
        Object.entries({ width: d.width, height: d.height, left: d.left, top: d.top, right: d.right, bottom: d.bottom, gap: GAP }).forEach(function (pair) {
            host.style.setProperty('--sheet-' + pair[0], pair[1] + 'px');
        });
        return d;
    }
    // Input units exclude existing widgets. A unit is a line or an atomic block.
    function plan(blocks, page) {
        var d = dimensions(page), gaps = [], sheet = 0, added = 0, oversized = false, ends = [], references = [], noteProblem = '',rowOrigins=new Map();
        var hasNotes=blocks.footnotes&&blocks.footnotes.length||blocks.some(function(block){return block.references&&block.references.length;});
        var notes = hasNotes && window.yavcPageFootnotes ? window.yavcPageFootnotes.create(blocks,d) : null;
        // Resolve keep chains once, so a long run of headings stays linear.
        for(var i=blocks.length-1;i>=0;i--){
            ends[i]=blocks[i].keepNext&&i+1<blocks.length&&!blocks[i+1].breakBefore&&!blocks[i+1].control?ends[i+1]:i;
            if(notes){var following=ends[i]>i?references[i+1]:new Set(),own=blocks[i].references||[];references[i]=own.every(function(label){return following.has(label);})?following:new Set(Array.from(following).concat(own));}
        }
        blocks.forEach(function (block, index) {
            if(block.rowNode&&!rowOrigins.has(block.rowNode))rowOrigins.set(block.rowNode,block.rowTop+added);
            var top = block.top + added, start = sheet * (d.height + GAP), used = top - start;
            var last=blocks[ends[index]],group=last.top+last.height-block.top,height=group<=d.content+.5?Math.max(block.height,group):block.height;
            var noteProposal=notes?notes.proposal(height>block.height?references[index]:new Set(block.references||[]),d.content-used-height):null;
            if (index && (block.breakBefore || (used > .5 && used + height + (noteProposal?noteProposal.height:0) > d.content + .5))) {
                var space = d.height + GAP - used;
                var header = block.repeatHeader || 0;
                gaps.push({ index: index, pos: block.pos, height: Math.max(0, space), header: header,headerTop:(sheet+1)*(d.height+GAP),rowHeaderTop:block.rowNode?(sheet+1)*(d.height+GAP)-rowOrigins.get(block.rowNode):0 });
                added += Math.max(0, space) + header; sheet++;
                if(block.rowNode&&!block.band)rowOrigins.set(block.rowNode,block.rowTop+added);
                if (header + block.height > d.content + .5) oversized = true;
                used=header;
                if(notes){notes.next(d.content-height-header);noteProposal=notes.proposal(height>block.height?references[index]:new Set(block.references||[]),d.content-height-header);}
            }
            if(notes){
                if(height>block.height&&used+height+noteProposal.height>d.content+.5){height=block.height;noteProposal=notes.proposal(new Set(block.references||[]),d.content-used-height);}
                if(used+block.height+noteProposal.height>d.content+.5){oversized=true;noteProblem='A footnote leaves too little space for its reference on the page. Shorten the note or enlarge the page.';}
                notes.commit(noteProposal);
            }
            if (block.height > d.content + .5) { oversized = true; }
            if (sheet >= LIMIT) { throw new Error('Document view is limited to 1,000 pages.'); }
        });
        var result={ gaps: gaps, count: notes?Math.max(sheet+1,notes.finish()):sheet+1, oversized: oversized };
        if(notes){result.footnotes=notes.placements();result.problem=notes.problem()||noteProblem;}
        if(blocks.noteProblem)result.problem=blocks.noteProblem;
        return result;
    }
    function blocksOf(dom, positions, position) {
        var host = dom.closest('.doc-paginated'), scale = host ? Number(host.style.zoom) || 1 : 1;
        var top = dom.getBoundingClientRect().top, blocks = [], sourceIndex = 0;
        var ownNote=dom.closest('aside.doc-footnote'),gapGroups=new Map(),gaps=[];
        Array.from(dom.querySelectorAll('.doc-page-gap')).filter(function(node){return node.closest('aside.doc-footnote')===ownNote&&getComputedStyle(node).position!=='absolute';}).forEach(function(node){
            var rect=node.getBoundingClientRect(),key=node.getAttribute('data-page-gap-key'),gap={bottom:rect.bottom,height:rect.height/scale};
            if(key){var prior=gapGroups.get(key);if(prior){prior.bottom=Math.max(prior.bottom,gap.bottom);prior.height=Math.max(prior.height,gap.height);return;}gapGroups.set(key,gap);}gaps.push(gap);
        });
        blocks.gapHeight=gaps.reduce(function(sum,gap){return sum+gap.height;},0);
        var noteNodes=Array.from(dom.querySelectorAll('aside.doc-footnote[data-footnote]')).filter(function(node){return Number(node.getAttribute('data-footnote-number'))>0;}),noteOffsets=noteNodes.filter(function(node){return getComputedStyle(node).position!=='absolute';}).map(function(node){var rect=node.getBoundingClientRect(),css=getComputedStyle(node);return {bottom:rect.bottom,height:rect.height/scale+(parseFloat(css.marginTop)||0)+(parseFloat(css.marginBottom)||0)};}),offsets=gaps.concat(noteOffsets);
        if(noteNodes.some(function(node){return node.closest('.pm-source');}))blocks.noteProblem='A footnote uses source-only content. Edit its Markdown or simplify the unsupported block before using Document view.';
        function clean(y) { return (y - top) / scale - offsets.reduce(function (sum, gap) { return sum + (gap.bottom <= y + .5 ? gap.height : 0); }, 0); }
        blocks.footnotes=noteNodes.map(function(node){
            var body=node.querySelector(':scope > .doc-footnote-body'),rect=node.getBoundingClientRect(),units=body?blocksOf(body,null,position):[],height=rect.height/scale-(units.gapHeight||0);
            var prefix=units.length?(body.getBoundingClientRect().top-rect.top)/scale+units[0].top:0,last=units[units.length-1],tail=last?Math.max(0,height-prefix-(last.top+last.height-units[0].top)):0;
            var parent=node.offsetParent,parentRect=parent&&parent.getBoundingClientRect(),flowRect=dom.getBoundingClientRect(),originTop=parentRect?(parentRect.top-flowRect.top)/scale+parent.clientTop:0,originLeft=parentRect?(parentRect.left-flowRect.left)/scale+parent.clientLeft:0;
            return {label:node.getAttribute('data-footnote'),number:Number(node.getAttribute('data-footnote-number')),node:node,body:body,units:units,prefix:prefix,tail:tail,height:height,sourceOnly:!!node.closest('.pm-source'),originTop:originTop,originLeft:originLeft,width:flowRect.width/scale,pos:position?position(node.parentNode,Array.prototype.indexOf.call(node.parentNode.childNodes,node)):0,to:position?position(node.parentNode,Array.prototype.indexOf.call(node.parentNode.childNodes,node)+1):0,references:window.yavcPageFootnotes.references(node)};
        });
        function emit(node, suppliedPosition) {
            if (node.classList.contains('doc-page-gap')) return;
            if(noteNodes.indexOf(node)>=0)return;
            var rect = node.getBoundingClientRect(), css = getComputedStyle(node), content=node;
            while(content.matches('.doc-block,.pm-source,.dinah-src,.dinah-block')&&content.childElementCount===1)content=content.firstElementChild;
            var boxBody=content.matches('.pm-box')&&content.querySelector('.pm-box-frame:not(.sc-columns) .pm-box-body');
            if(boxBody&&!/^(avoid|avoid-page)$/.test(getComputedStyle(boxBody.parentElement).breakInside))content=boxBody;
            var contentCSS=getComputedStyle(content),previous=blocks[blocks.length-1];
            var shortcodeBody=content.matches('div,section,aside,article,details,figure')&&Array.from(content.classList).some(function(name){return name.startsWith('sc-');})&&/^(block|flow-root)$/.test(contentCSS.display)&&content.childElementCount>0;
            var container = shortcodeBody || content.matches('ul,ol,blockquote,.pm-task-body,.pm-box-body,.sc-notice,.notice-content,.notice-body,table,thead,tbody') || content.matches('li') && !!content.querySelector(':scope > p,:scope > ul,:scope > ol,:scope > blockquote,:scope > .pm-task-body');
            if (!node.matches('.pm-source') && !/^(avoid|avoid-page)$/.test(contentCSS.breakInside) && container) {
                var firstBlock = blocks.length;
                Array.from(content.children).forEach(function (child) {
                    var before = position ? position(child.parentNode, Array.prototype.indexOf.call(child.parentNode.childNodes, child)) : 0;
                    emit(child, before);
                });
                if (blocks.length > firstBlock) {
                    blocks[firstBlock].breakBefore = blocks[firstBlock].breakBefore || /^(page|always)$/.test(contentCSS.breakBefore);
                    var lastBlock = blocks[blocks.length - 1];
                    lastBlock.keepNext = lastBlock.keepNext || /^(avoid|avoid-page)$/.test(contentCSS.breakAfter);
                    lastBlock.breakAfter = lastBlock.breakAfter || /^(page|always)$/.test(contentCSS.breakAfter);
                }
                return;
            }
            var gapNode = node.parentElement && node.parentElement.matches('li') && node.parentElement.firstElementChild === node ? node.parentElement : node;
            if (gapNode !== node && position) suppliedPosition = position(gapNode.parentNode, Array.prototype.indexOf.call(gapNode.parentNode.childNodes, gapNode));
            var block = { node: gapNode, pos: suppliedPosition, top: clean(rect.top), row: node.tagName === 'TR', columns: node.cells ? node.cells.length : 0,
                height: clean(rect.bottom) - clean(rect.top) + Math.max(0, parseFloat(css.marginBottom) || 0),
                references:node.matches('aside.doc-footnote')?[]:window.yavcPageFootnotes?window.yavcPageFootnotes.references(node):[],
                keepNext:/^(avoid|avoid-page)$/.test(css.breakAfter)||/^(avoid|avoid-page)$/.test(contentCSS.breakAfter)||node.tagName==='TR'&&!!node.querySelector('th'),
                breakAfter:/^(page|always)$/.test(css.breakAfter)||/^(page|always)$/.test(contentCSS.breakAfter),
                control:node.matches('.sc-page-break')||!!node.querySelector('.sc-page-break'),
                breakBefore: /^(page|always)$/.test(css.breakBefore)||/^(page|always)$/.test(contentCSS.breakBefore)||!!(previous&&(previous.control||previous.breakAfter)) };
            if(block.row&&!node.querySelector('th')){
                var table=node.closest('table'),headers=[];
                if(table)for(var row of Array.from(table.rows)){if(row===node||!row.querySelector('th')||row.classList.contains('doc-page-gap'))break;headers.push(row);}
                block.headerRows=headers;block.repeatHeader=headers.reduce(function(sum,row){return sum+row.getBoundingClientRect().height/scale;},0);
            }
            var bands=block.row&&window.yavcPageLines&&window.yavcPageLines.rowBands(content,top,scale,offsets,position);
            if(bands){
                bands.forEach(function(band,index){blocks.push(Object.assign({},block,band,{rowTop:block.top,references:[],pos:index?band.pos:block.pos,breakBefore:!index&&block.breakBefore,breakAfter:index===bands.length-1&&block.breakAfter,keepNext:index===0&&bands.length>1||index===bands.length-2||index===bands.length-1&&block.keepNext}));});
                content.querySelectorAll('.doc-footnote-ref[data-footnote-ref]').forEach(function(ref){if(ref.closest('.doc-page-gap'))return;var y=clean(ref.getBoundingClientRect().top),index=0;while(index+1<bands.length&&bands[index+1].top<=y)index++;blocks[blocks.length-bands.length+index].references.push(ref.getAttribute('data-footnote-ref'));});
                return;
            }
            var lines = window.yavcPageLines && window.yavcPageLines.measure(content, top, scale, offsets, position);
            if (lines) {
                var lineReferences=lines.map(function(){return [];});
                content.querySelectorAll('.doc-footnote-ref[data-footnote-ref]').forEach(function(ref){var rect=ref.getBoundingClientRect(),middle=clean((rect.top+rect.bottom)/2),index=0;while(index+1<lines.length&&lines[index+1].top<=middle)index++;lineReferences[index].push(ref.getAttribute('data-footnote-ref'));});
                lines.forEach(function (line, i) {
                    blocks.push(Object.assign({}, block, line, { references:lineReferences[i],anchor: i ? line.anchor : null, pos: i ? line.pos : block.pos,
                        breakBefore: i === 0 && block.breakBefore, breakAfter: i === lines.length - 1 && block.breakAfter,
                        keepNext: i < line.orphans - 1 || i >= lines.length - line.widows && i < lines.length - 1 || i === lines.length - 1 && block.keepNext }));
                });
            } else blocks.push(block);
        }
        Array.from(dom.children).forEach(function (node) {
            if(node.classList.contains('doc-page-gap'))return;
            if(node.hasAttribute('data-host-suggestion')){emit(node,position?position(dom,Array.prototype.indexOf.call(dom.childNodes,node)):0);return;}
            emit(node,positions?positions[sourceIndex]:0);sourceIndex++;
        });
        return blocks;
    }
    function spacer(height, inline, block, header) {
        var list = block && block.node.parentElement && block.node.parentElement.matches('ul,ol');
        var node = document.createElement(inline ? 'span' : block && block.row ? 'tr' : list ? 'li' : 'div'); node.className = 'doc-page-gap' + (inline ? ' doc-page-gap-inline' : ''); node.style.height = height + 'px';
        if (block && block.row) {
            var cell = document.createElement('td'); cell.colSpan = block.columns; node.append(cell);
            if(header&&block.headerRows&&block.headerRows.length){
                node.style.removeProperty('height');var space=document.createElement('div');space.style.height=height+'px';cell.append(space);
                var copy=document.createElement('table'),body=document.createElement('tbody');copy.className='doc-table-repeated-header';copy.append(body);
                block.headerRows.forEach(function(row){body.append(row.cloneNode(true));});
                copy.querySelectorAll('[id]').forEach(function(item){item.removeAttribute('id');});copy.querySelectorAll('a,button,input,select,textarea,[tabindex]').forEach(function(item){item.setAttribute('tabindex','-1');if('disabled' in item)item.disabled=true;});
                var original=block.headerRows[0],columns=document.createElement('colgroup'),host=original.closest('.doc-paginated'),zoom=host?Number(host.style.zoom)||1:1;
                Array.from(original.cells).forEach(function(item){var col=document.createElement('col');col.style.width=item.getBoundingClientRect().width/zoom+'px';columns.append(col);});
                copy.prepend(columns);copy.style.tableLayout='fixed';copy.style.width='100%';cell.append(copy);
            }
        }
        node.setAttribute('aria-hidden', 'true'); node.contentEditable = 'false'; return node;
    }
    function rowSpacer(gap,block,column){
        var node=spacer(gap.height,true,null);node.setAttribute('data-page-gap-key','body:'+gap.index);
        var exhausted=block.anchors[column].exhausted;
        if(gap.header&&block.headerRows.length){
            var space=document.createElement('span');space.style.display='block';space.style.height=gap.height+'px';node.style.height='auto';if(!exhausted)node.append(space);
            var header=document.createElement('span');header.className='doc-cell-repeated-header';header.style.height=gap.header+'px';
            block.headerRows.forEach(function(row){var cell=row.cells[column];if(!cell)return;var copy=cell.cloneNode(true);copy.removeAttribute('id');copy.querySelectorAll('[id]').forEach(function(item){item.removeAttribute('id');});copy.querySelectorAll('a,button,input,select,textarea,[tabindex]').forEach(function(item){item.setAttribute('tabindex','-1');if('disabled' in item)item.disabled=true;});header.append.apply(header,Array.from(copy.childNodes));});node.append(header);
        }
        if(exhausted){
            node.style.position='absolute';node.style.top=gap.rowHeaderTop+'px';node.style.left='0';node.style.width='100%';node.style.height=(gap.header||0)+'px';
        }return node;
    }
    function noteGaps(layout,blocks){
        var first=new Map(),added=new Map(),result=[];
        (layout.footnotes||[]).forEach(function(piece){
            if(!first.has(piece.label)){first.set(piece.label,piece);added.set(piece.label,0);return;}
            var note=blocks.footnotes.find(function(item){return item.label===piece.label;}),unit=note.units[piece.from];if(!unit||note.sourceOnly)return;
            var height=piece.top-first.get(piece.label).top-note.prefix-(unit.top-note.units[0].top)-added.get(piece.label);
            height=Math.max(0,height);added.set(piece.label,added.get(piece.label)+height);result.push({block:unit,height:height,key:piece.label+':'+piece.part});
        });return result;
    }
    function noteFragment(piece,blocks){
        var note=blocks.footnotes.find(function(item){return item.label===piece.label;}),copy=note.node.cloneNode(false),body=note.body.cloneNode(false),label=note.node.querySelector(':scope > .doc-footnote-label');
        if(label)copy.append(label.cloneNode(true));
        body.append(window.yavcPageLines.fragment(note.body,note.units,piece.from,piece.end));copy.append(body);
        if(piece.part){copy.style.paddingTop='0';copy.style.borderTop='0';copy.setAttribute('aria-label','Footnote '+piece.number+' continued');}
        return copy;
    }
    function preview(o) {
        var alive = true, raf = 0, signature = '';
        function schedule() {
            if (!alive || raf) { return; }
            raf = requestAnimationFrame(function () {
                raf = 0; var page = o.settings();
                if (!alive || !page || !o.flow.isConnected || !o.flow.getBoundingClientRect().width) { return; }
                try {
                    // Preview source wrappers do not participate in physical layout.
                    Array.from(o.flow.querySelectorAll('.dinah-src[data-start]')).reverse().forEach(function (node) { node.replaceWith.apply(node, Array.from(node.childNodes)); });
                    var blocks = blocksOf(o.flow), layout = plan(blocks, page), next = JSON.stringify([page, layout.gaps, layout.count, layout.oversized,layout.footnotes,layout.problem]);
                    var d = surface(o.host, page); o.host.style.minHeight = (layout.count * (d.height + GAP) - GAP) + 'px'; o.host.setAttribute('data-page-count', layout.count);
                    o.changed(layout);
                    if (next !== signature) {
                        o.flow.querySelectorAll('.doc-page-gap').forEach(function (node) { node.remove(); }); o.flow.normalize();
                        blocks = blocksOf(o.flow); layout = plan(blocks, page);
                        if (window.yavcDocumentSettings) { window.yavcDocumentSettings.decorate(o.host, page, layout.count); }
                        signature = JSON.stringify([page, layout.gaps, layout.count, layout.oversized,layout.footnotes,layout.problem]);
                        layout.gaps.slice().reverse().forEach(function (gap) { var block = blocks[gap.index];if(block.rowNode&&block.band){block.anchors.forEach(function(anchor,column){if(anchor)window.yavcPageLines.insert(o.flow,anchor,rowSpacer(gap,block,column));});}else window.yavcPageLines.insert(o.flow, block, spacer(gap.height, !!block.anchor, block, gap.header)); });
                        (layout.footnotes||[]).filter(function(note){return !note.part&&!note.sourceOnly;}).forEach(function(note){var node=blocks.footnotes.find(function(item){return item.label===note.label;}).node;node.classList.add('doc-footnote-paged');node.style.top=(note.top-(note.originTop||0))+'px';node.style.left=-(note.originLeft||0)+'px';node.style.width=note.width+'px';});
                        noteGaps(layout,blocks).reverse().forEach(function(gap){window.yavcPageLines.insert(o.flow,gap.block,spacer(gap.height,!!gap.block.anchor,gap.block));});
                    }
                } catch (err) { o.problem(err.message); }
            });
        }
        var ro = new ResizeObserver(schedule); ro.observe(o.flow);
        o.flow.addEventListener('load', schedule, true); document.fonts.addEventListener('loadingdone', schedule); schedule();
        return { refresh: function () { signature = ''; schedule(); }, destroy: function () { alive = false; cancelAnimationFrame(raf); ro.disconnect(); o.flow.removeEventListener('load', schedule, true); document.fonts.removeEventListener('loadingdone', schedule); } };
    }
    function plugin(PM, o) {
        var key = new PM.PluginKey('document-pages'), signature = '', raf = 0, alive = true,seenDoc=null,tableWidths=new WeakMap(),gapIds=new WeakMap(),gapSerial=0;
        function gapIdentity(block){var node=block.rowNode||block.node;if(!gapIds.has(node))gapIds.set(node,++gapSerial);return gapIds.get(node);}
        function headerIdentity(block){return (block.headerRows||[]).map(function(row){return row.outerHTML;}).join('');}
        function tableDecorations(v,scale){
            var decorations=[];
            v.dom.querySelectorAll('table').forEach(function(table){
                if(table.closest('.doc-page-gap')||!table.rows.length)return;
                var values=tableWidths.get(table);
                if(!values){values={width:table.getBoundingClientRect().width/scale,cells:Array.from(table.rows[0].cells).map(function(cell){return cell.getBoundingClientRect().width/scale;})};tableWidths.set(table,values);}
                function range(node){var parent=node.parentNode,index=Array.prototype.indexOf.call(parent.childNodes,node);return [v.posAtDOM(parent,index),v.posAtDOM(parent,index+1)];}
                var span=range(table);if(span[1]>span[0])decorations.push(PM.Decoration.node(span[0],span[1],{style:'table-layout:fixed;width:'+values.width+'px;'}));
                Array.from(table.rows[0].cells).forEach(function(cell,index){var span=range(cell);if(span[1]>span[0])decorations.push(PM.Decoration.node(span[0],span[1],{style:'width:'+values.cells[index]+'px;'}));});
            });return decorations;
        }
        function schedule(v) {
            if (raf || !alive) { return; }
            raf = requestAnimationFrame(function () {
                raf = 0; if (!alive || v.isDestroyed) { return; }
                var page = o.pagination && o.pagination();
                if (!page || !v.dom.isConnected) {
                    if (signature) { signature = ''; v.dispatch(v.state.tr.setMeta(key, PM.DecorationSet.empty).setMeta('addToHistory', false)); }
                    return;
                }
                if (!v.dom.getClientRects().length || v.dom.getBoundingClientRect().width === 0) { return; }
                try {
                    // Keep mapped page furniture visible until its replacement
                    // has been measured. Clearing it paints a collapsed page.
                    if(seenDoc!==v.state.doc){seenDoc=v.state.doc;tableWidths=new WeakMap();signature='';}
                    var d = surface(o.page, page), positions = []; v.state.doc.forEach(function (n, pos) { positions.push(pos); });
                    var blocks = blocksOf(v.dom, positions, function (node, offset) { return v.posAtDOM(node, offset); }), layout = plan(blocks, page);
                    var next = JSON.stringify([page, layout.gaps, layout.count, layout.oversized,layout.footnotes,layout.problem]);
                    o.page.style.minHeight = (layout.count * (d.height + GAP) - GAP) + 'px';
                    o.page.setAttribute('data-page-count', layout.count);
                    if (o.paginated) { o.paginated(layout); }
                    if (next !== signature) {
                        signature = next;
                        if (window.yavcDocumentSettings) { window.yavcDocumentSettings.decorate(o.page, page, layout.count); }
                        var scale=Number(o.page.style.zoom)||1,widgets=tableDecorations(v,scale);layout.gaps.forEach(function(gap){var block=blocks[gap.index];
                            var identity=gapIdentity(block)+':'+gap.index+':'+gap.height+':'+gap.header+':'+(gap.header?headerIdentity(block):'');
                            if(block.rowNode&&block.band){block.anchors.forEach(function(anchor,column){if(anchor)widgets.push(PM.Decoration.widget(anchor.pos,function(){return rowSpacer(gap,block,column);},{side:gap.index,key:'cell:'+identity+':'+column+':'+gap.rowHeaderTop+':'+!!anchor.exhausted,ignoreSelection:true}));});}
                            else widgets.push(PM.Decoration.widget(gap.pos,function(){return spacer(gap.height,!!block.anchor,block,gap.header);},{side:-1,key:'gap:'+identity+':'+!!block.anchor,ignoreSelection:true}));
                        });
                        (layout.footnotes||[]).filter(function(note){return !note.part&&!note.sourceOnly;}).forEach(function(note){widgets.push(PM.Decoration.node(note.pos,note.to,{class:'doc-footnote-paged',style:'top:'+(note.top-(note.originTop||0))+'px;left:'+(-(note.originLeft||0))+'px;width:'+note.width+'px;'}));});
                        noteGaps(layout,blocks).forEach(function(gap){widgets.push(PM.Decoration.widget(gap.block.pos,function(){return spacer(gap.height,!!gap.block.anchor,gap.block);},{side:-1,key:'note:'+gap.key+':'+gap.height,ignoreSelection:true}));});
                        v.dispatch(v.state.tr.setMeta(key, PM.DecorationSet.create(v.state.doc, widgets)).setMeta('addToHistory', false));
                    }
                } catch (err) { if (o.paginationProblem) { o.paginationProblem(err.message); } }
            });
        }
        return new PM.Plugin({ key: key,
            state: { init: function () { return PM.DecorationSet.empty; }, apply: function (tr, value) { return tr.getMeta(key) || value.map(tr.mapping, tr.doc); } },
            props: { decorations: function (state) { return key.getState(state); } },
            view: function (v) {
                alive = true; signature = ''; var ro = new ResizeObserver(function () { schedule(v); }); ro.observe(v.dom);
                function loaded() { seenDoc=null;schedule(v); }
                v.dom.addEventListener('load', loaded, true); document.fonts.addEventListener('loadingdone', loaded); schedule(v);
                return { update: function () { schedule(v); }, destroy: function () { alive = false; cancelAnimationFrame(raf); raf = 0; ro.disconnect(); v.dom.removeEventListener('load', loaded, true); document.fonts.removeEventListener('loadingdone', loaded); } };
            }
        });
    }
    window.yavcDocumentPages = { paper: '--doc-size:11pt;--doc-ink:#202124;--doc-muted:#555555;--doc-accent:#2355a0;--doc-page:#ffffff;--doc-rule:#cfd4da;--doc-code:#e4e7eb;--doc-warning:#b86e1c;--doc-danger:#b42318;--doc-positive:#2e7d4f;', lookup: paper, length: length, settings: settings, properties: properties, dimensions: dimensions, surface: surface, plan: plan, blocks: blocksOf, spacer: spacer,noteFragment:noteFragment,preview: preview, plugin: plugin };
})();
