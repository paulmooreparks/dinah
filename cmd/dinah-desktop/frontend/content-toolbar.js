/* The toolbar Dinah gives a yavc.app content applet (pudlHost.commandToolbar,
   yavc.app command-toolbar-design.md). The applet hands over its default
   command ids, separated into groups by '-', and its command table; the
   toolbar is a PUDL folding toolbar of buttons that each say which command
   they are, so PUDL draws their words, glyphs, keys and state from the
   table and folds the groups as the window narrows. A command a document
   does not have now is left out until it has it again. Customising the
   toolbar is not offered yet, so the applet's defaults are the toolbar. */
(function () {
  'use strict';

  function field(command, name) { var v = command[name]; return typeof v === 'function' ? v.call(command) : v; }

  /* The commands the table has now, by id, with the caption a group takes
     from its first command and the choice group a command stands for. */
  function catalog(table) {
    var out = new Map(), walked = new Set();
    (function walk(commands, depth) {
      if (!Array.isArray(commands) || depth > 8) return;
      commands.forEach(function (c) {
        if (!c || typeof c !== 'object' || walked.has(c)) return;
        walked.add(c);
        try {
          if (typeof c.id === 'string' && !out.has(c.id)) {
            var choice = field(c, 'toolbarChoice');
            out.set(c.id, { caption: String(field(c, 'label') || c.id), choice: typeof choice === 'string' && /^[a-z][a-z0-9-]{0,63}$/.test(choice) ? choice : null });
          }
          walk(field(c, 'items'), depth + 1);
        } catch (e) { /* A command not ready yet is read again at the next change. */ }
      });
    })(table(), 0);
    return out;
  }

  function attach(root, options) {
    var slot = root.querySelector('[data-toolbar-slot]');
    if (!slot) throw new Error('A command toolbar needs a toolbar slot.');
    var bar = document.createElement('div');
    bar.className = 'toolbar content-toolbar';
    bar.setAttribute('role', 'toolbar');
    bar.setAttribute('aria-label', (options.label || 'Document') + ' Commands');
    bar.setAttribute('data-toolbar-fold', '');
    slot.appendChild(bar);
    var destroyed = false, drawn = '';
    function render() {
      if (destroyed) return;
      var known = catalog(options.table);
      var ids = (options.defaults || []).filter(function (id) { return id === '-' || known.has(id); });
      var signature = JSON.stringify(ids);
      if (signature === drawn) return;
      drawn = signature;
      bar.replaceChildren();
      var group = null;
      ids.forEach(function (id) {
        if (id === '-') { group = null; return; }
        if (!group) {
          group = document.createElement('div');
          group.className = 'toolbar-group';
          group.setAttribute('role', 'group');
          group.setAttribute('aria-label', known.get(id).caption);
          group.setAttribute('data-toolbar-glyph', 'more');
          bar.appendChild(group);
        }
        var c = known.get(id);
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
      if (window.pudlCommands) window.pudlCommands.changed();
      if (window.pudlToolbar) window.pudlToolbar.enhance(slot);
    }
    document.addEventListener('pudl:commands-change', render);
    render();
    return {
      commands: function () { return []; },
      destroy: function () {
        destroyed = true;
        document.removeEventListener('pudl:commands-change', render);
        bar.remove();
      }
    };
  }

  window.dinahContentToolbar = { attach: attach };
})();
