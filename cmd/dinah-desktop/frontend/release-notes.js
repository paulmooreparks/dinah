// Release notes have no local directory against which file links can resolve.
(function () {
  'use strict';
  var sources = new WeakMap();
  function set(root, text) {
    text = String(text || '');
    if (sources.get(root) === text) return;
    var words = document.createElement('div');
    words.className = 'msg-text';
    words.appendChild(window.dinahHTML.fragment(window.dinahChatText.markdown(text)));
    words.querySelectorAll('a').forEach(function (link) {
      if (!/^https?:\/\//i.test(link.getAttribute('href') || '')) link.replaceWith.apply(link, Array.from(link.childNodes));
    });
    words.querySelectorAll('pre, .msg-table').forEach(function (region) {
      region.tabIndex = 0;
      region.setAttribute('role', 'region');
      region.setAttribute('aria-label', region.tagName === 'PRE' ? 'Code example' : 'Release notes table');
    });
    window.dinahChatText.openLinks(words, function () { return ''; });
    root.classList.add('release-notes');
    root.replaceChildren(words);
    sources.set(root, text);
  }
  window.dinahReleaseNotes = { set: set };
}());
