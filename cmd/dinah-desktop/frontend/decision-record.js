/* Request lookup is read-only and independent of open-list drafts. */
(function () {
  'use strict';
  var node = window.dinahHelpers.node;
  window.dinahDecisionRecord = function (detail, id, validItem, current) {
    var frame = node('section', 'decisions-record'), source = node('header', 'decisions-source');
    var title = node('h2', '', 'Original Request'), meta = node('p', 'text-muted', 'Reading the request.'); source.append(title, meta);
    var status = node('div', 'decisions-delivery'); status.setAttribute('role', 'status'); status.textContent = 'Reading the original request without changing it.';
    var content = node('div', 'decisions-content'); frame.append(source, status, content); detail.replaceChildren(frame);
    function active() { return frame.isConnected && current(); }
    function read() {
      status.textContent = 'Reading the original request without changing it.'; content.replaceChildren();
      Promise.resolve().then(function () {
        var app = window.dinahHelpers.app();
        if (!app || !app.DecisionByID) throw new Error('This build cannot show closed requests.');
        return app.DecisionByID(id);
      }).then(function (value) {
        if (!active()) return;
        if (!value || value.readOnly !== true || typeof value.closed !== 'boolean' || !validItem(value.decision) || value.decision.id !== id) throw new Error('The request response is invalid.');
        var item = value.decision; title.textContent = item.asker;
        meta.textContent = item.workspace + (item.groupTitle ? ' · ' + item.groupTitle : '') + ' · ' + window.dinahRegional.format(new Date(item.createdAt), 'dayTime');
        status.textContent = value.closed ? 'This request is closed. This view is read-only.' : 'This request is still open. This linked view is read-only; use Requests to answer it.';
        content.appendChild(window.dinahDecisionContext(item));
        (item.kind === 'mention' ? [{ number: 0, text: item.text }] : item.questions).forEach(function (q) {
          var group = node('section', 'decisions-question'); group.appendChild(node('h3', '', item.kind === 'mention' ? 'Original Message' : 'Question ' + q.number));
          group.appendChild(node('p', 'decisions-question-text', q.text));
          if (item.kind === 'request') {
            group.appendChild(node('p', 'decisions-recommendation', 'Recommendation: ' + q.recommendation));
            group.appendChild(node('p', '', q.answered ? 'Answer: ' + q.answer : 'This question has no answer.'));
          }
          content.appendChild(group);
        });
        content.appendChild(node('p', 'decisions-destination', 'Originally sent to ' + item.destination + '.'));
      }).catch(function (error) {
        if (!active()) return;
        status.textContent = 'This request is unavailable. Its answer remains in the transcript.';
        content.replaceChildren(node('p', 'text-muted', window.dinahHelpers.message(error, 'sentence')));
        var retry = node('button', 'btn btn-sm', 'Retry Request Lookup'); retry.type = 'button'; retry.addEventListener('click', read); content.appendChild(retry);
      });
    }
    read();
  };
}());
