/* Request lookup is read-only and independent of open-list drafts. */
(function () {
  'use strict';
  var node = window.dinahHelpers.node;
  window.dinahDecisionRecord = function (detail, id, validItem, current) {
    var frame = node('section', 'decisions-record'), source = node('header', 'decisions-source');
    var title = node('h2', '', 'Original Request'), meta = node('p', 'text-muted', 'Loading the request.'); source.append(title, meta);
    var status = node('div', 'decisions-delivery'); status.setAttribute('role', 'status'); status.textContent = 'Loading the original request without changing it.';
    var content = node('div', 'decisions-content'); frame.append(source, status, content); detail.replaceChildren(frame);
    function active() { return frame.isConnected && current(); }
    function read() {
      status.classList.remove('danger');
      status.textContent = 'Loading the original request without changing it.'; content.replaceChildren();
      Promise.resolve().then(function () {
        var app = window.dinahHelpers.app();
        if (!app || !app.DecisionByID) throw new Error('This build cannot show closed requests.');
        return app.DecisionByID(id);
      }).then(function (value) {
        if (!active()) return;
        if (!value || value.readOnly !== true || typeof value.closed !== 'boolean' || !validItem(value.decision) || value.decision.id !== id) throw new Error('The request response is invalid.');
        var item = value.decision; title.textContent = item.asker;
        meta.textContent = item.workspace + (item.groupTitle ? ' · ' + item.groupTitle : '') + ' · ' + window.dinahRegional.format(new Date(item.createdAt), 'dayTime');
        // The host's record says how a closed request ended and when (status and settledAt). An answered request,
        // and one closed before the host kept its ending, is said as closed, as it was before.
        var how = value.closed ? item.status : '', when = Number.isFinite(Date.parse(item.settledAt)) ? ' on ' + window.dinahRegional.format(new Date(item.settledAt), 'dayTime') : '';
        var answered = item.questions.some(function (q) { return q.answered; });
        status.textContent = !value.closed ? 'This request is still open. This linked view is read-only; use Requests to answer it.' :
          how === 'dismissed' ? 'You dismissed this request' + when + '. ' + (answered ? 'The answers sent before then are kept.' : 'No answer was sent.') + ' This view is read-only.' :
          how === 'withdrawn' ? item.asker + ' withdrew this request' + when + '. This view is read-only.' : 'This request is closed. This view is read-only.';
        if (how === 'withdrawn') {
          var notice = node('div', 'notice'), words = node('div', 'notice-content');
          words.append(node('p', 'notice-title', item.asker + ' withdrew this request' + when), node('p', 'notice-body', 'It no longer needs an answer from you.')); notice.appendChild(words); content.appendChild(notice);
        }
        content.appendChild(window.dinahDecisionContext(item));
        (item.kind === 'mention' ? [{ number: 0, text: item.text }] : item.questions).forEach(function (q) {
          var group = node('section', 'decisions-question'); group.appendChild(node('h3', '', item.kind === 'mention' ? 'Original Message' : 'Question ' + q.number));
          group.appendChild(node('p', 'decisions-question-text', q.text));
          if (item.kind === 'request') {
            group.appendChild(node('p', 'decisions-recommendation', 'Recommendation: ' + q.recommendation));
            group.appendChild(node('p', '', q.answered ? 'Answer: ' + q.answer : how === 'dismissed' ? 'This question was dismissed without an answer.' : how === 'withdrawn' ? 'This question was withdrawn without an answer.' : 'This question has not been answered.'));
          }
          content.appendChild(group);
        });
        content.appendChild(node('p', 'decisions-destination', 'Originally sent to ' + item.destination + '.'));
      }).catch(function (error) {
        if (!active()) return;
        status.textContent = 'This request is unavailable. You can still read its answer in the chat.';
        status.classList.add('danger');
        content.replaceChildren(node('p', 'text-muted', window.dinahHelpers.message(error, 'sentence')));
        var retry = node('button', 'btn btn-sm', 'Try Again'); retry.type = 'button'; retry.addEventListener('click', read); content.appendChild(retry);
      });
    }
    read();
  };
}());
