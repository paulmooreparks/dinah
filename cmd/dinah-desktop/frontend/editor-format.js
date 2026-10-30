/* Formatting, insertion and navigation by key in an Editor window (editor-applet.js builds the window and
   adds this part with usePart). A part is a function of the window's shared
   object ed; what it keeps for itself stays inside the function, so each
   window has its own. It returns the functions other parts call through ed
   and a teardown for what must be released when the window closes. */
(function () {
  'use strict';

  var C = window.dinahEditorCommon;
  var api = C.api, VIEWS = C.VIEWS;

  window.dinahEditorParts.format = function (ed) {
    /* ---- Keys -----------
       One table declares the Editor's commands: their menu, label, key as
       the menu shows it, the key in CodeMirror's spelling when the text must
       take it, and what they do. The Edit and View rows are drawn from it,
       PUDL's menu bar takes the keys while the window is in front, and the
       text's keymap is made from the same rows, so no key is declared twice.
       The text's keymap comes before CodeMirror's own, which gives it the
       first say. */
    function toText() { if (ed.reading()) ed.setMode('split'); if (ed.view) ed.view.focus(); return !!ed.view; }
    function selected() { var s = ed.view.state.selection.main; return { from: s.from, to: s.to, text: ed.view.state.sliceDoc(s.from, s.to) }; }
    /* Wraps the selection in a mark, or takes the mark away when the
       selection already has it, inside or just outside. A selection whose
       middle has the mark too ("**a** and **b**") is two marked spans, so it
       is wrapped. A one-character mark next to another of the same character
       is part of a longer mark (the * of **), so it is not taken away. */
    function toggleMark(mark) {
      if (!toText()) return;
      var s = selected(), st = ed.view.state, n = mark.length;
      var outside = st.sliceDoc(Math.max(0, s.from - n), s.from) === mark && st.sliceDoc(s.to, s.to + n) === mark &&
        (n > 1 || (st.sliceDoc(Math.max(0, s.from - 2), s.from - 1) !== mark && st.sliceDoc(s.to + 1, s.to + 2) !== mark));
      var inner = s.text.slice(n, -n);
      if (s.text.length >= 2 * n && s.text.slice(0, n) === mark && s.text.slice(-n) === mark && inner.indexOf(mark) < 0) {
        ed.view.dispatch({ changes: { from: s.from, to: s.to, insert: inner }, selection: { anchor: s.from, head: s.to - 2 * n } });
      } else if (outside) {
        ed.view.dispatch({ changes: [{ from: s.from - n, to: s.from }, { from: s.to, to: s.to + n }], selection: { anchor: s.from - n, head: s.to - n } });
      } else {
        ed.view.dispatch({ changes: { from: s.from, to: s.to, insert: mark + s.text + mark }, selection: { anchor: s.from + n, head: s.to + n } });
      }
    }
    var BLOCK_PREFIX = /^(#{1,6}\s+|>\s?|[-*+]\s+|\d+[.)]\s+)/;
    /* Gives each line of the selection a block prefix in place of the one
       it has: a heading's #, a quote's >, a list's - or 1., or none for a
       paragraph. When every line already has the prefix asked for, it is
       taken away. */
    function setBlock(kind, level) {
      if (!toText()) return;
      var st = ed.view.state, s = st.selection.main, first = st.doc.lineAt(s.from).number, last = st.doc.lineAt(s.to).number;
      // A selection that ends at the start of a line does not take that line in.
      if (last > first && st.doc.line(last).from === s.to) last--;
      // A blank line between paragraphs keeps no prefix and is not counted.
      var lines = [];
      for (var n = first; n <= last; n++) if (st.doc.line(n).text.trim() || first === last) lines.push(st.doc.line(n));
      function prefix(i) { return kind === 'heading' ? '#'.repeat(level) + ' ' : kind === 'quote' ? '> ' : kind === 'bullet' ? '- ' : kind === 'numbered' ? (i + 1) + '. ' : ''; }
      var has = kind !== 'paragraph' && lines.every(function (line, i) {
        var m = BLOCK_PREFIX.exec(line.text);
        return m && (kind === 'numbered' ? /^\d+[.)]\s+$/.test(m[0]) : m[0].replace(/\s+$/, ' ') === prefix(i).replace(/\s+$/, ' '));
      });
      var changes = lines.map(function (line, i) {
        var m = BLOCK_PREFIX.exec(line.text), strip = m ? m[0].length : 0;
        return { from: line.from, to: line.from + strip, insert: has ? '' : prefix(i) };
      });
      ed.view.dispatch({ changes: changes });
    }
    /* Puts a block's Markdown in after the line the cursor is on, a blank
       line apart, with its placeholder text selected to be typed over. */
    function insertMarkdown(markdown) {
      if (!toText()) return;
      var st = ed.view.state, line = st.doc.lineAt(st.selection.main.head);
      var before = line.text.trim() ? '\n\n' : '', start = line.to + before.length;
      var hole = markdown.indexOf(window.dinahPageView.placeholder);
      ed.view.dispatch({
        changes: { from: line.to, insert: before + markdown + '\n' },
        selection: hole >= 0 ? { anchor: start + hole, head: start + hole + window.dinahPageView.placeholder.length } : { anchor: start + markdown.length },
        scrollIntoView: true
      });
    }
    /* Files added to the document: Insert › Image… and File…, or an image
       pasted or a file dropped into the text or the page. Each is kept with
       the draft and written with it when it is saved, as a file beside the
       document. An image goes in as
       an image and any other file as a link to it, where a new block goes on
       the page, or after the line the cursor is on in the text. */
    var IMAGE_TYPES = /^image\/(png|jpeg|gif|webp|svg\+xml)$/;
    function fileMarkdown(name, image) {
      var target = /[\s()<>]/.test(name) ? '<' + name + '>' : name;
      if (!image) return '[' + name.replace(/([\\\[\]])/g, '\\$1') + '](' + target + ')';
      return '![](' + target + ')';
    }
    function insertFiles(names, images) {
      if (!names || !names.length) return;
      var markdown = names.map(function (n, i) { return fileMarkdown(n, Array.isArray(images) ? images[i] : images); }).join('\n\n');
      if (ed.mode === 'page' && ed.pageView && ed.pageView.active()) ed.pageView.insertMarkdown(markdown).catch(function (err) { ed.say(String(err && err.message || err), true); });
      else insertMarkdown(markdown);
    }
    function pickFiles(images) {
      Promise.resolve().then(function () { return api().PickDraftFiles(ed.id, images); })
        .then(function (names) { if (!ed.disposed) insertFiles(names, images); })
        .catch(function (err) { ed.say(String(err && err.message || err), true); });
    }
    function readBase64(file) {
      return new Promise(function (resolve, reject) {
        var r = new FileReader();
        r.onload = function () { var s = String(r.result); resolve(s.slice(s.indexOf(',') + 1)); };
        r.onerror = function () { reject(r.error || new Error('The file could not be read.')); };
        r.readAsDataURL(file);
      });
    }
    function addFiles(list) {
      var files = Array.from(list || []);
      Promise.all(files.map(function (f) {
        // A pasted image has no name of its own.
        var name = f.name && f.name !== 'image.png' ? f.name : 'pasted-' + new Date().toISOString().replace(/[:.]/g, '-') + '.' + ((f.type.split('/')[1] || 'png').replace('svg+xml', 'svg').replace('jpeg', 'jpg'));
        return readBase64(f).then(function (data) { return api().AddDraftFile(ed.id, name, data); });
      })).then(function (names) {
        if (!ed.disposed) insertFiles(names, files.map(function (f) { return IMAGE_TYPES.test(f.type); }));
      }).catch(function (err) { ed.say(String(err && err.message || err), true); });
    }
    function inDocument(target) { return !!(target && target.closest && target.closest('.cm-editor, .ProseMirror')) && !target.closest('textarea, input'); }
    function filesOf(transfer) { return transfer ? Array.from(transfer.files || []) : []; }
    function pasteFiles(e) {
      var files = filesOf(e.clipboardData).filter(function (f) { return IMAGE_TYPES.test(f.type); });
      if (!ed.draft || !files.length || !inDocument(e.target)) return;
      e.preventDefault(); e.stopPropagation();
      addFiles(files);
    }
    function dropFiles(e) {
      var files = filesOf(e.dataTransfer);
      if (!ed.draft || !files.length || !inDocument(e.target)) return;
      e.preventDefault(); e.stopPropagation();
      addFiles(files);
    }
    ed.root.addEventListener('paste', pasteFiles, true);
    ed.root.addEventListener('drop', dropFiles, true);

    /* Wraps the selection in an opening and a closing tag, or takes them
       away when the selection has them, inside or just outside. */
    function toggleTag(open, close) {
      if (!toText()) return;
      var s = selected(), st = ed.view.state;
      var outside = st.sliceDoc(Math.max(0, s.from - open.length), s.from) === open && st.sliceDoc(s.to, s.to + close.length) === close;
      if (s.text.startsWith(open) && s.text.endsWith(close) && s.text.length >= open.length + close.length) {
        var inner = s.text.slice(open.length, s.text.length - close.length);
        ed.view.dispatch({ changes: { from: s.from, to: s.to, insert: inner }, selection: { anchor: s.from, head: s.from + inner.length } });
      } else if (outside) {
        ed.view.dispatch({ changes: [{ from: s.from - open.length, to: s.from }, { from: s.to, to: s.to + close.length }], selection: { anchor: s.from - open.length, head: s.to - open.length } });
      } else {
        ed.view.dispatch({ changes: { from: s.from, to: s.to, insert: open + s.text + close }, selection: { anchor: s.from + open.length, head: s.to + open.length } });
      }
    }
    function linkDialog() {
      if (!toText()) return;
      var s = selected();
      var dialog = document.createElement('dialog');
      dialog.className = 'dialog editor-dialog';
      dialog.setAttribute('aria-labelledby', ed.key + '-link-title');
      dialog.innerHTML = '<h2 class="dialog-title" id="' + ed.key + '-link-title">Link</h2><div class="dialog-body">' +
        '<div class="form-group"><label class="form-label" for="' + ed.key + '-link-text">Text</label><input class="form-input" id="' + ed.key + '-link-text"></div>' +
        '<div class="form-group"><label class="form-label" for="' + ed.key + '-link-url">Address</label><input class="form-input" id="' + ed.key + '-link-url" placeholder="https://"></div></div>' +
        '<div class="dialog-actions"><button type="button" class="btn" data-no>Cancel</button><button type="button" class="btn btn-primary" data-yes>Add the Link</button></div>';
      var text = dialog.querySelector('#' + ed.key + '-link-text'), url = dialog.querySelector('#' + ed.key + '-link-url');
      text.value = s.text;
      ed.root.appendChild(dialog);
      // Escape closes the dialog too, so the text takes focus back on close.
      function close() { dialog.close(); }
      dialog.addEventListener('close', function () { if (dialog.isConnected) dialog.remove(); ed.view.focus(); });
      dialog.querySelector('[data-no]').addEventListener('click', close);
      function add() {
        var address = url.value.trim();
        if (!address) { url.focus(); return; }
        var label = text.value.trim() || address, insert = '[' + label + '](' + address + ')';
        close();
        ed.view.dispatch({ changes: { from: s.from, to: s.to, insert: insert }, selection: { anchor: s.from + insert.length } });
      }
      dialog.querySelector('[data-yes]').addEventListener('click', add);
      [text, url].forEach(function (input) {
        input.addEventListener('keydown', function (e) { if (e.key === 'Enter' && !e.isComposing) { e.preventDefault(); add(); } });
      });
      dialog.showModal();
      (s.text ? url : text).focus();
    }
    /* Where each suggestion is in the text now. The marks on screen move
       with typing, so their places are read from them rather than from the
       draft's spans, which hold only while the text matches the saved
       draft. */
    function placedSpans() {
      var spans = {};
      if (!ed.view || !ed.marksField) return spans;
      ed.view.state.field(ed.marksField).between(0, ed.view.state.doc.length, function (from, to, deco) {
        var spec = deco.spec || {};
        var sid = spec.attributes ? spec.attributes['data-suggestion'] : spec.widget && spec.widget.s ? spec.widget.s.id : null;
        if (!sid) return;
        var span = spans[sid];
        spans[sid] = span ? [Math.min(span[0], from), Math.max(span[1], to)] : [from, to];
      });
      return spans;
    }
    /* The suggestions in the text's order, by where they are placed now.
       Going back compares with the selection's start, so a suggestion just
       selected by going forward is not found again. */
    function suggestionAt(direction, spans) {
      var ids = Object.keys(spans).sort(function (a, b) { return spans[a][0] - spans[b][0]; });
      if (!ids.length) return null;
      var sel = ed.view.state.selection.main;
      if (direction > 0) return ids.find(function (sid) { return spans[sid][0] > sel.head; }) || ids[0];
      var before = ids.filter(function (sid) { return spans[sid][0] < sel.from; });
      return before.length ? before[before.length - 1] : ids[ids.length - 1];
    }
    function goToSuggestion(direction) {
      if (!toText()) return;
      var spans = placedSpans(), sid = suggestionAt(direction, spans);
      if (!sid) { ed.say('The draft has no suggestions.'); return; }
      ed.view.dispatch({ selection: { anchor: spans[sid][0], head: spans[sid][1] }, scrollIntoView: true });
    }
    /* Go To on a proposal's row: its changes in the order of the text, the
       next one after the one last shown, so pressing it again walks through
       them. On the page the change's card is scrolled into the middle,
       flashed and its Accept focused; in the text the change is selected;
       Preview, which shows no suggestions, gives way to the page. */
    var proposalAt = {};
    function goToProposal(group) {
      var spans = ed.draft && ed.draft.spans || {};
      var ids = (ed.draft && ed.draft.suggestions || []).filter(function (s) { return s.group === group && !s.stale && spans[s.id]; })
        .sort(function (a, b) { return spans[a.id][0] - spans[b.id][0]; }).map(function (s) { return s.id; });
      if (!ids.length) { ed.say('This proposal has no change left in the text.'); return; }
      var sid = ids[(ids.indexOf(proposalAt[group]) + 1) % ids.length];
      proposalAt[group] = sid;
      showSuggestion(sid);
    }
    /* Shows one change: its card on the page, or its text selected. */
    function showSuggestion(sid) {
      var spans = ed.draft && ed.draft.spans || {};
      if (ed.mode === 'preview') ed.setMode('page');
      if (ed.mode === 'page' && ed.pageView && ed.pageView.active()) {
        var card = ed.root.querySelector('.pm-sugg[data-suggestion="' + CSS.escape(sid) + '"]');
        if (card) {
          card.scrollIntoView({ block: 'center' });
          card.classList.remove('flash'); void card.offsetWidth; card.classList.add('flash');
          var accept = card.querySelector('.btn-primary');
          if (accept) accept.focus({ preventScroll: true });
          return;
        }
      }
      if (!toText()) return;
      var at = placedSpans()[sid] || spans[sid];
      ed.view.dispatch({ selection: { anchor: at[0], head: at[1] }, scrollIntoView: true });
    }
    /* The suggestion the cursor is in, or touches. */
    function suggestionHere() {
      if (!ed.view) return null;
      var spans = placedSpans(), head = ed.view.state.selection.main.head;
      return Object.keys(spans).find(function (sid) { return spans[sid][0] <= head && head <= spans[sid][1]; }) || null;
    }
    function decideHere(accept) {
      var sid = suggestionHere();
      if (!sid) { ed.say('Put the cursor in a suggestion first; F7 goes to the next one.'); return; }
      ed.decide(sid, accept, false);
    }
    function goToProblem(direction) {
      if (!ed.problemLines.length) { ed.say('The draft has no problems.'); return; }
      if (!toText()) return;
      var line = ed.view.state.doc.lineAt(ed.view.state.selection.main.head).number;
      var next = direction > 0 ? ed.problemLines.find(function (l) { return l > line; }) || ed.problemLines[0]
        : ed.problemLines.filter(function (l) { return l < line; }).pop() || ed.problemLines[ed.problemLines.length - 1];
      ed.goToLine(next);
    }
    var VIEW_ORDER = Object.keys(VIEWS);
    function switchView() { ed.setMode(VIEW_ORDER[(VIEW_ORDER.indexOf(ed.mode) + 1) % VIEW_ORDER.length]); }
    /* The window's panes in order: the text and the preview; each one shown
       takes focus. */
    function panes() {
      var out = [];
      if (!ed.reading() && ed.view) out.push(ed.view.contentDOM);
      if (ed.mode !== 'text') out.push(ed.$('scroll'));
      return out;
    }
    function nextPane(direction) {
      var list = panes();
      if (!list.length) return;
      var at = list.findIndex(function (el) { return el.contains(document.activeElement) || el === document.activeElement; });
      var next = list[(at + direction + list.length) % list.length];
      if (next.tabIndex < 0 && !/^(BUTTON|INPUT|TEXTAREA)$/.test(next.tagName)) next.tabIndex = -1;
      next.focus();
    }

    return {
      selected: selected, toggleMark: toggleMark, setBlock: setBlock, insertMarkdown: insertMarkdown, pickFiles: pickFiles, toggleTag: toggleTag,
      linkDialog: linkDialog, goToSuggestion: goToSuggestion, goToProposal: goToProposal, decideHere: decideHere, goToProblem: goToProblem,
      switchView: switchView, nextPane: nextPane,
      teardown: function () { ed.root.removeEventListener('paste', pasteFiles, true); ed.root.removeEventListener('drop', dropFiles, true); }
    };
  };
})();
