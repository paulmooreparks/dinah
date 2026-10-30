/* Timing only. No event targets, clipboard contents or conversation data. */
(function () {
  'use strict';
  var enabled = false, samples = [], observers = [], timer = 0, since = 0;
  var support = {}, panel = document.getElementById('responsiveness-results');
  var toggle = document.getElementById('set-responsiveness');
  var key = 'dinah-responsiveness';
  function prune() {
    var cutoff = performance.now() - 60000;
    samples = samples.filter(function (s) { return s.at >= cutoff; }).slice(-200);
  }
  function record(kind, label, duration, wait) {
    if (!enabled || document.hidden) return;
    if (kind === 'render' && duration < 6 && wait < 16) return;
    samples.push({ at: performance.now(), kind: kind, label: label, duration: duration, wait: wait || 0 });
    prune();
  }
  function snapshot() { prune(); return samples.map(function (s) { return Object.assign({}, s); }); }
  function draw() {
    panel.hidden = !enabled;
    if (!enabled) return;
    var list = snapshot();
    function peak(kind, field) { return Math.round(Math.max.apply(null, [0].concat(list.filter(function (s) { return s.kind === kind; }).map(function (s) { return s[field]; })))); }
    var stalls = list.filter(function (s) { return s.kind === 'stall'; });
    panel.querySelector('[data-input-delay]').textContent = support.event ? peak('input', 'duration') + ' ms maximum reported input delay' : 'Input timing unavailable in this browser';
    panel.querySelector('[data-stalls]').textContent = support.longtask ? stalls.length + ' recorded stalls; longest ' + peak('stall', 'duration') + ' ms' : 'Long-task timing unavailable in this browser';
    panel.querySelector('[data-render]').textContent = 'Scheduled work: longest task ' + peak('render', 'duration') + ' ms; longest wait ' + peak('render', 'wait') + ' ms';
    var rows = panel.querySelector('tbody'); rows.replaceChildren();
    list.slice(-20).reverse().forEach(function (s) {
      var row = document.createElement('tr');
      [Math.round((performance.now() - s.at) / 1000) + 's ago', s.label, Math.round(s.duration) + ' ms', Math.round(s.wait) + ' ms'].forEach(function (value) {
        var cell = document.createElement('td'); cell.textContent = value; row.appendChild(cell);
      });
      rows.appendChild(row);
    });
  }
  function refresh() { window.dinahRender.enqueue(panel, panel, draw, null, 'diagnostics'); }
  function stop() {
    observers.forEach(function (o) { o.disconnect(); }); observers = [];
    clearInterval(timer); timer = 0;
    window.dinahRender.cancel(panel);
  }
  function start() {
    stop();
    if (!enabled || document.hidden) return;
    since = performance.now();
    var types = window.PerformanceObserver && PerformanceObserver.supportedEntryTypes || [];
    ['event', 'longtask'].forEach(function (type) {
      support[type] = types.includes(type);
      if (!support[type]) return;
      try {
        var observer = new PerformanceObserver(function (entries) {
          entries.getEntries().forEach(function (entry) {
            if (entry.startTime < since) return;
            record(type === 'event' ? 'input' : 'stall', type === 'event' ? 'Input processing delay' : 'Main-thread task', type === 'event' ? entry.processingStart - entry.startTime : entry.duration, 0);
          });
        });
        observer.observe(type === 'event' ? { type: type, durationThreshold: 16 } : { type: type });
        observers.push(observer);
      } catch (_) { support[type] = false; }
    });
    timer = setInterval(refresh, 1000); refresh();
  }
  function set(value) {
    enabled = value === true;
    toggle.setAttribute('aria-checked', String(enabled));
    panel.hidden = !enabled;
    if (!enabled) samples = [];
    start();
  }
  toggle.addEventListener('click', function () {
    set(!enabled); window.dinahUIState.set(key, enabled);
  });
  document.getElementById('clear-responsiveness').addEventListener('click', function () { samples = []; refresh(); });
  document.addEventListener('visibilitychange', start);
  window.dinahUIState.ready.then(function () { set(window.dinahUIState.get(key, false)); });
  window.dinahResponsiveness = { record: record, snapshot: snapshot };
})();
