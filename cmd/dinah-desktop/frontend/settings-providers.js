/* The provider-specific parts of Settings, Agents. Each provider that has a surface of its own
   (provider-ollama.js and the like) registers a module here: mount(root, api) fills a container
   with the provider's markup and returns { apply(state) }, which is given what State says. The
   page mounts the module of each provider the engine's list has, in the list's order, and
   leaves the rest out, so shared code names no provider (docs/plans/group-chat.md, "No
   Hard-Coded Providers or Models"). A module is the provider's own and may name it. */
(function () {
  'use strict';

  var modules = new Map(), mounted = new Map(), api = null, lastState = null;

  function register(id, module) { modules.set(id, module); }
  // The calls the shared Settings page lends to the modules.
  function configure(shared) { api = shared; }

  function apply(state) {
    lastState = state;
    mounted.forEach(function (entry) { if (entry.view && entry.view.apply) entry.view.apply(state); });
  }

  /* Mounts the module of each listed provider that has one, once, in the list's order, and
     takes away one whose provider is no longer listed. */
  function draw(providers) {
    var host = document.getElementById('provider-modules');
    if (!host || !api) return;
    var listed = (providers || []).filter(function (p) { return modules.has(p.id); });
    mounted.forEach(function (entry, id) {
      if (!listed.some(function (p) { return p.id === id; })) { entry.root.remove(); mounted.delete(id); }
    });
    listed.forEach(function (p) {
      var entry = mounted.get(p.id);
      if (!entry) {
        var root = document.createElement('div');
        root.className = 'vstack gap-3'; root.dataset.providerModule = p.id;
        entry = { root: root, view: modules.get(p.id).mount(root, Object.assign({ name: p.name || '' }, api)) };
        mounted.set(p.id, entry);
        if (lastState && entry.view && entry.view.apply) entry.view.apply(lastState);
      }
      host.appendChild(entry.root);
    });
  }

  /* Gives a provider's module the container in the provider's row of the list, with the provider's entry
     and what the row lends (its status line, and replace, which puts a changed entry in the list). */
  function drawRow(container, provider, ctx) {
    var module = modules.get(provider.id);
    if (module && module.row && api) module.row(container, provider, Object.assign({}, api, ctx));
  }

  window.dinahProviderSettings = { register: register, configure: configure, apply: apply, draw: draw, drawRow: drawRow };
})();
