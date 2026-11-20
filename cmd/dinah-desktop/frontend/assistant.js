/* The window assistant (judge_assistant.go; the approved mockup is
   docs/mockups/window-assistant, 20 November 2026).

   A small place in the tray where the person says what they want of the
   windows in their own words, or presses one of their tasks. The page tells
   the engine what was asked and which windows are open, by title, kind,
   workspace and state. The engine says one thing to do, and this file does
   it with the window layer's own operations, each of which the person could
   do by hand. What a request changed is kept, so one press puts the windows
   back as they were.

   Assistant Tasks is a window of the workbench in which the person changes
   the list: a task is a name and the words the assistant is asked. */
(function () {
  'use strict';
  var H = window.dinahHelpers, node = H.node;
  function $(id) { return document.getElementById(id); }
  function app() { return H.app(); }
  function P() { return window.pudlWindows; }
  var TASKS_KIND = 'assistant-tasks';
  var KINDS = { chat: 'chat', 'chat-session': 'chat', editor: 'document', 'content-docs': 'document', 'content-diagrams': 'diagram', 'content-slides': 'slides' };
  var CONTENT = /^(editor|content-(docs|diagrams|slides))$/;
  var TILE_LIMIT = 18;

  /* ---- The windows as the assistant is told of them ------------------------ */

  function itemOf(win) {
    var root = win && win.querySelector('.dinah-applet');
    try { return root && root.dataset.item ? JSON.parse(root.dataset.item) : null; } catch (err) { return null; }
  }
  function titleOf(win) {
    var t = win.querySelector('.win-title');
    return t ? t.textContent.trim() : '';
  }
  /* A window the assistant may move: one of the workbench's own kinds that
     stands by itself. A window docked at an edge, a child of another window
     and the Assistant Tasks window are left where they are. */
  function movable(state, key) {
    var win = H.windowOf(key), item = itemOf(win), place = state.place[key];
    if (!win || !item || item.kind === TASKS_KIND) return false;
    if (win.hasAttribute('data-win-parent') || win.hasAttribute('data-win-dock-only')) return false;
    return !(place && /^dock-/.test(place.mode || ''));
  }
  var CHAT_STATES = { working: 'working', waiting: 'waiting', stuck: 'waiting', idle: 'idle' };
  /* Whether a document's window has changes not saved, where its
     application can say; a window that has not been shown since the start
     cannot, and is taken to have none it could lose by being left alone. */
  function unsaved(key) {
    if (!window.dinahContentWindows || !window.dinahContentWindows.listed) return Promise.resolve(null);
    if (window.pudlApplets && window.pudlApplets.started && !window.pudlApplets.started(key)) return Promise.resolve(null);
    return Promise.resolve().then(function () { return window.dinahContentWindows.listed(key); })
      .then(function (d) { return d ? !!d.modified : null; }, function () { return null; });
  }
  /* Every window the assistant may move, the one in front first, with what
     the engine is told of each and what this file needs to act on it. */
  function desk() {
    var state = P().state();
    var order = (state.zOrder || state.open).slice().reverse();
    var rows = order.filter(function (key) { return movable(state, key); }).map(function (key) {
      var win = H.windowOf(key), item = itemOf(win), detail = win.getAttribute('data-win-detail');
      var title = titleOf(win);
      return { id: key, kind: KINDS[item.kind] || item.kind, title: detail && detail !== title ? title + ' (' + detail + ')' : title, name: title,
        workspace: win.getAttribute('data-dinah-group-workspace') || '', state: KINDS[item.kind] === 'chat' ? CHAT_STATES[win.dataset.winState] || '' : '',
        away: !!state.min[key], content: CONTENT.test(item.kind), clean: false, item: item };
    });
    return Promise.all(rows.map(function (w) {
      if (!w.content) return null;
      return unsaved(w.id).then(function (changed) { if (changed) w.state = 'unsaved'; w.clean = changed === false; });
    })).then(function () { return rows; });
  }
  function told(rows) {
    return rows.map(function (w) {
      var out = { id: w.id, kind: w.kind, title: w.title };
      if (w.workspace) out.workspace = w.workspace;
      if (w.state) out.state = w.state;
      if (w.away) out.away = true;
      return out;
    });
  }

  /* ---- Doing what was asked ------------------------------------------------ */

  function plural(n, one, many) { return n === 1 ? '1 ' + one : n + ' ' + many; }
  function names(list) { return list.map(function (w) { return w.name; }).join(', '); }
  /* The grid a number of windows is tiled in. PUDL's zones are on sixths of
     the layer, so a row has one, two, three or six windows. */
  function gridFor(n) {
    if (n <= 3) return [Math.max(n, 1), 1];
    if (n === 4) return [2, 2];
    if (n <= 6) return [3, 2];
    if (n <= 9) return [3, 3];
    return n <= 12 ? [6, 2] : [6, 3];
  }
  /* Tiles windows in the area given as fractions of the layer, the first of
     them at the top left. The last row's windows share its width where that
     falls on the sixths. Each is placed from the last to the first, since a
     window placed comes to the front, and the first is to be in front. */
  function tile(keys, area, row) {
    area = area || { x: 0, y: 0, w: 1, h: 1 };
    var n = keys.length;
    if (!n) return;
    var g = row ? [n, 1] : gridFor(n), cols = g[0], rows = g[1], zones = [];
    keys.forEach(function (key, i) {
      var r = Math.floor(i / cols), inRow = r === rows - 1 ? n - r * cols : cols;
      var across = [1, 2, 3, 6].indexOf(inRow) >= 0 || row ? inRow : cols;
      zones.push({ x: area.x + (i % cols) * area.w / across, y: area.y + r * area.h / rows, w: area.w / across, h: area.h / rows });
    });
    for (var i = n - 1; i >= 0; i--) P().snap(keys[i], zones[i]);
  }
  function showOnly(rows, keep) {
    rows.forEach(function (w) { if (keep.indexOf(w.id) < 0 && !w.away) P().minimize(w.id); });
    keep.slice().reverse().forEach(function (key) { P().raise(key); });
  }
  /* What a request is about to change, for Undo: the arrangement as it is,
     and each window's item, by which a window that is closed is opened
     again. */
  function snapshot(rows) {
    var items = {};
    rows.forEach(function (w) { items[w.id] = w.item; });
    return { state: P().state(), items: items, moved: {}, closed: [] };
  }
  function carryOut(plan, rows) {
    var by = {}, before = snapshot(rows);
    rows.forEach(function (w) { by[w.id] = w; });
    var chosen = (plan.windows || []).filter(function (id) { return by[id]; });
    var showing = rows.filter(function (w) { return !w.away; });
    var said = plan.say || '';
    function move(keys) { keys.forEach(function (k) { before.moved[k] = true; }); }
    switch (plan.do) {
      case 'gather':
      case 'side': {
        var under = plan.do === 'side' && by[plan.under] && chosen.indexOf(plan.under) < 0 ? plan.under : '';
        var all = under ? chosen.concat(under) : chosen;
        if (!all.length) return { said: 'None of the open windows could be found to do that with.', before: null };
        var rest = '';
        if (all.length > TILE_LIMIT) { rest = ' Only the first ' + TILE_LIMIT + ' are arranged.'; all = all.slice(0, TILE_LIMIT); chosen = chosen.slice(0, TILE_LIMIT - (under ? 1 : 0)); }
        move(all);
        showOnly(rows, all);
        if (plan.do === 'gather') tile(all);
        else if (under) { P().snap(under, { x: 0, y: 4 / 6, w: 1, h: 2 / 6 }); tile(chosen, { x: 0, y: 0, w: 1, h: 4 / 6 }, true); }
        else tile(chosen, null, true);
        return { said: said + rest, before: before };
      }
      case 'tile': {
        var keys = showing.map(function (w) { return w.id; });
        if (!keys.length) return { said: 'No window is showing to tile.', before: null };
        var more = keys.length > TILE_LIMIT ? ' The ' + plural(keys.length - TILE_LIMIT, 'window', 'windows') + ' furthest back stayed as they were.' : '';
        keys = keys.slice(0, TILE_LIMIT);
        move(keys);
        tile(keys);
        return { said: 'Tiled the ' + plural(keys.length, 'window', 'windows') + ' that ' + (keys.length === 1 ? 'is' : 'are') + ' showing.' + more, before: before };
      }
      case 'working': {
        /* A chat that has ended its turn and a document with nothing
           unsaved are put away. Every other window stays where it is. */
        var done = showing.filter(function (w) { return w.state === 'idle' || (w.content && w.clean); });
        if (!done.length) return { said: 'Every window that is showing has work in it, or is one I leave alone.', before: null };
        done.forEach(function (w) { P().minimize(w.id); });
        return { said: 'Put away ' + plural(done.length, 'window', 'windows') + ' with no work in ' + (done.length === 1 ? 'it' : 'them') + ': ' + names(done) + '. Nothing is closed.', before: before };
      }
      case 'tidy': {
        /* Closing is limited to chats that are idle and documents with
           nothing unsaved, and the window in front stays, since the person
           is in it. A window that refuses to close stays too. */
        var front = P().top();
        var finished = rows.filter(function (w) { return w.id !== front && (w.state === 'idle' || (w.content && w.clean)); });
        var kept = rows.filter(function (w) { return w.state === 'unsaved'; });
        finished.forEach(function (w) { P().close(w.id); });
        var gone = finished.filter(function (w) { return !P().isOpen(w.id); });
        before.closed = gone.map(function (w) { return w.id; });
        var line = gone.length ? 'Closed ' + plural(gone.length, 'window', 'windows') + ': ' + names(gone) + '.' : 'Nothing was finished, so nothing is closed.';
        if (kept.length) line += ' Left ' + names(kept) + ' open, since ' + (kept.length === 1 ? 'it has' : 'they have') + ' changes not saved.';
        return { said: line, before: gone.length ? before : null };
      }
      case 'all': {
        if (!rows.some(function (w) { return w.away; })) return { said: 'Every window is showing already.', before: null };
        P().restoreAll();
        return { said: 'All ' + plural(rows.length, 'window is', 'windows are') + ' showing again.', before: before };
      }
      case 'front': {
        if (!chosen.length) return { said: 'None of the open windows could be found to bring to the front.', before: null };
        chosen.slice().reverse().forEach(function (key) { P().raise(key); });
        return { said: said, before: before };
      }
      default:
        return { said: said || 'Tell me what to find, gather, arrange or tidy.', before: null };
    }
  }
  function place(key, p) {
    if (!p || /^dock-/.test(p.mode || '')) return;
    if (p.mode === 'zone' && p.zone) P().snap(key, p.zone);
    else if (p.mode === 'floating') P().snap(key, null);
    else P().snap(key, p.mode);
  }
  /* Puts the windows back as a request found them: a window it closed is
     opened again, one it moved goes back to its place, and each is showing
     or put away, and in front of the others, as it was. */
  function putBack(before) {
    var was = before.state;
    return Promise.all(before.closed.map(function (key) {
      return Promise.resolve().then(function () { return window.dinahWorkbench.open(before.items[key]); }).catch(function () { return null; });
    })).then(function () {
      var order = (was.zOrder || was.open).filter(function (key) { return P().isOpen(key); });
      before.closed.forEach(function (key) { before.moved[key] = true; });
      order.forEach(function (key) { if (before.moved[key]) place(key, was.place[key]); });
      order.forEach(function (key) { if (!was.min[key] && before.items[key]) P().raise(key); });
      var now = P().state();
      order.forEach(function (key) { if (was.min[key] && !now.min[key]) P().minimize(key); });
      if (was.top && P().isOpen(was.top) && !was.min[was.top]) P().raise(was.top);
      return before.closed.filter(function (key) { return !P().isOpen(key); }).length;
    });
  }

  /* ---- Asking --------------------------------------------------------------- */

  var busy = false, undoable = null, looked = {};
  function say(text) { H.setText($('assistant-said'), text); }
  function controls() {
    $('assistant-go').disabled = busy;
    $('assistant-undo').disabled = busy || !undoable;
    Array.prototype.forEach.call($('assistant-tasks').querySelectorAll('button'), function (b) { b.disabled = busy; });
  }
  /* ask reads a request and does it, and resolves with the line that says
     what was done. One request runs at a time. */
  function ask(words) {
    words = String(words || '').trim();
    if (!words) return Promise.resolve('');
    if (busy) return Promise.resolve('The assistant is still on the last request.');
    if (!app() || !app().AskAssistant) return Promise.resolve('The assistant is not ready yet.');
    busy = true; controls();
    say('Looking at the windows…');
    return desk().then(function (rows) {
      return Promise.resolve(app().AskAssistant(words, told(rows))).then(function (plan) {
        var done = carryOut(plan || {}, rows);
        if (done.before) undoable = done.before;
        return done.said;
      });
    }).then(function (said) { say(said); return said; }, function (err) {
      var said = 'That could not be done: ' + H.message(err, 'sentence');
      say(said);
      return said;
    }).finally(function () {
      busy = false; controls(); news();
      if ($('assistant-menu').matches(':popover-open')) $('assistant-line').focus({ preventScroll: true });
    });
  }
  function undo() {
    if (busy || !undoable) return;
    var before = undoable;
    undoable = null; busy = true; controls();
    putBack(before).then(function (missing) {
      say(missing ? 'Put back as it was, but ' + plural(missing, 'window', 'windows') + ' could not be opened again.' : 'Put back as it was.');
    }, function (err) { say('It could not be put back: ' + H.message(err, 'sentence')); })
      .finally(function () { busy = false; controls(); news(); });
  }

  /* ---- Since You Last Looked ------------------------------------------------
     What the windows' own states say, read when the popover opens: a chat
     that has ended its turn or begun to wait since the popover was last
     closed, a chat that waits for the person, a chat still working, and a
     document with changes not saved. A line brings its window to the
     front. */
  function news() {
    var list = $('assistant-news');
    return desk().then(function (rows) {
      var lines = [];
      rows.forEach(function (w) {
        var was = looked[w.id];
        if (w.state === 'waiting') lines.push([w, w.name + ' is waiting for you.']);
        else if (w.state === 'idle' && was === 'working') lines.push([w, w.name + ' has finished its turn.']);
        else if (w.state === 'unsaved') lines.push([w, w.name + ' has changes not saved.']);
        else if (w.state === 'working') lines.push([w, w.name + ' is still working.']);
      });
      var items = lines.map(function (line) {
        var li = node('li'), b = node('button', '', line[1]);
        b.type = 'button';
        b.addEventListener('click', function () { if (P().isOpen(line[0].id)) P().raise(line[0].id); });
        li.appendChild(b);
        return li;
      });
      if (!items.length) { var none = node('li', '', 'Nothing needs you.'); none.setAttribute('data-none', ''); items.push(none); }
      if (list.textContent !== items.map(function (li) { return li.textContent; }).join('')) list.replaceChildren.apply(list, items);
      return rows;
    }).catch(function () { return []; });
  }
  function remember() {
    desk().then(function (rows) {
      looked = {};
      rows.forEach(function (w) { looked[w.id] = w.state; });
    }).catch(function () {});
  }

  /* ---- The tasks --------------------------------------------------------------
     The list is the engine's (AssistantTasks), read once and kept here; the
     Assistant Tasks window writes it, and both places are drawn from it. */
  var tasks = [], taskViews = [];
  function drawTasks() {
    var host = $('assistant-tasks');
    var rows = tasks.map(function (t) {
      var row = node('div', 'md-row'), b = node('button', 'md-item', t.name);
      b.type = 'button'; b.title = 'Asks: ' + t.ask; b.disabled = busy;
      b.addEventListener('click', function () { ask(t.ask); });
      row.appendChild(b);
      return row;
    });
    if (!rows.length) rows.push(node('p', 'assistant-none', 'No tasks yet. Add one with Edit Tasks.'));
    host.replaceChildren.apply(host, rows);
    taskViews.forEach(function (draw) { draw(); });
  }
  function setTasks(list) { tasks = (list || []).map(function (t) { return { name: t.name, ask: t.ask }; }); drawTasks(); }
  function loadTasks() {
    if (!app() || !app().AssistantTasks) return Promise.resolve();
    return Promise.resolve(app().AssistantTasks()).then(setTasks).catch(function () {});
  }

  function openTasks() { return window.dinahWorkbench.open(TASKS_KIND); }
  function initTasks(root) {
    root.classList.add('assistant-tasks-window');
    /* The window is never smaller than its list and its form need. PUDL
       reads a window's limits (data-win-min, in pixels) when it takes the
       window in, which is before this runs, and turns them into the two
       properties its stylesheet and resize gestures read; both are set
       here, so the limits hold for this window and for one restored. */
    var frame = root.closest('[data-win]');
    if (frame) {
      frame.setAttribute('data-win-min', '540,440');
      frame.style.setProperty('--win-min-w', '540px'); frame.style.setProperty('--win-min-h', '440px');
    }
    var list = [], chosen = 0, disposed = false, saving = false;
    function button(words, cls) { var b = node('button', cls || 'btn btn-sm', words); b.type = 'button'; return b; }
    var add = button('New Task'), up = button('Move Up'), down = button('Move Down'), restore = button('Restore the Starter Set…');
    var tools = node('div', 'assistant-tasks-tools'); tools.append(add, up, down, restore);
    var rows = node('div', 'assistant-tasks-list'); rows.setAttribute('role', 'listbox'); rows.setAttribute('aria-label', 'Tasks');
    var form = node('form', 'assistant-tasks-form');
    var serial = String(Date.now()) + Math.floor(Math.random() * 1000);
    function group(labelText, control, help) {
      var g = node('div', 'form-group'), label = node('label', 'form-label', labelText);
      control.id = 'assistant-task-' + labelText.replace(/\W+/g, '-').toLowerCase() + '-' + serial; label.htmlFor = control.id;
      g.append(label, control, node('p', 'form-help', help));
      return g;
    }
    var name = node('input', 'form-input'); name.type = 'text'; name.autocomplete = 'off'; name.maxLength = 80;
    var words = node('textarea', 'form-input'); words.maxLength = 600;
    var save = button('Save', 'btn btn-primary'); save.type = 'submit';
    var tryIt = button('Try It', 'btn'), remove = button('Delete Task', 'btn');
    var actions = node('div', 'assistant-tasks-actions'); actions.append(save, tryIt, remove);
    var status = node('p', 'assistant-tasks-status'); status.setAttribute('role', 'status');
    form.append(group('Name', name, 'What the task is called in the assistant’s list.'),
      group('What the Assistant Is Asked', words, 'The same words you could type to the assistant. It is asked this when you choose the task.'), actions, status);
    var body = node('div', 'assistant-tasks-body'); body.append(rows, form);
    root.append(tools, body);

    function note(text) { H.setText(status, text); }
    function complete(t) { return !!(t.name.trim() && t.ask.trim()); }
    function draw() {
      rows.replaceChildren.apply(rows, list.map(function (t, i) {
        var row = node('div', 'md-row'), b = node('button', 'md-item', t.name || 'New Task');
        b.type = 'button'; b.setAttribute('role', 'option'); b.setAttribute('aria-selected', String(i === chosen));
        b.addEventListener('click', function () { chosen = i; draw(); });
        row.appendChild(b);
        return row;
      }));
      var t = list[chosen];
      name.value = t ? t.name : ''; words.value = t ? t.ask : '';
      [name, words, save, tryIt, remove].forEach(function (c) { c.disabled = !t || saving; });
      add.disabled = restore.disabled = saving;
      up.disabled = saving || !t || chosen === 0; down.disabled = saving || !t || chosen === list.length - 1;
    }
    /* The whole list is written each time, in its order. A new task that
       has no name or words yet is not written until it has both. */
    function write(said) {
      if (list.some(function (t) { return !complete(t); })) { note('Give the new task a name and what to ask, and save it, first.'); return Promise.resolve(false); }
      saving = true; draw();
      return Promise.resolve().then(function () { return app().SetAssistantTasks(list); }).then(function (kept) {
        saving = false; setTasks(kept); note(said); return true;
      }, function (err) { saving = false; draw(); note('The tasks could not be saved: ' + H.message(err, 'sentence')); return false; });
    }
    function fromEngine() {
      if (disposed || saving) return;
      // A task being written here that is not saved yet is kept at the end.
      var pending = list.filter(function (t) { return !complete(t); });
      list = tasks.map(function (t) { return { name: t.name, ask: t.ask }; }).concat(pending);
      chosen = Math.max(0, Math.min(chosen, list.length - 1));
      draw();
    }
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var t = list[chosen];
      if (!t) return;
      var n = name.value.trim(), w = words.value.trim();
      if (!n || !w) { note('A task needs a name and what to ask.'); return; }
      t.name = n; t.ask = w;
      write('Saved. It is in the assistant’s list.');
    });
    add.addEventListener('click', function () {
      if (list.some(function (t) { return !complete(t); })) { chosen = list.findIndex(function (t) { return !complete(t); }); draw(); }
      else { list.push({ name: '', ask: '' }); chosen = list.length - 1; draw(); }
      name.focus(); note('Name the task and say what to ask.');
    });
    remove.addEventListener('click', function () {
      var t = list[chosen];
      if (!t) return;
      var called = t.name || 'the new task', wasSaved = complete(t) && tasks.some(function (k) { return k.name === t.name && k.ask === t.ask; });
      list.splice(chosen, 1); chosen = Math.max(0, Math.min(chosen, list.length - 1));
      if (wasSaved) write('Deleted ' + called + '.'); else { draw(); note('Deleted ' + called + '.'); }
    });
    function shift(by) {
      var to = chosen + by;
      if (to < 0 || to >= list.length) return;
      list.splice(to, 0, list.splice(chosen, 1)[0]); chosen = to;
      write('Moved.').then(function (ok) { if (!ok) fromEngine(); });
    }
    up.addEventListener('click', function () { shift(-1); });
    down.addEventListener('click', function () { shift(1); });
    restore.addEventListener('click', function () {
      /* The starter set takes the place of the person's tasks, so it asks
         first. Cancel comes first and has the focus. */
      var dialog = node('dialog', 'dialog'), yes = button('Restore the Starter Set', 'btn btn-primary'), no = button('Cancel', 'btn');
      var buttons = node('div', 'dialog-actions'); buttons.append(no, yes);
      dialog.setAttribute('aria-label', 'Restore the Starter Set?');
      dialog.append(node('h2', 'dialog-title', 'Restore the Starter Set?'), node('p', 'dialog-body', 'The six tasks the assistant started with take the place of your ' + plural(tasks.length, 'task', 'tasks') + '. Tasks you wrote yourself are deleted.'), buttons);
      no.addEventListener('click', function () { dialog.close(); });
      yes.addEventListener('click', function () {
        dialog.close();
        saving = true; draw();
        Promise.resolve().then(function () { return app().RestoreAssistantTasks(); }).then(function (kept) {
          saving = false; list = []; chosen = 0; setTasks(kept); note('The starter set is back.');
        }, function (err) { saving = false; draw(); note('The starter set could not be restored: ' + H.message(err, 'sentence')); });
      });
      dialog.addEventListener('close', function () { dialog.remove(); if (restore.isConnected) restore.focus({ preventScroll: true }); });
      document.body.append(dialog); dialog.showModal(); no.focus();
    });
    tryIt.addEventListener('click', function () {
      var w = words.value.trim();
      if (!w) { note('Say what to ask first.'); return; }
      note('Asking…');
      ask(w).then(function (said) { if (!disposed) note('The assistant: ' + said + (undoable ? ' Undo is in the assistant, in the tray.' : '')); });
    });
    taskViews.push(fromEngine);
    fromEngine();
    if (!tasks.length) loadTasks();
    return { focus: function () { (list.length ? name : add).focus(); }, destroy: function () { disposed = true; taskViews = taskViews.filter(function (f) { return f !== fromEngine; }); } };
  }

  /* ---- The tray --------------------------------------------------------------- */

  function toggle() { $('status-assistant').click(); }
  window.dinahWorkbench.register(TASKS_KIND, { pinnable: false, singleton: true, title: 'Assistant Tasks', glyph: 'list', init: initTasks });
  window.dinahAssistant = { ask: ask, undo: undo, open: toggle, openTasks: openTasks, desk: function () { return desk().then(told); } };
  document.addEventListener('DOMContentLoaded', function () {
    var menu = $('assistant-menu');
    if (!menu) return;
    // The commands are declared once the page's scripts have run, so they
    // follow the Window menu's own rows, which host-menus.js declares.
    if (window.dinahCommands) {
      /* Ctrl+Space is listened for here. The menu bar's dispatcher takes a
         key that is one character or one of its named keys, and Space is
         neither, so the command gives its key to Keyboard Shortcuts by its
         help and the key itself runs the command, which does not run while a
         dialog is open. */
      window.dinahCommands.register({ id: 'assistant', menu: 'window', label: 'Assistant', help: { label: 'Assistant', key: 'Ctrl+Space' }, run: toggle });
      document.addEventListener('keydown', function (e) {
        if (e.code !== 'Space' || !(e.ctrlKey || e.metaKey) || e.altKey || e.shiftKey || e.isComposing) return;
        e.preventDefault(); e.stopPropagation();
        window.dinahCommands.run('assistant');
      }, true);
      window.dinahCommands.register({ id: 'assistant-tasks', menu: 'window', label: 'Assistant Tasks…', run: function () { return openTasks(); } });
    }
    $('assistant-form').addEventListener('submit', function (e) {
      e.preventDefault();
      var line = $('assistant-line'), words = line.value.trim();
      if (!words || busy) return;
      line.value = '';
      ask(words);
    });
    $('assistant-undo').addEventListener('click', undo);
    $('assistant-edit').addEventListener('click', function () { menu.hidePopover(); openTasks().catch(window.dinahHost.fail || function () {}); });
    menu.addEventListener('toggle', function (e) {
      if (e.newState === 'open') { news(); if (!tasks.length) loadTasks(); $('assistant-line').focus({ preventScroll: true }); }
      else remember();
    });
    drawTasks();
    loadTasks();
  });
})();
