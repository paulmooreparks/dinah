/* Settings presents identity-bound selections. The service owns removal safety. */
(function () {
  'use strict';
  var $ = id => document.getElementById(id);
  var app = () => window.dinahHelpers.app();
  var node = window.dinahHelpers.node;
  var rows = new Map(), items = new Map(), selected = new Set(), serial = 0;
  var reading = null, busy = false, valid = false;
  function bytes(value) { return value.toLocaleString() + ' bytes'; }
  function number(value) { return Number.isSafeInteger(value) && value >= 0; }
  function id(value) { return typeof value === 'string' && /^[a-f0-9]{64}$/i.test(value); }
  function say(text, failed) {
    $('set-aside-status').textContent = text;
    $('set-aside-status').classList.toggle('danger', !!failed);
    $('set-aside-status').setAttribute('role', failed ? 'alert' : 'status');
  }
  function controls() {
    var total = 0;
    selected.forEach(key => { var item = items.get(key); if (item) total += item.sizeBytes; });
    $('set-aside-total').textContent = selected.size + ' selected; total logical size: ' + bytes(total) + '.';
    $('set-aside-remove').setAttribute('aria-disabled', String(busy || !!reading || !valid || !selected.size));
    $('set-aside-refresh').setAttribute('aria-disabled', String(busy || !!reading));
    rows.forEach(function (row, key) {
      var item = items.get(key);
      row.check.disabled = busy || !!reading || !valid || !item || !item.canRemove || selected.size >= 100 && !selected.has(key);
      row.check.checked = selected.has(key);
    });
  }
  function validateList(value) {
    if (!value || !number(value.totalBytes) || !Array.isArray(value.items) || !Array.isArray(value.warnings) || !value.warnings.every(w => typeof w === 'string')) return false;
    var sum = 0;
    return new Set(value.items.map(item => item && item.id)).size === value.items.length && value.items.every(function (item) {
      if (!item || !id(item.id) || !['program', 'name', 'date', 'reasonCode', 'reason'].every(key => typeof item[key] === 'string') || !item.name || !Number.isFinite(Date.parse(item.date)) || !number(item.sizeBytes) || !Array.isArray(item.runningPids) || !item.runningPids.every(pid => Number.isInteger(pid) && pid > 0 && pid <= 4294967295) || typeof item.fallback !== 'boolean' || typeof item.canRemove !== 'boolean' || item.canRemove && (item.fallback || item.runningPids.length || item.reasonCode)) return false;
      sum += item.sizeBytes; return Number.isSafeInteger(sum);
    }) && sum === value.totalBytes;
  }
  function draw(value) {
    items = new Map(value.items.map(item => [item.id, item]));
    selected.forEach(key => { if (!items.has(key) || !items.get(key).canRemove) selected.delete(key); });
    value.items.forEach(function (item) {
      var row = rows.get(item.id);
      if (!row) {
        var el = node('div', 'updates-row'), check = node('input', 'form-check'), words = node('label', 'updates-row-text');
        check.type = 'checkbox'; check.id = 'set-aside-copy-' + (++serial); words.htmlFor = check.id; words.tabIndex = 0;
        el.dataset.copyId = item.id; el.append(check, words); $('set-aside-list').appendChild(el);
        row = { el: el, check: check, words: words }; rows.set(item.id, row);
        check.addEventListener('change', function () {
          if (check.disabled) return;
          if (check.checked && selected.size < 100) selected.add(item.id); else selected.delete(item.id);
          controls();
        });
      }
      row.words.textContent = item.program + ': ' + item.name + '\n' + bytes(item.sizeBytes) + ' · ' + window.dinahRegional.format(new Date(item.date), 'dayTime') +
        '\n' + (item.reason || (item.canRemove ? 'Available to select.' : 'Removal is unavailable.')) +
        (item.runningPids.length ? '\nRunning process IDs: ' + item.runningPids.join(', ') : '');
    });
    rows.forEach(function (row, key) { if (!items.has(key)) row.words.textContent = 'This copy is no longer listed. Its row is kept in place.'; });
    $('set-aside-warnings').textContent = value.warnings.join('\n') || 'No process inspection warnings were reported.';
    say(value.items.length ? value.items.length + ' copies listed; total logical size: ' + bytes(value.totalBytes) + '.' : 'No set-aside programs were found.');
    valid = true;
  }
  function load() {
    if (busy) return Promise.resolve();
    if (reading) return reading;
    valid = false; say('Reading set-aside programs.');
    reading = Promise.resolve().then(() => app().ListSetAsidePrograms()).then(function (value) {
      if (!validateList(value)) throw new Error('The program inventory is invalid.');
      draw(value);
    }).catch(function (error) {
      say('Program copies could not be read. ' + window.dinahHelpers.message(error) + ' The previous rows are retained; removal is disabled.', true);
    }).finally(function () { reading = null; controls(); });
    controls(); return reading;
  }
  function validateRemoval(value, chosen) {
    if (!value || !number(value.removedBytes) || !Array.isArray(value.items) || value.items.length !== chosen.length) return false;
    var byId = new Map(chosen.map(item => [item.id, item])), seen = new Set(), sum = 0;
    return value.items.every(function (result) {
      if (!result || !byId.has(result.id) || seen.has(result.id) || !['removed', 'refused'].includes(result.state) || typeof result.reasonCode !== 'string' || typeof result.reason !== 'string') return false;
      seen.add(result.id); if (result.state === 'removed') sum += byId.get(result.id).sizeBytes;
      return true;
    }) && sum === value.removedBytes;
  }
  function remove() {
    if (busy || reading || !valid || !selected.size) return;
    var chosen = Array.from(selected).map(key => items.get(key)), total = chosen.reduce((sum, item) => sum + item.sizeBytes, 0);
    busy = true; controls();
    return window.dinahHost.ask('Remove Selected Programs?', 'Remove these ' + chosen.length + ' copies with a total logical size of ' + bytes(total) + '?\n' + chosen.map(item => item.name).join('\n') + '\nThe service checks protection again before removing each copy.', 'Remove').then(function (yes) {
      if (!yes) return;
      say('Removing the selected program copies.');
      return Promise.resolve().then(() => app().RemoveSetAsidePrograms({ ids: chosen.map(item => item.id) })).then(function (result) {
        if (!validateRemoval(result, chosen)) throw new Error('The removal result is invalid.');
        var byId = new Map(chosen.map(item => [item.id, item]));
        $('set-aside-results').textContent = 'Reported removed size: ' + bytes(result.removedBytes) + '.\n' + result.items.map(item => byId.get(item.id).name + ': ' + (item.state === 'removed' ? 'Removed.' : 'Kept. ' + (item.reason || item.reasonCode))).join('\n');
      }).catch(function (error) {
        $('set-aside-results').textContent = 'Removal completion is unconfirmed. ' + window.dinahHelpers.message(error) + ' The list will be read again. No removal is retried automatically; select and confirm again if needed.';
      }).finally(function () { selected.clear(); busy = false; return load(); });
    }).catch(function (error) { say(window.dinahHelpers.message(error), true); }).finally(function () { busy = false; controls(); });
  }
  $('set-aside-refresh').addEventListener('click', load);
  $('set-aside-remove').addEventListener('click', remove);
  var supported = !!(app() && app().ListSetAsidePrograms && app().RemoveSetAsidePrograms);
  function place(host) {
    if (!supported || !host) return Promise.resolve();
    if ($('set-aside-card').parentElement !== host) host.appendChild($('set-aside-card'));
    return load();
  }
  function showInUpdates() {
    var host = $('updates-set-aside');
    if (host && supported) host.hidden = false;
    return place(host);
  }
  window.dinahSetAsidePrograms = Object.freeze({ showInUpdates: showInUpdates, refresh: function () { return supported ? load() : Promise.resolve(); } });
  window.dinahSettingsSections.register('set-aside', { state: () => '', shown: function () { return place($('settings-set-aside')); } });
  $('settings-tab-set-aside').hidden = !supported;
  // A restored Updates address may be adopted before this script registers.
  if (!$('settings-updates').hidden) showInUpdates();
  controls();
})();
