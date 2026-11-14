/* The fixed feature identity is stored by startup in whats_new.go. */
(function () {
  'use strict';
  window.dinahWhatsNew = {
    id: 'whats-new-20261114',
    actions: function (updates) {
      var open = function (kind, state) { return window.dinahWorkbench.open(kind, state || ''); };
      var actions = [
        { label: 'Open Decisions', run: function () { return open('decisions'); } },
        { label: 'Open Working with Chats', run: function () { return open('help-chats'); } },
        { label: 'Open Background Chats', run: function () { return open('settings', 'chat?background'); } },
        { label: 'Open Set-aside Programs', run: function () { return open('settings', 'set-aside'); } }
      ];
      if (updates) actions.push({ label: 'Open Updates', run: function () { return open('settings', 'updates'); } });
      return actions;
    }
  };
})();
