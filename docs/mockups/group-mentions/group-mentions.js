/* The group mentions mockup. A group chat's composer offers the same @ list
   a one-on-one chat does (chat-mentions.js: windows, applets, open documents
   and open chats across workspaces), after the group's own members. A member
   mentioned is asked to answer, as today; anything else is a reference that
   wakes nobody, and the sent message says which is which. The list is drawn
   by the shipped mention-list.js. Invented content; nothing talks to Dinah. */
(function () {
  'use strict';
  var params = new URLSearchParams(location.search);
  var theme = params.get('theme') === 'dark' ? 'dark' : 'light';
  var bare = params.has('bare');
  document.documentElement.setAttribute('data-theme', theme);

  function h(tag, attrs) {
    var el = document.createElement(tag);
    Object.keys(attrs || {}).forEach(function (k) {
      var v = attrs[k];
      if (v === false || v === null || v === undefined) return;
      if (k === 'class') el.className = v; else if (k === 'text') el.textContent = v; else if (k === 'style') el.setAttribute('style', v); else el.setAttribute(k, v === true ? '' : v);
    });
    for (var i = 2; i < arguments.length; i++) { var c = arguments[i]; if (c === null || c === undefined || c === false) continue; (Array.isArray(c) ? c : [c]).forEach(function (x) { if (x !== null && x !== undefined && x !== false) el.append(x); }); }
    return el;
  }
  function glyph(name) { return h('span', { class: 'glyph', style: '--glyph: var(--glyph-' + name + ')', 'aria-hidden': 'true' }); }
  function $(s, root) { return (root || document).querySelector(s); }

  /* ---- Invented content ---- */
  var GROUPS = {
    group: { title: 'Release Planning', kind: 'group', members: [
      { id: 'c01', title: 'Desktop Lead', workspace: 'studio', state: 'idle' },
      { id: 'c02', title: 'Front End', workspace: 'studio', state: 'working' },
      { id: 'c06', title: 'Docs Lead', workspace: 'harbor', state: 'idle' },
      { id: 'c13', title: 'Scratch Pad', workspace: 'studio', state: 'closed' }] },
    team: { title: 'Desktop Team', kind: 'team', lead: 'c01', members: [
      { id: 'c01', title: 'Desktop Lead', workspace: 'studio', state: 'idle', lead: true },
      { id: 'c02', title: 'Front End', workspace: 'studio', state: 'working' },
      { id: 'c03', title: 'Engine', workspace: 'studio', state: 'idle' },
      { id: 'c04', title: 'Design Review', workspace: 'studio', state: 'waiting' }] }
  };
  /* What a one-on-one chat's @ list offers today, as chat-mentions.js builds it. */
  var WINDOWS = [{ id: 'files', title: 'Files', meta: 'The workspace’s files' }, { id: 'diagrams', title: 'Diagrams', meta: 'Diagrams drawn from text' }, { id: 'docs', title: 'Docs', meta: 'Documents to read and write' }, { id: 'terminal', title: 'Terminal', meta: 'A command line in the workspace' }];
  var APPLETS = [{ id: 'git', title: 'Git', meta: 'Status · Commit · Branches' }, { id: 'notifications', title: 'Notifications', meta: 'List · Answer · Dismiss' }];
  var DOCS = [{ id: 'd1', title: 'release-notes.md', meta: 'Open in Docs · studio' }, { id: 'd2', title: 'install-plan.canvas', meta: 'Open in Diagrams · studio' }];
  var CHATS = [
    { id: 'c03', title: 'Engine', workspace: 'studio', state: 'idle' },
    { id: 'c04', title: 'Design Review', workspace: 'studio', state: 'waiting' },
    { id: 'c14', title: 'Spreadsheet Helper', workspace: 'harbor', state: 'idle' },
    { id: 'c10', title: 'Research Lead', workspace: 'atlas', state: 'waiting' }];

  var M = { group: 'group', mentions: [], log: [], start: '' };
  function G() { return GROUPS[M.group]; }
  function memberIds() { return G().members.map(function (m) { return m.id; }); }
  function stateBadge(state) {
    var words = { working: 'Working', waiting: 'Waiting', idle: 'Idle', closed: 'Closed' };
    var b = h('span', { class: 'badge', text: words[state] || state }); if (state !== 'closed') b.dataset.state = state === 'waiting' ? 'waiting' : state; return b;
  }
  function tokenOf(title, kind) { return kind === 'member' || kind === 'everyone' ? '@' + title : '@' + title.replace(/\s+/g, '_'); }

  /* The @ list: the group's members first, since only they are asked to
     answer, then what a one-on-one chat offers, each a reference. A chat
     that is a member is listed once, as a member. */
  function items(q) {
    q = (q || '').toLowerCase();
    function hit(t) { return !q || t.toLowerCase().indexOf(q) >= 0 || t.toLowerCase().replace(/\s+/g, '_').indexOf(q) >= 0; }
    var out = [];
    G().members.forEach(function (m) {
      if (!hit(m.title)) return;
      out.push({ group: 'Members, asked to answer', label: m.title, meta: m.workspace + (m.lead ? ' · the team’s lead' : '') + (m.state === 'closed' ? ' · closed, opens in the background for the mention' : ''), state: m.state, kind: 'member', id: m.id });
    });
    if (!q || 'everyone'.indexOf(q) === 0) out.push({ group: 'Members, asked to answer', label: 'everyone', meta: 'Every member chat, opening closed ones in the background', kind: 'everyone', id: 'everyone' });
    WINDOWS.forEach(function (w) { if (hit(w.title)) out.push({ group: 'Windows', label: w.title, meta: w.meta + ' · a reference, asks no one', kind: 'window', id: w.id, glyph: 'window' }); });
    APPLETS.forEach(function (a) { if (hit(a.title)) out.push({ group: 'Applets', label: a.title, meta: a.meta + ' · a reference, asks no one', kind: 'applet', id: a.id, glyph: 'app' }); });
    DOCS.forEach(function (d) { if (hit(d.title)) out.push({ group: 'Open Documents', label: d.title, meta: d.meta + ' · a reference, asks no one', kind: 'document', id: d.id, glyph: 'document' }); });
    CHATS.filter(function (c) { return memberIds().indexOf(c.id) < 0; }).forEach(function (c) {
      if (hit(c.title + ' ' + c.workspace)) out.push({ group: 'Other Chats, referred to and not asked', label: c.title, meta: c.workspace + ' · not in this group, so it is not asked', state: c.state, kind: 'chat', id: c.id, glyph: 'comments' });
    });
    return out;
  }

  /* ---- The composer ---- */
  var pop = { start: -1, items: [], sel: 0 };
  function box() { return $('#group-text'); }
  function query() {
    var t = box(), before = t.value.slice(0, t.selectionStart), m = /(^|\s)@([^@\n]{0,40})$/.exec(before);
    if (!m) return null;
    var at = before.length - m[2].length - 1;
    var done = M.mentions.some(function (x) { return t.value.slice(at, at + x.token.length) === x.token; });
    return done ? null : { at: at, q: m[2] };
  }
  function drawPop() {
    var q = query(), host = $('#group-pop');
    if (!q) { closePop(); return; }
    pop.start = q.at;
    pop.items = items(q.q);
    if (!pop.items.length) { window.dinahMentionList.draw(host, [], 0, null, { notes: ['Nothing matches. Search for a member, a window, an applet, an open document or another chat.'] }); box().setAttribute('aria-expanded', 'true'); return; }
    pop.sel = Math.min(pop.sel, pop.items.length - 1);
    window.dinahMentionList.draw(host, pop.items.map(function (it) { return { group: it.group, label: it.label, meta: it.meta, badge: it.state ? stateBadge(it.state) : null, key: it.kind + ':' + it.id }; }), pop.sel, pick,
      { hint: 'Members are asked to answer; everything else is a reference and asks no one · Enter or Tab adds · Esc closes' });
    box().setAttribute('aria-expanded', 'true');
  }
  function closePop() { var host = $('#group-pop'); if (host) host.hidden = true; pop.start = -1; if (box()) box().removeAttribute('aria-expanded'); }
  function pick(i) {
    var it = pop.items[i]; if (!it) return;
    var t = box(), token = tokenOf(it.label, it.kind), end = t.selectionStart;
    t.value = t.value.slice(0, pop.start) + token + ' ' + t.value.slice(end);
    var caret = pop.start + token.length + 1; t.setSelectionRange(caret, caret);
    M.mentions = M.mentions.filter(function (m) { return m.token !== token; }).concat([{ token: token, kind: it.kind, id: it.id, title: it.label, glyph: it.glyph, workspace: (CHATS.filter(function (c) { return c.id === it.id; })[0] || {}).workspace }]);
    closePop(); drawReach(); t.focus();
  }
  /* The mentions the text still holds. */
  function bound() { var v = box() ? box().value : ''; return M.mentions.filter(function (m) { return v.indexOf(m.token) >= 0; }); }
  function names(list) { return list.length < 3 ? list.join(' and ') : list.slice(0, -1).join(', ') + ' and ' + list[list.length - 1]; }
  /* Who the message reaches is the engine's answer, as today: references
     never change it. The line also counts the references, which reach no one. */
  function reachOf(list) {
    var asked = list.filter(function (m) { return m.kind === 'member'; }), everyone = list.some(function (m) { return m.kind === 'everyone'; });
    if (everyone) return { words: 'Reaches everyone in the group', asked: G().members.map(function (m) { return m.title; }) };
    if (asked.length) return { words: 'Reaches ' + names(asked.map(function (m) { return m.title; })), asked: asked.map(function (m) { return m.title; }) };
    if (G().kind === 'team') return { words: 'Reaches the lead, Desktop Lead', asked: ['Desktop Lead'] };
    return { words: 'Reaches Front End, the member in conversation', asked: ['Front End'] };
  }
  function drawReach() {
    var list = bound(), refs = list.filter(function (m) { return m.kind !== 'member' && m.kind !== 'everyone'; }), r = reachOf(list);
    var words = r.words + (refs.length ? '. ' + (refs.length === 1 ? '1 reference asks' : refs.length + ' references ask') + ' no one.' : '');
    var line = $('.group-reach'); if (line.textContent !== words) { line.textContent = words; line.title = words; }
    $('#send').disabled = !box().value.trim();
  }
  function send() {
    var text = box().value.trim(); if (!text) return;
    var list = bound(), r = reachOf(list);
    M.log.push({ from: 'You', at: '14:20', text: text, mentions: list, asked: r.asked });
    box().value = ''; M.mentions = []; closePop(); drawLog(); drawReach(); box().focus();
  }

  /* ---- A sent message: members asked are the existing mention (bold, accent);
     references are plain, with their kind's glyph and a dotted underline. Under
     the text, the same difference in words. ---- */
  function marked(m) {
    var p = h('p', {}), text = m.text, at = 0;
    var tokens = (m.mentions || []).slice().sort(function (a, b) { return b.token.length - a.token.length; });
    while (at < text.length) {
      var next = -1, which = null;
      tokens.forEach(function (t) { var i = text.indexOf(t.token, at); if (i >= 0 && (next < 0 || i < next)) { next = i; which = t; } });
      if (next < 0) break;
      if (next > at) p.append(text.slice(at, next));
      if (which.kind === 'member' || which.kind === 'everyone') p.append(h('span', { class: 'mention', title: which.kind === 'everyone' ? 'Asks every member to answer' : 'Asks ' + which.title + ' to answer', text: which.token }));
      else p.append(h('span', { class: 'mention-ref', title: 'A reference to ' + which.title + '. It asks no one to answer.' }, glyph(which.glyph || 'link'), which.token));
      at = next + which.token.length;
    }
    if (at < text.length) p.append(text.slice(at));
    return p;
  }
  function card(m) {
    var refs = (m.mentions || []).filter(function (x) { return x.kind !== 'member' && x.kind !== 'everyone'; });
    var refNames = refs.map(function (x) { return x.kind === 'chat' ? x.title + ' (' + x.workspace + ')' : x.title; });
    return h('div', { class: 'msg' + (m.from === 'You' ? ' msg-you' : ''), 'data-sent': '' },
      h('div', { class: 'msg-head' }, h('span', { class: 'msg-label', text: m.from + ' · ' + m.at })),
      h('div', { class: 'msg-text' }, marked(m)),
      m.asked ? h('div', { class: 'msg-reach' },
        h('p', { 'data-asked': '' }, glyph('comments'), h('span', {}, 'Asked to answer: ', h('strong', { text: names(m.asked) }), '.')),
        refs.length ? h('p', { 'data-referred': '' }, glyph('link'), h('span', {}, 'Referred to, not asked: ', h('strong', { text: names(refNames) }), '.')) : null) : null);
  }
  function drawLog() {
    var log = $('.mock-log');
    log.replaceChildren.apply(log, M.log.map(card));
    log.scrollTop = log.scrollHeight;
  }

  var noteTimer;
  function note(text) {
    var region = $('.mock-note-region'); if (!region) return;
    region.replaceChildren(h('div', { class: 'toast info', role: 'status' }, h('p', { class: 'toast-text', text: text }), h('button', { class: 'toast-close', type: 'button', 'aria-label': 'Dismiss', 'data-close-note': '' })));
    clearTimeout(noteTimer); noteTimer = setTimeout(function () { region.replaceChildren(); }, 4500);
  }

  /* ---- The page ---- */
  function build() {
    document.body.className = 'frame';
    var g = G();
    var tryMenu = bare ? null : h('div', { class: 'mock-try' }, h('span', { class: 'text-muted', text: 'Mockup:' }),
      h('select', { class: 'form-select', 'aria-label': 'Try something in the mockup', 'data-try': '' },
        h('option', { value: '', text: 'Try something…' }),
        h('option', { value: 'group', text: 'A group chat' }),
        h('option', { value: 'team', text: 'A team chat' }),
        h('option', { value: 'theme', text: theme === 'dark' ? 'Switch to light' : 'Switch to dark' }),
        h('option', { value: 'again', text: 'Start this state again' }),
        h('option', { value: 'index', text: 'Back to the list of states' })));
    var composer = h('form', { class: 'group-composer bus-composer', id: 'composer' },
      h('div', { class: 'slash-pop group-mention-pop', id: 'group-pop', role: 'listbox', 'aria-label': 'Mentions', hidden: true }),
      h('div', { class: 'composer-row' },
        h('label', { class: 'visually-hidden', for: 'group-text', text: 'Message to the Group' }),
        h('textarea', { class: 'form-textarea', id: 'group-text', rows: 3, 'aria-autocomplete': 'list', 'aria-controls': 'group-pop', placeholder: 'Write to the group. Type @ for a member to ask, or for a window, an applet, a document or another chat to refer to.' }),
        h('div', { class: 'composer-actions' },
          h('button', { type: 'button', class: 'btn', 'data-add': '', title: 'Add a mention', text: 'Add…' }),
          h('button', { type: 'submit', class: 'btn btn-primary', id: 'send', disabled: true, text: 'Send' }))),
      h('p', { class: 'form-help group-reach' }));
    var win = h('section', { class: 'win active dinah-window mock-win', 'data-win-mode': 'maximized', role: 'region', 'aria-labelledby': 'win-title' },
      h('div', { class: 'win-head' }, glyph('comments'), h('h2', { class: 'win-title', id: 'win-title', text: g.title })),
      h('div', { class: 'win-body' }, h('div', { class: 'mock-log', role: 'log', 'aria-label': 'Conversation' }), composer));
    document.body.append(h('header', { class: 'mock-top' }, h('strong', { text: 'Dinah' }), tryMenu), win, h('div', { class: 'mock-note-region' }));
    var t = box();
    t.addEventListener('input', function () { pop.sel = 0; drawPop(); drawReach(); });
    t.addEventListener('keydown', function (e) {
      var host = $('#group-pop'); if (host.hidden || !pop.items.length) { if (e.key === 'Escape') closePop(); return; }
      var a = window.dinahMentionList.key(e); if (!a) return;
      e.preventDefault();
      if (a === 'down' || a === 'up') { pop.sel = window.dinahMentionList.step(pop.sel, pop.items.length, a); drawPop(); }
      else if (a === 'pick') pick(pop.sel); else closePop();
    });
    t.addEventListener('blur', function () { setTimeout(function () { if (document.activeElement !== box()) closePop(); }, 150); });
    composer.addEventListener('submit', function (e) { e.preventDefault(); send(); });
  }
  document.addEventListener('click', function (e) {
    if (e.target.closest('[data-add]')) {
      var t = box(), before = t.value.slice(0, t.selectionStart), after = t.value.slice(t.selectionEnd), ins = (before && !/\s$/.test(before) ? ' ' : '') + '@';
      t.value = before + ins + after; t.setSelectionRange(before.length + ins.length, before.length + ins.length); t.focus(); pop.sel = 0; drawPop(); return;
    }
    if (e.target.closest('[data-close-note]')) $('.mock-note-region').replaceChildren();
  });
  document.addEventListener('change', function (e) {
    if (!e.target.matches('[data-try]')) return;
    var v = e.target.value; e.target.value = '';
    if (v === 'group' || v === 'team') location.search = '?state=' + (v === 'team' ? 'team' : 'list') + (theme === 'dark' ? '&theme=dark' : '');
    else if (v === 'theme') { theme = theme === 'dark' ? 'light' : 'dark'; document.documentElement.setAttribute('data-theme', theme); e.target.querySelector('option[value="theme"]').textContent = theme === 'dark' ? 'Switch to light' : 'Switch to dark'; }
    else if (v === 'again') location.search = '?state=' + M.start + (theme === 'dark' ? '&theme=dark' : '');
    else if (v === 'index') location.search = '?state=index';
  });

  /* ---- The states. Each is a starting point; from there the page works. ---- */
  var EARLIER = [
    { from: 'Desktop Lead', at: '14:02', text: '@Front End, is the installer change in the release branch yet?', mentions: [{ token: '@Front End', kind: 'member', title: 'Front End' }], asked: ['Front End'] },
    { from: 'Front End', at: '14:06', text: 'Yes, merged this morning. The notes still need the new screenshot.', mentions: [], asked: null }
  ];
  function typeInto(text) { var t = box(); t.focus(); t.value = text; t.setSelectionRange(text.length, text.length); pop.sel = 0; drawPop(); drawReach(); }
  function chooseAll(picks) {
    picks.forEach(function (p) { var t = box(); t.value += (t.value && !/\s$/.test(t.value) ? ' ' : '') + '@' + p.q; t.setSelectionRange(t.value.length, t.value.length); drawPop(); var i = pop.items.findIndex(function (it) { return it.kind === p.kind && it.label === p.label; }); pick(i < 0 ? 0 : i); if (p.after) { if (/^[,.;:’']/.test(p.after)) t.value = t.value.replace(/ $/, ''); t.value += p.after; t.setSelectionRange(t.value.length, t.value.length); } });
    drawReach();
  }
  var COMPOSED = [{ q: 'Docs', kind: 'member', label: 'Docs Lead', after: ', please check the screenshot in ' }, { q: 'release', kind: 'document', label: 'release-notes.md', after: 'against ' }, { q: 'Spreadsheet', kind: 'chat', label: 'Spreadsheet Helper', after: '’s numbers, and open it in ' }, { q: 'Docs', kind: 'window', label: 'Docs', after: 'when you do.' }];
  var S = [
    { key: 'list', title: 'Typing @ in a group chat: members first, then windows, applets, open documents and other chats', after: function () { typeInto('@'); } },
    { key: 'filtered', title: 'Typing @ and a few letters searches all of them', after: function () { typeInto('Please look at @rel'); } },
    { key: 'composed', title: 'A message with a member asked and three references: the line under the box says who it reaches', after: function () { chooseAll(COMPOSED); } },
    { key: 'sent', title: 'Sent: the member asked and the references look different, and the lines under the message say which is which', after: function () { chooseAll(COMPOSED); send(); } },
    { key: 'references-only', title: 'Only references: the message reaches who it would have reached anyway', after: function () { chooseAll([{ q: 'install', kind: 'document', label: 'install-plan.canvas', after: 'is updated for the new installer.' }]); } },
    { key: 'team', title: 'A team chat: the same list, and a message with no member mentioned goes to the lead', run: function () { M.group = 'team'; }, after: function () { typeInto('@'); } },
    { key: 'empty', title: 'The composer before anything is typed', after: function () { drawReach(); } }
  ];
  window.__mockStates = S.map(function (s) { return s.key; });
  window.__mockModel = function () { return { mentions: bound().map(function (m) { return { token: m.token, kind: m.kind }; }), log: M.log.map(function (m) { return { text: m.text, asked: m.asked, refs: (m.mentions || []).filter(function (x) { return x.kind !== 'member' && x.kind !== 'everyone'; }).map(function (x) { return x.title; }) }; }) }; };

  function start() {
    var key = params.get('state') || 'list';
    var state = S.find(function (s) { return s.key === key; });
    if (!state) {
      document.body.className = 'index';
      document.body.append(h('h1', { text: 'Group mentions mockup' }),
        h('p', { text: 'A group or team chat’s composer offers the same @ list as a one-on-one chat: windows, applets, open documents and other chats across workspaces, after the group’s own members. Mentioning a member asks it to answer, as today. Anything else is a reference that wakes nobody, and a sent message shows the difference in its marks and in words under it. Every state below is a starting point, and from there the page works. Narrow the browser below 640 pixels for the phone layout.' }),
        h('p', {}, h('a', { href: '?state=list', text: 'Start with the @ list' }), ' is the best place to begin.'),
        h('ul', {}, S.map(function (s) { return h('li', {}, s.title + ': ', ['light', 'dark'].map(function (t) { return h('a', { href: '?state=' + s.key + (t === 'dark' ? '&theme=dark' : ''), text: t + ' ' }); })); })));
      return;
    }
    M.start = state.key;
    if (state.run) state.run();
    M.log = EARLIER.slice();
    build(); drawLog(); drawReach();
    document.fonts.ready.then(function () {
      if (state.after) state.after();
      setTimeout(function () { document.documentElement.dataset.mockSettled = 'true'; }, 100);
    });
  }
  start();
}());
