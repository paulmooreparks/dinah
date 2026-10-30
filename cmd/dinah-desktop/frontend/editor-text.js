/* The text surface of an Editor window (editor-applet.js builds the window and
   adds this part with usePart). A part is a function of the window's shared
   object ed; what it keeps for itself stays inside the function, so each
   window has its own. It returns the functions other parts call through ed
   and a teardown for what must be released when the window closes. */
(function () {
  'use strict';

  var C = window.dinahEditorCommon;
  var api = C.api, originRef = C.originRef, isMarkdown = C.isMarkdown;
  var M = window.dinahEditorCodeMirror;
  var makeTheme = M.theme;

  window.dinahEditorParts.text = function (ed) {
    /* ---- Text ---------------------------------------------------------- */

    function position() {
      if (!ed.view) return;
      var head = ed.view.state.selection.main.head, line = ed.view.state.doc.lineAt(head);
      ed.say('Line ' + line.number + ', column ' + (head - line.from + 1));
    }
    /* Save, Save As and Open are Documents commands, whose keys the menu bar
       takes anywhere, the text included, so the text
       declares none of them. */
    function makeView(text) {
      ed.wrapComp = new ed.CM.Compartment();
      ed.view = new ed.CM.EditorView({
        parent: ed.$('text'),
        state: ed.CM.EditorState.create({
          doc: text,
          extensions: [
            ed.CM.lineNumbers(), ed.CM.highlightActiveLineGutter(), ed.CM.highlightSpecialChars(),
            ed.CM.history(), ed.CM.drawSelection(), ed.CM.dropCursor(), ed.CM.indentOnInput(),
            ed.CM.bracketMatching(), ed.CM.closeBrackets(), ed.CM.rectangularSelection(), ed.CM.crosshairCursor(),
            ed.CM.highlightActiveLine(), ed.CM.highlightSelectionMatches(),
            ed.CM.keymap.of(ed.textKeymap()),
            // Of CodeMirror's search keys only Go to Line is kept; find and
            // replace are the Editor's own bar, which works on the page too.
            ed.CM.keymap.of([].concat(ed.CM.closeBracketsKeymap, ed.CM.defaultKeymap, ed.CM.searchKeymap.filter(function (k) { return k.key === 'Mod-Alt-g'; }), ed.CM.historyKeymap, [ed.CM.indentWithTab])),
            makeTheme(ed.CM),
            ed.suggestionMarks(),
            ed.findMarksField(),
            ed.wrapComp.of(ed.wrapLines ? ed.CM.EditorView.lineWrapping : []),
            isMarkdown(ed.draft) ? ed.CM.markdown() : [],
            ed.CM.EditorView.contentAttributes.of({ 'aria-label': 'Text of ' + (ed.draft.title || 'the draft') }),
            ed.CM.EditorView.updateListener.of(function (u) {
              if (u.docChanged && !ed.applying) ed.schedule();
              if (u.docChanged && ed.finding.open) ed.scheduleFind();
              // A change that did not come from the page is read into it.
              if (u.docChanged && !ed.fromPage && ed.pageView) ed.pageView.textChanged();
              // The text's line and column mean nothing in Page view, where
              // the status line keeps what the page said.
              if (u.selectionSet || u.docChanged) { if (ed.mode !== 'page') position(); ed.commandsChanged(); ed.scheduleContext(); }
            })
          ]
        })
      });
    }
    /* Page view (page-view.js) edits the whole document as its page. What
       is typed there comes into the text here, as the smallest change that
       covers it, and is kept in the draft as typing in the text is. */
    function attachPage() {
      return window.dinahPageView.attach({
        root: ed.root, page: ed.$('page'), scroll: ed.$('scroll'),
        text: function () { return ed.view.state.doc.toString(); },
        setText: function (text) {
          var old = ed.view.state.doc.toString();
          if (old === text) return;
          var start = 0, oldEnd = old.length, newEnd = text.length;
          while (start < oldEnd && start < newEnd && old.charCodeAt(start) === text.charCodeAt(start)) start++;
          while (oldEnd > start && newEnd > start && old.charCodeAt(oldEnd - 1) === text.charCodeAt(newEnd - 1)) { oldEnd--; newEnd--; }
          ed.fromPage = true;
          try { ed.view.dispatch({ changes: { from: start, to: oldEnd, insert: text.slice(start, newEnd) }, userEvent: 'input.page' }); } finally { ed.fromPage = false; }
        },
        docOf: function (text) { return api().PageDocumentOf(ed.id, text); },
        markdownOf: function (doc) { return api().PageDocumentMarkdown(doc); },
        sourceOf: function (src) { return api().PageSourceOf(ed.id, src); },
        blocksOf: function (markdown) { return api().PageBlocksOf(ed.id, markdown); },
        draw: function (el, html, diagrams) {
          window.dinahDocumentPreview.renderBlocks(el, [{ line: 0, html: html, diagrams: diagrams }], ed.key);
          window.dinahDocumentPreview.loadAttachments(el, originRef(ed.draft), { draft: ed.id });
        },
        keys: ed.textKeymap,
        decide: function (suggestion, accept) { ed.decide(suggestion, accept, false); },
        editSuggestion: ed.suggestionEditor,
        say: ed.say,
        changed: function () { ed.commandsChanged(); ed.pageSelectionChanged(); },
        failed: function (text) { ed.pageOff = true; if (ed.mode === 'page') ed.setMode('split'); else { ed.pageView.enable(false); ed.commandsChanged(); ed.schedulePreview(0); } ed.say(text, true); }
      });
    }
    function setWrap(on) {
      ed.wrapLines = on;
      if (ed.view) ed.view.dispatch({ effects: ed.wrapComp.reconfigure(on ? ed.CM.EditorView.lineWrapping : []) });
    }
    function command(name) {
      if (!ed.view) return;
      ({
        undo: function () { ed.CM.undo(ed.view); },
        redo: function () { ed.CM.redo(ed.view); },
        find: function () { ed.openFind(false); },
        replace: function () { ed.openFind(true); },
        line: function () { searchCommand('Mod-Alt-g'); },
        next: function () { ed.findStep(1); },
        previous: function () { ed.findStep(-1); }
      })[name]();
    }
    /* Go to Line is CodeMirror's own command, taken from its search keymap
       by the key it gives it, since the bundle does not export it by name. */
    function searchCommand(cmKey) {
      if (!ed.view) return;
      if (ed.reading()) ed.setMode('split');
      var binding = ed.CM.searchKeymap.find(function (k) { return k.key === cmKey; });
      if (binding) binding.run(ed.view);
      ed.view.focus();
    }

    return { position: position, makeView: makeView, attachPage: attachPage, setWrap: setWrap, command: command };
  };
})();
