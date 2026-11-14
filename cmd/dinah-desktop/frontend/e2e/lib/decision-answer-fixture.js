const { launch } = require('./page');
const { makeRequest } = require('./decisions-fixture');
const origin = (extra = {}) => ({ version: 1, requestId: 'request:101', messageId: 101, kind: 'request', questions: [
  { number: 2, text: 'Which build should we use?', answer: 'Use the tested build.', asRecommended: true },
  { number: 7, text: 'When should we proceed?', answer: 'Use the tested build.', asRecommended: false }
], ...extra });
const entry = (id, answerOrigin = origin()) => ({ role: 'delivered', text: 'The ordinary agent-facing prose stays available.', provider: 'codex', delivered: { id, kind: 'text', from: 'paul', time: '2026-11-14T10:00:00Z', answerOrigin } });
async function start(options = {}) {
  const request = makeRequest('request:101', { messageId: 101, group: '', groupTitle: '', destination: 'Asking chat', questions: [
    { number: 2, text: 'Which build should we use?', recommendation: 'Use the tested build.', choices: [], answered: true, answer: 'Use the tested build.' },
    { number: 7, text: 'When should we proceed?', recommendation: 'Tomorrow.', choices: [], answered: true, answer: 'Friday.' }
  ] });
  const t = await launch({ viewport: { width: 1440, height: 1000 }, args: { record: { decision: request, closed: true, readOnly: true }, entries: [entry(501)], ...options }, app: data => {
    const clone = value => JSON.parse(JSON.stringify(value));
    window.__answerEntries = clone(data.entries); window.__answerRecord = clone(data.record); window.__answerReads = []; window.__answerMutations = [];
    const base = [{ role: 'you', text: 'Earlier question.', provider: 'codex' }, { role: 'agent', text: 'Earlier result.', provider: 'codex' }];
    const info = id => ({ sessionId: id, provider: 'codex', cwd: 'C:/Fixture', title: id, open: true, working: false, queued: [] });
    return {
      State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'], defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [], accountName: 'Paul Parks', busHandle: 'paul' }),
      Sessions: async () => ['live', 'saved'].map(info),
      SessionsAt: async () => ['live', 'saved'].map(info),
      ResolveSessionContinuation: async id => id,
      SavedConversation: async id => ({ available: true, complete: true, streamGen: 'answer-fixture', streamThrough: 0, messages: clone(id === 'saved' ? base.concat(__answerEntries) : base) }),
      ChatEvents: async () => ({ gen: 'answer-fixture', head: 0, events: [], reset: false }),
      SessionContinuation: async () => ({ history: [] }),
      OpenSessionAt: async (workspace, id) => info(id),
      Decisions: async () => ({ items: [], nextBefore: 0 }),
      DecisionByID: async id => {
        __answerReads.push(id); const value = clone(__answerRecord);
        if (window.__holdAnswerRead) await new Promise(resolve => { (window.__answerPending || (window.__answerPending = [])).push(resolve); });
        if (window.__failAnswerRead) throw new Error('The request no longer exists.');
        return value;
      },
      AnswerDecision: async input => { __answerMutations.push(['answer', input]); throw new Error('Read-only lookup must not answer'); },
      DismissDecision: async input => { __answerMutations.push(['dismiss', input]); throw new Error('Read-only lookup must not dismiss'); },
      Send: async () => {}, Queue: async () => {}, Steer: async () => {}, Unqueue: async () => {}, Answer: async () => {}, Cancel: async () => {}, Stop: async () => {},
      SetApprovals: async () => {}, SaveRun: async text => { window.__answerExport = text; return ''; }
    };
  }});
  t.chatKey = await t.page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'live' }));
  t.chat = t.page.locator('[data-win="' + t.chatKey + '"]');
  return t;
}
async function deliver(t, value) { await t.page.evaluate(value => __emit('agent:sent', { sessionId: 'live', kind: 'bus', delivered: [value] }), value); }
module.exports = { start, origin, entry, deliver };
