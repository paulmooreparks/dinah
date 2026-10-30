// The light theme is the default when the person has not chosen one. This
// runs before the page is drawn, from its own file, because the page's
// Content Security Policy runs no inline script.
(function () {
  var saved = null;
  try { saved = localStorage.getItem('pudl-theme'); } catch (e) { /* storage blocked */ }
  if (!saved) pudlSetTheme('light');
})();
