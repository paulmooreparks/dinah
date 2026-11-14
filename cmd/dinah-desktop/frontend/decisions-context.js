/* Group context is a bounded read; opening a group is a separate action. */
(function () {
  'use strict';
  var node = window.dinahHelpers.node;
  function messageLink(item, message) {
    var key = window.dinahWorkbench.key({ kind: 'bus-room', path: item.group });
    var link = node('a', '', 'Open Message');
    var params = new URLSearchParams({ open: key }); params.set('view.' + key, JSON.stringify({ message: message.id })); link.href = '?' + params.toString();
    link.addEventListener('click', function (event) {
      if (event.button || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
      event.preventDefault(); window.dinahBus.openGroup(item.group, { message: message.id }).catch(window.dinahHost.fail);
    });
    return link;
  }
  function valid(messages, item) {
    return Array.isArray(messages) && messages.length <= 5 && messages.every(function (message, index) {
      return message && Number.isSafeInteger(message.id) && message.id > 0 && message.id < item.messageId && (!index || messages[index - 1].id < message.id) && message.room === item.group && typeof message.from === 'string' && typeof message.text === 'string' && Number.isFinite(Date.parse(message.createdAt));
    });
  }
  window.dinahDecisionContext = function (item) {
    var area = node('section', 'decisions-context'); area.setAttribute('aria-label', 'Conversation Context');
    area.appendChild(node('h3', '', 'Before This Message'));
    var body = node('div', 'decisions-context-body'); area.appendChild(body);
    function read() {
      body.replaceChildren(node('p', 'text-muted', 'Reading the group conversation.')); body.setAttribute('aria-busy', 'true');
      Promise.resolve().then(function () {
        var app = window.dinahHelpers.app();
        if (!app || !app.BusGroupMessageContext) throw new Error('Conversation context is unavailable in this build.');
        return app.BusGroupMessageContext(item.group, item.messageId);
      }).then(function (messages) {
        if (!area.isConnected) return;
        if (!valid(messages, item)) throw new Error('The conversation response is invalid.');
        body.replaceChildren();
        if (!messages.length) body.appendChild(node('p', 'text-muted', 'There are no earlier messages in this group.'));
        messages.forEach(function (message) {
          var entry = node('article', 'decisions-context-message'); entry.dataset.contextMessage = String(message.id);
          var meta = node('p', 'decisions-context-meta'); meta.appendChild(node('strong', '', message.from));
          var time = node('time', '', window.dinahRegional.format(new Date(message.createdAt), 'dayTime')); time.dateTime = message.createdAt; meta.appendChild(time);
          var text = node('p', 'decisions-context-text', message.text);
          var toggle = node('button', 'btn btn-sm', 'Expand Message'); toggle.type = 'button'; toggle.setAttribute('aria-expanded', 'false');
          toggle.addEventListener('click', function () { var expanded = toggle.getAttribute('aria-expanded') !== 'true'; toggle.setAttribute('aria-expanded', String(expanded)); entry.dataset.expanded = String(expanded); toggle.textContent = expanded ? 'Collapse Message' : 'Expand Message'; });
          var actions = node('div', 'decisions-actions'); actions.append(toggle, messageLink(item, message)); entry.append(meta, text, actions); body.appendChild(entry);
        });
      }).catch(function (error) {
        if (!area.isConnected) return;
        body.replaceChildren(node('p', 'text-muted', 'Conversation could not be read. ' + window.dinahHelpers.message(error, 'sentence')));
        var retry = node('button', 'btn btn-sm', 'Retry Conversation'); retry.type = 'button'; retry.addEventListener('click', read); body.appendChild(retry);
      }).finally(function () { body.removeAttribute('aria-busy'); });
    }
    if (item.group) read(); else body.appendChild(node('p', 'text-muted', 'This request was sent directly and has no group conversation.'));
    return area;
  };
}());
