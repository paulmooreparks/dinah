/* Settings observes the installed update service and package operations.
   Release selection, validation, placement and repair stay with their owners. */
(function () {
  'use strict';
  var $ = id => document.getElementById(id);
  var app = () => window.dinahHelpers.app();
  var message = error => window.dinahHelpers.message(error);
  var service = window.dinahServices.object('updates');
  var gateway = null, release = null, releaseFailure = '', inventory = null;
  var sequences = {}, busy = {}, timer = null, preview = null, packageState = null, sourceCount = 0;
  var programs = [], runningRows = new Map(), packageRows = new Map(), repairRows = new Map();

  function node(tag, cls, text) {
    var el = document.createElement(tag); el.className = cls || '';
    if (text !== undefined) el.textContent = text;
    return el;
  }
  function status(id, words, failed) {
    var el = $(id); el.textContent = words; el.classList.toggle('danger', !!failed);
    el.setAttribute('role', failed ? 'alert' : 'status');
  }
  function row(host, key, withButton) {
    var el = node('div', 'updates-row'), words = node('div', 'updates-row-text');
    el.dataset.key = key; words.tabIndex = 0; el.appendChild(words);
    var button = null;
    if (withButton) { button = node('button', 'btn btn-sm', 'Update'); button.type = 'button'; button.disabled = true; el.appendChild(button); }
    host.appendChild(el); return { el: el, words: words, button: button };
  }
  function available() {
    if (!gateway) gateway = Promise.resolve().then(function () {
      return app() && app().ProductServices ? app().ProductServices() : [];
    }).then(function (names) {
      if (!(names || []).includes('updates')) throw new Error('This build has no configured release or agent-settings service.');
      return service;
    }).catch(function (error) { gateway = null; throw error; });
    return gateway;
  }
  // Requests may overlap on reopening. Only the newest request in a section
  // can acknowledge its state, failure or busy controls.
  function request(section, task, done, failed, background) {
    var seq = sequences[section] = (sequences[section] || 0) + 1;
    if (!background) { busy[section] = true; controls(); }
    return Promise.resolve().then(task).then(function (value) {
      if (seq === sequences[section]) done(value);
    }).catch(function (error) {
      if (seq === sequences[section]) failed(error);
    }).finally(function () {
      if (!background && seq === sequences[section]) { busy[section] = false; controls(); }
    });
  }
  // PUDL's aria-disabled button keeps keyboard focus during an operation.
  // Its disabled state is also checked here for keyboard/programmatic clicks.
  function disableAction(el, disabled) { el.disabled = false; el.setAttribute('aria-disabled', String(!!disabled)); }
  function actionDisabled(el) { return el.getAttribute('aria-disabled') === 'true'; }
  function controls() {
    $('updates-refresh').disabled = !!busy.inventory;
    $('updates-check').disabled = !!busy.release || !!busy.install || !!(release && release.downloading);
    disableAction($('updates-install'), !!busy.release || !!busy.install || !release || !!release.reason || !release.available || !!release.installed || !!release.downloading || !!releaseFailure);
    $('updates-package-check').disabled = !!busy.packages || !!busy.packageInstall || !sourceCount;
    packageRows.forEach(function (r) { disableAction(r.button, !!busy.packages || !!busy.packageInstall || !r.update || !!r.update.reason); });
    $('updates-repair-check').disabled = !!busy.repair || !!busy.repairApply;
    disableAction($('updates-repair-apply'), !!busy.repair || !!busy.repairApply || !preview || !preview.token || !(preview.findings || []).some(f => f.state === 'stale' && !f.error));
  }
  function drawRelease(s) {
    release = s;
    var words;
    if (s.reason) words = s.reason;
    else if (s.error) words = 'Update failed: ' + s.error;
    else if (s.downloading) words = 'Downloading and preparing ' + (s.latest || 'the release') + '. Installed and running versions remain separate below.';
    else if (s.installed) words = (s.latest || 'The release') + ' is installed. Running copies are listed below; nothing has been restarted.';
    else if (s.ready) words = (s.latest || 'The release') + ' is downloaded and checked, but has not been installed.';
    else if (!s.latest) words = 'No published release was found. Installed versions are shown above.';
    else if (s.available) words = s.latest + ' is available. This window reports ' + (s.current || 'an unknown running version') + '.';
    else words = 'No newer release was found. The service reports ' + (s.current || 'an unknown current version') + ' and latest release ' + s.latest + '.';
    status('updates-release-status', releaseFailure || words, !!(releaseFailure || s.error));
    // Notes are untrusted text, and do not need HTML to explain a release.
    if ($('updates-notes').textContent !== (s.notes || 'Release notes are unavailable.')) $('updates-notes').textContent = s.notes || 'Release notes are unavailable.';
    var bar = $('updates-progress');
    if (s.downloading && s.size > 0) {
      bar.max = s.size; bar.value = Math.max(0, Math.min(s.received || 0, s.size));
      $('updates-progress-text').textContent = bar.value + ' of ' + s.size + ' bytes';
    } else if (s.downloading) {
      bar.removeAttribute('value'); $('updates-progress-text').textContent = 'Download size is not known.';
    } else {
      bar.max = 1; bar.value = s.installed ? 1 : 0;
      $('updates-progress-text').textContent = s.installed ? 'Installation finished; running copies are unchanged.' : 'No download in progress.';
    }
    controls();
  }
  function drawInventory(s) {
    if (!s || !Array.isArray(s.installed) || !Array.isArray(s.running)) throw new Error('The program inventory was not a supported status result.');
    inventory = s;
    s.installed.forEach(function (p) {
      var r = programs.find(r => r.file === p.file) || programs.find(r => !r.file);
      if (!r) throw new Error('The release reported more than three components.');
      r.file = p.file;
      var copies = s.running.filter(c => c.program === p.file || fileName(c.path).startsWith(p.file.toLowerCase()));
      r.words.textContent = p.name + ' (' + p.file + ')\nInstalled: ' + (p.version || 'unknown') + (p.error ? '; ' + p.error : '') +
        '\nRunning: ' + (s.runningError ? 'unknown' : copies.length ? copies.map(c => c.version || 'unknown version').join('; ') : 'not reported');
    });
    programs.forEach(function (r, i) {
      if (!r.file || !s.installed.some(p => p.file === r.file)) r.words.textContent = (r.file || 'Program ' + (i + 1)) + '\nNo component was reported.';
    });
    var seen = new Set();
    s.running.forEach(function (p) {
      var key = String(p.pid), r = runningRows.get(key);
      if (!r) { r = row($('updates-running'), key); runningRows.set(key, r); }
      seen.add(key);
      r.words.textContent = 'PID ' + p.pid + (p.self ? ' (this window)' : '') + ': ' + (p.current ? 'installed copy' : 'older copy; restart its owner when ready') +
        '\nVersion: ' + (p.version || 'unknown') + '\nStarted by: ' + (p.startedBy || 'owner not reported') + '\n' + p.path;
    });
    if (!s.runningError) runningRows.forEach(function (r, key) {
      if (!seen.has(key)) r.words.textContent = 'PID ' + key + '\nNo longer reported as running. Its row is retained in place.';
    });
    status('updates-inventory-status', s.installed.some(p => p.error || !p.version) ? 'Inventory read. One or more installed components are missing or unidentifiable.' : 'Installed versions read. Running versions and source identifiers are reported separately.', s.installed.some(p => p.error || !p.version));
    status('updates-running-status', s.runningError ? 'Running state is unknown: ' + s.runningError :
      s.running.length ? s.behind + ' older running copies reported. An external client needs its own restart; this page stops none.' : 'No running copies were reported.', !!s.runningError);
  }
  function fileName(path) { return String(path || '').split(/[\\/]/).pop().toLowerCase(); }
  function refreshInventory() {
    status('updates-inventory-status', 'Reading installed and running versions…');
    return request('inventory', () => available().then(s => s.Status()), drawInventory, function (error) {
      status('updates-inventory-status', 'Could not read versions: ' + message(error) + (inventory ? ' The rows retain the last successful reading.' : ''), true);
      status('updates-running-status', 'Running state is unknown until status can be read.', true);
    });
  }
  function drawRestart(s) {
    if (!s || typeof s.ready !== 'boolean') { status('updates-restart-status', 'Restart status is unavailable.'); return; }
    var counts = (Number.isFinite(s.working) ? s.working : 'unknown') + ' active chats; ' + (Number.isFinite(s.writing) ? s.writing : 'unknown') + ' document writes in progress.';
    status('updates-restart-status', (s.pending ? 'Restart is waiting. ' : s.ready ? 'Restart to Update is available in Notifications. ' : 'No desktop restart is currently offered. ') + counts);
  }
  function refreshRestart() {
    var seq = sequences.restart = (sequences.restart || 0) + 1;
    Promise.resolve().then(() => app().UpdateState()).then(function (s) {
      if (seq === sequences.restart) drawRestart(s);
    }).catch(function (error) { if (seq === sequences.restart) status('updates-restart-status', 'Restart status could not be read: ' + message(error), true); });
  }
  function checkRelease() {
    releaseFailure = ''; status('updates-release-status', 'Checking for a release…');
    return request('release', () => available().then(s => s.Check()), drawRelease, function (error) {
      releaseFailure = 'Could not check for a release: ' + message(error) + (release ? ' The last successful release reading remains visible.' : '');
      status('updates-release-status', releaseFailure, true);
    });
  }
  function installRelease() {
    if (actionDisabled($('updates-install'))) return;
    releaseFailure = ''; status('updates-release-status', 'Requesting installation…');
    return request('install', () => available().then(s => s.Download()), function (s) {
      drawRelease(s); refreshInventory(); follow();
    }, function (error) {
      releaseFailure = 'Installation request failed: ' + message(error);
      status('updates-release-status', releaseFailure, true); refreshInventory();
    });
  }
  function visible() { return !$('settings-updates').hidden && !$('settings').closest('#applet-storage'); }
  function follow() {
    if (timer) return;
    timer = setTimeout(function () {
      timer = null;
      if (!visible()) return;
      if (busy.release || busy.install) { follow(); return; }
      var wasInstalling = release && release.downloading;
      request('release', () => available().then(s => s.State()), function (s) {
        drawRelease(s);
        if (wasInstalling && !s.downloading) refreshInventory();
      }, function (error) { status('updates-release-status', 'Update progress could not be read: ' + message(error), true); }, true).then(follow);
    }, 2000);
  }
  function packageKey(p) { return JSON.stringify([p.target, p.workspace || '', p.name]); }
  function drawPackages(s) {
    packageState = s;
    var seen = new Set();
    (s.installed || []).forEach(function (p) {
      var key = packageKey(p), r = packageRows.get(key);
      if (!r) {
        r = row($('updates-packages'), key, true); packageRows.set(key, r);
        r.button.addEventListener('click', function () { updatePackage(r); });
      }
      seen.add(key); r.update = (s.updates || []).find(u => packageKey(u) === key) || null;
      r.words.textContent = p.name + '\nInstalled: ' + p.version + '; ' + (p.target === 'me' ? 'personal, all workspaces' : 'workspace: ' + p.workspace) +
        '\n' + (r.update ? 'Available: ' + r.update.to + (r.update.reason ? '; blocked: ' + r.update.reason : '') : s.checked ? 'No newer compatible version found.' : 'Not checked yet.');
      r.button.setAttribute('aria-label', 'Update ' + p.name + ' ' + (p.target === 'me' ? 'for you' : 'in ' + p.workspace));
      r.button.title = r.update && r.update.reason || 'Shows the installer preview before applying this package.';
    });
    packageRows.forEach(function (r, key) { if (!seen.has(key)) { r.update = null; r.words.textContent = 'This package is no longer reported as installed. Its row is retained in place.'; } });
    status('updates-package-status', !sourceCount ? 'No package update source is configured.' : s.off ? 'All configured package sources are turned off.' : s.checked ?
      ((s.updates || []).length ? 'Newer package versions found. Choose a package to preview its update.' : 'Package check finished; no newer compatible installed-package versions were found.') :
      (s.installed || []).length ? 'Installed packages read. Check to look for newer versions.' : 'No skill package is installed in this scope.');
    controls();
  }
  function readPackages(check) {
    status('updates-package-status', check ? 'Checking skill packages…' : 'Reading installed packages…');
    return request('packages', async function () {
      var state = await app().State(); sourceCount = state.packageSources || 0;
      if (check && sourceCount) return Object.assign({}, await app().FindPackageUpdates(), { checked: true });
      return { installed: await app().InstalledPackages(), updates: [], checked: false };
    }, drawPackages, function (error) {
      status('updates-package-status', 'Could not ' + (check ? 'check' : 'read') + ' skill packages: ' + message(error) + (packageState ? ' Rows retain the last successful reading.' : ''), true);
    });
  }
  function updatePackage(r) {
    if (actionDisabled(r.button) || !r.update) return;
    var update = r.update;
    status('updates-package-status', 'Previewing ' + update.name + ' before installation…');
    return request('packageInstall', () => window.dinahPackageUpdates.install(update), function (installed) {
      status('updates-package-status', installed ? update.name + ' was installed. Agents remain running; restart them when ready.' : 'Package installation was cancelled or blocked.');
      if (installed) readPackages(true);
    }, function (error) { status('updates-package-status', update.name + ' could not be installed: ' + message(error), true); });
  }
  function drawRepair(result, applied) {
    if (!result || !Array.isArray(result.findings)) throw new Error('The repair service did not return supported findings.');
    if (!applied && result.findings.some(f => f.state === 'stale' && (!result.token || !Array.isArray(result.programs) || !result.programs.length))) {
      throw new Error('The repair preview lacks its token or proposed destination. Nothing can be repaired from this preview.');
    }
    var seen = new Set();
    result.findings.forEach(function (f) {
      var key = JSON.stringify([f.agent, f.file, f.project || '', f.server]), r = repairRows.get(key);
      if (!r) { r = row($('updates-repair'), key); repairRows.set(key, r); }
      seen.add(key);
      var outcome = f.error ? 'Repair failed: ' + f.error : f.changed ? 'Repaired' : f.state === 'stale' ? 'Would repair this owned entry' :
        f.state === 'current' ? 'Current; no change' : f.state === 'absent' ? 'No entry; none will be added' : f.state === 'kept' ? 'Custom entry kept; ownership or arguments differ' : 'Settings could not be read';
      r.words.textContent = f.agent + ': ' + (f.project ? 'project ' + f.project : 'user scope') + '\n' + outcome +
        '\nRuns: ' + (f.runs || 'not reported') + '\nInstalled tools: ' + ((result.programs || []).join('; ') || 'not reported') + (f.backup ? '\nBackup: ' + f.backup : '');
    });
    repairRows.forEach(function (r, key) { if (!seen.has(key)) r.words.textContent = 'This entry was not reported by the latest check. Its row is retained in place.'; });
    $('updates-repair-lines').textContent = (result.lines || []).join('\n') || 'No additional repair details were reported.';
    var failed = result.failed || result.findings.some(f => f.error || !['stale', 'current', 'absent', 'kept'].includes(f.state) || (applied && f.state === 'stale' && !f.changed));
    preview = applied || failed ? null : result;
    status('updates-repair-status', (applied ? failed ? 'Some settings could not be repaired. Read each outcome; no automatic retry occurred.' : 'Repair finished. Existing agent processes are unchanged.' :
      failed ? 'Some settings could not be checked. Repair is unavailable until a successful check.' : result.findings.some(f => f.state === 'stale') ? 'Check finished. Review the proposed destinations before pressing Repair Agent Settings.' : 'Check finished. No owned stale entry needs repair.') +
      (result.limitations && result.limitations.length ? ' ' + result.limitations.join(' ') : ''), failed);
    controls();
  }
  function checkRepair() {
    preview = null; status('updates-repair-status', 'Checking agent settings without changing them…');
    return request('repair', () => available().then(s => s.PreviewAgentSettings()), r => drawRepair(r, false), function (error) {
      status('updates-repair-status', 'Agent settings could not be checked: ' + message(error), true);
    });
  }
  function repair() {
    if (actionDisabled($('updates-repair-apply'))) return;
    var token = preview.token; preview = null;
    status('updates-repair-status', 'Repairing the settings from the preview…');
    return request('repairApply', () => available().then(s => s.RepairAgentSettings(token, [])), r => drawRepair(r, true), function (error) {
      status('updates-repair-status', 'Repair did not confirm completion: ' + message(error) + ' Check again before another repair.', true);
    });
  }
  for (var i = 0; i < 3; i++) {
    var p = row($('updates-programs'), String(i)); p.words.textContent = 'Program ' + (i + 1) + '\nWaiting for the version inventory.'; programs.push(p);
  }
  $('updates-refresh').addEventListener('click', refreshInventory);
  $('updates-check').addEventListener('click', checkRelease);
  $('updates-install').addEventListener('click', installRelease);
  $('updates-package-check').addEventListener('click', () => readPackages(true));
  $('updates-repair-check').addEventListener('click', checkRepair);
  $('updates-repair-apply').addEventListener('click', repair);
  $('updates-notifications').addEventListener('click', function () { window.dinahWorkbench.open('needs-you').catch(window.dinahHost.fail); });
  window.dinahHost.on('update:state', function (s) { sequences.restart = (sequences.restart || 0) + 1; drawRestart(s); });
  document.addEventListener('dinah:workspace-selected', function () { if (visible()) readPackages(false); });
  window.dinahSettingsSections.register('updates', { state: () => '', shown: function () {
    refreshInventory(); refreshRestart(); readPackages(false); checkRelease(); follow();
  } });
  controls();
  // A build contributes this section only when it supplies the service.
  // Discovery failure keeps the section unavailable, including URL restores.
  available().then(function () { $('settings-tab-updates').hidden = false; }).catch(function () {});
})();
