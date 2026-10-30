/* Bound the DOM and parsing cost of a single oversized message. The caller
   retains the complete text for copy, export and provider context. */
(function () {
  'use strict';
  var limit = 16384, views = new WeakMap();
  function boundary(text, index) {
    if (index > 0 && index < text.length && /[\uDC00-\uDFFF]/.test(text[index])) return index - 1;
    return index;
  }
  /* A text within the limit is drawn by small. Its second argument is true
     when the element still shows the paged view, so that a caller drawing
     incrementally knows to start again. */
  function draw(el, text, small) {
    if (text.length <= limit) { small(text, views.delete(el)); return; }
    var view = views.get(el);
    if (!view) {
      view = { text: text, page: 0, tail: true };
      views.set(el, view); el.replaceChildren();
      var bar = document.createElement('span'); bar.className = 'bounded-text-controls';
      bar.style.cssText = 'display:flex;flex-wrap:wrap;gap:.4rem;align-items:center;white-space:normal';
      view.label = document.createElement('span'); bar.appendChild(view.label);
      function button(label, move) {
        var b = document.createElement('button'); b.type = 'button'; b.className = 'btn btn-sm'; b.textContent = label;
        b.addEventListener('click', function () {
          move(); window.dinahRender.enqueue(el, null, function () { if (el.isConnected && views.get(el) === view) paint(); }, null, 'large text page');
        });
        bar.appendChild(b); return b;
      }
      view.first = button('First', function () { view.tail = false; view.page = 0; });
      view.prev = button('Previous', function () { view.tail = false; view.page--; });
      view.next = button('Next', function () { view.tail = false; view.page++; });
      view.last = button('Latest', function () { view.tail = true; });
      view.body = document.createElement('span');
      view.body.style.cssText = 'display:block;white-space:pre-wrap;overflow-wrap:anywhere';
      el.append(bar, view.body);
    }
    view.text = text;
    function paint() {
      var count = Math.ceil(view.text.length / limit);
      view.page = view.tail ? count - 1 : Math.max(0, Math.min(view.page, count - 1));
      var start = boundary(view.text, view.page * limit), end = boundary(view.text, Math.min(view.text.length, (view.page + 1) * limit));
      view.label.textContent = 'Large message, plain text page ' + (view.page + 1) + ' of ' + count + '. All text is retained.';
      view.body.textContent = view.text.slice(start, end);
      view.first.disabled = view.prev.disabled = view.page === 0;
      view.next.disabled = view.page === count - 1;
      view.last.disabled = view.tail;
    }
    paint();
  }
  window.dinahBoundedText = { draw: draw };
})();
