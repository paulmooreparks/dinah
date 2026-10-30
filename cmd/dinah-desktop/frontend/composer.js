/* The box where the person writes a message, shared by a chat (app.js) and
   by a group chat and a thread (bus-applets.js). The box grows with its
   text up to the stylesheet's limit (app.css, .composer-input). The key
   chosen in Settings › Chat sends: Enter, with Shift+Enter for a new line,
   or Ctrl+Enter, with Enter for one; Ctrl+Enter sends under either. Up in
   the box, with the caret on the first line it shows, brings back the
   previous message sent from it, and Down, from the last line, goes forward
   again, back to what was being typed. A line that wraps counts as the
   lines it shows. What each window sends, what its history holds and what
   else its box offers, such as a chat's skills or a group's mentions, is
   the window's own. */
(function () {
  'use strict';

  /* Completion lists live beside the scrolling body, so its height limit
     cannot clip the choices. Their height is bounded by the window above. */
  function layout(form) {
    var pop = form.querySelector('.slash-pop'), content = document.createElement('div');
    content.className = 'composer-scroll';
    if (pop) pop.remove();
    while (form.firstChild) content.appendChild(form.firstChild);
    form.appendChild(content);
    if (pop) form.appendChild(pop);
    function sizeCompletion() {
      if (!pop || pop.hidden || !form.isConnected) return;
      var bounds = form.parentElement.getBoundingClientRect(), top = form.getBoundingClientRect().top;
      pop.style.maxHeight = Math.max(32, Math.min(top - Math.max(0, bounds.top) - 8, 18 * parseFloat(getComputedStyle(form).fontSize))) + 'px';
    }
    var resize = new ResizeObserver(sizeCompletion); resize.observe(form);
    var visibility = new MutationObserver(sizeCompletion);
    if (pop) visibility.observe(pop, { attributes: true, attributeFilter: ['hidden'] });
    return { content: content, destroy: function () { resize.disconnect(); visibility.disconnect(); } };
  }

  /* The key that sends (Settings › Chat, prompt_keys.go). app.js sets it
     from the settings, and the windows read it when a key is pressed. */
  var sendKey = 'enter';
  function setSendKey(value) { sendKey = value === 'ctrl-enter' ? 'ctrl-enter' : 'enter'; document.querySelectorAll('[data-composer-send]').forEach(sendTitle); }
  function sendKeyName() { return sendKey === 'ctrl-enter' ? 'Ctrl+Enter' : 'Enter'; }
  function sendTitle(button) { button.setAttribute('data-tooltip', 'Send (' + sendKeyName() + ')'); }

  /* Every text area grows with its text (app.css, textarea.form-textarea);
     where the browser has no field-sizing, the box is sized here on each
     keystroke, and the stylesheet's min-height and max-height bound it
     either way. */
  var autoSize = !CSS.supports('field-sizing', 'content');
  function fit(box) {
    if (!autoSize || !box || box.tagName !== 'TEXTAREA') return;
    box.style.height = 'auto';
    box.style.height = box.scrollHeight + 2 + 'px';
  }
  document.addEventListener('input', function (e) { if (e.target.matches && e.target.matches('textarea.form-textarea')) fit(e.target); });

  /* Whether the caret is on the first and on the last line the box shows.
     A hidden copy of the box, with its width and text styles, lays out the
     same text with a zero-width mark at the start, at the caret and at the
     end; the caret is on the first line when its mark is level with the
     first mark. The marks are word joiners, so they add no place to wrap. */
  var mirrored = ['fontFamily', 'fontSize', 'fontWeight', 'fontStyle', 'fontVariant', 'fontStretch', 'fontKerning', 'fontFeatureSettings', 'letterSpacing', 'wordSpacing', 'lineHeight', 'textTransform', 'textIndent', 'tabSize', 'wordBreak', 'overflowWrap', 'hyphens', 'direction', 'paddingLeft', 'paddingRight'];
  function caretRows(box) {
    var css = getComputedStyle(box), copy = document.createElement('div'), style = copy.style;
    mirrored.forEach(function (p) { style[p] = css[p]; });
    style.position = 'absolute'; style.visibility = 'hidden'; style.left = '-10000px'; style.top = '0';
    style.boxSizing = 'content-box'; style.border = '0'; style.whiteSpace = 'pre-wrap';
    style.width = Math.max(0, box.clientWidth - parseFloat(css.paddingLeft) - parseFloat(css.paddingRight)) + 'px';
    function mark() { var m = document.createElement('span'); m.textContent = '⁠'; return m; }
    var v = box.value, pos = box.selectionStart, start = mark(), caret = mark(), end = mark();
    copy.append(start, v.slice(0, pos), caret, v.slice(pos), end);
    document.body.appendChild(copy);
    var rows = { first: Math.abs(caret.offsetTop - start.offsetTop) < 2, last: Math.abs(caret.offsetTop - end.offsetTop) < 2 };
    copy.remove();
    return rows;
  }

  /* The messages sent from one box, oldest first, for Up and Down. An
     entry is whatever write() puts back in the box: a chat's is its text,
     and a group's is its text with the chats it mentions. read() returns
     what is in the box now, which Down brings back after the newest entry.
     h.draft(now) is the person's draft: what Up set aside while an earlier
     message is in the box, and otherwise now, what the box holds. */
  function history(read, write) {
    var h = { entries: [], at: -1, typing: null };
    h.draft = function (now) { return h.at >= 0 ? h.typing : now; };
    // A list loaded later replaces the entries unless the person is
    // going through them.
    h.set = function (list) { if (h.at < 0) h.entries = list || []; };
    h.clear = function () { h.entries = []; h.at = -1; };
    h.push = function (entry) { if (entry) h.entries.push(entry); h.at = -1; };
    h.key = function (e, box) {
      if (e.key === 'ArrowUp' && h.entries.length && caretRows(box).first) {
        if (h.at < 0) { h.typing = read(); h.at = h.entries.length; }
        if (h.at > 0) { h.at--; e.preventDefault(); write(h.entries[h.at]); box.setSelectionRange(0, 0); return true; }
      } else if (e.key === 'ArrowDown' && h.at >= 0 && caretRows(box).last) {
        h.at++;
        var next = h.at < h.entries.length ? h.entries[h.at] : h.typing;
        if (h.at >= h.entries.length) h.at = -1;
        e.preventDefault();
        write(next);
        return true;
      }
      return false;
    };
    return h;
  }

  /* The box's keys: the send key, and Up and Down through o.history.
     o.send(event) sends. A list the box opens over itself, such as a
     chat's skills or a group's members, takes its keys first, in the
     capturing phase, and stops them reaching these. */
  function bind(box, o) {
    box.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); o.send(e); return; }
      if (e.key === 'Enter' && sendKey === 'enter' && !e.shiftKey && !e.altKey && !e.isComposing) { e.preventDefault(); o.send(e); return; }
      if (e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) return;
      if (o.history) o.history.key(e, box);
    });
  }

  /* The box as a window builds it in code: the text area, a place for the
     list it opens over itself, and Send, in the markup and with the classes
     of a chat's box (index.html, .composer-row). o.label is the box's name
     and o.placeholder its hint. Send's tooltip says the send key in force
     when the pointer or the focus reaches it. */
  var made = 0;
  function box(o) {
    var row = document.createElement('div'); row.className = 'composer-row';
    var id = 'composer-text-' + (++made);
    var label = document.createElement('label'); label.className = 'visually-hidden'; label.htmlFor = id; label.textContent = o.label;
    var text = document.createElement('textarea'); text.className = 'form-textarea composer-input'; text.id = id; text.rows = 2;
    text.placeholder = o.placeholder || o.label;
    var pop = document.createElement('div'); pop.className = 'slash-pop'; pop.id = id + '-pop'; pop.setAttribute('role', 'listbox'); pop.hidden = true;
    var actions = document.createElement('div'); actions.className = 'composer-actions';
    var send = document.createElement('button'); send.type = 'submit'; send.className = 'btn btn-primary'; send.textContent = 'Send';
    send.setAttribute('data-composer-send', '');
    function title() { sendTitle(send); }
    title();
    send.addEventListener('pointerenter', title); send.addEventListener('focus', title);
    actions.appendChild(send);
    row.append(label, text, pop, actions);
    return { el: row, text: text, pop: pop, button: send };
  }

  /* What was typed in a box and not sent, kept across a restart (Paul,
     28 October 2026) in the saved UI state (ui-state.js), which is in
     Dinah's home folder and never in a workspace. Each draft is under
     "dinah-draft:" and a key the window chooses: a chat's is its session,
     and a group's or a thread's is its own. The UI state writes it to disk
     shortly after each change and again when the window is hidden or
     closed.

     keeper(key, restore) is one box's draft. The saved state is read once
     the layout has been restored (ui-state.js, readSaved), and the saved
     draft is given to restore(value) then. Until a box has read its draft
     it saves nothing, so an empty box made before the saved state is ready
     never writes over the draft it is about to show. k.save(value) keeps
     the draft, and a value of null drops it; k.rekey(key) moves the draft
     to another key, as when a new chat gets its session. */
  var prefix = 'dinah-draft:';
  function store() { return window.dinahUIState; }
  function readable() { var s = store(); return !!s && !s.isRestoring(); }
  function keeper(key, restore) {
    var k = { key: key, read: false, gone: false };
    k.save = function (value) {
      if (!k.read || k.gone) return;
      store().set(prefix + k.key, value == null ? undefined : value);
    };
    k.rekey = function (next) {
      if (next === k.key) return;
      if (k.read && !k.gone) {
        var value = store().get(prefix + k.key);
        store().set(prefix + k.key, undefined);
        if (value != null) store().set(prefix + next, value);
      }
      k.key = next;
    };
    // A box that is gone, such as one whose chat or group was deleted,
    // drops its draft and saves no more.
    k.forget = function () { if (k.read) k.save(null); else forget(k.key); k.gone = true; };
    function read() {
      if (k.gone) return;
      k.read = true;
      var value = store().get(prefix + k.key);
      if (value != null) restore(value);
    }
    if (readable()) read();
    else if (store()) document.addEventListener('dinah:ui-state-restored', read, { once: true });
    return k;
  }
  // Drops the draft of a box that may not be open, such as a deleted
  // chat's.
  function forget(key) { if (readable()) store().set(prefix + key, undefined); }

  /* ---- Files attached to the next message ----
     A file pasted into a box, dropped on its window or chosen from the
     computer goes with the next message, shown as a chip above the box with
     its picture or a file glyph, its name, its size and Remove. A chat and a
     group chat share these chips. A file larger than 25 MB, the limit the
     bus keeps (bus.MaxAttachmentFile), is refused when it is added, with the
     reason, and is never sent. A file is { kind: 'file', name, mimeType,
     data, size, thumb }, its data in base64 and thumb a picture's data
     address; a chat's list also holds workspace documents, { kind:
     'resource', name, ref }, which the chat adds itself.

     attachments(o) is one box's chips. o.host is the list they are drawn
     in. o.get() returns the window's list and o.set(list) replaces it.
     o.changed() is called after a file is added or removed, as after typing,
     and o.drawn() after every drawing, such as to save a draft. o.alive()
     says whether the window is still there when a file has been read.
     o.note(list) returns a line to show after the chips, or nothing, and
     o.toast(text) tells the person something went wrong.

     o.limit() returns the limit on the files in one message, { max, reason },
     or nothing when there is none (Paul, 28 October 2026). A chat's is its
     provider's, set in Settings › Agents, and a group's the smallest among
     its member chats (file_limits.go), whose reason says which member set
     it. Only files count: a workspace document goes as a link. Once the
     limit is reached, a file pasted or dropped is refused with the reason,
     c.full() is true and c.fullReason() is the reason, which the window
     puts on its Attach… command as it disables it, and the line after the
     chips says it. */
  var MAX_FILE = 25 * 1024 * 1024;
  function sizeWords(n) { return n > 1048576 ? (n / 1048576).toFixed(1) + ' MB' : Math.max(1, Math.round(n / 1024)) + ' KB'; }
  function attachments(o) {
    var c = {}, reading = 0;
    function alive() { return !o.alive || o.alive(); }
    function limit() { var l = o.limit ? o.limit() : null; return l && l.max > 0 ? l : null; }
    function counted() { return o.get().filter(function (a) { return a.kind === 'file'; }).length; }
    // How many more files the message takes, counting those still being read.
    function room() { var l = limit(); return l ? Math.max(0, l.max - counted() - reading) : Infinity; }
    c.full = function () { return room() === 0; };
    c.fullReason = function () { var l = limit(); return l && c.full() ? 'No more files can be attached. ' + l.reason : ''; };
    function refuse(name) { o.toast((name || 'The file') + ' was not attached. ' + limit().reason); }
    c.draw = function () {
      var host = o.host, list = o.get();
      host.innerHTML = '';
      host.hidden = list.length === 0;
      if (o.drawn) o.drawn();
      list.forEach(function (a, i) {
        var li = document.createElement('li');
        li.className = 'attachment';
        li.innerHTML = (a.thumb ? '<img alt="">' : '<span class="glyph" style="--glyph: var(--glyph-file)" aria-hidden="true"></span>') +
          '<span class="attachment-name"></span><span class="attachment-size"></span>' +
          '<button type="button" class="icon-btn attachment-remove"><span class="glyph" style="--glyph: var(--glyph-close)" aria-hidden="true"></span></button>';
        if (a.thumb) li.querySelector('img').src = a.thumb;
        li.querySelector('.attachment-name').textContent = a.name;
        li.querySelector('.attachment-size').textContent = a.kind === 'resource' ? 'from ' + (window.dinahProduct ? window.dinahProduct.name : 'Dinah') : sizeWords(a.size);
        var rm = li.querySelector('.attachment-remove');
        rm.setAttribute('aria-label', 'Remove ' + a.name);
        rm.title = 'Remove';
        rm.addEventListener('click', function () { o.get().splice(i, 1); c.draw(); if (o.changed) o.changed(); });
        host.appendChild(li);
      });
      // A note is words, or { text, warn } for a warning, which is marked by a glyph and a colour.
      [o.note ? o.note(list) : '', c.fullReason()].forEach(function (said) {
        if (!said) return;
        var warn = typeof said === 'object' && said.warn;
        var note = document.createElement('li');
        note.className = 'attachment-note' + (warn ? ' attachment-warn' : '');
        note.textContent = typeof said === 'object' ? said.text : said;
        host.appendChild(note);
      });
    };
    // Files from the clipboard, a drop or the page's own file input.
    c.add = function (files) {
      Array.prototype.forEach.call(files || [], function (f) {
        if (f.size > MAX_FILE) { o.toast((f.name || 'The file') + ' is larger than 25 MB, so it was not attached.'); return; }
        if (room() === 0) { refuse(f.name); return; }
        // The file holds its place while it is read, so that files dropped
        // together cannot pass the limit; load comes before loadend, and the
        // place is given up at whichever ends the read first.
        reading++;
        var held = true;
        function release() { if (held) { held = false; reading--; } }
        var reader = new FileReader();
        reader.onloadend = release;
        reader.onload = function () {
          release();
          if (!alive()) return;
          var url = String(reader.result), comma = url.indexOf(',');
          var name = f.name || ('pasted-' + new Date().toISOString().replace(/[:.]/g, '-') + '.' + ((f.type.split('/')[1] || 'bin').replace('jpeg', 'jpg')));
          o.get().push({ kind: 'file', name: name, mimeType: f.type || 'application/octet-stream', data: url.slice(comma + 1), size: f.size, thumb: /^image\//.test(f.type) ? url : '' });
          c.draw();
          if (o.changed) o.changed();
        };
        reader.onerror = function () { o.toast('The attachment could not be read. Try copying it again, or attach it from the computer.'); };
        reader.readAsDataURL(f);
      });
    };
    /* Files chosen in the system's dialog (attach.go, PickAttachments),
       which refuses a file over the limit with the reason. Where the page
       runs outside the app, fallback, a hidden file input, is opened. The
       dialog cannot be told how many files to allow, so the files chosen
       past the limit on the files in one message are refused afterwards,
       by name, with the reason. */
    c.pick = function (fallback) {
      if (room() === 0) { o.toast(c.fullReason()); return Promise.resolve(); }
      var api = window.dinahHelpers.app();
      if (!api || typeof api.PickAttachments !== 'function') { if (fallback) fallback.click(); return Promise.resolve(); }
      return Promise.resolve().then(function () { return api.PickAttachments(); }).then(function (files) {
        if (!alive()) return;
        files = files || [];
        var fit = Math.min(files.length, room());
        if (fit < files.length) {
          var left = files.slice(fit).map(function (f) { return f.name; });
          o.toast(left.join(', ') + (left.length === 1 ? ' was' : ' were') + ' not attached. ' + limit().reason);
        }
        files.slice(0, fit).forEach(function (f) {
          o.get().push({ kind: 'file', name: f.name, mimeType: f.mimeType, data: f.data, size: f.size,
            thumb: /^image\//.test(f.mimeType) ? 'data:' + f.mimeType + ';base64,' + f.data : '' });
        });
        c.draw();
        if (o.changed) o.changed();
      });
    };
    /* The files on a clipboard: its files, or the file items of a copied
       picture. The window decides whether a paste is its own. */
    c.clipboardFiles = function (cd) {
      if (!cd) return [];
      var files = Array.from(cd.files || []);
      if (!files.length) Array.from(cd.items || []).forEach(function (item) {
        if (item.kind === 'file') { var file = item.getAsFile(); if (file) files.push(file); }
      });
      return files;
    };
    /* Files dropped on zone are added, and the zone is outlined while files
       are dragged over it. after() runs once they are added, such as to put
       the focus back in the box. */
    c.dropOn = function (zone, after) {
      ['dragenter', 'dragover'].forEach(function (t) {
        zone.addEventListener(t, function (e) {
          if (!e.dataTransfer || Array.prototype.indexOf.call(e.dataTransfer.types || [], 'Files') < 0) return;
          e.preventDefault();
          e.dataTransfer.dropEffect = 'copy';
          zone.classList.add('dropping');
        });
      });
      zone.addEventListener('dragleave', function (e) { if (!zone.contains(e.relatedTarget)) zone.classList.remove('dropping'); });
      zone.addEventListener('drop', function (e) {
        zone.classList.remove('dropping');
        if (!e.dataTransfer || !e.dataTransfer.files.length) return;
        e.preventDefault();
        c.add(e.dataTransfer.files);
        if (after) after();
      });
    };
    // The files as they are sent: their names, types and contents.
    c.files = function () {
      return o.get().filter(function (a) { return a.kind === 'file'; }).map(function (a) { return { kind: 'file', name: a.name, mimeType: a.mimeType, data: a.data }; });
    };
    return c;
  }

  window.dinahComposer = { setSendKey: setSendKey, sendKey: function () { return sendKey; }, sendKeyName: sendKeyName, fit: fit, caretRows: caretRows,
    history: history, bind: bind, box: box, layout: layout, keeper: keeper, forget: forget, attachments: attachments, sizeWords: sizeWords, maxFile: MAX_FILE };
})();
