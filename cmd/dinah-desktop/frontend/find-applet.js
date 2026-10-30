/* Find and Open combines registered sources, chat rows and commands. */
(function () {
  'use strict';

  function create(adapter) {
    const { fail, nextNotification, showChat } = adapter;
    const $ = id => document.getElementById(id);
    let findSources = [];

    /* ---- Find across the open workspaces ---------------------------------- */
    /* Find and Open lists chats, documents and windows, the recent
       documents first, newest first; typed after ">", or opened by Commands…,
       it lists the commands of the menus instead, with their menu and key. */
    var findMode = 'resources';
    function commandRows(query) {
      return window.dinahCommands.searchable().filter(function (c) {
        return (c.label + ' ' + c.menu).toLowerCase().includes(query);
      }).map(function (c) {
        return { title: c.label, detail: c.menu + (c.shortcut ? ' · ' + c.shortcut : '') + (c.reason ? ' · ' + c.reason : ''), disabled: !c.enabled, open: c.run };
      });
    }
    function drawGoTo() {
      var raw = $('go-to-filter').value.trim(), query = raw.toLowerCase(), found = [];
      if (findMode === 'commands' || raw.charAt(0) === '>') {
        drawFound(commandRows(query.replace(/^>\s*/, '')), true);
        return;
      }
      found = adapter.chatRows();
      if (findMode === 'tools') found = window.dinahWorkbench.launchers().map(function (tool) { return { title: tool.title, detail: tool.description || '', glyph: tool.glyph, open: tool.run }; });
      else if (findMode === 'windows') found = window.dinahOpenWindows ? window.dinahOpenWindows() : [];
      else {
        found = found.concat(dinahSkills.records().map(function (skill) {return {title:skill.name,detail:'Skill · '+skill.origin,open:function(){return dinahSkills.open(skill);}};}));
        findSources.forEach(function (source) { try { found = found.concat(source() || []); } catch (err) { /* A failing source hides only its own items. */ } });
        if (window.dinahOpenWindows) found = found.concat(window.dinahOpenWindows().map(function (win) { return Object.assign({}, win, { detail: 'Open Window · ' + win.detail }); }));
      }
      found = found.filter(function (item) { return (item.title + ' ' + item.detail).toLowerCase().includes(query); });
      /* A recent document has a rank, its place in Recent, and ranked rows come
         first. */
      found.sort(function (a, b) {
        var ra = typeof a.rank === 'number' ? a.rank : Infinity, rb = typeof b.rank === 'number' ? b.rank : Infinity;
        return ra !== rb ? ra - rb : a.title.localeCompare(b.title);
      });
      drawFound(found, false);
    }
    function drawFound(found, commands) {
      var host = $('go-to-list'); host.replaceChildren();
      found.forEach(function (item) {
        var row = document.createElement('tr');
        var cell = document.createElement('td');
        var button = document.createElement('a'); button.href = '#';
        var title = document.createElement('span'); title.textContent = item.title;
        if (item.glyph) { var glyph = document.createElement('span'); glyph.className = 'glyph'; glyph.style.setProperty('--glyph', 'var(--glyph-' + item.glyph + ')'); glyph.setAttribute('aria-hidden', 'true'); title.prepend(glyph, document.createTextNode(' ')); }
        var detail = document.createElement('td'); detail.textContent = item.detail;
        if (item.disabled) { row.classList.add('go-to-disabled'); button.setAttribute('aria-disabled', 'true'); }
        button.append(title); cell.appendChild(button); row.append(cell, detail); host.appendChild(row);
        function activate() { if (item.disabled) return; $('go-to').close(); Promise.resolve().then(item.open).catch(fail); }
        button.addEventListener('click', function (event) { event.preventDefault(); activate(); });
      });
      window.pudlGrid.enhance(host.closest('table'));
      $('go-to-empty').textContent = commands ? 'No matching commands.' : findMode === 'tools' ? 'No matching tools.' : findMode === 'windows' ? 'No matching open windows.' : 'No matching items in the open workspaces.';
      $('go-to-empty').hidden = !!host.children.length;
    }
    function openGoTo(mode) {
      findMode = mode || 'resources';
      $('go-to-title').textContent = findMode === 'tools' ? 'Find a Tool' : findMode === 'windows' ? 'All Windows' : findMode === 'commands' ? 'Commands' : 'Find and Open';
      $('go-to-filter').placeholder = findMode === 'tools' ? 'Find a Tool' : findMode === 'windows' ? 'Find an Open Window' : findMode === 'commands' ? 'Find a Command' : 'Find a Chat, Document or Window (Type > for Commands)';
      $('go-to-filter').value = ''; drawGoTo();
      if (!$('go-to').open) $('go-to').showModal();
      $('go-to-filter').focus();
    }
    $('go-to-filter').addEventListener('input', drawGoTo);
    $('go-to-filter').addEventListener('keydown', function (event) {
      var first = $('go-to-list').firstElementChild;
      if (event.key === 'ArrowDown' && first) { event.preventDefault(); first.focus(); }
      /* Enter opens the first row, so Ctrl+P then Enter opens the last
         document. */
      if (event.key === 'Enter' && first) { event.preventDefault(); first.querySelector('a').click(); }
    });
    function frontWindowKind() {
      var s = window.pudlWindows && window.pudlWindows.state(), win = s && s.top && document.querySelector('[data-win="' + CSS.escape(s.top) + '"]');
      return win && !s.min[s.top] ? win.dataset.winKind || '' : '';
    }
    window.dinahCommands.register({ id: 'find-and-open', glyph: 'search', menu: 'view', mnemonic: 'O', label: 'Find and Open…', shortcut: 'Ctrl+P', run: function () { openGoTo(); } });
    window.dinahCommands.register({ id: 'commands', glyph: 'slash', menu: 'view', mnemonic: 'M', label: 'Commands…', shortcut: 'Ctrl+Shift+P', run: function () { openGoTo('commands'); } });
    window.dinahCommands.register({ id: 'go-to-question', menu: 'view', mnemonic: 'Q', label: 'Go to the Question', shortcut: 'F9', run: function () { nextNotification(); } });
    /* In a Terminal, Ctrl+J is a line feed, so the Terminal keeps it. */
    window.dinahCommands.register({ id: 'go-to-chat', glyph: 'comments', menu: 'view', mnemonic: 'G', label: 'Go to the Chat', shortcut: 'Ctrl+J', enabled: function () { return frontWindowKind() !== 'terminal'; }, run: function () { return showChat(); } });
    window.dinahCommands.register({ id: 'notifications', glyph: 'warning', menu: 'view', mnemonic: 'N', label: 'Notifications', shortcut: 'Ctrl+Shift+I', run: function () { return window.dinahWorkbench.open('needs-you'); } });
    // Find and Open's own keys, listed with the View menu's.
    window.dinahCommands.addHelp('View', [
      { where: 'Find and Open', label: 'Open the First Match', key: 'Enter' },
      { where: 'Find and Open', label: 'Move Into the Matches', key: 'Down' },
      { where: 'Find and Open', label: 'Search the Commands Instead', key: '> typed first' }]);

    function addSource(source) {
      findSources.push(source);
      return () => { findSources = findSources.filter(item => item !== source); };
    }
    return { drawGoTo, openGoTo, addSource };
  }

  window.dinahFind = { create };
})();
