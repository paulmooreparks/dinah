/* Claude's own part of Settings, Agents: installing its adapter and choosing how it is billed (an API key,
   or the person's own Claude Code sign-in). It is the provider's own surface, so it names the provider and
   keeps its own markup and calls; the Settings page gives it the container in Claude's row of the list
   (settings-providers.js, row). */
(function () {
  'use strict';
  var api = null;

  function setup(row, p, ctx) {
    var state = ctx.status, app = api.app, settingsStatus = api.settingsStatus, ask = api.ask, productName = api.productName;
    var c = p.claude, box = document.createElement('div'); box.className = 'vstack gap-2 claude-setup'; row.appendChild(box);
    function replace(result) { ctx.replace(result); }
    if (!c.adapterInstalled && c.canInstall) {
      var install = document.createElement('button'); install.type = 'button'; install.className = 'btn btn-sm btn-primary'; install.textContent = 'Install the Claude Adapter';
      install.addEventListener('click', function () {
        install.disabled = true;
        settingsStatus(state, 'Installing the Claude adapter ' + c.adapterVersion + ' with npm. This takes a minute or two…');
        Promise.resolve().then(function () { return app().InstallClaudeAdapter(); }).then(function (result) {
          replace(result); settingsStatus('agents-status', 'The Claude adapter ' + c.adapterVersion + ' is installed.');
        }).catch(function (error) { settingsStatus(state, String(error), true); install.disabled = false; });
      });
      var installRow = document.createElement('div'); installRow.className = 'settings-actions'; installRow.appendChild(install); box.appendChild(installRow);
    }
    var heading = document.createElement('h4'); heading.className = 'card-subtitle'; heading.textContent = 'Billing'; box.appendChild(heading);
    var words = document.createElement('p'); words.className = 'form-help'; box.appendChild(words);
    if (c.apiKey) {
      words.textContent = 'Claude is billed to the API key you stored, in your system keychain. Remove it to use your own Claude Code sign-in instead.';
      var remove = document.createElement('button'); remove.type = 'button'; remove.className = 'btn btn-sm'; remove.textContent = 'Remove the Key';
      remove.addEventListener('click', function () {
        ask('Remove the Claude API Key?', 'Claude will use your own Claude Code sign-in from then on. Open Claude chats are disconnected and reconnect when you next send a message.', 'Remove the Key', true).then(function (yes) {
          if (!yes) return;
          remove.disabled = true;
          return Promise.resolve().then(function () { return app().SetClaudeAPIKey(''); }).then(function (result) { replace(result); settingsStatus('agents-status', 'The Claude API key was removed; Claude uses your own sign-in.'); });
        }).catch(function (error) { settingsStatus(state, String(error), true); remove.disabled = false; });
      });
      var removeRow = document.createElement('div'); removeRow.className = 'settings-actions'; removeRow.appendChild(remove); box.appendChild(removeRow);
      return;
    }
    words.textContent = 'Claude uses your own Claude Code sign-in: run claude in a terminal once and sign in with /login. ' + productName() + ' never asks for your Claude password. To bill an API key from the Claude Console instead, enter it here; it is kept in your system keychain.';
    var label = document.createElement('label'); label.className = 'form-label'; label.textContent = 'Claude API Key'; label.htmlFor = 'claude-api-key';
    var input = document.createElement('input'); input.className = 'form-input'; input.type = 'password'; input.id = 'claude-api-key'; input.autocomplete = 'off'; input.spellcheck = false;
    var save = document.createElement('button'); save.type = 'button'; save.className = 'btn btn-sm'; save.textContent = 'Use This Key'; save.disabled = true;
    input.addEventListener('input', function () { save.disabled = !input.value.trim(); });
    save.addEventListener('click', function () {
      save.disabled = true;
      Promise.resolve().then(function () { return app().SetClaudeAPIKey(input.value); }).then(function (result) {
        input.value = ''; replace(result); settingsStatus('agents-status', 'Claude is billed to the API key you stored.');
      }).catch(function (error) { settingsStatus(state, String(error), true); save.disabled = false; });
    });
    var keyRow = document.createElement('div'); keyRow.className = 'settings-actions'; keyRow.append(input, save);
    box.append(label, keyRow);
  }

  window.dinahProviderSettings.register('claude', {
    mount: function () { return {}; },
    row: function (container, provider, ctx) { api = ctx; if (provider.claude) setup(container, provider, ctx); }
  });
})();
