/* Native application fullscreen is independent of browser and content windows. */
(function () {
  'use strict';
  var app = window.dinahHelpers.app(), state = { available: false, fullscreen: false }, busy = false, reading = null;
  var topbar = document.querySelector('.topbar');
  if (!topbar || !window.pudlNative) return;
  function report(error) { document.dispatchEvent(new CustomEvent('dinah:menu-error', { detail: error })); }
  // The title is the product's name (product.js), drawn again when State
  // reports the build's own.
  document.addEventListener('dinah:product', function (e) { if (chrome && e.detail.name) chrome.update({ title: e.detail.name }); });
  var chrome = pudlNative.attach(topbar, { title: window.dinahProduct ? window.dinahProduct.name : '', icon: '/dinah-mark.svg', state: { fullscreen: false }, actions: {
    minimize: function () { return action('minimize'); },
    restore: function () { return action('restore'); },
    close: function () { return action('close'); }
  } });
  function refresh() {
    if (reading) return reading;
    if (typeof app?.NativeWindowState !== 'function') return Promise.resolve();
    reading = Promise.resolve().then(function () { return app.NativeWindowState(); }).then(function (snapshot) {
      if (!snapshot || typeof snapshot.available !== 'boolean' || typeof snapshot.fullscreen !== 'boolean') return;
      var changed = state.available !== snapshot.available || state.fullscreen !== snapshot.fullscreen;
      state = snapshot;
      chrome.update({ fullscreen: state.available && state.fullscreen });
      if (changed) window.dinahCommands.flush({refreshOpenMenus:true});
    }).finally(function () { reading = null; });
    return reading;
  }
  async function action(name) {
    if (!state.available || busy) return false;
    busy = true;window.dinahCommands.flush({refreshOpenMenus:true});
    try { await app.NativeWindowAction(name); await refresh(); return true; }
    finally { busy = false;window.dinahCommands.flush({refreshOpenMenus:true}); }
  }
  window.dinahNativeWindow = { toggle: function () { return action('toggle').catch(function (error) { report(error); throw error; }); }, refresh: refresh };
  window.dinahCommands.register({ id: 'fullscreen', menu: 'view', mnemonic: 'F', label: 'Full Screen', shortcut: 'F11',
    visible: function () { return state.available; }, enabled: function () { return !busy; }, checked: function () { return state.fullscreen; },
    run: function () { return action('toggle'); } });
  function fullscreenKey(event) { return event.key==='F11'&&!event.ctrlKey&&!event.metaKey&&!event.altKey&&!event.shiftKey&&state.available; }
  document.addEventListener('keydown', function (event) {
    if(!fullscreenKey(event)||event.defaultPrevented)return;
    if(event.repeat||event.isComposing){event.preventDefault();event.stopPropagation();return;}
    window.dinahCommands.flush({refreshOpenMenus:true});
    if(document.querySelector('dialog[open]')){
      event.preventDefault();event.stopPropagation();action('toggle').catch(report);
    }
  }, true);
  // PUDL normally dispatches F11 through the View command. Reserve the key
  // even while that command is disabled, so the browser never takes it.
  window.addEventListener('keydown', function(event){
    if(!fullscreenKey(event)||event.defaultPrevented)return;
    event.preventDefault();
    if(!event.repeat&&!event.isComposing)action('toggle').catch(report);
  });
  topbar.addEventListener('pudl:native-error', function (event) { report(event.detail.error); });
  window.addEventListener('focus', function () { refresh().catch(report); });
  document.addEventListener('visibilitychange', function () { if (!document.hidden) refresh().catch(report); });
  setInterval(function () { if (!document.hidden) refresh().catch(report); }, 1000);
  refresh().catch(report);
})();
