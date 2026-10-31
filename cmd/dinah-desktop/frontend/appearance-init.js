// Native preferences replace this first-paint cache when the bridge responds.
(function () {
  var root = document.documentElement, cached = {};
  try { cached = JSON.parse(localStorage.getItem('dinah-appearance-startup-v1') || '{}'); } catch (_) { /* Storage is optional. */ }
  if (!cached || typeof cached !== 'object' || Array.isArray(cached)) cached = {};
  var mode = cached.mode;
  if (!['light', 'dark', 'system'].includes(mode)) {
    // Read the former key only to preserve the first frame during migration.
    try { mode = localStorage.getItem('pudl-theme'); } catch (_) { /* Storage is optional. */ }
  }
  root.dataset.themePref = ['light', 'dark', 'system'].includes(mode) ? mode : 'light';
  root.dataset.skin = typeof cached.skin === 'string' && /^[a-z][a-z0-9-]{0,31}$/.test(cached.skin) ? cached.skin : 'standard';
})();
