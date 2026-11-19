/* Singleton views keep their controls while closed so asynchronous updates
   and unsaved settings remain available when the window is reopened. */
(function () {
  'use strict';
  var storage = document.getElementById('applet-storage');
  var applets = {
    workspace: { id: 'dinah.workspace', title: 'Workspace', glyph: 'folder', workspace: false },
    'chat-browser': { id: 'dinah.chat', title: 'Chats', glyph: 'comments', workspace: false },
    'needs-you': { id: 'dinah.notifications', title: 'Notifications', glyph: 'warning', workspace: false },
    settings: { id: 'dinah.settings', title: 'Settings', glyph: 'gear', workspace: false }
  };
  /* A Settings section that has a view of its own, such as Agent File
     Access with its log filters, keeps that view in the Settings window's
     address after the section's name and a "?": "file-access?outcome=denied".
     The section registers state(), which returns its part of the address,
     and shown(part), which is called whenever the section is shown with the
     part it is shown with; it calls changed() when its view changes. */
  var sectionHooks = {}, settingsChanged = null;
  window.dinahSettingsSections = Object.freeze({
    register: function (name, hook) { sectionHooks[name] = hook; },
    changed: function () { if (settingsChanged) settingsChanged(); }
  });
  var backgroundChats = false;
  sectionHooks.chat = {
    state: function () { return backgroundChats ? 'background' : ''; },
    shown: function (part) {
      backgroundChats = part === 'background';
      if (backgroundChats) requestAnimationFrame(function () {
        if (!window.pudlWindows || window.pudlWindows.state().top !== 'settings') return;
        document.getElementById('set-background-chats').closest('section').scrollIntoView({ block: 'start' });
        document.getElementById('set-background-chat-limit').focus({ preventScroll: true });
      });
    }
  };
  function retain(kind, title, glyph, node, mode, fields) {
    var content = document.querySelector(node);
    content.hidden = false;
    storage.appendChild(content);
    window.dinahWorkbench.register(kind, {
      applet: applets[kind],
      singleton: true, title: title, glyph: glyph, mode: kind === 'settings' ? undefined : mode || 'dock-left',
      dockOnly: kind === 'workspace',
      init: function (root, opts) {
        root.appendChild(content);
        var settingsSection = 'appearance';
        function settingsTab(value, record) {
          // The sections are the tabs the page has.
          var at = String(value || '').indexOf('?'), name = at < 0 ? String(value || '') : value.slice(0, at), part = at < 0 ? '' : value.slice(at + 1);
          var choices = Array.from(content.querySelectorAll('[data-settings-section]')).filter(function (tab) { return !tab.hidden; }).map(function (tab) { return tab.dataset.settingsSection; });
          settingsSection = choices.includes(name) ? name : 'appearance';
          content.querySelectorAll('[data-settings-section]').forEach(function (tab) {
            var selected = tab.dataset.settingsSection === settingsSection;
            tab.setAttribute('aria-selected', String(selected)); tab.tabIndex = selected ? 0 : -1;
            document.getElementById(tab.getAttribute('aria-controls')).hidden = !selected;
          });
          var hook = sectionHooks[settingsSection];
          if (hook && hook.shown) hook.shown(part);
          document.dispatchEvent(new CustomEvent('dinah:settings-section', { detail: { section: settingsSection } }));
          if (record) opts.changed(state());
        }
        if (kind === 'settings') settingsChanged = function () { opts.changed(state()); };
        function settingsClick(event) {
          var tab = event.target.closest('[data-settings-section]');
          if (tab && !tab.hidden) settingsTab(tab.dataset.settingsSection, true);
        }
        if (kind === 'settings') content.addEventListener('click', settingsClick);
        function state() {
          if (kind === 'settings') {
            var hook = sectionHooks[settingsSection], part = hook && hook.state ? hook.state() : '';
            return settingsSection + (part ? '?' + part : '');
          }
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
        /* The three panes add no menu to the bar, since the host's menus
           have their commands (docs/design/menus.md): Workspace has the
           workspace's, Chat has New Chat and the Broadcast rows, Window
           has Go to the Question, and View, Refresh reads the Workspace
           and Chats panes again (reload, below). Settings lists the
           buttons of the section it shows. */
        function commands() {
          if (kind !== 'settings') return [];
          var controls = Array.from(content.querySelector('#settings-' + settingsSection).querySelectorAll('button:not([role="radio"])'));
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
            return button.dataset.cmd ? window.dinahCommands.describe(button.dataset.cmd, label) : { label: label, disabled: button.disabled || button.getAttribute('aria-disabled') === 'true', run: function () { button.click(); } };
          });
          // A button with no words, such as a colour swatch, is no command.
          return commands.filter(function (command) { return command && command.label; });
        }
        var reload = { workspace: 'refresh-workspace', 'chat-browser': 'refresh-sessions' }[kind];
        return { state: state, setState: setState, commands: commands, reload: reload ? function () { window.dinahCommands.run(reload); } : undefined, destroy: function () {
          if (kind === 'settings') {
            content.removeEventListener('click', settingsClick);
            document.dispatchEvent(new CustomEvent('dinah:settings-section', { detail: { section: '' } }));
          }
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
