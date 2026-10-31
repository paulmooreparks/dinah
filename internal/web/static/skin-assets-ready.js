/* Reconcile failed static sheets before the first page body can be drawn. */
(function () {
  'use strict';
  function explain() {
    if (!document.body || !window.pudlSkins) return;
    const unavailable = pudlSkins.current() !== pudlSkins.preference();
    let notice = document.getElementById('skin-unavailable-notice');
    if (unavailable && !notice) {
      notice = document.createElement('div'); notice.id = 'skin-unavailable-notice'; notice.className = 'notice warning'; notice.setAttribute('role','status');
      const content = document.createElement('div'); content.className = 'notice-content';
      const title = document.createElement('p'); title.className = 'notice-title'; title.textContent = 'Skin unavailable';
      const body = document.createElement('p'); body.className = 'notice-body'; body.textContent = 'Standard is shown because the chosen skin could not load. Your saved choice is kept.';
      content.append(title,body); notice.append(content); (document.querySelector('.dinah-main') || document.body).prepend(notice);
    }
    if (notice) notice.hidden = !unavailable;
  }
  document.addEventListener('pudl:skin-change',explain);
  window.dinahSkinAssets?.reconcile();
  document.addEventListener('DOMContentLoaded',explain,{once:true});
})();
