/* A window whose content nobody can see is not drawn.

   Every open window used to be laid out and painted in full on every frame
   in which anything on the page changed, however many windows lay on top of
   it. With 32 chats open, one keystroke cost 35 ms of the page's time where
   it cost 1.3 ms with one chat open, and nearly all of the difference was
   the browser working through the content of windows that could not be seen
   (measured 17 November 2026, docs/design/scaling.md).

   This file marks each window whose content area is completely covered by
   the windows above it with data-dinah-covered, and the stylesheet gives
   such a window's content content-visibility: hidden (app.css), which the
   browser documents as skipping the content's rendering while keeping its
   state, such as its scroll positions:
   https://developer.mozilla.org/en-US/docs/Web/CSS/content-visibility
   The window's frame and title bar are always drawn.

   The rule is exact, never a guess, since a wrong answer would show an empty
   window:

   - A window covers what is beneath it only where it is opaque. Its opaque
     part is its whole frame where the skin paints one, and its content area
     alone where the skin leaves the frame clear. Its rounded corners are
     left out, so content that shows past a corner is never hidden.
   - A window counts as covering only while it is plainly drawn: shown, fully
     opaque, and not transformed, filtered, clipped or blended.
   - A window sized by its content neither covers nor is covered, since its
     size would then depend on content that is not laid out.
   - Only the windows that are children of the window layer take part,
     since those are the ones whose stacking order can be compared.

   The marks are worked out again whenever a window's own attributes change
   (its place, its size and its order are all written there), whenever a
   window is added or removed, and whenever the layer, a window or the skin
   changes size or look. That is read from the page itself, so a window moved
   by a drag, by a key or by anything else is accounted for before the next
   frame is drawn. The window layer also announces each committed change
   (pudl:windows-change) before it moves focus into the window it raised, and
   the marks are worked out at once then, because the content of a covered
   window cannot take focus. */
