/* The UI SDK is for reviewed, bundled applets in the host trust boundary. */
(function () {
  'use strict';
  var workspace = '', contributions = new Map(), mounts = [], listeners = new Set();
  var requestTimer = 0;
  const api = window.dinahHelpers.app;
  const node = window.dinahHelpers.node;
  function button(text, fn) { var b = node('button', 'btn btn-sm', text); b.type = 'button'; b.addEventListener('click', fn); return b; }
  function error(err) { if (window.pudlToast) window.pudlToast(String(err), {kind:'danger'}); }
  function change(value) { listeners.forEach(function (fn) { fn(value); }); refreshRequests().catch(error); }
  function scoped(provider, ws) {
    return Object.freeze({
      invoke: function (ref, input) { return api().InvokeAppletAt(ws, provider, ref, JSON.stringify(input)).then(function (v) { change(v); return v; }); },
      invokeJSON: function (ref, inputJSON) { return api().InvokeAppletAt(ws, provider, ref, inputJSON).then(function (v) { change(v); return v; }); },
      wait: async function (id, signal) {
        for (;;) {
          if (signal && signal.aborted) throw new Error('The view stopped waiting. The invocation is still available in Workspace.');
          var v = await api().AppletInvocationAt(ws,id);
          if (v.status === 'done') { change(v); return v; }
          if (!['running','waiting'].includes(v.status)) throw new Error(v.error || 'The applet action ' + v.status + '.');
          await new Promise(function (resolve) { setTimeout(resolve,200); });
        }
      },
      status: function (id) { return api().AppletInvocationAt(ws,id); },
      cancel: function (id) { return api().CancelAppletInvocationAt(ws,id).then(function () { change(); }); },
      artifact: function (id) { return api().AppletArtifactAt(ws,id); },
      saveArtifact: function (id) { return api().SaveAppletArtifactAt(ws,id); },
      invocations: function () { return api().AppletInvocationsAt(ws).then(function (list) { return (list || []).filter(function (v) { return v.action.startsWith(provider+'/'); }); }); },
      workspace: ws
    });
  }
  var STATUS = {
    running: { text: 'Running', kind: 'accent' }, waiting: { text: 'Waiting', kind: 'warn', label: 'Waiting for someone in Notifications' },
    done: { text: 'Done', kind: 'positive' }, failed: { text: 'Failed', kind: 'danger' }, cancelled: { text: 'Stopped' }
  };
  function artifactButton(artifact, ws) { return button('Open '+artifact.name,function () { window.dinahWorkbench.open({kind:'applet-artifact',path:artifact.id,workspace:ws,title:artifact.name}).catch(error); }); }
  function renderInvocations(host, ws, filter) {
    var alive = true, generation = 0;
    async function draw() {
      var serial = ++generation, list = await api().AppletInvocationsAt(ws);
      if (!alive || serial !== generation) return;
      host.replaceChildren();
      (list || []).filter(filter || function () { return true; }).slice(0,20).forEach(function (v) {
        // A row per invocation, its status as a badge; opening it opens the
        // first thing it made, or Notifications while it waits for someone.
        var output = null;
        var row = window.dinahBrowserList.row({
          title: v.action, meta: [v.error || ''], badge: STATUS[v.status] || { text: v.status },
          onOpen: function () {
            if (v.artifacts && v.artifacts.length) return window.dinahWorkbench.open({kind:'applet-artifact',path:v.artifacts[0].id,workspace:ws,title:v.artifacts[0].name}).catch(error);
            if (v.status === 'waiting') return window.dinahWorkbench.open('needs-you').catch(error);
            if (output) output.hidden = !output.hidden;
          }
        });
        row.classList.add('applet-result');
        var more = node('div','browser-row-actions');
        (v.artifacts || []).forEach(function (artifact) { more.appendChild(artifactButton(artifact,ws)); });
        if (more.children.length) row.appendChild(more);
        if (v.status === 'done' && (!v.artifacts || !v.artifacts.length)) { output = node('pre','applet-output',v.outputJSON); output.hidden = true; row.appendChild(output); }
        host.appendChild(row);
      });
      if (!host.children.length) host.appendChild(window.dinahBrowserList.empty({ title: 'No Results Yet', body: 'What an applet does for a chat or for you appears here.' }));
    }
    var refresh = function () { draw().catch(error); };
    listeners.add(refresh); refresh();
    return function () { alive = false; listeners.delete(refresh); };
  }
  function renderWorkspace() {
    mounts.forEach(function (destroy) { destroy(); }); mounts = [];
    var host = document.getElementById('workspace-applets'); if (!host) return;
    host.replaceChildren(); if (!workspace || !api().AppletInvocationsAt) return;
    // Each contribution is a group like the application's own. Its actions,
    // if it gives any, are at the end of the group's heading: one as a
    // button, several in an Actions menu.
    contributions.forEach(function (c) {
      var g = section('applet-' + c.id, c.title, false);
      var actions = (c.actions || []).map(function (a) { return { label: a.label, title: a.title, run: function () { Promise.resolve().then(function () { return a.run(workspace); }).catch(error); } }; });
      if (actions.length === 1) { var b = node('button','btn btn-sm browser-group-action',actions[0].label); b.type = 'button'; if (actions[0].title) b.title = actions[0].title; b.addEventListener('click',actions[0].run); g.head.appendChild(b); }
      else if (actions.length > 1) { var m = window.dinahBrowserList.actionsMenu(actions); m.classList.add('browser-group-action'); g.head.appendChild(m); }
      g.body.classList.add('applet-workspace'); host.appendChild(g.el);
      var destroy = c.mount(g.body,scoped(c.provider,workspace)); if (destroy) mounts.push(destroy);
    });
    // Applet results are entries under Notifications, and each applet's own
    // window lists its invocations, so the pane has no results group.
  }
  function section(id, title, closedByDefault) {
    var sections = window.dinahBrowserSections;
    return window.dinahBrowserList.group({ id: id, title: title, open: sections ? sections.open(id, closedByDefault) : !closedByDefault, onToggle: function (open) { if (sections) sections.save(id, open); } });
  }
  /* A request or result is an entry in the host's notifications, announced
     by a toast when it first appears. */
  function addEntry(notices, entry, said, kind) {
    var fresh = !notices.get(entry.id);
    notices.add(entry);
    var host = window.dinahHost;
    if (fresh && host && host.toast) host.toast(said, kind, { noted: true });
  }
  /* A script that calls applets in a loop sends an event for each call.
     Each event used to start a load and throw away the one before it, so
     while the calls kept coming no load ever drew, and a posted
     notification showed only once they stopped. Now the events of a
     moment make one load, every load that finishes draws, and a load never
     draws over one started after it. A load is one call, AppletAttention,
     which gives the questions and the results together. */
  var loadSerial = 0, drawnSerial = 0, gather = 0, waiting = [];
  function refreshRequests() {
    return new Promise(function (resolve, reject) {
      waiting.push({ resolve: resolve, reject: reject });
      if (gather) return;
      gather = setTimeout(function () {
        gather = 0;
        var these = waiting; waiting = [];
        loadRequests(++loadSerial).then(function () { these.forEach(function (w) { w.resolve(); }); }, function (err) { these.forEach(function (w) { w.reject(err); }); });
      }, 100);
    });
  }
  async function loadRequests(serial) {
    var notices = window.dinahHost && window.dinahHost.notices; if (!notices || !(api().AppletAttention || api().AppletRequests)) return;
    var values = api().AppletAttention
      ? await api().AppletAttention().then(function (both) { return [both.requests, both.results]; })
      : await Promise.all([api().AppletRequests(), api().AppletResults ? api().AppletResults() : Promise.resolve([])]);
    var list = values[0], results = values[1], ids = new Set();
    if (serial < drawnSerial) return;
    drawnSerial = serial;
    clearTimeout(requestTimer);requestTimer = 0;
    if ((list || []).length) {
      var next = Math.min.apply(null,list.map(function(v){return new Date(v.deadline).getTime()-Date.now()+100;}));
      requestTimer = setTimeout(function(){refreshRequests().catch(error);},Number.isFinite(next) ? Math.max(100,Math.min(30000,next)) : 30000);
    }
    (list || []).forEach(function (v) {
      var id = 'applet-question-'+v.id; ids.add(id);
      var old = notices.get(id); if (old && old.data.version === String(v.version)) return;
      var chat = v.ownerKind === 'chat' ? v.ownerId : '';
      addEntry(notices, { id: id, type: 'question', description: v.request.title, source: chat ? window.dinahHost.chatTitle(chat) : '',
        target: chat ? { chat: chat } : null, announced: true, className: 'question applet-question', data: { version: String(v.version), session: chat || '*' },
        render: function (card) { card.id = id; questionCard(card, v); } }, 'An applet asks: ' + v.request.title, 'warn');
    });
    notices.list(function (e) { return e.id.indexOf('applet-question-') === 0 && !ids.has(e.id); }).forEach(function (e) { notices.remove(e.id); });
    var resultIDs = new Set();
    (results || []).forEach(function (v) {
      var id = 'applet-result-'+v.id; resultIDs.add(id);
      var old = notices.get(id); if (old && old.data.version === String(v.version)) return;
      var titles = {done:'Applet Result',failed:'Applet Action Failed',cancelled:'Applet Action Cancelled',timeout:'Applet Action Timed Out'};
      var notification;
      if (v.status === 'done' && v.action === 'dinah.notifications/post@1') {
        try { notification = JSON.parse(v.outputJSON); } catch (_) {}
      }
      var type = v.status === 'failed' ? 'error' : ['cancelled', 'timeout'].includes(v.status) ? 'warning' : notification ? 'info' : v.status === 'done' ? 'success' : 'info';
      var heading = notification ? notification.title : titles[v.status] || 'Applet Result';
      // A posted notification's toast says what it says; any other result's
      // says which action it came from.
      var said = notification ? (notification.body && notification.body !== notification.title ? notification.title + ' ' + notification.body : notification.title) : heading + ': ' + v.action;
      addEntry(notices, { id: id, type: type, description: notification ? said : heading + ': ' + v.action + (v.error ? ' ' + v.error : ''),
        announced: true, className: 'question applet-result-notice', data: { version: String(v.version), session: '*', appletWorkspace: v.workspace || '', appletId: v.id },
        // Dismiss and Clear All dismiss the result in Go, where its state is kept.
        dismissible: true, dismiss: function () { return api().DismissAppletResultAt(v.workspace, v.id).then(function () { change(); }); },
        render: function (card) { card.id = id; resultCard(card, v, heading, notification); } }, said, { error: 'danger', warning: 'warn', success: 'positive' }[type] || 'info');
    });
    notices.list(function (e) { return e.id.indexOf('applet-result-') === 0 && !resultIDs.has(e.id); }).forEach(function (e) { notices.remove(e.id); });
  }
  /* A request's full card: what it asks, from where, and its form. */
  function questionCard(card, v) {
      var content = node('div','notice-body vstack gap-2');card.appendChild(content);
      content.appendChild(node('strong','',v.request.title));
      content.appendChild(node('p','form-help',v.action+' · '+(v.workspace || 'Personal')));
      content.appendChild(node('p','form-help',(v.ownerKind === 'chat' ? 'Chat ' : 'Applet ')+v.ownerId+' · Expires '+dinahRegional.format(v.deadline,'dateTime')));
      content.appendChild(node('p','',v.request.body));
      var form = node('form','vstack gap-2'), controls = [];
      (v.request.fields || []).forEach(function (field) {
        var label = node('label','form-label',field.label), control;
        if (field.type === 'boolean') { control = node('input'); control.type = 'checkbox'; }
        else if (field.type === 'choice') { control = node('select','form-select'); control.add(new Option('Choose…','')); (field.choices || []).forEach(function (choice) { control.add(new Option(choice,choice)); }); control.required = !!field.required; }
        else { control = node('textarea','form-textarea'); control.rows = 2; control.required = !!field.required; }
        control.name = field.name; label.appendChild(control); form.appendChild(label); controls.push([field,control]);
      });
      var actions = node('div','notice-actions hstack gap-2'), submit = node('button','btn btn-primary btn-sm','Complete Request'); submit.type = 'submit'; actions.appendChild(submit);
      actions.appendChild(button('Cancel Request',function () { api().CancelAppletInvocationAt(v.workspace,v.id).then(function () { change(); }).catch(error); }));
      form.appendChild(actions); form.addEventListener('submit',function (e) {
        e.preventDefault(); if(submit.disabled)return;var response = Object.create(null); controls.forEach(function (pair) { response[pair[0].name] = pair[0].type === 'boolean' ? pair[1].checked : pair[1].value; });
        submit.disabled = true;
        api().AnswerAppletRequest(v.workspace,v.id,v.version,JSON.stringify(response)).then(change).catch(function (err) { submit.disabled = false; error(err); });
      }); content.appendChild(form);
  }
  /* A result's full card: its heading, where it came from, its output,
     and its artifacts. The model adds Dismiss. */
  function resultCard(card, v, heading, notification) {
      var content = node('div','notice-body vstack gap-2'); card.appendChild(content);
      content.appendChild(node('strong','',heading));
      if (notification) content.appendChild(node('p','',notification.body));
      content.appendChild(node('p','form-help',v.action+' · '+(v.workspace || 'Personal')));
      content.appendChild(node('p','form-help',(v.ownerKind === 'chat' ? 'Chat ' : 'Applet ')+v.ownerId));
      if (v.outputJSON) {
        var details = node('details'); details.appendChild(node('summary','','Result')); details.appendChild(node('pre','applet-output',v.outputJSON)); content.appendChild(details);
      }
      if (v.error) content.appendChild(node('p','',v.error));
      var actions = node('div','notice-actions hstack gap-2');
      (v.artifacts || []).forEach(function (artifact) { actions.appendChild(artifactButton(artifact,v.workspace)); });
      content.appendChild(actions);
  }
  window.dinahAppletSDK = Object.freeze({
    apiVersion: 1, forApplet: scoped,
    catalog: function () { return api().AppletCatalog(); },
    watch: function (fn) { listeners.add(fn); return function () { listeners.delete(fn); }; },
    registerWorkspaceContribution: function (contribution) { if (contributions.has(contribution.id)) throw new Error('Duplicate workspace contribution.'); contributions.set(contribution.id,contribution); renderWorkspace(); },
    setWorkspace: function (ws) { workspace = ws || ''; renderWorkspace(); refreshRequests().catch(error); },
    renderInvocations: renderInvocations,
    refreshRequests: refreshRequests,
    showArtifact: function (artifact,ws) { return window.dinahWorkbench.open({kind:'applet-artifact',path:artifact.id,workspace:ws,title:artifact.name}); }
  });
  window.dinahWorkbench.register('applet-artifact',{ applet: { id: 'dinah.artifacts', title: 'Applet Artifacts', glyph: 'file', workspace: true },title:'Applet Artifact',glyph:'file',init:function(root,opts,item) {
    var alive = true, h = node('div','applet-artifact vstack gap-3'); root.appendChild(h);
    var artifact;
    api().AppletArtifactAt(item.workspace,item.path).then(function (v) {
      if (!alive) return; artifact = v; h.appendChild(node('h2','',v.name));
      var bytes = Uint8Array.from(atob(v.dataBase64),function(c){return c.charCodeAt(0);});
      if (v.mime === 'image/svg+xml') { var img = node('img'); img.alt = v.name; img.src = 'data:image/svg+xml;base64,'+v.dataBase64; h.appendChild(img); h.appendChild(button('Save SVG…',function(){api().SaveAppletArtifactAt(item.workspace,item.path).catch(error);})); }
      else h.appendChild(node('pre','applet-output',new TextDecoder().decode(bytes)));
      h.appendChild(node('p','form-help',v.size+' bytes · SHA-256 '+v.sha256));
    }).catch(error);
    return {commands:function(){return artifact && artifact.mime === 'image/svg+xml' ? [{label:'Save SVG…',run:function(){api().SaveAppletArtifactAt(item.workspace,item.path).catch(error);}}] : [];},destroy:function(){alive=false;}};
  }});
  if (window.runtime) {
    window.dinahHost.on('applet:invocation',change);
    window.dinahHost.on('applet:error',function(v){error(v.error);});
  }
  document.addEventListener('DOMContentLoaded',function(){refreshRequests().catch(error);});
})();
