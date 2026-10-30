/* Draws the chrome every mockup page shares: Dinah's application bar with
   its menu bar, and PUDL's taskbar with Dinah's tray. The menu bar is
   written as PUDL's menu bar script would draw it, because that script is
   not loaded here. The body's data-front gives the title of the window in
   front, whose menus stand in a second group, and data-open says which
   title is shown pressed because its menu is open. */
(function () {
  'use strict';
  var body = document.body;
  var front = body.dataset.front || '', open = body.dataset.open || '', glyph = body.dataset.frontGlyph || 'comments';
  function title(words, extra) {
    var pressed = open && words === open ? ' aria-expanded="true"' : ' aria-expanded="false"';
    return '<button type="button" class="menubar-title' + (extra || '') + '"' + pressed + ' aria-haspopup="menu"><span class="menubar-words">' + words + '</span></button>';
  }
  var site = '<div class="menubar-menu" role="presentation"><button type="button" class="menubar-glyph" aria-label="Menus"></button>' +
    title('Dinah', ' menubar-brand') + title('Documents') + title('View') + title('Tools') + title('Window') + title('Help') + '</div>';
  var frontGroup = front ? '<div class="menubar-menu menubar-front" role="presentation"><button type="button" class="menubar-glyph" aria-label="Window Menus"></button>' +
    '<button type="button" class="menubar-title" aria-expanded="' + (open === front) + '"><span class="glyph" style="--glyph: var(--glyph-' + glyph + ')" aria-hidden="true"></span><span class="menubar-words">' + front + '</span></button>' +
    (body.dataset.actions === 'none' ? '' : '<button type="button" class="menubar-title" id="actions-title" aria-expanded="' + (open === 'Actions') + '"><span class="glyph" style="--glyph: var(--glyph-list)" aria-hidden="true"></span><span class="menubar-words">Actions</span></button>') +
    '</div>' : '';
  var bar = document.createElement('header');
  bar.className = 'topbar';
  bar.innerHTML = '<nav class="menubar" data-menubar aria-label="Dinah"><div class="menubar-row">' + site + frontGroup + '</div></nav>' +
    '<span class="brand app-brand"><img src="../../../cmd/dinah-desktop/frontend/dinah-mark.svg" alt="" width="20" height="20"><span>Dinah</span></span>';
  body.insertBefore(bar, body.firstChild);

  var foot = document.createElement('footer');
  foot.className = 'win-bar';
  foot.innerHTML = '<nav class="win-dock" aria-label="Open Windows"></nav><nav class="status-area" aria-label="Status">' +
    '<button type="button" class="status-item"><span class="glyph" style="--glyph: var(--glyph-folder)" aria-hidden="true"></span><span>studio</span></button>' +
    '<button type="button" class="status-item"><span class="glyph" style="--glyph: var(--glyph-warning)" aria-hidden="true"></span><span class="status-words">Notifications</span></button>' +
    '<button type="button" class="status-item"><span class="glyph" style="--glyph: var(--glyph-ring)" aria-hidden="true"></span><span class="status-words">Claude</span><span class="badge positive">on</span></button></nav>';
  body.appendChild(foot);
})();
