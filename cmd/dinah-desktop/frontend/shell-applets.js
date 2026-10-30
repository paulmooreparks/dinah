/* Singleton views keep their controls while closed so asynchronous updates
   and unsaved settings remain available when the window is reopened. */
(function () {
  'use strict';
  var storage = document.getElementById('applet-storage');
  function retain(kind, title, glyph, node, mode, fields) {
    var content = document.querySelector(node);
    content.hidden = false;
    storage.appendChild(content);
    window.dinahWorkbench.register(kind, {
      singleton: true, title: title, glyph: glyph, mode: kind === 'settings' ? undefined : mode || 'dock-left',
      dockOnly: kind === 'workspace',
      init: function (root, opts) {
        root.appendChild(content);
        var settingsSection = 'appearance';
        function settingsTab(value, record) {
          // The sections are the tabs the page has.
          var choices = Array.from(content.querySelectorAll('[data-settings-section]')).map(function (tab) { return tab.dataset.settingsSection; });
          settingsSection = choices.includes(value) ? value : 'appearance';
          content.querySelectorAll('[data-settings-section]').forEach(function (tab) {
            var selected = tab.dataset.settingsSection === settingsSection;
            tab.setAttribute('aria-selected', String(selected)); tab.tabIndex = selected ? 0 : -1;
            document.getElementById(tab.getAttribute('aria-controls')).hidden = !selected;
          });
          if (record) opts.changed(settingsSection);
        }
        function settingsClick(event) {
          var tab = event.target.closest('[data-settings-section]');
          if (tab) settingsTab(tab.dataset.settingsSection, true);
        }
        if (kind === 'settings') content.addEventListener('click', settingsClick);
        function state() {
          if (kind === 'settings') return settingsSection;
          var value = {};
          (fields || []).forEach(function (id) { value[id] = document.getElementById(id).value; });
          return JSON.stringify(value);
        }
        function setState(value) {
          if (kind === 'settings') { settingsTab(value, false); return; }
          try {
            var parsed = JSON.parse(value || '{}');
            (fields || []).forEach(function (id) {
              var el = document.getElementById(id);
              if (typeof parsed[id] !== 'string') return;
              el.value = parsed[id];
              el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input'));
            });
          } catch (err) { /* An invalid filter state leaves the controls unchanged. */ }
        }
        function changed() { opts.changed(state()); }
        (fields || []).forEach(function (id) {
          var el = document.getElementById(id);
          el.addEventListener(el.tagName === 'SELECT' ? 'change' : 'input', changed);
        });
        setState(opts.state);
        function commands() {
          if (kind === 'workspace') {
            return ['new-workspace', 'new-empty-workspace', 'rename-workspace', 'refresh-workspace', 'remove-workspace'].map(function (name) {
              var button = document.querySelector('#workspace-status-menu [data-cmd="' + name + '"]');
              return window.dinahCommands.describe(name, button.textContent);
            });
          }
          // Notifications' window offers the next entry that needs the person.
          if (kind === 'needs-you') {
            var notices = window.dinahHost && window.dinahHost.notices;
            return [{ label: 'Go to Next Notification', disabled: !notices || !notices.count().needsPerson, run: function () {
              if (notices && notices.count().needsPerson) window.dinahCommands.run('next-notification');
            } }];
          }
          var controls = kind === 'chat-browser' ?
            Array.from(content.querySelectorAll('button[data-cmd]')) : kind === 'settings' ?
            Array.from(content.querySelector('#settings-' + settingsSection).querySelectorAll('button:not([role="radio"])')) : [];
          var commands = controls.map(function (button) {
            var label = button.textContent.trim();
            if (button.dataset.open === 'model-prefs') label = button.closest('#defaults') ? 'Choose Default Model…' : 'Choose Favorite Models…';
            if (button.id === 'usage-save') label = 'Save Usage Page';
            var provider = button.closest('[data-provider]');
            if (provider) {
              var name = provider.querySelector('.card-title').textContent;
              if (label === 'Sign In') label = 'Sign In to ' + name;
              else if (label === 'Test Connection') label = 'Test ' + name + ' Connection';
              else if (label === 'Choose Executable…') label = 'Choose ' + name + ' Executable…';
              else if (label === 'Use Automatic Discovery') label = 'Use Automatic Discovery for ' + name;
            }
            return button.dataset.cmd ? window.dinahCommands.describe(button.dataset.cmd, label) : { label: label, disabled: button.disabled, run: function () { button.click(); } };
          });
          function hostCommand(label, cmd) { commands.push(window.dinahCommands.describe(cmd, label)); }
          if (kind === 'chat-browser') { hostCommand('Refresh Chats', 'refresh-sessions'); hostCommand('Show Hidden Chats Again', 'unhide-sessions'); }
          return commands;
        }
        return { state: state, setState: setState, commands: commands, destroy: function () {
          if (kind === 'settings') content.removeEventListener('click', settingsClick);
          (fields || []).forEach(function (id) {
            var el = document.getElementById(id);
            el.removeEventListener(el.tagName === 'SELECT' ? 'change' : 'input', changed);
          });
          storage.appendChild(content);
        } };
      }
    });
  }
  // Workspace and In Progress dock on the left, and Chats and Notifications
  // on the right, so neither tab strip runs out of room at its first width;
  // a layout the person saved keeps its own sides.
  retain('workspace', 'Workspace', 'folder', '#sidebar');
  retain('chat-browser', 'Chats', 'comments', '#chat-browser-view', 'dock-right', ['chat-filter', 'chat-scope']);
  retain('needs-you', 'Notifications', 'warning', '#needs-you', 'dock-right');
  retain('settings', 'Settings', 'gear', '#settings');
})();
