/* A diagram file as a picture, and a diagram file opened for editing.

   Two things a document that shows a saved diagram ({{diagram:path}}) needs
   of the window (docs/design/content-applets.md, "A diagram a document
   shows").

   A picture of it. A place a document is published to may be unable to
   draw a diagram file, as a page of a wiki is, and then shows a picture of
   it. Only the content applets can draw a canvas or a Mermaid diagram, and
   they are on the page, so the engine asks the page (internal/pageask,
   kind "diagram-picture") and this file answers with yavc.app's
   yavcDiagramPicture (content/diagram-kit/picture.js, which needs no
   applet window). The fonts in the picture are the page's own, so a canvas
   that uses a font a product gave the page is drawn with it. The applets'
   file is fetched the first time a picture is asked for.

   Edit Diagram. The applets ask the host to open a diagram file for
   editing (pudlHost.contentWorkspace.openDiagram, yavc.app v0.2.74), and
   only where the host does not take the request do they ask PUDL for the
   applet that handles the "diagram" verb, in a window PUDL opens. Dinah
   opens every document's window itself, one per document, and takes those
   verbs out of the applets' definitions (content-host.js), so it takes
   the request: the file is opened in Diagrams as any document is, in a
   window of its own (open, below, which content-host.js offers as the
   service). */
(function () {
  'use strict';
  function api() { return window.dinahHelpers.app(); }
  function message(err) { return String(err && err.message || err); }

  var loading = null;
  function kit() {
    if (window.yavcDiagramPicture) return Promise.resolve(window.yavcDiagramPicture);
    if (!loading) {
      loading = new Promise(function (resolve, reject) {
        var s = document.createElement('script');
        s.src = 'yavc/content/diagram-kit/picture.js';
        s.onload = function () { if (window.yavcDiagramPicture) resolve(window.yavcDiagramPicture); else reject(new Error('The diagram drawing could not be started.')); };
        s.onerror = function () { reject(new Error('The diagram drawing could not be loaded.')); };
        document.head.appendChild(s);
      }).catch(function (err) { loading = null; throw err; });
    }
    return loading;
  }
  function bytesOf(base64) {
    var raw = atob(base64), out = new Uint8Array(raw.length);
    for (var i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
    return out;
  }
  function base64Of(blob) {
    return new Promise(function (resolve, reject) {
      var reader = new FileReader();
      reader.onload = function () { resolve(String(reader.result).split(',')[1] || ''); };
      reader.onerror = function () { reject(new Error('The picture could not be read.')); };
      reader.readAsDataURL(blob);
    });
  }
  /* A path a canvas gives for one of its pictures, from the folder the
     canvas file is in, as one path without . and .. parts. */
  function joined(folder, path) {
    var parts = [];
    (folder ? folder.split('/') : []).concat(String(path).split('/')).forEach(function (p) {
      if (p === '..') parts.pop(); else if (p && p !== '.') parts.push(p);
    });
    return parts.join('/');
  }
  /* args: { name, path, kind, text, files, scale }. name is the diagram
     file's name, from which its kind is read when none is given. path is
     where the file is among files, a map of the files beside it by their
     paths, each as base64, from which a canvas's own pictures are read.
     The answer is { mime: 'image/png', data, width, height, problems }. */
  function draw(args) {
    args = args || {};
    var files = args.files || {}, at = String(args.path || args.name || ''), folder = at.indexOf('/') >= 0 ? at.slice(0, at.lastIndexOf('/')) : '';
    return kit().then(function (k) {
      var kind = args.kind || k.kindOf(String(args.name || ''));
      if (!kind) throw new Error('The file is not a D2, Mermaid or Canvas diagram.');
      return k.draw({ kind: kind, text: String(args.text || ''), format: 'png', scale: args.scale > 0 ? args.scale : 2,
        pictures: function (path) {
          var want = joined(folder, path), base = String(path).split('/').pop();
          var name = Object.prototype.hasOwnProperty.call(files, want) ? want : Object.keys(files).filter(function (f) { return f.split('/').pop() === base; })[0];
          return Promise.resolve(name ? { bytes: bytesOf(files[name]) } : null);
        } });
    }).then(function (out) {
      if (!out || !out.blob) throw new Error('The diagram was drawn as no picture.');
      return base64Of(out.blob).then(function (data) {
        return { mime: 'image/png', data: data, width: out.width, height: out.height, problems: (out.problems || []).map(String) };
      });
    });
  }
  window.dinahDiagramPicture = { draw: draw };
  if (window.dinahWorkbench && window.dinahWorkbench.answerAsks) window.dinahWorkbench.answerAsks('diagram-picture', draw);

  /* Edit Diagram: the file at a virtual path, under /workspaces or /home
     or the person's home as ~/, opened in Diagrams. */
  function openDiagram(path) {
    path = String(path || '');
    var found = path.indexOf('~/') === 0 ? Promise.resolve(api().HomeMountPath()).then(function (home) { return home + path.slice(1); }) : Promise.resolve(path);
    return found.then(function (full) { return api().StatWorkspaceMount(full); }).then(function (item) {
      if (!item || !item.ref || item.directory) throw new Error('The diagram file was not found.');
      return api().OpenDraft(item.ref);
    }).then(function (v) { return window.dinahContentWindows.open(v, 'diagrams'); })
      .catch(function (err) { if (window.dinahHost) window.dinahHost.toast('The diagram could not be opened: ' + message(err), 'danger'); });
  }
  window.dinahDiagramPicture.open = openDiagram;
})();
