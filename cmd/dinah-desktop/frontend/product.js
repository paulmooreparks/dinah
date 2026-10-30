/* The product this build is, as the page shows it: its name and its default
   theme, which the build's profile gives (internal/profile). The page starts
   with the values written on its html element, so nothing changes while it
   loads, and app.js puts in the ones State reports through apply.

   An element marked data-product-name shows the name as its text. An
   element with data-product-attrs="title aria-label" has the name put into
   each attribute listed, wherever the markup's own name stands in it.
   Scripts read dinahProduct.name for text they write. */
(function () {
  'use strict';
  var root = document.documentElement;
  var initial = root.getAttribute('data-product-name') || '';
  var product = { name: initial, defaultTheme: root.getAttribute('data-product-theme') || '' };
  // The attributes as the markup wrote them, split where its name stood.
  var templates = new WeakMap();

  function fill(scope) {
    scope = scope || document;
    scope.querySelectorAll('[data-product-name]').forEach(function (el) {
      if (el !== root) el.textContent = product.name;
    });
    if (!initial) return;
    scope.querySelectorAll('[data-product-attrs]').forEach(function (el) {
      var saved = templates.get(el);
      if (!saved) {
        saved = {};
        el.getAttribute('data-product-attrs').split(/\s+/).forEach(function (attr) {
          if (attr && el.hasAttribute(attr)) saved[attr] = el.getAttribute(attr).split(initial);
        });
        templates.set(el, saved);
      }
      Object.keys(saved).forEach(function (attr) { el.setAttribute(attr, saved[attr].join(product.name)); });
    });
    if (scope === document && initial) document.title = document.title.split(initial).join(product.name);
  }

  window.dinahProduct = {
    get name() { return product.name; },
    get defaultTheme() { return product.defaultTheme; },
    // apply takes State's product and fills the page with it.
    apply: function (info) {
      if (!info) return;
      if (info.name) product.name = info.name;
      if (info.defaultTheme) product.defaultTheme = info.defaultTheme;
      fill(document);
      // The menu bar reads its titles from the markup when it is built.
      if (window.pudlMenubar && window.pudlMenubar.refresh) window.pudlMenubar.refresh();
      // Scripts that drew the name elsewhere draw it again on this event.
      document.dispatchEvent(new CustomEvent('dinah:product', { detail: { name: product.name } }));
    },
    fill: fill
  };
})();
