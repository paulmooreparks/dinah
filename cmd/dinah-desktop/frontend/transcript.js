/* A transcript owns following, deliberate navigation and per-message seen
   state. Chat and group adapters supply their message rows and persistence. */
(function () {
  'use strict';
  var node = window.dinahHelpers.node;
  function mount(o) {
    var log = o.log, content = o.content, disposed = false, following = true;
    var frame = 0, navigationTarget = null;
    var known = new Map(), unseen = new Map(), started = false, sequence = 0, localKeys = new WeakMap(), seenSignature = '';
    log.setAttribute('data-follow-end', '');
    log.tabIndex = log.tabIndex < 0 ? 0 : log.tabIndex;
    var wrap = o.jump && o.jump.parentElement || node('div', 'jump-end-wrap');
    var jump = o.jump || node('button', 'icon-btn jump-end');
    if (!o.jump) {
      jump.type = 'button'; jump.setAttribute('aria-label', 'Jump to the latest'); jump.title = 'Jump to the latest';
      var glyph = node('span', 'glyph'); glyph.style.setProperty('--glyph', 'var(--glyph-arrow-down)'); glyph.setAttribute('aria-hidden', 'true');
      jump.appendChild(glyph); wrap.appendChild(jump); log.appendChild(wrap);
    }
    var controls = node('div', 'transcript-unread-controls'), stopped = node('button', 'btn btn-sm', 'Jump to Where You Stopped');
    stopped.type = 'button'; controls.appendChild(stopped); controls.hidden = true; log.before(controls);
    var divider = node('p', 'transcript-unread-divider'); divider.setAttribute('role', 'status');
    var unreadGlyph = node('span', 'glyph'), unreadWords = node('span');
    unreadGlyph.style.setProperty('--glyph', 'var(--glyph-info)'); unreadGlyph.setAttribute('aria-hidden', 'true'); divider.append(unreadGlyph, unreadWords);
    function atEnd() { return log.scrollHeight - log.scrollTop - log.clientHeight < 24; }
    function active() {
      var win = o.root.closest('.win');
      return !!win && !win.hidden && !!log.clientHeight && document.visibilityState === 'visible' && window.pudlWindows.state().top === win.dataset.win;
    }
    function ready() { return (!o.ready || o.ready()) && document.documentElement.dataset.uiStateReady === 'true'; }
    function setFollowing(value) {
      following = !!value;
      log.dataset.following = String(following);
      if (following) { navigationTarget = null; window.dinahChatText.releasePlace(log); }
      if (o.followingChanged) o.followingChanged(following);
      jump.hidden = following || !log.clientHeight || atEnd();
    }
    function release() {
      navigationTarget = null;
      window.dinahChatText.releasePlace(log);
      if (window.dinahUIState) window.dinahUIState.releaseScroll(log);
    }
    function wholeVisible(el) {
      if (!el || !el.isConnected) return false;
      var a = log.getBoundingClientRect(), b = el.getBoundingClientRect();
      var style = getComputedStyle(log), paddingTop = parseFloat(style.paddingTop) || 0, paddingBottom = parseFloat(style.paddingBottom) || 0;
      var top = Math.max(a.top + log.clientTop + paddingTop, 0), bottom = Math.min(a.top + log.clientTop + log.clientHeight - paddingBottom, innerHeight);
      var intersection = Math.max(0, Math.min(b.bottom, bottom) - Math.max(b.top, top));
      return b.height > 0 && intersection + 2 >= Math.min(b.height, bottom - top);
    }
    function messages() {
      return (o.entries ? o.entries() : []).filter(function (entry) { return entry.element && entry.eligible !== false; }).map(function (entry) {
        if (entry.key === undefined) { var identity = entry.identity || entry.element; if (!localKeys.has(identity)) localKeys.set(identity, 'local-' + (++sequence)); entry.key = localKeys.get(identity); }
        entry.key = String(entry.key); return entry;
      });
    }
    function updateSeen() {
      if (disposed || !ready()) return;
      var rows = messages(), present = new Set();
      rows.forEach(function (entry) {
        present.add(entry.key);
        if (!known.has(entry.key) && (started || o.initialSeen && !o.initialSeen(entry))) unseen.set(entry.key, entry);
        known.set(entry.key, entry);
        if (unseen.has(entry.key)) unseen.set(entry.key, entry);
      });
      started = true;
      Array.from(unseen.keys()).forEach(function (key) { if (!present.has(key)) unseen.delete(key); });
      if (active()) {
        if (atEnd()) { unseen.clear(); setFollowing(true); }
        else unseen.forEach(function (entry, key) { if (wholeVisible(entry.element)) unseen.delete(key); });
      }
      var pending = rows.filter(function (entry) { return unseen.has(entry.key); }), first = pending[0];
      var anchor = !following && rows.find(function (entry) { return entry.element.getBoundingClientRect().bottom > log.getBoundingClientRect().top; });
      var anchorTop = anchor && anchor.element.getBoundingClientRect().top - log.getBoundingClientRect().top;
      var words = pending.length + (pending.length === 1 ? ' new message' : ' new messages');
      if (first) {
        if (unreadWords.textContent !== words) unreadWords.textContent = words;
        if (divider.nextElementSibling !== first.element) first.element.before(divider);
      } else if (divider.parentNode) divider.remove();
      controls.hidden = !first;
      if (anchor && anchor.element.isConnected) {
        var shift = anchor.element.getBoundingClientRect().top - log.getBoundingClientRect().top - anchorTop;
        if (Math.abs(shift) > 0.5) { log.scrollTop += shift; }
      }
      var reachedEnd = active() && atEnd();
      var signature = JSON.stringify([rows.map(function (entry) { return entry.key; }), pending.map(function (entry) { return entry.key; }), reachedEnd]);
      if (signature !== seenSignature) {
        seenSignature = signature;
        if (o.seenChanged) o.seenChanged({ entries: rows, unseen: pending, atEnd: reachedEnd });
      }
    }
    function check() {
      if (disposed || !log.clientHeight) return;
      // Only deliberate navigation releases following. A delayed layout scroll
      // can carry an old position after both the extent and viewport settled.
      if (following) queueTail();
      else setFollowing(atEnd());
      updateSeen();
    }
    function scroll(force) {
      if (disposed || !log.clientHeight) return;
      if (window.dinahUIState && window.dinahUIState.restorePending(log)) {
        window.dinahUIState.restoreNow(log); setFollowing(atEnd()); updateSeen(); return;
      }
      if (force) setFollowing(true);
      if (following) { window.dinahChatText.releasePlace(log); log.scrollTop = log.scrollHeight; }
      jump.hidden = following || atEnd(); updateSeen();
    }
    function queueTail() {
      if (disposed || !following || o.canTail && !o.canTail()) return;
      schedule();
    }
    function centreTarget() {
      if (!navigationTarget) return;
      if (!navigationTarget.isConnected) { navigationTarget = null; return; }
      var viewport = log.getBoundingClientRect(), row = navigationTarget.getBoundingClientRect();
      var shift = (row.top + row.bottom - viewport.top - viewport.bottom) / 2;
      if (Math.abs(shift) > 0.5) log.scrollTop += shift;
    }
    function navigate(row) {
      release(); setFollowing(false); navigationTarget = row;
      centreTarget(); schedule();
    }
    function schedule() {
      if (disposed || frame) return;
      frame = requestAnimationFrame(function () { frame = 0; if (disposed) return; if (following && (!o.canTail || o.canTail())) scroll(); else { updateSeen(); centreTarget(); } });
    }
    function input(event) {
      if (event.type === 'keydown' && (!['ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End', ' '].includes(event.key) || event.target.closest('input, textarea, [contenteditable="true"]'))) return;
      if (event.type === 'pointerdown' && event.target !== log) return;
      release();
      if (event.type !== 'wheel' || event.deltaY < 0) setFollowing(false);
    }
    function restored() { window.dinahChatText.releasePlace(log); setFollowing(atEnd()); schedule(); }
    function latest() {
      release(); if (o.jumped) o.jumped(); scroll(true);
    }
    function whereStopped() {
      var first = Array.from(unseen.values())[0]; if (!first) return;
      release(); setFollowing(false); first.element.scrollIntoView({ block: 'start' }); schedule();
    }
    jump.addEventListener('click', latest); stopped.addEventListener('click', whereStopped);
    log.addEventListener('scroll', check, { passive: true }); log.addEventListener('dinah:scroll-restored', restored);
    ['wheel', 'touchstart', 'pointerdown', 'keydown'].forEach(function (name) { log.addEventListener(name, input, { passive: true }); });
    var mutation = new MutationObserver(schedule); mutation.observe(content, { childList: true, subtree: true, characterData: true });
    var resize = new ResizeObserver(schedule); resize.observe(content); resize.observe(log);
    document.addEventListener('pudl:windows-change', schedule); document.addEventListener('visibilitychange', schedule);
    setFollowing(true); schedule();
    return {
      pause: function () { release(); setFollowing(false); },
      resetMessages: function () { known.clear(); unseen.clear(); started = false; seenSignature = ''; divider.remove(); controls.hidden = true; },
      navigate: navigate, atEnd: atEnd, following: function () { return following; }, scroll: scroll, queueTail: queueTail, update: schedule,
      keep: function (run, deferTail) {
        if (!following) return window.dinahChatText.keepPlace(log, run);
        window.dinahChatText.releasePlace(log); var result = run(); if (!deferTail) scroll(true); return result;
      },
      position: function (value) {
        if (!value) return { following: following, top: log.scrollTop };
        release(); setFollowing(value.following); log.scrollTop = following ? log.scrollHeight : value.top; schedule();
      },
      destroy: function () {
        disposed = true; if (frame) cancelAnimationFrame(frame); mutation.disconnect(); resize.disconnect(); controls.remove(); divider.remove();
        jump.removeEventListener('click', latest); stopped.removeEventListener('click', whereStopped);
        log.removeEventListener('scroll', check); log.removeEventListener('dinah:scroll-restored', restored);
        ['wheel', 'touchstart', 'pointerdown', 'keydown'].forEach(function (name) { log.removeEventListener(name, input); });
        document.removeEventListener('pudl:windows-change', schedule); document.removeEventListener('visibilitychange', schedule);
      }
    };
  }
  window.dinahTranscript = { mount: mount };
})();
