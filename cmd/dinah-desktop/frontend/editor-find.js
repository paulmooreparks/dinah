/* Find and Replace for an Editor window (editor-applet.js builds the window and
   adds this part with usePart). A part is a function of the window's shared
   object ed; what it keeps for itself stays inside the function, so each
   window has its own. It returns the functions other parts call through ed
   and a teardown for what must be released when the window closes. */
(function () {
  'use strict';

  var C = window.dinahEditorCommon;
  var isMarkdown = C.isMarkdown;

  window.dinahEditorParts.find = function (ed) {
    /* ---- Find and Replace ------------------------------------------------
       One bar for every view, above the text and the page: the matches are
       marked in the text, or on the page in Visual, and the current one is
       selected there. Find keeps to what was asked: the replace row appears
       with Replace… or its button. The query and its options stay while the
       window is open, so Find Next works after the bar is closed. */
    // The query and matches are ed.finding, which the views and the text read.
    var finding = { open: false, query: '', matches: [], current: -1, options: { matchCase: false, wholeWord: false, regexp: false } }, findTimer = 0;
    var findMarks = null, setFindMarks = null;
    function findMarksField() {
      setFindMarks = ed.CM.StateEffect.define();
      findMarks = ed.CM.StateField.define({
        create: function () { return ed.CM.Decoration.none; },
        update: function (deco, tr) {
          deco = deco.map(tr.changes);
          tr.effects.forEach(function (e) { if (e.is(setFindMarks)) deco = e.value; });
          return deco;
        },
        provide: function (f) { return ed.CM.EditorView.decorations.from(f); }
      });
      return findMarks;
    }
    function escapeRegExp(s) { return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'); }
    /* The query as a regular expression, or null for none; a bad expression
       is said in the count. */
    function findRegExp() {
      var q = ed.finding.query, o = ed.finding.options;
      if (!q) return null;
      var source = o.regexp ? q : escapeRegExp(q);
      if (o.wholeWord) source = '\\b(?:' + source + ')\\b';
      try { return new RegExp(source, 'g' + (o.matchCase ? '' : 'i') + (o.regexp ? 'm' : '')); } catch (e) { return undefined; }
    }
    function matcherOf(re) {
      return function (text) {
        var out = [], m;
        re.lastIndex = 0;
        while ((m = re.exec(text))) {
          if (m[0].length === 0) { re.lastIndex++; continue; }
          out.push({ from: m.index, to: m.index + m[0].length });
        }
        return out;
      };
    }
    function findOnPage() { return ed.mode === 'page' && ed.pageView && ed.pageView.active(); }
    function findCount(words) { ed.$('find-count').textContent = words; }
    function scheduleFind() { clearTimeout(findTimer); findTimer = setTimeout(function () { runFind(true); }, 120); }
    /* Looks for the query where the person looks, marks every match and
       makes the first at or after the cursor the current one. */
    function runFind(keep) {
      clearTimeout(findTimer);
      if (!ed.view) return;
      var re = findRegExp();
      if (re === undefined) { clearFindMarks(); findCount('Not a valid expression'); return; }
      if (!re) { clearFindMarks(); findCount(''); return; }
      var matcher = matcherOf(re), was = ed.finding.current, next;
      if (findOnPage()) {
        var r = ed.pageView.find(matcher);
        ed.finding.matches = new Array(r.count);
        next = r.after;
      } else {
        var text = ed.view.state.doc.toString();
        ed.finding.matches = matcher(text);
        var head = ed.view.state.selection.main.from;
        next = ed.finding.matches.findIndex(function (m) { return m.from >= head; });
        if (keep && was >= 0 && ed.finding.matches[was] && ed.finding.matches[was].from === head) next = was;
      }
      if (!ed.finding.matches.length) { ed.finding.current = -1; drawFindMarks(); findCount('No matches'); return; }
      showMatch(next < 0 ? 0 : next);
    }
    function showMatch(i) {
      ed.finding.current = i;
      if (findOnPage()) ed.pageView.goToMatch(i);
      else if (ed.finding.matches[i]) {
        var m = ed.finding.matches[i];
        ed.view.dispatch({ selection: { anchor: m.from, head: m.to }, scrollIntoView: true });
      }
      drawFindMarks();
      findCount((i + 1) + ' of ' + ed.finding.matches.length);
    }
    function drawFindMarks() {
      if (!setFindMarks || !ed.view) return;
      var ranges = findOnPage() ? [] : ed.finding.matches.map(function (m, i) {
        return ed.CM.Decoration.mark({ class: 'cm-find-match' + (i === ed.finding.current ? ' cm-find-current' : '') }).range(m.from, m.to);
      });
      ed.view.dispatch({ effects: setFindMarks.of(ed.CM.Decoration.set(ranges, true)) });
    }
    function clearFindMarks() {
      ed.finding.matches = []; ed.finding.current = -1;
      drawFindMarks();
      if (ed.pageView) ed.pageView.find(null);
    }
    function findStep(step) {
      if (!ed.finding.query) { openFind(false); return; }
      if (!ed.finding.matches.length) { runFind(false); if (!ed.finding.matches.length) return; }
      var n = ed.finding.matches.length;
      showMatch(((ed.finding.current < 0 ? (step > 0 ? -1 : 0) : ed.finding.current) + step + n) % n);
    }
    /* What the person selected becomes the query, when it is on one line. */
    function selectedWords() {
      var s = findOnPage() ? ed.pageView.selectionText() : ed.view ? ed.selected().text : '';
      return /\n/.test(s) ? '' : s.trim();
    }
    function openFind(withReplace) {
      if (!ed.draft) return;
      if (ed.mode === 'preview') ed.setMode(ed.pageView && !ed.pageOff && isMarkdown(ed.draft) ? 'page' : 'split');
      var bar = ed.$('find'), input = ed.$('find-input');
      var words = selectedWords();
      if (words) { ed.finding.query = words; input.value = words; }
      ed.finding.open = true;
      bar.hidden = false;
      setReplaceRow(withReplace);
      runFind(false);
      input.focus();
      input.select();
    }
    function setReplaceRow(on) {
      ed.$('find-replace-row').hidden = !on;
      ed.$('find-show-replace').setAttribute('aria-pressed', String(!!on));
      if (on) ed.$('replace-input').focus();
    }
    function closeFind() {
      ed.finding.open = false;
      ed.$('find').hidden = true;
      clearFindMarks();
      if (findOnPage()) ed.pageView.focus(); else if (ed.view) ed.view.focus();
    }
    /* A replacement, with the match's own groups when the query is an
       expression ($1, $&), as String.replace reads them. */
    function replacementFor(matched) {
      var re = findRegExp(), text = ed.$('replace-input').value;
      if (!re) return text;
      re.lastIndex = 0;
      return ed.finding.options.regexp ? matched.replace(new RegExp(re.source, re.flags.replace('g', '')), text) : text;
    }
    function replaceOne() {
      if (!ed.finding.matches.length) runFind(false);
      var i = ed.finding.current;
      if (i < 0 || !ed.finding.matches.length) return;
      if (findOnPage()) ed.pageView.replaceMatch(i, replacementFor(ed.pageView.matchText(i)));
      else {
        var m = ed.finding.matches[i];
        ed.view.dispatch({ changes: { from: m.from, to: m.to, insert: replacementFor(ed.view.state.sliceDoc(m.from, m.to)) } });
      }
      runFind(false);
    }
    function replaceAll() {
      if (!ed.finding.matches.length) runFind(false);
      if (!ed.finding.matches.length) return;
      var n;
      if (findOnPage()) n = ed.pageView.replaceAllMatches(replacementFor);
      else {
        var state = ed.view.state;
        var changes = ed.finding.matches.map(function (m) { return { from: m.from, to: m.to, insert: replacementFor(state.sliceDoc(m.from, m.to)) }; });
        n = changes.length;
        ed.view.dispatch({ changes: changes });
      }
      runFind(false);
      ed.say(n === 1 ? 'Replaced 1 match.' : 'Replaced ' + n + ' matches.');
    }
    ed.$('find-input').addEventListener('input', function () { ed.finding.query = ed.$('find-input').value; scheduleFind(); });
    ed.$('find-input').addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); findStep(e.shiftKey ? -1 : 1); }
      else if (e.key === 'Escape') { e.preventDefault(); closeFind(); }
    });
    ed.$('replace-input').addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); replaceOne(); }
      else if (e.key === 'Escape') { e.preventDefault(); closeFind(); }
    });
    ed.$('find-next').addEventListener('click', function () { findStep(1); });
    ed.$('find-previous').addEventListener('click', function () { findStep(-1); });
    ed.$('find-close').addEventListener('click', closeFind);
    ed.$('find-show-replace').addEventListener('click', function () { setReplaceRow(ed.$('find-replace-row').hidden); });
    ed.$('replace-one').addEventListener('click', replaceOne);
    ed.$('replace-all').addEventListener('click', replaceAll);
    ed.root.querySelectorAll('[data-ed-find-option]').forEach(function (b) {
      b.addEventListener('click', function () {
        var on = b.getAttribute('aria-pressed') !== 'true';
        b.setAttribute('aria-pressed', String(on));
        ed.finding.options[b.dataset.edFindOption] = on;
        runFind(false);
      });
    });

    return {
      finding: finding, findMarksField: findMarksField, scheduleFind: scheduleFind, findStep: findStep, openFind: openFind,
      teardown: function () { clearTimeout(findTimer); }
    };
  };
})();
