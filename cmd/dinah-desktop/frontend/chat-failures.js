/* Failed steps stay in the run. A person can opt into a compact review link
   for a chat without turning recovered tool errors into Notifications. */
(function () {
  'use strict';
  var records = new Map(), listeners = new Set();
  var maxChats = 32, maxSteps = 100, maxText = 262144;
  function preference(id) { return 'dinah-failure-summaries:' + id; }
  function enabled(id) { return !!(window.dinahUIState && dinahUIState.get(preference(id), false)); }
  function changed() { listeners.forEach(function (draw) { draw(); }); }
  function dismiss(id) {
    var record = records.get(id);
    if (record && record.row) record.row.remove();
    records.delete(id);
    changed();
  }
  function sync(session) {
    var record = records.get(session.id);
    if (!record) return;
    if (!enabled(session.id)) { if (record.row) record.row.remove(); return; }
    if (!record.row) {
      var row = document.createElement('li'); row.className = 'activity-row failure-summary';
      var words = document.createElement('span'); row.appendChild(words);
      var review = document.createElement('button'); review.type = 'button'; review.className = 'btn btn-sm'; review.textContent = 'Review Failed Commands';
      review.addEventListener('click', function () {
        dinahWorkbench.open({ kind: 'chat-failures', workspace: session.info.cwd || '', path: session.id }).catch(window.dinahHost.fail);
      });
      var close = document.createElement('button'); close.type = 'button'; close.className = 'btn btn-sm'; close.textContent = 'Dismiss';
      close.addEventListener('click', function () { dismiss(session.id); });
      row.append(review, close); record.row = row;
    }
    record.row.firstChild.textContent = record.total + (record.total === 1 ? ' step failed.' : ' steps failed.');
    if (record.row.parentNode !== session.log) session.log.appendChild(record.row);
  }
  function recordFailures(session, failures, name) {
    if (!failures.length) return;
    var record = records.get(session.id) || { steps: [], total: 0, text: 0 };
    record.name = name || session.id;
    records.delete(session.id); records.set(session.id, record);
    failures.forEach(function (step) {
      var entry = { name: (step.name || 'Failed step').slice(0, 2048), command: (step.command || '').slice(0, 4096), text: (step.text || '').slice(-32768) };
      if ((step.text || '').length > 32768) entry.text = '[Showing the last part of the output]\n' + entry.text;
      entry.size = entry.name.length + entry.command.length + entry.text.length;
      entry.id = ++record.total;
      record.steps.push(entry); record.text += entry.size;
      while (record.steps.length > maxSteps || record.text > maxText) record.text -= record.steps.shift().size;
    });
    while (records.size > maxChats) dismiss(records.keys().next().value);
    sync(session); changed();
  }
  window.dinahFailures = {
    enabled: enabled, sync: sync, record: recordFailures,
    setEnabled: function (session, value) { dinahUIState.set(preference(session.id), !!value); sync(session); }
  };
  dinahWorkbench.register('chat-failures', { applet: { id: 'dinah.chat', title: 'Chats', glyph: 'comments', workspace: true }, title: 'Failed Commands', glyph: 'warning', init: function (root, opts, item) {
    root.classList.add('failure-review');
    var owner = records.get(item.path), win = root.closest('[data-win]');
    if (owner && win) pudlWindows.retitle(win.dataset.win, 'Failed Commands · ' + owner.name);
    function draw() { dinahRender.enqueue(root, root, render); }
    function render() {
      var expanded = new Set(Array.from(root.querySelectorAll('details[open]'), function (detail) { return detail.dataset.failureId; }));
      root.replaceChildren();
      var record = records.get(item.path), note = document.createElement('p');
      note.textContent = !record ? 'No failure details are retained for this chat.' : record.total + ' failed steps. The agent may have recovered; check its reply in the chat.';
      root.appendChild(note);
      if (!record) return;
      if (record.total > record.steps.length) {
        var limit = document.createElement('p'); limit.textContent = 'Showing the latest ' + record.steps.length + ' steps within the review limit. Earlier details remain in the chat log.'; root.appendChild(limit);
      }
      record.steps.forEach(function (step) {
        var detail = document.createElement('details'), summary = document.createElement('summary'), pre = document.createElement('pre');
        detail.dataset.failureId = String(step.id); detail.open = expanded.has(String(step.id));
        summary.textContent = step.name; pre.textContent = (step.command ? step.command + '\n\n' : '') + (step.text || 'No output was supplied.');
        detail.append(summary, pre); root.appendChild(detail);
      });
    }
    listeners.add(draw); draw();
    return { destroy: function () { listeners.delete(draw); dinahRender.cancel(root); } };
  } });
})();
