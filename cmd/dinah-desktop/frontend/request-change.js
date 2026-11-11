/* Request a Change: the person selects a passage of a document, says what
   should change, and chooses a chat of the document's workspace. The chat's
   agent gets the request with the passage, and answers with suggestions the
   person accepts or rejects in the document's window (the Documents applet,
   applet_documents.go). Nothing here changes the document.

   The Editor and Docs offer the command in their Review menu and in the
   menu of a right click on the selection (content-host.js,
   documentCommands), and give the selection as the text it stands for.

   Load after app.js's parts that give window.dinahHost, window.dinahChats
   and window.dinahWorkspaceBrowser. */
(function () {
  'use strict';
  function $(id) { return document.getElementById(id); }
  function app() { return window.go.main.App; }
  function same(a, b) {
    function norm(p) { return String(p || '').replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase(); }
    return !!a && norm(a) === norm(b);
  }

  /* The chats that can propose to the document: those of its workspace,
     since an agent reads and proposes within its own workspace. The one
     used last is first. */
  function chatsFor(ref) {
    var records = window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.records() : [];
    var record = records.find(function (r) { return r.id === ref.workspaceId; });
    var folder = record && (record.executionPath || record.path);
    var known = window.dinahChats ? window.dinahChats.known() : [];
    return known.filter(function (c) { return same(c.cwd, folder); })
      .sort(function (a, b) { return String(b.updatedAt).localeCompare(String(a.updatedAt)); });
  }

  /* What the agent is asked, after the person's own words. The passage and
     the document's name go with the message as its document (history.go,
     draftContext). */
  function instruction(ref) {
    return 'This is a request to change the selected passage of the document. Propose the change as suggestions and do not make it yourself: ' +
      'read the document with dinah.documents/read@1 and propose with dinah.documents/suggest@1, with path ' + JSON.stringify(ref.path) +
      ' and sourceId ' + JSON.stringify(ref.sourceId) + '. Change only the selected passage unless the request cannot be met without more, ' +
      'and give my request as the reason. I accept or reject each suggestion in the document\'s window.';
  }

  var pending = null;
  function close(result) {
    var p = pending;
    pending = null;
    if ($('request-change').open) $('request-change').close();
    if (p) p.resolve(result);
  }

  /* open asks for the request and the chat, and resolves when the request
     is sent or the person cancels. */
  function open(request) {
    if (pending) close(false);
    var chats = chatsFor(request.ref), select = $('request-change-chat');
    $('request-change-passage').textContent = request.selection;
    $('request-change-words').value = '';
    select.textContent = '';
    chats.forEach(function (c) {
      var option = document.createElement('option');
      option.value = c.id;
      option.textContent = c.title + (c.exec === 'running' ? ' (working; the request waits its turn)' : '');
      select.appendChild(option);
    });
    $('request-change-none').hidden = chats.length > 0;
    select.disabled = chats.length === 0;
    $('request-change-send').disabled = chats.length === 0;
    $('request-change-error').hidden = true;
    $('request-change').showModal();
    $('request-change-words').focus();
    return new Promise(function (resolve) { pending = { resolve: resolve, request: request }; });
  }

  function send(event) {
    event.preventDefault();
    if (!pending) return;
    var request = pending.request, words = $('request-change-words').value.trim(), chat = $('request-change-chat').value;
    if (!words || !chat) return;
    var doc = { draftId: request.draftId, title: request.title, path: '', selection: request.selection };
    $('request-change-send').disabled = true;
    Promise.resolve(window.dinahChats.open(chat)).then(function () {
      return app().Send(chat, words + '\n\n' + instruction(request.ref), doc, []);
    }).then(function () {
      /* The dialog goes first: a toast made while it is open belongs to
         it and would go with it. */
      close(true);
      if (window.dinahHost) window.dinahHost.toast('The request went to the chat. Its suggestions will appear in "' + request.title + '".', 'info');
    }, function (err) {
      $('request-change-send').disabled = false;
      $('request-change-error').textContent = String(err && err.message || err).replace(/^Error: /, '');
      $('request-change-error').hidden = false;
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    if (!$('request-change')) return;
    $('request-change-form').addEventListener('submit', send);
    $('request-change-cancel').addEventListener('click', function () { close(false); });
    $('request-change').addEventListener('close', function () { if (pending) close(false); });
  });

  window.dinahRequestChange = { open: open, chatsFor: chatsFor };
})();
