/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Public CodeMirror state APIs with reversible original line endings. */
(function (global) {
    'use strict';
    var LIMIT = 10 * 1024 * 1024, DEPTH = 200, BUDGET=32*1024*1024;
    function normalize(raw) { return raw.replace(/\r\n?/g, '\n'); }
    function endings(raw) { return (raw.match(/\r\n|\r|\n/g) || []).map(function (value) { return value === '\r\n' ? 'R' : value === '\r' ? 'C' : 'L'; }).join(''); }
    function restore(text, sequence) {
        if (typeof text !== 'string' || text.length > LIMIT || typeof sequence !== 'string' || !/^[LRC]*$/.test(sequence) || sequence.length !== (text.match(/\n/g) || []).length) throw new Error('The recovered source line endings are invalid.');
        var at = 0;
        return text.replace(/\n/g, function () { return { L: '\n', R: '\r\n', C: '\r' }[sequence[at++]]; });
    }
    function changed(raw, changes) {
        var crlf=[],extra=0,at=-1;
        while((at=raw.indexOf('\r\n',at+1))>=0)crlf.push(at-extra++);
        function offset(position){var low=0,high=crlf.length;while(low<high){var mid=(low+high)>>>1;if(crlf[mid]<position)low=mid+1;else high=mid;}return position+low;}
        function local(position){
            var cr=raw.indexOf('\r',position),lf=raw.indexOf('\n',position),next=cr<0?lf:lf<0?cr:Math.min(cr,lf);
            if(next>=0)return raw[next]==='\r'?(raw[next+1]==='\n'?'\r\n':'\r'):'\n';
            var prior=Math.max(raw.lastIndexOf('\r',position-1),raw.lastIndexOf('\n',position-1));
            return prior<0?'\n':raw[prior]==='\r'?'\r':raw[prior-1]==='\r'?'\r\n':'\n';
        }
        var edits = [];
        changes.iterChanges(function (from, to, fromB, toB, inserted) {
            var start=offset(from);
            edits.push({ from: start, to: offset(to), text: inserted.toString().replace(/\n/g, local(start)) });
        });
        for (var i = edits.length - 1; i >= 0; i--) raw = raw.slice(0, edits[i].from) + edits[i].text + raw.slice(edits[i].to);
        if (raw.length > LIMIT * 2) throw new Error('The source exceeds its raw text limit.');
        return raw;
    }
    function create(CM) {
        var initial = CM.Facet.define({ combine: function (values) { return values.length ? values[values.length - 1] : ''; } }), replay = CM.StateEffect.define(),adoption=CM.StateEffect.define();
        function selected(selection, length) { selection.ranges.forEach(function (range) { if (range.from < 0 || range.to > length) throw new Error('The recovered history selection is invalid.'); }); return selection; }
        function encode(entry) { return { undo: entry.undo.toJSON(), redo: entry.redo.toJSON(), before: entry.before.toJSON(), after: entry.after.toJSON(), beforeEndings: entry.beforeEndings, afterEndings: entry.afterEndings }; }
        function decode(entry) {
            if (!entry || typeof entry !== 'object') throw new Error('The recovered history event is invalid.');
            return { undo: CM.ChangeSet.fromJSON(entry.undo), redo: CM.ChangeSet.fromJSON(entry.redo), before: CM.EditorSelection.fromJSON(entry.before), after: CM.EditorSelection.fromJSON(entry.after), beforeEndings: entry.beforeEndings, afterEndings: entry.afterEndings };
        }
        function cost(entry){var bytes=256+2*(entry.beforeEndings.length+entry.afterEndings.length)+32*(entry.before.ranges.length+entry.after.ranges.length);[entry.undo,entry.redo].forEach(function(change){change.iterChanges(function(a,b,c,d,inserted){bytes+=64+inserted.length*2;});});return bytes;}
        function retained(entries){return entries.reduce(function(sum,entry){return sum+entry.bytes;},0);}
        function validate(stack, doc, sequence, undo) {
            for (var i = stack.length - 1; i >= 0; i--) {
                var entry = stack[i], step = undo ? entry.undo : entry.redo, reverse = undo ? entry.redo : entry.undo, next = step.apply(doc), previousEndings = undo ? entry.beforeEndings : entry.afterEndings;
                if(next.length>LIMIT)throw new Error('The recovered history exceeds the source limit.');
                if ((undo ? entry.afterEndings : entry.beforeEndings) !== sequence || !reverse.apply(next).eq(doc)) throw new Error('The recovered history does not match the source.');
                selected(undo ? entry.after : entry.before, doc.length); selected(undo ? entry.before : entry.after, next.length);
                restore(next.toString(), previousEndings); doc = next; sequence = previousEndings;
            }
        }
        var field = CM.StateField.define({
            create: function (state) { var raw = state.facet(initial); if (normalize(raw) !== state.doc.toString()) raw = state.doc.toString(); return { raw: raw, done: [], undone: [], time: 0, group: false }; },
            update: function (value, tr) {
                var adopted=tr.effects.find(function(effect){return effect.is(adoption);});
                if(adopted){if(typeof adopted.value!=='string'||adopted.value.length>LIMIT*2||normalize(adopted.value)!==tr.newDoc.toString())throw new Error('The incoming source does not match its document.');return {raw:adopted.value,done:[],undone:[],time:0,group:false};}
                var effect = tr.effects.find(function (effect) { return effect.is(replay); });
                if (effect) return effect.value;
                if (!tr.docChanged) return tr.selection ? Object.assign({}, value, { group: false }) : value;
                if(tr.newDoc.eq(tr.startState.doc))return Object.assign({},value,{group:false});
                var raw = changed(value.raw, tr.changes);
                if (normalize(raw) !== tr.newDoc.toString()) throw new Error('The source line-ending map no longer matches its document.');
                if (tr.annotation(CM.Transaction.addToHistory) === false) return { raw: raw, done: [], undone: [], time: 0, group: false };
                var done = value.done.slice(), now = Date.now(), isolation = tr.annotation(CM.isolateHistory),group=tr.isUserEvent('input.type')?'input.type':tr.isUserEvent('input.visual')?'input.visual':tr.isUserEvent('delete.backward')?'delete.backward':tr.isUserEvent('delete.forward')?'delete.forward':false;
                var entry = { undo: tr.changes.invert(tr.startState.doc), redo: tr.changes, before: tr.startState.selection, after: tr.newSelection, beforeEndings: endings(value.raw), afterEndings: endings(raw) }, previous = done[done.length - 1];
                if (group && value.group===group && now - value.time < 500 && isolation !== 'before' && isolation !== 'full' && previous && previous.after.eq(tr.startState.selection)) {
                    entry.undo = entry.undo.compose(previous.undo); entry.redo = previous.redo.compose(entry.redo); entry.before = previous.before; entry.beforeEndings = previous.beforeEndings; done.pop();
                }
                entry.bytes=cost(entry);done.push(entry);var bytes=retained(done);while(done.length>DEPTH||bytes>BUDGET)bytes-=done.shift().bytes;
                return { raw: raw, done: done, undone: [], time: now, group: isolation !== 'after' && isolation !== 'full'&&group };
            },
            toJSON: function (value) { return { version: 1, endings: endings(value.raw), done: value.done.map(encode), undone: value.undone.map(encode) }; },
            fromJSON: function (json, state) {
                if (!json || json.version !== 1 || !Array.isArray(json.done) || !Array.isArray(json.undone) || json.done.length + json.undone.length > DEPTH) throw new Error('The recovered source history is invalid.');
                var raw = restore(state.doc.toString(), json.endings), done = json.done.map(decode), undone = json.undone.map(decode);
                done.concat(undone).forEach(function(entry){if(typeof entry.beforeEndings!=='string'||typeof entry.afterEndings!=='string')throw new Error('The recovered history line endings are invalid.');entry.bytes=cost(entry);});
                if(retained(done)+retained(undone)>BUDGET)throw new Error('The recovered history exceeds its memory budget.');
                validate(done, state.doc, json.endings, true); validate(undone, state.doc, json.endings, false);
                return { raw: raw, done: done, undone: undone, time: 0, group: false };
            }
        });
        function can(name, state) { var value = state.field(field, false); return !!value && (name === 'undo' ? value.done : value.undone).length > 0; }
        function run(name, view) {
            if (view.state.readOnly || !can(name, view.state)) return false;
            var value = view.state.field(field), done = value.done.slice(), undone = value.undone.slice(), undo = name === 'undo', entry = (undo ? done : undone).pop(), changes = undo ? entry.undo : entry.redo;
            (undo ? undone : done).push(entry);
            var next = { raw: restore(changes.apply(view.state.doc).toString(), undo ? entry.beforeEndings : entry.afterEndings), done: done, undone: undone, time: 0, group: false };
            view.dispatch({ changes: changes, selection: undo ? entry.before : entry.after, effects: replay.of(next), annotations: CM.Transaction.userEvent.of(name), scrollIntoView: true }); return true;
        }
        return Object.freeze({ field: field, extension: function (raw) { return [initial.of(raw || ''), field]; }, adopt:function(raw){return adoption.of(raw);},raw: function (state) { return state.field(field).raw; }, endings: function (state) { return endings(state.field(field).raw); }, retainedBytes:function(state){var value=state.field(field);return retained(value.done)+retained(value.undone);},budget:BUDGET,can: can, run: run });
    }
    var api = Object.freeze({ create: create, normalize: normalize, endings: endings, restore: restore, changed: changed });
    if (typeof module === 'object' && module.exports) module.exports = api; else global.yavcSourceHistory = api;
})(typeof window === 'undefined' ? null : window);
