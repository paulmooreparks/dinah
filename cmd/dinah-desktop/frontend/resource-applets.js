/* Files and Terminal use the same virtual mount namespace. */
(function () {
  'use strict';
  /* The workspace a Files window is on the taskbar under is the
     one of the place it was opened at: the workspace of its source when it
     has one, or the workspace whose folder in the catalog (/workspaces/<alias>)
     the path is in. A place outside any workspace, such as the home folder
     or the catalog itself, is no workspace's. */
  function filesWorkspace(item) {
    if (item.workspace) return item.workspace;
    var path = item.path || '';
    try { var parsed = JSON.parse(path); if (parsed && typeof parsed.path === 'string') path = parsed.path; } catch (_) { /* A plain path. */ }
    return { mount: path };
  }
  dinahWorkbench.register('files', { applet: { id: 'dinah.files', title: 'Files', glyph: 'folder', workspace: true }, title: 'Files', glyph: 'folder', workspaceOf: filesWorkspace, launcher: { mnemonic: 'F', description: 'Browse files and workspace sources.', run: function () { return dinahResources.openFiles('/workspaces'); } }, init: function (root, opts, item) { return dinahFiles.init(root, opts, item); } });
  // A terminal restored behind other windows starts when it is seen, or in
  // idle time after the windows in sight.
  dinahWorkbench.register('terminal', { applet: { id: 'dinah.terminal', title: 'Terminal', glyph: 'script', workspace: false }, title: 'Terminal', glyph: 'script', start: 'idle', launcher: { title: 'New Terminal', mnemonic: 'T', description: 'Use commands to browse workspace resources.', applet: 'dinah.terminal', run: function () { return dinahResources.currentPath().then(dinahResources.openTerminal); } }, init: function (root, opts, item) {
    var address;
    try { address = JSON.parse(item.path); } catch (_) { address = null; }
    if (!Array.isArray(address) || address.length < 2 || address.length > 4 || typeof address[0] !== 'string' || typeof address[1] !== 'string' || address[2] && (typeof address[2] !== 'object' || typeof address[2].workspaceId !== 'string' || typeof address[2].sourceId !== 'string') || address[3] != null && typeof address[3] !== 'string') throw new Error('The saved terminal address is invalid.');
    // The fourth part is the shell; a Terminal saved before there was a
    // choice runs Dinah's own.
    var shell = address[3] || 'dinah', terminal = Object.assign({}, item, { path: address[0], instanceId: address[1], ref: address[2] || null, initialRef: address[2] || null, shell: shell });
    return shell === 'dinah' ? dinahTerminal.init(root, opts, terminal) : dinahShellTerminal.init(root, opts, terminal);
  } });
})();
