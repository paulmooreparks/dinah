/* Claude's own part of Settings, Agents: installing its adapter, choosing which release of the adapter runs,
   and choosing how Claude is billed (an API key, or the person's own Claude Code sign-in). It is the provider's
   own surface, so it names the provider and keeps its own markup and calls; the Settings page gives it the
   container in Claude's row of the list (settings-providers.js, row). */
(function () {
  'use strict';
  var api = null;

  function setup(row, p, ctx) {
    var state = ctx.status, app = api.app, settingsStatus = api.settingsStatus, ask = api.ask, productName = api.productName;
    var c = p.claude, box = document.createElement('div'); box.className = 'vstack gap-2 claude-setup'; row.appendChild(box);
    function replace(result) { ctx.replace(result); }
    function button(text, primary) {
      var b = document.createElement('button'); b.type = 'button'; b.className = 'btn btn-sm' + (primary ? ' btn-primary' : ''); b.textContent = text; return b;
    }
    function actions() { var r = document.createElement('div'); r.className = 'settings-actions'; box.appendChild(r); return r; }
    function subtitle(text) { var h = document.createElement('h4'); h.className = 'card-subtitle'; h.textContent = text; box.appendChild(h); }
    function help(text) { var w = document.createElement('p'); w.className = 'form-help'; w.textContent = text; box.appendChild(w); return w; }

    // The tested release is installed here when Claude has none, or runs on an earlier one after an update
    // and the app is not installing the tested one by itself.
    if (!c.adapterInstalled && c.canInstall && !c.adapterInstalling && !c.adapterNewer) {
      var install = button('Install the Claude Adapter', true);
      install.addEventListener('click', function () {
        install.disabled = true;
        settingsStatus(state, 'Installing the Claude adapter ' + c.adapterVersion + ' with npm. This takes a minute or two…');
        Promise.resolve().then(function () { return app().InstallClaudeAdapter(); }).then(function (result) {
          replace(result); settingsStatus('agents-status', 'The Claude adapter ' + c.adapterVersion + ' is installed.');
        }).catch(function (error) { settingsStatus(state, String(error), true); install.disabled = false; });
      });
      actions().appendChild(install);
    }
    billing();
    if (c.canInstall && (c.adapterInstalled || c.adapterNewer)) adapter();

    function billing() {
      subtitle('Billing');
      if (c.apiKey) {
        help('Claude is billed to the API key you stored, in your system keychain. Remove it to use your own Claude Code sign-in instead.');
        var remove = button('Remove the Key');
        remove.addEventListener('click', function () {
          ask('Remove the Claude API Key?', 'Claude will use your own Claude Code sign-in from then on. Open Claude chats are disconnected and reconnect when you next send a message.', 'Remove the Key', true).then(function (yes) {
            if (!yes) return;
            remove.disabled = true;
            return Promise.resolve().then(function () { return app().SetClaudeAPIKey(''); }).then(function (result) { replace(result); settingsStatus('agents-status', 'The Claude API key was removed; Claude uses your own sign-in.'); });
          }).catch(function (error) { settingsStatus(state, String(error), true); remove.disabled = false; });
        });
        actions().appendChild(remove);
        return;
      }
      help('Claude uses your own Claude Code sign-in: run claude in a terminal once and sign in with /login. ' + productName() + ' never asks for your Claude password. To bill an API key from the Claude Console instead, enter it here; it is kept in your system keychain.');
      var label = document.createElement('label'); label.className = 'form-label'; label.textContent = 'Claude API Key'; label.htmlFor = 'claude-api-key';
      var input = document.createElement('input'); input.className = 'form-input'; input.type = 'password'; input.id = 'claude-api-key'; input.autocomplete = 'off'; input.spellcheck = false;
      var save = button('Use This Key'); save.disabled = true;
      input.addEventListener('input', function () { save.disabled = !input.value.trim(); });
      save.addEventListener('click', function () {
        save.disabled = true;
        Promise.resolve().then(function () { return app().SetClaudeAPIKey(input.value); }).then(function (result) {
          input.value = ''; replace(result); settingsStatus('agents-status', 'Claude is billed to the API key you stored.');
        }).catch(function (error) { settingsStatus(state, String(error), true); save.disabled = false; });
      });
      box.appendChild(label);
      actions().append(input, save);
    }

    // Which release of the adapter Claude runs on. The tested release is the default. npm's newest one is
    // looked for only when the person asks, and the line that says what npm answered is always there, so
    // nothing below it moves when the answer comes.
    function adapter() {
      subtitle('Adapter');
      help(c.adapterNewer ?
        'Claude runs on the adapter ' + c.adapterInUse + ', which you chose. ' + productName() + ' was tested with ' + c.adapterVersion + '.' :
        'Claude runs on the adapter ' + c.adapterVersion + ', the release ' + productName() + ' was tested with. A newer release can offer new models sooner, and ' + productName() + ' was not tested with it.');
      var found = help('npm has not been asked for its newest release.'); found.className = 'form-help settings-answer claude-adapter-found'; found.setAttribute('role', 'status');
      var look = button('Look for a Newer Adapter'), use = button('Use the Newest Adapter'), newest = null;
      use.disabled = true;
      var buttons = actions(); buttons.append(look, use);
      function busy(on) { look.disabled = on; use.disabled = on || !newest; if (back) back.disabled = on; }
      look.addEventListener('click', function () {
        newest = null; busy(true); found.textContent = 'Asking npm for its newest release…';
        Promise.resolve().then(function () { return app().NewestClaudeAdapter(); }).then(function (r) {
          var words = 'The newest release on npm is ' + r.version + released(r.released);
          if (r.version === c.adapterInUse) words += ', the one Claude runs on.';
          else if (!r.newer) words += ', which is no newer than ' + r.tested + '.';
          else { words += '.'; newest = r.version; }
          found.textContent = words; busy(false);
        }).catch(function (error) { found.textContent = 'npm was not asked.'; settingsStatus(state, String(error), true); busy(false); });
      });
      use.addEventListener('click', function () {
        var version = newest;
        ask('Use the Claude Adapter ' + version + '?', productName() + ' was tested with ' + c.adapterVersion + ' and not with ' + version + ', so Claude may misbehave on it. You can return to the tested release here. Installing takes a minute or two. Open Claude chats are disconnected and reconnect when you next send a message.', 'Use ' + version, false).then(function (yes) {
          if (!yes) return;
          busy(true);
          settingsStatus(state, 'Installing the Claude adapter ' + version + ' with npm. This takes a minute or two…');
          return Promise.resolve().then(function () { return app().UseClaudeAdapter(version); }).then(function (result) {
            replace(result); settingsStatus('agents-status', 'Claude runs on the adapter ' + version + ' from its next start.');
          });
        }).catch(function (error) { settingsStatus(state, String(error), true); busy(false); });
      });
      var back = null;
      if (c.adapterNewer) {
        back = button('Return to the Tested Adapter');
        back.addEventListener('click', function () {
          ask('Return to the Claude Adapter ' + c.adapterVersion + '?', 'Claude will run on the release ' + productName() + ' was tested with. Open Claude chats are disconnected and reconnect when you next send a message.', 'Return to ' + c.adapterVersion, false).then(function (yes) {
            if (!yes) return;
            busy(true);
            return Promise.resolve().then(function () { return app().UseTestedClaudeAdapter(); }).then(function (result) {
              replace(result); settingsStatus('agents-status', 'Claude runs on the adapter ' + c.adapterVersion + ' from its next start.');
            });
          }).catch(function (error) { settingsStatus(state, String(error), true); busy(false); });
        });
        buttons.appendChild(back);
      }
    }
  }

  // released is when npm published a release, as words after its number, or nothing when npm did not say.
  function released(when) {
    var at = when ? new Date(Date.parse(when)) : null;
    if (!at || isNaN(at.getTime())) return '';
    return ', published on ' + at.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
  }

  window.dinahProviderSettings.register('claude', {
    mount: function () { return {}; },
    row: function (container, provider, ctx) { api = ctx; if (provider.claude) setup(container, provider, ctx); }
  });
})();
