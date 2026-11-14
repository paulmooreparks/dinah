/* Durable per-request drafts share the host's saved UI state and error reporting. */
(function () {
  'use strict';
  var prefix = 'dinah-decisions:';
  function copy(value) { return JSON.parse(JSON.stringify(value)); }
  function key(id) { return prefix + encodeURIComponent(id); }
  function read(id) {
    var saved = window.dinahUIState.get(key(id), null);
    if (saved === null) return { version: 1, answers: {}, reviewed: '', operation: null };
    if (!saved || saved.version !== 1 || !saved.answers || typeof saved.answers !== 'object' || Array.isArray(saved.answers) || typeof saved.reviewed !== 'string' || !Object.values(saved.answers).every(function (answer) { return answer && typeof answer.text === 'string' && typeof answer.asRecommended === 'boolean'; })) throw new Error('The saved decision draft is invalid.');
    var op = saved.operation;
    if (op) {
      if (!['AnswerDecision', 'DismissDecision', 'ConfirmDecisionBroadcast'].includes(op.method) || !['prepared', 'submitted', 'unknown', 'committed', 'rejected'].includes(op.status) || !op.input || typeof op.input.operationId !== 'string' || !op.input.operationId || op.input.operationId.length > 128) throw new Error('The saved delivery identity is invalid.');
      if (op.method === 'ConfirmDecisionBroadcast') {
        if (!/^broadcast-(active|all)$/.test(id) || typeof op.input.token !== 'string' || !op.input.token) throw new Error('The saved broadcast identity is invalid.');
      } else {
        if (!id.startsWith('request:') || op.input.id !== id.slice(8) || !Number.isSafeInteger(op.input.revision) || op.input.revision < 1) throw new Error('The saved request identity is invalid.');
        if (op.method === 'AnswerDecision' && (!Array.isArray(op.input.answers) || !op.input.answers.length || !op.input.answers.every(function (answer) { return Number.isSafeInteger(answer.number) && answer.number >= 0 && typeof answer.text === 'string' && typeof answer.asRecommended === 'boolean'; }))) throw new Error('The saved answers are invalid.');
      }
      if ((op.status === 'committed' || op.status === 'rejected') && (!op.receipt || op.receipt.operationId !== op.input.operationId || op.receipt.state !== op.status || op.status === 'rejected' && (!op.receipt.reason || typeof op.receipt.reason.message !== 'string'))) throw new Error('The saved delivery receipt is invalid.');
    }
    return saved;
  }
  function write(id, draft) {
    window.dinahUIState.set(key(id), copy(draft));
    return window.dinahUIState.flush();
  }
  window.dinahDecisionDrafts = { read: read, write: write };
}());
