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

  /* ---- Network places ----
     A rule on a network folder holds only as far as the folder's identity
     is verified (docs/design/agent-file-access.md). Each network place gets
     one block of fixed height: its path with a badge for its state, and two
     lines saying what that state means for the rule, so a change of state
     changes words, never the size of the row. What is or is not refused
     comes first, so cutting the text short can only cut the reason after
     it; a block cut short opens to its whole text when it is focused or
     tapped, and a screen reader reads it whole. The state comes from the
     engine's structured fields, never from the wording of its reason. */
  function when(at) {
    return at && !isNaN(new Date(at)) ? window.dinahHelpers.fmtDate(at).trim() : '';
  }
  function stalledFor(rule) {
    return ((policy && policy.networkWorkers) || []).filter(function (w) { return w.stalled && (w.ruleIDs || []).indexOf(rule.id) >= 0; });
  }
  // verified; last-known, when a folder not verified now is held to its
  // last verified identity, whether it is unverified or unsupported;
  // unsupported, a lasting limit of the folder's file system; or unverified.
  function networkState(root) {
    // A verdict is shown only with when it was reached. Before the first
    // check the engine reports unverified with a plain reason; a verified
    // status without its time is not taken for one.
    if (root.status === 'verified' && !when(root.lastVerifiedAt)) return 'pending';
    if (root.status === 'verified') return 'verified';
    if (root.usingLastKnownIdentity) return 'last-known';
    return root.status === 'unsupported' ? 'unsupported' : 'unverified';
  }
  function sentence(text) { text = String(text || '').trim(); return text && !/[.!?]$/.test(text) ? text + '.' : text; }
  function networkWords(root, stalled) {
    var last = when(root.lastVerifiedAt), state = networkState(root), words;
    if (state === 'pending') {
      words = 'Whether other paths to this folder are refused is not known until it is checked.';
    } else if (state === 'verified') {
      words = 'Other paths to this folder are refused too. Verified ' + last + '.';
    } else if (state === 'unsupported') {
      words = 'Other paths to this folder, such as a mapped drive letter, are not refused, because its file system cannot identify the folder. This will not change.' + (root.reason ? ' ' + sentence(root.reason) : '');
    } else if (state === 'last-known') {
      words = 'Other paths to this folder are refused while it keeps the identity last verified' + (last ? ' ' + last : '') + '. It could not be verified just now.' + (root.reason ? ' ' + sentence(root.reason) : '');
    } else {
      words = 'Another path to this folder, such as a mapped drive letter, is not refused until the folder is verified, because this rule matches it by name only. It is not verified.' + (root.reason ? ' ' + sentence(root.reason) : '');
    }
    // The Check Stalled badge already says a check is stuck.
    if (stalled) words += ' The check of this folder has not finished since ' + (when(stalled.startedAt) || 'it started') + '.';
    return words;
  }
  var BADGES = { pending: ['', 'Not Checked Yet'], verified: ['positive', 'Verified'], unsupported: ['warn', 'Cannot Be Verified'], 'last-known': ['warn', 'Last Known Identity'], unverified: ['warn', 'Unverified'] };
  /* A block whose words are cut short takes focus, so a keyboard or a tap
     can open it to its whole text (app.css); one that fits does not. */
  function markCutShort(scope) {
    requestAnimationFrame(function () {
      scope.querySelectorAll('.file-access-network-detail').forEach(function (detail) {
        if (detail.scrollHeight > detail.clientHeight + 1) detail.tabIndex = 0; else detail.removeAttribute('tabindex');
      });
    });
  }
  function networkList(rule) {
    var list = document.createElement('ul'), stalled = stalledFor(rule);
    list.className = 'file-access-network';
    list.setAttribute('aria-label', 'Network folders');
    rule.networkRoots.forEach(function (root) {
      var item = document.createElement('li'), head = document.createElement('span'), path = document.createElement('span'), detail = document.createElement('p');
      var hung = stalled.filter(function (w) { return w.root === root.path || (w.roots || []).indexOf(root.path) >= 0; })[0];
      var state = networkState(root);
      item.className = 'file-access-network-root';
      item.dataset.status = state === 'last-known' ? 'unverified' : state;
      if (state === 'last-known') item.dataset.lastKnown = '';
      if (hung) item.dataset.stalled = '';
      head.className = 'file-access-network-head';
      path.className = 'file-access-place';
      // A rule on one network folder names it above already.
      path.textContent = rule.networkRoots.length > 1 ? root.path : 'This network folder';
      var badge = document.createElement('span');
      badge.className = 'badge ' + BADGES[state][0];
      badge.textContent = BADGES[state][1];
      // The path comes first, so a change of the badges after it never moves it.
      path.title = root.path;
      head.append(path, badge);
      if (hung) { var slow = document.createElement('span'); slow.className = 'badge warn'; slow.textContent = 'Check Stalled'; head.append(slow); }
      detail.className = 'form-help file-access-network-detail';
      detail.textContent = networkWords(root, hung);
      item.append(head, detail);
      list.append(item);
    });
    return list;
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
    if ((rule.networkRoots || []).length) text.append(networkList(rule));
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
    markCutShort(list);
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
    /* A search or a listing is one row for the whole operation, with how
       many places it looked at and how many of those were refused or
       failed (provider-file-reads.md: operation, visited, denied, errors). */
    if (r.operation && (r.visited || r.denied || r.errors)) {
      var parts = [];
      if (r.visited) parts.push(r.visited + (r.visited === 1 ? ' place looked at' : ' places looked at'));
      if (r.denied) parts.push(r.denied + ' refused');
      if (r.errors) parts.push(r.errors + ' failed');
      var detail = document.createElement('p'); detail.className = 'form-help file-access-operation';
      detail.textContent = r.operation.charAt(0).toUpperCase() + r.operation.slice(1) + ': ' + parts.join(', ') + '.';
      what.append(detail);
    }
    // A read judged while a network folder of its rule was unverified.
    if ((r.unverifiedRuleIDs || []).length) {
      var risk = document.createElement('p'); risk.className = 'form-help file-access-unverified';
      risk.textContent = 'Judged while a network folder in its rule was not verified, so the rule matched that folder by name only.';
      what.append(risk);
    }
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
    // A network folder's state changed; the event may carry no version, so
    // the policy, with its network states, is read again.
    window.dinahHost.on('file-access:network', function () { if (policy) loadPolicy(); });
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
