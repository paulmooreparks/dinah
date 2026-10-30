(function () {
  'use strict';
  /* Restart to Update (update.go). When a newer build is installed, the
     tray shows Update, and its menu and the Dinah menu restart into it:
     either once the running turns end, holding new ones meanwhile, or at
     once, after a question, ending the turns still running. */
  const app = window.dinahHelpers.app;
  function host() { return window.dinahHost; }
  var state = { ready: false, pending: false, working: 0 };

  function turns(n) { return n === 1 ? '1 chat is' : n + ' chats are'; }
  function draw() {
    var item = document.getElementById('status-update');
    if (!item) return;
    item.hidden = !state.ready;
    var name = window.dinahProduct.name;
    var note = state.pending ? name + ' restarts when the running turns end. ' + turns(state.working) + ' still working; new messages wait until after the restart.'
      : 'A newer build of ' + name + ' is installed. Restarting reopens your chats, and their messages wait for them.';
    var label = state.pending ? name + ' restarts for its update when the running turns end' : 'A newer build of ' + name + ' is installed';
    item.setAttribute('aria-label', label); item.title = label;
    item.querySelector('.status-words').textContent = state.pending ? 'Restarting' : 'Update';
    document.getElementById('update-note').textContent = note;
    document.getElementById('update-restart').hidden = state.pending;
    document.getElementById('update-cancel').hidden = !state.pending;
    window.dinahCommands.changed();
  }
  /* When a newer build arrives, a toast says so with Restart Now as its
     action (PUDL 0.90.0), and Notifications keeps a card with the same
     step, as the tray's Update item does, since a toast with an action
     stays until it is dismissed but is easy to dismiss and forget. The
     action is the restart-now command below, so the toast's button takes
     its label from the command and asks the same question as the tray's
     Restart Now. Both go when the newer build is no longer waiting. */
  var toast = null;
  function announce(was) {
    var notices = host().notices, name = window.dinahProduct.name;
    if (state.ready && !was) {
      var words = 'A newer build of ' + name + ' is installed. Restart Now restarts ' + name + ' into it and reopens your chats.';
      if (window.pudlToast) toast = window.pudlToast(words, { action: { run: 'restart-now', label: 'Restart Now' } });
      if (notices) notices.add({ id: 'dinah-update', type: 'info', glyph: 'reload', title: 'A newer build of ' + name + ' is installed.', description: 'A newer build of ' + name + ' is installed.',
        body: 'Restart Now restarts ' + name + ' into it, ending any turn still running, and reopens your chats. The Update item in the status bar can also restart when the running turns end.',
        actions: [{ label: 'Restart Now…', primary: true, run: function () { return restart(true); } }], dismissible: true, announced: true, className: 'dinah-update' });
    } else if (!state.ready && was) {
      if (toast && toast.isConnected) { var close = toast.querySelector('.toast-close'); if (close) close.click(); }
      toast = null;
      if (notices) notices.remove('dinah-update');
    }
  }
  // A reread of the state is applied unless an event has already given a newer one (state-order.js).
  function read(s) { if (window.dinahStateOrder.accept('update:state', '', s, true)) set(s); }
  function set(s) { var was = state.ready; state = s || state; draw(); announce(was); }

  function restart(now) {
    if (!now) return Promise.resolve(app().RestartToUpdate(false)).catch(host().fail);
    /* The count the question gives is read now: the page's copy is only as new as the last time
       the engine sent it, and chats may have finished since (the engine sends it when the update
       arrives and while a restart waits). A failed read leaves the copy the page has. */
    var fresh = app().UpdateState ? Promise.resolve(app().UpdateState()).then(read).catch(function () {}) : Promise.resolve();
    return fresh.then(function () {
      var words = state.working ? turns(state.working) + ' still working, and ' + (state.working === 1 ? 'its turn ends' : 'their turns end') + ' unfinished.' : 'No chat is working.';
      return host().ask('Restart Now?', words + ' The chats reopen after the restart.', 'Restart Now', state.working > 0);
    }).then(function (yes) {
      if (yes) return app().RestartToUpdate(true);
    }).catch(host().fail);
  }

  window.dinahCommands.register({ id: 'restart-to-update', menu: 'dinah', mnemonic: 'U', glyph: 'reload', label: function () { return state.pending ? 'Cancel the Restart' : 'Restart When Chats Finish'; },
    visible: function () { return state.ready; },
    run: function () { return state.pending ? Promise.resolve(app().CancelRestart()) : restart(false); } });
  /* Restart Now as a PUDL command for the page, which the toast's action
     runs. Its label ends in an ellipsis because it asks first; PUDL drops
     the ellipsis on the toast's button. */
  if (window.pudlCommands) window.pudlCommands.register(document.body, function () {
    return [{ id: 'restart-now', label: 'Restart Now…', glyph: 'reload', hidden: function () { return !state.ready; }, run: function () { return restart(true); } }];
  });

  document.addEventListener('DOMContentLoaded', function () {
    document.getElementById('update-restart').addEventListener('click', function () { restart(false); });
    document.getElementById('update-now').addEventListener('click', function () { restart(true); });
    document.getElementById('update-cancel').addEventListener('click', function () { Promise.resolve(app().CancelRestart()).catch(host().fail); });
    host().on('update:state', set);
    host().onResync(function () { return app() && app().UpdateState ? Promise.resolve(app().UpdateState()).then(read) : null; });
    if (app() && app().UpdateState) Promise.resolve(app().UpdateState()).then(read).catch(function () {});
  });
  window.dinahUpdate = { state: function () { return Object.assign({}, state); } };
})();
