/* The clipboard commands and the key rows of an Editor window (editor-applet.js builds the window and
   adds this part with usePart). A part is a function of the window's shared
   object ed; what it keeps for itself stays inside the function, so each
   window has its own. It returns the functions other parts call through ed
   and a teardown for what must be released when the window closes. */
(function () {
  'use strict';

  var C = window.dinahEditorCommon;
  var api = C.api, message = C.message;
  var K = window.dinahEditorKeys;
  var EDITOR_KEYS = K.table, editorKeyRows = K.rows;

  window.dinahEditorParts.clipboard = function (ed) {
    /* ---- The clipboard ----------------------------------------------------
       The Edit menu's Cut, Copy, Paste and Select All act where the person
       was last typing in this window: the page, the text, or a box such as a
       suggestion's edit box. Cut and Copy have that editor cut or copy as it does
       for Ctrl+X and Ctrl+C (document.execCommand, so the clipboard gets the
       same HTML and text either way); Paste takes what Go reads from the
       clipboard (ClipboardRead), since the page cannot read the clipboard
       without the web view asking the person, and pastes it as Ctrl+V
       would. The keys themselves are the web view's and the editors'. */
    var lastTyped = null;
    ed.root.addEventListener('focusin', function (e) {
      var t = e.target;
      if (!t || !t.closest) return;
      // A box first: a code block's Markdown is edited in a box on the page.
      var place = t.matches('textarea, input[type="text"], input[type="search"], input:not([type])') && !t.readOnly && !t.disabled ? t : null;
      place = place || t.closest('.ProseMirror') || t.closest('.cm-content');
      if (place) lastTyped = place;
    });
    /* Where the commands act: where the person was last typing. A box that
       has closed since, such as a code block's Markdown, which closes when
       the menu is opened, is not replaced by another place, so that nothing
       is pasted where the person did not mean. */
    function typingPlace() {
      if (lastTyped) {
        if (lastTyped.isConnected && ed.root.contains(lastTyped) && lastTyped.getClientRects().length) return lastTyped;
        lastTyped = null;
        ed.say('The box you were typing in has closed. Click where the text should go and choose the command again, or use the key in the box.', true);
        return null;
      }
      if (ed.pageView && ed.pageView.active() && ed.mode === 'page') return ed.pageView.element();
      return ed.view ? ed.view.contentDOM : null;
    }
    function onPageEditor(place) { return !!ed.pageView && ed.pageView.active() && place === ed.pageView.element(); }
    function focusPlace(place) {
      if (onPageEditor(place)) ed.pageView.focus();
      else if (ed.view && place === ed.view.contentDOM) ed.view.focus();
      else place.focus();
    }
    function clipboardCommand(name) {
      var place = typingPlace();
      if (!place) return;
      focusPlace(place);
      if (!document.execCommand(name)) ed.say(name === 'cut' ? 'The selection could not be cut; use Ctrl+X.' : 'The selection could not be copied; use Ctrl+C.', true);
    }
    function pasteCommand() {
      var place = typingPlace();
      if (!place) return;
      Promise.resolve(api().ClipboardRead()).then(function (c) {
        c = c || {};
        var text = (c.text || '').replace(/\r\n/g, '\n');
        if (onPageEditor(place)) return ed.pageView.paste({ html: c.html || '', text: text });
        focusPlace(place);
        if (!text) return;
        if (ed.view && place === ed.view.contentDOM) {
          ed.view.dispatch(Object.assign(ed.view.state.replaceSelection(text), { scrollIntoView: true, userEvent: 'input.paste' }));
          return;
        }
        var from = place.selectionStart, to = place.selectionEnd;
        place.setRangeText(text, from, to, 'end');
        place.dispatchEvent(new Event('input', { bubbles: true }));
      }).catch(function (err) { ed.say('The clipboard could not be read: ' + message(err), true); });
    }
    function selectAllCommand() {
      var place = typingPlace();
      if (!place) return;
      if (onPageEditor(place)) { ed.pageView.selectAll(); return; }
      focusPlace(place);
      if (ed.view && place === ed.view.contentDOM) ed.view.dispatch({ selection: { anchor: 0, head: ed.view.state.doc.length } });
      else place.select();
    }
    /* What each row of the Editor's key table (EDITOR_KEYS) does in this
       window, and when it can. */
    function actions() {
      var none = !ed.view, suggestions = !!(ed.draft && (ed.draft.suggestions || []).length);
      /* A command of the text acts on the part open in Page view, when it
         can there. */
      function onPage(name, run) { return function () { if (!(ed.mode === 'page' && ed.pageView && ed.pageView.active() && ed.pageView.run(name))) run(); }; }
      var onThePage = ed.mode === 'page' && !!ed.pageView && ed.pageView.active();
      /* Inserting a block works on the page and in the text, where the
         block's Markdown goes in after the line the cursor is on. */
      function insertAction(name) {
        return { run: onPage(name, function () { ed.insertMarkdown(window.dinahPageView.insert(name).markdown); }), disabled: none || !window.dinahPageView };
      }
      // A table is changed on the page, in the table the cursor is in, when
      // the change can be made there.
      function tableAction(name) {
        return { run: function () { ed.pageView.run(name); }, disabled: !onThePage || !ed.pageView.can(name) };
      }
      var does = {
        undo: { run: onPage('undo', function () { ed.command('undo'); }), disabled: none },
        redo: { run: onPage('redo', function () { ed.command('redo'); }), disabled: none },
        cut: { run: function () { clipboardCommand('cut'); }, disabled: none },
        copy: { run: function () { clipboardCommand('copy'); }, disabled: none },
        paste: { run: pasteCommand, disabled: none || !api().ClipboardRead },
        'select-all': { run: selectAllCommand, disabled: none },
        find: { run: function () { ed.command('find'); }, disabled: none },
        replace: { run: function () { ed.command('replace'); }, disabled: none },
        'find-next': { run: function () { ed.command('next'); }, disabled: none },
        'find-previous': { run: function () { ed.command('previous'); }, disabled: none },
        'go-to-line': { run: function () { ed.command('line'); }, disabled: none },
        bold: { run: onPage('bold', function () { ed.toggleMark('**'); }), disabled: none },
        // CommonMark ignores _ inside a word, so italics use *.
        italic: { run: onPage('italic', function () { ed.toggleMark('*'); }), disabled: none },
        code: { run: onPage('code', function () { ed.toggleMark('`'); }), disabled: none },
        // Underline is <u> in Markdown and strikethrough ~~.
        underline: { run: onPage('underline', function () { ed.toggleTag('<u>', '</u>'); }), disabled: none },
        strike: { run: onPage('strike', function () { ed.toggleMark('~~'); }), disabled: none },
        link: { run: onPage('link', ed.linkDialog), disabled: none },
        h1: { run: onPage('h1', function () { ed.setBlock('heading', 1); }), disabled: none },
        h2: { run: onPage('h2', function () { ed.setBlock('heading', 2); }), disabled: none },
        h3: { run: onPage('h3', function () { ed.setBlock('heading', 3); }), disabled: none },
        h4: { run: onPage('h4', function () { ed.setBlock('heading', 4); }), disabled: none },
        paragraph: { run: onPage('paragraph', function () { ed.setBlock('paragraph'); }), disabled: none },
        bullets: { run: onPage('bullets', function () { ed.setBlock('bullet'); }), disabled: none },
        numbers: { run: onPage('numbers', function () { ed.setBlock('numbered'); }), disabled: none },
        quote: { run: function () { ed.setBlock('quote'); }, disabled: none },
        'insert-image': { run: function () { ed.pickFiles(true); }, disabled: none || !ed.draft },
        'insert-file': { run: function () { ed.pickFiles(false); }, disabled: none || !ed.draft },
        'insert-table': insertAction('insert-table'),
        'insert-checklist': insertAction('insert-checklist'),
        'insert-code': insertAction('insert-code'),
        'row-above': tableAction('row-above'),
        'row-below': tableAction('row-below'),
        'column-before': tableAction('column-before'),
        'column-after': tableAction('column-after'),
        'delete-row': tableAction('delete-row'),
        'delete-column': tableAction('delete-column'),
        'delete-table': tableAction('delete-table'),
        'next-suggestion': { run: function () { ed.goToSuggestion(1); }, disabled: none || !suggestions },
        'previous-suggestion': { run: function () { ed.goToSuggestion(-1); }, disabled: none || !suggestions },
        accept: { run: function () { ed.decideHere(true); }, disabled: none || !suggestions },
        reject: { run: function () { ed.decideHere(false); }, disabled: none || !suggestions },
        'next-problem': { run: function () { ed.goToProblem(1); }, disabled: none || !ed.problemLines.length },
        'previous-problem': { run: function () { ed.goToProblem(-1); }, disabled: none || !ed.problemLines.length },
        'switch-view': { run: ed.switchView },
        wrap: { run: function () { ed.setWrap(!ed.wrapLines); }, disabled: none, checked: ed.wrapLines },
        problems: { run: function () { ed.$('problems-btn').click(); }, disabled: ed.$('problems-btn').hidden, checked: ed.problemsOpen },
        'next-pane': { run: function () { ed.nextPane(1); } },
        'previous-pane': { run: function () { ed.nextPane(-1); } }
      };
      // No command runs under one of the window's dialogs.
      if (ed.root.querySelector('dialog[open]')) Object.keys(does).forEach(function (k) { does[k].disabled = true; });
      return does;
    }
    /* The window's commands for command search, with why one cannot run
       now. */
    var SEARCH_MENUS = { edit: 'Edit', insert: 'Insert', format: 'Format', review: 'Review', view: 'View' };
    function searchRows() {
      var does = actions(), modal = !!ed.root.querySelector('dialog[open]');
      function reason(id) {
        if (modal) return 'A dialog in the Editor window is open.';
        if (!ed.view) return 'The text is still loading.';
        if (/suggestion|^accept$|^reject$/.test(id)) return 'The draft has no suggestions.';
        if (/problem/.test(id)) return 'The draft has no problems.';
        return '';
      }
      return editorKeyRows().map(function (r) {
        var a = does[r.id] || {};
        return { id: 'editor-' + r.id, label: r.label, menu: SEARCH_MENUS[r.menu] || r.menu, shortcut: r.shortcut || '', enabled: !a.disabled,
          reason: a.disabled ? reason(r.id) : '', run: function () { var now = actions()[r.id]; if (now && !now.disabled) now.run(); } };
      });
    }
    /* A menu's rows, as PUDL's menu bar takes them. */
    function menuRows(menu) {
      var does = actions();
      function rowOf(r) {
        var a = does[r.id] || {}, row = { label: r.label, run: a.run, disabled: !!a.disabled };
        if (r.shortcut) row.shortcut = r.shortcut;
        if (r.glyph) row.glyph = r.glyph;
        if (r.glyphOnly) row.glyphOnly = true;
        if (typeof a.checked === 'boolean') row.checked = a.checked;
        if (a.radio) row.radio = a.radio;
        return row;
      }
      return EDITOR_KEYS.filter(function (r) { return r === '-' + menu || r.menu === menu; }).map(function (r) {
        if (typeof r === 'string') return '-';
        if (r.items) return { label: r.label, glyph: r.glyph, items: r.items.map(rowOf) };
        return rowOf(r);
      });
    }
    /* The text's keymap, made from the rows that give a key in CodeMirror's
       spelling. A row that cannot run now leaves the key to CodeMirror. */
    function textKeymap() {
      return editorKeyRows().filter(function (r) { return r.cm; }).map(function (r) {
        return { key: r.cm, preventDefault: true, run: function () {
          var a = actions()[r.id];
          if (!a || a.disabled) return false;
          a.run();
          return true;
        } };
      });
    }

    return { actions: actions, searchRows: searchRows, menuRows: menuRows, textKeymap: textKeymap };
  };
})();
