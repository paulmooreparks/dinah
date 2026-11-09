/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Colouring for D2 and Mermaid in CodeMirror (docs/diagrams-design.md,
   "What this means for the Editor"), for Diagrams and later the Editor.
   Each is a CodeMirror stream mode, which reads a line at a time and needs
   nothing from the engine, so the text is coloured before the engine has
   loaded and while the diagram has a mistake in it.

     yavcDiagramModes.d2(CM)        a StreamLanguage for D2
     yavcDiagramModes.mermaid(CM)   a StreamLanguage for Mermaid
     yavcDiagramModes.kindOf(path)  'd2', 'mermaid' or null, by the suffix

   The words each language reserves come from its own reference: D2's
   reserved keywords (d2graph's ReservedKeywords and StyleKeywords, D2
   0.7.2) and Mermaid's diagram types and statements. */
(function () {
    'use strict';
    if (window.yavcDiagramModes) { return; }

    function set(words) { var o = Object.create(null); words.split(/\s+/).forEach(function (w) { if (w) { o[w] = true; } }); return o; }

    var D2_KEYWORDS = set('label shape icon constraint tooltip link near width height top left direction class classes vars ' +
        'layers scenarios steps style grid-rows grid-columns grid-gap vertical-gap horizontal-gap source-arrowhead target-arrowhead ' +
        'filled d2-config d2-legend layout-engine theme-id dark-theme-id pad center sketch theme-overrides dark-theme-overrides ' +
        'suspend unsuspend import');
    var D2_STYLES = set('opacity stroke fill fill-pattern stroke-width stroke-dash border-radius font font-size font-color bold italic ' +
        'underline shadow 3d multiple double-border animated text-transform');
    var D2_ATOMS = set('true false null');

    function d2(CM) {
        return CM.StreamLanguage.define({
            name: 'd2',
            startState: function () { return { block: null, blockComment: false }; },
            copyState: function (s) { return { block: s.block, blockComment: s.blockComment }; },
            token: function (stream, state) {
                /* A block comment, """ to """. */
                if (state.blockComment) {
                    if (stream.skipTo('"""')) { stream.match('"""'); state.blockComment = false; } else { stream.skipToEnd(); }
                    return 'comment';
                }
                /* A block string, such as |md … | or |`js … `|, read to its
                   closing run, which mirrors the opening one. */
                if (state.block) {
                    if (stream.skipTo(state.block)) { stream.match(state.block); state.block = null; } else { stream.skipToEnd(); }
                    return 'string';
                }
                if (stream.eatSpace()) { return null; }
                if (stream.match('"""')) { state.blockComment = true; return 'comment'; }
                if (stream.peek() === '#') { stream.skipToEnd(); return 'comment'; }
                var pipes = stream.match(/^\|+(`+)?/);
                if (pipes) {
                    var open = pipes[0];
                    state.block = open.split('').reverse().join('');
                    stream.match(/^[A-Za-z0-9_+-]*/);
                    return 'string';
                }
                if (stream.match(/^"(?:[^"\\]|\\.)*"?/) || stream.match(/^'(?:[^'\\]|\\.)*'?/)) { return 'string'; }
                if (stream.match(/^\.\.\.@[^\s;{}]+/) || stream.match(/^@[^\s;{}]+/)) { return 'meta'; }
                if (stream.match(/^\$\{[^}]*\}?/)) { return 'meta'; }
                if (stream.match(/^<?(?:-+|=+|\.+)(?:>|\*)?/) && /[<>-]/.test(stream.current())) { return 'operator'; }
                if (stream.match(/^-?\d+(?:\.\d+)?\b/)) { return 'number'; }
                if (stream.match(/^[{}()\[\]]/)) { return 'bracket'; }
                if (stream.match(/^[:;.,*&!]/)) { return 'punctuation'; }
                var word = stream.match(/^[^\s:;.{}()\[\]"'#|<>,*&!$@]+(?:[ \t]+(?!->|<-|--)[^\s:;.{}()\[\]"'#|<>,*&!$@]+)*/);
                if (word) {
                    var w = word[0].trim();
                    if (D2_ATOMS[w]) { return 'atom'; }
                    var next = stream.string.charAt(stream.pos);
                    if ((next === ':' || next === '.' || next === ' ' || next === '') && D2_KEYWORDS[w]) { return 'keyword'; }
                    if (D2_STYLES[w] && /^\s*:/.test(stream.string.slice(stream.pos))) { return 'propertyName'; }
                    return 'variableName';
                }
                stream.next();
                return null;
            },
            languageData: { commentTokens: { line: '#' } }
        });
    }

    var MM_TYPES = set('graph flowchart sequenceDiagram classDiagram stateDiagram stateDiagram-v2 erDiagram gantt pie journey gitGraph ' +
        'mindmap timeline quadrantChart requirementDiagram xychart-beta sankey-beta block-beta C4Context');
    var MM_WORDS = set('subgraph end participant actor note left right of over loop alt else opt par and rect critical break activate deactivate ' +
        'autonumber title section dateFormat axisFormat excludes todayMarker class state direction TB TD BT RL LR click style classDef linkStyle ' +
        'commit branch checkout merge cherry-pick showData x-axis y-axis quadrant-1 quadrant-2 quadrant-3 quadrant-4 as');

    function mermaid(CM) {
        return CM.StreamLanguage.define({
            name: 'mermaid',
            startState: function () { return {}; },
            token: function (stream) {
                if (stream.eatSpace()) { return null; }
                if (stream.match('%%')) { stream.skipToEnd(); return 'comment'; }
                if (stream.match(/^"(?:[^"\\]|\\.)*"?/)) { return 'string'; }
                if (stream.match(/^(?:<<|[<x o])?(?:-{2,}|={2,}|-\.+-|-+>>|-+\)|-+x)(?:>>|>|x|o|\))?/) && /[-=]/.test(stream.current())) { return 'operator'; }
                if (stream.match(/^\|[^|]*\|/)) { return 'string'; }
                if (stream.match(/^-?\d+(?:\.\d+)?\b/)) { return 'number'; }
                if (stream.match(/^[\[\](){}]/)) { return 'bracket'; }
                if (stream.match(/^[:;,]/)) { return 'punctuation'; }
                var word = stream.match(/^[A-Za-z_][\w-]*/);
                if (word) {
                    if (MM_TYPES[word[0]]) { return 'keyword'; }
                    if (MM_WORDS[word[0]]) { return 'keyword'; }
                    return 'variableName';
                }
                stream.next();
                return null;
            },
            languageData: { commentTokens: { line: '%%' } }
        });
    }

    function kindOf(path) {
        var p = String(path || '').toLowerCase();
        if (/\.d2$/.test(p)) { return 'd2'; }
        if (/\.(mmd|mermaid)$/.test(p)) { return 'mermaid'; }
        return null;
    }

    window.yavcDiagramModes = Object.freeze({ d2: d2, mermaid: mermaid, kindOf: kindOf });
})();
