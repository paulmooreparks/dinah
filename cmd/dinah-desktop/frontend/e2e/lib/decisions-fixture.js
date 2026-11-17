const { launch } = require('./page');
// The settlement host makes canDismiss true for every open request (internal/bus/decisions.go). A request whose
// answer can no longer be delivered is one with a dismissalReason.
const makeRequest = (id, extra = {}) => Object.assign({ id, canDismiss: true, dismissalReason: '', revision: 1, messageId: Number(id.replace(/\D/g, '')) || 1, kind: 'request', createdAt: '2026-11-14T00:00:00Z', askerSession: 'session-' + id, asker: 'Review ' + id, workspace: 'C:/Fixture', group: '#Review', groupTitle: 'Review Group', destination: 'Review Group, replying to Review ' + id, text: 'Please decide.', questions: [
  { number: 1, text: 'Which release should we use?', recommendation: 'Use the reviewed release.', choices: ['Reviewed', 'Wait'], answered: false, answer: '' },
  { number: 2, text: 'When should we proceed?', recommendation: 'Proceed tomorrow.', choices: [], answered: false, answer: '' }
]}, extra);
async function setup(items, extra = {}) {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, args: { items, ...extra }, app: args => {
    const copy = value => JSON.parse(JSON.stringify(value));
    if (!sessionStorage.getItem('decision-items')) sessionStorage.setItem('decision-items', JSON.stringify(args.items));
    window.__decisionItems = JSON.parse(sessionStorage.getItem('decision-items'));
    window.__operations = JSON.parse(sessionStorage.getItem('decision-operations') || '{}');
    window.__previews = {}; window.__delivered = [];
    // What the settlement host keeps: the receipt of each committed dismissal by its operation, and the record of
    // each closed request with how and when it ended. A refusal is not kept, as in internal/bus/request_settlement.go.
    window.__settlements = JSON.parse(sessionStorage.getItem('decision-settlements') || '{}');
    window.__closedRecords = JSON.parse(sessionStorage.getItem('decision-closed') || '{}');
    window.__dismissals = [];
    const keep = () => { sessionStorage.setItem('decision-items', JSON.stringify(__decisionItems)); sessionStorage.setItem('decision-operations', JSON.stringify(__operations)); sessionStorage.setItem('decision-settlements', JSON.stringify(__settlements)); sessionStorage.setItem('decision-closed', JSON.stringify(__closedRecords)); };
    const close = (item, status, actorKind) => { __closedRecords[item.id] = { decision: Object.assign(copy(item), { status, settledAt: window.__settledAt || '2026-11-16T10:58:00Z', actorKind, canDismiss: false, dismissalReason: '' }), closed: true, readOnly: true }; };
    window.__closeRequest = (id, status, actorKind) => { const item = __decisionItems.find(i => i.id === id); __decisionItems = __decisionItems.filter(i => i.id !== id); close(item, status, actorKind); keep(); };
    const refuse = (input, code, text, outcome) => Object.assign({ operationId: input.operationId, id: input.id, revision: input.revision || 0, state: 'refused', code, text }, outcome ? { outcome } : {});
    const settle = input => {
      if (__settlements[input.operationId]) return copy(__settlements[input.operationId]);
      const item = __decisionItems.find(i => i.id === input.id);
      if (window.__refuseDismiss) return refuse(input, __refuseDismiss.code, __refuseDismiss.text, __refuseDismiss.outcome);
      if (!item) return refuse(input, 'request_closed', 'This request is already closed.', (__closedRecords[input.id] || { decision: {} }).decision.status);
      if (item.revision !== input.revision) return refuse(input, 'revision_changed', 'This request changed. Refresh it before continuing.');
      __decisionItems = __decisionItems.filter(i => i.id !== item.id); close(item, 'dismissed', 'person');
      const receipt = { operationId: input.operationId, id: item.id, revision: item.revision + 1, state: 'committed', outcome: 'dismissed', settledAt: __closedRecords[item.id].decision.settledAt };
      __settlements[input.operationId] = receipt; __dismissals.push(copy(input)); keep(); return copy(receipt);
    };
    const commit = input => {
      if (__operations[input.operationId]) return copy(__operations[input.operationId]);
      const item = __decisionItems.find(i => i.id === input.id);
      if (window.__rejectAnswer || !item || item.revision !== input.revision || !input.answers && !item.canDismiss) {
        const receipt = { operationId: input.operationId, state: 'rejected', id: input.id, revision: item ? item.revision : input.revision, closed: false, messageIds: [], reason: { code: 'revision_changed', message: 'The request changed. Review it again.' } };
        __operations[input.operationId] = receipt; keep(); return copy(receipt);
      }
      if (input.answers) input.answers.forEach(answer => { const q = item.questions.find(q => q.number === answer.number); if (q) { q.answered = true; q.answer = answer.asRecommended ? q.recommendation : answer.text; } });
      item.revision++;
      const closed = !input.answers || item.kind === 'mention' || item.questions.every(q => q.answered);
      if (closed) { __decisionItems = __decisionItems.filter(i => i.id !== item.id); close(item, input.answers ? 'answered' : 'dismissed', 'person'); }
      const receipt = { operationId: input.operationId, state: 'committed', id: item.id, revision: item.revision, closed, messageIds: input.answers ? [901] : [], reason: null };
      __operations[input.operationId] = receipt; __delivered.push(copy(input)); keep(); return copy(receipt);
    };
    const api = {
      ReadUIState: async () => JSON.parse(sessionStorage.getItem('decision-ui') || '{"version":1,"values":{}}'),
      SaveUIState: async value => {
        if (window.__saveFailure) throw new Error('Draft storage unavailable');
        if (window.__holdSave) await new Promise(resolve => { window.__releaseSave = resolve; });
        sessionStorage.setItem('decision-ui', JSON.stringify(value));
      },
      Decisions: async before => {
        if (window.__listFailure) throw new Error('Request service unavailable');
        if (window.__malformedList) return { items: [{}], nextBefore: 0 };
        const remaining = __decisionItems.filter(i => !before || i.messageId < before), items = copy(remaining.slice(0, window.__pageSize || 100));
        return { items, nextBefore: items.length && items.length < remaining.length ? items[items.length - 1].messageId : 0 };
      },
      BusGroupMessageContext: async (group, id) => {
        if (window.__holdContext) await new Promise(resolve => { (window.__contextPending || (window.__contextPending = [])).push(resolve); });
        if (window.__contextFailure) throw new Error('The group could not be read');
        return copy(window.__contextMessages || []);
      },
      AnswerDecision: async input => {
        if (window.__beforeFailure) throw new Error('Receipt lost before commit');
        const receipt = commit(input);
        if (window.__lostReceipt) throw new Error('Receipt lost after commit');
        if (window.__malformedReceipt) return {};
        return receipt;
      },
      DismissDecision: async input => commit(input),
      DismissRequest: async input => {
        if (window.__dismissBeforeFailure) throw new Error('Dismissal lost before commit');
        const receipt = settle(input);
        if (window.__lostDismissal) throw new Error('Dismissal receipt lost after commit');
        return window.__malformedDismissal ? { operationId: input.operationId, id: input.id, revision: input.revision, state: 'committed' } : receipt;
      },
      RequestOperation: async (id, operation) => {
        if (window.__operationFailure) throw new Error('Operation lookup unavailable');
        const receipt = __settlements[operation];
        if (!receipt) return { operationId: operation, id: '', revision: 0, state: 'unknown' };
        return receipt.id === id ? copy(receipt) : refuse({ operationId: operation, id }, 'not_authorized', 'This operation belongs to another request.');
      },
      DecisionByID: async id => { if (!__closedRecords[id]) throw new Error('This request is no longer available.'); return copy(__closedRecords[id]); },
      DecisionOperation: async operation => copy(__operations[operation] || { operationId: operation, state: 'unknown', id: '', revision: 0, closed: false, messageIds: [], reason: null }),
      PreviewDecisionBroadcast: async (scope, text) => {
        const preview = { token: 'preview-' + scope, scope, text, count: 2, createdAt: new Date().toISOString(), expiresAt: new Date(Date.now() + (window.__expiredPreview ? -1000 : 300000)).toISOString(), targets: [
          { session: 'one', title: 'One Review', workspace: 'C:/One' }, { session: 'two', title: 'Two Review', workspace: 'C:/Two' }
        ] }; __previews[preview.token] = preview; return copy(preview);
      },
      ConfirmDecisionBroadcast: async input => {
        if (__operations[input.operationId]) return copy(__operations[input.operationId]);
        const receipt = window.__rejectBroadcast ? { operationId: input.operationId, state: 'rejected', id: '', revision: 0, closed: false, messageIds: [], reason: { code: 'preview_expired', message: 'The preview expired. Preview the recipients again.' } } : { operationId: input.operationId, state: 'committed', id: '', revision: 0, closed: true, messageIds: [902, 903], reason: null };
        __operations[input.operationId] = receipt; if (receipt.state === 'committed') __delivered.push(copy(input)); keep();
        if (window.__lostReceipt) throw new Error('Broadcast receipt lost');
        return copy(receipt);
      }
    };
    if (args.entryPoints) {
      api.DecisionEntries = async () => {
        const snapshot = { count: __decisionItems.length, items: __decisionItems.map(i => ({ id: i.id, messageId: i.messageId })) };
        if (window.__holdEntries) await new Promise(resolve => { window.__releaseEntries = resolve; });
        if (window.__entriesFailure) throw new Error('Entry service unavailable');
        return window.__malformedEntries ? { count: 1, items: [] } : snapshot;
      };
      api.Notifications = async () => JSON.parse(sessionStorage.getItem('attention') || '[]');
      api.DismissNotification = async id => {
        window.__dismissed = (window.__dismissed || []).concat(id);
        sessionStorage.setItem('attention', JSON.stringify(JSON.parse(sessionStorage.getItem('attention') || '[]').filter(n => n.id !== id)));
      };
    }
    if (args.tasks) {
      // The task host's side of a question, as cmd/dinah-desktop/request_state.go and internal/taskhost give it:
      // the pending list, each question's record (revision, status), its Dismiss command, and settlement by
      // DismissRequest with the question's key. The host emits task:answered after it commits and before it returns the receipt.
      window.__taskPending = copy(args.tasks); window.__taskRecords = {}; window.__taskOff = copy(args.taskOff || {}); window.__taskSettlements = {}; window.__taskDismissals = []; window.__taskAnswers = [];
      const record = key => __taskRecords[key] || (__taskRecords[key] = { revision: 1, status: 'open' });
      const end = (key, status, actorKind) => { const r = record(key); r.status = status; r.revision++; r.settledAt = '2026-11-16T11:05:00Z'; r.actorKind = actorKind; __taskPending = __taskPending.filter(q => q.key !== key); return r; };
      window.__endTask = (key, status, announce) => { const r = end(key, status, status === 'withdrawn' ? 'owner' : status === 'cancelled' ? 'host' : 'person'); if (announce) __emit('task:answered', { key, disposition: status, revision: r.revision, service: 'fixture.flow', workspace: 'ws-a', task: 'task-a', generation: 'g1' }); };
      api.PendingTaskPermissions = async () => copy(__taskPending);
      api.TaskQuestionStatus = async key => Object.assign({ key }, record(key));
      api.RequestState = async id => {
        if (window.__stateFailure) throw new Error('Question state unavailable');
        const r = record(id), open = r.status === 'open' && __taskPending.some(q => q.key === id), off = !open ? 'This request is no longer open.' : __taskOff[id] || '';
        const command = name => off ? { name, enabled: false, reason: off } : { name, enabled: true, invocation: name === 'Dismiss' ? 'DismissRequest' : 'Answer' };
        return Object.assign({ id, kind: 'task-question', commands: [command('Dismiss'), { name: 'Withdraw', enabled: false, reason: open ? 'Only the available asking owner can withdraw this request.' : off }, command('Answer')], request: copy(args.tasks.find(q => q.key === id) || null) }, r);
      };
      api.Answer = async (key, option) => { __taskAnswers.push([key, option]); __endTask(key, 'answered', true); };
      const requestSettle = api.DismissRequest, requestOperation = api.RequestOperation;
      api.DismissRequest = async input => {
        if (!args.tasks.some(q => q.key === input.id)) return requestSettle(input);
        if (window.__taskBeforeFailure) throw new Error('Task dismissal lost before commit');
        let receipt = __taskSettlements[input.operationId];
        if (!receipt) {
          const r = record(input.id);
          if (window.__refuseTaskDismiss) receipt = refuse(input, __refuseTaskDismiss.code, __refuseTaskDismiss.text);
          else if (r.status !== 'open') receipt = refuse(input, 'request_closed', 'This question has ended.', r.status);
          else if (r.revision !== input.revision) receipt = refuse(input, 'revision_changed', 'This question changed.');
          else {
            __endTask(input.id, 'dismissed', !window.__silentTaskEnd);
            receipt = __taskSettlements[input.operationId] = { operationId: input.operationId, id: input.id, revision: r.revision, state: 'committed', outcome: 'dismissed', settledAt: r.settledAt };
            __taskDismissals.push(copy(input));
          }
        }
        if (window.__lostTaskDismissal) throw new Error('Task dismissal receipt lost after commit');
        return copy(receipt);
      };
      api.RequestOperation = async (id, operation) => {
        if (!args.tasks.some(q => q.key === id)) return requestOperation(id, operation);
        return copy(__taskSettlements[operation] || { id, operationId: operation, state: 'unknown' });
      };
    }
    if (args.unavailable) delete api.Decisions;
    if (args.contextRoom) {
      api.BusOverview = async () => ({ rooms: [{ name: 'review', title: 'Review Group', members: [] }], workspaces: [], inbox: [], links: [] });
      api.BusGroupMessages = async () => copy(args.contextRoom);
      api.BusGroupMessagesFrom = async () => copy(args.contextRoom);
      api.BusMarkGroupRead = async () => {};
    }
    return api;
  }});
  if (!extra.noOpen) {
    await t.page.evaluate(() => dinahWorkbench.open('decisions'));
    await t.page.locator('.decisions').waitFor();
  }
  return t;
}
async function choose(t, id) {
  await t.page.locator('.decisions-row[data-request-id="' + id + '"] > a').click();
  await t.page.locator('.decisions-detail h2').waitFor();
}
module.exports = { setup, makeRequest, choose };
