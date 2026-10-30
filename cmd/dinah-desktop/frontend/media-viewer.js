/* Adapted from Paul Parks's parkscomputing.com/wwwroot/js/lightbox.js.
   The original gallery, show/unzoom, click-zoom and drag/swipe handling are
   retained. Dinah supplies sanitized SVG snapshots, applet regions,
   keyboard image controls and focus restoration. Classic-page hashes,
   frame handoff and history entries do not apply to the desktop workbench. */
(function () {
  'use strict';
  var dialog = document.getElementById('media-viewer');
  if (!dialog) return;
  var stage = dialog.querySelector('[data-media-stage]'), img = null;
  var caption = dialog.querySelector('.lightbox-caption'), counter = dialog.querySelector('.lightbox-counter');
  var prevBtn = dialog.querySelector('.lightbox-prev'), nextBtn = dialog.querySelector('.lightbox-next');
  var items = [], index = 0, opener = null, request = 0, down = null;
  // A bus message's pictures are under its text, outside it (chat-text.js,
  // attachments).
  var contentImages = '.msg-text img, .msg-attachments img, .doc-page img, .fm-preview img';
  var contentMedia = contentImages + ', .msg-diagram-svg > svg';
  function regionOf(el) { return el.closest('.dinah-applet') || el.closest('.win-body') || el.closest('.content-body') || document.body; }
  function eligible(image) {
    // The page editor's caret helper (ProseMirror's separator) is not a picture.
    if (!image.matches(contentImages) || image.classList.contains('ProseMirror-separator') ||
      image.closest('.lightbox,a,button,[role="button"]:not([data-media-expand]),.glyph,.icon,.msg-diagram,[data-no-lightbox]')) return false;
    return !!image.closest('.sent-images') || !(image.complete && image.naturalWidth && image.naturalWidth <= 32 && image.naturalHeight <= 32);
  }
  function captionOf(el) {
    var figure = el.closest('figure'), text = figure && figure.querySelector('figcaption');
    return text ? text.textContent.trim() : el.getAttribute('alt') || el.getAttribute('aria-label') || 'Diagram';
  }
  function snapshot(source) {
    var item = { alt: source.getAttribute('alt') || '', caption: captionOf(source) };
    if (source.localName === 'svg') {
      var box = source.viewBox.baseVal;
      item.width = box.width || source.width.baseVal.value || 300;
      item.height = box.height || source.height.baseVal.value || 150;
      item.x = box.x || 0; item.y = box.y || 0;
      item.svg = window.dinahDiagrams.sanitize(new XMLSerializer().serializeToString(source));
    } else item.src = source.currentSrc || source.src;
    return item;
  }
  function galleryFor(source) {
    var nodes = Array.from(regionOf(source).querySelectorAll(contentMedia)).filter(function (el) { return el.localName === 'svg' || eligible(el); });
    if (!nodes.includes(source)) nodes = [source];
    return { items: nodes.map(snapshot), at: nodes.indexOf(source) };
  }
  function preload(i) { if (items[i] && items[i].src) { var image = new Image(); image.src = items[i].src; } }
  function unzoom() { dialog.classList.remove('lightbox-zoomed'); stage.scrollLeft = 0; stage.scrollTop = 0; }
  function naturalWidth() { return items[index] && items[index].width || img && img.naturalWidth || 0; }
  function naturalHeight() { return items[index] && items[index].height || img && img.naturalHeight || 0; }
  function markZoomable() {
    var box = img && img.getBoundingClientRect();
    dialog.classList.toggle('lightbox-zoomable', !!box && (naturalWidth() > box.width + 1 || naturalHeight() > box.height + 1 || dialog.classList.contains('lightbox-zoomed')));
  }
  function show(i) {
    if (i < 0 || i >= items.length) return;
    var generation = ++request;
    index = i; down = null; unzoom(); dialog.classList.remove('lightbox-zoomable');
    stage.replaceChildren(); img = null;
    var item = items[index];
    caption.textContent = item.caption; caption.hidden = !item.caption;
    counter.textContent = items.length > 1 ? (index + 1) + ' of ' + items.length : '';
    prevBtn.hidden = index === 0; nextBtn.hidden = index === items.length - 1;
    if (item.svg) {
      stage.innerHTML = item.svg;
      window.dinahWorkbench.namespaceSVG(stage); img = stage.querySelector('svg');
      img.setAttribute('viewBox', [item.x, item.y, item.width, item.height].join(' '));
      img.setAttribute('width', item.width); img.setAttribute('height', item.height);
      ['width', 'height', 'max-width', 'max-height'].forEach(function (name) { img.style.removeProperty(name); });
      img.classList.add('lightbox-img'); markZoomable();
    } else {
      var image = new Image(); image.className = 'lightbox-img'; image.alt = item.alt; image.draggable = false;
      image.onload = function () { if (generation === request && dialog.open) markZoomable(); };
      image.onerror = function () {
        if (generation !== request || !dialog.open) return;
        img = null; stage.textContent = 'The image could not be loaded.'; markZoomable();
      };
      img = image; stage.appendChild(image); image.src = item.src;
      if (image.complete) markZoomable();
    }
    preload(index - 1); preload(index + 1);
  }
  function openList(list, at) {
    items = list;
    if (!dialog.open) dialog.showModal();
    show(at); dialog.querySelector('[data-media-close]').focus();
  }
  function open(source, trigger, label) {
    opener = trigger || source;
    try {
      var gallery = galleryFor(source);
      if (label) gallery.items[gallery.at].caption = label;
      openList(gallery.items, gallery.at);
    } catch (error) {
      ++request; down = null; unzoom(); img = null; items = [];
      stage.textContent = 'The image could not be displayed: ' + String(error.message || error);
      caption.textContent = ''; counter.textContent = ''; prevBtn.hidden = nextBtn.hidden = true;
      markZoomable(); if (!dialog.open) dialog.showModal(); dialog.querySelector('[data-media-close]').focus();
    }
  }
  function zoomAt(x, y) {
    if (!img || !dialog.classList.contains('lightbox-zoomable')) return;
    var rect = img.getBoundingClientRect(), fx = (x - rect.left) / rect.width, fy = (y - rect.top) / rect.height;
    dialog.classList.add('lightbox-zoomed');
    stage.scrollLeft = fx * naturalWidth() - stage.clientWidth / 2;
    stage.scrollTop = fy * naturalHeight() - stage.clientHeight / 2;
  }
  prevBtn.addEventListener('click', function () { show(index - 1); });
  nextBtn.addEventListener('click', function () { show(index + 1); });
  dialog.querySelector('[data-media-close]').addEventListener('click', function () { dialog.close(); });
  dialog.addEventListener('keydown', function (event) {
    if (event.altKey || event.ctrlKey || event.metaKey) return;
    if (event.key === 'ArrowLeft') { event.preventDefault(); show(index - 1); }
    else if (event.key === 'ArrowRight') { event.preventDefault(); show(index + 1); }
    else if (event.key === 'Home') { event.preventDefault(); show(0); }
    else if (event.key === 'End') { event.preventDefault(); show(items.length - 1); }
    else if (event.key === '0') { event.preventDefault(); unzoom(); markZoomable(); }
    else if (event.key === '1' && img) { event.preventDefault(); var box = img.getBoundingClientRect(); zoomAt(box.left + box.width / 2, box.top + box.height / 2); }
  });
  window.addEventListener('resize', markZoomable);
  stage.addEventListener('pointerdown', function (event) {
    if (event.button !== 0) return;
    down = { id: event.pointerId, x: event.clientX, y: event.clientY, sl: stage.scrollLeft, st: stage.scrollTop, moved: false, type: event.pointerType, onImg: img && (event.target === img || img.contains(event.target)) };
    if (dialog.classList.contains('lightbox-zoomed') && event.pointerType === 'mouse') { stage.setPointerCapture(event.pointerId); event.preventDefault(); }
  });
  stage.addEventListener('pointermove', function (event) {
    if (!down || down.id !== event.pointerId) return;
    var dx = event.clientX - down.x, dy = event.clientY - down.y;
    if (Math.abs(dx) > 5 || Math.abs(dy) > 5) down.moved = true;
    if (dialog.classList.contains('lightbox-zoomed') && down.type === 'mouse') { stage.scrollLeft = down.sl - dx; stage.scrollTop = down.st - dy; }
  });
  stage.addEventListener('pointerup', function (event) {
    if (!down || down.id !== event.pointerId) return;
    var d = down; down = null;
    var dx = event.clientX - d.x, dy = event.clientY - d.y, zoomed = dialog.classList.contains('lightbox-zoomed');
    if (!zoomed && d.type === 'touch' && Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy) * 1.5) { show(index + (dx < 0 ? 1 : -1)); return; }
    if (d.moved) return;
    if (!d.onImg) { if (!zoomed) dialog.close(); return; }
    if (zoomed) { unzoom(); markZoomable(); return; }
    zoomAt(event.clientX, event.clientY);
  });
  stage.addEventListener('pointercancel', function () { down = null; });
  stage.addEventListener('lostpointercapture', function () { down = null; });
  dialog.addEventListener('click', function (event) { if (event.target === dialog) dialog.close(); });
  dialog.addEventListener('close', function () {
    if (dialog.open) return;
    ++request; down = null; unzoom(); img = null; items = []; stage.replaceChildren();
    var target = opener && opener.isConnected ? opener : document.getElementById('main'); opener = null;
    if (target) target.focus({ preventScroll: true });
  });
  function enhance(root) {
    var images = root.matches && root.matches('img') ? [root] : Array.from(root.querySelectorAll ? root.querySelectorAll('img') : []);
    images.forEach(function (image) {
      if (!eligible(image)) {
        if (image.dataset.mediaExpand) { delete image.dataset.mediaExpand; image.removeAttribute('tabindex'); image.removeAttribute('role'); image.removeAttribute('aria-label'); }
        var frame = image.parentElement;
        if (frame && frame.classList.contains('media-image-frame')) frame.replaceWith(image);
        return;
      }
      if (image.dataset.mediaExpand) {
        if (image.naturalWidth && image.parentElement.classList.contains('media-image-frame') && !image.parentElement.style.width) image.parentElement.style.width = image.naturalWidth + 'px';
        return;
      }
      image.dataset.mediaExpand = 'image'; image.tabIndex = 0;
      image.setAttribute('role', 'button'); image.setAttribute('aria-label', 'Expand Image' + (image.alt ? ': ' + image.alt : ''));
      var frame = document.createElement('span'), expand = document.createElement('button');
      frame.className = 'media-frame media-image-frame'; expand.type = 'button'; expand.className = 'icon-btn media-expand';
      if (image.naturalWidth) frame.style.width = (image.getBoundingClientRect().width || image.naturalWidth) + 'px';
      expand.innerHTML = '<span class="glyph" style="--glyph:var(--glyph-search)" aria-hidden="true"></span>';
      expand.setAttribute('aria-label', image.getAttribute('aria-label')); expand.title = expand.getAttribute('aria-label');
      expand.addEventListener('click', function () { open(image, expand, image.alt || 'Image'); });
      image.replaceWith(frame); frame.append(image, expand);
    });
  }
  function imageAction(event) {
    var image = event.target.closest && event.target.closest('img[data-media-expand]');
    if (!image || image.closest('a,button,[data-no-lightbox]') || event.defaultPrevented || dialog.open) return;
    if (event.type === 'keydown' && !['Enter',' '].includes(event.key) || event.type === 'click' && (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey)) return;
    event.preventDefault(); open(image, image, image.alt || 'Image');
  }
  document.addEventListener('click', imageAction); document.addEventListener('keydown', imageAction);
  document.addEventListener('load', function (event) { if (event.target.localName === 'img') enhance(event.target); }, true);
  new MutationObserver(function (changes) { changes.forEach(function (change) { change.addedNodes.forEach(enhance); }); }).observe(document.body, { childList: true, subtree: true });
  enhance(document.body);
  window.dinahMediaViewer = { open: open, enhance: enhance };
  // The viewer's keys, for Help › Keyboard Shortcuts.
  if (window.dinahCommands && window.dinahCommands.addHelp) {
    window.dinahCommands.addHelp('Image Viewer', [
      { label: 'Open an Image or Chart in the Viewer', key: 'Enter or Space on it' },
      { label: 'Previous or Next Image', key: 'Left / Right' },
      { label: 'First or Last Image', key: 'Home / End' },
      { label: 'Fit the Image', key: '0' },
      { label: 'Show the Image at Full Size', key: '1' },
      { label: 'Close the Viewer', key: 'Esc' }]);
  }
})();
