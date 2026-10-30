/* Codex's own part of Settings, Agents: how far its applet calls and its sandbox may go without a
   question. It is the provider's own surface, so it names the provider and keeps its own markup and
   calls; the Settings page mounts it when the engine's list has Codex in it (settings-providers.js). */
(function () {
  'use strict';

  function mount(root, api) {
    var product = window.dinahProduct ? window.dinahProduct.name : '';
    root.innerHTML = `<section class="card vstack gap-3" aria-labelledby="set-codex-applet-approvals">
  <h3 class="card-title" id="set-codex-applet-approvals">Codex Approvals</h3>
  <div class="seg" id="codex-applet-approvals" role="radiogroup" aria-labelledby="set-codex-applet-approvals">
    <button type="button" role="radio" data-value="ask" aria-checked="true" tabindex="0">Ask Before Invoking</button>
    <button type="button" role="radio" data-value="allow" aria-checked="false" tabindex="-1">Allow Bundled Applet Calls</button>
    <button type="button" role="radio" data-value="all" aria-checked="false" tabindex="-1">Approve Everything</button>
  </div>
  <p class="form-help">Allowing calls applies only to ${product}'s bundled applets through its managed connection. Shell commands, external tools and applet questions that require your answer keep their own approval rules.</p>
  <p class="form-help">Approve Everything allows commands, writes outside the workspace and bundled applet calls without a question. Changes apply to open chats; the sandbox changes with the next turn. Applet connection configuration also applies when a chat reconnects. Questions that need your answer still wait for you.</p>
  <p class="settings-status" id="codex-applet-approvals-status" role="status"></p>
</section>
<section class="card vstack gap-3" aria-labelledby="set-codex-approvals">
  <h3 class="card-title" id="set-codex-approvals">Codex Sandbox</h3>
  <div class="seg" id="codex-approvals" role="radiogroup" aria-labelledby="set-codex-approvals">
    <button type="button" role="radio" data-value="ask" aria-checked="true" tabindex="0">Ask Before Commands</button>
    <button type="button" role="radio" data-value="workspace" aria-checked="false" tabindex="-1">Workspace, Unattended</button>
    <button type="button" role="radio" data-value="full" aria-checked="false" tabindex="-1">Full Access, Unattended</button>
  </div>
  <p class="form-help">Workspace, Unattended lets Codex run commands and change files in the workspace without a question, so you can start it and walk away. A command that would write outside the workspace or reach the network is refused, not put to you. Full Access, Unattended also allows those, with no sandbox at all.</p>
  <p class="form-help">This choice applies to new chats and chats you reconnect. Approve Everything overrides it with full access. It also covers the applet actions of those chats, which are otherwise asked about here.</p>
  <p class="settings-status" id="codex-approvals-status" role="status"></p>
</section>`;
    function $(id) { return root.querySelector('#' + id); }
    var app = api.app, settingsStatus = api.settingsStatus, reload = api.reloadSettingsAfterSave;

    function setApplet(mode, saved) {
      var control = $('codex-applet-approvals');
      mode = mode === 'allow' || mode === 'all' ? mode : 'ask';
      control.querySelectorAll('[role="radio"]').forEach(function (button) {
        var checked = button.dataset.value === mode;
        button.setAttribute('aria-checked', String(checked)); button.tabIndex = checked ? 0 : -1;
      });
      if (saved !== false) control.dataset.saved = mode;
    }
    api.segmentKeyboard($('codex-applet-approvals'));
    $('codex-applet-approvals').addEventListener('click', function (event) {
      var button = event.target.closest('[role="radio"]');
      if (!button || button.disabled) return;
      var control = this, previous = control.dataset.saved || 'ask', mode = button.dataset.value;
      if (mode === previous) return;
      setApplet(mode, false);
      control.querySelectorAll('button').forEach(function (item) { item.disabled = true; });
      settingsStatus('codex-applet-approvals-status', 'Saving…');
      Promise.resolve().then(function () { return app().SetCodexAppletApprovals(mode); }).then(function () {
        setApplet(mode);
        settingsStatus('codex-applet-approvals-status', 'Saved. Open chats use the new sandbox from the next turn. Reconnect to update applet connection configuration.');
        return reload('codex-applet-approvals-status');
      }, function (error) {
        setApplet(control.dataset.saved || previous);
        settingsStatus('codex-applet-approvals-status', 'The preference could not be saved: ' + error, true);
      }).finally(function () { control.querySelectorAll('button').forEach(function (item) { item.disabled = false; }); });
    });

    /* How far a Codex chat may go without asking, so that it can run unattended (codex_approvals.go). */
    function setSandbox(policy, saved) {
      var control = $('codex-approvals');
      policy = policy === 'workspace' || policy === 'full' ? policy : 'ask';
      control.querySelectorAll('[role="radio"]').forEach(function (button) {
        var checked = button.dataset.value === policy;
        button.setAttribute('aria-checked', String(checked)); button.tabIndex = checked ? 0 : -1;
      });
      if (saved !== false) control.dataset.saved = policy;
    }
    api.segmentKeyboard($('codex-approvals'));
    $('codex-approvals').addEventListener('click', function (event) {
      var button = event.target.closest('[role="radio"]');
      if (!button || button.disabled) return;
      var control = this, previous = control.dataset.saved || 'ask', policy = button.dataset.value;
      if (policy === previous) return;
      setSandbox(policy, false);
      control.querySelectorAll('button').forEach(function (item) { item.disabled = true; });
      settingsStatus('codex-approvals-status', 'Saving…');
      Promise.resolve().then(function () { return app().SetCodexApprovals(policy); }).then(function () {
        setSandbox(policy);
        settingsStatus('codex-approvals-status', 'Saved for new chats and for chats you reconnect.');
        return reload('codex-approvals-status');
      }, function (error) {
        setSandbox(control.dataset.saved || previous);
        settingsStatus('codex-approvals-status', 'The preference could not be saved: ' + error, true);
      }).finally(function () { control.querySelectorAll('button').forEach(function (item) { item.disabled = false; }); });
    });

    // What State says, at start and after each save.
    return {
      apply: function (state) {
        setSandbox(state.codexApprovals || 'ask');
        if (state.codexAppletApprovals) setApplet(state.codexAppletApprovals);
      }
    };
  }

  window.dinahProviderSettings.register('codex', { mount: mount });
})();
