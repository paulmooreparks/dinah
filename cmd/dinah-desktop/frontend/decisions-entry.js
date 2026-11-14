/* The status count and attention links share one lightweight, complete snapshot. */
(function () {
  'use strict';
  var current = { count: 0, items: [] }, byMessage = new Map(), pending = null, revision = 0, timer = null, known = false, failure = '';
  function app() { return window.dinahHelpers.app(); }
  function available() { return !!(app() && app().DecisionEntries); }
  function draw() {
    var item = document.getElementById('status-decisions');
    if (!item) return;
    item.hidden = !available();
    var badge = item.querySelector('[data-status-badge]');
    badge.textContent = known ? current.count > 99 ? '99+' : String(current.count) : failure ? '?' : '…';
    badge.toggleAttribute('data-status-pending', !known && !failure);
    item.dataset.statusState = known && current.count ? 'waiting' : 'idle';
    item.dataset.statusText = failure ? 'Unavailable' : known ? current.count + ' open' : 'Reading';
    item.dataset.statusColour = failure || known && current.count ? 'warn' : 'text-muted';
    item.querySelector('.glyph').style.color = 'var(--' + item.dataset.statusColour + ')';
    badge.classList.toggle('warn', !!(failure || known && current.count));
    item.dataset.statusCount = known ? String(current.count) : '0';
    item.toggleAttribute('data-status-count-known', known);
    if (known && current.count) item.setAttribute('data-status-tone', 'waiting'); else item.removeAttribute('data-status-tone');
    var label = failure ? 'Decisions: open requests could not be read' : known ? 'Decisions: ' + current.count + (current.count === 1 ? ' open request' : ' open requests') : 'Decisions: reading open requests';
    item.title = label; item.setAttribute('aria-label', label);
  }
  function changed() { draw(); document.dispatchEvent(new CustomEvent('dinah:decision-entries')); }
  function refresh() {
    clearTimeout(timer); timer = null;
    if (!available()) { known = false; changed(); return Promise.resolve(current); }
    if (pending) return pending;
    var at = revision;
    pending = Promise.resolve().then(function () { return app().DecisionEntries(); }).then(function (value) {
      if (!value || !Number.isSafeInteger(value.count) || value.count < 0 || !Array.isArray(value.items) || value.count !== value.items.length || !value.items.every(function (entry) { return entry && typeof entry.id === 'string' && entry.id && Number.isSafeInteger(entry.messageId) && entry.messageId > 0; }) || new Set(value.items.map(function (entry) { return entry.id; })).size !== value.count || new Set(value.items.map(function (entry) { return entry.messageId; })).size !== value.count) throw new Error('The Decisions count is invalid.');
      if (at !== revision) return;
      current = value; byMessage = new Map(value.items.map(function (entry) { return [entry.messageId, entry.id]; })); known = true; failure = ''; changed();
    }).catch(function (error) { if (at === revision) { known = false; failure = String(error); changed(); } }).then(function () {
      pending = null;
      if (at !== revision) return refresh();
      if (failure) throw new Error('Open requests could not be read. Try again.');
      return current;
    });
    return pending;
  }
  function invalidate() { revision++; clearTimeout(timer); timer = setTimeout(function () { refresh().catch(function () {}); }, 75); }
  window.dinahDecisionEntries = {
    available: available,
    get: function (message) { return byMessage.get(message) || null; },
    resolve: function (message) { return refresh().then(function () { return byMessage.get(message) || null; }); },
    refresh: refresh
  };
  document.getElementById('status-decisions').addEventListener('click', function () { window.dinahWorkbench.open('decisions').catch(window.dinahHost.fail); });
  window.dinahHost.on('bus:changed', invalidate);
  window.dinahHost.onResync(function () { revision++; return refresh(); });
  window.dinahUIState.ready.then(function () { return refresh(); }).catch(function () {});
}());
