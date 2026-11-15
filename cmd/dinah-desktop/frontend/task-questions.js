/* Task questions share one pending state across their reading surfaces. */
(function () {
  'use strict';
  var entries = new Map(), ended = new Set(), versions = new Map(), listeners = new Set();
  var clock = 0, reads = 0, cards = 0, connected = false, uiReady = false, early = new Map(), app;
  function valid(q) {
    if (q && q.answerType === 'text' && q.options === null) q = Object.assign({}, q, { options: [] });
    return q && ['key', 'service', 'serviceName', 'workspace', 'workspaceName', 'task', 'generation', 'taskTitle', 'question', 'askedAt'].every(function (name) { return typeof q[name] === 'string' && !!q[name]; }) &&
      !('sessionId' in q) && Number.isFinite(Date.parse(q.askedAt)) && q.target &&
      (typeof q.target.href === 'string' && q.target.href.startsWith('?') && !q.target.reason || typeof q.target.reason === 'string' && !!q.target.reason && !q.target.href) &&
      Array.isArray(q.options) && q.options.length <= 16 && (q.answerType === 'text' ? Number.isInteger(q.minLength) && Number.isInteger(q.maxLength) && q.minLength >= 0 && q.maxLength > 0 && q.maxLength >= q.minLength && q.maxLength <= 16384 && q.options.length === 0 : q.options.length > 0 && (!q.answerType || q.answerType === 'multiple' && Number.isInteger(q.minSelections) && Number.isInteger(q.maxSelections) && q.minSelections >= 0 && q.maxSelections >= q.minSelections && q.maxSelections <= q.options.length)) && q.options.every(function (o) { return o && typeof o.optionId === 'string' && !!o.optionId && typeof o.name === 'string' && !!o.name && typeof o.kind === 'string'; }) &&
      new Set(q.options.map(function (o) { return o.optionId; })).size === q.options.length;
  }
  function changed(key) {
    versions.set(key, ++clock);
    listeners.forEach(function (fn) { fn(key); });
    document.querySelectorAll('[data-task-question-key="' + CSS.escape(key) + '"]').forEach(function (card) { paint(card, entries.get(key)); });
  }
  function asked(q) {
    if (!valid(q) || ended.has(q.key)) return;
    if (!uiReady) { early.set(q.key, q); return; }
    if (q.options === null) q = Object.assign({}, q, { options: [] });
    var old = entries.get(q.key);
    // A host key identifies an immutable question. Replayed events cannot reset an answer.
    if (old) return;
    var saved = window.dinahUIState.get('task-question:' + q.key, null);
    var entry = { question: q, choice: '', text: '', optionIds: [], answering: false, uncertain: false, status: '' };
    if (saved && typeof saved.text === 'string' && typeof saved.choice === 'string' && Array.isArray(saved.optionIds) && saved.optionIds.every(function (id) { return q.options.some(function (o) { return o.optionId === id; }); })) {
      entry.text = saved.text; entry.choice = saved.choice; entry.optionIds = saved.optionIds;
    }
    entries.set(q.key, entry); changed(q.key);
  }
  function finish(event) {
    if (!event || typeof event.key !== 'string' || !event.key) return;
    ended.add(event.key); entries.delete(event.key); changed(event.key);
  }
  function refresh() {
    if (!app || !app().PendingTaskPermissions) return Promise.resolve();
    var sequence = ++reads, before = clock;
    return Promise.resolve().then(function () { return app().PendingTaskPermissions(); }).then(function (list) {
      if (sequence !== reads) return;
      list = list || [];
      if (!Array.isArray(list) || !list.every(valid) || new Set(list.map(function (q) { return q.key; })).size !== list.length) throw new Error('The task questions could not be read.');
      var pending = new Set(list.map(function (q) { return q.key; }));
      Array.from(entries.keys()).forEach(function (key) { if (!pending.has(key) && (versions.get(key) || 0) <= before) finish({ key: key }); });
      list.forEach(function (q) {
        if (ended.has(q.key) || (versions.get(q.key) || 0) > before) return;
        asked(q);
        var entry = entries.get(q.key);
        if (entry && entry.uncertain) { entry.uncertain = false; entry.answering = false; entry.status = 'This question is still waiting. Review your answer before sending again.'; changed(q.key); }
      });
    });
  }
  function validation(entry) {
    if (!entry) return 'This question is no longer waiting.';
    var q = entry.question;
    if (window.dinahUIState.initializationError()) return 'Saved drafts could not be read. Sending is disabled.';
    if (entry.answering) return 'Your answer is being sent.';
    if (entry.uncertain) return 'Check the question status before sending again.';
    if (entry.draftError) return entry.draftError;
    if (q.answerType === 'text') {
      var n = Array.from(entry.text).length;
      if (n < q.minLength) return 'Enter at least ' + q.minLength + ' characters.';
      if (n > q.maxLength) return 'Remove ' + (n - q.maxLength) + ' characters before sending.';
      if (/[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/u.test(entry.text)) return 'Your answer contains an incomplete character.';
    } else if (q.answerType === 'multiple') {
      if (entry.optionIds.length < q.minSelections) return 'Choose at least ' + q.minSelections + ' options.';
      if (entry.optionIds.length > q.maxSelections) return 'Choose no more than ' + q.maxSelections + ' options.';
    } else if (!q.options.some(function (o) { return o.optionId === entry.choice; })) return 'Choose an answer to send it.';
    return '';
  }
  function save(key) {
    var entry = entries.get(key); if (!entry) return;
    window.dinahUIState.set('task-question:' + key, { text: entry.text, choice: entry.choice, optionIds: entry.optionIds });
    entry.draftError = '';
    window.dinahUIState.flush().catch(function (error) { if (entries.get(key) === entry) { entry.draftError = 'Your draft could not be saved. ' + window.dinahHelpers.message(error, 'sentence'); changed(key); } });
    changed(key);
  }
  function answer(key) {
    var entry = entries.get(key);
    if (!entry || validation(entry)) return Promise.resolve();
    entry.answering = true; entry.status = 'Your answer is being sent.'; changed(key);
    return window.dinahUIState.flush().then(function () {
      var q = entry.question;
      return q.answerType ? app().AnswerTaskQuestion(key, JSON.stringify(q.answerType === 'text' ? { type: 'text', text: entry.text } : { type: 'multiple', optionIds: q.options.filter(function (o) { return entry.optionIds.includes(o.optionId); }).map(function (o) { return o.optionId; }) })) : app().Answer(key, entry.choice);
    }).then(function () { finish({ key: key }); }).catch(function (error) {
      if (entries.get(key) !== entry) return;
      entry.answering = false; entry.uncertain = true;
      entry.status = 'Your answer could not be confirmed. ' + window.dinahHelpers.message(error, 'sentence'); changed(key);
      return refresh().catch(function () { if (entries.get(key) === entry) { entry.status = 'Your answer could not be confirmed. Check the question status before sending again.'; changed(key); } });
    });
  }
  function paint(card, entry) {
    if (!entry) { card.hidden = true; return; }
    var q = entry.question;
    card.querySelectorAll('input').forEach(function (input) { input.disabled = entry.answering || entry.uncertain; input.checked = q.answerType === 'multiple' ? entry.optionIds.includes(input.value) : input.value === entry.choice; });
    var text = card.querySelector('textarea');
    if (text) { if (text.value !== entry.text) text.value = entry.text; text.disabled = entry.answering || entry.uncertain; }
    var reason = validation(entry), send = card.querySelector('[data-task-answer]'), check = card.querySelector('[data-task-check]');
    if (send) { send.disabled = !!reason; send.title = reason; }
    if (check) check.disabled = entry.answering;
    var count = card.querySelector('[data-task-count]');
    if (count) {
      var n = q.answerType === 'text' ? Array.from(entry.text).length : entry.optionIds.length;
      var over = q.answerType === 'text' ? n > q.maxLength : n > q.maxSelections;
      count.textContent = (over ? '▲ ' : '') + (q.answerType === 'text' ? n + ' of ' + q.maxLength + ' characters (minimum ' + q.minLength + ')' : n + ' selected; choose ' + q.minSelections + ' to ' + q.maxSelections);
      count.dataset.tone = over ? 'danger' : ''; if (text) text.setAttribute('aria-invalid', String(over));
    }
    var status = card.querySelector('[data-task-status]'); if (status) status.textContent = entry.status || entry.draftError || '';
  }
  function render(card, key, settings) {
    settings = settings || {};
    var entry = entries.get(key); if (!entry) return;
    var q = entry.question, node = window.dinahHelpers.node, radioGroup = 'task-answer-' + (++cards);
    card.dataset.taskQuestionKey = key;
    var content = node('div', 'notice-content'), title = node('p', 'notice-title', q.question);
    if (!settings.embedded) content.append(title, node('p', 'notice-body', q.serviceName + ' asks about "' + q.taskTitle + '" in ' + q.workspaceName + '.'));
    var meta = node('p', 'notices-meta', 'Asked at '), at = node('time', 'notices-time', window.dinahRegional.format(new Date(q.askedAt), 'dayTime'));
    at.dateTime = q.askedAt; meta.appendChild(at); if (!settings.embedded) content.appendChild(meta);
    if (!settings.embedded && q.target.href) {
      var link = node('a', 'btn btn-sm', 'Open Task'); link.href = q.target.href;
      link.addEventListener('click', function (event) {
        if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || event.button) return;
        event.preventDefault();
        var query = new URLSearchParams(q.target.href.slice(1)), page = query.get('open');
        window.dinahWorkbench.open(page, query.get('view.' + page) || '').catch(window.dinahHost.fail);
      }); content.appendChild(link);
    } else if (!settings.embedded) content.appendChild(node('p', 'notice-body', q.target.reason + ' You can still answer this question.'));
    var choices = node('fieldset', 'task-question-choices'); choices.appendChild(node('legend', '', 'Your Answer'));
    if (q.answerType === 'text') {
      var label = node('label', '', 'Your Answer'), text = node('textarea', 'form-input form-textarea'); text.value = entry.text; text.setAttribute('aria-label', 'Your Answer'); text.addEventListener('input', function () { entry.text = text.value; save(key); }); label.appendChild(text); choices.appendChild(label);
    } else q.options.forEach(function (option) {
      var label = node('label'), radio = node('input'); radio.type = q.answerType === 'multiple' ? 'checkbox' : 'radio'; radio.name = radioGroup; radio.value = option.optionId;
      radio.addEventListener('change', function () { if (entries.get(key) !== entry || entry.answering || entry.uncertain) return; if (q.answerType === 'multiple') entry.optionIds = radio.checked ? entry.optionIds.concat(option.optionId) : entry.optionIds.filter(function (id) { return id !== option.optionId; }); else entry.choice = option.optionId; save(key); });
      label.append(radio, document.createTextNode(option.name)); choices.appendChild(label);
    }); content.appendChild(choices);
    if (q.answerType) { var count = node('p', 'decisions-count'); count.dataset.taskCount = ''; count.setAttribute('aria-live', 'polite'); content.appendChild(count); }
    var status = node('p', 'task-question-result'); status.dataset.taskStatus = ''; status.setAttribute('role', 'status'); status.setAttribute('aria-live', 'polite');
    var actions = node('div', 'notice-actions hstack gap-2'), send = node('button', 'btn btn-sm btn-primary', 'Send Answer'), check = node('button', 'btn btn-sm', 'Check Question Status');
    send.type = check.type = 'button'; send.dataset.taskAnswer = ''; check.dataset.taskCheck = '';
    send.addEventListener('click', function () { answer(key); });
    check.addEventListener('click', function () { refresh().catch(function (error) { if (entries.get(key) === entry) { entry.status = window.dinahHelpers.message(error, 'sentence'); changed(key); } }); });
    actions.append(send, check); if (!settings.embedded) content.append(status, actions); card.appendChild(content); paint(card, entry);
  }
  window.dinahTaskQuestions = {
    connect: function (adapter) {
      if (connected) return; connected = true; app = adapter.app;
      adapter.on('task:permission', asked); adapter.on('task:answered', finish);
      window.dinahHost.onResync(refresh);
      window.dinahUIState.ready.then(function () { uiReady = true; early.forEach(asked); early.clear(); return refresh(); }).catch(adapter.fail);
    },
    get: function (key) { return entries.get(key); },
    list: function () { return Array.from(entries.values()); },
    subscribe: function (listener) { listeners.add(listener); return function () { listeners.delete(listener); }; },
    refresh: refresh, answer: answer, render: render, validation: function (key) { return validation(entries.get(key)); },
    open: function (key) { var entry = entries.get(key); if (!entry || !entry.question.target.href) return; var query = new URLSearchParams(entry.question.target.href.slice(1)), page = query.get('open'); return window.dinahWorkbench.open(page, query.get('view.' + page) || ''); }
  };
}());
