/* The toolbar Dinah gives a yavc.app content applet (pudlHost.commandToolbar,
   yavc.app command-toolbar-design.md). The applet hands over its app key,
   its default command ids, separated into groups by '-', and its command
   table; the toolbar is a PUDL folding toolbar of buttons that each say
   which command they are, so PUDL draws their words, glyphs, keys and state
   from the table and folds the groups as the window narrows. A command a
   document does not have now is left out until it has it again.

   The person chooses what the toolbar holds. Its last control, a pencil at
   the far right, opens Customize Toolbar, as the applet's View menu does
   (the two commands this returns for the applet's table, Show the Toolbar
   and Customize Toolbar…). What is chosen is kept for each application, in
   the applets' settings under command-toolbars, and every open window of
   that application follows a change. This is yavc.app's own controller
   (its internal/web/static/command-toolbar.js) written for this window. */
(function () {
  'use strict';

  var FILE = 'command-toolbars', SHOW = 'common-toolbar-show', EDIT = 'common-toolbar-customize', LIMIT = 60, serial = 0;

  function field(command, name) { var v = command[name]; return typeof v === 'function' ? v.call(command) : v; }
  function esc(text) { return String(text).replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; }); }
  function toast(text) { if (window.dinahHost && window.dinahHost.toast) window.dinahHost.toast(text, 'danger'); }

  /* The ids a toolbar may hold: strings, each once, at most 60, with no
     separator first, last or doubled. Anything else in what was kept is
     passed over, so a damaged setting cannot break the toolbar. */
  function items(value, fallback) {
    if (!Array.isArray(value)) value = fallback || [];
    var seen = new Set(), out = [];
    value.slice(0, LIMIT).forEach(function (id) {
      if (typeof id !== 'string' || !id || id.length > 128) return;
      if (id === '-') { if (out.length && out[out.length - 1] !== '-') out.push(id); }
      else if (!seen.has(id) && id !== SHOW && id !== EDIT) { seen.add(id); out.push(id); }
    });
    if (out[out.length - 1] === '-') out.pop();
    return out;
  }
  function normalize(value, defaults) {
    var v = value && typeof value === 'object' && !Array.isArray(value) && value.version === 1 ? value : {};
    return { version: 1, visible: typeof v.visible === 'boolean' ? v.visible : true,
      inRibbon: typeof v.inRibbon === 'boolean' ? v.inRibbon : true, items: items(v.items, defaults) };
  }

  /* What is known of one command: its caption, the name Customize Toolbar
     lists it by (toolbarLabel where the applet gives one), the choice group
     it stands for and its group's folding priority. */
  function entry(c, path) {
    var caption = String(field(c, 'label') || c.id || ''), label = String(field(c, 'toolbarLabel') || caption);
    var choice = null, priority = 0;
    try { choice = field(c, 'toolbarChoice'); priority = field(c, 'toolbarPriority'); } catch (e) { choice = null; priority = 0; }
    return { id: c.id, caption: caption, label: path ? path + ' / ' + label : label, own: label,
      choice: typeof choice === 'string' && /^[a-z][a-z0-9-]{0,63}$/.test(choice) ? choice : null,
      priority: typeof priority === 'number' && Number.isFinite(priority) ? Math.max(-100, Math.min(100, priority)) : 0 };
  }
  /* The commands the table has now, by id. Only the table's own rows are
     read unless deep is asked for: a menu command's items are its menu's,
     which PUDL reads when the menu opens, and reading them here on every
     change of a command's state made the Editor rebuild its Insert
     catalogue through its Markdown engine at every keystroke. They are
     read for Customize Toolbar, and while the toolbar holds a command that
     is only in a menu. */
  function catalog(table, deep) {
    var out = new Map(), walked = new Set();
    function walk(commands, path, depth) {
      if (!Array.isArray(commands) || depth > 8) return;
      commands.forEach(function (c) {
        if (!c || typeof c !== 'object' || walked.has(c)) return;
        walked.add(c);
        var e;
        try { e = entry(c, path); } catch (err) { return; /* A command not ready yet is read again at the next change. */ }
        if (typeof c.id === 'string' && c.id !== SHOW && c.id !== EDIT && !out.has(c.id)) out.set(c.id, e);
        if (!deep) return;
        try { walk(field(c, 'items'), e.label, depth + 1); } catch (err) { /* A menu not ready yet is read again at the next change. */ }
      });
    }
    walk(table() || [], '', 0);
    return out;
  }

  function attach(root, options) {
    if (!/^[a-z][a-z0-9-]{0,63}$/.test(options.app || '')) throw new Error('A toolbar needs a plain app key.');
    var slot = root.querySelector('[data-toolbar-slot]');
    if (!slot) throw new Error('A command toolbar needs a toolbar slot.');
    var config = window.pudlHost && window.pudlHost.config;
    var label = options.label || 'Document', prefix = 'content-toolbar-' + (++serial) + '-';
    var prefs = normalize(null, options.defaults), drawn = null;
    var destroyed = false, loading = true, saving = false, off = null, dialog = null, opening = null, frame = 0, focusFrame = 0;
    var bar = document.createElement('div');
    bar.className = 'toolbar content-toolbar';
    bar.setAttribute('role', 'toolbar');
    bar.setAttribute('aria-label', label + ' Commands');
    bar.setAttribute('data-toolbar-fold', '');
    slot.appendChild(bar);

    function commandsChanged() { if (window.pudlCommands) window.pudlCommands.changed(); }
    /* A change of any command's state is followed by many in a burst (each
       keystroke), so the toolbar is looked at once a frame. */
    function changed() { if (!frame) frame = requestAnimationFrame(render); }
    function known() {
      var top = catalog(options.table, false);
      if (prefs.items.every(function (id) { return id === '-' || top.has(id); })) return top;
      return catalog(options.table, true);
    }
    function render() {
      cancelAnimationFrame(frame); frame = 0;
      if (destroyed) return;
      var have = known();
      var shown = prefs.items.filter(function (id) { return id === '-' || have.has(id); });
      var signature = JSON.stringify([prefs.visible, prefs.inRibbon, shown.map(function (id) { return id === '-' ? id : [id, have.get(id).choice, have.get(id).priority]; })]);
      if (signature === drawn) return;
      drawn = signature;
      bar.hidden = !prefs.visible;
      slot.hidden = !prefs.visible;
      if (prefs.inRibbon) bar.removeAttribute('data-toolbar-ribbon'); else bar.setAttribute('data-toolbar-ribbon', 'hide');
      var ownedFocus = bar.contains(document.activeElement), focused = ownedFocus && document.activeElement.getAttribute('data-command');
      bar.replaceChildren();
      var group = null;
      shown.forEach(function (id) {
        if (id === '-') { group = null; return; }
        var c = have.get(id);
        if (!group) {
          group = document.createElement('div');
          group.className = 'toolbar-group';
          group.setAttribute('role', 'group');
          group.setAttribute('aria-label', c.caption);
          group.setAttribute('data-toolbar-glyph', 'more');
          group.setAttribute('data-toolbar-priority', String(c.priority));
          bar.appendChild(group);
        }
        group.setAttribute('data-toolbar-priority', String(Math.max(Number(group.getAttribute('data-toolbar-priority')) || 0, c.priority)));
        if (c.choice) {
          var seg = document.createElement('div');
          seg.className = 'seg sm';
          seg.setAttribute('data-command-choice', c.choice);
          group.appendChild(seg);
          return;
        }
        var button = document.createElement('button');
        button.type = 'button';
        button.setAttribute('data-command', id);
        group.appendChild(button);
      });
      /* The pencil that opens Customize Toolbar, kept at the row's end and
         drawn as its glyph alone (PUDL's data-toolbar-end and
         data-toolbar-glyph-only). */
      var tools = document.createElement('div');
      tools.className = 'toolbar-group';
      tools.setAttribute('role', 'group');
      tools.setAttribute('aria-label', 'Toolbar Settings');
      tools.setAttribute('data-toolbar-end', '');
      tools.setAttribute('data-toolbar-priority', '100');
      var edit = document.createElement('button');
      edit.type = 'button';
      edit.setAttribute('data-command', EDIT);
      edit.setAttribute('data-toolbar-glyph-only', '');
      tools.appendChild(edit);
      bar.appendChild(tools);
      commandsChanged();
      if (window.pudlToolbar) window.pudlToolbar.enhance(slot);
      if (ownedFocus) {
        cancelAnimationFrame(focusFrame);
        focusFrame = requestAnimationFrame(function () {
          focusFrame = 0;
          if (destroyed || (!bar.contains(document.activeElement) && document.activeElement !== document.body)) return;
          if (window.pudlToolbar && window.pudlToolbar.fit) window.pudlToolbar.fit(bar);
          var visible = Array.from(bar.querySelectorAll('button, input, select, a[href]')).filter(function (b) { return !b.disabled && b.getAttribute('aria-disabled') !== 'true' && b.getClientRects().length && getComputedStyle(b).visibility !== 'hidden'; });
          var back = visible.find(function (b) { return focused && b.getAttribute('data-command') === focused; }) || visible[0];
          (back || root).focus({ preventScroll: true });
        });
      }
    }

    function reason() { return !config || !config.load || !config.set ? 'Toolbar settings cannot be kept here.' : loading ? 'Toolbar settings are loading.' : saving ? 'Toolbar settings are saving.' : null; }
    function persist(next) {
      if (destroyed) return Promise.resolve(false);
      saving = true; commandsChanged();
      var update = {}; update[options.app] = next;
      return Promise.resolve(config.set(FILE, update)).then(function (error) {
        if (error) throw new Error(error);
        if (destroyed) return false;
        prefs = normalize(next, options.defaults); render(); return true;
      }).finally(function () { saving = false; if (!destroyed) commandsChanged(); });
    }
    function toggle() {
      return persist(Object.assign({}, prefs, { visible: !prefs.visible })).catch(function (err) {
        if (!destroyed) toast('Toolbar settings were not saved: ' + (err && err.message || err));
      });
    }

    function customize() {
      if (opening) return opening;
      var draft = { version: 1, items: prefs.items.slice(), visible: prefs.visible, inRibbon: prefs.inRibbon }, entries = catalog(options.table, true);
      var ui = null, busy = false;
      function q(sel) { return ui.body.querySelector(sel); }
      function lists(index) {
        var available = q('[data-available]'), selected = q('[data-selected]'), query = q('[data-search]').value.toLowerCase();
        var prior = available.value;
        available.innerHTML = Array.from(entries.values()).filter(function (c) { return draft.items.indexOf(c.id) < 0 && (c.label + ' ' + c.id).toLowerCase().indexOf(query) >= 0; })
          .sort(function (a, b) { return a.label.localeCompare(b.label); }).map(function (c) { return '<option value="' + esc(c.id) + '">' + esc(c.label) + '</option>'; }).join('');
        if (Array.from(available.options).some(function (o) { return o.value === prior; })) available.value = prior;
        else available.selectedIndex = available.options.length ? 0 : -1;
        selected.innerHTML = draft.items.map(function (id, i) { return '<option value="' + i + '">' + esc(id === '-' ? 'Separator' : entries.has(id) ? entries.get(id).label : id + ' (not in this document)') + '</option>'; }).join('');
        selected.selectedIndex = draft.items.length ? Math.max(0, Math.min(index == null ? 0 : index, draft.items.length - 1)) : -1;
        buttons();
      }
      function buttons() {
        var available = q('[data-available]'), selected = q('[data-selected]');
        var i = selected.selectedIndex, count = draft.items.length, has = !!available.value;
        q('[data-available-caption]').textContent = available.options[available.selectedIndex] ? available.options[available.selectedIndex].text : 'No commands are left to add.';
        q('[data-selected-caption]').textContent = selected.options[i] ? selected.options[i].text : 'The toolbar is empty.';
        ['add', 'separator', 'remove', 'up', 'down'].forEach(function (op) {
          q('[data-edit="' + op + '"]').disabled = busy || (op === 'add' ? !has || count >= LIMIT : op === 'separator' ? count >= LIMIT : i < 0 || (op === 'up' ? i === 0 : op === 'down' ? i === count - 1 : false));
        });
      }
      function apply() {
        if (busy || destroyed) return;
        busy = true;
        var error = q('[data-error]'); error.hidden = true;
        draft.visible = q('[data-visible]').checked; draft.inRibbon = q('[data-ribbon]').checked;
        ui.dialog.querySelectorAll('button').forEach(function (b) { b.disabled = true; }); q('fieldset').disabled = true;
        persist(normalize(draft, options.defaults)).then(function (ok) { if (ok && !destroyed) ui.close('apply'); }).catch(function (err) {
          if (!destroyed) { error.textContent = 'Toolbar settings were not saved: ' + (err && err.message || err); error.hidden = false; }
        }).finally(function () {
          busy = false;
          if (!destroyed && ui.dialog.open) { ui.dialog.querySelectorAll('button').forEach(function (b) { b.disabled = false; }); q('fieldset').disabled = false; buttons(); }
        });
      }
      opening = window.yavcDialog.open({ title: 'Customize ' + label + ' Toolbar', className: 'content-toolbar-dialog',
        body: '<fieldset class="content-toolbar-fields"><div class="form-group"><label class="form-label" for="' + prefix + 'search">Search Available Commands</label><input class="form-input" id="' + prefix + 'search" type="search" data-search></div>' +
          '<div class="content-toolbar-columns"><label class="form-label content-toolbar-available-label" for="' + prefix + 'available">Available Commands</label><label class="form-label content-toolbar-selected-label" for="' + prefix + 'selected">Toolbar Commands</label>' +
          '<select class="form-input" id="' + prefix + 'available" size="10" data-available></select>' +
          '<div class="content-toolbar-transfer" role="group" aria-label="Transfer Commands"><button type="button" class="btn" data-edit="add" aria-label="Add to Toolbar" title="Add the selected available command to the toolbar">&gt;</button>' +
          '<button type="button" class="btn" data-edit="remove" aria-label="Remove from Toolbar" title="Remove the selected toolbar command">&lt;</button></div>' +
          '<select class="form-input" id="' + prefix + 'selected" size="10" data-selected></select>' +
          '<p class="form-help content-toolbar-caption" data-available-caption aria-hidden="true"></p><p class="form-help content-toolbar-caption" data-selected-caption aria-hidden="true"></p>' +
          '<div class="hstack content-toolbar-order" role="group" aria-label="Toolbar Order"><button type="button" class="btn btn-sm" data-edit="up"><span class="glyph" style="--glyph:var(--glyph-arrow-up)" aria-hidden="true"></span>Move Up</button>' +
          '<button type="button" class="btn btn-sm" data-edit="down"><span class="glyph" style="--glyph:var(--glyph-arrow-down)" aria-hidden="true"></span>Move Down</button><button type="button" class="btn btn-sm" data-edit="separator">Add Separator</button></div></div>' +
          '<label class="check"><input type="checkbox" data-visible' + (draft.visible ? ' checked' : '') + '> Show the Toolbar</label><label class="check"><input type="checkbox" data-ribbon' + (draft.inRibbon ? ' checked' : '') + '> Keep Visible with the Ribbon</label><button type="button" class="btn btn-sm" data-edit="reset">Restore Defaults</button></fieldset><p class="form-error" data-error role="alert" hidden></p>',
        buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'apply', label: 'Apply', primary: true }],
        check: function (value) { if (value === 'apply') { apply(); return true; } },
        onOpen: function (context) {
          ui = context; dialog = context; lists(0);
          ui.dialog.addEventListener('cancel', function (e) { if (busy) e.preventDefault(); });
          ui.body.addEventListener('input', function (e) { if (e.target.hasAttribute('data-search')) lists(q('[data-selected]').selectedIndex); });
          ui.body.addEventListener('change', buttons);
          ui.body.addEventListener('click', function (e) {
            var b = e.target.closest('[data-edit]'); if (!b || b.disabled || busy) return;
            var op = b.dataset.edit, i = q('[data-selected]').selectedIndex;
            if (op === 'add' || op === 'separator') { draft.items.splice(i + 1, 0, op === 'add' ? q('[data-available]').value : '-'); i++; }
            else if (op === 'remove') draft.items.splice(i, 1);
            else if (op === 'up' || op === 'down') { var next = i + (op === 'up' ? -1 : 1), id = draft.items.splice(i, 1)[0]; draft.items.splice(next, 0, id); i = next; }
            else if (op === 'reset') { draft = normalize(null, options.defaults); q('[data-visible]').checked = draft.visible; q('[data-ribbon]').checked = draft.inRibbon; i = 0; }
            lists(i);
          });
        }
      }).finally(function () {
        opening = null; dialog = null;
        setTimeout(function () { if (!destroyed && document.activeElement === document.body && !document.querySelector('dialog[open]')) root.focus({ preventScroll: true }); }, 0);
      });
      return opening;
    }

    function update(all) { if (!destroyed) { prefs = normalize(all && all[options.app], options.defaults); render(); commandsChanged(); } }
    document.addEventListener('pudl:commands-change', changed);
    render();
    if (config && config.load) {
      Promise.resolve(config.load(FILE, {})).then(function (all) {
        if (destroyed) return;
        update(all);
        if (config.onChange) off = config.onChange(FILE, update);
      }).catch(function (err) { if (!destroyed) toast('Toolbar settings could not be read: ' + (err && err.message || err)); })
        .finally(function () { loading = false; if (!destroyed) commandsChanged(); });
    } else loading = false;

    return {
      commands: function () {
        return [
          { id: SHOW, label: 'Show the Toolbar', glyph: 'panel', checked: function () { return prefs.visible; }, disabled: function () { return !!reason(); }, reason: reason, run: toggle },
          { id: EDIT, label: 'Customize Toolbar…', glyph: 'pencil', disabled: function () { return !!reason(); }, reason: reason, run: customize }
        ];
      },
      destroy: function () {
        destroyed = true;
        cancelAnimationFrame(frame); cancelAnimationFrame(focusFrame);
        if (off) off();
        document.removeEventListener('pudl:commands-change', changed);
        if (dialog) dialog.close('cancel');
        slot.hidden = false;
        bar.remove();
      }
    };
  }

  window.dinahContentToolbar = { attach: attach, normalize: normalize };
})();
