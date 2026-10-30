/* Puts the markup of the singleton views (the Settings view and the Needs
   You view) into the applet storage. Each is a template at the end of
   index.html, under a heading for its applet; the chat's template is mounted
   from there by chat-applet.js. This is the first script to run after the page
   is parsed, so the controls exist before any other script looks for them. */
(function () {
  'use strict';
  var storage = document.getElementById('applet-storage');
  ['needs-you-markup', 'settings-markup'].forEach(function (id) {
    storage.appendChild(document.getElementById(id).content.cloneNode(true));
  });
})();
