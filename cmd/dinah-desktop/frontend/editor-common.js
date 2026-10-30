/* What the Editor's files share that belongs to no window: the bridge
   accessor, the product name, the error message, the views and the
   helpers that read a draft's origin (window.dinahEditorCommon), and the
   registry the parts of an Editor window add themselves to
   (window.dinahEditorParts, used by editor-applet.js). */
(function () {
  'use strict';

  // The product's name as people see it (product.js).
  function productName() { return window.dinahProduct ? window.dinahProduct.name : ''; }
  function host() { return window.dinahHost; }
  /* Errors from Go start in lower case, as Go writes them; shown to the
     person, they read as sentences. */
  function message(err) { return window.dinahHelpers.message(err, 'sentence'); }

  /* The views, in the order the selector shows them and Ctrl+E goes
     through them: Visual (page-view.js, the document edited as its page),
     Text, Split (the text beside the page, both edited), Preview. The
     visual view's key stays "page" in the code, the saved state and the
     tests. */
  var VIEWS = { page: 'Visual', text: 'Text', split: 'Split', preview: 'Preview' };

  /* The resource a draft edits, as a workspace resource reference, or null
     for a new document. */
  // A workspace's name by its ID, from the registry the Workspace pane has.
  function workspaceName(id) {
    var records = id && window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.records() : [];
    var record = records.find(function (r) { return r.id === id; });
    return record ? record.name || '' : '';
  }
  function originRef(d) {
    if (!d || d.origin.kind === 'new') return null;
    return { workspaceId: d.origin.workspaceId, sourceId: d.origin.sourceId, path: d.origin.path };
  }
  function isMarkdown(d) {
    return !d || d.origin.kind !== 'local-folder' || /\.(md|markdown)$/i.test(d.origin.path);
  }

  window.dinahEditorCommon = {
    api: window.dinahHelpers.app, productName: productName, host: host, message: message,
    VIEWS: VIEWS, workspaceName: workspaceName, originRef: originRef, isMarkdown: isMarkdown
  };
  window.dinahEditorParts = {};
})();
