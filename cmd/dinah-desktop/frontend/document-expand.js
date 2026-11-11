/* A document with its own shortcodes written out.

   A document can define shortcodes of its own ({{define:name}} ...
   {{/define}}), which the content applets replace by what they stand for
   when they draw it. A place a document is published to that is not the
   applets', as a page of a wiki is, has no reading of such a code, so the
   document is published with every use replaced and the definitions taken
   out (docs/design/content-applets.md, "A document's own shortcodes").

   Only the applets' engine has the rules for that: arguments and their
   defaults, nesting, the limits, and what is left alone inside code. The
   engine is on the page, so the program asks the page (internal/pageask,
   kind "document-expand") and this file answers with the engine's own
   call, yavcMdEngine().expand (yavc.app v0.2.75). The shortcodes a product
   brings are passed with the text, so that what is inside a container of
   the product's that holds text as it is written, is left as it is.

   The answer is { text, problems }. A use that cannot be expanded stays as
   it is written and is listed in problems, each with its name, a message
   and its place in the text. */
(function () {
  'use strict';
  var loading = null;
  function engine() {
    if (window.yavcMdEngine) return window.yavcMdEngine();
    if (!loading) {
      loading = new Promise(function (resolve, reject) {
        var s = document.createElement('script');
        s.src = 'yavc/content/md-engine/loader.js';
        s.onload = function () { if (window.yavcMdEngine) resolve(); else reject(new Error('The document engine could not be started.')); };
        s.onerror = function () { reject(new Error('The document engine could not be loaded.')); };
        document.head.appendChild(s);
      }).catch(function (err) { loading = null; throw err; });
    }
    return loading.then(function () { return window.yavcMdEngine(); });
  }
  function hosts() {
    if (!window.dinahShortcodeServices) return Promise.resolve([]);
    return window.dinahShortcodeServices.shortcodes().then(function (r) { return r && r.shortcodes || []; }, function () { return []; });
  }
  /* args: { text, shared }. shared is the text of a file of definitions
     several documents share, or nothing. */
  function expand(args) {
    args = args || {};
    return Promise.all([engine(), hosts()]).then(function (got) {
      if (!got[0] || typeof got[0].expand !== 'function') throw new Error('The document engine cannot write a document\'s own shortcodes out.');
      return got[0].expand({ text: String(args.text || ''), shared: String(args.shared || ''), hosts: got[1] });
    }).then(function (out) {
      return { text: String(out && out.text || ''), problems: (out && out.problems || []).map(function (p) { return { name: String(p.name || ''), message: String(p.message || ''), at: Number(p.at) || 0 }; }) };
    });
  }
  window.dinahDocumentExpand = { expand: expand };
  if (window.dinahWorkbench && window.dinahWorkbench.answerAsks) window.dinahWorkbench.answerAsks('document-expand', expand);
})();
