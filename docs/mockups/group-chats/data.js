/* The chats and groups every mockup page shows. Paul is the person. A
   chat's state is working, open or closed; an open chat may be in a
   window or, after a message woke it, open in the background. */
window.mock = {
  person: 'Paul',
  chats: [
    { id: 'dinah', title: 'Dinah Development', workspace: 'studio', provider: 'Claude', state: 'working', active: 'Now' },
    { id: 'ledger', title: 'Ledger Development', workspace: 'ledger', provider: 'Claude', state: 'open', active: '5 min ago' },
    { id: 'timesheet', title: 'Ledger Timesheet Applet', workspace: 'ledger', provider: 'Claude', state: 'closed', active: 'Yesterday' },
    { id: 'pudl', title: 'PUDL Development', workspace: 'pudl', provider: 'Claude', state: 'open', active: '20 min ago' },
    { id: 'tela', title: 'Tela Relay Fixes', workspace: 'tela', provider: 'Codex', state: 'closed', active: '3 days ago' },
    { id: 'overt', title: 'Overt Parser', workspace: 'Overt', provider: 'Devin', state: 'closed', active: 'Last week' },
    { id: 'untitled', title: 'Untitled Chat in studio', workspace: 'studio', provider: 'Ollama', state: 'open', active: '1 hour ago' }
  ],
  groups: [
    { title: 'PUDL Release Review', members: ['pudl', 'dinah', 'ledger'], when: 'today 10:09', unread: 3 },
    { title: 'Dinah and Ledger', members: ['dinah', 'ledger'], when: 'today 09:12', unread: 0 }
  ],
  // The state's word and the PUDL badge that shows it with its glyph:
  // working is the accent badge with the ring, open the positive badge
  // with the circle, closed the plain badge with Dinah's small circle.
  states: {
    working: { word: 'Working', badge: 'badge accent running', label: 'The agent is working' },
    open: { word: 'Open', badge: 'badge positive', label: 'Open in a window' },
    background: { word: 'Open', badge: 'badge positive', label: 'Open in the background, without a window' },
    closed: { word: 'Closed', badge: 'badge', label: 'Closed; a message that wakes it opens it in the background' }
  },
  chat: function (id) { return this.chats.find(function (c) { return c.id === id; }); },
  // Workspaces in alphabetical order, each with its chats, most recently
  // active first (the order the data is written in).
  byWorkspace: function () {
    var names = [];
    this.chats.forEach(function (c) { if (names.indexOf(c.workspace) < 0) names.push(c.workspace); });
    names.sort(function (a, b) { return a.localeCompare(b, 'en', { sensitivity: 'base' }); });
    var chats = this.chats;
    return names.map(function (n) { return { workspace: n, chats: chats.filter(function (c) { return c.workspace === n; }) }; });
  },
  badge: function (state) {
    var s = this.states[state];
    return '<span class="' + s.badge + '" title="' + s.label + '">' + s.word + '</span>';
  },
  esc: function (t) { return String(t).replace(/&/g, '&amp;').replace(/</g, '&lt;'); }
};
