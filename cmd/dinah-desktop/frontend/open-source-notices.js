/* Open-Source Notices: the window Help, About opens with its Open-Source
   Notices button. It lists every open-source component in the programs with
   its version and licence, and shows the chosen one's notices as its authors
   wrote them. The list is third-party-notices.json, which tools/notices
   writes from what the programs link and what the page bundles; a product
   built from this source serves its own from its product directory. The
   approved mockup is docs/mockups/about-notices.html (17 November 2026). */
(function () {
  'use strict';
  var node = window.dinahHelpers.node;
  var loaded = null;
  // The list is read once, when the window is first opened; nothing is fetched at start.
  function load() {
    if (!loaded) {
      loaded = fetch('third-party-notices.json', { credentials: 'same-origin' }).then(function (res) {
        if (!res.ok) throw new Error('The notices could not be read.');
        return res.json();
      }).then(function (data) { return data.components || []; }).catch(function (error) { loaded = null; throw error; });
    }
    return loaded;
  }
  function versionWords(c) { return c.version ? 'Version ' + c.version : 'As bundled'; }
  function noticeText(c) {
    return c.notices.map(function (n) { return c.notices.length > 1 ? n.file + '\n\n' + n.text.trim() : n.text.trim(); }).join('\n\n----------------------------------------\n\n');
  }
  // Every notice in one text, for Save All Notices.
  function allText(components) {
    var product = (window.dinahProduct && window.dinahProduct.name) || document.documentElement.dataset.productName || '';
    var out = ['Third-Party Notices', '', product + ' includes the open-source software listed here, with the licence texts its authors ask to be kept with it.', ''];
    components.forEach(function (c) {
      out.push('========================================================================', c.name + (c.version ? ' ' + c.version : ''), c.where + '. ' + c.licence + '.' + (c.note ? ' ' + c.note : ''), '', noticeText(c), '');
    });
    return out.join('\n');
  }
  function init(root) {
    root.classList.add('open-source-notices');
    var components = [], chosen = -1, disposed = false;
    var list = node('div', 'notices-list'), detail = node('div', 'notices-detail');
    var filter = node('input', 'form-input'); filter.type = 'search'; filter.placeholder = 'Filter by Name or Licence'; filter.setAttribute('aria-label', 'Filter by name or licence');
    var count = node('p', 'notices-count'); count.setAttribute('role', 'status');
    var rows = node('div', 'notices-rows'), table = node('table', 'data-table'), head = node('thead'), body = node('tbody');
    table.setAttribute('aria-label', 'Open-source components');
    var headRow = node('tr');
    ['Component', 'Version', 'Licence'].forEach(function (words) { var th = node('th', '', words); th.scope = 'col'; headRow.appendChild(th); });
    head.appendChild(headRow); table.append(head, body); rows.appendChild(table);
    list.append(filter, count, rows);
    var name = node('h3', 'notices-name'), meta = node('p', 'notices-meta'), text = node('pre', 'notices-text'); text.tabIndex = 0;
    detail.setAttribute('aria-live', 'polite');
    var actions = node('div', 'notices-actions');
    var copy = node('button', 'btn notices-copy', 'Copy This Notice'); copy.type = 'button';
    var save = node('button', 'btn', 'Save All Notices…'); save.type = 'button';
    copy.disabled = save.disabled = true;
    actions.append(copy, save);
    detail.append(name, meta, text, actions);
    // The window is the container the panes measure themselves against, so a narrow window stacks them.
    var panes = node('div', 'notices-panes');
    panes.append(list, detail);
    root.appendChild(panes);

    function show(i) {
      chosen = i;
      var c = components[i];
      name.textContent = name.title = c.name;
      meta.textContent = meta.title = versionWords(c) + ' · ' + c.licence + ' · ' + c.where + '.' + (c.note ? ' ' + c.note : '');
      text.textContent = noticeText(c);
      text.scrollTop = 0;
      Array.prototype.forEach.call(body.rows, function (row) { row.setAttribute('aria-selected', String(Number(row.dataset.index) === i)); });
      copy.disabled = false;
    }
    function applyFilter() {
      var words = filter.value.trim().toLowerCase(), shown = 0;
      Array.prototype.forEach.call(body.rows, function (row) {
        var c = components[row.dataset.index], match = !words || (c.name + ' ' + c.licence).toLowerCase().indexOf(words) >= 0;
        if (row.hidden === match) row.hidden = !match;
        if (match) shown++;
      });
      count.textContent = shown === components.length ? components.length + ' components' : shown + ' of ' + components.length + ' components';
    }
    filter.addEventListener('input', applyFilter);
    copy.addEventListener('click', function () {
      if (chosen < 0) return;
      window.dinahChatText.copy(noticeText(components[chosen]), 'Copied', copy);
    });
    save.addEventListener('click', function () {
      window.dinahResources.saveFile({ name: 'THIRD-PARTY-NOTICES.txt', title: 'Save All Notices', blob: new Blob([allText(components)], { type: 'text/plain' }) }).catch(window.dinahHost.fail);
    });
    count.textContent = 'Reading the notices…';
    load().then(function (found) {
      if (disposed) return;
      components = found;
      components.forEach(function (c, i) {
        var row = body.insertRow(); row.dataset.index = i;
        var open = node('button', 'notices-open', c.name); open.type = 'button';
        open.addEventListener('click', function () { show(i); });
        row.insertCell().appendChild(open);
        row.insertCell().textContent = c.version || 'as bundled';
        row.insertCell().textContent = c.licence;
      });
      save.disabled = !components.length;
      applyFilter();
      if (components.length) show(0);
    }).catch(function (error) {
      if (disposed) return;
      count.textContent = window.dinahHelpers.message(error, 'sentence');
    });
    return { focus: function () { filter.focus(); }, destroy: function () { disposed = true; } };
  }
  function open() { return window.dinahWorkbench.open('open-source-notices'); }
  window.dinahWorkbench.register('open-source-notices', { pinnable: false, singleton: true, title: 'Open-Source Notices', glyph: 'file', mode: 'maximized', init: init });
  window.dinahOpenSourceNotices = { open: open };
  document.addEventListener('DOMContentLoaded', function () {
    var button = document.getElementById('about-notices');
    if (!button) return;
    button.addEventListener('click', function () {
      var about = document.getElementById('about');
      if (about && about.open) about.close();
      open().catch(window.dinahHost.fail);
    });
  });
})();
