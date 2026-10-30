/* The Editor's text surface: loading CodeMirror 6 (vendor/codemirror-6.43)
   and its theme, set up as the Parks Computing editor sets it up; that
   folder's NOTICE.txt records what was taken. editor-applet.js reads them
   as window.dinahEditorCodeMirror. */
(function () {
  'use strict';

  var cmReady = null;
  function loadCodeMirror() {
    if (window.CM && window.CM.EditorView) return Promise.resolve(window.CM);
    if (!cmReady) {
      cmReady = new Promise(function (resolve, reject) {
        var script = document.createElement('script');
        script.src = 'vendor/codemirror-6.43/codemirror.js';
        script.onload = function () { resolve(window.CM); };
        script.onerror = function () { cmReady = null; reject(new Error('The text editor could not be loaded.')); };
        document.head.appendChild(script);
      });
    }
    return cmReady;
  }

  /* Colours from PUDL's tokens, as CSS variables, so the editor follows the
     theme with no code of its own (from the Parks Computing editor). */
  function makeTheme(CM) {
    var tint = function (pct) { return 'color-mix(in srgb, var(--accent) ' + pct + '%, transparent)'; };
    return [
      CM.EditorView.theme({
        '&': { color: 'var(--text)', backgroundColor: 'var(--input-bg)', height: '100%', fontSize: 'var(--editor-size, 14px)' },
        '.cm-scroller': { fontFamily: 'var(--editor-font, var(--mono))', lineHeight: '1.5' },
        '.cm-content': { caretColor: 'var(--accent)' },
        '.cm-gutters': { backgroundColor: 'var(--surface)', color: 'var(--text-muted)', borderRight: '1px solid var(--border)' },
        '.cm-activeLine': { backgroundColor: tint(6) },
        '.cm-activeLineGutter': { backgroundColor: tint(12), color: 'var(--text)' },
        '&.cm-focused .cm-cursor': { borderLeftColor: 'var(--accent)' },
        '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': { backgroundColor: tint(26) + ' !important' },
        '.cm-selectionMatch': { backgroundColor: tint(14) },
        '.cm-matchingBracket, &.cm-focused .cm-matchingBracket': { backgroundColor: tint(22), outline: '1px solid ' + tint(50) },
        '.cm-searchMatch': { backgroundColor: 'color-mix(in srgb, var(--warn) 30%, transparent)' },
        '.cm-searchMatch.cm-searchMatch-selected': { backgroundColor: 'color-mix(in srgb, var(--warn) 55%, transparent)' },
        '.cm-panels': { backgroundColor: 'var(--surface)', color: 'var(--text)', borderColor: 'var(--border)' },
        '.cm-panels input, .cm-panels button': { font: 'inherit', fontSize: '13px' },
        '.cm-tooltip': { backgroundColor: 'var(--surface)', color: 'var(--text)', border: '1px solid var(--border)' },
        '&.cm-focused': { outline: 'none' }
      }),
      CM.syntaxHighlighting(CM.HighlightStyle.define([
        { tag: [CM.tags.keyword, CM.tags.operator], color: 'var(--syntax-keyword)' },
        { tag: [CM.tags.string, CM.tags.special(CM.tags.string), CM.tags.regexp, CM.tags.monospace], color: 'var(--syntax-string)' },
        { tag: [CM.tags.number, CM.tags.bool, CM.tags.null], color: 'var(--syntax-number)' },
        { tag: [CM.tags.comment, CM.tags.meta, CM.tags.quote, CM.tags.processingInstruction], color: 'var(--syntax-comment)', fontStyle: 'italic' },
        { tag: [CM.tags.function(CM.tags.variableName), CM.tags.variableName, CM.tags.definition(CM.tags.variableName)], color: 'var(--syntax-name)' },
        { tag: [CM.tags.tagName, CM.tags.typeName, CM.tags.angleBracket], color: 'var(--syntax-tag)' },
        { tag: [CM.tags.attributeName, CM.tags.propertyName], color: 'var(--syntax-attr)' },
        { tag: CM.tags.heading, color: 'var(--syntax-heading)', fontWeight: '700' },
        { tag: [CM.tags.link, CM.tags.url], color: 'var(--syntax-link)', textDecoration: 'underline' },
        { tag: CM.tags.invalid, color: 'var(--syntax-error)' },
        { tag: CM.tags.strong, fontWeight: '700' },
        { tag: CM.tags.emphasis, fontStyle: 'italic' }
      ]))
    ];
  }
  // The diff view (diff-applet.js) draws its text with the same theme.
  window.dinahEditorTheme = makeTheme;

  window.dinahEditorCodeMirror = { load: loadCodeMirror, theme: makeTheme };
})();
