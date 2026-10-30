/* The suggestions an agent makes, the proposals above the text and the selected passage of an Editor window (editor-applet.js builds the window and
   adds this part with usePart). A part is a function of the window's shared
   object ed; what it keeps for itself stays inside the function, so each
   window has its own. It returns the functions other parts call through ed
   and a teardown for what must be released when the window closes. */
(function () {
  'use strict';

  var C = window.dinahEditorCommon;
  var api = C.api, productName = C.productName, message = C.message;

  window.dinahEditorParts.suggestions = function (ed) {
    /* ---- Suggestions --------------------------------------------------- */

    /* A suggestion is shown in the text where it applies: the text it would
       remove struck through, and the text it would put in underlined after
       it, with Accept and Reject. The marks move with typing until the next
       view of the draft places them again. Stale ones, whose text has been
       changed since, are listed with the proposals above the text. */
    var setMarks = null, marksShown = '';
    function suggestionMarks() {
      setMarks = ed.CM.StateEffect.define();
      ed.marksField = ed.CM.StateField.define({
        create: function () { return ed.CM.Decoration.none; },
        update: function (deco, tr) {
          deco = deco.map(tr.changes);
          tr.effects.forEach(function (e) { if (e.is(setMarks)) deco = e.value; });
          return deco;
        },
        provide: function (f) { return ed.CM.EditorView.decorations.from(f); }
      });
      return ed.marksField;
    }
    /* The widget class extends CodeMirror's, which exists only once the
       bundle has loaded, so it is made then. */
    var Insertion = null;
    function insertionClass() {
      if (Insertion) return;
      Insertion = class extends ed.CM.WidgetType {
        constructor(s) { super(); this.s = s; }
        eq(other) { return other.s.id === this.s.id && other.s.after === this.s.after && !!other.s.edited === !!this.s.edited; }
        toDOM() { return insertionDOM(this.s); }
        ignoreEvent() { return true; }
      };
    }
    function insertionDOM(s) {
      var span = document.createElement('span');
      span.className = 'cm-sugg-ins';
      span.title = 'Suggested by ' + s.author + (s.edited ? ', edited by you' : '') + (s.reason ? ': ' + s.reason : '');
      var text = document.createElement('span');
      /* A paragraph's line break is shown by the line itself, so the
         buttons stay beside the words. */
      text.textContent = s.after.replace(/\n$/, '');
      span.appendChild(text);
      var buttons = [];
      [['✓', 'Accept', true], ['✗', 'Reject', false], ['✎', 'Edit', null]].forEach(function (b) {
        var button = document.createElement('button');
        button.type = 'button';
        button.className = 'cm-sugg-btn';
        button.textContent = b[0];
        button.setAttribute('aria-label', b[1] + ' this suggestion');
        button.title = b[1];
        button.addEventListener('mousedown', function (e) { e.preventDefault(); });
        button.addEventListener('click', function () { if (b[2] === null) edit(); else decide(s.id, b[2], false); });
        buttons.push(button);
        span.appendChild(button);
      });
      // Edit puts the suggested words in a box in their place; double-clicking
      // them does the same.
      function edit() {
        if (span.querySelector('.sugg-edit')) return;
        var box = suggestionEditor(s, function () { box.el.remove(); text.hidden = false; buttons.forEach(function (b) { b.hidden = false; }); });
        text.hidden = true;
        buttons.forEach(function (b) { b.hidden = true; });
        span.appendChild(box.el);
        box.focus();
      }
      text.addEventListener('dblclick', edit);
      return span;
    }
    function syncMarks() {
      if (!ed.view || !ed.draft || ed.view.state.doc.toString() !== ed.draft.text) return;
      var spans = ed.draft.spans || {}, list = (ed.draft.suggestions || []).filter(function (s) { return spans[s.id]; });
      var signature = ed.draft.revision + '|' + list.map(function (s) { return s.id + spans[s.id].join(','); }).join('|');
      if (signature === marksShown) return;
      marksShown = signature;
      insertionClass();
      var ranges = [];
      list.forEach(function (s) {
        var from = spans[s.id][0], to = spans[s.id][1];
        if (to > from) ranges.push(ed.CM.Decoration.mark({ class: 'cm-sugg-del', attributes: { 'data-suggestion': s.id } }).range(from, to));
        ranges.push(ed.CM.Decoration.widget({ widget: new Insertion(s), side: 1 }).range(to));
      });
      ed.view.dispatch({ effects: setMarks.of(ed.CM.Decoration.set(ranges, true)) });
    }
    /* The page shows the same suggestions, block by block, with the same
       Accept and Reject (page-view.js). */
    function pageMarks() {
      if (!ed.pageView || !ed.draft) return;
      var spans = ed.draft.spans || {};
      ed.pageView.suggest(ed.draft.text, (ed.draft.suggestions || []).filter(function (s) { return spans[s.id]; }).map(function (s) {
        return { id: s.id, from: spans[s.id][0], to: spans[s.id][1], after: s.after || '', author: s.author, reason: s.reason, edited: !!s.edited };
      }));
    }
    function decide(suggestion, accept, all) {
      if (ed.busy || !ed.draft) return;
      ed.busy = true;
      ed.drawHead();
      ed.flush().then(function () {
        if (ed.outside) throw new Error('Settle the change made elsewhere in ' + productName() + ' first.');
        return accept ? api().AcceptSuggestion(ed.id, ed.draft.revision, suggestion, all) : api().RejectSuggestion(ed.id, suggestion, all);
      }).then(function (v) {
        ed.draft = v;
        ed.replaceText(v.text);
        ed.schedulePreview();
        ed.say(accept ? (all ? 'The proposal is in the text.' : 'The suggestion is in the text.') : (all ? 'The proposal was rejected.' : 'The suggestion was rejected.'));
      }).catch(function (err) { ed.say(message(err), true); })
        .finally(function () { ed.busy = false; ed.drawHead(); });
    }
    /* Editing what a suggestion proposes before deciding on it: the new
       text is changed in the draft's record of the suggestion, the text
       itself stays until the suggestion is accepted. */
    function revise(suggestion, after) {
      if (ed.busy || !ed.draft) return Promise.reject(new Error('The Editor is busy; try again in a moment.'));
      ed.busy = true;
      ed.drawHead();
      return Promise.resolve().then(function () { return api().ReviseSuggestion(ed.id, suggestion, after); }).then(function (v) {
        ed.draft = v;
        marksShown = '';
        syncMarks();
        pageMarks();
        drawProposals();
        ed.say('Your wording is in the suggestion; accept it to put it in the text.');
      }).finally(function () { ed.busy = false; ed.drawHead(); });
    }
    /* The box a suggestion's new text is edited in, on the page and in the
       text alike: its Markdown, Save (Ctrl+Enter) and Cancel (Escape).
       done() is called when the box is to go. */
    function suggestionEditor(s, done) {
      var box = document.createElement('span');
      box.className = 'sugg-edit';
      var area = document.createElement('textarea');
      area.className = 'form-textarea sugg-edit-text';
      area.value = s.after.replace(/\n$/, '');
      area.setAttribute('aria-label', 'The suggested text, as Markdown');
      var row = document.createElement('span');
      row.className = 'sugg-edit-actions';
      var save = document.createElement('button');
      save.type = 'button'; save.className = 'btn btn-sm btn-primary'; save.textContent = 'Save';
      var cancel = document.createElement('button');
      cancel.type = 'button'; cancel.className = 'btn btn-sm'; cancel.textContent = 'Cancel';
      var note = document.createElement('span');
      note.className = 'form-help'; note.textContent = 'Ctrl+Enter saves, Escape cancels.';
      row.append(save, cancel, note);
      box.append(area, row);
      function keep() {
        // The suggestion's own line ending stays, so a paragraph stays one.
        var text = area.value + (/\n$/.test(s.after) && !/\n$/.test(area.value) ? '\n' : '');
        if (text === s.after) { done(); return; }
        save.disabled = cancel.disabled = true;
        revise(s.id, text).then(done, function (err) { save.disabled = cancel.disabled = false; ed.say(message(err), true); });
      }
      save.addEventListener('click', keep);
      cancel.addEventListener('click', function () { done(); });
      area.addEventListener('keydown', function (e) {
        e.stopPropagation();
        if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); keep(); }
        else if (e.key === 'Escape') { e.preventDefault(); done(); }
      });
      return { el: box, focus: function () { area.focus(); area.setSelectionRange(area.value.length, area.value.length); } };
    }
    /* The proposals above the text: each group of suggestions made together
       is a line saying who suggests how many changes where, with Go To,
       Accept All and Reject All. A sentence its author flagged for the
       person is listed on its line. Stale changes are counted so none is
       lost unannounced. */
    function proposalAuthor(author) { return String(author || '').replace(/\s*\(chat [^)]*\)\s*$/, ''); }
    /* The heading the proposal's first change is under. */
    function proposalHeading(at) {
      if (at == null || !ed.draft) return '';
      var before = ed.draft.text.slice(0, at), heading = '', m, re = /^#{1,6}[ \t]+(.+?)[ \t]*#*[ \t]*$/gm;
      while ((m = re.exec(before))) heading = m[1];
      return heading;
    }
    function changesWords(n) { return n === 1 ? 'one change' : n + ' changes'; }
    function proposalGroups() {
      var spans = ed.draft && ed.draft.spans || {}, groups = {}, order = [];
      (ed.draft && ed.draft.suggestions || []).forEach(function (s) {
        var g = groups[s.group];
        if (!g) { g = groups[s.group] = { id: s.group, first: s, live: 0, stale: 0, at: null, flags: s.flags || [] }; order.push(s.group); }
        if (s.stale) { g.stale++; return; }
        g.live++;
        if (spans[s.id] && (g.at == null || spans[s.id][0] < g.at)) g.at = spans[s.id][0];
      });
      return order.map(function (k) { groups[k].heading = proposalHeading(groups[k].at); return groups[k]; });
    }
    function proposalButton(text, run, primary) {
      var b = document.createElement('button');
      b.type = 'button';
      b.className = 'btn btn-sm' + (primary ? ' btn-primary' : '');
      b.textContent = text;
      b.addEventListener('click', run);
      return b;
    }
    function proposalLine(g) {
      var row = document.createElement('div');
      row.className = 'editor-proposal';
      row.dataset.proposal = g.id;
      var text = document.createElement('span');
      text.className = 'editor-proposal-text';
      var where = g.heading ? '"' + g.heading + '"' : '';
      var count = changesWords(g.live) + (g.stale ? (g.live ? ', and ' : '') + (g.stale === 1 ? 'one that no longer fits the text' : g.stale + ' that no longer fit the text') : '');
      text.textContent = proposalAuthor(g.first.author) + ' suggests ' + count + (where ? ' in ' + where : '') + (g.first.reason ? ': ' + g.first.reason : '') + '.';
      row.appendChild(text);
      if (g.flags.length) row.appendChild(window.dinahBrowserList.badge({ text: 'Flagged', kind: 'warn', label: 'The proposal has sentences its author flagged as possibly wrong or contradictory' }));
      if (g.live) {
        var go = proposalButton('Go To', function () { ed.goToProposal(g.id); });
        go.title = g.live > 1 ? 'Show the next of these changes' : 'Show this change';
        row.appendChild(go);
        row.appendChild(proposalButton('Accept All', function () { decide(g.first.id, true, true); }, true));
      }
      row.appendChild(proposalButton('Reject All', function () { decide(g.first.id, false, true); }));
      if (g.flags.length) {
        var ul = document.createElement('ul');
        ul.className = 'editor-proposal-flags';
        g.flags.forEach(function (f) { var li = document.createElement('li'); li.textContent = (f.id ? f.id + ': ' : '') + f.reason; ul.appendChild(li); });
        row.appendChild(ul);
      }
      return row;
    }
    function drawProposals() {
      var holder = ed.$('proposals'), groups = proposalGroups();
      holder.replaceChildren();
      holder.hidden = !groups.length;
      groups.forEach(function (g) { holder.appendChild(proposalLine(g)); });
    }

    /* ---- The selected passage ------------------------------------------- */

    /* The selection is widened to whole lines, which are paragraphs in
       Markdown, because suggestions are placed paragraph by paragraph. In
       Visual view the selection is the page's: its blocks are written as
       Markdown and found in the text, so the passage is the text's own, as
       in Text and Split. Writing them takes a call, so the passage is kept
       here and the commands drawn again when it arrives. */
    var pagePassage = '', pagePassageAsked = 0;
    function pageSelectionChanged() {
      var blocks = ed.mode === 'page' && ed.pageView && ed.pageView.active() ? ed.pageView.selectedBlocks() : null;
      var asked = ++pagePassageAsked;
      if (!blocks) { if (pagePassage) { pagePassage = ''; ed.commandsChanged(); ed.scheduleContext(); } return; }
      Promise.resolve(api().PageDocumentMarkdown(blocks)).then(function (md) {
        if (ed.disposed || asked !== pagePassageAsked) return;
        md = String(md || '').replace(/^\n+/, '');
        var text = ed.view ? ed.view.state.doc.toString() : '';
        var passage = md.trim() && text.indexOf(md) >= 0 ? md : md.trim() && text.indexOf(md.replace(/\n+$/, '')) >= 0 ? md.replace(/\n+$/, '') + '\n' : '';
        if (passage === pagePassage) return;
        pagePassage = passage;
        ed.commandsChanged(); ed.scheduleContext();
      }).catch(function () { /* The passage stays as it was. */ });
    }
    function selectedPassage() {
      if (ed.mode === 'page' && ed.pageView && ed.pageView.active()) return pagePassage;
      if (!ed.view) return '';
      var sel = ed.view.state.selection.main;
      if (sel.empty) return '';
      var doc = ed.view.state.doc, from = doc.lineAt(sel.from).from, last = doc.lineAt(sel.to > sel.from && doc.lineAt(sel.to).from === sel.to ? sel.to - 1 : sel.to);
      var to = last.to < doc.length ? last.to + 1 : last.to;
      return doc.sliceString(from, to);
    }

    return { marksField: null, suggestionMarks: suggestionMarks, syncMarks: syncMarks, pageMarks: pageMarks, decide: decide, suggestionEditor: suggestionEditor, drawProposals: drawProposals, pageSelectionChanged: pageSelectionChanged, selectedPassage: selectedPassage };
  };
})();
