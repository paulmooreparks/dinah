/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Measured paragraph boundaries. DOM ranges and widget anchors stay outside source. */
(function () {
    'use strict';
    var LIMIT = 20000;
    function blocked(element,position){
        return Array.from(element.querySelectorAll('img,svg,iframe,[contenteditable="false"]')).some(function(node){
            if(node.closest('.doc-page-gap')||node.matches('.doc-footnote-ref'))return false;
            // View decorations have no source extent. Use the public source
            // mapping instead of depending on an editor's helper DOM classes.
            if(position){try{var parent=node.parentNode,at=Array.prototype.indexOf.call(parent.childNodes,node);if(position(parent,at)===position(parent,at+1))return false;}catch(err){return true;}}
            return true;
        });
    }
    function lines(element, origin, scale, gaps, position, single) {
        /* A host's flowing block is drawn inside a source block, which the page does not enter, but the page breaks its own lines and rows (decision 173). */
        if (!element.matches('p,li,td,th') || element.closest('.pm-source') && !element.closest('[data-host-flow]') || blocked(element,position)) return null;
        var css = getComputedStyle(element);
        if (/^(avoid|avoid-page)$/.test(css.breakInside) || /^(pre|nowrap)$/.test(css.whiteSpace)) return null;
        function clean(y) { return (y - origin) / scale - gaps.reduce(function (sum, gap) { return sum + (gap.bottom <= y + .5 ? gap.height : 0); }, 0); }
        var walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT), text, rows = [], range = document.createRange();
        while ((text = walker.nextNode())) {
            if(text.parentElement&&text.parentElement.closest('.doc-page-gap'))continue;
            var atom = text.nodeType === Node.ELEMENT_NODE && text.matches('.doc-footnote-ref');
            if (!atom && (text.nodeType !== Node.TEXT_NODE || !text.length || text.parentElement.closest('.doc-page-gap,.doc-footnote-ref'))) continue;
            if (atom) range.selectNode(text); else range.selectNodeContents(text);
            var rects = Array.from(range.getClientRects()).filter(function (r) { return r.width > 0 && r.height > 0; });
            for (var rect of rects) {
                var top = clean(rect.top), bottom = clean(rect.bottom), previous = rows[rows.length - 1];
                if (previous && top < previous.bottom - .5) { previous.bottom = Math.max(previous.bottom, bottom); continue; }
                // Find the first character on this measured line. Character
                // ranges use UTF-16 DOM offsets, matching ProseMirror positions.
                var low = 0, high = atom ? 0 : text.length;
                while (low < high) {
                    var mid = Math.floor((low + high) / 2);
                    range.setStart(text, mid); range.setEnd(text, mid + 1);
                    if (range.getBoundingClientRect().top < rect.top - .5) low = mid + 1; else high = mid;
                }
                if (!atom && low === text.length) continue;
                // A range must never start between a Unicode surrogate pair.
                if (!atom && low && /[\uDC00-\uDFFF]/.test(text.data[low])) low--;
                var anchor = atom ? text.parentNode : text, at = atom ? Array.prototype.indexOf.call(anchor.childNodes, text) : low;
                rows.push({ anchor: anchor, offset: at, top: top, bottom: bottom, pos: position ? position(anchor, at) : 0 });
                if (rows.length > LIMIT) return null;
            }
        }
        if (rows.length < (single?1:2)) return null;
        var box = element.getBoundingClientRect(), end = clean(box.bottom) + Math.max(0, parseFloat(css.marginBottom) || 0);
        rows.forEach(function (row, i) {
            row.top = i ? (rows[i - 1].bottom + row.top) / 2 : clean(box.top);
            row.height = Math.max(0, (i + 1 < rows.length ? (row.bottom + rows[i + 1].top) / 2 : end) - row.top);
            row.line = i; row.lines = rows.length;
            row.orphans = Math.max(1, Math.min(10, parseInt(css.orphans, 10) || 2));
            row.widows = Math.max(1, Math.min(10, parseInt(css.widows, 10) || 2));
        });
        return rows;
    }
    function rowBands(row,origin,scale,gaps,position){
        if(row.cells.length>64||Array.from(row.cells).some(function(cell){return cell.rowSpan!==1||cell.colSpan!==1||cell.querySelector('table,ul,ol,pre')||blocked(cell,position);}))return null;
        function clean(y){return (y-origin)/scale-gaps.reduce(function(sum,gap){return sum+(gap.bottom<=y+.5?gap.height:0);},0);}
        var cells=Array.from(row.cells),streams=cells.map(function(cell){return lines(cell,origin,scale,gaps,position,true)||[];}),candidates=[];
        if(streams.some(function(stream,index){return !stream.length&&cells[index].textContent.trim();}))return null;
        streams.forEach(function(stream){stream.slice(1).forEach(function(line){candidates.push(line.top);});});
        candidates.sort(function(a,b){return a-b;});var cuts=[],last=-Infinity;
        candidates.forEach(function(cut){
            if(cut-last<1)return;var anchors=[],safe=true;
            streams.forEach(function(stream,index){
                var next=stream.findIndex(function(line){return line.top>=cut-.5;}),previous=next<0?stream[stream.length-1]:stream[next-1];
                if(previous&&previous.bottom>cut+.5||next===0){safe=false;return;}
                if(next<0){var cell=cells[index];anchors.push({anchor:cell,offset:cell.childNodes.length,pos:position?position(cell,cell.childNodes.length):0,exhausted:true});return;}
                var line=stream[next];anchors.push({anchor:line.anchor,offset:line.offset,pos:line.pos});
            });
            if(safe&&anchors.some(function(anchor){return anchor&&!anchor.exhausted;})){cuts.push({top:cut,anchors:anchors});last=cut;}
        });
        if(!cuts.length)return null;
        var rect=row.getBoundingClientRect(),top=clean(rect.top),bottom=clean(rect.bottom),bands=[{top:top,anchors:cells.map(function(cell){return {anchor:cell,offset:0,pos:position?position(cell,0):0};})}].concat(cuts);
        bands.forEach(function(band,index){band.height=(index+1<bands.length?bands[index+1].top:bottom)-band.top;band.band=index;band.bands=bands;band.rowNode=row;band.pos=(band.anchors.find(Boolean)||{}).pos||0;});return bands;
    }
    function insert(flow, block, gap) {
        if (!block.anchor) { block.node.parentNode.insertBefore(gap, block.node); return; }
        /* An anchor measured among the gaps of an earlier layout, which a host's drawing holds as elements (pages.js, applyFlowGaps), counts them among its
           children, and they are taken out before the new ones go in; an anchor at the end of an element is at its end all the same. */
        var range = document.createRange(); range.setStart(block.anchor, block.anchor.nodeType === Node.ELEMENT_NODE ? Math.min(block.offset, block.anchor.childNodes.length) : block.offset); range.collapse(true); range.insertNode(gap);
    }
    function fragment(flow, blocks, from, to) {
        var range = document.createRange(), first = blocks[from], last = blocks[to];
        if(!first)return document.createDocumentFragment();
        if(first.rowNode)range.setStartBefore(first.rowNode);
        else if (first.anchor) range.setStart(first.anchor, first.offset); else range.setStartBefore(first.node);
        if (!last) range.setEnd(flow, flow.childNodes.length);
        else if(last.rowNode&&last.band)range.setEndAfter(last.rowNode);
        else if (last.anchor) range.setEnd(last.anchor, last.offset); else range.setEndBefore(last.node);
        var ancestor = range.commonAncestorContainer, output = range.cloneContents();
        if (ancestor.nodeType === Node.TEXT_NODE) ancestor = ancestor.parentElement;
        while (ancestor && ancestor !== flow) {
            var shell = ancestor.cloneNode(false); shell.removeAttribute('id'); shell.append(output); output = shell; ancestor = ancestor.parentElement;
        }
        output.querySelectorAll('aside.doc-footnote[data-footnote-number]').forEach(function(note){if(Number(note.getAttribute('data-footnote-number'))>0){var parent=note.parentElement;note.remove();while(parent&&parent!==output&&parent.matches('blockquote,div,section,aside')&&!parent.textContent.trim()&&!parent.querySelector('img,svg,hr,br,input')){var next=parent.parentElement;parent.remove();parent=next;}}});
        // A continued ordered list retains its displayed item number.
        var start = range.startContainer.nodeType === Node.TEXT_NODE ? range.startContainer.parentElement : range.startContainer.childNodes[range.startOffset] || range.startContainer;
        var lists = [], continuedItems = [], branch = start;
        while (branch && branch !== flow) {
            if (branch.tagName === 'LI' && !(range.startContainer === branch.parentNode && branch.parentNode.childNodes[range.startOffset] === branch)) continuedItems.unshift(branch);
            if (branch.parentElement && branch.parentElement.tagName === 'OL') {
                var list = branch.parentElement, items = Array.from(list.children).filter(function (item) { return item.tagName === 'LI' && !item.classList.contains('doc-page-gap'); });
                lists.unshift((Number(list.getAttribute('start')) || 1) + Math.max(0, items.indexOf(branch)));
            }
            branch = branch.parentElement;
        }
        var continued = output.nodeType === Node.ELEMENT_NODE && output.tagName === 'OL' ? [output] : [];
        continued = continued.concat(Array.from(output.querySelectorAll('ol')));
        lists.forEach(function (number, i) { if (continued[i]) continued[i].setAttribute('start', number); });
        var itemClones = output.nodeType === Node.ELEMENT_NODE && output.tagName === 'LI' ? [output] : [];
        itemClones = itemClones.concat(Array.from(output.querySelectorAll('li')));
        continuedItems.forEach(function (_, i) { if (itemClones[i]) itemClones[i].classList.add('doc-list-continuation'); });
        function openEdge(container, side) {
            var tags = new Set(), source = container.nodeType === Node.TEXT_NODE ? container.parentElement : container;
            while (source && source !== flow) { tags.add(source.tagName); source = source.parentElement; }
            var edge = output;
            while (edge[side === 'start' ? 'firstChild' : 'lastChild']) edge = edge[side === 'start' ? 'firstChild' : 'lastChild'];
            while (edge) {
                if (edge.nodeType === Node.ELEMENT_NODE && tags.has(edge.tagName)) edge.classList.add('doc-fragment-open-' + side);
                if (edge === output) break;
                edge = edge.parentNode;
            }
        }
        openEdge(range.startContainer, 'start'); openEdge(range.endContainer, 'end');
        // A table fragment must retain the measured column widths. Letting a
        // shorter page choose new widths can rewrap cells after pagination.
        var sourceTables = Array.from(flow.querySelectorAll('table')).filter(function (table) { return range.intersectsNode(table); });
        var tableCopies = output.nodeType === Node.ELEMENT_NODE && output.tagName === 'TABLE' ? [output] : [];
        tableCopies = tableCopies.concat(Array.from(output.querySelectorAll('table')));
        tableCopies.forEach(function (table, i) {
            var original = sourceTables[i], row = original && original.rows[0];
            if (!row) return;
            var originals=Array.from(original.rows).filter(function(item){return range.intersectsNode(item);}),copies=Array.from(table.rows);
            originals.forEach(function(item,rowIndex){
                var begin=first.rowNode===item?first:null,end=last&&last.rowNode===item?last:null,copy=copies[rowIndex];if(!copy||!begin&&!end)return;
                Array.from(item.cells).forEach(function(cell,column){
                    var selected=document.createRange(),start=begin&&begin.anchors[column],stop=end&&end.anchors[column];
                    if(begin&&!start){copy.cells[column].replaceChildren();return;}
                    if(start)selected.setStart(start.anchor,start.offset);else selected.setStart(cell,0);
                    if(end){if(stop)selected.setEnd(stop.anchor,stop.offset);else selected.setEnd(cell,cell.childNodes.length);}else selected.setEnd(cell,cell.childNodes.length);
                    copy.cells[column].replaceChildren(selected.cloneContents());
                });
            });
            if(!table.querySelector('th')){
                var head=document.createElement('thead');
                for(var originalRow of Array.from(original.rows)){if(!originalRow.querySelector('th'))break;head.append(originalRow.cloneNode(true));}
                head.querySelectorAll('[id]').forEach(function(item){item.removeAttribute('id');});
                if(head.children.length){head.setAttribute('data-repeated-header','');table.prepend(head);}
            }
            var columns = document.createElement('colgroup'), host = flow.closest('.doc-paginated'), zoom = host ? Number(host.style.zoom) || 1 : 1;
            Array.from(row.cells).forEach(function (cell) { var column = document.createElement('col'); column.style.width = cell.getBoundingClientRect().width / zoom + 'px'; columns.append(column); });
            table.querySelectorAll(':scope > colgroup').forEach(function (group) { group.remove(); });
            table.prepend(columns); table.style.tableLayout = 'fixed'; table.style.width = original.getBoundingClientRect().width / zoom + 'px';
        });
        return output;
    }
    window.yavcPageLines = Object.freeze({ measure: lines,rowBands:rowBands,insert: insert, fragment: fragment });
})();
