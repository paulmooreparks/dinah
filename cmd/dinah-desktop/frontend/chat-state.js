/* The engine owns execution, lifecycle and aggregate priority. Every page
   surface uses this one presentation of that state (plans/chat-states.md). */
(function () {
  'use strict';
  // PUDL's static state glyphs (0.96.1, WINDOW-STATUS.md): each state keeps
  // one still glyph under every motion preference.
  /* Background work is what the agent runs after its turn has ended
     (docs/design/background-activity.md). PUDL's window states have no
     such state, so it is drawn as PUDL's working state, its glyph and
     colour, with its own words; windows show it as working with this label
     (pudl). */
  var mapping = {
    working: { word: 'Working', colour: 'accent', glyph: 'play' },
    background: { word: 'Background work', short: 'Background', colour: 'accent', glyph: 'play', pudl: 'working' },
    waiting: { word: 'Waiting', colour: 'warn', glyph: 'pause' },
    stuck: { word: 'Stuck', colour: 'danger', glyph: 'warning' },
    idle: { word: 'Idle', colour: '', glyph: 'square' }
  };
  // Every short word a labelled compact mark can show (badge).
  var SHORT = ['Working', 'Background', 'Waiting', 'Stuck', 'Idle', 'Closed', 'Retired', 'Elsewhere', 'Unknown'];
  function shortOf(spec) { return spec.short || spec.word; }
  /* The background work still running, in words, from the counts the engine
     gives on any execution state: "2 background tasks, 1 subagent". */
  function backgroundWords(exec) {
    var tasks = exec && exec.tasks || 0, helpers = exec && exec.subagents || 0, parts = [];
    if (tasks) parts.push(tasks + (tasks === 1 ? ' background task' : ' background tasks'));
    if (helpers) parts.push(helpers + (helpers === 1 ? ' subagent' : ' subagents'));
    return parts.join(', ');
  }
  var chats = new Map(), buses = new Map(), groups = new Map();
  function read(value) {
    value = value || {};
    return chats.get(value.sessionId || value.chat || value.id) || buses.get(value.bus || value.id) || value.chatState || value;
  }
  function counts(value) {
    return value && (groups.get(value.name) || value.counts || value.team || value);
  }
  function apply(snapshot) {
    if (!snapshot || !window.dinahStateOrder.accept('agent:chatstate', '', snapshot)) return false;
    chats.clear(); buses.clear(); groups.clear();
    (snapshot.chats || []).forEach(function (entry) {
      if (entry.sessionId) chats.set(entry.sessionId, entry);
      if (entry.bus) buses.set(entry.bus, entry);
    });
    (snapshot.groups || []).forEach(function (entry) { groups.set(entry.name, entry.counts); });
    document.dispatchEvent(new CustomEvent('dinah:execution-state', { detail: snapshot }));
    return true;
  }
  function connect(options) {
    function changed(snapshot) { if (apply(snapshot) && options.changed) options.changed(snapshot); }
    function refresh() {
      var app = window.dinahHelpers.app();
      return app && app.ChatStates ? Promise.resolve(app.ChatStates()).then(changed).catch(options.fail) : Promise.resolve();
    }
    window.dinahHost.on('agent:chatstate', changed);
    window.dinahHost.onResync(refresh);
    return refresh();
  }
  function describeState(value, options) {
    value = value || {}; options = options || {};
    var aggregate = Object.prototype.hasOwnProperty.call(value, 'top') || typeof value.unknown === 'number' || typeof value.closed === 'number' || !!(value.counts && !value.life && !value.chatState && !value.exec);
    value = aggregate ? counts(value) : read(value);
    var state = aggregate ? value.top : value.exec && value.exec.state;
    var life = value.life;
    if (!aggregate && value.elsewhere) return { text: value.elsewhere, short: 'Elsewhere', kind: '', cls: 'chat-state chat-lifecycle', label: value.elsewhere, glyph: '' };
    if (!aggregate && (life === 'closed' || life === 'retired')) return { text: life === 'closed' ? 'Closed' : 'Retired', short: life === 'closed' ? 'Closed' : 'Retired', kind: '', cls: 'chat-state chat-lifecycle', label: life === 'closed' ? 'The session has ended' : 'The team member is retired', glyph: '' };
    var spec = mapping[state], why = !aggregate && value.exec && value.exec.why || '', words = spec ? spec.word : '';
    if (aggregate) {
      var totals = value.counts || value;
      var parts = ['stuck', 'waiting', 'working', 'background', 'idle', 'unknown', 'closed'].filter(function (name) { return totals[name] > 0; }).map(function (name) { return totals[name] + ' ' + (name === 'unknown' ? 'running elsewhere' : name === 'background' ? 'with background work' : name); });
      words = parts.length ? parts.join(', ') : words;
      if (!spec && words) return { text: words, short: totals.unknown ? 'Elsewhere' : 'Closed', kind: '', cls: 'chat-state chat-lifecycle', label: words, glyph: '' };
      if (!totals.stuck && !totals.waiting && !totals.working && !totals.background && !totals.idle && totals.closed) words = spec.word + ' (' + words + ')';
    } else if (why && options.reason !== false && (state === 'waiting' || state === 'stuck' || state === 'background')) words += ' (' + why + ')';
    if (!spec) return { text: 'Unknown', short: 'Unknown', kind: '', cls: 'chat-state chat-state-unknown', label: 'The engine has not supplied an execution state', glyph: '' };
    /* A chat in another state with background work still running says so
       in its name and tooltip, so Working after a turn never hides it. */
    var also = !aggregate && state !== 'background' ? backgroundWords(value.exec) : '';
    var label = aggregate ? spec.word + ': ' + words : spec.word + (why ? ': ' + why : '') + (also ? '. Background work continues: ' + also : '');
    return { state: state, pudl: spec.pudl || state, text: words, short: shortOf(spec), kind: spec.colour, glyph: spec.glyph, cls: 'chat-state chat-state-' + state, label: label, background: backgroundWords(value.exec) };
  }
  function describe(value, options) {
    var spec = describeState(value, options);
    spec.variables = { '--chat-state-colour': 'var(--' + (spec.kind || 'text-muted') + ')' };
    return spec;
  }
  // The chip each short word is drawn in, for the hidden copies of a list's
  // state chip (badge).
  var RESERVES = SHORT.map(function (word) {
    var state = Object.keys(mapping).filter(function (k) { return shortOf(mapping[k]) === word; })[0];
    return { text: word, pudl: state ? mapping[state].pudl || state : '', cls: 'chat-state ' + (state ? 'chat-state-' + state : word === 'Unknown' ? 'chat-state-unknown' : 'chat-lifecycle') + ' chat-state-reserve' };
  });
  /* A chip with an execution state is PUDL's execution-state badge (0.96.4):
     data-state gives it the state's glyph and colour, the ones the taskbar's
     stack rows draw, and the words are Dinah's. Background work is PUDL's
     working state with its own words. A lifecycle word has no state and
     stays a plain badge. */
  function chip(spec, text) {
    var el = window.dinahBrowserList.badge({ text: text, cls: spec.cls, label: spec.label, chip: false });
    if (spec.pudl) el.dataset.state = spec.pudl;
    return el;
  }
  function badge(value, options) {
    var spec = describe(value, options);
    /* A chat list draws the standard chip with the state's short word, as
       wide as its own glyph and word. It sits at the right of an unpainted
       slot that hidden copies of every state's chip size to the widest, so
       the row never moves when the state changes. */
    if (options && options.labelled) {
      var slot = document.createElement('span'); slot.className = 'chat-state-slot' + (options.className ? ' ' + options.className : '');
      slot.appendChild(chip(spec, spec.short));
      RESERVES.forEach(function (r) { var copy = chip(r, r.text); copy.setAttribute('aria-hidden', 'true'); copy.removeAttribute('title'); slot.appendChild(copy); });
      return slot;
    }
    /* Every chip shows the state's short word, as the title bar's does, and
       keeps the full description, a reason or counts included, in its
       tooltip and accessible name (Paul, 2 November 2026). */
    var el = chip(spec, spec.short);
    if (options && options.className) el.classList.add(options.className);
    if (options && options.compact) {
      el.classList.add('chat-state-compact');
      var words = document.createElement('span'); words.className = 'chat-state-words'; words.textContent = spec.glyph ? '' : spec.text;
      el.replaceChildren(words);
    }
    return el;
  }
  window.dinahChatState = { describe: describe, badge: badge, read: read, counts: counts, apply: apply, connect: connect };
})();
