/* The preview and the problems list of an Editor window (editor-applet.js builds the window and
   adds this part with usePart). A part is a function of the window's shared
   object ed; what it keeps for itself stays inside the function, so each
   window has its own. It returns the functions other parts call through ed
   and a teardown for what must be released when the window closes. */
(function () {
  'use strict';

  var C = window.dinahEditorCommon;
  var api = C.api, message = C.message, originRef = C.originRef;

  window.dinahEditorParts.preview = function (ed) {
    /* ---- Preview ------------------------------------------------------- */

    var previewTimer = 0, previewRequest = 0;
    function schedulePreview(delay) {
      if (ed.mode === 'text' || ed.disposed) return;
      clearTimeout(previewTimer);
      previewTimer = setTimeout(drawPreview, delay === undefined ? 250 : delay);
    }
    /* While the page is the editor, only the problems are taken from the
       preview. */
    function drawPreview() {
      var generation = ++previewRequest, scroll = ed.$('scroll'), top = scroll.scrollTop;
      function current() { return !ed.disposed && generation === previewRequest; }
      return Promise.resolve().then(function () { return api().PreviewDraft(ed.id); }).then(function (p) {
        if (!current()) return;
        if (ed.pageWanted()) { showProblems(p.problems || []); return; }
        var page = ed.$('page'), extra = [];
        function update() { if (current()) showProblems((p.problems || []).concat(extra)); }
        window.dinahDocumentPreview.renderBlocks(page, p.blocks, ed.key, {
          current: current,
          diagram: function (diagram, result) {
            if (result.error !== undefined) { extra.push({ line: diagram.line, severity: 'error', message: 'The ' + diagram.language + ' diagram could not be drawn: ' + result.error }); update(); }
          }
        });
        window.dinahDocumentPreview.loadAttachments(page, originRef(ed.draft), { draft: ed.id, current: current, problem: function (problem) { extra.push(problem); update(); } });
        scroll.scrollTop = top;
        update();
      }).catch(function (err) {
        if (current()) ed.$('page').textContent = 'The preview could not be drawn: ' + message(err);
      });
    }
    function showProblems(problems) {
      var list = ed.$('problems'), btn = ed.$('problems-btn');
      var sorted = problems.slice().sort(function (a, b) { return a.line - b.line; });
      ed.problemLines = sorted.map(function (p) { return p.line; }).filter(Boolean);
      btn.hidden = !sorted.length;
      btn.textContent = sorted.length === 1 ? '1 Problem' : sorted.length + ' Problems';
      btn.classList.toggle('btn-danger', sorted.some(function (p) { return p.severity === 'error'; }));
      list.replaceChildren();
      sorted.forEach(function (p) {
        var li = document.createElement('li');
        li.innerHTML = '<button type="button"><span class="doc-line"></span><span class="badge"></span><span class="doc-msg"></span></button>';
        li.querySelector('.doc-line').textContent = p.line ? 'Line ' + p.line : '';
        var badge = li.querySelector('.badge');
        badge.textContent = p.severity;
        badge.classList.add(p.severity === 'error' ? 'danger' : p.severity === 'warning' ? 'warn' : 'info');
        li.querySelector('.doc-msg').textContent = p.message;
        li.firstChild.addEventListener('click', function () { if (p.line) goToLine(p.line); });
        list.appendChild(li);
      });
      list.hidden = !sorted.length || !ed.problemsOpen;
      btn.setAttribute('aria-expanded', String(!list.hidden));
    }
    ed.$('problems-btn').addEventListener('click', function () {
      ed.problemsOpen = !ed.problemsOpen;
      ed.$('problems').hidden = !ed.problemsOpen;
      ed.$('problems-btn').setAttribute('aria-expanded', String(ed.problemsOpen));
    });

    /* A line of the text, shown in the text view; from the preview, a
       double-click on a block goes to the block's line. */
    function goToLine(line) {
      if (!ed.view) return;
      if (ed.reading()) ed.setMode('split');
      var at = ed.view.state.doc.line(Math.max(1, Math.min(line, ed.view.state.doc.lines)));
      ed.view.dispatch({ selection: { anchor: at.from }, scrollIntoView: true });
      ed.view.focus();
    }
    ed.$('page').addEventListener('dblclick', function (e) {
      // In Page view a double-click selects a word.
      if (ed.mode === 'page') return;
      var block = e.target.closest('.doc-block');
      if (block && block.dataset.line) goToLine(+block.dataset.line);
    });
    ed.$('page').addEventListener('click', function (e) {
      var link = e.target.closest('a[href]');
      if (!link) return;
      var href = link.getAttribute('href');
      if (href.charAt(0) === '#') return;
      e.preventDefault();
      if (/^https?:\/\//i.test(href) && window.runtime && window.runtime.BrowserOpenURL) window.runtime.BrowserOpenURL(href);
    });

    return {
      problemLines: [], schedulePreview: schedulePreview, drawPreview: drawPreview, goToLine: goToLine,
      teardown: function () { clearTimeout(previewTimer); }
    };
  };
})();
