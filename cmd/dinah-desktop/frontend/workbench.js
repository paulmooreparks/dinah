/* Dinah owns item identities and applet commands. PUDL owns windows,
   their placement, focus, menus and URL state. */
(function () {
  'use strict';
  // The product's name as people see it (product.js).
  function productName() { return window.dinahProduct ? window.dinahProduct.name : ''; }
  var kinds = new Map();
  var instances = new Map();
  var launchers = new Map();
  var svgSerial = 0;
  var embedSerial = 0;
  var layoutKey = 'dinah-workbench-layout';
  function saveLayout(event) {
    if (!window.dinahUIState || dinahUIState.isRestoring()) return;
    dinahUIState.set(layoutKey, location.search);
    if (event && event.detail && event.detail.place) {
      var placements = dinahUIState.get('dinah-window-placements', {});
      Object.assign(placements, event.detail.place);
      dinahUIState.set('dinah-window-placements', placements);
    }
  }
  document.addEventListener('pudl:windows-change', saveLayout);
  document.addEventListener('dinah:ui-state-restored', function (event) {
    if (!window.pudlWindows || event.detail.explicit || event.detail.restored) return;
    window.pudlWindows.state().open.forEach(function (key) {
      var item = decode(key), cfg = kinds.get(item ? item.kind : key);
      if (cfg && !cfg.mode && !cfg.dockOnly) window.pudlWindows.snap(key, dinahUIState.windowMode(item ? item.kind : key));
    });
  });
  document.addEventListener('pudl:window-place', function (event) {
    if (!window.dinahUIState) return;
    var remembered = dinahUIState.get('dinah-window-placements', {})[event.detail.key];
    if (remembered) { event.detail.placement = remembered; return; }
    var item = decode(event.detail.key), cfg = kinds.get(item ? item.kind : event.detail.key);
    if (!cfg || cfg.mode || cfg.dockOnly) return;
    var mode = dinahUIState.windowMode(item ? item.kind : event.detail.key);
    var template = document.getElementById('win-source-' + event.detail.key);
    if (template) template.content.querySelector('[data-win]').dataset.winMode = mode;
    var win = document.querySelector('[data-win="' + CSS.escape(event.detail.key) + '"]');
    if (win) win.dataset.winMode = mode;
  }, true);

  /* The scrolling elements of a window are watched so their positions are
     kept. Each element is looked at once, when it enters the window, and
     the look (a computed style, which lays the page out) is taken once a
     frame for everything that entered since; looking at every element of
     the window again on every change laid the page out for every change. */
  function watchScroll(root, key) {
    if (!window.dinahUIState) return function () {};
    var seen = new WeakSet(), queued = [], frame = 0;
    function consider(el) {
      if (el.closest('[data-ui-scroll-ignore]')) return;
      if (seen.has(el)) return;
      seen.add(el);
      if (['INPUT', 'SELECT'].includes(el.tagName)) return;
      var id = el.dataset.uiScroll || el.dataset.viewId || el.id || (el.classList.contains('win-body') ? 'window-body' : null);
      if (!id) return;
      var style = getComputedStyle(el);
      if (!/(auto|scroll)/.test(style.overflowX + ' ' + style.overflowY)) return;
      dinahUIState.watchScroll(el, 'dinah-scroll:' + (root.dataset.win || key) + ':' + id, { followEnd: el.hasAttribute('data-follow-end'), defer: el.hasAttribute('data-scroll-defer') });
    }
    function scan(scope) {
      if (scope.nodeType !== 1 || scope.closest('[data-ui-scroll-ignore]')) return;
      if (scope.matches('[id], [data-ui-scroll], .win-body')) consider(scope);
      scope.querySelectorAll('[id], [data-ui-scroll], .win-body').forEach(consider);
    }
    function scanQueued() {
      frame = 0;
      var batch = queued;
      queued = [];
      batch.forEach(function (el) { if (el.isConnected) scan(el); });
    }
    scan(root);
    var observer = new MutationObserver(function (records) {
      records.forEach(function (r) { Array.prototype.forEach.call(r.addedNodes, function (n) { if (n.nodeType === 1) queued.push(n); }); });
      if (queued.length && !frame) frame = requestAnimationFrame(scanQueued);
    });
    observer.observe(root, { childList: true, subtree: true });
    return function () { observer.disconnect(); cancelAnimationFrame(frame); dinahUIState.detachScroll(root); };
  }

  function encode(item) {
    var bytes = new TextEncoder().encode(JSON.stringify([item.kind, item.workspace || '', item.path || '']));
    return 'item-' + btoa(Array.from(bytes, function (b) { return String.fromCharCode(b); }).join(''))
      .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  }
  function decode(key) {
    if (!key.startsWith('item-')) return null;
    try {
      var bytes = Uint8Array.from(atob(key.slice(5).replace(/-/g, '+').replace(/_/g, '/')), function (c) { return c.charCodeAt(0); });
      var value = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes));
      if (!Array.isArray(value) || value.length !== 3 || value.some(function (s) { return typeof s !== 'string'; })) return null;
      return { kind: value[0], workspace: value[1], path: value[2] };
    } catch (e) { return null; }
  }
  function source(item) {
    var cfg = kinds.get(item.kind);
    if (!cfg) throw new Error('Unknown applet: ' + item.kind);
    var key = cfg.singleton ? item.kind : encode(item);
    if (document.getElementById('win-source-' + key)) return key;
    var template = document.createElement('template');
    template.id = 'win-source-' + key;
    var win = document.createElement('section');
    win.className = 'win dinah-window';
    win.dataset.win = key;
    win.dataset.winKind = item.kind;
    win.dataset.winMode = cfg.mode || (window.dinahUIState ? dinahUIState.windowMode(item.kind) : 'floating');
    win.dataset.winGlyph = cfg.glyph || 'app';
    if (cfg.dockOnly) win.setAttribute('data-win-dock-only', '');
    // A desktop file has no public HTTP address to share.
    win.setAttribute('data-win-href', '');
    // A kind that shows status in its title bar (cfg.statuses) has PUDL's
    // status group from the start, empty: PUDL adopts the group with the
    // window, and the kind fills it later (PUDL 0.88.0, WINDOW-STATUS.md).
    win.innerHTML = '<header class="win-head"><h2 class="win-title"></h2>' +
      (cfg.statuses ? '<div class="win-statuses" role="group" aria-label="Window status"></div>' : '') +
      '<nav class="win-chrome" aria-label="Window">' +
      '<button type="button" class="win-btn" data-win-action="minimize" aria-label="Minimize"></button>' +
      '<button type="button" class="win-btn" data-win-action="maximize" aria-label="Maximize or Restore"></button>' +
      '<button type="button" class="win-btn" data-win-action="close" aria-label="Close"></button></nav></header>' +
      '<div class="win-body dinah-applet" data-applet-fit="fill"></div>';
    var kept = cfg.singleton ? null : savedTitles()[key];
    win.querySelector('.win-title').textContent = (cfg.titleOf && cfg.titleOf(item)) || (kept && kept.title) || item.title || cfg.title;
    if (kept && kept.detail) win.setAttribute('data-win-detail', kept.detail);
    var root = win.querySelector('.dinah-applet');
    root.dataset.applet = 'dinah-' + item.kind;
    root.dataset.item = JSON.stringify(item);
    if (cfg.loadingText) {
      var pending = document.createElement('p'); pending.className = 'form-help';
      pending.dataset.appletLoading = ''; pending.setAttribute('role', 'status'); pending.textContent = cfg.loadingText;
      root.appendChild(pending);
    }
    groupWindow(win, item);
    template.content.appendChild(win);
    document.body.appendChild(template);
    return key;
  }
  /* Registrations declare the applet identity independently of window kind.
     workspace:false means workspace-free; workspace:true requires a resolved
     owner. Missing metadata or ownership stays ungrouped until regroup runs.
     PUDL owns projection, overflow and represented-window command scope. */
  var grouping = null;
  var GROUP_ATTRS = { key: 'data-win-group', label: 'data-win-group-label', glyph: 'data-win-group-glyph' };
  function groupWindow(win, item) {
    var cfg = item && kinds.get(item.kind), group = null;
    var applet = cfg && (typeof cfg.applet === 'function' ? cfg.applet(item, win.dataset.win) : cfg.applet);
    if (applet && applet.id && applet.title) {
      var owner = applet.workspace === true && grouping ? grouping(cfg.workspaceOf ? cfg.workspaceOf(item, win.dataset.win) : item.workspace || '') : null;
      if (applet.workspace === false || owner) {
        // Encoding the tuple keeps distinct IDs distinct, including punctuation.
        var identity = owner ? [applet.id, owner.key] : [applet.id];
        var bytes = new TextEncoder().encode(JSON.stringify(identity));
        var key = 'applet-' + btoa(Array.from(bytes, function (byte) { return String.fromCharCode(byte); }).join('')).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
        group = { key: key, label: applet.title + (owner ? ' (' + owner.label + ')' : ''), glyph: applet.glyph || cfg.glyph || 'app' };
      }
    }
    Object.keys(GROUP_ATTRS).forEach(function (part) {
      var name = GROUP_ATTRS[part], value = group ? group[part] : '';
      if (value) { if (win.getAttribute(name) !== value) win.setAttribute(name, value); }
      else if (win.hasAttribute(name)) win.removeAttribute(name);
    });
  }
  function windowItem(win) {
    var root = win.querySelector('.dinah-applet');
    try { return root && root.dataset.item ? JSON.parse(root.dataset.item) : null; } catch (err) { return null; }
  }
  function regroup() {
    document.querySelectorAll('template[id^="win-source-"]').forEach(function (template) {
      var win = template.content.querySelector('.win[data-win]');
      if (win) groupWindow(win, windowItem(win));
    });
    document.querySelectorAll('.win[data-win]').forEach(function (win) { groupWindow(win, windowItem(win)); });
  }
  document.addEventListener('dinah:ui-state-restored', regroup);
  /* PUDL copies a window from its markup when it is asked to open it and
     adds the copy to the page later, so a copy taken before the workspaces
     were read, as the windows of the saved layout are at startup, missed
     the regroup that followed and was in no group until a workspace was
     next chosen. Each window is grouped again as it opens. */
  document.addEventListener('pudl:window-open', function (e) {
    var win = e.target;
    if (win && win.matches && win.matches('.win[data-win]')) groupWindow(win, windowItem(win));
  });
  /* A window's title is its applet's to set, and PUDL keeps none (0.66.0),
     while a window restored behind others starts its applet only when it is
     seen. So that such a window shows the title it last had, not its kind's,
     Dinah keeps each open window's last title and detail in its UI
     state and gives them to the window's markup. A window of which there is
     only one, such as the Group Chats pane, always has its kind's title, so
     a kind that is renamed is renamed in a saved layout too; the pane kept
     the title Bus from before it was renamed (28 October 2026). */
  var TITLES_KEY = 'dinah-window-titles', titlesTimer = 0;
  function savedTitles() {
    try { return (window.dinahUIState && window.dinahUIState.get(TITLES_KEY)) || {}; } catch (err) { return {}; }
  }
  function saveTitles() {
    clearTimeout(titlesTimer);
    titlesTimer = setTimeout(function () {
      if (!window.dinahUIState || !window.pudlWindows) return;
      var out = {};
      window.pudlWindows.state().open.forEach(function (key) {
        var win = document.querySelector('.win[data-win="' + CSS.escape(key) + '"]'), t = win && win.querySelector('.win-title');
        if (t && t.textContent.trim()) out[key] = { title: t.textContent.trim(), detail: win.getAttribute('data-win-detail') || '' };
      });
      try { window.dinahUIState.set(TITLES_KEY, out); } catch (err) { /* A view preference is optional. */ }
    }, 500);
  }
  document.addEventListener('pudl:window-open', function (e) {
    var win = e.target, t = win.querySelector && win.querySelector('.win-title');
    if (!t) return;
    new MutationObserver(saveTitles).observe(t, { childList: true, characterData: true, subtree: true });
    new MutationObserver(saveTitles).observe(win, { attributes: true, attributeFilter: ['data-win-detail'] });
    saveTitles();
  });
  /* The UI state is read after the windows from the address have their
     markup; their kept titles are given then, to the markup and to a window
     whose applet has not started. */
  function applySavedTitles() {
    var saved = savedTitles();
    Object.keys(saved).forEach(function (key) {
      var single = kinds.get(key);
      if (single && single.singleton) return;
      var kept = saved[key], tpl = document.getElementById('win-source-' + key);
      var t = tpl && tpl.content.querySelector('.win-title');
      if (t) {
        t.textContent = kept.title;
        if (kept.detail) tpl.content.querySelector('.win').setAttribute('data-win-detail', kept.detail);
      }
      if (window.pudlWindows && window.pudlWindows.state().open.indexOf(key) >= 0 && window.pudlApplets && window.pudlApplets.started && !window.pudlApplets.started(key)) {
        window.pudlWindows.retitle(key, kept.title, kept.detail || '');
      }
    });
  }
  // Item metadata can arrive before an idle applet starts. Its registration
  // may supply a known title without mounting the applet or opening a provider.
  function refreshTitles() {
    function refresh(win) {
      var item = windowItem(win), cfg = item && kinds.get(item.kind);
      var name = cfg && cfg.titleOf && cfg.titleOf(item);
      if (!name) return;
      var title = win.querySelector('.win-title');
      if (!title || title.textContent === name) return;
      if (win.isConnected && window.pudlWindows) window.pudlWindows.retitle(win.dataset.win, name);
      else title.textContent = name;
    }
    document.querySelectorAll('template[id^="win-source-"]').forEach(function (tpl) { var win = tpl.content.querySelector('.win'); if (win) refresh(win); });
    document.querySelectorAll('.win[data-win]').forEach(refresh);
  }
  function hydrate() {
    (new URLSearchParams(location.search).get('open') || '').split(',').forEach(function (key) {
      var item = decode(key);
      if (!item && kinds.has(key)) item = { kind: key };
      if (item && kinds.has(item.kind)) source(item);
    });
  }
  // The rows of a window's own menu, as its title bar's menu shows them.
  function windowMenuRows(key) {
    return window.pudlWindows && window.pudlWindows.menuCommands ? window.pudlWindows.menuCommands(key) : [];
  }
  function setWindowState(key, state) {
    var url = new URL(location.href);
    if (state == null) url.searchParams.delete('view.' + key);
    else url.searchParams.set('view.' + key, String(state));
    history.replaceState(history.state, '', url);
    var instance = instances.get(key);
    if (instance && instance.setState) instance.setState(state);
    saveLayout();
  }
  window.dinahWorkbench = {
    windowMenuRows: windowMenuRows,
    applySavedTitles: applySavedTitles, refreshTitles: refreshTitles,
    registerLauncher: function (id, definition) {
      if (!id || !definition.title || typeof definition.run !== 'function') throw new Error('A tool needs an identity, title and launcher.');
      if (launchers.has(id)) throw new Error('Duplicate tool launcher: ' + id);
      launchers.set(id, Object.freeze(Object.assign({}, definition, { id: id })));
      document.dispatchEvent(new CustomEvent('dinah:launchers-change'));
    },
    unregisterLauncher: function (id) { if (launchers.delete(id)) document.dispatchEvent(new CustomEvent("dinah:launchers-change")); },
    launchers: function () { return Array.from(launchers.values()); },
    embed: function (item, root, opts) {
      var cfg = kinds.get(item.kind);
      if (!cfg) throw new Error('Unknown applet: ' + item.kind);
      var key = 'embedded-' + (++embedSerial) + '-' + encode(item);
      root.classList.add('dinah-applet', 'dinah-embedded-applet');
      root.dataset.embeddedKind = item.kind;
      var instance = cfg.init(root, Object.assign({}, opts, {embedded:true, key:key}), item);
      var stopScroll = watchScroll(root, key), disposed = false;
      return Object.assign({}, instance, {destroy:function () {
        if (disposed) return;
        disposed = true;
        stopScroll();
        if (instance.destroy) instance.destroy();
      }});
    },
    register: function (kind, cfg) {
      kinds.set(kind, cfg);
      if (cfg.launcher) this.registerLauncher(kind, Object.assign({ title: cfg.title, glyph: cfg.glyph, windowTitle: cfg.title }, cfg.launcher));
      // A kind may start when its window is seen, or in idle time after the
      // windows in sight (PUDL 0.66.0's start).
      window.pudlApplets.register('dinah-' + kind, { start: cfg.start || 'open', init: function (root, opts) {
        var key = root.closest('[data-win]').dataset.win;
        var pending = root.querySelector(':scope > [data-applet-loading]'); if (pending) pending.remove();
        var instance = cfg.init(root, opts, JSON.parse(root.dataset.item));
        var stopScroll = watchScroll(root.closest('[data-win]'), key);
        instances.set(key, instance);
        /* Every window's menu opens with the rows of its window menu
           (Collapse, Dock, Reset, Close and the rest), as its title bar's
           menu has them, then its own commands under Actions. A window
           with menus of its own (the Editor's) puts the rows there itself. */
        var menus = instance.menus || function () {
          var win = root.closest('[data-win]'), title = win && win.querySelector('.win-title');
          // The window's menu takes its kind's glyph, and Actions the list's,
          // so each folds to its glyph before the menu bar collapses.
          var titles = [{ label: cfg.singleton ? cfg.title : (title && title.textContent.trim()) || cfg.title, glyph: cfg.glyph || 'app', items: windowMenuRows(key) }];
          var own = instance.commands ? instance.commands() : [];
          if (own.length) titles.push({ id: 'actions', label: 'Actions', mnemonic: 'A', glyph: 'list', items: own });
          return { titles: titles };
        };
        return Object.assign({}, instance, { menus: menus, destroy: function () {
          stopScroll();
          instances.forEach(function (value, id) { if (value === instance) instances.delete(id); });
          if (instance.destroy) instance.destroy();
        } });
      } });
      if (cfg.singleton) source({ kind: kind });
      hydrate();
    },
    open: function (item, state) {
      if (typeof item === 'string') item = { kind: item };
      var key = source(item);
      if (state !== undefined) setWindowState(key, state);
      if (window.pudlWindows.state().open.includes(key)) {
        window.pudlWindows.raise(key);
        return Promise.resolve(key);
      }
      return new Promise(function (resolve, reject) {
        function opened(e) {
          if (e.target.dataset.win !== key) return;
          document.removeEventListener('pudl:window-open', opened);
          resolve(key);
        }
        document.addEventListener('pudl:window-open', opened);
        try { window.pudlWindows.open(key); }
        catch (err) { document.removeEventListener('pudl:window-open', opened); reject(err); }
      });
    },
    key: encode,
    // Apply a navigation target without raising a background restored window.
    setState: setWindowState,
    /* A window for a resource: the resource viewer. */
    resourceItem: function (ref, title) {
      return { kind: 'workspace-resource', workspace: ref.workspaceId, path: JSON.stringify([ref.sourceId, ref.path || '']), title: title || 'Resource' };
    },
    resourceRef: function (item) {
      if (item.kind !== 'workspace-resource') return null;
      var parts = JSON.parse(item.path);
      if (!Array.isArray(parts) || parts.length !== 2 || parts.some(function (part) { return typeof part !== 'string'; })) throw new Error('Invalid resource address');
      return { workspaceId: item.workspace, sourceId: parts[0], path: parts[1] };
    },
    namespaceSVG: function (root) {
      root.querySelectorAll('svg:not([data-dinah-svg])').forEach(function (svg) {
        svg.dataset.dinahSvg = String(++svgSerial);
        var ids = new Map(), prefix = 'dinah-svg-' + svgSerial + '-';
        [svg].concat(Array.from(svg.querySelectorAll('[id]'))).forEach(function (el) {
          if (el.id) { ids.set(el.id, prefix + el.id); el.id = prefix + el.id; }
        });
        function references(value) {
          ids.forEach(function (next, old) {
            value = value.replace(new RegExp('#' + old.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '(?![\\w-])', 'g'), '#' + next);
          });
          return value;
        }
        [svg].concat(Array.from(svg.querySelectorAll('*'))).forEach(function (el) {
          Array.from(el.attributes).forEach(function (attr) {
            if (attr.name === 'id') return;
            var value = attr.value;
            if (attr.name === 'aria-labelledby' || attr.name === 'aria-describedby') value = value.split(/\s+/).map(function (id) { return ids.get(id) || id; }).join(' ');
            else value = references(value);
            if (value !== attr.value) el.setAttribute(attr.name, value);
          });
          if (el.localName === 'style') el.textContent = references(el.textContent);
        });
      });
    },
    rekey: function (oldKey, item) {
      var key = source(item);
      if (!window.pudlWindows.rekey(oldKey, key, true)) throw new Error('The chat window could not be renamed.');
      var instance = instances.get(oldKey);
      instances.delete(oldKey);
      if (instance) instances.set(key, instance);
      if (window.dinahUIState) {
        dinahUIState.migratePrefix('dinah-scroll:' + oldKey + ':', 'dinah-scroll:' + key + ':');
        dinahUIState.migratePrefix('dinah-chart:' + oldKey + ':', 'dinah-chart:' + key + ':');
      }
      var win = document.querySelector('[data-win="' + CSS.escape(key) + '"]');
      win.dataset.winKind = item.kind;
      win.querySelector('.dinah-applet').dataset.item = JSON.stringify(item);
      // A chat continued into another workspace moves to that one's stack.
      groupWindow(win, item);
      return key;
    },
    /* fn(workspace) answers {key, label} with a stable workspace ID for its
       record's ID, its folder or { mount } (a path under /workspaces), or
       null for none. */
    setGrouping: function (fn) { grouping = fn; regroup(); },
    regroup: regroup,
    item: decode,
    mount: function (root, template, key) {
      root.appendChild(document.getElementById(template).content.cloneNode(true));
      var ids = new Map();
      root.querySelectorAll('[id]').forEach(function (el) {
        var id = el.id;
        el.dataset.viewId = id;
        el.id = key + '-' + id;
        ids.set(id, el.id);
      });
      ['for', 'aria-controls', 'aria-labelledby', 'aria-describedby', 'popovertarget', 'data-menu-anchor'].forEach(function (attr) {
        root.querySelectorAll('[' + attr + ']').forEach(function (el) {
          el.setAttribute(attr, el.getAttribute(attr).split(/\s+/).map(function (id) { return ids.get(id) || id; }).join(' '));
        });
      });
      function selector(s) { return s.replace(/#([a-zA-Z][\w-]*)/g, function (_, id) { return '#' + CSS.escape(ids.get(id) || key + '-' + id); }); }
      return {
        id: function (id) { return root.querySelector(selector('#' + id)); },
        query: function (s) { return root.querySelector(selector(s)); },
        all: function (s) { return root.querySelectorAll(selector(s)); }
      };
    }
  };
  document.addEventListener('DOMContentLoaded', hydrate);
  /* An agent's questions about the open windows and their
     commands (applet_window_commands.go). Only the windows
     of the calling workspace are seen; a window gives its workspace by its
     folder or by its ID. A command is looked up in that window alone, a
     command that would destroy something is left to the person, and one
     that cannot run now is refused with its reason, as PUDL refuses it. */
  function sameFolder(a, b) {
    function norm(s) { return String(s || '').replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase(); }
    return !!a && norm(a) === norm(b);
  }
  function windowsOf(args) {
    if (!window.pudlWindows) return [];
    return window.pudlWindows.state().open.map(function (key) {
      var win = document.querySelector('.win[data-win="' + CSS.escape(key) + '"]'), root = win && win.querySelector('.dinah-applet');
      var item = null;
      try { item = root && JSON.parse(root.dataset.item); } catch (err) { item = null; }
      if (!item) return null;
      // A tool's window that belongs to no workspace is every workspace's. The application's own panes, which
      // also have none, and another workspace's windows are not.
      if (!item.workspace) return launchers.has(item.kind) ? { key: key, win: win, root: root, item: item } : null;
      if (!sameFolder(item.workspace, args.workspace) && !(args.workspaceId && item.workspace === args.workspaceId)) return null;
      return { key: key, win: win, root: root, item: item };
    }).filter(Boolean);
  }
  function askedWindow(args) {
    var found = windowsOf(args).filter(function (w) { return w.key === args.window; })[0];
    if (!found) throw new Error('There is no open window ' + JSON.stringify(args.window) + ' in this workspace; list the windows for their keys.');
    if (window.pudlApplets && window.pudlApplets.started && !window.pudlApplets.started(found.key)) {
      throw new Error('The window ' + JSON.stringify(found.win.querySelector('.win-title').textContent.trim()) + ' has not been shown since ' + productName() + ' started, so its commands are not ready; ask the person to bring it to the front.');
    }
    return found;
  }
  var pageAsks = {
    windows: function (args) {
      var s = window.pudlWindows.state();
      return { windows: windowsOf(args).map(function (w) {
        var out = { window: w.key, kind: w.item.kind, title: w.win.querySelector('.win-title').textContent.trim(), front: s.top === w.key && !s.min[w.key] };
        var detail = w.win.getAttribute('data-win-detail');
        if (detail) out.detail = detail;
        return out;
      }) };
    },
    commands: function (args) {
      return { commands: window.pudlCommands.list(askedWindow(args).root) };
    },
    run: function (args) {
      var w = askedWindow(args), cmd = window.pudlCommands.find(args.command, w.root);
      if (!cmd) throw new Error('The window has no command ' + JSON.stringify(args.command) + '; list its commands for their ids.');
      var label = window.pudlCommands.value(cmd, 'label') || cmd.id;
      if (window.pudlCommands.value(cmd, 'danger')) throw new Error(label + ' would destroy something, so it is left to the person to press.');
      return window.pudlCommands.run(cmd.id, args.value, w.root, { owner: { kind: 'agent' } }).then(function () { return { ran: label }; });
    },
    /* A tool's window, opened as Tools opens it; a tool whose window is
       open is brought to the front. */
    open: function (args) {
      var tool = launchers.get(args.tool);
      if (!tool) throw new Error(productName() + ' has no tool ' + JSON.stringify(args.tool) + '; the tools are ' + Array.from(launchers.keys()).join(', ') + '.');
      return Promise.resolve(tool.run()).then(function () {
        var s = window.pudlWindows.state(), keys = s.open.filter(function (k) { var item = decode(k); return (item ? item.kind : k) === args.tool; });
        var key = keys.indexOf(s.top) >= 0 ? s.top : keys[keys.length - 1];
        if (!key) throw new Error('The ' + tool.title + ' window did not open.');
        return { window: key };
      });
    }
  };
  /* The ids of the questions the engine put to the page, kept after they are
     answered, so one that a reread returns after the page answered it is not
     run again. */
  var askedIds = new Set();
  function pageAsked(ask) {
    var answer = window.dinahHelpers.app() && window.dinahHelpers.app().AnswerPage;
    if (!answer || !ask || askedIds.has(ask.id)) return;
    askedIds.add(ask.id);
    if (askedIds.size > 1000) askedIds.delete(askedIds.values().next().value);
    Promise.resolve().then(function () {
      var op = pageAsks[ask.op];
      if (!op) throw new Error(productName() + ' does not know the question ' + ask.op);
      return op(ask.args || {});
    }).then(function (result) { return answer(ask.id, JSON.stringify(result), ''); },
      function (err) { return answer(ask.id, '', (err && err.message) || String(err)); });
  }
  if (window.runtime) {
    window.dinahHost.on('page:ask', pageAsked);
    // After a resync the questions the engine is still waiting on are read again.
    window.dinahHost.onResync(function () {
      var app = window.dinahHelpers.app();
      if (!app || !app.PendingPageAsks) return null;
      return Promise.resolve(app.PendingPageAsks()).then(function (list) { (list || []).forEach(pageAsked); });
    });
  }
  document.addEventListener('pudl:applet-state', function (e) {
    var win = e.target.closest('[data-win]');
    if (win) e.detail.state = new URLSearchParams(location.search).get('view.' + win.dataset.win);
  });
  document.addEventListener('pudl:applet-change', function (e) {
    var win = e.target.closest('[data-win]');
    if (!win) return;
    var url = new URL(location.href), name = 'view.' + win.dataset.win;
    if (e.detail.state == null) url.searchParams.delete(name);
    else url.searchParams.set(name, e.detail.state);
    if (url.href !== location.href) {
      try { history.replaceState(history.state, '', url); }
      catch (err) { /* Local file previews can disallow History API writes. */ }
    }
    saveLayout();
  });
  window.addEventListener('popstate', function () {
    hydrate();
    var query = new URLSearchParams(location.search);
    instances.forEach(function (instance, key) {
      if (instance.setState) instance.setState(query.get('view.' + key));
    });
  });
})();