(function () {
  'use strict';
  var MARK = 'data-dinah-covered';
  var layer = null, queued = false, observed = new WeakSet();
  var attributes = null, children = null, sizes = null;

  function px(value) { var n = parseFloat(value); return /px$/.test(value) && isFinite(n) ? n : NaN; }
  /* A colour with no transparency. The computed value is rgb(r, g, b) for an
     opaque colour and carries an alpha part otherwise, whatever its notation:
     https://drafts.csswg.org/cssom/#serializing-css-values */
  function opaque(color) { return /^rgb\(/.test(color) || /^color\([^/]*\)$/.test(color); }
  function plain(style) {
    return style.visibility === 'visible' && style.opacity === '1' && style.transform === 'none' && style.filter === 'none' &&
      style.clipPath === 'none' && style.mixBlendMode === 'normal' && (style.maskImage === 'none' || style.maskImage === undefined);
  }
  /* The part of a rounded box that is certainly inside it, as rectangles:
     the box with its corner squares cut off, which is two rectangles, and
     the box drawn in from every side by the distance at which its corner
     meets the rounding (r less r over the square root of two, here rounded
     up). What is left out is a few pixels at each corner, so two windows of
     one size in one place, whose rounded corners coincide, are both drawn:
     that costs a little work and can never show an empty window. */
  function inside(rect, style) {
    var radii = [style.borderTopLeftRadius, style.borderTopRightRadius, style.borderBottomLeftRadius, style.borderBottomRightRadius].map(function (v) { return px(String(v).split(' ')[0]); });
    var r = radii.some(isNaN) ? Math.min(rect.width, rect.height) / 2 : Math.max.apply(null, radii);
    r = Math.ceil(r);
    if (!r) return [{ left: rect.left, top: rect.top, right: rect.right, bottom: rect.bottom }];
    var d = Math.ceil(r * 0.2929);
    return [{ left: rect.left + r, top: rect.top, right: rect.right - r, bottom: rect.bottom },
      { left: rect.left, top: rect.top + r, right: rect.right, bottom: rect.bottom - r },
      { left: rect.left + d, top: rect.top + d, right: rect.right - d, bottom: rect.bottom - d }];
  }
  /* Whether the rectangles together cover the whole of the target. The
     target is cut along every rectangle's edges, and each piece must lie
     inside one of them. */
  function covers(target, rects) {
    if (target.right - target.left <= 0 || target.bottom - target.top <= 0) return true;
    var near = rects.filter(function (r) { return r.right > target.left && r.left < target.right && r.bottom > target.top && r.top < target.bottom; });
    if (!near.length) return false;
    var xs = [target.left, target.right], ys = [target.top, target.bottom];
    near.forEach(function (r) {
      if (r.left > target.left && r.left < target.right) xs.push(r.left);
      if (r.right > target.left && r.right < target.right) xs.push(r.right);
      if (r.top > target.top && r.top < target.bottom) ys.push(r.top);
      if (r.bottom > target.top && r.bottom < target.bottom) ys.push(r.bottom);
    });
    xs.sort(function (a, b) { return a - b; }); ys.sort(function (a, b) { return a - b; });
    for (var i = 0; i < xs.length - 1; i++) {
      if (xs[i + 1] === xs[i]) continue;
      for (var j = 0; j < ys.length - 1; j++) {
        if (ys[j + 1] === ys[j]) continue;
        var x = (xs[i] + xs[i + 1]) / 2, y = (ys[j] + ys[j + 1]) / 2;
        if (!near.some(function (r) { return x > r.left && x < r.right && y > r.top && y < r.bottom; })) return false;
      }
    }
    return true;
  }

  function windows() {
    return Array.prototype.filter.call(layer.children, function (el) { return el.classList.contains('win'); });
  }
  function watch(wins) {
    wins.forEach(function (win) {
      if (observed.has(win)) return;
      observed.add(win);
      attributes.observe(win, { attributes: true });
      sizes.observe(win);
    });
  }
  function update() {
    queued = false;
    if (!layer || !layer.isConnected) return;
    var wins = windows();
    watch(wins);
    // Every measurement is taken before any mark is written, so the page is laid out once.
    var view = layer.getBoundingClientRect();
    var visible = { left: Math.max(view.left, 0), top: Math.max(view.top, 0), right: Math.min(view.right, window.innerWidth), bottom: Math.min(view.bottom, window.innerHeight) };
    var facts = wins.map(function (win) {
      var body = null;
      for (var i = 0; i < win.children.length; i++) if (win.children[i].classList.contains('win-body')) { body = win.children[i]; break; }
      var fact = { win: win, body: body, shown: !win.hidden, order: 0, shapes: [], target: null, sized: win.getAttribute('data-win-size') === 'content' };
      if (!fact.shown || !body) { fact.shown = false; return fact; }
      var style = getComputedStyle(win), rect = win.getBoundingClientRect();
      if (style.display === 'none' || !rect.width || !rect.height) { fact.shown = false; return fact; }
      fact.order = Number(style.zIndex) || 0;
      var bodyStyle = getComputedStyle(body), bodyRect = body.getBoundingClientRect();
      var bodyShown = bodyStyle.display !== 'none' && bodyRect.width > 0 && bodyRect.height > 0;
      if (bodyShown && !fact.sized) {
        // What could be seen of the content area: the part inside the layer and the page.
        fact.target = { left: Math.max(bodyRect.left, visible.left), top: Math.max(bodyRect.top, visible.top), right: Math.min(bodyRect.right, visible.right), bottom: Math.min(bodyRect.bottom, visible.bottom) };
      }
      if (!fact.sized && plain(style) && style.position !== 'static') {
        if (opaque(style.backgroundColor)) fact.shapes = inside(rect, style);
        else if (bodyShown && plain(bodyStyle) && opaque(bodyStyle.backgroundColor)) fact.shapes = inside(bodyRect, bodyStyle);
      }
      return fact;
    });
    var changed = [];
    facts.forEach(function (fact) {
      var covered = false;
      if (fact.shown && fact.target) {
        var above = [];
        facts.forEach(function (other) { if (other !== fact && other.shown && other.order > fact.order) above = above.concat(other.shapes); });
        covered = covers(fact.target, above);
      }
      if (covered !== fact.win.hasAttribute(MARK)) changed.push([fact.win, covered]);
    });
    changed.forEach(function (change) {
      change[0].toggleAttribute(MARK, change[1]);
      // A window that can be seen again tells its content, which may have work it left undone while covered.
      if (!change[1]) document.dispatchEvent(new CustomEvent('dinah:window-uncovered', { detail: { window: change[0] } }));
    });
  }
  /* Changes made in one task are answered once, after the task and before
     the frame is drawn: a microtask runs before rendering, as the HTML
     event loop orders them. */
  function soon() {
    if (queued) return;
    queued = true;
    Promise.resolve().then(update);
  }
  function start() {
    layer = document.querySelector('[data-win-layer]');
    if (!layer || !window.CSS || !CSS.supports('content-visibility', 'hidden')) { layer = null; return; }
    attributes = new MutationObserver(function (records) {
      if (records.some(function (r) { return r.attributeName !== MARK; })) soon();
    });
    children = new MutationObserver(soon);
    children.observe(layer, { childList: true });
    sizes = new ResizeObserver(soon);
    sizes.observe(layer);
    // The skin and the theme, which decide what a window is painted with, are attributes of the page's root.
    new MutationObserver(soon).observe(document.documentElement, { attributes: true });
    window.addEventListener('resize', soon);
    document.addEventListener('pudl:windows-change', update);
    update();
  }
  window.dinahCovered = {
    /* Whether the element is inside a window whose content is not drawn. An
       applet asks before it measures its own content, since measuring makes
       the browser lay that content out after all. */
    is: function (el) { return !!(el && el.closest && el.closest('.win[' + MARK + ']')); },
    update: update,
    // For tests: the pure geometry.
    covers: covers
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start, { once: true });
  else start();
})();
