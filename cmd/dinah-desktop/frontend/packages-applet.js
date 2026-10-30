/* Package installation, updates and removal keep their confirmation views
   and notify the shell when an agent needs to read new skills. */
(function () {
  'use strict';

  function create(adapter) {
    const { app, on, toast, fail, productName, workspaceLabel, providerName, frontProvider, restartAgent, notices } = adapter;
    const $ = id => document.getElementById(id);

    /* ---- Packages (docs/design/packages.md) --------------------------------
       Settings › Packages lists the packages installed for the person and in
       the workspace in front, each with Remove…, and Install From a File…
       installs one from its ZIP. Update Packages… is there only when the
       build registered a package source, which Dinah's own build does not. */
    var packageSources = 0, installedPackages = [];
    function packageWhere(p) { return p.target === 'me' ? 'for you, in every workspace' : 'in ' + workspaceLabel(p.workspace); }
    function drawPackages() {
      var list = $('packages-installed');
      if (!list) return;
      document.querySelectorAll('[data-cmd="update-packages"]').forEach(function (b) { b.hidden = !packageSources; });
      window.dinahCommands.changed();
      if (!app() || !app().InstalledPackages) return;
      Promise.resolve(app().InstalledPackages()).then(function (found) {
        installedPackages = found || [];
        list.replaceChildren();
        installedPackages.forEach(function (p) {
          var li = document.createElement('li');
          li.className = 'hstack gap-3 align-center';
          var words = document.createElement('span');
          words.textContent = p.name + ' version ' + p.version + ', ' + packageWhere(p) + '.';
          var remove = document.createElement('button');
          remove.type = 'button'; remove.className = 'btn btn-sm'; remove.textContent = 'Remove…';
          remove.setAttribute('aria-label', 'Remove ' + p.name + ' ' + packageWhere(p) + '…');
          remove.addEventListener('click', function () { removePackage(p); });
          li.append(words, remove);
          list.appendChild(li);
        });
        list.hidden = !installedPackages.length;
        $('packages-none').hidden = !!installedPackages.length;
      }).catch(fail);
    }
    on('packages:changed', drawPackages);
    window.dinahHost.onResync(function () { return drawPackages(); });
    document.addEventListener('dinah:workspace-selected', drawPackages);

    /* Where a package goes: for the person, so every workspace has it, or into
       the workspace in front alone. Resolves to 'me', 'workspace' or null when
       cancelled. */
    function choosePackageTarget() {
      return new Promise(function (resolve) {
        var dialog = document.createElement('dialog'); dialog.className = 'dialog';
        dialog.setAttribute('aria-labelledby', 'install-target-title');
        dialog.innerHTML = '<h2 class="dialog-title" id="install-target-title">Install a Package</h2>' +
          '<div class="dialog-body"><div class="form-group"><label class="form-label" for="install-target">Install For</label>' +
          '<span class="form-select-wrap"><select class="form-select" id="install-target">' +
          '<option value="me">Me, in Every Workspace</option><option value="workspace">This Workspace Only</option></select></span>' +
          '<p class="form-help" id="install-target-help"></p></div></div>' +
          '<div class="dialog-actions"><button type="button" class="btn" data-no>Cancel</button><button type="button" class="btn btn-primary" data-yes>Choose the Package…</button></div>';
        var select = dialog.querySelector('select'), help = dialog.querySelector('#install-target-help'), chosen = null;
        // Without a workspace in front there is no workspace to install in.
        if (!adapter.workspace()) select.querySelector('option[value="workspace"]').remove();
        function explain() {
          help.textContent = select.value === 'me'
            ? 'Every workspace gets its skills, however it is set up.'
            : 'Only ' + workspaceLabel(adapter.workspace()) + ' gets them, as a copy in its main folder.';
        }
        select.addEventListener('change', explain);
        explain();
        dialog.querySelector('[data-no]').addEventListener('click', function () { dialog.close(); });
        dialog.querySelector('[data-yes]').addEventListener('click', function () { chosen = select.value; dialog.close(); });
        dialog.addEventListener('close', function () { dialog.remove(); resolve(chosen); }, { once: true });
        document.body.appendChild(dialog); dialog.showModal(); select.focus();
      });
    }
    /* The plan of an install or a removal, before it runs: what it does where,
       what it adds, replaces and removes, and what the person should know,
       each in a section of its own. Resolves to true on Install or Remove. */
    function showInstallPlan(title, plan, intro, where, removing) {
      return new Promise(function (resolve) {
        var dialog = document.createElement('dialog');
        dialog.className = 'dialog install-plan';
        dialog.setAttribute('aria-labelledby', 'install-plan-title');
        dialog.innerHTML = '<h2 class="dialog-title" id="install-plan-title"></h2><div class="dialog-body"></div>' +
          '<div class="dialog-actions"><button type="button" class="btn" data-no>Cancel</button><button type="button" class="btn" data-yes></button></div>';
        dialog.querySelector('.dialog-title').textContent = title;
        dialog.querySelector('[data-yes]').textContent = removing ? 'Remove' : 'Install';
        dialog.querySelector('[data-yes]').classList.add(removing ? 'btn-danger' : 'btn-primary');
        var body = dialog.querySelector('.dialog-body');
        function para(text, cls) { var p = document.createElement('p'); p.textContent = text; if (cls) p.className = cls; body.appendChild(p); return p; }
        function section(heading, text, items, cls) {
          var h = document.createElement('h3'); h.className = 'install-plan-heading'; h.textContent = heading; body.appendChild(h);
          if (text) para(text, 'form-help');
          var ul = document.createElement('ul'); ul.className = 'install-plan-list' + (cls ? ' ' + cls : '');
          items.forEach(function (it) { var li = document.createElement('li'); li.textContent = it; ul.appendChild(li); });
          body.appendChild(ul);
        }
        para(intro);
        var facts = document.createElement('dl'); facts.className = 'install-plan-facts';
        var rows = removing
          ? [['Version', plan.from], ['Files', plan.removed + ' removed'], ['Where', where]]
          : [['Version', plan.from ? plan.from + ' → ' + plan.to : plan.to], ['Files', plan.added + ' added, ' + plan.updated + ' updated, ' + plan.removed + ' removed'], ['Where', where]];
        rows.forEach(function (f) {
          if (!f[1]) return;
          var dt = document.createElement('dt'), dd = document.createElement('dd'); dt.textContent = f[0]; dd.textContent = f[1]; facts.append(dt, dd);
        });
        body.appendChild(facts);
        if ((plan.duplicates || []).length) section('Also in This Workspace', 'This workspace has its own copy of these skills. Remove one copy of each, since which one an agent uses is not documented.', plan.duplicates, 'install-plan-names');
        if ((plan.homeOnly || []).length) section('Kept in the Package\'s Folder', productName() + ' does not copy these to your own folders. Their files are installed in the package\'s folder, and the skills that use them find them there.', plan.homeOnly);
        if ((plan.changed || []).length) section('Changed by Hand', 'These files were changed since they were installed. They are ' + (removing ? 'removed' : 'replaced') + ', and a copy of each is kept first.', plan.changed);
        if ((plan.warnings || []).length) section('Also Note', '', plan.warnings);
        var answer = false;
        dialog.querySelector('[data-no]').addEventListener('click', function () { dialog.close(); });
        dialog.querySelector('[data-yes]').addEventListener('click', function () { answer = true; dialog.close(); });
        dialog.addEventListener('close', function () { dialog.remove(); resolve(answer); }, { once: true });
        document.body.appendChild(dialog); dialog.showModal(); dialog.querySelector('[data-yes]').focus();
      });
    }
    function installPackages() {
      return choosePackageTarget().then(function (target) {
        if (target === 'me') return installPersonalPackages();
        if (target === 'workspace') return installWorkspacePackages();
      }).catch(fail);
    }
    function installPersonalPackages(given) {
      var ws = adapter.workspace();
      return Promise.resolve().then(function () { return given || app().ChoosePackageBundle(); }).then(function (bundle) {
        if (!bundle) return;
        return app().PlanPersonalPackages(ws, bundle).then(function (plan) {
          if (plan.conflicts.length) {
            toast('The package cannot be installed for you. ' + productName() + ' did not install these, and will not overwrite them: ' + plan.conflicts.join('; ') + '.', 'danger');
            return;
          }
          var intro = (plan.from ? 'This updates ' + plan.name + ' for you, in every workspace.' : 'This installs ' + plan.name + ' for you, in every workspace.') +
            ' Its skills reach every workspace, however it is set up.';
          return showInstallPlan('Install ' + plan.name + ' for You?', plan, intro, plan.home + ', with copies in your own folders').then(function (yes) {
            if (!yes) return;
            return app().InstallPersonalPackages(ws, bundle, plan.token).then(function (done) {
              packagesChanged('Installed ' + done.name + ' ' + done.to + ' for you. Every workspace now has its skills.' + (done.backup ? ' Replaced files are kept in ' + done.backup + '.' : ''));
              return true;
            });
          });
        });
      });
    }
    function installWorkspacePackages(given) {
      if (!adapter.workspace()) { toast('Open the workspace to install the package in first.', 'warn'); return Promise.resolve(); }
      var ws = adapter.workspace();
      return Promise.resolve().then(function () { return given || app().ChoosePackageBundle(); }).then(function (bundle) {
        if (!bundle) return;
        return app().PlanPackages(ws, bundle).then(function (plan) {
          var intro = (plan.from ? 'This updates ' + plan.name + ' in ' + workspaceLabel(ws) + '.' : 'This installs ' + plan.name + ' in ' + workspaceLabel(ws) + '.') +
            ' Files of your own outside the package are never touched.';
          return showInstallPlan('Install ' + plan.name + ' in This Workspace?', plan, intro, ws).then(function (yes) {
            if (!yes) return;
            return app().InstallPackages(ws, bundle, plan.token).then(function (done) {
              packagesChanged('Installed ' + done.name + ' ' + done.to + ' in ' + workspaceLabel(ws) + '.' + (done.backup ? ' Replaced files are kept in ' + done.backup + '.' : ''));
              return true;
            });
          });
        });
      });
    }
    /* After a package is installed, updated or removed, the agent in front
       restarts to read the skills, unless a chat is working, which a restart
       would stop. */
    function packagesChanged(message) {
      drawPackages();
      var busy = adapter.hasWorkingChat();
      if (busy) {
        toast(message + ' A chat is working, so ' + providerName(frontProvider()) + ' reads the new skills when you restart it from its menu in the status bar.', 'positive');
        return;
      }
      toast(message + ' ' + providerName(frontProvider()) + ' restarts now to read the new skills.', 'positive');
      restartAgent();
    }
    /* Remove… beside an installed package takes out what its install put in,
       after showing what goes; every file is copied first, and nothing it did
       not install is touched. */
    function removePackage(p) {
      var ws = p.workspace || adapter.workspace();
      return Promise.resolve(app().PlanRemovePackages(p.target, ws, p.name)).then(function (plan) {
        var intro = p.target === 'me'
          ? 'This removes ' + plan.name + ' as it was installed for you, with the copies in your own folders. Workspaces with a copy of their own keep it.'
          : 'This removes ' + plan.name + ' from ' + workspaceLabel(ws) + '. Your other files are not touched.';
        return showInstallPlan(p.target === 'me' ? 'Remove ' + plan.name + ' for You?' : 'Remove ' + plan.name + ' From This Workspace?', plan, intro, p.target === 'me' ? plan.home + ', with copies in your own folders' : ws, true).then(function (yes) {
          if (!yes) return;
          return app().RemovePackages(p.target, ws, p.name, plan.token).then(function (done) {
            packagesChanged('Removed ' + done.name + (p.target === 'me' ? ' for you.' : ' from ' + workspaceLabel(ws) + '.') + (done.backup ? ' A copy of every file removed is kept in ' + done.backup + '.' : ''));
          });
        });
      }).catch(fail);
    }
    /* Update Packages… asks the build's package sources now, says what is
       installed and what is new, and has one button that installs the rest,
       each install showing its plan first. */
    function openPackageUpdates() {
      var old = $('package-update');
      if (old) old.remove();
      var dialog = document.createElement('dialog');
      dialog.id = 'package-update'; dialog.className = 'dialog update-dialog';
      dialog.setAttribute('aria-labelledby', 'package-update-title');
      dialog.innerHTML = '<h2 class="dialog-title" id="package-update-title">Update Packages</h2><div class="dialog-body" aria-live="polite"></div><div class="dialog-actions"></div>';
      dialog.addEventListener('close', function () { dialog.remove(); });
      document.body.appendChild(dialog); dialog.showModal();
      var body = dialog.querySelector('.dialog-body'), actions = dialog.querySelector('.dialog-actions');
      function say(text, cls) { var p = document.createElement('p'); p.textContent = text; if (cls) p.className = cls; body.appendChild(p); }
      function button(label, primary, run) {
        var b = document.createElement('button'); b.type = 'button'; b.className = 'btn' + (primary ? ' btn-primary' : ''); b.textContent = label;
        b.addEventListener('click', run); actions.appendChild(b); return b;
      }
      function close() { if (dialog.open) dialog.close(); }
      say('Looking for new versions of your packages…');
      button('Close', false, close).focus();
      return Promise.resolve(app().FindPackageUpdates()).then(function (status) {
        body.replaceChildren(); actions.replaceChildren();
        var list = document.createElement('ul'); list.className = 'update-list';
        var todo = [];
        (status.installed || []).forEach(function (p) {
          var u = (status.updates || []).find(function (x) { return x.target === p.target && x.name === p.name && (x.workspace || '') === (p.workspace || ''); });
          var li = document.createElement('li');
          li.textContent = p.name + ' version ' + p.version + ', ' + packageWhere(p) + (u ? ': version ' + u.to + ' is available.' : ': up to date.');
          if (u && u.reason) { var note = document.createElement('span'); note.className = 'form-help'; note.textContent = ' Before you update, note that ' + u.reason + '.'; li.appendChild(note); }
          list.appendChild(li);
          if (u) todo.push({ label: 'Update', run: function () { return u.target === 'me' ? installPersonalPackages(u.bundle) : (adapter.workspace() === u.workspace ? installWorkspacePackages(u.bundle) : Promise.reject(new Error('Open ' + workspaceLabel(u.workspace) + ' to update its package.'))); } });
        });
        (status.available || []).forEach(function (b) {
          var li = document.createElement('li');
          li.textContent = b.name + ' version ' + b.version + ' is available and not installed. Install puts its skills in every workspace, for you.';
          list.appendChild(li);
          todo.push({ label: 'Install', run: function () { return installPersonalPackages(b.bundle); } });
        });
        if (list.children.length) body.appendChild(list);
        if (status.off) say('Looking for new versions is turned off in the settings.', 'form-help');
        else if (!list.children.length) say('No package was found.');
        else if (!todo.length) say('Your packages are up to date.');
        if (!todo.length) { button('Close', false, close).focus(); return; }
        button('Later', false, close);
        button(todo.length === 1 ? todo[0].label + '…' : 'Update All…', true, function () {
          close();
          todo.reduce(function (done, step) { return done.then(function () { return step.run(); }); }, Promise.resolve()).catch(fail);
        }).focus();
      }).catch(function (err) {
        body.replaceChildren(); actions.replaceChildren();
        say(String(err && err.message || err)); button('Close', false, close).focus();
      });
    }
    on('packages:update-available', function (u) {
      var id = 'package-update-' + u.target + '-' + (u.workspace || '') + '-' + u.name + '-' + u.to;
      var heading = 'Version ' + u.to + ' of ' + u.name + ' is available.';
      // PUDL's download glyph also means bringing something here from
      // elsewhere (0.92.0), as installing a newer version of a package does.
      notices.add({ id: id, type: 'info', glyph: 'download', title: heading, description: heading,
        body: (u.target === 'me' ? 'You have version ' + u.from + '.' : workspaceLabel(u.workspace) + ' has version ' + u.from + '.') + (u.reason ? ' Before you update, note that ' + u.reason + '.' : ''),
        actions: [{ label: 'Update…', primary: true, run: function () { notices.remove(id); return openPackageUpdates(); } }],
        dismissible: true, announced: true, className: 'package-update' });
      toast('A new version of ' + u.name + ' is available. Use Update… in Notifications to install it.', 'info', { noted: true });
    });
    window.dinahCommands.register({ id: 'update-packages', menu: 'dinah', label: 'Update Packages…', visible: function () { return packageSources > 0; }, run: openPackageUpdates });

    return { installPackages, openPackageUpdates, hasSources: () => packageSources > 0, setSources: count => { packageSources = count; } };
  }

  window.dinahPackages = { create };
})();
