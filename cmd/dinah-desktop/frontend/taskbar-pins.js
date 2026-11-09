/* Native-profile taskbar preferences use explicit workbench identities. */
(function () {
  'use strict';
  var store = 'dinah-taskbar-pins-v1', scope = 'dinah-native-profile', ready = false;
  var bench = window.dinahWorkbench;
  function shellInfo(id) {
    var item = bench.item(id);
    if (!item || item.kind !== 'shell') return null;
    var shell = item.path === 'default' ? {id:'default',name:'Default Shell'} : dinahShellTerminal.shells().find(function (s) { return s.id === item.path; });
    if (!shell) return null;
    return {label:shell.name,glyph:'terminal',open:function () {
      return dinahResources.currentPath().then(function (path) { return dinahResources.openTerminal(path,null,shell.id === 'default' ? undefined : shell.id); });
    }};
  }
  function info(provider,id) { return provider === 'shells' ? shellInfo(id) : bench.pinResolve(provider,id); }
  var providers = ['applets','items','shells'].map(function (provider) {
    return {id:provider,resolve:function (request) {
      var found = info(provider,request.id);
      return found ? {status:'available',destination:{id:request.id,label:found.label,glyph:found.glyph,icon:found.icon,command:true}} : {status:'unavailable'};
    }};
  });
  function initialize() {
    if (ready) return;
    var saved = dinahUIState.get(store,{version:1,pins:[],order:[],references:[]});
    pudlWindows.configureTaskbar({providers:providers,scope:scope,pins:saved.pins || [],order:saved.order || [],references:saved.references || []});
    ready = true;
  }
  document.addEventListener('pudl:taskbar-change',function () {
    if (ready) dinahUIState.set(store,Object.assign({version:1},pudlWindows.taskbarState()));
  });
  document.addEventListener('pudl:taskbar-activate',function (event) {
    if (!ready) return;
    var request = event.detail;
    var ref = pudlWindows.taskbarState().references.find(function (r) { return r.id === request.id; });
    if (!ref) return;
    event.preventDefault();
    request.waitUntil(Promise.resolve().then(function () {
      var found = info(ref.provider,ref.id);
      if (!found) throw new Error('This pinned destination is unavailable');
      return found.open();
    }).then(function (key) { return bench.ready(key); }));
  });
  document.addEventListener('pudl:taskbar-activation-end',function (event) {
    if (event.detail.status === 'failed') document.dispatchEvent(new CustomEvent('dinah:menu-error',{detail:event.detail.error || new Error('Pinned destination could not be opened')}));
  });
  /* Resolving the destinations again resolves each pinned one, and PUDL
     redraws the taskbar after each; a document's title changes with every
     pause in typing (its "(changed)" mark), so the refresh waits for a
     quiet quarter second and runs once for a burst of changes. */
  var refreshTimer = 0;
  function refreshDestinationsSoon() {
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(function () { refreshTimer = 0; if (ready) pudlWindows.refreshTaskbarDestinations(); }, 250);
  }
  document.addEventListener('dinah:pin-categories-change', refreshDestinationsSoon);
  document.addEventListener('dinah:launchers-change', refreshDestinationsSoon);
  document.addEventListener('dinah:workspace-selected', refreshDestinationsSoon);
  function reference(action) {
    if (action.indexOf('terminal:') === 0 && action !== 'terminal:refresh') return {provider:'shells',id:bench.key({kind:'shell',path:action === 'terminal:default' ? dinahShellTerminal.defaultShell() : action.slice(9)})};
    if (action === 'new-document') return {provider:'applets',id:'app-editor'};
    if (action.indexOf('tool:') === 0) return bench.launcherPinReference(action.slice(5));
    var kind = action.indexOf('tool:') === 0 ? action.slice(5) : ({'new-document':'editor','new-chat':'chat','show-workspace':'workspace','needs-you':'needs-you'})[action] || action;
    return bench.pinReference({kind:kind});
  }
  window.dinahTaskbarPins = {reference:reference};
  if (document.documentElement.dataset.uiStateReady === 'true') initialize();
  else document.addEventListener('dinah:ui-state-restored',initialize,{once:true});
})();
