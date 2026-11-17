/* Task questions share one pending state across their reading surfaces. */
(function () {
  'use strict';
  var checkingDismissal = 'Dinah is checking whether this question can be dismissed.';
  // The ways the host says a question ended (taskhost's question record).
  var dispositions = ['answered', 'dismissed', 'withdrawn', 'cancelled'];
  // ended maps each finished key to how it ended, so that a replayed event cannot restore the question and a
  // reading surface can say what happened to it.
  var entries = new Map(), ended = new Map(), versions = new Map(), listeners = new Set();
  var clock = 0, reads = 0, cards = 0, connected = false, uiReady = false, early = new Map(), app;
  function message(error) { return window.dinahHelpers.message(error, 'sentence'); }
  function operationId() { return Array.from(crypto.getRandomValues(new Uint32Array(4))).map(function (n) { return n.toString(16).padStart(8, '0'); }).join(''); }
  function dismissKey(key) { return 'task-question-dismissal:' + key; }
  function valid(q) {
    if (q && q.answerType === 'text' && q.options === null) q = Object.assign({}, q, { options: [] });
    return q && ['key', 'service', 'serviceName', 'workspace', 'workspaceName', 'task', 'generation', 'taskTitle', 'question', 'askedAt'].every(function (name) { return typeof q[name] === 'string' && !!q[name]; }) &&
      !('sessionId' in q) && Number.isFinite(Date.parse(q.askedAt)) && q.target &&
      (typeof q.target.href === 'string' && q.target.href.startsWith('?') && !q.target.reason || typeof q.target.reason === 'string' && !!q.target.reason && !q.target.href) &&
      Array.isArray(q.options) && q.options.length <= 16 && (q.answerType === 'text' ? Number.isInteger(q.minLength) && Number.isInteger(q.maxLength) && q.minLength >= 0 && q.maxLength > 0 && q.maxLength >= q.minLength && q.maxLength <= 16384 && q.options.length === 0 : q.options.length > 0 && (!q.answerType || q.answerType === 'multiple' && Number.isInteger(q.minSelections) && Number.isInteger(q.maxSelections) && q.minSelections >= 0 && q.maxSelections >= q.minSelections && q.maxSelections <= q.options.length)) && q.options.every(function (o) { return o && typeof o.optionId === 'string' && !!o.optionId && typeof o.name === 'string' && !!o.name && typeof o.kind === 'string'; }) &&
      new Set(q.options.map(function (o) { return o.optionId; })).size === q.options.length;
  }
  // notify redraws the reading surfaces. What the host says about dismissal is not a change to the question,
  // so it is told this way and leaves the version that orders pending snapshots alone.
  function notify(key) {
    listeners.forEach(function (fn) { fn(key); });
    document.querySelectorAll('[data-task-question-key="' + CSS.escape(key) + '"]').forEach(function (card) { paint(card, entries.get(key)); });
  }
  function changed(key) { versions.set(key, ++clock); notify(key); }
  function asked(q) {
    if (!valid(q) || ended.has(q.key)) return;
    if (!uiReady) { early.set(q.key, q); return; }
    if (q.options === null) q = Object.assign({}, q, { options: [] });
    var old = entries.get(q.key);
    // A host key identifies an immutable question. Replayed events cannot reset an answer.
    if (old) return;
    var saved = window.dinahUIState.get('task-question:' + q.key, null);
    var entry = { question: q, choice: '', text: '', optionIds: [], answering: false, uncertain: false, status: '', tone: '',
      dismissal: { enabled: false, reason: checkingDismissal, revision: 0 }, dismissing: false, dismissUnconfirmed: false, operation: null, stateReads: 0 };
    if (saved && typeof saved.text === 'string' && typeof saved.choice === 'string' && Array.isArray(saved.optionIds) && saved.optionIds.every(function (id) { return q.options.some(function (o) { return o.optionId === id; }); })) {
      entry.text = saved.text; entry.choice = saved.choice; entry.optionIds = saved.optionIds;
    }
    // A dismissal that was sent before the window closed has a saved operation. Its result is checked with the
    // same operation, never a new one.
    var kept = window.dinahUIState.get(dismissKey(q.key), null);
    if (kept && kept.id === q.key && typeof kept.operationId === 'string' && kept.operationId && kept.operationId.length <= 128 && Number.isSafeInteger(kept.revision) && kept.revision > 0) { entry.operation = kept; entry.dismissUnconfirmed = true; }
    entries.set(q.key, entry); changed(q.key);
    if (entry.operation) reconcileDismissal(q.key); else readState(q.key);
  }
  function finish(event) {
    if (!event || typeof event.key !== 'string' || !event.key) return;
    var entry = entries.get(event.key), how = dispositions.includes(event.disposition) ? event.disposition : '', ending = ended.get(event.key);
    // The first report of how a question ended is kept. One that arrives without saying how leaves room for a later one that does.
    if (!ending) { ending = { disposition: how, question: entry ? entry.question : null }; ended.set(event.key, ending); }
    else if (!ending.disposition) ending.disposition = how;
    if (entry) window.dinahUIState.set(dismissKey(event.key), undefined);
    entries.delete(event.key); changed(event.key);
    if (entry && !ending.disposition) readEnding(event.key);
  }
  // A question found missing from the pending list ended without its event. The host's record says how.
  function readEnding(key) {
    if (!app || !app().TaskQuestionStatus) return;
    Promise.resolve().then(function () { return app().TaskQuestionStatus(key); }).then(function (state) {
      var ending = ended.get(key);
      if (!ending || ending.disposition || !state || state.key !== key || !dispositions.includes(state.status)) return;
      ending.disposition = state.status; notify(key);
    }).catch(function () { /* The entry has already left its lists. An ending that cannot be read stays unsaid. */ });
  }
  // dismissCommand reads the host's Dismiss command from what RequestState returns. It gives whether the person
  // may use the command now, the host's reason when not, and the revision a dismissal must be sent with.
  function dismissCommand(state, key) {
    var command = state && Array.isArray(state.commands) ? state.commands.find(function (c) { return c && c.name === 'Dismiss'; }) : null;
    if (!state || state.id !== key || !Number.isSafeInteger(state.revision) || state.revision < 1 || typeof state.status !== 'string' || !command || typeof command.enabled !== 'boolean' || !command.enabled && (typeof command.reason !== 'string' || !command.reason)) throw new Error('The state of this question could not be read.');
    return { enabled: command.enabled, reason: command.enabled ? '' : command.reason, revision: state.revision };
  }
  function readState(key) {
    var entry = entries.get(key);
    if (!entry) return Promise.resolve();
    var sequence = ++entry.stateReads;
    if (!app().RequestState || !app().DismissRequest || !app().RequestOperation) { entry.dismissal = { enabled: false, reason: 'This build cannot dismiss task questions.', revision: 0 }; notify(key); return Promise.resolve(); }
    entry.readingState = true;
    return Promise.resolve().then(function () { return app().RequestState(key); }).then(function (state) {
      if (entries.get(key) !== entry || entry.stateReads !== sequence) return;
      entry.readingState = false; entry.dismissal = dismissCommand(state, key); notify(key);
    }).catch(function (error) {
      if (entries.get(key) !== entry || entry.stateReads !== sequence) return;
      entry.readingState = false; entry.dismissal = { enabled: false, reason: 'Dinah could not check whether this question can be dismissed. ' + message(error), revision: 0 }; notify(key);
    });
  }
  // Whether the person can dismiss the question now, and the reason when not. The host's command comes last,
  // after what this page is itself in the middle of.
  function dismissal(entry) {
    if (!entry) return { enabled: false, reason: 'This question is no longer waiting.' };
    if (entry.answering) return { enabled: false, reason: 'Your answer is being sent.' };
    if (entry.uncertain) return { enabled: false, reason: 'Check the question status before dismissing.' };
    if (entry.dismissing) return { enabled: false, reason: 'This question is being dismissed.' };
    if (entry.dismissUnconfirmed) return { enabled: false, reason: 'Dismissing has not been confirmed. Check the question status.' };
    return entry.dismissal;
  }
  // dismiss asks first. confirmed, when given, runs once the person has said yes and before anything is sent, so
  // that the surface can bring the question into view for what follows.
  function dismiss(key, confirmed) {
    var entry = entries.get(key);
    if (!entry || !dismissal(entry).enabled) return Promise.resolve();
    var q = entry.question;
    // The last argument makes the committing button the danger button, with Cancel first and focused and no primary button, as PUDL asks of a destructive action.
    return window.dinahHost.ask('Dismiss This Question?', 'No answer is sent, and it leaves Decisions. ' + q.serviceName + ' is told you dismissed it, with no answer, and the task decides what happens next. Use this when you have dealt with it elsewhere, or it can no longer be answered.', 'Dismiss', true).then(function (yes) {
      if (!yes || entries.get(key) !== entry || !dismissal(entry).enabled) return;
      if (confirmed) confirmed();
      entry.dismissing = true; entry.status = 'Dismissing this question.'; entry.tone = ''; notify(key);
      // The revision is read now and not taken from the earlier read, so that the host refuses a dismissal of a question that has changed since.
      return Promise.resolve().then(function () { return app().RequestState(key); }).then(function (state) {
        if (entries.get(key) !== entry) return;
        var can = dismissCommand(state, key);
        if (!can.enabled) { entry.dismissing = false; entry.dismissal = can; entry.status = can.reason; entry.tone = 'danger'; notify(key); return; }
        entry.operation = { operationId: operationId(), id: key, revision: can.revision };
        // The operation is saved before it is sent, so that a result lost with the window can still be checked.
        window.dinahUIState.set(dismissKey(key), entry.operation);
        return window.dinahUIState.flush().then(function () { if (entries.get(key) === entry) return submitDismissal(key, entry); });
      }).catch(function (error) {
        if (entries.get(key) !== entry) return;
        entry.dismissing = false; entry.operation = null; window.dinahUIState.set(dismissKey(key), undefined);
        entry.status = 'This question was not dismissed. ' + message(error); entry.tone = 'danger'; notify(key);
      });
    });
  }
  function submitDismissal(key, entry) {
    var operation = entry.operation;
    entry.dismissing = true; entry.dismissUnconfirmed = false; entry.status = 'Dismissing this question.'; entry.tone = ''; notify(key);
    return Promise.resolve().then(function () { return app().DismissRequest(operation); }).then(function (receipt) { return settleDismissal(key, entry, receipt); }).catch(function (error) {
      if (entries.get(key) !== entry) return;
      entry.dismissing = false; entry.dismissUnconfirmed = true; entry.tone = 'danger';
      entry.status = 'Dismissing has not been confirmed. ' + message(error) + ' Check the question status.'; notify(key);
    });
  }
  // A lost result is looked up by its operation. The host has no result for an operation it never finished, and
  // then the same operation is sent again, which cannot settle the question twice.
  function reconcileDismissal(key) {
    var entry = entries.get(key);
    if (!entry || !entry.operation || entry.dismissing) return Promise.resolve();
    var operation = entry.operation;
    entry.dismissing = true; entry.status = 'Checking whether this question was dismissed.'; entry.tone = ''; notify(key);
    return Promise.resolve().then(function () { return app().RequestOperation(key, operation.operationId); }).then(function (receipt) {
      if (entries.get(key) !== entry) return;
      if (receipt && receipt.operationId === operation.operationId && receipt.state === 'unknown') return submitDismissal(key, entry);
      return settleDismissal(key, entry, receipt);
    }).catch(function (error) {
      if (entries.get(key) !== entry) return;
      entry.dismissing = false; entry.dismissUnconfirmed = true; entry.tone = 'danger';
      entry.status = 'Dismissing has still not been confirmed. ' + message(error); notify(key);
    });
  }
  function settleDismissal(key, entry, receipt) {
    var operation = entry.operation;
    if (!receipt || receipt.operationId !== operation.operationId || receipt.id !== key) throw new Error('The result of the dismissal could not be read.');
    if (receipt.state === 'committed' && receipt.outcome === 'dismissed') { finish({ key: key, disposition: 'dismissed' }); return; }
    if (receipt.state !== 'refused' || typeof receipt.code !== 'string' || typeof receipt.text !== 'string' || !receipt.text) throw new Error('The result of the dismissal could not be read.');
    // A question that ended another way first has left the host's pending list, and the refusal says how it ended.
    if (receipt.code === 'request_closed') { finish({ key: key, disposition: receipt.outcome }); return; }
    if (entries.get(key) !== entry) return;
    window.dinahUIState.set(dismissKey(key), undefined);
    entry.operation = null; entry.dismissing = false; entry.dismissUnconfirmed = false;
    // The host refused, so Dismiss stays disabled with the host's reason until the question's status is checked again.
    entry.dismissal = { enabled: false, reason: receipt.text, revision: 0 };
    entry.status = receipt.text; entry.tone = 'danger'; notify(key);
  }
  function refresh() {
    if (!app || !app().PendingTaskPermissions) return Promise.resolve();
    var sequence = ++reads, before = clock;
    return Promise.resolve().then(function () { return app().PendingTaskPermissions(); }).then(function (list) {
      if (sequence !== reads) return;
      list = list || [];
      if (!Array.isArray(list) || !list.every(valid) || new Set(list.map(function (q) { return q.key; })).size !== list.length) throw new Error('The task questions could not be read.');
      var pending = new Set(list.map(function (q) { return q.key; }));
      // A question with an unconfirmed dismissal is not finished from the list alone. Its operation is looked up below, which says how it ended.
      Array.from(entries.keys()).forEach(function (key) { if (!pending.has(key) && (versions.get(key) || 0) <= before && !entries.get(key).operation) finish({ key: key }); });
      list.forEach(function (q) {
        if (ended.has(q.key) || (versions.get(q.key) || 0) > before) return;
        asked(q);
        var entry = entries.get(q.key);
        if (entry && entry.uncertain) { entry.uncertain = false; entry.answering = false; entry.status = 'This question is still waiting. Review your answer before sending again.'; entry.tone = ''; changed(q.key); }
      });
      // Checking the questions also checks a dismissal whose result was lost, and asks the host again whether each question can be dismissed.
      return Promise.all(Array.from(entries.values()).map(function (entry) { return entry.operation ? reconcileDismissal(entry.question.key) : entry.dismissing || entry.readingState ? null : readState(entry.question.key); }));
    });
  }
  function validation(entry) {
    if (!entry) return 'This question is no longer waiting.';
    var q = entry.question;
    if (window.dinahUIState.initializationError()) return 'Saved drafts could not be read. Sending is disabled.';
    if (entry.answering) return 'Your answer is being sent.';
    if (entry.uncertain) return 'Check the question status before sending again.';
    if (entry.dismissing) return 'This question is being dismissed.';
    if (entry.dismissUnconfirmed) return 'Dismissing has not been confirmed. Check the question status before sending.';
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
    entry.answering = true; entry.status = 'Your answer is being sent.'; entry.tone = ''; changed(key);
    return window.dinahUIState.flush().then(function () {
      var q = entry.question;
      return q.answerType ? app().AnswerTaskQuestion(key, JSON.stringify(q.answerType === 'text' ? { type: 'text', text: entry.text } : { type: 'multiple', optionIds: q.options.filter(function (o) { return entry.optionIds.includes(o.optionId); }).map(function (o) { return o.optionId; }) })) : app().Answer(key, entry.choice);
    }).then(function () { finish({ key: key, disposition: 'answered' }); }).catch(function (error) {
      if (entries.get(key) !== entry) return;
      entry.answering = false; entry.uncertain = true;
      entry.status = 'Your answer could not be confirmed. ' + window.dinahHelpers.message(error, 'sentence'); changed(key);
      return refresh().catch(function () { if (entries.get(key) === entry) { entry.status = 'Your answer could not be confirmed. Check the question status before sending again.'; changed(key); } });
    });
  }
  function paint(card, entry) {
    if (!entry) { card.hidden = true; return; }
    var q = entry.question;
    // An answer cannot change while it, or a dismissal of its question, is on its way or unconfirmed.
    var held = entry.answering || entry.uncertain || entry.dismissing || entry.dismissUnconfirmed;
    card.querySelectorAll('input').forEach(function (input) { input.disabled = held; input.checked = q.answerType === 'multiple' ? entry.optionIds.includes(input.value) : input.value === entry.choice; });
    var text = card.querySelector('textarea');
    if (text) { if (text.value !== entry.text) text.value = entry.text; text.disabled = held; }
    var reason = validation(entry), send = card.querySelector('[data-task-answer]'), check = card.querySelector('[data-task-check]');
    if (send) { send.disabled = !!reason; send.title = reason; }
    if (check) check.disabled = entry.answering || entry.dismissing;
    var count = card.querySelector('[data-task-count]');
    if (count) {
      var n = q.answerType === 'text' ? Array.from(entry.text).length : entry.optionIds.length;
      var over = q.answerType === 'text' ? n > q.maxLength : n > q.maxSelections;
      count.textContent = (over ? '▲ ' : '') + (q.answerType === 'text' ? n + ' of ' + q.maxLength + ' characters (minimum ' + q.minLength + ')' : n + ' selected; choose ' + q.minSelections + ' to ' + q.maxSelections);
      count.dataset.tone = over ? 'danger' : ''; if (text) text.setAttribute('aria-invalid', String(over));
    }
    var status = card.querySelector('[data-task-status]'); if (status) { status.textContent = entry.status || entry.draftError || ''; status.dataset.tone = entry.status && entry.tone === 'danger' ? 'danger' : ''; }
    // The card's own Dismiss Question keeps its place in every state. When it cannot be used it is disabled, and the line under the buttons says why.
    var dismissButton = card.querySelector('[data-task-dismiss]');
    if (dismissButton) { var can = dismissal(entry); dismissButton.disabled = !can.enabled; dismissButton.title = can.reason; card.querySelector('[data-task-dismiss-reason]').textContent = can.reason; }
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
      radio.addEventListener('change', function () { if (entries.get(key) !== entry || entry.answering || entry.uncertain || entry.dismissing || entry.dismissUnconfirmed) return; if (q.answerType === 'multiple') entry.optionIds = radio.checked ? entry.optionIds.concat(option.optionId) : entry.optionIds.filter(function (id) { return id !== option.optionId; }); else entry.choice = option.optionId; save(key); });
      label.append(radio, document.createTextNode(option.name)); choices.appendChild(label);
    }); content.appendChild(choices);
    if (q.answerType) { var count = node('p', 'decisions-count'); count.dataset.taskCount = ''; count.setAttribute('aria-live', 'polite'); content.appendChild(count); }
    var status = node('p', 'task-question-result'); status.dataset.taskStatus = ''; status.setAttribute('role', 'status'); status.setAttribute('aria-live', 'polite');
    var actions = node('div', 'notice-actions hstack gap-2'), send = node('button', 'btn btn-sm btn-primary', 'Send Answer'), check = node('button', 'btn btn-sm', 'Check Question Status');
    send.type = check.type = 'button'; send.dataset.taskAnswer = ''; check.dataset.taskCheck = '';
    send.addEventListener('click', function () { answer(key); });
    check.addEventListener('click', function () { refresh().catch(function (error) { if (entries.get(key) === entry) { entry.status = window.dinahHelpers.message(error, 'sentence'); changed(key); } }); });
    actions.append(send, check);
    if (!settings.embedded) {
      // A card in Notifications dismisses its question itself, with the confirmation Decisions asks. The reason
      // line is always there with one height, so a reason that appears or goes moves nothing.
      var dismissButton = node('button', 'btn btn-sm', 'Dismiss Question…'), reason = node('p', 'text-muted task-question-dismiss-reason');
      dismissButton.type = 'button'; dismissButton.dataset.taskDismiss = ''; reason.dataset.taskDismissReason = '';
      reason.id = 'task-dismiss-reason-' + cards; dismissButton.setAttribute('aria-describedby', reason.id);
      dismissButton.addEventListener('click', function () { Promise.resolve().then(function () { return dismiss(key); }).catch(window.dinahHost.fail); });
      actions.prepend(dismissButton); content.append(status, actions, reason);
    }
    card.appendChild(content); paint(card, entry);
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
    // dismissal says whether the person can dismiss the question now and the reason when not; dismiss asks first.
    // ending says how a finished question ended ('' when the host has not said) and what the question was.
    dismiss: dismiss, dismissal: function (key) { return dismissal(entries.get(key)); }, ending: function (key) { return ended.get(key) || null; },
    open: function (key) { var entry = entries.get(key); if (!entry || !entry.question.target.href) return; var query = new URLSearchParams(entry.question.target.href.slice(1)), page = query.get('open'); return window.dinahWorkbench.open(page, query.get('view.' + page) || ''); }
  };
}());
