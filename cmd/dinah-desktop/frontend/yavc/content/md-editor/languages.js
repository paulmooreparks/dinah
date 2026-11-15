/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* The languages the editor can colour (docs/language-packs-design.md, decisions 1 to 3 and DESIGN.md decision 188).
   This table is the one list of language identifiers, file extensions and Markdown fence names. A grammar is a script
   built in this repository (apps/md-editor/language-service/build.mjs) that records its CodeMirror legacy mode as
   window.yavcCodeModes[id]; it is loaded the first time a file or a code block of its language appears, from the package,
   and nothing comes from a document or a host. "editor" says that the Text view colours the language, and "page" that the
   rendered page does (none does yet, which is separate work). */
(function () {
    'use strict';
    var SELF = document.currentScript.src, BASE = new URL('.', SELF).href, STAMP = SELF.indexOf('?') >= 0 ? SELF.slice(SELF.indexOf('?')) : '';
    var TABLE = [
        { id: 'go', name: 'Go', extensions: ['go'], names: [], fences: ['go', 'golang'], grammar: 'vendor/codemirror-6.43/lang/go.js', editor: true, page: false }
    ];
    var byId = {}, byExtension = {}, byName = {}, byFence = {};
    TABLE.forEach(function (row) {
        byId[row.id] = row;
        row.extensions.forEach(function (e) { byExtension[e] = row.id; });
        row.names.forEach(function (n) { byName[n.toLowerCase()] = row.id; });
        row.fences.forEach(function (f) { byFence[f] = row.id; });
    });
    var modes = {}, support = {};

    /* The grammar's mode, from its script, which is fetched once. A script that cannot be loaded leaves the language plain. */
    function mode(id) {
        var row = byId[id];
        if (!row || !row.editor) { return Promise.reject(new Error('There is no grammar for ' + id + '.')); }
        if (!modes[id]) {
            modes[id] = window.yavcUtil.loadScript(new URL(row.grammar, BASE).href + STAMP).then(function () {
                var m = window.yavcCodeModes && window.yavcCodeModes[id];
                if (!m) { throw new Error('The grammar for ' + id + ' did not register.'); }
                return m;
            }).catch(function (error) { delete modes[id]; throw error; });
        }
        return modes[id];
    }
    /* A CodeMirror language support for the identifier, made once from the editor's own CodeMirror. */
    function supportOf(CM, id) {
        if (!support[id]) { support[id] = mode(id).then(function (m) { return new CM.LanguageSupport(CM.StreamLanguage.define(m)); }); }
        return support[id];
    }
    window.yavcLanguages = {
        table: TABLE.map(function (row) { return Object.assign({}, row); }),
        /* The identifier of the language of a file, by its extension or its whole name, or null. */
        byPath: function (path) {
            var base = String(path || '').split(/[\\/]/).pop().toLowerCase(), dot = base.lastIndexOf('.');
            if (byName[base]) { return byName[base]; }
            return dot > 0 && byExtension[base.slice(dot + 1)] || null;
        },
        /* The identifier of the language a Markdown fence names, or null. The first word of the info string is the name. */
        byFence: function (info) { return byFence[String(info || '').trim().split(/\s+/)[0].toLowerCase()] || null; },
        name: function (id) { return byId[id] ? byId[id].name : null; },
        has: function (id) { return !!(byId[id] && byId[id].editor); },
        /* A promise of the CodeMirror extension that colours the language. */
        support: supportOf,
        /* The descriptions Markdown's codeLanguages option takes, so that a fenced block is coloured when its grammar has loaded. */
        descriptions: function (CM) {
            return TABLE.filter(function (row) { return row.editor; }).map(function (row) {
                return CM.LanguageDescription.of({ name: row.name, alias: row.fences, load: function () { return supportOf(CM, row.id); } });
            });
        }
    };
}());
