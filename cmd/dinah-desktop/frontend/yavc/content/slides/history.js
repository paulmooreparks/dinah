/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* A deck owns one ordered history across its source, rich and structural edits. */
(function () {
    'use strict';
    function create(initial, options) {
        options = options || {};
        var limit = options.limit || 100, budget = options.bytes || 8 * 1024 * 1024;
        var current = Object.assign({}, initial), undo = [], redo = [], last = null;
        function bytes() { return undo.concat(redo).reduce(function (n, entry) { return n + entry.text.length * 2; }, 0); }
        function trim() {
            while (undo.length + redo.length > limit || bytes() > budget) {
                if (undo.length) undo.shift(); else if (redo.length) redo.shift(); else break;
            }
        }
        return Object.freeze({
            record: function (next, kind, time) {
                time = time == null ? Date.now() : time;
                if (next.text === current.text) { current = Object.assign({}, next); return false; }
                var merge = kind && /^(text|page):/.test(kind) && last && last.kind === kind && time - last.time < 750 && time >= last.time;
                if (!merge) undo.push(current);
                current = Object.assign({}, next); redo = []; last = {kind: kind, time: time}; trim(); return true;
            },
            boundary: function () { last = null; },
            can: function (direction) { return (direction === 'undo' ? undo : redo).length > 0; },
            take: function (direction) {
                var from = direction === 'undo' ? undo : redo, to = direction === 'undo' ? redo : undo;
                if (!from.length) return null;
                to.push(current); current = from.pop(); last = null; trim(); return Object.assign({}, current);
            }
        });
    }
    window.yavcDeckHistory = Object.freeze({create: create});
})();
