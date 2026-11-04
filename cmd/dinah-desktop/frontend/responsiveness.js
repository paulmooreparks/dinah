/* Timing only. Producer labels and browser entry objects are never exported. */
(function () {
  'use strict';
  var api = window.dinahTimingSnapshot, recorder = api.create();
  var enabled = false, generation = 0, observers = [], timer = 0;
  var panel = document.getElementById('responsiveness-results');
  var toggle = document.getElementById('set-responsiveness');
  var copyButton = document.getElementById('copy-responsiveness');
  var status = document.getElementById('responsiveness-copy-status');
  var key = 'dinah-responsiveness', backend = null, control = null, sentGeneration = -1, copying = false, copyPhase = '', preparation = null;
  var COPY_DEADLINE_MS = 10000;
  function showStatus(text) { status.textContent = text; status.hidden = !text; }
  function app() { return window.dinahHelpers.app(); }
  function clock() { return { utc: new Date().toISOString(), monoMs: performance.now() }; }
  function record(kind, label, duration, wait) {
    if (!enabled || document.hidden) return;
    if (kind === 'render') recorder.record(label, duration, wait || 0);
  }
  function draw() {
    panel.hidden = !enabled;
    if (!enabled) return;
    var view = recorder.panel(), support = view.features;
    panel.querySelector('[data-input-delay]').textContent = support.eventTiming === 'measured' ? Math.round(view.inputPeak) + ' ms maximum reported input delay' : 'Input timing unavailable in this browser';
    panel.querySelector('[data-stalls]').textContent = support.longTasks === 'measured' ? view.taskCount + ' recorded stalls; longest ' + Math.round(view.taskPeak) + ' ms' : 'Long-task timing unavailable in this browser';
    panel.querySelector('[data-render]').textContent = 'Scheduled work: longest task ' + Math.round(view.renderPeak) + ' ms; longest wait ' + Math.round(view.waitPeak) + ' ms';
    panel.querySelector('[data-long-frames]').textContent = support.longAnimationFrames === 'measured' ? Math.round(view.framePeak) + ' ms longest reported animation frame' : 'Long-frame timing unavailable in this browser';
    var rows = panel.querySelector('tbody'); rows.replaceChildren();
    view.items.slice(-20).reverse().forEach(function (sample) {
      var row = document.createElement('tr');
      [Math.max(0, Math.round((performance.now() - sample.at) / 1000)) + 's ago', sample.label, Math.round(sample.duration) + ' ms', Math.round(sample.wait) + ' ms'].forEach(function (value) {
        var cell = document.createElement('td'); cell.textContent = value; row.appendChild(cell);
      }); rows.appendChild(row);
    });
  }
  function refresh() { window.dinahRender.enqueue(panel, panel, draw, null, 'diagnostics'); }
  function stopObservers() {
    observers.forEach(function (observer) { observer.disconnect(); }); observers = [];
    clearInterval(timer); timer = 0; window.dinahRender.cancel(panel);
  }
  function startObservers() {
    stopObservers();
    if (!enabled || document.hidden) return;
    var types = window.PerformanceObserver && PerformanceObserver.supportedEntryTypes || [];
    [['event', 'eventTiming', recorder.input], ['longtask', 'longTasks', recorder.task], ['long-animation-frame', 'longAnimationFrames', recorder.frame]].forEach(function (source) {
      if (!types.includes(source[0])) { recorder.feature(source[1], 'not_supported'); return; }
      try {
        var observer = new PerformanceObserver(function (entries) {
          if (!enabled || document.hidden) return;
          entries.getEntries().forEach(source[2]);
        });
        observer.observe(source[0] === 'event' ? { type: 'event', durationThreshold: 16 } : { type: source[0] });
        observers.push(observer); recorder.feature(source[1], 'measured');
      } catch (_) { recorder.feature(source[1], 'unavailable'); }
    });
    timer = setInterval(refresh, 1000); refresh();
  }
  // Requests are serialized, including changes made while a bridge call is pending.
  function syncBackend() {
    if (control) return control;
    control = (async function () {
      while (sentGeneration !== generation) {
        var mine = generation, wanted = enabled; sentGeneration = mine; backend = null;
        try {
          var service = app();
          if (!service || typeof service.SetResponsivenessRecording !== 'function') break;
          var state = api.validate('RecordingState', await service.SetResponsivenessRecording(wanted));
          if (mine === generation && state.enabled === wanted) backend = { browserGeneration: mine, generation: state.generation };
        } catch (_) { backend = null; }
      }
    })().finally(function () { control = null; if (sentGeneration !== generation) syncBackend(); });
    return control;
  }
  function set(value) {
    var changed = enabled !== (value === true);
    if (changed) { enabled = value === true; generation++; }
    toggle.setAttribute('aria-checked', String(enabled)); panel.hidden = !enabled;
    if (changed || generation === 0) {
      if (enabled) { recorder.begin(generation); if (document.hidden) recorder.visibility(true); }
      else recorder.end(generation);
    }
    showStatus(copying && copyPhase === 'writing' ? 'A requested timing copy is still waiting for the clipboard. Turning recording off does not cancel it.' : '');
    copyButton.disabled = !enabled || copying;
    startObservers(); syncBackend();
  }
  async function copySnapshot() {
    if (!enabled || copying) return;
    if (preparation) { showStatus('A previous timing request is still pending. Try copying again after it finishes.'); return; }
    var mine = generation, start = clock(), browser = recorder.snapshot();
    copying = true; copyPhase = 'preparing'; copyButton.disabled = true; showStatus('');
    var timer;
    var expired = function () { return performance.now() - start.monoMs >= COPY_DEADLINE_MS; };
    async function prepare() {
      await syncBackend();
      if (expired() || !enabled || mine !== generation || !backend || backend.browserGeneration !== mine) throw new Error();
      var expectedBackendGeneration = backend.generation, service = app();
      if (!service || typeof service.GetResponsivenessTimingSnapshot !== 'function') throw new Error();
      var captured = api.validate('BackendSnapshot', await service.GetResponsivenessTimingSnapshot());
      if (expired() || !enabled || mine !== generation || !captured.recording.enabled || captured.recording.generation !== expectedBackendGeneration) throw new Error();
      var copiedAt = clock();
      return api.serialize({ schemaVersion: 1, copiedAt: copiedAt, copyRequest: { anchor: browser.anchor, startMonoMs: start.monoMs, durationMs: copiedAt.monoMs - start.monoMs, complete: true }, browser: browser, backend: captured });
    }
    try {
      var pending = preparation = prepare();
      var settled = function () { if (preparation === pending) preparation = null; };
      pending.then(settled, settled);
      var text = await Promise.race([pending, new Promise(function (_, reject) { timer = setTimeout(function () { reject(new Error()); }, Math.max(0, COPY_DEADLINE_MS - (performance.now() - start.monoMs))); })]);
      clearTimeout(timer);
      if (expired() || !enabled || mine !== generation) throw new Error();
      copyPhase = 'writing'; showStatus('Timing copy submitted; waiting for clipboard.');
      if (window.runtime && typeof window.runtime.ClipboardSetText === 'function') {
        if (await window.runtime.ClipboardSetText(text) === false) throw new Error();
      } else if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') await navigator.clipboard.writeText(text);
      else throw new Error();
      showStatus('The requested timing snapshot was copied. It includes retained input-event times; review it before sharing.');
    } catch (_) {
      showStatus(mine !== generation || !enabled ? 'Recording changed. Turn recording on and copy again.' : 'The timing snapshot could not be copied. Timing data may be unavailable.');
    } finally { clearTimeout(timer); copying = false; copyPhase = ''; copyButton.disabled = !enabled; }
  }
  toggle.addEventListener('click', function () { set(!enabled); window.dinahUIState.set(key, enabled); });
  copyButton.addEventListener('click', copySnapshot);
  document.getElementById('clear-responsiveness').addEventListener('click', async function () {
    // Clear both sources through the same serialized generation transitions.
    if (enabled) {
      set(false); var mine = generation;
      await syncBackend();
      if (!enabled && generation === mine) set(true);
    }
  });
  document.addEventListener('visibilitychange', function () {
    recorder.visibility(document.hidden); startObservers();
  });
  window.dinahUIState.ready.then(function () { set(window.dinahUIState.get(key, false)); });
  window.dinahResponsiveness = { record: record, snapshot: function () { return recorder.panel().items; } };
})();
