/* Record the browser's stylesheet error event before static links load. */
(function () {
  'use strict';
  const failed = new Set();
  function reconcile() { failed.forEach(link => window.pudlSkins?.unregister(link.dataset.pudlSkin)); }
  window.dinahSkinAssets = { failed, reconcile };
  document.addEventListener('error', event => {
    const link = event.target;
    if (!(link instanceof HTMLLinkElement) || !link.dataset.pudlSkin) return;
    failed.add(link);
    reconcile();
  }, true);
  document.addEventListener('load', event => {
    const link = event.target;
    if (link instanceof HTMLLinkElement && link.dataset.pudlSkin) { failed.delete(link); window.pudlSkins?.register(link.dataset.pudlSkin,link); }
  }, true);
})();
