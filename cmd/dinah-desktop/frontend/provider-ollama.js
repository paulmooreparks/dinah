/* Ollama's own part of Settings, Agents: when its tools ask the person, and where its file
   tools may go. It is the provider's own surface, so it names the provider and keeps its own
   markup and calls; the Settings page mounts it when the engine's list has Ollama in it
   (settings-providers.js). */
(function () {
  'use strict';

  var FILE_SCOPES = {
    anywhere: 'Ollama\'s file tools can read and write wherever you can.',
    workspace: 'Ollama\'s file tools stay in the chat\'s workspace folder and the local folders of its workspace\'s sources. A request outside them is refused.',
    folders: 'Ollama\'s file tools stay in the workspace folders and the folders listed here. A request outside them is refused.'
  };

  function mount(root, api) {
    var product = window.dinahProduct ? window.dinahProduct.name : '';
    root.innerHTML = `<section class="card vstack gap-3" aria-labelledby="set-ollama-approvals">
  <h3 class="card-title" id="set-ollama-approvals">Ollama Approvals</h3>
  <div class="seg" id="ollama-approvals" role="radiogroup" aria-labelledby="set-ollama-approvals">
    <button type="button" role="radio" data-value="chat" aria-checked="true" tabindex="0">Once in Each Chat</button>
    <button type="button" role="radio" data-value="ask" aria-checked="false" tabindex="-1">Every Command</button>
    <button type="button" role="radio" data-value="allow" aria-checked="false" tabindex="-1">Never</button>
    <button type="button" role="radio" data-value="all" aria-checked="false" tabindex="-1">Approve Everything</button>
  </div>
  <p class="form-help">${product} runs Ollama's tool calls itself, and asks before a shell command, a write outside the workspace, or an applet action that changes something. Once in Each Chat asks the first time and then remembers your answer for that chat, which the reconnect at startup keeps. Every Command asks about each command line and remembers only that exact line. Never does not ask at all: Ollama's commands run in your own shell, with no sandbox.</p>
  <p class="form-help">Approve Everything allows commands, writes outside the workspace and bundled applet calls without a question. Changes apply to open chats from the next turn. Existing Never settings continue to allow those actions. Questions that need your answer still wait for you.</p>
  <p class="settings-status" id="ollama-approvals-status" role="status"></p>
</section>
<section class="card vstack gap-3" aria-labelledby="set-ollama-files">
  <h3 class="card-title" id="set-ollama-files">Ollama File Access</h3>
  <div class="seg" id="ollama-file-scope" role="radiogroup" aria-labelledby="set-ollama-files">
    <button type="button" role="radio" data-value="anywhere" aria-checked="true" tabindex="0">Anywhere</button>
    <button type="button" role="radio" data-value="workspace" aria-checked="false" tabindex="-1">Workspace Folders</button>
    <button type="button" role="radio" data-value="folders" aria-checked="false" tabindex="-1">Workspace Folders and These</button>
  </div>
  <p class="form-help" id="ollama-file-describe"></p>
  <div class="vstack gap-2" id="ollama-file-folders-group" hidden>
    <ul class="row-list" id="ollama-file-folders" aria-label="Other folders Ollama's file tools may use"></ul>
    <div class="hstack gap-2">
      <input class="form-input" id="ollama-file-folder-input" type="text" aria-label="Full path of a folder to add" placeholder="Full path of a folder">
      <button type="button" class="btn" id="ollama-file-folder-add">Add Folder</button>
    </div>
  </div>
  <p class="form-help">This confines Ollama's file tools (read, write, list, find and search) and the folder a command starts in. It does not confine what a command reads once it runs, and a limit does not make commands ask. To confine commands as well, choose an approval level that asks, under Ollama Approvals. A new install starts with the workspace folders; an existing install keeps Anywhere until you change it.</p>
  <p class="settings-status" id="ollama-file-status" role="status"></p>
</section>
<section class="card vstack gap-3" aria-labelledby="set-ollama-rounds">
  <h3 class="card-title" id="set-ollama-rounds">Ollama Steps per Turn</h3>
  <div class="form-group">
    <label class="form-label" for="ollama-round-limit">Step Limit</label>
    <div class="hstack gap-2">
      <input class="form-input" id="ollama-round-limit" type="number" min="1" step="1" inputmode="numeric" placeholder="No Limit">
      <button type="button" class="btn btn-sm" id="ollama-round-save">Save</button>
      <button type="button" class="btn btn-sm" id="ollama-round-clear">No Limit</button>
    </div>
  </div>
  <p class="form-help">A turn is a round of work and the tool calls it makes, counted in steps. With no limit a turn runs until the model finishes. At the limit the turn stops at that count, the chat says so, and it offers Continue.</p>
  <p class="settings-status" id="ollama-round-status" role="status"></p>
</section>`;
    function $(id) { return root.querySelector('#' + id); }
    var app = api.app, settingsStatus = api.settingsStatus, reload = api.reloadSettingsAfterSave;

    /* Once in each chat, every command line, or never (ollama_approvals.go). */
    function setApprovals(policy, saved) {
      var control = $('ollama-approvals');
      policy = policy === 'ask' || policy === 'allow' || policy === 'all' ? policy : 'chat';
      control.querySelectorAll('[role="radio"]').forEach(function (button) {
        var checked = button.dataset.value === policy;
        button.setAttribute('aria-checked', String(checked)); button.tabIndex = checked ? 0 : -1;
      });
      if (saved !== false) control.dataset.saved = policy;
    }
    api.segmentKeyboard($('ollama-approvals'));
    $('ollama-approvals').addEventListener('click', function (event) {
      var button = event.target.closest('[role="radio"]');
      if (!button || button.disabled) return;
      var control = this, previous = control.dataset.saved || 'chat', policy = button.dataset.value;
      if (policy === previous) return;
      setApprovals(policy, false);
      control.querySelectorAll('button').forEach(function (item) { item.disabled = true; });
      settingsStatus('ollama-approvals-status', 'Saving…');
      Promise.resolve().then(function () { return app().SetOllamaApprovals(policy); }).then(function () {
        setApprovals(policy);
        settingsStatus('ollama-approvals-status', 'Saved. Open chats use this choice from the next turn.');
        return reload('ollama-approvals-status');
      }, function (error) {
        setApprovals(control.dataset.saved || previous);
        settingsStatus('ollama-approvals-status', 'The preference could not be saved: ' + error, true);
      }).finally(function () { control.querySelectorAll('button').forEach(function (item) { item.disabled = false; }); });
    });

    /* Anywhere, the workspace's folders, or those and a list of others; the engine checks each
       call against the choice, so a change applies to a running chat from its next tool call. A
       new install starts with the workspace folders, an existing one with Anywhere. */
    var fileScope = 'anywhere', fileFolders = [];
    function drawFileAccess() {
      var control = $('ollama-file-scope');
      control.querySelectorAll('[role="radio"]').forEach(function (button) {
        var checked = button.dataset.value === fileScope;
        button.setAttribute('aria-checked', String(checked)); button.tabIndex = checked ? 0 : -1;
      });
      $('ollama-file-describe').textContent = FILE_SCOPES[fileScope];
      $('ollama-file-folders-group').hidden = fileScope !== 'folders';
      var list = $('ollama-file-folders');
      list.replaceChildren();
      if (!fileFolders.length) {
        var none = document.createElement('li'); none.className = 'row-list-empty'; none.textContent = 'No other folders.'; list.appendChild(none);
      }
      fileFolders.forEach(function (path) {
        var li = document.createElement('li'), name = document.createElement('span'), remove = document.createElement('button');
        name.className = 'row-list-text'; name.textContent = path;
        remove.type = 'button'; remove.className = 'btn btn-sm'; remove.textContent = 'Remove'; remove.setAttribute('aria-label', 'Remove ' + path);
        remove.addEventListener('click', function () { saveFileAccess(fileScope, fileFolders.filter(function (p) { return p !== path; })); });
        li.append(name, remove); list.appendChild(li);
      });
    }
    function setFileAccess(scope, folders) {
      fileScope = scope === 'workspace' || scope === 'folders' ? scope : 'anywhere';
      fileFolders = Array.isArray(folders) ? folders.slice() : [];
      drawFileAccess();
    }
    function fileAccessBusy(on) {
      root.querySelectorAll('#ollama-file-scope button, #ollama-file-folders button, #ollama-file-folder-add, #ollama-file-folder-input').forEach(function (item) { item.disabled = on; });
    }
    function saveFileAccess(scope, folders) {
      var before = { scope: fileScope, folders: fileFolders.slice() };
      setFileAccess(scope, folders);
      fileAccessBusy(true);
      settingsStatus('ollama-file-status', 'Saving…');
      return Promise.resolve().then(function () { return app().SetOllamaFileScope(scope, folders); }).then(function () {
        settingsStatus('ollama-file-status', 'Saved. Open chats use this from the next tool call.');
        return reload('ollama-file-status');
      }, function (error) {
        setFileAccess(before.scope, before.folders);
        settingsStatus('ollama-file-status', 'The choice could not be saved: ' + error, true);
      }).finally(function () { fileAccessBusy(false); });
    }
    api.segmentKeyboard($('ollama-file-scope'));
    $('ollama-file-scope').addEventListener('click', function (event) {
      var button = event.target.closest('[role="radio"]');
      if (!button || button.disabled || button.dataset.value === fileScope) return;
      saveFileAccess(button.dataset.value, fileFolders);
    });
    function addFolder() {
      var input = $('ollama-file-folder-input'), path = input.value.trim();
      if (!path) { settingsStatus('ollama-file-status', 'Type the full path of a folder to add.', true); return; }
      if (fileFolders.indexOf(path) >= 0) { settingsStatus('ollama-file-status', 'That folder is already listed.', true); return; }
      input.value = '';
      saveFileAccess(fileScope, fileFolders.concat([path]));
    }
    $('ollama-file-folder-add').addEventListener('click', addFolder);
    $('ollama-file-folder-input').addEventListener('keydown', function (event) { if (event.key === 'Enter') { event.preventDefault(); addFolder(); } });
    /* Steps per turn (the round limit): none by default; empty field means
       none. A stop at the limit is shown with a Continue offer in the chat. */
    var rounds = { limit: 0, chosen: false };
    function setRounds(value) {
      rounds = value && typeof value.limit === 'number' ? { limit: value.limit, chosen: !!value.chosen } : { limit: 0, chosen: false };
      drawRounds();
    }
    function drawRounds() {
      var input = $('ollama-round-limit');
      if (document.activeElement !== input) input.value = rounds.limit > 0 ? String(rounds.limit) : '';
      $('ollama-round-clear').disabled = !rounds.chosen;
    }
    function roundsBusy(on) { ['ollama-round-limit', 'ollama-round-save', 'ollama-round-clear'].forEach(function (id) { $(id).disabled = on; }); }
    function saveRounds(n) {
      roundsBusy(true);
      settingsStatus('ollama-round-status', 'Saving…');
      Promise.resolve().then(function () { return app().SetOllamaRoundLimit(n); }).then(function () {
        settingsStatus('ollama-round-status', n > 0 ? 'Saved. A turn stops after ' + n + (n === 1 ? ' step' : ' steps') + ', and the chat offers to continue.' : 'Saved. A turn runs until the model finishes.');
        return reload('ollama-round-status');
      }, function (error) {
        settingsStatus('ollama-round-status', 'The limit could not be saved: ' + error, true);
      }).finally(function () { roundsBusy(false); drawRounds(); });
    }
    $('ollama-round-save').addEventListener('click', function () {
      var text = $('ollama-round-limit').value.trim(), n = text === '' ? 0 : Number(text);
      if (!Number.isInteger(n) || n < 1) { settingsStatus('ollama-round-status', 'Enter a whole number of steps, or leave the field empty for no limit.', true); return; }
      saveRounds(n);
    });
    $('ollama-round-limit').addEventListener('keydown', function (event) { if (event.key === 'Enter') { event.preventDefault(); $('ollama-round-save').click(); } });
    $('ollama-round-clear').addEventListener('click', function () { saveRounds(0); });
    setFileAccess('anywhere', []);

    // What State says, at start and after each save.
    return {
      apply: function (state) {
        setApprovals(state.ollamaApprovals || 'chat');
        if (state.ollamaFileScope) setFileAccess(state.ollamaFileScope, state.ollamaFileFolders);
        if (state.ollamaRoundLimit) setRounds(state.ollamaRoundLimit);
      }
    };
  }

  window.dinahProviderSettings.register('ollama', { mount: mount });
})();
