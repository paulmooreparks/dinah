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
    return JSON.stringify([item.id, item.destination, item.askerSession, item.group, item.messageId, item.text, item.canDismiss, item.dismissalReason,
      item.questions.map(function (q) { return [q.number, q.text, q.recommendation, q.choices]; })]);
  }
  function validItem(item) {
    return item && ['text', 'workspace', 'askerSession', 'group', 'groupTitle', 'createdAt'].every(function (key) { return typeof item[key] === 'string'; }) && Number.isFinite(Date.parse(item.createdAt)) && Number.isSafeInteger(item.messageId) && item.messageId > 0 && typeof item.id === 'string' && item.id && Number.isSafeInteger(item.revision) && item.revision > 0 &&
      (item.kind === 'request' || item.kind === 'mention') && typeof item.canDismiss === 'boolean' && typeof item.dismissalReason === 'string' && typeof item.destination === 'string' && item.destination && typeof item.asker === 'string' &&
      Array.isArray(item.questions) && (item.kind === 'mention' ? item.questions.length === 0 : item.questions.length > 0 && item.questions.length <= 20) && new Set(item.questions.map(function (q) { return q.number; })).size === item.questions.length && item.questions.every(function (q) { return Number.isSafeInteger(q.number) && q.number > 0 && typeof q.text === 'string' && typeof q.recommendation === 'string' && Array.isArray(q.choices) && q.choices.every(function (c) { return typeof c === 'string'; }) && typeof q.answered === 'boolean' && typeof q.answer === 'string'; });
  }
  function validReceipt(receipt, op) {
    return receipt && receipt.operationId === op.input.operationId && (receipt.state === 'committed' || receipt.state === 'rejected') && receipt.id === (op.input.id || '') &&
      Number.isSafeInteger(receipt.revision) && receipt.revision >= 0 && typeof receipt.closed === 'boolean' && Array.isArray(receipt.messageIds) && receipt.messageIds.every(Number.isSafeInteger) && (receipt.state === 'committed' ? receipt.reason === null : receipt.reason && typeof receipt.reason.code === 'string' && typeof receipt.reason.message === 'string' && !!receipt.reason.message && receipt.messageIds.length === 0);
  }
  function init(root, opts) {
    var disposed = false, ready = false, readNumber = 0, refreshing = false, refreshAgain = false, moreBefore = 0, loadedPages = 1, readPromise = null, readValid = false;
    var items = new Map(), order = [], selected = '', selectedCopy = null, filter = '', view = 'requests';
    var rendered = '', draft = null, draftKey = '', saving = 0, draftError = '', pending = new Set();
    root.classList.add('decisions');
    // PUDL temporarily makes automatic docks rails while this window needs room.
    root.closest('[data-win]').setAttribute('data-win-room', '84rem');
    var toolbar = node('div', 'decisions-toolbar'); toolbar.setAttribute('role', 'toolbar'); toolbar.setAttribute('aria-label', 'Decisions');
    var listCommand = button('Requests', function () { navigate('requests', ''); });
    var search = node('input', 'form-input'); search.type = 'search'; search.placeholder = 'Filter Requests'; search.setAttribute('aria-label', 'Filter Requests');
    var refresh = button('Refresh', function () { if (view === 'record') { rendered = ''; draw(); } return load(false); });
    var more = button('Earlier Requests', function () { return load(true); }); more.disabled = true;
    toolbar.append(listCommand, search, refresh, more);
    var status = node('p', 'decisions-status text-muted', 'Reading requests.'); status.setAttribute('role', 'status'); status.setAttribute('aria-live', 'polite');
    var body = node('div', 'decisions-body'), list = node('ul', 'decisions-list'); list.setAttribute('aria-label', 'Open Requests');
    var detail = node('section', 'decisions-detail'); detail.setAttribute('aria-label', 'Request Details');
    body.append(list, detail); root.append(toolbar, status, body);
    var form = null, send = null, receiptLine = null, changedLine = null, review = null, destination = null;
    function address() { var p = new URLSearchParams(); if (selected) p.set('request', selected); if (filter) p.set('q', filter); if (view !== 'requests') p.set('view', view); return p.toString(); }
    function navigate(nextView, id) { view = nextView; selected = id || ''; rendered = ''; opts.changed(address()); draw(); }
    function say(text) { if (!disposed) status.textContent = text; }
    function current() { return items.get(selected) || selectedCopy; }
    function stateRead(value) {
      var p = new URLSearchParams(value || ''); selected = p.get('request') || ''; filter = p.get('q') || '';
      view = /^(broadcast-active|broadcast-all|record)$/.test(p.get('view')) ? p.get('view') : 'requests'; search.value = filter; rendered = '';
    }
    function draftFor(id) { return window.dinahDecisionDrafts.read(id); }
    function store(id, value) { return window.dinahDecisionDrafts.write(id, value); }
    function persist() {
      var key = draftKey, value = draft; saving++; draftError = ''; controls();
      return store(key, value).then(function () { if (draftKey === key) draftError = ''; }, function (error) {
        if (draftKey === key) draftError = 'Draft could not be saved. ' + errorText(error);
        throw error;
      }).finally(function () { saving--; controls(); });
    }
    function saveEdited() { if (draft.operation && !workingOp(draft)) draft.operation = null; persist().catch(function () {}); }
    function workingOp(value) { return value && value.operation && !['committed', 'rejected'].includes(value.operation.status); }
    function controls() {
      if (disposed || !draft || !form) return;
      var item = current(), busy = !!workingOp(draft), broadcast = view !== 'requests';
      var blocked = busy || !ready;
      form.querySelectorAll('textarea,input,select,[data-edit-answer]').forEach(function (el) { el.disabled = blocked || !broadcast && (!items.has(selected) || !!(draft.operation && draft.operation.status === 'committed' && draft.operation.receipt && draft.operation.receipt.closed)); });
      if (send && !broadcast) {
        var fields = item && item.kind === 'mention' ? [{ number: 0, answered: false }] : item ? item.questions : [];
        var any = fields.some(function (q) { var a = draft.answers[q.number]; return !q.answered && a && (a.asRecommended || a.text.trim()); });
        send.disabled = blocked || saving > 0 || !!draftError || !item || !items.has(selected) || item.kind === 'request' && item.canDismiss || draft.reviewed !== signature(item) || !any;
      }
      var dismissButton = form.querySelector('[data-dismiss-decision]');
      if (dismissButton) dismissButton.disabled = busy || saving > 0 || !items.has(selected) || !item.canDismiss;
      if (changedLine && !broadcast) {
        var changed = item && draft.reviewed !== signature(item);
        changedLine.textContent = changed ? 'The request or destination changed. Review it before sending.' : !items.has(selected) ? 'This request is not in the loaded open list. Its draft is kept. Refresh or load earlier requests.' : '';
        review.hidden = !changed; review.disabled = busy || saving > 0;
      }
      if (broadcast) {
        var previewButton = form.querySelector('[data-broadcast="preview"]'), confirmButton = form.querySelector('[data-broadcast="confirm"]');
        if (previewButton) previewButton.disabled = blocked || form.dataset.previewPending === 'true' || !(draft.text || '').trim();
        if (confirmButton) confirmButton.disabled = blocked || saving > 0 || !!draftError || !draft.preview || Date.parse(draft.preview.expiresAt) <= Date.now() || draft.preview.text !== draft.text;
      }
      if (receiptLine) {
        var op = draft.operation;
        receiptLine.textContent = deliveryStatus();
        detail.querySelectorAll('[data-operation]').forEach(function (el) { el.disabled = !workingOp(draft) || pending.has(op.input.operationId); });
      }
    }
    function deliveryStatus() {
      if (draftError) return draftError;
      var op = draft.operation;
      if (!op) return saving ? 'Saving draft.' : 'Draft saved.';
      if (op.status === 'rejected') return op.receipt.reason.message;
      if (op.status !== 'committed') return 'Delivery is unconfirmed. Check its status before sending again.';
      if (op.method === 'DismissDecision') return 'Message dismissed. No reply was sent.';
      if (op.method === 'ConfirmDecisionBroadcast') return 'Broadcast queued on the bus for the previewed chats.';
      return 'Delivered to the bus. The asking chat may not have read it yet.';
    }
    function drawList() {
      var visible = order.filter(function (id) { var item = items.get(id); return item && (!filter || [item.asker, item.workspace, item.groupTitle, item.text].join(' ').toLowerCase().includes(filter.toLowerCase())); });
      var existing = new Map(Array.from(list.children).map(function (li) { return [li.dataset.requestId, li]; }));
      visible.forEach(function (id) {
        var item = items.get(id), li = existing.get(id);
        if (!li) { li = node('li', 'decisions-row'); li.dataset.requestId = id; var a = node('a'); a.append(node('strong'), node('small'), node('small')); a.addEventListener('click', function (e) { e.preventDefault(); selectedCopy = item; navigate('requests', id); }); li.appendChild(a); list.appendChild(li); }
        var link = li.firstChild, summary = item.kind === 'request' ? item.questions.filter(function (q) { return !q.answered; }).length + ' questions waiting' : 'Message asking for you';
        link.href = '?' + new URLSearchParams({ open: 'decisions', 'view.decisions': 'request=' + encodeURIComponent(id) }).toString();
        link.setAttribute('aria-current', String(view === 'requests' && selected === id));
        link.children[0].textContent = item.kind === 'request' ? item.questions[0].text : item.text.split(/\r?\n/)[0];
        link.children[1].textContent = item.asker + (item.groupTitle ? ' in ' + item.groupTitle : '') + ' · ' + item.workspace;
        link.children[2].textContent = summary + ' · ' + window.dinahRegional.format(new Date(item.createdAt), 'dayTime'); existing.delete(id);
      });
      existing.forEach(function (li) { li.remove(); });
    }
    function question(q, item) {
      var field = node('fieldset', 'decisions-question'); field.dataset.question = String(q.number);
      field.appendChild(node('legend', '', item.kind === 'mention' ? 'Your Reply' : 'Question ' + q.number));
      field.appendChild(node('p', 'decisions-question-text', q.text));
      if (q.answered) { field.appendChild(node('p', 'text-muted', 'Answered: ' + q.answer)); return field; }
      if (item.kind === 'request') field.appendChild(node('p', 'decisions-recommendation', 'Recommendation: ' + q.recommendation));
      var a = draft.answers[q.number] || { text: '', asRecommended: false }, label = node('label', '', item.kind === 'mention' ? 'Your Reply' : 'Answer ' + q.number), input = node('textarea', 'form-input');
      input.value = a.asRecommended ? q.recommendation : a.text; input.maxLength = 8000; input.setAttribute('aria-label', item.kind === 'mention' ? 'Your Reply' : 'Answer ' + q.number); label.appendChild(input); field.appendChild(label);
      input.addEventListener('input', function () { draft.answers[q.number] = { text: input.value, asRecommended: false }; saveEdited(); });
      if (item.kind === 'request') {
        var recommend = button('As Recommended', function () { draft.answers[q.number] = { text: '', asRecommended: true }; input.value = q.recommendation; saveEdited(); }); recommend.dataset.editAnswer = ''; field.appendChild(recommend);
        if (q.choices.length) {
          var choices = node('select', 'form-select'); choices.setAttribute('aria-label', 'Suggested Answers ' + q.number); choices.add(new Option('Choose a suggested answer', ''));
          q.choices.forEach(function (choice) { choices.add(new Option(choice, choice)); });
          choices.addEventListener('change', function () { if (choices.value) { input.value = choices.value; draft.answers[q.number] = { text: choices.value, asRecommended: false }; saveEdited(); } }); field.appendChild(choices);
        }
      }
      return field;
    }
    function operationActions() {
      var actions = node('div', 'decisions-actions'), check = button('Check Delivery', reconcile);
      check.dataset.operation = 'check'; actions.appendChild(check); return actions;
    }
    function deliveryPanel() {
      var panel = node('div', 'decisions-delivery');
      receiptLine = node('p', 'decisions-receipt'); receiptLine.setAttribute('role', 'status');
      panel.append(receiptLine, operationActions()); return panel;
    }
    function drawRequest() {
      var item = items.get(selected) || selectedCopy;
      if (selected && (!item || item.id !== selected)) { var saved = draftFor('request:' + selected); if (validItem(saved.request) && saved.request.id === selected) item = saved.request; }
      if (!selected || !item || item.id !== selected) { detail.replaceChildren(node('p', 'text-muted', selected ? 'The selected request is not in the loaded page. Load earlier requests or choose another. Saved drafts are retained.' : 'Choose a request to read and answer.')); form = null; return; }
      selectedCopy = item;
      var shape = selected + '|' + signature(item) + '|' + item.revision;
      if (rendered === shape) { controls(); return; }
      rendered = shape; draftKey = 'request:' + item.id; draft = draftFor(draftKey); draft.request = item; draftError = '';
      if (!draft.reviewed && !workingOp(draft)) draft.reviewed = signature(item);
      form = node('form'); form.addEventListener('submit', function (event) { event.preventDefault(); if (!send.disabled) answer(); });
      var source = node('header', 'decisions-source');
      source.appendChild(node('h2', '', item.asker));
      source.appendChild(node('p', 'text-muted', (item.kind === 'request' ? 'Questions for You' : item.group ? 'Group Mention' : 'Message for You') + ' · ' + item.workspace + (item.groupTitle ? ' · ' + item.groupTitle : '') + ' · ' + window.dinahRegional.format(new Date(item.createdAt), 'dayTime')));
      form.append(source, deliveryPanel());
      var content = node('div', 'decisions-content'); form.appendChild(content);
      changedLine = node('p', 'text-warning'); review = button('I Have Reviewed the Updated Request', function () { draft.reviewed = signature(item); saveEdited(); });
      var reviewPanel = node('div', 'decisions-review'); reviewPanel.append(changedLine, review); content.appendChild(reviewPanel);
      if (item.dismissalReason) content.appendChild(node('p', 'decisions-recommendation', item.dismissalReason));
      content.appendChild(window.dinahDecisionContext(item));
      (item.kind === 'mention' ? [{ number: 0, text: item.text, choices: [], answered: false }] : item.questions).forEach(function (q) { content.appendChild(question(q, item)); });
      destination = node('p', 'decisions-destination', 'Send to ' + item.destination + '. Reply to message ' + item.messageId + '.'); content.appendChild(destination);
      send = button('Send Answers', answer, true); var actions = node('div', 'decisions-actions'); actions.appendChild(send);
      if (item.canDismiss) { var dismissButton = button(item.kind === 'mention' ? 'Dismiss Message' : 'Dismiss Request', dismiss); dismissButton.dataset.dismissDecision = ''; actions.appendChild(dismissButton); } content.appendChild(actions);
      var oldActive = detail.contains(document.activeElement) && document.activeElement.tagName === 'TEXTAREA' ? document.activeElement : null;
      var oldLabel = oldActive && oldActive.getAttribute('aria-label'), oldStart = oldActive && oldActive.selectionStart, oldEnd = oldActive && oldActive.selectionEnd;
      detail.replaceChildren(form); controls();
      if (oldLabel) { var replacement = Array.from(form.querySelectorAll('textarea')).find(function (input) { return input.getAttribute('aria-label') === oldLabel; }); if (replacement && !replacement.disabled) { replacement.focus({ preventScroll: true }); replacement.setSelectionRange(oldStart, oldEnd); } }
      if (workingOp(draft)) reconcile();
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
      return window.dinahHost.ask(item.kind === 'mention' ? 'Dismiss This Message?' : 'Dismiss This Request?', 'This closes the plain request without sending a reply to ' + item.destination + '.', 'Dismiss').then(function (yes) { if (yes && !disposed && draftKey === 'request:' + item.id) return begin('DismissDecision', { operationId: operationId(), id: item.id, revision: item.revision }); });
    }
    function begin(method, input) {
      var key = draftKey, value = draft;
      draft.operation = { method: method, input: input, status: 'prepared' };
      return persist().then(function () { return execute(key, value); }).catch(function (error) { say(errorText(error)); });
    }
    function accept(key, value, receipt) {
      var op = value.operation; if (!validReceipt(receipt, op)) throw new Error('The delivery response could not be confirmed.');
      op.status = receipt.state; op.receipt = receipt;
      if (op.method === 'ConfirmDecisionBroadcast') value.preview = null;
      if (receipt.state === 'committed' && op.method === 'AnswerDecision') op.input.answers.forEach(function (answer) { delete value.answers[answer.number]; });
      return store(key, value).then(function () { if (key === draftKey) { draft = value; if (view !== 'requests') rendered = ''; controls(); } return load(false); });
    }
    function execute(key, value) {
      var op = value.operation;
      if (pending.has(op.input.operationId)) return Promise.resolve();
      pending.add(op.input.operationId); op.status = 'submitted'; controls();
      return store(key, value).then(function () { return app()[op.method](op.input); }).then(function (receipt) { return accept(key, value, receipt); }).catch(function (error) {
        if (!['committed', 'rejected'].includes(op.status)) { op.status = 'submitted'; return store(key, value).then(function () { say('Delivery is unconfirmed. ' + errorText(error)); }); }
        say('The reply committed, but its local state could not be saved. ' + errorText(error));
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
      }).catch(function (error) { say('Delivery is still unconfirmed. ' + errorText(error)); }).finally(function () { pending.delete(op.input.operationId); if (key === draftKey) controls(); });
    }
    function drawBroadcast() {
      var scope = view === 'broadcast-active' ? 'active' : 'all', key = 'broadcast-' + scope;
      if (rendered === key) { controls(); return; }
      rendered = key; draftKey = key; draft = draftFor(key); draftError = '';
      form = node('form', 'decisions-broadcast'); form.addEventListener('submit', function (e) { e.preventDefault(); });
      form.appendChild(node('h2', '', scope === 'active' ? 'Broadcast to Active' : 'Broadcast to All'));
      form.appendChild(deliveryPanel());
      var content = node('div', 'decisions-content'); form.appendChild(content);
      content.appendChild(node('p', '', scope === 'active' ? 'The preview lists chats working when the engine reads them.' : 'The preview lists every eligible chat on the bus. Closed chats wait for a background place. External terminals cannot be opened by Dinah.'));
      var label = node('label', '', 'Message'), input = node('textarea', 'form-input'); input.setAttribute('aria-label', 'Broadcast Message'); input.maxLength = 16000; input.value = draft.text || ''; label.appendChild(input); content.appendChild(label);
      var previewBox = node('div', 'decisions-preview'), previewButton = button('Preview Recipients', preview), confirmButton = button('Confirm Broadcast…', confirmBroadcast, true); confirmButton.disabled = true;
      input.addEventListener('input', function () { draft.text = input.value; draft.preview = null; previewBox.replaceChildren(); confirmButton.disabled = true; saveEdited(); });
      previewButton.dataset.broadcast = 'preview'; confirmButton.dataset.broadcast = 'confirm';
      var actions = node('div', 'decisions-actions'); actions.append(previewButton, confirmButton); content.append(actions, previewBox);
      detail.replaceChildren(form);
      function paintPreview() {
        var value = draft.preview; previewBox.replaceChildren(); confirmButton.disabled = true;
        if (!value) return;
        previewBox.appendChild(node('p', '', value.count + (value.count === 1 ? ' chat' : ' chats') + ' will receive this message. Preview expires ' + window.dinahRegional.format(new Date(value.expiresAt), 'dayTime') + '.'));
        var ul = node('ul'); value.targets.forEach(function (target) { ul.appendChild(node('li', '', target.title + ' · ' + target.workspace)); }); previewBox.appendChild(ul);
        confirmButton.disabled = workingOp(draft) || saving > 0 || !!draftError || Date.parse(value.expiresAt) <= Date.now();
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
      root.dataset.decisionDetail = String(view !== 'requests' || !!selected);
      drawList();
      try {
        if (view === 'record') {
          form = null;
          var recordId = selected, shape = 'record|' + recordId;
          if (rendered !== shape) {
            rendered = shape;
            window.dinahDecisionRecord(detail, recordId, validItem, function () { return !disposed && view === 'record' && selected === recordId; });
          }
        } else if (view === 'requests') drawRequest(); else drawBroadcast();
      } catch (error) { form = null; say(errorText(error)); detail.replaceChildren(node('p', 'text-muted', 'The saved draft could not be read. It has not been replaced.')); }
    }
    function load(append) {
      if (!ready || disposed) return Promise.resolve();
      if (!available()) { say('Decisions are not available in this build.'); refresh.disabled = true; return Promise.resolve(); }
      if (refreshing) { refreshAgain = true; return readPromise; }
      var before = append ? moreBefore : 0, sequence = ++readNumber;
      refreshing = true; readValid = false; refresh.disabled = true; more.disabled = true;
      var collected = [], pages = 0, keepSelection = !append && !!selected && view === 'requests', targetPages = append ? 1 : loadedPages;
      function readPage(cursor) {
        return Promise.resolve().then(function () { return app().Decisions(cursor); }).then(function (page) {
          if (!page || !Array.isArray(page.items) || !page.items.every(validItem) || !Number.isSafeInteger(page.nextBefore) || page.nextBefore < 0 || cursor && page.nextBefore >= cursor) throw new Error('The request list is invalid.');
          collected.push.apply(collected, page.items); pages++;
          if (page.nextBefore && (pages < targetPages || keepSelection && !collected.some(function (item) { return item.id === selected; }))) return readPage(page.nextBefore);
          return { items: collected, nextBefore: page.nextBefore };
        });
      }
      readPromise = readPage(before).then(function (result) {
        if (disposed || sequence !== readNumber) return;
        if (!result || !Array.isArray(result.items) || !result.items.every(validItem) || new Set(result.items.map(function (item) { return item.id; })).size !== result.items.length || !Number.isSafeInteger(result.nextBefore) || result.nextBefore < 0) throw new Error('The request list is invalid.');
        if (!append) { if (items.has(selected)) selectedCopy = items.get(selected); items.clear(); }
        result.items.forEach(function (item) { items.set(item.id, item); if (!order.includes(item.id)) order.push(item.id); });
        order = order.filter(function (id) { return items.has(id); }); moreBefore = result.nextBefore; loadedPages = append ? loadedPages + pages : pages;
        say(items.size ? items.size + ' open requests loaded.' : 'No open requests.'); draw(); readValid = true;
      }).catch(function (error) { say('Requests could not be read. ' + errorText(error)); }).finally(function () {
        refreshing = false; if (!disposed) { refresh.disabled = false; more.disabled = !moreBefore; }
        if (refreshAgain) { refreshAgain = false; return load(false); }
      });
      return readPromise;
    }
    search.addEventListener('input', function () { filter = search.value; opts.changed(address()); drawList(); });
    stateRead(opts.state);
    var unsubscribe = window.dinahHost.on('bus:changed', function () { load(false); });
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
    return { ensureRequest: ensureRequest, state: address, setState: function (state) { stateRead(state); draw(); }, destroy: function () { disposed = true; unsubscribe(); unresync(); }, commands: function () {
      return [{ label: 'Refresh Requests', run: function () { return load(false); } }, { label: 'Broadcast to Active…', run: function () { navigate('broadcast-active'); } }, { label: 'Broadcast to All…', run: function () { navigate('broadcast-all'); } }];
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
