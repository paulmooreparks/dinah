/* The views of an Editor window and the share of Split the text takes (editor-applet.js builds the window and
   adds this part with usePart). A part is a function of the window's shared
   object ed; what it keeps for itself stays inside the function, so each
   window has its own. It returns the functions other parts call through ed
   and a teardown for what must be released when the window closes. */
(function () {
  'use strict';

  var C = window.dinahEditorCommon;
  var VIEWS = C.VIEWS, isMarkdown = C.isMarkdown;

  window.dinahEditorParts.views = function (ed) {
    /* ---- Views --------------------------------------------------------- */

    /* Visual and Preview show the page without the text. */
    function reading() { return ed.mode === 'preview' || ed.mode === 'page'; }
    /* Whether the page is the editor: in Visual, and in Split beside the
       text, where what is typed on either side reaches the other. A page
       that could not be built stays off until Visual is chosen again. */
    // pageOff and pageEnabled are returned below and written through ed.
    function pageWanted() { return !!ed.pageView && !ed.pageOff && (ed.mode === 'page' || ed.mode === 'split'); }
    function setMode(next, quiet) {
      var hadText = ed.$('text').contains(document.activeElement), hadPreview = ed.$('scroll').contains(document.activeElement);
      ed.mode = VIEWS[next] ? next : 'split';
      if (ed.views.indexOf(ed.mode) < 0) ed.mode = ed.views.indexOf('split') >= 0 ? 'split' : ed.views[0];
      // Only Markdown has a page to edit.
      if (ed.mode === 'page' && ed.draft && !isMarkdown(ed.draft)) ed.mode = 'text';
      if (ed.mode === 'page') ed.pageOff = false;
      ed.$('panes').dataset.view = ed.mode;
      if (ed.mode === 'split') ed.$('split').removeAttribute('data-split-pane');
      else ed.$('split').setAttribute('data-split-pane', ed.mode === 'text' ? 'first' : 'second');
      // Leaving the page puts its last changes into the text, and the
      // preview is drawn from that.
      var wantPage = pageWanted();
      // The formatting toolbar is for every view that edits: the page, the
      // text and both.
      if (ed.pageView) ed.pageEnabled = ed.pageView.enable(wantPage).then(function () { if (!wantPage && ed.mode !== 'text') ed.schedulePreview(0); ed.commandsChanged(); });
      // Focus stays in the window when the pane that had it is hidden, so
      // the window's keys go on working.
      if (reading() && hadText) ed.$('scroll').focus();
      if (ed.mode === 'text' && hadPreview && ed.view) ed.view.focus();
      ed.commandsChanged();
      // The find bar follows the view: its matches are marked where the
      // person now looks.
      if (ed.finding.open) ed.scheduleFind();
      if (wantPage || (ed.mode !== 'text' && !ed.pageView)) ed.schedulePreview(0);
      if (ed.view && !reading()) ed.view.requestMeasure();
      if (!quiet) ed.root.dispatchEvent(new CustomEvent('pudl:applet-change', { bubbles: true, detail: { state: ed.mode } }));
    }
    /* Preview is for reading. Opening the document again to work on it
       switches to Split; typing a character in the preview switches to Split with the cursor
       at the passage at the top of the preview. Preview has no caret, so the
       character itself is not typed; Space scrolls, as it does in any page,
       and keys pressed on the window's buttons and fields are theirs. */
    ed.root.addEventListener('dinah:editor-work', function () { if (ed.mode === 'preview') setMode('split'); });
    /* A host's Check button: the problems are drawn again from the draft
       as it is and the list is opened. */
    ed.root.addEventListener('dinah:editor-check', function () {
      ed.problemsOpen = true;
      ed.$('problems').hidden = !ed.$('problems').children.length;
      ed.$('problems-btn').setAttribute('aria-expanded', String(!ed.$('problems').hidden));
      if (ed.mode === 'text') { ed.drawPreview(); return; }
      ed.schedulePreview(0);
    });
    ed.root.addEventListener('keydown', function (e) {
      if (ed.mode !== 'preview' || !ed.view || e.ctrlKey || e.metaKey || e.altKey || e.key.length !== 1 || e.key === ' ') return;
      if (!ed.$('scroll').contains(e.target) || e.target.closest('a, button, input, textarea, select, [contenteditable="true"]')) return;
      e.preventDefault();
      var top = ed.$('scroll').getBoundingClientRect().top;
      var shown = Array.from(ed.$('page').querySelectorAll('.doc-block')).find(function (b) { return b.getBoundingClientRect().bottom > top; });
      var line = shown && +shown.dataset.line || 1;
      setMode('split');
      ed.goToLine(line);
      ed.say('The text is shown beside the page, so you can type where you were reading.');
    });

    /* The text's share of Split is one preference for every Editor, kept as
       a fraction of the width, so it holds when the window is wider or
       narrower. */
    var widthKey = 'dinah-editor-text-share';
    try {
      var share = window.dinahUIState && window.dinahUIState.get(widthKey);
      if (typeof share === 'number' && share > 0 && share < 1) ed.$('split').style.setProperty('--split-a', (share * 100).toFixed(2) + '%');
    } catch (err) { /* A view preference is optional. */ }
    ed.$('split-handle').addEventListener('pudl:split', function (e) {
      var whole = ed.$('split').getBoundingClientRect().width;
      var share = e.detail.size == null || !whole ? undefined : e.detail.size / whole;
      if (share !== undefined) ed.$('split').style.setProperty('--split-a', (share * 100).toFixed(2) + '%');
      if (window.dinahUIState) window.dinahUIState.set(widthKey, share);
    });

    return { pageOff: false, pageEnabled: Promise.resolve(), reading: reading, pageWanted: pageWanted, setMode: setMode };
  };
})();
