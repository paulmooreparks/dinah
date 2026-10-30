/* Cooperative rendering for Dinah's live views. A draw callback does one
   bounded piece of work and returns true if it has more. */
(function () {
  'use strict';
  var pending = new Map(), roots = new Map(), frame = 0, running = false, invalidated = false;
  var budget = 6;
  var observer = new ResizeObserver(wake);
  function watch(root) {
    if (!root) return;
    var count = roots.get(root) || 0;
    if (!count) observer.observe(root);
    roots.set(root, count + 1);
  }
  function unwatch(root) {
    if (!root) return;
    var count = roots.get(root) - 1;
    if (count) roots.set(root, count);
    else { roots.delete(root); observer.unobserve(root); }
  }
  function visible(root) {
    return !root || (root.isConnected && root.getClientRects().length > 0);
  }
  function wake() {
    if (!running && !frame && pending.size && !document.hidden) frame = requestAnimationFrame(flush);
  }
  function remove(key) {
    var job = pending.get(key);
    if (!job) return;
    pending.delete(key); unwatch(job.root);
    if (!pending.size && frame) { cancelAnimationFrame(frame); frame = 0; }
  }
  function enqueue(key, root, draw, after, label) {
    if (running) invalidated = true;
    var previous = pending.get(key);
    if (previous && previous.root === root) {
      previous.draw = draw; previous.after = after; previous.version++;
    } else {
      remove(key);
      pending.set(key, { root: root, draw: draw, after: after, version: 0, since: performance.now(), label: label || 'view update' });
      watch(root);
    }
    if (!previous || previous.root !== root || !previous.blocked) wake();
  }
  function flush() {
    frame = 0;
    if (document.hidden) return;
    running = true;
    invalidated = false;
    // Read visibility before any writes. Hidden jobs remain queued without
    // requesting frames until a window or size change wakes the queue.
    var ready = [];
    pending.forEach(function (job, key) {
      var wasBlocked = job.blocked;
      job.blocked = !visible(job.root);
      if (job.blocked || wasBlocked) job.since = performance.now();
      if (!job.blocked) ready.push([key, job]);
    });
    var deadline = performance.now() + budget, after = [], more = false;
    try {
      for (var i = 0; i < ready.length; i++) {
        if (i && performance.now() >= deadline) { more = true; break; }
        var key = ready[i][0], job = ready[i][1];
        if (pending.get(key) !== job) continue;
        // Move unfinished work to the tail so a busy view cannot starve
        // the others. A job can run only once in this frame.
        pending.delete(key); pending.set(key, job);
        try {
          var version = job.version;
          var started = performance.now(), wait = started - job.since;
          if (job.draw(deadline) === true || version !== job.version) more = true;
          else if (pending.get(key) === job) remove(key);
          if (window.dinahResponsiveness) window.dinahResponsiveness.record('render', job.label, performance.now() - started, wait);
          job.since = performance.now();
          if (job.after) after.push({ fn: job.after, label: job.label });
        } catch (error) {
          remove(key);
          setTimeout(function (err) { return function () { throw err; }; }(error), 0);
        }
      }
      // Geometry reads, including chat scrolling, follow all frame writes.
      after.forEach(function (entry) {
        try {
          var started = performance.now();
          entry.fn();
          if (window.dinahResponsiveness) window.dinahResponsiveness.record('render', 'layout follow-up', performance.now() - started, 0);
        }
        catch (error) { setTimeout(function () { throw error; }, 0); }
      });
    } finally {
      running = false;
      if (more || invalidated) wake();
    }
  }
  document.addEventListener('pudl:windows-change', wake);
  document.addEventListener('visibilitychange', function () {
    pending.forEach(function (job) { job.since = performance.now(); });
    wake();
  });
  window.dinahRender = { enqueue: enqueue, cancel: remove, wake: wake };
})();
