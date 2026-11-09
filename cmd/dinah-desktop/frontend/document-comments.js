/* The comments on a document, in a margin beside its window's applet
   (docs/design/document-comments.md). A document's comment threads are
   kept beside it, never in its text, so no reader of the document sees
   them. Each is a card: who said what, Reply, Resolve, and for a resolved
   thread Reply to Reopen; a card's quote goes to the words in the text.
   New Comment… opens a thread on the words selected in the applet, or on
   the line under the caret. Which comments are shown is one preference for
   every window: the open ones, all of them, or none; the count is shown
   whatever the choice.

   A document whose source keeps comments of its own (a page on a site)
   brings them in when the window opens and once a minute, and sends what
   was written here with the next save or publish; a thread the site has
   not seen yet says so. */
(function () {
  'use strict';
  var SHOWN_KEY = 'dinah-document-comments', shown = 'open';
  try { var kept = window.dinahUIState && window.dinahUIState.get(SHOWN_KEY); if (kept === 'off' || kept === 'all') shown = kept; } catch (err) { /* A view preference is optional. */ }
  var listeners = [];
  function setShownAll(which) {
    shown = which;
    try { if (window.dinahUIState) window.dinahUIState.set(SHOWN_KEY, which === 'open' ? undefined : which); } catch (err) { /* A view preference is optional. */ }
    listeners.forEach(function (fn) { fn(); });
  }
  function node(tag, className, text) {
    var el = document.createElement(tag);
    if (className) el.className = className;
    if (text != null) el.textContent = text;
    return el;
  }
  function when(at) {
    var d = new Date(at);
    if (isNaN(d.getTime())) return '';
    return window.dinahRegional && window.dinahRegional.format ? window.dinahRegional.format(d, 'dayTime') : d.toLocaleString();
  }
  var KIND = { question: 'Question', finding: 'Finding', comment: 'Comment' };
  function message(err) { return String(err && err.message || err).replace(/^Error: /, ''); }

  /* mount puts the margin into aside for the draft, and returns its
     controls. options: api() (the App), host() (dinahHost), flush() (the
     applet's text into the draft), applet (the element the applet draws
     in), changed() (the window's commands changed). */
  function mount(aside, options) {
    var api = options.api, host = options.host, draftId = options.draftId;
    var list = [], composing = null, replying = {}, currentId = null, disposed = false, timer = 0, revealed = null, loading = null;
    var head = node('div', 'document-comments-head'), count = node('span', 'document-comments-count'), cards = node('div', 'document-comments-list');
    var newButton = node('button', 'btn btn-sm', 'New Comment…'); newButton.type = 'button'; newButton.addEventListener('click', function () { newComment(); });
    head.append(count, newButton);
    aside.append(head, cards);
    aside.hidden = true;

    function visible() {
      if (shown === 'off') return [];
      return list.filter(function (t) { return shown === 'all' || !t.resolved || t.id === revealed; });
    }
    function load() {
      if (disposed || !api().DraftThreads) return Promise.resolve();
      if (loading) return loading;
      loading = Promise.resolve(api().DraftThreads(draftId)).then(function (got) {
        if (disposed) return;
        var was = current(), now = (got || []).find(function (t) { return was && t.id === was.id; });
        if (shown === 'off' && now && (now.messages || []).length > (was.messages || []).length) setShownAll('open');
        list = got || [];
        draw();
        if (options.changed) options.changed();
      }).catch(function (err) { if (host()) host().toast('The comments could not be read: ' + message(err), 'danger'); }).finally(function () { loading = null; });
      return loading;
    }
    function bringIn() {
      if (disposed || !api().BringInDocumentComments) return Promise.resolve();
      return Promise.resolve(api().BringInDocumentComments(draftId)).then(function (n) { if (n > 0) return load(); }).catch(function () { /* The next check tries again. */ });
    }
    function current() { return list.find(function (t) { return t.id === currentId; }) || null; }
    function draw() {
      if (disposed) return;
      var items = visible();
      var open = list.filter(function (t) { return !t.resolved; }).length, resolved = list.length - open;
      count.textContent = list.length ? open + (open === 1 ? ' open comment' : ' open comments') + (resolved ? ', ' + resolved + ' resolved' : '') : 'No comments';
      aside.hidden = !(list.length || composing);
      cards.replaceChildren();
      if (composing) cards.appendChild(composer());
      items.forEach(function (t) { cards.appendChild(card(t)); });
      if (!items.length && !composing && list.length) cards.appendChild(node('p', 'form-help', shown === 'off' ? 'Comments are hidden. Show them from the Document menu.' : 'Every comment is resolved. Show all comments from the Document menu to read them.'));
    }
    function button(label, primary, fn) {
      var b = node('button', 'btn btn-sm' + (primary ? ' btn-primary' : ''), label);
      b.type = 'button'; b.addEventListener('click', fn);
      return b;
    }
    function card(t) {
      var c = node('section', 'card document-comment');
      c.dataset.thread = t.id;
      if (t.resolved) c.dataset.resolved = 'true';
      if (t.id === currentId) c.dataset.current = 'true';
      var headRow = node('div', 'document-comment-head');
      var kind = node('span', 'badge' + (t.kind === 'finding' && t.severity === 'MUST' ? ' danger' : t.kind === 'question' ? ' accent' : ''), (KIND[t.kind] || 'Comment') + (t.severity ? ' · ' + t.severity : ''));
      var state = node('span', 'document-comment-state', t.resolved ? 'Resolved' : t.unsent ? 'Not sent yet' : t.waiting ? 'Waiting on you' : t.due ? 'For the agent' : 'Open');
      headRow.append(kind, state);
      c.appendChild(headRow);
      var quote = node('button', 'document-comment-quote', '“' + (t.quote.length > 140 ? t.quote.slice(0, 140) + '…' : t.quote) + '”');
      quote.type = 'button'; quote.title = 'Go to the passage';
      quote.addEventListener('click', function () { goTo(t); });
      c.appendChild(quote);
      if (t.lost) c.appendChild(node('p', 'document-comment-lost', 'The words this comment is about are no longer in the text.'));
      (t.messages || []).forEach(function (m) {
        var p = node('p', 'document-comment-message');
        var who = node('strong', '', m.author || (m.by === 'agent' ? 'An agent' : 'Someone'));
        var meta = node('span', 'document-comment-when', ' · ' + when(m.at) + (m.kind === 'resolve' ? ' · resolved' : m.kind === 'reopen' ? ' · reopened' : ''));
        p.append(who, meta);
        if (m.text) { p.appendChild(document.createElement('br')); p.appendChild(node('span', 'document-comment-text', m.text)); }
        c.appendChild(p);
      });
      if (replying[t.id] != null) c.appendChild(replyBox(t));
      else {
        var acts = node('div', 'document-comment-actions');
        acts.appendChild(button(t.resolved ? 'Reply to Reopen' : 'Reply', false, function () { replying[t.id] = ''; draw(); focusReply(t.id); }));
        if (!t.resolved) acts.appendChild(button('Resolve', false, function () { resolve(t); }));
        c.appendChild(acts);
      }
      return c;
    }
    function replyBox(t) {
      var box = node('div', 'document-comment-reply'), area = node('textarea', 'form-textarea');
      area.rows = 3; area.setAttribute('aria-label', 'Your reply'); area.value = replying[t.id] || '';
      area.addEventListener('input', function () { replying[t.id] = area.value; });
      area.addEventListener('keydown', function (e) { if (e.key === 'Enter' && e.ctrlKey) { e.preventDefault(); send(); } if (e.key === 'Escape') { e.preventDefault(); cancel(); } });
      function send() {
        var text = area.value.trim();
        if (!text) { area.focus(); return; }
        Promise.resolve(api().ReplyDraftThread(draftId, t.id, text)).then(function () { delete replying[t.id]; return load(); }).catch(function (err) { host().toast('The reply was not saved: ' + message(err), 'danger'); });
      }
      function cancel() { delete replying[t.id]; draw(); }
      var acts = node('div', 'document-comment-actions');
      acts.append(button('Cancel', false, cancel), button('Reply', true, send));
      box.append(area, acts);
      return box;
    }
    function focusReply(id) {
      var area = cards.querySelector('[data-thread="' + CSS.escape(id) + '"] textarea');
      if (area) area.focus();
    }
    function resolve(t) {
      Promise.resolve(api().ResolveDraftThread(draftId, t.id, '')).then(load).catch(function (err) { host().toast('The comment was not resolved: ' + message(err), 'danger'); });
    }
    /* The words selected in the applet, as a span of the draft's text: the
       selected words, and which occurrence of them, counted in the words
       the applet shows before the selection. */
    function selectionSpan(text) {
      var sel = window.getSelection();
      if (!sel || sel.rangeCount === 0 || sel.isCollapsed || !options.applet) return null;
      var range = sel.getRangeAt(0);
      if (!options.applet.contains(range.commonAncestorContainer)) return null;
      var words = sel.toString();
      if (!words.trim()) return null;
      var before = document.createRange();
      before.selectNodeContents(options.applet);
      before.setEnd(range.startContainer, range.startOffset);
      var shownBefore = before.toString();
      var candidates = [], at = text.indexOf(words);
      while (at >= 0) { candidates.push(at); at = text.indexOf(words, at + 1); }
      if (!candidates.length) {
        // The applet shows the words without the text's marks; a single
        // line of them is looked for as a line's words.
        var trimmed = words.trim();
        at = text.indexOf(trimmed);
        if (at < 0) return null;
        return [at, at + trimmed.length];
      }
      var nth = 0, from = 0, i;
      while ((i = shownBefore.indexOf(words, from)) >= 0) { nth++; from = i + 1; }
      var chosen = candidates[Math.min(nth, candidates.length - 1)];
      return [chosen, chosen + words.length];
    }
    function newComment() {
      if (disposed) return Promise.resolve();
      return (options.flush ? options.flush() : Promise.resolve()).then(function () { return api().GetDraft(draftId); }).then(function (v) {
        var span = selectionSpan(v.text || '');
        if (!span) {
          if (host()) host().toast('Select the words the comment is about, then choose New Comment again.', 'warn');
          return;
        }
        if (shown === 'off') setShownAll('open');
        composing = { from: span[0], to: span[1], words: (v.text || '').slice(span[0], span[1]), text: '' };
        draw();
        var area = cards.querySelector('.document-comment-new textarea');
        if (area) area.focus();
      }).catch(function (err) { if (host()) host().toast(message(err), 'danger'); });
    }
    function composer() {
      var c = node('section', 'card document-comment document-comment-new');
      c.appendChild(node('div', 'document-comment-head', 'New Comment'));
      c.appendChild(node('p', 'document-comment-quote', '“' + (composing.words.length > 140 ? composing.words.slice(0, 140) + '…' : composing.words) + '”'));
      var area = node('textarea', 'form-textarea');
      area.rows = 3; area.setAttribute('aria-label', 'Your comment'); area.value = composing.text;
      area.addEventListener('input', function () { composing.text = area.value; });
      area.addEventListener('keydown', function (e) { if (e.key === 'Enter' && e.ctrlKey) { e.preventDefault(); send(); } if (e.key === 'Escape') { e.preventDefault(); cancel(); } });
      function send() {
        var text = area.value.trim();
        if (!text) { area.focus(); return; }
        var at = composing;
        Promise.resolve(api().OpenDraftThread(draftId, at.from, at.to, text)).then(function (t) { composing = null; currentId = t && t.id || null; return load(); }).catch(function (err) { host().toast('The comment was not saved: ' + message(err), 'danger'); });
      }
      function cancel() { composing = null; draw(); }
      var acts = node('div', 'document-comment-actions');
      acts.append(button('Cancel', false, cancel), button('Comment', true, send));
      c.append(area, acts);
      return c;
    }
    /* Going to a comment selects its words where the applet shows them,
       through the page's own find, and marks its card. */
    function goTo(t) {
      currentId = t.id;
      cards.querySelectorAll('.document-comment').forEach(function (c) { if (c.dataset.thread === currentId) c.dataset.current = 'true'; else delete c.dataset.current; });
      var words = (t.quote || '').split('\n')[0].trim();
      if (words && options.applet && window.find) {
        var sel = window.getSelection();
        if (sel) { sel.removeAllRanges(); var start = document.createRange(); start.setStart(options.applet, 0); start.collapse(true); sel.addRange(start); }
        window.find(words, true, false, false, false, false, false);
      }
    }
    function step(direction) {
      if (shown === 'off' && list.some(function (t) { return !t.resolved; })) { setShownAll('open'); }
      var items = visible().slice().sort(function (a, b) { return a.from - b.from; });
      if (!items.length) return;
      var i = items.findIndex(function (t) { return t.id === currentId; });
      i = i < 0 ? (direction > 0 ? 0 : items.length - 1) : (i + direction + items.length) % items.length;
      goTo(items[i]);
      var c = cards.querySelector('[data-thread="' + CSS.escape(items[i].id) + '"]');
      if (c && c.scrollIntoView) c.scrollIntoView({ block: 'nearest' });
    }
    var unsubscribe = host() ? host().on('threads:changed', function (e) { if (e && e.id === draftId) load(); }) : function () {};
    var redraw = function () { revealed = null; draw(); if (options.changed) options.changed(); };
    listeners.push(redraw);
    load().then(bringIn);
    timer = setInterval(bringIn, 60000);
    return {
      reload: load,
      newComment: newComment,
      next: function () { step(1); },
      previous: function () { step(-1); },
      count: function () { return list.length; },
      unsent: function () { return list.some(function (t) { return t.unsent; }); },
      shown: function () { return shown; },
      setShown: setShownAll,
      destroy: function () {
        disposed = true; clearInterval(timer); unsubscribe();
        listeners = listeners.filter(function (fn) { return fn !== redraw; });
      }
    };
  }
  window.dinahDocumentComments = { mount: mount, shown: function () { return shown; }, setShown: setShownAll };
})();
