/* The typed snapshots are authoritative. Reply prose stays available for export. */
(function () {
  'use strict';
  var node = window.dinahHelpers.node;
  function valid(origin) {
    return origin && origin.version === 1 && Number.isSafeInteger(origin.messageId) && origin.messageId > 0 && ['request', 'mention'].includes(origin.kind) && origin.requestId === origin.kind + ':' + origin.messageId &&
      Array.isArray(origin.questions) && origin.questions.length > 0 && origin.questions.length <= 20 && new Set(origin.questions.map(function (q) { return q && q.number; })).size === origin.questions.length && origin.questions.every(function (q) {
        return q && Number.isSafeInteger(q.number) && (origin.kind === 'mention' ? origin.questions.length === 1 && q.number === 0 && q.asRecommended === false : q.number > 0) && typeof q.text === 'string' && typeof q.answer === 'string' && typeof q.asRecommended === 'boolean';
      });
  }
  function card(origin, heading) {
    var el = node('section', 'card delivered-card decision-answer-card'); el.dataset.requestId = origin.requestId;
    var head = node('div', 'card-head'); head.appendChild(node('span', 'card-title delivered-heading', heading));
    var body = node('div', 'card-body decision-answer-body'); body.setAttribute('aria-label', 'Questions and Answers from Decisions'); body.tabIndex = 0;
    origin.questions.forEach(function (q) {
      var group = node('section', 'decision-answer-question');
      group.appendChild(node('h4', '', origin.kind === 'mention' ? 'Original Message' : 'Question ' + q.number));
      group.appendChild(node('p', '', q.text));
      group.appendChild(node('h4', '', 'Answer'));
      group.appendChild(node('p', '', q.answer));
      if (q.asRecommended) group.appendChild(node('p', 'text-muted decision-answer-mode', 'As recommended.'));
      body.appendChild(group);
    });
    var footer = node('div', 'decision-answer-footer'), link = node('a', '', 'Open Request in Decisions');
    var state = new URLSearchParams({ request: origin.requestId, view: 'record' }).toString();
    link.href = '?' + new URLSearchParams({ open: 'decisions', 'view.decisions': state }).toString();
    link.addEventListener('click', function (event) {
      if (event.button || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
      event.preventDefault(); window.dinahWorkbench.open('decisions', state).catch(window.dinahHost.fail);
    });
    footer.appendChild(link); el.append(head, body, footer); return el;
  }
  window.dinahDecisionAnswerCard = Object.freeze({ valid: valid, create: card });
}());
