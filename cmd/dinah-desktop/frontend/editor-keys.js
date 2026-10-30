/* The Editor's key table (editor-applet.js reads it as
   window.dinahEditorKeys). It loads just before editor-applet.js, so the
   Editor's part of Help › Keyboard Shortcuts is added where it was. */
(function () {
  'use strict';

  /* The Editor's commands and keys, declared once: the Edit and View rows of every Editor
     window, the text's keymap (cm, in CodeMirror's spelling, for a key the
     text must take) and the Editor's part of Help › Keyboard Shortcuts all
     come from this table. A string is a separator in that menu; items make a
     flyout. */
  var EDITOR_KEYS = [
    { id: 'undo', menu: 'edit', label: 'Undo', shortcut: 'Ctrl+Z', glyph: 'undo' },
    { id: 'redo', menu: 'edit', label: 'Redo', shortcut: 'Ctrl+Y', glyph: 'redo' },
    '-edit',
    // The keys of these four are the web view's and the editors' own, which
    // PUDL's menu bar keeps for the field being typed in and does not show
    // on a row; Help › Keyboard Shortcuts lists them (key). The rows act
    // where the person was last typing (clipboardCommand).
    { id: 'cut', menu: 'edit', label: 'Cut', key: 'Ctrl+X' },
    { id: 'copy', menu: 'edit', label: 'Copy', key: 'Ctrl+C', glyph: 'copy' },
    { id: 'paste', menu: 'edit', label: 'Paste', key: 'Ctrl+V' },
    { id: 'select-all', menu: 'edit', label: 'Select All', key: 'Ctrl+A' },
    '-edit',
    { id: 'find', menu: 'edit', label: 'Find…', shortcut: 'Ctrl+F', glyph: 'search' },
    { id: 'replace', menu: 'edit', label: 'Replace…', shortcut: 'Ctrl+H', cm: 'Mod-h' },
    { id: 'find-next', menu: 'edit', label: 'Find Next', shortcut: 'F3' },
    { id: 'find-previous', menu: 'edit', label: 'Find Previous', shortcut: 'Shift+F3' },
    { id: 'go-to-line', menu: 'edit', label: 'Go to Line…', shortcut: 'Ctrl+G', cm: 'Mod-g' },
    // The Insert menu: what is put into the document.
    { id: 'link', menu: 'insert', label: 'Link…', shortcut: 'Ctrl+K', cm: 'Mod-k', glyph: 'link' },
    '-insert',
    { id: 'insert-image', menu: 'insert', label: 'Image…', glyph: 'image' },
    { id: 'insert-file', menu: 'insert', label: 'File…', glyph: 'clip' },
    { id: 'insert-table', menu: 'insert', label: 'Table', glyph: 'table' },
    { id: 'insert-checklist', menu: 'insert', label: 'Checklist', glyph: 'check' },
    { id: 'insert-code', menu: 'insert', label: 'Code Block', glyph: 'code' },
    // The Format menu: how the text looks, its paragraphs, lists and tables.
    // The marks and lists everyone knows are their glyphs alone on the
    // ribbon (glyphOnly), named in a tooltip.
    { id: 'bold', menu: 'format', label: 'Bold', shortcut: 'Ctrl+B', cm: 'Mod-b', glyph: 'bold', glyphOnly: true },
    { id: 'italic', menu: 'format', label: 'Italic', shortcut: 'Ctrl+I', cm: 'Mod-i', glyph: 'italic', glyphOnly: true },
    { id: 'underline', menu: 'format', label: 'Underline', shortcut: 'Ctrl+U', cm: 'Mod-u', glyph: 'underline', glyphOnly: true },
    { id: 'strike', menu: 'format', label: 'Strikethrough', shortcut: 'Ctrl+Shift+X', cm: 'Mod-Shift-x', glyph: 'strike', glyphOnly: true },
    { id: 'code', menu: 'format', label: 'Code', shortcut: 'Ctrl+Shift+C', cm: 'Mod-Shift-c', glyph: 'code' },
    '-format',
    { menu: 'format', label: 'Paragraph Style', glyph: 'paragraph', items: [
      { id: 'h1', label: 'Heading 1', shortcut: 'Ctrl+1', cm: 'Mod-1' },
      { id: 'h2', label: 'Heading 2', shortcut: 'Ctrl+2', cm: 'Mod-2' },
      { id: 'h3', label: 'Heading 3', shortcut: 'Ctrl+3', cm: 'Mod-3' },
      { id: 'h4', label: 'Heading 4', shortcut: 'Ctrl+4', cm: 'Mod-4' },
      { id: 'paragraph', label: 'Paragraph', shortcut: 'Ctrl+0', cm: 'Mod-0' }] },
    { id: 'bullets', menu: 'format', label: 'Bulleted List', shortcut: 'Ctrl+Shift+L', cm: 'Mod-Shift-l', glyph: 'list', glyphOnly: true },
    { id: 'numbers', menu: 'format', label: 'Numbered List', shortcut: 'Ctrl+Shift+O', cm: 'Mod-Shift-o', glyph: 'list-numbered', glyphOnly: true },
    { id: 'quote', menu: 'format', label: 'Quote', glyph: 'quote' },
    '-format',
    { menu: 'format', label: 'Table', glyph: 'table', items: [
      { id: 'row-above', label: 'Add a Row Above' },
      { id: 'row-below', label: 'Add a Row Below' },
      { id: 'column-before', label: 'Add a Column Before' },
      { id: 'column-after', label: 'Add a Column After' },
      { id: 'delete-row', label: 'Delete the Row' },
      { id: 'delete-column', label: 'Delete the Column' },
      { id: 'delete-table', label: 'Delete the Table' }] },
    // The Review menu: the agent's suggestions and the document's problems.
    { id: 'next-suggestion', menu: 'review', label: 'Next Suggestion', shortcut: 'F7', cm: 'F7' },
    { id: 'previous-suggestion', menu: 'review', label: 'Previous Suggestion', shortcut: 'Shift+F7', cm: 'Shift-F7' },
    { id: 'accept', menu: 'review', label: 'Accept the Suggestion', shortcut: 'Alt+Enter', cm: 'Alt-Enter', glyph: 'tick' },
    { id: 'reject', menu: 'review', label: 'Reject the Suggestion', shortcut: 'Alt+Delete', cm: 'Alt-Delete', glyph: 'close' },
    '-review',
    { id: 'next-problem', menu: 'review', label: 'Next Problem', shortcut: 'F8', cm: 'F8', glyph: 'warning' },
    { id: 'previous-problem', menu: 'review', label: 'Previous Problem', shortcut: 'Shift+F8', cm: 'Shift-F8' },
    { id: 'switch-view', menu: 'view', label: 'Switch View', shortcut: 'Ctrl+E', cm: 'Mod-e' },
    '-view',
    { id: 'wrap', menu: 'view', label: 'Wrap Lines', shortcut: 'Alt+Z', cm: 'Alt-z' },
    { id: 'problems', menu: 'view', label: 'Show Problems', shortcut: 'Ctrl+Shift+M', cm: 'Mod-Shift-m' },
    { id: 'next-pane', menu: 'view', label: 'Next Pane', shortcut: 'F6', cm: 'F6' },
    { id: 'previous-pane', menu: 'view', label: 'Previous Pane', shortcut: 'Shift+F6', cm: 'Shift-F6' }
  ];
  /* The table's rows that are commands, the flyouts' rows among them. */
  function editorKeyRows() {
    var out = [];
    EDITOR_KEYS.forEach(function (r) {
      if (typeof r === 'string') return;
      if (r.items) r.items.forEach(function (i) { out.push(Object.assign({ menu: r.menu }, i)); });
      else out.push(r);
    });
    return out;
  }
  /* The Editor's keys for Help › Keyboard Shortcuts: its commands, from the
     table above, then those of each part of the window, which are the text's
     own (CodeMirror's default keymap), the page's (page-view.js), a
     suggestion's edit box and Preview's. */
  if (window.dinahCommands && window.dinahCommands.addHelp) {
    window.dinahCommands.addHelp('Editor', editorKeyRows().filter(function (r) { return r.shortcut || r.key; }).map(function (r) { return { label: r.label, key: r.shortcut || r.key }; }).concat([
      { where: 'In the Text', label: 'Redo', key: 'Ctrl+Shift+Z' },
      { where: 'In the Text', label: 'Move the Line Up or Down', key: 'Alt+Up / Alt+Down' },
      { where: 'In the Text', label: 'Copy the Line Up or Down', key: 'Shift+Alt+Up / Shift+Alt+Down' },
      { where: 'In the Text', label: 'Delete the Line', key: 'Ctrl+Shift+K' },
      { where: 'In the Text', label: 'Indent More or Less', key: 'Ctrl+] / Ctrl+[' },
      { where: 'In the Text', label: 'Select the Next Match', key: 'Ctrl+D' },
      { where: 'In the Text', label: 'Leave the Text with Tab', key: 'Ctrl+M, then Tab' },
      { where: 'Visual View', label: 'Next or Previous Cell of a Table', key: 'Tab / Shift+Tab' },
      { where: 'Visual View', label: 'Add a Row, From the Last Cell', key: 'Tab' },
      { where: 'Visual View', label: 'Indent or Outdent a List Item', key: 'Tab / Shift+Tab' },
      { where: 'Visual View', label: 'Insert a Checklist, Table or Code Block', key: '/ on an empty line' },
      { where: 'Visual View', label: 'Choose in the / Menu', key: 'Up / Down, then Enter or Tab' },
      { where: 'Visual View', label: 'Put a Block Back on the Page', key: 'Esc or Ctrl+Enter' },
      { where: 'Visual View', label: 'Redo', key: 'Ctrl+Shift+Z' },
      { where: 'Editing a Suggestion', label: 'Keep the New Wording', key: 'Ctrl+Enter' },
      { where: 'Editing a Suggestion', label: 'Leave the Wording as It Was', key: 'Esc' },
      { where: 'Preview', label: 'Switch to Split at That Line', key: 'Any letter' },
      { where: 'Preview', label: 'Scroll', key: 'Space / Shift+Space' }]));
  }

  window.dinahEditorKeys = { table: EDITOR_KEYS, rows: editorKeyRows };
})();
