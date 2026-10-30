/* Settings owns its controls and save requests. The shell supplies provider
   preferences and the callbacks that update open chats. */
(function () {
  'use strict';

  function create(adapter) {
    const { app, on, toast, fail, ask, productName, providerName, providerOptions, optValues, byCat, valueWords, modelRows, loadModelPrices, drawAgentState } = adapter;
    const $ = id => document.getElementById(id);

    /* settingsStatus writes a Settings section's status line. A failure is a
       PUDL danger notice that stays until the next status, and it is a toast
       and a Notifications entry too, since the line is easy to miss; a line
       redrawn with a failure already reported passes quiet. */
    function settingsStatus(id, text, failed, quiet) {
      var el = typeof id === 'string' ? $(id) : id;
      if (!el) return;
      if (el.dataset.statusClass === undefined) el.dataset.statusClass = el.className;
      text = String(text || '').replace(/(^|: )Error: /, '$1');
      el.className = el.dataset.statusClass + (failed ? ' notice danger' : '');
      el.setAttribute('role', failed ? 'alert' : 'status');
      if (!failed) { el.textContent = text || ''; return; }
      var body = document.createElement('span');
      body.className = 'notice-content';
      body.textContent = text;
      el.replaceChildren(body);
      if (!quiet) toast(text, 'danger', { title: 'Settings:' });
    }
    /* reloadSettingsAfterSave reads Settings back after a save. The save has
       already happened, so a failure here says so and leaves the control as
       the person set it. */
    function reloadSettingsAfterSave(id) {
      return Promise.resolve().then(refreshSettingsPreferences).catch(function (error) {
        settingsStatus(id, 'The setting was saved, but Settings could not be read back: ' + error, true);
      });
    }

    /* ---- Settings ---------------------------------------------------------
       New Sessions sets the defaults every new session gets: the mode in a
       drop-down, and the model through the Default and Favorite Models
       dialog, which also keeps the favorites the menu over the prompt offers
       first. Settings shows each as one line, so that its page has no list
       scrolling inside it. The list of what Devin offers comes from the last
       session the app saw. */
    function modelName(o, value) {
      var v = optValues(o).filter(function (x) { return x.value === value; })[0];
      return v ? v.name : value;
    }
    function drawDefaults() {
      var host = $('defaults');
      host.innerHTML = '';
      if (!adapter.catalog().length) {
        $('defaults-help').textContent = 'Test the connection or open a session to load ' + providerName(adapter.defaultProvider()) + '\'s models.';
        return;
      }
      $('defaults-help').textContent = 'What every new session starts with. A session keeps its own choices after that.';
      adapter.catalog().forEach(function (o) {
        if (o.type !== 'select') return;
        if (o.category === 'model') {
          var line = document.createElement('div');
          line.className = 'form-group';
          line.innerHTML = '<span class="form-label">Model</span><div class="settings-line"><span></span><button type="button" class="btn btn-sm" data-open="model-prefs">Choose…</button></div>' +
            '<p class="form-help">Available models and thinking levels depend on the selected provider.</p>';
          line.querySelector('.settings-line > span').textContent = adapter.defaults()[o.id] ? modelName(o, adapter.defaults()[o.id]) : providerName(adapter.defaultProvider()) + '\'s own default';
          host.appendChild(line);
          return;
        }
        var vals = optValues(o);
        var g = document.createElement('div');
        g.className = 'form-group';
        g.innerHTML = '<label class="form-label"></label><span class="form-select-wrap"><select class="form-select"></select></span>';
        var label = g.querySelector('label'), sel = g.querySelector('select');
        label.textContent = o.category === 'mode' ? 'Mode' : o.name;
        label.htmlFor = sel.id = 'default-' + o.id;
        var none = document.createElement('option');
        none.value = ''; none.textContent = providerName(adapter.defaultProvider()) + '\'s own default';
        sel.appendChild(none);
        vals.forEach(function (v) {
          var opt = document.createElement('option');
          opt.value = v.value; opt.textContent = v.name + (v.description && o.category !== 'model' ? ': ' + v.description : '');
          opt.selected = adapter.defaults()[o.id] === v.value;
          sel.appendChild(opt);
        });
        sel.addEventListener('change', function () {
          app().SetDefault(o.id, sel.value ? sel.value : null).then(function (d) { adapter.setDefaults(d || {}); toast('New sessions will start with this.', 'positive'); }).catch(fail);
        });
        host.appendChild(g);
      });
    }
    function drawFavs() {
      var model = byCat(adapter.catalog(), 'model');
      var names = model ? adapter.pinned().filter(function (p) { return optValues(model).some(function (v) { return v.value === p; }); }).map(function (p) { return modelName(model, p); }) : [];
      $('favs-summary').textContent = !model ? 'Test the connection or open a session to load available models.'
        : names.length ? names.join(', ') : 'No favorites yet.';
      document.querySelectorAll('#settings [data-open="model-prefs"]').forEach(function (b) { b.disabled = false; });
    }

    function openSettings() {
      scanProviders();
      drawDefaults();
      drawFavs();
      window.dinahWorkbench.open('settings').catch(fail);
    }

    function setWindowModeControl(mode, saved) {
      var control = $('new-window-mode');
      control.querySelectorAll('[role="radio"]').forEach(function (button) {
        var checked = button.dataset.value === mode;
        button.setAttribute('aria-checked', String(checked)); button.tabIndex = checked ? 0 : -1;
      });
      if (saved !== false) control.dataset.saved = mode;
    }
    function segmentKeyboard(group) {
      group.addEventListener('keydown', function (event) {
        var button = event.target.closest('[role="radio"]');
        if (!button || button.disabled || event.altKey || event.ctrlKey || event.metaKey) return;
        var buttons = Array.from(group.querySelectorAll('[role="radio"]')), index = buttons.indexOf(button), next;
        if (event.key === 'Home') next = buttons[0];
        else if (event.key === 'End') next = buttons[buttons.length - 1];
        else if (['ArrowRight', 'ArrowDown', 'ArrowLeft', 'ArrowUp'].includes(event.key)) {
          var advance = event.key === 'ArrowRight' || event.key === 'ArrowDown';
          if (getComputedStyle(group).direction === 'rtl' && (event.key === 'ArrowRight' || event.key === 'ArrowLeft')) advance = !advance;
          next = buttons[(index + (advance ? 1 : -1) + buttons.length) % buttons.length];
        }
        if (next && !next.disabled) { event.preventDefault(); next.focus(); next.click(); }
      });
    }
    segmentKeyboard($('new-window-mode')); segmentKeyboard($('settings-theme'));
    /* Settings â€º Agents: the applet actions allowed always, each with Forget
       (applet_consent.go). */
    function drawAppletGrants() {
      var list = $('applet-grants');
      if (!list || !app().AppletGrants) return;
      Promise.resolve(app().AppletGrants()).then(function (grants) {
        list.replaceChildren();
        if (!grants || !grants.length) {
          var none = document.createElement('li');
          none.className = 'form-help';
          none.textContent = 'No applet action is allowed always.';
          list.append(none);
          return;
        }
        grants.forEach(function (g) {
          var row = document.createElement('li'), text = document.createElement('span'), forget = document.createElement('button');
          row.className = 'hstack gap-3 align-center';
          text.textContent = g.title + ' in ' + g.workspace;
          forget.type = 'button'; forget.className = 'btn btn-sm'; forget.textContent = 'Forget';
          forget.setAttribute('aria-label', 'Forget ' + g.title + ' in ' + g.workspace);
          forget.addEventListener('click', function () {
            forget.disabled = true;
            Promise.resolve(app().ForgetAppletGrant(g.workspace, g.action)).then(function () {
              settingsStatus('applet-grants-status', g.title + ' will be asked about again.');
              drawAppletGrants();
            }, function (error) {
              forget.disabled = false;
              settingsStatus('applet-grants-status', 'It could not be forgotten: ' + error, true);
            });
          });
          row.append(text, forget);
          list.append(row);
        });
      }).catch(function (error) { settingsStatus('applet-grants-status', 'The allowed actions could not be read: ' + error, true); });
    }
    /* Settings › Diagnostics: how much the log keeps (applog.go). */
    segmentKeyboard($('log-level'));
    function setLogLevelControl(level, saved) {
      var control = $('log-level');
      control.querySelectorAll('[role="radio"]').forEach(function (button) {
        var checked = button.dataset.value === level;
        button.setAttribute('aria-checked', String(checked)); button.tabIndex = checked ? 0 : -1;
      });
      if (saved !== false) control.dataset.saved = level;
    }
    $('log-level').addEventListener('click', function (event) {
      var button = event.target.closest('[role="radio"]');
      if (!button || button.disabled) return;
      var control = this, previous = control.dataset.saved || 'problems', level = button.dataset.value;
      if (level === previous) return;
      setLogLevelControl(level, false);
      control.querySelectorAll('button').forEach(function (item) { item.disabled = true; });
      Promise.resolve().then(function () { return app().SetLogLevel(level); }).then(function () {
        setLogLevelControl(level);
        settingsStatus('log-status', level === 'off' ? 'The log keeps nothing now.' : level === 'all' ? 'The log keeps everything now.' : 'The log keeps errors and warnings now.');
        return reloadSettingsAfterSave('log-status');
      }, function (error) { setLogLevelControl(control.dataset.saved || previous); settingsStatus('log-status', 'The preference could not be saved: ' + error, true); }).finally(function () {
        control.querySelectorAll('button').forEach(function (item) { item.disabled = false; });
      });
    });
    $('show-log-folder').addEventListener('click', function () {
      Promise.resolve().then(function () { return app().ShowLogFolder(); }).catch(function (error) { settingsStatus('log-status', 'The folder could not be shown: ' + error, true); });
    });
    $('new-window-mode').addEventListener('click', function (event) {
      var button = event.target.closest('[role="radio"]');
      if (!button || button.disabled) return;
      var control = this, previous = control.dataset.saved || 'floating', mode = button.dataset.value;
      if (mode === previous) return;
      setWindowModeControl(mode, false);
      control.querySelectorAll('button').forEach(function (item) { item.disabled = true; });
      Promise.resolve().then(function () { return app().SetNewWindowMode(mode); }).then(function () {
        setWindowModeControl(mode);
        settingsStatus('ui-settings-status', 'New windows will open ' + mode + '.');
        return reloadSettingsAfterSave('ui-settings-status');
      }, function (error) { setWindowModeControl(control.dataset.saved || previous); settingsStatus('ui-settings-status', 'The preference could not be saved: ' + error, true); }).finally(function () {
        control.querySelectorAll('button').forEach(function (item) { item.disabled = false; });
      });
    });

    /* A kind of window with a choice of its own has a row under Open New
       Windows: Like Other Windows follows it, and Reset returns the kind to the
       choice it ships with. */
    var WINDOW_KIND_LABELS = { editor: 'Editor Windows' };
    var windowKindChoices = [['same', 'Like Other Windows'], ['floating', 'Floating'], ['maximized', 'Maximized']];
    function drawWindowKindModes(modes, shipped) {
      var host = $('window-kind-modes');
      modes = modes || {}; shipped = shipped || {};
      if (window.dinahUIState && window.dinahUIState.setWindowModes) window.dinahUIState.setWindowModes(modes);
      Object.keys(shipped).sort().forEach(function (kind) {
        var row = host.querySelector('[data-kind="' + CSS.escape(kind) + '"]');
        if (!row) {
          row = document.createElement('div');
          row.className = 'vstack gap-2'; row.dataset.kind = kind;
          var label = document.createElement('span');
          label.className = 'form-label'; label.id = 'window-kind-' + kind + '-label';
          label.textContent = WINDOW_KIND_LABELS[kind] || kind.charAt(0).toUpperCase() + kind.slice(1) + ' Windows';
          var line = document.createElement('div'); line.className = 'settings-window-kind-row';
          var seg = document.createElement('div');
          seg.className = 'seg'; seg.setAttribute('role', 'radiogroup'); seg.setAttribute('aria-labelledby', label.id);
          windowKindChoices.forEach(function (choice) {
            var button = document.createElement('button');
            button.type = 'button'; button.setAttribute('role', 'radio'); button.dataset.value = choice[0]; button.textContent = choice[1];
            seg.appendChild(button);
          });
          var reset = document.createElement('button');
          reset.type = 'button'; reset.className = 'btn btn-sm'; reset.dataset.reset = ''; reset.textContent = 'Reset';
          line.append(seg, reset); row.append(label, line); host.appendChild(row);
          segmentKeyboard(seg);
          seg.addEventListener('click', function (event) {
            var button = event.target.closest('[role="radio"]');
            if (!button || button.disabled || button.dataset.value === row.dataset.saved) return;
            saveWindowKindMode(row, button.dataset.value);
          });
          reset.addEventListener('click', function () { saveWindowKindMode(row, ''); });
        }
        var mode = modes[kind] || shipped[kind];
        row.dataset.saved = mode; row.dataset.shipped = shipped[kind];
        markWindowKind(row, mode);
      });
    }
    function markWindowKind(row, mode) {
      row.querySelectorAll('[role="radio"]').forEach(function (button) {
        var checked = button.dataset.value === mode;
        button.setAttribute('aria-checked', String(checked)); button.tabIndex = checked ? 0 : -1;
      });
      row.querySelector('[data-reset]').disabled = mode === row.dataset.shipped;
    }
    function saveWindowKindMode(row, mode) {
      var previous = row.dataset.saved, controls = row.querySelectorAll('button');
      markWindowKind(row, mode || row.dataset.shipped);
      controls.forEach(function (item) { item.disabled = true; });
      var label = row.querySelector('.form-label').textContent;
      Promise.resolve().then(function () { return app().SetWindowKindMode(row.dataset.kind, mode); }).then(function () {
        var now = mode || row.dataset.shipped;
        row.dataset.saved = now;
        settingsStatus('ui-settings-status', label + (now === 'same' ? ' will open like other windows.' : ' will open ' + now + '.'));
        return reloadSettingsAfterSave('ui-settings-status');
      }, function (error) {
        row.dataset.saved = previous;
        settingsStatus('ui-settings-status', 'The preference could not be saved: ' + error, true);
      }).finally(function () {
        controls.forEach(function (item) { item.disabled = false; });
        markWindowKind(row, row.dataset.saved);
      });
    }

    function drawProviders() {
      drawAppletGrants();
      providerOptions($('default-provider'), adapter.defaultProvider());
      var host = $('agent-providers');
      host.replaceChildren();
      adapter.providers().forEach(function (p) {
        var row = document.createElement('section');
        row.className = 'card vstack gap-3'; row.dataset.provider = p.id;
        var heading = document.createElement('h3'); heading.className = 'card-title'; heading.textContent = p.name || providerName(p.id); row.appendChild(heading);
        var state = document.createElement('p'); state.className = 'form-help';
        // What is wrong is said in the words the engine gives for the provider's status; when nothing is, it is ready.
        state.textContent = (p.installed ? 'Ready' : p.statusWords || 'Not Ready') + (p.detail ? '. ' + p.detail : ''); row.appendChild(state);
        // The program Dinah starts for the provider, as the engine gives it (Claude's is Node.js, which runs its adapter).
        var facts = p.facts || {}, exeWord = facts.executable && facts.executable !== p.name ? facts.executable + ': ' : '';
        if (p.executableOverride || p.path) { var path = document.createElement('p'); path.className = 'form-help agent-path'; path.textContent = exeWord + (p.executableOverride ? 'Selected executable: ' + p.executableOverride : p.path); path.style.overflowWrap = 'anywhere'; row.appendChild(path); }
        var locationActions = document.createElement('div'); locationActions.className = 'settings-actions'; row.appendChild(locationActions);
        function locationAction(label, change) {
          var button = document.createElement('button'); button.type = 'button'; button.className = 'btn btn-sm'; button.textContent = label;
          button.addEventListener('click', function () {
            locationActions.querySelectorAll('button').forEach(function (b) { b.disabled = true; });
            Promise.resolve().then(change).then(function (changed) {
              if (changed === false) return;
              return scanProviders().then(function () { settingsStatus('agents-status', 'Executable selection saved. Restart ' + productName() + ' to apply it to an agent that is already connected.'); });
            }).catch(function (error) { settingsStatus(state, String(error), true); }).finally(function () { locationActions.querySelectorAll('button').forEach(function (b) { b.disabled = false; }); });
          });
          locationActions.appendChild(button);
        }
        // More than one Devin command line works: each is listed with what it
        // printed as its version, for the person to choose; the one whose
        // version looks newest is marked, and nothing is chosen for them.
        if ((p.choices || []).length > 1) {
          var choices = document.createElement('ul'); choices.className = 'agent-choices vstack gap-2'; choices.setAttribute('aria-label', (p.name || 'Agent') + ' command lines that work');
          p.choices.forEach(function (c) {
            var li = document.createElement('li'); li.className = 'agent-choice';
            var words = document.createElement('span'); words.className = 'form-help';
            words.textContent = c.path + (c.release ? ' (' + c.release + ')' : '') + (c.path === p.likely ? ', likely the current one' : '');
            words.style.overflowWrap = 'anywhere';
            var use = document.createElement('button'); use.type = 'button'; use.className = 'btn btn-sm' + (c.path === p.likely ? ' btn-primary' : ''); use.textContent = 'Use This One';
            use.addEventListener('click', function () {
              use.disabled = true;
              Promise.resolve().then(function () { return app().SetAgentExecutable(p.id, c.path); }).then(scanProviders).then(function () { settingsStatus('agents-status', (p.name || 'The agent') + ' will use ' + c.path + '.'); }).catch(function (error) { settingsStatus(state, String(error), true); use.disabled = false; });
            });
            li.append(words, use); choices.appendChild(li);
          });
          row.insertBefore(choices, locationActions);
        }
        // A provider that is a server and not a program Dinah starts has no executable to choose.
        if (facts.chooseExecutable) {
          locationAction(exeWord ? 'Choose ' + facts.executable + '…' : 'Choose Executable…', function () { return app().ChooseAgentExecutable(p.id); });
          if (p.executableOverride) locationAction('Use Automatic Discovery', function () { return app().SetAgentExecutable(p.id, ''); });
        }
        window.dinahProviderSettings.drawRow(row, p, { status: state, replace: replaceProviderEntry });
        drawFileLimit(row, p.id);
        drawSubstitutions(row, p.id);
        {
          var actions = document.createElement('div'); actions.className = 'settings-actions'; row.appendChild(actions);
          function action(label, method) {
            var button = document.createElement('button'); button.type = 'button'; button.className = 'btn btn-sm'; button.textContent = label; button.disabled = !p.installed;
            button.addEventListener('click', function () {
              actions.querySelectorAll('button').forEach(function (b) { b.disabled = true; });
              settingsStatus(state, label === 'Sign In' ? 'Complete sign-in in your browser, then test the connection.' : 'Checking the connection…');
              Promise.resolve().then(function () { return app()[method](p.id); }).then(function (result) {
                if (result && result.id) {
                  adapter.replaceProvider(result);
                  if (result.catalog) adapter.setProviderCatalog(p.id, result.catalog);
                  if (p.id === adapter.defaultProvider() && result.catalog) { adapter.setCatalog(result.catalog); drawDefaults(); drawFavs(); }
                  drawProviders();
                  adapter.refreshComposerOptions();
                } else { settingsStatus(state, 'Sign-in was requested. Test Connection checks whether it completed.'); }
              }).catch(function (error) { settingsStatus(state, String(error), true); }).finally(function () { actions.querySelectorAll('button').forEach(function (b) { b.disabled = false; }); });
            }); actions.appendChild(button);
          }
          action('Test Connection', 'AgentCheck');
          if (facts.signIn) action('Sign In', 'AgentSignIn');
        }
        host.appendChild(row);
      });
    }
    /* Files per Message on a provider's card (file_limits.go): the limit on
       the files the person attaches to one message, whose default comes from
       the provider's documentation, shown beside it with its source. An empty
       field is no limit, and Use Default returns to the documented default.
       setFileLimits takes the limits from State and fills every card's field,
       redraws each chat's chips, whose line says when the limit is reached,
       and tells the group windows, whose limit comes from their members. */
    function setFileLimits(limits) {
      adapter.setFileLimits(limits || {});
      document.querySelectorAll('[data-file-limit]').forEach(function (field) { fillFileLimit(field, adapter.fileLimits()[field.dataset.fileLimit]); });
      adapter.redrawAttachments();
      document.dispatchEvent(new CustomEvent('dinah:file-limits-changed'));
    }
    function drawFileLimit(row, provider) {
      var field = document.createElement('div'); field.className = 'vstack gap-2 file-limit'; field.dataset.fileLimit = provider;
      var id = 'file-limit-' + provider;
      var heading = document.createElement('h4'); heading.className = 'card-subtitle'; heading.textContent = 'Files per Message';
      var label = document.createElement('label'); label.className = 'visually-hidden'; label.htmlFor = id; label.textContent = providerName(provider) + ' Files per Message';
      var input = document.createElement('input'); input.className = 'form-input file-limit-input'; input.type = 'number'; input.min = '0'; input.step = '1'; input.id = id; input.placeholder = 'No Limit'; input.inputMode = 'numeric';
      var save = document.createElement('button'); save.type = 'button'; save.className = 'btn btn-sm'; save.textContent = 'Save'; save.dataset.save = '';
      var reset = document.createElement('button'); reset.type = 'button'; reset.className = 'btn btn-sm'; reset.textContent = 'Use Default'; reset.dataset.reset = '';
      var line = document.createElement('div'); line.className = 'settings-actions'; line.append(input, save, reset);
      var source = document.createElement('p'); source.className = 'form-help file-limit-source';
      var status = document.createElement('p'); status.className = 'settings-status'; status.setAttribute('role', 'status');
      field.append(heading, label, line, source, status);
      row.appendChild(field);
      function busy(on) { input.disabled = save.disabled = on; reset.disabled = on || !(adapter.fileLimits()[provider] && adapter.fileLimits()[provider].chosen); }
      function store(call, said) {
        busy(true);
        settingsStatus(status, 'Saving…');
        Promise.resolve().then(call).then(function () {
          settingsStatus(status, said());
          return reloadSettingsAfterSave(status);
        }, function (error) { settingsStatus(status, 'The limit could not be saved: ' + error, true); })
          .finally(function () { busy(false); fillFileLimit(field, adapter.fileLimits()[provider]); });
      }
      save.addEventListener('click', function () {
        var text = input.value.trim(), n = text === '' ? 0 : Number(text);
        if (!Number.isInteger(n) || n < 0) { settingsStatus(status, 'Enter a whole number of files, or leave the field empty for no limit.', true); return; }
        store(function () { return app().SetProviderFileLimit(provider, n); }, function () { return n ? providerName(provider) + ' messages take ' + n + (n === 1 ? ' file' : ' files') + ' at most.' : providerName(provider) + ' messages have no limit on their files.'; });
      });
      input.addEventListener('keydown', function (e) { if (e.key === 'Enter') { e.preventDefault(); save.click(); } });
      reset.addEventListener('click', function () {
        store(function () { return app().ResetProviderFileLimit(provider); }, function () { return providerName(provider) + ' uses its documented default again.'; });
      });
      fillFileLimit(field, adapter.fileLimits()[provider]);
    }
    /* Saved Setting Substitutions on a provider's card
       (option_substitutions.go): what the person chose, with Always in Open
       With…, to reopen this provider's chats with when a chat's saved value is
       not offered, each with Remove. The list is absent when there are none. */
    function setOptionSubstitutions(subs) {
      adapter.setOptionSubstitutions(subs || {});
      document.querySelectorAll('[data-substitutions]').forEach(function (field) { fillSubstitutions(field, field.dataset.substitutions); });
    }
    function drawSubstitutions(row, provider) {
      var field = document.createElement('div'); field.className = 'vstack gap-2 option-substitutions'; field.dataset.substitutions = provider;
      var heading = document.createElement('h4'); heading.className = 'card-subtitle'; heading.textContent = 'Saved Setting Substitutions';
      heading.id = 'option-substitutions-' + provider;
      var help = document.createElement('p'); help.className = 'form-help';
      help.textContent = "When a chat's saved setting is not offered and the replacement you chose is, the chat opens with the replacement and keeps its saved setting for when it is offered again.";
      var list = document.createElement('ul'); list.className = 'row-list'; list.setAttribute('aria-labelledby', heading.id);
      var status = document.createElement('p'); status.className = 'settings-status'; status.setAttribute('role', 'status');
      field.append(heading, help, list, status);
      row.appendChild(field);
      fillSubstitutions(field, provider);
    }
    function fillSubstitutions(field, provider) {
      var subs = adapter.optionSubstitutions()[provider] || [], list = field.querySelector('ul'), status = field.querySelector('.settings-status');
      field.hidden = !subs.length;
      list.replaceChildren.apply(list, subs.map(function (sub) {
        var li = document.createElement('li'); li.className = 'hstack gap-3 align-center';
        var text = document.createElement('span');
        text.textContent = (sub.optionName || sub.option) + ': ' + valueWords(sub.toName, sub.to) + ' when ' + valueWords(sub.fromName, sub.from) + " isn't offered.";
        var remove = document.createElement('button'); remove.type = 'button'; remove.className = 'btn btn-sm'; remove.textContent = 'Remove';
        remove.setAttribute('aria-label', 'Remove ' + text.textContent);
        remove.addEventListener('click', function () {
          remove.disabled = true;
          Promise.resolve().then(function () { return app().RemoveOptionSubstitution(provider, sub.option, sub.from); }).then(function () {
            settingsStatus('agents-status', 'Chats saved with ' + valueWords(sub.fromName, sub.from) + " will offer Open With again when it isn't offered.");
            return reloadSettingsAfterSave('agents-status');
          }, function (error) { remove.disabled = false; settingsStatus(status, 'The substitution could not be removed: ' + error, true); });
        });
        li.append(text, remove);
        return li;
      }));
    }
    function fillFileLimit(field, v) {
      field.hidden = !v;
      if (!v) return;
      var input = field.querySelector('input'), source = field.querySelector('.file-limit-source');
      if (document.activeElement !== input) input.value = v.limit > 0 ? String(v.limit) : '';
      if (v.ceiling > 0) input.max = String(v.ceiling); else input.removeAttribute('max');
      field.querySelector('[data-reset]').disabled = !v.chosen;
      source.replaceChildren();
      source.append((v.default > 0 ? 'Default: ' + v.default + '. ' : 'Default: no limit. ') + v.source);
      if (v.sourceUrl) {
        var link = document.createElement('a'); link.href = v.sourceUrl; link.textContent = v.sourceUrl; link.className = 'file-limit-link';
        link.addEventListener('click', function (e) {
          if (!(window.runtime && window.runtime.BrowserOpenURL)) return;
          e.preventDefault();
          window.runtime.BrowserOpenURL(v.sourceUrl);
        });
        source.append(' Source: ', link, '.');
      }
    }
    /* Claude's own part of its card (docs/proposals/claude-agent-integration.md):
       installing the pinned adapter, and who pays. Dinah never signs
       anyone in to Claude; with no API key stored, Claude uses the person's
       own Claude Code sign-in. */
    // The calls the provider modules (provider-ollama.js and the like) are lent.
    // A provider's entry changed (a key stored, an adapter installed): the list and the chat's options follow.
    function replaceProviderEntry(result) {
      if (result && result.id) adapter.replaceProvider(result);
      drawProviders();
      adapter.refreshComposerOptions();
    }
    window.dinahProviderSettings.configure({ ask: ask, productName: productName, app: app, toast: toast, fail: fail, settingsStatus: settingsStatus, reloadSettingsAfterSave: reloadSettingsAfterSave, segmentKeyboard: segmentKeyboard });
    function scanProviders() {
      if (!app().AgentProviders) return Promise.resolve();
      $('scan-agents').disabled = true;
      settingsStatus('agents-status', 'Looking for installed agents…');
      /* Only a failure of the engine's call is said as text for the person. An error in what the page does
         with the answer is a fault in the page, so it is not turned into that text: it goes on as a
         rejection, which the page reports as an error. */
      return app().AgentProviders().then(function (list) {
        adapter.setProviders(list || []);
        adapter.providers().forEach(function (p) { if (p.pinned) adapter.setProviderPins(p.id, p.pinned); });
        readAllProviderPreferences();
        drawProviders();
        window.dinahProviderSettings.draw(adapter.providers());
        settingsStatus('agents-status', 'Discovery finished. Installation alone does not confirm sign-in.');
        adapter.refreshComposerOptions();
      }, function (error) { settingsStatus('agents-status', 'Installed agents could not be found: ' + error, true); }).finally(function () { $('scan-agents').disabled = false; });
    }
    $('scan-agents').addEventListener('click', scanProviders);
    $('default-provider').addEventListener('change', function () {
      var select = $('default-provider'); select.disabled = true;
      app().SetDefaultProvider(select.value).then(refreshSettingsPreferences).then(function () {
        toast('New chats will use ' + providerName(adapter.defaultProvider()) + '.', 'positive');
      }).catch(function (error) { select.value = adapter.defaultProvider(); fail(error); }).finally(function () { select.disabled = false; });
    });

    // The Default and Favorite Models dialog, over Settings.
    var prefsProvider = '', prefsData = null, prefsRead = 0, prefsLoading = false, prefsMessage = '', prefsFailed = false;
    var prefsWriting = new Set(), prefsVersions = {};
    function cachePreferences(data) {
      var provider = data.provider;
      adapter.setProviderCatalog(provider, data.catalog || []);
      adapter.setProviderDefaults(provider, data.defaults || {});
      adapter.setProviderPins(provider, data.pinned || []);
      if (provider === adapter.defaultProvider()) {
        adapter.selectPreferences(provider);
        drawDefaults(); drawFavs();
      }
      adapter.refreshComposerOptions();
    }
    /* A provider's model catalog, defaults and favorites as the engine has them
       cached, read without connecting to the provider. The page is given only the
       default provider's catalog at start, so a new chat of another provider would
       have no Model control until a chat of that provider had run; reading them
       here gives it one as soon as the provider is chosen. A failure is quiet:
       the chat starts as it would have, and its own options arrive then. */
    function readProviderPreferences(provider) {
      if (!app().ProviderModelPreferences) return Promise.resolve();
      var version = prefsVersions[provider] || 0;
      return Promise.resolve().then(function () { return app().ProviderModelPreferences(provider); }).then(function (data) {
        if (!data || data.provider !== provider || prefsWriting.has(provider) || version !== (prefsVersions[provider] || 0)) return;
        cachePreferences({ provider: provider, catalog: data.catalog || [], defaults: data.defaults || {}, pinned: data.pinned || [] });
      }).catch(function () {});
    }
    function readAllProviderPreferences() {
      return Promise.all(adapter.providers().map(function (p) { return readProviderPreferences(p.id); }));
    }
    function prefsProviderOptions() {
      var select = $('prefs-provider'), choices = adapter.providers().slice();
      select.replaceChildren();
      choices.forEach(function (provider) {
        var option = document.createElement('option'); option.value = provider.id;
        option.textContent = provider.name || providerName(provider.id);
        select.appendChild(option);
      });
      select.value = prefsProvider;
    }
    function drawPrefs() {
      var provider = prefsProvider, name = providerName(provider), model = prefsData && byCat(prefsData.catalog, 'model');
      var cur = model && prefsData.defaults[model.id], busy = prefsWriting.has(provider);
      settingsStatus('prefs-status', prefsLoading ? 'Loading ' + name + ' model preferences…' : busy ? 'Saving ' + name + ' model preferences…' : prefsMessage, !prefsLoading && !busy && prefsFailed, true);
      $('prefs-status').setAttribute('aria-busy', String(prefsLoading || busy));
      $('prefs-default').textContent = model ? 'New ' + name + ' sessions start with ' + (cur ? modelName(model, cur) : name + '\'s own default') + '.' : '';
      $('prefs-own-default').textContent = 'Use ' + name + '\'s Own Default';
      $('prefs-own-default').disabled = !cur || prefsLoading || busy;
      $('prefs-filter').disabled = prefsLoading || !model;
      if (!model || prefsLoading) {
        $('prefs-list').replaceChildren();
        if (!prefsLoading && !prefsMessage) settingsStatus('prefs-status', 'No saved model catalog is available for ' + name + '. Use Test Connection under Settings > Agents to load its models. No prompt is sent.');
        return;
      }
      modelRows($('prefs-list'), model, {
        filter: $('prefs-filter').value, current: cur, provider: provider,
        pinned: prefsData.pinned, disabled: busy,
        choose: function (value) { writePrefs(provider, 'default', model.id, value); },
        pin: function (value, on) { return writePrefs(provider, 'pin', value, on); },
        redraw: drawPrefs
      });
    }
    function loadPrefs() {
      var provider = prefsProvider, request = ++prefsRead, version = prefsVersions[provider] || 0;
      prefsLoading = true; prefsMessage = ''; prefsFailed = false; prefsData = null; drawPrefs();
      return Promise.resolve().then(function () {
        if (!app().ProviderModelPreferences) throw new Error('Provider model preferences are unavailable.');
        return app().ProviderModelPreferences(provider);
      }).then(function (data) {
        if (request !== prefsRead || provider !== prefsProvider || !$('model-prefs').open) return;
        if (version !== (prefsVersions[provider] || 0)) return loadPrefs();
        if (!data || data.provider !== provider) throw new Error('The agent returned model preferences for a different provider.');
        prefsData = { provider: provider, catalog: data.catalog || [], defaults: data.defaults || {}, pinned: data.pinned || [] };
        cachePreferences(prefsData); prefsLoading = false; drawPrefs();
      }).catch(function (error) {
        if (request !== prefsRead || provider !== prefsProvider || !$('model-prefs').open) return;
        prefsLoading = false; prefsMessage = providerName(provider) + ' model preferences could not be loaded. ' + String(error); prefsFailed = true; fail(prefsMessage); drawPrefs();
      });
    }
    function writePrefs(provider, kind, id, value) {
      if (prefsWriting.has(provider)) return Promise.resolve();
      prefsWriting.add(provider); prefsVersions[provider] = (prefsVersions[provider] || 0) + 1;
      prefsMessage = ''; prefsFailed = false; drawPrefs();
      return Promise.resolve().then(function () {
        return kind === 'default' ? app().SetDefaultForProvider(provider, id, value) : app().SetPinnedForProvider(provider, id, value);
      }).then(function (result) {
        var data = { provider: provider, catalog: adapter.providerCatalogs()[provider] || [], defaults: adapter.providerDefaults()[provider] || {}, pinned: adapter.providerPins()[provider] || [] };
        if (kind === 'default') data.defaults = result || {}; else data.pinned = result || [];
        cachePreferences(data);
        if (provider === prefsProvider && $('model-prefs').open) { prefsData = data; prefsMessage = providerName(provider) + ' model preferences saved.'; }
      }).catch(function (error) {
        var message = providerName(provider) + ' model preferences could not be saved. ' + String(error);
        if (provider === prefsProvider && $('model-prefs').open) { prefsMessage = message; prefsFailed = true; }
        fail(message);
      }).finally(function () {
        prefsVersions[provider] = (prefsVersions[provider] || 0) + 1;
        prefsWriting.delete(provider); drawPrefs();
      });
    }
    function openPrefs() {
      if (!prefsProvider) prefsProvider = adapter.defaultProvider();
      prefsProviderOptions();
      $('prefs-filter').value = '';
      if (!$('model-prefs').open) $('model-prefs').showModal();
      loadPrefs(); $('prefs-provider').focus();
      loadModelPrices(prefsProvider);
    }
    $('prefs-provider').addEventListener('change', function () { prefsProvider = this.value; $('prefs-filter').value = ''; loadPrefs(); loadModelPrices(prefsProvider); });
    $('prefs-filter').addEventListener('input', drawPrefs);
    $('prefs-own-default').addEventListener('click', function () { var model = prefsData && byCat(prefsData.catalog, 'model'); if (model) writePrefs(prefsProvider, 'default', model.id, null); });
    $('model-prefs').addEventListener('close', function () { prefsRead++; drawFavs(); drawDefaults(); });
    document.addEventListener('click', function (e) {
      var b = e.target.closest && e.target.closest('[data-open="model-prefs"]');
      if (b) openPrefs();
    });

    var savedThinking = false, thinkingSaving = false, shownThinking = false;
    function setThinking(on, saved) {
      shownThinking = on;
      if (saved !== false) savedThinking = on;
      document.body.classList.toggle('show-thoughts', on);
      $('set-thinking').setAttribute('aria-checked', String(on));
      window.dinahCommands.changed();
    }
    function saveThinking(value) {
      if (thinkingSaving) return;
      thinkingSaving = true;
      setThinking(value, false);
      $('set-thinking').disabled = true;
      window.dinahCommands.changed();
      Promise.resolve().then(function () { return app().SetShowThinking(value); }).then(refreshSettingsPreferences).catch(function (error) {
        setThinking(savedThinking); fail(error);
      }).finally(function () {
        thinkingSaving = false; $('set-thinking').disabled = false;
        window.dinahCommands.changed();
      });
    }
    $('set-thinking').addEventListener('click', function () { saveThinking($('set-thinking').getAttribute('aria-checked') !== 'true'); });
    /* Notify Me When I Am Needed turns the system's notifications on or off
       (host_notices.go). */
    function setNotifyNeeded(on) { $('set-notify-needed').setAttribute('aria-checked', String(!!on)); }
    $('set-notify-needed').addEventListener('click', function () {
      var button = $('set-notify-needed'), value = button.getAttribute('aria-checked') !== 'true';
      if (!app().SetNotifyWhenNeeded) return;
      button.disabled = true; setNotifyNeeded(value);
      Promise.resolve().then(function () { return app().SetNotifyWhenNeeded(value); }).then(refreshSettingsPreferences)
        .catch(function (error) { setNotifyNeeded(!value); fail(error); }).finally(function () { button.disabled = false; });
    });
    /* The system's notifications are shown only while the window is not in
       front. Where Go cannot ask the system (Linux), the page's own focus
       decides: document.hasFocus() is false while the window does not have
       system focus (HTML Standard, "has focus steps"). */
    function reportFocus() {
      if (app() && app().ReportWindowFocus) Promise.resolve(app().ReportWindowFocus(document.hasFocus() && !document.hidden)).catch(function () {});
    }
    window.addEventListener('focus', reportFocus);
    window.addEventListener('blur', reportFocus);
    document.addEventListener('visibilitychange', reportFocus);

    /* The key that sends a prompt (Settings › Chat, prompt_keys.go): Enter,
       with Shift+Enter for a new line, or Ctrl+Enter, with Enter for one.
       Ctrl+Enter sends under either. Every window's box reads it from the
       shared composer (composer.js). */
    function setSendKey(value) {
      window.dinahComposer.setSendKey(value);
      $('set-send-key').value = window.dinahComposer.sendKey();
    }
    function sendKeyName() { return window.dinahComposer.sendKeyName(); }
    /* The person's name (Settings › Chat, person_name.go): what a delivered
       message's heading says in place of the person's bus handle. Empty uses
       the account's name, which the field shows as its placeholder. */
    function setPersonName(chosen, account, handle) {
      if (typeof chosen === 'string' && document.activeElement !== $('set-person-name')) $('set-person-name').value = chosen;
      if (typeof account === 'string') $('set-person-name').placeholder = account;
      var before = JSON.stringify(window.dinahPerson || {});
      window.dinahPerson = { chosen: $('set-person-name').value.trim(), account: $('set-person-name').placeholder, handle: handle || (window.dinahPerson || {}).handle || '' };
      if (JSON.stringify(window.dinahPerson) !== before) document.dispatchEvent(new CustomEvent('dinah:person-name'));
    }
    $('set-person-name').addEventListener('change', function () {
      var field = this, value = field.value.trim();
      if (!app().SetPersonName) return;
      Promise.resolve().then(function () { return app().SetPersonName(value); }).then(function () {
        setPersonName(value);
        return refreshSettingsPreferences();
      }).catch(fail);
    });
    $('set-send-key').addEventListener('change', function () {
      var select = this, before = window.dinahComposer.sendKey(), value = select.value;
      if (!app().SetPromptSendKey) return;
      select.disabled = true; setSendKey(value);
      Promise.resolve().then(function () { return app().SetPromptSendKey(value); }).then(refreshSettingsPreferences)
        .catch(function (error) { setSendKey(before); fail(error); }).finally(function () { select.disabled = false; });
    });

    /* Removing old attachments (Settings › Chat, bus_views.go): the files the
       person attached to group chats' messages, older than the age chosen.
       The count and the space come first, in the question, and nothing is
       removed until the person says yes; the result says what was freed. */
    function fileCount(n) { return n === 1 ? '1 file' : n + ' files'; }
    $('set-attachments-remove').addEventListener('click', function () {
      var button = this, days = parseInt($('set-attachment-age').value, 10) || 0;
      var age = days ? 'older than ' + $('set-attachment-age').selectedOptions[0].textContent.toLowerCase() : 'of any age';
      if (!app().OldAttachments || !app().RemoveOldAttachments) return;
      button.disabled = true;
      Promise.resolve().then(function () { return app().OldAttachments(days); }).then(function (n) {
        if (!n || !n.files) { toast('No attachments ' + age + ' are kept, so there is nothing to remove.'); return; }
        return ask('Remove ' + n.files + (n.files === 1 ? ' Attachment?' : ' Attachments?'),'The ' + fileCount(n.files) + ' you attached to group chats\' messages ' + age + ' take ' + window.dinahComposer.sizeWords(n.bytes) + '. They are deleted for good. Their messages stay, and show each file as removed.', 'Remove Attachments', true).then(function (yes) {
          if (!yes) return;
          return Promise.resolve(app().RemoveOldAttachments(days)).then(function (done) {
            toast('Removed ' + fileCount(done.files) + ', freeing ' + window.dinahComposer.sizeWords(done.bytes) + '.', 'positive');
          });
        });
      }).catch(fail).finally(function () { button.disabled = false; });
    });

    /* The view a Markdown document opens in when it has none of its own yet
       (Settings › Appearance, document_views.go). */
    function setDocumentViews(views) {
      if (!views) return;
      if (window.dinahDocumentViews) window.dinahDocumentViews(views);
      if (views['text/markdown']) $('set-markdown-view').value = views['text/markdown'];
    }
    $('set-markdown-view').addEventListener('change', function () {
      var select = this, value = select.value;
      if (!app().SetDocumentView) return;
      select.disabled = true;
      Promise.resolve().then(function () { return app().SetDocumentView('text/markdown', value); }).then(refreshSettingsPreferences)
        .catch(function (error) { refreshSettingsPreferences(); fail(error); }).finally(function () { select.disabled = false; });
    });

    var settingsRefreshGeneration = 0;
    function refreshSettingsPreferences() {
      var generation = ++settingsRefreshGeneration;
      return Promise.resolve().then(function () { return app().State(); }).then(function (state) {
        if (generation !== settingsRefreshGeneration) return;
        if (Array.isArray(state.workspaceRecords)) {
          adapter.setWorkspaceRecords(state.workspaceRecords);
        }
        if (state.newWindowMode === 'floating' || state.newWindowMode === 'maximized') {
          setWindowModeControl(state.newWindowMode);
          if (window.dinahUIState) window.dinahUIState.setNewWindowMode(state.newWindowMode);
        }
        if (state.windowModesShipped) drawWindowKindModes(state.windowModes, state.windowModesShipped);
        if (typeof state.notifyWhenNeeded === 'boolean') setNotifyNeeded(state.notifyWhenNeeded);
        if (state.promptSendKey) setSendKey(state.promptSendKey);
        setPersonName(state.personName, state.accountName, state.busHandle);
        window.dinahProviderSettings.apply(state);
        if (state.fileLimits) setFileLimits(state.fileLimits);
        if (state.optionSubstitutions) setOptionSubstitutions(state.optionSubstitutions);
        setDocumentViews(state.documentViews);
        if (typeof state.showThinking === 'boolean') setThinking(state.showThinking);
        if (state.logLevel) setLogLevelControl(state.logLevel);
        if (state.defaultProvider && state.defaultProvider !== adapter.defaultProvider()) {
          adapter.applyDefaultProvider(state);
          drawProviders(); drawDefaults(); drawFavs();
          adapter.updateDefaultProvider();
          drawAgentState();
        }
      });
    }
    on('settings:changed', function () { refreshSettingsPreferences().then(readAllProviderPreferences).catch(fail); });
    window.dinahHost.onResync(function () { return refreshSettingsPreferences().then(readAllProviderPreferences); });

    return { readProviderPreferences, thinkingState: () => ({ checked: shownThinking, busy: thinkingSaving }), drawDefaults, openSettings, setWindowModeControl, drawWindowKindModes, setFileLimits, setOptionSubstitutions, scanProviders, drawPrefs, setThinking, saveThinking, setNotifyNeeded, reportFocus, setSendKey, sendKeyName, setPersonName, setDocumentViews, settingsStatus };
  }

  window.dinahSettings = { create };
})();
