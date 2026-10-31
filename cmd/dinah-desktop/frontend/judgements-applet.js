/* Judgements (docs/plans/judgement-agents.md, step 3): the tray widget with
   its glyph and colour for on, off or warning; its popup with the day's
   summary; the Judgements page at its own address, with the graph over time
   and the list of decisions; and the Judgements section of Settings. The
   reads and writes are the engine's page table: JudgementState and
   JudgementSummary (the widget, the popup and the graph), JudgementRecords
   (the list), JudgementKinds, JudgementSettings and JudgementModels (the
   section), SetJudgementSettings and CorrectJudgement (the writes), and the
   numbered judgements:changed event, after which the widget re-reads its
   state rather than keeping a count of its own. Until the first kind of
   call is wired into the program the store is empty, and the page says so. */
(function () {
  'use strict';
  var $ = function (id) { return document.getElementById(id); };
  function app() { return window.dinahHelpers.app(); }
  function host() { return window.dinahHost; }
  function node(tag, className, text) { var el = document.createElement(tag); if (className) el.className = className; if (text !== undefined) el.textContent = text; return el; }
  // The feature exists where the engine built it; before then the widget is not there.
  function available() { return !!(app() && app().JudgementState && app().JudgementSummary && app().JudgementRecords); }

  function ago(iso) {
    var at = Date.parse(iso);
    if (!iso || !isFinite(at)) return 'nothing yet';
    var ms = Date.now() - at;
    if (ms < 45000) return 'just now';
    if (ms < 3600000) return Math.max(1, Math.round(ms / 60000)) + ' minutes ago';
    if (ms < 86400000) return Math.round(ms / 3600000) + ' hours ago';
    return Math.round(ms / 86400000) + ' days ago';
  }
  function whenFull(iso) { if (!iso) return 'never'; var at = Date.parse(iso); return isFinite(at) ? new Date(at).toLocaleString() : 'never'; }
  function costWords(r) { return r.costUnit ? (r.costIn + r.costOut) + ' ' + r.costUnit : 'no cost recorded'; }
  function providerDisplayName(id) { return id ? (window.dinahProviderName ? window.dinahProviderName(id) : id) : ''; }
  function budgetNumbers(b) {
    if (b.unmeasured) return 'the cost of ' + b.unmeasured + (b.unmeasured === 1 ? ' call' : ' calls') + ' this week was not measured, so the budget cannot be held to.';
    return b.spent.toLocaleString() + ' of ' + b.limit.toLocaleString() + ' ' + b.unit + ' spent this week, ' + b.left.toLocaleString() + ' left, ' + Math.floor(b.allowance).toLocaleString() + ' allowed by now (the week began ' + new Date(Date.parse(b.periodStart)).toLocaleDateString(undefined, { weekday: 'long' }) + ' at ' + new Date(Date.parse(b.periodStart)).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' }) + ').';
  }
  function budgetLine(b) {
    return providerDisplayName(b.provider) + ': ' + budgetNumbers(b) + ' of ' + b.limit.toLocaleString() + ' ' + b.unit + ' spent this week, ' + b.left.toLocaleString() + ' left, ' + Math.floor(b.allowance).toLocaleString() + ' allowed by now (the week began ' + new Date(Date.parse(b.periodStart)).toLocaleDateString(undefined, { weekday: 'long' }) + ' at ' + new Date(Date.parse(b.periodStart)).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' }) + ').';
  }
  function poolLine(p) {
    if (!p || !p.max) return '';
    var line = 'Pool: ' + p.size + ' with the judge at once, between ' + p.min + ' and ' + p.max + (p.inFlight ? '; ' + p.inFlight + ' in flight now' : '');
    if (p.changed) line += '; it ' + p.changed.why + ' ' + ago(p.changed.at);
    return line + '.';
  }
  function pretty(text) {
    if (!text) return '';
    try { return JSON.stringify(JSON.parse(text), null, 2); } catch (error) { return text; }
  }
  function dayStart(i) {
    var d = new Date(); d.setHours(0, 0, 0, 0);
    return new Date(d.getTime() - i * 86400000);
  }

  /* ---- The tray widget and its popup ---- */
  var state = null;
  function stateWords(s) {
    if (!s) return 'Judgements: reading';
    if (s.state === 'off') return 'Judgements: off (' + (s.why || 'turned off') + ')';
    if (s.paused) return 'Judgements: paused until ' + whenFull(s.paused.until) + ', ' + providerDisplayName(s.paused.provider) + ': ' + s.paused.reason;
    if (s.state === 'warning') return 'Judgements: warning, ' + (s.why || 'the judge is failing');
    return 'Judgements: on, ' + s.calls + (s.calls === 1 ? ' call' : ' calls') + ' today, ' + s.fallbacks + ' answered by the rule';
  }
  function drawWidget() {
    var item = $('status-judgements');
    if (!available()) { item.hidden = true; return; }
    item.hidden = false;
    var badge = item.querySelector('.badge');
    item.dataset.judgement = state ? state.state : '';
    item.dataset.statusState = state && (state.paused || state.state === 'warning') ? 'waiting' : 'idle';
    item.dataset.statusText = !state ? 'Reading' : state.paused ? 'Paused' : state.state === 'off' ? 'Off' : state.state === 'warning' ? 'Warning' : 'On';
    item.dataset.statusGlyph = state && (state.paused || state.state === 'warning') ? 'warning' : state && state.state === 'on' ? 'tick' : 'ring';
    item.dataset.statusColour = state && (state.paused || state.state === 'warning') ? 'warn' : state && state.state === 'on' ? 'positive' : 'text-muted';
    item.dataset.statusCount = state && state.unseen || 0;
    var glyph = item.querySelector('.glyph');
    glyph.style.setProperty('--glyph', 'var(--glyph-' + item.dataset.statusGlyph + ')');
    glyph.style.color = 'var(--' + item.dataset.statusColour + ')';
    badge.className = 'badge';
    if (!state) { badge.hidden = true; }
    else if (state.unseen > 0) { badge.hidden = false; badge.classList.add('warn'); badge.textContent = state.unseen > 99 ? '99+' : String(state.unseen); }
    else if (state.state === 'off') { badge.hidden = false; badge.textContent = 'off'; }
    else { badge.hidden = true; badge.textContent = ''; }
    var words = stateWords(state);
    if (state && state.unseen > 0) words += ', ' + state.unseen + (state.unseen === 1 ? ' undismissed warning' : ' undismissed warnings');
    item.setAttribute('aria-label', words);
    item.title = words;
    if ($('judgements-status-menu').matches(':popover-open')) syncPopup();
  }
  function readState() {
    if (!available()) { drawWidget(); return Promise.resolve(); }
    return Promise.resolve().then(function () { return app().JudgementState(); }).then(function (s) {
      if (window.dinahStateOrder.accept('judgements:changed', '', s, true)) { state = s; drawWidget(); }
    });
  }
  function acceptState(s) {
    if (window.dinahStateOrder.accept('judgements:changed', '', s, false)) { state = s; drawWidget(); }
  }
  /* A row of the widget's popup: the day's line, or a link that opens the
     page or its Settings section, since no state lives only in this popup. */
  function menuRow(text, meta, run) {
    var row = node('div', 'md-row');
    var link = node('a', 'md-item');
    link.href = '#';
    link.appendChild(node('span', null, text));
    if (meta) link.appendChild(node('span', 'md-meta', meta));
    if (run) link.addEventListener('click', function (e) { e.preventDefault(); $('judgements-status-menu').hidePopover(); run(); });
    row.appendChild(link);
    return row;
  }
  function openPage() { return window.dinahWorkbench.open('judgements').catch(host().fail); }
  function openSection() {
    return window.dinahWorkbench.open('settings').then(function () {
      var tab = document.querySelector('[data-settings-section="judgements"]');
      if (tab) tab.click();
    }).catch(host().fail);
  }
  var popupSummary = '';
  function dismissWarning(id, button) {
    button.disabled = true;
    Promise.resolve().then(function () {
      return id == null ? app().DismissJudgementWarnings() : app().DismissJudgementWarning(id);
    }).then(readState).catch(host().fail).finally(function () {
      button.disabled = button.classList.contains('judgements-dismiss-all') && !(state && state.warnings && state.warnings.length);
    });
  }
  function syncPopup() {
    var list = $('judgements-status-list'), first = list.querySelector('.judgements-popup-line');
    if (!first) return;
    first.textContent = stateWords(state) + '.';
    list.querySelector('.judgements-popup-today').textContent = popupSummary;
    var warnings = state && state.warnings || [], rows = list.querySelector('.judgements-warnings');
    var focused = document.activeElement, hadFocus = rows.contains(focused), oldButtons = Array.from(rows.querySelectorAll('button'));
    var focusIndex = oldButtons.indexOf(focused);
    var kept = new Set();
    warnings.forEach(function (warning, index) {
      var id = String(warning.id), row = Array.from(rows.children).find(function (el) { return el.dataset.warningId === id; });
      if (!row) {
        row = node('li', 'judgements-warning vstack gap-2'); row.dataset.warningId = id;
        var text = node('p', 'form-help');
        var dismiss = node('button', 'btn btn-sm', 'Dismiss'); dismiss.type = 'button';
        dismiss.addEventListener('click', function () { dismissWarning(warning.id, dismiss); });
        row.append(text, dismiss);
      }
      var reason = warning.reason || warning.why;
      row.querySelector('p').textContent = whenFull(warning.at) + ': ' + reason;
      row.querySelector('button').setAttribute('aria-label', 'Dismiss warning: ' + reason);
      kept.add(row);
      if (rows.children[index] !== row) rows.insertBefore(row, rows.children[index] || null);
    });
    Array.from(rows.children).forEach(function (row) { if (!kept.has(row)) row.remove(); });
    rows.hidden = !warnings.length;
    list.querySelector('.judgements-warnings-empty').hidden = !!warnings.length;
    var all = list.querySelector('.judgements-dismiss-all'); all.disabled = !warnings.length;
    if (hadFocus && !focused.isConnected) {
      var buttons = rows.querySelectorAll('button');
      (buttons[Math.min(focusIndex, buttons.length - 1)] || list.querySelector('a')).focus();
    } else if (focused === all && all.disabled) list.querySelector('a').focus();
  }
  function drawPopup() {
    var list = $('judgements-status-list');
    list.replaceChildren();
    if (!available()) return;
    popupSummary = '';
    var first = node('p', 'form-help judgements-popup-line', stateWords(state) + '.');
    var summary = node('div', 'judgements-popup-summary'); summary.appendChild(first);
    list.appendChild(summary);
    var warnings = node('ul', 'judgements-warnings vstack gap-3'); warnings.setAttribute('aria-label', 'Judgement warnings');
    var all = node('button', 'btn btn-sm judgements-dismiss-all', 'Dismiss All'); all.type = 'button';
    all.addEventListener('click', function () { dismissWarning(null, all); });
    summary.appendChild(all);
    var body = node('div', 'judgements-popup-body'); list.appendChild(body);
    body.appendChild(warnings);
    body.appendChild(node('p', 'form-help judgements-warnings-empty', 'No warnings.'));
    body.appendChild(node('p', 'form-help judgements-popup-today'));
    Promise.resolve().then(function () { return app().JudgementSummary('', ''); }).then(function (sum) {
      if (!list.isConnected || !first.isConnected) return;
      var line = sum.calls ? 'Today: ' + sum.calls + (sum.calls === 1 ? ' call' : ' calls') + ', ' + sum.fallbacks + ' by the rule. The latest was ' + ago(sum.latest) + '.'
        : 'Nothing has been judged yet. The records will appear on the Judgements page.';
      popupSummary = line; syncPopup();
    }).catch(function () {});
    if (state && state.calibration && state.calibration.state === 'running') body.appendChild(node('p', 'form-help judgements-popup-calibration', 'Calibration is choosing the judge’s model now.'));
    (state && state.budget || []).forEach(function (b) { body.appendChild(node('p', 'form-help judgements-popup-budget', budgetLine(b))); });
    var pool = state && state.pool && poolLine(state.pool);
    if (pool) body.appendChild(node('p', 'form-help judgements-popup-pool', pool));
    body.appendChild(menuRow('Open Judgements', '', openPage));
    body.appendChild(menuRow('Judgement Settings…', '', openSection));
    syncPopup();
  }

  /* ---- The Judgements page ---- */
  var WARN = { judge: 'positive', rule: 'warn' };
  function answerBadge(r) {
    var badge = node('span', 'badge ' + (WARN[r.used] || ''), r.used === 'judge' ? 'judge' : 'rule: ' + (r.why || ''));
    return badge;
  }
  function initPage(root, opts) {
    var el = node('div', 'judgements vstack gap-4');
    root.appendChild(el);
    var kinds = [];
    var range = 7, filters = { kind: '', used: '', why: '', model: '' }, selectedDay = null;
    var records = [], next = 0, pages = 1, request = 0, chosenId = 0, revealChoice = false, disposed = false;

    var bar = node('div', 'judgements-toolbar hstack gap-3 align-center');
    var daySeg = node('div', 'seg');
    daySeg.setAttribute('role', 'radiogroup');
    daySeg.setAttribute('aria-label', 'Show');
    [['7', '7 Days'], ['30', '30 Days']].forEach(function (x) {
      var b = node('button', null, x[1]);
      b.type = 'button'; b.setAttribute('role', 'radio'); b.dataset.value = x[0];
      daySeg.appendChild(b);
    });
    function pickRange(days) {
      range = days;
      daySeg.querySelectorAll('[role="radio"]').forEach(function (b) {
        var on = b.dataset.value === String(days);
        b.setAttribute('aria-checked', String(on)); b.tabIndex = on ? 0 : -1;
      });
      selectedDay = null;
    }
    daySeg.addEventListener('click', function (e) {
      var b = e.target.closest('[role="radio"]');
      if (b) { pickRange(Number(b.dataset.value)); pages = 1; chosenId = 0; tell(); loadAll(); }
    });
    var kindSel = node('select', 'form-select'); kindSel.setAttribute('aria-label', 'Kind');
    var usedSel = node('select', 'form-select');
    usedSel.setAttribute('aria-label', 'Answered by');
    [['', 'Any Answer'], ['judge', 'By the Judge'], ['rule', 'By the Rule']].forEach(function (x) { usedSel.appendChild(new Option(x[1], x[0])); });
    var reasonSel = node('select', 'form-select');
    reasonSel.setAttribute('aria-label', 'Reason');
    [['', 'Any Reason'], ['no-judge', 'No Judge Chosen'], ['late', 'Late'], ['form', 'The Wrong Form'], ['check', 'Refused by the Check'], ['error', 'The Judge Failed'], ['off', 'Turned Off'], ['budget', 'The Budget Spent']].forEach(function (x) { reasonSel.appendChild(new Option(x[1], x[0])); });
    var modelSel = node('select', 'form-select'); modelSel.setAttribute('aria-label', 'Model');
    [kindSel, usedSel, reasonSel, modelSel].forEach(function (sel) {
      sel.addEventListener('change', function () {
        filters = { kind: kindSel.value, used: usedSel.value, why: reasonSel.value, model: modelSel.value };
        pages = 1; chosenId = 0; tell();
        loadRecords(true);
      });
    });
    var openSettings = node('button', 'btn btn-sm', 'Settings…');
    openSettings.type = 'button';
    openSettings.addEventListener('click', openSection);
    bar.append(daySeg, kindSel, usedSel, reasonSel, modelSel, openSettings);
    var total = node('p', 'form-help judgements-total', '');
    el.appendChild(total);
    var budgetCard = node('div', 'judgements-budget-panel');
    el.appendChild(budgetCard);
    var graphHost = node('div', 'judgements-graph-host');
    el.appendChild(graphHost);
    var breakdown = node('div', 'judgements-breakdown hstack gap-4');
    el.appendChild(breakdown);

    /* The page is its address: the range, the day, the filters and the
       decision open are read back as its state, so a reload or a link keeps
       them, and its record sits beside the list it was chosen from (J3 +
       J6, PUDL's master-detail). The list's own tools live at its head. */
    var layout = node('div', 'md-layout judgements-md');
    layout.dataset.mdPane = 'list';
    var bodyRow = node('div', 'md-body');
    var side = node('div', 'md-sidebar');
    var sideHead = node('div', 'md-sidebar-head');
    sideHead.appendChild(node('h3', 'card-subtitle', 'Decisions'));
    sideHead.appendChild(bar);
    side.appendChild(sideHead);
    var listWrap = node('div', 'judgements-list');
    side.appendChild(listWrap);
    var more = node('button', 'btn btn-sm judgements-more', 'More');
    more.type = 'button';
    more.addEventListener('click', function () { pages++; tell(); loadRecords(false); });
    var moreRow = node('div', 'judgements-more-row');
    moreRow.appendChild(more);
    side.appendChild(moreRow);
    var resize = node('div', 'md-resize');
    resize.tabIndex = 0;
    resize.setAttribute('role', 'separator');
    resize.setAttribute('aria-orientation', 'vertical');
    resize.setAttribute('aria-label', 'Resize the list and the decision');
    bodyRow.append(side, resize);
    var detailCol = node('div', 'md-detail');
    detailCol.tabIndex = 0;
    var back = node('a', 'md-back', 'Back to Decisions');
    back.href = '#';
    back.addEventListener('click', function (e) {
      e.preventDefault(); chosenId = 0; tell(); drawDetail(); markRows();
      side.scrollIntoView({ block: 'start' });
    });
    detailCol.appendChild(back);
    var detail = node('div', 'judgement-detail card vstack gap-3');
    detail.hidden = true;
    detailCol.appendChild(detail);
    bodyRow.appendChild(detailCol);
    layout.appendChild(bodyRow);
    el.appendChild(layout);
    var noChoice = node('p', 'form-help', 'Choose a decision to read its record.');
    detailCol.appendChild(noChoice);

    function stateString() {
      var day = selectedDay ? selectedDay.getFullYear() + '-' + String(selectedDay.getMonth() + 1).padStart(2, '0') + '-' + String(selectedDay.getDate()).padStart(2, '0') : '';
      return JSON.stringify({ r: range, d: day, k: filters.kind, u: filters.used, w: filters.why, m: filters.model, open: chosenId || 0, p: pages });
    }
    /* The page's state, asked as the applet's state changes (PUDL's own
       contract): the same one its reload reads. */
    function tell() { if (opts.changed) opts.changed(stateString()); }
    function readState(s) {
      try {
        var v = s ? JSON.parse(s) : {};
        pickRange(v.r === 30 ? 30 : 7);
        filters = { kind: v.k || '', used: v.u || '', why: v.w || '', model: v.m || '' };
        kindSel.value = filters.kind; usedSel.value = filters.used; reasonSel.value = filters.why;
        if (v.d) { var day = new Date(v.d + 'T00:00:00'); if (!isNaN(day.getTime())) selectedDay = day; }
        chosenId = Number(v.open) || 0;
        revealChoice = !!chosenId;
        pages = Math.max(1, Math.floor(Number(v.p) || 1));
      } catch (err) { /* An address from elsewhere reads as none. */ }
    }
    function days() {
      var out = [];
      for (var i = range - 1; i >= 0; i--) out.push(dayStart(i));
      return out;
    }
    function loadGraph() {
      var starts = days();
      var reads = starts.map(function (d, i) {
        var to = i === starts.length - 1 ? new Date() : starts[i + 1];
        return Promise.resolve().then(function () { return app().JudgementSummary(d.toISOString(), to.toISOString()); });
      });
      return Promise.all(reads).then(function (sums) {
        if (disposed) return;
        graphHost.replaceChildren();
        var maxCalls = sums.reduce(function (m, s) { return Math.max(m, s.calls); }, 0);
        if (!maxCalls) return;
        var graph = node('div', 'judgements-graph');
        graph.setAttribute('role', 'group');
        graph.setAttribute('aria-label', 'Calls per day, with the share answered by the rule');
        sums.forEach(function (s, i) {
          var d = starts[i];
          var b = node('button', 'btn btn-sm jbar');
          b.type = 'button';
          var pressed = selectedDay && d.getTime() === selectedDay.getTime();
          b.setAttribute('aria-pressed', String(!!pressed));
          var judged = s.calls - s.fallbacks;
          var title = d.toLocaleDateString() + ': ' + s.calls + (s.calls === 1 ? ' call' : ' calls') + ', ' + s.fallbacks + ' by the rule';
          b.title = title;
          b.setAttribute('aria-label', title + (pressed ? '. Filtering the list. Press to stop filtering.' : '. Press to filter the list to this day.'));
          var fill = node('span', 'jbar-fill');
          fill.style.height = Math.max(2, Math.round(s.calls / maxCalls * 80)) + 'px';
          if (judged) { var jp = node('span', 'jbar-judge'); jp.style.flexGrow = String(judged); fill.appendChild(jp); }
          if (s.fallbacks) { var fp = node('span', 'jbar-fallback'); fp.style.flexGrow = String(s.fallbacks); fill.appendChild(fp); }
          b.appendChild(fill);
          b.appendChild(node('span', 'jbar-day', d.toLocaleDateString(undefined, { weekday: 'narrow' })));
          b.addEventListener('click', function () {
            selectedDay = pressed ? null : d;
            pages = 1; chosenId = 0;
            tell();
            loadGraph();
            loadRecords(true);
          });
          graph.appendChild(b);
        });
        var legend = node('p', 'form-help judgements-graph-legend', 'Judge: solid fill. Rule: striped fill.');
        graphHost.append(legend, graph);
      });
    }
    function breakdownTable(host_, title, rows, nameOf) {
      var card = node('div', 'judgements-by vstack gap-2');
      card.appendChild(node('h4', 'card-subtitle', title));
      var wrap = node('div', 'data-table-wrap');
      var t = node('table', 'data-table');
      t.innerHTML = '<thead><tr><th>' + title.slice(3) + '</th><th>Calls</th><th>By the Rule</th><th>Disagreed</th><th>Corrected</th><th>Average</th><th>Longest</th><th>Cost</th></tr></thead>';
      var body = node('tbody');
      if (!rows.length) {
        var empty = node('tr', 'data-table-empty');
        empty.innerHTML = '<td colspan="8">Nothing to break down.</td>';
        body.appendChild(empty);
      }
      rows.forEach(function (g) {
        var tr = node('tr');
        [nameOf(g), String(g.calls), String(g.fallbacks), String(g.disagreed), String(g.corrected), g.avgMs + ' ms', g.maxMs + ' ms', g.costUnit ? (g.costIn + g.costOut) + ' ' + g.costUnit : 'Not reported'].forEach(function (text) {
          tr.appendChild(node('td', null, text));
        });
        body.appendChild(tr);
      });
      t.appendChild(body);
      wrap.appendChild(t);
      card.appendChild(wrap);
      host_.appendChild(card);
    }
    function loadSummary() {
      var from = days()[0].toISOString(), to = new Date().toISOString();
      return Promise.resolve().then(function () { return app().JudgementSummary(from, to); }).then(function (s) {
        if (disposed) return;
        total.textContent = s.calls ? s.calls + (s.calls === 1 ? ' call' : ' calls') + ' in the last ' + range + ' days: ' + (s.calls - s.fallbacks) + ' answered by the judge, ' + s.fallbacks + ' by the rule.'
          : 'Nothing has been judged yet. When a judgement agent answers a call in place of a rule, the call is recorded here.';
        breakdown.replaceChildren();
        breakdownTable(breakdown, 'By Kind', s.groups, function (g) { return g.kind; });
        var byModel = {};
        s.groups.forEach(function (g) {
          var key = g.model || '(no judge)';
          var m = byModel[key] || (byModel[key] = { model: key, calls: 0, fallbacks: 0, disagreed: 0, corrected: 0, avgMs: 0, maxMs: 0, costIn: 0, costOut: 0, costUnit: '' });
          m.avgMs = Math.round((m.avgMs * m.calls + g.avgMs * g.calls) / (m.calls + g.calls));
          m.calls += g.calls; m.fallbacks += g.fallbacks; m.disagreed += g.disagreed; m.corrected += g.corrected;
          m.maxMs = Math.max(m.maxMs, g.maxMs); m.costIn += g.costIn; m.costOut += g.costOut; m.costUnit = m.costUnit || g.costUnit;
        });
        breakdownTable(breakdown, 'By Model', Object.keys(byModel).map(function (k) { return byModel[k]; }), function (g) { return g.model; });
        // The model filter's choices are the models the record has seen.
        var seen = {}, keep = filters.model;
        s.groups.forEach(function (g) { if (g.model) seen[g.model] = true; });
        modelSel.replaceChildren(new Option('Any Model', ''));
        Object.keys(seen).sort().forEach(function (m) { modelSel.appendChild(new Option(m, m)); });
        if (keep && !seen[keep]) modelSel.appendChild(new Option(keep, keep));
        modelSel.value = keep;
      });
    }
    function rowOf(r) {
      var row = node('div', 'md-row');
      row.dataset.judgement = r.id;
      var link = node('a', 'md-item');
      var url = new URL(location.href);
      var view = JSON.parse(stateString()); view.open = r.id;
      url.searchParams.set('view.judgements', JSON.stringify(view));
      link.href = url.href;
      link.appendChild(node('span', null, r.kind));
      link.appendChild(node('span', 'md-meta', whenFull(r.time)));
      link.appendChild(node('span', 'md-meta', (r.model || 'No judge') + ' · ' + r.elapsedMs + ' ms · ' + costWords(r)));
      link.appendChild(answerBadge(r));
      if (r.correctedAt) link.appendChild(node('span', 'badge positive', 'Corrected'));
      link.addEventListener('click', function (e) {
        if (e.ctrlKey || e.metaKey || e.shiftKey || e.altKey || e.button) return;
        e.preventDefault(); chosenId = r.id; tell(); drawDetail(); markRows();
        detailCol.scrollIntoView({ block: 'start' });
      });
      row.appendChild(link);
      return row;
    }
    function markRows() {
      listWrap.querySelectorAll('[data-judgement]').forEach(function (row) {
        var link = row.querySelector('a');
        if (Number(row.dataset.judgement) === chosenId) link.setAttribute('aria-current', 'true');
        else link.removeAttribute('aria-current');
      });
      layout.dataset.mdPane = chosenId ? 'detail' : 'list';
      noChoice.hidden = !!chosenId;
    }
    function drawRows() {
      listWrap.replaceChildren();
      if (!records.length) listWrap.appendChild(node('p', 'form-help judgements-empty', 'No decisions to show.'));
      records.forEach(function (r) { listWrap.appendChild(rowOf(r)); });
      more.hidden = !next;
      markRows();
      drawDetail();
      if (revealChoice && chosenId) { detailCol.scrollIntoView({ block: 'start' }); revealChoice = false; }
    }
    function loadRecords(reset) {
      var epoch = ++request;
      var filter = { limit: 50 };
      if (filters.kind) filter.kind = filters.kind;
      if (filters.used) filter.used = filters.used;
      if (filters.why) filter.why = filters.why;
      if (filters.model) filter.model = filters.model;
      if (selectedDay) {
        filter.from = selectedDay.toISOString();
        var end = new Date(selectedDay); end.setDate(end.getDate() + 1);
        filter.to = end.toISOString();
      }
      var kept = reset ? [] : records.slice();
      var cursor = reset ? 0 : next;
      var remaining = reset ? pages : 1;
      more.disabled = true;
      function read() {
        var query = Object.assign({}, filter);
        if (cursor) query.before = cursor;
        return Promise.resolve().then(function () { return app().JudgementRecords(query); }).then(function (page) {
          if (disposed || epoch !== request) return;
          kept = kept.concat(page.records || []);
          cursor = page.next || 0;
          if (--remaining > 0 && cursor) return read();
          records = kept; next = cursor;
          drawRows();
        });
      }
      return read().catch(function (error) {
        if (!disposed && epoch === request) total.textContent = 'The decisions could not be read: ' + error;
      }).finally(function () { if (epoch === request) more.disabled = false; });
    }
    function field(label, text) {
      var g = node('div', 'judgement-field vstack gap-1');
      g.appendChild(node('span', 'form-label', label));
      var v = node('pre', 'judgement-value');
      v.textContent = text;
      g.appendChild(v);
      return g;
    }
    function drawDetail() {
      var r = records.filter(function (x) { return x.id === chosenId; })[0];
      detail.replaceChildren();
      if (!r) { detail.hidden = true; return; }
      detail.hidden = false;
      var kind = kinds.filter(function (k) { return k.name === r.kind; })[0] || {};
      var head = node('div', 'hstack gap-3 align-center');
      head.appendChild(node('h4', 'card-subtitle judgement-detail-title', r.kind));
      head.appendChild(answerBadge(r));
      if (r.correctedAt) head.appendChild(node('span', 'badge accent', 'corrected'));
      detail.appendChild(head);
      detail.appendChild(node('p', 'form-help', whenFull(r.time) + ' · version ' + r.version +
        (r.used === 'judge' ? ' · ' + r.provider + ' ' + r.model + ' (temperature ' + r.temperature + ', seed ' + r.seed + ')' : '') +
        ' · ' + r.elapsedMs + ' ms of the ' + r.deadlineMs + ' ms allowed' +
        (r.used === 'rule' && r.why ? ' · the rule answered because of: ' + r.why : '')));
      detail.appendChild(field('What the Judge Saw', pretty(r.input)));
      detail.appendChild(field('What the Judge Answered', pretty(r.answer) || '(nothing)'));
      detail.appendChild(field('What the Rule Would Have Answered', pretty(r.ruleAnswer)));
      detail.appendChild(field('What Dinah Used', pretty(r.out)));
      if (r.effectKind) {
        var wentTo = node('div', 'judgement-field vstack gap-1');
        wentTo.appendChild(node('span', 'form-label', 'Where It Went'));
        if (r.effectKind === 'chat' && window.dinahOpenChat) {
          var link = node('a', null, 'the chat “' + r.effectId + '”');
          link.href = '#';
          link.addEventListener('click', function (e) {
            e.preventDefault();
            Promise.resolve(window.dinahOpenChat(r.effectId)).catch(host().fail);
          });
          wentTo.appendChild(link);
        } else {
          wentTo.appendChild(node('span', null, r.effectKind + ' ' + r.effectId));
        }
        detail.appendChild(wentTo);
      }
      if (r.correctedAt) {
        detail.appendChild(field('Your Correction', r.correction + ', ' + whenFull(r.correctedAt)));
      } else if (kind.correctable) {
        var box = node('div', 'judgement-correct hstack gap-3 align-center');
        var button = node('button', 'btn btn-sm', 'Correct…');
        button.type = 'button';
        var input = node('input', 'form-input');
        input.type = 'text'; input.hidden = true;
        input.setAttribute('aria-label', 'Your correction');
        var save = node('button', 'btn btn-sm btn-primary', 'Save');
        save.type = 'button'; save.hidden = true;
        var status = node('span', 'form-help', '');
        button.addEventListener('click', function () { input.hidden = false; save.hidden = false; button.hidden = true; input.focus(); });
        save.addEventListener('click', function () {
          var value = input.value.trim();
          if (!value) { status.textContent = 'Say what it should have been.'; return; }
          save.disabled = true;
          Promise.resolve(app().CorrectJudgement(r.id, value)).then(function (fresh) {
            Object.assign(r, fresh);
            drawRows();
            drawDetail();
            host().toast('Your correction is kept with the call.', 'positive');
          }, function (error) {
            status.textContent = 'The correction could not be saved: ' + error;
          }).finally(function () { save.disabled = false; });
        });
        input.addEventListener('keydown', function (e) { if (e.key === 'Enter') { e.preventDefault(); save.click(); } });
        box.append(button, input, save, status);
        detail.appendChild(box);
      }
    }
    /* The week's budget per provider with a limit, and the pool as it stands,
       read from the same numbered state the widget shows. */
    function loadBudgetPanel() {
      return Promise.resolve().then(function () { return app().JudgementState(); }).then(function (s) {
        if (disposed) return;
        budgetCard.replaceChildren();
        (s.budget || []).forEach(function (b) {
          var row = node('div', 'judgement-budget-row vstack gap-1');
          var words = node('div', 'hstack gap-3 align-center');
          words.appendChild(node('span', 'judgement-budget-name', providerDisplayName(b.provider)));
          // Spent, the limit and what's left, in the panel's own short words.
        words.appendChild(node('span', 'form-help', b.unmeasured ? 'the cost of ' + b.unmeasured + (b.unmeasured === 1 ? ' call' : ' calls') + ' this week was not measured, so the budget cannot be held to.' : b.spent.toLocaleString() + ' of ' + b.limit.toLocaleString() + ' ' + b.unit + ' spent, ' + b.left.toLocaleString() + ' left'));
          row.appendChild(words);
          var track = node('span', 'judgement-budget-track');
          var fillP = Math.min(100, b.limit ? b.spent / b.limit * 100 : 0);
          var fill = node('span', 'judgement-budget-fill'); fill.style.width = fillP + '%';
          var allowanceP = Math.min(100, b.limit ? b.allowance / b.limit * 100 : 0);
          var mark = node('span', 'judgement-budget-allowance'); mark.style.left = allowanceP + '%';
          mark.title = Math.floor(b.allowance).toLocaleString() + ' ' + b.unit + ' allowed by now';
          track.append(fill, mark);
          var label = b.unit + ' spent of the budget, with a mark at what is allowed by now';
          track.setAttribute('role', 'img'); track.setAttribute('aria-label', label);
          row.appendChild(track);
          budgetCard.appendChild(row);
        });
        var pool = poolLine(s.pool);
        if (pool) budgetCard.appendChild(node('p', 'form-help judgement-pool-line', pool));
      });
    }
    function loadAll() {
      return Promise.all([loadGraph(), loadSummary(), loadRecords(true), loadBudgetPanel()]);
    }
    pickRange(7);
    if (opts.state) readState(opts.state);
    if (available()) {
      Promise.resolve().then(function () { return app().JudgementKinds(); }).then(function (ks) {
        if (disposed) return;
        kinds = ks || [];
        kindSel.replaceChildren(new Option('Any Kind', ''));
        kinds.forEach(function (k) { kindSel.appendChild(new Option(k.name, k.name)); });
        kindSel.value = filters.kind;
      }).catch(function () {});
      loadAll().catch(function () {});
    } else {
      total.textContent = 'Judgement agents are not available in this build.';
      more.hidden = true;
    }
    return { destroy: function () { disposed = true; }, refresh: function () { if (available()) loadAll().catch(function () {}); },
      state: function () { return stateString(); },
      setState: function (s) {
        readState(s);
        [modelSel].forEach(function (sel) { sel.value = filters.model; });
        if (available()) loadAll().catch(function () {});
      } };
  }
  var pageApi = null;
  if (window.dinahWorkbench) window.dinahWorkbench.register('judgements', { applet: { id: 'dinah.judgements', title: 'Judgements', glyph: 'tick', workspace: false },
    singleton: true, title: 'Judgements', glyph: 'tick',
    init: function (root, opts) {
      var page = initPage(root, opts); pageApi = page;
      return Object.assign({}, page, { destroy: function () { page.destroy(); if (pageApi === page) pageApi = null; },
        commands: function () { return [{ label: 'Judgement Settings…', run: openSection }]; } });
    }
  });

  /* ---- Settings › Judgements ---- */
  var settings = null, settingsBusy = false, judgeProviders = null;
  function loadJudgementProviders() {
    if (!app().JudgementProviders) return Promise.resolve();
    return Promise.resolve().then(function () { return app().JudgementProviders(); }).then(function (list) {
      judgeProviders = list || [];
      drawSettings();
    }).catch(function () {});
  }
  /* Choosing a model whose calls spend the person's plan is confirmed first,
     with cost in plain numbers from the engine and the budget as it stands
     (Paul, 30 October 2026; Dinah UI's wording). Cancel restores the pick,
     and a local model asks nothing. */
  function confirmJudgeModel(rec) {
    return new Promise(function (resolve) {
      var dlg = document.createElement('dialog');
      dlg.className = 'dialog judgement-model-confirm';
      var form = document.createElement('form'); form.method = 'dialog';
      var title = document.createElement('h2'); title.className = 'dialog-title';
      title.textContent = 'Use ' + rec.name + ' for Judgements?';
      var body = document.createElement('div'); body.className = 'dialog-body vstack gap-3';
      var cost = document.createElement('p'); cost.className = 'form-help';
      cost.textContent = rec.calls
        ? 'A judgement on ' + rec.name + ' has cost about ' + rec.typicalCost.toLocaleString() + ' ' + rec.unit + ' a call (the median of ' + rec.calls + (rec.calls === 1 ? ' call' : ' calls') + ').'
        : 'The cost of a call is not known yet. Each call starts a session on your plan.';
      body.appendChild(cost);
      var row = (settings && settings.budgets || []).filter(function (b) { return b.provider === rec.id && b.limit !== null; })[0];
      var budgetLineEl = document.createElement('p'); budgetLineEl.className = 'form-help';
      budgetLineEl.textContent = row ? 'A budget of ' + row.limit.toLocaleString() + ' ' + row.unit + ' a week is set below.' : 'No budget is set.';
      body.appendChild(budgetLineEl);
      var actions = document.createElement('div'); actions.className = 'dialog-actions';
      var done = false;
      function finish(yes) { if (done) return; done = true; if (dlg.open) dlg.close(); dlg.remove(); resolve(yes); }
      var cancel = document.createElement('button'); cancel.type = 'button'; cancel.className = 'btn'; cancel.textContent = 'Cancel';
      cancel.addEventListener('click', function () { finish(false); });
      dlg.addEventListener('close', function () { finish(false); });
      var use = document.createElement('button'); use.type = 'submit'; use.className = 'btn btn-primary'; use.textContent = 'Use ' + rec.name;
      use.addEventListener('click', function (e) { e.preventDefault(); finish(true); });
      actions.append(cancel, use);
      form.append(title, body, actions);
      dlg.appendChild(form);
      document.body.appendChild(dlg);
      dlg.showModal();
    });
  }
  /* The engine's envelope: { settings, revision, conflict }. The page keeps
     the revision beside the fields it saves with. */
  function unpackSettings(r) {
    if (!r || !r.settings) return r;
    return Object.assign({}, r.settings, { revision: r.revision, conflict: !!r.conflict });
  }
  function drawSettings() {
    var card = $('judgements-card');
    if (!card) return;
    if (!available()) {
      $('judgements-status').textContent = 'Judgement agents are not available in this build.';
      return;
    }
    if (!settings) return;
    var toggle = $('judgements-enabled');
    toggle.setAttribute('aria-checked', String(!!settings.enabled));
    $('judgements-enabled-help').textContent = settings.enabled ? 'Calls go to the judge.' : 'Every call uses its rule.';
    var model = $('judgement-model');
    Promise.resolve().then(function () { return app().JudgementModels(); }).then(function (models) {
      if (!settings) return;
      var keep = settings.model;
      model.replaceChildren(new Option('No model yet; every call uses its rule', ''));
      (models || []).forEach(function (m) {
        // A provider's name comes from its record, never from a word here.
        var pname = m.provider ? (window.dinahProviderName ? window.dinahProviderName(m.provider) : m.provider) : '';
        model.appendChild(new Option(m.name + (pname && pname !== m.name ? ' (' + pname + ')' : ''), m.name));
      });
      model.value = keep || '';
    }).catch(function () {});
    var providerSel = $('judgement-provider'), providerNote = $('judgement-provider-note');
    if (providerSel) {
      var chosen = (judgeProviders || []).filter(function (p) { return p.id === settings.provider; })[0];
      providerSel.replaceChildren();
      (judgeProviders || []).filter(function (p) { return p.available || p.id === settings.provider; }).forEach(function (p) {
        providerSel.appendChild(new Option(p.name, p.id, false, p.id === settings.provider));
      });
      providerSel.value = settings.provider || '';
      var bits = [];
      if (chosen && chosen.note) bits.push(chosen.note);
      (judgeProviders || []).filter(function (p) { return !p.available; }).forEach(function (p) { bits.push(p.name + ' cannot judge now: ' + p.why); });
      providerNote.textContent = bits.join(' ');
    }
    if (document.activeElement !== $('judgement-retention')) $('judgement-retention').value = String(settings.retentionDays || 90);
    if (document.activeElement !== $('judgement-warn-streak')) $('judgement-warn-streak').value = String(settings.warnStreak || 3);
    Promise.resolve().then(function () { return app().JudgementKinds(); }).then(function (ks) {
      if (!settings) return;
      var list = $('judgement-kinds');
      list.replaceChildren();
      (ks || []).forEach(function (k) {
        var li = node('li', 'judgement-kind vstack gap-2');
        li.dataset.kind = k.name;
        var line = node('div', 'hstack gap-3 align-center');
        var sw = node('button', 'switch');
        sw.type = 'button'; sw.setAttribute('role', 'switch');
        var on = (settings.kindsOff || []).indexOf(k.name) < 0;
        sw.setAttribute('aria-checked', String(on));
        sw.innerHTML = '<span class="switch-track"><span class="switch-thumb"></span></span>';
        sw.appendChild(node('span', null, 'Judge ' + k.name));
        sw.addEventListener('click', function () {
          var off = new Set(settings.kindsOff || []);
          if (off.has(k.name)) off.delete(k.name); else off.add(k.name);
          saveSettings({ kindsOff: Array.from(off) }, 'judgements-status', (off.has(k.name) ? k.name + ' now uses its rule.' : k.name + ' is judged again.'));
        });
        line.appendChild(sw);
        line.appendChild(node('span', 'form-help', 'allowed ' + (k.deadlineMs / 1000) + ' s' + (k.inputCap ? '; the call shows it at most ' + k.inputCap.toLocaleString() + ' bytes' : '')));
        li.appendChild(line);
        li.appendChild(node('p', 'form-help', k.describe));
        var ul = node('ul', 'form-help');
        (k.input || []).forEach(function (f) { ul.appendChild(node('li', null, f.doc)); });
        var detail = disclosure('What the judge sees', ul, 'judgement-may-see', 'judgement-input-' + encodeURIComponent(k.name));
        li.appendChild(detail);
        list.appendChild(li);
      });
      if (!(ks || []).length) list.appendChild(node('li', 'form-help', 'No kind of judgement call is declared yet.'));
    }).catch(function () {});
    drawBudgets(); fillPeriod(); fillPool(); refreshCalibrationLine();
  }
  /* The budget's rows come from the settings: one for each provider that can
     judge, its unit from the provider's row. Empty is no budget. */
  function drawBudgets() {
    var hostEl = $('judgement-budgets');
    if (!hostEl || !settings) return;
    hostEl.replaceChildren();
    (settings.budgets || []).forEach(function (b) {
      var row = node('div', 'hstack gap-3 align-center judgement-budget-edit');
      row.dataset.provider = b.provider;
      row.appendChild(node('span', 'judgement-budget-name', providerDisplayName(b.provider) + ' (' + b.unit + ')'));
      var input = node('input', 'form-input');
      input.type = 'number'; input.min = '0'; input.step = '1'; input.inputMode = 'numeric'; input.placeholder = 'No Budget';
      input.setAttribute('aria-label', providerDisplayName(b.provider) + ' budget for the week');
      if (b.limit !== null && b.limit !== undefined && document.activeElement !== input) input.value = String(b.limit);
      row.appendChild(input);
      hostEl.appendChild(row);
      var rec = (judgeProviders || []).filter(function (x) { return x.id === b.provider && x.note; })[0];
      if (rec) hostEl.appendChild(node('p', 'form-help judgement-budget-note', rec.note));
    });
    if (!(settings.budgets || []).length) hostEl.appendChild(node('p', 'form-help', 'No provider can judge yet, so there is nothing to budget.'));
  }
  function fillPeriod() {
    var day = $('judgement-period-weekday'), hour = $('judgement-period-hour');
    if (!day || !hour || !settings) return;
    if (!day.options.length) {
      var sunday = new Date(2026, 10, 8); // a Sunday
      for (var d = 0; d < 7; d++) day.appendChild(new Option(new Date(sunday.getTime() + d * 86400000).toLocaleDateString(undefined, { weekday: 'long' }), String(d)));
      for (var h = 0; h < 24; h++) hour.appendChild(new Option(new Date(2026, 0, 4, h).toLocaleTimeString(undefined, { hour: 'numeric' }), String(h)));
    }
    var p = settings.period || { weekday: 1, hour: 0 };
    day.value = String(p.weekday); hour.value = String(p.hour);
  }
  function fillPool() {
    var min = $('judgement-pool-min'), max = $('judgement-pool-max');
    if (!min || !max || !settings) return;
    var pool = settings.pool || { min: 1, max: 4 };
    if (document.activeElement !== min) min.value = String(pool.min);
    if (document.activeElement !== max) max.value = String(pool.max);
  }
  /* Calibration: who chose the model, where a run stands, the last report,
     and Use the Chosen Model where the run's choice is not the person's. */
  function drawCalibration(cal, report) {
    var line = $('judgement-calibration-line');
    if (!line || !settings) return;
    cal = cal || { state: 'idle', chosen: '' };
    var current = settings.model ? 'The judge is ' + settings.model + ' (' + (settings.modelSource === 'calibration' ? 'calibration’s choice' : settings.modelSource === 'person' ? 'your choice' : 'chosen') + ').'
      : 'No model is chosen; every call uses its rule.';
    var words;
    if (cal.state === 'running') words = 'Calibration is running now, trying candidate models in the background. ' + current;
    else if (cal.state === 'chosen' && cal.chosen && cal.chosen !== settings.model) words = current + ' Calibration found ' + cal.chosen + ' passing and faster, ' + ago(cal.at) + '; it is offered beside this.';
    else if (cal.state === 'chosen' && settings.modelSource === 'calibration') words = current + ' Calibration chose it ' + ago(cal.at) + '.';
    else if (cal.state === 'none') words = 'The last run found no model that answered every case correctly, ' + ago(cal.at) + '. ' + current;
    else if (cal.state === 'stopped') words = 'The last run was stopped, ' + ago(cal.at) + '. ' + current;
    else words = current + ' Calibration has not run. With no model chosen it runs by itself against the resident models; it never replaces a choice of yours.';
    line.textContent = words;
    var run = $('judgements-run-calibration');
    if (run) run.disabled = !settings.enabled || cal.state === 'running';
    var use = $('judgements-use-chosen');
    if (use) {
      var chosen = cal.state === 'chosen' && cal.chosen;
      use.hidden = !chosen || settings.model === chosen;
      if (chosen) { use.dataset.model = chosen; use.textContent = 'Use ' + chosen; }
    }
    var details = $('judgement-calibration-details'), box = $('judgement-calibration-report');
    if (!details || !box) return;
    if (!report || !report.models || !report.models.length) { details.hidden = true; return; }
    details.hidden = false;
    box.replaceChildren();
    if (report.note) box.appendChild(node('p', 'form-help', report.note));
    var table = node('table', 'data-table');
    table.innerHTML = '<thead><tr><th>Model</th><th></th><th>Answered</th><th>Median</th><th>Note</th></tr></thead>';
    var body = node('tbody');
    report.models.forEach(function (m) {
      var tr = node('tr');
      var tags = [m.candidate ? '' : 'not a candidate', m.loaded ? 'loaded for this run' : ''].filter(function (x) { return x; }).join(', ');
      [m.name, tags, m.passed + ' of ' + m.total, m.total ? m.medianMs + ' ms' : '', m.why || ''].forEach(function (x) { tr.appendChild(node('td', null, x)); });
      body.appendChild(tr);
    });
    table.appendChild(body);
    box.appendChild(table);
    if (report.cases && report.cases.length) {
      var ul = node('ul', 'form-help');
      report.cases.forEach(function (c) { ul.appendChild(node('li', null, (c.name || c.kind) + ': put in “' + c.input + '”. The answer must offer ' + (c.accepts || []).join(' or ') + ' and pass its kind’s check.')); });
      box.appendChild(disclosure('What the models were tested on (' + report.cases.length + ' calls)', ul, 'judgement-calibration-cases', 'judgement-calibration-cases-body'));
    }
  }
  function refreshCalibrationLine() {
    if (!available() || !settings) return;
    Promise.resolve().then(function () { return app().JudgementState(); }).then(function (s) {
      return Promise.resolve().then(function () { return app().JudgementCalibration(); }).then(function (report) {
        drawCalibration(s && s.calibration, report);
      });
    }).catch(function () {});
  }
  function saveSettings(patch, statusId, said) {
    if (settingsBusy || !settings) return;
    settingsBusy = true;
    var status = $(statusId);
    var next = Object.assign({}, settings, patch);
    /* SetJudgementSettings takes the values and the revision they were read
       at; a stale one writes nothing and answers with the current settings
       and conflict true (the other writer is calibration, in step 5). */
    var revision = settings.revision;
    delete next.revision;
    delete next.conflict;
    Promise.resolve().then(function () { return app().SetJudgementSettings(next, revision); }).then(function (r) {
      settings = unpackSettings(r) || next;
      if (status) {
        status.classList.remove('notice', 'danger');
        status.textContent = settings.conflict ? 'The settings changed since you read them; the newer ones are shown.' : said || 'Saved.';
      }
      drawSettings();
    }, function (error) {
      if (status) status.textContent = 'The settings could not be saved: ' + error;
      /* The other writer's version stands, so read it back rather than keep
         the page's copy. */
      Promise.resolve().then(function () { return app().JudgementSettings(); }).then(function (r) { settings = unpackSettings(r); drawSettings(); }).catch(function () {});
    }).finally(function () { settingsBusy = false; });
  }
  function disclosure(label, body, className, bodyId) {
    var wrap = node('div', className);
    var button = node('button', 'btn btn-sm disclosure', label);
    button.type = 'button'; button.setAttribute('aria-expanded', 'false');
    body.id = bodyId;
    button.setAttribute('aria-controls', bodyId);
    body.hidden = true;
    button.addEventListener('click', function () {
      body.hidden = !body.hidden;
      button.setAttribute('aria-expanded', String(!body.hidden));
    });
    wrap.append(button, body);
    return wrap;
  }
  function initSection() {
    if (!available()) { drawSettings(); return; }
    Promise.resolve().then(function () { return app().JudgementSettings(); }).then(function (r) { settings = unpackSettings(r); drawSettings(); loadJudgementProviders(); }).catch(function (error) {
      $('judgements-status').textContent = 'The judgement settings could not be read: ' + error;
    });
    $('judgement-calibration-toggle').addEventListener('click', function () {
      var body = $('judgement-calibration-report');
      body.hidden = !body.hidden;
      this.setAttribute('aria-expanded', String(!body.hidden));
    });
    $('judgements-enabled').addEventListener('click', function () {
      saveSettings({ enabled: $('judgements-enabled').getAttribute('aria-checked') !== 'true' }, 'judgements-status',
        $('judgements-enabled').getAttribute('aria-checked') !== 'true' ? 'Judgement agents answer the calls now.' : 'Every call uses its rule now.');
    });
    if ($('judgement-provider')) $('judgement-provider').addEventListener('change', function () {
      var id = $('judgement-provider').value;
      if (id === (settings.provider || '')) return;
      saveSettings({ provider: id, model: '' }, 'judgements-status', 'The provider is chosen. Choose its model next.');
    });
    $('judgement-model').addEventListener('change', function () {
      var name = $('judgement-model').value;
      var save = function () { saveSettings({ model: name }, 'judgements-status', name ? name + ' answers the judgement calls now.' : 'No model is chosen, so every call uses its rule.'); };
      var rec = (judgeProviders || []).filter(function (p) { return p.id === settings.provider; })[0];
      if (name && rec && rec.note) {
        confirmJudgeModel(rec).then(function (yes) {
          if (yes) save();
          else { var m = $('judgement-model'); if (m) m.value = settings.model || ''; }
        });
      } else save();
    });
    $('judgements-save-numbers').addEventListener('click', function () {
      var days = Number($('judgement-retention').value), streak = Number($('judgement-warn-streak').value);
      var status = $('judgements-status');
      if (!Number.isInteger(days) || days < 1) { status.textContent = 'Keep records for a whole number of days, 1 at least.'; return; }
      if (!Number.isInteger(streak) || streak < 1) { status.textContent = 'Warn after a whole number of fallbacks, 1 at least.'; return; }
      saveSettings({ retentionDays: days, warnStreak: streak }, 'judgements-status',
        'Records are kept for ' + days + (days === 1 ? ' day' : ' days') + ', and the tray warns after ' + streak + (streak === 1 ? ' fallback' : ' fallbacks') + ' in a row.');
    });
    $('judgements-save-budgets').addEventListener('click', function () {
      var status = $('judgements-status'), bad = false;
      var budgets = (settings.budgets || []).map(function (b) {
        var input = $('judgement-budgets').querySelector('[data-provider="' + b.provider + '"] input');
        var text = input ? input.value.trim() : '';
        if (text === '') return { provider: b.provider, unit: b.unit, limit: null };
        var n = Number(text);
        if (!Number.isInteger(n) || n < 0) { bad = true; return null; }
        return { provider: b.provider, unit: b.unit, limit: n };
      });
      if (bad) { status.textContent = 'A budget is a whole number, or empty for none.'; return; }
      saveSettings({ budgets: budgets, period: { weekday: Number($('judgement-period-weekday').value), hour: Number($('judgement-period-hour').value) } }, 'judgements-status', 'The budgets and the week they run on are saved.');
    });
    $('judgements-save-pool').addEventListener('click', function () {
      var min = Number($('judgement-pool-min').value), max = Number($('judgement-pool-max').value), status = $('judgements-status');
      if (!Number.isInteger(min) || !Number.isInteger(max) || min < 1 || max < min) { status.textContent = 'The pool takes whole numbers of 1 at least, with at most not below at least.'; return; }
      saveSettings({ pool: { min: min, max: max } }, 'judgements-status', 'The judge’s pool stays between ' + min + ' and ' + max + ' at once.');
    });
    $('judgements-run-calibration').addEventListener('click', function () {
      var b = this, status = $('judgements-status');
      b.disabled = true;
      Promise.resolve().then(function () { return app().RunJudgementCalibration(); }).then(function () {
        status.textContent = 'Calibration is running; its report lands here when it finishes.';
        refreshCalibrationLine();
      }, function (error) {
        status.textContent = 'Calibration could not start: ' + error;
      }).finally(function () { b.disabled = false; });
    });
    $('judgements-use-chosen').addEventListener('click', function () {
      var chosen = this.dataset.model;
      if (chosen) saveSettings({ model: chosen }, 'judgements-status', chosen + ' is the judge now, your choice.');
    });
    $('judgements-open-page').addEventListener('click', openPage);
  }

  document.addEventListener('DOMContentLoaded', function () {
    drawWidget();
    if (!available()) return;
    readState().catch(function () {});
    initSection();
    host().on('settings:changed', function () { loadJudgementProviders().catch(function () {}); });
    host().on('judgements:changed', function (data) {
      acceptState(data);
      if (pageApi && pageApi.refresh) pageApi.refresh();
      refreshCalibrationLine();
    });
    host().onResync(function () {
      readState().catch(function () {});
      if (pageApi && pageApi.refresh) pageApi.refresh();
    });
    $('judgements-status-menu').addEventListener('beforetoggle', function (e) {
      if (e.newState !== 'open') return;
      this.style.removeProperty('--judgements-popup-width');
      this.style.removeProperty('--judgements-popup-height');
      this.style.removeProperty('--judgements-summary-height');
      drawPopup();
    });
    $('judgements-status-menu').addEventListener('toggle', function (e) {
      if (e.newState !== 'open' || !this.matches(':popover-open')) return;
      // The public placement call completes opening before we retain its size.
      window.pudlMenu.place(this);
      var box = this.getBoundingClientRect(), summary = this.querySelector('.judgements-popup-summary');
      this.style.setProperty('--judgements-popup-width', box.width + 'px');
      this.style.setProperty('--judgements-popup-height', box.height + 'px');
      this.style.setProperty('--judgements-summary-height', summary.getBoundingClientRect().height + 'px');
    });
    $('status-judgements').addEventListener('keydown', function (event) {
      if (event.key !== 'ArrowDown' || event.altKey || event.ctrlKey || event.metaKey) return;
      event.preventDefault();
      var menu = $('judgements-status-menu');
      if (!menu.matches(':popover-open')) menu.showPopover();
    });
  });
})();
