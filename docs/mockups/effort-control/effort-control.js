/* The Effort control mockup. One chat window with the composer's real
   toolbar, drawn by the shipped pudl-commands.js and pudl-toolbar.js from a
   command table like the chat applet's, with the Thinking button replaced
   by an Effort button. The button always says "Effort" and shows the level
   as a fixed-width mark; pressing it opens a panel with the levels as a
   PUDL segmented control. Invented chats; nothing here talks to Dinah. */
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

  /* ---- Invented chats. The level names and descriptions are the kind each
     agent program reports; Dinah shows them as given and invents none. ---- */
  var CHATS = {
    engine: { title: 'Engine', provider: 'Codex', mode: 'Ask Before Edits', model: 'gpt-5.5-codex', current: 'medium', levels: [
      { value: 'low', name: 'low', description: 'Fast responses with lighter reasoning.' },
      { value: 'medium', name: 'medium', description: 'Balances speed and reasoning depth for everyday tasks.' },
      { value: 'high', name: 'high', description: 'Greater reasoning depth for complex problems.' }] },
    frontend: { title: 'Front End', provider: 'Claude', mode: 'Accept Edits', model: 'Opus 5.5', current: 'high', levels: [
      { value: 'low', name: 'Low', description: 'Quick answers with little deliberation.' },
      { value: 'medium', name: 'Medium', description: 'Moderate thinking for routine work.' },
      { value: 'high', name: 'High', description: 'Thorough thinking for most coding work.' },
      { value: 'xhigh', name: 'Extra High', description: 'Extended thinking for hard problems; slower and uses more of your allowance.' },
      { value: 'max', name: 'Max', description: 'The most thinking the model can do; the slowest and most costly.' }] },
    local: { title: 'Local Helper', provider: 'Ollama', mode: 'Ask Before Edits', model: 'qwen3:32b', current: 'true', levels: [
      { value: 'true', name: 'On', description: '' }] },
    notes: { title: 'Notes Writer', provider: 'Claude', mode: 'Ask Before Edits', model: 'Haiku 5.5', current: '', levels: [] }
  };
  var M = { chat: 'engine', running: false, runLevel: null, start: '', opened: null };
  function chat() { return CHATS[M.chat]; }
  function levelOf(c, v) { return c.levels.find(function (l) { return l.value === v; }); }
  function nameOf(c, v) { var l = levelOf(c, v); return l ? l.name : ''; }
  /* What each agent program documents about a change made while a turn
     runs (Engine's review, docs/review/effort-change-protocol-2026-11-16.md
     on engine/effort-protocol). Only Codex documents that the turn running
     keeps its level; Dinah promises nothing the program does not document. */
  function timing(c) { return c.provider === 'Codex' ? 'next-turn' : c.provider === 'Ollama' ? 'next-request' : 'unspecified'; }
  function afterWords(c) {
    var t = timing(c), now = nameOf(c, c.current);
    return t === 'next-turn' ? 'The turn already running keeps ' + nameOf(c, M.runLevel) + '; ' + now + ' applies from the next turn.'
      : t === 'next-request' ? 'Ollama uses ' + now + ' from its next request to the model, which may come before this turn ends.'
      : c.provider + ' doesn’t say whether ' + now + ' applies to the turn already running.';
  }

  /* ---- The Effort button's own content: the glyph, the word, the mark. ---- */
  function drawMark() {
    var c = chat(), btn = $('[data-command="effort"]'); if (!btn) return;
    var mark = btn.querySelector('.effort-mark'), now = btn.querySelector('.effort-now');
    var at = c.levels.findIndex(function (l) { return l.value === c.current; });
    mark.replaceChildren.apply(mark, c.levels.map(function (l, i) { return h('i', { 'data-on': i <= at ? '' : false }); }));
    now.textContent = c.levels.length ? nameOf(c, c.current) + ', level ' + (at + 1) + ' of ' + c.levels.length : '';
  }

  /* ---- The panel: the "Effort" label and the levels as a segmented
     control, nothing else, so it is always the same size (Paul, 16 Nov).
     A press chooses and closes it; the arrow keys move along the levels;
     Escape closes it without a change. ---- */
  function drawPanel(container) {
    var c = chat();
    var seg = h('div', { class: 'seg sm effort-seg', role: 'group', 'aria-label': 'Effort' }, c.levels.map(function (l) {
      return h('button', { type: 'button', 'data-level': l.value, 'aria-pressed': String(l.value === c.current), title: l.name, text: l.name });
    }));
    container.append(h('div', { class: 'effort-panel' }, h('div', { class: 'md-section-label', text: 'Effort' }), seg));
    seg.addEventListener('keydown', function (e) {
      var all = Array.from(seg.children), i = all.indexOf(document.activeElement), to = -1;
      if (e.key === 'ArrowRight' || e.key === 'ArrowDown') to = Math.min(all.length - 1, i + 1);
      else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') to = Math.max(0, i - 1);
      else if (e.key === 'Home') to = 0; else if (e.key === 'End') to = all.length - 1;
      if (to < 0) return;
      e.preventDefault(); e.stopPropagation(); all[to].focus();
    });
    seg.addEventListener('click', function (e) {
      var b = e.target.closest('[data-level]'); if (!b) return;
      choose(b.dataset.level);
      var pop = container.closest('[popover]'); if (pop && pop.matches(':popover-open')) pop.hidePopover();
      focusEffort();
    });
  }
  function focusEffort() {
    var b = $('[data-command="effort"]');
    if (b && b.getClientRects().length) return b.focus();
    var fold = $('.composer-toolbar .menu-btn[aria-label="Agent"]'); if (fold) fold.focus();
  }
  function choose(v) {
    var c = chat(); if (c.current === v) return;
    c.current = v; drawMark(); drawWorking(); window.pudlCommands.changed();
    if (M.running && v !== M.runLevel) note('Effort is now ' + nameOf(c, v) + ' for this chat. ' + afterWords(c));
  }

  /* ---- The command table, shaped like the chat applet's. ---- */
  function commands() {
    var c = chat();
    function menu(id, label, g, current, values) { return { id: id, label: label, glyph: g, current: current, detail: current, items: values.map(function (v) { return { id: id + '-' + v, label: v, choice: 'mock-' + id, value: v, checked: v === current, run: function () { note('In Dinah this changes the ' + label.toLowerCase() + ' to ' + v + '.'); } }; }) }; }
    var list = [
      menu('provider', 'Provider', 'app', c.provider, ['Claude', 'Codex', 'Ollama']),
      menu('mode', 'Mode', 'shield', c.mode, ['Ask Before Edits', 'Accept Edits', 'Plan']),
      menu('model', 'Model', 'tag', c.model, [c.model]),
      { id: 'add', label: 'Add Files…', glyph: 'clip', run: function () { note('In Dinah this adds files to the message.'); } },
      { id: 'skills', label: 'Skills', glyph: 'slash', run: function () { note('In Dinah this lists the skills.'); } },
      { id: 'subagents', label: 'Subagents', glyph: 'branch', hidden: true, items: [] },
      { id: 'drop-queue', label: 'Drop Queued', glyph: 'close', hidden: true, run: function () {} },
      { id: 'stop-turn', label: 'Stop', glyph: 'square', danger: true, hidden: !M.running, run: function () { finishTurn(); } }
    ];
    if (M.today) {
      list.push({ id: 'thinking', label: 'Thinking', glyph: 'gear', hidden: !c.levels.length, current: nameOf(c, c.current), detail: nameOf(c, c.current),
        items: c.levels.map(function (l, i) { return { id: 'thinking-' + i, label: l.name, detail: l.description || undefined, choice: 'mock-thinking', value: l.value, checked: l.value === c.current, run: function () { c.current = l.value; window.pudlCommands.changed(); } }; }) });
    } else {
      list.push({ id: 'effort', label: 'Effort', glyph: 'gear', hidden: !c.levels.length, detail: c.levels.length ? nameOf(c, c.current) : undefined, panel: drawPanel });
    }
    return list;
  }

  /* ---- The running turn (mockup only: the Try menu starts and ends it). ---- */
  function startTurn() { M.running = true; M.runLevel = chat().current; drawWorking(); window.pudlCommands.changed(); }
  function finishTurn() { M.running = false; M.runLevel = null; drawWorking(); window.pudlCommands.changed(); }
  function drawWorking() {
    var row = $('.mock-working'); if (!row) return;
    row.hidden = !M.running; if (!M.running) return;
    var c = chat(), later = c.current !== M.runLevel;
    row.replaceChildren(h('span', { class: 'badge', 'data-state': 'working', text: 'Working' }),
      h('span', { text: c.title + ' is working; the turn started at ' + nameOf(c, M.runLevel) + ' effort.' + (later ? ' ' + afterWords(c) : '') }));
  }

  var noteTimer;
  function note(text) {
    var region = $('.mock-note-region'); if (!region) return;
    region.replaceChildren(h('div', { class: 'toast info', role: 'status' }, h('p', { class: 'toast-text', text: text }), h('button', { class: 'toast-close', type: 'button', 'aria-label': 'Dismiss', 'data-close-note': '' })));
    clearTimeout(noteTimer); noteTimer = setTimeout(function () { region.replaceChildren(); }, 5000);
  }

  /* ---- The page ---- */
  function build() {
    document.body.className = 'frame';
    var c = chat();
    var tryMenu = bare ? null : h('div', { class: 'mock-try' }, h('span', { class: 'text-muted', text: 'Mockup:' }),
      h('select', { class: 'form-select', 'aria-label': 'Try something in the mockup', 'data-try': '' },
        h('option', { value: '', text: 'Try something…' }),
        h('option', { value: 'chat:engine', text: 'A chat with three levels' }),
        h('option', { value: 'chat:frontend', text: 'A chat with five levels' }),
        h('option', { value: 'chat:local', text: 'A chat with one level' }),
        h('option', { value: 'chat:notes', text: 'A chat with no effort setting' }),
        h('option', { value: 'turn', text: 'Start or finish a turn' }),
        h('option', { value: 'claude-turn', text: 'A turn running in a Claude chat' }),
        h('option', { value: 'theme', text: theme === 'dark' ? 'Switch to light' : 'Switch to dark' }),
        h('option', { value: 'restart', text: 'Start this state again' }),
        h('option', { value: 'index', text: 'Back to the list of states' })));
    var top = h('header', { class: 'mock-top' }, h('strong', { text: 'Dinah' }), tryMenu);
    var effortBtn = M.today ? h('button', { type: 'button', 'data-command': 'thinking' })
      : h('button', { type: 'button', class: 'btn btn-sm', 'data-command': 'effort', 'data-command-own': '', 'aria-describedby': 'effort-now' },
        glyph('gear'), h('span', { class: 'toolbar-words', text: 'Effort' }), h('span', { class: 'effort-mark', 'aria-hidden': 'true' }), h('span', { class: 'visually-hidden effort-now', id: 'effort-now' }));
    var toolbar = h('div', { class: 'toolbar composer-toolbar', role: 'toolbar', 'aria-label': 'Message', 'data-toolbar-captions': 'none', 'data-toolbar-fold': '' },
      h('div', { class: 'toolbar-group', 'data-toolbar-group': 'Agent', 'data-toolbar-glyph': 'gear', 'data-toolbar-priority': '3' },
        h('button', { type: 'button', 'data-command': 'provider' }), h('button', { type: 'button', 'data-command': 'mode' }), h('button', { type: 'button', 'data-command': 'model' }), effortBtn),
      h('div', { class: 'toolbar-group', 'data-toolbar-group': 'Message', 'data-toolbar-glyph': 'clip', 'data-toolbar-priority': '1' },
        h('button', { type: 'button', 'data-command': 'add' }), h('button', { type: 'button', 'data-command': 'skills' })),
      h('div', { class: 'toolbar-group', 'data-toolbar-group': 'Turn', 'data-toolbar-glyph': 'square', 'data-toolbar-priority': '0' },
        h('button', { type: 'button', 'data-command': 'subagents' }), h('button', { type: 'button', 'data-command': 'drop-queue' })));
    var composer = h('form', { class: 'composer', id: 'composer' },
      h('div', { class: 'composer-bar' }, toolbar,
        h('span', { class: 'composer-stop-slot' }, h('span', { class: 'composer-stop-reserve btn btn-sm', 'aria-hidden': 'true', inert: '' }, glyph('square'), h('span', { text: 'Stopping' })), h('button', { type: 'button', class: 'composer-stop', 'data-command': 'stop-turn' }))),
      h('div', { class: 'composer-row' },
        h('label', { class: 'visually-hidden', for: 'prompt', text: 'What should the agent do?' }),
        h('textarea', { class: 'form-textarea composer-input', id: 'prompt', rows: '2', placeholder: 'Say what you want the agent to do, or type / for a skill.' }),
        h('div', { class: 'composer-actions' }, h('button', { type: 'submit', class: 'btn btn-primary', id: 'send', disabled: true, title: 'Send (Ctrl+Enter)' }, h('span', { class: 'composer-send-label', text: 'Send' })), h('button', { type: 'button', class: 'btn', disabled: true, text: 'Steer' }))));
    var log = h('div', { class: 'mock-log' },
      h('p', { class: 'mock-you', text: 'Please tidy up the settings page and make the save button stay put while it saves.' }),
      h('p', { text: 'I moved the save button into the dialog footer and gave it a fixed width, so its label can change from Save to Saving without anything shifting. The tests pass.' }),
      h('div', { class: 'mock-working', hidden: true }));
    var win = h('section', { class: 'win active dinah-window mock-win', 'data-win-mode': 'maximized', role: 'region', 'aria-labelledby': 'win-title' },
      h('div', { class: 'win-head' }, glyph('comments'), h('h2', { class: 'win-title', id: 'win-title', text: c.title })),
      h('div', { class: 'win-body' }, log, composer));
    document.body.append(top, win, h('div', { class: 'mock-note-region' }));
    window.pudlCommands.register(win, commands);
    if (window.pudlToolbar && window.pudlToolbar.enhance) window.pudlToolbar.enhance(win);
    window.pudlCommands.changed();
    drawMark(); drawWorking();
  }
  function switchChat(key) {
    M.chat = key; if (M.running) M.runLevel = chat().current;
    $('#win-title').textContent = chat().title;
    drawMark(); drawWorking(); window.pudlCommands.changed();
  }

  document.addEventListener('click', function (e) { if (e.target.closest('[data-close-note]')) $('.mock-note-region').replaceChildren(); });
  document.addEventListener('submit', function (e) { e.preventDefault(); });
  /* The panel opens with focus on the level in effect, after the menu
     script has placed its own focus. */
  document.addEventListener('toggle', function (e) {
    if (e.newState !== 'open' || !e.target.querySelector) return;
    var seg = e.target.querySelector('.effort-seg'); if (!seg) return;
    setTimeout(function () { var p = seg.querySelector('[aria-pressed="true"]'); if (p && e.target.matches(':popover-open')) p.focus(); }, 0);
  }, true);
  document.addEventListener('toggle', function (e) {
    /* Escape or a click outside closes the panel; focus goes back to the button. */
    if (e.newState === 'closed' && e.target.querySelector && e.target.querySelector('.effort-panel') && (document.activeElement === document.body || !document.activeElement)) focusEffort();
  }, true);
  document.addEventListener('change', function (e) {
    if (!e.target.matches('[data-try]')) return;
    var v = e.target.value; e.target.value = '';
    if (v.indexOf('chat:') === 0) switchChat(v.slice(5));
    else if (v === 'turn') { if (M.running) finishTurn(); else startTurn(); }
    else if (v === 'claude-turn') { if (M.running) finishTurn(); switchChat('frontend'); startTurn(); }
    else if (v === 'theme') { theme = theme === 'dark' ? 'light' : 'dark'; document.documentElement.setAttribute('data-theme', theme); e.target.querySelector('option[value="theme"]').textContent = theme === 'dark' ? 'Switch to light' : 'Switch to dark'; }
    else if (v === 'restart') location.search = '?state=' + M.start + (theme === 'dark' ? '&theme=dark' : '');
    else if (v === 'index') location.search = '?state=index';
  });

  /* ---- The states. Each is a starting point; from there the page works. ---- */
  function openEffort(byKeys) {
    var b = $('[data-command="effort"]');
    /* On a phone the toolbar has folded the Agent group into a menu, and
       the same panel draws inside it. */
    if (!b.getClientRects().length) b = $('.composer-toolbar .menu-btn[aria-label="Agent"]') || b;
    if (byKeys) { b.focus(); b.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); b.click(); } else b.click();
  }
  var S = [
    { key: 'composer', title: 'The composer of a chat with three levels (Codex: low, medium, high)' },
    { key: 'open', title: 'Its Effort panel, open', after: function () { openEffort(); } },
    { key: 'five', title: 'A chat with five levels (Claude: Low to Max)', run: function () { M.chat = 'frontend'; } },
    { key: 'five-open', title: 'Its Effort panel, open', run: function () { M.chat = 'frontend'; }, after: function () { openEffort(); } },
    { key: 'one', title: 'A chat with one level (Ollama: On)', run: function () { M.chat = 'local'; } },
    { key: 'one-open', title: 'Its Effort panel, open: the one level, pressed', run: function () { M.chat = 'local'; }, after: function () { openEffort(); } },
    { key: 'running', title: 'A turn is running in a Codex chat: the panel is unchanged, and the working line says what Codex documents', run: function () { M.running = true; M.runLevel = 'high'; CHATS.engine.current = 'high'; }, after: function () { openEffort(); } },
    { key: 'running-claude', title: 'Changed while a Claude turn runs: Claude doesn’t say when a change applies, so the working line promises nothing', run: function () { M.chat = 'frontend'; M.running = true; M.runLevel = 'high'; CHATS.frontend.current = 'medium'; } },
    { key: 'running-changed', title: 'Changed while a Codex turn runs: Codex documents that the turn keeps its level and the next turn gets the new one', run: function () { M.running = true; M.runLevel = 'high'; CHATS.engine.current = 'medium'; } },
    { key: 'keyboard', title: 'The keyboard: Enter opens the panel on the level in effect, and Right moves to the next', run: function () { M.chat = 'frontend'; }, after: function () { openEffort(true); setTimeout(function () { var f = document.activeElement; if (f && f.dataset.level) f.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })); }, 60); } },
    { key: 'none', title: 'A chat with no effort setting shows no button', run: function () { M.chat = 'notes'; } },
    { key: 'today', title: 'For comparison: the Thinking button as it is today', run: function () { M.today = true; } }
  ];
  window.__mockStates = S.map(function (s) { return s.key; });
  window.__mockModel = function () { return { chat: M.chat, current: chat().current, running: M.running, runLevel: M.runLevel }; };

  function start() {
    var key = params.get('state') || 'composer';
    var state = S.find(function (s) { return s.key === key; });
    if (!state) {
      document.body.className = 'index';
      document.body.append(h('h1', { text: 'Effort control mockup' }),
        h('p', { text: 'The Thinking button in the composer becomes an Effort button. Its label never changes, and a small mark of one fixed width shows the level: one cell for each level the chat offers, filled up to the level in effect. Pressing it opens a panel with the levels as a segmented control, so the whole range shows and one press chooses. Every state below is a starting point, and from there the page works. The Try menu at the top right switches between chats with three, five, one and no levels, and starts or finishes a turn. Narrow the browser below 640 pixels to see the phone layout, where the toolbar folds and the same panel opens from the Agent menu.' }),
        h('p', {}, h('a', { href: '?state=composer', text: 'Start with the composer' }), ' is the best place to begin.'),
        h('ul', {}, S.map(function (s) { return h('li', {}, s.title + ': ', ['light', 'dark'].map(function (t) { return h('a', { href: '?state=' + s.key + (t === 'dark' ? '&theme=dark' : ''), text: t + ' ' }); })); })));
      return;
    }
    M.start = state.key;
    if (state.run) state.run();
    build();
    document.fonts.ready.then(function () {
      requestAnimationFrame(function () {
        if (state.after) state.after();
        setTimeout(function () { document.documentElement.dataset.mockSettled = 'true'; }, 200);
      });
    });
  }
  start();
}());
