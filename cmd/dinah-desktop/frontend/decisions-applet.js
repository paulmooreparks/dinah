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
  function dismisses(op) { return !!op && (op.method === 'DismissRequest' || op.method === 'DismissDecision'); }
  // DismissRequest and RequestOperation give the settlement receipt {operationId,id,revision,state,outcome,settledAt,code,text}
  // (docs/design/request-settlement.md). It is kept in the shape of a decision receipt, so that a saved operation
  // reads the same whichever method sent it. A receipt that is neither a dismissal nor a refusal is not a result.
  function settlementReceipt(receipt, op) {
    if (!receipt || receipt.operationId !== op.input.operationId || receipt.id !== op.input.id || !Number.isSafeInteger(receipt.revision) || receipt.revision < 0) return null;
    var kept = { operationId: receipt.operationId, id: receipt.id, revision: receipt.revision, messageIds: [] };
    if (receipt.state === 'committed' && receipt.outcome === 'dismissed' && Number.isFinite(Date.parse(receipt.settledAt))) return Object.assign(kept, { state: 'committed', closed: true, reason: null });
    if (receipt.state === 'refused' && typeof receipt.code === 'string' && receipt.code && typeof receipt.text === 'string' && receipt.text) return Object.assign(kept, { state: 'rejected', closed: receipt.code === 'request_closed', reason: { code: receipt.code, message: receipt.text } });
    return null;
  }
  function workingOp(value) { return value && value.operation && !['committed', 'rejected'].includes(value.operation.status); }
  function unconfirmed(op, still) { return (dismisses(op) ? 'Dismissing has ' : 'Sending has ') + (still ? 'still ' : '') + 'not been confirmed. '; }
  // The sentences the window and a notification's card both say about a dismissal.
  var noDismissBuild = 'This build cannot dismiss requests.', resultUnchecked = 'The last send or dismissal has not been confirmed. Check its result first.';
  var badDismissResult = 'The dismissal result could not be confirmed.', notWaiting = 'This request is no longer waiting.', draftsUnread = 'Saved drafts could not be read. Sending is disabled.';
  // What the host and the request's saved draft allow, for the window and for a notification's card. Each adds
  // what it is itself in the middle of. saved reads the draft, and is asked only when the host allows the dismissal.
  function requestDismissal(item, saved) {
    if (!app().DismissRequest || !app().RequestOperation) return { enabled: false, reason: noDismissBuild };
    if (!item.canDismiss) return { enabled: false, reason: 'This request cannot be dismissed.' };
    var draft;
    try { draft = saved(); } catch (error) { return { enabled: false, reason: errorText(error) }; }
    if (workingOp(draft)) return { enabled: false, reason: resultUnchecked };
    return { enabled: true, reason: '' };
  }
  // How a request that left the open requests ended, from its record: its chat withdrew it, or the person dismissed it.
  // Nothing is said of a request that was answered, or whose record does not say.
  function endingWords(item) {
    if (item.status === 'withdrawn') return { text: item.asker + ' withdrew its request, so it no longer needs an answer. Anything you had typed for it is gone.', link: 'Show What Was Withdrawn' };
    if (item.status === 'dismissed') return { text: 'You dismissed the request from ' + item.asker + '. No answer was sent.', link: 'Show the Request', glyph: 'check' };
    return null;
  }
  // Dismiss asks first and says who is told. The wording is the approved mockup's, except where the host does
  // otherwise: its reason for an answer that cannot be delivered also covers a chat that only left its group,
  // and such a chat is still told, so the dialog says nobody can be told only of a chat that is gone.
  function askDismiss(item) {
    var partly = item.questions.some(function (q) { return q.answered; });
    var told = item.dismissalReason ? item.dismissalReason + ' If the chat itself is gone, nobody can be told.' : item.asker + ' is told you dismissed it, so it does not wait for you.';
    var body = (partly ? 'No more answers are sent, and it leaves Decisions. The answers you already sent are kept. ' : 'No answer is sent, and it leaves Decisions. ') + told + ' Use this when you have dealt with it elsewhere, or it can no longer be answered.';
    // The last argument makes the committing button the danger button, with Cancel first and focused and no primary button, as PUDL asks of a destructive action.
    return window.dinahHost.ask('Dismiss This Request?', body, 'Dismiss', true);
  }
  // What is said of a dismissal that has been saved and has no result yet.
  function dismissProgress(op, isChecking, isSending) {
    return isChecking ? 'Checking whether this request was dismissed.' : op.status === 'prepared' || isSending ? 'Dismissing this request.' : 'Dismissing has not been confirmed. Check the result before trying again.';
  }

  /* ---- Dismiss Request on a notification's card -----------------------------
     A card in Notifications dismisses the request behind it without opening this window. It asks with the
     window's dialog, and the dismissal is the request's saved operation, the one the window keeps in the
     request's draft (decision-drafts.js). A result that is lost is therefore looked up under its own operation,
     from the card or from the window, and is never sent under a second identity. cards maps a request to what
     its card is in the middle of and to the last thing it has to say. */
  var cards = new Map(), cardsChecked = new Set();
  // The operations being sent or checked now, by the window or by a card, so that neither sends what the other has on its way.
  var pending = new Set();
  // about is the operation an unconfirmed result's words are about. Once that operation has a result, from the window's check for one, the words go.
  function cardState(id) { var s = cards.get(id); if (!s) { s = { busy: '', status: '', tone: '', about: '' }; cards.set(id, s); } return s; }
  // The cards hear through the document that a request's saved operation changed, or that the request is closed.
  // said is what a card that goes for another reason than its own dismissal leaves behind.
  function cardsTold(id, closed, said) { document.dispatchEvent(new CustomEvent('dinah:request-dismissal', { detail: { id: id, closed: !!closed, said: said || '' } })); }
  // A change made from a card reaches the cards and the open window.
  function requestChanged(id, closed, said) {
    cardsTold(id, closed, said);
    if (page) page.operationChanged(id, !!closed);
  }
  function savedDraft(id) { return window.dinahDecisionDrafts.read('request:' + id); }
  // The card changes only the draft's operation, and the request it belongs to, so an answer being written in the window is never replaced.
  function saveOperation(id, op, item) {
    var draft = savedDraft(id); draft.operation = op; if (item) draft.request = item;
    return window.dinahDecisionDrafts.write('request:' + id, draft);
  }
  // What a card shows: whether Dismiss Request can be used and the reason when not, the line under its buttons,
  // and whether a dismissal sent from a card or from the window still waits to have its result checked.
  function cardDismissal(id) {
    var s = cardState(id), out = { enabled: false, reason: '', status: s.status, tone: s.tone, check: false };
    if (!app() || !app().DismissRequest || !app().RequestOperation || !app().DecisionByID || !app().RequestState) { out.reason = noDismissBuild; return out; }
    // A dismissal is saved before it is sent, which a saved state that could not be read does not allow.
    if (window.dinahUIState.initializationError()) { out.reason = draftsUnread; return out; }
    var saved;
    try { saved = savedDraft(id); } catch (error) { out.reason = errorText(error); return out; }
    if (s.about && !s.busy && !(workingOp(saved) && saved.operation.input.operationId === s.about)) { s.about = ''; s.status = out.status = ''; s.tone = out.tone = ''; }
    if (!workingOp(saved)) { out.enabled = true; return out; }
    out.reason = resultUnchecked;
    var op = saved.operation;
    // While the window or a card is sending or checking the operation there is nothing to press.
    var sending = pending.has(op.input.operationId);
    if (op.method === 'DismissRequest') { out.check = !s.busy && !sending; if (!s.status) out.status = dismissProgress(op, s.busy === 'checking', sending); }
    return out;
  }
  // A card drawn for a request whose saved dismissal has no result, as after the window was closed with one on its
  // way, checks that operation once, as the window does when it opens such a request.
  function cardShown(id) {
    if (cardsChecked.has(id)) return;
    cardsChecked.add(id);
    var state = cardDismissal(id);
    if (state.check) checkFromCard(id);
  }
  function cardFailed(id, text, about) { var s = cardState(id); s.busy = ''; s.status = text; s.tone = 'danger'; s.about = about || ''; requestChanged(id); }
  function dismissFromCard(id) {
    var s = cardState(id);
    if (s.busy) return Promise.resolve();
    if (!cardDismissal(id).enabled) { requestChanged(id); return Promise.resolve(); }
    s.busy = 'reading';
    // The request is read now, so that the dialog describes it as it is and the dismissal is sent with its current
    // revision. DecisionByID gives the request as a read-only record, with no commands, so whether it may be
    // dismissed now is read from RequestState, the host's word on what may be done with a request; the window has
    // the same from the open list. A real run found the card refusing every request when it read DecisionByID alone.
    return Promise.resolve().then(function () { return Promise.all([app().DecisionByID(id), app().RequestState(id)]); }).then(function (values) {
      var value = values[0], state = values[1];
      if (!value || typeof value.closed !== 'boolean' || !validItem(value.decision) || value.decision.id !== id) throw new Error('The request could not be read.');
      // A request that ended another way has no open request behind its card any more, and the card goes with a word on how it ended.
      if (value.closed) { cards.delete(id); requestChanged(id, true, (endingWords(value.decision) || { text: notWaiting }).text); return; }
      var command = function (name) { return state && Array.isArray(state.commands) ? state.commands.find(function (c) { return c && c.name === name; }) : null; };
      var dismiss = command('Dismiss'), answer = command('Answer');
      if (!state || state.id !== id || !Number.isSafeInteger(state.revision) || !dismiss || typeof dismiss.enabled !== 'boolean') throw new Error('The request could not be read.');
      if (state.revision !== value.decision.revision || state.status !== 'open') throw new Error('The request changed while it was read. Try again.');
      // The host disables Answer with the reason an answer cannot be delivered (request_state.go), which the dialog says.
      var item = Object.assign({}, value.decision, { canDismiss: dismiss.enabled, dismissalReason: answer && answer.enabled === false && answer.reason || '' });
      if (!dismiss.enabled) { cardFailed(id, dismiss.reason || 'This request cannot be dismissed.'); return; }
      var can = requestDismissal(item, function () { return savedDraft(id); });
      if (!can.enabled) { cardFailed(id, can.reason); return; }
      s.busy = 'asking';
      return askDismiss(item).then(function (yes) {
        if (!yes) { s.busy = ''; return; }
        // The window may have sent something for this request while the dialog was open.
        if (workingOp(savedDraft(id))) { s.busy = ''; requestChanged(id); return; }
        var op = { method: 'DismissRequest', input: { operationId: operationId(), id: item.id, revision: item.revision }, status: 'prepared', destination: item.destination };
        s.busy = 'sending'; s.status = ''; s.tone = '';
        // The operation is saved before it is sent, so that a result lost with the window can still be checked.
        return saveOperation(id, op, item).then(function () { requestChanged(id); return sendFromCard(id, op); }, function (error) {
          // It was never sent, so it is taken back.
          try { var draft = savedDraft(id); draft.operation = null; window.dinahDecisionDrafts.write('request:' + id, draft).catch(function () {}); } catch (unreadable) { /* The draft's own failure is the card's reason from now on. */ }
          cardFailed(id, 'This request was not dismissed. ' + errorText(error));
        });
      });
    }).catch(function (error) { cardFailed(id, 'This request was not dismissed. ' + errorText(error)); });
  }
  function sendFromCard(id, op) {
    var s = cardState(id);
    s.busy = 'sending'; s.status = ''; s.tone = ''; s.about = ''; op.status = 'submitted'; pending.add(op.input.operationId);
    return saveOperation(id, op).then(function () { requestChanged(id); return app().DismissRequest(op.input); }).then(function (receipt) { return settleFromCard(id, op, receipt); }).catch(function (error) {
      // Whether the host dismissed the request is not known. The saved operation stays, and its result is checked under the same identity.
      pending.delete(op.input.operationId); cardFailed(id, unconfirmed(op) + errorText(error), op.input.operationId);
    });
  }
  // A lost result is looked up by its operation. The host has no result for an operation it never finished, and
  // then the same operation is sent again, which cannot dismiss the request twice.
  function checkFromCard(id) {
    var s = cardState(id), saved;
    if (s.busy) return Promise.resolve();
    try { saved = savedDraft(id); } catch (error) { requestChanged(id); return Promise.resolve(); }
    var op = saved.operation;
    // An operation the window is sending or checking now is left to the window.
    if (!workingOp(saved) || op.method !== 'DismissRequest' || pending.has(op.input.operationId)) { requestChanged(id); return Promise.resolve(); }
    s.busy = 'checking'; s.status = ''; s.tone = ''; s.about = ''; pending.add(op.input.operationId); requestChanged(id);
    return Promise.resolve().then(function () { return app().RequestOperation(op.input.id, op.input.operationId); }).then(function (receipt) {
      if (receipt && receipt.operationId === op.input.operationId && receipt.state === 'unknown') return sendFromCard(id, op);
      return settleFromCard(id, op, receipt);
    }).catch(function (error) { pending.delete(op.input.operationId); cardFailed(id, unconfirmed(op, true) + errorText(error), op.input.operationId); });
  }
  function settleFromCard(id, op, receipt) {
    var kept = settlementReceipt(receipt, { input: op.input });
    if (!kept) throw new Error(badDismissResult);
    op.status = kept.state; op.receipt = kept; pending.delete(op.input.operationId);
    var s = cardState(id), closed = kept.state === 'committed' || kept.closed;
    s.busy = '';
    // A refusal leaves the request open, and the card says the host's reason. A dismissal ends the card's part, and so
    // does a request the host says is already closed, of which the host's reason is said as the card goes.
    if (closed) cards.delete(id); else { s.status = kept.reason.message; s.tone = 'danger'; }
    // The host's result stands whether or not it could be kept on this computer.
    return Promise.resolve().then(function () { return saveOperation(id, op); }).catch(function () {}).then(function () { requestChanged(id, closed, closed && kept.state === 'rejected' ? kept.reason.message : ''); });
  }
  function init(root, opts) {
    var disposed = false, ready = false, readNumber = 0, refreshing = false, refreshAgain = false, moreBefore = 0, loadedPages = 1, readPromise = null, readValid = false;
    var items = new Map(), order = [], selected = '', selectedCopy = null, filter = '', view = 'requests';
    var pane = 'request', lastFinished = null, quoteWatch = null;
    var rendered = '', draft = null, draftKey = '', saving = 0, draftError = '', checking = new Set();
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
    // Whether the person can dismiss an entry now, and the reason when not. The host allows it: canDismiss on a
    // request, the Dismiss command on a task question. The page adds only what it is itself in the middle of.
    function dismissal(item) {
      if (!item || !items.has(item.id)) return { enabled: false, reason: 'This request is not in the list of open requests.' };
      if (item.kind === 'task') return window.dinahTaskQuestions.dismissal(item.taskQuestion.key);
      return requestDismissal(item, function () { return draftKey === 'request:' + item.id ? draft : draftFor('request:' + item.id); });
    }
    // The line the last finished entry left behind, with the way to its record. A task question has no record.
    function finishedLine(line) {
      line.textContent = lastFinished.text;
      var finished = lastFinished;
      if (!finished.id.startsWith('task:')) line.appendChild(button(finished.link || 'Show Answers', function () { navigate('record', finished.id); }));
    }
    function controls() {
      if (disposed || !form) return;
      var dismissButton = form.querySelector('[data-dismiss-decision]');
      if (dismissButton) {
        // Dismiss keeps its place and size in every state. When it cannot be used it is disabled, and the line under the buttons says why.
        var can = dismissal(current());
        dismissButton.disabled = !can.enabled; dismissButton.title = can.reason; form.querySelector('.decisions-dismiss-reason').textContent = can.reason;
      }
      if (view === 'requests' && current() && current().kind === 'task') {
        var key = current().taskQuestion.key, entry = window.dinahTaskQuestions.get(key), why = window.dinahTaskQuestions.validation(key);
        send.disabled = !!why; send.title = why;
        receiptLine.textContent = entry && entry.status || why || 'Your answer is ready to send.';
        if (lastFinished && why && !(entry && (entry.status || entry.draftError || entry.answering || entry.dismissing))) finishedLine(receiptLine);
        receiptLine.classList.toggle('danger', !!(entry && entry.status && entry.tone === 'danger'));
        return;
      }
      if (!draft) return;
      var item = current(), busy = !!workingOp(draft), broadcast = view !== 'requests';
      var blocked = busy || !ready;
      // dismissalReason is the host's word that an answer can no longer be delivered. canDismiss is true for every open request and says nothing about answering.
      var undeliverable = !!(item && item.dismissalReason);
      form.querySelectorAll('textarea,input,select,[data-edit-answer]').forEach(function (el) { el.disabled = blocked || !broadcast && (undeliverable || !items.has(selected) || !!(draft.operation && draft.operation.status === 'committed' && draft.operation.receipt && draft.operation.receipt.closed)); });
      if (send && !broadcast) {
        var fields = item && item.kind === 'mention' ? [{ number: 0, answered: false }] : item ? item.questions : [];
        var any = fields.some(function (q) { var a = draft.answers[q.number]; return !q.answered && a && (a.asRecommended || a.text.trim()); });
        send.disabled = blocked || !!draftError || !item || !items.has(selected) || undeliverable || draft.reviewed !== signature(item) || !any;
      }
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
        // The notice above the questions gives the host's reason. A dismissal's own result, once there is one, is said here in place of this line.
        if (undeliverable && !op) receiptLine.textContent = 'An answer cannot be delivered. Dismissing closes the request.';
        if (changedLine && changedLine.textContent && !(dismisses(op) && op.status === 'rejected')) receiptLine.textContent = changedLine.textContent;
        if (status.textContent) receiptLine.textContent = status.textContent;
        else if (!broadcast && lastFinished && !draft.operation && send && send.disabled && !undeliverable) finishedLine(receiptLine);
        if (draftError) receiptLine.textContent = draftError;
        receiptLine.classList.toggle('danger', !!draftError || !!op && op.status === 'rejected');
        detail.querySelectorAll('[data-operation]').forEach(function (el) { el.hidden = !workingOp(draft); el.disabled = !workingOp(draft) || pending.has(op.input.operationId); if (workingOp(draft)) el.textContent = dismisses(op) ? 'Check Dismiss Result' : 'Check Send Result'; });
      }
    }
    function deliveryStatus() {
      if (draftError) return draftError;
      var op = draft.operation;
      if (!op) return send && send.disabled ? 'Choose or write an answer to send it.' : 'Your answer is ready to send.';
      if (op.status === 'rejected') return op.receipt.reason.message;
      if (op.status !== 'committed' && dismisses(op)) return dismissProgress(op, checking.has(op.input.operationId), pending.has(op.input.operationId));
      if (op.status !== 'committed') return 'Sending has not been confirmed. Check the result before sending again.';
      if (dismisses(op)) return 'This request was dismissed. No answer was sent.';
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
      var task = item.kind === 'task', q = item.taskQuestion, can = dismissal(item);
      // The commands are the same for every entry, in the approved mockup's groups. One that cannot apply is disabled with its reason.
      var rows = [
        { label: 'Open', run: function () { navigate('requests', item.id); } },
        '-',
        { label: 'Open Chat', disabled: task || !!item.dismissalReason, reason: task ? 'A task question comes from a service, not a chat.' : item.dismissalReason, run: function () { return openChat(item); } },
        { label: 'Open Group', disabled: task || !item.group, reason: task ? 'This task question has no group.' : 'This request was asked directly.', run: function () { return window.dinahBus.openGroup(item.group); } },
        { label: 'Open Task', disabled: !task || !q.target.href, reason: task ? q.target.reason : 'This request comes from a chat, not a task.', run: function () { return window.dinahTaskQuestions.open(q.key); } },
        '-',
        { label: 'Dismiss ' + (task ? 'Question' : item.kind === 'mention' ? 'Message' : 'Request') + '…', danger: true, disabled: !can.enabled, reason: can.reason, run: function () { Promise.resolve().then(function () { return dismiss(item.id); }).catch(window.dinahHost.fail); } }
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
      var dismissButton = button('Dismiss', function () { return dismiss(item.id); }); dismissButton.dataset.dismissDecision = '';
      send = button('Send', item.kind === 'task' ? function () { return window.dinahTaskQuestions.answer(item.taskQuestion.key); } : answer, true);
      // The line under the buttons says why Dismiss cannot be used. It is always there with one height, so a reason that appears or goes moves nothing.
      var slot = node('span', 'decisions-dismiss'), reason = node('small', 'text-muted decisions-dismiss-reason'); reason.id = 'dismiss-reason-' + item.id.replace(/[^a-zA-Z0-9]/g, '-'); dismissButton.setAttribute('aria-describedby', reason.id); slot.append(dismissButton); actions.append(slot, send, reason); if (item.kind !== 'task') actions.appendChild(operationActions());
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
    // Dismiss asks first and says who is told (askDismiss, which a notification's card uses as well).
    function dismiss(id) {
      var item = items.get(id);
      if (!item || !dismissal(item).enabled) return;
      // A question dismissed from its row is opened once the person has confirmed, so that its result is said in its own foot.
      if (item.kind === 'task') return window.dinahTaskQuestions.dismiss(item.taskQuestion.key, function () { if (!disposed && items.has(id) && (view !== 'requests' || selected !== id)) { selectedCopy = items.get(id); navigate('requests', id); } });
      return askDismiss(item).then(function (yes) {
        var now = items.get(id);
        if (!yes || disposed || !now || !dismissal(now).enabled) return;
        // The dismissal is the open request's saved operation, so a request dismissed from its row is opened first.
        if (view !== 'requests' || selected !== id) { selectedCopy = now; navigate('requests', id); }
        if (!form || draftKey !== 'request:' + id || !draft || !draft.request || draft.request.id !== id || workingOp(draft)) return;
        return begin('DismissRequest', { operationId: operationId(), id: now.id, revision: now.revision });
      });
    }
    function begin(method, input) {
      var key = draftKey, value = draft;
      draft.operation = { method: method, input: input, status: 'prepared' };
      if (method !== 'ConfirmDecisionBroadcast') draft.operation.destination = current().destination;
      return persist().then(function () { tellCard(key); return execute(key, value); }).catch(function (error) { say(errorText(error)); });
    }
    // A notification's card shows whether its request has a send or a dismissal without a result, so it hears when one begins or ends here.
    // closed is true when the host's result closed the request, and then its notifications go as well.
    function tellCard(key, closed) { if (key.startsWith('request:')) cardsTold(key.slice(8), closed); }
    function accept(key, value, receipt) {
      var op = value.operation;
      if (op.method === 'DismissRequest') receipt = settlementReceipt(receipt, op);
      if (!validReceipt(receipt, op)) throw new Error(dismisses(op) ? badDismissResult : 'The send result could not be confirmed.');
      var item = items.get(receipt.id) || selectedCopy;
      if (item && item.id !== receipt.id) item = null;
      var destination = op.destination || (item ? item.destination : 'the asking chat');
      op.status = receipt.state; op.receipt = receipt;
      if (op.method === 'ConfirmDecisionBroadcast') value.preview = null;
      if (receipt.state === 'committed' && op.method === 'AnswerDecision') op.input.answers.forEach(function (answer) { delete value.answers[answer.number]; });
      // A dismissal keeps the answers a partly answered request already had, and the line says so.
      var finished = !dismisses(op) ? { id: receipt.id, text: 'Your reply was sent to ' + destination + '.', glyph: 'check' } :
        { id: receipt.id, text: 'You dismissed the request' + (item ? ' from ' + item.asker : '') + '. ' + (item && item.questions.some(function (q) { return q.answered; }) ? 'The answers already sent are kept.' : 'No answer was sent.'), link: 'Show the Request', glyph: 'check' };
      return store(key, value).then(function () { if (key === draftKey) { draft = value; if (view !== 'requests') rendered = ''; controls(); } if (receipt.state === 'committed' && receipt.closed && selected === receipt.id) { lastFinished = finished; selected = ''; selectedCopy = null; rendered = ''; } return load(false); });
    }
    function execute(key, value) {
      var op = value.operation;
      if (pending.has(op.input.operationId)) return Promise.resolve();
      pending.add(op.input.operationId); op.status = 'submitted'; controls();
      return store(key, value).then(function () { return app()[op.method](op.input); }).then(function (receipt) { return accept(key, value, receipt); }).catch(function (error) {
        if (!['committed', 'rejected'].includes(op.status)) { op.status = 'submitted'; return store(key, value).then(function () { say(unconfirmed(op) + errorText(error)); }); }
        say('The result was confirmed, but could not be saved on this computer. ' + errorText(error));
      }).finally(function () { pending.delete(op.input.operationId); controls(); tellCard(key, op.status === 'committed' && !!op.receipt && op.receipt.closed); });
    }
    function reconcile() {
      if (!draft || !workingOp(draft)) return Promise.resolve();
      var key = draftKey, value = draft, op = value.operation;
      if (pending.has(op.input.operationId)) return Promise.resolve();
      pending.add(op.input.operationId); checking.add(op.input.operationId); controls();
      // A dismissal sent through the settlement host is looked up there, by its request and its operation. The
      // host has no result for an operation it never finished, and then the same operation is sent again.
      var settlement = op.method === 'DismissRequest';
      return Promise.resolve().then(function () { return settlement ? app().RequestOperation(op.input.id, op.input.operationId) : app().DecisionOperation(op.input.operationId); }).then(function (receipt) {
        var unknown = receipt && receipt.operationId === op.input.operationId && receipt.state === 'unknown' && (settlement || receipt.id === '' && receipt.revision === 0 && receipt.closed === false && Array.isArray(receipt.messageIds) && receipt.messageIds.length === 0 && receipt.reason === null);
        if (unknown) { op.status = 'unknown'; pending.delete(op.input.operationId); checking.delete(op.input.operationId); return execute(key, value); }
        return accept(key, value, receipt);
      }).catch(function (error) { say(unconfirmed(op, true) + errorText(error)); }).finally(function () { pending.delete(op.input.operationId); checking.delete(op.input.operationId); if (key === draftKey) controls(); tellCard(key, op.status === 'committed' && !!op.receipt && op.receipt.closed); });
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
      if (!items.size && view === 'requests') { var explanation = node('div', 'empty-state'); explanation.append(node('p', 'empty-state-title', status.textContent || 'There are no open requests.'), node('p', 'empty-state-body', 'When a chat or task asks you a question, the request appears here.')); empty.replaceChildren(explanation); if (lastFinished) { var done = node('p', 'decisions-done'), mark = node('span', 'glyph'); mark.setAttribute('aria-hidden', 'true'); mark.dataset.glyph = lastFinished.glyph || 'info'; mark.style.setProperty('--glyph', 'var(--glyph-' + mark.dataset.glyph + ')'); finishedLine(done); done.prepend(mark); empty.appendChild(done); } }
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
    // A request that left the list without an action in this window was settled somewhere else. Its record says
    // whether its chat withdrew it or the person dismissed it, and the line is corrected when that is known.
    function readEnding(finished) {
      if (!app().DecisionByID) return;
      Promise.resolve().then(function () { return app().DecisionByID(finished.id); }).then(function (value) {
        var item = value && value.decision;
        if (disposed || lastFinished !== finished || !item || item.id !== finished.id || typeof item.asker !== 'string') return;
        var ending = endingWords(item);
        if (!ending) return;
        Object.assign(finished, ending);
        draw();
      }).catch(function () { /* The line already says that the request is no longer waiting. */ });
    }
    // How a task question ended, in the approved mockup's words. The host's event or record says how; when
    // neither has, the line says only that the question is no longer waiting.
    function taskFinished(key) {
      var ending = window.dinahTaskQuestions.ending(key), q = ending && ending.question, how = q ? ending.disposition : '';
      return { id: 'task:' + key, glyph: how === 'dismissed' || how === 'answered' ? 'check' : 'info', text:
        how === 'dismissed' ? 'You dismissed the question from ' + q.serviceName + '. No answer was sent.' :
        how === 'withdrawn' ? q.serviceName + ' withdrew its question, so it no longer needs an answer. Anything you had typed for it is gone.' :
        how === 'answered' ? 'Your answer was sent to ' + q.serviceName + ', which continues the task ' + q.taskTitle + '.' :
        how === 'cancelled' ? 'The task ' + q.taskTitle + ' was cancelled, so its question closed. No answer was sent.' : 'This task question is no longer waiting.' };
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
        order = order.filter(function (id) { return items.has(id); }); syncTasks(); if (view === 'requests' && previouslyOpen && selected && !items.has(selected) && !workingOp(draft)) { lastFinished = { id: selected, text: notWaiting }; readEnding(lastFinished); selected = ''; selectedCopy = null; rendered = ''; } if (lastFinished && !selected && order.length) { selected = order[0]; opts.changed(address()); } moreBefore = result.nextBefore; loadedPages = append ? loadedPages + pages : pages;
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
      if (selected === id && !window.dinahTaskQuestions.get(key)) { lastFinished = taskFinished(key); selected = ''; selectedCopy = null; rendered = ''; syncTasks(); if (order.length) selected = order[0]; opts.changed(address()); }
      // How the question ended can be read after it has left the list, and then the line it left is said again.
      else if (lastFinished && lastFinished.id === id && !window.dinahTaskQuestions.get(key)) lastFinished = taskFinished(key);
      draw();
    });
    var unresync = window.dinahHost.onResync(function () { return load(false); });
    window.dinahUIState.ready.then(function () {
      if (disposed) return;
      if (window.dinahUIState.initializationError()) { say(draftsUnread); return; }
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
    // A notification's card changed this request's saved operation, or its request closed. The open request takes
    // the operation from the saved draft and keeps everything else it has, and a closed request is read out of the list.
    function operationChanged(id, closed) {
      if (disposed || !ready) return;
      if (draft && draftKey === 'request:' + id) { try { draft.operation = draftFor(draftKey).operation; } catch (error) { draftError = errorText(error); } controls(); }
      if (closed) load(false);
    }
    return { ensureRequest: ensureRequest, operationChanged: operationChanged, state: address, setState: function (state) { stateRead(state); draw(); }, destroy: function () { disposed = true; if (quoteWatch) quoteWatch.disconnect(); unsubscribe(); unresync(); unTask(); },
      // The window adds no menu: View, Refresh reads the requests again,
      // and the Broadcast rows are in Chat (docs/design/menus.md).
      canReload: available, reload: function () { if (view === 'record') { rendered = ''; draw(); } return load(false); } };
  }
  window.dinahWorkbench.register('decisions', { applet: { id: 'dinah.decisions', title: 'Decisions', glyph: 'comments', workspace: false }, singleton: true, title: 'Decisions', glyph: 'comments', mode: 'maximized', init: function (root, opts) { var instance = init(root, opts); page = instance; var destroy = instance.destroy; instance.destroy = function () { destroy(); if (page === instance) page = null; }; return instance; },
    launcher: { title: 'Decisions', run: function () { return window.dinahWorkbench.open('decisions'); } } });
  window.dinahDecisions = { openRequest: function (id) { return window.dinahWorkbench.open('decisions', new URLSearchParams({ request: id }).toString()).then(function () { if (!page) throw new Error('Decisions could not be opened.'); return page.ensureRequest(id); }); },
    // For the card of a notification that belongs to an open request (notifications-applet.js). dismissal says what the
    // card shows, dismissRequest asks and then dismisses, checkDismissal looks up a result that was lost, and shown is
    // called when a card is drawn. Each change is announced on the document as dinah:request-dismissal.
    dismissal: cardDismissal, dismissRequest: dismissFromCard, checkDismissal: checkFromCard, shown: cardShown };
  function open(state) { return window.dinahWorkbench.open('decisions', state || '').catch(window.dinahHost.fail); }
  // Decisions is a tool, opened from the launcher, and Broadcast is a chat command drawn in the Chats pane's Actions; none of the three is in View since 19 November 2026 (docs/design/menus.md).
  window.dinahCommands.register({ id: 'decisions', menu: 'tools', mnemonic: 'D', label: 'Decisions', visible: available, run: function () { return open(); } });
  ['active', 'all'].forEach(function (scope) { window.dinahCommands.register({ id: 'broadcast-' + scope, menu: 'chat', after: 'chat-sep-broadcast', mnemonic: scope === 'active' ? 'V' : 'B', label: scope === 'active' ? 'Broadcast to Active…' : 'Broadcast to All…', visible: available,
    enabled: function () { return !!(app().PreviewDecisionBroadcast && app().ConfirmDecisionBroadcast); }, run: function () { return open('view=broadcast-' + scope); } }); });
}());
