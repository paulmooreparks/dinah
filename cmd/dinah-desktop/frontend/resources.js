/* Shared human-facing resource adapter for Files and Terminal. */
(function () {
  'use strict';
  const app = window.dinahHelpers.app;
  function has(item, action) { return (item.actions || []).includes(action); }
  function sameRef(a, b) { return !!a && !!b && a.workspaceId === b.workspaceId && a.sourceId === b.sourceId && (a.path || '') === (b.path || ''); }
  function sameOwner(a, b) { return !!a && !!b && a.workspaceId === b.workspaceId && a.sourceId === b.sourceId; }
  // Expected methods validate the mounted source. Its root may resolve to a canonical page ID.
  function compatibleRef(a, b) { return sameOwner(a, b) && (sameRef(a, b) || !(a.path || '') || !(b.path || '')); }
  /* The folders every tree has opened are watched on disk (workspace_watch.go),
     so a change made outside Dinah, by an agent, another program
     or a sync client, arrives as workspace:changed of kind
     disk-changed, and each tree lists its opened folders again. Each tree
     gives a function that lists its opened folders; the set is sent after
     the trees settle, and only when it changed. */
  var trees = new Set(), watchTimer = 0, watchedKey = null;
  function watchFolders() {
    clearTimeout(watchTimer);
    watchTimer = setTimeout(function () {
      var refs = [], seen = {};
      trees.forEach(function (opened) {
        opened().forEach(function (ref) {
          var k = JSON.stringify([ref.workspaceId || '', ref.sourceId || '', ref.path || '']);
          if (!seen[k]) { seen[k] = true; refs.push(ref); }
        });
      });
      var key = Object.keys(seen).sort().join('|');
      if (key === watchedKey || !app().WatchFolders) return;
      watchedKey = key;
      Promise.resolve(app().WatchFolders(refs)).catch(function () { watchedKey = null; });
    }, 200);
  }
  /* A source kept somewhere else, on a site for one, may keep what it has
     read so that it lists quickly (docs/design/source-kinds.md). Its kind
     says when it finds a change there, which arrives as workspace:changed
     of kind source-changed and is listed again like a change on disk. A
     Refresh command calls forget first, so that what it lists next is asked
     of the place the source is kept. Forgetting never stops the listing. */
  function forget(ref) {
    if (!ref || !ref.sourceId || !app() || !app().RefreshWorkspaceResource) return Promise.resolve();
    return Promise.resolve().then(function () { return app().RefreshWorkspaceResource(ref); }).catch(function () {});
  }
  var subscribers = new Set();
  function notify(event) { subscribers.forEach(function (fn) { fn(event); }); }
  function subscribe(fn) { subscribers.add(fn); return function () { subscribers.delete(fn); }; }
  document.addEventListener('dinah:workspace-resource-labels', function () { notify({kind:'labels'}); });
  window.dinahHost.on('workspace:changed', notify);
  // After a resync every tree lists its folders again and the catalog and terminals check theirs.
  window.dinahHost.onResync(function () { notify({ kind: 'resync' }); });
  function verify(item) {
    return (item.ref && app().ResolveWorkspaceResourceMount ? app().ResolveWorkspaceResourceMount(item.ref) : item.ref ? app().StatWorkspaceMountExpected(item.path, item.ref) : app().StatWorkspaceMount(item.path)).then(function (current) {
      if (item.ref && !compatibleRef(item.ref, current.ref)) throw new Error('This location now refers to a different workspace source. Open it again from the workspace catalog.');
      return current;
    });
  }
  function openFiles(path, ref) { return dinahWorkbench.open({ kind: 'files', workspace: ref ? ref.workspaceId : '', path: ref ? JSON.stringify({ path: path, ref: ref }) : path || '/workspaces' }); }
  /* A Terminal runs the shell given, or the one chosen in Settings, which is
     Dinah's own when the computer's shells cannot be listed. */
  function openTerminal(path, ref, shell) {
    var id = Array.from(crypto.getRandomValues(new Uint8Array(16)), function (value) { return value.toString(16).padStart(2, '0'); }).join('');
    var a = window.dinahHelpers.app();
    var chosen = shell ? Promise.resolve(shell) : a && a.TerminalShells ? Promise.resolve(a.TerminalShells()).then(function (s) { return s.default; }).catch(function () { return 'dinah'; }) : Promise.resolve('dinah');
    return chosen.then(function (name) { return dinahWorkbench.open({ kind: 'terminal', path: JSON.stringify([path || '/workspaces', id, ref || null, name || 'dinah']) }); });
  }
  /* A product says which of its windows opens an entry
     (docs/design/product-applets.md, "The Window That Opens an Entry"). Its
     page file registers an opener for a kind of entry, or for the end of a
     file's name:

       dinahResources.registerOpener({ itemKind: 'sample-record', title: 'Records', glyph: 'ticket',
         open: function (item, opts) { return dinahWorkbench.open({ kind: 'sample-records', path: item.ref.path }); } });
       dinahResources.registerOpener({ suffix: '.sample.md', open: function (item, opts) { ... } });

     Every way of opening an entry asks openerFor before its own rules, at
     the time of the open, so an opener registered after the layout was
     restored serves every later open. An opener for the entry's kind is
     chosen before one for its name, and of two suffixes that both end the
     name the longer is chosen. A key that is already registered is refused
     with a message on the console, and the first registration stays. With
     no opener registered, every open goes as it did before. */
  var kindOpeners = new Map(), suffixOpeners = new Map();
  function registerOpener(o) {
    o = o || {};
    var kind = typeof o.itemKind === 'string' ? o.itemKind.trim() : '', suffix = typeof o.suffix === 'string' ? o.suffix.trim().toLowerCase() : '';
    if (typeof o.open !== 'function' || !kind === !suffix) {
      console.error('dinahResources.registerOpener needs an open function and either an itemKind or a suffix. Nothing was registered.');
      return false;
    }
    if (suffix && (suffix[0] !== '.' || suffix.length < 2 || suffix.indexOf('/') >= 0 || suffix.indexOf('\\') >= 0)) {
      console.error('dinahResources.registerOpener was given the suffix "' + suffix + '". A suffix is the end of a file name and starts with a dot, as ".sample.md" does. Nothing was registered.');
      return false;
    }
    var table = kind ? kindOpeners : suffixOpeners, key = kind || suffix;
    if (table.has(key)) {
      console.error('dinahResources.registerOpener: an opener is already registered for the ' + (kind ? 'item kind "' : 'suffix "') + key + '". The first registration stays and this one is refused.');
      return false;
    }
    table.set(key, { itemKind: kind, suffix: suffix, open: o.open, title: typeof o.title === 'string' ? o.title : '', glyph: typeof o.glyph === 'string' ? o.glyph : '' });
    return true;
  }
  /* The opener of an entry, or null. The entry is a resource item, or as
     much of one as the caller has: its kind where that is known, and its
     name, which is read from ref.path or else from path. A folder is
     matched by its kind only. */
  function openerFor(item) {
    if (!item) return null;
    if (item.kind && kindOpeners.has(item.kind)) return kindOpeners.get(item.kind);
    if (item.directory || !suffixOpeners.size) return null;
    var name = String(item.ref && item.ref.path || item.path || '').replace(/\\/g, '/').split('/').pop().toLowerCase(), best = null;
    if (!name) return null;
    suffixOpeners.forEach(function (opener, suffix) {
      if (name.endsWith(suffix) && (!best || suffix.length > best.suffix.length)) best = opener;
    });
    return best;
  }
  function runOpener(opener, item, opts) { return Promise.resolve().then(function () { return opener.open(item, opts || {}); }); }
  /* Opens what a reference points to in its opener's window, for a caller
     that has the reference and not the item: a link, a Recent document, a
     command. The promise gives { result } when an opener took it, and null
     when none did, so the caller goes on with its own rules. An entry's
     kind is known only from its item, so the item is read when an opener
     for a kind is registered, and when one matched by name, so that the
     opener is given the whole item. */
  function openRef(ref, opts) {
    if (!ref || !kindOpeners.size && !suffixOpeners.size) return Promise.resolve(null);
    var bare = { ref: ref };
    if (!kindOpeners.size && !openerFor(bare)) return Promise.resolve(null);
    return verify(bare).then(function (current) { return Object.assign({}, current, { ref: current.ref || ref }); }, function () { return bare; }).then(function (item) {
      var opener = openerFor(item) || openerFor(bare);
      return opener ? runOpener(opener, item, opts).then(function (result) { return { result: result }; }) : null;
    });
  }
  /* plain opens the entry by the built-in rules alone, for a command that
     says where it opens, such as Open in Files. */
  function open(item, isCurrent, plain) {
    return verify(item).then(function (current) {
      if (isCurrent && !isCurrent()) return;
      var opener = plain ? null : openerFor(current);
      if (opener) return runOpener(opener, current, { intent: 'read' });
      if (current.ref && has(current, 'preview')) return dinahWorkbench.open(dinahWorkbench.resourceItem(current.ref, current.title));
      if (current.directory && has(current, 'list')) return openFiles(current.path, current.ref);
      if (['open-external', 'reveal'].some(function (action) { return has(current, action); })) return current.ref ? app().OpenWorkspaceResource(current.ref) : app().OpenWorkspaceMount(current.path);
      throw new Error('This resource has no supported Open action.');
    });
  }
  // The same item actions serve Files, Workspace and their context menus.
  function itemCommands(item, fail) {
    if (!item) return [];
    function run(fn) { return function () { return Promise.resolve().then(fn).catch(fail); }; }
    return [
      { id: 'open', label: item.directory ? 'Open in Files' : 'Open', run: run(function () { return open(item, null, !!item.directory); }) },
      { id: 'edit', label: 'Edit', hidden: !item.ref || !has(item, 'edit'), run: run(function () { return dinahEditor.openResource(item.ref); }) },
      { id: 'edit-text', label: 'Edit as Text', hidden: !item.ref || !has(item, 'edit-text'), run: run(function () { return dinahEditor.openResource(item.ref, 'text'); }) },
      { id: 'open-external', label: has(item, 'reveal') ? 'Show in Folder' : 'Open Externally', hidden: !has(item, 'reveal') && !has(item, 'open-external'), run: run(function () { return item.ref ? app().OpenWorkspaceResource(item.ref) : app().OpenWorkspaceMount(item.path); }) },
      // An item on a site has an address, which Copy Link puts on the clipboard.
      { id: 'copy-link', label: 'Copy Link', hidden: !item.url, run: run(function () { host().copy(item.url); return item.url; }) },
      { id: 'copy-path', label: 'Copy Path', hidden: !item.ref || !item.writable, run: run(function () { return copyPath(item); }) },
      { id: 'rename', label: 'Rename…', shortcut: 'F2', hidden: !renames(item), run: run(function () { return renameFile(item); }) }
    ].filter(function (command) { return !command.hidden; });
  }
  /* A product adds rows of its own to the menu of an entry or of a source
     (docs/design/product-applets.md, "A Product's Rows in an Entry's
     Menu"). Its page file registers a function for a kind of entry, or for
     a kind of source:

       dinahResources.registerEntryCommands({ itemKind: 'sample-query', commands: function (item) {
         return [{ id: 'sample-open-board', label: 'Open in Sample Board', glyph: 'board', run: function (item) { ... } },
           { id: 'sample-remove-query', label: 'Remove…', danger: true, disabled: item.changed, reason: 'It has changes that are not saved.', run: function (item) { ... } }];
       } });
       dinahResources.registerEntryCommands({ sourceKind: 'sample-tracker', commands: function (item) { ... } });

     An itemKind is matched against the kind of an entry inside a source,
     and a sourceKind against the kind of the source whose own row the menu
     is for. The function is called each time a menu for such an entry
     opens, and each time the Files window reads its commands, with the
     entry's item, and returns the rows for the entry as it is then. A row
     is { id, label, glyph, run(item), disabled, reason, danger }. The rows
     stand in a group of their own after the base's rows, and a danger row
     stands with the destructive rows at the end of the menu. A disabled
     row is drawn disabled with its reason. A run that throws, or whose
     promise is rejected, is reported as a failure of one of the base's
     rows is. A key that is already registered is refused with a message on
     the console and the first registration stays. A row without an id, a
     label or a run function, a row whose id one of the base's rows has,
     and a second row with the same id are left out with a message on the
     console. With nothing registered every menu is as it was. */
  var kindCommands = new Map(), sourceCommands = new Map(), refusedRows = new Set();
  /* The ids of the base's own rows for an entry, in an entry's menu and in
     the Files window's commands, where a product's rows are commands of
     the same window. */
  var BASE_COMMAND_IDS = ['open', 'edit', 'edit-text', 'open-external', 'copy-link', 'copy-path', 'rename', 'up', 'home', 'retry', 'browse-children', 'new-folder', 'upload', 'refresh', 'preview-pane', 'new-terminal', 'files-actions',
    'open-in-files', 'pin', 'unpin', 'refresh-source', 'refresh-children', 'remove-source', 'create-folders', 'choose-folder', 'move-up', 'move-down'];
  function registerEntryCommands(o) {
    o = o || {};
    var kind = typeof o.itemKind === 'string' ? o.itemKind.trim() : '', source = typeof o.sourceKind === 'string' ? o.sourceKind.trim() : '';
    if (typeof o.commands !== 'function' || !kind === !source) {
      console.error('dinahResources.registerEntryCommands needs a commands function and either an itemKind or a sourceKind. Nothing was registered.');
      return false;
    }
    var table = kind ? kindCommands : sourceCommands, key = kind || source;
    if (table.has(key)) {
      console.error('dinahResources.registerEntryCommands: commands are already registered for the ' + (kind ? 'item kind "' : 'source kind "') + key + '". The first registration stays and this one is refused.');
      return false;
    }
    table.set(key, o.commands);
    if (window.pudlCommands) window.pudlCommands.changed();
    return true;
  }
  /* Whether an item is a source's own row. In a tree it is known by its
     place, since a source's root may have a path of its own, and a pin of a
     source has no path. */
  function isSourceRoot(item) {
    if (!item || !item.ref || !item.ref.sourceId) return false;
    if (item.path) { var parts = item.path.split('/'); return parts[1] === 'workspaces' && parts.length === 4; }
    return !item.ref.path;
  }
  function refuseRow(key, id, why) {
    var once = key + '\n' + id + '\n' + why;
    if (refusedRows.has(once)) return;
    refusedRows.add(once);
    console.error('dinahResources.registerEntryCommands: a row for "' + key + '"' + (id ? ' with the id "' + id + '"' : '') + ' ' + why + ' The row is left out.');
  }
  /* A product's rows for an entry, as rows a menu and PUDL's commands both
     take. fail is given what a run throws or rejects with. current, when
     given, returns a promise of the item to run with, for a list that has
     less than the whole item when its menu opens. */
  function entryCommands(item, fail, current) {
    if (!item || !kindCommands.size && !sourceCommands.size) return [];
    var root = isSourceRoot(item), key = root ? sourceKind(item.ref) || item.kind || '' : item.kind || '';
    var make = key && (root ? sourceCommands : kindCommands).get(key);
    if (!make) return [];
    var list;
    try { list = make(item); } catch (err) { console.error('dinahResources.registerEntryCommands: the commands function for "' + key + '" failed.', err); return []; }
    if (!Array.isArray(list)) return [];
    var seen = new Set(), report = fail || function (err) { if (host() && host().fail) host().fail(err); };
    return list.map(function (c) {
      var id = c && typeof c.id === 'string' ? c.id.trim() : '';
      if (!c || !id || typeof c.label !== 'string' || !c.label.trim() || typeof c.run !== 'function') { refuseRow(key, id, 'needs an id, a label and a run function.'); return null; }
      if (BASE_COMMAND_IDS.indexOf(id) >= 0) { refuseRow(key, id, 'has the id of one of the base\'s own rows.'); return null; }
      if (seen.has(id)) { refuseRow(key, id, 'has the id of another row of the same entry.'); return null; }
      seen.add(id);
      return { id: id, label: c.label.trim(), glyph: typeof c.glyph === 'string' && c.glyph ? c.glyph : undefined, disabled: !!c.disabled, reason: typeof c.reason === 'string' && c.reason ? c.reason : undefined, danger: !!c.danger,
        run: function () { return Promise.resolve().then(function () { return current ? current() : item; }).then(function (now) { return c.run(now || item); }).catch(report); } };
    }).filter(Boolean);
  }
  /* The rows of an entry's menu with the product's rows in their places.
     base is the rows the base gives, which may end with its destructive
     group: a separator followed by rows of which one is a danger row. The
     product's rows follow the base's other rows in a group of their own,
     and its danger rows stand first in the destructive group, so that the
     base's own destructive row stays last. With no rows from a product,
     base is returned as it was given. */
  function withEntryCommands(base, item, fail, current) {
    var own = entryCommands(item, fail, current);
    if (!own.length) return base;
    var cut = base.lastIndexOf('-'), tail = cut >= 0 && base.slice(cut + 1).some(function (r) { return r && r.danger; }) ? base.slice(cut + 1) : [];
    var head = tail.length ? base.slice(0, cut) : base.slice();
    var plain = own.filter(function (r) { return !r.danger; }), danger = own.filter(function (r) { return r.danger; }).concat(tail);
    var rows = head;
    if (plain.length) rows = rows.concat(rows.length ? ['-'] : [], plain);
    if (danger.length) rows = rows.concat(rows.length ? ['-'] : [], danger);
    return rows;
  }
  /* The item of an entry a list has only a reference to, as the Pinned
     list has: what the list knows (its kind, its title, whether it is a
     folder) with the status, the changed mark and the counts the trees have
     listed for it. current reads the whole item, for a row's run. */
  function itemOfRef(ref, basics) {
    var marked = known.get(refKey(ref)) || {};
    var item = Object.assign({}, basics, { ref: ref });
    ['status', 'changed', 'count', 'changedCount'].forEach(function (name) { if (marked[name] != null && marked[name] !== false) item[name] = marked[name]; });
    return { item: item, current: function () { return verify({ ref: ref }).then(function (now) { return Object.assign({}, now, { ref: now.ref || ref }); }, function () { return item; }); } };
  }
  function currentPath() {
    return app().State().then(function (state) {
      var record = (state.workspaceRecords || []).find(function (entry) { return entry.id === state.workspaceId; });
      return record && record.mountAlias ? '/workspaces/' + record.mountAlias : '/workspaces';
    });
  }
  /* The home folder's path, /home/<name>, from Dinah; until it
     answers, the name every installation starts with. */
  var homePath = '/home/dinah', homeLoading = null;
  function loadHome(again) {
    if (!homeLoading || again) {
      homeLoading = Promise.resolve().then(function () { return app() && app().HomeMountPath ? app().HomeMountPath() : homePath; })
        .then(function (path) { if (typeof path === 'string' && /^\/home\/[^/]+$/.test(path)) homePath = path; return homePath; }, function () { return homePath; });
    }
    return homeLoading;
  }
  /* A resource's glyph by what the item is: the one its kind gives it (a
     page on a site, an issue), else a folder or a document. Every surface
     that lists resources chooses through this, so they agree. */
  function glyphName(item) { return item.glyph || (item.directory ? 'folder' : 'document'); }
  /* An entry's status and its changed mark, which a source's provider sets
     on the entry (resource.Entry.Status and Changed,
     docs/design/source-kinds.md). Every surface that lists entries draws
     them through marks, so they agree: the changed mark in a place of its
     own, and the status as PUDL's badge, whose state gives its glyph and
     colour together. The style sheet puts the mark's place after the badge
     at the end of a row, and before it in Files, where the marks follow
     the name. An entry with neither has no marks and is drawn as it
     was before. The changed mark's place is kept whenever there is a
     status, so the badge and the name do not move when the mark comes or
     goes.

     A list too narrow for a name and a badge with its words shows the
     badge compactly, as its glyph and colour alone, and the label is then
     the badge's accessible name and its tooltip, which it has in both
     forms. The style sheet chooses the form from the width of the list
     with a container query (app.css, "An entry's status"), and no script
     measures a row. A badge's words are never cut short, so the width at
     which they are shown depends on how long the label is: the badge says
     which of four lengths its label has, by the number of its characters,
     and the longest are never shown in a row. */
  function wordsSize(label) { return label.length <= 12 ? 'short' : label.length <= 24 ? 'medium' : label.length <= 40 ? 'long' : 'none'; }
  var BADGE_STATES = { accent: true, positive: true, warn: true, danger: true };
  function statusOf(item) {
    var status = item && item.status;
    if (!status || typeof status.label !== 'string' || !status.label.trim()) return null;
    return { label: status.label.trim(), state: BADGE_STATES[status.state] ? status.state : '' };
  }
  /* A folder's counts, which a source's provider sets on the folder's entry
     (resource.Entry.Count and ChangedCount, docs/design/source-kinds.md, "A
     Folder's Counts"): how many entries the folder stands for, and how many
     of them have changes the person has not seen. A count that is not
     known is absent from the item, which is different from a count of
     zero. The total is drawn as the count badge the section heads have,
     and the changed count beside it as PUDL's accent badge with the
     changed mark's glyph. A count that is not known draws nothing and
     keeps no place, and a changed count of zero draws nothing. An entry
     that is not a folder has no counts. The counts are the last of the
     marks, so the total ends where the marks end. */
  function wholeCount(value) { return typeof value === 'number' && isFinite(value) && value >= 0 ? Math.floor(value) : null; }
  function countsOf(item) {
    if (!item || !item.directory) return null;
    var total = wholeCount(item.count), changed = wholeCount(item.changedCount) || null;
    return total === null && changed === null ? null : { total: total, changed: changed };
  }
  function hasMarks(item) { return !!item && (!!statusOf(item) || !!item.changed || !!countsOf(item)); }
  function countMark(className, number, name, tip) {
    var mark = document.createElement('span');
    mark.className = className; mark.textContent = String(number);
    mark.setAttribute('role', 'img');
    mark.setAttribute('aria-label', name);
    mark.title = tip || name;
    return mark;
  }
  function countMarks(counts) {
    var host = document.createElement('span');
    var total = counts.total === null ? '' : counts.total === 1 ? '1 entry' : counts.total + ' entries';
    var changed = counts.changed === null ? '' : counts.changed + ' changed since you last looked';
    host.className = 'resource-counts' + (total && changed ? ' resource-counts-both' : '');
    // The changed count's tooltip has the total too, for a list too narrow to draw both.
    if (changed) host.appendChild(countMark('badge accent resource-count-changed', counts.changed, changed, total ? changed + ', of ' + total : changed));
    if (total) host.appendChild(countMark('badge browser-group-count resource-count-total', counts.total, total));
    return host;
  }
  function marksKey(item) { var status = statusOf(item), counts = countsOf(item); return JSON.stringify([status && status.label, status && status.state, !!(item && item.changed), counts && counts.total, counts && counts.changed]); }
  function marks(item) {
    var status = statusOf(item), changed = !!(item && item.changed), counts = countsOf(item);
    if (!status && !changed && !counts) return null;
    var host = document.createElement('span'), slot = document.createElement('span');
    host.className = 'resource-marks' + (counts ? status || changed ? ' resource-marks-counted' : ' resource-marks-counts-only' : ''); host.dataset.marks = marksKey(item);
    slot.className = 'resource-changed-slot';
    if (changed) {
      var mark = document.createElement('span');
      mark.className = 'glyph resource-changed';
      mark.setAttribute('role', 'img');
      mark.setAttribute('aria-label', 'Changed since you last looked');
      mark.title = 'Changed since you last looked';
      slot.appendChild(mark);
    }
    // A folder with counts alone has no place for the changed mark, since nothing is kept for what may never come.
    if (status || changed) host.appendChild(slot);
    if (status) {
      var badge = document.createElement('span'), words = document.createElement('span');
      badge.className = 'badge resource-status' + (status.state ? ' ' + status.state : '');
      badge.setAttribute('role', 'img');
      badge.setAttribute('aria-label', 'Status: ' + status.label);
      badge.title = 'Status: ' + status.label;
      badge.dataset.words = wordsSize(status.label);
      words.className = 'resource-status-words'; words.textContent = status.label;
      badge.appendChild(words); host.appendChild(badge);
    }
    if (counts) host.appendChild(countMarks(counts));
    return host;
  }
  /* The statuses the trees have listed, by reference, for a list that has
     an entry's reference and not its item, as the Pinned list has. An entry
     no tree has listed is asked for once, when it is in a source that is
     not a folder of files, since those have no statuses. A change is
     announced with the dinah:resource-status event, and a change at a
     source has every asked entry asked for again. */
  var known = new Map(), asked = new Set(), announcing = false;
  function refKey(ref) { return JSON.stringify([ref.workspaceId || '', ref.sourceId || '', ref.path || '']); }
  function announce() {
    if (announcing) return;
    announcing = true;
    setTimeout(function () { announcing = false; document.dispatchEvent(new CustomEvent('dinah:resource-status')); }, 0);
  }
  function learn(item) {
    if (!item || !item.ref || !item.ref.sourceId) return;
    var key = refKey(item.ref), was = known.get(key), marked = hasMarks(item), counts = countsOf(item);
    if (!marked && !was) return;
    var now = marked ? { status: statusOf(item), changed: !!item.changed, directory: !!item.directory, count: counts ? counts.total : null, changedCount: counts ? counts.changed : null } : null;
    if (was && now && marksKey(was) === marksKey(now)) return;
    if (now) known.set(key, now); else known.delete(key);
    announce();
  }
  function sourceKind(ref) {
    var records = window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.records() : [];
    var record = records.find(function (r) { return r.id === ref.workspaceId; });
    var source = record && (record.sources || []).find(function (s) { return s.id === ref.sourceId; });
    return source ? source.kind : '';
  }
  function marksOf(ref) {
    if (!ref || !ref.sourceId) return null;
    var key = refKey(ref), kind = sourceKind(ref);
    if (!asked.has(key) && kind && kind !== 'local-folder' && ref.path) {
      asked.add(key);
      verify({ ref: ref }).then(function (current) { learn(Object.assign({}, current, { ref: ref })); }).catch(function () {});
    }
    return marks(known.get(key));
  }
  subscribe(function (event) {
    if (!asked.size || !event || event.kind !== 'source-changed' && event.kind !== 'resync') return;
    asked.clear(); announce();
  });
  /* Whether a folder of the file system takes a new document: a folder on
     the computer that can be written. The rest is read-only: the top of the
     file system and the listing of a workspace's mounts. */
  function readOnly(node) { return !!node && !!node.directory && !node.writable; }
  /* The read-only mark: a struck-through pencil, as the Finder shows for a
     folder that cannot be written to, PUDL's pencil-off glyph. */
  function readOnlyMark() {
    var mark = document.createElement('span');
    mark.className = 'glyph fm-readonly-mark';
    mark.setAttribute('role', 'img');
    mark.setAttribute('aria-label', 'Read-only');
    mark.title = 'Read-only: nothing can be saved here';
    return mark;
  }
  /* New Folder, shared by Open, Save As and Files: a folder on the computer
     that can be written takes a new folder. The name is asked in a PUDL
     dialog over whatever is open, the folder is made, the listing is loaded
     again, and the promise gives the new folder's node, or null when the
     person cancels. */
  function takesFolder(node) { return !!node && !!node.directory && !!node.writable && !!node.ref; }

  /* Uploading copies files from anywhere on the computer into a folder of a
     workspace. When the folder already
     has files of the same names, one question says which and asks whether to
     replace them, keep both or skip them; Cancel uploads nothing. A toast
     says what was done, and the promise gives the report, or null when the
     person cancelled. */
  function host() { return window.dinahHost; }
  function names(list) { return list.length > 3 ? list.slice(0, 3).join(', ') + ' and ' + (list.length - 3) + ' more' : list.join(', '); }
  function upload(dir, paths) {
    if (!takesFolder(dir)) return Promise.reject(new Error('Files can be uploaded only to a folder on the computer for now.'));
    if (!paths || !paths.length) return Promise.resolve(null);
    return Promise.resolve(app().UploadConflicts(dir.ref, paths)).then(function (taken) { return existingChoice(dir, taken); }).then(function (existing) {
      if (!existing) return null;
      return Promise.resolve(app().UploadToWorkspaceFolder(dir.ref, paths, existing)).then(function (r) { return reported(dir, r); });
    });
  }
  /* Files dropped from the system's file manager reach the page as their
     contents and names, not their paths, so they are sent to the shell's
     POST /upload, which applies the same rules as an upload by path. */
  function uploadDropped(dir, files, folders) {
    if (!takesFolder(dir)) return Promise.reject(new Error('Files can be uploaded only to a folder on the computer for now.'));
    var refused = folders.map(function (n) { return n + ': a folder is not uploaded; upload the files in it'; });
    if (!files.length) return Promise.resolve(reported(dir, { refused: refused }));
    var list = files.map(function (f) { return f.name; });
    return Promise.resolve(app().UploadNameConflicts(dir.ref, list)).then(function (taken) { return existingChoice(dir, taken); }).then(function (existing) {
      if (!existing) return null;
      var form = new FormData();
      form.append('folder', JSON.stringify(dir.ref));
      form.append('existing', existing);
      files.forEach(function (f) { form.append('file', f, f.name); });
      return fetch('/upload', { method: 'POST', credentials: 'same-origin', body: form }).then(function (res) {
        if (!res.ok) return res.text().then(function (t) { throw new Error(t || 'The upload failed.'); });
        return res.json();
      }).then(function (r) { r.refused = refused.concat(r.refused || []); return reported(dir, r); });
    });
  }
  /* What happens to names the folder already has: one question says which
     and asks whether to replace them, keep both or skip them. "skip" when
     there are none, and null when the person cancelled. */
  function existingChoice(dir, taken) {
    if (!taken || !taken.length) return 'skip';
    var where = dir.title || dir.name || dir.path, one = taken.length === 1;
    return host().ask(one ? 'Replace ' + taken[0] + '?' : 'Replace ' + taken.length + ' Files?',
      where + ' already has ' + (one ? 'a file named ' + taken[0] : 'files named ' + names(taken)) + '. Replace ' + (one ? 'it' : 'them') + ', keep both (the new ' + (one ? 'one is' : 'ones are') + ' named with a number), or skip ' + (one ? 'it' : 'them') + ' and upload the rest.',
      'Replace', true, [{ label: 'Skip', value: 'skip' }, { label: 'Keep Both', value: 'keep-both' }])
      .then(function (answer) { return answer === true ? 'replace' : answer || null; });
  }
  /* A file the page made, such as a picture of what a window draws, saved
     where the person chooses: the Save dialog starts in the current
     workspace's main folder, takes a folder on the computer that can be
     written, and has To Your Computer… for anywhere else. It starts in the
     main folder because the workspace itself is the list of what is
     attached to it and takes no file. A workspace with no main folder
     starts in its one folder of files, and at that list when it has none
     or several. o is { name, title, blob }. The
     promise gives { name, where } or { path } once the file is written, or
     null when the person cancelled. A name the folder already has is asked
     about, as an upload's is. */
  function saveFile(o) {
    var done = null;
    function base64(blob) {
      return new Promise(function (resolve, reject) {
        var reader = new FileReader();
        reader.onload = function () { resolve(String(reader.result).split(',')[1] || ''); };
        reader.onerror = function () { reject(new Error('The file could not be read.')); };
        reader.readAsDataURL(blob);
      });
    }
    function mainFolder() {
      return Promise.resolve(app().State()).then(function (state) {
        var record = (state.workspaceRecords || []).find(function (entry) { return entry.id === state.workspaceId; });
        /* The main folder, or the one folder of files of a workspace
           that has no main one. */
        var folders = record ? (record.sources || []).filter(function (s) { return s.kind === 'local-folder'; }) : [];
        var source = record && record.primarySourceId || (folders.length === 1 ? folders[0].id : '');
        if (!source || !app().ResolveWorkspaceResourceMount) return undefined;
        return Promise.resolve(app().ResolveWorkspaceResourceMount({ workspaceId: record.id, sourceId: source, path: '' })).then(function (item) {
          return item && item.directory && item.writable ? item.path : undefined;
        });
      }).catch(function () { return undefined; });
    }
    return mainFolder().then(function (start) {
    return window.dinahFilePicker.pick({ mode: 'save', title: o.title || 'Save As', name: o.name, start: start,
      saveKind: function (dir) {
        return takesFolder(dir) ? { kind: 'file' } : { why: 'Read-only. Open a folder that can be written, or use To Your Computer… for a place outside the workspace.' };
      },
      commit: function (choice) {
        if (choice.computer) {
          return base64(o.blob).then(function (data) { return app().SaveFileToComputer(host().workspaceId(), choice.computer, data); }).then(function (path) {
            if (!path) return false;
            done = { path: path };
            return true;
          });
        }
        var dir = choice.dir, where = dir.title || dir.name || dir.path;
        return Promise.resolve(app().UploadNameConflicts(dir.ref, [choice.name])).then(function (taken) { return existingChoice(dir, taken); }).then(function (existing) {
          if (!existing) return false;
          var form = new FormData();
          form.append('folder', JSON.stringify(dir.ref));
          form.append('existing', existing);
          form.append('file', new File([o.blob], choice.name, { type: o.blob.type || 'application/octet-stream' }), choice.name);
          return fetch('/upload', { method: 'POST', credentials: 'same-origin', body: form }).then(function (res) {
            if (!res.ok) return res.text().then(function (t) { throw new Error(t || 'The file could not be saved.'); });
            return res.json();
          }).then(function (r) {
            if ((r.refused || []).length) throw new Error(r.refused.join('; '));
            var written = (r.added || []).concat(r.replaced || [], r.kept || []);
            if (!written.length) return false;
            done = { name: choice.name, where: where };
            return true;
          });
        });
      } });
    }).then(function () {
      /* Said once the dialog has gone: a toast made while a dialog is open
         belongs to it and goes with it. */
      if (done) host().toast(done.path ? 'Saved ' + done.path + '.' : 'Saved ' + done.name + ' to ' + done.where + '.', 'info');
      return done;
    });
  }
  /* A toast says what an upload did; the report is passed on. */
  function reported(dir, r) {
    var where = dir.title || dir.name || dir.path;
    var done = (r.added || []).length + (r.replaced || []).length + (r.kept || []).length;
    var parts = [];
    if (done) parts.push((done === 1 ? '1 file' : done + ' files') + ' uploaded to ' + where + '.');
    if ((r.skipped || []).length) parts.push('Skipped: ' + names(r.skipped) + '.');
    if ((r.refused || []).length) parts.push('Not uploaded: ' + r.refused.join('; ') + '.');
    var refused = (r.refused || []).length > 0;
    host().toast(parts.join(' ') || 'Nothing was uploaded.', refused ? 'warn' : 'info', { keep: refused });
    return r;
  }
  function chooseAndUpload(dir) {
    return Promise.resolve(app().ChooseUploadFiles()).then(function (paths) { return paths && paths.length ? upload(dir, paths) : null; });
  }
  function copyPath(item) {
    return Promise.resolve(app().WorkspaceLocalPath(item.ref)).then(function (p) { host().copy(p); return p; });
  }
  /* Files dropped from the system's file manager land on a drop target: an
     element registered here, which says which folder a drop on it means
     with a resolver, so the Workspace tree and Files each decide. Only a
     drag that carries files is taken, and the target is outlined while one
     is over it (app.css, .upload-drop-active). */
  var dropTargets = new WeakMap(), dropOver = null;
  function uploadTarget(el, resolve) { el.dataset.uploadTarget = ''; dropTargets.set(el, resolve); }
  function targetOf(node) {
    var el = node;
    while (el && !(el.dataset && 'uploadTarget' in el.dataset && dropTargets.has(el))) el = el.parentElement;
    return el;
  }
  function carriesFiles(e) { return e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types || [], 'Files') >= 0; }
  function markDrop(el) {
    if (dropOver === el) return;
    if (dropOver) dropOver.classList.remove('upload-drop-active');
    dropOver = el;
    if (el) el.classList.add('upload-drop-active');
  }
  document.addEventListener('dragover', function (e) {
    if (!carriesFiles(e)) return;
    var el = targetOf(e.target);
    markDrop(el);
    if (!el) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = 'copy';
  });
  document.addEventListener('dragleave', function (e) { if (!e.relatedTarget) markDrop(null); });
  document.addEventListener('drop', function (e) {
    if (!carriesFiles(e)) return;
    var el = targetOf(e.target);
    markDrop(null);
    if (!el) return;
    e.preventDefault();
    var dir = dropTargets.get(el)(e.target);
    if (!dir) return;
    // A dropped folder arrives as an entry that is a directory, which is
    // reported rather than sent.
    var files = [], folders = [], items = e.dataTransfer.items || [];
    for (var i = 0; i < items.length; i++) {
      if (items[i].kind !== 'file') continue;
      var entry = items[i].webkitGetAsEntry && items[i].webkitGetAsEntry(), f = items[i].getAsFile();
      if (entry && entry.isDirectory) folders.push(entry.name);
      else if (f) files.push(f);
    }
    uploadDropped(dir, files, folders).then(function (r) { if (r) notify({ kind: 'uploaded', ref: dir.ref }); }).catch(function (err) { host().toast(String(err && err.message || err), 'danger'); });
  });
  /* askName asks for a name in a PUDL dialog: o.title, o.label, o.value,
     o.help, o.yes, and o.selectTo, how much of the value starts selected.
     o.commit(name) does the work; while it runs the dialog waits, and an
     error it gives is said in the dialog, which stays open. The promise
     gives commit's result, or null when the person cancels or leaves the
     value as it was. */
  function askName(o) {
    return new Promise(function (resolve) {
      var dlg = document.createElement('dialog'), id = 'dinah-ask-name-' + Date.now();
      dlg.className = 'dialog dinah-ask-name';
      dlg.setAttribute('aria-labelledby', id + '-title');
      dlg.innerHTML = '<form method="dialog">' +
        '<h2 class="dialog-title" id="' + id + '-title"></h2>' +
        '<div class="dialog-body"><div class="form-group">' +
          '<label class="form-label" for="' + id + '-name"></label>' +
          '<input class="form-input" id="' + id + '-name" autocomplete="off" spellcheck="false" aria-describedby="' + id + '-help">' +
          '<p class="form-help" id="' + id + '-help"></p>' +
        '</div><p class="form-error" role="alert" hidden></p></div>' +
        '<div class="dialog-actions"><button type="button" class="btn" data-name="cancel">Cancel</button><button type="submit" class="btn btn-primary"></button></div>' +
      '</form>';
      var input = dlg.querySelector('input'), error = dlg.querySelector('.form-error'), yes = dlg.querySelector('[type="submit"]'), done = false;
      dlg.querySelector('.dialog-title').textContent = o.title;
      dlg.querySelector('.form-label').textContent = o.label;
      dlg.querySelector('.form-help').textContent = o.help || '';
      yes.textContent = o.yes || 'OK';
      input.value = o.value || '';
      function finish(result) { if (done) return; done = true; if (dlg.open) dlg.close(); dlg.remove(); resolve(result); }
      function say(text) { error.textContent = text || ''; error.hidden = !text; input.toggleAttribute('aria-invalid', !!text); }
      dlg.addEventListener('close', function () { finish(null); });
      dlg.querySelector('[data-name="cancel"]').addEventListener('click', function () { finish(null); });
      input.addEventListener('input', function () { say(''); });
      dlg.addEventListener('submit', function (event) {
        event.preventDefault();
        var name = input.value.trim();
        if (!name) { say('Type a ' + o.label.toLowerCase() + '.'); return; }
        if (name === (o.value || '')) { finish(null); return; }
        yes.disabled = true;
        Promise.resolve().then(function () { return o.commit(name); }).then(finish).catch(function (err) {
          var text = String(err && err.message || err).replace(/^Error:\s*/, '');
          say(text.charAt(0).toUpperCase() + text.slice(1) + (/[.!?]$/.test(text) ? '' : '.'));
          yes.disabled = false; input.focus();
        });
      });
      document.body.appendChild(dlg);
      dlg.showModal();
      input.focus();
      input.setSelectionRange(0, o.selectTo == null ? input.value.length : o.selectTo);
    });
  }
  /* Rename… for a file of a local folder, offered where the file's actions
     include rename: the file is renamed in its own folder, and its drafts,
     Recent entry and pin go with it (documents.Ops.Rename). The name is
     offered with its extension unselected, since the extension is usually
     kept. The promise gives the renamed file, or null when the person
     cancels. */
  function renames(item) { return !!item && has(item, 'rename') && !!item.ref && !!app().RenameDocumentFile; }
  function renameFile(item) {
    if (!item || !item.ref || !item.ref.path) return Promise.reject(new Error('Only a file in a folder on the computer can be renamed here.'));
    var name = item.ref.path.slice(item.ref.path.lastIndexOf('/') + 1), dot = name.lastIndexOf('.');
    return askName({ title: 'Rename the File', label: 'Name', value: name, yes: 'Rename', selectTo: dot > 0 ? dot : null,
      help: 'The file keeps its folder. A document open in the Editor follows it, with its changes.',
      commit: function (next) { return app().RenameDocumentFile(item.ref, next); } }).then(function (renamed) {
      if (!renamed) return null;
      notify({ kind: 'renamed', from: item.ref, ref: renamed.ref });
      return renamed;
    });
  }
  function newFolder(F, dir) {
    if (!takesFolder(dir)) return Promise.reject(new Error('A folder can be made only in a folder on the computer.'));
    return new Promise(function (resolve) {
      var dlg = document.createElement('dialog'), id = 'dinah-new-folder-' + Date.now();
      dlg.className = 'dialog dinah-new-folder';
      dlg.setAttribute('aria-labelledby', id + '-title');
      dlg.innerHTML = '<form method="dialog">' +
        '<h2 class="dialog-title" id="' + id + '-title">New Folder</h2>' +
        '<div class="dialog-body"><div class="form-group">' +
          '<label class="form-label" for="' + id + '-name">Folder Name</label>' +
          '<input class="form-input" id="' + id + '-name" autocomplete="off" spellcheck="false" value="New Folder">' +
          '<p class="form-help"></p>' +
        '</div><p class="form-error" role="alert" hidden></p></div>' +
        '<div class="dialog-actions"><button type="button" class="btn" data-folder="cancel">Cancel</button><button type="submit" class="btn btn-primary">Create</button></div>' +
      '</form>';
      var input = dlg.querySelector('input'), error = dlg.querySelector('.form-error'), create = dlg.querySelector('[type="submit"]'), done = false;
      dlg.querySelector('.form-help').textContent = 'The folder is made in ' + (dir.title || dir.name || dir.path) + '.';
      function finish(node) { if (done) return; done = true; if (dlg.open) dlg.close(); dlg.remove(); resolve(node); }
      function say(text) { error.textContent = text || ''; error.hidden = !text; }
      dlg.addEventListener('close', function () { finish(null); });
      dlg.querySelector('[data-folder="cancel"]').addEventListener('click', function () { finish(null); });
      input.addEventListener('input', function () { say(''); });
      dlg.addEventListener('submit', function (event) {
        event.preventDefault();
        var name = input.value.trim();
        if (!name) { say('Give the folder a name.'); return; }
        create.disabled = true;
        Promise.resolve().then(function () { return window.dinahHelpers.app().CreateWorkspaceFolder(dir.ref, name); })
          .then(function () { return F.load(dir, false, true); })
          // A long folder is listed a page at a time, so the new folder is
          // looked up by its path when the first page does not have it.
          .then(function () { return F.childNamed(dir, name) || F.lookup(dir.path.replace(/\/$/, '') + '/' + name); })
          .then(function (node) { finish(node || null); })
          .catch(function (err) {
            var text = String(err && err.message || err).replace(/^Error:\s*/, '');
            say(text.charAt(0).toUpperCase() + text.slice(1) + (/[.!?]$/.test(text) ? '' : '.'));
            create.disabled = false; input.focus();
          });
      });
      document.body.appendChild(dlg);
      dlg.showModal();
      input.focus(); input.select();
    });
  }
  // The path a person is shown is made of titles where entries have them,
  // because a name can be an identifier that tells them nothing (a page's
  // number on a site). The path itself, which Copy Path gives, is unchanged.
  function shownPath(node) {
    var parts = [];
    for (var n = node; n && n.path !== '/'; n = n.parent) if (!n.contentFacet) parts.unshift(n.title || n.name);
    return n ? '/' + parts.join('/') : node.path;
  }
  /* Marks a file browser over a tree for what it shows, after each redraw:
     a read-only folder gets the read-only mark, a crumb shows its folder's
     title, and the Date and Size columns are hidden when no entry has them.
     Files and the Editor's Open and Save As dialog share it; root is the
     element with the dinah-files class. */
  function decorate(root, F, cwd) {
    /* An entry whose kind has a glyph of its own is drawn with it, in the
       tree and the rows, through the fm-kind-glyph class and the
       --resource-glyph property (files-applet.css). */
    function mark(element, node) {
      if (!node) return;
      element.classList.toggle('fm-read-only', readOnly(node));
      element.classList.toggle('fm-kind-glyph', !!node.glyph);
      if (node.glyph) element.style.setProperty('--resource-glyph', 'var(--glyph-' + node.glyph + ')');
      else element.style.removeProperty('--resource-glyph');
    }
    function marked(host, node) {
      if (!host) return;
      var old = host.querySelector(':scope > .fm-readonly-mark');
      if (readOnly(node) && !old) host.appendChild(readOnlyMark());
      else if (!readOnly(node) && old) old.remove();
      /* The entry's status and changed mark stand last, at the end of the
         row, after the read-only mark. */
      var had = host.querySelector(':scope > .resource-marks'), now = hasMarks(node) ? marksKey(node) : '';
      if (had && had.dataset.marks === now) { if (host.lastChild !== had) host.appendChild(had); }
      else { if (had) had.remove(); if (now) host.appendChild(marks(node)); }
      host.classList.toggle('fm-has-marks', !!now);
    }
    root.querySelectorAll('[data-fb="tree"] a[data-path]').forEach(function (link) { var node = F.resolve(null, link.dataset.path); mark(link, node); marked(link, node); });
    root.querySelectorAll('.fm-row[data-name]').forEach(function (row) {
      var node = F.childNamed(cwd, row.dataset.name);
      mark(row, node);
      if (node) marked(row.querySelector('.fm-c-name .fm-cell'), node);
    });
    root.querySelectorAll('[data-fb="crumbs"] a[data-path], [data-fb="crumbs"] [aria-current="location"]').forEach(function (crumb) {
      var node = crumb.dataset.path ? F.resolve(null, crumb.dataset.path) : cwd;
      if (!node) return;
      // The folder shown carries the mark, as the Finder's status bar does.
      if (!crumb.dataset.path) marked(crumb, node);
      if (node.path === '/' || node.path === '/home') return;
      // The crumb's words are its first text; the mark after them stays.
      var words = crumb.firstChild && crumb.firstChild.nodeType === 3 ? crumb.firstChild : crumb.insertBefore(document.createTextNode(''), crumb.firstChild);
      words.textContent = node.title || node.name;
      // An entry kept under something other than the words shown says those
      // words in full, as the workspace's tree does (workspace-browser.js).
      // A workspace and a source keep their paths, which Copy Path gives.
      var inSource = node.ref && node.ref.path && !node.contentFacet;
      crumb.title = inSource && node.title && node.title !== node.name ? node.title : node.path;
    });
    root.classList.toggle('fm-no-date', !(cwd.children || []).some(function (node) { return node.date; }));
    root.classList.toggle('fm-no-size', !(cwd.children || []).some(function (node) { return typeof node.size === 'number'; }));
  }
  function createTree() {
    var nodes = new Map(), identities = new Map(), disposed = false, listeners = new Set(), refreshing = false, pending = false;
    function identity(item) {
      if (!item.ref) return null;
      var root = item.ref.sourceId && item.path.startsWith('/workspaces/') && item.path.split('/').length === 4;
      return JSON.stringify([item.ref.workspaceId, item.ref.sourceId, root ? '' : item.ref.path || '']);
    }
    function put(item) {
      var key = identity(item), retained = key && identities.get(key), node = nodes.get(item.path);
      if (retained && retained.path !== item.path) {
        if (nodes.get(retained.path) === retained) nodes.delete(retained.path);
        node = retained; nodes.set(item.path, node);
      }
      if (node && (node.ref || item.ref) && !compatibleRef(node.ref, item.ref)) {
        nodes.forEach(function (_, path) { if (path.startsWith(item.path + '/')) nodes.delete(path); });
        node = null;
      }
      if (!node) { node = { name: item.path.slice(item.path.lastIndexOf('/') + 1), path: item.path, loaded: false, nextStart: -1 }; nodes.set(item.path, node); }
      Object.assign(node, item);
      // A folder that was missing and now exists is listed without the mark,
      // and so is a source that has become available.
      if (!item.missing) delete node.missing;
      if (!item.unavailable) delete node.unavailable;
      // An entry that has lost its status, or its changes, is listed without them.
      if (!item.status) delete node.status;
      if (!item.changed) delete node.changed;
      // A folder whose count is no longer known is listed without it.
      if (item.count == null) delete node.count;
      if (item.changedCount == null) delete node.changedCount;
      learn(node);
      node.name = item.path.slice(item.path.lastIndexOf('/') + 1);
      if (key) identities.set(key, node);
      if (item.directory && !node.children) node.children = [];
      return node;
    }
    var root = put({ path: '/', title: '/', kind: 'directory', directory: true, actions: ['list'] });
    var homeParent = put({path:'/home',title:'home',kind:'catalog',directory:true,actions:['list']}); homeParent.parent = root;
    function homeNode(path) {
      var node = put({path:path,title:path.slice(path.lastIndexOf('/') + 1),kind:'directory',directory:true,writable:true,actions:['list'],ref:{workspaceId:'',sourceId:'dinah-home',path:''}});
      node.parent = homeParent; return node;
    }
    // The home folder is /home/<name>, and the name can change; the folder
    // keeps its identity.
    var home = homeNode(homePath);
    function placeHome(path) {
      if (disposed || !path || path === home.path) return;
      nodes.delete(home.path); home = homeNode(path);
      homeParent.children = (homeParent.children || []).filter(function (child) { return child.ref && child.ref.sourceId !== 'dinah-home'; }).concat([home]);
      listeners.forEach(function (fn) { fn(); });
    }
    loadHome().then(placeHome);
    function contentOf(node) {
      if (!node || !node.directory || !has(node, 'preview')) return null;
      var content = node.contentEntry || (node.contentEntry = {});
      Object.assign(content, {name:'@content',title:node.title,path:node.path,kind:node.kind,directory:false,ref:node.ref,revision:node.revision,parent:node,contentFacet:true,actions:(node.actions || []).filter(function (action) { return action !== 'list'; })});
      return content;
    }
    async function lookup(path) {
      var node = nodes.get(path);
      if (node) return node;
      var item = await app().StatWorkspaceMount(path);
      if (disposed) throw new Error('The resource browser was closed.');
      node = put(item);
      if (node.path !== '/') {
        node.parent = await lookup(node.path.slice(0, node.path.lastIndexOf('/')) || '/');
        if (!node.parent.children.includes(node)) node.parent.children.push(node);
      }
      return node;
    }
    function load(node, more, force) {
      if (!node || !node.directory) return Promise.resolve();
      if (node.loading) return node.loading;
      var start = more ? node.nextStart : 0;
      node.loading = verify(node).then(function (current) {
        put(current);
        if (node.loaded && !more && !force || more && node.nextStart < 0) return null;
        return node.ref ? app().ListWorkspaceMountsExpected(node.path, start, 100, node.ref) : app().ListWorkspaceMounts(node.path, start, 100);
      }).then(function (page) {
        if (!page) return;
        if (disposed) return;
        var entries = (page.entries || []).map(function (item) { var child = put(item); child.parent = node; return child; });
        node.children = more ? node.children.concat(entries.filter(function (child) { return !node.children.some(function (old) { return old.path === child.path; }); })) : entries;
        if (!node.loaded) watchFolders();
        node.loaded = true;
        node.error = '';
        node.loadedThrough = start;
        node.nextStart = typeof page.nextStart === 'number' ? page.nextStart : -1;
        if (node.nextStart >= 0 && node.nextStart <= start) throw new Error('The resource provider returned an invalid next page.');
      }).finally(function () { node.loading = null; });
      return node.loading;
    }
    async function refresh() {
      if (refreshing) { pending = true; return; }
      refreshing = true;
      try {
        do {
          pending = false;
          // Refresh ancestors as well as loaded listings so labels and breadcrumbs agree.
          for (var node of Array.from(nodes.values())) {
            if (disposed) return;
            try {
              put(await verify(node));
              if (node.path !== '/') node.parent = await lookup(node.path.slice(0, node.path.lastIndexOf('/')) || '/');
              node.error = '';
              if (node.loaded && node.directory) {
                var through = node.loadedThrough || 0;
                await load(node, false, true);
                while (node.nextStart >= 0 && node.nextStart <= through) await load(node, true);
              }
            } catch (error) { node.error = String(error); }
          }
          if (!disposed) listeners.forEach(function (fn) { fn(); });
        } while (pending && !disposed);
      } finally { refreshing = false; }
    }
    var unsubscribe = subscribe(function (event) {
      if (event && event.kind === 'home-renamed') loadHome(true).then(placeHome);
      refresh().catch(function () {});
    });
    function opened() {
      return Array.from(nodes.values()).filter(function (n) { return n.directory && n.loaded && n.ref; }).map(function (n) { return n.ref; });
    }
    trees.add(opened);
    return {
      root: function () { return root; }, home: function () { return home; }, refresh: refresh,
      lookup: lookup, load: load, resolve: function (_, path) { return nodes.get(path); },
      pathOf: function (node) { return node.path; }, displayPath: function (node) { return node.path; },
      listEntries: function (node) { var content = contentOf(node); return (content ? [content] : []).concat(node.children || []); },
      realOf: function (node) { return node; }, childNamed: function (node, name) { return name === '@content' ? contentOf(node) : (node.children || []).find(function (child) { return child.name === name; }); },
      sameNode: function (a, b) { return !!a && !!b && a.path === b.path && (!(a.ref || b.ref) || sameRef(a.ref, b.ref)); },
      kindOf: function (node) { return node.directory ? 'dir' : 'file'; },
      size: function (node) { return node.size; }, onChange: function (fn) { listeners.add(fn); return function () { listeners.delete(fn); }; },
      destroy: function () { disposed = true; unsubscribe(); listeners.clear(); trees.delete(opened); watchFolders(); }
    };
  }
  function commands() {
    return dinahWorkbench.launchers().map(function (tool) { return {name:tool.id,description:tool.description || tool.title,usage:tool.id}; }).concat([{name:'applets',description:'Discover applet actions and invoke them through the shared SDK.',usage:'applets [list | actions [id] | invoke <action> <json> | run <action> <json> | status <id> | cancel <id>]'}]);
  }
  function executionPathOf(records, id) {
    var record = id && records.find(function (entry) { return entry.id === id; });
    if (!record) return '';
    if (record.executionPath) return record.executionPath;
    var primary = (record.sources || []).find(function (source) { return source.kind === 'local-folder' && (!record.primarySourceId || source.id === record.primarySourceId); });
    return primary ? primary.path : '';
  }
  async function invoke(name, args, stillCurrent, context) {
    if (name !== 'applets') {
      var tool = dinahWorkbench.launchers().find(function (entry) { return entry.id === name; });
      if (!tool) throw new Error('Unknown command: ' + name);
      if (args.length) throw new Error('Usage: ' + name);
      if (!stillCurrent || stillCurrent()) await tool.run();
      return '';
    }
    var catalog = await dinahAppletSDK.catalog(), operation = args[0] || 'list';
    if (stillCurrent && !stillCurrent()) return '';
    if (operation === 'list') return (catalog || []).map(function (entry) { return entry.id + '  ' + entry.title; }).join('\n');
    if (operation === 'actions') return (catalog || []).filter(function (entry) { return !args[1] || entry.id === args[1]; }).flatMap(function (entry) { return (entry.actions || []).map(function (action) { return JSON.stringify(action); }); }).join('\n');
    var state = await app().State(), provider = (catalog || []).find(function (entry) { return (entry.actions || []).some(function (action) { return action.ref === args[1]; }); });
    var records = state.workspaceRecords || [], recordId = state.workspaceId || '';
    if (context) {
      if (context.ref) recordId = context.ref.workspaceId;
      else {
        var alias = (context.path || '').split('/')[2], owner = records.find(function (entry) { return entry.mountAlias === alias; });
        recordId = (context.path || '').startsWith('/workspaces/') && owner ? owner.id : '';
      }
    }
    /* An action runs for a workspace's folder on the computer, which is what
       the other windows give it, so a workspace found by its ID is given by
       its folder; one with no folder of its own is no workspace here. */
    var workspace = context || recordId ? executionPathOf(records, recordId) : state.workspace || '';
    var sdk = dinahAppletSDK.forApplet(provider ? provider.id : 'dinah.terminal', workspace);
    if (stillCurrent && !stillCurrent()) return '';
    if (operation === 'invoke' && args.length === 3 && provider) return JSON.stringify(await sdk.invokeJSON(args[1], args[2]));
    /* run waits for the action to end and gives its output, for a script
       that needs the answer; a failed action is an error. */
    if (operation === 'run' && args.length === 3 && provider) {
      var started = await sdk.invokeJSON(args[1], args[2]);
      var ended = started.status === 'done' ? started : await sdk.wait(started.id, { get aborted() { return !!stillCurrent && !stillCurrent(); } });
      return ended.outputJSON || '{}';
    }
    if (operation === 'status' && args.length === 2) return JSON.stringify(await sdk.status(args[1]));
    if (operation === 'cancel' && args.length === 2) { await sdk.cancel(args[1]); return 'Cancelled.'; }
    throw new Error('Usage: applets [list | actions [id] | invoke <action> <json> | run <action> <json> | status <id> | cancel <id>]');
  }
  /* What a folder takes as a document's home, for Save As: a file by name in
     a folder on the computer that belongs to a workspace or to the home
     folder. The reasons say where to go, since the person has done nothing
     wrong. */
  function documentHomes() {
    return function (dir) {
      if (!dir.ref) return { why: 'Read-only. Open a workspace, or your home folder, to save the document in one of its folders.' };
      if (dir.writable) return { kind: 'file' };
      // A folder of a source that makes its own documents, such as a page
      // on a site, takes a new document by its title.
      if (dir.creates) return { kind: 'file' };
      // The workspace itself is the list of what is mounted in it, as a
      // workspace is in an editor, and takes no file of its own.
      if (dir.kind === 'workspace') return { why: 'Read-only. Open one of the workspace’s folders to save the document there.' };
      return { why: 'Read-only. A document is saved in a folder that can be written.' };
    };
  }

  window.dinahResources = { saveFile: saveFile, forget: forget, itemCommands: itemCommands, documentHomes: documentHomes, glyphName: glyphName, marks: marks, marksOf: marksOf, registerOpener: registerOpener, registerEntryCommands: registerEntryCommands, entryCommands: entryCommands, withEntryCommands: withEntryCommands, itemOfRef: itemOfRef, openerFor: openerFor, runOpener: runOpener, openRef: openRef, kindOpeners: function () { return kindOpeners.size > 0; }, open: open, openFiles: openFiles, openTerminal: openTerminal, currentPath: currentPath, createTree: createTree, decorate: decorate, shownPath: shownPath, readOnly: readOnly, readOnlyMark: readOnlyMark, takesFolder: takesFolder, newFolder: newFolder, askName: askName, renames: renames, renameFile: renameFile, upload: upload, chooseAndUpload: chooseAndUpload, copyPath: copyPath, uploadTarget: uploadTarget, homePath: loadHome, has: has, verify: verify, sameRef: sameRef, compatibleRef: compatibleRef, subscribe: subscribe, commands: commands, invoke: invoke };
})();
