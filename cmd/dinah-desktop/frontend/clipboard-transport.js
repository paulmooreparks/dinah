/* The host side of PUDL's clipboard contract (PUDL docs/EDIT-MENUS.md,
   "Host clipboard"). PUDL's edit menu for plain fields reads and writes the
   clipboard through window.pudlClipboard, and a host gives it a transport
   with setTransport({ readText, writeText }). Dinah reads the clipboard
   with App.ClipboardRead, which needs no permission prompt from the web
   view, and writes it with the Wails runtime's ClipboardSetText. Both are
   bridged by the shell server too, so this file runs unchanged in the main
   page and in a document window's page.

   The file must load after pudl/pudl-menu.js, which defines
   window.pudlClipboard as it runs. Both are deferred scripts, so their
   order in the page is the order they run in. No page loads pudl-menu.js
   later or by script, and the page's own frames (the theme previews and
   the skin contrast check) load no scripts, so setting the transport once
   here covers every accessor Dinah has.

   A PUDL release before 0.109.0 has no window.pudlClipboard, and then
   this file does nothing at all. It never defines the accessor itself.

   Every failure becomes a rejection with an Error, including a write the
   runtime reports as false, since the contract says a refused operation
   must reject. Running the file twice sets the transport once, because
   each setTransport call also retires the menu's pending edits. */
(function () {
  'use strict';

  var clipboard = window.pudlClipboard;
  if (!clipboard || typeof clipboard.setTransport !== 'function') return;
  if (window.dinahClipboardTransport) return;

  function failure(error, fallback) {
    if (error instanceof Error) return error;
    var text = error && error.message ? String(error.message) : error ? String(error) : '';
    return new Error(text || fallback);
  }

  function app() {
    var helpers = window.dinahHelpers;
    return helpers && typeof helpers.app === 'function' ? helpers.app() : null;
  }

  var transport = Object.freeze({
    readText: function () {
      return new Promise(function (resolve) {
        var host = app();
        if (!host || typeof host.ClipboardRead !== 'function') throw new Error('The clipboard cannot be read here.');
        resolve(host.ClipboardRead());
      }).then(function (content) {
        if (content == null) return '';
        if (typeof content !== 'object') throw new Error('The clipboard was read in a form this page does not understand.');
        return typeof content.text === 'string' ? content.text : '';
      }, function (error) {
        throw failure(error, 'The clipboard could not be read.');
      });
    },
    writeText: function (text) {
      return new Promise(function (resolve) {
        var runtime = window.runtime;
        if (!runtime || typeof runtime.ClipboardSetText !== 'function') throw new Error('The clipboard cannot be written here.');
        resolve(runtime.ClipboardSetText(String(text)));
      }).then(function (written) {
        if (written === false) throw new Error('The clipboard could not be written.');
      }, function (error) {
        throw failure(error, 'The clipboard could not be written.');
      });
    }
  });

  clipboard.setTransport(transport);
  window.dinahClipboardTransport = transport;
})();
