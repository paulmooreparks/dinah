// The parts of CodeMirror 6 that Dinah's Editor and its diff view use,
// bundled into one file that defines the global CM. Built with:
//   esbuild entry.js --bundle --format=iife --global-name=CM --minify --legal-comments=eof
export { EditorState, Compartment, StateEffect, StateField, Transaction, Prec, Text } from '@codemirror/state';
export {
    EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter,
    drawSelection, dropCursor, rectangularSelection, crosshairCursor, highlightSpecialChars,
    placeholder, Decoration, WidgetType, gutter, GutterMarker, ViewPlugin
} from '@codemirror/view';
export { defaultKeymap, history, historyField, historyKeymap, indentWithTab, undo, redo } from '@codemirror/commands';
export {
    searchKeymap, highlightSelectionMatches, openSearchPanel, search,
    searchPanelOpen, getSearchQuery, setSearchQuery, SearchQuery
} from '@codemirror/search';
export {
    bracketMatching, indentOnInput, syntaxHighlighting, HighlightStyle, StreamLanguage,
    foldGutter, foldKeymap, indentUnit
} from '@codemirror/language';
export { linter, lintGutter, lintKeymap, forceLinting } from '@codemirror/lint';
export { closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete';
// The diff view: side by side (MergeView) and inline (unifiedMergeView),
// with the chunks they find and the commands that move between them.
export {
    MergeView, unifiedMergeView, getChunks, getOriginalDoc, updateOriginalDoc, originalDocChangeEffect,
    goToNextChunk, goToPreviousChunk, acceptChunk, rejectChunk, uncollapseUnchanged, diff, presentableDiff, Chunk
} from '@codemirror/merge';
export { markdown } from '@codemirror/lang-markdown';
export { json, jsonParseLinter } from '@codemirror/lang-json';
// HTML, CSS and JavaScript come into the bundle with Markdown, which
// colours HTML inside a document; the diff view colours whole files with
// them.
export { javascript } from '@codemirror/lang-javascript';
export { html } from '@codemirror/lang-html';
export { css } from '@codemirror/lang-css';
export { shell } from '@codemirror/legacy-modes/mode/shell';
// Small stream languages for the files a diff most often shows.
export { go } from '@codemirror/legacy-modes/mode/go';
export { python } from '@codemirror/legacy-modes/mode/python';
export { yaml } from '@codemirror/legacy-modes/mode/yaml';
export { lua } from '@codemirror/legacy-modes/mode/lua';
export { powerShell } from '@codemirror/legacy-modes/mode/powershell';
export { toml } from '@codemirror/legacy-modes/mode/toml';
export { rust } from '@codemirror/legacy-modes/mode/rust';
export { xml } from '@codemirror/legacy-modes/mode/xml';
export { standardSQL } from '@codemirror/legacy-modes/mode/sql';
export { c, cpp, csharp, java } from '@codemirror/legacy-modes/mode/clike';
export { properties } from '@codemirror/legacy-modes/mode/properties';
export { dockerFile } from '@codemirror/legacy-modes/mode/dockerfile';
export { tags } from '@lezer/highlight';
