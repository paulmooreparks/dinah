/* A Terminal running a real shell: PowerShell, the Command Prompt, Git Bash,
   WSL or any other the computer has (terminal_shells.go). Go runs the shell
   in a pseudoconsole; this draws it with the same xterm.js and theme as
   Dinah's own shell (terminal-applet.js), sends what is typed and each
   change of size, and writes what the shell draws. Typed text is sent in
   order, one call at a time, with what is typed meanwhile gathered into the
   next. When the shell ends, Enter starts it again. */
(function () {
  'use strict';
  const app = window.dinahHelpers.app;
  function host() { return window.dinahHost; }
  const message = window.dinahHelpers.message;
  function bytes(base64) { var s = atob(base64), out = new Uint8Array(s.length); for (var i = 0; i < s.length; i++) out[i] = s.charCodeAt(i); return out; }

  function init(root, opts, item) {
    opts = opts || {};
    root.classList.add('pc-terminal', 'pc-terminal-fill');
    root.dataset.terminalShell = item.shell;
    var screen = document.createElement('div'); screen.className = 'pc-terminal-screen'; root.appendChild(screen);
    var key = 'dinah-terminal:' + item.instanceId, saved = window.dinahUIState && item.instanceId ? dinahUIState.get(key, {}) : {};
    var explicitFontSize = Number.isInteger(saved.fontSize) && saved.fontSize >= 6 && saved.fontSize <= 22;
    var fontSize = explicitFontSize ? saved.fontSize : (window.dinahAppearance ? dinahAppearance.content('terminal').size : 14);
    var term, fit, observer, destroyed = false, id = '', ended = false, pending = '', sending = false, resizeTimer = 0, offOutput, offExit;

    function save() { if (window.dinahUIState && item.instanceId) dinahUIState.set(key, { fontSize: explicitFontSize ? fontSize : undefined }); }
    /* The window is titled with what the shell or the program running in it
       sets as its title (the OSC 0 and 2 sequences, which the pseudoconsole
       passes on and xterm.js reports with onTitleChange), and otherwise
       with the shell's name. The folder the shell started in is its
       detail. */
    var shellName = '', shellFolder = '', shown = '';
    function title(name, folder) {
      if (name !== undefined) { shellName = name || ''; shellFolder = folder || ''; }
      window.dinahRender.enqueue(title, root, function () {
        var text = (!ended && shown) || shellName;
        var win = root.closest('[data-win]');
        if (text && win && window.pudlWindows && window.pudlWindows.retitle) window.pudlWindows.retitle(win.dataset.win, text, shellFolder);
      });
    }
    /* What is typed goes to the shell in order: one call at a time, with
       anything typed while a call is out sent together after it. */
    function send(data) {
      if (!id || ended) return;
      pending += data;
      if (sending) return;
      sending = true;
      (function next() {
        var chunk = pending; pending = '';
        Promise.resolve(app().TerminalWrite(id, chunk)).catch(function () {}).then(function () {
          if (pending && !destroyed && !ended) next(); else sending = false;
        });
      })();
    }
    function resize() {
      if (!fit || destroyed || !screen.clientWidth || !screen.clientHeight) return;
      fit.fit();
    }
    /* A shell may draw, or even end, before the call that started it
       returns, so while one starts, events for shells not yet known are
       kept, and its own are played once its ID is known. */
    var early = null;
    function output(m) { if (m.id === id) term.write(bytes(m.data)); else if (early) early.push(['output', m]); }
    function exit(m) {
      if (m.id !== id) { if (early) early.push(['exit', m]); return; }
      ended = true; root.dataset.terminalReady = 'false';
      shown = ''; title();
      term.write('\r\n\x1b[2mThe shell ended with code ' + m.code + '. Press Enter to start it again.\x1b[0m\r\n');
    }
    function start() {
      ended = false;
      early = [];
      root.dataset.terminalReady = 'false';
      return Promise.resolve(item.preparedId ? app().StartPreparedExtensionTerminal(item.preparedId, term.cols, term.rows) : app().TerminalStart(item.shell, item.ref || null, term.cols, term.rows)).then(function (started) {
        if (destroyed) { app().TerminalClose(started.id); return; }
        id = started.id;
        root.dataset.terminalId = id;
        root.dataset.terminalReady = 'true';
        var kept = early; early = null;
        kept.forEach(function (e) { if (e[1].id === id) (e[0] === 'output' ? output : exit)(e[1]); });
        title(started.name, started.folder);
        if (root.closest('.win.active')) term.focus();
      }).catch(function (err) {
        early = null;
        ended = true;
        term.write('\r\n\x1b[31mThe shell could not start: ' + message(err).replace(/\x1b/g, '') + '\x1b[0m\r\n');
      });
    }
    function copySelection() {
      var text = term.getSelection();
      if (!text) return false;
      host().copy(text);
      term.clearSelection();
      return true;
    }
    function paste() {
      return Promise.resolve(app().ClipboardRead()).then(function (c) { if (c && c.text) term.paste(c.text); }).catch(host().fail);
    }

    function sized() {
      return new Promise(function (resolve) {
        if (screen.clientWidth && screen.clientHeight) { resolve(); return; }
        var watcher = new ResizeObserver(function () {
          if (destroyed || (screen.clientWidth && screen.clientHeight)) { watcher.disconnect(); resolve(); }
        });
        watcher.observe(screen);
      });
    }
    var ready = Promise.all([window.dinahTerminal.assets(), window.dinahUIState ? dinahUIState.ready : Promise.resolve(), window.dinahAppearance ? dinahAppearance.ready : Promise.resolve()]).then(sized).then(function () {
      if (destroyed) return;
      term = new window.Terminal({ cursorBlink: true, fontFamily: getComputedStyle(root).getPropertyValue('--mono').trim() || 'Consolas, "Courier New", monospace', fontSize: fontSize, theme: window.dinahTerminal.theme(root), scrollback: 5000, minimumContrastRatio: 4.5, allowProposedApi: false });
      fit = new window.FitAddon.FitAddon(); term.loadAddon(fit); term.open(screen); fit.fit();
      term.textarea.setAttribute('aria-label', 'Terminal');
      /* A key the page has already taken, such as the menu bar's, is not
         the shell's. Ctrl+C copies when text is selected, as in Windows
         Terminal, and Ctrl+Shift+C and Ctrl+Shift+V always copy and paste. */
      term.attachCustomKeyEventHandler(function (e) {
        if (e.defaultPrevented) return false;
        if (e.type !== 'keydown') return true;
        var k = e.key.toLowerCase();
        if (e.ctrlKey && !e.altKey && k === 'c' && (e.shiftKey || term.hasSelection())) { e.preventDefault(); copySelection(); return false; }
        if (e.ctrlKey && e.shiftKey && !e.altKey && k === 'v') { e.preventDefault(); paste(); return false; }
        if (ended && e.key === 'Enter') { e.preventDefault(); term.write('\r\n'); start(); return false; }
        return true;
      });
      term.onData(send);
      // Control characters are left out of a title, and a long one is cut.
      term.onTitleChange(function (t) { shown = String(t || '').replace(/[\x00-\x1f\x7f]/g, '').trim().slice(0, 200); title(); });
      term.onResize(function (size) {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(function () { if (id && !ended) app().TerminalResize(id, size.cols, size.rows).catch(function () {}); }, 80);
      });
      offOutput = host().on('terminal:output', function (m) { if (m && !destroyed) output(m); });
      offExit = host().on('terminal:exit', function (m) { if (m && !destroyed) exit(m); });
      observer = new ResizeObserver(resize); observer.observe(screen);
      document.addEventListener('pudl:theme-change', applyTheme); document.addEventListener('dinah:appearance-change', applyTheme); applyTheme();
      return start();
    }).catch(function (error) { if (!destroyed) { screen.textContent = 'The terminal could not start: ' + message(error); screen.classList.add('pc-terminal-failed'); } });
    function applyTheme() { if (!term || destroyed) return; term.options.theme = window.dinahTerminal.theme(root); if (window.dinahAppearance) { var appearance = dinahAppearance.content('terminal'); term.options.fontFamily = appearance.font; if (!explicitFontSize) { fontSize = appearance.size; term.options.fontSize = fontSize; } } if (window.dinahRender) window.dinahRender.enqueue(applyTheme, root, resize, null, 'terminal appearance'); else resize(); }
    function textSize(n) { if (!term) return; explicitFontSize = true; fontSize = Math.max(6, Math.min(22, n)); term.options.fontSize = fontSize; resize(); save(); }

    return {
      ready: ready,
      state: function () { return ''; },
      setState: function () {},
      commands: function () {
        return [
          { label: 'Copy', disabled: !term || !term.hasSelection(), run: copySelection },
          { label: 'Paste', run: paste },
          { label: 'Clear Terminal', run: function () { if (term) term.clear(); } },
          { label: 'Restart the Shell', run: function () { if (!term) return; var old = id; id = ''; if (old) app().TerminalClose(old); term.reset(); start(); } },
          { label: 'Larger Text', run: function () { textSize(fontSize + 1); } },
          { label: 'Smaller Text', run: function () { textSize(fontSize - 1); } },
          { label: 'Reset Text Size', run: function () { explicitFontSize = false; if (!window.dinahAppearance && term) { fontSize = 14; term.options.fontSize = fontSize; } applyTheme(); save(); } }];
      },
      resize: resize,
      destroy: function () {
        destroyed = true;
        window.dinahRender.cancel(title);
        if (id && !ended) app().TerminalClose(id);
        if (offOutput) offOutput();
        if (offExit) offExit();
        if (observer) observer.disconnect();
        clearTimeout(resizeTimer);
        document.removeEventListener('pudl:theme-change', applyTheme); document.removeEventListener('dinah:appearance-change', applyTheme); if (window.dinahRender) window.dinahRender.cancel(applyTheme);
        if (term) term.dispose();
      }
    };
  }
  /* The shells this computer has, for Tools › New Terminal With, read once
     as the page loads; the menu is drawn again when they arrive. */
  var shells = [];
  /* Settings › Terminal chooses the shell a new Terminal runs. */
  function drawChoice(s) {
    var select = document.getElementById('set-terminal-shell');
    if (!select) return;
    select.replaceChildren();
    (s.shells || []).forEach(function (shell) { select.add(new Option(shell.name, shell.id)); });
    select.value = s.default || '';
  }
  async function refreshShells() {
    var result = await app().TerminalShells();
    shells = result.shells || []; drawChoice(result);
    document.dispatchEvent(new CustomEvent('dinah:launchers-change'));
    var list = document.getElementById('terminal-profiles');
    if (list && app().TerminalProfiles) {
      list.replaceChildren();
      (await app().TerminalProfiles()).forEach(function (profile) {
        var row = document.createElement('div'), text = document.createElement('span'), remove = document.createElement('button');
        row.className = 'hstack gap-2'; text.textContent = profile.name + ' (' + profile.path + ')'; remove.type = 'button'; remove.className = 'btn btn-sm'; remove.textContent = 'Remove'; remove.setAttribute('aria-label', 'Remove ' + profile.name);
        remove.addEventListener('click', function () { action(async function () { await app().RemoveTerminalProfile(profile.id); await refreshShells(); }); });
        row.append(text, remove); list.append(row);
      });
    }
  }
  async function action(fn) { var status = document.getElementById('terminal-profile-status'); try { await fn(); if (status) status.textContent = ''; } catch (error) { if (status) status.textContent = message(error); } }
  document.addEventListener('DOMContentLoaded', function () {
    var a = window.dinahHelpers.app();
    if (!a || !a.TerminalShells) return;
    action(refreshShells);
    var refresh = document.getElementById('terminal-refresh');
    if (refresh) refresh.addEventListener('click', function () { action(refreshShells); });
    var open = document.getElementById('terminal-open-selected');
    if (open) open.addEventListener('click', function () { action(async function () { var path = await dinahResources.currentPath(); await dinahResources.openTerminal(path, null, document.getElementById('set-terminal-shell').value); }); });
    var add = document.getElementById('terminal-profile-add');
    if (add) add.addEventListener('click', function () { action(async function () {
      var args = JSON.parse(document.getElementById('terminal-profile-args').value);
      if (!Array.isArray(args) || args.some(function (arg) { return typeof arg !== 'string'; })) throw Error('Arguments must be a JSON array of strings.');
      await a.AddTerminalProfile(document.getElementById('terminal-profile-name').value, document.getElementById('terminal-profile-path').value, args);
      await refreshShells();
    }); });
    var select = document.getElementById('set-terminal-shell');
    if (select) select.addEventListener('change', function () {
      Promise.resolve(a.SetTerminalShell(select.value)).catch(function (err) {
        window.dinahHost.toast(message(err), 'danger', { title: 'Settings:' });
        Promise.resolve(a.TerminalShells()).then(drawChoice);
      });
    });
  });
  window.dinahShellTerminal = { init: init, refresh: refreshShells, shells: function () { return shells.slice(); } };
})();
