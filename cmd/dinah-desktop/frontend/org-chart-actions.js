/* Person-facing forms use only the actions and choices supplied by OrgChartActions. */
(function () {
  'use strict';
  var node = window.dinahHelpers.node, app = window.dinahHelpers.app;
  var draftKeys = ['action', 'chat', 'group', 'lead', 'member', 'title', 'workspaceId', 'provider'];
  function create(root, host) {
    var state = new URLSearchParams(), view = null, action = null, key = '', sequence = 0, disposed = false, busy = false;
    var form = null, inputs = {}, memberInputs = [], outcome = null, submit = null, report = null, completed = new Map();
    var openerId = '', openerControl = 'box';
    root.classList.add('org-action');
    root.addEventListener('cancel', function (event) { event.preventDefault(); back(); });
    function button(text, run) { var b = node('button', 'btn btn-sm', text); b.type = 'button'; b.addEventListener('click', function () { Promise.resolve().then(run).catch(window.dinahHost.fail); }); return b; }
    function navigate(p, push) { state = new URLSearchParams(p); host.navigate(state, push); }
    function href(value) {
      var url = new URL(value, location.href), p = new URLSearchParams(host.state());
      if (url.origin !== location.origin) throw new Error('The chart action link is not local.');
      var incoming = new URLSearchParams(url.searchParams.get('view.org-chart') || '');
      if (!incoming.get('action')) {
        var opened = new Set((new URL(location.href).searchParams.get('open') || '').split(',').filter(Boolean));
        (url.searchParams.get('open') || '').split(',').filter(Boolean).forEach(function (id) { opened.add(id); });
        var target = new URL(location.href); target.searchParams.set('open', Array.from(opened).join(',')); location.assign(target.href); return;
      }
      if (!state.get('action')) completed.forEach(function (result, id) { if (result.status === 'succeeded') completed.delete(id); });
      draftKeys.forEach(function (name) { p.delete(name); });
      draftKeys.forEach(function (name) { incoming.getAll(name).forEach(function (value) { p.append(name, value); }); });
      navigate(p, true);
    }
    function back() { var p = new URLSearchParams(host.state()); draftKeys.forEach(function (name) { p.delete(name); }); navigate(p, true); }
    function menuRows(value) {
      var rows = value.chatId ? [{ label: 'Open Chat', run: function () { return host.openChat(value.chatId); } }, '-'] : [];
      return rows.concat((value.actions || []).map(function (a) { return { id: a.kind, label: a.label, run: function () { href(a.href); } }; }), (value.unavailable || []).map(function (a) { return { id: a.kind, label: { assign_team: 'Assign to a Team…', replace_team_lead: 'Make Team Lead…', make_lead: 'Make Lead…', create_team: 'Create Team…', create_group: 'Create Group…', start_chat: 'Start a Chat…', delete_chat: 'Delete Chat…' }[a.kind] || 'Unavailable', disabled: true, reason: a.reason }; }));
    }
    function menu(chatId, mark, detail) {
      openerId = chatId;
      openerControl = mark.matches('.org-chart-actions-button') || mark.contains(document.activeElement) && document.activeElement.matches('.org-chart-actions-button') ? 'actions' : 'box';
      var request = ++sequence;
      detail.add([{ id: 'loading', label: 'Reading available actions…', disabled: true }]); detail.mark(mark);
      Promise.resolve().then(function () { return app().OrgChartActions(chatId); }).then(function (value) {
        if (!disposed && request === sequence) window.pudlMenu.updateContext(menuRows(value), mark);
      }).catch(function () { if (!disposed && request === sequence) window.pudlMenu.updateContext([{ label: 'Actions could not be read. Try again.', disabled: true }], mark); });
    }
    function save() {
      var p = new URLSearchParams(state); ['group', 'lead', 'member', 'title', 'workspaceId', 'provider'].forEach(function (name) { p.delete(name); });
      Object.keys(inputs).forEach(function (name) { if (inputs[name].value) p.set(name, inputs[name].value); });
      memberInputs.forEach(function (entry) { if (entry.input.checked) p.append('member', entry.id); });
      state = p; host.changed(p); validate();
    }
    function label(text, input) { var wrap = node('label', 'form-label', text); input.setAttribute('aria-label', text); wrap.append(input); form.append(wrap); return input; }
    function choices(name, text, rows, initial) {
      var input = node('select', 'form-select'), empty = node('option', '', 'Choose…'); empty.value = ''; input.append(empty);
      rows.forEach(function (row) { var o = node('option', '', row.title); o.value = row.id; input.append(o); });
      input.value = state.get(name) || initial || ''; input.required = true; input.addEventListener('change', save); inputs[name] = label(text, input); return input;
    }
    function validate() {
      if (!submit) return;
      var members = memberInputs.filter(function (entry) { return entry.input.checked && (!inputs.lead || entry.id !== inputs.lead.value); });
      var missing = Object.keys(inputs).some(function (name) { return !inputs[name].value.trim(); });
      submit.disabled = busy || !!outcome || missing || (['create_team', 'create_group'].includes(action.kind) && !members.length);
      Array.from(form.elements).forEach(function (input) { if (input !== submit) input.disabled = !input.hasAttribute('data-org-cancel') && (busy || !!outcome); });
    }
    function render(value) {
      view = value; action = value.actions.find(function (a) { return a.kind === state.get('action'); });
      root.replaceChildren(); root.setAttribute('aria-label', action ? action.label : 'Action unavailable');
      var cancel = button('Cancel', back);
      cancel.setAttribute('data-org-cancel', '');
      if (!action) { root.append(node('h2', 'dialog-title', 'This action is unavailable'), node('p', '', (value.unavailable.find(function (a) { return a.kind === state.get('action'); }) || {}).reason || 'The chat or its available actions changed. Return to the chart and try again.'), cancel); return; }
      outcome = null; inputs = {}; memberInputs = [];
      root.append(node('h2', 'dialog-title', action.label), node('p', 'org-action-subject', value.title || 'Org Chart'), node('p', 'dialog-body', action.explanation));
      form = node('form', 'org-action-form org-dialog-fields'); root.append(form);
      if (['create_team', 'create_group'].includes(action.kind)) { var title = node('input', 'form-input'); title.type = 'text'; title.required = true; title.value = state.get('title') || ''; title.addEventListener('input', save); inputs.title = label('Name', title); }
      if (action.kind === 'assign_team') choices('lead', 'Team Lead', action.chats || []);
      if (action.kind === 'make_lead') choices('group', 'Ordinary Group', action.groups || []);
      if (action.kind === 'replace_team_lead') choices('group', 'Team', action.groups || [], action.groups && action.groups.length === 1 ? action.groups[0].id : '');
      if (action.kind === 'create_team') choices('lead', 'Lead', action.leads || [], value.chatId);
      if (action.kind === 'start_chat') { choices('workspaceId', 'Workspace', action.workspaces || []); choices('provider', 'Agent Program', action.providers || []); }
      if (['create_team', 'create_group'].includes(action.kind)) {
        var fieldset = node('fieldset', 'org-action-members org-dialog-members'); fieldset.append(node('legend', '', 'Members')); form.append(fieldset);
        (action.chats || []).forEach(function (chat) { var wrap = node('label'), input = node('input'); input.type = 'checkbox'; input.checked = state.getAll('member').includes(chat.id) || action.kind === 'create_group' && !state.has('member') && chat.id === value.chatId; input.addEventListener('change', save); wrap.append(input, document.createTextNode(chat.title)); fieldset.append(wrap); memberInputs.push({ id: chat.id, input: input }); });
      }
      if (action.confirmation) form.append(node('p', 'org-action-confirmation', action.confirmation));
      submit = node('button', 'btn btn-primary', action.kind === 'delete_chat' ? 'Delete Chat' : action.label.replace(/…$/, '')); submit.type = 'submit';
      var controls = node('div', 'dialog-actions'); controls.append(cancel, submit); form.append(controls);
      report = node('section', 'org-action-result'); report.hidden = true; report.setAttribute('role', 'status'); report.setAttribute('aria-label', 'Action result'); root.append(report);
      form.addEventListener('submit', function (event) { event.preventDefault(); if (!submit.disabled) run(); });
      var prior = completed.get(fingerprint(values())); if (prior) showResult(prior); validate();
      var first = root.querySelector('input:not(:disabled), select:not(:disabled), button:not(:disabled)'); if (first) first.focus({ preventScroll: true });
    }
    function values() {
      var request = { kind: action.kind, token: action.token, chatId: view.chatId };
      var fields = { group: 'groupId', lead: 'leadId', title: 'title', workspaceId: 'workspaceId', provider: 'provider' };
      Object.keys(inputs).forEach(function (name) { request[fields[name]] = inputs[name].value; });
      if (memberInputs.length) request.memberIds = memberInputs.filter(function (entry) { return entry.input.checked && (!inputs.lead || entry.id !== inputs.lead.value); }).map(function (entry) { return entry.id; });
      return request;
    }
    function fingerprint(request) { return JSON.stringify([request.kind, request.chatId, request.groupId || '', request.leadId || '', request.title || '', request.workspaceId || '', request.provider || '', (request.memberIds || []).slice().sort()]); }
    function showResult(result) {
      outcome = result; report.hidden = false; report.replaceChildren(node('p', '', result.message || 'The change did not finish. Review the chart before trying again.')); report.dataset.status = result.status;
      if (result.groupId && action.kind === 'create_team' && result.status === 'partial') report.append(node('p', '', 'The group is retained. Use the links below to inspect it or continue.'));
      var names = new Map((action.chats || []).concat(action.leads || []).map(function (chat) { return [chat.id, chat.title]; }));
      var members = node('ul', 'org-action-outcomes'); (result.members || []).forEach(function (member) { members.append(node('li', '', (names.get(member.chatId) || 'Chat') + ': ' + (member.status === 'assigned' ? 'Assigned.' : member.reason || 'Left out.'))); }); if (members.childNodes.length) report.append(members);
      (result.recovery || []).forEach(function (target) { var a = node('a', 'btn btn-sm', target.label); a.href = target.href; a.addEventListener('click', function (event) { event.preventDefault(); href(target.href); }); report.append(a); });
    }
    function run() {
      var request = values(), submitted = fingerprint(request), submittedKey = key, submittedSequence = sequence, subject = inputs.title && inputs.title.value || view.title || 'Untitled chat', kind = action.kind;
      busy = true; validate(); report.hidden = false; report.replaceChildren(node('p', '', 'Applying the change.'));
      return Promise.resolve().then(function () { return app().ApplyOrgChartAction(request); }).then(function (result) {
        if (disposed) return;
        completed.set(submitted, result); if (sequence === submittedSequence && form && form.isConnected && fingerprint(values()) === submitted) showResult(result);
        var opening = result.created ? window.dinahChats.created(result.created).catch(function () {
          result = Object.assign({}, result, { status: 'partial', message: 'The chat was created, but its window could not be opened. Find it in Chats.' });
          completed.set(submitted, result); if (key === submittedKey && sequence === submittedSequence) showResult(result);
        }) : Promise.resolve();
        if (result.created) subject = result.created.title || 'Untitled chat';
        return opening.then(function () { return host.refresh(); }).then(function () {
          if (!disposed && result.status === 'succeeded') { if (key === submittedKey && sequence === submittedSequence) back(); host.success(result, subject, kind); }
        });
      }).catch(function () {
        if (disposed) return;
        var unknown = { status: 'warning', message: 'The result could not be read. Check the chart and groups before starting another change. This request will not be repeated automatically.' };
        completed.set(submitted, unknown); if (sequence === submittedSequence && form && form.isConnected && fingerprint(values()) === submitted) showResult(unknown); host.refresh();
      }).finally(function () { busy = false; if (!disposed) validate(); });
    }
    function read(p) {
      state = new URLSearchParams(p); var next = state.get('action') ? [state.get('action'), state.get('chat') || '', state.get('group') || ''].join('\0') : '';
      if (!next) { if (root.open) root.close(); if (openerId) host.returnFocus(openerId, openerControl); }
      else if (!root.open) { openerId = openerId || state.get('chat') || ''; root.showModal(); }
      if (next === key) return; key = next; ++sequence;
      if (!next) return;
      root.replaceChildren(node('p', '', 'Reading available actions.')); var request = sequence;
      Promise.resolve().then(function () { return app().OrgChartActions(state.get('chat') || ''); }).then(function (value) { if (!disposed && request === sequence) render(value); }).catch(function () { if (!disposed && request === sequence) { root.replaceChildren(button('Back to Chart', back), node('p', '', 'The available actions could not be read. Return to the chart and try again.')); } });
    }
    return { read: read, menu: menu, href: href, write: function (p) { draftKeys.forEach(function (name) { state.getAll(name).forEach(function (value) { p.append(name, value); }); }); }, destroy: function () { disposed = true; ++sequence; if (root.open) root.close(); } };
  }
  window.dinahOrgChartManagement = { create: create };
}());
