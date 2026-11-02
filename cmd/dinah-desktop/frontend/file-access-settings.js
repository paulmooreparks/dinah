/* Settings › Agent File Access (docs/design/agent-file-access.md). Agents
   read files through Dinah wherever they need to, except in the places the
   policy protects; this page shows those places and the log of reads.

   Go owns the policy and the log (docs/design/provider-file-reads.md). The
   page draws one list of rules in three kinds, each with only the actions Go
   returns for it: a protected rule has none, a default can be turned off or
   on, and a custom rule can be removed. Turning a default off and removing a
   rule loosen protection, so both ask first. Every change carries the
   policy's version, and one that Go refuses as stale reads the policy again.

   The log is newest first, by chat, time range and outcome. Those filters
   are part of the Settings window's address, after "file-access?". A read
   that arrives while the reader is at the top of the log is added there;
   while they are further down, the rows stay where they are and a button
   in the filter row says how many newer reads wait. Once rows are shown,
   each chat's count of refused reads is acknowledged through the newest
   row of it on screen. */
(function () {
  'use strict';
  var SECTION = 'file-access', PAGE = 50;
  var SINCE = { hour: 3600e3, week: 7 * 864e5 };
  var policy = null, rows = [], next = 0, waiting = 0, mounted = false, shownOnce = false, previewTimer = 0, logRequest = 0, policyRequest = 0;
  var filter = { chat: '', since: '', outcome: '' };

  function app() { return window.dinahHelpers.app(); }
  function $(id) { return document.getElementById(id); }
  function message(error) { return window.dinahHelpers.message(error); }
  function available() { var a = app(); return !!(a && a.FileAccessPolicy && a.FileAccessLog); }
  function ask(title, body, yes) { return window.dinahHost.ask(title, body, yes, true); }
  function status(id, text, failed) {
    var el = $(id); if (!el) return;
    el.textContent = text || ''; el.classList.toggle('danger', !!failed);
  }
  function glyph(name) {
    var g = document.createElement('span'); g.className = 'glyph'; g.setAttribute('aria-hidden', 'true');
    g.style.setProperty('--glyph', 'var(--glyph-' + name + ')');
    return g;
  }
  function stale(error) { return /version|changed/i.test(message(error)); }

  /* ---- Address ---- */
  function stateText() {
    var p = new URLSearchParams();
    if (filter.chat) p.set('chat', filter.chat);
    if (SINCE[filter.since] || filter.since === 'today') p.set('since', filter.since);
    if (filter.outcome === 'denied') p.set('outcome', 'denied');
    return p.toString();
  }
  function readState(text) {
    var p = new URLSearchParams(text || '');
    var since = p.get('since') || '';
    filter = { chat: p.get('chat') || '', since: SINCE[since] || since === 'today' ? since : '', outcome: p.get('outcome') === 'denied' ? 'denied' : '' };
  }
  function sinceTime() {
    if (filter.since === 'today') { var d = new Date(); d.setHours(0, 0, 0, 0); return d.toISOString(); }
    return SINCE[filter.since] ? new Date(Date.now() - SINCE[filter.since]).toISOString() : '';
  }

  /* ---- Rules ---- */
  function ruleRow(rule) {
    var row = document.createElement('li'), text = document.createElement('div'), place = document.createElement('span'), reason = document.createElement('p');
    row.className = 'file-access-rule'; row.dataset.rule = rule.id; row.dataset.kind = rule.kind;
    var paths = rule.resolvedPaths || [];
    place.className = 'file-access-place'; place.textContent = paths.length ? paths.join(', ') : rule.pattern;
    reason.className = 'form-help'; reason.textContent = rule.reason || '';
    text.className = 'file-access-text'; text.append(place, reason);
    if (!paths.length) { var absent = document.createElement('p'); absent.className = 'form-help'; absent.textContent = 'Not present on this computer.'; text.append(absent); }
    if (rule.kind === 'default' && rule.enabled === false) { var off = document.createElement('p'); off.className = 'form-help'; off.textContent = 'Off. Agents can read this place.'; text.append(off); }
    var lead = rule.kind === 'protected' ? glyph('shield') : document.createElement('span');
    lead.classList.add('file-access-mark');
    if (rule.kind === 'protected') { var word = document.createElement('span'); word.className = 'visually-hidden'; word.textContent = 'Always protected. '; text.prepend(word); }
    row.append(lead, text);
    var actions = rule.actions || [];
    if (actions.indexOf('enable') >= 0 || actions.indexOf('disable') >= 0) {
      var on = rule.enabled !== false, sw = document.createElement('button');
      sw.type = 'button'; sw.className = 'switch'; sw.setAttribute('role', 'switch'); sw.setAttribute('aria-checked', String(on));
      sw.innerHTML = '<span class="switch-track"><span class="switch-thumb"></span></span>';
      sw.append('Protected');
      sw.setAttribute('aria-label', 'Protect ' + place.textContent);
      sw.addEventListener('click', function () { setDefault(rule, !on, sw); });
      row.append(sw);
    }
    if (actions.indexOf('remove') >= 0) {
      var remove = document.createElement('button');
      remove.type = 'button'; remove.className = 'btn btn-sm'; remove.textContent = 'Remove';
      remove.setAttribute('aria-label', 'Remove ' + place.textContent);
      remove.addEventListener('click', function () { removeRule(rule, remove); });
      row.append(remove);
    }
    if (rule.kind === 'default' && rule.enabled === false) row.dataset.off = '';
    return row;
  }
  /* The list is drawn again in place: each row is replaced by its new one,
     and the control that had focus gets it back in its row's new copy, or
     the path field when its row is gone. */
  function drawRules() {
    var list = $('file-access-rules'), focused = document.activeElement, focusRule = focused && list.contains(focused) && focused.closest('[data-rule]');
    var focusAt = focusRule ? Array.from(focusRule.querySelectorAll('button')).indexOf(focused) : -1, id = focusRule && focusRule.dataset.rule;
    list.replaceChildren.apply(list, (policy && policy.rules || []).map(ruleRow));
    if (id) {
      var again = list.querySelector('[data-rule="' + CSS.escape(id) + '"]');
      var target = again && again.querySelectorAll('button')[focusAt];
      (target || $('file-access-path')).focus({ preventScroll: true });
    }
  }
  function loadPolicy() {
    var mine = ++policyRequest;
    return Promise.resolve(app().FileAccessPolicy()).then(function (p) {
      if (mine !== policyRequest) return;
      policy = p; drawRules(); preview();
    }).catch(function (error) { status('file-access-status', 'The places could not be read: ' + message(error), true); });
  }
  // A change Go refused because another window changed the policy first
  // reads the policy again and says so; any other failure is reported.
  function changed(p, done) { policy = p; drawRules(); status('file-access-status', done); }
  function refused(error) {
    loadPolicy();
    if (stale(error)) { status('file-access-status', 'The places changed in another window. The list shows them as they are now.'); return; }
    status('file-access-status', message(error, true), true);
  }
  function setDefault(rule, enabled, control) {
    var place = (rule.resolvedPaths || [])[0] || rule.pattern;
    var asked = enabled ? Promise.resolve(true) : ask('Stop Protecting This Place?', 'Agents will be able to read ' + place + ' through ' + window.dinahProduct.name + '. ' + (rule.reason || '') + ' The change is recorded in the log, and you can protect it again here.', 'Stop Protecting');
    asked.then(function (yes) {
      if (!yes) return;
      control.disabled = true;
      return Promise.resolve(app().SetFileAccessDefault(rule.id, enabled, policy.version)).then(function (p) {
        changed(p, enabled ? place + ' is protected again.' : place + ' is no longer protected.');
      }, refused).then(function () { control.disabled = false; });
    });
  }
  function removeRule(rule, control) {
    var place = (rule.resolvedPaths || [])[0] || rule.pattern;
    ask('Remove This Place?', 'Agents will be able to read ' + place + ' through ' + window.dinahProduct.name + ' unless another place in the list covers it.', 'Remove').then(function (yes) {
      if (!yes) return;
      control.disabled = true;
      return Promise.resolve(app().RemoveFileAccessEntry(rule.id, policy.version)).then(function (p) {
        changed(p, place + ' was removed.');
      }, function (error) { control.disabled = false; refused(error); });
    });
  }

  /* ---- Adding a place ---- */
  var checked = null;
  function preview() {
    var field = $('file-access-path'), add = $('file-access-add'), out = $('file-access-preview'), path = field.value.trim();
    clearTimeout(previewTimer); checked = null; add.disabled = true;
    if (!path) { out.textContent = ''; return; }
    var asked = path;
    previewTimer = setTimeout(function () {
      Promise.resolve(app().CheckFileAccessPath(asked, window.dinahHost.workspaceId() || '')).then(function (c) {
        if (field.value.trim() !== asked) return;
        if (!c || !c.valid) { out.textContent = (c && c.error) || 'That is not a place on this computer.'; return; }
        var covered = (c.matchedRuleIDs || []).map(function (id) { var r = (policy && policy.rules || []).find(function (x) { return x.id === id; }); return r ? ((r.resolvedPaths || [])[0] || r.pattern) : ''; }).filter(Boolean);
        out.textContent = 'That is ' + c.resolvedPath + '.' + (covered.length ? ' It is already protected as part of ' + covered.join(', ') + '.' : '') + (c.insideWorkspace ? ' It is inside this workspace.' : '');
        checked = c; add.disabled = false;
      }).catch(function (error) { if (field.value.trim() === asked) out.textContent = message(error, true); });
    }, 200);
  }
  function addRule() {
    var field = $('file-access-path'), path = field.value.trim();
    if (!checked || !path) return;
    $('file-access-add').disabled = true;
    Promise.resolve(app().AddFileAccessEntry(path, policy.version)).then(function (p) {
      field.value = ''; $('file-access-preview').textContent = '';
      changed(p, path + ' is protected.');
      field.focus({ preventScroll: true });
    }, function (error) { $('file-access-add').disabled = false; refused(error); });
  }

  /* ---- Reads ---- */
  // Each outcome is one of PUDL's badges, which draws its kind's glyph and
  // colour; the word says the outcome as well.
  var OUTCOMES = {
    allowed: { kind: 'positive', word: function (r) { return 'Read, ' + window.dinahHelpers.fmtSize(r.bytes || 0); } },
    // A refused row may stand for several refusals of the same chat, rule
    // and source; its path and time are the latest, and count says how
    // many (provider-file-reads.md). No count means one.
    denied: { kind: 'danger', word: function (r) { return 'Refused' + (r.count > 1 ? ' ' + r.count + ' times' : '') + (r.reason ? ': ' + r.reason : ''); } },
    error: { kind: 'warn', word: function (r) { return 'Failed' + (r.reason ? ': ' + r.reason : ''); } },
    'policy-change': { kind: '', word: function (r) { return r.reason || 'The places changed'; } }
  };
  function logRow(r) {
    var tr = document.createElement('tr'), when = document.createElement('td'), chat = document.createElement('td'), place = document.createElement('td'), what = document.createElement('td');
    tr.dataset.read = r.id;
    var t = document.createElement('time'); window.dinahRegional.stamp(t, r.at, 'dayTime'); when.append(t);
    chat.textContent = r.outcome === 'policy-change' ? 'You' : (r.chatTitle || r.chat || '');
    place.className = 'file-access-place'; place.textContent = r.path || '';
    if (r.count > 1) place.title = 'The latest of ' + r.count + ' refused reads';
    var o = OUTCOMES[r.outcome] || OUTCOMES.error, badge = document.createElement('span');
    badge.className = 'badge' + (o.kind ? ' ' + o.kind : ''); badge.textContent = o.word(r);
    what.append(badge);
    when.dataset.label = 'When'; chat.dataset.label = 'Chat'; place.dataset.label = 'Place'; what.dataset.label = 'What happened';
    tr.append(when, chat, place, what);
    return tr;
  }
  function matches(r) {
    if (filter.chat && r.chat !== filter.chat) return false;
    if (filter.outcome === 'denied' && r.outcome !== 'denied') return false;
    var since = sinceTime();
    return !since || new Date(r.at) >= new Date(since);
  }
  // The chats the log has shown, for the Chat filter, with the one chosen
  // kept even before a row of it is read.
  var chatTitles = new Map();
  function drawChats() {
    var select = $('file-access-chat');
    rows.forEach(function (r) { if (r.chat && r.outcome !== 'policy-change') chatTitles.set(r.chat, r.chatTitle || r.chat); });
    if (filter.chat && !chatTitles.has(filter.chat)) chatTitles.set(filter.chat, filter.chat);
    var options = [['', 'Every chat']].concat(Array.from(chatTitles.entries()).sort(function (a, b) { return a[1].localeCompare(b[1]); }));
    if (select.options.length === options.length && options.every(function (o, i) { return select.options[i].value === o[0] && select.options[i].text === o[1]; })) { select.value = filter.chat; return; }
    select.replaceChildren.apply(select, options.map(function (o) { var opt = document.createElement('option'); opt.value = o[0]; opt.textContent = o[1]; return opt; }));
    select.value = filter.chat;
  }
  function drawLog() {
    var body = $('file-access-log').tBodies[0];
    body.replaceChildren.apply(body, rows.map(logRow));
    var empty = $('file-access-empty');
    empty.hidden = rows.length > 0;
    empty.textContent = filter.chat || filter.since || filter.outcome ? 'No reads match these choices.' : 'No agent has read anything outside its workspace yet.';
    $('file-access-older').hidden = !next;
    drawWaiting();
    drawChats();
  }
  function drawWaiting() {
    var b = $('file-access-newer');
    b.hidden = !waiting;
    b.textContent = waiting === 1 ? 'Show 1 Newer Read' : 'Show ' + waiting + ' Newer Reads';
  }
  function acknowledge(shown) {
    var a = app(); if (!a.AcknowledgeFileAccessLog) return;
    var through = new Map();
    shown.forEach(function (r) { if (r.outcome === 'denied' && r.chat && !(through.get(r.chat) > r.id)) through.set(r.chat, r.id); });
    through.forEach(function (id, chat) { Promise.resolve(a.AcknowledgeFileAccessLog(chat, id)).catch(function () {}); });
  }
  function loadLog(older) {
    var mine = ++logRequest;
    var request = { chat: filter.chat, since: sinceTime(), outcome: filter.outcome, before: older ? next : 0, limit: PAGE };
    return Promise.resolve(app().FileAccessLog(request)).then(function (page) {
      if (mine !== logRequest) return;
      var got = (page && page.rows) || [];
      rows = older ? rows.concat(got) : got; next = (page && page.nextBefore) || 0;
      if (!older) waiting = 0;
      status('file-access-log-status', '');
      drawLog();
      acknowledge(got);
    }).catch(function (error) { if (mine === logRequest) status('file-access-log-status', 'The log could not be read: ' + message(error), true); });
  }
  // Whether the reader is at the top of the log: its first row is in sight.
  function atTop() {
    var body = $('file-access-log').tBodies[0], first = body.rows[0];
    if (!first) return true;
    var scroller = $('file-access-log').closest('.settings-panel') || document.scrollingElement;
    return first.getBoundingClientRect().top >= scroller.getBoundingClientRect().top - 1;
  }
  /* Go says only that the log has new rows (file-access:read), so the page
     reads the newest page under the same filters and takes the rows newer
     than the newest it shows. One read at a time; an event during it is
     caught by the next. */
  var catching = null, again = false;
  function arrived() {
    if (!mounted || $('settings-file-access').hidden) return;
    if (catching) { again = true; return; }
    var newest = rows.length ? rows[0].id : 0, mine = logRequest;
    catching = Promise.resolve(app().FileAccessLog({ chat: filter.chat, since: sinceTime(), outcome: filter.outcome, before: 0, limit: PAGE })).then(function (page) {
      if (mine !== logRequest) return;
      var fresh = ((page && page.rows) || []).filter(function (r) { return r.id > newest && matches(r); });
      if (!fresh.length) return;
      if (atTop() && !waiting) {
        rows = fresh.concat(rows);
        var body = $('file-access-log').tBodies[0];
        fresh.slice().reverse().forEach(function (r) { body.prepend(logRow(r)); });
        $('file-access-empty').hidden = true;
        drawChats();
        acknowledge(fresh);
      } else { waiting = fresh.length; drawWaiting(); }
    }).catch(function () {}).then(function () {
      catching = null;
      if (again) { again = false; arrived(); }
    });
  }

  /* ---- Mounting ---- */
  function filtersFromControls() {
    filter = { chat: $('file-access-chat').value, since: $('file-access-since').value, outcome: $('file-access-outcome').value };
    window.dinahSettingsSections.changed();
    loadLog();
  }
  function controlsFromFilter() {
    $('file-access-since').value = filter.since; $('file-access-outcome').value = filter.outcome;
    drawChats();
  }
  function mount() {
    if (mounted) return;
    mounted = true;
    $('file-access-path').addEventListener('input', preview);
    $('file-access-path').addEventListener('keydown', function (e) { if (e.key === 'Enter' && !$('file-access-add').disabled) { e.preventDefault(); addRule(); } });
    $('file-access-add').addEventListener('click', addRule);
    ['file-access-chat', 'file-access-since', 'file-access-outcome'].forEach(function (id) { $(id).addEventListener('change', filtersFromControls); });
    $('file-access-older').addEventListener('click', function () { loadLog(true); });
    $('file-access-newer').addEventListener('click', function () {
      loadLog().then(function () { $('file-access-log').scrollIntoView({ block: 'nearest' }); });
    });
    window.dinahHost.on('file-access:read', arrived);
    // The policy event carries the new version.
    window.dinahHost.on('file-access:policy', function (version) { if (policy && version !== policy.version) loadPolicy(); });
  }
  function shown(part) {
    if (!$('settings-file-access')) return;
    readState(part);
    var can = available();
    $('file-access-unavailable').hidden = can;
    $('file-access-path').disabled = !can;
    if (!can) return;
    mount();
    controlsFromFilter();
    if (!shownOnce) { shownOnce = true; loadPolicy(); }
    loadLog();
  }

  window.dinahSettingsSections.register(SECTION, { state: stateText, shown: shown });
})();
