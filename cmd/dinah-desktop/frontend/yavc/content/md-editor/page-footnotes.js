/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Page-owned note reservations. References and definitions remain source nodes. */
(function () {
    'use strict';
    function create(blocks, dimensions) {
        var definitions = blocks.footnotes || [], notes = new Map(definitions.map(function (note) { return [note.label, note]; })), placed = new Set(), pages = [], page = [], reserved = 0, problem = '',pending=[];
        if(definitions.length>256)throw new Error('Document view supports at most 256 footnote definitions.');
        function pieces(note,cap){
            if(!note.units||!note.units.length||note.height<=cap)return [{note:note,from:0,to:note.units?note.units.length:0,height:note.height,part:0}];
            var result=[],units=note.units,from=0;
            while(from<units.length){
                var to=from,height=0;
                while(to<units.length){
                    var end=to;while(end+1<units.length&&units[end].keepNext)end++;
                    var next=units[end].top+units[end].height-units[from].top+(from===0?note.prefix||0:0)+(end===units.length-1?note.tail||0:0);
                    if(next>cap+.5){if(to===from){end=to;next=units[end].height+(from===0?note.prefix||0:0);if(next>dimensions.content-8.5){problem='A footnote contains an indivisible block taller than its continuation area.';return [{note:note,from:0,to:units.length,height:note.height,part:0}];}}else break;}
                    height=next;to=end+1;
                }
                result.push({note:note,from:from,to:to,height:height,part:result.length});from=to;
            }
            return result;
        }
        function proposal(references,available) {
            var fresh = [], visited = new Set(),future=[],discovered=[];
            function visit(label) {
                if (visited.has(label) || placed.has(label)) return;
                visited.add(label);
                var note = notes.get(label);
                if (!note) { problem = 'A footnote reference has no definition. Restore its definition or remove the reference.'; return; }
                discovered.push(note);(note.references || []).forEach(visit);
            }
            references.forEach(visit);
            var budget=available==null?dimensions.content:Math.max(0,available),cap=Math.max(0,Math.min(dimensions.content/2,(budget-reserved)/Math.max(1,discovered.length)-8));
            discovered.forEach(function(note){var parts=pieces(note,cap);fresh.push(parts[0]);future=future.concat(parts.slice(1));});
            return { notes: fresh,future:future,height: fresh.reduce(function (sum, note) { return sum + note.height + 8; }, reserved) };
        }
        function commit(value) { value.notes.forEach(function (piece) { placed.add(piece.note.label); page.push(piece); });pending=pending.concat(value.future||[]);reserved = value.height; }
        function next(available) {
            pages.push(page);page=[];reserved=0;
            var budget=available==null?dimensions.content/2+8:Math.min(dimensions.content/2+8,Math.max(0,available));
            while(pending.length&&reserved+pending[0].height+8<=budget+.5){var piece=pending.shift();page.push(piece);reserved+=piece.height+8;}
            if(available==null&&pending.length&&!page.length){var piece=pending.shift();page.push(piece);reserved=piece.height+8;}
        }
        function finish(){while(pending.length){if(pages.length>=999)throw new Error('Document view is limited to 1,000 pages.');next();}return pages.length+1;}
        function placements() {
            var result = [];
            pages.concat([page]).forEach(function (items, number) {
                var top = number * (dimensions.height + 24) + dimensions.content - items.reduce(function (sum, note) { return sum + note.height + 8; }, 0);
                items.forEach(function (piece) { var note=piece.note;top += 8; result.push({ label: note.label, number: note.number, pos: note.pos, to: note.to, page: number + 1, top: top, height: piece.height,part:piece.part,from:piece.from,end:piece.to,sourceOnly:note.sourceOnly,originTop:note.originTop,originLeft:note.originLeft,width:note.width }); top += piece.height; });
            });
            return result;
        }
        return { proposal: proposal, commit: commit, next: next,finish:finish,placements: placements, problem: function () { return problem; } };
    }
    function references(node) { return Array.from(node.querySelectorAll('.doc-footnote-ref[data-footnote-ref]')).filter(function(ref){return !ref.closest('.doc-page-gap');}).map(function (ref) { return ref.getAttribute('data-footnote-ref'); }); }
    window.yavcPageFootnotes = Object.freeze({ create: create, references: references });
})();
