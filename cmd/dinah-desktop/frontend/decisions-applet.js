/* The Decisions view consumes docs/design/decisions.md. The engine owns delivery. */
(function () {
  'use strict';
  var page = null;
  var node = window.dinahHelpers.node;
  function app() { return window.dinahHelpers.app(); }
  function available() { return !!(app() && app().Decisions); }
  function errorText(error) { return window.dinahHelpers.message(error, 'sentence'); }
  function operationId() { return Array.from(crypto.getRandomValues(new Uint32Array(4))).map(function (n) { return n.toString(16).padStart(8, '0'); }).join(''); }
  function button(text, action, primary) {
    var el = node('button', 'btn btn-sm' + (primary ? ' btn-primary' : ''), text); el.type = 'button';
    el.addEventListener('click', function () { Promise.resolve().then(action).catch(window.dinahHost.fail); }); return el;
  }
  function signature(item) {
    if (item.kind === 'task') return JSON.stringify(item.taskQuestion);
    return JSON.stringify([item.id, item.destination, item.askerSession, item.group, item.messageId, item.text, item.canDismiss, item.dismissalReason,
      item.questions.map(function (q) { return [q.number, q.text, q.recommendation, q.choices]; })]);
  }
  function validWireItem(item) {
    return item && ['text', 'workspace', 'askerSession', 'group', 'groupTitle', 'createdAt'].every(function (key) { return typeof item[key] === 'string'; }) && Number.isFinite(Date.parse(item.createdAt)) && Number.isSafeInteger(item.messageId) && item.messageId > 0 && typeof item.id === 'string' && item.id && Number.isSafeInteger(item.revision) && item.revision > 0 &&
      (item.kind === 'request' || item.kind === 'mention') && typeof item.canDismiss === 'boolean' && typeof item.dismissalReason === 'string' && typeof item.destination === 'string' && item.destination && typeof item.asker === 'string' &&
      Array.isArray(item.questions) && (item.kind === 'mention' ? item.questions.length === 0 : item.questions.length > 0 && item.questions.length <= 20) && new Set(item.questions.map(function (q) { return q.number; })).size === item.questions.length && item.questions.every(function (q) { return Number.isSafeInteger(q.number) && q.number > 0 && typeof q.text === 'string' && typeof q.recommendation === 'string' && Array.isArray(q.choices) && q.choices.every(function (c) { return typeof c === 'string'; }) && typeof q.answered === 'boolean' && typeof q.answer === 'string'; });
  }
  function validItem(item) { return validWireItem(item) && item.kind !== 'mention'; }

  function validReceipt(receipt, op) {
    return receipt && receipt.operationId === op.input.operationId && (receipt.state === 'committed' || receipt.state === 'rejected') && receipt.id === (op.input.id || '') &&
      Number.isSafeInteger(receipt.revision) && receipt.revision >= 0 && typeof receipt.closed === 'boolean' && Array.isArray(receipt.messageIds) && receipt.messageIds.every(Number.isSafeInteger) && (receipt.state === 'committed' ? receipt.reason === null : receipt.reason && typeof receipt.reason.code === 'string' && typeof receipt.reason.message === 'string' && !!receipt.reason.message && receipt.messageIds.length === 0);
  }
  function init(root, opts) {
    var disposed = false, ready = false, readNumber = 0, refreshing = false, refreshAgain = false, moreBefore = 0, loadedPages = 1, readPromise = null, readValid = false;
    var items = new Map(), order = [], selected = '', selectedCopy = null, filter = '', view = 'requests';
    var pane = 'request', lastFinished = null, quoteWatch = null;
    var rendered = '', draft = null, draftKey = '', saving = 0, draftError = '', pending = new Set();
    root.classList.add('decisions');
    // PUDL temporarily makes automatic docks rails while this window needs room.
    root.closest('[data-win]').setAttribute('data-win-room', '84rem');
    var layout = node('div', 'md-layout'), body = node('div', 'md-body decisions-body');
    var sidebar = node('aside', 'md-sidebar'), toolbar = node('div', 'md-sidebar-head decisions-toolbar');
        var search = node('input', 'form-input md-filter'); search.type = 'search'; search.placeholder = 'Filter Requests'; search.setAttribute('aria-label', 'Filter Requests');
    var count = node('span', 'badge', '0'), label = node('p', 'md-section-label', 'Open Requests '); label.appendChild(count);
    var refresh = button('Refresh', function () { if (view === 'record') { rendered = ''; draw(); } return load(false); }); refresh.hidden = true;
    var more = button('Show More Requests', function () { return load(true); }); more.disabled = true;
    toolbar.append(search, label);
    var status = node('p', 'decisions-status text-muted', 'Loading requests.'); status.setAttribute('role', 'status'); status.setAttribute('aria-live', 'polite');
    var listNotice = node('div', 'notice danger decisions-list-notice'); listNotice.hidden = true; listNotice.setAttribute('role', 'alert');
    var listNoticeContent = node('div', 'notice-content'), listNoticeText = node('p', 'notice-body');
    var retryRead = button('Refresh', function () { return load(false); }), retryActions = node('div', 'notice-actions');
    retryActions.appendChild(retryRead); listNoticeContent.append(node('p', 'notice-title', 'Requests could not be read'), listNoticeText, retryActions); listNotice.appendChild(listNoticeContent);
    var list = node('ul', 'decisions-list decisions-rows md-list'); list.setAttribute('aria-label', 'Open Requests');
    var listEnd = node('p', 'decisions-list-end', 'That is every open request.');
    var detail = node('section', 'decisions-detail md-detail'); detail.setAttribute('aria-label', 'Request Details');
    var empty = node('section', 'decisions-empty');
    sidebar.append(toolbar, listNotice, list, more, listEnd); body.append(sidebar, detail); layout.appendChild(body); root.append(status, layout, empty);
    var form = null, send = null, receiptLine = null, changedLine = null, review = null, destination = null;
    function address() { var p = new URLSearchParams(); if (selected) p.set('request', selected); if (filter) p.set('q', filter); if (view !== 'requests') p.set('view', view); if (pane === 'about') p.set('pane', 'about'); return p.toString(); }
    function navigate(nextView, id) { view = nextView; pane = 'request'; selected = id || ''; rendered = ''; opts.changed(address()); draw(); }
    function say(text) { if (disposed) return; status.textContent = text; if (receiptLine && receiptLine.isConnected && text) receiptLine.textContent = text; if (empty && !empty.hidden && empty.querySelector('.empty-state-title')) empty.querySelector('.empty-state-title').textContent = text || 'There are no open requests.'; }
    function current() { return items.get(selected) || (selectedCopy && selectedCopy.id === selected ? selectedCopy : null); }
    function stateRead(value) {
      var p = new URLSearchParams(value || ''); selected = p.get('request') || ''; filter = p.get('q') || '';
      pane = p.get('pane') === 'about' ? 'about' : 'request';
      view = /^(broadcast-active|broadcast-all|record)$/.test(p.get('view')) ? p.get('view') : 'requests'; search.value = filter; rendered = '';
    }
    function draftFor(id) { return window.dinahDecisionDrafts.read(id); }
    function store(id, value) { return window.dinahDecisionDrafts.write(id, value); }
    function persist() {
      var key = draftKey, value = draft; saving++; draftError = ''; controls();
      return store(key, value).then(function () { if (draftKey === key) draftError = ''; }, function (error) {
        if (draftKey === key) draftError = 'Your draft could not be saved. ' + errorText(error);
        throw error;
      }).finally(function () { saving--; controls(); });
    }
    function saveEdited() { say(''); if (draft.operation && !workingOp(draft)) draft.operation = null; persist().catch(function () {}); }
    function workingOp(value) { return value && value.operation && !['committed', 'rejected'].includes(value.operation.status); }
    function controls() {
      if (disposed || !form) return;
      if (view === 'requests' && current() && current().kind === 'task') {
        var key = current().taskQuestion.key, entry = window.dinahTaskQuestions.get(key), why = window.dinahTaskQuestions.validation(key);
        send.disabled = !!why; send.title = why;
        receiptLine.textContent = entry && entry.status || why || 'Your answer is ready to send.';
        return;
      }
      if (!draft) return;
      var item = current(), busy = !!workingOp(draft), broadcast = view !== 'requests';
      var blocked = busy || !ready;
      form.querySelectorAll('textarea,input,select,[data-edit-answer]').forEach(function (el) { el.disabled = blocked || !broadcast && (item && item.kind === 'request' && item.canDismiss || !items.has(selected) || !!(draft.operation && draft.operation.status === 'committed' && draft.operation.receipt && draft.operation.receipt.closed)); });
      if (send && !broadcast) {
        var fields = item && item.kind === 'mention' ? [{ number: 0, answered: false }] : item ? item.questions : [];
        var any = fields.some(function (q) { var a = draft.answers[q.number]; return !q.answered && a && (a.asRecommended || a.text.trim()); });
        send.disabled = blocked || !!draftError || !item || !items.has(selected) || item.kind === 'request' && item.canDismiss || draft.reviewed !== signature(item) || !any;
      }
      var dismissButton = form.querySelector('[data-dismiss-decision]');
      if (dismissButton) { dismissButton.disabled = true; dismissButton.title = window.dinahTaskQuestions.dismissUnavailableReason; }
      if (changedLine && !broadcast) {
        var changed = item && draft.reviewed !== signature(item);
        changedLine.textContent = changed ? 'The request or destination changed. Review it before sending.' : !items.has(selected) ? 'This request is not in the list of open requests. Your draft is saved. Refresh or load earlier requests.' : '';
        review.hidden = !changed; review.disabled = busy;
      }
      if (broadcast) {
        var previewButton = form.querySelector('[data-broadcast="preview"]'), confirmButton = form.querySelector('[data-broadcast="confirm"]');
        if (previewButton) previewButton.disabled = blocked || form.dataset.previewPending === 'true' || !(draft.text || '').trim();
        if (confirmButton) confirmButton.disabled = blocked || !!draftError || !draft.preview || Date.parse(draft.preview.expiresAt) <= Date.now() || draft.preview.text !== draft.text;
      }
      if (receiptLine) {
        var op = draft.operation;
        receiptLine.textContent = deliveryStatus();
        if (item && item.kind === 'request' && item.canDismiss) receiptLine.textContent = item.dismissalReason || 'This answer can no longer reach the asking chat.';
        if (changedLine && changedLine.textContent) receiptLine.textContent = changedLine.textContent;
        if (status.textContent) receiptLine.textContent = status.textContent;
        else if (!broadcast && lastFinished && !draft.operation && send && send.disabled) { var finished = lastFinished; receiptLine.textContent = finished.text; if (!finished.id.startsWith('task:')) receiptLine.appendChild(button('Show Answers', function () { navigate('record', finished.id); })); }
        if (draftError) receiptLine.textContent = draftError;
        receiptLine.classList.toggle('danger', !!draftError || !!op && op.status === 'rejected');
        detail.querySelectorAll('[data-operation]').forEach(function (el) { el.hidden = !workingOp(draft); el.disabled = !workingOp(draft) || pending.has(op.input.operationId); });
      }
    }
    function deliveryStatus() {
      if (draftError) return draftError;
      var op = draft.operation;
      if (!op) return send && send.disabled ? 'Choose or write an answer to send it.' : 'Your answer is ready to send.';
      if (op.status === 'rejected') return op.receipt.reason.message;
      if (op.status !== 'committed') return 'Sending has not been confirmed. Check the result before sending again.';
      if (op.method === 'DismissDecision') return 'Message dismissed. No reply was sent.';
      if (op.method === 'ConfirmDecisionBroadcast') return 'Your message is waiting for the chats in the preview.';
      return 'Your reply was sent. The asking chat may not have read it yet.';
    }
    function taskItems() {
      return window.dinahTaskQuestions.list().map(function (entry) { var q = entry.question; return { id: 'task:' + q.key, kind: 'task', asker: q.serviceName, workspace: q.workspaceName, groupTitle: '', text: q.question, createdAt: q.askedAt, questions: [], taskQuestion: q }; });
    }
    function syncTasks() {
      items.forEach(function (item, id) { if (item.kind === 'task') items.delete(id); });
      taskItems().forEach(function (item) { items.set(item.id, item); });
      order = Array.from(items.keys()).sort(function (a, b) { var x = items.get(a), y = items.get(b); return Date.parse(x.createdAt) - Date.parse(y.createdAt) || (x.messageId || 0) - (y.messageId || 0) || a.localeCompare(b); });
    }
    function openChat(item) {
      return window.dinahChats.refreshInventory().then(function () { if (!window.dinahChats.ref(item.askerSession)) throw new Error('The asking chat is no longer available.'); return window.dinahChats.open(item.askerSession); });
    }
    function rowMenu(item, owner, point) {
      var task = item.kind === 'task', q = item.taskQuestion;
      var rows = [
        { label: 'Open', run: function () { navigate('requests', item.id); } },
        { label: 'Open Chat', disabled: task || !!item.dismissalReason, reason: task ? 'A task question comes from a service, not a chat.' : item.dismissalReason, run: function () { return openChat(item); } },
        { label: 'Open Group', disabled: task || !item.group, reason: task ? 'This task question has no group.' : 'This request was asked directly.', run: function () { return window.dinahBus.openGroup(item.group); } },
        { label: 'Open Task', disabled: !task || !q.target.href, reason: task ? q.target.reason : 'This request comes from a chat, not a task.', run: function () { return window.dinahTaskQuestions.open(q.key); } },
        { label: 'Dismiss ' + (task ? 'Question' : item.kind === 'mention' ? 'Message' : 'Request') + '…', disabled: true, reason: window.dinahTaskQuestions.dismissUnavailableReason }
      ];
      var r = owner.getBoundingClientRect(); window.pudlMenu.openAt(point ? point.x : r.left, point ? point.y : r.bottom, rows, { owner: owner, for: owner });
    }
    function drawList() {
      count.textContent = String(items.size);
      var visible = order.filter(function (id) { var item = items.get(id); return item && (!filter || [item.asker, item.workspace, item.groupTitle, item.text].join(' ').toLowerCase().includes(filter.toLowerCase())); });
      var existing = new Map(Array.from(list.children).map(function (li) { return [li.dataset.requestId, li]; }));
      visible.forEach(function (id) {
        var item = items.get(id), li = existing.get(id);
        if (!li) {
          li = node('li', 'decisions-row md-row'); li.dataset.requestId = id;
          var a = node('a', 'md-item'); a.append(node('strong', 'decisions-row-title'), node('small', 'md-meta'), node('small', 'decisions-row-state md-meta'));
          a.addEventListener('click', function (e) { e.preventDefault(); selectedCopy = items.get(id); navigate('requests', id); });
          var menu = button('⋯', function () { rowMenu(items.get(id), menu); }); menu.dataset.decisionActions = ''; menu.setAttribute('aria-label', 'Actions for this request');
          li.addEventListener('contextmenu', function (e) { e.preventDefault(); rowMenu(items.get(id), menu, { x: e.clientX, y: e.clientY }); });
          li.addEventListener('keydown', function (e) { if (e.key === 'ContextMenu' || e.shiftKey && e.key === 'F10') { e.preventDefault(); rowMenu(items.get(id), menu); } });
          li.append(a, menu);
        }
        var link = li.firstChild, pendingQuestions = item.questions.filter(function (q) { return !q.answered; });
        var summary = item.kind === 'task' ? '◆ Task' : item.dismissalReason ? '▲ Cannot be delivered' : item.kind === 'mention' ? '• Message' : '◆ ' + pendingQuestions.length + (pendingQuestions.length === item.questions.length ? ' questions waiting' : ' of ' + item.questions.length + ' still open');
        link.href = '?' + new URLSearchParams({ open: 'decisions', 'view.decisions': 'request=' + encodeURIComponent(id) }).toString();
        link.setAttribute('aria-current', String(view === 'requests' && selected === id));
        link.children[0].textContent = item.kind === 'request' ? (pendingQuestions[0] || item.questions[0]).text : item.text.split(/\r?\n/)[0];
        link.children[1].textContent = item.asker + (item.groupTitle ? ' in ' + item.groupTitle : '') + ' · ' + item.workspace;
        link.children[2].replaceChildren(node('span', 'badge' + (item.dismissalReason ? ' warn' : ''), summary), node('time', '', window.dinahRegional.format(new Date(item.createdAt), 'dayTime'))); existing.delete(id); list.appendChild(li);
      });
      existing.forEach(function (li) { li.remove(); });
      if (!visible.length && items.size) list.appendChild(node('li', 'decisions-list-end', 'No requests match this filter.'));
      more.hidden = !moreBefore; listEnd.hidden = !!moreBefore;
    }
    function question(q, item) {
      var field = node('fieldset', 'decisions-question'); field.dataset.question = String(q.number);
      field.appendChild(node('legend', 'md-section-label', item.kind === 'mention' ? 'Your Reply' : 'Question ' + q.number + ' of ' + item.questions.length));
      if (item.kind !== 'mention') field.appendChild(node('p', 'decisions-question-text', q.text));
      if (q.answered) { field.appendChild(node('p', 'decisions-answered', '✓ Answered: ' + q.answer)); return field; }
      if (item.kind === 'request') { var recommendation = node('p', 'decisions-recommendation'); recommendation.append(node('strong', '', 'Recommended'), document.createTextNode(q.recommendation)); field.appendChild(recommendation); }
      var a = draft.answers[q.number] || { text: '', asRecommended: false }, options = node('div', 'form-options');
      var input = node('textarea', 'form-input form-textarea'); input.value = typeof a.ownText === 'string' ? a.ownText : a.text; input.maxLength = 8000; input.setAttribute('aria-label', item.kind === 'mention' ? 'Your Reply' : 'Answer ' + q.number);
      var radios = [];
      function choice(text, value) { var label = node('label', 'check'), radio = node('input'); radio.type = 'radio'; radio.name = 'answer-' + q.number; radio.value = value; radio.checked = a.selection ? a.selection === value : value === 'recommended' ? a.asRecommended : !a.asRecommended && (value === 'own' ? !!a.text && !q.choices.includes(a.text) : a.text === q.choices[Number(value.slice(7))]); radio.addEventListener('change', function () { draft.answers[q.number] = { text: value === 'recommended' ? input.value : value === 'own' ? input.value : q.choices[Number(value.slice(7))], asRecommended: value === 'recommended', selection: value, ownText: input.value }; saveEdited(); }); radios.push(radio); label.append(radio, document.createTextNode(text)); options.appendChild(label); return radio; }
      if (item.kind === 'request') { choice('As recommended', 'recommended'); q.choices.forEach(function (text, index) { choice(text, 'choice-' + index); }); var own = choice('My own answer', 'own'); field.appendChild(options); }
      var label = node('label', 'decisions-own'); if (item.kind === 'mention') label.appendChild(node('span', '', 'Your Reply')); label.appendChild(input); field.appendChild(label);
      input.addEventListener('input', function () { if (own) { radios.forEach(function (radio) { radio.checked = radio === own; }); } draft.answers[q.number] = { text: input.value, asRecommended: false, selection: 'own', ownText: input.value }; saveEdited(); });
      return field;
    }
    function operationActions() {
      var actions = node('div', 'decisions-actions'), check = button('Check Send Result', reconcile);
      check.dataset.operation = 'check'; actions.appendChild(check); return actions;
    }
    function deliveryPanel() {
      var panel = node('div', 'decisions-delivery');
      receiptLine = node('p', 'decisions-receipt'); receiptLine.setAttribute('role', 'status');
      panel.append(receiptLine, operationActions()); return panel;
    }
    function requestHead(item) {
      var head = node('header', 'decisions-head decisions-source');
      var back = button('Back to Requests', function () { navigate('requests', ''); }); back.classList.add('md-back');
      head.append(back, node('h2', '', item.kind === 'task' ? item.taskQuestion.taskTitle : item.asker));
      head.appendChild(node('p', 'decisions-head-meta', item.asker + ' · ' + item.workspace + (item.groupTitle ? ' · ' + item.groupTitle : '') + ' · ' + window.dinahRegional.format(new Date(item.createdAt), 'dayTime')));
      var about = button('About This Request', function () { pane = 'about'; opts.changed(address()); draw(); detail.querySelector('.decisions-about-back').focus(); }); about.classList.add('decisions-about-open'); head.appendChild(about);
      return head;
    }
    function aboutPane(item) {
      var about = node('aside', 'decisions-about'); about.setAttribute('aria-label', 'About This Request');
      var back = button('Back to Request', function () { pane = 'request'; opts.changed(address()); draw(); detail.querySelector('.decisions-about-open').focus(); }); back.classList.add('decisions-about-back');
      about.append(back, node('h3', 'md-section-label', 'About This Request'));
      var table = node('table', 'kv-table');
      function fact(name, value) { var tr = node('tr'); tr.append(node('th', '', name), node('td', '', value)); table.appendChild(tr); }
      fact(item.kind === 'task' ? 'Service' : 'From', item.asker);
      if (item.kind === 'task') fact('Task', item.taskQuestion.taskTitle); else if (item.groupTitle) fact('Group', item.groupTitle);
      fact('Workspace', item.workspace); fact('Asked', window.dinahRegional.format(new Date(item.createdAt), 'dayTime'));
      if (item.kind !== 'task') fact('Reply goes to', item.destination);
      about.appendChild(table);
      var opener = button(item.kind === 'task' ? 'Open Task' : 'Open Chat', function () { return item.kind === 'task' ? window.dinahTaskQuestions.open(item.taskQuestion.key) : openChat(item); });
      opener.disabled = item.kind === 'task' ? !item.taskQuestion.target.href : !!item.dismissalReason; opener.title = item.kind === 'task' ? item.taskQuestion.target.reason || '' : item.dismissalReason; about.appendChild(opener);
      if (item.kind !== 'task') about.appendChild(window.dinahDecisionContext(item));
      return about;
    }
    function footer(item) {
      var foot = node('footer', 'decisions-foot'), actions = node('div', 'decisions-foot-actions');
      receiptLine = node('p', 'decisions-receipt decisions-foot-status'); receiptLine.setAttribute('role', 'status'); receiptLine.setAttribute('aria-live', 'polite');
      var dismissButton = button('Dismiss', function () {}); dismissButton.dataset.dismissDecision = ''; dismissButton.disabled = true; dismissButton.title = window.dinahTaskQuestions.dismissUnavailableReason;
      send = button('Send', item.kind === 'task' ? function () { return window.dinahTaskQuestions.answer(item.taskQuestion.key); } : answer, true);
      var unavailable = node('span', 'decisions-dismiss-unavailable'), reason = node('small', 'text-muted', window.dinahTaskQuestions.dismissUnavailableReason); reason.id = 'dismiss-unavailable-' + item.id.replace(/[^a-zA-Z0-9]/g, '-'); dismissButton.setAttribute('aria-describedby', reason.id); unavailable.append(dismissButton); reason.classList.add('decisions-dismiss-reason'); actions.append(unavailable, send, reason); if (item.kind !== 'task') actions.appendChild(operationActions());
      foot.append(receiptLine, actions); return foot;
    }
    function drawRequest() {
      var item = items.get(selected) || selectedCopy;
      if (selected && (!item || item.id !== selected)) { var saved = draftFor('request:' + selected); if (validItem(saved.request) && saved.request.id === selected) item = saved.request; }
      if (!selected || !item || item.id !== selected) { detail.replaceChildren(node('p', 'text-muted', selected ? 'The selected request is not in this list. Your draft is saved.' : 'Choose a request to read and answer.')); form = null; return; }
      selectedCopy = item;
      var shape = selected + '|' + signature(item) + '|' + item.revision;
      if (rendered === shape) { controls(); return; }
      rendered = shape; draftError = ''; if (quoteWatch) { quoteWatch.disconnect(); quoteWatch = null; }
      form = node('form'); form.addEventListener('submit', function (event) { event.preventDefault(); if (!send.disabled) { if (item.kind === 'task') window.dinahTaskQuestions.answer(item.taskQuestion.key); else answer(); } });
      var split = node('div', 'decisions-split'), scroll = node('div', 'decisions-scroll decisions-content'), content = node('div', 'decisions-page decisions-main'); scroll.appendChild(content);
      form.appendChild(requestHead(item));
      if (item.kind === 'task') {
        draft = null; draftKey = ''; content.appendChild(node('p', 'decisions-question-text', item.text));
        var card = node('section', 'decisions-task'); window.dinahTaskQuestions.render(card, item.taskQuestion.key, { embedded: true }); content.appendChild(card);
        var check = button('Check Question Status', function () { return window.dinahTaskQuestions.refresh().catch(function (error) { say(errorText(error)); }); }); content.appendChild(check);
      } else {
        draftKey = 'request:' + item.id; draft = draftFor(draftKey); draft.request = item;
        if (!draft.reviewed && !workingOp(draft)) draft.reviewed = signature(item);
        changedLine = node('p', 'text-warning'); review = button('I Have Reviewed the Updated Request', function () { draft.reviewed = signature(item); saveEdited(); });
        var reviewPanel = node('div', 'decisions-review'); reviewPanel.append(changedLine, review); content.appendChild(reviewPanel);
        if (item.dismissalReason) content.appendChild(node('p', 'notice notice-warn', item.dismissalReason));
        if (item.kind === 'mention') {
          content.appendChild(node('h3', 'md-section-label', 'Message'));
          var quote = node('blockquote', 'decisions-message', item.text); quote.dataset.folded = 'true'; content.appendChild(quote);
          var whole = button('Show the Whole Message', function () { var folded = quote.dataset.folded === 'true'; quote.dataset.folded = String(!folded); whole.setAttribute('aria-expanded', String(folded)); whole.textContent = folded ? 'Fold the Message' : 'Show the Whole Message'; }); whole.hidden = true; whole.setAttribute('aria-expanded', 'false'); content.appendChild(whole);
          quoteWatch = new ResizeObserver(function () { whole.hidden = quote.scrollHeight <= parseFloat(getComputedStyle(quote).lineHeight) * 12 + 1; });
        }
        (item.kind === 'mention' ? [{ number: 0, text: '', choices: [], answered: false }] : item.questions).forEach(function (q) { content.appendChild(question(q, item)); });
      }
      split.append(scroll, aboutPane(item)); form.append(split, footer(item));
      var oldActive = detail.contains(document.activeElement) && document.activeElement.tagName === 'TEXTAREA' ? document.activeElement : null;
      var oldLabel = oldActive && oldActive.getAttribute('aria-label'), oldStart = oldActive && oldActive.selectionStart, oldEnd = oldActive && oldActive.selectionEnd;
      detail.replaceChildren(form); controls(); if (quoteWatch) quoteWatch.observe(quote);
      if (oldLabel) { var replacement = Array.from(form.querySelectorAll('textarea')).find(function (input) { return input.getAttribute('aria-label') === oldLabel; }); if (replacement && !replacement.disabled) { replacement.focus({ preventScroll: true }); replacement.setSelectionRange(oldStart, oldEnd); } }
      if (draft && workingOp(draft)) reconcile();
    }
    function answer() {
      if (!send || send.disabled || workingOp(draft)) return;
      var item = current(), fields = item.kind === 'mention' ? [{ number: 0, answered: false }] : item.questions;
      var answers = fields.filter(function (q) { var a = draft.answers[q.number]; return !q.answered && a && (a.asRecommended || a.text.trim()); }).map(function (q) { var a = draft.answers[q.number]; return { number: q.number, text: a.asRecommended ? '' : a.text, asRecommended: !!a.asRecommended }; });
      return begin('AnswerDecision', { operationId: operationId(), id: item.id, revision: item.revision, answers: answers });
    }
    function dismiss() {
      if (workingOp(draft) || !items.has(selected) || !current().canDismiss) return;
      var item = current();
      return window.dinahHost.ask(item.kind === 'mention' ? 'Dismiss This Message?' : 'Dismiss This Request?', 'This closes the request without sending a reply to ' + item.destination + '.', 'Dismiss').then(function (yes) { if (yes && !disposed && draftKey === 'request:' + item.id) return begin('DismissDecision', { operationId: operationId(), id: item.id, revision: item.revision }); });
    }
    function begin(method, input) {
      var key = draftKey, value = draft;
      draft.operation = { method: method, input: input, status: 'prepared' };
      if (method !== 'ConfirmDecisionBroadcast') draft.operation.destination = current().destination;
      return persist().then(function () { return execute(key, value); }).catch(function (error) { say(errorText(error)); });
    }
    function accept(key, value, receipt) {
      var op = value.operation; if (!validReceipt(receipt, op)) throw new Error('The send result could not be confirmed.');
      var item = items.get(receipt.id) || selectedCopy;
      var destination = op.destination || (item && item.id === receipt.id ? item.destination : 'the asking chat');
      op.status = receipt.state; op.receipt = receipt;
      if (op.method === 'ConfirmDecisionBroadcast') value.preview = null;
      if (receipt.state === 'committed' && op.method === 'AnswerDecision') op.input.answers.forEach(function (answer) { delete value.answers[answer.number]; });
      return store(key, value).then(function () { if (key === draftKey) { draft = value; if (view !== 'requests') rendered = ''; controls(); } if (receipt.state === 'committed' && receipt.closed && selected === receipt.id) { lastFinished = { id: receipt.id, text: op.method === 'DismissDecision' ? 'Request dismissed. No answer was sent.' : 'Your reply was sent to ' + destination + '.' }; selected = ''; selectedCopy = null; rendered = ''; } return load(false); });
    }
    function execute(key, value) {
      var op = value.operation;
      if (pending.has(op.input.operationId)) return Promise.resolve();
      pending.add(op.input.operationId); op.status = 'submitted'; controls();
      return store(key, value).then(function () { return app()[op.method](op.input); }).then(function (receipt) { return accept(key, value, receipt); }).catch(function (error) {
        if (!['committed', 'rejected'].includes(op.status)) { op.status = 'submitted'; return store(key, value).then(function () { say('Sending has not been confirmed. ' + errorText(error)); }); }
        say('The result was confirmed, but could not be saved on this computer. ' + errorText(error));
      }).finally(function () { pending.delete(op.input.operationId); controls(); });
    }
    function reconcile() {
      if (!draft || !workingOp(draft)) return Promise.resolve();
      var key = draftKey, value = draft, op = value.operation;
      if (pending.has(op.input.operationId)) return Promise.resolve();
      pending.add(op.input.operationId); controls();
      return Promise.resolve().then(function () { return app().DecisionOperation(op.input.operationId); }).then(function (receipt) {
        if (receipt && receipt.operationId === op.input.operationId && receipt.state === 'unknown' && receipt.id === '' && receipt.revision === 0 && receipt.closed === false && Array.isArray(receipt.messageIds) && receipt.messageIds.length === 0 && receipt.reason === null) { op.status = 'unknown'; pending.delete(op.input.operationId); return execute(key, value); }
        return accept(key, value, receipt);
      }).catch(function (error) { say('Sending has still not been confirmed. ' + errorText(error)); }).finally(function () { pending.delete(op.input.operationId); if (key === draftKey) controls(); });
    }
    function drawBroadcast() {
      var scope = view === 'broadcast-active' ? 'active' : 'all', key = 'broadcast-' + scope;
      if (rendered === key) { controls(); return; }
      rendered = key; draftKey = key; draft = draftFor(key); draftError = '';
      form = node('form', 'decisions-broadcast'); form.addEventListener('submit', function (e) { e.preventDefault(); });
      var broadcastHead = node('header', 'decisions-head'); broadcastHead.appendChild(node('h2', '', scope === 'active' ? 'Broadcast to Active' : 'Broadcast to All')); form.appendChild(broadcastHead);
      var broadcastFoot = deliveryPanel(); broadcastFoot.classList.add('decisions-foot');
      var content = node('div', 'decisions-content'); form.appendChild(content);
      content.appendChild(node('p', '', scope === 'active' ? 'The preview lists chats that are working when you request it.' : 'The preview lists all available chats. Closed chats wait until there is room to open them in the background. Chats in separate terminal windows cannot be opened here.'));
      var label = node('label', '', 'Message'), input = node('textarea', 'form-input'); input.setAttribute('aria-label', 'Broadcast Message'); input.maxLength = 16000; input.value = draft.text || ''; label.appendChild(input); content.appendChild(label);
      var previewBox = node('div', 'decisions-preview'), previewButton = button('Preview Recipients', preview), confirmButton = button('Confirm Broadcast…', confirmBroadcast, true); confirmButton.disabled = true;
      input.addEventListener('input', function () { draft.text = input.value; draft.preview = null; previewBox.replaceChildren(); confirmButton.disabled = true; saveEdited(); });
      previewButton.dataset.broadcast = 'preview'; confirmButton.dataset.broadcast = 'confirm';
      var actions = node('div', 'decisions-actions'); actions.classList.add('decisions-foot-actions'); actions.append(previewButton, confirmButton); content.appendChild(previewBox); broadcastFoot.appendChild(actions); form.appendChild(broadcastFoot);
      detail.replaceChildren(form);
      function paintPreview() {
        var value = draft.preview; previewBox.replaceChildren(); confirmButton.disabled = true;
        if (!value) return;
        previewBox.appendChild(node('p', '', value.count + (value.count === 1 ? ' chat' : ' chats') + ' will receive this message. Preview expires ' + window.dinahRegional.format(new Date(value.expiresAt), 'dayTime') + '.'));
        var ul = node('ul'); value.targets.forEach(function (target) { ul.appendChild(node('li', '', target.title + ' · ' + target.workspace)); }); previewBox.appendChild(ul);
        confirmButton.disabled = workingOp(draft) || !!draftError || Date.parse(value.expiresAt) <= Date.now();
      }
      function preview() {
        if (workingOp(draft) || !input.value.trim() || form.dataset.previewPending === 'true') return;
        var broadcastForm = form; broadcastForm.dataset.previewPending = 'true';
        var text = input.value; previewButton.disabled = true;
        return Promise.resolve().then(function () { return app().PreviewDecisionBroadcast(scope, text); }).then(function (value) {
          if (draftKey !== key || draft.text !== text) return;
          if (!value || !value.token || value.scope !== scope || value.text !== text || !Number.isFinite(Date.parse(value.expiresAt)) || !Array.isArray(value.targets) || value.count !== value.targets.length || !value.targets.every(function (t) { return t && typeof t.session === 'string' && typeof t.title === 'string' && typeof t.workspace === 'string'; })) throw new Error('The broadcast preview is invalid.');
          draft.preview = value; draft.operation = null; return persist().then(paintPreview);
        }).catch(function (error) { say(errorText(error)); }).finally(function () { delete broadcastForm.dataset.previewPending; controls(); });
      }
      function confirmBroadcast() {
        var value = draft.preview;
        if (confirmButton.disabled || !value || workingOp(draft)) return;
        if (Date.parse(value.expiresAt) <= Date.now()) { say('The preview expired. Preview recipients again.'); paintPreview(); return; }
        return window.dinahHost.ask('Send This Broadcast?', 'Send the displayed message to these ' + value.count + ' chats?\n' + value.targets.map(function (t) { return t.title; }).join('\n'), 'Send Broadcast').then(function (yes) {
          if (!yes || disposed || draftKey !== key || draft.preview !== value) return;
          confirmButton.disabled = true; return begin('ConfirmDecisionBroadcast', { operationId: operationId(), token: value.token });
        });
      }
      paintPreview(); controls(); if (workingOp(draft)) reconcile();
    }
    function draw() {
      if (disposed || !ready) return;
      syncTasks(); if (view !== 'requests' && quoteWatch) { quoteWatch.disconnect(); quoteWatch = null; }
      layout.dataset.mdPane = selected || view !== 'requests' ? 'detail' : 'list';
      root.dataset.decisionPane = pane;
      empty.hidden = items.size > 0 || view !== 'requests'; layout.hidden = !items.size && view === 'requests';
      if (!items.size && view === 'requests') { var explanation = node('div', 'empty-state'); explanation.append(node('p', 'empty-state-title', status.textContent || 'There are no open requests.'), node('p', 'empty-state-body', 'When an agent asks you a question or mentions you in a group, it appears here with its recommendation.')); empty.replaceChildren(explanation); if (lastFinished) { var done = node('p', 'decisions-done', lastFinished.text); if (!lastFinished.id.startsWith('task:')) done.appendChild(button('Show Answers', function () { navigate('record', lastFinished.id); })); empty.appendChild(done); } }
      root.dataset.decisionDetail = String(view !== 'requests' || !!selected);
      drawList();
      try {
        if (view === 'record') {
          form = null;
          var recordId = selected, shape = 'record|' + recordId;
          if (rendered !== shape) {
            rendered = shape;
            window.dinahDecisionRecord(detail, recordId, validItem, function () { return !disposed && view === 'record' && selected === recordId; });
            detail.querySelector('.decisions-source').prepend(button('Back to Requests', function () { navigate('requests', ''); }));
          }
        } else if (view === 'requests') drawRequest(); else drawBroadcast();
      } catch (error) { form = null; say(errorText(error)); detail.replaceChildren(node('p', 'text-muted', 'The saved draft could not be read. It has not been replaced.')); }
    }
    function load(append) {
      if (!ready || disposed) return Promise.resolve();
      if (!available()) { say('Decisions are not available in this build.'); refresh.disabled = true; return Promise.resolve(); }
      if (refreshing) { refreshAgain = true; return readPromise; }
      var before = append ? moreBefore : 0, sequence = ++readNumber;
      refreshing = true; retryRead.disabled = true; readValid = false; refresh.disabled = true; more.disabled = true;
      var collected = [], pages = 0, keepSelection = !append && !!selected && view === 'requests', targetPages = append ? 1 : loadedPages;
      function readPage(cursor) {
        return Promise.resolve().then(function () { return app().Decisions(cursor); }).then(function (page) {
          if (!page || !Array.isArray(page.items) || !page.items.every(validWireItem) || !Number.isSafeInteger(page.nextBefore) || page.nextBefore < 0 || cursor && page.nextBefore >= cursor) throw new Error('The request list is invalid.');
          collected.push.apply(collected, page.items.filter(validItem)); pages++;
          if (page.nextBefore && (pages < targetPages || keepSelection && !collected.some(function (item) { return item.id === selected; }))) return readPage(page.nextBefore);
          return { items: collected, nextBefore: page.nextBefore };
        });
      }
      readPromise = readPage(before).then(function (result) {
        if (disposed || sequence !== readNumber) return;
        if (!result || !Array.isArray(result.items) || !result.items.every(validItem) || new Set(result.items.map(function (item) { return item.id; })).size !== result.items.length || !Number.isSafeInteger(result.nextBefore) || result.nextBefore < 0) throw new Error('The request list is invalid.');
        var previouslyOpen = items.has(selected);
        if (!append) { if (items.has(selected)) selectedCopy = items.get(selected); items.clear(); }
        result.items.forEach(function (item) { items.set(item.id, item); if (!order.includes(item.id)) order.push(item.id); });
        order = order.filter(function (id) { return items.has(id); }); syncTasks(); if (view === 'requests' && previouslyOpen && selected && !items.has(selected) && !workingOp(draft)) { lastFinished = { id: selected, text: 'This request is no longer waiting.' }; selected = ''; selectedCopy = null; rendered = ''; } if (lastFinished && !selected && order.length) { selected = order[0]; opts.changed(address()); } moreBefore = result.nextBefore; loadedPages = append ? loadedPages + pages : pages;
        listNotice.hidden = true; listNoticeText.textContent = ''; say(''); draw(); readValid = true;
      }).catch(function (error) { if (disposed) return; listNoticeText.textContent = errorText(error); listNotice.hidden = false; say('Requests could not be read. ' + errorText(error)); }).finally(function () {
        refreshing = false; if (!disposed) { refresh.disabled = false; retryRead.disabled = false; more.disabled = !moreBefore; }
        if (refreshAgain) { refreshAgain = false; return load(false); }
      });
      return readPromise;
    }
    search.addEventListener('input', function () { filter = search.value; opts.changed(address()); drawList(); });
    stateRead(opts.state);
    var unsubscribe = window.dinahHost.on('bus:changed', function () { load(false); });
    var unTask = window.dinahTaskQuestions.subscribe(function (key) {
      var id = 'task:' + key;
      if (selected === id && !window.dinahTaskQuestions.get(key)) { lastFinished = { id: id, text: 'This task question is no longer waiting.' }; selected = ''; selectedCopy = null; rendered = ''; syncTasks(); if (order.length) selected = order[0]; opts.changed(address()); }
      draw();
    });
    var unresync = window.dinahHost.onResync(function () { return load(false); });
    window.dinahUIState.ready.then(function () {
      if (disposed) return;
      if (window.dinahUIState.initializationError()) { say('Saved drafts could not be read. Sending is disabled.'); return; }
      ready = true; draw(); load(false);
    });
    function ensureRequest(id) {
      return window.dinahUIState.ready.then(function () { return load(false); }).then(function () {
        if (disposed || !ready || !readValid || view !== 'requests' || selected !== id || !items.has(id) || !form || draftKey !== 'request:' + id) throw new Error('This request could not be shown. It may no longer be open.');
        var row = Array.from(list.children).find(function (entry) { return entry.dataset.requestId === id; });
        if (!row) throw new Error('This request could not be shown.');
        row.scrollIntoView({ block: 'nearest' });
        return true;
      });
    }
    return { ensureRequest: ensureRequest, state: address, setState: function (state) { stateRead(state); draw(); }, destroy: function () { disposed = true; if (quoteWatch) quoteWatch.disconnect(); unsubscribe(); unresync(); unTask(); }, commands: function () {
      return [{ label: 'Refresh Requests', disabled: !available(), run: function () { if (view === 'record') { rendered = ''; draw(); } return load(false); } }, { label: 'Broadcast to Active…', run: function () { navigate('broadcast-active'); } }, { label: 'Broadcast to All…', run: function () { navigate('broadcast-all'); } }];
    } };
  }
  window.dinahWorkbench.register('decisions', { applet: { id: 'dinah.decisions', title: 'Decisions', glyph: 'comments', workspace: false }, singleton: true, title: 'Decisions', glyph: 'comments', mode: 'maximized', init: function (root, opts) { var instance = init(root, opts); page = instance; var destroy = instance.destroy; instance.destroy = function () { destroy(); if (page === instance) page = null; }; return instance; },
    launcher: { title: 'Decisions', run: function () { return window.dinahWorkbench.open('decisions'); } } });
  window.dinahDecisions = { openRequest: function (id) { return window.dinahWorkbench.open('decisions', new URLSearchParams({ request: id }).toString()).then(function () { if (!page) throw new Error('Decisions could not be opened.'); return page.ensureRequest(id); }); } };
  function open(state) { return window.dinahWorkbench.open('decisions', state || '').catch(window.dinahHost.fail); }
  window.dinahCommands.register({ id: 'decisions', menu: 'view', label: 'Decisions', visible: available, run: function () { return open(); } });
  ['active', 'all'].forEach(function (scope) { window.dinahCommands.register({ id: 'broadcast-' + scope, menu: 'view', label: scope === 'active' ? 'Broadcast to Active…' : 'Broadcast to All…', visible: available,
    enabled: function () { return !!(app().PreviewDecisionBroadcast && app().ConfirmDecisionBroadcast); }, run: function () { return open('view=broadcast-' + scope); } }); });
}());
