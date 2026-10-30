/* The command table, keys and menus of an Editor window (editor-applet.js builds the window and
   adds this part with usePart). A part is a function of the window's shared
   object ed; what it keeps for itself stays inside the function, so each
   window has its own. It returns the functions other parts call through ed
   and a teardown for what must be released when the window closes. */
(function () {
  'use strict';

  var C = window.dinahEditorCommon;
  var productName = C.productName, VIEWS = C.VIEWS;
  var K = window.dinahEditorKeys;
  var editorKeyRows = K.rows;

  window.dinahEditorParts.commands = function (ed) {
    /* ---- The command table ----------------------------------------------
       Every command of the window, declared once for PUDL's commands
       (pudl-commands.js): the key table's rows with what they do now
       (actions), a glyph where a toolbar shows one, and the commands of the
       toolbars alone (the views and the document's).
       The toolbars, PUDL's keys and an agent all reach a command by its id
       here; pudlCommands.list(root) is the window's commands as data, with
       their state now, and pudlCommands.run(id) runs one as pressing it
       would. A field that is a function is read each time the command is
       drawn, and actions() is computed once for a whole redraw. */
    var GLYPHS = { bold: 'bold', italic: 'italic', underline: 'underline', strike: 'strike', code: 'code', bullets: 'list', numbers: 'list-numbered',
      quote: 'quote', link: 'link', 'insert-checklist': 'check', 'insert-image': 'image', 'insert-file': 'clip', 'insert-code': 'script', undo: 'back', find: 'search' };
    // The commands only the formatting toolbar shows; Preview has no
    // formatting, so they are hidden there.
    var FORMATTING = /^(bold|italic|underline|strike|code|bullets|numbers|quote|link|insert-checklist|insert-image|insert-file|insert-code|paragraph-style|table-menu)$/;
    var nowActions = null, nowMarks = null;
    function does(id) {
      if (!nowActions) { nowActions = ed.actions(); Promise.resolve().then(function () { nowActions = null; nowMarks = null; }); }
      return nowActions[id] || {};
    }
    function marks() {
      if (!nowMarks) { var p = ed.activePage(); nowMarks = p ? p.marks() : textMarks(); }
      return nowMarks;
    }
    function pressed(id) {
      var m = marks();
      return id === 'bold' ? !!m.strong : id === 'italic' ? !!m.em : id === 'bullets' ? m.list === 'bullet_list' : id === 'numbers' ? m.list === 'ordered_list' : id === 'quote' ? m.block === 'blockquote' : !!m[id];
    }
    var TOGGLES = /^(bold|italic|underline|strike|code|bullets|numbers|quote)$/;
    function keyCommand(r) {
      var c = { id: r.id, label: r.label,
        disabled: function () { return !!does(r.id).disabled || !!ed.openFailed; },
        run: function () { var a = does(r.id); if (a.run && !a.disabled) return a.run(); } };
      // A key PUDL's menu bar keeps for the field being typed in (Cut, Copy,
      // Paste, Select All) is listed in Help but not bound here.
      if (r.shortcut) c.shortcut = r.shortcut;
      if (GLYPHS[r.id]) c.glyph = GLYPHS[r.id];
      if (TOGGLES.test(r.id)) c.checked = function () { return pressed(r.id); };
      else c.checked = function () { var a = does(r.id); return typeof a.checked === 'boolean' ? a.checked : null; };
      if (FORMATTING.test(r.id)) c.hidden = function () { return ed.mode === 'preview'; };
      return c;
    }
    function commandTable() {
      var all = editorKeyRows().filter(function (r) { return r.id; }).map(keyCommand);
      var styleIds = ['paragraph', 'h1', 'h2', 'h3', 'h4'];
      var formatting = function () { return ed.mode === 'preview'; };
      all.push(
        { id: 'paragraph-style', label: 'Paragraph Style', glyph: 'paragraph', hidden: formatting, items: styleIds,
          current: function () { var b = marks().block || 'paragraph'; var r = editorKeyRows().find(function (x) { return x.id === b; }); return r ? r.label : 'Paragraph'; } },
        { id: 'table-menu', label: 'Table', glyph: 'table', hidden: formatting,
          // In the table's own menu, Insert's "Table" would not say enough.
          items: [{ id: 'insert-a-table', label: 'Insert a Table', disabled: function () { return !!does('insert-table').disabled; }, run: function () { return does('insert-table').run(); } }, '-', 'row-above', 'row-below', 'column-before', 'column-after', 'delete-row', 'delete-column', 'delete-table'] }
      );
      ed.views.forEach(function (v) {
        all.push({ id: 'view-' + v, label: VIEWS[v], choice: 'view', choiceLabel: 'View', checked: function () { return ed.mode === v; }, run: function () { ed.setMode(v); } });
      });
      var opening = function () { return ed.draft ? ed.openFailed || null : ed.openFailed || 'The document is still opening.'; };
      all.push(
        { id: 'save-as', label: 'Save As…', hidden: function () { return ed.isNew(); }, disabled: function () { return !ed.draft || ed.busy || !!ed.outside; },
          reason: opening, run: function () { ed.saveAs(); } },
        { id: 'rename', label: 'Rename…', shortcut: 'F2', disabled: function () { return !ed.canRename(); },
          reason: function () { return opening() || ed.documentController.whyNot('rename') || null; }, run: ed.rename },
        { id: 'save', label: function () { return ed.draft && ed.isNew() ? 'Save As…' : 'Save'; },
          hidden: function () { return !!ed.draft && !ed.isNew() && !ed.draft.writeBack; },
          disabled: function () { return !ed.draft || ed.busy || !!ed.outside || (!ed.isNew() && (!ed.draft.writeBack || !ed.unsaved())); },
          reason: function () {
            if (opening()) return opening();
            if (ed.busy || ed.outside) return null;
            if (!ed.isNew() && !ed.draft.writeBack) return 'This source cannot be written from ' + productName();
            if (!ed.isNew() && !ed.unsaved()) return 'There are no changes to save';
            return null;
          }, run: ed.save }
      );
      return all;
    }
    var unregisterCommands = window.pudlCommands ? window.pudlCommands.register(ed.root, commandTable) : function () {};
    function textMarks() {
      if (!ed.view) return {};
      var st = ed.view.state, s = st.selection.main, line = st.doc.lineAt(s.head), t = line.text, out = {};
      var h = /^(#{1,6})\s/.exec(t);
      out.block = h ? 'h' + h[1].length : /^>\s?/.test(t) ? 'blockquote' : 'paragraph';
      out.list = /^\s*[-*+]\s/.test(t) ? 'bullet_list' : /^\s*\d+[.)]\s/.test(t) ? 'ordered_list' : null;
      var sel = st.sliceDoc(s.from, s.to), before = st.sliceDoc(Math.max(0, s.from - 4), s.from), after = st.sliceDoc(s.to, Math.min(st.doc.length, s.to + 4));
      function wrapped(open, close) {
        close = close || open;
        return (sel.length >= open.length + close.length && sel.startsWith(open) && sel.endsWith(close)) || (before.endsWith(open) && after.startsWith(close));
      }
      out.strong = wrapped('**');
      out.em = !out.strong && wrapped('*');
      out.code = wrapped('`');
      out.strike = wrapped('~~');
      out.underline = wrapped('<u>', '</u>');
      return out;
    }

    return { openFailed: null, teardown: unregisterCommands };
  };
})();
