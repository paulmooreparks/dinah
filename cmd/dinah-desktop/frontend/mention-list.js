/* The @ list under a composer: a chat's, which offers windows, applets,
   documents and chats (chat-mentions.js), and a group's, which offers its
   members (bus-applets.js). Each composer decides what an @ is and what it
   offers; this draws the list and reads its keys, so both look and answer
   the keyboard the same way (FE-17).

     draw(host, items, selected, pick, options) fills host and shows it.
       Each item is { group, label, meta, badge, key }: a heading is drawn
       where group changes, badge is an element put after the words, and key,
       when given, is the row's data-mention. pick(index) runs when a row is
       clicked. options.notes are lines under the rows, and options.hint the
       line of keys at the end; with no items and a note, the note alone is
       shown.
     key(event) is 'down', 'up', 'pick' or 'close' for a key the list
       answers while open, or null.
     step(selected, count, direction) is the row Up or Down moves to. */
(function () {
  'use strict';

  function el(tag, className, text) {
    var e = document.createElement(tag);
    if (className) e.className = className;
    if (text != null) e.textContent = text;
    return e;
  }

  function draw(host, items, selected, pick, options) {
    options = options || {};
    var parts = [], group;
    items.forEach(function (it, i) {
      if (it.group && it.group !== group) { group = it.group; parts.push(el('div', 'md-section-label', group)); }
      var row = el('div', 'md-row skill-row mention-row');
      row.setAttribute('role', 'option');
      row.setAttribute('aria-selected', String(i === selected));
      if (it.key != null) row.dataset.mention = it.key;
      var a = el('a', 'md-item');
      a.href = '#';
      a.appendChild(el('span', 'mention-name', it.label));
      if (it.meta) a.appendChild(el('span', 'md-meta', it.meta));
      if (it.badge) a.appendChild(it.badge);
      // The box keeps the focus, so the caret stays where the @ was typed.
      a.addEventListener('mousedown', function (e) { e.preventDefault(); });
      a.addEventListener('click', function (e) { e.preventDefault(); pick(i); });
      row.appendChild(a);
      parts.push(row);
    });
    (options.notes || []).forEach(function (text) { parts.push(el('p', 'form-help menu-note', text)); });
    if (items.length && options.hint) parts.push(el('div', 'slash-hint', options.hint));
    host.replaceChildren.apply(host, parts);
    host.hidden = false;
    var current = host.querySelector('[aria-selected="true"]');
    if (current) current.scrollIntoView({ block: 'nearest' });
  }

  function key(e) {
    if (e.key === 'ArrowDown') return 'down';
    if (e.key === 'ArrowUp') return 'up';
    if ((e.key === 'Enter' && !e.ctrlKey && !e.metaKey) || e.key === 'Tab') return 'pick';
    if (e.key === 'Escape') return 'close';
    return null;
  }

  function step(selected, count, direction) {
    return (selected + (direction === 'down' ? 1 : count - 1)) % count;
  }

  window.dinahMentionList = { draw: draw, key: key, step: step };
})();
