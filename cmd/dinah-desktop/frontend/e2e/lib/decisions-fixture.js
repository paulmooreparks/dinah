const { launch } = require('./page');
const makeRequest = (id, extra = {}) => Object.assign({ id, canDismiss: !!extra.canDismiss || extra.kind === 'mention', dismissalReason: '', revision: 1, messageId: Number(id.replace(/\D/g, '')) || 1, kind: 'request', createdAt: '2026-11-14T00:00:00Z', askerSession: 'session-' + id, asker: 'Review ' + id, workspace: 'C:/Fixture', group: '#Review', groupTitle: 'Review Group', destination: 'Review Group, replying to Review ' + id, text: 'Please decide.', questions: [
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
    const keep = () => { sessionStorage.setItem('decision-items', JSON.stringify(__decisionItems)); sessionStorage.setItem('decision-operations', JSON.stringify(__operations)); };
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
      if (closed) __decisionItems = __decisionItems.filter(i => i.id !== item.id);
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
