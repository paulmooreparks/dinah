/* A named chat is created inside its group picker, without sending a prompt.
   The engine owns creation, defaults and cleanup when setup fails. */
(function () {
  'use strict';
  var H = window.dinahHelpers, node = H.node, serial = 0;
  function mount(root, o) {
    var prefix = 'group-new-' + (++serial), disposed = false, busy = false, epoch = 0, ready = false;
    var values = Object.assign({ workspace: '', provider: '', model: '', title: '' }, o.values), models = [];
    var form = node('form', 'picker-new-chat vstack gap-3'); form.noValidate = true;
    var heading = node('h2', 'section-title', 'New Chat'); heading.id = prefix + '-heading'; form.setAttribute('aria-labelledby', heading.id);
    var body = node('div', 'vstack gap-2'), fields = {};
    function field(key, label, select) {
      var group = node('div', 'form-group'), caption = node('label', 'form-label', label);
      var input = node(select ? 'select' : 'input', select ? 'form-select' : 'form-input');
      input.id = prefix + '-' + key; caption.htmlFor = input.id;
      if (select) { var wrap = node('div', 'form-select-wrap'); wrap.appendChild(input); group.append(caption, wrap); }
      else { input.type = 'text'; input.autocomplete = 'off'; group.append(caption, input); }
      fields[key] = input; body.appendChild(group); return input;
    }
    var workspace = field('workspace', 'Workspace', true), provider = field('provider', 'Provider', true);
    var model = field('model', 'Model', true), title = field('title', 'Title', false); title.required = true;
    var help = node('p', 'form-help', 'The provider’s default permissions apply. No message is sent. Cancelling Add Chats after creation keeps the created ordinary chat.');
    var status = node('div'); status.hidden = true; status.setAttribute('role', 'alert');
    var retry = node('button', 'btn btn-sm', 'Retry Choices'); retry.type = 'button'; retry.hidden = true;
    var actions = node('div', 'dialog-actions'), cancel = node('button', 'btn', 'Cancel'), create = node('button', 'btn btn-primary', 'Create Chat');
    cancel.type = 'button'; create.type = 'submit'; actions.append(cancel, create); form.append(heading, body, help, status, retry, actions); root.appendChild(form);
    function collect() { Object.keys(fields).forEach(function (key) { values[key] = fields[key].value; }); if (o.changed) o.changed(Object.assign({}, values)); }
    function error(text) {
      status.hidden = false; status.className = 'notice danger';
      var content = node('div', 'notice-content'); content.append(node('p', 'notice-title', 'The Chat Could Not Be Created'), node('p', 'notice-body', text)); status.replaceChildren(content);
    }
    function controls() { Object.values(fields).forEach(function (input) { input.disabled = busy || !ready; }); model.disabled = busy || !ready || !models.length; cancel.disabled = busy; create.disabled = busy || !ready || !workspace.value || !provider.value; }
    function loadModels(keep) {
      var request = ++epoch; ready = false; controls();
      return Promise.resolve().then(function () { return H.app().ProviderModelPreferences(provider.value); }).then(function (prefs) {
        if (disposed || request !== epoch) return;
        var option = (prefs.catalog || []).find(function (x) { return x.category === 'model'; }); models = [];
        if (option) (option.options || []).forEach(function (entry) { (entry.options || [entry]).forEach(function (x) { models.push(x); }); });
        model.replaceChildren(new Option('Provider Default', ''));
        models.forEach(function (x) { model.appendChild(new Option(x.name, x.value)); });
        var selected = keep || option && (prefs.defaults || {})[option.id] || '';
        model.value = models.some(function (x) { return x.value === selected; }) ? selected : '';
        ready = true; controls(); collect();
      }).catch(function (e) { if (!disposed && request === epoch) { error(H.message(e, 'sentence')); retry.hidden = false; } });
    }
    function load() {
      ready = false; controls(); retry.hidden = true; status.hidden = true;
      return Promise.all([H.app().State(), H.app().AgentProviders()]).then(function (all) {
        if (disposed) return;
        var state = all[0] || {}, providers = all[1] || [], records = state.workspaceRecords || [];
        if (!records.length) records = (state.workspaces || []).map(function (path) { return { name: path, executionPath: path }; });
        workspace.replaceChildren(); records.forEach(function (r) { workspace.appendChild(new Option(r.name + ' (' + r.executionPath + ')', r.executionPath)); });
        workspace.value = records.some(function (r) { return r.executionPath === values.workspace; }) ? values.workspace : state.workspace || '';
        if (!workspace.value && workspace.options.length) workspace.selectedIndex = 0;
        provider.replaceChildren(); providers.forEach(function (p) { var item = new Option(p.name + (p.statusWords ? ' (' + p.statusWords + ')' : ''), p.id); item.disabled = !p.installed; provider.appendChild(item); });
        var selected = values.provider || state.defaultProvider;
        provider.value = providers.some(function (p) { return p.id === selected && p.installed; }) ? selected : (providers.find(function (p) { return p.installed; }) || {}).id || '';
        title.value = values.title;
        if (!provider.value || !workspace.value) { error('Choose an available provider and workspace first.'); return; }
        return loadModels(values.model);
      }).catch(function (e) { if (!disposed) { error(H.message(e, 'sentence')); retry.hidden = false; } });
    }
    provider.addEventListener('change', function () { values.provider = provider.value; values.model = ''; status.hidden = true; loadModels(''); });
    [workspace, model, title].forEach(function (input) { input.addEventListener('input', collect); input.addEventListener('change', collect); });
    cancel.addEventListener('click', function () { if (!busy) o.cancel(); });
    form.addEventListener('keydown', function (e) { if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); if (!busy) o.cancel(); } });
    retry.addEventListener('click', load);
    form.addEventListener('submit', function (e) {
      e.preventDefault(); if (busy || !ready) return; collect();
      if (!values.title.trim()) { error('Give the chat a title.'); title.focus(); return; }
      busy = true; controls(); status.hidden = true;
      Promise.resolve().then(function () { return H.app().CreateNamedChat(values.workspace, values.provider, values.model, values.title.trim()); }).then(function (info) {
        // Even if its picker was closed during the request, the created chat
        // remains an ordinary chat in the application's lists.
        H.events.dispatch('agent:session', info);
        var ref = window.dinahChats.ref(info.sessionId) || { id: info.sessionId, cwd: info.cwd || values.workspace, title: info.title || values.title.trim(), provider: values.provider, workspace: values.workspace, state: 'open', untitled: false };
        if (!disposed) o.created(ref);
      }).catch(function (e) { if (!disposed) error(H.message(e, 'sentence')); }).finally(function () { busy = false; if (!disposed) controls(); });
    });
    load().then(function () { if (!disposed && ready) title.focus(); });
    return { focus: function () { if (ready && !busy) title.focus(); }, destroy: function () { disposed = true; epoch++; form.remove(); }, values: function () { return Object.assign({}, values); } };
  }
  window.dinahGroupNewChat = { mount: mount };
})();
