/* Each document keeps its own preview, scroll position and file watcher. */
(function () {
  'use strict';
  function renderBlocks(page, blocks, key, options) {
    options = options || {};
    var pending = [];
    page.replaceChildren();
    (blocks || []).forEach(function (b) {
      var block = document.createElement('div');
      block.className = 'doc-block'; block.dataset.line = b.line;
      // The preview keeps the HTML written in the document, which may have
      // come from anyone, so it is made safe before it is shown
      // (safe-html.js).
      block.replaceChildren(window.dinahHTML.fragment(b.html));
      window.dinahWorkbench.namespaceSVG(block); page.appendChild(block);
      (b.diagrams || []).forEach(function (diagram) {
        if (!['d2', 'mermaid'].includes(diagram.language)) return;
        var target = block.querySelector('[data-preview-diagram="' + CSS.escape(diagram.id) + '"]');
        if (!target || !window.dinahDiagrams) return;
        pending.push(window.dinahDiagrams.draw(target, diagram.language, diagram.source, function () {
          return page.contains(block) && (!options.current || options.current());
        }, function (result) {
          if (options.diagram) options.diagram(diagram, result);
          if (options.changed) options.changed();
        }, true));
      });
    });
    var anchors = new Map();
    page.querySelectorAll('[id]').forEach(function (el) {
      if (el.closest('svg')) return;
      var original = el.id; el.id = key + '-preview-' + original; anchors.set(original, el.id);
    });
    page.querySelectorAll('a[href^="#"]').forEach(function (link) {
      var original;
      try { original = decodeURIComponent(link.getAttribute('href').slice(1)); } catch (_) { return; }
      if (anchors.has(original)) link.setAttribute('href', '#' + encodeURIComponent(anchors.get(original)));
    });
    return Promise.all(pending);
  }
  function loadAttachments(page, ref, options) {
    options = options || {};
    page.querySelectorAll('img[data-attachment]').forEach(function (img) {
      var name = img.dataset.attachment, block = img.closest('.doc-block');
      function current() { return page.contains(img) && (!options.current || options.current()); }
      img.addEventListener('load', function () {
        if (!current()) return;
        if (window.dinahMediaViewer) window.dinahMediaViewer.enhance(img);
        if (options.changed) options.changed();
      }, { once: true });
      img.addEventListener('error', function () {
        if (!current()) return;
        var broken = document.createElement('span'); broken.className = 'dinah-broken-image';
        broken.textContent = 'Image could not be loaded: ' + name;
        var frame = img.parentElement;
        if (frame && frame.classList.contains('media-image-frame')) frame.replaceWith(broken);
        else img.replaceWith(broken);
        if (options.problem) options.problem({ line: Number(block && block.dataset.line) || 1, severity: 'error', message: broken.textContent });
        if (options.changed) options.changed();
      }, { once: true });
      // An Editor's draft shows the files added to it as well as its
      // document's, so it asks for them by the draft.
      img.src = options.draft ? '/draft-file?' + new URLSearchParams({ draft: options.draft, name: name }) :
        ref ? '/resource-attachment?' + new URLSearchParams({ workspaceId: ref.workspaceId, sourceId: ref.sourceId, path: ref.path || '', name: name }) :
        '/doc-file?' + new URLSearchParams({ workspace: options.workspace, doc: options.path, name: name });
    });
  }
  window.dinahDocumentPreview = { renderBlocks: renderBlocks, loadAttachments: loadAttachments };
  var documentApplet = {
    title: 'Resource', glyph: 'document',
    init: function (root, opts, item) {
      var win = root.closest('[data-win]'), key = opts.key || win.dataset.win;
      var doc = Object.assign({}, item), disposed = false, request = 0;
      var resourceRef = window.dinahWorkbench.resourceRef(item), resource = null;
      var problemsOpen = opts.state === 'problems', ui = window.dinahUIState;
      delete doc.kind;
      doc.title = doc.title || 'Resource';
      var $ = function (name) { return root.querySelector('[data-doc-' + name + ']'); };
      root.classList.add('document-applet');
      // The row's controls are drawn from the window's commands (PUDL's
      // commands), which agents see too.
      root.innerHTML = '<div class="toolbar doc-head" role="toolbar" aria-label="Resource"><button type="button" class="btn btn-sm" data-command="refresh" data-doc-refresh></button>' +
        '<button type="button" class="btn btn-sm" data-command="open-external" data-doc-external></button>' +
        '<button type="button" class="btn btn-sm doc-problems-btn" data-command="show-problems" data-doc-problems-btn></button></div>' +
        '<p class="doc-meta" data-doc-meta></p><ol class="doc-problems" data-doc-problems hidden></ol>' +
        '<div class="doc-scroll" data-doc-scroll><article class="doc-page" data-doc-page aria-label="Document Preview"></article></div>';
      $('problems').id = key + '-problems';
      $('problems-btn').setAttribute('aria-controls', $('problems').id);
      var external = '', problemCount = 0, problemErrors = false;
      $('page').textContent = 'Drawing the document…';
      $('scroll').dataset.uiScroll = 'document';
      if (ui) ui.watchScroll($('scroll'), 'dinah-scroll:' + key + ':document', {defer:true});
      function restoreScroll() {
        if (ui && !disposed) { ui.refreshScroll($('scroll')); ui.scrollReady($('scroll')); }
      }
      $('page').addEventListener('click', function (e) {
        var link = e.target.closest('a[href]');
        if (!link || !$('page').contains(link)) return;
        var href = link.getAttribute('href');
        if (href.startsWith('#')) {
          e.preventDefault();
          var id;
          try { id = decodeURIComponent(href.slice(1)); } catch (_) { return; }
          var target = $('page').querySelector('[id="' + CSS.escape(id) + '"]');
          if (target) target.scrollIntoView({block:'start'});
          return;
        }
        // A link that is not to the web never navigates the app's own page.
        e.preventDefault();
        if (!/^https?:\/\//i.test(href)) return;
        if (window.runtime && window.runtime.BrowserOpenURL) window.runtime.BrowserOpenURL(href);
        else window.open(href, '_blank', 'noopener,noreferrer');
      });

      const api = window.dinahHelpers.app;
      function goToLine(line) {
        var best = null;
        $('page').querySelectorAll('.doc-block').forEach(function (b) { if (+b.dataset.line <= line) best = b; });
        best = best || $('page').querySelector('.doc-block');
        if (!best) return;
        best.scrollIntoView({ block: 'center' });
        best.classList.remove('flash'); void best.offsetWidth; best.classList.add('flash');
      }
      function draw(v) {
        resource = v.resource;
        doc.title = v.title || (resource && resource.title) || doc.title;
        if (!opts.embedded) window.pudlWindows.retitle(key, doc.title);
        var page = $('page');
        page.classList.toggle('media-preview', ['image', 'd2', 'mermaid'].includes(v.format));
        page.dataset.previewFormat = v.format || '';
        page.setAttribute('aria-label', v.format === 'image' ? 'Image Preview' : ['d2', 'mermaid'].includes(v.format) ? 'Diagram Preview' : 'Document Preview');
        var diagramProblems = new Map(), attachmentProblems = [], generation = request;
        function current() { return !disposed && generation === request; }
        function updateProblems() { if (current()) showProblems((v.problems || []).concat(Array.from(diagramProblems.values()), attachmentProblems)); }
        renderBlocks(page, v.blocks, key, {
          current: function () { return !disposed && generation === request; }, changed: restoreScroll,
          diagram: function (diagram, result) {
            if (result.error !== undefined) diagramProblems.set(diagram.id, { line: diagram.line, severity: 'error', message: 'The ' + diagram.language + ' diagram could not be drawn: ' + result.error });
            else diagramProblems.delete(diagram.id);
            updateProblems();
          }
        });
        var actions = resource && resource.actions || [];
        external = actions.includes('open-external') ? 'Open Externally' : '';
        loadAttachments(page, resource && resource.ref || resourceRef, {
          current: current, changed: restoreScroll,
          problem: function (problem) { attachmentProblems.push(problem); updateProblems(); }
        });
        updateProblems();
        // The document viewer embedded in another window, such as a skill's,
        // has no request bar.
        if ($('request')) $('request').hidden = true;
      }
      function showProblems(problems) {
        var probs = problems.slice().sort(function (a, b) { return a.line - b.line; });
        var list = $('problems');
        problemCount = probs.length;
        problemErrors = probs.some(function (p) { return p.severity === 'error'; });
        list.replaceChildren();
        probs.forEach(function (p) {
          var li = document.createElement('li');
          li.innerHTML = '<button type="button"><span class="doc-line"></span><span class="badge"></span><span class="doc-msg"></span></button>';
          li.querySelector('.doc-line').textContent = 'Line ' + p.line;
          var badge = li.querySelector('.badge');
          badge.textContent = p.severity;
          badge.classList.add(p.severity === 'error' ? 'danger' : p.severity === 'warning' ? 'warn' : 'info');
          li.querySelector('.doc-msg').textContent = p.message;
          li.firstChild.addEventListener('click', function () { goToLine(p.line); });
          list.appendChild(li);
        });
        list.hidden = !probs.length || !problemsOpen;
        if (window.pudlCommands) window.pudlCommands.changed();
        resourceLabel();
      }
      function resourceLabel() {
        if (disposed) return;
        var ref = resource && resource.ref || resourceRef;
        $('meta').textContent = (window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.describe(ref) : ref.workspaceId + ' · ' + ref.sourceId) + ' · ' + doc.title + (resource && resource.revision ? ' · revision ' + resource.revision : '');
      }
      document.addEventListener('dinah:workspace-resource-labels', resourceLabel);
      async function load(keepScroll) {
        if (!api()) return;
        var generation = ++request, top = $('scroll').scrollTop;
        try {
          var v = await api().PreviewWorkspaceResource(resourceRef);
          if (disposed || generation !== request) return;
          draw(v);
          if (keepScroll) $('scroll').scrollTop = top;
          restoreScroll();
        } catch (err) {
          if (!disposed && generation === request) $('meta').textContent = 'The document could not be drawn: ' + err;
        }
      }
      function refresh() { load(true); }
      function toggleProblems() {
        problemsOpen = !problemsOpen;
        $('problems').hidden = !problemsOpen;
        if (opts.embedded) { if (opts.changed) opts.changed(problemsOpen ? 'problems' : null); }
        else root.dispatchEvent(new CustomEvent('pudl:applet-change', {bubbles:true, detail:{state:problemsOpen ? 'problems' : null}}));
      }
      function openExternal() {
        return Promise.resolve().then(function () { return api().OpenWorkspaceResource(resourceRef); }).catch(function (error) { $('meta').textContent = String(error); });
      }
      function commandTable() {
        return [
          { id: 'refresh', label: 'Refresh', glyph: 'reload', run: refresh },
          { id: 'open-external', label: external || 'Open Externally', glyph: 'new-window', hidden: !external, run: openExternal },
          { id: 'show-problems', label: problemCount === 1 ? '1 Problem' : problemCount + ' Problems', tone: problemErrors ? 'danger' : 'warn', hidden: !problemCount, checked: problemsOpen, run: toggleProblems }
        ];
      }
      var unregisterCommands = window.pudlCommands ? window.pudlCommands.register(root, commandTable) : function () {};
      var ready = load(false);
      return {
        ready: ready,
        state: function () { return problemsOpen ? 'problems' : null; },
        setState: function (value) {
          problemsOpen = value === 'problems';
          $('problems').hidden = !problemsOpen || !problemCount;
          if (window.pudlCommands) window.pudlCommands.changed();
        },
        commands: function () { return ['refresh', 'show-problems', 'open-external']; },
        destroy: function () {
          disposed = true;
          ++request;
          unregisterCommands();
          document.removeEventListener('dinah:workspace-resource-labels', resourceLabel);
        }
      };
    }
  };
  window.dinahWorkbench.register('workspace-resource', documentApplet);
  /* A document opens in the Editor. A document window kept in a layout saved
     before then opens the Editor for its file and closes itself. */
  window.dinahWorkbench.register('document', { title: 'Document', glyph: 'document', init: function (root, opts, item) {
    var key = opts.key || root.closest('[data-win]').dataset.win;
    root.textContent = 'Opening the document in the Editor…';
    Promise.resolve().then(function () { return window.dinahHelpers.app().DocumentRefAt(item.workspace, item.path); })
      .then(function (ref) { return window.dinahEditor.openResource(ref, 'read'); })
      .catch(function (err) { if (window.dinahHost) window.dinahHost.toast(String(err && err.message || err), 'danger'); })
      .then(function () { if (!opts.embedded) window.pudlWindows.close(key); });
    return {};
  } });
})();
